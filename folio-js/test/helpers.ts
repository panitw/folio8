import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { shipped } from '../src/fonts.js'
import type { Diagnostic, FontSet } from '../src/types.js'

export const repoRoot = join(dirname(fileURLToPath(import.meta.url)), '..', '..')

/** One renderable fixture, as folio8-go recorded it. Carries no hash by design. */
export interface CorpusFixture {
  slug: string
  /** Whether the render passes the fixture's own data.json (`{}` when false). */
  data: boolean
  /** Whether the render passes the fixture's own params.json. */
  params: boolean
  /** Go's diagnostic sequence for this render, in order. */
  diagnostics: Diagnostic[]
}

export interface CorpusManifest {
  folio8Version: string
  /** The manifest's own record of how many fixtures it carries. */
  count: number
  fixtures: CorpusFixture[]
  excluded: { slug: string; reason: string }[]
}

/**
 * The corpus conformance manifest, derived from Go by
 * folio8-go/wasm/cmd/render/corpus_test.go and held equal to it there. Both
 * bindings drive their byte-identity suites from this one file, so neither can
 * quietly narrow its fixture list.
 *
 * It deliberately carries NO golden digest: `expected.json` stays the single
 * source for every hash, which is what {@link expectedSha256} reads.
 */
export const corpus = JSON.parse(readFileSync(join(repoRoot, 'folio-js', 'test', 'data', 'go-corpus.json'), 'utf8')) as CorpusManifest

/** The committed hash for a fixture, read from its own `expected.json`. */
export function expectedSha256(slug: string): string {
  const expected = JSON.parse(readFileSync(join(repoRoot, 'fixtures', slug, 'expected.json'), 'utf8')) as { sha256: string }
  return expected.sha256
}

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
