import type { OfflineLifecycle } from './offline-lifecycle'
import { coreCacheAssets, coreCachedBytes, formatMiB, type S1Payload } from './release-payload'
import { BrandMark } from './BrandMark'

function rowState(lifecycle: OfflineLifecycle, assetUrl: string, embedded: boolean): 'verified' | 'active' | 'pending' | 'failed' | 'embedded' {
  if (embedded) return 'embedded'
  if (lifecycle.failedAssetUrl === assetUrl) return 'failed'
  if (lifecycle.verifiedAssetUrls.includes(assetUrl)) return 'verified'
  if (lifecycle.activeAssetUrl === assetUrl) return 'active'
  return 'pending'
}
const rowText = (state: ReturnType<typeof rowState>) => state === 'verified' ? '✓ verified' : state === 'active' ? '→ fetching and verifying' : state === 'failed' ? '× unavailable' : state === 'embedded' ? '— embedded in engine; no second request' : '— pending'

export function LoadScreen({ lifecycle, payload, engineState, onRetry }: Readonly<{ lifecycle: OfflineLifecycle; payload?: S1Payload; engineState: 'waiting' | 'starting' | 'failed'; onRetry: () => void }>) {
  // THE DENOMINATOR IS THE BLOCKING SET, NOT THE RELEASE
  // (spec-deferred-offline-cache, story 2). `payload.cachedBytes` and
  // `payload.assetCount` are the whole release — 80 assets and 18.63 MiB — and
  // the worker now waits for the core tier alone. Counting against the release
  // would leave this bar stalled at 57% on a designer that is already
  // interactive: a number that is not merely unhelpful but false about what is
  // being waited for.
  const core = payload ? coreCacheAssets(payload) : []
  const verifiedAssets = core.filter((asset) => lifecycle.verifiedAssetUrls.includes(asset.assetUrl))
  const verified = verifiedAssets.reduce((total, asset) => total + asset.bytes, 0)
  const total = payload ? coreCachedBytes(payload) : 0
  const complete = lifecycle.cacheReady
  const announcedVerified = complete ? verified : Math.min(verified, Math.max(0, total - 1))
  const coreUrls = new Set(core.map((asset) => asset.assetUrl))
  const coreRows = payload?.rows.filter((row) => coreUrls.has(row.assetUrl)) ?? []
  const unavailable = lifecycle.state === 'unavailable' || engineState === 'failed'
  const phase = unavailable ? (engineState === 'failed' ? 'Local engine/template could not start' : lifecycle.failure === 'timeout' ? 'Cache check timed out' : lifecycle.failedAssetUrl ? `Cache failed for ${lifecycle.failedAssetUrl}` : 'Offline cache unavailable') : engineState === 'starting' ? 'Starting engine' : lifecycle.state === 'caching' ? 'Caching verified assets' : 'Checking cache'
  const progressMessage = payload ? `${formatMiB(announcedVerified)} of ${formatMiB(total)} verified; ${verifiedAssets.length} of ${core.length} core release assets${complete ? '; complete cache marker verified' : ''}` : phase
  return <main className="load-screen" aria-labelledby="load-title">
    <section className="load-column">
      {/* STORY 14.5 — THE BRAND IS THERE BEFORE LOADING FINISHES.
          22px here against the document bar's 18px, from the SAME component:
          `size` is a real parameter, not a second drawing. The mark is
          decorative, so this line still announces `Folio8 / OFFLINE` and nothing
          more. The case follows `resources/logo.png`, which sets the wordmark as
          `Folio8`; the ` / OFFLINE` suffix is this screen's own and stays, since
          the mockup's bare wordmark would drop the state the screen reports.
          NEVER on the manifest rows below: the 13px shape in the mockup there is
          the CJK row's in-progress marker, one value of a three-value status
          vocabulary (✓ → × —), and putting the mark there would make the
          product's brand read as "loading". */}
      <p className="load-brand"><span className="brand-lockup"><BrandMark size={22} />Folio8 / OFFLINE</span></p><h1 id="load-title" tabIndex={-1}>Preparing folio8</h1>
      <p className="load-copy">Preparing the offline rendering engine and fonts. This browser normally reuses its verified local cache until browser storage is cleared or evicted.</p>
      <p className="load-phase" role="status" aria-live="polite" aria-atomic="true" aria-label="Offline preparation status">{unavailable ? '× ' : engineState === 'starting' ? '→ ' : '— '}{phase}</p>
      {payload && <><div className="load-progress" role="progressbar" aria-label="Verified offline cache progress" aria-valuemin={0} aria-valuemax={total} aria-valuenow={announcedVerified} aria-valuetext={progressMessage}><span style={{ width: `${total === 0 ? 0 : (announcedVerified / total) * 100}%` }} /></div><p className="load-numeric" role="status" aria-live="polite" aria-atomic="true">{progressMessage}</p>
        {/* THE LIST ITEMISES CORE ROWS ONLY (owner decision, 2026-09-19), so it
            names exactly what is being waited for. The CJK row is the one this
            removes: 4.72 MiB of it, deferred since story 2, and a row sitting
            at `— pending` for an asset nobody is fetching is the screen telling
            the author to wait for something that is not coming. A row whose
            delivery is `embedded-in-engine` reports the ENGINE's asset URL, so
            the dictionary stays by the same test rather than by an exception. */}
        <ul className="load-manifest" aria-label="Offline payload manifest">{coreRows.map((row) => { const state = rowState(lifecycle, row.assetUrl, row.delivery === 'embedded-in-engine'); return <li key={row.id}><span className="load-manifest-name"><b aria-hidden="true">{rowText(state).slice(0, 1)}</b>{row.label}</span><code>{formatMiB(row.bytes)} {rowText(state).slice(2)}</code></li> })}</ul>
        <p className="load-copy">These are the assets the designer waits for. Fonts and examples outside this list are fetched the first time a document needs them.</p></>}
      {unavailable && <button className="load-retry" type="button" autoFocus onClick={onRetry}>Retry preparation</button>}
    </section>
  </main>
}
