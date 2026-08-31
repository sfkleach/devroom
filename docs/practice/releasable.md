# Releasable

Checks worth running before deciding a release is ready to cut. Broader
than a single change's `docs/practice/definition-of-done.md` — this is a
project-wide sanity pass done occasionally, not on every commit.

## 1. Retire documents that are no longer living

Docs written early (proposals, vision/walkthrough narratives, superseded
plans) tend to drift as the design evolves — `docs/vision.md`'s move to
`docs/archive/` is the precedent. Before a release, look for anything in
`docs/` that:

- No longer reflects current behavior and isn't worth the effort of
  keeping it fixed. The alternative to archiving is committing to
  actively maintain it, and most early narrative docs aren't worth that
  ongoing cost.
- Has been superseded by `docs/specs/*.md` or `docs/decisions/*.md`.

Move it to `docs/archive/` (`git mv`, so history is preserved) rather
than deleting it or leaving it in place to rot, where a future reader
might mistake it for current.

## 2. `CHANGELOG.md`'s `## Unreleased` section is complete

Not just "no obviously wrong entries" — that's `definition-of-done`'s
job, per change — but a release-time sweep specifically for gaps:
cross-check `## Unreleased` against `cmd/devroom/*.go`'s actual
subcommands and confirm nothing shipped is missing an entry. This has
happened before: `list`, `describe`, `destroy`, `version`, and the TUI
itself were all missing from the changelog until caught during v0.1.0
prep.

## 3. `docs/specs/*.md` covers every subcommand and matches its behavior

One spec file per subcommand or major feature, each describing actual
current behavior. A missing or stale spec is a release blocker, not a
"someday" cleanup.

## 4. `just test` passes cleanly

gofmt, `go vet`, unit tests, functional tests — the same bar as every
change, re-confirmed at the project level right before cutting.

## 5. No open, release-blocking issues

A quick look at `gh issue list` for anything that should block this
specific version. Most open issues are just backlog, not blockers, but
that's worth a conscious check rather than an assumption.

## Related

- `docs/practice/definition-of-done.md` — the per-change bar this
  checklist assumes has already been holding throughout.
- `docs/practice/release-process.md` — the mechanical steps to actually
  cut the release, once this checklist is satisfied.
