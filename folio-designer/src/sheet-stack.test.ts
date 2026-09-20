import { describe, expect, it } from 'vitest'
import type { CanvasProjection } from './engine-protocol'
import { MAX_CANVAS_SHEETS, columnEdgeAfterDrag, columnForStackY, sheetPitch, sheetStack, stackYForColumn } from './sheet-stack'

// The page-count matrix's own geometry, so every number below is one the
// engine really produces: A4 portrait, margins {30, 54, 42, 36}, page header
// 18pt, page footer 24pt, and therefore a content window of
// 841890 − 30000 − 42000 − 18000 − 24000 = 727890 millipoints.
const WINDOW = 727890
const CONTENT_TOP = 48000
const component = (id: string, y: number, height = 24_000): CanvasProjection['components'][number] => ({ id, type: 'text', band: 'content', x: 0, y, width: 72_000, height, resizable: true })
const canvas = (patch: Partial<CanvasProjection>): CanvasProjection => ({
  width: 595276, height: 841890, orientation: 'portrait', preset: 'A4', locale: 'en', utcOffset: '+00:00', embedFonts: true,
  marginTop: 30_000, marginRight: 54_000, marginBottom: 42_000, marginLeft: 36_000,
  gridIncrement: 6000, commandWidth: 595276, commandHeight: 841890,
  fontFamilies: ['body'], fontChains: [{ name: 'body', entries: ['Noto Sans'] }], defaultFontSize: 12_000, defaultLineSpacing: 1_000,
  contentWindowHeight: WINDOW, contentWindowCount: 1, contentWindowOrigins: [0], contentWindowPages: [0], contentWindowCountIsExact: true,
  bands: [
    { name: 'pageHeader', x: 36_000, y: 30_000, width: 505_276, height: 18_000 },
    { name: 'content', x: 36_000, y: CONTENT_TOP, width: 505_276, height: WINDOW },
    { name: 'pageFooter', x: 36_000, y: 775_890, width: 505_276, height: 24_000 },
  ],
  components: [],
  ...patch,
}) as CanvasProjection

// The projection's own answer for the Story 7.5 control fixture: three
// elements a round 728pt apart. The closed form would answer
// [0, 727890, 1455780].
const control = { contentWindowCount: 3, contentWindowOrigins: [0, 728_000, 1_456_000], contentWindowPages: [0, 0, 0] }
// A column of ordinary prose: each window begins at the top of the first line
// that did not fit, which is a little SHORT of the previous window's foot.
const prose = { contentWindowCount: 3, contentWindowOrigins: [0, 715_000, 1_430_000], contentWindowPages: [0, 0, 0] }

