import { catalogueFaces, type CatalogueFace, type CatalogueScript } from './generated/font-catalogue'
// TYPE ONLY, AND THERE IS NO CYCLE: `held-local-faces.ts` imports the generated
// catalogue and nothing else from this module's neighbourhood. The record is
// imported rather than restated because the whole point of it is that ONE value
// carries both answers and names which is which — see `familyIsInstalled`.
import type { LocalFaceHoldings } from './held-local-faces'
import { familyIndex, familyIndexExcludedCjkFamilies, familyIndexPublishedFamilies, type IndexFamily } from './generated/font-index'
import { censusIsComplete, type FamilyCensus, type StoredFace } from './font-store'

// THE TWO TIERS, AND THE JOIN BETWEEN THEM (D-16.R.3).
//
// THE BUNDLED CATALOGUE DID NOT GO AWAY. The 21 committed faces survive Epic 16
// unchanged, as the LOCAL FACE TIER — `font-catalogue.json`, the per-face
// `LICENSE*` and `NOTICE.md` beside each binary, and the build-time gate over
// all of it. The pick GAINS A SOURCE; it does not swap one.
//
// (It is the "local face tier" and not the "derived-static tier". Measured:
// `tools/fontgen/instance_faces.py` drives a hardcoded three-entry list of
// ENGINE faces and none of the 21 designer faces is derived — every `NOTICE.md`
// says NO DERIVATION APPLIES. Calling them derived would carry an error into the
// name.)
//
// LOCAL WINS, WITH NO FETCH AT ALL — no `METADATA.pb`, no licence file, no
// bytes. The committed bytes carry a STRONGER record than any fetch can produce:
// a reviewed licence identifier, the upstream licence file committed beside the
// binary, and a provenance note. Preferring a fetch would replace a verified
// record with an unverified one.
//
// DIVERGENCE IS DELIBERATELY NOT RECONCILED. Under AD-8 and D-16.2 a face is
// identified by the SHA-256 of its bytes, so "upstream released a newer version"
// is a DIFFERENT FACE, not a newer one. There is no staleness check, no update
// prompt and no version compare in this epic; the deferral is registered in
// `deferred-work.md` with its trigger.

// AND A THIRD TIER SINCE STORY 16.2 (D-16.R.33 R1): THE MACHINE STORE.
//
// A face this designer has fetched BEFORE, kept in origin-scoped browser storage
// under the SHA-256 of its bytes. It is a source exactly as the other two are:
// picking it fetches nothing, works with the network down, and embeds the same
// three-part licence record the fetch would have produced — because the store
// keeps that record beside the bytes.
//
// IT SITS BETWEEN THE TWO, AND THE ORDER IS THE HONEST ONE. The local tier
// wins over it: those 31 faces carry a REVIEWED licence identifier, the
// upstream licence file committed beside the binary, and a build-time gate over
// all of it — a stronger record than any fetch can produce, including the fetch
// that filled this store. The store wins over the web tier for the plain reason
// that it is the same bytes without the round-trip.
//
// THE SEAM IS BUILT HERE. 16.2 predicted that Story 16.4 would add headings and
// "not reshape the union"; the second half of that prediction was WRONG and is
// corrected rather than carried. 16.4 measured this module's own documented
// order against the order it produced and found them different (see
// `offeredFamilies`), so 16.4 repaired the ORDER as well as adding the headings.
// The ARMS are untouched, and that is the part the seam was for: every
// exhaustive switch over `FamilySource` reds until a new arm is handled, which
// is what makes the hand-off a mechanism rather than a sentence in a spec.
//
// `AVAILABLE LOCALLY` IS THE DROPDOWN HEADING OVER BOTH INSTALLED ARMS — THIS
// ONE AND THE LOCAL ONE (Story 16.4, D-16.R.72). 16.2 read the name as this arm
// alone and said so here; 16.4 owned the question 16.2 delegated and settled it
// the other way, because the heading's axis is WHERE ARE THE BYTES rather than
// WHEN DID THEY ARRIVE. A face that ships inside this release and a face this
// designer fetched last week are both on this machine, both need no network,
// and both embed on a pick — so splitting them would be a provenance difference
// with no consequence at the moment of choosing, and a fourth group besides.
//
// THE 31 LOCAL-TIER FACES THEREFORE SIT UNDER THAT HEADING TOO, which is why
// the group is never empty on a fresh machine. `familyIsInstalled` below is the
// one predicate that decides it, for the control and for the browser alike.
//
// THE PANEL HEADING IN `App.tsx` IS A DIFFERENT REGION AND NO LONGER SHARES THE
// NAME: the machine store's own panel reads TYPEFACES THIS DESIGNER HAS
// DOWNLOADED, because it lists this arm only. Two differently populated regions
// may not share one name, and until 16.4 they did.
//
// WHAT IS SETTLED, IN EVERY READING: it never means "the fonts installed on
// this computer". SPEC-fonts' *"No host fonts"* Non-goal is the one clause
// D-16.1 left standing and it is untouched: the Local Font Access API is not
// used, referenced or feature-detected anywhere in this designer, and
// `src/host-font-access.test.ts` is the tripwire.
//
// AND SINCE spec-install-all-face-cuts STORY 1, AN INSTALLED TIER HOLDS A
// FAMILY'S FACE **SET** RATHER THAN ONE FACE.
//
// A pick installs every RIBBI cut the family publishes, so `Kanit` on this
// machine is up to four records — Regular, Bold, Italic, Bold Italic — and the
// browser must offer that family ONCE with each cut resolving to the record
// whose `style` matches. The union used to carry one face per arm and a fold
// downstream picked one of them; the fold is gone, not tie-broken. See
// `offeredFamilies` for what replaced it and why a better tie-break would have
// been the wrong repair.
export type FamilySource =
  /** The cuts of this family the release ships. Picking fetches nothing beyond the release's own assets. */
  | Readonly<{ tier: 'local'; family: string; faces: ReadonlyArray<CatalogueFace> }>
  /** The cuts of this family this designer fetched before and kept, with the census that says what it publishes. */
  | Readonly<{ tier: 'stored'; family: string; faces: ReadonlyArray<StoredFace>; census?: FamilyCensus }>
  /** A family from the build-time index snapshot. Picking it fetches. */
  | Readonly<{ tier: 'web'; family: string; row: IndexFamily }>

