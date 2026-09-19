// THE `source` PROVENANCE SHAPE, SHARED BY THE TWO SUITES THAT ASSERT IT
// (D-16.R.13, DW-160). Test-only, and it lives beside `sfnt-fixture.ts` for
// that reason.
//
// It is shared rather than duplicated because the defect this tripwire guards
// is **the two tiers drifting apart**. Two copies of the predicate can drift
// exactly as the two writers did, and then each tier would be checked against
// its own idea of the rule — which is the failure one level up from the one
// being prevented.
//
//   `src/font-provenance.test.ts`  — the committed tier, over every generated
//                                    catalogue row, plus `webFaceSource`.
//   `src/font-source.test.ts`      — the fetched tier's REAL write path, on an
//                                    outcome from `fetchWebFamily` with no
//                                    `today` argument, so the default date
//                                    expression is observed rather than
//                                    bypassed.
//
// THE FIELD BEING MATCHED HAS A GRAMMAR, AND THE PROHIBITIONS ARE READ AGAINST
// IT rather than against the flat text:
//
//   <owner>/<repo>[@<release>] — <path within the project>, fetched YYYY-MM-DD
//   \________ the project half ________/  \_ the path half _/  \_ the date _/
//
// Two of the four prohibitions are POSITIONAL — a host and a moving ref are
// wrong in particular SLOTS of that grammar, not merely wrong somewhere in the
// string — and reading them as substring scans produced measured false
// positives on real upstreams. See `hostShaped` and `branchShaped`.

/**
 * A HOST, RECOGNISED BY ITS TLD RATHER THAN BY A LIST OF KNOWN FONT HOSTS.
 *
 * A list of the hosts this product does reach would pass the moment somebody
 * reached for a new one, which is the whole failure shape. The TLD set is
 * deliberately narrow so that a FILENAME does not read as a host: `.ttf`,
 * `.pb`, `.md` and a version like `4.005_Desktop` are not in it.
 *
 * **ANCHORED TO ONE WHOLE SEGMENT, and the anchoring is the fix.** This matches
 * a single `/`-separated segment in full (`^…$`) — it is NOT a substring scan
 * over `source`, and it is applied only at the slots `hostSlotsOf` returns: the
 * head segment of the project half AND the head segment of the path half, the
 * two places a pasted retrieval path can put a host. The unanchored version
 * scanned the whole field and therefore read the repository NAME in
 *
 *   `notofonts/notofonts.github.io@v2.0 — ofl/notosans/NotoSans-Regular.ttf, …`
 *
 * as a host, because `github.io` ends in the `io` TLD. That is a real,
 * plausible next-batch upstream, and the false positive would have reddened the
 * committed-tier loop for the entire catalogue the day that family landed. A
 * repository whose NAME contains a TLD is not a host reference; the narrowing
 * removes that reading and nothing else — a genuine host still stands at the
 * head of one half or the other, and is still caught there.
 *
 * The interior `(?:\.[a-z0-9][a-z0-9-]*)*` is what lets a MULTI-LABEL host —
 * `<sub>.<name>.<tld>`, not just `<name>.<tld>` — match as ONE segment now that
 * `\b` is gone. Hosts are named generically throughout these comments, in the
 * house style of `font-provenance.test.ts`: every point here is about the SHAPE
 * of a host in a slot, and none of them depends on which host it is. Spelling
 * one would also fail `scripts/forbidden-font-hosts.mjs`, which scans comments
 * exactly as it scans code — `src/font-source.ts` is the single declared home
 * of every allowed host (D-16.3, D-16.4).
 */
export const hostShaped = /^[a-z0-9][a-z0-9-]*(?:\.[a-z0-9][a-z0-9-]*)*\.(?:com|org|net|io|dev|co|app|sh|xyz)$/i

/**
 * WHOLE-FIELD SCANS, DELIBERATELY, and unchanged.
 *
 * Unlike a host and a ref, these two have no legitimate slot: a `://` anywhere
 * in the field is a resolvable-looking promise, and a 64-hex digest anywhere in
 * the field is a second authority on a fact the asset key already owns. Neither
 * has a measured false positive against real upstream names, so neither is
 * narrowed.
 */
export const schemeShaped = /[a-z][a-z0-9+.-]*:\/\//i
export const digestShaped = /\b[0-9a-f]{64}\b/i

/**
 * A MOVING REF OF ANY NAME, not just the one that was actually there.
 *
 * The defect was `main`. Pinning the tripwire to that word alone would let the
 * identical defect back in spelled `develop`, `trunk` or `Main` — a guard
 * fitted to the instance rather than to the class. Case-insensitive for the
 * same reason.
 *
 * **ANCHORED TO ONE WHOLE SEGMENT, like `hostShaped`.** This matches a single
 * slot in full (`^…$`); it is NOT a substring scan, and it is applied only at
 * the slots `refSlotsOf` returns. The unanchored version scanned the whole
 * field and therefore tripped on
 *
 *   `google/fonts — ofl/dev/Dev-Regular.ttf`
 *
 * because a DIRECTORY is literally named `dev`. A directory named `dev` is a
 * directory; the word only means "moving ref" where a ref would be read from.
 * The word list is untouched — only where it is looked for has changed.
 */
