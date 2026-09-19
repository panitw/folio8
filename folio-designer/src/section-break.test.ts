import { describe, expect, it } from 'vitest'
import type { CanvasProjection } from './engine-protocol'
import { contentBandHeight, proposedSectionBreak, sectionBreakOnPage, sectionBreakPlacement } from './section-break'

const projection = (patch: Partial<CanvasProjection>): CanvasProjection => ({
  width: 595276, height: 841890, orientation: 'portrait', preset: 'A4', locale: 'en', utcOffset: '+07:00', marginTop: 36000, marginRight: 36000, marginBottom: 36000, marginLeft: 36000, gridIncrement: 6000, commandWidth: 595276, commandHeight: 841890, fontFamilies: [], fontChains: [], defaultFontSize: 12000, defaultLineSpacing: 1000,
  contentWindowHeight: 729890, contentWindowCount: 1, contentWindowOrigins: [0], contentWindowPages: [0], contentWindowCountIsExact: true,
  bands: [{ name: 'pageHeader', x: 36000, y: 36000, width: 523276, height: 20000 }, { name: 'content', x: 36000, y: 56000, width: 523276, height: 729890 }, { name: 'pageFooter', x: 36000, y: 785890, width: 523276, height: 20000 }],
  components: [],
  ...patch,
})

describe('proposedSectionBreak', () => {
  it('follows the pointer from the original offset', () => {
    expect(proposedSectionBreak(400_000, 40_000, 729_890)).toBe(440_000)
    expect(proposedSectionBreak(400_000, -25_000, 729_890)).toBe(375_000)
  })

  it('stops one millipoint inside the content band\'s top and bottom, which the engine refuses', () => {
    expect(proposedSectionBreak(10_000, -50_000, 729_890)).toBe(1)
    expect(proposedSectionBreak(700_000, 90_000, 729_890)).toBe(729_889)
    expect(proposedSectionBreak(1, -1_000, 729_890)).toBe(1)
  })

  it('rounds a zoomed float to a whole millipoint', () => {
    expect(proposedSectionBreak(400_000, -32726.999999999996, 729_890)).toBe(367_273)
  })
})

describe('sectionBreakPlacement', () => {
  it('is absent when the projection carries no break', () => {
    expect(sectionBreakPlacement(projection({}))).toBeUndefined()
  })

  it('draws the line on the one sheet whose window holds the offset', () => {
    expect(sectionBreakPlacement(projection({ sectionBreak: 400_000 }))).toEqual({ sheet: 0, y: 400_000 })
    const twoWindows = projection({ sectionBreak: 900_000, contentWindowCount: 2, contentWindowOrigins: [0, 728_000], contentWindowPages: [0, 0] })
    expect(sectionBreakPlacement(twoWindows)).toEqual({ sheet: 1, y: 172_000 })
  })

  it('reads the content band height off the projection', () => {
    expect(contentBandHeight(projection({}))).toBe(729_890)
  })
})

// SPEC-multi-pages story 2: the break is page 1's, placed among page 1's
// windows only. A later page's page-local origin of 0 must not claim it.
describe('sectionBreakPlacement across designed pages', () => {
  it('draws the line only on page 1 sheets', () => {
    expect(sectionBreakPlacement(projection({ sectionBreak: 400_000, contentWindowCount: 2, contentWindowOrigins: [0, 0], contentWindowPages: [0, 1], pageBreaks: [true, true] }))).toEqual({ sheet: 0, y: 400_000 })
    expect(sectionBreakPlacement(projection({ sectionBreak: 900_000, contentWindowCount: 3, contentWindowOrigins: [0, 728_000, 0], contentWindowPages: [0, 0, 1], pageBreaks: [true, true] }))).toEqual({ sheet: 1, y: 172_000 })
  })
})

// SPEC-multi-pages story 5: every designed page has its own break. A one-page
// projection carries the one-page pair; a multi-page one, one entry per page.
describe('sectionBreakOnPage and per-page placement', () => {
  it('reads the one-page pair for page 1 of a one-page projection only', () => {
    expect(sectionBreakOnPage(projection({ sectionBreak: 400_000 }), 0)).toEqual({ offset: 400_000, anchored: true })
    expect(sectionBreakOnPage(projection({ sectionBreak: 400_000, sectionBreakAnchor: false }), 0)).toEqual({ offset: 400_000, anchored: false })
    expect(sectionBreakOnPage(projection({ sectionBreak: 400_000 }), 1)).toBeUndefined()
    expect(sectionBreakOnPage(projection({}), 0)).toBeUndefined()
  })

  it('reads each page of a multi-page projection, and draws each page break among its own sheets', () => {
    const multi = projection({ contentWindowCount: 3, contentWindowOrigins: [0, 600_000, 0], contentWindowPages: [0, 0, 1], pageBreaks: [true, false], sectionBreaks: [650_000, 300_000], sectionBreakAnchors: [true, false] })
    expect(sectionBreakOnPage(multi, 0)).toEqual({ offset: 650_000, anchored: true })
    expect(sectionBreakOnPage(multi, 1)).toEqual({ offset: 300_000, anchored: false })
    expect(sectionBreakPlacement(multi, 0)).toEqual({ sheet: 1, y: 50_000 })
    expect(sectionBreakPlacement(multi, 1)).toEqual({ sheet: 2, y: 300_000 })
    const secondOnly = { ...multi, sectionBreaks: [null, 300_000], sectionBreakAnchors: [true, true] }
    expect(sectionBreakPlacement(secondOnly, 0)).toBeUndefined()
    expect(sectionBreakOnPage(secondOnly, 1)).toEqual({ offset: 300_000, anchored: true })
  })
})
