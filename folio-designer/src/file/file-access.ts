// The file boundary carries opaque bytes and honest local-file metadata only.
// It deliberately has no knowledge of the .folio document structure.
export type FileTarget = Readonly<{
  kind: 'in-place'
  name: string
  handle: LocalFileHandle
}>

export type LocalFileHandle = Readonly<{
  name: string
  getFile(): Promise<File>
  createWritable(): Promise<WritableFile>
}>

export type WritableFile = Readonly<{
  write(bytes: ArrayBuffer): Promise<void>
  close(): Promise<void>
}>

export type LocalFile = Readonly<{
  bytes: ArrayBuffer
  name: string
  target?: FileTarget
}>

export type SavedLocalFile = Readonly<{
  name: string
  target?: FileTarget
}>

export type SaveRequest = Readonly<{
  bytes: ArrayBuffer
}>

export type SaveTargetRequest = Readonly<{
  suggestedName: string
  currentTarget?: FileTarget
  saveAs: boolean
  format: LocalFileFormat
}>

// Target acquisition is deliberately separate from writing. Chromium's save
// picker needs the click's transient user activation; serialization may take
// long enough that waiting for it first would lose that activation.
export type AcquiredSaveTarget = Readonly<{
  name: string
  target?: FileTarget
  format: LocalFileFormat
}>

export interface FileAccess {
  open(): Promise<LocalFile>
  acquireSaveTarget(request: SaveTargetRequest): Promise<AcquiredSaveTarget>
  writeSave(target: AcquiredSaveTarget, request: SaveRequest): Promise<SavedLocalFile>
}

export class FileAccessCancelled extends Error {
  constructor() { super('Local file selection was cancelled') }
}

export class FileAccessFailure extends Error {
  constructor(message: string) { super(message) }
}

export function isFileAccessCancelled(error: unknown): error is FileAccessCancelled {
  return error instanceof FileAccessCancelled
}

// WHAT THREW, IN ONE BOUNDED PHRASE — the same ruling `engine.worker.ts` makes
// at its own boundary, for the same reason, arrived at the same way.
//
// Every tier here used to collapse an unanticipated throw into one fixed
// sentence: `new FileAccessFailure('Could not save local file')`, with the
// DOMException's `name` and `message` dropped on the floor. That sentence is
// the least useful in the product. It is never a refusal this code authored —
// a cancel is already its own type — so it fires exactly when nobody
// anticipated the failure, and it is exactly then that it erases the only
// evidence there is. A save that fails on alternate attempts reads as "Could
// not save local file" both times, and an author can photograph that screen
// all day without anyone being able to say whether the file was locked, the
// permission had lapsed, or the disk was full. `NoModificationAllowedError`
// and `NotAllowedError` are different problems with different fixes, and the
// browser had already named which one it was.
//
// The cause is bounded rather than trusted: it is a string this code did not
// author — a DOM exception's, or V8's — so it is cut like every other foreign
// string that crosses a boundary in this application.
export function describeFileThrow(thrown: unknown): string {
  if (thrown instanceof DOMException) return `${thrown.name}: ${thrown.message}`.slice(0, 200)
  if (thrown instanceof Error) return `${thrown.name || 'Error'}: ${thrown.message}`.slice(0, 200)
  if (typeof thrown === 'string') return thrown.slice(0, 200)
  return Object.prototype.toString.call(thrown).slice(0, 200)
}

// The sentence an author reads: what this boundary was doing, then what the
// browser called the thing that stopped it.
export const fileFailureFor = (thrown: unknown, doing: string): FileAccessFailure => new FileAccessFailure(`${doing}: ${describeFileThrow(thrown)}`)

// THE LOCAL FILE FORMAT, AND WHY IT IS ONE VALUE RATHER THAN THREE CONSTANTS.
//
// A save has exactly three format-shaped facts: the picker's own description,
// the MIME type the blob carries, and the extension the suggested name ends
// with. Before Story 13.1 all three were `.folio` literals in three different
// modules — `folio8Name` here, `folio8PickerType` in `file-system-access.ts`, and
// the blob `type` in `input-download.ts` — so a second format could be added by
// changing two of them and shipping a PDF named `.folio`, or a `.pdf` written
// as `application/json`. Carrying them together, and carrying the SAME value on
// the acquired target the write reads, makes it structurally impossible for the
// bytes to disagree with the picker that named them (AD-20: one file-access
// interface, parameterised, never a second tier).
export type LocalFileFormat = Readonly<{ description: string; mimeType: string; extension: string }>

export const folioFileFormat: LocalFileFormat = { description: 'folio8 template', mimeType: 'application/json', extension: '.folio' }
export const pdfFileFormat: LocalFileFormat = { description: 'PDF document', mimeType: 'application/pdf', extension: '.pdf' }
// STORY 5 (startup templates) — SAVE SAMPLE DATA'S FORMAT, AND IT IS THE SAME
// VALUE THE SAMPLE PICKER ALREADY OPENS WITH. `sample-file.ts` builds its open
// picker type from this constant, so the wording an author reads when they load
// a sample and the wording they read when they save one cannot drift apart.
export const jsonSampleFileFormat: LocalFileFormat = { description: 'JSON sample data', mimeType: 'application/json', extension: '.json' }

