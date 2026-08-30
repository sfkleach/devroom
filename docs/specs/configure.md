# `devroom configure`

## Purpose
Interactively edit the *per-repo* `devroom.toml` (never the user/system
levels), including full CRUD on `[[ai]]` entries, without hand-editing TOML.

## Usage
```
devroom configure
```
No flags/args. Always targets `<root>/.config/devroom/devroom.toml`.

## Behavior
- Session is seeded **only** from the repo-level file (`loadConfigureSession`),
  never from the three-level merged `config.Load()` result — seeding from
  the merge would risk silently "promoting" a user/system-level-only value
  into the repo file the moment the session saves.
- If the repo file has keys this version of devroom doesn't recognise
  (`toml.MetaData.Undecoded()`), warns up front that saving will drop them.
- Menu loop (numbered fields, same `readCommand`/`promptLine` helpers the
  TUI uses):
  1. `runtime` (validated: must be `docker`, `podman`, or blank)
  2. `base_image`
  3. `build_script`
  4. `enter_script`
  5. `leave_script`
  6. `ai_default`
  - `a` — manage `[[ai]]` entries: list, add (prompts for a name, or edits
    the existing entry if the name is already taken), edit (`name`,
    `enabled` tri-state, `install_command`, `credential_paths`,
    `describe_command`, `env`), delete (with a y/N confirmation).
  - `s` — save: re-confirms before dropping any undecoded keys, then
    regenerates the *entire* file from the in-memory `Config`
    (`buildConfigureOutput`) and exits.
  - `r` — reload from disk, discarding unsaved changes.
  - `q` — quit without saving.
- Every field edit: show current value, blank input leaves it unchanged,
  `-` explicitly clears it to empty/unset.
- Output regeneration (`buildConfigureOutput`) writes scalar keys first
  (only non-empty ones, each with an explanatory comment), then all
  `[[ai]]` blocks last — a hard TOML requirement, since a bare `key =
  value` after a table header belongs to that table. String values are
  quoted via `tomlQuote` (escapes `\\`, `"`, and control chars), since
  these are freely user-typed unlike `init.md`'s fixed template values.

## Configuration
Reads and rewrites the per-repo `devroom.toml` only.

## Related specs
- [configuration.md](configuration.md) — the schema being edited.
- [tui.md](tui.md) — the `c` key calls this subcommand, then reloads config afterward.
