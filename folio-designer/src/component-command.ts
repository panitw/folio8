// Component commands are opaque, versioned Go vocabulary. This module does
// not describe a .folio element or serialize a document; it only encodes the
// final intent that crosses the existing worker FIFO.
//
// STORY 15.2a: every value on the wire now goes through command-json.ts. This
// file used to hand-roll its own escape table AND splice `id`, `type` and
// `band` in raw. Quoting is the whole fix for the splice — a crafted id becomes
// one string value, and the engine answers with a located refusal for an
// element it cannot find, which is correct behaviour rather than a workaround.
//
// ⚠ THE RAW id SPLICE WAS HYGIENE, NOT THE SEVERITY, and this file is where a
// future reader is most likely to get that backwards. A crafted component id
// cannot arrive from an opened document: the Go loader's validateElementID
// (AD-10/AC34) admits only `^e[0-9a-z]+$` for every element id at parse time.
// `engine-protocol.ts` is the INBOUND projection guard — a different direction
// and a different population, and it can only ever carry ids the loader already
// admitted.
//
// THE SEVERITY LIVES ON bindComponentScalarCommand BELOW. Its segments are JSON
// object keys taken verbatim from the author's sample-data file, nothing
// constrains a JSON key, and they went through the quoter this file used to
// hand-roll. Open a data file, click a node, press Connect — no typing.
import { commandBytes, jsonArray, jsonBoolean, jsonNumber, jsonString } from './command-json'

export type PaletteKind = 'text' | 'image' | 'table' | 'line' | 'rect' | 'barcode' | 'qrcode'

const point = (value: number): string => jsonNumber(Math.round(value * 1000) / 1000)

// Canvas projections and interaction drafts use millipoints. Commands use
// canonical point literals, so the conversion occurs once at this opaque
// boundary rather than accidentally sending projection units to Go.
const millipoints = (value: number): string => point(value / 1000)

// SPEC-multi-pages story 3: `page` is the 0-based designed page a content
// element is placed on or moved to. It is written last and only when given, so
// a page-1 command keeps today's exact bytes.
const pageField = (page: number | undefined) => page === undefined ? [] : [['page', jsonNumber(page)] as const]

export function createComponentCommand(type: PaletteKind, band: 'pageHeader' | 'content' | 'pageFooter', x: number, y: number, snap: boolean, page?: number): ArrayBuffer {
  return commandBytes('createComponent', [['type', jsonString(type)], ['band', jsonString(band)], ['x', point(x)], ['y', point(y)], ['width', jsonNumber(72)], ['height', jsonNumber(24)], ['snap', jsonBoolean(snap)], ...pageField(page)])
}
export function dropComponentCommand(type: PaletteKind, pageX: number, pageY: number, snap: boolean, page?: number): ArrayBuffer {
  return commandBytes('dropComponent', [['type', jsonString(type)], ['x', point(pageX)], ['y', point(pageY)], ['snap', jsonBoolean(snap)], ...pageField(page)])
}
export function moveComponentCommand(id: string, x: number, y: number, snap: boolean): ArrayBuffer {
  return commandBytes('moveComponent', [['id', jsonString(id)], ['x', millipoints(x)], ['y', millipoints(y)], ['snap', jsonBoolean(snap)]])
}
export function resizeComponentCommand(id: string, width: number, height: number, snap: boolean): ArrayBuffer {
  return commandBytes('resizeComponent', [['id', jsonString(id)], ['width', millipoints(width)], ['height', millipoints(height)], ['snap', jsonBoolean(snap)]])
}
// One rectangle, one command. Any anchor other than the bottom-right corner
// moves the origin while it sizes, and move-then-resize would be two history
// entries for one drag with an intermediate rectangle nobody asked for.
export function setComponentBoundsCommand(id: string, x: number, y: number, width: number, height: number, snap: boolean): ArrayBuffer {
  return commandBytes('setComponentBounds', [['id', jsonString(id)], ['x', millipoints(x)], ['y', millipoints(y)], ['width', millipoints(width)], ['height', millipoints(height)], ['snap', jsonBoolean(snap)]])
}
export function deleteComponentCommand(id: string): ArrayBuffer {
  return commandBytes('deleteComponent', [['id', jsonString(id)]])
}
export function duplicateComponentCommand(id: string, snap: boolean): ArrayBuffer {
  return commandBytes('duplicateComponent', [['id', jsonString(id)], ['snap', jsonBoolean(snap)]])
}
// A whole selection in one command, so a group delete or paste is one undo
// entry. Go validates every id before it mutates anything.
export function deleteComponentsCommand(ids: ReadonlyArray<string>): ArrayBuffer {
  return commandBytes('deleteComponents', [['ids', jsonArray(ids.map(jsonString))]])
}
// SPEC-multi-pages: `page` pastes the content copies onto that page; omitted,
// every copy stays in its source's band and page (today's bytes).
export function duplicateComponentsCommand(ids: ReadonlyArray<string>, snap: boolean, page?: number): ArrayBuffer {
  return commandBytes('duplicateComponents', [['ids', jsonArray(ids.map(jsonString))], ['snap', jsonBoolean(snap)], ...pageField(page)])
}

// The tree supplies decoded JSON object-key segments only. Go owns the
// expression grammar, root/params scope, target eligibility, canonical
// mutation, and diagnostics; this factory intentionally does none of those.
export function bindComponentScalarCommand(id: string, segments: ReadonlyArray<string>): ArrayBuffer {
  // This encodes JSON transport only; it does not turn a key into a folio8
  // expression. Complete escaping keeps decoded keys unambiguous until Go
  // verifies the exact segment sequence.
  return commandBytes('bindComponentScalar', [['id', jsonString(id)], ['segments', jsonArray(segments.map(jsonString))]])
}

export function bindTableCollectionCommand(id: string, segments: ReadonlyArray<string>): ArrayBuffer {
  return commandBytes('bindTableCollection', [['id', jsonString(id)], ['segments', jsonArray(segments.map(jsonString))]])
}

// The query and final command use the same centrally encoded movement intent.
export function moveComponentsCommand(ids: ReadonlyArray<string>, referenceId: string, dx: number, dy: number, snap: boolean, expectedRevision: number, constrainToWindow?: boolean, page?: number): ArrayBuffer {
  return commandBytes('moveComponents', [['ids', jsonArray(ids.map(jsonString))], ['referenceId', jsonString(referenceId)], ['dx', millipoints(dx)], ['dy', millipoints(dy)], ['snap', jsonBoolean(snap)], ['expectedRevision', jsonNumber(expectedRevision)], ...(constrainToWindow === undefined ? [] : [['constrainToWindow', jsonBoolean(constrainToWindow)] as const]), ...pageField(page)])
}
