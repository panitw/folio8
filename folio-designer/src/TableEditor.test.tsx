import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import App from './App'
import { TableEditor } from './TableEditor'
import { MAX_ENGINE_HISTORY_ENTRIES } from './engine-protocol'
import type { EngineClient } from './engine-client'
import type { FileAccess } from './file/file-access'
import { acceptSampleData } from './sample-data'
import { alignGlyphs, alignSegments } from './segmented-control'

// STORY 14.7 — THE TABLE EDITOR'S OWN TEST FILE, AND WHY IT EXISTS AT ALL.
//
// `DW-351` records that the matrix's keyboard lattice has never been driven
// over more than ONE ROW. Every table fixture in this repository carries a
// single column, so `moveFocus`'s vertical loop cannot step, and its
// `enabled()` skip — the branch that walks past a disabled or ABSENT cell —
// is never exercised. "The navigation survived the rebuild" measured against
// that is a comparison against nothing.
//
// So this file was written and watched to pass BEFORE the six-column rebuild,
// against the eleven-column matrix, over a THREE-column table whose footers
// differ from one another. Every proof here is about the lattice itself:
// vertical movement, the disabled skip in both axes, Home/End over an
// end-disabled row, and — after the rebuild — the revealed cells and the
// focus that must survive one of them disappearing.

const canvas = {
  width: 595276, height: 841890, orientation: 'portrait' as const, preset: 'A4' as const, locale: 'en' as const, utcOffset: '+07:00', embedFonts: true,
  marginTop: 36000, marginRight: 36000, marginBottom: 36000, marginLeft: 36000, gridIncrement: 6000,
  commandWidth: 595276, commandHeight: 841890,
  fontFamilies: ['body'], fontChains: [{ name: 'body', entries: [{ face: 'Noto Sans', assetKey: '', family: '', style: '', bold: '', italic: '', boldItalic: '' }] }],
  defaultFontSize: 12000, defaultLineSpacing: 1000,
  contentWindowHeight: 729890, contentWindowCount: 1, contentWindowOrigins: [0], contentWindowPages: [0], contentWindowCountIsExact: true,
  bands: [
    { name: 'pageHeader' as const, x: 36000, y: 36000, width: 523276, height: 20000 },
    { name: 'content' as const, x: 36000, y: 56000, width: 523276, height: 729890 },
    { name: 'pageFooter' as const, x: 36000, y: 785890, width: 523276, height: 20000 },
  ],
  components: [{ id: 'e7', type: 'table' as const, band: 'content' as const, x: 23276, y: 0, width: 300000, height: 12000, resizable: false }],
}
const tableHeaderProjection = { headerHeight: 12000, altRowBackground: '', headerFontFamily: '', headerFontFamilyResolved: 'body', headerFontSize: 0, headerFontSizeResolved: 12000, headerLineSpacing: 0, headerLineSpacingResolved: 1000, headerBackground: '', headerBackgroundResolved: '', headerColor: '', headerColorResolved: '', headerValign: '', headerValignResolved: 'top', headerAlign: '', headerAlignResolved: 'left', headerBold: false, headerBoldResolved: false, headerItalic: false, headerItalicResolved: false, 'headerBorder.width': '', 'headerBorder.widthResolved': '', 'headerBorder.color': '', 'headerBorder.colorResolved': '', 'headerBorder.edges': '', 'headerBorder.edgesResolved': '', minHeight: 0, 'rules.width': '', 'rules.widthResolved': '', 'rules.color': '', 'rules.colorResolved': '', 'rules.between': '', paddingLeft: '', paddingRight: '', paddingHeaderOverride: false }

type Footer = '' | 'sum' | 'avg' | 'count'
type ColumnFixture = Readonly<{ id: string; header: string; width: number; align: 'left' | 'center' | 'right'; headerAlign?: '' | 'left' | 'center' | 'right'; headerAlignResolved?: 'left' | 'center' | 'right'; rowField: string; binding?: string; rowFieldEditable?: boolean; footer: Footer; footerOf: string; footerFormat: string }>

// THREE COLUMNS, WITH THREE DIFFERENT FOOTER SHAPES, and each difference is
// load-bearing rather than decoration:
//   • column 1 aggregates a SUM, so it carries both a source and a format;
//   • column 2 aggregates NOTHING, so it carries neither — the row with the
//     holes the lattice has to walk past;
//   • column 3 aggregates a COUNT, which takes no source but does take a
//     format — the asymmetric middle case.
// Widths sum to 174pt (72 + 60 + 42) against a band remainder of 500pt: the
// content band is 523276 millipoints wide, the table sits at x = 23276, and
// `band.width − table.x` is 500000 millipoints — the same arithmetic
// `containComponent` does. So the budget's three states are reachable by moving
// one number, which is what the three budget tests below do.
const defaultColumns: ReadonlyArray<ColumnFixture> = [
  { id: 'c1', header: 'Amount', width: 72000, align: 'right', rowField: 'amount', footer: 'sum', footerOf: 'transactions.amount', footerFormat: '#,##0.00' },
  { id: 'c2', header: 'Date', width: 60000, align: 'left', rowField: 'date', footer: '', footerOf: '', footerFormat: '' },
  { id: 'c3', header: 'Note', width: 42000, align: 'center', rowField: 'note', footer: 'count', footerOf: '', footerFormat: '0' },
]

// This fixture recognizes only the simple row-path projection used by these
// UI tests. Full expression validity and alias migration remain Go test claims.
const mockRowBinding = (binding: string, alias: string) => {
  const simple = /^\{\{([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\}\}$/.exec(binding)
  const field = simple?.[1] === alias ? simple[2]! : ''
  return { rowField: field, rowFieldEditable: binding === '' || field !== '' }
}
const projected = (columns: ReadonlyArray<ColumnFixture>, alias = 'row') => columns.map((column) => {
  const binding = column.binding ?? (column.rowField === '' ? '' : `{{${alias}.${column.rowField}}}`)
  return {
    id: column.id, header: column.header, width: column.width, proportion: '', align: column.align, headerAlign: column.headerAlign ?? '', headerAlignResolved: column.headerAlignResolved ?? (column.headerAlign || column.align),
    binding, ...mockRowBinding(binding, alias),
    footer: column.footer, footerOf: column.footerOf, footerFormat: column.footerFormat,
  }
})

const snapshotOf = (over: Partial<typeof canvas> = {}) => ({ documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: { ...canvas, ...over } })

// The engine mock answers every `table-columns` call from a MUTABLE fixture, so
// a test can move a column's footer the way a command would and watch the panel
// re-project. `commands` records what actually went to the engine.
//
// STORY 14.7b TAUGHT IT TWO THINGS IT COULD NOT SAY BEFORE, and both were
// blocking rather than cosmetic.
//
// (1) A NO-OP. It used to answer `revision: ++state.revision` for EVERY
// command, so it could not express a legal command that changes nothing — and
// the edit count Cancel depends on is defined by exactly that distinction. THE
// ENGINE DECIDES WHAT IS A MUTATION, NOT THE UI (App.test.tsx:3955-3957), so
// this mock decides the same way Go does: it serializes its own state before
// and after `apply` and holds the revision STILL when they agree, which is
// `bytes.Equal(canonical, e.bytes)` returning before `pushUndo`.
//
// (2) A HISTORY. Cancel is a compensating sequence of `undo` operations, and a
// mock with no undo stack can only prove how many were SENT — never that the
// document came back to where it started. So it keeps the same two stacks Go
// keeps, restores canonical serialized state from them, leaves the revision
// MONOTONIC across an undo exactly as `install` does, pushes onto redo before
// restoring, and clears redo on the next committed change.
//
// The HEADER arms are applied for the same reason: `Clear Header text colour`
// on an unset field is the reachable no-op AC3 is driven through, and a mock
// that ignored header commands would have made every header edit a no-op and
// the proof vacuous.
// ⚠ THIS MAP IS THE MOCK'S WHOLE KNOWLEDGE OF WHAT A HEADER-STYLE COMMAND DOES,
// AND UNTIL STORY 14.8 A FIELD MISSING FROM IT WAS A SILENT NO-OP. The arm below
// used to read `if (key !== undefined)` and do nothing otherwise, so a test
// driving a NEW field would have watched the panel send a command, watched the
// projection not move, and passed — proving nothing at all. That was not a live
// vacuity while the map's seven entries matched the panel's seven-field union; it
// became one the moment the union grew, which is this story. `headerStyleKeyFor`
// now THROWS, and `refuses a header-style command naming a field the mock does
// not model` is the negative control that keeps it throwing — for the field after
// the twelfth as much as for these three.
const HEADER_STYLE_KEYS = { fontFamily: 'headerFontFamily', fontSize: 'headerFontSize', lineSpacing: 'headerLineSpacing', background: 'headerBackground', color: 'headerColor', valign: 'headerValign', align: 'headerAlign', 'border.width': 'headerBorder.width', 'border.color': 'headerBorder.color', 'border.edges': 'headerBorder.edges' } as const
const NUMERIC_HEADER_KEYS: ReadonlyArray<string> = ['headerFontSize', 'headerLineSpacing']
// The one field whose value is an ARRAY on the wire. The mock stores the edge set
// in the projection's own comma-joined spelling, so it converts once, here.
const ARRAY_HEADER_KEYS: ReadonlyArray<string> = ['headerBorder.edges']
// ⚠ AND THE ONE LENGTH THE PROJECTION SPELLS AS A STRING. `headerBorder.width`
// carries the same thousandths as its numeric siblings and differs only in how it
// spells ABSENCE: `''` rather than 0, because `0` is a legal declared width (the
// thinnest device line PDF can draw) and a numeric member cannot hold both
// meanings. The mock has to store it the way Go sends it, or every test that
// reads the panel back through a re-projection would be reading a shape the
// engine never emits — which is the one thing this mock exists not to do.
const MILLIPOINT_STRING_HEADER_KEYS: ReadonlyArray<string> = ['headerBorder.width']
// ⚠ AND IT ROUNDS, BECAUSE `String(Number(v) * 1000)` COULD PRODUCE A VALUE THE
// REAL GUARD REFUSES. `Number('0.29') * 1000` is `289.99999999999994` in IEEE-754
// doubles, and `String()` of that is a non-integer string — a shape
// `isTableColumns`' digits-only clause rejects outright and one Go's
// `strconv.FormatInt` can never emit. A mock that stores it is a harness admitting
// what production forbids, which turns every read-back below into an assertion
// about a projection the engine does not produce.
//
// Go's own arm reads this through `propertyLength`, which accepts THREE decimal
// places and refuses a fourth. So this rounds to the nearest thousandth — which
// recovers the 290 the engine computes — and THROWS if the input carried more
// precision than that, rather than silently accepting a value the command door
// would have refused.
function millipointString(value: unknown): string {
  const raw = Number(value) * 1000
  const thousandths = Math.round(raw)
  if (!Number.isFinite(raw) || Math.abs(raw - thousandths) > 1e-6) throw new Error(`the table-editor engine mock was sent ${JSON.stringify(value)}, which is not a length in points to at most three decimal places — propertyLength refuses it, so the mock must not admit it`)
  return String(thousandths)
}
export function headerStyleKeyFor(field: string): string {
  const key = (HEADER_STYLE_KEYS as Record<string, string | undefined>)[field]
  if (key === undefined) throw new Error(`the table-editor engine mock does not model the header-style field ${JSON.stringify(field)} — a command it cannot apply is a command whose test proves nothing`)
  return key
}

// (3) A REFUSAL. `refuseCommand` lets a test send a command that REACHES the
// engine and comes back an error — which is the only honest way to drive the
// matrix's "a rejected command is not counted" row. A dialog that declined to
// dispatch would prove nothing: a count keyed on dispatches and a count keyed on
// an observed revision change agree when nothing is sent at all. The thrown
// shape is the one `componentDiagnostic` renders — an `elementId` and a message
// — so the refusal arrives at the dialog's error surface located, exactly as
// `App.test.tsx`'s canvas-refusal test drives it.
const REFUSED_COMMAND = 'the engine refused this command'

// (4) A HELD UNDO. Story 14.7b's `Done` and Escape are live gestures WHILE the
// compensating sequence runs — the dialog is still on screen, because closing is
// the success path only — so proving they cannot tear it down needs the sequence
// to actually be in flight when they are pressed. `pauseUndoAt` parks one `undo`
// on a promise the test resolves by hand; nothing else about the mock changes.
//
// (5) A COMPONENT MOVE. It is the one committed edit reachable AFTER the dialog
// has closed and with no selection change — an arrow nudge on the still-selected
// table — which is what the discard sentence's own promise ("until your next
// committed edit") has to be measured against. Its x rides in canonical form so
// the mock treats it as a real change, exactly as Go would.
function tableEngine(initial: ReadonlyArray<ColumnFixture> = defaultColumns, over: Partial<typeof canvas> = {}, options: Readonly<{ failUndoAt?: number; pauseUndoAt?: number; pauseCommandAt?: number; refuseCommand?: (command: Readonly<Record<string, unknown>>) => boolean }> = {}) {
  const state = { columns: [...initial], collection: 'transactions[]', alias: 'row', header: { ...tableHeaderProjection }, componentX: 23276, revision: 1 }
  const history = { undo: [] as string[], redo: [] as string[] }
  const commands: string[] = []
  let undos = 0
  let releaseUndo!: () => void
  const heldUndo = new Promise<void>((resolve) => { releaseUndo = resolve })
  let releaseCommand!: () => void
  const heldCommand = new Promise<void>((resolve) => { releaseCommand = resolve })
  // THE CANONICAL FORM, and the revision is deliberately NOT in it: Go compares
  // document BYTES, and a comparison that included the revision would call every
  // command a change.
  const canonical = () => JSON.stringify({ columns: state.columns, collection: state.collection, alias: state.alias, header: state.header, componentX: state.componentX })
  const restore = (serialized: string) => { const parsed = JSON.parse(serialized) as { columns: ColumnFixture[]; collection: string; alias: string; header: typeof tableHeaderProjection; componentX: number }; state.columns = parsed.columns; state.collection = parsed.collection; state.alias = parsed.alias; state.header = parsed.header; state.componentX = parsed.componentX }
  // THE MOCK APPLIES WHAT IT IS SENT. A frozen projection cannot prove a
  // READ-BACK — the uncontrolled boxes in this panel keep whatever was typed
  // into them whether or not the document took it — so every assertion about a
  // committed value below reads the value back through a re-projection that
  // this little in-memory engine actually computed from the command.
  const apply = (command: Readonly<Record<string, unknown>>) => {
    const columnId = String(command.columnId ?? '')
    const edit = (change: (column: ColumnFixture) => ColumnFixture) => { state.columns = state.columns.map((column) => column.id === columnId ? change(column) : column) }
    // The nudge's command, in the mock's own canonical form. `x` arrives in
    // POINTS (the command layer divides millipoints by 1000), and it is a change
    // like any other: history entry, revision, the lot.
    if (command.kind === 'moveComponent') state.componentX = Number(command.x) * 1000
    if (command.kind === 'configureTableBinding') {
      const alias = String(command.alias) === '' ? 'row' : String(command.alias)
      state.columns = state.columns.map((column) => {
        if (column.binding === undefined) return column
        const row = mockRowBinding(column.binding, state.alias)
        return row.rowField ? { ...column, proportion: '', binding: `{{${alias}.${row.rowField}}}`, ...row } : column
      })
      state.collection = String(command.collection); state.alias = alias
    }
    if (command.kind === 'updateTableColumnFooter') edit((column) => ({ ...column, proportion: '', footer: command.footer as Footer, footerOf: String(command.footerOf), footerFormat: String(command.footerFormat) }))
    if (command.kind === 'updateTableColumnExpression') edit((column) => ({ ...column, proportion: '', binding: String(command.binding), ...mockRowBinding(String(command.binding), state.alias) }))
    if (command.kind === 'updateTableColumn' && command.field === 'align') edit((column) => ({ ...column, proportion: '', align: command.value as ColumnFixture['align'] }))
    if (command.kind === 'updateTableColumn' && command.field === 'header') edit((column) => ({ ...column, proportion: '', header: String(command.value) }))
    if (command.kind === 'updateTableColumn' && command.field === 'width') edit((column) => ({ ...column, proportion: '', width: Number(command.value) * 1000 }))
    if (command.kind === 'removeTableColumn') state.columns = state.columns.filter((column) => column.id !== columnId)
    if (command.kind === 'addTableColumn') {
      let width = 72000
      const remaining = (over.bands ?? canvas.bands).find((band) => band.name === 'content')!.width - state.componentX
      if (state.columns.reduce((sum, column) => sum + column.width, 0) + width > remaining) {
        const widest = state.columns.reduce((best, column, index) => column.width > 1 && (best < 0 || column.width > state.columns[best]!.width) ? index : best, -1)
        if (widest < 0) throw new Error('no splittable column')
        width = Math.floor(state.columns[widest]!.width / 2)
        state.columns = state.columns.map((column, index) => index === widest ? { ...column, proportion: '', width: column.width - width } : column)
      }
      state.columns = [...state.columns.slice(0, Number(command.index)), { id: `n${state.columns.length + 1}`, header: `Column ${state.columns.length + 1}`, width, align: 'left', rowField: '', footer: '', footerOf: '', footerFormat: '' }, ...state.columns.slice(Number(command.index))]
    }
    if (command.kind === 'moveTableColumn') { const moving = state.columns.find((column) => column.id === columnId); if (moving) { const rest = state.columns.filter((column) => column.id !== columnId); state.columns = [...rest.slice(0, Number(command.toIndex)), moving, ...rest.slice(Number(command.toIndex))] } }
    if (command.kind === 'setTableHeaderHeight') state.header = { ...state.header, headerHeight: Number(command.height) * 1000 }
    if (command.kind === 'setTableAltRowBackground') state.header = { ...state.header, altRowBackground: command.op === 'clear' ? '' : String(command.value) }
    if (command.kind === 'updateTableHeaderStyle') {
      const key = headerStyleKeyFor(String(command.field))
      // A clear REMOVES the key, which the projection reports as the field's
      // empty value — '' for a string, 0 for a length. Clearing what is already
      // empty therefore leaves canonical form untouched, which is the whole of
      // the no-op arm.
      // ⚠ THE `set` BRANCH IS A FUNCTION AND NOT A VALUE, because a CLEAR
      // carries no `value` at all: computed eagerly, the array branch read
      // `undefined.join(',')` and threw, so every edge clear arrived at the
      // dialog as a refusal rather than as the no-op it is.
      const cleared = NUMERIC_HEADER_KEYS.includes(key) ? 0 : ''
      const set = () => ARRAY_HEADER_KEYS.includes(key) ? (command.value as ReadonlyArray<string>).join(',')
        : NUMERIC_HEADER_KEYS.includes(key) ? Number(command.value) * 1000
        : MILLIPOINT_STRING_HEADER_KEYS.includes(key) ? millipointString(command.value)
        : String(command.value)
      state.header = { ...state.header, [key]: command.op === 'clear' ? cleared : set() }
    }
  }
  const snap = () => ({ ...snapshotOf(over), revision: state.revision, canUndo: history.undo.length > 0, canRedo: history.redo.length > 0 })
  const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
    if (operation === 'command') {
      const text = new TextDecoder().decode(payload); commands.push(text)
      const parsed = JSON.parse(text) as Readonly<Record<string, unknown>>
      if (options.pauseCommandAt === commands.length) await heldCommand
      // A REFUSED COMMAND IS RECORDED IN `commands` — it did reach the engine —
      // and then rejects before `apply`. So no state moves, no history entry is
      // pushed and the revision stands still: `install` is never reached, and
      // there is nothing for Cancel to unwind.
      if (options.refuseCommand?.(parsed) === true) throw Object.assign(new Error(REFUSED_COMMAND), { elementId: 'e7' })
      const before = canonical()
      apply(parsed)
      // NO CHANGE, NO REVISION AND NO HISTORY ENTRY — Go returns
      // `e.Snapshot(), nil` before pushUndo, before `e.redo = nil` and before
      // `install`, which is the sole site of `e.revision++`.
      if (canonical() !== before) { history.undo.push(before); history.redo = []; state.revision++ }
      return { snapshot: snap() }
    }
    if (operation === 'undo') {
      undos++
      if (options.pauseUndoAt === undos) await heldUndo
      if (options.failUndoAt === undos || history.undo.length === 0) throw Object.assign(new Error('Nothing to undo'), { code: 'UNDO_UNAVAILABLE' })
      history.redo.push(canonical())
      restore(history.undo.pop() as string)
      state.revision++
      return { snapshot: snap() }
    }
    if (operation === 'redo') {
      if (history.redo.length === 0) throw Object.assign(new Error('Nothing to redo'), { code: 'REDO_UNAVAILABLE' })
      history.undo.push(canonical())
      restore(history.redo.pop() as string)
      state.revision++
      return { snapshot: snap() }
    }
    if (operation === 'table-columns') return { snapshot: snap(), tableColumns: { revision: state.revision, table: { tableId: 'e7', sizing: 'points' as const, collection: state.collection, alias: state.alias, ...state.header, totalWidth: state.columns.reduce((sum, col) => sum + col.width, 0), columns: projected(state.columns, state.alias) } } }
    return { snapshot: snap() }
  })
  return { state, commands, request, canonical, releaseUndo, releaseCommand, engine: { request } as unknown as EngineClient, snapshot: snapshotOf(over) }
}

