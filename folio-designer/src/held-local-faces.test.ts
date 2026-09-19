import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { catalogueFaces } from './generated/font-catalogue'
import { initialHeldLocalFamilies, readHeldLocalFamilies } from './held-local-faces'

// THE PARTIALLY-HELD STATE, WHICH IS THE ONLY ONE THAT MEASURES ANYTHING
// (spec-deferred-offline-cache, story 2).
//
// jsdom has no Cache API, so every other suite in this repository exercises the
// no-cache branch — under which this module correctly answers "every family" and
// a body that returned every family UNCONDITIONALLY would be indistinguishable.
// These cases drive the injected lookup instead, so the held set is genuinely a
// function of what the cache answers.
const halfTheCatalogue = catalogueFaces.filter((_, index) => index % 2 === 0)
const heldUrls = new Set(halfTheCatalogue.map((face) => face.url))
const release = 'a'.repeat(64)

describe('which catalogue families this browser actually holds', () => {
  it('holds exactly the families whose bytes the cache answers for, and none of the rest', async () => {
    const asked: string[] = []
    const held = await readHeldLocalFamilies(release, async (url) => { asked.push(url); return heldUrls.has(url) ? { ok: true } : undefined })

    // NON-VACUITY FIRST: the two halves must both have members, or "some held,
    // some not" is a sentence about a population of one kind.
    expect(halfTheCatalogue.length).toBeGreaterThan(1)
    expect(catalogueFaces.length - halfTheCatalogue.length).toBeGreaterThan(1)
    expect([...held].sort()).toEqual([...new Set(halfTheCatalogue.map((face) => face.family))].sort())
    for (const face of catalogueFaces) {
      if (heldUrls.has(face.url)) continue
      expect(held.has(face.family), `${face.family} has no bytes in this cache and must not be reported as held`).toBe(false)
    }
    // AND EVERY FACE WAS ASKED ABOUT, by its own content-addressed URL — a probe
    // that asked about a subset would report the rest as absent for the wrong
    // reason and still satisfy the set comparison above.
    expect([...asked].sort()).toEqual(catalogueFaces.map((face) => face.url).sort())
  })

  it('reads a lookup that throws as not held, per face, without failing the whole probe', async () => {
    const held = await readHeldLocalFamilies(release, async (url) => {
      if (!heldUrls.has(url)) throw new Error('storage cannot be read')
      return { ok: true }
    })
    // The storage errors did not take the readable faces down with them, and
    // nothing that threw is claimed as available offline.
    expect([...held].sort()).toEqual([...new Set(halfTheCatalogue.map((face) => face.family))].sort())
  })

  it('answers every family where there is no release to open a cache for, before and after the probe', async () => {
    const every = new Set(catalogueFaces.map((face) => face.family))
    // No releaseId is the dev server and these suites: no service worker, no
    // content-addressed release, and therefore no deferral to be honest about.
    expect([...initialHeldLocalFamilies(undefined)].sort()).toEqual([...every].sort())
    expect([...await readHeldLocalFamilies(undefined)].sort()).toEqual([...every].sort())
  })
})

// THE CACHE NAME HAS TWO SPELLINGS AND THEY MUST BE ONE SENTENCE.
//
// The worker owns the name — `const CACHE_NAME = 'folio8-release-' + RELEASE.id`
// in `scripts/offline-service-worker-template.mjs`, pinned as a required
// substring by `verify-offline-release.mjs`. Since story 2 the PAGE names it
// too, because `caches.match(url, { cacheName })` is how the held-set probe
// asks this release's cache and not every cache on the origin.
//
// A DRIFT HERE IS SILENT AND ONE-DIRECTIONAL, which is exactly why it is worth
// a test rather than a comment: a page looking in a cache that does not exist
// finds nothing, so every catalogue family quietly leaves AVAILABLE LOCALLY and
// no error is raised anywhere. Under-claiming is the safe direction — the
// families stay reachable through `Add fonts…` — and silence is what makes it
// the kind of defect that ships. The prefix is READ from the worker template
// rather than re-typed here, on the idiom `verify-offline-release.test.mjs`
// already uses for the cache-asset bound: a literal in this file would be a
// third authority rather than a tie between the two that exist.
describe('the release cache name the page and the worker must agree on', () => {
  it('probes the cache the worker actually fills', async () => {
    const template = readFileSync(join(import.meta.dirname, '..', 'scripts', 'offline-service-worker-template.mjs'), 'utf8')
    const declaration = template.split('\n').filter((line) => line.startsWith('const CACHE_NAME = '))
    expect(declaration, 'the worker must declare its cache name on exactly one line for this tie to read it').toHaveLength(1)
    const prefix = /^const CACHE_NAME = '([^']+)' \+ RELEASE\.id$/.exec(declaration[0])?.[1]
    expect(prefix, `the worker's cache-name declaration is not the shape this tie reads: ${declaration[0]}`).toBeDefined()

    // THE REAL DEFAULT LOOKUP, not an injected one: `cacheName` is chosen inside
    // that default, so an injected `match` would prove nothing about it.
    const probed: Array<string | undefined> = []
    const stub = { match: (_url: string, options?: { cacheName?: string }) => { probed.push(options?.cacheName); return Promise.resolve(undefined) } }
    const globals = globalThis as { caches?: unknown }
    const had = 'caches' in globals
    const previous = globals.caches
    globals.caches = stub
    try { await readHeldLocalFamilies(release) } finally { if (had) globals.caches = previous; else delete globals.caches }

    expect(probed.length, 'every catalogue face must be asked about, or the held set is a guess').toBe(catalogueFaces.length)
    expect([...new Set(probed)], 'the page must ask the same cache the worker writes, or AVAILABLE LOCALLY empties with nothing said').toEqual([`${prefix}${release}`])
  })
})
