import type { CanvasProjection } from './engine-protocol'

// THE SHEET STACK, AS ARITHMETIC.
//
// Everything here is a pure function of numbers the ENGINE projected — the
// window origins, the window height, the page height, the band rectangles —
// plus this module's own two declared display constants and the local zoom.
// Nothing measures the DOM, and nothing may: `canvas-authority-contract.
// test.ts` bans getBoundingClientRect, every offset*/client*/scroll* metric
// and getComputedStyle across the whole designer, which is why the gap
// between two sheets is declared HERE as a number and written out as a custom
// property rather than read back from a CSS token.
//
// It is a `.ts` and not a `.tsx` on purpose: oxlint's `only-export-components`
// baseline is exactly four warnings, and a non-component export added beside
// a component would make a fifth. A pure module is also the only way to unit
// test this geometry at all.

// MAX_CANVAS_SHEETS bounds the DRAWING, never the projected value. D-7.4.2's
// settled shape: truncate the drawing, keep the number, make the degraded
// state distinguishable from the empty one, and derive the bound rather than
// picking a round one.
//
// THE DERIVATION. Epic 7's narrative target is forty pages. The projection's
// own body-text paint budget is 1920 lines (maxCanvasBodyTextLines), which at
// the decision log's corrected forty-to-fifty lines per A4 window is under
// fifty windows of solid prose — so past that, a window can only come from a
// DECLARED placement gap, not from text anyone is reading. 120 is three times
// the epic's stated target and more than twice what the paint budget can
// fill. Each sheet is a page's worth of DOM and the canvas is unvirtualised
// (DW-34), which is the cost this bounds. The model retains the full projected
// count and truncation state independently of the sheets mounted by the UI.
export const MAX_CANVAS_SHEETS = 120

// The vertical gap between two sheets, in CSS pixels at zoom 1 — the stack's
// own constant, not `--space-5`. The stack must be able to invert its own
// display geometry to keep a drag tracking the pointer across a seam, and a
// token the browser would have to read back is exactly the measurement this
// canvas may not make.
export const SHEET_STACK_GAP = 24

export type CanvasComponentProjection = CanvasProjection['components'][number]

// One occurrence of a component on one sheet. `home` is true for exactly one
// occurrence per component: the window its own top falls in. Only the home
// occurrence is interactive and accessibly named — two identical accessible
// names for one component would break selection, RTL's getByLabelText and
// Playwright's strict mode alike (Ruling G).
export type SheetOccurrence = Readonly<{ component: CanvasComponentProjection; y: number; home: boolean }>

export type Sheet = Readonly<{
  // The window this sheet draws, 0-based, and where that window begins in the
  // content column — straight from contentWindowOrigins, never index * height.
  index: number
  origin: number
  // SPEC-multi-pages story 2: the designed page this window belongs to
  // (contentWindowPages), and whether it is that page's first sheet — where the
  // page label is drawn. Origins are PAGE-LOCAL, so `origin` is measured in this
  // page's own column.
  page: number
  pageStart: boolean
  // Where the NEXT window begins, measured down this sheet's content band,
  // when that falls inside the band. Absent when the next window begins past
  // this sheet's own foot — a declared gap — in which case the band's foot IS
  // the boundary and the skipped column region is drawn by nobody.
  seam?: number
  content: ReadonlyArray<SheetOccurrence>
}>

export type SheetStack = Readonly<{
  sheets: ReadonlyArray<Sheet>
  // What the projection said, before the drawing budget was applied.
  windowCount: number
  truncated: boolean
  // Go's ContentWindowCountIsExact, retained as model metadata independently
  // of whether the UI displays a count. True means the projected count is exact.
  isExact: boolean
}>

const contentComponents = (canvas: CanvasProjection): ReadonlyArray<CanvasComponentProjection> => canvas.components.filter((component) => component.band === 'content')

// SPEC-multi-pages story 2: the designed page a component belongs to, as Go
// projected it. A one-page projection may omit it, and then every component is
// on page 0; a header or footer component is always 0.
export const componentPage = (component: CanvasComponentProjection): number => component.page ?? 0

