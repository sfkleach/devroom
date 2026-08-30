# `devroom build`

## Purpose
Build (or rebuild) the shared base image for this repo: OS + tools from
`build_script` + any enabled `[[ai]]` CLI installs. This is the "slow tier"
in the two-tier model — see [room-lifecycle.md](room-lifecycle.md).

## Usage
```
devroom build
```
No flags beyond the global ones. Takes no arguments. Always rebuilds
unconditionally when run — there's no "only if stale" check in this
subcommand itself (contrast the TUI's `n`/`e` guards, which check
`baseImageBuilt` before letting you proceed without one).

## Behavior
1. Load config; require `runtime` and `base_image` to be set (distinct
   error messages for each, both hinting at `devroom init` if there's no
   config file at all).
2. Resolve `owner`/`repo` from the git remote; the target image tag is
   `dev-<owner>-<repo>:base`.
3. Create a temp directory; if `build_script` resolves to an existing file
   (see [configuration.md](configuration.md) for resolution rules), copy it
   in as `build.sh`.
4. Generate a `Containerfile` (`generateContainerfile`):
   - `FROM <base_image>`, `ENV DEBIAN_FRONTEND=noninteractive`
   - Install `gh` and `glab` from their official apt repos
     (`forgeToolsInstall`) — a hard dependency, since both `new` and `enter`
     authenticate clones via whichever forge CLI matches the origin host;
     this assumes a Debian/Ubuntu base image
   - `COPY build.sh` + `RUN bash /tmp/build.sh`, only if a build script was
     found
   - One `RUN <install_command>` per enabled `[[ai]]` entry with a
     non-empty `install_command`, in config order, *after* `build_script* —
     an AI CLI's install may depend on a toolchain `build_script` installs
     (e.g. Claude Code needing Node/npm)
5. Run `<runtime> build -t dev-<owner>-<repo>:base <tmpdir>`, streaming
   stdout/stderr directly.

There is no confirmation prompt — a rebuild silently replaces the existing
`:base` tag. Existing room containers keep running on the image ID they
were created from (see [room-lifecycle.md](room-lifecycle.md)), so this is
safe for running rooms but they become "stale" relative to the new tag
(visible via `devroom list -i`).

## Configuration
Reads `runtime`, `base_image`, `build_script`, and all `[[ai]]` entries.

## Related specs
- [configuration.md](configuration.md)
- [ai-integration.md](ai-integration.md) — `aiInstallSteps`
- [room-lifecycle.md](room-lifecycle.md) — base image vs. room container tiers
- [destroy.md](destroy.md) — the inverse operation
