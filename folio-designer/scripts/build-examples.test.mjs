// @vitest-environment node
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import { existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { buildExampleCli, buildExamples, exampleIds, thumbnailWidth } from './build-examples.mjs'

// The build gate: a bundled example that does not render cleanly must fail
// `build:wasm`, naming the example. Each failure path of the I/O matrix is
// driven through the real stage and the real CLI.

const cleanTemplate = (value) => `${JSON.stringify({
  assets: {},
  bands: {
    content: { elements: [{ id: 'e1', type: 'text', x: 0, y: 0, width: 40, height: 14, value, style: { fontFamily: 'body', fontSize: 10 } }] },
    pageFooter: { elements: [], height: 10 },
    pageHeader: { elements: [], height: 10 },
  },
  fonts: { body: ['Noto Sans'] },
  locale: 'en',
  nextId: 2,
  page: { margin: { bottom: 10, left: 10, right: 10, top: 10 }, orientation: 'portrait', size: 'A4' },
  utcOffset: '+00:00',
  version: '1.0',
})}\n`

describe('build-examples gate', () => {
  let work
  let cli

  beforeAll(() => {
    work = mkdtempSync(join(tmpdir(), 'folio8-build-examples-test-'))
    cli = buildExampleCli(work)
  }, 180_000)

  afterAll(() => {
    if (work) rmSync(work, { recursive: true, force: true })
  })

  const stage = (name, files) => {
    const sourceDir = join(work, name, 'source')
    const generatedDir = join(work, name, 'generated')
    mkdirSync(sourceDir, { recursive: true })
    mkdirSync(generatedDir, { recursive: true })
    for (const [file, content] of Object.entries(files)) writeFileSync(join(sourceDir, file), content)
    const fingerprinted = []
    const copied = {}
    // Copies the bytes aside, because the stage deletes its work dir on return.
    const fingerprint = (source, label) => { fingerprinted.push(label); copied[label] = readFileSync(source); return label }
    return { sourceDir, generatedDir, fingerprinted, fingerprint, copied }
  }

  it('rejects an example whose render raises a warning, naming it and printing the diagnostic', async () => {
    const { sourceDir, generatedDir, fingerprint, fingerprinted } = stage('warning', {
      'clipped.folio': cleanTemplate('{{name}}'),
      'clipped.sample.json': '{"name": "Unbreakablenamefartoowideforafortypointbox"}\n',
    })
    const error = await buildExamples({ ids: ['clipped'], sourceDir, generatedDir, fingerprint, cli }).then(() => undefined, (caught) => caught)
    expect(error).toBeInstanceOf(Error)
    expect(error.message).toMatch(/example 'clipped'/)
    expect(error.message).toMatch(/WARNING TEXT_CLIPPED_WIDTH element=e1/)
    expect(fingerprinted, 'nothing is fingerprinted when an example fails').toEqual([])
    expect(existsSync(`${generatedDir}/example-assets.ts`)).toBe(false)
  })

  it('rejects an example with no sample data, naming the missing path', async () => {
    const { sourceDir, generatedDir, fingerprint } = stage('missing', { 'lonely.folio': cleanTemplate('Hi') })
    await expect(buildExamples({ ids: ['lonely'], sourceDir, generatedDir, fingerprint, cli })).rejects.toThrow(`example 'lonely': missing ${join(sourceDir, 'lonely.sample.json')}`)
  })

  it('rejects an example whose template fails to load, naming it and carrying the CLI stderr', async () => {
    const { sourceDir, generatedDir, fingerprint } = stage('invalid', {
      'broken.folio': '{"version": "1.0", "bands": ',
      'broken.sample.json': '{}\n',
    })
    const error = await buildExamples({ ids: ['broken'], sourceDir, generatedDir, fingerprint, cli }).then(() => undefined, (caught) => caught)
    expect(error).toBeInstanceOf(Error)
    expect(error.message).toMatch(/example 'broken'/)
    expect(error.message).toMatch(/exited 1/)
    expect(error.message, "the CLI's own stderr is carried into the build error").toMatch(/expected a JSON object/)
  })

  // POSITIVE CONTROL: the same stage over a clean example succeeds, so the
  // rejections above are the gate discriminating rather than the stage failing.
  it('accepts a clean example and emits a thumbnail exactly the declared width', async () => {
    const { sourceDir, generatedDir, fingerprint, fingerprinted, copied } = stage('clean', {
      'hello.folio': cleanTemplate('Hi'),
      'hello.sample.json': '{}\n',
    })
    const emitted = await buildExamples({ ids: ['hello'], sourceDir, generatedDir, fingerprint, cli })
    expect(emitted).toEqual([{ id: 'hello', template: 'hello.folio', sample: 'hello.sample.json', thumbnail: 'hello.thumbnail.png' }])
    expect(fingerprinted).toEqual(['hello.folio', 'hello.sample.json', 'hello.thumbnail.png'])
    const png = copied['hello.thumbnail.png']
    expect(png.subarray(1, 4).toString('latin1'), 'PNG signature').toBe('PNG')
    expect(png.readUInt32BE(16), 'IHDR width').toBe(thumbnailWidth)
    const emittedModule = `${generatedDir}/example-assets.ts`
    expect(readFileSync(emittedModule, 'utf8')).toContain(`{ id: "hello", template: example0Template, sample: example0Sample, thumbnail: example0Thumbnail }`)
  }, 60_000)
})

describe('the example sources', () => {
  it('are exactly the listed example ids, each with its sample data', () => {
    const sourceDirectory = join(import.meta.dirname, '..', 'public', 'templates', 'examples')
    const files = readdirSync(sourceDirectory)
    const stems = files.filter((file) => file.endsWith('.folio')).map((file) => file.slice(0, -'.folio'.length)).sort()
    expect(stems, 'a .folio with no id never ships; an id with no .folio fails the build late').toEqual([...exampleIds].sort())
    for (const id of exampleIds) expect(files, `${id} has no sample data`).toContain(`${id}.sample.json`)
  })
})

describe('the emitted runtime examples', () => {
  const runtimeDirectory = join(import.meta.dirname, '..', 'src', 'generated', 'runtime')

  it('carries a template, a sample and a PNG thumbnail 264 px wide for every listed example', async () => {
    const { createCanvas, loadImage } = await import('@napi-rs/canvas')
    const files = readdirSync(runtimeDirectory)
    for (const id of exampleIds) {
      expect(files.filter((file) => new RegExp(`^${id}\\.[a-f0-9]{20}\\.folio$`).test(file)), `${id} template`).toHaveLength(1)
      expect(files.filter((file) => new RegExp(`^${id}\\.sample\\.[a-f0-9]{20}\\.json$`).test(file)), `${id} sample`).toHaveLength(1)
      const thumbnails = files.filter((file) => new RegExp(`^${id}\\.thumbnail\\.[a-f0-9]{20}\\.png$`).test(file))
      expect(thumbnails, `${id} thumbnail`).toHaveLength(1)
      const png = readFileSync(join(runtimeDirectory, thumbnails[0]))
      expect(png.subarray(0, 8).equals(Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a])), 'PNG signature').toBe(true)
      expect(png.readUInt32BE(16), 'IHDR width').toBe(thumbnailWidth)
      expect(png.readUInt32BE(20), 'an A4 portrait page is taller than wide').toBeGreaterThan(thumbnailWidth)
      // A blank white rasterization would pass every check above.
      const image = await loadImage(png)
      const canvas = createCanvas(image.width, image.height)
      const context = canvas.getContext('2d')
      context.drawImage(image, 0, 0)
      const { data } = context.getImageData(0, 0, image.width, image.height)
      let inked = 0
      for (let offset = 0; offset < data.length; offset += 4) if (data[offset] < 230 || data[offset + 1] < 230 || data[offset + 2] < 230) inked++
      expect(inked / (image.width * image.height), `${id} thumbnail is (nearly) blank`).toBeGreaterThan(0.02)
    }
  })
})
