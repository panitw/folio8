import fs from 'node:fs'
import path from 'node:path'
import { createHash } from 'node:crypto'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// THE EXECUTABLE HALF OF `PROVENANCE.md`. The manifest beside this file is a
// claim about two vendored PDF.js source files at one tag; every load-bearing
// row of it is DERIVED here rather than narrated there. Nothing in this file
// restates a hash, a tag or a prohibition — each is read out of the artifact
// that owns it, so a manifest that drifts from the bytes reds instead of
// quietly becoming fiction.

const vendorDir = path.dirname(fileURLToPath(import.meta.url))
// src/vendor/pdfjs → src/vendor → src → the designer root.
const designerRoot = path.dirname(path.dirname(path.dirname(vendorDir)))
const manifestName = 'PROVENANCE.md'
const vendoredSources = ['pdf_thumbnail_view.js', 'renderable_view.js'] as const

// THE WHOLE DIRECTORY, BY NAME. `PDFThumbnailView`'s dependency closure is two
// files, and D-13.6.4 authorised exactly those two — so the failure worth
// guarding is not a changed byte but a THIRD file arriving on a later
// re-vendor, dragging in the viewer application this rail deliberately does not
// have. A new name here fails until someone puts it in this list on purpose.
const expectedFiles = [
  manifestName,
  'pdf_thumbnail_view.d.ts',
  'pdf_thumbnail_view.js',
  'renderable_view.d.ts',
  'renderable_view.js',
  'vendor-pin.test.ts',
]

const manifest = fs.readFileSync(path.join(vendorDir, manifestName), 'utf8')

const digest = (file: string) => createHash('sha256').update(fs.readFileSync(path.join(vendorDir, file))).digest('hex')

// The manifest's file table is `| file | lines | bytes | upstream sha | vendored
// sha |`, and the LAST cell is the hash of the bytes as they sit in this tree —
// which for `pdf_thumbnail_view.js` is deliberately NOT the upstream one, since
// three recorded modifications stand between them.
function recordedDigest(file: string): string {
  const row = manifest.split('\n').find((line) => line.startsWith(`| \`${file}\``))
  expect(row, `${file} has no row in ${manifestName}`).toBeDefined()
  const cells = (row as string).split('|').map((cell) => cell.trim())
  return (cells[cells.length - 2] as string).replaceAll('`', '')
}