/**
 * THE CUT A CALLER MEANS WHEN IT MEANS "THE FAMILY", AND IT IS RESOLVED BY
 * `style` RATHER THAN BY POSITION.
 *
 * Everything that embeds, previews or describes a family reaches for its
 * upright Regular: the chain entry names it, the specimen is set in it, the
 * document paints with it. Reading `faces[0]` would make that a fact about
 * arrival order — the store's listing sorts by family then KEY, which is the
 * content hash, which is arbitrary — and handing the author one of four faces
 * on the strength of a digest ordering is the silent substitution the
 * content-address key exists to refuse.
 *
 * `undefined` FOR A SET WITH NO REGULAR IN IT IS A REAL ANSWER AND CALLERS SAY
 * SO IN WORDS. It cannot arise from an install — the Regular is required before
 * any byte is kept — but it CAN arise from a store whose Regular was dropped as
 * unsound between a listing and a read, and inventing a substitute there would
 * put a face nobody chose one step from a document.
 */
export const regularCutOf = <T extends Readonly<{ style: string }>>(faces: ReadonlyArray<T>): T | undefined =>
  faces.find((face) => face.style === 'Regular')

/**
 * The scripts a pickable row's face covers, read off whichever tier the row is.
 * Every tier carries them; only the field they sit in differs, and spelling the
 * discriminant here keeps the narrowing the compiler's rather than a comment's.
 *
 * IT IS THE REGULAR'S COVERAGE, NOT THE UNION OF THE SET'S. A family's cuts are
 * the same typeface at four weights and slopes; claiming the Bold covers a
 * script the Regular does not would be a fact about one cut presented as a fact
 * about the family, and the proposed fallback tail is computed from it.
 */
export function sourceScripts(source: FamilySource): ReadonlyArray<string> {
  if (source.tier === 'web') return source.row.scripts
  // WIDENED TO THE ONE SHAPE BOTH INSTALLED TIERS SHARE. A catalogue face and a
  // stored face differ in every field but the two this answer needs, and
  // narrowing the union arm by arm would be the same expression written twice.
  const cuts: ReadonlyArray<Readonly<{ style: string; scripts: ReadonlyArray<string> }>> = source.faces
  // THE REGULAR'S COVERAGE, AND A SET WITH NO REGULAR FALLS BACK TO ITS FIRST
  // CUT RATHER THAN TO NOTHING: a family whose Regular was dropped as unsound
  // still covers the scripts its remaining cuts do, and reporting no coverage
  // would propose a fallback tail for every script the family actually has.
  return regularCutOf(cuts)?.scripts ?? cuts[0]?.scripts ?? []
}

/* `indexSnapshotDate` STOOD HERE AND WENT WITH THE DISCLOSURE (Story 16.10).
   Its only reader was `familyIndexDisclosure()`, the sentence the browser's
   header no longer draws, so restating the snapshot date for a UI that never
   asks for it again is dead. `familyIndexSnapshotDate` is untouched in
   `generated/font-index.ts`, where `font-index.test.ts` still reads it. */

