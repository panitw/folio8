import type { LocalFontFile } from './font-file'
import { faceDeclaredCopyright, faceDeclaredLicence, faceDeclaredLicenceText, faceFamilyName, faceIsVariable, faceSubfamilyName, faceWeightClass } from './font-name-table'
import { mediaTypeOf } from './font-source'

// TURNING FILES THE AUTHOR PICKED INTO FACES, AND NOTHING ELSE.
//
// THIS MODULE TOUCHES NO STORE, NO DOCUMENT AND NO NETWORK. It is a pure
// function from the bytes `font-file.ts` handed over to the faces they hold and
// the sentences that refuse the rest — which is what makes every row of the
// story's edge-case matrix drivable with a `File` and no browser storage at all.
// The store write is `App.tsx`'s, behind the acknowledgement, and the
// acknowledgement is what admits any of this.
//
// THE PER-FILE CHECKS ARE PER FILE, EXACTLY AS THEY ARE FOR A FETCHED FAMILY.
// `font-source.ts`'s `fetchCut` refuses one cut on its own evidence so that a
// family whose italic is a variable build still installs its Regular and its
// Bold; an author who picks four cuts and whose italic is a variable build gets
// the other three and is told about the one. A whole-pick refusal would make one
// bad file cost the author three good ones.
//
// NOTHING HERE ROUTES THROUGH `src/font-licence.ts`. That module decides whether
// terms a FAMILY PUBLISHES are terms this product is willing to distribute, and
// it governs the catalogue tier and keeps its allowlist. A face the AUTHOR
// supplies does not go through it: holding the right licence for a font they
// load is the author's responsibility, and this product takes no position on it.
// What is recorded is what the binary states, transcribed.

/**
 * One face read out of a picked file, BEFORE the acknowledgement.
 *
 * ⚠ IT CARRIES NO `source`, AND THE ABSENCE IS THE POINT. `source` records the
 * day the author asserted their right to the face, which is the day they
 * ANSWERED THE DIALOG — not the day they opened the picker. The two are the
 * same on almost every import and differ on the one that straddles UTC
 * midnight, so the field is stamped at the answer by `acknowledgedFace` and
 * there is no shape in which an unacknowledged face carries one.
 */
export type ImportedFace = Readonly<{
  family: string
  style: string
  licence: string
  licenceText: string
  copyright: string
  mediaType: string
  /** `OS/2.usWeightClass` when the face declares a usable one. See `faceWeightClass`. */
  weight?: number
  bytes: ArrayBuffer
}>

/**
 * An imported face with the acknowledgement stamped on it: everything the store
 * and `embedFontFamily` need.
 *
 * ⚠ `authorAcknowledged` IS THE FACT AND `source` IS THE SENTENCE ABOUT IT
 * (story 6, D4). They are stamped together, by one writer, and they say the
 * same thing in two registers: one a boolean the engine reads to admit a face
 * whose binary declares terms Folio would otherwise refuse, the other prose a
 * person reads in a document's font record. The boolean exists so that nothing
 * downstream ever has to parse the prose back — which would be a second
 * authority over one fact, and is what story 5's D3 forbids.
 *
 * IT IS `true` AND NEVER A VARIABLE. There is no shape in which an
 * unacknowledged face reaches this type: accepting the dialog is what produces
 * one, and declining imports nothing.
 */
export type AcknowledgedFace = ImportedFace & Readonly<{ source: string; authorAcknowledged: true }>

/** The faces of one family, grouped by the binaries' own name records rather than by anything the author typed. */
export type ImportedFamily = Readonly<{ family: string; faces: ReadonlyArray<ImportedFace> }>

/**
 * One file that was picked and is not among the faces, with the reason.
 *
 * `file` IS THE FILENAME AND IT GOES NO FURTHER THAN THIS SENTENCE. A refusal
 * that cannot say WHICH of four files was refused is not one the author can act
 * on; nothing on `ImportedFace` carries it, so no filename reaches the store or
 * a document.
 */
export type RefusedFontFile = Readonly<{ file: string; reason: string }>

export type FontImport = Readonly<{ families: ReadonlyArray<ImportedFamily>; refused: ReadonlyArray<RefusedFontFile> }>

