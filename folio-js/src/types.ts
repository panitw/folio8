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

/** Face name to font file bytes, Go's `folio8.FontSet`. */
export type FontSet = Map<string, Uint8Array>

/** What {@link render} produces: the PDF plus every warning, in Go's order. */
export interface RenderResult {
  bytes: Uint8Array
  diagnostics: Diagnostic[]
}
