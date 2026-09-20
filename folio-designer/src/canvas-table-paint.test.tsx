import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import App, { canvasDisplay } from './App'
import { ENGINE_PROTOCOL_VERSION, MAX_CANVAS_PROPERTY_STRING, MAX_CANVAS_TABLE_LABEL_LINE_LENGTH, MAX_CANVAS_TABLE_LABEL_LINES, parseInbound, type CanvasProjection, type CanvasTableColumn } from './engine-protocol'
import type { EngineClient } from './engine-client'

const appCss = fs.readFileSync(path.join(path.dirname(fileURLToPath(import.meta.url)), 'App.css'), 'utf8')
const cssRule = (selector: string) => {
  const match = appCss.match(new RegExp(`${selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\s*\\{([^}]*)\\}`))
  expect(match, `App.css must declare a \`${selector}\` rule`).toBeTruthy()
  return (match as RegExpMatchArray)[1] as string
}

// STORY 14.9 — THE CANVAS DRAWS THE TABLE IT WILL PRINT.
//
// Every row of the story's I/O & Edge-Case Matrix, against the painted DOM and
// against the protocol guard that lets the projection through at all.
//
// ⚠ EVERY "IS NOT THERE" CLAIM IS PROVED BY ADDING THE FORBIDDEN THING, at each
// position it could occupy, and never by reverting the implementation.
// Reverting cannot falsify an absence claim — a query that finds nothing finds
// nothing whether the rule holds or the query is broken — and four false greens
// in this epic were produced exactly that way.

const canvas: CanvasProjection = { width: 595276, height: 841890, orientation: 'portrait', preset: 'A4', locale: 'en', utcOffset: '+07:00', embedFonts: true, marginTop: 36000, marginRight: 36000, marginBottom: 36000, marginLeft: 36000, gridIncrement: 6000, commandWidth: 595276, commandHeight: 841890, fontFamilies: ['body'], fontChains: [{ name: 'body', entries: [{ face: 'Noto Sans', assetKey: '', family: '', style: '', bold: '', italic: '', boldItalic: '' }] }], defaultFontSize: 12000, defaultLineSpacing: 1000, contentWindowHeight: 729890, contentWindowCount: 1, contentWindowOrigins: [0], contentWindowPages: [0], contentWindowCountIsExact: true, bands: [{ name: 'pageHeader', x: 36000, y: 36000, width: 523276, height: 20000 }, { name: 'content', x: 36000, y: 56000, width: 523276, height: 729890 }, { name: 'pageFooter', x: 36000, y: 785890, width: 523276, height: 20000 }], components: [] }

// The projection's OWN column type, imported rather than re-derived here: an
// alias nothing references cannot keep the painter and the guard naming one
// shape, which is the whole reason it is exported.
type Column = CanvasTableColumn
const column = (id: string, label: string, width: number, bind: string, headerAlign: Column['headerAlign'] = 'left', cellAlign: Column['cellAlign'] = 'left'): Column => ({ id, label, labelLines: label === '' ? [] : label.split('\n'), width, headerAlign, cellAlign, bind })

// The design's own five-column statement table, at the widths TableEditor.dc.html
// declares — NOT at Binding.dc.html's pixel tracks, which are a drawing and not
// the document (22.0 / 64.0 / 28.0 / 28.0 / 32.0 points).
const fiveColumns: ReadonlyArray<Column> = [
  column('e10', 'Date', 22_000, '{{date}}'),
  column('e11', 'Description', 64_000, '{{description}}'),
  column('e12', 'Debit', 28_000, '{{debit}}', 'right', 'right'),
  column('e13', 'Credit', 28_000, '{{credit}}', 'right', 'right'),
  column('e14', 'Balance', 32_000, '{{balance}}', 'right', 'right'),
]

const table = (patch: Partial<CanvasProjection['components'][number]> = {}) => ({ id: 'e7', type: 'table' as const, band: 'content' as const, x: 0, y: 0, width: 174_000, height: 16_000, resizable: false, tableBind: 'transactions[]', columns: fiveColumns, ...patch })
const engine = () => ({ request: vi.fn(async () => ({ snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3 } })) }) as unknown as EngineClient
const mount = (components: ReadonlyArray<CanvasProjection['components'][number]>, projection: CanvasProjection = canvas) =>
  render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...projection, components: [...components] } }} />)

const home = (container: Element) => container.querySelector('.canvas-component:not(.canvas-component-echo)[data-component-id="e7"]') as HTMLElement
const textsOf = (root: Element, selector: string) => Array.from(root.querySelectorAll(selector)).map((node) => node.textContent)
const alignsOf = (root: Element, selector: string) => Array.from(root.querySelectorAll(selector)).map((node) => (node as HTMLElement).style.textAlign)

