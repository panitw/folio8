# Releasing folio8

This document is the procedure for cutting a folio8 release. It exists because
**a release with no written procedure is not a release** — the same rule
D-000.58 applies to gate procedures, one level up.

It covers the Go engine module, `folio8-go`. The designer's own version and
force-upgrade policy are at the end.

**Released version:** `folio8-go/v1.0.0`

`TestVersionAgreesWithReleasingDoc` (`folio8-go/version_test.go`) reads the line
above and fails unless `folio8.Version` equals it, so the stamp in the code and
the release this document names cannot drift apart.

## `folio8-go/v1.0.0`

**Cut after SPEC-client-libraries stories 1 and 3, before the folio-js and
folio-dotnet bindings are built** (owner decision, 2026-09-16, recorded in
`_bmad-output/specs/spec-client-libraries/SPEC.md`). This supersedes the
earlier "first tag after Epic 6" trigger (D-000.78): Epics 7–14, 16 and 17 and
Epic 15's completed stories all landed after that decision without amending it, and the bindings need a stable version to
build against.

The tag is a `v1` release, not a `v0` pre-release, because it commits to semver. **Cutting it is
irreversible**: D-1.1.c fixes the public API at it, and any later breaking
change needs a `/v2` import path that every caller edits.

**Inside the release:** everything on `main` at the release commit — Epics 1–14,
16 and 17, Epic 15's completed stories (15.1, 15.2a, 15.2b), and the client-library
prerequisites: the designer surface moved behind `internal/` (story 1), a signed
colour-and-strokes golden with both unused style codes retired (story 3), and
this procedure (story 2). **v1.0.0 freezes render and validate only.**

**Released from the tag gate (owner rulings):** 8.4d (size budget) and 8.4k
(licence exception) are designer-release and `lint` work, not Go-tag
prerequisites; DW-230 stays open against Story 15.2. **DW-68 is ruled: v1.0.0
ships the clip** — an aggregate-only over-tall keep-together group keeps
rendering clipped with `TABLE_ROW_CLIPPED_HEIGHT`, and making it fatal is now a
`/v2` choice. **Gated the tag, and met:** DW-147 (the `fixtures/colour-strokes/`
golden, owner sign-off in its `signoff.json`) and D-7.8.2 (`STYLE_COLOR_INVALID`
and `STYLE_LINE_SPACING_INVALID` retired).

## Release checklist

Each precondition below was re-verified on the release tree on 2026-09-17.

### 1. The third-party licence manifest ships with the release

**Obligation:** the committed manifest at **`lint/MANIFEST.md`** is attached to
the release as an artifact.

AD-26's substance already ships and is guarded continuously: every module in
the resolved graph carries a resolved licence, an unresolvable one fails the
build, and `TestManifestUpToDate` (`lint/internal/manifest/manifest_test.go`)
fails if the committed file drifts from what the generator produces. What this
line adds is **publication**: `gh release create` below attaches the file.

Regenerate with `cd lint && go run ./cmd/genmanifest` (from inside `lint/`,
which has its own `go.mod` — not from the repo root).

*Discharges DW-3, retired at Epic 4 planning.* **Re-verified:**
`TestManifestUpToDate` passes.

### 2. The public API surface is deliberate

**Obligation:** the exported surface of packages `folio8` and `fonts` is
reviewed as a whole before it freezes, because D-1.1.c fixes it at the tag.

**Measured for v1.0.0: 60 items** — package `folio8` has 7 funcs, 8 types,
32 consts (27 `DiagCode*`, `SeverityWarning`, `SeverityError`, `Version`,
`LocaleTableVersion`, `MaxParameterReferenceNameLength`), 3 methods and
9 struct fields; package `fonts` has `Shipped`. The surface was reviewed as a
whole at story 1's spec checkpoint, which cut it from 287 items.

**The live trigger is `TestPublicSurfaceMatchesTheFrozenV1Census`**
(`folio8-go/public_surface_census_test.go`). It pins every exported identifier by
package, kind and name — not signatures, types or constant values — fails
naming an addition `UNEXPECTED` and a removal `GONE`, fails if a scan finds
nothing, and fails if any importable package other than `folio8` and `fonts`
appears under `folio8-go/`. This line is what a release reads, not
what fires.

*Discharges DW-4's surface re-measure.*

### 3. The call-graph walker is precise, or its precondition still holds

**Obligation:** `buildFolio8CallGraph` (`folio8-go/render_arch_test.go`) resolves
methods by name alone. Before a tag, either replace it with a `go/types`
version in `lint`, or confirm its precondition — that no two receiver types in
package `folio8` declare the same method name — still holds.

*Backstop for DW-20. The live trigger is the pinned injectivity assertion
beside the walker itself; it fires at the commit that creates the collision,
which is years earlier than anyone reads this file.* **Re-verified:**
`TestFolio8MethodNamesAreInjective` passes, and an independent `go/ast` scan of
package `folio8`'s non-test sources found 41 methods under 41 distinct names.

