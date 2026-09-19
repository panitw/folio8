---
title: 'Cover colour in the golden corpus and retire the two unused style codes'
type: 'chore'
created: '2026-09-17'
status: 'done'
baseline_commit: '627a93248e9848d5bb501f790abd305640d7ae85'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/specs/spec-client-libraries/SPEC.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** No golden fixture declares text `color` or a `border`, so the four-target byte-identity check has never rendered coloured ink or strokes (DW-147). The public codes `STYLE_COLOR_INVALID` and `STYLE_LINE_SPACING_INVALID` are specific codes that no consumer branches on (D-7.8.2). Both gaps must close before story 2 freezes the API at `folio-go/v1.0.0`.

**Approach:**
- Add a signed golden fixture, `fixtures/colour-strokes/`, registered everywhere a golden must be.
- Remove both codes from the registry, the public constants and the docs.
- Line spacing collapses into `TEMPLATE_FIELD_INVALID` at its one load site.

## Boundaries & Constraints

**Always:**
- **Render-neutral.** Every existing golden hash stays identical.
- **What the fixture declares.** At least:
  - text `style.color` on text and on a table `headerStyle`
  - `border.color` with partial `edges` on a text element
  - a `rect` and a `line` stroked in a non-black colour
  - a table with `rules.color` and `altRowBackground`
  - one filled `background`

  Every colour differs from `#000000`, and `input.folio` uses format version `3.1` or later.
- **Fixture registration follows `multi-page-flow` (commit 15871c7):**
  - the template const plus a fixture test with a fresh-process variant
  - a `render_test.go` selector
  - a `matrixDocuments` entry
  - the `matrix.yml` docs list plus the four hash upload paths
  - a `goldenDigestRecord` entry
  - `declaredEpic2GateObligations` entries

  It also follows `declared-variants` for `<slug>_signoff_matrix_test.go`.
- **Sign-off is the owner's.** The agent renders and records the digest but writes no `signoff.json` during implementation. At the done checkpoint the owner examines `expected.pdf`, and the file is written only on their explicit instruction, in the `declared-variants/signoff.json` shape. Until then, the sign-off gate is the declared transient red.
- **Retiring a code** removes its root `DiagCode*` const, its `internal/diag` constant plus its `allCodes` and `dispositions` entries, its pins (`diag_test.go`, `diag_bridge_test.go`), and its census trigger. Rewrite every test and doc that asserts the code. Registry tests stay derived and are not skipped. `TestRegistryIsAdditiveOnly` gets a dated note that D-7.8.2 retired two codes before the tag.
- **Line spacing:** `parse_bands.go:1000` uses `newLoadError`, so the field stays `<prefix>.lineSpacing` with the same element ID and reason.
- **Colour moves to load (owner decision, 2026-09-17).** A `#RRGGBB` predicate lives in `internal/template`, following `linespacing.go`. The loader checks every colour field and fails with `TEMPLATE_FIELD_INVALID`, located at the field (e.g. `style.border.color`, `headerStyle.color`, `rules.color`, `altRowBackground`). Root `parseHexColor` keeps decoding. The render sites can no longer see a bad colour: they become unreachable guards, with no public code. The command-door checks use the same predicate. The owner accepted one behaviour change: a bad colour on an element that never draws (hidden, or an empty table) is now refused at load.
- Keep `docs/*.md` and their `.html` twins in step.

**Never:**
- Write or pre-fill `signoff.json`, or describe the golden as accepted, without the owner's words.
- Keep either code as an alias or a deprecated constant.
- Change which documents render, apart from the colour-at-load ruling above.
- Cut a tag, push, or touch story 2's work.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Bad line spacing | `"lineSpacing": 1000.001` in a file | load fails with `TEMPLATE_FIELD_INVALID`, field `style.lineSpacing` | same message reason as today |
| Bad line spacing, command | inspector sets `lineSpacing` to 0 | same component failure as today | unchanged |
| Bad colour | `"color": "#12345"` on a hidden element | load and `Validate` fail with `TEMPLATE_FIELD_INVALID`, field `style.color` | no render starts |
| Coloured golden | fixture rendered on darwin/linux × amd64/arm64 | one sha256 across all four targets | a mismatch fails `TestCrossTargetByteIdentity` |
| Unsigned golden | fixture recorded, no `signoff.json` yet | the sign-off matrix test fails, naming the owner's step | a declared transient red, not skipped |

</frozen-after-approval>

## Code Map

