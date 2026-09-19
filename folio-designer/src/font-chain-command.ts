// Font chain commands are OPAQUE ENGINE INTENT, never a browser-side font
// model. Nothing here knows what a duplicate name is, what an empty chain is,
// which chains an element still names, or which faces this build ships: every
// one of those is a rule the engine owns and answers with its own sentence.
// These six builders only put the author's ask on the wire, byte for byte, in
// the exact shape folio-go/component_commands.go's six handlers read.
//
// EVERY value is encoded with JSON.stringify — the table-column encoder's
// model, and the only correct answer to "escape this for JSON". A chain name
// is author-typed and unconstrained (Go accepts any non-empty UTF-8 string up
// to 512 bytes), so `"`, `\` and a pasted C0 control all reach this module,
// and a hand-rolled escape table that misses one produces bytes Go cannot
// parse — which loses the located refusal the panel exists to display.
//
// STORY 15.2a moved that encoder into command-json.ts, which every builder in
// the designer now shares. The header above stated the rule; the import below
// is what makes it one rule rather than six ENCODER MODULES agreeing by hand.
// (Six encoder modules, not the six chain builders below — a different six.)
//
// THE FIELD COUNT IS PART OF THE CONTRACT. componentFields(raw, N) counts
// every top-level key, `kind` and `version` included, and refuses anything
// else: add 4, rename 4, delete 3, addEntry 5, moveEntry 5, removeEntry 4,
// embedFontFamily 12. An extra field is not ignored, it is a refusal — which
// is why every builder below still lists its own fields, in order, at the
// call site.
//
// STORY 11.4 CHANGED A FIELD'S SHAPE AND NO FIELD'S COUNT. `addFontChain`'s
// `entries` and `embedFontFamily`'s `tail` used to be arrays of strings and
// are now arrays of `FontChainEntryAsk` — the format's own chain-entry shape,
// so a pick can DECLARE the cuts a family has instead of writing a chain that
// can never bold. Both arities are exactly what they were.
import type { JsonField } from './command-json'
import { commandBytes, jsonArray, jsonNumber, jsonObject, jsonString } from './command-json'

const quote = jsonString
// Go reads these with commandInt, which requires an integer literal. They are
// browser-derived list positions, never author text, and they are still
// encoded rather than spliced so this module has exactly one encoder.
const index = jsonNumber

/**
 * A CHAIN ENTRY AS A COMMAND MAY ASK FOR IT — the FORMAT'S OWN chain-entry
 * shape, minus the one arm a command may not write (Story 11.4, route C).
 *
 * A bare string is a face name. An object is a `face` discriminant plus any of
 * the CLOSED variant set — `bold`, `italic`, `boldItalic` — naming the face
 * that entry is drawn in at that weight and slope. There is deliberately no
 * `asset` arm and no way to add one: `folio-format.md` gives an entry three
 * legal shapes and this type admits two of them.
 *
 * ⚠ THAT OMISSION IS THE POINT, AND IT REPLACES A RULE WITH A MECHANISM.
 * Before this story `entries` and `tail` were `[]string` in Go, and the reason
 * written beside them was that every entry a caller can express is a FACE
 * NAME, so a caller cannot put a second asset entry in a chain by writing one
 * down. `[]string` was the MECHANISM for that; the property is what mattered,
 * and it is unchanged — the object form refuses `asset` on both sides, so the
 * guarantee is still structural rather than something a person must remember.
 *
 * IT IS NOT A SECOND GRAMMAR. A `variants` sidecar keyed by index was refused
 * on D-11.2.2's own ground: it invents a second way to say what the format
 * already says.
 */
export type FontChainEntryAsk = string | Readonly<{ face: string; bold?: string; italic?: string; boldItalic?: string }>

// THE CLOSED VARIANT SET, spelled once in this module and consumed by the one
// encoder below, so the two builders that carry entries cannot disagree about
// it. Widening it is a MAJOR format change the format doc already prices; the
// engine refuses a fourth key at load whatever this list says.
const variantKeys = ['bold', 'italic', 'boldItalic'] as const

