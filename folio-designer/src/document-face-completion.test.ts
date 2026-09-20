// COMPLETING AN OPENED DOCUMENT'S FAMILIES — THE PURE HALF
// (spec-install-all-face-cuts, CAP-4, story 4).
//
// One test per row of the story's I/O matrix that needs no App: the selection
// is a function of the projection, the offered sources and the release cache's
// holdings, so every one of those rows can be measured here rather than through
// a rendered designer. The rows that need a fetch, a store or a document — the
// non-mutation proof, the open-not-blocked proof, the generation guard, the
// offline outcome, the decline and the partial local landing — are in
// `App.test.tsx`.
import { describe, expect, it } from 'vitest'
import { COMPLETION_CONFIRM_LABEL, COMPLETION_DECLINE_LABEL, COMPLETION_OFFLINE, COMPLETION_QUESTION_TITLE, completionProgress, completionQuestion, completionSettled, completionShortfall, documentFamilies, incompleteDocumentFamilies } from './document-face-completion'
import type { CanvasProjection } from './engine-protocol'
import type { FamilySource } from './font-index'
import type { LocalFaceHoldings } from './held-local-faces'
import type { FamilyCensus, StoredFace } from './font-store'
import { MAX_ENGINE_FONT_FAMILIES } from './engine-protocol'
import { catalogueFaces } from './generated/font-catalogue'

// A chain entry in its two projected kinds. AD-8: exactly one of `face` and
// `assetKey` is non-empty, and only an EMBEDDED entry carries a family name.
const shipped = (name: string) => ({ face: name, assetKey: '', family: '', style: '', bold: '', italic: '', boldItalic: '' })
const carried = (assetKey: string, family: string) => ({ face: '', assetKey, family, style: 'Regular', bold: '', italic: '', boldItalic: '' })
const chains = (...entries: ReadonlyArray<ReadonlyArray<ReturnType<typeof shipped> | ReturnType<typeof carried>>>): CanvasProjection['fontChains'] =>
  entries.map((list, index) => ({ name: `chain${index}`, entries: list }))

const storedFace = (family: string, style: string): StoredFace => ({ key: `${family}-${style}`.toLowerCase(), family, style, licence: 'OFL-1.1', licenceText: 'terms', copyright: 'Copyright', source: 'upstream', mediaType: 'font/ttf', scripts: ['latin'], fetchedAt: '2026-09-20', byteLength: 10 })
const census = (family: string, published: ReadonlyArray<string>, refused: FamilyCensus['refused'] = []): FamilyCensus => ({ family, published, refused, recordedAt: '2026-09-20' })
const storedSource = (family: string, styles: ReadonlyArray<string>, row?: FamilyCensus): FamilySource =>
  row === undefined ? { tier: 'stored', family, faces: styles.map((style) => storedFace(family, style)) } : { tier: 'stored', family, faces: styles.map((style) => storedFace(family, style)), census: row }

// THE COMMITTED TIER'S FIXTURE IS THE REAL CATALOGUE, not a hand-built one: the
// local arm's whole question is what the shipped catalogue declares for a
// family, and a fixture would make the answer true by construction.
const multiCutFamily = (() => {
  const counts = new Map<string, number>()
  for (const face of catalogueFaces) counts.set(face.family, (counts.get(face.family) ?? 0) + 1)
  const found = [...counts].find(([, count]) => count > 1)?.[0]
  if (found === undefined) throw new Error('the shipped catalogue declares no family with more than one cut, so this story has no local arm to measure')
  return found
})()
const localSource = (family: string): FamilySource => ({ tier: 'local', family, faces: catalogueFaces.filter((face) => face.family === family) })

const holdings = (complete: ReadonlyArray<string>, usable: ReadonlyArray<string> = complete): LocalFaceHoldings => ({ usable: new Set(usable), complete: new Set(complete) })

describe("the families an opened document names", () => {
  it('names the families of EMBEDDED entries only, deduped and in chain order', () => {
    expect(documentFamilies(chains(
      [carried('a'.repeat(64), 'Kanit'), shipped('Noto Sans Thai')],
      [carried('b'.repeat(64), 'Inter'), carried('c'.repeat(64), 'Kanit')],
    ))).toEqual(['Kanit', 'Inter'])
  })

  // SHIPPED-ONLY DOCUMENT (matrix row 2). A `face` entry names a FontSet face
  // the build ships; there is no family here this designer could complete.
  it('names nothing for a document whose chains name only shipped faces', () => {
    expect(documentFamilies(chains([shipped('Noto Sans'), shipped('Noto Sans SC')]))).toEqual([])
  })

  // THE DISCRIMINANT DECIDES, NOT THE STRING'S SHAPE: a 64-character face name
  // is a legal face name, and reading it as an asset key would cross the two
  // namespaces on exactly the value that looks like it could not.
  it('reads the discriminant rather than the shape of the string', () => {
    expect(documentFamilies(chains([shipped('f'.repeat(64))]))).toEqual([])
  })

  it('skips an embedded entry that carries no family name rather than guessing one', () => {
    expect(documentFamilies(chains([carried('d'.repeat(64), '')]))).toEqual([])
  })
})

