---
title: 'Cut folio8-go/v1.0.0'
type: 'chore'
created: '2026-09-17'
status: 'draft'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-client-libraries/SPEC.md'
  - '{project-root}/RELEASING.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `folio8-go` has never been tagged, so neither client library has a stable version to build against. RELEASING.md names the wrong tag (`v0.1.0`). It relies on a pinned surface census that does not exist and leaves the tag procedure unwritten. `folio8.Version` still reads `0.0.0-dev`.

**Approach:** Make the release true and guarded before it exists:
- pin the public surface
- stamp `Version`, with a test that it agrees with the release RELEASING.md names
- rewrite RELEASING.md around `v1.0.0` with a written procedure
- record discharged preconditions and superseded decisions

Then, and only on the owner's explicit go-ahead, tag the release commit `folio8-go/v1.0.0`, push it and publish the GitHub release with `lint/MANIFEST.md` attached.

## Boundaries & Constraints

**Always:**
- **The tag** is `folio8-go/v1.0.0`, directory-prefixed per AD-22, annotated, on the commit this story produces.
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

- `folio8-go/version.go:9`: `const Version = "0.0.0-dev"`. The comment on line 7 names `v0.1.0`. Consumers are `preview_identity.go:20` and `internal/wasm/engine.go:270`; check for any test that pins a preview-identity digest.
- `folio8-go/diag_bridge_test.go:73,150`: the closest census pattern (go/ast plus an inline pin list, checked both ways, with a vacuity guard). `lint/internal/rules/glyphidcensus_test.go:275-288` (`reportSetDiff`) shows the UNEXPECTED/GONE wording.
- `folio8-go/docs_examples_test.go:336-395`: `exportedIdentifiers` returns unqualified, deduplicated names, so a same-named field and type collapse into one. Don't reuse it for the census. Its floor is exactly 60, with no slack.
- **Surface to pin:** funcs LoadTemplate, ParseTemplate, ParameterReferences, Render, RenderTo, SerializeTemplate, Validate; types Data, Diagnostic, FontSet, Params, RenderError, Result, Severity, Template; consts: the `DiagCode*` set as it stands after the style-code retirement (27 today, once `DiagCodeStyleColorInvalid` and `DiagCodeStyleLineSpacingInvalid` are gone; pin whatever the tree then declares), LocaleTableVersion, MaxParameterReferenceNameLength, Version, SeverityWarning, SeverityError; methods Severity.String, RenderError.Error, RenderError.Unwrap; fields Diagnostic.{Severity,Code,ElementID,DataPath,Message}, RenderError.{Diagnostic,Err}, Result.{Bytes,Diagnostics}; `fonts.Shipped`.
- `RELEASING.md`: lines 13-20 hold the tag heading and the "after Epic 6" trigger. Lines 24-63 hold the three preconditions, where #2 still says "40 items". Lines 91-97 list what remains unwritten: version stamping, changelog, tag command, matrix record.
- `lint/internal/manifest/releasing_test.go:28,89-90`: fails unless RELEASING.md contains the literal `folio8-go/v0.1.0`. Change it with the doc.
- User docs naming no release: `README.md:64` (`go get …@main`) and `:191` ("v0.1.0 has not been cut"); `docs/rendering-library.md:23-37` (no tag yet, `@main`, pseudo-version), `:891` and `:1193` (`"0.0.0-dev"`), `:1201` (`cmd/folio8@main`); the `.html` twin at `:282-284` and `:1113`.
- Code comments naming `v0.1.0`: `render.go:2331`, `render_arch_test.go:82,363`, `diagnostic.go:50`, `internal/diag/diag.go:364`, `internal/template/parse.go:717,741`.
- Records: D-1.1.c addendum at `folio-mvp-decision-log.md:1324-1343`; D-000.78 at `:12027`. DW-3 at `deferred-work.md:154` (retired into RELEASING.md #1). DW-4 at `:753` (trigger re-affirmation, census re-measure, lead checkpoint). DW-20 at `:1872` (injectivity backstop).
- No CI workflow triggers on tags or releases, and there is no release tooling. `gh` is available. The only existing tag is `pre-email-rewrite`.

## Tasks & Acceptance

**Execution:**
- [ ] `folio8-go/public_surface_census_test.go` -- add the pinned census per Boundaries -- makes RELEASING.md's safety claim true
- [ ] `folio8-go/version.go` plus a version-agreement test -- `Version = "1.0.0"`, and a test comparing it to the version RELEASING.md names as released -- a stamp and the doc cannot drift
- [ ] `RELEASING.md` and `lint/internal/manifest/releasing_test.go` -- retitle around `folio8-go/v1.0.0`; replace the "after Epic 6" trigger with the re-affirmed decision naming what is inside the release; record each precondition's re-verified result; write version stamping, the changelog policy (GitHub release notes per tag), the exact tag/push/release commands (`lint/MANIFEST.md` attached), and how the green `matrix.yml` run for the release commit is recorded -- the procedure 15.3 owed
- [ ] `README.md`, `docs/rendering-library.md` and `.html` -- install against `folio8-go@v1.0.0`, state that the API is frozen, `Version` reads `"1.0.0"` -- a new integrator follows released instructions
- [ ] code comments listed in Code Map -- name `v1.0.0` -- no stale tag name in shipped source
- [ ] decision records -- append dated amendments: D-1.1.c/D-000.78 superseded by the `v1.0.0` owner decision; DW-3/DW-4/DW-20 discharged with evidence; the tag-gate rulings from Boundaries recorded against 8.4d/8.4k (plus their sprint-status pointer), DW-68, DW-147, DW-230 and D-7.8.3's before-the-tag set -- nothing owed evaporates

**Acceptance Criteria:**
- Given the release tree, when `grep -rn "v0\.1\.0"` runs over `README.md`, `RELEASING.md`, `docs/` and `folio8-go/` (non-`_bmad-output`), then it finds nothing.
- Given the release tree, when the full suite, the matrix-tagged suite and lint run with `-count=1`, then all pass and every golden hash is unchanged.
- Given the owner's go-ahead after the done checkpoint, when the RELEASING.md procedure runs, then `folio8-go/v1.0.0` exists on `origin` at the release commit, and a GitHub release for it carries `lint/MANIFEST.md`.

## Implementation Notes

## Spec Change Log

## Review Triage Log

## Design Notes

**Why the tag is not an implementation task.** It must sit on the commit that contains this story's own changes, and that commit only exists at step 5. Pushing a tag is also irreversible for anyone who fetches it. So implementation stops at a reviewed, committed release tree. After the done checkpoint, and only on explicit confirmation, the RELEASING.md procedure runs verbatim:

```
git tag -a folio8-go/v1.0.0 <release-commit> -m "folio8-go v1.0.0"
git push origin main folio8-go/v1.0.0
gh release create folio8-go/v1.0.0 lint/MANIFEST.md --title "folio8-go v1.0.0" --notes-file <notes>
```

## Verification

**Commands:**
- `cd folio8-go && go test -count=1 -run 'TestPublicSurface|Version' -v .` -- expected: census and version-agreement tests pass
- `cd folio8-go && go test -count=1 -skip '^TestCorpusMeetsP6ExerciseFloors$' ./...` -- expected: all pass
- `cd folio8-go && go test -count=1 -tags=matrix -skip '^TestCorpusMeetsP6ExerciseFloors$|^TestShippedFacesReproduceFromUpstream$|^TestCrossTargetByteIdentity$|^TestFMAProbeDiverges$' ./...` -- expected: all pass
- `cd lint && go test -count=1 ./...` -- expected: all pass, including TestManifestUpToDate
- `grep -rn "v0\.1\.0" README.md RELEASING.md docs folio8-go` -- expected: no output

**Manual checks (if no CLI):**
- Temporarily add then remove an exported identifier: the census goes red naming it.
