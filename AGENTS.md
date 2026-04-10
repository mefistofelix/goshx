# Terms

- `XProject` MUST mean the current project.
- `XDev` MUST mean the developer instructing work on `XProject`.
- `XAgent` MUST mean the AI agent developing `XProject`.
- `XProjectUser` MUST mean the final released-project user.

# Rules

- `XAgent` MUST initialize git if the project is not already a git repository the git user information to use is specified in GIT_USER.txt if empty use 'agent' username.
- `XAgent` MUST keep `.gitignore` present and meaningfully updated for this project.
- `XAgent` MUST create a local git commit on the current branch at the end of every prompt-round if local changes were made.
- `XAgent` MUST explicitly notify `XDev` if something goes wrong.
- `XAgent` MUST be technical and concise in answers and comments to `XDev`.
- `XAgent` MUST answer in the language used by `XDev`.
- `XAgent` MUST keep established programming terms in their natural form when literal translation would be misleading.
- `XAgent` MUST keep code comments, `README.md`, and other project user-facing text in English unless `XDev` explicitly requests otherwise.

- `DESIDERATA.md` MUST be treated as the high-level target feature set and MUST NOT be edited by `XAgent`.
- Before implementing anything derived from `DESIDERATA.md`, `XAgent` MUST compare the logical differences between `DESIDERATA.md` and `README.md`.
- If `README.md` is missing or empty, `XAgent` MUST treat the project as not yet implemented, and the logical difference to implement is the full content of `DESIDERATA.md`.
- Before implementing anything derived from those differences, `XAgent` MUST ask `XDev` for confirmation.
- `README.md` MUST be treated as the end-user-facing description of currently implemented behavior.
- Whenever final user-facing behavior changes, `XAgent` MUST update `README.md` in the best existing section or create a new one, and the update MUST be simple, synthesized, and non-redundant.

- `XAgent` MUST follow `CODE_STYLE.md` for every written or rewritten line of code.
- If `CODE_DESIGN.md` exists and is not empty, `XAgent` MUST follow `CODE_DESIGN.md` for every code-structure decision.
- If `CODE_DESIGN.md` exists and is not empty, it MUST be treated as authoritative for signatures, structs, classes, functions, methods, and structural intent.
- If `CODE_DESIGN.md` exists and is not empty, `XAgent` MUST NOT invent or add structural elements that are not present in `CODE_DESIGN.md`.
- If `CODE_DESIGN.md` exists and is not empty and `XAgent` determines that it cannot be followed, `XAgent` MUST explain why, MUST propose the needed design change to `XDev`, MUST wait for `XDev` to update `CODE_DESIGN.md`, and MUST NOT edit `CODE_DESIGN.md` directly.
- If `CODE_DESIGN.md` is missing or empty, `XAgent` MAY choose the code structure freely while still following all the other project rules.

- `XAgent` MUST keep source files under `src/`.
- `XAgent` MUST keep build outputs under `bin/`.
- `XAgent` MUST keep the self-contained build system in a single script file for each platform `build.sh/bat`.
- `XAgent` MUST keep the main source file as `src/main.<ext>` according to the language, for example `src/main.go`.
- `XAgent` MUST NOT proliferate source files and MUST keep code in the main source file unless `CODE_DESIGN.md` explicitly requires otherwise.
- `XAgent` MUST keep the self-contained test system under in a single script file for each platform `test.sh/bat`.
- Build and test subsystems MUST store cache and ephemeral data in `build_cache/` and `test_cache/` respectively.

- Tooling and build systems MUST be standalone, relocatable on the filesystem, platform-independent when reasonably possible, and cold-bootstrappable.
- Tooling and build systems MUST NOT require manual dependency download, manual extraction, or manual configuration by `XDev` or `XProjectUser`.
- Build and test entrypoints SHOULD be small scripts such as `build.bat`, `build.sh`, or equivalent, able to bootstrap their requirements automatically.
- `XAgent` SHOULD use the latest stable runtimes, compilers, and tools.
- `XAgent` SHOULD prefer Ubuntu for WSL and GitHub Actions.
- Text, script, and source files SHOULD use LF line endings regardless of host OS, unless another format is strictly required.

