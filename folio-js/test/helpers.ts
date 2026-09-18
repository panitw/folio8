import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { shipped } from '../src/fonts.js'
import type { FontSet } from '../src/types.js'

export const repoRoot = join(dirname(fileURLToPath(import.meta.url)), '..', '..')

let fonts: FontSet | undefined

/**
 * Loads the package's own shipped set, so the synchronous accessor below can
 * hand it to the many render call sites that are not in async position.
 * test/setup.ts runs this before each test FILE's suites, in that file's own
 * module registry — so once per file, not once per run.
 */
export async function loadShippedFonts(): Promise<FontSet> {
  fonts = await shipped()
  return fonts
}

/**
 * The shipped set — exactly what an installer gets from `folio-js/fonts`,
 * read from the packaged files rather than rebuilt from the Go tree, so the
 * suite exercises the same bytes and the same lookup installers do.
 */
export function shippedFonts(): FontSet {
  if (!fonts) throw new Error('test/helpers: loadShippedFonts() has not run — is test/setup.ts still in vitest.config.ts?')
  return fonts
}

export function repoFile(path: string): Uint8Array {
  return new Uint8Array(readFileSync(join(repoRoot, path)))
}

export function sha256(bytes: Uint8Array): string {
  return createHash('sha256').update(bytes).digest('hex')
}