describe('the families an opened document is short of cuts for', () => {
  // NOTHING TO COMPLETE (matrix row 1).
  it('selects nothing when every family the document names is complete', () => {
    const complete = storedSource('Kanit', ['Regular', 'Bold'], census('Kanit', ['Regular', 'Bold']))
    expect(incompleteDocumentFamilies(chains([carried('a'.repeat(64), 'Kanit')]), [complete], holdings([]))).toEqual([])
  })

  // STORED FAMILY SHORT A CUT (matrix row 4).
  it('selects a stored family whose census publishes a cut this machine does not hold', () => {
    const short = storedSource('Kanit', ['Regular'], census('Kanit', ['Regular', 'Bold']))
    expect(incompleteDocumentFamilies(chains([carried('a'.repeat(64), 'Kanit')]), [short], holdings([]))).toEqual([short])
  })

  // PRE-STORY-1 FAMILY (matrix row 5): face records, no census row. Counted
  // incomplete, because the census is the only authority on what a family
  // publishes and completion is what establishes it.
  it('counts a family with face records and no census as short', () => {
    const unchecked = storedSource('Kanit', ['Regular', 'Bold', 'Italic', 'Bold Italic'])
    expect(incompleteDocumentFamilies(chains([carried('a'.repeat(64), 'Kanit')]), [unchecked], holdings([]))).toEqual([unchecked])
  })

  // PERMANENTLY REFUSED CUT (matrix row 10). `censusIsComplete` already treats
  // it as settled, so it is neither offered nor fetched — and a TRANSIENT
  // refusal is deliberately the other answer.
  it('leaves a permanently refused cut settled and keeps a transient one short', () => {
    const permanent = storedSource('Kanit', ['Regular'], census('Kanit', ['Regular', 'Bold'], [{ style: 'Bold', reason: 'variable fvar', permanence: 'permanent' }]))
    const transient = storedSource('Inter', ['Regular'], census('Inter', ['Regular', 'Bold'], [{ style: 'Bold', reason: 'the host stalled', permanence: 'transient' }]))
    const projection = chains([carried('a'.repeat(64), 'Kanit'), carried('b'.repeat(64), 'Inter')])
    expect(incompleteDocumentFamilies(projection, [permanent, transient], holdings([]))).toEqual([transient])
  })

  // CATALOGUE FAMILY SHORT A CUT (matrix row 6). A committed family is embedded
  // into the document exactly as a fetched one is, so it arrives through the
  // same `assetKey` filter — and `complete` is the field that decides.
  it('selects a committed family absent from the release cache\'s complete set, and drops it once held', () => {
    const local = localSource(multiCutFamily)
    const projection = chains([carried('a'.repeat(64), multiCutFamily)])
    expect(incompleteDocumentFamilies(projection, [local], holdings([], [multiCutFamily]))).toEqual([local])
    expect(incompleteDocumentFamilies(projection, [local], holdings([multiCutFamily]))).toEqual([])
  })

  // ⚠ `familyIsComplete`, NEVER `familyIsInstalled` (D-8). A family whose
  // Regular is held is INSTALLED and may still be short; selecting on
  // installedness would select nothing the document is already painting with.
  it('selects a usable-but-incomplete family, which an installedness test would not', () => {
    const local = localSource(multiCutFamily)
    expect(incompleteDocumentFamilies(chains([carried('a'.repeat(64), multiCutFamily)]), [local], holdings([], [multiCutFamily]))).toEqual([local])
  })

  it('leaves out a family no offered source knows, silently', () => {
    expect(incompleteDocumentFamilies(chains([carried('a'.repeat(64), 'Withdrawn Family')]), [storedSource('Kanit', ['Regular'])], holdings([]))).toEqual([])
  })

  it('keeps document order and reports one entry per family however many chains name it', () => {
    const kanit = storedSource('Kanit', ['Regular'], census('Kanit', ['Regular', 'Bold']))
    const inter = storedSource('Inter', ['Regular'], census('Inter', ['Regular', 'Bold']))
    const projection = chains([carried('a'.repeat(64), 'Kanit')], [carried('b'.repeat(64), 'Inter'), carried('c'.repeat(64), 'Kanit')])
    expect(incompleteDocumentFamilies(projection, [inter, kanit], holdings([]))).toEqual([kanit, inter])
  })
})

