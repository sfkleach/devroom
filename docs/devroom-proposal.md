# Proposal: `devroom` — repo-aware Claude workspace launcher

## What it does

A single script (e.g. `~/.local/bin/devroom`) that you run from any repo working copy. It:

1. Detects the repo identity and derives a local image name
2. Builds the image if it doesn't exist, using the repo's `scripts/jumpstart.sh`
3. Launches the container with the standard credential mounts

## Image naming

Derived from the git remote so the same image is reused across sessions:

```
dev-sfkleach-fuselage:latest
```

i.e. `dev-<owner>-<repo>:<branch-or-latest>`. Stored only in the local podman image store — nothing pushed anywhere.

## Configuration

`devroom` follows a three-level configuration hierarchy, with each level overriding the one above:

| Scope | Path |
|---|---|
| System-wide | `/etc/xdg/devroom/config` |
| User-wide | `~/.config/devroom/config` |
| Per-repo | `REPOROOT/.config/devroom/config` |

The per-repo path mirrors the user-wide path, treating the repo root as structurally analogous to `$HOME`. Per-repo config may be committed to the repository (shared defaults) or added to `.gitignore` (personal overrides).

### Configuration keys (v0.1)

| Key | Default | Description |
|---|---|---|
| `runtime` | `podman` | Container runtime to use: `podman` or `docker` |
| `base_image` | `ubuntu:24.04` | Base image for the generated `Containerfile` |
| `jumpstart_script` | `scripts/jumpstart.sh` | Repo-relative path to the pre-requisite install script |

### Example config file

```toml
runtime = "docker"
base_image = "ubuntu:24.04"
jumpstart_script = "scripts/jumpstart.sh"
```

## Build flow

The tool generates a `Containerfile` on the fly (no file committed to the repo), using the
`base_image` and `jumpstart_script` from resolved configuration:

```dockerfile
FROM <base_image>

# Prevent apt from prompting during jumpstart.sh
ENV DEBIAN_FRONTEND=noninteractive

# Copy and run the repo's jumpstart script to install all pre-reqs
COPY <jumpstart_script> /tmp/jumpstart.sh
RUN bash /tmp/jumpstart.sh

# Clone the repo fresh so the container has a clean working copy
ARG REPO_URL
RUN git clone $REPO_URL /workspace/repo

WORKDIR /workspace/repo

ENTRYPOINT ["bash", "-c", "git pull && exec bash"]
```

Built with:

```bash
<runtime> build --build-arg REPO_URL=<remote> -t dev-sfkleach-fuselage:latest .
```

## Launch command

```bash
<runtime> run -it --rm \
  -v ~/.claude:/root/.claude:ro \
  -v ~/.ssh:/root/.ssh:ro \
  -v ~/.gitconfig:/root/.gitconfig:ro \
  -v ~/.config/gh:/root/.config/gh:ro \
  dev-sfkleach-fuselage:latest
```

## Rebuild trigger

The tool rebuilds the image if:

- No image exists yet
- `--rebuild` flag is passed explicitly
- The resolved `jumpstart_script` is newer than the image (checked via `<runtime> image inspect`
  creation time vs file mtime)

## Sketch of the script

```bash
#!/usr/bin/env bash
set -euo pipefail

# Find repo root
REPO_ROOT=$(git rev-parse --show-toplevel)

# Load configuration (system < user < per-repo)
load_config() {
    local key="$1" default="$2"
    local val="$default"
    for cfg in \
        "/etc/xdg/devroom/config" \
        "$HOME/.config/devroom/config" \
        "$REPO_ROOT/.config/devroom/config"
    do
        [[ -f "$cfg" ]] && val=$(grep "^${key}\s*=" "$cfg" | tail -1 | sed 's/.*=\s*"\?\([^"]*\)"\?/\1/')
    done
    echo "$val"
}

RUNTIME=$(load_config runtime podman)
BASE_IMAGE=$(load_config base_image ubuntu:24.04)
JUMPSTART_REL=$(load_config jumpstart_script scripts/jumpstart.sh)
JUMPSTART="$REPO_ROOT/$JUMPSTART_REL"

# Derive image name from remote
REMOTE_URL=$(git remote get-url origin)
OWNER=$(echo "$REMOTE_URL" | sed 's|.*[:/]\([^/]*\)/[^/]*|\1|')
REPO=$(basename -s .git "$REMOTE_URL")
IMAGE="dev-$OWNER-$REPO:latest"

# Build if needed
NEEDS_BUILD=false
if ! $RUNTIME image exists "$IMAGE"; then
    NEEDS_BUILD=true
elif [[ "$JUMPSTART" -nt $($RUNTIME image inspect "$IMAGE" --format '{{.Created}}') ]]; then
    echo "jumpstart.sh is newer than image — rebuilding"
    NEEDS_BUILD=true
fi
[[ "${1:-}" == "--rebuild" ]] && NEEDS_BUILD=true

if $NEEDS_BUILD; then
    echo "==> Building $IMAGE ..."
    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT
    cp "$JUMPSTART" "$TMPDIR/jumpstart.sh"
    # write Containerfile into TMPDIR using $BASE_IMAGE, build from there
    ...
fi

# Launch
exec $RUNTIME run -it --rm \
  -v ~/.claude:/root/.claude:ro \
  -v ~/.ssh:/root/.ssh:ro \
  -v ~/.gitconfig:/root/.gitconfig:ro \
  -v ~/.config/gh:/root/.config/gh:ro \
  "$IMAGE"
```

## Open questions before implementing

1. **Where does `devroom` live?** In `~/.local/bin` (personal tool) or committed to each
   repo as `scripts/devroom.sh`? The former works across all repos; the latter keeps it
   version-controlled with the project.
2. **Branch-specific images?** Tag by branch (`dev-sfkleach-fuselage:main`) or always
   `latest`?
3. **Should it pass the current branch** into the container so `git pull` checks out the
   right branch?
