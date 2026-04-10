package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	hxlib "hx/src/hx"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chzyer/readline"
	urootcore "github.com/u-root/u-root/pkg/core"
	urootgzip "github.com/u-root/u-root/pkg/core/gzip"
	urootln "github.com/u-root/u-root/pkg/core/ln"
	"golang.org/x/term"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

type shell_app struct {
	cwd        string
	argv0      string
	env        []string
	builtins   map[string]builtin_def
	runner     *interp.Runner
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	is_windows bool
}

type builtin_def struct {
	name    string
	usage   string
	handler builtin_handler
}

type builtin_context struct {
	ctx         context.Context
	app         *shell_app
	name        string
	args        []string
	stdin       io.Reader
	stdout      io.Writer
	stderr      io.Writer
	working_dir string
	core_base   urootcore.Base
}

type shell_options struct {
	command     string
	script_path string
	script_args []string
	interactive bool
}

type builtin_handler func(builtin_context) int

func main() {
	os.Exit(run_main())
}

func run_main() int {
	opts, err := parse_cli_args(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	app, err := new_shell_app()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return app.run(opts)
}

func parse_cli_args(argv []string) (shell_options, error) {
	opts := shell_options{}
	args := argv[1:]
	if len(args) == 0 {
		opts.interactive = true
		return opts, nil
	}
	if args[0] == "-c" {
		if len(args) < 2 {
			return opts, errors.New("missing command after -c")
		}
		opts.command = args[1]
		opts.script_args = args[2:]
		return opts, nil
	}
	opts.script_path = args[0]
	opts.script_args = args[1:]
	return opts, nil
}

func new_shell_app() (*shell_app, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	app := &shell_app{
		cwd:        cwd,
		argv0:      filepath.Base(os.Args[0]),
		env:        append([]string{}, os.Environ()...),
		builtins:   map[string]builtin_def{},
		stdin:      os.Stdin,
		stdout:     os.Stdout,
		stderr:     os.Stderr,
		is_windows: runtime.GOOS == "windows",
	}
	app.register_builtins()
	return app, nil
}

func (app *shell_app) run(opts shell_options) int {
	params := append([]string{app.argv0}, opts.script_args...)
	if err := app.init_runner(params, opts.interactive); err != nil {
		fmt.Fprintln(app.stderr, err)
		return 1
	}
	if opts.command != "" {
		return app.run_with_error(app.run_command_text(opts.command))
	}
	if opts.script_path != "" {
		return app.run_with_error(app.run_script_file(opts.script_path))
	}
	return app.run_interactive()
}

func (app *shell_app) run_with_error(err error) int {
	if err == nil {
		return 0
	}
	if status, ok := interp.IsExitStatus(err); ok {
		return int(status)
	}
	fmt.Fprintln(app.stderr, err)
	return 1
}

func (app *shell_app) init_runner(params []string, interactive bool) error {
	runner, err := interp.New(
		interp.Dir(app.cwd),
		interp.Env(expand.ListEnviron(app.env...)),
		interp.Params(params...),
		interp.StdIO(app.stdin, app.stdout, app.stderr),
		interp.Interactive(interactive),
		interp.ExecHandler(app.exec_program),
	)
	if err != nil {
		return err
	}
	app.runner = runner
	return nil
}

func (app *shell_app) register_builtins() {
	app.builtins["base64"] = builtin_def{name: "base64", usage: "base64 [-d] [file]", handler: builtin_base64}
	app.builtins["cat"] = builtin_def{name: "cat", usage: "cat [file...]", handler: builtin_cat}
	app.builtins["cp"] = builtin_def{name: "cp", usage: "cp [-r] source... destination", handler: builtin_cp}
	app.builtins["find"] = builtin_def{name: "find", usage: "find [path] [-name pattern]", handler: builtin_find}
	app.builtins["gzip"] = builtin_def{name: "gzip", usage: "gzip [file...]", handler: adapt_core_command(func() urootcore.Command { return urootgzip.New("gzip") })}
	app.builtins["head"] = builtin_def{name: "head", usage: "head [-n count] [file]", handler: builtin_head}
	app.builtins["hx"] = builtin_def{name: "hx", usage: "hx [flags] <source> [destination]", handler: builtin_hx}
	app.builtins["ln"] = builtin_def{name: "ln", usage: "ln [-svfTiLPr] target... link", handler: adapt_core_command(func() urootcore.Command { return urootln.New() })}
	app.builtins["ls"] = builtin_def{name: "ls", usage: "ls [-a] [-l] [path...]", handler: builtin_ls}
	app.builtins["mkdir"] = builtin_def{name: "mkdir", usage: "mkdir [-p] path...", handler: builtin_mkdir}
	app.builtins["mktemp"] = builtin_def{name: "mktemp", usage: "mktemp [-d] [template]", handler: builtin_mktemp}
	app.builtins["mv"] = builtin_def{name: "mv", usage: "mv source... destination", handler: builtin_mv}
	app.builtins["rm"] = builtin_def{name: "rm", usage: "rm [-r] [-f] path...", handler: builtin_rm}
	app.builtins["tail"] = builtin_def{name: "tail", usage: "tail [-n count] [file]", handler: builtin_tail}
	app.builtins["touch"] = builtin_def{name: "touch", usage: "touch file...", handler: builtin_touch}
}

func (app *shell_app) run_interactive() int {
	if stdin_file, ok := app.stdin.(*os.File); ok && term.IsTerminal(int(stdin_file.Fd())) {
		return app.run_interactive_readline(stdin_file)
	}
	return app.run_interactive_plain()
}

func (app *shell_app) run_interactive_plain() int {
	reader := bufio.NewReader(app.stdin)
	for {
		fmt.Fprint(app.stdout, app.prompt())
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			fmt.Fprintln(app.stderr, err)
			return 1
		}
		line = strings.TrimSpace(line)
		if line != "" {
			runErr := app.run_command_text(line)
			if runErr != nil {
				if status, ok := interp.IsExitStatus(runErr); ok {
					if app.runner.Exited() {
						return int(status)
					}
					fmt.Fprintf(app.stderr, "exit status %d\n", status)
				} else {
					fmt.Fprintln(app.stderr, runErr)
				}
			}
		}
		if errors.Is(err, io.EOF) {
			return 0
		}
		if app.runner.Exited() {
			return 0
		}
	}
}

