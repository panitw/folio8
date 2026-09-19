import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { ENGINE_PROTOCOL_VERSION } from './engine-protocol'

// STORY 16.0 — THE BOUNDARY MUST SAY WHAT THREW.
//
// `execute`'s catch used to report ONE sentence, "The engine returned an
// invalid response", for every throw it saw, and keep none of them. That is
// the second defect this story fixes, and it is a real defect independent of
// the first: it fires exactly when nobody anticipated the failure, and it is
// exactly then that it erases the only evidence there is.
//
// This is a BEHAVIOURAL test of the real `engine.worker.ts` module, not a
// source scan. It boots the worker against a stub host, forces one throw per
// boundary stage, and reads what the worker posts back. It reds if any of
// those messages is reverted to the bare string, and it reds if the stages
// stop being distinguishable.
//
// The `undefined` case below is not invented: it is the ACTUAL production
// fault this story diagnosed. `Folio8WasmHost.handle` returned `undefined`
// because Go's wasm runtime had fatally OOMed inside the command and the
// program had exited, and `JSON.parse(undefined)` is what the designer's user
// saw as "the engine returned an invalid response".

type Posted = { requestId?: string; ok?: boolean; error?: { code: string; message: string }; kind?: string; preview?: Record<string, unknown> }

const hostStub: { handle: (request: string) => string } = { handle: () => '' }
const posted: Posted[] = []

// The glue module the worker dynamic-imports on the DEV branch. A data: URL
// keeps the whole boot in this file rather than adding a fixture module to
// src/, which the ownership-contract scan reads as production source.
const glueModule = 'data:text/javascript,'
  + encodeURIComponent('globalThis.Go = class { constructor() { this.importObject = {} } run() { globalThis.Folio8WasmHost = globalThis.__folio8BoundaryTestHost } }')

vi.mock('./generated/offline-assets', () => ({
  runtimeAssetUrls: { wasmExec: glueModule, wasm: 'about:blank', starter: '', sans: '', sansCjk: '', sansThai: '', mono: '', plexSans: '', plexSansThai: '' },
}))

function request(operation: string, payload?: ArrayBuffer) {
  return { protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'request', requestId: `r-${posted.length}-${Math.random().toString(36).slice(2)}`, operation, ...(payload !== undefined ? { payload } : {}) }
}

// One round trip through the real worker: post a request, wait for the
// response carrying that id.
async function roundTrip(operation: string, payload?: ArrayBuffer): Promise<Posted> {
  const message = request(operation, payload)
  ;(self as unknown as { onmessage: (event: { data: unknown }) => void }).onmessage({ data: message })
  for (let tick = 0; tick < 500; tick++) {
    const hit = posted.find((entry) => entry.requestId === message.requestId)
    if (hit) return hit
    await new Promise((resolve) => setTimeout(resolve, 1))
  }
  throw new Error(`the worker never answered ${message.requestId}`)
}

beforeAll(async () => {
  ;(globalThis as unknown as { __folio8BoundaryTestHost: unknown }).__folio8BoundaryTestHost = hostStub
  vi.stubGlobal('fetch', async () => new Response(new Uint8Array()))
  vi.stubGlobal('WebAssembly', { ...WebAssembly, instantiateStreaming: async () => ({ instance: {} as WebAssembly.Instance, module: {} as WebAssembly.Module }) })
  self.postMessage = ((message: Posted) => { posted.push(message) }) as typeof self.postMessage
  await import('./engine.worker')
  // Let boot() settle before the first request is queued.
  for (let tick = 0; tick < 200 && !posted.some((entry) => entry.kind === 'lifecycle'); tick++) await new Promise((resolve) => setTimeout(resolve, 1))
  expect(posted.some((entry) => entry.kind === 'lifecycle')).toBe(true)
})

afterEach(() => { hostStub.handle = () => '' })

