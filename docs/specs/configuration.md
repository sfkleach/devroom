# Configuration (`devroom.toml`)

## Purpose
The schema and resolution rules for devroom's configuration file, which
every subcommand (other than `version`) loads before doing anything else.

## File resolution
`config.Load(root)` (internal/config/config.go) merges three levels, each
overriding keys the previous one(s) set (later files win per-key, not
per-file — see below):

| Scope | Path |
|---|---|
| System-wide | `/etc/xdg/devroom/devroom.toml` |
| User-wide | `~/.config/devroom/devroom.toml` |
| Per-repo | `<root>/.config/devroom/devroom.toml` |

Each existing file is decoded in turn directly into the same `Config`
struct, so a key present at a lower level and absent at a higher level keeps
its lower-level value; a key set at multiple levels takes the
highest-precedence (last-loaded) value. `[[ai]]` is the exception: TOML
array-of-tables decoding replaces the whole slice, so a file that declares
any `[[ai]]` entries replaces the entire list from earlier levels rather
than merging entry-by-entry.

If none of the three files exist, `Load` returns `config.ErrNoConfig`.
Every subcommand that sees this prints "No devroom configuration file
found." plus a hint to run `devroom init`, then exits(1) — see
[init.md](init.md).

## Scalar keys

| Key | Default when unset | Read by |
|---|---|---|
| `runtime` | (none — required) | everywhere; must be `"docker"` or `"podman"` |
| `base_image` | (none — required by `build`) | [build.md](build.md) |
| `build_script` | `~/.config/devroom/build.sh` | [build.md](build.md) |
| `enter_script` | `~/.config/devroom/enter.sh` | [enter.md](enter.md), [new.md](new.md) |
| `leave_script` | `~/.config/devroom/leave.sh` | [enter.md](enter.md) |
| `ai_default` | (none) | [describe.md](describe.md) |

`runtime` is checked for non-empty by every subcommand that shells out to a
container engine; nothing validates the `docker`/`podman` value itself
except `configure`'s interactive editor (see [configure.md](configure.md)).

`build_script`, `enter_script`, and `leave_script` are resolved by
`resolveHostScript` (cmd/devroom/scripts.go): a configured value is
resolved relative to `root` (if relative) or expanded from `~/` (if
prefixed), and the default falls back to a fixed filename under
`~/.config/devroom/`. If the resolved path doesn't exist on disk, the
script is silently skipped (not an error) — this is how "delete the file
and the config line to opt out" works throughout `devroom init`'s scaffold.

## `[[ai]]` entries
Each entry describes one AI CLI integration, installed into the shared base
image and mounted into every room:

| Field | Description |
|---|---|
| `name` | Identifier referenced by `ai_default` |
| `enabled` | `*bool`; `nil`/omitted defaults to `true` — `IsEnabled()` is the accessor |
| `install_command` | Run once during `devroom build`, after `build_script` |
| `credential_paths` | Host paths bind-mounted (rw, same path both sides) into every room |
| `describe_command` | Shell command for `devroom describe`; `{}` is replaced with the prompt text |
| `env` | Host environment variable *names* forwarded (bare `-e VAR`, value taken from the running host env at container-creation time) |

`cfg.ResolveDefaultAI()` looks up the `AIEntry` named by `ai_default`,
erroring if `ai_default` is unset, or if it names a missing or disabled
entry. See [ai-integration.md](ai-integration.md) for how these fields
drive `build`/`new`/`enter`/`describe`.

## Example
```toml
runtime = "podman"
base_image = "ubuntu:22.04"
build_script = "scripts/build.sh"
ai_default = "claude"

[[ai]]
name = "claude"
install_command = "npm install -g --prefix /usr/local @anthropic-ai/claude-code"
credential_paths = ["~/.claude", "~/.claude.json"]
describe_command = "claude -p {}"
```

## Related specs
- [init.md](init.md) — scaffolds this file plus `build.sh`/`enter.sh`/`leave.sh`.
- [configure.md](configure.md) — interactive editor for this file (per-repo level only).
- [ai-integration.md](ai-integration.md) — how `[[ai]]` entries are consumed.