func (app *shell_app) run_interactive_readline(stdin_file *os.File) int {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          app.prompt(),
		AutoComplete:    app,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		Stdin:           stdin_file,
		Stdout:          app.stdout,
		Stderr:          app.stderr,
	})
	if err != nil {
		fmt.Fprintln(app.stderr, err)
		return app.run_interactive_plain()
	}
	defer rl.Close()
	for {
		line, err := rl.Readline()
		if errors.Is(err, readline.ErrInterrupt) {
			if line == "" {
				fmt.Fprintln(app.stdout)
				continue
			}
		}
		if errors.Is(err, io.EOF) {
			fmt.Fprintln(app.stdout)
			return 0
		}
		if err != nil {
			fmt.Fprintln(app.stderr, err)
			return 1
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		runErr := app.run_command_text(line)
		if runErr != nil {
			if status, ok := interp.IsExitStatus(runErr); ok {
				if app.runner.Exited() {
					return int(status)
				}
				fmt.Fprintf(app.stderr, "exit status %d\n", status)
			} else {
				fmt.Fprintln(app.stderr, runErr)
			}
		}
		if app.runner.Exited() {
			return 0
		}
	}
}

func (app *shell_app) prompt() string {
	return "goshx$ "
}

func (app *shell_app) Do(line []rune, pos int) ([][]rune, int) {
	if pos < 0 || pos > len(line) {
		pos = len(line)
	}
	segment_start := completion_segment_start(line[:pos])
	token_start := completion_token_start(line[segment_start:pos]) + segment_start
	token := string(line[token_start:pos])
	arg_index := completion_arg_index(line[segment_start:pos], token_start-segment_start)
	suggestions := []string{}
	if should_complete_paths(token, arg_index) {
		suggestions = app.complete_path_candidates(token)
	} else {
		suggestions = app.complete_command_candidates(token)
	}
	if len(suggestions) == 0 {
		return nil, 0
	}
	out := make([][]rune, 0, len(suggestions))
	for _, suggestion := range suggestions {
		if !strings.HasPrefix(strings.ToLower(suggestion), strings.ToLower(token)) {
			continue
		}
		out = append(out, []rune(suggestion[len(token):]))
	}
	return out, len(token)
}

