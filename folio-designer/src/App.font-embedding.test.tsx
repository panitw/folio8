import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { IDBFactory as FakeIndexedDBFactory } from 'fake-indexeddb'
import App from './App'
import type { EngineClient } from './engine-client'
import { sfntWithNames } from './test/sfnt-fixture'
import { openFontStore, storedFaceKey, type StoredFaceRecord } from './font-store'
import { authorSuppliedFaceSource } from './font-import'

// spec-font-sources-and-embedding STORY 6 — THE SETTING GOVERNS WHAT A SAVE
// WRITES, AND IT DOES SO AT THE MOMENT A FACE IS CHOSEN (D1).
//
// EVERY ROW OF THE STORY'S I/O MATRIX IS DRIVEN THROUGH THE WHOLE DESIGNER,
// because none of the claims is about a module: "picking a family with
// embedding off adds nothing to `assets`", "an acknowledged face reaches the
// wire as acknowledged", "turning the setting off warns once and then strips".
// Each of those is a sentence about the bytes that leave this designer, and the
// only place those bytes exist is at the engine boundary.
//
// THE STORE IS `fake-indexeddb`, per test, for `App.font-store.test.tsx`'s
// reasons: jsdom provides no IndexedDB, so without it every test here would be
// measuring the designer's degraded path instead of the one authors use.

const face = (name: string) => ({ face: name, assetKey: '', family: '', style: '', bold: '', italic: '', boldItalic: '' })
const assetEntry = (key: string, family: string, style: string, declared: Readonly<{ bold?: string }> = {}) =>
  ({ face: '', assetKey: key, family, style, bold: declared.bold ?? '', italic: '', boldItalic: '' })

const canvas = { width: 595276, height: 841890, orientation: 'portrait' as const, preset: 'A4' as const, locale: 'en' as const, utcOffset: '+00:00', embedFonts: true, marginTop: 36000, marginRight: 36000, marginBottom: 36000, marginLeft: 36000, gridIncrement: 6000, commandWidth: 595276, commandHeight: 841890, fontFamilies: ['body'], fontChains: [{ name: 'body', entries: [face('Noto Sans')] }], defaultFontSize: 12000, defaultLineSpacing: 1000, contentWindowHeight: 729890, contentWindowCount: 1, contentWindowOrigins: [0], contentWindowPages: [0], contentWindowCountIsExact: true, bands: [{ name: 'pageHeader' as const, x: 36000, y: 36000, width: 523276, height: 20000 }, { name: 'content' as const, x: 36000, y: 56000, width: 523276, height: 729890 }, { name: 'pageFooter' as const, x: 36000, y: 785890, width: 523276, height: 20000 }], components: [] }
const textComponent = { id: 'e1', type: 'text' as const, band: 'content' as const, x: 0, y: 0, width: 72_000, height: 24_000, resizable: true, value: 'Hello' }
/** The same element, already set in the brand family — what a first-use-of-a-cut press acts on. */
const brandText = { ...textComponent, fontFamily: 'Brand Grotesk' }
const engine = (request: unknown) => ({ request }) as unknown as EngineClient

// THE BRAND FACE THE AUTHOR SUPPLIED, AND ITS BINARY SAYS NOTHING ABOUT TERMS.
//
// ⚠ THE BLANK TRIO IS THE POINT, NOT A SHORTCUT. `licence`, `licenceText` and
// `copyright` are all `''` because a commercial foundry face legitimately
// declares no name ID 0, 13 or 14 — and a document embedding one is refused
// unless the record carries the author's acknowledgement. A fixture with terms
// would pass whether or not the acknowledgement reached the wire.
const brandBytes = sfntWithNames([{ platform: 3, nameID: 1, value: 'Brand Grotesk' }])
const brandBoldBytes = sfntWithNames([{ platform: 3, nameID: 1, value: 'Brand Grotesk' }, { platform: 3, nameID: 2, value: 'Bold' }])

const acknowledgedRecord = async (style: string, bytes: ArrayBuffer): Promise<StoredFaceRecord> => ({
  key: await storedFaceKey(bytes),
  family: 'Brand Grotesk',
  style,
  licence: '',
  licenceText: '',
  copyright: '',
  source: authorSuppliedFaceSource('2026-09-21'),
  authorAcknowledged: true,
  mediaType: 'font/ttf',
  scripts: [],
  fetchedAt: '2026-09-21',
  byteLength: bytes.byteLength,
  bytes,
})

