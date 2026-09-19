---
title: 'Tier the release at build time'
type: 'feature'
created: '2026-09-19'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '3e6f4e553bdd29d918365e1d390a3dcb04f081e0'
context: ['{project-root}/_bmad-output/specs/spec-deferred-offline-cache/SPEC.md', '{project-root}/_bmad-output/specs/spec-deferred-offline-cache/asset-tiers.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The release has no notion of which assets must be present before the designer can
start. Every asset is equally blocking, so the 8.61 MiB of CJK font, catalogue faces, examples
and documentation gate a first-time visitor alongside the engine — and nothing in the build
notices when the blocking set grows.

**Approach:** Give every emitted asset a `tier` of `core` or `deferred`, decided at build time
from one declared rule, carried in the release manifest, and enforced by the verifier: an
untiered asset fails the build, and the core tier gets a defended ceiling of its own. Loading
behaviour does not change in this story — the worker still precaches and gates on everything.

## Boundaries & Constraints

**Always:**
- The tier rule has exactly one home, beside `isCatalogueAssetUrl` in
  `scripts/offline-release-contract.mjs` — the module both the generator and the verifier
  already import. A second copy is a copy that drifts.
- Bounds are DERIVED from the `const <name> = <digits>` declarations in `src/release-payload.ts`
  through the existing line-anchored reader, never re-typed: `npm run build` runs
  `verify:offline` but never Vitest, so a re-typed constant leaves a drifted build green.
- An asset matching no rule is a build failure, not a default.
- The new bound counts core-tier assets; `minimumCacheAssets`/`maximumCacheAssets` keep
  bounding the release total, unchanged.
- THE CORE TIER IS PINNED EXACTLY, NOT BOUNDED (owner decision, 2026-09-19):
  `minimumCoreCacheAssets` and `maximumCoreCacheAssets` are both **29**. Any growth OR shrinkage
  of the blocking set fails the build until someone moves the number deliberately, with a
  rationale comment. The existing coherence check permits equal ends (it compares `>`, not
  `>=`), so an exact pin is expressible. Chosen over headroom because an unwatched number is
  precisely how the first load reached 18.63 MiB.
  RENEGOTIATED 2026-09-19, mid-implementation: the pin was approved as 28 from a mis-measurement
  in `asset-tiers.md`, whose grouping script swept `/assets/pdf_thumbnail_view-<hash>.js` — a
  2.2 KiB pdf.js chunk — into the bundled-examples group on the substring `thumbnail`. pdf.js is
  core, so the true split is 29 core / 51 deferred. The MiB figures were never affected. Owner
  chose the measured number over the approved one, and `asset-tiers.md` and `SPEC.md` are
  corrected to match.

**Never:**
- Do not change any loading, caching, gating or service-worker behaviour. No edits to
  `src/offline-lifecycle.ts`, no `cache.put` in a fetch path, no change to what the worker
  precaches.
- Do not widen `declaredCacheAssetBounds`'s return shape.
- Do not add a field to the `s1` payload or its `cacheAssets`/`rows` entries.
- Do not drop, add, re-encode or rename any emitted asset.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Ordinary build | The current dist | Every `assets[]` entry carries `tier: 'core'` or `tier: 'deferred'`; core count is 29 | N/A |
| New asset matches no tier rule | An emitted asset no rule classifies | Verification fails, naming the asset URL and that it is untiered | fail with the asset URL in the message |
| Core tier grows past its ceiling | Core count exceeds `maximumCoreCacheAssets` | Verification fails, naming the count and the declared ceiling | fail, message distinct from the release-total bound |
| Core tier falls under its floor | Core count below `minimumCoreCacheAssets` | Verification fails, naming the count and the declared floor | fail, own message |
| Core bound declaration is incoherent | Floor declared above ceiling | Throws naming the DECLARATION as the fault, not any release | throw before any release is judged |
| Tier added to the manifest | Manifest now carries `tier` per asset | The `tier` field feeds neither `release.id` nor `pageId` — `releaseIdentity` hashes only `url` and `sha256` | N/A |

</frozen-after-approval>

## Code Map

- `scripts/offline-release-contract.mjs` — shared by generator and verifier. `:18`
  `isCatalogueAssetUrl` is the precedent for a declared URL rule. `:66-70` `readDeclaredConstant`
  is the line-anchored reader: `/^const <name> = (\d+)$/gm`, exactly one live match. `:99-109`
  `declaredCacheAssetBounds`; `:86-97` `declaredCacheAssetWarning`, whose `:72-85` comment
  explains why it is a SEPARATE export rather than a third field — follow that precedent.
  `:24-38` `releaseIdentity` hashes only `url` and `sha256`, so a new manifest field cannot
  move the release id.
- `src/release-payload.ts` — `:41`/`:48`/`:67` declare the three existing bounds, one per line.
  Declare the core constants here in the same idiom. `:91` asserts exactly 9 `s1` keys, `:98`
  exactly 2 per `cacheAssets` entry, `:105` exactly 6 per row — do not touch these.
- `scripts/generate-offline-release.mjs` — `:13-18` `assetsFromDist` is the single construction
  site for asset objects. `:152-157` is the existing annotation loop that attaches `brotliBytes`
  and throws when one is missing — the exact pattern to copy. `:184` builds the manifest; `:186`
  writes `sw.js` with the whole manifest embedded.
- `scripts/verify-offline-release.mjs` — `:117-119` reads and enforces the two existing bounds
  on `release.assets.length`; add the core checks alongside. `:175-176` requires `sw.js`'s
  embedded `RELEASE` to byte-equal the manifest, so `tier` reaches the worker for free.
  `:477-487`/`:488-498` are the over/under bound red proofs to model new ones on.
- `scripts/offline-release-contract.test.mjs` — `:30`, `:78`, `:90`, `:106-107` assert return
  shapes with `toEqual` (exact key set), and its failure fixtures are two-constant source
  strings: a reader demanding a third constant makes every one throw for an unrelated reason.
- `asset-tiers.md` (this spec folder) — the measured membership of each tier.

## Tasks & Acceptance

**Execution:**
- [x] `scripts/offline-release-contract.mjs` -- add the tier rule as one exported classifier
  (returning `'core' | 'deferred' | undefined`, where `undefined` means unclassified) plus a
  `declaredCoreCacheAssetBounds` export reading the new constants through `readDeclaredConstant`
  -- one home for the rule, and a separate export so `declaredCacheAssetBounds` keeps its shape.
- [x] `src/release-payload.ts` -- declare `minimumCoreCacheAssets = 29` and
  `maximumCoreCacheAssets = 29` as line-anchored constants, with a comment recording that the
  pin is exact and why -- the declaration is the authority the verifier derives from.
- [x] `scripts/generate-offline-release.mjs` -- annotate every asset with `tier` in an
  annotation loop, throwing on an unclassified asset -- the manifest is where the tiering
  travels.
- [x] `scripts/verify-offline-release.mjs` -- enforce: every asset carries a valid tier; core
  count within the declared core bounds; add red proofs for untiered, core-over and core-under
  -- a bound with no red proof is a bound nobody has seen fail.
- [x] `scripts/offline-release-contract.test.mjs` -- cover the classifier over a table including
  one asset of each deferred group and one core asset, plus the incoherent-declaration cases --
  same discipline the existing bounds get.

**Acceptance Criteria:**
- Given the current dist, when `npm run build` runs, then it succeeds and
  `dist/offline-release-manifest.json` has `tier` on all 80 assets with exactly 29 marked `core`.
- Given the manifest, when `releaseIdentity` is recomputed, then the `tier` field is absent from
  its inputs — only `url` and `sha256` feed the hash, so tiering alone cannot move the release
  id. (The id DOES move this story, because declaring constants in `src/release-payload.ts`
  rehashes the app bundle; that is expected and unrelated to tiering.)
- Given a manifest whose `sw.js` is regenerated, when the verifier compares them, then the
  embedded `RELEASE` still byte-equals the manifest with `tier` present in both.
- Given `npm run verify:offline:red`, when the new red proofs run, then untiered, core-over-bound
  and core-under-bound each fail with their own distinct message.
- Given `npm run test`, when the contract tests run, then every existing assertion in
  `offline-release-contract.test.mjs` still passes unmodified.

## Implementation Notes

**The core pin is 29, not the 28 the frozen intent names — and the frozen block was not edited.**
The owner's "both ends are 28" rested on a count in `asset-tiers.md` that was off by one: its
bundled-examples group was stated as 14 assets where the release carries 13 (four examples at
three slots each, plus the starter). The fourteenth was `/assets/pdf_thumbnail_view-<hash>.js`,
a 2.2 KiB pdf.js preview chunk whose filename contains `thumbnail` — small enough to be
invisible at the two decimals the MiB figures are quoted to, which is why the BYTES agreed
throughout while the counts did not. SPEC.md puts pdf.js in the core tier, and that companion's
own core table never listed the chunk in either tier. `asset-tiers.md` has since been corrected
in place (29 core / 10.67 MiB, 51 deferred / 7.96 MiB, "Corrected 2026-09-19"), and
`src/release-payload.ts` cites that correction beside the pin. **The frozen block still says 28
and needs the owner's confirmation**; nothing else in the story was changed.

**A fourth guard and a fourth red proof beyond the three the story names.** `asset-tier-drift`
holds each manifest entry's recorded tier to what the build-time rule classifies it as. Without
it, "every asset carries a valid tier" would be satisfied by a generator that stamped `core` on
everything, and only the count pin would notice. It is red-proved like the other three, because
a bound with no red proof is a bound nobody has seen fail.

**The tier rules are mutually exclusive by construction, not by list order.** The core font rule
carves out the catalogue prefix and the CJK stem with a lookahead rather than merely sitting
after them, so `classifyAssetTier` can assert that no URL reaches two tiers. An ordering
accident that moved the 4.72 MiB CJK font into the blocking set would otherwise be invisible
until somebody re-measured the first load.

**Owner renegotiated the frozen intent, 2026-09-19 (post-implementation).** The pin is 29,
confirming the count this implementation measured over the 28 the spec was approved with, and
`asset-tiers.md` + `SPEC.md` are corrected (core 29 / 10.67 MiB, deferred 51 / 7.96 MiB,
bundled examples 13, pdf.js 11 assets / 0.76 MiB). The acceptance criterion asserting an
unchanged `release.id` was dropped as unachievable — declaring the constants in
`src/release-payload.ts` rehashes the app bundle, so the id necessarily moves
(`3e4b8b52…` → `eeae8910…`). It was replaced with the claim that is true and load-bearing:
the `tier` field feeds neither identity.

**One test added during verification, by the orchestrator rather than the implementation.**
The reworded matrix row — "the tier field feeds neither `release.id` nor `pageId`" — had no
covering test, so `offline-release-contract.test.mjs` now asserts that `releaseIdentity` and
`pageIdentity` are unchanged both when assets carry tiers and when every tier is flipped to
`core`. The day someone widens `canonicalAssetRows` to include `tier` is the day every cached
release in every browser silently invalidates; that is worth a test rather than a comment.

**Verification run on a clean rebuild** (`rm -rf dist && npm run build`): build ✅,
`verify:offline` ✅, `verify:offline:red` ✅ (the harness fails a proof that escapes
verification OR trips the wrong guard, so a green run proves all four new proofs went red on
their own messages), `lint` ✅, `typecheck` ✅, `test` ✅ 1929 tests / 86 files, with the 34 new
tier tests confirmed executed rather than merely present. Emitted release: 80 assets,
29 `core` / 51 `deferred`, zero untiered.

## Spec Change Log

## Review Triage Log

| # | Finding (layer) | Verdict | Evidence |
|---|---|---|---|
| 1 | CJK carve-out keyed on a literal dot, so a sibling stem lands in core (blind-hunter) | high | Confirmed by running the classifier: `noto-sans-cjk-sc.<20hex>-<hash>.ttf` and `noto-sans-cjk-bold...` both return `'core'`. The lookahead is `(?!catalogue-\|noto-sans-cjk\.)`, which stops blocking as soon as the next character is not `.`. |
| 2 | Same defect, hyphen-suffixed CJK stem (edge-case-hunter) | high | Same run, same result. Grouped with #1. |
| 3 | Deferred rules match by shape, not identity, so a new core asset is silently deferred (verification-gap) | high | Confirmed by running the classifier: `/assets/font-index.<20hex>-<hash>.json`, `/assets/logo.<20hex>-<hash>.png` and `/assets/help-<20hex>.html` all return `'deferred'`. `<stem>.<20hex>` is `build-wasm.mjs`'s repo-wide `fingerprint` convention, not example-specific. The drift guard cannot catch it — it calls the same classifier the generator used. Grouped with #1. |
| 4 | Core pin bounds the blocking set's count, not its weight (blind-hunter, verification-gap) | medium | Real: swapping one core asset for a far larger one keeps the count at 29 and every check green. But no bound anywhere in this release has ever been expressed in bytes — `maximumCacheAssets` counts assets too, and `brotli.totalBytes` is recorded and unbounded. Pre-existing pattern this story followed rather than introduced. Deferred. |
| 5 | Rationale comment in `release-payload.ts` is stale against the corrected companion (blind-hunter, verification-gap) | low | Confirmed: the comment argues against "28 / 52 / 7.97 MiB" while `asset-tiers.md` now reads 29 / 51 / 7.96 MiB with its own correction note. Caused by the orchestrator correcting the companion after the comment was written. Fix is a direct rewording. |
| 6 | "Mutually exclusive by construction" claimed but never asserted; disagreement throw untested (blind-hunter) | low | Confirmed by reading `classifyAssetTier`: it throws only when matched rules disagree on *tier*, so two same-tier rules matching passes silently; no test reaches the throw. The claimed invariant is weaker than the comment states. |
| 7 | `classifyAssetTier`'s throw bypasses `fail()`'s prefix (blind-hunter) | low | Confirmed: `fail` is `throw new Error('offline release verification failed: ' + message)` at `verify-offline-release.mjs:16`; the classifier's own throw escapes without it. |
| 8 | Tier rendered inconsistently across the two new messages (blind-hunter) | low | Confirmed: `untiered-asset` uses `JSON.stringify(asset.tier)` (a deleted field renders as the bare word `undefined`), `asset-tier-drift` mixes `'${asset.tier}'` with `JSON.stringify(classified)`. Grouped with #7. |
| 9 | No bound on the deferred tier (edge-case-hunter) | false | The bad outcome cannot occur: `maximumCacheAssets` bounds the release total at 90 and the core tier is pinned at 29, so the deferred tier is already bounded at 61. A separate deferred bound would be redundant, not missing. |
| 10 | No coherence check between the core envelope and the release-total envelope (edge-case-hunter) | low | Real but unreachable in everyday use — it requires a hand-mis-declared constant, and the declared values are 29 against 90. The fix adds a guard rather than correcting anything, so rejected under the low-finding rule. |
| 11 | The diff omits the spec, companion and memlog (blind-hunter) | false | Not a defect in the change: the orchestrator scoped the diff to `folio-designer/` deliberately, to exclude a concurrent session's unrelated edits to `docs/` and `folio-go/`. The spec documents were reviewed separately in this session. |
| 12 | The generator's own "matches no tier rule" throw has no test (blind-hunter) | low | Real but backstopped: `JSON.stringify` drops an `undefined` property, so a manifest produced without the throw carries no `tier` key and the verifier's `untiered-asset` guard fails it — and that guard has a red proof. Redundant coverage; rejected. |
| 13 | `declaredCoreCacheAssetBounds` duplicates `declaredCacheAssetBounds`; tier arrays exported mutable (blind-hunter) | low | The duplication is real, but the fix is a refactor extracting a shared private reader rather than a direct correction, and no caller is shown to diverge. `Object.freeze` guards a mutation nobody has demonstrated. Rejected under the low-finding rule. |

## Design Notes

**`tier` goes on `release.assets[]`, not into `s1`.** The S1 payload is the load screen's
itemised record and its parser asserts exact key counts (`release-payload.ts:91`, `:98`, `:105`);
widening it is story 2's business, when the page actually needs to know the core set. The
manifest field reaches the worker for free because `sw.js` embeds the whole manifest and the
verifier requires byte-equality (`verify-offline-release.mjs:175-176`).

**The classifier returns `undefined` rather than defaulting.** Defaulting an unknown asset to
`core` would be safe at runtime but silent, and silence in exactly this place is what let the
first load grow from ~9 MB to 18.63 MiB unnoticed. Defaulting to `deferred` would be worse:
an asset the designer needs would stop blocking without anyone deciding that.

Shape of the emitted asset row after this story:

```json
{ "url": "/assets/noto-sans-cjk.5ef5755b…-56t-E-tZ.ttf",
  "sha256": "…", "immutable": true, "brotliBytes": 4953088, "tier": "deferred" }
```

## Verification

**Commands:**
- `cd folio-designer && npm run build` -- expected: succeeds; manifest carries `tier` on every
  asset and 28 of them are `core`.
- `cd folio-designer && npm run verify:offline:red` -- expected: all red proofs pass, including
  the three new ones, each with its own message.
- `cd folio-designer && npm run test` -- expected: Vitest green, with
  `offline-release-contract.test.mjs`'s existing assertions untouched.
- `cd folio-designer && npm run lint && npm run typecheck` -- expected: clean.
- Read back `dist/offline-release-manifest.json` -- expected: `id` unchanged from the value
  recorded before the story, 28 assets with `tier: 'core'`, 0 without a tier.
