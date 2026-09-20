import { PassThrough } from 'node:stream'
import { describe, expect, it } from 'vitest'
import { FolioRenderError, parseTemplate, render, renderTo, validate } from '../src/index.js'
import { repoFile, sha256, shippedFonts } from './helpers.js'

// The face-fallback selector, and the empty font set that issue #1 reports —
// through the binding the guide advertises both in.
//
// The template is built here rather than read from fixtures/ because no
// committed fixture names a face nobody supplies, which is the whole condition
// this capability exists for. It is the same document the Go and .NET tests
// use.
const brandChainTemplate = JSON.stringify({
  assets: {},
  bands: {
    content: {
      elements: [
        { id: 'e1', type: 'text', x: 0, y: 0, width: 500, height: 20, value: '{{name}}', style: { fontFamily: 'body', fontSize: 14 } },
      ],
    },
    pageFooter: { elements: [], height: 20 },
    pageHeader: { elements: [], height: 20 },
  },
  fonts: { body: ['Brand Face'] },
  locale: 'en',
  nextId: 2,
  page: { margin: { bottom: 36, left: 36, right: 36, top: 36 }, orientation: 'portrait', size: 'A4' },
  utcOffset: '+00:00',
  version: '1.0',
})

const embeddedTemplate = () => repoFile('fixtures/embedded-font/input.folio')

describe('a document that carries its faces needs no font set', () => {
  // CAP-7 / issue #1, in the binding the guide promises it in. Every other JS
  // case uses a template whose `assets` is `{}`, so nothing else here renders
  // an embedded document from an empty map.
  it('renders from an empty map', async () => {
    const tpl = await parseTemplate(embeddedTemplate())
    const { bytes } = await render(tpl, {}, null, new Map())
    expect(bytes.length).toBeGreaterThan(0)
  })

  it('validates against an empty map', async () => {
    const found = await validate(embeddedTemplate(), {}, null, new Map())
    expect(found.some((d) => d.code === 'TEXT_FACE_ABSENT')).toBe(false)
  })

  // The claim that makes deleting the old refusal safe rather than merely
  // permitted: a set this document never consults cannot change its bytes.
  it('is unmoved by a font set it never consults', async () => {
    const tpl = await parseTemplate(embeddedTemplate())
    const empty = await render(tpl, {}, null, new Map())
    const unrelated = await render(tpl, {}, null, new Map([['A Face This Document Never Names', shippedFonts().get('Roboto')!]]))
    expect(sha256(unrelated.bytes)).toBe(sha256(empty.bytes))
  })
})

describe('the face-fallback selector', () => {
  it('defaults to strict, so an absent face still refuses', async () => {
    const tpl = await parseTemplate(brandChainTemplate)
    const error = await render(tpl, { name: 'Hi' }, null, shippedFonts()).catch((reason: unknown) => reason)
    expect(error).toBeInstanceOf(FolioRenderError)
    expect((error as FolioRenderError).diagnostic.code).toBe('TEXT_FACE_ABSENT')
  })

  it("'substitute' paints a pool face and says so", async () => {
    const tpl = await parseTemplate(brandChainTemplate)
    const { bytes, diagnostics } = await render(tpl, { name: 'Hi' }, null, shippedFonts(), 'substitute')
    expect(bytes.length).toBeGreaterThan(0)
    const substituted = diagnostics.filter((d) => d.code === 'TEXT_FACE_SUBSTITUTED')
    expect(substituted.length).toBeGreaterThan(0)
    for (const d of substituted) {
      expect(d.severity).toBe('warning')
      expect(d.elementId).toBe('e1')
      expect(d.message).toContain('Brand Face')
      expect(d.message).toContain('painted in ')
    }
  })

  it('still refuses when the renderer holds nothing that covers the character', async () => {
    const tpl = await parseTemplate(brandChainTemplate)
    const error = await render(tpl, { name: 'Hi' }, null, new Map(), 'substitute').catch((reason: unknown) => reason)
    expect(error).toBeInstanceOf(FolioRenderError)
    expect((error as FolioRenderError).diagnostic.code).toBe('TEXT_FACE_ABSENT')
  })

  // Dropping `fallback` from the renderTo wrapper leaves every case above
  // green while a lenient caller gets a refusal, so the forwarding is pinned
  // directly.
  it('is forwarded by renderTo', async () => {
    const tpl = await parseTemplate(brandChainTemplate)
    const sink = new PassThrough()
    const chunks: Uint8Array[] = []
    sink.on('data', (chunk: Uint8Array) => chunks.push(chunk))
    const diagnostics = await renderTo(sink, tpl, { name: 'Hi' }, null, shippedFonts(), 'substitute')
    expect(chunks.length).toBeGreaterThan(0)
    expect(diagnostics.some((d) => d.code === 'TEXT_FACE_SUBSTITUTED')).toBe(true)

    await expect(renderTo(new PassThrough(), tpl, { name: 'Hi' }, null, shippedFonts())).rejects.toBeInstanceOf(FolioRenderError)
  })

  it('is forwarded by validate', async () => {
    const found = await validate(brandChainTemplate, { name: 'Hi' }, null, shippedFonts(), 'substitute')
    expect(found.some((d) => d.code === 'TEXT_FACE_SUBSTITUTED')).toBe(true)
    await expect(validate(brandChainTemplate, { name: 'Hi' }, null, shippedFonts())).rejects.toBeInstanceOf(FolioRenderError)
  })

  // Refused by the BINDING, as a TypeError, never clamped — Go returns a named
  // error for the same input and .NET throws ArgumentException.
  it('refuses a value outside the closed set', async () => {
    const tpl = await parseTemplate(brandChainTemplate)
    // @ts-expect-error the point of the test is the value TypeScript forbids
    await expect(render(tpl, { name: 'Hi' }, null, shippedFonts(), 'lenient')).rejects.toBeInstanceOf(TypeError)
    // @ts-expect-error same
    await expect(validate(brandChainTemplate, { name: 'Hi' }, null, shippedFonts(), 7)).rejects.toBeInstanceOf(TypeError)
  })

  it('is deterministic', async () => {
    const tpl = await parseTemplate(brandChainTemplate)
    const first = await render(tpl, { name: 'Hello' }, null, shippedFonts(), 'substitute')
    const again = await render(tpl, { name: 'Hello' }, null, shippedFonts(), 'substitute')
    expect(sha256(again.bytes)).toBe(sha256(first.bytes))
  })
})