- `folio-go/internal/diag/diag.go`: constants at :165-173 and :259-301; `allCodes` :438-468 (ours :452, :457); `dispositions` :490-520 (:499, :504); notes at :311-314 and :360-364. `diag_test.go:42,47,81-102`.
- `folio-go/diagnostic.go:261-288`: the root constants. `diag_bridge_test.go:44,49,133`; `diagnostic_registry_census_test.go:78-97,107,151` (size check derived).
- `folio-go/internal/template/parse_bands.go:994-1000`: the line-spacing site. Compare `newLoadError` at :912 and `errors.go:21,257-268`. `render_error.go:94` ("four overriding specific codes" becomes three). `errors_test.go:112-118` needs a different specific code, e.g. `TABLE_FOOTER_SOURCE_FORBIDDEN`.
- `folio-go/internal/template/linespacing.go:1-25`: the precedent for a load-time predicate inside `internal/template` (AD-1: this package cannot import root).
- Colour render sites: `element_box.go:269-271`, `table_frame.go:91-96`, `table_render.go:585-611,711-725`. Command-door checks: `component_commands.go:1754,3760-3770`, `page_setup.go:1688-1711`.
- Colour load fields: `parse_bands.go:525-560` (`altRowBackground`, `headerStyle`, `rules`), :619-623 (`rules.color`), `decodeStyle` :866+ (`color`, `background`, `border.color` at :626, :934, :946, :1072).
- Tests asserting the colour code: `element_ink_test.go:88,219`; `element_box_test.go:341,619`; `table_render_test.go:454-471`; `table_ruled_form_test.go:867`; `table_alternating_row_test.go:169`. Line-spacing tests: `line_spacing_test.go:280,311`; `wasm/cmd/engine/main_test.go:207-217`.
- Docs: `docs/folio-format.md:184,872` / `.html:405,713`; `docs/rendering-library.md:433,436,913,1140,1150` / `.html:576,579,905,1062,1072`. Prose at `fixtures/line-spacing/README.md:78`.
- Fixture template: `multi_page_flow_template.go`, `multi_page_flow_fixture_test.go:256`, `render_test.go:883,1159`, `matrix_test.go:770-790,~2127`, `.github/workflows/matrix.yml:83,134,185,236,295`, `matrix_registration_test.go:40`, `byte_neutrality_test.go:101,707-714,758,1084-1114`, `golden_structural_validity_test.go:90`.
- Sign-off template: `fixtures/declared-variants/signoff.json`, `declared_variants_signoff_matrix_test.go:95,153,189`.
- DW-147 at `deferred-work.md:7003`: mark it resolved by this story after the owner signs.

## Tasks & Acceptance

**Execution:**
- [x] `fixtures/colour-strokes/` + `folio-go/colour_strokes_{template,fixture_test,signoff_matrix_test}.go` + all registration points above -- the new signed-pending golden -- DW-147
- [x] Record the golden with `CGO_ENABLED=0 GOWORK=off GOTOOLCHAIN=go1.26.0 go run ./cmd/folio8 render …` and write the sha to `expected.json`, the README and the second literal -- one digest, every declared site
- [x] `internal/diag`, `diagnostic.go`, pin and census tests -- remove both codes -- D-7.8.2
- [x] `parse_bands.go:1000` + `errors.go` / `render_error.go` comments + `errors_test.go` -- collapse line spacing
- [x] Colour sites + their tests -- move validation to load, with a table-driven load test covering every colour field; rewrite the render-code tests as load tests
- [x] `docs/folio-format.{md,html}`, `docs/rendering-library.{md,html}`, `fixtures/line-spacing/README.md` -- remove both codes; describe where colour now fails
- [x] `folio-designer`: `npm run build:wasm` -- the engine was rebuilt

**Acceptance Criteria:**
- Given the retired codes, when you grep the repo for `STYLE_COLOR_INVALID|STYLE_LINE_SPACING_INVALID|StyleColorInvalid|StyleLineSpacingInvalid` outside `_bmad-output/`, then there are no matches.
- Given the golden corpus, when the existing fixture tests run, then every pre-existing hash is unchanged.
- Given the new fixture, when `go test -tags=matrix -run 'ColourStrokes.*SignOff'` runs before the owner signs, then it fails with instructions and is listed as a declared obligation.

## Spec Change Log

## Review Triage Log

