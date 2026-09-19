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
const MESSAGE_VERSION = ${messageVersion}
const cacheableRequest = ${isCacheableStaticRequest.toString()}
const documentNavigation = ${isCacheableDocumentNavigation.toString()}
const statusRequest = ${isStatusRequest.toString()}
const versionRequest = ${isVersionRequest.toString()}
const activateRequest = ${isActivateRequest.toString()}

async function progress(state, asset) {
  await notify({ type: 'offline-progress', state, assetUrl: asset?.url ?? null })
}

async function completeCache() {
  const cache = await caches.open(CACHE_NAME)
  let activeAsset = null
  try { for (const [index, asset] of RELEASE.assets.entries()) {
    activeAsset = asset
    await progress('active', asset)
    const response = await fetch(asset.url, { cache: 'reload', credentials: 'omit' })
    if (!response.ok || response.type === 'opaque') throw new Error('offline asset missing')
    const bytes = await response.clone().arrayBuffer()
    const digest = await crypto.subtle.digest('SHA-256', bytes)
    const actual = Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('')
    if (actual !== asset.sha256) throw new Error('offline asset integrity mismatch')
    await cache.put(asset.url, response)
    // Delay the final verified notification until the marker exists. That makes
    // a 100% page readout proof of a complete release, not just of the last put.
    if (index < RELEASE.assets.length - 1) await progress('verified', asset)
    }
    await cache.put(MARKER, new Response(RELEASE.id, { headers: { 'content-type': 'text/plain' } }))
    await progress('verified', activeAsset)
  } catch (error) { error.asset = activeAsset; throw error }
}

async function hasCompleteCache() {
  const cache = await caches.open(CACHE_NAME)
  const marker = await cache.match(MARKER)
  if (!marker || await marker.text() !== RELEASE.id) return false
  return (await Promise.all(RELEASE.assets.map((asset) => cache.match(asset.url)))).every(Boolean)
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
    event.respondWith(caches.open(CACHE_NAME).then((cache) => cache.match('/index.html')).then((response) => response || Response.error()))
    return
  }
  if (documentNavigation(request, self.location.origin, STATIC_PATHS)) {
    event.respondWith(caches.open(CACHE_NAME).then((cache) => cache.match(url.pathname)).then((response) => response || Response.error()))
    return
  }
  if (!cacheableRequest(request, self.location.origin, STATIC_PATHS)) return
  event.respondWith(caches.open(CACHE_NAME).then((cache) => cache.match(url.pathname)).then((response) => response || Response.error()))
})

self.addEventListener('message', (event) => {
  if (versionRequest(event.data)) { event.ports[0] && event.ports[0].postMessage({ version: MESSAGE_VERSION, type: 'release-version', appVersion: RELEASE.appVersion, releaseId: RELEASE.id }); return }
  if (activateRequest(event.data)) { self.skipWaiting(); return }
  if (!statusRequest(event.data)) return
  event.waitUntil(hasCompleteCache().then((complete) => notify({ type: 'offline-status', state: complete ? 'ready' : 'unavailable' }, event.source)))
})
`
}
