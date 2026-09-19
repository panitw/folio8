import { describe, expect, it } from 'vitest'
import vm from 'node:vm'
import { isActivateRequest, isCacheableDocumentNavigation, isCacheableStaticRequest, isStatusRequest, isVersionRequest, serviceWorkerSource } from './offline-service-worker-template.mjs'

const release = (id = 'a'.repeat(64), workerRevision = 'b'.repeat(64)) => ({ version: 2, id, pageId: 'c'.repeat(64), workerRevision, assets: [{ url: '/index.html', sha256: 'd'.repeat(64), immutable: false, tier: 'core' }, { url: '/assets/app-abc12345.js', sha256: 'e'.repeat(64), immutable: true, tier: 'core' }] })
const request = (overrides = {}) => ({ url: 'https://folio8.test/assets/app-abc12345.js', method: 'GET', credentials: 'omit', mode: 'cors', ...overrides })

function workerHarness(workerRelease, options = {}) {
  const handlers = {}
  const cacheData = options.cacheData ?? new Map()
  const deleted = []
  const caches = {
    open: async (name) => {
      if (!cacheData.has(name)) cacheData.set(name, new Map())
      const store = cacheData.get(name)
      return { match: async (key) => store.get(key), put: async (key, value) => store.set(key, value) }
    },
    delete: async (name) => { deleted.push(name); return cacheData.delete(name) },
    keys: async () => [...cacheData.keys()],
  }
  const clients = { claim: async () => {}, matchAll: async () => options.windows ?? [] }
  const skipWaitingCalls = []
  const self = { location: { origin: 'https://folio8.test' }, addEventListener: (name, handler) => { handlers[name] = handler }, skipWaiting: () => { skipWaitingCalls.push(true) } }
  vm.runInNewContext(serviceWorkerSource(workerRelease), { self, caches, clients, fetch: options.fetch ?? (async () => { throw new Error('network unavailable') }), crypto: globalThis.crypto, Response, URL, Set, Map, Promise, Uint8Array })
  return { handlers, cacheData, deleted, skipWaitingCalls }
}

