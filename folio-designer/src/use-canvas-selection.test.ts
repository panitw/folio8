import { act, renderHook } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { EngineClient, EngineResult } from './engine-client'
import type { CanvasProjection } from './engine-protocol'
import { useCanvasSelection } from './use-canvas-selection'
import { canvasDisplay } from './App'

const canvas: CanvasProjection = {
  orientation: 'portrait', preset: 'A4', locale: 'en', utcOffset: '+00:00', marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, gridIncrement: 6000, commandWidth: 600000, commandHeight: 800000, fontFamilies: [], fontChains: [], defaultFontSize: 12000, defaultLineSpacing: 1000,
  width: 600000, height: 800000, contentWindowHeight: 600000, contentWindowOrigins: [0], contentWindowPages: [0], contentWindowCount: 1, contentWindowCountIsExact: true,
  bands: [{ name: 'pageHeader', x: 0, y: 0, width: 600000, height: 100000 }, { name: 'content', x: 0, y: 100000, width: 600000, height: 600000 }, { name: 'pageFooter', x: 0, y: 700000, width: 600000, height: 100000 }],
  components: [{ id: 'e1', type: 'rect', band: 'content', x: 10000, y: 10000, width: 20000, height: 20000, resizable: true }, { id: 'e2', type: 'rect', band: 'content', x: 60000, y: 10000, width: 20000, height: 20000, resizable: true }],
}
const input = (x = 0, y = 0) => ({ pointerId: 1, clientX: x, clientY: y })
function harness() {
  const pending: { resolve: (result: EngineResult) => void; reject: (error: Error) => void; payload: ArrayBuffer }[] = []
  const request = vi.fn((_op: string, payload: ArrayBuffer) => new Promise<EngineResult>((resolve, reject) => pending.push({ resolve, reject, payload })))
  const props = { engine: { request } as unknown as EngineClient, canvas, revision: 1, generation: 1, zoom: 1, selection: ['e1', 'e2'], enabled: true, snap: true, documentDelta: canvasDisplay.documentDelta, capture: vi.fn(), release: vi.fn(), onSelection: vi.fn(), onCommit: vi.fn(async (_payload: ArrayBuffer) => {}), onError: vi.fn() }
  const view = renderHook((context) => useCanvasSelection(context), { initialProps: props })
  const reply = async (index: number, dx: number, dy = 0, revision = 1) => act(async () => { pending[index]!.resolve({ snapshot: { documentState: 'loaded', revision, byteLength: 1 }, groupMove: { revision, dx, dy } }) })
  return { ...view, props, pending, request, reply }
}

