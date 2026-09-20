import { describe, expect, it } from 'vitest'
import { addableFamilyCount, familyIsComplete, indexCategories, indexScripts, offeredFamilies, sourceScripts, type FamilySource } from './font-index'
import type { LocalFaceHoldings } from './held-local-faces'
import { browserRows, buttonLabel, buttonName, confirmLabel, cutLine, emptyStateHeading, familiesPerPage, filterRows, filtersActive, gridSpecimenCap, latinSample, noFilters, pageCount, pageLine, pageOf, pendingLine, resultLine, rowState, rowTierNote, scriptBadge, sizeReadout, sortRows, specimenFor, specimenSize, thaiSample, weightLine, type BrowserRow, type BrowserSort, type BrowserView, type RowState } from './font-browser-model'
import type { FamilyCensus, FamilyCutRefusal, StoredFace } from './font-store'
import { catalogueFaces } from './generated/font-catalogue'
import { faceCutOfCatalogueStyle, faceCuts, type FaceCut } from './font-source'
import { shippedFamilyCutsOf } from './shipped-face-cuts'

// THE FONT BROWSER'S LOGIC, ASSERTED AGAINST THE DESIGN IT WAS PORTED FROM
// (Story 16.3).
//
// `Font Browser.dc.html`'s `renderVals()` settles the edge cases; these are the
// assertions that the port kept them, plus the three places the port
// DELIBERATELY departs from the mockup — the dropped `Most styles` arm, the
// search predicate narrowed to family and category, and a footer that states
// what this product actually embeds.

// THE SNAPSHOT ROW CARRIES ITS CUT SET SINCE spec-install-all-face-cuts STORY
// 5, projected at emit onto the closed RIBBI four. It DEFAULTS TO THE
// REGULAR-ONLY CASE rather than to the four-cut one, because that is the
// population: 947 of the 1,270 offered web families publish a Regular and
// nothing else, and a fixture defaulting to four cuts would make the common row
// the one nothing in this file exercises.
const webRow = (family: string, category: string, scripts: ReadonlyArray<'latin' | 'thai'>, popularity: number, cuts: ReadonlyArray<FaceCut> = ['Regular']): FamilySource =>
  ({ tier: 'web', family, row: { family, category, scripts, cuts, variable: false, popularity } })

const storedRecord = (family: string, scripts: ReadonlyArray<string>, style = 'Regular'): StoredFace => ({
  key: 'a'.repeat(64), family, style, licence: 'OFL-1.1', licenceText: 'terms', copyright: 'c',
  source: 'google/fonts — ofl/x/X-Regular.ttf, fetched 2026-09-03', authorAcknowledged: false, mediaType: 'font/ttf', scripts,
  fetchedAt: '2026-09-03', byteLength: 4,
})

const census = (family: string, published: ReadonlyArray<string>, refused: ReadonlyArray<FamilyCutRefusal> = []): FamilyCensus =>
  ({ family, published, refused, recordedAt: '2026-09-19' })

const row = (family: string, category: string | undefined, scripts: ReadonlyArray<string>, popularity?: number, cuts: ReadonlyArray<FaceCut> = ['Regular']): BrowserRow =>
  ({ family, source: webRow(family, category ?? 'Serif', [], popularity ?? 0, cuts), ...(category === undefined ? {} : { category }), ...(popularity === undefined ? {} : { popularity }), scripts, cuts })

