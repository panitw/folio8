import { describe, expect, it, vi } from 'vitest'
import { createEngineClientSingleton, EngineClient, isProducerRenderFailure, type WorkerPort } from './engine-client'
import { ENGINE_PROTOCOL_VERSION, type EngineRequest } from './engine-protocol'

class FakeWorker implements WorkerPort {
  onmessage: ((event: MessageEvent<unknown>) => void) | null = null
  onerror: ((event: ErrorEvent) => void) | null = null
  readonly sent: EngineRequest[] = []
  terminated = 0
  readonly transfers: Transferable[][] = []
  postMessage(message: EngineRequest, transfer: Transferable[] = []): void { this.sent.push(message); this.transfers.push(transfer) }
  terminate(): void { this.terminated++ }
  emit(data: unknown): void { this.onmessage?.({ data } as MessageEvent<unknown>) }
  ready(): void { this.emit({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'lifecycle', state: 'ready' }) }
  respond(requestId: string, revision: number, bytes?: ArrayBuffer): void { this.emit({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId, ok: true, snapshot: { documentState: 'loaded', revision, byteLength: 10 }, ...(bytes ? { bytes } : {}) }) }
  refuse(requestId: string, code: string, message: string): void { this.emit({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId, ok: false, error: { code, message } }) }
}

