import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import App, { tableSampleCandidates } from './App'
import { DataPanel } from './DataPanel'
import { acceptSampleData } from './sample-data'
import type { CanvasProjection, CanvasTableColumn, EngineSnapshot } from './engine-protocol'
import type { EngineClient } from './engine-client'
import { startBlankFromNew } from './test/new-document'

// STORY 14.10 — A TABLE COLUMN IS BOUND FROM THE MAIN WINDOW.
//
// Every row of the story's I/O & Edge-Case Matrix, plus AC7's required guard.
//
// ⚠ WHAT THIS FILE CANNOT PROVE, STATED HERE RATHER THAN IMPLIED BY ITS
// GREENNESS. jsdom does not implement `pointer-events` hit testing:
// `fireEvent.click(span)` makes the span the event target whatever App.css says,
// so every row below passes with `pointer-events: auto` entirely absent and the
// product completely inert. The source-text pin at the foot of this file is the
// SECOND cover — it proves the rule is in the stylesheet, not that a click
// resolves to the column — and `e2e/table-column-binding.spec.ts` is the first.
// Neither alone is sufficient and the story requires both (Q5).

const appCss = fs.readFileSync(path.join(path.dirname(fileURLToPath(import.meta.url)), 'App.css'), 'utf8')

const canvas: CanvasProjection = { width: 595276, height: 841890, orientation: 'portrait', preset: 'A4', locale: 'en', utcOffset: '+07:00', embedFonts: true, marginTop: 36000, marginRight: 36000, marginBottom: 36000, marginLeft: 36000, gridIncrement: 6000, commandWidth: 595276, commandHeight: 841890, fontFamilies: ['body'], fontChains: [{ name: 'body', entries: [{ face: 'Noto Sans', assetKey: '', family: '', style: '', bold: '', italic: '', boldItalic: '' }] }], defaultFontSize: 12000, defaultLineSpacing: 1000, contentWindowHeight: 729890, contentWindowCount: 1, contentWindowOrigins: [0], contentWindowPages: [0], contentWindowCountIsExact: true, bands: [{ name: 'pageHeader', x: 36000, y: 36000, width: 523276, height: 20000 }, { name: 'content', x: 36000, y: 56000, width: 523276, height: 729890 }, { name: 'pageFooter', x: 36000, y: 785890, width: 523276, height: 20000 }], components: [] }

const column = (id: string, label: string, bind: string): CanvasTableColumn => ({ id, label, labelLines: label === '' ? [] : [label], width: 60_000, headerAlign: 'left', cellAlign: 'left', bind })
const DATE = column('e10', 'Date', '{{row.date}}')
const AMOUNT = column('e11', 'Amount', '')
const table = (columns: ReadonlyArray<CanvasTableColumn> | null = [DATE, AMOUNT], bind = 'transactions[]') => ({ id: 'e7', type: 'table' as const, band: 'content' as const, x: 0, y: 0, width: 120_000, height: 16_000, resizable: false, ...(bind === '' ? {} : { tableBind: bind }), ...(columns === null ? {} : { columns: [...columns] }) })
const other = { id: 'e2', type: 'text' as const, band: 'content' as const, x: 0, y: 200_000, width: 60_000, height: 12_000, resizable: true }

// The fixture sample: one collection with two row fields, one document-scope
// scalar outside it, and a nested object inside the row so the walk's
// dotted-path arm is exercised rather than assumed.
const SAMPLE_JSON = '{"transactions":[{"date":"01 Jul","debit":12,"who":{"name":"Ada"}}],"total":5}'
const sample = () => acceptSampleData('c.json', new TextEncoder().encode(SAMPLE_JSON).buffer)

type Sent = { commands: string[] }
const engineFor = (sent: Sent, revision = 1, refuse?: string) => {
  const snapshotOf = (rev: number, components: ReadonlyArray<CanvasProjection['components'][number]>) => ({ documentState: 'loaded' as const, revision: rev, byteLength: 3, canvas: { ...canvas, components: [...components] } })
  let current = revision
  let components: ReadonlyArray<CanvasProjection['components'][number]> = [table(), other]
  return {
    request: vi.fn(async (operation: string, payload?: ArrayBuffer) => {
      if (operation === 'command' && payload) {
        const text = new TextDecoder().decode(new Uint8Array(payload))
        sent.commands.push(text)
        if (refuse) throw { elementId: 'e7', message: refuse }
        current++
      }
      return { snapshot: snapshotOf(current, components) }
    }),
    setComponents: (next: ReadonlyArray<CanvasProjection['components'][number]>) => { components = next },
  }
}
const mount = (components: ReadonlyArray<CanvasProjection['components'][number]>, engine: { request: unknown }, withSample = true) =>
  render(<App engine={engine as unknown as EngineClient} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: [...components] } }} {...(withSample ? { initialSampleData: sample() } : {})} />)

