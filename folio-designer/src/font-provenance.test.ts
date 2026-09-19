import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { catalogueFaces } from './generated/font-catalogue'
import { fontHostDeclarations, fontsRepositoryHost, webFaceSource } from './font-source'
import { assertProvenanceShape } from './test/provenance-shape'

// STORY 16.1a — THE TRIPWIRE UNDER `source`, ON BOTH TIERS (D-16.R.13, DW-160).
//
// `source` is one of the twelve wire fields a `.folio` records for an embedded
// face, and until this story its two writers disagreed in KIND:
//
//   committed tier  `folio-designer/public/fonts/<dir>/<file> — see that
//                    directory's NOTICE.md …`   (a path into THIS repository,
//                    naming a file the recipient of a `.folio` does not have)
//   fetched tier    `<the declared fetch host>/google/fonts/main/<path>`
//                    (a bare MUTABLE BRANCH URL)
//
// D-16.R.13's verdict: `source` carries the **upstream project**, the **path
// within it**, and the **fetch date** — and three things it must never carry.
//
//   - **No resolvable-looking URL.** A string shaped like an address is a
//     promise of fetchability, and a promise that decays is worse than none: a
//     dead link in a year reads as "this provenance is broken" when the
//     provenance is intact.
//   - **No branch name.** `main` does not identify bytes.
//   - **No SHA-256.** The face is stored under its digest as its asset key, and
//     the key travels with the record when an asset is lifted into another
//     document. Restating it here would create two authorities on one fact that
//     can disagree.
//
// AND THIS FILE EXISTS BECAUSE THE CONVENTION ALONE WILL NOT HOLD. The ruling
// asked for the tripwire by name. Both tiers are asserted here, together, on
// purpose: the defect being guarded against is not "a bad string" but "the two
// writers drifting apart", which no test looking at one tier can see.

const here = path.dirname(fileURLToPath(import.meta.url))

// THE PREDICATE ITSELF LIVES IN `src/test/provenance-shape.ts`, shared with
// `font-source.test.ts`, which asserts the same shape over the FETCHED tier's
// real write path. Shared rather than copied on purpose: the defect this
// tripwire guards is the two tiers drifting apart, and two copies of the rule
// can drift exactly as the two writers did.

