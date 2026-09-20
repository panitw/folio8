import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { IDBFactory as FakeIndexedDBFactory, IDBObjectStore as FakeIndexedDBObjectStore } from 'fake-indexeddb'
import App from './App'
import type { EngineClient } from './engine-client'
import { sfntWithNames } from './test/sfnt-fixture'
import { embeddedFaceFamily } from './embedded-face-family'
import { previewFaceFamily } from './preview-face-family'
import { openFontStore, storedFaceKey, type FontStore, type StoreOutcome, type StoredFaceRecord } from './font-store'
import { webFamilies } from './font-index'
import { catalogueFaces } from './generated/font-catalogue'
import { shippedFaceFamily } from './shipped-face-family'
import { startBlankFromNew } from './test/new-document'

// THE CATALOGUE OFFERS ONE ROW PER FAMILY, NOT ONE PER FACE
// (spec-install-all-face-cuts, story 3). `catalogueFaces` is up to four cuts of
// one family since that story; every count below is about what the family
// control DRAWS, which is one option per family (CAP-5), so the denominator is
// the distinct family count rather than the row count.
const catalogueFamilyCount = new Set(catalogueFaces.map((face) => face.family)).size


// STORY 16.7'S OWN SHORT SAMPLE TEXT, deliberately NOT `font-browser-model.
// ts`'s `latinSample`/`thaiSample` — see `App.tsx`'s own comment on
// `familyControlLatinSample` for why the dropdown draws different text than
// the font browser's full sentence.
const familyControlLatinSample = 'Aa Bb 123'
const familyControlThaiSample = 'กขค Aa'

// STORY 16.2 AT THE BROWSER BOUNDARY — A FETCHED FACE STAYS ON THIS MACHINE.
//
// These tests drive the whole designer, because the claims are about the
// DESIGNER and not about the store module: "picking it again fetches nothing",
// "it works with the network down", "storage that cannot be opened leaves a
// working designer". None of those can be asserted from `font-store.ts` alone.
//
// THE BACKING STORE IS `fake-indexeddb`, installed onto `globalThis` for this
// file only. jsdom 28.1.0 provides no IndexedDB at all, so without it the
// designer would take its own degraded path here and every test below would be
// asserting the degradation. A FRESH factory per test, because the store's
// whole property is that it survives — including, otherwise, into the next
// test.
//
// AND THE RESIDUAL IS NAMED: real browser IndexedDB is proven only by a
// browser, and that witness is routed to Story 16.3's run. Nothing here claims
// otherwise.