function recordedTag(): string {
  const match = manifest.match(/^\| tag \| `([^`]+)` \|$/m)
  expect(match, `${manifestName} records no tag`).not.toBeNull()
  return (match as RegExpMatchArray)[1] as string
}

// THE PROHIBITION LIST IS READ OUT OF THE CONTRACT, NOT COPIED FROM IT.
//
// `canvas-authority-contract.test.ts` cannot be IMPORTED — it calls `describe`
// at module scope, so importing it would re-register its whole suite inside
// this file and run every one of its rows twice under a second name. Its array
// is therefore parsed out of its source: one regex literal per non-comment
// line, which is the shape that file has kept since Story 8.4a. If the shape
// ever changes, the non-vacuity checks below fail rather than silently
// returning a shorter list — a scan that stops seeing something is the exact
// failure AD-17 exists to prevent.
function contractProhibitions(): readonly RegExp[] {
  const contract = fs.readFileSync(path.join(designerRoot, 'src', 'canvas-authority-contract.test.ts'), 'utf8')
  const block = contract.match(/const prohibited = \[\n([\s\S]*?)\n\]\n/)
  expect(block, 'the contract no longer declares its prohibition array in the parsed shape').not.toBeNull()
  return (block as RegExpMatchArray)[1]
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line.length > 0 && !line.startsWith('//'))
    .map((line) => {
      const literal = line.replace(/,$/, '').match(/^\/(.*)\/([a-z]*)$/)
      expect(literal, `unparsed prohibition line: ${line}`).not.toBeNull()
      const [, source, flags] = literal as RegExpMatchArray
      return new RegExp(source as string, flags)
    })
}

describe('the vendored PDF.js pin', () => {
  it('holds exactly the files the manifest accounts for', () => {
    expect(fs.readdirSync(vendorDir).sort()).toEqual(expectedFiles)
  })

  it('matches the SHA-256 each file is recorded under in the manifest', () => {
    for (const file of vendoredSources) expect(digest(file), file).toBe(recordedDigest(file))
    // Non-vacuity: the two recorded hashes are real, distinct 64-hex digests,
    // so a manifest row emptied to `` could not pass the loop above.
    for (const file of vendoredSources) expect(recordedDigest(file)).toMatch(/^[0-9a-f]{64}$/)
    expect(recordedDigest(vendoredSources[0])).not.toBe(recordedDigest(vendoredSources[1]))
  })

  it('records the same version the package depends on', () => {
    const installed = (JSON.parse(fs.readFileSync(path.join(designerRoot, 'package.json'), 'utf8')) as { dependencies: Record<string, string> }).dependencies['pdfjs-dist']
    // The vendored source calls into `pdfjs-dist/build/pdf.mjs` at runtime, so a
    // package bump that leaves the manifest behind is a fork drifting away from
    // the library it calls. `v` is upstream's tag prefix and nothing more.
    expect(recordedTag()).toBe(`v${installed}`)
  })

  it('keeps the Apache-2.0 header on every vendored file', () => {
    for (const file of vendoredSources) {
      const source = fs.readFileSync(path.join(vendorDir, file), 'utf8')
      expect(source, file).toContain('Copyright')
      expect(source, file).toContain('Licensed under the Apache License, Version 2.0')
      expect(source, file).toContain('WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND')
    }
  })

  it('is clean under the authority contract\'s own prohibitions rather than exempt from them', () => {
    // AC3 asked for a narrow AD-17 exclusion on the premise that vendored PDF.js
    // measures its own rendered output. Measured, that premise does not hold for
    // these two files — the 36 hits live in `pdf_thumbnail_viewer.js`, the file
    // D-13.6.4 removed from scope. So no exclusion is written anywhere, and this
    // row is the positive form of the same guarantee: the vendored bytes are
    // CLEAN, not waived. A re-vendor that drags in a measuring file reds here
    // with nothing to inherit a carve-out from.
    const prohibitions = contractProhibitions()
    expect(prohibitions.length).toBeGreaterThanOrEqual(17)
    // Positive control on the extraction itself, spelled by parts so this file
    // does not plant the very identifier it is scanning for into the corpus the
    // contract scans. If the parse ever returned an empty or broken list, this
    // is what notices.
    const planted = `const rect = node.${['getBounding', 'ClientRect'].join('')}()`
    expect(prohibitions.filter((pattern) => pattern.test(planted))).not.toEqual([])
    for (const file of vendoredSources) {
      const source = fs.readFileSync(path.join(vendorDir, file), 'utf8')
      expect(prohibitions.filter((pattern) => pattern.test(source)).map(String), file).toEqual([])
    }
  })

  it('records every deviation from upstream that the source actually carries', () => {
    const source = fs.readFileSync(path.join(vendorDir, 'pdf_thumbnail_view.js'), 'utf8')
    // Three modifications, marked at four places (the third declares two
    // constants above the site that uses them), and each explained in the
    // manifest. The negative rows below are spelled tightly enough to survive
    // the FOLIO8 MODIFICATION comments, which necessarily NAME what they removed.
    expect(source.match(/FOLIO8 MODIFICATION \d of 3/g) ?? []).toHaveLength(4)
    expect(source).toContain('pdfjs-dist/build/pdf.mjs')
    expect(source).not.toMatch(/from ['"]pdfjs-lib['"]/)
    expect(source).not.toContain('import { AppOptions }')
    expect(source).not.toMatch(/\?\?\s*AppOptions\.get|\|\|\s*AppOptions\.get/)
    expect(fs.readFileSync(path.join(vendorDir, 'renderable_view.js'), 'utf8')).not.toContain('FOLIO8 MODIFICATION')
    // The one file that was deliberately NOT taken, named in the manifest so a
    // later reader finds the reason before they go looking for it.
    expect(manifest).toContain('pdf_thumbnail_viewer.js')
  })
})
