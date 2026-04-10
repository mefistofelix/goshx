# goshx

This document describes the desired end state of `goshx`.
It is a target feature set, not a statement of what is already implemented.

## Product goal

`goshx` should be a `bash`-compatible shell implemented in pure `Go`.

Its main goal is to provide a cross-platform shell experience with a single static binary and with fewer external runtime dependencies than a traditional shell environment.

The shell should parse and execute shell language through [`mvdan/sh`](https://github.com/mvdan/sh) and should integrate selected common Unix-style commands as in-process builtins instead of spawning separate executables.

This direction follows the same general idea used by [`go-task/task`](https://github.com/go-task/task) for Windows-friendly core utilities integration, but `goshx` should push that model further by integrating a broader and more coherent builtin command set.

## Core architecture

`goshx` must use:

- [`mvdan/sh`](https://github.com/mvdan/sh) as the shell parser and interpreter
- [`u-root/u-root`](https://github.com/u-root/u-root) as the primary source of embeddable implementations for common Linux-style commands

The interpreter integration should map command execution to native `Go` functions through the execution hooks exposed by `mvdan/sh`.
When a command is supported internally, `goshx` should execute the corresponding `Go` implementation directly inside the current process.

The project should prefer direct in-process execution over external process spawning whenever that is technically sound and behaviorally compatible.

## Functional goals

`goshx` should provide:

- interactive shell usage
- non-interactive script execution
- command execution compatible with common `bash` usage patterns
- internal implementations for a practical subset of common Unix commands
- fallback to external commands only when a builtin implementation is unavailable or intentionally not supported

The shell should aim to be useful both as:

- a daily lightweight command shell
- a scripting/runtime shell for automation
- a portable shell bundled with other tools or agents

## Builtin command strategy

Builtin commands are a primary product feature.

`goshx` should embed and adapt a selected set of commands from `u-root`, especially commands that:

- are commonly used in shell scripts
- are stable enough to expose as part of the default shell experience
- can run meaningfully across Windows and Linux
- benefit from avoiding external process startup

The project must not blindly expose every `u-root` command as-is.
Each command may require adaptation for:

- shell integration
- argument and exit-code behavior
- path handling
- cross-platform filesystem semantics
- stdin/stdout/stderr behavior inside the interpreter runtime

The initial builtin scope should prioritize mainstream utility commands such as:

- file and directory operations
- text inspection and filtering
- environment and process-adjacent helpers
- archive and download helpers when safely portable

The command set should grow incrementally, with compatibility and practical usefulness preferred over raw command count.

## Bash compatibility direction

`goshx` should be `bash`-compatible in the pragmatic sense needed for real scripts and interactive usage, not by reimplementing the GNU `bash` codebase.

Compatibility priorities should include:

- standard shell syntax expected by `mvdan/sh`
- variables and environment handling
- pipelines and redirections
- command substitution
- quoting behavior
- shell control flow commonly used in scripts

Where exact `bash` behavior is not feasible, the project should prefer:

- documented behavior
- predictable behavior
- script portability where practical

## Cross-platform contract

`goshx` must run on at least:

- Windows x64
- Linux x64

The same high-level shell behavior should be available on both platforms.
Platform-specific differences should be minimized and documented when unavoidable.

Particular care should be given to:

- path syntax and normalization
- executable lookup
- filesystem metadata differences
- symlink behavior
- terminal behavior
- line ending handling

## Single-binary distribution

The released product should be a single static binary per target platform whenever technically possible.

`goshx` should be usable without requiring:

- a separate `bash` installation
- a separate coreutils installation
- a separately installed helper runtime for the builtins

This self-contained distribution model is a core product value.

## External command policy

The shell should support external command execution for compatibility, but builtin execution must be preferred for commands that `goshx` explicitly integrates.

The resolution policy should be intentional and documented.
At minimum, the design should define:

- when a builtin wins over an external executable
- whether users can force external execution
- how name conflicts are surfaced
- how command-not-found behavior works

## Performance direction

`goshx` should be optimized for low-overhead command execution in common shell workflows.

Important performance goals include:

- avoiding process spawn for integrated commands
- minimizing runtime dependencies
- keeping startup time low
- keeping interactive command latency low
- preserving streaming behavior for pipelines and I/O-heavy commands

Performance improvements must not come at the cost of surprising shell semantics.

## Integration of `hx`

`goshx` should also integrate the `hx` project as an internal capability for generic download and extraction workflows:

- repository: [`mefistofelix/hx`](https://github.com/mefistofelix/hx)

This integration should make `hx` functionality available without requiring a separate executable.

The intended use cases include:

- downloading remote artifacts
- extracting common archive formats
- combining fetch and extraction steps inside shell scripts
- providing a more portable bootstrap story for automation tasks

The exact surface may be exposed either as:

- one or more dedicated builtin commands
- a builtin helper command namespace
- a shell-facing command whose behavior is powered internally by `hx`

The chosen UX should favor concise scripting and predictable error handling.

## Error and exit behavior

Builtin commands must behave like normal shell commands from the script author's perspective.

This includes:

- meaningful exit codes
- stderr output for failures
- stdout output suitable for piping
- propagation of failure state into shell control flow

Internal implementation details must not leak into the user-facing command contract more than necessary.

## Compatibility and adaptation work

Because `u-root` commands were not all designed as drop-in shell builtins, `goshx` should include an adaptation layer where needed.

That layer may handle:

- argument normalization
- shared I/O wiring
- shell process state integration
- platform-specific wrappers
- consistent error reporting

One explicit product goal is to support more adapted `u-root` commands than the smaller subset demonstrated by `go-task/task`.

## Documentation goals

The project documentation should clearly explain:

- which shell language features are supported
- which commands are built in
- which commands still fall back to external processes
- where behavior intentionally differs from GNU `bash`
- how Windows behavior differs from Linux when it must differ

The builtin command inventory should be treated as part of the product contract.

## Testing direction

Testing should focus on black-box shell behavior.

The test plan should cover:

- interactive-like command execution where practical
- script execution
- builtin vs external command resolution
- pipelines involving builtins
- redirections involving builtins
- cross-platform path behavior
- `hx`-powered download and extraction flows

Tests should validate user-visible shell semantics first, and implementation details second.

## Release direction

Release artifacts should provide:

- a Windows x64 binary
- a Linux x64 binary

The release should emphasize that `goshx` is:

- `Go`-based
- `bash`-compatible
- cross-platform
- single-binary
- builtin-heavy rather than spawn-heavy

## Summary

`goshx` is intended to be a portable `bash`-compatible shell in pure `Go` that combines:

- `mvdan/sh` for parsing and interpretation
- integrated `u-root` command implementations as in-process builtins
- future integrated `hx` capabilities for download and extraction

The result should be a practical shell with lower dependency surface, lower process-spawn overhead, and a stronger single-binary cross-platform story than a traditional shell plus external core utilities stack.
