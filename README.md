# Devroom

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

The following are mounted **read-only**:

| Mount | Purpose |
|---|---|
| `~/.ssh` | Git SSH authentication |
| `~/.gitconfig` | Git identity and settings |
| `~/.config/gh` | GitHub CLI (`gh`) credentials |
| `~/.config/glab` | GitLab CLI (`glab`) credentials |
| `$SSH_AUTH_SOCK` | SSH agent socket (if present) |
