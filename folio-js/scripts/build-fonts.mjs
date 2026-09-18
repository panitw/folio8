// Copies the eleven fonts.Shipped() faces, and each face's licence and notice,
// from folio8-go/fonts/ into folio-js/fonts/, and writes fonts/manifest.json
// for src/fonts.ts to read.
//
// The bytes are NOT committed to folio-js: folio8-go/fonts/ is the one source
// of truth (the engine embeds those same files), and a second tracked 14 MB
// copy could diverge silently. Every face is checked against
// test/data/go-parity.json's shippedFaces — the record
// folio8-go/wasm/cmd/render/parity_test.go writes from the engine — by size,
// and against the Go source file itself by sha256, so a divergence is a build
// failure instead of a rendering difference.
//
// Nothing half-built survives: the output directory is removed on ANY failure,
// so a later step can never mistake a partial copy for a complete one.
import { createHash } from 'node:crypto'
import { mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { faceLicenceFiles, shippedFaces } from './faces.mjs'

const packageRoot = join(dirname(fileURLToPath(import.meta.url)), '..')
const goFontsRoot = join(packageRoot, '..', 'folio8-go', 'fonts')
const outputDir = join(packageRoot, 'fonts')
const parityPath = join(packageRoot, 'test', 'data', 'go-parity.json')

const digest = (bytes) => createHash('sha256').update(bytes).digest('hex')

const problems = []

function readSource(path) {
  try {
    return readFileSync(path)
  } catch {
    problems.push(`missing from folio8-go: ${path}`)
    return undefined
  }
}

rmSync(outputDir, { recursive: true, force: true })
try {
  build()
} finally {
  if (problems.length > 0) rmSync(outputDir, { recursive: true, force: true })
}
if (problems.length > 0) {
  throw new Error(`folio-js: the shipped font set has drifted from folio8-go:\n  ${problems.join('\n  ')}`)
}

function build() {
  let parity
  try {
    parity = JSON.parse(readFileSync(parityPath, 'utf8')).shippedFaces
  } catch (error) {
    problems.push(`${parityPath} is not a readable fonts.Shipped() record: ${error instanceof Error ? error.message : String(error)}`)
    return
  }
  const expected = new Map(parity.map((face) => [face.name, face.byteLength]))
  if (expected.size !== shippedFaces.length) {
    problems.push(`fonts.Shipped() records ${expected.size} faces, this build copies ${shippedFaces.length}`)
  }

  mkdirSync(outputDir, { recursive: true })

  const manifest = []
  for (const face of shippedFaces) {
    const source = readSource(join(goFontsRoot, face.dir, face.file))
    const licences = faceLicenceFiles.map((licence) => [licence, readSource(join(goFontsRoot, face.dir, licence))])
    if (!source || licences.some(([, bytes]) => !bytes)) continue

    const want = expected.get(face.name)
    if (want === undefined) problems.push(`${face.name}: not in go-parity.json's shippedFaces — fonts.Shipped() does not ship it`)
    else if (want !== source.length) problems.push(`${face.name}: ${face.dir}/${face.file} is ${source.length} bytes, fonts.Shipped() records ${want}`)

    mkdirSync(join(outputDir, face.dir), { recursive: true })
    writeFileSync(join(outputDir, face.dir, face.file), source)
    for (const [licence, bytes] of licences) writeFileSync(join(outputDir, face.dir, licence), bytes)

    // The copy is read back and compared byte for byte. A same-size change to
    // a face — the case a size check cannot see — fails the BUILD here rather
    // than waiting for prepack.
    const copied = readFileSync(join(outputDir, face.dir, face.file))
    if (digest(copied) !== digest(source)) {
      problems.push(`${face.name}: the copy at fonts/${face.dir}/${face.file} does not match folio8-go/fonts/${face.dir}/${face.file}`)
      continue
    }
    manifest.push({ name: face.name, file: `${face.dir}/${face.file}`, byteLength: source.length })
  }

  if (problems.length > 0) return
  writeFileSync(join(outputDir, 'manifest.json'), `${JSON.stringify({ faces: manifest }, null, 2)}\n`)
}