describe('engine client protocol and lifecycle', () => {
  it('shares one worker and initialization promise across repeated consumers', async () => {
    let constructed = 0
    const worker = new FakeWorker()
    const getClient = createEngineClientSingleton(() => { constructed++; return worker })
    const firstPromise = getClient()
    const secondPromise = getClient()
    worker.ready()
    const [first, second] = await Promise.all([firstPromise, secondPromise])
    expect(first).toBe(second)
    expect(constructed).toBe(1) // Red proof target: a second constructor fails here.
  })

  it('rejects calls before startup and settles FIFO-correlated immutable results', async () => {
    const worker = new FakeWorker()
    const client = new EngineClient(worker)
    await expect(client.request('snapshot')).rejects.toMatchObject({ code: 'ENGINE_NOT_READY' })
    worker.ready()
    const first = client.request('snapshot')
    const second = client.request('serialize')
    expect(worker.sent.map((request) => request.requestId)).toEqual(['request-1', 'request-2'])
    worker.respond('request-1', 1)
    worker.respond('request-2', 2, new Uint8Array([1, 2]).buffer)
    const [one, two] = await Promise.all([first, second])
    expect(one.snapshot.revision).toBe(1)
    expect(two.snapshot.revision).toBe(2)
    expect(Object.isFrozen(one.snapshot)).toBe(true)
    expect(() => { (one.snapshot as { revision: number }).revision = 99 }).toThrow()
    expect(new Uint8Array(two.bytes!)).toEqual(new Uint8Array([1, 2]))
  })

  it('fails closed rather than applying an out-of-order response', async () => {
    const worker = new FakeWorker()
    const client = new EngineClient(worker)
    worker.ready()
    const first = client.request('snapshot')
    const second = client.request('serialize')
    worker.respond('request-2', 2)
    await expect(first).rejects.toMatchObject({ code: 'PROTOCOL_OUT_OF_ORDER' })
    await expect(second).rejects.toMatchObject({ code: 'PROTOCOL_OUT_OF_ORDER' })
    expect(client.state).toBe('failed')
  })

  it('fails closed on a duplicate, unknown version, and termination, rejecting pending work once', async () => {
    const worker = new FakeWorker()
    const client = new EngineClient(worker)
    worker.ready()
    const pending = client.request('snapshot')
    worker.respond('request-1', 1)
    await pending
    worker.respond('request-1', 1) // Red proof: duplicate response cannot settle twice.
    expect(client.state).toBe('failed')
    expect(worker.terminated).toBe(1)

    const secondWorker = new FakeWorker()
    const secondClient = new EngineClient(secondWorker)
    secondWorker.ready()
    const outstanding = secondClient.request('snapshot')
    secondClient.terminate()
    await expect(outstanding).rejects.toMatchObject({ code: 'ENGINE_TERMINATED' })

    const thirdWorker = new FakeWorker()
    const thirdClient = new EngineClient(thirdWorker)
    thirdWorker.emit({ protocolVersion: 999, kind: 'lifecycle', state: 'ready' })
    expect(thirdClient.state).toBe('failed')
  })

  it('abandons a caller without cancelling or mutating worker state', async () => {
    const worker = new FakeWorker()
    const client = new EngineClient(worker)
    worker.ready()
    const controller = new AbortController()
    const pending = client.request('snapshot', undefined, controller.signal)
    controller.abort()
    await expect(pending).rejects.toMatchObject({ code: 'REQUEST_ABORTED' })
    worker.respond('request-1', 1) // late, legitimate terminal response is consumed
    expect(client.state).toBe('ready')
    expect(worker.terminated).toBe(0)
  })

  it('preserves bounded component diagnostic location on a command rejection', async () => {
    const worker = new FakeWorker()
    const client = new EngineClient(worker)
    worker.ready()
    const pending = client.request('command', new Uint8Array([1]).buffer)
    worker.emit({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'request-1', ok: false, error: { code: 'COMPONENT_INVALID', message: 'bad x', elementId: 'e1', dataPath: 'component.x' } })
    await expect(pending).rejects.toMatchObject({ code: 'COMPONENT_INVALID', elementId: 'e1', dataPath: 'component.x', message: 'bad x' })
  })

  it('preserves the closed render-failure provenance without a browser code map', async () => {
    const worker = new FakeWorker()
    const client = new EngineClient(worker)
    worker.ready()
    const pending = client.request('render', { template: new Uint8Array([1]).buffer, data: new Uint8Array([2]).buffer, params: new Uint8Array([3]).buffer })
    worker.emit({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'request-1', ok: false, error: { code: 'PARAMETER_REQUIRED', message: 'The template could not be processed', elementId: 'e1', dataPath: 'params.reportDate' } })
    await expect(pending).rejects.toMatchObject({ code: 'PARAMETER_REQUIRED', message: 'The template could not be processed', elementId: 'e1', dataPath: 'params.reportDate' })
    await pending.catch((error: unknown) => expect(isProducerRenderFailure(error)).toBe(true))
  })

  it('does not label identity or worker lifecycle errors as producer render failures', async () => {
    const worker = new FakeWorker()
    const client = new EngineClient(worker)
    worker.ready()
    const identity = client.request('identity', { data: new Uint8Array([1]).buffer, params: new Uint8Array([2]).buffer })
    worker.emit({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'request-1', ok: false, error: { code: 'IDENTITY_UNAVAILABLE', message: 'identity unavailable' } })
    await identity.catch((error: unknown) => expect(isProducerRenderFailure(error)).toBe(false))

    const render = client.request('render', { template: new Uint8Array([1]).buffer, data: new Uint8Array([2]).buffer, params: new Uint8Array([3]).buffer })
    worker.onerror?.({} as ErrorEvent)
    await render.catch((error: unknown) => expect(isProducerRenderFailure(error)).toBe(false))
  })

  it('requests one asset key and resolves its bytes like serialize', async () => {
    const worker = new FakeWorker()
    const client = new EngineClient(worker)
    worker.ready()
    const pending = client.request('asset', new TextEncoder().encode('a'.repeat(64)).buffer)
    expect(worker.sent[0]!.operation).toBe('asset')
    worker.respond('request-1', 1, new Uint8Array([1, 2, 3]).buffer)
    const result = await pending
    expect(new Uint8Array(result.bytes!)).toEqual(new Uint8Array([1, 2, 3]))
  })

  it('fails closed when every operation receives surplus table metadata', async () => {
		for (const operation of ['render', 'identity', 'serialize', 'asset', 'parameter-references', 'stand-in-data', 'snapshot', 'command', 'undo', 'redo'] as const) {
    const worker = new FakeWorker()
    const client = new EngineClient(worker)
    worker.ready()
			const payload = operation === 'render' ? { template: new Uint8Array([1]).buffer, data: new Uint8Array([2]).buffer, params: new Uint8Array([3]).buffer } : operation === 'identity' ? { data: new Uint8Array([1]).buffer, params: new Uint8Array([2]).buffer } : operation === 'command' || operation === 'asset' ? new Uint8Array([1]).buffer : undefined
			const pending = client.request(operation, payload)
			const base = { protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response' as const, requestId: 'request-1', ok: true as const, snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 10 }, tableColumns: { revision: 1, table: { tableId: 'e7', sizing: 'points', totalWidth: 72000, collection: 'items[]', alias: 'row', headerHeight: 12000, altRowBackground: '', headerFontFamily: '', headerFontFamilyResolved: 'body', headerFontSize: 0, headerFontSizeResolved: 12000, headerLineSpacing: 0, headerLineSpacingResolved: 1000, headerBackground: '', headerBackgroundResolved: '', headerColor: '', headerColorResolved: '', headerValign: '', headerValignResolved: 'top', headerAlign: '', headerAlignResolved: 'left', headerBold: false, headerBoldResolved: false, headerItalic: false, headerItalicResolved: false, 'headerBorder.width': '', 'headerBorder.widthResolved': '', 'headerBorder.color': '', 'headerBorder.colorResolved': '', 'headerBorder.edges': '', 'headerBorder.edgesResolved': '', minHeight: 0, 'rules.width': '', 'rules.widthResolved': '', 'rules.color': '', 'rules.colorResolved': '', 'rules.between': '', paddingLeft: '', paddingRight: '', paddingHeaderOverride: false, columns: [] } } }
			worker.emit(operation === 'render' ? { ...base, bytes: new Uint8Array([9]).buffer, preview: { revision: 1, identity: 'a'.repeat(64), pdfSha256: 'b'.repeat(64), diagnostics: [], elapsedMs: 7, version: '0.0.0-dev' } } : operation === 'identity' ? { ...base, preview: { revision: 1, identity: 'a'.repeat(64) } } : operation === 'serialize' || operation === 'asset' || operation === 'stand-in-data' ? { ...base, bytes: new Uint8Array([9]).buffer } : operation === 'parameter-references' ? { ...base, parameterReferences: { revision: 1, names: [] } } : base)
			await expect(pending).rejects.toMatchObject({ code: 'PROTOCOL_OPERATION_MISMATCH' })
			expect(client.state).toBe('failed')
		}
  })

  // STORY 13.3 — THE SECOND HOP CARRIES THE RENDER FACTS THROUGH, AND THE
  // OPERATION GATE KNOWS WHICH REPLIES OWE THEM.
  //
  // `#settle` rebuilds the preview member by member inside `deepFreeze`, so a
  // field it does not name is dropped after admission and before App.tsx —
  // silently, which is the Story 12.3 failure shape. This asserts the values
  // arrive, not merely that the keys exist, and it asserts the ZERO case,
  // because a rebuild written with `??` or truthiness would turn a legitimate
  // `0 ms` into a missing field.
  it('carries the render facts across the client rebuild, zero included', async () => {
    for (const elapsedMs of [0, 412]) {
      const worker = new FakeWorker()
      const client = new EngineClient(worker)
      worker.ready()
      const pending = client.request('render', { template: new Uint8Array([1]).buffer, data: new Uint8Array([2]).buffer, params: new Uint8Array([3]).buffer })
      worker.emit({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'request-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 10 }, bytes: new Uint8Array([9]).buffer, preview: { revision: 1, identity: 'a'.repeat(64), pdfSha256: 'b'.repeat(64), diagnostics: [], elapsedMs, version: '0.0.0-dev' } })
      const result = await pending
      expect(result.preview?.elapsedMs, `the client rebuild dropped elapsedMs (${elapsedMs} ms)`).toBe(elapsedMs)
      expect(result.preview?.version, 'the client rebuild dropped version').toBe('0.0.0-dev')
      // An identity reply owes neither, and still resolves.
      const identityWorker = new FakeWorker()
      const identityClient = new EngineClient(identityWorker)
      identityWorker.ready()
      const identity = identityClient.request('identity', { data: new Uint8Array([1]).buffer, params: new Uint8Array([2]).buffer })
      identityWorker.emit({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'request-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 10 }, preview: { revision: 1, identity: 'a'.repeat(64) } })
      expect((await identity).preview?.elapsedMs).toBeUndefined()
    }
  })

  it('fails closed when a table-column reply carries unrelated success payloads', async () => {
		const worker = new FakeWorker()
		const client = new EngineClient(worker)
		worker.ready()
		const pending = client.request('table-columns', new TextEncoder().encode('{"id":"e7"}').buffer)
		worker.emit({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'request-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 10 }, tableColumns: { revision: 1, table: { tableId: 'e7', sizing: 'points', totalWidth: 72000, collection: 'items[]', alias: 'row', headerHeight: 12000, altRowBackground: '', headerFontFamily: '', headerFontFamilyResolved: 'body', headerFontSize: 0, headerFontSizeResolved: 12000, headerLineSpacing: 0, headerLineSpacingResolved: 1000, headerBackground: '', headerBackgroundResolved: '', headerColor: '', headerColorResolved: '', headerValign: '', headerValignResolved: 'top', headerAlign: '', headerAlignResolved: 'left', headerBold: false, headerBoldResolved: false, headerItalic: false, headerItalicResolved: false, 'headerBorder.width': '', 'headerBorder.widthResolved': '', 'headerBorder.color': '', 'headerBorder.colorResolved': '', 'headerBorder.edges': '', 'headerBorder.edgesResolved': '', minHeight: 0, 'rules.width': '', 'rules.widthResolved': '', 'rules.color': '', 'rules.colorResolved': '', 'rules.between': '', paddingLeft: '', paddingRight: '', paddingHeaderOverride: false, columns: [] } }, parameterReferences: { revision: 1, names: [] } })
		await expect(pending).rejects.toMatchObject({ code: 'PROTOCOL_OPERATION_MISMATCH' })
    expect(client.state).toBe('failed')
  })

  // STORY 12.3 — THE SILENT DROP, AND ITS TEST.
  //
  // #settle used to hand-enumerate the table object's four members. Columns rode
  // a spread and survived; a table-LEVEL member passed isTableColumns, reached
  // #settle, and was DISCARDED before App.tsx ever saw it — no protocol failure,
  // no console line, nothing in the DOM. A guard mismatch at least kills the
  // worker loudly. This failed quietly, which is worse, and NOTHING covered it:
  // every table assertion in the suite reads `columns`.
  //
  // The claim is therefore made against the SETTLED RESULT rather than against
  // any rendering of it, and it is made member by member so a partial
  // re-enumeration cannot pass.
  it('carries every table-level projection member through the settle path', async () => {
    const worker = new FakeWorker()
    const client = new EngineClient(worker)
    worker.ready()
    const pending = client.request('table-columns', new TextEncoder().encode('{"id":"e7"}').buffer)
    const table = { tableId: 'e7', sizing: 'points', totalWidth: 72000, collection: 'items[]', alias: 'row', headerHeight: 16000, altRowBackground: '#DDEEFF', headerFontFamily: 'body', headerFontFamilyResolved: 'body', headerFontSize: 14000, headerFontSizeResolved: 14000, headerLineSpacing: 1500, headerLineSpacingResolved: 1500, headerBackground: '#101010', headerBackgroundResolved: '#101010', headerColor: '#FFFFFF', headerColorResolved: '#FFFFFF', headerValign: 'middle', headerValignResolved: 'middle', headerAlign: 'center', headerAlignResolved: 'center', headerBold: false, headerBoldResolved: false, headerItalic: false, headerItalicResolved: false, 'headerBorder.width': '2000', 'headerBorder.widthResolved': '2000', 'headerBorder.color': '#334455', 'headerBorder.colorResolved': '#334455', 'headerBorder.edges': 'top,bottom', 'headerBorder.edgesResolved': 'top,bottom', minHeight: 0, 'rules.width': '', 'rules.widthResolved': '', 'rules.color': '', 'rules.colorResolved': '', 'rules.between': '', paddingLeft: '', paddingRight: '', paddingHeaderOverride: false, columns: [{ id: 'e8', header: 'Amount', width: 72000, proportion: '', align: 'right' as const, headerAlign: '' as const, headerAlignResolved: 'right' as const, binding: '{{row.amount}}', rowField: 'amount', rowFieldEditable: true, footer: '' as const, footerOf: '', footerFormat: '' }] }
    worker.emit({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'request-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 10 }, tableColumns: { revision: 1, table } })
    const settled = await pending
    // EVERY member, compared as a whole object: naming a subset here would
    // reproduce the very defect — a hand-written member list that a later
    // widening walks past.
    expect(settled.tableColumns?.table).toEqual(table)
    expect(Object.keys(settled.tableColumns!.table)).toEqual(Object.keys(table))
    // It is still a COPY and still frozen: nothing the worker sent stays
    // reachable through the result, and the result cannot be edited in place.
    expect(settled.tableColumns?.table).not.toBe(table)
    expect(settled.tableColumns?.table.columns[0]).not.toBe(table.columns[0])
    expect(Object.isFrozen(settled.tableColumns)).toBe(true)
    // Red proof for the copy half: mutating the worker's own object afterwards
    // must not reach the settled result.
    table.headerFontSize = 99
    expect(settled.tableColumns?.table.headerFontSize).toBe(14000)
  })

  it('rejects singleton startup failure and clears listeners on every terminal state', async () => {
    const worker = new FakeWorker()
    const getClient = createEngineClientSingleton(() => worker)
    const starting = getClient()
    worker.emit({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'lifecycle', state: 'failed', error: { code: 'WASM_INITIALIZATION_FAILED', message: 'safe' } })
    await expect(starting).rejects.toMatchObject({ code: 'WASM_INITIALIZATION_FAILED' })
    expect(worker.onmessage).toBeNull()
    expect(worker.onerror).toBeNull()

    const terminated = new FakeWorker()
    const client = new EngineClient(terminated)
    client.terminate()
    expect(terminated.onmessage).toBeNull()
    expect(terminated.onerror).toBeNull()
  })
})

describe('group move query transport', () => {
  it('returns frozen revision-correlated accepted geometry', async () => {
    const worker = new FakeWorker(); const client = new EngineClient(worker); worker.ready()
    const pending = client.request('group-move-preview', new Uint8Array([1]).buffer)
    worker.emit({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'request-1', ok: true, snapshot: { documentState: 'loaded', revision: 3, byteLength: 10 }, groupMove: { revision: 3, dx: -1125, dy: 2227 } })
    const result = await pending
    expect(result.groupMove).toEqual({ revision: 3, dx: -1125, dy: 2227 })
    expect(Object.isFrozen(result.groupMove)).toBe(true)
  })
  it.each(['missing', 'revision', 'unsafe', 'surplus', 'wrong-operation'])('refuses malformed group evidence: %s', async (kind) => {
    const worker = new FakeWorker(); const client = new EngineClient(worker); worker.ready()
    const pending = client.request(kind === 'wrong-operation' ? 'snapshot' : 'group-move-preview', kind === 'wrong-operation' ? undefined : new Uint8Array([1]).buffer)
    const groupMove = { revision: kind === 'revision' ? 2 : 3, dx: kind === 'unsafe' ? Number.MAX_SAFE_INTEGER + 1 : 1, dy: 2, ...(kind === 'surplus' ? { arbitrary: true } : {}) }
    worker.emit({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'request-1', ok: true, snapshot: { documentState: 'loaded', revision: 3, byteLength: 10 }, ...(kind === 'missing' ? {} : { groupMove }) })
    await expect(pending).rejects.toMatchObject({ code: kind === 'missing' || kind === 'wrong-operation' ? 'PROTOCOL_OPERATION_MISMATCH' : 'PROTOCOL_INVALID' })
  })
})

// ---------------------------------------------------------------------------
// THE ABSENT-FACE RETRY (spec-deferred-offline-cache, story 5).
//
// The transport answers exactly ONE refusal code with something other than a
// rejection, and every property below is about the boundary of that exception
// rather than about fonts: which request is sent again, how many times, and
// what the caller is told when the recovery cannot help.
// ---------------------------------------------------------------------------
describe('the absent-face retry', () => {
  const refusalCode = 'TEXT_FACE_ABSENT'

  it('resends the refused request once, after the recovery reports a face was supplied', async () => {
    const worker = new FakeWorker()
    const client = new EngineClient(worker)
    worker.ready()
    const recovered: string[] = []
    client.onAbsentFace(async (error) => { recovered.push(error.message); return true })
    const pending = client.request('load', new Uint8Array([7, 7, 7]).buffer)
    expect(worker.sent).toHaveLength(1)
    worker.refuse('request-1', refusalCode, 'face "Noto Sans SC" is not present in the supplied FontSet')
    await vi.waitFor(() => expect(worker.sent).toHaveLength(2))
    expect(recovered).toEqual(['face "Noto Sans SC" is not present in the supplied FontSet'])
    // THE SAME OPERATION AND THE SAME BYTES. The first copy was TRANSFERRED
    // and is detached; a retry that resent it would send an empty buffer.
    expect(worker.sent[1].operation).toBe('load')
    expect(new Uint8Array(worker.sent[1].payload as ArrayBuffer)).toEqual(new Uint8Array([7, 7, 7]))
    worker.respond('request-2', 5)
    await expect(pending).resolves.toMatchObject({ snapshot: { revision: 5 } })
  })

  it('surfaces the second refusal rather than recovering for ever', async () => {
    const worker = new FakeWorker()
    const client = new EngineClient(worker)
    worker.ready()
    let attempts = 0
    client.onAbsentFace(async () => { attempts++; return true })
    const pending = client.request('load', new Uint8Array([1]).buffer)
    worker.refuse('request-1', refusalCode, 'face "Noto Sans SC" is not present in the supplied FontSet')
    await vi.waitFor(() => expect(worker.sent).toHaveLength(2))
    worker.refuse('request-2', refusalCode, 'face "Noto Sans SC" is not present in the supplied FontSet')
    await expect(pending).rejects.toMatchObject({ code: refusalCode })
    expect(attempts, 'one recovery per request, whatever the recovery claims').toBe(1)
    expect(worker.sent).toHaveLength(2)
  })

  it('reports the ORIGINAL refusal when the recovery could not supply the face', async () => {
    const worker = new FakeWorker()
    const client = new EngineClient(worker)
    worker.ready()
    client.onAbsentFace(async () => false)
    const pending = client.request('load', new Uint8Array([1]).buffer)
    worker.refuse('request-1', refusalCode, 'face "Noto Sans SC" is not present in the supplied FontSet')
    await expect(pending).rejects.toMatchObject({ code: refusalCode, message: 'face "Noto Sans SC" is not present in the supplied FontSet' })
    expect(worker.sent, 'a recovery that supplied nothing must not resend anything').toHaveLength(1)
  })

  it('reports the original refusal when the recovery itself throws', async () => {
    const worker = new FakeWorker()
    const client = new EngineClient(worker)
    worker.ready()
    client.onAbsentFace(async () => { throw new Error('the network is gone') })
    const pending = client.request('load', new Uint8Array([1]).buffer)
    worker.refuse('request-1', refusalCode, 'face "Noto Sans SC" is not present in the supplied FontSet')
    await expect(pending, 'the author is told which face is missing, never what the recovery tripped over').rejects.toMatchObject({ code: refusalCode })
  })

  // ⚠ THE RECOVERY'S OWN REQUEST MUST NEVER RECURSE THROUGH IT. `install-face`
  // is what the recovery issues; a refusal of it that re-entered the recovery
  // would be a loop with an await in the middle.
  it('never recovers an install-face refusal', async () => {
    const worker = new FakeWorker()
    const client = new EngineClient(worker)
    worker.ready()
    let attempts = 0
    client.onAbsentFace(async () => { attempts++; return true })
    const pending = client.request('install-face', { face: 'Noto Sans SC', bytes: new Uint8Array([1, 2]).buffer })
    expect(worker.transfers[0], 'the face bytes must be TRANSFERRED, never structurally cloned').toHaveLength(1)
    worker.refuse('request-1', refusalCode, 'face "Noto Sans SC" is not present in the supplied FontSet')
    await expect(pending).rejects.toMatchObject({ code: refusalCode })
    expect(attempts).toBe(0)
  })

  // Every other refusal is a refusal, and this is the control that says so.
  it('leaves every other refusal exactly as it was', async () => {
    const worker = new FakeWorker()
    const client = new EngineClient(worker)
    worker.ready()
    let attempts = 0
    client.onAbsentFace(async () => { attempts++; return true })
    const pending = client.request('load', new Uint8Array([1]).buffer)
    worker.refuse('request-1', 'TEXT_MISSING_GLYPH', 'no supplied face covers U+6C49')
    await expect(pending).rejects.toMatchObject({ code: 'TEXT_MISSING_GLYPH' })
    expect(attempts).toBe(0)
  })

  // And with no recovery installed the client behaves exactly as it always did,
  // which is what makes the focused unit suites and the dev server unaffected.
  it('rejects as before when no recovery is installed', async () => {
    const worker = new FakeWorker()
    const client = new EngineClient(worker)
    worker.ready()
    const pending = client.request('load', new Uint8Array([1]).buffer)
    worker.refuse('request-1', refusalCode, 'face "Noto Sans SC" is not present in the supplied FontSet')
    await expect(pending).rejects.toMatchObject({ code: refusalCode })
    expect(worker.sent).toHaveLength(1)
  })
})
