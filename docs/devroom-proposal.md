# Proposal: `devroom` — repo-aware Claude workspace launcher

## What it does

A Go binary (installed to `$GOBIN`) that you run from any repo working copy. It
presents a TUI menu for managing persistent containerised development environments
("rooms"), each tied to a feature branch. It:

1. Detects the repo identity from `git remote` and derives image/container names
2. Builds a base image if one does not exist, using the repo's `build_script`
3. Creates or resumes room containers on demand, with standard credential mounts

## Architecture: two tiers

`devroom` separates the slow part (installing tools) from the per-feature part
(cloning and branching):

| Tier | What it is | Lifecycle |
|---|---|---|
| **Base image** | OS + tools from `build_script` | Built once per project; destroyed explicitly with `X` or `devroom destroy` |
| **Room container** | Clone of repo checked out to the room's branch | Created on first entry; persists across reboots; destroyed explicitly with `R` or `devroom destroy` |

The base image is shared across all rooms in a project. Room containers are
persistent — exiting the shell stops the container but does not delete it;
re-entering resumes it via `<engine> start`.

## Forge detection

`devroom` infers the forge CLI from the origin URL's hostname: `github.com`
maps to `gh`, any other host containing `gitlab` (self-hosted instances
included) maps to `glab`. Any other host is unsupported and fails clearly.

Rather than bind-mounting the host's `~/.config/gh` / `~/.config/glab`
directories (which doesn't work when the CLI's token is backed by the OS
keyring rather than a plaintext file), devroom acquires a token from the
host CLI (`gh auth token` / `glab auth status --show-token`) and pipes it
over a dedicated stdin into a one-shot login of the same CLI inside the
container, then configures git's credential helper to use it for HTTPS
clones. See
[docs/decisions/0000-forge-cli-token-auth-instead-of-ssh-agent-forwarding](decisions/0000-forge-cli-token-auth-instead-of-ssh-agent-forwarding/0000-forge-cli-token-auth-instead-of-ssh-agent-forwarding.md)
for the full rationale.


## Naming

### Base image

Derived from the git remote, shared across all rooms for the project:

```
dev-sfkleach-widgetzilla:base
```

i.e. `dev-<owner>-<repo>:base`. Stored only in the local container engine image
store — nothing is pushed anywhere.

### Room containers

Named from the project and room nickname:

```
devroom-sfkleach-widgetzilla-taskbar-rampage
```

i.e. `devroom-<owner>-<repo>-<nickname>`.

## Configuration

`devroom` follows a three-level configuration hierarchy, with each level
overriding the one above:

| Scope | Path |
|---|---|
| System-wide | `/etc/xdg/devroom/devroom.toml` |
| User-wide | `~/.config/devroom/devroom.toml` |
| Per-repo | `REPOROOT/.config/devroom/devroom.toml` |

The per-repo config may be committed to the repository (shared defaults) or
added to `.gitignore` (personal overrides).

### Configuration keys (v0.1)

| Key | Default | Description |
|---|---|---|
| `runtime` | `podman` | Container engine to use: `podman` or `docker` |
| `base_image` | `ubuntu:24.04` | Base OS image for the generated `Containerfile` |
| `build_script` | `~/.config/devroom/build.sh` | Path to the prerequisite install script, run during `devroom build` |
| `enter_script` | `~/.config/devroom/enter.sh` | Path to a shell snippet sourced during `devroom enter`, before the interactive shell starts |
| `ai_default` | (none) | Which `[[ai]]` entry backs `devroom describe` |

Beyond these scalar keys, one or more `[[ai]]` tables describe the AI CLI
integrations available in every room:

| `[[ai]]` field | Description |
|---|---|
| `name` | Identifier referenced by `ai_default` |
| `enabled` | Installed/mounted into every room unless set to `false` (default: `true`) |
| `install_command` | Run once during `devroom build`, baked into the shared base image |
| `credential_paths` | Host paths bind-mounted (rw) into every room at `enter`/`new` time |
| `describe_command` | Command used inside the container to generate AI descriptions; `{}` is substituted with the description prompt |
| `env` | Host environment variable names forwarded into every room |

### Example config file

`install_command` runs *after* `build_script` (see Build flow below), so it
can rely on a toolchain `build_script` installs — e.g. Claude Code needs
Node/npm, which the base image has no other route to by default. `build_script`
should symlink whatever it installs into `/usr/local/bin` rather than just
exporting `PATH`: that's the one location guaranteed to be on `PATH` for
every invocation style, including the non-interactive `bash -c` execs
`devroom` itself uses (unlike `/etc/profile.d` or shell rc files, which only
apply to login/interactive shells).

```toml
runtime = "podman"
base_image = "ubuntu:22.04"
build_script = "scripts/build.sh"
ai_default = "claude"

[[ai]]
name = "claude"
# build.sh installs Node/npm via fnm and symlinks them into /usr/local/bin
# before this runs.
install_command = "npm install -g --prefix /usr/local @anthropic-ai/claude-code"
# ~/.claude.json is Claude Code's top-level config file (OAuth account,
# project trust state); ~/.claude/ is the separate directory alongside it
# (settings, history, backups). Both are needed.
credential_paths = ["~/.claude", "~/.claude.json"]
describe_command = "claude -p {}"
```

## Room state

Room metadata (nickname → branch mapping, creation time) is stored outside the
repo in:

```
~/.local/share/devroom/<owner>-<repo>/rooms.toml
```

This keeps room state across reboots without polluting the repository.

## Build flow (base image)

The tool generates a `Containerfile` on the fly (no file committed to the repo),
using `base_image` and `build_script` from resolved configuration. The base
image installs tools only — it does not clone the repo.

