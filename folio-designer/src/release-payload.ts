export type S1Row = Readonly<{ id: 'engine' | 'latin-font' | 'thai-font' | 'cjk-font' | 'noto-sans-bold-font' | 'noto-sans-italic-font' | 'noto-sans-bold-italic-font' | 'noto-sans-thai-bold-font' | 'roboto-bold-font' | 'roboto-italic-font' | 'roboto-bold-italic-font' | 'thai-dictionary'; label: 'Engine' | 'Latin font' | 'Thai font' | 'CJK font' | 'Noto Sans Bold' | 'Noto Sans Italic' | 'Noto Sans Bold Italic' | 'Noto Sans Thai Bold' | 'Roboto Bold' | 'Roboto Italic' | 'Roboto Bold Italic' | 'Thai dictionary'; delivery: 'cached-asset' | 'embedded-in-engine'; assetUrl: string; bytes: number; sha256: string }>
// THE TIER TRAVELS WITH EVERY CACHE ASSET SINCE STORY 2 OF
// spec-deferred-offline-cache. Story 1 stamped it into the manifest the WORKER
// embeds and deliberately left this parser alone; the worker now precaches and
// gates on the core tier, so without the tier HERE the page could not say what
// `cacheReady` covers, could not count progress against the set actually being
// waited for, and could not itemise the load screen's rows to it.
export type S1CacheAsset = Readonly<{ assetUrl: string; bytes: number; tier: 'core' | 'deferred' }>
export type S1Payload = Readonly<{ version: 1; releaseId: string; pageId: string; unit: 'MiB'; decimals: 2; cachedBytes: number; assetCount: number; cacheAssets: readonly S1CacheAsset[]; rows: readonly S1Row[] }>
// THE BLOCKING SET, AS THE PAGE SEES IT — one derivation, read by the lifecycle
// reducer and by the load screen, so the two cannot disagree about what is
// being waited for. It is NOT a bound check: `coreCacheAssetFloor`/
// `coreCacheAssetCeiling` pin the BUILD, and a page that rejected a payload for
// carrying 28 core assets would refuse to start over a number the build has
// already refused to emit.
export const coreCacheAssets = (payload: S1Payload): readonly S1CacheAsset[] => payload.cacheAssets.filter((asset) => asset.tier === 'core')
export const coreCachedBytes = (payload: S1Payload): number => coreCacheAssets(payload).reduce((total, asset) => total + asset.bytes, 0)
// EVERY REJECTION CARRIES ITS OWN NAME. The bound and the fifteen unrelated shape
// checks used to share one bare `undefined`, so "this release lists more assets
// than the reader accepts" and "this is not a payload at all" were the same
// value — an all-clear indistinguishable from a couldn't-look, on the first
// screen a user sees. `asset-count-over-maximum` in particular is reachable by
// that cause and no other, which is what makes it evidence.
export type S1PayloadRejection =
  | 'not-an-object'
  | 'payload-shape'
  | 'asset-count-over-maximum'
  | 'asset-count-under-minimum'
  | 'cache-assets-invalid'
  // KEPT DISTINCT FROM `cache-assets-invalid` ON PURPOSE (story 2), AND NAMED
  // FOR WHAT IT ACTUALLY CATCHES. The exact-key-count check above runs first, so
  // an entry with NO tier key is a shape fault and rejects as
  // `cache-assets-invalid`; what reaches this arm is an entry that carries a
  // tier this reader does not recognise — a release emitted by a build whose
  // tier vocabulary has moved. It was first written as
  // `cache-asset-tier-missing`, which was a lie about its own trigger: nothing
  // missing ever reaches it.
  | 'cache-asset-tier-unrecognised'
  | 'cached-bytes-mismatch'
  | 'row-not-an-object'
  | 'row-shape'
  | 'row-delivery-composition'
  | 'no-bootstrap'
  | 'malformed-json'
export type S1PayloadResult = Readonly<{ ok: true; payload: S1Payload }> | Readonly<{ ok: false; reason: S1PayloadRejection }>

