import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { catalogueFaces } from './generated/font-catalogue'
import { initialLocalFaceHoldings, localFaceIsHeld, readLocalFaceHoldings } from './held-local-faces'

// THE PARTIALLY-HELD STATE, WHICH IS THE ONLY ONE THAT MEASURES ANYTHING
// (spec-deferred-offline-cache, story 2).
//
// jsdom has no Cache API, so every other suite in this repository exercises the
// no-cache branch — under which this module correctly answers "every family" and
// a body that returned every family UNCONDITIONALLY would be indistinguishable.
// These cases drive the injected lookup instead, so the held set is genuinely a
// function of what the cache answers.
// A FAMILY IS HELD ONLY WHEN EVERY CUT IT DECLARES IS CACHED
// (spec-install-all-face-cuts, story 3). The fixture used to be every other
// FACE, which was a fair "some held, some not" while each family had exactly
// one; with cuts it would have split most families down the middle and said
// nothing about the rule that replaced the old one. So the split is by FAMILY,
// and a deliberately PARTIAL family is planted beside the two clean halves.
const catalogueFamilies = [...new Set(catalogueFaces.map((face) => face.family))].sort()
const cutsOf = (family: string) => catalogueFaces.filter((face) => face.family === family)
// The partial case is a family with more than one cut, holding its Regular
// alone — the exact state CAP-4 exists to finish, and the state the old
// any-one-cut rule reported as COMPLETE.
const partialFamily = catalogueFamilies.find((family) => cutsOf(family).length > 1) as string
const wholeFamilies = catalogueFamilies.filter((family, index) => family !== partialFamily && index % 2 === 0)
const heldUrls = new Set([
  ...wholeFamilies.flatMap((family) => cutsOf(family).map((face) => face.url)),
  ...cutsOf(partialFamily).filter((face) => face.style === 'Regular').map((face) => face.url),
])
const release = 'a'.repeat(64)

