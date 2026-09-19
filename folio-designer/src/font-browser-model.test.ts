import { describe, expect, it } from 'vitest'
import { addableFamilyCount, indexCategories, indexScripts, offeredFamilies, type FamilySource } from './font-index'
import { browserRows, buttonLabel, buttonName, confirmLabel, emptyStateHeading, familiesPerPage, filterRows, filtersActive, gridSpecimenCap, latinSample, noFilters, pageCount, pageLine, pageOf, pendingLine, resultLine, rowState, rowTierNote, scriptBadge, sizeReadout, sortRows, specimenFor, specimenSize, thaiSample, weightLine, type BrowserRow, type BrowserSort, type BrowserView, type RowState } from './font-browser-model'
import type { StoredFace } from './font-store'

// THE FONT BROWSER'S LOGIC, ASSERTED AGAINST THE DESIGN IT WAS PORTED FROM
// (Story 16.3).
//
// `Font Browser.dc.html`'s `renderVals()` settles the edge cases; these are the
// assertions that the port kept them, plus the three places the port
// DELIBERATELY departs from the mockup — the dropped `Most styles` arm, the
// search predicate narrowed to family and category, and a footer that states
// what this product actually embeds.

const webRow = (family: string, category: string, scripts: ReadonlyArray<'latin' | 'thai'>, popularity: number): FamilySource =>
  ({ tier: 'web', family, row: { family, category, scripts, variable: false, popularity } })

const storedRecord = (family: string, scripts: ReadonlyArray<string>): StoredFace => ({
  key: 'a'.repeat(64), family, style: 'Regular', licence: 'OFL-1.1', licenceText: 'terms', copyright: 'c',
  source: 'google/fonts — ofl/x/X-Regular.ttf, fetched 2026-09-03', mediaType: 'font/ttf', scripts,
  fetchedAt: '2026-09-03', byteLength: 4,
})

const row = (family: string, category: string | undefined, scripts: ReadonlyArray<string>, popularity?: number): BrowserRow =>
  ({ family, source: webRow(family, category ?? 'Serif', [], popularity ?? 0), ...(category === undefined ? {} : { category }), ...(popularity === undefined ? {} : { popularity }), scripts })

describe('the font browser describes the families it is given', () => {
  it('reads category, popularity and scripts off the snapshot, and says nothing it cannot read', () => {
    const [sarabun] = browserRows([webRow('Sarabun', 'Sans Serif', ['latin', 'thai'], 12)])
    expect(sarabun?.category).toBe('Sans Serif')
    expect(sarabun?.scripts).toEqual(['latin', 'thai'])
    // A family the snapshot has no row for keeps its tier's own scripts and
    // carries no category at all — never a guessed one.
    const [invented] = browserRows([{ tier: 'stored', family: 'A Face Only This Machine Has', record: storedRecord('A Face Only This Machine Has', ['latin']) }])
    expect(invented?.category).toBeUndefined()
    expect(invented?.popularity).toBeUndefined()
    expect(invented?.scripts).toEqual(['latin'])
  })

  it('names every tier a row can come from, and cannot gain a fourth silently', () => {
    // MECHANICAL (Story 16.5): the web arm's verb follows the action the
    // dialog now performs. The tier and the arm count are unchanged.
    expect(rowTierNote(webRow('Kanit', 'Sans Serif', ['latin', 'thai'], 8))).toBe('downloaded when you install it')
    expect(rowTierNote({ tier: 'stored', family: 'Kanit', record: storedRecord('Kanit', ['latin']) })).toBe('downloaded to this machine')
    // THE LOCAL ARM, OVER A REAL COMMITTED FACE. It is the arm the other two are
    // measured against — the 31 faces that need no network at all — and it was
    // the one arm nothing exercised.
    const local = offeredFamilies('Arimo').find((source) => source.tier === 'local')
    expect(local, 'Arimo is a committed local-tier face').toBeDefined()
    expect(rowTierNote(local as FamilySource)).toBe('on this machine')
    expect(() => rowTierNote({ tier: 'gossip' } as unknown as FamilySource)).toThrow(/gossip/)
  })

  it('reads a local-tier row\'s coverage off the committed face, not off the snapshot', () => {
    const local = offeredFamilies('').filter((source) => source.tier === 'local')
    expect(local.length).toBeGreaterThan(20)
    const rows = browserRows(local)
    for (const row of rows) {
      const face = local.find((source) => source.family === row.family)
      expect(face?.tier).toBe('local')
      // The branch the two index-less local families depend on: their scripts
      // come from the face this machine holds, because no snapshot row exists to
      // read them from.
      expect(row.scripts).toEqual(face?.tier === 'local' ? face.face.scripts : [])
    }
    // AND THOSE TWO ARE REALLY THERE, so this is not a vacuous loop over rows
    // that all happen to have an index row behind them.
    const unlisted = rows.filter((row) => row.category === undefined)
    expect(unlisted.length).toBeGreaterThan(0)
    for (const row of unlisted) expect(row.scripts.length).toBeGreaterThan(0)
  })

  it('derives its chip vocabularies from the snapshot rather than from the mockup', () => {
    // The mockup draws four category chips and six writing-system ones
    // (All/Latin/Thai/Cyrillic/Greek). Neither list is this snapshot's.
    expect(indexCategories).toContain('Handwriting')
    expect(indexCategories.length).toBeGreaterThanOrEqual(5)
    expect([...indexScripts]).toEqual(['latin', 'thai'])
    expect(indexScripts).not.toContain('cyrillic')
  })
})

