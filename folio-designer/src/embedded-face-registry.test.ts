import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { embeddedFaceFamily } from './embedded-face-family'
import { registerCarriedFaces } from './embedded-face-registry'

// The seam registers into the PAGE's font set, which jsdom does not implement,
// so the set and the face constructor are stubbed here. They are installed
// with `Object.defineProperty` because neither exists on jsdom's document or
// global to be assigned over — this is a stub, not a way around the authority
// contract: nothing in this file measures anything, and the registry under
// test computes no metric either.
const oneKey = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
const otherKey = 'fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210'

type Constructed = Readonly<{ family: string; bytes: unknown }>

const constructed: Constructed[] = []
const inSet: unknown[] = []
let refuseToLoad = false

class StubFace {
  readonly family: string
  constructor(family: string, bytes: unknown) {
    this.family = family
    constructed.push({ family, bytes })
  }
  load(): Promise<StubFace> {
    return refuseToLoad ? Promise.reject(new Error('this face will not parse')) : Promise.resolve(this)
  }
}

const stubSet = {
  add: (face: unknown) => { inSet.push(face) },
  delete: (face: unknown) => { const at = inSet.indexOf(face); if (at >= 0) inSet.splice(at, 1); return at >= 0 },
}

// Two turns of the microtask queue plus a macrotask: the seam chains a fetch
// promise, then the face's own load promise, before it touches the set.
const settle = () => new Promise((resolve) => setTimeout(resolve, 0))
const bytes = (length: number) => new ArrayBuffer(length)

beforeEach(() => {
  constructed.length = 0
  inSet.length = 0
  refuseToLoad = false
  Object.defineProperty(globalThis, 'FontFace', { value: StubFace, configurable: true, writable: true })
  Object.defineProperty(document, 'fonts', { value: stubSet, configurable: true, writable: true })
})

afterEach(() => {
  Reflect.deleteProperty(globalThis, 'FontFace')
  Reflect.deleteProperty(document, 'fonts')
})

describe('registering the faces a document carries', () => {
  it('registers one face per carried asset key, under the family derived from that key', async () => {
    const reported: string[][] = []
    const release = registerCarriedFaces([oneKey, otherKey], async () => bytes(8), (keys) => reported.push([...keys].sort()))
    await settle()
    expect(constructed.map((face) => face.family).sort()).toEqual([embeddedFaceFamily(oneKey), embeddedFaceFamily(otherKey)].sort())
    expect(inSet).toHaveLength(2)
    expect(reported[reported.length - 1]).toEqual([oneKey, otherKey].sort())
    release()
    expect(inSet).toEqual([])
  })

  it('registers a repeated key once, however many chain entries name it', async () => {
    const release = registerCarriedFaces([oneKey, oneKey, oneKey], async () => bytes(8), () => undefined)
    await settle()
    expect(constructed).toHaveLength(1)
    expect(inSet).toHaveLength(1)
    release()
  })

  // THE DEGRADE PATH, and the claim is about the SESSION, not only about this
  // module: a carried face whose bytes do not arrive must leave the canvas
  // painting with the stylesheet's declared stack. It reports the key as NOT
  // registered, which is what keeps the fragment off a family nothing declares.
  it('registers nothing and reports nothing when the asset request yields no bytes', async () => {
    const reported: string[][] = []
    const release = registerCarriedFaces([oneKey], async () => undefined, (keys) => reported.push([...keys]))
    await settle()
    expect(constructed).toEqual([])
    expect(inSet).toEqual([])
    expect(reported).toEqual([])
    release()
  })

  it('registers nothing when the asset request rejects, and does not rethrow', async () => {
    const reported: string[][] = []
    const release = registerCarriedFaces([oneKey], async () => { throw new Error('the engine refused this asset') }, (keys) => reported.push([...keys]))
    await settle()
    expect(constructed).toEqual([])
    expect(inSet).toEqual([])
    expect(reported).toEqual([])
    release()
  })

  it('registers nothing when the bytes arrive but the face will not parse', async () => {
    refuseToLoad = true
    const reported: string[][] = []
    const release = registerCarriedFaces([oneKey], async () => bytes(8), (keys) => reported.push([...keys]))
    await settle()
    expect(constructed).toHaveLength(1)
    expect(inSet).toEqual([])
    expect(reported).toEqual([])
    release()
  })

  it('registers nothing for a document that was replaced while the bytes were in flight', async () => {
    const reported: string[][] = []
    const release = registerCarriedFaces([oneKey], async () => bytes(8), (keys) => reported.push([...keys]))
    release()
    await settle()
    expect(inSet).toEqual([])
    expect(reported).toEqual([])
  })

  it('releases exactly the faces it added, so a superseded document leaves none behind', async () => {
    const first = registerCarriedFaces([oneKey], async () => bytes(8), () => undefined)
    await settle()
    const strangerFace = { family: 'a face this seam never added' }
    stubSet.add(strangerFace)
    const second = registerCarriedFaces([otherKey], async () => bytes(8), () => undefined)
    await settle()
    expect(inSet).toHaveLength(3)
    first()
    expect(inSet).toContain(strangerFace)
    expect(inSet).toHaveLength(2)
    second()
    expect(inSet).toEqual([strangerFace])
  })

  it('registers nothing and stays silent where the page has no font set at all', async () => {
    Reflect.deleteProperty(globalThis, 'FontFace')
    const reported: string[][] = []
    const release = registerCarriedFaces([oneKey], async () => bytes(8), (keys) => reported.push([...keys]))
    await settle()
    expect(constructed).toEqual([])
    expect(reported).toEqual([])
    release()
  })
})
