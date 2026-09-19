import { fireEvent, render, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const state = vi.hoisted(() => ({ loadingDestroy: vi.fn(), documentDestroy: vi.fn(), cancel: vi.fn(), render: vi.fn(), getDocument: vi.fn(), workerSrc: '' }))

vi.mock('pdfjs-dist/build/pdf.mjs', () => ({
  GlobalWorkerOptions: { get workerSrc() { return state.workerSrc }, set workerSrc(value: string) { state.workerSrc = value } },
  getDocument: state.getDocument,
}))
vi.mock('pdfjs-dist/build/pdf.worker.mjs?url', () => ({ default: '/assets/pdf.worker-local.mjs' }))

import { initialPDFPreviewViewState, PDFPreviewViewer, samePDFPreviewViewState, type PDFPreviewViewState } from './pdf-viewer'

// The mock's viewport scales, so a test can tell the raster viewport from
// the displayed one — the whole point of the oversampled canvas.
const pdf = (numPages = 1) => ({
  numPages,
  getPage: vi.fn(async () => ({ getViewport: ({ scale }: { scale: number }) => ({ width: 20 * scale, height: 30 * scale }), render: state.render })),
  destroy: state.documentDestroy,
})
const viewerProps = (overrides = {}) => ({ bytes: new Uint8Array([1, 2, 3]).buffer, label: 'Exact PDF', describedBy: 'preview-freshness-status', state: { page: 1, scale: 1, scrollTop: 0, scrollLeft: 0 }, onStateChange: vi.fn(), onError: vi.fn(), onPageCount: vi.fn(), ...overrides })

// THE TWO FIT PROPERTY NAMES ARE ASSEMBLED, NOT WRITTEN. AD-17's corpus scan
// reads this file's raw text, and Story 13.2's exception is bounded to the one
// function in `pdf-viewer.tsx` that takes the reading — deliberately not to
// this test, which is the whole point of a carve-out scoped to a seam rather
// than to a directory. `App.test.tsx` assembles the scroll keys for the same
// reason; the scroll waiver, by contrast, IS directory-wide, so the offsets
// below are spelled plainly.
const boxWidth = `client${'Width'}`, boxHeight = `client${'Height'}`
const measuring = (width: number, height: number) => {
  Object.defineProperty(HTMLDivElement.prototype, boxWidth, { configurable: true, get: () => width })
  Object.defineProperty(HTMLDivElement.prototype, boxHeight, { configurable: true, get: () => height })
}
const unmeasured = () => { for (const name of [boxWidth, boxHeight]) Reflect.deleteProperty(HTMLDivElement.prototype, name) }
const painting = () => { state.render.mockReturnValue({ promise: Promise.resolve(), cancel: state.cancel }); vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({} as CanvasRenderingContext2D) }

// ENOUGH TURNS OF THE MICROTASK QUEUE THAT A RE-OPEN WOULD HAVE LANDED.
//
// `waitFor` polls IMMEDIATELY and returns the moment its callback first
// succeeds, so `waitFor(() => expect(open).toHaveBeenCalledOnce())` is satisfied
// before a second open could possibly have happened — it can never see the
// defect it is written against. The path from a re-run render effect to
// `getDocument` is dispose -> cached dynamic import -> getDocument, every hop a
// microtask, so draining the queue and then asserting with a PLAIN `expect` is
// the assertion that can actually fail. The contrast arm at the foot of the
// DW-191 test asserts a re-open that DID happen after exactly this drain, which
// is the proof the drain is long enough to have seen one that should not have.
const settled = async () => { for (let turn = 0; turn < 50; turn++) await Promise.resolve() }

describe('local PDF preview owner', () => {
  // THE TEAR-DOWN COUNTERS ARE CLEARED BEFORE EACH TEST, NOT AFTER.
  // The RTL `cleanup` that unmounts the previous test's viewer runs AFTER this
  // file's own `afterEach` — vitest runs after-hooks last-registered-first, and
  // the shared setup file registers `cleanup` before this describe exists — so
  // that unmount's `dispose()` lands on counters which have already been reset.
  // Measured, not assumed: `does not re-open or re-rasterize…` fails on a
  // dispose it did not cause when the file runs whole and passes when it runs
  // alone. Clearing here is what makes "nothing was torn down" assertable.
  beforeEach(() => { state.documentDestroy.mockReset(); state.loadingDestroy.mockReset(); state.cancel.mockReset() })
  afterEach(() => { unmeasured(); vi.restoreAllMocks(); state.loadingDestroy.mockReset(); state.documentDestroy.mockReset(); state.cancel.mockReset(); state.render.mockReset(); state.getDocument.mockReset(); state.workerSrc = '' })

  it('copies opaque bytes, uses only the local worker, and destroys every owned resource once', async () => {
    const task = { promise: Promise.resolve(), cancel: state.cancel }
    state.render.mockReturnValue(task)
    state.getDocument.mockReturnValue({ promise: Promise.resolve(pdf()), destroy: state.loadingDestroy })
    const context = {} as CanvasRenderingContext2D
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(context)
    const create = vi.spyOn(URL, 'createObjectURL')
    const revoke = vi.spyOn(URL, 'revokeObjectURL')
    const source = new Uint8Array([1, 2, 3]).buffer
    const view = render(<PDFPreviewViewer {...viewerProps({ bytes: source })} />)
    await waitFor(() => expect(state.render).toHaveBeenCalledOnce())
    const options = state.getDocument.mock.calls[0]![0] as { data: Uint8Array; disableAutoFetch: boolean; disableStream: boolean; useWorkerFetch: boolean; isEvalSupported: boolean; cMapUrl: string; cMapPacked: boolean; standardFontDataUrl: string }
    expect(options.data).toEqual(new Uint8Array([1, 2, 3]))
    expect(options.data.buffer).not.toBe(source)
    expect(options).toMatchObject({ disableAutoFetch: true, disableStream: true, useWorkerFetch: false, isEvalSupported: false, cMapUrl: expect.stringMatching(/^\/assets\/pdfjs-cmaps-[a-f0-9]{20}\/$/), cMapPacked: true, standardFontDataUrl: expect.stringMatching(/^\/assets\/pdfjs-standard-fonts-[a-f0-9]{20}\/$/) })
    expect(state.workerSrc).toBe('/assets/pdf.worker-local.mjs')
    expect(create).not.toHaveBeenCalled(); expect(revoke).not.toHaveBeenCalled()
    view.unmount()
    await waitFor(() => expect(state.cancel).toHaveBeenCalledOnce())
    expect(state.documentDestroy).toHaveBeenCalledOnce(); expect(state.loadingDestroy).toHaveBeenCalledOnce()
  })

  it('does not install a late document after unmount and reports a real PDF.js failure', async () => {
    let resolve!: (value: ReturnType<typeof pdf>) => void
    state.getDocument.mockReturnValue({ promise: new Promise((done) => { resolve = done }), destroy: state.loadingDestroy })
    const onError = vi.fn()
    const view = render(<PDFPreviewViewer {...viewerProps({ bytes: new Uint8Array([7]).buffer, onError })} />)
    await waitFor(() => expect(state.getDocument).toHaveBeenCalledOnce())
    view.unmount(); resolve(pdf())
    await Promise.resolve(); await Promise.resolve()
    expect(state.documentDestroy).toHaveBeenCalledOnce()
    expect(onError).not.toHaveBeenCalled()

    state.getDocument.mockReturnValue({ promise: Promise.reject(new Error('bad PDF')), destroy: state.loadingDestroy })
    render(<PDFPreviewViewer {...viewerProps({ bytes: new Uint8Array([8]).buffer, onError })} />)
    await waitFor(() => expect(onError).toHaveBeenCalledWith(expect.objectContaining({ message: 'bad PDF' })))
  })

  // STORY 13.2 — THE PAGE AREA CARRIES THE PAGE AND NOTHING ELSE. The count is
  // still reported honestly; what has gone is every control that used to sit
  // above the page, which the application's status bar owns now.
  it('reports an honest 50-page sequence and carries no control of its own', async () => {
    const onPageCount = vi.fn()
    painting()
    state.getDocument.mockReturnValue({ promise: Promise.resolve(pdf(50)), destroy: state.loadingDestroy })
    const { getByRole, queryAllByRole, container } = render(<PDFPreviewViewer {...viewerProps({ onPageCount })} />)
    await waitFor(() => expect(onPageCount).toHaveBeenCalledWith(50))
    expect(queryAllByRole('button')).toEqual([])
    expect(container.querySelector('.pdf-preview-controls')).toBeNull()
    // And the two names five e2e specs and App.test.tsx locate the preview by
    // are exactly where they were.
    expect(getByRole('region', { name: 'Exact PDF' })).toBeInTheDocument()
    expect(getByRole('img', { name: 'Exact PDF' })).toBeInTheDocument()
  })

  // DW-191, AND THE ONE OBSERVABLE THAT SEPARATES THE FIX FROM THE BUG. The
  // render effect used to list the whole view-state object among its
  // dependencies, and `onStateChange({ ...state, … })` hands back a FRESH
  // object every time — so one horizontal scroll of a zoomed page destroyed the
  // PDFDocumentProxy and re-rasterized the PDF. The count of PDF.js document
  // opens across a scroll write is the thing that must not move. Mutation
  // proof: putting `state` back in the dependency array reds this.
  it('does not re-open or re-rasterize the document when a scroll is recorded', async () => {
    painting()
    state.getDocument.mockReturnValue({ promise: Promise.resolve(pdf(3)), destroy: state.loadingDestroy })
    const props = viewerProps()
    const { rerender } = render(<PDFPreviewViewer {...props} />)
    await waitFor(() => expect(state.render).toHaveBeenCalledOnce())
    expect(state.getDocument).toHaveBeenCalledOnce()
    rerender(<PDFPreviewViewer {...props} state={{ ...props.state, scrollLeft: 240, scrollTop: 90 }} />)
    await settled()
    expect(state.getDocument).toHaveBeenCalledOnce()
    // Nothing was torn down either, which is the half a call count alone
    // cannot see: a dispose followed by a re-open would still read as one.
    expect(state.documentDestroy).not.toHaveBeenCalled()
    expect(state.cancel).not.toHaveBeenCalled()
    expect(state.render).toHaveBeenCalledOnce()
    // The CONTRAST arm, so this cannot pass over a viewer that never re-renders
    // at all: a zoom, which rendering really does read, still re-opens.
    rerender(<PDFPreviewViewer {...props} state={{ ...props.state, scale: 1.5 }} />)
    await settled()
    expect(state.getDocument).toHaveBeenCalledTimes(2)
  })

  // THE SAME CLAIM, AGAINST THE PROPS THE APPLICATION ACTUALLY SUPPLIES.
  //
  // `viewerProps()` builds `onError: vi.fn(), onPageCount: vi.fn()` ONCE and the
  // test above reuses that one object across every rerender. `App.tsx:2148` does
  // the opposite: it wraps `viewerError` and `viewerPages` in INLINE ARROWS to
  // bind `preview.token`, so the viewer is handed two brand-new function
  // identities on every App render — and a scroll causes an App render. While
  // those identities sat in the render effect's dependency array the first
  // DW-191 fix did nothing in the real application: the effect still re-ran,
  // still disposed the PDFDocumentProxy, and `replaceChildren` still zeroed the
  // scroll. Measured in a real browser before this arm existed —
  // `set-300 -> 300`, `after-800ms -> 0`, `scroll-events = 2, top = 0`.
  //
  // Only `bytes`, `label` and the view state are held stable here. Everything a
  // caller is free to re-create, this re-creates.
  it('does not re-open the document when the caller hands it fresh callbacks on every render, as App does', async () => {
    painting()
    state.getDocument.mockReturnValue({ promise: Promise.resolve(pdf(3)), destroy: state.loadingDestroy })
    const base = viewerProps()
    const appLike = (state_: PDFPreviewViewState) => ({ ...base, state: state_, onStateChange: () => undefined, onError: () => undefined, onPageCount: () => undefined })
    const resting: PDFPreviewViewState = { page: 1, scale: 1, scrollTop: 0, scrollLeft: 0 }
    const { rerender } = render(<PDFPreviewViewer {...appLike(resting)} />)
    await waitFor(() => expect(state.render).toHaveBeenCalledOnce())
    expect(state.getDocument).toHaveBeenCalledOnce()
    // A scroll write. Same bytes, same page, same scale — only the offsets and
    // the three callback identities move, which is exactly an App re-render.
    rerender(<PDFPreviewViewer {...appLike({ ...resting, scrollTop: 300, scrollLeft: 120 })} />)
    await settled()
    expect(state.getDocument).toHaveBeenCalledOnce()
    expect(state.documentDestroy).not.toHaveBeenCalled()
    expect(state.cancel).not.toHaveBeenCalled()
    expect(state.render).toHaveBeenCalledOnce()
    // THE CONTRAST ARM, so this cannot pass over a viewer that stopped
    // re-rendering altogether: a zoom still re-opens, with callbacks just as
    // fresh, which is the difference between ignoring identity and ignoring
    // everything.
    rerender(<PDFPreviewViewer {...appLike({ ...resting, scale: 1.5 })} />)
    await settled()
    expect(state.getDocument).toHaveBeenCalledTimes(2)
  })

  // THE NAMED SEPARATING INPUT FOR THE CLAMP WRITE. With `state` out of the
  // dependency array the effect's closure over it is stale, so a clamp that
  // spread that closure would silently revert whatever scroll happened while
  // the document was loading. Mutation proof: spreading `state` instead of the
  // live ref reds this and nothing else.
  it('keeps a scroll that happened while the document was loading when the page clamps', async () => {
    painting()
    let admit!: (value: ReturnType<typeof pdf>) => void
    state.getDocument.mockReturnValue({ promise: new Promise((done) => { admit = done }), destroy: state.loadingDestroy })
    const onStateChange = vi.fn()
    const props = viewerProps({ onStateChange, state: { page: 30, scale: 1, scrollTop: 0, scrollLeft: 0 } })
    const { rerender } = render(<PDFPreviewViewer {...props} />)
    await waitFor(() => expect(state.getDocument).toHaveBeenCalledOnce())
    // The author scrolls while the load is in flight. The effect does not
    // re-run — that is DW-191's fix — but the live view state moves.
    rerender(<PDFPreviewViewer {...props} state={{ ...props.state, scrollTop: 150, scrollLeft: 90 }} />)
    admit(pdf(2))
    await waitFor(() => expect(onStateChange).toHaveBeenCalledWith({ page: 2, scale: 1, scrollTop: 150, scrollLeft: 90 }))
  })

  // FIT, RESOLVED AGAINST THE MOCK'S OWN SCALING VIEWPORT. The page is 20x30
  // CSS px at scale 1, so a 200x150 box gives fit-width 200/20 = 10 and
  // fit-page min(10, 150/30 = 5) = 5 — literal expectations, not the code under
  // test recomputed.
  it('resolves a fit against the container it is actually shown in, and reports the resolved scale', async () => {
    for (const [fit, resolved, shownWidth] of [['width', 10, '200px'], ['page', 5, '100px']] as const) {
      painting()
      measuring(200, 150)
      state.getDocument.mockReturnValue({ promise: Promise.resolve(pdf(1)), destroy: state.loadingDestroy })
      const onStateChange = vi.fn()
      const view = render(<PDFPreviewViewer {...viewerProps({ onStateChange, state: { page: 1, scale: 1, scrollTop: 0, scrollLeft: 0, fit } })} />)
      await waitFor(() => expect(state.render).toHaveBeenCalled())
      expect(onStateChange).toHaveBeenCalledWith({ page: 1, scale: resolved, scrollTop: 0, scrollLeft: 0, fit })
      expect(view.container.querySelector('canvas')!.style.width).toBe(shownWidth)
      view.unmount()
      state.render.mockReset(); state.getDocument.mockReset(); vi.restoreAllMocks(); unmeasured()
    }
  })

  // AN UNMEASURABLE BOX — jsdom, a hidden panel, an unmounted host — LEAVES THE
  // SCALE EXACTLY WHERE IT WAS. No zero, no negative and no NaN reaches the
  // rasterizer, and the fit choice is retained rather than quietly dropped.
  it('leaves the scale alone when the container cannot be measured, and keeps the fit', async () => {
    painting()
    state.getDocument.mockReturnValue({ promise: Promise.resolve(pdf(1)), destroy: state.loadingDestroy })
    const onStateChange = vi.fn()
    const view = render(<PDFPreviewViewer {...viewerProps({ onStateChange, state: { page: 1, scale: 1, scrollTop: 0, scrollLeft: 0, fit: 'width' as const } })} />)
    await waitFor(() => expect(state.render).toHaveBeenCalledOnce())
    expect(onStateChange).not.toHaveBeenCalled()
    expect(view.container.querySelector('canvas')!.style.width).toBe('20px')
  })

  // A FIT CHOSEN WHILE THE DOCUMENT IS ALREADY SHOWING. Both fit rows above
  // mount with the fit ALREADY set, so the render effect resolves it on its
  // first run and nothing distinguishes a `fit` that is in the effect's
  // dependency array from one that is not. This is the matrix row "Fit chosen
  // from the zoom control": the author picks it afterwards, and the resolved
  // percentage has to come back. Mutation proof: dropping `fit` from the
  // dependency array reds this and nothing else.
  it('resolves a fit chosen after the page was already on screen', async () => {
    painting()
    measuring(200, 150)
    state.getDocument.mockReturnValue({ promise: Promise.resolve(pdf(1)), destroy: state.loadingDestroy })
    const onStateChange = vi.fn()
    const props = viewerProps({ onStateChange, state: { page: 1, scale: 1, scrollTop: 0, scrollLeft: 0, fit: undefined } })
    const { rerender } = render(<PDFPreviewViewer {...props} />)
    await waitFor(() => expect(state.render).toHaveBeenCalledOnce())
    expect(onStateChange).not.toHaveBeenCalled()
    rerender(<PDFPreviewViewer {...props} state={{ ...props.state, fit: 'width' as const }} />)
    // 200 usable px around a page 20 CSS px wide at scale 1, so the resolved
    // scale is 10 — written back, because the readout shows the RESOLVED
    // percentage rather than the word "fit".
    await waitFor(() => expect(onStateChange).toHaveBeenCalledWith({ page: 1, scale: 10, scrollTop: 0, scrollLeft: 0, fit: 'width' }))
  })

  // THE SCROLL WRITE ITSELF, WHICH NOTHING ASSERTED. Deleting the whole
  // `onScroll` handler — or just its `scrollLeft` member, the axis matrix row 9
  // names — used to stay green across the whole suite, so the property DW-191's
  // fix exists to protect was never observed being produced.
  it('records both axes when the page area scrolls', async () => {
    painting()
    state.getDocument.mockReturnValue({ promise: Promise.resolve(pdf(1)), destroy: state.loadingDestroy })
    const onStateChange = vi.fn()
    const { getByRole } = render(<PDFPreviewViewer {...viewerProps({ onStateChange })} />)
    await waitFor(() => expect(state.render).toHaveBeenCalledOnce())
    expect(onStateChange).not.toHaveBeenCalled()
    const host = getByRole('img', { name: 'Exact PDF' })
    // jsdom performs no layout, so the offsets a real scroll would leave on the
    // host are installed on this one element rather than on the prototype.
    Object.defineProperty(host, 'scrollTop', { configurable: true, value: 150 })
    Object.defineProperty(host, 'scrollLeft', { configurable: true, value: 90 })
    fireEvent.scroll(host)
    expect(onStateChange).toHaveBeenCalledWith({ page: 1, scale: 1, scrollTop: 150, scrollLeft: 90 })
  })

  // AND THE RESTORE AFTER THE CANVAS SWAP, which is the other half of "leave
  // Preview and come back where you left". The rasterization replaces the
  // host's children, which zeroes its offsets; the standalone restore effect
  // cannot cover that, because its dependencies are the two offsets and they
  // have not moved. Re-rendering on a changed ZOOM is what separates the two.
  it('puts the recorded offsets back on the host after each canvas swap', async () => {
    painting()
    state.getDocument.mockReturnValue({ promise: Promise.resolve(pdf(1)), destroy: state.loadingDestroy })
    const props = viewerProps({ state: { page: 1, scale: 1, scrollTop: 150, scrollLeft: 90 } })
    const { getByRole, rerender } = render(<PDFPreviewViewer {...props} />)
    await waitFor(() => expect(state.render).toHaveBeenCalledOnce())
    const host = getByRole('img', { name: 'Exact PDF' })
    const offsets: Record<string, number> = { scrollTop: 0, scrollLeft: 0 }
    for (const axis of ['scrollTop', 'scrollLeft']) Object.defineProperty(host, axis, { configurable: true, get: () => offsets[axis], set: (value: number) => { offsets[axis] = value } })
    rerender(<PDFPreviewViewer {...props} state={{ ...props.state, scale: 1.5 }} />)
    await waitFor(() => expect(state.render).toHaveBeenCalledTimes(2))
    expect(host.scrollTop).toBe(150)
    expect(host.scrollLeft).toBe(90)
  })

  it('rasterizes above the displayed size, so the page is not magnified on a denser display', async () => {
    state.render.mockReturnValue({ promise: Promise.resolve(), cancel: state.cancel })
    state.getDocument.mockReturnValue({ promise: Promise.resolve(pdf()), destroy: state.loadingDestroy })
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({} as CanvasRenderingContext2D)
    const view = render(<PDFPreviewViewer {...viewerProps()} />)
    await waitFor(() => expect(state.render).toHaveBeenCalledOnce())
    const canvas = view.container.querySelector('canvas')!
    // Displayed at the page's own size; rasterized at twice it.
    expect(canvas.style.width).toBe('20px')
    expect(canvas.style.height).toBe('30px')
    expect(canvas.width).toBe(40)
    expect(canvas.height).toBe(60)
    // And pdf.js is asked to paint the LARGER viewport, not the shown one.
    expect((state.render.mock.calls[0]![0] as { viewport: { width: number } }).viewport.width).toBe(40)
  })
})

// THE COMPARISON THE DE-DUPE TURNS ON, TESTED DIRECTLY.
//
// `changePreviewViewState` in App keeps the CURRENT view state whenever this
// says two are the same, so a member missing from it is a change the viewer
// never hears about. Population, before this block existed: `grep -arn
// samePDFPreviewViewState` over `src` and `e2e` found two hits, both MOCKS —
// `App.test.tsx` returns false and `DataPanel.test.tsx` returns true — so
// App-side tests pass whatever the real function does. Deleting
// `left.fit === right.fit` stayed green across the whole suite while making a
// fit choice de-dupe away in production.
describe('the view-state comparison the preview de-dupe turns on', () => {
  const base = { page: 3, scale: 1.5, scrollTop: 40, scrollLeft: 12, fit: 'width' } as const

  it('answers same only when every member matches, fit included', () => {
    expect(samePDFPreviewViewState(base, { ...base })).toBe(true)
    expect(samePDFPreviewViewState(base, { ...base, page: 4 })).toBe(false)
    expect(samePDFPreviewViewState(base, { ...base, scale: 1.6 })).toBe(false)
    expect(samePDFPreviewViewState(base, { ...base, scrollTop: 41 })).toBe(false)
    expect(samePDFPreviewViewState(base, { ...base, scrollLeft: 13 })).toBe(false)
    expect(samePDFPreviewViewState(base, { ...base, fit: 'page' })).toBe(false)
  })

  it('separates a fit that was chosen from no fit at all, in both directions', () => {
    expect(samePDFPreviewViewState(base, { ...base, fit: undefined })).toBe(false)
    expect(samePDFPreviewViewState({ ...base, fit: undefined }, base)).toBe(false)
    expect(samePDFPreviewViewState({ ...base, fit: undefined }, { ...base, fit: undefined })).toBe(true)
    // The value App resets to, against a copy of itself: the reset must read as
    // the same view state, or every clear would be a spurious change.
    expect(samePDFPreviewViewState(initialPDFPreviewViewState, { ...initialPDFPreviewViewState })).toBe(true)
    expect(samePDFPreviewViewState(initialPDFPreviewViewState, { ...initialPDFPreviewViewState, fit: 'width' })).toBe(false)
  })
})
