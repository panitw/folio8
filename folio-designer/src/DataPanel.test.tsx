import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import App, { PROSE_COMMIT_DEBOUNCE_MS } from './App'
import { DataPanel } from './DataPanel'
import { acceptSampleData } from './sample-data'
import type { EngineClient } from './engine-client'
import { FileAccessCancelled, jsonSampleFileFormat, type AcquiredSaveTarget, type SavedLocalFile, type SaveRequest, type SaveTargetRequest } from './file/file-access'
import type { SampleFileAccess } from './sample-file'
import { PDF_FIXTURE_DIGEST, RENDER_ELAPSED_MS, RENDER_ENGINE_VERSION } from './test/pdf-fixture'
import { startBlankFromNew } from './test/new-document'

// face() builds the PROJECTED shape of a named-face chain entry (Story 8.3:
// an entry is a discriminated object, not a string). A named face carries no
// family and no style — its name is its identity.
const face = (name: string) => ({ face: name, assetKey: '', family: '', style: '', bold: '', italic: '', boldItalic: '' })

vi.mock('./preview/pdf-viewer', () => ({
  initialPDFPreviewViewState: { page: 1, scale: 1, ['scroll' + 'Top']: 0, ['scroll' + 'Left']: 0 },
  samePDFPreviewViewState: () => true,
  PDFPreviewViewer: () => null,
}))

const sampleBytes = new TextEncoder().encode('{"customer":{"name":"Ada"},"items":[{"sku":"A-1"}]}').buffer
const replacementBytes = new TextEncoder().encode('{"report":{"id":2}}').buffer
const canvas = { width: 595276, height: 841890, orientation: 'portrait' as const, preset: 'A4' as const, locale: 'en' as const, utcOffset: '+00:00', embedFonts: true, marginTop: 36000, marginRight: 36000, marginBottom: 36000, marginLeft: 36000, gridIncrement: 6000, commandWidth: 595276, commandHeight: 841890, fontFamilies: ['body'], fontChains: [{ name: 'body', entries: [face('Noto Sans')] }], defaultFontSize: 12000, defaultLineSpacing: 1000, contentWindowHeight: 729890, contentWindowCount: 1, contentWindowOrigins: [0], contentWindowPages: [0], contentWindowCountIsExact: true, bands: [{ name: 'pageHeader' as const, x: 36000, y: 36000, width: 523276, height: 20000 }, { name: 'content' as const, x: 36000, y: 56000, width: 523276, height: 729890 }, { name: 'pageFooter' as const, x: 36000, y: 785890, width: 523276, height: 20000 }], components: [] }
const snapshot = { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas }

