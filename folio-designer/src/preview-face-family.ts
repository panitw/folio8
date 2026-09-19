// THE ONE RULE THAT TURNS A BROWSABLE FAMILY NAME INTO A CSS FAMILY THE FONT
// BROWSER'S SPECIMENS ARE SET IN (Story 16.3).
//
// IT IS THE THIRD LIFETIME IN THIS DESIGNER AND IT IS ARGUED, NOT ASSUMED.
// `embedded-face-family.ts` names a face THE OPEN DOCUMENT CARRIES; the
// registration behind it lives as long as that document. `shipped-face-family.ts`
// names a face THE BUILD SHIPS; that one is declared at build time and never
// released. This names a face NOTHING OWNS YET — a family the author is looking
// at in a browser, which may never enter any document at all. Its registration
// lives as long as the row is on screen, and `preview-face-registry.ts` is where
// that lifetime is written down.
//
// THE WHOLE POINT IS THAT IT CANNOT COLLIDE WITH THE OTHER TWO. `document.fonts`
// is a GLOBAL, NAME-KEYED registry — which is the hazard D-8.4.1 was written
// around — so a preview face registered under a name the canvas also asks for
// would let a modal's scroll position decide what the page paints. The prefix
// below is disjoint from `folio8-carried-` by its own text and disjoint from
// every shipped face name by having a prefix at all, and
// `preview-face-registry.test.ts` asserts both directions rather than trusting
// the reading.
//
// AND IT IS NEVER A SECOND AUTHORITY ON WHAT A DOCUMENT CONTAINS. A family with
// a preview face is not embedded, is not in `fontFamilies`, and this module
// produces nothing that any command, any projection or any `.folio` ever sees.
// It produces a CSS token for one `<span>` in one modal.
//
// THE SUFFIX IS AN INJECTIVE ENCODING OF THE FAMILY NAME, NOT A SLUG. A slug
// folds `Foo Bar` and `Foo-Bar` onto one name, and two families sharing one
// registry name is exactly the silent substitution the content-addressed key
// exists to refuse one layer up. Lowercase hex of the name's UTF-8 bytes is
// injective by construction and is, by the same construction, an ASCII
// `<custom-ident>`: digits and the letters a-f only, behind a prefix that starts
// with a letter. Nothing to escape and nothing to quote, which matters because
// this value is written into an INLINE `font-family` declaration.
//
// IT IS A PREDICATE AND NOT A THROW, exactly as its two siblings are. A name
// this module declines is a family whose specimen is not set in itself — the row
// says so in words rather than quietly rendering the sample in the panel's own
// typeface, which is the one thing the story's contract forbids outright.
const previewFaceFamilyPrefix = 'folio8-preview-'

// A BOUND ON THE INPUT, because the output goes into a stylesheet.
// The longest family in the shipped snapshot is well under this; the number is
// a ceiling on what may be encoded, not a measurement of what is there.
const maxPreviewFamilyNameLength = 128

export function isPreviewableFamilyName(family: string): boolean {
  return family.length > 0 && family.length <= maxPreviewFamilyNameLength
}

/**
 * The CSS family a specimen of `family` is set in, or `undefined` when the name
 * is one this module declines to encode.
 */
export function previewFaceFamily(family: string): string | undefined {
  if (!isPreviewableFamilyName(family)) return undefined
  let hex = ''
  for (const byte of new TextEncoder().encode(family)) hex += byte.toString(16).padStart(2, '0')
  return `${previewFaceFamilyPrefix}${hex}`
}