/* NEITHER OF THESE HAS A UI READER, AND THE COMMENT THAT CLAIMED ONE WAS WRONG.
   MEASURED at Story 16.10, over `src`, `e2e` and `scripts`:
   `indexPublishedFamilies` has ZERO readers — its own definition below and one
   prose mention at `:157`, nothing else; `indexExcludedCjkFamilies` has exactly
   one, `font-index.test.ts:62`. No component reads either.

   THEY STAY ANYWAY, and the distinction matters. `indexPublishedFamilies` was
   already orphaned BEFORE this story — 16.10 removed `familyIndexDisclosure()`,
   which never read it — so deleting it is a separate change with a separate
   justification, not this story's tidying. It is also the named carrier the
   comment at `:157` points at to keep the published count and the addable count
   from being confused for one another. `indexExcludedCjkFamilies` still has its
   one reader. Both are restatements of `generated/font-index.ts`, kept so a
   caller need not reach into the generated module by name. */
export const indexPublishedFamilies = familyIndexPublishedFamilies
export const indexExcludedCjkFamilies = familyIndexExcludedCjkFamilies

/**
 * THE JOIN KEY IS EXACT `family` STRING EQUALITY. No case-folding, no whitespace
 * normalisation, no fuzzy match.
 *
 * Measured ground for why looseness is refused rather than merely unnecessary:
 * `Inter Display` and `Source Serif 4 Display` have NO INDEX ROW AT ALL, so 2 of
 * the 21 are unjoinable under any normalisation — while `Geist` / `Geist Mono` /
 * `Geist Pixel` is exactly the neighbourhood a loose matcher gets wrong. A LOCAL
 * FACE WITH NO INDEX ROW IS LOCAL-TIER-ONLY, AND THAT IS CORRECT BEHAVIOUR, NOT
 * A DEFECT.
 *
 * ⚠ IT GROUPS RATHER THAN OVERWRITING, AND THAT IS A REPAIR RATHER THAN A
 * GENERALISATION. `new Map(catalogueFaces.map(face => [face.family, face]))` is
 * LAST-WINS: the moment the catalogue carries two cuts of one family, one of
 * them silently disappears from every reader of this map, chosen by position in
 * a generated file. It does not carry two today — every committed face is a
 * single upright Regular — so the defect is latent rather than live, and it is
 * fixed here, in this story, so that the story which SUPPLIES those cuts has
 * only data to supply.
 */
const localByFamily: ReadonlyMap<string, ReadonlyArray<CatalogueFace>> = (() => {
  const grouped = new Map<string, CatalogueFace[]>()
  for (const face of catalogueFaces) {
    const held = grouped.get(face.family)
    if (held === undefined) grouped.set(face.family, [face])
    else held.push(face)
  }
  return grouped
})()

/** The catalogue families, once each, in the order the generated catalogue writes them. */
const localFamilies: ReadonlyArray<Readonly<{ family: string; faces: ReadonlyArray<CatalogueFace> }>> = [...localByFamily].map(([family, faces]) => ({ family, faces }))

export const localTierHolds = (family: string): boolean => localByFamily.has(family)

/**
 * `axes` IS A PREDICTION, AND THIS IS THE ONLY PLACE IT IS CONSULTED.
 *
 * About a quarter of the published library ships as a single variable file
 * holding every weight at once. This product does not accept one: accepting it
 * would mean guessing which weight the author meant, and would make the same
 * template print differently on different machines. Browser-side instancing is
 * refused by ruling (D-16.5(c)) — it makes the embedded face a function of the
 * author's runtime — and folio8 has no backend, so there is no third place to do
 * it.
 *
 * Measured: all 558 axes-declaring families still list a `400` key under
 * `fonts`, so the offered-weights map cannot answer this and `axes != []` is the
 * only signal the snapshot carries. It is a good heuristic, verified on Roboto
 * and six others, AND IT IS A HEURISTIC. The authority stays Go: the engine
 * refuses a variable face if one ever reaches it, and hiding a row here does not
 * relax that by one byte.
 *
 * IT IS NOT CONSULTED FOR A FAMILY THE LOCAL TIER HOLDS. "Variable-only" is a
 * property of the BYTE SOURCE, not of the family (D-16.R.2a): `Roboto` and
 * `Inter` are committed here as byte-for-byte upstream STATICS, from
 * `googlefonts/roboto-classic` and `rsms/inter` v4.1, and appear among the
 * variable-only rows only because the `google/fonts` mirror carries VF-only
 * builds of them. A family in the local tier is offered from the local tier and
 * the index's opinion about it is never consulted.
 */
