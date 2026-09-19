import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import App, { PROSE_COMMIT_DEBOUNCE_MS } from './App'
import type { CanvasProjection, EngineSnapshot } from './engine-protocol'
import type { EngineClient } from './engine-client'

// STORY 14.2 — A LINE IS A THICKNESS AND A COLOUR; A RECTANGLE IS A FILL.
//
// ⚠ THIS FILE IS THE PROJECT'S FIRST LINE-SELECTION COVERAGE. Measured before
// it was written: `type: 'line'` appeared nowhere in `folio-designer/src` or
// `folio-designer/e2e`, while the identical shape for `'rect'` returned ten or
// more hits. So the suite's greenness on this surface was worth nothing — not
// because the tests were weak, but because nothing was watching it at all.
//
// Story 14.2 changes the inspector words. The follow-up tests at the end
// also cover canvas endpoint resizing while preserving Thickness. A Line is drawn as a
// very short, very wide filled box; that is how the engine models it and this
// story does not move it. Every control below writes exactly the field it wrote
// when it was called `H`, `W` or `Background`, through the same
// `updateComponentProperties`. The byte proofs are what hold that to account.

// ---------------------------------------------------------------------------
// THE FIXTURE, and a fake engine that actually holds a document.
// ---------------------------------------------------------------------------

const canvas: CanvasProjection = { width: 595276, height: 841890, orientation: 'portrait', preset: 'A4', locale: 'en', utcOffset: '+07:00', marginTop: 36000, marginRight: 36000, marginBottom: 36000, marginLeft: 36000, gridIncrement: 6000, commandWidth: 595276, commandHeight: 841890, fontFamilies: ['body'], fontChains: [{ name: 'body', entries: [{ face: 'Noto Sans', assetKey: '', family: '', style: '', bold: '', italic: '', boldItalic: '' }] }], defaultFontSize: 12000, defaultLineSpacing: 1000, contentWindowHeight: 729890, contentWindowCount: 1, contentWindowOrigins: [0], contentWindowPages: [0], contentWindowCountIsExact: true, bands: [{ name: 'pageHeader', x: 36000, y: 36000, width: 523276, height: 20000 }, { name: 'content', x: 36000, y: 56000, width: 523276, height: 729890 }, { name: 'pageFooter', x: 36000, y: 785890, width: 523276, height: 20000 }], components: [] }

type Component = CanvasProjection['components'][number]

// A rule as the engine actually creates one: `lineDropHeight` is 1pt and a
// created Line is given `Background: "#000000"` (`component_commands.go`).
const horizontalLine: Component = { id: 'e1', type: 'line', band: 'content', x: 0, y: 0, width: 72_000, height: 1_000, resizable: true, background: '#000000' }
const verticalLine: Component = { ...horizontalLine, width: 1_000, height: 72_000 }
const squareLine: Component = { ...horizontalLine, width: 10_000, height: 10_000 }
// A Line that CARRIES a border. `elementBoxDeclaration` is kind-agnostic, so
// this paints; the panel withholds the controls and discloses the fact.
const borderedLine: Component = { ...horizontalLine, borderWidth: 2_000, borderColor: '#c81e1e', borderEdges: ['bottom'] }
const rect: Component = { id: 'e1', type: 'rect', band: 'content', x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, background: '#1b2a4a', borderWidth: 1_000, borderColor: '#000000', borderEdges: ['top', 'right', 'bottom', 'left'] }
const text: Component = { id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello' }
const secondText: Component = { ...text, id: 'e9', y: 100_000 }

const wire = (payload: ArrayBuffer): string => new TextDecoder().decode(payload)

// THE FAKE HOLDS A DOCUMENT, and it has to: guardrail 5's assertion is a
// REVISION DELTA followed by an undo, and a fake that answers a fixed constant
// would let a two-command implementation pass while looking identical from the
// outside. So this one applies each command to its own component list, pushes
// the prior list onto a history stack, and pops it on `undo` — the smallest
// model that can tell one history entry from two.
//
// It is NOT the engine and makes no claim to be: it validates nothing, and it
// deliberately does no snapping, which is the one behaviour
// `updateComponentProperties` genuinely does not have.
function documentEngine(initial: ReadonlyArray<Component>, refuse?: (changes: Record<string, unknown>) => unknown) {
  const sent: ArrayBuffer[] = []
  const revisions: number[] = []
  let components: ReadonlyArray<Component> = initial
  const history: ReadonlyArray<Component>[] = []
  let revision = 1
  const snapshot = (): EngineSnapshot => ({ documentState: 'loaded', revision, byteLength: 3, canUndo: history.length > 0, canRedo: false, canvas: { ...canvas, components } })
  const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
    if (operation === 'command' && payload) {
      sent.push(payload)
      const command = JSON.parse(wire(payload)) as { ids: ReadonlyArray<string>; changes: Record<string, { op: string; value?: unknown }> }
      const refusal = refuse?.(command.changes)
      if (refusal) throw refusal
      history.push(components)
      components = components.map((component) => command.ids.includes(component.id) ? applyChanges(component, command.changes) : component)
      revision += 1
      revisions.push(revision)
    }
    if (operation === 'undo') { components = history.pop() ?? components; revision += 1 }
    return { snapshot: snapshot() }
  })
  return { sent, revisions, request, engine: { request } as unknown as EngineClient, initial: snapshot(), current: () => components }
}

// The projection carries MILLIPOINTS; the wire carries the author's own literal
// in POINTS, unquoted, because `numberLiteral` passes it through byte for byte.
// The x1000 lives here for the same reason it lives in Go: one side owns it.
function applyChanges(component: Component, changes: Record<string, { op: string; value?: unknown }>): Component {
  let next: Record<string, unknown> = { ...component }
  for (const [key, change] of Object.entries(changes)) {
    if (change.op === 'clear') { delete next[key]; continue }
    if (change.op === 'null') { next[key] = undefined; continue }
    next = { ...next, [key]: typeof change.value === 'number' ? Math.round(change.value * 1000) : change.value }
  }
  return next as Component
}

