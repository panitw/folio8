import { addableFamilyCount, indexRowFor, type FamilySource } from './font-index'

// THE FONT BROWSER'S BEHAVIOUR, PORTED FROM THE DESIGN RATHER THAN RE-DERIVED
// FROM A SCREENSHOT (Story 16.3).
//
// `Font Browser.dc.html`'s `renderVals()` is the behavioural spec, and it
// settles a dozen edge cases that a picture does not: what the row's button says
// when a family is in the template versus merely staged, what the footer says at
// zero, which sample text a Thai-covering family gets, when `reset filters`
// appears. Every one of those is ported here, function for function, so the
// design and the product agree on the edges rather than on the middle.
//
// WHAT IS DELIBERATELY NOT PORTED, AND WHY, IN ONE PLACE:
//
//   `FAMILIES` (14) AND `SYSTEM_FONTS` (6) ARE PLACEHOLDER DATA. The story says
//   so in those words. Fourteen rows stand in for the snapshot; the six host
//   fonts are superseded by D-16.2 and by Story 16.4, which together made
//   `AVAILABLE LOCALLY` mean every face this machine already holds — the
//   release's committed catalogue and the faces this designer fetched and kept
//   — and never the fonts installed on the machine.
//
//   THE `designer` FIELD DOES NOT EXIST HERE (D-16.R.33 R3). Neither the
//   generated module nor the raw snapshot carries it, so the search predicate is
//   family and category only, and no row prints a designer's name.
//
//   THERE IS NO `Most styles` SORT ARM (D-16.R.33 R3). This product embeds
//   exactly one face per family — the upright Regular at weight 400 — so a style
//   count sorts on a difference the product erases before the author can act on
//   it.
//
//   `stylesLabel` GOES WITH IT, for the same reason and with the same
//   consequence: the row's secondary line carries what this product can honestly
//   say about a family — where its bytes come from, and which scripts it covers.

/** The design's two samples. A specimen is set in one of these unless the author types their own. */
export const latinSample = 'Everyone has the right to freedom of thought'
export const thaiSample = 'ทุกคนมีสิทธิในเสรีภาพแห่งความคิด'

/** The design's slider bounds and its opening size, in CSS pixels. */
export const minSpecimenSize = 14
export const maxSpecimenSize = 56
export const defaultSpecimenSize = 34

export type BrowserSort = 'Trending' | 'A – Z'
export const browserSorts: ReadonlyArray<BrowserSort> = ['Trending', 'A – Z']

export type BrowserView = 'Row' | 'Grid'
export const browserViews: ReadonlyArray<BrowserView> = ['Row', 'Grid']

/**
 * A CARD'S SPECIMEN IS CAPPED, AND THE READOUT HAS TO SAY THE SAME NUMBER THE
 * SCREEN IS USING.
 *
 * The design draws Grid cards 150 px tall, so it caps their specimen at 26 px
 * however far the slider is pushed. That is right; what was wrong was the rail
 * printing the slider's value beside a card set at the cap — one region stating
 * something the region beside it contradicts, which is the same defect the
 * footer's weight line was corrected for. The cap lives here, with one
 * exhaustive answer for both readers.
 */
export const gridSpecimenCap = 26

export function specimenSize(view: BrowserView, size: number): number {
  switch (view) {
    case 'Row': return size
    case 'Grid': return Math.min(size, gridSpecimenCap)
    default: {
      const unhandled: never = view
      throw new Error(`a results view nothing sizes reached the font browser: ${String(unhandled as BrowserView)}`)
    }
  }
}

/** What the rail prints: the size in use, and the slider's own value when the cap has moved it. */
export function sizeReadout(view: BrowserView, size: number): string {
  const effective = specimenSize(view, size)
  return effective === size ? `${effective}px` : `${effective}px of ${size}`
}

