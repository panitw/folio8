import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App.tsx'
import { getEngineClient, type EngineClient } from './engine-client.ts'
import type { EngineSnapshot } from './engine-protocol.ts'
import { engineMayStart, registerOfflineLifecycle, type OfflineLifecycle } from './offline-lifecycle.ts'
import { isDevBypassReason, loadS1Payload, payloadForLifecycle, type S1Payload } from './release-payload.ts'
import { runtimeAssetUrls } from './generated/offline-assets.ts'
// The bundled example templates (startup templates). Loading the module from
// the application entry — never the engine worker — puts their template,
// sample and thumbnail URLs in Vite's asset graph. App opens the startup dialog
// only when it is handed them, which happens once the engine is ready.
import { exampleAssets } from './generated/example-assets.ts'
import { loadStarterAfterEngineReady } from './startup-sequence.ts'
import { selectFileAccess, selectImageFileAccess, selectSampleFileAccess } from './file/capability.ts'

const root = createRoot(document.getElementById('root')!)
let lifecycle: OfflineLifecycle = { state: 'checking', cacheReady: false, verifiedAssetUrls: [] }
let payload: S1Payload | undefined
let engine: EngineClient | undefined
let snapshot: EngineSnapshot | undefined
let blankBytes: ArrayBuffer | undefined
let engineState: 'waiting' | 'starting' | 'failed' = 'waiting'
let started = false
let stopObservation: (() => void) | undefined
let observationInFlight = false
const fileAccess = selectFileAccess()
const sampleFileAccess = selectSampleFileAccess()
const imageFileAccess = selectImageFileAccess()
const render = () => root.render(<StrictMode><App key={engine ? 'engine-ready' : 'engine-loading'} engine={engine} fileAccess={fileAccess} sampleFileAccess={sampleFileAccess} imageFileAccess={imageFileAccess} initialSnapshot={snapshot} blankBytes={blankBytes} examples={engine ? exampleAssets : undefined} offlineState={lifecycle.state} loadState={lifecycle} payload={payload} engineState={engineState} onRetry={startObservation} /></StrictMode>)
async function startEngine() {
  if (started || !engineMayStart(lifecycle)) return
  started = true; engineState = 'starting'; render()
  try {
    const startedEngine = await loadStarterAfterEngineReady(getEngineClient(), runtimeAssetUrls.starter, (url) => fetch(url, { credentials: 'omit' }))
    engine = startedEngine.client; snapshot = startedEngine.snapshot; blankBytes = startedEngine.blankBytes; render()
  } catch { engineState = 'failed'; render() }
}
async function startObservation() {
  if (engineState === 'failed') { window.location.reload(); return }
  if (observationInFlight) return
  observationInFlight = true
  stopObservation?.()
  lifecycle = { state: 'checking', cacheReady: false, verifiedAssetUrls: [] }; engineState = 'waiting'; render()
  try {
    const result = loadS1Payload()
    // Both decisions this block makes about the result are DELEGATED to pure
    // functions in release-payload.ts, because nothing imports this module and
    // Vitest collects no test that could execute an expression written here.
    // Inline, either could be mutated with every gate staying green.
    payload = payloadForLifecycle(result)
    const expectedPageId = document.querySelector('meta[name="folio8-page-release"]')?.getAttribute('content') ?? undefined
    // The version THIS page was built at, beside the release identity it already
    // reads. Absent on a dev server and on any page built before versioning, and
    // absent is answered by `upgradeIsMandatory` as "optional" rather than by a
    // guess in either direction.
    const appVersion = document.querySelector('meta[name="folio8-app-version"]')?.getAttribute('content') ?? undefined
    // The dev server emits no release bootstrap, so there is nothing to verify.
    // Start the engine straight from the module graph and let the shell say so.
    // GATED ON THAT ONE REASON, not on a falsy payload: a bootstrap that is
    // malformed, or over the release bound, is a real fault and must not be read
    // as "the dev server did not emit one" and quietly bypassed.
    if (import.meta.env.DEV && isDevBypassReason(result)) { lifecycle = { state: 'dev-bypass', cacheReady: false, verifiedAssetUrls: [] }; render(); void startEngine(); return }
    stopObservation = registerOfflineLifecycle(expectedPageId, payload, (next) => { lifecycle = next; render(); void startEngine() }, undefined, appVersion)
    render()
  } finally { observationInFlight = false }
}
void startObservation()