describe('sheet stack model', () => {
  it('draws one sheet per projected window, at the projected origin', () => {
    const stack = sheetStack(canvas(control))
    expect(stack.sheets.map((sheet) => sheet.index)).toEqual([0, 1, 2])
    expect(stack.sheets.map((sheet) => sheet.origin)).toEqual([0, 728_000, 1_456_000])
    expect(stack.windowCount).toBe(3)
    expect(stack.truncated).toBe(false)
  })

  it('puts the seam where the NEXT window begins, and nowhere when that is past the foot', () => {
    // Ordinary prose: the next window begins inside this sheet, and the space
    // below the marker is the leftover the engine will not use.
    expect(sheetStack(canvas(prose)).sheets.map((sheet) => sheet.seam)).toEqual([715_000, 715_000, undefined])
    // The control fixture, where the elements sit 728000 apart in a 727890
    // window: the next window begins 110 millipoints PAST this sheet's own
    // foot, so there is no in-sheet marker and the band foot is the boundary.
    expect(sheetStack(canvas(control)).sheets.map((sheet) => sheet.seam)).toEqual([undefined, undefined, undefined])
    // A declared ten-window gap: two windows, and the skipped column region
    // between them is drawn by nobody.
    const gap = sheetStack(canvas({ contentWindowCount: 2, contentWindowOrigins: [0, 7_280_000], contentWindowPages: [0, 0] }))
    expect(gap.sheets.map((sheet) => sheet.seam)).toEqual([undefined, undefined])
  })

  it('RED PROOF: the closed form puts the seam somewhere else, on the very fixture that hides the count', () => {
    // Substituting the window height multiplied by an index for the projected origin —
    // the spelling paginate.go forbids by name — on the control fixture,
    // where the COUNT is three either way. The seams disagree: the closed
    // form draws a marker at the foot of every sheet; the engine draws none.
    const closedForm = canvas({ contentWindowCount: 3, contentWindowOrigins: [0, WINDOW, 2 * WINDOW], contentWindowPages: [0, 0, 0] })
    expect(sheetStack(closedForm).sheets.map((sheet) => sheet.seam)).toEqual([WINDOW, WINDOW, undefined])
    expect(sheetStack(closedForm).sheets.map((sheet) => sheet.seam)).not.toEqual(sheetStack(canvas(control)).sheets.map((sheet) => sheet.seam))
    // And on the gap fixture it is not even the same number of sheets.
    expect(sheetStack(canvas({ contentWindowCount: 11, contentWindowOrigins: Array.from({ length: 11 }, (_value, index) => index * WINDOW), contentWindowPages: Array.from({ length: 11 }, () => 0) })).sheets).toHaveLength(11)
    expect(sheetStack(canvas({ contentWindowCount: 2, contentWindowOrigins: [0, 7_280_000], contentWindowPages: [0, 0] })).sheets).toHaveLength(2)
  })

  it('draws a component on every window it intersects, with exactly one home', () => {
    // e2 begins in window one and runs 400pt past the seam into window two.
    const spanning = sheetStack(canvas({ ...prose, components: [component('e1', 0), component('e2', 600_000, 400_000), component('e3', 1_450_000)] }))
    expect(spanning.sheets.map((sheet) => sheet.content.map((occurrence) => occurrence.component.id))).toEqual([['e1', 'e2'], ['e2'], ['e3']])
    // The in-sheet Y is the column offset MINUS this window's origin — the
    // second occurrence is drawn above its own sheet's content top, which is
    // what makes the run continuous across the seam.
    expect(spanning.sheets.map((sheet) => sheet.content.map((occurrence) => occurrence.y))).toEqual([[0, 600_000], [600_000 - 715_000], [1_450_000 - 1_430_000]])
    // Exactly one occurrence of e2 is the home, and it is the window its own
    // top falls in. Two homes would be two identical accessible names for one
    // component.
    const homes = spanning.sheets.flatMap((sheet) => sheet.content.filter((occurrence) => occurrence.home).map((occurrence) => `${occurrence.component.id}@${sheet.index}`))
    expect(homes).toEqual(['e1@0', 'e2@0', 'e3@2'])
  })

  it('gives a component the engine never paginated a home anyway, rather than losing it', () => {
    // A text element whose font chain will not resolve contributes no extents,
    // so no window was ever opened at its top. It is still a component the
    // author has to be able to select.
    const orphan = sheetStack(canvas({ contentWindowCount: 2, contentWindowOrigins: [0, 7_280_000], contentWindowPages: [0, 0], components: [component('e9', 3_000_000)] }))
    expect(orphan.sheets.flatMap((sheet) => sheet.content.map((occurrence) => `${occurrence.component.id}@${sheet.index}:${occurrence.home}`))).toEqual(['e9@0:true'])
    // And it is drawn WITHIN that sheet. Its own top is 3_000_000, which is
    // past the foot of a 727_890 window, so an unclamped offset put it outside
    // the band — clipped out of sight by .band-window on a stacked canvas, i.e.
    // exactly the loss this test is named for, one layer further down.
    expect(orphan.sheets[0]?.content[0]?.y).toBe(WINDOW - 24000)
  })

  it('draws the first budgeted sheets and says the value was larger', () => {
    const many = MAX_CANVAS_SHEETS + 40
    const stack = sheetStack(canvas({ contentWindowCount: many, contentWindowOrigins: Array.from({ length: many }, (_value, index) => index * 700_000), contentWindowPages: Array.from({ length: many }, () => 0) }))
    expect(stack.sheets).toHaveLength(MAX_CANVAS_SHEETS)
    expect(stack.windowCount).toBe(many)
    expect(stack.truncated).toBe(true)
    // The budget truncates the DRAWING and never the value: the last drawn
    // sheet is still at its own projected origin.
    expect(stack.sheets[MAX_CANVAS_SHEETS - 1]?.origin).toBe((MAX_CANVAS_SHEETS - 1) * 700_000)
  })

  it('carries the honesty claim through from the projection rather than deciding it', () => {
    // Both directions, because the sense of this field inverted and a
    // carried-through boolean is the easiest thing in the world to invert by
    // accident on the way through.
    expect(sheetStack(canvas({ contentWindowCountIsExact: true })).isExact).toBe(true)
    expect(sheetStack(canvas({ contentWindowCountIsExact: false })).isExact).toBe(false)
  })
})

