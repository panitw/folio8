import { describe, expect, it } from 'vitest'
import { PAGE_RAIL_BOUND, railEntries, railEventBus, railLinkService, railPlaceholderViewport, railRenderingQueue, railThumbnailChrome, railTruncationLabel } from './page-rail-facts'
import type { PDFThumbnailView } from '../vendor/pdfjs/pdf_thumbnail_view.js'

describe('the page rail enumerates a render', () => {
  it('numbers one entry per page, in page order', () => {
    expect(railEntries(5, 1).entries.map((entry) => entry.page)).toEqual([1, 2, 3, 4, 5])
    expect(railEntries(5, 1).remaining).toBeUndefined()
  })

  it('marks exactly the current page, and marks it from the argument alone', () => {
    const marked = (currentPage: number) => railEntries(5, currentPage).entries.filter((entry) => entry.current).map((entry) => entry.page)
    expect(marked(3)).toEqual([3])
    expect(marked(1)).toEqual([1])
    // THE SAME CALL, TWICE, WITH A DIFFERENT CURRENT PAGE. Nothing here
    // accumulates, so the marking is a pure function of the arguments — which is
    // the property `previewViewState` being the single page-state authority
    // rests on.
    expect(marked(5)).toEqual([5])
    expect(marked(3)).toEqual([3])
  })

  it('bounds a long document and says how much it left out', () => {
    const long = railEntries(34, 1, 12)
    expect(long.entries).toHaveLength(12)
    expect(long.entries.at(-1)?.page).toBe(12)
    expect(long.entries.map((entry) => entry.page)).not.toContain(13)
    expect(long.remaining).toBe(22)
    expect(railTruncationLabel(long.remaining as number)).toBe('… 22 more')
  })

  // THE TWO ROWS EITHER SIDE OF THE BOUND, which is where an off-by-one lives.
  it('says nothing at all at the bound, and says one at the bound plus one', () => {
    expect(railEntries(12, 1, 12).entries).toHaveLength(12)
    expect(railEntries(12, 1, 12).remaining).toBeUndefined()
    expect(railEntries(13, 1, 12).entries).toHaveLength(12)
    expect(railEntries(13, 1, 12).remaining).toBe(1)
    expect(railTruncationLabel(1)).toBe('… 1 more')
  })

  it('handles a one-page document and a render that has not reported yet', () => {
    expect(railEntries(1, 1).entries.map((entry) => entry.page)).toEqual([1])
    expect(railEntries(1, 1).remaining).toBeUndefined()
    // No page count yet: no entries and no truncation — never a thumbnail for a
    // page that does not exist.
    expect(railEntries(undefined, 1)).toEqual({ entries: [] })
    expect(railEntries(0, 1)).toEqual({ entries: [] })
  })

  it('marks nothing when the author is on a page the bound left out', () => {
    const long = railEntries(34, 30, 12)
    expect(long.entries).toHaveLength(12)
    expect(long.entries.filter((entry) => entry.current)).toEqual([])
    // The rail is not the route to page 30 — the status bar is — so showing no
    // marking is the honest answer rather than marking page 12.
    expect(long.remaining).toBe(22)
  })

  it('keeps the bound in one constant', () => {
    expect(PAGE_RAIL_BOUND).toBe(12)
    // The default argument IS the constant, so a revision is a one-line change
    // rather than a hunt through call sites.
    expect(railEntries(34, 1).entries).toHaveLength(PAGE_RAIL_BOUND)
    expect(railEntries(34, 1).remaining).toBe(34 - PAGE_RAIL_BOUND)
  })
})

describe('the collaborators the vendored thumbnail view reaches for', () => {
  it('never parks a draw, and reports the real page count', () => {
    expect(railRenderingQueue.isHighestPriority({} as PDFThumbnailView)).toBe(true)
    expect(railLinkService(34).pagesCount).toBe(34)
    expect(railLinkService(1).pagesCount).toBe(1)
    expect(railPlaceholderViewport).toEqual({ width: 44, height: 62, rotation: 0 })
  })

  // THE EVENT BUS IS A SINK, AND "IS A SINK" IS A CLAIM ABOUT WHAT IT DOES NOT
  // DO. Asserting that a void function returns `undefined` proves only that it
  // is void; what matters is that `thumbnailrendered` goes NOWHERE — pdf.js's
  // viewer uses this bus to drive other components, and folio8 has none, so a bus
  // that grew a listener or a queue would be an unowned side channel out of the
  // rasterisation pass. (`page-rail.test.tsx` carries the other half: rasterising
  // a whole document never calls the navigation funnel.)
  it('dispatches nowhere and accumulates nothing', () => {
    const event = { source: {} as PDFThumbnailView, pageNumber: 1, pdfPage: undefined }
    // One method, no listener registry, no queue to inspect afterwards.
    expect(Object.keys(railEventBus)).toEqual(['dispatch'])
    const before = Object.getOwnPropertyNames(railEventBus)
    for (let page = 1; page <= 20; page++) railEventBus.dispatch('thumbnailrendered', { ...event, pageNumber: page })
    // Twenty events later the object is the same shape it started as: nothing was
    // recorded, buffered or subscribed.
    expect(Object.getOwnPropertyNames(railEventBus)).toEqual(before)
    expect(JSON.stringify(railEventBus)).toBe('{}')
  })

  it('takes the pdf.js chrome back off the DOM folio8 wraps in its own button', () => {
    const imageContainer = document.createElement('div')
    imageContainer.tabIndex = 0
    imageContainer.style.height = '178px'
    railThumbnailChrome({ imageContainer } as PDFThumbnailView)
    // Not focusable inside an `aria-hidden` subtree, and not 178px tall inside a
    // 62px well — the two facts pdf.js sets for a viewer application folio8 does
    // not have.
    expect(imageContainer.tabIndex).toBe(-1)
    expect(imageContainer.style.height).toBe('')
  })
})
