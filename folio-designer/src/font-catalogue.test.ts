import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
// THE GENERATED MODULE IS A SUBJECT HERE, not a convenience. Nothing observed
// it before, which is exactly how a build that gave 17 of 21 faces another
// project's licence text shipped green: `font-catalogue.json` was right, every
// binary was right, and the artifact BETWEEN them — the only thing the pick
// actually reads — was checked by nothing.
import { catalogueFaces as generatedFaces } from './generated/font-catalogue'
// THE SHARED sfnt `name`-TABLE READER (Story 16.1). This file used to write out
// its own `DataView` walk, byte-identical to a second copy in
// `scripts/build-wasm.mjs`; Story 16.1 needed a THIRD at runtime, over bytes
// fetched from a third party, and extracted the walk here instead of adding one.
import { nameTableString, requireStaticTrueTypeTables, type SfntTable } from './font-name-table'

// STORY 8.5 — THE CATALOGUE, HELD TO ITS OWN RECORD.
//
// `font-catalogue.json` is the single declaration of which faces ship
// (Design Note 4): `scripts/build-wasm.mjs` loops over it, fingerprints each
// binary into `src/generated/runtime/`, and emits one `@font-face` rule per
// entry. That makes the manifest a claim about twenty-one committed binaries,
// and this file is where the claim meets the bytes.
//
// THREE CLAIMS, ONE PER ACCEPTANCE CRITERION:
//
//   AC1 — every face travels with the unmodified upstream `LICENSE*` and a
//   `NOTICE.md` recording the pinned upstream version, the upstream archive
//   digest, the committed digest, the byte size, the fetch date and the path
//   inside the archive; and `shasum -a 256` of the binary EQUALS the digest its
//   own NOTICE records. A provenance record nothing checks becomes a false
//   statement the first time a binary is swapped — and swapping a weight or an
//   italic keeps every name-table check green, which is exactly why the digest
//   is the tie.
//
//   AC6 — RETIRED AND REPLACED, NOT DELETED (spec-install-all-face-cuts,
//   story 3). It read: "every face is a SINGLE UPRIGHT STATIC REGULAR — no
//   bold, no italic, no oblique, no variable axis", and that was a true
//   statement about a catalogue that held one Regular per family. The committed
//   tier now carries every cut its 31 families publish, so the invariant is
//   re-stated per row rather than dropped: a row declares a `style`, and the
//   binary's own `name`, `OS/2`, `head` and `post` tables must agree with it —
//   subfamily, `usWeightClass`, the fsSelection BOLD/ITALIC/REGULAR bits,
//   `macStyle` and `italicAngle`. A row mislabelled `Bold` over a Regular
//   binary reds naming the row, its declared style and the subfamily read from
//   the bytes. What SURVIVES unchanged is the other half of the old claim, and
//   it is the half that is still absolute: NO VARIABLE AXIS, and glyf/`.ttf`
//   outlines only. The generated `@font-face` rules still declare no
//   `font-weight` and no `font-style` — a cut is its own CSS family name, as
//   the thirteen hand-written rules already do it.
//
//   AC3 — at least twenty NEW families beyond the six already shipped, each
//   with bytes of its own.
//
//   STORY 8.6 ADDED A FOURTH: `scripts` — what each face covers, which the
//   designer proposes a fallback tail from. It is checked against that face's
//   OWN `cmap`, in BOTH directions: a script the manifest claims and the
//   binary cannot draw would give a document a chain with no fallback for
//   runes nothing in it covers, and a script the binary DOES cover that the
//   manifest omits would staple a redundant shipped face onto every chain
//   picking it. Both are silent; neither is visible in a rendered page until
//   somebody types in that script.

const here = path.dirname(fileURLToPath(import.meta.url))
const designerRoot = path.join(here, '..')
const cataloguePath = path.join(designerRoot, 'font-catalogue.json')
const generatorPath = path.join(designerRoot, 'scripts', 'build-wasm.mjs')
const fontsRoot = path.join(designerRoot, 'public', 'fonts')

/**
 * The families the generator declares BY HAND, which the catalogue must not
 * collide with — DERIVED from this file's own `shippedSlotFaces` table below
 * rather than re-typed beside it.
 *
 * IT WAS A SECOND COPY AND IT HAD ALREADY DRIFTED. This list stood at the six
 * pre-Story-11.1 families while `scripts/build-wasm.mjs` widened its own
 * `shippedFamilies` to THIRTEEN, so a `font-catalogue.json` entry redeclaring
 * `Roboto Bold` satisfied the collision assertion below and was refused only
 * by the generator's build-time throw. Story 11.1 then put a thirteen-row slot
 * table in this same file, making the disagreement internal to one file — so
 * the copy is deleted and the population read off the table instead.
 *
 * A FUNCTION, not a `const`: `shippedSlotFaces` is declared further down and
 * this would be a temporal-dead-zone read at module scope. Every caller is
 * inside a test body, which runs after the whole module has evaluated.
 */
const shippedFamilies = (): ReadonlyArray<string> => Object.values(shippedSlotFaces).map((face) => face.cssFamily)

interface CatalogueFace { id: string; directory: string; file: string; family: string; style: string; licence: string; scripts: ReadonlyArray<string> }

/**
 * WHAT EACH DECLARED `style` MUST BE TRUE OF IN THE BYTES
 * (spec-install-all-face-cuts, story 3).
 *
 * The closed set is the document format's: a chain entry declares a base face
 * plus `bold`, `italic` and `boldItalic`, so a fifth weight has nowhere to be
 * written and must not reach the catalogue. `scripts/build-wasm.mjs` refuses a
 * row outside this set at build time; this table is what the BYTES are then
 * held to, which is the check a build-time string comparison cannot make.
 *
 * `subfamily` IS THE BINARY'S OWN WORD FOR THE CUT and is not the `style`
 * token: the row says `BoldItalic` (one word, the format's key spelling) and
 * upstream's name table says `Bold Italic`. Both spellings are stated here,
 * once, rather than derived at the point of comparison.
 */
const catalogueCuts: Readonly<Record<string, { subfamily: string; usWeightClass: number; bold: boolean; italic: boolean; macStyle: number }>> = {
  Regular: { subfamily: 'Regular', usWeightClass: 400, bold: false, italic: false, macStyle: 0x0000 },
  Bold: { subfamily: 'Bold', usWeightClass: 700, bold: true, italic: false, macStyle: 0x0001 },
  Italic: { subfamily: 'Italic', usWeightClass: 400, bold: false, italic: true, macStyle: 0x0002 },
  BoldItalic: { subfamily: 'Bold Italic', usWeightClass: 700, bold: true, italic: true, macStyle: 0x0003 },
}

/**
 * THE CSS FAMILY NAME A ROW IS EMITTED UNDER, derived from (family, style) the
 * way `scripts/build-wasm.mjs`'s `cssFamilyOf` derives it.
 *
 * DUPLICATED FROM THE GENERATOR DELIBERATELY, on this file's standing
 * convention: this suite must be able to redden on its own, and a generator
 * that changed its derivation while this copy stood still is exactly the
 * disagreement the uniqueness assertion below exists to catch.
 */
const cssFamilyOf = (face: CatalogueFace) => face.style === 'Regular' ? face.family : `${face.family} ${face.style === 'BoldItalic' ? 'Bold Italic' : face.style}`

const catalogue: ReadonlyArray<CatalogueFace> = JSON.parse(fs.readFileSync(cataloguePath, 'utf8'))
const faceDirectory = (face: CatalogueFace) => path.join(fontsRoot, face.directory)
const faceFile = (face: CatalogueFace) => path.join(faceDirectory(face), face.file)
const digest = (file: string) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex')

// ---------------------------------------------------------------------------
// THE SMALLEST sfnt READ THAT ANSWERS "IS THIS ONE UPRIGHT STATIC REGULAR".
// No font library, deliberately, on the ground `src/font-binary-identity.test.ts`
// already records: reading four integers out of a table directory is a short
// DataView walk, and a parser dependency added to check them would put a new
// package in the designer's graph. Story 16.1 moved the `name`-table half of
// that walk into `src/font-name-table.ts` and left the OS/2, head and post reads
// here, where they are this file's own question.
//
// ⚠ ONE READER, AND THE COST IS STATED. The comment that used to sit on
// `copyright` below claimed the comparison was "between two independent readers
// rather than one reader agreeing with itself". After the extraction that is no
// longer true of the browser side, and the claim is corrected rather than left
// standing: the generated catalogue and this test now read nameID 0 through the
// SAME module. What was bought is that the runtime reader — the one that decides
// what `font.copyright` a fetched face publishes — is the reader these 21
// committed faces exercise on every run, instead of a third hand-copy nobody
// checks. The independence that remains is Go's: `internal/fontset` walks the
// same table again, from the bytes, for its own different question.
//
// THE SWITCH WAS WITNESSED, not assumed. This file's assertions were run against
// the generated catalogue emitted by the OLD hand-written reader and again by the
// shared one, and all 21 `copyright` values were byte-identical.
// ---------------------------------------------------------------------------

