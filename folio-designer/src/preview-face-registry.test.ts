import { afterEach, describe, expect, it, vi } from 'vitest'
import { embeddedFaceFamily } from './embedded-face-family'
import { registerCarriedFaces } from './embedded-face-registry'
import { isPreviewableFamilyName, previewFaceFamily } from './preview-face-family'
import { openPreviewFaceRegistry } from './preview-face-registry'
import { shippedFaceFamily } from './shipped-face-family'

// THE PREVIEW LIFETIME (Story 16.3), AND THE COLLISION IT MUST NOT BE ABLE TO
// HAVE.
//
// The story names this as the place it can go wrong, in those words: the page's
// font set is a GLOBAL, NAME-KEYED registry, so a preview face landing under a
// name the canvas asks for would let which page of a MODAL the author last
// looked at decide what the DOCUMENT paints. "Must not be able to" is stronger
// than "does not", so the disjointness is asserted here rather than read off the
// prefixes.

// The page font set jsdom does not implement, written the way `App.test.tsx` and
// `App.font-store.test.tsx` write it — with neither of the two spellings
// `canvas-authority-contract.test.ts` forbids, so this file needs no carve-out
// from that contract.
function installStubFontSet(): Readonly<{ restore: () => void; live: () => ReadonlyArray<string> }> {
  class StubFace {
    readonly family: string
    constructor(family: string) { this.family = family }
    load(): Promise<StubFace> { return Promise.resolve(this) }
  }
  const live: StubFace[] = []
  const set = {
    add: (loaded: StubFace) => { live.push(loaded); return undefined },
    delete: (loaded: StubFace) => { const at = live.indexOf(loaded); if (at >= 0) live.splice(at, 1); return undefined },
  }
  Object.defineProperty(globalThis, 'FontFace', { value: StubFace, configurable: true, writable: true })
  Object.defineProperty(document, 'fonts', { value: set, configurable: true, writable: true })
  return {
    restore: () => { Reflect.deleteProperty(globalThis, 'FontFace'); Reflect.deleteProperty(document, 'fonts') },
    live: () => live.map((face) => face.family),
  }
}

// THE SAME PAGE FONT SET, BUT EVERY FACE REFUSES TO PARSE. `face.load()`
// rejecting is the outcome a real corrupt or truncated font produces, and it
// used to be swallowed with no trace — the row waited on it for ever.
function installRefusingFontSet(): Readonly<{ restore: () => void; live: () => ReadonlyArray<string> }> {
  class RefusingFace {
    readonly family: string
    constructor(family: string) { this.family = family }
    load(): Promise<RefusingFace> { return Promise.reject(new Error('these bytes are not a font')) }
  }
  const live: RefusingFace[] = []
  const set = { add: (loaded: RefusingFace) => { live.push(loaded); return undefined }, delete: () => undefined }
  Object.defineProperty(globalThis, 'FontFace', { value: RefusingFace, configurable: true, writable: true })
  Object.defineProperty(document, 'fonts', { value: set, configurable: true, writable: true })
  return {
    restore: () => { Reflect.deleteProperty(globalThis, 'FontFace'); Reflect.deleteProperty(document, 'fonts') },
    live: () => live.map((face) => face.family),
  }
}

const bytes = () => new Uint8Array([0, 1, 2, 3]).buffer
// A MACROTASK, NOT A COUNTED NUMBER OF MICROTASK TICKS. The registration chain
// is `readBytes` -> `then` -> `face.load()` -> `then`/`catch`, and a promise
// RETURNED from a `then` costs extra adoption ticks; a hand-counted flush passed
// for the resolve path and was one tick short for the reject path, which made a
// real defect look like a test that could not see it. A timer drains whatever
// the queue holds.
const settle = async () => { await new Promise((resolve) => setTimeout(resolve, 0)) }

let installed: Readonly<{ restore: () => void; live: () => ReadonlyArray<string> }> | undefined
afterEach(() => { installed?.restore(); installed = undefined })

