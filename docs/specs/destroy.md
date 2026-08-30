# `devroom destroy`

## Purpose
Remove the shared base image for this repo — the "full reset", after which
the next `devroom build` starts from scratch.

## Usage
```
devroom destroy [-f|--force] [-k|--keep-children]
```
- `-f`/`--force`: proceed even if the base image is already missing;
  delete any existing rooms without asking first.
- `-k`/`--keep-children`: leave existing rooms in place rather than
  deleting them (forces the image removal itself, since the runtime won't
  remove an image a container still references unless forced).

## Behavior
1. Load config; require `runtime`. Resolve `owner`/`repo`; target is
   `dev-<owner>-<repo>:base`.
2. Check the image exists (`imageID`). If it doesn't and `-f` wasn't
   passed, error out ("base image does not exist").
3. List existing rooms (`listRoomNicknames`). If there are any and
   `-k` was **not** passed:
   - `-f` set → proceed without asking.
   - otherwise → print the room list and `confirmYN("Delete these rooms
     too?", false)` (default No). Declining aborts cleanly with a hint to
     re-run with `-k` or retire rooms first.
   - if proceeding: stop+remove each room via the same
     `stopAndRemoveContainer` [retire.md](retire.md) uses, then treat the
     room list as empty from here on.
4. If the image didn't exist (only reachable with `-f`), there's nothing
   left to do — return.
5. `<runtime> rmi [-f] dev-<owner>-<repo>:base` — `-f` is added only if
   `-k` kept rooms still referencing the image (containers freeze to the
   image ID they were created from, so the tag removal itself doesn't break
   them, but the runtime still refuses an untagged-but-referenced removal
   without force).

## Configuration
Reads `runtime`.

## Related specs
- [build.md](build.md) — the inverse operation.
- [retire.md](retire.md) — shares `stopAndRemoveContainer`; per-room equivalent.
- [room-lifecycle.md](room-lifecycle.md)
- [tui.md](tui.md) — the `X` key.
