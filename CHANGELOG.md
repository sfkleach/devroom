# Change Log for Devroom

Following the style in https://keepachangelog.com/en/1.0.0/

## Unreleased

### Added

- `devroom init` — scaffold a `.config/devroom/devroom.toml` for the current repo.
- `devroom build` — generate a Containerfile and build the shared base image, installing `gh`/`glab` and running an optional `build_script`.
- `devroom new` — create a new room container, optionally checking out a fresh feature branch (`-b`).
- `devroom enter` — enter or resume a room's interactive shell; on first entry, detects the git forge, authenticates, and clones the repo.
- `devroom close` — stop and remove a room's container.
- Forge detection and authentication for GitHub and GitLab: a token is acquired from `gh`/`glab` on the host and piped into a one-shot login inside the container, instead of forwarding SSH keys/agent.
- Room containers get a matching unprivileged user (same UID/GID/username as the host), `sudo`, and read-write access to `~/.claude` so Claude Code runs as the same identity as the host.
- `build_script` config key — a script run once during `devroom build`, baked into the shared base image (defaults to `~/.config/devroom/build.sh`).
- `enter_script` config key — a shell snippet sourced inside the room just before the interactive shell starts, for prompt tools/aliases (defaults to `~/.config/devroom/enter.sh`).
- `DEVROOM_NICKNAME` environment variable, available inside every room for use by `enter_script`.

Known limitations of this first pass are tracked in GH issue #1.
