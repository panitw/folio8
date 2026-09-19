// A DOCUMENT'S DEFERRED CANVAS FACES, ESTABLISHED AND FETCHED ON OPEN
// (spec-deferred-offline-cache, story 3 — CAP-3).
//
// THE MAP UNDER TEST IS THE REAL GENERATED ONE, not a fixture: `canvasFaceAssets`
// is parsed out of the stylesheet `build-wasm.mjs` emits, and a fixture here
// would pass over exactly the drift that parse exists to prevent. So the
// families named below are the families `runtime-fonts.css` actually declares,
// and their URLs are read from the map rather than typed.
import { describe, expect, it, vi } from 'vitest'
import { canvasFaceAssets } from './generated/canvas-face-assets'
import { deferredFaceAssets, paintedCanvasFaces, prefetchDeferredFaces } from './document-face-prefetch'
import type { CanvasProjection } from './engine-protocol'
import { tieredCanvasFacePayload } from './test/tiered-payload'

type Fragment = Readonly<{ text: string; x: number; face?: string; assetKey?: string }>
// A projection carrying one text component whose paint report names these
// fragments — the engine's own statement of which face it measured each run
// with, which is the only input this module reads.
const painted = (...fragments: ReadonlyArray<Fragment>) => ({
  components: [{ id: 'a', type: 'text', textPaint: { overflow: false, truncated: false, lines: [{ top: 0, baseline: 10, advance: 10, width: 10, fragments }] } }],
  // A CHAIN THAT REACHES FURTHER THAN THE PAINT DOES, in every fixture, because
  // that gap is the behaviour under test: the bundled examples all end theirs in
  // `Noto Sans SC` and none of their Latin text ever reaches it.
  fontChains: [{ name: 'body', entries: [{ face: 'Noto Sans', assetKey: '', family: '', style: '', bold: '', italic: '', boldItalic: '' }, { face: 'Noto Sans SC', assetKey: '', family: '', style: '', bold: '', italic: '', boldItalic: '' }] }],
}) as unknown as CanvasProjection

const url = (family: string) => {
  const found = canvasFaceAssets.get(family)
  if (found === undefined) throw new Error(`runtime-fonts.css declares no rule for ${family}, so this test is asserting over a family the canvas cannot ask for`)
  return found
}

const payloadTiering = tieredCanvasFacePayload

describe('the faces a document paints in', () => {
  it('reads the face the engine measured each fragment with', () => {
    expect(paintedCanvasFaces(painted({ text: 'a', x: 0, face: 'Roboto' }, { text: 'b', x: 5, face: 'Roboto Bold' }))).toEqual(new Set(['Roboto', 'Roboto Bold']))
  })

  // AD-8: a fragment carries a face name OR a carried asset's key, and the two
  // are different namespaces. A carried face arrived with the document and needs
  // no fetch; asking the canvas face map about its key would be a question in
  // the wrong vocabulary, answered `undefined` today and collidable tomorrow.
  it('reads nothing off a carried fragment', () => {
    expect(paintedCanvasFaces(painted({ text: 'a', x: 0, assetKey: 'Roboto' }))).toEqual(new Set())
  })

  it('has nothing to say about a projection that is not there', () => {
    expect(paintedCanvasFaces(undefined)).toEqual(new Set())
  })
})

