import { describe, expect, it } from 'vitest'
import type { CanvasProjection } from './engine-protocol'
import { contentPageAt, enclosedComponents, selectionRectangle, translatedCanvas } from './canvas-selection'
import { MAX_CANVAS_SHEETS, sheetPitch } from './sheet-stack'

const base: CanvasProjection = {
  width: 600000, height: 800000, orientation: 'portrait', preset: 'A4', locale: 'en', utcOffset: '+00:00',
  marginTop: 30000, marginBottom: 30000, marginLeft: 40000, marginRight: 40000,
  gridIncrement: 6000, commandWidth: 600000, commandHeight: 800000, fontFamilies: [], fontChains: [], defaultFontSize: 12000, defaultLineSpacing: 1000,
  contentWindowHeight: 620000, contentWindowOrigins: [0], contentWindowPages: [0], contentWindowCount: 1, contentWindowCountIsExact: true,
  bands: [{ name: 'pageHeader', x: 40000, y: 30000, width: 520000, height: 60000 }, { name: 'content', x: 40000, y: 90000, width: 520000, height: 620000 }, { name: 'pageFooter', x: 40000, y: 710000, width: 520000, height: 60000 }], components: [],
}
const box = (id: string, x: number, y: number, type: CanvasProjection['components'][number]['type'] = 'rect', height = 20000): CanvasProjection['components'][number] => ({ id, type, band: 'content', x, y, width: 30000, height, resizable: type !== 'table' })

describe('rectangle selection geometry (AC-1, AC-2, AC-4, AC-5)', () => {
  it.each([[0, 0, 100000, 200000], [100000, 0, 0, 200000], [0, 200000, 100000, 0], [100000, 200000, 0, 0]])('normalizes direction and includes equality but excludes partial boxes: %s', (x1, y1, x2, y2) => {
    const canvas = { ...base, components: [box('e1', 0, 0), box('e2', 30000, 90000), box('e3', 40000, 95000)] }
    expect(enclosedComponents(canvas, 1, selectionRectangle({ x: x1, y: y1 }, { x: x2, y: y2 }))).toEqual(['e1', 'e2'])
  })
  it.each([0.5, 1, 1.5])('includes every overlapping kind at zoom %s without text-ink authority', (zoom) => {
    const components = (['text', 'rect', 'image', 'line', 'table', 'barcode', 'qrcode'] as const).map((kind, i) => box(`e${i}`, 1000, 1000, kind, kind === 'line' ? 1000 : 20000))
    expect(enclosedComponents({ ...base, components }, zoom, { left: 41000, top: 91000, right: 71000, bottom: 111000 })).toEqual(components.map((c) => c.id))
    expect(enclosedComponents({ ...base, components: [components[0]!] }, zoom, { left: 45000, top: 95000, right: 65000, bottom: 100000 })).toEqual([])
  })
  it('deduplicates repeated bands and requires every visible portion of split content', () => {
    const canvas = { ...base, contentWindowOrigins: [0, 600000], contentWindowPages: [0, 0], contentWindowCount: 2, components: [{ ...box('e1', 0, 0), band: 'pageHeader' as const }, box('e2', 0, 590000, 'text', 40000)] }
    const pitch = sheetPitch(canvas, 1)
    expect(enclosedComponents(canvas, 1, { left: 0, top: pitch, right: 600000, bottom: pitch + 800000 })).toEqual(['e1'])
    expect(enclosedComponents(canvas, 1, { left: 0, top: 0, right: 600000, bottom: pitch + 800000 })).toEqual(['e1', 'e2'])
  })
  it('excludes undrawn column gaps and portions beyond the drawing cap', () => {
    const gap = { ...base, contentWindowOrigins: [0, 900000], contentWindowPages: [0, 0], contentWindowCount: 2, components: [box('e1', 0, 600000, 'text', 400000)] }
    expect(enclosedComponents(gap, 1, { left: 0, top: 0, right: 600000, bottom: 2000000 })).toEqual([])
    const capped = { ...base, contentWindowOrigins: Array.from({ length: MAX_CANVAS_SHEETS + 1 }, (_, i) => i * 600000), contentWindowPages: Array.from({ length: MAX_CANVAS_SHEETS + 1 }, () => 0), contentWindowCount: MAX_CANVAS_SHEETS + 1, components: [box('e1', 0, 600000 * MAX_CANVAS_SHEETS - 10000, 'text', 30000)] }
    expect(enclosedComponents(capped, 1, { left: 0, top: 0, right: 600000, bottom: 1000000000 })).toEqual([])
  })
  it('translates paint offsets without altering dimensions, source geometry, or unselected members', () => {
    const component = { ...box('e1', 1000, 2000, 'text'), textPaint: { overflow: false, truncated: false, lines: [{ top: 3000, baseline: 14000, advance: 12000, width: 10000, fragments: [{ x: 4000, text: 'abc' }] }] } }
    const canvas = { ...base, components: [component, box('e2', 70000, 80000)] }
    const moved = translatedCanvas(canvas, ['e1'], 1125, 2227)
    expect(moved.components[0]?.textPaint?.lines[0]?.baseline).toBe(16227)
    expect(moved.components[0]?.textPaint?.lines[0]?.fragments[0]?.x).toBe(5125)
    expect(moved.components[0]?.width).toBe(component.width)
    expect(moved.components[1]).toBe(canvas.components[1])
    expect(component.y).toBe(2000)
  })
  it('keeps a fitted image offset inside its moving component and leaves cancellation geometry intact', () => {
    const component = { ...box('e1', 10000, 20000, 'image'), image: { assetKey: 'logo', mediaType: 'image/png', width: 2, height: 1, drawX: 14000, drawY: 25000, drawWidth: 20000, drawHeight: 10000 } }
    const canvas = { ...base, components: [component, box('e2', 60000, 20000)] }
    const moved = translatedCanvas(canvas, ['e1', 'e2'], 3125, 5227).components[0]!
    expect(moved.image!.drawX - moved.x).toBe(4000)
    expect(moved.image!.drawY - moved.y).toBe(5000)
    expect(moved.image!.drawWidth).toBe(20000)
    expect(moved.image!.drawHeight).toBe(10000)
    expect(canvas.components[0]).toBe(component)
    expect(component.image.drawX).toBe(14000)
    expect(component.image.drawY).toBe(25000)
  })
})

