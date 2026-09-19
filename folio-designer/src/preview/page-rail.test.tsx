import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { act, fireEvent, render, screen, waitFor, within, type RenderResult } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

const state = vi.hoisted(() => ({ getDocument: vi.fn(), getPage: vi.fn(), documentDestroy: vi.fn(), loadingDestroy: vi.fn(), workerSrc: '' }))

// THE MODULE MOCK FOLLOWS `pdf-viewer.test.tsx`'s, plus the two names the
// VENDORED file imports statically. `pdf_thumbnail_view.js` reaches for
// `OutputScale` and `RenderingCancelledException` at module scope, so a factory
// that omitted them would fail to link rather than fail an assertion.
vi.mock('pdfjs-dist/build/pdf.mjs', () => ({
  GlobalWorkerOptions: { get workerSrc() { return state.workerSrc }, set workerSrc(value: string) { state.workerSrc = value } },
  getDocument: state.getDocument,
  OutputScale: class { sx = 1; sy = 1; scaled = false; limitCanvas() { /* no canvas is rasterised in jsdom */ } },
  RenderingCancelledException: class extends Error {},
}))
vi.mock('pdfjs-dist/build/pdf.worker.mjs?url', () => ({ default: '/assets/pdf.worker-local.mjs' }))

import { PageRail } from './page-rail'
import { pdfjsViewerAssets } from '../generated/pdfjs-assets'
import { PDFThumbnailView } from '../vendor/pdfjs/pdf_thumbnail_view.js'

// RASTERISATION IS NOT PROVEN HERE, AND CANNOT BE. Measured in jsdom 28.1.0:
// `canvas.getContext('2d')` returns null, and `OffscreenCanvas`,
// `createImageBitmap` and `ImageBitmap` are all undefined. The `canvas` package
// is not installed and must not be added — `font-store.test.ts:426` and
// `font-name-table.test.ts:160` both assert `dependencies` by exact equality. So
// `draw` is stubbed and the witness that the vendored code actually rasterises is
// `e2e/preview-page-rail.spec.ts`, in a real browser.
const drawn = () => vi.spyOn(PDFThumbnailView.prototype, 'draw').mockResolvedValue(undefined)

const pdfPage = () => ({ rotate: 0, getViewport: () => ({ width: 44, height: 62, rotation: 0 }) })
const openDocument = (numPages: number) => {
  state.getPage.mockImplementation(async () => pdfPage())
  state.getDocument.mockReturnValue({ promise: Promise.resolve({ numPages, getPage: state.getPage, destroy: state.documentDestroy }), destroy: state.loadingDestroy })
}
// ONE BUFFER, SHARED ACROSS RE-RENDERS. `bytes` is a dependency of the
// rasterisation effect, so a fresh `ArrayBuffer` per `railProps()` call would
// make every rerender look like a NEW DOCUMENT — which is exactly the defect
// `does not re-rasterise when only the current page changes` is written against,
// and it would have been the harness causing it rather than the rail.
const bytes = new Uint8Array([1, 2, 3]).buffer
const railProps = (overrides: Partial<Parameters<typeof PageRail>[0]> = {}) => ({ bytes, pages: 5, currentPage: 1, onGoToPage: vi.fn(), ...overrides })
const entries = () => screen.getAllByRole('button').map((button) => button.getAttribute('aria-label'))
const rail = () => screen.getByRole('navigation', { name: 'Page thumbnails' })
const wells = () => Array.from(rail().querySelectorAll<HTMLElement>('.page-rail-well'))
const containerOf = (index: number) => wells()[index]?.querySelector('.thumbnailImageContainer') as HTMLElement
const readSource = (...segments: string[]) => readFileSync(join(import.meta.dirname, ...segments), 'utf8')