const hash = /^[a-f0-9]{64}$/
// TWELVE ROWS SINCE STORY 11.1: the four the release always carried, the seven
// weighted and sloped cuts, and the dictionary. Itemised per face rather than
// aggregated (D-11.1.15) — the manifest is a record before it is a screen, and
// "which face cost what" is the one question an aggregate cannot answer.
// Ordered, and read positionally against `rows` below: `engine` first and
// `thai-dictionary` last is the emitted shape, and an id in the wrong slot reds
// here. The two rows this module makes a CLAIM about are keyed by id instead.
const ids = ['engine', 'latin-font', 'thai-font', 'cjk-font', 'noto-sans-bold-font', 'noto-sans-italic-font', 'noto-sans-bold-italic-font', 'noto-sans-thai-bold-font', 'roboto-bold-font', 'roboto-italic-font', 'roboto-bold-italic-font', 'thai-dictionary'] as const
const labels = ['Engine', 'Latin font', 'Thai font', 'CJK font', 'Noto Sans Bold', 'Noto Sans Italic', 'Noto Sans Bold Italic', 'Noto Sans Thai Bold', 'Roboto Bold', 'Roboto Italic', 'Roboto Bold Italic', 'Thai dictionary'] as const
// scripts/offline-release-contract.mjs DERIVES these numbers from these lines —
// the two bounds here and the approach-warning threshold below them — and
// scripts/verify-offline-release.mjs fails the build on a release outside the
// bounds, so a release over the bound can no longer be emitted in silence.
// Keep each on its own line as `const <name> = <digits>`: that reader is
// line-anchored and requires exactly one live match, so reformatting, renaming
// or duplicating any of these lines fails the build loudly rather than
// disabling it.
const minimumCacheAssets = 10
// RAISED 65 → 90 BY THE STARTUP TEMPLATES (story 1, the example bundling
// pipeline), WITH STATED HEADROOM RATHER THAN TUNED TO THE COUNT. The release
// already sat at 65 of 65; each bundled example spends three slots (template,
// sample JSON, thumbnail), so the four planned examples take twelve, and the
// remaining margin is room for the next unrelated batch rather than a ceiling
// fitted to this one.
//
// RAISED 90 → 166 BY THE COMMITTED TIER'S CUTS (spec-install-all-face-cuts,
// story 3). The catalogue went from one upright Regular per family to every cut
// those 31 families publish — 31 rows to 107, ONE ROW BEING ONE DIST ASSET
// BEING ONE CACHE SLOT — and the release went from 80 emitted assets to 156,
// measured from `dist/offline-release-manifest.json` after `npm run build` and
// not projected. 166 is that measurement plus the SAME 10-slot reserve today's
// 90 carried over 80: the margin is deliberately the one this file already
// used, so the ceiling is a decision about headroom rather than a number
// fitted to one batch.
//
// ⚠ IT IS DERIVED FROM 156, NOT FROM THE 158 THE STORY'S FROZEN INTENT
// PROPOSED. That figure predates the owner's DM Sans ruling: `DMSans-Italic.ttf`
// cannot prove its own style upstream — the ITALIC bit is clear and macStyle is
// 0x0 while `italicAngle` is -10 — so the italic and its bold-italic sibling are
// deliberately withheld rather than admitted by exception, and the tier ships
// that family's Regular and Bold alone. Two fewer faces, two fewer slots.
//
// ⚠ THE BLOCKING CORE TIER DID NOT MOVE. All 76 new faces are `deferred`
// (`scripts/offline-release-contract.mjs`), so the core pins below are still
// 30/30 and none of this is in the first load.
const maximumCacheAssets = 166
// THE APPROACH WARNING'S THRESHOLD (Story 11.1, D-11.1.10). NOT A BOUND:
// nothing in this module reads it, nothing rejects a payload for crossing it,
// and `maximumCacheAssets` above is still the only number that refuses a
// release. `scripts/verify-offline-release.mjs` reads it — through
// `declaredCacheAssetWarning`, never as a literal of its own — and WARNS,
// naming the remaining margin, when a release reaches it.
//
// 56 IS "ONE STORY LIKE THIS ONE BELOW THE CEILING". Story 11.1 spent seven
// slots in a single change and left the release at 61 of 64, so a threshold
// eight below the maximum is the point at which the next comparable batch
// would no longer fit. It is declared here, beside the two bounds, because
// this is the file the derivation reader is anchored to, and it obeys the same
// `const <name> = <digits>` shape on a line of its own for the same reason
// they do.
//
// RAISED 56 → 82 WITH THE MAXIMUM (startup templates, story 1): still eight
// below the ceiling, the same one-comparable-batch margin, after the twelve
// slots four examples take.
//
// RAISED 82 → 158 WITH THE MAXIMUM AGAIN (spec-install-all-face-cuts, story 3):
// still exactly eight below the ceiling, the same margin this threshold has
// carried since Story 11.1, over the measured 156. So the release emitted by
// this story is two slots under the warning and ten under the bound, and the
// next comparable batch is the one that has to say so out loud.
const warnCacheAssets = 158
// THE DECLARATION ABOVE IS SHAPED FOR A TEXT READER IN ANOTHER LANGUAGE
// (`scripts/offline-release-contract.mjs` matches `^const <name> = <digits>$`),
// not for a TypeScript importer, so nothing in `src/` reads it. It is exported
// here under a name that says what the number is for.
//
// AND IT HAS A CONSUMER, which is the only reason it is exported at all: a
// public symbol that exists to satisfy `noUnusedLocals` is a symbol that will
// be deleted by the next person who greps for its callers and finds none.
// `scripts/verify-offline-release.test.mjs` imports it and asserts it EQUALS
// `declaredCacheAssetWarning().warnCacheAssets` — the value the regex reader
// pulls out of this file's source text. That is the one assertion in the
// repository that puts the text-derived number beside the value this module
// actually evaluates to, and it is what makes the cross-language derivation a
// measurement rather than a convention.
//
// A REFERENCE, never a second copy of the value: the `const` line stays the
// single authority, and changing it changes this too.
export const cacheAssetApproachWarning = warnCacheAssets

