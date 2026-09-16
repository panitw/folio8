import fs from 'node:fs'
import { createHash } from 'node:crypto'
import { act, cleanup, createEvent, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App, { placementPoint, PROSE_COMMIT_DEBOUNCE_MS } from './App'
import type { OfflineLifecycle } from './offline-lifecycle'
import { isMacPlatform, shortcutHintsFor } from './shortcuts'
import { PREVIEW_DEBOUNCE_MS } from './preview/freshness'
import { PAGE_RAIL_BOUND } from './preview/page-rail-facts'
import { embeddedFaceFamily } from './embedded-face-family'
import { shippedFaceFamily } from './shipped-face-family'
import { shippedFamilyEntry } from './shipped-face-cuts'
import { FileAccessCancelled, FileAccessFailure, folioFileFormat, jsonSampleFileFormat, pdfFileFormat, type AcquiredSaveTarget, type FileAccess, type SavedLocalFile, type SaveRequest, type SaveTargetRequest } from './file/file-access'
import { FileSystemAccess } from './file/file-system-access'
import { InputDownloadAccess } from './file/input-download'
import type { EngineClient } from './engine-client'
import { LOCALE_TAGS, type CanvasProjection } from './engine-protocol'
import { acceptSampleData } from './sample-data'
import { MAX_CANVAS_SHEETS } from './sheet-stack'
import { catalogueFaces } from './generated/font-catalogue'
import { documentationAssetUrls } from './generated/documentation-assets'
import { PDF_FIXTURE_DIGEST, RENDER_ELAPSED_MS, RENDER_ENGINE_VERSION } from './test/pdf-fixture'
import { startBlankFromNew } from './test/new-document'
import { IDBFactory as FakeIndexedDBFactory } from 'fake-indexeddb'

// STORY 16.5 — SOME OF THESE TESTS NEED A MACHINE THAT CAN KEEP A FACE.
//
// Installing IS the store write: there is no command behind it to succeed, so
// an install into a browser that will not keep anything is a REFUSED install
// (`App.font-store.test.tsx` asserts that refusal in its own right). jsdom 28.1.0
// provides no IndexedDB at all, so the web-tier tests below install a FRESH fake
// factory for their own duration and put back whatever was there. It is scoped
// per test rather than to the file, because the other hundred-odd tests here are
// about a designer with no font store and must stay that way.
const withMachineStore = (): (() => void) => {
  const previous = Object.getOwnPropertyDescriptor(globalThis, 'indexedDB')
  Object.defineProperty(globalThis, 'indexedDB', { value: new FakeIndexedDBFactory(), configurable: true, writable: true })
  return () => {
    if (previous) Object.defineProperty(globalThis, 'indexedDB', previous)
    else Reflect.deleteProperty(globalThis, 'indexedDB')
  }
}

// face() builds the PROJECTED shape of a named-face chain entry (Story 8.3:
// an entry is a discriminated object, not a string). A named face carries no
// family and no style — its name is its identity.
// STORY 11.3: a projected entry carries its DECLARED style variants too, and
// they are always-present keys — '' is absent. `variants` lets a fixture declare
// a cut without every other call site restating three empty strings.
const face = (name: string, variants: Partial<Readonly<{ bold: string; italic: string; boldItalic: string }>> = {}) => ({ face: name, assetKey: '', family: '', style: '', bold: '', italic: '', boldItalic: '', ...variants })

// carried() is the projected shape of an EMBEDDED chain entry: no face name,
// an asset key, and the family/style Go read out of the asset's own `font`
// record for the panel to display. The key never becomes a family here — that
// derivation is embedded-face-family.ts's alone (D-8.4.1).
const carried = (assetKey: string, variants: Partial<Readonly<{ bold: string; italic: string; boldItalic: string }>> = {}) => ({ face: '', assetKey, family: 'Noto Sans Thai', style: 'Regular', bold: '', italic: '', boldItalic: '', ...variants })

// STORY 12.3 — the sixteen TABLE-LEVEL members the table-columns projection
// gained, as one fixture the table tests spread in.
//
// Two per header-style field: the bare name is what the DOCUMENT declares ('' or
// 0 for absent) and the `…Resolved` twin is what the ENGINE says will actually
// be used. This fixture is a table that declares no header style at all, so
// every committed member is absent while every resolved one carries the
// cascade's answer — which is the shape that makes "the panel shows the
// resolved value" observable at all.
const tableHeaderProjection = { sizing: 'points' as const, totalWidth: 72000, headerHeight: 12000, altRowBackground: '', headerFontFamily: '', headerFontFamilyResolved: 'body', headerFontSize: 0, headerFontSizeResolved: 12000, headerLineSpacing: 0, headerLineSpacingResolved: 1000, headerBackground: '', headerBackgroundResolved: '', headerColor: '', headerColorResolved: '', headerValign: '', headerValignResolved: 'top', headerAlign: '', headerAlignResolved: 'left', headerBold: false, headerBoldResolved: false, headerItalic: false, headerItalicResolved: false, 'headerBorder.width': '', 'headerBorder.widthResolved': '', 'headerBorder.color': '', 'headerBorder.colorResolved': '', 'headerBorder.edges': '', 'headerBorder.edgesResolved': '', minHeight: 0, 'rules.width': '', 'rules.widthResolved': '', 'rules.color': '', 'rules.colorResolved': '', 'rules.between': '', paddingLeft: '', paddingRight: '', paddingHeaderOverride: false }

// installStubFontSet installs the page font set jsdom does not implement and
// returns its own removal. `Object.defineProperty` because neither the face
// constructor nor the set exists to be assigned over.
//
// IT RECORDS WHAT THE SEAM DID TO IT, in order: the families added and the
// families removed. Registration and release are otherwise invisible — nothing
// in the DOM says a face was released — so a claim about the seam's LIFETIME
// can only be made against this record.
function installStubFontSet(): Readonly<{ restore: () => void; added: string[]; removed: string[] }> {
  class StubFace {
    readonly family: string
    constructor(family: string) { this.family = family }
    load(): Promise<StubFace> { return Promise.resolve(this) }
  }
  const added: string[] = []
  const removed: string[] = []
  const set = { add: (face: StubFace) => { added.push(face.family); return undefined }, delete: (face: StubFace) => { removed.push(face.family); return undefined } }
  Object.defineProperty(globalThis, 'FontFace', { value: StubFace, configurable: true, writable: true })
  Object.defineProperty(document, 'fonts', { value: set, configurable: true, writable: true })
  return { restore: () => { Reflect.deleteProperty(globalThis, 'FontFace'); Reflect.deleteProperty(document, 'fonts') }, added, removed }
}

// TWO text components drawing through ONE carried entry, so a per-component
// registration lifetime shows up as two asset requests instead of one.
const carriedFaceCanvas = (key: string) => {
  const paint = { overflow: false, truncated: false, lines: [{ top: 0, baseline: 12_000, advance: 16_000, width: 24_000, fragments: [{ text: 'สัญญา', x: 0, assetKey: key }] }] }
  const component = (id: string, y: number) => ({ id, type: 'text' as const, band: 'content' as const, x: 0, y, width: 72_000, height: 24_000, resizable: true, value: 'ignored', textPaint: paint })
  return { ...canvas, fontFamilies: ['body'], fontChains: [{ name: 'body', entries: [face('Noto Sans'), carried(key)] }], components: [component('e1', 0), component('e2', 30_000)] }
}

// TWO carried entries, ONE PER COMPONENT, so a claim about one key can be read
// off the DOM against a SETTLED outcome for the other. The fragment order in
// the container is the component order: `first` then `second`.
const twoCarriedFacesCanvas = (first: string, second: string) => {
  const paint = (key: string) => ({ overflow: false, truncated: false, lines: [{ top: 0, baseline: 12_000, advance: 16_000, width: 24_000, fragments: [{ text: 'สัญญา', x: 0, assetKey: key }] }] })
  const component = (id: string, y: number, key: string) => ({ id, type: 'text' as const, band: 'content' as const, x: 0, y, width: 72_000, height: 24_000, resizable: true, value: 'ignored', textPaint: paint(key) })
  return { ...canvas, fontFamilies: ['body'], fontChains: [{ name: 'body', entries: [face('Noto Sans'), carried(first), carried(second)] }], components: [component('e1', 0, first), component('e2', 30_000, second)] }
}

// ONE carried component that CROSSES A WINDOW SEAM, so the projection produces
// a home occurrence AND an echo of it on the next sheet. Every content window
// after the first is drawn by echoes, so a multi-sheet document is the ordinary
// case rather than an exotic one.
const carriedFaceEchoCanvas = (key: string) => {
  const paint = { overflow: false, truncated: false, lines: [{ top: 650_000, baseline: 662_000, advance: 16_000, width: 24_000, fragments: [{ text: 'สัญญา', x: 0, assetKey: key }] }] }
  const spanning = { id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 650_000, width: 72_000, height: 100_000, resizable: true, value: 'ignored', textPaint: paint }
  return { ...canvas, fontFamilies: ['body'], fontChains: [{ name: 'body', entries: [face('Noto Sans'), carried(key)] }], contentWindowCount: 3, contentWindowOrigins: [0, 700_000, 1_400_000], contentWindowPages: [0, 0, 0], components: [spanning] }
}

// ONE SHIPPED-FACE component that CROSSES A WINDOW SEAM, drawing LATIN text
// through a chain that names only "Noto Sans Thai" — the I/O matrix's
// "Latin through a Thai-first chain" row, projected. The engine attributes the
// fragment to the face it measured with; nothing here carries a font, so the
// only identity on the wire is the shipped one.
const shippedFaceEchoCanvas = (name: string) => {
  const paint = { overflow: false, truncated: false, lines: [{ top: 650_000, baseline: 662_000, advance: 16_000, width: 24_000, fragments: [{ text: 'A5', x: 0, face: name }] }] }
  const spanning = { id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 650_000, width: 72_000, height: 100_000, resizable: true, value: 'ignored', textPaint: paint }
  return { ...canvas, fontFamilies: ['body'], fontChains: [{ name: 'body', entries: [face(name)] }], contentWindowCount: 3, contentWindowOrigins: [0, 700_000, 1_400_000], contentWindowPages: [0, 0, 0], components: [spanning] }
}

// The family sequence a rendered fragment asks for, quotes removed: jsdom
// re-spells single quotes as double ones when a declaration is read back, and
// the claim is about WHICH families are asked for and in what ORDER.
const familiesAskedFor = (node: HTMLElement) => node.style.fontFamily === '' ? [] : node.style.fontFamily.split(',').map((entry) => entry.trim().replace(/^['"]|['"]$/g, ''))

// STORY 13.6 — THE PAGES RAIL IS STOOD IN FOR, FOR THE SAME REASON THE VIEWER
// IS. The real rail opens a `PDFDocumentProxy` and drives vendored pdf.js
// thumbnail code; in jsdom `canvas.getContext('2d')` is null and
// `OffscreenCanvas` does not exist, so rasterisation is proven against the
// component in `preview/page-rail.test.tsx` and in a real browser in
// `e2e/preview-page-rail.spec.ts`. What only `App` can answer is asserted here:
// that the palette gives way to the rail and back, what the rail is HANDED, and
// that a rail click reaches the same page-state funnel the status bar writes
// through.
//
// ⚠ THE STAND-IN IMPORTS THE REAL BOUND AND SPELLS THE REAL LABELS. Both were
// divergences worth naming: a hard-coded `12` here meant setting
// `PAGE_RAIL_BOUND` to 10 left `toHaveLength(12)` passing against a rail showing
// ten, and a `Page N thumbnail` label meant every App-level query in this file
// named a string the shipped rail never produces. A stand-in that answers to
// names production does not use is a test of the stand-in.
vi.mock('./preview/page-rail', async () => {
  const { PAGE_RAIL_BOUND } = await import('./preview/page-rail-facts')
  return {
    PageRail: ({ pages, currentPage, onGoToPage }: { pages?: number; currentPage: number; onGoToPage: (page: number) => void }) => <nav aria-label="Page thumbnails"><p>PAGES</p><code data-testid="page-rail-props">{JSON.stringify({ pages, currentPage })}</code>{Array.from({ length: Math.min(pages ?? 0, PAGE_RAIL_BOUND) }).map((_, index) => index + 1).map((page) => <button key={page} type="button" aria-label={`Page ${page}`} aria-current={page === currentPage ? 'page' : undefined} onClick={() => onGoToPage(page)}>{page}</button>)}</nav>,
  }
})

vi.mock('./preview/pdf-viewer', () => ({
  initialPDFPreviewViewState: { page: 1, scale: 1, ['scroll' + 'Top']: 0, ['scroll' + 'Left']: 0 },
  samePDFPreviewViewState: () => false,
  // STORY 13.2 — THE STAND-IN NOW CARRIES THE VIEW STATE IN BOTH DIRECTIONS.
  //
  // The status bar's navigation writes THROUGH App into this prop, and the real
  // viewer writes back through `onStateChange` when the author scrolls. Neither
  // half was observable while this stand-in ignored both, so the two buttons and
  // the readout below are added; the first two buttons are untouched, because a
  // hundred-odd tests locate them by exactly those names.
  //
  // The readout is `JSON.stringify`, so the scroll members reach the test under
  // their real names WITHOUT those names ever being written in this file — the
  // canvas-authority corpus scan reads this source text and does not waive it.
  PDFPreviewViewer: ({ label, describedBy, state, onStateChange, onPageCount, onError }: { label: string; describedBy: string; state: Record<string, unknown>; onStateChange: (next: Record<string, unknown>) => void; onPageCount: (pages: number) => void; onError: (error: Error) => void }) => <><button type="button" aria-label={label} aria-describedby={describedBy} onClick={() => onPageCount(1)}>Admit local PDF</button><button type="button" aria-label="Fail local PDF viewer" onClick={() => onError(new Error('viewer rejected bytes'))}>Fail local PDF viewer</button><button type="button" aria-label="Admit long local PDF" onClick={() => onPageCount(34)}>Admit long local PDF</button><button type="button" aria-label="Scroll local PDF viewer" onClick={() => onStateChange({ ...state, ['scroll' + 'Top']: 240, ['scroll' + 'Left']: 12 })}>Scroll local PDF viewer</button><code data-testid="pdf-viewer-state">{JSON.stringify(state)}</code></>,
}))

const bytes = new Uint8Array([1, 2, 3]).buffer
// The family combobox lists TWO groups since Story 8.6 — the chains the
// document declares, and the bundled catalogue it does not. Only a catalogue
// entry carries a source note, so that is what separates them here: the note is
// what the entry DOES, not how it looks, and a control that stopped
// distinguishing the two would fail this rather than restyle past it.
//
// MECHANICAL (Story 16.5): the note used to be the literal `add to document` on
// every catalogue arm. It now says which of two things a pick does — INSTALL a
// family this machine does not hold, or USE one it does — and the one phrase all
// three arms still share is the machine. `font-index.test.ts` is where the three
// sentences themselves are pinned; this only needs the partition.
// STORY 16.7 GAVE EVERY OPTION AN ARIA-HIDDEN SPECIMEN, so raw `textContent`
// now carries a sample the row's accessible name deliberately excludes (the
// honesty rule requires it: the specimen is decorative to assistive
// technology). `optionText` reads what a screen reader would — the row's name
// and its note, if it still has one — by dropping every `aria-hidden`
// descendant before reading text, so these assertions stay about what the row
// SAYS rather than about whether its specimen happened to finish loading.
const optionText = (option: Element): string => {
  const clone = option.cloneNode(true) as Element
  clone.querySelectorAll('[aria-hidden="true"]').forEach((node) => node.remove())
  return clone.textContent ?? ''
}
// AND `declaredOptions` NOW SCOPES BY GROUP MEMBERSHIP RATHER THAN BY TEXT.
// The "this machine" substring used to be true of every non-declared row and
// false of every declared one; Story 16.7 (D-16.R.72, narrowed) removed the
// per-row note from `AVAILABLE LOCALLY` too, so that heuristic would now let a
// local-tier row through as though it were declared. `IN THIS TEMPLATE` is the
// group's own accessible name and the one fact this control never overloads.
const declaredOptions = () => {
  const listbox = screen.getByRole('listbox', { name: 'Fonts' })
  const group = within(listbox).queryByRole('group', { name: 'IN THIS TEMPLATE' })
  return group ? within(group).queryAllByRole('option') : []
}
const sample = acceptSampleData('sample.json', new TextEncoder().encode('{"customer":{"name":"Preview customer"},"transactions":[]}').buffer)
const canvas = { width: 595276, height: 841890, orientation: 'portrait' as const, preset: 'A4' as const, locale: 'en' as const, utcOffset: '+07:00', marginTop: 36000, marginRight: 36000, marginBottom: 36000, marginLeft: 36000, gridIncrement: 6000, commandWidth: 595276, commandHeight: 841890, fontFamilies: ['body', 'heading'], fontChains: [{ name: 'body', entries: [face('Noto Sans')] }, { name: 'heading', entries: [face('Noto Sans'), face('Noto Sans Thai')] }], defaultFontSize: 12000, defaultLineSpacing: 1000, contentWindowHeight: 729890, contentWindowCount: 1, contentWindowOrigins: [0], contentWindowPages: [0], contentWindowCountIsExact: true, bands: [{ name: 'pageHeader' as const, x: 36000, y: 36000, width: 523276, height: 20000 }, { name: 'content' as const, x: 36000, y: 56000, width: 523276, height: 729890 }, { name: 'pageFooter' as const, x: 36000, y: 785890, width: 523276, height: 20000 }], components: [] }
const snapshot = (revision: number) => ({ documentState: 'loaded' as const, revision, byteLength: 3, canvas })
const engine = (request = vi.fn(async (operation: string) => ({ snapshot: { documentState: 'loaded' as const, revision: operation === 'command' ? 2 : 1, byteLength: 3 }, ...(operation === 'serialize' ? { bytes } : {}) }))) => ({ request: (operation: string, payload?: ArrayBuffer, ...rest: unknown[]) => {
  if (operation === 'group-move-preview') {
    const intent = JSON.parse(new TextDecoder().decode(payload))
    return Promise.resolve({ snapshot: { documentState: 'loaded', revision: intent.expectedRevision, byteLength: 3 }, groupMove: { revision: intent.expectedRevision, dx: intent.dx * 1000, dy: intent.dy * 1000 } })
  }
  return (request as (...args: unknown[]) => unknown)(operation, payload, ...rest)
} }) as unknown as EngineClient

describe('application shell', () => {
  it('hydrates engine-owned state when asynchronous startup replaces the loading shell', () => {
    const lifecycle = { state: 'ready' as const, cacheReady: true, verifiedAssetUrls: [] }
    const view = render(<App key="engine-loading" loadState={lifecycle} engineState="starting" />)
    expect(screen.getByRole('status', { name: 'Engine preparation status' })).toHaveTextContent('Starting local engine')
    view.rerender(<App key="engine-ready" engine={engine()} initialSnapshot={snapshot(1)} loadState={lifecycle} engineState="starting" />)
    expect(screen.getByTestId('engine-snapshot')).toHaveTextContent('GO SNAPSHOT · REVISION 1')
    expect(screen.getByLabelText('Report page with Page Header, Content, and Page Footer')).toBeInTheDocument()
  })

  it('renders every persistent desktop landmark and honest later regions', () => {
    render(<App initialSnapshot={snapshot(1)} />)
    expect(screen.getByLabelText('Document bar')).toBeInTheDocument()
    expect(screen.getByLabelText('Component palette')).toBeInTheDocument()
    expect(screen.getByLabelText('Canvas region')).toBeInTheDocument()
    expect(screen.getByLabelText('Report page with Page Header, Content, and Page Footer')).toBeInTheDocument()
    expect(screen.getByLabelText('Properties panel')).toBeInTheDocument()
    expect(screen.getByLabelText('Status bar')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'PREVIEW' })).toHaveAttribute('aria-pressed', 'false')
  })

  // STORY 16.4 — THE STATUS BAR STATES THE FONT COUNT, AND IT IS THE SAME COUNT
  // THE DROPDOWN'S FIRST GROUP DRAWS.
  //
  // The two surfaces are asserted AGAINST EACH OTHER rather than each against a
  // literal, because the property is that they teach one model from one source:
  // `canvas.fontFamilies`, which is `IN THIS TEMPLATE`'s own predicate. A count
  // read from anywhere else — the fonts added this session, say, which is what
  // the mockup binds — would pass a two-literal test and fail this one.
  it('states the template font count in the status bar, from the source the first group groups on', () => {
    const componentCanvas = { ...canvas, components: [{ id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello' }] }
    render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    const bar = screen.getByLabelText('Status bar')
    expect(within(bar).getByTestId('template-font-count')).toHaveTextContent('2 fonts in template')
    fireEvent.click(screen.getByLabelText('text component e1'))
    fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
    expect(within(screen.getByRole('group', { name: 'IN THIS TEMPLATE' })).getAllByRole('option'), 'the bar and the first group must be counting the same thing').toHaveLength(2)
  })

  // AND IT AGREES WITH ITSELF AT ONE. A hardcoded "N fonts" reads "1 fonts",
  // which is the kind of small lie a status bar makes for ever.
  it('says one font in the singular', () => {
    const oneChain = { ...canvas, fontFamilies: ['body'], fontChains: [{ name: 'body', entries: [face('Noto Sans')] }] }
    render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: oneChain }} />)
    expect(screen.getByTestId('template-font-count')).toHaveTextContent('1 font in template')
  })

  // ⚠ STORY 14.6 REGRESSION FENCE — THE TABLE EDITOR'S SAMPLE-DRIVEN DATALIST.
  // `tableSampleCandidates` is gated on `kind === 'collection' && segments?.length`,
  // and 14.6 briefly stripped `segments` from every collection node in
  // `sample-data.ts` to stop an empty collection being offered as a scalar
  // candidate. That emptied BOTH datalists for every template — in the same
  // story whose new context bar started sending table authors here — and the
  // whole suite stayed green, because nothing anywhere read a <datalist>
  // option. This row is that missing reader.
  //
  it('offers the loaded sample collections and scoped row fields as table editor candidates', async () => {
    const tableCanvas = { ...canvas, components: [{ id: 'e7', type: 'table' as const, band: 'content' as const, x: 0, y: 0, width: 72000, height: 12000, resizable: false }] }
    const tableSnapshot = { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: tableCanvas }
    const request = vi.fn(async (operation: string) => {
      if (operation === 'table-columns') return { snapshot: tableSnapshot, tableColumns: { revision: 1, table: { tableId: 'e7', collection: 'transactions[]', alias: 'row', ...tableHeaderProjection, columns: [{ id: 'e8', header: 'Amount', width: 72000, proportion: '', align: 'right' as const, headerAlign: '' as const, headerAlignResolved: 'right' as const, binding: '', rowField: '', rowFieldEditable: true, footer: '' as const, footerOf: '', footerFormat: '' }] } } }
      return { snapshot: tableSnapshot }
    })
    const sample = acceptSampleData('c.json', new TextEncoder().encode('{"transactions":[{"date":"01 Jul","debit":12}]}').buffer)
    const { container } = render(<App engine={engine(request)} initialSnapshot={tableSnapshot} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'table component e7' }))
    fireEvent.click(screen.getByRole('button', { name: 'Configure columns' }))
    await screen.findByRole('grid', { name: 'Table columns' })
    const optionValues = (id: string) => Array.from(container.querySelectorAll(`#${id} option`)).map((option) => option.getAttribute('value'))
    expect(optionValues('table-collection-candidates')).toEqual(['transactions[]'])
    expect(optionValues('table-row-field-candidates')).toEqual(['{{row.date}}', '{{row.debit}}'])
    expect(screen.getByRole('combobox', { name: 'Binding for column 1' })).toHaveValue('')
    expect(screen.getByLabelText('Binding for column 1')).toBeInTheDocument()
  })

  it('opens an engine-projected, keyboard-operable table matrix with named controls', async () => {
    const tableCanvas = { ...canvas, components: [{ id: 'e7', type: 'table' as const, band: 'content' as const, x: 0, y: 0, width: 72000, height: 12000, resizable: false }] }
    const tableSnapshot = { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: tableCanvas }
    const request = vi.fn(async (operation: string) => {
      if (operation === 'table-columns') return { snapshot: tableSnapshot, tableColumns: { revision: 1, table: { tableId: 'e7', collection: 'items[]', alias: 'row', ...tableHeaderProjection, columns: [{ id: 'e8', header: 'Amount', width: 72000, proportion: '', align: 'right' as const, headerAlign: '' as const, headerAlignResolved: 'right' as const, binding: '{{row.amount}}', rowField: 'amount', rowFieldEditable: true, footer: '' as const, footerOf: '', footerFormat: '' }] } } }
      return { snapshot: tableSnapshot }
    })
    render(<App engine={engine(request)} initialSnapshot={tableSnapshot} />)
    fireEvent.click(screen.getByRole('button', { name: 'table component e7' }))
    fireEvent.click(screen.getByRole('button', { name: 'Configure columns' }))
    const grid = await screen.findByRole('grid', { name: 'Table columns' })
    // STORY 14.7 — SEVEN COLUMNS, WHICH IS WHAT THE DESIGN DRAWS (HEADER ALIGN
    // joined CELL ALIGN). The eleven included four row ACTIONS wearing column
    // headers (`Move earlier`, `Move later`, `Remove`, `Add after`) and three
    // fields for the one footer concept. `aria-colcount` counts the columns; the
    // keyboard lattice behind them is fifteen cells wide and is
    // `TableEditor.test.tsx`'s subject.
    expect(grid).toHaveAttribute('aria-colcount', '7')
		expect(grid).toHaveAttribute('aria-rowcount', '2')
    // The exact walk includes the restored row-field control.
    const header = screen.getByRole('textbox', { name: 'Header for column 1' })
    header.focus(); fireEvent.keyDown(header, { key: 'ArrowRight' })
    expect(document.activeElement).toBe(screen.getByRole('combobox', { name: 'Binding for column 1' }))
    fireEvent.keyDown(document.activeElement!, { key: 'ArrowRight', altKey: true })
    expect(document.activeElement).toBe(screen.getByRole('spinbutton', { name: 'Width for column 1 in points' }))
    expect(screen.getByRole('button', { name: 'Move column 1 earlier' })).toBeDisabled()
		expect(screen.getByRole('button', { name: 'Move column 1 later' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Remove column 1' })).toBeInTheDocument()
  })

  it('traps the focused matrix, closes on Escape, and restores its invoking control', async () => {
    const tableCanvas = { ...canvas, components: [{ id: 'e7', type: 'table' as const, band: 'content' as const, x: 0, y: 0, width: 72000, height: 12000, resizable: false }] }
    const tableSnapshot = { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: tableCanvas }
    render(<App engine={engine(vi.fn(async (operation: string) => operation === 'table-columns' ? { snapshot: tableSnapshot, tableColumns: { revision: 1, table: { tableId: 'e7', collection: 'items[]', alias: 'row', ...tableHeaderProjection, columns: [{ id: 'e8', header: 'Amount', width: 72000, proportion: '', align: 'left' as const, headerAlign: '' as const, headerAlignResolved: 'left' as const, binding: '{{row.amount}}', rowField: 'amount', rowFieldEditable: true, footer: '' as const, footerOf: '', footerFormat: '' }] } } } : { snapshot: tableSnapshot }))} initialSnapshot={tableSnapshot} />)
    fireEvent.click(screen.getByRole('button', { name: 'table component e7' }))
    const invoker = screen.getByRole('button', { name: 'Configure columns' })
    invoker.focus(); fireEvent.click(invoker)
    const header = await screen.findByRole('textbox', { name: 'Header for column 1' })
    expect(document.activeElement).toBe(header)
    // STORY 12.3 AMENDED THIS ASSERTION, AND THE AMENDMENT IS THE DECISION
    // (D-12.3.2). D-12.3.2's order was: Close Table Editor, Root collection, Row
    // alias, the one active matrix cell, then the HEADER AND ROWS controls in
    // document order, ending at "Header alignment". The old line before it read
    // like a forward tab and was in fact the WRAP branch — trapDialog filters to
    // tabIndex >= 0, which excludes every non-active matrix cell, so the active
    // cell happened to be last.
    //
    // THAT ORDER HAS SINCE MOVED TWICE MORE — Story 14.7 sent the exit down to
    // the footer bar and Story 14.7b made it a `Cancel` / `Done` pair — so the
    // list now BEGINS at Root collection and ENDS at Done. Both ends of it are
    // asserted below, re-derived from the DOM.

    // THE FORWARD HANDOFF, ASSERTED RATHER THAN ASSUMED. The amendment removed
    // the old forward-reading line and did not replace it, so the matrix's
    // handoff into the new section could regress in silence. jsdom moves focus
    // for no Tab of its own, so the property is taken in the two halves that
    // ARE observable here:
    //
    //   1. the trap DECLINES to intercept a forward Tab from the matrix cell —
    //      true only while the cell is no longer last in its list, so putting
    //      the section back above the matrix reds this line; and
    //   2. the next tabbable control after the cell, in the trap's own document
    //      order, is the header section's first control.
    //
    // TAB FIRST WALKS THE ROW'S FIELDS (`tabThroughMatrix`): header → binding →
    // width → the pressed alignment segment → footer aggregate, and Shift+Tab
    // walks back. Only from the last field does the handoff below apply.
    const walk = [screen.getByRole('combobox', { name: 'Binding for column 1' }), screen.getByRole('spinbutton', { name: 'Width for column 1 in points' }), screen.getByRole('button', { name: 'Header align left for column 1' }), screen.getByRole('button', { name: 'Align left for column 1' }), screen.getByRole('combobox', { name: 'Footer aggregate for column 1' })]
    for (const next of walk) { fireEvent.keyDown(document.activeElement!, { key: 'Tab' }); expect(document.activeElement).toBe(next) }
    fireEvent.keyDown(document.activeElement!, { key: 'Tab', shiftKey: true })
    expect(document.activeElement).toBe(walk[3])
    fireEvent.keyDown(document.activeElement!, { key: 'Tab' })
    const lastCell = walk[4]!
    fireEvent.keyDown(lastCell, { key: 'Tab' })
    expect(document.activeElement, 'a forward Tab from the matrix cell must not wrap: the cell is no longer last').toBe(lastCell)
    const dialogElement = screen.getByRole('dialog', { name: 'Table Editor' })
    // `textarea` IS IN THIS SELECTOR BECAUSE IT IS IN THE TRAP'S (TableEditor's
    // own `focusable` query). SPEC-table-rules made the header-label cell a
    // `<textarea>` — a label may hold a line feed and an `<input>` cannot — and
    // a selector here that omitted the element the trap includes would measure a
    // different list from the one the product walks.
    const tabbable = Array.from(dialogElement.querySelectorAll<HTMLElement>('button:not([disabled]), input:not([disabled]), textarea:not([disabled]), select:not([disabled])')).filter((element) => element.tabIndex >= 0)
    // STORY 14.7 RE-ORDERED THIS LIST A SECOND TIME AND STORY 14.7b HAS NOW
    // RE-ORDERED IT A THIRD, AND EVERY ONE OF THE THREE IS INTENDED — say so,
    // because an unexplained re-ordering reads as a regression to the next
    // author. 14.7: `Add column` left the row and became one control BELOW the
    // grid, so it is what now follows the active matrix cell; and `Close Table
    // Editor` left the heading for the footer bar, so it stopped being FIRST in
    // the trap's list. 14.7b: that one button is now the `Cancel` / `Done` pair,
    // so the list's LAST member is `Done` and `Cancel` sits immediately before
    // it. The wrap therefore runs from `Done` to `Root collection` — the
    // dialog's first control — and backwards from `Root collection` to `Done`.
    //
    // ⚠ AND THE LIST'S TAIL IS NOW STATE-DEPENDENT. `trapDialog` selects
    // `button:not([disabled])`, and `Cancel` is disabled while a command is in
    // flight and above the engine's history bound — so it drops OUT of this list
    // in both those states and the wrap ends move again. That case is proved in
    // `TableEditor.test.tsx`, where the count can be set directly; here the
    // count is zero and `Cancel` is present. BOTH ENDS ARE RE-DERIVED FROM THE
    // DOM rather than named, so a fourth re-ordering has to face the assertions
    // and not the comment.
    expect(tabbable[tabbable.indexOf(lastCell) + 1]).toBe(screen.getByRole('button', { name: 'Add column' }))
    // BOTH ENDS ARE THE LIST'S OWN, and the pair's ORDER is read off the list
    // rather than off a hard-coded offset from its tail: `Cancel` is asserted to
    // be the member immediately BEFORE whatever the last member turns out to be.
    // A fourth re-ordering then reds the identity of the ends, which is the
    // claim, instead of an arithmetic that happens to still land on a button.
    const firstControl = tabbable[0] as HTMLElement
    const lastControl = tabbable[tabbable.length - 1] as HTMLElement
    expect(firstControl).toBe(screen.getByLabelText('Root collection'))
    expect(lastControl).toBe(screen.getByRole('button', { name: 'Done' }))
    expect(tabbable[tabbable.indexOf(lastControl) - 1]).toBe(screen.getByRole('button', { name: 'Cancel' }))
    lastControl.focus()
    fireEvent.keyDown(lastControl, { key: 'Tab' })
    expect(document.activeElement).toBe(firstControl)
    fireEvent.keyDown(document.activeElement!, { key: 'Tab', shiftKey: true })
    expect(document.activeElement).toBe(lastControl)
    fireEvent.keyDown(screen.getByRole('dialog', { name: 'Table Editor' }), { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Table Editor' })).not.toBeInTheDocument())
    expect(document.activeElement).toBe(invoker)
  })

  it('admits a committed table snapshot after deselection and never reopens a closed session', async () => {
    const tableCanvas = { ...canvas, components: [{ id: 'e7', type: 'table' as const, band: 'content' as const, x: 0, y: 0, width: 72000, height: 12000, resizable: false }] }
    const first = { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: tableCanvas }
    const second = { documentState: 'loaded' as const, revision: 2, byteLength: 4, canvas: { ...tableCanvas, components: [{ ...tableCanvas.components[0]!, width: 144000 }] } }
    let releaseProjection!: () => void
    const delayedProjection = new Promise<{ snapshot: typeof second; tableColumns: { revision: number; table: typeof tableHeaderProjection & { tableId: string; collection: string; alias: string; columns: { id: string; header: string; width: number; proportion: string; align: 'left'; headerAlign: ''; headerAlignResolved: 'left'; binding: string; rowField: string; rowFieldEditable: boolean; footer: ''; footerOf: string; footerFormat: string }[] } } }>((resolve) => { releaseProjection = () => resolve({ snapshot: second, tableColumns: { revision: 2, table: { tableId: 'e7', collection: 'items[]', alias: 'row', ...tableHeaderProjection, columns: [{ id: 'e8', header: 'Amount', width: 72000, proportion: '', align: 'left', headerAlign: '' as const, headerAlignResolved: 'left' as const, binding: '{{row.amount}}', rowField: 'amount', rowFieldEditable: true, footer: '', footerOf: '', footerFormat: '' }] } } }) })
    let queries = 0
    const request = vi.fn((operation: string) => {
      if (operation === 'table-columns') { queries++; return queries === 1 ? Promise.resolve({ snapshot: first, tableColumns: { revision: 1, table: { tableId: 'e7', collection: 'items[]', alias: 'row', ...tableHeaderProjection, columns: [{ id: 'e8', header: 'Amount', width: 72000, proportion: '', align: 'left' as const, headerAlign: '' as const, headerAlignResolved: 'left' as const, binding: '{{row.amount}}', rowField: 'amount', rowFieldEditable: true, footer: '' as const, footerOf: '', footerFormat: '' }] } } }) : delayedProjection }
      if (operation === 'command') return Promise.resolve({ snapshot: second })
      return Promise.resolve({ snapshot: first })
    })
    render(<App engine={engine(request)} initialSnapshot={first} />)
    fireEvent.click(screen.getByRole('button', { name: 'table component e7' }))
    fireEvent.click(screen.getByRole('button', { name: 'Configure columns' }))
    await screen.findByRole('button', { name: 'Add column' })
    fireEvent.click(screen.getByRole('button', { name: 'Add column' }))
    // STORY 14.7b — `Done`, NOT `Cancel`. This test measures what a COMMITTED
    // snapshot does after the dialog closes; `Cancel` would issue an undo for
    // the column just added and change the very thing being measured.
    fireEvent.click(screen.getByRole('button', { name: 'Done' }))
    releaseProjection()
    await waitFor(() => expect(screen.getByTestId('engine-snapshot')).toHaveTextContent('REVISION 2'))
    expect(screen.queryByRole('dialog', { name: 'Table Editor' })).not.toBeInTheDocument()
  })

  // STORY 12.3 — THE HEADER AND ROWS SECTION.
  //
  // The engine has always accepted, stored and rendered headerHeight,
  // altRowBackground and headerStyle; until now nothing in the product could
  // write any of them, so a striped table meant hand-editing the file the
  // designer had just saved. These assertions are about the missing half.
  // THE MOCK ANSWERS EACH `table-columns` CALL SEPARATELY, and that is not a
  // convenience. It used to return ONE frozen projection for every request and
  // reply to 'command' with the same revision, so no control in this section
  // could ever change and every assertion below was about the FIRST render. A
  // whole class of defect was therefore invisible: the colour rows are
  // half-controlled — an uncontrolled text box beside a controlled chip — and a
  // committed value that never moves can never desynchronise them. `after` is
  // what the SECOND and later projections carry, exactly as the delayed-
  // projection test above already does with its own `queries` counter.
  const headerStyledTable = (over: Partial<typeof tableHeaderProjection> = {}, after?: Partial<typeof tableHeaderProjection>) => {
    const tableCanvas = { ...canvas, components: [{ id: 'e7', type: 'table' as const, band: 'content' as const, x: 0, y: 0, width: 72000, height: 12000, resizable: false }] }
    const tableSnapshot = { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: tableCanvas }
    let queries = 0
    const request = vi.fn(async (operation: string) => {
      if (operation !== 'table-columns') return { snapshot: tableSnapshot }
      queries++
      const table = { ...tableHeaderProjection, ...over, ...(queries > 1 ? after ?? {} : {}) }
      return { snapshot: tableSnapshot, tableColumns: { revision: 1, table: { tableId: 'e7', collection: 'items[]', alias: 'row', ...table, columns: [{ id: 'e8', header: 'Amount', width: 72000, proportion: '', align: 'right' as const, headerAlign: '' as const, headerAlignResolved: 'right' as const, binding: '{{row.amount}}', rowField: 'amount', rowFieldEditable: true, footer: '' as const, footerOf: '', footerFormat: '' }] } } }
    })
    return { request, tableSnapshot }
  }
  const openHeaderSection = async (request: ReturnType<typeof headerStyledTable>['request'], tableSnapshot: ReturnType<typeof headerStyledTable>['tableSnapshot']) => {
    render(<App engine={engine(request)} initialSnapshot={tableSnapshot} />)
    fireEvent.click(screen.getByRole('button', { name: 'table component e7' }))
    fireEvent.click(screen.getByRole('button', { name: 'Configure columns' }))
    await screen.findByRole('grid', { name: 'Table columns' })
    request.mockClear()
  }
  const commandsSent = (request: ReturnType<typeof headerStyledTable>['request']) =>
    (request.mock.calls as unknown as ReadonlyArray<[string, ArrayBuffer]>).filter(([operation]) => operation === 'command').map(([, payload]) => new TextDecoder().decode(payload))

  it('offers the three table-level subjects and shows the engine\'s resolved value for each unset header field', async () => {
    const { request, tableSnapshot } = headerStyledTable()
    await openHeaderSection(request, tableSnapshot)
    // Committed: the fixture declares no header style at all, so every box is
    // empty and every select sits on its own "Not set" option.
    expect(screen.getByRole('spinbutton', { name: 'Header height in points' })).toHaveValue(12)
    expect(screen.getByRole('textbox', { name: 'Alternating row background' })).toHaveValue('')
    expect(screen.getByRole('textbox', { name: 'Header font family' })).toHaveValue('')
    expect(screen.getByRole('combobox', { name: 'Header alignment' })).toHaveValue('')
    expect(screen.getByRole('combobox', { name: 'Header vertical alignment' })).toHaveValue('')
    // RESOLVED: what the document will actually use, and it is here because the
    // ENGINE sent it. The panel composes nothing — these strings are the
    // projection's `…Resolved` members, and a browser that worked them out
    // would be running a second copy of the engine's cascade.
    //
    // AND IT IS CARRIED BY THE CONTROL ITSELF, not by a line beside it. The
    // inspector's rule over `FieldSpec.empty` — the engine's answer for an
    // uncommitted field "is shown as a placeholder, never as a value" — now
    // holds here too, so each box stays EMPTY (asserted above) while showing
    // what the cascade will use. The placeholder is what proves the two states
    // are still distinct: a resolved value written in as `toHaveValue` would be
    // a table that had frozen the cascade into the document.
    expect(screen.getByRole('textbox', { name: 'Header font family' })).toHaveAttribute('placeholder', 'body')
    expect(screen.getByRole('spinbutton', { name: 'Header font size (pt)' })).toHaveAttribute('placeholder', '12')
    expect(screen.getByRole('spinbutton', { name: 'Header line spacing' })).toHaveAttribute('placeholder', '1')
    // A select has no placeholder, so its empty option's own label carries the
    // value — which is what this control's comment always claimed it did.
    expect(screen.getByRole('combobox', { name: 'Header alignment' })).toHaveTextContent('Not set (left)')
    expect(screen.getByRole('combobox', { name: 'Header vertical alignment' })).toHaveTextContent('Not set (top)')
    // An empty RESOLVED value is a real answer — the cascade found nothing to
    // resolve from — and is spelled as one rather than as a blank. It is not
    // the SAME answer for every field, though, and one word for all of them
    // is wrong for the ink: a header with no resolved background paints
    // nothing, but a header with no resolved COLOUR still draws, in the
    // renderer's own default. A shared "nothing" claimed the one thing that
    // cannot happen. The distinction survived the move into the placeholder.
    expect(screen.getByRole('textbox', { name: 'Header background' })).toHaveAttribute('placeholder', 'none — no fill painted')
    expect(screen.getByRole('textbox', { name: 'Header text colour' })).toHaveAttribute('placeholder', "renderer's default ink")
    expect(screen.getByRole('textbox', { name: 'Header text colour' })).not.toHaveAttribute('placeholder', 'none — no fill painted')
    // And the section is a NAMED group. An aria-label on a bare div with no
    // role is dropped by the accessibility tree, so it named nothing at all.
    expect(screen.getByRole('group', { name: 'Table header and rows' })).toBeInTheDocument()
    // Neither number advertises a value both arms refuse: setTableHeaderHeight
    // and the fontSize arm each require a POSITIVE length, so `min="0"` offered
    // the author a value the engine would send straight back.
    expect(screen.getByRole('spinbutton', { name: 'Header height in points' })).toHaveAttribute('min', '1')
    expect(screen.getByRole('spinbutton', { name: 'Header font size (pt)' })).toHaveAttribute('min', '0.5')
    expect(screen.getByRole('spinbutton', { name: 'Header line spacing' })).toHaveAttribute('min', '0.1')
  })

  it('shows the engine\'s fallback for a field the document does not declare, with nothing on the wire to derive it from', async () => {
    // The table declares `style.fontSize: 8` and no headerStyle, so the engine
    // resolves 8pt. The committed member is 0 (absent) and NOTHING else in this
    // projection carries the table's own style — so a panel showing 8pt can only
    // be reading the engine's answer. A browser that composed the fallback
    // itself would have nothing here to compose it from, which is the point.
    const { request, tableSnapshot } = headerStyledTable({ headerFontSize: 0, headerFontSizeResolved: 8000 })
    await openHeaderSection(request, tableSnapshot)
    expect(screen.getByRole('spinbutton', { name: 'Header font size (pt)' })).toHaveValue(null)
    expect(screen.getByRole('spinbutton', { name: 'Header font size (pt)' })).toHaveAttribute('placeholder', '8')
  })

  it('renders an unset colour as unset rather than as black', async () => {
    const { request, tableSnapshot } = headerStyledTable()
    await openHeaderSection(request, tableSnapshot)
    // `swatchColor('')` returns #000000 — the picker accepts nothing else — so
    // an absent colour without the dashed treatment reads as a colour the
    // author chose. This is invisible to every command assertion.
    const unset = screen.getByLabelText('Pick Header background')
    expect(unset).toHaveValue('#000000')
    expect(unset.className).toContain('property-swatch-unset')
    expect(screen.getByLabelText('Pick Alternating row background').className).toContain('property-swatch-unset')
  })

  it('marks a committed colour as set', async () => {
    const { request, tableSnapshot } = headerStyledTable({ headerBackground: '#101010', headerBackgroundResolved: '#101010' })
    await openHeaderSection(request, tableSnapshot)
    const chip = screen.getByLabelText('Pick Header background')
    expect(chip).toHaveValue('#101010')
    expect(chip.className).not.toContain('property-swatch-unset')
  })

  it('commits the header height, the alternating row background and one header-style field on blur', async () => {
    const { request, tableSnapshot } = headerStyledTable()
    await openHeaderSection(request, tableSnapshot)
    const height = screen.getByRole('spinbutton', { name: 'Header height in points' })
    fireEvent.blur(height, { target: { value: '18' } })
    await waitFor(() => expect(commandsSent(request)).toHaveLength(1))
    expect(commandsSent(request)[0]).toBe('{"kind":"setTableHeaderHeight","version":1,"id":"e7","height":18}')
  })

  it('sends the engine a set for a typed colour and a clear for an emptied one', async () => {
    const { request, tableSnapshot } = headerStyledTable({ altRowBackground: '#DDEEFF' })
    await openHeaderSection(request, tableSnapshot)
    const alt = screen.getByRole('textbox', { name: 'Alternating row background' })
    fireEvent.blur(alt, { target: { value: '' } })
    await waitFor(() => expect(commandsSent(request)).toHaveLength(1))
    expect(commandsSent(request)[0]).toBe('{"kind":"setTableAltRowBackground","version":1,"id":"e7","op":"clear"}')
  })

  it('passes a malformed colour to the engine rather than inventing a second validation', async () => {
    const { request, tableSnapshot } = headerStyledTable()
    await openHeaderSection(request, tableSnapshot)
    fireEvent.blur(screen.getByRole('textbox', { name: 'Alternating row background' }), { target: { value: 'not-a-colour' } })
    await waitFor(() => expect(commandsSent(request)).toHaveLength(1))
    // Go's parseHexColor is the gate and its located sentence is what the author
    // reads. A panel that refused this locally would own a rule it cannot keep
    // in step with the file door.
    expect(commandsSent(request)[0]).toBe('{"kind":"setTableAltRowBackground","version":1,"id":"e7","op":"set","value":"not-a-colour"}')
  })

  it('clears a header-style field from its own control and offers no clear for the required header height', async () => {
    const { request, tableSnapshot } = headerStyledTable({ headerAlign: 'center', headerAlignResolved: 'center' })
    await openHeaderSection(request, tableSnapshot)
    fireEvent.change(screen.getByRole('combobox', { name: 'Header alignment' }), { target: { value: '' } })
    await waitFor(() => expect(commandsSent(request)).toHaveLength(1))
    expect(commandsSent(request)[0]).toBe('{"kind":"updateTableHeaderStyle","version":1,"id":"e7","field":"align","op":"clear"}')
    // headerHeight is REQUIRED — `parse_bands.go` hard-errors on its absence —
    // so no clear affordance is rendered for it, while its clearable neighbours
    // all have one.
    expect(screen.queryByRole('button', { name: 'Clear Header height (pt)' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Clear Header height in points' })).toBeNull()
    for (const label of ['Alternating row background', 'Header font family', 'Header font size (pt)', 'Header line spacing', 'Header background', 'Header text colour', 'Header border width (pt)', 'Header border colour']) {
      expect(screen.getByRole('button', { name: `Clear ${label}` })).toBeInTheDocument()
    }
    // ⚠ AND THE EDGE SET HAS NO `×` OF ITS OWN, WHICH IS ALSO A RULING. A glyph
    // button outside a segmented control moves `control-vocabulary-contract`'s
    // V2 census, and the engine refuses an empty edge array — so unchecking every
    // edge already expresses the clear, and the panel sends `op: "clear"` for it.
    expect(screen.queryByRole('button', { name: 'Clear Header border edges' })).toBeNull()
  })

  // THE COLOUR ROW IS HALF-CONTROLLED, AND THE TWO HALVES MUST NOT DRIFT.
  //
  // The text box is uncontrolled (`defaultValue`) while the chip beside it is
  // controlled (`value`). After a swatch pick committed and re-projected, the
  // chip moved and the BOX KEPT THE OLD HEX — so the author's next blur on that
  // box compared stale DOM text against the new committed value, found them
  // different, and sent `op: "set"` with the OLD colour, silently undoing the
  // pick they had just made. This test is only possible because the mock now
  // answers each projection request separately; against one frozen projection
  // nothing could ever move and the defect was invisible.
  it('keeps a colour box in step with its chip after a swatch pick re-projects', async () => {
    const { request, tableSnapshot } = headerStyledTable(
      { headerBackground: '#101010', headerBackgroundResolved: '#101010' },
      { headerBackground: '#20c020', headerBackgroundResolved: '#20c020' },
    )
    await openHeaderSection(request, tableSnapshot)
    expect(screen.getByRole('textbox', { name: 'Header background' })).toHaveValue('#101010')
    fireEvent.change(screen.getByLabelText('Pick Header background'), { target: { value: '#20c020' } })
    await waitFor(() => expect(commandsSent(request)).toHaveLength(1))
    expect(commandsSent(request)[0]).toBe('{"kind":"updateTableHeaderStyle","version":1,"id":"e7","field":"background","op":"set","value":"#20c020"}')
    await waitFor(() => expect(screen.getByLabelText('Pick Header background')).toHaveValue('#20c020'))
    expect(screen.getByRole('textbox', { name: 'Header background' })).toHaveValue('#20c020')
    // AND THE PROOF THAT IT MATTERS: blurring the untouched box now sends
    // nothing. While it held the stale hex it sent a set for the OLD colour.
    fireEvent.blur(screen.getByRole('textbox', { name: 'Header background' }))
    await Promise.resolve()
    expect(commandsSent(request)).toHaveLength(1)
  })

  // AND THE SAME AFTER A CLEAR, which is the other commit that moves the chip.
  it('empties a colour box when its × clear re-projects the field as absent', async () => {
    const { request, tableSnapshot } = headerStyledTable(
      { headerColor: '#c81e1e', headerColorResolved: '#c81e1e' },
      { headerColor: '', headerColorResolved: '' },
    )
    await openHeaderSection(request, tableSnapshot)
    expect(screen.getByRole('textbox', { name: 'Header text colour' })).toHaveValue('#c81e1e')
    fireEvent.click(screen.getByRole('button', { name: 'Clear Header text colour' }))
    await waitFor(() => expect(commandsSent(request)).toHaveLength(1))
    expect(commandsSent(request)[0]).toBe('{"kind":"updateTableHeaderStyle","version":1,"id":"e7","field":"color","op":"clear"}')
    await waitFor(() => expect(screen.getByLabelText('Pick Header text colour').className).toContain('property-swatch-unset'))
    expect(screen.getByRole('textbox', { name: 'Header text colour' })).toHaveValue('')
    // The box that kept '#c81e1e' after the clear would have re-SET it on the
    // author's next blur, putting back the colour they had just removed.
    fireEvent.blur(screen.getByRole('textbox', { name: 'Header text colour' }))
    await Promise.resolve()
    expect(commandsSent(request)).toHaveLength(1)
  })

  // THE `set` PATH, AND THE POINTS <-> MILLIPOINTS ROUND TRIP IN BOTH
  // DIRECTIONS. Everything asserted above was a clear, the alt-row pair or the
  // header height; nothing asserted that typing into a header-style field emits
  // `updateTableHeaderStyle … op:"set"`, and the unit conversion was unasserted
  // at the UI in either direction — a committed 14000 rendering as 14, and a
  // typed 16 travelling as 16 for Go to multiply.
  it('renders committed millipoints as points and sends the author\'s points back for each header-style field', async () => {
    const { request, tableSnapshot } = headerStyledTable({ headerFontSize: 14000, headerFontSizeResolved: 14000, headerLineSpacing: 1500, headerLineSpacingResolved: 1500, headerFontFamily: 'body', headerFontFamilyResolved: 'body' })
    await openHeaderSection(request, tableSnapshot)
    // DIRECTION ONE: what the engine committed, as the author reads it.
    expect(screen.getByRole('spinbutton', { name: 'Header font size (pt)' })).toHaveValue(14)
    expect(screen.getByRole('spinbutton', { name: 'Header line spacing' })).toHaveValue(1.5)
    expect(screen.getByRole('textbox', { name: 'Header font family' })).toHaveValue('body')
    // DIRECTION TWO: what the author types, as the engine receives it. The
    // panel multiplies nothing — `16` travels as `16` and Go makes it 16000.
    fireEvent.blur(screen.getByRole('spinbutton', { name: 'Header font size (pt)' }), { target: { value: '16' } })
    await waitFor(() => expect(commandsSent(request)).toHaveLength(1))
    expect(commandsSent(request)[0]).toBe('{"kind":"updateTableHeaderStyle","version":1,"id":"e7","field":"fontSize","op":"set","value":16}')
  })

  it('sends a set for line spacing, for a font family and for a picked swatch', async () => {
    for (const [label, act, wire] of [
      ['Header line spacing', () => fireEvent.blur(screen.getByRole('spinbutton', { name: 'Header line spacing' }), { target: { value: '2' } }), '{"kind":"updateTableHeaderStyle","version":1,"id":"e7","field":"lineSpacing","op":"set","value":2}'],
      ['Header font family', () => fireEvent.blur(screen.getByRole('textbox', { name: 'Header font family' }), { target: { value: 'display' } }), '{"kind":"updateTableHeaderStyle","version":1,"id":"e7","field":"fontFamily","op":"set","value":"display"}'],
      ['Pick Header background', () => fireEvent.change(screen.getByLabelText('Pick Header background'), { target: { value: '#20c020' } }), '{"kind":"updateTableHeaderStyle","version":1,"id":"e7","field":"background","op":"set","value":"#20c020"}'],
      ['Pick Alternating row background', () => fireEvent.change(screen.getByLabelText('Pick Alternating row background'), { target: { value: '#ddeeff' } }), '{"kind":"setTableAltRowBackground","version":1,"id":"e7","op":"set","value":"#ddeeff"}'],
      ['Header vertical alignment', () => fireEvent.change(screen.getByRole('combobox', { name: 'Header vertical alignment' }), { target: { value: 'middle' } }), '{"kind":"updateTableHeaderStyle","version":1,"id":"e7","field":"valign","op":"set","value":"middle"}'],
    ] as ReadonlyArray<[string, () => void, string]>) {
      const { request, tableSnapshot } = headerStyledTable()
      await openHeaderSection(request, tableSnapshot)
      act()
      await waitFor(() => expect(commandsSent(request), `${label} sent nothing`).toHaveLength(1))
      expect(commandsSent(request)[0], label).toBe(wire)
      cleanup()
    }
  })

  // ONE COMMAND PER DRAG.
  //
  // <input type="color"> fires `onChange` continuously while the author drags,
  // and every one of those would be an engine command and an undo entry;
  // commitTableColumn's revision-mismatch branch also calls
  // revokeTableEditor(), so a burst could close the panel out from under the
  // author. The property is pinned here rather than left to the mechanism that
  // currently supplies it.
  //
  // TWO GUARDS SUPPLY IT and either one is enough: TableEditor's own `busy`
  // check, and App.tsx:commitTableColumn's `tableEditorBusy`. Removing either
  // alone still yields one command; removing BOTH yields three, which is what
  // shows this assertion is not vacuous. The events are dispatched inside a
  // SINGLE act() — the most batching-friendly shape available — precisely so
  // this is a real test rather than one the harness wins for free.
  it('dispatches one command for a burst of picker changes, not one per change', async () => {
    const { request, tableSnapshot } = headerStyledTable()
    await openHeaderSection(request, tableSnapshot)
    const chip = screen.getByLabelText('Pick Header background') as HTMLInputElement
    const nativeValue = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')!.set!
    await act(async () => {
      for (const value of ['#111111', '#222222', '#333333']) {
        nativeValue.call(chip, value)
        chip.dispatchEvent(new Event('change', { bubbles: true }))
      }
    })
    await waitFor(() => expect(commandsSent(request)).toHaveLength(1))
    expect(commandsSent(request), 'a drag must not become one engine command and one undo entry per frame').toHaveLength(1)
    expect(commandsSent(request)[0]).toBe('{"kind":"updateTableHeaderStyle","version":1,"id":"e7","field":"background","op":"set","value":"#111111"}')
  })

  // A BLUR THAT LANDS WHILE A COMMAND IS IN FLIGHT MUST NOT VANISH.
  //
  // `if (busy) return` dropped the edit with no visual restore, so the box was
  // left holding text the document does not hold and never would — the author
  // saw their value sitting in the field with nothing to tell them it had gone
  // nowhere. The committed value goes back into the box instead.
  it('restores the committed value when a blur lands while a command is in flight', async () => {
    const tableCanvas = { ...canvas, components: [{ id: 'e7', type: 'table' as const, band: 'content' as const, x: 0, y: 0, width: 72000, height: 12000, resizable: false }] }
    const tableSnapshot = { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: tableCanvas }
    const projection = { snapshot: tableSnapshot, tableColumns: { revision: 1, table: { tableId: 'e7', collection: 'items[]', alias: 'row', ...tableHeaderProjection, headerFontFamily: 'body', headerFontFamilyResolved: 'body', columns: [{ id: 'e8', header: 'Amount', width: 72000, proportion: '', align: 'right' as const, headerAlign: '' as const, headerAlignResolved: 'right' as const, binding: '{{row.amount}}', rowField: 'amount', rowFieldEditable: true, footer: '' as const, footerOf: '', footerFormat: '' }] } } }
    let releaseCommand!: () => void
    const request = vi.fn((operation: string) => {
      if (operation === 'table-columns') return Promise.resolve(projection)
      if (operation === 'command') return new Promise<{ snapshot: typeof tableSnapshot }>((resolve) => { releaseCommand = () => resolve({ snapshot: tableSnapshot }) })
      return Promise.resolve({ snapshot: tableSnapshot })
    })
    render(<App engine={engine(request as never)} initialSnapshot={tableSnapshot} />)
    fireEvent.click(screen.getByRole('button', { name: 'table component e7' }))
    fireEvent.click(screen.getByRole('button', { name: 'Configure columns' }))
    await screen.findByRole('grid', { name: 'Table columns' })
    request.mockClear()
    // A command starts and does not finish, so the panel is busy.
    fireEvent.click(screen.getByRole('button', { name: 'Clear Alternating row background' }))
    await waitFor(() => expect(commandsSent(request)).toHaveLength(1))
    // The author's blur on a DIFFERENT box lands mid-flight.
    const family = screen.getByRole('textbox', { name: 'Header font family' })
    fireEvent.blur(family, { target: { value: 'display' } })
    // Nothing was sent for it — that part was already true — and the box no
    // longer claims the document holds "display". The node is re-keyed, so this
    // re-queries rather than reusing the reference above.
    expect(commandsSent(request)).toHaveLength(1)
    expect(screen.getByRole('textbox', { name: 'Header font family' }), 'a discarded edit must not be left on screen looking committed').toHaveValue('body')
    releaseCommand()
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Header font family' })).not.toBeDisabled())
  })

  // A NUMBER INPUT REPORTING `badInput` MUST NOT DELETE THE FIELD. Browsers
  // report an unparseable number input's value as '', which the blur handler
  // read as an emptied box and therefore as a CLEAR — so typing garbage into
  // the font size silently removed it from the document.
  it('commits nothing when a number input cannot parse what the author typed', async () => {
    const { request, tableSnapshot } = headerStyledTable({ headerFontSize: 14000, headerFontSizeResolved: 14000 })
    await openHeaderSection(request, tableSnapshot)
    const size = screen.getByRole('spinbutton', { name: 'Header font size (pt)' })
    // jsdom does not compute validity from typed text, so `badInput` is staged
    // directly: the browser condition this guard exists for is exactly "the
    // element reports badInput and reports its value as ''".
    Object.defineProperty(size, 'validity', { configurable: true, value: { badInput: true } })
    fireEvent.blur(size, { target: { value: '' } })
    await Promise.resolve()
    expect(commandsSent(request), 'garbage in a number box must not be read as a clear').toEqual([])
    // And a genuinely emptied box — one the browser CAN parse — still clears.
    Object.defineProperty(size, 'validity', { configurable: true, value: { badInput: false } })
    fireEvent.blur(size, { target: { value: '' } })
    await waitFor(() => expect(commandsSent(request)).toHaveLength(1))
    expect(commandsSent(request)[0]).toBe('{"kind":"updateTableHeaderStyle","version":1,"id":"e7","field":"fontSize","op":"clear"}')
  })

  // SPEC-table-rules' RULED AREA, end to end through App's wiring: the exact
  // bytes each control puts on the channel, so a panel callback and a command
  // builder that each pass alone cannot disagree about the spelling between them.
  it('commits Minimum height as one setTableMinHeight command with the author\u2019s points', async () => {
    const { request, tableSnapshot } = headerStyledTable()
    await openHeaderSection(request, tableSnapshot)
    fireEvent.blur(screen.getByRole('spinbutton', { name: 'Minimum height in points' }), { target: { value: '600' } })
    await waitFor(() => expect(commandsSent(request)).toHaveLength(1))
    expect(commandsSent(request)[0]).toBe('{"kind":"setTableMinHeight","version":1,"id":"e7","op":"set","value":600}')
  })

  it('commits ticking Rule between rows as one updateTableRules set of that boundary', async () => {
    const { request, tableSnapshot } = headerStyledTable()
    await openHeaderSection(request, tableSnapshot)
    fireEvent.click(screen.getByRole('checkbox', { name: 'Rule between rows' }))
    await waitFor(() => expect(commandsSent(request)).toHaveLength(1))
    expect(commandsSent(request)[0]).toBe('{"kind":"updateTableRules","version":1,"id":"e7","field":"between","op":"set","value":["rows"]}')
  })

  it('commits Table Editor cell padding to the table through updateComponentProperties: set 4, then clear', async () => {
    // The second and later projections carry the committed 4pt, so the emptied
    // box differs from what the document declares and sends a clear.
    const { request, tableSnapshot } = headerStyledTable({}, { paddingLeft: '4000' })
    await openHeaderSection(request, tableSnapshot)
    fireEvent.blur(screen.getByRole('textbox', { name: 'Cell padding left in points' }), { target: { value: '4' } })
    await waitFor(() => expect(commandsSent(request)).toHaveLength(1))
    expect(commandsSent(request)[0]).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e7"],"changes":{"paddingLeft":{"op":"set","value":4}}}')
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Cell padding left in points' })).toHaveValue('4'))
    fireEvent.blur(screen.getByRole('textbox', { name: 'Cell padding left in points' }), { target: { value: '' } })
    await waitFor(() => expect(commandsSent(request)).toHaveLength(2))
    expect(commandsSent(request)[1]).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e7"],"changes":{"paddingLeft":{"op":"clear"}}}')
  })

  it('leaves the matrix untouched: seven columns and its arrow navigation still work with the new section present', async () => {
    const { request, tableSnapshot } = headerStyledTable()
    await openHeaderSection(request, tableSnapshot)
    const grid = screen.getByRole('grid', { name: 'Table columns' })
    expect(grid).toHaveAttribute('aria-colcount', '7')
    // Row-field authoring is one stop between the label and width.
    const header = screen.getByRole('textbox', { name: 'Header for column 1' })
    header.focus(); fireEvent.keyDown(header, { key: 'ArrowRight' })
    expect(document.activeElement).toBe(screen.getByRole('combobox', { name: 'Binding for column 1' }))
    fireEvent.keyDown(document.activeElement!, { key: 'ArrowRight', altKey: true })
    expect(document.activeElement).toBe(screen.getByRole('spinbutton', { name: 'Width for column 1 in points' }))
    fireEvent.keyDown(document.activeElement!, { key: 'ArrowLeft' })
    expect(document.activeElement).toBe(screen.getByRole('combobox', { name: 'Binding for column 1' }))
    fireEvent.keyDown(document.activeElement!, { key: 'ArrowLeft', altKey: true })
    expect(document.activeElement).toBe(header)
    // Home reaches the row's FIRST ENABLED control, which on a one-column table
    // is `Remove column 1`: both reorder affordances are disabled at both ends
    // of a single row, and Home declines to land on either.
    fireEvent.keyDown(document.activeElement!, { key: 'Home' })
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Remove column 1' }))
    // And nothing was committed by merely opening the panel: an author who
    // changes nothing must leave the document alone.
    expect(commandsSent(request)).toEqual([])
  })

  it('replaces the canvas with Preview, cancels an older render, and never dirties or installs its late PDF', async () => {
    let releaseSerialize!: (value: { snapshot: ReturnType<typeof snapshot>; bytes: ArrayBuffer }) => void
    const request = vi.fn((operation: string) => {
      if (operation === 'identity') return Promise.resolve({ snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } })
      if (operation === 'serialize') return new Promise<{ snapshot: ReturnType<typeof snapshot>; bytes: ArrayBuffer }>((resolve) => { releaseSerialize = resolve })
      if (operation === 'render') return Promise.resolve({ snapshot: snapshot(1), bytes: new Uint8Array([9]).buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: PDF_FIXTURE_DIGEST, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } })
      return Promise.resolve({ snapshot: snapshot(1) })
    })
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    expect(screen.queryByLabelText('Canvas region')).not.toBeInTheDocument()
    expect(screen.getByText('Rendering local PDF')).toBeInTheDocument()
    await waitFor(() => expect(request).toHaveBeenCalledWith('serialize', undefined, expect.any(AbortSignal)))
    // STORY 13.5 — DRIVEN FROM THE MODE SWITCH, WHICH IS NOW THE ONLY ONE.
    // The preview heading's second Return-to-Design button was removed; the
    // document bar's DESIGN button carries the SAME `returnToDesign` reference,
    // so what this row proves about cancellation is unchanged.
    fireEvent.click(screen.getByRole('button', { name: 'DESIGN' }))
    releaseSerialize({ snapshot: snapshot(1), bytes })
    await waitFor(() => expect(screen.getByLabelText('Canvas region')).toBeInTheDocument())
    expect(screen.queryByText(/Go production digest/)).not.toBeInTheDocument()
    expect(screen.getByText('Unsaved local changes')).toBeInTheDocument()
    expect(request.mock.calls.map(([operation]) => operation)).toEqual(['parameter-references', 'identity', 'serialize'])
  })

  it('coalesces manual and debounced rerenders behind one active FIFO operation', async () => {
    let releaseIdentity!: () => void
    let identityCalls = 0
    const request = vi.fn((operation: string) => {
      if (operation === 'identity') {
        identityCalls++
        if (identityCalls === 1) return new Promise<{ snapshot: ReturnType<typeof snapshot>; preview: { revision: number; identity: string } }>((resolve) => { releaseIdentity = () => resolve({ snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }) })
        return Promise.resolve({ snapshot: snapshot(1), preview: { revision: 1, identity: 'c'.repeat(64) } })
      }
      if (operation === 'serialize') return Promise.resolve({ snapshot: snapshot(1), bytes })
      if (operation === 'render') return Promise.resolve({ snapshot: snapshot(1), bytes: new Uint8Array([9]).buffer, preview: { revision: 1, identity: 'c'.repeat(64), pdfSha256: PDF_FIXTURE_DIGEST, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } })
      return Promise.resolve({ snapshot: snapshot(1) })
    })
    vi.useFakeTimers()
    try {
      render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
      fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
      fireEvent.change(screen.getByRole('textbox', { name: 'Raw parameter JSON' }), { target: { value: '{"transactions":[1]}' } })
      fireEvent.change(screen.getByRole('textbox', { name: 'Raw parameter JSON' }), { target: { value: '{"transactions":[2]}' } })
      fireEvent.click(screen.getByRole('button', { name: 'Re-render' }))
      // STORY 13.5 — ADVANCED BY A BOUND, NOT DRAINED. Preview now holds a
      // ticking interval that re-arms itself while a render is installed, and
      // `runAllTimersAsync` drains until the queue is EMPTY: against a timer
      // that schedules its own successor that is a loop with no end. A bound
      // well past the 250 ms debounce settles everything this row is about.
      await vi.advanceTimersByTimeAsync(2000)
      expect(request.mock.calls.filter(([operation]) => operation === 'identity')).toHaveLength(1)
      releaseIdentity()
      await vi.advanceTimersByTimeAsync(2000)
      await Promise.resolve(); await Promise.resolve(); await Promise.resolve()
      expect(request.mock.calls.filter(([operation]) => operation === 'identity')).toHaveLength(2)
    } finally {
      vi.useRealTimers()
    }
  })

  it('uses the engine reference projection for keyboard-operable parameter inputs and retains accepted bytes through an invalid draft', async () => {
    const request = vi.fn(async (operation: string) => {
      if (operation === 'parameter-references') return { snapshot: snapshot(1), parameterReferences: { revision: 1, names: ['reportDate'] } }
      if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
      if (operation === 'render') return { snapshot: snapshot(1), bytes: new Uint8Array([9]).buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: PDF_FIXTURE_DIGEST, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
      return { snapshot: snapshot(1) }
    })
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    const named = await screen.findByRole('textbox', { name: 'Value for params.reportDate' })
    fireEvent.change(named, { target: { value: '"2026-08-28T00:00:00Z"' } })
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'identity')).toHaveLength(2))
    const accepted = request.mock.calls.filter(([operation]) => operation === 'identity').at(-1)! as unknown as [string, { params: ArrayBuffer }]
    expect(new TextDecoder().decode(accepted[1].params)).toContain('2026-08-28T00:00:00Z')
    fireEvent.change(screen.getByRole('textbox', { name: 'Raw parameter JSON' }), { target: { value: '{ nope' } })
    expect(screen.getByRole('alert')).toHaveTextContent('last accepted parameter document remains in Preview')
    expect(screen.getByText('Unsaved local changes')).toBeInTheDocument()
    expect(request.mock.calls.filter(([operation]) => operation === 'command')).toHaveLength(0)
  })

  it('states pending, failed, and empty parameter discovery without inventing fields', async () => {
    let release!: () => void
    const pending = new Promise<{ snapshot: ReturnType<typeof snapshot>; parameterReferences: { revision: number; names: string[] } }>((resolve) => { release = () => resolve({ snapshot: snapshot(1), parameterReferences: { revision: 1, names: [] } }) })
    const request = vi.fn((operation: string) => operation === 'parameter-references' ? pending : Promise.resolve(operation === 'identity' ? { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } } : { snapshot: snapshot(1) }))
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    expect(screen.getByText('Discovering parameter references from the local engine…')).toBeInTheDocument()
    release()
    await screen.findByText('The local engine found no parameter references in this template.')
  })

  it('states a failed parameter projection rather than calling it an empty projection', async () => {
    const request = vi.fn(async (operation: string) => {
      if (operation === 'parameter-references') throw new Error('worker unavailable')
      if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
      return { snapshot: snapshot(1) }
    })
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await screen.findByText('The local engine could not provide parameter references. The raw parameter document is still available.')
    expect(screen.queryByText('The local engine found no parameter references in this template.')).not.toBeInTheDocument()
  })

  it('edits named parameters without rewriting raw numeric lexemes or special own keys', async () => {
    const request = vi.fn(async (operation: string) => {
      if (operation === 'parameter-references') return { snapshot: snapshot(1), parameterReferences: { revision: 1, names: ['__proto__', 'constructor', 'reportDate'] } }
      if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
      if (operation === 'render') return { snapshot: snapshot(1), bytes: new Uint8Array([9]).buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: PDF_FIXTURE_DIGEST, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
      return { snapshot: snapshot(1) }
    })
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    const reportDate = await screen.findByRole('textbox', { name: 'Value for params.reportDate' })
    fireEvent.change(screen.getByRole('textbox', { name: 'Raw parameter JSON' }), { target: { value: '{"constructor":1.00e+2,"__proto__":-0,"other":123.4500}' } })
    expect(screen.getByRole('textbox', { name: 'Value for params.constructor' })).toHaveValue('1.00e+2')
    expect(screen.getByRole('textbox', { name: 'Value for params.__proto__' })).toHaveValue('-0')
    reportDate.focus()
    fireEvent.change(reportDate, { target: { value: '"2026-08-28T00:00:00Z"' } })
    expect(document.activeElement).toBe(reportDate)
    fireEvent.click(screen.getByRole('button', { name: 'Re-render' }))
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'identity').length).toBeGreaterThan(1))
    const accepted = request.mock.calls.filter(([operation]) => operation === 'identity').at(-1)! as unknown as [string, { params: ArrayBuffer }]
    const exact = '{"constructor":1.00e+2,"__proto__":-0,"other":123.4500,"reportDate":"2026-08-28T00:00:00Z"}'
    expect(new TextDecoder().decode(accepted[1].params)).toBe(exact)
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'render').length).toBeGreaterThan(0))
    const rendered = request.mock.calls.filter(([operation]) => operation === 'render').at(-1)! as unknown as [string, { params: ArrayBuffer }]
    expect(new TextDecoder().decode(rendered[1].params)).toBe(exact)
  })

  it('refreshes the engine reference projection after Undo while Preview remains open', async () => {
    let references = 0
    const historySnapshot = { ...snapshot(2), canUndo: false, canRedo: true }
    const initial = { ...snapshot(1), canUndo: true, canRedo: false }
    const request = vi.fn(async (operation: string) => {
      if (operation === 'parameter-references') {
        references++
        return { snapshot: references === 1 ? initial : historySnapshot, parameterReferences: { revision: references === 1 ? 1 : 2, names: references === 1 ? ['reportDate'] : ['branch'] } }
      }
      if (operation === 'undo') return { snapshot: historySnapshot }
      if (operation === 'identity') return { snapshot: references > 1 ? historySnapshot : initial, preview: { revision: references > 1 ? 2 : 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: references > 1 ? historySnapshot : initial, bytes }
      if (operation === 'render') return { snapshot: references > 1 ? historySnapshot : initial, bytes: new Uint8Array([9]).buffer, preview: { revision: references > 1 ? 2 : 1, identity: 'b'.repeat(64), pdfSha256: PDF_FIXTURE_DIGEST, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
      return { snapshot: initial }
    })
    render(<App engine={engine(request)} initialSnapshot={initial} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await screen.findByRole('textbox', { name: 'Value for params.reportDate' })
    fireEvent.click(screen.getByRole('button', { name: /^Undo/ }))
    await screen.findByRole('textbox', { name: 'Value for params.branch' })
    expect(screen.queryByRole('textbox', { name: 'Value for params.reportDate' })).not.toBeInTheDocument()
    expect(request.mock.calls.filter(([operation]) => operation === 'parameter-references')).toHaveLength(2)
  })

  it('waits for matching PDF.js admission before claiming current exact output', async () => {
    const request = vi.fn(async (operation: string) => {
      if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
      if (operation === 'render') return { snapshot: snapshot(1), bytes: new Uint8Array([9]).buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: PDF_FIXTURE_DIGEST, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
      return { snapshot: snapshot(1) }
    })
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Stale historical PDF/ })).toBeInTheDocument())
    expect(document.getElementById('preview-freshness-status')).toHaveClass('preview-status')
    expect(screen.queryByText('EXACT LOCAL PRODUCTION PDF')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Stale historical PDF/ }))
    expect(screen.queryByText('EXACT LOCAL PRODUCTION PDF')).not.toBeInTheDocument()
    expect(document.getElementById('preview-freshness-status')).toHaveClass('sr-only')
    expect(screen.getByRole('button', { name: /Current exact local production PDF/ })).toHaveAttribute('aria-describedby', 'preview-freshness-status')
  })

  it('keeps producer diagnostics hidden and inert until their exact PDF is admitted, then revokes them on input change', async () => {
    const request = vi.fn(async (operation: string) => {
      if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
      if (operation === 'render') return { snapshot: snapshot(1), bytes: new Uint8Array([9]).buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: PDF_FIXTURE_DIGEST, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [{ severity: 'warning' as const, code: 'CONTENT_CLIPPED', elementId: 'gone', dataPath: 'bands.content.gone', message: 'Content was clipped' }] } }
      return { snapshot: snapshot(1) }
    })
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Stale historical PDF/ })).toBeInTheDocument())
    expect(screen.queryByLabelText('Render diagnostics')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Stale historical PDF/ }))
    await waitFor(() => expect(screen.getByLabelText('Render diagnostics')).toBeInTheDocument())
    fireEvent.change(screen.getByRole('textbox', { name: 'Raw parameter JSON' }), { target: { value: '{"transactions":[1]}' } })
    expect(screen.queryByLabelText('Render diagnostics')).not.toBeInTheDocument()
  })

  it('returns to Design and announces an unavailable authoritative warning target without selecting a substitute', async () => {
    const request = vi.fn(async (operation: string) => {
      if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
      if (operation === 'render') return { snapshot: snapshot(1), bytes: new Uint8Array([9]).buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: PDF_FIXTURE_DIGEST, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [{ severity: 'warning' as const, code: 'CONTENT_CLIPPED', elementId: 'gone', dataPath: '', message: 'Content was clipped' }] } }
      return { snapshot: snapshot(1) }
    })
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Stale historical PDF/ })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Stale historical PDF/ }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Locate on canvas' })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Locate on canvas' }))
    await waitFor(() => expect(screen.getByLabelText('Canvas region')).toBeInTheDocument())
    expect(screen.getByText('Locate unavailable: the authoritative element is no longer present.')).toHaveAttribute('role', 'status')
  })

  it('returns from a path-only render failure without requiring an element id', async () => {
    const failure = Object.assign(new Error('The template could not be processed'), { code: 'RENDER_INVALID', dataPath: 'items[0]', producerRenderFailure: true as const })
    const request = vi.fn(async (operation: string) => {
      if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
      if (operation === 'render') throw failure
      return { snapshot: snapshot(1) }
    })
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByLabelText('Local render failure')).toBeInTheDocument())
    fireEvent.click(within(screen.getByLabelText('Local render failure')).getByRole('button', { name: 'Return to Design' }))
    await waitFor(() => expect(screen.getByLabelText('Canvas region')).toBeInTheDocument())
  })

  it('returns from a located render failure by selecting only the current projected element', async () => {
    const failure = Object.assign(new Error('The template could not be processed'), { code: 'RENDER_INVALID', elementId: 'e7', producerRenderFailure: true as const })
    const locatedCanvas = { ...canvas, components: [{ id: 'e7', type: 'rect' as const, band: 'content' as const, x: 0, y: 0, width: 72000, height: 12000, resizable: true }] }
    const locatedSnapshot = { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: locatedCanvas }
    const request = vi.fn(async (operation: string) => {
      if (operation === 'identity') return { snapshot: locatedSnapshot, preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: locatedSnapshot, bytes }
      if (operation === 'render') throw failure
      return { snapshot: locatedSnapshot }
    })
    render(<App engine={engine(request)} initialSnapshot={locatedSnapshot} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    const card = await screen.findByLabelText('Local render failure')
    fireEvent.click(within(card).getByRole('button', { name: 'Return to Design' }))
    const component = await screen.findByRole('button', { name: 'rect component e7' })
    expect(component).toHaveClass('canvas-component-selected')
    expect(screen.getByText('Selected e7 in Design.')).toHaveAttribute('role', 'status')
  })

  it('retries an active failed render through the existing scheduler without mutating the document', async () => {
    let renders = 0
    const failure = Object.assign(new Error('The template could not be processed'), { code: 'RENDER_INVALID', elementId: 'e7', dataPath: 'params.reportDate', producerRenderFailure: true as const })
    const request = vi.fn(async (operation: string) => {
      if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
      if (operation === 'render') { renders++; if (renders === 1) throw failure; return { snapshot: snapshot(1), bytes: new Uint8Array([9]).buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: PDF_FIXTURE_DIGEST, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } } }
      return { snapshot: snapshot(1) }
    })
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    const card = await screen.findByLabelText('Local render failure')
    expect(card).toHaveTextContent('RENDER_INVALID')
    fireEvent.click(within(card).getByRole('button', { name: 'Retry preview' }))
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'render')).toHaveLength(2))
    expect(request.mock.calls.filter(([operation]) => operation === 'command')).toHaveLength(0)
    expect(screen.queryByLabelText('Local render failure')).not.toBeInTheDocument()
    expect(screen.getByText('Unsaved local changes')).toBeInTheDocument()
  })

  it('forces a fresh FIFO render after a same-identity last-good PDF failure and retains that PDF as stale', async () => {
    let renders = 0
    const failure = Object.assign(new Error('The template could not be processed'), { code: 'RENDER_INVALID', elementId: 'e7', producerRenderFailure: true as const })
    const request = vi.fn(async (operation: string) => {
      if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
      if (operation === 'render') {
        renders++
        if (renders > 1) throw failure
        return { snapshot: snapshot(1), bytes: new Uint8Array([9]).buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: PDF_FIXTURE_DIGEST, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
      }
      return { snapshot: snapshot(1) }
    })
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    const admitted = await screen.findByRole('button', { name: /Stale historical PDF/ })
    fireEvent.click(admitted)
    await waitFor(() => expect(screen.getByRole('button', { name: /Current exact local production PDF/ })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Fail local PDF viewer' }))
    expect(screen.queryByLabelText('Local render failure')).not.toBeInTheDocument()
    expect(screen.getByText(/local PDF viewer could not display/i)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Re-render' }))
    const card = await screen.findByLabelText('Local render failure')
    expect(screen.getByRole('button', { name: /Stale historical PDF/ })).toBeInTheDocument()
    const retry = within(card).getByRole('button', { name: 'Retry preview' })
    retry.focus()
    fireEvent.keyDown(retry, { key: 'Enter' })
    fireEvent.click(retry)
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'render')).toHaveLength(3))
    expect(request.mock.calls.filter(([operation]) => operation === 'identity')).toHaveLength(3)
    expect(request.mock.calls.filter(([operation]) => operation === 'serialize')).toHaveLength(3)
    expect(request.mock.calls.filter(([operation]) => operation === 'command')).toHaveLength(0)
    expect(screen.getByLabelText('Local render failure')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Stale historical PDF/ })).toBeInTheDocument()
  })

  it('revokes a delayed render failure after leaving Preview so its actions cannot reach Design', async () => {
    let rejectRender!: (error: Error) => void
    const request = vi.fn((operation: string) => {
      if (operation === 'identity') return Promise.resolve({ snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } })
      if (operation === 'serialize') return Promise.resolve({ snapshot: snapshot(1), bytes })
      if (operation === 'render') return new Promise<never>((_, reject: (error: Error) => void) => { rejectRender = reject })
      return Promise.resolve({ snapshot: snapshot(1) })
    })
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'render')).toHaveLength(1))
    // STORY 13.5 — DRIVEN FROM THE MODE SWITCH, WHICH IS NOW THE ONLY ONE.
    // The preview heading's second Return-to-Design button was removed; the
    // document bar's DESIGN button carries the SAME `returnToDesign` reference,
    // so what this row proves about cancellation is unchanged.
    fireEvent.click(screen.getByRole('button', { name: 'DESIGN' }))
    rejectRender(Object.assign(new Error('The template could not be processed'), { code: 'RENDER_INVALID', elementId: 'e7', producerRenderFailure: true as const }))
    await waitFor(() => expect(screen.getByLabelText('Canvas region')).toBeInTheDocument())
    expect(screen.queryByLabelText('Local render failure')).not.toBeInTheDocument()
    expect(request.mock.calls.filter(([operation]) => operation === 'command')).toHaveLength(0)
  })

  it('names local file controls, persistent unsaved state, and offline availability', () => {
    render(<App />)
    const open = screen.getByRole('button', { name: 'Open local template' })
    expect(open).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Save local template' })).toBeInTheDocument()
    expect(screen.getByText('Unsaved local changes')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'New…' })).toBeDisabled()
    expect(screen.getByRole('status', { name: 'Offline availability' })).toHaveTextContent('Offline cache unavailable')
  })

  // THE DOCUMENT BAR AND THE CANVAS TOOLBAR ARE GLYPH CONTROLS WITH A HOVER GUIDE.
  //
  // Story 14.1 spelled the document bar as words; the owner has since ruled both
  // toolbars glyphs. `control-vocabulary-contract.test.tsx` holds the family to
  // one treatment — it cannot say "this one still answers to its old name, and
  // its hover guide names its shortcut", which is what this row asserts.
  //
  // ⚠ EVERY ACCESSIBLE NAME IS THE ONE THE CONTROL HAD AS A WORD, so no unit or
  // e2e query moved. The guide is `data-tip`, painted by CSS, and there is no
  // `title` — the browser's own tooltip would draw a second one over it.
  it('draws all six local-file controls as glyphs in one named group, keeping every accessible name', () => {
    render(<App />)
    const shortcuts = shortcutHintsFor()
    const group = screen.getByRole('group', { name: 'Local file actions' })
    const buttons = within(group).getAllByRole('button')
    expect(buttons.map((button) => [button.getAttribute('aria-label'), button.getAttribute('data-tip')])).toEqual([
      ['Open local template', 'Open'],
      ['Save local template', `Save (${shortcuts.save})`],
      ['Save As', 'Save As'],
      ['New…', 'New…'],
      ['Undo', `Undo (${shortcuts.undo})`],
      ['Redo', `Redo (${shortcuts.redo})`],
    ])
    for (const button of buttons) {
      expect(button).toHaveAccessibleName(button.getAttribute('aria-label')!)
      expect(button.querySelector('svg.tool-icon[aria-hidden="true"]')).not.toBeNull()
      expect(button).toHaveTextContent(/^$/)
      expect(button).not.toHaveAttribute('title')
    }
  })

  // THE DOCUMENTATION LINK. Its own group, AFTER the mode switch and last in the
  // bar, so `Local file actions` above stays exactly six. It is a real link to
  // the bundled guide, opened in a new tab so the editor's state survives, and
  // it is enabled with no engine, no file access and no template — every one of
  // the file actions beside it is disabled in this very render.
  it('offers the bundled rendering library guide as an always-enabled glyph link in its own group after the mode switch', () => {
    render(<App />)
    const bar = screen.getByRole('banner', { name: 'Document bar' })
    const group = within(bar).getByRole('group', { name: 'Documentation' })
    expect(within(group).queryAllByRole('button')).toEqual([])
    const links = within(group).getAllByRole('link')
    expect(links).toHaveLength(1)
    const link = links[0]!
    expect(link).toHaveAccessibleName('Rendering library documentation')
    expect(link).toHaveAttribute('aria-label', 'Rendering library documentation')
    expect(link).toHaveAttribute('data-tip', 'Documentation')
    expect(link).not.toHaveAttribute('title')
    expect(documentationAssetUrls.guide).toMatch(/rendering-library-[a-f0-9]{20}\.html$/)
    expect(link).toHaveAttribute('href', documentationAssetUrls.guide)
    expect(link).toHaveAttribute('target', '_blank')
    expect(link).toHaveAttribute('rel', 'noopener noreferrer')
    expect(link).not.toHaveAttribute('aria-disabled')
    expect(link).not.toHaveAttribute('tabindex')
    expect(link.querySelector('svg.tool-icon[aria-hidden="true"]')).not.toBeNull()
    expect(link).toHaveTextContent(/^$/)
    // Independent of engine and template: the file actions are disabled here.
    expect(screen.getByRole('button', { name: 'Open local template' })).toBeDisabled()
    // Placement: immediately after the mode switch, and the bar's last item.
    expect(within(bar).getByRole('group', { name: 'Designer mode' }).nextElementSibling).toBe(group)
    expect(bar.lastElementChild).toBe(group)
    expect(within(screen.getByRole('group', { name: 'Local file actions' })).queryAllByRole('link')).toEqual([])
    // Keyboard: the link takes focus.
    link.focus()
    expect(link).toHaveFocus()
  })

  it('pins a labelled documentation link to the foot of the component palette', () => {
    render(<App />)
    const palette = screen.getByRole('navigation', { name: 'Component palette' })
    const link = within(palette).getByRole('link', { name: 'Documentation' })
    expect(link).toHaveAttribute('href', documentationAssetUrls.guide)
    expect(link).toHaveAttribute('target', '_blank')
    expect(link).toHaveAttribute('rel', 'noopener noreferrer')
    expect(palette.lastElementChild).toBe(link)
  })

  it('draws the canvas toolbar as glyphs whose hover guide names each shortcut', () => {
    render(<App />)
    const shortcuts = shortcutHintsFor()
    const tools = screen.getByLabelText('Canvas controls')
    expect(within(tools).getAllByRole('button').map((button) => [button.getAttribute('aria-label'), button.getAttribute('data-tip')])).toEqual([
      ['Zoom out', 'Zoom out'],
      ['Zoom in', 'Zoom in'],
      ['Grid on', 'Grid on'],
      ['Snap on', `Snap on (${shortcuts.snap})`],
      ['Duplicate', `Duplicate (${shortcuts.duplicate})`],
      ['Delete', `Delete (${shortcuts.delete} key)`],
      // SPEC-multi-pages story 2 (D-2.3): the two page buttons join the glyph
      // toolbar. With no document yet, each states why it is disabled.
      ['Add page', 'Add page: no document is open'],
      ['Delete page', 'Delete page: no document is open'],
    ])
    for (const button of within(tools).getAllByRole('button')) {
      expect(button.querySelector('svg.tool-icon[aria-hidden="true"]')).not.toBeNull()
      expect(button).toHaveTextContent(/^$/)
    }
    expect(within(tools).getByRole('img', { name: `Nudge (${shortcuts.nudge})` })).toHaveAttribute('data-tip', `Nudge (${shortcuts.nudge})`)
    fireEvent.click(within(tools).getByRole('button', { name: 'Grid on' }))
    expect(within(tools).getByRole('button', { name: 'Grid off' })).toHaveAttribute('data-tip', 'Grid off')
  })

  // STORY 14.5 / AC1 + AC4 — THE MARK IS SEEN AND NEVER HEARD.
  //
  // This is an ABSENCE claim, and an absence cannot be falsified by reverting
  // the implementation: deleting the mark makes "no second announcement" pass
  // more easily, not less (D-14.4.3). So the fence is built to red on an
  // ADDITION — give the `<svg>` `role="img" aria-label="folio8"` and both the
  // role sweep and the contributed-name sweep go red.
  //
  // ⚠ THE ROLE SWEEP PASSES `hidden: true` DELIBERATELY. Testing Library's role
  // queries exclude `aria-hidden` subtrees by default, so the default spelling
  // would stay GREEN against exactly the mutation this test exists to catch —
  // a `role="img"` added while `aria-hidden` is still in place. Both spellings
  // are asserted: the default one says the mark is out of the tree, the
  // `hidden: true` one says it carries no role to expose in the first place.
  it('wears the brand mark before the word FOLIO8, and the mark announces nothing', () => {
    render(<App />)
    const lockup = screen.getByLabelText('Document bar').querySelector('.brand-lockup')
    expect(lockup, 'the document bar must carry the mark-and-word lockup').not.toBeNull()
    const svg = lockup!.querySelector('svg')
    expect(svg, 'the lockup must contain the inline mark').not.toBeNull()

    // ⚠ THE ROOT IS SWEPT ALONGSIDE ITS DESCENDANTS. `querySelectorAll` and
    // `within(...)` both EXCLUDE the element they are called on, so a
    // `role="img" aria-label="folio8"` placed on the LOCKUP ITSELF escapes every
    // descendant-scoped fence. That is not a hypothetical: `role="img"` makes
    // the children presentational, so on the load screen the same mutation
    // would have AT announce "folio8" in place of "FOLIO8 / OFFLINE" — silencing
    // the offline state this screen exists to report.
    const namedNodesIn = (root: Element) => [root, ...Array.from(root.querySelectorAll('*'))]
      .filter((node) => ['aria-label', 'aria-labelledby', 'role', 'title'].some((attribute) => node.hasAttribute(attribute)) || node.tagName.toLowerCase() === 'title')
      .map((node) => `${node.tagName.toLowerCase()}${node.getAttribute('role') ? `[role=${node.getAttribute('role')}]` : ''}`)

    // (a) decorative, (d) the document bar's declared size
    expect(svg).toHaveAttribute('aria-hidden', 'true')
    expect(svg).toHaveAttribute('width', '18')
    expect(svg).toHaveAttribute('height', '18')
    // The class is the only join between `--color-select` and `currentColor`.
    expect(svg!.getAttribute('class'), 'the .brand-mark rule reaches the SVG through this attribute alone').toBe('brand-mark')

    // (b) no role of its own, in either spelling of the sweep
    expect(within(lockup as HTMLElement).queryAllByRole('img')).toEqual([])
    expect(within(lockup as HTMLElement).queryAllByRole('img', { hidden: true }), 'the mark must carry no role at all, not merely a role hidden from the tree').toEqual([])

    // (c) the pair's accessible text is the wordmark ALONE. Hidden subtrees
    // contribute no text, and nothing contributes a name of its own — the
    // second half is what an added `aria-label` reds.
    const announced = lockup!.cloneNode(true) as Element
    for (const hidden of Array.from(announced.querySelectorAll('[aria-hidden="true"]'))) hidden.remove()
    expect(announced.textContent?.replace(/\s+/g, ' ').trim(), 'the mark and the word announce the product name once').toBe('FOLIO8')
    expect(namedNodesIn(lockup!), 'nothing in the lockup — the wrapper INCLUDED — may contribute a name of its own').toEqual([])

    // (AC1) the mark is drawn BEFORE the word, provable without a browser.
    expect(lockup!.firstElementChild, 'the mark is the first child; the word follows it').toBe(svg)
    expect(svg!.compareDocumentPosition(lockup!.querySelector('.brand')!) & Node.DOCUMENT_POSITION_FOLLOWING, 'the wordmark must follow the mark in document order').toBeTruthy()

    // The word itself is untouched by this story.
    expect(lockup!.querySelector('.brand')).toHaveTextContent('FOLIO8')
  })

  it('labels the development bypass instead of claiming a verified cache', () => {
    render(<App offlineState="dev-bypass" />)
    expect(screen.getByRole('status', { name: 'Offline availability' })).toHaveTextContent('Offline layer bypassed (dev)')
  })

  it('announces the checking, ready, and waiting-update lifecycle states', () => {
    const { rerender } = render(<App offlineState="checking" />)
    const status = screen.getByRole('status', { name: 'Offline availability' })
    expect(status).toHaveAttribute('aria-live', 'polite')
    expect(status).toHaveTextContent('Offline cache checking')
    rerender(<App offlineState="ready" />)
    expect(status).toHaveTextContent('Offline ready')
    rerender(<App offlineState="update-available" />)
    expect(status).toHaveTextContent('Update available; current release remains usable')
  })

  it('bypasses S1 when the current cache and engine are already ready', () => {
    render(<App loadState={{ state: 'ready', cacheReady: true, verifiedAssetUrls: [] }} engineState="starting" />)
    expect(screen.getByRole('status', { name: 'Engine preparation status' })).toHaveTextContent('Starting local engine')
    expect(screen.queryByRole('heading', { name: 'Preparing folio8' })).not.toBeInTheDocument()
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
  })

  it('loads only opaque adapter bytes through Go, establishes a clean baseline, and dirties after a committed command', async () => {
    const request = vi.fn(async (operation: string) => ({ snapshot: snapshot(operation === 'command' ? 8 : 7), ...(operation === 'serialize' ? { bytes } : {}) }))
    const files: FileAccess = { open: vi.fn(async () => ({ bytes, name: 'report.folio' })), acquireSaveTarget: vi.fn(), writeSave: vi.fn() }
    render(<App engine={engine(request)} fileAccess={files} initialSnapshot={snapshot(1)} />)
    fireEvent.click(screen.getByRole('button', { name: 'Open local template' }))
    await waitFor(() => expect(screen.getByText('report.folio')).toBeInTheDocument())
    expect(request.mock.calls.map(([operation]) => operation)).toEqual(['load', 'serialize'])
    expect(screen.getByText('Saved local file')).toBeInTheDocument()
    fireEvent.change(screen.getByRole('textbox', { name: 'Top margin (pt)' }), { target: { value: '37' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(screen.getByText('Unsaved local changes')).toBeInTheDocument())
  })

  it('keeps zoom, grid, and snap local while an explicit Go page-setup command alone dirties the document', async () => {
    const request = vi.fn(async (operation: string) => ({ snapshot: snapshot(operation === 'command' ? 2 : 1), ...(operation === 'serialize' ? { bytes } : {}) }))
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} />)
    fireEvent.click(screen.getByRole('button', { name: 'Zoom in' }))
    fireEvent.click(screen.getByRole('button', { name: 'Grid on' }))
    fireEvent.click(screen.getByRole('button', { name: 'Snap on' }))
    expect(request).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Grid off' })).toHaveAttribute('aria-pressed', 'false')
    expect(screen.getByRole('button', { name: 'Snap off' })).toHaveAttribute('aria-pressed', 'false')
    fireEvent.change(screen.getByRole('textbox', { name: 'Top margin (pt)' }), { target: { value: '37.125' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(request).toHaveBeenCalledTimes(1))
    expect(request.mock.calls[0]![0]).toBe('command')
    const command = request.mock.calls[0] as unknown as [string, ArrayBuffer]
    expect(new TextDecoder().decode(command[1])).toContain('"top":37.125')
    expect(screen.getByText('Unsaved local changes')).toBeInTheDocument()
  })

  it('drives the full authoritative undo/redo depth, bounds, and divergent branch from engine snapshots', async () => {
    let revision = 1
    let undoDepth = 0
    let redoDepth = 0
    const historySnapshot = () => ({ ...snapshot(revision), canUndo: undoDepth > 0, canRedo: redoDepth > 0 })
    const request = vi.fn(async (operation: string) => {
      if (operation === 'command') { revision++; undoDepth++; redoDepth = 0; return { snapshot: historySnapshot() } }
      if (operation === 'undo') { revision++; undoDepth--; redoDepth++; return { snapshot: historySnapshot() } }
      if (operation === 'redo') { revision++; undoDepth++; redoDepth--; return { snapshot: historySnapshot() } }
      return { snapshot: historySnapshot(), ...(operation === 'serialize' ? { bytes } : {}) }
    })
    render(<App engine={engine(request)} initialSnapshot={historySnapshot()} />)
    fireEvent.change(screen.getByRole('textbox', { name: 'Top margin (pt)' }), { target: { value: '37' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Undo' })).toBeEnabled())
    fireEvent.change(screen.getByRole('textbox', { name: 'Top margin (pt)' }), { target: { value: '38' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'command')).toHaveLength(2))
    fireEvent.click(screen.getByRole('button', { name: 'Undo' }))
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'undo')).toHaveLength(1))
    expect(screen.getByRole('button', { name: 'Undo' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Redo' })).toBeEnabled()
    fireEvent.click(screen.getByRole('button', { name: 'Undo' }))
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'undo')).toHaveLength(2))
    expect(screen.getByRole('button', { name: 'Undo' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Redo' }))
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'redo')).toHaveLength(1))
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'command')).toHaveLength(3))
    expect(screen.getByRole('button', { name: 'Redo' })).toBeDisabled()
  })

  it('keeps a no-op command non-dirty and out of browser history when the engine returns its stable snapshot', async () => {
    const request = vi.fn(async () => ({ snapshot: { ...snapshot(1), canUndo: false, canRedo: true } }))
    render(<App engine={engine(request)} initialSnapshot={{ ...snapshot(1), canUndo: false, canRedo: true }} />)
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(screen.getByRole('button', { name: 'Undo' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Redo' })).toBeEnabled()
  })

  it.each([
    [true, { save: '⌘S', undo: '⌘Z', redo: '⇧⌘Z', preview: '⌥P', snap: '⌥S' }],
    [false, { save: 'Ctrl+S', undo: 'Ctrl+Z', redo: 'Ctrl+Y', preview: 'Alt+P', snap: 'Alt+S' }],
  ])('uses one platform-normalized shortcut map (%s)', (mac, expected) => {
    expect(shortcutHintsFor(mac)).toMatchObject(expected)
  })

  it.each([
    ['property draft', { key: 'z', ctrlKey: true }],
    ['IME composition', { key: 'z', ctrlKey: true, isComposing: true }],
  ])('does not route Undo through an editable %s', (_name, keyboard) => {
    const request = vi.fn(async () => ({ snapshot: { ...snapshot(1), canUndo: false, canRedo: true } }))
    render(<App engine={engine(request)} initialSnapshot={{ ...snapshot(1), canUndo: true }} />)
    fireEvent.keyDown(screen.getByRole('textbox', { name: 'Top margin (pt)' }), keyboard)
    expect(request).not.toHaveBeenCalled()
  })

  // -------------------------------------------------------------------------
  // STORY 14.7b / DW-368 — THE GLOBAL SHORTCUT STOPS AT AN OPEN MODAL.
  //
  // The guard above answers "is the TARGET editable?". Inside an open modal the
  // author is very often focused on a BUTTON, which is not editable, so every
  // shortcut below the guard line used to fire straight through the dialog at
  // the document behind it. Measured at 68aa91f: the arrow keys sent
  // `moveComponent id:"e7"` and Cmd+D sent `duplicateComponent id:"e7"` — and
  // `e7` is THE VERY TABLE the dialog had open, because both arms require a
  // single selection and `openTableEditor` requires that selection to be the
  // table it edits. So the dialog projected one table while the document held
  // two, with nothing on screen to show it. Cmd+Z was different and no better:
  // it destroyed the dialog out from under the author.
  //
  // These proofs are the counterpart of the count: `Cancel`'s bound of
  // MAX_ENGINE_HISTORY_ENTRIES undos is sound only while the dialog's own
  // commands are the only source of history entries. A leaked nudge pushes an
  // entry the dialog never counted, and the sequence would then leave one of the
  // dialog's own edits standing.
  // -------------------------------------------------------------------------
  const modalTableCanvas = { ...canvas, components: [{ id: 'e7', type: 'table' as const, band: 'content' as const, x: 0, y: 0, width: 72000, height: 12000, resizable: false }] }
  // canUndo AND canRedo are BOTH true, and a single component is selected below,
  // so every suppressed arm is one that WOULD have fired. A snapshot with no
  // history would make these assertions pass against a guard that does nothing.
  const modalTableSnapshot = { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: modalTableCanvas, canUndo: true, canRedo: true }
  const modalTableRequest = () => vi.fn(async (operation: string) => operation === 'table-columns'
    ? { snapshot: modalTableSnapshot, tableColumns: { revision: 1, table: { tableId: 'e7', collection: 'items[]', alias: 'row', ...tableHeaderProjection, columns: [{ id: 'e8', header: 'Amount', width: 72000, proportion: '', align: 'left' as const, headerAlign: '' as const, headerAlignResolved: 'left' as const, binding: '{{row.amount}}', rowField: 'amount', rowFieldEditable: true, footer: '' as const, footerOf: '', footerFormat: '' }] } } }
    : { snapshot: modalTableSnapshot, ...(operation === 'serialize' ? { bytes } : {}) })
  const openTableEditorOver = async (fileAccess?: FileAccess) => {
    const request = modalTableRequest()
    render(<App engine={engine(request)} {...(fileAccess ? { fileAccess } : {})} initialSnapshot={modalTableSnapshot} />)
    fireEvent.click(screen.getByRole('button', { name: 'table component e7' }))
    fireEvent.click(screen.getByRole('button', { name: 'Configure columns' }))
    await screen.findByRole('dialog', { name: 'Table Editor' })
    return request
  }

  it.each([
    ['Undo', { key: 'z', ctrlKey: true }],
    ['Redo (Ctrl+Y)', { key: 'y', ctrlKey: true }],
    ['Redo (Shift+Ctrl+Z)', { key: 'z', ctrlKey: true, shiftKey: true }],
    ['Duplicate', { key: 'd', ctrlKey: true }],
    ['ArrowLeft', { key: 'ArrowLeft' }],
    ['ArrowRight', { key: 'ArrowRight' }],
    ['ArrowUp', { key: 'ArrowUp' }],
    ['ArrowDown', { key: 'ArrowDown' }],
    ['Snap (Alt+S)', { key: 's', altKey: true }],
    ['Preview (Alt+P)', { key: 'p', altKey: true }],
    ['Delete', { key: 'Delete' }],
    ['Backspace', { key: 'Backspace' }],
  ])('sends no %s to the document from a button inside the open table editor', async (_name, keyboard) => {
    const request = await openTableEditorOver()
    const settled = request.mock.calls.length
    const snap = screen.getByRole('button', { name: /^Snap/ }).getAttribute('aria-pressed')
    // A BUTTON, NOT AN INPUT. `isEditableTarget` would already have stopped an
    // input, so a proof taken there would pass against no guard at all.
    const done = screen.getByRole('button', { name: 'Done' })
    done.focus()
    fireEvent.keyDown(done, keyboard)
    await act(async () => { await Promise.resolve() })
    // NO OPERATION AND NO COMMAND. Asserted as the whole tail of the call list
    // rather than as a count of one kind: `moveComponent` and
    // `duplicateComponent` travel as `command`, undo/redo as their own
    // operations, and a per-kind filter would have missed whichever kind the
    // next leak used.
    expect(request.mock.calls.slice(settled)).toEqual([])
    expect(screen.getByRole('dialog', { name: 'Table Editor' })).toBeInTheDocument()
    // The two arms that reach no engine at all still have to be seen not to
    // fire: Alt+S toggles snap in the browser and Alt+P swaps the whole main.
    expect(screen.getByRole('button', { name: /^Snap/ })).toHaveAttribute('aria-pressed', snap)
    expect(screen.getByLabelText('Canvas region')).toBeInTheDocument()
  })

  // THE POSITIVE CONTROLS FOR EVERY ONE OF THOSE TEN ARMS, AND WITHOUT THEM NINE
  // OF THE TEN ASSERT NOTHING. "No command reached the engine" is satisfied just
  // as well by a key that could never have fired — a stale spelling, a state the
  // fixture does not actually hold, a modifier the platform reads differently —
  // so each row below presses the SAME key with NO DIALOG OPEN and shows it does
  // act. The pair is the measurement; either half alone is a shape that passes
  // against a guard that does nothing, or against a shortcut that does nothing.
  //
  // Same fixture, same selection, same snapshot: only the dialog is absent.
  const selectTableWithNoDialog = () => {
    const request = modalTableRequest()
    render(<App engine={engine(request)} initialSnapshot={modalTableSnapshot} />)
    fireEvent.click(screen.getByRole('button', { name: 'table component e7' }))
    // The whole difference from the suppressed rows, asserted rather than
    // assumed: a row that quietly opened the dialog would be measuring the guard
    // twice and the shortcut never.
    expect(screen.queryByRole('dialog', { name: 'Table Editor' })).toBeNull()
    return request
  }
  const sentCommands = (request: ReturnType<typeof modalTableRequest>) =>
    (request.mock.calls as unknown as ReadonlyArray<[string, ArrayBuffer]>).filter(([operation]) => operation === 'command').map(([, payload]) => new TextDecoder().decode(payload))
  const sentOperations = (request: ReturnType<typeof modalTableRequest>, operation: string) => request.mock.calls.filter(([sent]) => sent === operation)

  it.each([
    ['Undo', { key: 'z', ctrlKey: true }, (request: ReturnType<typeof modalTableRequest>) => { expect(sentOperations(request, 'undo')).toHaveLength(1) }],
    // ⚠ Ctrl+Y IS THE NON-MAC REDO SPELLING, and the arm's dependency on that is
    // made explicit here rather than left to the environment. The handler's redo
    // condition is `!mac && modifier && 'y'`, so on a Mac-reporting navigator
    // this row — and the suppressed row above it — would be vacuous: the key
    // could not fire with or without a dialog. `navigator.platform` is pinned to
    // '' for the assertion below, and `isMacPlatform()` is read to prove the pin
    // took, so a jsdom that starts reporting 'MacIntel' reds this line instead of
    // silently emptying two proofs.
    ['Redo (Ctrl+Y)', { key: 'y', ctrlKey: true }, (request: ReturnType<typeof modalTableRequest>) => { expect(isMacPlatform()).toBe(false); expect(sentOperations(request, 'redo')).toHaveLength(1) }],
    ['Redo (Shift+Ctrl+Z)', { key: 'z', ctrlKey: true, shiftKey: true }, (request: ReturnType<typeof modalTableRequest>) => { expect(sentOperations(request, 'redo')).toHaveLength(1) }],
    ['Duplicate', { key: 'd', ctrlKey: true }, (request: ReturnType<typeof modalTableRequest>) => { expect(sentCommands(request).join('')).toContain('"kind":"duplicateComponent"') }],
    // THE NUDGE'S DIRECTION IS PART OF THE CONTROL. `moveComponent` alone would
    // pass for a handler that sent the same command for all four keys, and the
    // table sits at x = 0, y = 0 in this fixture, so one step is ±1 point.
    ['ArrowLeft', { key: 'ArrowLeft' }, (request: ReturnType<typeof modalTableRequest>) => { expect(sentCommands(request)).toEqual(['{"kind":"moveComponent","version":1,"id":"e7","x":-1,"y":0,"snap":false}']) }],
    ['ArrowRight', { key: 'ArrowRight' }, (request: ReturnType<typeof modalTableRequest>) => { expect(sentCommands(request)).toEqual(['{"kind":"moveComponent","version":1,"id":"e7","x":1,"y":0,"snap":false}']) }],
    ['ArrowUp', { key: 'ArrowUp' }, (request: ReturnType<typeof modalTableRequest>) => { expect(sentCommands(request)).toEqual(['{"kind":"moveComponent","version":1,"id":"e7","x":0,"y":-1,"snap":false}']) }],
    ['ArrowDown', { key: 'ArrowDown' }, (request: ReturnType<typeof modalTableRequest>) => { expect(sentCommands(request)).toEqual(['{"kind":"moveComponent","version":1,"id":"e7","x":0,"y":1,"snap":false}']) }],
    // The two that reach no engine at all, so their controls are read off the
    // browser: Snap toggles its own pressed state, Alt+P swaps the whole main.
    ['Snap (Alt+S)', { key: 's', altKey: true }, () => { expect(screen.getByRole('button', { name: /^Snap/ })).toHaveAttribute('aria-pressed', 'false') }],
    ['Preview (Alt+P)', { key: 'p', altKey: true }, () => { expect(screen.queryByLabelText('Canvas region')).toBeNull() }],
    // macOS: Option+S and Option+P type "ß" and "π"; the physical key still matches.
    ['Snap (⌥S on macOS)', { key: 'ß', code: 'KeyS', altKey: true }, () => { expect(screen.getByRole('button', { name: /^Snap/ })).toHaveAttribute('aria-pressed', 'false') }],
    ['Preview (⌥P on macOS)', { key: 'π', code: 'KeyP', altKey: true }, () => { expect(screen.queryByLabelText('Canvas region')).toBeNull() }],
    ['Delete', { key: 'Delete' }, (request: ReturnType<typeof modalTableRequest>) => { expect(sentCommands(request)).toEqual(['{"kind":"deleteComponent","version":1,"id":"e7"}']) }],
    ['Backspace', { key: 'Backspace' }, (request: ReturnType<typeof modalTableRequest>) => { expect(sentCommands(request)).toEqual(['{"kind":"deleteComponent","version":1,"id":"e7"}']) }],
  ])('sends %s to the document when no dialog is open, which is what makes the suppressed arm a measurement', async (_name, keyboard, verify) => {
    const platform = Object.getOwnPropertyDescriptor(window.navigator, 'platform')
    Object.defineProperty(window.navigator, 'platform', { value: '', configurable: true })
    try {
      const request = selectTableWithNoDialog()
      // SNAP STARTS PRESSED, so the Alt+S control below is a flip and not a
      // reading of the initial state.
      expect(screen.getByRole('button', { name: /^Snap/ })).toHaveAttribute('aria-pressed', 'true')
      fireEvent.keyDown(screen.getByLabelText('Canvas region'), keyboard)
      await act(async () => { await Promise.resolve() })
      verify(request)
    } finally {
      if (platform) Object.defineProperty(window.navigator, 'platform', platform)
      else Reflect.deleteProperty(window.navigator, 'platform')
    }
  })

  // -------------------------------------------------------------------------
  // CANVAS KEYBOARD SHORTCUTS: copy, paste, delete, select all.
  // -------------------------------------------------------------------------
  const shortcutComponents = [
    { id: 'e1', type: 'rect' as const, band: 'pageHeader' as const, x: 0, y: 0, width: 12000, height: 12000, resizable: true },
    { id: 'e2', type: 'rect' as const, band: 'content' as const, x: 0, y: 0, width: 12000, height: 12000, resizable: true },
    { id: 'e3', type: 'rect' as const, band: 'content' as const, x: 24000, y: 0, width: 12000, height: 12000, resizable: true },
  ]
  const shortcutSnapshot = (components: typeof shortcutComponents, revision: number) => ({ documentState: 'loaded' as const, revision, byteLength: 3, canvas: { ...canvas, components }, canUndo: true, canRedo: false })
  // Duplicates append a copy per id (named e<revision>a, e<revision>b, …);
  // deletes remove the ids. Every other operation answers the current snapshot.
  // Undo pops a component list; the seeded entry is the document before e3.
  const shortcutRequest = () => {
    let revision = 1
    let components = shortcutComponents
    const history = [shortcutComponents.filter((component) => component.id !== 'e3')]
    return vi.fn(async (operation: string, payload?: ArrayBuffer) => {
      if (operation === 'undo' && history.length > 0) { components = history.pop()!; revision++ }
      if (operation === 'command' && payload) {
        const command = JSON.parse(new TextDecoder().decode(payload)) as { kind: string; id?: string; ids?: string[] }
        history.push(components)
        revision++
        if (command.kind === 'duplicateComponents') components = [...components, ...command.ids!.map((id, index) => ({ ...components.find((component) => component.id === id)!, id: `e${revision}${'abcdef'[index]}` }))]
        if (command.kind === 'deleteComponents') components = components.filter((component) => !command.ids!.includes(component.id))
        if (command.kind === 'deleteComponent') components = components.filter((component) => component.id !== command.id)
      }
      return { snapshot: shortcutSnapshot(components, revision), ...(operation === 'serialize' ? { bytes } : {}) }
    })
  }
  const shortcutCommands = (request: ReturnType<typeof shortcutRequest>) =>
    (request.mock.calls as unknown as ReadonlyArray<[string, ArrayBuffer]>).filter(([operation]) => operation === 'command').map(([, payload]) => new TextDecoder().decode(payload))
  const selectedComponentIds = () => Array.from(document.querySelectorAll<HTMLElement>('.canvas-component-selected[data-component-id]')).map((element) => element.dataset.componentId)
  const onPlatform = async (value: string, run: () => Promise<void>) => {
    const platform = Object.getOwnPropertyDescriptor(window.navigator, 'platform')
    Object.defineProperty(window.navigator, 'platform', { value, configurable: true })
    try { await run() } finally {
      if (platform) Object.defineProperty(window.navigator, 'platform', platform)
      else Reflect.deleteProperty(window.navigator, 'platform')
    }
  }
  const renderShortcutCanvas = (fileAccess?: FileAccess) => {
    const request = shortcutRequest()
    render(<App engine={engine(request)} {...(fileAccess ? { fileAccess } : {})} initialSnapshot={shortcutSnapshot(shortcutComponents, 1)} />)
    return request
  }
  const selectContentPair = () => {
    fireEvent.click(screen.getByLabelText('rect component e2'))
    fireEvent.click(screen.getByLabelText('rect component e3'), { shiftKey: true })
    expect(selectedComponentIds()).toEqual(['e2', 'e3'])
  }

  it('copies a group without a command, pastes it as one duplicateComponents, selects the copies and stair-steps', async () => {
    await onPlatform('', async () => {
      const request = renderShortcutCanvas()
      selectContentPair()
      const region = screen.getByLabelText('Canvas region')
      expect(fireEvent.keyDown(region, { key: 'c', ctrlKey: true })).toBe(false)
      await act(async () => { await Promise.resolve() })
      expect(shortcutCommands(request)).toEqual([])
      fireEvent.keyDown(region, { key: 'v', ctrlKey: true })
      await waitFor(() => expect(shortcutCommands(request)).toEqual(['{"kind":"duplicateComponents","version":1,"ids":["e2","e3"],"snap":true}']))
      await waitFor(() => expect(selectedComponentIds()).toEqual(['e2a', 'e2b']))
      fireEvent.keyDown(region, { key: 'v', ctrlKey: true })
      await waitFor(() => expect(shortcutCommands(request)[1]).toBe('{"kind":"duplicateComponents","version":1,"ids":["e2a","e2b"],"snap":true}'))
    })
  })

  it('drops copied ids that are gone and sends nothing when none remain, or when nothing was copied', async () => {
    await onPlatform('', async () => {
      const request = renderShortcutCanvas()
      const region = screen.getByLabelText('Canvas region')
      expect(fireEvent.keyDown(region, { key: 'v', ctrlKey: true })).toBe(true)
      await act(async () => { await Promise.resolve() })
      expect(shortcutCommands(request)).toEqual([])
      fireEvent.click(screen.getByLabelText('rect component e2'))
      fireEvent.keyDown(region, { key: 'c', ctrlKey: true })
      fireEvent.keyDown(region, { key: 'Delete' })
      await waitFor(() => expect(shortcutCommands(request)).toEqual(['{"kind":"deleteComponent","version":1,"id":"e2"}']))
      await waitFor(() => expect(screen.queryByLabelText('rect component e2')).toBeNull())
      fireEvent.keyDown(region, { key: 'v', ctrlKey: true })
      await act(async () => { await Promise.resolve() })
      expect(shortcutCommands(request)).toHaveLength(1)
    })
  })

  it.each([
    ['Mac', 'MacIntel', { metaKey: true }, { ctrlKey: true }],
    ['Windows', 'Win32', { ctrlKey: true }, { metaKey: true }],
  ])('reads the %s primary modifier for copy and paste and ignores the other one', async (_name, platform, primary, other) => {
    await onPlatform(platform, async () => {
      const request = renderShortcutCanvas()
      fireEvent.click(screen.getByLabelText('rect component e2'))
      const region = screen.getByLabelText('Canvas region')
      fireEvent.keyDown(region, { key: 'c', ...other })
      fireEvent.keyDown(region, { key: 'v', ...other })
      expect(fireEvent.keyDown(region, { key: 'a', ...other })).toBe(true)
      await act(async () => { await Promise.resolve() })
      expect(shortcutCommands(request)).toEqual([])
      expect(selectedComponentIds()).toEqual(['e2'])
      fireEvent.keyDown(region, { key: 'c', ...primary })
      fireEvent.keyDown(region, { key: 'v', ...primary })
      await waitFor(() => expect(shortcutCommands(request)).toEqual(['{"kind":"duplicateComponents","version":1,"ids":["e2"],"snap":true}']))
    })
  })

  it('pastes only the copied ids that are still in the document', async () => {
    await onPlatform('', async () => {
      const request = renderShortcutCanvas()
      selectContentPair()
      const region = screen.getByLabelText('Canvas region')
      fireEvent.keyDown(region, { key: 'c', ctrlKey: true })
      // Clicking a member of a group keeps the group; clear it to delete e3 alone.
      fireEvent.keyDown(region, { key: 'Escape' })
      fireEvent.click(screen.getByLabelText('rect component e3'))
      expect(selectedComponentIds()).toEqual(['e3'])
      fireEvent.keyDown(region, { key: 'Delete' })
      await waitFor(() => expect(screen.queryByLabelText('rect component e3')).toBeNull())
      fireEvent.keyDown(region, { key: 'v', ctrlKey: true })
      await waitFor(() => expect(shortcutCommands(request)).toEqual(['{"kind":"deleteComponent","version":1,"id":"e3"}', '{"kind":"duplicateComponents","version":1,"ids":["e2"],"snap":true}']))
    })
  })

  it('reports a refused group delete and keeps the selection standing', async () => {
    const accepted = shortcutRequest()
    const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
      if (operation === 'command' && payload && new TextDecoder().decode(payload).includes('"deleteComponents"')) throw new Error('component was not found')
      return accepted(operation, payload)
    })
    render(<App engine={engine(request)} initialSnapshot={shortcutSnapshot(shortcutComponents, 1)} />)
    selectContentPair()
    fireEvent.keyDown(screen.getByLabelText('Canvas region'), { key: 'Delete' })
    expect(await screen.findByRole('alert')).toHaveTextContent(/component was not found/)
    expect(selectedComponentIds()).toEqual(['e2', 'e3'])
    expect(screen.getByLabelText('rect component e3')).toBeInTheDocument()
  })

  it('deletes a whole selection with one deleteComponents and clears the selection', async () => {
    const request = renderShortcutCanvas()
    selectContentPair()
    const region = screen.getByLabelText('Canvas region')
    expect(fireEvent.keyDown(region, { key: 'Backspace' })).toBe(false)
    await waitFor(() => expect(shortcutCommands(request)).toEqual(['{"kind":"deleteComponents","version":1,"ids":["e2","e3"]}']))
    await waitFor(() => expect(selectedComponentIds()).toEqual([]))
    // Nothing selected: nothing sent and the key's default is left alone.
    expect(fireEvent.keyDown(region, { key: 'Delete' })).toBe(true)
    await act(async () => { await Promise.resolve() })
    expect(shortcutCommands(request)).toHaveLength(1)
  })

  it('selects every component in the last band touched, and prevents the page text selection', async () => {
    await onPlatform('', async () => {
      const request = renderShortcutCanvas()
      const region = screen.getByLabelText('Canvas region')
      // Before any band is touched the focus area is Content.
      expect(fireEvent.keyDown(region, { key: 'a', ctrlKey: true })).toBe(false)
      expect(selectedComponentIds()).toEqual(['e2', 'e3'])
      fireEvent.click(screen.getByLabelText('rect component e1'))
      fireEvent.keyDown(region, { key: 'a', ctrlKey: true })
      expect(selectedComponentIds()).toEqual(['e1'])
      fireEvent.pointerDown(screen.getByLabelText('Page Footer'), { pointerId: 9, clientX: 1, clientY: 1, button: 2 })
      fireEvent.keyDown(region, { key: 'a', ctrlKey: true })
      expect(selectedComponentIds()).toEqual([])
      fireEvent.focus(screen.getByLabelText('Content'))
      fireEvent.keyDown(region, { key: 'a', ctrlKey: true })
      expect(selectedComponentIds()).toEqual(['e2', 'e3'])
      expect(shortcutCommands(request)).toEqual([])
    })
  })

  it('leaves copy, paste, select all and delete to the browser inside an editable target', async () => {
    await onPlatform('', async () => {
      const request = renderShortcutCanvas()
      selectContentPair()
      const field = screen.getAllByRole('textbox')[0]!
      for (const keyboard of [{ key: 'c', ctrlKey: true }, { key: 'v', ctrlKey: true }, { key: 'a', ctrlKey: true }, { key: 'Delete' }, { key: 'Backspace' }]) {
        expect(fireEvent.keyDown(field, keyboard)).toBe(true)
      }
      await act(async () => { await Promise.resolve() })
      expect(shortcutCommands(request)).toEqual([])
      expect(selectedComponentIds()).toEqual(['e2', 'e3'])
      // The editable copy did not fill the canvas clipboard either.
      fireEvent.keyDown(screen.getByLabelText('Canvas region'), { key: 'v', ctrlKey: true })
      await act(async () => { await Promise.resolve() })
      expect(shortcutCommands(request)).toEqual([])
    })
  })

  it('sends no clipboard or delete command in Preview mode', async () => {
    await onPlatform('', async () => {
      const request = renderShortcutCanvas()
      fireEvent.click(screen.getByLabelText('rect component e2'))
      fireEvent.keyDown(screen.getByLabelText('Canvas region'), { key: 'c', ctrlKey: true })
      fireEvent.keyDown(window, { key: 'p', altKey: true })
      await waitFor(() => expect(screen.queryByLabelText('Canvas region')).toBeNull())
      for (const keyboard of [{ key: 'v', ctrlKey: true }, { key: 'a', ctrlKey: true }, { key: 'Delete' }, { key: 'Backspace' }]) fireEvent.keyDown(document.body, keyboard)
      await act(async () => { await Promise.resolve() })
      expect(shortcutCommands(request)).toEqual([])
    })
  })

  it('keeps the clipboard across Undo and pastes the copied ids that survive it', async () => {
    await onPlatform('', async () => {
      const request = renderShortcutCanvas()
      selectContentPair()
      const region = screen.getByLabelText('Canvas region')
      fireEvent.keyDown(region, { key: 'c', ctrlKey: true })
      fireEvent.keyDown(region, { key: 'z', ctrlKey: true })
      await waitFor(() => expect(screen.queryByLabelText('rect component e3')).toBeNull())
      fireEvent.keyDown(region, { key: 'v', ctrlKey: true })
      await waitFor(() => expect(shortcutCommands(request)).toEqual(['{"kind":"duplicateComponents","version":1,"ids":["e2"],"snap":true}']))
    })
  })

  it('empties the clipboard when a different document is opened', async () => {
    await onPlatform('', async () => {
      const files: FileAccess = { open: vi.fn(async () => ({ bytes, name: 'other.folio' })), acquireSaveTarget: vi.fn(), writeSave: vi.fn() }
      const request = renderShortcutCanvas(files)
      fireEvent.click(screen.getByLabelText('rect component e2'))
      fireEvent.keyDown(screen.getByLabelText('Canvas region'), { key: 'c', ctrlKey: true })
      fireEvent.click(screen.getByRole('button', { name: 'Open local template' }))
      await waitFor(() => expect(screen.getByRole('button', { name: 'Open local template' })).toBeEnabled())
      await waitFor(() => expect(request.mock.calls.some(([operation]) => operation === 'load')).toBe(true))
      await act(async () => { await Promise.resolve() })
      expect(fireEvent.keyDown(screen.getByLabelText('Canvas region'), { key: 'v', ctrlKey: true })).toBe(true)
      await act(async () => { await Promise.resolve() })
      expect(shortcutCommands(request)).toEqual([])
    })
  })

  it('falls back to the copied originals when the pasted copies were undone', async () => {
    await onPlatform('', async () => {
      const request = renderShortcutCanvas()
      fireEvent.click(screen.getByLabelText('rect component e2'))
      const region = screen.getByLabelText('Canvas region')
      fireEvent.keyDown(region, { key: 'c', ctrlKey: true })
      fireEvent.keyDown(region, { key: 'v', ctrlKey: true })
      await waitFor(() => expect(selectedComponentIds()).toEqual(['e2a']))
      fireEvent.keyDown(region, { key: 'z', ctrlKey: true })
      await waitFor(() => expect(screen.queryByLabelText('rect component e2a')).toBeNull())
      fireEvent.keyDown(region, { key: 'v', ctrlKey: true })
      await waitFor(() => expect(shortcutCommands(request)).toEqual(['{"kind":"duplicateComponents","version":1,"ids":["e2"],"snap":true}', '{"kind":"duplicateComponents","version":1,"ids":["e2"],"snap":true}']))
    })
  })

  it('leaves Delete and select all to the browser on a focused button outside the canvas', async () => {
    await onPlatform('', async () => {
      const request = renderShortcutCanvas()
      selectContentPair()
      const outside = screen.getByRole('button', { name: 'Undo' })
      expect(screen.getByLabelText('Canvas region').contains(outside)).toBe(false)
      outside.focus()
      expect(fireEvent.keyDown(outside, { key: 'Delete' })).toBe(true)
      expect(fireEvent.keyDown(outside, { key: 'a', ctrlKey: true })).toBe(true)
      await act(async () => { await Promise.resolve() })
      expect(shortcutCommands(request)).toEqual([])
      expect(selectedComponentIds()).toEqual(['e2', 'e3'])
    })
  })

  it('sends nothing for Delete, paste or select all while a placement is armed', async () => {
    await onPlatform('', async () => {
      const request = renderShortcutCanvas()
      selectContentPair()
      const region = screen.getByLabelText('Canvas region')
      fireEvent.keyDown(region, { key: 'c', ctrlKey: true })
      fireEvent.click(screen.getByRole('button', { name: 'Place Rectangle' }))
      expect(screen.getByRole('button', { name: 'Place Rectangle' })).toHaveAttribute('aria-pressed', 'true')
      fireEvent.keyDown(region, { key: 'Delete' })
      fireEvent.keyDown(region, { key: 'v', ctrlKey: true })
      expect(fireEvent.keyDown(region, { key: 'a', ctrlKey: true })).toBe(true)
      fireEvent.keyDown(screen.getByLabelText('rect component e2'), { key: 'Delete' })
      await act(async () => { await Promise.resolve() })
      expect(shortcutCommands(request)).toEqual([])
    })
  })

  it('enables the toolbar Delete for a group and sends one deleteComponents', async () => {
    const request = renderShortcutCanvas()
    selectContentPair()
    const button = screen.getByRole('button', { name: 'Delete' })
    expect(button).toBeEnabled()
    fireEvent.click(button)
    await waitFor(() => expect(shortcutCommands(request)).toEqual(['{"kind":"deleteComponents","version":1,"ids":["e2","e3"]}']))
  })

  it('ignores a repeated or a second in-flight Delete, and Shift+Delete', async () => {
    const request = renderShortcutCanvas()
    selectContentPair()
    const region = screen.getByLabelText('Canvas region')
    expect(fireEvent.keyDown(region, { key: 'Delete', shiftKey: true })).toBe(true)
    expect(fireEvent.keyDown(region, { key: 'Backspace', shiftKey: true })).toBe(true)
    expect(fireEvent.keyDown(region, { key: 'Delete', repeat: true })).toBe(true)
    await act(async () => { await Promise.resolve() })
    expect(shortcutCommands(request)).toEqual([])
    fireEvent.keyDown(region, { key: 'Delete' })
    fireEvent.keyDown(region, { key: 'Delete' })
    await waitFor(() => expect(shortcutCommands(request)).toEqual(['{"kind":"deleteComponents","version":1,"ids":["e2","e3"]}']))
    await act(async () => { await Promise.resolve() })
    expect(shortcutCommands(request)).toHaveLength(1)
  })

  it('sends Delete after a released resize whose command is still pending', async () => {
    const placed = { id: 'e9', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72000, height: 24000, resizable: true }
    const componentCanvas = { ...canvas, components: [placed] }
    let settle: (() => void) | undefined
    const request = vi.fn(async (operation: string) => {
      if (operation === 'command' && !settle) await new Promise<void>((resolve) => { settle = resolve })
      return { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas: componentCanvas } }
    })
    render(<App engine={engine(request)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    const body = screen.getByLabelText('text component e9')
    fireEvent.click(body)
    const handle = screen.getByRole('button', { name: 'Resize e9' })
    fireEvent.pointerDown(handle, { pointerId: 1, clientX: 10, clientY: 10 })
    fireEvent.pointerMove(handle, { pointerId: 1, clientX: 20, clientY: 16 })
    fireEvent.pointerUp(handle, { pointerId: 1, clientX: 20, clientY: 16 })
    await waitFor(() => expect(settle).toBeDefined())
    body.focus()
    expect(fireEvent.keyDown(body, { key: 'Delete' })).toBe(false)
    await waitFor(() => expect(shortcutCommands(request as unknown as ReturnType<typeof shortcutRequest>)).toHaveLength(2))
    const sent = shortcutCommands(request as unknown as ReturnType<typeof shortcutRequest>)
    expect(JSON.parse(sent[0]!)).toMatchObject({ kind: 'setComponentBounds', id: 'e9' })
    expect(sent[1]).toBe('{"kind":"deleteComponent","version":1,"id":"e9"}')
    await act(async () => { settle!() })
  })

  it('sends no paste or select all from inside the open table editor', async () => {
    await onPlatform('', async () => {
      const request = modalTableRequest()
      render(<App engine={engine(request)} initialSnapshot={modalTableSnapshot} />)
      fireEvent.click(screen.getByRole('button', { name: 'table component e7' }))
      fireEvent.keyDown(screen.getByLabelText('Canvas region'), { key: 'c', ctrlKey: true })
      fireEvent.click(screen.getByRole('button', { name: 'Configure columns' }))
      await screen.findByRole('dialog', { name: 'Table Editor' })
      const settled = request.mock.calls.length
      const done = screen.getByRole('button', { name: 'Done' })
      done.focus()
      expect(fireEvent.keyDown(done, { key: 'v', ctrlKey: true })).toBe(true)
      expect(fireEvent.keyDown(done, { key: 'a', ctrlKey: true })).toBe(true)
      await act(async () => { await Promise.resolve() })
      expect(request.mock.calls.slice(settled)).toEqual([])
      expect(screen.getByRole('dialog', { name: 'Table Editor' })).toBeInTheDocument()
    })
  })

  it('keeps the Save shortcut working while the table editor is open, because it sits above the guard', async () => {
    const acquireSaveTarget = vi.fn(async () => ({ name: 'untitled.folio', format: folioFileFormat }))
    const writeSave = vi.fn(async () => ({ name: 'untitled.folio' }))
    const files: FileAccess = { open: vi.fn(), acquireSaveTarget, writeSave }
    await openTableEditorOver(files)
    const done = screen.getByRole('button', { name: 'Done' })
    done.focus()
    fireEvent.keyDown(done, { key: 's', ctrlKey: true })
    await waitFor(() => expect(writeSave).toHaveBeenCalledOnce())
    // Saving is not a mutation of what the modal edits, so the dialog is
    // untouched by it.
    expect(screen.getByRole('dialog', { name: 'Table Editor' })).toBeInTheDocument()
  })

  it('leaves the roving lattice inside the dialog moving focus exactly as it did', async () => {
    await openTableEditorOver()
    // THE GUARD SUPPRESSES THE DOCUMENT MUTATION, NOT THE DIALOG'S OWN
    // NAVIGATION. The matrix's arrow keys are React handlers on the cells; the
    // nudge was a native window listener. Both saw the same key.
    // One hop deleted (Q4a), for the reason the two walks above carry.
    const header = screen.getByRole('textbox', { name: 'Header for column 1' })
    header.focus()
    fireEvent.keyDown(header, { key: 'ArrowRight' })
    expect(document.activeElement).toBe(screen.getByRole('combobox', { name: 'Binding for column 1' }))
    fireEvent.keyDown(document.activeElement!, { key: 'ArrowRight', altKey: true })
    expect(document.activeElement).toBe(screen.getByRole('spinbutton', { name: 'Width for column 1 in points' }))
  })

  it('does not over-reach: with no dialog open, Undo still reaches the window handler', async () => {
    // The guard's negative control. It keys on the MODAL BEING OPEN and on
    // nothing else — not on a modifier, not on a key list — so with no dialog on
    // screen the shortcut is exactly what it was.
    const request = vi.fn(async (operation: string) => ({ snapshot: { ...snapshot(1), canUndo: operation !== 'undo', canRedo: operation === 'undo' } }))
    render(<App engine={engine(request)} initialSnapshot={{ ...snapshot(1), canUndo: true }} />)
    fireEvent.keyDown(screen.getByLabelText('Canvas region'), { key: 'z', ctrlKey: true })
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'undo')).toHaveLength(1))
  })

  // STORY 14.3: A PLACED COMPONENT IS THE SELECTED COMPONENT.
  //
  // The placed component. `snapshot(2)` carries the module-level canvas, whose
  // `components` is EMPTY — which is why the test below could not observe any
  // of Story 14.3's claims before it was given a snapshot that actually adds a
  // component. The new id is not on the wire in either direction: it is derived
  // by diffing the component ids across the commit, so a mock that adds nothing
  // is a mock in which there is nothing to select.
  const placedText = { id: 'e9', type: 'text' as const, band: 'content' as const, x: 36_000, y: 56_000, width: 72_000, height: 24_000, resizable: true }
  const placedTextSnapshot = { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas: { ...canvas, components: [placedText] } }
  const armAndPlace = () => {
    fireEvent.click(screen.getByRole('button', { name: 'Place Text' }))
    fireEvent.keyDown(screen.getByLabelText('Content'), { key: 'Enter' })
  }
  // THE FOCUS MOVE IS DEFERRED BY ONE MACROTASK, so a NEGATIVE assertion about
  // focus made in the same tick passes whether or not focus was wrongly moved —
  // it beats the timer it means to disprove. Every "nothing is focused" row
  // waits a real macrotask turn first, so the absence is a measurement.
  const settleDeferredFocus = async () => { await act(async () => { await new Promise((resolve) => setTimeout(resolve, 0)) }) }
  // App.css is scanned with COMMENTS BLANKED. The pad rows gather by the VALUE a
  // rule mentions (`--hit-pad`, `z-index`), not by its selector, so prose in a
  // comment is a false positive — and the rules below carry long comments that
  // discuss exactly those names. Blanking preserves line positions, so the
  // line-oriented gathering is unchanged. This is deliberately NOT the raw
  // policy the band-boundary rows use: those gather by SELECTOR, where a
  // commented-out rule counting as live is the property they want.
  const sheetWithoutComments = () => fs.readFileSync('src/App.css', 'utf8').replace(/\/\*[\s\S]*?\*\//g, (block) => block.replace(/[^\n]/g, ' '))
  const ruleLines = (predicate: (selector: string) => boolean) => sheetWithoutComments().split('\n')
    .filter((line) => line.includes('{') && predicate(line.slice(0, line.indexOf('{'))))

  it('offers only the seven fixed palette components, plus the Section Break, and sends an opaque Go placement command', async () => {
    const request = vi.fn(async () => ({ snapshot: placedTextSnapshot }))
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} />)
    expect(screen.getAllByRole('button', { name: /Place / }).map((button) => button.getAttribute('aria-label'))).toEqual(['Place Text', 'Place Image', 'Place Table', 'Place Line', 'Place Rectangle', 'Place Barcode', 'Place QR Code', 'Place Section Break'])
    // THE POSITIVE CONTROL for the absence asserted further down: this is what
    // an empty selection puts in the inspector, and it is what placing used to
    // leave standing.
    expect(screen.getByText('Component properties require a selection.')).toBeInTheDocument()
    armAndPlace()
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    const [operation, payload] = request.mock.calls[0] as unknown as [string, ArrayBuffer]
    expect(operation).toBe('command')
    expect(new TextDecoder().decode(payload)).toBe('{"kind":"dropComponent","version":1,"type":"text","x":36,"y":56,"snap":true}')
    // AND THE THING IT MADE IS THE THING THAT IS SELECTED, AND FOCUSED.
    await waitFor(() => expect(screen.getByLabelText('text component e9')).toHaveFocus())
    expect(screen.getByText('e9 · band: content')).toBeInTheDocument()
    expect(screen.getByLabelText('Resize e9')).toBeInTheDocument()
    expect(screen.queryByText('Component properties require a selection.')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Apply page setup' })).not.toBeInTheDocument()
    // Neither the selection nor the focus sent anything: the create is still the
    // only command in the log.
    expect(request).toHaveBeenCalledOnce()
  })

  // ⚠ THE FOCUS IS DRIVEN BY A RENDER, NOT BY A TIMER, AND THIS ROW IS THE
  // DISCRIMINATOR BETWEEN THE TWO. It flushes MICROTASKS ONLY and never a timer
  // turn: a focus armed with `setTimeout(…, 0)` cannot have happened yet at this
  // point, while a focus taken in the effect that follows the render mounting
  // the element already has.
  //
  // The timer version was a real defect, not a style question. It attempted
  // focus exactly once, and on any sheet after the first the extra render pass
  // meant the element was not mounted when it fired — the lookup found nothing,
  // `?.focus()` swallowed the miss, and focus was lost for good. Review measured
  // that as a hard failure on the later-sheet row; on this machine the single
  // tick happened to win the race, which is exactly why the guard has to be
  // about the MECHANISM rather than about the outcome on one machine.
  it('takes focus on the render that mounts the placed component, not on a later macrotask', async () => {
    const request = vi.fn(async () => ({ snapshot: placedTextSnapshot }))
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} />)
    armAndPlace()
    await act(async () => { for (let turn = 0; turn < 8; turn++) await Promise.resolve() })
    expect(screen.getByLabelText('text component e9')).toHaveFocus()
  })

  // AND IT DOES NOT STEAL FOCUS THE AUTHOR HAS ALREADY MOVED. A placement whose
  // command is still in flight must not yank the caret out of a field the author
  // has since clicked into — the failure mode a bare deferred focus has by
  // construction, because it cannot know anything has changed.
  it('gives up its focus claim when the author has moved focus somewhere else meanwhile', async () => {
    let answer: (() => void) | undefined
    const request = vi.fn(async () => { await new Promise<void>((resolve) => { answer = resolve }); return { snapshot: placedTextSnapshot } })
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} />)
    armAndPlace()
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    // The author has moved on to a control that OUTLIVES the selection. It has
    // to outlive it, or there would be nothing left to steal: any Page Setup
    // field is unmounted by the very selection under test, and focus falling
    // back to `body` because a field was replaced is not the author moving it.
    const elsewhere = screen.getByRole('button', { name: 'Zoom in' })
    elsewhere.focus()
    expect(elsewhere).toHaveFocus()
    answer!()
    await waitFor(() => expect(screen.getByLabelText('text component e9')).toBeInTheDocument())
    await settleDeferredFocus()
    // The placement still selects — that is AC1 and it is not in question — but
    // the caret stays where the author put it.
    expect(screen.getByText('e9 \u00b7 band: content')).toBeInTheDocument()
    expect(screen.getByLabelText('text component e9')).not.toHaveFocus()
    expect(elsewhere).toHaveFocus()
  })

  // THE REFUSAL ROW. A rejected create must leave the selection exactly where
  // it was and move focus nowhere — the derived id never exists, so there is
  // nothing to select, and the existing `componentDiagnostic` alert path is the
  // only thing that changes.
  it('leaves selection and focus alone when the engine refuses the placement', async () => {
    const request = vi.fn(async () => ({ snapshot: placedTextSnapshot })).mockRejectedValue(Object.assign(new Error('the content band cannot hold it'), { code: 'COMPONENT_INVALID' }))
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} />)
    armAndPlace()
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('the content band cannot hold it'))
    await settleDeferredFocus()
    expect(screen.getByText('Component properties require a selection.')).toBeInTheDocument()
    expect(screen.queryByLabelText(/component e/)).not.toBeInTheDocument()
    expect(document.activeElement).toBe(document.body)
  })

  // THE NO-NEW-ID ROW, AND IT DEGRADES SILENTLY. An accepted command whose
  // snapshot adds no id must select nothing rather than reach for a stale one —
  // the existing component here is the id a "select whatever is there" shortcut
  // would wrongly land on.
  it('selects nothing when an accepted command adds no component id', async () => {
    const standing = { id: 'e1', type: 'rect' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true }
    const standingSnapshot = { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: { ...canvas, components: [standing] } }
    const request = vi.fn(async () => ({ snapshot: { ...standingSnapshot, revision: 2 } }))
    render(<App engine={engine(request)} initialSnapshot={standingSnapshot} />)
    armAndPlace()
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    await settleDeferredFocus()
    expect(screen.getByText('Component properties require a selection.')).toBeInTheDocument()
    expect(screen.queryByLabelText('Resize e1')).not.toBeInTheDocument()
    expect(document.activeElement).toBe(document.body)
  })

  // D-12.A FORK 3. SELECTION AND FOCUS ARE TWO FACTS AND THE WINDOW-LEVEL ARROW
  // HANDLER IS A THIRD, so a test that only checked the happy state could not
  // tell one firing from all three. The arrow handler is NOT gated on the canvas
  // region, and a placed component is now selected — so the keystroke that
  // places must not also nudge, and the arrow that follows must send exactly one
  // move rather than one per handler that happens to agree.
  it('places with selection and focus as separate facts, and nudges neither during nor twice after', async () => {
    const request = vi.fn(async () => ({ snapshot: placedTextSnapshot }))
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} />)
    armAndPlace()
    await waitFor(() => expect(screen.getByLabelText('text component e9')).toHaveFocus())
    // FACT ONE: the selection. FACT TWO: the focus, asserted above. Losing
    // either one alone reds this row.
    expect(screen.getByLabelText('Resize e9')).toBeInTheDocument()
    // The placing keystroke sent the create and nothing else — no nudge rode
    // along on the Enter that placed it.
    expect(request).toHaveBeenCalledOnce()
    // And the arrow that follows is ONE move, raised on the focused element so
    // it travels the real route: the component's own onKeyDown, then the window
    // listener. Only the second of those may act on it.
    fireEvent.keyDown(screen.getByLabelText('text component e9'), { key: 'ArrowRight' })
    await waitFor(() => expect(request).toHaveBeenCalledTimes(2))
    expect(new TextDecoder().decode((request.mock.calls[1] as unknown as [string, ArrayBuffer])[1])).toBe('{"kind":"moveComponent","version":1,"id":"e9","x":37,"y":56,"snap":false}')
    expect(request).toHaveBeenCalledTimes(2)
  })

  it.each([true, false])('nudges fractional projected coordinates precisely and keeps Snap %s unchanged', async (snap) => {
    const at = (revision: number, x: number, y: number) => ({
      documentState: 'loaded' as const, revision, byteLength: 3,
      canvas: { ...canvas, components: [{ id: 'e9', type: 'rect' as const, band: 'content' as const, x, y, width: 72_000, height: 24_000, resizable: true }] },
    })
    const request = vi.fn(async () => ({ snapshot: at(3, 13_375, 14_625) }))
      .mockResolvedValueOnce({ snapshot: at(2, 13_375, 24_625) })
    render(<App engine={engine(request)} initialSnapshot={at(1, 12_375, 24_625)} />)
    if (!snap) fireEvent.click(screen.getByRole('button', { name: /^Snap on/ }))
    const component = screen.getByLabelText('rect component e9')
    fireEvent.click(component)

    fireEvent.keyDown(component, { key: 'ArrowRight' })
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'X (pt)' })).toHaveValue('13.375'))
    // The second proposal must use the engine's returned x, preserving its
    // millipoint precision while Shift requests exactly ten points on y.
    fireEvent.keyDown(component, { key: 'ArrowUp', shiftKey: true })
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Y (pt)' })).toHaveValue('14.625'))
    const sent = (request.mock.calls as unknown as Array<[string, ArrayBuffer]>).map(([, payload]) => new TextDecoder().decode(payload))
    expect(sent).toEqual([
      '{"kind":"moveComponent","version":1,"id":"e9","x":13.375,"y":24.625,"snap":false}',
      '{"kind":"moveComponent","version":1,"id":"e9","x":13.375,"y":14.625,"snap":false}',
    ])
    expect(screen.getByRole('button', { name: /^Snap/ })).toHaveAttribute('aria-pressed', String(snap))
  })

  // AC2's ARITHMETIC, WHICH IS THE HALF JSDOM CAN HONESTLY SEE. It applies no
  // stylesheet and every rect is zeros, so the pointer geometry itself is proved
  // in `e2e/placed-component-selection.spec.ts` and nowhere here. What is
  // observable here is the inline custom property the pad is computed into, read
  // the way `--component-x` is read elsewhere in this file.
  const padCanvas = {
    ...canvas,
    components: [
      { id: 'e1', type: 'line' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 1_000, resizable: true, background: '#000000' },
      { id: 'e2', type: 'image' as const, band: 'content' as const, x: 0, y: 20_000, width: 4_000, height: 4_000, resizable: true, background: '#000000' },
      { id: 'e3', type: 'text' as const, band: 'content' as const, x: 0, y: 40_000, width: 72_000, height: 24_000, resizable: true, background: '#000000' },
    ],
  }
  const padSnapshot = { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: padCanvas }

  it('pads only the axis a component leaves short of the comfortable hit size, whatever its kind', () => {
    render(<App engine={engine()} initialSnapshot={padSnapshot} />)
    // THE RULE IS PER-ELEMENT-SIZE, NEVER PER-KIND: the 4pt image is padded on
    // both axes on exactly the terms the 1pt line is padded on one, and the 24pt
    // text box is padded on neither.
    // The 1pt line paints at 1px inside a 2px interaction wrapper. Its 5px
    // padding on each side therefore produces the intended 12px hit region.
    expect(screen.getByLabelText('line component e1').style.getPropertyValue('--hit-pad-x')).toBe('0px')
    expect(screen.getByLabelText('line component e1').style.getPropertyValue('--hit-pad-y')).toBe('5px')
    expect(screen.getByLabelText('image component e2').style.getPropertyValue('--hit-pad-x')).toBe('4px')
    expect(screen.getByLabelText('image component e2').style.getPropertyValue('--hit-pad-y')).toBe('4px')
    expect(screen.getByLabelText('text component e3').style.getPropertyValue('--hit-pad-x')).toBe('0px')
    expect(screen.getByLabelText('text component e3').style.getPropertyValue('--hit-pad-y')).toBe('0px')
  })

  it('recomputes the pad from the interaction box at each zoom rather than pinning a fixed number', () => {
    render(<App engine={engine()} initialSnapshot={padSnapshot} />)
    fireEvent.click(screen.getByRole('button', { name: 'Zoom out' }))
    expect(screen.getByLabelText('Canvas zoom')).toHaveTextContent('90%')
    // THE 4pt IMAGE IS THE ZOOM WITNESS, because it draws ABOVE the 2px floor at
    // both zooms and so its pad tracks the zoom alone: 4px drawn -> 4px pad at
    // 100%, 3.6px drawn -> 4.2px pad at 90%. Both reach exactly 12px. A pad
    // pinned to a fixed number reds here.
    const image = screen.getByLabelText('image component e2')
    expect(image.style.getPropertyValue('--component-height')).toBe('3.6px')
    expect(image.style.getPropertyValue('--hit-pad-y')).toBe('4.2px')
    // The 1pt line's interaction wrapper stays at 2px at both zooms, so its
    // pad stays at 5px even though the painted line follows the zoom.
    const line = screen.getByLabelText('line component e1')
    expect(line.style.getPropertyValue('--component-height')).toBe('0.9px')
    expect(line.style.getPropertyValue('--hit-pad-y')).toBe('5px')
    // 24pt at 0.9 is 21.6px, still over the comfortable size, so still no pad.
    expect(screen.getByLabelText('text component e3').style.getPropertyValue('--hit-pad-y')).toBe('0px')
  })

  // ⚠ THE GUARD THAT SEPARATES THIS STORY'S MECHANISM FROM THE DEFECT DW-345
  // RECORDS, AND IT IS WRITTEN TO RED IF THE PADDING IS EVER RE-IMPLEMENTED AS
  // BOX INFLATION.
  //
  // A test asserting only "a click near a line selects it" passes on BOTH
  // implementations — including the wrong one, which widens the element and lets
  // `.canvas-box { inset: 0 }` follow it. That is exactly how the `max(2px, …)`
  // floor above came to exist, and that floor is left alone here on the grounds
  // DW-345 records rather than folded into a selection story.
  //
  // Component paint uses the `--component-*` values and `.canvas-box`; lines
  // override the box's dimensions to avoid inheriting the interaction floor.
  // Neither paint nor the outer box may mention the pad, and the
  // pad may exist ONLY as a pseudo-element hung outside the box on negative
  // insets. jsdom applies no stylesheet, so the declarations are read from the
  // source the way canvas-authority-contract.test.ts reads it, and the values
  // from the DOM.
  it('pads the hit region without inflating the box, so the paint is byte-identical either way', () => {
    const css = sheetWithoutComments()
    const cssRule = (selector: string) => {
      const found = css.match(new RegExp(`^${selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')} \\{([^}]*)\\}`, 'm'))
      expect(found, `App.css must declare a \`${selector}\` rule for this row to be about anything`).not.toBeNull()
      return found![1]!.trim()
    }
    // HALF ONE — THE DECLARATIONS. The painted boxes are recorded literally, so
    // any of them learning about the pad by any spelling reds this row.
    expect(cssRule('.canvas-component')).toContain('width: max(2px, var(--component-width))')
    expect(cssRule('.canvas-component')).toContain('height: max(2px, var(--component-height))')
    expect(cssRule('.canvas-component-line')).toBe('height: max(2px, var(--component-height));')
    expect(cssRule('.canvas-box')).toBe('position: absolute; inset: 0; box-sizing: border-box; pointer-events: none;')
    for (const selector of ['.canvas-component', '.canvas-component-line', '.canvas-box']) {
      expect(cssRule(selector), `${selector} is the PAINT and may not know about the hit pad`).not.toContain('--hit-pad')
    }
    // Every rule that mentions the pad is a pseudo-element rule, GATHERED from
    // the sheet rather than listed here, so a second pad rule added later is
    // covered without anyone remembering to come back.
    const padRules = css.split('\n').filter((line) => line.includes('--hit-pad'))
    expect(padRules, 'App.css must declare the pad somewhere for this row to be about anything').not.toHaveLength(0)
    for (const rule of padRules) expect(rule).toMatch(/^\.[^{]*::before \{/)
    // And it reaches OUTSIDE the box by a negative inset — never by a width, a
    // height, a padding or a margin, every one of which would move the box
    // instead. The declaration list is an ALLOWLIST, which is the only way "it
    // paints nothing" is real.
    const pad = cssRule('.canvas-component::before')
    expect(pad).toContain('inset: calc(-1 * var(--hit-pad-y, 0px)) calc(-1 * var(--hit-pad-x, 0px))')
    expect(pad).not.toMatch(/\b(?:width|height|padding|margin)\s*:/)
    // THE ALLOWLIST GREW BY ONE, BY AUTHORIZATION, AND THE REASON IS RECORDED
    // HERE SO A LATER READER SEES A DELIBERATE WIDENING RATHER THAN DRIFT.
    // `z-index` was added by the frozen block's amendment of 2026-09-09: without
    // it a later sibling's pad hit-tests over an earlier sibling's painted box,
    // so this story would have made every component adjacent to a thin one
    // HARDER to grab. The allowlist did its job by reddening on the unauthorized
    // property; it is widened, never bypassed, and it still bars a background, a
    // border, an outline, a width or a height.
    expect(pad.split(';').map((one) => one.trim()).filter(Boolean).map((one) => one.slice(0, one.indexOf(':')).trim()).sort()).toEqual(['background', 'content', 'inset', 'position', 'z-index'])
    expect(pad).toMatch(/background:\s*transparent/)

    // HALF TWO — THE VALUES, and the reachable area really does grow. Both
    // halves are load-bearing: removing the pad entirely reds the pad column,
    // and inflating the box instead reds the geometry columns and the
    // declarations above.
    render(<App engine={engine()} initialSnapshot={padSnapshot} />)
    for (const [label, width, height, padX, padY] of [
      ['line component e1', '72px', '1px', '0px', '5px'],
      ['image component e2', '4px', '4px', '4px', '4px'],
      ['text component e3', '72px', '24px', '0px', '0px'],
    ] as const) {
      const element = screen.getByLabelText(label)
      // The box is EXACTLY the projection at this zoom. The pad reaches it
      // nowhere.
      expect(element.style.getPropertyValue('--component-width')).toBe(width)
      expect(element.style.getPropertyValue('--component-height')).toBe(height)
      expect(element.style.getPropertyValue('--hit-pad-x')).toBe(padX)
      expect(element.style.getPropertyValue('--hit-pad-y')).toBe(padY)
      // The painted fill carries no geometry of its own, so it cannot be where
      // an inflation hides.
      const box = element.querySelector('.canvas-box') as HTMLElement | null
      expect(box, `${label} must paint a .canvas-box for this row to be about anything`).not.toBeNull()
      for (const property of ['width', 'height', 'top', 'right', 'bottom', 'left', 'inset', 'padding', 'margin']) expect(box!.style.getPropertyValue(property)).toBe('')
    }
  })

  // THE "PLACEMENT BEATS PADDING" AND "PADDING RESTORED" ROWS, MECHANISM HALF.
  //
  // ⚠ WHAT THIS DOES NOT PROVE: that a click 4px from a rule with a palette
  // kind armed actually PLACES rather than selecting the rule. That is pointer
  // geometry, it needs a real hit test at real coordinates, and it lives in
  // `e2e/placed-component-selection.spec.ts` — which per D-000.33 is COMPILED
  // in this story and does not execute until the Epic 14 boundary gate. Read
  // this row as the hook the stylesheet keys on, and nothing more.
  //
  // BOTH ARMS ARE ASSERTED. A hook that is always on is as useless as one that
  // is never on — it would make the pad permanently inert while still answering
  // "is the hook there".
  //
  // ⚠ THE HOOK IS `.canvas-region-placing`, THE CLASS THAT ALREADY EXISTED for
  // App.css:113's `cursor: copy`. It is not a second encoding of the same state:
  // review found `data-placing` duplicating this class off the same `placing`
  // value on the same element, and the duplicate was removed rather than pinned
  // together by a test.
  it('marks the canvas region while a palette kind is armed and unmarks it on every route that disarms', async () => {
    const request = vi.fn(async () => ({ snapshot: placedTextSnapshot }))
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} />)
    const region = screen.getByLabelText('Canvas region')
    expect(region.matches('.canvas-region-placing')).toBe(false)
    fireEvent.click(screen.getByRole('button', { name: 'Place Line' }))
    expect(region.matches('.canvas-region-placing')).toBe(true)
    // ROUTE ONE: Escape, which is `clearInteraction` reached without a pointer.
    fireEvent.keyDown(region, { key: 'Escape' })
    expect(region.matches('.canvas-region-placing')).toBe(false)
    // ROUTE TWO — AND THIS IS THE "PADDING RESTORED" ROW: the placement itself
    // disarms, so the pad takes pointer events back the moment the component
    // lands rather than staying inert for the rest of the session.
    fireEvent.click(screen.getByRole('button', { name: 'Place Text' }))
    expect(region.matches('.canvas-region-placing')).toBe(true)
    armAndPlace()
    await waitFor(() => expect(request).toHaveBeenCalled())
    expect(region.matches('.canvas-region-placing')).toBe(false)
    await waitFor(() => expect(screen.getByLabelText('text component e9')).toHaveFocus())
    expect(region.matches('.canvas-region-placing')).toBe(false)
  })

  // AND THE RULE THAT HOOK EXISTS FOR. Without it the padded region swallows the
  // placement pointerup the band beneath it was meant to receive — the same
  // defect `.canvas-component-echo` was made inert for, in the same file.
  //
  // ⚠ WHAT THIS DOES NOT PROVE: that the suppression works. jsdom applies no
  // stylesheet, so this is the declaration and not its effect; the effect is in
  // the e2e spec named above and is unexecuted until the boundary gate.
  //
  // The rules are GATHERED FROM THE SHEET rather than named here, exactly as the
  // `--hit-pad` rules are in the byte-identity row above, so a second pad rule
  // added later cannot slip past this one either.
  it('renders the pad inert while a placement is armed, and only while one is', () => {
    const selectorOf = (rule: string) => rule.slice(0, rule.indexOf('{'))
    const padSurface = ruleLines((selector) => /\.canvas-component::before/.test(selector))
    expect(padSurface, 'App.css must style the hit-pad pseudo-element for this row to be about anything').not.toHaveLength(0)
    const inert = padSurface.filter((rule) => /pointer-events:\s*none/.test(rule))
    expect(inert, 'App.css must make the hit pad inert while a placement is armed').not.toHaveLength(0)
    // Every inert rule is GATED by something ahead of the pad in its selector,
    // and they all key on the same gate. An ungated `pointer-events: none` would
    // retire the whole padded region while still answering "is there a
    // suppression rule".
    const gates = new Set(inert.map((rule) => selectorOf(rule).replace(/\s*\.canvas-component::before\s*$/, '').trim() || undefined))
    expect(gates.has(undefined), 'a rule making the pad inert must be gated, never unconditional').toBe(false)
    expect([...gates], 'every inert rule must key on the one armed-placement gate').toHaveLength(1)
    const gate = [...gates][0] as string
    // And the BASE rule is not inert — gathered the same way, so one rule cannot
    // satisfy both halves.
    for (const rule of padSurface.filter((one) => !selectorOf(one).includes(gate))) {
      expect(rule, 'the unarmed pad must take pointer events').not.toMatch(/pointer-events\s*:/)
    }
    // THE JOIN: the gate the sheet keys on is a selector the canvas region
    // really matches when a kind is armed and really fails to match when none
    // is — read off the gathered selector rather than written out twice, so a
    // rename or a deletion on either side reds this row.
    render(<App engine={engine()} initialSnapshot={padSnapshot} />)
    const region = screen.getByLabelText('Canvas region')
    expect(region.matches(gate)).toBe(false)
    fireEvent.click(screen.getByRole('button', { name: 'Place Line' }))
    expect(region.matches(gate)).toBe(true)
  })

  // A COMPONENT'S OWN PAINT OUTRANKS A NEIGHBOUR'S INVISIBLE PAD — the two
  // matrix rows added by the frozen block's amendment of 2026-09-09.
  //
  // ⚠ WHAT THIS DOES NOT PROVE: which element a press actually resolves to.
  // jsdom applies no stylesheet and computes no stacking, so this row is the
  // DECLARED mechanism; the resolution itself is measured in
  // `e2e/placed-component-selection.spec.ts`, which is compiled here and
  // executes at the Epic 14 boundary gate.
  it('drops the hit pad below every component box, and keeps the ancestor stacking context that makes that possible', () => {
    const padRules = ruleLines((selector) => /\.canvas-component::before/.test(selector))
    expect(padRules, 'App.css must style the hit pad for this row to be about anything').not.toHaveLength(0)
    const declared = /z-index:\s*(-?\d+)/.exec(padRules.join('\n'))
    expect(declared, 'the pad must declare a z-index, or a later sibling pad covers an earlier sibling box').not.toBeNull()
    // NEGATIVE, not merely present: the pad must sit BELOW every component box,
    // and a positive or zero z-index would raise it further instead.
    expect(Number(declared![1])).toBeLessThan(0)
    // ⚠ THE OTHER HALF, AND IT IS NOT OPTIONAL. A negative z-index resolves in
    // the nearest ANCESTOR STACKING CONTEXT. Until this story there was none
    // between the pad and the root — no z-index, transform, opacity or filter on
    // .canvas-component, .band-window, .page-band or .page-surface — so the pad
    // sank below `.page-surface`'s opaque background and stopped being reachable
    // at all. Measured in Chromium, not reasoned: with the z-index alone, a
    // press 4px from a rule landed on the band and selected nothing. Delete the
    // isolation and the story silently loses its whole feature, which is why the
    // guard pins both halves rather than the one that looks like the fix.
    expect(ruleLines((selector) => /^\.(?:page-band|page-surface|band-window|canvas-region)\b/.test(selector.trim()))
      .filter((rule) => /isolation:\s*isolate/.test(rule)),
    'an ancestor of the hit pad must establish a stacking context, or the negative z-index escapes to the root').not.toHaveLength(0)
    // AND THE COMPONENT BOXES THEMSELVES STAY UNRANKED, which is what keeps
    // AC3's thin-vs-thin determinism free rather than invented. Gathered as
    // every rule reaching a component that is NOT a pseudo-element rule.
    const boxRules = ruleLines((selector) => /\.canvas-component/.test(selector) && !/::(?:before|after)/.test(selector))
    expect(boxRules, 'App.css must declare component box rules for this row to be about anything').not.toHaveLength(0)
    for (const rule of boxRules) expect(rule, 'a z-index on a component BOX would replace AC3 tree order with an invented one').not.toMatch(/z-index\s*:/)
  })

  // AC3 IS PRESERVATION, AND THIS ROW SAYS WHICH MECHANISM IT PROTECTS rather
  // than discovering a behaviour. Alternation is already structurally
  // impossible, on two named grounds, and both are measured here. Reverting the
  // padded region leaves this row green — which is the point: it measures
  // determinism, not padding. The real-coordinate half, clicking one spot over
  // two overlapping thin components, is in e2e/placed-component-selection.spec.ts.
  it('resolves two overlapping thin components deterministically, by the mechanism it names', () => {
    const overlapping = { ...canvas, components: [
      { id: 'e1', type: 'line' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 1_000, resizable: true, background: '#000000' },
      { id: 'e2', type: 'line' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 1_000, resizable: true, background: '#000000' },
    ] }
    const overlapSnapshot = { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: overlapping }
    const request = vi.fn(async () => ({ snapshot: overlapSnapshot }))
    // MECHANISM ONE: nothing overrides the natural stacking order, so the LAST
    // sibling in document order paints on top and takes the pointer. Six
    // z-index declarations in this sheet; none of them on a canvas component.
    // ⚠ SCOPED TO THE BOXES, NOT TO EVERY `.canvas-component` SELECTOR, and the
    // narrowing is deliberate rather than a weakening: the pad pseudo-element
    // now carries a z-index BY AUTHORIZATION (see the arbitration row above),
    // while the things that hit-test as components must stay unranked so tree
    // order alone decides. The row above owns the pad's half.
    const css = sheetWithoutComments()
    for (const rule of ruleLines((selector) => /\.canvas-component/.test(selector) && !/::(?:before|after)/.test(selector))) {
      expect(rule).not.toMatch(/z-index\s*:/)
    }
    // POSITIVE CONTROL: the same gathering finds a z-index where one really is
    // declared, so the absence above is a measurement.
    expect(ruleLines((selector) => /\.canvas-text-truncated/.test(selector)).filter((rule) => /z-index\s*:/.test(rule))).not.toHaveLength(0)
    expect(css).toContain('.canvas-text-truncated')
    render(<App engine={engine(request)} initialSnapshot={overlapSnapshot} />)
    expect(Array.from(document.querySelectorAll<HTMLElement>('[data-component-id]')).map((element) => element.dataset.componentId)).toEqual(['e1', 'e2'])
    // MECHANISM TWO: `begin()` stops the pointerdown, so exactly ONE component
    // handles a given press and nothing above it gets a second say.
    //
    // ⚠ THE LISTENER IS ON `document`, AND THAT PLACEMENT IS THE WHOLE
    // MEASUREMENT. React 18 delegates from the render CONTAINER, so a native
    // listener on any element between the component and that container — the
    // band, the page surface — runs BEFORE React has dispatched anything and
    // sees every press whatever the handler goes on to do. `document` is above
    // the container, so it is reached only if the synthetic handler let the
    // native event carry on past it.
    const above = vi.fn()
    document.addEventListener('pointerdown', above)
    try {
      for (let press = 1; press <= 3; press++) {
        const target = screen.getByLabelText('line component e2')
        fireEvent.pointerDown(target, { pointerId: press, clientX: 1, clientY: 1 })
        fireEvent.pointerUp(target, { pointerId: press, clientX: 1, clientY: 1 })
        // The same component every time. Never alternating.
        expect(screen.getByText('e2 \u00b7 band: content')).toBeInTheDocument()
      }
      expect(above).not.toHaveBeenCalled()
      // POSITIVE CONTROL, on the same listener: a press on the band — which
      // stops nothing — does reach it, so the absence above is a measurement of
      // `stopPropagation` and not of a listener that never fires.
      fireEvent.pointerDown(screen.getByLabelText('Content'), { pointerId: 4, clientX: 1, clientY: 1, button: 2 })
      expect(above).toHaveBeenCalledOnce()
    }
    finally { document.removeEventListener('pointerdown', above) }
    expect(request).not.toHaveBeenCalled()
  })

  it('converts a local band pointer position through the shared display mapping before proposing placement', () => {
    const localX = ['offset', 'X'].join('')
    const localY = ['offset', 'Y'].join('')
    expect(placementPoint({ [localX]: 120, [localY]: 40 } as unknown as MouseEvent, canvas.bands[1]!, 1)).toEqual({ x: 156, y: 96 })
  })

  it('keeps selection local and deletes one unambiguous selected component through Go', async () => {
    const componentCanvas = { ...canvas, components: [{ id: 'e9', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72000, height: 24000, resizable: true }] }
    const request = vi.fn(async () => ({ snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas: componentCanvas } }))
    render(<App engine={engine(request)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    fireEvent.click(screen.getByLabelText('text component e9'))
    expect(request).not.toHaveBeenCalled()
    const region = screen.getByLabelText('Canvas region')
    region.focus()
    fireEvent.keyDown(region, { key: 'Delete' })
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(new TextDecoder().decode((request.mock.calls[0] as unknown as [string, ArrayBuffer])[1])).toBe('{"kind":"deleteComponent","version":1,"id":"e9"}')
  })

  it('does not send a move for a pointer selection, but commits one point-valued move after a real drag', async () => {
    const componentCanvas = { ...canvas, components: [{ id: 'e9', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72000, height: 24000, resizable: true }] }
    const request = vi.fn(async () => ({ snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas: componentCanvas } }))
    render(<App engine={engine(request)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    const component = screen.getByLabelText('text component e9')
    fireEvent.pointerDown(component, { pointerId: 1, clientX: 10, clientY: 10 })
    fireEvent.pointerUp(component, { pointerId: 1, clientX: 10, clientY: 10 })
    expect(request).not.toHaveBeenCalled()
    fireEvent.pointerDown(component, { pointerId: 2, clientX: 10, clientY: 10 })
    fireEvent.pointerMove(component, { pointerId: 2, clientX: 13, clientY: 12 })
    fireEvent.pointerUp(component, { pointerId: 2, clientX: 13, clientY: 12 })
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(new TextDecoder().decode((request.mock.calls[0] as unknown as [string, ArrayBuffer])[1])).toBe('{"kind":"moveComponents","version":1,"ids":["e9"],"referenceId":"e9","dx":3,"dy":2,"snap":true,"expectedRevision":1,"constrainToWindow":true}')
  })

  it('renders selection chrome outside the band clip using projected sheet coordinates', async () => {
    const placed = { id: 'e9', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72000, height: 24000, resizable: true }
    const componentCanvas = { ...canvas, components: [placed] }
    const request = vi.fn(async () => ({ snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas: componentCanvas } }))
    render(<App engine={engine(request)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    const body = screen.getByLabelText('text component e9')
    fireEvent.click(body)
    const handle = screen.getByRole('button', { name: 'Resize e9' })
    expect(body.closest('.band-window')).not.toBeNull()
    expect(handle.closest('.band-window')).toBeNull()
    const chrome = handle.closest('.canvas-selection-chrome') as HTMLElement
    expect(chrome.closest('.page-band')).toBeNull()
    expect(chrome.style.getPropertyValue('--component-x')).toBe(`${canvas.bands[1]!.x / 1000}px`)
    expect(chrome.style.getPropertyValue('--component-y')).toBe(`${canvas.bands[1]!.y / 1000}px`)
    expect(chrome.querySelector('.canvas-dimension')).toHaveTextContent('72 × 24')
    expect(screen.getAllByLabelText('text component e9')).toHaveLength(1)
    expect(document.querySelectorAll('[data-component-id="e9"]')).toHaveLength(1)
    const northwest = chrome.querySelector('.selection-handle-nw')!
    fireEvent.pointerDown(northwest, { pointerId: 1, clientX: 10, clientY: 10 })
    fireEvent.pointerMove(northwest, { pointerId: 1, clientX: 14, clientY: 13 })
    expect(screen.getByRole('textbox', { name: 'Width (pt)' })).toHaveValue('68')
    expect(screen.getByRole('textbox', { name: 'Height (pt)' })).toHaveValue('21')
    fireEvent.pointerUp(northwest, { pointerId: 1, clientX: 14, clientY: 13 })
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(JSON.parse(new TextDecoder().decode((request.mock.calls[0] as unknown as [string, ArrayBuffer])[1]))).toMatchObject({ kind: 'setComponentBounds', id: 'e9', x: 4, y: 3, width: 68, height: 21 })
  })

  it('keeps oversized content paint clipped and hides off-window resize endpoints', () => {
    const placed = { id: 'e9', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72000, height: 2400000, resizable: true }
    render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: [placed] } }} />)
    const body = screen.getByLabelText('text component e9')
    fireEvent.click(body)
    expect(body.closest('.band-window')).not.toHaveClass('band-window-open')
    const chrome = document.querySelector('.canvas-selection-chrome') as HTMLElement
    expect(chrome).toHaveClass('canvas-selection-chrome-overflow')
    expect(chrome.style.getPropertyValue('--chrome-visible-height')).toBe(`${canvas.bands[1]!.height / 1000}px`)
    expect(chrome.querySelector('.selection-handle-nw')).not.toBeNull()
    expect(chrome.querySelector('.selection-handle-s')).toBeNull()
    expect(chrome.querySelector('.selection-handle-e')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Resize e9' })).not.toBeInTheDocument()
    expect(body.style.getPropertyValue('--component-height')).toBe(`${placed.height / 1000}px`)
  })

  it('focuses a pointer-selected body before keyboard deletion', async () => {
    const componentCanvas = { ...canvas, components: [{ id: 'e9', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72000, height: 24000, resizable: true }] }
    const request = vi.fn(async () => ({ snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas: { ...canvas, components: [] } } }))
    render(<App engine={engine(request)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    screen.getByRole('button', { name: 'Zoom in' }).focus()
    const component = screen.getByLabelText('text component e9')
    fireEvent.pointerDown(component, { pointerId: 1, clientX: 10, clientY: 10 })
    fireEvent.pointerUp(component, { pointerId: 1, clientX: 10, clientY: 10 })
    expect(component).toHaveFocus()
    fireEvent.keyDown(document.activeElement!, { key: 'Delete' })
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(JSON.parse(new TextDecoder().decode((request.mock.calls[0] as unknown as [string, ArrayBuffer])[1]))).toMatchObject({ kind: 'deleteComponent', id: 'e9' })
  })

  it('tracks a drag and a resize live in the geometry fields, then lands the accepted engine geometry', async () => {
    const placed = { id: 'e9', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true }
    const componentCanvas = { ...canvas, components: [placed] }
    const movedCanvas = { ...canvas, components: [{ ...placed, x: 6_000, y: 4_000 }] }
    const request = vi.fn(async () => ({ snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas: movedCanvas } }))
    render(<App engine={engine(request)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    const component = screen.getByLabelText('text component e9')
    fireEvent.click(component)
    const x = screen.getByRole('textbox', { name: 'X (pt)' })
    const y = screen.getByRole('textbox', { name: 'Y (pt)' })
    expect(x).toHaveValue('0')
    fireEvent.pointerDown(component, { pointerId: 1, clientX: 10, clientY: 10 })
    fireEvent.pointerMove(component, { pointerId: 1, clientX: 13, clientY: 12 })
    // The transient proposal the canvas paints is the value the panel shows,
    // and it cannot be typed over while the pointer owns it.
    await waitFor(() => expect(x).toHaveValue('3'))
    expect(y).toHaveValue('2')
    expect(x).toHaveAttribute('readonly')
    fireEvent.pointerMove(component, { pointerId: 1, clientX: 19, clientY: 10 })
    await waitFor(() => expect(x).toHaveValue('9'))
    expect(y).toHaveValue('0')
    fireEvent.pointerUp(component, { pointerId: 1, clientX: 19, clientY: 10 })
    // Go's accepted geometry replaces the proposal; 9 was never committed.
    await waitFor(() => expect(x).toHaveValue('6'))
    expect(y).toHaveValue('4')
    expect(x).not.toHaveAttribute('readonly')

    const width = screen.getByRole('textbox', { name: 'Width (pt)' })
    fireEvent.pointerDown(screen.getByRole('button', { name: 'Resize e9' }), { pointerId: 2, clientX: 0, clientY: 0 })
    fireEvent.pointerMove(screen.getByRole('button', { name: 'Resize e9' }), { pointerId: 2, clientX: 8, clientY: 5 })
    expect(width).toHaveValue('80')
    expect(screen.getByRole('textbox', { name: 'Height (pt)' })).toHaveValue('29')
  })

  it('keeps the drop proposal painted until the engine geometry lands', async () => {
    const placed = { id: 'e9', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true }
    const componentCanvas = { ...canvas, components: [placed] }
    const movedCanvas = { ...canvas, components: [{ ...placed, x: 6_000, y: 4_000 }] }
    let answer: (() => void) | undefined
    const request = vi.fn(async () => {
      await new Promise<void>((resolve) => { answer = resolve })
      return { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas: movedCanvas } }
    })
    render(<App engine={engine(request)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    const component = screen.getByLabelText('text component e9')
    fireEvent.pointerDown(component, { pointerId: 1, clientX: 10, clientY: 10 })
    fireEvent.pointerMove(component, { pointerId: 1, clientX: 19, clientY: 10 })
    await waitFor(() => expect(component.style.getPropertyValue('--component-x')).toBe('9px'))
    fireEvent.pointerUp(component, { pointerId: 1, clientX: 19, clientY: 10 })
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    // Red proof: dropping the proposal on pointer-up painted 0px here, so the
    // element visibly jumped back to where the drag started and stayed there
    // until Go answered. The proposal owns the paint until the answer lands.
    await waitFor(() => expect(component.style.getPropertyValue('--component-x')).toBe('9px'))
    answer!()
    await waitFor(() => expect(component.style.getPropertyValue('--component-x')).toBe('6px'))
    expect(screen.getByRole('textbox', { name: 'X (pt)' })).not.toHaveAttribute('readonly')
  })

  it('toggles Shift-click selection once without engine traffic and clears it on a click on the page itself', () => {
    const componentCanvas = { ...canvas, components: [{ id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72000, height: 24000, resizable: true }, { id: 'e2', type: 'rect' as const, band: 'content' as const, x: 80000, y: 0, width: 72000, height: 24000, resizable: true }] }
    const request = vi.fn(async () => ({ snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas: componentCanvas } }))
    render(<App engine={engine(request)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    fireEvent.pointerDown(screen.getByLabelText('text component e1'), { pointerId: 1, clientX: 1, clientY: 1 })
    fireEvent.pointerUp(screen.getByLabelText('text component e1'), { pointerId: 1, clientX: 1, clientY: 1 })
    fireEvent.pointerDown(screen.getByLabelText('rect component e2'), { pointerId: 2, clientX: 1, clientY: 1, shiftKey: true })
    fireEvent.pointerUp(screen.getByLabelText('rect component e2'), { pointerId: 2, clientX: 1, clientY: 1, shiftKey: true })
    expect(screen.getByLabelText('Resize e1')).toBeInTheDocument()
    expect(screen.getByLabelText('Resize e2')).toBeInTheDocument()
    fireEvent.click(screen.getByLabelText('Report page with Page Header, Content, and Page Footer'))
    expect(screen.queryByLabelText('Resize e1')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Resize e2')).not.toBeInTheDocument()
    expect(request).not.toHaveBeenCalled()
  })

  // Story 17.2: THE BACKDROP IS THE GREY SPACE AROUND THE PAGE, and a click
  // there used to clear the selection. It no longer does. The eight tests
  // below are one per row of the story's I/O matrix, and most of them assert
  // routes this story did NOT touch — the page surface, Escape, plain
  // selection, shift-extension — so the diff can be read as one branch
  // narrowed rather than the selection mechanism changed.
  const selectionCanvas = { ...canvas, components: [{ id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72000, height: 24000, resizable: true }, { id: 'e2', type: 'rect' as const, band: 'content' as const, x: 80000, y: 0, width: 72000, height: 24000, resizable: true }] }
  const selectionSnapshot = { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: selectionCanvas }
  const renderSelectable = () => {
    const request = vi.fn(async () => ({ snapshot: selectionSnapshot }))
    render(<App engine={engine(request)} initialSnapshot={selectionSnapshot} />)
    return request
  }
  // Selection is taken on pointerdown, not click (App.tsx:2365), so a test
  // that means "select this" has to say pointerdown.
  const pointerSelect = (label: string, pointerId: number, extend = false) => {
    const target = screen.getByLabelText(label)
    fireEvent.pointerDown(target, { pointerId, clientX: 1, clientY: 1, shiftKey: extend })
    fireEvent.pointerUp(target, { pointerId, clientX: 1, clientY: 1, shiftKey: extend })
  }
  const identityOf = (id: string, band: string) => `${id} · band: ${band}`

  it('keeps a selected component selected when the click lands on the backdrop beside the page', () => {
    const request = renderSelectable()
    pointerSelect('text component e1', 1)
    fireEvent.click(screen.getByLabelText('text component e1'))
    expect(screen.getByLabelText('Resize e1')).toBeInTheDocument()
    fireEvent.click(screen.getByLabelText('Canvas region'))
    expect(screen.getByLabelText('Resize e1')).toBeInTheDocument()
    expect(screen.getByText(identityOf('e1', 'content'))).toBeInTheDocument()
    expect(request).not.toHaveBeenCalled()
  })

  it('does nothing at all when the backdrop is clicked with nothing selected', () => {
    const request = renderSelectable()
    expect(screen.getByRole('button', { name: 'Apply page setup' })).toBeInTheDocument()
    // The positive control for the absence asserted in the last test below:
    // this is what an empty selection puts in the inspector.
    expect(screen.getByText('Component properties require a selection.')).toBeInTheDocument()
    fireEvent.click(screen.getByLabelText('Canvas region'))
    expect(screen.getByRole('button', { name: 'Apply page setup' })).toBeInTheDocument()
    expect(screen.getByText('Component properties require a selection.')).toBeInTheDocument()
    expect(screen.queryByLabelText('Resize e1')).not.toBeInTheDocument()
    expect(request).not.toHaveBeenCalled()
  })

  it('still clears a single selection when the click lands on the page surface itself', () => {
    const request = renderSelectable()
    pointerSelect('text component e1', 1)
    fireEvent.click(screen.getByLabelText('text component e1'))
    expect(screen.getByLabelText('Resize e1')).toBeInTheDocument()
    fireEvent.click(screen.getByLabelText('Report page with Page Header, Content, and Page Footer'))
    expect(screen.queryByLabelText('Resize e1')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Apply page setup' })).toBeInTheDocument()
    expect(request).not.toHaveBeenCalled()
  })

  it('still clears the selection on Escape in the canvas region, which is now the deliberate way to', () => {
    const request = renderSelectable()
    pointerSelect('text component e1', 1)
    fireEvent.click(screen.getByLabelText('text component e1'))
    expect(screen.getByLabelText('Resize e1')).toBeInTheDocument()
    const region = screen.getByLabelText('Canvas region')
    region.focus()
    fireEvent.keyDown(region, { key: 'Escape' })
    expect(screen.queryByLabelText('Resize e1')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Apply page setup' })).toBeInTheDocument()
    // AND IT IS STILL NOT GATED ON THE TARGET TEST. The keydown above has
    // target === currentTarget, so it would survive a gate; this one is raised
    // on a descendant and must clear all the same.
    pointerSelect('text component e1', 3)
    expect(screen.getByLabelText('Resize e1')).toBeInTheDocument()
    fireEvent.keyDown(screen.getByLabelText('text component e1'), { key: 'Escape' })
    expect(screen.queryByLabelText('Resize e1')).not.toBeInTheDocument()
    expect(request).not.toHaveBeenCalled()
  })

  it('still moves the selection to another component when that component is clicked', () => {
    const request = renderSelectable()
    pointerSelect('text component e1', 1)
    pointerSelect('rect component e2', 2)
    expect(screen.queryByLabelText('Resize e1')).not.toBeInTheDocument()
    expect(screen.getByLabelText('Resize e2')).toBeInTheDocument()
    expect(screen.getByText(identityOf('e2', 'content'))).toBeInTheDocument()
    expect(request).not.toHaveBeenCalled()
  })

  it('still extends the selection on a shift-click', () => {
    const request = renderSelectable()
    pointerSelect('text component e1', 1)
    pointerSelect('rect component e2', 2, true)
    expect(screen.getByLabelText('Resize e1')).toBeInTheDocument()
    expect(screen.getByLabelText('Resize e2')).toBeInTheDocument()
    expect(request).not.toHaveBeenCalled()
  })

  // THE TABLE EDITOR'S FATE ON A BACKDROP CLICK (the question Story 17.2 left
  // open). It stays. The revokeTableEditor call rode along with the clear because
  // editor is bound to the one selected component and that binding is checked
  // against `selectedRef` everywhere; the selection now survives the click, so
  // the binding does too. Revoking anyway would make a stray click MORE
  // destructive than the clear it replaced: it bumps `tableEditorSession`,
  // which App.tsx:679-681 reads to decide whether an in-flight column commit
  // may still re-project. That last part is a reading of the source, not
  // something asserted here; what this test measures is that the dialog lives.
  it('leaves an open table editor open when the backdrop is clicked', async () => {
    const tableCanvas = { ...canvas, components: [{ id: 'e7', type: 'table' as const, band: 'content' as const, x: 0, y: 0, width: 72000, height: 12000, resizable: false }] }
    const tableSnapshot = { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: tableCanvas }
    const request = vi.fn(async (operation: string) => operation === 'table-columns' ? { snapshot: tableSnapshot, tableColumns: { revision: 1, table: { tableId: 'e7', collection: 'items[]', alias: 'row', ...tableHeaderProjection, columns: [{ id: 'e8', header: 'Amount', width: 72000, proportion: '', align: 'left' as const, headerAlign: '' as const, headerAlignResolved: 'left' as const, binding: '{{row.amount}}', rowField: 'amount', rowFieldEditable: true, footer: '' as const, footerOf: '', footerFormat: '' }] } } } : { snapshot: tableSnapshot })
    render(<App engine={engine(request)} initialSnapshot={tableSnapshot} />)
    fireEvent.click(screen.getByRole('button', { name: 'table component e7' }))
    fireEvent.click(screen.getByRole('button', { name: 'Configure columns' }))
    await screen.findByRole('dialog', { name: 'Table Editor' })
    const opened = request.mock.calls.length
    fireEvent.click(screen.getByLabelText('Canvas region'))
    expect(screen.getByRole('dialog', { name: 'Table Editor' })).toBeInTheDocument()
    expect(screen.getByText(identityOf('e7', 'content'))).toBeInTheDocument()
    expect(request).toHaveBeenCalledTimes(opened)
    expect(request.mock.calls.map((call) => call[0])).not.toContain('command')
  })

  it('does not swap the inspector to page setup on a backdrop click', () => {
    const request = renderSelectable()
    pointerSelect('text component e1', 1)
    fireEvent.click(screen.getByLabelText('text component e1'))
    expect(screen.queryByRole('button', { name: 'Apply page setup' })).not.toBeInTheDocument()
    fireEvent.click(screen.getByLabelText('Canvas region'))
    expect(screen.queryByRole('button', { name: 'Apply page setup' })).not.toBeInTheDocument()
    expect(screen.queryByText('Component properties require a selection.')).not.toBeInTheDocument()
    expect(screen.getByText(identityOf('e1', 'content'))).toBeInTheDocument()
    expect(request).not.toHaveBeenCalled()
  })

  it('leaves a dirty session untouched for an open cancellation or failure', async () => {
    const files: FileAccess = { open: vi.fn().mockRejectedValueOnce(new FileAccessCancelled()).mockRejectedValueOnce(new Error('denied')), acquireSaveTarget: vi.fn(), writeSave: vi.fn() }
    render(<App engine={engine()} fileAccess={files} initialSnapshot={{ documentState: 'loaded', revision: 2, byteLength: 3 }} />)
    const open = screen.getByRole('button', { name: 'Open local template' })
    fireEvent.click(open)
    await waitFor(() => expect(files.open).toHaveBeenCalledTimes(1))
    expect(screen.getByText('Untitled template')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    fireEvent.click(open)
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Could not open local file'))
    expect(screen.getByText('Untitled template')).toBeInTheDocument()
    expect(screen.getByText('Unsaved local changes')).toBeInTheDocument()
  })

  // AN ENGINE THAT NEVER REPLIES MUST NOT TAKE THE FILE BAR WITH IT.
  //
  // `fileBusy` is the sole condition that disables Open, Save, Save As and
  // Start blank, and it is held across the engine round-trip. EngineClient
  // rejects every pending request when the worker raises or is terminated, so a
  // worker that DIES releases the bar; a worker that merely stops replying —
  // a wedged wasm call — raises nothing, so before the deadline its request
  // settled never and all four buttons stayed disabled for the life of the tab,
  // wearing `cursor: not-allowed` with no sentence anywhere saying why. The
  // reported symptom was "clicking Open does nothing".
  //
  // The double below is the wedge, not an approximation of one: it honours the
  // abort exactly as EngineClient does and is otherwise silent forever.
  it('releases the file bar and reports the failure when an engine step never replies', async () => {
    vi.useFakeTimers()
    try {
      const request = vi.fn((_operation: string, _payload?: ArrayBuffer, signal?: AbortSignal) => new Promise((_resolve, reject) => {
        signal?.addEventListener('abort', () => reject(Object.assign(new Error('Engine request was abandoned'), { code: 'REQUEST_ABORTED' })), { once: true })
      }))
      const files: FileAccess = { open: vi.fn(async () => ({ bytes, name: 'wedged.folio' })), acquireSaveTarget: vi.fn(), writeSave: vi.fn() }
      render(<App engine={engine(request as never)} fileAccess={files} initialSnapshot={{ documentState: 'loaded', revision: 2, byteLength: 3 }} />)

      const open = screen.getByRole('button', { name: 'Open local template' })
      fireEvent.click(open)
      await act(async () => { await vi.advanceTimersByTimeAsync(0) })
      expect(request).toHaveBeenCalledWith('load', bytes, expect.any(AbortSignal))

      // THE LATCH IS REAL WHILE THE STEP IS OUTSTANDING. Without this the
      // release below could pass on a bar that was never disabled.
      expect(open).toBeDisabled()
      expect(screen.getByRole('button', { name: 'Save local template' })).toBeDisabled()
      expect(screen.getByText('Opening local file…')).toBeInTheDocument()

      // One millisecond short of the deadline the bar is still held: the
      // release is the deadline's doing and not the passage of any time at all.
      await act(async () => { await vi.advanceTimersByTimeAsync(19_999) })
      expect(open).toBeDisabled()

      await act(async () => { await vi.advanceTimersByTimeAsync(1) })
      expect(open).toBeEnabled()
      expect(screen.getByRole('button', { name: 'Save local template' })).toBeEnabled()
      expect(screen.getByRole('alert')).toHaveTextContent('Could not open local file')
      expect(screen.queryByText('Opening local file…')).not.toBeInTheDocument()
      // The wedge changed nothing about the session it failed to replace.
      expect(screen.getByText('Untitled template')).toBeInTheDocument()
    } finally { vi.useRealTimers() }
  })

  // THE SETTLED STATUS GOES; THE BUSY ONE AND THE ALERT STAY.
  //
  // All three halves matter. Retiring a BUSY status would restore the original
  // defect — disabled buttons with nothing saying why — and retiring an ALERT
  // would throw away the only account of a failure the author gets.
  it('retires a settled file status on a timer, and never the busy one or the alert', async () => {
    vi.useFakeTimers()
    try {
      // One resolver per engine step, released in order: Open takes `load` and
      // then `serialize`, and the status is busy until both have landed.
      const pending: Array<(value: unknown) => void> = []
      const request = vi.fn(() => new Promise((resolve) => { pending.push(resolve) }))
      const files: FileAccess = { open: vi.fn(async () => ({ bytes, name: 'timed.folio' })), acquireSaveTarget: vi.fn(), writeSave: vi.fn() }
      render(<App engine={engine(request as never)} fileAccess={files} initialSnapshot={{ documentState: 'loaded', revision: 2, byteLength: 3 }} />)
      const flush = async () => { await act(async () => { await vi.advanceTimersByTimeAsync(0) }) }
      const settle = async () => { await act(async () => { pending.shift()?.({ snapshot: { documentState: 'loaded', revision: 5, byteLength: 3 }, bytes }) }); await flush() }

      fireEvent.click(screen.getByRole('button', { name: 'Open local template' }))
      await flush()

      // THE BUSY STATUS OUTLASTS THE WINDOW, because the buttons it explains
      // are still disabled. This is the assertion that would catch a future
      // simplification dropping the `fileBusy` gate from the effect.
      await act(async () => { await vi.advanceTimersByTimeAsync(6_000) })
      expect(screen.getByText('Opening local file…')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Open local template' })).toBeDisabled()

      await settle()
      await settle()
      expect(screen.getByText(/Opened local file timed\.folio/)).toBeInTheDocument()

      // The settled status survives to the edge of the window and not past it.
      await act(async () => { await vi.advanceTimersByTimeAsync(5_999) })
      expect(screen.getByText(/Opened local file timed\.folio/)).toBeInTheDocument()
      await act(async () => { await vi.advanceTimersByTimeAsync(1) })
      expect(screen.queryByText(/Opened local file timed\.folio/)).not.toBeInTheDocument()
    } finally { vi.useRealTimers() }
  })

  it('leaves a reported failure on screen however long the author takes to read it', async () => {
    vi.useFakeTimers()
    try {
      const files: FileAccess = { open: vi.fn(async () => { throw new FileAccessFailure('Could not open local file: NotAllowedError: permission lapsed') }), acquireSaveTarget: vi.fn(), writeSave: vi.fn() }
      render(<App engine={engine()} fileAccess={files} initialSnapshot={{ documentState: 'loaded', revision: 2, byteLength: 3 }} />)
      fireEvent.click(screen.getByRole('button', { name: 'Open local template' }))
      await act(async () => { await vi.advanceTimersByTimeAsync(0) })
      expect(screen.getByRole('alert')).toHaveTextContent('NotAllowedError: permission lapsed')
      await act(async () => { await vi.advanceTimersByTimeAsync(600_000) })
      expect(screen.getByRole('alert')).toHaveTextContent('NotAllowedError: permission lapsed')
    } finally { vi.useRealTimers() }
  })

  // A DESCRIBED BOUNDARY FAILURE REACHES THE AUTHOR INTACT.
  //
  // `fileFailureFor` builds "<what was being done>: <what the browser called
  // the throw>" at the file boundary; the catch here must not overwrite that
  // with its own generic sentence, or the evidence is erased one layer above
  // where it was preserved. Anything that is NOT a file-boundary failure still
  // gets the plain sentence — the second half of this test.
  it('reports the file boundary\'s own sentence, and keeps the plain one for everything else', async () => {
    const files: FileAccess = {
      open: vi.fn(),
      acquireSaveTarget: vi.fn(async () => ({ name: 'locked.folio', format: folioFileFormat })),
      writeSave: vi.fn()
        .mockRejectedValueOnce(new FileAccessFailure('Could not save local file: NoModificationAllowedError: The file is locked'))
        .mockRejectedValueOnce(new Error('something else entirely')),
    }
    render(<App engine={engine()} fileAccess={files} initialSnapshot={{ documentState: 'loaded', revision: 2, byteLength: 3 }} />)
    const save = screen.getByRole('button', { name: 'Save local template' })

    fireEvent.click(save)
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('NoModificationAllowedError: The file is locked'))

    fireEvent.click(save)
    await waitFor(() => expect(files.writeSave).toHaveBeenCalledTimes(2))
    expect(screen.getByRole('alert')).toHaveTextContent('Could not save local file')
    expect(screen.getByRole('alert')).not.toHaveTextContent('something else entirely')
  })

  // The sentence explaining a file action belongs with the buttons it explains.
  // Both copies used to render at the tail of the design and preview mains,
  // which scroll, so the reason a file button was disabled was routinely off
  // the bottom of the window while the deny cursor was up in the bar.
  it('renders the local file message inside the document bar, with the actions it explains', async () => {
    const files: FileAccess = { open: vi.fn(async () => ({ bytes, name: 'placed.folio' })), acquireSaveTarget: vi.fn(), writeSave: vi.fn() }
    render(<App engine={engine()} fileAccess={files} initialSnapshot={{ documentState: 'loaded', revision: 2, byteLength: 3 }} />)
    fireEvent.click(screen.getByRole('button', { name: 'Open local template' }))
    await waitFor(() => expect(screen.getByText(/Opened local file placed\.folio/)).toBeInTheDocument())
    const actions = screen.getByRole('group', { name: 'Local file actions' })
    expect(actions).toContainElement(screen.getByText(/Opened local file placed\.folio/))
    expect(screen.getByRole('banner', { name: 'Document bar' })).toContainElement(actions)
  })

  it('clears temporary busy wording after save cancellation without changing the session', async () => {
    const files: FileAccess = { open: vi.fn(), acquireSaveTarget: vi.fn(async () => { throw new FileAccessCancelled() }), writeSave: vi.fn() }
    render(<App engine={engine()} fileAccess={files} initialSnapshot={{ documentState: 'loaded', revision: 2, byteLength: 3 }} />)
    fireEvent.click(screen.getByRole('button', { name: 'Save As' }))
    await waitFor(() => expect(files.acquireSaveTarget).toHaveBeenCalledOnce())
    expect(screen.queryByText(/Preparing Save As/)).not.toBeInTheDocument()
    expect(screen.getByText('Untitled template')).toBeInTheDocument()
    expect(screen.getByText('Unsaved local changes')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('acquires a target before serialization, preserves dirty on failure, and handles the Save shortcut', async () => {
    let rejectSave = true
    const request = vi.fn(async () => ({ snapshot: { documentState: 'loaded' as const, revision: 3, byteLength: 3 }, bytes }))
    const acquireSaveTarget = vi.fn(async () => ({ name: 'untitled.folio', format: folioFileFormat }))
    const writeSave = vi.fn(async () => { if (rejectSave) throw new Error('denied'); return { name: 'untitled.folio' } })
    const files: FileAccess = { open: vi.fn(), acquireSaveTarget, writeSave }
    render(<App engine={engine(request)} fileAccess={files} initialSnapshot={{ documentState: 'loaded', revision: 3, byteLength: 3 }} />)
    fireEvent.keyDown(window, { key: 's', ctrlKey: true })
    await waitFor(() => expect(writeSave).toHaveBeenCalledTimes(1))
    expect(acquireSaveTarget).toHaveBeenCalledBefore(request)
    expect(request).toHaveBeenCalledBefore(writeSave)
    expect(screen.getByRole('alert')).toHaveTextContent('Could not save local file')
    expect(screen.getByText('Unsaved local changes')).toBeInTheDocument()
    rejectSave = false
    fireEvent.click(screen.getByRole('button', { name: 'Save local template' }))
    await waitFor(() => expect(screen.getByText('Downloaded local file untitled.folio')).toBeInTheDocument())
    expect(screen.getByText('Saved local file')).toBeInTheDocument()
  })

  it('routes Start blank through the engine and returns to an unnamed unsaved local workspace', async () => {
    const request = vi.fn(async () => ({ snapshot: { documentState: 'loaded' as const, revision: 9, byteLength: 3 }, bytes }))
    render(<App engine={engine(request)} fileAccess={{ open: vi.fn(), acquireSaveTarget: vi.fn(), writeSave: vi.fn() }} blankBytes={bytes} initialSnapshot={{ documentState: 'loaded', revision: 4, byteLength: 3 }} />)
    startBlankFromNew()
    // The third argument is the file bar's abort deadline: Start blank holds
    // `fileBusy` across this request, so an engine that never replies would
    // otherwise latch every file button off for the life of the tab.
    await waitFor(() => expect(request).toHaveBeenCalledWith('load', bytes, expect.any(AbortSignal)))
    expect(screen.getByText('Untitled template')).toBeInTheDocument()
    expect(screen.getByText('Unsaved local changes')).toBeInTheDocument()
  })

  it('keeps a noncanonical valid open dirty until the canonical engine bytes are written', async () => {
    const canonical = new Uint8Array([9, 8, 7]).buffer
    const request = vi.fn(async (operation: string) => ({ snapshot: { documentState: 'loaded' as const, revision: 7, byteLength: 3 }, ...(operation === 'serialize' ? { bytes: canonical } : {}) }))
    const files: FileAccess = { open: vi.fn(async () => ({ bytes, name: 'noncanonical.folio' })), acquireSaveTarget: vi.fn(), writeSave: vi.fn() }
    render(<App engine={engine(request)} fileAccess={files} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3 }} />)
    fireEvent.click(screen.getByRole('button', { name: 'Open local template' }))
    await waitFor(() => expect(screen.getByText(/canonical local changes need saving/)).toBeInTheDocument())
    expect(screen.getByText('Unsaved local changes')).toBeInTheDocument()
  })

  it('does not roll back or clean a newer engine revision after an older save settles', async () => {
    let releaseWrite: (() => void) | undefined
    let releaseCommit: (() => void) | undefined
    const writeSave = vi.fn(() => new Promise<{ name: string }>((resolve) => { releaseWrite = () => resolve({ name: 'untitled.folio' }) }))
    const request = vi.fn((operation: string): Promise<{ snapshot: ReturnType<typeof snapshot>; bytes?: ArrayBuffer }> => {
      if (operation === 'command') return new Promise((resolve) => { releaseCommit = () => resolve({ snapshot: snapshot(3) }) })
      return Promise.resolve({ snapshot: snapshot(2), bytes })
    })
    const files: FileAccess = { open: vi.fn(), acquireSaveTarget: vi.fn(async () => ({ name: 'untitled.folio', format: folioFileFormat })), writeSave }
    render(<App engine={engine(request)} fileAccess={files} initialSnapshot={snapshot(2)} />)
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save local template' }))
    await waitFor(() => expect(writeSave).toHaveBeenCalledOnce())
    releaseCommit!()
    await waitFor(() => expect(screen.getByTestId('engine-snapshot')).toHaveTextContent('REVISION 3'))
    releaseWrite!()
    await waitFor(() => expect(screen.getByText('Unsaved local changes')).toBeInTheDocument())
    expect(screen.getByTestId('engine-snapshot')).toHaveTextContent('REVISION 3')
    expect(screen.getByText(/newer local changes need saving/)).toBeInTheDocument()
  })

  it('paints each Go band rectangle at its projected origin and uses one zoomed display scale for page and grid', () => {
    render(<App initialSnapshot={snapshot(1)} />)
    const page = screen.getByLabelText('Report page with Page Header, Content, and Page Footer')
    const header = screen.getByLabelText('Page Header')
    expect(page.style.getPropertyValue('--page-display-width')).toBe('595.276px')
    expect(page.style.getPropertyValue('--page-display-height')).toBe('841.89px')
    expect(page.style.getPropertyValue('--grid-display-pitch')).toBe('6px')
    expect(header.style.getPropertyValue('--band-x')).toBe('36px')
    expect(header.style.getPropertyValue('--band-y')).toBe('36px')
    expect(header.style.getPropertyValue('--band-width')).toBe('523.276px')
    expect(header.style.getPropertyValue('--band-height')).toBe('20px')
    fireEvent.click(screen.getByRole('button', { name: 'Zoom in' }))
    expect(page.style.getPropertyValue('--page-display-width')).toBe('654.8036px')
    expect(page.style.getPropertyValue('--grid-display-pitch')).toBe('6.6px')
  })

  it('paints only pre-broken engine text lines without changing local document state', () => {
    const textCanvas = { ...canvas, components: [{ id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72000, height: 24000, resizable: true, value: 'do not paint this value', textPaint: { overflow: false, truncated: false, lines: [{ top: 0, baseline: 12000, advance: 16000, width: 24000, fragments: [{ text: 'engine ', x: 0 }, { text: 'line', x: 16000 }] }] } }] }
    const request = vi.fn()
    render(<App engine={engine(request)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: textCanvas }} />)
    expect(screen.getByText('engine', { exact: true })).toBeInTheDocument()
    expect(screen.getByText('line', { exact: true })).toBeInTheDocument()
    expect(screen.queryByText('do not paint this value')).not.toBeInTheDocument()
    expect(screen.getByLabelText('text component e1: engine line')).toBeInTheDocument()
    expect(document.querySelector('.canvas-text-line')).toHaveStyle({ '--text-line-baseline': '12px', '--text-line-advance': '16px' })
    expect(request).not.toHaveBeenCalled()
  })

  // STORY 8.4a. THE FACE THE DOCUMENT ITSELF CARRIES, PAINTED WITH.
  //
  // The engine measures and renders with a face out of the document's own
  // `assets` map and attributes each painted fragment to the asset it resolved
  // to. Without this the browser had no CSS family for such a face at all, so
  // the canvas rasterized at the engine's x-positions with a fallback's
  // metrics and the glyphs collided — the reported defect, rebuilt for a
  // document that carries its own typeface.
  //
  // THE PAGE'S FONT SET IS STUBBED because jsdom implements none: it defines
  // no `FontFace` and no font set on `document`, so both are installed with
  // `Object.defineProperty` (neither exists to be assigned over). Nothing here
  // measures anything, and the seam under test computes no metric either —
  // every x, advance and line break in the fixture is the engine's.
  it('registers a carried face once for the whole document and paints its fragments with the family derived from its key', async () => {
    const key = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
    const fontSet = installStubFontSet()
    try {
      const requested: string[] = []
      const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
        if (operation === 'asset') { requested.push(new TextDecoder().decode(payload)); return { snapshot: snapshot(1), bytes: new Uint8Array([0, 1, 2, 3]).buffer } }
        return { snapshot: snapshot(1) }
      })
      const view = render(<App engine={engine(request) } initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: carriedFaceCanvas(key) }} />)
      // ONE REQUEST FOR THE WHOLE DOCUMENT, not one per component. The nearest
      // precedent, ImagePaint, is mounted once per component AND once per
      // repeated sheet; copying that lifetime here would add one face many
      // times under a single global family name and let an unmounting instance
      // delete a face another is still painting with. The fixture has two text
      // components drawing through the same carried entry precisely so a
      // per-component lifetime would show up as two requests.
      await waitFor(() => expect(requested).toEqual([key]))
      const painted = () => Array.from(view.container.querySelectorAll('.canvas-text-fragment')) as HTMLElement[]
      expect(painted().length).toBe(2)
      await waitFor(() => expect(painted().map((node) => node.style.fontFamily)).toEqual([embeddedFaceFamily(key), embeddedFaceFamily(key)]))
      // AND THE ENGINE'S OWN GEOMETRY IS UNTOUCHED BY IT (AD-17): the fragment
      // still paints at the x the engine measured, and the family is the only
      // thing this story added to it.
      expect(painted()[0]).toHaveStyle({ '--text-fragment-x': '0px' })
      expect(screen.getAllByLabelText(/text component e1/)[0]).toBeInTheDocument()
    } finally {
      fontSet.restore()
    }
  })

  // EVERY CONTENT WINDOW AFTER THE FIRST IS AN ECHO, so a document that runs
  // past one sheet paints most of itself through ComponentEcho rather than
  // through the home occurrence. A carried face that reached only the home
  // would leave sheets 2..N rasterizing at the ENGINE'S x-positions on the
  // fallback stack — precisely the collision this story exists to remove, on
  // most of the pages.
  //
  // MUTATION PROOF, RUN AND RECORDED: replacing `carriedFaces={carriedFaces}`
  // with `carriedFaces={NO_CARRIED_FACES}` in ComponentEcho reddens THIS test
  // on the echoed fragment and leaves every other designer test green — which
  // is why the assertion is written against the echoed node specifically and
  // not against the container's fragments as a set.
  it('paints a repeated sheet\'s echo with the carried face, not only the component\'s home', async () => {
    const key = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
    const fontSet = installStubFontSet()
    try {
      const request = vi.fn(async (operation: string) => operation === 'asset'
        ? { snapshot: snapshot(1), bytes: new Uint8Array([0, 1, 2, 3]).buffer }
        : { snapshot: snapshot(1) })
      const view = render(<App engine={engine(request) } initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: carriedFaceEchoCanvas(key) }} />)
      const home = () => Array.from(view.container.querySelectorAll('.canvas-component:not(.canvas-component-echo) .canvas-text-fragment')) as HTMLElement[]
      const echoed = () => Array.from(view.container.querySelectorAll('.canvas-component-echo .canvas-text-fragment')) as HTMLElement[]
      // The fixture really does produce both, or the claim below is vacuous.
      expect(home()).toHaveLength(1)
      expect(echoed()).toHaveLength(1)
      await waitFor(() => expect(echoed()[0]!.style.fontFamily).toBe(embeddedFaceFamily(key)))
      expect(home()[0]!.style.fontFamily).toBe(embeddedFaceFamily(key))
      // AD-17 on the echo too: the engine's own x is what it paints at, and
      // the family is the only thing this story put on it.
      expect(echoed()[0]!).toHaveStyle({ '--text-fragment-x': '0px' })
    } finally {
      fontSet.restore()
    }
  })

  // STORY 8.4e. THE FACE THE BUILD SHIPS, ASKED FOR BY THE NAME THE ENGINE
  // MEASURED WITH — AT THE HOME FRAGMENT AND AT THE ECHO.
  //
  // Until this story a shipped fragment set no family at all and fell to
  // `.canvas-text-fragment`'s fixed stack, which names all three shipped faces
  // in one Latin-first order whatever order the document declared. All three
  // faces cover `A` and `5` (their cmaps overlap 339 / 529 / 230 codepoints
  // pairwise, measured), so a document whose chain is ["Noto Sans Thai"] had
  // its Latin MEASURED with Noto Sans Thai and RASTERIZED with Noto Sans:
  // right glyphs, wrong advances, creeping out of position at the engine's own
  // x. The fragment now names the attributed face FIRST.
  //
  // WHAT THIS LAYER CAN AND CANNOT PROVE, said out loud rather than implied.
  // jsdom applies no stylesheet and loads no font, so "rasterized with" is not
  // observable here; what is observable — and is what the fix consists of — is
  // that the engine's name reaches the element and the element asks for it
  // first. The executed browser assertion is owed at the epic gate, once CI
  // runs the Playwright suite (D-8.4.25(b), (d), (e)).
  //
  // MUTATION PROOF, RUN AND RECORDED: dropping the shipped branch from
  // TextPaint's inline style reddens this test at BOTH nodes; replacing
  // `carriedFaces={carriedFaces}` with `NO_CARRIED_FACES` in ComponentEcho
  // does NOT redden it, which is why the echo is asserted directly.
  it('paints a shipped-face fragment with the face the engine attributed it to, at the home occurrence and at the echo', async () => {
    const operations: string[] = []
    const request = vi.fn(async (operation: string) => { operations.push(operation); return { snapshot: snapshot(1) } })
    const view = render(<App engine={engine(request) } initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: shippedFaceEchoCanvas('Noto Sans Thai') }} />)
    const home = () => Array.from(view.container.querySelectorAll('.canvas-component:not(.canvas-component-echo) .canvas-text-fragment')) as HTMLElement[]
    const echoed = () => Array.from(view.container.querySelectorAll('.canvas-component-echo .canvas-text-fragment')) as HTMLElement[]
    // The fixture really does produce both, or the claim below is vacuous.
    expect(home()).toHaveLength(1)
    expect(echoed()).toHaveLength(1)
    for (const node of [home()[0]!, echoed()[0]!]) {
      expect(familiesAskedFor(node)[0]).toBe('Noto Sans Thai')
      // AND THE DECLARED STACK IS STILL BEHIND IT. An inline declaration
      // replaces the rule rather than extending it, so a codepoint the
      // attributed face does not cover must still reach the other shipped
      // faces rather than the browser's default.
      expect(familiesAskedFor(node)).toEqual(['Noto Sans Thai', 'Noto Sans', 'Noto Sans SC', 'sans-serif'])
      // AD-17 on both: the engine's own x is what it paints at, and the family
      // is the only thing this story put on it.
      expect(node).toHaveStyle({ '--text-fragment-x': '0px' })
    }
    // NOTHING WAS FETCHED AND NOTHING WAS REGISTERED. A shipped face is
    // declared at build time; it needs no `asset` request and no runtime seam,
    // and asking for one would be a per-document cost this story does not add.
    expect(operations).not.toContain('asset')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getAllByLabelText(/text component e1/)[0]).toBeInTheDocument()
  })

  // THE UNATTRIBUTED FRAGMENT, which is what the stylesheet's stack is FOR
  // now. A fragment carrying neither identity is legal on the wire — the
  // browser's guard admits it deliberately — and it must paint, on the
  // declared stack, with no inline family of its own. This is the direction
  // that keeps the assertion above from passing for the wrong reason.
  it('leaves a fragment the engine attributed to nothing on the stylesheet\'s declared stack', async () => {
    const bare = shippedFaceEchoCanvas('Noto Sans Thai')
    const unattributed = { ...bare, components: bare.components.map((component) => ({ ...component, textPaint: { ...component.textPaint, lines: component.textPaint.lines.map((line) => ({ ...line, fragments: line.fragments.map(({ text, x }) => ({ text, x })) })) } })) }
    const view = render(<App engine={engine(vi.fn(async () => ({ snapshot: snapshot(1) })))} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: unattributed }} />)
    const painted = Array.from(view.container.querySelectorAll('.canvas-text-fragment')) as HTMLElement[]
    expect(painted).toHaveLength(2)
    for (const node of painted) {
      expect(node.style.fontFamily).toBe('')
      expect(node).toHaveStyle({ '--text-fragment-x': '0px' })
    }
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  // THE DEGRADE PATH, stated as a claim about the SESSION. An inline family
  // REPLACES the stylesheet rule rather than extending it, so asking for a
  // family whose bytes never arrived would take the fragment off the declared
  // stack onto whatever the browser defaults to. A failed asset request must
  // therefore leave the fragment exactly as a shipped-face fragment: painted,
  // named, and on the declared stack — and it must never reach the engine's
  // failure channel.
  //
  // IT IS READ AFTER THE CHAIN HAS SETTLED, WHICH IS THE WHOLE DIFFICULTY.
  // "The fragment has no family" is ALSO true of a registration that simply
  // has not finished, so asserting it the instant the request was issued
  // proves "not yet", not "degraded" — the identical assertion passes at that
  // instant in the SUCCESS case. The document therefore carries TWO carried
  // faces: one whose bytes are withheld until the other has already been
  // REJECTED, so the first face's arrival in the font set is a positive
  // condition that cannot hold until the failure was handled.
  //
  // MUTATION PROOF, RUN AND RECORDED: returning bytes for `unfetchable`
  // instead of throwing reddens this test — the second fragment acquires its
  // family and the font set holds two families, not one.
  it('keeps painting on the declared stack when a carried face\'s bytes cannot be fetched', async () => {
    const fetchable = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
    const unfetchable = 'fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210'
    const fontSet = installStubFontSet()
    try {
      let release: () => void = () => undefined
      const withheld = new Promise<ArrayBuffer>((resolve) => { release = () => resolve(new Uint8Array([0, 1, 2, 3]).buffer) })
      const requested: string[] = []
      const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
        if (operation !== 'asset') return { snapshot: snapshot(1) }
        const key = new TextDecoder().decode(payload)
        requested.push(key)
        if (key === unfetchable) throw Object.assign(new Error('no such asset'), { code: 'ASSET_UNAVAILABLE' })
        return { snapshot: snapshot(1), bytes: await withheld }
      })
      const view = render(<App engine={engine(request) } initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: twoCarriedFacesCanvas(fetchable, unfetchable) }} />)
      // Both were asked for; the unfetchable one has already rejected, because
      // its bytes were never withheld behind anything.
      await waitFor(() => expect([...requested].sort()).toEqual([fetchable, unfetchable].sort()))
      release()
      // THE POSITIVE CONDITION. The font set cannot hold the fetchable face
      // until its bytes were released, which happened after the other request
      // rejected — so everything below is read on a settled chain.
      await waitFor(() => expect(fontSet.added).toEqual([embeddedFaceFamily(fetchable)]))
      const painted = () => Array.from(view.container.querySelectorAll('.canvas-text-fragment')) as HTMLElement[]
      expect(painted().length).toBe(2)
      await waitFor(() => expect(painted()[0]!.style.fontFamily).toBe(embeddedFaceFamily(fetchable)))
      expect(painted()[1]!.style.fontFamily).toBe('')
      expect(painted()[1]!).toHaveStyle({ '--text-fragment-x': '0px' })
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
      expect(screen.getAllByLabelText(/text component e1/)[0]).toBeInTheDocument()
    } finally {
      fontSet.restore()
    }
  })

  // THE KEY IS A STRING FROM THE DOCUMENT, AND IT BECOMES A CSS FAMILY. The
  // projection admits a FRAGMENT's `assetKey` only as 64 lowercase hex, but a
  // CHAIN ENTRY's key — the one this effect fetches bytes for and derives the
  // family from — is admitted on length alone, so the shape is asserted at the
  // derivation. A key that is not one is a carried face the browser declines:
  // no request, no family, no registration, and the fragment stays on the
  // stylesheet's declared stack. It is a DEGRADE and not a refusal — nothing
  // reaches the failure channel and the worker is untouched.
  it('declines a chain entry whose asset key is not an asset key, rather than turning it into a family', async () => {
    const wellFormed = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
    const injected = 'IBM Plex Sans, monospace'
    const fontSet = installStubFontSet()
    try {
      const requested: string[] = []
      const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
        if (operation === 'asset') { requested.push(new TextDecoder().decode(payload)); return { snapshot: snapshot(1), bytes: new Uint8Array([0, 1, 2, 3]).buffer } }
        return { snapshot: snapshot(1) }
      })
      const view = render(<App engine={engine(request) } initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: twoCarriedFacesCanvas(wellFormed, injected) }} />)
      // The well-formed sibling settles, so the claim about the malformed one
      // is read after the effect has done everything it is going to do.
      await waitFor(() => expect(fontSet.added).toEqual([embeddedFaceFamily(wellFormed)]))
      expect(requested).toEqual([wellFormed])
      const painted = () => Array.from(view.container.querySelectorAll('.canvas-text-fragment')) as HTMLElement[]
      await waitFor(() => expect(painted()[0]!.style.fontFamily).toBe(embeddedFaceFamily(wellFormed)))
      // Not a family, not a partial family, not the string itself.
      expect(painted()[1]!.style.fontFamily).toBe('')
      expect(view.container.innerHTML).not.toContain(embeddedFaceFamily(injected))
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
      expect(screen.getAllByLabelText(/text component e2/)[0]).toBeInTheDocument()
    } finally {
      fontSet.restore()
    }
  })

  // THE LISTING IS PART OF THE EFFECT'S KEY, AND THIS IS THE CASE THAT PROVES
  // IT. `documentGenerationValue` advances only when the document is REPLACED
  // — open a file, new template, undo/redo — and an ordinary property commit
  // is none of those: it commits through setCurrentSnapshot without clearing
  // the document interaction, so the generation does not move.
  //
  // Release is invisible in the DOM — the fragment simply stops asking for the
  // family — so the claim is made against the font set's own record.
  //
  // MUTATION PROOF, RUN AND RECORDED: reducing the effect's dependency array
  // to `[engine, documentGenerationValue]` reddens this test and leaves the
  // rest of the designer suite green.
  //
  // TRIGGER CHANGED BY STORY 16.9: this used to drop the carried entry through
  // the chain editor's own 'Remove entry' control, which no longer exists —
  // the story removed the UI, not the chain data or the property-commit path.
  // The effect under test reacts to ANY snapshot update that narrows
  // `fontChains`, regardless of which command produced it, so an ordinary
  // Bold toggle (still available, still a same-document 'command') is used to
  // deliver the identical projection change the removed control used to.
  it('releases a carried face when a same-document commit drops the chain entry that carried it', async () => {
    const key = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
    const fontSet = installStubFontSet()
    try {
      const carrying = carriedFaceCanvas(key)
      const dropped = { ...carrying, fontChains: [{ name: 'body', entries: [face('Noto Sans')] }], components: carrying.components.map((component) => ({ ...component, bold: true })) }
      const loaded = (revision: number, projection: typeof carrying) => ({ documentState: 'loaded' as const, revision, byteLength: 3, canvas: projection })
      const request = vi.fn(async (operation: string) => {
        if (operation === 'asset') return { snapshot: loaded(1, carrying), bytes: new Uint8Array([0, 1, 2, 3]).buffer }
        if (operation === 'command') return { snapshot: loaded(2, dropped) }
        return { snapshot: loaded(1, carrying) }
      })
      render(<App engine={engine(request) } initialSnapshot={loaded(1, carrying)} />)
      await waitFor(() => expect(fontSet.added).toEqual([embeddedFaceFamily(key)]))
      expect(fontSet.removed).toEqual([])
      fireEvent.click(screen.getAllByLabelText(/text component e1/)[0]!)
      // Any same-document command that returns a narrowed `fontChains`
      // exercises the same effect the deleted chain-editor control drove.
      fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
      await waitFor(() => expect(fontSet.removed).toEqual([embeddedFaceFamily(key)]))
      // And it was released rather than re-registered: the document was never
      // replaced, so nothing asked for those bytes a second time.
      expect(fontSet.added).toEqual([embeddedFaceFamily(key)])
      expect(document.querySelectorAll('.canvas-text-fragment')).toHaveLength(2)
      expect(Array.from(document.querySelectorAll('.canvas-text-fragment')).map((node) => (node as HTMLElement).style.fontFamily)).toEqual(['', ''])
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    } finally {
      fontSet.restore()
    }
  })

  it('retains literal empty drafts, announces the precise engine diagnostic, and ignores a stale Apply draft reset', async () => {
    let resolveApply: ((value: { snapshot: ReturnType<typeof snapshot> }) => void) | undefined
    const request = vi.fn((operation: string) => operation === 'command' ? new Promise<{ snapshot: ReturnType<typeof snapshot> }>((resolve) => { resolveApply = resolve }) : Promise.resolve({ snapshot: snapshot(1) }))
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} />)
    const top = screen.getByRole('textbox', { name: 'Top margin (pt)' })
    fireEvent.change(top, { target: { value: '37' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    fireEvent.change(top, { target: { value: '38' } })
    resolveApply!({ snapshot: snapshot(2) })
    await waitFor(() => expect(top).toHaveValue('38'))
    request.mockRejectedValueOnce(Object.assign(new Error('must not be negative'), { code: 'PAGE_SETUP_INVALID', dataPath: 'page.margin.top' }))
    fireEvent.change(top, { target: { value: '' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('page.margin.top: must not be negative'))
    expect(top).toHaveValue('')
  })

  // -------------------------------------------------------------------------
  // STORY 12.1: THE BAND-HEIGHT ROWS.
  //
  // Band.Height had no writer anywhere in the product. These four tests are
  // about the two rows that now write it, and each asks the Story 12.4
  // question of itself: what would have to change for this to fail?

  it('shows the engine\'s own band heights and sends each row to the band it names', async () => {
    const request = vi.fn(async () => ({ snapshot: snapshot(2) }))
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} />)
    const header = screen.getByRole('textbox', { name: 'Page header height (pt)' })
    const footer = screen.getByRole('textbox', { name: 'Page footer height (pt)' })
    // The rows show what the ENGINE projected (20000 millipoints for both
    // bands), never a default and never a browser measurement.
    expect(header).toHaveValue('20')
    expect(footer).toHaveValue('20')
    fireEvent.change(header, { target: { value: '80' } })
    fireEvent.change(footer, { target: { value: '30' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(request).toHaveBeenCalledTimes(3))
    const sent = (request.mock.calls as unknown as ReadonlyArray<[string, ArrayBuffer]>).map(([, payload]) => new TextDecoder().decode(payload))
    // THE ROW→BAND MAP, PINNED TO THE BYTE. "A band-height command was sent"
    // passes just as happily with the two rows crossed — which is exactly the
    // defect a key→edge map rotation produced elsewhere in this repository
    // while its table test stayed green.
    // `"snap":false` is the fifth field Story 12.5 added, and the panel passes
    // it FALSE on every row: the box is typed, so an author who types 80 gets
    // 80. That is what 12.1's byte-identity criterion preserved — the DOCUMENT
    // bytes this path writes — while the command payload deliberately moved.
    expect(sent[0]).toBe('{"kind":"setBandHeight","version":1,"band":"pageHeader","height":80,"snap":false}')
    expect(sent[1]).toBe('{"kind":"setBandHeight","version":1,"band":"pageFooter","height":30,"snap":false}')
    // And the band heights go BEFORE the page setup, so the common refusal
    // leaves the document wholly unchanged.
    expect(sent[2]).toContain('"kind":"pageSetup"')
  })

  it('sends nothing for a band-height row the author did not change', async () => {
    const request = vi.fn(async () => ({ snapshot: snapshot(2) }))
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} />)
    fireEvent.change(screen.getByRole('textbox', { name: 'Page footer height (pt)' }), { target: { value: '30' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(request).toHaveBeenCalledTimes(2))
    const sent = (request.mock.calls as unknown as ReadonlyArray<[string, ArrayBuffer]>).map(([, payload]) => new TextDecoder().decode(payload))
    expect(sent.filter((command) => command.includes('setBandHeight'))).toEqual(['{"kind":"setBandHeight","version":1,"band":"pageFooter","height":30,"snap":false}'])
    expect(sent.some((command) => command.includes('"band":"pageHeader"'))).toBe(false)
  })

  it('sends no band-height command at all when only a margin changed', async () => {
    // AC2. A row the author did not touch is worth no command, no round trip
    // and no history entry, and when NEITHER band-height row was touched the
    // Apply is exactly the one command it was before Story 12.1. Nothing here
    // computes a bound; it compares the engine's own spelling of its own
    // number against the box beside it.
    //
    // THIS TEST USED TO CLAIM MORE THAN IT COULD. Its name and its comment said
    // it kept an ALREADY-STRANDED hand-edited document editable — a scenario
    // that cannot occur: engine-protocol.ts's isCanvas rejects a projection
    // carrying a stranded component, so such a document terminates the worker
    // when it is opened and is never edited at all. The test was green only
    // because it mocks the engine and never runs that guard. The property below
    // is real and is all that is asserted.
    const request = vi.fn(async () => ({ snapshot: snapshot(2) }))
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} />)
    fireEvent.change(screen.getByRole('textbox', { name: 'Top margin (pt)' }), { target: { value: '37' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    const only = new TextDecoder().decode((request.mock.calls[0] as unknown as [string, ArrayBuffer])[1])
    expect(only).toContain('"kind":"pageSetup"')
    expect(only).not.toContain('setBandHeight')
  })

  it('renders the engine\'s own located band-height refusal, never the fixed page-setup sentence', async () => {
    // The refusal names the ACT and the element: the height that was refused,
    // and what would have been stranded by it. Routing it through
    // pageSetupDiagnostic instead would throw all of that away and print a
    // sentence about size and margins, neither of which the author touched.
    const refusal = 'a pageHeader height of 79pt would leave e1 outside the band: it reaches 80000mp'
    const request = vi.fn((operation: string) => operation === 'command'
      ? Promise.reject(Object.assign(new Error(refusal), { elementId: 'e1', dataPath: 'bands.pageHeader.height' }))
      : Promise.resolve({ snapshot: snapshot(1) }))
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} />)
    fireEvent.change(screen.getByRole('textbox', { name: 'Page header height (pt)' }), { target: { value: '79' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent(`e1: ${refusal}`))
    expect(screen.getByRole('alert')).not.toHaveTextContent('Page setup is invalid. Check the selected size and margins.')
    // The sequence STOPPED at the first refusal: the pageSetup command that
    // would have followed was never sent, so the document is wholly unchanged.
    expect(request).toHaveBeenCalledOnce()
    // NO BROWSER-SIDE FLOOR (Story 17.4 item 9): the refused value stays in the
    // box exactly as typed. A panel that clamped it would have shown the author
    // a number they did not enter beside a refusal about the one they did.
    expect(screen.getByRole('textbox', { name: 'Page header height (pt)' })).toHaveValue('79')
  })

  it('shows no row and sends no command for a band the projection does not carry', async () => {
    // ABSENT IS NOT ZERO. projectedBandHeight returns undefined for a band the
    // engine did not project, and a `?? 0` there would seed the row with a
    // legal-looking height nobody projected — which then DIFFERS from any draft
    // and sends a band-height command built on the fabricated number.
    const footerless = { ...canvas, bands: canvas.bands.filter((band) => band.name !== 'pageFooter') }
    const request = vi.fn(async () => ({ snapshot: snapshot(2) }))
    render(<App engine={engine(request)} initialSnapshot={{ ...snapshot(1), canvas: footerless }} />)
    expect(screen.queryByRole('textbox', { name: 'Page footer height (pt)' })).toBeNull()
    expect(screen.getByRole('textbox', { name: 'Page header height (pt)' })).toHaveValue('20')
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(new TextDecoder().decode((request.mock.calls[0] as unknown as [string, ArrayBuffer])[1])).toContain('"kind":"pageSetup"')
  })

  it('keeps the typed margin standing when a band height is accepted and the page setup that follows is refused', async () => {
    // THE RESIDUE THE SPEC DISCLOSES, AND THE ONE PLACE THE MID-SEQUENCE
    // SNAPSHOT'S keepNewerDraft FLAG IS OBSERVABLE. The band height is
    // ACCEPTED, so a snapshot comes back and is installed mid-gesture; the
    // pageSetup command that follows is then REFUSED. If that install reseeded
    // the drafts, the margin the author typed would be wiped out of its box by
    // a command that never carried it — beside a refusal telling them the page
    // setup was rejected. Flipping the `true` in applyPageSetup's
    // setCurrentSnapshot(result.snapshot, true) to `false` reddens exactly this
    // test.
    const request = vi.fn((operation: string, payload?: ArrayBuffer) => {
      if (operation !== 'command') return Promise.resolve({ snapshot: snapshot(1) })
      return new TextDecoder().decode(payload as ArrayBuffer).includes('setBandHeight')
        ? Promise.resolve({ snapshot: snapshot(2) })
        : Promise.reject(new Error('the engine said something about page setup'))
    })
    render(<App engine={engine(request as never)} initialSnapshot={snapshot(1)} />)
    fireEvent.change(screen.getByRole('textbox', { name: 'Page header height (pt)' }), { target: { value: '80' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'Top margin (pt)' }), { target: { value: '37' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    // THE PAGE-SETUP SENTENCE, not a component one: the second half of the
    // gesture is a pageSetup command and its refusal is phrased by
    // pageSetupDiagnostic, which has no engine message to show for an
    // unlocated rejection.
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Page setup is invalid. Check the selected size and margins.'))
    expect(request).toHaveBeenCalledTimes(2)
    // AND THE BOXES STILL HOLD WHAT THE AUTHOR TYPED. The margin never reached
    // the document, so the panel must not pretend it did or that it never
    // existed.
    expect(screen.getByRole('textbox', { name: 'Top margin (pt)' })).toHaveValue('37')
    expect(screen.getByRole('textbox', { name: 'Page header height (pt)' })).toHaveValue('80')
  })

  it('abandons the rest of an Apply when the document is replaced mid-sequence', async () => {
    // APPLY IS A SEQUENCE OF UP TO THREE AWAITED ROUND TRIPS and the Apply
    // button stays live throughout, so the author can undo, open a file or
    // start a blank template between two of them — each of which REPLACES the
    // document. A later command of this sequence landing on that document would
    // carry heights and margins read from a document that is gone. Every other
    // async path in this file guards exactly this (applyProperties,
    // applyImageAsset, the binding commit) and so does this one.
    let releaseBandHeight: ((value: unknown) => void) | undefined
    const request = vi.fn((operation: string, payload?: ArrayBuffer) => {
      if (operation === 'undo') return Promise.resolve({ snapshot: { ...snapshot(9), canUndo: false } })
      if (operation !== 'command') return Promise.resolve({ snapshot: snapshot(1) })
      if (new TextDecoder().decode(payload as ArrayBuffer).includes('setBandHeight')) return new Promise((resolve) => { releaseBandHeight = resolve })
      return Promise.resolve({ snapshot: snapshot(3) })
    })
    render(<App engine={engine(request as never)} initialSnapshot={{ ...snapshot(1), canUndo: true }} />)
    fireEvent.change(screen.getByRole('textbox', { name: 'Page header height (pt)' }), { target: { value: '80' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'Top margin (pt)' }), { target: { value: '37' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(releaseBandHeight).toBeDefined())
    // The document is replaced while the band height is still in flight.
    fireEvent.click(screen.getByRole('button', { name: 'Undo' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Undo' })).toBeDisabled())
    await act(async () => { releaseBandHeight?.({ snapshot: snapshot(2) }) })
    const commands = request.mock.calls.filter(([operation]) => operation === 'command').map(([, payload]) => new TextDecoder().decode(payload as ArrayBuffer))
    // THE pageSetup COMMAND WAS NEVER SENT. Without the generation guard it
    // would have been, against a document the author has already left.
    expect(commands).toHaveLength(1)
    expect(commands[0]).toContain('setBandHeight')
  })

  it('starts no second Apply while the first is still in flight', async () => {
    let releaseBandHeight: ((value: unknown) => void) | undefined
    const request = vi.fn((operation: string, payload?: ArrayBuffer) => {
      if (operation !== 'command') return Promise.resolve({ snapshot: snapshot(1) })
      if (new TextDecoder().decode(payload as ArrayBuffer).includes('setBandHeight')) {
        if (releaseBandHeight) return Promise.resolve({ snapshot: snapshot(2) })
        return new Promise((resolve) => { releaseBandHeight = resolve })
      }
      return Promise.resolve({ snapshot: snapshot(3) })
    })
    render(<App engine={engine(request as never)} initialSnapshot={snapshot(1)} />)
    fireEvent.change(screen.getByRole('textbox', { name: 'Page header height (pt)' }), { target: { value: '80' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(releaseBandHeight).toBeDefined())
    // A second press while the first sequence is mid-flight: two interleaved
    // sequences would send band heights derived from drafts either of them may
    // already have superseded, and would double every history entry.
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    expect(request).toHaveBeenCalledTimes(1)
    await act(async () => { releaseBandHeight?.({ snapshot: snapshot(2) }) })
    // One band height, one page setup, and nothing from the two extra presses.
    expect(request.mock.calls.filter(([operation]) => operation === 'command')).toHaveLength(2)
  })

  // -------------------------------------------------------------------------
  // STORY 12.2: THE DOCUMENT LOCALE AND UTC OFFSET ROWS.
  //
  // `locale` and `utcOffset` had no writer anywhere in the product, and were
  // not even projected. These tests are about the two rows that now write them,
  // and each asks the same question of itself as 12.1's: what would have to
  // change for this to fail? A bare "a document-settings command was sent"
  // passes just as happily with the two commands crossed.

  it('shows the engine\'s own locale and offset, and offers exactly the four tags', () => {
    render(<App engine={engine()} initialSnapshot={snapshot(1)} />)
    // FROM THE PROJECTION AND FROM NOTHING ELSE. The fixture declares `en` and
    // `+07:00`; a default of `en`/`+00:00` in the panel would agree with the
    // first and disagree with the second, which is why they differ here.
    expect(screen.getByRole('combobox', { name: 'Document locale' })).toHaveValue('en')
    expect(screen.getByRole('textbox', { name: 'UTC offset (±HH:MM)' })).toHaveValue('+07:00')
    // THE OPTIONS ARE AD-12'S CLOSED SET, and the assertion is exact in both
    // directions: a fifth option is as wrong as a missing fourth, because the
    // panel may only propose values the loader will accept.
    const options = within(screen.getByRole('combobox', { name: 'Document locale' })).getAllByRole('option')
    expect(options.map((option) => (option as HTMLOptionElement).value)).toEqual([...LOCALE_TAGS])
    // The VISIBLE text is the tag itself, not a display name: the `.folio` file
    // says `zh-Hans`, and a display-name map would be a fifth artifact keyed by
    // the tag set, needing a tie of its own.
    expect(options.map((option) => option.textContent)).toEqual([...LOCALE_TAGS])
  })

  it('does not assert a locale for a document that has said nothing', () => {
    // NO CANVAS. `draftFor(undefined)` seeds `locale: ''`, and no tag option
    // carries that value — so without the disabled placeholder the browser
    // paints the FIRST option and the control reads `en` for a document that
    // does not exist.
    //
    // WHAT WOULD HAVE TO CHANGE FOR THIS TO FAIL: delete the placeholder from
    // PageSetup. Nothing else in the suite would notice, because the BEHAVIOUR
    // is already safe here — Apply is disabled and applyPageSetup returns early
    // — so no command assertion can see it. That is exactly why the display
    // needs its own test: this is Story 17.3's defect (a control showing a
    // default the document never chose) wearing a select instead of a number.
    render(<App engine={engine()} />)
    const locale = screen.getByRole('combobox', { name: 'Document locale' })
    expect(locale).toHaveValue('')
    expect(locale).not.toHaveValue('en')
    // The placeholder exists, is first, and cannot be chosen — a selectable
    // placeholder would let the author propose `''`, which the loader refuses.
    const options = within(locale).getAllByRole('option')
    expect(options.map((option) => (option as HTMLOptionElement).value)).toEqual(['', ...LOCALE_TAGS])
    expect((options[0] as HTMLOptionElement).disabled).toBe(true)
    // And the offset row shows emptiness rather than a fabricated `+00:00`.
    expect(screen.getByRole('textbox', { name: 'UTC offset (±HH:MM)' })).toHaveValue('')
  })

  it('sends exactly the locale command when only the locale changed', async () => {
    const request = vi.fn(async () => ({ snapshot: snapshot(2) }))
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} />)
    fireEvent.change(screen.getByRole('combobox', { name: 'Document locale' }), { target: { value: 'th' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(request).toHaveBeenCalledTimes(2))
    const sent = (request.mock.calls as unknown as ReadonlyArray<[string, ArrayBuffer]>).map(([, payload]) => new TextDecoder().decode(payload))
    // PINNED TO THE BYTE. "A locale command was sent" passes with the two rows
    // crossed, and with a tag the panel invented.
    expect(sent[0]).toBe('{"kind":"setDocumentLocale","version":1,"locale":"th"}')
    // AND NO OFFSET COMMAND AT ALL: the offset row was not touched, so it is
    // worth no command, no round trip and no history entry.
    expect(sent.some((command) => command.includes('setDocumentUTCOffset'))).toBe(false)
    expect(sent[1]).toContain('"kind":"pageSetup"')
  })

  it('sends exactly the offset command when only the offset changed', async () => {
    const request = vi.fn(async () => ({ snapshot: snapshot(2) }))
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} />)
    fireEvent.change(screen.getByRole('textbox', { name: 'UTC offset (±HH:MM)' }), { target: { value: '+09:00' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(request).toHaveBeenCalledTimes(2))
    const sent = (request.mock.calls as unknown as ReadonlyArray<[string, ArrayBuffer]>).map(([, payload]) => new TextDecoder().decode(payload))
    expect(sent[0]).toBe('{"kind":"setDocumentUTCOffset","version":1,"utcOffset":"+09:00"}')
    expect(sent.some((command) => command.includes('setDocumentLocale'))).toBe(false)
    expect(sent[1]).toContain('"kind":"pageSetup"')
  })

  it('sends both, locale first, when both rows changed', async () => {
    const request = vi.fn(async () => ({ snapshot: snapshot(2) }))
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} />)
    fireEvent.change(screen.getByRole('combobox', { name: 'Document locale' }), { target: { value: 'ja' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'UTC offset (±HH:MM)' }), { target: { value: '+09:00' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(request).toHaveBeenCalledTimes(3))
    const sent = (request.mock.calls as unknown as ReadonlyArray<[string, ArrayBuffer]>).map(([, payload]) => new TextDecoder().decode(payload))
    // TWO COMMANDS, ONE FIELD EACH, in order — and both pinned, because a
    // single arm carrying both fields is exactly the shape Story 15.2a forbids
    // and the shape "a command was sent" cannot distinguish.
    expect(sent[0]).toBe('{"kind":"setDocumentLocale","version":1,"locale":"ja"}')
    expect(sent[1]).toBe('{"kind":"setDocumentUTCOffset","version":1,"utcOffset":"+09:00"}')
    expect(sent[2]).toContain('"kind":"pageSetup"')
  })

  it('sends no document-settings command at all when only a margin changed', async () => {
    const request = vi.fn(async () => ({ snapshot: snapshot(2) }))
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} />)
    fireEvent.change(screen.getByRole('textbox', { name: 'Top margin (pt)' }), { target: { value: '37' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    const only = new TextDecoder().decode((request.mock.calls[0] as unknown as [string, ArrayBuffer])[1])
    expect(only).toContain('"kind":"pageSetup"')
    expect(only).not.toContain('setDocument')
  })

  it('re-selecting the locale already in force sends nothing', async () => {
    // The projection says `en`; selecting `en` again is not a change. The
    // comparison is between two strings the ENGINE spelled, so an untouched row
    // is byte-equal by construction.
    const request = vi.fn(async () => ({ snapshot: snapshot(2) }))
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} />)
    fireEvent.change(screen.getByRole('combobox', { name: 'Document locale' }), { target: { value: 'en' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(new TextDecoder().decode((request.mock.calls[0] as unknown as [string, ArrayBuffer])[1])).toContain('"kind":"pageSetup"')
  })

  it('renders the engine\'s own located offset refusal, never the fixed page-setup sentence', async () => {
    // NO BROWSER-SIDE VALIDATION (Story 17.4 item 9, D-12.B). ±HH:MM is the
    // engine's rule — one predicate its loader and its command door share
    // (D-12.C) — so the panel sends `+99:99`, the engine refuses it in its own
    // words, and the existing role="alert" path prints those words. Routing it
    // through pageSetupDiagnostic instead would throw the sentence away and
    // print one about size and margins, neither of which the author touched.
    const refusal = 'utcOffset must match ±HH:MM'
    const request = vi.fn((operation: string) => operation === 'command'
      ? Promise.reject(Object.assign(new Error(refusal), { dataPath: 'utcOffset' }))
      : Promise.resolve({ snapshot: snapshot(1) }))
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} />)
    fireEvent.change(screen.getByRole('textbox', { name: 'UTC offset (±HH:MM)' }), { target: { value: '+99:99' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'Top margin (pt)' }), { target: { value: '37' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent(`utcOffset: ${refusal}`))
    expect(screen.getByRole('alert')).not.toHaveTextContent('Page setup is invalid. Check the selected size and margins.')
    // THE SEQUENCE STOPPED at the first refusal: the band heights and the
    // pageSetup command that would have followed were never sent, so the
    // document is wholly unchanged.
    expect(request).toHaveBeenCalledOnce()
    // AND THE REFUSED VALUE STAYS IN THE BOX exactly as typed. A panel that
    // restored the projected offset would show the author a value they did not
    // enter beside a refusal about the one they did.
    expect(screen.getByRole('textbox', { name: 'UTC offset (±HH:MM)' })).toHaveValue('+99:99')
    expect(screen.getByRole('textbox', { name: 'Top margin (pt)' })).toHaveValue('37')
  })

  it('abandons the rest of an Apply when the document is replaced while a locale command is in flight', async () => {
    // THE GENERATION GUARD, ON THE NEW ARM. `abandons the rest of an Apply when
    // the document is replaced mid-sequence` above does this for the BAND-HEIGHT
    // loop — but it changes only a header height and a margin, so the
    // document-settings loop `continue`s past both rows without ever awaiting,
    // and the new arm's own generation re-check is never executed by it.
    //
    // Here the hang is on setDocumentLocale, so the replacement lands while the
    // FIRST await of the sequence is outstanding. Deleting the new arm's
    // `if (documentGeneration.current !== requestDocument) return` reddens this.
    let releaseLocale: ((value: unknown) => void) | undefined
    const request = vi.fn((operation: string, payload?: ArrayBuffer) => {
      if (operation === 'undo') return Promise.resolve({ snapshot: { ...snapshot(9), canUndo: false } })
      if (operation !== 'command') return Promise.resolve({ snapshot: snapshot(1) })
      if (new TextDecoder().decode(payload as ArrayBuffer).includes('setDocumentLocale')) return new Promise((resolve) => { releaseLocale = resolve })
      return Promise.resolve({ snapshot: snapshot(3) })
    })
    render(<App engine={engine(request as never)} initialSnapshot={{ ...snapshot(1), canUndo: true }} />)
    fireEvent.change(screen.getByRole('combobox', { name: 'Document locale' }), { target: { value: 'th' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'UTC offset (±HH:MM)' }), { target: { value: '+09:00' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'Page header height (pt)' }), { target: { value: '80' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'Top margin (pt)' }), { target: { value: '37' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(releaseLocale).toBeDefined())
    // The document is replaced while the locale command is still in flight.
    fireEvent.click(screen.getByRole('button', { name: 'Undo' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Undo' })).toBeDisabled())
    await act(async () => { releaseLocale?.({ snapshot: snapshot(2) }) })
    const commands = request.mock.calls.filter(([operation]) => operation === 'command').map(([, payload]) => new TextDecoder().decode(payload as ArrayBuffer))
    // NOTHING AFTER THE LOCALE WAS SENT — not the offset, not the band height,
    // not the pageSetup. Without the guard all four would have landed on a
    // document the author has already left, carrying values read from one that
    // is gone.
    expect(commands).toHaveLength(1)
    expect(commands[0]).toContain('setDocumentLocale')
  })

  it('keeps the typed margin standing when a locale is accepted and the page setup that follows is refused', async () => {
    // keepNewerDraft ON THE NEW ARM. The band loop's
    // `setCurrentSnapshot(result.snapshot, true)` is pinned by `keeps the typed
    // margin standing when a band height is accepted…` above; the
    // document-settings loop passes the same `true` and nothing pinned it.
    //
    // The locale command is ACCEPTED, so a snapshot comes back and is installed
    // MID-GESTURE; the pageSetup that follows is then REFUSED. If that install
    // reseeded the drafts, the margin the author typed — and the tag they
    // picked — would be wiped out of their controls by a command that never
    // carried the margin at all. Flipping that `true` to `false` reddens this.
    const request = vi.fn((operation: string, payload?: ArrayBuffer) => {
      if (operation !== 'command') return Promise.resolve({ snapshot: snapshot(1) })
      return new TextDecoder().decode(payload as ArrayBuffer).includes('setDocumentLocale')
        ? Promise.resolve({ snapshot: snapshot(2) })
        : Promise.reject(new Error('the engine said something about page setup'))
    })
    render(<App engine={engine(request as never)} initialSnapshot={snapshot(1)} />)
    fireEvent.change(screen.getByRole('combobox', { name: 'Document locale' }), { target: { value: 'th' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'Top margin (pt)' }), { target: { value: '37' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    // The PAGE-SETUP sentence, because the second half of the gesture is a
    // pageSetup command and pageSetupDiagnostic has no engine message to show
    // for an unlocated rejection.
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Page setup is invalid. Check the selected size and margins.'))
    expect(request).toHaveBeenCalledTimes(2)
    expect(screen.getByRole('textbox', { name: 'Top margin (pt)' })).toHaveValue('37')
    expect(screen.getByRole('combobox', { name: 'Document locale' })).toHaveValue('th')
  })

  it('gives every numeric page-setup row the numeric keypad and the offset row none', () => {
    // Field's `inputMode` DEFAULT, asserted. Story 12.2 turned a hardcoded
    // `inputMode="decimal"` into a prop defaulting to 'decimal' so the offset
    // row — a `±HH:MM` string — could opt out. That comment makes a claim about
    // six other rows, and nothing checked it: flipping the default to 'text'
    // changes all six silently, and flipping the offset row to 'decimal' hands
    // an author a keypad with no `+`, `-` or `:` on it.
    render(<App engine={engine()} initialSnapshot={{ ...snapshot(1), canvas: { ...canvas, preset: 'custom' as const } }} />)
    const numeric = ['Width (pt)', 'Height (pt)', 'Top margin (pt)', 'Right margin (pt)', 'Bottom margin (pt)', 'Left margin (pt)', 'Page header height (pt)', 'Page footer height (pt)']
    // Non-vacuity: every row named above must actually be on screen, or the
    // loop asserts nothing about the ones that are missing.
    for (const label of numeric) expect(screen.getByRole('textbox', { name: label })).toHaveAttribute('inputmode', 'decimal')
    expect(screen.getByRole('textbox', { name: 'UTC offset (±HH:MM)' })).not.toHaveAttribute('inputmode', 'decimal')
  })

  it('renders the engine\'s own located locale refusal and stops before the offset command', async () => {
    const refusal = 'locale must be one of en, th, zh-Hans, ja (AD-12)'
    const request = vi.fn((operation: string) => operation === 'command'
      ? Promise.reject(Object.assign(new Error(refusal), { dataPath: 'locale' }))
      : Promise.resolve({ snapshot: snapshot(1) }))
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} />)
    fireEvent.change(screen.getByRole('combobox', { name: 'Document locale' }), { target: { value: 'th' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'UTC offset (±HH:MM)' }), { target: { value: '+09:00' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent(`locale: ${refusal}`))
    expect(screen.getByRole('alert')).not.toHaveTextContent('Page setup is invalid. Check the selected size and margins.')
    // THE OFFSET COMMAND WAS NEVER SENT. A sequence that carried on would apply
    // half a gesture to a document whose first half was refused.
    expect(request).toHaveBeenCalledOnce()
    expect(screen.getByRole('combobox', { name: 'Document locale' })).toHaveValue('th')
    expect(screen.getByRole('textbox', { name: 'UTC offset (±HH:MM)' })).toHaveValue('+09:00')
  })

  it('keeps component drafts local, sends exactly one Enter/Blur commit, and locates a Go diagnostic', async () => {
    const componentCanvas = { ...canvas, components: [{ id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello', fontFamily: 'body', fontSize: 12_000, borderEdges: ['bottom' as const] }] }
    const request = vi.fn((operation: string) => operation === 'command' ? Promise.reject(Object.assign(new Error('must fit the content band'), { elementId: 'e1', dataPath: 'component.x' })) : Promise.resolve({ snapshot: snapshot(1) }))
    render(<App engine={engine(request)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    const x = screen.getByRole('textbox', { name: 'X (pt)' })
    fireEvent.change(x, { target: { value: '9999' } })
    expect(request).not.toHaveBeenCalled()
    fireEvent.keyDown(x, { key: 'Enter' })
    fireEvent.blur(x)
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('e1: component.x: must fit the content band'))
    expect(x).toHaveValue('9999')
    expect(x).toHaveAttribute('aria-invalid', 'true')
  })

  it('routes the one CONTENT field to the value or expression command by what was typed', async () => {
    const componentCanvas = { ...canvas, components: [{ id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello' }] }
    const sent: ArrayBuffer[] = []
    const request = vi.fn(async (_operation: string, payload?: ArrayBuffer) => { if (payload) sent.push(payload); return { snapshot: { documentState: 'loaded' as const, revision: 1 + sent.length, byteLength: 3, canvas: componentCanvas } } })
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    expect(screen.queryByRole('textbox', { name: 'Text expression' })).not.toBeInTheDocument()
    const field = screen.getByRole('textbox', { name: 'Text' })
    // Story 7.4: CONTENT is a textarea, so Enter puts a line feed in the
    // draft and commits NOTHING. Blur is this field's commit; every other
    // field keeps Enter.
    fireEvent.change(field, { target: { value: 'Customer: {{customer.name}}' } })
    fireEvent.keyDown(field, { key: 'Enter' })
    expect(sent).toHaveLength(0)
    fireEvent.blur(field)
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(new TextDecoder().decode(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"expression":{"op":"set","value":"Customer: {{customer.name}}"}}}')
    const again = screen.getByRole('textbox', { name: 'Text' })
    fireEvent.change(again, { target: { value: 'Plain heading' } })
    fireEvent.blur(again)
    await waitFor(() => expect(sent).toHaveLength(2))
    expect(new TextDecoder().decode(sent[1]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"value":{"op":"set","value":"Plain heading"}}}')
  })

  it('marks the expression-bearing fields with fx, and lights it once the field holds one', () => {
    const componentCanvas = { ...canvas, components: [{ id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello' }] }
    render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    const text = screen.getByRole('textbox', { name: 'Text' })
    const marker = (input: HTMLElement) => input.parentElement!.querySelector('.property-fx')
    expect(text).toHaveAttribute('aria-description', 'Accepts literal text, or {{ }} expressions')
    expect(marker(text)).toHaveTextContent('fx')
    expect(marker(text)).not.toHaveClass('property-fx-active')
    fireEvent.change(text, { target: { value: 'Customer: {{customer.name}}' } })
    expect(marker(text)).toHaveClass('property-fx-active')
    // A geometry field is a literal to Go, which rejects a placeholder in it:
    // it must carry no cue at all.
    expect(marker(screen.getByRole('textbox', { name: 'X (pt)' }))).toBeNull()
    const visible = screen.getByRole('textbox', { name: 'Visible if' })
    expect(visible).toHaveAttribute('aria-description', 'Accepts a boolean or null formula, e.g. loanAmount > 20000, written without {{ }}')
    expect(marker(visible)).not.toHaveClass('property-fx-active')
    fireEvent.change(visible, { target: { value: 'loanAmount > 20000' } })
    expect(marker(visible)).toHaveClass('property-fx-active')
  })

  it('authors BOX colours through the picker, states pt on an empty size, and drops the padding rows', async () => {
    const componentCanvas = { ...canvas, components: [{ id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello' }] }
    const sent: ArrayBuffer[] = []
    const request = vi.fn(async (_operation: string, payload?: ArrayBuffer) => { if (payload) sent.push(payload); return { snapshot: { documentState: 'loaded' as const, revision: 1 + sent.length, byteLength: 3, canvas: componentCanvas } } })
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    // An unset size still says which unit it wants.
    const border = screen.getByRole('textbox', { name: 'Border width (pt)' })
    expect(border).toHaveValue('')
    expect(border.parentElement).toHaveTextContent('pt')
    for (const label of ['Padding top (pt)', 'Padding right (pt)', 'Padding bottom (pt)', 'Padding left (pt)']) expect(screen.queryByRole('textbox', { name: label })).not.toBeInTheDocument()
    const picker = screen.getByLabelText('Pick Border colour')
    fireEvent.change(picker, { target: { value: '#c81e1e' } })
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(new TextDecoder().decode(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"borderColor":{"op":"set","value":"#c81e1e"}}}')
    fireEvent.change(screen.getByLabelText('Pick Background'), { target: { value: '#0b1120' } })
    await waitFor(() => expect(sent).toHaveLength(2))
    expect(new TextDecoder().decode(sent[1]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"background":{"op":"set","value":"#0b1120"}}}')
  })

  it('paints the engine-projected box on the canvas, and nothing where none is projected', () => {
    const boxed = { id: 'e1', type: 'rect' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, background: '#1b2a4a', borderWidth: 2_000, borderColor: '#c81e1e', borderEdges: ['bottom' as const] }
    const plain = { id: 'e2', type: 'text' as const, band: 'content' as const, x: 0, y: 40_000, width: 72_000, height: 24_000, resizable: true, value: 'Plain' }
    render(<App initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: [boxed, plain] } }} />)
    const box = screen.getByLabelText('rect component e1').querySelector('.canvas-box') as HTMLElement
    expect(box).not.toBeNull()
    expect(box.style.background).toBe('rgb(27, 42, 74)')
    expect(box.style.borderBottom).toBe('2px solid rgb(200, 30, 30)')
    // An edge the engine does not paint is not painted here either.
    expect(box.style.borderTop).toBe('0px')
    expect(screen.getByLabelText('text component e2').querySelector('.canvas-box')).toBeNull()
  })

  it('sets the text colour from TYPOGRAPHY and paints the canvas in it', async () => {
    const inked = { id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello', color: '#c81e1e', textPaint: { overflow: false, truncated: false, lines: [{ top: 0, baseline: 10_000, advance: 12_000, width: 30_000, fragments: [{ text: 'Hello', x: 0 }] }] } }
    const sent: ArrayBuffer[] = []
    const componentCanvas = { ...canvas, components: [inked] }
    const request = vi.fn(async (_operation: string, payload?: ArrayBuffer) => { if (payload) sent.push(payload); return { snapshot: { documentState: 'loaded' as const, revision: 1 + sent.length, byteLength: 3, canvas: componentCanvas } } })
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    // The canvas paints the engine's ink, never a browser-side default.
    const element = screen.getByLabelText('text component e1: Hello')
    const paint = element.querySelector('.canvas-text-paint') as HTMLElement
    expect(paint.style.getPropertyValue('--text-ink')).toBe('#c81e1e')
    fireEvent.click(element)
    fireEvent.change(screen.getByLabelText('Pick Text colour'), { target: { value: '#1b2a4a' } })
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(new TextDecoder().decode(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"color":{"op":"set","value":"#1b2a4a"}}}')
  })

  it('keeps a newer property draft through an unrelated successful snapshot and exposes table truth', async () => {
    const componentCanvas = { ...canvas, components: [{ id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello' }, { id: 'e2', type: 'table' as const, band: 'content' as const, x: 0, y: 30_000, width: 72_000, height: 12_000, resizable: false, tableBind: 'transactions[]' }] }
    let resolve: ((value: { snapshot: { documentState: 'loaded'; revision: number; byteLength: number; canvas: typeof componentCanvas } }) => void) | undefined
    const request = vi.fn((operation: string) => operation === 'command' ? new Promise<{ snapshot: { documentState: 'loaded'; revision: number; byteLength: number; canvas: typeof componentCanvas } }>((done) => { resolve = done }) : Promise.resolve({ snapshot: snapshot(1) }))
    render(<App engine={engine(request)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    const value = screen.getByRole('textbox', { name: 'Text' })
    fireEvent.change(value, { target: { value: 'newer literal' } })
    fireEvent.blur(value)
    resolve!({ snapshot: { documentState: 'loaded', revision: 2, byteLength: 3, canvas: componentCanvas } })
    await waitFor(() => expect(value).toHaveValue('newer literal'))
    fireEvent.click(screen.getByLabelText('table component e2'))
    expect(screen.queryByRole('textbox', { name: 'Width (pt)' })).not.toBeInTheDocument()
    // STORY 14.4 / AC3. UPDATED, NOT DELETED — this is the only coverage of
    // that surface in this file. The value is still stated and still comes
    // straight from the projection; what changed is that the statement now
    // names WHERE it is edited, and that it is the ONLY one (the `(display
    // only)` line and the ungated BINDING section both spoke for a table too).
    const note = screen.getByText(/^Table binding: transactions\[\]/)
    expect(note).toHaveTextContent('edited in the table editor, under Configure columns')
    expect(screen.getByRole('button', { name: 'Configure columns' })).toBeInTheDocument()
  })
})

// STORY 16.3 — THE FONT BROWSER AT THE APP SEAM.
//
// WHAT THESE COVER THAT `FontBrowser.test.tsx` CANNOT. That file renders the
// modal directly and supplies its own `sources`, `inTemplate` and
// `previewBytes`, so every wire between App and the modal is stubbed out. A
// review demonstrated the gap by DROPPING the second argument from
// `offeredFamilies(query, stored)` in `browsableFamilies`: it type-checks,
// because the parameter defaults to `[]`, and every face this machine already
// holds silently vanishes from the browser — with the whole suite green. These
// tests render the real `App` and address the modal through the door.
describe('the font browser opens from the family control', () => {
  // THE MODAL FETCHES THE MOMENT IT OPENS, and those fetches must not outlive
  // the test that started them. Every row on the first page asks
  // `browserSpecimenBytes` for a face, and for the web tier that is
  // `fetchWebFamily` — up to four probes each. Left to the real global `fetch`
  // they were still in flight when the test ended, and the next test's own stub
  // then counted them as its own: a measured 10 calls where one was expected,
  // in a test about a completely different control.
  //
  // A REJECTING FETCH IS THE FIX RATHER THAN A MUTE ONE, because
  // `fetchWebFamily` STOPS at the first probe that throws. There is no second
  // probe to leak, and every row settles on "cannot be shown set in itself" —
  // which is the correct rendering for a machine with no route upstream and the
  // state these tests read anyway.
  let restoreFetch: typeof globalThis.fetch
  beforeEach(() => {
    restoreFetch = globalThis.fetch
    globalThis.fetch = vi.fn(async () => { throw new TypeError('Failed to fetch') }) as never
  })
  afterEach(async () => {
    // Drain whatever the rejections queued before handing the global back.
    await new Promise((resolve) => setTimeout(resolve, 0))
    globalThis.fetch = restoreFetch
  })

  const textComponent = { id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello' }
  // `Arimo` is a COMMITTED local-tier face, so it is in the offered population
  // with no network at all, and declaring a chain of that name puts the same
  // family on both sides of the `In template` question.
  const withArimoDeclared = { ...canvas, components: [textComponent], fontFamilies: ['body', 'Arimo'], fontChains: [{ name: 'body', entries: [face('Noto Sans')] }, { name: 'Arimo', entries: [face('Noto Sans')] }] }

  const openDoor = (projection = withArimoDeclared) => {
    const request = vi.fn(async () => ({ snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3 } }))
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: projection }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
    fireEvent.click(screen.getByRole('button', { name: /^Add fonts…/ }))
    return request
  }

  it('mounts the dialog from the last row of the open dropdown', () => {
    const restore = installStubFontSet()
    try {
      openDoor()
      const dialog = screen.getByRole('dialog', { name: 'Font browser' })
      expect(dialog).toHaveAttribute('aria-modal', 'true')
      // The door closed the dropdown on its way out, so the listbox is gone and
      // the modal is what has focus.
      expect(screen.queryByRole('listbox', { name: 'Fonts' })).toBeNull()
    } finally {
      restore.restore()
    }
  })

  it('names the sub-label in the door\'s accessible name, not only on screen', () => {
    const restore = installStubFontSet()
    try {
      const request = vi.fn(async () => ({ snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3 } }))
      render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: withArimoDeclared }} />)
      fireEvent.click(screen.getByLabelText('text component e1'))
      fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
      // An `aria-label` REPLACES the contents, so a bare "Add fonts…" deleted
      // the one sentence saying what the row does for everybody who cannot see
      // it.
      const door = screen.getByRole('button', { name: 'Add fonts… Browse and embed web fonts' })
      expect(door).toBeInTheDocument()
      // AND NO SHORTCUT GLYPH, WHICH IS RULED RATHER THAN FORGOTTEN (D-16.R.33
      // R2). The mockup prints `⌘G` beside this row and no key is bound, so a
      // glyph here would be a false UI string. Story 16.4 restates the matrix
      // row; the assertion is added because nothing held it.
      expect(door.textContent, 'a hint glyph beside an unbound key is a false label').not.toMatch(/⌘|⌥/)
      expect(screen.getByRole('listbox', { name: 'Fonts' }).contains(door), 'the door is a real button OUTSIDE the listbox, never a non-option child of it').toBe(false)
    } finally {
      restore.restore()
    }
  })

  it('carries the DOCUMENT\'s declared chains into the modal as `In template`', async () => {
    const restore = installStubFontSet()
    // A STORE IS SUPPLIED SO THE DIALOG IS IN ITS ORDINARY MODE. jsdom provides
    // no IndexedDB, and a browser that cannot keep typefaces puts the confirm
    // control into Story 16.5's degraded model, where it names the count instead
    // of the action — which is asserted in `FontBrowser.test.tsx`, and is not
    // what this test is about.
    const restoreStore = withMachineStore()
    try {
      openDoor()
      fireEvent.change(screen.getByRole('textbox', { name: 'Search fonts' }), { target: { value: 'Arimo' } })
      // `canvas.fontFamilies` reached the modal: the family the document
      // already declares cannot be staged again.
      const inTemplate = await screen.findByRole('button', { name: 'Arimo is in this template' })
      expect(inTemplate).toBeDisabled()
      expect(screen.getByRole('button', { name: 'Install on this machine' })).toBeDisabled()
    } finally {
      restore.restore()
      restoreStore()
    }
  })

  it('returns focus to the family control when the modal closes', async () => {
    const restore = installStubFontSet()
    try {
      openDoor()
      fireEvent.keyDown(screen.getByRole('dialog', { name: 'Font browser' }), { key: 'Escape' })
      await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Font browser' })).toBeNull())
      // The door unmounted with the dropdown, so without this focus would be on
      // `<body>` and a keyboard-only author would tab in from the top of the
      // page after every Escape (UX-DR25).
      await waitFor(() => expect(screen.getByRole('combobox', { name: 'Font family' })).toHaveFocus())
    } finally {
      restore.restore()
    }
  })
})

describe('typography controls over the engine-projected closed sets', () => {
  const textComponent = { id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello' }
  const select = (request = vi.fn(async () => ({ snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3 } }))) => {
    const componentCanvas = { ...canvas, components: [textComponent] }
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    return request
  }

  it('offers the document\'s declared font chains, searched, and commits the chosen one', async () => {
    const request = select()
    const combobox = screen.getByRole('combobox', { name: 'Font family' })
    fireEvent.focus(combobox)
    // SCOPED TO THE DECLARED GROUP. Since Story 8.6 the listbox carries a
    // second group — the bundled catalogue — and this test is about the first:
    // a catalogue entry is not a chain the document declares, and folding the
    // two together would make "the document's declared chains" mean "every
    // family that exists".
    const listed = () => declaredOptions().map(optionText)
    expect(listed()).toEqual(['body', 'heading'])
    fireEvent.change(combobox, { target: { value: 'head' } })
    expect(listed()).toEqual(['heading'])
    fireEvent.click(screen.getByRole('option', { name: 'heading' }))
    await waitFor(() => expect(request).toHaveBeenCalledWith('command', expect.anything()))
    // The typed search text is never a value: only a listed family is sent.
    expect(screen.queryByRole('listbox', { name: 'Fonts' })).not.toBeInTheDocument()
  })

  it('states a search that matches neither a declared chain nor a family on this machine, instead of offering to invent one', () => {
    select()
    const combobox = screen.getByRole('combobox', { name: 'Font family' })
    fireEvent.focus(combobox)
    fireEvent.change(combobox, { target: { value: 'Helvetica' } })
    expect(screen.queryByRole('option')).not.toBeInTheDocument()
    // CORRECTED WITH THE CHANGE, NOT EDITED QUIETLY (Story 16.4, narrowed by
    // 16.9). This pinned `Nothing in this document or the catalogue matches …`
    // — a sentence naming ONE of the three places the control searched at
    // 16.4, written when the catalogue was the whole offer. Story 16.9 then
    // dropped the control's own third place, "or in the list you can install"
    // — this control no longer searches there at all, `Add fonts…` does — so
    // the sentence names only the two groups drawn above it now.
    expect(screen.getByText('Nothing in this template or on this machine matches "Helvetica".')).toBeInTheDocument()
  })

  // STORY 16.4 — THREE GROUPS, ON THE AXIS THE CODE ALREADY FORKS ON.
  //
  // The grouping key is (declared?, `familyIsInstalled`?) and nothing else, so
  // these assertions are written against what each group's rows DO — the note a
  // row carries says whether picking it uses the face or downloads it — rather
  // than against a class name, which would pass on a control that grouped
  // alphabetically.
  const dropdown = () => screen.getByRole('listbox', { name: 'Fonts' })
  const groupRows = (label: string) => within(screen.getByRole('group', { name: label })).getAllByRole('option').map(optionText)

  // STORY 16.9 NARROWED THIS FROM THREE GROUPS TO TWO: `AVAILABLE TO INSTALL`
  // (the `web` arm, ~1,273 families not on this machine) is gone from this
  // dropdown. `Add fonts…` is now the only door to that relationship; this
  // control offers only what is usable now, with no network and no wait.
  it('draws the two groups, disjoint and complete, on the where-are-the-bytes axis', () => {
    select()
    fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
    // THE ORDER IS THE MODEL: in the file, on the machine.
    expect(within(dropdown()).getAllByRole('group').map((group) => group.getAttribute('aria-label'))).toEqual(['IN THIS TEMPLATE', 'AVAILABLE LOCALLY'])
    const template = groupRows('IN THIS TEMPLATE')
    const local = groupRows('AVAILABLE LOCALLY')
    // COMPLETE: every option the listbox owns sits in exactly one group.
    expect(template.length + local.length).toBe(within(dropdown()).getAllByRole('option').length)
    // DISJOINT: no family is offered twice under two different promises.
    const names = [...template, ...local].map((text) => text.split(' — ')[0])
    expect(new Set(names).size).toBe(names.length)
    // AND EACH HEADING TELLS THE TRUTH ABOUT ITS OWN ROWS.
    expect(template).toEqual(['body', 'heading'])
    // STORY 16.7 (Design Note 2, narrowing D-16.R.72): the per-row note that
    // used to restate "needs no download" is now the specimen instead, so the
    // truthful claim left to make about AVAILABLE LOCALLY's OWN ROWS is that
    // none of them still carries any note at all — every row this dropdown
    // draws is one this machine already holds, so no row needs one any more.
    expect(local.every((text) => !text.includes(' — ')), `no AVAILABLE LOCALLY row may still carry a per-row note: ${local.slice(0, 3).join(' / ')}`).toBe(true)
    // THE SECOND GROUP IS POPULATED ON A FRESH MACHINE, which is the half of
    // D-16.R.72 a store-shaped reading of the heading would have got wrong: the
    // committed faces ship inside the release, so they are always on it.
    expect(local, 'the committed faces are on this machine whether or not anything was ever downloaded').toHaveLength(catalogueFaces.length)
  })

  // STORY 16.9 — THERE IS NO THIRD GROUP LEFT TO CAP. Both remaining groups
  // render in full, unconditionally: neither is bounded any more, because
  // the population that once needed a bound (~1,273 web-tier rows) is no
  // longer offered here at all.
  it('renders both remaining groups in full, with no cap and no "Showing N of M" note', () => {
    select()
    fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
    expect(groupRows('IN THIS TEMPLATE')).toHaveLength(2)
    expect(groupRows('AVAILABLE LOCALLY')).toHaveLength(catalogueFaces.length)
    expect(screen.queryByRole('group', { name: 'AVAILABLE TO INSTALL' })).not.toBeInTheDocument()
    expect(screen.queryByText(/Showing \d+ of \d+/)).not.toBeInTheDocument()
  })

  // STORY 16.9 ALSO REMOVED TWO OTHER CONTROLS FROM THIS FIELD — THE CLEAR
  // BUTTON, AND THE DISCLOSURE THAT REVEALED THE FONT-CHAIN EDITOR — AND
  // UNTIL NOW NEITHER HAD A GUARD OF ITS OWN, ONLY A RETIREMENT COMMENT. A
  // comment cannot fail a build; this can.
  //
  // POPULATION STATED BESIDE THE ZERO, PER STANDING RULE: with the combobox
  // focused, this property renders exactly one button (`buttonNames` below) —
  // the disclosure fixed by the chevron correction above. If either removed
  // control ever came back it would be a SECOND or THIRD entry in that same
  // array, not a query silently matching nothing; the assertions below are
  // therefore not the only thing that would catch a regression, but they name
  // the two controls this story specifically removed rather than leaving that
  // to an incidental count.
  it('offers no control to clear the family, and none to reveal a font-chain editor', () => {
    select()
    const combobox = screen.getByRole('combobox', { name: 'Font family' })
    fireEvent.focus(combobox)
    // SCOPED TO THIS FIELD, NOT THE WHOLE PAGE: the property panel around it
    // carries its own buttons (Bold, alignment, Add fonts…) that have nothing
    // to do with what this test is about. `.property-combobox` is the field's
    // own wrapper, rendered by `FontFamilyProperty` itself.
    const field = combobox.closest('.property-combobox')
    if (!field) throw new Error('the font family combobox is no longer inside .property-combobox; re-scope this test rather than deleting it')
    const buttonNames = within(field as HTMLElement).getAllByRole('button').map((button) => button.getAttribute('aria-label') ?? button.textContent)
    // `.property-combobox` wraps the field AND its open overlay (the shell
    // holding the listbox and, below it, `Add fonts…`), so both legitimate
    // buttons appear here — the population is two, not one, and that is
    // stated rather than assumed.
    expect(buttonNames, 'the population this zero is measured against — every button this field renders with the dropdown open').toEqual(['Hide fonts', 'Add fonts… Browse and embed web fonts'])
    expect(screen.queryByRole('button', { name: 'Clear Font family' }), 'text always has a typeface; there is no control to clear it').not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Edit font chains' }), '`FontChainEditor.tsx` and its render site are deleted').not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Hide font chains' }), 'the same disclosure, in its other label').not.toBeInTheDocument()
  })

  // STORY 16.9 DISCHARGES A REGISTERED HAZARD: a pick from the removed
  // `AVAILABLE TO INSTALL` group used to block up to 30s on a stall and 180s
  // against a slow host, because the dropdown could trigger a fetch that only
  // a web-tier row could reach (`fetchWebFamily`, against the one declared
  // repository host). With no web row left in this control, no code path
  // from opening or filtering the dropdown can reach that fetch at all —
  // never mind stall or succeed.
  //
  // "NO REQUEST" MEANS NO REQUEST TO A THIRD PARTY, NOT LITERALLY ZERO CALLS
  // TO `fetch`. `AVAILABLE LOCALLY`'s own specimens (Story 16.7) read their
  // bytes from THIS RELEASE'S OWN bundle, over a relative URL, the same read
  // `runtimeAssetUrls` assets get behind the service worker — offline-capable,
  // no third party, and unrelated to the hazard this story closes. So this
  // installs the same page-font stub the specimen tests use (`installStubFontSet`,
  // above) rather than leaving `FontFace` undefined, which would make every
  // specimen fetch silently never fire and prove nothing about THIS story's
  // claim. The assertion is that among however many same-origin reads happen,
  // NONE is an absolute URL — which is exactly what would change if a web-tier
  // row, and the fetch it can reach, ever came back.
  //
  // THE OBSERVER IS PROVEN ALIVE, NOT ASSUMED (the "vacuous survivor" this
  // suite has been burned by before): the same spy that records only
  // same-origin calls from opening and filtering is proven capable of
  // recording a call at all, from the very rows this dropdown draws.
  it('reaches no third-party host opening and filtering the dropdown, though the same spy can see the local reads it does make', async () => {
    const bundleAsset = new Uint8Array([0x00, 0x01, 0x00, 0x00, 0x7f]).buffer
    const fetchStub = vi.fn(async (_url: string) => ({ ok: true, arrayBuffer: async () => bundleAsset }))
    const restore = globalThis.fetch
    globalThis.fetch = fetchStub as never
    const fontSet = installStubFontSet()
    try {
      select()
      const combobox = screen.getByRole('combobox', { name: 'Font family' })
      fireEvent.focus(combobox)
      // SETTLES ON A REAL SPECIMEN, so the assertions below run after
      // whatever this dropdown is going to fetch has actually been asked
      // for — never on a synchronous race that would pass by never giving
      // the effect a chance to run.
      await waitFor(() => expect(fetchStub).toHaveBeenCalled())
      fireEvent.change(combobox, { target: { value: 'lora' } })
      fireEvent.change(combobox, { target: { value: 'Kanit' } })
      fireEvent.change(combobox, { target: { value: '' } })
      await waitFor(() => expect(groupRows('AVAILABLE LOCALLY').length).toBeGreaterThan(0))
      // THE CORE CLAIM: not one of however many calls happened names an
      // external host. `fetchWebFamily` is the one function in this codebase
      // that ever builds an absolute URL to the declared repository host, and
      // nothing reachable from this dropdown calls it any more.
      const urls = fetchStub.mock.calls.map((call) => String(call[0]))
      expect(urls.length, 'the observer must have seen something, or the claim below is vacuous').toBeGreaterThan(0)
      expect(urls.every((url) => !/^https?:\/\//.test(url)), `every request must be same-origin, never a third-party host: ${urls.filter((url) => /^https?:\/\//.test(url)).slice(0, 3).join(', ')}`).toBe(true)
    } finally {
      globalThis.fetch = restore
      fontSet.restore()
    }
  })

  // A HEADING IS SUPPRESSED ONLY WHEN ITS OWN GROUP IS EMPTY AFTER FILTERING —
  // never because another group emptied, and never while it still owns a row.
  // STORY 16.9 removed the third group; 'sara' (Sarabun/Sarala) matches
  // neither remaining group — it exists only in the web-tier snapshot this
  // control no longer offers — so filtering it now empties the dropdown
  // entirely rather than isolating a third heading.
  it('suppresses each heading only on its own empty group', () => {
    select()
    const combobox = screen.getByRole('combobox', { name: 'Font family' })
    fireEvent.focus(combobox)
    // A declared chain name that matches no typeface at all: group 1 alone.
    fireEvent.change(combobox, { target: { value: 'heading' } })
    expect(within(dropdown()).getAllByRole('group').map((group) => group.getAttribute('aria-label'))).toEqual(['IN THIS TEMPLATE'])
    // A family on this machine, matching no declared chain: group 2 alone.
    fireEvent.change(combobox, { target: { value: 'lora' } })
    expect(within(dropdown()).getAllByRole('group').map((group) => group.getAttribute('aria-label'))).toEqual(['AVAILABLE LOCALLY'])
    // A query matching neither remaining group empties the dropdown, rather
    // than falling through to a group this control no longer draws.
    fireEvent.change(combobox, { target: { value: 'sara' } })
    expect(within(dropdown()).queryAllByRole('group')).toEqual([])
    expect(screen.getByText('Nothing in this template or on this machine matches "sara".')).toBeInTheDocument()
  })

  // THE KEYBOARD IS LINEAR EVEN THOUGH THE LIST IS NOT. Two groups is two
  // headings interleaved into ONE option sequence, and 8.6's reason for the
  // flat list survives unchanged — which is exactly why the heading
  // interleave had to go: it was the one element of the walk that read a
  // position semantically.
  it('walks both groups in one arrow-key sequence, and wraps', () => {
    select()
    const combobox = screen.getByRole('combobox', { name: 'Font family' })
    fireEvent.focus(combobox)
    const owner = () => document.getElementById(combobox.getAttribute('aria-activedescendant') ?? '')?.closest('[role="group"]')?.getAttribute('aria-label') ?? undefined
    const total = within(dropdown()).getAllByRole('option').length
    expect(total, 'a walk over an empty list would prove nothing').toBeGreaterThan(2)
    const visited: Array<string | undefined> = []
    for (let step = 0; step < total; step += 1) {
      visited.push(owner())
      fireEvent.keyDown(combobox, { key: 'ArrowDown' })
    }
    // ONE CONTIGUOUS RUN PER GROUP, IN THE ORDER THEY ARE DRAWN. Three runs
    // would mean the walk crossed a group boundary and came back.
    expect(visited.filter((label, index) => index === 0 || label !== visited[index - 1])).toEqual(['IN THIS TEMPLATE', 'AVAILABLE LOCALLY'])
    // AND THE SEQUENCE IS ONE SEQUENCE: the step past the last row is the first.
    expect(owner()).toBe('IN THIS TEMPLATE')
    // Backwards from the top lands on the last row of the last group, which is
    // the same walk read the other way.
    fireEvent.keyDown(combobox, { key: 'ArrowUp' })
    expect(owner()).toBe('AVAILABLE LOCALLY')
  })

  // STORY 8.6's DEFERRAL, CLOSED HERE RATHER THAN MULTIPLIED (it would have gone
  // from six non-option children to seven). NOTHING PINNED THE PRESENTATION ROLE
  // BEFORE THIS TEST — measured: zero assertions over the whole test corpus — so
  // the fix would otherwise have been unguarded, and a note dropped back into the
  // list would break the listbox again in silence.
  it('owns only groups of options, with every note outside the list and referenced from it', () => {
    select()
    fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
    const listbox = dropdown()
    expect(listbox.querySelectorAll('[role="presentation"]'), 'a listbox may not own a presentational child').toHaveLength(0)
    expect(Array.from(listbox.children).map((child) => child.getAttribute('role')), 'every child of the listbox is a group').toEqual(['group', 'group'])
    for (const group of within(listbox).getAllByRole('group')) {
      for (const child of Array.from(group.children)) {
        expect(child.getAttribute('role') === 'option' || child.getAttribute('aria-hidden') === 'true', `${group.getAttribute('aria-label')} owns a child that is neither an option nor hidden from the tree: ${child.outerHTML.slice(0, 80)}`).toBe(true)
      }
    }
    // POSITIVE CONTROL, IN THE ONE STATE THAT STILL DRAWS A NOTE. The standing
    // explanation was removed from this dropdown by OWNER decision, so the notes
    // node exists only when the search matches nothing. The rule it was written
    // for is unchanged and is asserted where it can still bite: a note is never a
    // row in the list, and the list names the note that describes it.
    expect(listbox.getAttribute('aria-describedby'), 'with rows to show there is no note to describe them').toBeNull()
    fireEvent.change(screen.getByRole('combobox', { name: 'Font family' }), { target: { value: 'Helvetica' } })
    const empty = screen.getByText(/Nothing in this template or on this machine matches/)
    const emptied = dropdown()
    expect(emptied, 'the empty-state sentence is about the list and is not a row in it').not.toContainElement(empty)
    const notes = document.getElementById(emptied.getAttribute('aria-describedby') ?? '')
    expect(notes, 'the list must name the note that describes it').not.toBeNull()
    expect(notes).toContainElement(empty)
  })

  // MATRIX ROW: TWO COMPONENTS WITH DIFFERENT FAMILIES STILL SAY `Mixed`, AND
  // THE THREE GROUPS DO NOT CHANGE THAT. The row was carried as "as today" and
  // "as today" was asserted by nothing, so a control rebuilt around a partition
  // could have lost it in silence.
  it('keeps the Mixed placeholder over a selection with two different families', () => {
    const two = [
      { id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'One', fontFamily: 'body' },
      { id: 'e2', type: 'text' as const, band: 'content' as const, x: 0, y: 30_000, width: 72_000, height: 24_000, resizable: true, value: 'Two', fontFamily: 'heading' },
    ]
    render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: two } }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    fireEvent.click(screen.getByLabelText('text component e2'), { shiftKey: true })
    const combobox = screen.getByRole('combobox', { name: 'Font family' })
    expect(combobox).toHaveAttribute('placeholder', 'Mixed')
    expect(combobox).toHaveAttribute('aria-description', 'Mixed value')
    // AND NO ROW CLAIMS TO BE THE SELECTED ONE, because none of them is.
    fireEvent.focus(combobox)
    expect(within(screen.getByRole('listbox', { name: 'Fonts' })).queryAllByRole('option', { selected: true })).toEqual([])
  })

  // STORY 8.6, AC4. THE TWO GROUPS ARE DIFFERENT KINDS OF THING, and the
  // difference is asserted by what each does when it is picked — not by a
  // class name, which would pass on a control where both options committed the
  // same property.
  it('offers the bundled catalogue as a second, visibly distinct group whose entries the document does not declare', () => {
    select()
    const combobox = screen.getByRole('combobox', { name: 'Font family' })
    fireEvent.focus(combobox)
    // A family the document declares no chain for. It is offered, it is marked
    // as an addition rather than a selection, and the headings say which group
    // is which.
    // The accessible name collapses the note's leading space, so the pattern is
    // whitespace-tolerant rather than pinned to one spelling of the gap.
    //
    // STORY 16.1: THE NOTE NOW SAYS WHICH TIER THE ROW COMES FROM. `Inter` is
    // one of the 21 committed faces — the LOCAL FACE TIER — so its row states
    // that nothing is downloaded. A family that exists only in the build-time
    // index snapshot carries the plain note, because picking it fetches.
    const inter = screen.getByRole('option', { name: /^Inter$/ })
    expect(inter).toBeInTheDocument()
    // STORY 16.4 DREW THREE HEADINGS ON THE AXIS `WHERE ARE THE BYTES`; STORY
    // 16.9 NARROWED THE DROPDOWN BACK TO TWO. The two 8.6 shipped — `In this
    // document` and `Catalogue — not yet in this document` — named a WHEN and
    // a source that was never the only one; the group a row sits in is a pure
    // function of (declared?, `familyIsInstalled`?), and each heading is the
    // accessible name of the group that owns those rows. `AVAILABLE TO
    // INSTALL` is gone from this control — `Add fonts…` is its door now.
    expect(screen.getByRole('group', { name: 'IN THIS TEMPLATE' })).toBeInTheDocument()
    expect(screen.getByRole('group', { name: 'AVAILABLE LOCALLY' })).toBeInTheDocument()
    expect(screen.queryByRole('group', { name: 'AVAILABLE TO INSTALL' })).not.toBeInTheDocument()
    // RETIRED, AND THE SUBJECT NO LONGER EXISTS: the disclosure stopped rendering
    // in this dropdown (OWNER, 2026-09-03 — the standing explanation above
    // `Add fonts…` was cut), and Story 16.10 then removed its last render site,
    // the font browser's header, because the design draws no paragraph there.
    // `familyIndexDisclosure()` IS DELETED — measured at zero consumers — so
    // there is no sentence left for any surface to assert. The COUNT it quoted
    // survives: `addableFamilyCount`'s own two facts are pinned in
    // `font-index.test.ts`, and `resultLine`, the line that still prints that
    // count in the browser's results toolbar, has its output pinned separately in
    // `font-browser-model.test.ts:285-288`. Two different subjects, two files.
    // THE DISK-FONT DECLINE IS RETIRED FROM THIS SURFACE (OWNER, 2026-09-03).
    // It was re-derived at 16.4 rather than carried, and it was the only place
    // this product answered "where do I add my own font file?" — there is no
    // import control to be found missing. The owner cut the standing explanation
    // above `Add fonts…` and this sentence went with it, so the question now has
    // no answer in the UI. Recorded here rather than left to be rediscovered.
    // The declared group never carries the addition note: picking one of those
    // sets a property, and it is already in the file.
    expect(declaredOptions().map(optionText)).toEqual(['body', 'heading'])
    // A SNAPSHOT-ONLY FAMILY IS NO LONGER REACHABLE BY TYPING HERE AT ALL
    // (Story 16.9). `Kanit` is in the published web-tier snapshot and not
    // among the 31 local/stored faces, so it is not offered by this control
    // under any query — `Add fonts…` is the only door to it now.
    fireEvent.change(combobox, { target: { value: 'Kanit' } })
    expect(screen.queryByRole('option', { name: /^Kanit/ }), 'a family not on this machine is not offered by this dropdown at all').not.toBeInTheDocument()
    expect(screen.getByText('Nothing in this template or on this machine matches "Kanit".')).toBeInTheDocument()
  })

  // STORY 8.6's AC1/AC3 AT THE BROWSER BOUNDARY, BEHAVIOUR-CHANGED BY STORY 16.5
  // INTO THE THIRD ARM'S WITNESS.
  //
  // `Inter` is a LOCAL-TIER face: it ships inside the release, so this machine
  // already holds it and there is nothing to install. Picking it is therefore
  // FIRST USE, and first use is two commands — `embedFontFamily` and then
  // `updateComponentProperties` — with two undo entries. The order is forced by
  // the engine (`canvas.fontFamilies` is the closed set `style.fontFamily` may
  // name), and asserting it ON THE WIRE, in order, is what makes this a claim
  // about behaviour rather than about a class name.
  //
  // What changed is the SECOND command. Under Story 8.6 a catalogue pick
  // deliberately did not set `fontFamily`; under embed-on-use, setting a
  // component's family to a face this machine holds IS the author asking for
  // both, so both happen — as two commands, never fused into one.
  it('embeds and then commits the property, as two commands, when a family this machine holds is picked', async () => {
    const face = new Uint8Array([0x00, 0x01, 0x00, 0x00, 0x7f]).buffer
    const fetchStub = vi.fn(async (_url: string) => ({ ok: true, arrayBuffer: async () => face }))
    const restore = globalThis.fetch
    globalThis.fetch = fetchStub as never
    try {
      // `select`'s mock is declared with no parameters, so its recorded calls
      // are an empty tuple to TypeScript. The arguments are read back through
      // a widened view rather than by changing that shared signature, which
      // every other test in this describe block is written against.
      const request = select()
      const sent = request.mock.calls as unknown as ReadonlyArray<readonly [string, ArrayBuffer]>
      fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
      fireEvent.click(screen.getByRole('option', { name: /^Inter$/ }))
      await waitFor(() => expect(request).toHaveBeenCalledWith('command', expect.anything()))
      // THE BYTES CAME FROM THE BUNDLE, not from a host. The URL is one of the
      // release's own content-addressed assets, which is what makes the pick
      // work offline — SPEC-fonts requires no call to either Google Fonts host
      // at any point, and `forbidden-font-hosts.test.ts` is what names them.
      //
      // STORY 16.1 MAKES THIS THE LOCAL-TIER WITNESS (D-16.R.3): `Inter` is one
      // of the 21 committed faces, so the pick issues EXACTLY ONE request, to a
      // relative release asset — no `METADATA.pb`, no licence file, no third
      // party. `Inter` is variable-only on the `google/fonts` mirror and static
      // here, which is precisely why the snapshot's `axes` field is not
      // consulted for a family the local tier holds.
      expect(fetchStub).toHaveBeenCalledTimes(1)
      expect(String(fetchStub.mock.calls[0][0])).not.toMatch(/^https?:/)
      // TWO COMMANDS, IN THIS ORDER. Not one fused command, and not the embed
      // alone: the property is committed only after the chain is declared,
      // because the engine refuses it otherwise.
      await waitFor(() => expect(sent).toHaveLength(2))
      const kinds = sent.map(([, buffer]) => (JSON.parse(new TextDecoder().decode(buffer)) as Record<string, unknown>)['kind'])
      expect(kinds).toEqual(['embedFontFamily', 'updateComponentProperties'])
      const property = JSON.parse(new TextDecoder().decode(sent[1][1])) as Record<string, unknown>
      expect(property['changes']).toEqual({ fontFamily: { op: 'set', value: 'Inter' } })
      expect(sent[0][0]).toBe('command')
      const payload = JSON.parse(new TextDecoder().decode(sent[0][1])) as Record<string, unknown>
      expect(payload['kind']).toBe('embedFontFamily')
      expect(payload['name']).toBe('Inter')
      expect(payload['family']).toBe('Inter')
      // The three keys the ENGINE REFUSES TO LOAD A DOCUMENT WITHOUT. A pick
      // that omitted any of them would produce a document the engine's own
      // parser rejects, so their presence is the browser's half of that
      // contract and not a detail.
      expect(payload['licence']).toBeTruthy()
      expect(payload['licenceText']).toBeTruthy()
      expect(payload['copyright']).toBeTruthy()
      expect(payload['data']).toBe('AAEAAH8=')
      // AC3: Inter covers Latin and nothing else, so the proposed tail is the
      // shipped faces for the scripts it does NOT cover, in order.
      //
      // STORY 11.4 — AND THE TAIL NOW DECLARES THE CUTS THOSE FACES HAVE. The
      // ORDER and the MEMBERSHIP are exactly what they were; what changed is
      // that `Noto Sans Thai` says it has a bold, so a document whose Thai
      // fallback came from a pick can bold its Thai. `Noto Sans SC` declares
      // nothing and stays a bare string — D-A, a permanent shipped condition,
      // not a row somebody has yet to fill in.
      expect(payload['tail']).toEqual([{ face: 'Noto Sans Thai', bold: 'Noto Sans Thai Bold' }, 'Noto Sans SC'])
      // AND THE PICKED ENTRY ITSELF DECLARES NO CUT, because it has none: one
      // embedded face, and every catalogue face is a single upright Regular.
      //
      // ⚠ THIS USED TO BE A VACUOUS ASSERTION AND IT IS WORTH SAYING WHY
      // (D-11.2.8). It read `Object.keys(payload).filter(k => VARIANTS.includes(k))`
      // — the TOP LEVEL of an `embedFontFamily` payload, which has never carried
      // a variant key in any version of this command and has nowhere to put one.
      // It could not fail whatever the implementation did, while wearing the
      // label of a claim about the PICKED ENTRY. The claim is now asserted at
      // the two places it can actually break: the picked family is not in the
      // declared mirror, so there are no cuts for the entry to declare, and the
      // tail — the ONLY place this command can carry a cut — never names the
      // picked family, so no cut can arrive attached to it by the back door.
      // The entry itself is built inside the engine from the bytes above, and
      // `TestEmbedFontFamilyWritesTheAssetAndDeclaresTheChain` reads it back.
      expect(shippedFamilyEntry('Inter'), 'Inter is not a family the release ships, so a pick of it has no cuts to declare').toBeUndefined()
      const tailFaces = (payload['tail'] as ReadonlyArray<string | { face: string }>).map((entry) => typeof entry === 'string' ? entry : entry.face)
      expect(tailFaces).not.toContain('Inter')
    } finally {
      globalThis.fetch = restore
    }
  })

  // STORY 11.4 — THE PAYOFF, AND UNTIL THE REVIEW NOTHING ASSERTED IT.
  //
  // Every other test in this story checks what a pick WRITES. This one checks
  // what the author SEES afterwards, which is the only reason any of it was
  // funded: the element's B control states Story 11.3's absence sentence
  // before the pick and is the plain control after it. The whole chain of
  // mechanism — the mirror declares Roboto's cuts, the command carries them,
  // the engine projects them back, `selectionMissingCut` reads the projection —
  // is exercised end to end by one gesture, and any link of it breaking turns
  // this red.
  //
  // ⚠ THE PROJECTION IS REBUILT FROM THE PICK'S OWN COMMAND BYTES, never from a
  // fixture written beside it. A hand-written "after" chain would go on
  // declaring cuts for a pick that had stopped writing them. The engine's half
  // of the same seam — that it copies a chain entry's declared cuts into the
  // projection verbatim — is tied in Go by `canvas_projection_wire_test.go`.
  it('stops stating the absent cut once the picked family declares one', async () => {
    const NO_BOLD_HERE = 'No bold face in this family — the engine paints the regular face and warns.'
    // `body` declares ONE entry and no cut at all, so B states the absence.
    let projected: CanvasProjection = { ...canvas, components: [{ ...textComponent, fontFamily: 'body' }] }
    let revision = 1
    const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
      if (operation === 'command' && payload !== undefined) {
        const command = JSON.parse(new TextDecoder().decode(payload)) as Record<string, unknown>
        if (command['kind'] === 'addFontChain') {
          const name = command['name'] as string
          const entries = (command['entries'] as ReadonlyArray<string | Record<string, string>>).map((entry) => typeof entry === 'string' ? face(entry) : face(entry['face'], entry))
          projected = { ...projected, fontFamilies: [...projected.fontFamilies, name], fontChains: [...projected.fontChains, { name, entries }] }
        }
        if (command['kind'] === 'updateComponentProperties') {
          const value = ((command['changes'] as Record<string, Record<string, string>>)['fontFamily'] ?? {})['value']
          projected = { ...projected, components: projected.components.map((component) => ({ ...component, fontFamily: value ?? component.fontFamily })) }
        }
      }
      return { snapshot: { documentState: 'loaded' as const, revision: ++revision, byteLength: 3, canvas: projected } }
    })
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: projected }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    const NO_ITALIC_HERE = 'No italic face in this family — the engine paints the regular face and warns.'
    const control = (name: 'Bold' | 'Italic') => screen.getByRole('button', { name })
    // BEFORE — and this half is asserted so the "after" is a CHANGE and not a
    // control that never said anything (D-11.2.8).
    for (const [name, sentence] of [['Bold', NO_BOLD_HERE], ['Italic', NO_ITALIC_HERE]] as const) {
      expect(control(name).className).toContain('property-toggle-unavailable')
      expect(document.getElementById(control(name).getAttribute('aria-describedby')!)).toHaveTextContent(sentence)
    }

    fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
    fireEvent.click(screen.getByRole('option', { name: /^Roboto$/ }))
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'command')).toHaveLength(2))

    // AFTER — the family now DECLARES both cuts, so there is nothing to state.
    //
    // ⚠ THE ITALIC CONTROL IS THE SENSITIVE ONE, AND THAT IS MEASURED RATHER
    // THAN ASSUMED. `chainDeclaresCut` is an ANY-ENTRY rule (Q2, ratified at
    // 11.3's CHECKPOINT 1), and the proposed fallback tail carries `Noto Sans
    // Thai`, which declares a bold — so B would go plain even for a pick that
    // declared NOTHING for Roboto itself. Nothing in the tail declares an
    // italic, so I going plain is the picked family's own declaration and no
    // other entry's. Deleting Roboto's cuts from the mirror leaves B green and
    // turns I red, which is exactly the discrimination this test needs.
    await waitFor(() => expect(control('Italic').className).not.toContain('property-toggle-unavailable'))
    for (const name of ['Bold', 'Italic'] as const) {
      expect(control(name).className).not.toContain('property-toggle-unavailable')
      expect(control(name).getAttribute('aria-describedby')).toBeNull()
      expect(control(name)).not.toBeDisabled()
    }
    expect(screen.queryByText(NO_BOLD_HERE)).not.toBeInTheDocument()
    expect(screen.queryByText(NO_ITALIC_HERE)).not.toBeInTheDocument()
  })

  // STORY 11.4 — THE OTHER HALF OF THE SAME FORK, AND BOTH ARE ASSERTED
  // BECAUSE A TEST THAT ONLY CHECKED THIS ONE COULD NOT SEE THE CATALOGUE CASE
  // REGRESS.
  //
  // `Roboto` is a family the RELEASE ALREADY SHIPS — it is a `fonts.Shipped()`
  // key, and `folio8-designer/public/fonts/roboto/Roboto-Regular.ttf` and
  // `folio8-go/fonts/roboto/Roboto-Regular.ttf` are byte-identical, which
  // `TestShippedRobotoMatchesDesignerCatalogue` makes machine-checked. So
  // picking it used to embed ~348 KB of duplicate as a Regular-only entry that
  // could never bold, while `Roboto Bold` sat unreachable in the same FontSet.
  //
  // It now NAMES the face and declares that family's cuts. Still two commands
  // and two undo entries, still in the engine's forced order — only the first
  // command changed kind.
  it('names a family the release already ships, declaring its cuts and embedding nothing', async () => {
    // NO FETCH AT ALL is part of the claim: nothing is read, so nothing can be
    // written into the document. The stub is here to catch one, not to serve it.
    const fetchStub = vi.fn(async () => ({ ok: true, arrayBuffer: async () => new Uint8Array([0]).buffer }))
    const restore = globalThis.fetch
    globalThis.fetch = fetchStub as never
    try {
      const request = select()
      const sent = request.mock.calls as unknown as ReadonlyArray<readonly [string, ArrayBuffer]>
      fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
      fireEvent.click(screen.getByRole('option', { name: /^Roboto$/ }))
      await waitFor(() => expect(sent).toHaveLength(2))
      const kinds = sent.map(([, buffer]) => (JSON.parse(new TextDecoder().decode(buffer)) as Record<string, unknown>)['kind'])
      expect(kinds).toEqual(['addFontChain', 'updateComponentProperties'])
      // NOTHING WAS READ. `embedFontFamily` is the only command that carries
      // bytes, and it is not here; no bundle asset was fetched either.
      expect(fetchStub).not.toHaveBeenCalled()
      const payload = JSON.parse(new TextDecoder().decode(sent[0][1])) as Record<string, unknown>
      expect(payload['name']).toBe('Roboto')
      expect(payload['data'], 'a named family carries no bytes; naming a face is not embedding it').toBeUndefined()
      // THE CUTS, DECLARED. This is the assertion the whole story exists for:
      // the entry names the shipped face and says what Roboto's three cuts are,
      // so the element can bold without anything being inferred from a name.
      //
      // ⚠ AND IT CARRIES THE SAME PROPOSED FALLBACK TAIL THE EMBED PATH
      // COMPUTES. The declare path replaced an embed that computed one, and the
      // first cut of it sent a ONE-entry chain: latin kept working and every
      // Thai and CJK run in the document silently lost its fallback. Roboto's
      // catalogue `scripts` is `["latin"]`, so the answer is exactly the
      // three-entry chain `starter.folio` already declares — which
      // `pick_declares_cuts_ext_test.go` compares against that file itself, so
      // this expectation and the shipped document cannot drift apart.
      expect(payload['entries']).toEqual([
        { face: 'Roboto', bold: 'Roboto Bold', italic: 'Roboto Italic', boldItalic: 'Roboto Bold Italic' },
        { face: 'Noto Sans Thai', bold: 'Noto Sans Thai Bold' },
        'Noto Sans SC',
      ])
      // AND NO VARIANT NAMES ITS OWN BASE — such a pick would author a document
      // this story's own parse narrowing refuses to reload (D-11.2.11).
      const entry = (payload['entries'] as ReadonlyArray<Record<string, string>>)[0]
      for (const key of ['bold', 'italic', 'boldItalic']) expect(entry[key]).not.toBe(entry['face'])
      // THE PROPERTY IS COMMITTED SECOND, and only after the chain exists.
      const property = JSON.parse(new TextDecoder().decode(sent[1][1])) as Record<string, unknown>
      expect(property['changes']).toEqual({ fontFamily: { op: 'set', value: 'Roboto' } })
    } finally {
      globalThis.fetch = restore
    }
  })

  // RETIRED (Story 16.9): five tests drove `AVAILABLE TO INSTALL` — the
  // dropdown's third group, which no longer exists — by focusing the
  // combobox, typing 'Kanit', and clicking the row labelled 'install on
  // this machine'. That row is gone: 'Add fonts...' is now the only door to
  // a family not on this machine, so the scenarios these tests drove
  // (resolving METADATA.pb into an install; one pick at a time across the
  // web-tier resolution; releasing the pick hold on a mid-resolution
  // document replacement; a directory disagreeing with its declared
  // licence token; installing with no network) can no longer be reached
  // from this control at all. OF THE THREE MECHANISMS THEY EXERCISED, TWO ARE
  // UNIT-TESTED DIRECTLY AND ONE IS NOT, and the difference is now stated
  // instead of averaged over: `fetchWebFamily` / `timedFetcher`'s stall
  // handling is covered in `font-source.test.ts`, and the licence
  // classification in `font-licence.test.ts`.
  //
  // THE ONE-RESOLUTION-AT-A-TIME GUARD IS IN NEITHER FILE, and this comment
  // used to say it was. It cannot be: the guard is not in either module. It is
  // `App.tsx`'s own `fontChainBusyRef` / `holdFontChain` (App.tsx:114-115),
  // read by both surviving doors — `addFamilyToDocument` (App.tsx:885) and
  // `embedInstalledFamily` (App.tsx:1073) — and handed back on a document
  // replacement by `setCurrentSnapshot` rather than by either `finally`, which
  // is generation-guarded and deliberately declines to release a moved hold.
  // Measured against the claim: searching both named files for
  // `at a time|concurrent|overlapping|in flight|busy|re-entran` returns 0 hits,
  // against 29 `fetchWebFamily` hits in `font-source.test.ts` as the positive
  // control that the search itself works.
  //
  // WHERE IT IS COVERED NOW: two tests in `App.font-store.test.tsx`, driven
  // through the surviving `AVAILABLE LOCALLY` first-use door — 'releases the
  // pick hold when the document is replaced mid-resolution, so a later pick
  // still commits' (the flag stranded, so every later pick is dead for the
  // session) and 'refuses a pick made against another component while the
  // first embed is still in flight, and sends one embed' (the flag never
  // taken, so two overlapping picks both resolve and two embeds commit). The
  // deleted row is what went; the mechanism did not.
  //
  // AND ALL THREE ARE STILL REACHABLE END TO END through the font browser's
  // own 'Add fonts...' flow, which this story does not touch.

  // STORY 17.3 TURNED THIS ROW OVER. It used to assert the engine's default
  // arrived as a PLACEHOLDER and not a value — grey chrome the author could not
  // read as fact, could not step, and could not commit. It is now the box's own
  // text. The number is still the engine's and still arrives on the projection;
  // only where it is painted changed.
  it('shows the engine\'s own default size for an element that commits none, as the box\'s VALUE', () => {
    select()
    const size = screen.getByRole('textbox', { name: 'Font size (pt)' })
    expect(size).toHaveValue('12')
    // The placeholder still spells the same number, for the one state the value
    // does not cover: a box the author has emptied but not yet blurred.
    expect(size).toHaveAttribute('placeholder', '12')
  })

  it('commits alignment from the closed sets, and clears it by pressing the active segment again', async () => {
    const request = select()
    for (const name of ['Align left', 'Align center', 'Align right', 'Align justify', 'Vertical align top', 'Vertical align middle', 'Vertical align bottom']) {
      expect(screen.getByRole('button', { name })).toHaveAttribute('aria-pressed', 'false')
    }
    fireEvent.click(screen.getByRole('button', { name: 'Align center' }))
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
  })

  // STORY 14.1 / AC3 + AC4 — VERTICAL ALIGN IS DRAWN, NOT SPELLED.
  //
  // Align and Vertical align sit side by side in ONE `.property-grid` at
  // `1fr 1fr`, rendered by ONE `SegmentedProperty` through ONE
  // `.property-segment` class, and until now one drew icons while the other drew
  // the words TOP / MID / BOT at the same size in the same row. The seven
  // `label` values are untouched, so every accessible name here is the one
  // `App.test.tsx` above already asserts as a set.
  it('draws both alignment controls in one vocabulary, each segment keeping the name it answered to', () => {
    select()
    for (const name of ['Align left', 'Align center', 'Align right', 'Align justify', 'Vertical align top', 'Vertical align middle', 'Vertical align bottom']) {
      const segment = screen.getByRole('button', { name })
      expect(segment.querySelector('svg.segment-icon'), name).not.toBeNull()
      // NOT merely "it has an icon": an icon with a caption beside it would
      // still be words beside icons at the same size, which is the defect.
      expect(segment.textContent, name).toEqual('')
    }
    for (const word of ['TOP', 'MID', 'BOT']) expect(screen.queryByText(word), word).toBeNull()
  })

  // Story 7.4 / AC3. `style.align` admits `justify` for a text element, and
  // has since 7.3; a table's cells draw a justified value at their start
  // edge, so the value is meaningless there and the control must not offer
  // it. A MIXED selection is the case that decides the rule: one command goes
  // to every id, so the segment is offered only when EVERY selected component
  // is text.
  it('offers justify for text alone, and never for a table or a mixed selection', async () => {
    const table = { id: 'e2', type: 'table' as const, band: 'content' as const, x: 0, y: 30_000, width: 72_000, height: 12_000, resizable: false, tableBind: 'transactions[]' }
    const sent: ArrayBuffer[] = []
    const request = vi.fn(async (_operation: string, payload?: ArrayBuffer) => { if (payload) sent.push(payload); return { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3 } } })
    const componentCanvas = { ...canvas, components: [textComponent, table] }
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)

    const alignNames = () => within(screen.getByRole('group', { name: 'Align' })).getAllByRole('button').map((button) => button.getAttribute('aria-label'))
    fireEvent.click(screen.getByLabelText('text component e1'))
    expect(alignNames()).toEqual(['Align left', 'Align center', 'Align right', 'Align justify'])
    // The glyph is an SVG path, never a CSS declaration asking the browser to
    // justify: canvas-authority-contract.test.ts bans that across every
    // production, unit and e2e source.
    expect(screen.getByRole('button', { name: 'Align justify' }).querySelector('svg path')).toBeInTheDocument()

    fireEvent.click(screen.getByLabelText('table component e2'))
    expect(alignNames()).toEqual(['Align left', 'Align center', 'Align right'])

    fireEvent.click(screen.getByLabelText('text component e1'))
    fireEvent.click(screen.getByLabelText('table component e2'), { shiftKey: true })
    expect(alignNames()).toEqual(['Align left', 'Align center', 'Align right'])

    fireEvent.click(screen.getByLabelText('table component e2'), { shiftKey: true })
    fireEvent.click(screen.getByRole('button', { name: 'Align justify' }))
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(new TextDecoder().decode(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"align":{"op":"set","value":"justify"}}}')
  })

  // Story 7.4 / AC4. lineSpacing is a dimensionless RATIO carried as a RAW,
  // UNQUOTED JSON number: Go's own decoder performs the x1000 to thousandths,
  // exactly as it does for a value written in a .folio file. Quoting it, or
  // pre-multiplying it here, is refused by the engine.
  it('shows an unset ratio as the engine\'s own value and commits a typed one as a raw unquoted number', async () => {
    const sent: ArrayBuffer[] = []
    const request = vi.fn(async (_operation: string, payload?: ArrayBuffer) => { if (payload) sent.push(payload); return { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3 } } })
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: [textComponent] } }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    const leading = screen.getByRole('textbox', { name: 'Line spacing' })
    // STORY 17.3. The leading the declared chain itself rules — a ratio of 1 —
    // read from `canvas.defaultLineSpacing` and carried as the box's VALUE.
    // This file's `canvas` fixture sets it to 1000 thousandths, so the `1` below
    // is the projection's number and not a constant in the component.
    expect(canvas.defaultLineSpacing).toBe(1000)
    expect(leading).toHaveValue('1')
    expect(leading).toHaveAttribute('placeholder', '1')
    expect(leading).toHaveAttribute('inputMode', 'decimal')
    fireEvent.change(leading, { target: { value: '1.5' } })
    fireEvent.keyDown(leading, { key: 'Enter' })
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(new TextDecoder().decode(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"lineSpacing":{"op":"set","value":1.5}}}')
  })

  it('reads a committed line spacing back in the author\'s units and clears it from the same row', async () => {
    const sent: ArrayBuffer[] = []
    const request = vi.fn(async (_operation: string, payload?: ArrayBuffer) => { if (payload) sent.push(payload); return { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3 } } })
    const spaced = { ...canvas, components: [{ ...textComponent, lineSpacing: 1_500 }] }
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: spaced }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    // The engine carries thousandths; the author is shown the ratio.
    expect(screen.getByRole('textbox', { name: 'Line spacing' })).toHaveValue('1.5')
    fireEvent.click(screen.getByRole('button', { name: 'Clear Line spacing' }))
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(new TextDecoder().decode(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"lineSpacing":{"op":"clear"}}}')
  })

  // The mock reproduces the engine's REAL rejection, measured by running the
  // command through Go: applyPropertyChanges prefixes the command key and
  // template's validator words its reason in terms of that same key, so the
  // message the browser receives really does begin `lineSpacing: lineSpacing`.
  // ComponentCommandError carries it verbatim, with DataPath
  // `component.lineSpacing` on element `e1`; the Go half is pinned by
  // TestLineSpacingPropertyCommandDecodesThroughTheOneLoaderValidator.
  it('shows the engine\'s located line-spacing refusal and keeps the author\'s text', async () => {
    const engineMessage = 'lineSpacing: lineSpacing must be between 1 and 1000000 thousandths (0.001 to 1000); 0 is outside that range'
    const request = vi.fn((operation: string) => operation === 'command'
      ? Promise.reject(Object.assign(new Error(engineMessage), { elementId: 'e1', dataPath: 'component.lineSpacing' }))
      : Promise.resolve({ snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3 } }))
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: [textComponent] } }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    const leading = screen.getByRole('textbox', { name: 'Line spacing' })
    fireEvent.change(leading, { target: { value: '0' } })
    fireEvent.keyDown(leading, { key: 'Enter' })
    // The WHOLE located sentence, not a prefix of it: the element, the field
    // it was located to, and the engine's own reason including the offending
    // value the author typed.
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent(`e1: component.lineSpacing: ${engineMessage}`))
    expect(leading).toHaveValue('0')
    expect(leading).toHaveAttribute('aria-invalid', 'true')
  })

  it('presses the segment the engine has committed, and clears it from the same control', async () => {
    const aligned = { ...canvas, components: [{ ...textComponent, align: 'right' as const, valign: 'middle' as const }] }
    const sent: ArrayBuffer[] = []
    const request = vi.fn(async (_operation: string, payload?: ArrayBuffer) => { if (payload) sent.push(payload); return { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3 } } })
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: aligned }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    const right = screen.getByRole('button', { name: 'Align right' })
    expect(right).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Vertical align middle' })).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(right)
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(sent.map((payload) => new TextDecoder().decode(payload)).join('')).toContain('"op":"clear"')
  })

  // B and I are toggles, so they clear themselves: the inline × that used to
  // sit beside each of them was a second control for what one press already
  // says. Pressing a pressed toggle unsets the property rather than writing
  // `false`, which is SegmentedProperty's contract one row above.
  it('clears bold from the toggle itself, with no separate × beside it', async () => {
    const emboldened = { ...canvas, components: [{ ...textComponent, bold: true }] }
    const sent: ArrayBuffer[] = []
    const request = vi.fn(async (_operation: string, payload?: ArrayBuffer) => { if (payload) sent.push(payload); return { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3 } } })
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: emboldened }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    const bold = screen.getByRole('button', { name: 'Bold' })
    expect(bold).toHaveAttribute('aria-pressed', 'true')
    // Positive control for the two absences: the row that DOES keep an inline
    // clear is right beside these, so an empty panel cannot fake this pass.
    expect(screen.getByRole('button', { name: 'Clear Line spacing' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Clear Bold' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Clear Italic' })).not.toBeInTheDocument()
    fireEvent.click(bold)
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(new TextDecoder().decode(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"bold":{"op":"clear"}}}')
  })

  it('sets bold from an unset toggle, and never writes bold false', async () => {
    const sent: ArrayBuffer[] = []
    const request = vi.fn(async (_operation: string, payload?: ArrayBuffer) => { if (payload) sent.push(payload); return { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3 } } })
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: [textComponent] } }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    const bold = screen.getByRole('button', { name: 'Bold' })
    expect(bold).toHaveAttribute('aria-pressed', 'false')
    fireEvent.click(bold)
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(new TextDecoder().decode(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"bold":{"op":"set","value":true}}}')
  })

  // PICKING THE FAMILY A COMPONENT ALREADY HAS — AD-15, ON A PROPERTY COMMIT.
  //
  // `choose` (App.tsx:2688-2692) sends the command UNCONDITIONALLY. There is no
  // comparison against `committed` anywhere in that path, and there must not be
  // one: `:3096-3109` is a deliberate ruling the other way for the same class of
  // control, whose comment names the no-send behaviour "the tempting wrong
  // implementation". THIS TEST IS NOT A NO-SEND GUARD. It asserts the send, and
  // then asserts that the ENGINE is what makes the send harmless.
  //
  // The engine's half is `folio8-go/wasm/engine.go:240-246` — canonical bytes
  // that did not move return the stable snapshot, "not committed mutations:
  // preserve revision, dirty state, preview authority, and both history branches
  // exactly as they were". So the property that matters is a UI property: given
  // that snapshot back, the designer must not invent a revision, a dirty flag or
  // an Undo entry of its own.
  //
  // NOTHING COVERED THIS BEFORE. `:665-672` is the model — the same shape, on
  // page setup — and it is the only stable-snapshot test in this file; no test
  // anywhere covered a property commit, and none covered `fontFamily`. The gap
  // is what let Story 16.8's starter rename reach an e2e run before anything
  // said what a declared-family pick is supposed to do.
  //
  // THE DOCUMENT IS OPENED RATHER THAN RENDERED, because "stays non-dirty" is
  // otherwise unobservable: `App.tsx:1399` reads dirty as `savedRevision ===
  // undefined || snapshot.revision !== savedRevision`, and only a load or a save
  // ever establishes `savedRevision`.
  it('sends the command when the author picks the family a component already has, and the engine\'s stable snapshot leaves the revision, the dirty flag and Undo exactly where they were', async () => {
    // THE PANEL RENDERS ITS OWN CANVAS, WITH THE COMPONENT ALREADY CARRYING A
    // DECLARED FAMILY. `select()` above answers every command with a snapshot
    // that has NO canvas, and App.tsx:1227 replaces the snapshot wholesale — the
    // inspector would unmount after the first command and a later query could
    // pass because the panel had vanished rather than because anything held.
    const picked = { ...canvas, components: [{ ...textComponent, fontFamily: 'body' }] }
    const stable = { documentState: 'loaded' as const, revision: 7, byteLength: 3, canvas: picked, canUndo: false, canRedo: false }
    const moved = { ...stable, revision: 8, canvas: { ...canvas, components: [{ ...textComponent, fontFamily: 'heading' }] }, canUndo: true }
    const sent: ArrayBuffer[] = []
    // THE ENGINE DECIDES WHAT IS A MUTATION, NOT THE UI (AD-15), so the mock
    // forks on the payload's own value rather than on the call count: the same
    // command shape gets the stable snapshot when it changes nothing and an
    // advanced one when it does.
    const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
      if (operation === 'command' && payload) {
        sent.push(payload)
        return { snapshot: new TextDecoder().decode(payload).includes('"value":"heading"') ? moved : stable }
      }
      return { snapshot: stable, ...(operation === 'serialize' ? { bytes } : {}) }
    })
    const files: FileAccess = { open: vi.fn(async () => ({ bytes, name: 'report.folio' })), acquireSaveTarget: vi.fn(), writeSave: vi.fn() }
    render(<App engine={engine(request as never)} fileAccess={files} initialSnapshot={stable} />)
    fireEvent.click(screen.getByRole('button', { name: 'Open local template' }))
    await waitFor(() => expect(screen.getByText('Saved local file')).toBeInTheDocument())
    expect(screen.getByTestId('engine-snapshot')).toHaveTextContent('GO SNAPSHOT · REVISION 7')
    expect(screen.getByRole('button', { name: 'Undo' })).toBeDisabled()

    fireEvent.click(screen.getByLabelText('text component e1'))
    const combobox = screen.getByRole('combobox', { name: 'Font family' })
    expect(combobox).toHaveValue('body')
    fireEvent.focus(combobox)
    // The row is taken from the DECLARED group, which is the whole subject: a
    // declared row takes the plain-commit branch, never `commitFirstUse`.
    expect(declaredOptions().map(optionText)).toEqual(['body', 'heading'])
    fireEvent.click(within(screen.getByRole('group', { name: 'IN THIS TEMPLATE' })).getByRole('option', { name: 'body' }))

    // THE SEND, ASSERTED FIRST. If this ever reddens because someone added a
    // `draft !== committed` guard to `choose`, the fix is to remove the guard.
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(new TextDecoder().decode(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"fontFamily":{"op":"set","value":"body"}}}')
    // AND THE THREE THINGS THE STABLE SNAPSHOT MUST LEAVE ALONE.
    expect(screen.getByTestId('engine-snapshot')).toHaveTextContent('GO SNAPSHOT · REVISION 7')
    expect(screen.getByText('Saved local file')).toBeInTheDocument()
    expect(screen.queryByText('Unsaved local changes')).not.toBeInTheDocument()
    // BOTH HISTORY BRANCHES, because that is what the engine comment quoted
    // above actually promises. Measuring Undo alone would leave half the
    // sentence unasserted, and `stable` carries `canRedo: false` already.
    expect(screen.getByRole('button', { name: 'Undo' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Redo' })).toBeDisabled()

    // THE POSITIVE CONTROL, IN THIS TEST RATHER THAN A NEIGHBOURING ONE. Every
    // assertion above is an absence, and a panel that had stopped sending
    // anything at all would satisfy all of them. Picking a DIFFERENT declared
    // chain must send exactly one command carrying the changed value, and the
    // engine's answer to that one must move the document.
    fireEvent.focus(combobox)
    fireEvent.click(within(screen.getByRole('group', { name: 'IN THIS TEMPLATE' })).getByRole('option', { name: 'heading' }))
    await waitFor(() => expect(sent).toHaveLength(2))
    const changed = sent.map((payload) => new TextDecoder().decode(payload)).filter((command) => command.includes('"value":"heading"'))
    expect(changed).toEqual(['{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"fontFamily":{"op":"set","value":"heading"}}}'])
    await waitFor(() => expect(screen.getByTestId('engine-snapshot')).toHaveTextContent('GO SNAPSHOT · REVISION 8'))
    expect(screen.getByText('Unsaved local changes')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Undo' })).toBeEnabled()
  })

  // A DECLARED CATALOGUE FAMILY IS OFFERED ONCE — ASSERTED WHERE CI ACTUALLY
  // RUNS IT.
  //
  // `App.tsx:2604`'s `!families.includes(source.family)` is the whole of that
  // property: a family the document already declares has moved into the first
  // group and must not be offered again in the second. Story 16.8 made the
  // starter declare a catalogue family for the first time, so the clause stopped
  // being decorative and started being load-bearing on the very first document a
  // user opens.
  //
  // ITS ONLY GUARD WAS A SUITE NOBODY EXECUTES. `e2e/font-embed-boundary.spec.ts`
  // measures it in a real browser, and CI runs `npm run test:e2e:compile` — which
  // is `tsc -p tsconfig.e2e.json --noEmit` and nothing else (`ci.yml:249`);
  // `playwright test` appears in no workflow. Measured, not assumed: deleting that
  // clause leaves the entire vitest suite, the typecheck and oxlint green and
  // reddens only the spec that never runs. A property whose sole guard is
  // unexecuted is guarded by a comment.
  //
  // `Arimo` is a COMMITTED local-tier face — it is in `catalogueFaces`, so it is
  // in the offered population with no network at all — and declaring a chain of
  // that name is what puts one family on both sides of the declared/installed
  // question. It is the same fixture as the font-browser block's
  // `withArimoDeclared`, which is block-scoped there and cannot be reached from
  // here; both derive their counts from `catalogueFaces` rather than a numeral,
  // so neither can rot into a floor while the other moves.
  it('offers a declared CATALOGUE family under IN THIS TEMPLATE only, and subtracts it from AVAILABLE LOCALLY', () => {
    const declaredCatalogue = { ...canvas, components: [textComponent], fontFamilies: ['body', 'Arimo'], fontChains: [{ name: 'body', entries: [face('Noto Sans')] }, { name: 'Arimo', entries: [face('Noto Sans')] }] }
    const request = vi.fn(async () => ({ snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3 } }))
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: declaredCatalogue }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
    // PRESENT in the first group, in the document's own declaration order.
    expect(groupRows('IN THIS TEMPLATE')).toEqual(['body', 'Arimo'])
    const local = groupRows('AVAILABLE LOCALLY')
    // ABSENT from the second — the half a deleted filter breaks.
    expect(local, 'a family the document declares must not also be offered as one to take from this machine').not.toContain('Arimo')
    // AND THE COUNT, DERIVED, so "absent" cannot be satisfied by an empty group:
    // the local tier is every committed face EXCEPT the one now declared.
    expect(local).toHaveLength(catalogueFaces.length - 1)
    // POSITIVE CONTROL FOR THE POPULATION: a catalogue family this document does
    // NOT declare is still offered there, so the subtraction took exactly one.
    expect(local).toContain('Roboto')
  })
})

// Story 7.4: authoring body text in the designer. The CONTENT control was an
// <input type="text">, which cannot hold a line feed at all, so a
// multi-paragraph clause could not be typed OR pasted — the story's first AC
// in one sentence.
describe('Story 7.4: authoring a multi-paragraph clause', () => {
  const textComponent = { id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello' }
  const openEditor = (sent: ArrayBuffer[]) => {
    const request = vi.fn(async (_operation: string, payload?: ArrayBuffer) => { if (payload) sent.push(payload); return { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3 } } })
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: [textComponent] } }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    return screen.getByRole('textbox', { name: 'Text' })
  }

  it('is a textarea, so Enter inserts a paragraph break instead of committing', async () => {
    const sent: ArrayBuffer[] = []
    const field = openEditor(sent)
    expect(field.tagName).toBe('TEXTAREA')
    // Three paragraphs, typed. Enter commits nothing; blur sends ONE command
    // carrying the whole value with its line feeds intact.
    fireEvent.change(field, { target: { value: 'First clause.\nSecond clause.\nThird clause.' } })
    fireEvent.keyDown(field, { key: 'Enter' })
    expect(sent).toHaveLength(0)
    fireEvent.blur(field)
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(new TextDecoder().decode(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"value":{"op":"set","value":"First clause.\\nSecond clause.\\nThird clause."}}}')
  })

  it('commits a forty-page clause as one command, with every paragraph break intact', async () => {
    const sent: ArrayBuffer[] = []
    const field = openEditor(sent)
    // Well past the 512 BYTES the projection used to cap an element's value
    // at — about eighty English words, less than one numbered clause — which
    // did not merely blank the canvas but REJECTED the edit, because the
    // property command re-projects inside its own transaction.
    const clause = Array.from({ length: 1_900 }, (_value, index) => `Clause ${index + 1}. The parties agree as set out above.`).join('\n')
    expect(new TextEncoder().encode(clause).byteLength).toBeGreaterThan(512)
    fireEvent.change(field, { target: { value: clause } })
    fireEvent.blur(field)
    await waitFor(() => expect(sent).toHaveLength(1))
    const command = new TextDecoder().decode(sent[0]!)
    expect(JSON.parse(command).changes.value.value).toBe(clause)
    expect(sent).toHaveLength(1)
  })

  it('takes only the plain flavour of a word-processor paste, keeping paragraph breaks and dropping the formatting', async () => {
    const sent: ArrayBuffer[] = []
    const field = openEditor(sent)
    fireEvent.change(field, { target: { value: '' } })
    const flavours: Record<string, string> = {
      'text/plain': 'Clause 1.\nClause 2.',
      'text/html': '<p style="font-weight:700;font-family:Georgia">Clause 1.</p><p><em>Clause 2.</em></p>',
      'text/rtf': '{\\rtf1 \\b Clause 1.\\par}',
    }
    const read: string[] = []
    fireEvent.paste(field, { clipboardData: { getData: (flavour: string) => { read.push(flavour); return flavours[flavour] ?? '' } } })
    // The formatting is discarded by never being looked at: no sanitiser, no
    // new dependency, nothing to go wrong on an unusual clipboard.
    expect(read).toEqual(['text/plain'])
    expect(field).toHaveValue('Clause 1.\nClause 2.')
    fireEvent.blur(field)
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(JSON.parse(new TextDecoder().decode(sent[0]!)).changes.value.value).toBe('Clause 1.\nClause 2.')
  })

  it('inserts nothing when the clipboard has no plain flavour, instead of letting the browser paste the HTML', () => {
    const sent: ArrayBuffer[] = []
    const field = openEditor(sent)
    fireEvent.change(field, { target: { value: 'Clause 1.' } })
    const read: string[] = []
    const flavours: Record<string, string> = {
      'text/html': '<p style="font-weight:700;font-family:Georgia">Pasted from a word processor</p>',
      'text/rtf': '{\\rtf1 \\b Pasted from a word processor\\par}',
    }
    // fireEvent returns false when the event was cancelled, which is the only
    // way to see from here that the BROWSER's own paste was refused. Without
    // preventDefault the browser inserts text it derives from the text/html
    // flavour — formatting laundered in through the door the story closed.
    const dispatched = fireEvent.paste(field, { clipboardData: { getData: (flavour: string) => { read.push(flavour); return flavours[flavour] ?? '' } } })
    expect(dispatched).toBe(false)
    expect(read).toEqual(['text/plain'])
    expect(field).toHaveValue('Clause 1.')
    expect(sent).toHaveLength(0)
  })

  it('leaves the caret at the end of the pasted text, not at the end of the field', () => {
    const sent: ArrayBuffer[] = []
    const field = openEditor(sent) as HTMLTextAreaElement
    fireEvent.change(field, { target: { value: 'Clause 1.\nClause 3.' } })
    // The caret sits at the start of the LAST paragraph, and a clause is
    // pasted in front of it. A controlled textarea re-rendered with a new
    // value drops the caret at the very end unless it is restored, which would
    // put the author's next keystroke in the wrong paragraph.
    field.setSelectionRange(10, 10)
    fireEvent.paste(field, { clipboardData: { getData: () => 'Clause 2.\n' } })
    expect(field).toHaveValue('Clause 1.\nClause 2.\nClause 3.')
    expect(field.selectionStart).toBe(20)
    expect(field.selectionEnd).toBe(20)
  })

  it('reverts and blurs on Escape, exactly as every single-line field does', () => {
    const sent: ArrayBuffer[] = []
    const field = openEditor(sent)
    fireEvent.change(field, { target: { value: 'a draft\nnobody wants' } })
    fireEvent.keyDown(field, { key: 'Escape' })
    expect(field).toHaveValue('Hello')
    expect(sent).toHaveLength(0)
  })

  it('paints one canvas line per engine line and says, in words, when the paint is only a prefix', () => {
    const line = (top: number, text: string) => ({ top, baseline: top + 10_000, advance: 14_000, width: 30_000, fragments: [{ text, x: 0 }] })
    const cut = { ...canvas, components: [{ ...textComponent, textPaint: { overflow: false, truncated: true, lines: [line(0, 'First clause.'), line(14_000, 'Second clause.')] } }] }
    render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: cut }} />)
    expect(document.querySelectorAll('.canvas-text-line')).toHaveLength(2)
    // Stated in words at the component, and in the same sentence a screen
    // reader gets — never by colour, and never only by a CSS class.
    const notice = 'Canvas preview cut short. The whole text is in the document and prints in full.'
    expect(screen.getByText(notice)).toBeInTheDocument()
    expect(screen.getByLabelText(`text component e1: First clause. Second clause.; ${notice}`)).toBeInTheDocument()
  })

  it('says nothing about truncation for an untruncated paint or an empty one', () => {
    const notice = 'Canvas preview cut short. The whole text is in the document and prints in full.'
    const whole = { ...canvas, components: [
      { ...textComponent, textPaint: { overflow: false, truncated: false, lines: [{ top: 0, baseline: 10_000, advance: 14_000, width: 30_000, fragments: [{ text: 'First clause.', x: 0 }] }] } },
      { id: 'e2', type: 'text' as const, band: 'content' as const, x: 0, y: 30_000, width: 72_000, height: 24_000, resizable: true, textPaint: { overflow: false, truncated: false, lines: [] } },
    ] }
    render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: whole }} />)
    expect(screen.queryByText(notice)).not.toBeInTheDocument()
  })
})

// STORY 17.4: ARROW KEYS STEP A NUMBER FIELD.
//
// Fifteen tests: one per row of the story's eleven-row I/O matrix, plus four
// the matrix does not reach — the leading CEILING and the in-flight `disabled`
// decision (both found by mutation, which left the matrix rows green), the
// ORIGIN FLOOR on x and y (found at review: `containComponent` refuses negative
// geometry on this same command path), and the MODIFIED arrow. The trap the story exists
// to avoid is ARITHMETIC, not keys: every value here is a decimal string Go
// parses exactly and refuses beyond three places, passed through UNQUOTED. So
// the assertions below read the WIRE LITERAL and not merely the box, and the
// leading row is the one that would expose float arithmetic — but ONLY when it
// steps more than once. Measured: `1 + 0.1` is exactly `1.1` in IEEE doubles,
// while `1.1 + 0.1` is `1.2000000000000002`, which the engine rejects.
describe('Story 17.4: arrow keys step a number field', () => {
  const at = (over: Record<string, unknown>) => ({ ...canvas, components: [{ id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello', ...over }] })
  // The engine answers without a canvas, so the panel keeps the projection it
  // has and the draft under test is the only thing that moves. Where a test
  // needs the COMMITTED value to follow, it hands back a canvas instead.
  const recorder = (answer?: CanvasProjection) => {
    const sent: ArrayBuffer[] = []
    const request = vi.fn(async (_operation: string, payload?: ArrayBuffer) => { if (payload) sent.push(payload); return { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, ...(answer ? { canvas: answer } : {}) } } })
    return { sent, request }
  }
  const wire = (payload: ArrayBuffer) => new TextDecoder().decode(payload)
  const select = (label = 'text component e1') => fireEvent.click(screen.getByLabelText(label))

  it('steps a point field UP by one point, and the committed value follows', async () => {
    const { sent, request } = recorder(at({ width: 13_000 }))
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({ width: 12_000 }) }} />)
    select()
    const width = screen.getByRole('textbox', { name: 'Width (pt)' })
    expect(width).toHaveValue('12')
    // The arrow is HANDLED, which is what stops the browser throwing the caret
    // to the start of the field on every repeat of a hold.
    expect(fireEvent.keyDown(width, { key: 'ArrowUp' })).toBe(false)
    expect(width).toHaveValue('13')
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(wire(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"width":{"op":"set","value":13}}}')
    // And the engine's own answer is what the box ends up reading — the step
    // does not leave a draft the document never accepted.
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Width (pt)' })).toHaveValue('13'))
  })

  it('steps a point field DOWN by one point', async () => {
    const { sent, request } = recorder()
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({ width: 12_000 }) }} />)
    select()
    const width = screen.getByRole('textbox', { name: 'Width (pt)' })
    fireEvent.keyDown(width, { key: 'ArrowDown' })
    expect(width).toHaveValue('11')
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(wire(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"width":{"op":"set","value":11}}}')
  })

  // THE FLOAT ROW, AND THE ONE ASSERTION IN THIS FILE THAT HAD TO BE MEASURED
  // RATHER THAN REASONED.
  //
  // `1 + 0.1` is EXACTLY `1.1` in IEEE doubles — measured in node: `(1 + 0.1)
  // === 1.1` is `true` and `String(1 + 0.1)` is `"1.1"`. So a SINGLE step up
  // from `1` DOES NOT DISCRIMINATE: a float implementation passes it. This
  // test originally stepped once, and mutation proved it worthless — the whole
  // step path was rewritten to `Number(draft) + 0.1` and every test in this
  // describe stayed green.
  //
  // The divergence begins at the SECOND step: `1.1 + 0.1` is
  // `1.2000000000000002`, and it compounds (`1.3000000000000003`,
  // `1.4000000000000004`, …). `decimal.go` refuses every one of those with
  // "has more than three decimal places", and the literal travels UNQUOTED, so
  // the author would see an arrow press rejected for no reason they could see.
  // Stepping repeatedly is not thoroughness here — it is the only thing that
  // makes this row falsifiable at all.
  it('steps leading by a tenth repeatedly without ever spelling a float', async () => {
    // A REAL ROUND TRIP, because a repeated step needs one. The other tests can
    // answer without a canvas; this one cannot — when the engine answers with
    // no canvas the inspector unmounts, so the second press lands on a detached
    // node, does nothing, and the climb stalls at `1.1` looking green. The mock
    // therefore echoes the committed literal back, read out of the wire by
    // DIGIT GROUPS so the test's own bookkeeping introduces no float either.
    const sent: ArrayBuffer[] = []
    const echoed = (literal: string) => {
      const [whole, fraction = ''] = literal.split('.')
      return Number.parseInt(whole as string, 10) * 1000 + Number.parseInt(fraction.padEnd(3, '0') || '0', 10)
    }
    let committedThousandths = 1_000
    const request = vi.fn(async (_operation: string, payload?: ArrayBuffer) => {
      if (payload) {
        sent.push(payload)
        const literal = /"value":([-0-9.]+)/.exec(new TextDecoder().decode(payload))?.[1]
        if (literal !== undefined) committedThousandths = echoed(literal)
      }
      return { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas: at({ lineSpacing: committedThousandths }) } }
    })
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({ lineSpacing: 1_000 }) }} />)
    select()
    expect(screen.getByRole('textbox', { name: 'Line spacing' })).toHaveValue('1')
    // `1.2` is the first value a float implementation gets wrong; the rest pin
    // that it does not drift back into agreement further up.
    const climb = ['1.1', '1.2', '1.3', '1.4', '1.5']
    for (const [index, want] of climb.entries()) {
      // RE-QUERIED every iteration, deliberately: a commit re-renders the panel
      // and the handle taken before it is stale, so firing on it silently does
      // nothing and the climb would stall at `1.1` while still reading green on
      // a single-step assertion.
      const box = screen.getByRole('textbox', { name: 'Line spacing' })
      fireEvent.keyDown(box, { key: 'ArrowUp' })
      expect(box).toHaveValue(want)
      await waitFor(() => expect(sent).toHaveLength(index + 1))
      expect(wire(sent[index]!)).toBe(`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"lineSpacing":{"op":"set","value":${want}}}}`)
    }
    // And NO literal it ever sent can be one Go refuses: at most three decimal
    // places, which is the whole of decimal.go's rule.
    for (const payload of sent) expect(wire(payload)).toMatch(/"value":-?\d+(\.\d{1,3})?\}/)
    // The engine ends up holding the value the box shows: five presses, five
    // commits, and no drift between what was displayed and what was sent.
    expect(committedThousandths).toBe(1_500)
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Line spacing' })).toHaveValue('1.5'))
  })

  // THE CEILING — a different guard from the floor, and one the matrix's two
  // floor rows do NOT reach: deleting `Math.min(floored, highest)` left all ten
  // of them green. `lineSpacing` is the only field with an upper bound
  // (`MaxLineSpacingThousandths = 1000000`, a ratio of 1000).
  it("clamps leading UP to the engine's ceiling rather than stepping past it", async () => {
    const { sent, request } = recorder(at({ lineSpacing: 1_000_000 }))
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({ lineSpacing: 999_950 }) }} />)
    select()
    const leading = screen.getByRole('textbox', { name: 'Line spacing' })
    expect(leading).toHaveValue('999.95')
    // A tenth up from 999.95 is 1000.05, which the engine refuses. The step
    // lands ON the ceiling instead. This arm MOVES, so it proves the cap
    // clamps rather than merely declining to act.
    fireEvent.keyDown(leading, { key: 'ArrowUp' })
    expect(leading).toHaveValue('1000')
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(wire(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"lineSpacing":{"op":"set","value":1000}}}')
    expect(wire(sent[0]!)).not.toContain('1000.05')
    // AT the ceiling it stops dead: the key is still taken, but nothing changed
    // and no second command is sent.
    const settled = await screen.findByDisplayValue('1000')
    expect(fireEvent.keyDown(settled, { key: 'ArrowUp' })).toBe(false)
    expect(settled).toHaveValue('1000')
    await Promise.resolve()
    expect(sent).toHaveLength(1)
  })

  // THE DESIGN DECISION THE SPEC'S DESIGN NOTES RECORD, tied to a test so it
  // cannot be undone silently. `shared` carries `disabled: pending`, and
  // MEASURED IN CHROMIUM 1217, disabling a focused input moves
  // `document.activeElement` to the body and re-enabling does NOT give focus
  // back. Key repeat is delivered to the focused element, so raising `pending`
  // on a step would end an arrow HOLD after exactly one press — the one thing
  // this story's browser run exists to photograph. jsdom does not implement
  // blur-on-disable, so the reachable assertion is the `disabled` attribute
  // itself, asserted in BOTH directions against Enter, which still disables.
  it('does not disable the field while a STEP is in flight, though Enter still does', async () => {
    const sent: ArrayBuffer[] = []
    let answer: (() => void) | undefined
    const request = vi.fn(async (_operation: string, payload?: ArrayBuffer) => {
      if (payload) sent.push(payload)
      await new Promise<void>((resolve) => { answer = resolve })
      return { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas: at({ width: 13_000 }) } }
    })
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({ width: 12_000 }) }} />)
    select()
    fireEvent.keyDown(screen.getByRole('textbox', { name: 'Width (pt)' }), { key: 'ArrowUp' })
    await waitFor(() => expect(sent).toHaveLength(1))
    // In flight, and still focusable: the hold survives.
    expect(screen.getByRole('textbox', { name: 'Width (pt)' })).not.toBeDisabled()
    answer!()
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Width (pt)' })).toHaveValue('13'))
    // The contrast arm: a typed commit is not held, and disables exactly as it
    // always has. Without it this test would pass over a control that never
    // disables for any reason.
    const typed = screen.getByRole('textbox', { name: 'Width (pt)' })
    fireEvent.change(typed, { target: { value: '55' } })
    fireEvent.keyDown(typed, { key: 'Enter' })
    await waitFor(() => expect(sent).toHaveLength(2))
    expect(screen.getByRole('textbox', { name: 'Width (pt)' })).toBeDisabled()
    answer!()
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Width (pt)' })).not.toBeDisabled())
  })

  it('stops a point field at the smallest LEGAL value rather than stepping into a refusal', async () => {
    const { sent, request } = recorder(at({ width: 1 }))
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({ width: 1_000 }) }} />)
    select()
    const width = screen.getByRole('textbox', { name: 'Width (pt)' })
    expect(width).toHaveValue('1')
    // One point down from 1pt is 0, which `component_commands.go` refuses with
    // "width must be positive". The step clamps to the smallest value the
    // representation can spell instead of proposing zero.
    fireEvent.keyDown(width, { key: 'ArrowDown' })
    expect(width).toHaveValue('0.001')
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(wire(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"width":{"op":"set","value":0.001}}}')
    expect(wire(sent[0]!)).not.toContain('"value":0}')
    // AT the floor it stops dead: the key is still taken, so the caret does not
    // jump, but nothing changed and NO second command is sent.
    const settled = await screen.findByDisplayValue('0.001')
    expect(fireEvent.keyDown(settled, { key: 'ArrowDown' })).toBe(false)
    expect(settled).toHaveValue('0.001')
    await Promise.resolve()
    expect(sent).toHaveLength(1)
  })

  it("stops leading at the engine's own floor and sends nothing", async () => {
    const { sent, request } = recorder()
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({ lineSpacing: 1 }) }} />)
    select()
    const leading = screen.getByRole('textbox', { name: 'Line spacing' })
    expect(leading).toHaveValue('0.001')
    fireEvent.keyDown(leading, { key: 'ArrowDown' })
    expect(leading).toHaveValue('0.001')
    await Promise.resolve()
    expect(sent).toHaveLength(0)
    expect(request).not.toHaveBeenCalled()
  })

  // RETIRED AND REPLACED BY STORY 17.3 (orchestrator ruling, 2026-09-04). This
  // row used to read 'does nothing on an UNSET field, and sends no command',
  // on the stated precondition that "an unset field has no value to step, and
  // its placeholder is not one". Story 17.3 removed that precondition: leading
  // and font size now carry the engine's own effective value as text, so the
  // draft parses and the arrow steps it. The guard was correct for the world it
  // shipped into; it is obsolete in the world 17.3 creates, and preserving it
  // would have split the arrow from the keyboard on the same visible value —
  // the exact special case `expect(stepped).toEqual(typed)` exists to forbid.
  //
  // ONLY the unset arm retired. The MIXED row below is untouched and still
  // closed by the same predicate.
  it('steps an UNSET field from the value the box shows, and sends what typing that value would send', async () => {
    // TWO INDEPENDENT DRIVES OF THE SAME FIELD, compared whole — 17.4's own
    // shape. If someone reinstates the unset guard, `stepped` goes empty while
    // `typed` still carries a command, and this reddens on the equality alone.
    const outcome = async (drive: (box: HTMLElement) => void) => {
      const { sent, request } = recorder()
      const view = render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({}) }} />)
      select()
      const box = screen.getByRole('textbox', { name: 'Line spacing' })
      // The document sets NEITHER key, and the box still reads the engine's
      // ratio rather than nothing.
      expect(box).toHaveValue('1')
      drive(box)
      await waitFor(() => expect(sent).toHaveLength(1))
      const captured = { commands: sent.length, wire: sent.map(wire), value: (box as HTMLInputElement).value }
      view.unmount()
      return captured
    }
    const stepped = await outcome((box) => { fireEvent.keyDown(box, { key: 'ArrowUp' }) })
    const typed = await outcome((box) => { fireEvent.change(box, { target: { value: '1.1' } }); fireEvent.keyDown(box, { key: 'Enter' }) })
    // THE PROPERTY THE RULING RESTS ON: the arrow and the keyboard are the same
    // act on the same visible value.
    expect(stepped).toEqual(typed)
    // Non-vacuity for it — something really was sent, and it is the literal Go
    // parses as a ratio and not pre-multiplied thousandths.
    expect(stepped.commands).toBe(1)
    expect(stepped.wire[0]).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"lineSpacing":{"op":"set","value":1.1}}}')
    expect(stepped.value).toBe('1.1')
  })

  // The SIZE box takes the same key, one POINT at a time from the projected
  // default, so the retirement is a property of both shown fields and not of
  // leading alone.
  it('steps an UNSET font size from the engine\'s projected default', async () => {
    const { sent, request } = recorder()
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({}) }} />)
    select()
    const size = screen.getByRole('textbox', { name: 'Font size (pt)' })
    expect(size).toHaveValue('12')
    expect(fireEvent.keyDown(size, { key: 'ArrowUp' })).toBe(false)
    expect(size).toHaveValue('13')
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(wire(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"fontSize":{"op":"set","value":13}}}')
  })

  // A field with NO projected default is unchanged by 17.3 and keeps 17.4's
  // behaviour exactly: border width's placeholder is the word `none`, which is
  // not a value, so its draft is still empty and its arrow still does nothing.
  // This is the positive control for "only fontSize and lineSpacing moved".
  it('still does nothing on an unset field the engine projects no default for', async () => {
    const { sent, request } = recorder()
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({}) }} />)
    select()
    const border = screen.getByRole('textbox', { name: 'Border width (pt)' })
    expect(border).toHaveValue('')
    expect(border).toHaveAttribute('placeholder', 'none')
    expect(fireEvent.keyDown(border, { key: 'ArrowUp' })).toBe(true)
    expect(border).toHaveValue('')
    await Promise.resolve()
    expect(sent).toHaveLength(0)
    expect(request).not.toHaveBeenCalled()
  })

  // THE SECOND DELEGATED ROW, CLOSED BY THE SAME PREDICATE AND NOT A SECOND
  // BRANCH. A mixed selection already presents as an empty draft, so "the
  // draft does not parse" covers it. Stepping it would mean picking one
  // component's width and flattening every other component onto it.
  it('does nothing on a MIXED selection, and sends no command — until the author types a value into it', async () => {
    const mixed = { ...canvas, components: [
      { id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true },
      { id: 'e2', type: 'rect' as const, band: 'content' as const, x: 80_000, y: 0, width: 90_000, height: 24_000, resizable: true },
    ] }
    const { sent, request } = recorder()
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: mixed }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    fireEvent.click(screen.getByLabelText('rect component e2'), { shiftKey: true })
    const width = screen.getByRole('textbox', { name: 'Width (pt)' })
    expect(width).toHaveValue('')
    expect(width).toHaveAttribute('placeholder', 'Mixed')
    expect(fireEvent.keyDown(width, { key: 'ArrowUp' })).toBe(true)
    expect(width).toHaveValue('')
    await Promise.resolve()
    expect(sent).toHaveLength(0)
    // A mixed field the author has TYPED into is no longer empty, and steps
    // like any other draft. Note what this means, since the rationale above is
    // easy to over-read: the guard is "the draft does not parse", NOT "never
    // flatten a selection". Flattening stays one keystroke away and is reached
    // here deliberately — typing a value into a mixed field and committing it
    // to the whole selection is the control's shipped behaviour, and stepping
    // that typed value is the same act. What the empty-draft rule buys is that
    // a bare nudge on an untouched mixed field cannot do it by accident.
    fireEvent.change(width, { target: { value: '20' } })
    fireEvent.keyDown(width, { key: 'ArrowUp' })
    expect(width).toHaveValue('21')
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(wire(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1","e2"],"changes":{"width":{"op":"set","value":21}}}')
  })

  it('does nothing while a DRAG owns the field, exactly as typing does nothing', async () => {
    const { sent, request } = recorder()
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({ width: 12_000 }) }} />)
    const component = screen.getByLabelText('text component e1')
    fireEvent.click(component)
    fireEvent.pointerDown(component, { pointerId: 1, clientX: 10, clientY: 10 })
    fireEvent.pointerMove(component, { pointerId: 1, clientX: 13, clientY: 12 })
    const x = screen.getByRole('textbox', { name: 'X (pt)' })
    expect(x).toHaveAttribute('readonly')
    await waitFor(() => expect(x).toHaveValue('3'))
    // Unhandled, and the live proposal is untouched: the pointer owns the
    // value, and no command is sent behind its back.
    expect(fireEvent.keyDown(x, { key: 'ArrowUp' })).toBe(true)
    await waitFor(() => expect(x).toHaveValue('3'))
    fireEvent.keyDown(x, { key: 'ArrowDown' })
    await waitFor(() => expect(x).toHaveValue('3'))
    await Promise.resolve()
    expect(sent).toHaveLength(0)
  })

  it('leaves a NON-NUMERIC field to the browser, and steps exactly the fields the control already calls decimal', async () => {
    const { sent, request } = recorder()
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({}) }} />)
    select()
    // The prose field and the colour field are both left alone.
    const prose = screen.getByRole('textbox', { name: 'Text' })
    expect(fireEvent.keyDown(prose, { key: 'ArrowUp' })).toBe(true)
    expect(prose).toHaveValue('Hello')
    const borderColour = screen.getByRole('textbox', { name: 'Border colour' })
    fireEvent.change(borderColour, { target: { value: '12' } })
    expect(fireEvent.keyDown(borderColour, { key: 'ArrowUp' })).toBe(true)
    expect(borderColour).toHaveValue('12')
    await Promise.resolve()
    expect(sent).toHaveLength(0)

    // THE NUMERIC SET IS THE ONE THE CONTROL ALREADY KNOWS. Rather than
    // restate a list here — the second authority the story forbids — every
    // textbox in the panel is given the same readable draft and the same
    // arrow, and the set that MOVED is asserted against the set the control
    // marks `inputmode="decimal"` for its own reasons.
    const boxes = screen.getAllByRole('textbox').filter((box) => box.tagName === 'INPUT')
    const decimal = boxes.filter((box) => box.getAttribute('inputmode') === 'decimal').map((box) => box.getAttribute('aria-label'))
    const stepped: string[] = []
    for (const box of boxes) {
      fireEvent.change(box, { target: { value: '4' } })
      fireEvent.keyDown(box, { key: 'ArrowUp' })
      if ((box as HTMLInputElement).value !== '4') stepped.push(box.getAttribute('aria-label') as string)
    }
    expect(decimal).toEqual(['X (pt)', 'Y (pt)', 'Width (pt)', 'Height (pt)', 'Font size (pt)', 'Line spacing', 'Border width (pt)'])
    expect(stepped).toEqual(decimal)
  })

  // FOUND AT REVIEW, NOT BY THE MATRIX. The story's Code Map cited only the
  // `> 0` rule at `component_commands.go:1006`, so x and y were read as
  // unbounded and stepped freely through zero. They are not:
  // `updateComponentPropertiesInPlace` also calls `containComponent`
  // (`:880` -> `:1912`), whose first clause refuses `x < 0 || y < 0`. A
  // component dropped at the origin — which is every fixture in this file, and
  // the common case in the app — would have answered one ArrowDown with an
  // engine refusal the author never asked for. That is precisely the failure
  // the story exists to prevent, so the floor is mirrored and asserted here.
  it('stops x at the band origin rather than stepping to a negative the engine refuses', async () => {
    const { sent, request } = recorder(at({ x: 0 }))
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({ x: 1_000 }) }} />)
    select()
    const x = screen.getByRole('textbox', { name: 'X (pt)' })
    expect(x).toHaveValue('1')
    // One point down from 1pt is 0, which is legal and IS sent — the floor is
    // the origin, not the first positive millipoint. This arm moves, so the
    // test cannot pass by x being unsteppable.
    fireEvent.keyDown(x, { key: 'ArrowDown' })
    expect(x).toHaveValue('0')
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(wire(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"x":{"op":"set","value":0}}}')
    // AT the origin it stops dead. `-1` is what shipped before this guard.
    // Re-queried by ROLE, not by display value: `Y (pt)` also reads `0`.
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'X (pt)' })).toHaveValue('0'))
    const settled = screen.getByRole('textbox', { name: 'X (pt)' })
    expect(fireEvent.keyDown(settled, { key: 'ArrowDown' })).toBe(false)
    expect(settled).toHaveValue('0')
    await Promise.resolve()
    expect(sent).toHaveLength(1)
    for (const payload of sent) expect(wire(payload)).not.toContain('"value":-')
  })

  // A MODIFIED arrow is not this story's to take. Inside a text input Shift+
  // Arrow extends the selection and, on macOS, Cmd+Arrow and Alt+Arrow move the
  // caret; stepping on those would remove three shipped editing gestures. This
  // is the ABSENCE of modifier behaviour, which is different from the coarse/
  // fine stepping the story puts out of scope — that would need asking for.
  it('leaves a MODIFIED arrow to the browser and sends no command', async () => {
    const { sent, request } = recorder()
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({ width: 12_000 }) }} />)
    select()
    const width = screen.getByRole('textbox', { name: 'Width (pt)' })
    for (const modifier of [{ shiftKey: true }, { ctrlKey: true }, { altKey: true }, { metaKey: true }]) {
      expect(fireEvent.keyDown(width, { key: 'ArrowUp', ...modifier })).toBe(true)
      expect(fireEvent.keyDown(width, { key: 'ArrowDown', ...modifier })).toBe(true)
    }
    expect(width).toHaveValue('12')
    await Promise.resolve()
    expect(sent).toHaveLength(0)
    expect(request).not.toHaveBeenCalled()
    // Non-vacuity: the UNMODIFIED arrow on this very field still steps, so the
    // eight presses above were declined for their modifier and not because the
    // field was unsteppable.
    fireEvent.keyDown(width, { key: 'ArrowUp' })
    expect(width).toHaveValue('13')
    await waitFor(() => expect(sent).toHaveLength(1))
  })

  // THE BAND EDGE, BY RULING. The browser does NOT clamp here, and the
  // asymmetry with every floor above it is the whole point.
  //
  // A floor like `width > 0`, or `x >= 0`, is a property of the FIELD: those
  // values are illegal whatever document is open and wherever the component
  // sits. That fact is stable, so mirroring it in the browser cannot go stale
  // in a way that matters. A BAND EDGE is a property of the LAYOUT — it depends
  // on the component's position, on its band's extent, and on the page. For the
  // browser to clamp there it would have to compute where the content band
  // ends, which is geometry the engine owns and projects; that is the same
  // authority boundary AD-17 draws for text, and a second copy of
  // `containComponent` living in the inspector would quietly drift, all to save
  // the author one error message.
  //
  // So the arrow SENDS, the engine refuses, and the panel's existing located
  // alert renders. WHAT THIS TEST PINS is that the arrow is not a special case:
  // same wire literal, same alert, same field state as TYPING that value — so
  // that nobody later "fixes" the arrow into one. The containment rule itself
  // is deliberately not reproduced here (that is the ruling), so the refusal
  // driven below is the sentence the engine really returns: measured in
  // Chromium 1217, committing an out-of-band value yields
  // `e1: component.geometry: folio8: component geometry must stay within content`.
  it("sends a band-edge step and shows the engine's refusal, exactly as typing the same value does", async () => {
    const refusal = 'folio8: component geometry must stay within content'
    const outcome = async (drive: (box: HTMLElement) => void) => {
      const sent: ArrayBuffer[] = []
      const request = vi.fn((operation: string, payload?: ArrayBuffer) => {
        if (payload) sent.push(payload)
        return operation === 'command'
          ? Promise.reject(Object.assign(new Error(refusal), { elementId: 'e1', dataPath: 'component.geometry' }))
          : Promise.resolve({ snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3 } })
      })
      const view = render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({ width: 12_000 }) }} />)
      select()
      const box = screen.getByRole('textbox', { name: 'Width (pt)' })
      drive(box)
      await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument())
      const captured = {
        commands: sent.length,
        wire: sent.map(wire),
        alert: screen.getByRole('alert').textContent,
        value: (box as HTMLInputElement).value,
        invalid: box.getAttribute('aria-invalid'),
      }
      view.unmount()
      return captured
    }

    const stepped = await outcome((box) => { fireEvent.keyDown(box, { key: 'ArrowUp' }) })
    const typed = await outcome((box) => { fireEvent.change(box, { target: { value: '13' } }); fireEvent.keyDown(box, { key: 'Enter' }) })

    // ONE ASSERTION CARRIES THE RULING: the arrow and the keyboard are the same
    // act. Everything below is non-vacuity for it.
    expect(stepped).toEqual(typed)
    // The value really was SENT — the step did not clamp, swallow or skip it.
    expect(stepped.commands).toBe(1)
    expect(stepped.wire[0]).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"width":{"op":"set","value":13}}}')
    // And the engine's own located sentence really did reach the author.
    expect(stepped.alert).toBe(`e1: component.geometry: ${refusal}`)
    expect(stepped.invalid).toBe('true')
    // The author's value is kept, exactly as a refused TYPED value is kept, so
    // the next arrow steps from it rather than from a value that never landed.
    expect(stepped.value).toBe('13')
  })

  it('leaves Enter and Escape exactly as they were', async () => {
    const { sent, request } = recorder(at({ width: 40_000 }))
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({ width: 12_000 }) }} />)
    select()
    const width = screen.getByRole('textbox', { name: 'Width (pt)' })
    // ESCAPE still reverts the box to the committed value, and sends nothing.
    fireEvent.change(width, { target: { value: '99' } })
    fireEvent.keyDown(width, { key: 'Escape' })
    expect(width).toHaveValue('12')
    await Promise.resolve()
    expect(sent).toHaveLength(0)
    // ENTER still commits the typed draft, with the literal it always sent.
    fireEvent.change(width, { target: { value: '40' } })
    fireEvent.keyDown(width, { key: 'Enter' })
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(wire(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"width":{"op":"set","value":40}}}')
    // And both keys still behave on a field the arrows have been stepping. A
    // step is committed the moment it is taken, so Escape after one reverts to
    // the value the engine now holds and not to what was there before it.
    const stepping = await screen.findByDisplayValue('40')
    fireEvent.keyDown(stepping, { key: 'ArrowUp' })
    expect(stepping).toHaveValue('41')
    await waitFor(() => expect(sent).toHaveLength(2))
    fireEvent.keyDown(stepping, { key: 'Escape' })
    expect(stepping).toHaveValue('40')
  })
})

// STORY 17.3. SIZE AND LEADING SHOW THE VALUE THEY USE.
//
// The panel used to paint both defaults as grey placeholder chrome, so an
// author could not tell 12pt from "nothing has been decided". Both boxes now
// carry the real number and committing one writes it.
//
// THE SAFETY PROPERTY IS THE FIRST TEST BELOW AND IT IS NOT A FORMALITY:
// opening a document must never mutate it. Every existing `.folio` would
// silently rewrite itself on being looked at if the default were written
// anywhere but on an author's commit, and the assertion that catches that is
// `request` never having been called — not the box's text.
//
// THE NUMBERS ARE THE ENGINE'S. `defaultFontSize` and `defaultLineSpacing`
// both arrive on the projection; `sourcedFromTheProjection` below drives the
// panel with numbers no constant in App.tsx could produce, which is what stops
// this control becoming a second authority on a default Go owns.
describe('Story 17.3: size and leading show the value they use', () => {
  const at = (over: Record<string, unknown>) => ({ ...canvas, components: [{ id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello', ...over }] })
  const recorder = (answer?: CanvasProjection) => {
    const sent: ArrayBuffer[] = []
    const request = vi.fn(async (_operation: string, payload?: ArrayBuffer) => { if (payload) sent.push(payload); return { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, ...(answer ? { canvas: answer } : {}) } } })
    return { sent, request }
  }
  const wire = (payload: ArrayBuffer) => new TextDecoder().decode(payload)
  const select = () => fireEvent.click(screen.getByLabelText('text component e1'))
  const boxes = () => ({ size: screen.getByRole('textbox', { name: 'Font size (pt)' }), leading: screen.getByRole('textbox', { name: 'Line spacing' }) })

  // MATRIX ROWS 1 AND 2, AND THE STORY'S SAFETY PROPERTY IN ONE PLACE.
  it('OPENS A DOCUMENT THAT SETS NEITHER KEY, shows both engine defaults, and SENDS NO COMMAND', async () => {
    const { sent, request } = recorder()
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({}) }} />)
    select()
    const { size, leading } = boxes()
    expect(size).toHaveValue('12')
    expect(leading).toHaveValue('1')
    // Rendering, selecting and re-rendering the panel are all reads. Nothing
    // reached the engine, so nothing could have reached the document.
    await Promise.resolve()
    expect(sent).toHaveLength(0)
    expect(request).not.toHaveBeenCalled()
  })

  // THE ANTI-SECOND-AUTHORITY TEST. Both numbers are driven from the
  // projection to values NO literal in App.tsx spells — 10pt and a ratio of
  // 1.25. If either box ever went back to reciting its own constant, one of
  // these two assertions is the thing that reddens.
  it('reads BOTH defaults off the projection and not off a constant in the panel', () => {
    const projected = { ...canvas, defaultFontSize: 10_000, defaultLineSpacing: 1_250, components: at({}).components }
    const { request } = recorder()
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: projected }} />)
    select()
    const { size, leading } = boxes()
    expect(size).toHaveValue('10')
    expect(leading).toHaveValue('1.25')
    expect(size).toHaveAttribute('placeholder', '10')
    expect(leading).toHaveAttribute('placeholder', '1.25')
  })

  // MATRIX ROW 3. The one row that would stay green under the tempting wrong
  // implementation: fold the default into `committed` and `draft !== committed`
  // is false, so committing the shown value sends NOTHING while the box still
  // reads 12 and every other test passes.
  it('WRITES THE SHOWN DEFAULT when the author commits it unchanged', async () => {
    const { sent, request } = recorder()
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({}) }} />)
    select()
    const { size } = boxes()
    expect(size).toHaveValue('12')
    fireEvent.keyDown(size, { key: 'Enter' })
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(wire(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"fontSize":{"op":"set","value":12}}}')
  })

  it('WRITES THE SHOWN LEADING when the author commits it unchanged, as a raw ratio', async () => {
    const { sent, request } = recorder()
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({}) }} />)
    select()
    const { leading } = boxes()
    expect(leading).toHaveValue('1')
    fireEvent.keyDown(leading, { key: 'Enter' })
    await waitFor(() => expect(sent).toHaveLength(1))
    // `1`, not `1000`: Go's own decoder performs the x1000 to thousandths, and
    // sending pre-multiplied thousandths is refused as a ratio of 1000.
    expect(wire(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"lineSpacing":{"op":"set","value":1}}}')
  })

  // MATRIX ROW 4 — unchanged behaviour, asserted so the story cannot have
  // broken it on the way past.
  it('commits a CHANGED value exactly as it did before', async () => {
    const { sent, request } = recorder()
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({}) }} />)
    select()
    const { size } = boxes()
    fireEvent.change(size, { target: { value: '14' } })
    fireEvent.keyDown(size, { key: 'Enter' })
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(wire(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"fontSize":{"op":"set","value":14}}}')
  })

  // MATRIX ROW 7.
  it('shows the COMPONENT\'S OWN value where it sets one, and not the default', () => {
    const { request } = recorder()
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({ fontSize: 9_000, lineSpacing: 1_500 }) }} />)
    select()
    const { size, leading } = boxes()
    expect(size).toHaveValue('9')
    expect(leading).toHaveValue('1.5')
  })

  // MATRIX ROW 5, THROUGH THE `×`. Clearing still sends `op:"clear"`, which is
  // what makes Go store the zero Presence that OMITS the key from the file —
  // this story adds a way to set the default explicitly, it does not remove
  // the way to unset. The box then comes back to the value it inherits.
  it('CLEARS to an omitted key, and the box returns to the engine default', async () => {
    const { sent, request } = recorder(at({}))
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({ fontSize: 9_000 }) }} />)
    select()
    expect(boxes().size).toHaveValue('9')
    fireEvent.click(screen.getByRole('button', { name: 'Clear Font size (pt)' }))
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(wire(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"fontSize":{"op":"clear"}}}')
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Font size (pt)' })).toHaveValue('12'))
  })

  // `×` NOW APPEARS ON THESE TWO ROWS EVEN WHEN THE KEY IS ABSENT, because the
  // box has a value to reset. Pressing it is idempotent — the key was already
  // omitted and stays omitted — and the ROW MUST NOT GO BLANK behind it. There
  // is no committed transition to fall back on here (`''` before, `''` after),
  // so the reconciliation of the accepted command is the only thing that puts
  // the value back.
  it('clears an ALREADY-ABSENT key without blanking the row', async () => {
    const { sent, request } = recorder(at({}))
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({}) }} />)
    select()
    expect(boxes().size).toHaveValue('12')
    fireEvent.click(screen.getByRole('button', { name: 'Clear Font size (pt)' }))
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(wire(sent[0]!)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"fontSize":{"op":"clear"}}}')
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Font size (pt)' })).toHaveValue('12'))
  })

  // MATRIX ROW 5, THROUGH THE KEYBOARD, on a key that is ALREADY absent. There
  // is nothing to clear, so nothing is sent — and the row must not be left
  // sitting blank beside a canvas that is still painting 12.
  it('sends NOTHING when the author empties an already-unset box, and puts the default back', async () => {
    const { sent, request } = recorder()
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({}) }} />)
    select()
    const { size } = boxes()
    fireEvent.change(size, { target: { value: '' } })
    expect(size).toHaveValue('')
    fireEvent.blur(size)
    await Promise.resolve()
    expect(sent).toHaveLength(0)
    expect(request).not.toHaveBeenCalled()
    await waitFor(() => expect(screen.getByRole('textbox', { name: 'Font size (pt)' })).toHaveValue('12'))
  })

  // MATRIX ROW 6, AND THE SURVIVING HALF OF STORY 17.4'S GUARD. `inherited`
  // deliberately does not fill a MIXED draft: the components genuinely
  // disagree, a filled box would lie about them, and — because the arrow step
  // reads the draft — it would put a flattening edit one nudge key away.
  //
  // THIS REDS IF THE `same &&` ARM IS DELETED: with it gone both boxes fill
  // with the engine default over a disagreeing selection, the placeholder stops
  // being `Mixed`, and the arrow starts sending a command.
  it('leaves a MIXED selection MIXED — no default shown, no default written, no step', async () => {
    const mixed = { ...canvas, components: [
      { id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello', fontSize: 9_000, lineSpacing: 1_100 },
      { id: 'e2', type: 'text' as const, band: 'content' as const, x: 80_000, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'World', fontSize: 11_000, lineSpacing: 1_400 },
    ] }
    const { sent, request } = recorder()
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: mixed }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    fireEvent.click(screen.getByLabelText('text component e2'), { shiftKey: true })
    const { size, leading } = boxes()
    expect(size).toHaveValue('')
    expect(leading).toHaveValue('')
    expect(size).toHaveAttribute('placeholder', 'Mixed')
    expect(leading).toHaveAttribute('placeholder', 'Mixed')
    // Unhandled, so the browser keeps its own caret behaviour in an empty box.
    expect(fireEvent.keyDown(size, { key: 'ArrowUp' })).toBe(true)
    expect(fireEvent.keyDown(leading, { key: 'ArrowUp' })).toBe(true)
    expect(size).toHaveValue('')
    expect(leading).toHaveValue('')
    await Promise.resolve()
    expect(sent).toHaveLength(0)
    expect(request).not.toHaveBeenCalled()
  })

  // MATRIX ROW 9. The engine's refusal path is untouched, and it is reached
  // FROM the shown default: the author edits the number the box now carries,
  // the engine says no, and the author's own text stays where they left it.
  it('keeps the author\'s text on the existing refusal path', async () => {
    const message = 'lineSpacing: lineSpacing must be between 1 and 1000000 thousandths (0.001 to 1000); 0 is outside that range'
    const request = vi.fn((operation: string) => operation === 'command'
      ? Promise.reject(Object.assign(new Error(message), { elementId: 'e1', dataPath: 'component.lineSpacing' }))
      : Promise.resolve({ snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3 } }))
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({}) }} />)
    select()
    const { leading } = boxes()
    expect(leading).toHaveValue('1')
    fireEvent.change(leading, { target: { value: '0' } })
    fireEvent.keyDown(leading, { key: 'Enter' })
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent(`e1: component.lineSpacing: ${message}`))
    expect(leading).toHaveValue('0')
    expect(leading).toHaveAttribute('aria-invalid', 'true')
  })

  // ESCAPE reverts to what the row INHERITS, not to an empty box: the document
  // still says nothing, so the honest state to return to is the engine's value.
  it('reverts on Escape to the inherited value rather than to nothing', () => {
    const { request } = recorder()
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({}) }} />)
    select()
    const { size } = boxes()
    fireEvent.change(size, { target: { value: '30' } })
    expect(size).toHaveValue('30')
    fireEvent.keyDown(size, { key: 'Escape' })
    expect(size).toHaveValue('12')
    expect(request).not.toHaveBeenCalled()
  })

  // POSITIVE CONTROL for every "no default is shown" claim above: the fields
  // 17.3 does NOT touch keep their placeholder chrome and their empty draft, so
  // the change really is scoped to fontSize and lineSpacing.
  it('touches NO field beyond fontSize and lineSpacing', () => {
    const { request } = recorder()
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at({}) }} />)
    select()
    for (const [name, placeholder] of [['Border width (pt)', 'none'], ['Text colour', 'black'], ['Visible if', 'always']] as const) {
      const box = screen.getByRole('textbox', { name })
      expect(box).toHaveValue('')
      expect(box).toHaveAttribute('placeholder', placeholder)
    }
    expect(request).not.toHaveBeenCalled()
  })
})

describe('spec-barcode-element: barcode canvas paint and inspector', () => {
  const barcodeComponent = { id: 'e1', type: 'barcode' as const, band: 'content' as const, x: 0, y: 0, width: 216_000, height: 48_000, resizable: true, value: '1234567890', barcode: { moduleWidth: 1_000, bars: [{ x: 10_000, width: 2_000 }, { x: 13_000, width: 1_000 }] } }

  it('draws one bar per Go-computed bar, placed and sized through the zoom rule', async () => {
    const view = render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: [barcodeComponent] } }} />)
    const bars = () => Array.from(view.container.querySelectorAll<HTMLElement>('[data-component-id="e1"] .canvas-barcode-bar'))
    expect(bars()).toHaveLength(2)
    expect(bars().map((bar) => [bar.style.left, bar.style.width])).toEqual([['10px', '2px'], ['13px', '1px']])
    fireEvent.click(screen.getByRole('button', { name: 'Zoom in' }))
    await waitFor(() => expect(screen.getByLabelText('Canvas zoom')).toHaveTextContent('110%'))
    expect(bars().map((bar) => [bar.style.left, bar.style.width])).toEqual([['11px', '2.2px'], ['14.3px', '1.1px']])
  })

  it('echoes the engine reason when a barcode cannot be painted', () => {
    const { barcode: _paint, ...unpainted } = barcodeComponent
    render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: [{ ...unpainted, barcodeUnavailable: 'doesNotFit' as const }] } }} />)
    expect(screen.getByText('Barcode not drawn — it does not fit its box')).toBeInTheDocument()
    expect(document.querySelector('.canvas-barcode-bar')).toBeNull()
  })

  it('offers a Content field and no Fill or Border controls for a selected barcode', () => {
    render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: [barcodeComponent] } }} />)
    fireEvent.click(screen.getByLabelText('barcode component e1'))
    expect(screen.getByRole('textbox', { name: 'Content' })).toHaveValue('1234567890')
    expect(screen.queryByRole('textbox', { name: /^(Fill|Background|Border)/ })).not.toBeInTheDocument()
    // Positive control: visibility still sits in the BOX section.
    expect(screen.getByRole('textbox', { name: 'Visible if' })).toBeInTheDocument()
  })

  it('shows Content over five rows, lets Enter make a new line and commits it after a pause without leaving the field', async () => {
    vi.useFakeTimers()
    try {
      const request = vi.fn(async () => ({ snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas: { ...canvas, components: [barcodeComponent] } } }))
      const sent = () => (request.mock.calls as unknown as ReadonlyArray<[string, ArrayBuffer]>).filter(([operation]) => operation === 'command').map(([, payload]) => new TextDecoder().decode(payload))
      render(<App engine={engine(request)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: [barcodeComponent] } }} />)
      fireEvent.click(screen.getByLabelText('barcode component e1'))
      const box = screen.getByRole('textbox', { name: 'Content' }) as HTMLTextAreaElement
      expect(box.tagName).toBe('TEXTAREA')
      expect(box).toHaveAttribute('rows', '5')
      box.focus()
      // Enter is left to the textarea (not prevented, nothing committed): it
      // inserts the new line that jsdom does not type itself, so the change does.
      expect(fireEvent.keyDown(box, { key: 'Enter' })).toBe(true)
      expect(sent()).toHaveLength(0)
      fireEvent.change(box, { target: { value: '1234\n567890' } })
      expect(sent()).toHaveLength(0)
      await act(async () => { await vi.advanceTimersByTimeAsync(PROSE_COMMIT_DEBOUNCE_MS) })
      // The line feed goes to Go, which stores it as a carriage return.
      expect(sent().at(-1)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"value":{"op":"set","value":"1234\\n567890"}}}')
      expect(document.activeElement).toBe(box)
    } finally { vi.useRealTimers() }
  })
})

describe('spec-qrcode-element: QR code canvas paint and inspector', () => {
  const absent = { state: 'absent' as const }
  const authored = { visibleIf: absent, fontFamily: absent, fontSize: absent, lineSpacing: absent, bold: absent, italic: absent, align: absent, valign: absent, color: absent, background: absent, borderWidth: absent, borderColor: absent, borderEdges: absent, errorCorrection: { state: 'value' as const, value: 'H' as const } }
  const qrcodeComponent = { id: 'e1', type: 'qrcode' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 72_000, resizable: true, value: 'folio8', authored, qrcode: { moduleWidth: 2_000, rects: [{ x: 8_000, y: 8_000, width: 14_000, height: 2_000 }, { x: 8_000, y: 10_000, width: 2_000, height: 2_000 }] } }

  it('draws one rect per Go-computed module run, placed and sized through the zoom rule', async () => {
    const view = render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: [qrcodeComponent] } }} />)
    const rects = () => Array.from(view.container.querySelectorAll<HTMLElement>('[data-component-id="e1"] .canvas-qrcode-rect'))
    expect(rects()).toHaveLength(2)
    expect(rects().map((rect) => [rect.style.left, rect.style.top, rect.style.width, rect.style.height])).toEqual([['8px', '8px', '14px', '2px'], ['8px', '10px', '2px', '2px']])
    fireEvent.click(screen.getByRole('button', { name: 'Zoom in' }))
    await waitFor(() => expect(screen.getByLabelText('Canvas zoom')).toHaveTextContent('110%'))
    expect(rects().map((rect) => [rect.style.left, rect.style.width])).toEqual([['8.8px', '15.4px'], ['8.8px', '2.2px']])
  })

  it('echoes the engine reason when a QR code cannot be painted', () => {
    const { qrcode: _paint, ...unpainted } = qrcodeComponent
    render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: [{ ...unpainted, qrcodeUnavailable: 'tooLong' as const }] } }} />)
    expect(screen.getByText('QR code not drawn — its value is too long for a QR Code at this level')).toBeInTheDocument()
    expect(document.querySelector('.canvas-qrcode-rect')).toBeNull()
  })

  it('offers Content and an error-correction control, no Fill or Border, and sends the level as an engine property', async () => {
    const request = vi.fn(async () => ({ snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas: { ...canvas, components: [qrcodeComponent] } } }))
    render(<App engine={engine(request)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: [qrcodeComponent] } }} />)
    fireEvent.click(screen.getByLabelText('qrcode component e1'))
    expect(screen.getByRole('textbox', { name: 'Content' })).toHaveValue('folio8')
    expect(screen.getByRole('textbox', { name: 'Content' })).toHaveAttribute('rows', '5')
    expect(screen.queryByRole('textbox', { name: /^(Fill|Background|Border)/ })).not.toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Visible if' })).toBeInTheDocument()
    // The authored level is the pressed segment; pressing another sets it.
    expect(screen.getByRole('button', { name: /Error correction H/ })).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(screen.getByRole('button', { name: /Error correction Q/ }))
    await waitFor(() => expect(request).toHaveBeenCalled())
    const sent = (request.mock.calls as unknown as ReadonlyArray<[string, ArrayBuffer]>).filter(([operation]) => operation === 'command').map(([, payload]) => new TextDecoder().decode(payload))
    expect(sent.at(-1)).toBe('{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"errorCorrection":{"op":"set","value":"Q"}}}')
  })
})

describe('Story 5.13: image asset selection', () => {
  const imageComponent = { id: 'e1', type: 'image' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 48_000, resizable: true, image: { mediaType: 'image/png', assetKey: 'a'.repeat(64), width: 300, height: 200, drawX: 6_000, drawY: 8_000, drawWidth: 60_000, drawHeight: 40_000 } }
  const undecodableImageComponent = { id: 'e2', type: 'image' as const, band: 'content' as const, x: 0, y: 60_000, width: 72_000, height: 48_000, resizable: true, imageUnavailable: 'undecodable' as const }
  const textComponent = { id: 'e3', type: 'text' as const, band: 'content' as const, x: 0, y: 120_000, width: 72_000, height: 24_000, resizable: true }
  const imageFileAccess = (openImage: () => Promise<{ bytes: ArrayBuffer; mediaType: string; name: string }>) => ({ openImage }) as unknown as import('./image-file').ImageFileAccess

  it('shows the IMAGE section carrying the engine snapshot identity for a single image selection, and never for other selections', () => {
    const componentCanvas = { ...canvas, components: [imageComponent, textComponent] }
    render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} imageFileAccess={imageFileAccess(async () => ({ bytes, mediaType: 'image/png', name: 'logo.png' }))} />)
    // Empty selection: no IMAGE section.
    expect(screen.queryByText('IMAGE')).not.toBeInTheDocument()
    // Single image selection: IMAGE section with identity from the snapshot.
    fireEvent.click(screen.getByLabelText('image component e1'))
    expect(screen.getByText('IMAGE')).toBeInTheDocument()
    expect(screen.getByText('image/png · 300×200px · asset aaaaaaaaaaaa…')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Choose image…' })).toBeInTheDocument()
    // Mixed selection (image + text): no IMAGE section.
    fireEvent.click(screen.getByLabelText('text component e3'), { shiftKey: true })
    expect(screen.queryByText('IMAGE')).not.toBeInTheDocument()
    // Non-image single selection: no IMAGE section.
    fireEvent.click(screen.getByLabelText('text component e3'))
    expect(screen.queryByText('IMAGE')).not.toBeInTheDocument()
  })

  it('states the concrete reason for an asset this version cannot render, distinguished by text, not colour alone', () => {
    const componentCanvas = { ...canvas, components: [undecodableImageComponent] }
    render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} imageFileAccess={imageFileAccess(async () => ({ bytes, mediaType: 'image/png', name: 'logo.png' }))} />)
    fireEvent.click(screen.getByLabelText('image component e2'))
    expect(screen.getByText("This version cannot render this asset's media type.")).toBeInTheDocument()
  })

  it('shows an empty, choosable box for a placed image with no file yet', () => {
    // Go projects neither a paint nor an unavailable reason for a box the
    // author has not filled: nothing to draw, and nothing wrong.
    const emptyImageComponent = { id: 'e2', type: 'image' as const, band: 'content' as const, x: 0, y: 60_000, width: 72_000, height: 48_000, resizable: true }
    const componentCanvas = { ...canvas, components: [emptyImageComponent] }
    render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} imageFileAccess={imageFileAccess(async () => ({ bytes, mediaType: 'image/png', name: 'logo.png' }))} />)
    expect(screen.getByText('No image')).toBeInTheDocument()
    expect(screen.queryByText(/Image unavailable/)).not.toBeInTheDocument()
    fireEvent.click(screen.getByLabelText('image component e2'))
    expect(screen.getByText(/No image chosen yet/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Choose image…' })).toBeEnabled()
  })

  it('states the concrete reason when no local picker capability is available in this browser tier', () => {
    const componentCanvas = { ...canvas, components: [imageComponent] }
    render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    fireEvent.click(screen.getByLabelText('image component e1'))
    const button = screen.getByRole('button', { name: 'Choose image…' })
    expect(button).toBeDisabled()
    expect(screen.getByText('No local file picker is available in this browser tier.')).toBeInTheDocument()
  })

  it('drives the keyboard path from a real keyboard SELECTION to a focus-visible picker control, then commits through it', async () => {
    // Finding 15 (review of 2026-08-29): the original version of this test
    // selected the component with fireEvent.click (a mouse-shaped
    // interaction) and only PROVED the picker button was focusable, never
    // dispatching a key event at all. CanvasComponent has its own
    // onKeyDown handler for Enter/Space selection (App.tsx) — exercise
    // THAT, not a click, for the selection half of "selection to picker".
    const componentCanvas = { ...canvas, components: [imageComponent] }
    const openImage = vi.fn(async () => ({ bytes, mediaType: 'image/jpeg', name: 'logo.jpg' }))
    const request = vi.fn(async (operation: string, _payload?: ArrayBuffer) => ({ snapshot: { documentState: 'loaded' as const, revision: operation === 'command' ? 2 : 1, byteLength: 3, canvas: componentCanvas } }))
    render(<App engine={engine(request)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} imageFileAccess={imageFileAccess(openImage)} />)
    const component = screen.getByLabelText('image component e1')
    component.focus()
    fireEvent.keyDown(component, { key: 'Enter' })
    expect(screen.getByText('IMAGE')).toBeInTheDocument()
    const button = screen.getByRole('button', { name: 'Choose image…' })
    // "Keyboard-reachable... with visible colors.select focus" (AC2):
    // proved by moving focus WITHOUT a pointer event and checking it
    // landed. A native <button> converts Enter/Space into a real 'click'
    // event by construction in every browser — jsdom does not synthesise
    // that translation from a bare keyDown, so the committed activation
    // below stands in for it here; the Playwright suite drives the SAME
    // control with a real OS-level key press (image-asset.spec.ts).
    button.focus()
    expect(document.activeElement).toBe(button)
    fireEvent.click(button)
    await waitFor(() => expect(openImage).toHaveBeenCalledOnce())
    await waitFor(() => expect(request.mock.calls.some(([operation]) => operation === 'command')).toBe(true))
    const [, payload] = request.mock.calls.find(([operation]) => operation === 'command')!
    const command = new TextDecoder().decode(payload)
    expect(command).toContain('"kind":"setComponentAsset"')
    expect(command).toContain('"id":"e1"')
    expect(command).toContain('"mediaType":"image/jpeg"')
  })

  it('shows a located diagnostic when the command rejects the picked file, and shows nothing when the picker is cancelled', async () => {
    const componentCanvas = { ...canvas, components: [imageComponent] }
    const request = vi.fn(async (operation: string) => operation === 'command' ? Promise.reject(Object.assign(new Error('asset exceeds the 8388608-byte supported size'), { elementId: 'e1', dataPath: 'component.data' })) : { snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: componentCanvas } })
    render(<App engine={engine(request)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} imageFileAccess={imageFileAccess(async () => ({ bytes, mediaType: 'image/png', name: 'huge.png' }))} />)
    fireEvent.click(screen.getByLabelText('image component e1'))
    fireEvent.click(screen.getByRole('button', { name: 'Choose image…' }))
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('e1: asset exceeds the 8388608-byte supported size'))
  })

  it('shows no error when the local picker is cancelled', async () => {
    const componentCanvas = { ...canvas, components: [imageComponent] }
    render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} imageFileAccess={imageFileAccess(async () => { throw new FileAccessCancelled() })} />)
    fireEvent.click(screen.getByLabelText('image component e1'))
    fireEvent.click(screen.getByRole('button', { name: 'Choose image…' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Choose image…' })).not.toBeDisabled())
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('does not install a setComponentAsset result that resolves after a document replacement (Finding 4)', async () => {
    // AC1's own named red proof: "a command result installed after
    // document replacement". Element ids are reused across documents
    // (e1, e2, ...), and this closure spans the two longest awaits in the
    // app — an OS file dialog, then an engine command carrying up to
    // megabytes — so if Open/Start blank/undo lands in between, a stale
    // command result must never overwrite the newer, authoritative
    // document. Before the fix, applyImageAsset called setCurrentSnapshot
    // unconditionally with no generation/revision guard, matching every
    // OTHER committed-command path's (bindPickedPath, etc.) shape only in
    // that one respect being ABSENT.
    const componentCanvas = { ...canvas, components: [imageComponent] }
    let resolveOpenImage: ((value: { bytes: ArrayBuffer; mediaType: string; name: string }) => void) | undefined
    const openImage = vi.fn(() => new Promise<{ bytes: ArrayBuffer; mediaType: string; name: string }>((resolve) => { resolveOpenImage = resolve }))
    const request = vi.fn(async (operation: string) => {
      if (operation === 'load') return { snapshot: { documentState: 'loaded' as const, revision: 50, byteLength: 3 } }
      if (operation === 'command') return { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas: componentCanvas } }
      return { snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: componentCanvas } }
    })
    render(<App engine={engine(request)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} imageFileAccess={imageFileAccess(openImage)} blankBytes={bytes} />)

    // Start the asset pick — this awaits openImage(), which we hold open.
    fireEvent.click(screen.getByLabelText('image component e1'))
    fireEvent.click(screen.getByRole('button', { name: 'Choose image…' }))
    await waitFor(() => expect(openImage).toHaveBeenCalledOnce())

    // A DOCUMENT REPLACEMENT lands while the picker is still open.
    startBlankFromNew()
    await waitFor(() => expect(screen.getByText('Untitled template')).toBeInTheDocument())
    await waitFor(() => expect(screen.getByTestId('engine-snapshot')).toHaveTextContent('GO SNAPSHOT · REVISION 50'))

    // NOW the picker resolves and the stale command completes.
    resolveOpenImage!({ bytes, mediaType: 'image/jpeg', name: 'logo.jpg' })
    await waitFor(() => expect(request.mock.calls.some(([operation]) => operation === 'command')).toBe(true))

    // The blank document (revision 50) must still be showing — the stale
    // command's revision-2 result must never have been installed.
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(screen.getByTestId('engine-snapshot')).toHaveTextContent('GO SNAPSHOT · REVISION 50')
    expect(screen.getByText('Untitled template')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('paints the image inside the Go-owned draw rectangle at zoom, fetched per asset key, shows an honest placeholder for an undecodable asset, and revokes its object URL on every trigger AC3 names', async () => {
    let paintCounter = 0
    const createObjectURL = vi.fn(() => `blob:paint-${++paintCounter}`)
    const revokeObjectURL = vi.fn()
    const priorCreate = URL.createObjectURL; const priorRevoke = URL.revokeObjectURL
    ;(URL as unknown as { createObjectURL: typeof createObjectURL }).createObjectURL = createObjectURL
    ;(URL as unknown as { revokeObjectURL: typeof revokeObjectURL }).revokeObjectURL = revokeObjectURL
    try {
      const canvasWithKey = (assetKey: string, extra: ReadonlyArray<typeof undecodableImageComponent> = [undecodableImageComponent]) =>
        ({ ...canvas, components: [{ ...imageComponent, image: { ...imageComponent.image, assetKey } }, ...extra] })
      const canvasA = canvasWithKey('a'.repeat(64))
      // Finding 11: asset REPLACEMENT (same element id, new assetKey) is a
      // distinct trigger from document replacement — exercised via a
      // committed setComponentAsset, never a document generation bump.
      const canvasB = canvasWithKey('b'.repeat(64))
      const openImage = vi.fn(async () => ({ bytes, mediaType: 'image/jpeg', name: 'logo.jpg' }))
      const request = vi.fn(async (operation: string) => {
        if (operation === 'asset') return { snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3 }, bytes }
        if (operation === 'command') return { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas: canvasB } }
        // Finding 11: document REPLACEMENT (Start blank) with the SAME
        // assetKey as canvasB — isolates the `generation` dependency from
        // `assetKey`, so a future edit that dropped `generation` from
        // ImagePaint's effect deps would leak here with nothing else red.
        if (operation === 'load') return { snapshot: { documentState: 'loaded' as const, revision: 9, byteLength: 3, canvas: canvasB } }
        return { snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: canvasA } }
      })
      const view = render(<App engine={engine(request)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: canvasA }} imageFileAccess={imageFileAccess(openImage)} blankBytes={bytes} />)
      await waitFor(() => expect(createObjectURL).toHaveBeenCalledOnce())
      const img = () => view.container.querySelector('img.canvas-image-paint') as HTMLImageElement
      expect(img().src).toContain('blob:paint-1')
      // The undecodable second element paints an honest, named placeholder,
      // never a blank box and never a crash.
      expect(screen.getByText(/Image unavailable/)).toBeInTheDocument()

      // Finding 3: the painted element's geometry must equal the engine's
      // OWN draw rectangle (image.drawX/Y/W/H relative to component.x/y),
      // mapped through canvasDisplay's zoom rule — never object-fit, never
      // a browser-computed fit. Checked at the default zoom (1) first.
      expect(img().style.left).toBe('6px'); expect(img().style.top).toBe('8px')
      expect(img().style.width).toBe('60px'); expect(img().style.height).toBe('40px')

      // Finding 3/10: AC3 says the rectangle is "mapped through the
      // existing zoom rule" — this was asserted at NO zoom before. One
      // step of "Zoom in" (+0.1) must scale every one of the four values
      // by exactly the same factor the engine's own zoom rule uses.
      fireEvent.click(screen.getByRole('button', { name: 'Zoom in' }))
      await waitFor(() => expect(screen.getByLabelText('Canvas zoom')).toHaveTextContent('110%'))
      expect(img().style.left).toBe('6.6px'); expect(img().style.top).toBe('8.8px')
      expect(img().style.width).toBe('66px'); expect(img().style.height).toBe('44px')

      // Finding 11, trigger 1 of 3: ASSET REPLACEMENT. Pick a new file for
      // the same element; the committed command repoints its assetKey, and
      // that alone (not a document generation bump) must revoke the first
      // URL and fetch a second.
      fireEvent.click(screen.getByLabelText('image component e1'))
      fireEvent.click(screen.getByRole('button', { name: 'Choose image…' }))
      await waitFor(() => expect(openImage).toHaveBeenCalledOnce())
      await waitFor(() => expect(createObjectURL).toHaveBeenCalledTimes(2))
      expect(revokeObjectURL).toHaveBeenCalledWith('blob:paint-1')
      expect(img().src).toContain('blob:paint-2')

      // Finding 11, trigger 2 of 3: DOCUMENT REPLACEMENT. Start blank loads
      // a canvas whose e1 element carries the SAME assetKey as canvasB —
      // only `generation` changed, isolating it from the assetKey trigger
      // just exercised above.
      startBlankFromNew()
      await waitFor(() => expect(createObjectURL).toHaveBeenCalledTimes(3))
      expect(revokeObjectURL).toHaveBeenCalledWith('blob:paint-2')
      expect(img().src).toContain('blob:paint-3')

      // Finding 11, trigger 3 of 3: DELETION (unmount) revokes the URL this
      // effect most recently created — no accumulation across a session.
      view.unmount()
      expect(revokeObjectURL).toHaveBeenCalledWith('blob:paint-3')
    } finally {
      (URL as unknown as { createObjectURL: typeof priorCreate }).createObjectURL = priorCreate
      ;(URL as unknown as { revokeObjectURL: typeof priorRevoke }).revokeObjectURL = priorRevoke
    }
  })
})

// STORY 7.6 — THE CANVAS DRAWS EVERY PAGE THE DOCUMENT WILL PRODUCE.
//
// Every projection below is a fixture: the sheets, the seams and the
// disclosures are read from `contentWindowOrigins` and
// `contentWindowCountIsExact`, so a test that changed the origins and saw the
// same drawing would be a test of nothing.
describe('canvas sheet stack', () => {
  const origins = [0, 700_000, 1_400_000]
  const threeWindows = { ...canvas, contentWindowCount: 3, contentWindowOrigins: origins, contentWindowPages: [0, 0, 0] }
  const header = { id: 'h1', type: 'text' as const, band: 'pageHeader' as const, x: 0, y: 0, width: 72_000, height: 12_000, resizable: true }
  const footer = { id: 'f1', type: 'text' as const, band: 'pageFooter' as const, x: 0, y: 0, width: 72_000, height: 12_000, resizable: true }
  const at = (id: string, y: number, height = 24_000) => ({ id, type: 'text' as const, band: 'content' as const, x: 0, y, width: 72_000, height, resizable: true })
  const snapshotOf = (projection: CanvasProjection) => ({ documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: projection })
  const sheetLabels = () => Array.from(document.querySelectorAll('.page-surface')).map((surface) => surface.getAttribute('aria-label'))

  it('draws one sheet per projected window, in order, repeating the two bands the engine repeats', () => {
    render(<App engine={engine()} initialSnapshot={snapshotOf({ ...threeWindows, components: [header, at('e1', 0), footer] })} />)
    expect(sheetLabels()).toEqual([
      'Report page 1 of 3 with Page Header, Content, and Page Footer',
      'Report page 2 of 3 with Page Header, Content, and Page Footer',
      'Report page 3 of 3 with Page Header, Content, and Page Footer',
    ])
    // Each sheet carries all three bands, and the repeating bands carry their
    // components — because the engine repeats them onto every printed page.
    expect(document.querySelectorAll('.page-band-pageHeader')).toHaveLength(3)
    expect(document.querySelectorAll('.page-band-pageFooter')).toHaveLength(3)
    expect(document.querySelectorAll('.page-band-content')).toHaveLength(3)
    // One accessible name each, though: a repeated component is one
    // component, and two identical names would make selection ambiguous.
    expect(screen.getByLabelText('text component h1')).toBeInTheDocument()
    expect(screen.getByLabelText('text component f1')).toBeInTheDocument()
    expect(document.querySelectorAll('.canvas-component-echo')).toHaveLength(4)
  })

  it('marks the seam at the projected origin of the NEXT window, and nowhere when that is past the foot', () => {
    render(<App engine={engine()} initialSnapshot={snapshotOf(threeWindows)} />)
    const seams = Array.from(document.querySelectorAll('.page-seam')).map((seam) => (seam as HTMLElement).style.getPropertyValue('--seam-display-y'))
    // origins[1] − origins[0] and origins[2] − origins[1], at zoom 1. The
    // last sheet has no next window, so it draws no marker.
    expect(seams).toEqual(['700px', '700px'])
    // RED PROOF, run and recorded: substituting the window height multiplied
    // by an index for the projected origin gives 729.89px here — the wrong
    // place by a tenth of an inch, and by nine whole sheets on a column with
    // a declared gap. The origins move the marker; nothing else does.
    expect(seams).not.toContain(`${canvas.contentWindowHeight / 1000}px`)
  })

  it('draws no in-sheet seam where the next window begins past the sheet own foot', () => {
    // A declared gap: the next window begins far below this sheet, so the
    // band's own foot IS the boundary and the skipped column region is drawn
    // by nobody.
    const declaredGap = { ...canvas, contentWindowCount: 2, contentWindowOrigins: [0, 7_280_000], contentWindowPages: [0, 0] }
    render(<App engine={engine()} initialSnapshot={snapshotOf(declaredGap)} />)
    expect(document.querySelectorAll('.page-surface')).toHaveLength(2)
    expect(document.querySelectorAll('.page-seam')).toHaveLength(0)
  })

  it('draws a component that crosses a seam on both windows, with one interactive home', () => {
    const spanning = at('e2', 650_000, 100_000)
    render(<App engine={engine()} initialSnapshot={snapshotOf({ ...threeWindows, components: [spanning] })} />)
    // getByLabelText throws on more than one match, so this assertion IS the
    // uniqueness claim.
    const home = screen.getByLabelText('text component e2')
    expect(home.style.getPropertyValue('--component-y')).toBe('650px')
    const echoes = Array.from(document.querySelectorAll('.canvas-component-echo')) as HTMLElement[]
    expect(echoes).toHaveLength(1)
    // The echo is positioned at the component's column offset MINUS the
    // window it is drawn on, so the run reads as continuous across the seam.
    expect(echoes[0]?.style.getPropertyValue('--component-y')).toBe('-50px')
    expect(echoes[0]?.getAttribute('aria-hidden')).toBe('true')
    expect(echoes[0]?.getAttribute('role')).toBeNull()
  })

  it('sends a later-sheet placement as the band-aware createComponent command, carrying a COLUMN coordinate', async () => {
    const request = vi.fn(async () => ({ snapshot: snapshot(2) }))
    render(<App engine={engine(request)} initialSnapshot={snapshotOf(threeWindows)} />)
    fireEvent.click(screen.getByRole('button', { name: 'Place Text' }))
    fireEvent.keyDown(screen.getByLabelText('Content on page 3 of 3'), { key: 'Enter' })
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    // 1400pt is contentWindowOrigins[2] in points — a position in the column,
    // never a pin to sheet three. Go's hitTestBand rectangle is one page tall
    // and is not moved: this routes around it rather than through it.
    expect(new TextDecoder().decode((request.mock.calls[0] as unknown as [string, ArrayBuffer])[1])).toBe('{"kind":"createComponent","version":1,"type":"text","band":"content","x":0,"y":1400,"width":72,"height":24,"snap":true}')
  })

  // ⚠ THE SECOND PLACEMENT SPELLING SELECTS TOO, AND NOTHING ELSE PROVED IT.
  // Review deleted `.then(selectPlaced)` from `placeInBand` ALONE, left `place`
  // intact, and the whole suite stayed green at 1217/1217: every other unit and
  // e2e row places on sheet one, which goes through `place`. This row is the
  // only thing standing between `placeInBand` and a silent regression.
  //
  // ⚠ WHAT THIS DOES NOT COVER: thin-target selection. Per DW-344
  // `createComponentCommand` hardcodes `width: 72, height: 24`, so anything
  // placed on a later sheet arrives as a slab whether the palette said Line or
  // not. This row exercises the LATER-SHEET ROUTE and says nothing about the hit
  // pad; do not read it as covering both.
  it('selects and focuses a component placed on a LATER sheet, not only on sheet one', async () => {
    const placed = at('e9', 1_400_000)
    const request = vi.fn(async () => ({ snapshot: { ...snapshotOf({ ...threeWindows, components: [placed] }), revision: 2 } }))
    render(<App engine={engine(request)} initialSnapshot={snapshotOf(threeWindows)} />)
    expect(screen.getByText('Component properties require a selection.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Place Text' }))
    fireEvent.keyDown(screen.getByLabelText('Content on page 3 of 3'), { key: 'Enter' })
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    await waitFor(() => expect(screen.getByLabelText(/^text component e9/)).toHaveFocus())
    expect(screen.getByText('e9 \u00b7 band: content')).toBeInTheDocument()
    expect(screen.queryByText('Component properties require a selection.')).not.toBeInTheDocument()
    // Neither the selection nor the focus sent anything of their own.
    expect(request).toHaveBeenCalledOnce()
  })

  it('sends a later-sheet POINTER placement through the same column translation as the keyboard one', async () => {
    // The pointer branch and the keyboard branch are two different expressions
    // on the same handler, and only the keyboard one was exercised: replacing
    // the pointer branch with `placeInBand(band.name, point.x, point.y)` — no
    // column origin, page-absolute x — left the whole designer suite green
    // while a mouse-dropped component silently landed on sheet one.
    const request = vi.fn(async () => ({ snapshot: snapshot(2) }))
    render(<App engine={engine(request)} initialSnapshot={snapshotOf(threeWindows)} />)
    fireEvent.click(screen.getByRole('button', { name: 'Place Text' }))
    const band = screen.getByLabelText('Content on page 3 of 3')
    const localX = ['offset', 'X'].join('')
    const localY = ['offset', 'Y'].join('')
    // jsdom exposes these two as prototype getters that always answer 0, so a
    // plain fireEvent property bag is silently dropped; they have to be
    // defined on the native event the handler actually reads.
    const released = createEvent.pointerUp(band)
    Object.defineProperty(released, localX, { value: 120 })
    Object.defineProperty(released, localY, { value: 40 })
    fireEvent(band, released)
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    // x is band-relative (156 page-absolute less the band's own 36), and y is
    // the COLUMN offset: contentWindowOrigins[2] of 1400pt plus the 40pt the
    // pointer sat below the band's head.
    expect(new TextDecoder().decode((request.mock.calls[0] as unknown as [string, ArrayBuffer])[1])).toBe('{"kind":"createComponent","version":1,"type":"text","band":"content","x":120,"y":1440,"width":72,"height":24,"snap":true}')
  })

  it.each(['Enter', ' '])('keeps image placement in a zero-height repeated header from targeting Content with %s', async (key) => {
    const emptyHeader = { ...threeWindows, bands: threeWindows.bands.map((band) => band.name === 'pageHeader' ? { ...band, height: 0 } : band) }
    const request = vi.fn(async () => { throw new Error('component geometry must stay within pageHeader') })
    render(<App engine={engine(request)} initialSnapshot={snapshotOf(emptyHeader)} />)
    fireEvent.click(screen.getByRole('button', { name: 'Place Image' }))
    fireEvent.keyDown(screen.getByLabelText('Page Header on page 2 of 3'), { key })
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    const sent = JSON.parse(new TextDecoder().decode((request.mock.calls[0] as unknown as [string, ArrayBuffer])[1]))
    expect(sent).toMatchObject({ kind: 'createComponent', type: 'image', band: 'pageHeader' })
    expect(screen.getByRole('button', { name: 'Place Image' })).toHaveAttribute('aria-pressed', 'false')
    expect(await screen.findByRole('alert')).toHaveTextContent('component geometry must stay within pageHeader')
  })

  it('keeps the FIRST sheet on today dropComponent payload even when the stack is deep', async () => {
    const request = vi.fn(async () => ({ snapshot: snapshot(2) }))
    render(<App engine={engine(request)} initialSnapshot={snapshotOf(threeWindows)} />)
    fireEvent.click(screen.getByRole('button', { name: 'Place Text' }))
    fireEvent.keyDown(screen.getByLabelText('Content on page 1 of 3'), { key: 'Enter' })
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(new TextDecoder().decode((request.mock.calls[0] as unknown as [string, ArrayBuffer])[1])).toBe('{"kind":"dropComponent","version":1,"type":"text","x":36,"y":56,"snap":true}')
  })

  it('uses the engine accepted edge for a single drag and keeps its content clip', async () => {
    const request = vi.fn(async () => ({ snapshot: snapshot(2) }))
    const client = engine(request)
    const ordinary = client.request.bind(client)
    client.request = ((operation: string, payload?: ArrayBuffer) => operation === 'group-move-preview'
      ? Promise.resolve({ snapshot: snapshotOf(threeWindows), groupMove: { revision: 1, dx: 0, dy: 676000 } })
      : ordinary(operation as never, payload)) as EngineClient['request']
    render(<App engine={client} initialSnapshot={snapshotOf({ ...threeWindows, components: [at('e1', 0)] })} />)
    const component = screen.getByLabelText('text component e1')
    fireEvent.pointerDown(component, { pointerId: 1, clientX: 10, clientY: 10 })
    fireEvent.pointerMove(component, { pointerId: 1, clientX: 10, clientY: 875.89 })
    await waitFor(() => expect(component.style.getPropertyValue('--component-y')).toBe('676px'))
    expect(document.querySelectorAll('.band-window-open')).toHaveLength(0)
    fireEvent.pointerUp(component, { pointerId: 1, clientX: 10, clientY: 875.89 })
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(JSON.parse(new TextDecoder().decode((request.mock.calls[0] as unknown as [string, ArrayBuffer])[1]))).toMatchObject({ kind: 'moveComponents', dy: 865.89, constrainToWindow: true })
  })

  it('renders multiple sheets without the sheet-count banner', () => {
    render(<App engine={engine()} initialSnapshot={snapshotOf(threeWindows)} />)
    expect(document.querySelectorAll('.page-surface')).toHaveLength(3)
    expect(screen.queryByRole('status', { name: 'Canvas sheet disclosure' })).not.toBeInTheDocument()
  })

  it('renders a data-length document without the sheet-count banner', () => {
    const bound = { ...canvas, contentWindowCountIsExact: false, components: [{ id: 'e8', type: 'table' as const, band: 'content' as const, x: 0, y: 54_000, width: 400_000, height: 28_000, resizable: false, tableBind: 'transactions[]' }] }
    render(<App engine={engine()} initialSnapshot={snapshotOf(bound)} />)
    expect(screen.queryByRole('status', { name: 'Canvas sheet disclosure' })).not.toBeInTheDocument()
    expect(document.querySelectorAll('.page-seam')).toHaveLength(0)
    expect(sheetLabels()).toEqual(['Report page with Page Header, Content, and Page Footer'])
  })

  it('describes column positioning in later-sheet component names and keeps first-sheet names unchanged', () => {
    render(<App engine={engine()} initialSnapshot={snapshotOf({ ...threeWindows, components: [at('e1', 0), at('e3', 1_450_000)] })} />)
    // The exact sentence, so deleting it turns this red rather than merely
    // shortening a string nobody asserted.
    expect(screen.getByLabelText('text component e3; on canvas page 3 of 3, which is a consequence of the content above it and can change when the data does — a column position, not a pin to page 3')).toBeInTheDocument()
    // The first sheet's component keeps the name it had before this story.
    expect(screen.getByLabelText('text component e1')).toBeInTheDocument()
  })

  it('draws the first budgeted sheets without the sheet-count banner and never blanks', () => {
    const many = MAX_CANVAS_SHEETS + 5
    const budgeted = { ...canvas, contentWindowCount: many, contentWindowOrigins: Array.from({ length: many }, (_value, index) => index * 700_000), contentWindowPages: Array.from({ length: many }, () => 0) }
    render(<App engine={engine()} initialSnapshot={snapshotOf(budgeted)} />)
    expect(document.querySelectorAll('.page-surface')).toHaveLength(MAX_CANVAS_SHEETS)
    expect(screen.queryByRole('status', { name: 'Canvas sheet disclosure' })).not.toBeInTheDocument()
    expect(screen.queryByText('Waiting for Go page geometry.')).not.toBeInTheDocument()
  })

  // AC5, ASSERTED RATHER THAN ASSUMED. A template whose content column
  // occupies one window and whose content band holds nothing whose length
  // comes from data must render what it rendered at 2c5cfa1: the same DOM,
  // the same accessible names, the same command payload. The compile-only e2e
  // specs address these labels in Playwright STRICT MODE, where a duplicate
  // or a renamed label would be caught by nothing executable.
  it('renders a single-page template with the stable selection stack', async () => {
    const request = vi.fn(async () => ({ snapshot: snapshot(2) }))
    render(<App engine={engine(request)} initialSnapshot={snapshotOf({ ...canvas, components: [at('e1', 0)] })} />)
    expect(sheetLabels()).toEqual(['Report page with Page Header, Content, and Page Footer'])
    expect(screen.getByLabelText('Page Header')).toBeInTheDocument()
    expect(screen.getByLabelText('Content')).toBeInTheDocument()
    expect(screen.getByLabelText('Page Footer')).toBeInTheDocument()
    expect(screen.getByLabelText('text component e1')).toBeInTheDocument()
    // The coordinate stack remains; no seam, clipping window, echo, or disclosure.
    expect(document.querySelectorAll('.page-seam')).toHaveLength(0)
    expect(document.querySelectorAll('.sheet-stack')).toHaveLength(1)
    expect(document.querySelectorAll('.band-window')).toHaveLength(1)
    expect(document.querySelectorAll('.canvas-component-echo')).toHaveLength(0)
    expect(screen.queryByRole('status', { name: 'Canvas sheet disclosure' })).not.toBeInTheDocument()
    // And the payload, byte for byte.
    fireEvent.click(screen.getByRole('button', { name: 'Place Text' }))
    fireEvent.keyDown(screen.getByLabelText('Content'), { key: 'Enter' })
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(new TextDecoder().decode((request.mock.calls[0] as unknown as [string, ArrayBuffer])[1])).toBe('{"kind":"dropComponent","version":1,"type":"text","x":36,"y":56,"snap":true}')
  })
})

// RETIRED (Story 16.9): this file carried a ~560-line describe block,
// 'the font chain editor, where fonts are chosen', covering every control
// FontChainEditor.tsx drew — rename, delete, add/move/remove entry, the
// empty-document state, undo/redo interaction, and the disclosure button
// beside the family combobox that revealed it. `FontChainEditor.tsx` is
// deleted along with its render site, so none of those controls exist to
// assert any more. THE DATA THE EDITOR EDITED IS UNTOUCHED: a document's
// `fonts` map still carries its fallback chains exactly as before — Thai
// and CJK text still renders through them — and `engine-protocol.ts` still
// names the six chain commands the editor used to issue; only the UI that
// issued them is gone. Chain-command construction itself is still tested
// directly in `font-chain-command.test.ts`, independent of any UI.

// STORY 17.5: THE CONTENT BOX RESIZES FROM ITS BOTTOM EDGE.
//
// Eighteen tests. Eleven are the story's I/O matrix, one per row. The other
// seven are rows the matrix does not have. Two are about the timer this
// control sits on rather than about the drag: that a resize sends NO command
// (with a positive control that the same field's typing does), and that Story
// 17.1's debounce still fires afterwards. The other five came out of review,
// and four of them were measured defects rather than hypotheses: a RIGHT-BUTTON
// press started a resize (Chromium 1217: 122px after a 50px move, with the
// context menu then blocking the page); the drag carried no pointer id, so a
// second pointer rebased it; the anchor was recorded AFTER capture was
// requested, so a throw from `setPointerCapture` would have swallowed the
// press; a missed `pointerup` left the drag live, so a bare hover resized the
// box. The fifth closes a hole in the tests rather than in the code — pointer
// capture, which the frozen Boundaries require in terms, had NO executed
// coverage at all: deleting the production call reddened nothing.
//
// WHAT JSDOM CAN AND CANNOT SEE, STATED ONCE. There is no layout here and no
// stylesheet, so the drag's ARITHMETIC is testable — it is pure `clientY`
// against the press, and the height it produces is written as an INLINE style
// this file reads back — while the CURSOR and the ABSENT GRIP are only real in
// a browser. Two rows below are therefore asserted against the CSS SOURCE and
// say so. Worse, jsdom does not move focus on pointerdown AT ALL, which makes
// "the caret did not move" vacuous here: the falsifiable measurement of the
// `preventDefault()` that protects the caret is `fireEvent.pointerDown`'s
// RETURN VALUE, and the caret itself is proved in the browser run.
describe('the CONTENT box resizes from its bottom edge', () => {
  const paintOf = (text: string) => ({ overflow: false, truncated: false, lines: text === '' ? [] : [{ top: 0, baseline: 10_000, advance: 14_000, width: 30_000, fragments: [{ text, x: 0 }] }] })
  const box = (id: string, y: number, text: string) => ({ id, type: 'text' as const, band: 'content' as const, x: 0, y, width: 72_000, height: 24_000, resizable: true, value: text, textPaint: paintOf(text) })
  // TWO text components, because one of the matrix rows is what a change of
  // SELECTION does to an authored height, and that needs somewhere else to go.
  const at = (first: string) => ({ ...canvas, components: [box('e1', 0, first), box('e2', 40_000, 'Second')] })

  // This block's own engine double: `hold()` freezes it mid-command so a test
  // can drag ACROSS an in-flight commit. It echoes a canvas carrying the
  // accepted value, which is what makes "the panel still commits after a drag"
  // an assertion about the round trip rather than about the box alone.
  const resizeEngine = () => {
    const sent: string[] = []
    let held: (() => void) | undefined
    let holding = false
    let revision = 1
    let value = 'Hello'
    const answer = () => ({ snapshot: { documentState: 'loaded' as const, revision, byteLength: 3, canvas: at(value) } })
    const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
      if (operation !== 'command' || payload === undefined) return answer()
      const wire = new TextDecoder().decode(payload)
      sent.push(wire)
      if (holding) await new Promise<void>((resolve) => { held = resolve })
      const fields = (JSON.parse(wire) as { changes?: Record<string, { op: string; value: string }> }).changes
      if (fields?.value !== undefined) { value = fields.value.value; revision += 1 }
      return answer()
    })
    return { sent, request, hold: () => { holding = true }, release: () => { holding = false; held?.(); held = undefined } }
  }
  const elapse = async (ms = PROSE_COMMIT_DEBOUNCE_MS) => { await act(async () => { await vi.advanceTimersByTimeAsync(ms) }) }
  const settle = async () => { await act(async () => { await Promise.resolve(); await Promise.resolve(); await Promise.resolve() }) }
  const openEditor = (request: ReturnType<typeof resizeEngine>['request']) => {
    const view = render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at('Hello') }} />)
    // A painted component's accessible name CARRIES ITS TEXT, so the anchored
    // regex reaches it by the part of the name that does not move.
    fireEvent.click(screen.getByLabelText(/^text component e1/))
    const field = screen.getByRole('textbox', { name: 'Text' }) as HTMLTextAreaElement
    field.focus()
    return { view, field }
  }
  // The handle is `aria-hidden` — deliberately, see the keyboard row — so it
  // has no role and no name to be queried by, and the container is how every
  // other aria-hidden handle in this file is reached.
  const handleOf = (view: ReturnType<typeof render>) => view.container.querySelector('.property-prose-resize') as HTMLElement
  const type = (field: HTMLTextAreaElement, text: string) => fireEvent.change(field, { target: { value: text } })
  const changesOf = (wire: string) => (JSON.parse(wire) as { changes: Record<string, { op: string; value: string }> }).changes
  const appCss = () => fs.readFileSync('src/App.css', 'utf8')

  beforeEach(() => { vi.useFakeTimers() })
  afterEach(() => { vi.useRealTimers() })

  // MATRIX ROW 1. Hover the bottom edge -> the resize cursor.
  it('puts a full-width hit strip carrying ns-resize on the bottom edge of the box', async () => {
    const { view } = openEditor(resizeEngine().request)
    await elapse(0)
    const handle = handleOf(view)
    expect(handle).not.toBeNull()
    // The strip belongs to the BORDERED BOX the author sees, not to the
    // textarea inside its padding: a handle anywhere else is the original
    // complaint in a different place.
    expect(handle.parentElement?.className).toContain('property-field-prose')
    // The cursor itself is a stylesheet fact and jsdom applies no stylesheet,
    // so this is read from the SOURCE and photographed in the browser run.
    const rule = appCss().match(/^\.property-prose-resize \{[^}]*\}/m)
    expect(rule).not.toBeNull()
    expect(rule![0]).toMatch(/cursor:\s*ns-resize/)
    // FULL WIDTH, and on the edge this test's own name claims. Review found
    // this row asserting only the cursor and the two sides: the strip could
    // have been pinned to the TOP of the box, or given a 1px hit target, and
    // it stayed green.
    expect(rule![0]).toMatch(/left:\s*0/)
    expect(rule![0]).toMatch(/right:\s*0/)
    expect(rule![0]).toMatch(/bottom:\s*-?\d/)
    expect(rule![0]).not.toMatch(/\btop:/)
    const hit = /height:\s*(\d+)px/.exec(rule![0])
    expect(hit, 'the strip must declare a hit height').not.toBeNull()
    expect(Number(hit![1])).toBeGreaterThanOrEqual(6)
    expect(Number(hit![1])).toBeLessThanOrEqual(8)
  })

  // MATRIX ROW 2. Drag down 40px from the floor -> the box is 40px taller.
  it('grows the box by the pointer travel on a downward drag', async () => {
    const { view, field } = openEditor(resizeEngine().request)
    await elapse(0)
    const handle = handleOf(view)
    // The resting box carries NO inline height: it is sitting on the CSS
    // floor, which is what makes the arithmetic below start at 72 honestly.
    expect(field.style.height).toBe('')
    fireEvent.pointerDown(handle, { pointerId: 1, buttons: 1, clientY: 300 })
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: 340 })
    fireEvent.pointerUp(handle, { pointerId: 1, clientY: 340 })
    expect(field.style.height).toBe('112px')
    // And the text is still in it — the box grew, the value did not move.
    expect(field).toHaveValue('Hello')
  })

  // MATRIX ROW 3. Drag up past the floor -> clamped at 72px, not refused.
  it('clamps an upward drag at the 72px floor instead of refusing it', async () => {
    const { view, field } = openEditor(resizeEngine().request)
    await elapse(0)
    const handle = handleOf(view)
    fireEvent.pointerDown(handle, { pointerId: 1, buttons: 1, clientY: 300 })
    // Far past the floor, and then far past zero.
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: 240 })
    expect(field.style.height).toBe('72px')
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: -500 })
    expect(field.style.height).toBe('72px')
    // Clamped, not stuck: the same drag still grows again on the way back.
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: 320 })
    expect(field.style.height).toBe('92px')
    fireEvent.pointerUp(handle, { pointerId: 1, clientY: 320 })
    expect(field.style.height).toBe('92px')
  })

  // MATRIX ROW 4. Drag with a caret in the text -> caret and focus unmoved.
  it('presses the handle without taking the focus or the caret', async () => {
    const { view, field } = openEditor(resizeEngine().request)
    await elapse(0)
    type(field, 'Invoice for Ada')
    field.setSelectionRange(7, 7)
    const handle = handleOf(view)
    // THE REAL MEASUREMENT. `fireEvent` returns false when the handler called
    // `preventDefault()`, and that call is the whole protection: without it a
    // browser moves focus out of the textarea on pointerdown, and `blur` on
    // this field is a commit path.
    expect(fireEvent.pointerDown(handle, { pointerId: 1, buttons: 1, clientY: 300 })).toBe(false)
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: 350 })
    fireEvent.pointerUp(handle, { pointerId: 1, clientY: 350 })
    expect(field.style.height).toBe('122px')
    // These two are TRUE BUT WEAK HERE and are kept as the browser run's unit
    // shadow: jsdom does not move focus on pointerdown at all, so they would
    // pass with the `preventDefault()` deleted. The row above is what fails.
    expect(document.activeElement).toBe(field)
    expect(field.selectionStart).toBe(7)
  })

  // MATRIX ROW 5. Drag with an unflushed 17.1 debounce -> neither flushed nor
  // cancelled; it fires on its own schedule.
  it('leaves a pending debounce exactly where it was, to fire on its own clock', async () => {
    const { sent, request } = resizeEngine()
    const { view, field } = openEditor(request)
    await elapse(0)
    type(field, 'Half a clause')
    await elapse(PROSE_COMMIT_DEBOUNCE_MS / 2)
    expect(sent).toHaveLength(0)
    expect(vi.getTimerCount()).toBe(1)
    const handle = handleOf(view)
    fireEvent.pointerDown(handle, { pointerId: 1, buttons: 1, clientY: 300 })
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: 330 })
    fireEvent.pointerUp(handle, { pointerId: 1, clientY: 330 })
    // NOT FLUSHED: nothing went early. NOT CANCELLED: the timer is still armed.
    expect(sent).toHaveLength(0)
    expect(vi.getTimerCount()).toBe(1)
    expect(field.style.height).toBe('102px')
    // And it fires on the schedule the KEYSTROKE set, not one the drag reset.
    await elapse(PROSE_COMMIT_DEBOUNCE_MS / 2)
    await settle()
    expect(sent).toHaveLength(1)
    expect(changesOf(sent[0]!).value!.value).toBe('Half a clause')
  })

  // MATRIX ROW 6. Drag while a commit is IN FLIGHT -> the queued send drains.
  it('does not disturb a queued commit while a command is in flight', async () => {
    const { sent, request, hold, release } = resizeEngine()
    const { view, field } = openEditor(request)
    await elapse(0)
    hold()
    type(field, 'One')
    await elapse()
    await settle()
    // The first command is dispatched and stuck in the engine.
    expect(sent).toHaveLength(1)
    // More text across it: this one has nowhere to go but the queue.
    type(field, 'One two')
    await elapse()
    await settle()
    expect(sent).toHaveLength(1)
    const handle = handleOf(view)
    fireEvent.pointerDown(handle, { pointerId: 1, buttons: 1, clientY: 300 })
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: 336 })
    fireEvent.pointerUp(handle, { pointerId: 1, clientY: 336 })
    expect(field.style.height).toBe('108px')
    expect(sent).toHaveLength(1)
    // The drag touched neither the queue flag nor what the engine was told, so
    // the parked send still goes when the engine answers.
    release()
    await settle()
    expect(sent).toHaveLength(2)
    expect(changesOf(sent[1]!).value!.value).toBe('One two')
  })

  // MATRIX ROW 7. Release outside the panel -> pointercancel ends it too.
  it('ends the drag on pointercancel and leaves no drag state behind', async () => {
    const { view, field } = openEditor(resizeEngine().request)
    await elapse(0)
    const handle = handleOf(view)
    fireEvent.pointerDown(handle, { pointerId: 1, buttons: 1, clientY: 300 })
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: 350 })
    expect(field.style.height).toBe('122px')
    fireEvent.pointerCancel(handle, { pointerId: 1 })
    // A stuck drag would keep tracking the pointer forever. This one does not.
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: 900 })
    expect(field.style.height).toBe('122px')
    // Positive control, so "the move did nothing" is not "moves never do
    // anything": a fresh press resumes from the height the author left.
    fireEvent.pointerDown(handle, { pointerId: 2, buttons: 1, clientY: 100 })
    fireEvent.pointerMove(handle, { pointerId: 2, buttons: 1, clientY: 110 })
    expect(field.style.height).toBe('132px')
    fireEvent.pointerUp(handle, { pointerId: 2, clientY: 110 })
  })

  // MATRIX ROW 8. The native grip -> absent.
  it('turns the user agent grip off on the field that now draws its own', async () => {
    const { field } = openEditor(resizeEngine().request)
    await elapse(0)
    // jsdom paints nothing, so ABSENCE is asserted where the grip is declared.
    const scoped = appCss().match(/^textarea\.property-value-prose \{[^}]*\}/m)
    expect(scoped).not.toBeNull()
    expect(scoped![0]).toMatch(/resize:\s*none/)
    expect(scoped![0]).not.toMatch(/resize:\s*vertical/)
    // Non-vacuity for the rule above: the field on screen really does wear the
    // class that rule selects, and it really is a textarea.
    expect(field.tagName).toBe('TEXTAREA')
    expect(field.className).toContain('property-value-prose')
    // And nothing put the grip back inline.
    expect(field.style.resize).toBe('')
  })

  // MATRIX ROW 9. The font-family input -> unchanged height, no handle.
  it('gives the combobox that shares the class no handle and no height', async () => {
    const { view, field } = openEditor(resizeEngine().request)
    await elapse(0)
    const family = screen.getByRole('combobox', { name: 'Font family' })
    // The shared class is the whole hazard: both controls wear it.
    expect(family.className).toContain('property-value-prose')
    expect(field.className).toContain('property-value-prose')
    // Exactly one handle in the whole panel, and it is not in the family row.
    expect(view.container.querySelectorAll('.property-prose-resize')).toHaveLength(1)
    expect(family.closest('.property-editor')?.querySelector('.property-prose-resize')).toBeNull()
    expect((family as HTMLInputElement).style.height).toBe('')
    // Dragging the CONTENT box leaves the combobox exactly where it was.
    const handle = handleOf(view)
    fireEvent.pointerDown(handle, { pointerId: 1, buttons: 1, clientY: 300 })
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: 400 })
    fireEvent.pointerUp(handle, { pointerId: 1, clientY: 400 })
    expect(field.style.height).toBe('172px')
    expect((family as HTMLInputElement).style.height).toBe('')
  })

  // MATRIX ROW 10, FIRST HALF. Selection changes while tall -> back to the
  // floor. See the story's Design Notes: this is what the user agent's grip
  // did, because the grip's height was state on a DOM element that the
  // `documentGeneration:selection` key unmounts.
  it('starts each selection at the floor rather than inheriting a height', async () => {
    const { view, field } = openEditor(resizeEngine().request)
    await elapse(0)
    const handle = handleOf(view)
    fireEvent.pointerDown(handle, { pointerId: 1, buttons: 1, clientY: 300 })
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: 380 })
    fireEvent.pointerUp(handle, { pointerId: 1, clientY: 380 })
    expect(field.style.height).toBe('152px')
    fireEvent.click(screen.getByLabelText(/^text component e2/))
    await settle()
    const next = screen.getByRole('textbox', { name: 'Text' }) as HTMLTextAreaElement
    expect(next).toHaveValue('Second')
    expect(next.style.height).toBe('')
    // And the handle came back with the fresh row, so the affordance is not
    // what was lost.
    expect(handleOf(view)).not.toBeNull()
  })

  // MATRIX ROW 11. Keyboard only -> no regression, and no new tab stop.
  it('adds no keyboard tab stop, as the grip was not keyboard-reachable either', async () => {
    const { view, field } = openEditor(resizeEngine().request)
    await elapse(0)
    const handle = handleOf(view)
    // A <button> here would have added a tab stop to every text selection.
    expect(handle.tagName).toBe('SPAN')
    expect(handle.getAttribute('aria-hidden')).toBe('true')
    expect(handle.hasAttribute('tabindex')).toBe(false)
    expect(handle.tabIndex).toBe(-1)
    // Positive control for the line above, which would otherwise pass over any
    // element at all: the field the handle belongs to IS in the tab order.
    expect(field.tabIndex).toBe(0)
  })

  // NOT A MATRIX ROW. A GESTURE THAT IS NOT A DRAG MUST NOT START ONE, and
  // three of the four rows below were measured as live defects by review
  // rather than imagined: a right-press really did resize the box in Chromium
  // 1217, and the missing pointer id and the missing button-state check are
  // both reachable from ordinary input.
  it('ignores a press that is not the primary button', async () => {
    const { view, field } = openEditor(resizeEngine().request)
    await elapse(0)
    const handle = handleOf(view)
    // Measured before the guard: right-press then a 50px move set the box to
    // 122px, and the context menu then blocked the page mid-gesture.
    // `toBe(true)` is the second half of the assertion — the guard returns
    // BEFORE `preventDefault()`, so a right-press is left to the browser and
    // the context menu still opens.
    expect(fireEvent.pointerDown(handle, { pointerId: 1, button: 2, buttons: 2, clientY: 300 })).toBe(true)
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 2, clientY: 350 })
    fireEvent.pointerUp(handle, { pointerId: 1, button: 2, clientY: 350 })
    expect(field.style.height).toBe('')
    // Middle button, the other one an author reaches by accident.
    fireEvent.pointerDown(handle, { pointerId: 1, button: 1, buttons: 4, clientY: 300 })
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 4, clientY: 350 })
    expect(field.style.height).toBe('')
    // POSITIVE CONTROL, so the two zeros above are not a handle that never
    // works: the same gesture on the primary button resizes.
    fireEvent.pointerDown(handle, { pointerId: 1, button: 0, buttons: 1, clientY: 300 })
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: 350 })
    expect(field.style.height).toBe('122px')
    fireEvent.pointerUp(handle, { pointerId: 1, clientY: 350 })
  })

  it('ignores a second pointer, so nothing can rebase the gesture under it', async () => {
    const { view, field } = openEditor(resizeEngine().request)
    await elapse(0)
    const handle = handleOf(view)
    fireEvent.pointerDown(handle, { pointerId: 1, buttons: 1, clientY: 300 })
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: 340 })
    expect(field.style.height).toBe('112px')
    // A SECOND FINGER. Its press must not move the anchor, and its moves must
    // not be read as the first pointer's travel.
    fireEvent.pointerDown(handle, { pointerId: 2, buttons: 1, clientY: 500 })
    fireEvent.pointerMove(handle, { pointerId: 2, buttons: 1, clientY: 900 })
    expect(field.style.height).toBe('112px')
    // Nor may it END the first pointer's drag by lifting.
    fireEvent.pointerUp(handle, { pointerId: 2, clientY: 900 })
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: 360 })
    // Still measured from the ORIGINAL press at 300, not rebased to 500.
    expect(field.style.height).toBe('132px')
    fireEvent.pointerUp(handle, { pointerId: 1, clientY: 360 })
  })

  it('records the anchor before requesting capture, so a refused capture cannot swallow the press', async () => {
    const { view, field } = openEditor(resizeEngine().request)
    await elapse(0)
    const handle = handleOf(view)
    // `setPointerCapture` is SPECIFIED to throw NotFoundError for a pointerId
    // that is not active. Ordered before the anchor, such a throw unwinds past
    // the assignment and the author holds the edge while nothing moves —
    // silently, because an exception out of a React event handler is reported
    // rather than shown. The order is measured HERE, from inside the call
    // itself: this stub runs at exactly the moment capture is requested, and
    // the move it fires only does anything if the anchor is ALREADY recorded.
    handle.setPointerCapture = () => { fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: 320 }) }
    fireEvent.pointerDown(handle, { pointerId: 1, buttons: 1, clientY: 300 })
    expect(field.style.height).toBe('92px')
    fireEvent.pointerUp(handle, { pointerId: 1, clientY: 320 })
  })

  it('stops tracking when a move arrives with no button held', async () => {
    const { view, field } = openEditor(resizeEngine().request)
    await elapse(0)
    const handle = handleOf(view)
    fireEvent.pointerDown(handle, { pointerId: 1, buttons: 1, clientY: 300 })
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: 350 })
    expect(field.style.height).toBe('122px')
    // THE POINTERUP NEVER ARRIVES — capture lost to something outside this
    // component, the element taken out from under the gesture. Without the
    // button-state check the drag outlives the press, and the author's next
    // bare hover across the strip resizes the box with nothing held down.
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 0, clientY: 500 })
    expect(field.style.height).toBe('122px')
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 0, clientY: 900 })
    expect(field.style.height).toBe('122px')
    // Positive control: a real press still resizes from where it was left.
    fireEvent.pointerDown(handle, { pointerId: 1, buttons: 1, clientY: 300 })
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: 310 })
    expect(field.style.height).toBe('132px')
    fireEvent.pointerUp(handle, { pointerId: 1, clientY: 310 })
  })

  // THE BOUNDARIES REQUIRE CAPTURE IN TERMS — "so it survives leaving the
  // strip" — and jsdom leaves `setPointerCapture` undefined, which is why the
  // production call is optional-called and why deleting it reddened NOTHING
  // until this row existed. The double is the block's own jsdom-shadow idiom:
  // the browser run is where the surviving drag is actually observed.
  it('captures the pointer for the drag', async () => {
    const { view, field } = openEditor(resizeEngine().request)
    await elapse(0)
    const handle = handleOf(view)
    const capture = vi.fn()
    handle.setPointerCapture = capture
    fireEvent.pointerDown(handle, { pointerId: 7, buttons: 1, clientY: 300 })
    expect(capture).toHaveBeenCalledWith(7)
    // Non-vacuity: the press it was taken on is a press that really drags.
    fireEvent.pointerMove(handle, { pointerId: 7, buttons: 1, clientY: 330 })
    expect(field.style.height).toBe('102px')
    fireEvent.pointerUp(handle, { pointerId: 7, clientY: 330 })
    // And a press the guards refuse takes no capture either.
    capture.mockClear()
    fireEvent.pointerDown(handle, { pointerId: 8, button: 2, buttons: 2, clientY: 300 })
    expect(capture).not.toHaveBeenCalled()
  })

  // NOT A MATRIX ROW, AND THE PROPERTY THE STORY IS REALLY ABOUT: the height
  // is VIEW state. A drag sends no command, marks nothing dirty and changes no
  // saved byte.
  it('sends no command for a resize, while the same field still commits typing', async () => {
    const { sent, request } = resizeEngine()
    const { view, field } = openEditor(request)
    await elapse(0)
    const before = request.mock.calls.length
    const handle = handleOf(view)
    fireEvent.pointerDown(handle, { pointerId: 1, buttons: 1, clientY: 300 })
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: 320 })
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: 360 })
    fireEvent.pointerUp(handle, { pointerId: 1, clientY: 360 })
    // NO TIMER WAS ARMED AT ALL, which is the assertion that actually bites.
    // "Nothing was sent" alone is satisfied by a drag that DOES arm the 17.1
    // debounce, because `sendProseDraft` returns early when the draft still
    // equals what the engine was last told — measured, by adding
    // `scheduleProseCommit()` to the pointerdown handler and watching this
    // test stay green until this line existed.
    expect(vi.getTimerCount()).toBe(0)
    // Long past any debounce the drag could have armed.
    await elapse(PROSE_COMMIT_DEBOUNCE_MS * 5)
    await settle()
    expect(sent).toEqual([])
    // Not one round trip of ANY kind, so this is not merely "no command".
    expect(request.mock.calls).toHaveLength(before)
    expect(field.style.height).toBe('132px')
    // THE POSITIVE CONTROL, so the zero above is not a panel that never
    // commits: the same field, the same session, one keystroke.
    type(field, 'Now it sends')
    await elapse()
    await settle()
    expect(sent).toHaveLength(1)
    expect(changesOf(sent[0]!).value!.value).toBe('Now it sends')
  })

  // NOT A MATRIX ROW EITHER, AND THE OTHER HALF OF THE SELECTION PAIR. Half
  // one showed the height resetting on a selection change; on its own that is
  // not falsifiable, because a height that reset on ANY state change would
  // pass it. This is what makes it a property of the SELECTION key: the height
  // survives typing, a fired debounce and an accepted commit.
  it('keeps the height across typing and a committed debounce, which still fires', async () => {
    const { sent, request } = resizeEngine()
    const { view, field } = openEditor(request)
    await elapse(0)
    const handle = handleOf(view)
    fireEvent.pointerDown(handle, { pointerId: 1, buttons: 1, clientY: 300 })
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 1, clientY: 348 })
    fireEvent.pointerUp(handle, { pointerId: 1, clientY: 348 })
    expect(field.style.height).toBe('120px')
    type(field, 'Typed after the drag')
    await elapse()
    await settle()
    // Story 17.1's timer is untouched by the drag: it still fires and commits.
    expect(sent).toHaveLength(1)
    expect(changesOf(sent[0]!).value!.value).toBe('Typed after the drag')
    // And the canvas followed, so this is a real accepted round trip.
    expect(screen.getByLabelText(/^text component e1/)).toHaveAccessibleName(/Typed after the drag/)
    // The height did not evaporate: `documentGeneration` does not bump on a
    // property commit, so nothing remounted the row.
    expect(field.style.height).toBe('120px')
  })
})

// STORY 15.2a — THE DOCUMENT-ORIGINATED LEG, AND THE ONE THAT CARRIES THE
// SEVERITY.
//
// The typed-draft cases prove the encoder. Only this one proves the story's
// reachability claim, and it proves it through the gesture itself rather than
// by calling a factory: a sample-data file is accepted, its node is clicked,
// and Connect is pressed. No typing anywhere.
//
// A bind segment is a JSON object KEY taken verbatim out of that file
// (`sample-data.ts`'s DiscoveryParser), and nothing constrains what characters
// a JSON key may hold. Those keys used to travel through `component-command.ts`'s
// hand-rolled quoter, which iterated the value BY CODE POINT and then escaped
// from `charCodeAt(0)` — the high surrogate alone, with the low unit never
// emitted. The result parsed, so nothing anywhere reported an error, and the
// engine received an ADDRESS the author had not picked.
//
// Every escaping test that preceded this story used BMP-only inputs, which is
// exactly why the defect survived being read.
describe('a non-BMP key from a data file reaches the engine as the author\'s own code points', () => {
  const EMOJI = '\u{1F600}'
  const bound = (over: Record<string, unknown>) => ({ ...canvas, components: [{ id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello', ...over }] })

  it('carries the key through accept, click and Connect without mutilating it', async () => {
    const sent: string[] = []
    const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
      if (operation === 'command' && payload !== undefined) sent.push(new TextDecoder().decode(payload))
      return { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas: bound({}) } }
    })
    // THE FILE, not a hand-built segment list. The astral character is inside
    // the JSON KEY, which is the part nothing constrains.
    const data = acceptSampleData('keys.json', new TextEncoder().encode(`{"customer":{"na${EMOJI}me":"Ada"}}`).buffer)
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: bound({}) }} initialSampleData={data} />)
    fireEvent.click(screen.getByLabelText(/^text component e1/))
    fireEvent.click(screen.getByRole('tab', { name: 'DATA' }))
    const customer = screen.getAllByRole('treeitem').find((item) => item.getAttribute('aria-level') === '2' && item.textContent?.startsWith('customer'))
    expect(customer).toBeDefined()
    customer!.focus()
    fireEvent.keyDown(customer!, { key: 'ArrowRight' })
    const leaf = screen.getAllByRole('treeitem').find((item) => item.getAttribute('aria-level') === '3' && item.textContent?.startsWith(`na${EMOJI}me`))
    // Non-vacuity: if the tree never rendered the astral key, pressing Connect
    // below would bind nothing and every assertion after it would be about an
    // empty list.
    expect(leaf).toBeDefined()
    leaf!.focus()
    fireEvent.keyDown(leaf!, { key: 'Enter' })
    // STORY 14.6 — A PICK BINDS IMMEDIATELY (owner ruling). The Enter above IS
    // the bind; the intermediate "Connect selected path" control is gone.
    await waitFor(() => expect(sent).toHaveLength(1))

    const wire = sent[0] as string
    const command = JSON.parse(wire) as { kind: string; id: string; segments: string[] }
    expect(command.kind).toBe('bindComponentScalar')
    expect(command.segments).toEqual(['customer', `na${EMOJI}me`])
    // THE ROUND TRIP, asserted by CODE POINT. `toEqual` on the string alone
    // would also pass for a value that merely looks right in a diff.
    expect([...(command.segments[1] as string)]).toEqual(['n', 'a', EMOJI, 'm', 'e'])
    // The defect's own signature: the high surrogate emitted alone, with the
    // low unit dropped, which PARSES and so raised no error anywhere.
    expect(wire).not.toContain('\\ud83d')
    expect(wire).not.toContain('\\uD83D')
  })
})

// STORY 12.5: A BAND BOUNDARY IS DRAGGED ON THE CANVAS.
//
// The behavioural half of the story's I/O matrix — rows 1-5 and 8-16. Rows 6
// and 7 are the ENGINE's (folio8-go/band_height_command_test.go pins that
// snapping rounds and that snapping off does not); what is observable here is
// the field reaching the wire, which is the row this file adds beside them.
// Row 17 is byte identity, which lives in Go.
//
// WHAT THIS BLOCK IS ALSO FOR, and it is a gap this story measured rather than
// assumed: BEFORE IT, NO TEST IN THIS REPOSITORY ASSERTED THE RENDERED TEXT OF
// A CANVAS-GESTURE REFUSAL. `/usr/bin/grep -n 'commitError\|file-message'
// src/App.test.tsx` returned zero hits, with `points(` as the positive control.
// The story's whole safety argument is that a refusal reaches the author
// legibly, and nothing proved any canvas refusal was displayed at all.
//
// RED PROOFS (D-000.14), each run by mutating PRODUCTION code and never an
// expectation, and each recorded with the row it reddens:
//
//  1. DELETE THE FLOOR — band-boundary.ts's clamp low bound removed
//     (`Math.min(…, limit)`): 'stops the proposal at zero rather than showing a
//     negative band' FAILS here, and three rows of band-boundary.test.ts with
//     it. Nothing else in this block moves.
//  2. DELETE THE CEILING — the clamp's high bound replaced by
//     Number.POSITIVE_INFINITY: 'stops the proposal at the mirrored ceiling and
//     releases a legal height' FAILS.
//  3. DELETE SEND-ONLY-IF-CHANGED — `if (proposed === original) return` removed
//     from sendBandHeight: 'sends nothing when the pointer comes back to where
//     the drag began' FAILS, on a command sent for a gesture that changed
//     nothing — which in the running app is a history entry and a dirty mark
//     for a drag the author cancelled by hand.
//  4. DELETE THE stopPropagation — removed from nudgeBoundary: 'moves the
//     boundary and NOT a selected component when both could answer the key'
//     FAILS, because the window `shortcut` handler's SELECTION-driven arrow arm
//     fires as well and one key moves two things.
describe('Story 12.5: a band boundary is dragged on the canvas', () => {
  // The projected geometry every row below is measured against. header 20pt +
  // content 729.89pt + footer 20pt = 769.89pt of printable column, so the
  // mirrored ceiling for either capping band is 769.89 - 20 - 0.001 =
  // 749.889pt. Zoom is 1, so one pixel of pointer travel is one point.
  const ceiling = '749.889'
  const text = (id: string, band: 'pageHeader' | 'content' | 'pageFooter', y: number) => ({ id, type: 'text' as const, band, x: 0, y, width: 72_000, height: 12_000, resizable: true })
  const withComponents = (...components: ReadonlyArray<ReturnType<typeof text>>) => ({ ...canvas, components })
  const snapshotOf = (projection: CanvasProjection) => ({ documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: projection })
  // One uniform double type, so every row below can hand `open` its own
  // answer without the default's narrower inference fighting it.
  const boundaryEngine = (answer: () => Promise<unknown>) => vi.fn(answer)
  const open = (projection: CanvasProjection = canvas, request = boundaryEngine(async () => ({ snapshot: snapshot(2) }))) => {
    const view = render(<App engine={engine(request as never)} initialSnapshot={snapshotOf(projection)} />)
    return { view, request }
  }
  const sentCommands = (request: ReturnType<typeof boundaryEngine>) => (request.mock.calls as unknown as ReadonlyArray<[string, ArrayBuffer]>).map(([, payload]) => new TextDecoder().decode(payload))
  const headerHandle = () => screen.getByRole('button', { name: 'Resize the page header' })
  const footerHandle = () => screen.getByRole('button', { name: 'Resize the page footer' })
  const readout = () => document.querySelector('.band-boundary-readout')?.textContent
  const proposalOffset = () => (document.querySelector('.band-boundary-proposal') as HTMLElement | null)?.style.getPropertyValue('--boundary-display-y')
  // The band rects, read as the CSS custom properties the projection wrote.
  // AC: no `.page-band` moves for the duration of a gesture.
  const bandGeometry = () => Array.from(document.querySelectorAll('.page-band')).map((band) => `${(band as HTMLElement).style.getPropertyValue('--band-y')}/${(band as HTMLElement).style.getPropertyValue('--band-height')}`)
  const press = (handle: HTMLElement, clientY: number, pointerId = 1) => fireEvent.pointerDown(handle, { pointerId, button: 0, buttons: 1, clientY })
  const drag = (handle: HTMLElement, clientY: number, pointerId = 1) => fireEvent.pointerMove(handle, { pointerId, buttons: 1, clientY })
  const release = (handle: HTMLElement, clientY: number, pointerId = 1) => fireEvent.pointerUp(handle, { pointerId, buttons: 0, clientY })

  // MATRIX ROW 1.
  it('tracks the pointer with a proposal and sends one pageHeader command on release', async () => {
    const { request } = open()
    const handle = headerHandle()
    const resting = bandGeometry()
    press(handle, 100)
    drag(handle, 140)
    // The proposal, and only the proposal: the readout is the height the gesture
    // PROPOSES — not a prediction of what will be written, because with Snap on
    // the engine rounds it — and the line sits 40pt below the boundary's own
    // place. Snap is off in no row here, so the two differ; the wire assertion
    // below is what pins the proposal, and Go pins the rounding.
    expect(readout()).toBe('60')
    expect(proposalOffset()).toBe('40px')
    // AC. No band rect was re-placed by the browser, and nothing was sent.
    expect(bandGeometry()).toEqual(resting)
    expect(request).not.toHaveBeenCalled()
    release(handle, 140)
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(sentCommands(request)[0]).toBe('{"kind":"setBandHeight","version":1,"band":"pageHeader","height":60,"snap":true}')
    // The proposal is gone: what the author sees after release is what the
    // engine accepted, re-projected.
    expect(document.querySelector('.band-boundary-proposal')).toBeNull()
  })

  // MATRIX ROW 2. The footer grows as the pointer RISES, which is the half a
  // suite that only dragged the header would never see.
  it('grows the page footer on an upward drag and sends one pageFooter command', async () => {
    const { request } = open()
    const handle = footerHandle()
    press(handle, 400)
    drag(handle, 375)
    expect(readout()).toBe('45')
    expect(proposalOffset()).toBe('-25px')
    release(handle, 375)
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(sentCommands(request)[0]).toBe('{"kind":"setBandHeight","version":1,"band":"pageFooter","height":45,"snap":true}')
  })

  // MATRIX ROW 3. The edge above the PAGE HEADER tab is the page margin, not a
  // band boundary, so there is nothing there to grab.
  it('gives the page header band no handle, because its top edge is the page margin', () => {
    open()
    expect(document.querySelectorAll('.band-boundary-handle')).toHaveLength(2)
    expect(document.querySelector('.page-band-pageHeader .band-boundary-handle')).toBeNull()
    expect(document.querySelector('.page-band-content .band-boundary-handle')).not.toBeNull()
    expect(document.querySelector('.page-band-pageFooter .band-boundary-handle')).not.toBeNull()
    // AC: no accessible name collides with the three region names the shipped
    // e2e specs reach for under Playwright strict mode.
    expect(screen.getByLabelText('Page Header')).toBeInTheDocument()
    expect(screen.getByLabelText('Content')).toBeInTheDocument()
    expect(screen.getByLabelText('Page Footer')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Page Header' })).toBeNull()
  })

  // MATRIX ROW 4. The send-only-if-changed rule, which is also what keeps a
  // no-op gesture out of undo history.
  it('sends nothing when the pointer comes back to where the drag began', async () => {
    const { request } = open()
    const handle = headerHandle()
    press(handle, 100)
    drag(handle, 160)
    expect(readout()).toBe('80')
    drag(handle, 100)
    expect(readout()).toBe('20')
    release(handle, 100)
    await act(async () => { await Promise.resolve() })
    expect(request).not.toHaveBeenCalled()
  })

  // MATRIX ROW 5. Under the 2px travel gate this is a press, not a drag — and
  // it must not LOOK like one either. A line and a readout painted for a
  // gesture that has already decided to send nothing is the canvas showing the
  // author a proposal it will discard without telling them.
  it('treats a press with sub-threshold travel as a press rather than a drag, and paints nothing', async () => {
    const { request } = open()
    const handle = headerHandle()
    press(handle, 100)
    // The bare press: no travel at all, and nothing on the canvas.
    expect(document.querySelector('.band-boundary-proposal')).toBeNull()
    expect(document.querySelector('.band-boundary-readout')).toBeNull()
    drag(handle, 101)
    expect(document.querySelector('.band-boundary-proposal')).toBeNull()
    expect(document.querySelector('.band-boundary-readout')).toBeNull()
    release(handle, 101)
    await act(async () => { await Promise.resolve() })
    expect(request).not.toHaveBeenCalled()
    // NON-VACUITY: two more pixels and the same gesture DOES paint, so the
    // absences above are the threshold and not a proposal that never renders.
    press(handle, 200)
    drag(handle, 203)
    expect(document.querySelector('.band-boundary-proposal')).not.toBeNull()
  })

  // MATRIX ROW 8. A negative band height is a property of the FIELD and is
  // never proposed, so the line stops at the band's own origin.
  it('stops the proposal at zero rather than showing a negative band', () => {
    open()
    const handle = headerHandle()
    press(handle, 100)
    drag(handle, 0)
    expect(readout()).toBe('0')
    expect(proposalOffset()).toBe('-20px')
  })

  // MATRIX ROW 9. The mirrored content-window ceiling. The pointer runs on; the
  // boundary does not, and the value released is legal.
  it('stops the proposal at the mirrored ceiling and releases a legal height', async () => {
    const { request } = open()
    const handle = headerHandle()
    press(handle, 100)
    drag(handle, 1000)
    expect(readout()).toBe(ceiling)
    // Further travel moves nothing: the clamp is a stop, not a scaling.
    drag(handle, 4000)
    expect(readout()).toBe(ceiling)
    release(handle, 4000)
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(sentCommands(request)[0]).toBe(`{"kind":"setBandHeight","version":1,"band":"pageHeader","height":${ceiling},"snap":true}`)
  })

  // MATRIX ROW 10, AND THE FIRST ASSERTION IN THIS REPOSITORY THAT A CANVAS
  // GESTURE'S REFUSAL IS RENDERED AT ALL. The engine refuses a band shortened
  // past a component's lowest edge, with its own located sentence; the whole
  // safety argument of this story is that the author reads it.
  it('renders the engine\'s own refusal sentence in the canvas alert', async () => {
    const refusing = boundaryEngine(async () => { throw { elementId: 'e1', dataPath: 'bands.pageHeader.height', message: 'a pageHeader height of 12pt would leave e1 outside the band: it reaches 62pt' } })
    const { request } = open(withComponents(text('e1', 'pageHeader', 50_000)), refusing)
    const handle = headerHandle()
    press(handle, 100)
    drag(handle, 92)
    release(handle, 92)
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('e1: a pageHeader height of 12pt would leave e1 outside the band: it reaches 62pt')
    expect(alert.className).toContain('file-message')
    // The proposal is discarded either way: a refused gesture must not leave a
    // line on the canvas claiming a height the document does not have.
    expect(document.querySelector('.band-boundary-proposal')).toBeNull()
  })

  // MATRIX ROW 11.
  it('resizes the focused boundary by a point on an arrow key and ten with shift', async () => {
    const { request } = open()
    const handle = headerHandle()
    handle.focus()
    fireEvent.keyDown(handle, { key: 'ArrowDown' })
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(sentCommands(request)[0]).toBe('{"kind":"setBandHeight","version":1,"band":"pageHeader","height":21,"snap":true}')
    fireEvent.keyDown(handle, { key: 'ArrowUp', shiftKey: true })
    await waitFor(() => expect(request).toHaveBeenCalledTimes(2))
    expect(sentCommands(request)[1]).toBe('{"kind":"setBandHeight","version":1,"band":"pageHeader","height":10,"snap":true}')
    // The FOOTER's arrows are the other direction of the same key, because the
    // footer's height is measured upward.
    fireEvent.keyDown(footerHandle(), { key: 'ArrowUp' })
    await waitFor(() => expect(request).toHaveBeenCalledTimes(3))
    expect(sentCommands(request)[2]).toBe('{"kind":"setBandHeight","version":1,"band":"pageFooter","height":21,"snap":true}')
  })

  // MATRIX ROW 12. The window `shortcut` handler's arrow arm is SELECTION
  // driven, not focus driven, so without the handle's own stopPropagation both
  // would fire and the author would move two things with one key.
  it('moves the boundary and NOT a selected component when both could answer the key', async () => {
    // The double echoes a projection that STILL CARRIES e1, because the
    // contrast arm below needs a component to be there after the boundary
    // command has landed — a snapshot with no components would make the second
    // arrow key send nothing for a reason that has nothing to do with focus.
    const projection = withComponents(text('e1', 'content', 0))
    const { request } = open(projection, boundaryEngine(async () => ({ snapshot: snapshotOf(projection) })))
    fireEvent.pointerDown(screen.getByLabelText('text component e1'), { pointerId: 3, clientX: 1, clientY: 1 })
    fireEvent.pointerUp(screen.getByLabelText('text component e1'), { pointerId: 3, clientX: 1, clientY: 1 })
    // The resize handles are the selection, rendered.
    expect(screen.getByLabelText('Resize e1')).toBeInTheDocument()
    const handle = headerHandle()
    handle.focus()
    fireEvent.keyDown(handle, { key: 'ArrowDown' })
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    const sent = sentCommands(request)
    expect(sent[0]).toContain('"kind":"setBandHeight"')
    expect(sent.some((command) => command.includes('moveComponent'))).toBe(false)
    // The contrast arm: with nothing of ours focused the same key DOES move the
    // component, so the row above is a measurement of the guard rather than of
    // a nudge path that never fires.
    fireEvent.keyDown(window, { key: 'ArrowDown' })
    await waitFor(() => expect(request).toHaveBeenCalledTimes(2))
    expect(sentCommands(request)[1]).toContain('"kind":"moveComponent"')

    // AND THE HORIZONTAL ARROWS, which the boundary has no use for and must
    // still not hand on: they reach the same SELECTION-driven arm and would
    // move the component sideways while focus sat on the strip.
    handle.focus()
    fireEvent.keyDown(handle, { key: 'ArrowLeft' })
    fireEvent.keyDown(handle, { key: 'ArrowRight' })
    await act(async () => { await Promise.resolve() })
    expect(request).toHaveBeenCalledTimes(2)

    // A MODIFIER IS THE APPLICATION'S. Undo must still reach the window
    // handler from a focused handle, or the strip becomes a keyboard trap for
    // every shortcut in the product.
    const shortcut = vi.fn()
    window.addEventListener('keydown', shortcut)
    fireEvent.keyDown(handle, { key: 'z', ctrlKey: true })
    expect(shortcut).toHaveBeenCalledOnce()
    window.removeEventListener('keydown', shortcut)
  })

  // P4's other half: Enter and Space on a focused handle reach the band
  // <section>'s own onKeyDown, which treats them as a PLACEMENT and drops an
  // armed palette component at the band origin.
  it('swallows Enter and Space rather than placing an armed palette component', async () => {
    const { request } = open()
    fireEvent.click(screen.getByRole('button', { name: 'Place Text' }))
    const handle = headerHandle()
    handle.focus()
    fireEvent.keyDown(handle, { key: 'Enter' })
    fireEvent.keyDown(handle, { key: ' ' })
    await act(async () => { await Promise.resolve() })
    expect(request).not.toHaveBeenCalled()
    // NON-VACUITY: the same key on the BAND does place, so the row above
    // measures the handle's guard and not a placement path that never fires.
    fireEvent.keyDown(screen.getByLabelText('Content'), { key: 'Enter' })
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(sentCommands(request)[0]).toContain('"kind":"dropComponent"')
  })

  // P5. The strip spans the whole page width plus 118px and lies over the band,
  // so with a palette kind armed a placement press anywhere along the boundary
  // would start a resize. Every other band pointer handler is gated on
  // `placing`; this one is too.
  it('begins no drag while a palette placement is armed', async () => {
    const { request } = open()
    fireEvent.click(screen.getByRole('button', { name: 'Place Text' }))
    const handle = headerHandle()
    press(handle, 100)
    drag(handle, 160)
    expect(document.querySelector('.band-boundary-proposal')).toBeNull()
    release(handle, 160)
    await act(async () => { await Promise.resolve() })
    expect(request).not.toHaveBeenCalled()
  })

  // P3. Escape is the established cancel for every other canvas interaction,
  // and a boundary drag that survived it would leave a live gesture and a line
  // on screen with nothing to end them.
  it('abandons a live drag on Escape, and on the canvas-wide clear', () => {
    open()
    const handle = headerHandle()
    press(handle, 100)
    drag(handle, 160)
    expect(document.querySelector('.band-boundary-proposal')).not.toBeNull()
    fireEvent.keyDown(handle, { key: 'Escape' })
    expect(document.querySelector('.band-boundary-proposal')).toBeNull()
    // The same abort reached through clearInteraction, which is what a document
    // replaced mid-drag (undo, load, Start blank) runs.
    press(handle, 100)
    drag(handle, 160)
    expect(document.querySelector('.band-boundary-proposal')).not.toBeNull()
    fireEvent.keyDown(screen.getByLabelText('Canvas region'), { key: 'Escape' })
    expect(document.querySelector('.band-boundary-proposal')).toBeNull()
  })

  // P12. A pointerup can carry the pointer somewhere new without an intervening
  // move — a fast flick, a capture handed back, a coalesced sequence — and the
  // release must commit where the author let go, not where the last move was.
  it('commits the coordinate the pointer was released at, not the last move\'s', async () => {
    const { request } = open()
    const handle = headerHandle()
    press(handle, 100)
    drag(handle, 140)
    expect(readout()).toBe('60')
    release(handle, 190)
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(sentCommands(request)[0]).toBe('{"kind":"setBandHeight","version":1,"band":"pageHeader","height":110,"snap":true}')
  })

  // P2 (HIGH). THE ONE CONVERSION THE BROWSER OWNS IN THIS FEATURE, at the only
  // zoom where it can go wrong. Every other row runs at zoom 1, where
  // `documentDelta(px, 1) * 1000` is an integer by luck; the ladder's other
  // rungs hand `points()` a float, and `points()` assumes whole millipoints.
  //
  // MEASURED: at zoom 1.1 a 36px upward drag on a 60pt header gives
  // 27273.000000000004, which `points()` spells "27.273.00000000000364" — not a
  // JSON number, so `jsonNumber` replaces it with `null` and the command goes
  // out as `"height":null`. The author reads that string in the readout for the
  // whole gesture and then the engine refuses the command.
  it('keeps the proposal a whole millipoint at a zoom other than 1', async () => {
    // A 60pt header, because the artifact needs a proposal that does not land
    // on the floor: 60 - 32.727 = 27.273.
    const tall = { ...canvas, bands: [{ name: 'pageHeader' as const, x: 36_000, y: 36_000, width: 523_276, height: 60_000 }, { name: 'content' as const, x: 36_000, y: 96_000, width: 523_276, height: 689_890 }, { name: 'pageFooter' as const, x: 36_000, y: 785_890, width: 523_276, height: 20_000 }] }
    const { request } = open(tall)
    fireEvent.click(screen.getByRole('button', { name: 'Zoom in' }))
    expect(screen.getByLabelText('Canvas zoom')).toHaveTextContent('110%')
    const handle = headerHandle()
    press(handle, 100)
    drag(handle, 64)
    // The readout the author actually reads. Without the rounding this is
    // "27.273.00000000000364".
    expect(readout()).toBe('27.273')
    // AND IT IS THE ZOOM'S OWN ARITHMETIC: at zoom 1 the same 36px would be
    // 36pt and the header would read 24. A conversion that ignored zoom would
    // pass every other row in this block.
    expect(readout()).not.toBe('24')
    release(handle, 64)
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(sentCommands(request)[0]).toBe('{"kind":"setBandHeight","version":1,"band":"pageHeader","height":27.273,"snap":true}')
    expect(sentCommands(request)[0]).not.toContain('null')
  })

  // MATRIX ROW 13. One NAMED boundary per DOCUMENT, on the home sheet,
  // following the occurrence.home idiom the repeating components already use.
  // Every other sheet draws an aria-hidden, untabbable handle that still drags.
  it('names exactly two handles for a three-sheet stack, both on the home sheet, and hides the rest', () => {
    open({ ...canvas, contentWindowCount: 3, contentWindowOrigins: [0, 700_000, 1_400_000], contentWindowPages: [0, 0, 0] })
    expect(document.querySelectorAll('.page-surface')).toHaveLength(3)
    expect(document.querySelectorAll('.page-band-pageFooter')).toHaveLength(3)
    // getByRole throws on more than one match, so these two calls ARE the
    // uniqueness claim.
    expect(headerHandle()).toBeInTheDocument()
    expect(footerHandle()).toBeInTheDocument()
    const home = document.querySelectorAll('.page-surface')[0] as HTMLElement
    expect(home.contains(headerHandle()) && home.contains(footerHandle())).toBe(true)
    const handles = Array.from(document.querySelectorAll('.band-boundary-handle'))
    expect(handles).toHaveLength(6)
    const hidden = handles.filter((handle) => !home.contains(handle))
    expect(hidden).toHaveLength(4)
    expect(hidden.every((handle) => handle.getAttribute('aria-hidden') === 'true' && handle.getAttribute('tabindex') === '-1' && !handle.hasAttribute('aria-label'))).toBe(true)
  })

  // SPEC-multi-pages: the header and footer heights are shared, so they resize
  // from any sheet, with today's command, and the proposal shows on every sheet.
  it('resizes the page header from a later sheet with the same command', async () => {
    const { request } = open({ ...canvas, contentWindowCount: 2, contentWindowOrigins: [0, 0], contentWindowPages: [0, 1], pageBreaks: [true, true] })
    const later = document.querySelectorAll('.page-surface')[1]!.querySelector('.page-band-content .band-boundary-handle') as HTMLElement
    expect(later).toHaveAttribute('aria-hidden', 'true')
    press(later, 100)
    drag(later, 106)
    expect(document.querySelectorAll('.band-boundary-proposal')).toHaveLength(2)
    release(later, 106)
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    // No page, no sheet: the one shared height, exactly as from the home sheet.
    expect(sentCommands(request)[0]).toMatch(/^\{"kind":"setBandHeight","version":1,"band":"pageHeader","height":[0-9.]+,"snap":true\}$/)
  })

  // MATRIX ROW 14. A right-button press fires pointerdown like any other, and
  // without the guard a context-menu click on the strip starts a drag.
  it('begins no drag on a non-primary button', async () => {
    const { request } = open()
    const handle = headerHandle()
    fireEvent.pointerDown(handle, { pointerId: 1, button: 2, buttons: 2, clientY: 100 })
    drag(handle, 160)
    expect(document.querySelector('.band-boundary-proposal')).toBeNull()
    release(handle, 160)
    await act(async () => { await Promise.resolve() })
    expect(request).not.toHaveBeenCalled()
  })

  // MATRIX ROW 15, both endings. A pointercancel and a move with nothing held
  // down are the same ending: the proposal is discarded and nothing is sent.
  it('discards the proposal when the pointer is cancelled or the button is released unseen', async () => {
    const { request } = open()
    const handle = headerHandle()
    press(handle, 100)
    drag(handle, 160)
    expect(readout()).toBe('80')
    fireEvent.pointerCancel(handle, { pointerId: 1, clientY: 160 })
    expect(document.querySelector('.band-boundary-proposal')).toBeNull()
    await act(async () => { await Promise.resolve() })
    expect(request).not.toHaveBeenCalled()
    // The other ending: a move with buttons === 0 is a HOVER, not a drag, and
    // it must not resize anything on the next bare pass across the strip.
    press(handle, 100)
    drag(handle, 160)
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 0, clientY: 200 })
    expect(document.querySelector('.band-boundary-proposal')).toBeNull()
    fireEvent.pointerMove(handle, { pointerId: 1, buttons: 0, clientY: 400 })
    expect(document.querySelector('.band-boundary-proposal')).toBeNull()
    release(handle, 400)
    await act(async () => { await Promise.resolve() })
    expect(request).not.toHaveBeenCalled()
  })

  // MATRIX ROW 16. Without the id check a second finger's press REBASES the
  // anchor under the first one's drag, and a second finger's move is accepted
  // as if the first had made it.
  it('lets the first pointer own the gesture and ignores a second', async () => {
    const { request } = open()
    const handle = headerHandle()
    press(handle, 100)
    drag(handle, 140)
    expect(readout()).toBe('60')
    press(handle, 300, 2)
    drag(handle, 340, 2)
    expect(readout()).toBe('60')
    release(handle, 340, 2)
    await act(async () => { await Promise.resolve() })
    expect(request).not.toHaveBeenCalled()
    // The FIRST pointer still owns it, and still commits its own travel.
    drag(handle, 150)
    expect(readout()).toBe('70')
    release(handle, 150)
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(sentCommands(request)[0]).toContain('"height":70')
  })

  // THE AFFORDANCE, WHICH IS A STYLESHEET FACT AND SO IS READ FROM THE SOURCE.
  // jsdom applies no stylesheet, and the shipped Playwright suite is not run by
  // any workflow (DW-193), so App.css's own text is the only place these are
  // observable by anything that executes.
  //
  // COMMENT POLICY, DECLARED AND ASSERTED IN BOTH DIRECTIONS (D-000.27): these
  // extractions are RAW — a commented-out rule counts as live — which is the
  // policy property-prose-height.test.ts already uses over this same file. The
  // extraction FAILS LOUDLY rather than passing vacuously when it finds
  // nothing, because a guard that silently matched zero rules is quoted as
  // evidence exactly like one that matched.
  const appCss = () => fs.readFileSync('src/App.css', 'utf8')
  const ruleFor = (source: string, selector: string) => source.match(new RegExp(`^\\${selector} \\{([^}]*)\\}`, 'm'))

  it('declares the hit strip as a transparent ns-resize band on the boundary, and nothing that paints', () => {
    const css = appCss()
    const handle = ruleFor(css, '.band-boundary-handle')
    expect(handle, 'App.css must declare a .band-boundary-handle rule').not.toBeNull()
    // AN ALLOWLIST, WHICH IS THE ONLY WAY "IT PAINTS NOTHING" IS REAL. Review
    // forced this move once already on .property-prose-resize, after a
    // two-item denylist let `border-top`, `box-shadow` and `outline` through.
    const declared = handle![1].split(';').map((one) => one.trim()).filter((one) => one !== '').map((one) => one.slice(0, one.indexOf(':')).trim())
    expect([...declared].sort()).toEqual(['background', 'border', 'cursor', 'height', 'left', 'padding', 'position', 'top', 'touch-action', 'width'].sort())
    expect(handle![0]).toMatch(/background:\s*transparent/)
    expect(handle![0]).toMatch(/border:\s*0/)
    expect(handle![0]).toMatch(/cursor:\s*ns-resize/)
    // Load-bearing: without it a touch press is taken as a pan, the browser
    // fires pointercancel, and the drag ends before it starts.
    expect(handle![0]).toMatch(/touch-action:\s*none/)
    // A hit height in the 6-8px band, straddling the rule rather than sitting
    // wholly above or below it.
    const hit = /height:\s*(\d+)px/.exec(handle![0])
    expect(hit, 'the strip must declare a hit height').not.toBeNull()
    expect(Number(hit![1])).toBeGreaterThanOrEqual(6)
    expect(Number(hit![1])).toBeLessThanOrEqual(8)
    expect(handle![0]).toMatch(/top:\s*-\d/)
    // A generated box would paint without appearing in the rule at all.
    expect(css).not.toMatch(/\.band-boundary-handle\s*::?(?:after|before)/)
  })

  // P1 (HIGH). THE DEFECT THIS ROW EXISTS FOR, and it shipped invisible because
  // jsdom applies no stylesheet: App.css's band-tab rule is `.page-band > span`
  // at specificity (0,1,1), and a class rule is (0,1,0). The proposal and the
  // readout are DIRECT CHILDREN of the band, so as <span>s the TAB rule won —
  // `top: 0` beating `top: var(--boundary-display-y)`, plus a border, a tint, a
  // translate a tab's width off the page and text-transform: uppercase. The
  // line would never have tracked the pointer in a real browser.
  //
  // They are <div>s now. This row pins BOTH halves: the rules say what they
  // must say, and no `.page-band > …` selector the sheet declares can reach
  // them — gathered FROM THE STYLESHEET rather than listed here, so a child
  // rule added later is covered without anyone remembering to come back.
  it('paints the proposal and the readout out of reach of the band-tab rule', () => {
    const css = appCss()
    for (const selector of ['.band-boundary-proposal', '.band-boundary-readout']) {
      const rule = ruleFor(css, selector)
      expect(rule, `App.css must declare a ${selector} rule`).not.toBeNull()
      expect(rule![0]).toMatch(/top:\s*var\(--boundary-display-y\)/)
      expect(rule![0]).toMatch(/pointer-events:\s*none/)
    }
    open()
    const handle = headerHandle()
    press(handle, 100)
    drag(handle, 140)
    const painted = ['.band-boundary-proposal', '.band-boundary-readout'].map((selector) => document.querySelector(selector) as HTMLElement | null)
    expect(painted.filter((node) => node !== null)).toHaveLength(2)
    const childSelectors = [...css.matchAll(/^(\.page-band > [^{,]+?) \{/gm)].map((match) => (match[1] as string).trim())
    expect(childSelectors, 'App.css must declare at least one .page-band child rule for this row to be about anything').not.toHaveLength(0)
    for (const selector of childSelectors) {
      for (const node of painted) expect(node!.matches(selector), `${selector} out-specificities the class rule and must not match .${node!.className}`).toBe(false)
    }
    // POSITIVE CONTROL. The same gathered selectors really do reach the band
    // tab, so "does not match" above is a measurement rather than a regex that
    // matched nothing. The tab is untouched by this story and must stay so.
    const tab = document.querySelector('.page-band-content > span') as HTMLElement
    expect(tab, 'the band tab must still be the band section\'s first span child').not.toBeNull()
    expect(tab.textContent).toBe('Content')
    expect(childSelectors.some((selector) => tab.matches(selector))).toBe(true)
  })

  // THE DESIGN RECORD IS FOUND, NOT SPELLED. Hard-coding `ux-folio-2026-08-23`
  // would red a unit test on the next UX revision directory, so the revision is
  // discovered and its uniqueness asserted — 0 or 2 is a THROW, never
  // first-match-wins (the shape offline-release-contract.mjs already uses).
  const designSource = () => {
    const root = '../_bmad-output/planning-artifacts/ux-designs'
    const revisions = fs.readdirSync(root).filter((entry) => fs.existsSync(`${root}/${entry}/DESIGN.md`))
    if (revisions.length !== 1) throw new Error(`expected exactly one UX revision carrying a DESIGN.md under ${root}; found ${revisions.length}: ${revisions.join(', ')}`)
    return fs.readFileSync(`${root}/${revisions[0]}/DESIGN.md`, 'utf8')
  }
  // EACH NUMERAL IS READ FROM ITS OWN BLOCK. An unscoped `/^\s+overhang:/m`
  // takes the first match anywhere in a 500-line document, so a stray
  // `overhang:` added above would silently retarget the width assertion below.
  // `band-tab:` genuinely appears twice — once under typography and once under
  // components — which is why the block is selected by the KEY it carries and
  // a count other than one throws.
  const designPixels = (source: string, block: string, key: string): string => {
    const declaration = new RegExp(`^    ${key}: '(\\d+)px'$`, 'm')
    const holding = [...source.matchAll(new RegExp(`^  ${block}:\\n((?:    \\S.*\\n)+)`, 'gm'))]
      .map((match) => match[1] as string)
      .filter((body) => declaration.test(body))
    if (holding.length !== 1) throw new Error(`DESIGN.md must declare exactly one ${block} block carrying ${key}; found ${holding.length}`)
    return (declaration.exec(holding[0] as string) as RegExpExecArray)[1] as string
  }

  it('reaches the band tab, at the offset DESIGN.md declares for it', () => {
    // UX-DR25's "hit targets larger than their visual footprint", tied to the
    // design record rather than to a number somebody liked: 104px is
    // band-tab.offsetFromPage and 14px is band-boundary.overhang, so the strip
    // spans the tab's horizontal position AND ends exactly where the dashed
    // rule the author sees ends. 104 + 14 = 118.
    const design = designSource()
    const tabOffset = designPixels(design, 'band-tab', 'offsetFromPage')
    const overhang = designPixels(design, 'band-boundary', 'overhang')
    // BOTH numerals pinned, not one: an unpinned overhang lets the width
    // assertion follow whatever the document happens to say.
    expect(tabOffset).toBe('104')
    expect(overhang).toBe('14')
    const handle = ruleFor(appCss(), '.band-boundary-handle')
    expect(handle, 'App.css must declare a .band-boundary-handle rule').not.toBeNull()
    expect(handle![0]).toContain(`left: calc(-1 * var(--band-x) - ${tabOffset}px)`)
    expect(handle![0]).toContain(`width: calc(var(--page-display-width) + ${Number(tabOffset) + Number(overhang)}px)`)
    // The extraction's own non-vacuity, in both directions and on synthetic
    // input: it finds the block that carries the key, and it THROWS rather than
    // guessing when no block or two blocks do.
    expect(designPixels("  band-tab:\n    offsetFromPage: '7px'\n  band-tab:\n    fontSize: '9px'\n", 'band-tab', 'offsetFromPage')).toBe('7')
    expect(() => designPixels("  band-tab:\n    fontSize: '9px'\n", 'band-tab', 'offsetFromPage')).toThrow(/exactly one/)
    expect(() => designPixels("  band-tab:\n    offsetFromPage: '7px'\n  band-tab:\n    offsetFromPage: '8px'\n", 'band-tab', 'offsetFromPage')).toThrow(/exactly one/)
  })

  // AC2's affordance distinction, and the extraction's own non-vacuity: the
  // ONLY ns-resize the canvas grows is on the two real boundaries.
  it('gives the page header band no resize cursor, and the extraction proves it can see one', () => {
    const css = appCss()
    expect(css).not.toMatch(/\.page-band-pageHeader[^{]*\{[^}]*ns-resize/)
    // POSITIVE CONTROL. The same shape of match DOES fire on the selector that
    // really carries the cursor, so the absence above is a measurement.
    expect(css).toMatch(/\.band-boundary-handle[^{]*\{[^}]*ns-resize/)
    // AND THE COMMENT POLICY, ASSERTED IN BOTH DIRECTIONS on synthetic input
    // rather than by editing the file. RAW: a rule commented OUT at the start
    // of a line is still seen, so nobody buys their way past this guard by
    // wrapping the rule in `/* */`.
    expect(ruleFor('/*\n.band-boundary-handle { cursor: ns-resize; }\n*/\n', '.band-boundary-handle')).not.toBeNull()
    expect(ruleFor('.band-boundary-handle { cursor: ns-resize; }\n', '.band-boundary-handle')).not.toBeNull()
    // The other direction, and the extraction's OWN limit stated rather than
    // discovered later: it is line-anchored, so a rule that does not begin its
    // line is invisible to it — including one tucked after an inline comment.
    // That is why the allowlist row above fails loudly when it finds nothing.
    expect(ruleFor('/* x */ .band-boundary-handle { cursor: ns-resize; }\n', '.band-boundary-handle')).toBeNull()
    expect(ruleFor('.page-band > span { top: 0; }\n', '.band-boundary-handle')).toBeNull()
  })

  // MATRIX ROWS 6 AND 7, in the only place the browser can answer them: the
  // engine snaps, and what this side owes is the field. Go's
  // TestSetBandHeightSnapsToTheGridWhenAsked and
  // TestSetBandHeightLeavesAnUnsnappedHeightAlone are the other end of it.
  it('carries the canvas Snap toggle to the wire rather than rounding for itself', async () => {
    const { request } = open()
    fireEvent.click(screen.getByRole('button', { name: /^Snap on/ }))
    const handle = headerHandle()
    press(handle, 100)
    drag(handle, 141)
    // 61 is not on the 6pt grid, and nothing in the browser moved it.
    expect(readout()).toBe('61')
    release(handle, 141)
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(sentCommands(request)[0]).toBe('{"kind":"setBandHeight","version":1,"band":"pageHeader","height":61,"snap":false}')
  })

  // P8. THE READOUT SHOWS THE PROPOSAL, NOT A PREDICTION OF WHAT WILL BE
  // WRITTEN — and this row is what makes that a fact rather than a comment.
  // With Snap ON the engine rounds 61 to the 6pt grid, and the browser must
  // still show 61 and send 61: rounding the DISPLAY would mean a second
  // spelling of SnapNearest in TypeScript, which is the one thing AD-17 and
  // this story's R1 forbid outright. The accepted value reaches the author by
  // re-projection on release, not by the browser guessing it.
  it('shows the raw proposal under Snap-on rather than re-spelling the engine\'s grid', async () => {
    const { request } = open()
    expect(screen.getByRole('button', { name: /^Snap on/ })).toHaveAttribute('aria-pressed', 'true')
    const handle = headerHandle()
    press(handle, 100)
    drag(handle, 141)
    // 61 is one point off the grid. A browser that snapped the display would
    // show 60 here, and would be re-implementing the engine's rule to do it.
    expect(readout()).toBe('61')
    expect(readout()).not.toBe('60')
    release(handle, 141)
    await waitFor(() => expect(request).toHaveBeenCalledOnce())
    expect(sentCommands(request)[0]).toBe('{"kind":"setBandHeight","version":1,"band":"pageHeader","height":61,"snap":true}')
  })

  // P10. THE BLOCK'S OWN PLACEMENT, because the defect it closes was invisible
  // to every guard in the file: the 12.5 block was first spliced between Story
  // 17.1's header line and 17.1's body, so 17.1's documentation read as if it
  // described 12.5 — and 17.1's self-count guard, which counts `it(`s from its
  // describe to EOF, passed the whole time.
  //
  // EVERY MARKER BELOW IS MATCHED LINE-ANCHORED AND SPELLED AS A REGEX, never
  // as a plain literal, and that is not style. A bare `indexOf("describe('Story
  // 17" + ".1")` in THIS block would occur earlier in the file than the real
  // one, so the row would slice its own source and pass on nothing — and worse,
  // it would move Story 17.1's own self-count guard, which does exactly that
  // indexOf, onto this block's text. (Measured: it did. That guard went red
  // with `expected undefined to be defined` the moment this row was first
  // written with literals.) Line anchors exclude both, because a marker quoted
  // inside an expect is indented and a real one is at column 0.
  it('leaves each story header whole and adjacent to its own describe', () => {
    const source = fs.readFileSync('src/App.test.tsx', 'utf8')
    const at = (pattern: RegExp, what: string) => {
      const found = source.search(pattern)
      if (found < 0) throw new Error(`App.test.tsx no longer carries ${what} at the start of a line; re-derive this extraction rather than deleting the check`)
      return found
    }
    const header = at(/^\/\/ STORY 17\.1: THE CANVAS FOLLOWS THE CONTENT FIELD\.$/m, "Story 17.1's header")
    const owner = at(/^describe\('Story 17\.1/m, "Story 17.1's describe")
    expect(owner).toBeGreaterThan(header)
    // Nothing else may sit between a header and the describe it introduces.
    expect(source.slice(header, owner)).not.toMatch(/^describe\(/m)
    expect(source.slice(header, owner)).not.toMatch(/^\/\/ STORY 12\.5:/m)
    // And this block's own header is whole, immediately above its own describe.
    const mine = at(/^\/\/ STORY 12\.5: A BAND BOUNDARY IS DRAGGED ON THE CANVAS\.$/m, "Story 12.5's header")
    const myOwner = at(/^describe\('Story 12\.5/m, "Story 12.5's describe")
    expect(myOwner).toBeGreaterThan(mine)
    expect(source.slice(mine, myOwner)).not.toMatch(/^describe\(/m)
    expect(source.slice(mine, myOwner)).not.toMatch(/^\/\/ STORY 17\.1:/m)
    // The whole of this block sits before that header, which is the property
    // the splice violated.
    expect(myOwner).toBeLessThan(header)
  })
})


// STORY 11.3 — THE CANVAS PAINTS THE WEIGHT THE ENGINE RESOLVED, AND THE PANEL
// STATES A CUT THE FAMILY DOES NOT HAVE.
//
// Two defects and one gap, all on the same surface:
//
//   * The canvas set `--text-font-weight` from `component.bold` — THE REQUESTED
//     FLAG — and App.css fed it to `font-weight`. That is browser emboldening
//     (I-2, AD-17), and since Story 11.2 it was applied ON TOP OF the real bold
//     cut the engine had already resolved and named on the fragment.
//   * NOTHING ASSERTED THE TWO CUSTOM PROPERTIES. Measured before this story:
//     `--text-font-weight` and `--text-font-style` were written at exactly one
//     site and read at exactly one, and no test named either — so deleting them
//     reddened nothing and the deletion was invisible to the suite. The sibling
//     `--text-line-baseline` IS asserted (`:1433`), and `--text-ink` (`:2293`),
//     both in this file with the idiom below, which is what makes that a
//     coverage hole rather than a search artefact. The first test here is the
//     positive assertion the deletion needed.
//   * The B / I controls had no way to say a family has no such face, because
//     the projection did not carry the chain's declared variants (DW-239).
//
// THE ABSENCE STATE IS READ THROUGH THREE HANDLES, and each carries a different
// half of the claim: the button's `property-toggle-unavailable` class (the only
// handle jsdom has on a stylesheet it never parses), its `aria-describedby`
// (the ONE announcement path — the sentence is not folded into the accessible
// name as well), and the visible paragraph that id points at (the wording, and
// the stated way out). A test that read only the class would pass over a state
// that says nothing.
describe('the resolved weight, painted and stated', () => {
  const boldPaint = { overflow: false, truncated: false, lines: [{ top: 0, baseline: 10_000, advance: 12_000, width: 30_000, fragments: [{ text: 'Heading', x: 0, face: 'Roboto Bold Italic' }] }] }
  const robotoChain = { name: 'Roboto', entries: [face('Roboto', { bold: 'Roboto Bold', italic: 'Roboto Italic', boldItalic: 'Roboto Bold Italic' }), face('Noto Sans Thai', { bold: 'Noto Sans Thai Bold' }), face('Noto Sans SC')] }
  const cjkChain = { name: 'CJK', entries: [face('Noto Sans SC')] }
  // ⚠ THE FIXTURE'S FACE IS `Roboto Bold Italic`, NOT `Roboto Bold`. It carried
  // the latter beside `bold: true, italic: true`, which is a resolution the
  // engine cannot produce: `fontStyleOf(true, true)` asks the chain for
  // `boldItalic`, and this chain declares one. A fixture encoding an impossible
  // engine answer teaches the wrong model to the next reader even while every
  // assertion over it passes.
  const bolded = { id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Heading', fontFamily: 'Roboto', fontSize: 12_000, bold: true, italic: true, textPaint: boldPaint }
  const projection = (components: CanvasProjection['components'], chains: CanvasProjection['fontChains'] = [robotoChain, cjkChain]): CanvasProjection => ({ ...canvas, fontFamilies: chains.map((chain) => chain.name), fontChains: chains, components })
  const mount = (components: CanvasProjection['components'], chains?: CanvasProjection['fontChains']) => {
    render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: projection(components, chains) }} />)
    fireEvent.click(screen.getByLabelText(/^text component e1/))
  }
  // The three handles, read together, so no test can assert a state that half
  // exists. `undefined` asserts the PLAIN control: no class, no description.
  const expectCut = (label: 'Bold' | 'Italic', sentence: string | undefined) => {
    const control = screen.getByRole('button', { name: label })
    if (sentence === undefined) {
      expect(control.className, `${label} must be the plain control`).not.toContain('property-toggle-unavailable')
      expect(control.getAttribute('aria-describedby'), `${label} must describe no absence`).toBeNull()
      return control
    }
    expect(control.className, `${label} must be in the unavailable state`).toContain('property-toggle-unavailable')
    // NOT DISABLED, EVER. A disabled control is the shape CHECKPOINT 1 refused:
    // it renders as on-or-off, and it would make a declared flag unclearable.
    expect(control).not.toBeDisabled()
    const described = control.getAttribute('aria-describedby')
    expect(described, `${label} must point at the reason`).not.toBeNull()
    expect(document.getElementById(described!)).toHaveTextContent(sentence)
    return control
  }
  const NO_BOLD = 'No bold face in this family — the engine paints the regular face and warns.'
  const NO_ITALIC = 'No italic face in this family — the engine paints the regular face and warns.'
  const NO_BOLD_ITALIC = 'No bold italic face in this family — it cannot do both at once. Turn off either one.'

  it('paints the face the engine resolved and applies no weight or slope of its own', () => {
    render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: projection([bolded]) }} />)
    const paint = screen.getByLabelText('text component e1: Heading').querySelector('.canvas-text-paint') as HTMLElement
    // POSITIVE CONTROL FIRST, so the three absences below are a measurement of
    // this node and not of a node that was never found. `--text-font-size` is
    // set on the SAME element by the SAME expression, and it stays: a size is
    // not a weight, and it is the engine's own.
    expect(paint).toHaveStyle({ '--text-font-size': '12px' })
    // THE DELETION, ASSERTED. Neither custom property, and neither property
    // they fed — a bold, italic component whose flags are both set.
    expect(paint.style.getPropertyValue('--text-font-weight')).toBe('')
    expect(paint.style.getPropertyValue('--text-font-style')).toBe('')
    expect(paint.style.fontWeight).toBe('')
    expect(paint.style.fontStyle).toBe('')
    // AND THE WEIGHT ARRIVES AS A FACE INSTEAD. `Roboto Bold Italic` is the face
    // the ENGINE resolved and named on the fragment; the browser asks for it by
    // name and synthesises nothing on top of it.
    const fragment = paint.querySelector('.canvas-text-fragment') as HTMLElement
    // Quotes normalised, because jsdom's CSSOM re-serialises the stack with
    // double quotes. The EXPECTED side is still the derivation rather than a
    // literal, so a change to the fallback stack reds here too.
    expect(fragment.style.fontFamily.replaceAll('"', '\'')).toBe(shippedFaceFamily('Roboto Bold Italic'))
    expect(fragment.style.fontWeight).toBe('')
    expect(fragment.style.fontStyle).toBe('')
  })

  // I/O MATRIX ROW: "bold element, NO variant declared". The engine's answer is
  // the entry's own BASE face plus a Warning, and the canvas must take that
  // answer rather than compensating for it — which is precisely what the
  // deleted `font-weight: 700` was doing. Every other absent-cut test here sets
  // `bold: false` and reads the CONTROL; this one sets `bold: true` and reads
  // the FRAGMENT, which is the half of the row nothing else covers.
  it('paints a bold element in the BASE face when the chain declares no bold cut', () => {
    const regularPaint = { overflow: false, truncated: false, lines: [{ top: 0, baseline: 10_000, advance: 12_000, width: 30_000, fragments: [{ text: 'Heading', x: 0, face: 'Noto Sans SC' }] }] }
    const flat = { ...bolded, fontFamily: 'CJK', italic: false, textPaint: regularPaint }
    render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: projection([flat]) }} />)
    const paint = screen.getByLabelText('text component e1: Heading').querySelector('.canvas-text-paint') as HTMLElement
    const fragment = paint.querySelector('.canvas-text-fragment') as HTMLElement
    // THE BASE FACE, and nothing added to it. Canvas and PDF agree because both
    // took the engine's answer; the canvas does not thicken what the engine
    // declined to thicken.
    expect(fragment.style.fontFamily.replaceAll('"', '\'')).toBe(shippedFaceFamily('Noto Sans SC'))
    expect(paint.style.getPropertyValue('--text-font-weight')).toBe('')
    expect(paint.style.fontWeight).toBe('')
    expect(fragment.style.fontWeight).toBe('')
  })

  // ⚠ THE EMBEDDED ARM, AND A REGRESSION THIS STORY WOULD OTHERWISE INTRODUCE.
  //
  // `carriedFaceKeys` read only `entry.assetKey`, which was complete while an
  // entry named ONE face. Since Story 11.2 an entry may name four, and for
  // `{"asset": K1, "bold": K2}` the ENGINE resolves a bold run to K2 and puts
  // K2 on the fragment. With K2 unfetched, `carriedFaces.has(K2)` is false,
  // `fragment.face` is empty on that arm, and the fragment gets NO `fontFamily`
  // at all — it falls to the stylesheet's stack while the document's own bold
  // bytes sit in its `assets` map. Before this story that fragment at least got
  // `font-weight: 700`, so deleting the synthetic weight without this fix makes
  // embedded bold STRICTLY WORSE (the D-11.3.1 shape: a compensation removed
  // without supplying what it compensated for).
  it('fetches an embedded entry\'s VARIANT asset key and paints the fragment with it', async () => {
    const base = '1111111111111111111111111111111111111111111111111111111111111111'
    const variant = '2222222222222222222222222222222222222222222222222222222222222222'
    const fontSet = installStubFontSet()
    try {
      const embeddedPaint = { overflow: false, truncated: false, lines: [{ top: 0, baseline: 12_000, advance: 16_000, width: 24_000, fragments: [{ text: 'สัญญา', x: 0, assetKey: variant }] }] }
      const component = { id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'ignored', bold: true, fontFamily: 'body', textPaint: embeddedPaint }
      const embedded = { name: 'body', entries: [carried(base, { bold: variant })] }
      const requested: string[] = []
      const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
        if (operation === 'asset') { requested.push(new TextDecoder().decode(payload)); return { snapshot: snapshot(1), bytes: new Uint8Array([0, 1, 2, 3]).buffer } }
        return { snapshot: snapshot(1) }
      })
      const view = render(<App engine={engine(request)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: projection([component], [embedded]) }} />)
      // BOTH KEYS ARE FETCHED. The base is the entry's discriminant and the
      // variant is a sibling of the same kind (AD-8), so both are `assets` keys
      // and both are faces this document may be painted with.
      await waitFor(() => expect(requested).toHaveLength(2))
      expect(requested.some((payload) => payload.includes(base)), 'the entry\'s own asset key').toBe(true)
      expect(requested.some((payload) => payload.includes(variant)), 'the entry\'s BOLD variant asset key').toBe(true)
      // AND THE FRAGMENT THE ENGINE ATTRIBUTED TO THE VARIANT IS PAINTED WITH
      // IT. This is the assertion the fetch exists for: a fetch with no family
      // on the node would be a request nobody uses.
      const painted = () => Array.from(view.container.querySelectorAll('.canvas-text-fragment')) as HTMLElement[]
      await waitFor(() => expect(painted().map((node) => node.style.fontFamily)).toEqual([embeddedFaceFamily(variant)]))
      // AND NOTHING SYNTHETIC ON TOP OF IT, which is the whole story on this arm.
      expect(painted()[0]!.style.fontWeight).toBe('')
      expect(painted()[0]!.style.fontStyle).toBe('')
    } finally {
      fontSet.restore()
    }
  })

  it('leaves the B and I controls plain when the chain declares the cut', () => {
    mount([bolded])
    expect(screen.getByRole('button', { name: 'Bold' })).toHaveAttribute('aria-pressed', 'true')
    expectCut('Bold', undefined)
    expectCut('Italic', undefined)
    expect(screen.queryByText(/No .* face in this family/)).toBeNull()
  })

  it('states the absent cut beside the control, on a chain no entry of which declares one', () => {
    mount([{ ...bolded, fontFamily: 'CJK', bold: false, italic: false }])
    // TWO DIFFERENT MISSING CUTS ARE TWO DIFFERENT FACTS, so they state two
    // sentences — the "once for the pair" rule is about ONE cut implicating
    // both controls, not about collapsing unrelated absences.
    expectCut('Bold', NO_BOLD)
    expectCut('Italic', NO_ITALIC)
    expect(screen.getByText(NO_BOLD)).toBeInTheDocument()
    expect(screen.getByText(NO_ITALIC)).toBeInTheDocument()
  })

  it('states an absent italic while the bold beside it stays plain', () => {
    const thai = { name: 'Thai', entries: [face('Noto Sans Thai', { bold: 'Noto Sans Thai Bold' })] }
    mount([{ ...bolded, fontFamily: 'Thai', bold: false, italic: false }], [thai])
    // Noto Sans Thai ships a Bold and upstream publishes no italic at all
    // (fonts.go), so the two controls must disagree — and a rule that answered
    // "the chain declares SOMETHING" rather than "the chain declares THIS cut"
    // would leave both plain.
    expectCut('Bold', undefined)
    expectCut('Italic', NO_ITALIC)
  })

  // ⚠ F1 — THE COMBINED CUT, the row this spec's I/O matrix never enumerated.
  //
  // `boldItalic` was projected across the whole new seam and read by nothing.
  // An element with BOTH flags set, on a chain declaring `bold` and `italic`
  // but not `boldItalic`, resolves to the base face and warns — while both
  // controls read plainly on. That is the state AC3 exists to prevent.
  //
  // THE FIXTURE'S DISCRIMINATING POWER IS PINNED FIRST, because a chain missing
  // only the combined cut is the ONE shape that tells the generalised predicate
  // from the per-axis one: on a chain missing `bold` outright both predicates
  // agree, and an assertion whose two sides could be equal is not an assertion
  // (D-11.2.8).
  it('enters the unavailable state for the COMBINED cut, and states it once for the pair', () => {
    const noCombined = { name: 'Pair', entries: [face('Roboto', { bold: 'Roboto Bold', italic: 'Roboto Italic' })] }
    expect(noCombined.entries[0]!.bold, 'the chain must DECLARE a bold, or this fixture cannot tell the combined rule from the per-axis one').not.toBe('')
    expect(noCombined.entries[0]!.italic, 'and an italic, for the same reason').not.toBe('')
    expect(noCombined.entries[0]!.boldItalic, 'and must be missing only the COMBINED cut').toBe('')
    mount([{ ...bolded, fontFamily: 'Pair' }], [noCombined])
    // BOTH controls are implicated: marking only one implies the other is fine.
    expectCut('Bold', NO_BOLD_ITALIC)
    expectCut('Italic', NO_BOLD_ITALIC)
    // ⚠ AND THE SENTENCE NAMES THE MISSING CUT, NOT THE CONTROL. The predicate
    // alone — without this — makes B say "No bold face in this family" on a
    // chain that DECLARES a bold. A panel that lies precisely is worse than one
    // that lies vaguely, so the false sentence is asserted absent by name.
    expect(screen.queryByText(NO_BOLD)).toBeNull()
    expect(screen.queryByText(NO_ITALIC)).toBeNull()
    // STATED ONCE FOR THE PAIR, through ONE announcement path: both buttons
    // point at the SAME element, and there is exactly one of it.
    expect(screen.getAllByText(NO_BOLD_ITALIC)).toHaveLength(1)
    expect(screen.getByRole('button', { name: 'Bold' }).getAttribute('aria-describedby'))
      .toBe(screen.getByRole('button', { name: 'Italic' }).getAttribute('aria-describedby'))
    // AND THE SENTENCE IS NOT ALSO FOLDED INTO THE ACCESSIBLE NAME (P9): the
    // plain names are what resolve, so nothing is announced twice.
    expect(screen.getByRole('button', { name: 'Bold' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Italic' })).toBeInTheDocument()
  })

  // THE SAME CHAIN, ONE FLAG OFF. The combined cut is only required once BOTH
  // are on, so B — whose press would produce plain bold — must be plain, while
  // I, whose press would produce the combined cut, must warn BEFORE the press.
  it('warns on the control whose press would need the missing combined cut, and not on the other', () => {
    const noCombined = { name: 'Pair', entries: [face('Roboto', { bold: 'Roboto Bold', italic: 'Roboto Italic' })] }
    mount([{ ...bolded, fontFamily: 'Pair', italic: false }], [noCombined])
    expectCut('Bold', undefined)
    expectCut('Italic', NO_BOLD_ITALIC)
    expect(screen.getAllByText(NO_BOLD_ITALIC)).toHaveLength(1)
  })

  // ⚠ THE `entries[0]` TRAP, AND THE ONE FIXTURE THAT CAN SEE IT.
  //
  // `declaredChainEntry` returns the chain's FIRST entry and sits three
  // functions from the call site, so it is the function an implementer reaches
  // for — and it is the wrong rule here (Q2, ratified at CHECKPOINT 1). The
  // starter's own chain CANNOT detect the error: its first entry is Roboto,
  // which declares a bold, so the first-entry rule and the all-entries rule
  // return the same answer on it. An assertion whose two sides could be equal
  // is not an assertion (D-11.2.8), so this fixture is built the other way
  // round — first entry with no bold, later entry with one — and the two rules
  // are pinned to genuinely DISAGREE on it before the control is read.
  it('asks every entry of the chain, not the first one', () => {
    const cjkFirst = { name: 'Mixed', entries: [face('Noto Sans SC'), face('Roboto', { bold: 'Roboto Bold' })] }
    expect(cjkFirst.entries[0]!.bold, 'the FIRST entry must declare no bold, or this fixture cannot tell the two rules apart').toBe('')
    expect(cjkFirst.entries.some((entry) => entry.bold.length > 0), 'a LATER entry must declare one, for the same reason').toBe(true)
    mount([{ ...bolded, fontFamily: 'Mixed', bold: false, italic: false }], [cjkFirst])
    // The all-entries rule says AVAILABLE; the first-entry rule would say
    // "no bold face in this family" while Latin bolds perfectly well.
    expectCut('Bold', undefined)
    // Italic is the control arm: NO entry of this chain declares one, so the
    // same fixture must produce the absent state for the other cut. Without it
    // this test would also pass over a rule that never reports an absence.
    expectCut('Italic', NO_ITALIC)
  })

  // THE SELECTION RULE IS `every`, AND THE DELIBERATE CHOICE IS PINNED. Swapping
  // it to `some` reddened nothing: every other test here selects ONE component.
  // A mixed selection in which one component's chain declares the cut must keep
  // the plain control, because "this family has no bold face" would be false of
  // half of it.
  it('keeps the control plain when only SOME of the selection lacks the cut', () => {
    const canBold = { ...bolded, id: 'e1', bold: false, italic: false, fontFamily: 'Roboto' }
    const cannot = { ...bolded, id: 'e2', y: 30_000, bold: false, italic: false, fontFamily: 'CJK' }
    render(<App engine={engine()} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: projection([canBold, cannot]) }} />)
    fireEvent.click(screen.getByLabelText(/^text component e1/))
    fireEvent.click(screen.getByLabelText(/^text component e2/), { shiftKey: true })
    // Non-vacuity: the selection really is two components.
    expect(screen.getByText('2 selected')).toBeInTheDocument()
    expectCut('Bold', undefined)
    expectCut('Italic', undefined)
    // And the SAME cut-less component alone still states it, so the green above
    // is the `every` rule and not a panel that never reports an absence.
    cleanup()
    mount([{ ...cannot, id: 'e1', y: 0 }])
    expectCut('Bold', NO_BOLD)
  })

  // ⚠ THE DECLARED-BUT-UNAVAILABLE STATE, DRIVEN BY ITS REAL ROUTE. Bold a
  // Roboto element, then switch its family to a CJK-only chain — both through
  // the panel's own controls, with the engine answering each one. A state only
  // reachable through a hand-built projection is a state nobody has shown is
  // reachable, and this is the state that decided Q1 against a disabled
  // control: the document carries `bold: true` and the author must be able to
  // clear it.
  it('keeps a declared-but-unavailable cut clearable after the family moves under it', async () => {
    const plain = { ...bolded, bold: false, italic: false }
    const sent: string[] = []
    let current: CanvasProjection = projection([plain])
    const request = vi.fn(async (_operation: string, payload?: ArrayBuffer) => {
      if (payload) {
        const text = new TextDecoder().decode(payload)
        sent.push(text)
        if (text.includes('"bold":{"op":"set"')) current = projection([{ ...plain, bold: true }])
        if (text.includes('"fontFamily"')) current = projection([{ ...plain, bold: true, fontFamily: 'CJK' }])
      }
      return { snapshot: { documentState: 'loaded' as const, revision: 1 + sent.length, byteLength: 3, canvas: current } }
    })
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: current }} />)
    fireEvent.click(screen.getByLabelText('text component e1: Heading'))
    fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Bold' })).toHaveAttribute('aria-pressed', 'true'))
    // NOW THE FAMILY MOVES, through the same dropdown an author uses.
    fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
    fireEvent.click(within(screen.getByRole('group', { name: 'IN THIS TEMPLATE' })).getByRole('option', { name: 'CJK' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Bold' }).className).toContain('property-toggle-unavailable'))
    const unavailable = expectCut('Bold', NO_BOLD)
    // THE DOCUMENT STILL CARRIES THE FLAG, so the control is still pressed —
    // and it is the third state, not the plain on state.
    expect(unavailable).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(unavailable)
    await waitFor(() => expect(sent.at(-1)).toContain('"bold":{"op":"clear"}'))
  })
})

// STORY 13.1 — THE PREVIEW KEEPS THE PDF.
//
// The engine stub below returns a LITERAL byte fixture and reports the SHA-256
// of that same literal. Every byte claim in this block is pinned to the fixture
// and to the digest, never read back out of `preview.bytes` — an assertion whose
// two sides come from the same place is not an assertion (D-11.2.8).
//
// The fixture is thirty-three bytes: a high byte, a NUL, a CR/LF pair and a run
// that is not a prefix of itself, so a truncation or a text re-encode changes it.
const exportedPdfBytes = new Uint8Array([37, 80, 68, 70, 45, 49, 46, 55, 10, 37, 226, 227, 207, 211, 10, 0, 255, 128, 1, 254, 200, 17, 42, 7, 240, 13, 10, 37, 37, 69, 79, 70, 10])
const exportedPdfDigest = '0ed9ca9f2a8912227c9bb42a9183e81e89a553e40619734cee78d79d6bfb7e7a'
// A SECOND, DISTINCT fixture for the mid-save replacement arm below. It shares
// no prefix with the first past `%PDF-`, so a write that picked up the newer
// preview cannot pass a truncation-insensitive comparison.
const replacementPdfBytes = new Uint8Array([37, 80, 68, 70, 45, 50, 46, 48, 10, 37, 200, 199, 198, 197, 10, 0, 1, 2, 253, 254, 255, 90, 91, 92, 13, 10, 37, 37, 69, 79, 70, 10, 7])
const replacementPdfDigest = '16693a98e884070a1122a0aa3d91ca2a7a5aded352e26d0f8b715431b404289d'

// Every render hands back a FRESH copy of the fixture, the way the real worker
// boundary does, so nothing downstream can pass a byte claim by sharing an
// object identity with the source.
const previewRequest = () => vi.fn(async (operation: string) => {
  // Answered so the parameter panel is settled rather than showing its own
  // `role="alert"`: the alert claims below are about the PDF save alone.
  if (operation === 'parameter-references') return { snapshot: snapshot(1), parameterReferences: { revision: 1, names: [] } }
  if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
  if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
  if (operation === 'render') return { snapshot: snapshot(1), bytes: exportedPdfBytes.slice().buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: exportedPdfDigest, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
  return { snapshot: snapshot(1) }
})

// A native tier over a fake picker, recording every buffer any handle received.
const nativeSaveTier = () => {
  const written: number[][] = []
  const writingHandle = (name: string) => ({ name, getFile: async () => new File([exportedPdfBytes], name), createWritable: async () => ({ write: async (buffer: ArrayBuffer) => { written.push([...new Uint8Array(buffer)]) }, close: async () => undefined }) })
  const picked = writingHandle('statement.pdf')
  const showSaveFilePicker = vi.fn(async () => picked)
  const showOpenFilePicker = vi.fn(async () => [picked])
  return { written, picked, showSaveFilePicker, showOpenFilePicker, access: new FileSystemAccess({ showOpenFilePicker, showSaveFilePicker }) }
}

// A download tier over a fake document, capturing the one blob it creates.
const downloadSaveTier = () => {
  const anchor = { href: '', download: '', style: { display: '' }, click: vi.fn(), remove: vi.fn() }
  const blobs: Blob[] = []
  const fakeDocument = { body: { append: vi.fn() }, createElement: vi.fn(() => anchor) } as unknown as Document
  const url = { createObjectURL: vi.fn((blob: Blob) => { blobs.push(blob); return 'blob:local' }), revokeObjectURL: vi.fn() }
  return { anchor, blobs, access: new InputDownloadAccess(fakeDocument, url) }
}

const showRenderedPreview = async (request: ReturnType<typeof previewRequest>, fileAccess?: FileAccess, admit = true) => {
  render(<App engine={engine(request)} fileAccess={fileAccess} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
  fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
  await waitFor(() => expect(screen.getByRole('button', { name: /Stale historical PDF/ })).toBeInTheDocument())
  if (admit) {
    fireEvent.click(screen.getByRole('button', { name: /Stale historical PDF/ }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Current exact local production PDF/ })).toBeInTheDocument())
  }
}

describe('Story 13.1: the preview keeps the PDF', () => {
  it('writes the engine bytes verbatim through the native tier, asking the engine for nothing and keeping every document field', async () => {
    const tier = nativeSaveTier()
    const request = previewRequest()
    await showRenderedPreview(request, tier.access)
    const engineCallsBeforeThePress = request.mock.calls.length
    fireEvent.click(screen.getByRole('button', { name: 'Save PDF' }))
    await waitFor(() => expect(tier.written).toHaveLength(1))
    // THE PICKER IS OFFERED A PDF NAME AND A PDF-ONLY FILTER — all three of the
    // former `.folio` hardcodings, arriving as one format.
    expect(tier.showSaveFilePicker).toHaveBeenCalledWith({ suggestedName: 'Untitled template.pdf', types: [{ description: 'PDF document', accept: { 'application/pdf': ['.pdf'] } }] })
    // THE BYTES, against the literal and against the digest the stub reported.
    expect(tier.written[0]).toEqual([...exportedPdfBytes])
    expect(createHash('sha256').update(Uint8Array.from(tier.written[0]!)).digest('hex')).toBe(exportedPdfDigest)
    // NOTHING WAS RE-RENDERED OR RE-SERIALIZED. The engine request count does
    // not move across the press.
    expect(request.mock.calls.length).toBe(engineCallsBeforeThePress)
    await waitFor(() => expect(screen.getByText('Saved PDF of revision 1 as statement.pdf')).toBeInTheDocument())
    // THE DOCUMENT IS UNTOUCHED: name, and the clean/dirty verdict that
    // `savedRevision` alone decides.
    expect(screen.getByText('Untitled template')).toBeInTheDocument()
    expect(screen.getByText('Unsaved local changes')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('downloads the same verbatim bytes under a PDF name and a PDF MIME in the fallback tier', async () => {
    const tier = downloadSaveTier()
    await showRenderedPreview(previewRequest(), tier.access)
    fireEvent.click(screen.getByRole('button', { name: 'Save PDF' }))
    await waitFor(() => expect(tier.blobs).toHaveLength(1))
    expect(tier.anchor.download).toBe('Untitled template.pdf')
    expect(tier.blobs[0]!.type).toBe('application/pdf')
    expect([...new Uint8Array(await tier.blobs[0]!.arrayBuffer())]).toEqual([...exportedPdfBytes])
    expect(createHash('sha256').update(new Uint8Array(await tier.blobs[0]!.arrayBuffer())).digest('hex')).toBe(exportedPdfDigest)
    await waitFor(() => expect(screen.getByText('Downloaded PDF of revision 1 as Untitled template.pdf')).toBeInTheDocument())
  })

  it('asks for a fresh target every time, carrying no currentTarget even while one is held', async () => {
    // A REAL RETAINED TARGET FIRST, because the request this test inspects is
    // only interesting when there IS something for it to have carried: with
    // `target` still undefined, `currentTarget: target` and no `currentTarget`
    // at all are the same object to a structural comparison.
    const heldTarget = { kind: 'in-place' as const, name: 'held.folio', handle: { name: 'held.folio', getFile: async () => new File([], 'held.folio'), createWritable: vi.fn(async () => ({ write: async () => undefined, close: async () => undefined })) } }
    const requests: SaveTargetRequest[] = []
    const acquireSaveTarget = vi.fn(async (request: SaveTargetRequest): Promise<AcquiredSaveTarget> => { requests.push(request); return requests.length === 1 ? { name: 'held.folio', target: heldTarget, format: folioFileFormat } : { name: 'held.pdf', format: pdfFileFormat } })
    const writeSave = vi.fn(async (): Promise<SavedLocalFile> => (requests.length === 1 ? { name: 'held.folio', target: heldTarget } : { name: 'held.pdf' }))
    const files: FileAccess = { open: vi.fn(), acquireSaveTarget, writeSave }
    render(<App engine={engine(previewRequest())} fileAccess={files} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'Save As' }))
    await waitFor(() => expect(screen.getByText('held.folio')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Stale historical PDF/ })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Stale historical PDF/ }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Current exact local production PDF/ })).toBeInTheDocument())
    // The template save DID carry the session's target, so the contrast below
    // is between two live requests rather than between a request and a wish.
    expect(requests[0]).toEqual({ suggestedName: 'Untitled template', currentTarget: undefined, saveAs: true, format: folioFileFormat })
    fireEvent.click(screen.getByRole('button', { name: 'Save PDF' }))
    await waitFor(() => expect(requests).toHaveLength(2))
    const pdfRequest: Readonly<Record<string, unknown>> = requests[1]!
    // `saveAs: true` AND NO `currentTarget` KEY AT ALL — asserted by key
    // presence, not by value, because `currentTarget: target` with a target in
    // hand is exactly the mutation that would overwrite the author's `.folio`
    // with PDF bytes, and `toEqual` treats an explicit `undefined` as absent.
    expect(Object.keys(pdfRequest).sort()).toEqual(['format', 'saveAs', 'suggestedName'])
    expect('currentTarget' in pdfRequest).toBe(false)
    expect(pdfRequest.saveAs).toBe(true)
    expect(pdfRequest.format).toBe(pdfFileFormat)
    expect(pdfRequest.suggestedName).toBe('held.folio')
    expect(heldTarget.handle.createWritable).not.toHaveBeenCalled()
  })

  it('shows the picker for a PDF save while a .folio target is held, leaves that handle unwritten, and leaves the template save clean and in place', async () => {
    const templateWrites: number[][] = []
    const pdfWrites: number[][] = []
    const recording = (name: string, sink: number[][]) => ({ name, getFile: async () => new File([new Uint8Array([1, 2, 3])], name), createWritable: vi.fn(async () => ({ write: async (buffer: ArrayBuffer) => { sink.push([...new Uint8Array(buffer)]) }, close: async () => undefined })) })
    const held = recording('held.folio', templateWrites)
    const picked = recording('statement.pdf', pdfWrites)
    // Order, not content: the fake must not decide what to hand back by
    // inspecting the very request this test is making a claim about.
    let pickerCalls = 0
    const showSaveFilePicker = vi.fn(async () => (++pickerCalls === 1 ? held : picked))
    render(<App engine={engine(previewRequest())} fileAccess={new FileSystemAccess({ showOpenFilePicker: vi.fn(), showSaveFilePicker })} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    // A TEMPLATE SAVE FIRST, so a real `.folio` handle is retained.
    fireEvent.click(screen.getByRole('button', { name: 'Save As' }))
    await waitFor(() => expect(templateWrites).toHaveLength(1))
    expect(screen.getByText('Saved local file')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Stale historical PDF/ })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Stale historical PDF/ }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Current exact local production PDF/ })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Save PDF' }))
    await waitFor(() => expect(pdfWrites).toHaveLength(1))
    // THE PICKER WAS SHOWN, and the retained `.folio` handle received nothing:
    // it is still on its single write, the one the template save made.
    expect(showSaveFilePicker).toHaveBeenCalledTimes(2)
    expect(showSaveFilePicker).toHaveBeenLastCalledWith({ suggestedName: 'held.pdf', types: [{ description: 'PDF document', accept: { 'application/pdf': ['.pdf'] } }] })
    expect(held.createWritable).toHaveBeenCalledOnce()
    expect(templateWrites).toHaveLength(1)
    expect(pdfWrites[0]).toEqual([...exportedPdfBytes])
    // AND `title`, `target` AND `savedRevision` ARE WHAT THEY WERE: the name is
    // unchanged, the document is still clean, and the next plain Save goes back
    // in place through the retained handle with no third picker.
    expect(screen.getByText('held.folio')).toBeInTheDocument()
    expect(screen.getByText('Saved local file')).toBeInTheDocument()
    // STORY 13.5 — THE MODE SWITCH. There is no failure card on screen in this
    // row, so the `Return to Design` this used to press was the preview
    // heading's, which is gone; the surviving control with that exact name
    // belongs to the failure card and is a different button.
    fireEvent.click(screen.getByRole('button', { name: 'DESIGN' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save local template' }))
    await waitFor(() => expect(templateWrites).toHaveLength(2))
    expect(showSaveFilePicker).toHaveBeenCalledTimes(2)
    expect(templateWrites[1]).toEqual([1, 2, 3])
  })

  it('names the save as stale before the press and names the stale revision in the completion', async () => {
    const tier = nativeSaveTier()
    await showRenderedPreview(previewRequest(), tier.access, false)
    // The bytes are byte-exact and savable; what is stale is the claim that they
    // are the CURRENT document, which PDF.js has not admitted.
    expect(screen.queryByRole('button', { name: 'Save PDF' })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Save stale PDF' }))
    await waitFor(() => expect(tier.written).toHaveLength(1))
    expect(tier.written[0]).toEqual([...exportedPdfBytes])
    await waitFor(() => expect(screen.getByText('Saved PDF of stale revision 1 as statement.pdf')).toBeInTheDocument())
  })

  it('offers a disabled control with a reason a screen reader reaches when nothing has been rendered or no file access exists', async () => {
    const files: FileAccess = { open: vi.fn(), acquireSaveTarget: vi.fn(), writeSave: vi.fn() }
    render(<App engine={engine()} fileAccess={files} initialSnapshot={snapshot(1)} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    const control = screen.getByRole('button', { name: 'Save PDF' })
    expect(control).toBeDisabled()
    expect(control).toHaveAccessibleDescription('Save PDF is unavailable: no local PDF has been rendered yet.')
    expect(files.acquireSaveTarget).not.toHaveBeenCalled()
    cleanup()
    // The other unavailability, told in its own words rather than as the same
    // sentence twice: this browser exposes no local file access at all.
    await showRenderedPreview(previewRequest(), undefined)
    const withoutAccess = screen.getByRole('button', { name: 'Save PDF' })
    expect(withoutAccess).toBeDisabled()
    expect(withoutAccess).toHaveAccessibleDescription('Save PDF is unavailable: this browser exposes no local file access.')
    cleanup()
    // ⚠ THE REASON NAMES THE CONTROL THE AUTHOR IS LOOKING AT. A reason reading
    // "Save PDF is unavailable" under a button labelled `Save stale PDF` names a
    // control that is not on screen.
    await showRenderedPreview(previewRequest(), undefined, false)
    const stale = screen.getByRole('button', { name: 'Save stale PDF' })
    expect(stale).toBeDisabled()
    expect(stale).toHaveAccessibleDescription('Save stale PDF is unavailable: this browser exposes no local file access.')
  })

  // PATCH 7/8 — THE TWO WRITERS ARE INTERLOCKED IN THE FUNCTIONS, NOT ONLY IN
  // THE RENDERED `disabled` ATTRIBUTES.
  //
  // Both presses are dispatched inside ONE `act`, so React has not re-rendered
  // either control and neither is disabled yet. What refuses the second press is
  // the other writer's latch — which is an invariant of these two functions
  // rather than a property of when React happens to flush.
  it('refuses a PDF save while a template save holds the latch, and the other way round', async () => {
    const requests: SaveTargetRequest[] = []
    let release!: () => void
    const acquireSaveTarget = vi.fn((request: SaveTargetRequest) => { requests.push(request); return new Promise<AcquiredSaveTarget>((resolve) => { release = () => resolve({ name: 'held', format: request.format }) }) })
    const files: FileAccess = { open: vi.fn(), acquireSaveTarget, writeSave: vi.fn(async (): Promise<SavedLocalFile> => ({ name: 'held' })) }
    await showRenderedPreview(previewRequest(), files)
    act(() => {
      screen.getByRole('button', { name: 'Save As' }).dispatchEvent(new MouseEvent('click', { bubbles: true }))
      screen.getByRole('button', { name: 'Save PDF' }).dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    expect(requests.map((request) => request.format)).toEqual([folioFileFormat])
    // AND THE REASON, WHILE IT IS THE ONE IN FLIGHT, does not call the export
    // "another" action.
    expect(screen.getByRole('button', { name: 'Save PDF' })).toHaveAccessibleDescription('Save PDF is unavailable while a local file action is in progress.')
    release()
    await waitFor(() => expect(screen.getByText(/Saved locally as held|Downloaded local file held/)).toBeInTheDocument())
    act(() => {
      screen.getByRole('button', { name: 'Save PDF' }).dispatchEvent(new MouseEvent('click', { bubbles: true }))
      screen.getByRole('button', { name: 'Save As' }).dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    expect(requests.map((request) => request.format)).toEqual([folioFileFormat, pdfFileFormat])
    release()
    await waitFor(() => expect(requests).toHaveLength(2))
  })

  it('stays silent when the author cancels the picker and changes nothing', async () => {
    const acquireSaveTarget = vi.fn(async () => { throw new FileAccessCancelled() })
    const writeSave = vi.fn()
    await showRenderedPreview(previewRequest(), { open: vi.fn(), acquireSaveTarget, writeSave })
    fireEvent.click(screen.getByRole('button', { name: 'Save PDF' }))
    await waitFor(() => expect(acquireSaveTarget).toHaveBeenCalledOnce())
    await waitFor(() => expect(screen.queryByText(/Preparing PDF save/)).not.toBeInTheDocument())
    expect(writeSave).not.toHaveBeenCalled()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.queryByText(/Saved PDF/)).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Current exact local production PDF/ })).toBeInTheDocument()
  })

  it('announces exactly one alert naming the PDF save when the write fails, and leaves the document and its retained target alone', async () => {
    const heldTarget = { kind: 'in-place' as const, name: 'held.folio', handle: { name: 'held.folio', getFile: async () => new File([], 'held.folio'), createWritable: async () => ({ write: async () => undefined, close: async () => undefined }) } }
    const requests: SaveTargetRequest[] = []
    const acquireSaveTarget = vi.fn(async (request: SaveTargetRequest): Promise<AcquiredSaveTarget> => { requests.push(request); return request.format === pdfFileFormat ? { name: 'held.pdf', format: pdfFileFormat } : { name: 'held.folio', target: heldTarget, format: folioFileFormat } })
    const writeSave = vi.fn(async (acquired: AcquiredSaveTarget): Promise<SavedLocalFile> => { if (acquired.format === pdfFileFormat) throw new Error('media removed'); return { name: 'held.folio', target: heldTarget } })
    const files: FileAccess = { open: vi.fn(), acquireSaveTarget, writeSave }
    render(<App engine={engine(previewRequest())} fileAccess={files} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'Save As' }))
    await waitFor(() => expect(screen.getByText('held.folio')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Stale historical PDF/ })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Stale historical PDF/ }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Current exact local production PDF/ })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Save PDF' }))
    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument())
    expect(screen.getAllByRole('alert')).toHaveLength(1)
    expect(screen.getByRole('alert')).toHaveTextContent('Could not save the preview PDF')
    expect(screen.queryByText(/Saved PDF/)).not.toBeInTheDocument()
    expect(screen.getByText('held.folio')).toBeInTheDocument()
    expect(screen.getByText('Saved local file')).toBeInTheDocument()
    // The preview itself is untouched, so the author can try again.
    expect(screen.getByRole('button', { name: /Current exact local production PDF/ })).toBeInTheDocument()
    // ⚠ AND `target` SURVIVED THE FAILURE. Losing it is silent until the
    // author's next plain Save, which would then open a picker instead of
    // writing the file they already named — so the surviving handle is asserted
    // by taking that next Save and reading the request it produced.
    // STORY 13.5 — THE MODE SWITCH. There is no failure card on screen in this
    // row, so the `Return to Design` this used to press was the preview
    // heading's, which is gone; the surviving control with that exact name
    // belongs to the failure card and is a different button.
    fireEvent.click(screen.getByRole('button', { name: 'DESIGN' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save local template' }))
    await waitFor(() => expect(requests).toHaveLength(3))
    expect(requests[2]).toEqual({ suggestedName: 'held.folio', currentTarget: heldTarget, saveAs: false, format: folioFileFormat })
  })

  // PATCH 4 — THE LATCH IS RELEASED ON EVERY PATH, AND NOTHING PROVED IT.
  //
  // Deleting `exportInFlight.current = false` from the `finally` left all 286
  // tests in this file green, because every one of them pressed Save PDF at
  // most once. Its user-facing effect is that Save PDF works exactly ONCE per
  // session, silently, with the button still enabled and no message at all.
  //
  // All three outcomes run through that one `finally`, so all three are pressed
  // here in sequence and a fourth press must still reach the picker.
  it('releases the in-flight latch after success, cancellation and failure alike', async () => {
    const outcomes = ['ok', 'cancel', 'fail', 'ok'] as const
    let press = 0
    const acquireSaveTarget = vi.fn(async (): Promise<AcquiredSaveTarget> => {
      if (outcomes[press] === 'cancel') { press++; throw new FileAccessCancelled() }
      return { name: 'statement.pdf', format: pdfFileFormat }
    })
    const writeSave = vi.fn(async (): Promise<SavedLocalFile> => {
      const outcome = outcomes[press++]
      if (outcome === 'fail') throw new Error('media removed')
      return { name: 'statement.pdf' }
    })
    const files: FileAccess = { open: vi.fn(), acquireSaveTarget, writeSave }
    await showRenderedPreview(previewRequest(), files)
    const control = () => screen.getByRole('button', { name: 'Save PDF' })
    fireEvent.click(control())
    await waitFor(() => expect(screen.getByText('Downloaded PDF of revision 1 as statement.pdf')).toBeInTheDocument())
    fireEvent.click(control())
    await waitFor(() => expect(acquireSaveTarget).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(screen.queryByText(/PDF save/)).not.toBeInTheDocument())
    fireEvent.click(control())
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Could not save the preview PDF'))
    fireEvent.click(control())
    await waitFor(() => expect(screen.getByText('Downloaded PDF of revision 1 as statement.pdf')).toBeInTheDocument())
    expect(acquireSaveTarget).toHaveBeenCalledTimes(4)
    expect(writeSave).toHaveBeenCalledTimes(3)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  // PATCH 5 — THE BYTES AND THE REVISION ARE THE ONES CAPTURED AT THE PRESS.
  //
  // `exportPreviewPdf` reads them off the record BEFORE the picker await. A
  // render that completes while the picker is still open replaces the live
  // preview underneath the save — and the write must still carry what the
  // author pressed on, never whatever arrived in the meantime.
  //
  // The non-vacuity arm is the evidence line: it is asserted to show the SECOND
  // digest before the picker is released, so the replacement demonstrably
  // happened and the byte claim is not passing over a preview that never moved.
  it('writes the bytes and names the revision captured at the press when a newer render lands mid-save', async () => {
    let renders = 0
    const request = vi.fn(async (operation: string) => {
      if (operation === 'parameter-references') return { snapshot: snapshot(1), parameterReferences: { revision: 1, names: [] } }
      if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
      if (operation === 'render') {
        const first = ++renders === 1
        return { snapshot: snapshot(1), bytes: (first ? exportedPdfBytes : replacementPdfBytes).slice().buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: first ? exportedPdfDigest : replacementPdfDigest, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
      }
      return { snapshot: snapshot(1) }
    })
    let releaseTarget!: () => void
    const written: number[][] = []
    const acquireSaveTarget = vi.fn(() => new Promise<AcquiredSaveTarget>((resolve) => { releaseTarget = () => resolve({ name: 'statement.pdf', format: pdfFileFormat }) }))
    const writeSave = vi.fn(async (_acquired: AcquiredSaveTarget, save: { bytes: ArrayBuffer }): Promise<SavedLocalFile> => { written.push([...new Uint8Array(save.bytes)]); return { name: 'statement.pdf' } })
    const files: FileAccess = { open: vi.fn(), acquireSaveTarget, writeSave }
    await showRenderedPreview(request, files)
    await waitFor(() => expect(screen.getByLabelText('Output hash')).toHaveTextContent(exportedPdfDigest))
    fireEvent.click(screen.getByRole('button', { name: 'Save PDF' }))
    await waitFor(() => expect(acquireSaveTarget).toHaveBeenCalledOnce())
    // A second render lands while the picker is still open.
    fireEvent.click(screen.getByRole('button', { name: 'Re-render' }))
    await waitFor(() => expect(screen.getByLabelText('Output hash')).toHaveTextContent(replacementPdfDigest))
    releaseTarget()
    await waitFor(() => expect(written).toHaveLength(1))
    expect(written[0]).toEqual([...exportedPdfBytes])
    expect(createHash('sha256').update(Uint8Array.from(written[0]!)).digest('hex')).toBe(exportedPdfDigest)
    expect(written[0]).not.toEqual([...replacementPdfBytes])
    await waitFor(() => expect(screen.getByText('Downloaded PDF of revision 1 as statement.pdf')).toBeInTheDocument())
  })

  // PATCH 6 — THE ALERT SURVIVES A TAB SWITCH.
  //
  // The pair first sat inside `<div role="tabpanel" … hidden={…}>`, so moving to
  // the DATA tab mid-save removed the `role="alert"` from the accessibility
  // tree entirely. jsdom reports `hidden` content as absent from `getByRole`,
  // so this test reds against that placement and passes against the main.
  //
  // STORY 13.3 — AND THE CONTROL ITSELF NOW SURVIVES IT TOO (DW-281, an OWNER
  // REQUEST). This row used to assert that `Save PDF` was ABSENT with the DATA
  // tab selected, which was a faithful record of the defect rather than of the
  // behaviour anyone wanted: the button, its label and its reason line were all
  // inside the same `hidden` tabpanel. The rail is a sibling of the tabpanels,
  // so all three stay in the accessibility tree whichever tab is selected —
  // re-parenting the action row back inside the tabpanel reds this.
  it('keeps the PDF save alert reachable when the inspector tab changes mid-save', async () => {
    let failWrite!: (error: Error) => void
    const writeSave = vi.fn(() => new Promise<SavedLocalFile>((_resolve, reject) => { failWrite = reject }))
    const files: FileAccess = { open: vi.fn(), acquireSaveTarget: vi.fn(async () => ({ name: 'statement.pdf', format: pdfFileFormat })), writeSave }
    await showRenderedPreview(previewRequest(), files)
    fireEvent.click(screen.getByRole('button', { name: 'Save PDF' }))
    await waitFor(() => expect(writeSave).toHaveBeenCalledOnce())
    fireEvent.click(screen.getByRole('tab', { name: 'DATA' }))
    expect(screen.getByRole('button', { name: 'Save PDF' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Re-render' })).toBeInTheDocument()
    failWrite(new Error('media removed'))
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Could not save the preview PDF'))
  })

  it('treats a second press while a save is in flight as a no-op, opening exactly one picker', async () => {
    let releaseTarget!: () => void
    const acquireSaveTarget = vi.fn(() => new Promise<{ name: string; format: typeof pdfFileFormat }>((resolve) => { releaseTarget = () => resolve({ name: 'statement.pdf', format: pdfFileFormat }) }))
    const writeSave = vi.fn(async () => ({ name: 'statement.pdf' }))
    await showRenderedPreview(previewRequest(), { open: vi.fn(), acquireSaveTarget, writeSave })
    const control = screen.getByRole('button', { name: 'Save PDF' })
    // THREE PRESSES INSIDE ONE `act`, deliberately. React does not re-render
    // between them, so the control is still enabled for the second and third —
    // which is the real rapid double-click, and the only shape in which the
    // in-flight LATCH is what stops them rather than the `disabled` attribute
    // that a later render puts on. Dispatching them through separate
    // `fireEvent.click` calls would flush a render in between and prove nothing.
    act(() => { for (let press = 0; press < 3; press++) control.dispatchEvent(new MouseEvent('click', { bubbles: true })) })
    expect(control).toBeDisabled()
    expect(acquireSaveTarget).toHaveBeenCalledOnce()
    releaseTarget()
    await waitFor(() => expect(writeSave).toHaveBeenCalledOnce())
    expect(acquireSaveTarget).toHaveBeenCalledOnce()
  })
})

// STORY 5 (spec-startup-templates), CAP-7 — SAVE SAMPLE DATA.
//
// The sample the Designer loaded is the one artefact an example gives the author
// that had no way out of the tab. These tests are written over the REAL two file
// tiers wherever the bytes matter, for the reason the PDF export's are: a save
// that re-encoded or truncated the body under the right filename would pass any
// test that only watched the stub it was handed.
describe('Story 5: the loaded sample data saves to a local file', () => {
  const sampleText = '{"customer":{"name":"Preview customer"},"transactions":[]}'
  const sampleBytes = [...new TextEncoder().encode(sampleText)]
  const openDataTab = () => fireEvent.click(screen.getByRole('tab', { name: 'DATA' }))
  const saveControl = () => screen.getByRole('button', { name: 'Save sample data' })
  // A native tier whose picker returns a handle under a DIFFERENT name than the
  // one suggested, so the status line is proved to read the saved name back
  // rather than echo what was offered.
  const nativeTier = (name = 'kept.json') => {
    const written: number[][] = []
    const handle = { name, getFile: async () => new File([], name, { type: 'application/json' }), createWritable: async () => ({ write: async (buffer: ArrayBuffer) => { written.push([...new Uint8Array(buffer)]) }, close: async () => undefined }) }
    const showSaveFilePicker = vi.fn(async (_options: { suggestedName: string; types: ReadonlyArray<unknown> }) => handle)
    const showOpenFilePicker = vi.fn(async () => [handle])
    return { written, showSaveFilePicker, access: new FileSystemAccess({ showOpenFilePicker, showSaveFilePicker }) }
  }
  const mount = (fileAccess?: FileAccess, initialSampleData = sample) => {
    const request = vi.fn(async (operation: string) => ({ snapshot: snapshot(1), ...(operation === 'serialize' ? { bytes } : {}) }))
    render(<App engine={engine(request)} fileAccess={fileAccess} initialSnapshot={snapshot(1)} initialSampleData={initialSampleData} />)
    openDataTab()
    return request
  }

  it('writes the accepted bytes verbatim through the native tier, offers the sample its own name, and leaves the document alone', async () => {
    const tier = nativeTier()
    const request = mount(tier.access)
    const engineCallsBeforeThePress = request.mock.calls.length
    fireEvent.click(saveControl())
    await waitFor(() => expect(tier.written).toHaveLength(1))
    // ONE FORMAT, THREE FACTS. The picker's description, the extension on the
    // suggested name, and the MIME the download tier below carries are one value
    // — `jsonSampleFileFormat` — and this is the description the sample OPEN
    // picker already used.
    expect(tier.showSaveFilePicker).toHaveBeenCalledWith({ suggestedName: 'sample.json', types: [{ description: 'JSON sample data', accept: { 'application/json': ['.json'] } }] })
    expect(tier.written[0]).toEqual(sampleBytes)
    await waitFor(() => expect(screen.getByText('Saved sample data as kept.json')).toBeInTheDocument())
    // THE DOCUMENT IS UNTOUCHED. No engine request crossed the press, so no
    // revision moved; the title and the clean/dirty verdict are what they were.
    expect(request.mock.calls.length).toBe(engineCallsBeforeThePress)
    expect(screen.getByText('Untitled template', { selector: '.document-name' })).toBeInTheDocument()
    expect(screen.getByText('Unsaved local changes')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('downloads the same bytes under the sample name and a JSON MIME in the fallback tier', async () => {
    const tier = downloadSaveTier()
    mount(tier.access)
    fireEvent.click(saveControl())
    await waitFor(() => expect(tier.blobs).toHaveLength(1))
    expect(tier.anchor.download).toBe('sample.json')
    expect(tier.blobs[0]!.type).toBe('application/json')
    expect([...new Uint8Array(await tier.blobs[0]!.arrayBuffer())]).toEqual(sampleBytes)
    // No picker, so no "Saved … as": the tier that cannot promise a destination
    // does not claim one.
    await waitFor(() => expect(screen.getByText('Downloaded sample data sample.json')).toBeInTheDocument())
  })

  it('writes the WHOLE original document for a sample whose tree was truncated for display', async () => {
    // Wider than `SAMPLE_LIMITS.children`, so the panel's projection is bounded
    // and `truncated` is set — the exact case where a re-serialized "save" would
    // hand back a shortened file and nobody would notice.
    const wide = `{${Array.from({ length: 80 }, (_, index) => `"k${index}":${index}`).join(',')}}`
    const tier = nativeTier()
    mount(tier.access, acceptSampleData('wide.json', new TextEncoder().encode(wide).buffer))
    expect(screen.getByText('Tree inspection is truncated to keep this local panel responsive.')).toBeInTheDocument()
    fireEvent.click(saveControl())
    await waitFor(() => expect(tier.written).toHaveLength(1))
    expect(new TextDecoder().decode(Uint8Array.from(tier.written[0]!))).toBe(wide)
    expect(tier.showSaveFilePicker.mock.calls[0]![0]).toMatchObject({ suggestedName: 'wide.json' })
  })

  // ACCEPTANCE CRITERION 1, AND IT IS THE WHOLE POINT OF THE STORY: the saved
  // file must come BACK as sample data. Comparing bytes proves the write was
  // verbatim; running the real `acceptSampleData` over them proves the thing an
  // author actually cares about — that Load sample JSON on the saved file yields
  // the same tree the Preview was rendering from, truncation and all.
  it('hands over bytes that reload through acceptSampleData as the very same sample', async () => {
    const wide = `{${Array.from({ length: 80 }, (_, index) => `"k${index}":${index}`).join(',')}}`
    const loaded = acceptSampleData('wide.json', new TextEncoder().encode(wide).buffer)
    expect(loaded.truncated).toBe(true)
    const tier = nativeTier()
    mount(tier.access, loaded)
    fireEvent.click(saveControl())
    await waitFor(() => expect(tier.written).toHaveLength(1))
    // The reopen, done exactly as `loadSample` does it. The NAME is deliberately
    // different — that is the one thing an author may change in the picker — and
    // everything the panel and the engine read is identical.
    const reopened = acceptSampleData('elsewhere.json', Uint8Array.from(tier.written[0]!).buffer)
    expect(reopened.tree).toEqual(loaded.tree)
    expect(reopened.truncated).toBe(loaded.truncated)
    expect([...new Uint8Array(reopened.bytes)]).toEqual([...new Uint8Array(loaded.bytes)])
  })

  // THE CROSS-LATCH, IN THE ONE SHAPE THAT CAN SEE IT. `sampleSaveInFlight` is
  // read by `save` and by `exportPreviewPdf`, and a `disabled` attribute cannot
  // stand in for it: React does not re-render between two clicks dispatched in
  // one flush, so both buttons are still enabled when the second lands. Without
  // the guard the template save proceeds, opens a second picker, and writes the
  // serialized document over the destination the author chose for their sample.
  it('refuses a template save dispatched in the same flush as a sample save', async () => {
    const written: Array<Readonly<{ name: string; bytes: number[] }>> = []
    const acquireSaveTarget = vi.fn(async (request: SaveTargetRequest): Promise<AcquiredSaveTarget> => ({ name: request.suggestedName, format: request.format }))
    const writeSave = vi.fn(async (target: AcquiredSaveTarget, request: SaveRequest): Promise<SavedLocalFile> => { written.push({ name: target.name, bytes: [...new Uint8Array(request.bytes)] }); return { name: target.name } })
    mount({ open: vi.fn(), acquireSaveTarget, writeSave })
    const sampleSave = saveControl()
    const templateSave = screen.getByRole('button', { name: 'Save local template' })
    expect(templateSave).toBeEnabled()
    act(() => {
      sampleSave.dispatchEvent(new MouseEvent('click', { bubbles: true }))
      templateSave.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await waitFor(() => expect(writeSave).toHaveBeenCalledOnce())
    expect(acquireSaveTarget).toHaveBeenCalledOnce()
    expect(written).toEqual([{ name: 'sample.json', bytes: sampleBytes }])
  })

  // The same guard's OTHER reader. `exportPreviewPdf` carries its own copy, and
  // a PDF save that slipped through would write the rendered document over the
  // sample's destination just as the template save would.
  it('refuses a PDF export dispatched in the same flush as a sample save', async () => {
    const written: Array<Readonly<{ name: string; bytes: number[] }>> = []
    const acquireSaveTarget = vi.fn(async (request: SaveTargetRequest): Promise<AcquiredSaveTarget> => ({ name: request.suggestedName, format: request.format }))
    const writeSave = vi.fn(async (target: AcquiredSaveTarget, request: SaveRequest): Promise<SavedLocalFile> => { written.push({ name: target.name, bytes: [...new Uint8Array(request.bytes)] }); return { name: target.name } })
    await showRenderedPreview(previewRequest(), { open: vi.fn(), acquireSaveTarget, writeSave })
    const pdfSave = screen.getByRole('button', { name: 'Save PDF' })
    openDataTab()
    const sampleSave = saveControl()
    act(() => {
      sampleSave.dispatchEvent(new MouseEvent('click', { bubbles: true }))
      pdfSave.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await waitFor(() => expect(writeSave).toHaveBeenCalledOnce())
    expect(written).toEqual([{ name: 'sample.json', bytes: sampleBytes }])
  })

  it('renders the control disabled, not absent, in a shell with no local save tier', () => {
    render(<App engine={engine()} initialSnapshot={snapshot(1)} initialSampleData={sample} sampleFileAccess={{ openSample: vi.fn() }} />)
    openDataTab()
    // The sample was loaded, so there IS something to save — the control stays
    // on screen and states its unavailability by being dead, rather than
    // vanishing and leaving the author to wonder where it went.
    expect(saveControl()).toBeDisabled()
    // ...and the sample PICKER is unaffected: this shell can still load one. The
    // two controls answer to two different capabilities, which is why the save's
    // disabled state is not `available`'s.
    expect(screen.getByRole('button', { name: 'Replace sample JSON' })).toBeEnabled()
  })

  it('says nothing and writes nothing when the picker is dismissed, and re-enables the control', async () => {
    const acquireSaveTarget = vi.fn(async () => { throw new FileAccessCancelled() })
    const writeSave = vi.fn(async (): Promise<SavedLocalFile> => ({ name: 'kept.json' }))
    mount({ open: vi.fn(), acquireSaveTarget, writeSave })
    fireEvent.click(saveControl())
    await waitFor(() => expect(saveControl()).toBeEnabled())
    expect(writeSave).not.toHaveBeenCalled()
    // Nothing in the bar: neither the "Preparing…" line the press put up nor a
    // "Saved"/"Downloaded" claim about a file that was never written.
    expect(document.querySelector('.bar-message')).toBeNull()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('names the failure in the bar and releases the file latch when the write throws', async () => {
    const acquireSaveTarget = vi.fn(async (request: SaveTargetRequest): Promise<AcquiredSaveTarget> => ({ name: request.suggestedName, format: request.format }))
    const writeSave = vi.fn(async () => { throw new FileAccessFailure('Could not save local file: NotAllowedError: permission lapsed') })
    mount({ open: vi.fn(), acquireSaveTarget, writeSave })
    fireEvent.click(saveControl())
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('NotAllowedError: permission lapsed'))
    // `fileBusy` released: a failed save does not leave the workspace latched.
    expect(saveControl()).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Save local template' })).toBeEnabled()
  })

  it('takes one press while a save is in flight, and offers no control with no sample loaded', async () => {
    let releaseTarget!: () => void
    const acquireSaveTarget = vi.fn(() => new Promise<AcquiredSaveTarget>((resolve) => { releaseTarget = () => resolve({ name: 'kept.json', format: jsonSampleFileFormat }) }))
    const writeSave = vi.fn(async (): Promise<SavedLocalFile> => ({ name: 'kept.json' }))
    mount({ open: vi.fn(), acquireSaveTarget, writeSave })
    const control = saveControl()
    // Three presses inside one `act`, for the reason the PDF export's twin test
    // gives: no render flushes between them, so the in-flight LATCH is what
    // stops the second and third rather than a `disabled` attribute.
    act(() => { for (let press = 0; press < 3; press++) control.dispatchEvent(new MouseEvent('click', { bubbles: true })) })
    expect(control).toBeDisabled()
    expect(acquireSaveTarget).toHaveBeenCalledOnce()
    releaseTarget()
    await waitFor(() => expect(writeSave).toHaveBeenCalledOnce())

    cleanup()
    render(<App engine={engine()} fileAccess={{ open: vi.fn(), acquireSaveTarget, writeSave }} initialSnapshot={snapshot(1)} />)
    openDataTab()
    expect(screen.getByRole('button', { name: 'Load sample JSON' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Save sample data' })).not.toBeInTheDocument()
  })
})

// STORY 13.4 — PREVIEW RUNS WITHOUT SAMPLE DATA.
//
// Three gates used to make Preview refuse a template with no sample loaded:
// renderPreview's early return, `disabled` on Re-render, and a status
// line whose highest-priority branch said the preview was unavailable. None of
// them was guarded by a test, so nothing would have caught the refusal being
// reintroduced either. These are those guards.
describe('preview with no sample data', () => {
  const STAND_IN = '{"customer":{"name":""}}'
  const CONDITIONAL_SENTENCE = /If a formula uses conditions, literal conditions retain their authored meaning/
  const boundComponent = { id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 12_000, resizable: true, value: 'Hello, {{customer.name}}!' }
  const plainCanvas = { ...canvas, components: [boundComponent] }
  const conditionalCanvas = { ...canvas, components: [boundComponent, { id: 'e2', type: 'rect' as const, band: 'content' as const, x: 0, y: 20_000, width: 72_000, height: 12_000, resizable: true, visibleIf: 'flags.vip' }] }
  const inlineConditionCanvas = { ...canvas, components: [{ ...boundComponent, value: 'Status {{if(flags.vip, "member", "guest")}}' }] }

  // noDataEngine answers every operation the no-data path needs. `standIn` is
  // the document the ENGINE generated; the assertions below read the bytes the
  // engine was handed back on the data channel, so a designer that invented a
  // document of its own would fail rather than agree with itself.
  const noDataEngine = (projection: CanvasProjection = plainCanvas, standIn = STAND_IN) => {
    const loaded = { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: projection }
    const request = vi.fn(async (operation: string) => {
      if (operation === 'stand-in-data') return { snapshot: loaded, bytes: new TextEncoder().encode(standIn).buffer }
      if (operation === 'identity') return { snapshot: loaded, preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: loaded, bytes }
      if (operation === 'render') return { snapshot: loaded, bytes: new Uint8Array([9]).buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: PDF_FIXTURE_DIGEST, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
      return { snapshot: loaded }
    })
    return { request, loaded }
  }

  // Guarded: an unguarded index into find()'s result fails with a bare
  // TypeError, which says nothing about WHICH operation never happened.
  const dataChannel = (calls: ReadonlyArray<ReadonlyArray<unknown>>, operation: string) => {
    const call = calls.find((entry) => entry[0] === operation)
    if (!call) throw new Error(`the engine was never asked for '${operation}'; it received ${JSON.stringify(calls.map((entry) => entry[0]))}`)
    const payload = call[1] as { data?: ArrayBuffer } | undefined
    if (!payload?.data) throw new Error(`the '${operation}' request carried no data channel`)
    return new TextDecoder().decode(payload.data)
  }

  it('renders a page from the engine\'s own stand-in document and withholds every production claim', async () => {
    const { request, loaded } = noDataEngine()
    render(<App engine={engine(request)} initialSnapshot={loaded} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Stale historical PDF/ })).toBeInTheDocument())

    // The bytes the engine received on the data channel ARE the projection it
    // produced — not a guessed empty document assembled in the browser.
    expect(dataChannel(request.mock.calls, 'identity')).toBe(STAND_IN)
    expect(dataChannel(request.mock.calls, 'render')).toBe(STAND_IN)
    expect(screen.queryByText('Preview unavailable: no sample data loaded')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Re-render' })).toBeEnabled()

    // Admit the PDF, which is what would otherwise promote the screen to the
    // exact-production claim.
    fireEvent.click(screen.getByRole('button', { name: /Stale historical PDF/ }))
    expect(screen.queryByText('NO-DATA LAYOUT PREVIEW')).not.toBeInTheDocument()
    expect(screen.queryByText('EXACT LOCAL PRODUCTION PDF')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Current no-data layout PDF/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Current exact local production PDF/ })).not.toBeInTheDocument()
    expect(document.getElementById('preview-freshness-status')).toHaveTextContent('Current no-data layout PDF')
    expect(document.getElementById('preview-freshness-status')).toHaveClass('sr-only')
    expect(document.getElementById('preview-freshness-status')).not.toHaveTextContent('exact')
    expect(screen.getByText(/Stand-in local digest/)).toBeInTheDocument()
    expect(screen.queryByText(/Historical producer digest/)).not.toBeInTheDocument()
    // UX-DR25: the notice is labelled and keyboard-reachable, and App.css
    // gives it the shell's ordinary `:focus-visible` outline.
    expect(screen.getByRole('note', { name: 'No-data preview notice' })).toHaveAttribute('tabindex', '0')
  })

  // MATRIX ROW "No-data render" — THE BAR'S HEAD WORD IS A FRESHNESS CLAIM AND
  // NEVER AN EXACTNESS CLAIM, asserted at the app rather than at the pure
  // function, which is where the only coverage was.
  //
  // The temptation this row exists to forbid is real and it looks like care:
  // the screen is being scrupulous about exactness everywhere else on it, so
  // suppressing the token or narrowing it to something like `stand-in` reads
  // like more honesty. It is not. The bar answers HOW OLD, and a no-data render
  // that is current is exactly as current as any other. Story 13.4's exactness
  // disclosure stays where 13.4 put it, and this test pins the single visible warning alongside the token.
  it('reads current in the bar for a no-data render with a single visible no-data warning', async () => {
    const { request, loaded } = noDataEngine()
    render(<App engine={engine(request)} initialSnapshot={loaded} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Stale historical PDF/ })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Stale historical PDF/ }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Current no-data layout PDF/ })).toBeInTheDocument())

    // THE HEAD WORD IS `current`, WHOLE AND UNQUALIFIED — no suppression, no
    // second disclosure smuggled into the frame.
    expect(freshnessText()).toMatch(FRESH_CURRENT)
    expect(screen.getByLabelText('Render freshness').textContent).not.toMatch(/stand|no-data|layout|exact/i)
    // The warning carries the visible disclosure; the viewer retains its accessible status.
    expect(screen.queryByText('NO-DATA LAYOUT PREVIEW')).not.toBeInTheDocument()
    expect(document.getElementById('preview-freshness-status')).toHaveTextContent('Current no-data layout PDF')
    expect(document.getElementById('preview-freshness-status')).toHaveClass('sr-only')
    expect(screen.getByRole('note', { name: 'No-data preview notice' })).toBeInTheDocument()
  })

  it.each([
    ['visibility', conditionalCanvas],
    ['if text', inlineConditionCanvas],
    ['ternary-only text', { ...canvas, components: [{ ...boundComponent, value: '{{flags.vip ? "member" : "guest"}}' }] }],
    ['quoted question mark', { ...canvas, components: [{ ...boundComponent, value: '{{"Need help?"}}' }] }],
    ['plain binding', plainCanvas],
  ])('uses an accurate generic no-data notice for %s', async (_, projection) => {
    const { request, loaded } = noDataEngine(projection)
    render(<App engine={engine(request)} initialSnapshot={loaded} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    const notice = await screen.findByRole('note', { name: 'No-data preview notice' })
    expect(notice).toHaveTextContent(CONDITIONAL_SENTENCE)
    expect(notice).toHaveTextContent(/Parameters are excluded/)
    expect(notice).not.toHaveTextContent('Conditional content may be present or absent')
    expect(notice).not.toHaveTextContent('fabricates at least one condition')
  })

  it('marks the no-data preview stale when a sample is loaded, then re-renders on the exact-production path', async () => {
    const sampleBytes = new TextEncoder().encode('{"customer":{"name":"Ada"}}').buffer
    const openSample = vi.fn(async () => ({ name: 'sample.json', bytes: sampleBytes }))
    const { request, loaded } = noDataEngine()
    render(<App engine={engine(request)} initialSnapshot={loaded} sampleFileAccess={{ openSample }} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Stale historical PDF/ })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Stale historical PDF/ }))
    expect(screen.getByRole('button', { name: /Current no-data layout PDF/ })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: 'DATA' }))
    fireEvent.click(screen.getByRole('button', { name: 'Load sample JSON' }))
    // The identity key is over `data`, so replacing the stand-in bytes with
    // real ones re-keys the preview for free.
    await waitFor(() => expect(request.mock.calls.filter(([name]) => name === 'render')).toHaveLength(2))
    const renders = request.mock.calls.filter(([name]) => name === 'render') as unknown as ReadonlyArray<ReadonlyArray<unknown>>
    expect(new TextDecoder().decode(((renders[1] as unknown[])[1] as { data: ArrayBuffer }).data)).toBe('{"customer":{"name":"Ada"}}')
    // Wait on the DIGEST LINE, not on the viewer label: the label reads
    // `Stale historical PDF` for the stand-in record too, so waiting on it
    // would assert against whichever record happened to be installed.
    await waitFor(() => expect(screen.getByText(/Historical producer digest/)).toBeInTheDocument())
    expect(screen.queryByRole('note', { name: 'No-data preview notice' })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Stale historical PDF/ }))
    expect(screen.queryByText('EXACT LOCAL PRODUCTION PDF')).not.toBeInTheDocument()
    expect(document.getElementById('preview-freshness-status')).toHaveClass('sr-only')
    expect(screen.getByRole('button', { name: /Current exact local production PDF/ })).toBeInTheDocument()
    expect(screen.getByText(/Historical producer digest/)).toBeInTheDocument()
  })

  it('lands on a no-data preview when the sample is cleared, not on an empty idle screen', async () => {
    const { request, loaded } = noDataEngine()
    render(<App engine={engine(request)} initialSnapshot={loaded} initialSampleData={sample} blankBytes={new Uint8Array([7]).buffer} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Stale historical PDF/ })).toBeInTheDocument())
    expect(screen.queryByRole('note', { name: 'No-data preview notice' })).not.toBeInTheDocument()

    // Start blank clears the accepted sample through clearSampleData.
    startBlankFromNew()
    await waitFor(() => expect(request.mock.calls.some(([name]) => name === 'stand-in-data')).toBe(true))
    expect(await screen.findByRole('note', { name: 'No-data preview notice' })).toBeInTheDocument()
    expect(document.getElementById('preview-freshness-status')).not.toHaveTextContent('Preview is waiting for local inputs')
  })

  it('installs no preview and invents no empty document when the stand-in projection is unavailable', async () => {
    const { loaded } = noDataEngine()
    const request = vi.fn(async (operation: string) => {
      if (operation === 'stand-in-data') throw new Error('Stand-in data is unavailable for this template')
      if (operation === 'identity') return { snapshot: loaded, preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: loaded, bytes }
      if (operation === 'render') return { snapshot: loaded, bytes: new Uint8Array([9]).buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: PDF_FIXTURE_DIGEST, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
      return { snapshot: loaded }
    })
    render(<App engine={engine(request)} initialSnapshot={loaded} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(document.getElementById('preview-freshness-status')).toHaveTextContent('Stand-in data is unavailable for this template'))
    expect(request.mock.calls.some(([name]) => name === 'render')).toBe(false)
    expect(request.mock.calls.some(([name]) => name === 'identity')).toBe(false)
    expect(screen.queryByRole('button', { name: /Stale historical PDF/ })).not.toBeInTheDocument()
  })

  // PATCH 3 — THE NOTICE DESCRIBES THE BYTES ON SCREEN, NOT THE CURRENT INPUTS.
  //
  // Both transitions are asserted, and the second render is HELD so the window
  // between them is a state this test can stand in rather than a race. Keying
  // the notice on `!sampleData` passes the second half and fails the first: the
  // notice would unmount the instant a sample is loaded, while the stand-in PDF
  // is still the thing displayed and the digest line still says so.
  it('keeps the notice on screen while stand-in bytes are displayed, and drops it when real bytes replace them', async () => {
    const sampleBytes = new TextEncoder().encode('{"customer":{"name":"Ada"}}').buffer
    const openSample = vi.fn(async () => ({ name: 'sample.json', bytes: sampleBytes }))
    const loaded = { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: plainCanvas }
    let renders = 0
    let releaseSecondRender: () => void = () => undefined
    const held = new Promise<void>((resolve) => { releaseSecondRender = resolve })
    const request = vi.fn(async (operation: string) => {
      if (operation === 'stand-in-data') return { snapshot: loaded, bytes: new TextEncoder().encode(STAND_IN).buffer }
      if (operation === 'identity') return { snapshot: loaded, preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: loaded, bytes }
      if (operation === 'render') {
        renders++
        if (renders === 2) await held
        return { snapshot: loaded, bytes: new Uint8Array([9]).buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: PDF_FIXTURE_DIGEST, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
      }
      return { snapshot: loaded }
    })
    render(<App engine={engine(request)} initialSnapshot={loaded} sampleFileAccess={{ openSample }} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByText(/Stand-in local digest/)).toBeInTheDocument())
    expect(screen.getByRole('note', { name: 'No-data preview notice' })).toBeInTheDocument()

    // TRANSITION ONE: a sample is loaded and the second render is in flight.
    // The bytes on screen are still the stand-in ones, so the notice stays and
    // the digest line still names them — one PDF, one story about it.
    fireEvent.click(screen.getByRole('tab', { name: 'DATA' }))
    fireEvent.click(screen.getByRole('button', { name: 'Load sample JSON' }))
    await waitFor(() => expect(renders).toBe(2))
    expect(screen.getByRole('note', { name: 'No-data preview notice' })).toBeInTheDocument()
    expect(screen.getByText(/Stand-in local digest/)).toBeInTheDocument()

    // TRANSITION TWO: the real record installs and both lines change together.
    releaseSecondRender()
    await waitFor(() => expect(screen.getByText(/Historical producer digest/)).toBeInTheDocument())
    expect(screen.queryByRole('note', { name: 'No-data preview notice' })).not.toBeInTheDocument()
  })

  // PATCH 1 — SAVING IS WHERE THESE BYTES LEAVE THE MACHINE.
  //
  // The file outlives the session and nothing inside a PDF says its values were
  // fabricated, so the control and the completion both name it. The two
  // qualifiers are independent: this asserts each alone and both together,
  // against the existing `Save stale PDF` / `Saved PDF of stale revision 1`
  // wording, which must keep working unchanged.
  it('names the stand-in on the export control and on the completion, composing with staleness', async () => {
    const written: number[][] = []
    const files: FileAccess = {
      open: vi.fn(),
      acquireSaveTarget: vi.fn(async () => ({ name: 'statement.pdf', format: pdfFileFormat } as AcquiredSaveTarget)),
      writeSave: vi.fn(async (_target, payload: { bytes: ArrayBuffer }): Promise<SavedLocalFile> => { written.push([...new Uint8Array(payload.bytes)]); return { name: 'statement.pdf', target: {} as never } }),
    }
    const { request, loaded } = noDataEngine()
    render(<App engine={engine(request)} fileAccess={files} initialSnapshot={loaded} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Stale historical PDF/ })).toBeInTheDocument())

    // BOTH QUALIFIERS AT ONCE: unadmitted stand-in bytes.
    expect(screen.getByRole('button', { name: 'Save stale no-data PDF' })).toBeInTheDocument()

    // ONE QUALIFIER: admitted stand-in bytes.
    fireEvent.click(screen.getByRole('button', { name: /Stale historical PDF/ }))
    const control = screen.getByRole('button', { name: 'Save no-data PDF' })
    expect(screen.queryByRole('button', { name: 'Save PDF' })).not.toBeInTheDocument()
    fireEvent.click(control)
    await waitFor(() => expect(written).toHaveLength(1))
    expect(screen.getByText('Saved PDF of no-data revision 1 as statement.pdf')).toBeInTheDocument()
  })

  it('leaves a path absent from data that WAS supplied as the located producer Error it already was', async () => {
    const failure = Object.assign(new Error('folio8: Render: element e1: binding "customer.name" is absent from the report data'), { code: 'BINDING_PATH_ABSENT', elementId: 'e1', dataPath: 'customer.name', producerRenderFailure: true as const })
    const request = vi.fn(async (operation: string) => {
      if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
      if (operation === 'render') throw failure
      return { snapshot: snapshot(1) }
    })
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    const card = await screen.findByLabelText('Local render failure')
    expect(card).toHaveTextContent('BINDING_PATH_ABSENT')
    expect(card).toHaveTextContent('customer.name')
    // The stand-in projection is never consulted when a sample IS loaded.
    expect(request.mock.calls.some(([name]) => name === 'stand-in-data')).toBe(false)
  })
})

// STORY 13.2 — THE VIEWER'S NAVIGATION, WHERE THE STATUS BAR PUTS IT.
//
// The arithmetic behind these controls lives in `viewer-navigation.ts` and is
// pinned there against a matrix of its own, with no DOM in it at all. Nothing
// below re-derives any of it. What only App can be asked is the rest: that the
// controls exist in the bottom bar under names of their own, that the platform's
// keyboard reaches and operates them, that what the author types is handed to
// that arithmetic and its answer put back on screen, and that leaving Preview
// and coming back keeps the author's place.
//
// EVERY CONTROL IS LOCATED BY ROLE AND ACCESSIBLE NAME, never by class and never
// by test id. The names are the entire reason these can share one bar with the
// canvas zoom, so a query that reached past them would not be testing the
// property this story shipped.
const previewViewerState = () => JSON.parse(screen.getByTestId('pdf-viewer-state').textContent ?? '{}') as Record<string, number | string | undefined>

// A rendered preview whose document is THIRTY-FOUR pages, which is what makes
// "Page 7 of 34", a next-page press and the out-of-range refusal observable at
// all; the one-page admit button every other block uses cannot show any of them.
const showNavigablePreview = async (request = previewRequest()) => {
  render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
  fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
  await waitFor(() => expect(screen.getByRole('button', { name: 'Admit long local PDF' })).toBeInTheDocument())
  fireEvent.click(screen.getByRole('button', { name: 'Admit long local PDF' }))
  await waitFor(() => expect(screen.getByLabelText('PDF page status')).toHaveTextContent('Page 1 of 34'))
  return request
}

// A KEYBOARD ACTIVATION OF A NATIVE BUTTON, WHICH IS NOT A POINTER PRESS. The
// browser answers Enter and Space on a focused button by dispatching a click
// whose `detail` is 0; a pointer press carries 1 or more. jsdom implements no
// activation behaviour of its own, so that event is dispatched here directly,
// after asserting the element really did take focus.
const pressFromKeyboard = (control: HTMLElement) => {
  control.focus()
  expect(document.activeElement).toBe(control)
  fireEvent(control, createEvent.click(control, { detail: 0 }))
}

const commitTyped = (field: HTMLElement, value: string) => {
  fireEvent.change(field, { target: { value } })
  fireEvent.keyDown(field, { key: 'Enter' })
}

describe('Story 13.2: the viewer navigates from the preview toolbar', () => {
  it('carries every PDF navigation control above the PDF, under names the canvas zoom cannot answer to', async () => {
    await showNavigablePreview()
    const bar = screen.getByLabelText('Preview region')
    expect(within(bar).getByRole('button', { name: 'Previous PDF page' })).toBeInTheDocument()
    expect(within(bar).getByRole('button', { name: 'Next PDF page' })).toBeInTheDocument()
    expect(within(bar).getByRole('textbox', { name: 'PDF page number' })).toBeInTheDocument()
    expect(within(bar).getByRole('textbox', { name: 'PDF zoom percentage' })).toBeInTheDocument()
    expect(within(bar).getByRole('button', { name: 'Zoom out PDF' })).toBeInTheDocument()
    expect(within(bar).getByRole('button', { name: 'Zoom in PDF' })).toBeInTheDocument()
    expect(within(bar).getByLabelText('PDF page status')).toHaveTextContent('Page 1 of 34')
    // AND THEY ARE A NAMED GROUP RATHER THAN SEVEN LOOSE CONTROLS IN A BAR.
    // Located by role: ARIA forbids a name on a `generic` element and browsers
    // discard one, so an `aria-label` with no role is a grouping that does not
    // exist for anybody reading the bar through the accessibility tree.
    expect(within(bar).getByRole('group', { name: 'PDF navigation' })).toBeInTheDocument()
    const choice = within(bar).getByRole('combobox', { name: 'PDF zoom' })
    expect(within(choice).getAllByRole('option').map((option) => option.textContent)).toEqual(['Fit width', 'Fit page', '50%', '75%', '100%', '150%', '200%'])
    // AND NOT ONE OF THEM ANSWERS TO DESIGN MODE'S NAME. `getByRole` matches an
    // accessible name in full, so `Zoom in` finding nothing here is the claim
    // that `Zoom in PDF` is a name of its own rather than a prefix collision.
    expect(screen.queryByRole('button', { name: 'Zoom in' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Zoom out' })).toBeNull()
    expect(screen.queryByLabelText('Canvas zoom')).toBeNull()
  })

  // THE OTHER DIRECTION, which is the half that would go unnoticed: the PDF
  // controls must be absent from Design mode entirely rather than merely
  // renamed, or the bar would carry two zooms and the canvas one would be the
  // ambiguous one.
  it('leaves Design mode with the canvas zoom alone and no PDF controls at all', () => {
    render(<App engine={engine(previewRequest())} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    for (const name of ['Previous PDF page', 'Next PDF page', 'Zoom out PDF', 'Zoom in PDF']) expect(screen.queryByRole('button', { name })).toBeNull()
    expect(screen.queryByRole('textbox', { name: 'PDF page number' })).toBeNull()
    expect(screen.queryByRole('textbox', { name: 'PDF zoom percentage' })).toBeNull()
    expect(screen.queryByRole('combobox', { name: 'PDF zoom' })).toBeNull()
    expect(screen.queryByLabelText('PDF page status')).toBeNull()
    expect(screen.getByRole('button', { name: 'Zoom in' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Zoom out' })).toBeInTheDocument()
    expect(screen.getByLabelText('Canvas zoom')).toHaveTextContent('100%')
  })

  // THE BAR GAINED THE NAVIGATION, AND STORY 13.5 PRICED IT. Until 13.5 this
  // row read "it gave up nothing". The assurance line needs room the bar does
  // not have, so Preview now drops exactly two items — and the row that used to
  // assert "nothing moved" is the right place to say precisely WHICH two, and
  // that the other three did not go with them.
  it('keeps the snapshot, the offline region and the mode in the footer, having dropped exactly two items', async () => {
    await showNavigablePreview()
    const bar = screen.getByLabelText('Status bar')
    expect(within(bar).getByTestId('engine-snapshot')).toHaveTextContent('GO SNAPSHOT · REVISION 1')
    expect(within(bar).getByTestId('offline-status')).toBeInTheDocument()
    expect(within(bar).getByText('PREVIEW MODE')).toBeInTheDocument()
    expect(within(bar).queryByRole('button', { name: 'Next PDF page' })).not.toBeInTheDocument()
    // THE TWO THAT WENT, and nothing else. `offline-status` in particular stays
    // — it is a live region with five states, two of which ('Update available',
    // 'Offline cache unavailable') can arrive while an author sits in Preview.
    // In Preview it is VISUALLY hidden with `.sr-only` and nothing more; the
    // node, its role, its name and its text are all still here, which is why
    // this row still finds it. That pair of claims has its own test below.
    expect(within(bar).queryByText('LOCAL SHELL')).toBeNull()
    expect(within(bar).queryByTestId('template-font-count')).toBeNull()
  })

  it('reaches every control in toolbar order and moves the page from the keyboard alone', async () => {
    await showNavigablePreview()
    const bar = screen.getByRole('group', { name: 'PDF navigation' })
    // Off page one first: `◀` is disabled on the first page, and a disabled
    // control is out of the tab order for a reason that is not this story's.
    commitTyped(within(bar).getByRole('textbox', { name: 'PDF page number' }), '7')
    const controls = Array.from(bar.querySelectorAll<HTMLElement>('button, input, select'))
    expect(controls.map((control) => control.getAttribute('aria-label'))).toEqual(['Previous PDF page', 'PDF page number', 'Next PDF page', 'Zoom out PDF', 'PDF zoom', 'PDF zoom percentage', 'Zoom in PDF'])
    // Reachable is the platform's own tab order: native controls, none disabled
    // at this page, none pulled out of the sequence with a tabindex, each one
    // actually taking focus when it is asked to.
    for (const control of controls) {
      expect(control).not.toBeDisabled()
      expect(control).not.toHaveAttribute('tabindex')
      control.focus()
      expect(document.activeElement).toBe(control)
    }
    pressFromKeyboard(within(bar).getByRole('button', { name: 'Next PDF page' }))
    expect(within(bar).getByLabelText('PDF page status')).toHaveTextContent('Page 8 of 34')
    expect(previewViewerState().page).toBe(8)
    pressFromKeyboard(within(bar).getByRole('button', { name: 'Previous PDF page' }))
    expect(within(bar).getByLabelText('PDF page status')).toHaveTextContent('Page 7 of 34')
    expect(previewViewerState().page).toBe(7)
    // The typed fields take Enter on their own, which is the keyboard's own
    // path through them rather than a synthesized activation.
    const zoom = within(bar).getByRole('textbox', { name: 'PDF zoom percentage' })
    commitTyped(zoom, '150')
    expect(zoom).toHaveValue('150')
    expect(previewViewerState().scale).toBe(1.5)
  })

  it('refuses a typed page the document does not have and puts the current page back in the field', async () => {
    await showNavigablePreview()
    const bar = screen.getByLabelText('Preview region')
    const field = within(bar).getByRole('textbox', { name: 'PDF page number' })
    const status = within(bar).getByLabelText('PDF page status')
    // The row that DOES navigate, first, so every refusal below is the refusal
    // of a move rather than of a no-op that was going to stay put anyway.
    commitTyped(field, '7')
    expect(status).toHaveTextContent('Page 7 of 34')
    expect(field).toHaveValue('7')
    expect(previewViewerState().page).toBe(7)
    for (const refused of ['0', '35', 'abc', '']) {
      commitTyped(field, refused)
      expect(status, `"${refused}" must not navigate`).toHaveTextContent('Page 7 of 34')
      expect(field, `"${refused}" must leave the current page in the field`).toHaveValue('7')
      expect(previewViewerState().page, `"${refused}" must not reach the viewer`).toBe(7)
    }
  })

  it('clamps a typed zoom into the viewer bounds and refuses one that is not a number at all', async () => {
    await showNavigablePreview()
    const bar = screen.getByLabelText('Preview region')
    const field = within(bar).getByRole('textbox', { name: 'PDF zoom percentage' })
    expect(field).toHaveValue('100')
    commitTyped(field, '250')
    expect(field).toHaveValue('200')
    expect(previewViewerState().scale).toBe(2)
    commitTyped(field, '10')
    expect(field).toHaveValue('50')
    expect(previewViewerState().scale).toBe(0.5)
    // Refused, not clamped: the field goes back to reading the zoom the viewer
    // is really at, and the viewer is not written to.
    commitTyped(field, 'abc')
    expect(field).toHaveValue('50')
    expect(previewViewerState().scale).toBe(0.5)
  })

  it('drops the fit the moment the author zooms by hand', async () => {
    await showNavigablePreview()
    const bar = screen.getByLabelText('Preview region')
    const choice = within(bar).getByRole('combobox', { name: 'PDF zoom' })
    fireEvent.change(choice, { target: { value: 'fit-width' } })
    expect(choice).toHaveValue('fit-width')
    expect(previewViewerState().fit).toBe('width')
    fireEvent.click(within(bar).getByRole('button', { name: 'Zoom in PDF' }))
    // The fit is GONE from the state the viewer is handed, not merely
    // overridden beside it — two answers to one question cannot both stand.
    expect(previewViewerState()).not.toHaveProperty('fit')
    expect(previewViewerState().scale).toBe(1.1)
    expect(within(bar).getByRole('textbox', { name: 'PDF zoom percentage' })).toHaveValue('110')
    // 110% names none of the listed choices, so the select stops claiming one.
    expect(choice).toHaveValue('custom')
  })

  // AC5 — THE AUTHOR'S PLACE SURVIVES A TRIP THROUGH DESIGN.
  //
  // All four members are asserted, and asserted against literals, because the
  // defect this replaces was a single `setPreviewViewState(initialPDFPreviewViewState)`
  // at the end of `runPreview`: a test that only checked the viewer had come
  // back, or that compared the state against itself, would have passed over it.
  it('keeps the page, the zoom, the fit and the scroll across a trip through Design', async () => {
    const request = await showNavigablePreview()
    const bar = screen.getByLabelText('Preview region')
    commitTyped(within(bar).getByRole('textbox', { name: 'PDF page number' }), '7')
    commitTyped(within(bar).getByRole('textbox', { name: 'PDF zoom percentage' }), '150')
    fireEvent.change(within(bar).getByRole('combobox', { name: 'PDF zoom' }), { target: { value: 'fit-width' } })
    // The scroll offsets can only ever be recorded by the viewer writing back,
    // which is what the stand-in's own button does.
    fireEvent.click(screen.getByRole('button', { name: 'Scroll local PDF viewer' }))
    expect(previewViewerState()).toEqual({ page: 7, scale: 1.5, fit: 'width', ['scroll' + 'Top']: 240, ['scroll' + 'Left']: 12 })
    const renders = () => request.mock.calls.filter(([operation]) => operation === 'render').length
    const before = renders()
    fireEvent.click(screen.getByRole('button', { name: 'DESIGN' }))
    expect(screen.queryByTestId('pdf-viewer-state')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    // Waited on the RE-RENDER completing, not on the viewer merely reappearing:
    // the reset this stands against sat at the end of that run, so a claim made
    // before the run finished would pass whether or not the reset were there.
    await waitFor(() => expect(renders()).toBe(before + 1))
    expect(previewViewerState()).toEqual({ page: 7, scale: 1.5, fit: 'width', ['scroll' + 'Top']: 240, ['scroll' + 'Left']: 12 })
  })

  // A TAB THROUGH THE FIELD IS NOT A TYPED ZOOM.
  //
  // Both typed fields commit on `blur` as well as on Enter, and with no draft
  // the field is showing the DERIVED readout — so a commit fired by focus alone
  // wrote that readout straight back, and `setPreviewScale` clears the fit. The
  // frozen matrix clears a fit when the author presses `+` or types a zoom, and
  // moving focus is neither. The mock's `samePDFPreviewViewState` answers false
  // deliberately, so a spurious write really does land and really is visible
  // here rather than being swallowed by App's de-dupe.
  it('keeps an active fit when focus merely passes through the typed fields', async () => {
    await showNavigablePreview()
    const bar = screen.getByLabelText('Preview region')
    fireEvent.change(within(bar).getByRole('combobox', { name: 'PDF zoom' }), { target: { value: 'fit-width' } })
    const settled = { page: 1, scale: 1, fit: 'width', ['scroll' + 'Top']: 0, ['scroll' + 'Left']: 0 }
    expect(previewViewerState()).toEqual(settled)
    // Focus arrives on each field and then leaves it for the next control,
    // which is what a Tab keypress does; nothing is typed at any point.
    for (const name of ['PDF zoom percentage', 'PDF page number']) {
      const field = within(bar).getByRole('textbox', { name })
      act(() => { field.focus() })
      expect(document.activeElement).toBe(field)
      // The move is real focus, not a synthesized event — jsdom dispatches both
      // `blur` and `focusout`, and `focusout` is the one React's `onBlur` is
      // mapped from. It is wrapped in `act` so whatever the commit would have
      // written is flushed to the DOM before the state below is read back;
      // without the wrapper a spurious write would sit unrendered and the row
      // would pass over the very defect it exists for. MEASURED both ways.
      act(() => { within(bar).getByRole('button', { name: 'Zoom in PDF' }).focus() })
      expect(document.activeElement).not.toBe(field)
      expect(previewViewerState(), `focus leaving "${name}" must write nothing`).toEqual(settled)
    }
    expect(within(bar).getByRole('combobox', { name: 'PDF zoom' })).toHaveValue('fit-width')
  })

  // THE OTHER ARM OF THE SELECT, which no row above ever selects: every fit
  // assertion in this block picks `fit-width`, so `fit: 'page'` was a branch
  // nothing reached.
  it('stores the page fit the select offers beside fit width', async () => {
    await showNavigablePreview()
    const bar = screen.getByLabelText('Preview region')
    const choice = within(bar).getByRole('combobox', { name: 'PDF zoom' })
    fireEvent.change(choice, { target: { value: 'fit-page' } })
    expect(choice).toHaveValue('fit-page')
    expect(previewViewerState().fit).toBe('page')
  })

  // THE STEPPER'S TWO ENDS. A control that stays enabled at the end of the
  // document offers a press that cannot do anything, and both predicates could
  // be deleted without a single assertion noticing.
  it('disables the stepper at whichever end of the document the author is on', async () => {
    await showNavigablePreview()
    const bar = screen.getByLabelText('Preview region')
    const previous = within(bar).getByRole('button', { name: 'Previous PDF page' })
    const next = within(bar).getByRole('button', { name: 'Next PDF page' })
    expect(previous).toBeDisabled()
    expect(next).not.toBeDisabled()
    fireEvent.click(next)
    expect(within(bar).getByLabelText('PDF page status')).toHaveTextContent('Page 2 of 34')
    expect(previous).not.toBeDisabled()
    commitTyped(within(bar).getByRole('textbox', { name: 'PDF page number' }), '34')
    expect(within(bar).getByLabelText('PDF page status')).toHaveTextContent('Page 34 of 34')
    expect(next).toBeDisabled()
    expect(previous).not.toBeDisabled()
  })

  // AND THE DOCUMENT THAT IS BOTH ENDS AT ONCE.
  it('offers neither direction on a document of a single page', async () => {
    render(<App engine={engine(previewRequest())} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    // Located by its own text: the one-page admit button in the viewer stub
    // carries the preview's label as its accessible name, not this string.
    await waitFor(() => expect(screen.getByText('Admit local PDF')).toBeInTheDocument())
    fireEvent.click(screen.getByText('Admit local PDF'))
    await waitFor(() => expect(screen.getByLabelText('PDF page status')).toHaveTextContent('Page 1 of 1'))
    const bar = screen.getByLabelText('Preview region')
    expect(within(bar).getByRole('button', { name: 'Previous PDF page' })).toBeDisabled()
    expect(within(bar).getByRole('button', { name: 'Next PDF page' })).toBeDisabled()
  })

  // THE MATRIX ROW "PREVIEW CLEARED", which is the one place the reset is
  // right: `invalidatePreview(clear)` throws the preview away entirely, so the
  // place the author was in a document that is gone is not a place to keep —
  // and the page count belongs to that document too. Start blank is the reach:
  // it clears the preview outright and then re-renders on its own tail.
  it('forgets the author place and the page count when the preview is cleared', async () => {
    render(<App engine={engine(previewRequest())} initialSnapshot={snapshot(1)} initialSampleData={sample} blankBytes={new Uint8Array([7]).buffer} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Admit long local PDF' })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Admit long local PDF' }))
    await waitFor(() => expect(screen.getByLabelText('PDF page status')).toHaveTextContent('Page 1 of 34'))
    const bar = screen.getByLabelText('Preview region')
    commitTyped(within(bar).getByRole('textbox', { name: 'PDF page number' }), '7')
    commitTyped(within(bar).getByRole('textbox', { name: 'PDF zoom percentage' }), '150')
    expect(previewViewerState()).toEqual({ page: 7, scale: 1.5, ['scroll' + 'Top']: 0, ['scroll' + 'Left']: 0 })
    startBlankFromNew()
    // THE READOUT IS THE STATUS BAR, NOT THE VIEWER. Clearing unmounts the
    // viewer with the record it belonged to, and Start blank also clears the
    // accepted sample, so nothing re-installs one here. The bar's two typed
    // fields read the view state directly and are rendered for the whole of
    // Preview mode, so they can see the reset the viewer is no longer there
    // to report — page 7 back to 1, 150% back to 100%.
    await waitFor(() => expect(screen.queryByTestId('pdf-viewer-state')).toBeNull())
    const cleared = screen.getByLabelText('Preview region')
    expect(within(cleared).getByRole('textbox', { name: 'PDF page number' })).toHaveValue('1')
    expect(within(cleared).getByRole('textbox', { name: 'PDF zoom percentage' })).toHaveValue('100')
    // And the count went with it: 34 belonged to a document that is gone, so
    // the indicator says the preview is rendering rather than claiming a length
    // it cannot know.
    expect(within(cleared).getByLabelText('PDF page status')).toHaveTextContent('Rendering PDF')
  })
})

// STORY 13.6: THE PALETTE COLUMN BECOMES THE PAGES RAIL.
//
// The rail's own enumeration, bound and marking are covered against the
// component in `preview/page-rail.test.tsx`. These rows are the ones that only
// exist against the real `App`.
describe('Story 13.6: the preview navigates by page thumbnails', () => {
  // THE MODE PARTITION, BOTH WAYS. The Design half is what stops the removal
  // leaking out of Preview: the palette was an unconditional child of
  // `.workbench` until this story, so a gate written on the wrong side of the
  // ternary would take it away everywhere and every `Place …` test would say so
  // — which is why the reverse claim is asserted here rather than assumed.
  it('replaces the component palette with the PAGES rail in Preview, and puts both back where they were in Design', async () => {
    await showNavigablePreview()
    expect(screen.getByLabelText('Page thumbnails')).toBeInTheDocument()
    expect(screen.getByText('PAGES')).toBeInTheDocument()
    // ABSENT FROM THE DOCUMENT, not merely hidden: the palette's landmark, its
    // label and all five of its controls are gone.
    expect(screen.queryByLabelText('Component palette')).toBeNull()
    expect(screen.queryByText('PALETTE')).toBeNull()
    expect(screen.queryAllByRole('button', { name: /^Place / })).toEqual([])
    fireEvent.click(screen.getByRole('button', { name: 'DESIGN' }))
    await waitFor(() => expect(screen.getByLabelText('Canvas region')).toBeInTheDocument())
    expect(screen.getByLabelText('Component palette')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: /^Place / }).map((button) => button.getAttribute('aria-label'))).toEqual(['Place Text', 'Place Image', 'Place Table', 'Place Line', 'Place Rectangle', 'Place Barcode', 'Place QR Code', 'Place Section Break'])
    expect(screen.queryByLabelText('Page thumbnails')).toBeNull()
    expect(screen.queryByText('PAGES')).toBeNull()
  })

  it('hands the rail the page count and the current page from the one page-state authority', async () => {
    await showNavigablePreview()
    expect(JSON.parse(screen.getByTestId('page-rail-props').textContent ?? '{}')).toEqual({ pages: 34, currentPage: 1 })
    commitTyped(within(screen.getByLabelText('Preview region')).getByRole('textbox', { name: 'PDF page number' }), '7')
    // The rail reads the SAME value the viewer does, because there is only one.
    expect(JSON.parse(screen.getByTestId('page-rail-props').textContent ?? '{}')).toEqual({ pages: 34, currentPage: 7 })
    expect(previewViewerState().page).toBe(7)
  })

  // THE FUNNEL, THROUGH THE REAL `App`, COMPARED AS A WHOLE OBJECT. A rail that
  // navigated by building a fresh view state would silently reset the zoom, and
  // an assertion on `page` alone would pass straight over that.
  it('moves the page through the same funnel the preview toolbar writes through, disturbing nothing else', async () => {
    await showNavigablePreview()
    const bar = screen.getByLabelText('Preview region')
    commitTyped(within(bar).getByRole('textbox', { name: 'PDF zoom percentage' }), '150')
    const settled = previewViewerState()
    expect(settled.scale).toBe(1.5)
    fireEvent.click(screen.getByRole('button', { name: 'Page 3' }))
    expect(previewViewerState()).toEqual({ ...settled, page: 3 })
    expect(within(bar).getByLabelText('PDF page status')).toHaveTextContent('Page 3 of 34')
    expect(within(bar).getByRole('textbox', { name: 'PDF zoom percentage' })).toHaveValue('150')
  })

  it('leaves a page the rail truncated away reachable from the preview toolbar, and marks none as current', async () => {
    await showNavigablePreview()
    commitTyped(within(screen.getByLabelText('Preview region')).getByRole('textbox', { name: 'PDF page number' }), '30')
    expect(previewViewerState().page).toBe(30)
    // Twelve entries, page 30 among none of them, and the rail marks nothing
    // rather than marking page 12 — the rail is never the only route to a page.
    // THE RAIL REALLY DID TRUNCATE, AND PAGE 30 IS REALLY NOT IN IT. Asserting
    // the entry count against `PAGE_RAIL_BOUND` alone would be self-consistent by
    // construction — the stand-in bounds itself by the same constant — so the
    // claims here are the ones that can actually fail: fewer entries than the
    // document has pages, no entry for the page the author is on, and NO marking
    // rather than a marking that has drifted onto page 12.
    const listed = screen.getAllByRole('button', { name: /^Page \d+$/ })
    expect(listed).toHaveLength(PAGE_RAIL_BOUND)
    expect(listed.length).toBeLessThan(34)
    expect(screen.queryByRole('button', { name: 'Page 30' })).toBeNull()
    expect(screen.queryAllByRole('button', { current: 'page' })).toEqual([])
  })

  it('renders no rail at all for a render that produced no document to enumerate', async () => {
    const failure = Object.assign(new Error('The template could not be processed'), { code: 'RENDER_INVALID', dataPath: 'items[0]', producerRenderFailure: true as const })
    const request = vi.fn(async (operation: string) => {
      if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
      if (operation === 'render') throw failure
      return { snapshot: snapshot(1) }
    })
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByLabelText('Local render failure')).toBeInTheDocument())
    // Zero bytes, so nothing to enumerate — and the palette does NOT come back
    // to fill the column either. The failure card is what the author reads.
    expect(screen.queryByLabelText('Page thumbnails')).toBeNull()
    expect(screen.queryByLabelText('Component palette')).toBeNull()
  })
})

// STORY 13.3: THE PREVIEW SCREEN IS THE EVIDENCE SCREEN.
//
// The rail's own presentation is covered against the component in
// `preview/evidence-rail.test.tsx`. These rows are the ones that only exist
// against the real `App`: the browser-side digest check that stands between a
// reply and an installed preview, the values the rail reads off a record the
// application actually built, and the placement DW-281 is about.
describe('Story 13.3: the preview screen is the evidence screen', () => {
  // A reply whose bytes and digest DISAGREE, in the shape a real corruption
  // takes: a perfectly well-formed 64-character digest — it passes every
  // protocol guard there is — that describes some other bytes. Before this
  // story the browser had no way to tell the difference, and the screen
  // printed it.
  const mismatchedRequest = () => vi.fn(async (operation: string) => {
    if (operation === 'parameter-references') return { snapshot: snapshot(1), parameterReferences: { revision: 1, names: [] } }
    if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
    if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
    if (operation === 'render') return { snapshot: snapshot(1), bytes: exportedPdfBytes.slice().buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: replacementPdfDigest, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
    return { snapshot: snapshot(1) }
  })

  it('refuses to install or display a preview whose digest does not describe its own bytes', async () => {
    const request = mismatchedRequest()
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(document.getElementById('preview-freshness-status')).toHaveTextContent('does not match the digest the engine reported'))
    // NOT INSTALLED AND NOT DISPLAYED. No viewer, no digest block, no export.
    expect(screen.queryByRole('button', { name: /Stale historical PDF/ })).not.toBeInTheDocument()
    expect(screen.getByLabelText('Output hash')).toHaveTextContent('Go production digest pending')
    expect(screen.getByLabelText('Output hash')).not.toHaveTextContent(replacementPdfDigest)
    expect(screen.getByLabelText('Render facts')).toHaveTextContent('No local render has produced a document yet.')
    // The refusal names BOTH sides, so an author can see which one moved.
    const status = document.getElementById('preview-freshness-status')!
    expect(status).toHaveTextContent(replacementPdfDigest.slice(0, 16))
    expect(status).toHaveTextContent(createHash('sha256').update(Uint8Array.from(exportedPdfBytes)).digest('hex').slice(0, 16))
    // AND IT IS NOT WRITTEN IN THE FORBIDDEN SHAPE. `preview-authority-contract`
    // pins the freshness line's own attributes; a digest mismatch routes
    // through `previewIssue`/`previewStatus` rather than minting a second
    // alert of its own.
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('installs and displays a preview whose digest does describe its bytes, and shows the whole of it', async () => {
    await showRenderedPreview(previewRequest())
    // ⚠ THE TWO SIDES ARE DERIVED INDEPENDENTLY. The DOM's copy travelled
    // engine reply → protocol → App → rail, and was admitted only because
    // `crypto.subtle` agreed with it inside the browser code. This expectation
    // is Node's own SHA-256 over the fixture bytes. Nothing here recomputes one
    // side from the other.
    const displayed = screen.getByLabelText('Output hash').querySelector('.rail-hash-value')!.textContent
    expect(displayed).toBe(createHash('sha256').update(Uint8Array.from(exportedPdfBytes)).digest('hex'))
    expect(displayed).toBe(exportedPdfDigest)
    expect(displayed).toHaveLength(64)
  })

  it('reads the five render values off the record the application built, and prints the version verbatim', async () => {
    const request = previewRequest()
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Stale historical PDF/ })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Admit long local PDF' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Current exact local production PDF/ })).toBeInTheDocument())
    const facts = screen.getByLabelText('Render facts')
    expect(Array.from(facts.querySelectorAll('dt')).map((term) => term.textContent)).toEqual(['engine', 'target', 'pages', 'elapsed', 'size'])
    // The engine's own constant, as it reads. `folio8-go v0.1` is the mockup's
    // invention: no git tag names a release, and printing one on the surface
    // whose rule is never to print an affirmation it cannot earn would be the
    // exact defect this screen exists to prevent.
    expect(facts).toHaveTextContent(`engine${RENDER_ENGINE_VERSION}`)
    expect(facts).toHaveTextContent(`elapsed${RENDER_ELAPSED_MS} ms`)
    // The page count is the VIEWER's, and it is the number the viewer admitted.
    expect(facts).toHaveTextContent('pages34')
    // The size is the length of the buffer the digest covers, not of anything
    // the browser re-encoded.
    expect(facts).toHaveTextContent(`size${exportedPdfBytes.byteLength} B`)
  })

  it('withholds the byte-identity sentence from a no-data preview and keeps every 13.4 withholding', async () => {
    const loaded = { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas }
    const request = vi.fn(async (operation: string) => {
      if (operation === 'stand-in-data') return { snapshot: loaded, bytes: new TextEncoder().encode('{}').buffer }
      if (operation === 'identity') return { snapshot: loaded, preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: loaded, bytes }
      if (operation === 'render') return { snapshot: loaded, bytes: exportedPdfBytes.slice().buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: exportedPdfDigest, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
      return { snapshot: loaded }
    })
    render(<App engine={engine(request)} initialSnapshot={loaded} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Stale historical PDF/ })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Stale historical PDF/ }))
    const block = screen.getByLabelText('Output hash')
    expect(block).toHaveTextContent('Stand-in local digest')
    expect(block).not.toHaveTextContent('Byte-identical across')
    expect(block.querySelector('.rail-hash-value')!.textContent).toBe(exportedPdfDigest)
    // 13.4's other withholdings are untouched.
    expect(screen.queryByText('NO-DATA LAYOUT PREVIEW')).not.toBeInTheDocument()
    expect(screen.getByRole('note', { name: 'No-data preview notice' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save no-data PDF' })).toBeInTheDocument()
  })

  it('marks the rail stale rather than letting an earlier render describe the current document', async () => {
    await showRenderedPreview(previewRequest())
    expect(screen.getByLabelText('Render facts')).not.toHaveTextContent('describe the earlier render')
    fireEvent.click(screen.getByRole('button', { name: 'Fail local PDF viewer' }))
    const facts = screen.getByLabelText('Render facts')
    expect(facts).toHaveTextContent('These values describe the earlier render, not the current document.')
    // And the one value whose authority is the viewer's goes with it: a page
    // count admitted for other bytes is not a fact about these.
    expect(Array.from(facts.querySelectorAll('dt')).map((term) => term.textContent)).not.toContain('pages')
  })

  it('sources the error count from the failed render, which is the only place an error can come from', async () => {
    const failure = Object.assign(new Error('The template could not be processed'), { code: 'RENDER_INVALID', elementId: 'e7', producerRenderFailure: true as const })
    const request = vi.fn(async (operation: string) => {
      if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
      if (operation === 'render') throw failure
      return { snapshot: snapshot(1) }
    })
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await screen.findByLabelText('Local render failure')
    const summary = screen.getByLabelText('Diagnostics summary')
    // ONE ERROR, FROM `currentFailure`. `EngineDiagnostic.severity` is the
    // literal 'warning' — the type has no error severity at all — so a count
    // taken over the diagnostics array could only ever read zero and would look
    // correct forever.
    expect(summary).toHaveTextContent('errors 1')
    expect(summary).toHaveTextContent('warnings 0')
    // The legend's second row is the one that applies, and it is present.
    expect(summary).toHaveTextContent('Square, solid — render failed')
  })

  it('counts the retained diagnostics and names each one\'s place in the document', async () => {
    const projection = { ...canvas, components: [{ id: 'e7', type: 'table' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true }] }
    const loaded = { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: projection }
    const request = vi.fn(async (operation: string) => {
      if (operation === 'identity') return { snapshot: loaded, preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: loaded, bytes }
      if (operation === 'render') return { snapshot: loaded, bytes: exportedPdfBytes.slice().buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: exportedPdfDigest, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [{ severity: 'warning' as const, code: 'CONTENT_CLIPPED', elementId: 'e7', dataPath: 'transactions[11].description', message: 'Row 12 exceeds content height. Clipped.' }] } }
      return { snapshot: loaded }
    })
    render(<App engine={engine(request)} initialSnapshot={loaded} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Stale historical PDF/ })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Stale historical PDF/ }))
    expect(screen.getByLabelText('Diagnostics summary')).toHaveTextContent('warnings 1')
    expect(screen.getByLabelText('Diagnostics summary')).toHaveTextContent('errors 0')
    // The three-part location, joined locally against the projection App holds
    // at the admitted revision.
    expect(screen.getByText('transactions[11].description · table · band content')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Locate on canvas' })).toBeInTheDocument()
  })

  // REVIEW P1 — A REFUSAL MUST NOT LEAVE THE PREVIOUS RENDER'S AFFIRMATION
  // STANDING.
  //
  // This is the sequence the unit fixtures could not reach: a clean preview is
  // installed and admitted, so DIAGNOSTICS reads "The render completed and
  // reported zero diagnostics."; then a re-render arrives corrupted. A digest
  // mismatch sets `previewIssue`, NOT `previewError`, so `currentFailure` stays
  // undefined and `errors` stays 0 — and the previous record's counts are still
  // 0 — so a zero state gated on the counts alone would keep affirming a clean
  // render across a render that was refused for corruption.
  it('stops affirming a clean render the moment one is refused for corruption', async () => {
    let renders = 0
    const request = vi.fn(async (operation: string) => {
      if (operation === 'parameter-references') return { snapshot: snapshot(1), parameterReferences: { revision: 1, names: [] } }
      if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
      if (operation === 'render') {
        renders++
        // The FIRST render is honest and installs; the SECOND carries a
        // well-formed digest of some other bytes.
        return { snapshot: snapshot(1), bytes: exportedPdfBytes.slice().buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: renders === 1 ? exportedPdfDigest : replacementPdfDigest, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
      }
      return { snapshot: snapshot(1) }
    })
    render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Stale historical PDF/ })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Stale historical PDF/ }))
    // THE PRECONDITION, ASSERTED: the affirmation really is on screen first, so
    // its later absence is a change rather than a fixture that never had it.
    expect(screen.getByLabelText('Diagnostics summary')).toHaveTextContent('The render completed and reported zero diagnostics.')

    fireEvent.click(screen.getByRole('button', { name: 'Re-render' }))
    await waitFor(() => expect(document.getElementById('preview-freshness-status')).toHaveTextContent('does not match the digest the engine reported'))
    const summary = screen.getByLabelText('Diagnostics summary')
    expect(summary).not.toHaveTextContent('The render completed and reported zero diagnostics.')
    expect(summary).toHaveTextContent('These counts describe the earlier render; its cards are not shown.')
    // And the RENDER block says the same thing about its own values, so the two
    // sections cannot disagree about which render is being described.
    expect(screen.getByLabelText('Render facts')).toHaveTextContent('These values describe the earlier render, not the current document.')
  })

  // REVIEW P3 — THE RE-CHECK AFTER THE DIGEST AWAIT, PINNED.
  //
  // `crypto.subtle.digest` is a suspension point this story introduced between
  // the render reply's validation and `installPreview`. Deleting the
  // `current(identity)` guard that follows it left all 1071 tests green: the
  // guard was correct and unwatched. Here the digest is HELD, the author leaves
  // Preview while it is in flight, and the resolution must find that it no
  // longer has the authority to install anything.
  //
  // ⚠ THE ABANDONED INSTALL IS OBSERVED BY GOING BACK, and the second render is
  // held open so that nothing new can install and answer for it. `enterPreview`
  // does not clear the record — so if the abandoned pass DID install, the
  // returning author is shown a stale historical PDF built from bytes rendered
  // for a document they had already left. In Design there is nothing on screen
  // to read this off, which is why the assertion is made from Preview.
  it('installs nothing when the author leaves Preview while the digest is still being computed', async () => {
    const realDigest = crypto.subtle.digest.bind(crypto.subtle)
    let releaseDigest!: () => void
    const heldDigest = new Promise<void>((resolve) => { releaseDigest = resolve })
    let digestCalls = 0
    const digestSpy = vi.spyOn(crypto.subtle, 'digest').mockImplementation(async (algorithm: AlgorithmIdentifier, data: BufferSource) => {
      digestCalls++
      if (digestCalls === 1) await heldDigest
      return realDigest(algorithm, data)
    })
    try {
      let renders = 0
      const request = vi.fn(async (operation: string) => {
        if (operation === 'parameter-references') return { snapshot: snapshot(1), parameterReferences: { revision: 1, names: [] } }
        if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
        if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
        if (operation === 'render') {
          // The SECOND visit's render never answers, so the only thing that
          // could put a PDF on screen there is the abandoned first pass.
          if (++renders > 1) await new Promise(() => undefined)
          return { snapshot: snapshot(1), bytes: exportedPdfBytes.slice().buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: exportedPdfDigest, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
        }
        return { snapshot: snapshot(1) }
      })
      render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
      fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
      // The render has landed and the digest is in flight; nothing is installed
      // yet, so there is no viewer to admit.
      await waitFor(() => expect(digestCalls).toBeGreaterThan(0))
      expect(screen.queryByRole('button', { name: /historical PDF/i })).not.toBeInTheDocument()

      // THE AUTHOR LEAVES, which is what `current()` reads: `cancelPreviewWork`
      // advances the request token and aborts the controller, and the mode ref
      // moves to design.
      fireEvent.click(screen.getByRole('button', { name: 'DESIGN' }))
      await waitFor(() => expect(screen.getByLabelText('Canvas region')).toBeInTheDocument())

      // Now let the digest resolve, into a world its caller has already left.
      await act(async () => { releaseDigest(); await Promise.resolve(); await Promise.resolve() })

      // BACK IN PREVIEW, WITH THE NEW RENDER HELD OPEN: nothing is displayed,
      // because nothing was installed.
      fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
      await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'render')).toHaveLength(2))
      expect(screen.queryByRole('button', { name: /historical PDF/i })).not.toBeInTheDocument()
      expect(screen.getByLabelText('Output hash')).toHaveTextContent('Go production digest pending')
      expect(screen.getByLabelText('Render facts')).toHaveTextContent('No local render has produced a document yet.')
    } finally {
      digestSpy.mockRestore()
    }
  })

  it('keeps Re-render, Save PDF and the export reason reachable with the DATA tab selected', async () => {
    // No file access at all, so the export is disabled and owes a reason — the
    // three things DW-281 took out of the accessibility tree together.
    await showRenderedPreview(previewRequest())
    fireEvent.click(screen.getByRole('tab', { name: 'DATA' }))
    expect(screen.getByRole('tab', { name: 'DATA' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByLabelText('Data panel')).toBeInTheDocument()
    // ⚠ jsdom reports content inside a `hidden` element as absent from
    // `getByRole`, which is what makes this row a real witness: re-parenting
    // the action row back into the properties tabpanel reds all four lines.
    const rerender = screen.getByRole('button', { name: 'Re-render' })
    const save = screen.getByRole('button', { name: /^Save.*PDF$/ })
    expect(rerender).toBeInTheDocument()
    expect(save).toBeDisabled()
    expect(save).toHaveAttribute('aria-describedby', 'preview-pdf-export-reason')
    expect(screen.getByText(/is unavailable: this browser exposes no local file access\./)).toHaveAttribute('id', 'preview-pdf-export-reason')
    // Reachable by keyboard alone, from the tab the author is on (UX-DR25).
    rerender.focus()
    expect(document.activeElement).toBe(rerender)
    // And the INPUTS tab keeps the parameter editor it has always held.
    fireEvent.click(screen.getByRole('tab', { name: 'INPUTS' }))
    expect(screen.getByText('PREVIEW INPUTS')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Re-render' })).toBeInTheDocument()
  })
})

// STORY 13.5 — THE CHROME AROUND THE PREVIEW.
//
// Two claims, and they are different claims: the frame states the render's own
// freshness (the document bar), and the application states its standing promise
// (the status bar). The first ticks; the second never changes.
//
// THE CLOCK IS FAKE IN EVERY TICKING ROW AND FROZEN WHILE THE PREVIEW IS BUILT.
// `advanceTimersByTimeAsync(0)` yields a real macrotask WITHOUT moving the
// clock, which is what lets the browser-side digest (a thread-pool promise, not
// a microtask) resolve at a known instant — so `installedAt` is a number this
// file knows, and every age below is asserted exactly rather than by pattern.
//
// TWELVE PASSES, AND THE HELPER PROVES THEY WERE ENOUGH RATHER THAN ASSUMING
// IT. Ten tests below rest on this drain, and a bare loop over a magic count is
// load-bearing in exactly the way that fails silently: let the preview pipeline
// grow past the count and the last state update lands AFTER the helper returns,
// so every age assertion after it reads a bar one step behind — green, wrong,
// and about nothing. So the helper watches the document across its own passes
// and records the last pass that changed it; if the chain ever grows to within
// `SETTLE_MARGIN` of the cap, this fails loudly and names the number it
// reached.
//
// EACH PASS IS ITS OWN `act`, WHICH IS WHAT MAKES THE DOCUMENT READABLE AT ALL.
// With one `act` wrapped around the whole loop, React commits nothing until the
// scope exits, so the markup is identical on every pass and the guard reads
// `-1` forever — an assertion that cannot fail, which is the defect it was
// written to remove. Measured with one `act` per pass: every settle in this
// block lands its change on pass 0 and the remaining eleven are quiet, so the
// margin here is real headroom and not a hopeful constant.
//
// A settle in which NOTHING changes is legitimate and stays legitimate (the
// cancellation row drains a result that must never reach the screen), which is
// why the claim is about the margin and not about a change having happened.
const SETTLE_PASSES = 12
const SETTLE_MARGIN = 3
const settleFrozen = async () => {
  let markup = document.body.innerHTML
  let lastChange = -1
  for (let step = 0; step < SETTLE_PASSES; step += 1) {
    await act(async () => { await vi.advanceTimersByTimeAsync(0) })
    const next = document.body.innerHTML
    if (next !== markup) { lastChange = step; markup = next }
  }
  expect(lastChange).toBeLessThan(SETTLE_PASSES - SETTLE_MARGIN)
}
// THE FIGURE'S SHAPE, TIER-AGNOSTIC ON PURPOSE. The three real-timer rows below
// assert this rather than a `ms`-only pattern: they run on the wall clock, and
// the age they read is however long the render, the digest and jsdom actually
// took. Pinning `ms` pinned a measurement of the machine — and it degraded in
// one direction only, because the age never shrinks: once a slow run crossed
// the 1000 ms boundary the `waitFor` form could never match again and would
// burn its whole timeout. What these rows are for survives intact: the head
// word, the `rendered … ago · <token>` shape, and the fact that the slot is the
// render's and not the page setup's. The EXACT ages are pinned where they can
// be — under the frozen clock, in `the age advances on its own`.
const FRESH_CURRENT = /^rendered \d+ (ms|s) ago · current$/
const freshnessText = () => screen.getByLabelText('Render freshness').textContent
const elapsedFigure = () => within(screen.getByLabelText('Render facts')).getByText('elapsed').nextElementSibling?.textContent

describe('Story 13.5: the chrome tells the truth about the preview', () => {
  // DESIGN MODE'S DOCUMENT BAR, ASSERTED FOR THE FIRST TIME. The page-setup slot
  // had no test at all before this story, which meant a Preview-only swap could
  // have taken Design's reading with it and nothing would have gone red.
  it('states the page setup in Design and says nothing about a render there', () => {
    render(<App engine={engine(previewRequest())} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    expect(screen.getByLabelText('Current page setup')).toHaveTextContent('A4 · portrait')
    expect(screen.queryByLabelText('Render freshness')).toBeNull()
  })

  it('names the absent page setup rather than an empty slot', () => {
    render(<App engine={engine(previewRequest())} initialSnapshot={{ documentState: 'loaded' as const, revision: 1, byteLength: 3 }} />)
    expect(screen.getByLabelText('Current page setup')).toHaveTextContent('Page setup unavailable')
  })

  // THE SLOT SWAPS, AND WHAT IT SWAPS TO IS THE RENDER'S OWN FRESHNESS. The page
  // setup is a fact about a template nobody is looking at in Preview.
  it('replaces the page setup with the render freshness in Preview', async () => {
    await showRenderedPreview(previewRequest())
    expect(screen.queryByLabelText('Current page setup')).toBeNull()
    expect(freshnessText()).toMatch(FRESH_CURRENT)
  })

  // NOT A LIVE REGION, AND THAT IS A DECISION RATHER THAN AN OMISSION. Every
  // neighbour in this bar is one — `status-copy`, and the status bar's own
  // offline element — so copying a neighbour here is the obvious mistake, and it
  // would announce the age to a screen-reader user once a second, then once a
  // minute, for as long as the preview is open. The two ARIA associations are
  // refused for a separate reason: nothing asks for them, and a cross-region
  // association breaks silently when either end moves.
  it('keeps the ticking figure out of the accessibility live region entirely', async () => {
    await showRenderedPreview(previewRequest())
    const slot = screen.getByLabelText('Render freshness')
    for (const attribute of ['role', 'aria-live', 'aria-atomic', 'title', 'aria-describedby']) expect(slot).not.toHaveAttribute(attribute)
  })

  // NO RECORD, NO AGE. Before the first render there is no instant to count
  // from, and the bar says so rather than printing an age of zero for a render
  // that has not happened.
  it('reads no render yet while the first render is still in flight', async () => {
    const request = vi.fn(async (operation: string) => {
      if (operation === 'parameter-references') return { snapshot: snapshot(1), parameterReferences: { revision: 1, names: [] } }
      if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
      if (operation === 'render') return new Promise<never>(() => undefined)
      return { snapshot: snapshot(1) }
    })
    render(<App engine={engine(request as never)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'render')).toHaveLength(1))
    expect(freshnessText()).toBe('no render yet')
    expect(screen.getByText('Rendering local PDF')).toBeInTheDocument()
  })

  // MATRIX ROW "Idle, no record" — AND THE FIRST QUESTION WAS WHETHER THE APP
  // CAN EVEN BE IN IT, because a contrived mount that reaches an unreachable
  // state proves nothing about the product.
  //
  // IT IS REACHABLE, by one route, and the route is synchronous-then-awaited.
  // `startBlank` (`App.tsx:1838`) and `open` (`App.tsx:1789`) both call
  // `invalidatePreview(true)` before their first `await`; that clears the
  // record and sets `idle` (`App.tsx:589-593`) and it does NOT touch `mode`.
  // So an author who starts a blank template from inside Preview sits in
  // Preview / `idle` / no record for as long as the engine's `load` takes to
  // answer, with the document bar on screen the whole time. Held open here, the
  // window is a state to stand in rather than a race to win.
  it('reads no render yet in Preview once the record is cleared and nothing has replaced it', async () => {
    const request = vi.fn(async (operation: string) => {
      if (operation === 'parameter-references') return { snapshot: snapshot(1), parameterReferences: { revision: 1, names: [] } }
      if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
      if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
      if (operation === 'render') return { snapshot: snapshot(1), bytes: exportedPdfBytes.slice().buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: exportedPdfDigest, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
      if (operation === 'load') return new Promise<never>(() => undefined)
      return { snapshot: snapshot(1) }
    })
    render(<App engine={engine(request as never)} initialSnapshot={snapshot(1)} initialSampleData={sample} blankBytes={new Uint8Array([7]).buffer} />)
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(screen.getByRole('button', { name: /Stale historical PDF/ })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: /Stale historical PDF/ }))
    // THE PRECONDITION, ASSERTED: there is a dated render on the bar first, so
    // its disappearance below is a transition rather than a fixture that never
    // had one.
    await waitFor(() => expect(freshnessText()).toMatch(FRESH_CURRENT))

    startBlankFromNew()
    await waitFor(() => expect(request.mock.calls.some(([operation]) => operation === 'load')).toBe(true))
    expect(screen.queryByTestId('pdf-viewer-state')).toBeNull()
    expect(freshnessText()).toBe('no render yet')
    // AND THE SLOT IS STILL THE RENDER'S. Falling back to the page setup when
    // there is no record would be a plausible reading of "the bar has nothing
    // to say about a render" and it is the wrong one: the author is looking at
    // Preview, and the page setup is a fact about a template nobody is looking
    // at. Nothing else in the suite forbids that fallback.
    expect(screen.queryByLabelText('Current page setup')).toBeNull()
    expect(document.getElementById('preview-freshness-status')).toHaveTextContent('Preview is waiting for local inputs')
  })

  describe('the age advances on its own', () => {
    beforeEach(() => { vi.useFakeTimers() })
    afterEach(() => { vi.useRealTimers() })

    // Built at a frozen clock so `installedAt` is the instant this helper
    // returns at, and every age below is measured from zero.
    const previewAtZero = async (request = previewRequest()) => {
      render(<App engine={engine(request)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
      fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
      await settleFrozen()
      fireEvent.click(screen.getByRole('button', { name: /Stale historical PDF/ }))
      await settleFrozen()
      expect(freshnessText()).toBe('rendered 0 ms ago · current')
      return request
    }

    // A render that is honest FIRST and dishonest SECOND, so the second pass
    // arrives over a record the first pass installed. `renders === 1` decides
    // which digest is reported, and only the digest differs: the bytes are the
    // same fixture both times, so nothing but the mismatch can explain the
    // refusal.
    const secondRenderCorrupted = () => {
      let renders = 0
      return vi.fn(async (operation: string) => {
        if (operation === 'parameter-references') return { snapshot: snapshot(1), parameterReferences: { revision: 1, names: [] } }
        if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
        if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
        if (operation === 'render') {
          renders++
          return { snapshot: snapshot(1), bytes: exportedPdfBytes.slice().buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: renders === 1 ? exportedPdfDigest : replacementPdfDigest, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
        }
        return { snapshot: snapshot(1) }
      })
    }

    // MATRIX ROW "Inputs changed" — AND THE AGE IS PINNED, NOT PATTERNED.
    //
    // This row previously asserted `/^rendered \d+ ms ago · stale$/`, and `\d+`
    // is satisfied by ZERO: a bar that dated a stale record from the moment the
    // inputs changed rather than from the render — which is the whole thing the
    // stamp exists to prevent — kept that row green. Under the frozen clock the
    // figure is a number this file chose, so the assertion is about the
    // render's own stamp and not merely about the shape of the string.
    it('moves the bar and the status line together when the inputs change, still counting from the render', async () => {
      await previewAtZero()
      await act(async () => { await vi.advanceTimersByTimeAsync(4000) })
      expect(freshnessText()).toBe('rendered 4 s ago · current')
      expect(screen.getByText('Current exact local PDF')).toBeInTheDocument()
      fireEvent.click(screen.getByRole('tab', { name: 'INPUTS' }))
      fireEvent.change(screen.getByRole('textbox', { name: 'Raw parameter JSON' }), { target: { value: '{"changed":1}' } })
      // No clock movement at all — only the queued work is drained — so the
      // debounced re-render cannot fire and the record under the bar is still
      // the one installed four seconds ago.
      await settleFrozen()
      expect(screen.getByText('STALE — inputs changed')).toBeInTheDocument()
      expect(freshnessText()).toBe('rendered 4 s ago · stale')
    })

    // MATRIX ROW "Digest mismatch over a good preview", AT THE APP AND NOT AT
    // THE PURE FUNCTION.
    //
    // `freshness.test.ts` covers this row by PASSING `hasRecord: true` in as an
    // argument, so it never reaches the expression in `App.tsx` that decides
    // whether a record exists. Measured: rewriting that call site's
    // `hasRecord: preview !== undefined` as
    // `hasRecord: previewStatus !== 'error' && preview !== undefined` makes the
    // bar read `no render yet` over a preview that is still on screen, and the
    // whole suite stayed green. This row is the witness of that decision.
    //
    // A digest mismatch NEITHER INSTALLS NOR CLEARS, which is what makes the
    // state expressible: the refused render leaves the earlier record on
    // screen, so the bar has a render to date and must go on dating it while
    // refusing to affirm it.
    it('keeps dating the surviving record and refuses to affirm it when a re-render is refused for corruption', async () => {
      await previewAtZero(secondRenderCorrupted() as never)
      await act(async () => { await vi.advanceTimersByTimeAsync(3000) })
      expect(freshnessText()).toBe('rendered 3 s ago · current')

      fireEvent.click(screen.getByRole('button', { name: 'Re-render' }))
      await settleFrozen()
      expect(document.getElementById('preview-freshness-status')).toHaveTextContent('does not match the digest the engine reported')
      // THE PRECONDITION THE ROW RESTS ON: a record really did survive the
      // refusal, so `no render yet` would be a false reading rather than a
      // defensible one.
      expect(screen.getByTestId('pdf-viewer-state')).toBeInTheDocument()
      // NEITHER `no render yet` NOR `current`. The bar dates the bytes it is
      // showing and calls them stale, because the app has just refused them.
      expect(freshnessText()).toBe('rendered 3 s ago · stale')
    })

    // MATRIX ROW "Render failed" — A DIFFERENT ROW, AND IT HAD NO APP-LEVEL
    // WITNESS AT ALL. Population searched: `src/` and `e2e/` entire, with
    // `grep -ran` (so `App.tsx`'s two NUL bytes cannot hide a hit).
    // `STALE — latest local render failed` appeared in exactly two places,
    // `preview/freshness.ts` and its own unit test — nothing drove the app into
    // the state that produces it.
    //
    // It shares the record-detection decision with the row above but NOT the
    // path into it: this one goes through `runPreview`'s catch, which sets
    // `staleReason` to `render-failed` and installs a `previewError`, and it is
    // `staleReason` that the status line branches on.
    it('dates the surviving record and names the failed render when a re-render rejects', async () => {
      let renders = 0
      const request = vi.fn(async (operation: string) => {
        if (operation === 'parameter-references') return { snapshot: snapshot(1), parameterReferences: { revision: 1, names: [] } }
        if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
        if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
        if (operation === 'render') {
          if (++renders > 1) throw Object.assign(new Error('The template could not be processed'), { code: 'RENDER_INVALID', elementId: 'e7', producerRenderFailure: true as const })
          return { snapshot: snapshot(1), bytes: exportedPdfBytes.slice().buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: exportedPdfDigest, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
        }
        return { snapshot: snapshot(1) }
      })
      await previewAtZero(request as never)
      await act(async () => { await vi.advanceTimersByTimeAsync(6000) })
      expect(freshnessText()).toBe('rendered 6 s ago · current')

      fireEvent.click(screen.getByRole('button', { name: 'Re-render' }))
      await settleFrozen()
      expect(document.getElementById('preview-freshness-status')).toHaveTextContent('STALE — latest local render failed')
      expect(screen.getByTestId('pdf-viewer-state')).toBeInTheDocument()
      expect(freshnessText()).toBe('rendered 6 s ago · stale')
    })

    // AC1, AND THE QUANTITY IT MUST NOT BE CONFUSED WITH. `elapsed` is how long
    // the render TOOK; the bar's figure is how long ago it FINISHED. They are
    // equal for one instant and diverge forever after, and both are on this
    // screen at the same time in the same 10px mono — so the row that proves one
    // moves is the row that must prove the other did not.
    it('advances the figure with no new render while the engine elapsed stays put', async () => {
      await previewAtZero()
      expect(elapsedFigure()).toBe('7 ms')
      await act(async () => { await vi.advanceTimersByTimeAsync(400) })
      expect(freshnessText()).toBe('rendered 400 ms ago · current')
      await act(async () => { await vi.advanceTimersByTimeAsync(2800) })
      expect(freshnessText()).toBe('rendered 3 s ago · current')
      await act(async () => { await vi.advanceTimersByTimeAsync(56_800) })
      expect(freshnessText()).toBe('rendered 1 min ago · current')
      // NOT ONE RE-RENDER PAID FOR ANY OF THAT, and the engine's own number is
      // exactly where it was an hour of wall clock earlier.
      expect(elapsedFigure()).toBe('7 ms')
    })

    // THE OTHER HALF OF THE STAMP, AND IT WAS THE UNASSERTED ONE.
    //
    // Every other ticking row here drives a path that does NOT install — a
    // digest mismatch, a rejected render, an abandoned one — or asserts that the
    // age SURVIVES something. All of them are satisfied by a `PreviewRecord`
    // stamped once and never again. Measured: rewriting the single install site
    // as `installedAt: previewRef.current?.installedAt ?? Date.now()` — stamp
    // only when there is no record yet — leaves the entire suite green without
    // this row. And that implementation is a bar that dates the bytes on screen
    // by when some EARLIER bytes finished: it would read `9 s ago` over a render
    // that had just completed, and go on drifting for as long as the author kept
    // re-rendering. `installedAt` describes THESE bytes, so new bytes re-stamp
    // it, and the figure returns to a fresh age.
    it('re-stamps the age when new bytes land, so the figure is about the render on screen', async () => {
      await previewAtZero()
      await act(async () => { await vi.advanceTimersByTimeAsync(9000) })
      expect(freshnessText()).toBe('rendered 9 s ago · current')

      // A SECOND RENDER THAT REALLY LANDS. Same engine, same bytes, same digest
      // — the only thing that differs from the record on screen is that this one
      // is new, which is the whole point: the age must reset on an install and
      // not on a change of content.
      fireEvent.click(screen.getByRole('button', { name: 'Re-render' }))
      await settleFrozen()
      // The install marks the candidate stale until the viewer admits it, and
      // the clock has not moved a millisecond across either step.
      expect(freshnessText()).toBe('rendered 0 ms ago · stale')
      fireEvent.click(screen.getByRole('button', { name: /Stale historical PDF/ }))
      await settleFrozen()
      expect(freshnessText()).toBe('rendered 0 ms ago · current')
      expect(screen.getByText('Current exact local PDF')).toBeInTheDocument()

      // AND IT COUNTS FROM THE NEW STAMP AFTERWARDS, not from the old one: a
      // reset that immediately jumped back would satisfy the line above.
      await act(async () => { await vi.advanceTimersByTimeAsync(2000) })
      expect(freshnessText()).toBe('rendered 2 s ago · current')
    })

    // THE STAMP IS THE RENDER'S, NOT THE VISIT'S. Leaving Preview and coming
    // back must not restart the count, or the bar would be describing the
    // author's navigation instead of the bytes on screen.
    it('keeps counting from the original render across a trip through Design', async () => {
      // The second render never lands, so the FIRST record is still the one on
      // screen when Preview comes back — which is the only condition under
      // which "the age survived" is a claim about anything.
      let renders = 0
      const request = vi.fn(async (operation: string) => {
        if (operation === 'parameter-references') return { snapshot: snapshot(1), parameterReferences: { revision: 1, names: [] } }
        if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
        if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
        if (operation === 'render') {
          if (++renders > 1) return new Promise<never>(() => undefined)
          return { snapshot: snapshot(1), bytes: exportedPdfBytes.slice().buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: exportedPdfDigest, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
        }
        return { snapshot: snapshot(1) }
      })
      await previewAtZero(request as never)
      await act(async () => { await vi.advanceTimersByTimeAsync(5000) })
      expect(freshnessText()).toBe('rendered 5 s ago · current')
      fireEvent.click(screen.getByRole('button', { name: 'DESIGN' }))
      await act(async () => { await vi.advanceTimersByTimeAsync(5000) })
      fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
      await settleFrozen()
      // Ten seconds since the render, not zero since the return.
      expect(freshnessText()).toBe('rendered 10 s ago · stale')
    })

    // DESIGN MODE MUST NEVER HOLD A LIVE INTERVAL. `setInterval` appears nowhere
    // else in `src`, so every call the spy sees is this story's, and the two
    // directions are asserted separately: none is armed while the canvas is up,
    // and the one armed in Preview is cleared on the way out.
    it('arms the interval only in Preview and clears it on the way back to Design', async () => {
      const armed = vi.spyOn(globalThis, 'setInterval')
      const cleared = vi.spyOn(globalThis, 'clearInterval')
      try {
        render(<App engine={engine(previewRequest())} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
        await settleFrozen()
        expect(armed).not.toHaveBeenCalled()
        fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
        await settleFrozen()
        expect(armed).toHaveBeenCalled()
        const clearedBefore = cleared.mock.calls.length
        fireEvent.click(screen.getByRole('button', { name: 'DESIGN' }))
        expect(cleared.mock.calls.length).toBeGreaterThan(clearedBefore)
        // AND NOTHING RE-ARMS IT BEHIND THE CANVAS. A cleanup that ran while a
        // tick was already scheduled would show up here as a fresh call.
        const armedBefore = armed.mock.calls.length
        await act(async () => { await vi.advanceTimersByTimeAsync(120_000) })
        expect(armed.mock.calls.length).toBe(armedBefore)
      } finally {
        armed.mockRestore()
        cleared.mockRestore()
      }
    })

    // AC3. THE HEADING'S BUTTON IS GONE AND THE ABILITY IT CARRIED IS NOT.
    // Asserting only that the mode changed is a test that passes over a
    // completely broken cancel, so all four halves of `cancelPreviewWork` are
    // read off observable consequences: the controller is aborted, the late
    // result is refused, no follow-up work is scheduled, and nothing the engine
    // said after the author left ever reached the screen.
    it('abandons a render in flight when the author presses DESIGN, and never installs its late result', async () => {
      const signals: AbortSignal[] = []
      let land!: (result: unknown) => void
      const request = vi.fn(async (operation: string, _payload: unknown, signal: AbortSignal) => {
        if (operation === 'parameter-references') return { snapshot: snapshot(1), parameterReferences: { revision: 1, names: [] } }
        if (operation === 'identity') return { snapshot: snapshot(1), preview: { revision: 1, identity: 'b'.repeat(64) } }
        if (operation === 'serialize') return { snapshot: snapshot(1), bytes }
        if (operation === 'render') { signals.push(signal); return new Promise((resolve) => { land = resolve }) }
        return { snapshot: snapshot(1) }
      })
      render(<App engine={engine(request as never)} initialSnapshot={snapshot(1)} initialSampleData={sample} />)
      fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
      await settleFrozen()
      expect(signals).toHaveLength(1)
      expect(signals[0]!.aborted).toBe(false)
      expect(freshnessText()).toBe('no render yet')

      // THE ONLY EXIT LEFT, and it aborts synchronously.
      fireEvent.click(screen.getByRole('button', { name: 'DESIGN' }))
      expect(signals[0]!.aborted).toBe(true)
      expect(screen.getByLabelText('Canvas region')).toBeInTheDocument()

      // The result lands in a world its caller has already left.
      land({ snapshot: snapshot(1), bytes: exportedPdfBytes.slice().buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: exportedPdfDigest, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } })
      await settleFrozen()
      await act(async () => { await vi.advanceTimersByTimeAsync(5000) })

      // NOT INSTALLED — there is no viewer anywhere in the document — and the
      // debounce and the scheduler are both empty: five seconds past a 250 ms
      // debounce produced no second render and no further engine traffic.
      expect(screen.queryByTestId('pdf-viewer-state')).toBeNull()
      expect(request.mock.calls.filter(([operation]) => operation === 'render')).toHaveLength(1)
      expect(request.mock.calls.filter(([operation]) => operation === 'command')).toHaveLength(0)
      expect(screen.getByLabelText('Canvas region')).toBeInTheDocument()

      // AND IT IS STILL REFUSED ON THE WAY BACK IN: returning to Preview starts
      // a NEW render rather than showing the bytes the abandoned one produced.
      fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
      await settleFrozen()
      expect(request.mock.calls.filter(([operation]) => operation === 'render')).toHaveLength(2)
      expect(screen.queryByTestId('pdf-viewer-state')).toBeNull()
      expect(freshnessText()).toBe('no render yet')
    })
  })

  // AC4 — GOAL B, WHICH SHARES NONE OF THE MACHINERY ABOVE. The status bar's
  // two-item drop is fenced on `mode` in both directions, and BOTH directions
  // are asserted: the Design-mode half is what stops the drop leaking out of
  // Preview, and it is the half that would go unnoticed.
  it('carries the standing local-only assurance in Preview and the two dropped items in Design', async () => {
    await showRenderedPreview(previewRequest())
    const bar = screen.getByLabelText('Status bar')
    expect(within(bar).getByTestId('local-only-assurance')).toHaveTextContent('no network · nothing left this machine')
    expect(within(bar).queryByText('LOCAL SHELL')).toBeNull()
    expect(within(bar).queryByTestId('template-font-count')).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: 'DESIGN' }))
    expect(within(bar).getByText('LOCAL SHELL')).toBeInTheDocument()
    expect(within(bar).getByTestId('template-font-count')).toHaveTextContent('2 fonts in template')
    expect(within(bar).queryByTestId('local-only-assurance')).toBeNull()
    // NEITHER MODE LOSES THE THREE THAT STAY.
    expect(within(bar).getByTestId('engine-snapshot')).toBeInTheDocument()
    expect(within(bar).getByTestId('offline-status')).toBeInTheDocument()
    expect(within(bar).getByText('DESIGN MODE')).toBeInTheDocument()
  })

  // RULING Q6 — THE OFFLINE LIVE REGION IS VISUALLY HIDDEN IN PREVIEW, NOT
  // DROPPED, AND NOT ALTERED IN ANY OTHER WAY. Hiding it is what makes the
  // Preview bar's spare room constant instead of varying by the 24 characters
  // between the shortest and the longest of `offlineLabel`'s five states;
  // keeping every ARIA affordance is what stops that being an accessibility
  // regression. BOTH HALVES ARE ASSERTED IN BOTH MODES: a class applied
  // unconditionally would take the region out of DESIGN's painted bar too, and
  // that is the half nothing else in this file would notice.
  it('hides the offline live region from the painted Preview bar while a screen reader loses nothing', async () => {
    await showRenderedPreview(previewRequest())
    const bar = screen.getByLabelText('Status bar')
    const inPreview = within(bar).getByTestId('offline-status')
    expect(inPreview).toHaveClass('sr-only')
    expect(inPreview).toHaveAttribute('role', 'status')
    expect(inPreview).toHaveAttribute('aria-live', 'polite')
    expect(inPreview).toHaveAttribute('aria-label', 'Offline availability')
    expect(inPreview).toHaveTextContent('Offline cache unavailable')
    // The assistive-technology view of the bar is unchanged: the region is
    // still reachable by role and accessible name, and it is the same node.
    expect(within(bar).getByRole('status', { name: 'Offline availability' })).toBe(inPreview)

    fireEvent.click(screen.getByRole('button', { name: 'DESIGN' }))
    const inDesign = within(bar).getByTestId('offline-status')
    expect(inDesign).not.toHaveClass('sr-only')
    // Not merely 'a different class' — Design's span carries no class at all,
    // which is exactly what it carried before this story touched the bar.
    expect(inDesign.getAttribute('class')).toBeNull()
    expect(inDesign).toHaveAttribute('role', 'status')
    expect(inDesign).toHaveAttribute('aria-live', 'polite')
    expect(inDesign).toHaveAttribute('aria-label', 'Offline availability')
    expect(inDesign).toHaveTextContent('Offline cache unavailable')
  })

  // THE ASSURANCE IS A STATEMENT, NOT A CONTROL. The bar's interactive set is
  // pinned exhaustively by name elsewhere in this file; this is the same claim
  // said from the other side, at the element that was added.
  it('adds nothing interactive and nothing announced to the status bar', async () => {
    await showRenderedPreview(previewRequest())
    const assurance = screen.getByTestId('local-only-assurance')
    expect(assurance.querySelectorAll('button, input, select, a')).toHaveLength(0)
    expect(assurance.tagName).toBe('SPAN')
    for (const attribute of ['role', 'aria-live', 'tabindex']) expect(assurance).not.toHaveAttribute(attribute)
  })
})

// STORY 17.1: THE CANVAS FOLLOWS THE CONTENT FIELD.
//
// TWENTY-THREE tests. NINE are the story's I/O matrix, one per row. The other
// fourteen are rows the matrix does not have, and mutation testing or review
// demanded every one of them: the late
// echo on the BLUR path (the reproduction the story's Code Map measured at
// c13864c, where the hazard was live before any debounce existed); a blur
// landing while a debounced command is IN FLIGHT, which lost the author's text
// outright until review caught it; the `disable = false` decision, whose
// consequence jsdom cannot see; a paste, which takes the same debounce; the
// dead-panel drain; ownership of the draft returning to the engine at dispatch
// and at a no-op pause; a refusal clearing itself when the author deletes back
// to the committed text; the suppression flag Escape raises being lowered
// again; the single-line rows NOT debouncing; and the two debounce constants'
// ordering; and this sentence's own count, which review caught claiming
// "Eleven" over fifteen tests and which is now read back off the source by the
// last test in the block, so it cannot quietly go stale again.
//
// THE MOCK IS THE POINT OF THIS BLOCK. Every prose test written before this
// story used a `request` that answers with NO canvas, so `applyProperties`
// returns `undefined`, and neither the committed transition nor `submit`'s
// reconciliation is ever reached. Those tests are blind to all three clobber
// sites by construction. The engine here ECHOES a canvas carrying the accepted
// value — and, on demand, echoes an OLDER value while a NEWER draft is held,
// which is the only shape in which the story's real risk is observable.
describe('Story 17.1: the canvas follows the content field', () => {
  // A paint the canvas can actually draw, so "the canvas shows the text" is
  // asserted against the CANVAS and not merely against the box the author
  // typed into. One engine line per commit; AD-17 means the browser never
  // decides where a break goes, so a stand-in engine may put them anywhere.
  const paintOf = (text: string) => ({ overflow: false, truncated: false, lines: text === '' ? [] : [{ top: 0, baseline: 10_000, advance: 14_000, width: 30_000, fragments: [{ text, x: 0 }] }] })
  const at = (text: string) => ({ ...canvas, components: [{ id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: text, textPaint: paintOf(text) }] })

  // `hold()` freezes the engine mid-command so a test can type ACROSS an
  // in-flight commit, and `release()` lets the answer land — late, and about
  // text the author has typed past. `refusal` stands in for the engine's
  // format validation on the `expression` command: measured at c13864c over
  // the whole ladder, `{{`, `{{c`, `{{cust` and `{{customer.name` are refused
  // with "component properties did not pass format validation" while `{`,
  // `{{customer.name}}` and `}}` are accepted. No Go runs in this file, so the
  // double reproduces the SHAPE of that refusal; what the test owns for itself
  // is the browser-side fact — which command a half-typed placeholder routes
  // to, and what the panel does with the answer.
  const proseEngine = (refusal?: (fields: Record<string, { op: string; value: string }>) => string | undefined, normalise: (text: string) => string = (text) => text) => {
    const sent: string[] = []
    let held: (() => void) | undefined
    let holding = false
    let revision = 1
    let engineValue = 'Hello'
    const answer = () => ({ snapshot: { documentState: 'loaded' as const, revision, byteLength: 3, canvas: at(engineValue) } })
    const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
      if (operation !== 'command' || payload === undefined) return answer()
      const wire = new TextDecoder().decode(payload)
      sent.push(wire)
      if (holding) await new Promise<void>((resolve) => { held = resolve })
      const parsed = JSON.parse(wire) as { kind: string; segments?: string[] }
      // AN EDIT THIS FIELD DID NOT MAKE. Binding a data path is the one surface
      // that rewrites a text component's `value` while the selection — and so
      // the panel, and so this field's own state — stays exactly where it was.
      // It is the only way to observe what the box does when the DOCUMENT moves
      // under it: undo deselects, and a selection change remounts the panel.
      if (parsed.kind === 'bindComponentScalar') {
        engineValue = `{{${(parsed.segments ?? []).join('.')}}}`
        revision += 1
        return answer()
      }
      const fields = JSON.parse(wire).changes as Record<string, { op: string; value: string }>
      const message = refusal?.(fields)
      if (message !== undefined) throw Object.assign(new Error(message), { elementId: 'e1' })
      engineValue = normalise((fields.value ?? fields.expression)!.value)
      revision += 1
      return answer()
    })
    return {
      sent,
      request,
      hold: () => { holding = true },
      release: () => { holding = false; held?.(); held = undefined },
      engineHolds: () => engineValue,
    }
  }
  const changes = (wire: string) => JSON.parse(wire).changes as Record<string, { op: string; value: string }>
  // The two things a test does between fake-timer steps: let the debounce
  // elapse, and let a released promise settle. Both inside `act`, so React has
  // committed before anything is asserted. `waitFor` is deliberately NOT used
  // here — with fake timers it advances them itself, which would fire the very
  // debounce several of these rows exist to prove does not fire.
  const elapse = async (ms = PROSE_COMMIT_DEBOUNCE_MS) => { await act(async () => { await vi.advanceTimersByTimeAsync(ms) }) }
  const settle = async () => { await act(async () => { await Promise.resolve(); await Promise.resolve(); await Promise.resolve() }) }
  const openEditor = (request: ReturnType<typeof proseEngine>['request']) => {
    const view = render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at('Hello') }} />)
    // The accessible name of a painted component CARRIES ITS TEXT, so the
    // component is reached by the part of the name that does not move.
    fireEvent.click(screen.getByLabelText(/^text component e1/))
    const field = screen.getByRole('textbox', { name: 'Text' }) as HTMLTextAreaElement
    field.focus()
    return { view, field }
  }
  const type = (field: HTMLTextAreaElement, text: string) => fireEvent.change(field, { target: { value: text } })
  // THE ONLY EDIT IN THIS PANEL THAT MOVES `value` WITHOUT THIS FIELD SENDING
  // IT, and without taking the panel away. Undo cannot serve: it passes
  // `clearDocumentInteraction` and empties the selection (App.tsx:1327), which
  // unmounts the inspector. A selection change remounts it too, through
  // `ComponentProperties`' key. So binding a data path from the DATA tab is how
  // the two guards below are reached at all — both are about what the box does
  // when the DOCUMENT moves under a draft the author has touched.
  const openWithSampleData = (request: ReturnType<typeof proseEngine>['request']) => {
    const sampleData = acceptSampleData('keys.json', new TextEncoder().encode('{"customer":{"name":"Ada"}}').buffer)
    render(<App engine={engine(request as never)} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: at('Hello') }} initialSampleData={sampleData} />)
    fireEvent.click(screen.getByLabelText(/^text component e1/))
    const field = screen.getByRole('textbox', { name: 'Text' }) as HTMLTextAreaElement
    field.focus()
    return field
  }
  const bindCustomerName = () => {
    fireEvent.click(screen.getByRole('tab', { name: 'DATA' }))
    const customer = screen.getAllByRole('treeitem').find((item) => item.getAttribute('aria-level') === '2' && item.textContent?.startsWith('customer'))!
    customer.focus()
    fireEvent.keyDown(customer, { key: 'ArrowRight' })
    const name = screen.getAllByRole('treeitem').find((item) => item.getAttribute('aria-level') === '3' && item.textContent?.startsWith('name'))!
    name.focus()
    fireEvent.keyDown(name, { key: 'Enter' })
    // STORY 14.6 — A PICK BINDS IMMEDIATELY (owner ruling). The Enter above IS
    // the bind; the intermediate "Connect selected path" control is gone.
  }

  beforeEach(() => { vi.useFakeTimers() })
  afterEach(() => { vi.useRealTimers() })

  // MATRIX ROW 1, and the story's first acceptance criterion whole: the canvas
  // paints the typed text WITHOUT focus leaving the field.
  it('paints the typed text on the canvas after a pause, without the author leaving the field', async () => {
    const { sent, request, engineHolds } = proseEngine()
    const { field } = openEditor(request)
    type(field, 'Invoice')
    // Before the debounce elapses nothing has been sent — this is a pause, not
    // a keystroke, and the story may not claim per-keystroke painting.
    expect(sent).toHaveLength(0)
    await elapse()
    await settle()
    expect(sent).toHaveLength(1)
    expect(changes(sent[0]!).value).toEqual({ op: 'set', value: 'Invoice' })
    expect(engineHolds()).toBe('Invoice')
    // THE CANVAS, not the box: the projected paint the engine returned is what
    // is on screen, and the author never blurred to get it.
    expect(screen.getByLabelText('text component e1: Invoice')).toBeInTheDocument()
    expect(document.activeElement).toBe(field)
    expect(field).toHaveValue('Invoice')
  })

  // MATRIX ROW 2 / AC2. Seven characters faster than the debounce is ONE
  // command, not seven — the whole reason this is a debounce and not an
  // onChange commit, since every command is a revision and an undo entry.
  it('sends exactly one command for seven characters typed faster than the debounce', async () => {
    const { sent, request } = proseEngine()
    const { field } = openEditor(request)
    for (const text of ['I', 'In', 'Inv', 'Invo', 'Invoi', 'Invoic', 'Invoice']) {
      type(field, text)
      await elapse(PROSE_COMMIT_DEBOUNCE_MS - 50)
    }
    expect(sent).toHaveLength(0)
    await elapse()
    await settle()
    expect(sent).toHaveLength(1)
    expect(changes(sent[0]!).value!.value).toBe('Invoice')
  })

  // MATRIX ROW 3. `submit`'s early return is right for a click — an author
  // cannot mean two — and wrong for a timer, where it means the last thing
  // typed never reaches the engine at all.
  it('does not drop a debounced commit that collides with one already in flight', async () => {
    const { sent, request, hold, release, engineHolds } = proseEngine()
    const { field } = openEditor(request)
    hold()
    type(field, 'Invoice')
    await elapse()
    expect(sent).toHaveLength(1)
    // The author types on while the first command is awaited, and its own
    // debounce elapses against a busy panel.
    type(field, 'Invoice 2026')
    await elapse()
    expect(sent).toHaveLength(1)
    release()
    await settle()
    // Nothing was silently discarded: the later text went as its own command.
    expect(sent).toHaveLength(2)
    expect(changes(sent[1]!).value!.value).toBe('Invoice 2026')
    expect(engineHolds()).toBe('Invoice 2026')
    expect(field).toHaveValue('Invoice 2026')
  })

  // MATRIX ROW 4. THE ECHO IS OLDER THAN THE DRAFT. The author's text wins.
  it('keeps the draft the author is holding when an older echo lands', async () => {
    const { sent, request, hold, release } = proseEngine()
    const { field } = openEditor(request)
    hold()
    type(field, 'Invoice')
    await elapse()
    expect(sent).toHaveLength(1)
    type(field, 'Invoice 2026')
    release()
    await settle()
    // The engine has just said `Invoice`. It is answering about text the
    // author moved past, and it may not overwrite it.
    expect(field).toHaveValue('Invoice 2026')
    expect(screen.getByLabelText('text component e1: Invoice')).toBeInTheDocument()
    // And the author's text is not merely preserved on screen — it reaches the
    // engine on the next debounce, so no character was lost anywhere.
    await elapse()
    await settle()
    expect(changes(sent[1]!).value!.value).toBe('Invoice 2026')
    expect(screen.getByLabelText('text component e1: Invoice 2026')).toBeInTheDocument()
  })

  // THE SAME HAZARD ON THE BLUR PATH, which is where the story's Code Map
  // MEASURED it at c13864c: typing `Invoice`, blurring, typing `Invoice 2026`
  // while the command was still in flight, then releasing the engine, left the
  // field reading `Invoice` — the later text destroyed. The debounce did not
  // create this; it only makes it ordinary. A blur commits with
  // `reconcileDraft` TRUE, so this row crosses `submit`'s reconciliation as
  // well as the committed transition.
  it('keeps a newer draft when a BLUR commit echoes an older value back', async () => {
    const { sent, request, hold, release } = proseEngine()
    const { field } = openEditor(request)
    hold()
    type(field, 'Invoice')
    fireEvent.blur(field)
    await settle()
    expect(sent).toHaveLength(1)
    field.focus()
    type(field, 'Invoice 2026')
    release()
    await settle()
    expect(field).toHaveValue('Invoice 2026')
  })

  // THE DEFECT REVIEW FOUND, AND THE ONE THAT ACTUALLY LOST TEXT.
  //
  // Every other row in this block happens to `elapse()` before it blurs, which
  // is exactly why they all stayed green over it. Blur while a DEBOUNCED
  // command is still in flight and two halves of the control conspire: `blur`
  // cancels the armed timer that would have queued the text, and then `commit`
  // is swallowed by `submit`'s in-flight guard. Measured before the fix —
  // `sent` = 1, the engine holding `Invoice`, and the field showing
  // `Invoice 2026` that nothing would ever reconcile.
  //
  // The assertion is deliberately on the ENGINE and not only on the command
  // count: what the story promises is that the author's characters arrive, not
  // that a second payload was constructed.
  it('does not lose text typed across an in-flight command when the author blurs instead of pausing', async () => {
    const { sent, request, hold, release, engineHolds } = proseEngine()
    const { field } = openEditor(request)
    hold()
    type(field, 'Invoice')
    await elapse()
    expect(sent).toHaveLength(1)
    // Typed across the in-flight command, then the author leaves the field
    // WITHOUT pausing — so no timer of this text's own ever fires.
    type(field, 'Invoice 2026')
    fireEvent.blur(field)
    release()
    await settle()
    expect(sent).toHaveLength(2)
    expect(changes(sent[1]!).value!.value).toBe('Invoice 2026')
    expect(engineHolds()).toBe('Invoice 2026')
    expect(field).toHaveValue('Invoice 2026')
  })

  // MATRIX ROW 5. Blur commits, as it always has, and the timer that was about
  // to fire for the same text does not become a second command.
  it('sends exactly one command when a blur lands on top of a pending debounce, and cancels the timer', async () => {
    const { sent, request } = proseEngine()
    const { field } = openEditor(request)
    // The designer schedules zero-delay timers of its own (the canvas focus
    // hand-off), so they are flushed first and the count below is the
    // debounce alone.
    await elapse(0)
    type(field, 'Invoice')
    // THE TIMER ITSELF, not merely its consequence. A blur that commits and
    // leaves the timer armed still produces one command here — the sender
    // declines to repeat text it has already sent — so counting commands alone
    // cannot tell a cancelled timer from a neutered one, and a timer left
    // armed is one that can fire against a later `committed`.
    expect(vi.getTimerCount()).toBe(1)
    // Mid-timer: the debounce has not elapsed.
    await elapse(PROSE_COMMIT_DEBOUNCE_MS - 50)
    fireEvent.blur(field)
    await elapse(0)
    expect(vi.getTimerCount()).toBe(0)
    await settle()
    expect(sent).toHaveLength(1)
    await elapse(PROSE_COMMIT_DEBOUNCE_MS * 2)
    await settle()
    expect(sent).toHaveLength(1)
    expect(changes(sent[0]!).value!.value).toBe('Invoice')
  })

  // THE PASTE PATH TAKES THE SAME DEBOUNCE. A clipboard is a typed edit by
  // another name — it changes the draft, and Story 7.4 exists precisely
  // because authors paste whole clauses in. Leaving it out would have made the
  // canvas follow typing but not pasting, with nothing to say why.
  it('debounces a paste into the prose field exactly as it debounces typing', async () => {
    const { sent, request, engineHolds } = proseEngine()
    const { field } = openEditor(request)
    // NOTHING IS TYPED FIRST, deliberately. A `change` before the paste would
    // arm the debounce by itself, and the timer it left would pick up the
    // pasted text when it fired — a paste that schedules nothing would still
    // look green. The caret is placed instead, which is also what makes this a
    // mid-field paste rather than a replacement.
    await elapse(0)
    field.setSelectionRange(5, 5)
    fireEvent.paste(field, { clipboardData: { getData: () => ['', 'Clause 2.'].join('\n') } })
    expect(field).toHaveValue(['Hello', 'Clause 2.'].join('\n'))
    expect(sent).toHaveLength(0)
    await elapse()
    await settle()
    expect(sent).toHaveLength(1)
    expect(engineHolds()).toBe(['Hello', 'Clause 2.'].join('\n'))
  })

  // MATRIX ROW 6 / AC4. THE FIELD IS FOCUSED, which is the whole test.
  //
  // The Story 7.4 test for this row (`reverts and blurs on Escape`) never
  // focuses, so `.blur()` is a no-op, no blur event fires, and its
  // `expect(sent).toHaveLength(0)` passes vacuously. MEASURED at c13864c with
  // a focused field: Escape sent one command carrying the unwanted draft and
  // did NOT revert. That test is left in place unchanged; this is the row that
  // bites.
  it('sends nothing and reverts when Escape is pressed in a FOCUSED field mid-typing', async () => {
    const { sent, request } = proseEngine()
    const { field } = openEditor(request)
    expect(document.activeElement).toBe(field)
    await elapse(0)
    type(field, ['a draft', 'nobody wants'].join('\n'))
    expect(vi.getTimerCount()).toBe(1)
    fireEvent.keyDown(field, { key: 'Escape' })
    // The matrix's own words for this row are "Timer cancelled", so the timer
    // is what is asserted — not just that nothing was sent, which stays true of
    // a timer that survives and then declines to act. The extra `elapse(0)`
    // discards the zero-delay focus timer Escape's blur hands the canvas.
    await elapse(0)
    expect(vi.getTimerCount()).toBe(0)
    // Escape's own synchronous blur must not commit the draft it just threw away.
    await settle()
    expect(field).toHaveValue('Hello')
    expect(sent).toHaveLength(0)
    // And no debounced command follows it: the timer went with the draft.
    await elapse(PROSE_COMMIT_DEBOUNCE_MS * 2)
    await settle()
    expect(sent).toHaveLength(0)
    expect(field).toHaveValue('Hello')
  })

  // THE OTHER HALF OF ESCAPE'S SUPPRESSION FLAG. Escape blurs, and the flag
  // exists so the blur it dispatches does not commit the draft Escape just
  // threw away — but on an UNFOCUSED field `.blur()` dispatches nothing, so a
  // flag that is raised and not lowered again stays raised, and the author's
  // NEXT real blur is silently swallowed. Story 7.4's Escape test is exactly
  // that unfocused shape, which is why this failure mode is reachable at all.
  it('does not swallow a later blur after an Escape that had nothing to blur', async () => {
    const { sent, request, engineHolds } = proseEngine()
    const { field } = openEditor(request)
    // Unfocused, the way Story 7.4's Escape test leaves it.
    field.blur()
    await settle()
    expect(sent).toHaveLength(0)
    fireEvent.keyDown(field, { key: 'Escape' })
    await settle()
    expect(sent).toHaveLength(0)
    // A real edit, and a real blur, which must still commit.
    field.focus()
    type(field, 'Invoice')
    fireEvent.blur(field)
    await settle()
    expect(sent).toHaveLength(1)
    expect(engineHolds()).toBe('Invoice')
  })

  // MATRIX ROW 7. A timer may not outlive the panel it was scheduled in.
  // The console spy below is NOT what carries this row, and the test is no
  // longer named as though it were. React is 19.2.0 and the "state update on an
  // unmounted component" warning was removed in React 18, so
  // `expect(complaints).toEqual([])` cannot fail on that account; it is kept
  // only to catch an unrelated console error appearing. What carries the row is
  // the timer count and the absence of a command.
  it('clears the timer on unmount, so no command fires from a dead panel', async () => {
    const { sent, request } = proseEngine()
    const { view, field } = openEditor(request)
    await elapse(0)
    type(field, 'Invoice')
    expect(vi.getTimerCount()).toBe(1)
    const complaints: unknown[][] = []
    const complained = vi.spyOn(console, 'error').mockImplementation((...args: unknown[]) => { complaints.push(args) })
    try {
      view.unmount()
      // CLEARED, not merely disarmed. The matrix says "Timer cleared", and a
      // timer that survives unmount is a reference to a dead render tree held
      // until it fires.
      await elapse(0)
      expect(vi.getTimerCount()).toBe(0)
      await elapse(PROSE_COMMIT_DEBOUNCE_MS * 4)
      await settle()
      expect(sent).toHaveLength(0)
      expect(complaints).toEqual([])
    } finally { complained.mockRestore() }
  })

  // MATRIX ROW 7, THE HALF A CLEARED TIMER DOES NOT COVER. A command already
  // in flight when the panel unmounts keeps its own continuation — that
  // closure is not a timer and nothing cancels it — and the QUEUED debounce
  // behind it would be drained from there. Clearing the timer on unmount is
  // not enough; the drain has to know the panel is gone.
  it('does not drain a queued debounce out of a dead panel', async () => {
    const { sent, request, hold, release } = proseEngine()
    const { view, field } = openEditor(request)
    hold()
    type(field, 'Invoice')
    await elapse()
    expect(sent).toHaveLength(1)
    // Queued behind the in-flight command, then the selection goes away.
    type(field, 'Invoice 2026')
    await elapse()
    expect(sent).toHaveLength(1)
    view.unmount()
    release()
    await settle()
    expect(sent).toHaveLength(1)
  })

  // THE OTHER HALF OF `unsentEdit`, AND THE ONE THE MATRIX DOES NOT REACH:
  // ownership goes BACK to the engine the moment a command carrying the text
  // is dispatched. A guard that is raised and never lowered would keep the
  // author's draft safe from every echo forever — including the ones that are
  // not stale at all — and the box would then go on showing text the document
  // does not hold, silently, with every gate green.
  //
  // The double normalises here (a trailing space is dropped) precisely so the
  // engine's answer is DISTINGUISHABLE from what was typed. When the engine and
  // the author agree character for character, as they do on the plain `value`
  // path, a reconciliation that never happens is invisible.
  it('lets the engine own the draft again once the text it holds has been sent', async () => {
    const { sent, request, engineHolds } = proseEngine(undefined, (text) => text.trimEnd())
    const { field } = openEditor(request)
    type(field, 'Invoice   ')
    await elapse()
    await settle()
    expect(sent).toHaveLength(1)
    expect(changes(sent[0]!).value!.value).toBe('Invoice   ')
    expect(engineHolds()).toBe('Invoice')
    // The author is holding nothing unsent, so the document is the authority
    // again and the box reads what the document actually holds.
    expect(field).toHaveValue('Invoice')
  })

  // MATRIX ROW 8. A half-typed `{{` now reaches the engine mid-word where it
  // never did before, and the DECIDED behaviour is to send it and let the
  // existing refusal path render. Suppressing the send would mean
  // re-implementing the engine's placeholder grammar in the browser — a
  // mirrored invariant this story carries no ruling for.
  //
  // THE SELF-CLEARING HALF IS NOT OPTIONAL: an author who must cross four
  // refusals on the way to a valid expression has to see the last one go.
  it('sends a half-typed placeholder as an expression, shows the refusal, and clears it when the text completes', async () => {
    const refusal = (fields: Record<string, { op: string; value: string }>) => fields.expression && !/^\{\{[^{}]*\}\}$/.test(fields.expression.value) ? 'component properties did not pass format validation' : undefined
    const { sent, request, engineHolds } = proseEngine(refusal)
    const { field } = openEditor(request)
    type(field, '{{cust')
    await elapse()
    await settle()
    // The BROWSER-side fact this test owns: `contentCommand` routed a
    // half-typed placeholder to `expression`, not to `value`.
    expect(Object.keys(changes(sent[0]!))).toEqual(['expression'])
    expect(screen.getByRole('alert')).toHaveTextContent('component properties did not pass format validation')
    // Typing is not blocked while the refusal stands — no `disabled`, and the
    // field still takes a keystroke.
    expect(field).not.toBeDisabled()
    expect(field).toHaveValue('{{cust')
    type(field, '{{customer.name}}')
    await elapse()
    await settle()
    expect(sent).toHaveLength(2)
    expect(engineHolds()).toBe('{{customer.name}}')
    // The alert cleared ITSELF on the next successful debounce.
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  // MATRIX ROW 9. A refusal that is not about placeholders at all: the
  // existing error path renders, and typing carries on over the top of it.
  it('renders the existing error path when the engine refuses, and does not block typing', async () => {
    const refusal = (fields: Record<string, { op: string; value: string }>) => fields.value?.value.endsWith(' ') ? 'value must not end in a space' : undefined
    const { sent, request, engineHolds } = proseEngine(refusal)
    const { field } = openEditor(request)
    type(field, 'Invoice ')
    await elapse()
    await settle()
    expect(screen.getByRole('alert')).toHaveTextContent('e1: value must not end in a space')
    expect(field).toHaveAttribute('aria-invalid', 'true')
    expect(field).not.toBeDisabled()
    // The refused text is still the author's, and still theirs to fix.
    expect(field).toHaveValue('Invoice ')
    type(field, 'Invoice')
    await elapse()
    await settle()
    expect(sent).toHaveLength(2)
    expect(engineHolds()).toBe('Invoice')
    expect(field).not.toHaveAttribute('aria-invalid')
  })

  // PATCH 2 (review). THE ALERT MUST CLEAR WHEN THE AUTHOR TAKES THE OFFENDING
  // TEXT BACK OUT, and it is the whole justification for sending half-typed
  // placeholders at all — "the existing refusal path renders" is only humane if
  // the author can see it go.
  //
  // The trap is that deleting back lands the draft on the text the DOCUMENT
  // already holds, so an early return keyed on `committed` sends nothing,
  // `applyProperties` never runs its `setPropertyError(undefined)`, and the
  // refusal sits there describing text that is no longer in the box. Keying on
  // what the engine was last TOLD is what distinguishes the two: it was told
  // `Invoice}}`, the document holds `Invoice`, and the difference is a command.
  it('clears a refusal when the author deletes the offending text back to the committed value', async () => {
    const refusal = (fields: Record<string, { op: string; value: string }>) => fields.expression && !/^\{\{[^{}]*\}\}$/.test(fields.expression.value) ? 'component properties did not pass format validation' : undefined
    const { sent, request, engineHolds } = proseEngine(refusal)
    const { field } = openEditor(request)
    type(field, 'Invoice')
    await elapse()
    await settle()
    expect(engineHolds()).toBe('Invoice')
    // Refused: a lone `}}` routes to `expression` and does not spell one.
    type(field, 'Invoice}}')
    await elapse()
    await settle()
    expect(screen.getByRole('alert')).toHaveTextContent('component properties did not pass format validation')
    // Straight back to the text the document already holds.
    type(field, 'Invoice')
    await elapse()
    await settle()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(field).not.toHaveAttribute('aria-invalid')
    expect(engineHolds()).toBe('Invoice')
    expect(sent).toHaveLength(3)
  })

  // PATCH 3 (review). A PAUSE THAT SENDS NOTHING MUST STILL HAND THE DRAFT BACK
  // TO THE ENGINE. `unsentEdit` is raised by every keystroke and lowered at
  // dispatch — so a pause that decides there is nothing to dispatch would leave
  // it raised for the life of the selection, and the box would stop following
  // the engine from then on, silently.
  //
  // Reaching it without an external editing surface: type ACROSS an in-flight
  // command and then back to exactly the text that command carried. The pause
  // then has nothing to send, and the engine's own answer — normalised here, so
  // it is distinguishable from what was typed — must still land in the box.
  it('hands the draft back to the engine after a pause that finds nothing to send', async () => {
    const { sent, request, hold, release } = proseEngine(undefined, (text) => text.trimEnd())
    const { field } = openEditor(request)
    hold()
    type(field, 'Invoice   ')
    await elapse()
    expect(sent).toHaveLength(1)
    // Away and back again while the command is awaited: the draft ends on the
    // text the engine was already told, so this pause sends nothing.
    type(field, 'Invoice   X')
    type(field, 'Invoice   ')
    await elapse()
    expect(sent).toHaveLength(1)
    release()
    await settle()
    // The engine normalised it. The box has to follow, which it can only do if
    // the pause above put ownership back.
    expect(field).toHaveValue('Invoice')
    expect(sent).toHaveLength(1)
  })

  // PATCH 4 (review). THE SCOPE BOUNDARY, WHICH NOTHING ELSE PINNED. The
  // spec's Ask First names "applying the debounce to any field other than the
  // prose content field", and that is a boundary a test has to hold: adding
  // `scheduleProseCommit()` to the single-line `<input>`'s `onChange` reddened
  // NOTHING in this block before this row existed.
  it('does not debounce a single-line field — only the prose one', async () => {
    const { sent, request } = proseEngine()
    openEditor(request)
    await elapse(0)
    const width = screen.getByRole('textbox', { name: 'Width (pt)' })
    fireEvent.change(width, { target: { value: '96' } })
    // No timer was armed at all, and none fires however long is waited.
    expect(vi.getTimerCount()).toBe(0)
    await elapse(PROSE_COMMIT_DEBOUNCE_MS * 5)
    await settle()
    expect(sent).toHaveLength(0)
    expect(width).toHaveValue('96')
    // The contrast arm, so this cannot pass over a panel that never commits:
    // the same field still commits on Enter, exactly as it always has.
    fireEvent.keyDown(width, { key: 'Enter' })
    await settle()
    expect(sent).toHaveLength(1)
    expect(changes(sent[0]!).width!.value).toBe(96)
  })

  // THE COUNT IN THIS BLOCK'S HEADER, PINNED. Review found that header claiming
  // "Eleven tests" over fifteen of them — a comment that had simply been left
  // behind, in a repo that treats a false comment as a defect. A prose count
  // cannot be maintained by intention, so it is read back off the source.
  it('states its own test count truthfully in its header', () => {
    // Read from the runner's working directory, not `import.meta.url`: the
    // react plugin rewrites that to a non-file URL in this `.tsx` file, which
    // `readFileSync` refuses.
    const source = fs.readFileSync('src/App.test.tsx', 'utf8')
    const start = source.indexOf("describe('Story 17.1")
    // The LAST all-caps "N tests." sentence before the describe is this block's
    // header; no other header in this file is written that way.
    const declared = [...source.slice(0, start).matchAll(/\/\/ ([A-Z-]+) tests\./g)].at(-1)
    expect(declared).toBeDefined()
    const spelled: Record<string, number> = { TEN: 10, ELEVEN: 11, TWELVE: 12, THIRTEEN: 13, FOURTEEN: 14, FIFTEEN: 15, SIXTEEN: 16, SEVENTEEN: 17, EIGHTEEN: 18, NINETEEN: 19, TWENTY: 20, 'TWENTY-ONE': 21, 'TWENTY-TWO': 22, 'TWENTY-THREE': 23 }
    const written = spelled[declared![1] as string]
    expect(written).toBeDefined()
    const end = source.indexOf("\ndescribe('", start + 1)
    expect(written).toBe((source.slice(start, end < 0 ? undefined : end).match(/\n {2}it\(/g) ?? []).length)
  })

  // A BLUR THAT SENDS NOTHING MUST STILL HAND THE DRAFT BACK.
  //
  // The pause path lowers the ownership flag when it finds nothing to send, but
  // blur CANCELS that pause — so a draft typed and then deleted back to the
  // committed text before the author clicks away never reaches it, `commit`
  // takes its no-op branch, and the flag would stay raised for the life of the
  // selection. Nothing after that could make the box follow the document again.
  //
  // Deliberately no `elapse()` before the blur: letting the debounce fire first
  // would lower the flag on the pause path and hide whether blur does its half.
  it('hands the draft back to the engine when a blur finds nothing to send', async () => {
    const { sent, request } = proseEngine()
    const field = openWithSampleData(request)
    type(field, 'Helloz')
    type(field, 'Hello')
    fireEvent.blur(field)
    await settle()
    expect(sent).toHaveLength(0)
    // The document now moves on its own. The box has to follow it.
    bindCustomerName()
    await settle()
    expect(field).toHaveValue('{{customer.name}}')
  })

  // WHAT THE ENGINE WAS TOLD HAS TO BE FORGOTTEN WHEN THE DOCUMENT MOVES
  // WITHOUT IT. `toldEngine` is what suppresses a duplicate command; left
  // pointing at text the document no longer holds, it suppresses a REAL one,
  // and the box and the document disagree with nothing to reconcile them.
  it('sends text again after the document has moved underneath it', async () => {
    const { sent, request, engineHolds } = proseEngine()
    const field = openWithSampleData(request)
    type(field, 'Invoice')
    await elapse()
    await settle()
    expect(sent).toHaveLength(1)
    expect(engineHolds()).toBe('Invoice')
    bindCustomerName()
    await settle()
    expect(field).toHaveValue('{{customer.name}}')
    // The author types the old text back. The document does NOT hold it any
    // more, so this is a real edit and it has to travel.
    type(field, 'Invoice')
    await elapse()
    await settle()
    // Three payloads on the wire: the first commit, the bind, and this one.
    expect(sent).toHaveLength(3)
    expect(changes(sent[2]!).value!.value).toBe('Invoice')
    expect(engineHolds()).toBe('Invoice')
    expect(field).toHaveValue('Invoice')
  })

  // PATCH 6 (review). The ordering of the two debounces was asserted in a
  // COMMENT that spelled the preview's number out, where it would rot the
  // moment either constant moved. It is asserted here instead, over the
  // constants themselves.
  it('pauses for less time than the PDF preview does', () => {
    expect(PROSE_COMMIT_DEBOUNCE_MS).toBeLessThan(PREVIEW_DEBOUNCE_MS)
  })

  // THE DECISION THE STORY'S CODE MAP FORCED, tied to a test so it cannot be
  // undone silently. `shared` carries `disabled: pending`, and a debounced
  // commit passes `disable = false` for the same reason Story 17.4's arrow
  // step does: disabling a FOCUSED input moves focus to the body and does not
  // give it back when the input is re-enabled (measured in Chromium 1217).
  // With the default, every debounce would blank the author's focus
  // mid-sentence and AC1 would be false in a real browser.
  //
  // JSDOM CANNOT SEE THE FOCUS THEFT — measured: the textarea reports
  // `disabled: true` mid-flight while `document.activeElement` stays the
  // textarea. So the reachable assertion is the `disabled` attribute itself,
  // in BOTH directions against blur, which still disables. The browser run is
  // the only place the theft is observable.
  it('does not disable the field while a DEBOUNCED commit is in flight, though a blur commit still does', async () => {
    const { sent, request, hold, release } = proseEngine()
    const { field } = openEditor(request)
    hold()
    type(field, 'Invoice')
    await elapse()
    expect(sent).toHaveLength(1)
    expect(screen.getByRole('textbox', { name: 'Text' })).not.toBeDisabled()
    release()
    await settle()
    // The contrast arm: without it this would pass over a control that never
    // disables for any reason.
    hold()
    type(field, 'Invoice 2026')
    fireEvent.blur(field)
    await settle()
    expect(sent).toHaveLength(2)
    expect(screen.getByRole('textbox', { name: 'Text' })).toBeDisabled()
    release()
    await settle()
    expect(screen.getByRole('textbox', { name: 'Text' })).not.toBeDisabled()
  })
})

describe('common property selection scope', () => {
  const textComponent = { id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72000, height: 24000, resizable: true, value: 'Sample' }
  const absent = { state: 'absent' as const }
  const authored = { visibleIf: absent, fontFamily: absent, fontSize: absent, lineSpacing: absent, bold: absent, italic: absent, align: absent, valign: absent, color: absent, background: absent, borderWidth: absent, borderColor: absent, borderEdges: absent, errorCorrection: absent }
  const members = [
    { ...textComponent, id: 'e1', authored },
    { ...textComponent, id: 'e2', x: 100000, y: 50000, authored },
    { ...textComponent, id: 'e3', x: 200000, y: 100000, authored },
  ]
  const selectPair = () => { fireEvent.click(screen.getByLabelText('text component e1')); fireEvent.click(screen.getByLabelText('text component e2'), { shiftKey: true }) }

  it('reconciles clearing divergent sizes to the shared inherited value without a blur edit', async () => {
    const components = members.slice(0, 2).map((component, i) => ({ ...component, authored: { ...authored, fontSize: { state: 'value' as const, value: (i + 1) * 10000 } } }))
    const snapshot = { documentState: 'loaded' as const, revision: 1, byteLength: 1, canvas: { ...canvas, components } }
    const request = vi.fn(async () => ({ snapshot: { ...snapshot, revision: 2, canvas: { ...canvas, components: members.slice(0, 2) } } }))
    render(<App engine={engine(request as never)} initialSnapshot={snapshot} />)
    selectPair()
    const size = screen.getByRole('textbox', { name: 'Font size (pt)' })
    expect(size).toHaveValue('')
    fireEvent.click(screen.getByRole('button', { name: 'Clear Font size (pt)' }))
    await waitFor(() => expect(size).toHaveValue('12'))
    fireEvent.focus(size); fireEvent.blur(size)
    expect(request).toHaveBeenCalledTimes(1)
  })

  it('uses the existing clear operation when the final border edge is unchecked', async () => {
    const components = members.slice(0, 2).map((component) => ({ ...component, authored: { ...authored, borderEdges: { state: 'value' as const, value: ['bottom'] as const } } }))
    const snapshot = { documentState: 'loaded' as const, revision: 1, byteLength: 1, canvas: { ...canvas, components } }
    const request = vi.fn(async () => ({ snapshot: { ...snapshot, revision: 2, canvas: { ...canvas, components: members.slice(0, 2) } } }))
    render(<App engine={engine(request as never)} initialSnapshot={snapshot} />)
    selectPair()
    fireEvent.click(screen.getByRole('checkbox', { name: 'Border bottom' }))
    await waitFor(() => expect(screen.getByRole('checkbox', { name: 'Border bottom' })).not.toBeChecked())
    const calls = request.mock.calls as unknown as [string, ArrayBuffer][]
    expect(JSON.parse(new TextDecoder().decode(calls[0]![1]))).toMatchObject({ ids: ['e1', 'e2'], changes: { borderEdges: { op: 'clear' } } })
  })

  it('keeps untouched shared, mixed and inherited fields inert in multi-selection', () => {
    const sent: ArrayBuffer[] = []
    const snapshot = { documentState: 'loaded' as const, revision: 1, byteLength: 1, canvas: { ...canvas, components: members } }
    const request = vi.fn(async (_op: string, payload?: ArrayBuffer) => { if (payload) sent.push(payload); return { snapshot } })
    render(<App engine={engine(request as never)} initialSnapshot={snapshot} />)
    selectPair()
    for (const name of ['X (pt)', 'Y (pt)', 'Width (pt)', 'Font size (pt)', 'Line spacing', 'Background', 'Visible if']) {
      const field = screen.getByRole('textbox', { name })
      fireEvent.focus(field); fireEvent.blur(field); fireEvent.keyDown(field, { key: 'Enter' })
    }
    const x = screen.getByRole('textbox', { name: 'X (pt)' })
    fireEvent.change(x, { target: { value: '123' } }); fireEvent.change(x, { target: { value: '' } }); fireEvent.blur(x)
    expect(sent).toEqual([])
    expect(screen.getByTestId('engine-snapshot')).toHaveTextContent('REVISION 1')
  })

  it('distinguishes absence, null and explicit false; hides an arbitrary mixed swatch and offers Clear for null', () => {
    const components = [{ ...members[0]!, authored: { ...authored, background: { state: 'null' as const }, bold: { state: 'value' as const, value: false }, visibleIf: { state: 'null' as const } } }, { ...members[1]!, authored: { ...authored, visibleIf: { state: 'null' as const } } }]
    const snapshot = { documentState: 'loaded' as const, revision: 1, byteLength: 1, canvas: { ...canvas, components } }
    render(<App initialSnapshot={snapshot} />)
    selectPair()
    expect(screen.getByRole('textbox', { name: 'Background' })).toHaveAttribute('aria-description', 'Mixed value')
    expect(screen.getByRole('button', { name: 'Bold, mixed' })).toHaveAttribute('aria-pressed', 'mixed')
    expect(screen.getByLabelText('Pick Background')).toHaveClass('property-swatch-unset')
    expect(screen.getByRole('button', { name: 'Clear Visible if' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Set Visible if null' })).not.toBeInTheDocument()
  })

  it('keeps late blur and pending property results scoped to the captured selection', async () => {
    const pending: ((result: { snapshot: typeof snapshot }) => void)[] = []
    const sent: ArrayBuffer[] = []
    const snapshot = { documentState: 'loaded' as const, revision: 1, byteLength: 1, canvas: { ...canvas, components: members } }
    const request = vi.fn((_op: string, payload?: ArrayBuffer) => { if (payload) { sent.push(payload); return new Promise<{ snapshot: typeof snapshot }>((resolve) => pending.push(resolve)) }; return Promise.resolve({ snapshot }) })
    render(<App engine={engine(request as never)} initialSnapshot={snapshot} />)
    selectPair()
    const old = screen.getByRole('textbox', { name: 'Font size (pt)' })
    fireEvent.change(old, { target: { value: '18' } })
    fireEvent.click(screen.getByLabelText('text component e3'))
    fireEvent.blur(old)
    expect(sent).toHaveLength(0)
    selectPair()
    const size = screen.getByRole('textbox', { name: 'Font size (pt)' })
    fireEvent.change(size, { target: { value: '18' } }); fireEvent.keyDown(size, { key: 'Enter' })
    fireEvent.keyDown(size, { key: 'Enter' })
    expect(sent).toHaveLength(1)
    fireEvent.click(screen.getByLabelText('text component e3'))
    await act(async () => pending[0]!({ snapshot: { ...snapshot, revision: 2 } }))
    expect(screen.getByText('e3 · band: content')).toBeInTheDocument()
    expect(JSON.parse(new TextDecoder().decode(sent[0]!))).toMatchObject({ ids: ['e1', 'e2'], changes: { fontSize: { op: 'set', value: 18 } } })
  })

  it('does not install a delayed bulk response over a replaced document, even with a newer revision', async () => {
    const snapshot = { documentState: 'loaded' as const, revision: 1, byteLength: 1, canvas: { ...canvas, components: members } }
    let resolveCommit!: (result: { snapshot: typeof snapshot }) => void
    const request = vi.fn((operation: string) => operation === 'command' ? new Promise<{ snapshot: typeof snapshot }>((resolve) => { resolveCommit = resolve }) : Promise.resolve({ snapshot: { ...snapshot, revision: 3, canvas: { ...canvas, components: [members[2]!] } } }))
    render(<App engine={engine(request as never)} initialSnapshot={snapshot} blankBytes={new Uint8Array([1]).buffer} />)
    selectPair()
    const size = screen.getByRole('textbox', { name: 'Font size (pt)' })
    fireEvent.change(size, { target: { value: '18' } }); fireEvent.keyDown(size, { key: 'Enter' })
    startBlankFromNew()
    await waitFor(() => expect(screen.getByTestId('engine-snapshot')).toHaveTextContent('REVISION 3'))
    await act(async () => resolveCommit({ snapshot: { ...snapshot, revision: 99 } }))
    expect(screen.getByTestId('engine-snapshot')).toHaveTextContent('REVISION 3')
    expect(screen.queryByLabelText('text component e1')).not.toBeInTheDocument()
  })
})


// spec-section-break CAP-1 / CAP-6: the Section Break on the design canvas. It
// behaves like an element — placed from the palette, selected with its own
// state, dragged, nudged, typed and deleted — and every gesture is ONE engine
// command. The engine snaps and refuses; these rows pin what the canvas sends
// and what it draws from the projection.
describe('spec-section-break: the Section Break on the canvas', () => {
  const withBreak = (offset = 400_000, patch: Partial<CanvasProjection> = {}): CanvasProjection => ({ ...canvas, sectionBreak: offset, ...patch })
  const snapshotOf = (projection: CanvasProjection, revision = 1) => ({ documentState: 'loaded' as const, revision, byteLength: 3, canvas: projection })
  const open = (projection: CanvasProjection = canvas, answer: (operation: string) => Promise<unknown> = async () => ({ snapshot: snapshotOf(projection, 2) })) => {
    const request = vi.fn(answer)
    render(<App engine={engine(request as never)} initialSnapshot={snapshotOf(projection)} />)
    return request
  }
  const sent = (request: ReturnType<typeof vi.fn>) => (request.mock.calls as unknown as ReadonlyArray<[string, ArrayBuffer]>).filter(([operation]) => operation === 'command').map(([, payload]) => new TextDecoder().decode(payload))
  const handle = () => screen.getByRole('button', { name: 'Section Break' })
  const entry = () => screen.getByRole('button', { name: 'Place Section Break' })
  const settle = async () => { await act(async () => { await Promise.resolve() }) }

  it('has no break by default, and placing one sends one snapped command and selects the line', async () => {
    const request = open(canvas, async () => ({ snapshot: snapshotOf(withBreak(), 2) }))
    expect(screen.queryByRole('button', { name: 'Section Break' })).toBeNull()
    expect(entry()).toBeEnabled()
    fireEvent.click(entry())
    expect(entry()).toHaveAttribute('aria-pressed', 'true')
    fireEvent.keyDown(screen.getByLabelText('Content'), { key: 'Enter' })
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"setSectionBreak","version":1,"offset":364.945,"snap":true}']))
    await waitFor(() => expect(handle()).toHaveAttribute('aria-pressed', 'true'))
    expect(entry()).toBeDisabled()
    expect(screen.getByText('This document already has its one Section Break.')).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Y (pt)' })).toHaveValue('400')
    await waitFor(() => expect(document.activeElement).toBe(handle()))
  })

  it('places nothing when the armed entry is aimed at the page header or footer', async () => {
    const request = open()
    fireEvent.click(entry())
    fireEvent.keyDown(screen.getByLabelText('Page Header'), { key: 'Enter' })
    fireEvent.keyDown(screen.getByLabelText('Page Footer'), { key: 'Enter' })
    await settle()
    expect(sent(request)).toEqual([])
  })

  it('draws the projected line once, selects it with its own state, and deletes it with one command', async () => {
    const request = open(withBreak(), async () => ({ snapshot: snapshotOf(canvas, 2) }))
    expect(document.querySelectorAll('.section-break-line')).toHaveLength(1)
    expect((document.querySelector('.section-break-line') as HTMLElement).style.getPropertyValue('--section-break-display-y')).toBe('400px')
    expect(handle()).toHaveAttribute('aria-pressed', 'false')
    fireEvent.click(handle())
    expect(handle()).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByText('SECTION BREAK')).toBeInTheDocument()
    fireEvent.keyDown(handle(), { key: 'Delete' })
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"removeSectionBreak","version":1}']))
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Section Break' })).toBeNull())
    expect(entry()).toBeEnabled()
  })

  it('deletes a selected break from the window Backspace too, and never through the component delete', async () => {
    const request = open(withBreak())
    fireEvent.click(handle())
    fireEvent.keyDown(document.body, { key: 'Backspace' })
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"removeSectionBreak","version":1}']))
  })

  it('drags by clientY with a proposal, sending one snapped command on release', async () => {
    const request = open(withBreak())
    const strip = handle()
    fireEvent.pointerDown(strip, { pointerId: 1, button: 0, buttons: 1, clientY: 100 })
    fireEvent.pointerMove(strip, { pointerId: 1, buttons: 1, clientY: 140 })
    expect(document.querySelector('.section-break-readout')?.textContent).toBe('440')
    expect((document.querySelector('.section-break-proposal') as HTMLElement).style.getPropertyValue('--section-break-display-y')).toBe('440px')
    expect(sent(request)).toEqual([])
    fireEvent.pointerUp(strip, { pointerId: 1, buttons: 0, clientY: 140 })
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"setSectionBreak","version":1,"offset":440,"snap":true}']))
    expect(document.querySelector('.section-break-proposal')).toBeNull()
  })

  it('sends nothing when Escape aborts a drag', async () => {
    const request = open(withBreak())
    const strip = handle()
    fireEvent.pointerDown(strip, { pointerId: 1, button: 0, buttons: 1, clientY: 100 })
    fireEvent.pointerMove(strip, { pointerId: 1, buttons: 1, clientY: 160 })
    fireEvent.keyDown(strip, { key: 'Escape' })
    expect(document.querySelector('.section-break-proposal')).toBeNull()
    fireEvent.pointerUp(strip, { pointerId: 1, buttons: 0, clientY: 160 })
    await settle()
    expect(sent(request)).toEqual([])
  })

  it('nudges 1pt with an arrow and 10pt with Shift, one unsnapped command per press', async () => {
    const request = open(withBreak())
    fireEvent.keyDown(handle(), { key: 'ArrowDown' })
    fireEvent.keyDown(handle(), { key: 'ArrowUp', shiftKey: true })
    await waitFor(() => expect(sent(request)).toEqual([
      '{"kind":"setSectionBreak","version":1,"offset":401,"snap":false}',
      '{"kind":"setSectionBreak","version":1,"offset":390,"snap":false}',
    ]))
  })

  it('commits a typed Y exactly, unsnapped, on Enter', async () => {
    const request = open(withBreak())
    fireEvent.click(handle())
    const field = screen.getByRole('textbox', { name: 'Y (pt)' })
    fireEvent.change(field, { target: { value: '520' } })
    fireEvent.keyDown(field, { key: 'Enter' })
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"setSectionBreak","version":1,"offset":520,"snap":false}']))
  })

  it('shows the engine refusal naming the blocking element, and the Y field reverts', async () => {
    open(withBreak(), async (operation) => {
      if (operation === 'command') throw Object.assign(new Error('a section break at 520pt would run through e5'), { code: 'COMPONENT_INVALID', elementId: 'e5' })
      return { snapshot: snapshotOf(withBreak(), 1) }
    })
    fireEvent.click(handle())
    const field = screen.getByRole('textbox', { name: 'Y (pt)' })
    fireEvent.change(field, { target: { value: '520' } })
    fireEvent.blur(field)
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('e5: a section break at 520pt would run through e5'))
    expect(screen.getByRole('textbox', { name: 'Y (pt)' })).toHaveValue('400')
  })

  it('draws the line on the sheet whose window holds the offset, and on no other', () => {
    open(withBreak(650_000, { contentWindowCount: 2, contentWindowOrigins: [0, 600_000], contentWindowPages: [0, 0] }))
    const pages = document.querySelectorAll('.page-surface')
    expect(pages).toHaveLength(2)
    expect(pages[0]!.querySelectorAll('.section-break-line')).toHaveLength(0)
    expect(pages[1]!.querySelectorAll('.section-break-line')).toHaveLength(1)
    expect((pages[1]!.querySelector('.section-break-line') as HTMLElement).style.getPropertyValue('--section-break-display-y')).toBe('50px')
    expect(screen.getAllByRole('button', { name: 'Section Break' })).toHaveLength(1)
  })

  it('deletes a selected break through the canvas toolbar Delete button', async () => {
    const request = open(withBreak())
    fireEvent.click(handle())
    const toolbarDelete = screen.getByRole('button', { name: 'Delete' })
    expect(toolbarDelete).toBeEnabled()
    fireEvent.click(toolbarDelete)
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"removeSectionBreak","version":1}']))
  })

  it('offers no Delete button in the Section Break Properties, only the Y field and the Anchor checkbox', () => {
    open(withBreak())
    fireEvent.click(handle())
    expect(screen.getByRole('textbox', { name: 'Y (pt)' })).toBeInTheDocument()
    expect(screen.getByRole('checkbox', { name: 'Anchor' })).toBeChecked()
    expect(screen.queryByRole('button', { name: 'Delete Section Break' })).toBeNull()
  })

  // spec-section-break CAP-7: the Anchor option.
  it('shows the anchor icon in the tab while anchored, and none while unanchored', () => {
    open(withBreak())
    expect(document.querySelectorAll('.section-break-tab [data-testid="section-break-anchor-icon"]')).toHaveLength(1)
    expect(document.querySelector('.section-break-tab')!.textContent).toBe('Section Break')
    cleanup()
    open(withBreak(400_000, { sectionBreakAnchor: false }))
    expect(document.querySelector('.section-break-tab')).not.toBeNull()
    expect(document.querySelectorAll('[data-testid="section-break-anchor-icon"]')).toHaveLength(0)
  })

  it('unchecking Anchor sends one engine command, and the answer hides the icon and unchecks the box', async () => {
    const request = open(withBreak(), async () => ({ snapshot: snapshotOf(withBreak(400_000, { sectionBreakAnchor: false }), 2) }))
    fireEvent.click(handle())
    fireEvent.click(screen.getByRole('checkbox', { name: 'Anchor' }))
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"setSectionBreakAnchor","version":1,"anchor":false}']))
    await waitFor(() => expect(screen.getByRole('checkbox', { name: 'Anchor' })).not.toBeChecked())
    expect(document.querySelectorAll('[data-testid="section-break-anchor-icon"]')).toHaveLength(0)
  })

  it('checking Anchor on an unanchored break sends anchor true', async () => {
    const request = open(withBreak(400_000, { sectionBreakAnchor: false }), async () => ({ snapshot: snapshotOf(withBreak(), 2) }))
    fireEvent.click(handle())
    expect(screen.getByRole('checkbox', { name: 'Anchor' })).not.toBeChecked()
    fireEvent.click(screen.getByRole('checkbox', { name: 'Anchor' }))
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"setSectionBreakAnchor","version":1,"anchor":true}']))
    await waitFor(() => expect(document.querySelectorAll('[data-testid="section-break-anchor-icon"]')).toHaveLength(1))
  })

  it('nudges a selected break from the window arrow keys when focus is off the line', async () => {
    const request = open(withBreak())
    fireEvent.click(handle())
    fireEvent.keyDown(document.body, { key: 'ArrowDown' })
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"setSectionBreak","version":1,"offset":401,"snap":false}']))
  })

  it('places nothing and moves nothing when a break appeared while the entry was armed', async () => {
    const component = { id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 12_000, resizable: true }
    const plain = { ...canvas, components: [component] }
    const broken = withBreak(400_000, { components: [{ ...component, belowSectionBreak: false }] })
    const request = vi.fn(async (operation: string) => ({ snapshot: operation === 'undo' ? { ...snapshotOf(broken, 3), canUndo: false, canRedo: true } : { ...snapshotOf(plain, 2), canUndo: true } }))
    render(<App engine={engine(request as never)} initialSnapshot={snapshotOf(plain)} />)
    // One accepted command, so undo is available.
    fireEvent.click(screen.getByLabelText(/text component e1/))
    fireEvent.keyDown(document.body, { key: 'ArrowDown' })
    await waitFor(() => expect(sent(request)).toHaveLength(1))
    fireEvent.click(entry())
    fireEvent.keyDown(document.body, { key: 'z', ctrlKey: true })
    await waitFor(() => expect(document.querySelectorAll('.section-break-line')).toHaveLength(1))
    fireEvent.keyDown(screen.getByLabelText('Content'), { key: 'Enter' })
    await settle()
    expect(sent(request).filter((command) => command.includes('SectionBreak'))).toEqual([])
  })

  it('selecting a component clears the break selection', () => {
    open(withBreak(400_000, { components: [{ id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 72_000, height: 12_000, resizable: true, belowSectionBreak: false }] }))
    fireEvent.click(handle())
    expect(handle()).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(screen.getByLabelText(/text component e1/))
    expect(handle()).toHaveAttribute('aria-pressed', 'false')
  })
})

// SPEC-multi-pages story 2: pages on the canvas. Each designed page is its own
// group of sheets with a "Page N" label; a page is selected by its empty space
// or its label; Add page and Delete page are one command each, and Delete page
// confirms in-app first. These rows pin what the canvas sends and draws.
describe('SPEC-multi-pages: pages on the canvas', () => {
  const snapshotOf = (projection: CanvasProjection, revision = 1, extra: object = {}) => ({ documentState: 'loaded' as const, revision, byteLength: 3, canvas: projection, ...extra })
  const text = (id: string, page: number, y = 0, band: 'content' | 'pageHeader' | 'pageFooter' = 'content') => ({ id, type: 'text' as const, band, x: 0, y, width: 72_000, height: 24_000, resizable: true, page })
  const pages = (count: number, patch: Partial<CanvasProjection> = {}): CanvasProjection => ({ ...canvas, contentWindowCount: count, contentWindowOrigins: Array.from({ length: count }, () => 0), contentWindowPages: Array.from({ length: count }, (_value, index) => index), pageBreaks: Array.from({ length: count }, (_value, index) => index !== 1), ...(count > 1 ? { sectionBreaks: Array.from({ length: count }, () => null), sectionBreakAnchors: Array.from({ length: count }, () => true) } : {}), ...patch })
  const open = (projection: CanvasProjection, answer: (operation: string) => Promise<unknown> = async () => ({ snapshot: snapshotOf(projection, 2) }), extra: object = {}) => {
    const request = vi.fn(answer)
    render(<App engine={engine(request as never)} initialSnapshot={snapshotOf(projection, 1, extra)} />)
    return request
  }
  const sent = (request: ReturnType<typeof vi.fn>) => (request.mock.calls as unknown as ReadonlyArray<[string, ArrayBuffer]>).filter(([operation]) => operation === 'command').map(([, payload]) => new TextDecoder().decode(payload))
  const tools = () => within(screen.getByLabelText('Canvas controls'))
  const surfaces = () => Array.from(document.querySelectorAll('.page-surface')) as HTMLElement[]
  const label = (n: number) => screen.getByRole('button', { name: `Page ${n}` })
  const settle = async () => { await act(async () => { await Promise.resolve() }) }

  it('keeps a one-page document exactly as it was, plus one Page 1 label', () => {
    open({ ...canvas, components: [text('e1', 0)] })
    expect(surfaces().map((surface) => surface.getAttribute('aria-label'))).toEqual(['Report page with Page Header, Content, and Page Footer'])
    expect(screen.getByLabelText('Content')).toBeInTheDocument()
    expect(screen.getByLabelText('text component e1')).toBeInTheDocument()
    expect(Array.from(document.querySelectorAll('.page-label')).map((node) => node.textContent)).toEqual(['Page 1'])
    expect(tools().getByRole('button', { name: 'Delete page' })).toBeDisabled()
    expect(tools().getByRole('button', { name: 'Delete page' })).toHaveAccessibleDescription('A document keeps at least one page.')
    expect(tools().getByRole('button', { name: 'Delete page' })).toHaveAttribute('data-tip', 'Delete page: a document keeps at least one page')
  })

  it('draws each page component only on its own page sheets, labels each page, and page 1 break only on page 1 sheets', () => {
    open(pages(2, { sectionBreaks: [400_000, null], components: [text('e1', 0, 0), text('e2', 1, 0)].map((component) => component.page === 0 ? { ...component, belowSectionBreak: false } : component) }))
    expect(Array.from(document.querySelectorAll('.page-label')).map((node) => node.textContent)).toEqual(['Page 1', 'Page 2'])
    const [first, second] = surfaces()
    expect(within(first!).getByLabelText('text component e1')).toBeInTheDocument()
    expect(within(first!).queryByLabelText('text component e2')).toBeNull()
    expect(within(second!).getByLabelText('text component e2')).toBeInTheDocument()
    expect(document.querySelectorAll('.canvas-component-echo')).toHaveLength(0)
    expect(first!.querySelectorAll('.section-break-line')).toHaveLength(1)
    expect(second!.querySelectorAll('.section-break-line')).toHaveLength(0)
    expect(within(first!).getByRole('button', { name: 'Section Break on page 1' })).toBeInTheDocument()
  })

  // SPEC-multi-pages story 5: a section break on every page. Each page's line
  // is drawn among its own sheets and named by its page; every gesture on it
  // names that page, and page 1's sends today's bytes.
  it('draws each page break on its own sheets, named by page, and edits page 2 break with page 1', async () => {
    const request = open(pages(2, { sectionBreaks: [400_000, 300_000], sectionBreakAnchors: [true, false], components: [{ ...text('e1', 0), belowSectionBreak: false }, { ...text('e2', 1), belowSectionBreak: false }] }))
    const [first, second] = surfaces()
    const breakOn = (n: number) => screen.getByRole('button', { name: `Section Break on page ${n}` })
    expect(first!.querySelectorAll('.section-break-line')).toHaveLength(1)
    expect(second!.querySelectorAll('.section-break-line')).toHaveLength(1)
    expect(first!.contains(breakOn(1))).toBe(true)
    expect(second!.contains(breakOn(2))).toBe(true)
    expect((second!.querySelector('.section-break-line') as HTMLElement).style.getPropertyValue('--section-break-display-y')).toBe('300px')
    // Page 2's break is unanchored, so only page 1's tab shows the anchor.
    expect(first!.querySelectorAll('[data-testid="section-break-anchor-icon"]')).toHaveLength(1)
    expect(second!.querySelectorAll('[data-testid="section-break-anchor-icon"]')).toHaveLength(0)
    fireEvent.click(breakOn(2))
    expect(breakOn(2)).toHaveAttribute('aria-pressed', 'true')
    expect(breakOn(1)).toHaveAttribute('aria-pressed', 'false')
    expect(screen.getByRole('textbox', { name: 'Y (pt)' })).toHaveValue('300')
    expect(screen.getByRole('checkbox', { name: 'Anchor' })).not.toBeChecked()
    // Selecting it made page 2 current: its Place Section Break is disabled.
    expect(screen.getByRole('button', { name: 'Place Section Break' })).toBeDisabled()
    expect(screen.getByText('This page already has its Section Break.')).toBeInTheDocument()
    fireEvent.keyDown(breakOn(2), { key: 'ArrowDown' })
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"setSectionBreak","version":1,"offset":301,"snap":false,"page":1}']))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Anchor' }))
    await waitFor(() => expect(sent(request)).toHaveLength(2))
    expect(sent(request)[1]).toBe('{"kind":"setSectionBreakAnchor","version":1,"anchor":true,"page":1}')
    const field = screen.getByRole('textbox', { name: 'Y (pt)' })
    fireEvent.change(field, { target: { value: '320' } })
    fireEvent.keyDown(field, { key: 'Enter' })
    await waitFor(() => expect(sent(request)).toHaveLength(3))
    expect(sent(request)[2]).toBe('{"kind":"setSectionBreak","version":1,"offset":320,"snap":false,"page":1}')
    fireEvent.keyDown(breakOn(2), { key: 'Delete' })
    await waitFor(() => expect(sent(request)).toHaveLength(4))
    expect(sent(request)[3]).toBe('{"kind":"removeSectionBreak","version":1,"page":1}')
    // Page 1's break keeps today's bytes.
    fireEvent.keyDown(breakOn(1), { key: 'ArrowUp' })
    await waitFor(() => expect(sent(request)).toHaveLength(5))
    expect(sent(request)[4]).toBe('{"kind":"setSectionBreak","version":1,"offset":399,"snap":false}')
  })

  it('deletes page 2 break from the toolbar Delete, naming page 2', async () => {
    const request = open(pages(2, { sectionBreaks: [null, 300_000] }))
    fireEvent.click(screen.getByRole('button', { name: 'Section Break on page 2' }))
    fireEvent.click(tools().getByRole('button', { name: 'Delete' }))
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"removeSectionBreak","version":1,"page":1}']))
  })

  it('nudges a selected page 2 break from the window arrow keys, naming page 2', async () => {
    const request = open(pages(2, { sectionBreaks: [null, 300_000] }))
    const handle = screen.getByRole('button', { name: 'Section Break on page 2' })
    fireEvent.click(handle)
    handle.blur()
    fireEvent.keyDown(document.body, { key: 'ArrowDown' })
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"setSectionBreak","version":1,"offset":301,"snap":false,"page":1}']))
  })

  it('drops the break selection when undo changes the page count, so Delete never hits another page break', async () => {
    // Undoing a page delete brings a page back before page 2: page 2's break is
    // now page 3's, and the new page 2 has a break of its own.
    const restored = pages(3, { sectionBreaks: [null, 200_000, 300_000] })
    const request = open(pages(2, { sectionBreaks: [null, 300_000] }), async () => ({ snapshot: snapshotOf(restored, 2, { canUndo: false, canRedo: true }) }), { canUndo: true, canRedo: false })
    fireEvent.click(screen.getByRole('button', { name: 'Section Break on page 2' }))
    fireEvent.click(screen.getByRole('button', { name: 'Undo' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Section Break on page 3' })).toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'Section Break on page 2' })).toHaveAttribute('aria-pressed', 'false')
    fireEvent.keyDown(document.body, { key: 'Delete' })
    await settle()
    expect(sent(request)).toEqual([])
  })

  it('drags page 2 break by clientY and sends one snapped command naming page 2', async () => {
    const request = open(pages(2, { sectionBreaks: [null, 300_000] }))
    const strip = screen.getByRole('button', { name: 'Section Break on page 2' })
    fireEvent.pointerDown(strip, { pointerId: 1, button: 0, buttons: 1, clientY: 100 })
    fireEvent.pointerMove(strip, { pointerId: 1, buttons: 1, clientY: 140 })
    expect(surfaces()[1]!.querySelector('.section-break-readout')?.textContent).toBe('340')
    expect(surfaces()[0]!.querySelector('.section-break-readout')).toBeNull()
    fireEvent.pointerUp(strip, { pointerId: 1, buttons: 0, clientY: 140 })
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"setSectionBreak","version":1,"offset":340,"snap":true,"page":1}']))
  })

  // D-5.1: Place Section Break follows the current page.
  it('disables Place Section Break only while the current page has a break, and skips such a band while armed', async () => {
    const withBoth = pages(2, { sectionBreaks: [364_945, 300_000], components: [text('e1', 0, 500_000), text('e2', 1, 0)].map((component) => ({ ...component, belowSectionBreak: component.page === 0 })) })
    const request = open(pages(2, { sectionBreaks: [null, 300_000], components: [text('e1', 0, 500_000), { ...text('e2', 1, 0), belowSectionBreak: false }] }), async () => ({ snapshot: snapshotOf(withBoth, 2) }))
    const entry = () => screen.getByRole('button', { name: 'Place Section Break' })
    expect(entry()).toBeEnabled()
    fireEvent.click(label(2))
    expect(entry()).toBeDisabled()
    expect(screen.getByText('This page already has its Section Break.')).toBeInTheDocument()
    fireEvent.keyDown(screen.getByLabelText('text component e1'), { key: 'Enter' })
    expect(entry()).toBeEnabled()
    expect(screen.queryByText('This page already has its Section Break.')).toBeNull()
    fireEvent.click(label(1))
    fireEvent.click(entry())
    expect(entry()).toHaveAttribute('aria-pressed', 'true')
    // Page 2's content band already has its break: no target, nothing sent.
    fireEvent.keyDown(screen.getByLabelText('Content on page 2 of 2'), { key: 'Enter' })
    await settle()
    expect(sent(request)).toEqual([])
    fireEvent.keyDown(screen.getByLabelText('Content on page 1 of 2'), { key: 'Enter' })
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"setSectionBreak","version":1,"offset":364.945,"snap":true}']))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Section Break on page 1' })).toHaveAttribute('aria-pressed', 'true'))
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Section Break on page 1' })))
    expect(entry()).toBeDisabled()
  })

  it('selects a page from its empty space or its label, outlines it, and shows its Page Break', async () => {
    const request = open(pages(2))
    fireEvent.click(surfaces()[1]!)
    expect(label(2)).toHaveAttribute('aria-pressed', 'true')
    expect(surfaces()[1]).toHaveClass('page-surface-selected')
    expect(surfaces()[0]).not.toHaveClass('page-surface-selected')
    expect(screen.getByText('PAGE SETUP')).toBeInTheDocument()
    expect(screen.getByText('PAGE 2')).toBeInTheDocument()
    const pageBreak = screen.getByRole('checkbox', { name: 'Page Break' })
    expect(pageBreak).not.toBeChecked()
    fireEvent.click(pageBreak)
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"setPageBreak","version":1,"page":1,"pageBreak":true}']))
    fireEvent.click(label(1))
    expect(label(1)).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('checkbox', { name: 'Page Break' })).toBeDisabled()
    expect(screen.getByRole('checkbox', { name: 'Page Break' })).toBeChecked()
    expect(screen.getByRole('checkbox', { name: 'Page Break' })).toHaveAccessibleDescription('Page 1 always starts the document, so Page Break does not apply to it.')
  })

  it('clears the page selection on Escape and on selecting an element', () => {
    open(pages(2, { components: [text('e1', 0)] }))
    fireEvent.click(label(2))
    fireEvent.keyDown(screen.getByLabelText('Canvas region'), { key: 'Escape' })
    expect(label(2)).toHaveAttribute('aria-pressed', 'false')
    expect(screen.queryByText('PAGE 2')).toBeNull()
    fireEvent.click(label(2))
    fireEvent.keyDown(screen.getByLabelText('text component e1'), { key: 'Enter' })
    expect(label(2)).toHaveAttribute('aria-pressed', 'false')
    expect(screen.queryByRole('checkbox', { name: 'Page Break' })).toBeNull()
  })

  it('adds a page after the selected page and selects it, or at the end with none selected', async () => {
    const request = open(pages(3), async () => ({ snapshot: snapshotOf(pages(4), 2) }))
    fireEvent.click(label(2))
    fireEvent.click(tools().getByRole('button', { name: 'Add page' }))
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"addPage","version":1,"after":1}']))
    await waitFor(() => expect(label(3)).toHaveAttribute('aria-pressed', 'true'))
    cleanup()
    const appended = open(pages(2), async () => ({ snapshot: snapshotOf(pages(3), 2) }))
    fireEvent.click(tools().getByRole('button', { name: 'Add page' }))
    await waitFor(() => expect(sent(appended)).toEqual(['{"kind":"addPage","version":1}']))
    await waitFor(() => expect(label(3)).toHaveAttribute('aria-pressed', 'true'))
  })

  it('confirms Delete page in-app, naming the page; Cancel and Escape send nothing', async () => {
    const request = open(pages(3, { components: [text('e1', 0), text('e2', 1)] }), async () => ({ snapshot: snapshotOf(pages(2), 2) }))
    fireEvent.keyDown(screen.getByLabelText('text component e2'), { key: 'Enter' })
    fireEvent.click(tools().getByRole('button', { name: 'Delete page' }))
    const dialog = screen.getByRole('dialog', { name: 'Delete page 2?' })
    expect(dialog).toHaveAttribute('aria-modal', 'true')
    await waitFor(() => expect(document.activeElement).toBe(within(dialog).getByRole('button', { name: 'Cancel' })))
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    expect(screen.queryByRole('dialog')).toBeNull()
    fireEvent.click(tools().getByRole('button', { name: 'Delete page' }))
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(screen.queryByRole('dialog')).toBeNull()
    await settle()
    expect(sent(request)).toEqual([])
    fireEvent.click(tools().getByRole('button', { name: 'Delete page' }))
    fireEvent.click(within(screen.getByRole('dialog', { name: 'Delete page 2?' })).getByRole('button', { name: 'Delete page' }))
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"deletePage","version":1,"page":1}']))
    await waitFor(() => expect(screen.queryAllByRole('button', { name: /^Page \d$/ })).toHaveLength(2))
  })

  it('targets the page of the selected element when no page is selected', () => {
    open(pages(3, { components: [text('e3', 2)] }))
    fireEvent.keyDown(screen.getByLabelText('text component e3'), { key: 'Enter' })
    fireEvent.click(tools().getByRole('button', { name: 'Delete page' }))
    expect(screen.getByRole('dialog', { name: 'Delete page 3?' })).toBeInTheDocument()
  })

  it('disables Delete page with its reason when there is no single page to name', () => {
    open(pages(2, { components: [text('h1', 0, 0, 'pageHeader'), text('e1', 0), text('e2', 1)] }))
    const button = () => tools().getByRole('button', { name: 'Delete page' })
    expect(button()).toBeDisabled()
    expect(button()).toHaveAccessibleDescription('Select a page, or content on one page.')
    fireEvent.keyDown(screen.getByLabelText('text component h1'), { key: 'Enter' })
    expect(button()).toBeDisabled()
    fireEvent.keyDown(screen.getByLabelText('text component e1'), { key: 'Enter' })
    expect(button()).toBeEnabled()
    // Select all in the content band: e1 on page 1 and e2 on page 2.
    fireEvent.focus(screen.getByLabelText('Content on page 1 of 2'))
    fireEvent.keyDown(screen.getByLabelText('Canvas region'), { key: 'a', ctrlKey: true })
    expect(screen.getByLabelText('text component e1')).toHaveClass('canvas-component-selected')
    expect(screen.getByLabelText('text component e2')).toHaveClass('canvas-component-selected')
    expect(button()).toBeDisabled()
    expect(button()).toHaveAttribute('data-tip', 'Delete page: the selection is on more than one page')
  })

  it('states a file action in progress as the reason both page buttons are disabled', async () => {
    const request = vi.fn(async (operation: string) => operation === 'load' ? new Promise<never>(() => undefined) : ({ snapshot: snapshotOf(pages(2), 2) }))
    const files: FileAccess = { open: vi.fn(async () => ({ bytes, name: 'busy.folio' })), acquireSaveTarget: vi.fn(), writeSave: vi.fn() }
    render(<App engine={engine(request as never)} fileAccess={files} initialSnapshot={snapshotOf(pages(2))} />)
    fireEvent.click(label(2))
    expect(tools().getByRole('button', { name: 'Delete page' })).toBeEnabled()
    fireEvent.click(screen.getByRole('button', { name: 'Open local template' }))
    await waitFor(() => expect(tools().getByRole('button', { name: 'Add page' })).toBeDisabled())
    for (const name of ['Add page', 'Delete page']) {
      const button = tools().getByRole('button', { name })
      expect(button).toBeDisabled()
      expect(button).toHaveAttribute('data-tip', `${name}: a file action is in progress`)
      expect(button).toHaveAccessibleDescription('A file action is in progress.')
    }
  })

  it('moves focus to the canvas region after a confirmed delete, and describes the dialog', async () => {
    open(pages(2), async () => ({ snapshot: snapshotOf({ ...canvas, pageBreaks: [true] }, 2) }))
    fireEvent.click(label(2))
    fireEvent.click(tools().getByRole('button', { name: 'Delete page' }))
    const dialog = screen.getByRole('dialog', { name: 'Delete page 2?' })
    expect(dialog).toHaveAccessibleDescription('This removes the page and everything on it.')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Delete page' }))
    await waitFor(() => expect(screen.queryAllByRole('button', { name: /^Page \d$/ })).toHaveLength(1))
    expect(document.activeElement).toBe(screen.getByLabelText('Canvas region'))
  })

  it('keeps keyboard focus in the dialog when its backdrop is pressed', async () => {
    const request = open(pages(2))
    fireEvent.click(label(2))
    fireEvent.click(tools().getByRole('button', { name: 'Delete page' }))
    const dialog = screen.getByRole('dialog', { name: 'Delete page 2?' })
    ;(document.activeElement as HTMLElement).blur()
    fireEvent.pointerDown(dialog)
    expect(document.activeElement).toBe(within(dialog).getByRole('button', { name: 'Cancel' }))
    fireEvent.keyDown(document.activeElement!, { key: 'Escape' })
    expect(screen.queryByRole('dialog')).toBeNull()
    await settle()
    expect(sent(request)).toEqual([])
  })

  it('deletes only the selected later-page element from the keyboard, even with Delete page enabled', async () => {
    for (const key of ['Delete', 'Backspace']) {
      const request = open(pages(3, { components: [text('e1', 0), text('e2', 1)] }))
      fireEvent.keyDown(screen.getByLabelText('text component e2'), { key: 'Enter' })
      expect(tools().getByRole('button', { name: 'Delete page' })).toBeEnabled()
      fireEvent.keyDown(screen.getByLabelText('Canvas region'), { key })
      await waitFor(() => expect(sent(request)).toEqual(['{"kind":"deleteComponent","version":1,"id":"e2"}']))
      expect(screen.queryByRole('dialog')).toBeNull()
      cleanup()
    }
  })

  it('resizes a later-page component within its own page column', async () => {
    // Page 2 has two windows; its component's foot is on page 2's second sheet.
    const request = open({ ...canvas, contentWindowCount: 3, contentWindowOrigins: [0, 0, 600_000], contentWindowPages: [0, 1, 1], pageBreaks: [true, true], components: [text('e2', 1, 590_000)] })
    fireEvent.keyDown(screen.getByLabelText('text component e2'), { key: 'Enter' })
    const handle = screen.getByRole('button', { name: 'Resize e2' })
    fireEvent.pointerDown(handle, { pointerId: 1, clientX: 10, clientY: 10 })
    fireEvent.pointerMove(handle, { pointerId: 1, clientX: 10, clientY: -90 })
    fireEvent.pointerUp(handle, { pointerId: 1, clientX: 10, clientY: -90 })
    await waitFor(() => expect(sent(request)).toHaveLength(1))
    // 100px up crosses page 2's seam onto page 2's first sheet (origin 0). A
    // page-1 mapping would keep the edge on sheet 1 and send a height of 1.
    expect(sent(request)[0]).toBe('{"kind":"setComponentBounds","version":1,"id":"e2","x":0,"y":590,"width":72,"height":189.89,"snap":true}')
  })

  it('never deletes a page from the Delete or Backspace key', async () => {
    const request = open(pages(2))
    fireEvent.click(label(2))
    const region = screen.getByLabelText('Canvas region')
    fireEvent.keyDown(region, { key: 'Delete' })
    fireEvent.keyDown(region, { key: 'Backspace' })
    await settle()
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(sent(request)).toEqual([])
  })

  // SPEC-multi-pages story 3: a palette element placed on a later page's content
  // band is created there, naming the page; page 1 keeps today's bytes.
  it('places a palette element on a later page content band into that page', async () => {
    const request = open(pages(2))
    fireEvent.click(screen.getByRole('button', { name: 'Place Text' }))
    fireEvent.keyDown(screen.getByLabelText('Content on page 2 of 2'), { key: 'Enter' })
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"createComponent","version":1,"type":"text","band":"content","x":0,"y":0,"width":72,"height":24,"snap":true,"page":1}']))
    fireEvent.click(screen.getByRole('button', { name: 'Place Text' }))
    fireEvent.keyDown(screen.getByLabelText('Content on page 1 of 2'), { key: 'Enter' })
    await waitFor(() => expect(sent(request)).toHaveLength(2))
    expect(sent(request)[1]).toBe('{"kind":"dropComponent","version":1,"type":"text","x":36,"y":56,"snap":true}')
    // Story 5: a section break placed on page 2's content band names page 2.
    fireEvent.click(screen.getByRole('button', { name: 'Place Section Break' }))
    fireEvent.keyDown(screen.getByLabelText('Content on page 2 of 2'), { key: 'Enter' })
    await waitFor(() => expect(sent(request)).toHaveLength(3))
    expect(sent(request)[2]).toBe('{"kind":"setSectionBreak","version":1,"offset":364.945,"snap":true,"page":1}')
  })

  it('places on a later page content band by pointer release into that page, at its page-local y', async () => {
    // Page 2 has two sheets; its second window begins 600pt down page 2's own
    // column, so a release 40pt below that sheet's band head is page-local y 640.
    const request = open(pages(2, { contentWindowCount: 3, contentWindowOrigins: [0, 0, 600_000], contentWindowPages: [0, 1, 1], pageBreaks: [true, true] }))
    fireEvent.click(screen.getByRole('button', { name: 'Place Text' }))
    // D-4.1 (story 4): page 2's continuation sheet names its designed page and its sheet.
    const band = screen.getByLabelText('Content on page 2 of 2, sheet 3 of 3')
    const released = createEvent.pointerUp(band)
    Object.defineProperty(released, ['offset', 'X'].join(''), { value: 120 })
    Object.defineProperty(released, ['offset', 'Y'].join(''), { value: 40 })
    fireEvent(band, released)
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"createComponent","version":1,"type":"text","band":"content","x":120,"y":640,"width":72,"height":24,"snap":true,"page":1}']))
  })

  // A paste lands on the page whose sheet the pointer is over; off every sheet
  // it is today's command and each copy stays on its source's page.
  it('pastes copied elements onto the page the pointer is over', async () => {
    const request = open(pages(2, { components: [text('e1', 0)] }))
    fireEvent.keyDown(screen.getByLabelText('text component e1'), { key: 'Enter' })
    const region = screen.getByLabelText('Canvas region')
    fireEvent.keyDown(region, { key: 'c', ctrlKey: true })
    fireEvent.pointerMove(surfaces()[1]!)
    fireEvent.keyDown(region, { key: 'v', ctrlKey: true })
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"duplicateComponents","version":1,"ids":["e1"],"snap":true,"page":1}']))
    await settle()
    fireEvent.pointerLeave(region)
    fireEvent.keyDown(region, { key: 'v', ctrlKey: true })
    await waitFor(() => expect(sent(request)).toHaveLength(2))
    expect(sent(request)[1]).toBe('{"kind":"duplicateComponents","version":1,"ids":["e1"],"snap":true}')
  })

  it('keeps today paste bytes in a one-page document with the pointer on its sheet', async () => {
    const request = open({ ...canvas, components: [text('e1', 0)] })
    fireEvent.keyDown(screen.getByLabelText('text component e1'), { key: 'Enter' })
    const region = screen.getByLabelText('Canvas region')
    fireEvent.keyDown(region, { key: 'c', ctrlKey: true })
    fireEvent.pointerMove(surfaces()[0]!)
    fireEvent.keyDown(region, { key: 'v', ctrlKey: true })
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"duplicateComponents","version":1,"ids":["e1"],"snap":true}']))
  })

  // One pitch at zoom 1 is 865.89pt (the page plus the 24px gap); the content
  // band starts 56pt down each sheet. A press on e1's top is stack y 56pt.
  it('moves a dragged element onto the page under the pointer, previewing it on that sheet', async () => {
    const request = open(pages(2, { components: [text('e1', 0)] }))
    const region = screen.getByLabelText('Canvas region')
    fireEvent.pointerDown(screen.getByLabelText('text component e1'), { pointerId: 1, clientX: 10, clientY: 10 })
    // 1,065.89px down: page 2's sheet, 200pt down its column.
    fireEvent.pointerMove(region, { pointerId: 1, clientX: 10, clientY: 1075.89 })
    await waitFor(() => expect(screen.getByLabelText('text component e1').closest('.page-surface')).toBe(surfaces()[1]))
    expect(screen.getByLabelText('text component e1').style.getPropertyValue('--component-y')).toBe('200px')
    fireEvent.pointerUp(region, { pointerId: 1, clientX: 10, clientY: 1075.89 })
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"moveComponents","version":1,"ids":["e1"],"referenceId":"e1","dx":0,"dy":200,"snap":true,"expectedRevision":1,"constrainToWindow":true,"page":1}']))
  })

  it('keeps today move bytes for a drag that stays on its own page', async () => {
    const request = open(pages(2, { components: [text('e2', 1)] }))
    const region = screen.getByLabelText('Canvas region')
    fireEvent.pointerDown(screen.getByLabelText('text component e2'), { pointerId: 1, clientX: 10, clientY: 10 })
    fireEvent.pointerMove(region, { pointerId: 1, clientX: 10, clientY: 110 })
    fireEvent.pointerUp(region, { pointerId: 1, clientX: 10, clientY: 110 })
    await waitFor(() => expect(sent(request)).toEqual(['{"kind":"moveComponents","version":1,"ids":["e2"],"referenceId":"e2","dx":0,"dy":100,"snap":true,"expectedRevision":1,"constrainToWindow":true}']))
  })

  it('clears the preview and shows the refusal when a move to another page is refused', async () => {
    const request = open(pages(2, { components: [text('e1', 0)] }), async (operation) => { if (operation === 'command') throw Object.assign(new Error('keepTogether group "signature" would have members on pages[1] and pages[0]'), { elementId: 'e1', dataPath: 'pages' }); return { snapshot: snapshotOf(pages(2), 1) } })
    const region = screen.getByLabelText('Canvas region')
    fireEvent.pointerDown(screen.getByLabelText('text component e1'), { pointerId: 1, clientX: 10, clientY: 10 })
    fireEvent.pointerMove(region, { pointerId: 1, clientX: 10, clientY: 1075.89 })
    await waitFor(() => expect(screen.getByLabelText('text component e1').closest('.page-surface')).toBe(surfaces()[1]))
    fireEvent.pointerUp(region, { pointerId: 1, clientX: 10, clientY: 1075.89 })
    await waitFor(() => expect(sent(request)).toHaveLength(1))
    expect(await screen.findByRole('alert')).toHaveTextContent('keepTogether group "signature"')
    await waitFor(() => expect(screen.getByLabelText('text component e1').closest('.page-surface')).toBe(surfaces()[0]))
  })

  it('selects no page once undo removes the selected page', async () => {
    open(pages(2), async () => ({ snapshot: snapshotOf({ ...canvas, pageBreaks: [true] }, 2, { canUndo: false, canRedo: true }) }), { canUndo: true, canRedo: false })
    fireEvent.click(label(2))
    fireEvent.click(screen.getByRole('button', { name: 'Undo' }))
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Page 2' })).toBeNull())
    expect(label(1)).toHaveAttribute('aria-pressed', 'false')
    expect(screen.queryByRole('checkbox', { name: 'Page Break' })).toBeNull()
  })

  // SPEC-multi-pages story 4: the shared header and footer are editable from any
  // page. The one named, interactive copy sits on the CURRENT page's first sheet;
  // every other copy is an aria-hidden echo that still takes a press.
  describe('header and footer from any page', () => {
    const h1 = text('h1', 0, 0, 'pageHeader')
    const f1 = text('f1', 0, 0, 'pageFooter')
    const named = (id: string) => Array.from(document.querySelectorAll(`[data-component-id="${id}"]`)) as HTMLElement[]
    const echoOn = (sheet: number, band = 'pageHeader') => surfaces()[sheet]!.querySelector(`.page-band-${band} .canvas-component-echo`) as HTMLElement
    const press = (node: HTMLElement, init: object = {}) => {
      fireEvent.pointerDown(node, { pointerId: 1, button: 0, clientX: 10, clientY: 10, ...init })
      fireEvent.pointerUp(screen.getByLabelText('Canvas region'), { pointerId: 1, clientX: 10, clientY: 10 })
      // A browser follows the release with a click, which the gesture consumes;
      // jsdom does not, so send it here or the NEXT click would be swallowed.
      fireEvent.click(screen.getByLabelText('Canvas region'))
    }
    const namedOn = (id: string, sheet: number) => { expect(named(id)).toHaveLength(1); expect(surfaces()[sheet]!.contains(named(id)[0]!)).toBe(true) }
    // Page 1 spans sheets 1–2; page 2 is sheet 3.
    const continued = (components: CanvasProjection['components']): CanvasProjection => ({ ...canvas, contentWindowCount: 3, contentWindowOrigins: [0, 600_000, 0], contentWindowPages: [0, 0, 1], pageBreaks: [true, true], components })

    it('selects from page 3: one named copy of each component, moved to page 3, focused, nothing sent', async () => {
      const request = open(pages(3, { components: [h1, f1] }))
      namedOn('h1', 0); namedOn('f1', 0)
      expect(document.querySelectorAll('.canvas-component-echo')).toHaveLength(4)
      press(echoOn(2))
      namedOn('h1', 2); namedOn('f1', 2)
      expect(screen.getAllByLabelText('text component h1')).toHaveLength(1)
      expect(named('h1')[0]).toHaveClass('canvas-component-selected')
      expect(document.querySelectorAll('.canvas-component-echo')).toHaveLength(4)
      expect(Array.from(document.querySelectorAll('.canvas-component-echo')).every((echo) => echo.getAttribute('aria-hidden') === 'true' && !echo.hasAttribute('data-component-id') && !echo.hasAttribute('role'))).toBe(true)
      await waitFor(() => expect(document.activeElement).toBe(named('h1')[0]))
      await settle()
      expect(sent(request)).toEqual([])
    })

    it('edits from page 3 with today updateComponentProperties bytes, the named copy staying on page 3', async () => {
      const edited = pages(3, { components: [{ ...h1, value: 'Beta' }] })
      const request = open(pages(3, { components: [{ ...h1, value: 'Acme' }] }), async () => ({ snapshot: snapshotOf(edited, 2) }))
      press(echoOn(2))
      const field = screen.getByRole('textbox', { name: 'Text' })
      fireEvent.change(field, { target: { value: 'Beta' } })
      fireEvent.blur(field)
      await waitFor(() => expect(sent(request)).toEqual(['{"kind":"updateComponentProperties","version":1,"ids":["h1"],"changes":{"value":{"op":"set","value":"Beta"}}}']))
      await waitFor(() => expect(screen.getByRole('textbox', { name: 'Text' })).toHaveValue('Beta'))
      namedOn('h1', 2)
    })

    it('drags an unselected echo on page 2 with the same moveComponents bytes as the named copy, and no page', async () => {
      const drag = async (node: () => HTMLElement) => {
        const request = open(pages(2, { components: [h1] }))
        const region = screen.getByLabelText('Canvas region')
        fireEvent.pointerDown(node(), { pointerId: 1, button: 0, clientX: 10, clientY: 10 })
        fireEvent.pointerMove(region, { pointerId: 1, clientX: 10, clientY: 20 })
        fireEvent.pointerUp(region, { pointerId: 1, clientX: 10, clientY: 20 })
        await waitFor(() => expect(sent(request)).toHaveLength(1))
        const [command] = sent(request)
        cleanup()
        return command!
      }
      const fromNamed = await drag(() => screen.getByLabelText('text component h1'))
      const fromEcho = await drag(() => echoOn(1))
      expect(fromNamed).toContain('"kind":"moveComponents"')
      expect(fromEcho).toBe(fromNamed)
      expect(fromEcho).not.toContain('"page"')
    })

    it('moves the current page with content selection, page selection and Escape', () => {
      open(pages(3, { components: [h1, text('e1', 0)] }))
      press(echoOn(2))
      namedOn('h1', 2)
      fireEvent.keyDown(screen.getByLabelText('text component e1'), { key: 'Enter' })
      namedOn('h1', 0)
      fireEvent.click(label(2))
      namedOn('h1', 1)
      press(echoOn(2))
      namedOn('h1', 2)
      fireEvent.keyDown(screen.getByLabelText('Canvas region'), { key: 'Escape' })
      namedOn('h1', 0)
    })

    it('clamps the current page to the last page when its page goes away under a live selection, keeping one named copy', async () => {
      // The answer drops page 3 while h1 stays selected: current clamps to page 2.
      const request = open(pages(3, { components: [{ ...h1, value: 'Acme' }] }), async () => ({ snapshot: snapshotOf(pages(2, { components: [{ ...h1, value: 'Beta' }] }), 2) }))
      press(echoOn(2))
      namedOn('h1', 2)
      const field = screen.getByRole('textbox', { name: 'Text' })
      fireEvent.change(field, { target: { value: 'Beta' } })
      fireEvent.blur(field)
      await waitFor(() => expect(sent(request)).toHaveLength(1))
      await waitFor(() => expect(screen.queryByRole('button', { name: 'Page 3' })).toBeNull())
      namedOn('h1', 1)
      expect(named('h1')[0]).toHaveClass('canvas-component-selected')
    })

    it('returns the named copy to page 1 when undo clears the selection', async () => {
      open(pages(3, { components: [h1] }), async () => ({ snapshot: snapshotOf(pages(2, { components: [h1] }), 2, { canUndo: false, canRedo: true }) }), { canUndo: true, canRedo: false })
      press(echoOn(2))
      namedOn('h1', 2)
      fireEvent.click(screen.getByRole('button', { name: 'Undo' }))
      await waitFor(() => expect(screen.queryByRole('button', { name: 'Page 3' })).toBeNull())
      namedOn('h1', 0)
    })

    it('keeps the current page when the named header copy is clicked after an echo press', () => {
      open(pages(3, { components: [h1] }))
      press(echoOn(2))
      namedOn('h1', 2)
      fireEvent.click(screen.getByLabelText('text component h1'))
      namedOn('h1', 2)
      expect(named('h1')[0]).toHaveClass('canvas-component-selected')
    })

    it('keeps the current page when a Shift-click adds content on another page', () => {
      open(pages(2, { components: [h1, text('e1', 0), text('e2', 1)] }))
      fireEvent.click(screen.getByLabelText('text component e1'))
      fireEvent.click(screen.getByLabelText('text component e2'), { shiftKey: true })
      expect(screen.getByLabelText('text component e2')).toHaveClass('canvas-component-selected')
      namedOn('h1', 0)
    })

    it('Shift-pressing an echo of the only selected header clears the selection back to page 1, and a click there keeps page 1', () => {
      open(pages(3, { components: [h1] }))
      press(echoOn(1))
      namedOn('h1', 1)
      press(echoOn(2), { shiftKey: true })
      expect(document.querySelectorAll('.canvas-component-selected')).toHaveLength(0)
      namedOn('h1', 0)
      fireEvent.click(screen.getByLabelText('text component h1'))
      expect(named('h1')[0]).toHaveClass('canvas-component-selected')
      namedOn('h1', 0)
    })

    it('Return to Design on a located failure outranks a stale page selection', async () => {
      const projection = pages(3, { components: [h1, text('e3', 2)] })
      const located = snapshotOf(projection)
      const failure = Object.assign(new Error('The template could not be processed'), { code: 'RENDER_INVALID', elementId: 'e3', producerRenderFailure: true as const })
      const request = vi.fn(async (operation: string) => {
        if (operation === 'identity') return { snapshot: located, preview: { revision: 1, identity: 'b'.repeat(64) } }
        if (operation === 'serialize') return { snapshot: located, bytes }
        if (operation === 'render') throw failure
        return { snapshot: located }
      })
      render(<App engine={engine(request as never)} initialSnapshot={located} initialSampleData={sample} />)
      fireEvent.click(label(2))
      fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
      const card = await screen.findByLabelText('Local render failure')
      fireEvent.click(within(card).getByRole('button', { name: 'Return to Design' }))
      expect(await screen.findByLabelText('text component e3')).toHaveClass('canvas-component-selected')
      namedOn('h1', 2)
    })

    it('selects an unselected footer echo on page 2 and names it there', () => {
      open(pages(2, { components: [h1, f1] }))
      press(echoOn(1, 'pageFooter'))
      namedOn('f1', 1)
      expect(named('f1')[0]).toHaveClass('canvas-component-selected')
    })

    it('keeps keyboard focus on the header when Escape moves its named copy back to page 1', async () => {
      open(pages(3, { components: [h1] }))
      press(echoOn(2))
      await waitFor(() => expect(document.activeElement).toBe(named('h1')[0]))
      fireEvent.keyDown(document.activeElement!, { key: 'Escape' })
      namedOn('h1', 0)
      await waitFor(() => expect(document.activeElement).toBe(screen.getByLabelText('text component h1')))
    })

    it('selects from a continuation sheet while the named copy stays on that page first sheet', () => {
      open(continued([h1]))
      press(echoOn(1))
      namedOn('h1', 0)
      expect(named('h1')[0]).toHaveClass('canvas-component-selected')
    })

    it('Shift+press on an echo removes the component from the selection, leaving the current page to the remaining selection', () => {
      open(pages(2, { components: [h1, text('e1', 0)] }))
      fireEvent.click(screen.getByLabelText('text component e1'))
      fireEvent.click(screen.getByLabelText('text component h1'), { shiftKey: true })
      expect(named('h1')[0]).toHaveClass('canvas-component-selected')
      press(echoOn(1), { shiftKey: true })
      expect(named('h1')[0]).not.toHaveClass('canvas-component-selected')
      expect(screen.getByLabelText('text component e1')).toHaveClass('canvas-component-selected')
      // Toggled out, so the press does not move the current page: e1's page stays.
      namedOn('h1', 0)
    })

    it('leaves the selection alone when an echo is pressed while placing', async () => {
      const request = open(pages(2, { components: [h1] }))
      fireEvent.click(screen.getByRole('button', { name: 'Place Rectangle' }))
      fireEvent.pointerDown(echoOn(1), { pointerId: 1, button: 0, clientX: 10, clientY: 10 })
      await settle()
      expect(named('h1')[0]).not.toHaveClass('canvas-component-selected')
      namedOn('h1', 0)
      expect(sent(request)).toEqual([])
    })

    it('keeps one-page names while an overflow-sheet echo selects its component', () => {
      open({ ...canvas, contentWindowCount: 2, contentWindowOrigins: [0, 700_000], contentWindowPages: [0, 0], components: [h1] })
      const names = () => [...surfaces().map((surface) => surface.getAttribute('aria-label')), ...Array.from(document.querySelectorAll('.page-band')).map((band) => band.getAttribute('aria-label'))]
      const before = names()
      expect(before).toContain('Report page 2 of 2 with Page Header, Content, and Page Footer')
      expect(before).toContain('Page Header on page 2 of 2')
      press(echoOn(1))
      namedOn('h1', 0)
      expect(named('h1')[0]).toHaveClass('canvas-component-selected')
      expect(names()).toEqual(before)
    })

    it('names sheets and bands by designed page, adding the sheet on a continuation (D-4.1)', () => {
      open(continued([h1, text('e1', 0, 610_000)]))
      expect(surfaces().map((surface) => surface.getAttribute('aria-label'))).toEqual([
        'Report page 1 of 2 with Page Header, Content, and Page Footer',
        'Report page 1 of 2, sheet 2 of 3 with Page Header, Content, and Page Footer',
        'Report page 2 of 2 with Page Header, Content, and Page Footer',
      ])
      expect(Array.from(document.querySelectorAll('.page-band-content')).map((band) => band.getAttribute('aria-label'))).toEqual(['Content on page 1 of 2', 'Content on page 1 of 2, sheet 2 of 3', 'Content on page 2 of 2'])
      expect(screen.getByLabelText('Page Header on page 2 of 2')).toBeInTheDocument()
      expect(screen.getByLabelText('text component e1; on canvas sheet 2 of 3, which is a consequence of the content above it and can change when the data does — a column position, not a pin to sheet 2')).toBeInTheDocument()
    })
  })
})

// STARTUP TEMPLATES, STORY 3 — THE DIALOG AT LAUNCH.
//
// App is handed the examples only by `main.tsx`, once the engine is ready. Every
// row of the story's I/O matrix is here against the fake engine: launch, Blank
// and Escape, an example opened into Preview from its sample, a failed fetch,
// and App mounted without examples.
describe('the startup dialog at launch', () => {
  const examples = ['invoice', 'bank-statement', 'legal-contract', 'electricity-bill'].map((id) => ({ id, template: `/examples/${id}.folio`, sample: `/examples/${id}.sample.json`, thumbnail: `/examples/${id}.thumbnail.png` }))
  const TEMPLATE = new Uint8Array([4, 5, 6]).buffer
  const SAMPLE = '{"customer":{"name":"Ada"},"transactions":[{"amount":1}]}'
  let restoreFetch: typeof globalThis.fetch
  let fetchMock: ReturnType<typeof vi.fn>
  const answer = (body: ArrayBuffer, status = 200) => ({ ok: status >= 200 && status < 300, status, arrayBuffer: async () => body.slice(0) })
  beforeEach(() => {
    restoreFetch = globalThis.fetch
    fetchMock = vi.fn(async (url: string) => url.endsWith('.json') ? answer(new TextEncoder().encode(SAMPLE).buffer) : answer(TEMPLATE))
    globalThis.fetch = fetchMock as never
  })
  afterEach(() => { globalThis.fetch = restoreFetch })

  // The starter at revision 1, and an engine that loads an example at revision 2
  // and renders it.
  const launch = (props: Partial<Parameters<typeof App>[0]> = {}) => {
    const starter = { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas }
    const opened = { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas }
    let current = starter
    const request = vi.fn(async (operation: string) => {
      if (operation === 'load') { current = opened; return { snapshot: opened } }
      if (operation === 'serialize') return { snapshot: current, bytes: TEMPLATE }
      if (operation === 'stand-in-data') return { snapshot: current, bytes: new TextEncoder().encode('{}').buffer }
      if (operation === 'identity') return { snapshot: current, preview: { revision: current.revision, identity: 'c'.repeat(64) } }
      if (operation === 'render') return { snapshot: current, bytes: new Uint8Array([9]).buffer, preview: { revision: current.revision, identity: 'c'.repeat(64), pdfSha256: PDF_FIXTURE_DIGEST, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
      return { snapshot: current }
    })
    render(<App engine={engine(request as never)} initialSnapshot={starter} blankBytes={bytes} examples={examples} {...props} />)
    return request
  }
  const dialog = () => screen.getByRole('dialog', { name: 'New template' })
  const card = (name: string) => within(dialog()).getByRole('button', { name })

  it('opens at launch with Blank and the four examples, Blank selected and focused, and a Cancel', () => {
    const request = launch()
    expect(dialog()).toHaveAttribute('aria-modal', 'true')
    const cards = within(within(dialog()).getByRole('group', { name: 'Start from' })).getAllByRole('button')
    expect(cards.map((entry) => entry.getAttribute('aria-label'))).toEqual(['Blank', 'Invoice', 'Bank Statement', 'Legal Contract', 'Electricity Bill'])
    expect(cards.map((entry) => entry.getAttribute('aria-pressed'))).toEqual(['true', 'false', 'false', 'false', 'false'])
    expect(card('Blank')).toHaveFocus()
    expect(Array.from(dialog().querySelectorAll('img')).map((image) => image.getAttribute('src'))).toEqual(examples.map((example) => example.thumbnail))
    expect(within(dialog()).getByTestId('startup-blank-page')).toBeInTheDocument()
    expect(card('Invoice')).toHaveAccessibleDescription('Line items, totals, payment QR invoice.sample.json')
    expect(card('Blank')).toHaveAccessibleDescription('Empty A4 page no sample data')
    expect(within(dialog()).getByRole('status')).toHaveTextContent('Blank starts an empty A4 page with no sample data')
    expect(within(dialog()).getByRole('button', { name: 'Start blank' })).toBeInTheDocument()
    expect(within(dialog()).getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
    fireEvent.click(card('Invoice'))
    expect(within(dialog()).getByRole('status')).toHaveTextContent('Invoice opens in Preview with invoice.sample.json')
    expect(within(dialog()).getByRole('button', { name: 'Open example' })).toBeInTheDocument()
    expect(fetchMock).not.toHaveBeenCalled()
    expect(request).not.toHaveBeenCalled()
  })

  it('does not open when App is mounted without examples', () => {
    render(<App engine={engine()} initialSnapshot={snapshot(1)} />)
    expect(screen.queryByRole('dialog', { name: 'New template' })).not.toBeInTheDocument()
  })

  it.each([
    ['Start blank', () => { fireEvent.click(card('Blank')); expect(within(dialog()).getByRole('status')).toHaveTextContent('Blank starts an empty A4 page'); fireEvent.click(within(dialog()).getByRole('button', { name: 'Start blank' })) }],
    ['Cancel', () => { fireEvent.click(card('Invoice')); fireEvent.click(within(dialog()).getByRole('button', { name: 'Cancel' })) }],
    ['Escape', () => { fireEvent.keyDown(card('Invoice'), { key: 'Escape' }) }],
  ])('%s closes the dialog on the starter at revision 1 with no engine request', async (_, dismiss) => {
    const request = launch()
    dismiss()
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New template' })).not.toBeInTheDocument())
    expect(request).not.toHaveBeenCalled()
    expect(fetchMock).not.toHaveBeenCalled()
    expect(screen.getByTestId('engine-snapshot')).toHaveTextContent('REVISION 1')
    expect(screen.getByText('Untitled template')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'PREVIEW' })).toHaveAttribute('aria-pressed', 'false')
  })

  it.each([
    ['the primary action', () => { fireEvent.click(card('Bank Statement')); fireEvent.click(within(dialog()).getByRole('button', { name: 'Open example' })) }],
    ['Enter on the card', () => { fireEvent.keyDown(card('Bank Statement'), { key: 'Enter' }) }],
    ['a double-click on the card', () => { fireEvent.doubleClick(card('Bank Statement')) }],
  ])('opens an example through %s: titled, unsaved, untargeted, in Preview from its sample', async (_, confirm) => {
    const request = launch()
    confirm()
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New template' })).not.toBeInTheDocument())
    expect(fetchMock).toHaveBeenCalledWith('/examples/bank-statement.folio', expect.objectContaining({ credentials: 'omit' }))
    expect(fetchMock).toHaveBeenCalledWith('/examples/bank-statement.sample.json', expect.objectContaining({ credentials: 'omit' }))
    expect(request).toHaveBeenCalledWith('load', TEMPLATE, expect.any(AbortSignal))
    expect(screen.getByText('Bank Statement', { selector: '.document-name' })).toBeInTheDocument()
    expect(screen.getByText('Unsaved local changes')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'PREVIEW' })).toHaveAttribute('aria-pressed', 'true')
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'render')).toHaveLength(1))
    const rendered = request.mock.calls.find(([operation]) => operation === 'render') as unknown as [string, { data: ArrayBuffer }]
    expect(new TextDecoder().decode(rendered[1].data)).toBe(SAMPLE)
    expect(request.mock.calls.some(([operation]) => operation === 'stand-in-data')).toBe(false)
    expect(screen.queryByRole('note', { name: 'No-data preview notice' })).not.toBeInTheDocument()
    // The sample tree is the one Load sample JSON would have installed.
    fireEvent.click(screen.getByRole('tab', { name: 'DATA' }))
    expect(screen.getByRole('tree', { name: 'Sample data paths' })).toHaveTextContent('customer')
  })

  it('an example opens with no file target, so Save asks where to write it', async () => {
    const acquireSaveTarget = vi.fn(async (_request: SaveTargetRequest): Promise<AcquiredSaveTarget> => { throw new FileAccessCancelled() })
    launch({ fileAccess: { open: vi.fn(), acquireSaveTarget, writeSave: vi.fn() } })
    fireEvent.click(card('Invoice'))
    fireEvent.click(within(dialog()).getByRole('button', { name: 'Open example' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New template' })).not.toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Save local template' }))
    await waitFor(() => expect(acquireSaveTarget).toHaveBeenCalledOnce())
    expect(acquireSaveTarget.mock.calls[0]![0]).toMatchObject({ suggestedName: 'Invoice', currentTarget: undefined, saveAs: false })
  })

  it('keeps the dialog open with the failure named when a fetch fails, loads nothing, and Blank still works', async () => {
    fetchMock.mockImplementation(async (url: string) => url.includes('bank-statement') ? answer(new ArrayBuffer(0), 404) : answer(TEMPLATE))
    const request = launch()
    fireEvent.click(card('Bank Statement'))
    fireEvent.click(within(dialog()).getByRole('button', { name: 'Open example' }))
    const alert = await within(dialog()).findByRole('alert')
    expect(alert).toHaveTextContent('Could not open Bank Statement')
    expect(alert).toHaveTextContent('HTTP 404')
    expect(request).not.toHaveBeenCalled()
    expect(screen.getByText('Untitled template')).toBeInTheDocument()
    fireEvent.keyDown(card('Bank Statement'), { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New template' })).not.toBeInTheDocument())
    expect(request).not.toHaveBeenCalled()
    expect(screen.getByTestId('engine-snapshot')).toHaveTextContent('REVISION 1')
  })

  const expectDismissedOnStarter = async () => {
    fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New template' })).not.toBeInTheDocument())
    expect(screen.getByTestId('engine-snapshot')).toHaveTextContent('REVISION 1')
  }

  it('refuses an example whose sample is not valid JSON with nothing replaced', async () => {
    fetchMock.mockImplementation(async (url: string) => url.endsWith('.json') ? answer(new TextEncoder().encode('{"customer":').buffer) : answer(TEMPLATE))
    const request = launch()
    fireEvent.click(card('Invoice'))
    fireEvent.click(within(dialog()).getByRole('button', { name: 'Open example' }))
    expect(await within(dialog()).findByRole('alert')).toHaveTextContent('Could not open Invoice')
    expect(request).not.toHaveBeenCalled()
    expect(screen.getByText('Untitled template')).toBeInTheDocument()
    await expectDismissedOnStarter()
    expect(request).not.toHaveBeenCalled()
  })

  it('keeps the dialog usable when the engine rejects the example\'s template', async () => {
    launch({ engine: engine(vi.fn(async (operation: string) => { if (operation === 'load') throw new Error('engine refused'); return { snapshot: snapshot(1) } }) as never) })
    fireEvent.click(card('Invoice'))
    fireEvent.click(within(dialog()).getByRole('button', { name: 'Open example' }))
    expect(await within(dialog()).findByRole('alert')).toHaveTextContent('Could not open Invoice')
    expect(screen.getByRole('button', { name: 'PREVIEW' })).toHaveAttribute('aria-pressed', 'false')
    expect(screen.getByText('Untitled template')).toBeInTheDocument()
    expect(screen.queryByText(/Opening example/)).not.toBeInTheDocument()
    await expectDismissedOnStarter()
  })

  it('gives up on a fetch that never settles, names the failure, and lets Escape close', async () => {
    fetchMock.mockImplementation((_url: string, init?: RequestInit) => new Promise((_, reject) => { init?.signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError'))) }))
    vi.useFakeTimers()
    try {
      launch()
      fireEvent.click(card('Invoice'))
      fireEvent.click(within(dialog()).getByRole('button', { name: 'Open example' }))
      expect(dialog()).toHaveAttribute('aria-busy', 'true')
      await act(async () => { await vi.advanceTimersByTimeAsync(20_000) })
      expect(within(dialog()).getByRole('alert')).toHaveTextContent('Could not open Invoice: its bundled file did not arrive in time')
      fireEvent.keyDown(card('Invoice'), { key: 'Escape' })
      expect(screen.queryByRole('dialog', { name: 'New template' })).not.toBeInTheDocument()
    } finally { vi.useRealTimers() }
  })

  it('keeps focus inside the dialog when a non-focusable part of it is clicked', async () => {
    launch()
    const title = within(dialog()).getByRole('heading', { name: 'New template' })
    // What a browser does on that click: focus leaves the card for the nearest
    // focusable ancestor of the click target — or for the body, if there is none.
    act(() => { card('Blank').blur(); fireEvent.mouseDown(title); (title.closest('[tabindex]') as HTMLElement | null)?.focus(); fireEvent.click(title) })
    expect(dialog()).toHaveFocus()
    await expectDismissedOnStarter()
  })

  it('traps Tab inside the dialog in both directions', () => {
    launch()
    const first = card('Blank')
    const last = within(dialog()).getByRole('button', { name: 'Start blank' })
    last.focus()
    fireEvent.keyDown(last, { key: 'Tab' })
    expect(first).toHaveFocus()
    fireEvent.keyDown(first, { key: 'Tab', shiftKey: true })
    expect(last).toHaveFocus()
  })

  it('pulls focus back into the cycle when the dialog section itself holds it', () => {
    launch()
    act(() => { dialog().focus() })
    fireEvent.keyDown(dialog(), { key: 'Tab', shiftKey: true })
    expect(within(dialog()).getByRole('button', { name: 'Start blank' })).toHaveFocus()
    act(() => { dialog().focus() })
    fireEvent.keyDown(dialog(), { key: 'Tab' })
    expect(card('Blank')).toHaveFocus()
  })

  it('lets no App shortcut fire while it is open, and releases them once it closes', async () => {
    const acquireSaveTarget = vi.fn(async (_request: SaveTargetRequest): Promise<AcquiredSaveTarget> => { throw new FileAccessCancelled() })
    launch({ fileAccess: { open: vi.fn(), acquireSaveTarget, writeSave: vi.fn() } })
    const mac = isMacPlatform()
    const saveKey = createEvent.keyDown(window, { key: 's', ctrlKey: !mac, metaKey: mac })
    fireEvent(window, saveKey)
    expect(saveKey.defaultPrevented, 'the browser must not open Save Page behind the dialog').toBe(true)
    fireEvent.keyDown(window, { key: 'p', altKey: true })
    expect(acquireSaveTarget).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'PREVIEW' })).toHaveAttribute('aria-pressed', 'false')
    fireEvent.keyDown(card('Invoice'), { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New template' })).not.toBeInTheDocument())
    fireEvent.keyDown(window, { key: 's', ctrlKey: !mac, metaKey: mac })
    await waitFor(() => expect(acquireSaveTarget).toHaveBeenCalledOnce())
  })
})

// spec-startup-templates STORY 4 — NEW…, OPEN EXISTING FILE… AND THE
// UNSAVED-CHANGES WARNING, which (owner renegotiation) comes BEFORE the startup
// dialog. Every row of the story's revised I/O matrix.
describe('New… and the startup dialog reopened', () => {
  const examples = ['invoice', 'bank-statement', 'legal-contract', 'electricity-bill'].map((id) => ({ id, template: `/examples/${id}.folio`, sample: `/examples/${id}.sample.json`, thumbnail: `/examples/${id}.thumbnail.png` }))
  const TEMPLATE = new Uint8Array([4, 5, 6]).buffer
  const SAMPLE = '{"customer":{"name":"Ada"}}'
  let restoreFetch: typeof globalThis.fetch
  let fetchMock: ReturnType<typeof vi.fn>
  const answer = (body: ArrayBuffer) => ({ ok: true, status: 200, arrayBuffer: async () => body.slice(0) })
  beforeEach(() => {
    restoreFetch = globalThis.fetch
    fetchMock = vi.fn(async (url: string) => url.endsWith('.json') ? answer(new TextEncoder().encode(SAMPLE).buffer) : answer(TEMPLATE))
    globalThis.fetch = fetchMock as never
  })
  afterEach(() => { globalThis.fetch = restoreFetch })

  type Snap = { documentState: 'loaded'; revision: number; byteLength: number; canvas: typeof canvas; canUndo?: boolean }
  // The starter at revision 1. A command is a real edit (next revision, Undo
  // available); a load installs a fresh document at the next revision.
  const mount = (props: Partial<Parameters<typeof App>[0]> = {}, failLoad = false) => {
    let current: Snap = { documentState: 'loaded', revision: 1, byteLength: 3, canvas }
    const request = vi.fn(async (operation: string) => {
      if (operation === 'command') { current = { ...current, revision: current.revision + 1, canUndo: true }; return { snapshot: current } }
      if (operation === 'load') { if (failLoad) throw new Error('engine refused'); current = { documentState: 'loaded', revision: current.revision + 1, byteLength: 3, canvas }; return { snapshot: current } }
      if (operation === 'serialize') return { snapshot: current, bytes: TEMPLATE }
      if (operation === 'stand-in-data') return { snapshot: current, bytes: new TextEncoder().encode('{}').buffer }
      if (operation === 'identity') return { snapshot: current, preview: { revision: current.revision, identity: 'c'.repeat(64) } }
      if (operation === 'render') return { snapshot: current, bytes: new Uint8Array([9]).buffer, preview: { revision: current.revision, identity: 'c'.repeat(64), pdfSha256: PDF_FIXTURE_DIGEST, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } }
      return { snapshot: current }
    })
    render(<App engine={engine(request as never)} initialSnapshot={current} blankBytes={bytes} {...props} />)
    return request
  }
  const files = (open: FileAccess['open'] = vi.fn()): FileAccess => ({ open, acquireSaveTarget: vi.fn(), writeSave: vi.fn() })
  const dialog = () => screen.getByRole('dialog', { name: 'New template' })
  const queryDialog = () => screen.queryByRole('dialog', { name: 'New template' })
  const warning = () => screen.getByRole('dialog', { name: 'Discard unsaved changes?' })
  const queryWarning = () => screen.queryByRole('dialog', { name: 'Discard unsaved changes?' })
  const card = (name: string) => within(dialog()).getByRole('button', { name })
  const dismissLaunch = () => fireEvent.keyDown(card('Blank'), { key: 'Escape' })
  const edit = async () => {
    fireEvent.click(within(screen.getByLabelText('Canvas controls')).getByRole('button', { name: 'Add page' }))
    await waitFor(() => expect(screen.getByTestId('engine-snapshot')).toHaveTextContent('REVISION 2'))
    expect(screen.getByRole('button', { name: 'Undo' })).toBeEnabled()
  }
  const expectEditedDocumentIntact = () => {
    expect(screen.getByTestId('engine-snapshot')).toHaveTextContent('REVISION 2')
    expect(screen.getByRole('button', { name: 'Undo' })).toBeEnabled()
  }
  const openInvoice = () => { fireEvent.click(card('Invoice')); fireEvent.click(within(dialog()).getByRole('button', { name: 'Open example' })) }
  // Launch, dismiss, make a real edit, press New… and Discard the warning.
  const discardedOverEdits = async (props: Partial<Parameters<typeof App>[0]> = {}, failLoad = false) => {
    const request = mount({ examples, ...props }, failLoad)
    dismissLaunch()
    await edit()
    const before = request.mock.calls.length
    fireEvent.click(screen.getByRole('button', { name: 'New…' }))
    fireEvent.click(within(warning()).getByRole('button', { name: 'Discard' }))
    return { request, before }
  }

  it('names the document bar button New…, keeps its glyph, and opens the dialog with Blank selected and focused', () => {
    mount({ examples })
    dismissLaunch()
    const button = screen.getByRole('button', { name: 'New…' })
    expect(button).toHaveAttribute('data-tip', 'New…')
    expect(button.querySelector('svg.tool-icon')).not.toBeNull()
    expect(screen.queryByRole('button', { name: 'Start blank' })).not.toBeInTheDocument()
    fireEvent.click(button)
    expect(card('Blank')).toHaveAttribute('aria-pressed', 'true')
    expect(card('Blank')).toHaveFocus()
  })

  it('offers Blank without examples, so New… works in any App', () => {
    mount()
    fireEvent.click(screen.getByRole('button', { name: 'New…' }))
    expect(within(within(dialog()).getByRole('group', { name: 'Start from' })).getAllByRole('button').map((entry) => entry.getAttribute('aria-label'))).toEqual(['Blank'])
  })

  it('New… with no unsaved changes opens the startup dialog directly, with no warning', () => {
    const request = mount({ examples })
    dismissLaunch()
    fireEvent.click(screen.getByRole('button', { name: 'New…' }))
    expect(queryWarning()).not.toBeInTheDocument()
    expect(card('Blank')).toHaveAttribute('aria-pressed', 'true')
    expect(request).not.toHaveBeenCalled()
  })

  it('treats a just-opened example as having no real edits, though the bar still says unsaved', async () => {
    mount({ examples })
    openInvoice()
    await waitFor(() => expect(queryDialog()).not.toBeInTheDocument())
    expect(screen.getByText('Unsaved local changes')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'New…' }))
    expect(queryWarning()).not.toBeInTheDocument()
    expect(queryDialog()).toBeInTheDocument()
  })

  it('a just-saved document opens the dialog without a warning', async () => {
    const saving: FileAccess = { open: vi.fn(), acquireSaveTarget: vi.fn(async () => ({ name: 'saved.folio', format: folioFileFormat })), writeSave: vi.fn(async () => ({ name: 'saved.folio' })) }
    mount({ examples, fileAccess: saving })
    dismissLaunch()
    await edit()
    fireEvent.click(screen.getByRole('button', { name: 'Save local template' }))
    await waitFor(() => expect(screen.getByText('Saved local file')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'New…' }))
    expect(queryWarning()).not.toBeInTheDocument()
    expect(queryDialog()).toBeInTheDocument()
  })

  it('New… with real edits shows the warning naming the document, Keep editing focused, no startup dialog and nothing requested', async () => {
    const request = mount({ examples })
    dismissLaunch()
    await edit()
    const before = request.mock.calls.length
    fireEvent.click(screen.getByRole('button', { name: 'New…' }))
    expect(warning()).toHaveAttribute('aria-modal', 'true')
    expect(warning()).toHaveAccessibleDescription('Untitled template has unsaved changes.')
    expect(warning().querySelector('.unsaved-warning-dot')).not.toBeNull()
    expect(within(warning()).getByText('Untitled template')).toHaveClass('unsaved-warning-document')
    expect(within(warning()).getByRole('button', { name: 'Keep editing' })).toHaveFocus()
    expect(within(warning()).getAllByRole('button').map((button) => button.textContent)).toEqual(['Keep editing', 'Discard'])
    expect(queryDialog()).not.toBeInTheDocument()
    expect(request.mock.calls.length).toBe(before)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('keeps Tab inside the warning, cycling Keep editing and Discard', async () => {
    mount({ examples })
    dismissLaunch()
    await edit()
    fireEvent.click(screen.getByRole('button', { name: 'New…' }))
    const keep = within(warning()).getByRole('button', { name: 'Keep editing' })
    const discard = within(warning()).getByRole('button', { name: 'Discard' })
    fireEvent.keyDown(keep, { key: 'Tab' })
    expect(discard).toHaveFocus()
    fireEvent.keyDown(discard, { key: 'Tab' })
    expect(keep).toHaveFocus()
    fireEvent.keyDown(keep, { key: 'Tab', shiftKey: true })
    expect(discard).toHaveFocus()
    expect(queryDialog()).not.toBeInTheDocument()
  })

  it.each([
    ['Keep editing', () => fireEvent.click(within(warning()).getByRole('button', { name: 'Keep editing' }))],
    ['Escape', () => fireEvent.keyDown(within(warning()).getByRole('button', { name: 'Discard' }), { key: 'Escape' })],
  ])('%s closes the warning with no startup dialog, nothing requested, and the document and Undo intact', async (_, keepEditing) => {
    const request = mount({ examples })
    dismissLaunch()
    await edit()
    const before = request.mock.calls.length
    fireEvent.click(screen.getByRole('button', { name: 'New…' }))
    keepEditing()
    expect(queryWarning()).not.toBeInTheDocument()
    expect(queryDialog()).not.toBeInTheDocument()
    expect(request.mock.calls.length).toBe(before)
    expectEditedDocumentIntact()
  })

  it('lets no App shortcut fire while the warning is open', async () => {
    const request = mount({ examples })
    dismissLaunch()
    await edit()
    const before = request.mock.calls.length
    fireEvent.click(screen.getByRole('button', { name: 'New…' }))
    const mac = isMacPlatform()
    fireEvent.keyDown(window, { key: 'z', ctrlKey: !mac, metaKey: mac })
    fireEvent.keyDown(window, { key: 'p', altKey: true })
    await act(async () => { await Promise.resolve() })
    expect(request.mock.calls.length).toBe(before)
    expect(screen.getByRole('button', { name: 'PREVIEW' })).toHaveAttribute('aria-pressed', 'false')
    expectEditedDocumentIntact()
  })

  it('Discard opens the startup dialog with Blank selected and replaces nothing', async () => {
    const { request, before } = await discardedOverEdits()
    expect(queryWarning()).not.toBeInTheDocument()
    expect(card('Blank')).toHaveAttribute('aria-pressed', 'true')
    expect(card('Blank')).toHaveFocus()
    expect(request.mock.calls.length).toBe(before)
    expectEditedDocumentIntact()
  })

  it.each([
    ['Cancel', () => fireEvent.click(within(dialog()).getByRole('button', { name: 'Cancel' }))],
    ['Escape', () => fireEvent.keyDown(card('Invoice'), { key: 'Escape' })],
  ])('%s after Discard closes the dialog with the revision and Undo untouched', async (_, dismiss) => {
    const { request, before } = await discardedOverEdits()
    fireEvent.click(card('Invoice'))
    dismiss()
    expect(queryDialog()).not.toBeInTheDocument()
    expect(request.mock.calls.length).toBe(before)
    expect(fetchMock).not.toHaveBeenCalled()
    expectEditedDocumentIntact()
  })

  it('an example after Discard opens in Preview with no further question', async () => {
    const { request } = await discardedOverEdits()
    openInvoice()
    await waitFor(() => expect(queryDialog()).not.toBeInTheDocument())
    expect(queryWarning()).not.toBeInTheDocument()
    expect(request).toHaveBeenCalledWith('load', TEMPLATE, expect.any(AbortSignal))
    expect(screen.getByText('Invoice', { selector: '.document-name' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'PREVIEW' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('an example that fails to load after Discard keeps the dialog open with an alert', async () => {
    await discardedOverEdits({}, true)
    openInvoice()
    expect(await within(dialog()).findByRole('alert')).toHaveTextContent('Could not open Invoice')
    expect(queryDialog()).toBeInTheDocument()
  })

  it('Blank after Discard loads the starter, titled Untitled template, and closes', async () => {
    const request = mount({ examples })
    openInvoice()
    await waitFor(() => expect(queryDialog()).not.toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'DESIGN' }))
    fireEvent.click(within(screen.getByLabelText('Canvas controls')).getByRole('button', { name: 'Add page' }))
    await waitFor(() => expect(screen.getByTestId('engine-snapshot')).toHaveTextContent('REVISION 3'))
    fireEvent.click(screen.getByRole('button', { name: 'New…' }))
    expect(warning()).toHaveAccessibleDescription('Invoice has unsaved changes.')
    fireEvent.click(within(warning()).getByRole('button', { name: 'Discard' }))
    fireEvent.click(within(dialog()).getByRole('button', { name: 'Start blank' }))
    expect(queryWarning()).not.toBeInTheDocument()
    await waitFor(() => expect(queryDialog()).not.toBeInTheDocument())
    expect(request).toHaveBeenCalledWith('load', bytes, expect.any(AbortSignal))
    expect(screen.getByText('Untitled template', { selector: '.document-name' })).toBeInTheDocument()
  })

  it('a press inside the warning keeps its keys working: Escape closes it, Tab lands on its buttons', async () => {
    mount({ examples })
    dismissLaunch()
    await edit()
    fireEvent.click(screen.getByRole('button', { name: 'New…' }))
    // What a browser does on a press on the heading: focus goes to the nearest
    // focusable ancestor — the section — rather than to <body>.
    act(() => { warning().focus() })
    expect(warning()).toHaveFocus()
    fireEvent.keyDown(warning(), { key: 'Tab' })
    expect([within(warning()).getByRole('button', { name: 'Keep editing' }), within(warning()).getByRole('button', { name: 'Discard' })]).toContain(document.activeElement)
    act(() => { warning().focus() })
    fireEvent.keyDown(warning(), { key: 'Escape' })
    expect(queryWarning()).not.toBeInTheDocument()
    expect(queryDialog()).not.toBeInTheDocument()
    expectEditedDocumentIntact()
  })

  it('Start blank resets the real-edits baseline, so New… afterwards opens the dialog without a warning', async () => {
    await discardedOverEdits()
    fireEvent.click(within(dialog()).getByRole('button', { name: 'Start blank' }))
    await waitFor(() => expect(queryDialog()).not.toBeInTheDocument())
    await waitFor(() => expect(screen.getByText('Untitled template', { selector: '.document-name' })).toBeInTheDocument())
    expect(screen.getByText('Started an unnamed local template')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'New…' }))
    expect(queryWarning()).not.toBeInTheDocument()
    expect(queryDialog()).toBeInTheDocument()
  })

  it('Blank that fails keeps a reopened dialog open and names the failure', async () => {
    mount({}, true)
    fireEvent.click(screen.getByRole('button', { name: 'New…' }))
    fireEvent.click(within(dialog()).getByRole('button', { name: 'Start blank' }))
    expect(await within(dialog()).findByRole('alert')).toHaveTextContent('Could not start a blank local template')
  })

  it('launch Blank still closes with no engine request', () => {
    const request = mount({ examples })
    fireEvent.click(within(dialog()).getByRole('button', { name: 'Start blank' }))
    expect(queryDialog()).not.toBeInTheDocument()
    expect(request).not.toHaveBeenCalled()
  })

  it('shows Open existing file… only when local file access exists', () => {
    mount({ examples })
    expect(within(dialog()).queryByRole('button', { name: 'Open existing file…' })).not.toBeInTheDocument()
  })

  it('Open existing file… at launch opens a picked .folio, closes the dialog and stays in the current mode', async () => {
    const open = vi.fn(async () => ({ bytes: TEMPLATE, name: 'statement.folio' }))
    const request = mount({ examples, fileAccess: files(open) })
    const button = within(dialog()).getByRole('button', { name: 'Open existing file…' })
    expect(button.querySelector('svg.tool-icon')).not.toBeNull()
    fireEvent.click(button)
    expect(open).toHaveBeenCalledOnce()
    await waitFor(() => expect(queryDialog()).not.toBeInTheDocument())
    expect(request).toHaveBeenCalledWith('load', TEMPLATE, expect.any(AbortSignal))
    expect(screen.getByText('statement.folio', { selector: '.document-name' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'PREVIEW' })).toHaveAttribute('aria-pressed', 'false')
  })

  it('Open existing file… after Discard opens the file with no further question', async () => {
    const open = vi.fn(async () => ({ bytes: TEMPLATE, name: 'statement.folio' }))
    await discardedOverEdits({ fileAccess: files(open) })
    fireEvent.click(within(dialog()).getByRole('button', { name: 'Open existing file…' }))
    expect(open).toHaveBeenCalledOnce()
    expect(queryWarning()).not.toBeInTheDocument()
    await waitFor(() => expect(queryDialog()).not.toBeInTheDocument())
    expect(screen.getByText('statement.folio', { selector: '.document-name' })).toBeInTheDocument()
  })

  it('a cancelled picker keeps the dialog open with no message and the document untouched', async () => {
    const open = vi.fn(async () => { throw new FileAccessCancelled() })
    const { request, before } = await discardedOverEdits({ fileAccess: files(open) })
    fireEvent.click(within(dialog()).getByRole('button', { name: 'Open existing file…' }))
    await waitFor(() => expect(open).toHaveBeenCalledOnce())
    await act(async () => { await Promise.resolve() })
    expect(queryDialog()).toBeInTheDocument()
    expect(within(dialog()).queryByRole('alert')).not.toBeInTheDocument()
    expect(request.mock.calls.length).toBe(before)
    expectEditedDocumentIntact()
  })

  it('an invalid file shows its failure in the footer and keeps the dialog open', async () => {
    const open = vi.fn(async () => ({ bytes: TEMPLATE, name: 'broken.folio' }))
    mount({ examples, fileAccess: files(open) }, true)
    fireEvent.click(within(dialog()).getByRole('button', { name: 'Open existing file…' }))
    expect(await within(dialog()).findByRole('alert')).toHaveTextContent('Could not open local file')
    expect(queryDialog()).toBeInTheDocument()
  })
})


describe('the update prompt', () => {
  const pending = (mandatory: boolean, pendingVersion = '2.0.0'): OfflineLifecycle => ({ state: 'update-available', cacheReady: true, verifiedAssetUrls: [], pendingVersion, mandatory })
  // The workspace only exists once an engine does, and the prompt lives in the
  // workspace: an author is never blocked by it before they have a document.
  const workspace = (loadState: OfflineLifecycle, files?: FileAccess) => {
    const request = vi.fn(async (operation: string) => ({ snapshot: snapshot(7), ...(operation === 'serialize' ? { bytes } : {}) }))
    return render(<App engine={engine(request)} fileAccess={files ?? { open: vi.fn(async () => ({ bytes, name: 'report.folio' })), acquireSaveTarget: vi.fn(), writeSave: vi.fn() }} initialSnapshot={snapshot(1)} offlineState={loadState.state} loadState={loadState} />)
  }

  it('offers an optional update the author can refuse, and stops asking once refused', () => {
    workspace(pending(false))
    expect(screen.getByRole('heading', { name: 'Update available' })).toBeInTheDocument()
    expect(screen.getByRole('dialog')).toHaveTextContent('Version 2.0.0 of folio8 is ready')
    // THE RUNNING RELEASE IS STILL GOOD, and the copy says so rather than
    // implying the author is running something broken.
    expect(screen.getByRole('dialog')).toHaveTextContent('This version keeps working')
    fireEvent.click(screen.getByRole('button', { name: 'Later' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('blocks on a required update and refuses to be dismissed', () => {
    workspace(pending(true))
    expect(screen.getByRole('heading', { name: 'Update required' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Later' })).not.toBeInTheDocument()
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  // ⚠ THE ONE THAT PROTECTS THE AUTHOR'S WORK. Activating reloads the tab, and a
  // reload takes unsaved edits with it. A required update on a dirty document
  // therefore offers NO WAY TO UPGRADE AT ALL — only a way to save. The block and
  // the document are not in tension: the block simply waits for the save.
  it('offers a required update no upgrade button at all while the document is unsaved', () => {
    workspace(pending(true))
    expect(screen.getByText('Unsaved local changes')).toBeInTheDocument()
    expect(screen.getByRole('dialog')).toHaveTextContent('has unsaved changes')
    expect(screen.queryByRole('button', { name: 'Upgrade now' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save As…' })).toBeInTheDocument()
  })

  it('offers the upgrade once the document is safe', async () => {
    workspace(pending(true))
    fireEvent.click(screen.getByRole('button', { name: 'Open local template' }))
    await waitFor(() => expect(screen.getByText('Saved local file')).toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'Upgrade now' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Save As…' })).not.toBeInTheDocument()
  })

  it('names the update generically when the waiting worker never said what it was', () => {
    workspace({ state: 'update-available', cacheReady: true, verifiedAssetUrls: [], mandatory: false })
    expect(screen.getByRole('dialog')).toHaveTextContent('A new version of folio8 is ready')
  })

  it('shows no prompt at all when there is no pending release', () => {
    workspace({ state: 'ready', cacheReady: true, verifiedAssetUrls: [] })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})