describe('the WASM boundary reports what threw', () => {
  // THE PRODUCTION FAULT, EXACTLY. Go's runtime exited mid-command, so
  // `handle` returned undefined and JSON.parse threw. Before this story the
  // author was told only that something was invalid.
  it('names a host response that is not JSON at all, rather than only that it was invalid', async () => {
    hostStub.handle = () => undefined as unknown as string
    const answer = await roundTrip('snapshot')
    expect(answer.ok).toBe(false)
    expect(answer.error?.code).toBe('WASM_RESPONSE_UNPARSABLE')
    // The stage sentence AND the thrown value's own words. Reverting either
    // half reds this.
    expect(answer.error?.message).toContain("The engine's response could not be parsed")
    expect(answer.error?.message).toContain('SyntaxError')
    expect(answer.error?.message).not.toBe('The engine returned an invalid response')
  })

  // THE REQUEST SIDE, PINNED. Mutation-proved at review: flipping `execute`'s
  // initial `let stage: BoundaryStage = 'request'` to `'reply'` left the other
  // four cases green, because every one of them reassigns `stage` before it
  // throws. The request-encoding arm is the ONLY one that reads the initial
  // value, so without this case it could silently regress to the pre-story
  // generic sentence — which is precisely the erased-evidence behaviour this
  // story exists to remove.
  //
  // btoa is what bytesToBase64 ends on, and it is the boundary's one genuinely
  // throwable request-side step (it raises InvalidCharacterError on a code
  // point over 255). Forcing it here forces the stage, not a rewritten
  // encoder.
  it('names a failure encoding the request, distinctly from anything the host did', async () => {
    const realBtoa = globalThis.btoa
    // The stub throws a plain Error carrying btoa's real name and words
    // rather than a DOMException. In a browser they are the same thing to
    // describeThrow — DOMException.prototype inherits from Error.prototype —
    // but under vitest the two live in different realms, so `instanceof Error`
    // is false and the assertion would be measuring the test harness's realms
    // instead of the boundary's reporting.
    vi.stubGlobal('btoa', () => {
      const failure = new Error('the string to be encoded contains invalid characters')
      failure.name = 'InvalidCharacterError'
      throw failure
    })
    // THE PAYLOAD IS BUILT WITH THIS FILE'S OWN `ArrayBuffer`, not through
    // TextEncoder. `execute` gates the encoder on `payload instanceof
    // ArrayBuffer`, and under vitest TextEncoder hands back a Node-realm
    // buffer that fails that check while still passing parseRequest's
    // realm-agnostic Object.prototype.toString guard — so the request would be
    // admitted, skip the encoder entirely, and this case would pass for the
    // wrong reason.
    // If the request never encoded, the host must never have been asked.
    let asked = false
    hostStub.handle = () => { asked = true; return JSON.stringify({ ok: true, snapshot: snapshotStub() }) }
    try {
      const answer = await roundTrip('command', new ArrayBuffer(16))
      expect(answer.error?.code).toBe('WASM_REQUEST_ENCODING_FAILED')
      expect(answer.error?.message).toBe('The engine request could not be encoded: InvalidCharacterError: the string to be encoded contains invalid characters')
      expect(answer.error?.message).not.toBe('The engine returned an invalid response')
      expect(asked).toBe(false)
    } finally {
      vi.stubGlobal('btoa', realBtoa)
    }
  })

  it('names a host that threw, and distinguishes it from a response that would not parse', async () => {
    hostStub.handle = () => { throw new TypeError('Go program has already exited') }
    const answer = await roundTrip('snapshot')
    expect(answer.error?.code).toBe('WASM_HOST_FAILED')
    expect(answer.error?.message).toBe('The engine threw while handling the request: TypeError: Go program has already exited')
  })

  // A RESPONSE-SIDE TRANSPORT BREACH USED TO BE INDISTINGUISHABLE FROM A
  // REQUEST-SIDE FAULT. base64ToBytesBounded throws a perfectly clear
  // sentence and the bare catch discarded it, so a bound that was working
  // correctly reported the same thing as an unanticipated crash.
  it('distinguishes a response that breaches its transport bound, and keeps that bound a refusal', async () => {
    const oversize = 'A'.repeat(4 * (32 * 1024 * 1024 + 8))
    hostStub.handle = () => JSON.stringify({ ok: true, snapshot: snapshotStub(), bytesBase64: oversize })
    const answer = await roundTrip('render', { template: new ArrayBuffer(1), data: new ArrayBuffer(1), params: new ArrayBuffer(1) } as unknown as ArrayBuffer)
    expect(answer.error?.code).toBe('WASM_RESPONSE_BYTES_REFUSED')
    expect(answer.error?.message).toBe("The engine's byte response was refused: Error: WASM byte response exceeds its transport limit")
  })

  // STORY 13.3 / REVIEW P5 — THE HAPPY PATH, WHICH THIS FILE HAD NONE OF.
  //
  // Everything above forces an error, and the two render facts the worker now
  // carries were defended only by a source-text scan in
  // `engine-protocol.test.ts`. That scan matches key NAMES, so it catches a
  // MISSING field and never a WRONG one: writing
  // `elapsedMs: result.renderRevision` at the hop leaves it, the client test
  // and this file all green, and the browser then prints a revision number in
  // milliseconds. This reads the values back off the message the real worker
  // actually posts, so a cross-wire is a different number and reds.
  //
  // The scan STAYS. It catches what this cannot: which of the two hops dropped
  // a field, by name, in a repository where one hop's output is the other's
  // input.
  it('carries the render facts through unchanged, by value and not merely by key', async () => {
    // Values chosen so no other field on the response could stand in for
    // either: none of the neighbouring numbers is 412, and the version is not
    // a digest, an identity or a revision.
    hostStub.handle = () => JSON.stringify({
      ok: true,
      snapshot: snapshotStub(),
      bytesBase64: btoa('\x09'),
      pdfSha256: 'a'.repeat(64),
      previewIdentity: 'b'.repeat(64),
      renderRevision: 7,
      diagnostics: [],
      elapsedMs: 412,
      version: '0.0.0-dev',
    })
    const answer = await roundTrip('render', { template: new ArrayBuffer(1), data: new ArrayBuffer(1), params: new ArrayBuffer(1) } as unknown as ArrayBuffer)
    expect(answer.ok).toBe(true)
    expect(answer.preview, 'the worker built no preview for a render reply').toBeDefined()
    expect(answer.preview!.elapsedMs, 'elapsedMs did not arrive with the value the engine reported').toBe(412)
    expect(answer.preview!.version, 'version did not arrive with the value the engine reported').toBe('0.0.0-dev')
    // The neighbours it could have been cross-wired to, pinned alongside, so a
    // swap between any two of them is visible here rather than plausible.
    expect(answer.preview!.revision).toBe(7)
    expect(answer.preview!.identity).toBe('b'.repeat(64))
    expect(answer.preview!.pdfSha256).toBe('a'.repeat(64))
  })

  // AN ENGINE REFUSAL IS NOT A BOUNDARY FAULT, and never was. It carries its
  // own diagnosticCode and located message through a different arm, and this
  // story must not have swept one into the other.
  it('leaves a located engine refusal exactly as it was', async () => {
    hostStub.handle = () => JSON.stringify({ ok: false, diagnosticCode: 'COMPONENT_INVALID', message: 'face exceeds the 6288384-byte supported size', dataPath: 'fonts.Noto Sans SC' })
    const answer = await roundTrip('command', new ArrayBuffer(2))
    expect(answer.error?.code).toBe('COMPONENT_INVALID')
    expect(answer.error?.message).toBe('face exceeds the 6288384-byte supported size')
  })
})

// The smallest object isSnapshot admits; the byte-bound test needs the
// response to reach the decode step, which means getting past the refusal arm.
function snapshotStub() {
  return { documentState: 'loaded', revision: 1, byteLength: 0, canUndo: false, canRedo: false }
}