// How many designed pages the projection has: its last window's page, plus one.
export function pageCountOf(canvas: CanvasProjection): number {
  return (canvas.contentWindowPages[canvas.contentWindowPages.length - 1] ?? 0) + 1
}

// One page's windows: the index of its first window in the whole stack, and
// its page-local origins in order. Windows are grouped by page in page order.
export function pageWindows(canvas: CanvasProjection, page: number): Readonly<{ first: number; origins: ReadonlyArray<number> }> {
  const origins: number[] = []
  let first = -1
  canvas.contentWindowPages.forEach((owner, index) => {
    if (owner !== page) return
    if (first < 0) first = index
    origins.push(canvas.contentWindowOrigins[index] as number)
  })
  return { first: Math.max(first, 0), origins: origins.length > 0 ? origins : [0] }
}

// The window a component BELONGS to: the last one that begins at or above its
// own top. origins[0] is 0 and a component's y is non-negative, so this always
// answers, and for every component the engine actually paginated it answers
// the window that contains the component's top.
export function homeWindow(origins: ReadonlyArray<number>, y: number): number {
  let home = 0
  for (let index = 1; index < origins.length; index += 1) {
    if ((origins[index] as number) <= y) home = index
    else break
  }
  return home
}

// WHERE A COLUMN OFFSET SITS ON THE SHEET THAT HOLDS IT, and the one place
// that decides it. Every component the engine paginated has a top inside its
// home window already, so this is the identity for them. It is NOT the
// identity for a component the engine never paginated — a text element whose
// font chain would not resolve contributes no column items, so no window ever
// begins at its top and it can sit in the region between one window's foot and
// the next window's origin. The spec draws no such region ("the skipped column
// region is not drawn"), so a point there has no drawn position of its own; it
// is shown with its selectable box inside the foot of its owning sheet.
//
// The point mapping used by resize remains separate from box fallback. When they
// disagreed, a zero-delta drag on such a component committed a column offset
// nine windows away: the drawing put it past its sheet, and the inverse then
// floored it onto a later sheet and added that sheet's origin.
const offsetWithinWindow = (columnY: number, origin: number, windowHeight: number): number => Math.min(Math.max(columnY - origin, 0), windowHeight)

export function sheetStack(canvas: CanvasProjection): SheetStack {
  const origins = canvas.contentWindowOrigins
  // Named for what it is, not shortened to `height`: the authority contract
  // bans a window position derived by multiplying the window height by an
  // index, and a text guard can only catch the spelling it can see.
  const windowHeight = canvas.contentWindowHeight
  const windowPages = canvas.contentWindowPages
  // The cap truncates the TAIL of the whole stack, across every page.
  const drawn = Math.min(origins.length, MAX_CANVAS_SHEETS)
  const components = contentComponents(canvas)
  // A component is homed among ITS OWN page's windows only: origins are
  // page-local, so page 2's y means nothing against page 1's origins.
  const perPage = new Map<number, ReturnType<typeof pageWindows>>()
  const windowsOf = (page: number) => { let found = perPage.get(page); if (!found) { found = pageWindows(canvas, page); perPage.set(page, found) } return found }
  const homes = new Map<string, number>()
  for (const component of components) { const own = windowsOf(componentPage(component)); homes.set(component.id, own.first + homeWindow(own.origins, component.y)) }
  const sheets: Sheet[] = []
  for (let index = 0; index < drawn; index += 1) {
    const origin = origins[index] as number
    const page = windowPages[index] ?? 0
    // A seam never crosses a page boundary: the next page starts a new column.
    const next = windowPages[index + 1] === page ? origins[index + 1] : undefined
    const content: SheetOccurrence[] = []
    for (const component of components) {
      if (componentPage(component) !== page) continue
      const home = homes.get(component.id) === index
      // Drawn on every window its extent intersects, and unconditionally on
      // its home window — so a component the engine never paginated (a text
      // element whose chain would not resolve contributes no extents) still
      // has exactly one occurrence somewhere rather than vanishing.
      const intersects = component.y < origin + windowHeight && component.y + component.height > origin
      // An occurrence that genuinely intersects this window keeps its exact
      // offset; only the home of a component that intersects NO window is
      // pulled onto its sheet, which is the sole case where the two differ.
      if (intersects) content.push({ component, y: component.y - origin, home })
      else if (home) content.push({ component, y: offsetWithinWindow(component.y, origin, Math.max(0, windowHeight - component.height)), home })
    }
    sheets.push({ index, origin, page, pageStart: index === 0 || windowPages[index - 1] !== page, ...(next !== undefined && next - origin <= windowHeight ? { seam: next - origin } : {}), content })
  }
  return { sheets, windowCount: origins.length, truncated: origins.length > drawn, isExact: canvas.contentWindowCountIsExact }
}

