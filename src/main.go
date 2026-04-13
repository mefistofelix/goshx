package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	gosedsed "github.com/mefistofelix/gosed/sed"
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

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	urootcore "github.com/u-root/u-root/pkg/core"
	urootchmod "github.com/u-root/u-root/pkg/core/chmod"
	urootfind "github.com/u-root/u-root/pkg/core/find"
	urootgzip "github.com/u-root/u-root/pkg/core/gzip"
	uroothead "github.com/u-root/u-root/pkg/core/head"
	urootln "github.com/u-root/u-root/pkg/core/ln"
	urootshasum "github.com/u-root/u-root/pkg/core/shasum"
	uroottail "github.com/u-root/u-root/pkg/core/tail"
	uroottar "github.com/u-root/u-root/pkg/core/tar"
	uroottee "github.com/u-root/u-root/pkg/core/tee"
	urootuname "github.com/u-root/u-root/pkg/core/uname"
	urootuniq "github.com/u-root/u-root/pkg/core/uniq"
	urootwc "github.com/u-root/u-root/pkg/core/wc"
	urootwget "github.com/u-root/u-root/pkg/core/wget"
	urootxargs "github.com/u-root/u-root/pkg/core/xargs"
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
	history    []string
	history_on bool
	history_fn string
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
	command         string
	script_path     string
	script_args     []string
	interactive     bool
	disable_history bool
	json_mode       bool
	json_payload    string
	json_out_mode   string
}

// json_request is the input schema for --json mode.
type json_request struct {
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Cwd         string            `json:"cwd"`
	Env         map[string]string `json:"env"`
	Stdin       string            `json:"stdin"`
	MergeOutput bool              `json:"merge_output"`
	TimeoutMs   int64             `json:"timeout_ms"`
}

