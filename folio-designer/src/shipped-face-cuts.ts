// THE FAMILY → CUTS TABLE FOR THE FACES THIS RELEASE SHIPS, AND IT IS THE
// PRODUCT'S FIRST DECLARED PRODUCTION GROUPING (Story 11.4).
//
// WHAT IT ANSWERS. `fonts.Shipped()` is a FLAT map of eleven face names, and
// its own comment says why: "NEVER PARSE ONE OF THESE KEYS (D-B). They are
// readable strings for a human writing a chain, not an encoding." So the
// engine knows `Roboto` and `Roboto Bold` are both faces and knows NOTHING
// about the two being one family's regular and bold cuts. That grouping is
// what a pick has to write into a chain entry, and before this module the only
// place in the tree that held it was one hand-authored document
// (`public/templates/starter.folio`).
//
// ⚠ IT IS HAND-WRITTEN DATA, AND IT MUST NOT READ AS A NAMING CONVENTION.
// D-11.2.1 / D-11.2.2: resolution is DECLARED, never constructed, parsed or
// inferred from a face name. `"Roboto" + " Bold"` is the foreclosed carrier
// written forwards and `TrimSuffix(key, " Bold")` is the same thing backwards;
// that all seven cuts below happen to read as their base plus a suffix is a
// fact about how upstream names faces, NOT the rule that produced this table.
// A family whose bold cut were named `Roboto Semibold Display` would take one
// more row here and no new code anywhere. Nothing below is derived from a
// string, and nothing may become derived from one.
//
// ⚠ AND IT IS NOT THE TABLE D-8.4.14 REFUSED. `shipped-face-family.ts`, its
// deliberate neighbour, says at its head that "a face-name → CSS-family table
// (a second authority maintained in lockstep with `fonts.Shipped()`) … is what
// this module is NOT". That refusal is about NAMING — which browser family a
// face is asked for by — and it was refused because the engine's own identity
// for the face already answers it, so a table there would have been a second
// answer to a question already answered. This table answers GROUPING: which
// faces are one family's cuts. Nothing in the engine answers that at all, in
// either direction, so there is no first authority for this one to be a second
// copy of. The two modules sit side by side so a reader meets both statements
// at once.
//
// WHY IT IS TRACKED SOURCE AND NOT GENERATED. `scriptFallbacks` in
// `scripts/build-wasm.mjs` is the closest precedent — hand-declared beside the
// shipped face list and emitted into `src/generated/font-catalogue.ts`. But
// `src/generated/` is gitignored, so a generated mirror never appears in a
// diff, and this table is precisely the artifact a reviewer has to check by
// eye against `fonts.Shipped()`. A tracked module also gives the tie test two
// SOURCES to compare, which is what `engine-bounds-mirror.test.ts` does.
//
// THE TIE IS `shipped-face-cuts.test.ts`: the bases plus the cuts below equal
// `fonts.Shipped()`'s key set, asserted BOTH ways. Four bases and seven cuts
// is eleven, which is that map exactly. A one-directional tie cannot see a
// face that LEAVES the FontSet, so neither direction is optional.
import type { FontChainEntryAsk } from './font-chain-command'

/**
 * One shipped family and the cuts it has.
 *
 * `family` is the base face's own `fonts.Shipped()` key. For all four families
 * the family name and the base face name are the same string — that is a
 * measured property of the shipped set, not a rule this module applies, and a
 * family whose base face were keyed differently would carry both spellings
 * here rather than have one derived from the other.
 *
 * An ABSENT key is an absent cut, and absence is a first-class answer: it is
 * what the engine answers with the base face plus a Warning (AC3), and what
 * the B and I controls state in words. `Noto Sans SC` declares nothing at all
 * and that is D-A, a permanent shipped condition — its Regular alone is
 * ~10.6 MB, so three instances were ruled out for the offline payload — not a
 * row somebody has yet to fill in.
 */
export type ShippedFamilyCuts = Readonly<{
  family: string
  bold?: string
  italic?: string
  boldItalic?: string
}>

