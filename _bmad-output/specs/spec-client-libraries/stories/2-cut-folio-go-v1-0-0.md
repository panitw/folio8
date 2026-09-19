---
title: 'Cut folio-go/v1.0.0'
type: 'chore'
created: '2026-09-17'
status: 'done'
baseline_commit: '0260ba6099554b118e193d455c1e218e839beedb'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-client-libraries/SPEC.md'
  - '{project-root}/RELEASING.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `folio-go` has never been tagged, so neither client library has a stable version to build against. RELEASING.md names the wrong tag (`v0.1.0`). It relies on a pinned surface census that does not exist and leaves the tag procedure unwritten. `folio8.Version` still reads `0.0.0-dev`.

**Approach:** Make the release true and guarded before it exists:
- pin the public surface
- stamp `Version`, with a test that it agrees with the release RELEASING.md names
- rewrite RELEASING.md around `v1.0.0` with a written procedure
- record discharged preconditions and superseded decisions

Then, and only on the owner's explicit go-ahead, tag the release commit `folio-go/v1.0.0`, push it and publish the GitHub release with `lint/MANIFEST.md` attached.

## Boundaries & Constraints

**Always:**
- **The tag** is `folio-go/v1.0.0`, directory-prefixed per AD-22, annotated, on the commit this story produces.
- **`Version`** is `"1.0.0"`. A test fails if it disagrees with the version RELEASING.md names as released.
- **Pinned census.** Covers package `folio8` and package `fonts`, qualified by kind: funcs, types, consts (including all `DiagCode*`), vars, `Type.Method`, `Type.Field`. The expected list is inline, per repo convention. It goes red naming each addition as unexpected and each removal as gone, and has a vacuity guard.
- **Re-verify the preconditions** on the release tree: `TestManifestUpToDate`, `TestFolio8MethodNamesAreInjective`, plus an independent method-name scan.
- **Decision records are amended by appending** a dated entry, never by rewriting historical text. That covers D-1.1.c's addendum, D-000.78, and DW-3/DW-4/DW-20.
- **Tag-gate rulings (owner decisions).** Record each in the tracker and deferred-work records; none may be dropped silently.
  - **8.4d and 8.4k are released from the tag gate.** They are designer-release and `lint` work, not Go-tag prerequisites. Amend their sprint-status pointer comment. 8.4d still owes its size-budget ruling.
  - **DW-68: v1.0.0 ships the clip.** An aggregate-only over-tall keep-together group keeps rendering clipped with `TABLE_ROW_CLIPPED_HEIGHT`. Record it as a ruling, so reversing it later is a conscious `/v2` choice.
  - **DW-147 gates the tag.** A golden fixture declaring `style.color` and element box strokes, with a recorded human sign-off, must exist first. It is built in story 3. **If it is absent when implementation starts, stop.**
  - **DW-230 is released from the tag gate** and stays OPEN against Story 15.2.
  - **D-7.8.2 gates the tag: retire both style codes first.** The audit found no consumer branching on `STYLE_COLOR_INVALID` or `STYLE_LINE_SPACING_INVALID`; both are only emitted (`element_box.go:271`, `table_frame.go:96`, `table_render.go:713,725`, `internal/template/parse_bands.go:1000`). Both are retired in story 3 before this tag, while removal is still free under AD-14. **If either public constant still exists when implementation starts, stop.** The census then pins the surface without them.
- **Changelog policy: GitHub release notes per tag.** No changelog file in the repo.
- **Golden hashes stay identical.** `Version` reaches no PDF byte, only preview identity and the wasm `RenderResult`.

**Never:**
- Push a commit or tag, or create a GitHub release, without the owner's explicit confirmation after the done checkpoint.
- Change the public API or any render behaviour.
- Add fixtures, or act on DW-68/DW-147, beyond what the Open Questions decide.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Surface grows | an exported identifier is added to `folio8` or `fonts` | census test fails naming it `UNEXPECTED` | N/A |
| Surface shrinks | an exported identifier is removed | census test fails naming it `GONE` | N/A |
| Vacuous census | scan reads the wrong directory, finds 0 | census test fails, never passes empty | N/A |
| Stamp drift | `Version` ≠ RELEASING.md's released version | version-agreement test fails naming both | N/A |