const addableFromTheWeb = (row: IndexFamily): boolean => !row.variable

/**
 * A FAMILY THAT CANNOT BE ADDED IS FILTERED OUT, NOT LISTED AND REFUSED
 * (D-16.R.2, owner).
 *
 * The refusal was priced as a long tail and it is not one: measured, 37 of the
 * 50 most popular families are variable-only — Roboto, Open Sans, Inter,
 * Montserrat, Raleway, Nunito, Oswald, Playfair Display. Listing them and
 * refusing means the most common first action in the product fails. A row the
 * author cannot act on is a row that should not be there.
 *
 * A HIDDEN ROW IS A PRESENTATION CHOICE, NEVER A GUARD. The engine's refusal
 * stays, for anything that reaches it by any other door.
 */
export const webFamilies: ReadonlyArray<IndexFamily> = familyIndex.filter((row) => addableFromTheWeb(row) && !localTierHolds(row.family))

/**
 * THE COUNT THE BROWSER REPORTS IS THE ADDABLE COUNT, and the caller is expected
 * to say which it is. It is not 1,946: that is how many families the source
 * published on the snapshot date, and it is carried separately
 * (`indexPublishedFamilies`) so the two can never be confused for each other.
 *
 * ⚠ IT COUNTS FAMILIES, AND THE LOCAL HALF USED TO COUNT ROWS. It was
 * `webFamilies.length + catalogueFaces.length`, which was the same number only
 * while the catalogue held one upright Regular per family. Since
 * spec-install-all-face-cuts story 3 a family declares up to four rows, so that
 * expression would report four Inters in a toolbar line — `N of 1,853 families`
 * — that offers each family exactly ONCE (CAP-5). `localFamilies` is the same
 * grouping `offeredFamilies` draws its local rows from, so the count and the
 * list cannot disagree.
 *
 * ROBOTO'S CUTS ARE NOT IN THE CATALOGUE AND THIS COUNT IS UNAFFECTED BY THAT.
 * `Roboto Bold`, `Roboto Italic` and `Roboto Bold Italic` ship as hardcoded
 * core faces rather than catalogue rows (see `scripts/build-wasm.mjs`), so the
 * family contributes one row here exactly as the four-cut families contribute
 * one group — a family is counted once whichever half of the build its cuts
 * came from.
 */
export const addableFamilyCount = webFamilies.length + localFamilies.length

/**
 * ONE ORDERED LIST OF EVERY FAMILY THE AUTHOR MAY PICK, local tier first, then
 * the faces this machine already holds, then the rest of the snapshot.
 *
 * THAT SENTENCE WAS FALSE FROM 16.2 UNTIL STORY 16.4, AND IT IS WORTH SAYING SO
 * HERE RATHER THAN QUIETLY FIXING IT. The stored rows were pushed inside the
 * snapshot loop, so they arrived at their WEB positions and the installed rows
 * came out in four alternating runs instead of one. The comment was written
 * above the function and never measured over it. `font-index.test.ts` now
 * measures the run structure on every run, because a comment is not a
 * measurement.
 *
 * THE ORDER IS PART OF THE CONTRACT, NOT A PRESENTATION DETAIL. The family
 * control groups this list under headings and caps only its tail; a caller that
 * trusts the heading over the order draws a heading it cannot fill. So the
 * guarantee is stated in one line: EVERY LOCAL AND STORED ROW COMES BEFORE
 * EVERY WEB ROW — exactly two runs of TIER, never four.
 *
 * IT IS A CLAIM ABOUT TIER, AND SINCE spec-deferred-offline-cache STORY 2 THAT
 * IS NO LONGER THE SAME CLAIM AS ONE ABOUT `familyIsInstalled`. The catalogue
 * is deferred, so a `local` row may be a family this browser has not fetched
 * and `familyIsInstalled` answers false for it — INTERLEAVED among local rows
 * that are held, because this function does not know or consult the held set.
 * That is correct: this module joins three lists and the family control is what
 * FILTERS on installedness, dropping the unheld rows before it groups. Stating
 * the invariant over `familyIsInstalled` would have been false the moment the
 * catalogue was deferred, and only an all-held input would have hidden it.
 *
 * Local first is the honest order rather than a preference: those rows need no
 * network, and the join above has already removed their web duplicates, so a
 * family present in both appears once, from the tier that can serve it offline.
 * The stored tier extends exactly that reasoning to a face the author fetched
 * last week.
 *
 * A STORED FAMILY REPLACES ITS WEB ROW; IT DOES NOT SIT BESIDE IT. One family,
 * one row, from the cheapest tier that can serve it — which is the same rule
 * `webFamilies` already applies to the local tier. A row offered twice, once as
 * "already here" and once as "will be downloaded", would make the author choose
 * between two spellings of one thing.
 *
 * A STORED FAMILY WITH NO WEB ROW IS STILL OFFERED. The index is a build-time
 * snapshot that ages, so a family fetched under one release can be withdrawn or
 * renamed upstream before the next. Its bytes are here, its licence record is
 * here, and refusing to offer it because a dated list no longer mentions it
 * would be the store failing at the one job it exists for.
 *
 * THE STORE'S LISTING IS PASSED IN, NEVER READ FROM HERE. The store's reads are
 * asynchronous and this function is called on every keystroke of a combobox.
 * The caller owns the read, its lifetime and its degradation; this module
 * remains a pure join over three inputs, which is also what keeps it testable
 * without a database.
 */