describe('the canvas draws the table it will print', () => {
  it('draws the chip, the real header labels and one representative row from the projection', () => {
    const view = mount([table()])
    const paint = home(view.container)
    // THE CHIP: the palette's own table glyph, the bound collection in the bind
    // accent, and the column count.
    expect(paint.querySelector('.canvas-table-chip-icon')).not.toBeNull()
    expect(paint.querySelector('.canvas-table-collection')?.textContent).toBe('transactions[]')
    expect(paint.querySelector('.canvas-table-count')?.textContent).toBe('5 columns')
    // THE HEADER LABELS, in the document's declared order.
    expect(textsOf(paint, '.canvas-table-heading')).toEqual(['Date', 'Description', 'Debit', 'Credit', 'Balance'])
    // ONE ROW, showing each column's BINDING rather than a value: the canvas
    // has no data, and one row is enough to show structure.
    expect(textsOf(paint, '.canvas-table-cell')).toEqual(['{{date}}', '{{description}}', '{{debit}}', '{{credit}}', '{{balance}}'])
    expect(paint.querySelectorAll('.canvas-table-grid')).toHaveLength(1)
    // AND NOTHING SAYS "Table" ANY MORE. The word was the whole drawing before
    // this story; asserting its absence is how a half-applied change is caught.
    expect(paint.textContent).not.toContain('Table')
  })

  it('lays the tracks out at the ENGINE\'s declared widths, through the one zoom mapping', () => {
    const view = mount([table()])
    const grid = home(view.container).querySelector('.canvas-table-grid') as HTMLElement
    // Derived from the projection through canvasDisplay.css — the same helper
    // every other painted length goes through — rather than written out as
    // pixels, so a test that changed the widths and saw the same drawing would
    // be a test of nothing.
    expect(grid.style.gridTemplateColumns).toBe(fiveColumns.map((each) => canvasDisplay.css(each.width, 1)).join(' '))
    // The positive control: these are not all the same number, so the
    // assertion above can tell one column's width from another's.
    expect(new Set(fiveColumns.map((each) => each.width)).size).toBeGreaterThan(1)
  })

  it('aligns the HEADER row by headerAlign and the representative row by cellAlign — never one value twice', () => {
    // R2's case, and the one the ruling required to be written: the two
    // resolved alignments GENUINELY DIFFER on the same column. The engine
    // resolves a header cell through resolveHeaderStyle (headerStyle.align
    // first) and a data cell through resolveBodyStyle (which never sees
    // headerStyle), so a table declaring both aligns its two rows differently.
    const split = [column('e10', 'Date', 22_000, '{{date}}', 'right', 'center'), column('e11', 'Amount', 40_000, '{{amount}}', 'right', 'center')]
    const view = mount([table({ columns: split, width: 62_000 })])
    const paint = home(view.container)
    expect(alignsOf(paint, '.canvas-table-heading')).toEqual(['right', 'right'])
    expect(alignsOf(paint, '.canvas-table-cell')).toEqual(['center', 'center'])
    // ⚠ THE PRECONDITION THE WHOLE ROW RESTS ON. If the fixture ever stopped
    // splitting them, both assertions above would still pass with one value
    // used twice — the both-sides-move-together shape.
    expect(split.every((each) => each.headerAlign !== each.cellAlign)).toBe(true)
    // ⚠ AND A SWAP MUST RED THIS. Presence cannot see a swap, so the two rows
    // are asserted to disagree with each other, in the direction the engine
    // resolved them.
    expect(alignsOf(paint, '.canvas-table-heading')).not.toEqual(alignsOf(paint, '.canvas-table-cell'))
  })

  it('insets headings and cells by the table\'s declared left/right padding, and keeps the stylesheet inset when none is declared', () => {
    const view = mount([table({ paddingLeft: 4_000, paddingRight: 2_000 })])
    const paint = home(view.container)
    const heading = paint.querySelector('.canvas-table-heading') as HTMLElement
    const cell = paint.querySelector('.canvas-table-cell') as HTMLElement
    // Exact values through the one zoom mapping, each edge to its own side, so a
    // left/right swap reds this.
    for (const each of [heading, cell]) {
      expect(each.style.paddingLeft).toBe(canvasDisplay.css(4_000, 1))
      expect(each.style.paddingRight).toBe(canvasDisplay.css(2_000, 1))
    }
    view.unmount()
    const plain = mount([table()])
    const plainCell = home(plain.container).querySelector('.canvas-table-cell') as HTMLElement
    expect(plainCell.style.paddingLeft).toBe('')
    expect(plainCell.style.paddingRight).toBe('')
  })

  it('maps the declared cell padding through the zoom rule, at a zoom that is not 1', () => {
    const view = mount([table({ paddingLeft: 4_000, paddingRight: 2_000 })])
    const paint = () => view.container.querySelector('.canvas-component:not(.canvas-component-echo)[data-component-id="e7"]') as HTMLElement
    fireEvent.click(screen.getByRole('button', { name: 'Zoom in' }))
    return waitFor(() => expect(screen.getByLabelText('Canvas zoom')).toHaveTextContent('110%')).then(() => {
      const heading = paint().querySelector('.canvas-table-heading') as HTMLElement
      const cell = paint().querySelector('.canvas-table-cell') as HTMLElement
      for (const each of [heading, cell]) {
        expect(each.style.paddingLeft).toBe(canvasDisplay.css(4_000, 1.1))
        expect(each.style.paddingRight).toBe(canvasDisplay.css(2_000, 1.1))
      }
      // The positive control: a painter ignoring zoom would still print the zoom-1 string.
      expect(canvasDisplay.css(4_000, 1.1)).not.toBe(canvasDisplay.css(4_000, 1))
    })
  })

  it('says the table is bound to nothing in the muted ink, not in the bind accent', () => {
    const view = mount([table({ tableBind: '' })])
    const paint = home(view.container)
    // DESIGN.md: "amber means data, and only data." A table bound to nothing
    // has none, so the chip must NOT carry the accent class.
    expect(paint.querySelector('.canvas-table-collection')).toBeNull()
    const unset = paint.querySelector('.canvas-table-chip .canvas-table-unset')
    expect(unset?.textContent).toBe('Not set')
    // The positive control for that absence: the same query DOES find the
    // accent class when the table is bound, so a null above is the rule
    // holding rather than the selector being wrong.
    expect(mount([table()]).container.querySelector('.canvas-table-collection')).not.toBeNull()
  })

  it('reads an unbound COLUMN as unbound, and never in the bind accent', () => {
    const view = mount([table({ columns: [column('e10', 'Date', 22_000, '{{date}}'), column('e11', 'Amount', 40_000, '')], width: 62_000 })])
    const paint = home(view.container)
    const cells = Array.from(paint.querySelectorAll('.canvas-table-grid > *')).slice(2)
    expect(cells.map((cell) => cell.textContent)).toEqual(['{{date}}', 'Not set'])
    // The BOUND cell carries the accent class; the UNBOUND one carries the
    // muted class and NOT the accent one. Amber means data and an unbound cell
    // has none — AC6 asks only that it read as unbound rather than as blank.
    expect(cells[0]?.className).toContain('canvas-table-cell')
    expect(cells[1]?.className).not.toContain('canvas-table-cell')
    expect(cells[1]?.className).toContain('canvas-table-unset')
    // And it is not an EMPTY cell either, which is the thing it must not be.
    expect(cells[1]?.textContent).not.toBe('')
  })

  it('draws the header cell for an empty label and paints no text in it', () => {
    // The renderer builds the cell rect and then skips the glyphs on an empty
    // label; the canvas matches it, because a missing cell would move every
    // column after it.
    const columns = [column('e10', '', 22_000, '{{date}}'), column('e11', 'Amount', 40_000, '{{amount}}')]
    const view = mount([table({ columns, width: 62_000 })])
    const paint = home(view.container)
    const headings = Array.from(paint.querySelectorAll('.canvas-table-heading'))
    expect(headings).toHaveLength(2)
    expect(headings[0]?.textContent).toBe('')
    expect(headings[1]?.textContent).toBe('Amount')
  })

  it('counts one column in the singular', () => {
    const view = mount([table({ columns: [column('e10', 'Date', 22_000, '{{date}}')], width: 22_000 })])
    expect(home(view.container).querySelector('.canvas-table-count')?.textContent).toBe('1 column')
  })

  it('says a table has no columns yet rather than drawing an empty frame', () => {
    const { columns: _columns, ...columnless } = table()
    const view = mount([columnless])
    const paint = home(view.container)
    const empty = paint.querySelector('.canvas-table-empty')
    expect(empty?.textContent).toContain('No columns yet.')
    // The dashed-outline flip is a CSS rule keyed on this exact class, which is
    // DESIGN.md's "Dashed grey on page | A placeholder with no content yet" and
    // ImagePlaceholder's shipped idiom. jsdom applies no stylesheet, so the DOM
    // can only show the class; the RULE is asserted against App.css's text in
    // the row below — deleting that line used to leave every test green.
    expect(empty?.className).toContain('canvas-table-empty')
    // No grid, no chip: there is nothing to draw, and an empty frame would look
    // like something failed.
    expect(paint.querySelector('.canvas-table-grid')).toBeNull()
    expect(paint.querySelector('.canvas-table-chip')).toBeNull()
    // ⚠ AND IT IS NOT A CONTROL. control-vocabulary-contract.test.tsx mounts a
    // columnless table in six of its seven swept states, so a <button> here
    // would enter its R1/R4 populations and a role="group" would red three
    // separate pinned assertions there. The plants below prove these queries
    // can see what they are looking for.
    expect(empty?.querySelectorAll('button, [role="button"], [role="group"], [role="tablist"]')).toHaveLength(0)
    const planted = document.createElement('div')
    planted.innerHTML = '<button type="button">Add column</button><div role="group"></div><div role="tablist"></div>'
    empty?.appendChild(planted)
    expect(empty?.querySelectorAll('button, [role="button"], [role="group"], [role="tablist"]')).toHaveLength(3)
  })

  it('keeps the component the single control, with one accessible name and no inner one', () => {
    const view = mount([table()])
    const paint = home(view.container)
    // ONE role="button", ONE data-component-id, ONE accessible name — the
    // component itself — and nothing inside claims any of the three.
    expect(screen.getByRole('button', { name: 'table component e7' })).toBe(paint)
    expect(paint.querySelectorAll('[role="button"], button')).toHaveLength(0)
    expect(paint.querySelectorAll('[data-component-id]')).toHaveLength(0)
    expect(paint.querySelectorAll('[aria-label]')).toHaveLength(0)
    expect(paint.querySelectorAll('[role="group"], [role="tablist"]')).toHaveLength(0)
    expect(paint.querySelectorAll('[tabindex]')).toHaveLength(0)
    // No cell borrows the component's own box class, which e9-5-border-no-ink
    // enumerates exactly.
    expect(paint.querySelectorAll('.canvas-table .canvas-box')).toHaveLength(0)
    // ⚠ EVERY ABSENCE ABOVE IS PROVED BY ADDING THE FORBIDDEN THING, at each
    // position it could occupy: the chip, a heading cell, a body cell and the
    // grid itself. A query that cannot find a planted violation is not evidence
    // of the rule; it is evidence of nothing.
    for (const selector of ['.canvas-table-chip', '.canvas-table-heading', '.canvas-table-cell', '.canvas-table-grid']) {
      const host = paint.querySelector(selector) as HTMLElement
      expect(host, selector).not.toBeNull()
      const plant = document.createElement('span')
      plant.setAttribute('role', 'button')
      plant.setAttribute('data-component-id', 'e7')
      plant.setAttribute('aria-label', 'table component e7')
      plant.setAttribute('tabindex', '0')
      plant.className = 'canvas-box'
      host.appendChild(plant)
      expect(paint.querySelectorAll('[role="button"], button'), selector).toHaveLength(1)
      expect(paint.querySelectorAll('[data-component-id]'), selector).toHaveLength(1)
      expect(paint.querySelectorAll('[aria-label]'), selector).toHaveLength(1)
      expect(paint.querySelectorAll('[tabindex]'), selector).toHaveLength(1)
      expect(paint.querySelectorAll('.canvas-table .canvas-box'), selector).toHaveLength(1)
      host.removeChild(plant)
      const grouped = document.createElement('span')
      grouped.setAttribute('role', 'group')
      host.appendChild(grouped)
      expect(paint.querySelectorAll('[role="group"], [role="tablist"]'), selector).toHaveLength(1)
      host.removeChild(grouped)
    }
    // And the control still reads as a WORD rather than a glyph: at least one
    // letter or digit survives the aria-hidden strip, which is what
    // control-vocabulary-contract's treatmentOf classifies on.
    const visible = paint.cloneNode(true) as Element
    visible.querySelectorAll('[aria-hidden="true"], [hidden]').forEach((node) => node.remove())
    expect(visible.textContent ?? '').toMatch(/[A-Za-z0-9]/)
  })

  it('repeats the same table body on a later sheet, decorative and unnamed', () => {
    // A component whose extent crosses a window boundary is drawn on every
    // window it intersects. The echo carries no role, no handlers and no name.
    const tall = { ...canvas, contentWindowCount: 2, contentWindowOrigins: [0, 400_000], contentWindowPages: [0, 0] }
    const view = mount([table({ y: 380_000, height: 60_000 })], tall)
    const echo = view.container.querySelector('.canvas-component-echo.canvas-component-table') as HTMLElement
    expect(echo).not.toBeNull()
    expect(echo.getAttribute('aria-hidden')).toBe('true')
    expect(echo.getAttribute('role')).toBeNull()
    expect(echo.getAttribute('aria-label')).toBeNull()
    // The SAME body, chip included — an echo that drew something else would
    // make the sheet stack a lie in a second way.
    expect(textsOf(echo, '.canvas-table-heading')).toEqual(['Date', 'Description', 'Debit', 'Credit', 'Balance'])
    expect(echo.querySelector('.canvas-table-collection')?.textContent).toBe('transactions[]')
    // And exactly one accessible name for the component, not two.
    expect(screen.getAllByRole('button', { name: 'table component e7' })).toHaveLength(1)
  })
})