const textCanvas = { ...canvas, components: [{ id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Text' }] }
// STORY 14.4 / AC2. A canvas whose one component CANNOT receive a scalar
// binding, and a mixed one, so the fifth ladder arm can be exercised against a
// real selection rather than against a prop set by hand.
const lineCanvas = { ...canvas, components: [{ id: 'e1', type: 'line' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 1_000, resizable: true, background: '#000000' }] }
const tableCanvas = { ...canvas, components: [{ id: 'e1', type: 'table' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 12_000, resizable: false, tableBind: 'transactions[]' }] }
const mixedCanvas = { ...canvas, components: [...textCanvas.components, { id: 'e2', type: 'rect' as const, band: 'content' as const, x: 0, y: 100_000, width: 72_000, height: 24_000, resizable: true, background: '#1b2a4a' }] }
const rectCanvas = { ...canvas, components: [{ id: 'e1', type: 'rect' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, background: '#1b2a4a' }] }
const imageCanvas = { ...canvas, components: [{ id: 'e1', type: 'image' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true }] }

const openDataTab = () => fireEvent.click(screen.getByRole('tab', { name: 'DATA' }))
const settleFrames = async () => { await Promise.resolve(); await Promise.resolve(); await Promise.resolve() }

describe('whole-table collection picks', () => {
  const row = (label: string) => screen.getAllByRole('treeitem').find((item) => item.querySelector('.tree-label')?.textContent === label)!
  const collections = () => acceptSampleData('collections.json', new TextEncoder().encode('{"items":[],"report":{"transactions":[{"ref":"A","nested":[{"id":1}]}]},"value":12}').buffer)
  const panel = (sample = collections(), props: Partial<React.ComponentProps<typeof DataPanel>> = {}) => <DataPanel sample={sample} busy={false} available selectedComponentId="e1" selectedComponentType="table" onLoad={() => undefined} {...props} />

  it.each(['click', 'Enter', ' '] as const)('binds populated and empty root-addressable arrays with %s, and activation toggles disclosure', (gesture) => {
    const onConnect = vi.fn()
    render(panel(collections(), { onConnect }))
    const activate = (node: HTMLElement) => gesture === 'click' ? fireEvent.click(node) : fireEvent.keyDown(node, { key: gesture })
    expect(screen.getByText('Table selected · pick a root collection to bind its rows.')).toBeInTheDocument()
    expect(screen.queryByText('TABLE ONLY')).not.toBeInTheDocument()
    activate(row('items[]'))
    expect(onConnect.mock.calls).toEqual([[['items']]])
    fireEvent.click(row('report'))
    const transaction = row('transactions[]')
    expect(transaction).toHaveAttribute('aria-expanded', 'false')
    activate(transaction)
    expect(transaction).toHaveAttribute('aria-expanded', 'true')
    expect(onConnect.mock.calls).toEqual([[['items']], [['report', 'transactions']]])
    activate(transaction)
    expect(transaction).toHaveAttribute('aria-expanded', 'false')
    expect(onConnect).toHaveBeenCalledTimes(3)
  })

  it('keeps arrow navigation browsing-only and withholds scalars and row arrays', () => {
    const onConnect = vi.fn()
    render(panel(collections(), { onConnect }))
    fireEvent.click(row('value'))
    fireEvent.click(row('report'))
    const transaction = row('transactions[]')
    fireEvent.keyDown(transaction, { key: 'ArrowRight' })
    expect(transaction).toHaveAttribute('aria-expanded', 'true')
    fireEvent.keyDown(row('item 1'), { key: 'ArrowRight' })
    fireEvent.click(row('ref'))
    const nested = row('nested[]')
    expect(nested).toHaveTextContent('Inside a collection · a table requires a root collection.')
    expect(nested.querySelector('.binding-dot')).toBeNull()
    fireEvent.click(nested)
    expect(nested).toHaveAttribute('aria-expanded', 'true')
    fireEvent.keyDown(transaction, { key: 'ArrowLeft' })
    expect(transaction).toHaveAttribute('aria-expanded', 'false')
    expect(onConnect).not.toHaveBeenCalled()
  })

  it.each([
    ['runtime array', '{"params":[{"nested":[]}]}', 'params[]', 'Runtime parameter'],
    ['runtime descendant', '{"params":{"nested":[]}}', 'nested[]', 'Runtime parameter'],
    ['root array', '[{"nested":[]}]', '$[]', 'no complete root key path'],
    ['truncated ancestor', JSON.stringify({ ['x'.repeat(121)]: { nested: [] } }), 'nested[]', 'no complete root key path'],
  ])('explains unavailable %s binding while keeping browsing possible', (_name, json, label, reason) => {
    const onConnect = vi.fn()
    render(panel(acceptSampleData('scope.json', new TextEncoder().encode(json).buffer), { onConnect }))
    if (!screen.getAllByRole('treeitem').some((item) => within(item).queryByText(label))) fireEvent.click(screen.getAllByRole('treeitem')[1]!)
    const target = row(label)
    expect(target).toHaveTextContent(reason)
    expect(target.querySelector('.binding-dot')).toBeNull()
    fireEvent.click(target)
    fireEvent.keyDown(target, { key: 'Enter' })
    expect(onConnect).not.toHaveBeenCalled()
  })

  it.each([
    { selectedComponentType: 'text' as const }, { selectedComponentType: 'image' as const },
    { selectedComponentType: 'line' as const }, { selectedComponentType: 'rect' as const },
    { selectedComponentType: undefined }, { selectedComponentId: undefined },
    { bindingBusy: true }, { onConnect: undefined },
  ])('never dispatches a collection without an available whole-table target: %j', (props) => {
    const onConnect = vi.fn()
    render(panel(collections(), { onConnect, ...props }))
    fireEvent.click(row('items[]'))
    fireEvent.click(row('report'))
    fireEvent.keyDown(row('transactions[]'), { key: 'Enter' })
    expect(onConnect).not.toHaveBeenCalled()
  })

  it.each(['a.b', '', 'สวัสดี', 'line\nbreak'])('keeps decoded key %j intact for Go and scopes the located refusal to that pick', (key) => {
    const sample = acceptSampleData('keys.json', new TextEncoder().encode(JSON.stringify({ [key]: [], other: [] })).buffer)
    const onConnect = vi.fn()
    const bindingError = { sample, componentID: 'e1', segments: [key], message: 'e1: collection segments must be identifiers' }
    const view = render(panel(sample, { onConnect, bindingError }))
    fireEvent.click(row(`${key}[]`))
    expect(onConnect).toHaveBeenCalledWith([key])
    expect(screen.getByRole('alert').closest('.data-panel')).not.toBeNull()
    view.rerender(panel(sample, { onConnect, bindingError, selectedComponentId: 'e2' }))
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    view.rerender(panel(sample, { onConnect, bindingError }))
    expect(screen.getByRole('alert')).toHaveTextContent('e1: collection segments')
    fireEvent.click(row('other[]'))
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})

describe('docked sample data panel', () => {
  it('keeps authoring available when empty, loads a tree, keeps accepted bytes authoritative, and preserves a prior sample on cancel', async () => {
    const openSample = vi.fn<SampleFileAccess['openSample']>().mockResolvedValueOnce({ name: 'sample.json', bytes: sampleBytes }).mockRejectedValueOnce(new FileAccessCancelled()).mockResolvedValueOnce({ name: 'replacement.json', bytes: replacementBytes })
    const request = vi.fn(async (...args: [string, unknown?, AbortSignal?]) => args[0] === 'identity' ? { snapshot, preview: { revision: 1, identity: 'b'.repeat(64) } } : args[0] === 'serialize' ? { snapshot, bytes: new Uint8Array([1]).buffer } : args[0] === 'render' ? { snapshot, bytes: new Uint8Array([9]).buffer, preview: { revision: 1, identity: 'b'.repeat(64), pdfSha256: PDF_FIXTURE_DIGEST, elapsedMs: RENDER_ELAPSED_MS, version: RENDER_ENGINE_VERSION, diagnostics: [] } } : { snapshot })
    render(<App engine={{ request } as unknown as EngineClient} initialSnapshot={snapshot} sampleFileAccess={{ openSample }} />)
    openDataTab()
    expect(screen.getByLabelText('Data panel')).toBeInTheDocument()
    expect(screen.getByText('No sample data loaded.')).toBeInTheDocument()
    expect(screen.getByText('Sample data is never written into the template.')).toBeInTheDocument()
    expect(screen.getByLabelText('Canvas region')).toBeInTheDocument()
    const load = screen.getByRole('button', { name: 'Load sample JSON' }); load.focus(); expect(load).toHaveFocus()
    fireEvent.click(load)
    await waitFor(() => expect(screen.getByText('sample.json')).toBeInTheDocument())
    expect(screen.getByRole('tree', { name: 'Sample data paths' })).toHaveTextContent('items[]')
    fireEvent.click(screen.getByRole('button', { name: 'PREVIEW' }))
    await waitFor(() => expect(request.mock.calls.some(([operation]) => operation === 'identity')).toBe(true))
    const data = request.mock.calls.find(([operation]) => operation === 'identity')![1] as unknown as { data: ArrayBuffer }
    expect(new Uint8Array(data.data)).toEqual(new Uint8Array(sampleBytes))
    // STORY 13.5 — ONE MODE CONTROL, AND ITS NAME DOES NOT MOVE. This used to
    // match by pattern because the preview heading's button carried two names —
    // `Cancel and return to Design` while a render was in flight, `Return to
    // Design` once a PDF was installed — which made the wording on screen at
    // this line a timing property of the render pipeline. That button is gone.
    // The document bar's DESIGN switch calls the same `returnToDesign` and is
    // named the same in both states, so the pattern is no longer needed.
    fireEvent.click(screen.getByRole('button', { name: 'DESIGN' }))
    fireEvent.click(screen.getByRole('button', { name: 'Replace sample JSON' }))
    await waitFor(() => expect(screen.getByText('sample.json')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Replace sample JSON' }))
    await waitFor(() => expect(screen.getByText('replacement.json')).toBeInTheDocument())
  })

  it('provides one roving tree-item route through branches and scalar leaves', async () => {
    const sample = acceptSampleData('keys.json', new TextEncoder().encode('{"customer":{"name":"Ada","none":null},"items":[]}').buffer)
    render(<DataPanel sample={sample} busy={false} available onLoad={() => undefined} />)
    const root = screen.getAllByRole('treeitem').find((item) => item.getAttribute('aria-level') === '1')!
    root.focus(); expect(root).toHaveFocus()
    fireEvent.keyDown(root, { key: 'ArrowDown' })
    const customer = screen.getAllByRole('treeitem').find((item) => item.getAttribute('aria-level') === '2' && item.textContent?.startsWith('customer'))!
    await waitFor(() => expect(customer).toHaveFocus())
    fireEvent.keyDown(customer, { key: 'ArrowRight' })
    fireEvent.keyDown(customer, { key: 'ArrowDown' })
    const name = screen.getAllByRole('treeitem').find((item) => item.getAttribute('aria-level') === '3' && item.textContent?.startsWith('name'))!
    await waitFor(() => expect(name).toHaveFocus())
    fireEvent.keyDown(name, { key: 'End' })
    const items = screen.getAllByRole('treeitem').find((item) => item.getAttribute('aria-level') === '2' && item.textContent?.startsWith('items'))!
    await waitFor(() => expect(items).toHaveFocus())
    expect(customer).toHaveAttribute('aria-expanded', 'true')
  })

  it('restores the root tab stop when replacing a sample after nested navigation', async () => {
    const first = acceptSampleData('first.json', new TextEncoder().encode('{"customer":{"name":"Ada"}}').buffer)
    const replacement = acceptSampleData('replacement.json', replacementBytes)
    const { rerender } = render(<DataPanel sample={first} busy={false} available onLoad={() => undefined} />)
    const customer = screen.getAllByRole('treeitem').find((item) => item.textContent?.startsWith('customer'))!
    customer.focus(); fireEvent.keyDown(customer, { key: 'ArrowRight' })
    const name = screen.getAllByRole('treeitem').find((item) => item.textContent?.startsWith('name'))!
    fireEvent.keyDown(name, { key: 'ArrowDown' })
    rerender(<DataPanel sample={replacement} busy={false} available onLoad={() => undefined} />)
    await waitFor(() => {
      const items = screen.getAllByRole('treeitem')
      expect(items.filter((item) => item.tabIndex === 0)).toHaveLength(1)
      expect(items[0]).toHaveAttribute('tabindex', '0')
    })
    const root = screen.getAllByRole('treeitem')[0]!
    root.focus(); fireEvent.keyDown(root, { key: 'ArrowDown' })
    await waitFor(() => expect(screen.getAllByRole('treeitem').find((item) => item.textContent?.startsWith('report'))).toHaveFocus())
  })

  it('shows a binding rejection only for its original sample, component, and picked path', () => {
    const sample = acceptSampleData('keys.json', new TextEncoder().encode('{"customer":{"name":"Ada","email":"a@example.test"}}').buffer)
    const error = { sample, componentID: 'e1', segments: ['customer', 'name'], message: 'e1: binding rejected' }
    // A REFUSAL PRESUPPOSES A DISPATCH. The picked row is now set only when the
    // command actually goes out, so this fixture supplies the text kind and an
    // `onConnect` — without them the panel withholds the pick and there is no
    // path for the engine to have refused in the first place.
    render(<DataPanel sample={sample} busy={false} available selectedComponentId="e1" selectedComponentType="text" bindingError={error} onLoad={() => undefined} onConnect={() => undefined} />)
    const customer = screen.getAllByRole('treeitem').find((item) => item.textContent?.startsWith('customer'))!
    fireEvent.click(customer)
    const name = screen.getAllByRole('treeitem').find((item) => item.textContent?.startsWith('name'))!
    fireEvent.click(name)
    expect(screen.getByRole('alert')).toHaveTextContent('binding rejected')
    const email = screen.getAllByRole('treeitem').find((item) => item.textContent?.startsWith('email'))!
    fireEvent.click(email)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('selects a root scalar by keyboard and sends one opaque binding command, then paints distinct binding state', async () => {
    const boundCanvas = { ...textCanvas, components: [{ ...textCanvas.components[0]!, value: '{{customer.name}}', binding: 'customer.name' }] }
    const request = vi.fn(async (operation: string) => operation === 'command'
      ? { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 4, canUndo: true, canvas: boundCanvas } }
      : { snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: textCanvas } })
    const sample = acceptSampleData('keys.json', new TextEncoder().encode('{"customer":{"name":"Ada"},"items":[]}').buffer)
    render(<App engine={{ request } as unknown as EngineClient} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: textCanvas }} initialSampleData={sample} />)
    openDataTab()
    fireEvent.click(screen.getByLabelText('text component e1'))
    const customer = screen.getAllByRole('treeitem').find((item) => item.getAttribute('aria-level') === '2' && item.textContent?.startsWith('customer'))!
    customer.focus()
    fireEvent.keyDown(customer, { key: 'ArrowRight' })
    const name = screen.getAllByRole('treeitem').find((item) => item.getAttribute('aria-level') === '3' && item.textContent?.startsWith('name'))!
    name.focus()
    fireEvent.keyDown(name, { key: 'Enter' })
    // STORY 14.6 — the pick above IS the bind; there is no intermediate control.
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'command')).not.toHaveLength(0))
    const commands = request.mock.calls.filter(([operation]) => operation === 'command') as unknown as Array<[string, ArrayBuffer]>
    expect(commands).toHaveLength(1)
    const [operation, payload] = commands[0]!
    expect(operation).toBe('command')
    expect(new TextDecoder().decode(payload)).toBe('{"kind":"bindComponentScalar","version":1,"id":"e1","segments":["customer","name"]}')
    await waitFor(() => expect(screen.getByText('Bound to').parentElement).toHaveTextContent('Bound to customer.name'))
    expect(screen.getByLabelText('text component e1; bound to customer.name')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Undo' })).toBeEnabled()
  })

  it('does not install a late binding response after Start blank replaces the document', async () => {
    let resolveBinding!: (value: { snapshot: { documentState: 'loaded'; revision: number; byteLength: number; canvas: typeof textCanvas } }) => void
    const boundCanvas = { ...textCanvas, components: [{ ...textCanvas.components[0]!, value: '{{customer.name}}', binding: 'customer.name' }] }
    const request = vi.fn((operation: string) => {
      if (operation === 'command') return new Promise<{ snapshot: { documentState: 'loaded'; revision: number; byteLength: number; canvas: typeof textCanvas } }>((resolve) => { resolveBinding = resolve })
      if (operation === 'load') return Promise.resolve({ snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 1, canvas: textCanvas } })
      return Promise.resolve({ snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: textCanvas } })
    })
    const sample = acceptSampleData('keys.json', new TextEncoder().encode('{"customer":{"name":"Ada"}}').buffer)
    render(<App engine={{ request } as unknown as EngineClient} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: textCanvas }} initialSampleData={sample} blankBytes={new Uint8Array([7]).buffer} />)
    openDataTab()
    fireEvent.click(screen.getByLabelText('text component e1'))
    const customer = screen.getAllByRole('treeitem').find((item) => item.getAttribute('aria-level') === '2' && item.textContent?.startsWith('customer'))!
    fireEvent.click(customer)
    fireEvent.click(screen.getAllByRole('treeitem').find((item) => item.getAttribute('aria-level') === '3' && item.textContent?.startsWith('name'))!)
    // STORY 14.6 — the pick above IS the bind; there is no intermediate control.
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'command')).toHaveLength(1))
    startBlankFromNew()
    await waitFor(() => expect(screen.getByText('Started an unnamed local template')).toBeInTheDocument())
    resolveBinding({ snapshot: { documentState: 'loaded', revision: 2, byteLength: 4, canvas: boundCanvas } })
    await Promise.resolve(); await Promise.resolve()
    expect(screen.queryByText('Bound to')).not.toBeInTheDocument()
    expect(screen.queryByText('customer.name')).not.toBeInTheDocument()
  })

  it('installs a committed binding after reselection while keeping the newer selection', async () => {
    const selectedCanvas = { ...textCanvas, components: [...textCanvas.components, { id: 'e2', type: 'text' as const, band: 'content' as const, x: 80_000, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Other' }] }
    const boundCanvas = { ...selectedCanvas, components: [{ ...selectedCanvas.components[0]!, value: '{{customer.name}}', binding: 'customer.name' }, selectedCanvas.components[1]!] }
    let resolveBinding!: (value: { snapshot: { documentState: 'loaded'; revision: number; byteLength: number; canvas: typeof selectedCanvas } }) => void
    const request = vi.fn((operation: string) => operation === 'command' ? new Promise<{ snapshot: { documentState: 'loaded'; revision: number; byteLength: number; canvas: typeof selectedCanvas } }>((resolve) => { resolveBinding = resolve }) : Promise.resolve({ snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: selectedCanvas } }))
    const sample = acceptSampleData('keys.json', new TextEncoder().encode('{"customer":{"name":"Ada"}}').buffer)
    render(<App engine={{ request } as unknown as EngineClient} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: selectedCanvas }} initialSampleData={sample} />)
    openDataTab()
    fireEvent.click(screen.getByLabelText('text component e1'))
    fireEvent.click(screen.getAllByRole('treeitem').find((item) => item.textContent?.startsWith('customer'))!)
    fireEvent.click(screen.getAllByRole('treeitem').find((item) => item.textContent?.startsWith('name'))!)
    // STORY 14.6 — the pick above IS the bind; there is no intermediate control.
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'command')).toHaveLength(1))
    fireEvent.click(screen.getByLabelText('text component e2'))
    resolveBinding({ snapshot: { documentState: 'loaded', revision: 2, byteLength: 4, canvas: boundCanvas } })
    await waitFor(() => expect(screen.getByLabelText('text component e1; bound to customer.name')).toBeInTheDocument())
    expect(screen.getByLabelText('text component e2')).toHaveClass('canvas-component-selected')
  })

  it('revokes a pending picker when an equal-revision Start blank replaces the document', async () => {
    let release!: (value: { name: string; bytes: ArrayBuffer }) => void
    const openSample = vi.fn(() => new Promise<{ name: string; bytes: ArrayBuffer }>((resolve) => { release = resolve }))
    const request = vi.fn(async (operation: string) => operation === 'serialize' ? { snapshot, bytes: new Uint8Array([1]).buffer } : { snapshot })
    render(<App engine={{ request } as unknown as EngineClient} initialSnapshot={snapshot} blankBytes={new Uint8Array([7]).buffer} sampleFileAccess={{ openSample }} />)
    openDataTab()
    fireEvent.click(screen.getByRole('button', { name: 'Load sample JSON' }))
    startBlankFromNew()
    await waitFor(() => expect(screen.getByText('Started an unnamed local template')).toBeInTheDocument())
    release({ name: 'late.json', bytes: sampleBytes })
    await Promise.resolve(); await Promise.resolve()
    expect(screen.queryByText('late.json')).not.toBeInTheDocument()
    expect(screen.getByText('No sample data loaded.')).toBeInTheDocument()
    expect(request.mock.calls.filter(([operation]) => operation === 'identity')).toHaveLength(0)
  })

  // STORY 5 (spec-startup-templates), CAP-7 — THE CONTROL ITSELF.
  //
  // Where App.test.tsx proves the BYTES that leave, these prove the button: when
  // it exists at all, what it is called, and the two states it is disabled in.
  // It is withheld with no sample rather than disabled, because with nothing
  // loaded there is no file the author could mean and so no reason to state
  // beside a dead control (DESIGN.md:592 — anything disabled states its reason).
  describe('Save sample data', () => {
    const sampleOf = (name = 'sample.json') => acceptSampleData(name, sampleBytes)
    const panel = (props: Partial<React.ComponentProps<typeof DataPanel>> = {}) => <DataPanel sample={sampleOf()} busy={false} available onLoad={() => undefined} {...props} />

    it('is absent until a sample is loaded, and sits immediately after the load control', () => {
      const onSave = vi.fn()
      const view = render(panel({ sample: undefined, onSave }))
      expect(screen.getByRole('button', { name: 'Load sample JSON' })).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Save sample data' })).not.toBeInTheDocument()
      view.rerender(panel({ onSave }))
      expect(screen.getAllByRole('button').slice(0, 2).map((button) => button.textContent)).toEqual(['Replace sample JSON', 'Save sample data'])
      fireEvent.click(screen.getByRole('button', { name: 'Save sample data' }))
      expect(onSave).toHaveBeenCalledOnce()
    })

    it('is disabled, and answers no click, while a file operation is in flight or no local save tier exists', () => {
      const onSave = vi.fn()
      const view = render(panel({ onSave, saveDisabled: true }))
      const control = screen.getByRole('button', { name: 'Save sample data' })
      expect(control).toBeDisabled()
      fireEvent.click(control)
      expect(onSave).not.toHaveBeenCalled()
      // The sample PICKER being unavailable is a different fact from the SAVE
      // tier being unavailable, and the two controls answer to their own.
      view.rerender(panel({ onSave, available: false, busy: true }))
      expect(screen.getByRole('button', { name: 'Replace sample JSON' })).toBeDisabled()
      expect(screen.getByRole('button', { name: 'Save sample data' })).toBeEnabled()
    })

    it('is offered for any loaded sample, whatever opened it', async () => {
      // A sample the AUTHOR opened with Load sample JSON, not an example's, and
      // the control is the same one.
      const openSample = vi.fn<SampleFileAccess['openSample']>().mockResolvedValue({ name: 'mine.json', bytes: sampleBytes })
      const acquireSaveTarget = vi.fn(async (request: SaveTargetRequest): Promise<AcquiredSaveTarget> => ({ name: request.suggestedName, format: request.format }))
      const writeSave = vi.fn(async (_target: AcquiredSaveTarget, _request: SaveRequest): Promise<SavedLocalFile> => ({ name: 'mine.json' }))
      const request = vi.fn(async (operation: string) => operation === 'serialize' ? { snapshot, bytes: new Uint8Array([1]).buffer } : { snapshot })
      render(<App engine={{ request } as unknown as EngineClient} initialSnapshot={snapshot} sampleFileAccess={{ openSample }} fileAccess={{ open: vi.fn(), acquireSaveTarget, writeSave }} />)
      openDataTab()
      expect(screen.queryByRole('button', { name: 'Save sample data' })).not.toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: 'Load sample JSON' }))
      await waitFor(() => expect(screen.getByText('mine.json')).toBeInTheDocument())
      fireEvent.click(screen.getByRole('button', { name: 'Save sample data' }))
      await waitFor(() => expect(writeSave).toHaveBeenCalledOnce())
      expect(acquireSaveTarget.mock.calls[0]![0]).toEqual({ suggestedName: 'mine.json', saveAs: true, format: jsonSampleFileFormat })
      expect(new Uint8Array(writeSave.mock.calls[0]![1].bytes)).toEqual(new Uint8Array(sampleBytes))
      expect(screen.getByText('Downloaded sample data mine.json')).toBeInTheDocument()
    })
  })
})


