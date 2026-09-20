import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { catalogueFaces } from './generated/font-catalogue'
import { familyIndex, familyIndexPublishedFamilies, familyIndexSnapshotDate } from './generated/font-index'
import { blankComments } from '../scripts/forbidden-font-hosts.mjs'
import { addableFamilyCount, familyIsComplete, familyIsInstalled, familySourceNote, indexCategories, indexRowFor, indexScripts, indexExcludedCjkFamilies, localTierHolds, offeredFamilies, regularCutOf, sourceScripts, webFamilies } from './font-index'
import type { FamilyCensus, FamilyCutRefusal, StoredFace } from './font-store'

// EVERY CATALOGUE FAMILY HELD — the state this designer is in once its
// catalogue faces have been fetched, and the one in which the offered-order
// claims below are about ORDER rather than about what has been downloaded.
// `familyIsInstalled` takes the held set explicitly since spec-deferred-offline-
// cache story 2, because a catalogue face is deferred and shipping in the
// release no longer proves it is on this machine.
const allHeld: ReadonlySet<string> = new Set(catalogueFaces.map((face) => face.family))
// AND A GENUINELY PARTIAL ONE: every other catalogue family fetched. This is
// the ordinary state of a browser that has used the designer for a while, and
// it is the input under which an ordering claim keyed on installedness stops
// being two runs — see the run-structure test below.
const halfHeld: ReadonlySet<string> = new Set(catalogueFaces.filter((_, index) => index % 2 === 0).map((face) => face.family))

// STORY 16.1 — THE TWO TIERS AND THE JOIN BETWEEN THEM (D-16.R.3, D-16.R.2).

const here = path.dirname(fileURLToPath(import.meta.url))
const manifest: ReadonlyArray<{ family: string; licence: string }> = JSON.parse(fs.readFileSync(path.join(here, '..', 'font-catalogue.json'), 'utf8'))

/**
 * THE STORY 16.1a BATCH, WRITTEN OUT RATHER THAN DERIVED.
 *
 * Derived from the catalogue it would be a tautology — "the families in the
 * catalogue are in the catalogue" — and the property under test is that these
 * TEN SPECIFIC families, the refused head of the popularity distribution, are
 * the ones an author can now reach. The membership rule that produced it is
 * D-16.R.16's, corrected by D-16.R.19: the refused families within the top 20
 * by `popularity` on the committed snapshot, minus CJK, minus the families the
 * tier already held, minus the `shippedFamilies` collisions, minus anything with
 * no obtainable static from its own project upstream (`Google Sans`, which
 * publishes none, and `Jost`, whose static names itself `Jost*`).
 */
const batchFamilies: ReadonlyArray<string> = [
  'Arimo', 'DM Sans', 'Lora', 'Montserrat', 'Open Sans',
  'Oswald', 'Plus Jakarta Sans', 'Roboto Condensed', 'Roboto Mono', 'Roboto Slab',
]

