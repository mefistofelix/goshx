# goshx

`goshx` is a `bash`-compatible shell in pure `Go` built on top of `mvdan/sh`.
Its goal is to provide a portable single-binary shell with integrated core commands so common workflows do not depend on external Unix utilities being installed.

## Current status

The project currently provides a first working vertical slice:

- `bash`-style command execution through the current `mvdan/sh` development branch
- interactive mode when started without arguments
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
- `cp`
- `find`
- `head`
- `hx`
- `ls`
- `mkdir`
- `mktemp`
- `mv`
- `rm`
- `tail`
- `touch`

## `hx` builtin

The current `hx` builtin is a minimal in-process helper for generic bootstrap workflows.

Supported subcommands:

- `hx get <url> [output]`
- `hx fetch <url> [output]`
- `hx extract <archive> [destination]`
- `hx shasum [-a 1|256|512] <file>`

Supported extraction formats:

- `.zip`
- `.tar`
- `.tar.gz`
- `.tgz`
- `.gz`

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