describe('the deferred assets an open would fetch', () => {
  it('names the deferred faces the document paints in and no others', () => {
    const payload = payloadTiering(['Noto Sans Thai', 'Lora', 'Inter'])
    expect(deferredFaceAssets(painted({ text: 'a', x: 0, face: 'Noto Sans' }, { text: 'b', x: 5, face: 'Noto Sans Thai' }, { text: 'c', x: 9, face: 'Lora' }), payload)).toEqual([url('Noto Sans Thai'), url('Lora')])
  })

  // THE BOUND THE SPEC WRITES: the faces THAT document needs. Not a warm-up of
  // the catalogue, and not a top-up of anything — `Inter` is deferred in this
  // payload and is not asked for, because nothing in the document paints in it.
  it('asks for nothing a document does not paint in', () => {
    expect(deferredFaceAssets(painted({ text: 'a', x: 0, face: 'Noto Sans' }), payloadTiering(['Inter', 'Lora']))).toEqual([])
  })

  // ⚠ THE ONE THAT COSTS 4.72 MiB. Every fixture above carries `Noto Sans SC`
  // in its CHAIN as the CJK fallback, exactly as the starter and all four
  // bundled examples do, and no fixture's text is ever measured in it. A
  // chain-wide prefetch would hold every first online open for the whole CJK
  // face to draw Latin invoices containing no CJK codepoint.
  it('leaves an unreached fallback tail to the lazy path, however large', () => {
    expect(deferredFaceAssets(painted({ text: 'a', x: 0, face: 'Noto Sans' }), payloadTiering(['Noto Sans SC']))).toEqual([])
  })

  // The move `catalogue-roboto` makes at this story, asserted as tier rather
  // than as a name: a CORE face is already precached and verified, so asking for
  // it would be a request the first load has already paid for.
  it('leaves a core face alone even when the document paints in it', () => {
    expect(deferredFaceAssets(painted({ text: 'a', x: 0, face: 'Roboto' }), payloadTiering(['Lora']))).toEqual([])
    expect(deferredFaceAssets(painted({ text: 'a', x: 0, face: 'Roboto' }), payloadTiering(['Roboto']))).toEqual([url('Roboto')])
  })

  // A page with no payload is a page with no release — `vite dev` and the unit
  // suites — so nothing is deferred and there is nothing to prefetch.
  it('prefetches nothing where there is no release', () => {
    expect(deferredFaceAssets(painted({ text: 'a', x: 0, face: 'Noto Sans SC' }), undefined)).toEqual([])
  })
})

describe('the prefetch itself', () => {
  const ok = { ok: true, status: 200, arrayBuffer: async () => new ArrayBuffer(4) } as Response

  it('fetches each URL once, same-origin and without credentials', async () => {
    const request = vi.fn(async (_url: string, _init?: RequestInit) => ok)
    await prefetchDeferredFaces(['/assets/a.ttf', '/assets/b.ttf'], 20_000, request as unknown as typeof fetch)
    expect(request.mock.calls.map(([target]) => target)).toEqual(['/assets/a.ttf', '/assets/b.ttf'])
    expect(request.mock.calls.every(([, init]) => init?.credentials === 'omit')).toBe(true)
  })

  // NEVER A REFUSAL, WHICH IS THE OWNER DECISION THIS WHOLE MODULE RESTS ON. A
  // canvas face is not the document's own bytes: the engine holds its own
  // embedded copies, so layout, pagination, preview and the PDF are exact, and
  // story 2's dismissible substitution warning is the settled answer for the
  // glyphs. A rejected fetch and a 404 must both leave the open alone.
  it.each([
    ['a rejected fetch', vi.fn(async () => { throw new TypeError('Failed to fetch') })],
    ['a refused response', vi.fn(async () => ({ ok: false, status: 404, arrayBuffer: async () => new ArrayBuffer(0) }))],
  ])('resolves through %s rather than failing the open', async (_name, request) => {
    await expect(prefetchDeferredFaces(['/assets/a.ttf'], 20_000, request as unknown as typeof fetch)).resolves.toBeUndefined()
  })

  // An offline browser is not asked: every request would fail after its own
  // retries while an author waited on a document that opens correctly anyway.
  it('asks for nothing while the browser reports itself offline', async () => {
    const request = vi.fn(async (_url: string, _init?: RequestInit) => ok)
    const onLine = Object.getOwnPropertyDescriptor(navigator, 'onLine')
    Object.defineProperty(navigator, 'onLine', { configurable: true, value: false })
    try { await prefetchDeferredFaces(['/assets/a.ttf'], 20_000, request as unknown as typeof fetch) } finally {
      if (onLine) Object.defineProperty(navigator, 'onLine', onLine); else Reflect.deleteProperty(navigator, 'onLine')
    }
    expect(request).not.toHaveBeenCalled()
  })

  it('gives up on a fetch that never settles rather than holding the open for ever', async () => {
    vi.useFakeTimers()
    try {
      const request = vi.fn((_url: string, init?: RequestInit) => new Promise<Response>((_, reject) => { init?.signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError'))) }))
      const settled = prefetchDeferredFaces(['/assets/a.ttf'], 20_000, request as unknown as typeof fetch)
      await vi.advanceTimersByTimeAsync(20_000)
      await expect(settled).resolves.toBeUndefined()
    } finally { vi.useRealTimers() }
  })
})