## Cutting a release

### Version stamping

`folio8.Version` (`folio8-go/version.go`) is the release's version without the
tag prefix: tag `folio8-go/v1.0.0` ↔ `Version = "1.0.0"`. Bump it, and the
**Released version** line at the top of this document, in the release commit
itself. Nothing else holds a copy: `TestVersionAgreesWithReleasingDoc` fails if
`Version` and that line disagree, and `TestReleasingDocNamesTheGuardedManifest`
(`lint`) reads the tag from the same line.

`Version` reaches no PDF byte — only the designer's preview identity and the
wasm `RenderResult` — so bumping it moves no golden hash. Each fixture's
`expected.json` keeps the `folio8GoVersion` that produced it; those records are
provenance and are **not** rewritten on a bump.

### Changelog

**GitHub release notes per tag.** There is no changelog file in the
repository. The notes for each tag state what changed since the previous
release, every breaking change an integrator will hit, and the matrix run
below.

For `v1.0.0` the notes carry the breaking changes since `main` that
integrators on `@main` pseudo-versions would hit:

- the designer surface (`Canvas`, `ApplyComponentCommand`, the projection
  types and the `folio8-go/wasm` package) left the public API;
- `STYLE_COLOR_INVALID` and `STYLE_LINE_SPACING_INVALID` were retired into
  `TEMPLATE_FIELD_INVALID`;
- an invalid colour is now refused when the template loads.

### The cross-target hash matrix

A release commit is tagged only after both `ci.yml` (Build, vet, and
guardrails — the full test suites and the designer e2e) and `matrix.yml`
(Cross-target byte identity) are **green on that exact commit**. The matrix
run is recorded in the release notes as its URL and the commit SHA it ran on.
It cannot be recorded in the repository, because the run only exists after the
commit does.

### The commands

Run from the repository root, on `main`, with the release commit at `HEAD` and
everything above done. **Pushing the tag is irreversible for anyone who
fetches it; do not run this without the owner's explicit go-ahead.**

```sh
SHA=$(git rev-parse HEAD)             # the release commit; use it for every step below
git push origin main                  # ci.yml and matrix.yml run on the release commit
git fetch origin && test "$(git rev-parse origin/main)" = "$SHA"   # what was pushed is what gets tagged

# Look up each workflow's push run for exactly this commit; each must list one run.
gh run list --workflow ci.yml     --commit "$SHA" --event push --limit 1
gh run list --workflow matrix.yml --commit "$SHA" --event push --limit 1
gh run watch <ci run id>     --exit-status   # non-zero exit on a red run: stop
gh run watch <matrix run id> --exit-status   # all four targets must be green

git tag -a folio8-go/v1.0.0 "$SHA" -m "folio8-go v1.0.0"
git push origin folio8-go/v1.0.0
gh release create folio8-go/v1.0.0 lint/MANIFEST.md --title "folio8-go v1.0.0" --notes-file <notes>

# Confirm the module proxy serves the tag.
GOPROXY=https://proxy.golang.org go list -m github.com/panitw/folio8/folio8-go@v1.0.0
```

The tag is pushed only after both runs are green, so a failing run needs no
tag deletion: fix forward on `main` and start again. **A published tag is never
moved or deleted.** A bad release is fixed forward with a new patch version,
adding a `retract` directive to `folio8-go/go.mod` for the bad one if needed. The tag is
directory-prefixed (AD-22) because the module lives in `folio8-go/`; Go resolves
`go get github.com/panitw/folio8/folio8-go@v1.0.0` from it.

## Choosing the designer version, and forcing an upgrade

`folio8-designer/package.json`'s `version` is the number an open tab compares
itself against, and its **MAJOR is the entire force-upgrade policy**:

| Bump | What an open tab does |
| --- | --- |
| patch / minor (`1.0.0` → `1.2.3`) | Offers a dismissible "Update available". The running release stays usable; "Later" ends the asking for that tab. |
| **major** (`1.9.9` → `2.0.0`) | Blocks with "Update required". No dismissal, no Escape, no Later. |

Nothing else promotes a release to mandatory. The release `id` is a content
hash, so it changes on every deploy and answers "are these the same bytes",
which is the wrong question for "must this author stop what they are doing".
Forcing is therefore an **authored act with a diff**: bump the major, and no
build step can do it for you.

**A forced upgrade never discards a document.** Activation reloads the tab, so
a blocked author with unsaved changes is offered a save and no upgrade button
at all; the upgrade appears once the work is safe. Before publishing a major,
be satisfied that stopping every open tab is worth it — a wrong major cannot be
recalled from tabs that already took it.

**Tabs notice within about 15 minutes**, and immediately on refocusing the tab
or regaining network. A tab left open across a deploy no longer waits for the
browser's own ~24h service-worker check.
