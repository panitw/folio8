---
title: 'Resolve a face once, and stop demanding a font set nobody reads'
type: 'feature'
created: '2026-09-20'
status: 'done'
route: 'dispatch'
review_loop_iteration: 1
baseline_commit: '85cdcc0b66178f55834d8003124a571ba6ed865c'
context:
  - '{project-root}/_bmad-output/specs/spec-font-sources-and-embedding/SPEC.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Two halves of one rule are wrong. A chain whose faces the renderer lacks kills the
render with `TEXT_FACE_ABSENT` even when other supplied faces could draw the rune — which turns a
deployment gap into a dead nightly run once a document may legitimately name a face it does not
carry. And the .NET binding refuses to render at all unless a non-empty `FontSet` is passed, even
for a template that embeds every face it names, so the set is unused by construction
([issue #1](https://github.com/panitw/folio8/issues/1)).

**Approach:** Make resolution one rule with three outcomes — an entry that resolves renders and
needs no font set at all; an entry that cannot resolve *with candidates available* is painted in a
coverage-resolved face the renderer does hold and reported by a new warning; an entry that cannot
resolve *with no candidate at all* stays `TEXT_FACE_ABSENT`. No argument check may pre-empt that
rule before resolution has been attempted.

## Decisions

- **D1 — Strict is the default; lenient is opt-in.** Today's `TEXT_FACE_ABSENT` behaviour is what a
  caller gets when they ask for nothing, so no existing integrator changes behaviour and the
  designer cannot regress. Substitution is reached only by a caller that asks for it. The selector
  is a new optional argument on `Render`/`RenderTo`/`Validate`, not a field on `FontSet` —
  `FontSet` stays `map[string][]byte` and every caller that constructs one is untouched.
- **D2 — Pool order is embedded-first, then supplied, each sorted by face name.** The document's
  own embedded assets are searched before the supplied `FontSet`, because an embedded face was
  chosen deliberately by the author and is likelier to match intent; within each arm, faces are
  sorted by name. First face that covers the rune wins.

## Boundaries & Constraints

**Always:**
- The substitution pool is **the faces the renderer was given** — the supplied `FontSet` plus the
  document's own embedded assets. Never "the shipped set": package `folio8` deliberately never
  imports `folio-go/fonts`, so the engine owns no faces. This keeps AD-8's *"pure lookup against
  the supplied FontSet, never a host font query"* intact.
- Substitution is coverage-resolved **per rune**, and its outcome is **deterministic** — identical
  inputs produce identical bytes. Never range a map to pick a face (D2 gives the order).
- Every substitution emits a warning naming the element, the rune, the face requested and the face
  painted. A silent substitution is a defect.
- The designer is untouched by D1: `page_setup.go` still catches `TEXT_FACE_ABSENT` by code so the
  browser learns which face to fetch (`InstallFace`). Confirm this, do not rewire it.
- The three libraries ship the same behaviour at 2.0.0.

**Never:**
- No new font source, no disk read, no designer change, no format change, no version move. Those
  are stories 2–6.
- Do not close issue #1. No `closes`/`fixes`/`resolves` keyword in any commit, PR body or
  changelog — it is closed by hand once `folio-dotnet` publishes.
- Do not touch the absent-**asset-key** load error, `refuseLicenceSignatures`, or subsetting.
- Do not make `FontSet` a struct (D1).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| All-embedded, no fonts | Every chain entry an `asset`; `FontSet` empty or absent | Renders; bytes identical to the same call with any font set | N/A |
| Chain covers the rune | Some member absent, another covers it | Renders as today, no diagnostic | N/A |
| Default, substitutable | No chain member covers the rune; a pool face does; no lenient opt-in | `TEXT_FACE_ABSENT`, unchanged | Located error |
| Lenient, substitutable | Same, lenient requested | Painted in the D2-ordered pool face + new warning | Warning, render completes |
| Lenient, no candidate | No chain member and no pool face covers it | `TEXT_FACE_ABSENT`, unchanged | Located error |
| Lenient, two candidates | Rune covered by an embedded asset and a supplied face | The embedded one wins (D2) | Warning naming it |
| Designer build | Chain names an uninstalled face | `TEXT_FACE_ABSENT` naming the face, as today | Located error |

</frozen-after-approval>

## Code Map

- `folio-go/render.go:1783` `resolveRuneFace` -- walks the chain per rune via `faceCovers` (`:1755`),
  collects absent member names. **The hook point** — substitution goes after this returns not-found.
- `folio-go/render.go:2198-2201` -- raise A, inside `shapeSegments`, gated on
  `len(absentFaces) > 0 && !unicode.IsControl(r)`. Message from `faceAbsentMessage` (`:1895`).
- `folio-go/render.go:2121-2137` -- **copy this precedent.** A non-covering style variant already
  falls back to the entry's base face and warns `TEXT_STYLE_FACE_UNDECLARED`; message built by
  `styleFaceUndeclaredMessage` (`:1979`), de-duped by `coalesceStyleFaceDiags` (`:1945`).
- `folio-go/wrap.go:653-681` -- raise B, `verticalModel`, when `len(metrics) == 0` (no chain member
  supplied at all). ElementID is `""`. **THIS IS THE ARM THE FIRST ATTEMPT GOT WRONG, on this
  spec's own bad instruction — it is NOT the "no candidate" state.** `len(metrics) == 0` means no
  member of the CHAIN is present; the substitution POOL may still hold a covering face. Under
  `FaceFallbackSubstitute` this arm must therefore consult the pool before refusing, exactly as
  `shapeSegments` does, and refuse only when the pool has nothing either. A single-entry chain
  naming an absent face — `{"body": ["Brand Face"]}` — is the spec's own Success-signal case and
  matrix row 4, and it reaches THIS arm, not `shapeSegments`. It currently refuses.
- **The vertical model must account for the face actually painted.** The line box is derived from
  chain-member metrics only, so a substitute taller than every present chain member overflows with
  no clipping warning. Whatever shape the fix takes, the metrics that size the line and the face
  that draws the glyphs must not disagree.
- `folio-go/render.go:1569` `fontCache.get` -- one lookup, two arms: embedded assets first
  (`asset:` prefix, `embedded_face.go:85-88`), then `fs[name]`. **This is the pool's source, and
  its two arms are already D2's two arms.** Cache built at exactly two sites, guarded by
  `font_cache_sites_test.go`.
- `folio-go/render_entry.go:160,:246` and `folio-go/validate.go:52` -- the three entry points that
  gain the optional lenient selector. `Render` checks only `t == nil`; `Validate` checks nothing.
- `folio-go/page_setup.go:1661-1666` -- catches `TEXT_FACE_ABSENT` by code to re-raise for
  `InstallFace` (`internal/wasm/engine.go:273`). Rationale at `:1601-1618`. **Unchanged under D1 —
  verify, do not rewire.**
- `folio-go/internal/diag/diag.go` -- four edits for a new code: const (`~:145`), `allCodes`
  (`:410-438`), `dispositions` (`:463-492`, use `DispositionWarning`), doc comment (`:47-65`).
- `folio-go/diagnostic.go:179-204` -- public mirror pattern for `DiagCode*`.
- `folio-dotnet/src/Folio8/Folio8.cs:181-183` -- `if (fonts.Count == 0)` throw. In shared helper
  `CheckDataAndFonts` (`:169`), so it covers `Render`, `RenderTo` and `Validate`. **Remove.** The
  null check at `:177-179` is a separate concern — **keep it.**
- Nothing to remove in JS, cshared or Go: no emptiness check exists in any of them (verified).
  An empty set already survives marshalling in every binding.

**Tests a new diagnostic code breaks first:** `internal/diag/diag_test.go:25` (`codePins`, needs a
test-owned literal), `diag_bridge_test.go:27` (`diagCodeBridgePins`, literal not `diag.CodeX`),
`diagnostic_registry_census_test.go:181` (`warnings` map needs a real render witness plus a count
bump at `:339`), `diag_no_empty_code_test.go:56`, `public_surface_census_test.go:46-80`,
`docs_examples_test.go:400-422` (exported identifiers must appear verbatim in
`docs/rendering-library.md`, and the `total < 59` floor rises).

**Wording that becomes false** (hand-written; the `.html` twins are maintained in lockstep, no
generator): `folio-dotnet/src/Folio8/FontSet.cs:10-13`, `Folio8.cs:53,88,123` + exception tags at
`:55,91,126`, `docs/folio-dotnet.md:89` (+ `.html:422`), `docs/folio-js.md:68` (+ `.html:385`),
`folio-go/cshared/README.md:157-158`, `_bmad-output/specs/spec-client-libraries/api-surface.md:47-48`.

## Tasks & Acceptance

**Execution:**
- [x] `folio-go/internal/diag/diag.go` -- register the new warning code (const, `allCodes`,
      `dispositions`, doc comment) -- four edits or the registry tests fail.
- [x] `folio-go/diagnostic.go` -- add the public `DiagCode*` mirror -- bindings read codes as data,
      so no JS/.NET list needs updating.
- [x] `folio-go/render_entry.go`, `folio-go/validate.go` -- add the optional lenient selector to
      `Render`, `RenderTo` and `Validate`, defaulting to today's strict behaviour (D1).
- [x] `folio-go/render.go` -- add the D2-ordered pool search after `resolveRuneFace` returns
      not-found, reached only under lenient; emit the warning; leave the no-candidate arm raising.
- [x] `folio-go/render.go` -- add the message constructor beside `styleFaceUndeclaredMessage` and
      coalesce per distinct rune -- matches the existing warning's shape and de-duping.
- [x] `folio-go/cshared`, `folio-go/wasm`, `folio-js`, `folio-dotnet` -- carry the selector across
      each boundary so all three libraries expose it at 2.0.0.
- [x] `folio-dotnet/src/Folio8/Folio8.cs` -- delete the empty-set throw, keep the null check,
      update the three doc comments and drop the `ArgumentException` tags for emptiness.
- [x] `folio-dotnet/test/Folio8.Tests/ArgumentTests.cs` -- replace `RenderRefusesAnEmptyFontSet`
      and `ValidateRefusesAnEmptyFontSet` with tests that an all-embedded template renders on an
      empty set; keep `RenderRefusesNullFonts`.
- [x] `folio-go` tests -- add cases for every I/O Matrix row, including the issue's own evidence:
      for an all-embedded document the font set must not affect output bytes at all.
- [x] docs and doc comments -- update every "omitting fonts is a caller error" site listed in the
      Code Map, `.md` and `.html` twin together.

**Added by review pass 1 (all boxes reset — the code is re-derived):**
- [x] `folio-go/wrap.go` -- make `verticalModel`'s empty-metrics arm pool-aware under lenient, and
      size the line box from the face actually painted -- root cause of review findings #1 and #2.
- [x] `folio-go/face_fallback_test.go` -- a lenient render of a SINGLE-ENTRY absent chain
      (`{"body": ["Brand Face"]}`) must succeed and warn; and a lenient render whose substitute is
      taller than every present chain member must not silently overflow -- matrix row 4 was passing
      only because every test used a two-entry chain with a present member.
- [x] `folio-go/render.go` -- route `buildPageModel`'s variadic through `resolveFaceFallback` so the
      internal seam refuses what the public path refuses -- finding #3.
- [x] `folio-dotnet/src/Folio8/Folio8.cs` -- reject an out-of-set `FaceFallback` with
      `ArgumentException` in the binding, as Go and JS do -- finding #4.
- [x] `folio-go/wasm/cmd/render/main.go` -- type-check the fallback argument before `.Int()` so a
      non-numeric value returns the binding's error instead of panicking -- finding #9.
- [x] `folio-go/render.go` -- make the embedded arm's order match what D2 and every doc comment
      say, or change the wording to match the order; sorting minted `asset:`+hash names is neither
      -- finding #8.
- [x] `folio-js/test`, `folio-dotnet/test` -- cover `renderTo`/`RenderTo` selector forwarding, and
      cover CAP-7 in JS (an all-embedded document rendered from an empty map) -- findings #5, #6.
- [x] `folio-go/face_fallback_test.go` -- a multi-row table under lenient must emit one warning per
      distinct rune, not per row -- finding #7.
- [x] `_bmad-output/specs/spec-client-libraries/api-surface.md`, `docs/rendering-library.{md,html}`
      -- add the `fallback` argument to the recorded API surface and make the registry row's
      disposition read like its siblings -- finding #10.
- [x] Tidy, all trivial: drop the dead `'\n'` clause or its "verbatim" claim (#12); resolve
      `substitutionPool`'s nil guard against `substituteFace` (#11); rename
      `coalesceStyleFaceDiags` now that it carries two codes (#13); fix the cshared comment's
      pointer to `Native.FaceFallback` (#14); fix the "four font sets" comment and give
      `errTooManyFaceFallbacks` an `errors.Is` test (#15); fix the `Substitutee` typo (#16).

**Acceptance Criteria:**
- Given a document embedding every face it names, when rendered with an empty font set, then it
  succeeds and produces bytes identical to the same render with a real font set, with garbage
  bytes, and with a zero-byte face.
- Given a chain naming an unsupplied face and a pool face covering the rune, when rendered with no
  selector, then `TEXT_FACE_ABSENT` is raised exactly as before this change.
- Given the same call with lenient requested, then a complete page set is produced and exactly one
  warning names the element, the rune, the face requested and the face painted.
- Given the same lenient document rendered twice on different machines, then the output bytes are
  identical, and given both an embedded asset and a supplied face cover the rune, the embedded one
  is painted.
- Given the designer's engine build, when a chain names a face it has not installed, then the
  render still refuses with `TEXT_FACE_ABSENT` naming the face, so `InstallFace` can fetch it.
- Given `go test -count=1 ./...` in `folio-go`, then it passes, including
  `diagnostic_registry_census_test.go`, `diag_bridge_test.go`, `public_surface_census_test.go`,
  `docs_examples_test.go` and `byte_neutrality_test.go`.

## Implementation Notes

- **The selector is variadic in Go** (`fallback ...FaceFallback`), which is the
  only shape Go offers for an optional argument. Passing two, or a value
  outside the closed set, is a named error rather than a clamp
  (`resolveFaceFallback`, `folio-go/face_fallback.go`).
- **The selector rides the `fontCache`, not `shapeSegments`' signature.** The
  cache already knows which faces the renderer holds — its two arms ARE D2's
  two arms — and it already reaches every site that resolves a rune to a face,
  so nothing else needed widening. `newDocumentFontCache` gained the parameter;
  `newFontCache` (test fixtures) is the zero value, which is strict.
  `page_setup.go`'s canvas site passes `FaceFallbackStrict` explicitly and says
  why.
- **The substitution arm's guard is the refusal's guard, verbatim.** It fires
  only where `TEXT_FACE_ABSENT` would have, so `TEXT_MISSING_GLYPH` — a chain
  every member of which WAS supplied — is untouched under either selector.
- **One Warning per (element, distinct rune)**, matching the style-variant
  precedent: `coalesceStyleFaceDiags` now carries both codes, safely, because
  the identity it compares is the whole `Diagnostic`. The tests assert four
  Warnings for the five-rune / four-distinct-rune fixture, so the coalescing is
  measured rather than assumed.
- **The C ABI is a breaking change**: `folio8_render` and `folio8_validate`
  gained a trailing `int32 fallback` before their out-parameters, so
  `abiVersion` moved 1 -> 2 and `Native.ExpectedAbiVersion` with it. The JS host
  gained a sixth argument and treats null/undefined as strict.
- **Byte identity for CAP-7 needs a document whose chain names ONLY assets.**
  `fixtures/embedded-font`'s chain is `["Noto Sans", <asset>]`, so supplying
  "Noto Sans" legitimately changes the page — an absent chain member contributes
  no line metrics and a present one does. The Go test therefore builds an
  all-asset document; the .NET test asserts the weaker, true claim (a font set
  the document never consults does not change the bytes) over that fixture.
- **Not done here, deliberately:** no version move (`2.0.0` is a later story),
  and no closing keyword for issue #1 anywhere.

## Spec Change Log

### Pass 1 — 2026-09-21 — the empty-metrics arm was mis-specified

**Triggering finding:** review findings #1 and #2. A chain whose every member is absent refuses
under `FaceFallbackSubstitute` before shaping is reached, so the spec's own Success-signal case —
a single-entry chain naming a brand face, rendered on a host that lacks it — does not work. Proved
empirically with a throwaway probe, not inferred.

**Root cause, and it is this spec's:** the Code Map said of `wrap.go`'s `verticalModel` raise,
*"This is the 'no candidate' state; keep it an error."* That is true under strict and false under
lenient: `len(metrics) == 0` means no CHAIN member is present, which says nothing about the POOL.
The implementer followed the instruction exactly.

**What was amended:** the `wrap.go` Code Map entry now states the distinction and requires the arm
to consult the pool under lenient; a companion entry requires the line box to be sized from the
face actually painted. Ten review findings that would otherwise have been patched into code about
to be replaced are folded in as explicit tasks instead.

**Known-bad state avoided:** shipping a capability that works only for multi-entry chains, with a
matrix row that passes because every test happened to include a present chain member — a green
suite over the exact case the story exists for.

**KEEP — what worked and must survive re-derivation:**
- `FaceFallback` as a **variadic optional argument** with `FaceFallbackStrict` as the zero value,
  and `resolveFaceFallback` as the single place the variadic becomes a value. Not a `FontSet`
  field. Existing callers compile untouched.
- Refusing a second selector and an out-of-set value rather than clamping, with the reasoning
  recorded at the site.
- Deliberately **no `String()` method** on `FaceFallback` — it collides with `Severity.String`
  under `TestFolio8MethodNamesAreInjective`.
- `substitutionPool` / `substituteFace` split, `slices.Sorted` over map keys for determinism, and
  the two-arm embedded-then-supplied order reusing `fontCache.get`'s own precedence.
- Skipping an unparseable candidate rather than raising on it.
- The substitution warning shaped after `styleFaceUndeclaredMessage`, coalesced per distinct rune,
  naming element, rune, face requested and face painted.
- The canvas site pinned explicitly to `FaceFallbackStrict`, with `InstallFace` verified intact.
- The .NET empty-`FontSet` throw deleted with the null check kept and the rationale recorded.
- ABI `1 -> 2` with `Native.ExpectedAbiVersion` moved in lockstep.
- The whole test file's naming style and its per-matrix-row structure.
- The .NET byte-identity test asserting the weaker true claim over `fixtures/embedded-font`, with
  the strong claim proved in Go against a purpose-built all-asset document.

## Review Triage Log

Pass 1 — three layers (blind-hunter, edge-case-hunter, verification-gap). One row per finding.

| # | Finding | Verdict | Evidence |
|---|---|---|---|
| 1 | A chain whose **every** member is absent never reaches substitution: `verticalModel` (`wrap.go:653`) raises `TEXT_FACE_ABSENT` on `len(metrics)==0` before shaping, ignoring the selector. Reported independently by edge-case and verification-gap. | **high** | Verified empirically. A throwaway probe rendering `{"body": ["Brand Face"]}` with `FaceFallbackSubstitute` over the shipped set was refused: *"none of the fallback chain's faces [Brand Face] is present in the supplied FontSet"*. `grep fallback folio-go/wrap.go` returns only message text — the selector never reaches it. This is the spec's own Success-signal case and matrix row 4. |
| 2 | A substituted face's line metrics never enter the vertical model, so a taller substitute can overflow its box with no clipping warning. | **medium** | Same root cause as #1 — `verticalModel` is computed from chain members only and knows nothing of substitution. Grouped with #1. |
| 3 | `buildPageModel`'s variadic takes `fallback[len-1]` and validates nothing, where `resolveFaceFallback` refuses both a second selector and an out-of-set value. | **medium** | Confirmed in the diff. An internal seam that silently clamps what the public path refuses; the next internal caller diverges. |
| 4 | `.NET` casts `(int)fallback` with no validation, so `(FaceFallback)99` surfaces as `InvalidOperationException` from native, where Go returns a named error and JS throws `TypeError` in the binding. | **medium** | Confirmed in the diff. Breaks the spec's "three libraries ship the same behaviour" constraint on the error contract. |
| 5 | `folio-js` has no CAP-7 coverage at all: every new JS case uses a template whose `assets` is `{}`, so nothing renders an all-embedded document from an empty map — yet `docs/folio-js.md` now promises it. | **medium** | Confirmed by reading `folio-js/test/api.test.ts`. The issue-#1 half of the story is untested in the binding the docs advertise it in. |
| 6 | `RenderTo` selector forwarding is unverified in both .NET and JS; Go pins it with `TestRenderToCarriesTheSelector`. | **medium** | Verification-gap demonstrated the mutation: dropping `fallback` from either wrapper leaves every existing test green while a lenient caller gets a refusal. |
| 7 | Table-scoped de-duplication of the new code is never exercised — every test emitting it shapes a single non-table element, so the cross-call memo is unverified. | **medium** | Verification-gap demonstrated: removing the new code from `coalesceStyleFaceDiags` keeps the suite green. A 500-row table would emit 500 duplicate warnings per rune. |
| 8 | `substitutionPool` sorts the **minted** embedded names (`asset:` + 64 hex), so embedded candidates order by content hash, not face name — contradicting D2 and the published wording in Go, .NET and JS. | **medium** | Confirmed against `embedded_face.go:88`. With two embedded covering faces the painted one is unpredictable from the documented rule. |
| 9 | wasm host calls `js.Value.Int()` on the fallback argument without a type check, so a non-numeric value panics instead of returning the binding's error. | **medium** | Confirmed in the diff at `wasm/cmd/render/main.go`. |
| 10 | `api-surface.md` was edited to loosen the fonts rule but never gained the new `fallback` argument, so the client-library contract still describes a `Render`/`Validate` shape that no longer exists. Registry row disposition also inconsistent with its siblings in both doc twins. | **medium** | Confirmed in the diff. |
| 11 | `substitutionPool` guards `cache == nil`; its only caller `substituteFace` then calls `cache.get` unguarded. Reported by all three layers. | **low** | Verified unreachable: every production `shapeSegments` call site passes a real cache (`render.go:851`, `table_render.go:907/1250/1506`, `page_number.go:463`, `page_setup.go:1054/1668`). Real as a misleading invariant, not as a live panic. |
| 12 | The substitution arm's third guard clause `(r != '\n' \|\| breaks == breaksAreDrawn)` is dead — `!unicode.IsControl(r)` already excludes U+000A — while its comment claims the guard is the refusal's "verbatim". | **low** | Confirmed: U+000A is category Cc. Dead clause plus a comment that contradicts the code. |
| 13 | `coalesceStyleFaceDiags` now memoizes two codes but keeps a name and doc opening that claim one. | **low** | Confirmed in the diff. |
| 14 | cshared keep-in-step comment names `Native.FaceFallback`, which does not exist; the managed enum lives in `FaceFallback.cs`. | **low** | Confirmed. A keep-in-step pointer to the wrong file. |
| 15 | Test comment says "four font sets" then lists five; `errTooManyFaceFallbacks` is a declared sentinel no test matches with `errors.Is`. | **low** | Confirmed in the diff. |
| 16 | .NET test method name typo: `SubstituteePaintsAPoolFaceAndSaysSo`. | **low** | Confirmed. Appears in CI output for the headline capability. |

**Routing.** #1 and #2 group on one root cause — the vertical model is computed from chain members
and knows nothing of substitution — and route to **bad_spec**: the Code Map instructed *"This is the
'no candidate' state; keep it an error"*, which is true under strict and false under lenient, and
that instruction is outside the frozen block. A bad_spec entry triggers a loopback, so every entry
below it is moot as written — the code is re-derived. All of #3–#16 are therefore folded into the
amended spec as explicit requirements rather than patched into code that is about to be replaced.

### Pass 2 — after the bad_spec re-derivation

Finding #1 from pass 1 is **resolved**: the single-entry absent chain now renders. Verified with the
same throwaway probe that proved the defect, and `brandChainTemplateJSON` — the fixture most new
tests use — is now that single-entry chain, so the case is covered by construction.

| # | Finding | Verdict | Evidence |
|---|---|---|---|
| 17 | A lenient render widens the line box whenever a chain member is absent, **even when nothing substitutes** — so bytes differ from strict with no diagnostic. | **medium** | Confirmed at `wrap.go:668`: the arm is gated `absent && lenient`, not on a substitution occurring. Accepted as a trade-off (see below); the documentation half routes to patch. |
| 18 | `substitutionPool` is rebuilt and re-sorted **per uncovered rune**, inside the rune loop, and again per element in `chainLineMetrics`. | **medium** | Confirmed. Pure function of `(embedded, fs)`; a large table over a substituting chain repeats the sort and the coverage walk for every rune occurrence. |
| 19 | A `{{page}}` slot under lenient is untested, and `digitTableRun` hard-fails unless shaping yields exactly one 10-glyph segment; its substitutions are also reported nowhere. | **medium** | Pre-verified by the verification-gap layer, which demonstrated the mutation. No fixture with a page slot is rendered leniently anywhere in Go, JS or .NET. |
| 20 | A style variant (bold/italic) is silently dropped when its covering face is substituted — only the typeface change is reported. | **medium** | Confirmed: the substitution arm does not resolve `styled[..]` coverage as the found-path does. |
| 21 | `folio8_validate`'s out-of-set fallback refusal is unverified at the C ABI; only `folio8_render` has a table row. | **medium** | Pre-verified. Clamping validate to strict leaves every Go/JS/.NET test green. |
| 22 | `TestTheSubstitutionPoolIsOrderedByFaceName` asserts vacuously — one carried face, so the ordering loop compares nothing. | **medium** | Confirmed. Reverting the embedded arm to hash order (the pass-1 defect) leaves the suite green. |
| 23 | The line-box/leading side effect is documented in `rendering-library` only, not in the JS or .NET guides. | **medium** | Confirmed. Violates this story's "three libraries ship the same behaviour" constraint. |
| 24 | `rendering-library` claims "one warning per element and distinct character"; table bodies emit two, because `buildPageModel` walks body cells in two passes. | **medium** | Confirmed against `TestSubstitutionIsCoalescedAcrossTableRows`, which itself accepts two. A published claim the code contradicts. |
| 25 | `wrap.go`'s no-candidate message still says "not present in the supplied FontSet", and the adjacent `units <= 0` message says "its present faces" — both now half-true under lenient, where the pool was also consulted and can supply the metrics. | **low** | Confirmed. Misleading rather than wrong-behaved; no test pins either message. |
| 26 | `docs/folio-js.md` says the selector is "an optional fifth argument" for all three; for `renderTo` it is the sixth. | **low** | Confirmed. |
| 27 | `api-surface.md` uses `3a.` as an ordered-list marker, which is not valid Markdown and renders as loose text mid-list. | **low** | Confirmed. |
| 28 | `table_render.go` still declares `var styleFaceSeen` with a one-code comment, though the renamed `coalesceFaceDiags` now carries both. | **low** | Confirmed — the pass-1 rename is half-applied. |
| 29 | .NET `ValidateAcceptsAnEmptyFontSet` asserts only `NotNull`, which cannot fail; its JS twin asserts no `TEXT_FACE_ABSENT`. | **low** | Confirmed. A test that passes whatever the engine reports. |
| 30 | Dead `carried` const in `TestLenientPrefersTheEmbeddedFace`; stray double blank line in `folio-js/src/types.ts`. | **low** | Confirmed. |
| 31 | The wasm host handles a null fallback and `engine.ts` types it `number \| null`, but `fallbackInput` never returns null — two layers own the same default, one unexercised. | **low** | Confirmed. |
| 32 | Embedded arm sorts by `displayName()` (`embedded "Family"`), which D2 and the guides call "face name". | **false** | `displayName()` returns the asset's declared `font.family` behind a constant prefix, so the order *is* family order — this is the pass-1 #8 fix working. Only the word differs; folded into the doc patches. |
| 33 | If every chain member is a carried asset this build cannot parse, `verticalModel` may refuse under lenient without consulting the pool. | **false** (checked, not deferred) | Sent back to be checked rather than deferred. It never reaches that refusal: with text, `shapeSegments` aborts first on the carried-face parse error, byte-identically under both selectors, because `absent` is `!cache.declares(...)` not `!present`; with no text the empty-metrics arm is not reached. Verified across `{"", "Hi"} x {Strict, Substitute}`. |

**Routing.** No `intent_gap` and no `bad_spec`: #17's behavioural half is an accepted trade-off
recorded in `DECISIONS.md` A-7 rather than a spec defect, and every other survivor's smallest fix is
local, adds no public surface, and guards no undemonstrated state. #18–#31 route to **patch**; #32 is
rejected on its refutation; #33 routes to **defer** with what would settle it.

## Design Notes

The existing style-variant fallback at `render.go:2121-2137` is the template for the whole change:
coverage chose an entry, the variant did not cover the rune, so it paints the entry's base face and
warns. The new arm is the same move one level out — the chain did not cover the rune, so paint the
best pool face and warn. Reusing its message constructor and its `coalesceStyleFaceDiags` de-duping
keeps one warning per distinct rune rather than one per glyph.

D1 makes this change **additive rather than breaking** on the render path: an integrator who
upgrades and changes nothing gets byte-identical output. The only behaviour that loosens is the
.NET empty-set refusal, which is a loosening, not a break — code that passed a font set still works.

D2 costs nothing to implement because `fontCache.get` already searches embedded-then-supplied; the
pool search reuses that order rather than inventing one, and only the within-arm name sort is new.

## Verification

**Commands:**
- `cd folio-go && go vet ./...` -- expected: clean.
- `cd folio-go && go test -count=1 ./...` -- expected: all pass. The registry, bridge, census,
  docs-example and byte-neutrality suites are the ones a new diagnostic code breaks first.
- `cd folio-go && go test -count=1 -run 'TestTargetRenderHash' -tags=matrix .` -- expected: pass;
  byte identity across targets is unmoved by this change.
- `cd folio-dotnet && dotnet test` -- expected: pass, with the two rewritten argument tests.
- `cd folio-js && npm test` -- expected: pass; JS has no emptiness check to remove, so this is a
  regression guard only.
- `cd folio-js && ./node_modules/.bin/oxlint && ./node_modules/.bin/tsc -p tsconfig.test.json
  --noEmit` -- expected: both exit 0. Run the binaries DIRECTLY, not via `npm run lint`: this
  environment's command proxy tries to parse the script's output as ESLint JSON and reports a
  spurious failure (`ESLint output (JSON parse failed…)`, exit 2) when both tools are clean.
  There is no `typecheck` script in `folio-js`; an earlier draft of this spec named one in error.