const openEditor = async (harness: ReturnType<typeof tableEngine>, sampleJson?: string) => {
  const sample = sampleJson === undefined ? undefined : acceptSampleData('c.json', new TextEncoder().encode(sampleJson).buffer)
  render(<App engine={harness.engine} initialSnapshot={harness.snapshot} initialSampleData={sample} />)
  fireEvent.click(screen.getByRole('button', { name: 'table component e7' }))
  fireEvent.click(screen.getByRole('button', { name: 'Configure columns' }))
  return screen.findByRole('dialog', { name: 'Table Editor' })
}

const cellOf = (element: Element | null) => (element as HTMLElement | null)?.dataset.matrixCell
const activeCell = () => cellOf(document.activeElement)
const press = (key: string) => fireEvent.keyDown(document.activeElement!, { key })

describe('the table editor matrix lattice', () => {
  it('steps between rows with ArrowUp and ArrowDown, which no single-column fixture can exercise', async () => {
    await openEditor(tableEngine())
    const first = screen.getByRole('textbox', { name: 'Header for column 1' })
    first.focus()
    expect(activeCell()).toBe(cellOf(first))
    press('ArrowDown')
    expect(document.activeElement).toBe(screen.getByRole('textbox', { name: 'Header for column 2' }))
    press('ArrowDown')
    expect(document.activeElement).toBe(screen.getByRole('textbox', { name: 'Header for column 3' }))
    // The lattice does not wrap: the last row's ArrowDown is a no-op, and the
    // first row's ArrowUp is too.
    press('ArrowDown')
    expect(document.activeElement).toBe(screen.getByRole('textbox', { name: 'Header for column 3' }))
    press('ArrowUp')
    press('ArrowUp')
    expect(document.activeElement).toBe(first)
    press('ArrowUp')
    expect(document.activeElement).toBe(first)
  })

  // A header label may hold a line feed, so its textarea owns ArrowUp/ArrowDown
  // while the caret has a line to move to, and hands them to the lattice only
  // at its first (ArrowUp) or last (ArrowDown) line.
  it('lets a multi-line header move its caret before the lattice claims ArrowUp', async () => {
    await openEditor(tableEngine())
    const second = screen.getByRole('textbox', { name: 'Header for column 2' }) as HTMLTextAreaElement
    fireEvent.change(second, { target: { value: 'A\nB' } })
    second.focus()
    second.setSelectionRange(3, 3)
    const onLineTwo = fireEvent.keyDown(second, { key: 'ArrowUp' })
    expect(onLineTwo, 'the textarea keeps its default caret move').toBe(true)
    expect(document.activeElement).toBe(second)
    second.setSelectionRange(1, 1)
    press('ArrowUp')
    expect(document.activeElement).toBe(screen.getByRole('textbox', { name: 'Header for column 1' }))
  })

  it('lets a multi-line header move its caret before the lattice claims ArrowDown', async () => {
    await openEditor(tableEngine())
    const second = screen.getByRole('textbox', { name: 'Header for column 2' }) as HTMLTextAreaElement
    fireEvent.change(second, { target: { value: 'A\nB' } })
    second.focus()
    second.setSelectionRange(1, 1)
    const onLineOne = fireEvent.keyDown(second, { key: 'ArrowDown' })
    expect(onLineOne, 'the textarea keeps its default caret move').toBe(true)
    expect(document.activeElement).toBe(second)
    second.setSelectionRange(2, 2)
    press('ArrowDown')
    expect(document.activeElement).toBe(screen.getByRole('textbox', { name: 'Header for column 3' }))
  })

  it('sizes a header textarea to its line count as the author types', async () => {
    await openEditor(tableEngine())
    const box = screen.getByRole('textbox', { name: 'Header for column 1' }) as HTMLTextAreaElement
    fireEvent.change(box, { target: { value: 'A' } })
    expect(box.rows).toBe(1)
    fireEvent.change(box, { target: { value: 'A\nB' } })
    expect(box.rows).toBe(2)
    fireEvent.change(box, { target: { value: '' } })
    expect(box.rows).toBe(1)
  })

  it('skips a cell it may not land on vertically rather than stopping at it', async () => {
    // Column 2 aggregates nothing, so its footer format is not a place focus
    // may land — disabled today, absent after the rebuild. Either way the
    // vertical scan must walk PAST it to column 3 rather than stopping.
    await openEditor(tableEngine())
    const format = screen.getByRole('textbox', { name: 'Footer format for column 1' })
    format.focus()
    press('ArrowDown')
    expect(document.activeElement).not.toBe(document.body)
    expect(document.activeElement).toBe(screen.getByRole('textbox', { name: 'Footer format for column 3' }))
  })

  it('skips a cell it may not land on horizontally, and Home and End reach the row\'s first and last enabled controls', async () => {
    await openEditor(tableEngine())
    // COUNT TAKES NO SOURCE, so column 3's source is not a landing place —
    // disabled today, absent after the rebuild. Walking LEFT from its format
    // must arrive at the aggregate rather than at the hole between them.
    const format = screen.getByRole('textbox', { name: 'Footer format for column 3' })
    format.focus()
    press('ArrowLeft')
    expect(document.activeElement).toBe(screen.getByRole('combobox', { name: 'Footer aggregate for column 3' }))
    // Row 1 cannot move earlier and row 3 cannot move later, so each end row
    // has one disabled affordance that Home/End must decline to land on.
    const firstRowHeader = screen.getByRole('textbox', { name: 'Header for column 1' })
    firstRowHeader.focus()
    press('Home')
    expect(document.activeElement).not.toBe(screen.getByRole('button', { name: 'Move column 1 earlier' }))
    expect(screen.getByRole('button', { name: 'Move column 1 earlier' })).toBeDisabled()
    expect((document.activeElement as HTMLElement).matches(':disabled')).toBe(false)
    const lastRowHeader = screen.getByRole('textbox', { name: 'Header for column 3' })
    lastRowHeader.focus()
    press('End')
    expect(document.activeElement).not.toBe(screen.getByRole('button', { name: 'Move column 3 later' }))
    expect(screen.getByRole('button', { name: 'Move column 3 later' })).toBeDisabled()
    expect((document.activeElement as HTMLElement).matches(':disabled')).toBe(false)
  })

  it('reaches every enabled control in a row by arrow key from that row\'s first cell', async () => {
    const dialog = await openEditor(tableEngine())
    // Row 1 carries the maximal shape — an aggregate with BOTH a source and a
    // format — so its walk visits the widest lattice the panel can draw.
    const header = screen.getByRole('textbox', { name: 'Header for column 1' })
    header.focus()
    press('Home')
    const visited = new Set<string>()
    for (let step = 0; step < 40; step++) {
      const cell = activeCell()
      if (cell === undefined || visited.has(cell)) break
      visited.add(cell)
      fireEvent.keyDown(document.activeElement!, { key: 'ArrowRight', altKey: true })
    }
    const enabledInRow = Array.from(dialog.querySelectorAll<HTMLElement>('[data-matrix-cell^="0:"]')).filter((cell) => !cell.matches(':disabled'))
    expect(enabledInRow.length).toBeGreaterThan(1)
    expect([...visited].sort()).toEqual(enabledInRow.map((cell) => cell.dataset.matrixCell!).sort())
  })
})

