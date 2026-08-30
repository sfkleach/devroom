# AI integration (`[[ai]]` entries)

## Purpose
How the `[[ai]]` config entries (see [configuration.md](configuration.md))
drive base-image install steps, per-room credential/env mounts, and
`devroom describe`'s AI invocation — the mechanism that lets devroom
support "any AI CLI", not just Claude Code, without code changes.

## Fields recap
`name`, `enabled` (default true), `install_command`, `credential_paths`,
`describe_command`, `env` — full field-by-field description in
[configuration.md](configuration.md).

## Build-time: installing into the base image
`aiInstallSteps` (cmd/devroom/build.go) emits one `RUN <install_command>`
Containerfile line per **enabled** entry with a non-empty
`install_command`, in config declaration order, appended *after*
`build_script`'s own `RUN bash /tmp/build.sh` step. This ordering is
required, not incidental: an AI CLI's install may depend on a toolchain
`build_script` sets up first (the shipped example — Claude Code needing
Node/npm via `fnm` — is exactly this case). See [build.md](build.md).

## Room-creation time: mounts (`aiRunArgs`, cmd/devroom/scripts.go)
Shared verbatim between `new.go` and `enter.go`'s first entry, so a room
created either way ends up with identical AI credentials mounted:
- For each **enabled** entry, each `credential_paths` entry is expanded
  (`~/` → home dir) and bind-mounted **read-write**, same host path on both
  sides (mirrors `~/.claude` needing write access for Claude Code's own
  state). A `credential_paths` entry that doesn't exist on the host is
  **skipped** rather than mounted — some runtimes silently create an empty
  directory at a missing bind-mount source, which is wrong when the entry
  actually names a file (e.g. `~/.claude.json`).
- Each `env` name is forwarded as a bare `-e VARNAME` (no `=value`), which
  both docker and podman resolve to the current host value of that
  variable at container-creation time.

Because these mounts happen at `new`/`enter`(first entry) rather than at
`build`, no AI credentials ever end up baked into the shared base image
itself — only the CLI binaries do.

## Describe-time: `devroom describe`
`cfg.ResolveDefaultAI()` picks the `AIEntry` named by `ai_default`; its
`describe_command` (must contain a `{}` placeholder) is what actually gets
invoked inside the room, with `{}` replaced by a verbosity-dependent prompt
string. Because the entry's `credential_paths` are already mounted into
every room from creation time (above), `describe` needs no separate API
key or config of its own — it just execs the same CLI the room already has
credentials for. Full request/response shape in [describe.md](describe.md).

## Related specs
- [configuration.md](configuration.md) — field definitions and `ResolveDefaultAI`.
- [build.md](build.md) — `aiInstallSteps`.
- [new.md](new.md), [enter.md](enter.md) — `aiRunArgs`.
- [describe.md](describe.md) — `describe_command` invocation.
