import type { S1CacheAsset, S1Payload } from '../release-payload'
import { canvasFaceAssets } from '../generated/canvas-face-assets'

/**
 * ONE S1 PAYLOAD FIXTURE FOR THE PREFETCH TESTS, AND IT IS A REAL `S1Payload`.
 *
 * Both suites that need one used to build their own through `as unknown as
 * S1Payload`, which defeated the one check they most wanted: that the fixture
 * still has the shape the module under test reads. A double cast turns a payload
 * whose `cacheAssets` rows have drifted into a green test. This is contextually
 * typed by the return annotation instead, so a drift in `S1CacheAsset` or
 * `S1Payload` reds here at compile time.
 *
 * ⚠ IT TIERS THE CANVAS FACE MAP, WHICH MAKES THE INTERSECTION TRUE BY
 * CONSTRUCTION, AND THAT IS WHY IT IS NOT THE PROOF OF ANYTHING. Under Vitest a
 * `?url` import resolves to a dev-server path (`/src/generated/runtime/…`), not
 * to the `/assets/<stem>-<hash>.<ext>` a build emits, so no unit fixture can
 * honestly assert that the page's URLs and the release manifest's agree. That
 * agreement is a property of the BUILD and is proved there, in
 * `scripts/verify-offline-release.mjs`. What these tests are for is the
 * selection rule — which declared face, in which tier, is asked for — and this
 * fixture exists to hold the tier half of that fixed.
 *
 * `rows` is empty on purpose: nothing in the prefetch path reads the load
 * screen's itemisation, and a twelve-row fixture here would be twelve more
 * lines to re-check every time that list moves.
 */
export function tieredCanvasFacePayload(deferredFamilies: ReadonlyArray<string>): S1Payload {
  const cacheAssets: ReadonlyArray<S1CacheAsset> = [...canvasFaceAssets].map(([family, assetUrl]): S1CacheAsset => ({ assetUrl, bytes: 1, tier: deferredFamilies.includes(family) ? 'deferred' : 'core' }))
  return { version: 1, releaseId: 'a'.repeat(64), pageId: 'b'.repeat(64), unit: 'MiB', decimals: 2, cachedBytes: cacheAssets.length, assetCount: cacheAssets.length, cacheAssets, rows: [] }
}
