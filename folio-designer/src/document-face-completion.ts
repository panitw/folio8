import type { CanvasProjection } from './engine-protocol'
import { familyIsComplete, type FamilySource } from './font-index'
import type { LocalFaceHoldings } from './held-local-faces'

/**
 * COMPLETING AN OPENED DOCUMENT'S FAMILIES — THE PURE HALF
 * (spec-install-all-face-cuts, CAP-4, story 4).
 *
 * THE PROBLEM THIS MODULE'S SELECTION EXISTS FOR. A document saved before this
 * spec carries ONE face per family, and a document naming a committed-catalogue
 * family may hold only some of that family's cuts in this release's cache.
 * Either way the author presses **B** and is told the cut is not on this
 * machine — and until this story nothing ever fetched it. The open-time
 * prefetch does not: it covers the faces the engine actually PAINTED
 * (`document-face-prefetch.ts`), and an unpressed bold is never painted.
 *
 * NO REACT, NO `fetch`, NO STORAGE. Everything here is a function of its
 * arguments, exactly as `font-index.ts` is, so the selection and every sentence
 * this surface says can be measured without an engine, a store or a cache. The
 * two arms that actually fetch live in `App.tsx` beside `installFamily`, whose
 * shape they mirror.
 */

/**
 * THE FAMILIES THIS DOCUMENT NAMES, READ OFF THE PROJECTION AND NEVER OFF THE
 * `.folio` (AD-17).
 *
 * ⚠ THE DISCRIMINANT DECIDES, NOT THE STRING'S SHAPE. A chain entry carries a
 * `face` name OR an `assetKey`, exactly one of them non-empty, and only the
 * EMBEDDED entries carry a `family` at all — Go reads that name out of the
 * asset's own `font` record. A shipped-face entry names a FontSet face and has
 * nothing this designer could complete, so it is out by the same test
 * `App.tsx`'s carried-face collection uses and for the same reason: a
 * 64-character face name is a legal face name, so filtering on what the string
 * LOOKS like would cross the two namespaces on exactly the value that looks
 * like it could not.
 *
 * BOTH TIERS ARRIVE THIS WAY. A committed-catalogue family is embedded into the
 * document on first use exactly as a fetched web family is (story 2, story 3),
 * so its entry is an `assetKey` entry too and the one filter finds both. There
 * is no second reading for the local tier.
 *
 * DEDUPED, IN CHAIN ORDER. A family named by two chains is one family to
 * complete, and the order the document declares is the order the author will
 * see counted — not an alphabetisation this module would have invented.
 */
export function documentFamilies(fontChains: CanvasProjection['fontChains']): ReadonlyArray<string> {
  const named: string[] = []
  const seen = new Set<string>()
  for (const chain of fontChains) {
    for (const entry of chain.entries) {
      if (entry.assetKey.length === 0) continue
      // An embedded entry with no family name is a projection this designer
      // cannot name a family from; it is skipped rather than guessed at.
      if (entry.family === '' || seen.has(entry.family)) continue
      seen.add(entry.family)
      named.push(entry.family)
    }
  }
  return named
}

/**
 * THOSE OF THEM THERE IS STILL SOMETHING TO FETCH FOR.
 *
 * ⚠ THE PREDICATE IS `familyIsComplete`, NEVER `familyIsInstalled`, AND THE TWO
 * ARE DELIBERATELY DIFFERENT QUESTIONS (D-8). `familyIsInstalled` answers "can
 * these bytes be used" — true of every family this document is already painting
 * with, which would select nothing at all. `familyIsComplete` answers "is
 * anything left to fetch", which is the question CAP-4 asks. Story 3 separated
 * them structurally after they were briefly one expression; nothing here
 * re-fuses them and no third predicate is written.
 *
 * IT RETURNS THE `FamilySource`, NOT THE NAME, because the caller has to know
 * WHICH TIER to fetch from and what the family publishes — and `offeredFamilies`
 * has already joined the catalogue, the machine store and the index snapshot to
 * answer both. Re-deriving a tier from a name here would be a second authority
 * on the join.
 *
 * A FAMILY `offeredFamilies` DOES NOT KNOW IS LEFT OUT, SILENTLY. A document
 * can name a family withdrawn from the index snapshot and absent from this
 * machine's store: there is no source to fetch from, so there is nothing to
 * offer and nothing to say. It is not an error and not a fifth absence state.
 *
 * ⚠ AND A FAMILY WITH NO CENSUS ROW COUNTS AS SHORT. That is every family
 * installed before spec-install-all-face-cuts story 1 — face records, no census
 * — and `familyIsComplete` reads it as incomplete because the census is the
 * only authority on what a family publishes. Completing it is what ESTABLISHES
 * the census, so this is the correct reading rather than a migration defect.
 */
export function incompleteDocumentFamilies(fontChains: CanvasProjection['fontChains'], sources: ReadonlyArray<FamilySource>, holdings: LocalFaceHoldings): ReadonlyArray<FamilySource> {
  const byFamily = new Map(sources.map((source) => [source.family, source] as const))
  const short: FamilySource[] = []
  for (const family of documentFamilies(fontChains)) {
    const source = byFamily.get(family)
    if (source !== undefined && !familyIsComplete(source, holdings)) short.push(source)
  }
  return short
}

