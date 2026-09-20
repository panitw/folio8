import { copyBytes, deepFreeze, ENGINE_PROTOCOL_VERSION, parseInbound, type EngineError, type EngineInbound, type EngineOperation, type EngineRequest, type EngineSnapshot, type IdentityPayload, type InstallFacePayload, type RenderPayload, type TableColumns, type GroupMovePreview } from './engine-protocol'

export interface WorkerPort {
  postMessage(message: EngineRequest, transfer?: Transferable[]): void
  terminate(): void
  onmessage: ((event: MessageEvent<unknown>) => void) | null
  onerror: ((event: ErrorEvent) => void) | null
}

export type EngineResult = Readonly<{ snapshot: EngineSnapshot; bytes?: ArrayBuffer; preview?: Readonly<{ revision: number; identity: string; pdfSha256?: string; diagnostics?: ReadonlyArray<{ severity: 'warning'; code: string; elementId: string; dataPath: string; message: string }>; elapsedMs?: number; version?: string }>; parameterReferences?: Readonly<{ revision: number; names: ReadonlyArray<string> }>; tableColumns?: TableColumns; groupMove?: GroupMovePreview }>

export type EnginePayload = ArrayBuffer | RenderPayload | IdentityPayload | InstallFacePayload

// THE ABSENT-FACE CODE, SPELLED ONCE ON THIS SIDE. It is Go's
// `DiagCodeTextFaceAbsent` (folio-go/internal/diag/diag.go), arriving as the
// `code` of a refusal, and it is the ONE code this transport is allowed to
// answer with anything other than a rejection.
export const TEXT_FACE_ABSENT = 'TEXT_FACE_ABSENT'

// A recovery is the application's answer to "the engine is short of a face".
// It is handed the refusal and returns whether it managed to supply what was
// missing; `true` means retry the request that refused, once.
//
// ⚠ THE CLIENT DOES NOT KNOW WHAT A FACE IS, AND MUST NOT. Resolving a name to
// a release asset, fetching it and deciding that a face the release does not
// carry is a dead end are all application questions
// (`src/absent-face-recovery.ts`). What lives here is only the one thing the
// transport owns: which request to send again, and the guarantee that it is
// sent again AT MOST ONCE.
export type AbsentFaceRecovery = (error: EngineError) => Promise<boolean>

type Pending = { operation: EngineOperation; payload?: EnginePayload; retried: boolean; signal?: AbortSignal; resolve: (result: EngineResult) => void; reject: (error: Error) => void }
type Live = { pending: Pending; detach: () => void }
type ClientState = 'starting' | 'ready' | 'failed' | 'terminated'

type ProducerRenderError = Error & Readonly<{ code: string; elementId?: string; dataPath?: string; producerRenderFailure: true }>

const errorFor = (code: string, message: string, dataPath?: string, elementId?: string) => Object.assign(new Error(message), { code, ...(dataPath !== undefined ? { dataPath } : {}), ...(elementId !== undefined ? { elementId } : {}) })
const producerRenderErrorFor = (code: string, message: string, dataPath?: string, elementId?: string): ProducerRenderError => Object.assign(errorFor(code, message, dataPath, elementId), { producerRenderFailure: true as const })

// This marker records transport provenance, not an error taxonomy.  Only a
// rejected producer `render` response earns the failed-render UI; worker,
// identity, serialization, and viewer failures remain local Preview state.
export function isProducerRenderFailure(error: unknown): error is ProducerRenderError {
  return error instanceof Error && (error as Partial<ProducerRenderError>).producerRenderFailure === true
}

export class EngineClient {
	#state: ClientState = 'starting'
	#nextRequest = 0
	#pending = new Map<string, Live>()
	#recover?: AbsentFaceRecovery
	#order: string[] = []
	#abandoned = new Set<string>()
	#ready: Promise<EngineClient>
	#resolveReady!: (client: EngineClient) => void
	#rejectReady!: (error: Error) => void
	private readonly worker: WorkerPort

	constructor(worker: WorkerPort) {
    this.worker = worker
    this.#ready = new Promise<EngineClient>((resolve, reject) => { this.#resolveReady = resolve; this.#rejectReady = reject })
    // Direct clients are useful in focused tests and do not always await the
    // lifecycle promise. Keep a handled observer while callers still receive
    // the original rejection from whenReady()/the singleton.
    void this.#ready.catch(() => undefined)
    worker.onmessage = (event) => this.#onMessage(event.data)
    worker.onerror = () => this.#fail('WORKER_RUNTIME_ERROR', 'The engine worker failed')
  }