// AND THE BOUND ITSELF, EXPORTED ON THE SAME TERMS AND FOR THE SAME REASON
// (spec-install-all-face-cuts, story 3). `src/release-payload.test.ts` builds
// the over-the-bound red proof by manufacturing a payload with one asset too
// many, and that fixture spelled the number out — `assetCount = 91` beside a
// `const maximumCacheAssets = 90` — which is a SECOND AUTHORITY on the bound
// and went stale the moment this story raised it. The fixture now derives its
// length from here, so a raise moves the proof with the number it proves.
//
// A REFERENCE, never a second copy of the value.
export const cacheAssetCeiling = maximumCacheAssets

// THE CORE TIER'S BOUND, AND IT IS A PIN RATHER THAN AN ENVELOPE
// (spec-deferred-offline-cache story 1, owner decision 2026-09-19).
//
// `minimumCacheAssets`/`maximumCacheAssets` above bound the release TOTAL and
// are unchanged. These two bound the BLOCKING SET: the assets a first-time
// visitor must have before the designer can be used at all.
// `scripts/offline-release-contract.mjs` classifies every emitted asset as
// `core` or `deferred`, `scripts/generate-offline-release.mjs` stamps that into
// the manifest, and `scripts/verify-offline-release.mjs` DERIVES these two
// numbers from these lines and refuses a release outside them.
//
// BOTH ENDS ARE THE SAME NUMBER ON PURPOSE. Any growth OR shrinkage of the
// blocking set fails the build until someone moves this number deliberately,
// with a rationale comment of their own. Headroom was considered and rejected:
// an unwatched number is precisely how the first load reached 18.63 MiB while
// `spec-folio` still recorded it as "~9 MB". The coherence check in
// `declaredCoreCacheAssetBounds` compares `>` and not `>=`, so an envelope with
// equal ends is expressible and legal.
//
// THE NUMBER IS MEASURED, AND `asset-tiers.md` IN THE SPEC FOLDER IS WHERE THE
// MEASUREMENT LIVES. It recorded 29 assets / 10.67 MiB against a deferred tier
// of 51 / 7.96 MiB under its own "Corrected 2026-09-19" note: an earlier
// measurement counted `/assets/pdf_thumbnail_view-<hash>.js` into the
// bundled-example group by its `thumbnail` substring, and that 2.2 KiB pdf.js
// preview chunk is core — SPEC.md puts pdf.js in the core tier. The correction
// moved only the COUNTS; the MiB figures were right throughout, which is why the
// miscount survived as long as it did. Whoever moves this number next:
// re-measure both tiers and correct that companion in the same change, or the
// next reader inherits the same mismatch.
//
// 29 → 30 AT STORY 3, AND THE ASSET IS NAMED: `catalogue-roboto`, the canvas
// copy of Roboto, 0.152 MiB, moved from `deferred` to `core` (owner decision,
// 2026-09-19). `runtime-fonts.css` maps the family `Roboto` to that catalogue
// face while `Roboto Bold`, `Roboto Italic` and `Roboto Bold Italic` map to
// shipped core files, so story 1 left one family straddling the tiers — and the
// starter and all four bundled examples declare that chain, which meant the
// DEFAULT DOCUMENT could not paint its body text without a deferred fetch. The
// tiers are now 30 / 10.82 MiB core against 50 / 7.81 MiB deferred, and
// `asset-tiers.md` was re-measured in the same change.
const minimumCoreCacheAssets = 30
const maximumCoreCacheAssets = 30
// THE CORE TIER'S WEIGHT, WHICH NOTHING GUARDED UNTIL NOW
// (spec-deferred-offline-cache, story 5). The two numbers above bound the
// blocking set's COUNT and say nothing at all about its size: the wasm is one
// asset whether it is 3 MiB or 8, so the 4.72 MiB of CJK glyphs that story 5
// took out of it could be put back with every existing pin still green. The
// build's own record said as much — `generate-offline-release.mjs` described
// its Brotli total as "a MEASUREMENT, not a budget, and nothing in this
// repository compares it to a threshold".
//
// ⚠ IT IS A CEILING, NOT AN EQUALITY, and that is the difference between this
// number and the count above. The count is pinned exactly because an asset
// entering or leaving the blocking set is always a decision; the WEIGHT moves
// by a few kilobytes every time the application's own JavaScript changes, and a
// pin that red every commit would be removed rather than respected.
//
// ⚠ THE FIGURE IS MEASURED, NEVER PROJECTED. `npm run build` at story 5 emitted
// 29 immutable core assets totalling 6,392,910 Brotli bytes (6.097 MiB), down
// from 11,335,794 (10.811 MiB) before it — the engine wasm alone falling from
// 8,508,122 to 3,564,400. The ceiling is that measurement plus 160,690 bytes,
// about 2.5%: room for ordinary bundle movement and nowhere near the ~4.9 MiB
// that re-embedding the CJK face would add. `/index.html` is outside the sum
// for the reason it is outside every other Brotli total here — it is the one
// mutable asset and carries no sidecar.
//
// RAISING IT IS THE DELIBERATE ACT THAT ADMITS A HEAVIER FIRST LOAD, exactly as
// raising `maximumCacheAssets` is the act that admits a longer one. Re-measure
// before moving it, and move `asset-tiers.md` in the same change.
const maximumCoreCacheBytes = 6553600
// AND ITS APPROACH WARNING, ON THE EXACT TERMS `warnCacheAssets` ALREADY HAS
// (spec-install-all-face-cuts story 3, owner-authorised at review).
//
// THE COUNT BOUND HAS HAD A WARNING SINCE STORY 11.1 AND THE BYTE CEILING HAD
// NONE, so its first and only signal was a hard `npm run build` failure with
// the release already assembled. That asymmetry stopped being academic when
// this story landed 76 committed faces: `src/generated/font-catalogue.ts`
// inlines a ~4 KB licence text, a copyright and a source string PER FACE and is
// bundled into the core tier, which took the blocking download to 6,407,803
// Brotli bytes — 97.8% of this ceiling, with 145,797 bytes left. The next
// batch of any size finds out by breaking the build.
//
// 6,422,528 IS THE CEILING LESS 131,072 BYTES (128 KiB), AND THE TWO NUMBERS
// IT IS DERIVED FROM ARE BOTH MEASURED. This story cost the core tier ~14,100
// Brotli bytes (6,392,910 -> 6,407,803) for 76 faces' worth of inlined licence,
// copyright and provenance text. Today's margin under the CEILING is 145,797
// bytes, so:
//
//   · the build is SILENT on the release that declares this threshold — a
//     warning firing on its own commit is one nobody reads;
//   · the NEXT batch the size of this one crosses the threshold and the build
//     says so, which is the signal that was missing;
//   · the ceiling itself is about nine such batches away, so the warning
//     arrives with room to act rather than as a post-mortem.
//
// `warnCacheAssets` is "one comparable batch below the ceiling" because a batch
// there is a decision somebody makes. The core WEIGHT moves on its own with
// every bundle change, so this one is deliberately several batches wide: a
// threshold 14 KB under the ceiling would be crossed and breached by the same
// commit, which is the failure it exists to precede.
//
// IT IS NOT A BOUND AND NOTHING FAILS ON IT. `maximumCoreCacheBytes` above is
// still the only number that refuses a release; this one decides when the build
// says out loud how much margin is left, and it warns with the MARGIN rather
// than the weight for the reason the asset warning does — the number nobody
// prints is the number nobody watches.
//
// It obeys the same `const <name> = <digits>` shape on a line of its own,
// because `scripts/offline-release-contract.mjs` reads it as text.
const warnCoreCacheBytes = 6422528
// EXPORTED FOR THE REASON `cacheAssetApproachWarning` IS, AND WITH THE SAME
// OBLIGATION: nothing in `src/` reads the `const` lines above (they are shaped
// for a text reader in another language), so without a consumer `noUnusedLocals`
// would be the only thing holding them. `scripts/verify-offline-release.test.mjs`
// asserts these equal `declaredCoreCacheAssetBounds()` — the values the regex
// reader pulls out of this file's source text — which is what makes the
// cross-language derivation a measurement rather than a convention.
//
// REFERENCES, never second copies: the `const` lines stay the single authority.
export const coreCacheAssetFloor = minimumCoreCacheAssets
export const coreCacheAssetCeiling = maximumCoreCacheAssets
export const coreCacheByteCeiling = maximumCoreCacheBytes
// EXPORTED UNDER THE SAME OBLIGATION AS `cacheAssetApproachWarning`: nothing in
// `src/` reads the `const` line above (it is shaped for a text reader in another
// language), so without a consumer `noUnusedLocals` would be the only thing
// holding it. `scripts/verify-offline-release.test.mjs` asserts it equals
// `declaredCoreCacheByteWarning().warnCoreCacheBytes` — the value the regex
// reader pulls out of this file's source text — which is what makes the
// cross-language derivation a measurement rather than a convention.
export const coreCacheByteApproachWarning = warnCoreCacheBytes
const reject = (reason: S1PayloadRejection): S1PayloadResult => ({ ok: false, reason })

