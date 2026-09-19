import { useEffect, useRef, useState } from 'react'
import type { PDFDocumentLoadingTask, PDFDocumentProxy } from 'pdfjs-dist/build/pdf.mjs'
import workerUrl from 'pdfjs-dist/build/pdf.worker.mjs?url'
import { pdfjsRuntimeAssets, pdfjsViewerAssets } from '../generated/pdfjs-assets'
import type { PDFThumbnailView } from '../vendor/pdfjs/pdf_thumbnail_view.js'
import { PAGE_RAIL_BOUND, railEntries, railEventBus, railLinkService, railPlaceholderViewport, railRenderingQueue, railThumbnailChrome, railTruncationLabel } from './page-rail-facts'

export type PageRailProps = Readonly<{ bytes: ArrayBuffer; pages?: number; currentPage: number; onGoToPage: (page: number) => void }>

// THE RAIL IS A PURE READER OF `previewViewState`, AND HOLDS NO PAGE STATE.
//
// `currentPage` arrives as a prop from `App.tsx:254`'s `previewViewState`, the
// single page-state authority, and a click leaves through `onGoToPage`, which
// App binds to the same `changePreviewViewState` funnel the status bar uses.
// There is no `useState` and no `useRef` here holding a page number: the only
// state is the list of thumbnail views, and the only ref is the rail element
// itself. That is not tidiness — it is the whole reason `pdf_thumbnail_viewer.js`
// (which owns a `_currentPageNumber`) was not vendored. `PDFThumbnailView` has
// no current-page value of its own; `toggleCurrent` is driven from here.
//
// ⚠ THE MARKING IS A PURE FUNCTION OF THE PROP. Re-rendering with a new
// `currentPage` moves it, with nothing rail-internal to fall out of step.
export function PageRail({ bytes, pages, currentPage, onGoToPage }: PageRailProps) {
  const { entries, remaining } = railEntries(pages, currentPage, PAGE_RAIL_BOUND)
  const rail = useRef<HTMLDivElement>(null)
  const [views, setViews] = useState<readonly PDFThumbnailView[]>([])
  const count = entries.length

  // THE RASTERISATION PASS. It depends on the bytes and on how many entries
  // there are — NOT on the current page, so navigating the rail never tears the
  // thumbnails down and draws them again.
  //
  // The document is opened here rather than borrowed from `PDFPreviewViewer`,
  // which keeps its `PDFDocumentProxy` private to its own effect. These are the
  // SAME BYTES the engine already produced — no second engine render and no
  // re-serialisation — and `bytes.slice(0)` is copied for the same reason the
  // viewer copies them: pdf.js transfers the buffer it is handed, and detaching
  // App's copy would blank the page area.
  useEffect(() => {
    if (count === 0) return
    let cancelled = false
    const created: PDFThumbnailView[] = []
    let opened: PDFDocumentProxy | undefined
    let loading: PDFDocumentLoadingTask | undefined
    const rasterise = async () => {
      // Referencing the generated map keeps every support byte in the release
      // set, exactly as the viewer does.
      void pdfjsRuntimeAssets
      // BOTH IMPORTS ARE DYNAMIC, AND THE VENDORED ONE HAS TO BE. It imports
      // `pdfjs-dist/build/pdf.mjs` at module scope, and pdf.js touches
      // `DOMMatrix` while evaluating — which jsdom does not define. A STATIC
      // import here would therefore put pdf.js in `App.tsx`'s module graph and
      // break every unit test that renders App without mocking it (measured:
      // three files, `ReferenceError: DOMMatrix is not defined`). It is also
      // what keeps pdf.js out of the initial chunk, exactly as the viewer does.
      const [pdfjs, thumbnails] = await Promise.all([import('pdfjs-dist/build/pdf.mjs'), import('../vendor/pdfjs/pdf_thumbnail_view.js')])
      pdfjs.GlobalWorkerOptions.workerSrc = workerUrl
      if (cancelled) return
      // `Array.from`, because `lib` is ES2023 + DOM without DOM.Iterable, so a
      // NodeList is array-LIKE here rather than iterable.
      const wells = Array.from(rail.current?.querySelectorAll('[data-page-rail-well]') ?? [])
      loading = pdfjs.getDocument({ data: new Uint8Array(bytes.slice(0)), disableAutoFetch: true, disableStream: true, stopAtErrors: true, isEvalSupported: false, useWorkerFetch: false, cMapUrl: pdfjsViewerAssets.cMapUrl, cMapPacked: pdfjsViewerAssets.cMapPacked, standardFontDataUrl: pdfjsViewerAssets.standardFontDataUrl })
      const document = await loading.promise
      if (cancelled) { await document.destroy(); return }
      opened = document
      for (const well of wells) {
        const page = Number(well.getAttribute('data-page-rail-well'))
        if (!Number.isInteger(page) || page < 1 || page > document.numPages) continue
        const view = new thumbnails.PDFThumbnailView({ container: well as HTMLElement, eventBus: railEventBus, id: page, defaultViewport: railPlaceholderViewport, linkService: railLinkService(document.numPages), renderingQueue: railRenderingQueue })
        railThumbnailChrome(view)
        created.push(view)
      }
      if (cancelled) { for (const view of created) view.destroy(); return }
      setViews(created)
      // ONE ENTRY'S FAILURE IS ONE ENTRY'S FAILURE, AND `reset()` IS WHAT MAKES
      // THAT TRUE RATHER THAN JUST INTENDED.
      //
      // ⚠ A FAILED `draw` DOES NOT LEAVE THE PLACEHOLDER BEHIND ON ITS OWN. Read
      // the vendored `draw`: on a non-cancellation rejection it records the error,
      // sets `renderingState = FINISHED`, then AWAITS `#convertCanvasToImage` —
      // which sets `image.src` and removes `missingThumbnailImage` — and only
      // then rethrows. So the entry would show a BLANK WHITE thumbnail, which
      // reads as a page that rendered empty. `reset()` is the vendored file's own
      // undo for exactly that state: it revokes the object URL, clears `src`, and
      // puts `missingThumbnailImage` back. Fixing it here rather than in the
      // vendored source is what keeps the fork at three recorded modifications.
      //
      // `reset()` re-runs `#updateDims`, which rewrites the inline height, so the
      // chrome goes back on after it.
      for (const view of created) {
        if (cancelled) return
        try {
          const pdfPage = await document.getPage(view.id)
          if (cancelled) return
          view.setPdfPage(pdfPage)
          railThumbnailChrome(view)
          await view.draw()
          railThumbnailChrome(view)
        }
        catch {
          // This entry goes back to its placeholder; the rail stays usable, and
          // nothing here reaches the page area the author is reading.
          try { view.reset(); railThumbnailChrome(view) } catch { /* a torn-down view has nothing to restore */ }
        }
      }
    }
    // The whole pass is best-effort for the same reason: the rail is a
    // navigation aid over a document the page area is already showing, so a
    // failure to open it costs thumbnails and nothing else.
    void rasterise().catch(() => undefined)
    return () => {
      cancelled = true
      for (const view of created) view.destroy()
      setViews([])
      void (async () => {
        try { await opened?.destroy() } catch { /* cancellation is expected */ }
        try { await loading?.destroy() } catch { /* cancellation is expected */ }
      })()
    }
  }, [bytes, count])

  // The vendored view's own current marking, driven from the prop. folio8's
  // `<button>` carries the marking the accessibility tree actually sees; this
  // keeps the pdf.js DOM inside it from contradicting it.
  //
  // ⚠ `railThumbnailChrome` IS NOT OPTIONAL HERE. `toggleCurrent` writes all
  // three of the properties that call clears — `tabIndex = 0` on the current
  // container, a reflected `ariaCurrent` (`"page"` on the current one and the
  // string `"false"` on every other), and, through `reset`, the inline height.
  // So it must run after EVERY call, on the first pass and on every `currentPage`
  // change, or the rail ships two `aria-current="page"` nodes per current entry
  // and a focusable element inside an `aria-hidden` subtree.
  useEffect(() => {
    for (const view of views) {
      view.toggleCurrent(view.id === currentPage)
      railThumbnailChrome(view)
    }
  }, [views, currentPage])

  return <nav className="page-rail" aria-label="Page thumbnails" ref={rail}>
    <p className="section-label">PAGES</p>
    <ol className="page-rail-list">
      {entries.map((entry) => <li key={entry.page}>
        {/* A REAL `<button>`, not the `div[role=button]` pdf.js builds inside
            it. Keyboard reach, activation by Enter and Space, and the focus
            ring are the platform's own, and the click leaves through the same
            funnel the status bar's stepper uses. The thumbnail well is
            `aria-hidden` because everything pdf.js puts in it — a second button
            role, a second `aria-current`, a localiser's `data-l10n-*` for a
            localiser folio8 does not have — is chrome for a viewer application
            that is not this one. */}
        <button className={`page-rail-entry${entry.current ? ' page-rail-entry-current' : ''}`} type="button" aria-label={`Page ${entry.page}`} aria-current={entry.current ? 'page' : undefined} onClick={() => onGoToPage(entry.page)}>
          <span className="page-rail-well" data-page-rail-well={entry.page} aria-hidden="true" />
          <span className="page-rail-number">{entry.page}</span>
        </button>
      </li>)}
    </ol>
    {/* THE BOUND IS NOT A CEILING ON NAVIGATION. The status bar reaches every
        page in the document, so truncating here costs the author a picture of
        page 30, never the route to it. */}
    {remaining !== undefined && <p className="page-rail-more">{railTruncationLabel(remaining)}</p>}
  </nav>
}