- `XAgent` MUST keep the test suite updated whenever a feature or code change is significant.
- `XAgent` MUST NOT run tests at every prompt-round by default.
- `XAgent` MUST run tests when something important changed.
- When running tests, `XAgent` MUST run only the tests relevant to the current development OS.
- For CLI projects, tests MUST treat the executable interface as the source of truth and MUST be conceived primarily as CLI black-box tests.
- For interoperability with public registries or services, tests MUST hit real public endpoints and real published artifacts, not simulated local equivalents.
- Such tests SHOULD use meaningful, mainstream, likely-stable examples and SHOULD optimize for low bandwidth and execution time.
- `XAgent` MUST NOT add code, features, flags, environment variables, or behavior only to simplify, fake, or mock tests unless `CODE_DESIGN.md` explicitly requires it.

- `XAgent` MUST keep the CI/workflow pipeline updated so it can create release artifacts for Linux x64 and Windows x64.
- The CI/workflow pipeline MUST reuse the existing build system.
- The CI/workflow pipeline MUST permit manual triggering.
- `XAgent` SHOULD avoid third-party GitHub Actions, including official GitHub Actions, when shell commands or `gh` can reasonably replace them.
- `XAgent` SHOULD prefer Linux GitHub runners and cross-compilation for other targets when practical; otherwise it SHOULD use multiple runners in the same workflow.

- `XAgent` MUST use and manage `gh` for repository, branch, push, workflow, tag, release, and metadata operations when those actions are needed.
- If `gh` authentication is missing or broken, `XAgent` MUST ask `XDev` to complete the authentication flow and MUST provide the URL/code flow needed to initialize the token with the required scopes.

- If the repository has a configured GitHub remote and `gh` is authenticated, then after every significant user-facing or release-relevant change `XAgent` MUST push the current branch.
- If the repository has a configured GitHub remote and `gh` is authenticated, then after every significant user-facing or release-relevant change `XAgent` MUST update the GitHub repository description if the project scope or message changed materially.
- If the repository has a configured GitHub remote and `gh` is authenticated, then after every significant user-facing or release-relevant change `XAgent` MUST create or update the release tag and the corresponding GitHub Release.
- In such cases, `XAgent` MUST state in the final response exactly which push, tag, and release were created.
- Significant user-facing or release-relevant changes MUST include at least new features, new CLI flags, behavior changes visible to `XProjectUser`, support for new platforms, formats, or protocols, and fixes to previously broken advertised behavior.
- `XAgent` MUST NOT treat push, tag, or release as optional when the previous conditions are met.
- If a GitHub remote is missing, `gh` is not authenticated, or push, tag, or release creation fails, `XAgent` MUST explicitly report the exact blocking command or error in the same prompt-round final response.
- If no significant user-facing or release-relevant change happened, `XAgent` MUST NOT create a tag or release.
- Missing a required push, tag, or release when the previous conditions are met MUST be treated as a rule violation.

- If the current development environment is Windows and Linux execution or testing is needed, `XAgent` MUST use WSL Ubuntu.
- If WSL or the Ubuntu distro is missing and Linux execution is needed, `XAgent` MUST install it.
- If privileged work is needed inside WSL, `XAgent` MUST use root first and SHOULD switch to a non-root user afterward when appropriate.

- `XAgent` MUST NOT modify, delete, or rewrite `AGENTS.md`, `CLAUDE.md`, `CODE_STYLE.md`, `CODE_DESIGN.md`, or `DESIDERATA.md` unless `XDev` explicitly requests that exact change.

- for c/c++ try to use the embedded zig clang c/c++ compiler, which is easyli binary distributed and include cross compiling out of the box

- if an msvc compiler or linker is really required install it standalone obviusly inside che build_cache subfolder with
  https://github.com/marler8997/msvcup
  msvcup install msvc autoenv msvc-14.44.17.14 sdk-10.0.22621.7
  and use the vcvars*.bat to add the linker and compiler commands to the path when required

- for rust download and use rustup install it standalone obviusly inside che build_cache subfolder