/**
 * A FAMILY'S STORED FACES ARE ALL OF THEM — THE FOLD THAT PICKED ONE IS GONE.
 *
 * WHAT USED TO STAND HERE. `mostRecentlyFetched` collapsed a family's stored
 * records to a single `StoredFace`, newest `fetchedAt` first and the
 * lexicographically smaller key breaking a tie. It was a careful rule and it
 * was answering the wrong question. `fetchedAt` is day-granular, so the four
 * cuts of one family installed in one session ALL TIE, and the tie-break then
 * handed the author whichever cut happened to hash lowest — an arbitrary face,
 * not the Regular, chosen by a digest.
 *
 * WHY THE TIE-BREAK WAS NOT WORTH REPAIRING. The fold was wrong in KIND. It
 * existed because `FamilySource` held ONE face per family and something had to
 * choose it; now the union holds the SET, so there is nothing to choose. A
 * repaired tie-break — prefer `style === 'Regular'`, say — would have been a
 * second authority on cut resolution sitting beside `regularCutOf`, agreeing
 * with it today and free to disagree tomorrow.
 *
 * ONE FAMILY IS STILL ONE ROW. That was the fold's real job and it is kept:
 * the browser offers a family once, whatever its face count, and each cut
 * resolves out of the set by its own `style`.
 *
 * TWO RECORDS OF THE SAME CUT ARE BOTH KEPT, AND THAT IS HONEST. Upstream can
 * re-cut a face between two fetches; under AD-8 those are DIFFERENT FACES, not
 * versions of one. `regularCutOf` takes the first match in the store's own
 * stable family-then-key order, which is the same face its listing shows first
 * — one answer to one question rather than two.
 */
export function offeredFamilies(query: string, storedListing: ReadonlyArray<StoredFace> = [], censusListing: ReadonlyArray<FamilyCensus> = []): ReadonlyArray<FamilySource> {
  const needle = query.trim().toLowerCase()
  const hit = (family: string) => needle === '' || family.toLowerCase().includes(needle)
  // THE LOCAL TIER IS NOT DISPLACED BY THE STORE. Its record is the stronger
  // one (see the note on `FamilySource`), and it needs no network either, so
  // there is nothing to win by preferring a fetched copy of the same family.
  const censusByFamily = new Map(censusListing.map((census) => [census.family, census]))
  const storedByFamily = new Map<string, StoredFace[]>()
  for (const record of storedListing) {
    if (localTierHolds(record.family)) continue
    const held = storedByFamily.get(record.family)
    if (held === undefined) storedByFamily.set(record.family, [record])
    else held.push(record)
  }
  const storedSource = (family: string, faces: ReadonlyArray<StoredFace>): FamilySource => {
    const census = censusByFamily.get(family)
    return census === undefined ? { tier: 'stored', family, faces } : { tier: 'stored', family, faces, census }
  }
  const local: ReadonlyArray<FamilySource> = localFamilies.filter((entry) => hit(entry.family)).map((entry) => ({ tier: 'local', family: entry.family, faces: entry.faces }))
  // A STORED ROW IS COLLECTED, NOT PUSHED WHERE ITS WEB ROW STOOD (Story 16.4).
  //
  // The snapshot loop is walked for its MEMBERSHIP — which families the store
  // can serve instead of the network — and never for its POSITION. It used to
  // contribute both, so a family the author had already downloaded took the
  // index rank of the row it replaced: measured, one planted stored face landed
  // at offset 900 of 1304, four alternation runs deep, under a heading that
  // says the bytes are already here. That is the defect this split repairs.
  const stored: FamilySource[] = []
  const web: FamilySource[] = []
  const offeredFromStore = new Set<string>()
  for (const row of webFamilies) {
    if (!hit(row.family)) continue
    const records = storedByFamily.get(row.family)
    if (records === undefined) { web.push({ tier: 'web', family: row.family, row }); continue }
    offeredFromStore.add(row.family)
    stored.push(storedSource(row.family, records))
  }
  // A stored family the snapshot no longer lists follows the ones it does, so
  // the two halves of the store stay adjacent and the run stays contiguous.
  const orphanedStored: ReadonlyArray<FamilySource> = [...storedByFamily]
    .filter(([family]) => !offeredFromStore.has(family) && hit(family))
    .map(([family, records]) => storedSource(family, records))
  return [...local, ...stored, ...orphanedStored, ...web]
}

