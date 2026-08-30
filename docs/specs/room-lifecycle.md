# Room lifecycle & naming

## Purpose
The two-tier container model devroom is built around, the naming
conventions derived from it, and how state transitions across
`build`/`new`/`enter`/`retire`/`destroy` fit together. This is the
connective-tissue spec the individual subcommand specs assume.

## The two tiers

| Tier | What it is | Created by | Destroyed by |
|---|---|---|---|
| **Base image** | OS + tools (`build_script` + `[[ai]]` installs) | `devroom build` | `devroom destroy` |
| **Room container** | A clone of the repo, checked out to one branch, layered on the base image | `devroom new` or first `devroom enter` | `devroom retire` |

The base image is shared across every room in a repo. Room containers are
persistent — exiting the shell stops the container but does not delete it;
re-entering resumes it (`<runtime> start`), it doesn't reclone or
reprovision.

Containers freeze to the *image ID* they were created from, not the mutable
`:base` tag — rebuilding the base image (`devroom build` again) doesn't
retroactively change already-created rooms. `devroom list -i` reports a
room's image as `current` (matches the live tag), `stale` (tag has moved
on), or `unknown` (tag no longer resolvable, e.g. destroyed). This is also
why `devroom destroy` needs `-f` to remove an image that stopped-but-not-
deleted containers still reference.

## Naming
- Base image: `dev-<owner>-<repo>:base` (local image store only — nothing
  is pushed anywhere).
- Room container: `devroom-<owner>-<repo>-<nickname>`.

`owner`/`repo` are derived once, from the git remote origin
(`devgit.RemoteOrigin` + `devgit.OwnerRepo`), and used consistently for
both naming schemes across every subcommand.

## Container-creation provisioning (shared by `new` and `enter`'s first entry)
Both entry points run the identical sequence, factored so they can't drift
apart:
1. `<runtime> run -d --name <container> ... <base image> sleep infinity`
   (an inert placeholder process, not a shell) with:
   - `DEVROOM_UID`/`GID`/`USER`/`HOME` env vars
   - `--userns=keep-id` under podman — rootless podman remaps container
     UIDs through the subuid range by default, which would otherwise break
     read access to bind-mounted host files (e.g. `~/.ssh/known_hosts`,
     breaking ssh host-key verification); `keep-id` maps the container's
     matching UID/GID back to the real host identity
   - the host's `~/.gitconfig` mounted read-only to a *side path*
     (`~/.gitconfig.host-ro`), not straight onto `$HOME/.gitconfig` — the
     forge login step needs to write a credential helper into a *writable*
     gitconfig (see [forge-integration.md](forge-integration.md))
   - `[[ai]]` credential/env mounts, `enter_script`/`leave_script` mounts
2. `userSetupScript` (cmd/devroom/usersetup.go), run as root:
   - removes a base image's baked-in user if it collides with the target
     UID and isn't already the right account (common at UID 1000)
   - creates the matching group/user at the host's UID/GID, home dir owned
     correctly
   - seeds `.bashrc` from `/etc/skel` and `.gitconfig` from the
     `.gitconfig.host-ro` side-mount, then strips any commit-signing config
     from the copied gitconfig (`commit.gpgsign`, `tag.gpgsign`,
     `gpg.format`, `user.signingkey`) — rooms have no access to the host's
     signing key or agent and shouldn't produce commits that claim to be
     cryptographically signed as the real developer
   - grants passwordless `sudo` (the account's password is locked, so a
     real password prompt could never be satisfied)
3. Forge login + HTTPS clone into `~/workspace` — see
   [forge-integration.md](forge-integration.md).

## State transitions
- **No container** → `new` or `enter` → provisioned, running (`new`) or
  running-then-shelled-into (`enter`'s first entry).
- **Running** → shell exits → **stopped** (container still exists).
- **Stopped** → `enter` → **running** again (`resumeEntry`: plain
  `<runtime> start`, no reprovisioning).
- **Any state** → `retire` → container gone, base image untouched.
- **Base image gone or kept, rooms optionally kept** → `destroy` →
  base image gone (rooms, if kept, become permanently `unknown`-image
  going forward since the tag they'd compare against no longer exists).

## Related specs
- [build.md](build.md), [new.md](new.md), [enter.md](enter.md), [retire.md](retire.md), [destroy.md](destroy.md)
- [forge-integration.md](forge-integration.md)
- [list.md](list.md) — surfaces these states/markers
