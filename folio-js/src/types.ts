/** A diagnostic's disposition: Go's `SeverityWarning` or `SeverityError`. */
export type Severity = 'warning' | 'error'

/** One engine diagnostic, field for field Go's `folio8.Diagnostic`. */
export interface Diagnostic {
  severity: Severity
  /** The stable registry code callers dispatch on. */
  code: string
  elementId: string
  dataPath: string
  /** Human-readable prose, identical to Go's; never parse it. */
  message: string
}

/** Report data as JSON: raw bytes, a JSON string, or a value sent as `JSON.stringify`. */
export type Data = Uint8Array | string | object

/** Runtime params as JSON, in the same forms as {@link Data}. */
export type Params = Uint8Array | string | object

/** Face name to font file bytes, Go's `folio8.FontSet`.
 *
 * It may be EMPTY. There is no default font set and no lookup on the machine
 * the render runs on, but a document that carries every face it names has
 * nothing for a font set to contribute, so an empty map is a legitimate call
 * rather than a caller error. */
export type FontSet = Map<string, Uint8Array>

/**
 * What a render does with a chain entry naming a face the renderer was never
 * given — Go's `folio8.FaceFallback`.
 *
 * - `'strict'` refuses with `TEXT_FACE_ABSENT`. It is the default, so a call
 *   that omits the argument behaves exactly as it did before the argument
 *   existed.
 * - `'substitute'` paints the character in a face the renderer WAS given —
 *   the document's own embedded assets first, then the supplied map, each in
 *   face-name order — and reports `TEXT_FACE_SUBSTITUTED`. A renderer holding
 *   nothing that covers the character still refuses with `TEXT_FACE_ABSENT`.
 */
export type FaceFallback = 'strict' | 'substitute'

/** What {@link render} produces: the PDF plus every warning, in Go's order. */
export interface RenderResult {
  bytes: Uint8Array
  diagnostics: Diagnostic[]
}
