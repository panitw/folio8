export type S1Row = Readonly<{ id: 'engine' | 'latin-font' | 'thai-font' | 'cjk-font' | 'noto-sans-bold-font' | 'noto-sans-italic-font' | 'noto-sans-bold-italic-font' | 'noto-sans-thai-bold-font' | 'roboto-bold-font' | 'roboto-italic-font' | 'roboto-bold-italic-font' | 'thai-dictionary'; label: 'Engine' | 'Latin font' | 'Thai font' | 'CJK font' | 'Noto Sans Bold' | 'Noto Sans Italic' | 'Noto Sans Bold Italic' | 'Noto Sans Thai Bold' | 'Roboto Bold' | 'Roboto Italic' | 'Roboto Bold Italic' | 'Thai dictionary'; delivery: 'cached-asset' | 'embedded-in-engine'; assetUrl: string; bytes: number; sha256: string }>
export type S1Payload = Readonly<{ version: 1; releaseId: string; pageId: string; unit: 'MiB'; decimals: 2; cachedBytes: number; assetCount: number; cacheAssets: readonly Readonly<{ assetUrl: string; bytes: number }>[]; rows: readonly S1Row[] }>
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
const maximumCacheAssets = 90
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
const warnCacheAssets = 82
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
// 29 IS THE MEASURED BLOCKING SET. `asset-tiers.md` in the spec folder records
// it as 29 assets / 10.67 MiB against a deferred tier of 51 / 7.96 MiB, under
// its own "Corrected 2026-09-19" note: an earlier measurement counted
// `/assets/pdf_thumbnail_view-<hash>.js` into the bundled-example group by its
// `thumbnail` substring, and that 2.2 KiB pdf.js preview chunk is core —
// SPEC.md puts pdf.js in the core tier. The correction moved only the COUNTS;
// the MiB figures were right throughout, which is why the miscount survived as
// long as it did. Whoever moves this number next: re-measure both tiers and
// correct that companion in the same change, or the next reader inherits the
// same mismatch.
const minimumCoreCacheAssets = 29
const maximumCoreCacheAssets = 29
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
  if (new Set(cacheAssets.map((asset) => typeof asset === 'object' && asset ? (asset as Record<string, unknown>).assetUrl : undefined)).size !== candidate.assetCount || !cacheAssets.every((asset) => { const item = asset as Record<string, unknown>; return asset && typeof asset === 'object' && Object.keys(item).length === 2 && typeof item.assetUrl === 'string' && item.assetUrl.startsWith('/') && item.assetUrl.length <= 256 && typeof item.bytes === 'number' && Number.isSafeInteger(item.bytes) && item.bytes > 0 })) return reject('cache-assets-invalid')
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
  return { ok: true, payload: { version: 1, releaseId: candidate.releaseId as string, pageId: candidate.pageId as string, unit: 'MiB', decimals: 2, cachedBytes: candidate.cachedBytes, assetCount: candidate.assetCount, cacheAssets: cacheAssets as { assetUrl: string; bytes: number }[], rows } }
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