/**
 * HOW MANY FAMILIES MAY BE ON SCREEN — AND THEREFORE HOW MANY FACES MAY BE
 * REGISTERED FOR PREVIEW — AT ONCE. THE BOUND IS WRITTEN DOWN HERE.
 *
 * A specimen must be set in its own family, so every row on screen wants a real
 * face: bytes fetched, decoded and added to the page's font set. Over a thousand
 * addable families is therefore a fetch storm and a memory problem, and the
 * story's contract makes the bound a requirement rather than a nicety.
 *
 * TWELVE, AND THE ARITHMETIC IS THE DESIGN'S OWN. The modal is 748 px tall; its
 * header, result line and footer take 46 + 32 + 50, leaving 620 px of results.
 * A Row-view row is the name line plus a specimen of up to 56 px plus 28 px of
 * padding — about 110 px at the top of the size range — so a screenful is six
 * rows. The Grid view draws three columns of 150 px cards, so a screenful is
 * nine. Twelve covers the larger of the two with a row to spare in either.
 *
 * IT IS A PAGE, NOT A SCROLL WINDOW, AND THAT IS A DELIBERATE CHOICE RATHER
 * THAN A SHORTCUT. Registering "what is visible" as the author scrolls means
 * reading scroll offsets or element rectangles, and `canvas-authority-contract`
 * forbids both in this codebase — for the canvas's sake, but the prohibition is
 * flat and this module is not going to be the exception that softens it. A page
 * is the same claim reached honestly: the rows off the page are not rendered at
 * all, so "release what scrolls away" becomes "release what leaves the page",
 * which is a fact about the DOM rather than an estimate about the viewport.
 */
export const familiesPerPage = 12

/**
 * A ROW THE BROWSER DRAWS. It carries its `FamilySource`, so the confirm path
 * hands the seam the same value the family control's own pick would hand it —
 * there is one way into the document and this is not a second one.
 */
export type BrowserRow = Readonly<{
  family: string
  source: FamilySource
  /** From the snapshot when it has a row for this family; `undefined` when it does not. */
  category?: string
  /** Lower is more popular. Absent for a family the snapshot does not carry. */
  popularity?: number
  // PLAIN STRINGS, NOT `CatalogueScript`, AND THE WIDENING IS THE HONEST
  // DIRECTION. A stored face's coverage was recorded when its bytes were
  // fetched and the store types it as `ReadonlyArray<string>`; narrowing it back
  // to the build's own three-value union would mean either asserting a cast over
  // data that came out of a database, or silently dropping a script the store
  // holds. The chips are derived from the same vocabulary (`indexScripts`), so
  // widening costs nothing a chip can act on.
  scripts: ReadonlyArray<string>
}>

export type BrowserFilters = Readonly<{
  query: string
  /** `undefined` is the design's `All` chip. */
  script?: string
  categories: ReadonlyArray<string>
}>

export const noFilters: BrowserFilters = { query: '', categories: [] }

/**
 * WHERE A ROW'S BYTES WILL COME FROM, IN THE AUTHOR'S OWN TERMS. One exhaustive
 * switch over `FamilySource`, the same shape `familySourceNote` uses, so a
 * fourth tier stops compiling here too.
 */
export function rowTierNote(source: FamilySource): string {
  switch (source.tier) {
    case 'local': return 'on this machine'
    case 'stored': return 'downloaded to this machine'
    case 'web': return 'downloaded when you install it'
    default: {
      const unhandled: never = source
      throw new Error(`a FamilySource tier nothing describes reached the font browser: ${String((unhandled as FamilySource).tier)}`)
    }
  }
}

/** Every offered family, described from the snapshot. The caller owns the offering; this owns the description. */
export function browserRows(sources: ReadonlyArray<FamilySource>): ReadonlyArray<BrowserRow> {
  return sources.map((source) => {
    const row = indexRowFor(source.family)
    const scripts = source.tier === 'local' ? source.face.scripts : source.tier === 'stored' ? source.record.scripts : source.row.scripts
    return {
      family: source.family,
      source,
      ...(row === undefined ? {} : { category: row.category, popularity: row.popularity }),
      // The tier's own scripts win over the snapshot's. A stored face's coverage
      // was recorded from the bytes this machine actually holds; the snapshot's
      // is a claim about whatever upstream publishes under that name today.
      scripts: scripts.length > 0 ? scripts : row?.scripts ?? [],
    }
  })
}

/**
 * THE MOCKUP'S FILTER PREDICATE, NARROWED BY D-16.R.33 R3.
 *
 * `renderVals()` matches the query against `f.name` OR `f.designer`; the
 * committed snapshot carries no designer field, so this matches family and
 * category. The script and category clauses are the mockup's, unchanged.
 */
export function filterRows(rows: ReadonlyArray<BrowserRow>, filters: BrowserFilters): ReadonlyArray<BrowserRow> {
  const needle = filters.query.trim().toLowerCase()
  return rows.filter((row) => {
    if (needle !== '' && !row.family.toLowerCase().includes(needle) && !(row.category ?? '').toLowerCase().includes(needle)) return false
    if (filters.script !== undefined && !row.scripts.includes(filters.script)) return false
    if (filters.categories.length > 0 && !filters.categories.includes(row.category ?? '')) return false
    return true
  })
}