describe('service worker static policy', () => {
  it('allows only credential-omitting manifest static requests', () => {
    const paths = new Set(['/assets/app-abc12345.js'])
    expect(isCacheableStaticRequest(request(), 'https://folio8.test', paths)).toBe(true)
    expect(isCacheableStaticRequest(request({ credentials: 'same-origin' }), 'https://folio8.test', paths)).toBe(true)
    expect(isCacheableStaticRequest(request({ credentials: 'include' }), 'https://folio8.test', paths)).toBe(true)
    expect(isCacheableStaticRequest(request({ url: 'https://evil.test/assets/app-abc12345.js' }), 'https://folio8.test', paths)).toBe(false)
    expect(isCacheableStaticRequest(request({ url: 'https://folio8.test/documents/customer.folio' }), 'https://folio8.test', paths)).toBe(false)
    expect(isCacheableStaticRequest(request({ method: 'POST' }), 'https://folio8.test', paths)).toBe(false)
  })

  it('admits a navigation only when it is a same-origin GET to a precached /assets/*.html entry', () => {
    const guide = '/assets/rendering-library-0123456789abcdef0123.html'
    const paths = new Set(['/index.html', guide, '/assets/app-abc12345.js'])
    const navigate = (overrides = {}) => request({ url: `https://folio8.test${guide}`, mode: 'navigate', ...overrides })
    expect(isCacheableDocumentNavigation(navigate(), 'https://folio8.test', paths)).toBe(true)
    expect(isCacheableDocumentNavigation(navigate({ url: `https://folio8.test${guide}?from=designer` }), 'https://folio8.test', paths)).toBe(true)
    // The static policy is unchanged: it still refuses every navigation.
    expect(isCacheableStaticRequest(navigate(), 'https://folio8.test', paths)).toBe(false)
    expect(isCacheableDocumentNavigation(navigate({ mode: 'cors' }), 'https://folio8.test', paths)).toBe(false)
    expect(isCacheableDocumentNavigation(navigate({ url: `https://evil.test${guide}` }), 'https://folio8.test', paths)).toBe(false)
    expect(isCacheableDocumentNavigation(navigate({ method: 'POST' }), 'https://folio8.test', paths)).toBe(false)
    expect(isCacheableDocumentNavigation(navigate({ url: 'https://folio8.test/assets/folio-format-0123456789abcdef0123.html' }), 'https://folio8.test', paths)).toBe(false)
    expect(isCacheableDocumentNavigation(navigate({ url: 'https://folio8.test/assets/app-abc12345.js' }), 'https://folio8.test', paths)).toBe(false)
    expect(isCacheableDocumentNavigation(navigate({ url: 'https://folio8.test/index.html' }), 'https://folio8.test', paths)).toBe(false)
  })

  it('serves a navigation to a precached documentation page from the release cache, and leaves other navigations alone', async () => {
    const guide = '/assets/rendering-library-0123456789abcdef0123.html'
    const base = release()
    const workerRelease = { ...base, assets: [...base.assets, { url: guide, sha256: 'f'.repeat(64), immutable: true, tier: 'deferred' }] }
    const cached = { body: 'guide' }
    const index = { body: 'index' }
    const cacheData = new Map([[`folio8-release-${workerRelease.id}`, new Map([[guide, cached], ['/index.html', index]])]])
    const harness = workerHarness(workerRelease, { cacheData })
    const dispatch = async (overrides) => {
      let responded
      harness.handlers.fetch({ request: request(overrides), respondWith: (promise) => { responded = promise } })
      return responded === undefined ? undefined : await responded
    }
    expect(await dispatch({ url: `https://folio8.test${guide}`, mode: 'navigate' })).toBe(cached)
    expect(await dispatch({ url: 'https://folio8.test/', mode: 'navigate' })).toBe(index)
    expect(await dispatch({ url: 'https://folio8.test/assets/folio-format-0123456789abcdef0123.html', mode: 'navigate' })).toBeUndefined()
    expect(await dispatch({ url: 'https://folio8.test/assets/app-abc12345.js', mode: 'navigate' })).toBeUndefined()
    expect(await dispatch({ url: `https://elsewhere.test${guide}`, mode: 'navigate' })).toBeUndefined()
  })

  it('executes the emitted status handler with no module closure', async () => {
    const workerRelease = release()
    const cacheName = `folio8-release-${workerRelease.id}`
    const marker = `/__folio8-release__/${workerRelease.id}`
    const cacheData = new Map([[cacheName, new Map([[marker, { text: async () => workerRelease.id }], ...workerRelease.assets.map((asset) => [asset.url, {}])])]])
    const harness = workerHarness(workerRelease, { cacheData })
    const messages = []
    let completion
    harness.handlers.message({ data: { version: 1, type: 'get-offline-status' }, source: { postMessage: (message) => messages.push(message) }, waitUntil: (promise) => { completion = promise } })
    await completion
    expect(messages).toEqual([{ version: 1, type: 'offline-status', state: 'ready', releaseId: workerRelease.id, pageId: workerRelease.pageId }])
    expect(isStatusRequest({ version: 1, type: 'get-offline-status' })).toBe(true)
  })

  it('keeps an old complete cache when a same-assets changed-worker install fails', async () => {
    const old = release('a'.repeat(64), 'b'.repeat(64))
    const next = release('f'.repeat(64), 'g'.repeat(64))
    const oldCache = `folio8-release-${old.id}`
    const cacheData = new Map([[oldCache, new Map([['/old', {}]])]])
    const harness = workerHarness(next, { cacheData })
    let completion
    harness.handlers.install({ waitUntil: (promise) => { completion = promise } })
    await expect(completion).rejects.toThrow('network unavailable')
    expect(cacheData.has(oldCache)).toBe(true)
    expect(harness.deleted).toEqual([`folio8-release-${next.id}`])
  })

  it('does not retire an old cache while a window can still rely on it, then retires it with no clients', async () => {
    const current = release()
    const currentCache = `folio8-release-${current.id}`
    const marker = `/__folio8-release__/${current.id}`
    const complete = new Map([[marker, { text: async () => current.id }], ...current.assets.map((asset) => [asset.url, {}])])
    const oldCache = 'folio8-release-old-complete'
    const cacheData = new Map([[currentCache, complete], [oldCache, new Map()]])
    const retained = workerHarness(current, { cacheData, windows: [{ postMessage: () => {} }] })
    let completion
    retained.handlers.activate({ waitUntil: (promise) => { completion = promise } })
    await completion
    expect(cacheData.has(oldCache)).toBe(true)
    const retired = workerHarness(current, { cacheData, windows: [] })
    retired.handlers.activate({ waitUntil: (promise) => { completion = promise } })
    await completion
    expect(cacheData.has(oldCache)).toBe(false)
  })
})

