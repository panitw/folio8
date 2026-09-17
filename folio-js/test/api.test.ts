import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { PassThrough, Writable } from 'node:stream'
import { describe, expect, it } from 'vitest'
import { FolioRenderError, loadTemplate, parameterReferences, parseTemplate, render, renderTo, validate, version } from '../src/index.js'
import { host } from '../src/engine.js'
import { repoFile, repoRoot, sha256, shippedFonts } from './helpers.js'

const colourTemplate = () => repoFile('fixtures/colour-strokes/input.folio')
const colourData = () => repoFile('fixtures/colour-strokes/data.json')

describe('templates', () => {
  it('loadTemplate reads the one path it is given', async () => {
    const tpl = await loadTemplate(join(repoRoot, 'fixtures', 'colour-strokes', 'input.folio'))
    const direct = await render(await parseTemplate(colourTemplate()), colourData(), undefined, shippedFonts())
    expect(sha256((await render(tpl, colourData(), undefined, shippedFonts())).bytes)).toBe(sha256(direct.bytes))
  })

  it('a malformed template rejects at parse with TEMPLATE_MALFORMED', async () => {
    const error = await parseTemplate('{').catch((reason: unknown) => reason)
    expect(error).toBeInstanceOf(FolioRenderError)
    expect((error as FolioRenderError).diagnostic.code).toBe('TEMPLATE_MALFORMED')
    expect((error as FolioRenderError).diagnostic.severity).toBe('error')
  })

  it('a malformed file rejects from loadTemplate', async () => {
    const dir = mkdtempSync(join(tmpdir(), 'folio-js-'))
    try {
      const path = join(dir, 'bad.folio')
      writeFileSync(path, '{')
      await expect(loadTemplate(path)).rejects.toBeInstanceOf(FolioRenderError)
    } finally {
      rmSync(dir, { recursive: true, force: true })
    }
  })

  it('is opaque: no readable or writable state', async () => {
    const tpl = await parseTemplate(colourTemplate())
    expect(Object.keys(tpl)).toEqual([])
    expect(Object.isFrozen(tpl)).toBe(true)
    const Ctor = tpl.constructor as new (...args: unknown[]) => unknown
    expect(() => new Ctor(Symbol('forged'), new Uint8Array())).toThrow(TypeError)
  })

  it('accepts a string template and object data, the same bytes as raw inputs', async () => {
    const fromBytes = await render(await parseTemplate(colourTemplate()), colourData(), undefined, shippedFonts())
    const text = new TextDecoder().decode(colourTemplate())
    const object = JSON.parse(new TextDecoder().decode(colourData())) as object
    const fromText = await render(await parseTemplate(text), object, undefined, shippedFonts())
    expect(sha256(fromText.bytes)).toBe(sha256(fromBytes.bytes))
  })

  it('parameterReferences returns documentDate', async () => {
    const tpl = await parseTemplate(repoFile('folio-js/test/data/document-date.folio'))
    expect(await parameterReferences(tpl)).toEqual(['documentDate'])
    expect(await parameterReferences(await parseTemplate(colourTemplate()))).toEqual([])
  })
})