// STORY 14.6 — THE SELECTION CONTEXT BAR, WHICH REPLACES THE PRE-FLIGHT LADDER.
//
// WHAT STORY 14.4 BUILT AND WHY IT IS RE-WORDED HERE. 14.4 added a fifth arm to
// an `unavailable` ladder so a Line's refusal — *"only text components can
// receive a scalar binding"* — was stated before the command round-tripped
// rather than after. Its own comment addressed this story by name: the arm's
// WORDING is presentation and 14.6 replaces it; what 14.6 inherits and must not
// re-derive is THE RULE, `SCALAR_BINDING_COMPONENT_TYPES` and its mirror in
// `engine-bounds-mirror.test.ts`.
//
// WHAT IS NEW. The reason now appears in a context bar ABOVE the tree, so it is
// read BEFORE any pick rather than after one (DW-352 — 14.4 still told every
// author to "choose an offered root scalar path" first). And a Table is no
// longer told the binding is unavailable: a table legally binds a collection.
// Whole-table collection picks now happen here in DATA as well.
describe('the data panel states what a pick would bind before the pick', () => {
  const pickCustomerName = () => {
    fireEvent.click(screen.getAllByRole('treeitem').find((item) => item.textContent?.startsWith('customer'))!)
    fireEvent.click(screen.getAllByRole('treeitem').find((item) => item.textContent?.startsWith('name'))!)
  }
  const sample = () => acceptSampleData('keys.json', new TextEncoder().encode('{"customer":{"name":"Ada"}}').buffer)
  // ⚠ A SINGLE MICROTASK IS NOT A FLUSH — the same standard `binding-
  // vocabulary.test.tsx` argues for. Read from the constant so it cannot drift
  // under the debounce it exists to outlast.
  const settle = async () => {
    await new Promise((resolve) => setTimeout(resolve, PROSE_COMMIT_DEBOUNCE_MS + 20))
    for (let turn = 0; turn < 4; turn++) await Promise.resolve()
  }
  const openApp = (canvasFixture: typeof textCanvas | typeof lineCanvas | typeof tableCanvas | typeof mixedCanvas | typeof rectCanvas | typeof imageCanvas) => {
    const request = vi.fn(async (operation: string) => operation === 'command'
      ? { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 4, canvas: canvasFixture } }
      : { snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: canvasFixture } })
    render(<App engine={{ request } as unknown as EngineClient} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: canvasFixture }} initialSampleData={sample()} />)
    openDataTab()
    return request
  }
  const commandsFrom = (request: ReturnType<typeof openApp>) => request.mock.calls.filter(([operation]) => operation === 'command')

  // ALL FOUR REFUSED KINDS, not the two that happened to read well. P5 of 14.4
  // found an article bug surviving because `image` was the one arm no case
  // exercised. The kind whose wording is awkward is the kind most likely to go
  // untested, so every kind keeps a row here.
  it.each([
    ['line', lineCanvas, 'Line selected · only text, barcode and QR code components can receive a scalar binding.'],
    ['rect', rectCanvas, 'Rectangle selected · only text, barcode and QR code components can receive a scalar binding.'],
    ['image', imageCanvas, 'Image selected · only text, barcode and QR code components can receive a scalar binding.'],
    // A table invites a collection pick while continuing to refuse scalars.
    ['table', tableCanvas, 'Table selected · pick a root collection to bind its rows.'],
  ])('states a selected %s\'s binding context and dispatches nothing for a scalar gesture', async (kind, fixture, message) => {
    const request = openApp(fixture)
    fireEvent.click(screen.getByLabelText(new RegExp(`^${kind} component e1`)))
    // ⚠ DW-352, AND THE ORDER IS THE POINT. This assertion runs BEFORE any tree
    // row is touched. Under 14.4 the panel said "Choose an offered root scalar
    // path." here and only spoke about the kind after the author had picked.
    const bar = screen.getByText(message)
    expect(bar).toHaveAttribute('role', 'status')
    expect(commandsFrom(request)).toHaveLength(0)
    // AND THE GESTURE ITSELF DISPATCHES NOTHING. With the connect control gone,
    // this is no longer a consequence of a disabled button: the click reaches a
    // live treeitem and the panel withholds the command on its own.
    pickCustomerName()
    await settle()
    expect(commandsFrom(request)).toHaveLength(0)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  // THE NON-VACUOUS HALF. This fixture differs from those above ONLY in the
  // selected component's kind, and the bar must invite the pick rather than
  // refuse it — an over-broad gate refusing everything would pass every
  // assertion above.
  it('invites the pick for a text component and sends the same bytes it always sent', async () => {
    const request = openApp(textCanvas)
    fireEvent.click(screen.getByLabelText(/^text component e1/))
    expect(screen.getByText('Text selected · binding to string')).toBeInTheDocument()
    expect(screen.queryByText(/only text components can receive a scalar binding/)).not.toBeInTheDocument()
    pickCustomerName()
    await waitFor(() => expect(commandsFrom(request)).toHaveLength(1))
    const [, payload] = commandsFrom(request)[0] as unknown as [string, ArrayBuffer]
    expect(new TextDecoder().decode(payload)).toBe('{"kind":"bindComponentScalar","version":1,"id":"e1","segments":["customer","name"]}')
  })

  // A MULTI-SELECTION HAS NO ONE KIND TO SPEAK FOR. `selectedComponentId` is
  // undefined for it, so the bar says the true thing rather than picking one of
  // the two kinds to complain about.
  it('keeps "select one component" for a multi-selection carrying a non-text kind', () => {
    openApp(mixedCanvas)
    fireEvent.click(screen.getByLabelText(/^text component e1/))
    fireEvent.click(screen.getByLabelText(/^rect component e2/), { shiftKey: true })
    expect(screen.getByText('No single component selected · select one component, then pick a path.')).toBeInTheDocument()
    expect(screen.queryByText(/only text components can receive a scalar binding/)).not.toBeInTheDocument()
  })

  // ARM ORDER, PROVED AT THE COMPONENT RATHER THAN INFERRED FROM THE APP. With
  // NOTHING selected the kind is unknown, and "select one component" is still
  // the true first thing to say. Passing both props here is the only way to
  // witness the ordering, because App never supplies a type without an id.
  it('lets the no-selection arm win even when a kind is supplied', () => {
    render(<DataPanel sample={sample()} busy={false} available selectedComponentType="line" onLoad={() => undefined} />)
    expect(screen.getByText('No single component selected · select one component, then pick a path.')).toBeInTheDocument()
    expect(screen.queryByText(/only text components can receive a scalar binding/)).not.toBeInTheDocument()
  })

  // P1 OF 14.4 — THE FAIL-OPEN, AND THE STATE THAT REACHES IT. `bindableKind`
  // is inherited verbatim: it fails CLOSED on an unknown kind. The id comes
  // from the selection unconditionally while the kind comes from the
  // projection, so an absent canvas or an id no longer in the projection yields
  // id-present / kind-absent, and a panel that does not know the kind cannot
  // know the engine will accept it.
  it('refuses to invite a pick when the selected component has no known kind, and dispatches nothing', () => {
    const onConnect = vi.fn()
    render(<DataPanel sample={sample()} busy={false} available selectedComponentId="e1" onLoad={() => undefined} onConnect={onConnect} />)
    expect(screen.getByText('The selected component is not in the current projection · no path can be bound to it.')).toBeInTheDocument()
    pickCustomerName()
    expect(onConnect).not.toHaveBeenCalled()
  })

  // NON-VACUITY for the row above: the identical render differing ONLY in a
  // known text kind binds on the same gesture.
  it('binds on the same gesture once the kind is known to be text', () => {
    const onConnect = vi.fn()
    render(<DataPanel sample={sample()} busy={false} available selectedComponentId="e1" selectedComponentType="text" onLoad={() => undefined} onConnect={onConnect} />)
    pickCustomerName()
    expect(onConnect).toHaveBeenCalledExactlyOnceWith(['customer', 'name'])
    expect(screen.queryByText(/only text components can receive a scalar binding/)).not.toBeInTheDocument()
  })
})