// EVERY RENDER IS UNMOUNTED BY THIS FILE, BEFORE RTL's OWN CLEANUP RUNS.
//
// The rasterisation pass opens with `await Promise.all([import(…), import(…)])`,
// which vitest resolves across a MACROTASK boundary — so a test that renders and
// then asserts synchronously leaves a continuation in flight that lands inside
// the NEXT test and calls `getDocument` there. The first version of this file
// papered over that with a real `setTimeout(0)` drain in `beforeEach`, which is a
// timing guess. Unmounting here is deterministic instead: the effect's cleanup
// sets `cancelled = true` synchronously, and the parked continuation hits its
// `if (cancelled) return` BEFORE it can reach `getDocument`. Measured: with this
// teardown in place, `toHaveBeenCalledOnce()` on `getDocument` holds with no
// drain anywhere in the file.
//
// vitest runs after-hooks last-registered-first and the shared setup file
// registers RTL's `cleanup` before this describe exists, so this really does run
// first.
let mounted: RenderResult | undefined
const mount = (props: Parameters<typeof PageRail>[0]) => { mounted = render(<PageRail {...props} />); return mounted }

// THE ONE DETERMINISTIC SETTLE, USED EVERYWHERE INSTEAD OF A MICROTASK LOOP.
//
// `waitFor` on the draw count is the real signal that the pass reached its last
// entry; the `act` that follows is what flushes the `setViews` commit and the
// MARKING EFFECT that runs off it. Both halves matter: without the `act`, whether
// the marking effect has run depends on how many other files are in the vitest
// invocation — measured, `railThumbnailChrome`'s deletion from the marking effect
// was caught when `src/App.test.tsx` ran alongside this file and invisible when
// this directory ran alone.
const settled = async (draw: ReturnType<typeof drawn>, count: number) => {
  await waitFor(() => expect(draw).toHaveBeenCalledTimes(count))
  await act(async () => { await Promise.resolve() })
}
// For the rows whose claim is that NOTHING further happens. `act` drains React's
// queue and the microtasks behind it, so this is the deterministic form of "the
// pass has had every chance to run again".
const quiet = async () => { await act(async () => { await Promise.resolve() }) }

