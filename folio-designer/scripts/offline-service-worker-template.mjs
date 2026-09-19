const messageVersion = 1

export function isCacheableStaticRequest(request, origin, paths) {
  const url = new URL(request.url)
  return request.method === 'GET' && url.origin === origin && paths.has(url.pathname) && request.mode !== 'navigate'
}

// A same-origin NAVIGATION to a precached `/assets/*.html` entry — the bundled
// documentation pages, opened in their own tab. Static requests above still
// refuse every navigation; this admits only exact manifest paths ending in .html.
export function isCacheableDocumentNavigation(request, origin, paths) {
  const url = new URL(request.url)
  return request.method === 'GET' && request.mode === 'navigate' && url.origin === origin && url.pathname.startsWith('/assets/') && url.pathname.endsWith('.html') && paths.has(url.pathname)
}

export function isStatusRequest(value) {
  return Boolean(value) && typeof value === 'object' && value.version === 1 && value.type === 'get-offline-status' && Object.keys(value).length === 2
}

// ASKED OF A WORKER THAT IS NOT THIS PAGE'S OWN, which is the whole reason it
// exists. A page only trusts broadcasts whose releaseId and pageId are its own,
// so a WAITING worker — a different release by definition — has no way to tell
// an open tab what version it carries. This request is answered on the caller's
// MessageChannel port instead of broadcast, so the answer reaches exactly the
// page that asked and no bystander tab can be moved by it.
export function isVersionRequest(value) {
  return Boolean(value) && typeof value === 'object' && value.version === 1 && value.type === 'get-release-version' && Object.keys(value).length === 2
}

// THE ONLY THING THAT MAY RETIRE A RUNNING RELEASE. `skipWaiting` is reachable
// from here and nowhere else: not from `install`, not from `activate`. That
// keeps the original invariant intact for every path the browser takes on its
// own — a new release still waits behind an open tab forever — and narrows the
// exception to one an author asked for, in a tab that has already established
// its document is safe to lose.
export function isActivateRequest(value) {
  return Boolean(value) && typeof value === 'object' && value.version === 1 && value.type === 'activate-pending-release' && Object.keys(value).length === 2
}