const seed = async (records: ReadonlyArray<StoredFaceRecord>, published: ReadonlyArray<string>): Promise<void> => {
  const opened = await openFontStore(globalThis.indexedDB)
  if (!opened.ok) throw new Error(opened.reason)
  for (const record of records) {
    const written = await opened.value.put(record)
    expect(written.ok, 'the fixture face must really be in the store before the designer opens it').toBe(true)
  }
  const recorded = await opened.value.putCensus({ family: 'Brand Grotesk', published, refused: [], recordedAt: '2026-09-21' })
  expect(recorded.ok, 'the fixture census must really be in the store').toBe(true)
}

const commands = (request: { mock: { calls: unknown[][] } }) => request.mock.calls
  .filter((call) => call[0] === 'command')
  .map((call) => JSON.parse(new TextDecoder().decode(call[1] as ArrayBuffer)) as Record<string, unknown>)

let restoreFetch: typeof globalThis.fetch
let restoreIndexedDB: PropertyDescriptor | undefined

beforeEach(() => {
  restoreFetch = globalThis.fetch
  restoreIndexedDB = Object.getOwnPropertyDescriptor(globalThis, 'indexedDB')
  Object.defineProperty(globalThis, 'indexedDB', { value: new FakeIndexedDBFactory(), configurable: true, writable: true })
  // THE NETWORK IS GONE THROUGHOUT. Every claim here is about a face this
  // machine already holds, so a request that escapes to the network is a defect
  // rather than a slow test.
  globalThis.fetch = (vi.fn(async () => { throw new TypeError('Failed to fetch') })) as never
})

afterEach(() => {
  globalThis.fetch = restoreFetch
  if (restoreIndexedDB) Object.defineProperty(globalThis, 'indexedDB', restoreIndexedDB)
  else Reflect.deleteProperty(globalThis, 'indexedDB')
})

const projection = (embedFonts: boolean, chains = canvas.fontChains, components: ReadonlyArray<typeof brandText | typeof textComponent> = [textComponent]) =>
  ({ ...canvas, embedFonts, fontFamilies: chains.map((chain) => chain.name), fontChains: chains, components })

const requestFor = (embedFonts: boolean, chains = canvas.fontChains, components: ReadonlyArray<typeof brandText | typeof textComponent> = [textComponent]) =>
  vi.fn(async (operation: string, _payload?: unknown) => ({ snapshot: { documentState: 'loaded' as const, revision: operation === 'command' ? 2 : 1, byteLength: 3, canvas: projection(embedFonts, chains, components) }, bytes: brandBytes }))

const mount = (request: unknown, embedFonts: boolean, chains = canvas.fontChains, components: ReadonlyArray<typeof brandText | typeof textComponent> = [textComponent]) => {
  render(<App engine={engine(request)} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: projection(embedFonts, chains, components) }} />)
  fireEvent.click(screen.getByLabelText(/^text component e1/))
}

/** Drives the family control the way an author does. */
const pick = (query: string, name: RegExp): boolean => {
  const combobox = screen.getByRole('combobox', { name: 'Font family' })
  fireEvent.focus(combobox)
  fireEvent.change(combobox, { target: { value: query } })
  const option = screen.queryByRole('option', { name })
  if (option) fireEvent.click(option)
  return option !== null
}

