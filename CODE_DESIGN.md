# Code Design

Implement the project in `Go`.

Keep the main source file as `src/main.go`.

Keep the source structure minimal.
Do not split code into additional source files unless the current file becomes clearly unreadable or a later design revision explicitly requires it.

## Design goals

The code structure must optimize for:

- a single self-contained binary
- `bash`-compatible execution through `mvdan/sh`
- in-process builtin command execution
- explicit adaptation of selected `u-root` commands
- future integration of `hx` without introducing a second process model
- low conceptual overhead

The project is a shell first, not a command collection first.
The command integration layer exists to serve shell execution.

## Dependency direction

The implementation should prefer the `Go` stdlib wherever possible.

Expected core dependencies:

- `mvdan.cc/sh/v3/syntax`
- `mvdan.cc/sh/v3/interp`
- `mvdan.cc/sh/v3/expand`
- selected packages or adapted code paths from `github.com/u-root/u-root`
- internal integration of `hx` from `https://github.com/mefistofelix/hx`

Expected stdlib-heavy areas:

- `context`
- `os`
- `io`
- `fmt`
- `path/filepath`
- `runtime`
- `strings`
- `errors`
- `time`
- `os/exec`
- `bufio`
- `net/url`

The implementation must not depend on a large shell framework beyond `mvdan/sh`.

## File layout

The minimal expected project layout is:

- `src/main.go`
- `build.bat`
- `build.sh`
- `test.bat`
- `test.sh`

If helper assets or generated files become necessary later, they must remain subordinate to this minimal layout and must not introduce an unnecessarily fragmented source tree.

## Architectural overview

The implementation should be organized around these logical areas inside `src/main.go`:

- CLI entry and mode selection
- shell runtime state
- builtin registry
- builtin adapters
- external command fallback
- interactive loop
- script execution path
- common error and exit handling

The shell runtime must be centered on one explicit runtime object rather than many loosely related globals.

## Primary structs

The implementation should define these primary structs.

### `type shell_app struct`

This is the top-level runtime container.

Expected fields:

- `cwd string`
- `argv0 string`
- `env []string`
- `builtins map[string]builtin_def`
- `runner *interp.Runner`
- `stdin io.Reader`
- `stdout io.Writer`
- `stderr io.Writer`
- `is_windows bool`

Purpose:

- hold process-level shell state
- own builtin registration
- own interpreter configuration
- route command execution

### `type builtin_def struct`

Expected fields:

- `name string`
- `usage string`
- `handler builtin_handler`
- `supports_pipeline bool`
- `is_pure bool`

Purpose:

- describe a builtin command
- register adapter metadata centrally
- keep help and capability flags near the handler

### `type builtin_context struct`

Expected fields:

- `ctx context.Context`
- `app *shell_app`
- `name string`
- `args []string`
- `stdin io.Reader`
- `stdout io.Writer`
- `stderr io.Writer`

Purpose:

- avoid passing many loose parameters
- expose shell runtime state to builtin handlers
- keep I/O wiring explicit

### `type shell_options struct`

Expected fields:

- `command string`
- `script_path string`
- `script_args []string`
- `interactive bool`
- `login bool`

Purpose:

- represent the selected process startup mode
- keep argument parsing separate from execution

### `type command_resolution struct`

Expected fields:

- `name string`
- `args []string`
- `builtin *builtin_def`
- `external_path string`

Purpose:

- make command dispatch explicit
- keep builtin-vs-external resolution testable

## Primary function signatures

These functions should form the main stable shape of the implementation.

### process entry

```go
func main()
func run_main() int
func parse_cli_args(argv []string) (shell_options, error)
```

`main` should be tiny.
`run_main` should centralize exit handling and return the shell exit code.

### app construction

```go
func new_shell_app(opts shell_options) (*shell_app, error)
func (app *shell_app) init_runner() error
func (app *shell_app) register_builtins()
```

`new_shell_app` should prepare runtime state and call initialization steps in a linear order.

### shell execution

```go
func (app *shell_app) run(opts shell_options) int
func (app *shell_app) run_interactive() int
func (app *shell_app) run_command_text(command string) error
func (app *shell_app) run_script_file(path string, args []string) error
func (app *shell_app) exec_program(ctx context.Context, args []string) error
```

`exec_program` is the key command dispatch hook used by the interpreter.
It must resolve builtins first and fall back to external execution second.

### parsing and evaluation helpers

```go
func parse_shell_program(name string, src string) (*syntax.File, error)
func parse_shell_reader(name string, r io.Reader) (*syntax.File, error)
```

These helpers keep syntax parsing explicit and reusable.

### command resolution

```go
func (app *shell_app) resolve_command(args []string) (command_resolution, error)
func (app *shell_app) lookup_builtin(name string) (*builtin_def, bool)
func (app *shell_app) lookup_external(name string) (string, bool)
```

