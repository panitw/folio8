import { useEffect, useRef } from 'react'
import type { PDFDocumentLoadingTask, PDFDocumentProxy, RenderTask } from 'pdfjs-dist/build/pdf.mjs'
import workerUrl from 'pdfjs-dist/build/pdf.worker.mjs?url'
import { pdfjsRuntimeAssets, pdfjsViewerAssets } from '../generated/pdfjs-assets'
import { fitPreviewScale, type PreviewBox, type PreviewFit } from './viewer-navigation'

// PDF.js is strictly a local rasterizer for bytes the Go renderer returned.
// The worker URL is a Vite-managed immutable release asset, never a CDN URL.

type ActiveDocument = { loading: PDFDocumentLoadingTask; document?: PDFDocumentProxy; task?: RenderTask; canvas?: HTMLCanvasElement }

export type PDFPreviewViewState = Readonly<{ page: number; scale: number; scrollTop: number; scrollLeft: number; fit?: PreviewFit }>
// How many bitmap pixels the preview rasterizes per displayed pixel. Two
// covers the common Retina case; a third would quadruple the bitmap for a
// difference few displays can resolve.
const previewOversample = 2
export const initialPDFPreviewViewState: PDFPreviewViewState = { page: 1, scale: 1, scrollTop: 0, scrollLeft: 0 }
export const samePDFPreviewViewState = (left: PDFPreviewViewState, right: PDFPreviewViewState) => left.page === right.page && left.scale === right.scale && left.scrollTop === right.scrollTop && left.scrollLeft === right.scrollLeft && left.fit === right.fit

// THE ONE PLACE THE DESIGNER ASKS THE BROWSER HOW BIG SOMETHING IS.
//
// `Fit width` and `Fit page` have no constant that can stand in for them: the
// answer is the container's actual pixel size in this window, on this machine,
// with this panel layout. AD-17's rule is not that the browser is never asked —
// it is that the asking is named, scoped and provable, which is why
// `canvas-authority-contract.test.ts` admits exactly the two spellings below,
// each written out on its own, inside exactly this function, and asserts this
// function is still here.
//
// The element handed in carries NO padding and NO border, deliberately: the
// reading includes padding, the property that could subtract it is banned, and
// hardcoding a mirror of `--space-6` would be a constant that drifts the moment
// the token moves. `App.css` puts the preview's padding on the region around
// the scroller instead, and `scrollbar-gutter: stable` on the scroller keeps a
// vertical scrollbar's appearance from changing the width mid-fit.
function measuredViewerBox(host: HTMLDivElement | null): PreviewBox {
  if (!host) return { width: 0, height: 0 }
  return { width: host.clientWidth, height: host.clientHeight }
}

// The scroll host is re-populated with a fresh canvas on every rasterization,
// which resets its offsets to zero. Re-applying the recorded ones right after
// the swap is what makes leaving Preview and coming back land where the author
// left — the standalone effect below cannot do it, because it runs while the
// host is still empty and the browser clamps an offset with nothing to scroll.
function restoreViewerScroll(host: HTMLDivElement | null, view: PDFPreviewViewState) {
  if (!host) return
  host.scrollTop = view.scrollTop
  host.scrollLeft = view.scrollLeft
}

export type PDFPreviewViewerProps = Readonly<{ bytes: ArrayBuffer; label: string; describedBy: string; state: PDFPreviewViewState; onStateChange: (state: PDFPreviewViewState) => void; onError: (error: Error) => void; onPageCount: (pages: number) => void }>

