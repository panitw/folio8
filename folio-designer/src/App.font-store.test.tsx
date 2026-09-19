import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { IDBFactory as FakeIndexedDBFactory, IDBObjectStore as FakeIndexedDBObjectStore } from 'fake-indexeddb'
import App from './App'
import type { EngineClient } from './engine-client'
import { sfntWithNames } from './test/sfnt-fixture'
import { embeddedFaceFamily } from './embedded-face-family'
import { previewFaceFamily } from './preview-face-family'
import { openFontStore, storedFaceKey, type StoredFaceRecord } from './font-store'
import { webFamilies } from './font-index'
import { catalogueFaces } from './generated/font-catalogue'
import { shippedFaceFamily } from './shipped-face-family'
import { startBlankFromNew } from './test/new-document'

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
const canvas = { width: 595276, height: 841890, orientation: 'portrait' as const, preset: 'A4' as const, locale: 'en' as const, utcOffset: '+00:00', marginTop: 36000, marginRight: 36000, marginBottom: 36000, marginLeft: 36000, gridIncrement: 6000, commandWidth: 595276, commandHeight: 841890, fontFamilies: ['body'], fontChains: [{ name: 'body', entries: [face('Noto Sans')] }], defaultFontSize: 12000, defaultLineSpacing: 1000, contentWindowHeight: 729890, contentWindowCount: 1, contentWindowOrigins: [0], contentWindowPages: [0], contentWindowCountIsExact: true, bands: [{ name: 'pageHeader' as const, x: 36000, y: 36000, width: 523276, height: 20000 }, { name: 'content' as const, x: 36000, y: 56000, width: 523276, height: 729890 }, { name: 'pageFooter' as const, x: 36000, y: 785890, width: 523276, height: 20000 }], components: [] }
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
  mediaType: 'font/ttf',
  scripts: ['latin'],
  fetchedAt: '2026-09-03',
  byteLength: bytes.byteLength,
  bytes,
})

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
    const written = await opened.value.put(storedOnly(key, bytes))
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
    const seedWrite = await seeded.value.put({ ...storedOnly(seedKey, kanitFace), family: 'Kanit', licence: 'OFL-1.1', licenceText: kanitLicence, copyright: 'Copyright 2020 The Kanit Project Authors', source: 'google/fonts — ofl/kanit/Kanit-Regular.ttf, fetched 2026-09-03' })
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
    expect(Object.keys(stored)).toHaveLength(12)

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
    const seedWrite = await seeded.value.put({ ...storedOnly(seedKey, kanitFace), family: 'Kanit', licenceText: kanitLicence, copyright: 'Copyright 2020 The Kanit Project Authors' })
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
    const written = await opened.value.put({ ...storedOnly(key, bytes), family: 'Philosopher' })
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
    expect(local, 'every installed row renders').toHaveLength(catalogueFaces.length + 1)
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
      const written = await opened.value.put({ ...storedOnly(await storedFaceKey(bytes), bytes), family: row.family })
      expect(written.ok, `the fixture face for ${row.family} must reach the store`).toBe(true)
    }
    globalThis.fetch = vi.fn(async () => { throw new TypeError('Failed to fetch') }) as never
    mount(commandRequest())
    fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
    // THE GROUP ITSELF IS THE SETTLE CONDITION — the deleted panel no longer
    // offers a DOM count to wait on, and this is a more direct proof of the
    // property under test anyway.
    await waitFor(() => expect(within(screen.getByRole('group', { name: 'AVAILABLE LOCALLY' })).getAllByRole('option')).toHaveLength(catalogueFaces.length + deep.length))
    const local = within(screen.getByRole('group', { name: 'AVAILABLE LOCALLY' })).getAllByRole('option')
    expect(local, 'every row under a heading promising the bytes are here must be drawn').toHaveLength(catalogueFaces.length + deep.length)
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
    const seedWrite = await seeded.value.put({ ...storedOnly(seedKey, kanitFace), family: 'Kanit', licenceText: kanitLicence, copyright: 'Copyright 2020 The Kanit Project Authors' })
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
    const seedWrite = await seeded.value.put({ ...storedOnly(seedKey, kanitFace), family: 'Kanit', licenceText: kanitLicence, copyright: 'Copyright 2020 The Kanit Project Authors' })
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
    const seedWrite = await seeded.value.put({ ...storedOnly(seedKey, kanitFace), family: 'Kanit', licenceText: kanitLicence, copyright: 'Copyright 2020 The Kanit Project Authors' })
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
    const written = await opened.value.put({ ...storedOnly(key, kanitFace), family: 'Kanit', licenceText: kanitLicence, copyright: 'Copyright 2020 The Kanit Project Authors' })
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
    const written = await opened.value.put({ ...storedOnly(key, kanitFace), family: 'Kanit', licenceText: kanitLicence, copyright: 'Copyright 2020 The Kanit Project Authors' })
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
    const written = await opened.value.put({ ...storedOnly(key, kanitFace), family: 'Kanit', licenceText: kanitLicence, copyright: 'Copyright 2020 The Kanit Project Authors' })
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
    const written = await opened.value.put({ ...storedOnly(key, kanitFace), family: 'Noto Sans', licenceText: kanitLicence, copyright: 'Copyright 2026 The Noto Project Authors' })
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
    const written = await opened.value.put(storedOnly(key, bytes))
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
      const written = await opened.value.put({ ...storedOnly(await storedFaceKey(bytes), bytes), family })
      expect(written.ok, `the fixture face for ${family} must reach the store`).toBe(true)
    }
    globalThis.fetch = vi.fn(async () => { throw new TypeError('Failed to fetch') }) as never
    mount(commandRequest())
    // THE FACES REALLY LANDED IN `storedFaces` — the settle condition, proven
    // through the dropdown rather than the deleted panel.
    fireEvent.focus(screen.getByRole('combobox', { name: 'Font family' }))
    await waitFor(() => expect(within(screen.getByRole('group', { name: 'AVAILABLE LOCALLY' })).getAllByRole('option')).toHaveLength(catalogueFaces.length + fixtures.length))

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
    const written = await opened.value.put({ ...storedOnly(key, bytes), family })
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
    const written = await opened.value.put({ ...storedOnly(key, bytes), family })
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
