# `devroom new`

## Purpose
Create a new room container from the shared base image, without entering
it — provisioning (user setup, forge auth, clone) happens up front so the
first `devroom enter` is just an exec into an already-ready container.

## Usage
```
devroom new [-b|--branch] <nickname>
```
- `<nickname>` (required, exactly one positional arg): both the room's
  identifier and, with `-b`, the branch name to create.
- `-b`/`--branch`: also create and check out a new branch named
  `<nickname>` in the *host* repo working copy (via `git branch --list` to
  check availability, then `git checkout -b` *inside the container* against
  the freshly cloned copy — the host branch check is just an availability
  guard, not itself a checkout).

## Behavior
1. Load config; require `runtime`.
2. Resolve `owner`/`repo`; **require the base image to already exist**
   (`baseImageBuilt`) — fails fast with "run 'devroom build' first" before
   any prompts or forge I/O, precisely so a missing base image can't waste
   a round-trip of forge auth.
3. Detect the forge from the remote host (`detectForge`); unsupported hosts
   fail clearly. Acquire an auth token from the host's forge CLI *before*
   creating any container, so credential problems fail fast — see
   [forge-integration.md](forge-integration.md).
4. If `-b`, check the branch name isn't already taken on the host repo
   (`checkBranchAvailable`); abort if it is.
5. `<runtime> run -d --name devroom-<owner>-<repo>-<nickname> ... <base
   image> sleep infinity` — a detached, long-lived placeholder process (not
   yet an interactive shell). Run args include:
   - `DEVROOM_UID`/`GID`/`USER`/`HOME` env vars for the setup script
   - `--userns=keep-id` for podman (UID/GID mapping — see
     [room-lifecycle.md](room-lifecycle.md))
   - the host `~/.gitconfig` mounted read-only to a side path (not straight
     onto `$HOME/.gitconfig`, since the login step needs to write to it)
   - `[[ai]]` credential/env mounts (`aiRunArgs`)
   - the `enter_script`/`leave_script` mounts, if configured/present
6. Exec `userSetupScript` as root inside the container to provision the
   matching unprivileged user — shared verbatim with `enter.go`'s first
   entry (see [room-lifecycle.md](room-lifecycle.md)).
7. Pipe the forge token into a one-shot login (`loginScript`) as the
   provisioned user.
8. Clone the repo over HTTPS into `~/workspace` as the provisioned user.
9. If `-b`, `git checkout -b <nickname>` inside the container's clone.
10. Print a hint to run `devroom enter <nickname>` next.

Any failure at steps 5–9 leaves a partially-provisioned container behind —
there's no rollback/cleanup on error.

## Configuration
Reads `runtime`; all `[[ai]]` entries; `enter_script`/`leave_script`.

## Related specs
- [enter.md](enter.md) — how the container created here is subsequently entered (it will already be "running", so `enter` execs a shell directly rather than treating it as a first entry).
- [forge-integration.md](forge-integration.md)
- [room-lifecycle.md](room-lifecycle.md)
