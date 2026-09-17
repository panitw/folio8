/**
 * folio-js: the folio8 engine for Node. Every call runs synchronously inside
 * WebAssembly and blocks the event loop for its duration; offloading large
 * renders to a worker thread is the caller's option for now.
 * @module
 */
import { readFile } from 'node:fs/promises'
import type { Writable } from 'node:stream'
import { host, unwrap } from './engine.js'
import { newTemplate, templateBytes, type Template } from './template.js'
import type { Data, Diagnostic, FontSet, Params, RenderResult } from './types.js'

export { FolioRenderError } from './errors.js'
export type { Template } from './template.js'
export type { Data, Diagnostic, FontSet, Params, RenderResult, Severity } from './types.js'
export { version } from './version.js'

const encoder = new TextEncoder()

function templateInput(bytes: unknown, name: string): Uint8Array {
  if (bytes instanceof Uint8Array) return bytes
  if (typeof bytes === 'string') return encoder.encode(bytes)
  throw new TypeError(`${name} must be a Uint8Array or a string`)
}

function jsonInput(value: unknown, name: string): Uint8Array {
  if (value instanceof Uint8Array) return value
  if (typeof value === 'string') return encoder.encode(value)
  if (value instanceof ArrayBuffer || value instanceof SharedArrayBuffer || ArrayBuffer.isView(value) || value instanceof Map || value instanceof Set) {
    throw new TypeError(`${name} must be a Uint8Array, a string or a JSON-serialisable object`)
  }
  if (typeof value === 'object' && value !== null) {
    const json: unknown = JSON.stringify(value)
    if (typeof json !== 'string') throw new TypeError(`${name} could not be serialised as JSON`)
    return encoder.encode(json)
  }
  throw new TypeError(`${name} must be a Uint8Array, a string or an object`)
}

function paramsInput(value: unknown): Uint8Array | null {
  return value === undefined || value === null ? null : jsonInput(value, 'params')
}

function fontsInput(fonts: unknown): [string[], Uint8Array[]] {
  if (!(fonts instanceof Map)) throw new TypeError('fonts must be a Map<string, Uint8Array>')
  const names: string[] = []
  const faces: Uint8Array[] = []
  for (const [name, face] of fonts as Map<unknown, unknown>) {
    if (typeof name !== 'string' || !(face instanceof Uint8Array)) throw new TypeError('fonts must be a Map<string, Uint8Array>')
    names.push(name)
    faces.push(face)
  }
  return [names, faces]
}

function templateInputOf(tpl: unknown): Uint8Array {
  const bytes = templateBytes(tpl)
  if (!bytes) throw new TypeError('tpl must be a Template from parseTemplate or loadTemplate')
  return bytes
}

function writableInput(writable: unknown): Writable {
  const candidate = writable as Partial<Writable> | null
  if (typeof candidate !== 'object' || candidate === null || typeof candidate.write !== 'function' || typeof candidate.once !== 'function' || typeof candidate.removeListener !== 'function') {
    throw new TypeError('writable must be a Node Writable stream')
  }
  return candidate as Writable
}

/** Parses `.folio` bytes, like Go's `ParseTemplate`. A malformed template rejects with `FolioRenderError`. */
export async function parseTemplate(bytes: Uint8Array | string): Promise<Template> {
  const input = templateInput(bytes, 'bytes')
  const reply = unwrap((await host()).parse(input))
  return newTemplate(reply.bytes ?? new Uint8Array())
}

/** Reads and parses the `.folio` file at `path`, like Go's `LoadTemplate`. */
export async function loadTemplate(path: string): Promise<Template> {
  if (typeof path !== 'string') throw new TypeError('path must be a string')
  return parseTemplate(await readFile(path))
}

/**
 * Renders a PDF, like Go's `Render`. A Go error rejects; warnings arrive in
 * `diagnostics`. The render runs synchronously inside WebAssembly and blocks
 * the event loop while it runs; use a worker thread to offload it.
 */
export async function render(tpl: Template, data: Data, params: Params | null | undefined, fonts: FontSet): Promise<RenderResult> {
  const t = templateInputOf(tpl)
  const d = jsonInput(data, 'data')
  const p = paramsInput(params)
  const [names, faces] = fontsInput(fonts)
  const reply = unwrap((await host()).render(t, d, p, names, faces))
  return { bytes: reply.bytes ?? new Uint8Array(), diagnostics: reply.diagnostics ?? [] }
}

/**
 * Renders completely, then writes the PDF to `writable` in one write, like
 * Go's `RenderTo`. Resolves to the warnings once the write completes. The
 * stream is never ended or destroyed, and nothing is written on failure.
 */
export async function renderTo(writable: Writable, tpl: Template, data: Data, params: Params | null | undefined, fonts: FontSet): Promise<Diagnostic[]> {
  const stream = writableInput(writable)
  const { bytes, diagnostics } = await render(tpl, data, params, fonts)
  await new Promise<void>((resolve, reject) => {
    let settled = false
    const settle = (error?: unknown) => {
      if (settled) return
      settled = true
      if (error) reject(error instanceof Error ? error : new Error(String(error)))
      else resolve()
    }
    const onError = (error: unknown) => settle(error)
    stream.once('error', onError)
    stream.write(bytes, (error) => {
      // On success the listener goes; on failure it stays until the stream
      // emits the error (a once listener removes itself), so a caller with
      // no 'error' listener of their own does not crash the process.
      if (!error) stream.removeListener('error', onError)
      settle(error ?? undefined)
    })
  })
  return diagnostics
}

/**
 * Validates template bytes against data and params, like Go's `Validate`:
 * resolves to Go's diagnostic slice verbatim and rejects only when Go
 * returns an error.
 */
export async function validate(bytes: Uint8Array | string, data: Data, params: Params | null | undefined, fonts: FontSet): Promise<Diagnostic[]> {
  const t = templateInput(bytes, 'bytes')
  const d = jsonInput(data, 'data')
  const p = paramsInput(params)
  const [names, faces] = fontsInput(fonts)
  return unwrap((await host()).validate(t, d, p, names, faces)).diagnostics ?? []
}

/** The param names a template reads, like Go's `ParameterReferences`. */
export async function parameterReferences(tpl: Template): Promise<string[]> {
  const t = templateInputOf(tpl)
  return unwrap((await host()).parameterReferences(t)).references ?? []
}
