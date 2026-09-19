import { catalogueFaces, type CatalogueFace, type CatalogueScript } from './generated/font-catalogue'
import { familyIndex, familyIndexExcludedCjkFamilies, familyIndexPublishedFamilies, type IndexFamily } from './generated/font-index'
import type { StoredFace } from './font-store'

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
export type FamilySource =
  /** A face this machine already holds. Picking it fetches nothing. */
  | Readonly<{ tier: 'local'; family: string; face: CatalogueFace }>
  /** A face this designer fetched before and kept. Picking it fetches nothing. */
  | Readonly<{ tier: 'stored'; family: string; record: StoredFace }>
  /** A family from the build-time index snapshot. Picking it fetches. */
  | Readonly<{ tier: 'web'; family: string; row: IndexFamily }>

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
 */
const localByFamily: ReadonlyMap<string, CatalogueFace> = new Map(catalogueFaces.map((face) => [face.family, face]))

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
 */
export const addableFamilyCount = webFamilies.length + catalogueFaces.length

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
 * guarantee is stated in one line: EVERY ROW SATISFYING `familyIsInstalled`
 * COMES BEFORE EVERY ROW THAT DOES NOT — exactly two runs, never four.
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
 * WHICH OF TWO STORED FACES OF ONE FAMILY IS OFFERED — A RULE, NOT AN ACCIDENT.
 *
 * The store is keyed by the SHA-256 of the bytes, so ONE FAMILY CAN HONESTLY
 * HAVE MORE THAN ONE ENTRY: upstream re-cut the face between two fetches, or
 * the author has a Regular and an Italic of it. Under AD-8 those are DIFFERENT
 * FACES, not versions of one, and this function is `offeredFamilies` — one row
 * per family — so exactly one of them is offered.
 *
 * IT USED TO BE WHICHEVER ONE CAME LAST IN `list()` ORDER, which is the store's
 * family-then-key sort — that is, the hash, that is, arbitrary. A menu that
 * silently hands the author one of two faces on the strength of a digest
 * ordering is the "silent substitution" the contract forbids in the very clause
 * that makes the key a content hash.
 *
 * THE RULE IS: THE MOST RECENTLY FETCHED WINS. `fetchedAt` is `YYYY-MM-DD`, so
 * a plain string comparison is chronological. It is the right default because
 * the newer entry is the one the author's last pick of that family actually
 * produced, so it is the one they last saw work.
 *
 * TIES GO TO THE LEXICOGRAPHICALLY SMALLER KEY. Two faces fetched on the same
 * day are common (a Regular and an Italic within one session), and the tie must
 * break on something stable rather than on arrival order. The smaller key is
 * also the one `font-store.ts`'s `list()` sorts FIRST within a family, so what
 * the menu offers and what the listing shows first are the same face rather
 * than two answers to one question.
 *
 * NEITHER OF THESE IS A DROPDOWN GROUP, AND 16.2 DOES NOT BUILD ONE. Offering
 * both styles as separate rows is Story 16.4's, which owns the grouping; this
 * story owes only a choice that is deterministic and stated.
 */
function mostRecentlyFetched(left: StoredFace, right: StoredFace): StoredFace {
  if (left.fetchedAt !== right.fetchedAt) return left.fetchedAt > right.fetchedAt ? left : right
  return left.key <= right.key ? left : right
}

export function offeredFamilies(query: string, storedListing: ReadonlyArray<StoredFace> = []): ReadonlyArray<FamilySource> {
  const needle = query.trim().toLowerCase()
  const hit = (family: string) => needle === '' || family.toLowerCase().includes(needle)
  // THE LOCAL TIER IS NOT DISPLACED BY THE STORE. Its record is the stronger
  // one (see the note on `FamilySource`), and it needs no network either, so
  // there is nothing to win by preferring a fetched copy of the same family.
  //
  // AND WHERE THE STORE HOLDS TWO FACES OF ONE FAMILY, WHICH ONE IS OFFERED IS
  // CHOSEN AND WRITTEN DOWN — see `mostRecentlyFetched`. It used to be whichever
  // `list()` happened to return last, which is the silent substitution the
  // content-address key exists to refuse.
  const storedByFamily = new Map<string, StoredFace>()
  for (const record of storedListing) {
    if (localTierHolds(record.family)) continue
    const held = storedByFamily.get(record.family)
    storedByFamily.set(record.family, held === undefined ? record : mostRecentlyFetched(held, record))
  }
  const local: ReadonlyArray<FamilySource> = catalogueFaces.filter((face) => hit(face.family)).map((face) => ({ tier: 'local', family: face.family, face }))
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
    const record = storedByFamily.get(row.family)
    if (record === undefined) { web.push({ tier: 'web', family: row.family, row }); continue }
    offeredFromStore.add(row.family)
    stored.push({ tier: 'stored', family: row.family, record })
  }
  // A stored family the snapshot no longer lists follows the ones it does, so
  // the two halves of the store stay adjacent and the run stays contiguous.
  const orphanedStored: ReadonlyArray<FamilySource> = [...storedByFamily.values()]
    .filter((record) => !offeredFromStore.has(record.family) && hit(record.family))
    .map((record) => ({ tier: 'stored', family: record.family, record }))
  return [...local, ...stored, ...orphanedStored, ...web]
}

/**
 * WHETHER THIS MACHINE ALREADY HOLDS THE FACE — THE ONE DEFINITION OF
 * "INSTALLED", read by the family control's fork and by the browser's row state
 * so the two cannot disagree (Story 16.5).
 *
 * TWO TIERS ARE INSTALLED AND ONE IS NOT, AND THE LINE IS "CAN THESE BYTES BE
 * HAD WITH NO NETWORK". The local tier ships inside the release behind the
 * service worker; the stored tier is what this designer fetched and kept. Only
 * a `web` row needs a download, so only a `web` row has anything to install.
 *
 * IT IS DELIBERATELY NOT A FOURTH TIER. A tier says where a face's BYTES come
 * from, and install/embed separation did not add a byte source — it split what a
 * pick does. Reinterpreting `'stored'` to mean "installed" would have compiled
 * everywhere and changed nothing, which is exactly why the new state is stated
 * here and in `rowState` instead.
 */
export const familyIsInstalled = (source: FamilySource): boolean => source.tier !== 'web'

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
 */
const offeredIndexRows: ReadonlyArray<IndexFamily> = [
  ...webFamilies,
  ...catalogueFaces.map((face) => indexByFamily.get(face.family)).filter((row): row is IndexFamily => row !== undefined),
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
