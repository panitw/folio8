---
title: 'Show a family''s cuts before the pick'
type: 'feature'
created: '2026-09-19'
status: 'draft'
route: 'dispatch'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The Add font dialog says nothing about which cuts a family has, so once stories 1–3
make a pick install a family's whole face set, a Regular-only family stays indistinguishable from a
four-cut one until the author applies it and presses **B**. SPEC-install-all-face-cuts CAP-6 closes
that: every listed family shows its cuts, and what is shown must match what installing yields.

**Approach:** Carry per-family cut data to the browser from the tier that already knows it — the
committed snapshot for web families, the catalogue face set for committed families, the local store
for families already installed — project it onto the format's closed four-cut set, and draw it in
the row's existing meta line as one more string owned by `font-browser-model.ts`.

## Boundaries & Constraints

**Always:**
- The cut vocabulary is the format's closed set only: Regular, Bold, Italic, Bold Italic. A family
  publishing 18 styles shows at most these four. Never a raw upstream style count.
- Every user-facing string lives in `font-browser-model.ts` beside the others, never inline in
  `FontBrowser.tsx`.
- `scripts/build-font-index.mjs` stays pure and offline: the emit is a function of the committed
  `font-index.json` alone, with no network and no snapshot regeneration (regeneration breaks the
  `d6d51f1` pin and fires DW-166 trigger 1).
- The generated row's field set is pinned by `font-index.test.ts:129-132`; widening it is a
  deliberate, commented retirement of that pin, not a silent edit.
- Re-measure the core-tier Brotli total after the change. `maximumCoreCacheBytes = 6553600` is a
  live ceiling; the last build measured 6,398,057 with 155,543 bytes of headroom.
- Every comment asserting the one-face model is part of this diff, not left standing:
  `font-browser-model.ts:26-34` (`Most styles` / `stylesLabel`, citing D-16.R.33 R3).

**Never:**
- No `Most styles` sort arm and no `designer` field. CAP-6 asks for a display, not a sort; the
  `designer` half of D-16.R.33 R3 is untouched and still stands on its own reasoning.
- No `METADATA.pb` fetch, no network call, and no speculative pre-fetch to build the display.
- No change to the install path, the store, the catalogue build or the document. This story reads
  what stories 1–3 produce and draws it.
- No fifth weight, no variable axes, no synthetic cuts — a family's variable rows stay filtered out
  of the dialog by `addableFromTheWeb` exactly as today.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Four-cut web family | snapshot `styles: ["400","400i","700","700i"]` | row states all four cuts | N/A |
| Regular-only web family | projects to Regular alone — 947 of 1,274 | row states Regular only | N/A |
| Many-weight web family | `styles: ["100"…"900"]`, non-variable | projects to Regular + Bold only | never names a 5th weight |
| Bold, no italic | `styles: ["400","700"]` | Regular + Bold | N/A |
| Family absent from snapshot | stored/orphaned tier, no index row | cuts read from the tier, not the snapshot | no row → no cut claim |
| Committed family | catalogue face set (story 3) | cuts from the faces actually shipped | N/A |
| Installed family, partial set | store holds 3 of 4 after one refused fetch | per Q3's ruling | N/A |
| Variable family | `variable: true` | not listed at all | unchanged |
| Family with no upright 400 | Buda, Molle, UnifrakturCook — the only 3 | per Q5's ruling | install refuses these outright today |

</frozen-after-approval>

## Open Questions

- **Q1 — What form does the cut display take?** The mockup drew `stylesLabel` ("16 styles" /
  "1 style") at `Font Browser.dc.html:367` (row) and `:390` (grid); review FB14 struck it, and the
  slot is empty today. Its raw-count form now contradicts this spec's closed four-cut set.
  Options: **(a) named cuts** — "Regular · Bold · Italic · Bold Italic", explicit but long in a
  flex row already carrying family, category, tier note and script badge; **(b) four-cut count** —
  "4 cuts" / "Regular only", shortest, but "3 cuts" does not say *which* three;
  **(c) letter badges** — "R B I BI" or B/I marks, compact and scannable, needs an accessible
  name and is new vocabulary; **(d) negative only** — draw nothing when a family has all four and
  "Regular only" when it does not, quietest, but CAP-6 says *every* family shows its cuts.
  *My read:* (a). CAP-6's success criterion is per-family and explicit, and the row already proves
  it tolerates a four-item meta line; (c) is the fallback if the row measures too wide.

