import { describe, expect, it, vi } from 'vitest'
import { activatePendingRelease, askWaitingVersion, engineMayStart, parseWorkerProgress, parseWorkerStatus, reduceOfflineLifecycle, registerOfflineLifecycle, upgradeIsMandatory } from './offline-lifecycle'
import type { S1Payload } from './release-payload'

const release = 'a'.repeat(64)
const page = 'b'.repeat(64)
const payload: S1Payload = { version: 1, releaseId: release, pageId: page, unit: 'MiB', decimals: 2, cachedBytes: 100, assetCount: 10, cacheAssets: ['/index.html', '/assets/engine.wasm', '/assets/latin.ttf', '/assets/thai.ttf', '/assets/cjk.ttf', '/assets/a.js', '/assets/b.js', '/assets/c.js', '/assets/d.js', '/assets/e.js'].map((assetUrl) => ({ assetUrl, bytes: 10 })), rows: [
  { id: 'engine', label: 'Engine', delivery: 'cached-asset', assetUrl: '/assets/engine.wasm', bytes: 10, sha256: release },
  { id: 'latin-font', label: 'Latin font', delivery: 'cached-asset', assetUrl: '/assets/latin.ttf', bytes: 10, sha256: release },
  { id: 'thai-font', label: 'Thai font', delivery: 'cached-asset', assetUrl: '/assets/thai.ttf', bytes: 10, sha256: release },
  { id: 'cjk-font', label: 'CJK font', delivery: 'cached-asset', assetUrl: '/assets/cjk.ttf', bytes: 10, sha256: release },
  { id: 'thai-dictionary', label: 'Thai dictionary', delivery: 'embedded-in-engine', assetUrl: '/assets/engine.wasm', bytes: 5, sha256: release },
] }