describe('renderTo', () => {
  it('writes render’s bytes once and leaves the stream writable', async () => {
    const expected = await render(await parseTemplate(colourTemplate()), colourData(), undefined, shippedFonts())
    const stream = new PassThrough()
    const chunks: Buffer[] = []
    stream.on('data', (chunk: Buffer) => chunks.push(chunk))
    const diagnostics = await renderTo(stream, await parseTemplate(colourTemplate()), colourData(), undefined, shippedFonts())
    expect(diagnostics).toEqual([])
    expect(sha256(Buffer.concat(chunks))).toBe(sha256(expected.bytes))
    expect(stream.writable).toBe(true)
    expect(stream.writableEnded).toBe(false)
    expect(stream.destroyed).toBe(false)
    stream.write('more')
    stream.end()
  })

  it('carries warnings like render', async () => {
    const tpl = await parseTemplate(repoFile('fixtures/wrapped-text/input.folio'))
    const data = repoFile('folio-js/test/data/wrapped-text-named.json')
    const expected = await render(tpl, data, undefined, shippedFonts())
    const stream = new PassThrough()
    stream.resume()
    expect(await renderTo(stream, tpl, data, undefined, shippedFonts())).toEqual(expected.diagnostics)
  })

  it('writes nothing when the render fails, and leaves the stream untouched', async () => {
    const stream = new PassThrough()
    let written = 0
    stream.on('data', (chunk: Buffer) => {
      written += chunk.length
    })
    const tpl = await parseTemplate(repoFile('fixtures/wrapped-text/input.folio'))
    await expect(renderTo(stream, tpl, '{}', undefined, shippedFonts())).rejects.toBeInstanceOf(FolioRenderError)
    expect(written).toBe(0)
    expect(stream.writableLength).toBe(0)
    expect(stream.writable).toBe(true)
    expect(stream.destroyed).toBe(false)
  })

  it('rejects with the stream’s write error', async () => {
    const failing = new Writable({
      write(_chunk, _encoding, callback) {
        callback(new Error('disk full'))
      },
    })
    failing.on('error', () => {})
    await expect(renderTo(failing, await parseTemplate(colourTemplate()), colourData(), undefined, shippedFonts())).rejects.toThrow('disk full')
  })

  it('a failing write with no caller error listener rejects without crashing', async () => {
    const failing = new Writable({
      write(_chunk, _encoding, callback) {
        callback(new Error('disk full'))
      },
    })
    await expect(renderTo(failing, await parseTemplate(colourTemplate()), colourData(), undefined, shippedFonts())).rejects.toThrow('disk full')
    // Let the stream's deferred 'error' emit run; an unhandled one would
    // fail this test run as an uncaught exception.
    await new Promise((r) => setImmediate(r))
    await new Promise((r) => setTimeout(r, 10))
    expect(failing.destroyed).toBe(true)
    expect(failing.listenerCount('error')).toBe(0)
  })

  it('waits for a slow write callback', async () => {
    let release: (() => void) | undefined
    let received = 0
    const slow = new Writable({
      highWaterMark: 1,
      write(chunk: Buffer, _encoding, callback) {
        received += chunk.length
        release = callback
      },
    })
    let resolved = false
    const pending = renderTo(slow, await parseTemplate(colourTemplate()), colourData(), undefined, shippedFonts()).then(() => {
      resolved = true
    })
    while (!release) await new Promise((r) => setTimeout(r, 5))
    await new Promise((r) => setTimeout(r, 20))
    expect(resolved).toBe(false)
    release()
    await pending
    expect(resolved).toBe(true)
    expect(received).toBeGreaterThan(0)
  })
})

describe('validate', () => {
  it('accepts string template bytes and resolves [] for a clean template', async () => {
    expect(await validate(new TextDecoder().decode(colourTemplate()), colourData(), undefined, shippedFonts())).toEqual([])
  })
})

describe('params', () => {
  it('null is Go nil, like undefined', async () => {
    const tpl = await parseTemplate(colourTemplate())
    const withUndefined = await render(tpl, colourData(), undefined, shippedFonts())
    const withNull = await render(tpl, colourData(), null, shippedFonts())
    expect(sha256(withNull.bytes)).toBe(sha256(withUndefined.bytes))
    expect(await validate(colourTemplate(), colourData(), null, shippedFonts())).toEqual([])
  })
})

describe('engine', () => {
  it('survives a panic inside a host call', async () => {
    const engine = await host()
    const reply = engine.parameterReferences(42 as unknown as Uint8Array)
    const envelope = JSON.parse(reply.envelope) as { ok: boolean; error?: { message?: string } }
    expect(envelope.ok).toBe(false)
    expect(envelope.error?.message).toMatch(/panic/)
    expect(await parameterReferences(await parseTemplate(colourTemplate()))).toEqual([])
  })
})

describe('version', () => {
  it('is the engine’s folio8.Version', async () => {
    expect(version).toBe((await host()).version)
  })
})
