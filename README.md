# goshx

`goshx` is a `bash`-compatible shell in pure `Go` built on top of `mvdan/sh`.
Its goal is to provide a portable single-binary shell with integrated core commands so common workflows do not depend on external Unix utilities being installed.

## Current status

The project currently provides a first working vertical slice:

- `bash`-style command execution through the current `mvdan/sh` development branch
- interactive mode when started without arguments
- interactive `bubbletea` prompt with multiline editing, backslash line-continuation, caret paste, current-directory prompt, `Tab` completion for builtin commands and filesystem paths, command lookup based on the shell's current `PATH`/`PATHEXT`, and filtered history navigation with `PgUp`/`PgDn`
- interactive history persisted next to the executable under `.goshx/history`, with opt-out via `--no-history`
- `-c` command execution mode
- script file execution mode
- builtin-first command dispatch
- fallback to external commands when no integrated builtin is available
- path resolution for in-process builtins through the current `u-root/pkg/core` base abstraction

## Current builtins

Shell-state builtins are delegated to the shell runtime when available, including:

- `cd`
- `pwd`
- `exit`
- `export`
- `unset`

Integrated in-process builtins currently implemented by `goshx`:

- `base64`
- `cat`
- `chmod`
- `cp`
- `find`
- `gzip`
- `head`
- `hx`
- `ln`
- `ls`
- `mkdir`
- `mktemp`
- `mv`
- `rm`
- `sed`
- `shasum`
- `tail`
- `tar`
- `tee`
- `touch`
- `uniq`
- `wc`
- `wget`
- `xargs`

## `hx` builtin

The current `gzip` builtin is now wired through the upstream `u-root/pkg/core/gzip` command interface.
The current `ln` builtin is wired through the forked `u-root/pkg/core/ln` command interface.
The current `hx` builtin now embeds the real [`hx`](https://github.com/mefistofelix/hx) runtime in-process instead of using the older `goshx`-local helper implementation.

On Windows, `ln -s` now reports an explicit hint to enable Developer Mode or run elevated when symbolic link creation is blocked by OS privilege policy.

This means `goshx` now exposes the same `hx` CLI surface as the upstream project:

```text
hx [flags] <source> [dest]
```

So generic download and extraction workflows run inside the shell process without spawning an external `hx` executable.

## Interactive prompt

When `goshx` detects a real TTY, it uses a `bubbletea`-based prompt that supports:

- multiline shell input
- `Enter` inserts a new line when the shell input is incomplete, including trailing `\` line continuations
- `Tab` cycles through available completions for the current token, and only auto-advances when there is a single completion candidate
- paste at the current caret position
- `Home` moves the caret to the start of the full prompt buffer
- a prompt prefix that shows the current working directory
- `End` moves the caret to the end of the full prompt buffer
- `Up` and `Down` browse unique history entries only when the caret is at the absolute start or end of the prompt buffer
- `PgUp` and `PgDn` browse unique history entries filtered by the prompt prefix up to the current caret position
- history navigation keeps the caret anchored to the same logical edge, or to the same caret offset for filtered browsing, across recalled entries
- `Esc` to clear the current prompt buffer

Interactive history is loaded from `.goshx/history` relative to the `goshx` binary directory and new commands are appended after execution regardless of success or failure. The history file keeps exactly one escaped line per command so multiline input and literal backslashes round-trip correctly.
Empty commands and consecutive duplicate commands are not appended to the persisted history file.
Use `--no-history` to disable history loading and prevent creation of the `.goshx` profile/history directory.

When no real TTY is available, `goshx` keeps using the plain non-interactive line reader fallback.

## JSON mode

`goshx --json` executes a JSON request in-process and writes a JSON response to stdout.
This mode is designed for programmatic use by AI agents and automation pipelines.
By default it uses the same history/profile handling as the normal shell mode; use `--no-history` to disable that.

The request can be supplied in either of these forms:

- `goshx --json '{"command":"echo hello"}'`
- `echo {"command":"echo hello"} | goshx --json`

When reading from `stdin`, `goshx` decodes a single JSON value instead of waiting to read the entire stream.
On Windows, inline JSON arguments depend on shell-specific quoting rules; piping the request to `stdin` is the most portable form.

Request schema:

```json
{
  "command":      "echo hello",
  "args":         ["echo", "hello"],
  "cwd":          "/optional/working/dir",
  "env":          {"KEY": "VALUE"},
  "stdin":        "optional stdin data",
  "merge_output": false,
  "timeout_ms":   0
}
```

Use either `command` or `args`. When `args` is present, each element is shell-quoted and executed as a single command invocation.

Response schema:

```json
{
  "exit_code":   0,
  "stdout":      "hello\n",
  "stderr":      "",
  "duration_ms": 12,
  "error":       ""
}
```

Output mode is selected with `--json-out-mode pretty|ndjson`.
If no mode is specified, `goshx` defaults to `pretty` when stdout is a TTY and to `ndjson` when stdout is not a TTY.
The process exits with the same code as the executed command.

## Architecture

`goshx` is built on three main dependencies:

- [`mvdan/sh`](https://github.com/mvdan/sh) — shell parser and interpreter
- [`mefistofelix/u-root`](https://github.com/mefistofelix/u-root) — fork of `u-root` with a `pkg/core` adapter layer for in-process builtin integration
- [`mefistofelix/hx`](https://github.com/mefistofelix/hx) — download and extraction integrated as a builtin

The `u-root` fork adds a `pkg/core` package that defines a `Command` interface (`SetIO`, `SetWorkingDir`, `Run`, `RunContext`) used to wire each builtin into the shell's stdin/stdout/stderr and working directory without spawning a subprocess.

## Build

Windows:

```bat
build.bat
```

Linux:

```sh
./build.sh
```

Optional explicit targets:

- `linux-amd64`
- `windows-amd64`

By default each build script builds only for the current platform.
The bootstrap currently targets `Go 1.26.2`, which was the latest stable release when this update was made.

## Test

Windows:

```bat
test.bat
```

Linux:

```sh
./test.sh
```

The current test suite is CLI black-box oriented and validates shell execution and builtin behavior through the produced executable.