/**
 * THE MIRROR. Every value is a `fonts.Shipped()` key, written out.
 *
 * Ordered as `Shipped()` writes them, so the two read side by side.
 */
export const shippedFamilyCuts: ReadonlyArray<ShippedFamilyCuts> = [
  { family: 'Noto Sans', bold: 'Noto Sans Bold', italic: 'Noto Sans Italic', boldItalic: 'Noto Sans Bold Italic' },
  { family: 'Noto Sans Thai', bold: 'Noto Sans Thai Bold' },
  // D-A. Declares no cut, and exercising that is ORDINARY behaviour.
  { family: 'Noto Sans SC' },
  { family: 'Roboto', bold: 'Roboto Bold', italic: 'Roboto Italic', boldItalic: 'Roboto Bold Italic' },
]

/**
 * THE MEMBERSHIP TEST FOR "THIS RELEASE ALREADY SHIPS THAT FAMILY", and it is
 * membership in the declared mirror — never `build-wasm.mjs`'s
 * `shippedFamilies`, never a parse of `fonts.go`, never a byte comparison.
 *
 * ⚠ MEASURED: `shippedFamilies` is wrong in BOTH directions for this question.
 * It omits plain `Roboto` and it includes `IBM Plex Sans`, `IBM Plex Mono` and
 * `IBM Plex Sans Thai`, which have no `Shipped()` key at all. It is the
 * BROWSER'S CSS family registry — which families the designer declares
 * `@font-face` rules for — and asking it this question would answer "Roboto is
 * not shipped" and would write `{"face": "IBM Plex Sans"}` into a document the
 * engine then refuses to render with.
 */
export function shippedFamilyCutsOf(family: string): ShippedFamilyCuts | undefined {
  return shippedFamilyCuts.find((row) => row.family === family)
}

export function isShippedFamily(family: string): boolean {
  return shippedFamilyCutsOf(family) !== undefined
}

/**
 * The chain entry a pick of `family` declares — the shipped face NAMED, with
 * its cuts, and no asset embedded.
 *
 * It returns the format's own chain-entry shape, so nothing downstream has to
 * know that a family has cuts at all: a family with none comes back as the
 * bare face name, which is exactly what a variant-free entry serialises to
 * (`{"face": "X"}` and `"X"` are the same entry, and the bare string is
 * canonical — a chain of bare strings keeps a document at `1.0`).
 *
 * `undefined` for a family this release does not ship: that pick embeds, and
 * embedding is a different path with a different command.
 */
export function shippedFamilyEntry(family: string): FontChainEntryAsk | undefined {
  const cuts = shippedFamilyCutsOf(family)
  if (cuts === undefined) return undefined
  // ⚠ BUILT FROM THE CUTS THAT EXIST, NEVER FROM ALL THREE KEYS. Spreading the
  // row wholesale produced `{face, bold: undefined, italic: undefined,
  // boldItalic: undefined}` — present keys holding `undefined` — which
  // contradicts this module's own stated convention that an ABSENT KEY is an
  // absent cut, and it is a difference anything reading the object by
  // `Object.keys`, `in`, or a structural comparison can see even though the
  // command encoder happens to skip `undefined` values. The convention is the
  // thing this module exists to state; it may not be true only by the grace of
  // one downstream encoder.
  const entry: { face: string; bold?: string; italic?: string; boldItalic?: string } = { face: cuts.family }
  for (const key of ['bold', 'italic', 'boldItalic'] as const) {
    const cut = cuts[key]
    if (cut !== undefined) entry[key] = cut
  }
  // A family with no cut at all is the BARE FACE NAME. `{"face": "X"}` and
  // `"X"` are the same entry and the bare string is canonical, so a cut-less
  // pick keeps the document at `1.0` rather than stamping `2.0` on a chain no
  // 1.x reader would have had trouble with.
  if (Object.keys(entry).length === 1) return cuts.family
  return entry
}
