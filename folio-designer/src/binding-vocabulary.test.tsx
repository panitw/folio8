import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import App, { PROSE_COMMIT_DEBOUNCE_MS } from './App'
import type { CanvasProjection, EngineSnapshot } from './engine-protocol'
import type { EngineClient } from './engine-client'

// STORY 14.4 — THE PANEL OFFERS NO CONTROL THE ENGINE WILL REFUSE.
//
// This file covers the INSPECTOR half. `DataPanel.test.tsx` covers the
// pre-flight ladder; `engine-bounds-mirror.test.ts` covers the Go/TypeScript
// tie that makes the ladder's judgement legal at all.
//
// WHAT WAS WRONG. The BINDING section had no kind gate, so selecting a Line, a
// Rectangle, an Image or a Table produced *"No engine binding on this
// component. Pick a root scalar in the Data tab."* — an instruction to attempt
// something `bindComponentScalar` refuses with *"only text components can
// receive a scalar binding"*. Separately a Table's binding was stated THREE
// times in this panel and editable in NONE of them.
//
// ⚠ ASSERT THE RENDERED TEXT, NEVER `FieldSpec.label`. Story 14.2's three false
// greens were all assertions that resolved through an accessible name which is
// never rendered on screen. Every assertion below reads the panel's own text or
// the presence of a section in the DOM.

const canvas: CanvasProjection = { width: 595276, height: 841890, orientation: 'portrait', preset: 'A4', locale: 'en', utcOffset: '+07:00', marginTop: 36000, marginRight: 36000, marginBottom: 36000, marginLeft: 36000, gridIncrement: 6000, commandWidth: 595276, commandHeight: 841890, fontFamilies: ['body'], fontChains: [{ name: 'body', entries: [{ face: 'Noto Sans', assetKey: '', family: '', style: '', bold: '', italic: '', boldItalic: '' }] }], defaultFontSize: 12000, defaultLineSpacing: 1000, contentWindowHeight: 729890, contentWindowCount: 1, contentWindowOrigins: [0], contentWindowPages: [0], contentWindowCountIsExact: true, bands: [{ name: 'pageHeader', x: 36000, y: 36000, width: 523276, height: 20000 }, { name: 'content', x: 36000, y: 56000, width: 523276, height: 729890 }, { name: 'pageFooter', x: 36000, y: 785890, width: 523276, height: 20000 }], components: [] }

type Component = CanvasProjection['components'][number]

const text: Component = { id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello' }
const boundText: Component = { ...text, value: '{{customer.name}}', binding: 'customer.name' }
const line: Component = { id: 'e1', type: 'line', band: 'content', x: 0, y: 0, width: 72_000, height: 1_000, resizable: true, background: '#000000' }
const rect: Component = { id: 'e1', type: 'rect', band: 'content', x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, background: '#1b2a4a' }
const image: Component = { id: 'e1', type: 'image', band: 'content', x: 0, y: 0, width: 72_000, height: 24_000, resizable: true }
const table: Component = { id: 'e1', type: 'table', band: 'content', x: 0, y: 30_000, width: 72_000, height: 12_000, resizable: false, tableBind: 'transactions[]' }
const unboundTable: Component = { ...table, tableBind: undefined }

// A fake that RECORDS every command and answers a fixed projection. Recording
// is the point: AC4's claim is that selecting a component dispatches ZERO
// commands, and only the request log can witness that.
function open(components: ReadonlyArray<Component>) {
  const sent: ArrayBuffer[] = []
  const snapshot = (): EngineSnapshot => ({ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components } })
  const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
    if (operation === 'command' && payload) sent.push(payload)
    return { snapshot: snapshot() }
  })
  render(<App engine={{ request } as unknown as EngineClient} initialSnapshot={snapshot()} />)
  return { sent, request }
}

const select = (id: string, kind: string) => fireEvent.click(screen.getByLabelText(new RegExp(`^${kind} component ${id}`)))
const panel = () => screen.getByLabelText('Properties panel')
const bindingSection = () => panel().querySelector('.property-section-binding')
// ⚠ A SINGLE MICROTASK IS NOT A FLUSH, and "no command was dispatched" is
// exactly the assertion a weak flush passes vacuously. Copied from
// line-rect-vocabulary.test.tsx and, like it, read from the panel's own
// debounce constant so it cannot drift under the timer it must outlast.
const settle = async () => {
  await new Promise((resolve) => setTimeout(resolve, PROSE_COMMIT_DEBOUNCE_MS + 20))
  for (let turn = 0; turn < 4; turn++) await Promise.resolve()
}
const occurrences = (haystack: string, needle: string) => haystack.split(needle).length - 1

