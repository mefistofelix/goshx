# goshx

`goshx` is a `bash`-compatible shell in pure `Go` built on top of `mvdan/sh`.
Its goal is to provide a portable single-binary shell with integrated core commands so common workflows do not depend on external Unix utilities being installed.

## Current status

The project currently provides a first working vertical slice:

- `bash`-style command execution through the current `mvdan/sh` development branch
- interactive mode when started without arguments
- interactive `bubbletea` prompt with multiline editing, backslash line-continuation, caret paste, current-directory prompt, `Tab` completion for builtin commands and filesystem paths, and filtered history navigation with `PgUp`/`PgDn`
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
- `cut`
- `date`
- `find`
- `grep`
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
- `sleep`
- `sort`
- `tail`
- `tar`
- `tee`
- `touch`
- `tr`
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
- paste at the current caret position
- a prompt prefix that shows the current working directory
- `Up` and `Down` history browsing when the caret is on the first or last logical input line
- `PgUp` and `PgDn` history browsing filtered by the current prompt text when the caret is on the first or last logical input line
- `Esc` to clear the current prompt buffer

Interactive history is loaded from `.goshx/history` relative to the `goshx` binary directory and new commands are appended after execution regardless of success or failure.
Use `--no-history` to disable history loading and prevent creation of the `.goshx` profile/history directory.

When no real TTY is available, `goshx` keeps using the plain non-interactive line reader fallback.

## JSON mode

`goshx --json` reads a JSON request from stdin, executes the command in-process, and writes a JSON response to stdout.
This mode is designed for programmatic use by AI agents and automation pipelines.

Request schema:

```json
{
  "command":      "echo hello",
  "cwd":          "/optional/working/dir",
  "env":          {"KEY": "VALUE"},
  "stdin":        "optional stdin data",
  "merge_output": false,
  "timeout_ms":   0
}
```

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

Output is pretty-printed by default. Use `--compact` to get single-line JSON.
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