/**
 * THE SORT ARMS, OVER SNAPSHOT FIELDS. `Trending` is the mockup's `trend`
 * ascending, which here is `popularity` — the snapshot's own rank, lower being
 * more popular.
 *
 * A FAMILY THE SNAPSHOT DOES NOT RANK SORTS LAST, NEVER FIRST. Two local-tier
 * families have no index row, and an absent rank read as zero would put the two
 * families the snapshot says least about at the top of the list it is ordering.
 * Ties break on the family name so the order is total and the same on every run.
 */
export function sortRows(rows: ReadonlyArray<BrowserRow>, sort: BrowserSort): ReadonlyArray<BrowserRow> {
  const ordered = [...rows]
  // AN EXHAUSTIVE SWITCH, NOT A TERNARY WITH A FALL-THROUGH — the house idiom
  // `rowTierNote` already uses. Written as `if A–Z … else Trending`, a third arm
  // added later would silently take the Trending branch and produce a list that
  // is merely in the wrong order: no compile error, no runtime error, and
  // nothing in the UI that looks wrong enough to investigate.
  switch (sort) {
    case 'A – Z':
      return ordered.sort((left, right) => left.family.localeCompare(right.family))
    case 'Trending':
      return ordered.sort((left, right) => (left.popularity ?? Number.MAX_SAFE_INTEGER) - (right.popularity ?? Number.MAX_SAFE_INTEGER) || left.family.localeCompare(right.family))
    default: {
      const unhandled: never = sort
      throw new Error(`a sort arm nothing orders reached the font browser: ${String(unhandled as BrowserSort)}`)
    }
  }
}

/** The mockup's `specimenFor`: the author's own words, else the sample the family's coverage earns. */
export function specimenFor(row: BrowserRow, previewText: string, useThaiSample: boolean): string {
  const typed = previewText.trim()
  if (typed !== '') return typed
  return useThaiSample && row.scripts.includes('thai') ? thaiSample : latinSample
}

/**
 * The mockup's `scriptBadge`, over the coverage this snapshot actually records.
 *
 * `Thai + Latin` IS ONLY CLAIMED WHEN BOTH ARE RECORDED, AND THAT IS A LIVE
 * CORRECTION RATHER THAN A PRECAUTION.
 *
 * The mockup prints `Thai + Latin` for any Thai-covering family, and this ported
 * it. Measured over the INDEX that is harmless — 0 of 1,811 snapshot rows are
 * thai-without-latin. But a LOCAL-tier row's coverage is read off the committed
 * face and not off the snapshot (see `browserRows`), and two of the committed
 * faces record `thai` alone: `Noto Sans Thai Looped` and `Noto Serif Thai`. The
 * browser was therefore printing `Thai + Latin` beside two shipped faces whose
 * own record claims no Latin at all — a badge asserting a coverage nothing
 * recorded, which is the same defect as a specimen set in a fallback and the one
 * this screen exists to remove.
 */
export function scriptBadge(row: BrowserRow): string {
  if (row.scripts.includes('thai')) return row.scripts.includes('latin') ? 'Thai + Latin' : 'Thai'
  if (row.scripts.length === 0) return 'script not stated'
  return row.scripts.map((script) => script === 'cjk' ? 'CJK' : `${script.slice(0, 1).toUpperCase()}${script.slice(1)}`).join(' + ')
}

/**
 * THE MOCKUP DREW THREE STATES AND STORY 16.5 MAKES IT FOUR, because the model
 * now has a relationship the mockup's had no name for: a face that is ON THIS
 * MACHINE and NOT IN THE DOCUMENT.
 *
 * Under Story 16.3 that state did not exist — confirming a family embedded it,
 * so a family was either in the template or addable to it. Under embed-on-use
 * the browser's action is INSTALL, and a family this machine already holds has
 * nothing left for this dialog to do: installing it again would fetch bytes the
 * store already carries and write them back over themselves.
 *
 * THE STATE IS ADDED RATHER THAN CARVED OUT OF `'addable'` IN PLACE. Leaving
 * three values and simply disabling some of them would have compiled everywhere
 * and left the screen's own vocabulary saying `+ Add` beside a face that is
 * already here.
 */
export type RowState = 'in-template' | 'staged' | 'installed' | 'addable'