// THE STACK'S DISPLAY-SPACE INVERSE, in document millipoints.
//
// A drag accumulates a linear pointer delta, which knows nothing about the
// repeated page-footer, the gap and the repeated page-header standing between
// one window's foot and the next window's head. Across a seam the component
// would drift from the hand by exactly that much. resize-anchor.ts's own
// header states the principle this repairs: a pointer that leaves the band
// should leave the component against that edge with its other axis still
// TRACKING THE HAND.
//
// The pitch is one whole page plus the stack's declared gap. The gap is the
// one display-space constant in the arithmetic, so it is divided by the zoom
// exactly once, here, and everything else is projected millipoints.
export function sheetPitch(canvas: CanvasProjection, zoom: number): number {
  return canvas.height + Math.round((SHEET_STACK_GAP / zoom) * 1000)
}

const contentBandTop = (canvas: CanvasProjection): number => (canvas.bands[1] as CanvasProjection['bands'][number]).y

// The drawn sheets of one page, falling back to the last drawn sheet when the
// cap truncated that page away entirely, so the mapping always has a sheet.
const drawnSheetsOf = (stack: SheetStack, page: number): ReadonlyArray<Sheet> => {
  const own = stack.sheets.filter((sheet) => sheet.page === page)
  return own.length > 0 ? own : stack.sheets.slice(-1)
}

// A column offset IN `page`'s COLUMN, as a distance down the whole stack from
// the top edge of sheet one's page. Origins are page-local, so the offset is
// mapped among that page's sheets only.
export function stackYForColumn(stack: SheetStack, canvas: CanvasProjection, zoom: number, columnY: number, page = 0): number {
  const own = drawnSheetsOf(stack, page)
  const sheet = own[Math.min(homeWindow(own.map((entry) => entry.origin), Math.max(columnY, 0)), own.length - 1)] as Sheet
  // Read through the SAME clamp the drawing uses, so the point this returns is
  // the point the author is actually looking at, and the inverse below can
  // recover it exactly.
  return sheet.index * sheetPitch(canvas, zoom) + contentBandTop(canvas) + offsetWithinWindow(columnY, sheet.origin, canvas.contentWindowHeight)
}

// And back: which sheet a point down the stack falls on, and what column
// offset that is. The two are inverses on every point of every drawn sheet,
// which is the property the drag depends on and sheet-stack.test.ts asserts.
// With `page`, the sheet is held to that page's sheets, so a drag never maps
// into another page's column (moving between pages is story 3).
export function columnForStackY(stack: SheetStack, canvas: CanvasProjection, zoom: number, stackY: number, page?: number): Readonly<{ window: number; columnY: number }> {
  const pitch = sheetPitch(canvas, zoom)
  const own = page === undefined ? stack.sheets : drawnSheetsOf(stack, page)
  const low = (own[0] as Sheet).index
  const high = (own[own.length - 1] as Sheet).index
  const index = Math.min(Math.max(Math.floor(stackY / pitch), low), high)
  const sheet = stack.sheets[index] as Sheet
  return { window: index, columnY: sheet.origin + (stackY - index * pitch - contentBandTop(canvas)) }
}

// What the drag actually asks for: given the column offset of the edge the
// gesture is moving and the raw document-space delta the pointer travelled,
// where does that edge end up in the COLUMN? One opaque number goes to Go,
// and it is a column coordinate, never a pin to a sheet. It stays within the
// component's own page.
export function columnEdgeAfterDrag(stack: SheetStack, canvas: CanvasProjection, zoom: number, columnEdge: number, delta: number, page = 0): number {
  return columnForStackY(stack, canvas, zoom, stackYForColumn(stack, canvas, zoom, columnEdge, page) + delta, page).columnY
}