describe('captured group lifecycle (G-5, G-9, AC-13, AC-14)', () => {
  it('paints successive accepted replies while coalescing and awaits the final release position', async () => {
    const h = harness()
    act(() => h.result.current.beginGroup(input(), ['e1', 'e2'], 'e1'))
    act(() => h.result.current.move(input(10)))
    act(() => h.result.current.move(input(20)))
    expect(h.request).toHaveBeenCalledTimes(1)
    await h.reply(0, 10000)
    expect(h.request).toHaveBeenCalledTimes(2)
    expect(h.result.current.group?.dx).toBe(10000)
    act(() => h.result.current.finish(input(30)))
    act(() => h.result.current.lostCapture())
    expect(h.props.onCommit).not.toHaveBeenCalled()
    await h.reply(1, 20000)
    expect(h.request).toHaveBeenCalledTimes(3)
    expect(h.result.current.group?.dx).toBe(20000)
    await h.reply(2, 30000)
    expect(h.props.onCommit).toHaveBeenCalledTimes(1)
    expect(JSON.parse(new TextDecoder().decode(h.props.onCommit.mock.calls[0]![0]))).toMatchObject({ ids: ['e1', 'e2'], referenceId: 'e1', dx: 30, dy: 0, expectedRevision: 1 })
    expect(h.result.current.consumeClick()).toBe(true)
    expect(h.result.current.group).toBeUndefined()
  })
  it('keeps the accepted preview and blocks a new gesture until the commit settles', async () => {
    const h = harness()
    let settle!: () => void
    h.props.onCommit.mockImplementation(() => new Promise<void>((resolve) => { settle = resolve }))
    act(() => h.result.current.beginGroup(input(), ['e1', 'e2'], 'e1'))
    act(() => h.result.current.finish(input(10)))
    await h.reply(0, 10000)
    expect(h.result.current.group?.dx).toBe(10000)
    expect(h.result.current.blocksPointer()).toBe(true)
    act(() => window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })))
    act(() => window.dispatchEvent(new Event('scroll')))
    expect(h.result.current.group?.dx).toBe(10000)
    act(() => h.result.current.beginRectangle(input(), { x: 0, y: 0 }, false))
    act(() => h.result.current.beginGroup(input(), ['e1', 'e2'], 'e2'))
    expect(h.result.current.group?.dx).toBe(10000)
    expect(h.props.onCommit).toHaveBeenCalledTimes(1)
    await act(async () => settle())
    expect(h.result.current.group).toBeUndefined()
    expect(h.result.current.blocksPointer()).toBe(false)
  })
  it('sends direct pointer travel with current-window constraints even across page chrome', () => {
    const h = harness()
    h.rerender({ ...h.props, snap: false, canvas: { ...canvas, contentWindowOrigins: [0, 600000, 1200000], contentWindowPages: [0, 0, 0], contentWindowCount: 3 } })
    // Press at column 1,190pt near page two's foot, then travel 140px into
    // page three. Go limits direct displacement to the starting window.
    act(() => h.result.current.beginGroup(input(), ['e1', 'e2'], 'e1'))
    act(() => h.result.current.move(input(0, 140)))
    expect(JSON.parse(new TextDecoder().decode(h.pending[0]!.payload))).toMatchObject({ dy: 140, constrainToWindow: true })
  })
  it.each(['selection', 'document', 'geometry', 'zoom', 'mode', 'capture', 'escape', 'scroll'].flatMap((change) => (change === 'capture' ? ['active'] : ['active', 'released']).map((phase) => [change, phase])))('cancels without accepting stale work on %s while %s', async (change, phase) => {
    const h = harness()
    act(() => h.result.current.beginGroup(input(), ['e1', 'e2'], 'e1'))
    act(() => h.result.current.move(input(10)))
    if (phase === 'released') act(() => h.result.current.finish(input(10)))
    if (change === 'selection') h.rerender({ ...h.props, selection: ['e2'] })
    else if (change === 'document') h.rerender({ ...h.props, generation: 2 })
    else if (change === 'geometry') h.rerender({ ...h.props, canvas: { ...canvas }, revision: 2 })
    else if (change === 'zoom') h.rerender({ ...h.props, zoom: 1.5 })
    else if (change === 'mode') h.rerender({ ...h.props, enabled: false })
    else if (change === 'capture') act(() => h.result.current.lostCapture())
    else if (change === 'escape') act(() => window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })))
    else act(() => window.dispatchEvent(new Event('scroll')))
    await h.reply(0, 10000)
    act(() => h.result.current.finish(input(10)))
    expect(h.props.onCommit).not.toHaveBeenCalled()
    expect(h.props.onSelection).not.toHaveBeenCalled()
    expect(h.result.current.group).toBeUndefined()
  })
  it('returns to zero without a command and does not swallow a later fresh click', async () => {
    const h = harness()
    act(() => h.result.current.beginGroup(input(), ['e1', 'e2'], 'e1'))
    act(() => h.result.current.move(input(10)))
    await h.reply(0, 10000)
    act(() => h.result.current.finish(input()))
    await h.reply(1, 0)
    expect(h.props.onCommit).not.toHaveBeenCalled()
    act(() => h.result.current.freshPointer())
    expect(h.result.current.consumeClick()).toBe(false)
  })
  it('reports a current refusal and discards a reply from a different revision', async () => {
    const h = harness()
    act(() => h.result.current.beginGroup(input(), ['e1', 'e2'], 'e1'))
    act(() => h.result.current.move(input(10)))
    await act(async () => h.pending[0]!.reject(new Error('e2 is missing')))
    expect(h.props.onError).toHaveBeenCalledTimes(1)
    act(() => h.result.current.finish(input(10)))
    expect(h.result.current.consumeClick()).toBe(true)
    expect(h.props.onSelection).not.toHaveBeenCalled()
    act(() => h.result.current.beginGroup(input(), ['e1', 'e2'], 'e1'))
    act(() => h.result.current.finish(input(10)))
    await h.reply(1, 10000, 0, 2)
    expect(h.props.onCommit).not.toHaveBeenCalled()
  })
})