describe('the build-time index snapshot', () => {
  // NON-VACUITY FIRST. Every filter below is over `familyIndex`, and an empty
  // or truncated snapshot satisfies all of them silently.
  it('ships a dated snapshot of a real population, and never fetches the list at runtime', () => {
    expect(familyIndex.length).toBeGreaterThan(1000)
    expect(familyIndexPublishedFamilies).toBeGreaterThanOrEqual(familyIndex.length)
    expect(familyIndexSnapshotDate).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    // THE LIST IS NOT FETCHABLE BY A BROWSER — the endpoint sends no
    // access-control-allow-origin — so no production module may reach for it.
    // `font-source.ts` names the host only because the BUILD script imports the
    // constant from there; nothing calls it.
    const production = fs.readdirSync(here, { recursive: true })
      .filter((entry): entry is string => typeof entry === 'string' && /\.tsx?$/.test(entry) && !/\.test\.tsx?$/.test(entry))
    expect(production.length).toBeGreaterThan(5)
    for (const entry of production) {
      // Comment-blanked, using the host scanner's own blanker: the generated
      // module NAMES the endpoint in its header to say the list came from a
      // build-time snapshot of it, and a check that could not tell that
      // disclosure from a call site would be satisfied by deleting the
      // disclosure.
      const code = blankComments(fs.readFileSync(path.join(here, entry), 'utf8'), '.ts')
      expect(code, `${entry} must not fetch the family index at runtime`).not.toMatch(/metadata\/fonts/)
    }
  })

  // THE NON-GOAL IS HONOURED IN THE SNAPSHOT, not in the browser: CJK stays on
  // the shipped-face path, so those rows never ship at all.
  it('excludes CJK families from the snapshot rather than filtering them in the browser', () => {
    expect(indexExcludedCjkFamilies).toBeGreaterThan(0)
    // A COROLLARY, NOT THE CHECK. `scriptsOf` emits only `latin` and `thai`, so
    // this line is structurally incapable of failing whatever the filter does —
    // the filter itself is exercised by the test below, over a fixture.
    expect(familyIndex.some((row) => row.scripts.includes('cjk'))).toBe(false)
  })

  // THE REAL FILTER, RUN. `trimSnapshot` — not a restatement of it — over a
  // hand-written `familyMetadataList` carrying one family per CJK subset the
  // exclusion list names, plus two that must survive. Asserting the returned
  // families AND the excluded count directly is what makes deleting a subset
  // from `cjkSubsets` red: nothing else in this suite reads that list, because
  // the shipped snapshot is a fixed artefact that a narrower filter would not
  // change until it were regenerated.
  it('runs the CJK exclusion over every subset it names, and counts what it excluded', async () => {
    const { trimSnapshot } = await import('../scripts/build-font-index.mjs')
    const cjk = ['chinese-simplified', 'chinese-traditional', 'chinese-hongkong', 'japanese', 'korean']
    const published = {
      familyMetadataList: [
        ...cjk.map((subset, index) => ({ family: `CJK ${subset}`, category: 'SANS_SERIF', subsets: [subset, 'latin', 'menu'], fonts: { 400: {} }, popularity: index + 1 })),
        { family: 'Variable Latin', category: 'SANS_SERIF', subsets: ['latin', 'menu'], axes: [{ tag: 'wght' }], fonts: { 400: {} }, popularity: 6 },
        { family: 'Static Thai', category: 'SERIF', subsets: ['latin', 'thai', 'menu'], fonts: { 400: {}, 700: {} }, popularity: 7 },
      ],
    }
    const snapshot = trimSnapshot(published, '2026-09-03')
    expect(snapshot.families.map((entry) => entry.family), 'every CJK subset in the list is excluded and nothing else is').toEqual(['Variable Latin', 'Static Thai'])
    expect(snapshot.publishedFamilies).toBe(7)
    expect(snapshot.excludedCjkFamilies).toBe(5)
    expect(snapshot.snapshotDate).toBe('2026-09-03')
    // AND THE TRIM ITSELF: `menu` is a subsetting artefact and is dropped, and
    // `axes` is carried as the variable-only PREDICTION the browser filters on.
    expect(snapshot.families[0].axes).toEqual(['wght'])
    expect(snapshot.families[1].axes).toEqual([])
    expect(snapshot.families[1].subsets).toEqual(['latin', 'thai'])
    expect(snapshot.families[1].styles).toEqual(['400', '700'])
  })

  // AN OFFLINE RELEASE BUILD IS A SHIPPED GATE, so the step that emits this
  // module MUST NOT REACH THE NETWORK. Asserted by running the real emit step
  // with `fetch` replaced by a thrower — a check over source text would pass on
  // an indirection, and this one cannot.
  it('emits the module from committed data with no network at all', async () => {
    const { emitFontIndexModule } = await import('../scripts/build-font-index.mjs')
    const restore = globalThis.fetch
    globalThis.fetch = (() => { throw new Error('the build step must not reach the network') }) as never
    try {
      const emitted = emitFontIndexModule()
      expect(emitted.families).toBe(familyIndex.length)
      expect(emitted.snapshotDate).toBe(familyIndexSnapshotDate)
    } finally {
      globalThis.fetch = restore
    }
  })

  it('carries no licence field, because no licence is knowable before a pick', () => {
    const row = familyIndex[0] as unknown as Record<string, unknown>
    expect(Object.keys(row).sort()).toEqual(['category', 'family', 'popularity', 'scripts', 'variable'])
  })
})