// STORY 14.9 / STEP-04 P2 — THE MARKER IS PINNED TO THE ELEMENTS, IN THE DOM.
//
// ⚠ WHY THIS ROW EXISTS. Stripping ` canvas-display-paint` from all six
// className sites in App.tsx left the whole designer suite green — 77 files,
// 1400 tests, exit 0. That class is the RUNTIME MECHANISM of R1's conditions 1
// and 2 (`min-width: 0; overflow: hidden; white-space: pre; text-overflow:
// ellipsis`): without it the browser starts making break decisions, and content
// widens a grid track the ENGINE sized. The only thing tying it to the paint
// was a whole-file substring check that a COMMENT already satisfied — an
// instrument whose silence is its answer.
//
// So the pin is per ELEMENT, over both branches of every ternary, and every one
// of the six sites is asserted to have been VISITED. Removing the class from
// any single site reds this row.
describe('every element that paints an engine-owned string carries the display-paint marker', () => {
  const marker = 'canvas-display-paint'
  // The five classes that paint a string on the table, and the two positions
  // `.canvas-table-unset` occupies — the chip's collection stand-in and a
  // cell's. Six sites, one per className branch in TablePaint.
  const sites = [
    { name: 'chip collection (bound)', selector: '.canvas-table-chip .canvas-table-collection' },
    { name: 'chip collection (unbound)', selector: '.canvas-table-chip .canvas-table-unset' },
    { name: 'column count', selector: '.canvas-table-count' },
    { name: 'header label', selector: '.canvas-table-heading' },
    { name: 'representative cell (bound)', selector: '.canvas-table-grid .canvas-table-cell' },
    { name: 'representative cell (unbound)', selector: '.canvas-table-grid .canvas-table-unset' },
  ] as const

  it('marks all six paint sites, over both branches of every ternary', () => {
    // TWO MOUNTS, because the six sites are three ternaries: a bound table with
    // bound columns reaches one branch of each, an unbound table with an
    // unbound column reaches the other.
    const bound = home(mount([table()]).container)
    const unbound = home(mount([table({ tableBind: '', columns: [column('e10', 'Date', 22_000, ''), column('e11', 'Amount', 40_000, '{{amount}}')], width: 62_000 })]).container)
    const seen = new Set<string>()
    for (const root of [bound, unbound]) {
      for (const site of sites) {
        for (const node of Array.from(root.querySelectorAll(site.selector))) {
          seen.add(site.name)
          expect(node.classList.contains(marker), `${site.name}: ${node.className}`).toBe(true)
        }
      }
    }
    // ⚠ NON-VACUITY, AND IT IS THE HALF THAT MAKES THE LOOP EVIDENCE. A
    // selector that matched nothing would pass the loop silently, which is the
    // shape of the check this row replaces.
    expect([...seen].sort()).toEqual(sites.map((site) => site.name).sort())
  })

  it('marks nothing that is NOT an engine-owned string, so the marker still means something', () => {
    const paint = home(mount([table()]).container)
    // The chip's glyph and the grid container paint no string; the no-columns
    // notice is a fixed chrome sentence in a placeholder frame that
    // deliberately WRAPS, and claims no exception (it predates the ruling in
    // ImagePlaceholder's form).
    for (const selector of ['.canvas-table-chip-icon', '.canvas-table-grid', '.canvas-table']) {
      const node = paint.querySelector(selector)
      expect(node, selector).not.toBeNull()
      expect((node as Element).classList.contains(marker), selector).toBe(false)
    }
    const { columns: _columns, ...columnless } = table()
    expect(home(mount([columnless]).container).querySelector(`.${marker}`)).toBeNull()
  })

  it('backs the marker with the rule that makes condition 2 true of it', () => {
    // The class is only a mechanism because App.css says so. Asserted here as
    // well as in canvas-authority-contract.test.ts because THIS file is what
    // proves the class reaches the elements.
    const rule = cssRule(`.${marker}`)
    expect(rule).toMatch(/white-space:\s*(?:pre|nowrap)\b/)
    expect(rule).toMatch(/overflow:\s*hidden\b/)
    expect(rule).toMatch(/min-width:\s*0\b/)
    expect(rule).toMatch(/text-overflow:\s*ellipsis\b/)
  })
})