/**
 * THE SUBFAMILY A FACE HAS WHEN IT DECLARES NONE, AND THE ONE THAT IS DROPPED
 * FROM THE KEY. `fontdir.faceKey` treats an absent subfamily and `Regular` as
 * the same thing — the family's default cut — and so does this.
 */
const regularStyle = 'Regular'

/**
 * THE FACE NAME A HOST'S DIRECTORY WOULD PRODUCE FOR THIS FACE — `fontdir`'s
 * `faceKey`, in TypeScript, and the two must not drift.
 *
 * THIS IS THE CONTRACT BETWEEN THE DESIGNER AND THE RENDERING HOST, and it is
 * the reason D2 is a constraint rather than a convention. A document authored
 * here may NAME its faces rather than carry them; a host resolves those names
 * out of a `FontSet` that `fontdir.Set` built from a directory on disk, keyed
 * by the binary's own `name` record 1 plus record 2 when the subfamily is
 * anything but `Regular`. If the designer keyed a face any other way — by
 * filename, by PostScript name, by the family record alone — the name the
 * document carries would not be the name the directory produces, and a
 * name-only document would fail to resolve on exactly the deployment this work
 * exists to serve.
 *
 * IT IS A JOIN, IN ONE DIRECTION ONLY, exactly as the Go side says of itself.
 * Nothing splits a face name back into its parts.
 */
export const importedFaceName = (family: string, style: string): string =>
  style === regularStyle ? family : `${family} ${style}`

/**
 * `source` FOR THE THIRD TIER, AND IT NAMES A MACHINE WITHOUT IDENTIFYING ONE.
 *
 * The other two tiers answer "which upstream project, which file in it, which
 * day" — `google/fonts — ofl/kanit/Kanit-Regular.ttf, fetched 2026-09-03`. An
 * author-supplied face has no upstream project and no path within one that
 * means anything to the recipient of a `.folio`, so it says the only true thing
 * there is to say: this face came from the person who authored the document,
 * who acknowledged on that day that they held the right to use it.
 *
 * IT CARRIES NO PATH, NO FILENAME AND NO MACHINE IDENTITY, and that is a
 * prohibition rather than an omission. A filesystem path would make the document
 * machine-specific; a filename is not identity anyway, since the family and the
 * cut are read from the binary and renaming the file changes neither; and a
 * machine name would put the author's computer into a file they send to a
 * customer. `src/test/provenance-shape.ts` holds this tier to its own shape,
 * beside the two it already holds the catalogue tiers to.
 *
 * IT SAYS `acknowledged` AND NOT `fetched`, because nothing was fetched. The
 * date is the day the author made the assertion that admitted the face.
 */
export const authorSuppliedFaceSource = (today: string): string =>
  `imported from the author's own machine, acknowledged ${today}`

/**
 * THE FACE AS IT GOES TO THE STORE, STAMPED WITH THE DAY THE ACKNOWLEDGEMENT
 * WAS MADE — which is the only day this record is about.
 *
 * `today` IS THE CALLER'S, AND IT IS COMPUTED WHEN THE AUTHOR ANSWERS rather
 * than when they opened the picker. A pick that straddles UTC midnight would
 * otherwise record a day on which nobody acknowledged anything.
 *
 * This is also the one writer of `source` on this tier, which is what
 * `font-provenance.test.ts` scrapes this file for.
 */
export const acknowledgedFace = (face: ImportedFace, today: string): AcknowledgedFace =>
  ({ ...face, source: authorSuppliedFaceSource(today), authorAcknowledged: true })

/**
 * A name record carrying a C0 or C1 control character or a DEL, which `fontdir`
 * refuses for the reason it gives: such a record is not a face name anybody can
 * type into a chain entry, and letting one become a key would put an unprintable
 * string into a document's `fonts` object and into every message that quotes it.
 */
const isControl = (value: string): boolean => {
  for (const character of value) {
    const code = character.codePointAt(0) ?? 0
    if (code < 0x20 || (code >= 0x7f && code <= 0x9f)) return true
  }
  return false
}