describe('the embed setting decides what a font gesture writes', () => {
  it('carries the face, acknowledgement and all, when embedding is on', async () => {
    // ⚠ THE PINNING TEST A-30 ASKED FOR, AND IT IS THE REASON THIS FILE EXISTS.
    // Stories 3 and 5 were each complete while the product could not do the
    // thing the epic is for: the author's acknowledgement was recorded at
    // import and then dropped on the floor, because every call site passed a
    // literal `false`. This asserts the whole route — import record, store,
    // wire — over a face whose binary declares NO terms at all, which the
    // engine refuses without the acknowledgement.
    await seed([await acknowledgedRecord('Regular', brandBytes)], ['Regular'])
    const request = requestFor(true)
    mount(request, true)

    await waitFor(() => expect(pick('Brand', /^Brand Grotesk$/), 'the stored family must be offered').toBe(true))
    await waitFor(() => expect(commands(request).length).toBeGreaterThan(0))
    const embed = commands(request).find((command) => command['kind'] === 'embedFontFamily')!
    expect(embed, 'first use of a stored family embeds it').toBeDefined()
    expect(embed['authorAcknowledged']).toBe(true)
    // AND THE TERMS ARE BLANK, WHICH IS WHAT MAKES THE FLAG LOAD-BEARING. With
    // any of the three populated the engine would admit this face anyway.
    expect(embed['licence']).toBe('')
    expect(embed['licenceText']).toBe('')
    expect(embed['copyright']).toBe('')
    expect(embed['data']).toBe(btoa(String.fromCharCode(...new Uint8Array(brandBytes))))
  })

  it('names the face and embeds nothing when embedding is off', async () => {
    await seed([await acknowledgedRecord('Regular', brandBytes)], ['Regular'])
    const request = requestFor(false)
    mount(request, false)

    await waitFor(() => expect(pick('Brand', /^Brand Grotesk$/), 'the stored family must be offered').toBe(true))
    await waitFor(() => expect(commands(request).length).toBeGreaterThan(0))
    const declared = commands(request)[0]!
    // THE CHAIN IS DECLARED BY NAME. `addFontChain` has no `asset` arm at all,
    // so this shape cannot carry bytes even by accident.
    expect(declared['kind']).toBe('addFontChain')
    expect(declared['name']).toBe('Brand Grotesk')
    expect((declared['entries'] as ReadonlyArray<unknown>)[0]).toBe('Brand Grotesk')
    // NOTHING EMBEDS, ON ANY COMMAND OF THE GESTURE.
    expect(commands(request).some((command) => command['kind'] === 'embedFontFamily')).toBe(false)
    expect(JSON.stringify(commands(request))).not.toContain('"data"')
  })

  it('declares the cuts this machine holds, so a named document can still bold', async () => {
    // ⚠ THE COMMAND VOCABULARY CANNOT ADD A VARIANT TO AN EXISTING ENTRY, so an
    // entry that did not declare its cuts when it was written could never gain
    // them and a name-only document could never bold at all. The cuts are
    // NAMED, never carried.
    await seed([await acknowledgedRecord('Regular', brandBytes), await acknowledgedRecord('Bold', brandBoldBytes)], ['Regular', 'Bold'])
    const request = requestFor(false)
    mount(request, false)

    await waitFor(() => expect(pick('Brand', /^Brand Grotesk$/)).toBe(true))
    await waitFor(() => expect(commands(request).length).toBeGreaterThan(0))
    const entries = commands(request)[0]!['entries'] as ReadonlyArray<unknown>
    expect(entries[0]).toEqual({ face: 'Brand Grotesk', bold: 'Brand Grotesk Bold' })
    expect(JSON.stringify(commands(request))).not.toContain('"data"')
  })

  it('names a cut on first use of it, in one unit with the property, when embedding is off', async () => {
    // THE PRESS IS STILL ONE UNIT AND ONE UNDO ENTRY. What changes is what the
    // unit carries: a face NAME rather than half a megabyte of bold.
    await seed([await acknowledgedRecord('Regular', brandBytes), await acknowledgedRecord('Bold', brandBoldBytes)], ['Regular', 'Bold'])
    const chains = [{ name: 'Brand Grotesk', entries: [face('Brand Grotesk')] }]
    const components = [brandText]
    const request = requestFor(false, chains, components)
    mount(request, false, chains, components)

    // The panel stops reporting an absent bold once the store listing lands,
    // which is this test's settle condition as well as its positive control.
    await waitFor(() => expect(screen.getByRole('button', { name: 'Bold' }).className).not.toContain('property-toggle-unavailable'))
    fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
    await waitFor(() => expect(commands(request)).toHaveLength(1))

    const unit = commands(request)[0]!
    expect(unit['kind']).toBe('applyCommands')
    const members = unit['commands'] as ReadonlyArray<Record<string, unknown>>
    // ⚠ THE CUT IS DECLARED ON THE BASE ENTRY'S OWN `bold`, AND NOTHING ELSE
    // WOULD WORK. The engine picks a weight off the field of the entry that
    // covered the rune, never off a later entry, so appending a sibling entry
    // named `Brand Grotesk Bold` would draw bold in the base face and leave the
    // panel still reporting the cut missing — so the next press would append
    // another one. There is no command that edits an entry's variants, so the
    // chain is rebuilt out of four that already exist, in one unit.
    expect(members).toHaveLength(5)
    expect(members[0]).toEqual({ kind: 'renameFontChain', version: 1, name: 'Brand Grotesk', to: 'Brand Grotesk (rebuilding)' })
    expect(members[1]).toEqual({ kind: 'addFontChain', version: 1, name: 'Brand Grotesk', entries: [{ face: 'Brand Grotesk', bold: 'Brand Grotesk Bold' }] })
    expect(members[2]).toEqual({ kind: 'updateComponentProperties', version: 1, ids: ['e1'], changes: { fontFamily: { op: 'set', value: 'Brand Grotesk' } } })
    expect(members[3]).toEqual({ kind: 'deleteFontChain', version: 1, name: 'Brand Grotesk (rebuilding)' })
    expect(members[4]).toEqual({ kind: 'updateComponentProperties', version: 1, ids: ['e1'], changes: { bold: { op: 'set', value: true } } })
    // NOTHING EMBEDS, AND NO SIBLING ENTRY IS APPENDED.
    expect(members.some((member) => member['kind'] === 'embedFontCut' || member['kind'] === 'addFontChainEntry')).toBe(false)
  })

  it('declares no cut against a chain that carries a face, and says why', async () => {
    // ⚠ A MIXED CHAIN IS REACHABLE: undo a strip, or open an `embedFonts:
    // false` document that carries assets. `addFontChain` has no `asset` arm,
    // so such a chain cannot be redeclared at all — refused with a sentence
    // rather than rebuilt into something that drops the carried face.
    await seed([await acknowledgedRecord('Regular', brandBytes), await acknowledgedRecord('Bold', brandBoldBytes)], ['Regular', 'Bold'])
    const chains = [{ name: 'Brand Grotesk', entries: [assetEntry('c'.repeat(64), 'Brand Grotesk', 'Regular'), face('Noto Sans Thai')] }]
    const request = requestFor(false, chains, [brandText])
    mount(request, false, chains, [brandText])

    // ⚠ AND THE BASE IS ENTRY ZERO, NOT "THE FIRST NAME ENTRY". On this chain
    // the first name entry is the Thai script fallback; declaring a cut against
    // it would bold the Thai and leave the Latin alone. Entry zero carries a
    // face, so this mode has no base at all and no plan is built — the press
    // commits the property and nothing else.
    fireEvent.click(screen.getByRole('button', { name: 'Bold' }))
    await waitFor(() => expect(commands(request)).toHaveLength(1))
    expect(commands(request)[0]!['kind'], 'a mixed chain must not be rebuilt by name').toBe('updateComponentProperties')
  })

  it('hands a named face to the engine from this machine, so the preview does not move', async () => {
    // ⚠ WITHOUT THIS, TOGGLING THE SETTING CHANGES THE PAGE SET. The engine
    // resolves a name out of the font set it holds, and a brand face off the
    // author's own disk is not among the eleven shipped ones — so the render
    // would refuse or substitute for a face sitting in the store.
    await seed([await acknowledgedRecord('Regular', brandBytes)], ['Regular'])
    const chains = [{ name: 'Brand Grotesk', entries: [face('Brand Grotesk')] }]
    const request = requestFor(false, chains, [brandText])
    mount(request, false, chains, [brandText])

    await waitFor(() => expect(request.mock.calls.some((call) => call[0] === 'install-face')).toBe(true))
    const installed = (request.mock.calls.find((call) => call[0] === 'install-face') as unknown as [string, Readonly<{ face: string; bytes: ArrayBuffer }>])[1]
    expect(installed.face, 'the engine is given the face under the name the document uses').toBe('Brand Grotesk')
    expect(new Uint8Array(installed.bytes)).toEqual(new Uint8Array(brandBytes))
  })

  it('records no acknowledgement for a catalogue face, which is the permanent answer', async () => {
    // THE NEGATIVE HALF OF THE PINNING TEST. `true` on the wire for every face
    // would pass the case above and would put an assertion the author never
    // made into every document — so the catalogue tier is pinned beside it,
    // over a record that reaches the embed by exactly the same route.
    await seed([{ ...await acknowledgedRecord('Regular', brandBytes), authorAcknowledged: false, licence: 'OFL-1.1', licenceText: 'terms', copyright: 'Copyright', source: 'google/fonts — ofl/brand/Brand-Regular.ttf, fetched 2026-09-03' }], ['Regular'])
    const request = requestFor(true)
    mount(request, true)

    await waitFor(() => expect(pick('Brand', /^Brand Grotesk$/)).toBe(true))
    await waitFor(() => expect(commands(request).length).toBeGreaterThan(0))
    const embed = commands(request).find((command) => command['kind'] === 'embedFontFamily')!
    expect(embed['authorAcknowledged']).toBe(false)
  })

  it('hands the engine nothing for a shipped CUT, which it already holds under that name', async () => {
    // ⚠ THE CUT HALF OF THE SHIPPED SET IS AS LOAD-BEARING AS THE FAMILY HALF.
    // The store can legitimately hold a downloaded `Noto Sans Bold`, and a
    // membership test that listed only the four families would install those
    // bytes over the engine's own face — replacing a face every render of this
    // release resolves with one this machine happened to fetch.
    //
    // ⚠ THE SETTLE IS A SECOND FACE THAT SORTS **AFTER** THE ONE UNDER TEST.
    // The installer walks the document's names in sorted order, so an install
    // of `Zeta Grotesk` proves the loop has already decided about `Noto Sans`
    // and `Noto Sans Bold` — where waiting on the store listing alone would let
    // a wrong decision land just after the assertion.
    await seed([{ ...await acknowledgedRecord('Regular', brandBytes), family: 'Noto Sans', style: 'Bold' }], ['Bold'])
    const opened = await openFontStore(globalThis.indexedDB)
    if (!opened.ok) throw new Error(opened.reason)
    expect((await opened.value.put({ ...await acknowledgedRecord('Regular', brandBoldBytes), family: 'Zeta Grotesk' })).ok).toBe(true)
    const chains = [{ name: 'body', entries: [{ ...face('Noto Sans'), bold: 'Noto Sans Bold' }, face('Zeta Grotesk')] }]
    const request = requestFor(false, chains)
    mount(request, false, chains)

    const installs = () => request.mock.calls.filter((call) => call[0] === 'install-face').map((call) => (call as unknown as [string, Readonly<{ face: string }>])[1].face)
    await waitFor(() => expect(installs()).toContain('Zeta Grotesk'))
    expect(installs(), 'a face this release ships must never be replaced by one this machine downloaded').toEqual(['Zeta Grotesk'])
  })

  it('hands the engine nothing for a shipped name, which it already holds', async () => {
    await seed([await acknowledgedRecord('Regular', brandBytes)], ['Regular'])
    const request = requestFor(false)
    mount(request, false)
    // The store listing is what would trigger an install if the filter were
    // wrong, so waiting for the family to be offered is waiting past the risk.
    await waitFor(() => expect(pick('Brand', /^Brand Grotesk$/)).toBe(true))
    expect(request.mock.calls.some((call) => call[0] === 'install-face')).toBe(false)
  })
})