/**
 * The mockup's three button states plus 16.5's fourth, in order of precedence.
 *
 * `installed` sits BELOW `staged` and above `addable`: a staged family is one
 * the author has just acted on in this dialog, and reporting it back as merely
 * "on this machine" would lose their own action. In practice the two cannot
 * both hold — an installed row cannot be staged — and the order says which
 * wins if they ever do rather than leaving it to `includes` call order.
 *
 * `installed` IS SUPPLIED, NOT DERIVED FROM THE FAMILY NAME. The caller reads
 * it off each row's own `FamilySource` through `familyIsInstalled`, which is the
 * same predicate the family control's fork consults, so the dialog and the
 * dropdown cannot come to different conclusions about the same face.
 *
 * IT IS REQUIRED, NOT DEFAULTED. It began life as `= []`, which meant a caller
 * that forgot it compiled and quietly reported `addable` — drawing `+ Install`
 * over a face this machine already holds. The `never` bindings below protect the
 * ARMS; nothing protects the INPUT except making it impossible to omit.
 */
export function rowState(family: string, inTemplate: ReadonlyArray<string>, staged: ReadonlyArray<string>, installed: ReadonlyArray<string>): RowState {
  if (inTemplate.includes(family)) return 'in-template'
  if (staged.includes(family)) return 'staged'
  return installed.includes(family) ? 'installed' : 'addable'
}

export function buttonLabel(state: RowState): string {
  switch (state) {
    case 'in-template': return 'In template'
    case 'staged': return '✓ Added'
    case 'installed': return 'On this machine'
    case 'addable': return '+ Install'
    default: {
      const unhandled: never = state
      throw new Error(`a row state nothing labels reached the font browser: ${String(unhandled as RowState)}`)
    }
  }
}

/** The accessible name for the same control, which the label alone cannot carry. */
export function buttonName(family: string, state: RowState): string {
  switch (state) {
    case 'in-template': return `${family} is in this template`
    case 'staged': return `Remove ${family} from the families to install`
    case 'installed': return `${family} is already on this machine`
    case 'addable': return `Install ${family} on this machine`
    default: {
      const unhandled: never = state
      throw new Error(`a row state nothing names reached the font browser: ${String(unhandled as RowState)}`)
    }
  }
}

/** The mockup's `resultLine`, over the addable population rather than fourteen placeholders. */
export function resultLine(shown: number, matching: number): string {
  if (matching === shown) return `${matching} of ${addableFamilyCount} families`
  return `${shown} of ${matching} matching families, out of ${addableFamilyCount}`
}

/** The mockup's `filtersActive`, and the exact condition `reset filters` is shown under. */
export function filtersActive(filters: BrowserFilters): boolean {
  return filters.query.trim() !== '' || filters.script !== undefined || filters.categories.length > 0
}

/**
 * The mockup's `pendingLine`, saying what confirm will actually do (Story 16.5).
 *
 * "ready to embed" was true under Story 16.3 and is false now: confirm keeps the
 * face on this machine and writes nothing into the document. `weightLine`
 * anticipated this and deliberately holds no destination language, so the two
 * sentences beside each other still make one claim between them.
 */
export function pendingLine(staged: number): string {
  if (staged === 0) return 'Select families to install on this machine'
  return `${staged} ${staged === 1 ? 'family' : 'families'} ready to install`
}

/**
 * THE MOCKUP'S `weightLine`, CORRECTED TO ONE FACT ABOUT WHAT A FACE IS.
 *
 * The mockup prints `≈ N weights · subset latin+thai` over `Math.min(styles, 4)`
 * per family. Both halves are false here, and the second was corrected TWICE
 * before it was right, which is why the reasoning is written down rather than
 * the conclusion.
 *
 *   THE WEIGHT COUNT IS UNAVAILABLE BY RULING. `styles` exists in the raw
 *   snapshot but is deliberately not projected into the generated module
 *   (D-16.R.33 R3, +1,326 brotli bytes to reverse), because this product embeds
 *   exactly ONE face per family — the upright Regular at weight 400 — so a style
 *   count sorts and totals on a difference the product erases.
 *
 *   AND THIS PRODUCT DOES SUBSET, WHICH IS THE OPPOSITE OF WHAT AN EARLIER
 *   VERSION OF THIS COMMENT SAID. `folio-go/internal/fontset` subsets at PDF
 *   RENDER, over the union of glyph IDs the document actually uses — one subset
 *   call per font per document. So "whole file, not subset" was false, and
 *   "subset latin+thai" is false too: the cut is driven by document usage, not
 *   by a script range, and it happens nowhere near this dialog. The clause is
 *   dropped rather than repaired, because a render-pipeline fact surfaced in a
 *   font-PICKING dialog is true, tangential, and invites a wrong inference about
 *   file size. ONE FACT PER SLOT.
 *
 * WHAT IS LEFT IS A FACT ABOUT WHAT A FACE IS, NOT ABOUT WHERE IT GOES, and that
 * is deliberate: Story 16.5 HAS NOW inverted the destination — confirm installs
 * rather than embeds — and this line needed no edit for it, because destination
 * language was kept out of it. The inversion landed in `confirmLabel` and
 * `pendingLine`, which is where it was written down that it would.
 * "One upright Regular, no bold or italic" is true under both models.
 *
 * THE SLOT IS CONDITIONAL AND THAT IS WHY IT EARNS ITS PLACE: it is empty until
 * families are staged, so it appears exactly when the author is deciding whether
 * to commit — the best disclosure moment on the screen. See this story's Design
 * Notes for the open question it is the cheapest place to disclose.
 */
