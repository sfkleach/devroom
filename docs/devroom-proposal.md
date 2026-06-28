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

## Build flow

The tool generates a `Containerfile` on the fly (no file committed to the repo):

```dockerfile
FROM ubuntu:24.04

# Prevent apt from prompting during jumpstart.sh
ENV DEBIAN_FRONTEND=noninteractive

# Copy and run the repo's jumpstart script to install all pre-reqs
COPY scripts/jumpstart.sh /tmp/jumpstart.sh
RUN bash /tmp/jumpstart.sh

# Clone the repo fresh so the container has a clean working copy
ARG REPO_URL
RUN git clone $REPO_URL /workspace/repo

WORKDIR /workspace/repo

ENTRYPOINT ["bash", "-c", "git pull && exec bash"]
```

Built with:

```bash
podman build --build-arg REPO_URL=<remote> -t dev-sfkleach-fuselage:latest .
```

## Launch command

```bash
podman run -it --rm \
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
- `scripts/jumpstart.sh` is newer than the image (checked via `podman image inspect`
  creation time vs file mtime)

## Sketch of the script

```bash
#!/usr/bin/env bash
set -euo pipefail

# Find repo root
REPO_ROOT=$(git rev-parse --show-toplevel)
JUMPSTART="$REPO_ROOT/scripts/jumpstart.sh"

# Derive image name from remote
REMOTE_URL=$(git remote get-url origin)
OWNER=$(echo "$REMOTE_URL" | sed 's|.*[:/]\([^/]*\)/[^/]*|\1|')
REPO=$(basename -s .git "$REMOTE_URL")
IMAGE="dev-$OWNER-$REPO:latest"

# Build if needed
NEEDS_BUILD=false
if ! podman image exists "$IMAGE"; then
    NEEDS_BUILD=true
elif [[ "$JUMPSTART" -nt $(podman image inspect "$IMAGE" --format '{{.Created}}') ]]; then
    echo "jumpstart.sh is newer than image — rebuilding"
    NEEDS_BUILD=true
fi
[[ "${1:-}" == "--rebuild" ]] && NEEDS_BUILD=true

if $NEEDS_BUILD; then
    echo "==> Building $IMAGE ..."
    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT
    cp "$JUMPSTART" "$TMPDIR/jumpstart.sh"
    # write Containerfile into TMPDIR, build from there
    ...
fi

# Launch
exec podman run -it --rm \
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
3. **What base image?** `ubuntu:24.04` hardcoded, or a comment in `jumpstart.sh` that
   the tool reads?
4. **Should it pass the current branch** into the container so `git pull` checks out the
   right branch?