export function serviceWorkerSource(release) {
  const encoded = JSON.stringify(release)
  return `/* Generated; do not edit. The release is a closed static allowlist. */
const RELEASE = ${encoded}
const CACHE_NAME = 'folio8-release-' + RELEASE.id
const MARKER = '/__folio8-release__/' + RELEASE.id
const STATIC_PATHS = new Set(RELEASE.assets.map((asset) => asset.url))
// THE BLOCKING SET AND THE ON-DEMAND SET, DERIVED FROM ONE AUTHORITY
// (spec-deferred-offline-cache, story 2). \`tier\` is stamped into every asset by
// scripts/offline-release-contract.mjs at build time and travels inside this
// embedded RELEASE record, so the worker classifies nothing and guesses nothing.
//
// CORE_ASSETS IS WHAT INSTALL WAITS FOR AND WHAT \`ready\` MEANS. DEFERRED_ASSETS
// is a lookup, not a list to walk: nothing iterates it, because nothing
// prefetches it. Its only reader is \`serveFromRelease\` below, which consults it
// on a cache MISS for a path an author's own action has just requested.
const CORE_ASSETS = RELEASE.assets.filter((asset) => asset.tier === 'core')
const DEFERRED_ASSETS = new Map(RELEASE.assets.filter((asset) => asset.tier === 'deferred').map((asset) => [asset.url, asset]))
const MESSAGE_VERSION = ${messageVersion}
const cacheableRequest = ${isCacheableStaticRequest.toString()}
const documentNavigation = ${isCacheableDocumentNavigation.toString()}
const statusRequest = ${isStatusRequest.toString()}
const versionRequest = ${isVersionRequest.toString()}
const activateRequest = ${isActivateRequest.toString()}

async function progress(state, asset) {
  await notify({ type: 'offline-progress', state, assetUrl: asset?.url ?? null })
}

// THE ONLY NETWORK READ IN THIS WORKER, AND IT IS ADDRESSED BY MANIFEST ENTRY
// RATHER THAN BY REQUEST. It takes an \`asset\` out of this release's own
// embedded record — never \`event.request\`, never a URL off the wire — fetches
// THAT url, and refuses to return bytes whose digest is not the one the release
// recorded. Both callers, the install precache and the on-demand deferred path,
// go through it, so there is exactly one place where bytes can enter this
// origin's cache and exactly one digest comparison guarding it.
//
// verify-offline-release.mjs counts the network reads in the emitted worker and
// requires there to be exactly one, here, immediately followed by this digest
// check. That is how the old blanket ban on two banned spellings is
// re-expressed: not "those two spellings are absent" but "every network read in
// this worker is this one, and it verifies". The banned spellings therefore may
// not appear even in a comment, which is why this note does not name them.
async function fetchVerified(asset) {
  const response = await fetch(asset.url, { cache: 'reload', credentials: 'omit' })
  if (!response.ok || response.type === 'opaque') throw new Error('offline asset missing')
  const bytes = await response.clone().arrayBuffer()
  const digest = await crypto.subtle.digest('SHA-256', bytes)
  const actual = Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('')
  if (actual !== asset.sha256) throw new Error('offline asset integrity mismatch')
  return response
}

// THE PRECACHE IS THE CORE TIER AND NOTHING ELSE. Install used to walk
// RELEASE.assets, so a first-time visitor waited for the CJK font, the 31
// catalogue faces, the bundled examples and the documentation before the
// designer would start. What it waits for now is what it cannot be used
// without; everything else arrives at the moment something asks for it.
async function completeCache() {
  const cache = await caches.open(CACHE_NAME)
  let activeAsset = null
  try { for (const [index, asset] of CORE_ASSETS.entries()) {
    activeAsset = asset
    await progress('active', asset)
    const response = await fetchVerified(asset)
    await cache.put(asset.url, response)
    // Delay the final verified notification until the marker exists. That makes
    // a 100% page readout proof of a complete core tier, not just of the last put.
    if (index < CORE_ASSETS.length - 1) await progress('verified', asset)
    }
    await cache.put(MARKER, new Response(RELEASE.id, { headers: { 'content-type': 'text/plain' } }))
    await progress('verified', activeAsset)
  } catch (error) { error.asset = activeAsset; throw error }
}

// COMPLETE MEANS THE CORE TIER IS HELD, AND IT MUST NOT MEAN ANY MORE THAN THAT.
// This answers the page's status request and gates activation, so a deferred
// asset counted here would make \`ready\` — and with it \`cacheReady\` and
// \`engineMayStart\` — false on a browser that is perfectly usable, and would
// come back false again the moment a cache eviction took a catalogue face.
async function hasCompleteCache() {
  const cache = await caches.open(CACHE_NAME)
  const marker = await cache.match(MARKER)
  if (!marker || await marker.text() !== RELEASE.id) return false
  return (await Promise.all(CORE_ASSETS.map((asset) => cache.match(asset.url)))).every(Boolean)
}

// ONE ANSWER FOR EVERY ALLOWED PATH, held or deferred (story 2).
//
// A HELD RESPONSE ALWAYS WINS, whatever tier it is: the second demand for a
// deferred face transfers nothing and works with the network down, which is the
// whole point of keeping it.
//
// A MISS IS SERVED FROM THE NETWORK ONLY WHEN THE PATH IS A DEFERRED ENTRY OF
// THIS RELEASE. That is the restriction the generic-fallback ban was always
// about: the lookup is into this worker's own embedded manifest, so a path that
// is not in it — including a core asset the cache somehow lost, and anything not
// in the release at all — is \`Response.error()\` exactly as before. A deferred
// fetch therefore cannot resolve against another release: the URL comes out of
// THIS RELEASE's record, and \`fetchVerified\` holds it to THAT entry's digest.
//
// FAILURE IS PER ASSET. An offline or failing deferred fetch errors this one
// response and touches neither the marker nor the core cache, so the release
// stays ready and the designer stays usable.
// ONE DEMAND PER ASSET AT A TIME. The CSS \`@font-face\` rule, the specimen read
// and an embed can each ask for the same face within a few milliseconds of one
// another, and without this every one of them would fetch, hash and cache-put
// the same bytes — 4.72 MiB apiece on the CJK face. Keyed by pathname and
// deleted on settle, so a failed demand is retried rather than remembered.
const inFlight = new Map()

async function serveFromRelease(pathname) {
  const cache = await caches.open(CACHE_NAME)
  const held = await cache.match(pathname)
  if (held) return held
  const deferred = DEFERRED_ASSETS.get(pathname)
  if (!deferred) return Response.error()
  let demand = inFlight.get(pathname)
  if (!demand) {
    demand = fetchVerifiedAndKeep(deferred).finally(() => inFlight.delete(pathname))
    inFlight.set(pathname, demand)
  }
  let response
  try { response = await demand } catch { return Response.error() }
  // EVERY CONSUMER GETS A CLONE AND THE SHARED RESPONSE IS NEVER READ. A body
  // can be consumed once, so handing the shared object to the first caller
  // would make the second caller's clone throw on a disturbed stream — the
  // failure the in-flight map would otherwise introduce.
  return response.clone()
}

// THE FETCH AND THE KEEP, AS ONE SHARED ACT — so concurrent demands write the
// cache once rather than once each.
//
// THE KEEP MAY FAIL ON ITS OWN AND THE RESPONSE STILL STANDS. A
// QuotaExceededError here would otherwise turn bytes that had already verified
// into a refusal: the author loses a face they had in hand because the browser
// had no room to KEEP it. Keeping is the optimisation; serving is the job, and
// the next demand tries the write again.
async function fetchVerifiedAndKeep(asset) {
  const response = await fetchVerified(asset)
  const keep = response.clone()
  try { await (await caches.open(CACHE_NAME)).put(asset.url, keep) } catch { /* not kept; still served */ }
  return response
}

async function notify(detail, target) {
  const message = { version: MESSAGE_VERSION, releaseId: RELEASE.id, pageId: RELEASE.pageId, ...detail }
  if (target && 'postMessage' in target) { target.postMessage(message); return }
  for (const client of await clients.matchAll({ type: 'window', includeUncontrolled: true })) client.postMessage(message)
}

self.addEventListener('install', (event) => {
  event.waitUntil(completeCache().catch(async (error) => { await progress('failed', error.asset); await caches.delete(CACHE_NAME); throw error }))
})

self.addEventListener('activate', (event) => {
  event.waitUntil((async () => {
    if (!await hasCompleteCache()) throw new Error('incomplete release cannot activate')
    // A waiting worker never takes existing tabs over. Retire an old cache only
    // when activation finds no window that could still rely on it. folio-release-
    // is the cache prefix releases used before the product was renamed folio8.
    const windows = await clients.matchAll({ type: 'window', includeUncontrolled: true })
    if (windows.length === 0) await Promise.all((await caches.keys()).filter((name) => /^folio8?-release-/.test(name) && name !== CACHE_NAME).map((name) => caches.delete(name)))
    await clients.claim()
    await notify({ type: 'offline-status', state: 'ready' })
  })())
})

self.addEventListener('fetch', (event) => {
  const request = event.request
  const url = new URL(request.url)
  if (request.mode === 'navigate' && url.origin === self.location.origin && (url.pathname === '/' || url.pathname === '/index.html')) {
    event.respondWith(serveFromRelease('/index.html'))
    return
  }
  // The bundled documentation pages are deferred, so opening one in its own tab
  // is a demand like any other and goes through the same door.
  if (documentNavigation(request, self.location.origin, STATIC_PATHS)) {
    event.respondWith(serveFromRelease(url.pathname))
    return
  }
  if (!cacheableRequest(request, self.location.origin, STATIC_PATHS)) return
  event.respondWith(serveFromRelease(url.pathname))
})

self.addEventListener('message', (event) => {
  if (versionRequest(event.data)) { event.ports[0] && event.ports[0].postMessage({ version: MESSAGE_VERSION, type: 'release-version', appVersion: RELEASE.appVersion, releaseId: RELEASE.id }); return }
  if (activateRequest(event.data)) { self.skipWaiting(); return }
  if (!statusRequest(event.data)) return
  event.waitUntil(hasCompleteCache().then((complete) => notify({ type: 'offline-status', state: complete ? 'ready' : 'unavailable' }, event.source)))
})
`
}
