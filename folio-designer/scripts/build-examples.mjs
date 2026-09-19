// THE BUNDLED EXAMPLE TEMPLATES (startup templates, CAP-4).
//
// Each example is `<id>.folio` beside `<id>.sample.json` in
// `public/templates/examples/`. The sample stays a separate file: a `.folio`
// never embeds sample data or a path to it.
//
// For every id in `exampleIds` this stage:
//   1. renders the template against its sample with the engine's own CLI in
//      strict mode, and FAILS THE BUILD on a non-zero exit or on any diagnostic
//      line at all — a warning included — naming the example and the CLI's
//      stderr;
//   2. rasterizes page 1 of that exact PDF to a PNG 264 px wide, so the
//      thumbnail cannot drift from what the example opens as (it is never
//      hand-drawn and never committed);
//   3. fingerprints the template, the sample and the thumbnail into the
//      immutable runtime tree, and emits `src/generated/example-assets.ts`.
//
// A MODULE OF ITS OWN, NOT KEYS IN `offline-assets.ts`: the engine worker
// imports that module, and the examples belong to the application only — the
// same reason `documentation-assets.ts` is separate.
//
// THREE CACHE SLOTS PER EXAMPLE. `maximumCacheAssets` in src/release-payload.ts
// was raised with headroom for four examples (twelve slots).
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import { execFileSync, spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

/** The bundled examples, in dialog order. */
export const exampleIds = ['invoice', 'bank-statement', 'legal-contract', 'electricity-bill']

/** Thumbnail width in CSS pixels. */
export const thumbnailWidth = 264

const scriptDesignerRoot = join(dirname(fileURLToPath(import.meta.url)), '..')

/** Builds `folio-go/cmd/folio8` once into `workDir` and returns the binary's path. */
export function buildExampleCli(workDir, { folio8GoDir = join(scriptDesignerRoot, '..', 'folio-go') } = {}) {
  const binary = join(workDir, process.platform === 'win32' ? 'folio8.exe' : 'folio8')
  execFileSync('go', ['build', '-buildvcs=false', '-o', binary, './cmd/folio8'], { cwd: folio8GoDir, stdio: ['ignore', 'pipe', 'inherit'] })
  return binary
}

/**
 * Renders one example strictly to `<workDir>/<id>.pdf`. Throws naming the
 * example on a missing file, a non-zero exit, or any stderr output (the CLI
 * prints one line per diagnostic there, and prints nothing on a clean render).
 */
export function renderExample({ id, sourceDir, cli, workDir }) {
  const template = join(sourceDir, `${id}.folio`)
  const sample = join(sourceDir, `${id}.sample.json`)
  for (const required of [template, sample]) {
    if (!existsSync(required)) throw new Error(`example '${id}': missing ${required}; every bundled example is <id>.folio beside <id>.sample.json`)
  }
  const pdf = join(workDir, `${id}.pdf`)
  // SOURCE_DATE_EPOCH is dropped so the CLI renders exactly what the designer
  // renders, with no documentDate injected from the environment.
  const env = { ...process.env }
  delete env.SOURCE_DATE_EPOCH
  const result = spawnSync(cli, ['render', '-data', sample, '-o', pdf, '-strict', template], { encoding: 'utf8', env })
  if (result.error) throw new Error(`example '${id}': could not run the folio8 CLI: ${result.error.message}`)
  const stderr = (result.stderr ?? '').trim()
  if (result.status !== 0 || stderr !== '') {
    throw new Error(`example '${id}' does not render cleanly against its sample data (folio8 render -strict exited ${result.status}); every bundled example must render with zero diagnostics:\n${stderr || '(no stderr)'}`)
  }
  if (!existsSync(pdf)) throw new Error(`example '${id}': the folio8 CLI exited 0 and wrote no PDF`)
  return { template, sample, pdf }
}

/** Rasterizes page 1 of `pdfPath` to a PNG exactly `width` px wide at `pngPath`. */
export async function rasterizeFirstPage(pdfPath, pngPath, width = thumbnailWidth) {
  const pdfjs = await import('pdfjs-dist/legacy/build/pdf.mjs')
  const { createCanvas } = await import('@napi-rs/canvas')
  const loadingTask = pdfjs.getDocument({ data: new Uint8Array(readFileSync(pdfPath)), isEvalSupported: false, verbosity: 0 })
  try {
    const document = await loadingTask.promise
    const page = await document.getPage(1)
    const natural = page.getViewport({ scale: 1 })
    const viewport = page.getViewport({ scale: width / natural.width })
    const canvas = createCanvas(width, Math.round(viewport.height))
    const context = canvas.getContext('2d')
    context.fillStyle = '#ffffff'
    context.fillRect(0, 0, canvas.width, canvas.height)
    await page.render({ canvas, canvasContext: context, viewport }).promise
    writeFileSync(pngPath, await canvas.encode('png'))
  } finally {
    await loadingTask.destroy()
  }
}

/**
 * The whole stage. `fingerprint(source, label)` is build-wasm.mjs's
 * content-addressing copy into the runtime tree; it returns the runtime
 * filename.
 */
export async function buildExamples({ designerRoot = scriptDesignerRoot, generatedDir, fingerprint, ids = exampleIds, sourceDir = join(designerRoot, 'public', 'templates', 'examples'), cli }) {
  const workDir = mkdtempSync(join(tmpdir(), 'folio8-examples-'))
  try {
    for (const id of ids) {
      // The id becomes a runtime filename stem and a label in the dialog.
      if (!/^[a-z][a-z0-9-]*$/.test(id)) throw new Error(`example id ${JSON.stringify(id)} is not lower-case kebab-case`)
    }
    if (new Set(ids).size !== ids.length) throw new Error(`example ids are not distinct: ${ids.join(', ')}`)
    const binary = cli ?? buildExampleCli(workDir)
    // Every example is rendered AND rasterized before any is fingerprinted, so
    // a broken one fails the build before anything is written.
    const rendered = ids.map((id) => ({ id, ...renderExample({ id, sourceDir, cli: binary, workDir }) }))
    for (const example of rendered) {
      example.png = join(workDir, `${example.id}.png`)
      await rasterizeFirstPage(example.pdf, example.png)
    }
    const emitted = rendered.map(({ id, template, sample, png }) => ({
      id,
      template: fingerprint(template, `${id}.folio`),
      sample: fingerprint(sample, `${id}.sample.json`),
      thumbnail: fingerprint(png, `${id}.thumbnail.png`),
    }))
    writeFileSync(join(generatedDir, 'example-assets.ts'),
      `// GENERATED by scripts/build-examples.mjs from public/templates/examples/. Do not edit.\n`
      + emitted.map((example, index) => `import example${index}Template from './runtime/${example.template}?url'\nimport example${index}Sample from './runtime/${example.sample}?url'\nimport example${index}Thumbnail from './runtime/${example.thumbnail}?url'`).join('\n')
      + `\n\nexport type ExampleAsset = Readonly<{ id: string; template: string; sample: string; thumbnail: string }>\n\n`
      + `export const exampleAssets: ReadonlyArray<ExampleAsset> = [\n`
      + emitted.map((example, index) => `  { id: ${JSON.stringify(example.id)}, template: example${index}Template, sample: example${index}Sample, thumbnail: example${index}Thumbnail },`).join('\n')
      + `\n]\n`)
    return emitted
  } finally {
    rmSync(workDir, { recursive: true, force: true })
  }
}
