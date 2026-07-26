# Task: Rename `close` to `retire`

The `close` subcommand and its TUI key are renamed to `retire`, to better
match the "room" metaphor — a room is retired, not closed. This is a pure
rename: no behaviour changes.

- `cmd/devroom/close.go` → `cmd/devroom/retire.go`. `closeCmd`/`runClose` →
  `retireCmd`/`runRetire`. `Use: "close <nickname>"` →
  `Use: "retire <nickname>"`. The "Room %q closed." message becomes "Room
  %q retired.".
- The TUI key changes from `Q` to `R` (`cmd/devroom/tui.go`): the menu
  legend and the switch case both move, and the nickname prompt text
  becomes "Retire room: ".
- Docs updated to match: `docs/devroom-proposal.md`'s TUI command table, CLI
  subcommand table, and "Close room" lifecycle section heading; `docs/vision.md`'s
  wording (its walkthrough already used different key letters than the
  real implementation before this change — that pre-existing drift is
  untouched here, only the "closes" → "retires" wording is updated).
- No `close` alias is kept — nothing has shipped yet (still under
  `## Unreleased` in `CHANGELOG.md`), so this is a clean rename rather than
  a deprecation.
