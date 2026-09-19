// THE ONE RULE THAT TURNS A DOCUMENT'S OWN ASSET INTO A CSS FAMILY NAME
// (Story 8.4a, D-8.4.1).
//
// The engine measures a carried face and tells the canvas, per PAINT
// FRAGMENT, which of the document's assets it resolved that fragment to. The
// browser has to ask for that face by a CSS family name, and this module is
// the whole of how that name is decided.
//
// IT DERIVES FROM THE ASSET KEY AND NEVER FROM `font.family`. AD-8 makes the
// asset key the resolver: an embedded face and a shipped face that share a
// family name must never substitute for one another, and `font.family` /
// `font.style` are DISPLAY identity — what the font panel prints, never how a
// face is found. `document.fonts` is a GLOBAL, NAME-KEYED registry, so a
// family derived from `font.family` would let a document's "Inter" collide
// with the build's own "Inter" in exactly the registry AD-8 keeps them out of,
// one layer below where AD-8 was written. The key is the content hash of the
// face's bytes, so two distinct faces cannot share a family and one face
// cannot acquire two.
//
// THE PREFIX IS NOT THE ENGINE'S. Go mints its own reserved face name for the
// same asset (`embedded_face.go`), and that name's prefix is spelled in that
// one Go file and nowhere else — in Go or in TypeScript. Spelling it here
// would be writing that derivation a second time, in a second language, that
// no Go test can pin. So the wire carries the ASSET KEY, both sides derive
// their own name from it, and this prefix is the browser's alone. It is
// deliberately in a namespace no design-system family could occupy.
//
// THE RESULT IS A CSS <custom-ident> BY CONSTRUCTION: an asset key is 64
// lowercase hex characters, so the whole name is ASCII letters, digits and
// hyphens beginning with a letter — nothing to escape and nothing to quote.
//
// "BY CONSTRUCTION" IS A CLAIM ABOUT THE INPUT, SO THE INPUT IS CHECKED.
// The claim above is only true of a key that really is 64 lowercase hex
// characters, and this family is written into an INLINE `font-family`
// declaration, which makes an unchecked key a string-injection path into a
// stylesheet. The projection guards a FRAGMENT's `assetKey` to exactly this
// shape (engine-protocol.ts), but a CHAIN ENTRY's key — the one the
// registration effect fetches bytes for and derives a family from — is
// admitted on length alone, so the shape is asserted here, at the derivation,
// for every key that reaches it in production.
//
// IT IS A PREDICATE AND NOT A THROW. A key that does not have the shape is a
// carried face the browser declines to register: the fragment keeps the
// stylesheet's declared stack and the canvas keeps painting, exactly as a
// failed fetch does. Refusing louder would turn a document fact into a
// session fault.
const embeddedFaceFamilyPrefix = 'folio8-carried-'

const carriedFaceAssetKey = /^[a-f0-9]{64}$/

export function isCarriedFaceAssetKey(assetKey: string): boolean {
  return carriedFaceAssetKey.test(assetKey)
}

export function embeddedFaceFamily(assetKey: string): string {
  return `${embeddedFaceFamilyPrefix}${assetKey}`
}
