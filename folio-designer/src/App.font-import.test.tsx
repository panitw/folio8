import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { IDBFactory as FakeIndexedDBFactory } from 'fake-indexeddb'
import App from './App'
import type { EngineClient } from './engine-client'
import type { FontFileAccess, LocalFontFile } from './font-file'
import { FileAccessCancelled } from './file/file-access'
import { openFontStore, storedFaceKey, type FamilyCensus, type StoredFace, type StoredFaceRecord } from './font-store'
import { sfntWithNames } from './test/sfnt-fixture'

// STORY 3 AT THE DESIGNER BOUNDARY — THE ACKNOWLEDGEMENT IS THE ADMISSION GATE.
//
// These cases drive the whole designer rather than `font-import.ts`, because
// the claims are about the DESIGNER: "declining stores nothing", "accepting
// puts the faces on this machine", "the family is listed exactly as a
// catalogue family is". None of those can be asserted from the pure import
// module, which writes nothing at all.
//
// ⚠ THE NETWORK IS DOWN IN EVERY CASE HERE, DELIBERATELY. CAP-1's success
// criterion is stated with the catalogue unreachable, and this work adds no
// fetch, no font service and no URL — so a case that passed only with a
// reachable upstream would be asserting the wrong product.

const face = (name: string) => ({ face: name, assetKey: '', family: '', style: '', bold: '', italic: '', boldItalic: '' })
const canvas = { width: 595276, height: 841890, orientation: 'portrait' as const, preset: 'A4' as const, locale: 'en' as const, utcOffset: '+00:00', marginTop: 36000, marginRight: 36000, marginBottom: 36000, marginLeft: 36000, gridIncrement: 6000, commandWidth: 595276, commandHeight: 841890, fontFamilies: ['body'], fontChains: [{ name: 'body', entries: [face('Noto Sans')] }], defaultFontSize: 12000, defaultLineSpacing: 1000, contentWindowHeight: 729890, contentWindowCount: 1, contentWindowOrigins: [0], contentWindowPages: [0], contentWindowCountIsExact: true, bands: [{ name: 'pageHeader' as const, x: 36000, y: 36000, width: 523276, height: 20000 }, { name: 'content' as const, x: 36000, y: 56000, width: 523276, height: 729890 }, { name: 'pageFooter' as const, x: 36000, y: 785890, width: 523276, height: 20000 }], components: [] }
const textComponent = { id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello' }
const engine = (request: unknown) => ({ request }) as unknown as EngineClient
const commandRequest = () => vi.fn(async (operation: string) => ({ snapshot: { documentState: 'loaded' as const, revision: operation === 'command' ? 2 : 1, byteLength: 3, canvas: { ...canvas, components: [textComponent] } } }))

const brandFace = (family: string, subfamily: string, records: ReadonlyArray<Readonly<{ nameID: number; value: string }>> = []): ArrayBuffer =>
  sfntWithNames([
    { platform: 3, nameID: 1, value: family },
    { platform: 3, nameID: 2, value: subfamily },
    ...records.map((record) => ({ platform: 3, ...record })),
  ])

const pickerYielding = (...files: ReadonlyArray<LocalFontFile>): FontFileAccess => ({ openFonts: vi.fn(async () => files) })
const pickerCancelled = (): FontFileAccess => ({ openFonts: vi.fn(async () => { throw new FileAccessCancelled() }) })
const ttf = (name: string, bytes: ArrayBuffer): LocalFontFile => ({ name, mediaType: 'font/ttf', bytes })

/** Everything the store holds right now, read past the designer rather than through its own listing. */
const facesOnThisMachine = async (): Promise<ReadonlyArray<StoredFace>> => {
  const opened = await openFontStore(globalThis.indexedDB)
  if (!opened.ok) return []
  const listed = await opened.value.list()
  return listed.ok ? listed.value : []
}

/** Every census this machine holds, read the same way. */
const censusesOnThisMachine = async (): Promise<ReadonlyArray<FamilyCensus>> => {
  const opened = await openFontStore(globalThis.indexedDB)
  if (!opened.ok) return []
  const listed = await opened.value.listCensus()
  return listed.ok ? listed.value : []
}

/** Writes a face and a census into the store before the designer opens it, as a real install would. */
const seed = async (record: StoredFaceRecord, census: FamilyCensus): Promise<void> => {
  const opened = await openFontStore(globalThis.indexedDB)
  if (!opened.ok) throw new Error(opened.reason)
  await opened.value.put(record)
  await opened.value.putCensus(census)
}

let restoreFetch: typeof globalThis.fetch
let restoreIndexedDB: PropertyDescriptor | undefined

beforeEach(() => {
  restoreFetch = globalThis.fetch
  restoreIndexedDB = Object.getOwnPropertyDescriptor(globalThis, 'indexedDB')
  Object.defineProperty(globalThis, 'indexedDB', { value: new FakeIndexedDBFactory(), configurable: true, writable: true })
  // THE CATALOGUE IS UNREACHABLE. See the file header.
  globalThis.fetch = vi.fn(async () => { throw new TypeError('Failed to fetch') }) as never
})

afterEach(() => {
  globalThis.fetch = restoreFetch
  if (restoreIndexedDB) Object.defineProperty(globalThis, 'indexedDB', restoreIndexedDB)
  else Reflect.deleteProperty(globalThis, 'indexedDB')
})

/** Mounts the designer, selects the text element and opens the font browser, which is the import's door. */
const openFontBrowser = (fontFileAccess?: FontFileAccess) => {
  render(<App engine={engine(commandRequest())} blankBytes={new Uint8Array([1, 2, 3]).buffer} fontFileAccess={fontFileAccess} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: { ...canvas, components: [textComponent] } }} />)
  fireEvent.click(screen.getByLabelText('text component e1'))
  fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
  fireEvent.click(screen.getByRole('button', { name: /^Add fonts…/ }))
  return screen.getByRole('dialog', { name: 'Font browser' })
}