const face = (name: string) => ({ face: name, assetKey: '', family: '', style: '', bold: '', italic: '', boldItalic: '' })
const canvas = { width: 595276, height: 841890, orientation: 'portrait' as const, preset: 'A4' as const, locale: 'en' as const, utcOffset: '+00:00', embedFonts: true, marginTop: 36000, marginRight: 36000, marginBottom: 36000, marginLeft: 36000, gridIncrement: 6000, commandWidth: 595276, commandHeight: 841890, fontFamilies: ['body'], fontChains: [{ name: 'body', entries: [face('Noto Sans')] }], defaultFontSize: 12000, defaultLineSpacing: 1000, contentWindowHeight: 729890, contentWindowCount: 1, contentWindowOrigins: [0], contentWindowPages: [0], contentWindowCountIsExact: true, bands: [{ name: 'pageHeader' as const, x: 36000, y: 36000, width: 523276, height: 20000 }, { name: 'content' as const, x: 36000, y: 56000, width: 523276, height: 729890 }, { name: 'pageFooter' as const, x: 36000, y: 785890, width: 523276, height: 20000 }], components: [] }
const textComponent = { id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello' }
const engine = (request: unknown) => ({ request }) as unknown as EngineClient

const kanitFace = sfntWithNames([{ platform: 3, nameID: 0, value: 'Copyright 2020 The Kanit Project Authors' }])
const kanitMetadata = 'name: "Kanit"\nlicense: "OFL"\nfonts {\n  style: "normal"\n  weight: 400\n  filename: "Kanit-Regular.ttf"\n}\n'
const kanitLicence = 'This Font Software is licensed under the SIL Open Font License, Version 1.1.'

/** The upstream a Kanit pick reads: three round-trips, in this order. */
const upstreamFetch = () => vi.fn(async (url: string) => {
  if (url.endsWith('/ofl/kanit/METADATA.pb')) return { ok: true, status: 200, text: async () => kanitMetadata }
  if (url.endsWith('/ofl/kanit/OFL.txt')) return { ok: true, status: 200, text: async () => kanitLicence }
  if (url.endsWith('/ofl/kanit/Kanit-Regular.ttf')) return { ok: true, status: 200, arrayBuffer: async () => kanitFace }
  return { ok: false, status: 404, text: async () => '' }
})

/** Everything the store holds right now, read past the designer rather than through its own list. */
const faceRecordsOnThisMachine = async (): Promise<ReadonlyArray<Readonly<{ family: string }>>> => {
  const opened = await openFontStore(globalThis.indexedDB)
  expect(opened.ok, 'the fake store must open, or reading it back asserts nothing').toBe(true)
  if (!opened.ok) return []
  const listed = await opened.value.list()
  expect(listed.ok).toBe(true)
  return listed.ok ? listed.value : []
}

/**
 * WAITS UNTIL THE STORE REALLY HOLDS A FACE BY THIS FAMILY NAME.
 *
 * Story 16.6 deleted the machine-store panel, which every "a pick installed"
 * test used to poll via `.machine-font-name` as its settle condition — the
 * panel simply rendered whatever `storedFaces` held, so waiting on its DOM was
 * a proxy for waiting on the write. Reading the store directly, past the
 * designer, is the same wait with no deleted DOM to depend on.
 */
const waitForStoredFamily = async (familyName: string): Promise<void> => {
  await waitFor(async () => expect((await faceRecordsOnThisMachine()).map((record) => record.family)).toContain(familyName))
}

// The page font set jsdom does not implement, installed for the one test that
// needs to watch a face reach it. Written the way `App.test.tsx` writes it —
// with neither of the two spellings `canvas-authority-contract.test.ts`
// forbids, so this file needs no carve-out from that contract.
function installStubFontSet(): Readonly<{ restore: () => void; added: string[] }> {
  class StubFace {
    readonly family: string
    constructor(family: string) { this.family = family }
    load(): Promise<StubFace> { return Promise.resolve(this) }
  }
  const added: string[] = []
  const set = { add: (loaded: StubFace) => { added.push(loaded.family); return undefined }, delete: () => undefined }
  Object.defineProperty(globalThis, 'FontFace', { value: StubFace, configurable: true, writable: true })
  Object.defineProperty(document, 'fonts', { value: set, configurable: true, writable: true })
  return { restore: () => { Reflect.deleteProperty(globalThis, 'FontFace'); Reflect.deleteProperty(document, 'fonts') }, added }
}

/**
 * A DOCUMENT THAT PAINTS A FACE IT DOES NOT CARRY.
 *
 * The chain declares one SHIPPED face and no carried entry, so
 * `carriedFaceKeys` — which reads `fontChains[].entries[].assetKey` — is empty
 * and the document-scoped registration has nothing to do. The projection still
 * attributes the fragment to `key`, so the only set that can supply a family
 * for it is the machine store's.
 */
const storedOnlyFaceCanvas = (key: string) => ({
  ...canvas,
  fontFamilies: ['body'],
  fontChains: [{ name: 'body', entries: [face('Noto Sans')] }],
  components: [{ id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'ignored', textPaint: { overflow: false, truncated: false, lines: [{ top: 0, baseline: 12_000, advance: 16_000, width: 24_000, fragments: [{ text: 'สัญญา', x: 0, assetKey: key }] }] } }],
})

/** One complete stored record over `bytes`, written straight into the store before the designer opens it. */
const storedOnly = (key: string, bytes: ArrayBuffer): StoredFaceRecord => ({
  key,
  family: 'A Machine Face',
  style: 'Regular',
  licence: 'OFL-1.1',
  licenceText: kanitLicence,
  copyright: 'Copyright 2026 A Face Only This Machine Has',
  source: 'google/fonts — ofl/amachineface/AMachineFace-Regular.ttf, fetched 2026-09-03',
  authorAcknowledged: false,
  mediaType: 'font/ttf',
  scripts: ['latin'],
  fetchedAt: '2026-09-03',
  byteLength: bytes.byteLength,
  bytes,
})

/**
 * SEEDS A FAMILY THIS MACHINE FULLY HOLDS — THE FACE RECORD **AND** ITS CENSUS.
 *
 * Since spec-install-all-face-cuts story 1 a stored family is not installed by
 * the mere fact of being stored: `familyIsInstalled` asks whether it holds every
 * cut it publishes, and the census is the only authority on what it publishes.
 * A face written without one reads INCOMPLETE — correctly, because a family
 * installed before this change really does have no such record — and the family
 * control stops offering it under AVAILABLE LOCALLY.
 *
 * EVERY TEST BELOW THAT STARTS FROM "this machine already holds X" therefore
 * seeds both, because that is what an install through this designer writes. The
 * incomplete state has its own case, named as such, rather than being the
 * accidental default of every fixture in the file.
 */
const seedInstalled = async (store: FontStore, record: StoredFaceRecord): Promise<StoreOutcome<void>> => {
  const written = await store.put(record)
  if (!written.ok) return written
  return store.putCensus({ family: record.family, published: [record.style], refused: [], recordedAt: record.fetchedAt })
}

let restoreFetch: typeof globalThis.fetch
let restoreIndexedDB: PropertyDescriptor | undefined

beforeEach(() => {
  restoreFetch = globalThis.fetch
  restoreIndexedDB = Object.getOwnPropertyDescriptor(globalThis, 'indexedDB')
  Object.defineProperty(globalThis, 'indexedDB', { value: new FakeIndexedDBFactory(), configurable: true, writable: true })
})

afterEach(() => {
  globalThis.fetch = restoreFetch
  if (restoreIndexedDB) Object.defineProperty(globalThis, 'indexedDB', restoreIndexedDB)
  else Reflect.deleteProperty(globalThis, 'indexedDB')
})

const mount = (request: unknown, chains = canvas.fontChains) => {
  const componentCanvas = { ...canvas, fontChains: chains, components: [textComponent] }
  render(<App engine={engine(request)} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
  fireEvent.click(screen.getByLabelText('text component e1'))
}

const commandRequest = () => vi.fn(async (operation: string) => ({ snapshot: { documentState: 'loaded' as const, revision: operation === 'command' ? 2 : 1, byteLength: 3, canvas: { ...canvas, components: [textComponent] } } }))

// STORY 16.7 GAVE EVERY OPTION AN ARIA-HIDDEN SPECIMEN, so raw `textContent`
// now carries a sample the row's accessible name deliberately excludes. This
// reads what a screen reader would — the row's name and its note, if it still
// has one — by dropping every `aria-hidden` descendant first.
const optionText = (option: Element): string => {
  const clone = option.cloneNode(true) as Element
  clone.querySelectorAll('[aria-hidden="true"]').forEach((node) => node.remove())
  return clone.textContent ?? ''
}

/** Drives the family control the way an author does. Returns false when the row is not offered. */
const pick = (query: string, name: RegExp) => {
  const combobox = screen.getByRole('combobox', { name: 'Font family' })
  fireEvent.focus(combobox)
  fireEvent.change(combobox, { target: { value: query } })
  const option = screen.queryByRole('option', { name })
  if (option) fireEvent.click(option)
  return option !== null
}

const embedPayloads = (request: { mock: { calls: unknown[][] } }) => request.mock.calls
  .filter((call) => call[0] === 'command')
  .map((call) => JSON.parse(new TextDecoder().decode(call[1] as ArrayBuffer)) as Record<string, unknown>)

// STORY 16.3 — THE FONT BROWSER'S SPECIMEN BYTES COME FROM THE SAME THREE TIERS
// A PICK DOES, AND THE STORED TIER IS THE ONE THAT MUST NEED NO NETWORK.
//
// `App.tsx`'s `browserSpecimenBytes` is never executed by `FontBrowser.test.tsx`,
// which supplies its own resolver on every render. A review demonstrated the
// gap two ways — letting the `stored` branch fall through to `fetchWebFamily`,
// and reading `source.family` where it should read `source.record.key` — and in
// both the whole suite stayed green while every face this machine already holds
// would have needed the network. That is precisely the DW-176 property this
// story claims to have witnessed, so it is asserted here, where a fake store and
// a stub page font set already stand up.
describe('the font browser sets a stored family\'s specimen from the store', () => {
  it('reads a stored specimen with every fetch rejecting, and says a web row cannot be shown', async () => {
    const bytes = sfntWithNames([{ platform: 3, nameID: 0, value: 'Copyright 2026 A Face Only This Machine Has' }])
    const key = await storedFaceKey(bytes)
    const opened = await openFontStore(globalThis.indexedDB)
    if (!opened.ok) throw new Error(opened.reason)
    const written = await seedInstalled(opened.value, storedOnly(key, bytes))
    expect(written.ok, 'the fixture face must really be in the store before the designer opens it').toBe(true)

    const fontSet = installStubFontSet()
    try {
      // THE NETWORK IS GONE. Every request fails, so a specimen that renders can
      // only have come from the store.
      const offline = vi.fn(async () => { throw new TypeError('Failed to fetch') })
      globalThis.fetch = offline as never
      mount(commandRequest())
      await waitForStoredFamily('A Machine Face')

      fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
      fireEvent.click(screen.getByRole('button', { name: /^Add fonts…/ }))
      const dialog = screen.getByRole('dialog', { name: 'Font browser' })

      // THE STORED ROW. Searched by name because a family the snapshot does not
      // rank sorts last, which is the correct order and the wrong page.
      fireEvent.change(screen.getByRole('textbox', { name: 'Search fonts' }), { target: { value: 'A Machine Face' } })
      expect(within(dialog).getByText('downloaded to this machine')).toBeInTheDocument()
      const specimen = await within(dialog).findByText('Everyone has the right to freedom of thought')
      expect(specimen.style.fontFamily, 'a family this machine already holds must be set in itself with no network').toBe(previewFaceFamily('A Machine Face'))
      expect(fontSet.added).toContain(previewFaceFamily('A Machine Face'))

      // AND A WEB ROW, ON THE SAME SCREEN, WITH THE SAME NETWORK. It says it
      // cannot be shown rather than borrowing the panel's typeface — which is
      // also what makes the line above a claim about the STORE and not about a
      // resolver that happens to succeed for everything.
      fireEvent.change(screen.getByRole('textbox', { name: 'Search fonts' }), { target: { value: 'Kanit' } })
      expect(await within(dialog).findByText(/Kanit cannot be shown set in itself/)).toBeInTheDocument()
      expect(within(dialog).getByText('downloaded when you install it')).toBeInTheDocument()
    } finally {
      fontSet.restore()
    }
  })
})

// STORY 16.3 — A REFUSAL REACHED THROUGH THE `'caller'` ANNOUNCER IS RETURNED,
// AND THE MODAL IS WHERE IT LANDS.
//
// The seam has two announcers: the family control's pick writes a refusal to the
// panel, and the browser's confirm gets it BACK so it can name it against the
// row that earned it. Nothing asserted the return value on the App side, and a
// review demonstrated what that costs: restore `refuse(reason); return` in place
// of `return refuse(reason)` and every family reports success — `confirm` clears
// the staged set and closes the modal, so a licence refusal or an upstream 404
// dismisses the browser with the author believing everything was embedded, and
// the `'caller'` path paints no panel alert either. The suite stayed green.
describe('the font browser names a refusal the seam returned', () => {
  it('keeps the modal open, names the family, and sends no command', async () => {
    const fontSet = installStubFontSet()
    try {
      // Nothing is published upstream: every probe 404s, which is the refusal
      // `fetchWebFamily` states in its own words.
      const gone = vi.fn(async () => ({ ok: false, status: 404, text: async () => '' }))
      globalThis.fetch = gone as never
      const request = commandRequest()
      mount(request)
      fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
      fireEvent.click(screen.getByRole('button', { name: /^Add fonts…/ }))
      const dialog = screen.getByRole('dialog', { name: 'Font browser' })

      fireEvent.change(screen.getByRole('textbox', { name: 'Search fonts' }), { target: { value: 'Kanit' } })
      fireEvent.click(await within(dialog).findByRole('button', { name: 'Install Kanit on this machine' }))
      fireEvent.click(within(dialog).getByRole('button', { name: 'Install 1 on this machine' }))

      // THE ENGINE'S OWN SENTENCE, INSIDE THE DIALOG, AGAINST THE FAMILY.
      expect(await within(dialog).findByText(/Kanit: Kanit is in this designer's snapshot of the family list but is no longer published upstream/)).toBeInTheDocument()
      // AND THE MODAL IS STILL THERE, with Kanit still staged, so the author
      // can read the refusal and retry without finding the family again.
      expect(screen.getByRole('dialog', { name: 'Font browser' })).toBeInTheDocument()
      expect(within(dialog).getByRole('button', { name: 'Remove Kanit from the families to install' })).toBeInTheDocument()
      // NEUTRALISED VACUOUS SURVIVOR (Story 16.5). This line was
      // `expect(embedPayloads(request)).toEqual([])` — "nothing reached the
      // document" — which was a real claim while confirming embedded and is
      // TRIVIALLY TRUE now that confirming never sends a command at all. It is
      // kept, because a confirm that started sending commands would still be a
      // defect, but it can no longer carry the test on its own.
      //
      // THE CLAIM THAT CAN STILL FAIL IS ABOUT THE MACHINE, WHICH IS WHERE A
      // CONFIRM NOW ACTS. A refused install must leave NOTHING on this machine:
      // no store row, no listing entry, and the family still offered as one to
      // install rather than one already downloaded. An install that wrote a
      // partial record before failing — bytes without terms, which is the
      // record shape the engine refuses a document over — passes the line
      // above and fails every line below it.
      expect(embedPayloads(request)).toEqual([])
      expect(await faceRecordsOnThisMachine(), 'a refused install must keep nothing').toEqual([])
      // AND THE FAMILY CONTROL'S OWN DROPDOWN STILL DOES NOT OFFER IT AT ALL
      // (Story 16.9 removed its install-tier group entirely). `offeredFamilies`
      // reads the store's own listing, so a row that had moved to the
      // already-downloaded note would mean a record survived the refusal —
      // the stronger, now-correct claim is that this control never named it
      // in the first place, refused or not.
      fireEvent.keyDown(dialog, { key: 'Escape' })
      await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Font browser' })).toBeNull())
      const combobox = screen.getByRole('combobox', { name: 'Font family' })
      fireEvent.focus(combobox)
      fireEvent.change(combobox, { target: { value: 'Kanit' } })
      expect(screen.queryByRole('option', { name: /^Kanit/ })).not.toBeInTheDocument()
    } finally {
      fontSet.restore()
    }
  })

  it('does not re-materialise over a replaced document carrying the previous one\'s staged set', async () => {
    const fontSet = installStubFontSet()
    try {
      globalThis.fetch = upstreamFetch() as never
      mount(commandRequest())
      fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
      fireEvent.click(screen.getByRole('button', { name: /^Add fonts…/ }))
      const dialog = screen.getByRole('dialog', { name: 'Font browser' })
      fireEvent.change(screen.getByRole('textbox', { name: 'Search fonts' }), { target: { value: 'Kanit' } })
      fireEvent.click(await within(dialog).findByRole('button', { name: 'Install Kanit on this machine' }))
      expect(within(dialog).getByText(/^1 family ready to install/)).toBeInTheDocument()

      // REPLACING THE DOCUMENT IS THE END OF THIS MODAL, not a pause in it. The
      // open flag lived outside `clearDocumentInteraction`, so the modal
      // vanished only while `canvas` was momentarily undefined and then came
      // BACK over the new document — still carrying a staged set assembled
      // against the old one.
      startBlankFromNew()
      await screen.findByText('Started an unnamed local template')
      // The settle condition is the modal GOING, not the document name — which
      // starts out as `Untitled template` and would have made this pass before
      // the replacement had happened at all.
      await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Font browser' })).toBeNull())
      // And it does not come back. The new document has a canvas of its own, so
      // the render guard alone never hid the modal for more than an instant.
      await waitFor(() => expect(screen.queryByText(/family ready to install/)).toBeNull())
      expect(screen.queryByRole('dialog', { name: 'Font browser' })).toBeNull()
    } finally {
      fontSet.restore()
    }
  })
})

// ─────────────────────────────────────────────────────────────────────────────
// spec-install-all-face-cuts STORY 1 — A PICK INSTALLS EVERY CUT THE FAMILY
// PUBLISHES, AND THE DOCUMENT IS UNCHANGED.
//
// WHY AT THIS LEVEL AND NOT ONLY IN `font-source.test.ts`. That file proves
// what the FETCH returns; these prove what reaches the machine and what does
// not reach the file — the store's record count, each record's own `style` and
// `source`, the census that terminates the re-offer loop, and the fact that no
// engine command is sent by any of it. Those are properties of the designer,
// not of the resolver, and none of them can be asserted from `font-source.ts`
// alone.
describe('installing a family puts every cut it publishes on this machine', () => {
  const twoCutMetadata = `name: "Kanit"
license: "OFL"
fonts {
  style: "normal"
  weight: 400
  filename: "Kanit-Regular.ttf"
}
fonts {
  style: "normal"
  weight: 700
  filename: "Kanit-Bold.ttf"
}
`
  const fourCutMetadata = `name: "Kanit"
license: "OFL"
fonts {
  style: "normal"
  weight: 400
  filename: "Kanit-Regular.ttf"
}
fonts {
  style: "normal"
  weight: 700
  filename: "Kanit-Bold.ttf"
}
fonts {
  style: "italic"
  weight: 400
  filename: "Kanit-Italic.ttf"
}
fonts {
  style: "italic"
  weight: 700
  filename: "Kanit-BoldItalic.ttf"
}
`
  // FOUR DISTINCT BYTE SEQUENCES, so "four records" cannot be satisfied by one
  // face written four times: the store is content-addressed, and four copies of
  // one face would collapse to ONE key.
  const cutBytes: Readonly<Record<string, ArrayBuffer>> = {
    'Kanit-Regular.ttf': sfntWithNames([{ platform: 3, nameID: 0, value: 'Copyright 2020 The Kanit Project Authors' }]),
    'Kanit-Bold.ttf': sfntWithNames([{ platform: 3, nameID: 0, value: 'Copyright 2020 The Kanit Project Authors — Bold' }]),
    'Kanit-Italic.ttf': sfntWithNames([{ platform: 3, nameID: 0, value: 'Copyright 2020 The Kanit Project Authors — Italic' }]),
    'Kanit-BoldItalic.ttf': sfntWithNames([{ platform: 3, nameID: 0, value: 'Copyright 2020 The Kanit Project Authors — Bold Italic' }]),
  }

  /**
   * Upstream serving a four-cut Kanit, minus whatever `withheld` names (404 —
   * upstream publishes no such file) and with `failing` naming cuts that answer
   * a given status instead (5xx — the host having a bad minute). The two are
   * separate arguments because the whole amendment is that they are NOT the
   * same failure.
   */
  const fourCutUpstream = (withheld: ReadonlyArray<string> = [], failing: Readonly<Record<string, number>> = {}) => vi.fn(async (url: string) => {
    if (url.endsWith('/ofl/kanit/METADATA.pb')) return { ok: true, status: 200, text: async () => fourCutMetadata }
    if (url.endsWith('/ofl/kanit/OFL.txt')) return { ok: true, status: 200, text: async () => kanitLicence }
    const file = Object.keys(cutBytes).find((name) => url.endsWith(`/ofl/kanit/${name}`))
    if (file !== undefined && Object.hasOwn(failing, file)) return { ok: false, status: failing[file], text: async () => '' }
    if (file !== undefined && !withheld.includes(file)) return { ok: true, status: 200, arrayBuffer: async () => cutBytes[file] }
    return { ok: false, status: 404, text: async () => '' }
  })

  /**
   * Upstream publishing a Regular and a Bold AND NOTHING ELSE.
   *
   * This is a different condition from `fourCutUpstream(['Kanit-Italic.ttf'])`
   * and the difference is the point: there, upstream publishes an italic and
   * will not serve it, so the census lists it as published and refused. Here
   * there is no italic to refuse, so it must not appear in either list.
   */
  const twoCutUpstream = () => vi.fn(async (url: string) => {
    if (url.endsWith('/ofl/kanit/METADATA.pb')) return { ok: true, status: 200, text: async () => twoCutMetadata }
    if (url.endsWith('/ofl/kanit/OFL.txt')) return { ok: true, status: 200, text: async () => kanitLicence }
    const file = ['Kanit-Regular.ttf', 'Kanit-Bold.ttf'].find((name) => url.endsWith(`/ofl/kanit/${name}`))
    if (file !== undefined) return { ok: true, status: 200, arrayBuffer: async () => cutBytes[file] }
    return { ok: false, status: 404, text: async () => '' }
  })

  /** Everything the store holds about a family, read past the designer. */
  const heldFaces = async (family: string) => (await faceRecordsOnThisMachine() as ReadonlyArray<StoredFaceRecord>).filter((held) => held.family === family)

  /** The family census, read past the designer. */
  const heldCensus = async (family: string) => {
    const opened = await openFontStore(globalThis.indexedDB)
    if (!opened.ok) throw new Error(opened.reason)
    const listed = await opened.value.listCensus()
    if (!listed.ok) throw new Error(listed.reason)
    return listed.value.find((census) => census.family === family)
  }

  /** Installs `Kanit` the one way an author can: through the font browser. */
  const installThroughTheBrowser = async () => {
    fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
    fireEvent.click(screen.getByRole('button', { name: /^Add fonts…/ }))
    const dialog = screen.getByRole('dialog', { name: 'Font browser' })
    fireEvent.change(screen.getByRole('textbox', { name: 'Search fonts' }), { target: { value: 'Kanit' } })
    fireEvent.click(await within(dialog).findByRole('button', { name: 'Install Kanit on this machine' }))
    fireEvent.click(within(dialog).getByRole('button', { name: 'Install 1 on this machine' }))
    return dialog
  }

  it('writes one record per cut, each with its own style and source, and sends no command', async () => {
    globalThis.fetch = fourCutUpstream() as never
    const request = commandRequest()
    mount(request)
    await installThroughTheBrowser()
    await waitFor(async () => expect(await heldFaces('Kanit')).toHaveLength(4))

    const held = await heldFaces('Kanit')
    expect([...held].map((cut) => cut.style).sort()).toEqual(['Bold', 'Bold Italic', 'Italic', 'Regular'])
    // FOUR KEYS, NEVER ONE WRITTEN FOUR TIMES. The store is keyed by the hash
    // of the bytes, so this is also the assertion that each record really holds
    // its own cut's bytes.
    expect(new Set(held.map((cut) => cut.key)).size).toBe(4)
    for (const cut of held) {
      // EVERY CUT CARRIES THE THREE FIELDS THE ENGINE REFUSES A DOCUMENT
      // WITHOUT, because each is its own record and a record without them is a
      // face the store cannot offer back.
      expect(cut.licence).toBe('OFL-1.1')
      expect(cut.licenceText).toBe(kanitLicence)
      expect(cut.copyright, `${cut.style} must carry its OWN nameID 0, not the Regular's`).toContain('Kanit Project Authors')
      expect(cut.mediaType).toBe('font/ttf')
    }
    // AND EACH `source` NAMES ITS OWN FILE. Four records pointing at one path
    // would be four claims about one face.
    const bold = held.find((cut) => cut.style === 'Bold')!
    expect(bold.source).toContain('ofl/kanit/Kanit-Bold.ttf')
    expect(bold.copyright).toContain('Bold')
    expect(held.find((cut) => cut.style === 'Regular')!.source).toContain('ofl/kanit/Kanit-Regular.ttf')
    expect(held.find((cut) => cut.style === 'Bold Italic')!.source).toContain('ofl/kanit/Kanit-BoldItalic.ttf')

    // THE CENSUS RECORDS WHAT UPSTREAM PUBLISHES, with nothing refused.
    expect(await heldCensus('Kanit')).toMatchObject({ family: 'Kanit', published: ['Regular', 'Bold', 'Italic', 'Bold Italic'], refused: [] })

    // AND THE DOCUMENT IS UNTOUCHED. This story installs; it embeds nothing,
    // declares no chain and sends no command — carrying the cuts into a
    // `.folio` is the next story's and doing any of it here would put bytes in
    // an author's file this story promised not to.
    expect(embedPayloads(request), 'installing sends no command at all').toEqual([])
  })

  it('writes only the cuts upstream publishes, and no placeholder for the ones it does not', async () => {
    // A FAMILY PUBLISHING FEWER THAN FOUR INSTALLS WHAT IT HAS. Absence here is
    // not a refusal — there is no italic upstream to refuse — so the census must
    // list exactly what is published and carry NO refusal, and the store must
    // hold exactly two records rather than four with two of them empty.
    globalThis.fetch = twoCutUpstream() as never
    mount(commandRequest())
    await installThroughTheBrowser()
    await waitFor(async () => expect(await heldFaces('Kanit')).toHaveLength(2))

    expect((await heldFaces('Kanit')).map((cut) => cut.style).sort()).toEqual(['Bold', 'Regular'])
    // AND THE CENSUS DOES NOT INVENT THE TWO IT NEVER SAW. A `published` entry
    // for a cut upstream does not publish would read as "never attempted" for
    // ever, which is the one state that keeps re-offering the family.
    expect(await heldCensus('Kanit')).toMatchObject({ family: 'Kanit', published: ['Regular', 'Bold'], refused: [] })
  })

  it('reads installed once every published cut is held, and is not offered for install again', async () => {
    globalThis.fetch = fourCutUpstream() as never
    mount(commandRequest())
    await installThroughTheBrowser()
    await waitFor(async () => expect(await heldFaces('Kanit')).toHaveLength(4))
    // A SUCCESSFUL CONFIRM CLOSES THE DIALOG, so the row state is read from a
    // FRESHLY OPENED one rather than from the detached node the install left
    // behind — which would report the staged state the confirm was acting on.
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Font browser' })).toBeNull())
    fireEvent.click(screen.getByRole('button', { name: /^Add fonts…/ }))
    const reopened = screen.getByRole('dialog', { name: 'Font browser' })
    fireEvent.change(screen.getByRole('textbox', { name: 'Search fonts' }), { target: { value: 'Kanit' } })
    // THE ROW REPORTS THE FAMILY IS HERE AND CANNOT BE STAGED AGAIN — D-5's
    // predicate, reaching the screen.
    const row = await within(reopened).findByLabelText(/Kanit/i, { selector: 'button.font-browser-add' })
    expect(row).toHaveAccessibleName('Kanit is already on this machine')
    expect(row, 'a complete family may not be installed again').toBeDisabled()
    fireEvent.keyDown(reopened, { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Font browser' })).toBeNull())
    // AND IT IS OFFERED FOR USE, ONCE, whatever its face count.
    const combobox = screen.getByRole('combobox', { name: 'Font family' })
    fireEvent.focus(combobox)
    fireEvent.change(combobox, { target: { value: 'Kanit' } })
    expect(screen.getAllByRole('option', { name: /^Kanit/ }), 'four faces is still one row').toHaveLength(1)
  })

  // THE MATRIX'S PER-CUT REFUSAL ROW, AT THE BOUNDARY: the family installs
  // without the cut, nothing is said at pick time (D-2), and the refusal is
  // RECORDED so the family does not re-offer for ever (D-5).
  it('installs the family without a cut upstream will not serve, silently, and records the refusal', async () => {
    globalThis.fetch = fourCutUpstream(['Kanit-Italic.ttf']) as never
    mount(commandRequest())
    await installThroughTheBrowser()
    await waitFor(async () => expect(await heldFaces('Kanit')).toHaveLength(3))

    expect((await heldFaces('Kanit')).map((cut) => cut.style).sort()).toEqual(['Bold', 'Bold Italic', 'Regular'])
    const census = await heldCensus('Kanit')
    expect(census?.published, 'what upstream publishes is unchanged by this machine failing to get it').toEqual(['Regular', 'Bold', 'Italic', 'Bold Italic'])
    expect(census?.refused.map((entry) => entry.style)).toEqual(['Italic'])
    expect(census?.refused[0]!.reason).toMatch(/responded 404/)
    // A 404 IS UPSTREAM STATING WHAT IT PUBLISHES, so the cut is SETTLED — and
    // that is what lets the family read complete two assertions down.
    expect(census?.refused[0]!.permanence).toBe('permanent')
    // NOTHING IS SAID AT PICK TIME. The author asked for a family and got the
    // family; a modal listing the cut upstream would not serve is noise they
    // cannot act on — so the install reads as an ordinary success: the dialog
    // closes, no refusal is named against the row, and no alert is raised.
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Font browser' })).toBeNull())
    expect(screen.queryByText(/Italic/), 'a skipped cut is silent at pick time').toBeNull()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    // AND THE RECORDED REFUSAL SETTLES THE CUT, so the row reads installed
    // rather than re-offering for ever — which is the whole reason D-2's record
    // exists rather than being written off as a skipped cut.
    fireEvent.click(screen.getByRole('button', { name: /^Add fonts…/ }))
    const reopened = screen.getByRole('dialog', { name: 'Font browser' })
    fireEvent.change(screen.getByRole('textbox', { name: 'Search fonts' }), { target: { value: 'Kanit' } })
    expect(await within(reopened).findByLabelText(/Kanit/i, { selector: 'button.font-browser-add' })).toHaveAccessibleName('Kanit is already on this machine')
  })

  // MATRIX ROW: "Transient cut failure | Bold's body stalls, or the machine is
  // offline | Regular installs; Bold recorded transient; family reads
  // INCOMPLETE and the Bold is retried on a later pick".
  //
  // THIS IS THE AMENDMENT'S OWN ACCEPTANCE TEST, IN PLAIN WORDS. The stall
  // sentence `font-source.ts` writes ends "Try the pick again if you like". The
  // first cut of this story recorded a stall exactly like a 404, so the family
  // read complete, the row reported "already on this machine", and the pick
  // that sentence invites COULD NOT BE MADE — a Bold upstream really publishes
  // was gone for good. This drives both halves: the failed pick, then the pick
  // the sentence promises, against an upstream that has recovered.
  it('retries a transiently refused cut on a later pick, so the stall sentence is true for it', async () => {
    // THE FIRST PICK: the Bold's host has a bad minute. 500 is transient; every
    // other cut lands.
    globalThis.fetch = fourCutUpstream([], { 'Kanit-Bold.ttf': 500 }) as never
    mount(commandRequest())
    await installThroughTheBrowser()
    await waitFor(async () => expect(await heldFaces('Kanit')).toHaveLength(3))
    expect((await heldFaces('Kanit')).map((cut) => cut.style).sort()).toEqual(['Bold Italic', 'Italic', 'Regular'])
    const first = await heldCensus('Kanit')
    expect(first?.refused.map((entry) => entry.style)).toEqual(['Bold'])
    expect(first?.refused[0]!.permanence, 'a 5xx says nothing about what upstream publishes').toBe('transient')

    // THE FAMILY READS INCOMPLETE, so the row is offered for install AGAIN —
    // which is the only way the missing Bold is reachable at all.
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Font browser' })).toBeNull())
    // AND IT IS STILL USABLE MEANWHILE (D-8): an incomplete family has not been
    // put out of reach, it has been put back in the installable group.
    const combobox = screen.getByRole('combobox', { name: 'Font family' })
    fireEvent.focus(combobox)
    fireEvent.change(combobox, { target: { value: 'Kanit' } })
    expect(within(screen.getByRole('group', { name: 'AVAILABLE LOCALLY' })).getAllByRole('option').map(optionText)).toContain('Kanit')
    fireEvent.keyDown(combobox, { key: 'Escape' })

    // THE SECOND PICK: upstream has recovered, and the author does exactly what
    // the sentence told them to.
    globalThis.fetch = fourCutUpstream() as never
    const upstream = globalThis.fetch as unknown as ReturnType<typeof fourCutUpstream>
    fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
    fireEvent.click(screen.getByRole('button', { name: /^Add fonts…/ }))
    const dialog = screen.getByRole('dialog', { name: 'Font browser' })
    fireEvent.change(screen.getByRole('textbox', { name: 'Search fonts' }), { target: { value: 'Kanit' } })
    const row = await within(dialog).findByRole('button', { name: 'Install Kanit on this machine' })
    expect(row, 'a transiently refused cut leaves the family offerable').toBeEnabled()
    fireEvent.click(row)
    fireEvent.click(within(dialog).getByRole('button', { name: 'Install 1 on this machine' }))

    // THE BOLD ARRIVES, AND ONLY THE BOLD. The three cuts already held are not
    // refetched — a retry that re-downloaded the whole family would make a bad
    // minute cost the author the whole install a second time.
    await waitFor(async () => expect(await heldFaces('Kanit')).toHaveLength(4))
    const asked = upstream.mock.calls.map(([url]) => String(url))
    expect(asked.filter((url) => url.endsWith('Kanit-Bold.ttf'))).toHaveLength(1)
    expect(asked.filter((url) => url.endsWith('Kanit-Regular.ttf')), 'the held cuts are not refetched').toEqual([])
    // AND THE FAMILY IS NOW COMPLETE, with the transient refusal cleared rather
    // than left standing beside the cut it no longer describes.
    expect(await heldCensus('Kanit')).toMatchObject({ refused: [] })
  })

  // MATRIX ROW: "Held but census-less, offline | Family installed before this
  // story; no network | Still listed in AVAILABLE LOCALLY and still usable
  // (D-8) | Never unusable for want of a census".
  //
  // THIS IS THE REGRESSION D-8 WAS WRITTEN AGAINST. The first cut of this story
  // made `familyIsInstalled` answer D-5's completeness question, so a family
  // installed by any earlier build — which carries no census, because the store
  // had none — read NOT INSTALLED and dropped out of the family control. With
  // the network up that is merely a wasted re-install; with it down it makes a
  // font sitting on the machine unusable, which is strictly worse than the
  // re-offer D-4 accepted.
  it('still lists and applies a family installed before this story, with no network at all', async () => {
    const regularBytes = cutBytes['Kanit-Regular.ttf']!
    const seeded = await openFontStore(globalThis.indexedDB)
    if (!seeded.ok) throw new Error(seeded.reason)
    // NO CENSUS, deliberately: that is exactly what an earlier build left.
    const seedWrite = await seeded.value.put({ ...storedOnly(await storedFaceKey(regularBytes), regularBytes), family: 'Kanit', licenceText: kanitLicence, copyright: 'Copyright 2020 The Kanit Project Authors', source: 'google/fonts — ofl/kanit/Kanit-Regular.ttf, fetched 2026-09-03' })
    expect(seedWrite.ok).toBe(true)
    expect(await heldCensus('Kanit'), 'the premise is a family with NO census').toBeUndefined()

    // THE NETWORK IS GONE, so anything that works can only have come from the
    // store — and anything that needs a fetch simply cannot happen.
    const offline = vi.fn(async () => { throw new TypeError('Failed to fetch') })
    globalThis.fetch = offline as never
    const request = commandRequest()
    mount(request)
    await waitForStoredFamily('Kanit')

    // IT IS LISTED UNDER THE HEADING THAT SAYS THE BYTES ARE HERE.
    fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
    expect(within(screen.getByRole('group', { name: 'AVAILABLE LOCALLY' })).getAllByRole('option').map(optionText)).toContain('Kanit')

    // AND IT CAN STILL BE APPLIED: the embed and the property, two commands, no
    // network touched at all.
    expect(pick('Kanit', /^Kanit$/), 'a census-less family must still be pickable').toBe(true)
    await waitFor(() => expect(embedPayloads(request).map((payload) => payload['kind'])).toEqual(['embedFontFamily', 'updateComponentProperties']))
    expect(offline, 'a family already on this machine must need no network').not.toHaveBeenCalled()
  })

  // D-3 — A FAMILY THIS MACHINE HOLDS A SHORT SET FOR IS INSTALLABLE AGAIN, AND
  // PICKING IT FETCHES ONLY WHAT IT LACKS.
  //
  // This is also the migration case: a family installed before this change
  // holds its Regular and has NO CENSUS, so every cut it lacks reads "never
  // attempted", the family reads incomplete, and it is offered for install
  // again. Nothing migrates it; picking it does.
  it('offers a family installed before this change again, and fetches only the cuts it lacks', async () => {
    const regularBytes = cutBytes['Kanit-Regular.ttf']!
    const seeded = await openFontStore(globalThis.indexedDB)
    if (!seeded.ok) throw new Error(seeded.reason)
    // SEEDED WITHOUT A CENSUS, deliberately: that is exactly what a build
    // before this story left behind.
    const seedWrite = await seeded.value.put({ ...storedOnly(await storedFaceKey(regularBytes), regularBytes), family: 'Kanit', licenceText: kanitLicence, copyright: 'Copyright 2020 The Kanit Project Authors', source: 'google/fonts — ofl/kanit/Kanit-Regular.ttf, fetched 2026-09-03' })
    expect(seedWrite.ok).toBe(true)

    const upstream = fourCutUpstream()
    globalThis.fetch = upstream as never
    mount(commandRequest())
    await waitForStoredFamily('Kanit')
    await installThroughTheBrowser()
    await waitFor(async () => expect(await heldFaces('Kanit')).toHaveLength(4))

    // THE REGULAR WAS NOT REFETCHED. It is the expensive cut and it was already
    // here; refetching it would make the repair cost more than the install.
    expect(upstream.mock.calls.map(([url]) => String(url)).filter((url) => url.endsWith('Kanit-Regular.ttf')), 'the held Regular must not be refetched').toEqual([])
    expect(upstream.mock.calls.map(([url]) => String(url)).filter((url) => url.endsWith('Kanit-Bold.ttf'))).toHaveLength(1)
    // AND THE FAMILY IS NOW COMPLETE, with a census it did not have before.
    expect(await heldCensus('Kanit')).toMatchObject({ published: ['Regular', 'Bold', 'Italic', 'Bold Italic'], refused: [] })
  })

  // D-1 — A CUT'S STORE WRITE FAILS PARTWAY: THE INSTALL IS REFUSED AND WHAT
  // LANDED STAYS.
  //
  // There is no delete path in this designer and none is added. The store is
  // content-addressed, so a cut written before the refusal is an orphan and not
  // a corruption: nothing points at it, no census claims it, and a retry finds
  // it under the same key rather than refetching a face the author already paid
  // for. The refusal is stated at the control the author acted on.
  it('refuses the install when a cut cannot be written, and leaves what already landed', async () => {
    globalThis.fetch = fourCutUpstream() as never
    mount(commandRequest())

    // THE REFUSAL IS INJECTED INTO THE REAL PLUMBING, at the exact place a
    // browser raises it, and only from the SECOND write — so the Regular lands
    // and the Bold does not, which is the partial state the ruling is about.
    const original = FakeIndexedDBObjectStore.prototype.put
    let writes = 0
    FakeIndexedDBObjectStore.prototype.put = function refuse(this: unknown, ...args: unknown[]) {
      writes += 1
      if (writes > 2) throw new DOMException('the origin has no room left for this face', 'QuotaExceededError')
      return (original as (...rest: unknown[]) => unknown).apply(this, args)
    } as typeof original
    let dialog: HTMLElement
    try {
      dialog = await installThroughTheBrowser()
      expect(await within(dialog).findByText(/Kanit was not installed on this machine/)).toBeInTheDocument()
    } finally {
      FakeIndexedDBObjectStore.prototype.put = original
    }

    // WHAT LANDED STAYS. The Regular is on this machine and nothing deleted it.
    expect((await heldFaces('Kanit')).map((cut) => cut.style)).toEqual(['Regular'])
    // AND NO CENSUS WAS WRITTEN, so the family reads incomplete and is offered
    // again — which is what makes the retry reachable.
    expect(await heldCensus('Kanit'), 'a refused install must not record a census that claims completeness').toBeUndefined()
  })
})

describe('a fetched face stays on this machine', () => {
  // RETIRED (Story 16.9): drove the install mechanism itself — fetch, keep,
  // send no command, offer the row back as already downloaded — by picking
  // the dropdown's removed install-tier row. There is no route left from
  // this control to a family that is not yet on this machine, so an install
  // can no longer be provoked here. The store-write half is unit-tested
  // directly in `font-store.test.ts`; the font browser's own "Add fonts…"
  // flow is the one remaining door to an install, and is untouched by this
  // story.

  // AC2 AND AC3 TOGETHER, WHICH IS THE WHOLE POINT OF THE STORE, RE-ANCHORED BY
  // STORY 16.5 ONTO THE MOMENT THAT NOW CARRIES THE EMBED.
  //
  // BEHAVIOUR-CHANGED. The first pick used to embed AND keep, so the reference
  // payload this test compared against was the first document's own command.
  // Installing sends no command, so that reference no longer exists — the
  // comparison is now against THE FIXTURE BYTES THEMSELVES, which is a stronger
  // anchor anyway: it cannot agree with a payload the designer produced twice
  // from the same defect.
  //
  // The claim is unchanged and is the one Story 16.2 exists for: a second
  // document, with THE NETWORK REMOVED ENTIRELY, still gets the face and its
  // terms out of the store. Only the trigger moved, from the pick to first use.
  //
  // TRIGGER CHANGED BY STORY 16.9: 'Kanit' reaches this machine by being
  // seeded directly into the store — exactly the record an install through
  // the (now removed) dropdown row would have written — rather than by
  // picking that row live. What this test owns is the SECOND document's
  // no-network first use, which is unaffected by how the face arrived.
  it('embeds a stored family in a second document with no network at all, fetching nothing', async () => {
    const seedKey = await storedFaceKey(kanitFace)
    const seeded = await openFontStore(globalThis.indexedDB)
    if (!seeded.ok) throw new Error(seeded.reason)
    const seedWrite = await seedInstalled(seeded.value, { ...storedOnly(seedKey, kanitFace), family: 'Kanit', licence: 'OFL-1.1', licenceText: kanitLicence, copyright: 'Copyright 2020 The Kanit Project Authors', source: 'google/fonts — ofl/kanit/Kanit-Regular.ttf, fetched 2026-09-03' })
    expect(seedWrite.ok, 'the fixture face must really be in the store before the designer opens it').toBe(true)

    globalThis.fetch = upstreamFetch() as never
    const first = commandRequest()
    mount(first)
    // THE SETTLE CONDITION IS THE STORE ROW, and it has to be: there is no
    // command to wait on any more, so waiting on one would wait for ever.
    await waitForStoredFamily('Kanit')
    expect(embedPayloads(first), 'nothing has embedded anything yet').toEqual([])
    startBlankFromNew()
    await waitFor(() => expect(screen.getByText('Untitled template')).toBeInTheDocument())

    // THE NETWORK IS GONE. Any request at all now fails, so a pick that
    // succeeds can only have come from the store.
    const offline = vi.fn(async () => { throw new TypeError('Failed to fetch') })
    globalThis.fetch = offline as never
    fireEvent.click(screen.getByLabelText('text component e1'))
    const before = embedPayloads(first).length
    expect(pick('Kanit', /^Kanit$/), 'the stored family must still be offered offline').toBe(true)
    await waitFor(() => expect(embedPayloads(first).length).toBeGreaterThan(before))
    expect(offline, 'a stored pick must not reach the network at all').not.toHaveBeenCalled()

    // AND IT IS THE SAME DOCUMENT CONTENT, not a degraded one: the same bytes
    // the upstream served, the three fields the engine refuses a document
    // without, and the same twelve-field arity.
    const stored = embedPayloads(first).find((payload) => payload['kind'] === 'embedFontFamily')!
    expect(stored, 'first use of a stored family must send the embed command').toBeDefined()
    expect(stored['data']).toBe(btoa(String.fromCharCode(...new Uint8Array(kanitFace))))
    expect(stored['licence']).toBe('OFL-1.1')
    expect(stored['licenceText']).toBe(kanitLicence)
    expect(stored['copyright']).toBe('Copyright 2020 The Kanit Project Authors')
    expect(String(stored['source'])).toContain('ofl/kanit/Kanit-Regular.ttf')
    // A CATALOGUE FACE OUT OF THE MACHINE STORE ACKNOWLEDGES NOTHING: the key
    // is on the wire because the arity counts it, and it is `false`, so the
    // document records no acknowledgement at all.
    expect(stored['authorAcknowledged']).toBe(false)
    expect(Object.keys(stored)).toHaveLength(13)

    // TWO COMMANDS, IN THIS ORDER, AND TWO UNDO ENTRIES. `canvas.fontFamilies`
    // is the closed set `style.fontFamily` may name, so the property command is
    // refused until the chain is declared — the engine forces the order and
    // nothing in the designer chooses it.
    await waitFor(() => expect(embedPayloads(first).map((payload) => payload['kind'])).toEqual(['embedFontFamily', 'updateComponentProperties']))
    const property = embedPayloads(first).at(-1)!
    expect(property['changes']).toEqual({ fontFamily: { op: 'set', value: 'Kanit' } })
    expect(property['ids']).toEqual(['e1'])
  })

  // RETIRED (Story 16.9): drove the offline-install refusal by picking the
  // dropdown's removed install-tier row for a family not on this machine —
  // there is no such row left to pick. `fetchWebFamily`'s own offline
  // sentence is unit-tested directly, independent of any UI, in
  // `font-source.test.ts` ("You cannot install a family without a network
  // connection").

  // RETIRED, STORY 16.6: `it('removes a face by name, and says documents that
  // embed it are unchanged', ...)`. Deleted by owner decision (D-16.R.82) along
  // with the per-face removal control it drove — `Remove Kanit (Regular) from
  // this machine` no longer exists anywhere in the designer, so nothing can
  // drive this test's premise. There is no replacement: the capability itself
  // is gone, not merely moved, and the deletion's own guard is the family
  // control still offering the face under `AVAILABLE LOCALLY` with no way to
  // let it go — asserted below in "renders no store section with faces
  // present, and offers no way to remove one".

  // RETIRED (Story 16.9): this drove the whole 3 → 2 → 1 journey — install
  // moving a row into AVAILABLE LOCALLY, first use moving it into IN THIS
  // TEMPLATE — by picking the dropdown's own install-tier row. The first leg
  // (3 → 2) no longer has a row to start from: there is nothing left on this
  // machine's control at group 3 to pick. THE SECOND LEG SURVIVES ELSEWHERE:
  // `App.test.tsx`'s "embeds and then commits the property, as two commands,
  // when a family this machine holds is picked" asserts the 2 → 1 move (two
  // commands, no network, offered once) against `Inter`, a local-tier family
  // that reaches AVAILABLE LOCALLY without a pick at all.

  // AND THE TWO COMMANDS ARE TWO SEPARATELY UNDOABLE ENTRIES, WHICH IS THE HALF
  // A COMMAND COUNT CANNOT SEE.
  //
  // The matrix row says "two commands, TWO UNDOS", and the contract's own Always
  // bullet says the fork keeps its shape: two decisions, two undos, a fusion
  // refused at 8.6 and refused again at 16.5. Every existing statement of that
  // in this file is a COMMENT — measured, four `undo` mentions in this file and
  // none of them an assertion — and a comment is not a measurement.
  //
  // DEPTH IS DRIVEN, NEVER READ. `App.tsx:147` exposes only the boolean
  // `canUndo`, so "two entries" is the observation that Undo is STILL ENABLED
  // after one press and disabled only after the second. The stub's depth is a
  // counter over the commands the designer actually sends — not a hardcoded
  // `canUndo: true`, which would pass over a fused single command.
  // TRIGGER CHANGED BY STORY 16.9: 'Kanit' is seeded directly into the store
  // rather than installed by picking the dropdown's removed install-tier
  // row, so the control for "installing pushes nothing to undo" moved with
  // it — there is nothing left in this test that installs at all.
  it('leaves two separately undoable entries after one pick from AVAILABLE LOCALLY', async () => {
    const seedKey = await storedFaceKey(kanitFace)
    const seeded = await openFontStore(globalThis.indexedDB)
    if (!seeded.ok) throw new Error(seeded.reason)
    const seedWrite = await seedInstalled(seeded.value, { ...storedOnly(seedKey, kanitFace), family: 'Kanit', licenceText: kanitLicence, copyright: 'Copyright 2020 The Kanit Project Authors' })
    expect(seedWrite.ok, 'the fixture face must really be in the store before the designer opens it').toBe(true)

    globalThis.fetch = upstreamFetch() as never
    let revision = 1
    let undoDepth = 0
    let declaredChains = ['body']
    // ONE COMMAND, ONE UNDO ENTRY — the engine's own rule (`folio-go/internal/wasm/engine.go`'s
    // single `pushUndo` per applied command), modelled here so the depth this
    // test reads is a consequence of what the designer sent rather than a
    // property of the stub.
    const historySnapshot = () => ({ documentState: 'loaded' as const, revision, byteLength: 3, canUndo: undoDepth > 0, canRedo: false, canvas: { ...canvas, fontFamilies: declaredChains, components: [textComponent] } })
    const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
      if (operation === 'command' && payload) {
        revision += 1
        undoDepth += 1
        const parsed = JSON.parse(new TextDecoder().decode(payload)) as Record<string, unknown>
        if (parsed['kind'] === 'embedFontFamily') declaredChains = ['body', 'Kanit']
        return { snapshot: historySnapshot() }
      }
      if (operation === 'undo') { revision += 1; undoDepth -= 1; return { snapshot: historySnapshot() } }
      return { snapshot: historySnapshot() }
    })
    mount(request)
    const undo = () => screen.getByRole('button', { name: 'Undo' })
    await waitForStoredFamily('Kanit')
    expect(undo(), 'nothing has been sent yet').toBeDisabled()

    // FIRST USE — the embed, then the property.
    expect(pick('Kanit', /^Kanit$/)).toBe(true)
    await waitFor(() => expect(embedPayloads(request).map((sent) => sent['kind'])).toEqual(['embedFontFamily', 'updateComponentProperties']))
    await waitFor(() => expect(undo()).toBeEnabled())

    // THE FIRST UNDO TAKES BACK ONE OF THEM, NOT BOTH.
    fireEvent.click(undo())
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'undo')).toHaveLength(1))
    expect(undo(), 'one gesture left TWO entries: a fused command would leave nothing to undo here').toBeEnabled()

    // AND THE SECOND EMPTIES THE HISTORY — exactly two, never three.
    fireEvent.click(undo())
    await waitFor(() => expect(request.mock.calls.filter(([operation]) => operation === 'undo')).toHaveLength(2))
    await waitFor(() => expect(undo(), 'two entries, not three: the pick may not push anything else').toBeDisabled())
  })

  // THE REGRESSION THE ORDER REPAIR EXISTS FOR, DRIVEN THROUGH THE CONTROL
  // RATHER THAN ASSERTED OVER `offeredFamilies`.
  //
  // At HEAD a stored family took the INDEX POSITION of the web row it
  // replaced. `Philosopher` sits at offset 891 of 1273 in the web-tier
  // snapshot — a fact `offeredFamilies`' repaired order must not let leak
  // into this control, which draws no web-tier rows at all any more
  // (Story 16.9) and so has no cap left to swallow anything under. The claim
  // this test still owns is narrower than it once was and no less real: a
  // stored face ranked deep in a list this control no longer shows must
  // still appear under AVAILABLE LOCALLY, drawn in full.
  it('draws a deeply-ranked stored family under AVAILABLE LOCALLY, in full', async () => {
    const bytes = sfntWithNames([{ platform: 3, nameID: 0, value: 'Copyright 2026 A Face Only This Machine Has' }])
    const key = await storedFaceKey(bytes)
    const opened = await openFontStore(globalThis.indexedDB)
    if (!opened.ok) throw new Error(opened.reason)
    const written = await seedInstalled(opened.value, { ...storedOnly(key, bytes), family: 'Philosopher' })
    expect(written.ok, 'the fixture face must really be in the store before the designer opens it').toBe(true)
    expect(webFamilies.some((row) => row.family === 'Philosopher'), 'the fixture must really rank deep in the web-tier snapshot, or this measures nothing').toBe(true)
    // WITH NO NETWORK, so nothing here can be explained by a fetch.
    globalThis.fetch = vi.fn(async () => { throw new TypeError('Failed to fetch') }) as never
    mount(commandRequest())
    await waitForStoredFamily('Philosopher')

    fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
    const local = within(screen.getByRole('group', { name: 'AVAILABLE LOCALLY' })).getAllByRole('option')
    // IT IS UNDER THE HEADING THAT SAYS THE BYTES ARE HERE, AND IT SAYS SO.
    expect(local.map(optionText)).toContain('Philosopher')
    // AND THE GROUP IS DRAWN IN FULL — 31 committed faces plus this one, with
    // no cap anywhere in this control to have swallowed it.
    expect(local, 'every installed row renders').toHaveLength(catalogueFamilyCount + 1)
    expect(screen.queryByRole('group', { name: 'AVAILABLE TO INSTALL' })).not.toBeInTheDocument()
    // AND IT IS OFFERED ONCE: from the store, never also from the snapshot.
    expect(screen.getAllByRole('option', { name: /^Philosopher/ })).toHaveLength(1)
  })

  // RETIRED (Story 16.9): the 3 → 1 store-unavailable degrade this drove by
  // picking the dropdown's removed install-tier row for 'Kanit'. There is no
  // route left from this control to a family that is not yet on this
  // machine, so the scenario cannot be provoked here any more. See the
  // retirement note on "still designs and still adds fonts when the store
  // cannot be opened at all" further down this file for the gap this and
  // that test both leave: neither this file nor `FontBrowser.test.tsx`
  // drives the degrade to a completed embed through the font browser's own
  // confirm flow, the one remaining door to an install.

  // AVAILABLE LOCALLY HAS NO CAP AT ALL (Story 16.9 removed the only other
  // group, the one that ever had one). With 31 committed faces and one stored
  // family the claim would be too easy to satisfy by accident, so the fixture
  // pushes the installed group to 56 rows — well past the 50-row bound the
  // removed group used to carry — and every one of them must still render.
  it('renders every installed row, uncapped, even when the installed group is far larger than any cap this control ever had', async () => {
    const opened = await openFontStore(globalThis.indexedDB)
    if (!opened.ok) throw new Error(opened.reason)
    const from = Math.floor(webFamilies.length * 0.7)
    const deep = webFamilies.slice(from, from + 25)
    expect(deep, 'the fixture needs enough stored families to overflow a union-wide cap').toHaveLength(25)
    for (const row of deep) {
      const bytes = sfntWithNames([{ platform: 3, nameID: 0, value: `Copyright 2026 ${row.family}` }])
      const written = await seedInstalled(opened.value, { ...storedOnly(await storedFaceKey(bytes), bytes), family: row.family })
      expect(written.ok, `the fixture face for ${row.family} must reach the store`).toBe(true)
    }
    globalThis.fetch = vi.fn(async () => { throw new TypeError('Failed to fetch') }) as never
    mount(commandRequest())
    fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
    // THE GROUP ITSELF IS THE SETTLE CONDITION — the deleted panel no longer
    // offers a DOM count to wait on, and this is a more direct proof of the
    // property under test anyway.
    await waitFor(() => expect(within(screen.getByRole('group', { name: 'AVAILABLE LOCALLY' })).getAllByRole('option')).toHaveLength(catalogueFamilyCount + deep.length))
    const local = within(screen.getByRole('group', { name: 'AVAILABLE LOCALLY' })).getAllByRole('option')
    expect(local, 'every row under a heading promising the bytes are here must be drawn').toHaveLength(catalogueFamilyCount + deep.length)
    expect(local.length, 'the fixture must really exceed any cap this control ever had, or this measures nothing').toBeGreaterThan(50)
    expect(screen.queryByRole('group', { name: 'AVAILABLE TO INSTALL' }), 'there is no third group left to bound anything against').not.toBeInTheDocument()
  })

  // RETIRED, STORY 16.6: `it('draws the region both refusals name, and puts the
  // remove control inside it', ...)`. The named region — `role="group"`,
  // `aria-label="Typefaces downloaded to this machine"` — and the per-face
  // remove control it carried are both deleted by owner decision (D-16.R.82).
  // `storeWriteRefusal` no longer names any region (its remedy clause is cut;
  // `font-store.test.ts` asserts the sentence names no remedy), and
  // `lateEmbedRefusal` no longer distinguishes a removable face from a bundled
  // one, so there is nothing left for either half of this test to check.

  // STORY 16.2's SELF-HEALING CONTRACT, WHICH STORY 16.5 BRIEFLY BROKE AND THEN
  // RESTORED — AND WHICH NOTHING HAD EVER ASSERTED.
  //
  // 16.2's matrix: *"Stored bytes fail to decode later | Corrupt entry | Entry
  // treated as absent and dropped; refetch on next pick | Self-healing, logged
  // honestly."* This story's first pass replaced that fallback with a refusal,
  // and the suite stayed green — because no test covered it. That is the same
  // gap the matrix audit found on two other rows, so the restored behaviour gets
  // a test rather than a comment.
  //
  // THE ENTRY IS DELETED BEHIND THE DESIGNER'S BACK, which is the real shape of
  // this: another tab removed it, or the store dropped it as unsound between the
  // listing and the read. The designer still believes it is installed — that is
  // the state the fallback exists for.
  //
  // BOTH HALVES ARE ASSERTED. The refetch must happen (a refusal would red the
  // embed assertion) AND the store must be healed (a fetch with no write back
  // would leave the next use fetching again for ever).
  // TRIGGER CHANGED BY STORY 16.9: seeded directly into the store rather than
  // reached by picking the dropdown's removed install-tier row.
  it('refetches and heals when a stored entry has gone missing, rather than refusing first use', async () => {
    const seedKey = await storedFaceKey(kanitFace)
    const seeded = await openFontStore(globalThis.indexedDB)
    if (!seeded.ok) throw new Error(seeded.reason)
    const seedWrite = await seedInstalled(seeded.value, { ...storedOnly(seedKey, kanitFace), family: 'Kanit', licenceText: kanitLicence, copyright: 'Copyright 2020 The Kanit Project Authors' })
    expect(seedWrite.ok, 'the fixture face must really be in the store before the designer opens it').toBe(true)

    globalThis.fetch = upstreamFetch() as never
    const request = commandRequest()
    mount(request)
    await waitForStoredFamily('Kanit')
    const installed = await faceRecordsOnThisMachine()
    expect(installed.map((record) => record.family)).toEqual(['Kanit'])

    // THE ENTRY VANISHES WITHOUT THE DESIGNER BEING TOLD. Its own listing still
    // has the row, so the family is still offered as one to use.
    const opened = await openFontStore(globalThis.indexedDB)
    if (!opened.ok) throw new Error(opened.reason)
    await opened.value.remove((installed[0] as unknown as { key: string }).key)
    expect(await faceRecordsOnThisMachine(), 'the fixture must really have emptied the store').toEqual([])

    // FIRST USE STILL WORKS. A refusal here would be the permanent local failure
    // the ruling refused: the author would have to find and press a removal
    // control on a row whose face is already gone.
    expect(pick('Kanit', /^Kanit$/), 'the stale listing must still offer the family').toBe(true)
    await waitFor(() => expect(embedPayloads(request).map((payload) => payload['kind'])).toEqual(['embedFontFamily', 'updateComponentProperties']))
    expect(screen.queryByRole('alert'), 'a self-healing path states no refusal').not.toBeInTheDocument()

    // AND THE STORE IS HEALED, not merely bypassed.
    await waitFor(async () => expect((await faceRecordsOnThisMachine()).map((record) => record.family)).toEqual(['Kanit']))
  })

  // RETIRED (Story 16.9): matrix row "Install a variable face" drove the
  // refusal by picking a family from the dropdown's own (now removed)
  // install-tier row — there is no route left from this control to a family
  // that is not yet on this machine, so an install-time refusal can no
  // longer be provoked here. The `fvar` filter itself is unit-tested
  // directly, independent of any UI, in `font-variable-face-tie.test.ts`
  // ("the install-time fvar filter, over the bytes Go embeds"). The font
  // browser's own "Add fonts…" flow is the one remaining door to an install,
  // and is untouched by this story; this end-to-end message wording is not
  // re-verified there.

  // MATRIX ROW: "Late embed refusal, stored face | Engine refuses an installed
  // face | One sentence: refused, nothing written, face still on this machine —
  // no removal instruction".
  //
  // THIS IS THE ONE ADMISSION CHECK THAT COULD NOT MOVE TO INSTALL, and the
  // whole reason the story is allowed to leave it in Go is that the residue is
  // DISCLOSED rather than left to surprise the author. An undisclosed dead end
  // is what D-16.R.46 Q4 forbids; a dead end the author can see is a stated
  // limit — but Story 16.6 deletes the one control that could ever have
  // cleared it, so the sentence below no longer offers a remedy, only the
  // disclosure.
  //
  // The engine's refusal is stubbed rather than provoked with a mislabelled
  // binary: the tie itself is Go's, proven in
  // `folio-go/internal/fontset/licencesignature_test.go` over committed bytes.
  // What is unproven anywhere else — and what this owns — is what the DESIGNER
  // does with a refusal that arrives at first use.
  // TRIGGER CHANGED BY STORY 16.9: 'Kanit' reaches AVAILABLE LOCALLY by being
  // seeded straight into the store rather than by picking the dropdown's
  // removed install-tier row — this test is about the refusal at FIRST USE,
  // which is unaffected by that removal.
  it('says an installed face cannot be embedded, names no removal control, and leaves it installed', async () => {
    const seedKey = await storedFaceKey(kanitFace)
    const seeded = await openFontStore(globalThis.indexedDB)
    if (!seeded.ok) throw new Error(seeded.reason)
    const seedWrite = await seedInstalled(seeded.value, { ...storedOnly(seedKey, kanitFace), family: 'Kanit', licenceText: kanitLicence, copyright: 'Copyright 2020 The Kanit Project Authors' })
    expect(seedWrite.ok, 'the fixture face must really be in the store before the designer opens it').toBe(true)

    globalThis.fetch = upstreamFetch() as never
    const sent: Record<string, unknown>[] = []
    const contradiction = 'fonts: font "Kanit": the face\'s own name table records it under the SIL Open Font License while this document declares Apache-2.0'
    const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
      if (operation === 'command' && payload) {
        const parsed = JSON.parse(new TextDecoder().decode(payload)) as Record<string, unknown>
        sent.push(parsed)
        // THE ENGINE REFUSES THE EMBED, exactly as the nameID-13 tie does: a
        // rejection carrying a located message, through the ordinary
        // command/diagnostic path.
        if (parsed['kind'] === 'embedFontFamily') throw { dataPath: 'fonts.Kanit', message: contradiction }
      }
      return { snapshot: { documentState: 'loaded' as const, revision: operation === 'command' ? 2 : 1, byteLength: 3, canvas: { ...canvas, components: [textComponent] } } }
    })
    mount(request)
    await waitForStoredFamily('Kanit')

    // FIRST USE — and this is where the refusal arrives.
    expect(pick('Kanit', /^Kanit$/)).toBe(true)
    const alert = await screen.findByRole('alert')

    // THE THREE THINGS THE ENGINE'S OWN SENTENCE CANNOT SAY, AND NO FOURTH.
    expect(alert.textContent, 'the author must be told the face IS here').toMatch(/Kanit is installed on this machine and cannot be embedded in this document/)
    expect(alert.textContent, "the engine's own reason must survive, not be replaced by a friendlier one").toContain(contradiction)
    expect(alert.textContent, 'the author must be told their file did not move').toMatch(/Nothing was written to the document/)
    // AND NO REMOVAL INSTRUCTION (Story 16.6): there is no control left to
    // point at, so the sentence stops offering a remedy rather than naming one
    // that is not on the screen.
    expect(alert.textContent, 'no remedy remains to offer').not.toMatch(/remove/i)

    // THE PROPERTY COMMAND WAS NOT SENT. `canvas.fontFamilies` never gained the
    // chain, so committing `style.fontFamily` would have been refused anyway —
    // and sending it would put a second, unrelated refusal in front of the
    // author on top of this one.
    expect(sent.map((payload) => payload['kind'])).toEqual(['embedFontFamily'])

    // AND THE FACE IS STILL ON THIS MACHINE — the sentence's own claim, checked
    // against the store rather than a deleted panel.
    expect((await faceRecordsOnThisMachine()).map((record) => record.family)).toEqual(['Kanit'])
  })

  // RETIRED (Story 16.9): matrix row 7 ("Store write fails at install")
  // drove a failing `put` through `installFamily` by picking a family from
  // the dropdown's own (now removed) install-tier row — there is no route
  // left from this control to a family that is not yet on this machine, so
  // an install-time refusal can no longer be provoked here at all. The font
  // browser's own "Add fonts…" flow is the one remaining door to an install,
  // and is untouched by this story; this scenario is not re-verified there.
  // Owner: whoever next touches `installFamily`'s store-write refusal.

  // THE FAILED HEAL IS SILENT NOW (Story 16.6, reversing 16.2's
  // stated-degradation clause by owner decision).
  //
  // `storeWriteDegradation` was the one sentence for this exact path — the
  // write-back after a refetch at first use — and its only render site was the
  // deleted panel's status line. Deleting the panel deletes the sentence with
  // it: there is nowhere left for it to appear, so this test now asserts the
  // document keeps the face and the machine quietly does not, with nothing
  // said, rather than asserting what used to be said.
  // TRIGGER CHANGED BY STORY 16.9: seeded directly into the store rather than
  // reached by picking the dropdown's removed install-tier row — the scenario
  // under test (a refetch at first use) starts from an already-installed
  // family regardless of how it got there.
  it('silently fails to heal the store after a refetch, while the document keeps the face', async () => {
    const seedKey = await storedFaceKey(kanitFace)
    const seeded = await openFontStore(globalThis.indexedDB)
    if (!seeded.ok) throw new Error(seeded.reason)
    const seedWrite = await seedInstalled(seeded.value, { ...storedOnly(seedKey, kanitFace), family: 'Kanit', licenceText: kanitLicence, copyright: 'Copyright 2020 The Kanit Project Authors' })
    expect(seedWrite.ok, 'the fixture face must really be in the store before the designer opens it').toBe(true)

    globalThis.fetch = upstreamFetch() as never
    const request = commandRequest()
    mount(request)
    await waitForStoredFamily('Kanit')
    const installed = await faceRecordsOnThisMachine()
    const opened = await openFontStore(globalThis.indexedDB)
    if (!opened.ok) throw new Error(opened.reason)
    await opened.value.remove((installed[0] as unknown as { key: string }).key)

    // THE REFETCH SUCCEEDS AND THE HEAL DOES NOT.
    const original = FakeIndexedDBObjectStore.prototype.put
    FakeIndexedDBObjectStore.prototype.put = function refuse(): never { throw new DOMException('the origin has no room left for this face', 'QuotaExceededError') }
    try {
      expect(pick('Kanit', /^Kanit$/)).toBe(true)
      await waitFor(() => expect(embedPayloads(request).map((payload) => payload['kind'])).toEqual(['embedFontFamily', 'updateComponentProperties']))
    } finally {
      FakeIndexedDBObjectStore.prototype.put = original
    }
    // THE DOCUMENT HAS THE FACE — the refetch and the embed both succeeded.
    expect(embedPayloads(request).map((payload) => payload['kind'])).toEqual(['embedFontFamily', 'updateComponentProperties'])
    // AND THE MACHINE QUIETLY DOES NOT. Read past the designer to prove the
    // heal genuinely failed rather than merely going unreported: with `put`
    // refusing, the store must stay empty.
    expect(await faceRecordsOnThisMachine(), 'the heal must have really failed, not merely gone unreported').toEqual([])
    expect(screen.queryByRole('alert'), 'a write-back failure is silent, not a refusal').not.toBeInTheDocument()
  })

  // MATRIX ROW: "Late embed refusal, bundled face | Engine refuses a local-tier
  // face | The same sentence as the stored case".
  //
  // Before Story 16.6, `lateEmbedRefusal` took a `removable` flag and this
  // tier's refusal read differently from the stored tier's — pointing the
  // author at a "Remove …" control that was never on the screen for a bundled
  // face, since it ships inside the release rather than in the machine store.
  // The flag and both its branches are gone now, so the two tiers must produce
  // the IDENTICAL sentence — asserted here against the exact string, not a
  // pattern, so the two tiers cannot quietly drift apart again.
  it('names a bundled face the same way it names a stored one, with no removal instruction', async () => {
    const bundled = sfntWithNames([{ platform: 3, nameID: 0, value: 'Copyright 2020 The Inter Project Authors' }])
    // The local tier reads its bytes from the release's own content-addressed
    // assets — a relative URL, never a host.
    globalThis.fetch = vi.fn(async (url: string) => {
      if (/^https?:/.test(String(url))) throw new TypeError('no third party may be contacted for a bundled face')
      return { ok: true, status: 200, arrayBuffer: async () => bundled }
    }) as never
    const contradiction = 'fonts: font "Inter": the face declares a licence its own name table contradicts'
    const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
      if (operation === 'command' && payload) {
        const parsed = JSON.parse(new TextDecoder().decode(payload)) as Record<string, unknown>
        if (parsed['kind'] === 'embedFontFamily') throw { dataPath: 'fonts.Inter', message: contradiction }
      }
      return { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas: { ...canvas, components: [textComponent] } } }
    })
    mount(request)
    expect(pick('Inter', /^Inter$/), 'a committed local-tier family must be offered for use').toBe(true)
    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toBe(`Inter is installed on this machine and cannot be embedded in this document: ${contradiction} Nothing was written to the document, and the face is still on this machine.`)
    expect(alert.textContent, 'no remedy remains to offer').not.toMatch(/remove/i)
  })

  // THE DOCUMENT MAY BE REPLACED WHILE THE EMBED IS IN FLIGHT, AND THE PROPERTY
  // COMMIT MUST NOT FOLLOW IT INTO THE NEW ONE.
  //
  // Every other async commit in `App.tsx` guards this way and there are tests
  // named for it; the third arm is a NEW async path and did not inherit it. It
  // matters because `applyProperties` SENDS FIRST AND GUARDS AFTER — it
  // dispatches the command unconditionally and only declines to install the
  // RESULT — so a stale commit is not a dropped response, it is
  // `updateComponentProperties` reaching the engine carrying the PREVIOUS
  // document's element ids.
  //
  // TRIGGER CHANGED BY STORY 16.9: 'Kanit' used to reach the machine by being
  // picked from the dropdown's own (now removed) `AVAILABLE TO INSTALL` row.
  // The scenario under test starts from a family already installed, so it is
  // unaffected by that removal — the fixture is seeded directly into the
  // store instead, exactly as `waitForStoredFamily`'s own callers already do
  // elsewhere in this file.
  it('does not commit the property when the document is replaced while the embed is in flight', async () => {
    const key = await storedFaceKey(kanitFace)
    const opened = await openFontStore(globalThis.indexedDB)
    if (!opened.ok) throw new Error(opened.reason)
    const written = await seedInstalled(opened.value, { ...storedOnly(key, kanitFace), family: 'Kanit', licenceText: kanitLicence, copyright: 'Copyright 2020 The Kanit Project Authors' })
    expect(written.ok, 'the fixture face must really be in the store before the designer opens it').toBe(true)

    globalThis.fetch = upstreamFetch() as never
    let release: () => void = () => {}
    const held = new Promise<void>((resolve) => { release = resolve })
    const sent: Record<string, unknown>[] = []
    let holdTheEmbed = false
    const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
      if (operation === 'command' && payload) {
        const parsed = JSON.parse(new TextDecoder().decode(payload)) as Record<string, unknown>
        sent.push(parsed)
        if (holdTheEmbed && parsed['kind'] === 'embedFontFamily') await held
      }
      return { snapshot: { documentState: 'loaded' as const, revision: operation === 'command' ? 2 : 1, byteLength: 3, canvas: { ...canvas, components: [textComponent] } } }
    })
    mount(request)
    await waitForStoredFamily('Kanit')

    holdTheEmbed = true
    expect(pick('Kanit', /^Kanit$/)).toBe(true)
    await waitFor(() => expect(sent.map((payload) => payload['kind'])).toEqual(['embedFontFamily']))

    // THE DOCUMENT IS REPLACED WHILE THE EMBED IS STILL IN FLIGHT.
    startBlankFromNew()
    await waitFor(() => expect(screen.getByText('Untitled template')).toBeInTheDocument())
    release()

    // AND THE PROPERTY NEVER FOLLOWS IT. Settled twice over — once through the
    // event loop and once through a fresh selection round-trip — so this is a
    // claim about the guard and not about when the assertion happened to run.
    await new Promise((resolve) => setTimeout(resolve, 0))
    fireEvent.click(screen.getByLabelText('text component e1'))
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(sent.map((payload) => payload['kind']), 'the property commit belongs to a document that is no longer open').toEqual(['embedFontFamily'])
    // AND NOTHING IS SHOWN AGAINST A CONTROL THE AUTHOR IS NO LONGER LOOKING AT.
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
  // THE ONE-RESOLUTION-AT-A-TIME GUARD, BOTH DIRECTIONS, AT THE ONLY LAYER THAT
  // HOLDS IT.
  //
  // `App.tsx:114` keeps the busy flag in a REF as well as in state, and
  // `setCurrentSnapshot` clears BOTH on a document replacement while
  // `embedInstalledFamily`'s own `finally` deliberately does NOT release a hold
  // whose generation has moved. Two copies of one flag, released from two
  // places, is a hazard with two distinct failure modes, and neither is visible
  // from `font-source.ts` or `font-licence.ts` — the guard is not in either
  // module, so no unit test of either can reach it. The two tests below own it.
  //
  // TRIGGER: `AVAILABLE LOCALLY` first use, which Story 16.9 left standing. The
  // deleted tests drove the removed `AVAILABLE TO INSTALL` row; the mechanism
  // they exercised is reachable through this door unchanged, so the fixture is
  // seeded straight into the store the way every other test in this file does.

  // (a) THE HOLD IS RELEASED WHEN THE DOCUMENT IS REPLACED UNDER IT.
  //
  // The releasing `finally` is generation-guarded, so on a replacement it is
  // `setCurrentSnapshot`'s own `holdFontChain(false)` — and nothing else — that
  // hands the flag back. Clear only the state copy there and the ref stays true
  // for the life of the session: every later pick takes the busy branch, the
  // author is told the designer is busy by a designer that is doing nothing,
  // and no font can ever be used again without a reload. Silent, permanent, and
  // invisible to a test that only replaces the document and stops there.
  it('releases the pick hold when the document is replaced mid-resolution, so a later pick still commits', async () => {
    const key = await storedFaceKey(kanitFace)
    const opened = await openFontStore(globalThis.indexedDB)
    if (!opened.ok) throw new Error(opened.reason)
    const written = await seedInstalled(opened.value, { ...storedOnly(key, kanitFace), family: 'Kanit', licenceText: kanitLicence, copyright: 'Copyright 2020 The Kanit Project Authors' })
    expect(written.ok, 'the fixture face must really be in the store before the designer opens it').toBe(true)

    globalThis.fetch = upstreamFetch() as never
    let release: () => void = () => {}
    const held = new Promise<void>((resolve) => { release = resolve })
    const sent: Record<string, unknown>[] = []
    let holdTheEmbed = false
    const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
      if (operation === 'command' && payload) {
        const parsed = JSON.parse(new TextDecoder().decode(payload)) as Record<string, unknown>
        sent.push(parsed)
        if (holdTheEmbed && parsed['kind'] === 'embedFontFamily') await held
      }
      return { snapshot: { documentState: 'loaded' as const, revision: operation === 'command' ? 2 : 1, byteLength: 3, canvas: { ...canvas, components: [textComponent] } } }
    })
    mount(request)
    await waitForStoredFamily('Kanit')

    holdTheEmbed = true
    expect(pick('Kanit', /^Kanit$/), 'the installed family must be offered for first use').toBe(true)
    await waitFor(() => expect(sent.map((payload) => payload['kind'])).toEqual(['embedFontFamily']))

    // THE DOCUMENT IS REPLACED WHILE THAT FIRST RESOLUTION IS STILL RUNNING, and
    // then the first resolution finishes into a document nobody is looking at.
    startBlankFromNew()
    await waitFor(() => expect(screen.getByText('Untitled template')).toBeInTheDocument())
    holdTheEmbed = false
    release()
    await new Promise((resolve) => setTimeout(resolve, 0))

    // AND THE NEXT PICK STILL WORKS. This is the whole claim: the flag was
    // handed back by the replacement, not stranded by a `finally` that declined
    // to release it.
    fireEvent.click(screen.getByLabelText('text component e1'))
    expect(pick('Kanit', /^Kanit$/), 'the installed family is still offered in the replacement document').toBe(true)
    await waitFor(() => expect(sent.map((payload) => payload['kind']), 'a pick after the replacement must reach the engine, not the busy branch').toEqual(['embedFontFamily', 'embedFontFamily', 'updateComponentProperties']))
    expect(screen.queryByRole('alert'), 'a released hold states no refusal').not.toBeInTheDocument()
  })

  // (b) THE HOLD IS TAKEN AT ALL, so a second pick made while the first is still
  // resolving is REFUSED rather than run beside it.
  //
  // THE SECOND PICK IS MADE AGAINST ANOTHER COMPONENT, because that is what
  // makes it a claim about `App.tsx`'s flag and not about the control's own
  // `pendingRef`: `ComponentProperties` is keyed by the selection, so clicking
  // the second box REMOUNTS the family control with a fresh per-control guard.
  // The only thing left standing between two picks at that instant is the
  // designer-wide hold — and if it is not taken, two resolutions run side by
  // side and two `embedFontFamily` commands commit for one author gesture each.
  it('refuses a pick made against another component while the first embed is still in flight, and sends one embed', async () => {
    const key = await storedFaceKey(kanitFace)
    const opened = await openFontStore(globalThis.indexedDB)
    if (!opened.ok) throw new Error(opened.reason)
    const written = await seedInstalled(opened.value, { ...storedOnly(key, kanitFace), family: 'Kanit', licenceText: kanitLicence, copyright: 'Copyright 2020 The Kanit Project Authors' })
    expect(written.ok, 'the fixture face must really be in the store before the designer opens it').toBe(true)

    globalThis.fetch = upstreamFetch() as never
    let release: () => void = () => {}
    const held = new Promise<void>((resolve) => { release = resolve })
    const sent: Record<string, unknown>[] = []
    let holdTheEmbed = false
    const twoBoxes = { ...canvas, components: [textComponent, { ...textComponent, id: 'e2', y: 48_000 }] }
    const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
      if (operation === 'command' && payload) {
        const parsed = JSON.parse(new TextDecoder().decode(payload)) as Record<string, unknown>
        sent.push(parsed)
        if (holdTheEmbed && parsed['kind'] === 'embedFontFamily') await held
      }
      return { snapshot: { documentState: 'loaded' as const, revision: operation === 'command' ? 2 : 1, byteLength: 3, canvas: twoBoxes } }
    })
    render(<App engine={engine(request)} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: twoBoxes }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))
    await waitForStoredFamily('Kanit')

    holdTheEmbed = true
    expect(pick('Kanit', /^Kanit$/), 'the installed family must be offered for first use').toBe(true)
    await waitFor(() => expect(sent.map((payload) => payload['kind'])).toEqual(['embedFontFamily']))

    // THE AUTHOR MOVES TO THE OTHER BOX AND PICKS AGAIN, mid-resolution.
    fireEvent.click(screen.getByLabelText('text component e2'))
    expect(pick('Kanit', /^Kanit$/), 'the row is still offered against the second component').toBe(true)
    // SETTLED GENEROUSLY FIRST, so a second resolution that DID start has
    // reached its own `embedFontFamily` by the time this is read — otherwise
    // the count would agree merely because nothing had run yet. Ten macrotasks,
    // not one: the stored tier reads IndexedDB before it dispatches, and one
    // tick is measurably too few to carry that read to the command.
    for (let tick = 0; tick < 10; tick++) await new Promise((resolve) => setTimeout(resolve, 0))
    expect(sent.map((payload) => payload['kind']), 'one resolution at a time: the second pick sends no command of its own').toEqual(['embedFontFamily'])
    const alert = await screen.findByRole('alert')
    expect(alert.textContent, 'the second pick is refused in words, against the control the author used').toBe('Kanit was not used: the designer was busy with another change. Try it again.')

    // AND THE FIRST RESOLUTION IS UNHARMED BY THE REFUSAL — it finishes, and the
    // property it was going to set is the one it declines to set, because the
    // selection moved. Settled through the event loop so this is a claim about
    // the guard and not about when the assertion ran.
    holdTheEmbed = false
    release()
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(sent.map((payload) => payload['kind']), 'still exactly one embed for the two picks').toEqual(['embedFontFamily'])
  })

  // ─────────────────────────────────────────────────────────────────────────
  // STORY 11.4 — THE DECLARE PATH'S TWO CONCURRENCY GUARDS.
  //
  // ⚠ BOTH WERE DELETABLE WITH A GREEN SUITE, PROVED BY MUTATION: removing
  // `declareShippedFamily`'s busy guard AND its post-await
  // document/selection-generation guard left the designer suite at exactly the
  // same file and test counts. The embed path's twins are driven by the
  // neighbouring test above, which uses a WEB-tier family and therefore never
  // reaches this function at all — the fork routes a shipped family somewhere
  // else, so the coverage that looked like it covered both covered one.
  // ─────────────────────────────────────────────────────────────────────────

  // GUARD ONE: THE DESIGNER-WIDE HOLD.
  //
  // Selecting the second box REMOUNTS the family control with a fresh
  // per-control `pendingRef`, so the only thing standing between two declares
  // at that instant is `fontChainBusyRef`. Without it two resolutions run side
  // by side and two `addFontChain` commands commit for one author gesture each
  // — and the second is refused by the engine as a duplicate chain name, which
  // is a refusal the author did nothing to earn.
  //
  // IT IS ALSO WHAT PINS `{ action: 'embed' }` ON THIS PATH. `FontFamilyProperty`
  // paints a pick refusal only when `pickError.control.action === 'embed'`, so
  // the alert asserted below is on screen ONLY because the declare path — which
  // embeds nothing — sends that exact string. A well-meaning rename to
  // `'declare'` without moving the painter's predicate would silently stop
  // surfacing every refusal this function makes; this assertion is what reds.
  it('refuses a second declare of a shipped family while the first is in flight, and sends one addFontChain', async () => {
    let release: () => void = () => {}
    const held = new Promise<void>((resolve) => { release = resolve })
    const sent: Record<string, unknown>[] = []
    let holdTheDeclare = true
    const twoBoxes = { ...canvas, components: [textComponent, { ...textComponent, id: 'e2', y: 48_000 }] }
    const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
      if (operation === 'command' && payload) {
        const parsed = JSON.parse(new TextDecoder().decode(payload)) as Record<string, unknown>
        sent.push(parsed)
        if (holdTheDeclare && parsed['kind'] === 'addFontChain') await held
      }
      return { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas: twoBoxes } }
    })
    render(<App engine={engine(request)} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: twoBoxes }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))

    expect(pick('Roboto', /^Roboto$/), 'Roboto is a shipped family the catalogue offers').toBe(true)
    await waitFor(() => expect(sent.map((payload) => payload['kind'])).toEqual(['addFontChain']))

    fireEvent.click(screen.getByLabelText('text component e2'))
    expect(pick('Roboto', /^Roboto$/), 'the row is still offered against the second component').toBe(true)
    // SETTLED GENEROUSLY, so a second resolution that DID start has reached its
    // own command by the time this is read — otherwise the count would agree
    // merely because nothing had run yet.
    for (let tick = 0; tick < 10; tick++) await new Promise((resolve) => setTimeout(resolve, 0))
    expect(sent.map((payload) => payload['kind']), 'one resolution at a time: the second pick declares nothing of its own').toEqual(['addFontChain'])
    const alert = await screen.findByRole('alert')
    expect(alert.textContent, 'the second pick is refused IN WORDS, at the control the author used').toBe('Roboto was not used: the designer was busy with another change. Try it again.')

    holdTheDeclare = false
    release()
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(sent.map((payload) => payload['kind']), 'still exactly one declare for the two picks').toEqual(['addFontChain'])
  })

  // GUARD TWO: THE DOCUMENT AND SELECTION THE ANSWER COMES BACK TO.
  //
  // The chain command is awaited, and the property commit that follows it names
  // the element ids the control was mounted with. If the selection — or the
  // whole document — moves under a slow machine while that await is open, the
  // commit would arrive carrying ids from a selection the author has left, or
  // from a document that no longer exists. Setting a property on a component
  // the author is not looking at is the failure; a document replacement is the
  // worse half of it, and it takes the SAME branch, because replacing the
  // document bumps `documentGeneration` AND clears the selection
  // (`setCurrentSnapshot`'s `clearDocumentInteraction`).
  //
  // THE CHAIN IS STILL DECLARED, and that is correct rather than a shortfall:
  // the command was accepted by the engine before the author moved, and undoing
  // it is the author's to decide.
  it('declares the chain but sets no component when the selection moves while the declare is in flight', async () => {
    let release: () => void = () => {}
    const held = new Promise<void>((resolve) => { release = resolve })
    const sent: Record<string, unknown>[] = []
    let holdTheDeclare = true
    const twoBoxes = { ...canvas, components: [textComponent, { ...textComponent, id: 'e2', y: 48_000 }] }
    const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
      if (operation === 'command' && payload) {
        const parsed = JSON.parse(new TextDecoder().decode(payload)) as Record<string, unknown>
        sent.push(parsed)
        if (holdTheDeclare && parsed['kind'] === 'addFontChain') await held
      }
      return { snapshot: { documentState: 'loaded' as const, revision: 2, byteLength: 3, canvas: twoBoxes } }
    })
    render(<App engine={engine(request)} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: twoBoxes }} />)
    fireEvent.click(screen.getByLabelText('text component e1'))

    expect(pick('Roboto', /^Roboto$/)).toBe(true)
    await waitFor(() => expect(sent.map((payload) => payload['kind'])).toEqual(['addFontChain']))

    // THE AUTHOR MOVES ON, and only then does the engine answer.
    fireEvent.click(screen.getByLabelText('text component e2'))
    holdTheDeclare = false
    release()
    for (let tick = 0; tick < 10; tick++) await new Promise((resolve) => setTimeout(resolve, 0))

    expect(sent.map((payload) => payload['kind']), 'no property is committed into a selection the author has left').toEqual(['addFontChain'])
    // AND THE ID IS NAMED, so the assertion above cannot pass for the wrong
    // reason on some later day when a second command is legitimately sent.
    const properties = sent.filter((payload) => payload['kind'] === 'updateComponentProperties')
    expect(properties.flatMap((payload) => payload['ids'] as ReadonlyArray<string>), 'a stale commit would carry e1, the element the pick was started on').toEqual([])
  })

  // STORY 11.4 / P19 — A STORED ROW FOR A FAMILY THE RELEASE ALREADY SHIPS
  // ROUTES TO THE DECLARE PATH, AND ITS FETCHED BYTES GO UNUSED. DELIBERATELY.
  //
  // `Noto Sans` and `Noto Sans Thai` are installable from the web index, so a
  // `stored` row for one of them can reach the family control's fork carrying
  // bytes this designer fetched and kept. The fork asks the DECLARED MIRROR,
  // not the tier, so that row is NAMED rather than embedded — which is the
  // right answer and not an accident: embedding would put a copy of a face
  // every machine already has into the document, as a Regular-only entry that
  // can never bold while the shipped Bold sits in the same FontSet.
  //
  // The stored copy is not wasted — it is what the specimen is painted with —
  // and nothing is deleted from the store. This fork decides what a DOCUMENT
  // carries, not what this machine keeps.
  it('names a shipped family even when this machine holds fetched bytes for it', async () => {
    const key = await storedFaceKey(kanitFace)
    const opened = await openFontStore(globalThis.indexedDB)
    if (!opened.ok) throw new Error(opened.reason)
    const written = await seedInstalled(opened.value, { ...storedOnly(key, kanitFace), family: 'Noto Sans', licenceText: kanitLicence, copyright: 'Copyright 2026 The Noto Project Authors' })
    expect(written.ok, 'the fixture face must really be in the store before the designer opens it').toBe(true)

    globalThis.fetch = upstreamFetch() as never
    const request = commandRequest()
    mount(request)
    await waitForStoredFamily('Noto Sans')

    expect(pick('Noto Sans', /^Noto Sans$/), 'the stored family must be offered for first use').toBe(true)
    await waitFor(() => expect(embedPayloads(request).map((payload) => payload['kind'])).toEqual(['addFontChain', 'updateComponentProperties']))
    const declared = embedPayloads(request)[0]
    // NO BYTES TRAVELLED. `embedFontFamily` is the only command that carries
    // any, and it was not sent.
    expect(declared['data'], 'a shipped family is named, never carried, whatever this machine happens to hold').toBeUndefined()
    // AND IT DECLARED THE SHIPPED FAMILY'S CUTS, which is the thing the stored
    // Regular-only copy could never have provided.
    expect(declared['entries']).toEqual([
      { face: 'Noto Sans', bold: 'Noto Sans Bold', italic: 'Noto Sans Italic', boldItalic: 'Noto Sans Bold Italic' },
      { face: 'Noto Sans Thai', bold: 'Noto Sans Thai Bold' },
      'Noto Sans SC',
    ])
    // THE STORE IS UNTOUCHED: routing a pick past the bytes does not throw them
    // away, and the specimen is still painted from them.
    expect((await faceRecordsOnThisMachine()).map((record) => record.family)).toContain('Noto Sans')
  })

  // THE DEGRADED CONFIRM WARNING, ASSERTED AGAINST THE CODE THAT DECIDES IT AND
  // NOT THE CODE THAT DISPLAYS IT.
  //
  // `FontBrowser.test.tsx` proves the dialog renders the warning when it is
  // HANDED `storeKeepsFaces={false}`. That proves a component renders what it is
  // told and nothing about what it is told: hardcode `storeKeepsFaces={true}` at
  // the `App.tsx` call site and that test stays green while a private-window
  // author sees `Install 5 on this machine` on a button that writes five faces
  // into their document. The clause would exist, a test would assert it, and the
  // alibi would still hold.
  //
  // SO THIS DRIVES THE WHOLE DESIGNER WITH NO STORE AT ALL and reads the name off
  // the real control, which is the only version of the claim the acceptance
  // criterion actually makes.
  it('computes the degraded confirm warning from a store it really could not open', async () => {
    Reflect.deleteProperty(globalThis, 'indexedDB')
    const fontSet = installStubFontSet()
    try {
      globalThis.fetch = upstreamFetch() as never
      mount(commandRequest())

      fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
      fireEvent.click(screen.getByRole('button', { name: /^Add fonts…/ }))
      const dialog = screen.getByRole('dialog', { name: 'Font browser' })
      fireEvent.change(screen.getByRole('textbox', { name: 'Search fonts' }), { target: { value: 'Kanit' } })
      fireEvent.click(await within(dialog).findByRole('button', { name: 'Install Kanit on this machine' }))

      // THE COUNT TRAVELS IN THE NAME OF THE CONTROL THAT DOES IT, and the name
      // CONTAINS the visible label rather than replacing it (WCAG 2.5.3), so a
      // speech-input author can still say what they can see.
      //
      // `findByRole` RATHER THAN `getByRole`, AND THAT IS THE SETTLE CONDITION.
      // `storeKeepsFaces` settles asynchronously after `openFontStore` resolves,
      // with no DOM to poll before this dialog even opens (Story 16.6 deleted
      // the panel note that used to serve as a settle signal), so the retry has
      // to live in the query that actually depends on the state, not before it.
      const confirm = await within(dialog).findByRole('button', { name: 'Install 1 on this machine — this browser will not keep fonts, so confirming adds 1 family to this document' })
      expect(confirm.textContent).toBe('Install 1 on this machine')
      expect(within(dialog).getByText(/This browser will not keep typefaces, so these go straight into the document/)).toBeInTheDocument()
    } finally {
      fontSet.restore()
    }
  })

  // RETIRED (Story 16.9), WITH A GAP NAMED RATHER THAN HIDDEN. This drove the
  // store-unavailable degrade (installing anyway, embedding directly, when
  // IndexedDB cannot be opened — D-16.R.82) through the dropdown's own
  // install-tier pick, which no longer exists. The neighbouring test
  // "computes the degraded confirm warning from a store it really could not
  // open" still proves the FONT BROWSER's confirm button carries the right
  // degraded label in this state, but nothing here or in `FontBrowser.test.tsx`
  // drives that confirm click through to a completed embed — `FontBrowser.
  // test.tsx` mocks `onAddFamily` rather than exercising `installFamily`'s
  // real degrade path. RETIRING RATHER THAN REWRITING: rebuilding this against
  // the browser's confirm flow needs a correctness check of that flow this
  // story did not do, and a wrong guess here would be a worse record than an
  // honestly named gap. Owner: whoever next touches `installFamily`'s
  // store-unavailable branch or the font browser's confirm handler.

  // THE MACHINE-SCOPED PREVIEW REGISTRATION, PINNED IN BOTH DIRECTIONS.
  //
  // `App.tsx` keeps TWO registrations — the document's carried faces and this
  // machine's stored faces — and unions them into `paintableFaces`, which is
  // what the canvas is given. Until this test, that union was worth nothing to
  // any assertion: a reviewer mutation-proved it by handing the canvas
  // `carriedFaces` instead of `paintableFaces` at BOTH call sites, and the
  // whole suite stayed green. The one machine-scoped registration in the
  // application had no test at all.
  //
  // THE FIXTURE IS THE ASSERTION. The document declares ONE chain entry, a
  // shipped face, so it carries no face of its own and `carriedFaces` is empty
  // for the life of this test. The store holds one face, and the projection
  // attributes its fragment to that face's asset key. So the fragment can only
  // acquire a family if the set the canvas is given is LARGER than the set the
  // document carries — which is the exact property the mutation removes.
  //
  // Neither registration is asked to do the other's job: nothing here goes
  // through the engine's `asset` operation, and the assertion below that the
  // engine was never asked for one is what says so.
  it('registers a face this machine holds but this document does not carry, and paints with it', async () => {
    const bytes = sfntWithNames([{ platform: 3, nameID: 0, value: 'Copyright 2026 A Face Only This Machine Has' }])
    const key = await storedFaceKey(bytes)
    const opened = await openFontStore(globalThis.indexedDB)
    if (!opened.ok) throw new Error(opened.reason)
    const written = await seedInstalled(opened.value, storedOnly(key, bytes))
    expect(written.ok, 'the fixture face must really be in the store before the designer opens it').toBe(true)

    const fontSet = installStubFontSet()
    try {
      // Every `asset` request is recorded, because the assertion at the end of
      // this test is that there were NONE: the document carries no face, so the
      // document-scoped registration has nothing to ask the engine for.
      const assetRequests: string[] = []
      const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
        if (operation === 'asset') assetRequests.push(new TextDecoder().decode(payload))
        return { snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: storedOnlyFaceCanvas(key) } }
      })
      const view = render(<App engine={engine(request)} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: storedOnlyFaceCanvas(key) }} />)
      // THE POSITIVE SETTLE CONDITION, so what follows is read on a chain that
      // has actually reached the machine registration rather than on a race:
      // the face is in the page's font set, under the family derived from the
      // content address, and exactly once.
      await waitFor(() => expect(fontSet.added).toEqual([embeddedFaceFamily(key)]))
      // AND IT REACHED THE CANVAS. This is the assertion the mutation reds: with
      // `carriedFaces` passed instead of `paintableFaces` the fragment keeps the
      // stylesheet's declared stack and this stays `''` forever.
      const painted = () => Array.from(view.container.querySelectorAll('.canvas-text-fragment')) as HTMLElement[]
      expect(painted().length).toBe(1)
      await waitFor(() => expect(painted()[0]!.style.fontFamily, 'a face held only by the machine store must still reach the canvas').toBe(embeddedFaceFamily(key)))
      // THE DOCUMENT CARRIED NOTHING, which is what makes the line above a
      // claim about the machine registration and not about the document one.
      // The document-scoped effect asks the ENGINE for bytes; it was never
      // asked, because the document declares no carried entry.
      expect(assetRequests, 'the document carries no face, so the engine must never be asked for one').toEqual([])
    } finally {
      fontSet.restore()
    }
  })

  // MATRIX ROW 1 (Story 16.6): "Panel with nothing stored | Empty store |
  // Typography panel renders no store section at all". The three-faces guard
  // below proves the POPULATED case only — it never mounts with an empty
  // store, so it says nothing about this row. And this is the row that
  // matters most: under the deleted code the EMPTY state rendered THE MOST, a
  // heading plus two full paragraphs and no controls at all, which is exactly
  // what the owner was looking at when they asked for the deletion. A
  // regression that restores only `faces.length === 0 ? <emptyBranch> : null`
  // would sail straight past the three-faces guard, because that guard cannot
  // see a branch it never renders under.
  it('renders no store section with nothing stored either', async () => {
    globalThis.fetch = vi.fn(async () => { throw new TypeError('Failed to fetch') }) as never
    mount(commandRequest())
    // THE STORE REALLY IS EMPTY — confirmed past the designer rather than
    // merely assumed from a fresh fixture, since `beforeEach` gives every test
    // its own IndexedDB factory.
    const opened = await openFontStore(globalThis.indexedDB)
    if (!opened.ok) throw new Error(opened.reason)
    const listed = await opened.value.list()
    expect(listed.ok && listed.value).toEqual([])

    // NO STORE SECTION ANYWHERE — the named region, its heading, its own
    // empty-state sentence and its CSS class, checked separately so a partial
    // revival of only one of them still reds this.
    expect(screen.queryByRole('group', { name: 'Typefaces downloaded to this machine' })).not.toBeInTheDocument()
    expect(screen.queryByText('TYPEFACES THIS DESIGNER HAS DOWNLOADED')).not.toBeInTheDocument()
    expect(screen.queryByText(/No typefaces have been downloaded to this machine yet/)).not.toBeInTheDocument()
    expect(document.querySelector('.machine-font-store')).toBeNull()
  })

  // THE DELETION'S OWN GUARD (Story 16.6). This story's Verification section
  // says it plainly: "proving a section is gone by observing a green suite
  // proves nothing; assert its absence with faces present." So this drives
  // three stored faces through the real store and mounts the whole designer,
  // then asserts the named region, its heading and its remove controls are
  // ALL absent — none of `MachineFontStore`'s three renders, checked
  // separately, so a partial revival (the heading returns but not the remove
  // button, say) still reds this.
  it('renders no store section with three faces present, and offers no way to remove one', async () => {
    const opened = await openFontStore(globalThis.indexedDB)
    if (!opened.ok) throw new Error(opened.reason)
    const fixtures = ['Alpha Machine Face', 'Beta Machine Face', 'Gamma Machine Face']
    for (const family of fixtures) {
      const bytes = sfntWithNames([{ platform: 3, nameID: 0, value: `Copyright 2026 ${family}` }])
      const written = await seedInstalled(opened.value, { ...storedOnly(await storedFaceKey(bytes), bytes), family })
      expect(written.ok, `the fixture face for ${family} must reach the store`).toBe(true)
    }
    globalThis.fetch = vi.fn(async () => { throw new TypeError('Failed to fetch') }) as never
    mount(commandRequest())
    // THE FACES REALLY LANDED IN `storedFaces` — the settle condition, proven
    // through the dropdown rather than the deleted panel.
    fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
    await waitFor(() => expect(within(screen.getByRole('group', { name: 'AVAILABLE LOCALLY' })).getAllByRole('option')).toHaveLength(catalogueFamilyCount + fixtures.length))

    // NO STORE SECTION ANYWHERE, in a typography panel that has three faces to
    // draw if it were still drawing them.
    expect(screen.queryByRole('group', { name: 'Typefaces downloaded to this machine' })).not.toBeInTheDocument()
    expect(screen.queryByText('TYPEFACES THIS DESIGNER HAS DOWNLOADED')).not.toBeInTheDocument()
    expect(document.querySelector('.machine-font-store')).toBeNull()
    expect(screen.queryAllByRole('button', { name: /^Remove .* from this machine$/ })).toEqual([])

    // AND THE THREE FACES ARE STILL OFFERED — the deletion costs no
    // discoverability, because `AVAILABLE LOCALLY` is the family control's own
    // list and was never the deleted panel's to begin with.
    const local = within(screen.getByRole('group', { name: 'AVAILABLE LOCALLY' })).getAllByRole('option')
    for (const family of fixtures) expect(local.map(optionText)).toContain(family)
  })
})