// json_response is the output schema for --json mode.
type json_response struct {
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

type builtin_handler func(builtin_context) int

type shell_prompt struct {
	app                    *shell_app
	input                  textarea.Model
	submitted              string
	submitted_empty        bool
	quit_render            string
	interrupted            bool
	eof                    bool
	history                []string
	history_index          int
	history_draft          string
	history_filter         string
	filtered_history       []string
	filtered_history_index int
	completion_active      bool
	completion_base_value  string
	completion_base_cursor int
	completion_token_start int
	completion_suggestions []string
	completion_index       int
	navigation_anchor      prompt_anchor
	term_height            int
}

type prompt_edge int

const (
	prompt_edge_none prompt_edge = iota
	prompt_edge_start
	prompt_edge_end
)

type prompt_anchor struct {
	edge   prompt_edge
	offset int
}

func main() {
	os.Exit(run_main())
}

func run_main() int {
	opts, err := parse_cli_args(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	app, err := new_shell_app(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if opts.json_mode {
		return app.run_json_mode(opts)
	}
	return app.run(opts)
}

func parse_cli_args(argv []string) (shell_options, error) {
	opts := shell_options{}
	args := argv[1:]
	// consume leading flags
	for len(args) > 0 {
		switch {
		case strings.HasPrefix(args[0], "--json="):
			opts.json_mode = true
			opts.json_payload = strings.TrimPrefix(args[0], "--json=")
		case args[0] == "--json":
			opts.json_mode = true
			if len(args) > 1 && !is_json_cli_flag(args[1]) && opts.json_payload == "" {
				opts.json_payload = args[1]
				args = args[1:]
			}
		case args[0] == "--no-history":
			opts.disable_history = true
		case strings.HasPrefix(args[0], "--json-out-mode="):
			opts.json_out_mode = strings.TrimPrefix(args[0], "--json-out-mode=")
		case args[0] == "--json-out-mode":
			if len(args) < 2 {
				return opts, errors.New("missing value after --json-out-mode")
			}
			opts.json_out_mode = args[1]
			args = args[1:]
		case args[0] == "--json-out-oneline":
			return opts, errors.New("--json-out-oneline has been renamed to --json-out-mode ndjson")
		case args[0] == "--compact":
			return opts, errors.New("--compact has been renamed to --json-out-mode ndjson")
		default:
			goto done
		}
		args = args[1:]
	}
done:
	if opts.json_out_mode != "" && !opts.json_mode {
		return opts, errors.New("--json-out-mode requires --json")
	}
	if opts.json_out_mode != "" && opts.json_out_mode != "pretty" && opts.json_out_mode != "ndjson" {
		return opts, errors.New(`--json-out-mode must be "pretty" or "ndjson"`)
	}
	if opts.json_mode {
		if len(args) > 0 {
			return opts, errors.New("unexpected positional arguments in --json mode")
		}
		return opts, nil
	}
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

func new_shell_app(opts shell_options) (*shell_app, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	history_file, history, history_on, err := resolve_history_state(!opts.disable_history)
	if err != nil {
		return nil, err
	}
	app := &shell_app{
		cwd:        cwd,
		argv0:      filepath.Base(os.Args[0]),
		env:        append([]string{}, os.Environ()...),
		builtins:   map[string]builtin_def{},
		history:    history,
		history_on: history_on,
		history_fn: history_file,
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

func (app *shell_app) run_json_mode(opts shell_options) int {
	json_out_mode := effective_json_output_mode(opts)
	write_resp := func(resp json_response) int {
		var out []byte
		if json_out_mode == "ndjson" {
			out, _ = json.Marshal(resp)
		} else {
			out, _ = json.MarshalIndent(resp, "", "  ")
		}
		fmt.Fprintln(os.Stdout, string(out))
		return resp.ExitCode
	}

	req, err := read_json_request(opts)
	if err != nil {
		return write_resp(json_response{ExitCode: 1, Error: "parsing request: " + err.Error()})
	}
	command_text, err := json_request_command(req)
	if err != nil {
		return write_resp(json_response{ExitCode: 1, Error: err.Error()})
	}

	// apply cwd and env overrides
	if req.Cwd != "" {
		app.cwd = req.Cwd
	}
	for k, v := range req.Env {
		app.env = append(app.env, k+"="+v)
	}

	// wire output capture
	stdout_buf := &bytes.Buffer{}
	stderr_buf := &bytes.Buffer{}
	app.stdout = stdout_buf
	if req.MergeOutput {
		app.stderr = stdout_buf
	} else {
		app.stderr = stderr_buf
	}
	if req.Stdin != "" {
		app.stdin = strings.NewReader(req.Stdin)
	} else {
		app.stdin = strings.NewReader("")
	}

	if err := app.init_runner(nil, false); err != nil {
		return write_resp(json_response{ExitCode: 1, Error: "init runner: " + err.Error()})
	}

	ctx := context.Background()
	var cancel context.CancelFunc
	if req.TimeoutMs > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutMs)*time.Millisecond)
		defer cancel()
	}

	app.append_history(command_text)
	file, parse_err := parse_shell_program("-c", command_text)
	if parse_err != nil {
		return write_resp(json_response{ExitCode: 1, Error: parse_err.Error()})
	}

	start := time.Now()
	run_err := app.runner.Run(ctx, file)
	app.sync_runtime_state()
	duration := time.Since(start)

	resp := json_response{
		Stdout:     stdout_buf.String(),
		Stderr:     stderr_buf.String(),
		DurationMs: duration.Milliseconds(),
	}
	if run_err != nil {
		if status, ok := interp.IsExitStatus(run_err); ok {
			resp.ExitCode = int(status)
		} else {
			resp.ExitCode = 1
			resp.Error = run_err.Error()
		}
	}
	return write_resp(resp)
}

func is_json_cli_flag(arg string) bool {
	return arg == "--no-history" || arg == "--json-out-mode"
}

func effective_json_output_mode(opts shell_options) string {
	if opts.json_out_mode != "" {
		return opts.json_out_mode
	}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		return "pretty"
	}
	return "ndjson"
}

func read_json_request(opts shell_options) (json_request, error) {
	if opts.json_payload != "" {
		req := json_request{}
		if err := json.Unmarshal([]byte(opts.json_payload), &req); err != nil {
			return req, err
		}
		return req, nil
	}
	req := json_request{}
	decoder := json.NewDecoder(os.Stdin)
	if err := decoder.Decode(&req); err != nil {
		return req, err
	}
	return req, nil
}

func json_request_command(req json_request) (string, error) {
	if strings.TrimSpace(req.Command) != "" && len(req.Args) > 0 {
		return "", errors.New(`json request must use either "command" or "args", not both`)
	}
	if len(req.Args) > 0 {
		parts := make([]string, len(req.Args))
		for i, arg := range req.Args {
			quoted, err := syntax.Quote(arg, syntax.LangBash)
			if err != nil {
				return "", err
			}
			parts[i] = quoted
		}
		return strings.Join(parts, " "), nil
	}
	if strings.TrimSpace(req.Command) == "" {
		return "", errors.New(`json request requires "command" or "args"`)
	}
	return req.Command, nil
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
	app.builtins["chmod"] = builtin_def{name: "chmod", usage: "chmod [-R] mode file...", handler: adapt_core_command(func() urootcore.Command { return urootchmod.New() })}
	app.builtins["cp"] = builtin_def{name: "cp", usage: "cp [-r] source... destination", handler: builtin_cp}
	app.builtins["find"] = builtin_def{name: "find", usage: "find [path] [-name pattern]", handler: adapt_core_command(func() urootcore.Command { return urootfind.New() })}
	app.builtins["gzip"] = builtin_def{name: "gzip", usage: "gzip [file...]", handler: adapt_core_command(func() urootcore.Command { return urootgzip.New("gzip") })}
	app.builtins["head"] = builtin_def{name: "head", usage: "head [-n count] [-c bytes] [file...]", handler: adapt_core_command(func() urootcore.Command { return uroothead.New() })}
	app.builtins["hx"] = builtin_def{name: "hx", usage: "hx [flags] <source> [destination]", handler: builtin_hx}
	app.builtins["ln"] = builtin_def{name: "ln", usage: "ln [-svfTiLPr] target... link", handler: adapt_core_command(func() urootcore.Command { return urootln.New() })}
	app.builtins["ls"] = builtin_def{name: "ls", usage: "ls [-a] [-l] [path...]", handler: builtin_ls}
	app.builtins["mkdir"] = builtin_def{name: "mkdir", usage: "mkdir [-p] path...", handler: builtin_mkdir}
	app.builtins["mktemp"] = builtin_def{name: "mktemp", usage: "mktemp [-d] [template]", handler: builtin_mktemp}
	app.builtins["mv"] = builtin_def{name: "mv", usage: "mv source... destination", handler: builtin_mv}
	app.builtins["rm"] = builtin_def{name: "rm", usage: "rm [-r] [-f] path...", handler: builtin_rm}
	app.builtins["sed"] = builtin_def{name: "sed", usage: "sed [options] [script] [file...]", handler: builtin_sed}
	app.builtins["tee"] = builtin_def{name: "tee", usage: "tee [-a] [file...]", handler: adapt_core_command(func() urootcore.Command { return uroottee.New() })}
	app.builtins["uniq"] = builtin_def{name: "uniq", usage: "uniq [-cdui] [input [output]]", handler: adapt_core_command(func() urootcore.Command { return urootuniq.New() })}
	app.builtins["shasum"] = builtin_def{name: "shasum", usage: "shasum [-a 1|256|512] [file...]", handler: adapt_core_command(func() urootcore.Command { return urootshasum.New() })}
	app.builtins["tail"] = builtin_def{name: "tail", usage: "tail [-n count] [file]", handler: adapt_core_command(func() urootcore.Command { return uroottail.New() })}
	app.builtins["tar"] = builtin_def{name: "tar", usage: "tar -c|-x|-t -f file [path]", handler: adapt_core_command(func() urootcore.Command { return uroottar.New() })}
	app.builtins["touch"] = builtin_def{name: "touch", usage: "touch file...", handler: builtin_touch}
	app.builtins["uname"] = builtin_def{name: "uname", usage: "uname [-asnrvmp]", handler: adapt_core_command(func() urootcore.Command { return urootuname.New() })}
	app.builtins["wc"] = builtin_def{name: "wc", usage: "wc [-lwrbc] [file...]", handler: adapt_core_command(func() urootcore.Command { return urootwc.New() })}
	app.builtins["wget"] = builtin_def{name: "wget", usage: "wget [-O file] url", handler: adapt_core_command(func() urootcore.Command { return urootwget.New() })}
	app.builtins["xargs"] = builtin_def{name: "xargs", usage: "xargs [options] [command [args...]]", handler: adapt_core_command(func() urootcore.Command { return urootxargs.New() })}
}

func (app *shell_app) run_interactive() int {
	if stdin_file, ok := app.stdin.(*os.File); ok && term.IsTerminal(int(stdin_file.Fd())) {
		return app.run_interactive_bubbletea(stdin_file)
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
			app.append_history(line)
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

func (app *shell_app) run_interactive_bubbletea(stdin_file *os.File) int {
	for {
		model := new_shell_prompt(app, app.history)
		program := tea.NewProgram(model, tea.WithInput(stdin_file), tea.WithOutput(app.stdout))
		final_model, err := program.Run()
		if err != nil {
			fmt.Fprintln(app.stderr, err)
			return app.run_interactive_plain()
		}
		prompt_model, ok := final_model.(shell_prompt)
		if !ok {
			fmt.Fprintln(app.stderr, "interactive prompt error")
			return 1
		}
		if prompt_model.eof {
			return 0
		}
		if prompt_model.interrupted {
			continue
		}
		line := strings.TrimSpace(prompt_model.submitted)
		if line == "" {
			continue
		}
		app.append_history(prompt_model.submitted)
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

func append_history_entry(history []string, entry string) ([]string, bool) {
	if strings.TrimSpace(entry) == "" {
		return history, false
	}
	if len(history) > 0 && history[len(history)-1] == entry {
		return history, false
	}
	return append(history, entry), true
}

func encode_history_entry(entry string) string {
	return strings.ReplaceAll(entry, "\\\n", "\n")
}

func normalize_history_entry(entry string) string {
	entry = strings.ReplaceAll(entry, "\\\n", "\n")
	return entry
}

func display_history_entry(entry string) string {
	return strings.ReplaceAll(entry, "\n", "\\\n")
}

func unique_history_entries(history []string) []string {
	if len(history) == 0 {
		return []string{}
	}
	seen := map[string]bool{}
	reversed := make([]string, 0, len(history))
	for i := len(history) - 1; i >= 0; i-- {
		entry := history[i]
		if seen[entry] {
			continue
		}
		seen[entry] = true
		reversed = append(reversed, entry)
	}
	unique := make([]string, 0, len(reversed))
	for i := len(reversed) - 1; i >= 0; i-- {
		unique = append(unique, reversed[i])
	}
	return unique
}

func resolve_history_state(enabled bool) (string, []string, bool, error) {
	if !enabled {
		return "", nil, false, nil
	}
	executable_path, err := os.Executable()
	if err != nil {
		return "", nil, false, err
	}
	history_file := filepath.Join(filepath.Dir(executable_path), ".goshx", "history")
	history, err := load_history_file(history_file)
	if err != nil {
		return "", nil, false, err
	}
	return history_file, history, true, nil
}

func load_history_file(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []string{}, nil
		}
		return nil, err
	}
	defer file.Close()
	entries := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		entry, decode_err := strconv.Unquote(line)
		if decode_err != nil {
			entry = line
		}
		entry = normalize_history_entry(entry)
		if strings.TrimSpace(entry) == "" {
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (app *shell_app) append_history(entry string) {
	if strings.TrimSpace(entry) == "" {
		return
	}
	normalized_entry := encode_history_entry(entry)
	var appended bool
	app.history, appended = append_history_entry(app.history, normalized_entry)
	if !appended {
		return
	}
	if !app.history_on || app.history_fn == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(app.history_fn), 0o755); err != nil {
		fmt.Fprintln(app.stderr, err)
		return
	}
	file, err := os.OpenFile(app.history_fn, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(app.stderr, err)
		return
	}
	_, write_err := fmt.Fprintln(file, strconv.Quote(normalized_entry))
	close_err := file.Close()
	if write_err != nil || close_err != nil {
		fmt.Fprintln(app.stderr, first_error(write_err, close_err))
	}
}

func (app *shell_app) prompt() string {
	return filepath.ToSlash(app.cwd) + "$ "
}

func (app *shell_app) continuation_prompt() string {
	return "..> "
}

func new_shell_prompt(app *shell_app, history []string) shell_prompt {
	input := textarea.New()
	input.ShowLineNumbers = false
	input.Prompt = app.prompt()
	input.SetPromptFunc(max_int(rune_len(app.prompt()), rune_len(app.continuation_prompt())), func(lineIdx int) string {
		if lineIdx == 0 {
			return app.prompt()
		}
		return app.continuation_prompt()
	})
	input.SetHeight(1)
	input.SetWidth(80)
	input.Focus()
	unique_history := unique_history_entries(history)
	prompt := shell_prompt{
		app:                    app,
		input:                  input,
		history:                unique_history,
		history_index:          len(unique_history),
		filtered_history_index: -1,
		term_height:            24,
	}
	return prompt
}

func (m shell_prompt) Init() tea.Cmd {
	return textarea.Blink
}

func (m shell_prompt) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.input.SetWidth(max_int(20, msg.Width-1))
		m.term_height = msg.Height
		m.recalc_height()
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.interrupted = true
			m.prepare_quit_render()
			return m, tea.Quit
		case "ctrl+d":
			if m.input.Value() == "" {
				m.eof = true
				m.prepare_quit_render()
				return m, tea.Quit
			}
			return m, nil
		case "esc":
			m.clear_input()
			return m, nil
		case "enter":
			if shell_input_is_complete(m.input.Value()) {
				m.submitted = m.input.Value()
				m.submitted_empty = strings.TrimSpace(m.submitted) == ""
				m.prepare_quit_render()
				return m, tea.Quit
			}
			m.reset_completion()
			m.insert_newline()
			return m, nil
		case "tab":
			m.navigation_anchor = prompt_anchor{}
			m.apply_completion()
			return m, nil
		case "home":
			m.reset_completion()
			m.navigation_anchor = prompt_anchor{edge: prompt_edge_start}
			m.move_cursor_to_prompt_start()
			return m, nil
		case "end":
			m.reset_completion()
			m.navigation_anchor = prompt_anchor{edge: prompt_edge_end}
			m.move_cursor_to_prompt_end()
			return m, nil
		case "up":
			if edge := m.effective_prompt_edge(); edge != prompt_edge_none {
				m.reset_completion()
				m.history_prev(edge)
				m.recalc_height()
			}
			return m, nil
		case "down":
			if edge := m.effective_prompt_edge(); edge != prompt_edge_none {
				m.reset_completion()
				m.history_next(edge)
				m.recalc_height()
			}
			return m, nil
		case "pgup":
			m.reset_completion()
			m.filtered_history_prev(m.current_prompt_anchor())
			m.recalc_height()
			return m, nil
		case "pgdown":
			m.reset_completion()
			m.filtered_history_next(m.current_prompt_anchor())
			m.recalc_height()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.recalc_height()
	if _, ok := msg.(tea.KeyMsg); ok {
		m.reset_filtered_history()
		m.reset_completion()
		m.navigation_anchor = prompt_anchor{}
		m.history_index = len(m.history)
	}
	return m, cmd
}

func (m shell_prompt) View() string {
	if m.submitted != "" || m.submitted_empty || m.interrupted || m.eof {
		return m.quit_render
	}
	return m.input.View()
}

func (m *shell_prompt) recalc_height() {
	value := m.input.Value()
	lines := strings.Split(value, "\n")
	prompt_w := rune_len(m.app.prompt())
	cont_w := rune_len(m.app.continuation_prompt())
	total := 0
	for i, line := range lines {
		avail := m.input.Width() - prompt_w
		if i > 0 {
			avail = m.input.Width() - cont_w
		}
		if avail < 1 {
			avail = 1
		}
		line_len := rune_len(line)
		if line_len == 0 {
			total++
		} else {
			total += (line_len + avail - 1) / avail
		}
	}
	max_h := m.term_height / 2
	if max_h < 4 {
		max_h = 4
	}
	m.input.SetHeight(clamp_int(total, 1, max_h))
}

func (m *shell_prompt) prepare_quit_render() {
	m.quit_render = m.render_quit_prompt_from_value(m.input.Value())
	m.input.SetHeight(1)
}

func (m shell_prompt) render_static_prompt_from_value(value string) string {
	lines := strings.Split(value, "\n")
	if len(lines) == 0 {
		return m.app.prompt()
	}
	rendered := make([]string, 0, len(lines))
	for i, line := range lines {
		prefix := m.prompt_prefix_for_line(i)
		rendered = append(rendered, prefix+line)
	}
	return strings.Join(rendered, "\n")
}

func (m shell_prompt) prompt_prefix_for_line(line_idx int) string {
	prompt := m.app.prompt()
	if line_idx > 0 {
		prompt = m.app.continuation_prompt()
	}
	max_width := max_int(rune_len(m.app.prompt()), rune_len(m.app.continuation_prompt()))
	padding := max_width - rune_len(prompt)
	if padding <= 0 {
		return prompt
	}
	return strings.Repeat(" ", padding) + prompt
}

func (m shell_prompt) render_quit_prompt_from_value(value string) string {
	rendered := m.render_static_prompt_from_value(value)
	return rendered + "\n"
}

func (m *shell_prompt) clear_input() {
	m.input.SetValue("")
	m.recalc_height()
	m.move_cursor_to_prompt_end()
	m.history_index = len(m.history)
	m.history_draft = ""
	m.reset_filtered_history()
}

func (m *shell_prompt) insert_newline() {
	m.input.InsertRune('\n')
	m.recalc_height()
	m.history_index = len(m.history)
	m.reset_filtered_history()
}

func (m *shell_prompt) apply_completion() {
	value := m.input.Value()
	pos := m.cursor_offset()
	if m.completion_active && value == m.render_completion_value() && pos == m.completion_rendered_cursor() {
		if len(m.completion_suggestions) == 0 {
			m.reset_completion()
			return
		}
		m.completion_index = (m.completion_index + 1) % len(m.completion_suggestions)
		m.apply_completion_candidate()
		return
	}
	runes := []rune(value)
	suggestions := m.app.complete_suggestions(runes, pos)
	if len(suggestions) == 0 {
		m.reset_completion()
		return
	}
	segmentStart := completion_segment_start(runes[:pos])
	tokenStart := completion_token_start(runes[segmentStart:pos]) + segmentStart
	m.completion_active = true
	m.completion_base_value = value
	m.completion_base_cursor = pos
	m.completion_token_start = tokenStart
	m.completion_suggestions = suggestions
	m.completion_index = 0
	m.apply_completion_candidate()
}

func (m *shell_prompt) reset_completion() {
	m.completion_active = false
	m.completion_base_value = ""
	m.completion_base_cursor = 0
	m.completion_token_start = 0
	m.completion_suggestions = nil
	m.completion_index = 0
}

func (m shell_prompt) render_completion_value() string {
	if !m.completion_active || len(m.completion_suggestions) == 0 {
		return m.input.Value()
	}
	baseRunes := []rune(m.completion_base_value)
	replacement := []rune(m.rendered_completion_text())
	newRunes := append([]rune{}, baseRunes[:m.completion_token_start]...)
	newRunes = append(newRunes, replacement...)
	newRunes = append(newRunes, baseRunes[m.completion_base_cursor:]...)
	return string(newRunes)
}

func (m shell_prompt) completion_rendered_cursor() int {
	if !m.completion_active || len(m.completion_suggestions) == 0 {
		return m.cursor_offset()
	}
	return m.completion_token_start + rune_len(m.rendered_completion_text())
}

func (m shell_prompt) rendered_completion_text() string {
	if !m.completion_active || len(m.completion_suggestions) == 0 {
		return ""
	}
	text := m.completion_suggestions[m.completion_index]
	if len(m.completion_suggestions) == 1 && !strings.HasSuffix(text, "/") && !strings.HasSuffix(text, "\\") {
		text += " "
	}
	return text
}

func (m *shell_prompt) apply_completion_candidate() {
	value := m.render_completion_value()
	cursor := m.completion_rendered_cursor()
	m.set_value_with_cursor(value, cursor)
	m.history_index = len(m.history)
	m.reset_filtered_history()
}

func (m shell_prompt) cursor_offset() int {
	lines := strings.Split(m.input.Value(), "\n")
	row := clamp_int(m.input.Line(), 0, max_int(len(lines)-1, 0))
	offset := 0
	for i := 0; i < row; i++ {
		offset += rune_len(lines[i]) + 1
	}
	return offset + m.input.LineInfo().CharOffset
}

func (m *shell_prompt) set_value_with_cursor(value string, offset int) {
	m.input.SetValue(value)
	lines := strings.Split(value, "\n")
	targetRow, targetCol := row_col_from_offset(lines, offset)
	for m.input.Line() > targetRow {
		m.input.CursorUp()
	}
	for m.input.Line() < targetRow {
		m.input.CursorDown()
	}
	m.input.SetCursor(targetCol)
}

func (m *shell_prompt) set_value_at_end(value string) {
	m.input.SetValue(value)
	m.move_cursor_to_prompt_end()
}

func (m *shell_prompt) set_value_at_start(value string) {
	m.input.SetValue(value)
	m.move_cursor_to_prompt_start()
}

func (m *shell_prompt) move_cursor_to_prompt_start() {
	for m.input.Line() > 0 {
		m.input.CursorUp()
	}
	m.input.CursorStart()
}

func (m *shell_prompt) move_cursor_to_prompt_end() {
	for m.input.Line() < m.input.LineCount()-1 {
		m.input.CursorDown()
	}
	m.input.CursorEnd()
}

func row_col_from_offset(lines []string, offset int) (int, int) {
	if len(lines) == 0 {
		return 0, 0
	}
	if offset < 0 {
		return 0, 0
	}
	remaining := offset
	for row, line := range lines {
		lineLen := rune_len(line)
		if remaining <= lineLen {
			return row, remaining
		}
		remaining -= lineLen
		if row < len(lines)-1 {
			if remaining == 0 {
				return row + 1, 0
			}
			remaining--
		}
	}
	last := len(lines) - 1
	return last, rune_len(lines[last])
}

func (m shell_prompt) is_at_prompt_start() bool {
	line_info := m.input.LineInfo()
	return m.input.Line() == 0 && line_info.RowOffset == 0 && line_info.CharOffset == 0
}

func (m shell_prompt) is_at_prompt_end() bool {
	line_info := m.input.LineInfo()
	return m.input.Line() == m.input.LineCount()-1 &&
		line_info.RowOffset == max_int(line_info.Height-1, 0) &&
		line_info.CharOffset == line_info.CharWidth
}

func (m shell_prompt) active_prompt_edge() prompt_edge {
	if m.is_at_prompt_start() {
		return prompt_edge_start
	}
	if m.is_at_prompt_end() {
		return prompt_edge_end
	}
	return prompt_edge_none
}

func (m shell_prompt) effective_prompt_edge() prompt_edge {
	if edge := m.active_prompt_edge(); edge != prompt_edge_none {
		return edge
	}
	return m.navigation_anchor.edge
}

func (m shell_prompt) current_prompt_anchor() prompt_anchor {
	anchor := prompt_anchor{edge: m.active_prompt_edge(), offset: m.cursor_offset()}
	if anchor.edge == prompt_edge_none && (m.navigation_anchor.edge != prompt_edge_none || m.navigation_anchor.offset != 0) {
		return m.navigation_anchor
	}
	return anchor
}

func prompt_anchor_from_edge(edge prompt_edge) prompt_anchor {
	return prompt_anchor{edge: edge}
}

func (m *shell_prompt) set_history_value(value string, anchor prompt_anchor) {
	display_value := display_history_entry(value)
	m.navigation_anchor = anchor
	if anchor.edge == prompt_edge_start {
		m.set_value_at_start(display_value)
		return
	}
	if anchor.edge == prompt_edge_end {
		m.set_value_at_end(display_value)
		return
	}
	offset := anchor.offset
	if offset > rune_len(display_value) {
		offset = rune_len(display_value)
	}
	m.set_value_with_cursor(display_value, offset)
}

func (m *shell_prompt) history_prev(edge prompt_edge) {
	m.reset_filtered_history()
	if len(m.history) == 0 {
		return
	}
	if m.history_index == len(m.history) {
		m.history_draft = m.input.Value()
	}
	if m.history_index > 0 {
		m.history_index--
		m.set_history_value(m.history[m.history_index], prompt_anchor_from_edge(edge))
	}
}

func (m *shell_prompt) history_next(edge prompt_edge) {
	m.reset_filtered_history()
	if len(m.history) == 0 {
		return
	}
	if m.history_index >= len(m.history) {
		return
	}
	m.history_index++
	if m.history_index == len(m.history) {
		m.set_history_value(m.history_draft, prompt_anchor_from_edge(edge))
		return
	}
	m.set_history_value(m.history[m.history_index], prompt_anchor_from_edge(edge))
}

func (m *shell_prompt) filtered_history_prev(anchor prompt_anchor) {
	current_anchor := m.current_prompt_anchor()
	filter := string([]rune(m.input.Value())[:current_anchor.offset])
	if m.filtered_history_index >= 0 {
		filter = m.history_filter
	}
	if !m.prepare_filtered_history(filter) {
		return
	}
	if m.filtered_history_index > 0 {
		m.filtered_history_index--
		item := m.filtered_history[m.filtered_history_index]
		m.set_history_value(item, anchor)
	}
}

func (m *shell_prompt) filtered_history_next(anchor prompt_anchor) {
	filter := m.history_filter
	if m.filtered_history_index < 0 {
		current_anchor := m.current_prompt_anchor()
		filter = string([]rune(m.input.Value())[:current_anchor.offset])
	}
	if !m.prepare_filtered_history(filter) {
		return
	}
	if m.filtered_history_index >= len(m.filtered_history) {
		return
	}
	m.filtered_history_index++
	if m.filtered_history_index == len(m.filtered_history) {
		m.set_history_value(m.history_draft, anchor)
		return
	}
	item := m.filtered_history[m.filtered_history_index]
	m.set_history_value(item, anchor)
}

func (m *shell_prompt) prepare_filtered_history(filter string) bool {
	if m.history_filter != filter || m.filtered_history_index < 0 {
		m.history_filter = filter
		m.filtered_history = filtered_history_entries(m.history, filter)
		m.filtered_history_index = len(m.filtered_history)
		m.history_draft = m.input.Value()
	}
	return len(m.filtered_history) > 0
}

func (m *shell_prompt) reset_filtered_history() {
	m.history_filter = ""
	m.filtered_history = nil
	m.filtered_history_index = -1
}

func filtered_history_entries(history []string, filter string) []string {
	matches := []string{}
	lower_filter := strings.ToLower(filter)
	for _, item := range history {
		display_item := display_history_entry(item)
		if filter == "" || strings.HasPrefix(strings.ToLower(display_item), lower_filter) {
			matches = append(matches, item)
		}
	}
	return matches
}

func rune_len(s string) int {
	return len([]rune(s))
}

func max_int(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp_int(value int, low int, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func shell_input_is_complete(src string) bool {
	if strings.TrimSpace(src) == "" {
		return true
	}
	// Check if last non-empty line ends with an unescaped backslash (line continuation).
	lines := strings.Split(strings.TrimRight(src, " \t\n"), "\n")
	lastLine := strings.TrimRight(lines[len(lines)-1], " \t")
	bsCount := 0
	for i := len(lastLine) - 1; i >= 0 && lastLine[i] == '\\'; i-- {
		bsCount++
	}
	if bsCount%2 == 1 {
		return false
	}
	_, err := parse_shell_program("-c", src)
	if err == nil {
		return true
	}
	return !syntax.IsIncomplete(err)
}

func (app *shell_app) complete_suggestions(line []rune, pos int) []string {
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
	return suggestions
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

func is_windows_drive_path(token string) bool {
	return len(token) >= 2 && token[1] == ':'
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
		filepath.IsAbs(token) ||
		is_windows_drive_path(token)
}

func (app *shell_app) complete_command_candidates(token string) []string {
	lower_token := strings.ToLower(token)
	seen := map[string]bool{}
	candidates := make([]string, 0, len(app.builtins)+64)
	windows_exts := app.runtime_path_exts()
	add := func(name string) {
		if strings.HasPrefix(strings.ToLower(name), lower_token) && !seen[name] {
			seen[name] = true
			candidates = append(candidates, name)
		}
	}
	for _, name := range shell_builtin_names() {
		add(name)
	}
	for name := range app.builtins {
		add(name)
	}
	// scan PATH directories for executables
	for _, dir := range filepath.SplitList(app.runtime_env_value("PATH")) {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if runtime.GOOS == "windows" {
				ext := strings.ToLower(filepath.Ext(name))
				if !windows_exts[ext] {
					if ext == "" && app.is_windows {
						add(name)
					}
					continue
				}
				if ext != "" {
					name = name[:len(name)-len(ext)]
				}
			}
			add(name)
		}
	}
	sort.Strings(candidates)
	return candidates
}

func (app *shell_app) runtime_env_value(name string) string {
	if app.runner != nil {
		if value, ok := runtime_var_value(app.runner.Vars, name); ok {
			return value
		}
	}
	if value, ok := runtime_list_env_value(app.env, name); ok {
		return value
	}
	return ""
}

func runtime_var_value(vars map[string]expand.Variable, name string) (string, bool) {
	if len(vars) == 0 {
		return "", false
	}
	if vr, ok := vars[name]; ok && vr.IsSet() && vr.Kind == expand.String {
		return vr.String(), true
	}
	if runtime.GOOS == "windows" {
		upper_name := strings.ToUpper(name)
		for key, vr := range vars {
			if strings.ToUpper(key) == upper_name && vr.IsSet() && vr.Kind == expand.String {
				return vr.String(), true
			}
		}
	}
	return "", false
}

func runtime_list_env_value(env []string, name string) (string, bool) {
	if runtime.GOOS == "windows" {
		upper_name := strings.ToUpper(name)
		for _, entry := range env {
			key, value, ok := strings.Cut(entry, "=")
			if ok && strings.ToUpper(key) == upper_name {
				return value, true
			}
		}
		return "", false
	}
	prefix := name + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return entry[len(prefix):], true
		}
	}
	return "", false
}

func (app *shell_app) runtime_path_exts() map[string]bool {
	exts := map[string]bool{}
	if runtime.GOOS != "windows" {
		return exts
	}
	path_ext := app.runtime_env_value("PATHEXT")
	if path_ext == "" {
		path_ext = ".COM;.EXE;.BAT;.CMD"
	}
	for _, ext := range strings.Split(path_ext, ";") {
		ext = strings.TrimSpace(ext)
		if ext == "" {
			continue
		}
		exts[strings.ToLower(ext)] = true
	}
	return exts
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
	var search_dir, display_prefix, prefix string

	if filepath.IsAbs(normalized) || is_windows_drive_path(normalized) {
		// absolute path or windows drive: split on last separator
		if last_sep := strings.LastIndexAny(normalized, `/\`); last_sep >= 0 {
			display_prefix = normalized[:last_sep+1]
			prefix = normalized[last_sep+1:]
			search_dir = filepath.Clean(display_prefix)
		} else {
			// e.g. "c:" with no separator yet — list the drive root
			search_dir = normalized + string(filepath.Separator)
			display_prefix = search_dir
			prefix = ""
		}
	} else {
		// relative path
		if last_sep := strings.LastIndexAny(normalized, `/\`); last_sep >= 0 {
			display_prefix = normalized[:last_sep+1]
			prefix = normalized[last_sep+1:]
			search_dir = filepath.Join(app.cwd, filepath.FromSlash(strings.ReplaceAll(display_prefix, "\\", "/")))
		} else {
			search_dir = app.cwd
			display_prefix = ""
			prefix = normalized
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
		candidate := display_prefix + name
		if entry.IsDir() {
			candidate += "/"
		}
		candidates = append(candidates, candidate)
	}
	sort.Strings(candidates)
	return candidates
}

func (app *shell_app) run_command_text(command string) error {
	file, err := parse_shell_program("-c", command)
	if err != nil {
		return err
	}
	defer app.sync_runtime_state()
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
	defer app.sync_runtime_state()
	return app.runner.Run(context.Background(), parsed)
}

func (app *shell_app) sync_runtime_state() {
	if app.runner == nil {
		return
	}
	app.cwd = app.runner.Dir
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

func builtin_sed(b builtin_context) int {
	err := gosedsed.Run(b.args, b.stdin, b.stdout, b.stderr)
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(gosedsed.ExitError); ok {
		return exitErr.Code
	}
	fmt.Fprintln(b.stderr, err)
	return 1
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