- **Q2 — What fidelity standard does the pre-pick display hold itself to?** Web-tier cuts come from
  the committed snapshot (`snapshotDate: "2026-09-03"`, 16 days old, ages between releases); the
  install fetches live from upstream. I measured 65 of 65 sampled non-variable families agreeing
  exactly with upstream `METADATA.pb`, and 0 drifted families across a 90-family probe — so the two
  agree today. But the asymmetry is real and new: a family that vanishes upstream is refused
  (`font-source.ts:440`), while a *cut* that vanishes upstream is simply not fetched, and nothing
  refuses or reports it. CAP-6's success criterion ("the cuts shown match what installing actually
  yields") is a hard requirement, so this needs a ruling rather than an assumption.
  Options: **(a) snapshot fidelity is the standard** — the row states what the snapshot says, on the
  same footing as the family list itself, and a drift is silently a smaller install;
  **(b) reconcile at install** — show snapshot cuts pre-pick, and when a promised cut does not
  arrive, say so where the install reports its outcome (couples this story to story 1's install
  path); **(c) web tier shows nothing** — only committed and already-installed families show cuts,
  which is exact but guts CAP-6 for the 1,274 web families that are most of the dialog.
  *My read:* (b). (a) is the dispatch note's position and is defensible on measurement, but CAP-6
  says *match*, and (b) is what makes a broken promise visible rather than silent.

- **Q3 — For a family already on this machine, are the cuts shown the ones HELD or the ones the
  family PUBLISHES?** Story 1 refuses each cut on its own terms, so a family can install with 3 of
  4 faces. Options: **(a) held** — the row reports this machine's truth and a failed italic simply
  is not claimed, but the same family reads differently on two machines; **(b) published** — the row
  reports the family, consistent everywhere, but claims a cut this machine does not have and
  pressing **I** then warns; **(c) held, with the shortfall visible** — held cuts shown plus a note
  that one could not be fetched.
  *My read:* (a). The stored tier's other fields already report this machine's truth over the
  snapshot's claim (`browserRows`, `font-browser-model.ts:167-170`), and (a) keeps that rule.

- **Q4 — Who corrects `weightLine`?** `font-browser-model.ts:377` returns "N faces · one upright
  Regular each, no bold or italic", pinned three ways in `font-browser-model.test.ts:241-247` and
  `:265-283` (including a loop over seven staged counts that requires that exact clause). Story 1
  makes it false the moment it lands. Options: **(a) story 5 owns it** — the browser's strings are
  this story's surface and the correction lands with the display that replaces its purpose;
  **(b) story 1 owns it** — the story that falsifies the claim repairs it, and story 5 must then
  not collide with that edit; **(c) a separate follow-up** — leaves a false footer shipping between
  stories 1 and 5.
  *My read:* (a), with the spec saying so plainly so story 1's builder leaves it alone. (c) is
  refused: it ships a footer that lies.

- **Q5 — Three listed families cannot install at all, and a cut display is what makes that
  visible.** Measured over the committed snapshot: `Buda` (`["300"]`), `Molle` (`["400i"]`) and
  `UnifrakturCook` (`["700"]`) are non-variable, pass `addableFromTheWeb`, and are therefore offered
  in the dialog today — but they publish no upright weight-400 face, so `font-source.ts:449-453`
  refuses them outright. This is a pre-existing defect, not one this story creates; it becomes
  load-bearing because a cut line would advertise "Bold" for a family the installer will refuse.
  Options: **(a) filter them out** — add the weight-400 requirement to `addableFromTheWeb`, which
  fixes the real bug but changes which families the dialog *offers*, outside this story's fence;
  **(b) show them as uninstallable** — a fourth row state, honest but new UI vocabulary;
  **(c) leave them and say nothing** — the cut line then advertises a cut no pick can deliver, which
  is the exact failure CAP-6 exists to prevent; **(d) defer** — register it and let story 1, which
  owns the install path and its refusal, take it.
  *My read:* (d), with (a) as the fix story 1 should make. Three rows in 1,274 do not justify
  widening this story's fence, but (c) is not acceptable and the spec should not pretend otherwise.

