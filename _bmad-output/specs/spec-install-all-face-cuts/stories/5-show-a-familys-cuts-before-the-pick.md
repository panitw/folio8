---
title: 'Show a family''s cuts before the pick'
type: 'feature'
created: '2026-09-19'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '617999d15d5e05fe18584818b96a020eb1521890'
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Stories 1–4 made a pick install every cut a family publishes, but the Add font dialog
still says nothing about which cuts that will be. A Regular-only family — 947 of the 1,270 offered —
is indistinguishable from a four-cut one until the author applies it and presses **B**. CAP-6 closes
that: every listed family shows its cuts, and what is shown must match what installing yields.

**Approach:** Carry per-family cut data to the browser from the tier that already knows it — the
committed snapshot projected at the emit step for web families, the 107 committed catalogue faces for
the local tier, and the persisted `FamilyCensus` for families already installed — and draw it as one
more meta span in the row, owned by `font-browser-model.ts`.

## Boundaries & Constraints

**Always:**
- The cut vocabulary is the closed RIBBI set `font-source.ts:204` already declares: `Regular`,
  `Bold`, `Italic`, `Bold Italic`. Never a raw upstream style count, never a fifth weight.
- ⚠ **Three spellings exist and must be reconciled, not re-minted:** stored/census faces use
  `Bold Italic`, catalogue faces use `BoldItalic`, chain keys use `boldItalic`. The bridges are
  `CATALOGUE_CUT_STYLES`/`catalogueCutOf` (App.tsx:5942, :5957) and `RIBBI_CUT_NAMES`/`ribbiCutOf`
  (App.tsx:5923). Reuse them; if the display needs them outside App.tsx, move them rather than copy.
- `scripts/build-font-index.mjs` stays pure and offline — a function of the committed
  `font-index.json` alone. No network, no snapshot regeneration (that breaks the `d6d51f1` pin and
  fires DW-166 trigger 1).
- Every user-facing string lives in `font-browser-model.ts`, never inline in `FontBrowser.tsx`.
- Re-measure the core Brotli total against `warnCoreCacheBytes` (6,422,528) as well as
  `maximumCoreCacheBytes` (6,553,600). Headroom to the warning is only **13,872 bytes**.

**Never:**
- No `Most styles` sort arm and no `designer` field. CAP-6 asks for a display, not a sort; those
  halves of D-16.R.33 R3 stand on their own reasoning and are untouched.
- No network call, no `METADATA.pb` fetch, and no speculative pre-fetch to build the display.
- No change to the install path, the store, the census, the catalogue build, or the document.
- Do not touch `weightLine`. Story 3 already corrected it to "N families · every cut each one
  publishes, up to four"; this change does not make it false again.
- **No fourth spelling and no second copy of a bridge.** If the display needs
  `CATALOGUE_CUT_STYLES`/`catalogueCutOf` or `RIBBI_CUT_NAMES`/`ribbiCutOf` outside `App.tsx`, MOVE
  them to a module both can import. A duplicated authority is the defect this epic has already paid
  for twice.
- **R3's reversal is argued on its criterion in the shipped code comment, not only in this spec** —
  never on the +947 bytes.
- **If the real build crosses `warnCoreCacheBytes`, stop and ask.** Do not move the warning; story 3
  added it deliberately.

## Decisions (owner-ruled or settled at re-plan)

- **D1 — Named cuts, spelled out** (owner). "Regular · Bold · Italic · Bold Italic". Not badges, not
  a count. **Measured at 1024px in chromium 1217: fits all 1,270 offered families, tightest slack
  58.9px** (Fira Sans Extra Condensed). The grid card is a separate box and is Q-A below.