// THE FORMATS WHOSE TRAILING EXTENSION `localFileName` WILL STRIP — and
// `jsonSampleFileFormat` IS DELIBERATELY NOT ONE OF THEM.
//
// Membership here is not "every format this boundary knows"; it is a claim that
// the suffix is this application's OWN output and may therefore be replaced when
// the author saves the same document as something else. `.folio` and `.pdf` are
// that. `.json` is not: it is the author's data file, and the name it ends with
// belongs to them.
//
// It is a SHIPPED-NAMING question rather than a taste one. A template can be
// titled `data.json` — the download tier's open input accepts
// `application/json`, and `installOpenedDocument` takes the title from the file
// name — and adding `.json` to this set silently re-offers that template as
// `data.folio` instead of `data.json.folio`, with the same loss on the PDF path.
// Story 5 adds a save; it does not get to rename the other two.
//
// The sample save needs nothing from this set: its own suffix is `.json`, so
// `endsWith(wanted)` below returns the name unchanged before the strip is ever
// reached, and a sample named without one simply gains it.
const knownFileFormats: ReadonlyArray<LocalFileFormat> = [folioFileFormat, pdfFileFormat]

// STRIPPING IS SYMMETRIC ACROSS BOTH FORMATS, DELIBERATELY, AND THIS IS THE
// RULING RATHER THAN AN ACCIDENT OF THE LOOP.
//
// A trailing extension belonging to EITHER known format is removed before the
// requested one is appended, in both directions: `statement.folio` is offered
// as `statement.pdf`, and `statement.pdf` is offered as `statement.folio` —
// not `statement.pdf.folio`. The asymmetric variant (strip only on the PDF
// path, because that is the direction this story needed) was considered and
// REJECTED: it produces `statement.pdf.folio` from a title a PDF save just
// suggested, and it can only be defended by a comment explaining why one
// direction is special, which is exactly the kind of rule a later
// simplification deletes. Symmetry needs no defence.
//
// AT MOST ONE extension is removed, and the strips do not chain. Folding over
// the format list removed BOTH — `report.folio.pdf` lost `.pdf`, and the
// result then ended in `.folio` and lost that too, so a `.folio` save produced
// `report.folio.folio`. One match, one slice.
//
// THE COMPARISON IS CASE-FOLDED ON BOTH SIDES. Both shipped extensions are
// lowercase today, so a bare `endsWith(format.extension)` happens to work — a
// guard that holds only because of a coincidence in its inputs is not a guard,
// and a format declared `.PDF` would double-suffix forever.
//
// ⚠ BUT CASE-FOLDING IS FOR RECOGNISING THE SUFFIX, NEVER FOR REWRITING IT. A
// name that already ends in the requested extension is returned UNCHANGED,
// casing and all, and that early return is the whole of the difference between
// `REPORT.FOLIO` and `REPORT.folio`.
//
// THE TWO RULES LOOK INCONSISTENT AND ARE NOT — stripping is symmetric across
// both formats on purpose, and casing is preserved on purpose — so the reason
// they differ is written here rather than left for someone to "fix". They
// differ in what a wrong answer COSTS. The worst case of a wrong strip is a
// pre-filled string in a save dialog that the author reads and overtypes. The
// worst case of a wrong case-fold is that they never see it: on a
// case-insensitive filesystem `REPORT.folio` silently overwrites
// `REPORT.FOLIO`, and on a case-sensitive one the author now holds two
// templates that differ by one keystroke, with nothing to say which one their
// next Open will pick up. They find out by losing work. A normalisation that
// can fork one document into two files is not cosmetic.
export function localFileName(name: string, format: LocalFileFormat): string {
  const cleaned = name.trim().replace(/[\\/]/g, '')
  const wanted = format.extension.toLowerCase()
  if (cleaned.toLowerCase().endsWith(wanted)) return cleaned
  // Longest match first, so a future format whose extension ends with another's
  // cannot leave the shorter one's tail behind.
  const trailing = [...knownFileFormats].sort((left, right) => right.extension.length - left.extension.length).find((known) => cleaned.toLowerCase().endsWith(known.extension.toLowerCase()))
  // The untitled fallback runs AFTER the strip, not before: `.folio` is a
  // stem-less name once its extension is removed, and appending to nothing
  // produced the dotfile `.pdf`. It is reached only on the CROSS-format path —
  // `.folio` saved as a `.folio` returns unchanged above, which is the shipped
  // behaviour and is deliberately left alone.
  const base = (trailing ? cleaned.slice(0, -trailing.extension.length) : cleaned) || 'untitled'
  return base.toLowerCase().endsWith(wanted) ? base : `${base}${format.extension}`
}