describe('the preview navigates by page thumbnails', () => {
  afterEach(async () => {
    mounted?.unmount()
    mounted = undefined
    await quiet()
    vi.restoreAllMocks()
    state.getDocument.mockReset(); state.getPage.mockReset(); state.documentDestroy.mockReset(); state.loadingDestroy.mockReset(); state.workerSrc = ''
  })

  it('enumerates a render as one numbered entry per page, in page order', () => {
    mount(railProps())
    expect(within(rail()).getByText('PAGES')).toBeInTheDocument()
    expect(entries()).toEqual(['Page 1', 'Page 2', 'Page 3', 'Page 4', 'Page 5'])
    // The number is on screen as well as in the name, which is what the design
    // draws beside each thumbnail.
    expect(within(rail()).getAllByText(/^[1-5]$/).map((node) => node.textContent)).toEqual(['1', '2', '3', '4', '5'])
  })

  it('marks the current page alone, and moves the marking when the prop moves', () => {
    const view = mount(railProps({ currentPage: 3 }))
    // `getAllByRole` ignores an `aria-hidden` subtree, which is what keeps the
    // `div[role=button]` pdf.js builds inside each well out of this count.
    expect(screen.getAllByRole('button', { current: 'page' }).map((button) => button.getAttribute('aria-label'))).toEqual(['Page 3'])
    expect(rail().querySelectorAll('.page-rail-entry-current')).toHaveLength(1)
    // RE-RENDERED WITH A NEW CURRENT PAGE, AND NOTHING RAIL-INTERNAL IS
    // INVOLVED: the marking is a pure function of the prop, so it follows.
    view.rerender(<PageRail {...railProps({ currentPage: 5 })} />)
    expect(screen.getAllByRole('button', { current: 'page' }).map((button) => button.getAttribute('aria-label'))).toEqual(['Page 5'])
    view.rerender(<PageRail {...railProps({ currentPage: 1 })} />)
    expect(screen.getAllByRole('button', { current: 'page' }).map((button) => button.getAttribute('aria-label'))).toEqual(['Page 1'])
  })

  // ⚠ THE `aria-current` ROW THAT CAN ACTUALLY FAIL, WHICH THE ONE ABOVE CANNOT.
  //
  // pdf.js's `toggleCurrent` writes `ariaCurrent` on its own container, and that
  // property IS reflected: `'page'` becomes `aria-current="page"` on a second
  // element inside folio8's button, and `false` becomes the literal string
  // `aria-current="false"` on EVERY other entry. So the shipped defect is only
  // visible once `PDFThumbnailView` instances exist and the marking effect has
  // run — and in the row above no document opens, so no thumbnail DOM is ever
  // built and a count of one is measured over a rail with no containers in it.
  // This row opens a document, settles the pass, and counts again.
  it('carries exactly one aria-current in the whole rail once the thumbnail DOM exists', async () => {
    const draw = drawn()
    openDocument(5)
    const view = mount(railProps({ currentPage: 3 }))
    await settled(draw, 5)
    // The thumbnail DOM really is there — otherwise this row would be as blind
    // as the one above.
    expect(rail().querySelectorAll('.thumbnailImageContainer')).toHaveLength(5)
    expect(rail().querySelectorAll('[aria-current]')).toHaveLength(1)
    expect(rail().querySelector('[aria-current]')).toBe(screen.getByRole('button', { name: 'Page 3' }))
    // And not one container carries the attribute in either state — `page` on the
    // current one, `false` on the other four.
    for (const container of Array.from(rail().querySelectorAll('.thumbnailImageContainer'))) expect(container.hasAttribute('aria-current')).toBe(false)
    // Still one after the marking moves.
    view.rerender(<PageRail {...railProps({ currentPage: 5 })} />)
    await quiet()
    expect(rail().querySelectorAll('[aria-current]')).toHaveLength(1)
    expect(rail().querySelector('[aria-current]')).toBe(screen.getByRole('button', { name: 'Page 5' }))
  })

  // ⚠ THE MARKING EFFECT'S `railThumbnailChrome` CALL, OBSERVED. `toggleCurrent`
  // writes three properties back on every call — `tabIndex = 0` on the current
  // container, `ariaCurrent`, and (through `reset`) the inline height — so the
  // call that puts them right has to run after it, on the first pass AND on every
  // `currentPage` rerender. Both are asserted, because the rerender path is the
  // one a first-pass-only fix would leave broken.
  it('keeps pdf.js chrome off the current entry after the marking effect, and after it moves', async () => {
    const draw = drawn()
    openDocument(5)
    const view = mount(railProps({ currentPage: 1 }))
    await settled(draw, 5)
    const assertClean = (label: string) => {
      for (const container of Array.from(rail().querySelectorAll<HTMLElement>('.thumbnailImageContainer'))) {
        // A focusable node inside an `aria-hidden` subtree is reachable by Tab
        // and invisible to the accessibility tree.
        expect(container.tabIndex, label).toBe(-1)
        expect(container.hasAttribute('aria-current'), label).toBe(false)
        // pdf.js sizes this from its own fixed 126px thumbnail width; the rail's
        // well is 44×62 and cannot override an inline style.
        expect(container.style.height, label).toBe('')
      }
    }
    assertClean('first pass')
    view.rerender(<PageRail {...railProps({ currentPage: 4 })} />)
    await quiet()
    assertClean('after the marking moved')
  })

  // THE FUNNEL ASSERTION, COMPARED AS A WHOLE OBJECT.
  //
  // `PDFPreviewViewState` carries `page`, `scale`, `scrollTop`, `scrollLeft` and
  // `fit`. A rail that navigated by handing the funnel a freshly built object
  // would silently reset zoom and scroll, and a test asserting `next.page === 4`
  // would pass straight over it. So the harness below is App's own
  // `goToPreviewPage` — `{ ...previewViewState, page }` — and the next state is
  // compared against the previous one in full. (`App.test.tsx` asserts the same
  // property through the real funnel and the real status-bar readouts.)
  it('requests a page change through the funnel once, changing only the page', async () => {
    const draw = drawn()
    openDocument(5)
    // The scroll members are spelled plainly: the AD-17 `scroll*` waiver is
    // directory-wide inside `src/preview/`, unlike the fit measurement's, which
    // is bounded to one function in `pdf-viewer.tsx`.
    const previous = { page: 1, scale: 1.5, fit: 'width', scrollTop: 240, scrollLeft: 12 }
    const funnel = vi.fn()
    mount(railProps({ currentPage: 1, onGoToPage: (page: number) => funnel({ ...previous, page }) }))
    await settled(draw, 5)
    fireEvent.click(screen.getByRole('button', { name: 'Page 4' }))
    expect(funnel).toHaveBeenCalledOnce()
    const next = funnel.mock.calls[0]![0] as Record<string, unknown>
    const before = previous as unknown as Record<string, unknown>
    expect(next).toEqual({ ...before, page: 4 })
    // The same keys, and exactly one of them different.
    expect(Object.keys(next)).toEqual(Object.keys(before))
    expect(Object.keys(next).filter((key) => next[key] !== before[key])).toEqual(['page'])
  })

  // AND THE RAIL NEVER NAVIGATES ON ITS OWN. `PDFThumbnailView` dispatches
  // `thumbnailrendered` through the event bus folio8 injects; if that sink ever
  // grew a behaviour, rasterising a document would move the author's page.
  it('never calls the funnel while rasterising, only on activation', async () => {
    const draw = drawn()
    openDocument(5)
    const onGoToPage = vi.fn()
    mount(railProps({ onGoToPage }))
    await settled(draw, 5)
    expect(onGoToPage).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: 'Page 2' }))
    expect(onGoToPage).toHaveBeenCalledExactlyOnceWith(2)
  })

  it('is reachable and activatable from the keyboard, as a real button', () => {
    const onGoToPage = vi.fn()
    mount(railProps({ onGoToPage }))
    const entry = screen.getByRole('button', { name: 'Page 4' })
    expect(entry.tagName).toBe('BUTTON')
    expect(entry).not.toHaveAttribute('tabindex')
    entry.focus()
    expect(document.activeElement).toBe(entry)
    // A keyboard activation of a native button arrives as a click with
    // `detail === 0`; jsdom implements no activation behaviour of its own.
    fireEvent.click(entry, { detail: 0 })
    expect(onGoToPage).toHaveBeenCalledExactlyOnceWith(4)
  })

  it('bounds a long render and says how many pages it left out', () => {
    mount(railProps({ pages: 34, currentPage: 1 }))
    expect(entries()).toHaveLength(12)
    expect(entries().at(-1)).toBe('Page 12')
    expect(screen.queryByRole('button', { name: 'Page 13' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Page 34' })).toBeNull()
    expect(within(rail()).getByText('… 22 more')).toBeInTheDocument()
  })

  it('says nothing at the bound and one at the bound plus one', () => {
    const view = mount(railProps({ pages: 12 }))
    expect(entries()).toHaveLength(12)
    expect(within(rail()).queryByText(/more$/)).toBeNull()
    view.rerender(<PageRail {...railProps({ pages: 13 })} />)
    expect(entries()).toHaveLength(12)
    expect(within(rail()).getByText('… 1 more')).toBeInTheDocument()
  })

  it('shows one entry for a one-page render, and no truncation', () => {
    mount(railProps({ pages: 1 }))
    expect(entries()).toEqual(['Page 1'])
    expect(within(rail()).queryByText(/more$/)).toBeNull()
  })

  it('renders its heading and nothing else before a render has reported a page count', () => {
    mount(railProps({ pages: undefined }))
    expect(within(rail()).getByText('PAGES')).toBeInTheDocument()
    expect(screen.queryAllByRole('button')).toEqual([])
    expect(within(rail()).queryByText(/more$/)).toBeNull()
    // Never a thumbnail for a page that does not exist: there is no well to
    // rasterise into, so nothing is opened and nothing is drawn.
    expect(wells()).toHaveLength(0)
    expect(state.getDocument).not.toHaveBeenCalled()
  })

  // ⚠ THE ORDER PRODUCTION ACTUALLY PRODUCES, AND THE ONLY ROW THAT VISITS IT.
  //
  // `previewPages` starts `undefined` and is set only later, by the VIEWER's
  // `onPageCount`. So the rail always mounts with no page count at all, takes the
  // `count === 0` early return, and depends entirely on the rasterisation effect
  // RE-ARMING when the count arrives. Every other row here mounts with `pages`
  // already a number — the one order production never produces — so narrowing the
  // effect's dependencies from `[bytes, count]` to `[bytes]` ships a rail that
  // never draws a single thumbnail and reddens nothing. Measured: with the deps
  // narrowed, `src/preview` plus `src/App.test.tsx` was 447 passed, 0 failed.
  it('re-arms and rasterises when the page count arrives after mount', async () => {
    const draw = drawn()
    openDocument(5)
    const view = mount(railProps({ pages: undefined }))
    await quiet()
    expect(state.getDocument).not.toHaveBeenCalled()
    expect(draw).not.toHaveBeenCalled()
    // The viewer reports the count, App re-renders the rail with it, and only now
    // is there anything to rasterise.
    view.rerender(<PageRail {...railProps({ pages: 5 })} />)
    await settled(draw, 5)
    expect(state.getDocument).toHaveBeenCalledOnce()
    expect(state.getPage.mock.calls.map((call) => call[0])).toEqual([1, 2, 3, 4, 5])
    expect(rail().querySelectorAll('.thumbnailImageContainer')).toHaveLength(5)
  })

  it('leaves a page beyond the bound unmarked while the rail still lists the first twelve', () => {
    mount(railProps({ pages: 34, currentPage: 30 }))
    expect(entries()).toHaveLength(12)
    expect(screen.queryAllByRole('button', { current: 'page' })).toEqual([])
    expect(rail().querySelectorAll('[aria-current]')).toHaveLength(0)
  })

  it('rasterises one thumbnail per entry from the bytes the engine already produced, with the local worker', async () => {
    const draw = drawn()
    openDocument(5)
    const source = new Uint8Array([1, 2, 3]).buffer
    mount(railProps({ bytes: source }))
    await settled(draw, 5)
    // ⚠ ALL NINE OPTIONS, NOT THE FOUR THAT ARE OBVIOUS. The rail opens its own
    // document, so it carries its own copy of the viewer's option object — and
    // the three easiest to lose in a drift are exactly the three that make CJK
    // and the standard fonts rasterise with no network: `cMapUrl`, `cMapPacked`
    // and `standardFontDataUrl`. The key set is asserted too, so an option
    // QUIETLY DROPPED fails as loudly as one changed.
    const options = state.getDocument.mock.calls[0]![0] as Record<string, unknown>
    expect(Object.keys(options).sort()).toEqual(['cMapPacked', 'cMapUrl', 'data', 'disableAutoFetch', 'disableStream', 'isEvalSupported', 'standardFontDataUrl', 'stopAtErrors', 'useWorkerFetch'])
    // The SAME bytes, copied rather than transferred — pdf.js detaches the
    // buffer it is handed, and detaching App's copy would blank the page area.
    expect(options.data).toEqual(new Uint8Array([1, 2, 3]))
    expect((options.data as Uint8Array).buffer).not.toBe(source)
    expect(options).toMatchObject({ disableAutoFetch: true, disableStream: true, stopAtErrors: true, isEvalSupported: false, useWorkerFetch: false })
    // The local, content-addressed release directories — asserted BOTH against
    // the generated module (so the rail cannot invent its own URL) and against
    // the immutable shape (so the generated module cannot quietly become a
    // network origin).
    expect(options.cMapUrl).toBe(pdfjsViewerAssets.cMapUrl)
    expect(options.standardFontDataUrl).toBe(pdfjsViewerAssets.standardFontDataUrl)
    expect(options.cMapPacked).toBe(pdfjsViewerAssets.cMapPacked)
    expect(options.cMapUrl).toMatch(/^\/assets\/pdfjs-cmaps-[a-f0-9]{20}\/$/)
    expect(options.standardFontDataUrl).toMatch(/^\/assets\/pdfjs-standard-fonts-[a-f0-9]{20}\/$/)
    expect(options.cMapPacked).toBe(true)
    expect(state.workerSrc).toBe('/assets/pdf.worker-local.mjs')
    expect(state.getDocument).toHaveBeenCalledOnce()
    // One page fetched per entry, and every entry is a real page of the document.
    expect(state.getPage.mock.calls.map((call) => call[0])).toEqual([1, 2, 3, 4, 5])
    // pdf.js's own thumbnail DOM landed inside folio8's wells, and stayed out of
    // the accessibility tree.
    expect(wells()).toHaveLength(5)
    for (const well of wells()) {
      expect(well.getAttribute('aria-hidden')).toBe('true')
      expect(well.querySelectorAll('img')).toHaveLength(1)
    }
  })

  it('opens the document once for the bounded set, not once per page of a long render', async () => {
    const draw = drawn()
    openDocument(34)
    mount(railProps({ pages: 34 }))
    await settled(draw, 12)
    expect(state.getDocument).toHaveBeenCalledOnce()
    expect(state.getPage.mock.calls.map((call) => call[0])).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12])
  })

  // ⚠ THE FAILING STUB EMULATES pdf.js's REAL FAILURE PATH, WHICH IS THE WHOLE
  // POINT OF THIS ROW.
  //
  // The vendored `draw` does NOT abandon the placeholder on a render rejection:
  // it records the error, sets `renderingState = FINISHED`, awaits
  // `#convertCanvasToImage` — which sets `image.src` and removes
  // `missingThumbnailImage` — and only then rethrows. A stub that merely throws
  // leaves the placeholder untouched and cannot see that at all, which is what
  // the first version of this row did. So this stub does what pdf.js does, and
  // the assertion is that the rail's `view.reset()` puts the placeholder back.
  it('keeps one failed thumbnail on its placeholder, and the rail and every other entry stay usable', async () => {
    let attempt = 0
    const draw = vi.spyOn(PDFThumbnailView.prototype, 'draw').mockImplementation(async function convertsThenFails(this: PDFThumbnailView) {
      attempt++
      // Exactly what `#convertCanvasToImage` does — on BOTH paths, which is the
      // point: the vendored `draw` runs this conversion before it rethrows, so a
      // failed entry and a successful one leave the DOM in the same state and
      // only `view.reset()` tells them apart afterwards.
      this.image.src = `blob:http://localhost/page-${attempt}`
      this.imageContainer.classList.remove('missingThumbnailImage')
      if (attempt === 2) throw new Error('rasterisation failed')
    })
    openDocument(5)
    const onGoToPage = vi.fn()
    mount(railProps({ onGoToPage }))
    await settled(draw, 5)
    // ENTRY 2 IS BACK ON ITS PLACEHOLDER: the marker class is restored and the
    // image carries no source, so nothing shows a blank white page as a render.
    const failed = containerOf(1)
    expect(failed.classList.contains('missingThumbnailImage')).toBe(true)
    expect(failed.querySelector('img')?.getAttribute('src')).toBe('')
    // And its neighbours were never touched.
    for (const index of [0, 2, 3, 4]) expect(containerOf(index).classList.contains('missingThumbnailImage'), `entry ${index + 1}`).toBe(false)
    // The chrome survived the `reset()` too — `reset` re-runs `#updateDims`,
    // which rewrites the inline height.
    expect(failed.style.height).toBe('')
    expect(failed.tabIndex).toBe(-1)
    // And the rail is still a rail: five entries, still navigable.
    expect(entries()).toEqual(['Page 1', 'Page 2', 'Page 3', 'Page 4', 'Page 5'])
    fireEvent.click(screen.getByRole('button', { name: 'Page 3' }))
    expect(onGoToPage).toHaveBeenCalledExactlyOnceWith(3)
  })

  it('keeps every entry when the document cannot be opened at all', async () => {
    drawn()
    state.getDocument.mockReturnValue({ promise: Promise.reject(new Error('not a PDF')), destroy: state.loadingDestroy })
    mount(railProps())
    await waitFor(() => expect(state.getDocument).toHaveBeenCalledOnce())
    await quiet()
    expect(entries()).toEqual(['Page 1', 'Page 2', 'Page 3', 'Page 4', 'Page 5'])
    expect(within(rail()).getByText('PAGES')).toBeInTheDocument()
  })

  it('destroys the document it opened when the rail goes away', async () => {
    const draw = drawn()
    openDocument(5)
    const view = mount(railProps())
    await settled(draw, 5)
    view.unmount()
    mounted = undefined
    await waitFor(() => expect(state.documentDestroy).toHaveBeenCalled())
    expect(state.loadingDestroy).toHaveBeenCalled()
  })

  it('does not re-rasterise when only the current page changes', async () => {
    const draw = drawn()
    openDocument(5)
    const view = mount(railProps({ currentPage: 1 }))
    await settled(draw, 5)
    view.rerender(<PageRail {...railProps({ currentPage: 4 })} />)
    await quiet()
    // A page change is navigation, not a new document: one open, one draw per
    // entry, and no tear-down.
    expect(state.getDocument).toHaveBeenCalledOnce()
    expect(draw).toHaveBeenCalledTimes(5)
    expect(state.documentDestroy).not.toHaveBeenCalled()
  })

  it('stores no page state of its own', () => {
    // The executable half of the one-authority claim is above — the marking
    // follows the prop on re-render, and nothing accumulates. This is the
    // textual half: the rail's own source names neither hook holding a page.
    for (const file of ['page-rail.tsx', 'page-rail-facts.ts']) {
      const text = readSource(file)
      // No hook holds a number, and no ref is ever assigned a page. The rail's
      // header names `_currentPageNumber` — the field in the file this story
      // deliberately did NOT vendor — so the claim is about the SHAPE of a
      // stored page, not about the string appearing in prose.
      expect(text.match(/use(?:State|Ref)\s*<[^>]*\bnumber\b/), file).toBeNull()
      expect(text.match(/\.current\s*=\s*(?:currentPage|page)\b/), file).toBeNull()
      expect(text.match(/setPage\s*\(|setCurrentPage\s*\(/), file).toBeNull()
    }
  })

  // THE SHELL RULE THE RAIL DEPENDS ON, GUARDED AS TEXT BECAUSE THAT IS ALL
  // jsdom CAN SEE.
  //
  // Preview's grid has three columns and its FIRST child is conditional — no
  // document, no rail. With auto-placement that drops `main.preview-region` into
  // the 132px track on the story's own failed-render row. The real proof is
  // geometric and lives in `e2e/preview-page-rail.spec.ts`; this row exists so
  // that DELETING the fix reddens something inside the story's own cadence, and
  // it is not claimed to be a layout assertion.
  it('assigns all three Preview columns by name rather than leaving the rail-less case to auto-placement', () => {
    const css = readSource('..', 'App.css')
    expect(css).toContain('.app-shell-preview .workbench { grid-template-columns: 132px minmax(0, 1fr) var(--panel-width); }')
    expect(css).toContain('.app-shell-preview .page-rail { grid-column: 1; }')
    expect(css).toContain('.app-shell-preview .preview-region { grid-column: 2; }')
    expect(css).toContain('.app-shell-preview .inspector-panel { grid-column: 3; }')
    // And the class the rail actually renders has a rule of its own rather than
    // being a dead hook.
    expect(css).toContain('.page-rail-number {')
    expect(readSource('page-rail.tsx')).toContain('className="page-rail-number"')
  })
})
