# `devroom describe`

## Purpose
Generate a fresh, AI-written summary of a room's current branch/progress by
running the configured `ai_default` CLI inside the room itself.

## Usage
```
devroom describe [-v...] <nickname>
```
- `<nickname>` required.
- `-v` (repeatable, `CountVarP`): 0 = one line, 1 (`-v`) = one paragraph,
  2+ (`-vv`, `-vvv`, ...) = a capped one-page detailed description. The
  actual wording sent to the AI CLI is fixed per level (`describePrompt`),
  not user-configurable.

## Behavior
1. Load config; require `runtime`.
2. `cfg.ResolveDefaultAI()` — requires `ai_default` to be set and to name an
   enabled `[[ai]]` entry; requires that entry's `describe_command` to be
   non-empty and contain a `{}` placeholder. All checked *before* touching
   git or containers, so config mistakes fail fast.
3. Resolve `owner`/`repo`; container must already exist
   (`containerState == ""` is an error: "no room named ...").
4. If the container isn't running, start it, generate the description, then
   `defer` stopping it again — so `describe` never leaves a stopped room
   running behind it, but does briefly start one if needed. A failure to
   stop afterward is a printed warning, not a returned error.
5. Build the prompt for the requested `-v` level, substitute it into
   `describe_command` in place of `{}` (shell-quoted via `shellQuote`,
   since the prompt text — unlike nicknames/paths elsewhere in the
   codebase — isn't a trusted identifier).
6. Exec inside the container as the matched host user:
   ```
   cd ~/workspace && { git diff main..HEAD; echo "---"; cat CHANGELOG* 2>/dev/null; } | <describe_command>
   ```
   and print its trimmed stdout.

Description is generated fresh every call — never cached — so it always
reflects current branch state.

## Configuration
Reads `runtime` and the `ai_default`-named `[[ai]]` entry's
`describe_command`.

## Related specs
- [ai-integration.md](ai-integration.md) — `describe_command`, credential mounts that make this possible without extra API keys.
- [configuration.md](configuration.md) — `ResolveDefaultAI` validation rules.