  get state(): ClientState { return this.#state }
  whenReady(): Promise<EngineClient> { return this.#ready }

  // ONE RECOVERY, INSTALLED ONCE, FOR THE LIFE OF THE CLIENT
  // (spec-deferred-offline-cache, CAP-6). It is a setter rather than a
  // constructor argument because the recovery needs the client it recovers for:
  // installing a face is itself an engine request.
  onAbsentFace(recover: AbsentFaceRecovery): void { this.#recover = recover }

  request(operation: EngineOperation, payload?: EnginePayload, signal?: AbortSignal): Promise<EngineResult> {
    if (this.#state !== 'ready') return Promise.reject(errorFor('ENGINE_NOT_READY', `Engine is ${this.#state}`))
    if (signal?.aborted) return Promise.reject(errorFor('REQUEST_ABORTED', 'Engine request was abandoned'))
    return new Promise<EngineResult>((resolve, reject) => {
      // THE PAYLOAD IS COPIED HERE AND KEPT, and the copy `#send` transfers is
      // a copy OF THAT. A transferred ArrayBuffer is detached, so a retry has
      // nothing to resend unless a retained copy exists — and the caller's own
      // buffer was never a candidate: it has been detached-safe since the day
      // this class started copying it.
      this.#send({ operation, payload: payload ? copyPayload(payload) : undefined, retried: false, signal, resolve, reject })
    })
  }

  #send(pending: Pending): void {
    const requestId = `request-${++this.#nextRequest}`
    const payloadCopy = pending.payload ? copyPayload(pending.payload) : undefined
    const request: EngineRequest = { protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId, operation: pending.operation, ...(payloadCopy ? { payload: payloadCopy } : {}) }
    const abort = () => {
      if (this.#pending.delete(requestId)) {
        this.#abandoned.add(requestId)
        pending.reject(errorFor('REQUEST_ABORTED', 'Engine request was abandoned'))
      }
    }
    pending.signal?.addEventListener('abort', abort, { once: true })
    this.#pending.set(requestId, { pending, detach: () => pending.signal?.removeEventListener('abort', abort) })
    this.#order.push(requestId)
    try { workerPost(this.worker, request, payloadCopy) } catch { this.#fail('WORKER_POST_FAILED', 'Could not send engine request') }
  }

  // WHAT HAPPENS BETWEEN A REFUSAL AND ITS ONE RETRY. The recovery is awaited
  // — it fetches ~10 MiB over the network — so anything can have happened by
  // the time it answers, and each of those states is checked rather than
  // assumed: the request may have been abandoned, the client may have failed
  // or been terminated, and the recovery may simply have had nothing to offer.
  // In every one of those cases the ORIGINAL refusal is what the caller sees,
  // never a second error describing the recovery's own disappointment.
  async #retryAfterRecovery(pending: Pending, error: EngineError): Promise<void> {
    let recovered = false
    try { recovered = await this.#recover!(error) } catch { recovered = false }
    if (pending.signal?.aborted) { pending.reject(errorFor('REQUEST_ABORTED', 'Engine request was abandoned')); return }
    if (!recovered || this.#state !== 'ready') { pending.reject(rejectionFor(pending.operation, error)); return }
    this.#send(pending)
  }

  terminate(): void {
    if (this.#state === 'terminated') return
    this.#state = 'terminated'
    this.#detach()
    this.worker.terminate()
    this.#rejectReady(errorFor('ENGINE_TERMINATED', 'The engine worker was terminated'))
    this.#rejectPending('ENGINE_TERMINATED', 'The engine worker was terminated')
  }

  #onMessage(raw: unknown): void {
    const message = parseInbound(raw)
    if (!message) { this.#fail('PROTOCOL_INVALID', 'The engine sent an invalid protocol message'); return }
    if (message.kind === 'lifecycle') {
      if (message.state === 'ready' && this.#state === 'starting') { this.#state = 'ready'; this.#resolveReady(this); return }
      this.#fail(message.error?.code ?? 'LIFECYCLE_INVALID', message.error?.message ?? 'Invalid engine lifecycle transition')
      return
    }
    this.#settle(message)
  }

  #settle(message: Exclude<EngineInbound, { kind: 'lifecycle' }>): void {
    if (this.#order[0] !== message.requestId) {
      this.#fail('PROTOCOL_OUT_OF_ORDER', 'The engine sent responses out of FIFO order')
      return
    }
    this.#order.shift()
    if (this.#abandoned.delete(message.requestId)) return
    const live = this.#pending.get(message.requestId)
    if (!live) { this.#fail('PROTOCOL_DUPLICATE_OR_UNKNOWN', 'The engine sent an unknown or duplicate response'); return }
    this.#pending.delete(message.requestId)
    live.detach()
    const pending = live.pending
    if (!message.ok) {
      // THE ONE REFUSAL THIS TRANSPORT ANSWERS INSTEAD OF REPORTING
      // (spec-deferred-offline-cache, CAP-6/CAP-7). The engine has said which
      // face it is short of; the application can fetch that face; the request
      // then succeeds. Reporting it to the caller would surface a failure that
      // is about the DESIGNER's download state and not about their document.
      //
      // ⚠ ONCE. `retried` is set before the retry is sent, so a request can
      // refuse, recover and refuse again — and the second refusal reaches the
      // caller. A document naming a face the release does not carry therefore
      // surfaces rather than looping, and so does a fetch that succeeds over
      // bytes the engine will not accept.
      //
      // ⚠ AND NEVER FOR `install-face` ITSELF, which is the request the
      // recovery makes: a recovery that recursed through its own refusal would
      // be a loop with an await in it.
      if (message.error.code === TEXT_FACE_ABSENT && this.#recover && !pending.retried && pending.operation !== 'install-face') {
        pending.retried = true
        void this.#retryAfterRecovery(pending, message.error)
        return
      }
      pending.reject(rejectionFor(pending.operation, message.error))
      return
    }
		const mismatch = !matchesOperationPayload(pending.operation, message)
		if (mismatch) { pending.reject(errorFor('PROTOCOL_OPERATION_MISMATCH', 'The engine response did not match its request')); this.#fail('PROTOCOL_OPERATION_MISMATCH', 'The engine response did not match its request'); return }
    const snapshot = deepFreeze({ ...message.snapshot }) as EngineSnapshot
    const bytes = message.bytes ? copyBytes(message.bytes) : undefined
		// STORY 13.3 — THE SECOND HAND-ENUMERATED HOP, and the same trap as the
		// worker's. The render arm is rebuilt member by member inside `deepFreeze`,
		// so a field absent from this literal never reaches App.tsx however well it
		// passed `isPreview`. All four render-only members are named together.
		const preview = message.preview ? deepFreeze({ revision: message.preview.revision, identity: message.preview.identity, ...(message.preview.pdfSha256 ? { pdfSha256: message.preview.pdfSha256, diagnostics: message.preview.diagnostics!.map((diagnostic) => ({ ...diagnostic })), elapsedMs: message.preview.elapsedMs!, version: message.preview.version! } : {}) }) : undefined
		const parameterReferences = message.parameterReferences ? deepFreeze({ revision: message.parameterReferences.revision, names: [...message.parameterReferences.names] }) : undefined
		// THE TABLE OBJECT IS SPREAD, NOT RE-ENUMERATED, and that is a FIX
		// rather than a tidy-up (Story 12.3).
		//
		// This line used to name the table's four members one at a time:
		// `{ tableId, collection, alias, columns: [...] }`. Columns rode a spread
		// and survived; a new table-LEVEL member did not. It passed
		// `isTableColumns`, reached here, and was DROPPED — silently, before
		// App.tsx ever saw it, with no protocol failure and nothing in the DOM to
		// say so. A guard mismatch at least kills the worker loudly; this failed
		// quietly, which is worse, and no test covered it. All sixteen of Story
		// 12.3's projection members ride this path.
		//
		// The copy is still a COPY — the object is rebuilt and every column
		// cloned, so nothing the worker sent stays reachable through the frozen
		// result — but its member list is now the response's own, so the next
		// story that widens the projection does not have to find this line.
		const tableColumns = message.tableColumns ? deepFreeze({ revision: message.tableColumns.revision, table: { ...message.tableColumns.table, columns: message.tableColumns.table.columns.map((column) => ({ ...column })) } }) : undefined
		pending.resolve(deepFreeze({ snapshot, ...(bytes ? { bytes } : {}), ...(preview ? { preview } : {}), ...(parameterReferences ? { parameterReferences } : {}), ...(tableColumns ? { tableColumns } : {}), ...(message.groupMove ? { groupMove: { ...message.groupMove } } : {}) }))
  }

  #fail(code: string, message: string): void {
    if (this.#state === 'failed' || this.#state === 'terminated') return
    this.#state = 'failed'
    this.#detach()
    this.worker.terminate()
    this.#rejectReady(errorFor(code, message))
    this.#rejectPending(code, message)
  }

  #rejectPending(code: string, message: string): void {
    for (const live of this.#pending.values()) { live.detach(); live.pending.reject(errorFor(code, message)) }
    this.#pending.clear()
    this.#abandoned.clear()
    this.#order = []
  }

  #detach(): void { this.worker.onmessage = null; this.worker.onerror = null }
}

function matchesOperationPayload(operation: EngineOperation, message: Extract<EngineInbound, { kind: 'response'; ok: true }>): boolean {
  if (operation !== 'group-move-preview' && message.groupMove !== undefined) return false
  const none = message.bytes === undefined && message.preview === undefined && message.parameterReferences === undefined && message.tableColumns === undefined
  switch (operation) {
    case 'render': return message.bytes !== undefined && message.preview?.pdfSha256 !== undefined && message.preview.diagnostics !== undefined && message.preview.elapsedMs !== undefined && message.preview.version !== undefined && message.parameterReferences === undefined && message.tableColumns === undefined
    case 'identity': return message.bytes === undefined && message.preview !== undefined && message.preview.pdfSha256 === undefined && message.preview.diagnostics === undefined && message.preview.elapsedMs === undefined && message.preview.version === undefined && message.parameterReferences === undefined && message.tableColumns === undefined
    case 'serialize': return message.bytes !== undefined && message.preview === undefined && message.parameterReferences === undefined && message.tableColumns === undefined
    case 'asset': return message.bytes !== undefined && message.preview === undefined && message.parameterReferences === undefined && message.tableColumns === undefined
    // The stand-in data document arrives as BYTES on the envelope that
    // already carries them. No new response field, no protocol version
    // change: a bytes-returning operation was already representable.
    case 'stand-in-data': return message.bytes !== undefined && message.preview === undefined && message.parameterReferences === undefined && message.tableColumns === undefined
    case 'parameter-references': return message.bytes === undefined && message.preview === undefined && message.parameterReferences !== undefined && message.tableColumns === undefined
    case 'group-move-preview': return none && message.groupMove !== undefined
    case 'table-columns': return message.bytes === undefined && message.preview === undefined && message.parameterReferences === undefined && message.tableColumns !== undefined
    // `install-face` FALLS THROUGH TO HERE, and that is the correct
    // statement about it: an install is not an edit, so its reply carries a
    // snapshot and nothing else — which is exactly what `none` already means.
    default: return none
  }
}

// A REJECTION'S PROVENANCE MARKER, in one place now that two call sites make
// one. Only a rejected producer `render` earns the failed-render UI.
function rejectionFor(operation: EngineOperation, error: EngineError): Error {
  return operation === 'render'
    ? producerRenderErrorFor(error.code, safeErrorMessage(error), error.dataPath, error.elementId)
    : errorFor(error.code, safeErrorMessage(error), error.dataPath, error.elementId)
}

function copyPayload(payload: EnginePayload): EnginePayload {
  return isArrayBuffer(payload) ? copyBytes(payload) : 'face' in payload ? { face: payload.face, bytes: copyBytes(payload.bytes) } : 'template' in payload ? { template: copyBytes(payload.template), data: copyBytes(payload.data), params: copyBytes(payload.params) } : { data: copyBytes(payload.data), params: copyBytes(payload.params) }
}

function workerPost(worker: WorkerPort, request: EngineRequest, payload?: EnginePayload): void {
  worker.postMessage(request, isArrayBuffer(payload) ? [payload] : payload ? ('face' in payload ? [payload.bytes] : 'template' in payload ? [payload.template, payload.data, payload.params] : [payload.data, payload.params]) : [])
}

function isArrayBuffer(value: unknown): value is ArrayBuffer { return Object.prototype.toString.call(value) === '[object ArrayBuffer]' }

export function createEngineClientSingleton(createWorker: () => WorkerPort): () => Promise<EngineClient> {
	let singleton: Promise<EngineClient> | undefined
	return () => {
		if (!singleton) {
      try {
        const client = new EngineClient(createWorker())
        singleton = client.whenReady()
      } catch {
        singleton = Promise.reject(errorFor('WORKER_CONSTRUCTION_FAILED', 'The engine worker could not be constructed'))
      }
    }
		return singleton
	}
}

function safeErrorMessage(error: EngineError): string {
  return error.message.slice(0, 512)
}

// This is the single discoverable Worker construction site in production.
// Vite's dev server serves worker modules unbundled, so a classic worker would
// receive `import` statements it cannot execute. Development asks for a module
// worker; the emitted release worker stays classic.
const appEngineClient = createEngineClientSingleton(() => import.meta.env.DEV
  ? new Worker(new URL('./engine.worker.ts', import.meta.url), { name: 'folio8-engine', type: 'module' })
  : new Worker(new URL('./engine.worker.ts', import.meta.url), { name: 'folio8-engine' }))
export const getEngineClient = (): Promise<EngineClient> => appEngineClient()