describe('the table editor scope, budget and summary', () => {
  it('states the collection, the sample item count and the band', async () => {
    // 34 items, of which the parser keeps only SAMPLE_LIMITS.items as children:
    // the count on the node is the true one and is what must be reported.
    const items = Array.from({ length: 34 }, (_, index) => `{"amount":${index}}`).join(',')
    await openEditor(tableEngine(), `{"transactions":[${items}]}`)
    expect(screen.getByRole('status', { name: 'Table scope' })).toHaveTextContent('transactions[] · 34 items in sample · band: content')
  })

  it('says the item count is unknown rather than showing a number when no sample is loaded', async () => {
    await openEditor(tableEngine())
    const scope = screen.getByRole('status', { name: 'Table scope' })
    expect(scope).toHaveTextContent('transactions[]')
    expect(scope).toHaveTextContent('item count unknown')
    expect(scope).toHaveTextContent('band: content')
    expect(scope.textContent).not.toMatch(/\d+ items/)
  })

  it('says the item count is unknown when the sampled collection carries no count', async () => {
    // A collection the parser truncated away entirely — the node exists with no
    // `count`, and absence there means unknown, never zero.
    await openEditor(tableEngine(), '{"other":[{"a":1}]}')
    const scope = screen.getByRole('status', { name: 'Table scope' })
    expect(scope).toHaveTextContent('item count unknown')
    expect(scope.textContent).not.toContain('0 items')
  })

  it('states the width budget under, exactly at, and over the space the band leaves', async () => {
    // 72 + 60 + 42 = 174pt against a band remainder of 500pt.
    await openEditor(tableEngine())
    const budget = screen.getByRole('status', { name: 'Width budget' })
    expect(budget).toHaveTextContent('174.0')
    expect(budget).toHaveTextContent('500.0')
    expect(budget).not.toHaveTextContent('exact')
    expect(budget).toHaveTextContent('326.0 to spare')
    // THE UNIT IS NOT SPELLED ON ONE FIGURE OUT OF THREE. All three are points
    // and none of them says so here; the `pt` an author reads sits beside the
    // WIDTH box they type into.
    expect(budget.textContent).not.toContain('pt')
  })

  it('carries the exact badge only when the columns fill the band remainder exactly', async () => {
    const exact: ReadonlyArray<ColumnFixture> = [
      { ...defaultColumns[0]!, width: 250000 },
      { ...defaultColumns[1]!, width: 150000 },
      { ...defaultColumns[2]!, width: 100000 },
    ]
    await openEditor(tableEngine(exact))
    const budget = screen.getByRole('status', { name: 'Width budget' })
    expect(budget).toHaveTextContent('exact')
    expect(budget).toHaveTextContent('500.0 of 500.0')
  })

  it('states the overflow in the author\'s terms when a loaded file exceeds the band', async () => {
    const over: ReadonlyArray<ColumnFixture> = [
      { ...defaultColumns[0]!, width: 400000 },
      { ...defaultColumns[1]!, width: 150000 },
      { ...defaultColumns[2]!, width: 100000 },
    ]
    await openEditor(tableEngine(over))
    const budget = screen.getByRole('status', { name: 'Width budget' })
    expect(budget).not.toHaveTextContent('exact')
    expect(budget).toHaveTextContent('150.0 over')
    expect(budget.textContent).not.toContain('pt')
  })

  it('summarises the columns and the aggregates in the footer bar', async () => {
    await openEditor(tableEngine())
    expect(screen.getByRole('status', { name: 'Column summary' })).toHaveTextContent('3 columns · 2 aggregates')
    // STORY 14.7b — THE ONE `Close Table Editor` IS NOW THE `Cancel` / `Done`
    // PAIR, and the summary is unchanged beside it.
    const footer = within(screen.getByRole('dialog', { name: 'Table Editor' }))
    expect(footer.getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
    expect(footer.getByRole('button', { name: 'Done' })).toBeInTheDocument()
    expect(footer.queryByRole('button', { name: 'Close Table Editor' })).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// THE ABSENCE FENCES (D-14.5.1).
//
// ⚠ THE FENCES READ THE DOM DIRECTLY RATHER THAN THROUGH ROLE QUERIES, and
// both halves of that are deliberate. `within(x)` cannot see a violation ON
// `x`, and Testing Library's role queries skip `aria-hidden` subtrees by
// default — so a fence written as `within(grid).queryAllByRole('columnheader')`
// is blind to the container itself and to anything hidden inside it. Each fence
// below is therefore proved by ITS OWN REDS: every forbidden thing is planted
// back at every position it could occupy — on the scanned root, as a direct
// child, inside a row, inside a cell, and inside an `aria-hidden` subtree — and
// the fence is asserted to red at all five.
// ---------------------------------------------------------------------------

const namesOf = (root: Element, selector: string): ReadonlyArray<string> =>
  [...(root.matches(selector) ? [root] : []), ...Array.from(root.querySelectorAll(selector))]
    .map((node) => (node.getAttribute('aria-label') ?? node.textContent ?? '').replace(/\s+/g, ' ').trim())

const columnHeaderNames = (root: Element) => namesOf(root, '[role="columnheader"]')
const RETIRED_COLUMN_HEADERS = ['Move earlier', 'Move later', 'Remove', 'Add after']
const retiredColumnHeaders = (root: Element) => columnHeaderNames(root).filter((name) => RETIRED_COLUMN_HEADERS.includes(name))

// Every position a planted violation could occupy inside a grid, as five roots
// the fence is run over one at a time.
const plantedEverywhere = (offender: string, markup: (id: string) => string): ReadonlyArray<readonly [string, Element]> => {
  const build = (inner: string) => { const host = document.createElement('div'); host.innerHTML = inner; return host.firstElementChild! }
  return [
    [`${offender} · on the scanned root itself`, build(markup('root'))],
    [`${offender} · as a direct child of the grid`, build(`<div role="grid">${markup('child')}</div>`)],
    [`${offender} · inside a row`, build(`<div role="grid"><div role="row">${markup('row')}</div></div>`)],
    [`${offender} · inside a cell`, build(`<div role="grid"><div role="row"><span role="gridcell">${markup('cell')}</span></div></div>`)],
    [`${offender} · inside an aria-hidden subtree`, build(`<div role="grid"><div aria-hidden="true">${markup('hidden')}</div></div>`)],
  ]
}

describe('the seven columns the design draws, and the four that left', () => {
  it('carries exactly seven columnheaders, spelled as the design spells them', async () => {
    await openEditor(tableEngine())
    const grid = screen.getByRole('grid', { name: 'Table columns' })
    expect(columnHeaderNames(grid)).toEqual(['#', 'HEADER LABEL', 'BINDING', 'WIDTH', 'HEADER ALIGN', 'CELL ALIGN', 'FOOTER AGGREGATE'])
    expect(retiredColumnHeaders(grid)).toEqual([])
    expect(grid).toHaveAttribute('aria-colcount', '7')
  })

  it('reds when any of the four retired columnheaders is put back, at every position it could occupy', () => {
    for (const offender of RETIRED_COLUMN_HEADERS) {
      for (const [where, root] of plantedEverywhere(offender, () => `<span role="columnheader">${offender}</span>`)) {
        expect(retiredColumnHeaders(root), where).toEqual([offender])
      }
    }
  })

  it('offers one named full binding input in the bound-field cell', async () => {
    await openEditor(tableEngine())
    const cell = screen.getByRole('grid', { name: 'Table columns' }).querySelector('.matrix-row [aria-colindex="3"]') as HTMLElement
    expect(cell.className).toContain('matrix-bound')
    expect(within(cell).getByRole('combobox', { name: 'Binding for column 1' })).toHaveValue('{{row.amount}}')
    expect(within(cell).queryByRole('status')).toBeNull()
    expect(cell.querySelectorAll('input')).toHaveLength(1)
  })

  it('keeps reorder and remove as named, keyboard-operable row affordances', async () => {
    const harness = tableEngine()
    await openEditor(harness)
    expect(screen.getByRole('button', { name: 'Move column 1 earlier' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Move column 1 earlier' })).toHaveAttribute('title', 'Column 1 is already first')
    expect(screen.getByRole('button', { name: 'Move column 3 later' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Move column 3 later' })).toHaveAttribute('title', 'Column 3 is already last')
    const later = screen.getByRole('button', { name: 'Move column 1 later' })
    later.focus()
    fireEvent.keyDown(later, { key: ' ' })
    fireEvent.click(later)
    await waitFor(() => expect(harness.state.columns.map((column) => column.id)).toEqual(['c2', 'c1', 'c3']))
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Header for column 1' })).toHaveValue('Date'))
  })

  it('appends through one Add column control below the grid, and reaches a position by reorder', async () => {
    const harness = tableEngine()
    await openEditor(harness)
    expect(screen.queryByRole('button', { name: 'Add column after column 1' })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Add column' }))
    await waitFor(() => expect(harness.state.columns).toHaveLength(4))
    expect(harness.state.columns[3]!.id).toBe('n4')
    expect(await screen.findByRole('textbox', { name: 'Header for column 4' })).toHaveValue('Column 4')
  })

  it('keeps the empty state, its stated reason and its own Add column control', async () => {
    await openEditor(tableEngine([]))
    expect(screen.getByText('No columns yet. Add a column to start the matrix.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add column' })).toBeEnabled()
    expect(screen.queryByRole('grid', { name: 'Table columns' })).toBeNull()
  })
})

describe('the ALIGN cell is the inspector\'s segmented control', () => {
  const alignGroups = (root: Element) => [...(root.matches('[role="group"]') ? [root] : []), ...Array.from(root.querySelectorAll('[role="group"]'))].filter((group) => (group.getAttribute('aria-label') ?? '').startsWith('Cell alignment for column '))
  const ALLOWED = ['Align left', 'Align center', 'Align right']
  const alignVocabularyViolations = (root: Element): ReadonlyArray<string> =>
    alignGroups(root).flatMap((group) => Array.from(group.querySelectorAll('button')).map((button) => button.getAttribute('aria-label') ?? ''))
      .filter((label) => !ALLOWED.some((allowed) => label.startsWith(`${allowed} for column `)))

  it('renders the shared module\'s three segments, and no fourth', async () => {
    const dialog = await openEditor(tableEngine())
    const groups = alignGroups(dialog)
    expect(groups).toHaveLength(3)
    const segments = Array.from(groups[0]!.querySelectorAll('button'))
    expect(segments.map((segment) => segment.getAttribute('aria-label'))).toEqual(['Align left for column 1', 'Align center for column 1', 'Align right for column 1'])
    expect(alignVocabularyViolations(dialog)).toEqual([])
    // LITERALLY THE INSPECTOR'S CONTROL: the same class, inside the same
    // container class, drawing the same paths out of the same module. A copy
    // that merely looked alike would have to reproduce all three.
    expect(groups[0]!.className).toBe('property-segmented')
    expect(segments.map((segment) => segment.className)).toEqual(['property-segment', 'property-segment', 'property-segment'])
    expect(segments.map((segment) => segment.querySelector('svg.segment-icon path')?.getAttribute('d'))).toEqual([alignGlyphs.left, alignGlyphs.center, alignGlyphs.right])
    expect(alignSegments.map((segment) => segment.value)).toEqual(['left', 'center', 'right'])
  })

  it('reds when a fourth segment is offered, at every position it could occupy', () => {
    // AC2 IS PROVED BY ADDING `justify` BACK, not by reverting the control. A
    // column's align is `ColumnAlignTokens`, which has three members; the
    // four-segment array belongs to the inspector's all-text selection alone.
    for (const [where, root] of plantedEverywhere('Align justify', () => '<div role="group" aria-label="Cell alignment for column 1"><button aria-label="Align justify for column 1"></button></div>')) {
      expect(alignVocabularyViolations(root), where).toEqual(['Align justify for column 1'])
    }
  })

  it('shows the committed alignment and commits a new one through the engine', async () => {
    const harness = tableEngine()
    await openEditor(harness)
    expect(screen.getByRole('button', { name: 'Align right for column 1' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Align left for column 1' })).toHaveAttribute('aria-pressed', 'false')
    fireEvent.click(screen.getByRole('button', { name: 'Align center for column 1' }))
    await waitFor(() => expect(harness.commands).toContain('{"kind":"updateTableColumn","version":1,"id":"e7","columnId":"c1","field":"align","value":"center"}'))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Align center for column 1' })).toHaveAttribute('aria-pressed', 'true'))
  })
})

describe('the collapsed FOOTER AGGREGATE and what it reveals', () => {
  const footerControlNames = (root: Element) => namesOf(root, 'input, select, textarea')
  const revealedFooterControls = (root: Element, column: number) => footerControlNames(root).filter((name) => name === `Footer source for column ${column}` || name === `Footer format for column ${column}`)
  const rowOf = (dialog: Element, index: number) => dialog.querySelectorAll('.matrix-row')[index]!

  it('offers one control reading none, and no source and no format anywhere in that row', async () => {
    const dialog = await openEditor(tableEngine())
    const aggregate = screen.getByRole('combobox', { name: 'Footer aggregate for column 2' })
    expect(aggregate).toHaveValue('')
    expect(Array.from(aggregate.querySelectorAll('option')).map((option) => option.textContent)).toEqual(['none', 'sum', 'avg', 'count'])
    expect(revealedFooterControls(rowOf(dialog, 1), 2)).toEqual([])
  })

  it('reveals a format but no source for count, and both for sum', async () => {
    const dialog = await openEditor(tableEngine())
    expect(revealedFooterControls(rowOf(dialog, 2), 3)).toEqual(['Footer format for column 3'])
    expect(revealedFooterControls(rowOf(dialog, 0), 1)).toEqual(['Footer source for column 1', 'Footer format for column 1'])
    expect(screen.getByRole('textbox', { name: 'Footer source for column 1' })).toHaveValue('transactions.amount')
  })

  it('reds when a source or a format is put back into a row that has no aggregate, at every position', () => {
    for (const control of ['Footer source for column 2', 'Footer format for column 2']) {
      for (const [where, root] of plantedEverywhere(control, () => `<input aria-label="${control}" />`)) {
        expect(revealedFooterControls(root, 2), where).toEqual([control])
      }
    }
  })

  it('keeps focus in the row when the aggregate is cleared out from under the source field', async () => {
    // THE HAZARD `focusCell` CARRIED UNTIL THIS STORY. A revealed cell that
    // disappears while it holds focus left `document.activeElement` on
    // `document.body`, because `focusCell` returned early on an ABSENT cell
    // instead of falling back the way it does for a disabled one.
    await openEditor(tableEngine())
    const source = screen.getByRole('textbox', { name: 'Footer source for column 1' })
    source.focus()
    expect(activeCell()).toBe('0:13')
    fireEvent.change(screen.getByRole('combobox', { name: 'Footer aggregate for column 1' }), { target: { value: '' } })
    await waitFor(() => expect(screen.queryByRole('textbox', { name: 'Footer source for column 1' })).toBeNull())
    expect(document.activeElement).not.toBe(document.body)
    expect(activeCell()).toMatch(/^0:/)
    expect(document.activeElement).toBe(screen.getByRole('combobox', { name: 'Footer aggregate for column 1' }))
  })

  it('commits a revealed source through the engine and reads it back', async () => {
    const harness = tableEngine()
    await openEditor(harness)
    const source = screen.getByRole('textbox', { name: 'Footer source for column 1' })
    fireEvent.blur(source, { target: { value: 'transactions.gross' } })
    await waitFor(() => expect(harness.state.columns[0]!.footerOf).toBe('transactions.gross'))
    await waitFor(() => expect(screen.getByRole('status', { name: 'Column summary' })).toHaveTextContent('3 columns · 2 aggregates'))
  })
})

describe('the row scope stays editable here, and the document answers', () => {
  it('commits a new collection and reads the engine\'s answer back out of the scope header', async () => {
    const harness = tableEngine()
    await openEditor(harness)
    const collection = screen.getByRole('combobox', { name: 'Root collection' })
    fireEvent.blur(collection, { target: { value: 'invoices[]' } })
    await waitFor(() => expect(harness.commands).toContain('{"kind":"configureTableBinding","version":1,"id":"e7","collection":"invoices[]","alias":""}'))
    // READ BACK THROUGH THE PROJECTION, never off the box that was typed into:
    // an uncontrolled input keeps whatever was typed whether or not the
    // document took it, so asserting its value would prove nothing.
    await waitFor(() => expect(screen.getByRole('status', { name: 'Table scope' })).toHaveTextContent('invoices[]'))
  })

  it('commits a new row alias and reads it back out of every column\'s projected binding', async () => {
    const harness = tableEngine()
    await openEditor(harness)
    const alias = screen.getByRole('textbox', { name: 'Row alias' })
    fireEvent.blur(alias, { target: { value: 'txn' } })
    await waitFor(() => expect(harness.commands).toContain('{"kind":"configureTableBinding","version":1,"id":"e7","collection":"transactions[]","alias":"txn"}'))
    await waitFor(() => expect(screen.getByRole('combobox', { name: 'Binding for column 1' })).toHaveValue('{{txn.amount}}'))
  })

  it('names itself to the accessibility tree and says where a collection is changed', async () => {
    await openEditor(tableEngine())
    const group = screen.getByRole('group', { name: 'Table row scope' })
    expect(group).toContainElement(screen.getByRole('combobox', { name: 'Root collection' }))
    expect(group).toContainElement(screen.getByRole('textbox', { name: 'Row alias' }))
    expect(group.textContent).toContain('Set the table’s collection and row alias here.')
    expect(group.textContent).not.toContain('local discovery hints only')
  })
})

// ---------------------------------------------------------------------------
// STEP-04 REVIEW FINDINGS, EACH WITH THE TEST THAT WOULD HAVE CAUGHT IT.
// ---------------------------------------------------------------------------

describe('the matrix sends no command it does not have to', () => {
  it('does not re-commit an alignment that is already committed', async () => {
    // Every other commit path in this panel suppresses an unchanged value; the
    // ALIGN cell did not, so clicking the segment already pressed sent a
    // command. THE COST IS NOT AN UNDO ENTRY — `folio-go/internal/wasm/engine.go`
    // short-circuits on `bytes.Equal(canonical, e.bytes)` and returns BEFORE
    // `pushUndo`, before the redo branch is cleared and before the revision
    // moves — it is a wasted engine round trip and a `busy` flicker over the
    // whole dialog. The guard is a convention this file already keeps
    // everywhere else, and no test clicked a pressed segment until this one.
    const harness = tableEngine()
    await openEditor(harness)
    const pressed = screen.getByRole('button', { name: 'Align right for column 1' })
    expect(pressed).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(pressed)
    await Promise.resolve()
    expect(harness.commands).toEqual([])
    // And the neighbouring segment still commits, so the guard is a guard and
    // not a control that stopped working.
    fireEvent.click(screen.getByRole('button', { name: 'Align left for column 1' }))
    await waitFor(() => expect(harness.commands).toHaveLength(1))
  })
})

describe('focus never lands on nobody inside an open modal', () => {
  it('lands on the empty state\'s Add column when the last column is removed', async () => {
    // ⚠ AND ESCAPE IS THE REASON THIS MATTERS. `trapDialog` is bound as
    // `onKeyDownCapture` on the dialog element, so a key pressed while focus
    // sits on `document.body` never reaches it: a stranded author could not
    // close the dialog with the keyboard at all.
    const harness = tableEngine([defaultColumns[0]!])
    await openEditor(harness)
    const remove = screen.getByRole('button', { name: 'Remove column 1' })
    remove.focus()
    fireEvent.click(remove)
    await waitFor(() => expect(harness.state.columns).toHaveLength(0))
    await screen.findByText('No columns yet. Add a column to start the matrix.')
    expect(document.activeElement).not.toBe(document.body)
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Add column' }))
    // The trap can hear a key again, which is the property the strand removed.
    fireEvent.keyDown(document.activeElement!, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).not.toBeInTheDocument())
  })

  it('keeps focus in the same row when a row is removed out from under it', async () => {
    // The case `nearestInRow` was rewritten for and the one the sibling test
    // does not reach: the surviving rows RENUMBER, so the row the author was on
    // is now a different column, and the cell they were on may not exist there.
    const harness = tableEngine()
    await openEditor(harness)
    const middleHeader = screen.getByRole('textbox', { name: 'Header for column 2' })
    middleHeader.focus()
    expect(activeCell()).toBe('1:3')
    fireEvent.click(screen.getByRole('button', { name: 'Remove column 2' }))
    await waitFor(() => expect(harness.state.columns.map((column) => column.id)).toEqual(['c1', 'c3']))
    await waitFor(() => expect(screen.queryByRole('textbox', { name: 'Header for column 3' })).toBeNull())
    expect(document.activeElement).not.toBe(document.body)
    // Row 1 is now the old column 3, and focus stayed on row 1 rather than
    // being thrown to the top of the matrix.
    expect(activeCell()).toMatch(/^1:/)
    expect(screen.getByRole('textbox', { name: 'Header for column 2' })).toHaveValue('Note')
  })

  it('does not reclaim focus that has already left the dialog', async () => {
    // The dialog's own `onFocusCapture` fires only for targets INSIDE the
    // dialog, so focus moving to something behind the modal left the panel
    // still believing the matrix held it — and the next re-projection dragged
    // the author back into the matrix from outside. The document-level
    // `focusin` listener is the only place that transition is observable.
    await openEditor(tableEngine())
    const cell = screen.getByRole('textbox', { name: 'Header for column 1' })
    cell.focus()
    const outside = screen.getByRole('button', { name: 'PREVIEW' })
    expect(screen.getByRole('dialog', { name: 'Table Editor' })).not.toContainElement(outside)
    outside.focus()
    outside.blur()
    expect(document.activeElement).toBe(document.body)
    fireEvent.change(screen.getByRole('combobox', { name: 'Footer aggregate for column 1' }), { target: { value: 'avg' } })
    await waitFor(() => expect(screen.getByRole('combobox', { name: 'Footer aggregate for column 1' })).toHaveValue('avg'))
    expect(document.activeElement).toBe(document.body)
  })
})

describe('the width budget states what it can and cannot know', () => {
  it('agrees with itself when the difference is smaller than the figure it prints', async () => {
    // 500.04pt against 500.0pt available: the printed numbers round to the same
    // figure while the stored values differ, so the line used to read
    // `Σ 500.0 of 500.0 available · 0.0 pt over` with NO exact badge — the
    // numbers saying exact and the badge saying not.
    const nearly: ReadonlyArray<ColumnFixture> = [
      { ...defaultColumns[0]!, width: 250040 },
      { ...defaultColumns[1]!, width: 150000 },
      { ...defaultColumns[2]!, width: 100000 },
    ]
    await openEditor(tableEngine(nearly))
    const budget = screen.getByRole('status', { name: 'Width budget' })
    expect(budget).toHaveTextContent('Σ 500.0 of 500.0 available')
    expect(budget).not.toHaveTextContent('exact')
    expect(budget).toHaveTextContent('under 0.1 over')
    expect(budget.textContent).not.toContain('0.0 over')
  })

  it('states that a table starting outside its band has no width that fits', async () => {
    // `band.width − table.x` goes NEGATIVE whenever the table's x sits past its
    // band's right edge — a hand-edited file, or a margin change that shrank the
    // band under an x the document already held. The read-out used to print
    // `of -50.0 available`, which is a negative quantity of space.
    const outside = { components: [{ ...canvas.components[0]!, x: 573276 }] }
    await openEditor(tableEngine(defaultColumns, outside))
    const budget = screen.getByRole('status', { name: 'Width budget' })
    expect(budget.textContent).not.toContain('-')
    expect(budget).toHaveTextContent('this table starts 50.0 outside its band, so no column width fits')
    expect(budget).not.toHaveTextContent('exact')
  })

  it('states the budget in the empty state, which is when it is most worth reading', async () => {
    // The foot carried both `Add column` and the budget and was gated on
    // `columns.length > 0`, so an author deciding how many columns will fit was
    // told nothing at exactly that moment.
    await openEditor(tableEngine([]))
    expect(screen.getByRole('status', { name: 'Width budget' })).toHaveTextContent('Σ 0.0 of 500.0 available · 500.0 to spare')
    // And still exactly ONE control named `Add column`: the empty state's own.
    expect(screen.getAllByRole('button', { name: 'Add column' })).toHaveLength(1)
  })
})

describe('the roving lattice addresses every control exactly once', () => {
  it('gives no two controls the same lattice address, and seats the aggregate after the last align segment', async () => {
    // `querySelector` resolves a duplicated `data-matrix-cell` by silently
    // picking one, so a lattice with two cells at one address has no failure of
    // its own to report. The align control owns a RUN of cells derived from
    // `alignSegments.length`; widening it used to have written the fourth
    // segment straight into the aggregate's slot.
    const dialog = await openEditor(tableEngine())
    const addresses = Array.from(dialog.querySelectorAll<HTMLElement>('[data-matrix-cell]')).map((cell) => cell.dataset.matrixCell!)
    expect(new Set(addresses).size).toBe(addresses.length)
    const columnOf = (element: Element) => Number((element as HTMLElement).dataset.matrixCell!.split(':')[1])
    const segments = Array.from(dialog.querySelectorAll('[aria-label^="Align "][aria-label$="for column 1"]'))
    expect(segments).toHaveLength(alignSegments.length)
    const segmentCells = segments.map(columnOf)
    expect(segmentCells).toEqual(Array.from({ length: alignSegments.length }, (_, offset) => segmentCells[0]! + offset))
    const aggregate = screen.getByRole('combobox', { name: 'Footer aggregate for column 1' })
    expect(columnOf(aggregate)).toBe(segmentCells[0]! + alignSegments.length)
  })
})

// ---------------------------------------------------------------------------
// STORY 14.7b — THE FOOTER'S `Cancel` / `Done` PAIR, AND THE COUNT BEHIND IT.
//
// Every arm here is driven through the SHIPPED GESTURE rather than through a
// prop: a changing edit is a real blur on a real matrix cell, and the no-op is a
// real click on the `×` beside `Header text colour`, which is `disabled={busy}`
// only and is never disabled on "already unset". The two boundary cases are the
// exception and say why in place.
// ---------------------------------------------------------------------------

// `Cancel` is `disabled={busy || overHistoryBound}`, so with the count under the
// bound its enabled state IS the dialog's idle state. Awaiting it is awaiting
// both the command and the re-projection behind it.
const idle = async () => { await waitFor(() => expect(screen.getByRole('button', { name: 'Cancel' })).toBeEnabled()) }

// A CHANGING EDIT, through the matrix cell an author types in.
const retitle = async (column: number, value: string) => {
  const box = screen.getByRole('textbox', { name: `Header for column ${column}` })
  fireEvent.change(box, { target: { value } })
  fireEvent.blur(box)
  await idle()
}

// THE REACHABLE NO-OP. `Clear Header text colour` on a field the projection
// reports as '' sends a LEGAL `clear` that removes a key which is not there, so
// canonical bytes do not move and the engine's revision stands still.
const clearHeaderColour = async () => {
  fireEvent.click(screen.getByRole('button', { name: 'Clear Header text colour' }))
  await idle()
}

const operations = (harness: ReturnType<typeof tableEngine>, operation: string) => harness.request.mock.calls.filter(([sent]) => sent === operation)
const reopen = async () => {
  fireEvent.click(screen.getByRole('button', { name: 'Configure columns' }))
  return screen.findByRole('dialog', { name: 'Table Editor' })
}

// ⚠ THIS WHOLE BLOCK CARRIES A RAISED TIMEOUT, AND THE REASON IS A MEASUREMENT,
// NOT A HUNCH. Every claim here drives the real dialog through several committed
// edits — each one a type, a blur and an engine round trip — so the tests are
// legitimately slow rather than accidentally slow. Locally the heaviest are
// ~1.4s, ~1.3s, ~1.2s, ~1.2s and ~1.0s against a 5s default, which reads as
// comfortable and is not: CI's runner is 3-5x slower on jsdom with `userEvent`
// (measured — simple tests that cost ~100ms here cost 300-900ms there), which
// puts five of them between 4s and 6s. Two of them TIMED OUT on CI at `482ea5d`
// while passing locally, and the rest were one scheduling hiccup behind.
//
// The block timeout is deliberately per-BLOCK rather than sprinkled over the two
// that happened to fail first. Fixing only those would have left four tests
// sitting just inside the limit — a suite-level instance of the same shape
// D-14.7.4 names, a guard that can only just pass, where the next slightly
// slower runner reds a claim that is true. The margin is wide for the reason the
// 101-edit test below states in its own comment: **a slow machine should red the
// claim, not the clock.**
describe('the table editor\'s Cancel discards what it counted', { timeout: 30_000 }, () => {
  it('counts only the edits the engine agreed changed the document, and unwinds exactly those', async () => {
    const harness = tableEngine()
    await openEditor(harness)
    // ONE EDIT IN AN EARLIER SESSION, KEPT WITH `Done`. It is what makes the
    // control at the foot of this test mean anything: without history from
    // BEFORE the dialog, an over-count would merely run out of undo rather than
    // reach back into the author's own work.
    await retitle(1, 'Committed earlier')
    fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    const earlier = harness.canonical()

    await reopen()
    const atOpen = harness.canonical()
    await retitle(2, 'When')
    await retitle(3, 'Remark')
    // TWO NO-OPS, and they are dispatched commands the engine accepted — the
    // count must not move for either.
    await clearHeaderColour()
    await clearHeaderColour()
    expect(harness.commands).toHaveLength(5)

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    // EXACTLY TWO, not four.
    expect(operations(harness, 'undo')).toHaveLength(2)
    expect(harness.canonical()).toBe(atOpen)
    expect(harness.canonical()).toBe(earlier)

    // THE CONTROL. Had the count come from DISPATCHES (5) or from accepted
    // commands (4) rather than from an observed revision change (2), the extra
    // steps would have run — and this is where they land: past the dialog-open
    // state, with the header an EARLIER session committed thrown away. That is
    // the single destructive failure guardrail 1 exists to prevent.
    await harness.request('undo')
    expect(harness.canonical()).not.toBe(atOpen)
    expect(harness.canonical()).not.toContain('Committed earlier')
  })

  it('does not count a command the engine refused, and leaves the existing error surface standing', async () => {
    // MATRIX ROW: "a rejected command is not counted". THE REFUSAL COMES FROM
    // THE ENGINE, not from the dialog declining to dispatch — that is what makes
    // the input SEPARATING. Three commands reach the engine and two of them move
    // the document, so a count keyed on dispatches (3) and a count keyed on an
    // observed revision change (2) give different answers here; a refusal the
    // dialog swallowed before sending would have them agree, and the test would
    // measure nothing. Proved the way the counted-discard test above proves it:
    // two genuinely changing edits, the refused one, then `Cancel`.
    const harness = tableEngine(defaultColumns, {}, { refuseCommand: (command) => command.value === 'Refused' })
    await openEditor(harness)
    const atOpen = harness.canonical()
    await retitle(1, 'Total')
    await retitle(2, 'When')
    await retitle(3, 'Refused')
    // ALL THREE REACHED THE ENGINE. The refused one is not a command that was
    // never sent.
    expect(harness.commands).toHaveLength(3)

    // THE EXISTING ERROR SURFACE IS UNCHANGED — the other half of this row. The
    // dialog's own `role="alert"` carries the engine's LOCATED sentence, and the
    // dialog stays open on it rather than tearing itself down.
    const failure = await screen.findByRole('alert')
    expect(failure).toHaveTextContent(`e7: ${REFUSED_COMMAND}`)
    expect(failure.className).toContain('file-message')
    expect(screen.getByRole('dialog', { name: 'Table Editor' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    // TWO, NOT THREE. The refusal never reached `install`, so it left no history
    // entry, and a third `undo` would have reached past the dialog-open state.
    expect(operations(harness, 'undo')).toHaveLength(2)
    expect(harness.canonical()).toBe(atOpen)
    // AND THE UNDO STACK IS EMPTY AT DIALOG-OPEN STATE, which is the same claim
    // read from the other side: had the count been 3, the third step would have
    // had nothing to consume and Cancel would have reported a failure instead of
    // a discard.
    await expect(harness.request('undo')).rejects.toThrow('Nothing to undo')
  })

  it('keeps every edit when the author presses Done, and sends no undo at all', async () => {
    const harness = tableEngine()
    await openEditor(harness)
    await retitle(1, 'Total')
    await retitle(2, 'When')
    await retitle(3, 'Remark')
    const kept = harness.canonical()
    fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    expect(operations(harness, 'undo')).toHaveLength(0)
    expect(harness.canonical()).toBe(kept)
    expect(harness.canonical()).toContain('Total')
  })

  it('treats Escape as Done rather than as Cancel, so it closes and keeps', async () => {
    const harness = tableEngine()
    const dialog = await openEditor(harness)
    await retitle(1, 'Total')
    await retitle(2, 'When')
    await retitle(3, 'Remark')
    const kept = harness.canonical()
    fireEvent.keyDown(dialog, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    expect(operations(harness, 'undo')).toHaveLength(0)
    expect(harness.canonical()).toBe(kept)
  })

  it('starts a reopened dialog at zero, because the count is a function of the session', async () => {
    const harness = tableEngine()
    await openEditor(harness)
    await retitle(1, 'Total')
    await retitle(2, 'When')
    fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    const kept = harness.canonical()

    await reopen()
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    expect(operations(harness, 'undo')).toHaveLength(0)
    expect(harness.canonical()).toBe(kept)
  })

  it('says nothing and sends nothing when there is nothing to discard', async () => {
    const harness = tableEngine()
    await openEditor(harness)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    expect(operations(harness, 'undo')).toHaveLength(0)
    expect(screen.queryByText(/Discarded/)).toBeNull()
  })

  it('states the discard and the honest limit on it, and the discarded edits are still redoable', async () => {
    const harness = tableEngine()
    await openEditor(harness)
    await retitle(1, 'Total')
    await retitle(2, 'When')
    await retitle(3, 'Remark')
    const beforeCancel = harness.canonical()
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    expect(operations(harness, 'undo')).toHaveLength(3)
    expect(harness.canonical()).not.toBe(beforeCancel)

    // THE NUMBER, AND THE LIMIT ON THE PROMISE. `Undo()` pushes onto redo before
    // restoring, so the discarded edits survive — but only until the next
    // committed command clears the engine's redo stack.
    const stated = screen.getByText(/Discarded 3 table editor edits/)
    expect(stated).toHaveAttribute('role', 'status')
    expect(stated).toHaveAttribute('aria-live', 'polite')
    expect(stated.textContent).toContain('Redo restores them until your next committed edit')

    // AC7, AND IT IS N PRESSES AND AN EXACT DOCUMENT, NOT ONE PRESS AND AN
    // INEQUALITY. "Redo restores them" is a claim about ALL THREE: a single redo
    // followed by `not.toEqual` is satisfied by an engine that restored one edit,
    // or a different edit, or half of one. Three presses and byte-equality
    // against the canonical form the document held immediately before `Cancel`
    // is the sentence's actual promise.
    for (const press of [1, 2, 3]) {
      const redo = screen.getByRole('button', { name: 'Redo' })
      await waitFor(() => expect(redo).toBeEnabled())
      fireEvent.click(redo)
      await waitFor(() => expect(operations(harness, 'redo')).toHaveLength(press))
    }
    expect(harness.canonical()).toBe(beforeCancel)
  })

  it('says it in the singular when exactly one edit is discarded', async () => {
    // THE SINGULAR ARM OF `discarded === 1 ? 'edit' : 'edits'` AND OF `'it' :
    // 'them'`. Every other discard here is 3, so both words rendered only in
    // their plural form and the ternaries were untested in one direction.
    const harness = tableEngine()
    await openEditor(harness)
    const atOpen = harness.canonical()
    await retitle(1, 'Total')

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    expect(operations(harness, 'undo')).toHaveLength(1)
    expect(harness.canonical()).toBe(atOpen)
    const stated = screen.getByText(/Discarded 1 table editor edit\./)
    expect(stated.textContent).toContain('Redo restores it until your next committed edit')
    // AND NOT THE PLURAL, in either place: a sentence that read "1 edits" or
    // "restores them" would satisfy a looser matcher.
    expect(stated.textContent).not.toContain('edits')
    expect(stated.textContent).not.toContain('them')
  })

  it('stops where it actually got to when an undo in the sequence fails, and stays open to say so', async () => {
    // The 3rd `undo` returns UNDO_UNAVAILABLE against a count of 4.
    const harness = tableEngine(defaultColumns, {}, { failUndoAt: 3 })
    await openEditor(harness)
    await retitle(1, 'Total')
    await retitle(2, 'When')
    await retitle(3, 'Remark')
    fireEvent.click(screen.getByRole('button', { name: 'Add column' }))
    await idle()

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    const failure = await screen.findByRole('alert')
    // THE DIALOG IS STILL OPEN. A closed dialog can state nothing, so closing is
    // the success path only.
    expect(screen.getByRole('dialog', { name: 'Table Editor' })).toBeInTheDocument()
    // BOTH NUMBERS, so a message claiming a completed discard cannot pass for
    // this one.
    expect(failure.textContent).toContain('Discarded 2 of 4 edits')
    expect(failure.textContent).toContain('The other 2 still stand')
    expect(failure.textContent).toContain('nothing left to undo')
    expect(operations(harness, 'undo')).toHaveLength(3)
    // AND ITS PROJECTION IS THE DOCUMENT IT REACHED, not the one it set out
    // from: two of the four edits were undone, so the fourth column is gone
    // again and the third column's header is back to 'Note'.
    //
    // ⚠ THE DOCUMENT IS READ FROM THE ENGINE AND THE COLUMN COUNT FROM THE DOM,
    // and the split is not a preference. The matrix's header box is UNCONTROLLED
    // and carries no `key`, so it keeps whatever was typed into it across a
    // re-projection — asserting its `value` would measure the author's typing,
    // not the document. The row COUNT is structural and the DOM does show it.
    await waitFor(() => expect(screen.queryByRole('textbox', { name: 'Header for column 4' })).toBeNull())
    expect(screen.getAllByRole('textbox', { name: /^Header for column/ })).toHaveLength(3)
    expect((JSON.parse(harness.canonical()) as { columns: ColumnFixture[] }).columns.map((column) => column.header)).toEqual(['Total', 'When', 'Note'])
  })

  // -------------------------------------------------------------------------
  // THE UNWIND IS NOT A MOMENT THE DIALOG CAN BE CLOSED IN.
  //
  // The sequence runs with the dialog STILL OPEN — closing is the success path
  // only — so `Done` and Escape are live gestures over a loop that is mid-flight.
  // Both tear the session down (`revokeTableEditor` advances
  // `tableEditorSession`), and the loop's own session guard then returns BEFORE
  // it installs the snapshot it reached: the engine ends k undos back while the
  // canvas, `snapshotRef` and the preview still show the pre-Cancel document,
  // with nothing on screen saying the two have parted company. That is what
  // `discarding` shuts, and it is `discarding` and never `busy` — `busy` is not
  // cleared by `setCurrentSnapshot`'s `clearDocumentInteraction` branch, so
  // gating a way out on it could leave a modal that cannot be closed at all.
  // -------------------------------------------------------------------------
  it.each([
    ['Done', (dialog: HTMLElement) => { const done = screen.getByRole('button', { name: 'Done' }); expect(done, 'Done must be disabled while the discard is unwinding').toBeDisabled(); fireEvent.click(done); expect(dialog).toBeInTheDocument() }],
    ['Escape', (dialog: HTMLElement) => { fireEvent.keyDown(dialog, { key: 'Escape' }) }],
  ])('does not let %s tear the discard down mid-unwind, and the engine never gets ahead of the screen', async (_name, dismiss) => {
    // THE SECOND UNDO OF THREE IS PARKED, so the loop is genuinely in flight when
    // the gesture lands: one undo has been applied to the document, two have not,
    // and the reached snapshot has not been installed anywhere yet.
    const harness = tableEngine(defaultColumns, {}, { pauseUndoAt: 2 })
    const dialog = await openEditor(harness)
    const atOpen = harness.canonical()
    await retitle(1, 'Total')
    await retitle(2, 'When')
    await retitle(3, 'Remark')
    await retitle(1, 'And one more')

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(operations(harness, 'undo')).toHaveLength(2))
    dismiss(dialog)
    await act(async () => { await Promise.resolve() })
    // THE DIALOG IS STILL HERE. Whether the gesture was refused at the control or
    // swallowed at the trap, what it must not do is end the session.
    expect(screen.getByRole('dialog', { name: 'Table Editor' })).toBeInTheDocument()

    harness.releaseUndo()
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    // ALL FOUR RAN AND THE DOCUMENT IS BACK AT DIALOG-OPEN STATE.
    expect(operations(harness, 'undo')).toHaveLength(4)
    expect(harness.canonical()).toBe(atOpen)
    // ⚠ AND THE SCREEN AGREES WITH THE ENGINE, which is the defect stated as an
    // assertion rather than as a count. A teardown mid-sequence makes the loop
    // return before `setCurrentSnapshot`, so the engine sits four undos back
    // while this read-out still shows the revision the last commit installed.
    expect(screen.getByTestId('engine-snapshot')).toHaveTextContent(`REVISION ${harness.state.revision}`)
    expect(screen.getByText(/Discarded 4 table editor edits/)).toBeInTheDocument()
  })

  it.each([
    ['Done', () => { expect(screen.getByRole('button', { name: 'Done' })).toBeEnabled(); fireEvent.click(screen.getByRole('button', { name: 'Done' })) }],
    ['Escape', (dialog: HTMLElement) => { fireEvent.keyDown(dialog, { key: 'Escape' }) }],
  ])('leaves %s working while an ordinary blur commit is in flight, and when nothing is', async (_name, dismiss) => {
    // THE CONTROL FOR THE PAIR ABOVE, and the reason the gate is `discarding` and
    // not `busy`. An ordinary commit raises `busy` too — `Cancel` is disabled
    // right here, which is how this test knows it is in flight — and the two ways
    // out must be untouched by it. A `busy`-gated Escape would also be a modal
    // with no exit the moment `busy` latched, which
    // `setCurrentSnapshot`'s `clearDocumentInteraction` branch can do.
    const harness = tableEngine()
    const dialog = await openEditor(harness)
    // IDLE FIRST, so "still works" has a baseline in this same test.
    expect(screen.getByRole('button', { name: 'Done' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeEnabled()

    const box = screen.getByRole('textbox', { name: 'Header for column 1' })
    fireEvent.change(box, { target: { value: 'Total' } })
    fireEvent.blur(box)
    expect(screen.getByRole('button', { name: 'Cancel' }), 'the commit must actually be in flight for this test to mean anything').toBeDisabled()
    dismiss(dialog)
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    // BOTH ARE `Done`: they close and KEEP, so no undo is sent by either.
    expect(operations(harness, 'undo')).toHaveLength(0)
    expect(screen.queryByText(/Discarded/)).toBeNull()
  })

  it('withdraws the discard sentence once it stops being true', async () => {
    const harness = tableEngine()
    await openEditor(harness)
    await retitle(1, 'Total')
    await retitle(2, 'When')
    await retitle(3, 'Remark')
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    expect(screen.getByText(/Discarded 3 table editor edits/)).toBeInTheDocument()

    // ⚠ THE SENTENCE PROMISES SOMETHING WITH AN EXPIRY: "Redo restores them until
    // your next committed edit". Go's next `install` sets `e.redo = nil`, so the
    // moment the document moves again the promise is FALSE — and a live region
    // still asserting it is worse than silence. An arrow nudge on the
    // still-selected table is the committed edit that ends it: it changes no
    // selection and never touches the edit count, which is why clearing the
    // sentence only where the COUNT is cleared would leave this case standing.
    fireEvent.keyDown(screen.getByLabelText('Canvas region'), { key: 'ArrowRight' })
    // ⚠ THESE TWO CARRY AN EXPLICIT BUDGET, AND THE BLOCK'S `timeout: 30_000`
    // IS NOT IT. That option bounds the TEST; `waitFor` runs on
    // @testing-library's own default of 1000ms, which no vitest timeout widens.
    // So the generous-looking block header says nothing about these lines.
    //
    // This test timed out HERE on CI at `1acc9cd` — `expected [] to have a
    // length of 1`, the moveComponent command absent after 1000ms — while the
    // whole file passed locally, and a re-run of the identical commit with no
    // code change went green, which is what marks it as the clock rather than
    // the claim.
    //
    // THE 1000ms WAS NEVER THE MARGIN IT LOOKED LIKE, and measuring the work
    // alone is what hides that. Instrumented here, this wait resolves in 8ms
    // both in isolation and under the full file — a 125x margin, which reads as
    // untouchable until you notice that 8ms bounds the CPU work and `waitFor`
    // budgets WALL CLOCK. A descheduled process pays wall clock for work it is
    // not doing. The same CI run spent 206s on this file and 5115ms and 6978ms
    // on single tests that cost about 100ms here — 50-70x — and at that factor
    // a budget with a 125x headroom is one scheduling hiccup from red. This is
    // `08781a6`'s finding ("the 30s was never as wide as it read") one level
    // down, where the block's own timeout cannot reach.
    //
    // 15s, not 30s: it must stay clear of the block's test timeout so a REAL
    // failure still surfaces as this assertion's diff rather than as a bare
    // "test timed out", which would report the clock and hide the claim — the
    // very inversion this comment exists to prevent.
    await waitFor(() => expect(harness.commands.filter((text) => text.includes('"kind":"moveComponent"'))).toHaveLength(1), { timeout: 15_000 })
    await waitFor(() => expect(screen.queryByText(/Discarded 3 table editor edits/)).toBeNull(), { timeout: 15_000 })
  })

  it('carries the count the application accumulated into the footer, over the engine\'s bound', async () => {
    // ⚠ THIS IS THE ONLY EXECUTING PROOF THAT `tableEditorEditCount` — the STATE
    // MIRROR beside the ref — reaches the dialog at all. Every other arm here
    // measures undo operations, which come from the REF; the two boundary tests
    // below render the dialog with a literal. So the mirror could be dropped
    // (`setTableEditorEdits` writing only `tableEditorEdits.current`) and nothing
    // would red, while a real author at 101 edits would see an enabled `Cancel`
    // that silently does nothing, because the loop refuses above the bound.
    //
    // ONE COLUMN, and the count is driven through the same shipped blur every
    // other arm uses — 101 of them, which is what the claim needs: the footer's
    // reason cannot be reached with fewer.
    const harness = tableEngine([defaultColumns[0]!])
    await openEditor(harness)
    const box = () => screen.getByRole('textbox', { name: 'Header for column 1' })
    // NOT `idle()`, AND NOT `waitFor`. That helper waits on `Cancel` becoming
    // enabled, which is the very thing this test drives to FALSE; and a hundred
    // `waitFor` polls cost more than the hundred commits do. `commitTableColumn`
    // awaits exactly two engine round trips against a synchronous mock, so
    // draining the microtask queue inside `act` settles each edit — and if it ever
    // stopped settling, the command count asserted immediately after the loop
    // would say so rather than the assertions quietly measuring a smaller count.
    const settle = async () => { await act(async () => { for (let turn = 0; turn < 6; turn++) await Promise.resolve() }) }
    for (let edit = 1; edit <= MAX_ENGINE_HISTORY_ENTRIES + 1; edit++) {
      const target = box()
      fireEvent.change(target, { target: { value: `Header ${edit}` } })
      fireEvent.blur(target)
      await settle()
      expect(box(), `edit ${edit} never settled`).toBeEnabled()
    }
    expect(harness.commands).toHaveLength(MAX_ENGINE_HISTORY_ENTRIES + 1)

    const note = screen.getByText(/Cancel is unavailable/)
    expect(note.textContent).toContain(`${MAX_ENGINE_HISTORY_ENTRIES + 1} edits`)
    expect(note.textContent).toContain(`${MAX_ENGINE_HISTORY_ENTRIES}-step history`)
    const cancel = screen.getByRole('button', { name: 'Cancel' })
    expect(cancel).toBeDisabled()
    expect(cancel).toHaveAttribute('aria-describedby', note.id)
    // AND THE WAY OUT THAT KEEPS THE WORK IS STILL THERE, which is the whole
    // reason a disabled Cancel is safe.
    expect(screen.getByRole('button', { name: 'Done' })).toBeEnabled()
    // ⚠ ITS OWN TIMEOUT, BECAUSE A HUNDRED AND ONE REAL COMMITS COST REAL TIME —
    // measured at ~6s here, against the 5s default. The alternative was to fake
    // the count, and a faked count cannot prove that the APPLICATION's mirror is
    // what the footer reads. The margin is deliberately wide so a slower machine
    // reds the claim rather than the clock.
    //
    // RAISED 30s → 60s. At `5677008` this test TIMED OUT on CI at 30s while the
    // whole file passed locally, and it was the only red in 1850 — the clock, not
    // the claim, exactly what the margin exists to prevent. The 30s was never as
    // wide as it read: the same runner took 196s over this file against 52s here,
    // a 3.7x that turns the ~6s measured above into ~26s, so 30s left about four
    // seconds of room and one scheduling hiccup spent it. 60s restores a margin
    // that is actually a margin. This is the block comment's own point arriving a
    // second time, now with the loop's real cost measured rather than estimated:
    // a guard that can only just pass reds a true claim on the next slower runner.
  }, 60_000)

  it('refuses Cancel in the footer while a local file operation is in flight, not only in the handler', async () => {
    // `cancelTableEditor` returns early on `fileBusy`, so without the same flag on
    // the button the author met an available-looking `Cancel` during a save that
    // swallowed the click and discarded nothing. Mirrored in both directions, as
    // the history bound already is.
    const harness = tableEngine()
    // A save target that never arrives holds `fileBusy` up for the whole test.
    const fileAccess = { open: vi.fn(), acquireSaveTarget: vi.fn(() => new Promise<never>(() => {})), writeSave: vi.fn() } as unknown as FileAccess
    render(<App engine={harness.engine} initialSnapshot={harness.snapshot} fileAccess={fileAccess} />)
    fireEvent.click(screen.getByRole('button', { name: 'table component e7' }))
    fireEvent.click(screen.getByRole('button', { name: 'Configure columns' }))
    await screen.findByRole('dialog', { name: 'Table Editor' })
    await retitle(1, 'Total')

    // Cmd/Ctrl+S sits ABOVE the modal guard and still saves from inside the
    // dialog, which is what puts this dialog in the state being tested.
    fireEvent.keyDown(screen.getByRole('button', { name: 'Done' }), { key: 's', ctrlKey: true })
    await waitFor(() => expect(fileAccess.acquireSaveTarget).toHaveBeenCalledOnce())
    const cancel = screen.getByRole('button', { name: 'Cancel' })
    expect(cancel).toBeDisabled()
    fireEvent.click(cancel)
    await act(async () => { await Promise.resolve() })
    expect(operations(harness, 'undo')).toHaveLength(0)
    expect(screen.getByRole('dialog', { name: 'Table Editor' })).toBeInTheDocument()
    // `Done` is not a document mutation, so a save in flight does not take it
    // away: the dialog still has a way out.
    expect(screen.getByRole('button', { name: 'Done' })).toBeEnabled()
  })

  // THE TWO SIDES OF THE HISTORY BOUND, AND THIS PAIR RENDERS THE DIALOG
  // DIRECTLY RATHER THAN DRIVING App — deliberately. The claim is about what the
  // FOOTER DOES WITH A COUNT, and reaching 101 committed edits through the real
  // gestures would spend a hundred round trips to set up a prop this dialog
  // simply receives. Everything else in this describe goes through App.
  const directProjection = { revision: 1, table: { tableId: 'e7', sizing: 'points' as const, totalWidth: defaultColumns.reduce((sum, col) => sum + col.width, 0), collection: 'transactions[]', alias: 'row', ...tableHeaderProjection, columns: projected(defaultColumns) } }
  // `busy`, `fileBusy` AND `discarding` ALL DEFAULT TO FALSE, so every call site
  // keeps exactly the meaning it had and each arm below passes only the one flag
  // it is about. `unmount` is returned so an arm can hold its own control — the
  // SAME count with the flag down — in one test without two dialogs sharing
  // `screen`.
  const renderFooter = (editCount: number, busy = false, fileBusy = false, discarding = false) => {
    const onCancel = vi.fn()
    const onClose = vi.fn()
    const { unmount } = render(<TableEditor projection={directProjection} busy={busy} fileBusy={fileBusy} discarding={discarding} candidates={[]} sampleAvailable={false} editCount={editCount} onClose={onClose} onCancel={onCancel} onAdd={vi.fn()} onRemove={vi.fn()} onMove={vi.fn()} onUpdate={vi.fn()} onTotalWidth={vi.fn()} onBinding={vi.fn()} onConfigure={vi.fn()} onFooter={vi.fn()} onHeaderHeight={vi.fn()} onAltRowBackground={vi.fn()} onHeaderStyle={vi.fn()} onMinHeight={vi.fn()} onRules={vi.fn()} onCellPadding={vi.fn()} />)
    return { onCancel, onClose, unmount }
  }

  it('keeps Cancel available at exactly the engine\'s history limit', () => {
    const { onCancel, onClose } = renderFooter(MAX_ENGINE_HISTORY_ENTRIES)
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeEnabled()
    expect(screen.queryByText(/Cancel is unavailable/)).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(onCancel).toHaveBeenCalledOnce()
    // Done and Escape are available here too, and they are the same act.
    expect(screen.getByRole('button', { name: 'Done' })).toBeEnabled()
    fireEvent.keyDown(screen.getByRole('dialog', { name: 'Table Editor' }), { key: 'Escape' })
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('disables Cancel one above the limit and states the reason in visible text', () => {
    const { onCancel, onClose } = renderFooter(MAX_ENGINE_HISTORY_ENTRIES + 1)
    const cancel = screen.getByRole('button', { name: 'Cancel' })
    expect(cancel).toBeDisabled()
    // A VISIBLE SENTENCE, not a bare grey-out and not a `title`. It names the
    // author's number and the engine's, and says what would go wrong.
    const note = screen.getByText(/Cancel is unavailable/)
    expect(note.textContent).toContain(`${MAX_ENGINE_HISTORY_ENTRIES + 1} edits`)
    expect(note.textContent).toContain(`${MAX_ENGINE_HISTORY_ENTRIES}-step history`)
    expect(note.textContent).toContain('land part-way')
    expect(cancel).toHaveAttribute('aria-describedby', note.id)
    // DONE AND ESCAPE STAY AVAILABLE. This is the state that forces Escape to
    // mean Done: if it meant Cancel the dialog would not be keyboard-dismissible
    // here at all.
    expect(screen.getByRole('button', { name: 'Done' })).toBeEnabled()
    fireEvent.keyDown(screen.getByRole('dialog', { name: 'Table Editor' }), { key: 'Escape' })
    expect(onClose).toHaveBeenCalledOnce()
    expect(onCancel).not.toHaveBeenCalled()

    // AND THE TRAP STILL WRAPS AT BOTH ENDS WITH `Cancel` GONE FROM ITS LIST.
    // `trapDialog` selects `button:not([disabled])`, so a disabled Cancel is not
    // in it and the wrap ends move — the third intended re-ordering of this
    // list. Both ends are re-derived from the DOM.
    const dialog = screen.getByRole('dialog', { name: 'Table Editor' })
    const tabbable = Array.from(dialog.querySelectorAll<HTMLElement>('button:not([disabled]), input:not([disabled]), select:not([disabled])')).filter((element) => element.tabIndex >= 0)
    expect(tabbable).not.toContain(cancel)
    const last = tabbable[tabbable.length - 1] as HTMLElement
    expect(last).toBe(screen.getByRole('button', { name: 'Done' }))
    last.focus()
    fireEvent.keyDown(last, { key: 'Tab' })
    expect(document.activeElement).toBe(tabbable[0])
    fireEvent.keyDown(document.activeElement!, { key: 'Tab', shiftKey: true })
    expect(document.activeElement).toBe(last)
  })

  it('disables Cancel while a command is in flight, at a count nowhere near the history bound', () => {
    // MATRIX ROW: "Cancel while a command is in flight". `Cancel` is
    // `disabled={busy || overHistoryBound}`, so THE COUNT IS THE SEPARATING
    // INPUT, not the flag: a test that set `busy` alongside a count of 101 could
    // not say which of the two conditions disabled the button, and would still
    // pass against a `Cancel` that ignored `busy` entirely. TWO is far under
    // MAX_ENGINE_HISTORY_ENTRIES, so only `busy` can be doing it here — and the
    // control at the foot of this test renders the SAME count with `busy` false
    // and finds Cancel enabled.
    const inFlight = renderFooter(2, true)
    const cancel = screen.getByRole('button', { name: 'Cancel' })
    expect(cancel).toBeDisabled()
    // AND IT IS NOT THE BOUND SAYING SO. The over-bound note is the only thing
    // that ever explains a disabled Cancel by the bound, and it is absent, as is
    // the `aria-describedby` that points at it.
    expect(screen.queryByText(/Cancel is unavailable/)).toBeNull()
    expect(cancel).not.toHaveAttribute('aria-describedby')
    // THE COUNT CANNOT MOVE UNDER THE LOOP, because the loop cannot be started:
    // a disabled button dispatches nothing.
    fireEvent.click(cancel)
    expect(inFlight.onCancel).not.toHaveBeenCalled()
    // `Done` stays available, so an in-flight command does not make the dialog a
    // trap — the same reason `Done` survives the over-bound state.
    expect(screen.getByRole('button', { name: 'Done' })).toBeEnabled()

    // THE CONTROL. Same count, nothing in flight: enabled, and it dispatches.
    // Without this arm the assertions above are satisfied by a `Cancel` that is
    // disabled for some other reason, or always.
    inFlight.unmount()
    const idleFooter = renderFooter(2)
    const enabled = screen.getByRole('button', { name: 'Cancel' })
    expect(enabled).toBeEnabled()
    fireEvent.click(enabled)
    expect(idleFooter.onCancel).toHaveBeenCalledOnce()
  })

  it('disables Cancel while a local file operation is in flight, and not by the bound', () => {
    // The third condition on this button, and the same separating shape the arm
    // above uses: TWO is far under MAX_ENGINE_HISTORY_ENTRIES and `busy` is false,
    // so only `fileBusy` can be disabling it here.
    const saving = renderFooter(2, false, true)
    const cancel = screen.getByRole('button', { name: 'Cancel' })
    expect(cancel).toBeDisabled()
    expect(screen.queryByText(/Cancel is unavailable/)).toBeNull()
    expect(cancel).not.toHaveAttribute('aria-describedby')
    fireEvent.click(cancel)
    expect(saving.onCancel).not.toHaveBeenCalled()
    // `Done` and Escape are unaffected: a save is not a reason to trap the author
    // in the dialog.
    expect(screen.getByRole('button', { name: 'Done' })).toBeEnabled()
    fireEvent.keyDown(screen.getByRole('dialog', { name: 'Table Editor' }), { key: 'Escape' })
    expect(saving.onClose).toHaveBeenCalledOnce()

    // THE CONTROL. Same count, no file operation: enabled, and it dispatches.
    saving.unmount()
    const idleFooter = renderFooter(2)
    const enabled = screen.getByRole('button', { name: 'Cancel' })
    expect(enabled).toBeEnabled()
    fireEvent.click(enabled)
    expect(idleFooter.onCancel).toHaveBeenCalledOnce()
  })

  it('shuts both ways out while the discard is unwinding, and only then', () => {
    // THE DIALOG-LEVEL HALF of the mid-unwind proof above: `discarding` is the
    // ONLY flag that takes `Done` and Escape away. The count is 2 and `busy` is
    // false in the first render, so nothing else could be doing it — and the
    // control that follows sets `busy` INSTEAD, at the same count, and finds both
    // ways out live. That pairing is the whole claim: a `busy`-gated Escape plus a
    // `busy` that can latch is a modal with no exit at all.
    const unwinding = renderFooter(2, false, false, true)
    const dialog = screen.getByRole('dialog', { name: 'Table Editor' })
    expect(screen.getByRole('button', { name: 'Done' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    fireEvent.keyDown(dialog, { key: 'Escape' })
    expect(unwinding.onClose).not.toHaveBeenCalled()

    unwinding.unmount()
    const committing = renderFooter(2, true)
    expect(screen.getByRole('button', { name: 'Done' })).toBeEnabled()
    fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    expect(committing.onClose).toHaveBeenCalledOnce()
    fireEvent.keyDown(screen.getByRole('dialog', { name: 'Table Editor' }), { key: 'Escape' })
    expect(committing.onClose).toHaveBeenCalledTimes(2)
  })
})

// ---------------------------------------------------------------------------
// STORY 14.8 — HEADER, CELLS AND BORDERS.
//
// The regroup, the two derived-fact statements, and the header border: its
// resolved twins, its one-attribute-per-command wire shape, and the words that
// disclose the cascade taking the block whole. The wire-shape rows of the I/O
// matrix live in `folio-go/table_header_style_test.go`, because they are claims
// about the bytes a command leaves behind; these are the rows only the panel can
// answer.
// ---------------------------------------------------------------------------
describe('the table editor\'s three headed sections', { timeout: 30_000 }, () => {
  // A dialog rendered DIRECTLY, so a test can hand it a projection the little
  // in-memory engine cannot reach — in particular a header border that RESOLVES
  // to something the committed members do not carry, which is the only shape
  // that can tell a resolved twin from an echo of the authored value.
  // `reproject` re-renders the SAME mount with a new header projection, which is
  // the only way to observe a REMOUNT: a fresh `render` would give a new DOM node
  // whether or not React re-keyed anything, so node identity across a rerender is
  // what distinguishes "the key changed" from "the test mounted twice".
  const renderPanel = (header: Partial<typeof tableHeaderProjection> = {}) => {
    const onHeaderStyle = vi.fn()
    const onMinHeight = vi.fn()
    const onRules = vi.fn()
    const panel = (over: Partial<typeof tableHeaderProjection>) => {
      const projection = { revision: 1, table: { tableId: 'e7', sizing: 'points' as const, totalWidth: defaultColumns.reduce((sum, col) => sum + col.width, 0), collection: 'transactions[]', alias: 'row', ...tableHeaderProjection, ...over, columns: projected(defaultColumns) } }
      return <TableEditor projection={projection} busy={false} fileBusy={false} discarding={false} candidates={[]} sampleAvailable={false} editCount={0} onClose={vi.fn()} onCancel={vi.fn()} onAdd={vi.fn()} onRemove={vi.fn()} onMove={vi.fn()} onUpdate={vi.fn()} onTotalWidth={vi.fn()} onBinding={vi.fn()} onConfigure={vi.fn()} onFooter={vi.fn()} onHeaderHeight={vi.fn()} onAltRowBackground={vi.fn()} onHeaderStyle={onHeaderStyle} onMinHeight={onMinHeight} onRules={onRules} onCellPadding={vi.fn()} />
    }
    const { unmount, rerender } = render(panel(header))
    return { onHeaderStyle, onMinHeight, onRules, unmount, reproject: (next: Partial<typeof tableHeaderProjection>) => rerender(panel(next)) }
  }

  it('draws HEADER, CELLS, BORDERS and RULED AREA as real headings inside the ONE existing group', () => {
    renderPanel()
    const group = screen.getByRole('group', { name: 'Table header and rows' })
    // THREE HEADINGS, in the design's order, and they are HEADINGS rather than
    // styled paragraphs: a screen-reader user must perceive the three sections a
    // sighted author sees, which is this story's own subject.
    expect(within(group).getAllByRole('heading', { level: 3 }).map((heading) => heading.textContent)).toEqual(['HEADER', 'CELLS', 'BORDERS', 'RULED AREA'])
    // ⚠ AND THE SECTION IS STILL ONE GROUP. Three groups would have taken the
    // shrunk sweep in `control-vocabulary-contract.test.tsx` from 32 group
    // instances to 34 and CLEARED its floor of 33, turning a pinned clause into
    // a guard that cannot fail. That file asserts the count; this asserts the
    // markup that produces it, so a heading promoted to a group reds here too —
    // whether by `role="group"` or by `aria-labelledby`.
    expect(within(group).queryAllByRole('group')).toEqual([])
    expect(screen.queryByRole('group', { name: 'Border edges' })).toBeNull()
    // SPEC-table-rules' own section adds no group either, for the same reason.
    expect(screen.queryByRole('group', { name: 'Ruled boundaries' })).toBeNull()
    // The old undivided heading is gone rather than kept alongside them.
    expect(screen.queryByText('HEADER AND ROWS')).toBeNull()
  })

  it('keeps every control the regroup moved, under the accessible name it already had', () => {
    renderPanel()
    const group = screen.getByRole('group', { name: 'Table header and rows' })
    // A REGROUP CHANGES NO ACCESSIBLE NAME. Every one of Story 12.3's nine
    // subjects is still here and still named the same, including the ONE that
    // moved sections — `Alternating row background` is the only data-row-scoped
    // control in the section, so it is the only block CELLS receives.
    for (const name of ['Header height in points', 'Header font family', 'Header font size (pt)', 'Header line spacing', 'Header background', 'Header text colour', 'Alternating row background']) {
      expect(within(group).getByLabelText(name), name).toBeInTheDocument()
    }
    for (const name of ['Header vertical alignment', 'Header alignment']) {
      expect(within(group).getByRole('combobox', { name }), name).toBeInTheDocument()
    }
    // AND IT LANDED UNDER THE RIGHT HEADING, which is the whole point of the
    // story: the headings and the controls are siblings in one grid, so "under
    // CELLS" means "after the CELLS heading and before the BORDERS one" in
    // document order. Compared by DOM position rather than by index, so adding a
    // control cannot silently change what this asserts.
    const cells = within(group).getByRole('heading', { level: 3, name: 'CELLS' })
    const borders = within(group).getByRole('heading', { level: 3, name: 'BORDERS' })
    const alt = within(group).getByLabelText('Alternating row background')
    expect(cells.compareDocumentPosition(alt) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(borders.compareDocumentPosition(alt) & Node.DOCUMENT_POSITION_PRECEDING).toBeTruthy()
    // The eight header-scoped controls stay ahead of CELLS.
    expect(cells.compareDocumentPosition(within(group).getByLabelText('Header font family')) & Node.DOCUMENT_POSITION_PRECEDING).toBeTruthy()
  })

  it('states the two derived facts rather than offering them as settings', () => {
    renderPanel()
    // NEITHER IS A CONTROL AT ALL — not a live one and not a disabled one. The
    // mockup drew `Row height` with a dropdown chevron and `Repeat on
    // continuation pages` as a ticked checkbox with a REQUIRED badge; the format
    // has no field behind either, and DESIGN.md's "don't draw an affordance the
    // product cannot honour" settles that against the drawing.
    expect(screen.queryByRole('combobox', { name: /Row height/ })).toBeNull()
    expect(screen.queryByRole('checkbox', { name: /Repeat on continuation/ })).toBeNull()
    expect(screen.queryByText('REQUIRED')).toBeNull()
    // THE VALUE THE ENGINE WILL USE, AND THE REASON IT IS NOT A CHOICE — the
    // shipped locked idiom, which DESIGN.md's "state the reason next to anything
    // disabled" is the rule for.
    expect(screen.getByLabelText('Row height note')).toHaveTextContent('auto')
    expect(screen.getByText(/Every row is as tall as the content it holds/)).toBeInTheDocument()
    // ⚠ AND THE REPEAT IS NOT WORDED AS AN ABSOLUTE. The engine carries
    // `DiagCodeTableHeaderRepeatSuppressed` — its own record of the page where
    // reserving the header would leave no room for a row — so a badge promising
    // a guarantee would promise what the engine can suspend. The sentence has to
    // name the exception, not merely avoid the word "always".
    expect(screen.getByText(/drops the repeat for that page only and warns/)).toBeInTheDocument()
  })

  it('shows the engine\'s resolved border, never the value the author typed', () => {
    // THE SEPARATING FIXTURE: the header declares a 3pt border and NOTHING else,
    // while the engine resolves a 1pt black bottom edge. Every committed member
    // therefore disagrees with its resolved twin, so a twin wired to the
    // committed value — the both-sides-move-together shape, which asserts
    // nothing — fails on all three.
    renderPanel({
      'headerBorder.width': '3000', 'headerBorder.widthResolved': '1000',
      'headerBorder.color': '', 'headerBorder.colorResolved': '#000000',
      'headerBorder.edges': '', 'headerBorder.edgesResolved': 'bottom',
    })
    expect(screen.getByRole('spinbutton', { name: 'Header border width (pt)' })).toHaveValue(3)
    expect(screen.getByRole('spinbutton', { name: 'Header border width (pt)' })).toHaveAttribute('placeholder', '1')
    expect(screen.getByRole('textbox', { name: 'Header border colour' })).toHaveValue('')
    expect(screen.getByRole('textbox', { name: 'Header border colour' })).toHaveAttribute('placeholder', '#000000')
    expect(screen.getByLabelText('Resolved Header border edges')).toHaveTextContent('Using: bottom')
    // The four edge boxes reflect what the DOCUMENT declares, not what resolves:
    // a checkbox showing the resolved set could never be unchecked back to absent.
    for (const edge of ['top', 'right', 'bottom', 'left']) {
      expect(screen.getByRole('checkbox', { name: `Header border ${edge} edge` }), edge).not.toBeChecked()
    }
    // ⚠ `min="0"`, NOT `min="0.5"`. Zero is the thinnest device line PDF can
    // draw and the loader accepts it; a control that refused it locally would
    // refuse a border the format defines.
    expect(screen.getByRole('spinbutton', { name: 'Header border width (pt)' })).toHaveAttribute('min', '0')
  })

  it('says nothing is painted when nothing is painted, which only the edge list can tell it', () => {
    // NO BORDER ANYWHERE — the resolved edge list is empty, and it is the ONLY
    // member the panel can read for "nothing is painted". Not because a resolved
    // width of 0 is ambiguous (it is not: the width is a string, so `''` is "no
    // border reaches this row" and `'0'` is a declared zero-width one) but
    // because of the OTHER way to paint nothing — a border that resolves while
    // its declared `edges` names no side, where the width resolves to a real
    // number and the emitter still strokes nothing.
    const nothing = renderPanel()
    // THE SAME CLAIM, READ OFF THE CONTROLS THE FACT NOW LIVES IN. Width and
    // colour carry it as a placeholder inside their own boxes (the inspector's
    // rule for an uncommitted field); the EDGE LIST cannot, because it is a set
    // of checkboxes with no empty state to label, so it keeps the note beside
    // it. Three controls, one fact, two spellings — and the spelling is decided
    // by whether the control has anywhere to put it.
    expect(screen.getByRole('spinbutton', { name: 'Header border width (pt)' })).toHaveAttribute('placeholder', 'nothing — no border painted')
    expect(screen.getByRole('textbox', { name: 'Header border colour' })).toHaveAttribute('placeholder', 'nothing — no border painted')
    expect(screen.getByLabelText('Resolved Header border edges')).toHaveTextContent('Using: nothing — no border is painted')
    // ⚠ THE FIRST DIALOG IS UNMOUNTED BEFORE THE SECOND IS RENDERED. Two live
    // `role="dialog"` trees would leave every `getBy*` below ambiguous and force
    // the assertion onto an ARRAY POSITION — a query keyed on mount order rather
    // than on the state under test, which silently reads the wrong panel the
    // moment either render moves. `renderPanel` returns `unmount` for this.
    nothing.unmount()
    // AND THE CONTRAST: a border that IS painted with a zero width says so rather
    // than reusing the nothing-painted sentence. This is the pair that makes the
    // assertions above a discrimination rather than a single reading.
    renderPanel({ 'headerBorder.widthResolved': '0', 'headerBorder.colorResolved': '#000000', 'headerBorder.edgesResolved': 'top,right,bottom,left' })
    expect(screen.getByRole('spinbutton', { name: 'Header border width (pt)' })).toHaveAttribute('placeholder', '0')
    expect(screen.getByLabelText('Resolved Header border edges')).toHaveTextContent('Using: top,right,bottom,left')
  })

  // ⚠ THE BOX ITSELF, FOR BOTH SPELLINGS OF "NO NUMBER TO SHOW". The declared-zero
  // test below this one asserts the takeover PROSE, and prose alone left the box
  // free to render anything and stay green — which is where the original defect
  // survived one layer up: `Number(table['headerBorder.width'])` folded `'0'` and
  // `''` both to 0, `styleNumber` rendered `defaultValue={committed === 0 ? '' :
  // …}`, and a LEGAL DECLARED ZERO-WIDTH BORDER showed an EMPTY box —
  // indistinguishable from unauthored. So the spinbutton's own value is asserted
  // for both cases, and it is the assertion the prose cannot make.
  it('shows a declared zero-width border as 0 and an absent one as empty', () => {
    const absent = renderPanel()
    expect(screen.getByRole('spinbutton', { name: 'Header border width (pt)' })).toHaveValue(null)
    absent.unmount()
    const zero = renderPanel({ 'headerBorder.width': '0', 'headerBorder.widthResolved': '0', 'headerBorder.colorResolved': '#000000', 'headerBorder.edgesResolved': 'top,right,bottom,left' })
    expect(screen.getByRole('spinbutton', { name: 'Header border width (pt)' })).toHaveValue(0)
    zero.unmount()
    // AND A DECLARED NON-ZERO STILL READS IN POINTS, so the branch above did not
    // buy the two absence spellings at the cost of the ordinary case.
    renderPanel({ 'headerBorder.width': '1500', 'headerBorder.widthResolved': '1500', 'headerBorder.colorResolved': '#000000', 'headerBorder.edgesResolved': 'top,right,bottom,left' })
    expect(screen.getByRole('spinbutton', { name: 'Header border width (pt)' })).toHaveValue(1.5)
  })

  // AND THE REMOUNT KEY DISTINGUISHES THE TWO, which `Number()` also destroyed:
  // `boxKey(0)` was the key for BOTH `''` and `'0'`, so CLEARING a declared zero
  // re-keyed nothing, React reused the same uncontrolled input, and the box kept
  // whatever text was in it — the half-controlled-row defect `boxKey` exists to
  // prevent, on the one pair of states it could no longer tell apart.
  //
  // Asserted as NODE IDENTITY ACROSS A REPROJECTION, which is the mechanism
  // itself: a changed key remounts and yields a new element, an unchanged key
  // reuses the old one. Two separate `render` calls could not say this — they
  // always produce different nodes.
  it('remounts the width box when a declared zero is cleared, and not on an unrelated change', () => {
    const panel = renderPanel({ 'headerBorder.width': '0', 'headerBorder.widthResolved': '0', 'headerBorder.colorResolved': '#000000', 'headerBorder.edgesResolved': 'top,right,bottom,left' })
    const declaredZero = screen.getByRole('spinbutton', { name: 'Header border width (pt)' })
    // The author types over the declared zero, and the CLEAR lands.
    fireEvent.change(declaredZero, { target: { value: '2' } })
    panel.reproject({ 'headerBorder.width': '', 'headerBorder.widthResolved': '', 'headerBorder.colorResolved': '', 'headerBorder.edgesResolved': '' })
    const cleared = screen.getByRole('spinbutton', { name: 'Header border width (pt)' })
    expect(cleared).not.toBe(declaredZero)
    // AND THE BOX SHOWS THE DOCUMENT'S ANSWER RATHER THAN THE TYPED TEXT, which
    // is the whole point of the remount.
    expect(cleared).toHaveValue(null)
    panel.unmount()
  })

  // ⚠ THE UN-AUTHORED BRANCH MAKES NO CLAIM ABOUT PROVENANCE, AND THIS TEST
  // ASSERTS THE ABSENCE OF THE CLAIM RATHER THAN THE PRESENCE OF THE TWINS.
  //
  // It used to say the header "takes the table's own border". That is a sentence
  // the panel CANNOT justify: a header declaring `{"border": {}}` — an empty
  // block the loader admits and `resolveHeaderStyle` takes WHOLE — has already
  // taken the border over while committing no sub-field, so all three committed
  // members read `''`, `borderAuthored` is false, and the panel asserted
  // inheritance about a header that had ended it. `{"border": {"edges": []}}` is
  // the same class. The resolved trio cannot rescue it either: a header that
  // genuinely inherits also resolves to a non-empty edge list.
  //
  // ASSERTING THE ABSENCE IS THE POINT. A test that only checked the resolved
  // notes were present would stay green if someone found this branch bare,
  // thought it unfinished, and reinstated the sentence — which is precisely the
  // move this narrowing has to survive. The two fixtures below are the empty
  // block and the empty edge list, i.e. the two states that made the old
  // sentence false, both driven through the real panel.
  it('makes no claim about provenance when nothing is authored', () => {
    // (a) THE EMPTY BLOCK: committed all absent, resolved at the format's own
    // defaults — the state the old sentence lied about.
    const emptyBlock = renderPanel({ 'headerBorder.widthResolved': '500', 'headerBorder.colorResolved': '#000000', 'headerBorder.edgesResolved': 'top,right,bottom,left' })
    expect(screen.queryByText(/takes the table’s own border/)).toBeNull()
    expect(screen.queryByText(/table’s border stops reaching/)).toBeNull()
    expect(screen.queryByText(/no longer follows the table’s border/)).toBeNull()
    // What it DOES say: nothing is authored, and the notes are the engine's.
    expect(screen.getByText(/No header border attribute is authored here/)).toBeInTheDocument()
    expect(screen.getByRole('spinbutton', { name: 'Header border width (pt)' })).toHaveAttribute('placeholder', '0.5')
    emptyBlock.unmount()
    // (b) THE EMPTY EDGE LIST: `hasBorder` is true and nothing is painted, and
    // the branch still claims no provenance.
    renderPanel({ 'headerBorder.widthResolved': '', 'headerBorder.colorResolved': '', 'headerBorder.edgesResolved': '' })
    expect(screen.queryByText(/takes the table’s own border/)).toBeNull()
    expect(screen.getByText(/No header border attribute is authored here/)).toBeInTheDocument()
  })

  // ⚠ THE TAKEOVER SENTENCE IS DRIVEN BY A THREE-WAY `||`, AND IT GETS THREE
  // CASES BECAUSE A DISJUNCTION TESTED THROUGH ONE DISJUNCT IS TWO GUARDS THAT
  // ARE NEVER INVOKED. It used to have exactly one case, through the COLOUR, and
  // that is why nothing caught the width defect: `headerBorder.width` was a
  // number spelling absence as 0, `0` is also a LEGAL DECLARED WIDTH (the
  // thinnest device line PDF can draw — `parse_bands.go` says so in those
  // words), so a header border authored as nothing but `{"width": 0}` failed
  // every disjunct and the panel told the author "nothing here is set, so this
  // header row takes the table's own border" about a header that had taken the
  // border over. The projection now spells the width as a string in the same
  // thousandths, `''` absent and `'0'` declared, which is what makes the first
  // case below expressible at all.
  //
  // EACH CASE AUTHORS EXACTLY ONE ATTRIBUTE AND LEAVES THE OTHER TWO ABSENT, so
  // deleting ONE disjunct from `borderAuthored` reds exactly ONE of them. Three
  // cases that all pass because some other disjunct is true would be the same
  // defect in a different costume, and a green suite would not say so. Measured
  // by actually performing the three deletions.
  //
  // ONE attribute authored is enough in every case: the cascade takes the block
  // WHOLE, so there is no half-way state to describe and the words must not wait
  // for the third attribute before saying so.
  it('states the takeover when the WIDTH alone is authored, including a declared ZERO', () => {
    // `'0'` BY NAME, because it is the value that produced the finding: a legal,
    // painted, declared border that a numeric member reported as absent. The
    // resolved trio is what the engine answers for it — a zero-width stroke on
    // all four edges in the format's own black.
    renderPanel({ 'headerBorder.width': '0', 'headerBorder.widthResolved': '0', 'headerBorder.colorResolved': '#000000', 'headerBorder.edgesResolved': 'top,right,bottom,left' })
    expect(screen.getByText(/no longer follows the table’s border/)).toBeInTheDocument()
    expect(screen.queryByText(/takes the table’s own border/)).toBeNull()
    expect(screen.getByText(/falls to the format’s own default rather than to the table’s value/)).toBeInTheDocument()
  })

  it('states the takeover when the COLOUR alone is authored', () => {
    renderPanel({ 'headerBorder.color': '#c81e1e', 'headerBorder.colorResolved': '#c81e1e', 'headerBorder.widthResolved': '500', 'headerBorder.edgesResolved': 'top,right,bottom,left' })
    expect(screen.getByText(/no longer follows the table’s border/)).toBeInTheDocument()
    expect(screen.queryByText(/takes the table’s own border/)).toBeNull()
    expect(screen.getByText(/falls to the format’s own default rather than to the table’s value/)).toBeInTheDocument()
  })

  it('states the takeover when the EDGE LIST alone is authored', () => {
    renderPanel({ 'headerBorder.edges': 'bottom', 'headerBorder.edgesResolved': 'bottom', 'headerBorder.widthResolved': '500', 'headerBorder.colorResolved': '#000000' })
    expect(screen.getByText(/no longer follows the table’s border/)).toBeInTheDocument()
    expect(screen.queryByText(/takes the table’s own border/)).toBeNull()
    expect(screen.getByText(/falls to the format’s own default rather than to the table’s value/)).toBeInTheDocument()
  })

  it('sends exactly the attribute the author touched, and nothing beside it', async () => {
    const harness = tableEngine()
    await openEditor(harness)
    const width = screen.getByRole('spinbutton', { name: 'Header border width (pt)' })
    fireEvent.change(width, { target: { value: '1.5' } })
    fireEvent.blur(width)
    await waitFor(() => expect(harness.commands).toHaveLength(1))
    // ONE FIELD, ONE OP, ONE VALUE. A colour or an edge list riding along here
    // would be a value the author never chose — and it is the reason the wire
    // carries three flat dotted attributes rather than one `border` object: a
    // block `set` could only be built by reading the other two back from the
    // projection and re-transmitting them.
    expect(harness.commands[0]).toBe('{"kind":"updateTableHeaderStyle","version":1,"id":"e7","field":"border.width","op":"set","value":1.5}')
    // The colour box beside it is a separate command, and still carries only its
    // own attribute.
    const colour = screen.getByRole('textbox', { name: 'Header border colour' })
    fireEvent.change(colour, { target: { value: '#c81e1e' } })
    fireEvent.blur(colour)
    await waitFor(() => expect(harness.commands).toHaveLength(2))
    expect(harness.commands[1]).toBe('{"kind":"updateTableHeaderStyle","version":1,"id":"e7","field":"border.color","op":"set","value":"#c81e1e"}')
  })

  it('authors the edge set from four bare checkboxes, in the format\'s own order, and clears it by emptying it', async () => {
    const harness = tableEngine()
    await openEditor(harness)
    // TICKING `bottom` FIRST AND `top` SECOND, so the command's order cannot be
    // the click order: the engine's projection joins the set canonically and the
    // panel sends it the same way, because the browser's own guard admits only
    // the format's order.
    fireEvent.click(screen.getByRole('checkbox', { name: 'Header border bottom edge' }))
    await waitFor(() => expect(harness.commands).toHaveLength(1))
    expect(harness.commands[0]).toBe('{"kind":"updateTableHeaderStyle","version":1,"id":"e7","field":"border.edges","op":"set","value":["bottom"]}')
    fireEvent.click(screen.getByRole('checkbox', { name: 'Header border top edge' }))
    await waitFor(() => expect(harness.commands).toHaveLength(2))
    expect(harness.commands[1]).toBe('{"kind":"updateTableHeaderStyle","version":1,"id":"e7","field":"border.edges","op":"set","value":["top","bottom"]}')
    // AND EMPTYING THE SET IS THE CLEAR. There is no `×` here and there must not
    // be one: a glyph button outside a segmented control moves `V2_CENSUS`, and
    // the engine refuses an empty edge array, so an emptied set can only mean
    // "remove the attribute".
    expect(screen.queryByRole('button', { name: 'Clear Header border edges' })).toBeNull()
    fireEvent.click(screen.getByRole('checkbox', { name: 'Header border top edge' }))
    await waitFor(() => expect(harness.commands).toHaveLength(3))
    expect(harness.commands[2]).toBe('{"kind":"updateTableHeaderStyle","version":1,"id":"e7","field":"border.edges","op":"set","value":["bottom"]}')
    fireEvent.click(screen.getByRole('checkbox', { name: 'Header border bottom edge' }))
    await waitFor(() => expect(harness.commands).toHaveLength(4))
    expect(harness.commands[3]).toBe('{"kind":"updateTableHeaderStyle","version":1,"id":"e7","field":"border.edges","op":"clear"}')
    // AND THE READ-BACK: the boxes come back unchecked because the DOCUMENT no
    // longer declares an edge list, which is what makes the clear a clear rather
    // than a set of nothing.
    await waitFor(() => expect(screen.getByRole('checkbox', { name: 'Header border bottom edge' })).not.toBeChecked())
  })

  it('does not count a border clear that removes what was never there', async () => {
    // MATRIX ROW: "clearing what is already absent" is a BYTE-LEVEL no-op, and
    // the claim that has to be made is about the EDIT COUNT rather than about the
    // absence of an error — `commitTableColumn` only counts when the engine's
    // revision actually moved, and a test that merely watched for an alert would
    // pass against a count keyed on dispatches. The count is observed the only
    // way this dialog exposes it: `Cancel` sends exactly that many undos.
    const harness = tableEngine()
    await openEditor(harness)
    const atOpen = harness.canonical()
    await retitle(1, 'Total')
    // THREE LEGAL COMMANDS THAT REMOVE NOTHING. All three reach the engine.
    fireEvent.click(screen.getByRole('button', { name: 'Clear Header border width (pt)' }))
    await idle()
    fireEvent.click(screen.getByRole('button', { name: 'Clear Header border colour' }))
    await idle()
    fireEvent.click(screen.getByRole('button', { name: 'Clear Header border width (pt)' }))
    await idle()
    expect(harness.commands).toHaveLength(4)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    // EXACTLY ONE, not four: the retitle moved the document and the three clears
    // did not.
    expect(operations(harness, 'undo')).toHaveLength(1)
    expect(harness.canonical()).toBe(atOpen)
  })

  // THE NEGATIVE CONTROL FOR THIS FILE'S OWN ENGINE MOCK, and it protects the
  // NEXT header-style field to be added rather than these three.
  //
  // ⚠ TWO POPULATIONS LIVE HERE AND THEY HAVE DIFFERENT SIZES, so neither is
  // described with the other's number. Go's `tableHeaderStyleFields` closed set
  // is TWELVE (it carries `bold` and `italic`, which the panel does not author —
  // DW-369, out of fence); the PANEL's authoring union, which is what
  // `HEADER_STYLE_KEYS` models and what this mock has to answer for, is TEN.
  //
  // The mock's header arm used to read `if (key !== undefined)` and silently do
  // nothing for a field absent from `HEADER_STYLE_KEYS`. That made an unmodelled
  // field a NO-OP: a test could drive the panel, watch the command go out, watch
  // the projection not move, and pass — proving nothing at all. It was not a live
  // vacuity while the map matched the panel's union exactly; it became one the
  // moment the union grew, which is this story. So the map is no longer allowed
  // to be quietly incomplete.
  it('refuses a header-style command naming a field the engine mock does not model', () => {
    expect(() => headerStyleKeyFor('border.dash')).toThrow(/does not model the header-style field "border.dash"/)
    // And the TEN of the panel's union that it DOES model all answer, so the
    // guard above is not simply refusing everything.
    for (const field of ['fontFamily', 'fontSize', 'lineSpacing', 'background', 'color', 'valign', 'align', 'border.width', 'border.color', 'border.edges']) {
      expect(headerStyleKeyFor(field), field).toMatch(/^header/)
    }
  })
})


describe('table column field authoring', () => {
  it.each([undefined, '{"transactions":[{"date":"today","customer":{"name":"Ada"}}],"other":[{"wrong":"field"}]}'])('commits typed relative paths through the fenced editor path with sample %s', async (sample) => {
    const harness = tableEngine()
    await openEditor(harness, sample)
    fireEvent.blur(screen.getByRole('textbox', { name: 'Row alias' }), { target: { value: 'txn' } })
    await idle()
    const field = screen.getByRole('combobox', { name: 'Binding for column 1' })
    expect(field).toHaveValue('{{txn.amount}}')
    const options = Array.from(document.querySelectorAll('#table-row-field-candidates option')).map((option) => option.getAttribute('value'))
    expect(options).toEqual(sample === undefined ? [] : ['{{txn.customer.name}}', '{{txn.date}}'])
    fireEvent.blur(field, { target: { value: '{{txn.customer.name}}' } })
    await idle()
    expect(harness.commands.at(-1)).toBe('{"kind":"updateTableColumnExpression","version":1,"id":"e7","columnId":"c1","binding":"{{txn.customer.name}}"}')
    expect(screen.getByRole('combobox', { name: 'Binding for column 1' })).toHaveValue('{{txn.customer.name}}')
    expect(screen.getByRole('combobox', { name: 'Binding for column 1' })).toHaveValue('{{txn.customer.name}}')
    expect(harness.state.revision).toBe(3)
  })

  it('re-scopes suggestions to the current root collection after it changes', async () => {
    await openEditor(tableEngine(), '{"transactions":[{"date":"today"}],"other":[{"name":"Ada"}]}')
    const values = () => Array.from(document.querySelectorAll('#table-row-field-candidates option')).map((option) => option.getAttribute('value'))
    expect(values()).toEqual(['{{row.date}}'])
    fireEvent.blur(screen.getByRole('combobox', { name: 'Root collection' }), { target: { value: 'other[]' } })
    await idle()
    expect(values()).toEqual(['{{row.name}}'])
  })

  it('clears once, skips untouched empty fields, and restores the binding with one undo', async () => {
    const harness = tableEngine()
    await openEditor(harness)
    fireEvent.blur(screen.getByRole('combobox', { name: 'Binding for column 1' }), { target: { value: '' } })
    await idle()
    expect(harness.commands).toEqual(['{"kind":"updateTableColumnExpression","version":1,"id":"e7","columnId":"c1","binding":""}'])
    expect(screen.getByRole('combobox', { name: 'Binding for column 1' })).toHaveValue('')
    fireEvent.blur(screen.getByRole('combobox', { name: 'Binding for column 1' }))
    expect(harness.commands).toHaveLength(1)
    fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    fireEvent.click(screen.getByRole('button', { name: 'Undo' }))
    await waitFor(() => expect(harness.state.columns[0]!.rowField).toBe('amount'))
    fireEvent.click(screen.getByRole('button', { name: 'table component e7' }))
    fireEvent.click(screen.getByRole('button', { name: 'Configure columns' }))
    await screen.findByRole('dialog', { name: 'Table Editor' })
    expect(screen.getByRole('combobox', { name: 'Binding for column 1' })).toHaveValue('{{row.amount}}')
    expect(screen.getByRole('combobox', { name: 'Binding for column 1' })).toHaveValue('{{row.amount}}')
  })

  it('restores the committed field after each refusal and counts no discarded edit', async () => {
    const harness = tableEngine(defaultColumns, {}, { refuseCommand: (command) => command.kind === 'updateTableColumnExpression' })
    const before = harness.canonical()
    await openEditor(harness)
    for (let attempt = 0; attempt < 2; attempt++) {
      fireEvent.blur(screen.getByRole('combobox', { name: 'Binding for column 1' }), { target: { value: '{{row.customer..name}}' } })
      await screen.findByRole('alert')
      await idle()
      expect(screen.getByRole('combobox', { name: 'Binding for column 1' })).toHaveValue('{{row.amount}}')
      expect(screen.getByRole('combobox', { name: 'Binding for column 1' })).toHaveValue('{{row.amount}}')
    }
    expect(harness.commands).toHaveLength(2)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    expect(harness.request.mock.calls.filter(([operation]) => operation === 'undo')).toHaveLength(0)
    expect(harness.canonical()).toBe(before)
  })

  it('edits complete formulas even when the relative-field projection is not editable', async () => {
    const expression = '{{formatNumber(row.amount, "#,##0.00")}}'
    const replacement = ' Total: {{formatNumber(row.amount * 1.07, "#,##0.00")}} '
    const harness = tableEngine([{ ...defaultColumns[0]!, rowField: '', binding: expression, rowFieldEditable: false }, ...defaultColumns.slice(1)])
    await openEditor(harness)
    const field = screen.getByRole('combobox', { name: 'Binding for column 1' })
    expect(field).toBeEnabled()
    expect(field).toHaveValue(expression)
    const header = screen.getByRole('textbox', { name: 'Header for column 1' })
    header.focus(); press('ArrowRight')
    expect(document.activeElement).toBe(field)
    fireEvent.blur(field, { target: { value: replacement } })
    await idle()
    expect(harness.state.columns[0]!.binding).toBe(replacement)
    expect(screen.getByRole('combobox', { name: 'Binding for column 1' })).toHaveValue(replacement)
    fireEvent.blur(screen.getByRole('combobox', { name: 'Binding for column 1' }))
    expect(harness.commands).toHaveLength(1)
  })

  it.each(['\n', '\r\n', '\r'])('does not commit untouched multiline binding with %j line endings on blur or Done', async (newline) => {
    const binding = `Code:${newline}{{upper(row.trn_code)}}`
    const harness = tableEngine([{ ...defaultColumns[0]!, binding, rowFieldEditable: false }])
    await openEditor(harness)
    const field = screen.getByRole('textbox', { name: 'Binding for column 1' })
    expect(field.tagName).toBe('TEXTAREA')
    expect(field).toHaveValue('Code:\n{{upper(row.trn_code)}}')
    fireEvent.blur(field)
    fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    expect(harness.commands).toEqual([])
    expect(harness.state.columns[0]!.binding).toBe(binding)
  })

  it.each(['Add column', 'Done', 'Escape', 'Cancel'])('handles pending multiline edits through %s without losing authored newlines', async (action) => {
    const binding = 'Code:\r\n{{row.trn_code}}'
    const draft = 'Updated:\n{{upper(row.trn_code)}}'
    const harness = tableEngine([{ ...defaultColumns[0]!, binding, rowFieldEditable: false }])
    await openEditor(harness)
    const field = screen.getByRole('textbox', { name: 'Binding for column 1' })
    field.focus()
    fireEvent.change(field, { target: { value: draft } })
    if (action === 'Escape') fireEvent.keyDown(field, { key: 'Escape' })
    else {
      const button = screen.getByRole('button', { name: action })
      expect(fireEvent.mouseDown(button, { button: 0 })).toBe(false)
      fireEvent.click(button)
    }
    if (action === 'Add column') await idle()
    else await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    expect(harness.state.columns[0]!.binding).toBe(action === 'Cancel' ? binding : draft)
    expect(harness.commands.map((command) => JSON.parse(command).kind)).toEqual(action === 'Cancel' ? [] : action === 'Add column' ? ['updateTableColumnExpression', 'addTableColumn'] : ['updateTableColumnExpression'])
  })

  it('restores a multiline binding after rejection and permits a subsequent exact edit', async () => {
    const binding = 'Code:\r\n{{row.trn_code}}'
    let refuse = true
    const harness = tableEngine([{ ...defaultColumns[0]!, binding, rowFieldEditable: false }], {}, { refuseCommand: () => refuse })
    await openEditor(harness)
    fireEvent.blur(screen.getByRole('textbox', { name: 'Binding for column 1' }), { target: { value: '{{upper(}}\n' } })
    await screen.findByRole('alert')
    await idle()
    expect(screen.getByRole('textbox', { name: 'Binding for column 1' })).toHaveValue('Code:\n{{row.trn_code}}')
    expect(harness.state.columns[0]!.binding).toBe(binding)
    refuse = false
    const draft = 'Code:\n{{upper(row.trn_code)}}'
    fireEvent.blur(screen.getByRole('textbox', { name: 'Binding for column 1' }), { target: { value: draft } })
    await idle()
    expect(harness.state.columns[0]!.binding).toBe(draft)
  })

  it('keeps ordinary matrix navigation and provides the native suggestion route on Alt+Down', async () => {
    const harness = tableEngine()
    await openEditor(harness, '{"transactions":[{"date":"today"}]}')
    const field = screen.getByRole('combobox', { name: 'Binding for column 1' })
    const showPicker = vi.fn()
    Object.defineProperty(field, 'showPicker', { configurable: true, value: showPicker })
    field.focus()
    fireEvent.keyDown(field, { key: 'ArrowDown', altKey: true })
    expect(showPicker).toHaveBeenCalledOnce()
    expect(document.activeElement).toBe(field)
    expect(harness.commands).toEqual([])
    press('ArrowDown')
    expect(document.activeElement).toBe(screen.getByRole('combobox', { name: 'Binding for column 2' }))
  })

  it('refreshes a previously edited width when Add splits it, and Cancel restores this session', async () => {
    const harness = tableEngine([{ ...defaultColumns[0]!, width: 400000 }])
    const before = harness.canonical()
    await openEditor(harness)
    fireEvent.blur(screen.getByRole('spinbutton', { name: 'Width for column 1 in points' }), { target: { value: '500' } })
    await idle()
    expect(screen.getByRole('spinbutton', { name: 'Width for column 1 in points' })).toHaveValue(500)
    fireEvent.click(screen.getByRole('button', { name: 'Add column' }))
    await idle()
    expect(screen.getByRole('spinbutton', { name: 'Width for column 1 in points' })).toHaveValue(250)
    expect(screen.getByRole('spinbutton', { name: 'Width for column 2 in points' })).toHaveValue(250)
    expect(screen.getByRole('status', { name: 'Width budget' })).toHaveTextContent('Σ 500.0 of 500.0 available')
    expect(screen.getByRole('combobox', { name: 'Binding for column 1' })).toHaveValue('{{row.amount}}')
    fireEvent.blur(screen.getByRole('combobox', { name: 'Binding for column 2' }), { target: { value: '{{row.date}}' } })
    await idle()
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    expect(harness.canonical()).toBe(before)
    expect(harness.request.mock.calls.filter(([operation]) => operation === 'undo')).toHaveLength(3)
  })

  it('retains the next nonmatrix Tab stop when a commit or refusal reprojects the editor', () => {
    const projection = { revision: 1, table: { tableId: 'e7', sizing: 'points' as const, totalWidth: defaultColumns.reduce((sum, col) => sum + col.width, 0), collection: 'transactions[]', alias: 'row', ...tableHeaderProjection, columns: projected(defaultColumns) } }
    const props = { projection, busy: false, fileBusy: false, discarding: false, candidates: [], sampleAvailable: false, editCount: 0, onClose: vi.fn(), onCancel: vi.fn(), onAdd: vi.fn(), onRemove: vi.fn(), onMove: vi.fn(), onUpdate: vi.fn(), onTotalWidth: vi.fn(), onBinding: vi.fn(), onConfigure: vi.fn(), onFooter: vi.fn(), onHeaderHeight: vi.fn(), onAltRowBackground: vi.fn(), onHeaderStyle: vi.fn(), onMinHeight: vi.fn(), onRules: vi.fn(), onCellPadding: vi.fn() }
    const { rerender } = render(<TableEditor {...props} />)
    const next = screen.getByRole('spinbutton', { name: 'Header font size (pt)' })
    next.focus()
    const committed = { ...projection, revision: 2, table: { ...projection.table, headerFontFamily: 'body' } }
    rerender(<TableEditor {...props} projection={committed} />)
    expect(document.activeElement).toBe(next)
    expect(screen.getByRole('textbox', { name: 'Header font family' })).toHaveValue('body')
    fireEvent.change(screen.getByRole('textbox', { name: 'Header font family' }), { target: { value: 'refused' } })
    rerender(<TableEditor {...props} projection={committed} error="Unknown font family" />)
    expect(document.activeElement).toBe(next)
    expect(screen.getByRole('textbox', { name: 'Header font family' })).toHaveValue('body')
  })
})


describe('table editor actions with a pending formula', () => {
  const oneColumn = [{ ...defaultColumns[0]!, width: 500000 }]
  const typeField = (value: string) => {
    const input = screen.getByRole('combobox', { name: 'Binding for column 1' })
    input.focus()
    fireEvent.change(input, { target: { value } })
    return input
  }
  const clickAction = (name: string) => {
    const button = screen.getByRole('button', { name })
    expect(fireEvent.mouseDown(button, { button: 0 })).toBe(false)
    fireEvent.click(button)
  }

  it('commits a dirty field before Add and preserves both commands in order', async () => {
    const harness = tableEngine(oneColumn)
    await openEditor(harness)
    typeField('{{upper(row.trn_code)}}')
    clickAction('Add column')
    await waitFor(() => expect(harness.state.columns).toHaveLength(2))
    await idle()
    expect(harness.commands.map((command) => JSON.parse(command).kind)).toEqual(['updateTableColumnExpression', 'addTableColumn'])
    expect(screen.getByRole('combobox', { name: 'Binding for column 1' })).toHaveValue('{{upper(row.trn_code)}}')
    expect(screen.getByRole('spinbutton', { name: 'Width for column 1 in points' })).toHaveValue(250)
    expect(screen.getByRole('spinbutton', { name: 'Width for column 2 in points' })).toHaveValue(250)
    expect(screen.getByRole('textbox', { name: 'Header for column 2' })).toHaveValue('Column 2')
  })

  it('waits for a delayed binding before dispatching the requested Add', async () => {
    const harness = tableEngine(oneColumn, {}, { pauseCommandAt: 1 })
    await openEditor(harness)
    typeField('{{upper(row.trn_code)}}')
    clickAction('Add column')
    expect(screen.getByRole('button', { name: 'Add column' })).toBeDisabled()
    expect(harness.commands).toHaveLength(1)
    expect(harness.state.columns).toHaveLength(1)
    await act(async () => { harness.releaseCommand() })
    await waitFor(() => expect(harness.state.columns).toHaveLength(2))
    expect(harness.commands.map((command) => JSON.parse(command).kind)).toEqual(['updateTableColumnExpression', 'addTableColumn'])
  })

  it('refuses Add when its pending binding is rejected, retaining committed values and history', async () => {
    const harness = tableEngine(oneColumn, {}, { refuseCommand: (command) => command.kind === 'updateTableColumnExpression' })
    const before = harness.canonical()
    await openEditor(harness)
    typeField('{{row.bad..field}}')
    clickAction('Add column')
    await screen.findByRole('alert')
    await idle()
    expect(harness.commands).toHaveLength(1)
    expect(harness.canonical()).toBe(before)
    expect(screen.getByRole('combobox', { name: 'Binding for column 1' })).toHaveValue('{{row.amount}}')
    expect(screen.queryByRole('textbox', { name: 'Header for column 2' })).toBeNull()
  })

  it.each(['Done', 'Escape'])('commits the pending field before closing with %s', async (action) => {
    const harness = tableEngine(oneColumn)
    await openEditor(harness)
    const input = typeField('{{upper(row.trn_code)}}')
    if (action === 'Escape') fireEvent.keyDown(input, { key: 'Escape' })
    else clickAction(action)
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    expect(harness.commands.map((command) => JSON.parse(command).kind)).toEqual(['updateTableColumnExpression'])
    expect(harness.state.columns[0]!.binding).toBe('{{upper(row.trn_code)}}')
  })

  it.each(['Done', 'Escape'])('keeps the editor open when %s refuses a pending field', async (action) => {
    const harness = tableEngine(oneColumn, {}, { refuseCommand: (command) => command.kind === 'updateTableColumnExpression' })
    await openEditor(harness)
    const input = typeField('{{row.bad..field}}')
    if (action === 'Escape') fireEvent.keyDown(input, { key: 'Escape' })
    else clickAction(action)
    await screen.findByRole('alert')
    await idle()
    expect(screen.getByRole('dialog', { name: 'Table Editor' })).toBeVisible()
    expect(harness.state.columns[0]!.rowField).toBe('amount')
    expect(screen.getByRole('combobox', { name: 'Binding for column 1' })).toHaveValue('{{row.amount}}')
  })

  it.each(['Done', 'Escape'])('never follows an in-flight binding with Add after %s revokes the session', async (action) => {
    const harness = tableEngine(oneColumn, {}, { pauseCommandAt: 1 })
    await openEditor(harness)
    typeField('{{upper(row.trn_code)}}')
    clickAction('Add column')
    expect(harness.commands).toHaveLength(1)
    if (action === 'Escape') fireEvent.keyDown(screen.getByRole('dialog', { name: 'Table Editor' }), { key: 'Escape' })
    else fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    await act(async () => { harness.releaseCommand() })
    await waitFor(() => expect(harness.state.columns[0]!.binding).toBe('{{upper(row.trn_code)}}'))
    expect(harness.commands).toHaveLength(1)
    expect(harness.state.columns).toHaveLength(1)
    fireEvent.click(screen.getByRole('button', { name: 'Configure columns' }))
    await screen.findByRole('dialog', { name: 'Table Editor' })
    expect(screen.getByRole('combobox', { name: 'Binding for column 1' })).toHaveValue('{{upper(row.trn_code)}}')
    expect(screen.queryByRole('textbox', { name: 'Header for column 2' })).toBeNull()
  })

  it('Cancel discards the dirty field without sending it and undoes only the committed edit from this session', async () => {
    const harness = tableEngine(oneColumn)
    const before = harness.canonical()
    await openEditor(harness)
    await retitle(1, 'Edited')
    typeField('{{upper(row.trn_code)}}')
    clickAction('Cancel')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    expect(harness.commands.map((command) => JSON.parse(command).kind)).toEqual(['updateTableColumn'])
    expect(harness.request.mock.calls.filter(([operation]) => operation === 'undo')).toHaveLength(1)
    expect(harness.canonical()).toBe(before)
  })

  it('an invalid width restores only its input and leaves the actual next focus target connected', async () => {
    const harness = tableEngine(oneColumn, {}, { refuseCommand: (command) => command.kind === 'updateTableColumn' && command.field === 'width' && command.value === '' })
    await openEditor(harness)
    const width = screen.getByRole('spinbutton', { name: 'Width for column 1 in points' })
    width.focus()
    fireEvent.change(width, { target: { value: '' } })
    const next = screen.getByRole('textbox', { name: 'Header font family' })
    act(() => { next.focus() })
    expect(document.activeElement).toBe(next)
    expect(next.isConnected).toBe(true)
    expect(width.isConnected).toBe(true)
    await waitFor(() => expect(width).toHaveValue(500))
    expect(harness.commands.map((command) => JSON.parse(command).value)).toEqual([''])
    expect(screen.getByRole('alert')).toHaveTextContent(REFUSED_COMMAND)
  })

  it('leaves normal caret and selection keys to the formula input and uses Alt arrows for matrix navigation', async () => {
    await openEditor(tableEngine(oneColumn))
    const field = typeField('{{row.amount}}')
    for (const modifier of ['', 'shiftKey', 'ctrlKey', 'metaKey']) {
      for (const key of ['ArrowLeft', 'ArrowRight', 'Home', 'End']) {
        expect(fireEvent.keyDown(field, { key, [modifier]: true })).toBe(true)
        expect(document.activeElement).toBe(field)
      }
    }
    fireEvent.keyDown(field, { key: 'ArrowRight', altKey: true })
    expect(document.activeElement).toBe(screen.getByRole('spinbutton', { name: 'Width for column 1 in points' }))
    field.focus()
    fireEvent.keyDown(field, { key: 'ArrowLeft', altKey: true })
    expect(document.activeElement).toBe(screen.getByRole('textbox', { name: 'Header for column 1' }))
  })
})

describe('creating multiline binding drafts', () => {
  const oneColumn = [{ ...defaultColumns[0]!, binding: '', rowField: '', width: 500000 }]
  const paste = (input: HTMLInputElement, text: string) => fireEvent.paste(input, { clipboardData: { getData: () => text } })
  const multiline = () => screen.getByRole('textbox', { name: 'Binding for column 1' }) as HTMLTextAreaElement

  it.each(['blur', 'Add column', 'Done', 'Escape', 'Cancel'])('preserves multiline paste into an empty binding through %s', async (action) => {
    const harness = tableEngine(oneColumn)
    const before = harness.canonical()
    await openEditor(harness)
    const input = screen.getByRole('combobox', { name: 'Binding for column 1' }) as HTMLInputElement
    input.focus()
    expect(paste(input, 'Code:\r\n{{upper(row.trn_code)}}')).toBe(false)
    const draft = 'Code:\n{{upper(row.trn_code)}}'
    expect(multiline()).toHaveValue(draft)
    expect(document.activeElement).toBe(multiline())
    expect(multiline().selectionStart).toBe(draft.length)
    expect(harness.commands).toEqual([])
    if (action === 'blur') fireEvent.blur(multiline())
    else if (action === 'Escape') fireEvent.keyDown(multiline(), { key: 'Escape' })
    else {
      const button = screen.getByRole('button', { name: action })
      expect(fireEvent.mouseDown(button, { button: 0 })).toBe(false)
      fireEvent.click(button)
    }
    if (action === 'blur' || action === 'Add column') await idle()
    else await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    expect(harness.state.columns[0]!.binding).toBe(action === 'Cancel' ? '' : draft)
    expect(harness.commands.map((command) => JSON.parse(command).kind)).toEqual(action === 'Cancel' ? [] : action === 'Add column' ? ['updateTableColumnExpression', 'addTableColumn'] : ['updateTableColumnExpression'])
    if (action === 'Cancel') expect(harness.canonical()).toBe(before)
  })

  it('preserves surrounding text and replacement selection when paste promotes a single-line literal', async () => {
    await openEditor(tableEngine([{ ...oneColumn[0]!, binding: 'prefix OLD suffix' }]))
    const input = screen.getByRole('combobox', { name: 'Binding for column 1' }) as HTMLInputElement
    input.focus()
    input.setSelectionRange(7, 10, 'backward')
    paste(input, 'first\rsecond')
    expect(multiline()).toHaveValue('prefix first\nsecond suffix')
    expect(document.activeElement).toBe(multiline())
    expect(multiline().selectionStart).toBe('prefix first\nsecond'.length)
    expect(multiline().selectionEnd).toBe(multiline().selectionStart)
  })

  it('promotes Shift+Enter without a command and retains native multiline caret and selection keys', async () => {
    const harness = tableEngine([{ ...oneColumn[0]!, binding: 'Code: OLD {{row.trn_code}}' }, defaultColumns[1]!])
    await openEditor(harness)
    const input = screen.getByRole('combobox', { name: 'Binding for column 1' }) as HTMLInputElement
    input.focus()
    input.setSelectionRange(6, 10)
    expect(fireEvent.keyDown(input, { key: 'Enter', shiftKey: true })).toBe(false)
    expect(multiline()).toHaveValue('Code: \n{{row.trn_code}}')
    expect(multiline().selectionStart).toBe(7)
    expect(harness.commands).toEqual([])
    for (const key of ['ArrowUp', 'ArrowDown']) {
      for (const shiftKey of [false, true]) {
        expect(fireEvent.keyDown(multiline(), { key, shiftKey })).toBe(true)
        expect(document.activeElement).toBe(multiline())
      }
    }
    fireEvent.blur(multiline())
    await idle()
    expect(harness.state.columns[0]!.binding).toBe('Code: \n{{row.trn_code}}')
  })

  it('restores the original binding after a promoted draft is refused and does not recommit the reset', async () => {
    const harness = tableEngine(defaultColumns, {}, { refuseCommand: (command) => command.kind === 'updateTableColumnExpression' })
    const before = harness.canonical()
    await openEditor(harness)
    const input = screen.getByRole('combobox', { name: 'Binding for column 1' }) as HTMLInputElement
    input.focus(); input.select()
    paste(input, '{{upper(}}\n')
    fireEvent.blur(multiline())
    await screen.findByRole('alert')
    await idle()
    expect(multiline()).toHaveValue('{{row.amount}}')
    expect(harness.canonical()).toBe(before)
    fireEvent.blur(multiline())
    expect(harness.commands).toHaveLength(1)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    expect(harness.request.mock.calls.filter(([operation]) => operation === 'undo')).toHaveLength(0)
  })

  it('does not commit when a promoted draft returns to the original full binding', async () => {
    const harness = tableEngine()
    const before = harness.canonical()
    await openEditor(harness)
    const input = screen.getByRole('combobox', { name: 'Binding for column 1' }) as HTMLInputElement
    input.focus(); input.select()
    paste(input, 'Changed:\n{{row.amount}}')
    fireEvent.change(multiline(), { target: { value: '{{row.amount}}' } })
    fireEvent.blur(multiline())
    fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull())
    expect(harness.commands).toEqual([])
    expect(harness.canonical()).toBe(before)
  })

  it('keeps projected row metadata and alias migration consistent after a complete simple binding edit', async () => {
    const harness = tableEngine()
    await openEditor(harness)
    fireEvent.blur(screen.getByRole('combobox', { name: 'Binding for column 1' }), { target: { value: '{{row.customer.name}}' } })
    await idle()
    expect(harness.state.columns[0]).toMatchObject({ binding: '{{row.customer.name}}', rowField: 'customer.name', rowFieldEditable: true })
    fireEvent.blur(screen.getByRole('textbox', { name: 'Row alias' }), { target: { value: 'txn' } })
    await idle()
    expect(screen.getByRole('combobox', { name: 'Binding for column 1' })).toHaveValue('{{txn.customer.name}}')
    // The binding syntax lives in the BINDING (i) explanation now, and it
    // follows the migrated alias exactly as the help line used to.
    fireEvent.click(screen.getByRole('button', { name: 'About bindings' }))
    const note = screen.getByRole('note')
    expect(note).toHaveTextContent('{{txn.date}}')
    expect(note).toHaveTextContent('{{upper(txn.trn_code)}}')
    expect(note).toHaveTextContent('In single-line bindings, Alt+Down')
  })
})

// ---------------------------------------------------------------------------
// spec-table-cell-padding-header-align-info — HEADER ALIGN, CELL PADDING and the
// two (i) explanations, one test per I/O row the panel owns.
// ---------------------------------------------------------------------------
describe('header alignment, cell padding and the (i) explanations', () => {
  const setup = (patch: Partial<typeof tableHeaderProjection> = {}, columns: ReadonlyArray<ColumnFixture> = defaultColumns, sizing: 'points' | 'proportion' = 'points') => {
    const projection = { revision: 1, table: { tableId: 'e7', sizing, totalWidth: 174000, collection: 'transactions[]', alias: 'row', ...tableHeaderProjection, ...patch, columns: projected(columns).map((column) => sizing === 'proportion' ? { ...column, proportion: '1' } : column) } }
    const props = { projection, busy: false, fileBusy: false, discarding: false, candidates: [], sampleAvailable: false, editCount: 0, onClose: vi.fn(), onCancel: vi.fn(), onAdd: vi.fn(), onRemove: vi.fn(), onMove: vi.fn(), onUpdate: vi.fn(), onTotalWidth: vi.fn(), onBinding: vi.fn(async () => true), onConfigure: vi.fn(), onFooter: vi.fn(), onHeaderHeight: vi.fn(), onAltRowBackground: vi.fn(), onHeaderStyle: vi.fn(), onMinHeight: vi.fn(), onRules: vi.fn(), onCellPadding: vi.fn() }
    const view = render(<TableEditor {...props} />)
    return { ...view, props }
  }

  it('presses the resolved header alignment in HEADER ALIGN while unset, and a segment only ever sets headerAlign', () => {
    const { props } = setup()
    expect(screen.getByRole('toolbar', { name: 'Header label alignment for column 1' })).toBeInTheDocument()
    // NO NEW GROUP: the control is a toolbar, so the contract's group floor holds.
    expect(screen.queryByRole('group', { name: /Header label alignment/ })).toBeNull()
    const followed = screen.getByRole('button', { name: 'Header align right for column 1' })
    expect(followed).toHaveAttribute('aria-pressed', 'true')
    expect(followed.getAttribute('title')).toContain('what the header prints until set')
    fireEvent.click(screen.getByRole('button', { name: 'Header align center for column 1' }))
    expect(props.onUpdate).toHaveBeenLastCalledWith('c1', 'headerAlign', 'center')
    // Pressing the followed segment makes the value explicit; there is no clear.
    fireEvent.click(followed)
    expect(props.onUpdate).toHaveBeenLastCalledWith('c1', 'headerAlign', 'right')
    expect(props.onUpdate.mock.calls.every(([, field]) => field === 'headerAlign')).toBe(true)
  })

  it('presses headerAlignResolved, not the cell alignment, when a table-wide header alignment differs from the column\'s', () => {
    // Go resolved the header to center (a table-wide header alignment the column
    // does not override), while the column's cell alignment is left.
    const { props } = setup({ headerAlign: 'center', headerAlignResolved: 'center' }, [{ ...defaultColumns[0]!, align: 'left', headerAlignResolved: 'center' }])
    const printed = screen.getByRole('button', { name: 'Header align center for column 1' })
    expect(printed).toHaveAttribute('aria-pressed', 'true')
    expect(printed.getAttribute('title')).toContain('what the header prints until set')
    expect(screen.getByRole('button', { name: 'Header align left for column 1' })).toHaveAttribute('aria-pressed', 'false')
    expect(screen.getByRole('button', { name: 'Align left for column 1' })).toHaveAttribute('aria-pressed', 'true')
    // Its Tab stop is the pressed segment.
    screen.getByRole('spinbutton', { name: 'Width for column 1 in points' }).focus()
    fireEvent.keyDown(document.activeElement!, { key: 'Tab' })
    expect(document.activeElement).toBe(printed)
    // Clicking the pressed segment of an unset header writes that value.
    fireEvent.click(printed)
    expect(props.onUpdate).toHaveBeenLastCalledWith('c1', 'headerAlign', 'center')
  })

  it('shows a committed headerAlign apart from the column\'s cell alignment', () => {
    const { props } = setup({}, [{ ...defaultColumns[0]!, headerAlign: 'center' }])
    expect(screen.getByRole('button', { name: 'Header align center for column 1' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Header align right for column 1' })).toHaveAttribute('aria-pressed', 'false')
    expect(screen.getByRole('button', { name: 'Align right for column 1' })).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(screen.getByRole('button', { name: 'Header align center for column 1' }))
    expect(props.onUpdate).not.toHaveBeenCalled()
  })

  it('tabs header label → binding → width → header align → cell align → footer aggregate', () => {
    setup()
    const header = screen.getByRole('textbox', { name: 'Header for column 1' })
    header.focus()
    for (const next of [
      screen.getByRole('combobox', { name: 'Binding for column 1' }),
      screen.getByRole('spinbutton', { name: 'Width for column 1 in points' }),
      screen.getByRole('button', { name: 'Header align right for column 1' }),
      screen.getByRole('button', { name: 'Align right for column 1' }),
      screen.getByRole('combobox', { name: 'Footer aggregate for column 1' }),
    ]) {
      fireEvent.keyDown(document.activeElement!, { key: 'Tab' })
      expect(document.activeElement).toBe(next)
    }
    fireEvent.keyDown(document.activeElement!, { key: 'Tab', shiftKey: true })
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Align right for column 1' }))
    fireEvent.keyDown(document.activeElement!, { key: 'Tab', shiftKey: true })
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Header align right for column 1' }))
  })

  it('commits cell padding left and right on blur, clears an emptied box and sends a non-number as typed', () => {
    const { props } = setup({ paddingLeft: '3000' })
    const left = screen.getByRole('textbox', { name: 'Cell padding left in points' })
    const right = screen.getByRole('textbox', { name: 'Cell padding right in points' })
    expect(left).toHaveValue('3')
    expect(right).toHaveValue('')
    fireEvent.blur(left)
    expect(props.onCellPadding).not.toHaveBeenCalled()
    fireEvent.blur(right, { target: { value: '4' } })
    expect(props.onCellPadding).toHaveBeenLastCalledWith('paddingRight', 'set', '4')
    fireEvent.blur(left, { target: { value: '' } })
    expect(props.onCellPadding).toHaveBeenLastCalledWith('paddingLeft', 'clear')
    fireEvent.blur(right, { target: { value: 'wide' } })
    expect(props.onCellPadding).toHaveBeenLastCalledWith('paddingRight', 'set', 'wide')
  })

  it('restores a refused padding box to the committed value', () => {
    const { props, rerender } = setup({ paddingLeft: '3000' })
    const left = screen.getByRole('textbox', { name: 'Cell padding left in points' })
    fireEvent.change(left, { target: { value: 'wide' } })
    rerender(<TableEditor {...props} error="e7: paddingLeft: must be a number" />)
    expect(screen.getByRole('textbox', { name: 'Cell padding left in points' })).toHaveValue('3')
    expect(screen.getByRole('alert')).toHaveTextContent('paddingLeft')
  })

  it('says the header row keeps its own padding when headerStyle.padding exists', () => {
    const { unmount } = setup()
    expect(screen.getByText(/on the header, data and footer rows/)).toBeInTheDocument()
    unmount()
    setup({ paddingHeaderOverride: true })
    expect(screen.getByText(/The header row uses its own padding/)).toBeInTheDocument()
  })

  it('opens each (i) explanation by its named button and closes it on a second press, on blur and on Escape before the dialog', () => {
    const { props } = setup()
    const bindings = screen.getByRole('button', { name: 'About bindings' })
    expect(bindings).toHaveAttribute('aria-expanded', 'false')
    // aria-controls names the panel only while it exists.
    expect(bindings).not.toHaveAttribute('aria-controls')
    fireEvent.click(bindings)
    expect(bindings).toHaveAttribute('aria-expanded', 'true')
    const note = screen.getByRole('note')
    expect(bindings).toHaveAttribute('aria-controls', note.id)
    expect(document.getElementById(note.id)).toBe(note)
    for (const fact of ['{{row.date}}', '{{upper(row.trn_code)}}', 'Shift+Enter', 'Alt+Down']) expect(note).toHaveTextContent(fact)
    fireEvent.click(bindings)
    expect(screen.queryByRole('note')).toBeNull()
    expect(bindings).not.toHaveAttribute('aria-controls')

    const widths = screen.getByRole('button', { name: 'About column widths' })
    fireEvent.click(widths)
    expect(screen.getByRole('note')).toHaveTextContent('Point widths')
    fireEvent.blur(widths)
    expect(screen.queryByRole('note')).toBeNull()

    fireEvent.click(widths)
    const dialog = screen.getByRole('dialog', { name: 'Table Editor' })
    fireEvent.keyDown(dialog, { key: 'Escape' })
    expect(screen.queryByRole('note')).toBeNull()
    expect(props.onClose).not.toHaveBeenCalled()
    fireEvent.keyDown(dialog, { key: 'Escape' })
    expect(props.onClose).toHaveBeenCalledOnce()
  })

  it('keeps an open explanation open on a mousedown inside its panel', () => {
    setup()
    const bindings = screen.getByRole('button', { name: 'About bindings' })
    bindings.focus()
    fireEvent.click(bindings)
    const note = screen.getByRole('note')
    // The mousedown's default (moving focus off the button) is prevented, so the
    // button keeps focus and its blur never closes the panel.
    expect(fireEvent.mouseDown(note)).toBe(false)
    expect(document.activeElement).toBe(bindings)
    expect(screen.getByRole('note')).toBe(note)
    expect(bindings).toHaveAttribute('aria-expanded', 'true')
  })

  it('moves the proportion sentence into the PROPORTION explanation', () => {
    setup({}, defaultColumns, 'proportion')
    expect(screen.queryByText(/Proportion sizing ·/)).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'About proportion sizing' }))
    expect(screen.getByRole('note')).toHaveTextContent('Proportion sizing · Columns share the table’s total width according to their proportions')
    expect(screen.getByRole('note')).toHaveTextContent('New columns start at 1')
  })
})

describe('proportion controls and pending numeric actions', () => {
  const setup = (accept = true) => {
    const onUpdate = vi.fn(async () => accept)
    const onTotalWidth = vi.fn(async () => accept)
    const onAdd = vi.fn(); const onClose = vi.fn(); const onCancel = vi.fn()
    const projection = { revision: 1, table: { tableId: 'e7', sizing: 'proportion' as const, totalWidth: 500000, collection: 'items[]', alias: 'row', ...tableHeaderProjection, columns: projected(defaultColumns).map((column, index) => ({ ...column, proportion: index === 1 ? '2' : '1', width: index === 1 ? 250000 : 125000 })) } }
    const props = { projection, busy: false, fileBusy: false, discarding: false, candidates: [], sampleAvailable: false, editCount: 0, onUpdate, onTotalWidth, onAdd, onClose, onCancel, onRemove: vi.fn(), onMove: vi.fn(), onBinding: vi.fn(async () => true), onConfigure: vi.fn(), onFooter: vi.fn(), onHeaderHeight: vi.fn(), onAltRowBackground: vi.fn(), onHeaderStyle: vi.fn(), onMinHeight: vi.fn(), onRules: vi.fn(), onCellPadding: vi.fn() }
    const rendered = render(<TableEditor {...props} />)
    return { ...rendered, props, onUpdate, onTotalWidth, onAdd, onClose, onCancel }
  }
  it('displays authored proportions and Go-resolved widths without a conversion selector', () => {
    setup()
    expect(screen.getByRole('group', { name: 'Proportion sizing' })).toBeVisible()
    expect(screen.getByRole('spinbutton', { name: 'Total table width in points' })).toHaveValue(500)
    expect(screen.getByRole('textbox', { name: 'Proportion for column 2' })).toHaveValue('2')
    expect(screen.getByLabelText('Resolved width for column 2 in points')).toHaveTextContent('250 pt')
    expect(screen.getByLabelText('Resolved width for column 2 in points')).toHaveAttribute('aria-live', 'off')
    expect(screen.getByText(/Add column starts with proportion 1/)).toBeVisible()
    expect(screen.queryByText(/another 72pt column/)).toBeNull()
    expect(screen.queryByRole('combobox', { name: /sizing/i })).not.toBeInTheDocument()
  })
  it.each(['9007199254740.991', '9223372036854775.807'])('preserves the exact proportion %s without a native floating-point stepper', async (value) => {
    const h = setup()
    const input = screen.getByRole('textbox', { name: 'Proportion for column 1' }) as HTMLInputElement
    expect(input).toHaveAttribute('inputmode', 'decimal')
    input.focus(); fireEvent.change(input, { target: { value } })
    expect(() => input.stepUp()).toThrow()
    expect(input).toHaveValue(value)
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    expect(screen.getByRole('textbox', { name: 'Proportion for column 2' })).toHaveFocus()
    await waitFor(() => expect(h.onUpdate).toHaveBeenCalledWith(defaultColumns[0]!.id, 'proportion', value))
  })
  it('recovers a replaced total control but preserves deliberate focus elsewhere', () => {
    const h = setup()
    const total = () => screen.getByRole('spinbutton', { name: 'Total table width in points' })
    total().focus()
    const changed = { ...h.props, projection: { ...h.props.projection, table: { ...h.props.projection.table, totalWidth: 400000 } } }
    h.rerender(<TableEditor {...changed} />)
    expect(total()).toHaveFocus()
    const alias = screen.getByRole('textbox', { name: 'Row alias' })
    alias.focus()
    h.rerender(<TableEditor {...h.props} />)
    expect(alias).toHaveFocus()
    total().focus()
    h.rerender(<TableEditor {...h.props} error="e8: allocation gives this column zero width" />)
    expect(total()).toHaveFocus()
  })
  it('preserves the native total-blur destination when busy interrupts that focus transfer', () => {
    const h = setup()
    const input = screen.getByRole('spinbutton', { name: 'Total table width in points' })
    const header = screen.getByRole('textbox', { name: 'Header for column 1' })
    input.focus()
    fireEvent.change(input, { target: { value: '400' } })
    fireEvent.blur(input, { relatedTarget: header })
    h.rerender(<TableEditor {...h.props} busy />)
    h.rerender(<TableEditor {...h.props} projection={{ ...h.props.projection, table: { ...h.props.projection.table, totalWidth: 400000 } }} />)
    expect(header).toHaveFocus()
  })
  for (const field of ['total', 'proportion'] as const) for (const action of ['Add column', 'Done', 'Escape', 'Cancel'] as const) {
    it(`${action} handles a pending ${field} draft`, async () => {
      const h = setup()
      const input = screen.getByRole(field === 'total' ? 'spinbutton' : 'textbox', { name: field === 'total' ? 'Total table width in points' : 'Proportion for column 1' })
      input.focus(); fireEvent.change(input, { target: { value: field === 'total' ? '400' : '1.234' } })
      if (action === 'Escape') fireEvent.keyDown(input, { key: 'Escape' })
      else { const button = screen.getByRole('button', { name: action }); fireEvent.mouseDown(button, { button: 0 }); fireEvent.click(button) }
      if (action === 'Cancel') { expect(h.onUpdate).not.toHaveBeenCalled(); expect(h.onTotalWidth).not.toHaveBeenCalled(); expect(h.onCancel).toHaveBeenCalledOnce(); return }
      await waitFor(() => expect(action === 'Add column' ? h.onAdd : h.onClose).toHaveBeenCalledOnce())
      if (field === 'total') expect(h.onTotalWidth).toHaveBeenCalledExactlyOnceWith('400')
      else expect(h.onUpdate).toHaveBeenCalledExactlyOnceWith(defaultColumns[0]!.id, 'proportion', '1.234')
    })
  }
  it('refuses pending invalid ratios without closing or adding, then restores committed input with its located error', async () => {
    const h = setup(false)
    const input = screen.getByRole('textbox', { name: 'Proportion for column 1' })
    fireEvent.change(input, { target: { value: '0' } }); fireEvent.keyDown(input, { key: 'Escape' })
    await waitFor(() => expect(h.onUpdate).toHaveBeenCalledWith(defaultColumns[0]!.id, 'proportion', '0'))
    expect(h.onClose).not.toHaveBeenCalled(); expect(h.onAdd).not.toHaveBeenCalled()
    h.rerender(<TableEditor {...h.props} error="e8: proportion must be positive" />)
    expect(input).toHaveValue('1')
    expect(screen.getByRole('alert')).toHaveTextContent('e8: proportion must be positive')
  })
})

// SPEC-table-rules' RULED AREA section: the interior lines and the ruled area's
// floor, authored in the TABLE EDITOR because the owner ruled they belong
// beside the header and the rows rather than in the inspector's BOX section —
// which authors the table's own frame, and which since this change is telling
// the truth about a table.
describe('the ruled area section', () => {
  const projectionWith = (over: Partial<typeof tableHeaderProjection>) => ({ revision: 1, table: { tableId: 'e7', sizing: 'points' as const, totalWidth: 72_000, collection: 'transactions[]', alias: 'row', ...tableHeaderProjection, ...over, columns: projected(defaultColumns) } })
  const mount = (over: Partial<typeof tableHeaderProjection> = {}) => {
    const onMinHeight = vi.fn()
    const onRules = vi.fn()
    render(<TableEditor projection={projectionWith(over)} busy={false} fileBusy={false} discarding={false} candidates={[]} sampleAvailable={false} editCount={0} onClose={vi.fn()} onCancel={vi.fn()} onAdd={vi.fn()} onRemove={vi.fn()} onMove={vi.fn()} onUpdate={vi.fn()} onTotalWidth={vi.fn()} onBinding={vi.fn()} onConfigure={vi.fn()} onFooter={vi.fn()} onHeaderHeight={vi.fn()} onAltRowBackground={vi.fn()} onHeaderStyle={vi.fn()} onMinHeight={onMinHeight} onRules={onRules} onCellPadding={vi.fn()} />)
    return { onMinHeight, onRules }
  }

  it('shows an absent floor as an empty box and commits the author\u2019s draft as typed', () => {
    const { onMinHeight } = mount()
    const box = screen.getByLabelText('Minimum height in points') as HTMLInputElement
    expect(box.value).toBe('')
    fireEvent.change(box, { target: { value: '600' } })
    fireEvent.blur(box)
    expect(onMinHeight).toHaveBeenCalledWith('set', '600')
  })

  it('shows a committed floor in points and clears it when the box is emptied', () => {
    const { onMinHeight } = mount({ minHeight: 600_000 })
    const box = screen.getByLabelText('Minimum height in points') as HTMLInputElement
    expect(box.value).toBe('600')
    fireEvent.change(box, { target: { value: '' } })
    fireEvent.blur(box)
    expect(onMinHeight).toHaveBeenCalledWith('clear')
  })

  it('ticks the boundaries the document declares and sends the whole set on a change', () => {
    const { onRules } = mount({ 'rules.between': 'columns' })
    const columnsBox = screen.getByLabelText('Rule between columns') as HTMLInputElement
    const rowsBox = screen.getByLabelText('Rule between rows') as HTMLInputElement
    expect(columnsBox.checked).toBe(true)
    expect(rowsBox.checked).toBe(false)
    fireEvent.click(rowsBox)
    // CANONICAL ORDER, because the engine's projection joins in that order and
    // its guard admits only that order.
    expect(onRules).toHaveBeenCalledWith('between', 'set', 'columns,rows')
  })

  it('treats unticking the last boundary as the clear, because an empty set paints nothing', () => {
    const { onRules } = mount({ 'rules.between': 'rows' })
    fireEvent.click(screen.getByLabelText('Rule between rows'))
    expect(onRules).toHaveBeenCalledWith('between', 'clear')
  })

  it('states the engine\u2019s resolved width and colour as placeholders, never as values', () => {
    mount({ 'rules.between': 'columns', 'rules.widthResolved': '500', 'rules.colorResolved': '#000000' })
    const width = screen.getByLabelText('Rule width (pt)') as HTMLInputElement
    expect(width.value).toBe('')
    expect(width.placeholder).toContain('0.5')
    const colour = screen.getByLabelText('Rule colour') as HTMLInputElement
    expect(colour.value).toBe('')
    expect(colour.placeholder).toContain('#000000')
  })

  it('commits a typed floor with a decimal and clears through the \u00d7', () => {
    const { onMinHeight } = mount({ minHeight: 600_000 })
    const box = screen.getByLabelText('Minimum height in points') as HTMLInputElement
    fireEvent.change(box, { target: { value: '12.5' } })
    fireEvent.blur(box)
    expect(onMinHeight).toHaveBeenCalledTimes(1)
    expect(onMinHeight).toHaveBeenLastCalledWith('set', '12.5')
    fireEvent.click(screen.getByRole('button', { name: 'Clear Minimum height' }))
    expect(onMinHeight).toHaveBeenLastCalledWith('clear')
  })

  it('sends the boundaries in canonical order whichever is ticked first', () => {
    const { onRules } = mount({ 'rules.between': 'rows' })
    fireEvent.click(screen.getByLabelText('Rule between columns'))
    expect(onRules).toHaveBeenCalledTimes(1)
    expect(onRules).toHaveBeenLastCalledWith('between', 'set', 'columns,rows')
  })

  it('ticking the first boundary from none sends a set of that one', () => {
    const { onRules } = mount()
    fireEvent.click(screen.getByLabelText('Rule between rows'))
    expect(onRules).toHaveBeenCalledTimes(1)
    expect(onRules).toHaveBeenLastCalledWith('between', 'set', 'rows')
  })

  it('disables rule width and colour until a boundary is ticked, because a boundaryless block draws nothing', () => {
    mount()
    expect((screen.getByLabelText('Rule width (pt)') as HTMLInputElement).disabled).toBe(true)
    expect((screen.getByLabelText('Rule colour') as HTMLInputElement).disabled).toBe(true)
    expect((screen.getByLabelText('Pick Rule colour') as HTMLInputElement).disabled).toBe(true)
    expect((screen.getByRole('button', { name: 'Clear Rule width (pt)' }) as HTMLButtonElement).disabled).toBe(true)
    expect((screen.getByRole('button', { name: 'Clear Rule colour' }) as HTMLButtonElement).disabled).toBe(true)
    cleanup()
    // The positive control: the same controls are live once a boundary is.
    const { onRules } = mount({ 'rules.between': 'columns', 'rules.widthResolved': '500', 'rules.colorResolved': '#000000' })
    const width = screen.getByLabelText('Rule width (pt)') as HTMLInputElement
    const colour = screen.getByLabelText('Rule colour') as HTMLInputElement
    expect(width.disabled).toBe(false)
    expect(colour.disabled).toBe(false)
    fireEvent.change(width, { target: { value: '0.75' } })
    fireEvent.blur(width)
    expect(onRules).toHaveBeenLastCalledWith('width', 'set', '0.75')
    fireEvent.change(colour, { target: { value: '#336699' } })
    fireEvent.blur(colour)
    expect(onRules).toHaveBeenLastCalledWith('color', 'set', '#336699')
  })

  it('still lets a committed width be cleared when no boundary is ticked', () => {
    const { onRules } = mount({ 'rules.width': '500', 'rules.widthResolved': '500' })
    expect((screen.getByLabelText('Rule width (pt)') as HTMLInputElement).disabled).toBe(true)
    const clear = screen.getByRole('button', { name: 'Clear Rule width (pt)' }) as HTMLButtonElement
    expect(clear.disabled).toBe(false)
    fireEvent.click(clear)
    expect(onRules).toHaveBeenLastCalledWith('width', 'clear')
  })

  it('disables the Minimum height clear while no floor is declared', () => {
    mount()
    expect((screen.getByRole('button', { name: 'Clear Minimum height' }) as HTMLButtonElement).disabled).toBe(true)
    cleanup()
    const { onMinHeight } = mount({ minHeight: 600_000 })
    const clear = screen.getByRole('button', { name: 'Clear Minimum height' }) as HTMLButtonElement
    expect(clear.disabled).toBe(false)
    fireEvent.click(clear)
    expect(onMinHeight).toHaveBeenLastCalledWith('clear')
  })

  it('says the floor applies to each page\u2019s slice and that column rules run each slice\u2019s height', () => {
    mount()
    const note = screen.getByLabelText('Minimum height note').textContent ?? ''
    expect(note).not.toContain('whichever is more')
    expect(note).toContain('each page')
    expect(note).toContain('content bottom')
    const honest = screen.getByText(/A rule is drawn once/).textContent ?? ''
    expect(honest).not.toContain('full height of the box')
    expect(honest).toContain('each page\u2019s slice')
  })

  it('says nothing is drawn when the document declares no rules block', () => {
    mount()
    expect((screen.getByLabelText('Rule width (pt)') as HTMLInputElement).placeholder).toContain('no rules drawn')
    expect(screen.getByLabelText('Resolved ruled boundaries').textContent).toContain('no interior lines')
  })
})

// SPEC-table-rules \u00a74: a column label may hold a line feed, so the control
// that authors one must be able to hold one. An `<input>` cannot: the character
// is dropped on paste and unreachable from the keyboard, so the control refused
// a value the format admits.
describe('a column label may be more than one line', () => {
  it('authors the header label in a control that accepts a line feed', () => {
    const onUpdate = vi.fn()
    const projection = { revision: 1, table: { tableId: 'e7', sizing: 'points' as const, totalWidth: 72_000, collection: 'transactions[]', alias: 'row', ...tableHeaderProjection, columns: projected(defaultColumns) } }
    render(<TableEditor projection={projection} busy={false} fileBusy={false} discarding={false} candidates={[]} sampleAvailable={false} editCount={0} onClose={vi.fn()} onCancel={vi.fn()} onAdd={vi.fn()} onRemove={vi.fn()} onMove={vi.fn()} onUpdate={onUpdate} onTotalWidth={vi.fn()} onBinding={vi.fn()} onConfigure={vi.fn()} onFooter={vi.fn()} onHeaderHeight={vi.fn()} onAltRowBackground={vi.fn()} onHeaderStyle={vi.fn()} onMinHeight={vi.fn()} onRules={vi.fn()} onCellPadding={vi.fn()} />)
    const label = screen.getByLabelText('Header for column 1')
    expect(label.tagName).toBe('TEXTAREA')
    fireEvent.change(label, { target: { value: '\u0e27\u0e31\u0e19\u0e17\u0e35\u0e48\nDATE' } })
    fireEvent.blur(label)
    expect(onUpdate).toHaveBeenCalledWith(defaultColumns[0]!.id, 'header', '\u0e27\u0e31\u0e19\u0e17\u0e35\u0e48\nDATE')
    expect(onUpdate).toHaveBeenCalledTimes(1)
    // And the plain two-line case, byte for byte.
    const second = screen.getByLabelText('Header for column 2')
    fireEvent.change(second, { target: { value: 'A\nB' } })
    fireEvent.blur(second)
    expect(onUpdate).toHaveBeenLastCalledWith(defaultColumns[1]!.id, 'header', 'A\nB')
  })
})
