/// <reference lib="webworker" />

import { ENGINE_PROTOCOL_VERSION, MAX_ENGINE_RENDER_PDF_BYTES, type EngineDiagnostic, type EngineError, type EngineRequest, type EngineSnapshot, type IdentityPayload, type InstallFacePayload, type RenderPayload, type TableColumns, type GroupMovePreview } from './engine-protocol'
import { EngineRequestAdmission } from './engine-worker-admission'
import { EngineWorkerQueue } from './engine-worker-queue'
import { runtimeAssetUrls } from './generated/offline-assets'

declare const Go: new () => { importObject: WebAssembly.Imports; run(instance: WebAssembly.Instance): void }

// `installFace` IS A SECOND ENTRY POINT, NOT A SECOND PROTOCOL
// (spec-deferred-offline-cache, CAP-6). It answers with the same response JSON
// `handle` answers with, so everything below this line parses one shape.
//
// ⚠ IT EXISTS TO AVOID `bytesToBase64`, WHICH IS THE WHOLE POINT. That function
// is a `String.fromCharCode` loop over every byte; the CJK face is 10.11 MiB,
// so routing it through the envelope would build a ~13.5 MiB intermediate
// string on this thread — and would then be refused by the envelope's 8 MiB
// bound at both ends anyway. A Uint8Array over the already-transferred
// ArrayBuffer is copied into Go memory by `js.CopyBytesToGo` with no
// intermediate at all.
type WasmHost = { handle(request: string): string; installFace(face: string, bytes: Uint8Array): string }
// The WASM host's own reply shape. `tableColumns` is the PROTOCOL type's table
// object, not a fourth structural copy of it: this line used to re-declare the
// ten column keys inline, so every record that widened the projection — the Go
// struct, the guard's key list, engine-client's settle path — missed it, and it
// went stale silently because nothing here is checked against anything (Story
// 12.3). Naming the shared type is what makes it impossible to miss again.
type WasmResponse = { groupMove?: GroupMovePreview; ok: boolean; snapshot?: EngineSnapshot; bytesBase64?: string; diagnosticCode?: string; message?: string; elementId?: string; dataPath?: string; pdfSha256?: string; previewIdentity?: string; renderRevision?: number; diagnostics?: EngineDiagnostic[]; elapsedMs?: number; version?: string; parameterReferences?: string[]; parameterReferenceRevision?: number; tableColumns?: TableColumns['table']; tableColumnsRevision?: number }

const worker = self as unknown as DedicatedWorkerGlobalScope
let host: WasmHost | undefined
let bootFailure: EngineError | undefined
let lifecycleState: 'ready' | 'failed' | undefined
let booted: Promise<void> | undefined
const admission = new EngineRequestAdmission()
const queue = new EngineWorkerQueue<EngineRequest>(async (request) => {
  await booted
  if (bootFailure) { respondFailure(request.requestId, bootFailure); return }
  await execute(request)
})

function lifecycle(state: 'ready' | 'failed', error?: EngineError) {
  if (lifecycleState === state) return
  lifecycleState = state
  worker.postMessage({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'lifecycle', state, ...(error ? { error } : {}) })
}

async function boot(): Promise<void> {
  try {
    // The emitted release worker is a classic worker; the dev server serves an
    // ES module worker instead (see the construction site in engine-client.ts).
    // The glue assigns `globalThis.Go` either way.
    if (import.meta.env.DEV) await import(/* @vite-ignore */ runtimeAssetUrls.wasmExec)
    else importScripts(runtimeAssetUrls.wasmExec)
    const go = new Go()
    const result = await WebAssembly.instantiateStreaming(fetch(runtimeAssetUrls.wasm), go.importObject)
    go.run(result.instance)
    const candidate = (globalThis as typeof globalThis & { Folio8WasmHost?: WasmHost }).Folio8WasmHost
    if (!candidate) throw new Error('wasm host did not register')
    if (bootFailure) return
    host = candidate
    lifecycle('ready')
  } catch {
    failWorker({ code: 'WASM_INITIALIZATION_FAILED', message: 'The folio8 engine could not start' })
  }
}