/**
 * WHETHER THIS MACHINE ALREADY HOLDS THE FACE — THE ONE DEFINITION OF
 * "INSTALLED", read by the family control's fork and by the browser's row state
 * so the two cannot disagree (Story 16.5).
 *
 * THE LINE IS, AND ALWAYS WAS, "CAN THESE BYTES BE HAD WITH NO NETWORK". What
 * changed in spec-deferred-offline-cache story 2 is that the LOCAL TIER STOPPED
 * ANSWERING THAT QUESTION BY ITS TIER ALONE. It used to: the release precached
 * all 80 assets (156 since spec-install-all-face-cuts story 3), so a catalogue
 * face shipping inside the release was, by the time anything could ask, on this
 * machine. The worker now precaches the core
 * tier only and the 31 catalogue faces are deferred, so a family can ship in
 * this release and still not be here — and `source.tier !== 'web'` would have
 * gone on claiming it was, which is exactly the untruth CAP-4 names.
 *
 * SO THE HOLDINGS ARE A REQUIRED ARGUMENT, NOT AN OPTIONAL ONE. A default of
 * "assume held" would let a caller that has not done the read quietly get the
 * old, wrong answer; making every call site pass it is what forced both
 * surfaces — the family control's AVAILABLE LOCALLY group and the font
 * browser's row state — to be looked at together. `readLocalFaceHoldings` in
 * `held-local-faces.ts` is the read; this module stays a pure function of its
 * inputs and opens no storage of its own.
 *
 * ⚠ IT READS `usable`, AND `familyIsComplete` READS `complete` — ONE VALUE, TWO
 * FIELDS, AND THE FIELD IS THE WHOLE POINT (story 3, finding F1). Both arms
 * were `heldLocalFamilies.has(source.family)` over the SAME all-cuts set, so
 * the two predicates were the identical expression and the doc comment below
 * described a distinction the code did not make. A family holding its Regular
 * alone therefore read NOT INSTALLED and dropped out of AVAILABLE LOCALLY —
 * and merely BROWSING reaches that state, because `browserSpecimenBytes`
 * caches each local row's Regular to draw its specimen and nothing else. A
 * record with two named fields is what stops the two sets being interchangeable
 * to the compiler a second time.
 *
 * THE STORED TIER IS UNCONDITIONAL AND THE WEB TIER IS STILL NEVER INSTALLED.
 * A stored face is in the machine store by definition — the listing it came
 * from IS the evidence — and a web row has bytes nowhere on this machine at all.
 *
 * ⚠ THIS PREDICATE ANSWERS "CAN THESE BYTES BE USED", NOT "IS THIS FAMILY
 * COMPLETE", AND THE TWO WERE BRIEFLY FUSED (D-8, orchestrator direction 3).
 * The first cut of spec-install-all-face-cuts story 1 made the stored arm
 * answer D-5's completeness question, which had a consequence nobody had
 * ruled on: a family installed before that story carries no census, so it read
 * NOT INSTALLED, so the family control's AVAILABLE LOCALLY group dropped it —
 * and offline that makes a font already sitting on the machine UNUSABLE. D-4
 * moved incomplete families back into the installable group; it never put them
 * out of reach. Completeness governs whether a family is RE-OFFERED FOR
 * INSTALL, and that is `familyIsComplete` below; it does not govern whether
 * what is here can be used.
 *
 * IT IS STILL DELIBERATELY NOT A FOURTH TIER. A tier says where a face's BYTES
 * COME FROM; whether this browser has yet fetched them is a different axis, and
 * it belongs in the argument rather than in the union.
 */
export const familyIsInstalled = (source: FamilySource, holdings: LocalFaceHoldings): boolean => {
  switch (source.tier) {
    // USABLE, NOT COMPLETE: the family's Regular is in this release's cache, so
    // it can be applied to a component and painted with the network down,
    // whatever cuts it is still short of. Those are `familyIsComplete`'s
    // business and the font browser's, not this control's.
    case 'local': return holdings.usable.has(source.family)
    // WRITTEN AS THE FACE SET RATHER THAN AS A BARE `true`, because that is the
    // claim being made: this family has faces on this machine. `offeredFamilies`
    // never builds this arm empty, so it reads `true` for every row the union
    // actually produces — and a caller that hand-built one with no faces gets
    // the honest answer instead of an inherited constant.
    case 'stored': return source.faces.length > 0
    case 'web': return false
    default: {
      const unhandled: never = source
      throw new Error(`a FamilySource tier nothing describes reached the installed predicate: ${String((unhandled as FamilySource).tier)}`)
    }
  }
}