describe('offline lifecycle message boundary', () => {
  it('accepts only a bounded status for the page release that requested it', () => {
    expect(parseWorkerStatus({ version: 1, type: 'offline-status', state: 'ready', releaseId: release, pageId: page })).toEqual({ version: 1, type: 'offline-status', state: 'ready', releaseId: release, pageId: page })
    expect(parseWorkerStatus({ version: 1, type: 'offline-status', state: 'ready', releaseId: 'not-a-release', pageId: page, template: '{customer: secret}' })).toBeUndefined()
    expect(parseWorkerStatus({ version: 2, type: 'offline-status', state: 'ready', releaseId: release, pageId: page })).toBeUndefined()
    expect(parseWorkerStatus({ version: 1, type: 'cache-all', state: 'ready', releaseId: release, pageId: page })).toBeUndefined()
  })

  it('keeps a waiting update visible when the active worker reports ready and rejects a mismatched page release', async () => {
    const channel = new EventTarget() as EventTarget & { register: ReturnType<typeof vi.fn>; controller?: ServiceWorker }
    const installing = new EventTarget() as EventTarget & { state: ServiceWorkerState }
    installing.state = 'installed'
    const active = { postMessage: vi.fn() } as unknown as ServiceWorker
    const registration = new EventTarget() as ServiceWorkerRegistration
    Object.assign(registration, { active, waiting: null, installing })
    channel.register = vi.fn(async () => registration)
    channel.controller = active
    Object.defineProperty(navigator, 'serviceWorker', { configurable: true, value: channel })
    Object.defineProperty(window, 'isSecureContext', { configurable: true, value: true })
    const states: string[] = []
    registerOfflineLifecycle(page, payload, (state) => states.push(state.state))
    await Promise.resolve()
    expect(active.postMessage).toHaveBeenCalledWith({ version: 1, type: 'get-offline-status' })
    Object.assign(registration, { waiting: installing })
    registration.dispatchEvent(new Event('updatefound'))
    installing.dispatchEvent(new Event('statechange'))
    channel.dispatchEvent(new MessageEvent('message', { data: { version: 1, type: 'offline-status', state: 'ready', releaseId: release, pageId: page } }))
    expect(states).toEqual(['checking', 'update-available', 'update-available'])
    channel.dispatchEvent(new MessageEvent('message', { data: { version: 1, type: 'offline-status', state: 'ready', releaseId: release, pageId: 'c'.repeat(64) } }))
    expect(states).toEqual(['checking', 'update-available', 'update-available'])
  })

  it('counts only verified, payload-bound asset events and never treats engine readiness as cache completion', () => {
    const initial = { state: 'checking' as const, cacheReady: false, verifiedAssetUrls: [] }
    const verified = parseWorkerProgress({ version: 1, type: 'offline-progress', state: 'verified', assetUrl: '/assets/engine.wasm', releaseId: release, pageId: page })!
    expect(reduceOfflineLifecycle(initial, verified, payload)).toMatchObject({ state: 'caching', verifiedAssetUrls: ['/assets/engine.wasm'], cacheReady: false })
    const malformed = parseWorkerProgress({ version: 1, type: 'offline-progress', state: 'verified', assetUrl: '/assets/engine.wasm', releaseId: release, pageId: page, document: 'secret' })
    expect(malformed).toBeUndefined()
    const unknown = parseWorkerProgress({ version: 1, type: 'offline-progress', state: 'verified', assetUrl: '/assets/not-in-release.js', releaseId: release, pageId: page })!
    expect(reduceOfflineLifecycle(initial, unknown, payload)).toEqual(initial)
    const mismatchedRelease = parseWorkerProgress({ version: 1, type: 'offline-progress', state: 'verified', assetUrl: '/assets/engine.wasm', releaseId: 'c'.repeat(64), pageId: page })!
    expect(reduceOfflineLifecycle(initial, mismatchedRelease, payload)).toEqual(initial)
    expect(reduceOfflineLifecycle(initial, { version: 1, type: 'offline-status', state: 'unavailable', releaseId: release, pageId: page }, payload)).toMatchObject({ state: 'unavailable', cacheReady: false })
  })

  it('removes every live observer callback and converts a stalled status exchange into retryable timeout', async () => {
    vi.useFakeTimers()
    const channel = new EventTarget() as EventTarget & { register: ReturnType<typeof vi.fn>; controller?: ServiceWorker }
    const active = { postMessage: vi.fn() } as unknown as ServiceWorker
    const registration = new EventTarget() as ServiceWorkerRegistration
    Object.assign(registration, { active, waiting: null, installing: null })
    channel.register = vi.fn(async () => registration)
    Object.defineProperty(navigator, 'serviceWorker', { configurable: true, value: channel })
    Object.defineProperty(window, 'isSecureContext', { configurable: true, value: true })
    const states: string[] = []
    const stop = registerOfflineLifecycle(page, payload, (state) => states.push(state.state), 10)
    await Promise.resolve()
    vi.advanceTimersByTime(10)
    expect(states).toContain('unavailable')
    const before = states.length
    stop()
    channel.dispatchEvent(new MessageEvent('message', { data: { version: 1, type: 'offline-status', state: 'ready', releaseId: release, pageId: page } }))
    expect(states).toHaveLength(before)
    vi.useRealTimers()
  })

  it('measures cache inactivity and never times out after the worker reports ready', async () => {
    vi.useFakeTimers()
    const channel = new EventTarget() as EventTarget & { register: ReturnType<typeof vi.fn>; controller?: ServiceWorker }
    const active = { postMessage: vi.fn() } as unknown as ServiceWorker
    const registration = new EventTarget() as ServiceWorkerRegistration
    Object.assign(registration, { active, waiting: null, installing: null })
    channel.register = vi.fn(async () => registration)
    Object.defineProperty(navigator, 'serviceWorker', { configurable: true, value: channel })
    Object.defineProperty(window, 'isSecureContext', { configurable: true, value: true })
    const states: string[] = []
    registerOfflineLifecycle(page, payload, (state) => states.push(state.state), 10)
    await Promise.resolve()
    vi.advanceTimersByTime(9)
    channel.dispatchEvent(new MessageEvent('message', { data: { version: 1, type: 'offline-progress', state: 'verified', assetUrl: '/assets/engine.wasm', releaseId: release, pageId: page } }))
    vi.advanceTimersByTime(9)
    expect(states).not.toContain('unavailable')
    channel.dispatchEvent(new MessageEvent('message', { data: { version: 1, type: 'offline-status', state: 'ready', releaseId: release, pageId: page } }))
    vi.advanceTimersByTime(20)
    expect(states.at(-1)).toBe('ready')
    vi.useRealTimers()
  })

  it('starts the engine on a verified cache or the development bypass, and on nothing else', () => {
    expect(engineMayStart({ state: 'ready', cacheReady: true, verifiedAssetUrls: [] })).toBe(true)
    expect(engineMayStart({ state: 'dev-bypass', cacheReady: false, verifiedAssetUrls: [] })).toBe(true)
    for (const state of ['checking', 'caching', 'unavailable'] as const) expect(engineMayStart({ state, cacheReady: false, verifiedAssetUrls: [] })).toBe(false)
  })
})