func completion_segment_start(line []rune) int {
	last := 0
	for i, r := range line {
		switch r {
		case ';', '|', '&', '(', ')':
			last = i + 1
		}
	}
	return last
}

func completion_token_start(line []rune) int {
	start := len(line)
	for start > 0 {
		r := line[start-1]
		if unicode_is_completion_separator(r) {
			break
		}
		start--
	}
	return start
}

func completion_arg_index(line []rune, token_start int) int {
	args := 0
	in_token := false
	for i, r := range line[:token_start] {
		if unicode_is_completion_separator(r) {
			if in_token {
				args++
				in_token = false
			}
			continue
		}
		if !in_token {
			in_token = true
		}
		if i == token_start-1 && in_token {
			args++
		}
	}
	return args
}

func unicode_is_completion_separator(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', ';', '|', '&', '(', ')':
		return true
	default:
		return false
	}
}

func should_complete_paths(token string, arg_index int) bool {
	if token == "" {
		return arg_index > 0
	}
	return arg_index > 0 ||
		strings.HasPrefix(token, ".") ||
		strings.HasPrefix(token, "~") ||
		strings.Contains(token, "/") ||
		strings.Contains(token, "\\") ||
		filepath.IsAbs(token)
}

func (app *shell_app) complete_command_candidates(token string) []string {
	seen := map[string]bool{}
	candidates := make([]string, 0, len(app.builtins)+16)
	for _, name := range shell_builtin_names() {
		if strings.HasPrefix(strings.ToLower(name), strings.ToLower(token)) && !seen[name] {
			seen[name] = true
			candidates = append(candidates, name)
		}
	}
	for name := range app.builtins {
		if strings.HasPrefix(strings.ToLower(name), strings.ToLower(token)) && !seen[name] {
			seen[name] = true
			candidates = append(candidates, name)
		}
	}
	sort.Strings(candidates)
	return candidates
}

func shell_builtin_names() []string {
	return []string{
		"cd",
		"pwd",
		"exit",
		"export",
		"unset",
		"source",
		"builtin",
		"command",
		"echo",
		"printf",
		"test",
		"true",
		"false",
		"complete",
		"compgen",
		"compopt",
	}
}

