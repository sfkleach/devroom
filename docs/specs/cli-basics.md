# CLI basics

## Purpose
Global flags and trivial one-off commands shared across every subcommand,
plus how bare `devroom` (no subcommand) is dispatched.

## Usage
```
devroom [--rootdir|-r PATH] [--version|-V] [subcommand ...]
devroom version
```

## Behavior

### `--rootdir` / `-r` (persistent flag, root.go)
Overrides which directory devroom treats as the project root. Every
subcommand resolves its root via `effectiveRootDir()` (root.go), which
returns `--rootdir` if set, otherwise `os.Getwd()`. This is what every other
spec means by "root" — the directory devroom looks in for
`.config/devroom/devroom.toml` and the git remote.

### `--version` / `-V` (root flag)
When set on the bare root command, prints `devroom <Version>` and returns
immediately — no config loading, no git remote lookup. `Version` is a
package-level var (`main.Version`, default `"dev"`) set at build time via
`-ldflags "-X main.Version=<ver>"`.

### `devroom version` (subcommand)
Identical output (`devroom <Version>`) via a dedicated subcommand, for
scripts that don't want to rely on flag parsing order.

### Bare `devroom` (no subcommand, no `--version`)
Falls through to `runTUI()` (root.go) — the interactive menu. See
[tui.md](tui.md).

## Related specs
- [tui.md](tui.md) — what runs when no subcommand and no `--version` are given.
- [configuration.md](configuration.md) — what `effectiveRootDir()`'s result feeds into.