// STORY 14.9 / STEP-04 P3, P4, P5, P9 — THE FOUR THINGS THAT COULD BE DELETED
// WITH EVERY TEST STILL GREEN.
describe('the drawing survives the documents that are awkward rather than typical', () => {
  it('keeps a valid, track-shaped grid declaration when a column width is NEGATIVE', () => {
    // A negative `<length>` is not a valid grid track size, so ONE negative
    // column would invalidate the whole declaration and hand every track's size
    // back to the browser's own content measurement — R1's condition 1, lost on
    // a document the matrix requires to paint. Nothing mounted such a table
    // before this row; the guard test only checked that the projection was
    // accepted.
    const columns = [column('e10', 'Date', -5_000, '{{date}}'), column('e11', 'Amount', 40_000, '{{amount}}')]
    const grid = home(mount([table({ columns, width: 35_000 })]).container).querySelector('.canvas-table-grid') as HTMLElement
    expect(grid.style.gridTemplateColumns).toBe([canvasDisplay.css(0, 1), canvasDisplay.css(40_000, 1)].join(' '))
    // The declaration survived AND carries no negative track — either failure
    // is the same defect, so both are named.
    expect(grid.style.gridTemplateColumns).not.toBe('')
    expect(grid.style.gridTemplateColumns).not.toMatch(/-\d/)
    // The projection itself is untouched: the clamp is a display floor and
    // refuses nothing. The negative column is still drawn, in its place.
    expect(Array.from(grid.children)).toHaveLength(4)
    expect(columns[0]?.width).toBe(-5_000)
  })

  it('maps the tracks through the zoom rule, at a zoom that is not 1', () => {
    // Every other row mounts at zoom 1, where a painter that ignored `zoom`
    // entirely produces exactly the expected string. One step of "Zoom in" is
    // +0.1, and canvasDisplay.css is the one mapping the whole canvas uses.
    const view = mount([table()])
    const grid = () => view.container.querySelector('.canvas-component:not(.canvas-component-echo)[data-component-id="e7"] .canvas-table-grid') as HTMLElement
    const atOne = grid().style.gridTemplateColumns
    fireEvent.click(screen.getByRole('button', { name: 'Zoom in' }))
    return waitFor(() => expect(screen.getByLabelText('Canvas zoom')).toHaveTextContent('110%')).then(() => {
      expect(grid().style.gridTemplateColumns).toBe(fiveColumns.map((each) => canvasDisplay.css(each.width, 1.1)).join(' '))
      // The positive control: the zoomed string genuinely differs from the
      // zoom-1 one, so a painter ignoring `zoom` cannot answer this row.
      expect(grid().style.gridTemplateColumns).not.toBe(atOne)
    })
  })

  it('flips the component outline to dashed for the no-columns state, by a rule that exists', () => {
    // The DOM row above can only see the class. This is the rule keyed on it,
    // read out of App.css — deleting the line left every test green before.
    const rule = cssRule('.canvas-component:not(.canvas-component-selected):has(.canvas-table-empty)::after')
    expect(rule).toMatch(/border-style:\s*dashed\b/)
    expect(rule).toMatch(/border-color:\s*var\(--color-page-outline-dash\)/)
  })

  it('keeps the column count legible when the collection path is long', () => {
    // A collection path has no length worth relying on. With the path unable to
    // shrink, the count was pushed past the chip's clip edge and vanished on
    // exactly the documents whose chip is hardest to read.
    const long = `transactions.${'segment.'.repeat(40)}rows[]`
    expect(long.length).toBeGreaterThan(MAX_CANVAS_PROPERTY_STRING / 4)
    const paint = home(mount([table({ tableBind: long })]).container)
    expect(paint.querySelector('.canvas-table-collection')?.textContent).toBe(long)
    expect(paint.querySelector('.canvas-table-count')?.textContent).toBe('5 columns')
    // jsdom lays nothing out, and the canvas may not measure anything anyway,
    // so which element yields is asserted where it is decided: in the rule.
    expect(cssRule('.canvas-table-collection')).toMatch(/flex:\s*0\s+1\s+auto\b/)
    expect(cssRule('.canvas-table-count')).toMatch(/flex:\s*none\b/)
  })
})

