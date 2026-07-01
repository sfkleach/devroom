# Proposal: `devroom` extension points

This document covers the mechanisms by which users can customise the shell
environment inside a devroom container without modifying `devroom` itself.

## The problem

`devroom` drops the user into an interactive shell inside a container. By
default it sets a minimal prompt (`<nickname>% `) so the user knows which room
they are in. But many developers use richer prompt tools — starship, powerline,
oh-my-zsh, a custom `PS1` — and expect these to work inside the devroom just as
they do on the host.

`devroom` cannot know about every prompt tool, so it provides three general
extension points instead.

---

## Extension point 1: `DEVROOM_NICKNAME`

`devroom` always injects the room nickname as an environment variable:

```bash
-e DEVROOM_NICKNAME=taskbar-rampage
```

Any prompt tool or shell config can reference `$DEVROOM_NICKNAME` to incorporate
the room name into the prompt. This is the only devroom-specific variable; all
other context (branch, repo, etc.) is available via standard git commands.

---

## Extension point 2: `init_script`

A shell snippet sourced inside the container just before the interactive shell
starts. Use this to initialise prompt tools, set aliases, or configure anything
that belongs in a shell session but not in the image itself.

### Configuration

```toml
init_script = "~/.config/devroom/init.sh"
```

The default path is `~/.config/devroom/init.sh` (user-wide). A per-repo
override can be set in `REPOROOT/.config/devroom/devroom.toml`. If the file
does not exist, the step is silently skipped.

The script is sourced (not executed) so it can set environment variables and
shell functions that persist into the interactive session.

### Example: starship

```bash
# ~/.config/devroom/init.sh

# Initialise starship for the current shell
eval "$(starship init ${SHELL##*/})"

# Make the room nickname available to the starship config
export STARSHIP_CUSTOM_DEVROOM="$DEVROOM_NICKNAME"
```

With a matching starship config on the host:

```toml
# ~/.config/starship.toml
[custom.devroom]
command = "echo $STARSHIP_CUSTOM_DEVROOM"
when = '[ -n "$STARSHIP_CUSTOM_DEVROOM" ]'
format = "[ $output]($style) "
style = "bold cyan"
```

### Example: plain PS1 fallback

```bash
# ~/.config/devroom/init.sh
export PS1="${DEVROOM_NICKNAME}% "
```

---

## Extension point 3: `extra_mounts`

A list of additional host paths to mount read-only into the container. Use this
to make host-side tool configuration available inside the room without baking it
into the image.

### Configuration

```toml
extra_mounts = [
    "~/.config/starship.toml",
    "~/.config/fish",
]
```

Each entry is a host path. The container-side mount point mirrors the host path
(e.g. `~/.config/starship.toml` → `/root/.config/starship.toml:ro`). Tilde
expansion is applied. Non-existent paths are silently skipped.

---

## Putting it together: starship example

A user who wants starship inside every devroom would:

1. Add `starship` installation to their `scripts/jumpstart.sh`:
   ```bash
   curl -sS https://starship.rs/install.sh | sh -s -- --yes
   ```

2. Add to `~/.config/devroom/devroom.toml`:
   ```toml
   extra_mounts = ["~/.config/starship.toml"]
   ```

3. Add to `~/.config/devroom/init.sh`:
   ```bash
   eval "$(starship init ${SHELL##*/})"
   export STARSHIP_CUSTOM_DEVROOM="$DEVROOM_NICKNAME"
   ```

On first entry to any new room, starship is installed (via the image build),
configured (via the mounted `starship.toml`), and initialised (via `init.sh`).
No changes to `devroom` configuration are needed per room or per repo.

---

## Interaction with shell selection

The `init_script` is sourced after `$SHELL` is resolved but before the
interactive session begins, so it can safely call `eval "$(starship init
${SHELL##*/})"` or equivalent without knowing the shell in advance.

## Default prompt fallback

If no `init_script` is configured or the file does not exist, `devroom` sets a
minimal fallback prompt so the user always knows which room they are in:

```bash
export PS1="${DEVROOM_NICKNAME}% "
```

This is overridden by anything the `init_script` sets.