/**
 * WHETHER THE FAMILY HOLDS EVERYTHING IT PUBLISHES — D-5's PREDICATE, AND THE
 * ONE THAT DECIDES WHETHER IT IS OFFERED FOR INSTALL AGAIN.
 *
 * INSTALLED AND COMPLETE ARE DIFFERENT QUESTIONS AND HAVE DIFFERENT READERS
 * (D-8). The family control asks `familyIsInstalled` — "can I use this now" —
 * and the font browser's row state asks this one — "is there anything left to
 * fetch". A family holding only the Regular of a family that publishes a Bold
 * answers YES to the first and NO to the second, which is exactly right: it is
 * usable today and there is more of it to get.
 *
 * A CUT IS SETTLED BY A **PERMANENT** REFUSAL AND BY NOTHING ELSE (D-2/D-5,
 * amended). A transient one — a stall, an offline minute, a 5xx — leaves the
 * family incomplete so the cut is fetched again on a later pick. That is what
 * makes `font-source.ts`'s stall sentence true for a cut that is not the base.
 *
 * A FAMILY WITH NO CENSUS READS INCOMPLETE, NOT COMPLETE. That is every family
 * installed before this story, and it is the honest answer rather than a
 * migration: the census is the only authority on what a family publishes, so
 * with none there is no cut this designer can claim to have. It is still
 * USABLE — see above — and picking it again writes a census and fetches only
 * what is missing.
 *
 * A `local` FAMILY NEEDS NO CENSUS AT ALL. The catalogue is the authority on
 * what the local tier publishes and it publishes exactly what it ships, so a
 * catalogue family that holds every cut it declares is complete by
 * construction — and `complete` is exactly that read.
 */
export const familyIsComplete = (source: FamilySource, holdings: LocalFaceHoldings): boolean => {
  switch (source.tier) {
    // COMPLETE, NOT USABLE: every cut the catalogue declares for this family is
    // cached. A family holding its Regular alone is usable and NOT complete,
    // and must keep being offered here so its remaining cuts are reachable.
    case 'local': return holdings.complete.has(source.family)
    case 'stored': {
      if (source.census === undefined) return false
      return censusIsComplete(source.census, new Set(source.faces.map((face) => face.style)))
    }
    case 'web': return false
    default: {
      const unhandled: never = source
      throw new Error(`a FamilySource tier nothing describes reached the completeness predicate: ${String((unhandled as FamilySource).tier)}`)
    }
  }
}

/**
 * THE TIER A ROW IS OFFERED FROM, IN THE AUTHOR'S OWN TERMS — one exhaustive
 * switch over `FamilySource`, so the union cannot gain an arm that nothing
 * describes.
 *
 * This is the seam D-16.R.33 R1 asked to be built here rather than left to
 * Story 16.4: adding a fourth tier without handling it stops compiling at the
 * `never`, which is what makes the hand-off enforceable.
 *
 * STORY 16.5 REWROTE WHAT EVERY ARM SAYS, BECAUSE PICKING NO LONGER MEANS ONE
 * THING FOR ALL THREE. A row this machine already holds is USED when it is
 * picked — the face is embedded and the property committed. A row it does not
 * hold is INSTALLED when it is picked, and nothing reaches the document until
 * something in the template is actually set in it. The note is the only place
 * an author is told which of those two a row will do, so it says so rather than
 * describing the tier for its own sake.
 */
export function familySourceNote(source: FamilySource): string {
  switch (source.tier) {
    case 'local': return ' — use it, already on this machine'
    case 'stored': return ' — use it, already downloaded to this machine'
    case 'web': return ' — install on this machine'
    default: {
      const unhandled: never = source
      // NO `JSON.stringify` HERE. `engine-ownership-contract.test.ts` keeps
      // JSON out of every module but the protocol envelopes and the three
      // command factories, and it is right to: this module joins three lists
      // and has no business serialising anything.
      throw new Error(`a FamilySource tier nothing describes reached the family control: ${String((unhandled as FamilySource).tier)}`)
    }
  }
}

