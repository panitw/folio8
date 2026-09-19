import { describe, expect, it } from 'vitest'

import { embeddedFaceFamily, isCarriedFaceAssetKey } from './embedded-face-family'

// The digests of two different faces. Written out rather than hashed here:
// what this module promises is a function of the string it is handed, and
// deriving the input from bytes would only test the derivation of the input.
const oneKey = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
const otherKey = 'fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210'

describe('the CSS family a carried face is asked for by', () => {
  it('derives a distinct family for each distinct asset key', () => {
    expect(embeddedFaceFamily(oneKey)).not.toBe(embeddedFaceFamily(otherKey))
    // And it is a FUNCTION of the key: the same asset asked for twice is the
    // same family, or a face registered once would be asked for under a name
    // nothing registered.
    expect(embeddedFaceFamily(oneKey)).toBe(embeddedFaceFamily(oneKey))
  })

  // D-8.4.1, stated as the case it exists for. `document.fonts` is a global
  // name-keyed registry, so two faces that share a `font.family` — one carried
  // by the document, one shipped by the build — must not share a CSS family or
  // one would silently substitute for the other. The derivation cannot even
  // see `font.family`, which is the strongest form of that promise: there is
  // no argument through which a display name could reach it.
  it('cannot collide with a shipped face that shares the document face\'s font.family', () => {
    const shipped = ['Inter', 'IBM Plex Sans', 'IBM Plex Sans Thai', 'IBM Plex Mono', 'Noto Sans', 'Noto Sans Thai', 'Noto Sans SC', 'sans-serif']
    for (const family of shipped) expect(embeddedFaceFamily(oneKey)).not.toBe(family)
    // An embedded "Inter" and a shipped "Inter" are two different families
    // even though the document calls them the same thing, because the key
    // decides and the key is the hash of the bytes.
    expect(embeddedFaceFamily(oneKey)).not.toBe('Inter')
    expect(embeddedFaceFamily(otherKey)).not.toBe('Inter')
  })

  // The engine's reserved namespace for the SAME asset is `asset:<key>`, and
  // that prefix is spelled in one Go file and must not be spelled a second
  // time in this language. If this name ever started with it, the browser
  // would be carrying a copy of an engine-internal derivation.
  it('is the browser\'s own namespace, not the engine\'s', () => {
    expect(embeddedFaceFamily(oneKey).startsWith('asset')).toBe(false)
    expect(embeddedFaceFamily(oneKey)).toContain(oneKey)
  })

  // It is written into a CSS font-family position, so it has to be a legal
  // custom-ident: no quoting, no escaping, nothing that could terminate the
  // declaration it is placed in.
  it('is a CSS identifier that needs no quoting', () => {
    expect(embeddedFaceFamily(oneKey)).toMatch(/^[A-Za-z][A-Za-z0-9-]*$/)
  })

  // AND "BY CONSTRUCTION" IS ONLY TRUE OF A KEY THAT HAS THE SHAPE, so the
  // shape is a predicate this module owns rather than an assumption about its
  // callers. The family lands in an INLINE `font-family` declaration; a key
  // carrying a quote, a semicolon or a brace would be a string injected into a
  // stylesheet. The FRAGMENT's key is admitted as 64 lowercase hex at the
  // protocol boundary — a chain ENTRY's key, which is the one the registration
  // effect derives a family from, is admitted on length alone.
  it('admits exactly the 64-lowercase-hex shape the engine mints, and nothing else', () => {
    expect(isCarriedFaceAssetKey(oneKey)).toBe(true)
    expect(isCarriedFaceAssetKey(otherKey)).toBe(true)
    for (const malformed of [
      '',
      'a'.repeat(63),
      'a'.repeat(65),
      'A'.repeat(64),
      `${'a'.repeat(63)}Z`,
      `${'a'.repeat(60)}"; }`,
      `${'a'.repeat(58)}, serif`,
      `${'a'.repeat(63)} `,
      ` ${'a'.repeat(64)}`,
      `${'a'.repeat(64)}\n${'b'.repeat(64)}`,
      'asset:0123456789abcdef',
      'Inter',
    ]) {
      expect(isCarriedFaceAssetKey(malformed)).toBe(false)
    }
  })

  // Every admitted key produces a name that needs no quoting — the property
  // above, asserted over the predicate rather than over one hand-written key.
  it('turns every admitted key, and only those, into an unquotable identifier', () => {
    for (const key of [oneKey, otherKey, '0'.repeat(64), 'f'.repeat(64)]) {
      expect(isCarriedFaceAssetKey(key)).toBe(true)
      expect(embeddedFaceFamily(key)).toMatch(/^[A-Za-z][A-Za-z0-9-]*$/)
    }
  })
})