func (app *shell_app) complete_path_candidates(token string) []string {
	normalized := strings.TrimPrefix(token, "~")
	search_dir := app.cwd
	display_dir := ""
	prefix := normalized
	if last_sep := strings.LastIndexAny(normalized, `/\`); last_sep >= 0 {
		display_dir = normalized[:last_sep+1]
		prefix = normalized[last_sep+1:]
		search_dir = filepath.Join(app.cwd, filepath.FromSlash(strings.ReplaceAll(display_dir, "\\", "/")))
	}
	if filepath.IsAbs(normalized) {
		search_dir = filepath.Dir(normalized)
		display_dir = filepath.Dir(normalized)
		prefix = filepath.Base(normalized)
		if display_dir == "." {
			display_dir = ""
		} else {
			display_dir += string(filepath.Separator)
		}
	}
	entries, err := os.ReadDir(search_dir)
	if err != nil {
		return nil
	}
	candidates := []string{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			continue
		}
		candidate := display_dir + name
		if entry.IsDir() {
			candidate += string(filepath.Separator)
		}
		candidates = append(candidates, filepath.ToSlash(candidate))
	}
	sort.Strings(candidates)
	return candidates
}

func (app *shell_app) run_command_text(command string) error {
	file, err := parse_shell_program("-c", command)
	if err != nil {
		return err
	}
	return app.runner.Run(context.Background(), file)
}

func (app *shell_app) run_script_file(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	parsed, err := parse_shell_reader(path, file)
	if err != nil {
		return err
	}
	return app.runner.Run(context.Background(), parsed)
}

func parse_shell_program(name string, src string) (*syntax.File, error) {
	return syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(src), name)
}

func parse_shell_reader(name string, r io.Reader) (*syntax.File, error) {
	return syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(r, name)
}

func (app *shell_app) exec_program(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return nil
	}
	if interp.IsBuiltin(args[0]) {
		return interp.HandlerCtx(ctx).Builtin(ctx, args)
	}
	if def, ok := app.builtins[args[0]]; ok {
		return app.run_builtin(ctx, def, args)
	}
	return interp.DefaultExecHandler(2*time.Second)(ctx, args)
}

func (app *shell_app) run_builtin(ctx context.Context, def builtin_def, args []string) error {
	hc := interp.HandlerCtx(ctx)
	base := new_core_base(hc.Stdin, hc.Stdout, hc.Stderr, hc.Dir, hc.Env.Get)
	code := def.handler(builtin_context{
		ctx:         ctx,
		app:         app,
		name:        def.name,
		args:        args[1:],
		stdin:       hc.Stdin,
		stdout:      hc.Stdout,
		stderr:      hc.Stderr,
		working_dir: hc.Dir,
		core_base:   base,
	})
	if code == 0 {
		return nil
	}
	return interp.NewExitStatus(uint8(code))
}

func new_core_base(stdin io.Reader, stdout io.Writer, stderr io.Writer, workingDir string, envGet func(string) expand.Variable) urootcore.Base {
	base := urootcore.Base{}
	base.Init()
	base.SetIO(stdin, stdout, stderr)
	base.SetWorkingDir(workingDir)
	base.SetLookupEnv(func(key string) (string, bool) {
		vr := envGet(key)
		if !vr.IsSet() {
			return "", false
		}
		return vr.String(), true
	})
	return base
}

func (b builtin_context) resolve_path(path string) string {
	return b.core_base.ResolvePath(path)
}

func adapt_core_command(builder func() urootcore.Command) builtin_handler {
	return func(b builtin_context) int {
		cmd := builder()
		cmd.SetIO(b.stdin, b.stdout, b.stderr)
		cmd.SetWorkingDir(b.working_dir)
		cmd.SetLookupEnv(b.core_base.LookupEnv)
		if err := cmd.RunContext(b.ctx, b.args...); err != nil {
			fmt.Fprintln(b.stderr, err)
			return 1
		}
		return 0
	}
}

func builtin_cat(b builtin_context) int {
	if len(b.args) == 0 {
		if _, err := io.Copy(b.stdout, b.stdin); err != nil {
			fmt.Fprintln(b.stderr, err)
			return 1
		}
		return 0
	}
	for _, path := range b.args {
		file, err := os.Open(b.resolve_path(path))
		if err != nil {
			fmt.Fprintln(b.stderr, err)
			return 1
		}
		_, copyErr := io.Copy(b.stdout, file)
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil {
			fmt.Fprintln(b.stderr, first_error(copyErr, closeErr))
			return 1
		}
	}
	return 0
}

func builtin_mkdir(b builtin_context) int {
	parents := false
	paths := []string{}
	for _, arg := range b.args {
		if arg == "-p" {
			parents = true
			continue
		}
		paths = append(paths, arg)
	}
	if len(paths) == 0 {
		fmt.Fprintln(b.stderr, "mkdir: missing operand")
		return 1
	}
	for _, path := range paths {
		path = b.resolve_path(path)
		var err error
		if parents {
			err = os.MkdirAll(path, 0o755)
		} else {
			err = os.Mkdir(path, 0o755)
		}
		if err != nil {
			fmt.Fprintln(b.stderr, err)
			return 1
		}
	}
	return 0
}

func builtin_rm(b builtin_context) int {
	recursive := false
	force := false
	paths := []string{}
	for _, arg := range b.args {
		switch arg {
		case "-r", "-R":
			recursive = true
		case "-f":
			force = true
		case "-rf", "-fr":
			recursive = true
			force = true
		default:
			paths = append(paths, arg)
		}
	}
	if len(paths) == 0 {
		fmt.Fprintln(b.stderr, "rm: missing operand")
		return 1
	}
	for _, path := range paths {
		path = b.resolve_path(path)
		info, err := os.Lstat(path)
		if err != nil {
			if force && errors.Is(err, fs.ErrNotExist) {
				continue
			}
			fmt.Fprintln(b.stderr, err)
			return 1
		}
		if info.IsDir() && !recursive {
			fmt.Fprintf(b.stderr, "rm: cannot remove '%s': is a directory\n", path)
			return 1
		}
		if info.IsDir() {
			err = os.RemoveAll(path)
		} else {
			err = os.Remove(path)
		}
		if err != nil {
			fmt.Fprintln(b.stderr, err)
			return 1
		}
	}
	return 0
}

func builtin_touch(b builtin_context) int {
	if len(b.args) == 0 {
		fmt.Fprintln(b.stderr, "touch: missing file operand")
		return 1
	}
	now := time.Now()
	for _, path := range b.args {
		path = b.resolve_path(path)
		file, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0o644)
		if err != nil {
			fmt.Fprintln(b.stderr, err)
			return 1
		}
		if err := file.Close(); err != nil {
			fmt.Fprintln(b.stderr, err)
			return 1
		}
		if err := os.Chtimes(path, now, now); err != nil {
			fmt.Fprintln(b.stderr, err)
			return 1
		}
	}
	return 0
}

func builtin_mv(b builtin_context) int {
	if len(b.args) < 2 {
		fmt.Fprintln(b.stderr, "mv: missing operand")
		return 1
	}
	dest := b.args[len(b.args)-1]
	sources := b.args[:len(b.args)-1]
	dest = b.resolve_path(dest)
	destInfo, destErr := os.Stat(dest)
	destIsDir := destErr == nil && destInfo.IsDir()
	if len(sources) > 1 && !destIsDir {
		fmt.Fprintln(b.stderr, "mv: destination must be a directory when moving multiple files")
		return 1
	}
	for _, source := range sources {
		source = b.resolve_path(source)
		target := dest
		if destIsDir {
			target = filepath.Join(dest, filepath.Base(source))
		}
		if err := os.Rename(source, target); err == nil {
			continue
		}
		if err := copy_path(source, target, true); err != nil {
			fmt.Fprintln(b.stderr, err)
			return 1
		}
		if err := os.RemoveAll(source); err != nil {
			fmt.Fprintln(b.stderr, err)
			return 1
		}
	}
	return 0
}

func builtin_cp(b builtin_context) int {
	recursive := false
	paths := []string{}
	for _, arg := range b.args {
		switch arg {
		case "-r", "-R":
			recursive = true
		default:
			paths = append(paths, arg)
		}
	}
	if len(paths) < 2 {
		fmt.Fprintln(b.stderr, "cp: missing operand")
		return 1
	}
	dest := paths[len(paths)-1]
	sources := paths[:len(paths)-1]
	dest = b.resolve_path(dest)
	destInfo, destErr := os.Stat(dest)
	destIsDir := destErr == nil && destInfo.IsDir()
	if len(sources) > 1 && !destIsDir {
		fmt.Fprintln(b.stderr, "cp: destination must be a directory when copying multiple files")
		return 1
	}
	for _, source := range sources {
		source = b.resolve_path(source)
		target := dest
		if destIsDir {
			target = filepath.Join(dest, filepath.Base(source))
		}
		if err := copy_path(source, target, recursive); err != nil {
			fmt.Fprintln(b.stderr, err)
			return 1
		}
	}
	return 0
}

func builtin_ls(b builtin_context) int {
	showAll := false
	longListing := false
	paths := []string{}
	for _, arg := range b.args {
		switch arg {
		case "-a":
			showAll = true
		case "-l":
			longListing = true
		default:
			paths = append(paths, arg)
		}
	}
	if len(paths) == 0 {
		paths = []string{"."}
	}
	for pathIndex, path := range paths {
		resolvedPath := b.resolve_path(path)
		info, err := os.Stat(resolvedPath)
		if err != nil {
			fmt.Fprintln(b.stderr, err)
			return 1
		}
		if len(paths) > 1 {
			if pathIndex > 0 {
				fmt.Fprintln(b.stdout)
			}
			fmt.Fprintf(b.stdout, "%s:\n", path)
		}
		if !info.IsDir() {
			write_ls_entry(b.stdout, path, info, longListing)
			continue
		}
		entries, err := os.ReadDir(resolvedPath)
		if err != nil {
			fmt.Fprintln(b.stderr, err)
			return 1
		}
		for _, entry := range entries {
			if !showAll && strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			entryInfo, err := entry.Info()
			if err != nil {
				fmt.Fprintln(b.stderr, err)
				return 1
			}
			write_ls_entry(b.stdout, entry.Name(), entryInfo, longListing)
		}
	}
	return 0
}

func write_ls_entry(w io.Writer, name string, info fs.FileInfo, longListing bool) {
	if longListing {
		fmt.Fprintf(w, "%s %12d %s %s\n", info.Mode().String(), info.Size(), info.ModTime().Format("2006-01-02 15:04"), name)
		return
	}
	fmt.Fprintln(w, name)
}

func builtin_find(b builtin_context) int {
	start := "."
	pattern := ""
	args := append([]string{}, b.args...)
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		start = b.resolve_path(args[0])
		args = args[1:]
	}
	for len(args) > 0 {
		if len(args) >= 2 && args[0] == "-name" {
			pattern = args[1]
			args = args[2:]
			continue
		}
		fmt.Fprintf(b.stderr, "find: unsupported argument %s\n", args[0])
		return 1
	}
	walkErr := filepath.WalkDir(start, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if pattern != "" {
			matched, matchErr := filepath.Match(pattern, d.Name())
			if matchErr != nil {
				return matchErr
			}
			if !matched {
				return nil
			}
		}
		fmt.Fprintln(b.stdout, path)
		return nil
	})
	if walkErr != nil {
		fmt.Fprintln(b.stderr, walkErr)
		return 1
	}
	return 0
}

func builtin_head(b builtin_context) int {
	count, files, ok := parse_count_args(b, 10, "head")
	if !ok {
		return 1
	}
	input, closeFn, err := open_optional_input(b, files)
	if err != nil {
		fmt.Fprintln(b.stderr, err)
		return 1
	}
	defer closeFn()
	scanner := bufio.NewScanner(input)
	lines := 0
	for scanner.Scan() {
		fmt.Fprintln(b.stdout, scanner.Text())
		lines++
		if lines >= count {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(b.stderr, err)
		return 1
	}
	return 0
}

func builtin_tail(b builtin_context) int {
	count, files, ok := parse_count_args(b, 10, "tail")
	if !ok {
		return 1
	}
	input, closeFn, err := open_optional_input(b, files)
	if err != nil {
		fmt.Fprintln(b.stderr, err)
		return 1
	}
	defer closeFn()
	scanner := bufio.NewScanner(input)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > count {
			lines = lines[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(b.stderr, err)
		return 1
	}
	for _, line := range lines {
		fmt.Fprintln(b.stdout, line)
	}
	return 0
}

func parse_count_args(b builtin_context, defaultCount int, name string) (int, []string, bool) {
	count := defaultCount
	files := []string{}
	args := append([]string{}, b.args...)
	for len(args) > 0 {
		if len(args) >= 2 && args[0] == "-n" {
			parsed, err := strconv.Atoi(args[1])
			if err != nil || parsed < 0 {
				fmt.Fprintf(b.stderr, "%s: invalid count\n", name)
				return 0, nil, false
			}
			count = parsed
			args = args[2:]
			continue
		}
		files = append(files, args[0])
		args = args[1:]
	}
	if len(files) > 1 {
		fmt.Fprintf(b.stderr, "%s: only one input file is supported in this slice\n", name)
		return 0, nil, false
	}
	return count, files, true
}

func open_optional_input(b builtin_context, files []string) (io.Reader, func(), error) {
	if len(files) == 0 {
		return b.stdin, func() {}, nil
	}
	file, err := os.Open(b.resolve_path(files[0]))
	if err != nil {
		return nil, func() {}, err
	}
	return file, func() { _ = file.Close() }, nil
}

func builtin_mktemp(b builtin_context) int {
	dirMode := false
	template := "goshx-*"
	for _, arg := range b.args {
		if arg == "-d" {
			dirMode = true
			continue
		}
		template = arg
	}
	if dirMode {
		path, err := os.MkdirTemp("", template)
		if err != nil {
			fmt.Fprintln(b.stderr, err)
			return 1
		}
		fmt.Fprintln(b.stdout, path)
		return 0
	}
	file, err := os.CreateTemp("", template)
	if err != nil {
		fmt.Fprintln(b.stderr, err)
		return 1
	}
	if err := file.Close(); err != nil {
		fmt.Fprintln(b.stderr, err)
		return 1
	}
	fmt.Fprintln(b.stdout, file.Name())
	return 0
}

func builtin_base64(b builtin_context) int {
	decode := false
	files := []string{}
	for _, arg := range b.args {
		if arg == "-d" || arg == "--decode" {
			decode = true
			continue
		}
		files = append(files, arg)
	}
	input, closeFn, err := open_optional_input(b, files)
	if err != nil {
		fmt.Fprintln(b.stderr, err)
		return 1
	}
	defer closeFn()
	if decode {
		decoder := base64.NewDecoder(base64.StdEncoding, input)
		if _, err := io.Copy(b.stdout, decoder); err != nil {
			fmt.Fprintln(b.stderr, err)
			return 1
		}
		return 0
	}
	encoder := base64.NewEncoder(base64.StdEncoding, b.stdout)
	if _, err := io.Copy(encoder, input); err != nil {
		fmt.Fprintln(b.stderr, err)
		_ = encoder.Close()
		return 1
	}
	if err := encoder.Close(); err != nil {
		fmt.Fprintln(b.stderr, err)
		return 1
	}
	fmt.Fprintln(b.stdout)
	return 0
}

func builtin_hx(b builtin_context) int {
	return hxlib.Main(append([]string{"hx"}, b.args...), b.stdout, b.stderr)
}

func copy_path(source string, target string, recursive bool) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if !recursive {
			return fmt.Errorf("cp: omitting directory '%s'; use -r", source)
		}
		return copy_dir(source, target)
	}
	return copy_file(source, target, info.Mode())
}

func copy_dir(source string, target string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(target, info.Mode()); err != nil {
		return err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(source, entry.Name())
		dstPath := filepath.Join(target, entry.Name())
		if err := copy_path(srcPath, dstPath, true); err != nil {
			return err
		}
	}
	return nil
}

func copy_file(source string, target string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func first_error(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