/**
 * READ ONE PICKED FILE. Returns the face, or the sentence that refuses it —
 * never both, and never a partial face.
 *
 * THE ORDER IS `fetchCut`'s ORDER, AND FOR ITS REASONS. The media type costs
 * nothing and is settled by the file's own name. The `fvar` filter runs next,
 * carrying `requireStaticTrueTypeTables` inside it — so a `.png` somebody
 * renamed `.ttf`, a `.woff` in disguise and a truncated download are all
 * refused by the container guard before any name record is asked for, in the
 * guard's own words. The name records are read last, over bytes already known
 * to be a static sfnt.
 *
 * ⚠ THE `fvar` REFUSAL MAY ONLY REFUSE, AND GO REMAINS THE AUTHORITY.
 * `fontset.RefuseVariableFace` still decides what enters a document; nothing
 * here admits anything. See `faceIsVariable`'s own doc for why a refuse-only
 * copy is permitted at install and forbidden at the command.
 *
 * AND ONLY ONE ABSENCE IS A REFUSAL. A missing family record cannot be keyed,
 * so the face has no name a host could resolve and there is nothing to store it
 * under. A missing copyright, licence or licence-text record is the binary
 * saying nothing, which is legal: it is transcribed as an empty string and the
 * face imports.
 */
function readPickedFace(file: LocalFontFile): Readonly<{ ok: true; face: ImportedFace }> | Readonly<{ ok: false; reason: string }> {
  const mediaType = mediaTypeOf(file.name)
  // ⚠ THE EXTENSION DECIDES WHICH MEDIA TYPE IS RECORDED, AND THE REFUSAL SAYS
  // SO IN THOSE TERMS rather than claiming the file is not a font. This module's
  // thesis is that a filename is not IDENTITY — the family and the cut come from
  // the binary and renaming changes neither — and that is untouched. But the
  // media type is a different question: `embedFontFamily` is given one, and
  // `fontdir` on the host side reads it off the extension too
  // (`fontExtensions`), so deriving it from the sfnt version here would put the
  // designer and the renderer on two rules for one fact. A valid face named
  // `BrandGrotesk.ttf.bak` is therefore refused, and the sentence tells the
  // author the one thing that fixes it.
  if (mediaType === undefined) return { ok: false, reason: 'does not end in .ttf or .otf, so this designer cannot say which kind of font file it is — rename it to the extension it really is and pick it again' }
  let family: string
  let subfamily: string
  let face: Omit<ImportedFace, 'family' | 'style'>
  try {
    if (faceIsVariable(file.bytes)) {
      return { ok: false, reason: 'is a VARIABLE font (it carries an `fvar` table), and a document may only carry a single static face — PDF 1.7 cannot express a variable font, and this designer will not pick an instance on your behalf' }
    }
    family = faceFamilyName(file.bytes)
    subfamily = faceSubfamilyName(file.bytes)
    // ONE READ, NOT TWO: the table walk is cheap but it is not free, and
    // calling it twice would let the shape below imply two questions were asked.
    const declaredWeight = faceWeightClass(file.bytes)
    face = {
      // TRANSCRIBED, NEVER COMPOSED. Name ID 0, 13 and 14, byte for byte as the
      // binary holds them, with no classification, no inference from the family
      // name and no defaulting to an identifier nobody read.
      copyright: faceDeclaredCopyright(file.bytes),
      licenceText: faceDeclaredLicenceText(file.bytes),
      licence: faceDeclaredLicence(file.bytes),
      // THE MEDIA TYPE IS THE TABLE'S, NOT THE BROWSER'S. `File.type` is the
      // operating system's guess off the same extension and is empty on some
      // hosts; the value the engine recognises is the one that must be stored.
      mediaType,
      // THE CUT'S OWN WEIGHT, FOR THE FAMILY THAT HAS NO `Regular`. An imported
      // family has no publisher to say which cut represents it, so `baseCutOf`
      // picks the one nearest upright text weight — and this is where that
      // number enters the designer. Spread for the reason `soundFace` spreads
      // it: a face declaring none must carry no key rather than an `undefined`.
      ...(declaredWeight === undefined ? {} : { weight: declaredWeight }),
      bytes: file.bytes,
    }
  } catch (error) {
    return { ok: false, reason: `is not a font this designer can read: ${error instanceof Error ? error.message : String(error)}` }
  }
  if (family === '') return { ok: false, reason: 'declares no family name (`name` table record 1), so it cannot be keyed — a face is found by the name its own binary states, never by its filename' }
  if (isControl(family) || isControl(subfamily)) return { ok: false, reason: 'declares a family or subfamily carrying control characters, which cannot be a face name' }
  // AN ABSENT SUBFAMILY IS THE DEFAULT CUT, which is `fontdir`'s rule and the
  // reason `Sarabun`'s Regular keys as `Sarabun` and not as `Sarabun Regular`.
  // Case-insensitive, because `strings.EqualFold` is.
  const style = subfamily === '' || subfamily.toLowerCase() === regularStyle.toLowerCase() ? regularStyle : subfamily
  return { ok: true, face: { ...face, family, style } }
}

