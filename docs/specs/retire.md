# `devroom retire`

## Purpose
Stop and delete a single room's container, keeping the shared base image
intact — the "middle" lifecycle exit, short of destroying the base image.

## Usage
```
devroom retire <nickname>
```
`<nickname>` required.

## Behavior
1. Load config; require `runtime`. Resolve `owner`/`repo`.
2. `containerState` must be non-empty (container must exist), or this
   errors with "no room named ...".
3. `stopAndRemoveContainer`: if currently running, `<runtime> stop`
   first; then unconditionally `<runtime> rm`. No confirmation prompt.

`stopAndRemoveContainer` is shared with [destroy.md](destroy.md), which
calls it once per room when clearing out rooms ahead of removing the base
image — so the two subcommands can't drift on what "cleanly remove one
room's container" means.

## Configuration
Reads `runtime`.

## Related specs
- [destroy.md](destroy.md) — shares `stopAndRemoveContainer`.
- [room-lifecycle.md](room-lifecycle.md) — retire vs. destroy as distinct lifecycle operations.
- [tui.md](tui.md) — the `R` key.
