# `devroom init`

## Purpose
Scaffold a per-repo `.config/devroom/devroom.toml` plus starter
`build.sh`/`enter.sh`/`leave.sh` scripts, so a new repo has a working
config to edit rather than starting from a blank file.

## Usage
```
devroom init
```
No flags beyond the global ones (see [cli-basics.md](cli-basics.md)). Takes
no arguments.

## Behavior
1. Resolve root via `effectiveRootDir()`.
2. If root doesn't look like a git repo (no `.git`), warn to stderr and
   `confirmYN("Create configuration anyway?", false)` — default answer is
   No; declining aborts with "Aborted." and no error.
3. If `<root>/.config/devroom/devroom.toml` already exists, print that it
   exists and exit cleanly (not an error) without touching anything.
4. Detect the runtime via `detectRuntime()`: prefers whichever of
   `docker`/`podman` is found in `PATH`; if both are present, defaults to
   `docker` and prints a note explaining the choice.
5. Write `devroom.toml` (see `buildInitConfig`) with:
   - `runtime` set to the detected value
   - `base_image = "ubuntu:latest"`
   - `build_script`, `enter_script`, `leave_script` all pointing at the
     files created in step 6, each with a comment explaining how to opt out
     (delete the file and the config line)
   - `ai_default = "claude"` plus one `[[ai]]` block for Claude Code
     (install via npm, mounts `~/.claude` and `~/.claude.json`,
     `describe_command = "claude -p {}"`)
6. Write `build.sh` and `enter.sh` and `leave.sh` under the same
   `.config/devroom/` directory via `writeIfAbsent` — each is only written
   if not already present; an existing file is left untouched and reported,
   never overwritten. Templates are pre-populated with commented-out
   starter snippets for common toolchains (C/C++, Node via fnm, Python via
   uv, Go, Rust) matching what `build_script`/`enter_script`/`leave_script`
   are for.

## Configuration
Writes `devroom.toml`; does not read any pre-existing config (a fresh
`init` doesn't merge with system/user-level files — see
[configuration.md](configuration.md) for how those interact with what gets
written here at repo level).

## Related specs
- [configuration.md](configuration.md) — the schema being scaffolded.
- [build.md](build.md), [enter.md](enter.md) — consumers of `build_script`/`enter_script`/`leave_script`.