export function parseS1Payload(value: unknown): S1PayloadResult {
  if (!value || typeof value !== 'object') return reject('not-an-object')
  const candidate = value as Record<string, unknown>
  if (Object.keys(candidate).length !== 9 || candidate.version !== 1 || !hash.test(String(candidate.releaseId)) || !hash.test(String(candidate.pageId)) || candidate.unit !== 'MiB' || candidate.decimals !== 2 || typeof candidate.cachedBytes !== 'number' || !Number.isSafeInteger(candidate.cachedBytes) || candidate.cachedBytes <= 0 || typeof candidate.assetCount !== 'number' || !Number.isSafeInteger(candidate.assetCount) || !Array.isArray(candidate.cacheAssets) || candidate.cacheAssets.length !== candidate.assetCount || !Array.isArray(candidate.rows) || candidate.rows.length !== ids.length) return reject('payload-shape')
  // The bound lives OUTSIDE the shape condition above so each arm is reachable by
  // its own cause alone. The condition above has already established that
  // `assetCount` is a safe integer and that `cacheAssets` has exactly that length.
  if (candidate.assetCount > maximumCacheAssets) return reject('asset-count-over-maximum')
  if (candidate.assetCount < minimumCacheAssets) return reject('asset-count-under-minimum')
  const cacheAssets = candidate.cacheAssets as unknown[]
  // THE KEY COUNT MOVED 2 → 3 DELIBERATELY (story 2), and it is still an EXACT
  // count rather than a minimum: an entry carrying a key this reader does not
  // know about is a release this page does not understand, and reading it
  // anyway is how a payload shape drifts without anyone noticing.
  if (new Set(cacheAssets.map((asset) => typeof asset === 'object' && asset ? (asset as Record<string, unknown>).assetUrl : undefined)).size !== candidate.assetCount || !cacheAssets.every((asset) => { const item = asset as Record<string, unknown>; return asset && typeof asset === 'object' && Object.keys(item).length === 3 && typeof item.assetUrl === 'string' && item.assetUrl.startsWith('/') && item.assetUrl.length <= 256 && typeof item.bytes === 'number' && Number.isSafeInteger(item.bytes) && item.bytes > 0 })) return reject('cache-assets-invalid')
  // ITS OWN ARM, REACHABLE BY ITS OWN CAUSE ALONE — the same argument the two
  // bound rejections above are built on. The shape check has already
  // established every entry is an object with exactly three keys, two of which
  // it named; this is the third.
  if (!cacheAssets.every((asset) => { const tier = (asset as Record<string, unknown>).tier; return tier === 'core' || tier === 'deferred' })) return reject('cache-asset-tier-unrecognised')
  if (cacheAssets.reduce<number>((total, asset) => total + (asset as { bytes: number }).bytes, 0) !== candidate.cachedBytes) return reject('cached-bytes-mismatch')
  const rows: S1Row[] = []
  for (let index = 0; index < ids.length; index++) {
    const row = candidate.rows[index]
    if (!row || typeof row !== 'object') return reject('row-not-an-object')
    const item = row as Record<string, unknown>
    if (Object.keys(item).length !== 6 || item.id !== ids[index] || item.label !== labels[index] || (item.delivery !== 'cached-asset' && item.delivery !== 'embedded-in-engine') || typeof item.assetUrl !== 'string' || !item.assetUrl.startsWith('/') || item.assetUrl.length > 256 || typeof item.bytes !== 'number' || !Number.isSafeInteger(item.bytes) || item.bytes <= 0 || typeof item.sha256 !== 'string' || !hash.test(item.sha256)) return reject('row-shape')
    rows.push(item as S1Row)
  }
  // ONE LINE, THREE COUPLINGS, ALL THREE KEYED BY ID SINCE STORY 11.1
  // (D-11.1.16). It read `cached.length !== 4 || rows[4].… || rows[0].…`, and
  // the two indices were the whole problem: `rows[4]` meant "the Thai
  // dictionary row" and `rows[0]` meant "the engine wasm row", and NEITHER said
  // so. Seven rows were inserted between them in this story and the indices
  // will move again in 11.3; an index that has drifted onto another row does
  // not fail, it checks the wrong row and passes.
  //
  // The composition claim itself is unchanged: every row is a cached asset
  // except the dictionary, which is embedded in the engine and therefore
  // reports the ENGINE's own asset URL rather than one of its own. That is why
  // the third clause compares the two — the dictionary is not a second
  // download, and a row saying otherwise would be a delivery fiction on the
  // first screen a user sees.
  const cached = rows.filter((row) => row.delivery === 'cached-asset')
  const dictionary = rows.find((row) => row.id === 'thai-dictionary')
  const engine = rows.find((row) => row.id === 'engine')
  if (cached.length !== ids.length - 1 || !dictionary || !engine || dictionary.delivery !== 'embedded-in-engine' || dictionary.assetUrl !== engine.assetUrl) return reject('row-delivery-composition')
  return { ok: true, payload: { version: 1, releaseId: candidate.releaseId as string, pageId: candidate.pageId as string, unit: 'MiB', decimals: 2, cachedBytes: candidate.cachedBytes, assetCount: candidate.assetCount, cacheAssets: cacheAssets as S1CacheAsset[], rows } }
}

