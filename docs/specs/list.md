# `devroom list`

## Purpose
Enumerate this repo's rooms, with optional extra columns and output formats
for both human reading and scripting.

## Usage
```
devroom list [-b|--branch] [-s|--statistics] [-i|--image] [-f|--format json|md]
```
- `-b`/`--branch`: add the room's current checked-out branch.
- `-s`/`--statistics`: add state, built time, last-entered time, size.
- `-i`/`--image`: add the image the room's container was built from, and
  whether it's "current"/"stale"/"unknown" relative to the live `:base` tag.
- `-f`/`--format`: `""` (default text), `"json"`, or `"md"`. Any other
  value is a validation error, checked before anything else runs.

## Behavior
1. Validate `--format`. Load config; require `runtime`. Resolve
   `owner`/`repo`.
2. `listRoomNicknames(runtime, owner, repo)`: lists all containers (running
   or stopped) whose name matches the `devroom-<owner>-<repo>-` prefix via
   `<runtime> ps -a --filter name=... --format {{.Names}}`, strips the
   prefix, sorts alphabetically. This is the same helper the TUI and
   `describe`/`destroy` use to enumerate rooms.
3. No rooms: prints `[]` (json) or "No rooms found for this repo." (text/md).
4. **Bare-nickname shortcut**: if format is default text *and* none of
   `-b`/`-s`/`-i` are set, just prints one nickname per line — no header,
   no table. Any explicit `-f` always produces the structured one-column-
   minimum output instead, regardless of the other flags.
5. Otherwise, builds a `NICKNAME` column plus any requested extra columns,
   fetching per-room data:
   - `roomBranch`: `(stopped)` placeholder if not running (never starts a
     stopped container just to read its branch — that would also disturb
     the "last entered" stat); otherwise `git rev-parse --abbrev-ref HEAD`
     inside the container.
   - `containerStatistics`: state (running/stopped), `Created`/`StartedAt`
     timestamps formatted by the runtime's own Go-template engine, and
     `SizeRw + SizeRootFs` rendered via `formatBytes` (binary units, KiB/
     MiB/...).
   - `roomImage`: the container's image ID, marked `current` if it matches
     the live `:base` tag's ID, `stale` if not, `unknown` if the tag itself
     can't be inspected (e.g. destroyed since).
6. Renders via `printListText` (tabwriter-aligned), `printListMarkdown`
   (space-padded GFM table), or `printListJSON` (array of objects keyed by
   column).

## Configuration
Reads `runtime`.

## Related specs
- [room-lifecycle.md](room-lifecycle.md) — what "stale"/"current" images and container states mean.
- [tui.md](tui.md) — reuses `listRoomNicknames` for its own room listing (does not call `runList`).
