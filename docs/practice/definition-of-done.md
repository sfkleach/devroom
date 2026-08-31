# Definition of Done

A change — a commit, a PR, a piece of work handed back for review — isn't
done until all three of the following hold, not just "the code works":

## 1. `CHANGELOG.md` is up to date

Every user-visible change — a new subcommand, a new flag, a behavior
change, a fix — gets a bullet under `## Unreleased` in `CHANGELOG.md`,
written as part of the change itself, not reconstructed later from
memory or git log. This is what makes cutting a release
(`docs/practice/release-process.md`) mechanical rather than an
archaeology exercise: by the time a release is due, `## Unreleased`
should already be accurate and complete, just waiting to be renamed and
dated.

Repo-internal changes (CI workflows, refactors with no user-visible
effect, doc-only fixes) don't need an entry — the changelog is for users
of `devroom`, not a commit log.

## 2. The specs are up to date

`docs/specs/*.md` describes actual, current behavior — not what was
planned, and not what used to be true. When a change alters what a
subcommand does, its spec gets updated as part of the same change, not
as a follow-up.

This includes cross-references between specs (e.g. `tui.md` describing
what each key dispatches to) — check both ends of a link when one side
changes.

## 3. `just test` passes

gofmt, `go vet`, unit tests, and the functional test suite
(`tests/functest.sh`) all pass — run locally before considering the work
finished, not left for CI to catch first. `just test` runs all four.

## Related

- `docs/practice/releasable.md` — the broader, project-wide checklist
  this per-change bar feeds into before a release.