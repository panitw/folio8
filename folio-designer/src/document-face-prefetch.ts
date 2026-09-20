/**
 * A DOCUMENT'S DEFERRED CANVAS FACES, FETCHED AS PART OF OPENING IT
 * (spec-deferred-offline-cache, story 3 — CAP-3).
 *
 * WHAT WAS WRONG. Since story 2 the catalogue and the CJK face are deferred:
 * the release carries them, the worker does not precache them, and each arrives
 * the first time something asks. The only thing that ever asked was a PAINT —
 * the browser resolving an `@font-face` rule while drawing text — so opening a
 * document in a face this browser had never held drew it in a substitute first
 * and corrected itself once the bytes landed, with story 2's warning appearing
 * and then being wrong. Establishing what the document needs at OPEN, and
 * fetching it while the network is there, is what CAP-3 asks for.
 *
 * ⚠ WHAT IT ACTUALLY DELIVERS IS LOCAL BYTES, NOT A PAINTED GLYPH. The
 * stylesheet is still what resolves a family, and it still resolves
 * asynchronously at paint time; nothing here registers a face or waits for one.
 * What changes is that the request the browser makes is answered from this
 * release's cache instead of the network, which is what closes the window the
 * substitution was drawn in. Claiming more than that would be claiming the
 * `@font-face` machinery this module deliberately does not touch.
 *
 * ⚠ IT IS BOUNDED TO THE FACES THE DOCUMENT'S OWN TEXT IS MEASURED IN, and that
 * bound is the spec's (`No background prefetch`). This is not a warm-up of the
 * catalogue, not a top-up of anything, and not a fetch of every family the panel
 * can offer. It is narrower than the document's CHAINS, too, and deliberately:
 * every bundled example's chain ends in `Noto Sans SC` as its CJK fallback, so a
 * chain-wide prefetch would hold a first online open for the whole 4.72 MiB CJK
 * face to paint Latin invoices that contain no CJK codepoint at all. A fallback
 * the text never reaches is not a face this document needs; if some later edit
 * reaches it, the existing lazy path fetches it then.
 *
 * ⚠ AND IT IS NEVER GROUNDS FOR A REFUSAL (owner decision, 2026-09-19). A
 * canvas face that will not arrive degrades the GLYPHS ON THIS SCREEN and
 * nothing else: the engine measures and renders from ITS OWN bytes for every
 * face, so layout, pagination, preview and the PDF are byte-identical either
 * way — and story 2 settled substitute-and-warn as the answer.
 *
 * ⚠ "ITS OWN BYTES" NO LONGER MEANS "EMBEDDED", AND THE DISTINCTION IS STORY
 * 5's. `folio-go/fonts/fonts.go` used to embed all eleven faces; the designer's
 * engine is now built `-tags nocjkface` and embeds ten, receiving the CJK face
 * from `absent-face-recovery.ts` the first time a document actually needs it.
 * The sentence above still holds, because a document that reached a PAINT has
 * already passed the engine's projection — and a document whose face the engine
 * does not hold does not reach a paint at all, it is refused by name (CAP-7).
 * There is no state in which this warning is shown over a layout the engine
 * guessed at. So every failure here is
 * swallowed: the open proceeds, and the warning is the browser's own
 * `loadingerror` report through `canvas-face-misses.ts`.
 *
 * THE TIER COMES FROM THE RELEASE, NOT FROM A PATTERN. `payload.cacheAssets`
 * carries story 1's per-asset `tier`, which is the one authority on what is
 * deferred; asking it is also what makes `catalogue-roboto` moving into the core
 * tier at story 3 need no edit here. A page with NO payload — `vite dev`, the
 * unit suites — has no release, therefore no deferral, therefore nothing to
 * prefetch, which is the same argument `held-local-faces.ts` makes for reading
 * "no cache" as "everything is held".
 */
import type { CanvasProjection } from './engine-protocol'
import type { S1Payload } from './release-payload'
import { canvasFaceAssets } from './generated/canvas-face-assets'