describe('the protocol guard admits the canvas table columns, and only those', () => {
  const inbound = (components: ReadonlyArray<unknown>) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: { ...canvas, components } } })
  const text = { id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello', textPaint: { overflow: false, truncated: false, lines: [] } }

  it('accepts a table with columns, and a table with none', () => {
    expect(inbound([table()])).toBeDefined()
    const { columns: _columns, ...columnless } = table()
    expect(inbound([columnless])).toBeDefined()
  })

  it('accepts a zero and a NEGATIVE column width, because both load and paint today', () => {
    // isTableColumns requires `width > 0` for the table EDITOR's projection.
    // Copying that bound onto this guard would make isCanvas false,
    // parseInbound undefined and PROTOCOL_INVALID terminate the worker — on a
    // document that draws now.
    expect(inbound([table({ columns: [column('e10', 'Date', 0, '{{date}}')], width: 0 })])).toBeDefined()
    expect(inbound([table({ columns: [column('e10', 'Date', -5, '{{date}}'), column('e11', 'Amount', 40_000, '{{amount}}')], width: 39_995 })])).toBeDefined()
    // And an empty label and an empty bind, which are declared-empty rather
    // than absent.
    expect(inbound([table({ columns: [column('e10', '', 22_000, '')], width: 22_000 })])).toBeDefined()
  })

  it('rejects a non-table component carrying columns, exactly as it rejects a non-table tableBind', () => {
    expect(inbound([{ ...text, columns: fiveColumns }])).toBeUndefined()
    // The positive control on the same fixture: without the member it passes,
    // so the rejection is attributable to the member and not to the base.
    expect(inbound([text])).toBeDefined()
    expect(inbound([{ ...text, tableBind: 'transactions[]' }])).toBeUndefined()
  })

  it('accepts a MULTIBYTE label at the length Go\'s byte clip can produce', () => {
    // THE TWO SIDES BOUND DIFFERENT UNITS. Go clips `len(column.Label)` — BYTES
    // — at 512; this guard bounds `column.label.length`, which is UTF-16 code
    // units. A Thai or CJK label is exactly where they diverge: three bytes to
    // the character, so Go's 512-byte clip yields ~170 characters. The guard
    // must accept that, and must accept a label right up to its OWN bound too,
    // because UTF-16 length is never greater than byte length (a BMP rune is 1
    // unit and 1-3 bytes; a supplementary rune is 2 units and 4 bytes).
    const thai = 'ก'.repeat(170)
    expect(new TextEncoder().encode(thai).length).toBeLessThanOrEqual(MAX_CANVAS_PROPERTY_STRING)
    expect(inbound([table({ columns: [column('e10', thai, 22_000, thai)], width: 22_000 })])).toBeDefined()
    // At the guard's own bound, in its own unit — 512 UTF-16 units of Thai is
    // 1536 bytes, which Go would have clipped, but the guard is not entitled to
    // refuse a value it did not measure.
    const atGuardBound = 'ก'.repeat(MAX_CANVAS_PROPERTY_STRING)
    expect(atGuardBound.length).toBe(MAX_CANVAS_PROPERTY_STRING)
    expect(inbound([table({ columns: [column('e10', atGuardBound, 22_000, '{{a}}')], width: 22_000 })])).toBeDefined()
    // One unit over is refused, so the bound is where it says it is.
    expect(inbound([table({ columns: [column('e10', `${atGuardBound}ก`, 22_000, '{{a}}')], width: 22_000 })])).toBeUndefined()
  })

  it('rejects two columns sharing an id, which the loader already refuses', () => {
    // TablePaint keys its two rows on the column id, so a duplicate would
    // produce duplicate React keys. The clause matches the component-level
    // dedupe this guard already keeps, and it can never newly refuse a document
    // that ships: `claimID` refuses a duplicate at load, document-wide, for a
    // `columns[].id` exactly as for an element id —
    // TestColumnIdsAreUniqueDocumentWide in folio-go proves that rather than
    // assuming it.
    const twice = [column('e10', 'Date', 22_000, '{{date}}'), column('e10', 'Amount', 40_000, '{{amount}}')]
    expect(inbound([table({ columns: twice, width: 62_000 })])).toBeUndefined()
    // The positive control: the same pair with distinct ids is accepted, so the
    // rejection is the duplicate's doing.
    expect(inbound([table({ columns: [twice[0] as Column, column('e11', 'Amount', 40_000, '{{amount}}')], width: 62_000 })])).toBeDefined()
  })

  it('rejects a column that is the wrong shape, in each direction the guard has', () => {
    const bad = (patch: Record<string, unknown>) => inbound([table({ columns: [{ ...column('e10', 'Date', 22_000, '{{date}}'), ...patch } as Column], width: 22_000 })])
    // A SURPLUS key — the direction that kills the worker when Go adds a field
    // and this list does not move with it.
    expect(bad({ footer: 'sum' })).toBeUndefined()
    // A DROPPED key. hasExactKeys rejects both ways, which is what keeps an
    // `omitempty` on the Go side from silently emptying a column.
    const { label: _label, ...noLabel } = column('e10', 'Date', 22_000, '{{date}}')
    expect(inbound([table({ columns: [noLabel as Column], width: 22_000 })])).toBeUndefined()
    // The two closed sets, each on its own account — `justify` reaches neither,
    // because columns[].align is a three-value vocabulary and a justified table
    // is refused at load.
    expect(bad({ headerAlign: 'justify' })).toBeUndefined()
    expect(bad({ cellAlign: 'justify' })).toBeUndefined()
    expect(bad({ headerAlign: '' })).toBeUndefined()
    expect(bad({ cellAlign: 'middle' })).toBeUndefined()
    // Types and bounds.
    expect(bad({ width: 1.5 })).toBeUndefined()
    expect(bad({ width: '22000' })).toBeUndefined()
    expect(bad({ id: '' })).toBeUndefined()
    expect(bad({ label: 'x'.repeat(513) })).toBeUndefined()
    expect(bad({ bind: 'x'.repeat(513) })).toBeUndefined()
    expect(bad({ label: 7 })).toBeUndefined()
    expect(inbound([table({ columns: 'five' as unknown as ReadonlyArray<Column>, width: 22_000 })])).toBeUndefined()
    // And the positive control, so every rejection above is the patch's doing.
    expect(bad({})).toBeDefined()
  })
})