| # | Layer | Finding | Verdict | Route | Evidence |
|---|---|---|---|---|---|
| 1 | blind | Header background #1B2A4A never asserted; heading ink satisfies the rg check | medium | patch | No page-model assertion on header fills; `rg` key collides with e1's ink. Fixed: exactly 3 header cells filled #1B2A4A. |
| 2 | blind, edge | Vacuous semantic checks (note found flag, header labels, rules `== 0`, `want()` continues) | low | patch | Confirmed in `colour_strokes_fixture_test.go`. Fixed: found flag, all three labels, 8 rules, early return. |
| 3 | verification-gap | Colour-by-data wording from `colourLoadError` unasserted | medium | patch | Pre-verified: no test matches the wording. Fixed in `colour_load_test.go` (present iff `{{`). |
| 4 | blind | Placeholder paragraph omits `rules.color` | low | patch | Doc list confirmed incomplete; added to md and html. |
| 5 | blind | Docs identifier floor 55 but count 58 | low | patch | Comment states 58; floor set to 58. |
| 6 | blind | `TestAHeaderColourDiagnosticNamesTheArmItTook` no longer tests cascade provenance | low | patch | Test now checks the load field prefix only; renamed and comment rewritten. |
| 7 | blind | Fixture README data.json row inaccurate | low | patch | Six rows, eleven colours; reworded. |
| 8 | blind, edge | Explicit `null` on `border.color`/`altRowBackground` now refused at load | low | reject | Before, null decoded to `""` and failed `parseHexColor` whenever drawn (`table_render.go:796`, `resolvedBorderColor`); moving that refusal to load is the owner's colour-at-load ruling. |
| 9 | edge | Malformed inert `style.color` on rect/line/image now refused | false | reject | Frozen intent: "The loader checks every colour field"; documented in folio-format. |
| 10 | blind | Nothing announces documents that stop loading | low | reject | Behaviour change is owner-accepted and documented in folio-format; release notes are story 2's scope (Never: touch story 2's work). |
| 11 | blind | No release/upgrade note for removed public constants | low | reject | Changelog = GitHub release notes per tag, owned by story 2. |
| 12 | blind | `IsHexColour` and `parseHexColor` could drift, no tie test | low | reject | Verification-gap layer confirmed identical acceptance today; fix adds a new test for an unlikely edit. |
| 13 | blind | Render guards return uncoded errors | false | reject | Spec: "unreachable guards, with no public code"; every command door checks the same predicate. |
| 14 | blind | Load test covers only `bands.content` | low | reject | All bands share `decodeStyle`/`decodeBorder`; extra cases add breadth for no new path. |
| 15 | blind | `diag_bridge_test.go` "Seven" comment inconsistent | false | reject | The comment is a historical mutation record; the prose entry names the retired code and the count remains true of that run. |
| 16 | edge | Colour arms of `checkStyleStringHasNoPlaceholder` now unreachable | low | reject | No bad outcome: harmless defence behind the loader; no named harm. |
| 17 | blind | Story file does not list the pending sign-off follow-ups | false | reject | Fix edits this build's spec; the sign-off is handled at the done checkpoint. |

## Implementation Notes

- Golden recorded: `fixtures/colour-strokes/expected.pdf`, sha256 `3e88b304a1660bea2e8d8958a35037ded436e2bea37fae9d8f0d2f6309b60d8c` (go1.26.0, darwin/arm64, via `cmd/folio8 render -strict`). No `signoff.json` written; `TestColourStrokesSemanticSignOffIsRecorded` is the declared transient red. Owner signed 2026-09-17 ("Everything looks alright"); `signoff.json` written at their instruction, `signoff` digest site added, DW-147 marked resolved.
- Colour predicate: `internal/template/colour.go` (`IsHexColour`). Checked in `decodeStyle` (`color`, `background`), `decodeBorder` (`color`), `decodeTableRules` (`color`) and `altRowBackground`. A value containing `{{` gets the colour-by-data wording appended to the reason. Render guards now return plain, uncoded errors.
- Command doors: `validPropertyColor` and `canvasPropertyColor` call `template.IsHexColour`.
- `TestDocsGuideNamesEveryExportedIdentifier` floor lowered from 60 to 55: the two retired constants took the census from 60 to 58.
- An explicit `null` `border.color` decodes to `""` (unchanged) and is now refused at load; previously it failed only when the border was drawn.
- Verification spelling: `FOLIO8_MATRIX_TARGET` takes `darwin/arm64`, not `darwin-arm64`.

## Verification

**Commands:**
- `cd folio-go && go vet ./... && go test -count=1 ./...` -- expected: green
- `cd lint && go test -count=1 ./...` -- expected: green (stage-rank and manifest rules)
- `cd folio-go && FOLIO8_MATRIX_TARGET=darwin/arm64 GOTOOLCHAIN=go1.26.0 CGO_ENABLED=0 go test -tags=matrix -run TestTargetRenderHash -count=1 .` -- expected: the colour-strokes hash equals `expected.json`
- `cd folio-designer && npm run build:wasm && npm test` -- expected: green

**Manual checks:**
- The four-target agreement runs only in CI `matrix.yml`. It runs after push, and pushing needs the owner's confirmation.