/**
 * THE SNAPSHOT ROW FOR A FAMILY, WHATEVER TIER OFFERS IT (Story 16.3).
 *
 * `webFamilies` above is the ADDABLE-FROM-THE-WEB list: it has already dropped
 * the variable-only rows and every family the local tier holds. That is the
 * right list to OFFER from and the wrong one to DESCRIBE from, because the
 * facts a browser prints beside a family — its category, how popular it is,
 * which scripts it covers — are properties of the family and are carried by the
 * snapshot for local-tier families too.
 *
 * SO THIS READS THE WHOLE `familyIndex`, INCLUDING ROWS `webFamilies` REMOVED,
 * AND THAT IS NOT A HOLE IN THE FILTER. A row reachable here can never become a
 * pick: `offeredFamilies` is the only source of a `FamilySource`, nothing here
 * produces one, and a local-tier family is offered from the local tier whatever
 * the snapshot's `variable` flag says about the mirror's build of it (D-16.R.2a).
 * This function answers "what does the snapshot say about this name", never
 * "may the author add it".
 *
 * A FAMILY WITH NO ROW IS `undefined` AND THE CALLER MUST SAY SO IN WORDS. Two
 * of the local-tier families have no index row at all — the join note above
 * names them — so "no category" is a real and permanent state, not a loading
 * one, and printing a guessed category for those two would be the browser
 * inventing a fact about a typeface.
 */
const indexByFamily: ReadonlyMap<string, IndexFamily> = new Map(familyIndex.map((row) => [row.family, row]))

export function indexRowFor(family: string): IndexFamily | undefined {
  return indexByFamily.get(family)
}

/**
 * THE POPULATION A CHIP VOCABULARY MUST BE DERIVED FROM IS THE ONE THE CHIPS
 * FILTER, AND THAT IS NOT `familyIndex`.
 *
 * `familyIndex` is 1,811 rows. The browser offers 1,273 web rows plus the 31 the
 * local tier holds: `addableFromTheWeb` alone drops 537 variable-only rows. A
 * vocabulary read off the wider list is a vocabulary that can name a value no
 * offered family carries — which is a chip that empties the list every time it
 * is pressed, the exact false affordance the derivation exists to prevent.
 *
 * MEASURED TODAY THE TWO AGREE EXACTLY: no category and no script is present in
 * the full index and absent from the offered population. THAT IS A MEASUREMENT
 * AND NOT A GUARANTEE — it is a coincidence of this snapshot's data, and one
 * release in which a category appears only among variable-only families would
 * reintroduce the dead chip. Deriving from the offered rows costs nothing and
 * cannot have that failure.
 *
 * ONE ROW PER LOCAL FAMILY, NOT PER LOCAL FACE. `catalogueFaces` is up to four
 * rows for one family since spec-install-all-face-cuts story 3; both readers
 * below collapse to a `Set`, so the duplicates would not change either
 * vocabulary — but a list called "the offered rows" that carries a family four
 * times is a list the next reader will count.
 */
const offeredIndexRows: ReadonlyArray<IndexFamily> = [
  ...webFamilies,
  ...localFamilies.map((entry) => indexByFamily.get(entry.family)).filter((row): row is IndexFamily => row !== undefined),
]

/**
 * THE CATEGORY VOCABULARY, READ OFF THE OFFERED POPULATION RATHER THAN TYPED OUT.
 *
 * `Font Browser.dc.html` draws four category chips — Sans Serif, Serif, Display,
 * Monospace — over fourteen placeholder families. The real vocabulary has FIVE
 * categories, and the fifth (Handwriting) is the third largest of them. A hand
 * copy of the mockup's four would hide 259 offered families behind chips that
 * look exhaustive — which is the failure mode a derived vocabulary cannot have.
 *
 * THE DENOMINATOR IS THE OFFERED POPULATION, NOT THE INDEX. 337 is Handwriting's
 * count over the whole snapshot; the browser offers 259 of them, and 259 is the
 * number a chip in this control actually reveals.
 */
export const indexCategories: ReadonlyArray<string> = [...new Set(offeredIndexRows.map((row) => row.category))].sort()

/**
 * AND THE WRITING-SYSTEM VOCABULARY, ON THE SAME GROUND.
 *
 * The mockup also draws Cyrillic and Greek chips. This snapshot records no such
 * coverage for any family — `CatalogueScript` is `latin`, `thai` and `cjk`, and
 * CJK is excluded from the snapshot by SPEC-fonts' own non-goal — so those two
 * chips would filter every result away every time they were pressed. A control
 * that can only ever empty the list is a false affordance, so the chips are the
 * scripts the snapshot and the local tier actually name.
 */
export const indexScripts: ReadonlyArray<CatalogueScript> = [...new Set([...offeredIndexRows.flatMap((row) => row.scripts), ...catalogueFaces.flatMap((face) => face.scripts)])].sort()