function fontView(file: string): DataView {
  const bytes = fs.readFileSync(file)
  return new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength)
}

const sfntTables = (view: DataView): Readonly<Record<string, SfntTable>> => requireStaticTrueTypeTables(view)

/** Everything AC6 asks a face about itself, read from its own bytes. */
function instanceOfFile(file: string) {
  const view = fontView(file)
  const tables = sfntTables(view)
  const os2 = tables['OS/2']
  const head = tables['head']
  const post = tables['post']
  if (os2 === undefined || head === undefined || post === undefined) throw new Error(`${file} is missing an OS/2, head or post table`)
  return {
    family: nameTableString(view, tables, 16) ?? nameTableString(view, tables, 1) ?? '<the file declares no family name>',
    subfamily: nameTableString(view, tables, 17) ?? nameTableString(view, tables, 2) ?? '<the file declares no subfamily name>',
    // nameID 13 is the LICENCE DESCRIPTION the face carries in its own bytes.
    licenceDescription: nameTableString(view, tables, 13) ?? '<the file declares no licence description>',
    // nameID 0 is the COPYRIGHT, and it is the value the generated catalogue
    // publishes as `copyright`. Read through the shared reader — see the note
    // above for what that costs and what it bought.
    copyright: nameTableString(view, tables, 0)?.trim(),
    usWeightClass: view.getUint16(os2.offset + 4),
    // fsSelection bit 0 ITALIC, bit 5 BOLD, bit 6 REGULAR, bit 9 OBLIQUE.
    fsSelection: view.getUint16(os2.offset + 62),
    // head.macStyle bit 0 bold, bit 1 italic.
    macStyle: view.getUint16(head.offset + 44),
    italicAngle: view.getInt32(post.offset + 4) / 65536,
    variableTables: ['fvar', 'gvar', 'avar', 'HVAR', 'MVAR'].filter((tag) => tag in tables),
    outlineTables: ['glyf', 'CFF ', 'CFF2'].filter((tag) => tag in tables),
  }
}

/**
 * The digest a face's own `NOTICE.md` records for the file it ships, on exactly
 * the reader `src/font-binary-identity.test.ts` uses for the six shipped faces —
 * duplicated rather than imported, so each guard reddens on its own without
 * depending on the other suite's helper staying correct, and so importing it
 * does not register that suite a second time under this one.
 */
function recordedShippedDigest(noticeFile: string): string {
  const notice = fs.readFileSync(noticeFile, 'utf8')
  const rows = [...notice.matchAll(/^\|[^|\n]*sha256 of the SHIPPED[^|\n]*\|\s*`([0-9a-f]{64})`\s*\|/gm)]
  if (rows.length !== 1) throw new Error(`${noticeFile} must record exactly one 'sha256 of the SHIPPED …' table row carrying a 64-hex digest, and records ${rows.length}`)
  return rows[0][1]
}

/**
 * THE UPSTREAM ARCHIVE A NOTICE PINS — its URL, its digest, its byte length and
 * the path the face was taken out of, read through the same
 * exactly-one-row rule every other NOTICE reader here applies.
 *
 * Added by spec-install-all-face-cuts story 3 for the claim its 76 new NOTICEs
 * make and nothing checked: *"The archive is the same one this family's Regular
 * was taken from, pinned by the same digest."*
 */
function recordedArchive(noticeFile: string): Readonly<{ url: string; digest: string; bytes: number; path: string }> {
  const notice = fs.readFileSync(noticeFile, 'utf8')
  const one = (label: string, pattern: RegExp): string => {
    const rows = [...notice.matchAll(pattern)]
    if (rows.length !== 1) throw new Error(`${noticeFile} must record exactly one ${label} row, and records ${rows.length}`)
    return rows[0][1]
  }
  return {
    url: one('download URL', /^\| Download URL \| (\S+) \|$/gm),
    digest: one('release archive sha256', /^\| sha256 of the release archive \| `([0-9a-f]{64})`/gm),
    bytes: Number(one('release archive byte length', /^\| sha256 of the release archive \| `[0-9a-f]{64}` \(([\d,]+) bytes\)/gm).replaceAll(',', '')),
    path: one('path inside the archive', /^\| Path inside the archive \| `(\S+)` \|$/gm),
  }
}

/** The byte count a NOTICE records for the file it ships, as a number. */
function recordedShippedSize(noticeFile: string): number {
  const notice = fs.readFileSync(noticeFile, 'utf8')
  const rows = [...notice.matchAll(/^\| Size \| ([0-9,]+) bytes \|$/gm)]
  if (rows.length !== 1) throw new Error(`${noticeFile} must record exactly one '| Size | <n> bytes |' row, and records ${rows.length}`)
  return Number(rows[0][1].replaceAll(',', ''))
}

/**
 * THE LICENCE A FACE DECLARES IN ITS OWN `name` TABLE, per SPDX identifier.
 *
 * WHY THIS EXISTS, AND WHY IT IS THE `licence` FIELD'S FIRST CONSUMER.
 * `src/font-binary-identity.test.ts` already holds each chrome face's nameID 13
 * to the SIL OFL, on the stated ground that a redistributed asset's terms
 * travel in its `name` table as well as in the `LICENSE*`/`NOTICE*` beside it.
 * This file adopted that suite's DIGEST tie and not its LICENCE tie, and the
 * gap is not theoretical: swap a face's binary and its NOTICE together — same
 * family, different terms — and the digest check, the name check and the
 * instance checks all stay green while `lint/MANIFEST.md` publishes a licence
 * the binary itself contradicts.
 *
 * AND IT IS WHAT MAKES `font-catalogue.json`'s `licence` FIELD LOAD-BEARING.
 * Until this table existed nothing in the designer read that field at all: it
 * restated a fact already held in three other places (the NOTICE, the manifest
 * row, `pinnedCensus`) and could disagree with all three in silence. Keyed off
 * it, the field now has exactly one job and reds when it is wrong.
 *
 * MEASURED over all 21 committed faces before being written, not assumed: the
 * 19 OFL-1.1 faces all carry the SIL sentence in nameID 13 — including
 * `cascadiacode` and `cascadiamono`, whose description OPENS "Microsoft
 * supplied font..." and carries the OFL sentence further in, which is why this
 * is a substring match rather than a prefix or an equality — and both
 * Ubuntu-font-1.0 faces read "Licensed under the Ubuntu Font Licence 1.0."
 *
 * A CLOSED TABLE, DELIBERATELY: an id with no entry here fails rather than
 * skipping, so admitting a new licence to the catalogue is a decision somebody
 * makes here rather than a silent hole. It is NOT the licence gate's allowlist
 * and must not be treated as one — `lint` owns admission (D-000.11); this only
 * asks whether the bytes agree with the label already admitted.
 */
const licenceSignatures: Readonly<Record<string, RegExp>> = {
  'OFL-1.1': /SIL Open Font License/i,
  'Ubuntu-font-1.0': /Ubuntu Font Licence/i,
  // ADMITTED BY STORY 16.1a, AND THIS LINE IS THE DECISION THE COMMENT ABOVE
  // DEMANDS BE MADE HERE. `Roboto Slab` is the batch's one non-OFL face:
  // `github.com/googlefonts/robotoslab` ships an Apache-2.0 `LICENSE.txt` and
  // the binary's nameID 13 reads "Licensed under the Apache License, Version
  // 2.0" — the two agree, which is the property this table exists to check.
  // `Apache-2.0` is the second of the owner's four ids (D-8.5.3), already on
  // `lint`'s `fontAssetLicenceAllowlist`, so admission is unchanged; what is
  // new is only that a catalogue face now exercises it.
  // VERSION-PINNED, matching `folio-go/internal/fontset/licencesignature_test.go`'s
  // own Apache pattern. `/Apache License/i` alone would be satisfied by an
  // Apache 1.1 name table while the manifest declared `Apache-2.0` — the
  // admitted id, carrying terms the bytes do not state. The optional comma is
  // there because the wording varies ("Apache License, Version 2.0" and
  // "Apache License Version 2.0" both occur upstream); the VERSION does not.
  'Apache-2.0': /Apache License,?\s+Version 2\.0/i,
}

/**
 * THE UNICODE `cmap`, AS A SET OF THE CODEPOINTS THE FACE ACTUALLY MAPS.
 *
 * Formats 4 and 12 only, and that is measured rather than assumed: all 21
 * committed faces carry one or the other under a (3,1), (3,10) or (0,x)
 * subtable. A face carrying neither throws here instead of being scored zero —
 * a coverage check that silently reads no subtable would report every script
 * uncovered and pass nothing, which is the vacuous-green shape this file's
 * other guards are written against.
 *
 * Format 12 groups are bounded per group, because a CJK face's cmap is
 * hundreds of thousands of codepoints and this test only ever asks about a
 * handful of them; no committed catalogue face is CJK today, and the bound is
 * what keeps that from becoming a minutes-long test if one ever is.
 */
