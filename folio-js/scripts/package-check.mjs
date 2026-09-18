// The gate on `npm pack`: a publish cannot ship an incomplete tarball.
//
// `prepack` rebuilds the wasm, the fonts and dist/ and then runs this, which
// re-reads what is actually on disk and refuses the pack if anything the
// `files` allowlist promises is missing, or if a packaged face's BYTES differ
// from folio8-go/fonts/. build-fonts.mjs checks sizes as it copies; this
// checks the bytes, after the fact, on the tree that is about to be packed.
import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { faceLicenceFiles, shippedFaces } from './faces.mjs'

const scriptDir = dirname(fileURLToPath(import.meta.url))

/** Files that are not fonts but must be in the tarball for it to work at all. */
export const requiredFiles = ['package.json', 'README.md', 'LICENSE', 'wasm/folio8-render.wasm', 'wasm/wasm_exec.js', 'dist/index.js', 'dist/index.d.ts', 'dist/fonts.js', 'dist/fonts.d.ts', 'fonts/manifest.json']

const digest = (bytes) => createHash('sha256').update(bytes).digest('hex')

/**
 * Everything wrong with the tree at `packageRoot`, as sentences naming what is
 * missing or what drifted. An empty array means the tree is packable.
 */
export function packageProblems({ packageRoot, goFontsRoot = join(packageRoot, '..', 'folio8-go', 'fonts'), parityPath = join(packageRoot, 'test', 'data', 'go-parity.json') }) {
  const problems = []
  const read = (path) => {
    try {
      return readFileSync(join(packageRoot, path))
    } catch {
      problems.push(`missing from the package: ${path}`)
      return undefined
    }
  }

  for (const path of requiredFiles) read(path)

  // A corrupt manifest or parity record must be a named refusal, not a raw
  // SyntaxError out of the middle of a pack.
  const manifestPath = join(packageRoot, 'fonts', 'manifest.json')
  const manifest = parseJson(readSafely(manifestPath), manifestPath, problems) ?? { faces: [] }
  const packaged = new Map((Array.isArray(manifest.faces) ? manifest.faces : []).map((face) => [face.name, face]))

  const parityBytes = readSafely(parityPath)
  if (!parityBytes) problems.push(`missing the fonts.Shipped() record: ${parityPath}`)
  const parity = parseJson(parityBytes, parityPath, problems)?.shippedFaces ?? []

  for (const face of shippedFaces) {
    const entry = packaged.get(face.name)
    if (!entry) {
      problems.push(`missing from the package: the face "${face.name}" (${face.dir}/${face.file})`)
      continue
    }
    const bytes = read(join('fonts', entry.file))
    for (const licence of faceLicenceFiles) read(join('fonts', face.dir, licence))
    if (!bytes) continue

    // The manifest's own byteLength is what src/fonts.ts's consumers see
    // recorded; a face that no longer matches it means the two were written
    // at different times.
    if (entry.byteLength !== bytes.length) {
      problems.push(`the packaged face "${face.name}" (${face.dir}/${face.file}) is ${bytes.length} bytes, fonts/manifest.json records ${entry.byteLength}`)
    }

    const source = readSafely(join(goFontsRoot, face.dir, face.file))
    if (!source) {
      problems.push(`cannot check "${face.name}" against its source: ${join(goFontsRoot, face.dir, face.file)} is unreadable`)
    } else if (digest(bytes) !== digest(source)) {
      problems.push(`the packaged face "${face.name}" (${face.dir}/${face.file}) differs from folio8-go/fonts/ — rebuild with npm run build:fonts`)
    }
  }

  // The package ships exactly what fonts.Shipped() ships: no more, no fewer.
  for (const face of parity) {
    if (!packaged.has(face.name)) problems.push(`fonts.Shipped() ships "${face.name}" and the package does not`)
  }
  for (const name of packaged.keys()) {
    if (!parity.some((face) => face.name === name)) problems.push(`the package ships "${name}" and fonts.Shipped() does not`)
  }

  return problems
}

function readSafely(path) {
  try {
    return readFileSync(path)
  } catch {
    return undefined
  }
}

/** JSON, or undefined and a named problem — never a thrown SyntaxError. */
function parseJson(bytes, path, problems) {
  if (!bytes) return undefined
  try {
    return JSON.parse(bytes.toString('utf8'))
  } catch (error) {
    problems.push(`${path} is not readable JSON: ${error instanceof Error ? error.message : String(error)}`)
    return undefined
  }
}

// Run directly — by prepack with no arguments, and by the package suite with
// --root/--go-fonts/--parity pointed at a deliberately broken tree.
if (process.argv[1]?.endsWith('package-check.mjs')) {
  const flag = (name) => {
    const at = process.argv.indexOf(`--${name}`)
    if (at === -1) return undefined
    const value = process.argv[at + 1]
    // A flag with no value must stop the run: silently falling back to the
    // real tree would report a doctored tree green.
    if (value === undefined || value.startsWith('--')) {
      console.error(`folio-js: --${name} needs a path`)
      process.exit(2)
    }
    return value
  }
  const packageRoot = flag('root') ?? join(scriptDir, '..')
  const problems = packageProblems({ packageRoot, goFontsRoot: flag('go-fonts'), parityPath: flag('parity') })
  if (problems.length > 0) {
    console.error(`folio-js: refusing to pack an incomplete package:\n  ${problems.join('\n  ')}`)
    process.exit(1)
  }
}
