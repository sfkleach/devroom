# Devroom

[![Build and Test](https://github.com/sfkleach/devroom/actions/workflows/ci.yml/badge.svg)](https://github.com/sfkleach/devroom/actions/workflows/ci.yml)

Creates containers to act as local, virtual rooms for running AI agents to
safely work on feature branches.

## Security notes

### Claude credentials (`~/.claude`)

`devroom enter` mounts `~/.claude` into the room container **read-write**. This
is necessary because Claude Code writes session state (tokens, conversation
history) back to that directory; a read-only mount causes it to prompt for
credentials on every entry.

Consequence: code running inside a room has full access to your Claude
credentials. This is an accepted trade-off — devroom rooms are intended for
trusted development work, not for running untrusted third-party code.

### Other credential mounts

`~/.gitconfig` is mounted **read-only** to `~/.gitconfig.host-ro`, then copied
once into a writable, container-local `~/.gitconfig` during room setup. This
seeds git identity (`user.name`/`user.email`) from the host; the two files
diverge from that point on, so later changes to your host `~/.gitconfig` do
not propagate into existing rooms.

Commit/tag signing config (`commit.gpgsign`, `tag.gpgsign`, `gpg.format`,
`user.signingkey`) is stripped from the copied gitconfig, so commits made
inside a room are unsigned. Rooms have no access to the host's private
signing key or an `ssh-agent`, and — since rooms exist for AI agents to work
in — shouldn't be able to produce commits cryptographically signed as your
real identity anyway.

Git authentication for the room's origin (GitHub or GitLab) is handled by
acquiring a token from `gh`/`glab` on the host (`gh auth token` or `glab auth
status --show-token`) and piping it, over a dedicated stdin, into a one-shot
login of the same CLI inside the container. The token is never passed as a
container environment variable or CLI argument, so it isn't persisted in
`podman`/`docker inspect` output or visible via `ps` on the host. `gh`/`glab`
are hard dependencies of the base image (installed automatically before your
`build_script` runs) — see
[docs/decisions/0000-forge-cli-token-auth-instead-of-ssh-agent-forwarding](docs/decisions/0000-forge-cli-token-auth-instead-of-ssh-agent-forwarding/0000-forge-cli-token-auth-instead-of-ssh-agent-forwarding.md)
for the full rationale.