export function PDFPreviewViewer({ bytes, label, describedBy, state, onStateChange, onError, onPageCount }: PDFPreviewViewerProps) {
  const host = useRef<HTMLDivElement>(null)
  const active = useRef<ActiveDocument | undefined>(undefined)
  const page = state.page
  const scale = state.scale
  const fit = state.fit
  // THE LIVE VIEW STATE, WHICH IS NOT THE RENDER EFFECT'S CLOSURE (DW-191).
  //
  // The effect below no longer lists `state` among its dependencies, so its
  // closure over `state` is stale the moment anything the effect does not
  // depend on changes — a scroll, most of all. Every write the effect makes
  // spreads THIS ref instead, so a page clamp firing after the author has
  // scrolled keeps the scroll rather than reverting it. The ref is refreshed by
  // an effect declared FIRST, so it is already current when the render effect
  // runs on the same commit.
  const live = useRef(state)
  // THE CALLBACKS ARE NOTIFICATION CHANNELS, NOT RENDER INPUTS (DW-191, part
  // two). Rendering CALLS these three; it is not DRIVEN by them, so their
  // identity has no business deciding whether to destroy and re-rasterize a
  // document. `App.tsx:2148` wraps two of them in inline arrows to bind
  // `preview.token`, which makes them fresh on EVERY App render — so leaving
  // them in the dependency array below meant a scroll still re-ran the effect,
  // disposed the PDFDocumentProxy and zeroed the scroll, exactly as before the
  // `state` fix. Holding them here makes the property hold for ANY caller
  // rather than resting on a `useCallback` discipline at each call site.
  //
  // The refresh runs in the SAME effect as `live`, declared before the render
  // effect. React runs every cleanup for a commit before any effect body, so a
  // superseded render is already `cancelled` by the time these point at the new
  // props — the stale-render guards below keep their meaning.
  const notify = useRef({ onStateChange, onError, onPageCount })
  useEffect(() => { live.current = state; notify.current = { onStateChange, onError, onPageCount } })

  useEffect(() => {
    let cancelled = false
    const dispose = async () => {
      const current = active.current
      active.current = undefined
      current?.task?.cancel()
      current?.canvas?.remove()
      try { await current?.document?.destroy() } catch { /* cancellation is expected */ }
      try { await current?.loading.destroy() } catch { /* cancellation is expected */ }
    }
    const render = async () => {
      await dispose()
      // Referencing the generated map keeps every support byte in the release
      // set. The two URLs are local, content-addressed directories whose
      // canonical filenames PDF.js appends itself.
      void pdfjsRuntimeAssets
      const pdfjs = await import('pdfjs-dist/build/pdf.mjs')
      pdfjs.GlobalWorkerOptions.workerSrc = workerUrl
      if (cancelled) return
      const loading = pdfjs.getDocument({ data: new Uint8Array(bytes.slice(0)), disableAutoFetch: true, disableStream: true, stopAtErrors: true, isEvalSupported: false, useWorkerFetch: false, cMapUrl: pdfjsViewerAssets.cMapUrl, cMapPacked: pdfjsViewerAssets.cMapPacked, standardFontDataUrl: pdfjsViewerAssets.standardFontDataUrl })
      const current: ActiveDocument = { loading }
      active.current = current
      try {
        const document = await loading.promise
        if (cancelled || active.current !== current) { await document.destroy(); return }
        current.document = document
        notify.current.onPageCount(document.numPages)
        const safePage = Math.min(Math.max(1, page), document.numPages)
        const pdfPage = await document.getPage(safePage)
        if (cancelled || active.current !== current) return
        // THE FIT RESOLVES BEFORE THE RASTER, NOT AFTER IT. Resolving here
        // means one rasterization at the right size rather than one at the old
        // scale followed by a second at the new one, and it is re-resolved for
        // whichever page is now showing — which is what makes a fit survive a
        // page change onto a page of a different size.
        const intrinsic = pdfPage.getViewport({ scale: 1 })
        const resolved = (fit ? fitPreviewScale(fit, { width: intrinsic.width, height: intrinsic.height }, measuredViewerBox(host.current)) : undefined) ?? scale
        // ONE WRITE, NOT TWO. A clamp and a fit resolution can both be due on
        // the same pass, and two separate writes would race: the second would
        // spread a `live.current` that React has not yet re-rendered with the
        // first, silently reverting it.
        const corrected: { page?: number; scale?: number } = {}
        if (safePage !== page) corrected.page = safePage
        if (resolved !== scale) corrected.scale = resolved
        if (corrected.page !== undefined || corrected.scale !== undefined) notify.current.onStateChange({ ...live.current, ...corrected })
        // The page is rasterized at a FIXED multiple of the size it is
        // shown at, and displayed at that shown size. A canvas with no CSS
        // size is painted one bitmap pixel per CSS pixel, which a display
        // with more device pixels than CSS pixels — every Retina-class
        // screen — then magnifies: the preview looked soft next to the
        // crisp chrome around it. The multiplier is a CONSTANT, never
        // read from the display: the display's own pixel ratio is banned by
        // name (AD-17, Story 5.9's contract, whose scanner reads comments
        // too), and a constant keeps the raster identical on
        // every machine — this is image fidelity, not geometry, and no
        // command, coordinate or document value is derived from it. The fit
        // scale above is the opposite case and is why the exception exists.
        const shown = pdfPage.getViewport({ scale: resolved })
        const viewport = pdfPage.getViewport({ scale: resolved * previewOversample })
        const canvas = window.document.createElement('canvas')
        canvas.width = Math.ceil(viewport.width); canvas.height = Math.ceil(viewport.height)
        canvas.style.width = `${Math.ceil(shown.width)}px`; canvas.style.height = `${Math.ceil(shown.height)}px`
        canvas.setAttribute('aria-label', `${label}, page ${safePage} of ${document.numPages}; visual PDF canvas`)
        const context = canvas.getContext('2d')
        if (!context) throw new Error('PDF canvas is unavailable')
        current.canvas = canvas
        host.current?.replaceChildren(canvas)
        restoreViewerScroll(host.current, live.current)
        current.task = pdfPage.render({ canvasContext: context, viewport })
        await current.task.promise
      } catch (reason) {
        if (!cancelled && active.current === current) notify.current.onError(reason instanceof Error ? reason : new Error('PDF preview could not be rendered'))
      }
    }
    void render()
    return () => { cancelled = true; void dispose() }
    // DW-191: `state` is NOT here, and that is the whole fix. It was a fresh
    // object literal on every `onStateChange({ ...state, … })`, so a single
    // horizontal scroll of a zoomed page tore the `PDFDocumentProxy` down and
    // re-rasterized the PDF. What rendering actually reads is listed instead —
    // and ONLY that. The three callbacks were listed here too, and App hands two
    // of them fresh on every render, so the tear-down survived the first fix;
    // they are reached through `notify` above instead.
  }, [bytes, label, page, scale, fit])

  useEffect(() => {
    if (!host.current) return
    host.current.scrollTop = state.scrollTop
    host.current.scrollLeft = state.scrollLeft
  }, [state.scrollLeft, state.scrollTop])

  // THE PAGE AREA CARRIES THE PAGE AND NOTHING ELSE (Story 13.2 / AC6). The
  // stepper, the page indicator and the zoom now live in the application's
  // bottom status bar, which is outside both `<main>`s and is where the design
  // puts them; App owns the view state already, so this is a move, not a lift.
  return <section className="pdf-preview" aria-label={label} aria-describedby={describedBy}>
    <div className="pdf-preview-scroll" ref={host} role="img" aria-label={label} aria-describedby={describedBy} tabIndex={0} onScroll={(event) => onStateChange({ ...live.current, scrollTop: event.currentTarget.scrollTop, scrollLeft: event.currentTarget.scrollLeft })} />
  </section>
}
