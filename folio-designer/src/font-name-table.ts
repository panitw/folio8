// THE ONE sfnt `name`-TABLE READER, AND WHY IT IS PRODUCTION SOURCE.
//
// Story 16.1 needs nameID 0 — a face's own copyright — AT RUNTIME, over bytes
// fetched seconds earlier from a third party. Two hand-written `DataView` walks
// answering that question already existed in this repository and NEITHER could
// be imported by the browser: one is a vitest file (`font-catalogue.test.ts`),
// one is a build script (`scripts/build-wasm.mjs`). A third hand-copy would have
// been the first one whose divergence nobody would notice, because the two
// existing copies are checked against each other by the very test that holds one
// of them. So the walk is extracted HERE, once, and both existing callers are
// switched onto it.
//
// NO PARSER DEPENDENCY, AND THAT IS A STANDING DECISION rather than an omission.
// `src/font-catalogue.test.ts` records the ground: `package.json`'s
// `dependencies` are exactly `pdfjs-dist`, `react` and `react-dom`, the bundle
// sits under a measured payload budget, and reading four integers out of a table
// directory is a short walk. Story 16.1 adds no dependency.
//
// NO `Buffer`, DELIBERATELY. Both copies this replaces used `Buffer.swap16()`,
// which is Node-only. This module runs in the browser, in vitest and under plain
// `node` (`scripts/build-wasm.mjs` imports it directly — Node 24 strips the
// types), so the two encodings the `name` table actually uses are decoded by
// hand below.
//
// THIS MODULE ANSWERS A DIFFERENT QUESTION FROM GO'S READER, and the duplication
// is forced rather than chosen. `copyright` is one of `embedFontFamily`'s twelve
// wire fields, so Go cannot supply an input to itself; Go reads the name table
// again, from the same bytes, for its own question (Story 16.1b's licence tie).
// Two readers answering two questions is correct here — one reader would be a
// check over its own input.

export type SfntTable = Readonly<{ offset: number; length: number }>

/**
 * The table directory: every four-character tag the file declares, with the
 * offset and length it declares for it. No version check — some callers want
 * one and some do not, so it is `requireStaticTrueTypeTables` that refuses.
 */
export function sfntTableDirectory(view: DataView): Readonly<Record<string, SfntTable>> {
  const tables: Record<string, SfntTable> = {}
  const count = view.getUint16(4)
  for (let index = 0; index < count; index++) {
    const record = 12 + index * 16
    let tag = ''
    for (let byte = 0; byte < 4; byte++) tag += String.fromCharCode(view.getUint8(record + byte))
    tables[tag] = { offset: view.getUint32(record + 8), length: view.getUint32(record + 12) }
  }
  return tables
}

/**
 * The same directory, but only for a file whose sfnt version says it is a
 * static TrueType outline font — `0x00010000` or the older `true`. An
 * OpenType/CFF (`OTTO`) or WOFF wrapper is refused here rather than being read
 * as if its offsets meant the same thing.
 */
export function requireStaticTrueTypeTables(view: DataView): Readonly<Record<string, SfntTable>> {
  // TOO SHORT TO HOLD A TABLE DIRECTORY IS THE SAME ANSWER, said in the same
  // words. A 200 carrying an error page — or two bytes — is not a font, and
  // reading its first four bytes as a version would be a `RangeError` from
  // inside the walk rather than a statement about the file.
  if (view.byteLength < 12) throw new Error(`not a static TrueType sfnt: ${view.byteLength} bytes is too short to carry a table directory`)
  const version = view.getUint32(0)
  if (version !== 0x00010000 && version !== 0x74727565) throw new Error(`not a static TrueType sfnt: version 0x${version.toString(16).padStart(8, '0')}`)
  return sfntTableDirectory(view)
}

// The two encodings a `name` record actually uses, decoded without `Buffer`.
//
// UTF-16BE for the Windows (3) and Unicode (0) platforms, latin1 for the
// Macintosh (1) platform's Roman script. The odd-length guard is not decorative:
// a record claiming platform 3 with an odd byte count is malformed, and pairing
// its bytes anyway invents a character.
const utf16BE = (view: DataView, at: number, length: number): string => {
  let out = ''
  for (let byte = 0; byte + 1 < length; byte += 2) out += String.fromCharCode(view.getUint16(at + byte))
  return out
}
const latin1 = (view: DataView, at: number, length: number): string => {
  let out = ''
  for (let byte = 0; byte < length; byte++) out += String.fromCharCode(view.getUint8(at + byte))
  return out
}

/**
 * The first readable value the `name` table carries for `nameID`, preferring a
 * Unicode-platform record and falling back to a single-byte one.
 *
 * Returns `undefined` when the face carries no `name` table at all, or carries
 * one with no record for this id. A caller that requires the value says so
 * itself — silence and absence are different answers and this reader does not
 * collapse them.
 */
export function nameTableString(view: DataView, tables: Readonly<Record<string, SfntTable>>, nameID: number): string | undefined {
  const name = tables['name']
  if (name === undefined) return undefined
  // EVERY READ IS BOUNDED BY WHAT THE TABLE ITSELF DECLARES, and by the file.
  // The offsets below come from third-party bytes fetched seconds earlier, so a
  // truncated or hostile `name` table must produce ABSENCE — which the caller
  // that requires a value turns into a stated refusal — rather than a slice of
  // whatever happens to follow, or a bare `RangeError` from the walk.
  const limit = Math.min(name.offset + name.length, view.byteLength)
  if (name.offset + 6 > limit) return undefined
  const count = view.getUint16(name.offset + 2)
  const storage = name.offset + view.getUint16(name.offset + 4)
  let singleByte: string | undefined
  for (let index = 0; index < count; index++) {
    const record = name.offset + 6 + index * 12
    if (record + 12 > limit) break
    if (view.getUint16(record + 6) !== nameID) continue
    const platform = view.getUint16(record)
    const length = view.getUint16(record + 8)
    const offset = view.getUint16(record + 10)
    if (storage + offset + length > limit) continue
    if ((platform === 3 || platform === 0) && length % 2 === 0) return utf16BE(view, storage + offset, length)
    singleByte ??= latin1(view, storage + offset, length)
  }
  return singleByte
}