## Code Map

- `folio-designer/font-index.json` — committed snapshot, 1,811 rows, every row already carrying
  `styles` (e.g. `["400","400i"]`). **Do not regenerate.** 84 distinct styles-arrays; 22 distinct
  values including `1`/`1000` (variable rows only).
- `folio-designer/scripts/build-font-index.mjs` — `trimFamily:74` already writes `styles`;
  `emitFontIndexModule:118-135` drops it. **This is the shortcut, and it holds.** The emit row at
  `:124` is the one line to widen. Project onto the four cuts *here*, at build time: measured over
  the emitted module at Brotli q11, the projection costs **+947 bytes against +2,464** for the raw
  array, and it makes a fifth weight unrepresentable rather than merely unused. (R3's +1,326 is a
  bundle-delta figure, not comparable to these two, which are module-in-isolation.)
- `folio-designer/src/generated/font-index.ts` — gitignored, rebuilt every build via
  `build-wasm.mjs:629`. `IndexFamily` at `:8` is the type to widen.
- `folio-designer/src/font-index.ts` — `FamilySource` `:77-83` (three tiers; **story 1 reshapes the
  local and stored arms into face sets — build on that, do not re-derive it**),
  `addableFromTheWeb:149` = `!row.variable`, `offeredFamilies:262`, `indexRowFor:407`.
- `folio-designer/src/font-browser-model.ts` — `BrowserRow:112` (add the cuts field),
  `browserRows:156` (tier-scripts-win precedent at `:167-170` is the model for tier-cuts-win),
  `rowTierNote:143` (the exhaustive-switch idiom a per-tier cut reader must follow),
  `weightLine:377`, and the now-false comment block `:26-34`.
- `folio-designer/src/FontBrowser.tsx` — row head `:253-259` and grid foot `:274-279` are the two
  render sites; `.font-browser-meta` / `.font-browser-row-head` styled at `App.css:1317`.
- `folio-designer/src/generated/font-catalogue.ts:36` — `CatalogueFace` already has `style: string`
  (every row `"Regular"` today); story 3 makes it a real per-face style. The committed tier's source.
- **The cut vocabulary already exists — do not mint a second one.** `StoredFace.style` and
  `CatalogueFace.style` both hold `"Regular"`, and `shipped-face-cuts.ts:77-83` is the worked mirror:
  `Regular` / `Bold` / `Italic` / `Bold Italic`. It coincides with the name-table subfamily for RIBBI
  weights only, which is another reason the display must never reach past the four cuts.
- `folio-designer/src/release-payload.ts:179` — `maximumCoreCacheBytes = 6553600`, the live ceiling
  the bundle growth lands on. Read out of source text by `offline-release-contract.mjs`; derive,
  never re-type.
- **Tests that must move, deliberately:** `font-index.test.ts:129-132` (exact generated field set),
  `font-browser-model.test.ts:241-247` and `:265-283` (`weightLine` pins — see Q4).
- **Tests that must not:** `FontBrowser.test.tsx:112` pins the *header's* element children as a
  closed set. The row has no such census — so this story must add its own positive assertion that
  the cut line is drawn, in both views.
- **Prior rulings this story reverses, and the reason:** `epic-16-decision-log.md:1416-1427`
  (D-16.R.33 R3) and `review-token-fidelity-font-browser.md:57` (FB14) declined the styles
  projection because "exactly one face is embedded per family — eighteen styles and one style
  deliver the author the identical thing". SPEC-install-all-face-cuts CAP-1 makes that premise
  false. R3 explicitly warns against reversing it on budget; this reversal is on the criterion.

## Tasks & Acceptance

**Execution:**
- [ ] `folio-designer/scripts/build-font-index.mjs` -- project each row's `styles` onto the closed
      four-cut set and emit it at `:124`; widen the emitted `IndexFamily` type -- gives every web
      family its cuts with no fetch, and makes a fifth weight unrepresentable.
- [ ] `folio-designer/src/font-index.test.ts` -- update the field-set pin at `:129-132` with a
      comment recording why it widened -- the retirement is deliberate, not incidental.