// SPEC-multi-pages story 2: a rectangle tests a content component against its
// own page's windows — page-local origins are meaningless on another page.
describe('rectangle selection across designed pages', () => {
  it('encloses a component only on its own page sheet', () => {
    const canvas: CanvasProjection = { ...base, contentWindowOrigins: [0, 0], contentWindowPages: [0, 1], contentWindowCount: 2, pageBreaks: [true, true], components: [{ ...box('e1', 0, 0), page: 0 }, { ...box('e2', 0, 0), page: 1 }] }
    const pitch = sheetPitch(canvas, 1)
    expect(enclosedComponents(canvas, 1, { left: 0, top: 0, right: 600000, bottom: 800000 })).toEqual(['e1'])
    expect(enclosedComponents(canvas, 1, { left: 0, top: pitch, right: 600000, bottom: pitch + 800000 })).toEqual(['e2'])
  })
})

// SPEC-multi-pages story 3: the preview of a move to another page, and the
// page under a point down the stack.
describe('moving to another page', () => {
  const twoPages: CanvasProjection = { ...base, contentWindowOrigins: [0, 0], contentWindowPages: [0, 1], contentWindowCount: 2, components: [{ id: 'e1', type: 'rect', band: 'content', x: 0, y: 10000, width: 20000, height: 20000, resizable: true, page: 0 }] }
  it('draws the moving components in the target page column only when a page is given', () => {
    expect(translatedCanvas(twoPages, ['e1'], 0, 5000).components[0]).toMatchObject({ y: 15000, page: 0 })
    expect(translatedCanvas(twoPages, ['e1'], 0, 190000, 1).components[0]).toMatchObject({ y: 200000, page: 1 })
  })
  it('names the page whose content band holds a stack point, and nothing off every content band', () => {
    const pitch = sheetPitch(twoPages, 1)
    expect(contentPageAt(twoPages, 1, 90000)).toEqual({ page: 0, columnY: 0 })
    expect(contentPageAt(twoPages, 1, pitch + 290000)).toEqual({ page: 1, columnY: 200000 })
    expect(contentPageAt(twoPages, 1, pitch + 50000)).toBeUndefined()
    expect(contentPageAt(twoPages, 1, 750000)).toBeUndefined()
    expect(contentPageAt(twoPages, 1, 2 * pitch + 290000)).toBeUndefined()
    expect(contentPageAt(twoPages, 1, -1)).toBeUndefined()
  })
})