worker.onmessage = (event: MessageEvent<unknown>) => {
  const result = admission.admit(event.data)
  if (result.kind === 'enqueue') { queue.enqueue(result.request); return }
  if (result.fatal) { failWorker(result.error); return }
  respondFailure(result.requestId!, result.error)
}

// THE BOUNDARY'S STAGES, named so a throw can say WHERE it happened.
//
// Story 16.0. This `catch` used to be bare, and reported one sentence — "The
// engine returned an invalid response" — for every throw it saw, keeping
// none of them. That sentence is the least useful in the product: it is
// never an engine refusal (those carry a diagnosticCode and a located
// message through the arm below), so it fires exactly when nobody
// anticipated the failure, and it is exactly then that it erases the only
// evidence there is. A failure an author can photograph but nobody can
// diagnose is two defects, and this is the second one.
//
// The stage is set immediately BEFORE the step it names, so the value the
// catch reads is the step that was running. Four are distinguishable and
// were not: encoding the request, the host throwing, the response not
// parsing, and a response-side transport-bound breach — which, before this,
// was indistinguishable from a request-side fault even though
// `base64ToBytesBounded` throws its own perfectly clear sentence.
type BoundaryStage = 'request' | 'host' | 'response' | 'bytes' | 'reply'
const boundaryCodes: Readonly<Record<BoundaryStage, string>> = {
  request: 'WASM_REQUEST_ENCODING_FAILED',
  host: 'WASM_HOST_FAILED',
  response: 'WASM_RESPONSE_UNPARSABLE',
  bytes: 'WASM_RESPONSE_BYTES_REFUSED',
  reply: 'WASM_PROTOCOL_FAILURE',
}
const boundarySentences: Readonly<Record<BoundaryStage, string>> = {
  request: 'The engine request could not be encoded',
  host: 'The engine threw while handling the request',
  response: "The engine's response could not be parsed",
  bytes: "The engine's byte response was refused",
  reply: 'The engine returned an invalid response',
}

// What threw, in one bounded phrase. `message` is a string this worker did
// not author — a DOM exception's, V8's, or Go's — so it is bounded like
// every other string on this protocol rather than trusted. The whole error
// stays under the 512 the protocol admits (engine-protocol.ts's `isError`),
// which is why the cause is cut at 320 and not at 512.
function describeThrow(thrown: unknown): string {
  if (thrown instanceof Error) return `${thrown.name || 'Error'}: ${thrown.message}`.slice(0, 320)
  if (typeof thrown === 'string') return thrown.slice(0, 320)
  return Object.prototype.toString.call(thrown).slice(0, 320)
}