export function weightLine(staged: number): string {
  if (staged === 0) return ''
  return `${staged} ${staged === 1 ? 'face' : 'faces'} · one upright Regular each, no bold or italic`
}

/**
 * The mockup's `confirmLabel`. STORY 16.5 INVERTED THE DESTINATION: the button
 * puts faces on this machine, and a font reaches the template when something in
 * the template is set in it.
 */
export function confirmLabel(staged: number): string {
  return staged === 0 ? 'Install on this machine' : `Install ${staged} on this machine`
}

/**
 * THE CONFIRM AFFORDANCE'S ACCESSIBLE NAME, WHICH IN DEGRADED MODE IS NOT THE
 * LABEL (Story 16.5, lead guardrail 2026-09-03).
 *
 * A browser that cannot keep typefaces gets the pre-16.5 model: a pick embeds
 * directly. In this dialog that means **confirming five staged families embeds
 * five faces into the document** — the exact outcome the owner's reversal was
 * made to prevent. It is acceptable as a STATED degradation of a rare path and
 * unacceptable unstated, so the count travels in the name of the control that
 * does it, at the moment the author is deciding whether to press it.
 *
 * IT NAMES THE COUNT RATHER THAN WARNING IN THE ABSTRACT. "This may add fonts to
 * your document" is a sentence an author skims; "confirming adds 5 families to
 * this document" is a number they can check against what they staged.
 */
export function confirmName(staged: number, storeKeepsFaces: boolean): string {
  if (storeKeepsFaces) return confirmLabel(staged)
  // THE ACCESSIBLE NAME CONTAINS THE VISIBLE LABEL, IT DOES NOT REPLACE IT
  // (WCAG 2.5.3, Label in Name). A name sharing no words with the text on the
  // button is unspeakable: a speech-input author says what they can see, and
  // "Install 2 on this machine" would match nothing. So the label leads and the
  // warning follows it.
  const warning = staged === 0
    ? 'this browser will not keep fonts, so confirming adds families straight to this document'
    : `this browser will not keep fonts, so confirming adds ${staged} ${staged === 1 ? 'family' : 'families'} to this document`
  return `${confirmLabel(staged)} — ${warning}`
}

/** The same fact for a sighted reader, in the footer beside the button. Empty when the store works. */
export function degradedFooterNote(storeKeepsFaces: boolean): string {
  return storeKeepsFaces ? '' : 'This browser will not keep typefaces, so these go straight into the document rather than onto this machine.'
}

/** The design's empty state, naming the query. */
export function emptyStateHeading(query: string): string {
  const typed = query.trim()
  return typed === '' ? 'No families match these filters' : `No families match “${typed}”`
}

export const emptyStateHint = 'Try a different spelling, or clear the category filters.'

/** Which page a row index falls on, and how many pages there are. Bounded by `familiesPerPage`. */
export function pageCount(matching: number): number {
  return Math.max(1, Math.ceil(matching / familiesPerPage))
}

export function pageOf(rows: ReadonlyArray<BrowserRow>, page: number): ReadonlyArray<BrowserRow> {
  const clamped = Math.max(0, Math.min(page, pageCount(rows.length) - 1))
  return rows.slice(clamped * familiesPerPage, clamped * familiesPerPage + familiesPerPage)
}

export function pageLine(page: number, matching: number): string {
  return `Page ${Math.min(page, pageCount(matching) - 1) + 1} of ${pageCount(matching)}`
}