describe('the local face tier', () => {
  // THE COMMITTED FACES SURVIVE EPIC 16 UNCHANGED. The family-pick path
  // gained a source; it did not swap one. It was 21 when Story 16.1 wrote this;
  // Story 16.1a's batch made it 31, and the count is deliberately not restated
  // here — the floor below is where the population is asserted, and a second
  // number in prose is a second authority that ages.
  it('is the whole bundled catalogue, unchanged', () => {
    // THE POPULATION FLOOR — ONE OF FOUR, AND ALL FOUR MOVE TOGETHER.
    // The other three are `src/font-catalogue.test.ts` ("declares at least twenty NEW families"),
    // `src/font-name-table.test.ts` ("reads a copyright out of every committed
    // catalogue face") and
    // `src/font-provenance.test.ts` ("is asserted over the whole committed tier").
    // Raised 20 -> 31 by Story 16.1a, which added ten families to the local
    // face tier. D-16.R.12: "a floor left at 21 while the tier grows to 30 is
    // a floor that stops measuring the thing it was built to measure" — and
    // D-16.R.18's correction to it: a floor that exists in N files is N
    // floors, so a batch that raises one and leaves the rest behind is
    // silently unmeasured at the ones it left.

    expect(catalogueFaces.length, 'the local face tier population floor; Story 16.1a raised it 20 -> 31').toBeGreaterThanOrEqual(31)
    expect(catalogueFaces.length).toBe(manifest.length)
    for (const face of catalogueFaces) expect(localTierHolds(face.family)).toBe(true)
  })

  // JOIN KEY: EXACT `family` STRING EQUALITY. No case-folding, no whitespace
  // normalisation, no fuzzy match. `Geist` / `Geist Mono` / `Geist Pixel` is
  // exactly the neighbourhood a loose matcher gets wrong.
  it('joins on exact string equality and on nothing looser', () => {
    for (const face of catalogueFaces) {
      expect(localTierHolds(face.family.toLowerCase()), `${face.family} must not join case-insensitively`).toBe(face.family === face.family.toLowerCase())
      expect(localTierHolds(` ${face.family}`)).toBe(false)
      expect(localTierHolds(face.family.replace(/ /g, ''))).toBe(face.family.indexOf(' ') === -1)
    }
  })

  // A LOCAL FACE WITH NO INDEX ROW IS LOCAL-TIER-ONLY, AND THAT IS CORRECT
  // BEHAVIOUR, NOT A DEFECT. Measured: `Inter Display` and `Source Serif 4
  // Display` have no index row at all, so 2 of 21 are unjoinable under any
  // normalisation.
  it('still offers a local face the published index has never heard of', () => {
    const indexed = new Set(familyIndex.map((row) => row.family))
    const unjoinable = catalogueFaces.filter((face) => !indexed.has(face.family))
    expect(unjoinable.length, 'this assertion is only meaningful while some local face is absent from the index').toBeGreaterThan(0)
    for (const face of unjoinable) {
      expect(offeredFamilies(face.family).some((source) => source.tier === 'local' && source.family === face.family)).toBe(true)
      expect(webFamilies.some((row) => row.family === face.family)).toBe(false)
    }
  })

  // "VARIABLE-ONLY" IS A PROPERTY OF THE BYTE SOURCE, NOT OF THE FAMILY
  // (D-16.R.2a). `Roboto` and `Inter` are committed here as byte-for-byte
  // upstream STATICS and appear among the mirror's variable-only rows only
  // because `google/fonts` carries VF-only builds of them. The index's `axes`
  // field is not consulted for a family the local tier holds.
  it('offers a locally-held family from the local tier even when the index calls it variable-only', () => {
    const variableUpstream = catalogueFaces.filter((face) => familyIndex.some((row) => row.family === face.family && row.variable))
    expect(variableUpstream.map((face) => face.family), 'the measured cases this rule exists for').toEqual(expect.arrayContaining(['Roboto', 'Inter', ...batchFamilies]))
    for (const face of variableUpstream) {
      const offered = offeredFamilies(face.family).filter((source) => source.family === face.family)
      expect(offered, `${face.family} must be offered exactly once`).toHaveLength(1)
      expect(offered[0].tier).toBe('local')
    }
  })

  // STORY 16.1a — THE BATCH, NAMED, AND WHAT ADDING IT DID TO THE COUNT.
  //
  // Every one of these ten is variable-only on the `google/fonts` mirror and was
  // therefore already filtered OUT of `webFamilies` before this story; each one's
  // own project publishes an ordinary static Regular, which is what the local
  // tier now carries. So ADDING A FAMILY LOCALLY IS WHAT MAKES IT OFFERED, and
  // the addable count rises by exactly the batch size rather than by some
  // number that depends on what the mirror happened to hold.
  it('offers every family the batch added, exactly once and from the local tier', () => {
    expect(batchFamilies.length, 'the batch list is empty, so every assertion over it is vacuous').toBe(10)
    for (const family of batchFamilies) {
      expect(localTierHolds(family), `${family} was added by Story 16.1a and the local tier does not hold it`).toBe(true)
      const offered = offeredFamilies(family).filter((source) => source.family === family)
      expect(offered, `${family} must be offered exactly once`).toHaveLength(1)
      expect(offered[0].tier, `${family} must be offered from the local tier, with no fetch`).toBe('local')
      expect(webFamilies.some((row) => row.family === family), `${family} must not also be offered from the web tier`).toBe(false)
    }
  })

  // AND THE COUNT ROSE BY EXACTLY THE BATCH SIZE. Recomputed from the index and
  // from the catalogue MINUS the batch, rather than compared against a number
  // typed in from a previous run — a hardcoded "before" is a second authority
  // that ages, and this one cannot.
  it('raised the addable count by exactly the batch size', () => {
    const beforeLocal = new Set(catalogueFaces.map((face) => face.family).filter((family) => !batchFamilies.includes(family)))
    expect(beforeLocal.size, 'the pre-batch tier is the catalogue minus the ten').toBe(catalogueFaces.length - batchFamilies.length)
    const beforeWeb = familyIndex.filter((row) => !row.variable && !beforeLocal.has(row.family))
    const beforeAddable = beforeWeb.length + beforeLocal.size
    expect(
      addableFamilyCount - beforeAddable,
      'the batch adds ten families the web tier could not offer, so the addable count must rise by exactly ten. A batch family that were statically published on the mirror would have been addable already, and the delta would be short by one.',
    ).toBe(batchFamilies.length)
  })

  it('offers a family the local tier holds exactly once, never from both tiers', () => {
    const offered = offeredFamilies('').map((source) => source.family)
    expect(new Set(offered).size).toBe(offered.length)
    for (const face of catalogueFaces) expect(webFamilies.some((row) => row.family === face.family)).toBe(false)
  })
})