describe('what this surface says', () => {
  it('asks a question that states the count and that nothing reaches the document', () => {
    expect(COMPLETION_QUESTION_TITLE).toBe("Complete this document's typefaces?")
    expect(completionQuestion(1)).toContain('1 family in this document is missing cuts')
    expect(completionQuestion(3)).toContain('3 families in this document are missing cuts')
    for (const count of [1, 3]) expect(completionQuestion(count)).toContain('nothing is added to the document')
  })

  it('offers the safe answer and the acting one, in words that name what each does', () => {
    expect(COMPLETION_DECLINE_LABEL).toBe('Not now')
    expect(COMPLETION_CONFIRM_LABEL).toBe('Fetch the missing cuts')
  })

  it('reports numerically, and keeps the numerator moving', () => {
    expect(completionProgress(0, 3)).toBe('Font completion: 0 of 3 families…')
    expect(completionProgress(2, 3)).toBe('Font completion: 2 of 3 families…')
    expect(completionSettled(3)).toBe('Font completion: 3 families completed.')
    expect(completionSettled(1)).toBe('Font completion: 1 family completed.')
    expect(completionShortfall(2, 3)).toBe('Font completion: 2 of 3 families completed, 1 short.')
    expect(COMPLETION_OFFLINE).toBe('Font completion: no network, so nothing was fetched.')
  })

  // ⚠ THE BAR CLIPS RATHER THAN WRAPS, so a sentence too long for it loses its
  // numeric readout silently. Measured in Chromium at 1024x768 on the design
  // bar's own worst case — a 4-digit revision, `256 fonts in template`,
  // `256 of 256 elements bound` and the longest offline label, with that label
  // hidden as a standing completion line hides it: a 58-character line leaves
  // the spacer at 10.00 px and the bar exactly at 1024.00. The mono face is
  // 6.00 px per character, so ONE more character still fits and two do not.
  // 59 is therefore the measured ceiling, and it is the budget.
  //
  // THE WHOLE DOMAIN IS SCANNED, NOT FOUR SAMPLES. The counts are what vary,
  // and the digit counts do not peak where a reader expects — the worst case is
  // neither 0-of-N nor N-of-N but a middle one, where all three numbers are
  // long at once.
  //
  // ⚠ THE POPULATION IS THE DOCUMENT'S FAMILIES, NOT THE INSTALLABLE ONES, AND
  // THE TWO ARE EASY TO CONFUSE. This scan was first bound by
  // `addableFamilyCount` — every family `offeredFamilies` can OFFER, which is
  // the whole web index plus the catalogue, around 1811. That is the population
  // an author could INSTALL FROM. It is not the population this sentence counts:
  // these lines report the families THIS DOCUMENT NAMES, and
  // `incompleteDocumentFamilies` returns a subset of `documentFamilies`, which
  // reads the projection's chains — and `engine-protocol.ts` caps a projection
  // at `MAX_ENGINE_FONT_FAMILIES` families. The wrong bound made the scan
  // produce a 60-character sentence the product cannot emit, and that sentence
  // measured 1026.00 px against a 1024 px viewport: a clip the code DOCUMENTED
  // but could not actually reach.
  //
  // ⚠ THE CAP IS READ, NEVER TYPED. Bounding by the engine's own constant means
  // that if the cap moves, this budget moves with it rather than quietly
  // becoming a lie — which is the whole reason the tighter bound is safe.
  //
  // ⚠ THE MAXIMUM IS ASSERTED EXACTLY, not merely under the ceiling. A bound
  // test alone lets the sentences drift up to the budget unnoticed, which is how
  // the figure this replaced came to be wrong; an exact one fails the day a
  // wording change moves it, and the docstring in `document-face-completion.ts`
  // moves with it. `documented` stays a LITERAL: deriving it from the same
  // builders the scan calls would assert the code against itself.
  it('prices every status-bar line against the bar\'s measured budget', () => {
    const budget = 59
    const documented = 58
    let longest = COMPLETION_OFFLINE
    for (let total = 1; total <= MAX_ENGINE_FONT_FAMILIES; total++) {
      for (let completed = 0; completed <= total; completed++) {
        for (const line of [completionProgress(completed, total), completionSettled(total), completionShortfall(completed, total)]) {
          if (line.length > longest.length) longest = line
        }
      }
    }
    expect(longest.length).toBeLessThanOrEqual(budget)
    expect(longest, `the longest status line this surface can say is now ${longest.length} characters; update the figure in document-face-completion.ts's budget note`).toHaveLength(documented)
    // The worst case is the three-number sentence — all three counts at three
    // digits at once — and it is named so a later reader knows which one to
    // re-price first. Named by shape rather than by scan order, because the
    // maximum is reached by many (completed, total) pairs and which one the
    // scan meets first is not a property worth pinning.
    expect(longest.startsWith('Font completion: ') && longest.endsWith(' short.')).toBe(true)
    expect(completionShortfall(100, 200)).toHaveLength(documented)
  })
})