const pressImport = () => fireEvent.click(screen.getByRole('button', { name: 'Import font files from this machine' }))

describe('importing font files from the author\'s own machine', () => {
  it('puts one picked face on this machine once the acknowledgement is accepted, with the catalogue unreachable', async () => {
    openFontBrowser(pickerYielding(ttf('whatever.ttf', brandFace('Brand Grotesk', 'Regular', [{ nameID: 0, value: 'Copyright 2026 A Foundry' }]))))
    pressImport()

    const dialog = await screen.findByRole('dialog', { name: /Import Brand Grotesk/ })
    // THE ASSERTION IS THE AUTHOR'S, AND THE PRODUCT MAKES NO CHECK OF ITS OWN.
    expect(within(dialog).getByText(/hold the right to use these typefaces/)).toBeInTheDocument()
    fireEvent.click(within(dialog).getByRole('button', { name: /I hold the right/ }))

    await waitFor(async () => expect((await facesOnThisMachine()).map((entry) => entry.family)).toEqual(['Brand Grotesk']))
    const [stored] = await facesOnThisMachine()
    expect(stored.style).toBe('Regular')
    expect(stored.copyright).toBe('Copyright 2026 A Foundry')
    expect(stored.mediaType).toBe('font/ttf')
    // NO FILESYSTEM PATH, NO FILENAME, NO MACHINE IDENTITY.
    expect(stored.source).not.toContain('whatever.ttf')
    expect(stored.source).toContain("the author's own machine")
  })

  it('takes ONE acknowledgement for four cuts picked in one gesture, and lists the family ONCE with four cuts', async () => {
    const browser = openFontBrowser(pickerYielding(
      ttf('1.ttf', brandFace('Brand Grotesk', 'Regular')),
      ttf('2.ttf', brandFace('Brand Grotesk', 'Bold')),
      ttf('3.ttf', brandFace('Brand Grotesk', 'Italic')),
      ttf('4.ttf', brandFace('Brand Grotesk', 'Bold Italic')),
    ))
    pressImport()
    const dialog = await screen.findByRole('dialog', { name: /Import 4 faces of Brand Grotesk/ })
    // ONE GESTURE, ONE DIALOG. The four cuts are named by the key a rendering
    // host would build for them, so what the author accepts is what travels.
    expect(within(dialog).getByText(/Brand Grotesk, Brand Grotesk Bold, Brand Grotesk Italic, Brand Grotesk Bold Italic/)).toBeInTheDocument()
    fireEvent.click(within(dialog).getByRole('button', { name: /I hold the right/ }))

    await waitFor(async () => expect(await facesOnThisMachine()).toHaveLength(4))

    // AND THE BROWSER LISTS IT EXACTLY AS A CATALOGUE FAMILY: one row, four
    // cuts, and an Install control that knows the family is already here.
    fireEvent.change(screen.getByRole('textbox', { name: 'Search fonts' }), { target: { value: 'Brand Grotesk' } })
    const rows = await within(browser).findAllByRole('listitem')
    expect(rows).toHaveLength(1)
    expect(within(rows[0]).getByText('Regular · Bold · Italic · Bold Italic')).toBeInTheDocument()
    expect(within(rows[0]).getByRole('button', { name: /Brand Grotesk/ })).toHaveAccessibleName('Brand Grotesk is already on this machine')
  })

  it('groups files from two families into two families, never into one', async () => {
    openFontBrowser(pickerYielding(
      ttf('1.ttf', brandFace('Brand Grotesk', 'Regular')),
      ttf('2.ttf', brandFace('House Serif', 'Bold')),
    ))
    pressImport()
    fireEvent.click(within(await screen.findByRole('dialog', { name: /Import 2 faces of 2 families/ })).getByRole('button', { name: /I hold the right/ }))
    await waitFor(async () => expect((await facesOnThisMachine()).map((entry) => entry.family).sort()).toEqual(['Brand Grotesk', 'House Serif']))
  })

  it('stores NOTHING when the acknowledgement is declined, and holds nothing pending', async () => {
    openFontBrowser(pickerYielding(ttf('1.ttf', brandFace('Brand Grotesk', 'Regular'))))
    pressImport()
    const dialog = await screen.findByRole('dialog', { name: /Import Brand Grotesk/ })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

    await waitFor(() => expect(screen.queryByRole('dialog', { name: /Import Brand Grotesk/ })).toBeNull())
    expect(await facesOnThisMachine()).toEqual([])
    // NOTHING IS HELD PENDING, AND THE NEXT IMPORT ASKS AGAIN RATHER THAN
    // RESUMING: a declined acknowledgement is not a deferred one.
    pressImport()
    expect(within(await screen.findByRole('dialog', { name: /Import Brand Grotesk/ })).getByText(/hold the right to use these typefaces/)).toBeInTheDocument()
    expect(await facesOnThisMachine()).toEqual([])
  })

  it('does nothing at all when the picker is dismissed with no file — no dialog, no error', async () => {
    const access = pickerCancelled()
    openFontBrowser(access)
    pressImport()
    await waitFor(() => expect(access.openFonts).toHaveBeenCalled())
    expect(screen.queryByRole('dialog', { name: /^Import/ })).toBeNull()
    expect(document.querySelector('.font-browser-import-message'), 'a cancelled picker must say nothing at all').toBeNull()
    expect(await facesOnThisMachine()).toEqual([])
  })

  it('imports the three good cuts and names the one it refused', async () => {
    const png = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 13, 0, 0]).buffer
    openFontBrowser(pickerYielding(
      ttf('regular.ttf', brandFace('Brand Grotesk', 'Regular')),
      ttf('bold.ttf', brandFace('Brand Grotesk', 'Bold')),
      ttf('italic.ttf', png),
    ))
    pressImport()
    const dialog = await screen.findByRole('dialog', { name: /Import 2 faces of Brand Grotesk/ })
    expect(within(dialog).getByText(/italic\.ttf/)).toBeInTheDocument()
    fireEvent.click(within(dialog).getByRole('button', { name: /I hold the right/ }))

    await waitFor(async () => expect((await facesOnThisMachine()).map((entry) => entry.style).sort()).toEqual(['Bold', 'Regular']))
    expect(await screen.findByText(/italic\.ttf/)).toBeInTheDocument()
  })

  it('asks nothing and says why when no picked file can be imported', async () => {
    openFontBrowser(pickerYielding(ttf('logo.png', new Uint8Array([1, 2, 3, 4]).buffer)))
    pressImport()
    expect(await screen.findByText(/No face could be imported\./)).toBeInTheDocument()
    expect(screen.queryByRole('dialog', { name: /^Import/ })).toBeNull()
    expect(await facesOnThisMachine()).toEqual([])
  })

  it('refuses the second of two files declaring the same face, and keeps one record for it', async () => {
    const bytes = brandFace('Brand Grotesk', 'Regular')
    openFontBrowser(pickerYielding(ttf('a.ttf', bytes), ttf('a-copy.ttf', bytes)))
    pressImport()
    fireEvent.click(within(await screen.findByRole('dialog', { name: /^Import/ })).getByRole('button', { name: /I hold the right/ }))

    await waitFor(async () => expect(await facesOnThisMachine()).toHaveLength(1))
    expect((await facesOnThisMachine())[0].key).toBe(await storedFaceKey(bytes))
    // AND THE COUNT IS OF FACES ON THE MACHINE, NOT OF FILES PICKED.
    expect(await screen.findByText(/One face is now on this machine\./)).toBeInTheDocument()
    expect(screen.getByText(/a-copy\.ttf/)).toBeInTheDocument()
  })

  it('does not duplicate a face whose bytes this machine already holds — the store is keyed by their SHA-256', async () => {
    const bytes = brandFace('Brand Grotesk', 'Regular')
    openFontBrowser(pickerYielding(ttf('a.ttf', bytes)))
    pressImport()
    fireEvent.click(within(await screen.findByRole('dialog', { name: /^Import/ })).getByRole('button', { name: /I hold the right/ }))
    // WAITING ON THE REPORT AND NOT ON THE STORE, because the control is
    // disabled until the write is over — which is itself the contract that
    // stops the browser closing mid-write.
    expect(await screen.findByText(/One face is now on this machine\./)).toBeInTheDocument()
    await waitFor(async () => expect(await facesOnThisMachine()).toHaveLength(1))

    // A SECOND GESTURE, THE SAME FILE. The acknowledgement is asked again — it
    // is never remembered — and the store still holds exactly one record.
    pressImport()
    fireEvent.click(within(await screen.findByRole('dialog', { name: /^Import/ })).getByRole('button', { name: /I hold the right/ }))
    await waitFor(async () => expect(await screen.findByText(/One face is now on this machine\./)).toBeInTheDocument())
    expect(await facesOnThisMachine()).toHaveLength(1)
    expect((await facesOnThisMachine())[0].key).toBe(await storedFaceKey(bytes))
  })

  // ⚠ THE JOIN THE STORE CHANGE EXISTS FOR. `font-import.test.ts` proves the
  // reader transcribes an absent record as `''`; `font-store.test.ts` proves an
  // empty licence field survives a write. Neither can see the defect that was
  // actually there, which lived exactly where they meet: the face was written,
  // read straight back as CORRUPT and dropped, and the author watched a family
  // they had just imported fail to appear.
  it('lists a face whose copyright, licence and licence-text records are ALL absent, after the listing is re-read', async () => {
    const browser = openFontBrowser(pickerYielding(ttf('silent.ttf', brandFace('Brand Grotesk', 'Regular'))))
    pressImport()
    fireEvent.click(within(await screen.findByRole('dialog', { name: /^Import/ })).getByRole('button', { name: /I hold the right/ }))

    await waitFor(async () => expect(await facesOnThisMachine()).toHaveLength(1))
    const [stored] = await facesOnThisMachine()
    expect(stored.copyright).toBe('')
    expect(stored.licence).toBe('')
    expect(stored.licenceText).toBe('')
    // AND THE DESIGNER'S OWN LISTING SEES IT, which is the half a store-level
    // test cannot reach: the row is drawn from `storedFaces` after
    // `refreshStoredFaces`, so a record dropped as unsound would leave this
    // search empty.
    fireEvent.change(screen.getByRole('textbox', { name: 'Search fonts' }), { target: { value: 'Brand Grotesk' } })
    expect(await within(browser).findByRole('button', { name: 'Brand Grotesk is already on this machine' })).toBeInTheDocument()
  })

  // ⚠ A REGRESSION GUARD OVER SHIPPED BEHAVIOUR. `putCensus` is a plain put
  // keyed by family, so a census written fresh REPLACES the one already there:
  // `published` would shrink to the cuts this machine holds, the family would
  // read COMPLETE, `+ Install` would disappear and the cuts it still lacks
  // would become unreachable.
  it('MERGES into an existing census rather than replacing it, so the cuts still to fetch stay offered', async () => {
    const regular = brandFace('Brand Grotesk', 'Regular')
    // SEEDED BEFORE THE DESIGNER OPENS, because the designer reads the store
    // once on mount: the machine already holds the Regular of a family that
    // publishes four cuts, with its Bold permanently refused upstream — exactly
    // what an install through this designer writes.
    await seed(
      { key: await storedFaceKey(regular), family: 'Brand Grotesk', style: 'Regular', licence: 'OFL-1.1', licenceText: 'terms', copyright: 'Copyright', source: 'google/fonts — ofl/brand/Brand-Regular.ttf, fetched 2026-09-03', mediaType: 'font/ttf', scripts: ['latin'], fetchedAt: '2026-09-03', byteLength: regular.byteLength, bytes: regular },
      { family: 'Brand Grotesk', published: ['Regular', 'Bold', 'Italic', 'Bold Italic'], refused: [{ style: 'Bold', reason: 'upstream publishes no Bold', permanence: 'permanent' }], recordedAt: '2026-09-03' },
    )
    const browser = openFontBrowser(pickerYielding(ttf('italic.ttf', brandFace('Brand Grotesk', 'Italic'))))
    // The seeded census has to have reached the designer before the import
    // merges into it; the row reading `Install` is that read having landed.
    fireEvent.change(screen.getByRole('textbox', { name: 'Search fonts' }), { target: { value: 'Brand Grotesk' } })
    await within(browser).findByRole('button', { name: 'Install Brand Grotesk on this machine' })

    pressImport()
    fireEvent.click(within(await screen.findByRole('dialog', { name: /^Import/ })).getByRole('button', { name: /I hold the right/ }))
    await waitFor(async () => expect(await facesOnThisMachine()).toHaveLength(2))

    const [census] = await censusesOnThisMachine()
    expect([...census.published].sort(), 'the import must not shrink what the family publishes').toEqual(['Bold', 'Bold Italic', 'Italic', 'Regular'])
    expect(census.refused.map((entry) => entry.style), 'a cut upstream permanently refused is still permanently refused').toEqual(['Bold'])

    // AND THE CONSEQUENCE THE AUTHOR SEES: the Bold Italic is still missing, so
    // the family is still offered for install rather than reading as complete.
    expect(await within(browser).findByRole('button', { name: 'Install Brand Grotesk on this machine' })).toBeInTheDocument()
  })

  // THE DIALOG'S KEYBOARD CONTRACT, IN `DeletePageDialog`'s SHAPE. It is a
  // contract and not polish: this dialog is the admission gate, so the way OUT
  // of it without admitting anything has to work from the keyboard alone.
  it('holds focus on Cancel, cycles Tab between the two buttons, and declines on Escape', async () => {
    const browser = openFontBrowser(pickerYielding(ttf('a.ttf', brandFace('Brand Grotesk', 'Regular'))))
    pressImport()
    const dialog = await screen.findByRole('dialog', { name: /^Import/ })
    expect(dialog).toHaveAttribute('aria-modal', 'true')

    const cancel = within(dialog).getByRole('button', { name: 'Cancel' })
    const confirm = within(dialog).getByRole('button', { name: /I hold the right/ })
    await waitFor(() => expect(document.activeElement).toBe(cancel))
    fireEvent.keyDown(dialog, { key: 'Tab' })
    expect(document.activeElement).toBe(confirm)
    fireEvent.keyDown(dialog, { key: 'Tab' })
    expect(document.activeElement).toBe(cancel)

    fireEvent.keyDown(dialog, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog', { name: /^Import/ })).toBeNull())
    expect(await facesOnThisMachine(), 'Escape is a decline, and a decline stores nothing').toEqual([])
    // AND THE KEY NEVER REACHED THE BROWSER BEHIND IT.
    expect(browser).toBeInTheDocument()
    expect(screen.getByRole('dialog', { name: 'Font browser' })).toBeInTheDocument()
  })

  it('offers no import at all in a browser that will not keep typefaces, and says why', async () => {
    Reflect.deleteProperty(globalThis, 'indexedDB')
    openFontBrowser(pickerYielding(ttf('a.ttf', brandFace('Brand Grotesk', 'Regular'))))
    await waitFor(() => expect(screen.getByText(/will not let the designer keep typefaces/)).toBeInTheDocument())
    expect(screen.queryByRole('button', { name: 'Import font files from this machine' })).toBeNull()
  })

  it('does not greet the author with the last import\'s report when the browser is reopened', async () => {
    openFontBrowser(pickerYielding(ttf('a.ttf', brandFace('Brand Grotesk', 'Regular'))))
    pressImport()
    fireEvent.click(within(await screen.findByRole('dialog', { name: /^Import/ })).getByRole('button', { name: /I hold the right/ }))
    expect(await screen.findByText(/One face is now on this machine\./)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Font browser' })).toBeNull())
    fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
    fireEvent.click(screen.getByRole('button', { name: /^Add fonts…/ }))
    expect(screen.queryByText(/now on this machine\./), 'a report about a finished act must not outlive the screen it was made on').toBeNull()
  })
})