- **D2 — Owner-widened fence: the dialog stops offering families with no upright Regular** (owner).
  `Buda`, `Molle` and `UnifrakturCook` publish no static 400 upright, so `font-source.ts:572` refuses
  them outright while `addableFromTheWeb` offers them. **This widens the story's fence beyond CAP-6's
  display and the owner authorized it**, because a cut line advertising a Bold no pick can deliver is
  the exact failure CAP-6 exists to prevent. Derive the condition from the projected cut set
  including `Regular` — the same predicate `publishedCuts`/`cutDeclarations` (`font-source.ts:229`,
  `:247`) applies — never a list of three names.
- **D3 — Fidelity is already reconciled; this story adds no reconciliation** (settled at re-plan).
  The hard requirement's named case cannot arise: `addableFromTheWeb` excludes every family declaring
  axes, and for non-variable families every upstream face is static (measured 0 variable faces across
  90 families; 30/30 variable-declaring families served VF files). The variable refusal is a
  per-family property the dialog's own filter applies *before* listing. Past that, the row
  **self-corrects**: on install its tier flips `web` → `stored` and the display reads the census,
  which records what actually happened. Any residual mismatch already reaches the author through
  story 2's four-state sentence (`unpublished` / `unfetched` / `unusable` / `unchecked`).
- **D4 — An installed family shows `published` minus permanently-refused cuts** (settled at
  re-plan). That set is literally "what installing this family yields on this machine", which is
  CAP-6's own wording, and it is `censusIsComplete`'s (`font-store.ts:230`) two components taken
  apart. A transiently-missing cut is still a yield and stays shown; the button already says there is
  more to fetch, because `FontBrowser.tsx:112` drives it from `familyIsComplete`, not
  `familyIsInstalled`. A permanently-refused cut is never shown — nothing will ever deliver it.
- **D6 — Named cuts in the grid card too** (owner, on Q-A). One vocabulary everywhere. Nothing
  clips, and the dialog never says two different things about one family.
- **D7 — Owner-widened fence: `.font-browser-row-foot` wraps** (owner, after the build). `App.css`
  was not among this story's named files; the owner authorized it on the post-implementation
  measurement. At `nowrap` the card's six flex items default to `min-width: auto` and shrink to
  min-content, so the cut line rendered as a **35px-wide, 65px-tall column with the `·` separators
  orphaned onto their own lines** and the specimen collapsed 57px → 18px. Wrapping lets the line
  break *between* spans: **one 148px line, an 82px foot, a 13px specimen**. The list row's head is
  deliberately excluded — it has 710px and nothing to wrap. D6 was ruled on a line COUNT, which
  understated the shape; this corrects it.
- **D5 — No second disclosure marker on a web row.** The row already states its tier
  ("downloaded when you install it") immediately beside the cut line. A second marker saying the cuts
  are a snapshot claim would be a second authority ageing on its own schedule.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Four-cut web family | snapshot `styles` ⊇ 400/700/400i/700i — 103 rows | "Regular · Bold · Italic · Bold Italic" | N/A |
| Regular-only web family | projects to Regular alone — 947 rows | "Regular" | N/A |
| Many-weight web family | `styles` 100…900, non-variable | Regular + Bold only | never names a 5th weight |
| Web family, no upright 400 | Buda / Molle / UnifrakturCook | **not offered at all** (D2) | `webFamilies` 1,273→1,270; `addableFamilyCount` 1,304→1,301 |
| Committed family | catalogue faces, 23 fams×4, 7×2, Roboto×1 | the cuts actually shipped | N/A |
| Installed, complete | census published ⊆ held | every published cut | N/A |
| Installed, transient gap | one cut refused `permanence:'transient'` | cut still shown; button stays `+ Install` | N/A |
| Installed, permanent refusal | one cut refused `permanence:'permanent'` | that cut **not** shown | `censusIsComplete` still true |
| Installed, no census row | `census === undefined` (pre-story-1 install) | held cuts only | never claims an unchecked cut |
| Variable family | `variable: true` — 537 rows | not listed at all | unchanged |

</frozen-after-approval>

## Code Map

- `folio-designer/font-index.json` — committed snapshot, 1,811 rows, each already carrying `styles`
  (`trimFamily:74`). **Do not regenerate.**