/** A `DataView` over a face's bytes, whatever container they arrived in. */
export function fontView(bytes: ArrayBuffer | ArrayBufferView): DataView {
  return ArrayBuffer.isView(bytes) ? new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength) : new DataView(bytes)
}

/**
 * nameID 0 — THE ONE STATEMENT OF PROVENANCE THAT CANNOT BE EDITED FROM OUTSIDE
 * THE BINARY, and the value that reaches a `.folio` as `font.copyright`.
 *
 * It is read from the face's own bytes and never from a family's upstream
 * metadata, for the reason `font-catalogue.md` already gives: a value copied
 * from somewhere else is a second authority on the face's provenance, and the
 * first binary swap makes the document publish a statement its own bytes
 * contradict.
 *
 * ABSENT IS A REFUSAL, NOT A BLANK. The engine refuses to load a document that
 * embeds a face with an empty `copyright` (`internal/template/parse.go`'s
 * `requireEmbeddedFaceLicence`), so a pick that carried one would write a file
 * the product's own parser rejects. Blank counts as absent: `" "` states
 * exactly as much as `""`.
 *
 * THE CONTAINER IS CHECKED FIRST, BECAUSE THIS READER'S HOTTEST CALLER IS AN
 * UNTRUSTED ONE. `src/font-source.ts` calls this on bytes fetched from a third
 * party moments earlier, so the version guard `requireStaticTrueTypeTables`
 * carries is exactly the guard those bytes need: an `OTTO`/CFF or WOFF wrapper,
 * or a 200 that is not a font at all, is REFUSED here rather than walked as if
 * its offsets meant the same thing. The other two callers read the committed
 * faces, every one of which is measured sfnt version `00010000`, so the check
 * costs them nothing.
 */
export function faceCopyright(bytes: ArrayBuffer | ArrayBufferView): string {
  const view = fontView(bytes)
  const copyright = nameTableString(view, requireStaticTrueTypeTables(view), 0)?.trim()
  if (!copyright) throw new Error('this face declares no copyright in its own `name` table (nameID 0); a face embedded into a document must state whose it is, and the engine refuses to load a document that does not')
  return copyright
}

/**
 * `fvar` — A FILTER THAT MAY ONLY REFUSE, AND GO REMAINS THE AUTHORITY.
 *
 * Story 16.5 separates INSTALLING a face from EMBEDDING it, which moves every
 * refusal the embed command makes to a later moment than the one the author
 * acted in. `fontset.RefuseVariableFace` (`folio-go/internal/fontset/variableface.go`)
 * IS STILL THE ONLY THING THAT DECIDES WHAT ENTERS A DOCUMENT. This predicate
 * decides only what is worth keeping on this machine, and it is written so that
 * it can never do more than that:
 *
 *   IT MAY ONLY REFUSE, NEVER ADMIT. A caller uses it to decline an install; no
 *   caller may use a `false` from it as permission to embed. Every way this can
 *   drift is therefore either today's behaviour or a loud failure at the moment
 *   the author acted: drift permissive and the face installs and Go refuses it
 *   at first use — exactly what happens today; drift strict and a legitimate
 *   face fails to install, loudly, in front of the person who asked for it.
 *   NEITHER OUTCOME WRITES A DOCUMENT. That asymmetry is the whole reason a
 *   second `fvar` test is permitted here and forbidden at or behind the command.
 *
 *   IT IS AN UPGRADE TO A FILTER THAT ALREADY SHIPS, NOT A NEW DOOR.
 *   `font-index.ts` already hides variable-only rows on the strength of the
 *   build-time snapshot's `axes` field, and says of itself that a hidden row is
 *   a presentation choice and that the authority stays Go. This reads the same
 *   property off THE BYTES IN HAND instead of off a field that ages between
 *   releases.
 *
 * SHAPED EXACTLY LIKE `faceCopyright` ABOVE, and for the same reason: it builds
 * the view and the table directory internally and returns a plain value, so no
 * caller has to hold a `DataView` or a directory to ask the question. That costs
 * one extra table-directory walk per fetched face — the twelve-byte header plus
 * sixteen bytes per record, no glyph parsing, no new dependency.
 *
 * THE CONTAINER GUARD RUNS FIRST, AND THE DIVERGENCE FROM GO IS DELIBERATE.
 * `requireStaticTrueTypeTables` THROWS for a 200 that is not a font, so an
 * unparsable face never reaches the `fvar` lookup at all; Go's
 * `RefuseVariableFace` returns `nil` for those same bytes, because it answers
 * exactly one question and an unparsable face is not a variable one. The two
 * sides deliberately answer differently there, `src/font-variable-face-tie.test.ts`
 * asserts that on purpose, and neither is to be widened to match the other.
 */
export function faceIsVariable(bytes: ArrayBuffer | ArrayBufferView): boolean {
  return 'fvar' in requireStaticTrueTypeTables(fontView(bytes))
}