/**
 * THE SENTENCES THIS SURFACE SAYS, SPELLED HERE RATHER THAN IN THE COMPONENT —
 * the repo's per-surface model-module convention, and what lets the wording be
 * measured without rendering an App.
 *
 * ⚠ THE QUESTION AND THE REPORTING ARE DELIBERATELY SPLIT (owner, 2026-09-20).
 * The ASK is a modal, in `UnsavedChangesDialog`'s shape, because a question
 * with no answer has nowhere to wait; the PROGRESS and the OUTCOME go to the
 * literal `status-bar` footer, because the author is editing throughout and a
 * modal over an edit would block the very thing CAP-4 protects.
 *
 * ⚠ EVERY STATUS-BAR STRING IS PRICED AGAINST A MEASURED BUDGET. The bar is
 * `nowrap` and CLIPS rather than wraps, and the mono face is 6.00 px per
 * character. Measured in Chromium at 1024×768 on the design bar's own worst
 * case — a 4-digit revision, `256 fonts in template`, `256 of 256 elements
 * bound`, and the longest offline label hidden the way a standing completion
 * line hides it — a 58-character line leaves the spacer at 10.00 px and the bar
 * at exactly 1024.00. One more character still fits; two do not.
 * **59 IS THE CEILING AND 58 IS THE LONGEST THIS SURFACE CAN SAY.**
 *
 * The worst case is the three-number shortfall, found by scanning the whole
 * domain rather than eyeballing the templates: the counts are what vary and the
 * digits do not peak where a reader expects. The family count is bounded by the
 * DOCUMENT's families — `MAX_ENGINE_FONT_FAMILIES`, which
 * `document-face-completion.test.ts` READS from `engine-protocol.ts` rather
 * than typing, so if that cap moves this budget moves with it instead of
 * quietly becoming a lie. It is emphatically NOT bounded by the families an
 * author could install from; that was the first bound, and it made this comment
 * claim a 60-character worst case which measured 1026.00 px — a clip the
 * product could describe but never emit. The test says why the two populations
 * are easy to confuse.
 *
 * The numeric readout is never dropped, never abbreviated past meaning, and
 * never allowed to clip: it is the only part of these sentences that carries a
 * fact.
 */

// `1 family`, `3 families`. Derived once rather than written into four
// sentences, because a plural that is right in three places and wrong in the
// fourth is the defect this closed helper cannot have.
const familyCount = (count: number): string => `${count} ${count === 1 ? 'family' : 'families'}`

/** The modal's heading. */
export const COMPLETION_QUESTION_TITLE = "Complete this document's typefaces?"

/**
 * The modal's question. It states the two facts the author needs to answer it:
 * how much is short, and that NOTHING reaches the document — the cuts are kept
 * on this machine, exactly as an install keeps them. A question that left the
 * second fact out would be asking permission to change a file.
 */
export const completionQuestion = (count: number): string =>
  `${familyCount(count)} in this document ${count === 1 ? 'is' : 'are'} missing cuts this designer can still fetch. They are kept on this machine; nothing is added to the document.`

/**
 * The two answers. The DECLINE is the safe one — it is focused first and Escape
 * means it — because the unsafe direction here is spending an author's network
 * on a question they have not read. A decline is not remembered: the next open
 * asks again, because nothing about the document has changed and a remembered
 * "no" would silently outlive the reason for it.
 */
export const COMPLETION_DECLINE_LABEL = 'Not now'
export const COMPLETION_CONFIRM_LABEL = 'Fetch the missing cuts'

/** How far through the families the run is. Numeric, and the numerator moves. */
export const completionProgress = (attempted: number, total: number): string =>
  `Font completion: ${attempted} of ${familyCount(total)}…`

/** Every family came out complete. */
export const completionSettled = (total: number): string =>
  `Font completion: ${familyCount(total)} completed.`

/**
 * Some did not, and the shortfall is STATED rather than rounded away. A family
 * that is still short is still offered for install, so this is an observation
 * and not a dead end — which is why it needs no remedy clause the status bar
 * has no room for.
 *
 * ⚠ IT IS THE LONGEST SENTENCE THIS SURFACE CAN SAY — three numbers in one
 * line — so it is the one the budget above is really about, and `still` was
 * dropped from it for exactly that reason. Both numbers and the subject stay:
 * "2 of 3" with no noun is a fraction of nothing.
 */
export const completionShortfall = (completed: number, total: number): string =>
  `Font completion: ${completed} of ${familyCount(total)} completed, ${total - completed} short.`

/**
 * OFFLINE IS ANSWERED BEFORE ANY REQUEST IS MADE, and the sentence says so.
 * `font-source.ts` draws exactly this line for a pick: a refusal that could not
 * make a request at all is a different fact from one that made it and got
 * nothing, and merging them would send an author to check a network that is
 * fine.
 */
export const COMPLETION_OFFLINE = 'Font completion: no network, so nothing was fetched.'
