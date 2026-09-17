import { PassThrough } from 'node:stream'
import { beforeAll, describe, expect, it, vi } from 'vitest'
import { repoFile, shippedFonts } from './helpers.js'

// Wrap the engine accessor so every call through it is counted: an argument
// error must reject with TypeError before the engine is reached.
const calls = vi.hoisted(() => ({ count: 0 }))
vi.mock('../src/engine.js', async (importOriginal) => {
  const original = await importOriginal<typeof import('../src/engine.js')>()
  return {
    ...original,
    host: () => {
      calls.count++
      return original.host()
    },
  }
})

const api = await import('../src/index.js')

describe('bad arguments throw TypeError without reaching the engine', () => {
  let tpl: Awaited<ReturnType<typeof api.parseTemplate>>
  beforeAll(async () => {
    tpl = await api.parseTemplate(repoFile('fixtures/colour-strokes/input.folio'))
  })

  const cases: [string, () => Promise<unknown>][] = [
    ['render without fonts', () => (api.render as (...a: unknown[]) => Promise<unknown>)(tpl, '{}', undefined)],
    ['render with fonts as an object', () => (api.render as (...a: unknown[]) => Promise<unknown>)(tpl, '{}', undefined, { Roboto: new Uint8Array() })],
    ['render with a non-bytes face', () => (api.render as (...a: unknown[]) => Promise<unknown>)(tpl, '{}', undefined, new Map([['Roboto', 'x']]))],
    ['render with data a number', () => (api.render as (...a: unknown[]) => Promise<unknown>)(tpl, 42, undefined, shippedFonts())],
    ['render with data null', () => (api.render as (...a: unknown[]) => Promise<unknown>)(tpl, null, undefined, shippedFonts())],
    ['render with data an ArrayBuffer', () => (api.render as (...a: unknown[]) => Promise<unknown>)(tpl, new ArrayBuffer(2), undefined, shippedFonts())],
    ['render with data a DataView', () => (api.render as (...a: unknown[]) => Promise<unknown>)(tpl, new DataView(new ArrayBuffer(2)), undefined, shippedFonts())],
    ['render with data an Int8Array', () => (api.render as (...a: unknown[]) => Promise<unknown>)(tpl, new Int8Array(2), undefined, shippedFonts())],
    ['render with data a Map', () => (api.render as (...a: unknown[]) => Promise<unknown>)(tpl, new Map([['a', 1]]), undefined, shippedFonts())],
    ['render with params a Set', () => (api.render as (...a: unknown[]) => Promise<unknown>)(tpl, '{}', new Set([1]), shippedFonts())],
    ['render with params a number', () => (api.render as (...a: unknown[]) => Promise<unknown>)(tpl, '{}', 7, shippedFonts())],
    ['render with a forged template', () => (api.render as (...a: unknown[]) => Promise<unknown>)({}, '{}', undefined, shippedFonts())],
    ['renderTo without a stream', () => (api.renderTo as (...a: unknown[]) => Promise<unknown>)(undefined, tpl, '{}', undefined, shippedFonts())],
    ['renderTo without fonts', () => (api.renderTo as (...a: unknown[]) => Promise<unknown>)(new PassThrough(), tpl, '{}', undefined)],
    ['validate without fonts', () => (api.validate as (...a: unknown[]) => Promise<unknown>)('{}', '{}', undefined)],
    ['validate with data a number', () => (api.validate as (...a: unknown[]) => Promise<unknown>)('{}', 1, undefined, shippedFonts())],
    ['validate with bytes a number', () => (api.validate as (...a: unknown[]) => Promise<unknown>)(1, '{}', undefined, shippedFonts())],
    ['parseTemplate with a number', () => (api.parseTemplate as (...a: unknown[]) => Promise<unknown>)(1)],
    ['loadTemplate with a number', () => (api.loadTemplate as (...a: unknown[]) => Promise<unknown>)(1)],
    ['parameterReferences with a forged template', () => (api.parameterReferences as (...a: unknown[]) => Promise<unknown>)({})],
  ]

  for (const [name, call] of cases) {
    it(name, async () => {
      const before = calls.count
      await expect(call()).rejects.toBeInstanceOf(TypeError)
      expect(calls.count).toBe(before)
    })
  }

  it('the counter sees a real call', async () => {
    const before = calls.count
    await api.parameterReferences(tpl)
    expect(calls.count).toBe(before + 1)
  })
})
