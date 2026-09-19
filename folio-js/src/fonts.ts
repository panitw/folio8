/**
 * The shipped font set, packaged with folio-js: the same eleven faces Go's
 * `fonts.Shipped()` returns, byte for byte.
 *
 * Fonts stay an explicit argument — nothing here becomes ambient. Pass the
 * Map to `render`, `renderTo` or `validate`, or build your own `FontSet` and
 * never call this at all.
 *
 * @module
 */
import { readFile } from 'node:fs/promises'
import type { FontSet } from './types.js'

interface Manifest {
  faces: { name: string; file: string; byteLength: number }[]
}

// scripts/build-fonts.mjs writes both the manifest and the faces beside it,
// from folio8-go/fonts/. Resolving through import.meta.url rather than
// process.cwd() is what makes this work from inside node_modules.
const fontsDir = new URL('../fonts/', import.meta.url)

let loading: Promise<readonly (readonly [string, Uint8Array])[]> | undefined

/**
 * The eleven faces `fonts.Shipped()` ships, read from the package's own
 * files: Roboto, Noto Sans (regular, bold, italic, bold italic each), Noto
 * Sans Thai (regular and bold) and Noto Sans SC.
 *
 * The first call reads about 14 MB from disk; every later call resolves
 * without reading again. Each call gets its OWN Map over the same face
 * bytes, so a caller that deletes or replaces an entry cannot change what
 * the next caller sees.
 */
export async function shipped(): Promise<FontSet> {
  loading ??= read().catch((error: unknown) => {
    // A failed read is not cached: the next call tries again.
    loading = undefined
    throw error
  })
  return new Map(await loading)
}

async function read(): Promise<readonly (readonly [string, Uint8Array])[]> {
  const manifestUrl = new URL('manifest.json', fontsDir)
  let manifest: Manifest
  try {
    manifest = JSON.parse(await readFile(manifestUrl, 'utf8')) as Manifest
  } catch (error) {
    throw new Error(`folio8: the packaged font manifest at ${manifestUrl.pathname} could not be read — run npm run build:fonts`, { cause: error })
  }
  if (!Array.isArray(manifest.faces)) throw new Error(`folio8: the packaged font manifest at ${manifestUrl.pathname} declares no faces — run npm run build:fonts`)
  return Promise.all(manifest.faces.map(async (face) => [face.name, new Uint8Array(await readFile(new URL(face.file, fontsDir)))] as const))
}