describe('sheet stack display-space inverse', () => {
  const model = sheetStack(canvas(prose))
  const projection = canvas(prose)

  it('is the pitch of one whole page plus the stack own gap, at every zoom', () => {
    expect(sheetPitch(projection, 1)).toBe(841_890 + 24_000)
    // The gap is a CSS length, so it grows in document space as the canvas
    // shrinks: at half zoom, 24 screen pixels are 48 points of document.
    expect(sheetPitch(projection, 0.5)).toBe(841_890 + 48_000)
  })

  // The expected window is written out rather than re-derived from the
  // function under test: a round trip that computed its own answer with the
  // same rule would agree with any rule at all.
  it.each([1, 0.5, 1.7])('round-trips a column offset through its own sheet at zoom %s', (zoom) => {
    for (const [columnY, window] of [[0, 0], [1_000, 0], [714_999, 0], [715_000, 1], [1_000_000, 1], [1_430_000, 2], [2_100_000, 2]] as const) {
      const stackY = stackYForColumn(model, projection, zoom, columnY)
      expect(columnForStackY(model, projection, zoom, stackY)).toEqual({ window, columnY })
    }
  })

  it('keeps a drag tracking the hand across a seam instead of drifting by the chrome', () => {
    // A component at the foot of window one, dragged down by the height of one
    // whole sheet-plus-gap. Tracking the hand means it lands one WINDOW lower
    // in the column — not one page-plus-gap lower, which is what a linear
    // pixel delta would have proposed.
    const pitch = sheetPitch(projection, 1)
    expect(columnEdgeAfterDrag(model, projection, 1, 700_000, pitch)).toBe(700_000 + 715_000)
    // RED PROOF of the same gesture without the inverse: the linear delta the
    // drag used to apply lands the component 149,890 millipoints — a footer,
    // a gap and a header — below the hand.
    expect(700_000 + pitch).not.toBe(700_000 + 715_000)
    expect(700_000 + pitch - (700_000 + 715_000)).toBe(150_890)
  })

  it('RED PROOF: the closed form lands the same drag on the wrong column offset', () => {
    const closedForm = sheetStack(canvas({ contentWindowCount: 3, contentWindowOrigins: [0, WINDOW, 2 * WINDOW], contentWindowPages: [0, 0, 0] }))
    const pitch = sheetPitch(projection, 1)
    expect(columnEdgeAfterDrag(closedForm, projection, 1, 700_000, pitch)).not.toBe(columnEdgeAfterDrag(model, projection, 1, 700_000, pitch))
  })

  it('does not move a component the pointer never moved, including one the engine never paginated', () => {
    // THE INVARIANT THE DRAG RESTS ON: zero travel commits the offset the
    // component already had. Asserted on a drawn point of every sheet, and on
    // the one kind of point that is NOT drawn — a column offset in the region
    // a declared gap skips, where a component the engine never paginated can
    // sit. That case used to answer 9_414_110 for a component at 3_000_000:
    // stackYForColumn placed it past its sheet's foot and columnForStackY then
    // floored it onto the next sheet and added that sheet's origin, so a click
    // that moved nothing committed a move of more than nine windows.
    const gap = canvas({ contentWindowCount: 2, contentWindowOrigins: [0, 7_280_000], contentWindowPages: [0, 0] })
    const gapStack = sheetStack(gap)
    for (const drawn of [0, 100_000, 700_000, 7_280_000, 7_400_000]) expect(columnEdgeAfterDrag(gapStack, gap, 1, drawn, 0)).toBe(drawn)
    // Not drawn, so it is shown and dragged against its own sheet's foot —
    // never teleported onto a later one.
    expect(columnEdgeAfterDrag(gapStack, gap, 1, 3_000_000, 0)).toBe(WINDOW)
    for (const zoom of [1, 0.5, 1.7]) for (const drawn of [0, 300_000, 715_000, 1_000_000, 1_430_000]) expect(columnEdgeAfterDrag(model, projection, zoom, drawn, 0)).toBe(drawn)
  })

  it('never proposes a sheet the stack does not draw', () => {
    const pitch = sheetPitch(projection, 1)
    expect(columnForStackY(model, projection, 1, -10_000).window).toBe(0)
    expect(columnForStackY(model, projection, 1, 99 * pitch).window).toBe(model.sheets.length - 1)
    // Above the first sheet's content band the offset goes negative, which is
    // a proposal the drag clamp floors at zero and Go validates regardless.
    expect(columnForStackY(model, projection, 1, 0).columnY).toBe(-CONTENT_TOP)
  })
})

