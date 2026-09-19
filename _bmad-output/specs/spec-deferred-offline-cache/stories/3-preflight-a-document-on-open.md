---
title: 'Preflight a document on open'
type: 'feature'
created: '2026-09-19'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: 'd43e2ab571bd316dc0b348cbd7724c19c08e9018'
context: ['{project-root}/_bmad-output/specs/spec-deferred-offline-cache/SPEC.md', '{project-root}/_bmad-output/specs/spec-deferred-offline-cache/asset-tiers.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Three things are left over from stories 1 and 2. An open that fails because a
deferred asset is unreachable says `Failed to fetch` rather than naming what is missing. A
document's deferred faces are fetched only when a paint has already failed, so the canvas
substitutes first and corrects itself after. And `Roboto` — the primary face of the starter and
all four bundled examples — resolves on the canvas to `catalogue-roboto`, which story 1 tiered
`deferred`, while its own Bold, Italic and Bold Italic cuts are core.

**Approach:** Move Roboto's canvas copy into the core tier so the default document paints
correctly on a cold start; fetch a document's deferred faces as part of opening it while the
network is there; and name the missing asset when an open genuinely cannot proceed.

## Boundaries & Constraints

**Always:**
- REFUSAL IS FOR THE DOCUMENT'S OWN BYTES ONLY (owner decision, 2026-09-19). An open is refused
  when the template or the sample JSON cannot be had — without those there is no document. A
  canvas face that cannot be fetched is NOT grounds for refusing an open: the engine holds its
  own embedded copies, so layout, pagination, preview and the PDF are all correct, and story 2's
  dismissible substitution warning is the settled answer for the glyphs.
- ROBOTO'S CANVAS COPY IS CORE (owner decision, 2026-09-19). `catalogue-roboto` moves from
  `deferred` to `core`: the pin goes 29 → 30 and the blocking load 10.67 → 10.82 MiB. One family
  split across two tiers was incoherent, and the default document paints in it.
- The refusal names the asset and the reason in the house voice: subject first, past tense for
  what did not happen, the concrete cause, then what was left untouched. Existing examples:
  ``${family} was not used: the designer was busy with another change. Try it again.``
- Prefetch on open is bounded to the faces THAT document declares. It is not a warm-up of the
  catalogue and not a background top-up.

**Never:**
- Do not refuse an open, or block one, because a canvas face is unreachable.
- Do not change `isCatalogueAssetUrl`. The generator uses it for `brotli.catalogue.totalBytes`
  and for the check that the emitted catalogue matches `font-catalogue.json`'s 31 faces; Roboto
  is still a catalogue face, it is merely no longer a deferred one.
- Do not let two tier rules match one URL. `classifyAssetTier` throws on any overlap, so the
  core Roboto rule and the deferred catalogue rule must be written to exclude each other.
- Do not weaken story 2's single-network-read guard in the worker.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Cold start, starter document | Core tier only | Roboto paints; no deferred fetch is needed to draw the default document | N/A |
| Example opened online, first time | Nothing cached | Template, sample and the document's deferred faces are fetched; canvas paints correctly with no substitution warning | N/A |
| Example opened offline, never opened | Template not cached, no network | Open refused, naming the example and that its bundled template is not on this machine; dialog stays open, nothing replaced | Refusal, document untouched |
| Example opened offline, opened before | Template cached | Opens from cache | N/A |
| Document opens, a declared face unreachable | Template cached, face not | Document OPENS; canvas substitutes and warns once per family | Warning, never a refusal |
| Local file open, offline | Bytes from the picker | Opens; any deferred face it declares is prefetched if online, else warned | Warning only |
| Sample JSON unreachable | Template cached, sample not | Open refused, naming the sample | Refusal, document untouched |

</frozen-after-approval>

## Code Map

- `scripts/offline-release-contract.mjs` — `ASSET_TIER_RULES` and `classifyAssetTier`; the
  classifier THROWS when two rules match one URL, so the new core-Roboto rule and the existing
  `catalogue face` deferred rule must be mutually exclusive by construction (the CJK carve-out
  in `SHELL_OR_DOCUMENT_FONT_ASSET` is the precedent). `isCatalogueAssetUrl` is a separate
  concern — `generate-offline-release.mjs` uses it for the catalogue byte subtotal and the
  31-face count check, and it must keep matching Roboto.
- `src/release-payload.ts` — `minimumCoreCacheAssets` / `maximumCoreCacheAssets`, both 29, read
  by the line-anchored `readDeclaredConstant`. Both become 30, with the reason recorded beside
  them as the existing comment does.
- `src/generated/runtime-fonts.css` — the canvas's name→file mapping. `Roboto` →
  `catalogue-roboto`, `Noto Sans SC` → `noto-sans-cjk`, `Noto Sans` / `Noto Sans Thai` → core
  shipped files. Generated by `scripts/build-wasm.mjs`.
- `src/generated/font-catalogue.ts` — `catalogueFaces` gives family → asset URL, the map a
  prefetch needs for catalogue families. `src/shipped-face-cuts.ts` and
  `src/generated/offline-assets.ts` carry the shipped side. If no single name→URL map exists,
  the generator already knows both halves and is the place to emit one.
- `src/App.tsx` — `openExample` at `:3107` (fetches template and sample at `:3114`, both before
  any engine call, and sets `startupError` at `:3134`); `fetchExampleFile` at `:138` (throws
  `its bundled file could not be read (HTTP …)` and `its bundled file did not arrive in time`);
  `installOpenedDocument` at `:3015` is the shared installer for every open path; `open` at
  `:2988` and `installPickedFile` at `:3004` are the local-file path; `fileFailureSentence` at
  `:2940`.
- `src/StartupDialog.tsx` — `:125` renders `<p className="startup-error" role="alert">`; `:100`
  renders the thumbnails, which are requested as soon as the dialog opens.
- `src/canvas-face-misses.ts` and `App.tsx:3810` — story 2's substitution warning, the settled
  answer for a face that cannot be painted.
- `src/App.test.tsx:11835` — `describe('the startup dialog at launch')`, including
  `:11945` (a failed fetch is named, nothing loaded) and `:11988` (the full refusal sentence).
  These are the tests the new wording must be written into.

## Tasks & Acceptance

**Execution:**
- [x] `scripts/offline-release-contract.mjs` -- classify `catalogue-roboto` as core, leaving the
  other 30 catalogue faces deferred and the two rules mutually exclusive -- one family may not
  straddle the tiers.
- [x] `src/release-payload.ts` -- move both core pins 29 → 30 with the reason recorded.
- [x] `_bmad-output/specs/spec-deferred-offline-cache/asset-tiers.md` and `SPEC.md` -- correct
  the counts and bytes to 30 / 10.82 MiB core and 50 / 7.81 MiB deferred.
- [x] `src/App.tsx` -- name the missing asset when an open is refused, in the house voice,
  distinguishing "not on this machine and the network is unavailable" from the existing HTTP and
  timeout causes.
- [x] `src/App.tsx` -- on open, fetch the deferred faces the document declares when the network
  is available, so the canvas paints correctly rather than substituting and correcting itself.
- [x] Tests -- the refusal wording, the prefetch, and a tier test that Roboto is core while
  another catalogue face is deferred.

**Acceptance Criteria:**
- Given a cold cache, when the designer starts and shows the starter, then Roboto paints with no
  deferred fetch and no substitution warning.
- Given an example never opened and no network, when it is opened, then the refusal names the
  example and the missing bundled file, the dialog stays open and the current document is
  unchanged.
- Given an example never opened with the network up, when it is opened, then its deferred faces
  are fetched as part of the open and no substitution warning appears.
- Given a document declaring a face that cannot be fetched, when it is opened, then it opens and
  warns rather than refusing.
- Given `npm run build`, then the manifest carries 30 core assets and verification passes.
- Given `npm run test`, `npm run verify:offline:red` and `npm run test:e2e`, then all pass.

## Implementation Notes

**The tier rules stayed mutually exclusive by subtraction, not by order.**
A module-local `CORE_CATALOGUE_FACE_IDS = ['roboto']` in `scripts/offline-release-contract.mjs` builds a
`core catalogue face` rule, and the existing `catalogue face` deferred rule became
`isCatalogueAssetUrl(url) && !isCoreCatalogueAssetUrl(url)` — the CJK carve-out's precedent,
because `classifyAssetTier` throws on any overlap at all rather than preferring an earlier rule.
The carve-out matches an id followed by a DOT, so `catalogue-robotocondensed`,
`catalogue-robotomono` and `catalogue-robotoslab` stay deferred; all three are now rows in the
contract test's deferred table. `isCatalogueAssetUrl` is untouched and still matches Roboto, so
`brotli.catalogue.totalBytes` and the emitted-versus-declared 31-face check are unaffected.

**Measured after `npm run build`: 30 core / 10.82 MiB against 50 deferred / 7.81 MiB** — exactly
the figures the intent predicted, with `catalogue-roboto` at 0.152 MiB. `asset-tiers.md` and
`SPEC.md` carry the re-measurement and a note saying what moved and why.

**The face name → asset URL map is parsed back out of the stylesheet the build just wrote.**
Nothing in `src/` could turn a chain entry's face NAME into the URL the browser resolves it
through: `font-catalogue.ts` knows the 31 catalogue families and `offline-assets.ts` knows the
shipped files under camel-case slot names. Rather than assemble a second derivation beside
`runtime-fonts.css`, `scripts/build-wasm.mjs` regex-reads the emitted rules and emits
`src/generated/canvas-face-assets.ts` from them, asserting the row count against
`shippedFamilies.length + catalogueFaces.length` so a parse that quietly matched fewer rules
fails the build instead of emitting a short map. New `.gitignore` line; `build-wasm.test.ts`'s
closed emission set moved eight → nine.

**The prefetch is awaited inside `installOpenedDocument`,** which is the one document-replacement
path for template bytes and therefore covers examples and local files alike. The faces are
knowable only once Go has projected the chains for those bytes, and the canvas draws at
`setCurrentSnapshot`, so the await sits between them — that ordering is what the new App test
proves by holding the face fetch open and asserting the document name has not yet appeared.
`src/document-face-prefetch.ts` holds the logic: the faces come from the projection's PAINT
REPORT rather than its chains (a chain is a search order whose tail is routinely never reached),
carried `assetKey` fragments are skipped (AD-8 — a different namespace, and they arrived with the
document anyway), the tier comes from `payload.cacheAssets` rather than from a pattern (so
Roboto's move needed no edit there), a page with no payload prefetches nothing, an
`onLine === false` browser is not asked, and every failure is swallowed.

**One e2e harness had encoded "deferred catalogue face" as "catalogue face".**
`font-embed-boundary.spec.ts`'s `holdEveryCatalogueFace` primed the machine by fetching every
manifest asset with `tier === 'deferred'` whose URL began `/assets/catalogue-`, then asserted it
had fetched one per catalogue family. Those were the same set until Roboto crossed over; now the
filter is the URL alone, and the core face is a cache hit. Two specs failed on it and pass now.

**The refusal names which of the two files, and adds the offline cause.** `fetchExampleFile` now
takes a role — `bundled template` or `sample data` — and a rejected fetch on a browser reporting
`onLine === false` throws `its <role> is not on this machine and this browser is offline` instead
of letting `Failed to fetch` reach the dialog. `startupRefusal` wraps the cause in the house
voice: `<name> was not opened: <cause>. The document you had open is unchanged.`

## Spec Change Log

## Review Triage Log

**Round 1 (2026-09-19) — eleven findings, all applied.**

1. *The URL intersection was asserted only against itself.* Under Vitest a `?url` import resolves
   to `/src/generated/runtime/…` and never to the `/assets/<stem>-<hash>.<ext>` a build emits, so
   NO unit fixture can honestly tie the page's URLs to the manifest's — and both fixtures built
   the tier table out of `canvasFaceAssets`, making the lookup true by construction. The tie now
   lives in `scripts/verify-offline-release.mjs`, where both halves are real: every row of the
   generated map must resolve to exactly one emitted release asset, at least one must be
   `deferred` (or the prefetch is dead code) and at least one `core`. Negative-probed by renaming
   one row's file: `canvas-face-map-drift: … the release carries 0 assets for it`.
2. *Nothing tied `CORE_CATALOGUE_FACE_IDS` to the starter.* `offline-release-contract.test.mjs`
   now reads `public/templates/starter.folio`'s own chains, maps each family through
   `font-catalogue.json`, and requires every catalogue family it declares to classify `core`.
   Negative-probed by adding `Lora` to the starter: reds naming the family and the id.
3. *The ids were interpolated into a RegExp unvalidated.* Each is now checked at module load
   against `font-catalogue.json`'s ids and against the lower-case-alphanumeric shape, so `robto`
   throws where it is written instead of reddening the pin in another module.
4. *The offline branch overwrote a real HTTP diagnosis.* `if (!response.ok) throw` sat inside the
   `try` whose `catch` carried the offline wording, so a 404 or 503 answered while `onLine` was
   false lost its status code. The rejection diagnosis moved onto the two awaits that can reject
   (`.catch` on the request and on the body read); the status arm is outside it. Covered by a new
   test: offline browser, 503 response, `(HTTP 503)` survives.
5. *The open blocked on faces that never paint.* Every bundled example's chain ends in
   `Noto Sans SC`, so a chain-wide prefetch awaited the whole 4.72 MiB CJK face to draw Latin
   invoices. `paintedCanvasFaces` now reads `textPaint…fragments[].face` — Go's own report of
   which face it measured each run with — leaving an unreached fallback tail to the lazy path.
6. *`installOpenedDocument`'s comment over-claimed.* It said every open path comes through it;
   `startBlank` and the launch starter do not. The comment now names the paths it covers and says
   why the other two are exempt (they install the starter, whose every face is core).
7. *The e2e helper lost its tier control.* It now asserts the split itself — exactly
   `catalogue-roboto` core, every other catalogue face deferred — so a face crossing in either
   direction reds with both lists.
8. *`startupRefusal` mispunctuated causes it did not author.* A cause this module wrote is now
   tagged (`ExampleFileCause`) and spoken; anything else is QUOTED and attributed, so the
   browser's `Failed to fetch` is no longer passed off as the product's own sentence. Trailing
   `.`/`!`/`?`/`…` are stripped from either kind. Three new cases cover the generic branch.
9. *The module header over-claimed.* It said the prefetch makes the canvas paint in those faces;
   it makes the BYTES LOCAL — the `@font-face` rule still resolves asynchronously at paint time.
   Lowered to what the code does.
10. *Two fixtures with `as unknown as S1Payload`.* Replaced by one real, contextually typed
    `src/test/tiered-payload.ts`, which also records why it is not itself the proof of anything.
11. *A comment read 2.85 MiB.* Left at 2.85 and explained — see the note below.

**One finding answered rather than applied.** The review asked for 2.85 → 2.86 MiB in
`e2e/startup-dialog.spec.ts`, by `3.01 − 0.152`. That is two ROUNDED figures subtracted: the 30
deferred catalogue faces measure 2.8539 MiB, which is 2.85 at the file's own two decimals, and
`asset-tiers.md` records the same measurement. Writing 2.86 would introduce the error rather than
fix it, so the comment keeps 2.85 and now says explicitly why the subtraction disagrees — which
is what stops the next reader "correcting" it again.

Layers: blind-hunter (BH), edge-case-hunter (EC), verification-gap (VG).

| # | Finding | Verdict | Evidence |
|---|---|---|---|
| BH2 / EC6 / VG1 | The prefetch's URL intersection is asserted only against itself, so CAP-3 could ship silently dead | high | Confirmed. `deferredFaceAssets` intersects the generated map's `?url` values with `payload.cacheAssets[].assetUrl` by string equality, and BOTH test helpers (`document-face-prefetch.test.ts` `payloadTiering`, `App.test.tsx` `tieringPayload`) build the payload out of `canvasFaceAssets` itself, so the match is true by construction. They agree today only because Vite dedupes to one hashed copy per face; nothing asserts it. If the spellings ever diverge the prefetch returns `[]` on every open, every test stays green, and the substitute-then-correct flash this story removes comes back. |
| VG2 / BH1 / EC5 | Nothing ties `CORE_CATALOGUE_FACE_IDS` to what `starter.folio` actually declares | high | Confirmed by the demonstration: adding a second catalogue family to the starter's chains leaves the classification tests, the 30/30 pin and every suite green while the first screen regresses to substitute-then-correct. The whole tier move exists for the starter, and the invariant behind it is held by a comment. |
| BH1 / EC5 | `installOpenedDocument`'s comment claims every open path comes through it; `startBlank` and the launch starter do not | medium | Confirmed: `startBlank` loads `blankBytes` straight into `engine.request('load', …)`, and the launch starter arrives via `initialSnapshot`. Harmless today only because Roboto became core in this same change — the claim is carried by the special case that made it unnecessary. |
| BH5 / EC2 | The offline branch overwrites a real HTTP diagnosis | medium | Confirmed by reading `fetchExampleFile`: `if (!response.ok) throw` is inside the `try`, so the `catch`'s `navigator.onLine === false` arm replaces it. A 404 or 503 answered while the browser reports offline loses its status code — and the comment beside it claims the opposite ("anything else keeps the throw's own words"). |
| BH4 / EC1 / EC3 / VG-other | The open now blocks on faces that never paint, including a 4.72 MiB CJK fallback on every example | medium | Real. Every bundled example's chain carries `Noto Sans SC` as its CJK fallback, so a first online open awaits the whole CJK face before the document appears, for glyphs the document does not contain. The frozen matrix does say the document's deferred faces are fetched on open, so this is a refinement of which faces qualify rather than a contradiction: a fallback for a script the text never uses is not one of "the document's faces". |
| BH7 | `CORE_CATALOGUE_FACE_IDS` is interpolated into a RegExp unescaped and unvalidated | medium | Real. A typo (`robto`) matches nothing, every catalogue face silently stays deferred, and the only thing that reds is the pin in another module with a message naming neither the list nor the typo — reverting the story with a misleading error. |
| BH6 / EC8 | The e2e helper lost its tier control instead of gaining a corrected one | low | Confirmed: the filter is now the URL prefix alone, so the harness asserts nothing about tiers; five more faces crossing to core would still prime 31 and still pass. |
| BH12 / EC7 | The refusal's generic branch is untested and mispunctuates non-authored causes | low | Confirmed: `startupRefusal` strips exactly one trailing `.`, and the three updated assertions match only the prefix, so a platform cause yields "… was not opened: Failed to fetch. …" — raw platform wording mid-sentence, in a change whose purpose is the house voice. Ellipsis, `!` and `?` are also unstripped. |
| BH3 | The module header claims the prefetch makes the canvas paint in the face; it only makes the bytes local | low | Fair: the `@font-face` rule still resolves asynchronously at paint time, so a substitute frame remains possible, merely far less likely. The claim, not the code, is what overreaches. |
| BH10 / BH11 | `as unknown as S1Payload` double-casts in both new fixtures, which are near-duplicates of each other | low | Confirmed. The cast defeats the type check these fixtures most need, and the two helpers differ only in `as const` placement. |
| EC9 | A comment's arithmetic reads 2.85 where the file's own two-decimal convention gives 2.86 | low | Confirmed: 3.01 − 0.152 = 2.858. |
| BH8 | A verifier comment dropped the asset count the rest of the diff updated | low | Real but immaterial; the number deliberately lives in `release-payload.ts`. |
| BH9 | The diff omits `.gitignore`, `SPEC.md`, `asset-tiers.md` and the story file | false | Not a defect in the change: the orchestrator scoped the diff to `folio-designer/` plus the new modules. All four exist and are correct — `.gitignore:131` does carry the generated file. |
| EC4 | Deferred faces are fetched with unbounded concurrency | low | A document declares a handful of faces, not dozens; the worker's in-flight map already collapses duplicates. |
| VG-other | The prefetch does not refresh `heldLocalFamilies` | low | Transient only: the set re-probes when the family list or font browser opens, so no durable wrong claim. |

## Design Notes

**Why refusal is narrow.** The engine embeds every shipped face (`folio-go/fonts/fonts.go`),
including Roboto and Noto Sans SC, so a document whose canvas faces are missing still lays out,
paginates, previews and renders byte-identically. The only thing degraded is the glyphs drawn on
the approximate canvas, and story 2 already settled that as substitute-and-warn. Refusing an open
for it would block a document that is, in every way the product promises, correct.

**Why Roboto crosses over.** `runtime-fonts.css` maps `Roboto` to `catalogue-roboto` while
`Roboto Bold`, `Roboto Italic` and `Roboto Bold Italic` map to the shipped core files. The
starter and all four examples declare that chain, so the default document needed a deferred
asset to paint its body text. 0.152 MiB is a small price for the first screen being right.

## Verification

**Run 2026-09-19, all green:** `npm run build` (manifest 30 core / 50 deferred, verification
passed), `npm run verify:offline:red`, `npx vitest run` (88 files, 1996 tests), `npx tsc -b`,
`npx oxlint` (pre-existing fast-refresh warnings only), `npm run test:e2e`.

**Commands:**
- `cd folio-designer && npm run build` -- expected: succeeds; manifest shows 30 core / 50 deferred.
- `cd folio-designer && npm run verify:offline:red` -- expected: all proofs pass.
- `cd folio-designer && npm run test` -- expected: green.
- `cd folio-designer && npm run lint && npm run typecheck` -- expected: clean.
- `cd folio-designer && npm run test:e2e` -- expected: green, including the offline startup specs.