// STORY 14.6 — THE ROW SHOWS THE VALUE, AND THE PANEL STOPS OFFERING WHAT THE
// ENGINE REFUSES.
const encode = (value: string) => new TextEncoder().encode(value).buffer
const THAI_NAME = 'สมชาย วงศ์ประเสริฐ'
const rowFor = (label: string) => screen.getAllByRole('treeitem').find((item) => item.textContent?.startsWith(label))!
const panelRoot = () => screen.getByLabelText('Data panel')

describe('the data tab is the binding panel the design drew', () => {
  const scalarSample = () => acceptSampleData('sample-statement.json', encode(`{"customer":{"name":"${THAI_NAME}","active":true}}`))
  const collectionSample = () => acceptSampleData('c.json', encode('{"transactions":[{"date":"01 Jul"},{"date":"02 Jul"}]}'))
  const emptyCollectionSample = () => acceptSampleData('e.json', encode('{"items":[]}'))
  const paramsSample = () => acceptSampleData('p.json', encode('{"params":{"reportDate":"2026-08-23"}}'))
  const showTree = (sample: ReturnType<typeof scalarSample>, onConnect = vi.fn()) => {
    render(<DataPanel sample={sample} busy={false} available selectedComponentId="e1" selectedComponentType="text" onLoad={() => undefined} onConnect={onConnect} />)
    return onConnect
  }

  // AC1. The row that used to read `string · "สมชาย วงศ์ประเสริฐ" · root scalar
  // candidate` now reads the author's own value.
  it('shows a scalar leaf its value instead of a type name, a count and a preview run together', () => {
    showTree(scalarSample())
    fireEvent.click(rowFor('customer'))
    const name = rowFor('name')
    expect(name).toHaveTextContent(THAI_NAME)
    // The concatenation is gone, not merely reordered — including the quoting
    // the projection applies to a string leaf, which is right for a machine and
    // wrong for a row that exists to show an author their own data.
    expect(name.textContent).not.toContain('root scalar candidate')
    expect(name.textContent).not.toContain('string')
    expect(name.textContent).not.toContain('"')
    expect(within(name).getByText(THAI_NAME)).toHaveClass('tree-value')
    // A boolean keeps its literal, which IS its value.
    expect(rowFor('active')).toHaveTextContent('true')
  })

  // AC2. `{ }` is new; `[]` already rode on the collection's own label.
  it('marks an object with braces and a collection with brackets', () => {
    showTree(scalarSample())
    expect(rowFor('customer')).toHaveTextContent('{ }')
    render(<DataPanel sample={collectionSample()} busy={false} available onLoad={() => undefined} />)
    expect(screen.getAllByRole('treeitem').some((item) => item.textContent?.startsWith('transactions[]'))).toBe(true)
  })

  // AC3. DESIGN.md:331-334 names a byte count among the values set in mono, so
  // the size is a `code`, not the mockup's sans span. The mockup loses.
  // AC3 NAMES THE RENDERING ITSELF — `sample-statement.json \u00b7 18 KB` — so the
  // format is the AC's, not this test's invention. DESIGN.md:331-334 supplies
  // only the FACE (a byte count "is set in mono"), which the <code> element
  // carries; it says nothing about rounding. An earlier spelling printed
  // `18,432 bytes` and cited DESIGN.md to overrule both the AC and the mockup,
  // which agreed with each other.
  it('names the loaded file and its size in the AC\'s own units, in mono', () => {
    const bytes = encode(`{"a":"${'x'.repeat(18424)}"}`)
    expect(bytes.byteLength).toBe(18432)
    render(<DataPanel sample={acceptSampleData('sample-statement.json', bytes)} busy={false} available onLoad={() => undefined} />)
    expect(screen.getByText('sample-statement.json')).toBeInTheDocument()
    expect(screen.getByText('18 KB').tagName).toBe('CODE')
    // AND THE ROUNDING IS REAL, not a coincidence of one input: the exact byte
    // count must not be what reaches the row.
    expect(screen.queryByText(/18,?432/)).not.toBeInTheDocument()
    expect(screen.queryByText(/bytes/)).not.toBeInTheDocument()
  })

  // The unit is chosen AFTER rounding, which is why a file just under a
  // mebibyte cannot print `1024 KB`. Reused from the evidence rail rather than
  // re-derived here, so this row also pins that the reuse is live.
  it('rounds up a unit rather than printing 1024 KB', () => {
    const bytes = encode(`{"a":"${'x'.repeat(1048570 - 8)}"}`)
    expect(bytes.byteLength).toBe(1048570)
    render(<DataPanel sample={acceptSampleData('big.json', bytes)} busy={false} available onLoad={() => undefined} />)
    expect(screen.getByText('1 MB')).toBeInTheDocument()
    expect(screen.queryByText(/1024 KB/)).not.toBeInTheDocument()
  })

  // AC5. The badge, the reason, and the two refusals underneath it.
  it('badges a populated collection TABLE ONLY, refuses its own row, and dims its children with a reason', () => {
    const onConnect = showTree(collectionSample())
    const transactions = rowFor('transactions[]')
    expect(transactions).toHaveTextContent('TABLE ONLY')
    expect(transactions).toHaveTextContent('Collection · 2 items. Text cannot bind a collection.')
    // ⚠ NOT `aria-disabled`: this row EXPANDS, and expanding is the only route
    // to its children. Announcing an operable expander as disabled is a lie to
    // assistive technology and hides the subtree behind it. "Cannot be picked"
    // is carried by the absent dot, the badge and the stated reason; only a row
    // that can be neither picked nor expanded is disabled.
    expect(transactions).not.toHaveAttribute('aria-disabled')
    expect(transactions).toHaveAttribute('aria-expanded')
    // NOT PICKABLE — the click expands it, and no bind is sent. The dot that
    // DESIGN.md:549 reserves for a bindable leaf is absent.
    expect(transactions.querySelector('.binding-dot')).toBeNull()
    fireEvent.click(transactions)
    expect(onConnect).not.toHaveBeenCalled()
    const item = rowFor('item 1')
    expect(item.closest('li')).toHaveClass('data-tree-dim')
    expect(item).toHaveTextContent('Inside a collection · a table row binds these, not a text component.')
    fireEvent.click(item)
    expect(onConnect).not.toHaveBeenCalled()
    const date = rowFor('date')
    expect(date.closest('li')).toHaveClass('data-tree-dim')
    expect(date).toHaveTextContent('Inside a collection · a table row binds these, not a text component.')
    expect(date.querySelector('.binding-dot')).toBeNull()
    fireEvent.click(date)
    fireEvent.keyDown(date, { key: 'Enter' })
    expect(onConnect).not.toHaveBeenCalled()
  })

  // Empty collections remain unavailable to text, while whole-table mode
  // offers the same retained collection segments.
  it('sends no scalar bind command for an empty collection, by click or by Enter', () => {
    const onConnect = showTree(emptyCollectionSample())
    const items = rowFor('items[]')
    expect(items).toHaveTextContent('Collection · 0 items. Text cannot bind a collection.')
    expect(items).toHaveTextContent('TABLE ONLY')
    fireEvent.click(items)
    fireEvent.keyDown(items, { key: 'Enter' })
    fireEvent.keyDown(items, { key: ' ' })
    expect(onConnect).not.toHaveBeenCalled()
    // Retain collection segments for whole-table picks and table discovery.
    // The scalar-mode refusal depends on node kind, not absent segments.
    expect(emptyCollectionSample().tree.children[0]!.segments).toEqual(['items'])
    expect(collectionSample().tree.children[0]!.segments).toEqual(['transactions'])
  })

  // FENCE — "the collection row is not pickable in scalar mode", with the marker ADDED BACK at
  // the source the story fixed. Reverting `sample-data.ts` alone must not
  // restore the offer, which is why `rowFor`'s rule names `collection`
  // explicitly rather than trusting the absence of `segments`.
  it('still refuses a collection row that carries a segments marker', () => {
    const parsed = emptyCollectionSample()
    const withSegments = { ...parsed, tree: { ...parsed.tree, children: parsed.tree.children.map((child) => ({ ...child, segments: ['items'] })) } }
    const onConnect = showTree(withSegments)
    const items = rowFor('items[]')
    // NON-VACUITY: the mutation really did land — an ordinary scalar carrying
    // the same marker in the same render IS pickable, so the refusal below is
    // the collection rule's doing and not a fixture that broke every click.
    expect(withSegments.tree.children[0]!.segments).toEqual(['items'])
    fireEvent.click(items)
    fireEvent.keyDown(items, { key: 'Enter' })
    expect(onConnect).not.toHaveBeenCalled()
    expect(items.querySelector('.binding-dot')).toBeNull()
    // THE CONTROL THIS COMMENT PROMISES, ACTUALLY PERFORMED. A scalar carrying
    // the same marker in its own render IS pickable, so the refusal above is
    // the collection rule and not a fixture that broke every click.
    cleanup()
    const scalarConnect = showTree(scalarSample())
    fireEvent.click(rowFor('customer'))
    fireEvent.click(rowFor('name'))
    expect(scalarConnect).toHaveBeenCalledOnce()
  })

  // AC6, FIRST SOURCE (DW-349) — a `params` key in the AUTHOR'S OWN sample JSON.
  // Go refuses it at `component_commands.go:749-751`: *"params is not a root
  // data binding"*.
  it('dims every node in a sample-JSON params namespace, states why, and never binds one', () => {
    const onConnect = showTree(paramsSample())
    const params = rowFor('params')
    expect(params.closest('li')).toHaveClass('data-tree-dim')
    expect(params).toHaveTextContent('Runtime parameter · the engine refuses a params path as a data binding.')
    fireEvent.click(params)
    const reportDate = rowFor('reportDate')
    expect(reportDate.closest('li')).toHaveClass('data-tree-dim')
    expect(reportDate).toHaveTextContent('Runtime parameter · the engine refuses a params path as a data binding.')
    expect(reportDate).toHaveAttribute('aria-disabled', 'true')
    expect(reportDate.querySelector('.binding-dot')).toBeNull()
    fireEvent.click(reportDate)
    fireEvent.keyDown(reportDate, { key: 'Enter' })
    expect(onConnect).not.toHaveBeenCalled()
  })

  // NON-VACUITY for the row above, in its own render so no stale tree can
  // answer for it: the SAME shape outside the `params` namespace IS offered, so
  // the refusal is the namespace's doing and not a fixture that broke clicking.
  it('offers the identical leaf shape under any other root namespace', () => {
    const offered = showTree(acceptSampleData('o.json', encode('{"settings":{"reportDate":"2026-08-23"}}')))
    fireEvent.click(rowFor('settings'))
    fireEvent.click(rowFor('reportDate'))
    expect(offered).toHaveBeenCalledExactlyOnceWith(['settings', 'reportDate'])
  })

  // FENCE — "never pickable", with `segments` ADDED to the params leaf.
  it('still refuses a params leaf that carries a segments marker', () => {
    const parsed = paramsSample()
    const params = parsed.tree.children[0]!
    const mutated = { ...parsed, tree: { ...parsed.tree, children: [{ ...params, segments: ['params'], children: params.children.map((child) => ({ ...child, segments: ['params', 'reportDate'] })) }] } }
    const onConnect = showTree(mutated)
    fireEvent.click(rowFor('params'))
    const reportDate = rowFor('reportDate')
    expect(mutated.tree.children[0]!.children[0]!.segments).toEqual(['params', 'reportDate'])
    fireEvent.click(reportDate)
    fireEvent.keyDown(reportDate, { key: 'Enter' })
    expect(onConnect).not.toHaveBeenCalled()
  })

  // AC6, SECOND SOURCE — the ENGINE's discovered parameters. These live in the
  // TEMPLATE, not in any sample file, so they are visible with no sample loaded
  // at all. Neither source may be satisfied by the other.
  it('shows the engine-discovered params namespace with a RUNTIME badge, values, and no control of any kind', () => {
    render(<DataPanel busy={false} available onLoad={() => undefined} runtimeParameters={{ status: 'ready', names: ['reportDate', 'branchName'], values: { reportDate: '"2026-08-23"' } }} />)
    const section = screen.getByLabelText('Runtime parameters from the template')
    expect(within(section).getByText('params')).toBeInTheDocument()
    expect(within(section).getByText('RUNTIME')).toBeInTheDocument()
    expect(within(section).getByText('2026-08-23')).toBeInTheDocument()
    expect(within(section).getByText('not set')).toBeInTheDocument()
    // NEVER PICKABLE. ⚠ The sweep includes the SECTION ITSELF — a `within(x)`
    // role query cannot see a violation ON `x` — and asks in both spellings,
    // because Testing Library excludes `aria-hidden` subtrees by default.
    expect(within(section).queryAllByRole('button')).toHaveLength(0)
    expect(within(section).queryAllByRole('button', { hidden: true })).toHaveLength(0)
    expect(within(section).queryAllByRole('treeitem', { hidden: true })).toHaveLength(0)
    for (const node of [section, ...Array.from(section.querySelectorAll('*'))]) {
      expect(node.tagName, 'a runtime parameter is display, never a control').not.toBe('BUTTON')
      if (node !== section) expect(node.getAttribute('role')).toBeNull()
    }
  })

  it('says the engine could not provide references rather than guessing an empty list', () => {
    render(<DataPanel busy={false} available onLoad={() => undefined} runtimeParameters={{ status: 'failed', names: [], values: {} }} />)
    expect(screen.getByText('The local engine could not provide the runtime parameters for this template.')).toBeInTheDocument()
    expect(screen.queryByText('The local engine found no runtime parameters in this template.')).not.toBeInTheDocument()
    // AND THE TWO FACTS STAY DIFFERENT: a genuinely empty ready list reads as
    // the template's own answer, not as the engine's silence.
    render(<DataPanel busy={false} available onLoad={() => undefined} runtimeParameters={{ status: 'ready', names: [], values: {} }} />)
    expect(screen.getByText('The local engine found no runtime parameters in this template.')).toBeInTheDocument()
  })

  // AC7. A pick binds immediately, and the picked row takes the bind accent.
  it('binds on the pick itself and marks the picked row with the bind accent', () => {
    const onConnect = showTree(scalarSample())
    fireEvent.click(rowFor('customer'))
    fireEvent.click(rowFor('name'))
    expect(onConnect).toHaveBeenCalledExactlyOnceWith(['customer', 'name'])
    expect(rowFor('name').closest('li')).toHaveClass('data-tree-picked')
  })

  // FENCE — "no connect control". Swept three ways, because the fence must red
  // wherever a button is ADDED: on the panel, inside the tree, and on a row.
  it('offers exactly one control in the whole panel, and it is the sample loader', () => {
    showTree(scalarSample())
    const panel = panelRoot()
    const buttons = Array.from(panel.querySelectorAll('button')).filter((element) => element.getAttribute('role') !== 'treeitem')
    expect(buttons.map((element) => element.textContent)).toEqual(['Replace sample JSON'])
    expect(within(panel).queryAllByRole('button', { hidden: true }).map((element) => element.textContent)).toEqual(['Replace sample JSON'])
    for (const node of [panel, ...Array.from(panel.querySelectorAll('*'))]) {
      if (node.getAttribute('role') === 'treeitem') continue
      expect(node.getAttribute('role'), 'nothing in this panel is a second control').not.toBe('button')
    }
    expect(screen.queryByRole('button', { name: 'Connect selected path' })).not.toBeInTheDocument()
  })

  // AC10, PRESERVATION. A badge is part of its row, never a control of its own,
  // and an unpickable row says so.
  it('announces a badge as part of its row and reports an unpickable row as disabled', () => {
    showTree(collectionSample())
    const transactions = rowFor('transactions[]')
    expect(within(transactions).getByText('TABLE ONLY').tagName).toBe('SPAN')
    // ⚠ THE SWEEP INCLUDES EVERY WRAPPER BETWEEN the badge and the row, so
    // adding a role or a name to `.tree-row` reds this as surely as adding one
    // to `.tree-badge`.
    for (const node of Array.from(transactions.querySelectorAll('*'))) {
      expect(node.getAttribute('role'), `${node.className} must not be announced as its own control`).toBeNull()
      expect(node.getAttribute('aria-label'), `${node.className} must not carry a name of its own`).toBeNull()
    }
    expect(within(transactions).queryAllByRole('button', { hidden: true })).toHaveLength(0)
    // DISABLED MEANS INOPERABLE, NOT MERELY UNPICKABLE. A populated collection
    // expands, so it is not disabled; an EMPTY one can be neither picked nor
    // expanded, so it is.
    expect(transactions).not.toHaveAttribute('aria-disabled')
    cleanup()
    showTree(emptyCollectionSample())
    const items = rowFor('items[]')
    expect(items).toHaveAttribute('aria-disabled', 'true')
    expect(items).toHaveTextContent('TABLE ONLY')
    // NON-VACUITY: an ordinary branch in its own render is NOT reported as
    // disabled, so the attribute above means what it says.
    cleanup()
    showTree(scalarSample())
    expect(rowFor('customer')).not.toHaveAttribute('aria-disabled')
  })

  // AC10, PRESERVATION — the roving tab stop survives every treatment this
  // story adds. Exactly one treeitem is reachable by Tab, before and after
  // arrow navigation.
  it('keeps exactly one roving tab stop through the restyled rows', async () => {
    showTree(collectionSample())
    const stops = () => screen.getAllByRole('treeitem').filter((item) => item.tabIndex === 0)
    expect(stops()).toHaveLength(1)
    const root = screen.getAllByRole('treeitem')[0]!
    root.focus()
    fireEvent.keyDown(root, { key: 'ArrowDown' })
    await waitFor(() => expect(rowFor('transactions[]')).toHaveFocus())
    expect(stops()).toHaveLength(1)
    expect(stops()[0]).toBe(rowFor('transactions[]'))
    fireEvent.keyDown(rowFor('transactions[]'), { key: 'Home' })
    await waitFor(() => expect(screen.getAllByRole('treeitem')[0]!).toHaveFocus())
    expect(stops()).toHaveLength(1)
  })

  // AC9. And the sentence the panel never said anywhere before.
  it('says there is no sample and that sample data is never written into the template', () => {
    render(<DataPanel busy={false} available onLoad={() => undefined} />)
    expect(screen.getByText('No sample data loaded.')).toBeInTheDocument()
    expect(screen.getByText('Sample data is never written into the template.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Load sample JSON' })).toBeInTheDocument()
  })
})