// RETIRED (Story 16.9): this describe block, 'a pick that stalls rather
// than failing', covered the 30s/180s stall hazard (D-16.R.14/.15) for a
// pick made from the dropdown's removed `AVAILABLE TO INSTALL` group. That
// hazard is DISCHARGED by this story, not merely untested: the group whose
// pick could reach `fetchWebFamily` (and therefore stall against the
// declared repository host) no longer exists in this control, so no
// interaction with this dropdown can reach that fetch at all. The
// underlying stall/timeout mechanism (`timedFetcher`, `fetchTimeoutMs`) is
// still unit-tested in `font-source.test.ts`, independent of any UI, and
// `App.test.tsx` now carries a fetch-spy assertion that opening and
// filtering this dropdown reaches no third-party host at all.

// STORY 16.7 — EVERY ROW SHOWS THE TYPEFACE IT NAMES, ONE TEST PER MATRIX ROW.
//
// Driven through the mounted designer, never through a unit resolver: the
// claim is that the FAMILY CONTROL's own dropdown draws a specimen, which is a
// fact about `App.tsx` wiring three modules together (the registry, the
// bytes reader, and the declared-chain resolution) and not a fact any one of
// them can prove alone. `getByRole`'s accessible-name matching is used
// throughout precisely because it is what excludes the `aria-hidden`
// specimen — the same computation a screen reader performs.
describe('Story 16.7 — every row shows the typeface it names', () => {
  it('matrix: local-tier row — sets Arimo\'s specimen in Arimo itself, right of the name', async () => {
    const arimo = catalogueFaces.find((entry) => entry.family === 'Arimo')
    if (!arimo) throw new Error('Arimo is missing from the local-tier fixture')
    const bytes = new Uint8Array([0x00, 0x01, 0x00, 0x00, 0x7f]).buffer
    const fontSet = installStubFontSet()
    globalThis.fetch = vi.fn(async (url: string) => url === arimo.url ? { ok: true, arrayBuffer: async () => bytes } : { ok: false, status: 404, text: async () => '' }) as never
    try {
      mount(commandRequest())
      fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
      const option = await screen.findByRole('option', { name: 'Arimo' })
      await waitFor(() => expect(option.querySelector('.property-option-specimen')).not.toBeNull())
      const specimen = option.querySelector('.property-option-specimen') as HTMLElement
      expect(specimen.getAttribute('aria-hidden')).toBe('true')
      expect(specimen.style.fontFamily).toBe(previewFaceFamily('Arimo'))
    } finally {
      fontSet.restore()
    }
  })

  it('matrix: Thai-covering row — the sample is Thai and lang is th, for a face whose scripts include thai', async () => {
    const family = 'Noto Sans Thai Looped'
    const thaiFace = catalogueFaces.find((entry) => entry.family === family)
    if (!thaiFace) throw new Error(`${family} is missing from the local-tier fixture`)
    expect(thaiFace.scripts, 'the fixture must really cover Thai, or this measures nothing').toContain('thai')
    const bytes = new Uint8Array([0x00, 0x01, 0x00, 0x00, 0x7f]).buffer
    const fontSet = installStubFontSet()
    globalThis.fetch = vi.fn(async (url: string) => url === thaiFace.url ? { ok: true, arrayBuffer: async () => bytes } : { ok: false, status: 404, text: async () => '' }) as never
    try {
      mount(commandRequest())
      const combobox = screen.getByRole('combobox', { name: 'Font family' })
      fireEvent.focus(combobox)
      fireEvent.change(combobox, { target: { value: family } })
      const option = await screen.findByRole('option', { name: family })
      await waitFor(() => expect(option.querySelector('.property-option-specimen')).not.toBeNull())
      const specimen = option.querySelector('.property-option-specimen') as HTMLElement
      expect(specimen.getAttribute('lang')).toBe('th')
      expect(specimen.textContent).toBe(familyControlThaiSample)
    } finally {
      fontSet.restore()
    }
  })

  it('matrix: stored row — the specimen is set from the store, with no network at all', async () => {
    const family = 'A Stored Specimen Face'
    const bytes = sfntWithNames([{ platform: 3, nameID: 0, value: `Copyright 2026 ${family}` }])
    const key = await storedFaceKey(bytes)
    const opened = await openFontStore(globalThis.indexedDB)
    if (!opened.ok) throw new Error(opened.reason)
    const written = await seedInstalled(opened.value, { ...storedOnly(key, bytes), family })
    expect(written.ok, 'the fixture face must really be in the store before the designer opens it').toBe(true)
    const fontSet = installStubFontSet()
    const fetchSpy = vi.fn(async () => { throw new TypeError('Failed to fetch') })
    globalThis.fetch = fetchSpy as never
    try {
      mount(commandRequest())
      await waitForStoredFamily(family)
      const combobox = screen.getByRole('combobox', { name: 'Font family' })
      // NARROWED IN THE SAME EVENT THE DROPDOWN OPENS WITH, never focused
      // first: an empty-query open would also show every `AVAILABLE LOCALLY`
      // row, each costing a real fetch of its own local URL that has nothing
      // to do with the one claim this test makes.
      fireEvent.change(combobox, { target: { value: family } })
      const option = await screen.findByRole('option', { name: family })
      await waitFor(() => expect(option.querySelector('.property-option-specimen')).not.toBeNull())
      const specimen = option.querySelector('.property-option-specimen') as HTMLElement
      expect(specimen.style.fontFamily).toBe(previewFaceFamily(family))
      expect(fetchSpy, 'a stored face must cost no network at all').not.toHaveBeenCalled()
    } finally {
      fontSet.restore()
    }
  })

  // RETITLED AND NARROWED BY STORY 16.9: the matrix row this used to witness —
  // "a web-tier row draws no specimen" — no longer has a row to be true of.
  // The dropdown offers no web-tier rows at all now, so the stronger and now
  // correct claim is that the family is not offered here, period, never mind
  // its specimen.
  it('matrix: a family not on this machine is not offered by this dropdown at all', async () => {
    const family = webFamilies[0]?.family
    if (family === undefined) throw new Error('the web tier fixture is empty')
    const fetchSpy = vi.fn(async () => { throw new TypeError('Failed to fetch') })
    globalThis.fetch = fetchSpy as never
    const fontSet = installStubFontSet()
    try {
      mount(commandRequest())
      const combobox = screen.getByRole('combobox', { name: 'Font family' })
      fireEvent.change(combobox, { target: { value: family } })
      // NO OPTION NAMES IT, under any heading, at all.
      expect(screen.queryByRole('option', { name: new RegExp(`^${family.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}`) })).not.toBeInTheDocument()
      expect(screen.getByText(`Nothing in this template or on this machine matches "${family}".`)).toBeInTheDocument()
      expect(fetchSpy, 'a family not on this machine must never be fetched to draw a menu').not.toHaveBeenCalled()
    } finally {
      fontSet.restore()
    }
  })

  it('matrix: bytes unreadable — no specimen, never a substitute face, and the row stays pickable', async () => {
    const family = 'An Unreadable Stored Face'
    const bytes = sfntWithNames([{ platform: 3, nameID: 0, value: `Copyright 2026 ${family}` }])
    const key = await storedFaceKey(bytes)
    const opened = await openFontStore(globalThis.indexedDB)
    if (!opened.ok) throw new Error(opened.reason)
    const written = await seedInstalled(opened.value, { ...storedOnly(key, bytes), family })
    expect(written.ok, 'the fixture face must really be in the store before the read is broken').toBe(true)
    const fontSet = installStubFontSet()
    globalThis.fetch = vi.fn(async () => { throw new TypeError('Failed to fetch') }) as never
    const originalGet = FakeIndexedDBObjectStore.prototype.get
    FakeIndexedDBObjectStore.prototype.get = function refuse(): never { throw new DOMException('the store cannot be read', 'UnknownError') }
    try {
      mount(commandRequest())
      const combobox = screen.getByRole('combobox', { name: 'Font family' })
      fireEvent.focus(combobox)
      fireEvent.change(combobox, { target: { value: family } })
      const option = await screen.findByRole('option', { name: family })
      // NEVER A SUBSTITUTE: give the declined read every chance to settle
      // before asserting the negative.
      await new Promise((resolve) => setTimeout(resolve, 0))
      expect(option.querySelector('.property-option-specimen')).toBeNull()
      // AND STILL PICKABLE — a face this control cannot preview is not a face
      // it refuses to let the author choose.
      fireEvent.click(option)
      expect(screen.queryByRole('listbox', { name: 'Fonts' }), 'the pick must have gone through, closing the dropdown').not.toBeInTheDocument()
    } finally {
      FakeIndexedDBObjectStore.prototype.get = originalGet
      fontSet.restore()
    }
  })

  it('matrix: declared chain row — the specimen is set in the face it resolves to', () => {
    mount(commandRequest())
    fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
    const templateGroup = screen.getByRole('group', { name: 'IN THIS TEMPLATE' })
    const bodyOption = within(templateGroup).getByRole('option', { name: 'body' })
    const specimen = bodyOption.querySelector('.property-option-specimen') as HTMLElement | null
    // THE FIXTURE'S `body` CHAIN IS `entries: [face('Noto Sans')]` — a shipped
    // entry, present without embedding anything — so this is the "declared
    // chain paints a shipped face" half of the matrix row.
    expect(specimen, 'a chain resolving to a shipped face must draw a specimen').not.toBeNull()
    expect(specimen!.getAttribute('aria-hidden')).toBe('true')
    // jsdom's own CSSOM reserializes a quoted family list with double quotes
    // on the way back out of `.style.fontFamily`; the quote CHARACTER is a
    // jsdom detail, never this control's claim, so both sides are compared
    // with their quoting normalised away.
    expect(specimen!.style.fontFamily.replace(/['"]/g, '')).toBe(shippedFaceFamily('Noto Sans')!.replace(/['"]/g, ''))
    expect(specimen!.textContent).toBe(familyControlLatinSample)
  })

  it('matrix: declared chain row — none when the chain resolves to no carried or shipped face', () => {
    // AN ENTRY THAT IS NEITHER: `assetKey` is not a carried key and `face` is
    // empty, so it fails both `isCarriedFaceAssetKey` and `isShippedFaceName`.
    mount(commandRequest(), [{ name: 'body', entries: [{ face: '', assetKey: '', family: '', style: '', bold: '', italic: '', boldItalic: '' }] }])
    fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
    const templateGroup = screen.getByRole('group', { name: 'IN THIS TEMPLATE' })
    const bodyOption = within(templateGroup).getByRole('option', { name: 'body' })
    expect(bodyOption.querySelector('.property-option-specimen')).toBeNull()
  })

  it('matrix: dropdown closed — every face this control registered is released', async () => {
    const arimo = catalogueFaces.find((entry) => entry.family === 'Arimo')
    if (!arimo) throw new Error('Arimo is missing from the local-tier fixture')
    const bytes = new Uint8Array([0x00, 0x01, 0x00, 0x00, 0x7f]).buffer
    // A STUB THAT TRACKS REMOVAL, unlike this file's own `installStubFontSet`
    // (whose `added` array only ever grows). Releasing on close is exactly
    // the fact under test, so this test needs to see a face LEAVE the set.
    class StubFace {
      readonly family: string
      constructor(family: string) { this.family = family }
      load(): Promise<StubFace> { return Promise.resolve(this) }
    }
    const live: StubFace[] = []
    Object.defineProperty(globalThis, 'FontFace', { value: StubFace, configurable: true, writable: true })
    Object.defineProperty(document, 'fonts', { value: { add: (loaded: StubFace) => { live.push(loaded) }, delete: (loaded: StubFace) => { const at = live.indexOf(loaded); if (at >= 0) live.splice(at, 1) } }, configurable: true, writable: true })
    globalThis.fetch = vi.fn(async (url: string) => url === arimo.url ? { ok: true, arrayBuffer: async () => bytes } : { ok: false, status: 404, text: async () => '' }) as never
    try {
      mount(commandRequest())
      const combobox = screen.getByRole('combobox', { name: 'Font family' })
      fireEvent.focus(combobox)
      await waitFor(() => expect(live.length).toBeGreaterThan(0))
      fireEvent.keyDown(combobox, { key: 'Escape' })
      expect(screen.queryByRole('listbox', { name: 'Fonts' })).not.toBeInTheDocument()
      expect(live, 'closing the dropdown must release every face it registered').toEqual([])
    } finally {
      Reflect.deleteProperty(globalThis, 'FontFace')
      Reflect.deleteProperty(document, 'fonts')
    }
  })

  it('matrix: screen reader on any row — the accessible name is the family alone, with the specimen absent from it', async () => {
    const arimo = catalogueFaces.find((entry) => entry.family === 'Arimo')
    if (!arimo) throw new Error('Arimo is missing from the local-tier fixture')
    const bytes = new Uint8Array([0x00, 0x01, 0x00, 0x00, 0x7f]).buffer
    const fontSet = installStubFontSet()
    globalThis.fetch = vi.fn(async (url: string) => url === arimo.url ? { ok: true, arrayBuffer: async () => bytes } : { ok: false, status: 404, text: async () => '' }) as never
    try {
      mount(commandRequest())
      fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
      const option = await screen.findByRole('option', { name: 'Arimo' })
      await waitFor(() => expect(option.querySelector('.property-option-specimen')).not.toBeNull())
      // STILL EXACTLY "Arimo": the specimen that just appeared inside this
      // very row is `aria-hidden` and never enters its accessible name.
      expect(screen.getByRole('option', { name: 'Arimo' })).toBe(option)
    } finally {
      fontSet.restore()
    }
  })
})

// ---------------------------------------------------------------------------
// SPEC-INSTALL-ALL-FACE-CUTS STORY 2 — EMBEDDING A CUT ON FIRST USE, AND THE
// THREE ABSENCES THE CENSUS CAN NOW TELL APART.
//
// These claims are about the DESIGNER and cannot be asserted anywhere smaller.
// "Pressing B embeds the held bold" spans the projection, the machine store and
// the command seam; "the sentence names which absence this is" spans the census
// the store holds and the panel that reads it. They live in this file because a
// real fake IndexedDB stands up here and the store is where both answers come
// from.
describe('a cut the machine holds reaches the document on first use', () => {
  // A 64-hex key, which is the shape an asset key really has — the panel
  // compares the held cut's key against this one, and a base that could not be
  // an asset key would make that comparison meaningless.
  const BASE_KEY = '1111111111111111111111111111111111111111111111111111111111111111'
  const kanitBoldBytes = sfntWithNames([{ platform: 3, nameID: 0, value: 'Copyright 2020 The Kanit Project Authors, Bold' }])
  const kanitRegularBytes = sfntWithNames([{ platform: 3, nameID: 0, value: 'Copyright 2020 The Kanit Project Authors, Regular' }])
  const kanitItalicBytes = sfntWithNames([{ platform: 3, nameID: 0, value: 'Copyright 2020 The Kanit Project Authors, Italic' }])
  const kanitBoldItalicBytes = sfntWithNames([{ platform: 3, nameID: 0, value: 'Copyright 2020 The Kanit Project Authors, Bold Italic' }])

  const base64Of = (bytes: ArrayBuffer): string => {
    let binary = ''
    for (const byte of new Uint8Array(bytes)) binary += String.fromCharCode(byte)
    return btoa(binary)
  }

  const kanitCut = (key: string, style: string, bytes: ArrayBuffer): StoredFaceRecord => ({
    key,
    family: 'Kanit',
    style,
    licence: 'OFL-1.1',
    licenceText: kanitLicence,
    copyright: 'Copyright 2020 The Kanit Project Authors',
    source: `google/fonts — ofl/kanit/Kanit-${style}.ttf, fetched 2026-09-20`,
    authorAcknowledged: false,
    mediaType: 'font/ttf',
    scripts: ['latin'],
    fetchedAt: '2026-09-20',
    byteLength: bytes.byteLength,
    bytes,
  })

  /** Writes face records and, when given one, a census — the two things an install leaves behind. */
  const seedMachine = async (records: ReadonlyArray<StoredFaceRecord>, census?: { published: ReadonlyArray<string>; refused: ReadonlyArray<{ style: string; reason: string; permanence: 'permanent' | 'transient' }> }): Promise<FontStore> => {
    const opened = await openFontStore(globalThis.indexedDB)
    if (!opened.ok) throw new Error(opened.reason)
    for (const record of records) {
      const written = await opened.value.put(record)
      expect(written.ok, 'the fixture face must really be in the store before the designer opens it').toBe(true)
    }
    if (census !== undefined) {
      const recorded = await opened.value.putCensus({ family: 'Kanit', published: census.published, refused: census.refused, recordedAt: '2026-09-20' })
      expect(recorded.ok, 'the fixture census must really be in the store').toBe(true)
    }
    return opened.value
  }

  // ⚠ A ONE-ENTRY CHAIN IS THE MINIMUM SHAPE, NOT THE SHAPE A PICK WRITES.
  // `embedFontFamily` appends `proposedFallbackTail` — the shipped faces for
  // the scripts the picked face does not cover — so a real pick's chain has
  // two or three entries and, since Story 11.4, those tail entries DECLARE
  // their own cuts. `kanitRealisticChain` below is that shape, and it is the
  // only fixture here that can tell an entry-level question from a chain-wide
  // one.
  const kanitChain = (declared: Readonly<{ bold?: string; italic?: string; boldItalic?: string }> = {}) => ({
    name: 'Kanit',
    entries: [{ face: '', assetKey: BASE_KEY, family: 'Kanit', style: 'Regular', bold: declared.bold ?? '', italic: declared.italic ?? '', boldItalic: declared.boldItalic ?? '' }],
  })

  // ⚠ THE CHAIN A PICK ACTUALLY WRITES, AND THE ONE FIXTURE THAT CAN SEE THE
  // ENTRY-VERSUS-CHAIN DEFECT.
  //
  // `Noto Sans Thai` is a shipped face that DECLARES A BOLD, and every pick
  // whose face does not cover Thai proposes it behind the embedded entry. So a
  // plan built on "does the CHAIN declare this cut" answers yes for almost
  // every catalogue family, never builds a plan, and pressing B embeds nothing
  // and says nothing — while I, for which no tail entry declares a cut, goes on
  // working. On a one-entry chain the two rules return the same answer, and an
  // assertion whose two sides could be equal is not an assertion (D-11.2.8).
  const kanitRealisticChain = () => ({
    name: 'Kanit',
    entries: [
      { face: '', assetKey: BASE_KEY, family: 'Kanit', style: 'Regular', bold: '', italic: '', boldItalic: '' },
      { face: 'Noto Sans Thai', assetKey: '', family: '', style: '', bold: 'Noto Sans Thai Bold', italic: '', boldItalic: '' },
      { face: 'Noto Sans SC', assetKey: '', family: '', style: '', bold: '', italic: '', boldItalic: '' },
    ],
  })
  const kanitText = { ...textComponent, fontFamily: 'Kanit' }

  const mountKanit = (request: unknown, chains = [kanitChain()]) => {
    const componentCanvas = { ...canvas, fontFamilies: chains.map((chain) => chain.name), fontChains: chains, components: [kanitText] }
    render(<App engine={engine(request)} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    fireEvent.click(screen.getByLabelText(/^text component e1/))
  }

  // The asset prefetch is answered so the base key's request does not reject
  // into the console; the payloads this file reads are the `command` ones.
  const cutRequest = (chains = [kanitChain()]) => vi.fn(async (operation: string) => {
    if (operation === 'asset') return { snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: { ...canvas, fontFamilies: chains.map((chain) => chain.name), fontChains: chains, components: [kanitText] } }, bytes: kanitRegularBytes }
    return { snapshot: { documentState: 'loaded' as const, revision: operation === 'command' ? 2 : 1, byteLength: 3, canvas: { ...canvas, fontFamilies: chains.map((chain) => chain.name), fontChains: chains, components: [kanitText] } } }
  })

  const NO_BOLD = 'No bold face in this family — the engine paints the regular face and warns.'
  const UNFETCHED_BOLD = 'This family has a bold face, but it is not on this machine — the engine paints the regular face and warns. Add the family again to fetch it.'
  const UNUSABLE_BOLD = 'This family has a bold face this designer cannot use — the engine paints the regular face and warns. Trying again will not help.'
  // D-owner-4's fourth state. It claims NOTHING about upstream in either
  // direction and names the one action that changes what this designer knows.
  const UNCHECKED_BOLD = 'This designer has not checked whether this family has a bold face — the engine paints the regular face and warns. Add the family again to find out.'

  // The three handles Story 11.3 established, unchanged: the class jsdom can
  // see, the one announcement path, and the visible wording it points at.
  const expectCut = (label: 'Bold' | 'Italic', sentence: string | undefined) => {
    const control = screen.getByRole('button', { name: label })
    if (sentence === undefined) {
      expect(control.className, `${label} must be the plain control`).not.toContain('property-toggle-unavailable')
      expect(control.getAttribute('aria-describedby'), `${label} must describe no absence`).toBeNull()
      return
    }
    expect(control.className, `${label} must be in the unavailable state`).toContain('property-toggle-unavailable')
    expect(control).not.toBeDisabled()
    const described = control.getAttribute('aria-describedby')
    expect(described, `${label} must point at the reason`).not.toBeNull()
    expect(document.getElementById(described!)).toHaveTextContent(sentence)
  }

  it('says nothing about a bold the machine holds and the document has not embedded yet', async () => {
    const boldKey = await storedFaceKey(kanitBoldBytes)
    await seedMachine([kanitCut(boldKey, 'Bold', kanitBoldBytes), kanitCut(await storedFaceKey(kanitRegularBytes), 'Regular', kanitRegularBytes)], { published: ['Regular', 'Bold'], refused: [] })
    mountKanit(cutRequest())

    // THE POSITIVE CONTROL IS THE STARTING STATE, and it doubles as this test's
    // settle condition: before the store has been listed the panel knows of no
    // held bold and correctly says so, so waiting for the sentence to GO is
    // waiting for the listing to arrive rather than for a render that never
    // had it.
    expect(screen.getByText(NO_BOLD)).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByText(NO_BOLD)).toBeNull())
    expectCut('Bold', undefined)
    // AND THE ITALIC BESIDE IT STILL WARNS: this machine holds no italic, so the
    // two controls must disagree. Without it this test would also pass over a
    // change that simply stopped reporting absences.
    expectCut('Italic', 'No italic face in this family — the engine paints the regular face and warns.')
  })

  it('sends the cut embed and the property commit as ONE unit, in that order', async () => {
    const boldKey = await storedFaceKey(kanitBoldBytes)
    await seedMachine([kanitCut(boldKey, 'Bold', kanitBoldBytes)], { published: ['Regular', 'Bold'], refused: [] })
    const request = cutRequest()
    mountKanit(request)
    await waitFor(() => expect(screen.queryByText(NO_BOLD)).toBeNull())

    fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
    await waitFor(() => expect(embedPayloads(request)).toHaveLength(1))

    // ONE COMMAND REACHED THE ENGINE, which is what one revision and one undo
    // entry rest on: `wasm.Engine.Apply` pushes one undo per accepted command,
    // so two commands here would be two undo steps whatever the engine did.
    const unit = embedPayloads(request)[0]!
    expect(unit.kind).toBe('applyCommands')
    const members = unit.commands as ReadonlyArray<Record<string, unknown>>
    expect(members).toHaveLength(2)

    // THE EMBED COMES FIRST, AND THE ORDER IS FORCED BY THE ENGINE: an entry may
    // only declare a cut the document actually carries, and a unit applies its
    // members in order against one document.
    expect(members[0]).toEqual({
      kind: 'embedFontCut', version: 1,
      name: 'Kanit', index: 0, cut: 'bold',
      family: 'Kanit', style: 'Bold',
      licence: 'OFL-1.1', licenceText: kanitLicence,
      copyright: 'Copyright 2020 The Kanit Project Authors',
      source: 'google/fonts — ofl/kanit/Kanit-Bold.ttf, fetched 2026-09-20',
      authorAcknowledged: false,
      mediaType: 'font/ttf',
      // THE BYTES ARE THE HELD ONES, read out of the store rather than fetched.
      data: base64Of(kanitBoldBytes),
    })
    expect(members[1]).toEqual({ kind: 'updateComponentProperties', version: 1, ids: ['e1'], changes: { bold: { op: 'set', value: true } } })
  })

  it('sends the property alone when the document already declares the cut', async () => {
    const boldKey = await storedFaceKey(kanitBoldBytes)
    await seedMachine([kanitCut(boldKey, 'Bold', kanitBoldBytes)], { published: ['Regular', 'Bold'], refused: [] })
    const declared = [kanitChain({ bold: boldKey })]
    const request = cutRequest(declared)
    mountKanit(request, declared)

    fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
    await waitFor(() => expect(embedPayloads(request)).toHaveLength(1))
    // NO UNIT AND NO SECOND COPY OF THE FACE. The engine's idempotent no-op is
    // the backstop for this, not the plan: a document must not gain bytes it
    // already carries because a toggle was pressed twice.
    expect(embedPayloads(request)[0]!.kind).toBe('updateComponentProperties')
  })

  it('sends no embed when clearing a toggle', async () => {
    const boldKey = await storedFaceKey(kanitBoldBytes)
    await seedMachine([kanitCut(boldKey, 'Bold', kanitBoldBytes)], { published: ['Regular', 'Bold'], refused: [] })
    const bolded = [kanitChain()]
    const request = cutRequest(bolded)
    const componentCanvas = { ...canvas, fontFamilies: ['Kanit'], fontChains: bolded, components: [{ ...kanitText, bold: true }] }
    render(<App engine={engine(request)} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    fireEvent.click(screen.getByLabelText(/^text component e1/))
    await waitFor(() => expect(screen.queryByText(NO_BOLD)).toBeNull())

    // Pressing a pressed B CLEARS the property. A document must never gain a
    // face because something was switched off.
    fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
    await waitFor(() => expect(embedPayloads(request)).toHaveLength(1))
    expect(embedPayloads(request)[0]).toEqual({ kind: 'updateComponentProperties', version: 1, ids: ['e1'], changes: { bold: { op: 'clear' } } })
  })

  it('refuses at the control, and sends nothing, when the held face has gone from the store', async () => {
    const boldKey = await storedFaceKey(kanitBoldBytes)
    const store = await seedMachine([kanitCut(boldKey, 'Bold', kanitBoldBytes)], { published: ['Regular', 'Bold'], refused: [] })
    const request = cutRequest()
    mountKanit(request)
    await waitFor(() => expect(screen.queryByText(NO_BOLD)).toBeNull())

    // THE STORE SELF-HEALS BY DROPPING, so a listing row whose bytes are gone is
    // an ordinary condition rather than a hypothetical — this is that state,
    // produced past the designer.
    expect((await store.remove(boldKey)).ok).toBe(true)

    fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
    expect(await screen.findByText(/Kanit bold is not on this machine any more/)).toBeInTheDocument()
    // AND THE PROPERTY DID NOT COMMIT EITHER. The embed and the commit are one
    // unit; half of one is exactly what a unit exists to make unreachable.
    expect(embedPayloads(request)).toHaveLength(0)
  })

  it('keeps the existing sentence, word for word, when upstream publishes no bold', async () => {
    // THE ITALIC IS THE SETTLE CONDITION AND THE DISCRIMINATOR AT ONCE. Before
    // the store has been listed the panel knows of no held cut and says so about
    // BOTH; waiting for the italic sentence to go is waiting for the listing,
    // and the bold sentence surviving that wait is the measurement. Without the
    // second cut this test would pass over a panel that had read nothing.
    const italicKey = await storedFaceKey(kanitItalicBytes)
    await seedMachine(
      [kanitCut(await storedFaceKey(kanitRegularBytes), 'Regular', kanitRegularBytes), kanitCut(italicKey, 'Italic', kanitItalicBytes)],
      { published: ['Regular', 'Italic'], refused: [] },
    )
    mountKanit(cutRequest())
    await waitFor(() => expectCut('Italic', undefined))
    // The census is the authority on what the family publishes and it names no
    // bold, so the sentence that names the family is the true one — and it is
    // asserted as the EXACT string, because "with its words unchanged" is the
    // acceptance criterion.
    expectCut('Bold', NO_BOLD)
    expect(screen.getByText(NO_BOLD)).toBeInTheDocument()
  })

  it('names a bold this machine could not fetch, and offers the re-pick as the way out', async () => {
    await seedMachine([kanitCut(await storedFaceKey(kanitRegularBytes), 'Regular', kanitRegularBytes)], {
      published: ['Regular', 'Bold'],
      refused: [{ style: 'Bold', reason: 'the request timed out', permanence: 'transient' }],
    })
    mountKanit(cutRequest())
    await waitFor(() => expectCut('Bold', UNFETCHED_BOLD))
    // THE SENTENCE CHANGED ITS SUBJECT AND THE OLD ONE IS GONE: the family is
    // not the one at fault here, and saying it is was the falsehood D-owner-3
    // exists to remove.
    expect(screen.queryByText(NO_BOLD)).toBeNull()
  })

  it('names a bold this engine cannot use, and offers no retry', async () => {
    await seedMachine([kanitCut(await storedFaceKey(kanitRegularBytes), 'Regular', kanitRegularBytes)], {
      published: ['Regular', 'Bold'],
      refused: [{ style: 'Bold', reason: 'the face carries a variable axis', permanence: 'permanent' }],
    })
    mountKanit(cutRequest())
    // PERMANENCE, NEVER PROSE, DECIDES. The two refusals above and here differ
    // in exactly one field, and the sentences they produce differ in what they
    // offer the author to do next.
    await waitFor(() => expectCut('Bold', UNUSABLE_BOLD))
    expect(screen.queryByText(UNFETCHED_BOLD)).toBeNull()
    expect(screen.queryByText(NO_BOLD)).toBeNull()
  })

  // D-owner-4, AND IT IS A FALSEHOOD REMOVED RATHER THAN A STATE ADDED.
  //
  // Three routes reach the panel with face records and no census row: the v1 to
  // v2 store migration is purely additive and writes none, a failed `putCensus`
  // leaves the faces written and the row absent, and `installFamily`'s
  // partial-write path refuses and returns BEFORE `recordFamilyCensus`. Reading
  // any of them as "not fetched" renders *"This family HAS a bold face"* — a
  // claim about UPSTREAM from a designer that has asked upstream nothing.
  it('says it has not checked when there are face records and no census row', async () => {
    await seedMachine([kanitCut(await storedFaceKey(kanitRegularBytes), 'Regular', kanitRegularBytes)])
    mountKanit(cutRequest())
    await waitFor(() => expectCut('Bold', UNCHECKED_BOLD))
    // NEITHER OF THE TWO CLAIMS IS MADE. The fourth sentence exists because
    // both of these would have been inventions: one says the family has the
    // cut, the other says it has none.
    expect(screen.queryByText(UNFETCHED_BOLD)).toBeNull()
    expect(screen.queryByText(NO_BOLD)).toBeNull()
  })

  // THE OTHER SIDE OF THE SAME BRANCH, AND IT IS A DIFFERENT FACT. No census
  // AND no face records is a family the store has never heard of — the
  // committed catalogue tier, whose faces are upstream files committed to this
  // repository byte for byte and whose absences are therefore genuine (settled
  // fork 4), or a chain from nowhere this designer knows. Today's sentence is
  // the weakest claim available and stays.
  it('keeps today\'s sentence for a family the store has never heard of', async () => {
    await seedMachine([kanitCut(await storedFaceKey(kanitRegularBytes), 'Regular', kanitRegularBytes)], { published: ['Regular', 'Bold'], refused: [] })
    // A DIFFERENT FAMILY IS IN THE STORE, which is what makes this a
    // measurement: the listing really has arrived, and it carries nothing for
    // the family the component names.
    const strangerChain = { name: 'Stranger', entries: [{ face: '', assetKey: BASE_KEY, family: 'Stranger', style: 'Regular', bold: '', italic: '', boldItalic: '' }] }
    const stranger = { ...textComponent, fontFamily: 'Stranger' }
    const componentCanvas = { ...canvas, fontFamilies: ['Stranger'], fontChains: [strangerChain], components: [stranger] }
    render(<App engine={engine(cutRequest())} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    fireEvent.click(screen.getByLabelText(/^text component e1/))
    await waitFor(async () => expect((await faceRecordsOnThisMachine()).map((record) => record.family)).toContain('Kanit'))
    expectCut('Bold', NO_BOLD)
    expect(screen.queryByText(UNCHECKED_BOLD)).toBeNull()
  })

  it('reads a bold whose bytes ARE the regular as genuine absence', async () => {
    // Upstream publishing a Bold byte-identical to the Regular: the engine
    // refuses the self-reference (D-11.2.11), the designer never sends it, and
    // a bold identical to the regular is no bold — so the sentence that names
    // the family is the true one.
    //
    // ⚠ THE HELD ITALIC IS THE POSITIVE CONTROL, and without it this test
    // cannot pass for the right reason: NO_BOLD is also the state before the
    // listing arrives, so settling on the store alone would let a panel that
    // never consults it go green. The italic sentence going away is the panel
    // reading the store; the bold sentence surviving that is the measurement.
    await seedMachine(
      [kanitCut(BASE_KEY, 'Bold', kanitBoldBytes), kanitCut(await storedFaceKey(kanitItalicBytes), 'Italic', kanitItalicBytes)],
      { published: ['Regular', 'Bold', 'Italic'], refused: [] },
    )
    mountKanit(cutRequest())
    await waitFor(() => expectCut('Italic', undefined))
    expectCut('Bold', NO_BOLD)
  })

  // ⚠ THE CHAIN A REAL PICK WRITES, AND THE ONE CASE THAT CAN SEE THE
  // ENTRY-VERSUS-CHAIN DEFECT.
  //
  // `proposedFallbackTail` puts `Noto Sans Thai` behind every embedded face
  // whose scripts exclude Thai, and that entry DECLARES A BOLD. A plan built on
  // "does the CHAIN declare this cut" therefore answers yes for most of the
  // catalogue, builds nothing, and leaves B doing nothing and saying nothing
  // while I still works. Every other fixture in this file is a one-entry chain,
  // on which the entry rule and the chain rule agree — so this case is the
  // whole coverage for the distinction.
  it('embeds the base entry\'s bold even when a FALLBACK entry already declares one', async () => {
    const boldKey = await storedFaceKey(kanitBoldBytes)
    // THE HELD ITALIC IS THE SETTLE CONDITION. No entry of this chain declares
    // an italic, so I warns until the listing lands and then goes plain —
    // whereas B never warns here at all (the fallback declares a bold), so the
    // bold sentence cannot be waited on.
    await seedMachine(
      [kanitCut(boldKey, 'Bold', kanitBoldBytes), kanitCut(await storedFaceKey(kanitItalicBytes), 'Italic', kanitItalicBytes)],
      { published: ['Regular', 'Bold', 'Italic'], refused: [] },
    )
    const realistic = [kanitRealisticChain()]
    // THE FIXTURE'S DISCRIMINATING POWER IS PINNED BEFORE IT IS USED: the base
    // entry declares no bold and a LATER entry does, so the two rules genuinely
    // disagree on it.
    expect(realistic[0]!.entries[0]!.bold, 'the BASE entry must declare no bold').toBe('')
    expect(realistic[0]!.entries.some((entry) => entry.bold.length > 0), 'a TAIL entry must declare one').toBe(true)

    const request = cutRequest(realistic)
    mountKanit(request, realistic)
    await waitFor(() => expectCut('Italic', undefined))
    fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
    await waitFor(() => expect(embedPayloads(request)).toHaveLength(1))

    const unit = embedPayloads(request)[0]!
    expect(unit.kind, 'the press must still assemble a unit: the fallback\'s bold is not the base entry\'s').toBe('applyCommands')
    const members = unit.commands as ReadonlyArray<Record<string, unknown>>
    expect(members).toHaveLength(2)
    expect(members[0]).toMatchObject({ kind: 'embedFontCut', name: 'Kanit', index: 0, cut: 'bold', data: base64Of(kanitBoldBytes) })
  })

  // THE THIRD CUT, WHOSE RIBBI SPELLING IS THE ONE THAT CAN GO WRONG SILENTLY.
  // `'Bold Italic'` carries a space and a second capital; mis-spelling it in
  // `RIBBI_CUT_NAMES` resolves to no stored record at all, so the combined cut
  // simply stops embedding while every `bold` and `italic` case stays green.
  it('embeds the BOLD ITALIC cut for an already-italic component', async () => {
    const key = await storedFaceKey(kanitBoldItalicBytes)
    await seedMachine([kanitCut(key, 'Bold Italic', kanitBoldItalicBytes)], { published: ['Regular', 'Bold Italic'], refused: [] })
    const chains = [kanitChain()]
    const request = cutRequest(chains)
    const componentCanvas = { ...canvas, fontFamilies: ['Kanit'], fontChains: chains, components: [{ ...kanitText, italic: true }] }
    render(<App engine={engine(request)} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    fireEvent.click(screen.getByLabelText(/^text component e1/))
    // Until the listing lands the panel knows of no held combined cut and says
    // so; the sentence going away is that listing arriving.
    await waitFor(() => expectCut('Bold', undefined))

    // B ON AN ALREADY-ITALIC ELEMENT ASKS THE CHAIN FOR `boldItalic`, not for
    // `bold` — the cut is the one the RESULTING combination needs.
    fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
    await waitFor(() => expect(embedPayloads(request)).toHaveLength(1))
    const members = embedPayloads(request)[0]!.commands as ReadonlyArray<Record<string, unknown>>
    expect(members[0]).toMatchObject({ kind: 'embedFontCut', cut: 'boldItalic', style: 'Bold Italic', data: base64Of(kanitBoldItalicBytes) })
  })

  // THE COMBINED CUT KEEPS ITS STATED WAY OUT IN EVERY STATE. Returning early
  // for the new states skipped the `boldItalic` branch, so the one cut whose
  // exit is "turn off either one" lost it in exactly the three states that were
  // added — and a state with no stated exit is the grey-out DESIGN.md forbids.
  it('states the way out of the combined cut in every absence state', async () => {
    await seedMachine([kanitCut(await storedFaceKey(kanitRegularBytes), 'Regular', kanitRegularBytes)], {
      published: ['Regular', 'Bold Italic'],
      refused: [{ style: 'Bold Italic', reason: 'the request timed out', permanence: 'transient' }],
    })
    const chains = [kanitChain()]
    const request = cutRequest(chains)
    // BOTH FLAGS ON, so BOTH controls ask the chain for the combined cut and
    // the "once for the pair" rule is what is being read. With only one flag
    // set the other control asks for its own axis and the pair would not be
    // implicated at all.
    const componentCanvas = { ...canvas, fontFamilies: ['Kanit'], fontChains: chains, components: [{ ...kanitText, bold: true, italic: true }] }
    render(<App engine={engine(request)} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    fireEvent.click(screen.getByLabelText(/^text component e1/))

    const combined = 'This family has a bold italic face, but it is not on this machine — the engine paints the regular face and warns. Add the family again to fetch it. Turn off either one.'
    await waitFor(() => expectCut('Bold', combined))
    // STATED ONCE FOR THE PAIR, through one announcement path, exactly as the
    // `unpublished` combined sentence is.
    expectCut('Italic', combined)
    expect(screen.getAllByText(combined)).toHaveLength(1)
  })

  // `a bold` BUT `an italic`. The article is derived from the cut's name, and
  // nothing else in this file reads an italic-cut sentence in a new state — so
  // without this the three sentences could say "a italic face" for ever.
  it('says "an italic face", not "a italic face"', async () => {
    await seedMachine([kanitCut(await storedFaceKey(kanitRegularBytes), 'Regular', kanitRegularBytes)], {
      published: ['Regular', 'Italic'],
      refused: [{ style: 'Italic', reason: 'the origin refused the request', permanence: 'permanent' }],
    })
    mountKanit(cutRequest())
    const sentence = 'This family has an italic face this designer cannot use — the engine paints the regular face and warns. Trying again will not help.'
    await waitFor(() => expectCut('Italic', sentence))
    expect(screen.queryByText(/has a italic/)).toBeNull()
  })

  // TWO FAMILIES IN ONE SELECTION PRODUCE TWO EMBED MEMBERS AND ONE PROPERTY
  // COMMAND. `firstUseCutPlans` keys its map on the (cut, chain) PAIR, and with
  // every other case selecting one component that key is inert: a map keyed on
  // the cut alone would collapse these two into one and the second family would
  // be set bold over a face the document does not carry.
  it('carries one embed member per family and one property command, in that order', async () => {
    const kanitBoldKey = await storedFaceKey(kanitBoldBytes)
    const sarabunBoldBytes = sfntWithNames([{ platform: 3, nameID: 0, value: 'Copyright 2018 The Sarabun Project Authors, Bold' }])
    const sarabunBoldKey = await storedFaceKey(sarabunBoldBytes)
    const opened = await openFontStore(globalThis.indexedDB)
    if (!opened.ok) throw new Error(opened.reason)
    expect((await opened.value.put(kanitCut(kanitBoldKey, 'Bold', kanitBoldBytes))).ok).toBe(true)
    expect((await opened.value.put({ ...kanitCut(sarabunBoldKey, 'Bold', sarabunBoldBytes), family: 'Sarabun' })).ok).toBe(true)
    expect((await opened.value.putCensus({ family: 'Kanit', published: ['Regular', 'Bold'], refused: [], recordedAt: '2026-09-20' })).ok).toBe(true)
    expect((await opened.value.putCensus({ family: 'Sarabun', published: ['Regular', 'Bold'], refused: [], recordedAt: '2026-09-20' })).ok).toBe(true)

    const SARABUN_KEY = '2222222222222222222222222222222222222222222222222222222222222222'
    const chains = [
      kanitChain(),
      { name: 'Sarabun', entries: [{ face: '', assetKey: SARABUN_KEY, family: 'Sarabun', style: 'Regular', bold: '', italic: '', boldItalic: '' }] },
    ]
    const second = { ...textComponent, id: 'e2', y: 30_000, fontFamily: 'Sarabun' }
    const componentCanvas = { ...canvas, fontFamilies: ['Kanit', 'Sarabun'], fontChains: chains, components: [kanitText, second] }
    const request = vi.fn(async (operation: string) => ({ snapshot: { documentState: 'loaded' as const, revision: operation === 'command' ? 2 : 1, byteLength: 3, canvas: componentCanvas }, bytes: kanitRegularBytes }))
    render(<App engine={engine(request)} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    fireEvent.click(screen.getByLabelText(/^text component e1/))
    fireEvent.click(screen.getByLabelText(/^text component e2/), { shiftKey: true })
    await waitFor(() => expect(screen.queryByText(NO_BOLD)).toBeNull())

    fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
    await waitFor(() => expect(embedPayloads(request)).toHaveLength(1))
    const members = embedPayloads(request)[0]!.commands as ReadonlyArray<Record<string, unknown>>
    expect(members).toHaveLength(3)
    // THE EMBEDS FIRST AND THE PROPERTY LAST, because a unit applies its
    // members in order and an entry may only declare a cut the document
    // already carries.
    expect(members.slice(0, 2).map((member) => member.name)).toEqual(['Kanit', 'Sarabun'])
    expect(members[0]).toMatchObject({ kind: 'embedFontCut', cut: 'bold', data: base64Of(kanitBoldBytes) })
    expect(members[1]).toMatchObject({ kind: 'embedFontCut', cut: 'bold', data: base64Of(sarabunBoldBytes) })
    expect(members[2]).toMatchObject({ kind: 'updateComponentProperties', ids: ['e1', 'e2'] })
  })

  // TWO FAMILIES MISSING THE SAME CUT FOR DIFFERENT REASONS HAVE NO ONE TRUE
  // SENTENCE, so the panel states none — the file's existing mixed-selection
  // rule ("a selection missing two DIFFERENT cuts has no one true sentence to
  // state, so it states none") extended to the axis the census opened. With
  // every other case selecting one component, the "reasons must agree"
  // conjunct is otherwise inert.
  it('states no sentence when the selection lacks the same cut for different reasons', async () => {
    const opened = await openFontStore(globalThis.indexedDB)
    if (!opened.ok) throw new Error(opened.reason)
    expect((await opened.value.put(kanitCut(await storedFaceKey(kanitRegularBytes), 'Regular', kanitRegularBytes))).ok).toBe(true)
    expect((await opened.value.put({ ...kanitCut(await storedFaceKey(kanitItalicBytes), 'Regular', kanitItalicBytes), family: 'Sarabun' })).ok).toBe(true)
    // Kanit publishes no bold at all; Sarabun publishes one this machine could
    // not fetch. Both controls are missing `bold`, for two different reasons.
    expect((await opened.value.putCensus({ family: 'Kanit', published: ['Regular'], refused: [], recordedAt: '2026-09-20' })).ok).toBe(true)
    expect((await opened.value.putCensus({ family: 'Sarabun', published: ['Regular', 'Bold'], refused: [{ style: 'Bold', reason: 'the request timed out', permanence: 'transient' }], recordedAt: '2026-09-20' })).ok).toBe(true)

    const SARABUN_KEY = '2222222222222222222222222222222222222222222222222222222222222222'
    const chains = [
      kanitChain(),
      { name: 'Sarabun', entries: [{ face: '', assetKey: SARABUN_KEY, family: 'Sarabun', style: 'Regular', bold: '', italic: '', boldItalic: '' }] },
    ]
    const second = { ...textComponent, id: 'e2', y: 30_000, fontFamily: 'Sarabun' }
    const componentCanvas = { ...canvas, fontFamilies: ['Kanit', 'Sarabun'], fontChains: chains, components: [kanitText, second] }
    render(<App engine={engine(cutRequest())} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    fireEvent.click(screen.getByLabelText(/^text component e1/))
    fireEvent.click(screen.getByLabelText(/^text component e2/), { shiftKey: true })

    // THE SETTLE CONDITION IS THE SENTENCE GOING AWAY: before the listing
    // arrives both families read `unpublished` and AGREE, so NO_BOLD is shown.
    // It disappears once the census makes them disagree.
    await waitFor(() => expect(screen.queryByText(NO_BOLD)).toBeNull())
    expectCut('Bold', undefined)
    expect(screen.queryByText(UNFETCHED_BOLD)).toBeNull()
  })

  // THE ENGINE REFUSING A MEMBER IS THE OTHER HALF OF THE REFUSAL SURFACE, and
  // only the designer-side store miss was covered. The whole unit is refused,
  // so `bold` never commits and the document is untouched — the half-state a
  // unit exists to make unreachable — and the refusal is anchored on the
  // control the author pressed.
  it('refuses the whole unit when the engine refuses the embed member', async () => {
    const boldKey = await storedFaceKey(kanitBoldBytes)
    await seedMachine([kanitCut(boldKey, 'Bold', kanitBoldBytes)], { published: ['Regular', 'Bold'], refused: [] })
    const chains = [kanitChain()]
    const committed = { ...canvas, fontFamilies: ['Kanit'], fontChains: chains, components: [kanitText] }
    const request = vi.fn(async (operation: string, payload?: ArrayBuffer) => {
      if (operation === 'command') {
        // The engine's own located refusal, in the shape every other refusal
        // test in this repository builds: the message IS the Error's message.
        throw Object.assign(new Error('this face carries an `fvar` table'), { code: 'COMPONENT_INVALID', dataPath: 'fonts.Kanit[0]' })
      }
      return { snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: committed }, bytes: payload }
    })
    render(<App engine={engine(request)} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: committed }} />)
    fireEvent.click(screen.getByLabelText(/^text component e1/))
    await waitFor(() => expect(screen.queryByText(NO_BOLD)).toBeNull())

    fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
    await waitFor(() => expect(embedPayloads(request)).toHaveLength(1))
    // THE REFUSAL IS SHOWN, beside the toggle that sent it: a member's refusal
    // travels out of the unit verbatim and is anchored here by the FIELDS the
    // intent carried, which is `bold`.
    expect(await screen.findByText(/`fvar`/)).toBeInTheDocument()
    // AND THE DOCUMENT DID NOT MOVE. The toggle still reads off, because the
    // engine installed no new snapshot and the panel shows only committed
    // values.
    expect(screen.getByRole('button', { name: 'Bold' })).toHaveAttribute('aria-pressed', 'false')
  })

  // ⚠ THE (cut, chain) DEDUPE KEY, AND THE ONLY SELECTION THAT CAN SEE IT.
  //
  // A two-FAMILY selection does not discriminate: keying on the chain alone
  // gives the same two plans. What tells the two keys apart is one FAMILY
  // needing two DIFFERENT cuts, which happens whenever the selection mixes an
  // already-italic component with an upright one — B asks the first for
  // `boldItalic` and the second for `bold`. A chain-only key collapses those
  // into one member, and the component whose cut was dropped is set bold over a
  // face the document does not carry: the base face, painted with a warning,
  // with nothing on screen saying why.
  it('carries BOTH cuts of ONE family when the selection needs two', async () => {
    const boldKey = await storedFaceKey(kanitBoldBytes)
    const boldItalicKey = await storedFaceKey(kanitBoldItalicBytes)
    const italicKey = await storedFaceKey(kanitItalicBytes)
    await seedMachine(
      [
        kanitCut(boldKey, 'Bold', kanitBoldBytes),
        kanitCut(boldItalicKey, 'Bold Italic', kanitBoldItalicBytes),
        // The held italic is the settle condition: both components ask for
        // `italic` on the I control, so they AGREE and the sentence is shown
        // until the listing lands.
        kanitCut(italicKey, 'Italic', kanitItalicBytes),
      ],
      { published: ['Regular', 'Bold', 'Italic', 'Bold Italic'], refused: [] },
    )
    const chains = [kanitChain()]
    const upright = { ...kanitText, id: 'e1' }
    const slanted = { ...kanitText, id: 'e2', y: 30_000, italic: true }
    const componentCanvas = { ...canvas, fontFamilies: ['Kanit'], fontChains: chains, components: [upright, slanted] }
    const request = vi.fn(async (operation: string) => ({ snapshot: { documentState: 'loaded' as const, revision: operation === 'command' ? 2 : 1, byteLength: 3, canvas: componentCanvas }, bytes: kanitRegularBytes }))
    render(<App engine={engine(request)} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    fireEvent.click(screen.getByLabelText(/^text component e1/))
    fireEvent.click(screen.getByLabelText(/^text component e2/), { shiftKey: true })
    // ⚠ MATCHED BY PREFIX, because the two components disagree about `italic`
    // and `BooleanProperty` names a non-uniform control "Italic, mixed". That
    // disagreement IS the fixture — it is what makes B ask for two different
    // cuts — so the mixed name is a consequence of the thing under test rather
    // than something to design around.
    await waitFor(() => expect(screen.getByRole('button', { name: /^Italic/ }).getAttribute('aria-describedby'), 'the held italic must stop being reported absent once the listing lands').toBeNull())

    fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
    await waitFor(() => expect(embedPayloads(request)).toHaveLength(1))
    const members = embedPayloads(request)[0]!.commands as ReadonlyArray<Record<string, unknown>>
    expect(members, 'two cuts of one family are two members, not one').toHaveLength(3)
    // ONE CHAIN, TWO CUTS. The names are deliberately identical, which is the
    // whole point: the chain cannot be what tells these two members apart.
    expect(members.slice(0, 2).map((member) => member.name)).toEqual(['Kanit', 'Kanit'])
    expect(members[0]).toMatchObject({ kind: 'embedFontCut', cut: 'bold', data: base64Of(kanitBoldBytes) })
    expect(members[1]).toMatchObject({ kind: 'embedFontCut', cut: 'boldItalic', data: base64Of(kanitBoldItalicBytes) })
    expect(members[2]).toMatchObject({ kind: 'updateComponentProperties', ids: ['e1', 'e2'] })
  })

  // THE TOGGLE SURVIVES A REFUSAL, which is the property `BooleanProperty`'s
  // `pendingRef` puts at risk: it is taken before the await and cleared only on
  // RETURN, so anything leaving that call by another route leaves the control
  // dead for the rest of the session with no message anywhere — an author reads
  // that as a broken button, not as a refused action.
  //
  // MEASURED BY PRESSING IT AGAIN AND WATCHING IT WORK, not by inspecting a
  // flag. The second press is made to succeed — the dropped record is put back
  // past the designer, and the stale listing still names it — so a dead control
  // and a live one produce visibly different results rather than two silences.
  it('leaves the toggle usable after a refusal', async () => {
    const boldKey = await storedFaceKey(kanitBoldBytes)
    const store = await seedMachine([kanitCut(boldKey, 'Bold', kanitBoldBytes)], { published: ['Regular', 'Bold'], refused: [] })
    const request = cutRequest()
    mountKanit(request)
    await waitFor(() => expect(screen.queryByText(NO_BOLD)).toBeNull())

    expect((await store.remove(boldKey)).ok).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
    expect(await screen.findByText(/Kanit bold is not on this machine any more/)).toBeInTheDocument()
    expect(embedPayloads(request)).toHaveLength(0)

    // The face comes back; the listing never changed, so the plan is the same
    // plan and only the read's outcome differs.
    expect((await store.put(kanitCut(boldKey, 'Bold', kanitBoldBytes))).ok).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
    await waitFor(() => expect(embedPayloads(request), 'the second press did nothing: the control was left pending by the first').toHaveLength(1))
    expect(embedPayloads(request)[0]!.kind).toBe('applyCommands')
  })
})


// ---------------------------------------------------------------------------
// A COMMITTED FAMILY'S CUT REACHES THE DOCUMENT TOO
// (spec-install-all-face-cuts story 3, review finding F2).
//
// THE HOLE THIS CLOSES WAS TOTAL, not partial. `cutEmbedPlan` resolved a cut
// from `storedFaces` and from nothing else, and `installFamily` deliberately
// writes NOTHING to the machine store for a local row — a second copy of a
// committed face there would be two answers to one question. So the store is
// empty of all 31 committed families by construction: pressing B on one built
// no plan, embedded nothing, and the panel said *"No bold face in this
// family"* about a bold sitting in this very release. CAP-3 — "no offline
// author loses bold that an online author would get" — was false for 30 of the
// 31 committed families, and the data half of this story could not have made
// it true on its own.
//
// THE FIXTURE IS A REAL CATALOGUE ROW, NEVER A HAND-BUILT ONE. The family, its
// cut set, its licence text, its copyright and its provenance string are read
// out of the generated module the product itself reads, so a story that changed
// what a catalogue row carries reds here rather than passing over a fixture
// that agrees only with itself.
//
// NO STORE IS SEEDED IN THIS BLOCK, AND THAT IS THE POINT.
// ---------------------------------------------------------------------------
describe('a committed family\'s cut reaches the document on first use', () => {
  const BASE_KEY = '2222222222222222222222222222222222222222222222222222222222222222'
  const base64Of = (bytes: ArrayBuffer): string => {
    let binary = ''
    for (const byte of new Uint8Array(bytes)) binary += String.fromCharCode(byte)
    return btoa(binary)
  }

  // A COMMITTED FAMILY THAT PUBLISHES A BOLD, CHOSEN BY MEASUREMENT. Reaching
  // for a name would pin this suite to one family's upstream and red the day
  // that family's cut set changed for reasons this test is not about.
  const boldRow = catalogueFaces.find((face) => face.style === 'Bold')!
  const committedFamily = boldRow.family
  // AND ONE THAT PUBLISHES NO ITALIC, which is what makes the absence half of
  // CAP-2 measurable: six committed families genuinely have none, and for them
  // the original sentence is TRUE and must keep its words.
  const italicLessRow = catalogueFaces.find((face) => face.style === 'Regular'
    && !catalogueFaces.some((other) => other.family === face.family && other.style === 'Italic'))!

  const chainFor = (family: string) => ({
    name: family,
    entries: [{ face: '', assetKey: BASE_KEY, family, style: 'Regular', bold: '', italic: '', boldItalic: '' }],
  })

  const mountCommitted = (request: unknown, family = committedFamily) => {
    const chains = [chainFor(family)]
    const component = { ...textComponent, fontFamily: family }
    const componentCanvas = { ...canvas, fontFamilies: [family], fontChains: chains, components: [component] }
    render(<App engine={engine(request)} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
    fireEvent.click(screen.getByLabelText(/^text component e1/))
  }

  const committedRequest = (family = committedFamily) => {
    const chains = [chainFor(family)]
    const component = { ...textComponent, fontFamily: family }
    const shot = (revision: number) => ({ documentState: 'loaded' as const, revision, byteLength: 3, canvas: { ...canvas, fontFamilies: [family], fontChains: chains, components: [component] } })
    return vi.fn(async (operation: string) => operation === 'asset'
      ? { snapshot: shot(1), bytes: new Uint8Array([1, 2, 3]).buffer }
      : { snapshot: shot(operation === 'command' ? 2 : 1) })
  }

  // THE RELEASE CACHE, STOOD IN FOR BY `fetch`. jsdom has no Cache API, so
  // `localFaceIsHeld` answers "held" for the reason it always does — no cache
  // means no service worker, no content-addressed release and therefore no
  // deferral — and the read under test is the `fetch(cut.url)` that follows.
  const serveCatalogue = (bytesFor: (url: string) => ArrayBuffer | undefined) => {
    const stub = vi.fn(async (url: string) => {
      const bytes = bytesFor(String(url))
      return bytes === undefined ? { ok: false, status: 404, arrayBuffer: async () => new ArrayBuffer(0) } : { ok: true, arrayBuffer: async () => bytes }
    })
    const restore = globalThis.fetch
    globalThis.fetch = stub as never
    return { stub, restore: () => { globalThis.fetch = restore } }
  }

  it('embeds the bold from the release cache, with the catalogue row\'s own licence and provenance', async () => {
    const boldBytes = new Uint8Array([0x00, 0x01, 0x00, 0x00, 0x42]).buffer
    const served = serveCatalogue((url) => url === boldRow.url ? boldBytes : new ArrayBuffer(4))
    try {
      const request = committedRequest()
      mountCommitted(request)
      // ⚠ THE PANEL MUST SAY NOTHING FIRST. A press that embedded while the
      // panel still claimed the family had no bold would satisfy the assertion
      // below and leave CAP-2 broken on screen.
      await waitFor(() => expect(screen.queryByText(/No bold face in this family/)).toBeNull())

      fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
      await waitFor(() => expect(embedPayloads(request)).toHaveLength(1))

      const unit = embedPayloads(request)[0]!
      expect(unit.kind, 'the press must assemble a unit: one embed and one property commit, one undo entry').toBe('applyCommands')
      const members = unit.commands as ReadonlyArray<Record<string, unknown>>
      expect(members).toHaveLength(2)
      // THE BYTES ARE THE CATALOGUE ASSET'S, read through the URL the generated
      // module carries — not a store record, which does not exist for this
      // family and must not be made to.
      expect(members[0]).toMatchObject({
        kind: 'embedFontCut', name: committedFamily, index: 0, cut: 'bold',
        family: committedFamily, data: base64Of(boldBytes),
        licence: boldRow.licence, licenceText: boldRow.licenceText,
        copyright: boldRow.copyright, source: boldRow.source,
      })
      // ⚠ `style` IS THE STORE'S SPELLING, NOT THE CATALOGUE'S. The row says
      // `Bold` for this cut and `BoldItalic` for the combined one; a document
      // must record one vocabulary whichever tier served the bytes.
      expect(members[0]!.style).toBe('Bold')
      expect(members[1]).toMatchObject({ kind: 'updateComponentProperties' })
      // AND THE URL ASKED FOR IS THE CUT'S, never the family's Regular — the
      // one substitution this path could make silently.
      expect(served.stub.mock.calls.map((call) => String(call[0]))).toContain(boldRow.url)
    } finally {
      served.restore()
    }
  })

  // THE COMBINED CUT, WHOSE TWO VOCABULARIES ARE THE ONES THAT CAN DIVERGE
  // SILENTLY. The catalogue spells it `BoldItalic` and the store `Bold Italic`;
  // a lookup using the wrong one resolves to no row at all, so the combined cut
  // simply stops embedding while every bold and italic case stays green.
  it('embeds the BOLD ITALIC cut of a committed family, across the two style spellings', async () => {
    const row = catalogueFaces.find((face) => face.style === 'BoldItalic')!
    const bytes = new Uint8Array([0x00, 0x01, 0x00, 0x00, 0x43]).buffer
    const served = serveCatalogue((url) => url === row.url ? bytes : new ArrayBuffer(4))
    try {
      const chains = [chainFor(row.family)]
      const request = committedRequest(row.family)
      const componentCanvas = { ...canvas, fontFamilies: [row.family], fontChains: chains, components: [{ ...textComponent, fontFamily: row.family, italic: true }] }
      render(<App engine={engine(request)} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: componentCanvas }} />)
      fireEvent.click(screen.getByLabelText(/^text component e1/))
      await waitFor(() => expect(screen.queryByText(/No bold italic face in this family/)).toBeNull())

      fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
      await waitFor(() => expect(embedPayloads(request)).toHaveLength(1))
      const members = embedPayloads(request)[0]!.commands as ReadonlyArray<Record<string, unknown>>
      expect(members[0]).toMatchObject({ kind: 'embedFontCut', cut: 'boldItalic', style: 'Bold Italic', data: base64Of(bytes) })
    } finally {
      served.restore()
    }
  })

  // CAP-2's OTHER HALF: the sentence is RESERVED for upstream-publishes-none,
  // and six committed families are exactly that. The catalogue is this tier's
  // census, so its silence about a cut is a genuine absence rather than an
  // unasked question.
  it('keeps the original sentence for a committed family that publishes no italic', async () => {
    const served = serveCatalogue(() => new ArrayBuffer(4))
    try {
      const request = committedRequest(italicLessRow.family)
      mountCommitted(request, italicLessRow.family)
      const italic = await screen.findByRole('button', { name: 'Italic' })
      await waitFor(() => expect(italic.className).toContain('property-toggle-unavailable'))
      const described = italic.getAttribute('aria-describedby')
      expect(described).not.toBeNull()
      expect(document.getElementById(described!)).toHaveTextContent('No italic face in this family — the engine paints the regular face and warns.')
      // AND THE SAME FAMILY'S BOLD SAYS NOTHING, which is what stops this
      // passing by warning about everything.
      expect(screen.getByRole('button', { name: 'Bold' }).className).not.toContain('property-toggle-unavailable')
    } finally {
      served.restore()
    }
  })

  // A READ THAT FAILS IS REFUSED BY NAME AND CHANGES NOTHING. The remedy is
  // stated, no command is sent, and the toggle is left usable — the same
  // surface the store-miss path answers with.
  it('refuses by name when the bundled cut cannot be read, and sends no command', async () => {
    const served = serveCatalogue((url) => url === boldRow.url ? undefined : new ArrayBuffer(4))
    try {
      const request = committedRequest()
      mountCommitted(request)
      await waitFor(() => expect(screen.queryByText(/No bold face in this family/)).toBeNull())
      fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
      expect(await screen.findByText(new RegExp(`${committedFamily} bold could not be read from the offline bundle`))).toBeInTheDocument()
      expect(embedPayloads(request), 'a refused read must not reach the document').toHaveLength(0)
      expect(screen.getByRole('button', { name: 'Bold' })).not.toBeDisabled()
    } finally {
      served.restore()
    }
  })

  // ⚠ AND NOTHING IS WRITTEN TO THE MACHINE STORE. The owner's ruling is that
  // the release cache stays the authority for committed faces and the store
  // stays the authority for fetched web faces; a second copy of a committed
  // face in the store would be two answers to one question. Asserted over the
  // real store rather than over a spy, because the claim is about what is
  // THERE afterwards.
  it('writes no copy of the committed face into the machine store', async () => {
    const boldBytes = new Uint8Array([0x00, 0x01, 0x00, 0x00, 0x44]).buffer
    const served = serveCatalogue((url) => url === boldRow.url ? boldBytes : new ArrayBuffer(4))
    try {
      const request = committedRequest()
      mountCommitted(request)
      await waitFor(() => expect(screen.queryByText(/No bold face in this family/)).toBeNull())
      fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
      await waitFor(() => expect(embedPayloads(request)).toHaveLength(1))

      const opened = await openFontStore(globalThis.indexedDB)
      expect(opened.ok).toBe(true)
      const listed = opened.ok ? await opened.value.list() : undefined
      expect(listed?.ok).toBe(true)
      const faces = listed?.ok ? listed.value : []
      expect(faces.filter((face) => face.family === committedFamily), 'a committed face must never be copied into the machine store').toEqual([])
    } finally {
      served.restore()
    }
  })
})
