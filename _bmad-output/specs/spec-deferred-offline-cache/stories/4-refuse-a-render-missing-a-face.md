---
title: 'Refuse a render that needs a face the font set does not carry'
type: 'feature'
created: '2026-09-19'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '4fb2268af555d8a159f66b92f85dc52135f9bb8d'
context: ['{project-root}/_bmad-output/specs/spec-deferred-offline-cache/SPEC.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** When a font chain names a face the supplied `FontSet` does not carry, the engine
skips that entry. If another entry covers the rune, that is correct and stays. If none does, the
rune is dropped under a `TEXT_MISSING_GLYPH` **Warning** and a PDF ships with the text silently
gone — the engine cannot know whether the absent face would have covered it. A caller passing
`fonts.Shipped()` wholesale never reaches this, which is why it has gone unnoticed; a `folio-js`
or `folio-dotnet` caller supplying a partial set can reach it today, and story 5 makes the
designer reach it too. The refusal must exist before that.

**Approach:** Distinguish "a face this document declares was never supplied" from "every declared
face was supplied and none covers this rune". The first becomes a located, coded refusal naming
the face; the second keeps `TEXT_MISSING_GLYPH` and its Warning exactly as it is. The existing
uncoded refusal for a chain with no present face at all joins the same code.

## Boundaries & Constraints

**Always:**
- The two conditions stay distinguishable. A rune that genuinely no supplied face covers is still
  a Warning with the rune dropped — refusing it would fail legitimately-unsupported text.
- A chain member that is absent but whose work another entry does is still skipped silently. An
  absent *styled* variant still falls back to its base face with no diagnostic (`render.go:1228`).
- The refusal is located: element id, and the data path of the style field the chain came from.
- The message reuses the existing wording at `render.go:1598` — "face %q is not present in the
  supplied FontSet" — rather than inventing a second phrasing for the same fact.

**Decisions (owner-settled, do not revisit):**
- The code is **`TEXT_FACE_ABSENT`** — the supply failure, kept distinct from the existing
  `TEXT_STYLE_FACE_UNDECLARED`, which is about what the chain declares rather than what the
  caller supplied. Keeps the `TEXT_*` prefix shared by the rest of the family.
- **It refuses for every caller, in every language.** This is a breaking change for `folio-js` and
  `folio-dotnet` callers who pass a partial font set: the render host compiles in no fonts, so
  such a caller gets a Warning and a PDF today and a hard refusal after this. That caller was
  shipping PDFs with characters silently missing, which is the outcome byte-identity exists to
  prevent. No opt-in flag, no per-language leniency — and it is called out in the release notes.

**Never:**
- Do not remove any face from `fonts.Shipped()`, and do not touch the designer's wasm host — that
  is story 5. This story changes engine behaviour only.
- No strictness flag, option, or environment switch. One behaviour everywhere.
- Do not move a byte of any golden fixture. Six carry Han text with PDF hashes pinned across three
  languages. They supply the full font set, so none of them should reach the new path; if one
  does, stop and report rather than re-pinning.
- Do not widen `chainLineMetrics`' tolerance of an absent member — it feeds the vertical model,
  and a member that cannot appear must not constrain line height.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Absent member, another covers | chain `["Noto Sans","Noto Sans SC"]`, SC absent, Latin text | Renders. No diagnostic. | N/A |
| Absent member, none covers | same chain, SC absent, Han text | Refusal naming `Noto Sans SC` | Coded Error, no PDF |
| No member present at all | chain `["Noto Sans SC"]`, SC absent, any text | Refusal naming `Noto Sans SC` | Coded Error, no PDF |
| All present, none covers | full set supplied, rune in no face | `TEXT_MISSING_GLYPH` Warning, rune dropped | unchanged |
| Absent styled variant | `Noto Sans` present, `Noto Sans Bold` absent, bold text | Renders with base face, no diagnostic | N/A |
| Absent member, rune is `\n` | SC absent, text contains newline | unchanged from today's newline suppression | N/A |

</frozen-after-approval>

## Code Map

- `folio-go/render.go:1533` `declares` — the one place that already knows a name is absent rather
  than uncovering. The signal to propagate; do not change what it answers.
- `folio-go/render.go:1745` `faceCovers`, `:1765` `resolveRuneFace` — the chain walk. `faceCovers`
  returns `false` for an absent member, collapsing it with "present but no glyph". These need to
  report the absence alongside the miss.
- `folio-go/render.go:1983` `shapeSegments`, emit at `:2091` — where `found == false` becomes
  `TEXT_MISSING_GLYPH`. The fork goes here: absent-seen → refusal, else the Warning unchanged.
  Note `seenMissingRunes` coalescing at `:2081` and the `'\n'` suppression.
- `folio-go/render.go:1831` `missingGlyphMessage` — names the whole chain. The new message must
  name the *absent member*, not the chain.
- `folio-go/wrap.go:630` `verticalModel` — the existing uncoded `fmt.Errorf` for no present face
  ("no line height can be derived from it"). Give it the new code and a location.
- `folio-go/render_error.go:74` `newRenderError(code, elementID, dataPath, err)` — the constructor
  to use. Warnings are struct literals; errors go through this.
- `folio-go/render.go:1196` `fontChain` — where `el.Style.Value.FontFamily` becomes a chain; the
  source of the `style.fontFamily` data path.
- **Registration, all five places:** `folio-go/internal/diag/diag.go` const block (~:71-387),
  `allCodes` at `:393`, `dispositions` at `:439` (this one is an **Error**); pins in
  `folio-go/internal/diag/diag_test.go:25` plus the exact count at `:86`; public bridge in
  `folio-go/diagnostic.go`; pins in `folio-go/diag_bridge_test.go` (`TestEveryPublicDiagCodeBridge…`
  at `:148`).
- **Docs contract:** `docs/rendering-library.md:1139-1163` is an exhaustive table — a new code needs
  a row, and `docs/rendering-library.html` (~:1162) mirrors it verbatim.
  `folio-go/docs_examples_test.go:397` fails if an exported identifier is absent from either, and
  carries an identifier floor at `:418` that must be raised.
- **Existing tests that must stay green:** `folio-go/chain_face_names_test.go:731` (absent member
  skipped, later entry draws — asserts zero diagnostics);
  `folio-go/vertical_model_test.go:422` part (1) asserts the *current* uncoded message text and
  will need updating to the coded refusal; `folio-go/ac4_coverage_test.go:101` and
  `folio-go/missing_glyph_coalesce_test.go` pin the Warning that must survive.
- No `folio-js` or `folio-dotnet` code change: neither mirrors the code set (`folio-js/src/types.ts:8`
  and `folio-dotnet/src/Folio8/Diagnostic.cs:57` both type `code` as a plain string).

## Tasks & Acceptance

**Execution:**
- [x] `folio-go/internal/diag/diag.go` -- add the code with an **Error** disposition and register it
      in `allCodes` -- every produced diagnostic must carry a registered code.
- [x] `folio-go/internal/diag/diag_test.go`, `folio-go/diag_bridge_test.go` -- add the pins and raise
      the exact count -- the registry tests assert the set both ways.
- [x] `folio-go/diagnostic.go` -- add the public `DiagCode*` bridge constant -- the public surface.
- [x] `folio-go/render.go` -- have `faceCovers`/`resolveRuneFace` report that a chain member was
      absent, and fork `shapeSegments`' `found == false` branch on it -- this is the behaviour change.
- [x] `folio-go/render.go` -- add the absent-face message naming the face -- distinct from
      `missingGlyphMessage`, which names the chain.
- [x] `folio-go/wrap.go` + its caller -- give `verticalModel`'s no-present-face error the same code
      and a location -- it is the same author fault one level up.
- [x] `folio-go/vertical_model_test.go` -- update part (1) to assert the coded, located refusal --
      it currently pins the uncoded message text.
- [x] new test file under `folio-go/` -- cover every row of the I/O matrix, each with a negative
      control -- the Warning-vs-refusal fork is the whole story and must fail if collapsed.
- [x] `docs/rendering-library.md` + `docs/rendering-library.html` -- add the table row and raise the
      identifier floor in `folio-go/docs_examples_test.go:418` -- docs are the source of truth.
- [x] `RELEASING.md:117` -- add a bullet to the standing breaking-change list: a partial `FontSet`
      that leaves a rune uncovered now refuses instead of warning -- there is no changelog file, so
      this list is the repository's record of what an integrator will hit.

**Acceptance Criteria:**
- Given a chain whose members are all supplied, when a rune is covered by none of them, then a
  `TEXT_MISSING_GLYPH` Warning is emitted, the rune is dropped, and a PDF is produced — unchanged.
- Given a chain with an absent member, when every rune is covered by the present members, then the
  render succeeds with no diagnostic.
- Given a chain with an absent member, when a rune is covered by no present member, then the render
  fails with the new code, the message names the absent face, and no PDF is returned.
- Given a chain with no member present at all, when the element is rendered, then the render fails
  with the same code and a located diagnostic rather than an uncoded error.
- Given the repository's committed fixtures, when the full suite runs, then no golden PDF hash
  changes and no fixture reaches the new refusal.

## Implementation Notes

**The fork, as built.** `faceCovers` now returns `(covers, absent, err)` — `absent` is
`cache.declares`' answer, which the boolean used to swallow. `resolveRuneFace` returns EVERY absent
chain member, in chain order, and only when coverage was not located, so an absent member whose
work a later entry does is still skipped in silence. All of them are named, because the engine
cannot know which one would have covered the rune — that ignorance is the refusal's whole reason
for existing. `shapeSegments` forks on that list inside the existing `'\n'` suppression, and
additionally never refuses on a `unicode.IsControl` rune: no font carries U+0009, so the refusal's
justification is false for a control character exactly as it is for U+000A. Non-empty means a
`newRenderError(DiagCodeTextFaceAbsent, elementID, "style.fontFamily", …)`; empty leaves
`TEXT_MISSING_GLYPH` byte-for-byte as it was — Warning behaviour is untouched, control character or
not. The styled-arm
`faceCovers` call discards the new return deliberately — an absent variant is Story 11.2's
condition, not this one.

**`verticalModel`'s location is the data path only.** The task named "wrap.go + its caller". The
error is coded where it is minted, in `verticalModel`, because `newRenderError` preserves the
message byte-for-byte (`RenderError.Error()` returns what it wraps) and the four production callers
plus thirty-odd test call sites would all have had to grow an `elementID` parameter to buy a second
location for a condition `shapeSegments` now refuses first, located at the element, on every input
with drawable text. `fontFamilyDataPath` is a shared constant so the two refusals cannot drift to
two spellings of one field.

**Which layer answers moved, and two tests moved with it.**
`TestChainOfOnlyUnusableEntriesProducesTheExistingLocatedError` and part (1) of
`TestVerticalModelErrorPathsAreUnreachableThroughRender` both pinned the uncoded
"none of the fallback chain's faces" message. A chain with no present member now refuses in
`shapeSegments` on its first drawable rune, so both assert the coded, located refusal instead. The
`verticalModel` arithmetic stays red-proved at its own seam by
`TestVerticalModelRefusesAChainWithNoPresentFace`, which now also pins the code; its message
assertion is untouched and still passes, which is the evidence that coding it broke nothing a
caller was reading. Through `Render` the `len(metrics)==0` path is now reachable only where shaping
has nothing to refuse — an element whose whole text is line feeds a caller consumes.

**A census trigger the task list did not name.** `TestDiagnosticRegistryErrorCensus` derives its
worklist from the registry, so a new Error code fails the suite until it has a real production
trigger. Added: `missingGlyphTemplateJSON` rendered against a `FontSet` that withholds the chain's
only face.

**No fixture moved.** `go test ./... -run 'Golden|Parity|Fixture' -count=1` is green and
`git status` shows no change under any fixture directory. `folio-js` (102 tests) and `folio-dotnet`
(141 tests) pass unchanged, as expected — neither mirrors the code set.

## Spec Change Log

**I/O matrix row 5, "Absent styled variant" — "no diagnostic" is not what the shipped engine
does.** The row expects "Renders with base face, no diagnostic", and the Boundaries repeat it. The
engine in fact emits Story 11.2's `TEXT_STYLE_FACE_UNDECLARED` **Warning** for an absent variant —
that is its documented absence arm (FR57), it predates this story, and the cited `render.go:1228`
comment describes only the SHAPING fallback, not the diagnostic `shapeSegments` emits a few lines
later. Silencing it would be an unrelated change to a shipped Warning's output, which this story's
Never list rules out in spirit. `TestAbsentStyledVariantStillFallsBackSilently` therefore asserts
what this story actually owes the row: the render succeeds with the base face, and nothing it
reports is `TEXT_FACE_ABSENT`. Its negative control withholds the BASE face from the same chain and
document, so the row still fails if the fork collapses.

## Review Triage Log

| # | Finding | Verdict | Evidence | Route |
|---|---------|---------|----------|-------|
| 1 | An uncovered control character (U+0009 tab) in text whose chain has an absent member now fails the whole render | **high** | Probed: `"Ada\tAdeline"`, chain `[Noto Sans, Noto Sans SC]`, SC withheld → refusal, 0 PDF bytes. Same document with the full set → a `TEXT_MISSING_GLYPH` Warning and a PDF. A tab is absent from output by design, exactly the reason the spec already carved out U+000A; no font carries it, so the "the absent face might have covered it" justification does not hold | patch |
| 2 | With more than one absent chain member the message names only the first, which may be a face that could never have covered the rune | **medium** | Probed: chain `[Noto Sans Thai, Noto Sans SC]`, both withheld, U+6C49 → names `"Noto Sans Thai"`. The message's whole purpose is to say which face to supply; this sends the author to supply an irrelevant one and be refused again | patch |
| 3 | `docs/rendering-library.md:376` states the missing-face error "is not a `*folio8.RenderError`, so handle it in your fallback branch" — now false | **high** | Read at the cited line. After this change it is a `*RenderError` carrying `TEXT_FACE_ABSENT`. The diff updated the code table but not the prose stating the same rule. `docs/` is the source of truth | patch |
| 4 | `docs/rendering-library.md:374` and `docs/folio-format.md:221` still say an uncovered character is always the `TEXT_MISSING_GLYPH` Warning | **medium** | Read at both lines. True only when every declared face was supplied | patch |
| 5 | `docs/folio-js.md` and `docs/folio-dotnet.md` do not mention the refusal, and these are what the broken callers read | **medium** | Both show missing-glyph handling as a warning branch. `RELEASING.md` is currently the only place a JS/.NET integrator is told | patch |
| 6 | `TestAbsentStyledVariantStillFallsBackSilently` can pass vacuously | **medium** | Its assertions live entirely inside `for _, d := range res.Diagnostics`; with zero diagnostics the body never runs, though its comment states as fact that the engine emits `TEXT_STYLE_FACE_UNDECLARED` here | patch |
| 7 | The `verticalModel` arm the diff itself declares still reachable has no through-`Render` test | **medium** | The old assertion on "no line height can be derived from it" was replaced; grep finds it in no test. Reachable via an element whose whole text is consumed line feeds — the one path where the unlocated (empty `ElementID`) diagnostic escapes to a caller | patch |
| 8 | `TestNoCommittedFixtureReachesTheAbsentFaceRefusal` does not measure what it claims | **medium** | Comment says "six committed goldens … hashes pinned across three languages" and calls itself AC5's direct statement; the body iterates five `baselineAcceptanceFixtures`, compares no hash, and infers "unchanged" from a non-nil byte slice | patch |
| 9 | The new doc block above `faceCovers` describes `resolveRuneFace`'s contract, citing a `found` result `faceCovers` does not have | **low** | Read at the cited block. `faceCovers` returns `(covers, absent, err)`. Direct correction, no complexity added | patch |
| 10 | `RELEASING.md` new bullet leaves a terminal period mid-list | **low** | The list is semicolon-separated with a period on the last item only | patch |
| 11 | A hidden element (`visibleIf` false) whose text needs an absent face fails the whole render | **false** | `render.go:854-860` states the rule deliberately: "a hidden element with a broken font chain still fails the render exactly as a" visible one (AD-24, R2/AC7). `shapeSegments` has always run for hidden elements and its errors have always propagated. Established design, not introduced here | — |
| 12 | The refusal prints the face with raw `%q` while the chain goes through `faceDisplayName`, so a carried `asset:<hex>` name would be printed verbatim | **false** | `declares` returns true for any embedded name, so `absent` is only ever set for a name that is neither embedded nor in the `FontSet`. A carried face can never be the absent one | — |
| 13 | The message names the element twice (suffix "in element e1" plus the caller's "element e1:" prefix) | **false** | `missingGlyphMessage` has carried the identical "in element e1" suffix under the same caller prefix since before this story. Consistent with the sibling diagnostic, not a defect introduced here | — |
| 15 | PATCH ROUND REGRESSION (found by my own verification, not the agent's): editing docs/folio-js.md and docs/folio-dotnet.md without their .html twins broke both libraries' twin-equality tests | **high** | `folio-js/test/docs.test.ts` diverged at token 1720, `Folio8Tests.DocsTests.TheTwinsPublishTheSameTextPunctuationIncluded` at token 2458. Caused by my patch instruction, which named the .html mirror for rendering-library but not for these two. Fixed; both suites now green | patch |
| 16 | docs/folio-format.html had silently desynced from its .md twin in the first round, policed by no test at all | **medium** | Found by hand while fixing #15. `folio-go`'s `guideTwins` asserts only that exported identifiers are NAMED in both files, never that their text matches; nothing covers the format pair. The .html was fixed; the missing guard is deferred | patch + defer |
| 14 | `addCanvasTableLabelLines` (`page_setup.go:1632`) swallows `shapeSegments` errors, silently degrading the refusal on the designer canvas path | **medium** | Real, but unreachable today: the designer host passes `fonts.Shipped()` wholesale. Story 5 is what makes that host supply a partial set | defer |

## Design Notes

The rule the fork implements, stated once:

```
rune uncovered  +  some chain member was absent  ->  refuse, naming that member
rune uncovered  +  every chain member present    ->  TEXT_MISSING_GLYPH (Warning)
```

The engine cannot ask whether an absent face would have covered the rune, which is exactly why the
absence has to be reported rather than assumed harmless. `declares` (`render.go:1533`) already
computes the distinguishing bit; today it is thrown away inside `faceCovers`' boolean.

`verticalModel`'s existing refusal is the same fault caught one level higher — with no member
present there are no metrics to derive a line height from. Giving both the one code means an author
sees the same diagnostic whether they omitted the only face or one of several.

## Verification

**Commands:**
- `cd folio-go && go build ./...` -- expected: clean
- `cd folio-go && go test ./...` -- expected: all pass, including the registry pin tests, the
  docs-identifier test, and every golden fixture unchanged
- `cd folio-go && go vet ./...` -- expected: clean
- `cd folio-js && npm test` -- expected: parity fixtures unchanged
- `cd folio-dotnet && dotnet test` -- expected: parity fixtures unchanged
- `cd folio-go && go test ./... -run 'Golden|Parity|Fixture' -count=1` -- expected: pass, confirming
  no committed PDF hash moved
