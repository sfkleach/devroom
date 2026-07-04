# 0000 - Forge CLI token auth instead of SSH agent forwarding, 2026-07-04

## Issue

`devroom new`/`devroom enter` need to authenticate `git clone` (and later
`push`/`fetch`) against the origin's forge from inside the room container.
The original design forwarded `$SSH_AUTH_SOCK` and bind-mounted `~/.ssh`
read-only. In practice this depends on a long-lived `ssh-agent` already
holding unlocked keys on the host; if none is running (a bare `ssh-agent`
process dies with its parent shell, so most terminals start with none), every
room creation fails non-interactively with "Host key verification failed" or
"Permission denied (publickey)", and the fix is out of devroom's hands.

This is a **Fork**: genuine alternatives were compared during the session
that hit the problem. Not post-hoc.

## Factors

- SSH agent forwarding only works if a persistent, already-unlocked agent
  exists on the host; devroom cannot create or unlock one itself without
  handling passphrases, which it must never do.
- `gh`/`glab` are already how this project's maintainer authenticates day to
  day, and both CLIs can resolve their own stored token even when it's
  backed by the OS keyring rather than a plaintext file (confirmed:
  `gh auth token` resolves a keyring-backed token; `~/.config/gh/hosts.yml`
  itself had no `oauth_token` field at all on the test machine).
- The previously-documented `~/.config/gh` / `~/.config/glab` read-only
  mounts (see docs/devroom-proposal.md) do not work when the token is
  keyring-backed, since the container has no access to the host's
  keyring/D-Bus session. The `glab` mount path was also wrong regardless
  (`~/.config/glab` vs. the real `~/.config/glab-cli`).
- Least privilege: forwarding the whole desktop keyring/D-Bus session into
  the container to make the existing config mount work would expose far more
  than a single token — rejected on that basis.
- `glab` has no dedicated machine-readable token command equivalent to
  `gh auth token`; the token must be scraped from `glab auth status
  --show-token`, which is inherently more fragile.

## Decision

Acquire a token from the forge CLI already authenticated on the host
(`gh auth token`, or parsed from `glab auth status --show-token`), and pipe it
over a dedicated stdin into a one-shot `<engine> exec` running as the target
user inside the container, which logs the same CLI in there
(`gh auth login --with-token`, `glab auth login --stdin`) and configures
git's `credential.<url>.helper` to invoke it. The clone then uses the HTTPS
remote URL instead of SSH.

This replaces the `~/.ssh` bind mount and `SSH_AUTH_SOCK` forwarding entirely
for devroom's own git operations, rather than keeping SSH as a silent
fallback: a silent fallback was exactly what produced the confusing,
hard-to-diagnose failures this decision fixes. If the host CLI isn't
installed or authenticated, `devroom new`/`enter` now fail immediately with a
clear message telling the user to run `gh auth login` / `glab auth login`.

The token never touches an env var on `run` (which `podman`/`docker inspect`
would persist for the container's lifetime) or a CLI argument (visible via
`ps` on the host); it only ever flows through a pipe into a single `exec`
call's stdin.

`gh`/`glab` are now hard dependencies of the base image, installed via their
official apt repositories before the project's own `jumpstart_script` runs
(see build.go's `forgeToolsInstall`).

## Consequences

- `~/.gitconfig` can no longer be bind-mounted straight onto the container
  user's `$HOME/.gitconfig`, because the credential-helper setup step needs
  to write to that file. It's instead mounted read-only to a side path
  (`~/.gitconfig.host-ro`) and copied into a real, writable, container-local
  `~/.gitconfig` once during user setup — this seeds identity (`user.name`/
  `user.email`) from the host but the two files diverge after that point
  (e.g. later host-side gitconfig edits won't propagate into existing rooms).
- Only `github.com` and self-hosted-heuristic `gitlab` hosts are supported;
  any other forge fails clearly with "unrecognised git forge host" rather
  than falling back to SSH. Plain SSH-only forges are no longer supported by
  `devroom new`/`enter` at all.
- `glab` token extraction depends on parsing human-oriented CLI output
  (`Token: <value>`), which could break if `glab` changes its status output
  format in a future release.
- Base image builds now require network access to `cli.github.com` and
  `packages.gitlab.com`, and are Debian/Ubuntu-specific (`apt-get`), matching
  the existing constraint already imposed by `jumpstart.sh`.
- `firstEntry` in `enter.go` was restructured from a single `run -it` with an
  embedded init script into phased `run -d` + `exec` steps (matching
  `new.go`), specifically so the forge token could be piped over a dedicated
  stdin without displacing the real terminal stdin needed for correct
  raw-mode TTY behaviour in the final interactive shell.