describe('a preview family cannot collide with a document family or a shipped one', () => {
  const carriedKey = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'

  it('puts the two derivations in namespaces that share no name', () => {
    // Not "these two happen to differ": the ONLY input a preview name has is a
    // family name, and the only input a carried name has is a 64-hex asset key.
    // Every preview name is checked against the carried derivation of a key that
    // spells the same characters, which is the closest the two can be brought.
    for (const family of ['Sarabun', 'Noto Sans Thai', carriedKey, 'folio8-carried-x', '']) {
      const preview = previewFaceFamily(family)
      if (preview === undefined) continue
      expect(preview).not.toBe(embeddedFaceFamily(carriedKey))
      expect(preview).not.toBe(embeddedFaceFamily(family))
      expect(preview.startsWith('folio8-carried-')).toBe(false)
      expect(embeddedFaceFamily(family).startsWith('folio8-preview-')).toBe(false)
    }
  })

  it('is not a name any shipped face can be asked for by', () => {
    for (const face of ['Noto Sans', 'Noto Sans Thai', 'Noto Sans SC']) {
      expect(shippedFaceFamily(face)).not.toContain('folio8-preview-')
      expect(previewFaceFamily(face)).not.toBe(shippedFaceFamily(face))
    }
  })

  it('encodes injectively, so two families can never share one registry name', () => {
    // A slug would fold these onto one name, and one registry name over two
    // faces is the silent substitution a content address exists to refuse.
    expect(previewFaceFamily('Foo Bar')).not.toBe(previewFaceFamily('Foo-Bar'))
    expect(previewFaceFamily('foo')).not.toBe(previewFaceFamily('Foo'))
    expect(previewFaceFamily('Sarabun')).toBe(previewFaceFamily('Sarabun'))
  })

  it('produces a CSS identifier and nothing that could escape one', () => {
    const hostile = previewFaceFamily('Evil\', sans-serif; background: url(x)')
    expect(hostile).toMatch(/^folio8-preview-[0-9a-f]+$/)
    expect(hostile).not.toContain(';')
    expect(hostile).not.toContain('\'')
  })

  it('declines a name rather than throwing, and says which names it declines', () => {
    expect(isPreviewableFamilyName('')).toBe(false)
    expect(previewFaceFamily('')).toBeUndefined()
    expect(previewFaceFamily('x'.repeat(129))).toBeUndefined()
    expect(previewFaceFamily('x'.repeat(128))).toBeDefined()
  })
})

