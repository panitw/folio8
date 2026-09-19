import { execFileSync } from 'node:child_process'
import { existsSync, readFileSync, statSync } from 'node:fs'
import { dirname, extname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

// STORY 16.2 — THE HOST-FONT SCAN.
//
// WHAT IT GUARDS. SPEC-fonts' Non-goal *"No host fonts. Faces installed on the
// authoring or rendering machine are never enumerated or read."* is the ONE
// clause of that Non-goal D-16.1 left standing — every other clause of it was
// reversed by an owner decision — and Story 16.2 is the story most likely to
// break it, because it introduces a dropdown group whose heading reads
// AVAILABLE LOCALLY. STORY 16.4 SETTLED WHAT THAT GROUP CONTAINS: every face
// this machine already holds — the faces committed inside this release AND the
// faces this designer fetched and kept. It still does not mean "typefaces this
// computer has", and the distance between those two readings is one browser API
// call. WIDENING THE GROUP DID NOT WIDEN THE CLAUSE: both arms are bytes this
// designer put there itself, and neither is read off the operating system.
//
// WHY IT IS A SOURCE SCAN OVER THE WHOLE TREE, AND NOT A UNIT TEST.
// A test that only checked the store module would pass while `App.tsx` called
// the API. The claim is about the designer, so the scan reads the designer.
//
// IT WOULD PASS VACUOUSLY ON INTRODUCTION, which is the same trap
// `forbidden-font-hosts.mjs` is shaped around and the reason this file is
// shaped like it: these spellings appear ZERO times in this repository today,
// so a scan added now is green before it is correct and would stay green if it
// scanned nothing, matched nothing, or exempted everything. Three things are
// therefore load-bearing, and `src/host-font-access.test.ts` has one test each:
// a POSITIVE CONTROL, a POPULATION FLOOR, and the COMMENT DIRECTION.
//
// THE CLAIM IS BOUNDED, and every message below is worded to claim the bounded
// thing (D-8.5.5): what a green here means is that none of these spellings
// appears in the scanned population. It is not a proof that no host font is
// ever read — a source scan cannot see a name assembled at runtime or a call
// made by a dependency.
//
// ⚠ THE SPELLINGS ARE NOT WRITTEN IN ANY COMMENT, HERE OR ANYWHERE. The scan
// reads RAW source, so a spelling named in prose is an occurrence like any
// other; the exemption is computed over COMMENT-BLANKED source, exactly as the
// host scan's is, so a marker inside a comment declares nothing. Read the
// spellings off the array below.

/**
 * THE LOCAL FONT ACCESS SURFACE, BY EVERY SPELLING IT GOES BY.
 *
 * Patterns rather than substrings, because one of the near neighbours in this
 * repository really does collide: `standardFontDataUrl` (PDF.js) contains the
 * interface name this API exposes, and a substring scan would report the PDF
 * preview as a host-font reader. Word boundaries are the fix, and the
 * measured collision is why they are here rather than a precaution.
 *
 * EACH ENTRY CARRIES THE DECLARATION MARKER ON ITS OWN LINE, IN CODE — that is
 * how this file, which must spell what it forbids, is not itself a violation.
 */
export const HOST_FONT_ACCESS_APIS = [
  { name: 'queryLocalFonts', pattern: /\bqueryLocalFonts\b/, declaration: 'folio8:host-font-declaration' },
  { name: 'navigator.fonts', pattern: /\bnavigator\s*\.\s*fonts\b/, declaration: 'folio8:host-font-declaration' },
  { name: 'local-fonts permission', pattern: /(['"`])local-fonts\1/, declaration: 'folio8:host-font-declaration' },
  { name: 'FontData', pattern: /\bFontData\b/, declaration: 'folio8:host-font-declaration' },
]

/** The marker a line must carry, IN CODE, to name one of these deliberately. */
export const HOST_FONT_DECLARATION_MARKER = 'folio8:host-font-declaration'

/**
 * The trees the scan reads: the whole designer, named positively rather than by
 * exclusion so the population is a decision somebody made.
 *
 * `src/` is the product. `scripts/` builds it. `e2e/` drives it. All three could
 * hold a call, and the browser specs in particular are where "just check what
 * fonts the machine has" would be most tempting to write.
 */
export const SCANNED_ROOTS = ['folio-designer/src', 'folio-designer/scripts', 'folio-designer/e2e']

export const SCANNED_EXTENSIONS = ['.ts', '.tsx', '.mts', '.cts', '.js', '.jsx', '.mjs', '.cjs', '.css', '.html', '.json']

/**
 * THE POPULATION FLOOR, AND THE FRACTION IS STATED BECAUSE THE FRACTION IS THE
 * WHOLE DESIGN OF IT.
 *
 * MEASURED AT STORY 15.2b, 2026-09-05: **146** files under `SCANNED_ROOTS`
 * carry a scanned extension — the number `npm run scan:host-fonts` prints. THE
 * MEASUREMENT IS THE TOTAL. The split observed alongside it was `145 tracked + 1
 * untracked-but-not-ignored`, and that is a PRE-COMMIT OBSERVATION rather than a
 * recorded measurement: the 1 was this story's own new test file, so committing
 * this story turns the same 146 into `146 + 0`. The split moves with the index;
 * the total does not, which is why the total is the figure written down.
 *
 * THESE ARE CORRECTED RECORDED MEASUREMENTS, WHICH IS A DIFFERENT ACT FROM
 * RE-MEASURING A FLOOR. **The floor value 86 does not move**, and nor does the
 * other scan's 400. What moved is the three numbers this docblock had written
 * down beside them, each of which had gone stale and each of which is a claim:
 *
 *   - **129 → 146.** The 129 was correct at Story 16.2 and is not any more.
 *     (An earlier draft than that recorded 123 and set the floor at 50. That
 *     123 was measured while Story 16.2's own six new files were still
 *     UNTRACKED, so the walk — which read `git ls-files` — could not see them.
 *     THAT IS THIS GUARD RECORDING THE EXACT DEFECT STORY 15.2b CLOSES, in its
 *     own words, one story before the fix; see `splitPopulation`.)
 *   - **86 is 2/3 of 129 (66.7%) → 86 is 58.9% of 146.** The fraction is
 *     stated because the fraction is the whole design of the floor, so a
 *     fraction quoted against a stale denominator misstates the design. (An
 *     earlier draft of this line wrote 59.3%, which is 86/145 — the TRACKED half
 *     — while the sentence quotes the total 146. A percentage computed against a
 *     different denominator than the one it names is the same defect this
 *     docblock exists to correct, one decimal place smaller: 86/146 = 58.9%.)
 *   - **The cross-citation of `forbidden-font-hosts.mjs`** said it *"floors 400
 *     against a measured 579-600, which is 67-69%"*. Measured now: 400 against
 *     631, which is **63.4%**. (An earlier draft wrote 63.5%, which is 400/630 —
 *     again the tracked half against a sentence quoting the total.)
 *
 * A floor at 50 was ~39% of the population, which means a walk that collapsed
 * to 60 files — half the designer unread — would still have reported all-clear,
 * and an all-clear from a scan that read half the tree is the exact failure this
 * guard is shaped around. 86 still sits well above that.
 *
 * It is still a FLOOR and not an equality: it must catch a walk that collapsed,
 * not fence in normal growth. At 146 it leaves room for two fifths of these
 * files to be deleted before the floor is the thing that reds.
 */
export const POPULATION_FLOOR = 86

/** Reuses the host scan's comment blanker, because the two guards want the same asymmetry. */
export { blankComments } from './forbidden-font-hosts.mjs'
import { blankComments } from './forbidden-font-hosts.mjs'

/**
 * The 1-based line numbers on which an API is DECLARED rather than used: the
 * line carries both the spelling and the marker, WITH COMMENTS BLANKED.
 *
 * The marker therefore has to be real code and cannot be written in a comment.
 * That asymmetry is the point: anyone can write a comment, and a comment is
 * exactly where somebody parks a call they mean to make later.
 */
export function exemptLineNumbers(source, pattern, extension = '.ts') {
  const lines = blankComments(source, extension).split('\n')
  const exempt = new Set()
  for (let index = 0; index < lines.length; index++) {
    if (pattern.test(lines[index]) && lines[index].includes(HOST_FONT_DECLARATION_MARKER)) exempt.add(index + 1)
  }
  return exempt
}

/** Every undeclared occurrence in `source`. The occurrence search runs over the RAW text. */
export function occurrencesIn(source, extension = '.ts') {
  const lines = source.split('\n')
  const found = []
  for (const { name, pattern } of HOST_FONT_ACCESS_APIS) {
    const exempt = exemptLineNumbers(source, pattern, extension)
    for (let index = 0; index < lines.length; index++) {
      if (!pattern.test(lines[index]) || exempt.has(index + 1)) continue
      found.push({ api: name, line: index + 1, text: lines[index].trim().slice(0, 160) })
    }
  }
  return found
}

const insideTheWalk = (path) => {
  const segments = path.split('/')
  if (segments.includes('.git') || segments.includes('node_modules')) return false
  return SCANNED_ROOTS.some((root) => path === root || path.startsWith(`${root}/`))
}

/** The filters that turn a git listing into the population: inside the walk, scanned extension, really a file. */
const insidePopulation = (root) => (path) => insideTheWalk(path)
  && SCANNED_EXTENSIONS.includes(extname(path).toLowerCase())
  && existsSync(join(root, path)) && statSync(join(root, path)).isFile()

/**
 * The files this scan reads, repo-root-relative, SPLIT BY WHICH GIT LISTING
 * EACH CAME FROM — see `scannedPopulation` for the flat list.
 *
 * STORY 15.2b — THIS GUARD HAD ALREADY BEEN FOOLED BY THE HOLE IT NOW CLOSES,
 * AND SAID SO IN ITS OWN DOCBLOCK. Read `POPULATION_FLOOR` above: the figure it
 * had to correct was measured *"while this story's own six new files were still
 * UNTRACKED, so the walk — which reads `git ls-files` — could not see them."*
 * The catch below has spelled the principle since Story 16.2 — *an unobtainable
 * population must never read as all-clear* — and the walk still read the
 * tracked half only. Unioning `git ls-files --others --exclude-standard` into
 * the listing is what makes the principle true of the walk and not only of the
 * message.
 *
 * ⚠ THE LISTING IS ALL THAT WIDENED. `SCANNED_ROOTS` (two-segment here, unlike
 * the other scan's), `SCANNED_EXTENSIONS`, `insideTheWalk` and the floor value
 * are untouched.
 *
 * ⚠ AND THE WIDENING BROUGHT ITS OWN BOUND, WRITTEN HERE RATHER THAN LEFT
 * IMPLIED. `--exclude-standard` honours `.git/info/exclude` and the machine's
 * `core.excludesFile` as well as `.gitignore`, so the untracked-but-not-ignored
 * half is MACHINE-DEPENDENT: the same tree can give two populations on two
 * laptops, and a file swept up by one developer's global excludes is a file this
 * scan does not read there. The tracked half is not — it is whatever the index
 * says, identically everywhere. This is the replacement for the *"anything in an
 * untracked file"* clause the widening made false, not a clause deleted and left
 * unreplaced.
 *
 * THROWS RATHER THAN RETURNING AN EMPTY LIST, and the second call gets its own
 * refusal rather than an exemption: if the untracked half cannot be obtained,
 * this must NOT degrade to the tracked listing and report it as if it were
 * everything. Six refusals in all: each listing's `execFileSync` catch, an empty
 * TRACKED half, both halves empty, an untracked nested repository the listing
 * could not recurse into, and a population carrying no scanned extension.
 */
function splitPopulation(root) {
  const run = (args, half) => {
    let listing
    try {
      listing = execFileSync('git', ['-C', root, 'ls-files', ...args, '-z'], { encoding: 'utf8', maxBuffer: 256 * 1024 * 1024, stdio: ['ignore', 'pipe', 'pipe'] })
    } catch (error) {
      throw new Error(`host-font scan could not look: \`${['git', 'ls-files', ...args].join(' ')}\` — the ${half} of the population — failed in ${root} (${String(error)}), and an unobtainable population must never read as all-clear. It must NOT degrade to the other listing.`)
    }
    return listing.split('\0').filter((entry) => entry !== '')
  }
  const trackedListing = run([], 'tracked half')
  const untrackedListing = run(['--others', '--exclude-standard'], 'untracked-but-not-ignored half')
  if (trackedListing.length === 0 && untrackedListing.length === 0) {
    throw new Error(`host-font scan could not look: git reports no file at all in ${root}, neither tracked nor untracked-but-not-ignored`)
  }
  // THE TRACKED HALF BEING EMPTY IS ITS OWN REFUSAL. Folding it into the
  // both-halves-empty check above would let a repository with an empty or
  // unreadable INDEX over a full worktree proceed, reporting `tracked: 0` and
  // treating the untracked half as the tree — a scan that could not look reading
  // as an all-clear, with only `POPULATION_FLOOR` behind it.
  if (trackedListing.length === 0) {
    throw new Error(`host-font scan could not look: git tracks no file at all in ${root}. An empty or unreadable index over a worktree that still has files in it is not a population, and the untracked-but-not-ignored half must never be reported as if it were the whole tree.`)
  }
  // A NESTED REPOSITORY IS LISTED AS A DIRECTORY AND NEVER RECURSED INTO.
  // Measured: `git ls-files --others --exclude-standard` emits an untracked
  // nested repository as one entry with a TRAILING SLASH and lists none of the
  // files inside it; that entry then fails `insidePopulation`'s `isFile()` test
  // and is dropped SILENTLY — a subtree unread while the guard claims whole-tree
  // coverage. Refuse instead, naming the path. Only entries inside the declared
  // trees are refused, because this guard's claim is bounded by `SCANNED_ROOTS`.
  const unrecursed = untrackedListing.filter((path) => path.endsWith('/') && insideTheWalk(path))
  if (unrecursed.length > 0) {
    throw new Error(`host-font scan could not look: git listed ${unrecursed.join(', ')} in ${root} as an untracked NESTED REPOSITORY — a single directory entry it does not recurse into — so the files inside it were never read. A scan that skipped a subtree must not report the rest as a clean whole tree. Either track or ignore that repository, or scan it separately.`)
  }
  const keep = insidePopulation(root)
  // Order-stable and duplicate-free IN BOTH HALVES. The tracked half is deduped
  // against itself too: during an unresolved merge `git ls-files` emits a
  // conflicted path once per stage, and a tracked half taken verbatim would read
  // those files two or three times, inflate the count and duplicate every
  // finding — while `files === tracked + untracked` still held, so the
  // widening's own invariant could not see it.
  const seen = new Set()
  const tracked = []
  for (const path of trackedListing) {
    if (seen.has(path) || !keep(path)) continue
    seen.add(path)
    tracked.push(path)
  }
  const untracked = []
  for (const path of untrackedListing) {
    if (seen.has(path) || !keep(path)) continue
    seen.add(path)
    untracked.push(path)
  }
  const files = [...tracked, ...untracked]
  if (files.length === 0) throw new Error(`host-font scan could not look: no file in ${root} under ${SCANNED_ROOTS.join(', ')} — tracked or untracked-but-not-ignored — carries a scanned extension`)
  return { files, tracked: tracked.length, untracked: untracked.length }
}

/** The flat population, tracked and untracked-but-not-ignored alike. See `splitPopulation`. */
export function scannedPopulation(root) {
  return splitPopulation(root).files
}

export function scanHostFontAccess(root, { floor = POPULATION_FLOOR } = {}) {
  const { files, tracked, untracked } = splitPopulation(root)
  const findings = []
  for (const path of files) {
    // TOCTOU: the population filter stat'd this path and the read happens later.
    // An UNTRACKED file — an editor temporary, a build scratch file, exactly the
    // half Story 15.2b added — is far likelier to vanish in between than a
    // tracked one. A file that no longer exists calls nothing, so ENOENT is
    // skipped; any other read error is a file this scan could not look at, and a
    // guard must never read a broken read as a clean file.
    let source
    try {
      source = readFileSync(join(root, path), 'utf8')
    } catch (error) {
      if (error && error.code === 'ENOENT') continue
      throw error
    }
    for (const occurrence of occurrencesIn(source, extname(path).toLowerCase())) findings.push({ file: path, ...occurrence })
  }
  // The split is carried as a VALUE so `files === tracked + untracked` is
  // readable by a test rather than inferred from the printed sentence.
  return { files: files.length, tracked, untracked, floor, findings }
}

export function assertNoHostFontAccess(root, options = {}) {
  const result = scanHostFontAccess(root, options)
  if (result.files < result.floor) {
    throw new Error(`host-font scan read only ${result.files} files, under its floor of ${result.floor} — refusing to report a clean population over one this small, because a scan that read almost nothing is indistinguishable from a scan that found nothing`)
  }
  if (result.findings.length > 0) {
    const named = result.findings.map((finding) => `  ${finding.file}:${finding.line}: ${finding.api}\n    ${finding.text}`).join('\n')
    throw new Error(`the Local Font Access API appears in the designer's source (${result.findings.length} occurrence(s) across ${result.files} files scanned):\n${named}\n`
      + 'SPEC-fonts forbids it outright: "No host fonts. Faces installed on the authoring or rendering machine are never enumerated or read." '
      + 'The designer\'s AVAILABLE LOCALLY group means faces this designer itself put on this machine — the release\'s committed catalogue and the faces it FETCHED and kept, src/font-store.ts — and never typefaces this computer has installed. '
      + 'A commented-out call fails too: comments are stripped from the EXEMPTION, not from the scan.')
  }
  return result
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const root = join(dirname(fileURLToPath(import.meta.url)), '..', '..')
  const result = assertNoHostFontAccess(root)
  // The population is REPORTED, not merely used. A scan whose only evidence is
  // "it passed" says nothing about what it looked at, and the claim it licenses
  // is bounded: about the scanned population, never about what the running
  // browser does.
  console.log(`host-font scan: ${result.files} files scanned under ${SCANNED_ROOTS.join(', ')} (${result.tracked} tracked + ${result.untracked} untracked-but-not-ignored), floor ${result.floor}, 0 occurrences of ${HOST_FONT_ACCESS_APIS.length} spellings`)
}