- `folio-designer/scripts/build-font-index.mjs:123-124` — the emit row literal; `styles` is collected
  and dropped here. **This is the whole seam.** Project onto the four cuts *at emit*: measured
  **+947 Brotli bytes against +2,464** for the raw array, and a fifth weight becomes unrepresentable
  rather than merely unused. Emitted type at `:132`. `font-store.ts:198` explicitly reserves this
  carry-through "for a later story" — this is that story.
- `folio-designer/src/generated/font-index.ts:8` — `IndexFamily`, the type to widen. Gitignored,
  rebuilt every build from `build-wasm.mjs`.
- `folio-designer/src/font-index.ts` — `FamilySource:93-99` (**local carries `faces:
  ReadonlyArray<CatalogueFace>`, stored carries `faces: ReadonlyArray<StoredFace>` + `census?:
  FamilyCensus`, web carries only `row: IndexFamily`** — the web tier is the only one with no cut
  data, which is what the emit step fixes); `addableFromTheWeb:229` (D2 changes this);
  `webFamilies:244`; `offeredFamilies:350`; `familyIsComplete:498`; `sourceScripts:132` is the
  worked precedent for reading a per-tier fact off a face set.
- `folio-designer/src/font-store.ts` — `FamilyCensus:201-209` (`published`, `refused`, `recordedAt`;
  **deliberately no `held` field** — held is read off `StoredFace.style`), `FamilyCutRefusal:173`,
  `FaceCutPermanence:170`, `censusIsComplete:230`. Persisted in IndexedDB store `'family-census'`.
- `folio-designer/src/font-source.ts` — `faceCuts:204` is the closed RIBBI vocabulary;
  `cutDeclarations:229` and `publishedCuts:247` are the condition D2 must derive from; the refusal is
  at `:572`.
- `folio-designer/src/font-browser-model.ts` — `BrowserRow:112-127` (no cut data yet; add it),
  `browserRows:156`, `rowTierNote:143` (the exhaustive-switch idiom a per-tier cut reader must
  follow), `resultLine` (reads `addableFamilyCount`, which D2 moves).
- `folio-designer/src/FontBrowser.tsx` — row head `:261-269`, grid foot `:282-287`. Both are
  `display:flex` with **no `flex-wrap`** and no `overflow:hidden`, so over-full content wraps and the
  box grows; nothing is ever clipped (measured).
- `folio-designer/src/release-payload.ts` — `maximumCoreCacheBytes:217` = 6,553,600;
  **`warnCoreCacheBytes:257` = 6,422,528 (new in story 3)**. Built manifest core total is
  **6,408,656**, so headroom to the warning is 13,872 bytes and +947 is **6.8%** of it.
- **Tests that must move, deliberately:** `font-index.test.ts:144-147` pins the generated row's exact
  field set; D2 moves any assertion over `addableFamilyCount` (1,274 → 1,271).
- **Tests that must not:** `font-browser-model.test.ts:240-254` and `:271-290` pin `weightLine`;
  leave them alone. `FontBrowser.test.tsx:112` pins the *header's* children as a closed set — the row
  has no such census, so this story must add its own positive assertion in **both** views.
- **Prior ruling this reverses, and on what grounds:** `epic-16-decision-log.md:1416-1427`
  (D-16.R.33 R3) declined this projection because *"exactly one face is embedded per family —
  eighteen styles and one style deliver the author the identical thing"*. CAP-1 has overturned that
  premise. R3 explicitly warns against reversal on budget, so the argument is the criterion, not the
  +947 bytes. Review FB14 (`review-token-fidelity-font-browser.md:57`) rests on the same premise.

## Tasks & Acceptance

**Execution:**
- [x] `folio-designer/scripts/build-font-index.mjs` -- project each row's `styles` onto the closed
      four-cut set and emit it at `:123-124`; widen the emitted `IndexFamily` type at `:132` --
      gives every web family its cuts with no fetch and makes a fifth weight unrepresentable.