- [ ] `folio-designer/src/font-browser-model.ts` -- add the cuts field to `BrowserRow`, read it
      per tier in `browserRows` through an exhaustive switch, and add the cut-line string function
      beside the others -- one description, three tiers, no fourth tier compiling silently.
- [ ] `folio-designer/src/font-browser-model.ts` -- rewrite the `:26-34` comment block and (per Q4)
      `weightLine:377` -- no comment or string may outlive the model it describes.
- [ ] `folio-designer/src/FontBrowser.tsx` -- draw the cut line in the row head and the grid foot --
      the two places the meta trio already appears.
- [ ] `folio-designer/src/font-browser-model.test.ts` -- cover every Matrix row, parameterised over
      the input rather than pinned at one value (the defect recorded at `:250-262`) -- a guard over
      one point is not a guard over the function.
- [ ] `folio-designer/src/FontBrowser.test.tsx` -- assert the cut line renders in both views --
      there is no row-children census to catch its absence.
- [ ] Per Q5's ruling only -- no change to which families are offered without it.
- [ ] `folio-designer/src/release-payload.ts` -- re-measure the core Brotli total; only if the
      ceiling is breached, raise it with its own measured rationale beside the `const`.

**Acceptance Criteria:**
- Given a non-variable family whose snapshot row lists weights beyond 400 and 700, when the dialog
  lists it, then the row names no cut outside the closed four-cut set.
- Given the emit step runs with `fetch` replaced by a thrower, when the module is generated, then it
  is generated successfully and carries the cuts field.
- Given a family the local store already holds faces for, when the dialog lists it, then its cuts
  come from the store and not from the snapshot.
- Given `npm run build`, when the offline release is verified, then the core tier is within
  `maximumCoreCacheBytes` and the core asset count is still exactly 30.

## Implementation Notes

## Spec Change Log

## Review Triage Log

## Design Notes

**Planned against uncommitted work.** The tree carries ~25 modified files of in-flight
spec-deferred-offline-cache story 5 work; the owner ruled to plan against the working tree, so every
line number and measurement above was read from disk, not at HEAD. `maximumCoreCacheBytes` and the
6,398,057-byte core total both come from that in-flight state.

**Depends on stories 1 and 3.** Story 1 reshapes `FamilySource`'s local and stored arms from a single
face to a face set; story 3 gives committed catalogue faces a real per-face `style` (today
`build-wasm.mjs:460` stamps every one of them `"Regular"`). Without story 3, the committed tier would
report every family as Regular-only — correct-looking and wrong. Neither is this story's to build.

**Why project at build time rather than carrying the raw array.** The raw `styles` array is an
inventory of upstream *offered* weights, accurate for the 1,274 non-variable rows the dialog offers
(measured: 65/65 agreeing with upstream `METADATA.pb`) and fiction for the 537 variable rows
(measured: 1/30 agreeing) that `addableFromTheWeb` already hides. Projecting to four cuts in the emit
step keeps the lie out of the module entirely, costs 62% less than carrying the array (+947 vs
+2,464 Brotli bytes, measured), and means no consumer can read the field as a full inventory.

**The four-cut projection over the 1,274 non-variable rows, measured:** Regular alone 947, Regular +
Bold 160, all four 103, Regular + Italic 49, Regular + Bold + Italic 12 — and three rows with no
upright 400 at all (Q5). So three quarters of the dialog will say "Regular only", which is the point
of CAP-6 rather than a defect in it.

## Verification

**Commands:**
- `npx vitest run src/font-browser-model.test.ts src/FontBrowser.test.tsx src/font-index.test.ts`
  (in `folio-designer`) -- expected: all pass, with the new cut cases present by name.
- `npx tsc -b --noEmit` (in `folio-designer`) -- expected: clean; note a solution `tsconfig` makes a
  bare `--noEmit` vacuous, so `-b` is required.
- `npm run build` (in `folio-designer`) -- expected: succeeds, `verify:offline` green, core tier
  still 30 assets and within `maximumCoreCacheBytes`.
- `node -e` over `dist/offline-release-manifest.json` -- expected: core immutable Brotli total
  reported and compared against 6553600; record the number in Implementation Notes either way.
