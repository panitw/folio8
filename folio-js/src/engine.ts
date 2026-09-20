import { readFile } from 'node:fs/promises'
import { FolioRenderError } from './errors.js'
import type { Diagnostic } from './types.js'

interface HostReply {
  envelope: string
  bytes?: Uint8Array
}

/** The functions folio-go/wasm/cmd/render registers as globalThis.Folio8RenderHost. */
export interface Host {
  parse(template: Uint8Array): HostReply
  // `fallback` is always a number: this binding resolves the default (see
  // fallbackInput in index.ts), so the host's own null/undefined arm is
  // never reached from here and the type does not pretend otherwise.
  render(template: Uint8Array, data: Uint8Array, params: Uint8Array | null, fontNames: string[], fontBytes: Uint8Array[], fallback: number): HostReply
  validate(template: Uint8Array, data: Uint8Array, params: Uint8Array | null, fontNames: string[], fontBytes: Uint8Array[], fallback: number): HostReply
  parameterReferences(template: Uint8Array): HostReply
  version: string
}

interface Envelope {
  ok: boolean
  diagnostics?: Diagnostic[]
  references?: string[]
  error?: { diagnostic?: Diagnostic; message?: string }
}

// Minimal shapes for the wasm globals this module touches; the ES lib this
// package compiles against does not declare WebAssembly.
type WasmInstance = object
type GoConstructor = new () => { importObject: object; run(instance: WasmInstance): Promise<void> }
interface WasmApi {
  instantiate(bytes: Uint8Array, imports: object): Promise<{ instance: WasmInstance }>
}

let booting: Promise<Host> | undefined

/** The one shared engine instance, started on first use. */
export function host(): Promise<Host> {
  if (!booting) {
    const current: Promise<Host> = boot(() => {
      // The Go program exited, so its host is dead: the next call boots a
      // fresh engine instead of reusing it.
      if (booting === current) booting = undefined
    }).catch((error: unknown) => {
      if (booting === current) booting = undefined
      throw error
    })
    booting = current
  }
  return booting
}

async function boot(onExit: () => void): Promise<Host> {
  const runtime = new URL('../wasm/', import.meta.url)
  await import(new URL('wasm_exec.js', runtime).href)
  const Go = (globalThis as { Go?: GoConstructor }).Go
  if (!Go) throw new Error('folio8: wasm_exec.js did not define Go')
  const go = new Go()
  const { instance } = await (globalThis as unknown as { WebAssembly: WasmApi }).WebAssembly.instantiate(await readFile(new URL('folio8-render.wasm', runtime)), go.importObject)
  const global = globalThis as { Folio8RenderHost?: Host }
  delete global.Folio8RenderHost
  // The Go program blocks in select {} and should never exit; its run promise
  // is not awaited, as in the designer's worker, but settling it means the
  // engine is gone.
  go.run(instance).then(onExit, onExit)
  const registered = global.Folio8RenderHost
  if (!registered) throw new Error('folio8: the render engine did not register')
  return registered
}

/** Decodes a host reply, throwing Go's error as FolioRenderError or Error. */
export function unwrap(reply: HostReply): Envelope & { bytes?: Uint8Array } {
  const envelope = JSON.parse(reply.envelope) as Envelope
  if (!envelope.ok) {
    if (envelope.error?.diagnostic) throw new FolioRenderError(envelope.error.diagnostic)
    throw new Error(envelope.error?.message ?? 'folio8: the engine failed without a message')
  }
  return { ...envelope, bytes: reply.bytes }
}