export function loadS1Payload(): S1PayloadResult {
  const node = document.getElementById('folio8-release-bootstrap')
  const text = node?.textContent
  if (!text) return reject('no-bootstrap')
  let bootstrap: unknown
  // THE TRY WRAPS EXACTLY ONE CALL, AND THE CATCH ADMITS EXACTLY ONE ERROR.
  // It used to wrap the parse AND parseS1Payload and convert anything thrown to a
  // bare `undefined` — but parseS1Payload never throws, so the only throw this
  // ever existed for is JSON.parse's SyntaxError. Anything else is a fault in the
  // page, not a malformed bootstrap, and is rethrown rather than disguised as one.
  //
  // It is narrowed rather than removed because removal is measurably worse.
  // main.tsx calls render() BEFORE its try, the block has a finally and no catch,
  // and startObservation is invoked as `void startObservation()`, so a propagating
  // SyntaxError would never reach registerOfflineLifecycle: instead of the stated
  // "Offline cache unavailable" phase and its Retry preparation button, the load
  // screen would sit on "Checking cache" forever with no message and no retry —
  // an unstated hang traded for a stated failure, in the name of removing silence.
  try { bootstrap = JSON.parse(text) } catch (error) { if (error instanceof SyntaxError) return reject('malformed-json'); throw error }
  // `JSON.parse('null')` succeeds, and reading `.s1` off it would throw a
  // TypeError the narrowed catch no longer covers. Guard the read instead of
  // widening the catch back: a null bootstrap must keep reaching the parser as a
  // rejection, exactly as it did before, or narrowing would have changed what a
  // user sees on a path this story is not scoped to change.
  return parseS1Payload(bootstrap && typeof bootstrap === 'object' ? (bootstrap as { s1?: unknown }).s1 : undefined)
}

