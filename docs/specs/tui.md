# TUI (interactive menu)

## Purpose
The command loop `devroom` opens when run with no subcommand — a
type-a-letter-then-Enter menu that's a thin dispatcher over the same `run*`
functions the direct subcommands use, so all real behavior lives in one
place (see each linked spec for what actually happens on each key).

## Entry point
Bare `devroom` with no `--version` → `runTUI()` (root.go), implemented in
cmd/devroom/tui.go.

## Behavior

### Setup
Loads config (same `ErrNoConfig` handling as every subcommand); resolves
`owner`/`repo` from the git remote. Uses a single shared
`bufio.Reader(os.Stdin)` for the whole session — a fresh reader per prompt
would risk dropping input already buffered ahead of the current line.

### Menu rendering (`printTUIMenu`)
The room list ("Rooms" header) is shown on the **first** screen only, then
suppressed until explicitly requested again via `l` — otherwise it would
reprint after every single command and drown out the command legend. When
the room list is empty, the message hints at `B` (build the base image) if
`baseImageBuilt` is false, since that's the most likely reason there are no
rooms yet. Only the first 9 rooms get a `1`-`9` shortcut number; beyond
that, a line hints at `e` (enter by name). The "Commands" section is always
shown, ending with a `command: ` prompt.

### Key dispatch
Reads one line via `readCommand` (first non-whitespace byte is the
command; a blank line just redraws the menu; `io.EOF`, e.g. Ctrl-D, exits
cleanly).

| Key | Action |
|---|---|
| `n` | Guards on `baseImageBuilt` first (prints "No base image found. Press 'B' to build one first." and stops if not) — done here rather than inside `new` so a missing base image is caught *before* prompting for a nickname whose answer would otherwise be discarded. Then prompts for a nickname and a y/N "create a branch matching the room name?", and calls `runNew`. |
| `1`-`9` | `runEnter` on the corresponding listed room; "No such room." if the slot is out of range. |
| `e` | Prompts for a nickname; requires it to already exist (`slices.Contains` against the current room list) — deliberately does **not** fall through to `runEnter`'s first-entry auto-create, so a typo doesn't silently create a room; points at `n` instead. |
| `l` | Sets `showRooms = true` for the next redraw. |
| `d` | `runDescribe` for every current room, in turn. |
| `c` | `runConfigure`, then reloads config from disk afterward (so subsequent guards like `baseImageBuilt`'s runtime use reflect any change). |
| `R` | Prompts for a nickname, calls `runRetire`. |
| `B` | `runBuild`. |
| `X` | `runDestroy`. |
| `q` | Returns, ending the loop. |
| anything else | "Unknown command ..." hint to press `q`. |

Errors from any dispatched action are printed to stderr and the loop
continues — a failed `enter` doesn't kill the whole menu.

### Terminal I/O notes
This is cooked-mode (type + Enter), not raw single-keypress — a real
terminal already echoes the Enter that ends a typed line, so the loop
prints no extra blank line after reading a command; only one blank line
separates each screen.

## Configuration
Reads `runtime` (via `cfg`) and re-reads it after `c` (configure).

## Related specs
Every dispatched key: [new.md](new.md), [enter.md](enter.md),
[list.md](list.md), [describe.md](describe.md), [configure.md](configure.md),
[retire.md](retire.md), [build.md](build.md), [destroy.md](destroy.md).
