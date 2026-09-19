import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { shippedFamilyCuts, shippedFamilyCutsOf, shippedFamilyEntry, isShippedFamily } from './shipped-face-cuts'

// THE MIRROR TIE: `shipped-face-cuts.ts` against `fonts.Shipped()`, BOTH WAYS.
//
// The idiom is `engine-bounds-mirror.test.ts`'s, copied rather than invented:
// read the Go source as TEXT, declare what is being tied, gate NON-VACUITY
// before claiming any equality from it, then assert the equality. The
// shipped-face precedent for the parse itself is `canvas-font-stack.test.ts`'s
// `shippedFaceNames()`, which pulls `Shipped()`'s keys out of `fonts.go` with
// the same two regexes used below.
//
// ⚠ THE PARSE IS TEST-SIDE AND MUST STAY TEST-SIDE (D-11.1.24's trap, and it
// has already cost this epic one reworded acceptance criterion). Production
// holds its own DECLARED copy — that is what `shipped-face-cuts.ts` is — and a
// TEST ties it to Go. Pointing production at a Go-source parse would make the
// browser's answer depend on a file it cannot read at runtime.
//
// ⚠ AND THE TIE IS ASSERTED IN BOTH DIRECTIONS, which is not a formality. A
// one-directional check ("every name the mirror writes is a Shipped() key")
// stays green when a face LEAVES the FontSet's other side — the mirror would
// simply stop covering it, and the designer would go on offering a family whose
// cuts the engine no longer ships.

const here = path.dirname(fileURLToPath(import.meta.url))
const enginePath = path.join(here, '..', '..', 'folio-go', 'fonts', 'fonts.go')
// The binary-verified witness. `shippedSlotFaces` is a closed 13-row table in
// this test file whose family/subfamily/usWeightClass are checked against what
// the committed binaries actually say about themselves — so it is where "Roboto
// Bold really is Roboto's Bold" is established from the bytes. It is TEST-ONLY
// and unimportable by production, and it is read here as source text for the
// same reason the Go file is.
const catalogueTestPath = path.join(here, 'font-catalogue.test.ts')

/** The face names `fonts.Shipped()` keys its FontSet by, in the order it writes them. */
function shippedFaceNames(fontsGo: string): ReadonlyArray<string> {
  const body = /func Shipped\(\) folio8\.FontSet \{[\s\S]*?\n\}/.exec(fontsGo)?.[0]
  if (body === undefined) throw new Error(`no Shipped() function in ${enginePath}`)
  return [...body.matchAll(/"([^"]+)":\s*\w+,/g)].map((m) => m[1])
}

/** Every face name the mirror writes down: the four bases, then the seven cuts. */
function mirrorFaceNames(): ReadonlyArray<string> {
  return shippedFamilyCuts.flatMap((row) => [row.family, row.bold, row.italic, row.boldItalic].filter((name): name is string => name !== undefined))
}

/**
 * `shippedSlotFaces`' rows, read out of the sibling test file as text.
 *
 * Only three columns are taken: the CSS family (which IS the engine's face
 * name — `shipped-face-family.ts`'s "the name is the family"), and the family
 * and subfamily the BINARY calls itself. Those last two are what make this a
 * witness rather than a third hand-written list: the row asserts them against
 * name IDs read off the committed bytes with fontTools.
 */
