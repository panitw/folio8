import { coreCacheAssets, type S1Payload } from './release-payload'

export type OfflineLifecycleState = 'checking' | 'caching' | 'ready' | 'update-available' | 'unavailable' | 'dev-bypass'
// `cacheReady` MEANS THE CORE TIER, AND MEANS NOTHING ABOUT THE DEFERRED ONE
// (spec-deferred-offline-cache, story 2). It used to assert that every asset in
// the release was cached and verified, because that is what the worker's
// `offline-status: ready` asserted. The worker now precaches the core tier and
// broadcasts `ready` over that; this flag is the page's copy of that claim and
// nothing more. It is still exactly what `engineMayStart` gates on — the engine
// carries its own embedded faces and needs no deferred asset to start — and it
// is deliberately NOT readable as "this browser holds the CJK font", "…the
// catalogue", or "…the bundled examples". Each of those is answered where it is
// asked, by the asset being there or being fetched.
export type OfflineLifecycle = Readonly<{ state: OfflineLifecycleState; cacheReady: boolean; verifiedAssetUrls: readonly string[]; activeAssetUrl?: string; failedAssetUrl?: string; failure?: 'timeout' | 'install' | 'unsupported'; pendingVersion?: string; mandatory?: boolean }>
// `vite dev` serves unbundled mutable modules, so no content-addressed release
// exists for the worker to verify. Development bypasses the offline gate
// visibly rather than simulating a verified cache: `cacheReady` keeps meaning
// "this browser holds a verified release", and `dev-bypass` is a separate,
// labelled reason to start the engine. Every reference is behind
// `import.meta.env.DEV`, so the branch and its strings are eliminated from
// production bundles; verify-offline-release.mjs proves their absence.
export const engineMayStart = (lifecycle: OfflineLifecycle) => lifecycle.cacheReady || (import.meta.env.DEV && lifecycle.state === 'dev-bypass')
const MESSAGE_VERSION = 1
const releaseIdPattern = /^[a-f0-9]{64}$/
const semverPattern = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/

// THE RUNTIME HALF OF THE MANDATORY RULE. Its build-time twin lives in
// scripts/offline-release-contract.mjs, which cannot be imported here because it
// reads the filesystem at module scope. The two are held together by a test that
// runs both over the same table rather than by hope; if you change the rule,
// change it there and let that test tell you about this one.
//
// CANNOT-TELL IS NEVER MANDATORY. A page built before versioning existed, or a
// waiting worker that declines to answer, leaves one side undefined — and the
// safe reading of missing evidence is the OPTIONAL prompt. The alternative locks
// an author out of a document on the strength of something we failed to learn.
export function upgradeIsMandatory(fromVersion: string | undefined, toVersion: string | undefined): boolean {
  const from = semverPattern.exec(fromVersion ?? '')
  const to = semverPattern.exec(toVersion ?? '')
  return Boolean(from && to) && Number(to![1]) > Number(from![1])
}

// What the page learned about the worker waiting behind it. `appVersion` is
// absent when the waiting worker never answered, which is not a failure: the
// update is still real and still offerable, just not provably mandatory.
export type PendingRelease = Readonly<{ kind: 'pending-release'; appVersion?: string; mandatory: boolean }>
type WorkerStatus = Readonly<{ version: number; type: 'offline-status'; state: 'ready' | 'unavailable'; releaseId: string; pageId: string }>
type WorkerProgress = Readonly<{ version: number; type: 'offline-progress'; state: 'active' | 'verified' | 'failed'; assetUrl: string | null; releaseId: string; pageId: string }>

function identity(candidate: Record<string, unknown>): boolean { return candidate.version === MESSAGE_VERSION && typeof candidate.releaseId === 'string' && releaseIdPattern.test(candidate.releaseId) && typeof candidate.pageId === 'string' && releaseIdPattern.test(candidate.pageId) }
export function parseWorkerStatus(value: unknown): WorkerStatus | undefined { if (!value || typeof value !== 'object') return undefined; const candidate = value as Record<string, unknown>; return Object.keys(candidate).length === 5 && identity(candidate) && candidate.type === 'offline-status' && (candidate.state === 'ready' || candidate.state === 'unavailable') ? candidate as WorkerStatus : undefined }
export function parseWorkerProgress(value: unknown): WorkerProgress | undefined { if (!value || typeof value !== 'object') return undefined; const candidate = value as Record<string, unknown>; return Object.keys(candidate).length === 6 && identity(candidate) && candidate.type === 'offline-progress' && (candidate.state === 'active' || candidate.state === 'verified' || candidate.state === 'failed') && (candidate.assetUrl === null || (typeof candidate.assetUrl === 'string' && candidate.assetUrl.startsWith('/') && candidate.assetUrl.length <= 256)) ? candidate as WorkerProgress : undefined }
const matches = (event: WorkerStatus | WorkerProgress, payload: S1Payload) => event.releaseId === payload.releaseId && event.pageId === payload.pageId
// A PROGRESS EVENT IS KNOWN ONLY IF IT IS ABOUT THE BLOCKING SET (story 2). The
// worker emits progress for core assets alone, so this narrowing changes no
// behaviour against a worker of this release — it states the invariant the
// screen depends on, so a deferred asset's URL can never enter
// `verifiedAssetUrls` and be counted into a denominator that does not contain
// it. An unknown URL stays the silent no-op it already was.
const known = (url: string, payload: S1Payload) => coreCacheAssets(payload).some((asset) => asset.assetUrl === url)