const homeOf = (container: Element, componentId: string) => container.querySelector(`.canvas-component:not(.canvas-component-echo)[data-component-id="${componentId}"]`) as HTMLElement
const home = (container: Element) => homeOf(container, 'e7')
const columnSpans = (container: Element, id: string, componentId = 'e7') => Array.from(homeOf(container, componentId).querySelectorAll(`[data-column-id="${id}"]`))
const selectedColumnIds = (container: Element) => Array.from(container.querySelectorAll('.canvas-table-column-selected')).map((node) => node.getAttribute('data-column-id'))
const clickColumn = (container: Element, id: string, init: Parameters<typeof fireEvent.click>[1] = {}) => fireEvent.click(columnSpans(container, id)[0]!, init)
const openData = () => fireEvent.click(screen.getByRole('tab', { name: 'DATA' }))

// The tree opens with only its root expanded, so a walk to a row field has to
// open the collection and its item first. Expansion is the tree's own gesture
// (a click on a branch treeitem), not something this helper simulates around.
const treeRow = (label: string) => screen.getAllByRole('treeitem').find((node) => within(node).queryByText(label) !== null)
const expand = (label: string) => { const row = treeRow(label); expect(row, `tree row ${label}`).toBeTruthy(); fireEvent.click(row!) }
const openRowFields = () => { expand('transactions[]'); expand('item 1') }