describe('what the browser shows and what it says about it', () => {
  // A FAMILY THAT CANNOT BE ADDED IS FILTERED OUT, NOT LISTED AND REFUSED
  // (D-16.R.2, owner). Measured: 37 of the 50 most popular families are
  // variable-only, so listing and refusing means the most common first action
  // in the product fails.
  it('filters out variable-only families the local tier does not hold', () => {
    const variable = familyIndex.filter((row) => row.variable)
    expect(variable.length, 'about a quarter of the library ships variable-only').toBeGreaterThan(300)
    for (const row of webFamilies) expect(row.variable, `${row.family} is variable-only and must not be listed`).toBe(false)
    // AND THE HIDDEN ROWS ARE HIDDEN, not merely marked.
    expect(offeredFamilies('').some((source) => source.tier === 'web' && source.row.variable)).toBe(false)
  })

  // THE COUNT'S OWN FACTS, REHOMED SO THEY OUTLIVE THE SENTENCE THAT QUOTED THEM
  // (Story 16.10). Both assertions were written inside a test named for
  // `familyIndexDisclosure()`. That sentence is gone — the design's header draws
  // no paragraph — but the number is not: `resultLine` still prints
  // `N of ${addableFamilyCount} families` in the browser's results toolbar. The
  // wording assertions retired with their subject; these two did not, because
  // deleting a live assertion along with a dead one leaves the number that still
  // ships unguarded while every gate stays green.
  //
  // 1,946 IS NOT THIS COUNT. It is what the upstream source PUBLISHED on the
  // snapshot date; the addable count is strictly smaller because variable-only
  // rows are dropped, and keeping the inequality asserted is what stops the two
  // being confused for one another.
  it('reports the ADDABLE count, which is strictly smaller than the published one', () => {
    expect(addableFamilyCount).toBe(webFamilies.length + catalogueFaces.length)
    expect(addableFamilyCount).toBeLessThan(familyIndexPublishedFamilies)
  })

  it('searches both tiers, local first, and matches on substring', () => {
    const results = offeredFamilies('kanit')
    expect(results.some((source) => source.family === 'Kanit')).toBe(true)
    const all = offeredFamilies('')
    const firstWeb = all.findIndex((source) => source.tier === 'web')
    const lastLocal = all.map((source) => source.tier).lastIndexOf('local')
    expect(lastLocal).toBeLessThan(firstWeb)
  })
})