/**
 * THE FACES THIS DOCUMENT'S TEXT IS ACTUALLY MEASURED IN, read off the
 * projection's own paint report.
 *
 * ⚠ IT READS `textPaint`, NOT `fontChains`, AND THAT IS THE WHOLE BOUND. A chain
 * is a SEARCH ORDER — the faces the engine may fall through to, per codepoint —
 * and its tail is routinely never reached: the starter and all four bundled
 * examples end theirs in `Noto Sans SC`, whose 4.72 MiB is 59% of the deferred
 * tier and which none of their Latin text touches. `textPaint…fragments[].face`
 * is the opposite kind of statement: it is Go reporting, per fragment, the face
 * it DID measure that text with. Asking the engine's own answer is also the only
 * way to get this right without re-implementing chain resolution in the browser,
 * which AD-17 forbids.
 *
 * ⚠ AND `assetKey` FRAGMENTS ARE NOT FACE NAMES (AD-8). A fragment carries one
 * or the other: `face` names a FontSet face the build ships, `assetKey` names a
 * face the DOCUMENT carries in its own bytes. A carried face needs no fetch —
 * it arrived with the document — and its key lives in a different namespace, so
 * asking the canvas face map about it would be a question in the wrong
 * vocabulary.
 */
export function paintedCanvasFaces(canvas: CanvasProjection | undefined): ReadonlySet<string> {
  const faces = new Set<string>()
  for (const component of canvas?.components ?? []) {
    for (const line of component.textPaint?.lines ?? []) {
      for (const fragment of line.fragments) if (fragment.face !== undefined && fragment.face !== '') faces.add(fragment.face)
    }
  }
  return faces
}

/**
 * The deferred asset URLs this document would need the browser to fetch, in the
 * canvas face map's own order so the set is stable to read and to assert.
 *
 * A face name with no row in the map is a face this build declares no
 * `@font-face` rule for, and there is nothing to fetch for it. A CORE face's URL
 * is already precached and verified, so it is left out too: asking for it would
 * be a request the first load has already paid.
 *
 * ⚠ THE URL STRINGS ON BOTH SIDES ARE THE SAME VITE-EMITTED VALUES, and that
 * agreement is proved against the REAL RELEASE rather than here: a unit fixture
 * that built the tier table out of `canvasFaceAssets` would make this
 * intersection true by construction and would stay green over a prefetch that
 * matched nothing. `scripts/verify-offline-release.mjs` ties every row of the
 * generated map to an emitted asset of the release, and fails the build if one
 * is missing or if none of them is deferred.
 */
export function deferredFaceAssets(canvas: CanvasProjection | undefined, payload: S1Payload | undefined): ReadonlyArray<string> {
  if (payload === undefined) return []
  const deferred = new Set(payload.cacheAssets.filter((asset) => asset.tier === 'deferred').map((asset) => asset.assetUrl))
  const painted = paintedCanvasFaces(canvas)
  const urls = new Set<string>()
  for (const [family, url] of canvasFaceAssets) if (painted.has(family) && deferred.has(url)) urls.add(url)
  return [...urls]
}

/**
 * Fetch them, drop every body, and never throw.
 *
 * THE RESPONSE BODY IS DELIBERATELY DISCARDED, exactly as the font browser's
 * install of a deferred catalogue face discards it: what the open is buying is
 * the CACHE ENTRY the service worker writes — hash-verified against this
 * release's own manifest — and the bytes travel into a paint through the
 * stylesheet afterwards, from cache.
 *
 * AN OFFLINE BROWSER IS NOT ASKED. `navigator.onLine === false` is the one
 * state the platform will state positively, and in it every request here would
 * fail after its own round of retries and back-off while an author waited on a
 * document that opens correctly regardless. `onLine === true` promises nothing,
 * which is why the failures are swallowed rather than trusted away.
 *
 * `fetch` IS INJECTED so this is testable without a network, and the deadline is
 * the caller's for the same reason the file bar's is: a request that never
 * settles must not hold an open for ever.
 */