export function reduceOfflineLifecycle(previous: OfflineLifecycle, event: WorkerStatus | WorkerProgress | PendingRelease | 'checking' | 'update-available' | 'unavailable' | 'timeout', payload?: S1Payload): OfflineLifecycle {
  // A PENDING RELEASE NEVER CLEARS `cacheReady`. The release this tab is running
  // stays complete and usable whether the author takes the update or not, so the
  // engine keeps running underneath the prompt.
  if (typeof event === 'object' && 'kind' in event) return { ...previous, state: 'update-available', pendingVersion: event.appVersion, mandatory: event.mandatory }
  if (typeof event === 'string') {
    if (event === 'checking') return previous.cacheReady ? previous : { ...previous, state: 'checking' }
    if (event === 'update-available') return { ...previous, state: 'update-available' }
    return { ...previous, state: 'unavailable', cacheReady: false, failure: event === 'timeout' ? 'timeout' : previous.failure ?? 'install' }
  }
  if (!payload || !matches(event, payload)) return previous
  if (event.type === 'offline-status') return event.state === 'ready' ? { ...previous, state: previous.state === 'update-available' ? 'update-available' : 'ready', cacheReady: true, activeAssetUrl: undefined, failedAssetUrl: undefined, failure: undefined } : previous.cacheReady ? previous : { ...previous, state: 'unavailable', failure: 'install' }
  if (event.state === 'failed') return previous.cacheReady ? previous : { ...previous, state: 'unavailable', activeAssetUrl: undefined, failedAssetUrl: event.assetUrl ?? undefined, failure: 'install' }
  if (!event.assetUrl || !known(event.assetUrl, payload)) return previous
  if (event.state === 'active') return { ...previous, state: 'caching', activeAssetUrl: event.assetUrl }
  if (!previous.verifiedAssetUrls.includes(event.assetUrl)) return { ...previous, state: 'caching', activeAssetUrl: undefined, verifiedAssetUrls: [...previous.verifiedAssetUrls, event.assetUrl] }
  return previous
}

// ASKED OVER A PRIVATE PORT, because the broadcast channel is closed to it by
// design: `matches()` above drops every message whose releaseId is not this
// page's own, and a waiting worker is a different release by definition. The
// reply comes back on the port this page created, so no other tab is affected
// and no unmatched broadcast has to be trusted.
//
// A SILENT WORKER RESOLVES `undefined` RATHER THAN HANGING. The update is still
// real and still worth offering; only its mandatory-ness becomes unknowable, and
// `upgradeIsMandatory` reads unknown as optional.
export function askWaitingVersion(worker: ServiceWorker, timeoutMs = 3000): Promise<string | undefined> {
  return new Promise((resolve) => {
    const channel = new MessageChannel()
    let settled = false
    const finish = (value: string | undefined) => { if (settled) return; settled = true; clearTimeout(timer); try { channel.port1.close() } catch { /* a closed port is the outcome we wanted */ } resolve(value) }
    const timer = setTimeout(() => finish(undefined), timeoutMs)
    channel.port1.onmessage = (event: MessageEvent<unknown>) => {
      const data = event.data as Record<string, unknown> | null
      finish(Boolean(data) && typeof data === 'object' && data!.version === MESSAGE_VERSION && data!.type === 'release-version' && typeof data!.appVersion === 'string' ? data!.appVersion as string : undefined)
    }
    try { worker.postMessage({ version: MESSAGE_VERSION, type: 'get-release-version' }, [channel.port2]) } catch { finish(undefined) }
  })
}

// THE AUTHOR'S OWN HAND ON THE SWITCH. This is the only caller of the worker's
// only route to `skipWaiting`, and callers must have established the document is
// safe first — the dialog that calls it refuses while there are unsaved edits.
// `controllerchange` is what proves the swap happened; reloading on anything
// earlier would reload into the SAME release and read as a no-op to the author.
export async function activatePendingRelease(reload: () => void = () => window.location.reload()): Promise<boolean> {
  if (!('serviceWorker' in navigator)) return false
  const registration = await navigator.serviceWorker.getRegistration('/')
  const waiting = registration?.waiting
  if (!waiting) return false
  navigator.serviceWorker.addEventListener('controllerchange', () => reload(), { once: true })
  waiting.postMessage({ version: MESSAGE_VERSION, type: 'activate-pending-release' })
  return true
}