describe('a whole table is bound from the main window', () => {
  const snapshotOf = (revision: number, collection = 'items[]'): EngineSnapshot => ({ documentState: 'loaded', revision, byteLength: 3, canvas: { ...canvas, components: [table([DATE, AMOUNT], collection), other] } })
  const chooseTable = (container: Element) => { fireEvent.click(home(container).querySelector('.canvas-table-chip') as HTMLElement); openData() }

  it('sends one collection command, paints the returned collection, and then offers only its column fields', async () => {
    const sent: Sent = { commands: [] }
    const engine = engineFor(sent)
    const view = mount([table([DATE, AMOUNT], 'items[]'), other], engine)
    chooseTable(view.container)
    expect(screen.getByText('Current engine binding:')).toHaveTextContent('items[]')
    fireEvent.click(treeRow('transactions[]')!)
    await waitFor(() => expect(sent.commands).toHaveLength(1))
    expect(sent.commands[0]).toBe('{"kind":"bindTableCollection","version":1,"id":"e7","segments":["transactions"]}')
    await waitFor(() => expect(home(view.container).querySelector('.canvas-table-collection')).toHaveTextContent('transactions[]'))
    expect(screen.getByText('Current engine binding:')).toHaveTextContent('transactions[]')
    expect(columnSpans(view.container, 'e10').at(-1)).toHaveTextContent('{{row.date}}')
    clickColumn(view.container, 'e11')
    expect(screen.getByText('Column Amount selected · binding to a row field of transactions[]')).toBeInTheDocument()
    expect(treeRow('transactions[]')).toHaveAttribute('aria-expanded', 'true')
    expand('item 1')
    fireEvent.click(treeRow('total')!)
    expect(sent.commands).toHaveLength(1)
    fireEvent.click(treeRow('debit')!)
    await waitFor(() => expect(sent.commands).toHaveLength(2))
    expect(sent.commands[1]).toBe('{"kind":"updateTableColumnBinding","version":1,"id":"e7","columnId":"e11","field":"debit"}')
  })

  it('shares the pending-binding latch with column and scalar picks and installs a committed result after reselection', async () => {
    let resolve!: (value: { snapshot: EngineSnapshot }) => void
    const commands: string[] = []
    const request = vi.fn((operation: string, payload?: ArrayBuffer) => {
      if (operation === 'command') {
        commands.push(new TextDecoder().decode(payload))
        return new Promise<{ snapshot: EngineSnapshot }>((done) => { resolve = done })
      }
      return Promise.resolve({ snapshot: snapshotOf(1, 'transactions[]') })
    })
    const view = mount([table(), other], { request })
    chooseTable(view.container)
    fireEvent.click(treeRow('transactions[]')!)
    fireEvent.keyDown(treeRow('transactions[]')!, { key: ' ' })
    expect(commands).toHaveLength(1)
    clickColumn(view.container, 'e11')
    fireEvent.keyDown(treeRow('transactions[]')!, { key: 'ArrowRight' })
    expand('item 1')
    expect(treeRow('debit')!.querySelector('.binding-dot')).not.toBeNull()
    fireEvent.click(treeRow('debit')!)
    fireEvent.click(homeOf(view.container, 'e2'))
    fireEvent.click(treeRow('total')!)
    expect(commands).toHaveLength(1)
    resolve({ snapshot: snapshotOf(2, 'transactions[]') })
    await waitFor(() => expect(home(view.container).querySelector('.canvas-table-collection')).toHaveTextContent('transactions[]'))
    expect(homeOf(view.container, 'e2')).toHaveClass('canvas-component-selected')
    expect(screen.queryByText('Asking the engine to bind the picked path…')).not.toBeInTheDocument()
    fireEvent.click(treeRow('total')!)
    expect(commands).toHaveLength(2)
    expect(commands[1]).toBe('{"kind":"bindComponentScalar","version":1,"id":"e2","segments":["total"]}')
    resolve({ snapshot: snapshotOf(3, 'transactions[]') })
    await waitFor(() => expect(screen.queryByText('Asking the engine to bind the picked path…')).not.toBeInTheDocument())
  })

  it.each(['sample', 'selection', 'document'] as const)('drops a late collection refusal after %s replacement', async (replacement) => {
    let reject!: (error: unknown) => void
    const request = vi.fn((operation: string) => operation === 'command'
      ? new Promise<never>((_resolve, fail) => { reject = fail })
      : Promise.resolve({ snapshot: snapshotOf(1) }))
    const view = render(<App engine={{ request } as unknown as EngineClient} initialSnapshot={snapshotOf(1)} initialSampleData={sample()} blankBytes={new Uint8Array([7]).buffer} sampleFileAccess={{ openSample: async () => ({ name: 'replacement.json', bytes: new TextEncoder().encode(SAMPLE_JSON).buffer }) }} />)
    chooseTable(view.container)
    fireEvent.click(treeRow('transactions[]')!)
    if (replacement === 'sample') {
      fireEvent.click(screen.getByRole('button', { name: 'Replace sample JSON' }))
      await screen.findByText('replacement.json')
    } else if (replacement === 'selection') {
      fireEvent.click(homeOf(view.container, 'e2'))
    } else {
      startBlankFromNew()
      await screen.findByText('Started an unnamed local template')
    }
    reject({ elementId: 'e7', message: 'old collection refusal' })
    await waitFor(() => expect(screen.queryByText('Asking the engine to bind the picked path…')).not.toBeInTheDocument())
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('does not install a late collection projection after document replacement', async () => {
    let resolve!: (value: { snapshot: EngineSnapshot }) => void
    const request = vi.fn((operation: string) => operation === 'command'
      ? new Promise<{ snapshot: EngineSnapshot }>((done) => { resolve = done })
      : Promise.resolve({ snapshot: snapshotOf(1) }))
    const view = render(<App engine={{ request } as unknown as EngineClient} initialSnapshot={snapshotOf(1)} initialSampleData={sample()} blankBytes={new Uint8Array([7]).buffer} />)
    chooseTable(view.container)
    fireEvent.click(treeRow('transactions[]')!)
    startBlankFromNew()
    await screen.findByText('Started an unnamed local template')
    resolve({ snapshot: snapshotOf(2, 'transactions[]') })
    await Promise.resolve(); await Promise.resolve()
    expect(home(view.container).querySelector('.canvas-table-collection')).toHaveTextContent('items[]')
  })

  it('does not discover row arrays or collections below truncated ancestor keys as root collections', () => {
    const loaded = acceptSampleData('scopes.json', new TextEncoder().encode(JSON.stringify({ transactions: [{ ref: 1, nested: [{ forbidden: 2 }] }], ['x'.repeat(121)]: { nested: [{ hidden: 3 }] } })).buffer)
    const scan = tableSampleCandidates(loaded.tree)
    expect(scan.candidates).toEqual([{ collection: 'transactions[]', field: 'ref' }])
    expect([...scan.byNode.values()]).toEqual(scan.candidates)
  })

  it('presents a collection-command refusal in DATA and retains the current engine binding', async () => {
    const sent: Sent = { commands: [] }
    const engine = engineFor(sent, 1, 'collection segments must be identifiers')
    const sample = acceptSampleData('invalid-key.json', new TextEncoder().encode('{"a.b":[]}').buffer)
    const view = render(<App engine={engine as unknown as EngineClient} initialSnapshot={snapshotOf(1)} initialSampleData={sample} />)
    chooseTable(view.container)
    fireEvent.click(treeRow('a.b[]')!)
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('e7: collection segments must be identifiers')
    expect(alert.closest('.data-panel')).not.toBeNull()
    expect(sent.commands).toEqual(['{"kind":"bindTableCollection","version":1,"id":"e7","segments":["a.b"]}'])
    expect(home(view.container).querySelector('.canvas-table-collection')).toHaveTextContent('items[]')
  })
})

describe('a table column is bound from the main window', () => {
  it('makes a clicked column the column selection while the table stays the component selection', () => {
    const sent: Sent = { commands: [] }
    const view = mount([table(), other], engineFor(sent))
    clickColumn(view.container, 'e10')
    // The column is marked, in cyan, on BOTH of its spans — heading and cell.
    expect(selectedColumnIds(view.container)).toEqual(['e10', 'e10'])
    // The owning table is still the component selection…
    expect(home(view.container).className).toContain('canvas-component-selected')
    // …and "Configure columns" stays live, which is the working function the
    // compound-id arm would have broken (`openTableEditor` guards on
    // `selectedRef.current[0] !== id`).
    expect(screen.getByRole('button', { name: 'Configure columns' })).toBeEnabled()
    // The identity strip names the column and offers no control.
    const strip = screen.getByText('Column').closest('.column-identity') as HTMLElement
    expect(within(strip).getByText('Date')).toBeInTheDocument()
    expect(strip.querySelectorAll('button, input, select, textarea, [role="button"]')).toHaveLength(0)
    // A CELL RESOLVES THE SAME COLUMN AS ITS HEADING — both spans carry the id,
    // and the unbound column's cell is `.canvas-table-unset` rather than
    // `.canvas-table-cell`, which is the branch a heading-only test never
    // reaches. `e11` has an empty bind and is still selectable.
    const cells = columnSpans(view.container, 'e11')
    const cell = cells[cells.length - 1]!
    expect(cell.className).toContain('canvas-table-unset')
    fireEvent.click(cell)
    expect(selectedColumnIds(view.container)).toEqual(['e11', 'e11'])
    expect(within(screen.getByText('Column').closest('.column-identity') as HTMLElement).getByText('Amount')).toBeInTheDocument()
  })

  it('selects the table with no column when the click resolves no column id', () => {
    const sent: Sent = { commands: [] }
    const view = mount([table(), other], engineFor(sent))
    clickColumn(view.container, 'e10')
    expect(selectedColumnIds(view.container)).toEqual(['e10', 'e10'])
    fireEvent.click(home(view.container).querySelector('.canvas-table-chip') as HTMLElement)
    expect(home(view.container).className).toContain('canvas-component-selected')
    expect(selectedColumnIds(view.container)).toEqual([])
    expect(screen.queryByText('Column')).toBeNull()
  })

  it('adds the table to a mixed selection when Shift-clicking a column', () => {
    const sent: Sent = { commands: [] }
    const view = mount([table(), other], engineFor(sent))
    fireEvent.click(screen.getByRole('button', { name: /text component e2/ }))
    fireEvent.click(columnSpans(view.container, 'e11')[0]!, { shiftKey: true })
    expect(selectedColumnIds(view.container)).toEqual([])
    // Exactly one component is selected — the owning table — so every
    // `selected.length === 1` gate keeps its current meaning.
    expect(view.container.querySelectorAll('.canvas-component-selected:not(.canvas-component-echo)')).toHaveLength(2)
    expect(home(view.container).className).toContain('canvas-component-selected')
  })

  it('offers exactly the row-scope fields of the table s collection, and names the column in the context bar', () => {
    const sent: Sent = { commands: [] }
    const view = mount([table(), other], engineFor(sent))
    clickColumn(view.container, 'e11')
    openData()
    expect(screen.getByText('Column Amount selected · binding to a row field of transactions[]')).toBeInTheDocument()
    openRowFields()
    const pickable = screen.getAllByRole('treeitem').filter((node) => node.getAttribute('aria-disabled') === null && node.querySelector('.binding-dot'))
    const offered = pickable.map((node) => node.querySelector('.tree-label')?.textContent)
    expect(offered).toEqual(['date', 'debit'])
  })

  it('offers a pickable set that is EXACTLY tableSampleCandidates output for that collection', () => {
    const sent: Sent = { commands: [] }
    const view = mount([table(), other], engineFor(sent))
    clickColumn(view.container, 'e11')
    openData()
    openRowFields()
    expand('who')
    const pickable = screen.getAllByRole('treeitem').filter((node) => node.querySelector('.binding-dot'))
    // Non-vacuity first: an empty walk would make the equality below true and
    // meaningless.
    const expected = tableSampleCandidates(sample().tree).candidates.filter((candidate) => candidate.collection === 'transactions[]').map((candidate) => candidate.field)
    expect(expected).toEqual(['date', 'debit', 'who.name'])
    // ONE SET, BOTH CONSUMERS. The panel offers the leaf of each candidate
    // path, so the comparison is against the last segment of each field.
    expect(pickable.map((node) => node.querySelector('.tree-label')?.textContent)).toEqual(expected.map((field) => field.split('.').at(-1)))
  })

  it('refuses the collection itself and any path outside row scope, stating the reason before the engine would', () => {
    const sent: Sent = { commands: [] }
    const view = mount([table(), other], engineFor(sent))
    clickColumn(view.container, 'e11')
    openData()
    // The collection node: a branch, refused with its own sentence.
    const collection = treeRow('transactions[]')!
    expect(collection.getAttribute('aria-disabled')).toBeNull() // still expandable
    expect(within(collection).getByText('Collection · a column binds one row field of transactions[], never a collection.')).toBeInTheDocument()
    // A document-scope scalar: refused, dimmed, and it dispatches nothing.
    const outside = treeRow('total')!
    expect(outside.getAttribute('aria-disabled')).toBe('true')
    expect(within(outside).getByText('Not a row field of transactions[] · that is all a table column can bind.')).toBeInTheDocument()
    fireEvent.click(outside)
    expect(sent.commands).toEqual([])
  })

  it('sends exactly one unchanged updateTableColumnBinding command for a picked row field', async () => {
    const sent: Sent = { commands: [] }
    const view = mount([table(), other], engineFor(sent))
    clickColumn(view.container, 'e11')
    openData()
    openRowFields()
    fireEvent.click(treeRow('debit')!)
    await waitFor(() => expect(sent.commands).toHaveLength(1))
    // THE BARE ROW-RELATIVE FIELD — never the alias, never the collection. Go
    // resolves the alias itself and writes `{{alias.field}}`.
    expect(sent.commands[0]).toBe('{"kind":"updateTableColumnBinding","version":1,"id":"e7","columnId":"e11","field":"debit"}')
    // AC6 — ONE COMMAND IS ONE UNDO STEP, and the guarantee is Go's:
    // `folio-go/internal/wasm/engine.go`'s `Apply` pushes exactly one undo per accepted
    // byte-changing command. The browser's part is not to bypass the command
    // path and not to send a second command (a `configureTableBinding` first,
    // say). That is what this asserts; nothing was built for it.
    expect(sent.commands.filter((text) => text.includes('configureTableBinding'))).toEqual([])
  })

  it('binds a column whose bind is empty, and a dotted row path, from the same set', async () => {
    const sent: Sent = { commands: [] }
    const view = mount([table(), other], engineFor(sent))
    clickColumn(view.container, 'e11')
    openData()
    openRowFields()
    expand('who')
    fireEvent.click(treeRow('name')!)
    await waitFor(() => expect(sent.commands).toHaveLength(1))
    expect(sent.commands[0]).toBe('{"kind":"updateTableColumnBinding","version":1,"id":"e7","columnId":"e11","field":"who.name"}')
  })

  it('shows an engine refusal where the pick was made', async () => {
    const sent: Sent = { commands: [] }
    const view = mount([table(), other], engineFor(sent, 1, 'field is not addressable'))
    clickColumn(view.container, 'e11')
    openData()
    openRowFields()
    fireEvent.click(treeRow('debit')!)
    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toBe('e7: field is not addressable')
    expect(alert.closest('.data-panel')).not.toBeNull()
  })

  it('drops the column selection when the id stops resolving', async () => {
    const sent: Sent = { commands: [] }
    const engine = engineFor(sent)
    const view = mount([table(), other], engine)
    clickColumn(view.container, 'e10')
    expect(selectedColumnIds(view.container)).toEqual(['e10', 'e10'])
    openData()
    expect(screen.getByText('Column Date selected · binding to a row field of transactions[]')).toBeInTheDocument()
    // The column is REMOVED and the document RE-PROJECTED, which is what
    // `commitTableColumn` does after a Remove in the table editor. The state
    // holds two ids and nothing else, so the next render simply fails to
    // resolve one of them.
    engine.setComponents([table([AMOUNT]), other])
    openRowFields()
    fireEvent.click(treeRow('date')!)
    await waitFor(() => expect(sent.commands).toHaveLength(1))
    await waitFor(() => expect(selectedColumnIds(view.container)).toEqual([]))
    // The panel stops offering a column that does not exist and falls back to
    // the table's own (unchanged) sentence.
    expect(screen.queryByText('Column Date selected · binding to a row field of transactions[]')).toBeNull()
    expect(screen.queryByText('Column')).toBeNull()
    expect(screen.getByText('Table selected · pick a root collection to bind its rows.')).toBeInTheDocument()
  })

  it('says the table is bound to nothing rather than offering fields it cannot bind', () => {
    const sent: Sent = { commands: [] }
    const view = mount([table([DATE, AMOUNT], ''), other], engineFor(sent))
    clickColumn(view.container, 'e10')
    openData()
    expect(screen.getByText('Column Date selected · its table is bound to no collection, so it has no row fields to offer.')).toBeInTheDocument()
    expect(screen.getAllByRole('treeitem').filter((node) => node.querySelector('.binding-dot'))).toHaveLength(0)
  })

  // PATCH — THE UNBOUND TABLE'S OWN SENTENCES, AND THE DEFECT IS AN EMPTY NOUN.
  //
  // `columnBindScope` is built whenever a column resolves; only `rowFields` is
  // emptied when the table binds nothing. So `scope.collection` is `''` on a
  // table mid-authoring, and the bound spellings interpolated it into
  // `a column binds one row field of , never a collection.` and
  // `Not a row field of  · …` — a missing noun and a double space, on a path an
  // author reaches by placing a table and clicking a column. The row below
  // asserts the exact replacements and then sweeps every reason in the tree for
  // the shape an empty interpolation leaves behind.
  it('names no empty collection in either refusal when the table is bound to nothing', () => {
    const sent: Sent = { commands: [] }
    const view = mount([table([DATE, AMOUNT], ''), other], engineFor(sent))
    clickColumn(view.container, 'e10')
    openData()
    const collection = treeRow('transactions[]')!
    expect(within(collection).getByText('Collection · this column’s table is bound to no collection, so it has no row scope.')).toBeInTheDocument()
    const outside = treeRow('total')!
    expect(outside.getAttribute('aria-disabled')).toBe('true')
    expect(within(outside).getByText('Not a row field · this column’s table is bound to no collection.')).toBeInTheDocument()
    // AND NOTHING ANYWHERE IN THE TREE CARRIES THE HOLE. Both bound spellings
    // leave a detectable scar when the collection is empty — `of ,` and the
    // double space in `of  ·` — so the sweep is a real instrument rather
    // than a restatement of the two assertions above.
    const reasons = Array.from(view.container.querySelectorAll('.tree-reason')).map((node) => node.textContent ?? '')
    expect(reasons.length, 'the tree must actually state some reasons for this sweep to sweep anything').toBeGreaterThan(0)
    for (const reason of reasons) {
      expect(reason, 'an empty collection was interpolated into a reason').not.toContain('  ')
      expect(reason, 'an empty collection was interpolated into a reason').not.toMatch(/\bof\s*[,·]/)
    }
  })

  // PATCH — THE `TABLE ONLY` BADGE IS SCALAR-GATE VOCABULARY AND DOES NOT
  // SURVIVE INTO COLUMN MODE. It means "a table, not this text component, binds
  // a collection"; beside a sentence saying THIS TABLE's column cannot bind the
  // collection it contradicts its own row. The positive control is in the same
  // row: select a text component and the badge comes straight back.
  it('keeps TABLE ONLY out of column and whole-table modes and restores it for text', () => {
    const sent: Sent = { commands: [] }
    const view = mount([table(), other], engineFor(sent))
    clickColumn(view.container, 'e11')
    openData()
    expect(within(treeRow('transactions[]')!).queryByText('TABLE ONLY')).toBeNull()
    // The refusal itself is untouched — the badge left, the stated reason did not.
    expect(within(treeRow('transactions[]')!).getByText('Collection · a column binds one row field of transactions[], never a collection.')).toBeInTheDocument()
    // POSITIVE CONTROL: the same query finds the badge the moment the panel is
    // back in scalar mode, so its absence above is a measurement. Whole-table
    // mode offers the collection and likewise has no TABLE ONLY badge.
    fireEvent.click(home(view.container).querySelector('.canvas-table-chip') as HTMLElement)
    expect(within(treeRow('transactions[]')!).queryByText('TABLE ONLY')).not.toBeInTheDocument()
    fireEvent.click(homeOf(view.container, 'e2'))
    expect(within(treeRow('transactions[]')!).getByText('TABLE ONLY')).toBeInTheDocument()
  })

  // PATCH — A `params` LEAF GETS THE REFUSAL THAT ACTUALLY GOVERNS IT.
  // `runtime` was computed in the column branch and read only by the badge, so
  // a params path fell through to the row-scope sentence. Go refuses a `params`
  // root as a data binding outright, for a column exactly as for a scalar.
  it('states the runtime-parameter refusal for a params path in column mode', () => {
    const sent: Sent = { commands: [] }
    const withParams = acceptSampleData('c.json', new TextEncoder().encode('{"params":{"asOf":"01 Jul"},"transactions":[{"date":"01 Jul"}]}').buffer)
    const engine = engineFor(sent)
    const view = render(<App engine={engine as unknown as EngineClient} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: [table(), other] } }} initialSampleData={withParams} />)
    clickColumn(view.container, 'e11')
    openData()
    expand('params')
    const leaf = treeRow('asOf')!
    expect(leaf.getAttribute('aria-disabled')).toBe('true')
    expect(within(leaf).getByText('Runtime parameter · not a row field, and the engine refuses a params path as a data binding.')).toBeInTheDocument()
    // NOT the row-scope sentence, which is what it used to read.
    expect(within(leaf).queryByText('Not a row field of transactions[] · that is all a table column can bind.')).toBeNull()
  })

  // PATCH — THE KEYBOARD PICK, AND ITS ABSENCE WAS A DEMONSTRATED FALSE GREEN.
  //
  // `DataTree` consults `rowFor(current, scope)` in its Enter/Space handler and
  // `rowFor(entry, scope)` in the render path — two separate calls. Every other
  // column-mode row in this file reaches the leaf with `fireEvent.click`, so
  // dropping the scope argument from the Enter/Space call alone left all of them
  // green while keyboard picking died silently: a row-scope leaf carries no
  // `segments`, so the scalar predicate returns false and Enter does nothing.
  //
  // ⚠ MUTATION-PROVED. With `rowFor(current)` in the Enter/Space arm this row
  // reds at `expect(sent.commands).toHaveLength(1)` — timed out, still zero
  // commands — and every other row in the file stays green. This is the DATA
  // tree, an existing keyboard-operable control, not the canvas keyboard gap
  // D-14.10.3 deferred: DW-387 does not cover it.
  it('sends the same one command when the row field is picked with the keyboard', async () => {
    const sent: Sent = { commands: [] }
    const view = mount([table(), other], engineFor(sent))
    clickColumn(view.container, 'e11')
    openData()
    openRowFields()
    const row = treeRow('debit')!
    row.focus()
    fireEvent.keyDown(row, { key: 'Enter' })
    await waitFor(() => expect(sent.commands).toHaveLength(1))
    expect(sent.commands[0]).toBe('{"kind":"updateTableColumnBinding","version":1,"id":"e7","columnId":"e11","field":"debit"}')
  })

  // PATCH — THE REFUSAL IS NARROWED ON BOTH IDS, AND THE COLUMN ID ALONE IS NOT
  // UNIQUE ACROSS TABLES.
  //
  // `bindPickedColumn` records the owning table in `componentID`, and
  // App.tsx:311-313 says in bold why both halves are load-bearing: "a bare
  // column id is not unique across tables". The narrowing had been comparing
  // the column id and the field only.
  //
  // ⚠ DRIVEN THROUGH THE PANEL RATHER THAN THROUGH THE CANVAS, and the reason
  // is that the canvas cannot ask this question. `select` clears
  // `bindingError` on every click, so re-selecting a column on a second table
  // through the canvas would find no alert whatever the narrowing said — a
  // green that measures the clear, not the key. Re-projecting the panel with a
  // different `tableId` and everything else held identical is the only way to
  // put the two ids in disagreement.
  it('shows a column refusal only for the table whose column was refused', () => {
    const loaded = sample()
    const scan = tableSampleCandidates(loaded.tree)
    const rowFields = new Map([...scan.byNode].filter(([, candidate]) => candidate.collection === 'transactions[]').map(([node, candidate]) => [node, candidate.field] as const))
    expect(rowFields.size, 'the fixture must offer row fields for this row to pick one').toBeGreaterThan(0)
    const error = { sample: loaded, componentID: 'e7', segments: [], message: 'e7: field is not addressable', columnId: 'e11', field: 'debit' }
    // EVERYTHING BUT THE TABLE ID IS HELD IDENTICAL — same sample, same column
    // id, same label, same collection, same offered fields, same recorded
    // refusal — so the only thing the assertions below can be reading is the
    // table comparison.
    const panel = (tableId: string) => <DataPanel sample={loaded} busy={false} available bindingError={error} columnScope={{ tableId, columnId: 'e11', label: 'Amount', collection: 'transactions[]', rowFields }} onLoad={() => undefined} onConnectColumn={() => undefined} />
    const { rerender } = render(panel('e7'))
    openRowFields()
    fireEvent.click(treeRow('debit')!)
    expect(screen.getByRole('alert').textContent).toBe('e7: field is not addressable')
    // The SAME column id on a DIFFERENT table does not inherit it.
    rerender(panel('e8'))
    expect(screen.queryByRole('alert')).toBeNull()
    // POSITIVE CONTROL: it comes straight back on the table it belongs to, so
    // the absence above is a narrowing rather than a panel that stopped
    // rendering alerts.
    rerender(panel('e7'))
    expect(screen.getByRole('alert').textContent).toBe('e7: field is not addressable')
  })

  it('leaves a columnless table with nothing addressable, and 14.9 s notice unchanged', () => {
    const sent: Sent = { commands: [] }
    const view = mount([table(null), other], engineFor(sent))
    expect(home(view.container).querySelectorAll('[data-column-id]')).toHaveLength(0)
    expect(home(view.container).textContent).toContain('No columns yet.')
  })
})