// THE TWO DECISIONS main.tsx MAKES ABOUT A RESULT, WHERE A TEST CAN EXECUTE THEM.
// Nothing in this repo imports main.tsx, and Vitest collects only
// `src/**/*.test.{ts,tsx}` and `scripts/**/*.test.mjs`, so a decision left inline
// there is run by NO gate: `payloadForLifecycle` collapsed to a bare `undefined`
// would type-check, pass every suite, and ship an app permanently reporting
// "Offline cache unavailable". Both decisions are pure functions of the result,
// so they live here and are asserted in release-payload.test.ts.
export function payloadForLifecycle(result: S1PayloadResult): S1Payload | undefined {
  // registerOfflineLifecycle and the load screen already speak `undefined` for
  // "no usable payload", so a rejection maps to that at exactly one place and
  // nothing downstream changes. The reason is not discarded on the way past — it
  // is what the predicate below reads.
  return result.ok ? result.payload : undefined
}

// The dev-server bypass covers ONE cause: the dev server emits no bootstrap node
// at all. A bootstrap that is malformed, or over the release bound, is a real
// fault and must never be read as "the dev server did not emit one" and quietly
// bypassed — so this is true for `no-bootstrap` and for no other reason.
export function isDevBypassReason(result: S1PayloadResult): boolean {
  return !result.ok && result.reason === 'no-bootstrap'
}

export function formatMiB(bytes: number, decimals = 2): string { return `${(bytes / (1024 * 1024)).toFixed(decimals)} MiB` }