describe('rectangle ownership', () => {
  it('preserves a backdrop click but lets a backdrop drag replace the selection', () => {
    const h = harness()
    act(() => h.result.current.beginRectangle(input(), { x: 0, y: 100000 }, false, false))
    act(() => h.result.current.finish(input()))
    expect(h.props.onSelection).not.toHaveBeenCalled()
    act(() => h.result.current.beginRectangle(input(), { x: 0, y: 100000 }, false, false))
    act(() => h.result.current.finish(input(40, 40)))
    expect(h.props.onSelection).toHaveBeenCalledWith(['e1'])
  })
  it.each(['escape', 'capture', 'zoom', 'mode', 'geometry', 'document', 'scroll'])('cancels rectangle geometry on %s', (change) => {
    const h = harness()
    act(() => h.result.current.beginRectangle(input(), { x: 0, y: 100000 }, false))
    act(() => h.result.current.move(input(40, 40)))
    if (change === 'zoom') h.rerender({ ...h.props, zoom: 1.5 })
    else if (change === 'capture') act(() => h.result.current.lostCapture())
    else if (change === 'mode') h.rerender({ ...h.props, enabled: false })
    else if (change === 'geometry') h.rerender({ ...h.props, canvas: { ...canvas }, revision: 2 })
    else if (change === 'document') h.rerender({ ...h.props, generation: 2 })
    else if (change === 'scroll') act(() => window.dispatchEvent(new Event('scroll')))
    else act(() => window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })))
    act(() => h.result.current.finish(input(40, 40)))
    expect(h.props.onSelection).not.toHaveBeenCalled()
    expect(h.result.current.rectangle).toBeUndefined()
  })
  it('replaces or unions once on release, and cancellation keeps starting selection', () => {
    const h = harness()
    act(() => h.result.current.beginRectangle(input(), { x: 0, y: 100000 }, false))
    act(() => h.result.current.finish(input(40, 40)))
    expect(h.props.onSelection).toHaveBeenLastCalledWith(['e1'])
    expect(h.props.onCommit).not.toHaveBeenCalled()
    act(() => h.result.current.beginRectangle(input(), { x: 0, y: 100000 }, true))
    act(() => h.result.current.finish(input(40, 40)))
    expect(h.props.onSelection).toHaveBeenLastCalledWith(['e1', 'e2'])
    h.props.onSelection.mockClear()
    act(() => h.result.current.beginRectangle(input(), { x: 0, y: 100000 }, false))
    act(() => h.result.current.move(input(40, 40)))
    act(() => h.result.current.cancel())
    expect(h.props.onSelection).not.toHaveBeenCalled()
  })
})

// SPEC-multi-pages story 3: two one-sheet pages. Pitch at zoom 1 is 824pt; the
// content band runs 100pt to 700pt down each sheet.
describe('moving a selection to another page (story 3)', () => {
  const twoPages: CanvasProjection = { ...canvas, contentWindowOrigins: [0, 0], contentWindowPages: [0, 1], contentWindowCount: 2, components: [{ ...canvas.components[0]!, page: 0 }, { ...canvas.components[1]!, page: 0 }, { id: 'e3', type: 'rect', band: 'content', x: 10000, y: 10000, width: 20000, height: 20000, resizable: true, page: 1 }] }
  const decode = (payload: ArrayBuffer) => JSON.parse(new TextDecoder().decode(payload))
  it('targets the page under the pointer, measured in that page column, and draws the preview there', async () => {
    const h = harness()
    h.rerender({ ...h.props, snap: false, canvas: twoPages })
    // Pressed on e1's top at page 1's column y 10pt: stack y 110pt.
    act(() => h.result.current.beginGroup(input(), ['e1', 'e2'], 'e1', 110000))
    // 1,014px down: stack y 1,124pt is page 2's sheet at column y 200pt.
    act(() => h.result.current.move(input(0, 1014)))
    expect(decode(h.pending[0]!.payload)).toMatchObject({ ids: ['e1', 'e2'], dy: 190, constrainToWindow: true, page: 1 })
    await h.reply(0, 0, 190000)
    expect(h.result.current.group).toEqual({ ids: ['e1', 'e2'], dx: 0, dy: 190000, page: 1 })
    act(() => h.result.current.finish(input(0, 1014)))
    await act(async () => {})
    expect(decode(h.props.onCommit.mock.calls[0]![0])).toMatchObject({ kind: 'moveComponents', dy: 190, page: 1 })
  })
  it('commits a move to another page even when its delta is zero', async () => {
    const h = harness()
    h.rerender({ ...h.props, snap: false, canvas: twoPages })
    act(() => h.result.current.beginGroup(input(), ['e1', 'e2'], 'e1', 110000))
    act(() => h.result.current.move(input(0, 824)))
    await h.reply(0, 0, 0)
    act(() => h.result.current.finish(input(0, 824)))
    await act(async () => {})
    expect(decode(h.props.onCommit.mock.calls[0]![0])).toMatchObject({ dx: 0, dy: 0, page: 1 })
  })
  it('keeps today bytes over its own page, a page gap, or for a selection spanning pages', () => {
    for (const [ids, travel] of [[['e1', 'e2'], 100], [['e1', 'e2'], 650], [['e1', 'e3'], 1014]] as const) {
      const h = harness()
      h.rerender({ ...h.props, snap: false, canvas: twoPages, selection: [...ids] })
      act(() => h.result.current.beginGroup(input(), ids, 'e1', 110000))
      act(() => h.result.current.move(input(0, travel)))
      expect(decode(h.pending[0]!.payload)).not.toHaveProperty('page')
      expect(decode(h.pending[0]!.payload)).toMatchObject({ dy: travel, constrainToWindow: true })
      h.unmount()
    }
  })
})
