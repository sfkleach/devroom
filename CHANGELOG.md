# Change Log for Devroom

Following the style in https://keepachangelog.com/en/1.0.0/

## v0.1.0

### Added

- `devroom init` — scaffold `.config/devroom/devroom.toml`, `build.sh`, `enter.sh`, and `leave.sh` for the current repo, pre-populated with commented-out starter commands for common toolchains (C/C++, Node via fnm, Python via uv, Go, Rust).
- `devroom build` — generate a Containerfile and build the shared base image, installing `gh`/`glab` and running an optional `build_script`.
- `devroom new` — create a new room container, optionally checking out a fresh feature branch (`-b`).
- `devroom enter` — enter or resume a room's interactive shell; on first entry, detects the git forge, authenticates, and clones the repo.
- `devroom retire` — stop and remove a room's container.
- Forge detection and authentication for GitHub and GitLab: a token is acquired from `gh`/`glab` on the host and piped into a one-shot login inside the container, instead of forwarding SSH keys/agent.
- Room containers get a matching unprivileged user (same UID/GID/username as the host), `sudo`, and read-write access to `~/.claude` so Claude Code runs as the same identity as the host.
- `build_script` config key — a script run once during `devroom build`, baked into the shared base image (defaults to `~/.config/devroom/build.sh`).
- `enter_script` config key — a shell snippet sourced inside the room just before the interactive shell starts, for prompt tools/aliases (defaults to `~/.config/devroom/enter.sh`).
- `leave_script` config key — a shell snippet run inside the room just after the interactive shell exits, for stopping services cleanly (defaults to `~/.config/devroom/leave.sh`).
- `DEVROOM_NICKNAME` environment variable, available inside every room for use by `enter_script`.
- `devroom configure` — interactively edit this repo's devroom.toml: scalar fields plus full CRUD on `[[ai]]` entries.
- `devroom list` — list this repo's rooms, with optional branch (`-b`), state/size statistics (`-s`), base-image staleness (`-i`), and JSON/Markdown output (`-f`).
- `devroom describe` — print an AI-generated summary of a room's branch/progress via the `ai_default` entry, with `-v` controlling detail level.
- `devroom destroy` — remove the shared base image for a repo (and its rooms, with confirmation unless `-f`/`-k`).
- `devroom version` (and `--version`/`-V`) — print the devroom version.
- The interactive TUI — running bare `devroom` opens a single-keypress menu (`n`/`1`-`9`/`e`/`l`/`d`/`c`/`B`/`R`/`X`/`q`) over every subcommand above.

Known limitations of this first pass are tracked in GH issue #1.