// THE TIERED PRECACHE, EXECUTED (spec-deferred-offline-cache, story 2).
//
// Every other fixture in this file is all-`core`, so a worker that widened its
// precache back to `RELEASE.assets` would pass all of them. These two cases are
// the ones that can tell the difference: a release with a real deferred asset in
// it, run through the emitted install and the emitted status answer.
describe('the precache and the readiness answer are the core tier', () => {
  const digest = async (bytes) => Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), (byte) => byte.toString(16).padStart(2, '0')).join('')

  // Bodies are real, so the emitted digest check is exercised rather than
  // bypassed: a precache that fetched the right URLs and skipped verification
  // would not survive this fixture.
  const tieredRelease = async () => {
    const parts = [['/index.html', 'core'], ['/assets/app-abc12345.js', 'core'], ['/assets/noto-sans-cjk.ttf', 'deferred'], ['/assets/catalogue-lora.ttf', 'deferred']]
    const bodies = new Map()
    const assets = []
    for (const [url, tier] of parts) {
      const bytes = new TextEncoder().encode(`bytes for ${url}`)
      bodies.set(url, bytes)
      assets.push({ url, sha256: await digest(bytes), immutable: url !== '/index.html', tier })
    }
    return { release: { version: 3, id: 'a'.repeat(64), pageId: 'c'.repeat(64), workerRevision: 'b'.repeat(64), assets }, bodies }
  }

  it('fetches every core asset at install and asks for no deferred asset at all', async () => {
    const { release: tiered, bodies } = await tieredRelease()
    const asked = []
    const harness = workerHarness(tiered, { fetch: async (url) => { asked.push(url); return new Response(bodies.get(url)) } })
    let completion
    harness.handlers.install({ waitUntil: (promise) => { completion = promise } })
    await completion

    const core = tiered.assets.filter((asset) => asset.tier === 'core').map((asset) => asset.url)
    const deferred = tiered.assets.filter((asset) => asset.tier === 'deferred').map((asset) => asset.url)
    // NON-VACUITY: both tiers must have members, or "only core" is a claim about
    // a release with one kind of asset in it.
    expect(core.length).toBeGreaterThan(0)
    expect(deferred.length).toBeGreaterThan(0)
    expect([...asked].sort()).toEqual([...core].sort())
    // AND THE CACHE HOLDS THE SAME SET, plus the completion marker. A deferred
    // asset written here would be 7.96 MiB a first-time visitor waited for.
    const stored = [...harness.cacheData.get(`folio8-release-${tiered.id}`).keys()]
    expect(stored.sort()).toEqual([...core, `/__folio8-release__/${tiered.id}`].sort())
  })

  it('answers ready with the whole deferred tier absent from the cache', async () => {
    const { release: tiered } = await tieredRelease()
    const marker = `/__folio8-release__/${tiered.id}`
    // Exactly the core tier and the marker: the state install leaves behind.
    const held = new Map([[marker, { text: async () => tiered.id }], ...tiered.assets.filter((asset) => asset.tier === 'core').map((asset) => [asset.url, {}])])
    for (const asset of tiered.assets) {
      if (asset.tier === 'deferred') expect(held.has(asset.url), 'this case only measures anything while the deferred tier really is absent').toBe(false)
    }
    const harness = workerHarness(tiered, { cacheData: new Map([[`folio8-release-${tiered.id}`, held]]) })
    const messages = []
    let completion
    harness.handlers.message({ data: { version: 1, type: 'get-offline-status' }, source: { postMessage: (message) => messages.push(message) }, waitUntil: (promise) => { completion = promise } })
    await completion
    expect(messages).toEqual([{ version: 1, type: 'offline-status', state: 'ready', releaseId: tiered.id, pageId: tiered.pageId }])
  })
})

describe('the two requests a waiting worker answers', () => {
  const versioned = { ...release(), appVersion: '2.0.0' }

  it('accepts only its own exact request shapes', () => {
    expect(isVersionRequest({ version: 1, type: 'get-release-version' })).toBe(true)
    expect(isActivateRequest({ version: 1, type: 'activate-pending-release' })).toBe(true)
    // The bounded shape is the point: an extra key is a different message.
    expect(isVersionRequest({ version: 1, type: 'get-release-version', force: true })).toBe(false)
    expect(isActivateRequest({ version: 2, type: 'activate-pending-release' })).toBe(false)
    expect(isActivateRequest({ version: 1, type: 'get-release-version' })).toBe(false)
    expect(isVersionRequest(null)).toBe(false)
    expect(isActivateRequest('activate-pending-release')).toBe(false)
  })

  it('answers its version on the caller\'s port and tells no one else', () => {
    const { handlers } = workerHarness(versioned)
    const posted = []
    handlers.message({ data: { version: 1, type: 'get-release-version' }, ports: [{ postMessage: (message) => posted.push(message) }] })
    expect(posted).toEqual([{ version: 1, type: 'release-version', appVersion: '2.0.0', releaseId: versioned.id }])
  })

  it('steps forward ONLY for the activation request, never for a status or version read', () => {
    const { handlers, skipWaitingCalls } = workerHarness(versioned)
    handlers.message({ data: { version: 1, type: 'get-release-version' }, ports: [{ postMessage: () => {} }] })
    handlers.message({ data: { version: 1, type: 'get-offline-status' }, source: { postMessage: () => {} }, waitUntil: () => {} })
    expect(skipWaitingCalls).toHaveLength(0)
    handlers.message({ data: { version: 1, type: 'activate-pending-release' } })
    expect(skipWaitingCalls).toHaveLength(1)
  })

  // THE INVARIANT THAT SURVIVED THE CHANGE. `skipWaiting` became reachable, but
  // only from a message an author's own tab sends. Nothing the BROWSER does on
  // its own — installing, activating — may retire a release under an open
  // document, so neither lifecycle handler may reach it.
  it('never steps forward from install or activate', () => {
    const source = serviceWorkerSource(versioned)
    for (const handler of ['install', 'activate']) {
      const start = source.indexOf(`self.addEventListener('${handler}'`)
      const end = source.indexOf("self.addEventListener('", start + 1)
      expect(source.slice(start, end < 0 ? undefined : end)).not.toContain('skipWaiting')
    }
    expect(source.split('skipWaiting').length - 1).toBe(1)
  })
})