// D-14.4.Q2 REFUSED OPTION (b) — a main-window editor for the table's own
// collection. This is the fence around that refusal, and it is asserted
// against the TABLE SECTION'S OWN DOM.
//
// ⚠ WHAT IT REPLACED, AND WHY. The first version queried for textboxes named
// `Root collection` and `Row alias`. Those two accessible names exist ONLY in
// `TableEditor.tsx`, which this file never renders — so both queries returned
// null no matter what the inspector contained, and a review DEMONSTRATED it: a
// real inspector-side collection editor was added to the TABLE section and the
// whole suite stayed green at 1246/0. The refused capability could have shipped
// under a passing fence. A negative assertion phrased in names the render can
// never produce is not a fence; it is decoration.
//
// So this asks the structural question instead: the TABLE section holds NO form
// control at all, and exactly one interactive element — the editor door, which
// Story 14.7 needs to still exist.
const expectNoInspectorSideTableEditor = () => {
  const section = panel().querySelector('.property-section-table')
  expect(section).not.toBeNull()
  expect(Array.from(section!.querySelectorAll('input, select, textarea, [contenteditable]'))).toHaveLength(0)
  const interactive = Array.from(section!.querySelectorAll('button, [role="button"], input, select, textarea, a[href]'))
  expect(interactive).toHaveLength(1)
  expect((interactive[0]?.textContent ?? '').trim()).toBe('Configure columns')
  // AND NOWHERE ELSE IN THE PANEL EITHER, so the refused control cannot simply
  // move one section down and keep this green.
  expect(screen.queryByRole('textbox', { name: /collection/i })).not.toBeInTheDocument()
  expect(screen.queryByRole('textbox', { name: /alias/i })).not.toBeInTheDocument()
}

describe('the BINDING section is offered only where the engine can honour it', () => {
  it('renders it, and its Data-tab note, for an unbound text component', () => {
    open([text])
    select('e1', 'text')
    expect(bindingSection()).not.toBeNull()
    expect(screen.getByText('No engine binding on this component. Pick a root scalar in the Data tab.')).toBeInTheDocument()
  })

  it('renders the bind chip for a bound text component', () => {
    open([boundText])
    select('e1', 'text')
    expect(bindingSection()).not.toBeNull()
    expect(screen.getByText('Bound to').parentElement).toHaveTextContent('Bound to customer.name')
    // The invitation is GONE once there is a binding — the two branches are
    // exclusive, and this is the non-vacuous half of the row above.
    expect(screen.queryByText(/Pick a root scalar in the Data tab/)).not.toBeInTheDocument()
  })

  // D-14.4.Q1: HIDE, not disable-and-explain. A section that exists only to
  // explain why it is empty is the disabled-and-mysterious case wearing a
  // paragraph. Go writes `component.Binding` at exactly one site, inside
  // `if element.Type == template.ElementText`, so hiding suppresses NO value
  // for these kinds and AC4 does not bite on AC1.
  it.each([
    ['line', line],
    ['rect', rect],
    ['image', image],
    ['table', table],
  ])('withholds it entirely for a %s, and invites no pick anywhere in the panel', (kind, component) => {
    open([component])
    select('e1', kind)
    expect(bindingSection()).toBeNull()
    expect(screen.queryByText(/Pick a root scalar in the Data tab/)).not.toBeInTheDocument()
    // NOT MERELY THAT ONE SENTENCE. The claim is that NOTHING in the inspector
    // invites the author to pick a data path for this kind.
    expect(panel().textContent ?? '').not.toMatch(/root scalar/i)
    expect(panel().textContent ?? '').not.toMatch(/Pick a root scalar/i)
  })

  // The negative half, and it is not vacuous: this selection differs from the
  // four above ONLY in carrying more than one component.
  it('omits the single-component binding section for multi-selection', () => {
    open([text, { ...line, id: 'e2', y: 100_000 }])
    select('e1', 'text')
    fireEvent.click(screen.getByLabelText(/^line component e2/), { shiftKey: true })
    expect(bindingSection()).toBeNull()
    expect(screen.queryByText('Binding is shown for one selected component.')).not.toBeInTheDocument()
  })
})