</frozen-after-approval>

## Code Map

- `folio-go/version.go:9`: `const Version = "0.0.0-dev"`. The comment on lines 5-8 names `v0.1.0`. Consumers are `preview_identity.go:20` and `internal/wasm/engine.go:270`; no test pins a preview-identity digest. **Leave every `fixtures/*/expected.json` `folio8GoVersion: "0.0.0-dev"` untouched.** It records the version that produced the golden, and `fixture_test.go:243-251` deliberately does not require it to equal `Version`, so the bump reddens nothing. Rewriting 32 records would falsify their provenance.
- `folio-go/diag_bridge_test.go:73,150`: the closest census pattern (go/ast plus an inline pin list, checked both ways, with a vacuity guard). `lint/internal/rules/glyphidcensus_test.go:275-288` (`reportSetDiff`) shows the UNEXPECTED/GONE wording.
- `folio-go/docs_examples_test.go:336-395`: `exportedIdentifiers` returns unqualified, deduplicated names, so a same-named field and type collapse into one. Don't reuse it for the census. Its floor is now exactly 58 (story 3 retired two constants), with no slack.
- **Surface to pin:** funcs LoadTemplate, ParseTemplate, ParameterReferences, Render, RenderTo, SerializeTemplate, Validate; types Data, Diagnostic, FontSet, Params, RenderError, Result, Severity, Template; consts: the 27 `DiagCode*` the tree declares after story 3 (commit 0260ba6 removed `DiagCodeStyleColorInvalid` and `DiagCodeStyleLineSpacingInvalid`; both preconditions are verified met), LocaleTableVersion, MaxParameterReferenceNameLength, Version, SeverityWarning, SeverityError; methods Severity.String, RenderError.Error, RenderError.Unwrap; fields Diagnostic.{Severity,Code,ElementID,DataPath,Message}, RenderError.{Diagnostic,Err}, Result.{Bytes,Diagnostics}; `fonts.Shipped`.
- `RELEASING.md`: lines 13-20 hold the tag heading and the "after Epic 6" trigger. Lines 24-63 hold the three preconditions, where #2 still says "40 items". Lines 65-89 hold the designer force-upgrade section, which stays as it is. Lines 91-97 list what remains unwritten: version stamping, changelog, tag command, matrix record.
- `lint/internal/manifest/releasing_test.go:28,89-90`: fails unless RELEASING.md contains the literal `folio-go/v0.1.0`. Change it with the doc.
- User docs naming no release: `README.md:64` (`go get …@main`) and `:191` ("v0.1.0 has not been cut"); `docs/rendering-library.md:23-37` (no tag yet, `@main`, pseudo-version), `:891` and `:1193` (`"0.0.0-dev"`), `:1201` (`cmd/folio8@main`); the `.html` twin at `:282-284` and `:1113`.
- Code comments naming `v0.1.0`: `render.go:2331`, `render_arch_test.go:82,363`, `diagnostic.go:49-50` (also says `Version = "0.0.0-dev"`), `internal/template/parse.go:717,741`.
- Records: D-1.1.c addendum at `folio-mvp-decision-log.md:1324-1343`; D-000.78 at `:12027`. DW-3 at `deferred-work.md:154` (retired into RELEASING.md #1). DW-4 at `:753` (trigger re-affirmation, census re-measure, lead checkpoint). DW-20 at `:1872` (injectivity backstop).
- No CI workflow triggers on tags or releases, and there is no release tooling. `gh` is available. The only existing tag is `pre-email-rewrite`.

## Tasks & Acceptance

**Execution:**
- [x] `folio-go/public_surface_census_test.go` -- add the pinned census per Boundaries -- makes RELEASING.md's safety claim true
- [x] `folio-go/version.go` plus a version-agreement test -- `Version = "1.0.0"`, and a test comparing it to the version RELEASING.md names as released -- a stamp and the doc cannot drift
- [x] `RELEASING.md` and `lint/internal/manifest/releasing_test.go` -- retitle around `folio-go/v1.0.0`; replace the "after Epic 6" trigger with the re-affirmed decision naming what is inside the release; record each precondition's re-verified result; write version stamping, the changelog policy (GitHub release notes per tag), the exact tag/push/release commands (`lint/MANIFEST.md` attached), and how the green `matrix.yml` run for the release commit is recorded -- the procedure 15.3 owed
- [x] `README.md`, `docs/rendering-library.md` and `.html` -- install against `folio-go@v1.0.0`, state that the API is frozen, `Version` reads `"1.0.0"` -- a new integrator follows released instructions
- [x] code comments listed in Code Map -- name `v1.0.0` -- no stale tag name in shipped source
- [x] decision records -- append dated amendments: D-1.1.c/D-000.78 superseded by the `v1.0.0` owner decision; DW-3/DW-4/DW-20 discharged with evidence; the tag-gate rulings from Boundaries recorded against 8.4d/8.4k (plus their sprint-status pointer), DW-68, DW-147, DW-230 and D-7.8.3's before-the-tag set -- nothing owed evaporates

**Acceptance Criteria:**
- Given the release tree, when `grep -rn "v0\.1\.0"` runs over `README.md`, `RELEASING.md`, `docs/` and `folio-go/` (non-`_bmad-output`), then it finds nothing.
- Given the release tree, when the full suite, the matrix-tagged suite and lint run with `-count=1`, then all pass and every golden hash is unchanged.
- Given the owner's go-ahead after the done checkpoint, when the RELEASING.md procedure runs, then `folio-go/v1.0.0` exists on `origin` at the release commit, and a GitHub release for it carries `lint/MANIFEST.md`.

## Implementation Notes

- Census pins 60 items (folio8: 7 funcs, 8 types, 32 consts, 3 methods, 9 fields; fonts: Shipped). Red-proved by hand: a probe exported func reads UNEXPECTED, a bogus pin reads GONE, pointing the fonts scan at an empty directory hits the vacuity Fatal. Build constraints are ignored by the scan on purpose.
- Version agreement reads RELEASING.md's single "**Released version:** `folio-go/vX.Y.Z`" line; red-proved by setting it to v1.0.1.
- Preconditions re-verified on the release tree: TestManifestUpToDate and TestFolio8MethodNamesAreInjective pass; independent go/ast scan: 41 methods, 41 distinct names.
- fixtures/*/expected.json folio8GoVersion left at 0.0.0-dev (provenance). Designer unit-test mocks using '0.0.0-dev' (src/test/pdf-fixture.ts, engine-*.test.ts, evidence-rail.test.tsx) are stand-ins not tied to the engine and are untouched; the e2e `preview-evidence-rail.spec.ts` reads the real engine `folio8.Version` through the wasm build, so it now expects '1.0.0'.
- The docs name no external release-notes URL: the guides are served offline, and none of them links off-site today.

## Spec Change Log

## Review Triage Log

| # | Layer | Finding | Verdict | Route | Evidence |
|---|---|---|---|---|---|
| 1 | verification-gap | `e2e/preview-evidence-rail.spec.ts:110` pins `0.0.0-dev` from the real engine | high | patch | Rail text comes from `folio8.Version` via wasm; ci.yml e2e would go red on the release commit. Fixed to `1.0.0`; the spec passed locally. |
| 2 | blind | RELEASING.md "Epics 7–16" contradicts the other records | low | patch | sprint-status: epic-15 in-progress, epic-17 done. Made consistent. |
| 3 | blind, edge | Census comment, RELEASING #2 and docs overclaim (names only; "never breaks") | medium | patch | Census pins `<pkg> <kind> <name>` only. Wording corrected to what is pinned and to standard semver. |
| 4 | blind, edge | A new non-internal package under folio-go/ escapes the census | medium | patch | Only `.` and `fonts` were scanned; `folio-go/wasm` was public until story 1. Added `TestOnlyRootAndFontsAreImportableLibraryPackages`, red-proved with a probe. |
| 5 | edge | `gh run watch` without `--exit-status`; ambiguous run lookup; HEAD drift; ci.yml not required | medium | patch | `gh run watch` exits 0 on failure without the flag. Procedure now pins `$SHA`, checks origin/main, and requires ci.yml and matrix.yml green. |
| 6 | blind, edge | lint `releasing_test.go` hard-codes `v1.0.0`, breaking at the next release | medium | patch | Now reads the single Released version line. |
| 7 | blind | No post-publish check or bad-release recovery | low | patch | Added a proxy check and the fix-forward/`retract` rule. |
| 8 | blind | `render_arch_test.go` "Until then" contradicts present tense; over-long line | low | patch | Reworded and reflowed. |
| 9 | edge | Census misses members promoted through an unexported embedded type | low | reject | No exported struct in `folio8` embeds a type today; the fix adds type resolution for an unlikely edit. |
| 10 | edge | ParenExpr receiver silently dropped | low | reject | gofmt never produces a parenthesised receiver type in this repo. |
| 11 | edge | Version test rejects pre-release suffixes | low | reject | No pre-release is planned; widening the regex adds untested branches. |
| 12 | blind | RELEASING.md mixes a reusable procedure with v1.0.0 specifics | low | reject | Restructuring the document is beyond a direct correction; the dated v1.0.0 sections are labelled. |
| 13 | blind | Guide lost its verified-against provenance and the GOPROXY hint | low | reject | The old provenance named a pseudo-version that no longer applies; the tag resolves through the proxy normally. |
| 14 | blind | Release notes are a placeholder; breaking list incomplete | low | reject | Notes are written at release time under the owner's go-ahead; the breaking list is in RELEASING.md. |
| 15 | blind | `parse.go` second-door comment ignores the /v2 rule | false | reject | The comment already requires the narrowing to be priced when added, which after the tag includes /v2. |
| 16 | blind | Story Code Map mixes baseline and result; no verification results | false | reject | Fix edits this build's spec; the Code Map describes the baseline by design. |
| 17 | blind | A `//go:build ignore` `package main` file would fail the census | low | reject | No such file exists in the scanned directories. |

## Design Notes

**Why the tag is not an implementation task.** It must sit on the commit that contains this story's own changes, and that commit only exists at step 5. Pushing a tag is also irreversible for anyone who fetches it. So implementation stops at a reviewed, committed release tree. After the done checkpoint, and only on explicit confirmation, the RELEASING.md procedure runs verbatim:

```
git push origin main                      # CI and matrix.yml run on the release commit
gh run watch <matrix run id>              # the four-target run must be green before tagging
git tag -a folio-go/v1.0.0 <release-commit> -m "folio-go v1.0.0"
git push origin folio-go/v1.0.0
gh release create folio-go/v1.0.0 lint/MANIFEST.md --title "folio-go v1.0.0" --notes-file <notes>
```

The tag is pushed only after the matrix run is green, so a failing run needs no tag deletion. The release notes name the matrix run URL. They also carry the breaking changes since `main` that integrators on pseudo-versions would hit: the designer surface left the public API (story 1); `STYLE_COLOR_INVALID` and `STYLE_LINE_SPACING_INVALID` were retired into `TEMPLATE_FIELD_INVALID`; and colour is now refused at load (story 3).

## Verification

**Commands:**
- `cd folio-go && go test -count=1 -run 'TestPublicSurface|Version' -v .` -- expected: census and version-agreement tests pass
- `cd folio-go && go test -count=1 -skip '^TestCorpusMeetsP6ExerciseFloors$' ./...` -- expected: all pass
- `cd folio-go && go test -count=1 -tags=matrix -skip '^TestCorpusMeetsP6ExerciseFloors$|^TestShippedFacesReproduceFromUpstream$|^TestCrossTargetByteIdentity$|^TestFMAProbeDiverges$' ./...` -- expected: all pass
- `cd lint && go test -count=1 ./...` -- expected: all pass, including TestManifestUpToDate
- `grep -rn "v0\.1\.0" README.md RELEASING.md docs folio-go` -- expected: no output

**Manual checks (if no CLI):**
- Temporarily add then remove an exported identifier: the census goes red naming it.