describe('the registry holds exactly the families on screen', () => {
  it('registers what arrives, releases what leaves, and releases everything on close', async () => {
    installed = installStubFontSet()
    const registry = openPreviewFaceRegistry(async () => bytes(), () => undefined)

    registry.show(['Sarabun', 'Lora'])
    await settle()
    expect([...installed.live()].sort()).toEqual([previewFaceFamily('Lora'), previewFaceFamily('Sarabun')].sort())
    expect(registry.statusOf('Sarabun')).toBe('ready')

    // A page turn. The families that left are removed from the page's font set,
    // not merely forgotten about.
    registry.show(['Chonburi'])
    await settle()
    expect(installed.live()).toEqual([previewFaceFamily('Chonburi')])

    registry.close()
    await settle()
    expect(installed.live()).toEqual([])
  })

  it('does not re-register a family that is still on screen', async () => {
    installed = installStubFontSet()
    const read = vi.fn(async () => bytes())
    const registry = openPreviewFaceRegistry(read, () => undefined)
    registry.show(['Sarabun'])
    await settle()
    registry.show(['Sarabun'])
    registry.show(['Sarabun'])
    await settle()
    expect(read).toHaveBeenCalledTimes(1)
    expect(installed.live()).toHaveLength(1)
    registry.close()
  })

  it('reports a family whose bytes cannot be had as unavailable, and stops asking for it', async () => {
    installed = installStubFontSet()
    const read = vi.fn(async (family: string) => family === 'Missing' ? undefined : bytes())
    const registry = openPreviewFaceRegistry(read, () => undefined)
    registry.show(['Missing', 'Lora'])
    await settle()
    expect(registry.statusOf('Missing')).toBe('unavailable')
    expect(registry.statusOf('Lora')).toBe('ready')
    // One row's upstream failure does not cost the other rows their specimens.
    expect(installed.live()).toEqual([previewFaceFamily('Lora')])
    // A REFUSED FAMILY IS NOT ASKED AGAIN while the modal is open — the author
    // paging back and forth must not re-run a fetch that has already failed —
    // while a family that merely LEFT the page is registered again when it
    // returns, because its face was released when it left.
    registry.show(['Missing'])
    registry.show(['Missing', 'Lora'])
    await settle()
    expect(read.mock.calls.filter(([family]) => family === 'Missing')).toHaveLength(1)
    expect(read.mock.calls.filter(([family]) => family === 'Lora')).toHaveLength(2)
    registry.close()
  })

  it('treats a resolver that throws as a family it cannot show, never as a session fault', async () => {
    installed = installStubFontSet()
    const registry = openPreviewFaceRegistry(async () => { throw new Error('the network is not there') }, () => undefined)
    registry.show(['Kanit'])
    await settle()
    expect(registry.statusOf('Kanit')).toBe('unavailable')
    registry.close()
  })

  it('says `preparing` until the face has actually reached the page, never `ready` early', async () => {
    installed = installStubFontSet()
    let release: ((value: ArrayBuffer) => void) | undefined
    const registry = openPreviewFaceRegistry(() => new Promise<ArrayBuffer>((resolve) => { release = resolve }), () => undefined)
    registry.show(['Slow'])
    await settle()
    expect(registry.statusOf('Slow')).toBe('preparing')
    release?.(bytes())
    await settle()
    expect(registry.statusOf('Slow')).toBe('ready')
    registry.close()
  })

  it('reports a face whose bytes will not parse as unavailable, never as still fetching', async () => {
    installed = installRefusingFontSet()
    const registry = openPreviewFaceRegistry(async () => bytes(), () => undefined)
    registry.show(['Corrupt'])
    await settle()
    // The bytes arrived and the face was constructed; it is the PARSE that
    // failed. Before the seam reported declines this row read `preparing` for
    // the life of the modal.
    expect(registry.statusOf('Corrupt')).toBe('unavailable')
    expect(installed.live()).toEqual([])
    registry.close()
  })

  it('never fetches for a family whose name the derivation declines, and states it as unavailable', async () => {
    installed = installStubFontSet()
    const read = vi.fn(async (family: string) => { void family; return bytes() })
    const registry = openPreviewFaceRegistry(read, () => undefined)
    // `previewFaceFamily` declines the empty name and anything past its length
    // bound. The row must say so, and no upstream resolution may be spent on it.
    registry.show(['', 'x'.repeat(129), 'Lora'])
    await settle()
    expect(registry.statusOf('')).toBe('unavailable')
    expect(registry.statusOf('x'.repeat(129))).toBe('unavailable')
    expect(registry.statusOf('Lora')).toBe('ready')
    expect(read.mock.calls.map(([family]) => family)).toEqual(['Lora'])
    registry.close()
  })

  it('registers nothing at all in a build with no page font set', async () => {
    // jsdom, with the stub NOT installed. The registry must degrade rather than
    // throw: the modal still lists families, and every row says its specimen is
    // not set in itself.
    const registry = openPreviewFaceRegistry(async () => bytes(), () => undefined)
    registry.show(['Sarabun'])
    await settle()
    expect(registry.statusOf('Sarabun')).toBe('preparing')
    registry.close()
  })
})

describe('the one seam still decides the document\'s own family for itself', () => {
  it('registers a carried face under the carried derivation when no derivation is handed in', async () => {
    installed = installStubFontSet()
    const key = 'f'.repeat(64)
    const release = registerCarriedFaces([key], async () => bytes(), () => undefined)
    await settle()
    expect(installed.live()).toEqual([embeddedFaceFamily(key)])
    release()
    expect(installed.live()).toEqual([])
  })

  it('registers nothing for a key the handed-in derivation declines', async () => {
    installed = installStubFontSet()
    const release = registerCarriedFaces([''], async () => bytes(), () => undefined, previewFaceFamily)
    await settle()
    expect(installed.live()).toEqual([])
    release()
  })
})