describe('which catalogue families this browser actually holds', () => {
  it('holds exactly the families whose bytes the cache answers for, and none of the rest', async () => {
    const asked: string[] = []
    const held = (await readLocalFaceHoldings(release, async (url) => { asked.push(url); return heldUrls.has(url) ? { ok: true } : undefined })).complete

    // NON-VACUITY FIRST: the three groups must all have members, or "some held,
    // some partly held, some not" is a sentence about a population of one kind.
    expect(wholeFamilies.length).toBeGreaterThan(1)
    expect(catalogueFamilies.length - wholeFamilies.length - 1).toBeGreaterThan(1)
    expect(cutsOf(partialFamily).length, 'the partial case must be a family with more than one cut, or it is the same case as a whole one').toBeGreaterThan(1)

    expect([...held].sort()).toEqual([...wholeFamilies].sort())
    for (const family of catalogueFamilies) {
      if (wholeFamilies.includes(family)) continue
      expect(held.has(family), `${family} has at least one cut this cache cannot answer for and must not be reported as held`).toBe(false)
    }
    // AND EVERY FACE WAS ASKED ABOUT, by its own content-addressed URL — a probe
    // that asked about a subset would report the rest as absent for the wrong
    // reason and still satisfy the set comparison above.
    expect([...asked].sort()).toEqual(catalogueFaces.map((face) => face.url).sort())
  })

  // THE CASE THE OLD RULE GOT WRONG, ON ITS OWN, so it reds with its own
  // sentence rather than inside the set comparison above. `held.add(family)` on
  // the first hit would report this family as complete, AVAILABLE LOCALLY would
  // promise a bold this browser does not have, and CAP-4 would never offer to
  // finish it.
  it('does not report a family whose Regular alone is cached as held', async () => {
    const held = (await readLocalFaceHoldings(release, async (url) => heldUrls.has(url) ? { ok: true } : undefined)).complete
    const cached = cutsOf(partialFamily).filter((face) => heldUrls.has(face.url))
    expect(cached.map((face) => face.style), 'the fixture must hold this family\'s Regular and nothing else').toEqual(['Regular'])
    expect(held.has(partialFamily), `${partialFamily} holds ${cached.length} of its ${cutsOf(partialFamily).length} cuts and must not read as complete`).toBe(false)
  })

  // THE OTHER HALF OF THE ONE PROBE (spec-install-all-face-cuts story 3,
  // finding F1). `complete` answers "is anything left to fetch"; `usable`
  // answers "can these bytes be used", and they are different answers for the
  // partial family above — which is the whole reason the record has two fields.
  it('reports a family whose Regular alone is cached as usable but not complete', async () => {
    const holdings = await readLocalFaceHoldings(release, async (url) => heldUrls.has(url) ? { ok: true } : undefined)
    expect(holdings.usable.has(partialFamily), 'its Regular is cached, so it can be applied and painted').toBe(true)
    expect(holdings.complete.has(partialFamily), 'cuts are still missing, so there is something left to fetch').toBe(false)
    // THE TWO FIELDS MUST GENUINELY DIFFER OVER THIS FIXTURE, or every
    // assertion here would also pass over a probe that still returned one set
    // under two names.
    expect([...holdings.usable].sort()).not.toEqual([...holdings.complete].sort())
    // A WHOLE FAMILY IS BOTH, and a family with nothing cached is neither — the
    // two ends that stop this passing by answering `true` to everything.
    for (const family of wholeFamilies) {
      expect(holdings.usable.has(family)).toBe(true)
      expect(holdings.complete.has(family)).toBe(true)
    }
    const absent = catalogueFamilies.find((family) => family !== partialFamily && !wholeFamilies.includes(family))!
    expect(holdings.usable.has(absent)).toBe(false)
    expect(holdings.complete.has(absent)).toBe(false)
  })

  // A FAMILY HOLDING A CUT BUT NOT ITS REGULAR IS NOT USABLE, and that is the
  // asymmetry `usable` exists to state: the Regular is what a pick embeds, so a
  // family with only its Bold in the cache can paint nothing the author can ask
  // for. Not reachable through any install path — it is asserted because the
  // rule is a choice rather than a consequence.
  it('does not report a family holding a cut but not its Regular as usable', async () => {
    const boldOnly = new Set(cutsOf(partialFamily).filter((face) => face.style !== 'Regular').slice(0, 1).map((face) => face.url))
    expect(boldOnly.size, 'the fixture must hold exactly one non-Regular cut').toBe(1)
    const holdings = await readLocalFaceHoldings(release, async (url) => boldOnly.has(url) ? { ok: true } : undefined)
    expect(holdings.usable.has(partialFamily)).toBe(false)
    expect(holdings.complete.has(partialFamily)).toBe(false)
  })

  // THE PRESS-TIME PROBE, WHICH ASKS ABOUT ONE URL RATHER THAN ABOUT A FAMILY.
  // `commitPropertiesEmbeddingCuts` uses it to refuse a cut whose bytes the
  // cache has lost between the family list opening and the keypress, rather
  // than letting `fetch` go to the network for them.
  it('answers for a single face url, and answers `held` where there is no cache to ask', async () => {
    const cut = cutsOf(partialFamily).find((face) => face.style === 'Regular')!
    const missing = cutsOf(partialFamily).find((face) => face.style !== 'Regular')!
    expect(await localFaceIsHeld(cut.url, release, async (url) => heldUrls.has(url) ? { ok: true } : undefined)).toBe(true)
    expect(await localFaceIsHeld(missing.url, release, async (url) => heldUrls.has(url) ? { ok: true } : undefined)).toBe(false)
    // A lookup that throws reads as NOT held, which is the safe direction: the
    // press is refused by name rather than fetching over the network.
    expect(await localFaceIsHeld(cut.url, release, async () => { throw new Error('storage cannot be read') })).toBe(false)
    // NO RELEASE TO OPEN A CACHE FOR IS THE DEV SERVER AND THESE SUITES, where
    // nothing is deferred and every face is served directly.
    expect(await localFaceIsHeld(cut.url, undefined)).toBe(true)
  })

  it('reads a lookup that throws as not held, per face, without failing the whole probe', async () => {
    const held = (await readLocalFaceHoldings(release, async (url) => {
      if (!heldUrls.has(url)) throw new Error('storage cannot be read')
      return { ok: true }
    })).complete
    // The storage errors did not take the readable faces down with them, and
    // nothing that threw is claimed as available offline.
    expect([...held].sort()).toEqual([...wholeFamilies].sort())
  })

  it('answers every family where there is no release to open a cache for, before and after the probe', async () => {
    const every = new Set(catalogueFaces.map((face) => face.family))
    // No releaseId is the dev server and these suites: no service worker, no
    // content-addressed release, and therefore no deferral to be honest about.
    // BOTH FIELDS ANSWER "every family" where there is no cache: no cache means
    // no deferral, so nothing is short of anything.
    expect([...initialLocalFaceHoldings(undefined).complete].sort()).toEqual([...every].sort())
    expect([...initialLocalFaceHoldings(undefined).usable].sort()).toEqual([...every].sort())
    const read = await readLocalFaceHoldings(undefined)
    expect([...read.complete].sort()).toEqual([...every].sort())
    expect([...read.usable].sort()).toEqual([...every].sort())
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
    try { await readLocalFaceHoldings(release) } finally { if (had) globals.caches = previous; else delete globals.caches }

    expect(probed.length, 'every catalogue face must be asked about, or the held set is a guess').toBe(catalogueFaces.length)
    expect([...new Set(probed)], 'the page must ask the same cache the worker writes, or AVAILABLE LOCALLY empties with nothing said').toEqual([`${prefix}${release}`])
  })
})