export const branchShaped = /^(?:main|master|trunk|develop|dev|head|latest|default)$/i

/**
 * THE PROJECT HALF, SHAPED — `<owner>/<repo>` with an optional `@<release>`.
 *
 * Asserting merely that it is non-empty is very nearly vacuous: `split(' — ')`
 * returns the WHOLE string when the separator is absent, so a `source` with no
 * separator at all would pass a non-empty check while carrying no path and no
 * project. The separator's presence is therefore asserted first, and the half
 * before it is held to a shape.
 */
export const projectShaped = /^[A-Za-z0-9][A-Za-z0-9._-]*\/[A-Za-z0-9][A-Za-z0-9._-]*(?:@[A-Za-z0-9][A-Za-z0-9._-]*)?$/

/**
 * THE PROJECT HALF — everything up to the ` — ` separator.
 *
 * `split` returning the WHOLE string when the separator is absent is load-
 * bearing here, not an accident to guard against: a bare pasted retrieval path
 * such as `<the declared fetch host>/google/fonts/main/ofl/kanit/Kanit-Regular.ttf`
 * carries no separator, so the whole of it is read as the project half and both
 * positional prohibitions still reach into it. That string trips the host slot
 * on its leading host segment and the ref slots on `main`, exactly as it did
 * before the narrowing — that is the case the narrowing must not lose.
 */
function projectHalfOf(value: string): string {
  return value.split(' — ')[0]
}

/**
 * THE PATH HALF — what sits between ` — ` and the trailing `, fetched …`.
 *
 * The date is stripped here rather than at the call sites because BOTH readers
 * of this half need it gone. A path half that is nothing but a host has no `/`
 * to split on, so its head segment would be the WHOLE half with the date still
 * attached — `<a host>, fetched 2026-09-03` — and no anchored TLD pattern
 * matches that. The host would slip through on punctuation.
 */
function pathHalfOf(value: string): string {
  return value.split(' — ').slice(1).join(' — ').replace(/, fetched \d{4}-\d{2}-\d{2}$/, '')
}

/**
 * THE SLOTS A HOST CAN OCCUPY: the HEAD `/`-separated segment of EACH half,
 * labelled so a failure says which one.
 *
 * A retrieval URL reads `host/owner/repo/ref/path…`, so a host always sits at
 * the HEAD of whatever was pasted — never after it. Every later segment of
 * either half is a repo name, a directory or a filename, none of which is a
 * host reference however it is spelled; that is what makes
 * `notofonts/notofonts.github.io` legal without making a host legal anywhere.
 *
 * BOTH HALVES, AND THE SECOND ONE IS A CORRECTION. Narrowing the check to the
 * project half alone silently dropped a bound the old whole-field scan held: a
 * `source` shaped `<owner>/<repo> — <a host>/s/<family>/<file>, fetched …` — a
 * retrieval path pasted into the PATH half — passed, because the only slot
 * being read was in the other half. That is not the false positive worth
 * removing: a host at the head of a pasted retrieval path is a host exactly
 * where a host appears. `schemeShaped` does not cover it either, since such a
 * paste carries no `://`. The two suites carry the concrete spelling; these
 * comments name it generically. The bound is restored at the right
 * OPERAND rather than by going back to a substring scan: one more head segment,
 * not the whole field. Real path halves head on `ofl`, `TTF`,
 * `source-serif-4.005_Desktop` and `Arimo-<40 hex>`, none of which is a TLD
 * segment, so the restored bound costs neither false positive B nor C.
 */
function hostSlotsOf(value: string): ReadonlyArray<readonly [string, string]> {
  return [
    ['the owner slot', projectHalfOf(value).split('/')[0]],
    ['the head of the path half', pathHalfOf(value).split('/')[0]],
  ]
}