// STORY 14.6 / AC8 — THE STATUS BAR STATES HOW MUCH OF THE DOCUMENT IS BOUND.
//
// "Bound" is `binding` OR `tableBind` (owner ruling). The exclusions read as
// surprising and are correct: `directCanvasBinding` populates `binding` only for
// a whole-value, single, non-reserved path placeholder.
const bandSpreadCanvas = { ...canvas, components: [
  { id: 'h1', type: 'text' as const, band: 'pageHeader' as const, x: 0, y: 0, width: 10, height: 10, resizable: true, value: '{{customer.name}}', binding: 'customer.name' },
  { id: 'c1', type: 'table' as const, band: 'content' as const, x: 0, y: 0, width: 10, height: 10, resizable: false, tableBind: 'transactions[]' },
  // DELIBERATELY UNBOUND, and this is the surprising half: a placeholder inside
  // a longer literal never populates `binding`.
  { id: 'c2', type: 'text' as const, band: 'content' as const, x: 0, y: 20, width: 10, height: 10, resizable: true, value: 'Total: {{amount}}' },
  { id: 'f1', type: 'line' as const, band: 'pageFooter' as const, x: 0, y: 0, width: 10, height: 1, resizable: true, background: '#000000' },
] }

describe('the status bar counts bound elements', () => {
  const boundCount = () => screen.queryByTestId('bound-element-count')
  const show = (canvasFixture: typeof canvas | typeof textCanvas | typeof bandSpreadCanvas) => {
    const request = vi.fn(async () => ({ snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: canvasFixture } }))
    render(<App engine={{ request } as unknown as EngineClient} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: canvasFixture }} />)
  }

  it('counts a scalar binding and a table binding alike, across all three bands', () => {
    show(bandSpreadCanvas)
    expect(boundCount()).toHaveTextContent('2 of 4 elements bound')
  })

  it('renders nothing at all when the document has no elements', () => {
    show(canvas)
    expect(boundCount()).not.toBeInTheDocument()
    // NON-VACUITY: the same query finds the span for a document that has one.
    show(textCanvas)
    expect(boundCount()).toHaveTextContent('0 of 1 element bound')
  })
})

