# `devroom enter`

## Purpose
Open an interactive shell in a room's container, provisioning it from
scratch on first entry, or starting/attaching to it on subsequent entries.

## Usage
```
devroom enter <nickname>
```
`<nickname>` is required (exactly one positional arg).

## Behavior
`runEnter` branches on `containerState(runtime, containerName)`:

- **`""` (container doesn't exist) → first entry** (`firstEntry`): detects
  the forge, acquires a token, then does the *same* run/provision/login/
  clone sequence as `devroom new` (container run with all the same mounts,
  `userSetupScript`, forge login, `git clone`) — see
  [new.md](new.md) and [room-lifecycle.md](room-lifecycle.md) for why these
  two are kept from drifting apart. Ends by calling `execShell`.
- **`"running"` → already attached elsewhere or left running**: calls
  `execShell` directly, no provisioning.
- **anything else (e.g. `"exited"`) → resume** (`resumeEntry`): `<runtime>
  start <container>` then `execShell`.

`devroom new` always leaves its container in the `"running"` state (it runs
`sleep infinity`), so entering a room created via `new` takes the middle
branch, not the first-entry branch — the provisioning already happened
during `new`.

### `execShell`
Regardless of which branch got here, this is what actually puts the user
at a shell:
1. Writes `~/.devroom_rc` inside the container (as the matched host user):
   sources `/etc/profile` then `~/.bashrc`, then either sources the
   mounted `enter_script` (`/etc/devroom/enter.sh`, if present) or falls
   back to `PS1="<nickname>% "`, then `cd ~/workspace`.
2. Runs `bash --init-file ~/.devroom_rc -i` (interactive, connected to the
   real terminal's stdin/stdout/stderr) — **not** `exec`'d, so control
   returns afterward.
3. After that interactive shell exits, sources the mounted `leave_script`
   (`/etc/devroom/leave.sh`) if present — for stopping services cleanly.
   `leave_script` does *not* run inside the interactive session, so it only
   sees what `/etc/profile` set up, not anything the session or
   `enter_script` exported (see [configuration.md](configuration.md)).
4. `DEVROOM_NICKNAME` is exported into the exec'd process for
   `enter_script`/`leave_script` to use.

## Configuration
Reads `runtime`; `enter_script`/`leave_script` (mounted at container-creation
time by `new.go`/`firstEntry`, not by `enter` itself — `enter` only relies
on the fixed container-side paths already being there).

## Related specs
- [new.md](new.md) — the provisioning steps first entry shares with it.
- [forge-integration.md](forge-integration.md)
- [room-lifecycle.md](room-lifecycle.md) — first entry vs. resume as lifecycle states.
- [tui.md](tui.md) — the `1`-`9`/`e` keys both call `runEnter` directly.