function binaryVerifiedSlots(source: string): ReadonlyArray<{ face: string; family: string; subfamily: string }> {
  return [...source.matchAll(/\{ cssFamily: '([^']+)', family: '([^']+)', subfamily: '([^']+)'/g)]
    .map((m) => ({ face: m[1], family: m[2], subfamily: m[3] }))
}

/**
 * The join between a binary's SUBFAMILY and the variant key a chain entry
 * declares it under.
 *
 * ⚠ THIS IS A TEST-SIDE JOIN AND IT MAY NEVER BECOME A PRODUCTION RULE. It
 * relates two tables that already exist; it does not DERIVE either of them, and
 * nothing production does consults a subfamily string at all (D-11.2.1: "Roboto"
 * + " Bold" is the foreclosed carrier written forwards, and reading "Bold" back
 * out of a name is the same thing backwards). It is here so a face that changes
 * which cut it IS reds, rather than passing because both tables were edited to
 * agree with each other.
 */
const cutKeyOfSubfamily: Readonly<Record<string, 'base' | 'bold' | 'italic' | 'boldItalic'>> = {
  Regular: 'base',
  Bold: 'bold',
  Italic: 'italic',
  'Bold Italic': 'boldItalic',
}

describe('the shipped family→cuts mirror', () => {
  const fontsGo = fs.readFileSync(enginePath, 'utf8')
  const catalogueTest = fs.readFileSync(catalogueTestPath, 'utf8')

  it('reads a non-vacuous set out of both sources before claiming any equality between them', () => {
    // A regex that quietly stops matching is the exact failure a tie assertion
    // exists to prevent, so the found sets are asserted whole first. Every
    // check below is over a list, and two empty lists are equal.
    const engine = shippedFaceNames(fontsGo)
    expect(engine.length, `read no face names out of Shipped() in ${enginePath}`).toBe(11)
    expect(new Set(engine).size, 'Shipped() declares a duplicate key').toBe(11)
    expect(shippedFamilyCuts).toHaveLength(4)
    expect(mirrorFaceNames()).toHaveLength(11)
    // FOUR BASES AND SEVEN CUTS IS ELEVEN, and that arithmetic is the tie's
    // whole shape — spelled out so a reader can check it by eye against the
    // eleven keys above.
    expect(shippedFamilyCuts.filter((row) => row.bold !== undefined || row.italic !== undefined || row.boldItalic !== undefined)).toHaveLength(3)
    expect(mirrorFaceNames().length - shippedFamilyCuts.length, 'the mirror declares seven cuts').toBe(7)
    // And the binary-verified witness really was found.
    expect(binaryVerifiedSlots(catalogueTest).length, `read no shippedSlotFaces rows out of ${catalogueTestPath}`).toBe(13)
  })

  it('holds the mirror and fonts.Shipped() at the same face set, asserted BOTH ways', () => {
    const engine = [...shippedFaceNames(fontsGo)].sort()
    const mirror = [...mirrorFaceNames()].sort()
    // Direction 1: the mirror names nothing the engine does not ship — a pick
    // would otherwise write a cut into a document the engine cannot draw, and
    // the author would get a base face and a Warning for a cut they were shown.
    expect(mirror.filter((name) => !engine.includes(name)), 'the mirror names faces fonts.Shipped() does not supply').toEqual([])
    // Direction 2: the engine ships nothing the mirror has lost. THIS is the
    // direction a one-sided tie cannot see.
    expect(engine.filter((name) => !mirror.includes(name)), 'fonts.Shipped() supplies faces the mirror never mentions, so a pick can no longer reach them').toEqual([])
    // And as one equality, so a reader sees the sets rather than two empty
    // difference lists.
    expect(mirror).toEqual(engine)
  })

  it('reds when the Go source moves, proved on a SYNTHETIC copy and never on the real file', () => {
    // MUTATION, BY DELETION, ON A COPY. The real fonts.go is never written to;
    // the text is edited in memory and the same comparison is re-run over it.
    const withoutRobotoBold = fontsGo.replace(/^.*"Roboto Bold":.*$\n/m, '')
    expect(withoutRobotoBold, 'the deletion did not change the source, so the mutation below proves nothing').not.toBe(fontsGo)
    const shrunk = shippedFaceNames(withoutRobotoBold)
    expect(shrunk).toHaveLength(10)
    expect(mirrorFaceNames().filter((name) => !shrunk.includes(name)), 'a face leaving the FontSet must red direction 1').toEqual(['Roboto Bold'])

    // And the other direction: a face ARRIVING in the FontSet with no row in
    // the mirror must red too.
    const withAnExtra = fontsGo.replace('"Roboto Bold":', '"Roboto Semibold": robotoBold,\n\t\t"Roboto Bold":')
    const grown = shippedFaceNames(withAnExtra)
    expect(grown).toHaveLength(12)
    expect(grown.filter((name) => !mirrorFaceNames().includes(name)), 'a face arriving in the FontSet must red direction 2').toEqual(['Roboto Semibold'])
  })

  it('agrees with the binary-verified witness about which family each cut belongs to', () => {
    // The witness answers a question the name parse above cannot: `Roboto Bold`
    // is Roboto's BOLD because its own name table says family Roboto, subfamily
    // Bold and its OS/2 says weight 700 — not because the string ends in "Bold".
    const slots = binaryVerifiedSlots(catalogueTest).filter((slot) => isShippedFamily(slot.family))
    // Non-vacuity, and the ONE face this witness does not cover, named rather
    // than absorbed: the filter drops the three IBM Plex design-system slots,
    // which have no `Shipped()` key at all, leaving TEN. Roboto's own Regular
    // is not a hardcoded slot — it is one of the 31 CATALOGUE faces, and the
    // engine's copy of it is tied to the designer's by
    // `TestShippedRobotoMatchesDesignerCatalogue`, which makes "there is
    // exactly one Roboto" machine-checked on the bytes.
    expect(slots).toHaveLength(10)
    for (const slot of slots) {
      const key = cutKeyOfSubfamily[slot.subfamily]
      expect(key, `no cut key for subfamily ${slot.subfamily}`).toBeDefined()
      const row = shippedFamilyCutsOf(slot.family)
      expect(row, `${slot.family} is not in the mirror`).toBeDefined()
      if (row === undefined || key === undefined) continue
      expect(key === 'base' ? row.family : row[key], `${slot.family}'s ${slot.subfamily} cut`).toBe(slot.face)
    }
    // AND THE OTHER WAY: every face the mirror names has a witnessed row, with
    // `Roboto` the single stated exception above. Asserting the LEFTOVER SET
    // rather than skipping it is what keeps the exception one face wide — a
    // second uncovered name reds here instead of joining it silently.
    const witnessed = new Set(slots.map((slot) => slot.face))
    expect(mirrorFaceNames().filter((name) => !witnessed.has(name)), 'the mirror names a face no binary-verified row covers').toEqual(['Roboto'])
  })

  it('declares no variant equal to its own base face', () => {
    // D-11.2.11 / DW-241: such a variant is a load error since Story 11.4, so a
    // mirror row carrying one would make every pick of that family author a
    // document the product cannot reopen. Asserted directly rather than left to
    // the engine to catch after the fact.
    for (const row of shippedFamilyCuts) {
      for (const cut of [row.bold, row.italic, row.boldItalic]) {
        if (cut === undefined) continue
        expect(cut, `${row.family} declares a cut naming its own base face`).not.toBe(row.family)
      }
    }
    // Non-vacuity: there really are cuts to compare.
    expect(shippedFamilyCuts.flatMap((row) => [row.bold, row.italic, row.boldItalic]).filter((cut) => cut !== undefined)).toHaveLength(7)
  })

  it('turns a family into the chain entry a pick declares, and a family with no cut into a bare name', () => {
    expect(shippedFamilyEntry('Roboto')).toEqual({ face: 'Roboto', bold: 'Roboto Bold', italic: 'Roboto Italic', boldItalic: 'Roboto Bold Italic' })
    expect(shippedFamilyEntry('Noto Sans Thai')).toEqual({ face: 'Noto Sans Thai', bold: 'Noto Sans Thai Bold' })
    // ⚠ AN ABSENT CUT IS AN ABSENT KEY, AND `toEqual` CANNOT SEE THE DIFFERENCE.
    // This row read `{ face, bold, italic: undefined, boldItalic: undefined }`
    // and passed — because `toEqual` treats a key holding `undefined` as equal
    // to no key at all, which is precisely the distinction this module's own
    // head comment says it makes. The builder was in fact returning
    // present-but-`undefined` keys, and no assertion here could tell. The key
    // set is read directly so the convention is asserted rather than assumed.
    expect(Object.keys(shippedFamilyEntry('Noto Sans Thai') as object).sort(), 'an absent cut is an ABSENT KEY, never a key holding undefined').toEqual(['bold', 'face'])
    expect(Object.keys(shippedFamilyEntry('Roboto') as object).sort()).toEqual(['bold', 'boldItalic', 'face', 'italic'])
    // AND IT IS OBSERVABLE, which is why it is worth a rule: `'italic' in entry`
    // and a structural comparison both answer differently for the two shapes,
    // and only the command encoder happens not to care.
    expect('italic' in (shippedFamilyEntry('Noto Sans Thai') as object)).toBe(false)
    // D-A, AND IT IS ORDINARY BEHAVIOUR, not an edge case: Noto Sans SC has no
    // face at any other weight and never will, so its entry declares nothing
    // and stays the bare face name a variant-free entry canonicalises to.
    expect(shippedFamilyEntry('Noto Sans SC')).toBe('Noto Sans SC')
    // A family this release does not ship has no entry here at all — that pick
    // embeds, which is a different path with a different command.
    expect(shippedFamilyEntry('Inter')).toBeUndefined()
    expect(isShippedFamily('Inter')).toBe(false)
    // ⚠ AND THE MEMBERSHIP TEST IS NOT `build-wasm.mjs`'s `shippedFamilies`,
    // which is measurably wrong in BOTH directions for this question: it omits
    // plain Roboto and includes three IBM Plex families with no Shipped() key.
    expect(isShippedFamily('Roboto')).toBe(true)
    expect(isShippedFamily('IBM Plex Sans')).toBe(false)
  })
})
