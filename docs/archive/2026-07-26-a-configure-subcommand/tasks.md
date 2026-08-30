# Task: Implement the `configure` subcommand

`devroom configure` is an interactive, menu-driven editor for this repo's
`.config/devroom/devroom.toml` — the file `devroom init` scaffolds. It's the
`c` command in the TUI (`docs/devroom-proposal.md`'s TUI command table,
`docs/vision.md`'s walkthrough), which was previously unwired.

It only ever edits the repo-level config file (never the user- or
system-level ones) — same path `init` writes to,
`REPOROOT/.config/devroom/devroom.toml`. There's no `--user`/`--system`
option.

## Behaviour

- On start, it loads that one file (not the three-level merged config —
  loading the merge would risk silently "promoting" a value that's only set
  at the user or system level into the repo file the moment you save). If
  the file doesn't exist yet, it starts from a blank config, same as if you
  were about to run `init` but interactively.
- If the file has any top-level keys this version of devroom doesn't
  recognise (e.g. something hand-added), it prints a one-time warning
  listing them before the menu appears — saving from that session drops
  them, since every save fully regenerates the file from what's currently
  in memory rather than patching the existing text.
- The top-level menu lists the five scalar keys (`runtime`, `base_image`,
  `build_script`, `enter_script`, `ai_default`) with their current values
  (or `(unset)`), a one-line summary of configured `[[ai]]` entries, and
  `a` to manage them. Pick a field by number to edit it: it shows the
  current value and a short description, and you can type a new value,
  press Enter to leave it exactly as it is, or type `-` to clear it. Fields
  can be filled in any order — it's a menu, not a linear wizard.
- `a` opens a submenu for full CRUD on `[[ai]]` entries: list, add, edit,
  delete (no reordering). Editing an entry drops into the same kind of
  field-by-field menu as the top-level one, covering `name`, `enabled`
  (unset/true/false), `install_command`, `credential_paths`,
  `describe_command`, `env`. Deleting asks for confirmation first.
- `runtime` only accepts `docker` or `podman` (or clearing it) and
  reprompts on anything else. `ai_default` doesn't have to match an
  existing `[[ai]]` entry yet — you might set it before adding the entry it
  names — but it prints a note if it currently doesn't resolve.
  `base_image`/`build_script`/`enter_script` and the free-text `[[ai]]`
  fields accept anything.
- Nothing is written to disk until you explicitly save. The main menu has
  three session-level actions: save (writes the file and exits — asks for
  confirmation first only if there were unrecognised keys that this save
  would drop), reload from disk (discards any unsaved changes in this
  session and re-reads the file), and quit without saving.

## Wiring into the TUI

Added a `c` case to `runTUI`'s switch (`cmd/devroom/tui.go`) that calls
`runConfigure` the same way `B`/`X`/etc. call their subcommands, and added
it to the menu legend between `d` and `B`. Since the TUI loads its config
once before entering its command loop, it's reloaded after `configure`
returns so later menu actions in the same session see any changes.