// STORY 14.6 — FENCE: SAMPLE DATA IS NEVER WRITTEN INTO THE TEMPLATE.
//
// The panel now says this in words, so the words need a guard. Preview input is
// a different channel: `identity` and `render` legitimately carry the accepted
// bytes as RUNTIME data. What must never happen is a sample value reaching the
// document — through a command, or through the bytes a save serialises.
describe('a loaded sample never reaches the template', () => {
  it('keeps every command and every serialised byte free of the sample s values', async () => {
    const sample = acceptSampleData('keys.json', new TextEncoder().encode('{"customer":{"name":"UNIQUE-SAMPLE-VALUE"}}').buffer)
    const serialized = new TextEncoder().encode('folio8 template bytes').buffer
    const request = vi.fn(async (operation: string) => operation === 'serialize'
      ? { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 4, canvas: textCanvas }, bytes: serialized }
      : { snapshot: { documentState: 'loaded' as const, revision: operation === 'command' ? 2 : 1, byteLength: 3, canvas: textCanvas } })
    render(<App engine={{ request } as unknown as EngineClient} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: textCanvas }} initialSampleData={sample} />)
    openDataTab()
    fireEvent.click(screen.getByLabelText(/^text component e1/))
    fireEvent.click(screen.getAllByRole('treeitem').find((item) => item.textContent?.startsWith('customer'))!)
    fireEvent.click(screen.getAllByRole('treeitem').find((item) => item.textContent?.startsWith('name'))!)
    const commands = await waitFor(() => {
      const sent = request.mock.calls.filter(([operation]) => operation === 'command')
      expect(sent).toHaveLength(1)
      return sent as unknown as Array<[string, ArrayBuffer]>
    })
    // NON-VACUITY: the command really was sent and really does carry the PATH.
    // ⚠ ASSERTED AS A PREFIX, NOT AS THE WHOLE ARRAY. A mutation that APPENDS
    // sample data to the segments must red the SCAN below, not this line — a
    // fence that reds on its own non-vacuity witness has proved nothing.
    expect(new TextDecoder().decode(commands[0]![1])).toContain('"id":"e1","segments":["customer","name"')
    for (const [operation, payload] of request.mock.calls as unknown as Array<[string, ArrayBuffer | undefined]>) {
      if (operation !== 'command' && operation !== 'serialize') continue
      expect(payload === undefined ? '' : new TextDecoder().decode(payload), `${operation} must not carry sample data`).not.toContain('UNIQUE-SAMPLE-VALUE')
    }
  })
})