function cmapCoverage(file: string): ReadonlySet<number> {
  const view = fontView(file)
  const tables = sfntTables(view)
  const cmap = tables['cmap']
  if (cmap === undefined) throw new Error(`${file} has no cmap table`)
  const subtables = view.getUint16(cmap.offset + 2)
  let chosen = -1
  for (let index = 0; index < subtables; index++) {
    const record = cmap.offset + 4 + index * 8
    const platform = view.getUint16(record)
    const encoding = view.getUint16(record + 2)
    if (!((platform === 3 && (encoding === 1 || encoding === 10)) || platform === 0)) continue
    const subtable = cmap.offset + view.getUint32(record + 4)
    const format = view.getUint16(subtable)
    if (format === 4 || format === 12) chosen = subtable
  }
  if (chosen < 0) throw new Error(`${file} carries no Unicode cmap subtable in format 4 or 12`)
  const covered = new Set<number>()
  if (view.getUint16(chosen) === 4) {
    const segmentBytes = view.getUint16(chosen + 6)
    const endOffset = chosen + 14
    const startOffset = endOffset + segmentBytes + 2
    const deltaOffset = startOffset + segmentBytes
    const rangeOffset = deltaOffset + segmentBytes
    for (let segment = 0; segment < segmentBytes / 2; segment++) {
      const start = view.getUint16(startOffset + segment * 2)
      const end = view.getUint16(endOffset + segment * 2)
      if (start === 0xffff) continue
      const delta = view.getInt16(deltaOffset + segment * 2)
      const range = view.getUint16(rangeOffset + segment * 2)
      for (let codepoint = start; codepoint <= end; codepoint++) {
        let glyph: number
        if (range === 0) glyph = (codepoint + delta) & 0xffff
        else {
          const at = rangeOffset + segment * 2 + range + (codepoint - start) * 2
          if (at + 1 >= view.byteLength) continue
          glyph = view.getUint16(at)
          if (glyph !== 0) glyph = (glyph + delta) & 0xffff
        }
        // GLYPH 0 IS .notdef — a mapping to it is the absence of a mapping,
        // and counting it would score every face as covering everything.
        if (glyph !== 0) covered.add(codepoint)
      }
    }
  } else {
    const groups = view.getUint32(chosen + 12)
    for (let group = 0; group < groups; group++) {
      const record = chosen + 16 + group * 12
      const start = view.getUint32(record)
      const end = view.getUint32(record + 4)
      for (let codepoint = start; codepoint <= end && codepoint - start < 70000; codepoint++) covered.add(codepoint)
    }
  }
  return covered
}

/**
 * THE PROBE PER SCRIPT: codepoints a face claiming that script must ALL map,
 * and a face not claiming it must map NONE of.
 *
 * They are ordinary letters rather than rarities on purpose. The question is
 * "can this face draw text in this script at all", not "is its coverage
 * complete" — a partial Latin face is still the right first entry in a chain,
 * whereas one that maps no Latin letter at all must not be. Each probe spans
 * more than one block of its script so a face carrying, say, only ASCII digits
 * does not pass as Latin.
 *
 * MEASURED over all 21 committed faces before being written: 19 map every
 * Latin probe and no Thai one; notosansthailooped and notoserifthai map every
 * Thai probe and no Latin one. No committed face is ambiguous under it, and no
 * committed face is CJK.
 */
const scriptProbes: Readonly<Record<string, ReadonlyArray<number>>> = {
  // A, Z, a, z, 0, 9 — Basic Latin letters and digits.
  latin: [0x41, 0x5a, 0x61, 0x7a, 0x30, 0x39],
  // ko kai, so suea, a vowel sign and a tone mark: consonants, vowel and tone.
  thai: [0x0e01, 0x0e2a, 0x0e30, 0x0e48],
  // Four common Han ideographs.
  cjk: [0x4e00, 0x4e8c, 0x6c34, 0x9fa5],
}