// SPEC-table-rules §4: the heading paints the ENGINE's packed label lines, one
// block per line, and never hands `label` to the browser to wrap.
describe('the canvas paints a column label as the engine packed it', () => {
  const inbound = (components: ReadonlyArray<unknown>) => parseInbound({ protocolVersion: ENGINE_PROTOCOL_VERSION, kind: 'response', requestId: 'canvas-1', ok: true, snapshot: { documentState: 'loaded', revision: 1, byteLength: 1, canvas: { ...canvas, components } } })
  it('draws one line element per labelLines entry, in order, and ignores label for the paint', () => {
    const packed: Column = { ...column('e10', 'ignored', 40_000, '{{date}}', 'center', 'left'), label: 'วันที่\nDATE of posting', labelLines: ['วันที่', 'DATE of', 'posting'] }
    const view = mount([table({ columns: [packed, column('e11', 'Amount', 40_000, '{{amount}}'), column('e12', '', 20_000, '{{x}}')], width: 100_000 })])
    const headings = home(view.container).querySelectorAll('.canvas-table-heading')
    expect(textsOf(headings[0] as Element, '.canvas-table-heading-line')).toEqual(['วันที่', 'DATE of', 'posting'])
    expect(textsOf(headings[1] as Element, '.canvas-table-heading-line')).toEqual(['Amount'])
    expect(headings[2]?.querySelectorAll('.canvas-table-heading-line')).toHaveLength(0)
    // Nothing but the line elements is painted inside the heading: no raw
    // line feed for the browser to act on.
    expect(Array.from((headings[0] as Element).childNodes).every((node) => (node as Element).classList?.contains('canvas-table-heading-line'))).toBe(true)
    expect(headings[0]?.textContent).not.toContain('\n')
    // The heading keeps its column id, its paint marker and its alignment.
    expect((headings[0] as HTMLElement).dataset.columnId).toBe('e10')
    expect(headings[0]?.classList.contains('canvas-display-paint')).toBe(true)
    expect((headings[0] as HTMLElement).style.textAlign).toBe('center')
  })

  it('declares each line a block and leaves the break to the engine', () => {
    const rule = cssRule('.canvas-table-heading-line')
    expect(rule).toMatch(/display:\s*block/)
    expect(rule).not.toMatch(/white-space/)
  })

  it('refuses a canvas column whose labelLines is missing, over-long or mistyped', () => {
    const bad = (patch: Record<string, unknown>) => inbound([table({ columns: [{ ...column('e10', 'Date', 22_000, '{{date}}'), ...patch } as Column], width: 22_000 })])
    const { labelLines: _lines, ...noLines } = column('e10', 'Date', 22_000, '{{date}}')
    expect(inbound([table({ columns: [noLines as Column], width: 22_000 })])).toBeUndefined()
    expect(bad({ labelLines: 'Date' })).toBeUndefined()
    expect(bad({ labelLines: [7] })).toBeUndefined()
    // The bounds are Go's own (maxCanvasTableLabelLines and
    // maxCanvasTableLabelLineLength), not the generic property-string bound: a
    // packed line is label TEXT, and a legitimate label line may be longer than
    // an identifier's 512 units.
    expect(MAX_CANVAS_TABLE_LABEL_LINES).toBe(256)
    expect(MAX_CANVAS_TABLE_LABEL_LINE_LENGTH).toBe(1024)
    expect(bad({ labelLines: ['x'.repeat(1025)] })).toBeUndefined()
    expect(bad({ labelLines: Array.from({ length: 257 }, () => 'x') })).toBeUndefined()
    // Positive controls at the bounds — and past the old property-string bound.
    expect(bad({ labelLines: ['x'.repeat(MAX_CANVAS_PROPERTY_STRING + 1)] })).toBeDefined()
    expect(bad({ labelLines: ['x'.repeat(1024)] })).toBeDefined()
    expect(bad({ labelLines: Array.from({ length: 256 }, () => 'x') })).toBeDefined()
    expect(bad({ labelLines: Array.from({ length: 16 }, () => 'x'.repeat(1024)) })).toBeDefined()
    expect(bad({ labelLines: [] })).toBeDefined()
  })
})