describe('the mockup filter predicate, narrowed to what the snapshot carries', () => {
  const rows = [row('Sarabun', 'Sans Serif', ['latin', 'thai'], 1), row('Lora', 'Serif', ['latin'], 2), row('Chonburi', 'Display', ['latin', 'thai'], 3)]

  it('matches family and category, and has no designer field to match', () => {
    expect(filterRows(rows, { ...noFilters, query: 'sara' }).map((entry) => entry.family)).toEqual(['Sarabun'])
    expect(filterRows(rows, { ...noFilters, query: 'display' }).map((entry) => entry.family)).toEqual(['Chonburi'])
    // `Cadson Demak` designs both Sarabun and Chonburi. The mockup's predicate
    // would have found them; this snapshot carries no designer at all.
    expect(filterRows(rows, { ...noFilters, query: 'cadson' })).toEqual([])
  })

  it('intersects the script chip with the category chips', () => {
    expect(filterRows(rows, { ...noFilters, script: 'thai' }).map((entry) => entry.family)).toEqual(['Sarabun', 'Chonburi'])
    expect(filterRows(rows, { ...noFilters, script: 'thai', categories: ['Display'] }).map((entry) => entry.family)).toEqual(['Chonburi'])
    expect(filterRows(rows, { ...noFilters, script: 'thai', categories: ['Serif'] })).toEqual([])
  })

  it('shows `reset filters` exactly while a filter is active', () => {
    expect(filtersActive(noFilters)).toBe(false)
    expect(filtersActive({ ...noFilters, query: '  ' })).toBe(false)
    expect(filtersActive({ ...noFilters, query: 'sara' })).toBe(true)
    expect(filtersActive({ ...noFilters, script: 'thai' })).toBe(true)
    expect(filtersActive({ ...noFilters, categories: ['Serif'] })).toBe(true)
  })
})

describe('the two sort arms, and the one the port dropped', () => {
  const rows = [row('Zed', 'Serif', ['latin'], 9), row('Alpha', 'Serif', ['latin'], 40), row('Unranked', undefined, ['latin'])]

  it('orders Trending by the snapshot rank, most popular first', () => {
    expect(sortRows(rows, 'Trending').map((entry) => entry.family)).toEqual(['Zed', 'Alpha', 'Unranked'])
  })

  it('sorts a family the snapshot does not rank LAST, never first', () => {
    // An absent rank read as zero would put the family the snapshot says least
    // about at the head of the list it is ordering.
    expect(sortRows(rows, 'Trending').at(-1)?.family).toBe('Unranked')
  })

  it('orders A – Z by family name', () => {
    expect(sortRows(rows, 'A – Z').map((entry) => entry.family)).toEqual(['Alpha', 'Unranked', 'Zed'])
  })

  it('cannot gain a third arm that silently orders as Trending', () => {
    // Written as a ternary this returned the Trending order for any unknown
    // arm — a list in the wrong order, with nothing anywhere saying so.
    expect(() => sortRows(rows, 'Most styles' as BrowserSort)).toThrow(/Most styles/)
  })
})

