// Sibling declaration for the vendored `pdf_thumbnail_view.js`. See the header
// of `renderable_view.d.ts` for why this is `export declare` rather than
// `declare module`, and for the AD-17 rule this file lives under.
//
// ONLY THE COLLABORATORS THE CLASS ACTUALLY TOUCHES ARE DECLARED. The
// vendored class reaches the objects injected into it at exactly three sites —
// `renderingQueue.isHighestPriority(this)`, `eventBus.dispatch('thumbnailrendered', …)`
// and `linkService.pagesCount` — so folio8's adapters are three tiny objects
// rather than the pdf.js viewer application those names come from.
//
// `pdfPage` is `unknown` deliberately. The only page type folio8 has is the one
// `src/preview/pdfjs-dist.d.ts` declares, which is narrower than what pdf.js
// hands the thumbnail (it has no `rotate`, and its viewport has no `clone`).
// Widening that declaration to satisfy this one would put a second, looser
// description of the same runtime object in the tree; passing the page straight
// through as an opaque value does not.

import type { RenderableView } from './renderable_view.js'

// The viewport the constructor needs before a page is attached: `#updateDims`
// reads `width`/`height`, the constructor reads `rotation`, and `setPdfPage`
// replaces the whole thing with the real page's viewport immediately after.
export type ThumbnailViewport = Readonly<{ width: number; height: number; rotation: number }>

export type ThumbnailRenderingQueue = { isHighestPriority(view: PDFThumbnailView): boolean }

export type ThumbnailRenderedEvent = { source: PDFThumbnailView; pageNumber: number; pdfPage: unknown }

export type ThumbnailEventBus = { dispatch(name: string, payload: ThumbnailRenderedEvent): void }

export type ThumbnailLinkService = { pagesCount: number }

export type PDFThumbnailViewOptions = {
  container: HTMLElement
  eventBus: ThumbnailEventBus
  id: number
  defaultViewport: ThumbnailViewport
  optionalContentConfigPromise?: Promise<unknown> | null
  linkService: ThumbnailLinkService
  renderingQueue: ThumbnailRenderingQueue
  maxCanvasPixels?: number
  maxCanvasDim?: number
  pageColors?: unknown
  enableSplitMerge?: boolean
}

export declare class PDFThumbnailView extends RenderableView {
  constructor(options: PDFThumbnailViewOptions)
  readonly id: number
  // The three elements the class builds and appends to `container`. folio8 reads
  // them only to hand its own presentation and accessibility rules to a DOM
  // pdf.js built for a viewer application folio8 does not have.
  readonly div: HTMLDivElement
  readonly imageContainer: HTMLDivElement
  readonly image: HTMLImageElement
  setPdfPage(pdfPage: unknown): void
  draw(): Promise<void>
  toggleCurrent(isCurrent: boolean): void
  reset(): void
  destroy(): void
  setPageLabel(label: string | null): void
}