// STORY 16.2 — THE THIRD TIER: THE FACES THIS MACHINE ALREADY HOLDS.
//
// The store's listing joins `FamilySource` as a third `'stored'` arm
// (D-16.R.33 R1). 16.2 builds the SEAM; 16.4 adds the headings that group it.
// The seam is what makes the hand-off a mechanism rather than a sentence: an
// exhaustive switch over the union stops compiling if an arm is unhandled.
describe('the faces this machine already holds', () => {
  const stored = (family: string, key: string, style = 'Regular'): StoredFace => ({
    key,
    family,
    style,
    licence: 'OFL-1.1',
    licenceText: 'SIL Open Font License',
    copyright: 'Copyright',
    source: `google/fonts — ofl/${family.toLowerCase()}/x.ttf, fetched 2026-09-03`,
    mediaType: 'font/ttf',
    scripts: ['latin'],
    fetchedAt: '2026-09-03',
    byteLength: 1024,
  })

  /**
   * THE CENSUS THAT MAKES A STORED FAMILY READ INSTALLED
   * (spec-install-all-face-cuts, story 1, D-5).
   *
   * A stored row is no longer installed by the mere fact of being stored: the
   * family must hold every cut it publishes, or carry a refusal for each it
   * lacks. Every case below that wants a family to read INSTALLED says which
   * cuts upstream publishes; the cases about ORDER and MEMBERSHIP deliberately
   * do not, because `offeredFamilies` does not consult the census at all.
   */
  const census = (family: string, published: ReadonlyArray<string> = ['Regular'], refused: ReadonlyArray<FamilyCutRefusal> = []): FamilyCensus =>
    ({ family, published, refused, recordedAt: '2026-09-03' })

  // A FAMILY THE STORE HOLDS IS OFFERED FROM THE STORE, NOT FROM THE WEB. One
  // family, one row, from the cheapest tier that can serve it — the same rule
  // the local tier already gets. Two rows for one family, one saying "already
  // here" and one saying "will be downloaded", would make the author choose
  // between two spellings of one thing.
  it('offers a stored family from the store instead of from the snapshot', () => {
    const snapshotOnly = webFamilies.find((row) => !localTierHolds(row.family))
    expect(snapshotOnly, 'the snapshot must contain at least one family the local tier does not hold').toBeDefined()
    const family = snapshotOnly!.family
    const withoutStore = offeredFamilies(family).filter((source) => source.family === family)
    expect(withoutStore.map((source) => source.tier)).toEqual(['web'])
    const withStore = offeredFamilies(family, [stored(family, 'a'.repeat(64))]).filter((source) => source.family === family)
    expect(withStore.map((source) => source.tier), 'a stored family replaces its web row rather than sitting beside it').toEqual(['stored'])
  })

  // THE LOCAL TIER IS NOT DISPLACED BY THE STORE. Those committed faces carry a
  // reviewed licence identifier, the upstream licence file committed beside the
  // binary and a build-time gate over all of it — a stronger record than any
  // fetch can produce, including the fetch that filled the store. And they need
  // no network either, so there is nothing to win by preferring a fetched copy.
  it('never displaces a local-tier family with a stored copy of the same name', () => {
    const local = catalogueFaces[0]!.family
    const offered = offeredFamilies(local, [stored(local, 'b'.repeat(64))]).filter((source) => source.family === local)
    expect(offered.map((source) => source.tier)).toEqual(['local'])
  })

  // A STORED FAMILY THE SNAPSHOT NO LONGER LISTS IS STILL OFFERED. The index is
  // a build-time snapshot that ages, so a family fetched under one release can
  // be withdrawn or renamed upstream before the next. Its bytes are here and
  // its licence record is here; refusing to offer it because a dated list no
  // longer mentions it would be the store failing at the one job it exists for.
  it('still offers a stored family the snapshot has since stopped listing', () => {
    const withdrawn = 'A Family Upstream Withdrew'
    expect(webFamilies.some((row) => row.family === withdrawn)).toBe(false)
    const offered = offeredFamilies(withdrawn, [stored(withdrawn, 'c'.repeat(64))])
    expect(offered.map((source) => [source.tier, source.family])).toEqual([['stored', withdrawn]])
  })

  // FOUR STORED FACES OF ONE FAMILY ARE ONE ROW, AND EACH CUT RESOLVES BY
  // `style` (spec-install-all-face-cuts, story 1).
  //
  // THESE TWO CASES REPLACE `mostRecentlyFetched` AND ITS SAME-DAY TIE-BREAK,
  // AND THE REPLACEMENT IS THE STORY'S OWN CLAIM. The fold used to collapse a
  // family's records to ONE, newest `fetchedAt` first and the lexicographically
  // smaller key breaking a tie. `fetchedAt` is day-granular, so four cuts
  // installed in one session ALL TIE and the tie-break handed the author
  // whichever cut hashed lowest — an arbitrary face chosen by a digest, which
  // is the silent substitution the content-address key exists to refuse. The
  // mechanism is deleted rather than tie-broken: the union holds the SET, so
  // there is nothing left to choose.
  it('offers a family holding four cuts exactly once, with each cut resolving by its own style', () => {
    const family = webFamilies.find((row) => !localTierHolds(row.family))!.family
    // THE KEYS ARE ORDERED AGAINST THE CUTS ON PURPOSE. The Regular carries the
    // lexicographically LARGEST key and the Bold Italic the smallest, so a
    // resolver that was really sorting by key — which is what the deleted
    // tie-break did — would hand back the Bold Italic and red here.
    const listing = [
      stored(family, 'f'.repeat(64), 'Regular'),
      stored(family, '0'.repeat(64), 'Bold Italic'),
      stored(family, '3'.repeat(64), 'Bold'),
      stored(family, '7'.repeat(64), 'Italic'),
    ]
    // AND IT IS STABLE ACROSS ARRIVAL ORDER, which is the other half of what
    // the fold was for: `list()` returns family-then-key order, so a resolver
    // reading position rather than `style` would answer differently here.
    for (const arrival of [listing, [...listing].reverse()]) {
      const offered = offeredFamilies(family, arrival).filter((source) => source.family === family)
      expect(offered.map((source) => source.tier), 'one family is still one row').toEqual(['stored'])
      const only = offered[0]!
      if (only.tier !== 'stored') throw new Error('the stored family was not offered from the store')
      expect(only.faces, 'every cut the store holds travels with the row').toHaveLength(4)
      expect(regularCutOf(only.faces)?.key, 'the Regular resolves by style, not by key order or arrival order').toBe('f'.repeat(64))
      for (const cut of ['Regular', 'Bold', 'Italic', 'Bold Italic']) {
        expect(only.faces.filter((face) => face.style === cut), `${cut} must resolve to exactly one record`).toHaveLength(1)
      }
    }
  })

  // AND SAME-DAY `fetchedAt` IS NO LONGER A TIE TO BREAK — it is the ordinary
  // state of a family installed in one session, and every cut survives it.
  it('keeps every cut of a family whose faces all carry the same fetch date', () => {
    const family = webFamilies.find((row) => !localTierHolds(row.family))!.family
    const sameDay = [
      { ...stored(family, 'a'.repeat(64), 'Regular'), fetchedAt: '2026-09-03' },
      { ...stored(family, 'b'.repeat(64), 'Bold'), fetchedAt: '2026-09-03' },
    ]
    const offered = offeredFamilies(family, sameDay).filter((source) => source.family === family)
    const only = offered[0]!
    if (only.tier !== 'stored') throw new Error('the stored family was not offered from the store')
    expect(only.faces.map((face) => face.style), 'a same-day install must not lose a cut').toEqual(['Regular', 'Bold'])
    expect(regularCutOf(only.faces)?.key).toBe('a'.repeat(64))
  })

  it('filters the stored tier by the same search the other two use', () => {
    const offered = offeredFamilies('zzzznothingmatchesthis', [stored('Kanit', 'd'.repeat(64))])
    expect(offered).toEqual([])
  })

  // STORY 16.4 — THE UNION ARRIVES IN THE ORDER THIS MODULE'S OWN DOC COMMENT
  // DOCUMENTS, AND THE ASSERTION IS THE RUN STRUCTURE RATHER THAN A ROW LIST.
  //
  // WHY A RUN STRUCTURE. A test naming families re-pins the order to one
  // snapshot and rots at the next index bump, which is how an ordering claim
  // ends up asserted by a comment instead of by a run. The property that
  // actually matters to every caller is one sentence long: everything
  // `familyIsInstalled` accepts comes before everything it does not.
  //
  // WHY IT MATTERS OUTSIDE THIS MODULE. The family control draws a heading over
  // the installed rows and caps only the tail. Under the pre-16.4 order the
  // installed rows came out in FOUR alternating runs, a planted stored face
  // landed at offset 900 of 1304, and 31 of 32 installed rows fell inside a
  // 50-row cap — so the control drew AVAILABLE LOCALLY over a group it could
  // not show in full. This test reds against that implementation.
  it('returns every installed row as one contiguous run, with a stored face planted deep in the snapshot', () => {
    const deep = Math.floor(webFamilies.length * 0.7)
    const planted = webFamilies[deep]!
    // THE PLANT IS THE POSITIVE CONTROL, so it is measured rather than assumed:
    // a stored family that happened to sit near the top of the snapshot would
    // pass the broken implementation too.
    expect(deep, 'the planted family must sit deep in the snapshot for this to measure anything').toBeGreaterThan(500)
    const offered = offeredFamilies('', [stored(planted.family, 'f'.repeat(64))])
    // MEASURED OVER TIER, NOT OVER `familyIsInstalled`, AND DRIVEN WITH A
    // PARTIALLY-HELD SET (spec-deferred-offline-cache, story 2). The catalogue
    // is deferred now, so a `local` row can be a family this browser has not
    // fetched; `offeredFamilies` does not consult the held set and must not, so
    // the unheld local rows sit INTERLEAVED among the held ones and an
    // installedness-keyed run structure is no longer two runs. Driving this
    // with an all-held set would restore `[true, false]` and quietly convert
    // the one live measurement of the ordering into a tautology.
    const runsOf = <T,>(flags: ReadonlyArray<T>) => flags.reduce<T[]>((acc, flag) => (acc.length === 0 || acc[acc.length - 1] !== flag ? [...acc, flag] : acc), [])
    const onThisMachineTier = offered.map((source) => source.tier !== 'web')
    expect(runsOf(onThisMachineTier), 'local and stored first, then everything that needs a download — two runs, never four').toEqual([true, false])
    // AND THE HELD SET IS GENUINELY PARTIAL HERE, which is what makes the line
    // above a claim about TIER rather than an accident of every row being held.
    const installed = offered.map((source) => familyIsInstalled(source, halfHeld))
    expect(installed.filter((flag) => flag).length, 'the partial held set must leave some catalogue rows uninstalled').toBeLessThan(onThisMachineTier.filter(Boolean).length)
    expect(runsOf(installed).length, 'an unheld catalogue family sits among held ones, so installedness is NOT two runs — the family control filters before it groups').toBeGreaterThan(2)
    // AND THE PLANTED ROW IS INSIDE THE ON-THIS-MACHINE RUN, so the structure
    // above cannot be satisfied by dropping it instead of moving it.
    const at = offered.findIndex((source) => source.family === planted.family)
    expect(at, 'the planted stored family must still be offered').toBeGreaterThanOrEqual(0)
    expect(offered[at]!.tier).toBe('stored')
    expect(at).toBeLessThan(onThisMachineTier.indexOf(false))
    // THE POPULATION IS STATED BESIDE THE STRUCTURE. Two runs over a list with
    // only one kind of row in it would be a vacuous pass.
    expect(onThisMachineTier.filter((flag) => flag).length).toBeGreaterThan(catalogueFaces.length)
    expect(onThisMachineTier.filter((flag) => !flag).length).toBeGreaterThan(0)
  })

  // AND THE SAME PROPERTY WITH THE STORE EMPTY, because the repair must not be
  // a special case that only fires when something is planted: the local tier is
  // the installed run on a fresh machine and it is already contiguous.
  it('returns one contiguous installed run with no store at all', () => {
    const runs = offeredFamilies('').map((source) => source.tier !== 'web').reduce<boolean[]>((acc, flag) => (acc.length === 0 || acc[acc.length - 1] !== flag ? [...acc, flag] : acc), [])
    expect(runs).toEqual([true, false])
  })

  // THE SEAM ITSELF. Every arm of the union has a sentence, and the switch that
  // produces it is exhaustive — so a fourth tier added without being handled
  // stops compiling rather than silently rendering nothing.
  //
  // BEHAVIOUR-CHANGED (Story 16.5). All three sentences said `add to document`,
  // and for two of the three tiers that is now false in the opposite direction
  // from the third: picking a family this machine does not hold INSTALLS it and
  // sends no command at all, while picking one it does hold embeds the face and
  // sets the property. The note is the only place an author is told which of
  // those two a row will do, so the assertion is about that and not about a
  // spelling.
  it('describes every tier of the union, and says which rows install and which are used', () => {
    const family = webFamilies.find((row) => !localTierHolds(row.family))!.family
    const web = offeredFamilies(family).find((source) => source.family === family)!
    const fromStore = offeredFamilies(family, [stored(family, 'e'.repeat(64))], [census(family)]).find((source) => source.family === family)!
    const shortOfACut = offeredFamilies(family, [stored(family, 'e'.repeat(64))], [census(family, ['Regular', 'Bold'])]).find((source) => source.family === family)!
    const noCensus = offeredFamilies(family, [stored(family, 'e'.repeat(64))]).find((source) => source.family === family)!
    const local = offeredFamilies(catalogueFaces[0]!.family).find((source) => source.tier === 'local')!
    expect(familySourceNote(local)).toBe(' — use it, already on this machine')
    expect(familySourceNote(fromStore)).toBe(' — use it, already downloaded to this machine')
    expect(familySourceNote(web)).toBe(' — install on this machine')
    // The two tiers that need no network say so; the one that does, does not
    // claim otherwise.
    expect(familySourceNote(local)).toMatch(/already/)
    expect(familySourceNote(fromStore)).toMatch(/already/)
    expect(familySourceNote(web)).not.toMatch(/already/)
    // AND NO ARM MAY CLAIM A PICK REACHES THE DOCUMENT ANY MORE. Two of the
    // three do embed, but not because they were "added to the document" — the
    // property commit that follows is the author's own act on their selection.
    for (const source of [local, fromStore, web]) expect(familySourceNote(source)).not.toMatch(/add to document/)
    // THE TIE BETWEEN THIS SENTENCE AND THE FORK IT DESCRIBES. `familyIsInstalled`
    // is what the family control switches on, so a note that said "use it" over a
    // row the control would install is caught here rather than in a screenshot.
    expect(familyIsInstalled(local, allHeld)).toBe(true)
    expect(familyIsInstalled(fromStore, allHeld)).toBe(true)
    expect(familyIsInstalled(web, allHeld)).toBe(false)
    // AND THE CLAUSE STORY 2 ADDED: a catalogue family this browser has not
    // fetched is NOT installed, whatever the release ships. The web tier is
    // still never installed.
    expect(familyIsInstalled(local, new Set())).toBe(false)
    expect(familyIsInstalled(fromStore, new Set())).toBe(true)
    expect(familyIsInstalled(web, new Set([web.family]))).toBe(false)
    // AND THE CLAUSE spec-install-all-face-cuts STORY 1 ADDED (D-4/D-5), WHICH
    // IS `familyIsComplete`'s AND NOT THIS PREDICATE'S (D-8). Holding the
    // Regular of a family that publishes a Bold is INCOMPLETE, and is offered
    // for install again so the missing cut is reachable at all.
    expect(familyIsComplete(shortOfACut, allHeld), 'a family short of a published cut is not complete').toBe(false)
    // A family installed before this change carries no census, so there is no
    // authority on what it publishes and it reads incomplete — the honest
    // answer rather than a migration.
    expect(familyIsComplete(noCensus, allHeld), 'no census means no claim this designer can make about completeness').toBe(false)
    // ⚠ AND BOTH OF THEM ARE STILL **USABLE** (D-8). Completeness governs
    // whether a family is re-offered for install; it does not govern whether
    // the faces already on this machine can be applied. A census-less family
    // dropping out of AVAILABLE LOCALLY would make a font sitting on the
    // machine unusable offline, which is strictly worse than the re-offer D-4
    // accepted.
    expect(familyIsInstalled(shortOfACut, allHeld), 'a family short of a cut is still usable').toBe(true)
    expect(familyIsInstalled(noCensus, allHeld), 'a family installed before this story is still usable').toBe(true)
    // AND THE REFUSAL CLAUSE, which is what stops incompleteness from being a
    // re-offer loop: a cut upstream cannot serve is settled once it has been
    // recorded as PERMANENT.
    const refusedItsBold = offeredFamilies(family, [stored(family, 'e'.repeat(64))], [census(family, ['Regular', 'Bold'], [{ style: 'Bold', reason: 'upstream publishes it as a variable font', permanence: 'permanent' }])]).find((source) => source.family === family)!
    expect(familyIsComplete(refusedItsBold, allHeld), 'a permanent refusal settles a cut this machine cannot have').toBe(true)
    // ⚠ A TRANSIENT ONE DOES NOT. The stall sentence says "try the pick again
    // if you like", and this is what makes that true for a cut that is not the
    // base: the family stays incomplete, so it is offered again and the Bold is
    // fetched on the next pick.
    const stalledItsBold = offeredFamilies(family, [stored(family, 'e'.repeat(64))], [census(family, ['Regular', 'Bold'], [{ style: 'Bold', reason: 'its body stopped responding', permanence: 'transient' }])]).find((source) => source.family === family)!
    expect(familyIsComplete(stalledItsBold, allHeld), 'a transient refusal settles nothing').toBe(false)
    expect(familyIsInstalled(stalledItsBold, allHeld), 'and the family is usable while it waits to be retried').toBe(true)
  })
})