- once chosen a shell scripting lang for build or test never mix bat sh or powershell,
  if we have a bat file for example don't interlacciate ps1 files, rewrite the stack in bat if che code is our
- dont use complex shell scripts structure follow CODE_STYLE.md also in this case but keep the code linear
  no labels for bat no functions for powershell bash etc, linear code from to to bottom
- you must remember that powershell does not support multiple commands on the same line via cmd1 && cmd2
- when using `functions.shell_command` in this environment, `XAgent` MUST assume the shell is PowerShell and MUST write commands with PowerShell syntax, not `cmd.exe` syntax and not POSIX shell syntax
- in PowerShell `XAgent` MUST NOT use `&&` or `||` as command separators; `XAgent` MUST split commands into separate tool calls or use PowerShell-native control flow only when really needed
- in PowerShell `XAgent` MUST NOT assume quoting, variable expansion, redirection, or path escaping behave like `cmd.exe` or bash; `XAgent` MUST write commands that are valid PowerShell commands as written
- if a command is intended for `cmd.exe` or bash specifically, `XAgent` MUST invoke that shell explicitly instead of assuming PowerShell will interpret it correctly
- the test script should not call build script
- if crosscompilation is possible for the project use only linux runners and reate release artifacts for boh platforms

- build is build only: it MUST bootstrap only the build dependencies it really needs and MUST NOT push, tag, release, test, or call CI-related remote operations
- test is test only: it MUST bootstrap only the test dependencies it really needs, MUST NOT call build, and MUST NOT install build-only dependencies
- by default `build.bat` and `build.sh` MUST build only for the current platform; optional explicit target arguments MAY build other supported platforms when the toolchain supports cross-compilation
- CI is release-artifact publishing only: it SHOULD prefer Linux runners when cross-compilation is supported and SHOULD create both Linux x64 and Windows x64 artifacts there
- CI MUST NOT use third-party GitHub Actions, MUST NOT push commits, branches, or tags, and MUST NOT create extra repository mutations unrelated to uploading release artifacts
- in CI `gh` MUST be used only to upload artifacts to the release of the current tag and, if needed, to download metadata or source of `XProject` itself
- in CI GitHub credentials and tokens SHOULD be passed only to avoid API or download rate limits and MUST NOT be used to add extra publish or push behavior
- release CI MAY be triggered either by tag push or by `workflow_dispatch`; both modes MUST remain available
- `workflow_dispatch` MAY be used by `XAgent` for CI testing when useful, and MAY also be preferred for manual release testing
- `XAgent` MUST choose exactly one CI trigger for a given intended release or test run and MUST NOT start a second trigger while another equivalent run is already active or already sufficient
- if both a tag-triggered run and a manual run exist for the same intended release or the same commit, `XAgent` MUST keep only one and MUST cancel the duplicate
- completly inogre the folder PRIVATE in the project root
- if it's not for debuggint the action/ci dont wait for github actions to finish 

## Code Exploration Policy
Use `cymbal` CLI for code navigation — prefer it over Read, Grep, Glob, or Bash for code exploration.
- **New to a repo?**: `cymbal structure` — entry points, hotspots, central packages. Start here.
- **To understand a symbol**: `cymbal investigate <symbol>` — returns source, callers, impact, or members based on what the symbol is.
- **To understand multiple symbols**: `cymbal investigate Foo Bar Baz` — batch mode, one invocation.
- **To trace an execution path**: `cymbal trace <symbol>` — follows the call graph downward (what does X call, what do those call).
- **To assess change risk**: `cymbal impact <symbol>` — follows the call graph upward (what breaks if X changes).
- Before reading a file: `cymbal outline <file>` or `cymbal show <file:L1-L2>`
- Before searching: `cymbal search <query>` (symbols) or `cymbal search <query> --text` (grep)
- Before exploring structure: `cymbal ls` (tree) or `cymbal ls --stats` (overview)
- To disambiguate: `cymbal show path/to/file.go:SymbolName` or `cymbal investigate file.go:Symbol`
- The index auto-builds on first use — no manual indexing step needed. Queries auto-refresh incrementally.
- All commands support `--json` for structured output.
