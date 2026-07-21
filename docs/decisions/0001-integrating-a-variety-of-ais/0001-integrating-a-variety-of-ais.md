# 0001 - Integrating a variety of AIs, 2026-07-19

## Issue

The current implementation hardcodes exactly one AI assistant: `enter.go`/
`new.go` unconditionally bind-mount `~/.claude` and the host's `claude`
binary into every room, with no config knob and no way to use a different
CLI. Separately, `devroom.toml` has an inert `describe_model` key intended to
back an AI-generated room description (`docs/devroom-proposal.md`, "AI room
description"), but no `describe` subcommand consumes it yet
(tracked in GH issue #1, "AI restriction").

There are two live consumers of "a working, credentialed AI CLI inside the
room": interactive use by the human at the shell prompt, and non-interactive
use to generate a branch description. Both need the same underlying
capability — a named AI integration with credentials and an invocation — so
this decision covers how that capability is configured and provisioned, not
the describe feature itself.

This is a **Leap**: no other repo/host was hardcoding a second AI CLI to
compare against: the design generalises the one hardcoded case (Claude)
directly to an open-ended list. It also folds in one narrow **Fork** —
whether the AI CLI's install step lives in `devroom.toml` (`[[ai]]`) or in
the project's own `build_script` — decided below.

## Factors

- Credential *shapes* differ across AI CLIs: some need a directory bind
  mount (`~/.claude`), some need an env var (`OPENAI_API_KEY`), some need
  both. A single scalar "token" per integration can't express this; it needs
  to be a small config table.
- Bind-mounting the host's binary (the current approach for `claude`) only
  works if that exact CLI happens to be installed on the host. That defeats
  the goal of supporting "AIs we don't anticipate" — a host without a given
  CLI installed simply couldn't use it in a room at all.
- The base image is already a shared, cached build step (`devroom build`,
  see `build.go`) that installs tool dependencies once via generated
  `Containerfile` RUN steps — `forgeToolsInstall` for `gh`/`glab` today, and
  the user's own `build_script` after it. Installing each AI CLI the same
  way (baked into the image) means every room gets it regardless of host
  state, with no per-room network hit.
- The alternative of doing the install via the project's own
  `build_script` was considered and rejected: it would make AI
  availability depend on the project author remembering to add it there,
  rather than being a first-class, declarative part of the AI integration
  config itself. Keeping `install_command` as a field on `[[ai]]` keeps the
  whole integration (credentials + invocation + how it gets installed)
  in one place.
- Credentials remain a per-room, per-user concern (they belong to the host
  user entering the room, not the shared image), so `credential_paths`/`env`
  stay mounted/passed at `enter`/`new` time, separate from the image build.

## Decision

Replace the single hardcoded Claude mount and the inert `describe_model` key
with a list of named AI integrations in `devroom.toml`:

```toml
ai_default = "claude"   # which entry backs `devroom describe`; all entries
                         # are still installed and mounted so any of them can
                         # be run interactively from the room's shell.

[[ai]]
name = "claude"
enabled = true   # default; installed/mounted into every room unless false
install_command = "npm install -g @anthropic-ai/claude-code"  # run once at
                                                               # `devroom build`
credential_paths = ["~/.claude"]   # bind-mounted (rw) at enter/new time
describe_command = "claude -p {}"   # {} substituted with the describe prompt

[[ai]]
name = "codex"
enabled = false  # kept in config (e.g. for occasional use) but not
                 # installed or mounted into rooms while disabled
install_command = "npm install -g @openai/codex"
env = ["OPENAI_API_KEY"]
describe_command = "codex exec {}"
```

`install_command` runs during `devroom build`, appended to the generated
Containerfile alongside `forgeToolsInstall`, before the project's own
`build_script` — so it's baked into the shared base image once rather
than re-run per room. `credential_paths` and `env` are applied per-room at
`enter`/`new` time, same as the current `~/.claude` mount, since credentials
belong to the entering host user rather than the shared image.

Every `[[ai]]` entry with `enabled` true (the default when the field is
omitted) is installed into the base image and mounted into every room;
entries with `enabled = false` are skipped entirely (no install, no mount)
but stay in `devroom.toml` so they can be flipped back on without
re-writing the config. `ai_default` must name an enabled entry.

## Consequences

- `devroom build` gains a network dependency on whatever registry each
  configured `install_command` uses (npm, pip, curl, etc.), matching the
  existing constraint already imposed on `forgeToolsInstall` and
  `build_script`.
- Updating an AI CLI's version now means rebuilding the base image, not
  picking up whatever is newer on the host — consistent with how
  `base_image` already works, not a new class of staleness.
- The host-binary-mount approach (today's only mechanism) is dropped as the
  primary path; a config with no `install_command` for a given tool simply
  won't have that binary available in the room. (Whether to keep host-mount
  as a fallback for tools with no `install_command` is left open — not
  decided here.)
- `describe_model` is retired in favour of `ai[].describe_command`; the
  `devroom describe` subcommand (not yet implemented) will resolve
  `ai_default` to pick which entry to invoke unless overridden.
- All *enabled* AI integrations are installed and mounted into every room,
  not just the default — so credentials for any enabled tool get mounted
  even if a given room never runs it interactively. `enabled = false` is the
  escape hatch: a configured-but-unused integration (e.g. one kept around
  for occasional use, or one whose credentials the user doesn't want mounted
  everywhere) is fully excluded from both build and mount steps rather than
  needing to be deleted from `devroom.toml`.

## Additional Notes

Implemented: `internal/config` now has `AIEntry`/`AI`/`AIDefault` as
described above; `build.go` emits an `install_command` `RUN` step per
enabled entry; `enter.go`/`new.go` mount each enabled entry's
`credential_paths`/`env` via a shared `aiRunArgs` helper instead of the
hardcoded Claude mount; and `devroom describe` (`describe.go`) resolves
`ai_default` and substitutes `{}` in its `describe_command`.

**Amendment, 2026-07-21**: the Decision above orders `install_command`
*before* the project's own `build_script`. In practice, Claude Code's
`install_command` needs Node/npm, which no base image has by default —
the only reasonable place to bootstrap that is `build_script` (it already
serves as the general per-project toolchain installer for Node/Python/Go/
Rust). So the implemented order is reversed: `forgeToolsInstall` →
`build_script` → `install_command`. This trades away the original
rationale (a project's `build_script` could assume the AI CLI already
exists) for the more common case (an AI CLI's `install_command` can assume
a `build_script`-installed toolchain exists).