// AC7 — THE REQUIRED GUARD.
//
// Under Q1(b) `selected` holds ELEMENT IDS and nothing else, so every one of the
// six delete/duplicate routes sees a real table id and behaves exactly as it
// does today. That is a property of the state shape rather than of a
// conditional, and a test that merely asserted "a command was sent" would pass
// on the compound-id arm too — a guard that cannot fail, which is the first
// entry in this run's defect catalogue.
//
// So the claim asserted here is the one the compound-id arm breaks: with a
// column selected, EVERY command any of the six routes sends names the TABLE id
// and NEVER the column id.
//
// ⚠ THE MUTATION THAT PROVES THIS IS THE NARROW ONE, AND THE OBVIOUS ONE IS A
// WEAKER PROOF THAN IT LOOKS. Both were run before this comment was written.
//   • PROVES IT — make the two funnels act on the column selection:
//     `deleteComponentCommand(selected[0]!)` -> `(columnSelection?.columnId ?? selected[0]!)`
//     in `deleteSelection`, and the same in `duplicateSelection`. Measured: 6 of
//     19 red, every one of them AT the `named the column id` assertion below,
//     with the real payload naming `e10`. That is the assertion having teeth.
//   • DOES NOT PROVE IT — making `select` put the column id into `selected`.
//     Measured: 17 of 19 red, but the six rows below fail at their PRECONDITION
//     (`expected [] to deeply equal ['e10','e10']`), because that mutation also
//     breaks the paint path this file selects through. Every row goes red
//     without the claim under test ever being evaluated.
// A mutation that reds a test without reaching its assertion is evidence the
// test runs, not evidence the test can see the defect.
describe('no route sends a command naming a column id while a column is selected', () => {
  const columnId = 'e10'
  const routes: ReadonlyArray<readonly [string, (container: Element) => void]> = [
    ['canvas-tools Delete', () => fireEvent.click(screen.getByRole('button', { name: 'Delete' }))],
    ['canvas-tools Duplicate', () => fireEvent.click(screen.getByRole('button', { name: 'Duplicate' }))],
    ['canvas-region Delete key', () => { const region = screen.getByLabelText('Canvas region'); region.focus(); fireEvent.keyDown(region, { key: 'Delete' }) }],
    ['canvas-region Backspace key', () => { const region = screen.getByLabelText('Canvas region'); region.focus(); fireEvent.keyDown(region, { key: 'Backspace' }) }],
    ['component Delete key', (container) => fireEvent.keyDown(home(container), { key: 'Delete' })],
    ['window Cmd/Ctrl+D', () => fireEvent.keyDown(window, { key: 'd', ctrlKey: true })],
  ]
  for (const [name, invoke] of routes) {
    it(`sends no command naming a column id: ${name}`, async () => {
      const sent: Sent = { commands: [] }
      const view = mount([table(), other], engineFor(sent))
      clickColumn(view.container, columnId)
      expect(selectedColumnIds(view.container)).toEqual([columnId, columnId])
      invoke(view.container)
      // Non-vacuity: the route must actually reach the engine, or "no column id
      // was named" is true of a route that did nothing at all.
      await waitFor(() => expect(sent.commands.length).toBeGreaterThan(0))
      for (const command of sent.commands) {
        expect(command, `${name} named the column id`).not.toContain(`"${columnId}"`)
        expect(command, `${name} did not name the table`).toContain('"e7"')
      }
    })
  }
})