/**
 * THE SLOTS A REF CAN OCCUPY: the `@<release>` slot, plus every project-half
 * segment PAST `<owner>/<repo>`.
 *
 * The release slot is where a pin belongs, so it is where a moving ref would be
 * substituted for one — `google/fonts@main — …` is the defect, spelled legally.
 * The segments past `<owner>/<repo>` are the other reading: when a whole
 * retrieval path is pasted in, `host/owner/repo/REF/path…` puts the ref at
 * index 3 of the project half, and everything from index 2 on is checked so the
 * ref cannot hide behind a missing or extra leading segment.
 *
 * THE PATH HALF IS DELIBERATELY NOT A REF SLOT — not even its head, which IS a
 * host slot. The asymmetry is the grammar's, not an oversight: a host can only
 * ever be pasted at the HEAD of a path, whereas a ref sits in the MIDDLE of one
 * (`host/owner/repo/REF/path…`), and by the time a path half has been split off
 * at ` — ` the ref position is behind it. What is left is the project's own
 * directory tree, where `ofl/dev/Dev-Regular.ttf` lives and a directory named
 * `dev` is a directory — the second measured false positive.
 *
 * A `source` with no release and only `<owner>/<repo>` yields NO ref slots, and
 * that is the correct passing condition rather than a skipped check: there is
 * no ref in it to be moving. `webFaceSource`'s `google/fonts — …` is exactly
 * that shape.
 */
function refSlotsOf(value: string): ReadonlyArray<string> {
  const projectHalf = projectHalfOf(value)
  const at = projectHalf.indexOf('@')
  const beforeRelease = at === -1 ? projectHalf : projectHalf.slice(0, at)
  const release = at === -1 ? [] : [projectHalf.slice(at + 1)]
  return [...release, ...beforeRelease.split('/').slice(2)]
}

/**
 * A TYPE-ONLY reference to vitest's `expect`, so this module carries no runtime
 * import of the test runner while still being fully typed at the call sites.
 */
type Expect = (typeof import('vitest'))['expect']

/**
 * `source` NAMES PROVENANCE, NOT A RETRIEVAL PATH. Four prohibitions — a
 * scheme, a host, a moving ref, a digest — and five positive requirements —
 * non-empty, a fetch date, the ` — ` separator, a shaped project half and a
 * non-empty path half — applied identically to both tiers.
 *
 * The scheme and digest prohibitions read the WHOLE field; the host and ref
 * prohibitions read the SLOTS where a host or a ref could actually stand — the
 * host at the head of EITHER half, the ref in the release slot or past
 * `<owner>/<repo>` — for the reasons written at `hostShaped`, `branchShaped`,
 * `hostSlotsOf` and `refSlotsOf`.
 *
 * `expect` is passed in rather than imported so this module stays free of a
 * test-runner import while being usable from either suite.
 */
export function assertProvenanceShape(expect: Expect, tier: string, subject: string, value: string): void {
  expect(value, `${tier}: ${subject} publishes an empty source`).not.toBe('')
  expect(value, `${tier}: ${subject} carries a URL scheme in \`source\`. A resolvable-looking string is a promise of fetchability, and a promise that decays reads as broken provenance (D-16.R.13).`).not.toMatch(schemeShaped)
  for (const [where, slot] of hostSlotsOf(value)) {
    expect(slot, `${tier}: ${subject} carries a HOST in ${where} of \`source\`. \`source\` names provenance, not a retrieval path (D-16.R.13).`).not.toMatch(hostShaped)
  }
  for (const slot of refSlotsOf(value)) {
    expect(slot, `${tier}: ${subject} carries a moving ref in \`source\` (in the release slot, or past <owner>/<repo>). A branch does not identify the bytes the field claims to describe — that was the defect (D-16.R.13).`).not.toMatch(branchShaped)
  }
  expect(value, `${tier}: ${subject} restates a SHA-256 in \`source\`. The digest is already the asset key, and duplicating it puts two authorities on one fact (D-16.R.13).`).not.toMatch(digestShaped)

  // AND IT IS STILL PROVENANCE, not merely a string that avoids four
  // prohibitions: a project, a path within it, and a fetch date.
  expect(value, `${tier}: ${subject} does not name a fetch date`).toMatch(/, fetched \d{4}-\d{2}-\d{2}$/)
  expect(value, `${tier}: ${subject} carries no ' — ' separator, so it names no project and no path within one`).toMatch(/ — /)
  expect(projectHalfOf(value), `${tier}: ${subject}'s project half is not shaped <owner>/<repo>[@<release>]`).toMatch(projectShaped)

  // THE PATH WITHIN THE PROJECT, WHICH NOTHING ABOVE ASSERTS. The comment
  // promises three things and only two were held to anything: measured,
  // `owner/repo@v1 — , fetched 2026-09-03` satisfied every assertion above it
  // with an empty path half. The path half is what says WHICH FILE of the
  // project these bytes are; without it the record names a repository and a
  // day, and a reader holding a `.folio` cannot get back to the file. Held to
  // presence only, which is all the real halves have in common
  // (`ofl/notosans/NotoSans-Regular.ttf`, `TTF/SourceSans3-Regular.ttf`).
  expect(pathHalfOf(value).trim(), `${tier}: ${subject} names a project and a fetch date but no path within the project, so it never says which file of that project the bytes are`).not.toBe('')
}