export function registerOfflineLifecycle(expectedPageId: string | undefined, payload: S1Payload | undefined, onState: (state: OfflineLifecycle) => void, timeoutMs = 8000, appVersion?: string, updatePollMs = 900_000): () => void {
  let current: OfflineLifecycle = { state: 'checking', cacheReady: false, verifiedAssetUrls: [] }
  let disposed = false
  let timeout: ReturnType<typeof setTimeout> | undefined
  const publish = (event: WorkerStatus | WorkerProgress | PendingRelease | 'checking' | 'update-available' | 'unavailable' | 'timeout') => { if (disposed) return; const next = reduceOfflineLifecycle(current, event, payload); if (JSON.stringify(next) !== JSON.stringify(current)) { current = next; onState(next) } }
  const resetTimeout = () => { if (timeout) clearTimeout(timeout); timeout = setTimeout(() => publish('timeout'), timeoutMs) }
  if (!expectedPageId || !payload || expectedPageId !== payload.pageId || !releaseIdPattern.test(expectedPageId) || !('serviceWorker' in navigator) || !window.isSecureContext) { publish('unavailable'); return () => { disposed = true } }
  onState(current)
  const onMessage = (event: MessageEvent<unknown>) => {
    const message = parseWorkerStatus(event.data) ?? parseWorkerProgress(event.data)
    if (!message) return
    if (message.type === 'offline-status' && message.state === 'ready') { if (timeout) clearTimeout(timeout); timeout = undefined }
    else if (message.type === 'offline-progress' && message.state !== 'failed') resetTimeout()
    publish(message)
  }
  navigator.serviceWorker.addEventListener('message', onMessage)
  resetTimeout()
  let registration: ServiceWorkerRegistration | undefined
  let installing: ServiceWorker | null = null
  // NOTHING RE-CHECKED BEFORE THIS. Registration fired `updatefound` only for an
  // update the browser happened to notice, which for a tab left open across a
  // deploy is "on the next navigation, or in about a day". An author who leaves
  // folio8 open all week was the last to hear about a release meant for them.
  // The three triggers are the three moments the answer can have changed: a
  // timer for the long-lived tab, coming back to the tab, and regaining network.
  let poll: ReturnType<typeof setInterval> | undefined
  const checkForUpdate = () => { if (!disposed) void registration?.update().catch(() => undefined) }
  const onVisible = () => { if (document.visibilityState === 'visible') checkForUpdate() }
  const startUpdatePolling = () => { poll = setInterval(checkForUpdate, updatePollMs); document.addEventListener('visibilitychange', onVisible); window.addEventListener('online', checkForUpdate) }
  const stopUpdatePolling = () => { if (poll) clearInterval(poll); poll = undefined; document.removeEventListener('visibilitychange', onVisible); window.removeEventListener('online', checkForUpdate) }
  // The waiting worker is asked what it is before the author is told anything, so
  // the prompt they see is already the right KIND of prompt. `update-available`
  // still publishes first for a worker that never answers.
  const announcePending = () => { const waiting = registration?.waiting; if (!waiting || disposed) return; void askWaitingVersion(waiting).then((version) => { if (!disposed) publish({ kind: 'pending-release', appVersion: version, mandatory: upgradeIsMandatory(appVersion, version) }) }) }
  const onStateChange = () => { if (!registration || !installing || disposed) return; if (installing.state === 'installed' && registration.waiting && navigator.serviceWorker.controller) { publish('update-available'); announcePending() } if (installing.state === 'redundant' && !navigator.serviceWorker.controller) publish('unavailable') }
  const onUpdateFound = () => { if (!registration) return; if (installing) installing.removeEventListener('statechange', onStateChange); installing = registration.installing; installing?.addEventListener('statechange', onStateChange); onStateChange() }
  void navigator.serviceWorker.register('/sw.js', { scope: '/' }).then((registered) => { if (disposed) return; registration = registered; registration.addEventListener('updatefound', onUpdateFound); onUpdateFound(); if (registration.waiting && navigator.serviceWorker.controller) { publish('update-available'); announcePending() } startUpdatePolling(); (registration.active ?? navigator.serviceWorker.controller)?.postMessage({ version: MESSAGE_VERSION, type: 'get-offline-status' }) }).catch(() => publish('unavailable'))
  return () => { disposed = true; if (timeout) clearTimeout(timeout); stopUpdatePolling(); navigator.serviceWorker.removeEventListener('message', onMessage); registration?.removeEventListener('updatefound', onUpdateFound); installing?.removeEventListener('statechange', onStateChange) }
}