describe('`source` names provenance and never a retrieval path', () => {
  // NON-VACUITY FIRST. Every assertion below is inside a loop over the
  // generated catalogue, and a module that emitted nothing would satisfy them
  // all in silence — the exact shape this suite's other guards are written
  // against.
  //
  // AND IT IS THE POPULATION FLOOR — ONE OF FOUR, AND ALL FOUR MOVE TOGETHER.
  // The other three are `src/font-catalogue.test.ts` ("declares at least twenty
  // NEW families"), `src/font-index.test.ts` ("is the whole bundled catalogue,
  // unchanged") and `src/font-name-table.test.ts` ("reads a copyright out of
  // every committed catalogue face"). Raised 20 -> 31 by Story 16.1a.
  // D-16.R.18: "a floor that exists in three files is three floors, and a
  // ruling that says *the floor* has already lost track of one of them" — this
  // site is the fourth, added by the same story, and is named at the other
  // three so the count cannot quietly go stale again.
  it('is asserted over the whole committed tier, not over a sample of it', () => {
    expect(catalogueFaces.length, 'the generated catalogue is empty, so every committed-tier assertion below is vacuous; this is one of FOUR population floors and all four move together').toBeGreaterThanOrEqual(31)
  })

  it('carries no scheme and no host on the committed tier', () => {
    for (const face of catalogueFaces) assertProvenanceShape(expect, 'committed tier', face.family, face.source)
  })

  // AND IT NO LONGER POINTS AT A FILE THAT DOES NOT TRAVEL. The old string named
  // `NOTICE.md` in this repository; a `.folio` reaches its recipient without it.
  it('inlines the pinned upstream release rather than pointing at this repository', () => {
    for (const face of catalogueFaces) {
      expect(face.source, `${face.family}: \`source\` points at a NOTICE.md the recipient of a .folio does not have`).not.toContain('NOTICE.md')
      expect(face.source, `${face.family}: \`source\` points into this repository's own tree`).not.toContain('folio-designer/')
      expect(face.source, `${face.family}: \`source\` names no pinned upstream release`).toMatch(/^[^\s@]+@[^\s@]+ — \S+, fetched \d{4}-\d{2}-\d{2}$/)
    }
  })

  it('carries no scheme and no host on the fetched tier', () => {
    assertProvenanceShape(expect, 'fetched tier', 'webFaceSource, called directly', webFaceSource('ofl/kanit/Kanit-Regular.ttf', '2026-09-03'))
    // The path within the project survives, because it is the half of the
    // record that says WHICH face — dropping it would make the field name a
    // project and nothing else.
    expect(webFaceSource('ofl/kanit/Kanit-Regular.ttf', '2026-09-03')).toContain('ofl/kanit/Kanit-Regular.ttf')
    // AND THE REAL WRITE PATH IS ASSERTED ELSEWHERE, deliberately. This case
    // calls `webFaceSource` with an EXPLICIT date, so it cannot observe
    // `fetchWebFamily`'s default `today` expression — measured: deleting the
    // `.slice(0, 10)` from that default publishes a full ISO timestamp on every
    // fetched pick and leaves this file entirely green. `font-source.test.ts`
    // drives `fetchWebFamily` with NO `today` argument against the kanit stub
    // and applies this same predicate to the outcome, which is the case that
    // reds under that mutation.
  })

  // THE HOST CONSTANTS ARE THE THING THE FETCHED TIER USED TO INTERPOLATE, so
  // the tripwire is stated against them by identity rather than by spelling: a
  // rename in `fontHostDeclarations` cannot walk out from under this.
  it('never interpolates a declared font host into the field', () => {
    expect(fontHostDeclarations.length, 'no host is declared, so the check below compares against nothing').toBeGreaterThan(0)
    const written = [...catalogueFaces.map((face) => face.source), webFaceSource('ofl/kanit/Kanit-Regular.ttf', '2026-09-03')]
    for (const { host } of fontHostDeclarations) {
      for (const value of written) expect(value, `\`source\` names the fetch host ${host}`).not.toContain(host)
    }
  })

  // AND THE ONE WRITER ON EACH TIER IS THE ONE THIS FILE CHECKS. Both
  // assertions above read a FUNCTION; a second assignment site writing its own
  // string would be invisible to them, which is how the branch URL survived a
  // suite that already had opinions about this field.
  //
  // BOTH TIERS ARE SCRAPED THE SAME WAY, AND THAT SYMMETRY IS THE POINT — IT IS
  // FINDING F7, WHOLE. The committed side used to be a bare `toContain`, which
  // proves only that the writer EXISTS and cannot see a SECOND `source:`
  // emission added anywhere else in the file — precisely the property this
  // test's own NAME claims. But making only that side exhaustive RELOCATED the
  // asymmetry instead of discharging it: the fetched side was still scraped
  // with `/^\s*source: (.+),$/gm`, LINE-ANCHORED and comma-terminated, and an
  // assertion permissive on one tier and exhaustive on the other IS the two
  // tiers drifting apart, living inside the guard against the two tiers
  // drifting apart. It does not matter which tier is the permissive one.
  //
  // WHY THE ANCHORED VERSION WAS INSUFFICIENT, MEASURED. The same second writer
  // — emitting the exact original defect, a bare `main` branch URL — was
  // appended to `font-source.ts` in two spellings:
  //
  //   `source:` on its own line          -> RED
  //   the identical code, reflowed to ONE line -> GREEN, 46 passed / 0 failed
  //
  // A FORMATTER DECIDED WHETHER THE GUARD COULD SEE THE WRITER. That is the
  // same defect the negative-case suite below exists for, wearing a different
  // mechanism: an assertion whose reach is decided by something other than the
  // property it claims to hold. And note the DIRECTION — the registered
  // deferral about reformatting records the regex going RED on a line wrap,
  // which is loud and gets fixed; this is a reflow making the guard SILENTLY
  // ADMIT a second writer. A deferral pointing one way is not coverage for the
  // other.
  //
  // ONE SCRAPE SHAPE, ONE EXCLUSION RULE, ONE GUARANTEE. `\bsource:` unanchored,
  // captured to the next comma, semicolon or newline, over both files, with
  // full-line `//` comments dropped first — the one stated exclusion, because
  // prose about `source:` is not an emission and both files are heavily
  // commented by design. An emission never sits on a line whose first non-space
  // characters are `//`, so nothing executable hides behind the rule.
  //
  // THE SECOND DIRECTION COMES FROM `toEqual` OVER THE WHOLE SCRAPE, not from
  // the presence of the known writer: the scrape lists EVERY `source:` in the
  // file, so removing or renaming the real emission shortens the list and a
  // second emission lengthens it, wherever and however it is formatted. Both
  // directions red on both tiers now. A `toContain` has only the first.
  //
  // NOTHING IS FILTERED OUT TO KEEP THE LISTS SHORT, and the non-emissions are
  // named below rather than excluded. Any rule narrow enough to drop them —
  // "ignore `source: string`" — is a rule a second emission could be dressed to
  // satisfy, which is how a guard gets talked out of its own reach.
  it('has exactly one writer per tier, and this file checks it', () => {
    const sourceMentions = (file: string): ReadonlyArray<string> =>
      [...file.split('\n').filter((line) => !/^\s*\/\//.test(line)).join('\n').matchAll(/\bsource:\s?([^,;\n]*)/g)].map((match) => match[1])

    // THE FETCHED TIER. Three mentions, one of them the writer:
    //   1. `parseFamilyMetadata(source: string)` — a PARAMETER named `source`,
    //      unrelated to the field. Listed, not filtered; the cost is that
    //      changing that function's signature reds this test, which is a cheap
    //      and visible price for a scrape nothing can hide from.
    //   2. `FetchedFace`'s `source: string` field declaration.
    //   3. the writer itself.
    const fetched = fs.readFileSync(path.join(here, 'font-source.ts'), 'utf8')
    expect(sourceMentions(fetched), 'font-source.ts must build `source` through webFaceSource and nowhere else; the first two entries are a function parameter and the FetchedFace field declaration, listed rather than filtered so no second writer can dress itself as one').toEqual([
      'string): FamilyMetadata | undefined {',
      'string',
      'webFaceSource(`${directory}/${slug}/${filename}`',
    ])
    // AND THE WRITER'S FULL ARGUMENT LIST, WHICH THE SCRAPE NECESSARILY TRUNCATES.
    // Capturing to the next comma is what keeps the committed tier's scrape from
    // swallowing the rest of a very long generated line, and the price is that
    // the fetched writer's second argument falls outside the capture. The
    // anchored regex did pin `today`, so this line exists so that nothing the
    // old assertion held is dropped by the change. It is NOT the exhaustiveness
    // guarantee — the `toEqual` above is; this pins one known string, which is
    // all a `toContain` can ever do.
    expect(fetched, 'font-source.ts must pass the fetch date through to webFaceSource; the `today` argument is what carries the default-date expression font-source.test.ts drives').toContain('source: webFaceSource(`${directory}/${slug}/${filename}`, today),')

    // THE COMMITTED TIER, THE SAME WAY. Two mentions: the `CatalogueFace` type
    // this script also generates (`… copyright: string; source: string;
    // scripts: …`), and the emission.
    const build = fs.readFileSync(path.join(here, '..', 'scripts', 'build-wasm.mjs'), 'utf8')
    expect(sourceMentions(build), 'build-wasm.mjs must build the committed tier\'s `source` through committedFaceSource and nowhere else; the first entry is the generated CatalogueFace type declaration, listed rather than filtered so no second emission can dress itself as one').toEqual([
      'string',
      '${JSON.stringify(committedFaceSource(face))}',
    ])
  })
})

// ─────────────────────────────────────────────────────────────────────────────
// THE PROHIBITION HALF OF THE TRIPWIRE, ASSERTED BY SOMETHING (F1, D-16.R.13).
//
// EVERYTHING ABOVE FEEDS `assertProvenanceShape` A KNOWN-GOOD VALUE. Nothing in
// this repository had ever fed the predicate a VIOLATING string, and the
// measurement is flat: replace `hostShaped`, `schemeShaped`, `digestShaped` and
// `branchShaped` with `/(?!)/` (never matches) and `projectShaped` with
// `/[\s\S]*/` (always matches), then run the helper's COMPLETE call scope —
// this file and `src/font-source.test.ts`, its only two importers — and the
// result is **33 passed, 0 failed**, identical to the unmutated baseline. Five
// neutered predicates, not one red test. The prohibitions were decoration.
//
// D-16.R.13 asked for this in terms: "A test asserts `source` contains no
// scheme and no host on either tier — that is the tripwire, and the convention
// alone will not hold." A tripwire nothing has ever walked into is a convention
// with a test file next to it. THIS SUITE EXISTS TO MAKE THAT MEASUREMENT
// IMPOSSIBLE: every predicate below is driven with a value that must trip it,
// so neutering any one of them reds a named case.
//
// AND THE ACCEPTED ROWS MATTER AS MUCH AS THE REJECTED ONES. `hostShaped` and
// `branchShaped` were just NARROWED to remove two measured false positives — a
// repository whose name ends in a TLD (`notofonts/notofonts.github.io`) and a
// directory literally named `dev` (`ofl/dev/Dev-Regular.ttf`). Without rows
// pinning those as ACCEPTED, nothing stops a future engineer widening the
// predicates straight back and re-introducing the false positives; without the
// rejected rows, nothing stops the narrowing going further until it matches
// nothing at all. The table holds both edges.

/**
 * THE HELPER'S PROHIBITIONS, KEYED BY A DISTINCTIVE FRAGMENT OF THE MESSAGE
 * EACH ONE FAILS WITH — one table, two uses: classifying a recorded failure,
 * and naming the message the REAL `expect` must throw with.
 *
 * Listed in the order `assertProvenanceShape` applies them, because THE ORDER
 * IS LOAD-BEARING: the helper short-circuits at the first failing `expect`, so
 * a row that asserts only "this throws" proves nothing about the prohibition it
 * was written for the moment an unrelated assertion starts firing first.
 */
const prohibitionFragments = [
  ['empty', 'publishes an empty source'],
  ['scheme', 'carries a URL scheme'],
  ['host', 'carries a HOST in'],
  ['ref', 'carries a moving ref'],
  ['digest', 'restates a SHA-256'],
  ['date', 'does not name a fetch date'],
  ['separator', "carries no ' — ' separator"],
  ['project', 'project half is not shaped'],
  ['path', 'no path within the project'],
] as const

type Prohibition = (typeof prohibitionFragments)[number][0]

function prohibitionOf(message: string): Prohibition {
  const hits = prohibitionFragments.filter(([, fragment]) => message.includes(fragment))
  // EXACTLY ONE, OR THROW. A rewritten helper message that matched two
  // fragments, or none, would otherwise reclassify a failure in silence and the
  // table would go on passing while asserting something else entirely.
  if (hits.length !== 1) throw new Error(`the failure message classifies to ${hits.length} prohibitions, not one — src/test/provenance-shape.ts and this table have drifted: ${message}`)
  return hits[0][0]
}

function fragmentFor(prohibition: Prohibition): string {
  const hit = prohibitionFragments.find(([name]) => name === prohibition)
  if (hit === undefined) throw new Error(`no message fragment is recorded for the ${prohibition} prohibition`)
  return hit[1]
}

/**
 * EVERY PROHIBITION A VALUE TRIPS, NOT MERELY THE FIRST.
 *
 * `assertProvenanceShape` SHORT-CIRCUITS under the real `expect` — it throws at
 * the first failing assertion — so the real runner can only ever report ONE
 * prohibition per value. That is not enough to state which prohibitions the
 * ORIGINAL DEFECT trips, and the original defect trips two:
 * `<the declared fetch host>/google/fonts/main/…` is caught by the HOST slot
 * AND by the REF slot, and preserving BOTH through the narrowing of those two
 * predicates was the entire point of the narrowing.
 *
 * The helper takes `expect` AS A PARAMETER precisely so it can be driven with
 * something other than vitest's — a seam it already has, put there so this
 * module could stay free of a test-runner import. A recording stand-in that
 * COLLECTS failures instead of throwing walks every assertion and reports all
 * of them, in helper order.
 *
 * IT REIMPLEMENTS THREE MATCHERS AND NO PREDICATE. `.not.toBe`, `.not.toMatch`
 * and `.toMatch` over the value the helper hands it — the regexes, the slot
 * splitting and the ordering all remain the helper's. And the recorder is never
 * trusted alone: every rejected row below ALSO runs under the real vitest
 * `expect` and must throw with the message of the FIRST prohibition this
 * recorder saw, which is what stops the table quietly becoming a second
 * implementation of the predicate that agrees only with itself.
 */
function prohibitionsTrippedBy(value: string): ReadonlyArray<Prohibition> {
  const failures: Prohibition[] = []
  const fail = (failed: boolean, message: string | undefined): void => {
    if (failed) failures.push(prohibitionOf(message ?? ''))
  }
  const recording = (actual: unknown, message?: string) => ({
    not: {
      toBe: (other: unknown) => fail(Object.is(actual, other), message),
      toMatch: (pattern: RegExp) => fail(pattern.test(String(actual)), message),
    },
    toMatch: (pattern: RegExp) => fail(!pattern.test(String(actual)), message),
  })
  assertProvenanceShape(recording as unknown as typeof expect, 'recorded', 'the row under test', value)
  return failures
}

type Row = Readonly<{ name: string; value: string; trips: ReadonlyArray<Prohibition> }>

// THE REJECTED ROWS. `trips` is the EXACT ordered set of prohibitions the value
// fires, not a set it merely contains: an exact list pins WHICH prohibition
// does the rejecting, which is the whole difference between this table and one
// that asserts "something threw" and passes for the wrong reason forever after.
const rejected: ReadonlyArray<Row> = [
  {
    // THE HOST IS COMPOSED FROM `fontsRepositoryHost`, NOT SPELLED, AND THAT
    // IS NOT A CONCESSION TO THE HOST SCAN — it is the same rule the test
    // `never interpolates a declared font host into the field` already states
    // twenty lines up: the tripwire is stated against the declared hosts BY
    // IDENTITY rather than by spelling, so a rename in `fontHostDeclarations`
    // cannot walk out from under it. It also keeps the one legal spelling of a
    // declared host in the one module allowed to hold it (D-16.4), which is why
    // `scan:font-hosts` is content. THE HOST ITSELF IS STILL THE REAL ONE: this
    // row's fidelity to the actual historical defect is the point of the row,
    // and substituting a fictional host would make it prove less. Do not
    // "simplify" this back to a literal.
    name: 'the ORIGINAL DEFECT — a bare mutable-branch retrieval path, which must trip the HOST slot AND the REF slot',
    value: `${fontsRepositoryHost}/google/fonts/main/ofl/kanit/Kanit-Regular.ttf`,
    trips: ['host', 'ref', 'date', 'separator', 'project', 'path'],
  },
  {
    name: 'a URL scheme — a resolvable-looking promise of fetchability, wherever it sits',
    value: 'google/fonts@v1.0 — https://fonts.example/ofl/kanit/Kanit-Regular.ttf, fetched 2026-09-03',
    trips: ['scheme'],
  },
  {
    // COMPOSED FOR THE SAME REASON, and here the host's IDENTITY is doing no
    // work at all: the row proves `hostShaped` fires in the owner slot, and any
    // host-shaped segment proves that. What it must NOT do is introduce a new
    // spelling of a real host into the tree — least of all one that is
    // forbidden OUTRIGHT (D-16.3), which is the exact string this product must
    // never contain. Reusing the already-declared host costs nothing and adds
    // no spelling.
    name: 'a host in the owner slot, where a pasted retrieval path puts one',
    value: `${fontsRepositoryHost}/fonts@v1.0 — ofl/kanit/Kanit-Regular.ttf, fetched 2026-09-03`,
    trips: ['host'],
  },
  {
    // THE SECOND HOST SLOT, WHICH A NARROWING ONCE DROPPED. Checking only the
    // project half let a host pasted into the PATH half through; this row is
    // the bound `hostSlotsOf` restored, held by a test rather than a comment.
    // Composed, as above. A WHOLE retrieval path pasted into the path half —
    // and note it trips the host slot ONLY: `main` sits in it, but the path
    // half is deliberately not a ref slot (`refSlotsOf` says why), so this row
    // pins that documented asymmetry as well as the restored bound.
    name: 'a host at the head of the path half — the bound the narrowing had to restore',
    value: `google/fonts — ${fontsRepositoryHost}/google/fonts/main/ofl/kanit/Kanit-Regular.ttf, fetched 2026-09-03`,
    trips: ['host'],
  },
  {
    // AND IT CARRIES A FETCH DATE, DELIBERATELY. A digest row missing its date
    // would fail on the date assertion first and prove nothing about
    // `digestShaped`; everything here but the digest is well-formed, so the
    // digest is the only thing left to reject it.
    name: 'a 64-hex SHA-256 — a second authority on a fact the asset key already owns',
    value: 'google/fonts@v1.0 — ofl/kanit/Kanit-Regular.9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08.ttf, fetched 2026-09-03',
    trips: ['digest'],
  },
  {
    // THE DEFECT SPELLED LEGALLY: perfect grammar, a moving ref in the one slot
    // a pin belongs in. `main` does not identify bytes.
    name: 'a moving ref in the release slot — the defect wearing the correct grammar',
    value: 'google/fonts@main — ofl/kanit/Kanit-Regular.ttf, fetched 2026-09-03',
    trips: ['ref'],
  },
  {
    name: 'no \' — \' separator at all, so `split` reads the whole string as the project half',
    value: 'google/fonts ofl/kanit/Kanit-Regular.ttf, fetched 2026-09-03',
    trips: ['separator', 'project', 'path'],
  },
  {
    // FINDING F6, NAILED DOWN. Measured: this exact string satisfied all seven
    // of the helper's assertions until an hour ago — a project and a day, and
    // no way back to the file. It names WHICH REPOSITORY and never WHICH FILE.
    name: 'an EMPTY PATH HALF (F6) — names a project and a fetch date and no file within the project',
    value: 'owner/repo@v1 — , fetched 2026-09-03',
    trips: ['path'],
  },
  {
    name: 'the empty string — a face that publishes no provenance at all',
    value: '',
    trips: ['empty', 'date', 'separator', 'project', 'path'],
  },
]

// THE ACCEPTED ROWS — THE ANTI-SOFTENING CONTROL. The first two are the two
// MEASURED FALSE POSITIVES the narrowing removed; they are here so a future
// widening of `hostShaped` or `branchShaped` back to a whole-field substring
// scan reds immediately instead of reddening the committed-tier loop for the
// entire catalogue on the day the relevant family lands.
const accepted: ReadonlyArray<Row> = [
  {
    name: 'a repository whose NAME ends in a TLD is not a host reference',
    value: 'notofonts/notofonts.github.io@v2.0 — ofl/notosans/NotoSans-Regular.ttf, fetched 2026-09-03',
    trips: [],
  },
  {
    name: 'a directory literally named `dev` is a directory, not a moving ref',
    value: 'google/fonts — ofl/dev/Dev-Regular.ttf, fetched 2026-09-03',
    trips: [],
  },
  {
    // WHAT "GOOD" ACTUALLY LOOKS LIKE, taken from the generated catalogue
    // rather than invented — and not the easy one: `4.005R` in the release slot
    // and `source-serif-4.005_Desktop` at the head of the path half are exactly
    // the shapes a version-number-blind host or ref scan misreads.
    name: 'the shape a real committed face uses, lifted from the generated catalogue',
    value: 'adobe-fonts/source-serif@4.005R — source-serif-4.005_Desktop/TTF/SourceSerif4Display-Regular.ttf, fetched 2026-09-02',
    trips: [],
  },
]

describe('`source`\'s prohibitions reject what they name, and nothing else', () => {
  // ANTI-VACUITY, TWO MECHANISMS, BOTH DELIBERATE.
  //
  //   1. `it.each` — every row is its own named test, so a truncated table
  //      shrinks the visible test count rather than passing in silence inside a
  //      loop that ran zero times. This is the same failure class the whole
  //      story is about, one level down.
  //   2. The counts and the COVERAGE are asserted here, in code. `it.each` over
  //      an EMPTY array still reports a green file, so the count alone is not
  //      optional — and the coverage assertion is the stronger half: every
  //      prohibition the helper has must be tripped by at least one row, so a
  //      tenth assertion added to `assertProvenanceShape` with no row to
  //      exercise it reds HERE rather than shipping unasserted, which is
  //      exactly how the nine below came to be unasserted.
  it('drives every prohibition the helper has, from a table that cannot go empty', () => {
    expect(rejected.length, 'the rejected table has been truncated; each row is the only thing asserting its prohibition').toBe(9)
    expect(accepted.length, 'the accepted table has been truncated; these rows are the only thing stopping the predicates being widened back to their measured false positives').toBe(3)
    const covered = [...new Set(rejected.flatMap((row) => row.trips))].sort()
    const all = prohibitionFragments.map(([name]) => name as Prohibition).sort()
    expect(covered, 'a prohibition in `assertProvenanceShape` is tripped by no row in this table, so neutering it would red nothing — that is the exact measurement this suite exists to make impossible (33 passed / 0 failed with five predicates neutered)').toEqual(all)
  })

  it.each(rejected)('rejects $name', (row: Row) => {
    // WHICH PROHIBITIONS, IN WHICH ORDER — recorded through the helper's own
    // `expect` seam, so short-circuiting cannot hide the second and later ones.
    expect(prohibitionsTrippedBy(row.value), `${row.value} does not trip exactly the prohibitions this row was written to pin`).toEqual(row.trips)

    // AND IT GENUINELY THROWS UNDER THE REAL VITEST `expect`. Without this the
    // table would be a second implementation of the predicate agreeing with
    // itself: the recorder never throws, so a helper that stopped rejecting
    // anything would still satisfy the assertion above if the recorder were all
    // that ran. The thrown message must be the FIRST prohibition recorded,
    // which is what ties the recorder's ordering to the real short-circuit.
    let thrown: unknown
    try {
      assertProvenanceShape(expect, 'committed tier', row.name, row.value)
    } catch (error) {
      thrown = error
    }
    expect(thrown, `${row.value} was accepted by assertProvenanceShape under the real \`expect\``).toBeInstanceOf(Error)
    expect(String((thrown as Error).message), `${row.value} threw, but not on the ${row.trips[0]} prohibition the recorder saw fire first`).toContain(fragmentFor(row.trips[0]))
  })

  it.each(accepted)('accepts $name', (row: Row) => {
    // Under the real `expect`: a throw here fails the test outright.
    assertProvenanceShape(expect, 'committed tier', row.name, row.value)
    // And under the recorder, so the assertion is that NOTHING fired rather
    // than that nothing fired FIRST — a value tripping a later prohibition
    // would be invisible to the short-circuiting call above only if an earlier
    // one also fired, but stating the empty set costs one line and closes it.
    expect(prohibitionsTrippedBy(row.value), `${row.value} is a legitimate provenance string and the predicates have been widened back over it`).toEqual([])
  })
})