/**
 * EVERY PICKED FILE, READ ON ITS OWN AND GROUPED INTO FAMILIES BY THE BINARIES'
 * OWN NAME RECORDS.
 *
 * FILES FROM TWO FAMILIES PICKED TOGETHER BECOME TWO FAMILIES, and four cuts of
 * one become one family of four. Nothing about the gesture decides that —
 * picking is how the author hands files over, and what they ARE is a property of
 * the bytes.
 *
 * FAMILIES COME BACK IN FIRST-APPEARANCE ORDER, so the dialog that asks for the
 * acknowledgement lists them in the order the author picked them rather than in
 * an order derived from a hash.
 *
 * `familyIsTaken` IS THE CALLER'S — `localTierHolds`, in the designer. It is an
 * argument rather than an import so this module stays a pure function of what
 * it is handed, and so a test can drive the collision without the catalogue.
 */
export function importFontFiles(files: ReadonlyArray<LocalFontFile>, familyIsTaken: (family: string) => boolean = () => false): FontImport {
  const grouped = new Map<string, ImportedFace[]>()
  const refused: RefusedFontFile[] = []
  // WHICH FILE CLAIMED EACH FACE NAME, so the duplicate report can name BOTH
  // files rather than only the loser — `fontdir.Set`'s `keyedFrom`, for the
  // same reason and with the same resolution: the FIRST file picked wins and
  // the other comes back as a skip. Determinism matters more than which one
  // wins, and no rule anywhere can tell which of two `Bold` files the author
  // meant.
  const keyedFrom = new Map<string, string>()
  for (const file of files) {
    const read = readPickedFace(file)
    if (!read.ok) { refused.push({ file: file.name, reason: read.reason }); continue }
    const { family, style } = read.face
    // A NAME THE DESIGNER ALREADY OFFERS IS REFUSED, AND REFUSED OUT LOUD.
    // `offeredFamilies` gives one row per family and lets the committed
    // catalogue's row win, so a face imported under a catalogue family's name
    // would be written to the store, reported as kept, and never appear
    // anywhere the author could reach it. Two faces cannot share one name in
    // this product; the honest moment to say so is the one the author acted in.
    if (familyIsTaken(family)) {
      refused.push({ file: file.name, reason: `declares the family ${family}, which this designer already offers from its own catalogue — two different faces cannot share one family name, and a face is named by its own binary, so this one cannot be renamed` })
      continue
    }
    const name = importedFaceName(family, style)
    const first = keyedFrom.get(name)
    if (first !== undefined) {
      refused.push({ file: file.name, reason: `declares the same face ${name} as ${first}, which was picked first — nothing here can tell which of two files you meant` })
      continue
    }
    keyedFrom.set(name, file.name)
    const held = grouped.get(family)
    if (held === undefined) grouped.set(family, [read.face])
    else held.push(read.face)
  }
  return { families: [...grouped].map(([family, faces]) => ({ family, faces })), refused }
}

/**
 * THE PER-FILE REFUSALS, AS ONE SENTENCE PER FILE, NAMED BY FILENAME.
 *
 * THE FILENAME IS THE ONLY HANDLE THE AUTHOR HAS ON A FILE THEY JUST PICKED —
 * they have not seen its name records, and four cuts of one family differ by
 * nothing else on screen. It appears HERE, in a message, and nowhere in a
 * record: `ImportedFace` has no field for it, so nothing that reaches the store
 * or a document can carry it.
 *
 * EMPTY IN, EMPTY OUT. A pick with nothing refused has nothing to report, and
 * the caller joins this into its own line rather than printing a heading over
 * a blank.
 */
export const refusedFontFileReport = (refused: ReadonlyArray<RefusedFontFile>): string =>
  refused.map((entry) => `${entry.file} ${entry.reason}.`).join(' ')