describe('the font browser describes the families it is given', () => {
  it('reads category, popularity and scripts off the snapshot, and says nothing it cannot read', () => {
    const [sarabun] = browserRows([webRow('Sarabun', 'Sans Serif', ['latin', 'thai'], 12)])
    expect(sarabun?.category).toBe('Sans Serif')
    expect(sarabun?.scripts).toEqual(['latin', 'thai'])
    // A family the snapshot has no row for keeps its tier's own scripts and
    // carries no category at all — never a guessed one.
    const [invented] = browserRows([{ tier: 'stored', family: 'A Face Only This Machine Has', faces: [storedRecord('A Face Only This Machine Has', ['latin'])] }])
    expect(invented?.category).toBeUndefined()
    expect(invented?.popularity).toBeUndefined()
    expect(invented?.scripts).toEqual(['latin'])
  })

  it('names every tier a row can come from, and cannot gain a fourth silently', () => {
    // MECHANICAL (Story 16.5): the web arm's verb follows the action the
    // dialog now performs. The tier and the arm count are unchanged.
    expect(rowTierNote(webRow('Kanit', 'Sans Serif', ['latin', 'thai'], 8))).toBe('downloaded when you install it')
    expect(rowTierNote({ tier: 'stored', family: 'Kanit', faces: [storedRecord('Kanit', ['latin'])] })).toBe('downloaded to this machine')
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
      expect(row.scripts).toEqual(face?.tier === 'local' ? sourceScripts(face) : [])
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

  it('corrects the mockup\'s weight line to what confirming actually fetches', () => {
    // The mockup prints `≈ N weights · subset latin+thai`. This product subsets
    // nothing here, so a footer repeating the mockup would be the one region
    // that lies about the file.
    //
    // ⚠ AND THE CLAUSE THAT REPLACED IT WENT FALSE IN ITS TURN
    // (spec-install-all-face-cuts story 3). It read `N faces · one upright
    // Regular each, no bold or italic`; confirming a staged family now installs
    // every cut that family publishes, so the old string said the opposite of
    // what the button does. The noun moved with it: `staged` counts FAMILIES,
    // which was the same number as faces only under the one-face rule.
    expect(weightLine(0)).toBe('')
    expect(weightLine(1)).toBe('1 family · every cut each one publishes, up to four')
    expect(weightLine(3)).toBe('3 families · every cut each one publishes, up to four')
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
      // AND IT STILL STATES THE FACE FACT, so this cannot be satisfied by
      // returning nothing — re-pointed by story 3 at what is now true.
      expect(line, `weightLine(${staged}) dropped the face fact`).toMatch(/every cut each one publishes, up to four/)
      // AND IT MAY NOT GO BACK TO CLAIMING ONE FACE PER FAMILY, which is the
      // statement this story retired and the one a merge could reinstate.
      expect(line, `weightLine(${staged}) claims one upright Regular per family again`).not.toMatch(/one upright Regular/)
    }
    // The empty slot stays empty — the one staged count with no sentence at all.
    expect(weightLine(0)).toBe('')
  })
})

/**
 * CAP-6 — EVERY LISTED FAMILY SHOWS ITS CUTS, AND WHAT IS SHOWN MATCHES WHAT
 * INSTALLING YIELDS (spec-install-all-face-cuts story 5).
 *
 * ⚠ PARAMETERISED OVER THE INPUT, NOT PINNED AT ONE VALUE, and that is this
 * file's own recorded defect applied before it could happen again: the weight
 * line's guards above were written as bans over `weightLine(3)` alone, and a
 * false clause emitted for `staged === 2` shipped with the full suite green. A
 * cut display has exactly that shape — one function over a set with sixteen
 * possible values — so every assertion below walks the cases rather than
 * sampling one.
 */
describe('the row names the cuts installing the family will yield', () => {
  // MATRIX ROWS 1–3: THE WEB TIER, over the cut set the emit step projected.
  it('names a web family\'s projected cuts, in RIBBI order, at every arity', () => {
    const cases: ReadonlyArray<readonly [ReadonlyArray<FaceCut>, string]> = [
      // The 103 four-cut families — the case the dialog could not tell from the
      // 947 below until this line existed.
      [['Regular', 'Bold', 'Italic', 'Bold Italic'], 'Regular · Bold · Italic · Bold Italic'],
      // THE POPULATION: 947 of 1,270 offered web families publish this and
      // nothing else.
      [['Regular'], 'Regular'],
      [['Regular', 'Bold'], 'Regular · Bold'],
      [['Regular', 'Italic'], 'Regular · Italic'],
      [['Regular', 'Bold', 'Italic'], 'Regular · Bold · Italic'],
      [['Regular', 'Italic', 'Bold Italic'], 'Regular · Italic · Bold Italic'],
    ]
    for (const [cuts, expected] of cases) {
      const [drawn] = browserRows([webRow('A Family', 'Serif', ['latin'], 1, cuts)])
      expect(drawn?.cuts, `the row must carry ${expected}`).toEqual(cuts)
      expect(cutLine(drawn as BrowserRow)).toBe(expected)
    }
  })

  // MATRIX ROW 3, OVER THE REAL SHIPPED POPULATION RATHER THAN A FIXTURE.
  // `Fira Sans Extra Condensed` publishes eighteen styles — 100…900 upright and
  // italic — and the row may name exactly two of them. A fixture cannot prove
  // that, because a fixture is written by the same hand as the projection; the
  // generated module is the thing that would carry a fifth weight if the emit
  // step ever let one through.
  it('never names a weight outside the closed RIBBI four, over every offered family', () => {
    const rows = browserRows(offeredFamilies(''))
    expect(rows.length, 'this guard is only meaningful over the real offered population').toBeGreaterThan(1000)
    const vocabulary = new Set<string>(faceCuts)
    for (const row of rows) {
      for (const cut of row.cuts) expect(vocabulary.has(cut), `${row.family} names ${cut}, which is not one of the four cuts`).toBe(true)
      // AND THE LINE IS THE CUTS, so the guard cannot be satisfied by a row that
      // carries the right set and prints something else.
      if (row.cuts.length > 0) expect(cutLine(row).split(' · ')).toEqual([...row.cuts])
    }
    const manyWeights = rows.find((row) => row.family === 'Fira Sans Extra Condensed')
    expect(manyWeights, 'the eighteen-style family is the one this guard exists for').toBeDefined()
    expect(cutLine(manyWeights as BrowserRow)).toBe('Regular · Bold · Italic · Bold Italic')
    // AND THE REGULAR-ONLY MAJORITY IS ACTUALLY THERE, so the guard above is not
    // passing over a population that happens to be uniform.
    expect(rows.filter((row) => row.source.tier === 'web' && row.cuts.length === 1).length).toBeGreaterThan(900)
  })

  // MATRIX ROW 5: THE COMMITTED TIER — and the spelling bridge is the whole
  // assertion. `font-catalogue.json` spells the combined cut `BoldItalic`;
  // every other tier spells it `Bold Italic`. A row printing the catalogue's
  // spelling would be a fourth vocabulary on the screen.
  it('names a committed family\'s cuts in the store\'s spelling, never the catalogue\'s', () => {
    const lineFor = (family: string): string => {
      const local = offeredFamilies(family).find((source) => source.tier === 'local' && source.family === family)
      expect(local, `${family} is a committed local-tier family`).toBeDefined()
      const [drawn] = browserRows([local as FamilySource])
      return cutLine(drawn as BrowserRow)
    }
    // The catalogue declares these rows as Regular / Bold / BoldItalic / Italic,
    // in that order, so this also pins that the answer is ordered by the
    // VOCABULARY and not by the order the rows happen to arrive in.
    expect(lineFor('Arimo')).toBe('Regular · Bold · Italic · Bold Italic')
    expect(lineFor('DM Sans')).toBe('Regular · Bold')
    // ⚠ ROBOTO IS FOUR CUTS AND `font-catalogue.json` CARRIES ONE OF THEM.
    // `Roboto Bold`, `Roboto Italic` and `Roboto Bold Italic` ship as hardcoded
    // CORE faces of the wasm build rather than as catalogue rows, so the
    // committed rows alone answer `Regular` — and a pick routes through
    // `commitDeclaredCuts`, which declares all four, so pressing B after that
    // pick bolds. The card said `Regular`. The matrix row for this tier is "the
    // cuts actually shipped", and for a family in `shipped-face-cuts.ts`'s
    // declared mirror that is the mirror's answer unioned with the catalogue's.
    expect(lineFor('Roboto')).toBe('Regular · Bold · Italic · Bold Italic')
    // AND ROBOTO IS THE ONLY LOCAL FAMILY THE MIRROR WIDENS TODAY. Pinned as a
    // listing rather than assumed: a fifth shipped family joining the committed
    // catalogue would red this and be read for its cuts before it shipped.
    const committedFamilies = [...new Set(catalogueFaces.map((face) => face.family))].sort()
    expect(committedFamilies.filter((family) => shippedFamilyCutsOf(family) !== undefined)).toEqual(['Roboto'])
    for (const row of browserRows(offeredFamilies('').filter((source) => source.tier === 'local'))) {
      expect(cutLine(row), `${row.family} prints the catalogue's own spelling`).not.toContain('BoldItalic')
    }
  })

  /**
   * THE BRIDGE THE TIER ABOVE IS READ THROUGH, OVER THE REAL COMMITTED ROWS.
   *
   * `faceCutOfCatalogueStyle` answers `undefined` for a style neither vocabulary
   * names, and the `local` arm DROPS that answer — which is right, because
   * naming a cut this dialog has no word for is how a fifth weight gets onto the
   * screen. But it means a committed row whose `style` stopped resolving would
   * VANISH from the cut line in silence, and the `not.toContain('BoldItalic')`
   * guard above would stay green precisely because the offending cut is not
   * printed at all. So the resolution is asserted directly, row by row, over the
   * population the guard cannot speak for.
   */
  it('resolves every committed row\'s style to a cut, so none can vanish from the line', () => {
    expect(catalogueFaces.length, 'this guard is only meaningful over the real committed tier').toBeGreaterThan(30)
    const vocabulary = new Set<string>(faceCuts)
    for (const face of catalogueFaces) {
      const cut = faceCutOfCatalogueStyle(face.style)
      expect(cut, `${face.family} ${face.style} resolves to no cut and would be dropped from the line in silence`).toBeDefined()
      expect(vocabulary.has(cut as string)).toBe(true)
    }
    // NON-VACUITY: the catalogue's own spelling of the combined cut is IN the
    // population, which is the one that resolves to nothing if typed by hand.
    expect(catalogueFaces.some((face) => face.style === 'BoldItalic'), 'the combined cut must be committed for this to mean anything').toBe(true)
    expect(faceCutOfCatalogueStyle('BoldItalic')).toBe('Bold Italic')
    // AND THE NEGATIVE HALF STAYS NEGATIVE: an unnamed style is `undefined`, a
    // real answer, never a guessed cut.
    expect(faceCutOfCatalogueStyle('Bold Italic')).toBeUndefined()
    expect(faceCutOfCatalogueStyle('SemiBold')).toBeUndefined()
  })

  // MATRIX ROWS 6–9: THE STORED TIER, WHICH IS D4 — `published` MINUS
  // PERMANENTLY-REFUSED CUTS. Every arm of that rule gets its own case, and the
  // two refusal permanences are asserted against each other rather than
  // separately, because the whole decision is that they differ.
  it('shows an installed family what it publishes, less what is permanently refused', () => {
    const storedSource = (faces: ReadonlyArray<string>, published?: ReadonlyArray<string>, refused: ReadonlyArray<FamilyCutRefusal> = []): FamilySource =>
      published === undefined
        ? { tier: 'stored', family: 'Kanit', faces: faces.map((style) => storedRecord('Kanit', ['latin'], style)) }
        : { tier: 'stored', family: 'Kanit', faces: faces.map((style) => storedRecord('Kanit', ['latin'], style)), census: census('Kanit', published, refused) }
    const stored = (faces: ReadonlyArray<string>, published?: ReadonlyArray<string>, refused: ReadonlyArray<FamilyCutRefusal> = []): BrowserRow =>
      browserRows([storedSource(faces, published, refused)])[0] as BrowserRow
    // `Kanit` IS A WEB FAMILY, NOT A CATALOGUE ONE, so it is in neither holding
    // set and the empty sets are the honest argument rather than a stub —
    // `familyIsComplete`'s `stored` arm reads the census and the held styles and
    // never touches this, which is exactly the fact worth stating once.
    const noLocalHoldings: LocalFaceHoldings = { usable: new Set(), complete: new Set() }
    const every = ['Regular', 'Bold', 'Italic', 'Bold Italic']

    // COMPLETE: published ⊆ held. Every published cut is shown.
    expect(cutLine(stored(every, every))).toBe('Regular · Bold · Italic · Bold Italic')

    // A TRANSIENT GAP IS STILL A YIELD. The Bold is not on this machine and the
    // stall settles nothing, so the row goes on naming it and the next pick
    // fetches it.
    const transientSource = storedSource(['Regular', 'Italic', 'Bold Italic'], every, [{ style: 'Bold', reason: 'the network stalled', permanence: 'transient' }])
    const transient = browserRows([transientSource])[0] as BrowserRow
    expect(cutLine(transient)).toBe('Regular · Bold · Italic · Bold Italic')
    // AC4, ON THE SAME FIXTURE AND THEREFORE AS ONE FACT. A named cut this
    // machine does not hold has to stay reachable, and the cut line alone does
    // not say that: INCOMPLETE is what re-offers the family, so the button goes
    // on offering to install and the next pick fetches the Bold.
    expect(familyIsComplete(transientSource, noLocalHoldings), 'a transient refusal settles nothing').toBe(false)

    // A PERMANENT REFUSAL IS NEVER SHOWN — nothing will ever deliver it — and
    // the SAME INPUT with the SAME cut missing differs only in the permanence.
    const permanentSource = storedSource(['Regular', 'Italic', 'Bold Italic'], every, [{ style: 'Bold', reason: 'upstream publishes no such file', permanence: 'permanent' }])
    const permanent = browserRows([permanentSource])[0] as BrowserRow
    expect(cutLine(permanent)).toBe('Regular · Italic · Bold Italic')
    expect(cutLine(permanent), 'a permanently refused cut may never be named').not.toContain('Bold ·')
    // AC3, ON THE SAME FIXTURE. Dropping the cut from the line and settling the
    // family are one decision: if this read INCOMPLETE the row would offer to
    // install a Bold it has just stopped naming, and offer it again for ever.
    // The two inputs differ ONLY in the permanence, and so does the answer.
    expect(familyIsComplete(permanentSource, noLocalHoldings), 'a permanent refusal settles the cut').toBe(true)

    // A CUT THE FAMILY DOES NOT PUBLISH IS NOT INVENTED FROM THE HELD SET
    // either — `published` is the authority, and it is the smaller list here.
    expect(cutLine(stored(every, ['Regular', 'Bold']))).toBe('Regular · Bold')

    // NO CENSUS: THE HELD CUTS AND NOT ONE MORE. This is every family installed
    // before story 1, and the snapshot row for the same family publishes four —
    // so a reader that fell back to the index would print four here.
    expect(cutLine(stored(['Regular', 'Bold']))).toBe('Regular · Bold')
    expect(cutLine(stored(['Bold Italic', 'Regular'])), 'held cuts are ordered by the vocabulary too').toBe('Regular · Bold Italic')

    // AND THE EMPTY ANSWER IS STATED RATHER THAN DRAWN BLANK.
    expect(cutLine(stored([]))).toBe('cuts not stated')
    expect(cutLine(stored([], ['Regular'], [{ style: 'Regular', reason: 'gone upstream', permanence: 'permanent' }]))).toBe('cuts not stated')
  })

  // THE LINE IS A DISPLAY AND CHANGES NOTHING ELSE ON THE ROW (the story's
  // "Never" list, asserted rather than trusted). `weightLine` in particular was
  // corrected by story 3 and this story must not make it false again.
  it('leaves the footer\'s own sentences alone', () => {
    expect(weightLine(3)).toBe('3 families · every cut each one publishes, up to four')
    for (const cuts of [['Regular'], ['Regular', 'Bold', 'Italic', 'Bold Italic']] as ReadonlyArray<ReadonlyArray<FaceCut>>) {
      const drawn = browserRows([webRow('A Family', 'Serif', ['latin'], 1, cuts)])[0] as BrowserRow
      // ONE FACT PER SLOT: the cut line names cuts and says nothing about where
      // the bytes come from — `rowTierNote` is the span beside it and owns that.
      expect(cutLine(drawn)).not.toMatch(/machine|install|download|snapshot/i)
    }
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