// STORY 16.3 — THE CHIP VOCABULARIES ARE TIED TO THE POPULATION THEY FILTER.
//
// The mockup hardcodes its chip lists, and its lists are exactly the values its
// FOURTEEN placeholder families happen to carry — four categories because there
// was no fifteenth family, and Cyrillic/Greek chips for coverage this snapshot
// records for nobody. Hand-copying either produces a control that can only ever
// empty the list.
//
// THESE TESTS DO NOT RECOMPUTE THE DERIVATION. Asserting a derived value equals
// itself computed the same way is a test that cannot fail. The expectation here
// is built from `offeredFamilies('')` — the function the BROWSER actually asks
// for its rows — so the two agree only if the vocabulary really is the offered
// population's.
describe('the browser chips name values the offered population actually carries', () => {
  const offered = offeredFamilies('')

  it('offers no chip that cannot match a family, in either vocabulary', () => {
    // The property that matters, stated directly: a chip that matches nothing is
    // a false affordance. This cannot be satisfied vacuously — it fails the
    // moment a value is added that no offered family carries.
    expect(offered.length).toBeGreaterThan(1000)
    for (const category of indexCategories) {
      const matching = offered.filter((source) => indexRowFor(source.family)?.category === category)
      expect(matching.length, `category chip "${category}" matches no offered family`).toBeGreaterThan(0)
    }
    for (const script of indexScripts) {
      const matching = offered.filter((source) => {
        const scripts = source.tier === 'local' ? sourceScripts(source) : indexRowFor(source.family)?.scripts ?? []
        return scripts.includes(script)
      })
      expect(matching.length, `script chip "${script}" matches no offered family`).toBeGreaterThan(0)
    }
  })

  it('names every value the offered population carries, so no family is unreachable by chip', () => {
    const categoriesOffered = [...new Set(offered.map((source) => indexRowFor(source.family)?.category).filter((value): value is string => value !== undefined))].sort()
    const scriptsOffered = [...new Set(offered.flatMap((source) => source.tier === 'local' ? [...sourceScripts(source)] : [...(indexRowFor(source.family)?.scripts ?? [])]))].sort()
    expect([...indexCategories]).toEqual(categoriesOffered)
    expect([...indexScripts]).toEqual(scriptsOffered)
  })

  it('carries the two values the mockup omitted and drops the two it invented', () => {
    // Handwriting is the mockup's omission and it is not a tail: it is the third
    // largest category of the OFFERED population. The figure is stated against
    // that denominator on purpose — 337 is the count over the whole 1,811-row
    // index, and the browser does not offer all of those.
    expect(indexCategories).toContain('Handwriting')
    const handwriting = offered.filter((source) => indexRowFor(source.family)?.category === 'Handwriting')
    expect(handwriting.length).toBeGreaterThan(200)
    expect(handwriting.length).toBeLessThan(337)
    expect(indexScripts).not.toContain('cyrillic')
    expect(indexScripts).not.toContain('greek')
    // AND `cjk` IS THE ONE THE TYPE WOULD HAVE ADDED. `CatalogueScript` carries
    // it and `scriptFallbackFaces` names a CJK fallback FACE, but no family is
    // offered under it — the snapshot excludes CJK wholesale. A vocabulary
    // derived from the TYPE rather than the DATA would ship a dead chip here.
    expect(indexScripts).not.toContain('cjk')
  })
})