- [x] `folio-designer/src/font-index.ts` -- per D2, require the projected cut set to include
      `Regular` in `addableFromTheWeb:229`, deriving the condition rather than naming families --
      the dialog stops offering three families no pick can install.
- [x] `folio-designer/src/font-index.ts` -- add a per-tier cut reader beside `sourceScripts:132`,
      exhaustive over `FamilySource`, applying D4 for the stored tier -- one place resolves cuts, and
      a fourth tier stops compiling.
- [x] `folio-designer/src/font-browser-model.ts` -- add the cuts field to `BrowserRow`, populate it
      in `browserRows`, and add the cut-line string function beside the others.
- [x] `folio-designer/src/FontBrowser.tsx` -- draw the cut line in the row head and (per Q-A) the
      grid foot.
- [x] `folio-designer/src/font-index.test.ts` -- update the field-set pin at `:144-147` and any
      `addableFamilyCount` assertion, each with a comment recording why it moved.
- [x] `folio-designer/src/font-browser-model.test.ts` -- cover every Matrix row, parameterised over
      the input rather than pinned at one value (the defect recorded at `:256-270`).
- [x] `folio-designer/src/FontBrowser.test.tsx` -- assert the cut line renders in both views --
      there is no row-children census to catch its absence.
- [x] `folio-designer/src/release-payload.ts` -- re-measure the core Brotli total; raise a constant
      only if a bound is actually breached, with its own measured rationale beside the `const`.

**Acceptance Criteria:**
- Given a non-variable family whose snapshot row lists weights beyond 400 and 700, when the dialog
  lists it, then the row names no cut outside the closed RIBBI set.
- Given the emit step runs with `fetch` replaced by a thrower, when the module is generated, then it
  succeeds and the rows carry cut data.
- Given a family whose census records a permanently-refused cut, when the dialog lists it, then that
  cut is not shown and `familyIsComplete` still reports it complete.
- Given a family whose census records a transiently-refused cut, when the dialog lists it, then that
  cut is shown and the button still offers to install.
- Given `npm run build`, when the offline release is verified, then the core tier is 30 assets and
  within `maximumCoreCacheBytes`, and `warnCoreCacheBytes` is reported either way.

## Implementation Notes

**⚠ THE FROZEN BLOCK WAS REOPENED BY THE OWNER TO CORRECT A COUNT I PROPAGATED.** At the re-plan I
reported the offered web population moving "1,274 → 1,271", and that was wrong twice over. Measured
from the built module: `webFamilies.length` goes **1,273 → 1,270** — the non-variable count of 1,274
already excludes one local-tier family, `Cousine` — and `addableFamilyCount` goes **1,304 → 1,301**.
The Regular-only numerator is unchanged at **947**, so the share is **947/1,270 = 74.57%**, not the
74.51% I derived.

**The expected BEHAVIOUR in both the D2 entry and the matrix row was always right**; only the counts
attached to them were wrong, and **the shipped code already carried the measured figures** — the
error lived only in the spec. The block was reopened because a wrong number surviving inside a frozen
block *because* the block is frozen is how a claim outlives its evidence. Swept the whole tree while
correcting it: every live site now reads 1,270 / 947 / 74.5%. Three `~1,273` approximations in
`App.tsx:6465`, `App.tsx:6702`, `App.test.tsx:4123`, `App.test.tsx:4154` and
`preview-face-registry.ts:18` are pre-existing, tilde-qualified, and about the pre-D2 population;
left alone as a separate sweep rather than smuggled into this diff.