describe('the specimen, the badge and the row button', () => {
  const sarabun = row('Sarabun', 'Sans Serif', ['latin', 'thai'], 1)
  const lora = row('Lora', 'Serif', ['latin'], 2)

  it('gives a Thai-covering family the Thai sample while the toggle is on', () => {
    expect(specimenFor(sarabun, '', true)).toBe(thaiSample)
    expect(specimenFor(sarabun, '', false)).toBe(latinSample)
    expect(specimenFor(lora, '', true)).toBe(latinSample)
  })

  it('lets the author\'s own words win over both samples', () => {
    expect(specimenFor(sarabun, 'ธนาคารกรุงศรี', true)).toBe('ธนาคารกรุงศรี')
    expect(specimenFor(lora, '  spaced  ', true)).toBe('spaced')
  })

  it('badges Thai coverage the design\'s way, and says so when the snapshot states nothing', () => {
    expect(scriptBadge(sarabun)).toBe('Thai + Latin')
    expect(scriptBadge(lora)).toBe('Latin')
    expect(scriptBadge(row('Allkin', 'Display', []))).toBe('script not stated')
  })

  it('does not claim Latin coverage for a family that records only Thai', () => {
    expect(scriptBadge(row('Thai Only', 'Display', ['thai']))).toBe('Thai')
  })

  it('finds thai-without-latin among the COMMITTED faces, so the badge fix is not hypothetical', () => {
    // The review priced this as unreachable at 0 of 1,811 INDEX rows, and over
    // the index that is right. A local-tier row's coverage comes off the
    // committed face instead, and two of those record `thai` alone — so the
    // browser really was printing `Thai + Latin` beside two shipped faces whose
    // own record claims no Latin.
    const thaiOnly = browserRows(offeredFamilies('')).filter((entry) => entry.scripts.includes('thai') && !entry.scripts.includes('latin'))
    expect(thaiOnly.map((entry) => entry.family).sort()).toEqual(['Noto Sans Thai Looped', 'Noto Serif Thai'])
    for (const entry of thaiOnly) {
      expect(entry.source.tier).toBe('local')
      expect(scriptBadge(entry)).toBe('Thai')
    }
  })

  // BEHAVIOUR-CHANGED (Story 16.5). The mockup drew three states; the model now
  // has four, because a face can be ON THIS MACHINE and NOT IN THE DOCUMENT — a
  // relationship that did not exist while confirming embedded. Every arm is
  // asserted, including the new one, and the ORDER is asserted rather than left
  // to `includes` call order.
  it('carries the mockup\'s three button states plus 16.5\'s installed state, in order of precedence', () => {
    expect(rowState('Lora', ['Lora'], ['Lora'], ['Lora'])).toBe('in-template')
    expect(rowState('Lora', [], ['Lora'], ['Lora'])).toBe('staged')
    expect(rowState('Lora', [], [], ['Lora'])).toBe('installed')
    expect(rowState('Lora', [], [], [])).toBe('addable')
    // AND THE INSTALLED SET CANNOT BE OMITTED. It used to default to `[]`, so a
    // caller that forgot it compiled and reported `addable` over a face this
    // machine holds; the argument is required now, and this line is what a
    // reader checks that against.
    // @ts-expect-error the installed set is required: omitting it must not compile
    expect(() => rowState('Lora', [], [])).toBeTypeOf('function')
    expect(buttonLabel('in-template')).toBe('In template')
    expect(buttonLabel('staged')).toBe('✓ Added')
    expect(buttonLabel('installed')).toBe('On this machine')
    expect(buttonLabel('addable')).toBe('+ Install')
    // The visible label is the design's; the accessible name has to name the
    // family, because a screen reader hears twelve buttons all called `+ Install`.
    expect(buttonName('Lora', 'addable')).toBe('Install Lora on this machine')
    expect(buttonName('Lora', 'staged')).toBe('Remove Lora from the families to install')
    expect(buttonName('Lora', 'installed')).toBe('Lora is already on this machine')
    expect(buttonName('Lora', 'in-template')).toBe('Lora is in this template')
    // AND NEITHER CAN GAIN A FIFTH SILENTLY. Both switches carry a `never`
    // binding, which is the only thing standing between a new state and a
    // button drawn with no label at all.
    expect(() => buttonLabel('borrowed' as RowState)).toThrow(/borrowed/)
    expect(() => buttonName('Lora', 'borrowed' as RowState)).toThrow(/borrowed/)
  })
})

describe('the rail states the size the screen is actually using', () => {
  it('caps a card\'s specimen and says so, rather than printing a size nothing is set at', () => {
    expect(specimenSize('Row', 56)).toBe(56)
    expect(specimenSize('Grid', 56)).toBe(gridSpecimenCap)
    expect(specimenSize('Grid', 20)).toBe(20)
    expect(sizeReadout('Row', 56)).toBe('56px')
    expect(sizeReadout('Grid', 20)).toBe('20px')
    // The readout names BOTH numbers when they differ: what the cards are set
    // at, and where the author left the slider.
    expect(sizeReadout('Grid', 56)).toBe('26px of 56')
  })

  it('cannot gain a third view that silently takes the Grid cap', () => {
    expect(() => specimenSize('Carousel' as BrowserView, 56)).toThrow(/Carousel/)
  })
})

