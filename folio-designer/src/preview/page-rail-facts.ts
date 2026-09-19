import type { PDFThumbnailView, ThumbnailEventBus, ThumbnailLinkService, ThumbnailRenderingQueue, ThumbnailViewport } from '../vendor/pdfjs/pdf_thumbnail_view.js'

// THE PAGE RAIL'S NON-COMPONENT HALF, IN A `.ts` AND NOT IN THE `.tsx`.
// `oxlint`'s `react(only-export-components)` fires the moment a `.tsx` exports a
// value that is not a component, and the designer's baseline is exactly four of
// those warnings. `evidence-rail.tsx` / `evidence-rail-facts.ts` set the
// precedent one directory up; this is the same split.

// THE TRUNCATION BOUND. A thumbnail row is 62px plus an 8px gap = 70px, so
// twelve is 840px of rail — enough to fill a typical viewport without
// truncating most documents. It is a fixed COUNT rather than a fit because the
// rail may not measure its own height: browser measurement is banned outside
// the one named seam in `pdf-viewer.tsx`. It is also a rasterisation budget —
// each entry is one `pdfPage.render` call at preview open.
//
// Change THIS CONSTANT, never a call site: `railEntries` takes the bound as an
// argument only so the tests can drive the boundary rows directly.
export const PAGE_RAIL_BOUND = 12

export type PageRailEntry = Readonly<{ page: number; current: boolean }>

// `remaining` is `undefined`, not `0`, when nothing was truncated. The rail
// renders the `… N more` line if and only if this field is present, so the
// "exactly at the bound" row of the matrix is settled by the type rather than
// by a `> 0` test at the render site that a later edit could drop.
export type PageRailEnumeration = Readonly<{ entries: readonly PageRailEntry[]; remaining?: number }>

// THE RAIL ENUMERATES; IT DOES NOT REMEMBER. `currentPage` arrives from
// `App.tsx`'s `previewViewState`, the single page-state authority, on every
// render. Nothing here stores it, and `current` is a pure function of the two
// arguments — which is what makes re-rendering with a new current page move the
// marking with no rail-internal state involved.
//
// A page beyond the bound marks NOTHING as current, deliberately: the status
// bar can still navigate there, and a rail that silently marked page 12 while
// the viewer showed page 30 would be lying about where the author is.
export function railEntries(pages: number | undefined, currentPage: number, bound: number = PAGE_RAIL_BOUND): PageRailEnumeration {
  if (pages === undefined || pages < 1 || bound < 1) return { entries: [] }
  const shown = Math.min(pages, bound)
  const entries: PageRailEntry[] = []
  for (let page = 1; page <= shown; page++) entries.push({ page, current: page === currentPage })
  return pages > bound ? { entries, remaining: pages - bound } : { entries }
}

export function railTruncationLabel(remaining: number): string {
  return `… ${remaining} more`
}

// THE THREE COLLABORATORS `PDFThumbnailView` ACTUALLY TOUCHES, and nothing
// else. The vendored class reaches outside itself at exactly three sites; these
// are folio8's answers to them, standing in for a pdf.js viewer application
// folio8 does not have and is not going to acquire.

// Nothing competes for the rasteriser here — the rail draws every entry it
// shows, once, and there is no scroll-driven priority to arbitrate — so the
// queue always says yes and `draw` never parks itself in `PAUSED`.
export const railRenderingQueue: ThumbnailRenderingQueue = { isHighestPriority: () => true }

// A SINK, NOT A BUS. `thumbnailrendered` is how pdf.js's own viewer tells its
// other components a thumbnail finished; folio8 has no such components, and the
// rail learns the same fact from `draw`'s promise. Swallowing the event keeps
// the vendored file unmodified at that site.
export const railEventBus: ThumbnailEventBus = { dispatch: () => undefined }

// Read once, for a localised page label folio8 does not render (it supplies its
// own accessible name). The count still has to be right, because a wrong one
// would reach the `data-l10n-args` attribute in the DOM.
export function railLinkService(pages: number): ThumbnailLinkService {
  return { pagesCount: pages }
}

// The viewport the constructor needs before a page is attached. `setPdfPage`
// replaces it with the real page's viewport immediately afterwards, so this is
// only what the placeholder box is shaped like — and the design's placeholder
// is 44×62, so that is what it says.
export const railPlaceholderViewport: ThumbnailViewport = { width: 44, height: 62, rotation: 0 }

// FOLIO8'S OWN ACCESSIBILITY AND PRESENTATION RULES, HANDED TO A DOM PDF.JS BUILT
// FOR A DIFFERENT APPLICATION.
//
// The vendored class builds `div.thumbnailImageContainer[role=button][tabindex]`
// because in pdf.js that element IS the control. Here the control is folio8's own
// `<button>` wrapping it, so a second button role inside the first would be
// invalid content, a duplicate in every `getAllByRole('button')`, and a second
// element carrying `aria-current`. The host span is `aria-hidden`, which removes
// the role and the ARIA state from the accessibility tree; the three lines below
// remove what `aria-hidden` cannot.
//
// ⚠ `ariaCurrent` IS REFLECTED, AND `false` REFLECTS AS THE STRING "false".
// Measured in jsdom 28.1.0 and true of every browser that implements ARIA
// reflection: `toggleCurrent(true)` sets `imageContainer.ariaCurrent = 'page'`,
// which writes `aria-current="page"` ON A SECOND ELEMENT inside folio8's already
// `aria-current` button — and `toggleCurrent(false)` writes
// `aria-current="false"`, so EVERY entry's container matches an `[aria-current]`
// selector, not just the current one. Only `null` removes the attribute. Without
// this line the shipped current entry carries two `aria-current="page"` nodes.
//
// The focus stop goes for the same reason: `toggleCurrent(true)` sets
// `tabIndex = 0`, and a focusable element inside an `aria-hidden` subtree is
// reachable by Tab while invisible to the accessibility tree.
//
// The inline height is cleared for a presentation reason: `#updateDims` sets
// `imageContainer.style.height` from pdf.js's fixed 126px thumbnail width, and
// an inline style cannot be overridden by the rail's 44×62 rule in `App.css`
// without `!important`, which appears nowhere in this codebase.
//
// ⚠ THIS MUST RUN AFTER EVERY `toggleCurrent` AND EVERY `reset`, because those
// are the two methods that write the three properties back.
export function railThumbnailChrome(view: PDFThumbnailView): void {
  view.imageContainer.tabIndex = -1
  view.imageContainer.ariaCurrent = null
  view.imageContainer.style.height = ''
}
