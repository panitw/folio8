import { existsSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { parseTemplate, render, version } from '../src/index.js'
import { corpus, expectedSha256, repoFile, repoRoot, sha256, shippedFonts } from './helpers.js'

// CAP-5, for folio-js: the WHOLE renderable corpus, not a hand-picked subset.
//
// The fixture list comes from folio-js/test/data/go-corpus.json, which
// folio-go/wasm/cmd/render/corpus_test.go derives from Go and holds equal to
// it — every fixtures/ directory is either in that manifest or excluded there
// with a stated reason, so a new fixture cannot go uncovered. The expected
// hash is read from each fixture's own committed expected.json and is never
// restated here; the expected diagnostics come from the manifest.
//
// This suite runs on every OS and Node version in the folio-js CI matrix, so
// a byte that moves on one platform alone reds exactly one leg.

// Counted as the renders ACTUALLY HAPPEN, and checked against the manifest at
// the end. Comparing the manifest's list length to the manifest's own count
// would only prove the manifest agrees with itself; this proves the suite ran
// the whole thing — a leg that renders nothing fails rather than passing empty.
let rendered = 0

describe('the Go corpus renders byte-identically', () => {
  it('was generated from the engine version this package reports', () => {
    expect(corpus.folio8Version).toBe(version)
  })

  it('every excluded fixture states why', () => {
    // Non-empty first: a vacuous pass over an empty list would report the
    // reason check as green while checking nothing.
    expect(corpus.excluded.length).toBeGreaterThan(0)
    for (const excluded of corpus.excluded) {
      expect(excluded.reason.trim().length, `fixtures/${excluded.slug} is excluded with no reason`).toBeGreaterThan(0)
    }
  })

  // Every fixture in the corpus renders clean today. Asserting that here
  // means a regenerated manifest that BAKES IN a new warning reds, instead of
  // both bindings quietly agreeing with it.
  it('every recorded diagnostic sequence is empty', () => {
    for (const fixture of corpus.fixtures) {
      expect(fixture.diagnostics, `fixtures/${fixture.slug}: the manifest records a diagnostic; if that is intended, say so deliberately`).toEqual([])
    }
  })

  for (const fixture of corpus.fixtures) {
    it(fixture.slug, async () => {
      const dir = join('fixtures', fixture.slug)
      // The manifest says which optional files Go used. If the tree disagrees,
      // a file was added or removed without regenerating, and the fixture is
      // named here rather than failing later as an unexplained hash.
      expect(existsSync(join(repoRoot, dir, 'data.json')), `fixtures/${fixture.slug}: data.json presence differs from the manifest — regenerate go-corpus.json`).toBe(fixture.data)
      expect(existsSync(join(repoRoot, dir, 'params.json')), `fixtures/${fixture.slug}: params.json presence differs from the manifest — regenerate go-corpus.json`).toBe(fixture.params)

      const tpl = await parseTemplate(repoFile(join(dir, 'input.folio')))
      const data = fixture.data ? repoFile(join(dir, 'data.json')) : '{}'
      const params = fixture.params ? repoFile(join(dir, 'params.json')) : undefined
      const result = await render(tpl, data, params, shippedFonts())
      rendered++
      // Named on failure: the fixture, what expected.json records, and what
      // this runtime actually produced.
      expect(sha256(result.bytes), `fixtures/${fixture.slug}: SHA-256 differs from its committed expected.json`).toBe(expectedSha256(fixture.slug))
      expect(result.diagnostics, `fixtures/${fixture.slug}: diagnostics differ from what Go recorded in go-corpus.json`).toEqual(fixture.diagnostics)
    })
  }

  // Last, so every case above has run. Vitest runs a file's tests in
  // declaration order.
  it('rendered every fixture the manifest records', () => {
    expect(rendered, 'this leg did not render the whole corpus').toBe(corpus.count)
  })
})