// An absent cut is an ABSENT KEY, never an empty string: Go refuses `""` as a
// variant ("an empty string names no face") and would refuse a whole pick for
// a cut the family simply does not have.
const chainEntry = (entry: FontChainEntryAsk): string => {
  if (typeof entry === 'string') return quote(entry)
  const fields: JsonField[] = [['face', quote(entry.face)]]
  for (const key of variantKeys) {
    const cut = entry[key]
    if (cut !== undefined) fields.push([key, quote(cut)])
  }
  return jsonObject(fields)
}

export function addFontChainCommand(name: string, entries: ReadonlyArray<FontChainEntryAsk>): ArrayBuffer {
  return commandBytes('addFontChain', [['name', quote(name)], ['entries', jsonArray(entries.map(chainEntry))]])
}

export function renameFontChainCommand(name: string, to: string): ArrayBuffer {
  return commandBytes('renameFontChain', [['name', quote(name)], ['to', quote(to)]])
}

export function deleteFontChainCommand(name: string): ArrayBuffer {
  return commandBytes('deleteFontChain', [['name', quote(name)]])
}

export function addFontChainEntryCommand(name: string, at: number, face: string): ArrayBuffer {
  return commandBytes('addFontChainEntry', [['name', quote(name)], ['index', index(at)], ['face', quote(face)]])
}

export function moveFontChainEntryCommand(name: string, from: number, to: number): ArrayBuffer {
  return commandBytes('moveFontChainEntry', [['name', quote(name)], ['from', index(from)], ['to', index(to)]])
}

export function removeFontChainEntryCommand(name: string, at: number): ArrayBuffer {
  return commandBytes('removeFontChainEntry', [['name', quote(name)], ['index', index(at)]])
}

// STORY 8.6 — THE PICK.
//
// The SEVENTH builder, and the first that carries BYTES. It is still opaque
// engine intent and the rule is unchanged: this module knows nothing about
// what a duplicate face is, what the content hash of these bytes is, whether
// the document already carries them, or whether the licence text is adequate.
// Go hashes, dedupes, decodes, bounds and refuses; the browser reads the face
// out of the precached content-addressed URL and puts the ask on the wire.
//
// `licenceText` and `copyright` are on the wire because the ENGINE REFUSES TO
// LOAD A DOCUMENT THAT EMBEDS A FACE WITHOUT THEM. They are not decoration the
// designer chose to send — a pick that omitted them would produce a document
// the engine's own parser rejects, so the command carries them and Go refuses
// the pick if any is empty.
//
// The bytes are base64-encoded HERE rather than sent as a binary payload for
// the same reason setComponentAssetCommand does it: the command is one opaque
// JSON document on one protocol, and a second framing for one command kind
// would be a second protocol.
export function embedFontFamilyCommand(face: {
  chain: string
  family: string
  style: string
  licence: string
  licenceText: string
  copyright: string
  source: string
  mediaType: string
  bytes: ArrayBuffer
  tail: ReadonlyArray<FontChainEntryAsk>
}): ArrayBuffer {
  return commandBytes('embedFontFamily', [
    ['name', quote(face.chain)],
    ['family', quote(face.family)], ['style', quote(face.style)],
    ['licence', quote(face.licence)], ['licenceText', quote(face.licenceText)],
    ['copyright', quote(face.copyright)], ['source', quote(face.source)],
    ['mediaType', quote(face.mediaType)], ['data', quote(base64(face.bytes))],
    ['tail', jsonArray(face.tail.map(chainEntry))],
  ])
}

// base64 over an ArrayBuffer, in chunks. `String.fromCharCode(...bytes)` on a
// 480 KB face spreads half a million arguments across the call stack and
// throws; the chunk size is small enough that it cannot, and large enough that
// the loop is not the cost.
const base64 = (bytes: ArrayBuffer): string => {
  const view = new Uint8Array(bytes)
  const chunk = 0x8000
  let binary = ''
  for (let at = 0; at < view.length; at += chunk) binary += String.fromCharCode(...view.subarray(at, at + chunk))
  return btoa(binary)
}
