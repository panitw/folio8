// THE ONE RULE THAT TURNS A SHIPPED FACE'S ENGINE NAME INTO A CSS
// FONT-FAMILY VALUE (Story 8.4e, D-8.4.14).
//
// It is `embedded-face-family.ts`'s twin, for the other population. The engine
// measures every fragment with exactly one face and tells the canvas which:
// the ASSET KEY when the document carries the face, and — since Story 8.4e —
// the FontSet NAME when the build ships it. The browser has to ask for that
// face by a CSS family, and this module is the whole of how that value is
// decided for the shipped half.
//
// THE NAME IS THE FAMILY. NOTHING IS MAPPED. D-8.4.14 settled it: "a carried
// face's browser family derives from the engine's identity for it (the asset
// key); a shipped face's from the engine's identity for it (the FontSet name).
// One rule for one question." Story 8.4b declares an `@font-face` under each
// of the engine's own face names over the engine's own bytes, so the name the
// engine measured with is already a family the browser can resolve. The two
// alternatives were rejected there BY NAME — renaming the generated families
// (IBM Plex is the design system's specified typeface, not the engine's to
// rename) and a face-name -> CSS-family table (a second authority maintained
// in lockstep with `fonts.Shipped()`). A table is what this module is NOT.
//
// AND NEVER FROM A CHAIN ENTRY'S `family` OR `style`. AD-8 and D-8.4.1 make
// the engine's identity the resolver; `family`/`style` are DISPLAY identity —
// what the font panel prints. This module reads neither, and it never reads a
// chain, a chain name or the projection's `fontFamily` field.
//
// WHY THE VALUE CARRIES A TAIL AT ALL. An inline `font-family` REPLACES the
// stylesheet rule rather than extending it, so a bare `'Noto Sans Thai'` would
// take the fragment OFF the declared stack: a codepoint that face does not
// cover would fall to the browser's default rather than to the other two
// shipped faces. The attributed face goes FIRST — that is the whole point,
// since a CSS stack is a first-match-wins search per codepoint and the three
// shipped faces' cmaps overlap (339 / 529 / 230 codepoints pairwise, all three
// covering `A` and `5`) — and the rest of the declared stack follows it
// unchanged.
//
// THE TAIL HAS ONE AUTHORITY, AND IT IS TIED TO THE STYLESHEET. The same list
// is spelled in `.canvas-text-fragment`'s rule in App.css, which stays as the
// fallback for an UNATTRIBUTED fragment (and is where three separate guards
// read the engine's face order from). Two spellings of one list is two lists,
// so `canvas-font-stack.test.ts` reads both sources and asserts they are the
// same sequence, entry for entry.
//
// "USABLE AS A CSS FAMILY" IS A CLAIM ABOUT THE INPUT, SO THE INPUT IS
// CHECKED. This value is written into an INLINE `font-family` declaration, and
// the engine's own guarantee — that the name can only be a key of the FontSet
// the build was given — is the ENGINE's, not the browser's. An unchecked name
// is a string-injection path into a stylesheet, so the shape is asserted here,
// at the derivation, for every name that reaches it. The admitted shape is a
// quotable family literal: letters and digits in space- or hyphen-separated
// words, which is what all three shipped names are and what no quote,
// backslash, comma, semicolon, brace or newline can be.
//
// IT IS A PREDICATE AND NOT A THROW, exactly as the carried side is. A name
// that does not have the shape is a face the browser declines to name: the
// fragment keeps the stylesheet's declared stack and the canvas keeps
// painting. Refusing louder would turn an engine fact into a session fault.
import { MAX_CANVAS_PROPERTY_STRING } from './engine-protocol'

// The fallback tail, in the exact CSS spelling `.canvas-text-fragment` uses:
// quoted families first, in `fonts.Shipped()`'s own order, then the generic
// keyword last. Tied to App.css by a guard that reads both sources.
export const canvasFragmentFallbackStack: ReadonlyArray<string> = ["'Noto Sans'", "'Noto Sans Thai'", "'Noto Sans SC'", 'sans-serif']

const shippedFaceName = /^[A-Za-z][A-Za-z0-9]*(?:[ -][A-Za-z0-9]+)*$/

export function shippedFaceFamily(face: string | undefined): string | undefined {
  if (face === undefined || face.length > MAX_CANVAS_PROPERTY_STRING || !shippedFaceName.test(face)) return undefined
  const first = `'${face}'`
  return [first, ...canvasFragmentFallbackStack.filter((entry) => entry !== first)].join(', ')
}

// isShippedFaceName is the call-site predicate, defined AS the derivation's own
// answer so the two can never disagree: there is one rule, asked twice.
export function isShippedFaceName(face: string | undefined): boolean {
  return shippedFaceFamily(face) !== undefined
}
