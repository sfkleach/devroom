# Release Process

This is the step-by-step checklist for cutting a devroom release. For the
*design* rationale behind the release pipeline itself (why GoReleaser, why
a linux-only build matrix, why `CHANGELOG.md` instead of a
conventional-commit auto-changelog), see `docs/decisions/` — this doc is
the "how", not the "why".

## 1. Versioning

devroom uses SemVer with a `v` prefix (`v0.1.0`), matching
`.goreleaser.yml`'s `{{.Tag}}` and `.github/workflows/release.yml`'s
`v*.*.*` tag trigger.

`go.mod`'s `go 1.27.0` line is the *minimum Go toolchain version*, not a
release version — Go modules have no release-version field of their own.
The git tag is the only source of truth for a release's version number.

Until `1.0.0`: bump the minor version for any user-visible change, the
patch version for a fix-only release. `1.0.0` marks a stable CLI/config
contract.

## 2. Pre-flight checklist

1. Confirm `main` is green: `gh run list --workflow=build-and-test.yml`.
2. Edit `CHANGELOG.md`: rename `## Unreleased` to `## [X.Y.Z] -
   YYYY-MM-DD`, and add a fresh, empty `## Unreleased` section above it
   for whatever comes next.
3. Run `just release-check` — it runs the full `just test` suite and then
   checks that `CHANGELOG.md`'s top section is no longer "Unreleased",
   inferring the version from that top section rather than being told it.
4. Commit the `CHANGELOG.md` edit and push to `main`. Wait for
   `build-and-test.yml` to go green on that exact commit before tagging —
   tagging a commit CI hasn't verified defeats the point of the `test`
   job that `release.yml` also runs.

## 3. Cutting the release

```
git tag -a vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

Pushing the tag triggers `.github/workflows/release.yml`:

1. `test` job: `just test` again, as a safety net.
2. `release` job: `just check-changelog "$GITHUB_REF_NAME"` — this time
   *with* the tag, so it must match exactly (unlike step 2's inferred
   check) — then `just extract-changelog "$GITHUB_REF_NAME"` to produce
   the release body, then `goreleaser` builds and publishes.

Watch it with `gh run watch` or `gh run list --workflow=release.yml`.

## 4. Post-release verification

- The GitHub Release has the right assets (linux/amd64 and linux/arm64
  tarballs, `checksums.txt`), the right body (matches the CHANGELOG
  section), and the right name.
- `go install github.com/sfkleach/devroom/cmd/devroom@vX.Y.Z` actually
  resolves and builds — the real end-to-end proof it works for a user.

## 5. Never re-tag a pushed version

Once a version tag has been pushed, treat it as immutable — even if the
release build failed, even if the GitHub Release doesn't look right, even
if a mistake turns up five minutes later. If something is wrong, cut a new
patch version instead of deleting and re-pushing the same tag.

This isn't just tidiness: Go's module checksum database can make
re-tagging actively break things for anyone who fetches the module. See
[Appendix: why re-tagging is unsafe](#appendix-why-re-tagging-is-unsafe)
below.

## 6. Troubleshooting

- **Something failed after the tag was pushed** — whether in the `test`
  job, the `release` job, or partway through GoReleaser itself (e.g. some
  assets uploaded, others not): the tag already exists on `origin` the
  moment `git push origin vX.Y.Z` succeeds, regardless of what happens in
  CI afterward. Do *not* assume it's safe to delete and reuse the same
  version number just because the failure was early or nothing looks
  "published" yet — which job failed doesn't change the actual risk.
  Check the appendix's `sum.golang.org` lookup first:
  - **No entry**: safe to delete the tag (and the GitHub Release, if one
    exists) — `git tag -d vX.Y.Z && git push origin :vX.Y.Z` — fix the
    problem, and retry with the *same* version.
  - **An entry exists**: don't reuse the version number under any
    circumstances. Fix the problem and cut a new patch version instead.
- **Testing `.goreleaser.yml` changes without publishing**: `goreleaser
  build --snapshot --clean` (build only, no release) or `goreleaser
  check` (config validation only) run locally, without needing a tag or
  a `GITHUB_TOKEN`.

## 7. Quick reference: `just` recipes

| Recipe | What it does |
|---|---|
| `just test` | gofmt check, `go vet`, unit tests, functional tests. |
| `just build` | Local production build, version ldflags from `git describe`. |
| `just check-changelog TAG` | Verify `CHANGELOG.md`'s top section matches `TAG` exactly. Used by `release.yml` once the tag is known. |
| `just extract-changelog TAG` | Print `CHANGELOG.md`'s section for `TAG`, used as the GitHub Release body. |
| `just release-check` | `just test`, then verify `CHANGELOG.md`'s top section isn't "Unreleased" — infers the version from that section itself, for use *before* a tag exists. |

---

## Appendix: why re-tagging is unsafe

A GitHub Release and a git tag are not where the actual danger lives — a
third system is: the Go module checksum database (`sum.golang.org`),
which `go install`/`go get` verify against by default
(`GOSUMDB=sum.golang.org`).

- A **GitHub Release** is UI and uploaded assets wrapped around a tag.
  Deleting it touches nothing Go-related — `go install` never talks to
  GitHub's Release API, only to the git repo via the module proxy.
- A **git tag** being pushed is necessary but not sufficient for the
  danger — a tag nothing has ever fetched via a Go tool is, in principle,
  still safe to delete and re-push. But that's very hard to verify from
  outside: GitHub's own dependency-graph indexing, proxy pre-fetching,
  and other automated crawlers can fetch a freshly-pushed tag within
  seconds.
- The actual point of no return: the first time anything — a user, a CI
  runner, an automated crawler — runs `go install
  .../devroom@vX.Y.Z` (or `@latest`, once it's the newest tag) through
  the default public proxy, that content gets a permanent,
  cryptographically signed entry in the append-only checksum log at
  `sum.golang.org`. After that, re-tagging the same version number with
  *different* content makes every subsequent fetch fail with a checksum
  mismatch for anyone hitting the old cached entry — there is no way to
  un-publish that log entry.

Because the trigger can happen before you'd notice, treat a tag as
immutable **from the moment it's pushed to `origin`**, not from when a
GitHub Release appears — even though in this repo's pipeline the two
happen within the same `release.yml` run, so the distinction rarely
matters here in practice.

To check whether a specific version was actually locked in:

```
curl https://sum.golang.org/lookup/github.com/sfkleach/devroom@vX.Y.Z
```

A response means it's permanent, regardless of what GitHub shows.