// THE SECOND COVER (Q5), AND IT IS EXPLICITLY NOT THE FIRST.
//
// This proves the declaration is in the stylesheet. It cannot prove the click
// resolves to the column: a later overriding rule, a covering element, or a
// wrapper that still captures would each leave it green. The real-browser click
// in e2e/table-column-binding.spec.ts is the instrument for that, and both are
// mutation-proved by deleting `pointer-events: auto`.
describe('App.css restores hit testing on the column spans', () => {
  const rule = '.canvas-table-heading:not(.canvas-component-echo *), .canvas-table-grid .canvas-table-cell:not(.canvas-component-echo *), .canvas-table-grid .canvas-table-unset:not(.canvas-component-echo *) { pointer-events: auto; cursor: pointer; }'
  it('declares pointer-events: auto on the heading and cell spans, over .canvas-table s pointer-events: none', () => {
    expect(appCss).toContain('.canvas-table { position: absolute; inset: 0; overflow-x: clip; overflow-y: visible; pointer-events: none;')
    expect(appCss).toContain(rule)
    // MUTATION-PROVED, not asserted: deleting the declaration reds this row.
    const mutated = appCss.replace(rule, rule.replace('pointer-events: auto; ', ''))
    expect(mutated).not.toBe(appCss)
    expect(mutated).not.toContain(rule)
  })

  // AND THE ECHOES ARE OUT OF IT, which is a SECOND claim rather than a
  // restatement of the first. `.canvas-component-echo` carries
  // `pointer-events: none` on its ROOT, and a descendant that sets it back to
  // `auto` is hit-tested regardless — the very mechanism the rule above uses to
  // beat `.canvas-table`'s `none`. So an unscoped rule would light up every
  // continuation sheet's repeated table body: `cursor: pointer` on a decoration,
  // and a click swallowed from the page surface that would otherwise have
  // cleared or kept the selection.
  //
  // ⚠ MUTATION-PROVED BY WIDENING, NOT BY DELETING. Dropping the `:not()` from
  // every compound is the defect this row exists for, and it is what reds the
  // assertion below.
  it('excludes the continuation-sheet echoes from the spans it makes hit-testable', () => {
    const compounds = rule.slice(0, rule.indexOf('{')).trim().split(', ')
    expect(compounds).toHaveLength(3)
    for (const compound of compounds) expect(compound, `${compound} must exclude the echo subtree`).toContain(':not(.canvas-component-echo *)')
    // The widened spelling — the rule as it shipped before this fence — must
    // not be in the stylesheet at all.
    expect(appCss).not.toContain(rule.replaceAll(':not(.canvas-component-echo *)', ''))
    // AND THE ECHO REALLY DOES RE-USE THESE CLASS NAMES, so the exclusion is
    // about something rather than defensive. Proved against the rendered DOM:
    // a table tall enough to cross a window boundary is echoed onto the next
    // sheet, and the echo paints the same `.canvas-table-grid` spans carrying
    // the same `data-column-id`s the shipped rule matches.
    const sent: Sent = { commands: [] }
    const tall = { ...table(), height: 800_000 }
    const stacked = { ...canvas, contentWindowCount: 2, contentWindowOrigins: [0, 700_000], contentWindowPages: [0, 0], components: [tall] }
    const view = render(<App engine={engineFor(sent) as unknown as EngineClient} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: stacked }} />)
    const echoed = Array.from(view.container.querySelectorAll('.canvas-component-echo .canvas-table-grid [data-column-id]'))
    expect(echoed.length, 'the fixture must actually paint an echoed table body').toBeGreaterThan(0)
    for (const span of echoed) expect(span.className).toMatch(/canvas-table-(?:heading|cell|unset)/)
  })
})