const select = (id: string, kind: string) => fireEvent.click(screen.getByLabelText(`${kind} component ${id}`))
// `unmount` is handed back so a test driving MORE THAN ONE gesture can tear the
// previous panel down. Testing-library cleans up between tests, not within one,
// and two live `App` instances share `document.body`: every `getByRole` then
// searches both, and the only reason that has not thrown is that the accessible
// names happen not to collide today. One relabel — which is this story's whole
// subject — turns that into "found multiple elements".
const open = (components: ReadonlyArray<Component>, refuse?: (changes: Record<string, unknown>) => unknown) => {
  const harness = documentEngine(components, refuse)
  const view = render(<App engine={harness.engine} initialSnapshot={harness.initial} />)
  return { ...harness, unmount: view.unmount }
}
const box = (name: string) => screen.getByRole('textbox', { name })
const missing = (name: string) => expect(screen.queryByRole('textbox', { name })).not.toBeInTheDocument()
const panel = () => screen.getByLabelText('Properties panel')
// ⚠ A SINGLE MICROTASK IS NOT A FLUSH, and "no command was sent" is exactly the
// assertion a weak flush passes vacuously. `await Promise.resolve()` drains one
// microtask: a dispatch armed on a TIMER (this panel debounces prose commits),
// one that needs a second `await`, or one at the end of an effect chain all slip
// past it and the test goes green on a defect. This waits a real macrotask turn
// LONGER THAN THE PANEL'S OWN LONGEST DEBOUNCE — read from the constant rather
// than written out, so it cannot drift under the debounce it is meant to
// outlast — and then drains the microtask queue several times over.
const settle = async () => {
  await new Promise((resolve) => setTimeout(resolve, PROSE_COMMIT_DEBOUNCE_MS + 20))
  for (let turn = 0; turn < 4; turn++) await Promise.resolve()
}
// THE VISIBLE WORD, NOT THE ACCESSIBLE NAME — and the distinction is this
// story's entire deliverable rather than a nicety. `FieldSpec.label` is the
// accessible name and is NEVER RENDERED; `affix` is the word a person actually
// reads, in `<span className="property-affix">`. A suite that only queries by
// accessible name stays green while every row on screen still says `W`, `H` and
// `Background` — measured, not hypothesised. So the affixes are swept out of
// the rendered panel, in order, and asserted as an exact list.
const visibleAffixes = () => Array.from(panel().querySelectorAll('.property-affix')).map((node) => (node.textContent ?? '').trim())