describe('the mandatory-upgrade rule', () => {
  // THE TABLE IS THE SPEC, and it is run twice: once against the runtime rule in
  // this module and once against its build-time twin in the contract, which the
  // browser cannot import. A change to either that the other does not follow reds
  // here rather than shipping a release that two halves of the system disagree
  // about — one telling a tab the update is required and the other that it is not.
  const table: ReadonlyArray<readonly [string | undefined, string | undefined, boolean]> = [
    ['1.0.0', '1.0.1', false],
    ['1.0.0', '1.9.9', false],
    ['1.9.9', '2.0.0', true],
    ['1.0.0', '3.0.0', true],
    ['2.0.0', '1.0.0', false],
    ['2.0.0', '2.0.0', false],
    // MISSING EVIDENCE IS NEVER MANDATORY. A page built before versioning, or a
    // worker that never answered, must not lock an author out of a document.
    [undefined, '2.0.0', false],
    ['1.0.0', undefined, false],
    [undefined, undefined, false],
    // Not a version at all reads as missing, not as zero.
    ['1.0', '2.0.0', false],
    ['1.0.0', 'v2.0.0', false],
    ['1.0.0', '02.0.0', false],
  ]

  it.each(table)('treats %s → %s as mandatory=%s', (from, to, expected) => {
    expect(upgradeIsMandatory(from, to)).toBe(expected)
  })

  it('agrees exactly with the build-time rule the release is stamped by', async () => {
    // @ts-expect-error The build-time contract is plain Node ESM with no types;
    // it is imported here precisely BECAUSE the browser bundle cannot import it,
    // which is the drift this test exists to catch.
    const contract = await import('../scripts/offline-release-contract.mjs') as { upgradeIsMandatory: (a?: string, b?: string) => boolean }
    for (const [from, to, expected] of table) {
      expect(contract.upgradeIsMandatory(from, to), `build-time rule disagreed on ${from} → ${to}`).toBe(expected)
      expect(upgradeIsMandatory(from, to)).toBe(contract.upgradeIsMandatory(from, to))
    }
  })

  it('carries a pending release into the state without ever retiring the running one', () => {
    const ready = { state: 'ready' as const, cacheReady: true, verifiedAssetUrls: ['/index.html'] }
    const next = reduceOfflineLifecycle(ready, { kind: 'pending-release', appVersion: '2.0.0', mandatory: true }, payload)
    expect(next).toMatchObject({ state: 'update-available', pendingVersion: '2.0.0', mandatory: true })
    // THE RUNNING RELEASE SURVIVES THE PROMPT. `cacheReady` is what lets the
    // engine keep running underneath an update the author has not taken yet.
    expect(next.cacheReady).toBe(true)
    expect(next.verifiedAssetUrls).toEqual(['/index.html'])
  })
})

describe('asking the waiting worker what it is', () => {
  const waitingWorker = (reply: unknown | undefined) => ({ postMessage: vi.fn((_message: unknown, transfer: Transferable[]) => { if (reply === undefined) return; const port = transfer[0] as MessagePort; port.postMessage(reply) }) }) as unknown as ServiceWorker

  it('reads the version off the private port, because the broadcast channel refuses another release', async () => {
    const worker = waitingWorker({ version: 1, type: 'release-version', appVersion: '2.1.0', releaseId: release })
    await expect(askWaitingVersion(worker)).resolves.toBe('2.1.0')
    expect((worker.postMessage as ReturnType<typeof vi.fn>).mock.calls[0][0]).toEqual({ version: 1, type: 'get-release-version' })
  })

  it('resolves undefined rather than hanging when the waiting worker never answers', async () => {
    vi.useFakeTimers()
    try {
      const pending = askWaitingVersion(waitingWorker(undefined), 50)
      await vi.advanceTimersByTimeAsync(60)
      await expect(pending).resolves.toBeUndefined()
    } finally { vi.useRealTimers() }
  })

  it('refuses a malformed or wrong-typed answer instead of treating it as a version', async () => {
    await expect(askWaitingVersion(waitingWorker({ version: 1, type: 'release-version', appVersion: 2 }))).resolves.toBeUndefined()
    await expect(askWaitingVersion(waitingWorker({ version: 2, type: 'release-version', appVersion: '2.0.0' }))).resolves.toBeUndefined()
    await expect(askWaitingVersion(waitingWorker({ version: 1, type: 'something-else', appVersion: '2.0.0' }))).resolves.toBeUndefined()
  })
})

describe('taking the pending release', () => {
  it('asks the waiting worker to step forward and reloads only once the swap actually happened', async () => {
    const waiting = { postMessage: vi.fn() } as unknown as ServiceWorker
    const channel = new EventTarget() as EventTarget & { getRegistration: ReturnType<typeof vi.fn> }
    channel.getRegistration = vi.fn(async () => ({ waiting }))
    Object.defineProperty(navigator, 'serviceWorker', { configurable: true, value: channel })
    const reload = vi.fn()
    expect(await activatePendingRelease(reload)).toBe(true)
    expect(waiting.postMessage).toHaveBeenCalledWith({ version: 1, type: 'activate-pending-release' })
    // NOT YET. The message only asks; until the controller actually changes the
    // tab would reload into the release it is already running.
    expect(reload).not.toHaveBeenCalled()
    channel.dispatchEvent(new Event('controllerchange'))
    expect(reload).toHaveBeenCalledTimes(1)
  })

  it('reports that nothing was taken when no release is waiting', async () => {
    const channel = new EventTarget() as EventTarget & { getRegistration: ReturnType<typeof vi.fn> }
    channel.getRegistration = vi.fn(async () => ({ waiting: null }))
    Object.defineProperty(navigator, 'serviceWorker', { configurable: true, value: channel })
    const reload = vi.fn()
    expect(await activatePendingRelease(reload)).toBe(false)
    expect(reload).not.toHaveBeenCalled()
  })
})