```dockerfile
FROM <base_image>

ENV DEBIAN_FRONTEND=noninteractive

COPY build.sh /tmp/build.sh
RUN bash /tmp/build.sh

RUN <install_command, for each enabled [[ai]] entry, in config order>
```

Each `[[ai]]` entry's `install_command` runs after `build_script`, not
before — it may depend on a toolchain `build_script` installs (see
Configuration above).

Built with:

```bash
<runtime> build -t dev-<owner>-<repo>:base <tmpdir>
```

The base image is rebuilt if:

- No base image exists yet
- `X` is pressed in the TUI (explicit destroy + rebuild on next entry)
- `devroom destroy` is run, followed by a new room entry
- The resolved `build_script` is newer than the image (checked via
  `<runtime> image inspect` creation time vs file mtime)

## Room container lifecycle

### Shell selection

`devroom` passes the host's `$SHELL` into the container and uses it as the
interactive shell, falling back to `/bin/bash` if the binary is not present in
the container image:

```bash
exec "${SHELL:-/bin/bash}" 2>/dev/null || exec /bin/bash
```

### First entry

```bash
<runtime> run -it --name devroom-<owner>-<repo>-<nickname> \
  -e DEVROOM_SHELL="${SHELL:-/bin/bash}" \
  -v ~/.claude:/root/.claude:ro \
  -v ~/.ssh:/root/.ssh:ro \
  -v ~/.gitconfig:/root/.gitconfig:ro \
  -v <forge-config-dir>:<forge-config-dir>:ro \
  dev-<owner>-<repo>:base \
  bash -c "[ -d /workspace/repo ] || git clone <remote> /workspace/repo && \
           cd /workspace/repo && git checkout <branch> && \
           export PS1='<nickname>% ' && \
           exec \${DEVROOM_SHELL} 2>/dev/null || exec /bin/bash"
```

### Re-entry (container stopped)

```bash
<runtime> start -ai devroom-<owner>-<repo>-<nickname>
```

### Retire room (container deleted, image kept)

```bash
<runtime> rm devroom-<owner>-<repo>-<nickname>
```

## TUI commands

`devroom` opens a single-keypress command loop when invoked from a repo root:

| Key | Action |
|---|---|
| `n` | Create a new room (prompts for nickname and branch) |
| `1`–`9` | Enter the listed room (start or resume its container) |
| `e` | Enter a room by name (for when there are more than 9) |
| `l` | List rooms |
| `d` | Show AI-generated description of each room's activity |
| `c` | Configure devroom interactively |
| `B` | Build (or rebuild) the base image |
| `R` | Retire a room: stop and delete its container (image kept) |
| `X` | Destroy the base image (rebuilt on next room entry) |
| `q` | Quit the TUI (no containers affected) |

### Branch nickname shorthand

When entering a branch name during `n`, `!!` expands to the room nickname:

```
What branch should it use? (taskbar-rampage): add/!!
```

resolves to `add/taskbar-rampage`.

## AI room description (`d`)

Pressing `d` generates a fresh description for each room by exec-ing into the
(running or briefly started) container and running:

```bash
{ git diff main..HEAD; echo "---"; cat CHANGELOG* 2>/dev/null; } \
  | <describe_command, with {} substituted for the description prompt>
```

`<describe_command>` comes from the `[[ai]]` entry named by `ai_default`
(see Configuration above) — e.g. `claude -p {}`. The description is
generated fresh each time rather than cached, so it reflects current branch
state. That entry's `credential_paths` are already mounted into every room,
so no additional API keys are required in `devroom` configuration.

If the room's container is stopped, `devroom` starts it temporarily to
generate the description, then stops it again.

## CLI subcommands

Beyond the TUI, `devroom` supports direct subcommands for scripting and
keyboard-shortcut workflows. All subcommands operate on the repo in the current
working directory.

### Informational flags (root command)

| Flag | Effect |
|---|---|
| `--help`, `-h` | Print usage summary and list of subcommands. |
| `--version`, `-V` | Print the `devroom` version and exit. |

These are also available as subcommands for scripting convenience.

### Subcommands

| Command | Effect |
|---|---|
| `devroom help [subcommand]` | Print usage for the given subcommand, or general help if omitted. |
| `devroom version` | Print the `devroom` version and exit. Useful in scripts. |
| `devroom init` | Create an initial `.config/devroom/devroom.toml`. |
| `devroom build` | Builds or rebuilds the base image, useful for checking the initialisation. |
| `devroom new [--nickname <name>] [--branch <branch>]` | Create a new room. Prompts for any omitted values. |
| `devroom enter <nickname>` | Enter the named room (start or resume its container). |
| `devroom list` | Print all rooms for this repo with their branch and container state. |
| `devroom describe <nickname> [-v...]` | Print the AI-generated description for the named room. Repeat `-v` for more detail. |
| `devroom retire <nickname>` | Stop and delete the named room's container (base image kept). |
| `devroom destroy [-y]` | Stop and delete all room containers for this repo, then delete the base image. Prompts for confirmation unless `-y` is passed. |

## Resolved design decisions

1. **Binary location**: installed to `$GOBIN`; a personal tool, not committed to repos.
2. **Branch-specific rooms**: each room maps to one branch; the base image is untagged by branch.
3. **Persistent containers**: `--rm` is not used; containers survive reboots and are stopped/started across sessions.
4. **Description runs inside the container**: leverages the `ai_default` entry's existing `credential_paths` mount; no separate API key needed.
5. **Config format**: TOML, three-level XDG hierarchy; per-repo file at `REPOROOT/.config/devroom/devroom.toml`.