// SPEC-multi-pages story 2: each designed page is its own group of sheets.
// Origins are page-local, so a component is homed, echoed and mapped among its
// own page's windows only, and a seam never crosses a page boundary.
describe('sheet stack across designed pages', () => {
  const onPage = (page: number, id: string, y: number, height = 24_000) => ({ ...component(id, y, height), page })
  const twoPages = canvas({
    contentWindowCount: 3, contentWindowOrigins: [0, 700_000, 0], contentWindowPages: [0, 0, 1], pageBreaks: [true, true],
    components: [onPage(0, 'a', 100_000), onPage(0, 'c', 690_000, 40_000), onPage(1, 'b', 710_000)],
  })

  it('groups sheets by page and marks each page start', () => {
    const stack = sheetStack(twoPages)
    expect(stack.sheets.map((sheet) => [sheet.page, sheet.pageStart])).toEqual([[0, true], [0, false], [1, true]])
  })

  it('homes and echoes a component only on its own page sheets', () => {
    const stack = sheetStack(twoPages)
    const ids = stack.sheets.map((sheet) => sheet.content.map((occurrence) => `${occurrence.component.id}${occurrence.home ? '' : '~'}`))
    // b's page-local y of 710000 would fall in page 1's second window; it is
    // page 2's, so it is drawn on page 2's sheet alone.
    expect(ids).toEqual([['a', 'c'], ['c~'], ['b']])
    expect(stack.sheets[2]!.content[0]!.y).toBe(710_000)
  })

  it('never draws a seam across a page boundary', () => {
    expect(sheetStack(twoPages).sheets.map((sheet) => sheet.seam)).toEqual([700_000, undefined, undefined])
  })

  it('keeps one page of the same geometry exactly as before', () => {
    const one = sheetStack(canvas({ ...prose, components: [component('a', 100_000)] }))
    expect(one.sheets.map((sheet) => [sheet.page, sheet.pageStart])).toEqual([[0, true], [0, false], [0, false]])
    expect(one.sheets.map((sheet) => sheet.seam)).toEqual([715_000, 715_000, undefined])
  })

  it('truncates the tail of the whole stack, across pages', () => {
    const many = MAX_CANVAS_SHEETS + 2
    const stack = sheetStack(canvas({ contentWindowCount: many, contentWindowOrigins: Array.from({ length: many }, (_value, index) => index < MAX_CANVAS_SHEETS ? index * 700_000 : 0), contentWindowPages: Array.from({ length: many }, (_value, index) => index < MAX_CANVAS_SHEETS ? 0 : 1), pageBreaks: [true, true] }))
    expect(stack.sheets).toHaveLength(MAX_CANVAS_SHEETS)
    expect(stack.windowCount).toBe(many)
    expect(stack.truncated).toBe(true)
  })

  it('maps a column offset among its own page sheets, and holds a drag to that page', () => {
    const stack = sheetStack(twoPages)
    const pitch = sheetPitch(twoPages, 1)
    expect(stackYForColumn(stack, twoPages, 1, 100_000, 1)).toBe(2 * pitch + CONTENT_TOP + 100_000)
    expect(stackYForColumn(stack, twoPages, 1, 100_000)).toBe(CONTENT_TOP + 100_000)
    // A drag from page 2 up past its top stays on page 2's sheet.
    expect(columnForStackY(stack, twoPages, 1, CONTENT_TOP, 1).window).toBe(2)
    expect(columnEdgeAfterDrag(stack, twoPages, 1, 100_000, -50_000, 1)).toBe(50_000)
  })
})