export async function prefetchDeferredFaces(urls: ReadonlyArray<string>, timeoutMs: number, request: typeof fetch = fetch): Promise<void> {
  // ⚠ `keep: false` IS NOT A TIDY-UP, IT IS THE HEAP. This path reads every
  // deferred face a document needs, in parallel, and wants NONE of the bodies:
  // what it is buying is the cache entry the service worker writes. Collecting
  // them into a map first would take peak heap from the largest body to the
  // SUM of them — tens of MiB on an open, for buffers thrown away one line
  // later.
  await fetchDeferredFaces(urls, timeoutMs, request, false, false)
}

/**
 * THE SAME FETCH, WITH ITS BODIES KEPT (spec-deferred-offline-cache, CAP-6).
 *
 * ⚠ THE BYTES WERE ALWAYS IN HAND HERE AND WERE ALWAYS THROWN AWAY, and until
 * this story that was right: `response.arrayBuffer()` was read only to make the
 * service worker write the CACHE ENTRY, and the paint that followed took the
 * face from that cache through the stylesheet. Nothing in the browser wanted
 * the buffer itself.
 *
 * The engine does now. Its wasm no longer embeds the CJK face, so the same
 * bytes this function already downloads are what `install-face` hands to Go —
 * and downloading them a second time, for the engine, would re-spend the
 * 4.72 MiB the whole spec exists to save. So the map is returned, keyed by the
 * URL each body came from, and `prefetchDeferredFaces` above stays exactly the
 * void-returning call the open path has always made.
 *
 * ⚠ IT STILL NEVER THROWS. A URL whose fetch failed is simply ABSENT from the
 * map: a caller reads "no bytes for this one" and decides for itself, which is
 * what `absent-face-recovery.ts` does when it reports a face it could not get.
 *
 * ⚠ AND `askOffline` IS WHY THIS TAKES A FLAG AT ALL. The prefetch above
 * declines to ask an offline browser, because every request would fail after
 * its own round of retries while an author waited on a document that opens
 * correctly regardless. The ENGINE's face is the opposite case: the spec
 * requires that "a CJK document, offline, with the face already cached opens
 * and renders normally from cache", and the only thing that can answer from
 * that cache is a request — the service worker serves a held deferred asset
 * without touching the network. Declining to ask would turn a cached face into
 * a refusal, which is the one outcome that row rules out.
 *
 * ⚠ AND `keep` IS WHY THE BODIES ARE OPTIONAL. The prefetch above reads each
 * body only to make the service worker write its cache entry and wants none of
 * them; retaining them all across the `Promise.all` would raise peak heap from
 * the largest face to the sum of every face a document declares. The recovery
 * wants exactly one body and keeps it.
 */
export async function fetchDeferredFaces(urls: ReadonlyArray<string>, timeoutMs: number, request: typeof fetch = fetch, askOffline = false, keep = true): Promise<ReadonlyMap<string, ArrayBuffer>> {
  const fetched = new Map<string, ArrayBuffer>()
  if (urls.length === 0) return fetched
  if (!askOffline && typeof navigator !== 'undefined' && navigator.onLine === false) return fetched
  const deadline = new AbortController()
  const handle = setTimeout(() => deadline.abort(), timeoutMs)
  try {
    await Promise.all(urls.map(async (url) => {
      try {
        const response = await request(url, { credentials: 'omit', signal: deadline.signal })
        // A non-OK response has nothing worth reading and nothing worth saying:
        // the paint that follows will substitute and the browser will report it.
        if (!response.ok) return
        const body = await response.arrayBuffer()
        if (keep) fetched.set(url, body)
      } catch { /* a face that will not arrive is story 2's warning, never this open's failure */ }
    }))
  } finally { clearTimeout(handle) }
  return fetched
}