describe('the footer states what confirming will actually do', () => {
  // BEHAVIOUR-CHANGED (Story 16.5). "ready to embed" and "Add N to template"
  // were true under Story 16.3 and are false now: confirm keeps the faces on
  // this machine and writes nothing into the document. The claim is asserted
  // BOTH ways — the new words present, the old destination absent — because the
  // regression this can suffer is a footer that goes on promising the file.
  it('carries the mockup\'s pending line and confirm label, inverted to the install', () => {
    expect(pendingLine(0)).toBe('Select families to install on this machine')
    expect(pendingLine(1)).toBe('1 family ready to install')
    expect(pendingLine(3)).toBe('3 families ready to install')
    expect(confirmLabel(0)).toBe('Install on this machine')
    expect(confirmLabel(3)).toBe('Install 3 on this machine')
    for (const line of [pendingLine(0), pendingLine(3), confirmLabel(0), confirmLabel(3)]) {
      expect(line, 'the footer may not claim the template moves: confirm sends no command').not.toMatch(/template|embed/i)
    }
  })

  it('corrects the mockup\'s weight line to what this product embeds', () => {
    // The mockup prints `≈ N weights · subset latin+thai`. This product embeds
    // one upright Regular per family and subsets nothing, so a footer repeating
    // the mockup would be the one region that lies about the file.
    expect(weightLine(0)).toBe('')
    expect(weightLine(1)).toBe('1 face · one upright Regular each, no bold or italic')
    expect(weightLine(3)).toBe('3 faces · one upright Regular each, no bold or italic')
  })

  // THESE ARE A SEPARATE `it` ON PURPOSE, AND THE REASON IS A DEFECT A RED-PROOF
  // FOUND IN THE FIRST VERSION OF THEM.
  //
  // They used to sit under the exact-string assertions above. Vitest aborts an
  // `it` at the first failing expectation, and an exact-string pin ENTAILS every
  // vocabulary ban over the same input — so those bans could never be the first
  // failure, and never executed. They read like guards and were documentation.
  //
  // The prover then showed what that cost: emitting the false clause for
  // `staged === 2` alone shipped `2 faces · … · whole file, not subset, added to
  // template` with the FULL SUITE GREEN, because every assertion here only ever
  // looked at `staged === 3`. A guard pinned to one value of an input the
  // function is parameterised over is a guard over one point, not over the
  // function.
  it('never speaks about subsetting or about a destination, at any staged count', () => {
    for (const staged of [1, 2, 3, 4, 5, 12, 40]) {
      const line = weightLine(staged)
      // The product DOES subset (at PDF render, over the glyphs the document
      // uses), so "subset latin+thai" and "whole file, not subset" are BOTH
      // false. The footer may not speak about it in either direction.
      expect(line, `weightLine(${staged}) speaks about subsetting`).not.toMatch(/weights|subset/i)
      // AND IT MAY NOT NAME A DESTINATION, because Story 16.5 inverts it: today
      // confirm embeds, and there it installs. Destination language lives in
      // `confirmLabel` and `pendingLine`, which 16.5 revises in one place.
      expect(line, `weightLine(${staged}) names a destination`).not.toMatch(/template|file|document|embed|install/i)
      // And it still states the face fact, so this cannot be satisfied by
      // returning nothing.
      expect(line, `weightLine(${staged}) dropped the face fact`).toMatch(/one upright Regular each, no bold or italic/)
    }
    // The empty slot stays empty — the one staged count with no sentence at all.
    expect(weightLine(0)).toBe('')
  })
})

describe('the result line, the empty state and the page bound', () => {
  it('states the shown count against the matching count and the addable total', () => {
    expect(resultLine(5, 5)).toBe(`5 of ${addableFamilyCount} families`)
    expect(resultLine(12, 340)).toBe(`12 of 340 matching families, out of ${addableFamilyCount}`)
    // NEVER 1,946: that is what the source PUBLISHED on the snapshot date.
    expect(resultLine(5, 5)).not.toContain('1946')
  })

  it('names the query in the empty state, and does not invent one when there is none', () => {
    expect(emptyStateHeading('qqq')).toBe('No families match “qqq”')
    expect(emptyStateHeading('  ')).toBe('No families match these filters')
  })

  it('bounds a page at `familiesPerPage` and clamps a page index past the end', () => {
    const rows = Array.from({ length: 30 }, (_, index) => row(`Family ${index}`, 'Serif', ['latin'], index))
    expect(pageOf(rows, 0)).toHaveLength(familiesPerPage)
    expect(pageOf(rows, 2)).toHaveLength(30 - 2 * familiesPerPage)
    expect(pageCount(30)).toBe(3)
    expect(pageCount(0)).toBe(1)
    expect(pageOf(rows, 99).map((entry) => entry.family)).toEqual(pageOf(rows, 2).map((entry) => entry.family))
    expect(pageLine(0, 30)).toBe('Page 1 of 3')
    expect(pageLine(9, 30)).toBe('Page 3 of 3')
  })

  it('never puts more families on a page than may be registered for preview at once', () => {
    // The bound is the whole reason the browser pages rather than scrolls: every
    // row on screen wants a real face registered for it.
    const everything = browserRows(offeredFamilies(''))
    expect(everything.length).toBeGreaterThan(1000)
    expect(pageOf(everything, 0).length).toBe(familiesPerPage)
  })
})