**Real-build byte figure, measured.** `npm run build` green, `verify:offline` green. Core tier 30
assets (29 Brotli-weighed immutable + `/index.html`, which is outside every Brotli sum by design).
Core Brotli total **6,409,322** bytes against a pre-story **6,408,656** — a delta of **+666** (the
last 4 are D7's CSS rule), leaving **13,206 bytes** under `warnCoreCacheBytes` (6,422,528) and
144,278 under the ceiling. **No constant
was moved.** The plan-time estimate was +947 module-in-isolation; the real figure came in lower
because review finding 3 made the 537 variable rows emit `cuts: []`. Total immutable assets 155
against `maximumCacheAssets` 166.

**Final verification.** `npx vitest run` 90 files / **2163 tests, 0 failed, 0 skipped** against a
`617999d` baseline of 90 / 2149: **15 new test names, 1 gone** (a rename — `carries no licence
field…` → `carries the projected cut set and still no licence field…`). `npx tsc -b --noEmit` clean —
note a bare `tsc --noEmit` reported clean while `-b` found two real errors during this story, exactly
the vacuity the Verification section warns about. `npm run lint` exactly 8 `only-export-components`,
unchanged.

**D7's measurement, re-taken against the edited `App.css`** (chromium 1217, 1024px). Grid card foot
**82px**, cut span **148px x 13px on one line** (was 35 x 65), specimen **13px** (was 18), overflow
**0** — both figures the owner ruled on, confirmed. List row re-measured and **untouched**:
`flex-wrap: nowrap`, head height 21px, tightest slack **58.9px** at `Fira Sans Extra Condensed`,
identical to the pre-change figure. The list view draws no `-foot` at all, so only the card moved.

⚠ **NO AUTOMATED TEST COVERS D7, AND NONE CAN AS THINGS STAND.** The component suite runs in jsdom,
which applies no stylesheet — every assertion about this change would pass with the rule deleted. The
browser measurement above is the verification, and it is the only one.

**`shipped-face-cuts.ts` was absent from the Code Map and should not have been.** The local tier has
**two** authorities on what a pick yields — the committed catalogue and the shipped mirror — and the
spec named only the first. That omission produced triage finding 1.

**The seam, end to end.** `styles` is projected onto the closed four cuts at the emit step
(`scripts/build-font-index.mjs`, `cutsOf`) and emitted as `cuts` on every `IndexFamily` row; the
generated type imports `FaceCut` from `font-source.ts`, so a fifth weight is unrepresentable rather
than merely unused. `font-index.ts` gained `sourceCuts`, one exhaustive switch over `FamilySource`
beside `sourceScripts`; `font-browser-model.ts` carries the result on `BrowserRow.cuts` and owns the
only user-facing string (`cutLine`); `FontBrowser.tsx` draws it in the row head and the grid card
foot, immediately after the tier note (D5), as an ordinary `font-browser-meta` span.

**The three spellings were MOVED, not copied.** `RIBBI_CUT_NAMES` / `ribbiCutOf` /
`CATALOGUE_CUT_STYLES` and the `StyleCut` type left `App.tsx` for `font-source.ts`, beside the
`faceCuts` vocabulary they reconcile. The reverse reader the local tier needs —
`faceCutOfCatalogueStyle`, catalogue `BoldItalic` back to RIBBI `Bold Italic` — is **derived from
those two records**, never restated: for each cut in `faceCuts` it finds the variant key whose RIBBI
name is that cut and takes its catalogue spelling, falling through to the cut's own name for the
Regular, which the format's variant set does not key. `App.tsx`'s `CUT_NAMES` stayed (it is the
panel's prose) and is now typed against the imported `StyleCut` rather than being what defines it.
There is no fourth spelling and no second copy of a bridge.

**D2 is derived, not listed.** `addableFromTheWeb` is now `!row.variable && row.cuts.includes(
'Regular')`. `Buda`, `Molle` and `UnifrakturCook` are the three families today's snapshot drops; the
test asserts the CONDITION over the whole population and uses the three names only as a non-vacuity
control. Offered web families 1,274 -> 1,271; `addableFamilyCount` is asserted as a derivation on
both sides, so nothing hardcodes either number.

**R3's reversal is argued in shipped code, on the criterion.** The `Most styles` paragraph at the
head of `font-browser-model.ts` stated the dead premise ("this product embeds exactly one face per
family"). It now records that CAP-1 overturned it — eighteen styles and one style no longer deliver
the author the identical thing — reverses the half of D-16.R.33 R3 that rested on it, keeps the
sort arm and the `designer` field out on their own reasoning, and says in as many words that the
reversal is not argued on the byte budget.

**What did NOT change**, deliberately: `weightLine` and its two pinned tests; the install path, the
store, the census, the catalogue build and the document; `font-index.json` (not regenerated — the
`d6d51f1` pin holds and DW-166 trigger 1 does not fire); `release-payload.ts` (no bound breached).

**One test moved that the Code Map did not predict.** `font-index.test.ts`'s "raised the addable
count by exactly the batch size" rebuilds the pre-batch web filter by hand and had to gain D2's
second clause; left at `!row.variable` it counted three families the new filter drops and reported a
delta of 7 for a batch of 10. The number under test is unchanged; the filter it has to mirror moved.
No assertion anywhere pinned `addableFamilyCount` at 1,274, so the Code Map's expected edit there was
a no-op.

## Delivery Evidence

- `npx tsc -b --noEmit` — clean.
- `npx vitest run` — **90 files / 2159 tests, all passing**, against the 90 / 2149 baseline at
  `617999d`. NEW (11): `font-index.test.ts` — *carries the projected cut set and still no licence
  field…* (renamed from *carries no licence field…*), *projects every row onto the closed four cuts,
  in RIBBI order*, *stops offering a family that publishes no upright Regular…*, *reads a family's
  cuts off whichever tier it comes from…*; `font-browser-model.test.ts` — *names a web family's
  projected cuts, in RIBBI order, at every arity*, *never names a weight outside the closed RIBBI
  four, over every offered family*, *names a committed family's cuts in the store's spelling…*,
  *shows an installed family what it publishes, less what is permanently refused*, *leaves the
  footer's own sentences alone*; `FontBrowser.test.tsx` — *names each family's cuts in the row head…*,
  *names the same cuts in the Grid card…*. GONE (1): *carries no licence field, because no licence is
  knowable before a pick* — renamed, not deleted.
- `npm run lint` — exactly 8 `only-export-components` warnings, 0 errors, unchanged.
- `npm run build` — succeeded, `verify:offline` green, **core tier 30 assets** (29 Brotli-weighed
  immutable + `/index.html`), inside the pinned 30/30 envelope.
- **Core Brotli total: 6,409,463 bytes.** Against `warnCoreCacheBytes` 6,422,528 — **clear, margin
  13,065 bytes**. Against `maximumCoreCacheBytes` 6,553,600 — clear, margin 144,137 bytes. The cut
  arrays cost **+807 Brotli bytes** against the +947 projected. No constant raised; neither bound
  breached, so `release-payload.ts` is untouched.
- Measured population after D2, from the committed snapshot: 1,811 rows, 537 variable, 1,274
  non-variable, **1,271 offered** (3 publish no static upright 400), of which **947 are Regular-only**
  and **103 are four-cut**. The `947 of the 1,274` footnote in `font-source.ts`, `font-store.ts` and
  their tests moved to `947 of the 1,271`, with the reason recorded at `publishedCuts`.

## Spec Change Log

- **2026-09-20 — re-planned at the coordinator's instruction** after stories 1, 6, 2, 3 and 4 landed
  (`617999d`). Q4 deleted: story 3 corrected `weightLine` when its own change made it a lie. Q1
  resolved by owner ruling and confirmed by browser measurement (D1). Q5 resolved by owner ruling as
  an explicitly widened fence (D2). Q2 and Q3 settled against shipped code rather than asked (D3,
  D4) — the census, the four-state absence vocabulary and the
  `familyIsInstalled`/`familyIsComplete` split answer both. One new question opened, Q-A, because the
  owner's D1 measurement was taken on the row and the grid card is a different box. The Code Map was
  rewritten from the tree at `617999d`; every line number in the prior draft had moved.

## Review Triage Log

Four layers ran: blind-hunter, edge-case-hunter, verification-gap, plus a targeted layer aimed at the
hard requirement and the grid card (coordinator's instruction). Every finding has a row.

| # | Finding | Verdict | Evidence | Route |
|---|---|---|---|---|
| 1 | `local` tier under-states **Roboto**: card says "Regular", a pick yields four cuts | **high** | Verified: `font-catalogue.json` has Roboto Regular only, `shipped-face-cuts.ts:82` declares its three cuts, `App.tsx:6808` routes any `isShippedFamily` pick to `commitDeclaredCuts`. Direct deviation from the matrix's "the cuts actually shipped", and the new test **pinned the wrong answer**. Roboto is the only affected family (verified by a listing pin). | patch |
| 2 | The network-free emit assertion is inert | **high** | Three layers found it independently; verification-gap demonstrated it by mutation (`cutsOf → []`, test stayed green). `familyIndex` is a static ESM import bound before the emit call. AC2 was unverified. | patch |
| 3 | All 537 variable rows ship non-empty `cuts` | medium | `cutsOf` never consulted `axes`. The function's own comment rejects "merely unused rather than unrepresentable"; this was exactly that. | patch |
| 4 | `sourceCuts` web arm bypassed `inRibbiOrder` | medium | The doc directly above asserts ordering is guaranteed; the arm was a raw passthrough holding only by the emit step's coincidence. | patch |
| 5 | Stale `1,273` at `font-index.ts:683-684` (and the word "alone") | medium | Measured `webFamilies.length` = 1,270 after D2. | patch |
| 6 | Stale `1,273` at `font-source.ts:457` | medium | Same measurement. `1,218` did not move — none of the three publishes a Regular, so none was ever in that numerator. | patch |
| 7 | `faceCutOfCatalogueStyle` untested; its `undefined` path drops a cut silently | medium | No test referenced it; the existing `not.toContain('BoldItalic')` guard stays green when the cut vanishes entirely. | patch |
| 8 | Component tests only ever rendered `tier: 'web'` rows | medium | `'cuts not stated'` — the string justified purely by how an empty span reads in the DOM — was never rendered. | patch |
| 9 | `cutDeclarations`' original JSDoc orphaned by an inserted second block | low | The new block sat between the old one and the `const`, so the old block documented nothing. | patch |
| 10 | `StyleCut` union + `styleCuts` array = two hand-written authorities | low | Inside the very comment block forbidding duplicated authorities; `FaceCut` forty lines up does it right. | patch |
| 11 | `font-index.ts:326` cited `font-source.ts:572`, invalidated by this story's own +67 lines | low | Refusal had moved to ~:650. | patch |
| 12 | `build-wasm.mjs:334` still pointed at `App.tsx`'s `CATALOGUE_CUT_STYLES` | low | This story moved it to `font-source.ts`. | patch |
| 13 | D2 test's `addableFamilyCount` formula not refresh-proof | low | `refusedUpstream` was not filtered by `!localTierHolds` while the first term was. | patch |
| 14 | `cutsOf`, `RIBBI_CUT_NAMES`, `styleCuts` exported with no external consumer | low | Widens the surface a copier can reach for, in a change whose point is to stop copying. `styleCuts` kept exported — the new local arm genuinely needs it. | patch |
| 15 | `const offered = familyIndex.filter(r => !r.variable)` misnamed | low | That is the 1,274 non-variable rows, not the offered population. | patch |
| 16 | Grid card: cut span renders **35px wide × 65px tall** (5 lines, separators orphaned); specimen collapses **57px → 18px** | medium | **Measured** by me, chromium 1217 @1024px: card 229px, foot content 203px, `scrollWidth == clientWidth == 203` (no overflow), card height holds at 150px. `flex-wrap: wrap` would give one 148px line at the cost of a taller foot (82px) and a 13px specimen. | **defer** |
| 17 | `.otf` and nameID-0 refusals recorded `permanence: 'transient'` though unfixable | medium (unverified reach) | `font-source.ts:783`'s catch is unconditional; `font-source.ts:340` admits `.otf` while `requireStaticTrueTypeTables` refuses it. Targeted layer probed 492 of 1,270 offered families: **0 non-`.ttf` faces**, so not reachable today. Pre-existing (story 1's code), exposed not caused. | **defer** |
| 18 | Snapshot lists a 400 whose upstream file is missing or `.woff2` | maybe-false | Would need the remaining 778 families probed. 492 probed, 0 disagreements, 0 non-`.ttf`. | **defer** |
| 19 | Stored census present but `published` empty while faces held → "cuts not stated" | **false** | Unreachable: a census is only written after a fetch, and `publishedCuts` is non-empty or the family is refused for having no Regular. | rejected |
| 20 | Two literal `·` separator spans added inline in `FontBrowser.tsx` violate "strings live in the model" | **false** | The `·` meta separator is the file's pre-existing pattern — two already stood there before this diff. Not a new authority. | rejected |
| 21 | Grid foot gets a horizontal scrollbar and text spills across the neighbouring card; "a taller box is impossible" | **false** | Measured: `scrollWidth == clientWidth == 203`, `overflow: visible`, and the foot **does** grow 26px → 65px. Flex items default to `min-width: auto`, so they shrink and wrap internally rather than overflowing. | rejected |
| 22 | Grid card grows 169px → 208px and the specimen stays 76px | **false** | Measured: card height holds at **150px** (`min-height` governs) and the specimen collapses **57px → 18px**. The opposite of the claim. | rejected |


## Design Notes

**Measured at 1024px in chromium 1217 against the real `App.css` and IBM Plex Sans, not reasoned
about.** Modal 980px → rail 236 + results 742 → row content **710px**. Named form, all 1,274 offered
families: **0 wrap**, tightest slack 58.9px. The only wrapping case needs `category not stated` *and*
`script not stated` together, which the snapshot cannot produce (every web row has both) and which is
reachable only via a stored family orphaned from the snapshot. Both flex containers are `nowrap` with
`overflow: visible`, so the failure mode throughout is a taller box, never hidden text — the row head
goes 21px → 31.2px. The grid card is the tight box and is Q-A.

**Why project at the emit step rather than carry the array.** `styles` is an inventory of upstream
*offered* weights: accurate for the 1,274 non-variable rows (65/65 agreed with upstream
`METADATA.pb`) and fiction for the 537 variable rows (1/30 agreed) that `addableFromTheWeb` hides.
Projecting at emit keeps the fiction out of the module entirely and costs 62% less.

**The 947 figure is already a committed fact.** `publishedCuts`' own comment
(`font-source.ts:241-244`) cites "947 of the 1,274 offered web families" — the measurement from this
story's first plan, absorbed by story 1. D2 moves the denominator to 1,271; that comment is not
wrong, but check whether it wants the same footnote.

## Verification

**Commands:**
- `npx vitest run src/font-browser-model.test.ts src/FontBrowser.test.tsx src/font-index.test.ts` --
  expected: pass, with the new cut cases present by name. Diff the test-NAME set against the
  baseline, never the count.
- `npx vitest run` (in `folio-designer`) -- expected: 90 files / 2149 tests at baseline `617999d`,
  plus this story's additions; report GONE/NEW name sets.
- `npx tsc -b --noEmit` -- expected: clean. A bare `--noEmit` is vacuous against a solution
  `tsconfig`; `-b` is required.
- `npm run lint` -- expected: exactly 8 `only-export-components` warnings, unchanged.
- `npm run build` -- expected: succeeds; `verify:offline` green; core tier 30 assets. ⚠ A `vite` dev
  server has been running 31+ hours and causes spurious `ENOTEMPTY`/`ENOENT`; the build takes ~3.5
  min and must run alone.
- `node -e` over `dist/offline-release-manifest.json` -- expected: report `brotli.core.totalBytes`
  against **both** 6,422,528 (warn) and 6,553,600 (ceiling). Record the number either way.