describe('the Story 8.5 catalogue ships the faces its manifest declares', () => {
  // NON-VACUITY FIRST. Every loop below is over `catalogue`, and an empty or
  // truncated manifest would satisfy all of them silently — the exact shape of
  // vacuous green this story's design notes are written against.
  it('declares at least twenty NEW families, none of them a family the thirteen shipped rules already declare', () => {
    // THE POPULATION FLOOR — ONE OF FOUR, AND ALL FOUR MOVE TOGETHER.
    // The other three are `src/font-index.test.ts` ("is the whole bundled catalogue, unchanged"),
    // `src/font-name-table.test.ts` ("reads a copyright out of every committed
    // catalogue face") and
    // `src/font-provenance.test.ts` ("is asserted over the whole committed tier").
    // Raised 20 -> 31 by Story 16.1a, which added ten families to the local
    // face tier, and 31 -> 107 by spec-install-all-face-cuts story 3, which
    // gave those families the cuts they publish. IT IS A FACE COUNT, NOT A
    // FAMILY COUNT: the tier still holds 31 families. D-16.R.12: "a floor left at 21 while the tier grows to 30 is
    // a floor that stops measuring the thing it was built to measure" — and
    // D-16.R.18's correction to it: a floor that exists in N files is N
    // floors, so a batch that raises one and leaves the rest behind is
    // silently unmeasured at the ones it left.
    //
    // RAISED 31 -> 107 BY spec-install-all-face-cuts STORY 3, AND IT IS NOW A
    // FACE FLOOR OVER A FAMILY FLOOR. The tier still holds 31 families; it now
    // holds every cut those families publish, so the number that must not go
    // stale is the row count — a story that dropped 76 cuts and left 31
    // Regulars would satisfy a floor of 31 exactly. The family floor is
    // asserted separately below so neither can drift behind the other.

    expect(catalogue.length, 'the committed tier population floor; story 3 raised it 31 -> 107 when the tier gained its cuts').toBeGreaterThanOrEqual(107)
    const families = catalogue.map((face) => face.family)
    expect(new Set(families).size, 'the committed tier holds 31 families; a batch that dropped one would still clear the face floor above').toBeGreaterThanOrEqual(31)
    // THE UNIQUENESS KEY IS (family, style), NOT family. It was `family` while
    // every row was that family's only face; a four-cut family makes `Inter`
    // legitimately appear four times, and what must still be unique is the CUT.
    const cuts = catalogue.map((face) => `${face.family}\u0000${face.style}`)
    expect(new Set(cuts).size, 'two catalogue entries declare the same cut of the same family').toBe(cuts.length)
    // A row outside the closed set is a face no cut resolver can find and a
    // face the format has no key to declare.
    expect(catalogue.filter((face) => !Object.hasOwn(catalogueCuts, face.style)).map((face) => `${face.id}: ${face.style}`), `a catalogue style must be one of ${Object.keys(catalogueCuts).join(', ')}`).toEqual([])
    // ALL FOUR CUTS MUST BE EXERCISED, or the per-row metadata loop below is a
    // Regular-only assertion wearing a table. Six families publish no italic,
    // and DM Sans's italic is withheld by ruling, so the population is
    // deliberately ragged — but it is never one-valued.
    expect([...new Set(catalogue.map((face) => face.style))].sort(), 'the tier must exercise every declared cut, or the per-row style assertion proves nothing a Regular-only one did not').toEqual(['Bold', 'BoldItalic', 'Italic', 'Regular'])
    // THIRTEEN SINCE STORY 11.1, and read off `shippedSlotFaces` rather than
    // from a list of its own — six was this assertion's population until the
    // seven cuts landed, and a stale copy here quietly stopped refusing the
    // seven names it had never heard of.
    const shipped = shippedFamilies()
    expect(shipped.length, 'the hand-written shipped population is thirteen since Story 11.1; a shorter list stops refusing the names it has not heard of').toBe(13)
    // AND THE COLLISION IS CHECKED OVER THE DERIVED CSS NAME, WHICH IS WHAT
    // THE BROWSER RESOLVES. `{ family: "Roboto", style: "Bold" }` derives
    // `Roboto Bold` — a hardcoded shipped rule's family and a CORE release
    // asset — and a second `@font-face` under that name would let the last rule
    // win silently. Checking `face.family` alone would pass over it, because
    // plain `Roboto` is a legitimate catalogue row.
    const cssNames = catalogue.map(cssFamilyOf)
    expect(cssNames.filter((name) => shipped.includes(name)), 'a catalogue row must not derive a CSS family one of the thirteen shipped rules already declares; Roboto\'s three cuts ship as hardcoded core faces and must not be redeclared here').toEqual([])
    expect(new Set(cssNames).size, 'two catalogue rows derive the same CSS family name, so one @font-face rule would silently shadow the other').toBe(cssNames.length)
    const directories = catalogue.map((face) => face.directory)
    expect(new Set(directories).size, 'two catalogue entries share a directory, so two families would resolve to one file').toBe(directories.length)
    const ids = catalogue.map((face) => face.id)
    expect(new Set(ids).size).toBe(ids.length)
    // Every id is the runtime filename stem AND the token the release manifest
    // recognises a catalogue asset by, so its shape is asserted, not assumed.
    expect(ids.filter((id) => !/^[a-z0-9]+$/.test(id)), 'a catalogue id must be lower-case alphanumeric').toEqual([])
  })

  // AC1. THE PROVENANCE RECORD IS TRUE OF THE BYTES BESIDE IT.
  it('gives every catalogue face a LICENSE, a NOTICE, and a NOTICE that describes the binary it sits next to', () => {
    for (const face of catalogue) {
      const directory = faceDirectory(face)
      const file = faceFile(face)
      expect(fs.existsSync(file), `${face.id}: ${path.relative(designerRoot, file)} is declared in font-catalogue.json and is not committed`).toBe(true)

      // `manifest.ResolveAssets` looks for a file whose name STARTS WITH
      // "LICENSE" — uppercase, no other spelling. A face carrying `LICENCE.txt`
      // or `OFL.txt` would fail the licence gate at build time with a message
      // about a missing licence file, which is a confusing way to learn that a
      // filename was copied verbatim from upstream.
      const licences = fs.readdirSync(directory).filter((name) => name.startsWith('LICENSE'))
      expect(licences.length, `${face.id}: exactly one LICENSE* file is expected beside the binary; found ${JSON.stringify(licences)}`).toBe(1)

      const notice = path.join(directory, 'NOTICE.md')
      expect(fs.existsSync(notice), `${face.id}: no NOTICE.md, which manifest.ResolveAssets (AC25, AD-26) already requires`).toBe(true)
      const text = fs.readFileSync(notice, 'utf8')

      // The copyright line the licence gate will publish in lint/MANIFEST.md.
      expect(text.split('\n').some((line) => line.includes('Copyright')), `${face.id}: NOTICE.md carries no line containing "Copyright", so the licence gate fails the build`).toBe(true)

      // AC1's six recorded facts, each asserted by the row that carries it.
      expect(text, `${face.id}: NOTICE.md records no pinned upstream release`).toMatch(/^\| Upstream project \| .+release `[^`]+` \|$/m)
      expect(text, `${face.id}: NOTICE.md records no download URL`).toMatch(/^\| Download URL \| https:\/\/\S+ \|$/m)
      expect(text, `${face.id}: NOTICE.md records no path inside the archive`).toMatch(/^\| Path inside the archive \| `\S+` \|$/m)
      expect(text, `${face.id}: NOTICE.md records no fetch date`).toMatch(/^\| Fetched \| \d{4}-\d{2}-\d{2} \|$/m)
      expect(text, `${face.id}: NOTICE.md records no upstream archive digest`).toMatch(/^\| sha256 of the release archive \| `[0-9a-f]{64}` \([\d,]+ bytes\) \|$/m)
      expect(text, `${face.id}: NOTICE.md does not state its relation to the source`).toMatch(/copied unmodified, no derivation/)

      // AND THE TWO FACTS THAT CAN BE FALSE OF THE FILE ITSELF.
      //
      // ⚠ WHAT THIS TIE CATCHES IS A BINARY AND ITS RECORD DISAGREEING INSIDE
      // ONE DIRECTORY, AND THAT IS ALL IT CATCHES. An earlier wording claimed
      // it caught "a different weight, a different style, a subset cut", which
      // overstates it in the one direction that matters now that a family
      // occupies four directories: a row REPOINTED from `inter-bold/` to
      // `inter-italic/` reads that directory's own NOTICE, finds it describes
      // that directory's own binary, and passes here. The assertion that
      // catches a repointed row is the per-row style claim further down — the
      // binary's subfamily, weight class and style bits against the row's
      // declared `style` — and the two are stated separately because they fail
      // on different faults.
      expect(
        digest(file),
        `${face.id}: the binary's sha256 is not the digest its own NOTICE.md records. Either the binary in this directory was `
        + 'swapped without amending the provenance record beside it, or the record was amended without the binary. Both make '
        + 'the NOTICE a false statement about the bytes it sits next to. (A row repointed at a DIFFERENT directory is a '
        + "different fault and is caught by this face's declared-style assertion, not here.)",
      ).toBe(recordedShippedDigest(notice))
      expect(recordedShippedSize(notice), `${face.id}: the recorded byte size is not the committed file's size`).toBe(fs.statSync(file).size)

      // AND THE TERMS THE BINARY ITSELF DECLARES (nameID 13), which is the one
      // statement of a face's licence that cannot be edited from outside the
      // binary. `font-catalogue.json`'s `licence` field is the key, so the
      // field is load-bearing rather than decorative.
      const signature = licenceSignatures[face.licence]
      expect(signature, `${face.id}: font-catalogue.json declares the licence '${face.licence}', which no entry in licenceSignatures recognises. Admitting a licence to the catalogue is a decision to record here, not a hole to fall through.`).toBeDefined()
      expect(
        instanceOfFile(file).licenceDescription,
        `${face.id}: font-catalogue.json declares '${face.licence}' and the binary's own name table (nameID 13) does not say so. `
        + 'A redistributed asset carries its terms in its name table as well as in the LICENSE*/NOTICE* beside it, and swapping '
        + 'a binary and its NOTICE together — same family, different terms — passes every other check in this file while '
        + 'lint/MANIFEST.md publishes a licence the bytes contradict.',
      ).toMatch(signature as RegExp)

      // The three records must also agree with each other: the NOTICE names the
      // same identifier the manifest declares.
      expect(text, `${face.id}: NOTICE.md does not name the SPDX identifier font-catalogue.json declares`).toContain(`\`${face.licence}\``)

      // STORY 8.6 — AND THE GENERATED MODULE PUBLISHES THIS FACE'S OWN TERMS.
      //
      // The designer sends `licenceText` and `copyright` with every pick, and
      // the engine refuses to load a document that embeds a face without them,
      // so these two strings ARE the terms a `.folio` travels under. They are
      // asserted per face, against this face's own directory and this face's
      // own bytes, because the failure they exist to catch is not "the field
      // is empty" — it is "the field is FULL, and it belongs to another
      // project".
      const generated = generatedFaces.find((emitted) => emitted.id === face.id)
      expect(generated, `${face.id}: font-catalogue.json declares it and src/generated/font-catalogue.ts emits no row for it, so the pick could not embed it`).toBeDefined()

      // (a) THE LICENCE TEXT IS THE ONE COMMITTED BESIDE THIS BINARY.
      // NOT keyed by SPDX identifier: the SIL OFL carries a per-project
      // preamble — a copyright line and a Reserved Font Name — so two OFL-1.1
      // faces ship two DIFFERENT texts, and an identifier classifies terms
      // rather than standing in for them.
      const licenceFile = path.join(directory, licences[0])
      expect(
        generated?.licenceText,
        `${face.id}: the generated catalogue publishes a licence text that is not the one committed beside this binary (${path.relative(designerRoot, licenceFile)}). Every document embedding this face would travel stating another project's terms.`,
      ).toBe(fs.readFileSync(licenceFile, 'utf8').trimEnd())
      expect(generated?.licence, `${face.id}: the generated catalogue and the manifest disagree on the SPDX identifier`).toBe(face.licence)

      // (b) THE COPYRIGHT IS THIS BINARY'S OWN nameID 0.
      const declaredCopyright = instanceOfFile(file).copyright
      expect(declaredCopyright, `${face.id}: the binary declares no copyright in its own name table (nameID 0), so there is nothing for the document to record`).toBeTruthy()
      expect(
        generated?.copyright,
        `${face.id}: the generated catalogue publishes a copyright that is not the one this face's own name table declares. nameID 0 is the one statement of provenance that cannot be edited from outside the binary.`,
      ).toBe(declaredCopyright)
    }
  })

  // NON-VACUITY FOR THE TWO ASSERTIONS ABOVE. Both are inside a loop over
  // `catalogue` and both reach the generated module through `.find()`, so a
  // module that emitted nothing at all would make them assert on `undefined`
  // rows — caught by the `toBeDefined()` above, but only face by face. This
  // states the population once, at the top level, so a truncated or stale
  // generated module reds here with one clear sentence.
  it('emits exactly one generated row per declared catalogue face', () => {
    expect(generatedFaces).toHaveLength(catalogue.length)
    expect(generatedFaces.map((face) => face.id).sort()).toEqual(catalogue.map((face) => face.id).sort())
    // And every row carries a URL into the fingerprinted runtime directory —
    // the bytes the pick reads. A row with no URL is a family the author can
    // see and cannot embed.
    expect(generatedFaces.filter((face) => typeof face.url !== 'string' || face.url === '')).toEqual([])
  })

  // ONE FAMILY, ONE PINNED ARCHIVE — THE CLAIM 76 NOTICES MAKE AND NOTHING
  // CHECKED (spec-install-all-face-cuts, story 3).
  //
  // Every cut's NOTICE says in words that its bytes came out of the SAME
  // upstream archive its family's Regular came from — "one coherent source, not
  // two" — and that sentence is the whole provenance argument for the 76 new
  // faces: it is why a cut inherits the Regular's already-reviewed licence
  // admission instead of needing its own. A sentence repeated 76 times with no
  // assertion behind it is exactly the shape of the licence-text defect this
  // suite was built after, where `font-catalogue.json` was right, every binary
  // was right, and the artifact between them was checked by nothing.
  //
  // FOUR FACTS, BECAUSE THREE OF THEM CAN AGREE WHILE THE FOURTH IS WRONG. A
  // cut re-fetched from a LATER release of the same project keeps the URL's
  // shape and changes the digest; one taken from a sibling family's archive in
  // the same org keeps neither. And the DIRECTORY inside the archive is checked
  // as well as the archive itself, because `RobotoCondensed-*` sits beside
  // `Roboto-*` and `CascadiaCodeNF-*` beside `CascadiaCode-*`: same archive,
  // different family, and the prefix trap this story's own boundary names.
  it('takes every cut out of the same pinned archive, and the same directory in it, as its family\'s Regular', () => {
    const regularOf = new Map(catalogue.filter((face) => face.style === 'Regular').map((face) => [face.family, face]))
    const cuts = catalogue.filter((face) => face.style !== 'Regular')
    // NON-VACUITY: this must run over the cuts, not over an empty list, and
    // every one of them must have a Regular to be compared against.
    expect(cuts.length, 'the committed tier declares no cut at all, so this assertion has nothing to check').toBeGreaterThanOrEqual(76)
    expect(cuts.filter((face) => !regularOf.has(face.family)).map((face) => face.id), 'every cut must belong to a family whose Regular is also declared; the upright Regular is the base and stays required').toEqual([])

    for (const face of cuts) {
      const regular = regularOf.get(face.family)!
      const mine = recordedArchive(path.join(faceDirectory(face), 'NOTICE.md'))
      const theirs = recordedArchive(path.join(faceDirectory(regular), 'NOTICE.md'))
      const where = `${face.id} (${face.family} ${face.style})`
      expect(mine.url, `${where}: its NOTICE pins a different upstream archive from ${regular.id}'s, while claiming both come from the same one`).toBe(theirs.url)
      expect(mine.digest, `${where}: its NOTICE records a different archive sha256 from ${regular.id}'s. A cut re-fetched from a later release of the same project keeps the URL and changes this.`).toBe(theirs.digest)
      expect(mine.bytes, `${where}: its NOTICE records a different archive byte length from ${regular.id}'s`).toBe(theirs.bytes)
      // THE DIRECTORY, NOT THE FILENAME: the filenames differ by construction
      // (`Arimo-Bold.ttf` beside `Arimo-Regular.ttf`), and what must match is
      // the folder they were both taken from.
      const directoryOf = (inside: string) => inside.slice(0, inside.lastIndexOf('/'))
      expect(directoryOf(mine.path), `${where}: taken from ${directoryOf(mine.path)} while its Regular was taken from ${directoryOf(theirs.path)} in the same archive. Several archives hold a DIFFERENT family whose filenames share a stem, so the directory is part of the contract.`).toBe(directoryOf(theirs.path))
      expect(mine.path, `${where}: records the same path inside the archive as its Regular, so one of the two NOTICEs describes a file it did not ship`).not.toBe(theirs.path)
    }
  })

  // AC6, RE-STATED PER ROW (spec-install-all-face-cuts, story 3).
  //
  // THIS ASSERTION USED TO BE "EVERY CATALOGUE FACE IS AN UPRIGHT STATIC
  // REGULAR 400", and it is retired rather than deleted: the committed tier now
  // carries the cuts its families publish, so the blanket claim is false of the
  // population while every reason it existed is still live. What replaces it is
  // the same question asked per row — DOES THIS BINARY AGREE WITH THE `style`
  // ITS ROW DECLARES — on the pattern the thirteen hardcoded slots have used
  // since Story 11.1 ('ships each hardcoded slot as the instance it is intended
  // to be', below). A declared style nothing reads off the bytes is how a
  // swapped or hand-edited binary ships: the CSS name, the release asset and
  // the chain entry would all still say `Inter Bold` while the file painted the
  // Regular.
  //
  // TWO CLAIMS SURVIVE THE RETIREMENT UNCHANGED, and they are the absolute
  // ones: NO VARIABLE TABLES — this product accepts no axis on any tier
  // (D-16.5(c)) — and glyf/TrueType outlines in a `.ttf`, which is what the
  // emitted `format('truetype')` rule and the engine's decoder both require.
  it('ships every catalogue face as the static cut its row declares, with no variable axis', () => {
    for (const face of catalogue) {
      const file = faceFile(face)
      const instance = instanceOfFile(file)
      const cut = catalogueCuts[face.style]
      const where = `${face.id} (${path.relative(designerRoot, file)})`
      expect(cut, `${where}: declares the style '${face.style}', which is outside the closed set ${Object.keys(catalogueCuts).join(', ')}`).toBeDefined()
      const say = `${where}: font-catalogue.json declares it the '${face.style}' cut of '${face.family}' and the bytes say family '${instance.family}', subfamily '${instance.subfamily}', usWeightClass ${instance.usWeightClass}`

      // (1) THE ROW'S `family` IS THE BASE FAMILY, AND THE BYTES ARE WHERE THAT
      // IS CHECKED. A bold cut's own name table calls itself family `Inter`,
      // subfamily `Bold` — so the row stays `{ family: "Inter", style: "Bold" }`
      // and the CSS name `Inter Bold` is DERIVED. Storing the CSS name in
      // `family` would red exactly here, which is one of the two ends that
      // force the split.
      expect(instance.family, `${say}. A family name is an assertion about bytes, and a cut's own name table calls itself by its BASE family.`).toBe(face.family)
      expect(instance.subfamily, `${say}, and a '${face.style}' row must carry subfamily '${cut.subfamily}'`).toBe(cut.subfamily)
      expect(instance.usWeightClass, `${say}, and a '${face.style}' row must carry OS/2.usWeightClass ${cut.usWeightClass}`).toBe(cut.usWeightClass)

      // (2) THE BITS AGREE WITH THE NAME. A face naming itself Bold while its
      // OS/2 and head tables say Regular is the shape a swapped binary takes,
      // and the subfamily check alone cannot see it. DM Sans's upstream italic
      // is exactly this defect — ITALIC clear, macStyle 0x0, italicAngle -10 —
      // and it is withheld from the tier by ruling rather than admitted by a
      // named exception here. THIS GUARD IS ABSOLUTE: no allowlist, no
      // "known upstream bug" clause.
      expect(Boolean(instance.fsSelection & 0x0020), `${say}: OS/2.fsSelection BOLD bit must be ${cut.bold}`).toBe(cut.bold)
      expect(Boolean(instance.fsSelection & 0x0001), `${say}: OS/2.fsSelection ITALIC bit must be ${cut.italic}`).toBe(cut.italic)
      expect(Boolean(instance.fsSelection & 0x0040), `${say}: OS/2.fsSelection REGULAR bit must be set for an upright Regular and clear for every cut`).toBe(!cut.bold && !cut.italic)
      // AND THE OBLIQUE BIT (0x0200), WHICH IS NOT DERIVABLE FROM THE OTHERS
      // and is the one style bit this population does not agree on. MEASURED
      // over all 107 committed faces: `Roboto Condensed Italic` (0x0201) and
      // `Roboto Condensed Bold Italic` (0x0221) set it — upstream's own
      // `static/` build, exactly as the three hardcoded Roboto cuts do — and
      // the other 105 do not. So the rule that IS true of the population is
      // stated, and it is one-directional: an UPRIGHT face may never claim
      // oblique. Asserting it clear everywhere would be false of those two;
      // asserting it set on every italic would be false of the other 44 — the
      // sloped population is 46 faces, 23 `Italic` and 23 `BoldItalic`.
      if (!cut.italic) expect(instance.fsSelection & 0x0200, `${say}: an upright cut may not set the OS/2.fsSelection OBLIQUE bit`).toBe(0)
      expect(instance.macStyle, `${say}, and a '${face.style}' row must carry head.macStyle 0x${cut.macStyle.toString(16).padStart(4, '0')}`).toBe(cut.macStyle)

      // (3) THE SLOPE IS IN THE OUTLINES, not only in a bit. The amount is
      // upstream's, so the SIGN is asserted rather than a pinned constant.
      if (cut.italic) expect(instance.italicAngle, `${say}, and an italic cut must carry a negative post.italicAngle`).toBeLessThan(0)
      else expect(instance.italicAngle, `${say}, and an upright cut must carry post.italicAngle exactly 0`).toBe(0)

      // (4) STATIC, ALWAYS — the half of the old AC6 that is still absolute.
      expect(instance.variableTables, `${where}: carries variable-font tables. Epic 11 (FR57) owns realize-vs-retire and the owner ruling has not been made (D-000.7).`).toEqual([])
      // NFR7's operative choice: the glyf/TrueType static build, not CFF.
      expect(instance.outlineTables, `${where}: is not a glyf/TrueType static build`).toEqual(['glyf'])
      expect(path.extname(face.file), `${where}: the engine decodes only font/ttf and font/otf, and the emitted rule declares format('truetype')`).toBe('.ttf')
    }
  })

  // AND THE READER DISCRIMINATES AT CATALOGUE SCALE, so the loop above means
  // "each row is the cut it claims" rather than "instanceOfFile answers the
  // same thing to everything". The Regular and the Bold of one COMMITTED family
  // are the pair that matters: they share a name[1] — which is precisely why
  // `family` can stay the base name on every row — and every field that
  // separates them is a field this suite would be worthless without.
  //
  // It is driven from the CATALOGUE rather than from hardcoded paths: the rows
  // are looked up by (family, style), so a tier that stopped declaring cuts
  // reds here rather than skipping silently.
  it('tells the four declared cuts of one committed family apart, from the bytes alone', () => {
    const cutOf = (family: string, style: string) => catalogue.find((face) => face.family === family && face.style === style)
    const inter = ['Regular', 'Bold', 'Italic', 'BoldItalic'].map((style) => cutOf('Inter', style))
    expect(inter.filter((face) => face === undefined), 'Inter must declare all four cuts, or this discrimination proof has nothing to compare').toEqual([])
    const [regular, bold, italic, boldItalic] = inter.map((face) => instanceOfFile(faceFile(face as CatalogueFace)))
    expect([regular.family, bold.family, italic.family, boldItalic.family], 'all four call themselves the same family, which is exactly why the family check alone cannot separate them').toEqual(['Inter', 'Inter', 'Inter', 'Inter'])
    expect([regular.subfamily, bold.subfamily, italic.subfamily, boldItalic.subfamily]).toEqual(['Regular', 'Bold', 'Italic', 'Bold Italic'])
    expect([regular.usWeightClass, bold.usWeightClass, italic.usWeightClass, boldItalic.usWeightClass]).toEqual([400, 700, 400, 700])
    expect([regular.macStyle, bold.macStyle, italic.macStyle, boldItalic.macStyle]).toEqual([0x0000, 0x0001, 0x0002, 0x0003])
    expect(regular.italicAngle).toBe(0)
    expect(bold.italicAngle).toBe(0)
    expect(italic.italicAngle).toBeLessThan(0)
    expect(boldItalic.italicAngle).toBeLessThan(0)
  })

  // A FAMILY INSTALLS THE CUTS IT HAS, AND AN ABSENCE IS A FIRST-CLASS ANSWER
  // (CAP-2). Six families publish no italic at all, and DM Sans's italic is
  // WITHHELD BY RULING rather than missing upstream — its upstream binary
  // cannot prove its own style, so shipping it would put a face in the tier
  // that the per-row assertion above would have to be weakened to admit.
  //
  // Asserted so the ragged shape is a recorded decision rather than something a
  // later reader repairs: a batch that quietly "completed" these families would
  // red here and have to say which it had found and where.
  it('declares the cuts each family publishes and no more, with the short families named', () => {
    const stylesOf = (family: string) => catalogue.filter((face) => face.family === family).map((face) => face.style).sort()
    const short = [...new Set(catalogue.map((face) => face.family))].filter((family) => stylesOf(family).length < 4).sort()
    expect(short, 'the families that declare fewer than four cuts, each for a recorded reason').toEqual([
      // No italic at all upstream — measured from each project's own archive.
      'Fira Code', 'Noto Sans Thai Looped', 'Noto Serif Thai', 'Oswald', 'Roboto Slab', 'Space Grotesk',
      // WITHHELD, NOT ABSENT: upstream publishes DMSans-Italic.ttf and its
      // fsSelection ITALIC bit is clear while its italicAngle is -10, so it
      // cannot prove its own style. Its bold-italic sibling is withheld with it
      // rather than shipping a bold italic with no italic beside it.
      'DM Sans',
      // ROBOTO IS SHORT FOR A DIFFERENT REASON AND IT IS NOT AN UPSTREAM GAP.
      // `Roboto Bold`, `Roboto Italic` and `Roboto Bold Italic` ship as
      // HARDCODED CORE release assets, not as catalogue rows; redeclaring them
      // here would emit a second byte-identical asset per cut, and retiring the
      // hardcoded copies would move the 30/30 core pin. The family's four faces
      // are assembled across the two halves of `scripts/build-wasm.mjs`.
      'Roboto',
    ].sort())
    for (const family of short) expect(stylesOf(family), `${family} must declare its Regular whatever else it is short of`).toContain('Regular')
    expect(stylesOf('DM Sans')).toEqual(['Bold', 'Regular'])
    expect(stylesOf('Roboto'), 'the catalogue declares Roboto\'s base alone; its cuts are hardcoded core faces').toEqual(['Regular'])
  })

  // STORY 8.6. THE DECLARED COVERAGE AGREES WITH THE BINARY'S OWN cmap.
  it('declares, for every catalogue face, exactly the scripts its own cmap can draw', () => {
    // NON-VACUITY: the probe table is what every assertion below reads, and a
    // manifest declaring a script it does not name would be scored against
    // nothing at all.
    const vocabulary = Object.keys(scriptProbes)
    expect(vocabulary.length, 'the probe table is empty, so every assertion below is vacuous').toBeGreaterThan(0)

    for (const face of catalogue) {
      const where = `${face.id} (${path.relative(designerRoot, faceFile(face))})`
      expect(Array.isArray(face.scripts) && face.scripts.length > 0, `${where}: font-catalogue.json declares no scripts; the designer proposes a fallback tail from this list, and a face claiming nothing would be given a fallback for every script including its own`).toBe(true)
      expect(face.scripts.filter((script) => !vocabulary.includes(script)), `${where}: declares a script outside the closed vocabulary ${vocabulary.join(', ')}. An unrecognised script proposes no fallback for itself and the chain draws tofu.`).toEqual([])

      const covered = cmapCoverage(faceFile(face))
      expect(covered.size, `${where}: its cmap maps no codepoint at all, so the coverage comparison below would assert nothing`).toBeGreaterThan(0)

      for (const script of vocabulary) {
        const probes = scriptProbes[script] as ReadonlyArray<number>
        const mapped = probes.filter((codepoint) => covered.has(codepoint))
        const hex = (list: ReadonlyArray<number>) => list.map((codepoint) => `U+${codepoint.toString(16).toUpperCase().padStart(4, '0')}`).join(', ')
        if (face.scripts.includes(script)) {
          // CLAIMED: the binary must draw all of them. A face claiming a
          // script it cannot draw gets NO shipped fallback for that script,
          // so the document renders tofu where it promised coverage.
          expect(mapped, `${where}: font-catalogue.json claims the script '${script}' and the face's own cmap maps only ${hex(mapped)} of ${hex(probes)}. A claimed script gets no fallback entry, so the chain would draw tofu.`).toEqual(probes)
        } else {
          // NOT CLAIMED: the binary must draw none of them. A face that does
          // cover a script it does not claim gets a redundant shipped face
          // stapled behind it in every chain that picks it.
          expect(mapped, `${where}: font-catalogue.json does not claim the script '${script}' and the face's own cmap maps ${hex(mapped)}. Every chain picking this family would carry a redundant shipped fallback for a script it already covers.`).toEqual([])
        }
      }
    }
  })

  // AND THE RULES THE GENERATOR EMITS DECLARE NO WEIGHT AND NO STYLE, so every
  // other weight the chrome asks for stays browser-synthesised from the one
  // face above. Read out of the generator SOURCE, which is tracked, rather than
  // out of `src/generated/runtime-fonts.css`, which is gitignored and only
  // exists after `build:wasm` — a guard whose strength depends on build order
  // is a guard that goes quietly vacuous.
  it('emits one catalogue rule per declared face, carrying no font-weight and no font-style', () => {
    const generator = fs.readFileSync(generatorPath, 'utf8')

    // THE LOOP IS THE MANIFEST'S, non-vacuously: the emitter must interpolate
    // the catalogue's own family and filename, so a template that had drifted
    // onto a literal list reds here rather than silently emitting six rules.
    const emitter = /catalogueFaces\.map\(\(face\) => `(@font-face \{[^`]*\})\\n`\)/.exec(generator)
    expect(emitter, `no catalogue @font-face emitter found in ${generatorPath}; the parse below would assert nothing`).not.toBeNull()
    const rule = (emitter as RegExpExecArray)[1]
    // THE FAMILY IS THE DERIVED CUT NAME, NOT THE ROW'S BASE FAMILY
    // (spec-install-all-face-cuts, story 3). It was `${face.family}` while a
    // family had one row; emitting four cuts under one bare family name with
    // no descriptors would let the last rule win for every cut of every
    // family. `cssFamily` is the generator's own (family, style) derivation,
    // and the two spellings are tied by the uniqueness assertion above, which
    // recomputes it here and holds it against the thirteen shipped names.
    expect(rule, 'the catalogue emitter must name each rule by its derived cut name, or four cuts collapse onto one CSS family').toContain('${face.cssFamily}')
    expect(rule, 'a bare ${face.family} would emit four rules under one name and silently keep only the last').not.toContain('${face.family}\'')
    expect(rule).toContain('${face.filename}')
    expect(rule).toContain("format('truetype')")
    expect(rule, 'a font-weight descriptor would declare a weight matrix this story does not ship (AC6)').not.toContain('font-weight')
    expect(rule, 'a font-style descriptor would declare an italic or oblique this story does not ship (AC6)').not.toContain('font-style')

    // And the generator reads the manifest rather than a hardcoded list.
    expect(generator).toContain("readFileSync(join(designerRoot, 'font-catalogue.json'), 'utf8')")
  })
})

// ---------------------------------------------------------------------------
// THE HARDCODED-SLOT FACES, HELD TO THEIR OWN BYTES (Story 11.1, D-11.1.7).
//
// THE POPULATION NOTHING VERIFIED. Every metadata assertion above is quantified
// over the CATALOGUE — `font-catalogue.json`'s entries, each held to upright
// Regular 400. The thirteen faces `scripts/build-wasm.mjs` fingerprints by NAME
// are a different population, they reach the browser by a hand-written
// `@font-face` rather than by the catalogue emitter, and until this story not
// one metadata claim was made about any of them. A `pyftsubset` cut, a swapped
// weight or a variable build dropped into one of those slots satisfied every
// gate in this repository.
//
// STORY 11.1 IS WHAT MAKES THAT GAP LOAD-BEARING, which is why it is closed
// here rather than deferred: seven of the thirteen are now NON-Regular, so this
// is the first time the one unguarded population contains a face whose
// correctness is not "Regular 400". The assertion is therefore per face against
// its INTENDED instance — never a blanket Regular, which would be false the
// moment the cuts landed and would have to be deleted rather than widened.
//
// ⚠ THE CSS FAMILY AND THE FILE'S OWN FAMILY ARE DIFFERENT STRINGS FOR A CUT,
// AND THAT IS CORRECT. `Noto Sans Bold` is the `fonts.Shipped()` key, the
// `@font-face` family and the family the canvas paints with — one readable
// string on three surfaces (D-11.1.5). The face's own `name` table calls itself
// family `Noto Sans`, subfamily `Bold`, because that is what it IS. Nothing may
// derive one from the other: a `TrimSuffix(key, " Bold")` anywhere reinstates
// the naming-convention weight carrier D-B foreclosed. The table below states
// both, separately, and each is checked against a different authority — the
// CSS family against the generator's own rule, the sfnt family against the
// bytes.
// ---------------------------------------------------------------------------

/**
 * The `assets` half of the generator: slot name -> source path under the
 * designer root, for every slot fingerprinted out of `public/fonts`.
 *
 * DUPLICATED FROM `src/font-binary-identity.test.ts`, deliberately and on this
 * repository's standing convention: importing it would register that file's
 * whole suite a second time under this one, and this guard must be able to
 * redden on its own. The `wasm`, `wasmExec` and `starter` slots are
 * fingerprinted from build products rather than from a committed path and are
 * not matched by construction.
 */
function shippedSlotSourcePaths(generator: string): Readonly<Record<string, string>> {
  const entries = [...generator.matchAll(/(\w+):\s*fingerprint\(join\(designerRoot,\s*((?:'[^']*'\s*,\s*)*'[^']*')\)\s*,/g)]
  return Object.fromEntries(entries.map((match) => [match[1], [...match[2].matchAll(/'([^']*)'/g)].map((segment) => segment[1]).join('/')]))
}

/** The `@font-face` half: the `assets` slot each hand-written rule interpolates -> the family it declares. */
function slotCssFamilies(generator: string): Readonly<Record<string, string>> {
  return Object.fromEntries([...generator.matchAll(/@font-face \{ font-family: '([^']+)'; src: url\('\.\/runtime\/\$\{assets\.(\w+)\}'\) format\('truetype'\); font-display: swap; \}/g)].map((match) => [match[2], match[1]]))
}

/**
 * WHAT EACH HARDCODED SLOT IS INTENDED TO BE. `cssFamily` is the name the
 * browser is given; `family`/`subfamily` are what the binary must call itself;
 * `bold`/`italic`/`oblique` are the intent the OS/2, head and post tables are
 * then held to agree with, so a face cannot claim Bold in its name and Regular
 * in its bits.
 *
 * ⚠ `oblique` IS PER FACE AND THE SHIPPED ITALICS DISAGREE ON IT. fsSelection
 * bit 9 (0x0200) is the one style bit this table cannot state as a rule, and
 * every value below is MEASURED off the committed binary with fontTools 4.63.0,
 * not predicted:
 *
 *   Roboto Italic       0x0201  ITALIC + OBLIQUE
 *   Roboto Bold Italic  0x0221  ITALIC + BOLD + OBLIQUE
 *   Noto Sans Italic    0x0081  ITALIC + USE_TYPO_METRICS, no OBLIQUE
 *   Noto Sans Bold Ital 0x00a1  ITALIC + BOLD + USE_TYPO_METRICS, no OBLIQUE
 *
 * Two shipped italic cuts of the same shape, from two upstreams, differing on
 * a bit that is recorded NOWHERE ELSE in this repository. The catalogue loop
 * above asserts 0x0200 is clear for all 21 catalogue faces, so its absence
 * here would have been the one style bit the whole repository stopped watching
 * at exactly the moment it stopped being uniform. A blanket rule — "italic
 * implies oblique", or "oblique is never set" — is FALSE of this population in
 * both directions, which is why the intent is stated per row.
 *
 * A CLOSED TABLE OVER A DERIVED POPULATION: the slot names are parsed out of
 * the generator and asserted to be exactly these keys, so a fourteenth slot
 * added without a row here reds rather than shipping unverified.
 */
const shippedSlotFaces: Readonly<Record<string, { cssFamily: string; family: string; subfamily: string; usWeightClass: number; bold: boolean; italic: boolean; oblique: boolean }>> = {
  // The design system's three, and the three Story 2.2 engine faces.
  plexSans: { cssFamily: 'IBM Plex Sans', family: 'IBM Plex Sans', subfamily: 'Regular', usWeightClass: 400, bold: false, italic: false, oblique: false },
  mono: { cssFamily: 'IBM Plex Mono', family: 'IBM Plex Mono', subfamily: 'Regular', usWeightClass: 400, bold: false, italic: false, oblique: false },
  plexSansThai: { cssFamily: 'IBM Plex Sans Thai', family: 'IBM Plex Sans Thai', subfamily: 'Regular', usWeightClass: 400, bold: false, italic: false, oblique: false },
  sans: { cssFamily: 'Noto Sans', family: 'Noto Sans', subfamily: 'Regular', usWeightClass: 400, bold: false, italic: false, oblique: false },
  sansThai: { cssFamily: 'Noto Sans Thai', family: 'Noto Sans Thai', subfamily: 'Regular', usWeightClass: 400, bold: false, italic: false, oblique: false },
  // NOTO SANS SC HAS NO CUT, AND THAT IS A RULING (D-A) rather than an
  // oversight: its Regular alone is 10,595,932 bytes, so three instances would
  // take the offline payload from ~11 MB to ~45 MB. A family with no face at a
  // requested weight is a permanent shipped condition, not a corner case.
  sansCjk: { cssFamily: 'Noto Sans SC', family: 'Noto Sans SC', subfamily: 'Regular', usWeightClass: 400, bold: false, italic: false, oblique: false },
  // Story 11.1's seven. Noto Sans Thai gains BOLD ONLY — `font-index.json`
  // records the family's styles as 100–900 with no italic variants at all, and
  // upstream publishes none, so seven is the whole realizable set.
  //
  // The four derived Noto cuts inherit `0x0080` (USE_TYPO_METRICS) and no
  // OBLIQUE from their variable-font sources; the three Roboto statics come
  // from upstream's own `android/static/` build, which sets OBLIQUE on both
  // its italics and sets neither REGULAR nor USE_TYPO_METRICS on its Bold.
  sansBold: { cssFamily: 'Noto Sans Bold', family: 'Noto Sans', subfamily: 'Bold', usWeightClass: 700, bold: true, italic: false, oblique: false },
  sansItalic: { cssFamily: 'Noto Sans Italic', family: 'Noto Sans', subfamily: 'Italic', usWeightClass: 400, bold: false, italic: true, oblique: false },
  sansBoldItalic: { cssFamily: 'Noto Sans Bold Italic', family: 'Noto Sans', subfamily: 'Bold Italic', usWeightClass: 700, bold: true, italic: true, oblique: false },
  sansThaiBold: { cssFamily: 'Noto Sans Thai Bold', family: 'Noto Sans Thai', subfamily: 'Bold', usWeightClass: 700, bold: true, italic: false, oblique: false },
  robotoBold: { cssFamily: 'Roboto Bold', family: 'Roboto', subfamily: 'Bold', usWeightClass: 700, bold: true, italic: false, oblique: false },
  robotoItalic: { cssFamily: 'Roboto Italic', family: 'Roboto', subfamily: 'Italic', usWeightClass: 400, bold: false, italic: true, oblique: true },
  robotoBoldItalic: { cssFamily: 'Roboto Bold Italic', family: 'Roboto', subfamily: 'Bold Italic', usWeightClass: 700, bold: true, italic: true, oblique: true },
}

describe('the hardcoded shipped slots are the faces the generator claims they are', () => {
  const generator = fs.readFileSync(generatorPath, 'utf8')
  const slots = shippedSlotSourcePaths(generator)

  // NON-VACUITY AND THE POPULATION TIE, FIRST. Both halves are parsed out of
  // source text by regex, and a regex that stops matching yields an empty
  // object over which every loop below passes silently.
  it('reads every hardcoded font slot out of the generator, and knows what each one is meant to be', () => {
    expect(Object.keys(slots).sort(), `read the wrong hardcoded font slots out of ${generatorPath}; a slot with no row in shippedSlotFaces would ship a face nothing checks`).toEqual(Object.keys(shippedSlotFaces).sort())
    expect(Object.keys(slots).length, 'six until Story 11.1, thirteen after it').toBe(13)
    // AND THE OBLIQUE COLUMN GENUINELY DISCRIMINATES. A per-face expectation
    // whose column holds one value everywhere is a blanket rule wearing a
    // table, and would be satisfied by an assertion this population has now
    // outgrown. Roboto's two italics set 0x0200 and Noto's two do not, so both
    // values must be present for the per-face check below to be worth its row.
    expect([...new Set(Object.values(shippedSlotFaces).map((face) => face.oblique))].sort(), 'the intended-oblique column carries one value, so asserting it per face proves nothing the catalogue loop did not already prove').toEqual([false, true])
  })

  // THE CSS FAMILY IS THE GENERATOR'S, READ FROM IT. The other end of the
  // table: a rule repointed at a different slot, or a family renamed, reds
  // here rather than making the metadata assertion below check the right bytes
  // under the wrong name.
  it('declares each slot under the family name the table says it does', () => {
    const declared = slotCssFamilies(generator)
    expect(Object.keys(declared).length, `read no hand-written @font-face rules out of ${generatorPath}`).toBe(13)
    expect(declared).toEqual(Object.fromEntries(Object.entries(shippedSlotFaces).map(([slot, face]) => [slot, face.cssFamily])))
  })

  // AND THE BYTES AGREE WITH THE INTENT — per face, never a blanket Regular.
  it('ships each hardcoded slot as the instance it is intended to be, read from its own name, OS/2, head and post tables', () => {
    for (const [slot, intended] of Object.entries(shippedSlotFaces)) {
      const relative = slots[slot]
      expect(relative, `the generator declares no source path for the '${slot}' slot`).toBeDefined()
      const file = path.join(designerRoot, relative)
      expect(fs.existsSync(file), `${relative} is fingerprinted for the '${slot}' slot and is not committed`).toBe(true)
      const instance = instanceOfFile(file)
      const say = `${relative} is declared to the browser as '${intended.cssFamily}'`

      // (1) THE MACHINE-READABLE FAMILY IS THE sfnt NAME, NEVER THE CSS FAMILY.
      // For the seven cuts these two strings differ on purpose.
      expect(instance.family, `${say}, and its own name table must call itself '${intended.family}' — the CSS family is a readable KEY and nothing may derive a family from it (D-B)`).toBe(intended.family)
      expect(instance.subfamily, `${say}, and its own name table must call itself subfamily '${intended.subfamily}'`).toBe(intended.subfamily)
      expect(instance.usWeightClass, `${say} and must carry OS/2.usWeightClass ${intended.usWeightClass}`).toBe(intended.usWeightClass)

      // (2) THE BITS AGREE WITH THE NAME. A face naming itself Bold while its
      // head.macStyle and OS/2.fsSelection say Regular is the shape a swapped
      // or hand-edited binary takes, and the name check alone cannot see it.
      expect(Boolean(instance.macStyle & 0x0001), `${say}: head.macStyle bold bit must be ${intended.bold}`).toBe(intended.bold)
      expect(Boolean(instance.macStyle & 0x0002), `${say}: head.macStyle italic bit must be ${intended.italic}`).toBe(intended.italic)
      expect(Boolean(instance.fsSelection & 0x0020), `${say}: OS/2.fsSelection BOLD bit must be ${intended.bold}`).toBe(intended.bold)
      expect(Boolean(instance.fsSelection & 0x0001), `${say}: OS/2.fsSelection ITALIC bit must be ${intended.italic}`).toBe(intended.italic)
      expect(Boolean(instance.fsSelection & 0x0040), `${say}: OS/2.fsSelection REGULAR bit must be set for an upright Regular and clear for every cut`).toBe(!intended.bold && !intended.italic)
      // AND THE OBLIQUE BIT (0x0200), WHICH IS NOT DERIVABLE FROM THE OTHERS.
      // Roboto's two italics set it and Noto's two do not, so this is the one
      // style bit that must be stated per face — see the table's own note. The
      // catalogue loop above holds all 21 catalogue faces to a CLEAR oblique
      // bit; without this line the thirteen hardcoded slots were the only
      // shipped population whose 0x0200 nothing looked at, and they are now
      // the only population in which it varies.
      expect(Boolean(instance.fsSelection & 0x0200), `${say}: OS/2.fsSelection OBLIQUE bit must be ${intended.oblique} (measured off the committed binary; Roboto's italics set it, Noto's do not, and neither is derivable from the italic bit)`).toBe(intended.oblique)

      // (3) THE SLOPE IS IN THE OUTLINES, not only in a bit. An upright face
      // must measure zero; a sloped one must be genuinely sloped, and the
      // amount is upstream's (-12.02 for the Noto italics, -12 for Roboto's),
      // so the sign is what is asserted rather than a pinned constant.
      if (intended.italic) expect(instance.italicAngle, `${say} and is an italic cut, so post.italicAngle must be negative`).toBeLessThan(0)
      else expect(instance.italicAngle, `${say} and is upright, so post.italicAngle must be exactly 0`).toBe(0)

      // (4) STATIC, ALWAYS. A variable build in one of these slots would let
      // the browser and the engine disagree about which instance was drawn,
      // and neither AD-21's byte identity nor AD-17's rasterizer-only contract
      // survives that.
      expect(instance.variableTables, `${say} and must be a STATIC instance; a variable build carries an axis the engine never asked for`).toEqual([])
      expect(instance.outlineTables, `${say} and must carry TrueType outlines — the emitted rule declares format('truetype')`).toEqual(['glyf'])
    }
  })

  // AND THE READER DISCRIMINATES, so the loop above means "each face is what it
  // claims" rather than "instanceOfFile answers the same thing to everything".
  // The Regular and the Bold of ONE family are the pair that matters: they share
  // a name[1], and every check that separates them is a check this suite would
  // be worthless without.
  it('tells a Regular from the Bold cut of the same family', () => {
    const regular = instanceOfFile(path.join(designerRoot, 'public/fonts/notosans/NotoSans-Regular.ttf'))
    const bold = instanceOfFile(path.join(designerRoot, 'public/fonts/notosans-bold/NotoSans-Bold.ttf'))
    const italic = instanceOfFile(path.join(designerRoot, 'public/fonts/notosans-italic/NotoSans-Italic.ttf'))
    expect(regular.family, 'all three call themselves the same family, which is exactly why the family check alone cannot separate them').toBe(bold.family)
    expect(regular.family).toBe(italic.family)
    expect([regular.subfamily, bold.subfamily, italic.subfamily]).toEqual(['Regular', 'Bold', 'Italic'])
    expect([regular.usWeightClass, bold.usWeightClass]).toEqual([400, 700])
    expect([regular.macStyle, bold.macStyle, italic.macStyle]).toEqual([0x0000, 0x0001, 0x0002])
    expect(regular.italicAngle).toBe(0)
    expect(italic.italicAngle).toBeLessThan(0)
  })
})
