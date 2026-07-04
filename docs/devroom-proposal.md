# Proposal: `devroom` — repo-aware Claude workspace launcher

## What it does

A Go binary (installed to `$GOBIN`) that you run from any repo working copy. It
presents a TUI menu for managing persistent containerised development environments
("rooms"), each tied to a feature branch. It:

1. Detects the repo identity from `git remote` and derives image/container names
2. Builds a base image if one does not exist, using the repo's `jumpstart_script`
3. Creates or resumes room containers on demand, with standard credential mounts

## Architecture: two tiers

`devroom` separates the slow part (installing tools) from the per-feature part
(cloning and branching):

| Tier | What it is | Lifecycle |
|---|---|---|
| **Base image** | OS + tools from `jumpstart_script` | Built once per project; destroyed explicitly with `X` or `devroom destroy` |
| **Room container** | Clone of repo checked out to the room's branch | Created on first entry; persists across reboots; destroyed explicitly with `Q` or `devroom destroy` |

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
| `jumpstart_script` | `scripts/jumpstart.sh` | Repo-relative path to the prerequisite install script |
| `summary_model` | `claude -p {}` | Command used inside the container to generate AI summaries |

### Example config file

```toml
runtime = "podman"
base_image = "ubuntu:22.04"
jumpstart_script = "scripts/jumpstart.sh"
summary_model = "claude"
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
using `base_image` and `jumpstart_script` from resolved configuration. The base
image installs tools only — it does not clone the repo.

```dockerfile
FROM <base_image>

ENV DEBIAN_FRONTEND=noninteractive

COPY jumpstart.sh /tmp/jumpstart.sh
RUN bash /tmp/jumpstart.sh
```

Built with:

```bash
<runtime> build -t dev-<owner>-<repo>:base <tmpdir>
```

The base image is rebuilt if:

- No base image exists yet
- `X` is pressed in the TUI (explicit destroy + rebuild on next entry)
- `devroom destroy` is run, followed by a new room entry
- The resolved `jumpstart_script` is newer than the image (checked via
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

### Close room (container deleted, image kept)

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
| `s` | Show AI-generated summary of each room's activity |
| `c` | Configure devroom interactively |
| `Q` | Close a room: stop and delete its container (image kept) |
| `X` | Destroy the base image (rebuilt on next room entry) |
| `q` | Quit the TUI (no containers affected) |

### Branch nickname shorthand

When entering a branch name during `n`, `!!` expands to the room nickname:

```
What branch should it use? (taskbar-rampage): add/!!
```

resolves to `add/taskbar-rampage`.

## AI room summary (`s`)

Pressing `s` generates a fresh summary for each room by exec-ing into the
(running or briefly started) container and running:

```bash
{ git diff main..HEAD; echo "---"; cat CHANGELOG* 2>/dev/null; } \
  | <summary_model> -p "Summarise what this feature branch is implementing."
```

The summary is generated fresh each time rather than cached, so it reflects
current branch state. The `~/.claude` credential mount means no additional API
keys are required in `devroom` configuration.

If the room's container is stopped, `devroom` starts it temporarily to generate
the summary, then stops it again.

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
| `devroom summary [<nickname>]` | Print the AI-generated summary for the named room, or all rooms if no nickname is given. |
| `devroom close <nickname>` | Stop and delete the named room's container (base image kept). |
| `devroom destroy [-y]` | Stop and delete all room containers for this repo, then delete the base image. Prompts for confirmation unless `-y` is passed. |

## Resolved design decisions

1. **Binary location**: installed to `$GOBIN`; a personal tool, not committed to repos.
2. **Branch-specific rooms**: each room maps to one branch; the base image is untagged by branch.
3. **Persistent containers**: `--rm` is not used; containers survive reboots and are stopped/started across sessions.
4. **Summary runs inside the container**: leverages the existing `claude` CLI mount; no separate API key needed.
5. **Config format**: TOML, three-level XDG hierarchy; per-repo file at `REPOROOT/.config/devroom/devroom.toml`.