describe('turning embedding off strips the faces the document carries', () => {
  const carriedKey = 'a'.repeat(64)
  const carriedBoldKey = 'b'.repeat(64)
  const carriedChains = [{ name: 'Brand Grotesk', entries: [assetEntry(carriedKey, 'Brand Grotesk', 'Regular'), face('Noto Sans Thai')] }]
  /** The same chain after a strip: the face named, the asset gone. What an undo has to put back. */
  const strippedChains = [{ name: 'Brand Grotesk', entries: [face('Brand Grotesk'), face('Noto Sans Thai')] }]

  const untickAndApply = () => {
    fireEvent.click(screen.getByRole('checkbox', { name: 'Embed fonts in the document' }))
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
  }

  const mountCarried = (request: unknown, chains = carriedChains) => {
    render(<App engine={engine(request)} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: projection(true, chains, []) }} />)
  }

  it('warns before the first strip and rewrites every carried entry as one unit', async () => {
    const request = requestFor(true, carriedChains, [])
    mountCarried(request)
    untickAndApply()

    // THE QUESTION COMES BEFORE ANY COMMAND. Nothing has been written when the
    // author is asked, which is what makes a decline cost nothing.
    await screen.findByRole('dialog', { name: 'Remove the faces this document carries?' })
    expect(commands(request), 'nothing may be sent before the question is answered').toEqual([])
    fireEvent.click(screen.getByRole('button', { name: 'Remove the faces' }))

    await waitFor(() => expect(commands(request).length).toBeGreaterThanOrEqual(2))
    const sent = commands(request)
    // ⚠ THE STRIP GOES FIRST AND THE SETTING FOLLOWS IT. The other order leaves
    // a refused strip on a document that declares `embedFonts: false` while
    // still carrying every asset, with nothing to roll back with.
    expect(sent[0]!['kind']).toBe('applyCommands')
    expect(sent[1]).toEqual({ kind: 'setDocumentEmbedFonts', version: 1, embedFonts: false })
    // ONE UNIT, THEREFORE ONE UNDO ENTRY: a half-stripped document is not a
    // state an author can reach.
    expect(sent[0]!['commands']).toEqual([
      // INSERT BEFORE REMOVE. `removeFontChainEntry` refuses to empty a chain,
      // and the name lands in exactly the position the asset entry held.
      { kind: 'addFontChainEntry', version: 1, name: 'Brand Grotesk', index: 0, face: 'Brand Grotesk' },
      { kind: 'removeFontChainEntry', version: 1, name: 'Brand Grotesk', index: 1 },
    ])
    // THE ASSETS THEMSELVES ARE THE ENGINE'S TO DROP — `removeFontChainEntry`
    // already collects what nothing names — so the designer sends no delete.
    expect(JSON.stringify(sent)).not.toContain('deleteAsset')
  })

  it('names every carried entry, including one a second chain also carries', async () => {
    const chains = [
      { name: 'Brand Grotesk', entries: [assetEntry(carriedKey, 'Brand Grotesk', 'Regular', { bold: carriedBoldKey })] },
      { name: 'display', entries: [face('Noto Sans'), assetEntry(carriedBoldKey, 'Brand Grotesk', 'Bold')] },
    ]
    const request = requestFor(true, chains, [])
    mountCarried(request, chains)
    untickAndApply()
    await screen.findByRole('dialog', { name: 'Remove the faces this document carries?' })
    fireEvent.click(screen.getByRole('button', { name: 'Remove the faces' }))

    await waitFor(() => expect(commands(request).length).toBeGreaterThanOrEqual(2))
    expect(commands(request)[0]!['commands']).toEqual([
      { kind: 'addFontChainEntry', version: 1, name: 'Brand Grotesk', index: 0, face: 'Brand Grotesk' },
      { kind: 'removeFontChainEntry', version: 1, name: 'Brand Grotesk', index: 1 },
      // THE SECOND CHAIN'S OWN ENTRY, AT ITS OWN INDEX. A per-chain walk is
      // what keeps the second chain's positions from being read against the
      // first one's.
      { kind: 'addFontChainEntry', version: 1, name: 'display', index: 1, face: 'Brand Grotesk Bold' },
      { kind: 'removeFontChainEntry', version: 1, name: 'display', index: 2 },
    ])
  })

  it('changes nothing at all when the author declines', async () => {
    const request = requestFor(true, carriedChains, [])
    mountCarried(request)
    untickAndApply()
    await screen.findByRole('dialog', { name: 'Remove the faces this document carries?' })
    fireEvent.click(screen.getByRole('button', { name: 'Keep embedding' }))

    await waitFor(() => expect(commands(request).length).toBeGreaterThan(0))
    // THE SETTING IS UNTOUCHED AND SO IS EVERY CHAIN. The page-setup command
    // still goes, because the author did not decline their margins.
    expect(commands(request).some((command) => command['kind'] === 'setDocumentEmbedFonts')).toBe(false)
    expect(commands(request).some((command) => command['kind'] === 'applyCommands')).toBe(false)
    expect(commands(request).at(-1)!['kind']).toBe('pageSetup')
    // AND THE BOX GOES BACK TO WHAT THE ENGINE HOLDS. Leaving it unticked would
    // have the panel reporting a setting the author had just declined to make.
    expect(screen.getByRole('checkbox', { name: 'Embed fonts in the document' })).toBeChecked()
  })

  it('asks once and only once, and a declined strip is still a first strip', async () => {
    const request = requestFor(true, carriedChains, [])
    mountCarried(request)

    // FIRST ATTEMPT, DECLINED. Nothing was stripped, so the next attempt is
    // still the first strip and is still worth announcing.
    untickAndApply()
    await screen.findByRole('dialog', { name: 'Remove the faces this document carries?' })
    fireEvent.click(screen.getByRole('button', { name: 'Keep embedding' }))
    await waitFor(() => expect(commands(request).length).toBeGreaterThan(0))

    // SECOND ATTEMPT, ACCEPTED.
    untickAndApply()
    await screen.findByRole('dialog', { name: 'Remove the faces this document carries?' })
    fireEvent.click(screen.getByRole('button', { name: 'Remove the faces' }))
    await waitFor(() => expect(commands(request).some((command) => command['kind'] === 'applyCommands')).toBe(true))

    // THIRD ATTEMPT: no question. The author has been told, and a modal on
    // every toggle is a modal nobody reads.
    const before = commands(request).length
    untickAndApply()
    await waitFor(() => expect(commands(request).length).toBeGreaterThan(before))
    expect(screen.queryByRole('dialog', { name: 'Remove the faces this document carries?' })).toBeNull()
  })

  it('leaves the setting alone when the strip itself is refused', async () => {
    // ⚠ THE ORDER IS WHAT MAKES THIS SAFE. A refused strip must not leave the
    // document declaring that it carries no faces while carrying every one of
    // them — there is no rollback for that, because the setting command has
    // already been accepted and is its own history entry.
    const refusing = vi.fn(async (operation: string) => {
      if (operation === 'command') throw new Error('a font chain named "Brand Grotesk" already exists')
      return { snapshot: { documentState: 'loaded' as const, revision: 1, byteLength: 3, canvas: projection(true, carriedChains, []) } }
    })
    mountCarried(refusing)
    untickAndApply()
    await screen.findByRole('dialog', { name: 'Remove the faces this document carries?' })
    fireEvent.click(screen.getByRole('button', { name: 'Remove the faces' }))

    // THE AUTHOR IS TOLD, IN THE ENGINE'S OWN WORDS.
    expect(await screen.findByText(/a font chain named/)).toBeInTheDocument()
    // AND NOTHING ELSE WAS SENT: no setting, no page setup.
    expect(commands(refusing).map((command) => command['kind'])).toEqual(['applyCommands'])
  })

  it('restores an undone strip in one step, faces and entries together', async () => {
    // THE UNDO IS DRIVEN, NOT INFERRED FROM THE UNIT'S SHAPE. `wasm.Engine.Apply`
    // pushes one undo entry per accepted command, so a strip that reached the
    // engine as ONE `applyCommands` is one step back — and the projection the
    // undo returns is the document with its carried entries and its assets.
    const undone = vi.fn(async (operation: string) => ({
      snapshot: { documentState: 'loaded' as const, revision: operation === 'undo' ? 3 : 2, byteLength: 3, canvas: projection(true, operation === 'undo' ? carriedChains : strippedChains, []), canUndo: true, canRedo: false },
    }))
    mountCarried(undone)
    untickAndApply()
    await screen.findByRole('dialog', { name: 'Remove the faces this document carries?' })
    fireEvent.click(screen.getByRole('button', { name: 'Remove the faces' }))
    await waitFor(() => expect(commands(undone).length).toBeGreaterThanOrEqual(2))

    await waitFor(() => expect(screen.getByLabelText('Undo')).not.toBeDisabled())
    fireEvent.click(screen.getByLabelText('Undo'))
    await waitFor(() => expect(undone.mock.calls.some((call) => call[0] === 'undo')).toBe(true))
    // ONE `undo` REQUEST, not two: the entries and the assets came back together.
    expect(undone.mock.calls.filter((call) => call[0] === 'undo')).toHaveLength(1)
  })

  it('tells the author when a stripped face is one nothing here can supply', async () => {
    // STRIPPING A FACE THIS MACHINE DOES NOT HOLD RENAMES IT INTO A NAME
    // NOTHING HERE RESOLVES, which moves the preview — so it is said out loud
    // in the one dialog whose job is informed consent, rather than discovered
    // from a page that came out in another face.
    const request = requestFor(true, carriedChains, [])
    mountCarried(request)
    untickAndApply()
    const dialog = await screen.findByRole('dialog', { name: 'Remove the faces this document carries?' })
    expect(within(dialog).getByText(/not on this machine and not one this release ships/)).toBeInTheDocument()
    // AND THE LOSS OF THE ENTRY'S DECLARED CUTS IS STATED TOO.
    expect(within(dialog).getByText(/bold or italic a carried entry declared is discarded/)).toBeInTheDocument()
    // AND THE UNDO PROMISE IS THE TRUE ONE: two commands, two history entries.
    expect(within(dialog).getByText(/the setting itself is a separate step/)).toBeInTheDocument()
  })

  it('says nothing about resolving when every stripped face is one this release ships', async () => {
    const chains = [{ name: 'body', entries: [assetEntry(carriedKey, 'Noto Sans', 'Regular')] }]
    const request = requestFor(true, chains, [])
    mountCarried(request, chains)
    untickAndApply()
    const dialog = await screen.findByRole('dialog', { name: 'Remove the faces this document carries?' })
    expect(within(dialog).queryByText(/not on this machine/)).toBeNull()
  })

  it('refuses a strip longer than one command unit, before the author is asked', async () => {
    // ⚠ TWO MEMBERS PER CARRIED ENTRY AGAINST THE ENGINE'S 1..64 BOUND. Asking
    // first and discovering the bound afterwards would refuse a destructive act
    // the author had already consented to.
    const entries = Array.from({ length: 33 }, (_, index) => assetEntry(`${index}`.padStart(64, 'a'), 'Brand Grotesk', index === 0 ? 'Regular' : `Weight ${index}`))
    const chains = [{ name: 'Brand Grotesk', entries }]
    const request = requestFor(true, chains, [])
    mountCarried(request, chains)
    untickAndApply()
    expect(await screen.findByText(/more than one change the engine will accept/)).toBeInTheDocument()
    expect(screen.queryByRole('dialog', { name: 'Remove the faces this document carries?' })).toBeNull()
    expect(commands(request)).toEqual([])
  })

  it('asks nothing when the document carries no face', async () => {
    const request = requestFor(true, canvas.fontChains, [])
    mountCarried(request, canvas.fontChains)
    untickAndApply()
    await waitFor(() => expect(commands(request).length).toBeGreaterThan(0))
    expect(screen.queryByRole('dialog', { name: 'Remove the faces this document carries?' })).toBeNull()
    expect(commands(request)[0]).toEqual({ kind: 'setDocumentEmbedFonts', version: 1, embedFonts: false })
    expect(commands(request).some((command) => command['kind'] === 'applyCommands')).toBe(false)
  })

  it('asks nothing when the setting is turned back ON', async () => {
    // A DOCUMENT MUST NEVER LOSE A FACE BECAUSE SOMETHING WAS SWITCHED ON.
    const chains = [{ name: 'Brand Grotesk', entries: [assetEntry(carriedKey, 'Brand Grotesk', 'Regular')] }]
    const request = requestFor(false, chains, [])
    render(<App engine={engine(request)} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: projection(false, chains, []) }} />)
    fireEvent.click(screen.getByRole('checkbox', { name: 'Embed fonts in the document' }))
    fireEvent.click(screen.getByRole('button', { name: 'Apply page setup' }))
    await waitFor(() => expect(commands(request).length).toBeGreaterThan(0))
    expect(screen.queryByRole('dialog', { name: 'Remove the faces this document carries?' })).toBeNull()
    expect(commands(request)[0]).toEqual({ kind: 'setDocumentEmbedFonts', version: 1, embedFonts: true })
  })

  it('refuses the whole gesture when a carried face cannot be named', async () => {
    // ⚠ THE SHAPE THE ENGINE REALLY EMITS FOR A RECORD WITH NO FAMILY IS THE
    // ASSET KEY, NOT AN EMPTY STRING (`page_setup.go`'s `projectFontChainEntry`
    // seeds `Family = AssetKey`). A guard testing for emptiness is dead code,
    // and such a document would have a SHA-256 written into its chain as a face
    // name — which no host directory can resolve.
    //
    // A HALF-EMPTY RECORD GOES THE SAME WAY, BY THE SAME RULE: an empty `style`
    // would be read as the family's Regular, which on a document carrying only
    // a Bold cut names a face it does not hold.
    for (const entry of [assetEntry(carriedKey, carriedKey, 'Regular'), assetEntry(carriedKey, 'Brand Grotesk', ''), assetEntry(carriedKey, '', '')]) {
      const chains = [{ name: 'Brand Grotesk', entries: [entry] }]
      const request = requestFor(true, chains, [])
      const view = render(<App engine={engine(request)} blankBytes={new Uint8Array([1, 2, 3]).buffer} initialSnapshot={{ documentState: 'loaded', revision: 1, byteLength: 3, canvas: projection(true, chains, []) }} />)
      untickAndApply()
      expect(await screen.findByText(/does not name both a family and a style/)).toBeInTheDocument()
      expect(screen.queryByRole('dialog', { name: 'Remove the faces this document carries?' })).toBeNull()
      expect(commands(request)).toEqual([])
      view.unmount()
    }
  })
})