describe('a Table states its binding once, and names where it is edited', () => {
  it('states a present binding exactly once and points at the table editor', () => {
    open([table])
    select('e1', 'table')
    const text_ = panel().textContent ?? ''
    // ONCE. Before this story the value reached the panel through the TABLE
    // section's `(display only)` line, and the BINDING section spoke for a
    // table as well — three statements, none of them editable.
    expect(occurrences(text_, 'transactions[]')).toBe(1)
    expect(occurrences(text_, 'Table binding:')).toBe(1)
    expect(text_).not.toContain('(display only)')
    const note = screen.getByText(/^Table binding: transactions\[\]/)
    expect(note).toHaveTextContent('edited in the table editor, under Configure columns')
    expect(note).toHaveTextContent('The stored value is unchanged')
    expect(note).toHaveClass('honest-note')
    expectNoInspectorSideTableEditor()
    // The value is STATED, never sitting in a box a focus-and-blur could
    // commit — the `shown: true` hazard this story is immune to by construction
    // (it authors no FieldSpec at all).
    expect(Array.from(panel().querySelectorAll('input')).map((input) => input.value)).not.toContain('transactions[]')
  })

  it('says so when there is no binding, without offering an inspector-side edit', () => {
    open([unboundTable])
    select('e1', 'table')
    const note = screen.getByText(/^Table binding: Not set/)
    expect(note).toHaveTextContent('edited in the table editor, under Configure columns')
    expect(occurrences(panel().textContent ?? '', 'Table binding:')).toBe(1)
    expectNoInspectorSideTableEditor()
  })
})

// I-5, INHERITED VERBATIM FROM STORY 14.2: the panel narrowing its vocabulary
// is never a document migration. Selecting an element writes nothing.
//
// ⚠ THIS GREEN IS NOT INHERITED. 14.2 required this mutation proof and recorded
// no outcome for it, so it is executed here rather than assumed: reverting the
// zero-dispatch guard (by dispatching a command on selection) must red this.
//
// ⚠ WHAT JSDOM CANNOT PROVE, NAMED RATHER THAN LEFT TO INFERENCE. It cannot see
// serialized `.folio` bytes: the save path is `engine.request('serialize')`,
// answered by the wasm engine, and every designer unit test injects a fake. The
// only place real serialize bytes are observable is
// `e2e/browser-native-roundtrip.spec.ts`, which this cadence COMPILES and DOES
// NOT RUN. What closes the gap is that this story changes NO Go code and adds
// no command: same gesture, same bytes, same document.
describe('selecting a component of any kind writes nothing', () => {
  it.each([
    ['text', text],
    ['line', line],
    ['rect', rect],
    ['image', image],
    ['table', table],
  ])('dispatches zero commands when a %s is selected', async (kind, component) => {
    const harness = open([component])
    select('e1', kind)
    await settle()
    expect(harness.sent).toHaveLength(0)
    expect(harness.request.mock.calls.filter(([operation]) => operation === 'command')).toHaveLength(0)
  })

  // ⚠ THIS TEST READS THE VALUES BACK, and the earlier version did not. It set
  // `borderWidth` and `borderColor` on the fixture, called itself "still
  // projects the values the panel has stopped speaking about", and then
  // asserted only that zero commands were sent. A review DEMONSTRATED the hole
  // by narrowing the BOX border row to `{!line && !table && …}` — the panel
  // stops showing a table's stored border, which is precisely the AC4
  // regression this test is named for — and the suite stayed green at 1246/0.
  //
  // "Nothing was WRITTEN" and "everything is still SHOWN" are two different
  // claims, and only the first was being made.
  it('still shows the values it never offered a control for, and writes nothing', async () => {
    const harness = open([{ ...table, borderWidth: 2_000, borderColor: '#c81e1e' }])
    select('e1', 'table')
    await settle()
    expect(harness.sent).toHaveLength(0)
    // THE BINDING, which this story removed the BINDING section for.
    expect(screen.getByText(/^Table binding: transactions\[\]/)).toBeInTheDocument()
    // AND THE BORDER, read back as rendered values rather than assumed from the
    // fixture. Millipoints in the projection, points on the row.
    expect(screen.getByRole('textbox', { name: 'Border width (pt)' })).toHaveValue('2')
    expect(screen.getByRole('textbox', { name: 'Border colour' })).toHaveValue('#c81e1e')
  })
})