// STORY 14.6 / AC6 — THE ONE DESIGN-MODE ENGINE ROUND-TRIP THIS STORY ADDS.
//
// Opening the DATA tab now issues a `parameter-references` request, which every
// other call site gates on preview mode. That was authorised on one condition:
// the fetch is LAZY (nothing before the tab is opened) and IDEMPOTENT (at most
// once per document generation, never per re-render, selection change or
// keystroke). An unbounded round-trip on the design path ships fine and
// degrades on a large template, so the trigger is counted here rather than
// described in a comment.
describe('the design-mode parameter fetch is lazy and idempotent', () => {
  const referenceCalls = (request: ReturnType<typeof vi.fn>) => request.mock.calls.filter(([operation]) => operation === 'parameter-references')

  it('asks once when the DATA tab opens, never before it, and not again for the same document', async () => {
    const request = vi.fn(async (operation: string) => operation === 'parameter-references'
      ? { snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: textCanvas }, parameterReferences: { revision: 1, names: ['reportDate'] } }
      : { snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: textCanvas } })
    const sample = acceptSampleData('keys.json', new TextEncoder().encode('{"customer":{"name":"Ada"}}').buffer)
    render(<App engine={{ request } as unknown as EngineClient} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: textCanvas }} initialSampleData={sample} blankBytes={new Uint8Array([7]).buffer} />)

    // LAZY: the panel is mounted behind `hidden`, and mounting must not ask.
    expect(referenceCalls(request)).toHaveLength(0)

    openDataTab()
    await waitFor(() => expect(referenceCalls(request)).toHaveLength(1))

    // ⚠ AND THE NAMES MUST REACH THE DOM, NOT MERELY THE ENGINE. Counting
    // requests is not observing the result: with only a call count asserted,
    // dropping the `runtimeParameters` prop at the DataPanel call site leaves
    // `tsc` and the whole suite green while AC6's namespace vanishes from the
    // real app — `RuntimeParameterSection` simply returns null. This is the
    // only assertion in the suite that renders the real <App> and looks for an
    // engine-discovered parameter name on screen.
    await waitFor(() => expect(within(screen.getByLabelText('Runtime parameters from the template')).getByText('reportDate')).toBeInTheDocument())

    // IDEMPOTENT ACROSS EVERY RE-RENDER TRIGGER THE PANEL HAS. Each of these
    // re-renders the DATA tab; none is a new document.
    fireEvent.click(screen.getByLabelText('text component e1'))
    fireEvent.click(screen.getAllByRole('treeitem').find((item) => item.textContent?.startsWith('customer'))!)
    fireEvent.click(screen.getByRole('tab', { name: 'PROPERTIES' }))
    openDataTab()
    fireEvent.click(screen.getByRole('tab', { name: 'PROPERTIES' }))
    openDataTab()
    await settleFrames()
    expect(referenceCalls(request), 'the design-mode fetch must not repeat within one document generation').toHaveLength(1)
  })

  it('asks again for a new document rather than showing the previous template s parameters', async () => {
    const request = vi.fn(async (operation: string) => operation === 'parameter-references'
      ? { snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: textCanvas }, parameterReferences: { revision: 1, names: ['reportDate'] } }
      : { snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 1, canvas: textCanvas } })
    render(<App engine={{ request } as unknown as EngineClient} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: textCanvas }} blankBytes={new Uint8Array([7]).buffer} />)
    openDataTab()
    await waitFor(() => expect(referenceCalls(request)).toHaveLength(1))
    startBlankFromNew()
    await waitFor(() => expect(screen.getByText('Started an unnamed local template')).toBeInTheDocument())
    await waitFor(() => expect(referenceCalls(request), 'a new document generation must re-arm the fetch').toHaveLength(2))
  })
})