Resolution order must be explicit and stable.

### builtin invocation

```go
type builtin_handler func(builtin_context) int

func (app *shell_app) run_builtin(ctx context.Context, def builtin_def, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error
func builtin_exit_error(code int) error
```

Builtin handlers should return shell-style exit codes.
The adapter layer should convert those codes into the interpreter-facing error flow.

### external execution

```go
func (app *shell_app) run_external(ctx context.Context, path string, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error
func (app *shell_app) build_exec_cmd(ctx context.Context, path string, args []string) *exec.Cmd
```

External execution is a compatibility fallback, not the primary execution path.

### interactive shell helpers

```go
func (app *shell_app) read_eval_print_loop(r *bufio.Reader) int
func (app *shell_app) prompt() string
```

The interactive path should remain simple and line-oriented unless later requirements force a richer terminal layer.

### builtin adapter helpers

```go
func adapt_u_root_cmd(run func() error) builtin_handler
func adapt_u_root_main(run func(args []string) int) builtin_handler
func adapt_hx_cmd(run func(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int) builtin_handler
```

These adapters are intentionally generic.
They exist so integrated commands can share shell I/O, exit behavior, and future platform-specific normalization.

### environment and path utilities

```go
func (app *shell_app) get_env(key string) string
func (app *shell_app) set_env(key string, value string) error
func normalize_exec_path(base_dir string, value string) string
func normalize_user_path(base_dir string, value string) string
func split_command_name(args []string) (string, []string, error)
```

These utilities should remain small and directly tied to shell behavior.

## Builtin inventory shape

The initial design must support these builtin groups.

### shell-native builtins

These should be implemented directly in `goshx` because they modify shell state or are simpler to own directly:

- `cd`
- `pwd`
- `exit`
- `export`
- `unset`
- `alias` only if later required

Expected signatures:

```go
func builtin_cd(b builtin_context) int
func builtin_pwd(b builtin_context) int
func builtin_exit(b builtin_context) int
func builtin_export(b builtin_context) int
func builtin_unset(b builtin_context) int
```

### adapted `u-root` builtins

These should be registered through adapters and expanded incrementally.

The first expected wave should focus on broadly useful commands such as:

- `cp`
- `mv`
- `rm`
- `mkdir`
- `rmdir`
- `ls`
- `cat`
- `touch`
- `chmod` where sensible cross-platform behavior exists
- `grep`
- `head`
- `tail`
- `find`
- `echo` if builtin behavior is better kept internal

Not every command must be implemented immediately, but the registry and adapter design must make this list straightforward to grow.

### `hx`-powered builtins

The design should reserve builtin names for `hx` integration early so the command surface stays coherent.

The preferred initial shape is:

- `hx`
- optionally `fetch`
- optionally `extract`

The minimal stable expectation is at least:

```go
func builtin_hx(b builtin_context) int
```

If `fetch` and `extract` are later split out, they should still delegate to the same `hx` integration layer rather than introducing duplicate logic.

## Interpreter integration contract

The `mvdan/sh` interpreter setup must route command execution through `goshx` command resolution.

The design expectation is:

- shell syntax parsing is handled by `mvdan/sh`
- shell evaluation is handled by `mvdan/sh`
- simple command execution is intercepted by `goshx`
- `goshx` decides builtin vs external execution
- builtin handlers run in-process with explicit stream wiring

Shell state that must remain process-local, such as current directory and exported environment, must be owned by `shell_app`, not by ad-hoc helper functions.

## Error model

Error handling should be centralized and minimal.

The expected pattern is:

- functions return `error` for transportable failure information
- builtin handlers return integer exit codes
- top-level process exit conversion happens in one place
- human-readable diagnostics are written consistently to `stderr`

Avoid spreading custom error structs unless a real need appears.

## Cross-platform policy

Cross-platform behavior must be handled intentionally in these areas:

- command lookup extensions on Windows
- path separators and normalization
- executable discovery through `PATH`
- unsupported POSIX-only flags or semantics inside adapted commands

Where `u-root` behavior is Linux-biased, the adapter layer should normalize or reject behavior explicitly rather than silently diverging.

## Testing-oriented design constraints

The structure must stay testable from the CLI surface.

This means:

- `parse_cli_args` must be deterministic
- command resolution must be separable from actual execution
- builtin dispatch must be explicit
- shell exit codes must be easy to assert

Internal helpers may exist, but the external shell behavior remains the source of truth.

## Implementation sequence

The intended implementation order is:

1. CLI argument parsing
2. `shell_app` construction
3. `mvdan/sh` interpreter wiring
4. shell-native builtins
5. external command fallback
6. first adapted `u-root` builtins
7. interactive loop
8. `hx` integration
9. incremental builtin expansion

This order keeps the project usable early while preserving the final architecture.