async function execute(request: EngineRequest): Promise<void> {
  let stage: BoundaryStage = 'request'
  try {
    const render = request.operation === 'render' ? request.payload as RenderPayload : undefined
    const identity = request.operation === 'identity' ? request.payload as IdentityPayload : undefined
    // THE ONE OPERATION THAT DOES NOT BUILD AN ENVELOPE. Its bytes are handed
    // to the host as they arrived — transferred, uncopied, unencoded.
    const installFace = request.operation === 'install-face' ? request.payload as InstallFacePayload : undefined
    const payloadBase64 = installFace === undefined && request.payload instanceof ArrayBuffer ? bytesToBase64(request.payload) : undefined
    const envelope = installFace ? '' : JSON.stringify({ operation: request.operation, ...(payloadBase64 ? { payloadBase64 } : {}), ...(render ? { templateBase64: bytesToBase64(render.template), dataBase64: bytesToBase64(render.data), paramsBase64: bytesToBase64(render.params) } : {}), ...(identity ? { dataBase64: bytesToBase64(identity.data), paramsBase64: bytesToBase64(identity.params) } : {}) })
    stage = 'host'
    const raw = installFace ? host!.installFace(installFace.face, new Uint8Array(installFace.bytes)) : host!.handle(envelope)
    stage = 'response'
    const result = JSON.parse(raw) as WasmResponse
    stage = 'reply'
    if (!result.ok || !result.snapshot) {
      respondFailure(request.requestId, {
        code: result.diagnosticCode ?? 'ENGINE_REJECTED',
        message: (result.message ?? 'Engine rejected request').slice(0, 512),
        ...(result.elementId ? { elementId: result.elementId.slice(0, 128) } : {}),
        ...(result.dataPath ? { dataPath: result.dataPath.slice(0, 256) } : {}),
      })
      return
    }
    // Reject a hostile/buggy producer before atob allocates its decoded
    // buffer. The protocol repeats this guard on the main-thread boundary.
    stage = 'bytes'
    const bytes = result.bytesBase64 ? base64ToBytesBounded(result.bytesBase64, request.operation === 'render' ? MAX_ENGINE_RENDER_PDF_BYTES : undefined) : undefined
    stage = 'reply'
    // ⚠ EVERY MEMBER OF THE RENDER ARM IS NAMED HERE, AND A GO FIELD THAT IS
    // NOT NAMED IS DROPPED BEFORE `parseInbound` EVER SEES IT. Story 12.3's
    // table projection was lost exactly this way, silently, with no protocol
    // failure and nothing in the DOM to say so. `elapsedMs` and `version` join
    // `pdfSha256`/`diagnostics` as one all-or-nothing group.
    const preview = request.operation === 'render' ? { revision: result.renderRevision, identity: result.previewIdentity, pdfSha256: result.pdfSha256, diagnostics: result.diagnostics, elapsedMs: result.elapsedMs, version: result.version } : request.operation === 'identity' ? { revision: result.renderRevision, identity: result.previewIdentity } : undefined
    const parameterReferences = request.operation === 'parameter-references' ? { revision: result.parameterReferenceRevision, names: result.parameterReferences } : undefined
    const tableColumns = request.operation === 'table-columns' ? { revision: result.tableColumnsRevision, table: result.tableColumns } : undefined
    worker.postMessage({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: request.requestId, ok: true, snapshot: result.snapshot, ...(bytes ? { bytes } : {}), ...(preview ? { preview } : {}), ...(parameterReferences ? { parameterReferences } : {}), ...(tableColumns ? { tableColumns } : {}), ...(request.operation === 'group-move-preview' ? { groupMove: result.groupMove } : {}) }, bytes ? [bytes] : [])
  } catch (thrown) {
    // REPORT THE CAUSE, DO NOT MERELY NAME THE STAGE. The sentence says what
    // this boundary was doing; the phrase after it is the thrown value's own,
    // which is the half that was being thrown away.
    respondFailure(request.requestId, { code: boundaryCodes[stage], message: `${boundarySentences[stage]}: ${describeThrow(thrown)}`.slice(0, 512) })
  }
}

function respondFailure(requestId: string, error: EngineError) {
  worker.postMessage({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId, ok: false, error })
}

function failWorker(error: EngineError): void {
  if (bootFailure) return
  bootFailure = error
  lifecycle('failed', error)
}

function bytesToBase64(bytes: ArrayBuffer): string { let text = ''; for (const byte of new Uint8Array(bytes)) text += String.fromCharCode(byte); return btoa(text) }
function base64ToBytesBounded(value: string, max?: number): ArrayBuffer {
  const padding = value.endsWith('==') ? 2 : value.endsWith('=') ? 1 : 0
  const byteLength = value.length % 4 === 0 ? value.length / 4 * 3 - padding : -1
  if (byteLength < 0 || (max !== undefined && byteLength > max)) throw new Error('WASM byte response exceeds its transport limit')
  const text = atob(value)
  if (max !== undefined && text.length > max) throw new Error('WASM byte response exceeds its transport limit')
  const bytes = new Uint8Array(text.length); for (let index = 0; index < text.length; index++) bytes[index] = text.charCodeAt(index); return bytes.buffer
}

booted = boot()