describe('a Line is authored as a thickness and a colour', () => {
  it('offers Length and Thickness on a HORIZONTAL rule, and never W or H', () => {
    open([horizontalLine])
    select('e1', 'line')
    // Length is the long axis, which for this box is `width`; Thickness is
    // `height`, the 1pt the engine calls the rule's thickness.
    expect(box('Length (pt)')).toHaveValue('72')
    expect(box('Thickness (pt)')).toHaveValue('1')
    missing('Width (pt)')
    missing('Height (pt)')
    expect(screen.getByRole('button', { name: 'Horizontal orientation' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Vertical orientation' })).toHaveAttribute('aria-pressed', 'false')
  })

  // ⚠ THE ASSERTION THAT ACTUALLY GUARDS THE STORY'S DELIVERABLE.
  //
  // Every other test in this file resolves controls by ACCESSIBLE NAME, which
  // is `FieldSpec.label` — a string this panel never renders. The word a person
  // reads is `affix`. MEASURED: reverting the Line's affixes to `W`/`H` and both
  // kinds' to `Background`, leaving the labels untouched, left the whole suite
  // green at 73 files / 1195 tests. The visible relabel — the entire point of
  // the story — was unverified. These sweep what is on screen.
  it('VISIBLY reads Length, Thickness and Colour on a Line, and never W, H or Background', () => {
    open([horizontalLine])
    select('e1', 'line')
    expect(visibleAffixes()).toEqual(['X', 'Y', 'Length', 'Thickness', 'Colour', 'Visibility'])
    // Spelled out as well as swept, because an exact list is easy to "fix" by
    // editing the list. These three words are the epic's complaint, by name.
    for (const engineWord of ['W', 'H', 'Background']) expect(visibleAffixes()).not.toContain(engineWord)
  })

  it('VISIBLY reads Fill on a Rectangle, and keeps W, H and the border rows', () => {
    open([rect])
    select('e1', 'rect')
    expect(visibleAffixes()).toEqual(['X', 'Y', 'W', 'H', 'Border', 'Border colour', 'Edges', 'Fill', 'Visibility'])
    expect(visibleAffixes()).not.toContain('Background')
  })

  // THE REGRESSION HALF. A text element is untouched by this story and must go
  // on reading exactly as it did — which is what makes the two lists above a
  // per-kind relabel rather than a global rename.
  it('leaves a text element VISIBLY reading W, H and Background', () => {
    open([text])
    select('e1', 'text')
    const affixes = visibleAffixes()
    expect(affixes).toContain('W')
    expect(affixes).toContain('H')
    expect(affixes).toContain('Background')
    for (const kindWord of ['Length', 'Thickness', 'Fill']) expect(affixes).not.toContain(kindWord)
    // `Colour` is deliberately NOT in that list. A text element's TYPOGRAPHY
    // section has spelled `style.color` as `Colour` since Story 10.1, and this
    // story does not move it. The two never share a panel — a Line renders no
    // TYPOGRAPHY section, because `typographic` is text-or-table — so a Line's
    // `Colour` row cannot be confused with the ink row beside it.
    expect(affixes).toContain('Colour')
  })

  // PATCH 3. `unit: 'pt'` is not decoration on these two rows: it alone gates
  // `inputMode="decimal"`, the visible `pt` suffix, and the WHOLE arrow-step
  // path (`step` returns early on `!numeric`). On a Line these are the only
  // geometry rows there are, so dropping it would silently make the rule's
  // length and thickness the two fields in the panel that cannot be nudged —
  // and, measured, nothing red.
  it('keeps Length and Thickness numeric: pt suffix, decimal input mode, and working arrow steps', async () => {
    const harness = open([horizontalLine])
    select('e1', 'line')
    for (const name of ['Length (pt)', 'Thickness (pt)']) {
      expect(box(name)).toHaveAttribute('inputmode', 'decimal')
      expect(box(name).parentElement).toHaveTextContent('pt')
    }
    const length = box('Length (pt)')
    expect(fireEvent.keyDown(length, { key: 'ArrowUp' })).toBe(false)
    expect(length).toHaveValue('73')
    await waitFor(() => expect(harness.sent).toHaveLength(1))
    expect(wire(harness.sent[0] as ArrayBuffer)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"width":{"op":"set","value":73}}}')
    const thickness = box('Thickness (pt)')
    fireEvent.keyDown(thickness, { key: 'ArrowUp' })
    expect(thickness).toHaveValue('2')
    await waitFor(() => expect(harness.sent).toHaveLength(2))
    expect(wire(harness.sent[1] as ArrayBuffer)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"height":{"op":"set","value":2}}}')
    // And DOWN, so a one-directional step cannot pass for a working one.
    fireEvent.keyDown(box('Thickness (pt)'), { key: 'ArrowDown' })
    await waitFor(() => expect(box('Thickness (pt)')).toHaveValue('1'))
  })

  it('maps Length onto height and Thickness onto width on a VERTICAL rule', () => {
    open([verticalLine])
    select('e1', 'line')
    expect(box('Length (pt)')).toHaveValue('72')
    expect(box('Thickness (pt)')).toHaveValue('1')
    expect(screen.getByRole('button', { name: 'Vertical orientation' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Horizontal orientation' })).toHaveAttribute('aria-pressed', 'false')
  })

  // THE TIE. A square "line" is degenerate and one of the two answers has to be
  // chosen; the rule is stated rather than discovered, and it reads horizontal.
  it('reads a SQUARE rule as horizontal, shows the committed values, and writes nothing', async () => {
    const harness = open([squareLine])
    select('e1', 'line')
    expect(screen.getByRole('button', { name: 'Horizontal orientation' })).toHaveAttribute('aria-pressed', 'true')
    expect(box('Length (pt)')).toHaveValue('10')
    expect(box('Thickness (pt)')).toHaveValue('10')
    await settle()
    expect(harness.sent).toHaveLength(0)
  })

  // AC4. Selecting an element is a READ. A panel narrowing its vocabulary is
  // never a document migration, and a derived label is never a write.
  it('dispatches ZERO commands for the act of selecting a Line', async () => {
    const harness = open([horizontalLine, secondText])
    select('e1', 'line')
    select('e9', 'text')
    select('e1', 'line')
    await settle()
    expect(harness.sent).toHaveLength(0)
    expect(harness.request.mock.calls.filter(([operation]) => operation === 'command')).toHaveLength(0)
  })

  it('withholds the border width, the border colour and the four edge checkboxes', () => {
    open([borderedLine])
    select('e1', 'line')
    missing('Border width (pt)')
    missing('Border colour')
    expect(screen.queryByLabelText('Pick Border colour')).not.toBeInTheDocument()
    for (const edge of ['top', 'right', 'bottom', 'left']) expect(screen.queryByRole('checkbox', { name: `Border ${edge}` })).not.toBeInTheDocument()
    expect(screen.queryByRole('group', { name: 'Border edges' })).not.toBeInTheDocument()
  })

  // D-14.2.Q1. Hiding costs DISCOVERABILITY, not preservation — `PropertyDraft`
  // writes nothing on mount, so a withheld control removes nothing. What it
  // does cost is that a border which PAINTS would otherwise vanish from the
  // author's view, so the panel says it is there.
  it('discloses a border the Line actually carries, and says nothing when it carries none', async () => {
    const harness = open([borderedLine])
    select('e1', 'line')
    const note = screen.getByText(/This line carries a border in the document/)
    expect(note).toBeInTheDocument()
    expect(note.textContent).toContain('The stored border is unchanged and still paints')
    // The disclosure is a READ. Nothing is written to remove or normalise the
    // border it names, and the projection still carries every part of it.
    await settle()
    expect(harness.sent).toHaveLength(0)
    expect(harness.current()[0]).toMatchObject({ borderWidth: 2_000, borderColor: '#c81e1e', borderEdges: ['bottom'] })
  })

  // PATCH 4. THE PREDICATE IS A THREE-WAY OR AND EACH ARM IS EXERCISED ALONE.
  //
  // Both Line fixtures in the repository set all three border keys at once, so
  // a fixture-driven test cannot tell `borderWidth || borderColor ||
  // borderEdges` from `borderWidth` alone — measured: collapsing the predicate
  // to its first arm left the suite green. The EDGES-ONLY case is the one that
  // matters most: Go's `borderPaints` returns true for any present, non-null,
  // non-empty edge set, so such a border PAINTS while a width-only predicate
  // would have the panel say nothing at all about it.
  it.each([
    ['a width alone', { borderWidth: 2_000 }],
    ['a colour alone', { borderColor: '#c81e1e' }],
    ['an edge set alone', { borderEdges: ['bottom' as const] }],
  ])('discloses a border declared as %s', (_case, border) => {
    open([{ ...horizontalLine, ...border }])
    select('e1', 'line')
    expect(screen.getByText(/This line carries a border in the document/)).toBeInTheDocument()
  })

  it('renders no disclosure for a Line with no projected border', () => {
    open([horizontalLine])
    select('e1', 'line')
    expect(screen.queryByText(/This line carries a border in the document/)).not.toBeInTheDocument()
    // The negative half has to be non-vacuous too: this fixture differs from
    // the three above ONLY in carrying no border key.
    expect(box('Colour')).toBeInTheDocument()
  })

  // PATCH 10. The sentence belongs where the controls it is about were
  // withheld, not adrift between the Visibility row and Visibility's own note.
  it('places the disclosure where the border controls would have been', () => {
    open([borderedLine])
    select('e1', 'line')
    const notes = Array.from(panel().querySelectorAll('.property-section-box .honest-note')).map((node) => node.textContent ?? '')
    expect(notes[0]).toContain('This line carries a border in the document')
    expect(notes[1]).toContain('Visibility takes a boolean or null formula')
  })

  it('spells the fill Colour on a Line, and keeps the null action under the new name', () => {
    open([horizontalLine])
    select('e1', 'line')
    expect(box('Colour')).toHaveValue('#000000')
    expect(screen.getByLabelText('Pick Colour')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Set Colour null' })).toBeInTheDocument()
    // The generic word is GONE for this kind, which is the whole complaint.
    missing('Background')
    expect(screen.queryByLabelText('Pick Background')).not.toBeInTheDocument()
  })
})

describe('a Rectangle is a fill and a border', () => {
  it('spells its background Fill and leaves every border control standing', () => {
    open([rect])
    select('e1', 'rect')
    expect(box('Fill')).toHaveValue('#1b2a4a')
    expect(screen.getByLabelText('Pick Fill')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Set Fill null' })).toBeInTheDocument()
    missing('Background')
    // A Rectangle IS a box, so its box controls are its own vocabulary.
    expect(box('Border width (pt)')).toBeInTheDocument()
    expect(box('Border colour')).toBeInTheDocument()
    for (const edge of ['top', 'right', 'bottom', 'left']) expect(screen.getByRole('checkbox', { name: `Border ${edge}` })).toBeChecked()
    // And its size is still a width and a height: only a Line is a rule.
    expect(box('Width (pt)')).toHaveValue('72')
    expect(box('Height (pt)')).toHaveValue('24')
    expect(screen.queryByRole('group', { name: 'Orientation' })).not.toBeInTheDocument()
  })
})

describe('the kind-specific vocabulary is gated on a single selection', () => {
  it('keeps generic dimensions and withholds borders on a multi-selection carrying a Line', () => {
    open([text, { ...horizontalLine, id: 'e2', y: 100_000 }])
    select('e1', 'text')
    fireEvent.click(screen.getByLabelText('line component e2'), { shiftKey: true })
    expect(box('Width (pt)')).toBeInTheDocument()
    expect(box('Height (pt)')).toBeInTheDocument()
    expect(box('Background')).toBeInTheDocument()
    missing('Border width (pt)')
    expect(screen.queryByRole('checkbox', { name: 'Border top' })).not.toBeInTheDocument()
    missing('Length (pt)')
    missing('Thickness (pt)')
    expect(screen.queryByRole('group', { name: 'Orientation' })).not.toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// THE BYTE PROOF (D-14.2.Q6).
// ---------------------------------------------------------------------------
//
// WHAT THIS PROVES: that the same gesture emits the same COMMAND BYTES it
// emitted before this story. WHAT IT DOES NOT PROVE: anything about the
// serialized `.folio` document directly. A vitest/jsdom test cannot obtain real
// serialized bytes — the save path is `engine.request('serialize')`, answered by
// the wasm engine, whose sole instantiation in production is `engine.worker.ts`
// and whose soleness `engine-ownership-contract.test.ts` enforces; every
// designer unit test injects a fake `EngineClient`. A test that appeared to
// compare document bytes would be comparing a fake's own constant to itself.
//
// The inference that closes the gap is that this story changes NO Go code:
// same gesture -> same command bytes -> same document. The middle term is the
// only one observable here and the only one this story is responsible for.
//
// THE RESIDUAL, NAMED: the real serialize path is exercised only by
// `e2e/browser-native-roundtrip.spec.ts`, which this story COMPILES and DOES
// NOT RUN.
describe('the relabelled controls emit the bytes their old names emitted', () => {
  // Both renders put the component at `e1`, so the two commands are comparable
  // BYTE FOR BYTE rather than modulo an id.
  const gesture = async (component: Component, drive: () => void) => {
    const harness = open([component])
    select('e1', component.type)
    drive()
    await waitFor(() => expect(harness.sent.length).toBeGreaterThan(0))
    const bytes = wire(harness.sent[0] as ArrayBuffer)
    // TORN DOWN BEFORE THE NEXT GESTURE. Two live panels in one body make every
    // query ambiguous, and the ambiguity is invisible until two rows happen to
    // share a name — in the one story that renames rows.
    harness.unmount()
    return bytes
  }

  it('sends Thickness as height, byte-identical to what the H field sent', async () => {
    const thickness = await gesture(horizontalLine, () => {
      const field = box('Thickness (pt)')
      fireEvent.change(field, { target: { value: '2' } })
      fireEvent.blur(field)
    })
    expect(thickness).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"height":{"op":"set","value":2}}}')
    const height = await gesture(text, () => {
      const field = box('Height (pt)')
      fireEvent.change(field, { target: { value: '2' } })
      fireEvent.blur(field)
    })
    expect(thickness).toBe(height)
  })

  it('sends Length as width, byte-identical to what the W field sent', async () => {
    const length = await gesture(horizontalLine, () => {
      const field = box('Length (pt)')
      fireEvent.change(field, { target: { value: '120' } })
      fireEvent.blur(field)
    })
    expect(length).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"width":{"op":"set","value":120}}}')
    const width = await gesture(text, () => {
      const field = box('Width (pt)')
      fireEvent.change(field, { target: { value: '120' } })
      fireEvent.blur(field)
    })
    expect(length).toBe(width)
  })

  // DW-146: `style.color` on a line or rect loads, serializes and round-trips
  // and is NEVER painted, never validated, never projected — the obvious-looking
  // field is the inert one. Colour maps to `background`, and this is where that
  // is nailed down rather than left to a comment.
  it('sends a Line\'s Colour as background, and a Rectangle\'s Fill as background', async () => {
    const colour = await gesture(horizontalLine, () => fireEvent.change(screen.getByLabelText('Pick Colour'), { target: { value: '#c81e1e' } }))
    expect(colour).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"background":{"op":"set","value":"#c81e1e"}}}')
    const fill = await gesture(rect, () => fireEvent.change(screen.getByLabelText('Pick Fill'), { target: { value: '#c81e1e' } }))
    expect(fill).toBe(colour)
    const background = await gesture(text, () => fireEvent.change(screen.getByLabelText('Pick Background'), { target: { value: '#c81e1e' } }))
    expect(colour).toBe(background)
    expect(colour).not.toContain('"color"')
  })

  it('sends the null action under its new name to the same field', async () => {
    const nulled = await gesture(horizontalLine, () => fireEvent.click(screen.getByRole('button', { name: 'Set Colour null' })))
    expect(nulled).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"background":{"op":"null"}}}')
    const background = await gesture(text, () => fireEvent.click(screen.getByRole('button', { name: 'Set Background null' })))
    expect(nulled).toBe(background)
  })
})

// ---------------------------------------------------------------------------
// THE ORIENTATION CONTROL — one command, one history entry, no snapping.
// ---------------------------------------------------------------------------

describe('toggling a rule\'s orientation', () => {
  it('is ONE command carrying both dimensions, and ONE revision', async () => {
    const harness = open([horizontalLine])
    select('e1', 'line')
    const before = harness.initial.revision
    fireEvent.click(screen.getByRole('button', { name: 'Vertical orientation' }))
    await waitFor(() => expect(harness.sent).toHaveLength(1))
    expect(wire(harness.sent[0] as ArrayBuffer)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"width":{"op":"set","value":1},"height":{"op":"set","value":72}}}')
    // THE REVISION DELTA IS THE ASSERTION, not the shape. Asserting only that
    // the box swapped would pass on a two-command implementation, which is
    // exactly what this control must not be.
    expect(harness.revisions).toEqual([before + 1])
    expect(harness.current()[0]).toMatchObject({ width: 1_000, height: 72_000 })
    // And the two rows keep their values under the new orientation: the rule is
    // still 72pt long and 1pt thick, read off the other axis.
    await waitFor(() => expect(box('Length (pt)')).toHaveValue('72'))
    expect(box('Thickness (pt)')).toHaveValue('1')
    expect(screen.getByRole('button', { name: 'Vertical orientation' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('is undone by ONE undo, which returns both dimensions together', async () => {
    const harness = open([horizontalLine])
    select('e1', 'line')
    fireEvent.click(screen.getByRole('button', { name: 'Vertical orientation' }))
    await waitFor(() => expect(harness.current()[0]).toMatchObject({ width: 1_000, height: 72_000 }))
    const undo = screen.getByRole('button', { name: 'Undo' })
    await waitFor(() => expect(undo).toBeEnabled())
    fireEvent.click(undo)
    // BOTH, from ONE undo. On a two-command implementation this would restore
    // only the second of them and the rule would come back 1pt x 1pt.
    await waitFor(() => expect(harness.current()[0]).toMatchObject({ width: 72_000, height: 1_000 }))
    // Undo clears the selection (the panel is keyed on the document
    // generation), so the rule is re-selected to read the panel back: it is
    // horizontal again, 72pt long and 1pt thick, from the one undo.
    select('e1', 'line')
    expect(screen.getByRole('button', { name: 'Horizontal orientation' })).toHaveAttribute('aria-pressed', 'true')
    expect(box('Length (pt)')).toHaveValue('72')
    expect(box('Thickness (pt)')).toHaveValue('1')
  })

  it('sends nothing when the orientation it already reads is pressed again', async () => {
    const harness = open([horizontalLine])
    select('e1', 'line')
    fireEvent.click(screen.getByRole('button', { name: 'Horizontal orientation' }))
    await settle()
    // Unlike `SegmentedProperty`, pressing the active segment does not clear:
    // there is no orientation property in the document to clear, and inventing
    // one would be the stored state AC1 forbids.
    expect(harness.sent).toHaveLength(0)
  })

  // D-14.2.Q7. A SQUARE RULE HAS NO ORIENTATION TO CHANGE.
  //
  // Swapping the dimensions of a square is the identity. The command would
  // carry the numbers the document already holds, the engine's byte-equality
  // short-circuit would decline to commit it at all — no revision, no undo
  // entry, no dirty flag — and the author would be left pressing a live control
  // whose silence is its whole answer. So the segment is disabled and says why.
  it('disables the other orientation on a square rule, and states the reason', async () => {
    const harness = open([squareLine])
    select('e1', 'line')
    const vertical = screen.getByRole('button', { name: 'Vertical orientation' })
    expect(vertical).toBeDisabled()
    // The reason is BESIDE the control and programmatically attached to it —
    // never a bare grey-out, which tells a screen reader nothing at all.
    expect(screen.getByText('A square rule has no orientation to change.')).toBeInTheDocument()
    expect(vertical).toHaveAccessibleDescription('A square rule has no orientation to change.')
    // The segment that reads TRUE stays pressed and is not disabled: it is
    // reporting the shape, not offering a change.
    expect(screen.getByRole('button', { name: 'Horizontal orientation' })).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(vertical)
    await settle()
    expect(harness.sent).toHaveLength(0)
  })

  // PATCH 6. THE CONTROL IS NEVER DISABLED WHILE ITS OWN COMMAND IS IN FLIGHT.
  //
  // Disabling a FOCUSED control moves focus to `<body>` and does not give it
  // back when the control is re-enabled (measured in Chromium 1217; the same
  // fact is why `PropertyDraft`'s arrow step passes `disable = false`). A toggle
  // is pressed BY the focused element, so a `disabled={pending}` here would
  // throw the author's focus away on every orientation change. jsdom does not
  // implement blur-on-disable, so this pins the MECHANISM rather than the focus:
  // the click's handler runs synchronously to its first await, so a `pending`
  // state would already have re-rendered the button disabled by this line.
  it('never disables itself while its command is in flight', async () => {
    const harness = open([horizontalLine])
    select('e1', 'line')
    const vertical = screen.getByRole('button', { name: 'Vertical orientation' })
    fireEvent.click(vertical)
    expect(vertical).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Horizontal orientation' })).toBeEnabled()
    await waitFor(() => expect(harness.sent).toHaveLength(1))
  })

  it('offers the reason on a non-square rule to nobody', () => {
    open([horizontalLine])
    select('e1', 'line')
    expect(screen.getByRole('button', { name: 'Vertical orientation' })).toBeEnabled()
    expect(screen.queryByText('A square rule has no orientation to change.')).not.toBeInTheDocument()
  })

  // The matrix's "Orientation refused" row. `containComponent` runs once per id
  // AFTER every change is applied, so what it refuses is the FINAL shape, and
  // its diagnostic is `component.geometry` — accurate for a width/height pair,
  // and therefore PRINTED. This is the refusal an author will actually hit.
  it('anchors a refusal on both dimensions and beside the control, and PRINTS an accurate path', async () => {
    const refusal = Object.assign(new Error('component must stay inside its band'), { code: 'COMPONENT_INVALID', elementId: 'e1', dataPath: 'component.geometry' })
    open([horizontalLine], (changes) => 'width' in changes && 'height' in changes ? refusal : undefined)
    select('e1', 'line')
    fireEvent.click(screen.getByRole('button', { name: 'Vertical orientation' }))
    await waitFor(() => expect(screen.getAllByRole('alert').length).toBeGreaterThan(0))
    const alerts = screen.getAllByRole('alert')
    // GUARDRAIL 2, AND ITS VACUOUS CASE. The refusal is anchored by the fields
    // the INTENT carried, so it resolves for `width` and for `height` — the
    // Length row, the Thickness row and the control itself. A refusal matching
    // NEITHER cannot happen: the set is built from the intent, the intent is
    // non-empty, and both of its members are rendered. Three alerts is that
    // fact counted; zero would be a refusal the author never sees.
    expect(alerts).toHaveLength(3)
    for (const alert of alerts) {
      expect(alert.textContent).toContain('component must stay inside its band')
      expect(alert.textContent).toContain('e1: ')
    }
    // Two of the three sit on the rows the intent moved, addressed by the field
    // rather than by the label — which is why a relabel never breaks routing.
    expect(document.getElementById('property-error-width')).not.toBeNull()
    expect(document.getElementById('property-error-height')).not.toBeNull()
    // The orientation control's own alert is wired to its buttons, not merely
    // placed beside them: a screen reader reaching the control is told it is
    // invalid and where the message is.
    const anchored = document.getElementById('property-error-orientation')
    expect(anchored).not.toBeNull()
    for (const segment of ['Horizontal orientation', 'Vertical orientation']) {
      expect(screen.getByRole('button', { name: segment })).toHaveAttribute('aria-invalid', 'true')
      expect(screen.getByRole('button', { name: segment })).toHaveAttribute('aria-errormessage', 'property-error-orientation')
    }
    // D-14.2.Q2b AS AMENDED: `component.geometry` IS PRINTED. `geometry` is not
    // a member of `PropertyField`, so it cannot be the mislabel the rule exists
    // to withhold — and it is the true location of a containment refusal. The
    // rule is "never print a field name that may be wrong", not "never print
    // anything"; discarding an accurate location would have thrown away the one
    // diagnostic this control actually produces.
    for (const alert of alerts) expect(alert.textContent).toContain('component.geometry')
  })

  // THE COMPLEMENT OF THE ROW ABOVE, so the pair pins BOTH directions of the
  // membership test rather than one. `propertyPath`'s fallback when it
  // recognises no key at all is the literal `changes`, which names no specific
  // field and therefore cannot mislabel one. Uninformative is not the same as
  // wrong, and the rule suppresses only what is wrong.
  it('prints a multi-field path that names no field at all', async () => {
    const refusal = Object.assign(new Error('component changes must be a non-empty object'), { code: 'COMPONENT_INVALID', elementId: 'e1', dataPath: 'component.changes' })
    open([horizontalLine], (changes) => 'width' in changes && 'height' in changes ? refusal : undefined)
    select('e1', 'line')
    fireEvent.click(screen.getByRole('button', { name: 'Vertical orientation' }))
    await waitFor(() => expect(screen.getAllByRole('alert').length).toBeGreaterThan(0))
    for (const alert of screen.getAllByRole('alert')) expect(alert.textContent).toBe('e1: component.changes: component changes must be a non-empty object')
  })

  // D-14.2.Q2b AT ITS SHARPEST — THE MISLABEL ITSELF, not merely a path the
  // panel happens to withhold.
  //
  // Go's `propertyPath` walks its canonical order and answers with the FIRST
  // key present in `changes`. On `{width, height}` that is always `width`, even
  // when `height` is what `applyPropertyChanges` refused — so the engine hands
  // the panel `component.width` for a failure of `height`. Printing it would be
  // a FALSE STATEMENT about which field was refused, in the epic whose subject
  // is the panel telling the truth. Declining to print a path known to be
  // unreliable is strictly more truthful than printing it.
  //
  // The correct repair is in Go and is an engine change this story does not
  // carry: registered as DW-333. Do not fix `propertyPath` here.
  it('withholds the MISLABEL: a height-caused refusal on {width, height} never prints component.width', async () => {
    const refusal = Object.assign(new Error('height must be a positive length'), { code: 'COMPONENT_INVALID', elementId: 'e1', dataPath: 'component.width' })
    open([horizontalLine], (changes) => 'width' in changes && 'height' in changes ? refusal : undefined)
    select('e1', 'line')
    fireEvent.click(screen.getByRole('button', { name: 'Vertical orientation' }))
    await waitFor(() => expect(screen.getAllByRole('alert').length).toBeGreaterThan(0))
    for (const alert of screen.getAllByRole('alert')) {
      expect(alert.textContent).toBe('e1: height must be a positive length')
      expect(alert.textContent).not.toContain('component.width')
    }
  })

  it('still prints the field path for a SINGLE-field refusal, exactly as before this story', async () => {
    const refusal = Object.assign(new Error('width must be positive'), { code: 'COMPONENT_INVALID', elementId: 'e1', dataPath: 'component.width' })
    open([horizontalLine], (changes) => Object.keys(changes).length === 1 ? refusal : undefined)
    select('e1', 'line')
    const field = box('Length (pt)')
    fireEvent.change(field, { target: { value: '0' } })
    fireEvent.blur(field)
    await waitFor(() => expect(screen.getAllByRole('alert')).toHaveLength(1))
    // The path the engine returns for a one-key `changes` object IS the key
    // that failed, so there is nothing unreliable to withhold.
    expect(screen.getAllByRole('alert')[0]?.textContent).toBe('e1: component.width: width must be positive')
  })
})

// ---------------------------------------------------------------------------
// THE DERIVED LABEL, AND WHY IT IS ALLOWED TO MOVE.
// ---------------------------------------------------------------------------

describe('a derived label is a function of committed state', () => {
  // ⚠ READ THIS BEFORE "FIXING" THE BEHAVIOUR BELOW INTO A LATCH.
  //
  // Orientation is DERIVED from the committed box because the alternative is
  // stored state, and a stored orientation is a new serialized key — which AC1
  // forbids and which would move the `.folio` format for a label. So when the
  // author sets Thickness above Length the shape genuinely becomes taller than
  // it is wide, the derivation re-reads as vertical, and the two labels swap
  // over the two values. The commit SUCCEEDS; nothing is refused, nothing is
  // rewritten, and no second command is sent.
  //
  // This is accepted and stated rather than prevented. The flip happens on
  // COMMIT, not per keystroke: the draft is local until blur or Enter, so no
  // value moves under the cursor mid-typing.
  it('swaps Length and Thickness over the two values when Thickness is committed above Length, with no second command', async () => {
    const harness = open([horizontalLine])
    select('e1', 'line')
    const thickness = box('Thickness (pt)')
    fireEvent.change(thickness, { target: { value: '80' } })
    fireEvent.blur(thickness)
    await waitFor(() => expect(harness.sent).toHaveLength(1))
    // ONE command, and it is the ordinary single-field `height` set — the same
    // bytes the `H` field sent. The relabel costs no extra traffic.
    expect(wire(harness.sent[0] as ArrayBuffer)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"height":{"op":"set","value":80}}}')
    // The box is now 72 x 80, so it reads as a vertical rule and the words move
    // to match: Length is the 80 that was just typed as Thickness.
    await waitFor(() => expect(screen.getByRole('button', { name: 'Vertical orientation' })).toHaveAttribute('aria-pressed', 'true'))
    expect(box('Length (pt)')).toHaveValue('80')
    expect(box('Thickness (pt)')).toHaveValue('72')
  })
})

// ---------------------------------------------------------------------------
// UX-DR25 — no control loses its accessible name.
// ---------------------------------------------------------------------------

describe('every renamed, added and withheld control keeps an unambiguous name', () => {
  it('names every control the Line panel renders', () => {
    open([borderedLine])
    select('e1', 'line')
    const panel = within(screen.getByLabelText('Properties panel'))
    for (const name of ['X (pt)', 'Y (pt)', 'Length (pt)', 'Thickness (pt)', 'Colour', 'Visible if']) expect(panel.getByRole('textbox', { name })).toBeInTheDocument()
    for (const name of ['Horizontal orientation', 'Vertical orientation', 'Set Colour null']) expect(panel.getByRole('button', { name })).toBeInTheDocument()
    expect(panel.getByRole('group', { name: 'Orientation' })).toBeInTheDocument()
    expect(panel.getByLabelText('Pick Colour')).toBeInTheDocument()
    // Not one control in the panel is left unnamed — asserted over what the
    // render produced rather than over the list above, which cannot see a
    // control nobody thought to name.
    for (const control of Array.from(screen.getByLabelText('Properties panel').querySelectorAll('.property-editor button, .property-editor input'))) expect(control).toHaveAccessibleName()
  })

  it('names every control the Rectangle panel renders', () => {
    open([rect])
    select('e1', 'rect')
    const panel = within(screen.getByLabelText('Properties panel'))
    for (const name of ['Width (pt)', 'Height (pt)', 'Fill', 'Border width (pt)', 'Border colour']) expect(panel.getByRole('textbox', { name })).toBeInTheDocument()
    expect(panel.getByRole('button', { name: 'Set Fill null' })).toBeInTheDocument()
    expect(panel.getByLabelText('Pick Fill')).toBeInTheDocument()
    for (const control of Array.from(screen.getByLabelText('Properties panel').querySelectorAll('.property-editor button, .property-editor input'))) expect(control).toHaveAccessibleName()
  })
})


describe('line canvas resizing preserves Thickness', () => {
  it.each([
    ['horizontal', horizontalLine, ['w', 'e']],
    ['vertical', verticalLine, ['n', 's']],
    ['square', squareLine, ['w', 'e']],
  ] as const)('offers only length handles for a %s line', (_name, line, anchors) => {
    open([line])
    select('e1', 'line')
    const chrome = document.querySelector('.canvas-selection-chrome')!
    expect(Array.from(chrome.querySelectorAll('.selection-handle')).map((handle) => handle.className)).toEqual(anchors.map((anchor) => `selection-handle selection-handle-${anchor}`))
    expect(screen.queryByRole('button', { name: 'Resize e1' })).not.toBeInTheDocument()
  })

  it.each([
    ['e', horizontalLine, 8, 30, 80, 1, 0, 0],
    ['w', { ...horizontalLine, x: 20000 }, 8, 30, 64, 1, 28, 0],
    ['s', verticalLine, 30, 8, 1, 80, 0, 0],
    ['n', { ...verticalLine, y: 20000 }, 30, 8, 1, 64, 0, 28],
    ['e', horizontalLine, -100, 30, 1, 1, 0, 0],
    ['s', verticalLine, 30, -100, 1, 1.001, 0, 0],
  ] as const)('keeps thickness in the preview and committed %s drag, even diagonally', async (anchor, line, dx, dy, width, height, x, y) => {
    const snapshot: EngineSnapshot = { documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: [line] } }
    const request = vi.fn(async (_operation: string, payload: ArrayBuffer) => {
      const bounds = JSON.parse(wire(payload)) as { x: number; y: number; width: number; height: number }
      return { snapshot: { ...snapshot, revision: 2, canvas: { ...canvas, components: [{ ...line, x: Math.round(bounds.x * 1000), y: Math.round(bounds.y * 1000), width: Math.round(bounds.width * 1000), height: Math.round(bounds.height * 1000) }] } } }
    })
    render(<App engine={{ request } as unknown as EngineClient} initialSnapshot={snapshot} />)
    select('e1', 'line')
    const handle = document.querySelector(`.canvas-selection-chrome .selection-handle-${anchor}`)!
    fireEvent.pointerDown(handle, { pointerId: 1, clientX: 0, clientY: 0 })
    fireEvent.pointerMove(handle, { pointerId: 1, clientX: dx, clientY: dy })
    expect(box('Thickness (pt)')).toHaveValue('1')
    expect(box('Length (pt)')).toHaveValue(String(Math.max(width, height)))
    fireEvent.pointerUp(handle, { pointerId: 1, clientX: dx, clientY: dy })
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    const payload = (request.mock.calls[0] as unknown as [string, ArrayBuffer])[1]
    // Snap stays enabled on the wire. Go preserves the line's short axis;
    // this UI fake deliberately only checks transport and geometry handoff.
    expect(JSON.parse(wire(payload))).toEqual({ kind: 'setComponentBounds', version: 1, id: 'e1', x, y, width, height, snap: true })
    await waitFor(() => expect(box('Length (pt)')).not.toHaveAttribute('readonly'))
    expect(box('Length (pt)')).toHaveValue(String(Math.max(width, height)))
    expect(box('Thickness (pt)')).toHaveValue('1')
  })

  it('honours Snap off for line endpoint resizing', async () => {
    const snapshot: EngineSnapshot = { documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: [horizontalLine] } }
    const request = vi.fn(async (_operation: string, _payload: ArrayBuffer) => ({ snapshot }))
    render(<App engine={{ request } as unknown as EngineClient} initialSnapshot={snapshot} />)
    fireEvent.click(screen.getByRole('button', { name: 'Snap on' }))
    select('e1', 'line')
    const handle = screen.getByRole('button', { name: 'Resize e1 end' })
    fireEvent.pointerDown(handle, { pointerId: 1, clientX: 0, clientY: 0 })
    fireEvent.pointerMove(handle, { pointerId: 1, clientX: 8.422, clientY: 30 })
    fireEvent.pointerUp(handle, { pointerId: 1, clientX: 8.422, clientY: 30 })
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(JSON.parse(wire(request.mock.calls[0][1]))).toEqual({ kind: 'setComponentBounds', version: 1, id: 'e1', x: 0, y: 0, width: 80.422, height: 1, snap: false })
  })

  it('does not commit a cross-axis-only drag', () => {
    const { request } = open([horizontalLine])
    select('e1', 'line')
    const handle = document.querySelector('.canvas-selection-chrome .selection-handle-e')!
    fireEvent.pointerDown(handle, { pointerId: 1, clientX: 0, clientY: 0 })
    fireEvent.pointerMove(handle, { pointerId: 1, clientX: 0, clientY: 30 })
    fireEvent.pointerUp(handle, { pointerId: 1, clientX: 0, clientY: 30 })
    expect(request).not.toHaveBeenCalled()
  })

  it('keeps all eight resize handles on rectangles', () => {
    open([rect])
    select('e1', 'rect')
    expect(document.querySelectorAll('.canvas-selection-chrome .selection-handle, .canvas-selection-chrome .resize-handle')).toHaveLength(8)
  })
})
