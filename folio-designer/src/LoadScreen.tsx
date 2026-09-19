import type { OfflineLifecycle } from './offline-lifecycle'
import { formatMiB, type S1Payload } from './release-payload'
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
  const verifiedAssets = payload?.cacheAssets.filter((asset) => lifecycle.verifiedAssetUrls.includes(asset.assetUrl)) ?? []
  const verified = verifiedAssets.reduce((total, asset) => total + asset.bytes, 0)
  const total = payload?.cachedBytes ?? 0
  const complete = lifecycle.cacheReady
  const announcedVerified = complete ? verified : Math.min(verified, Math.max(0, total - 1))
  const unavailable = lifecycle.state === 'unavailable' || engineState === 'failed'
  const phase = unavailable ? (engineState === 'failed' ? 'Local engine/template could not start' : lifecycle.failure === 'timeout' ? 'Cache check timed out' : lifecycle.failedAssetUrl ? `Cache failed for ${lifecycle.failedAssetUrl}` : 'Offline cache unavailable') : engineState === 'starting' ? 'Starting engine' : lifecycle.state === 'caching' ? 'Caching verified assets' : 'Checking cache'
  const progressMessage = payload ? `${formatMiB(announcedVerified)} of ${formatMiB(total)} verified; ${verifiedAssets.length} of ${payload.assetCount} release assets${complete ? '; complete cache marker verified' : ''}` : phase
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
        <ul className="load-manifest" aria-label="Offline payload manifest">{payload.rows.map((row) => { const state = rowState(lifecycle, row.assetUrl, row.delivery === 'embedded-in-engine'); return <li key={row.id}><span className="load-manifest-name"><b aria-hidden="true">{rowText(state).slice(0, 1)}</b>{row.label}</span><code>{formatMiB(row.bytes)} {rowText(state).slice(2)}</code></li> })}</ul>
        <p className="load-copy">CJK is called out because it is the largest measured font payload in this release.</p></>}
      {unavailable && <button className="load-retry" type="button" autoFocus onClick={onRetry}>Retry preparation</button>}
    </section>
  </main>
}
