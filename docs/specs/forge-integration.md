# Forge integration (GitHub / GitLab auth)

## Purpose
How devroom authenticates HTTPS git operations *inside* a room container
without forwarding SSH keys/agents or bind-mounting host CLI config —
shared logic used by both `devroom new` and `devroom enter` (first entry).
See also
[docs/decisions/0000-forge-cli-token-auth-instead-of-ssh-agent-forwarding](../decisions/0000-forge-cli-token-auth-instead-of-ssh-agent-forwarding/0000-forge-cli-token-auth-instead-of-ssh-agent-forwarding.md)
for the original rationale.

## Behavior

### Detection (`detectForge`, cmd/devroom/forge.go)
Classifies the remote origin's host:
- `github.com` exactly → GitHub (`gh`)
- any other host containing the substring `"gitlab"` (self-hosted included)
  → GitLab (`glab`)
- anything else → `forgeUnknown`, which both `new` and `enter` treat as a
  hard error ("only github.com and gitlab hosts are supported")

### Token acquisition (`forge.token()`)
- GitHub: `gh auth token` — works regardless of whether `gh` stores the
  token in a plaintext config file or the OS keyring, which is exactly why
  bind-mounting `~/.config/gh` doesn't work as an alternative.
- GitLab: `glab auth status --show-token`, parsed with a regex
  (`Token:\s*(\S+)`) since `glab` has no dedicated machine-readable token
  command; treated as best-effort and may need updating if `glab`'s output
  format changes. Explicitly checks for an "Invalid token" substring in the
  output to give a clearer error than a regex-match failure would.
- Both fail with an actionable message pointing at `gh auth login` /
  `glab auth login` rather than silently falling back to anything else.

Acquired **before** any container is created (in both `new` and `enter`),
so a misconfigured/missing forge CLI fails fast rather than after a room
has been partially created.

### In-container login (`loginScript`)
A bash script, piped the token over a dedicated stdin (not an env var or
CLI arg, to avoid it leaking into shell history or `ps`), that:
1. Reads the token from stdin into `DEVROOM_FORGE_TOKEN`.
2. Runs the forge CLI's one-shot login against the specific hostname
   (`gh auth login --with-token --insecure-storage --git-protocol https
   --hostname <host>` / `glab auth login --stdin --hostname <host>`) —
   persisted inside the container's own filesystem, not a host bind mount.
3. Sets `git config --global credential.<https://host>.helper` to
   `!gh auth git-credential` / `!glab auth git-credential`, so subsequent
   `git` HTTPS operations inside the room authenticate transparently.

This requires `$HOME/.gitconfig` to already be writable inside the
container at that point — which is why `new.go`/`enter.go` mount the host's
`~/.gitconfig` read-only to a *side path*
(`~/.gitconfig.host-ro`) rather than straight onto `$HOME/.gitconfig`, and
`userSetupScript` (see [room-lifecycle.md](room-lifecycle.md)) copies it
into place as a real, writable file before login runs.

### Clone
Once logged in, the repo is cloned via its HTTPS remote
(`devgit.HTTPSRemote(host, owner, repo)`) rather than the original
(possibly SSH) remote URL — HTTPS is what the credential helper above
actually intercepts.

## Related specs
- [new.md](new.md), [enter.md](enter.md) — the two call sites, both doing container-run → `userSetupScript` → forge login → clone in the same order.
- [room-lifecycle.md](room-lifecycle.md) — the surrounding container provisioning these steps are embedded in.
