import { execFileSync } from 'node:child_process'
import { existsSync, readFileSync, statSync } from 'node:fs'
import { dirname, extname, join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

// STORY 8.5, AC4 — THE FORBIDDEN-HOST SOURCE SCAN.
//
// WHAT IT PROVES, AND THE SENTENCE IT MUST NEVER BE USED TO SAY (D-8.5.5).
// It proves that no forbidden font host appears in the SCANNED POPULATION, and
// nothing more. It is NOT a proof that "no request leaves the machine" — a
// source scan cannot see a host assembled at runtime, a host in a dependency,
// or a request made by something this repository did not write. Every message
// below is worded to claim the bounded thing.
//
// ⚠ STORY 15.2b AMENDED THIS BOUND, IT DID NOT DELETE IT. That list used to
// name "a host in an untracked file" as a fourth hole, and it was a real one:
// `scannedPopulation` read `git ls-files` alone, and because no story in this
// project stages anything, every file a story wrote was invisible to this scan
// for the story's whole life. The walk now unions
// `git ls-files --others --exclude-standard` into the listing, so the untracked
// clause is FALSE for this scan and has been struck from the sentence above.
// The runtime, dependency and not-written-here clauses remain true and remain
// written. See `splitPopulation`.
//
// WHY IT EXISTS. `SPEC.md`'s `## Non-goals` forbids a live font service, an
// arbitrary URL and a download on first use, and D-8.5.12 declined the owner
// brief's "size not paid for at first load" shape ON THE CONTRACT rather than
// by inventing a middle tier. The catalogue is therefore BUNDLED AND
// PRECACHED. This scan is the assertion that the decline was honoured in code
// rather than only in prose: the moment someone reaches for the cheap shape,
// the first thing they type is one of these two hosts.
//
// IT WOULD PASS VACUOUSLY ON INTRODUCTION, AND THAT IS THE TRAP (Design Note
// 2). These hosts appeared ZERO times in this repository's source before this
// story, so a scan added now is green before it is correct and would stay green
// if it scanned nothing at all. Three things are therefore load-bearing rather
// than optional, and each has its own test in `src/forbidden-font-hosts.test.ts`:
// a POSITIVE CONTROL (a fixture that does contain a host, asserted to fail), a
// POPULATION FLOOR (the scan reports how many files it read and refuses to
// report a clean population over an implausibly small one), and the
// COMMENT DIRECTION below.

/**
 * The hosts a bundled-and-precached catalogue must never reach for.
 *
 * EACH ENTRY CARRIES THE DECLARATION MARKER ON ITS OWN LINE, IN CODE — that is
 * how this file, which must spell the hosts to forbid them, is not itself a
 * violation. See `exemptLineNumbers` for why the marker has to be code and
 * cannot be a comment.
 */
export const FORBIDDEN_FONT_HOSTS = [
  { host: 'fonts.googleapis.com', declaration: 'folio8:font-host-declaration' },
  { host: 'fonts.gstatic.com', declaration: 'folio8:font-host-declaration' },
]

/**
 * STORY 16.1 — THE SCAN'S SECOND HALF (D-16.4). THE GUARD IS AMENDED, NEVER
 * DELETED.
 *
 * D-16.1 reversed *"no live font service"*: the author now reaches the published
 * library, and a family's face, licence text and metadata are fetched from a
 * third party at the moment of the pick. The obvious reading of that reversal is
 * that this scan has lost its subject. It has not — it has gained a second one.
 *
 * THE TWO HALVES SAY DIFFERENT THINGS, and collapsing them would lose the
 * distinction the reversal turns on:
 *
 *   FIRST HALF (`FORBIDDEN_FONT_HOSTS`) — the two stylesheet-and-asset hosts
 *   declared above stay FORBIDDEN OUTRIGHT. Not because they are Google's, but
 *   because D-16.3 measured what the `css2` endpoint actually serves a browser:
 *   `woff2`, which `decodeRecognisedFont` refuses by design, split by
 *   `unicode-range` into per-script subsets that would embed partial coverage
 *   into a document naming the whole family. Forbidding the host is what keeps
 *   that trap shut.
 *
 *   SECOND HALF (`DECLARED_ONLY_FONT_HOSTS`, declared just below) — the
 *   repository host that serves face bytes with CORS, and the index host the
 *   build script snapshots. Both are ALLOWED, and allowed IN EXACTLY ONE PLACE:
 *   they may appear only on a line carrying the declaration marker in code, and
 *   anywhere else under `SCANNED_ROOTS` fails the build. That is what keeps
 *   `src/font-source.ts` the single declared home of every host this designer
 *   reaches, instead of a fetch site appearing wherever somebody found it
 *   convenient.
 *
 * ⚠ NEITHER HALF'S HOSTS MAY BE SPELLED IN THIS COMMENT, and that is the guard
 * working on its own file: the scan reads RAW source, so a host named in prose
 * here is an occurrence like any other. Read them off the two arrays.
 *
 * ⚠ THE SECOND HALF WOULD PASS VACUOUSLY ON INTRODUCTION IF ITS SUBJECT DID NOT
 * EXIST, which is the same trap the first half was written around and the reason
 * this amendment lands in the SAME story as the module it polices, rather than
 * in a successor: a guard that arrives after the population it polices is how
 * D-8.6.5 shipped green. `src/forbidden-font-hosts.test.ts` red-proves it by
 * DELETING THE HALF, never by falsifying a condition.
 */
export const DECLARED_ONLY_FONT_HOSTS = [
  { host: 'raw.githubusercontent.com', declaration: 'folio8:font-host-declaration' },
  { host: 'fonts.google.com', declaration: 'folio8:font-host-declaration' },
]

/** Both halves, scanned by one walk. The half a finding came from is carried on it. */
export const SCANNED_FONT_HOSTS = [
  ...FORBIDDEN_FONT_HOSTS.map((entry) => ({ ...entry, half: 'forbidden' })),
  ...DECLARED_ONLY_FONT_HOSTS.map((entry) => ({ ...entry, half: 'declared-only' })),
]

/** The marker a line must carry, IN CODE, to declare a host deliberately. */
export const DECLARATION_MARKER = 'folio8:font-host-declaration'

/**
 * The trees the scan reads: THE SOURCE THAT BUILDS THE PRODUCT, named
 * positively rather than by exclusion so the population is a decision somebody
 * made rather than whatever survived a filter.
 *
 * WHAT IS OUT, AND WHAT WAS MEASURED THERE (D-8.5.5 — a scan's claim is
 * bounded, and the bound is stated rather than implied):
 *
 *   - `_bmad-output/` is the planning and implementation RECORD. It is where
 *     this repository writes down that these hosts are forbidden — the spec for
 *     this very story names both of them — so scanning it would make the record
 *     of the rule a violation of it. Measured at Story 8.5: 15 occurrences,
 *     all in archived UX mockup HTML from 2026-08-23 and in the story artifacts
 *     that quote them.
 *   - `fixtures/` and `test-data/` hold PDFs and `.folio` documents, no source.
 *
 *   - `docs/` is published prose. Measured at Story 8.5: 3 occurrences, all in
 *     `docs/expression-reference.html`'s remote font stylesheet links. THAT
 *     EXCEPTION NO LONGER EXISTS: the rendering library documentation change
 *     replaced those links with system font stacks, because the designer now
 *     bundles `docs/rendering-library.html`, `docs/folio-format.html` and
 *     `docs/expression-reference.html` as precached release assets
 *     (`scripts/build-wasm.mjs`), and a shipped page must request no font host.
 *     The tree itself stays outside `SCANNED_ROOTS` — `docs/` is the
 *     out-of-walk control in `src/forbidden-font-hosts.test.ts` — so this scan
 *     does NOT prove those pages clean; the bound is stated, not implied.
 *
 * NOTHING ELSE THAT BUILDS, TESTS OR SHIPS THE PRODUCT IS EXCLUDED. The
 * designer, the engine, the lint module, the hash matrix, the font tools and
 * the CI workflows are all in.
 */
export const SCANNED_ROOTS = ['folio-designer', 'folio-go', 'lint', 'hashmatrix', 'tools', '.github']

/**
 * The file classes the scan reads within those trees: source, in every language
 * this repository writes, plus the manifests and stylesheets a font host would
 * realistically be typed into.
 */
// `.mts`/`.cts` ARE HERE BECAUSE THE FIRST ONE ARRIVED WITH THIS SCAN.
// `scripts/forbidden-font-hosts.d.mts` — this module's own type sidecar — is
// the repository's only `.mts` file, and it sat OUTSIDE the population this
// scanner's comment claims covers "the source that builds the product". A
// blind spot the guard's own change created is the cheapest kind to leave in.
export const SCANNED_EXTENSIONS = ['.ts', '.tsx', '.mts', '.cts', '.js', '.jsx', '.mjs', '.cjs', '.go', '.css', '.html', '.json', '.yml', '.yaml', '.sh', '.py', '.mod', '.sum']

/** Extensions whose language comments a line out with `#` rather than `//`. */
const HASH_COMMENT_EXTENSIONS = ['.yml', '.yaml', '.sh', '.py']

/**
 * `source` with every COMMENT's characters replaced by spaces, newlines kept,
 * so line and column numbers still line up with the original.
 *
 * ⚠ THIS IS NOT APPLIED TO THE SCAN. Read the direction carefully, because
 * getting it backwards is the whole defect this guard is shaped around: the
 * SCAN reads the RAW text, so a host commented out still fails. What is
 * stripped is the EXEMPTION — a `folio8:font-host-declaration` written inside a
 * comment declares nothing, because a comment is exactly where someone parks a
 * host they mean to use later. Comments are stripped FROM THE EXEMPTION, NOT
 * FROM THE SCAN.
 *
 * DELETE THIS STEP — pass the raw text to `exemptLineNumbers` instead — and the
 * comment-direction test reds on its own message. That is the mutation proof;
 * a guard that survives its own deletion is decoration.
 *
 * A character scanner rather than a regex, because a regex cannot tell
 * `// a comment` from the `//` inside `https://fonts.example`, and getting that
 * backwards makes the exemption fire on the very string it must not.
 */
export function blankComments(source, extension = '.ts') {
  const hash = HASH_COMMENT_EXTENSIONS.includes(extension)
  let out = ''
  let index = 0
  let quote
  const skip = (from) => { for (let at = from; at < source.length && source[at] !== '\n'; at++) out += ' '; return source.indexOf('\n', from) === -1 ? source.length : source.indexOf('\n', from) }
  while (index < source.length) {
    const char = source[index]
    if (quote !== undefined) {
      out += char
      if (char === '\\') { out += source[index + 1] ?? ''; index += 2; continue }
      if (char === quote || (char === '\n' && quote !== '`')) quote = undefined
      index++
      continue
    }
    if (char === '"' || char === '\'' || char === '`') { quote = char; out += char; index++; continue }
    if (char === '/' && source[index + 1] === '/') { index = skip(index); continue }
    if (hash && char === '#') { index = skip(index); continue }
    if (char === '<' && source.startsWith('<!--', index)) {
      const end = source.indexOf('-->', index)
      const stop = end === -1 ? source.length : end + 3
      for (let at = index; at < stop; at++) out += source[at] === '\n' ? '\n' : ' '
      index = stop
      continue
    }
    if (char === '/' && source[index + 1] === '*') {
      const end = source.indexOf('*/', index + 2)
      const stop = end === -1 ? source.length : end + 2
      for (let at = index; at < stop; at++) out += source[at] === '\n' ? '\n' : ' '
      index = stop
      continue
    }
    out += char
    index++
  }
  return out
}

/**
 * The 1-based line numbers on which `host` is DECLARED rather than used: the
 * line carries both the host and `DECLARATION_MARKER` **with comments blanked**.
 *
 * The marker therefore has to be real code — a string literal, an identifier —
 * and cannot be `// folio8:font-host-declaration`. That asymmetry is the point:
 * anyone can write a comment.
 */
export function exemptLineNumbers(source, host, extension = '.ts') {
  const stripped = blankComments(source, extension)
  const lines = stripped.split('\n')
  const exempt = new Set()
  for (let index = 0; index < lines.length; index++) {
    if (lines[index].includes(host) && lines[index].includes(DECLARATION_MARKER)) exempt.add(index + 1)
  }
  return exempt
}

/**
 * Every occurrence of a forbidden host in `source` that is not declared, as
 * `{ host, line, text }`. The occurrence search runs over the RAW source.
 */
export function occurrencesIn(source, extension = '.ts') {
  const lines = source.split('\n')
  const found = []
  for (const { host, half } of SCANNED_FONT_HOSTS) {
    const exempt = exemptLineNumbers(source, host, extension)
    for (let index = 0; index < lines.length; index++) {
      if (!lines[index].includes(host) || exempt.has(index + 1)) continue
      found.push({ host, half, line: index + 1, text: lines[index].trim().slice(0, 160) })
    }
  }
  return found
}

/** Whether a repo-relative path is inside one of the scanned trees. */
const insideTheWalk = (path) => {
  const segments = path.split('/')
  if (segments.includes('.git') || segments.includes('node_modules')) return false
  return SCANNED_ROOTS.includes(segments[0])
}

/** The filters that turn a git listing into the population: inside the walk, scanned extension, really a file. */
const insidePopulation = (root) => (path) => insideTheWalk(path)
  && SCANNED_EXTENSIONS.includes(extname(path).toLowerCase())
  && existsSync(join(root, path)) && statSync(join(root, path)).isFile()

/**
 * The files this scan reads, repo-root-relative, SPLIT BY WHICH GIT LISTING
 * EACH CAME FROM — see `scannedPopulation` for the flat list every caller but
 * `scanForbiddenFontHosts` wants.
 *
 * STORY 15.2b — WHY THERE ARE TWO LISTINGS. This walk read `git ls-files`
 * alone, which is TRACKED FILES ONLY. In this project no story stages
 * anything, so every file a story wrote was untracked for that story's entire
 * life and this scan read straight past it: Story 12.2 added five files and the
 * scan reported clean over a tree containing none of them
 * (`epic-11-14-decision-log.md`'s D-000.21, *"A gate that states its own hole
 * honestly"* — not the identically numbered D-000.21 in
 * `folio-mvp-decision-log.md`, which is a different rule). The second listing,
 * `git ls-files --others --exclude-standard`, adds the untracked-but-not-ignored
 * files, so nothing in the non-ignored tree goes unread. THE POINT IS NOT A
 * BIGGER NUMBER: it is that this scan can no longer confuse *"no forbidden host
 * found"* with *"I did not look at the files you just wrote."*
 *
 * ⚠ THE LISTING IS ALL THAT WIDENED. `SCANNED_ROOTS`, `SCANNED_EXTENSIONS` and
 * `insideTheWalk` are untouched, so an untracked file outside the declared trees
 * still cannot enter.
 *
 * ⚠ AND THE WIDENING BROUGHT A NEW BOUND IN, WHICH IS WRITTEN HERE RATHER THAN
 * LEFT IMPLIED. `--exclude-standard` is not just `.gitignore`: it also honours
 * `.git/info/exclude` and the machine's `core.excludesFile`. So the
 * untracked-but-not-ignored half is MACHINE-DEPENDENT — the same tree can yield
 * two different populations on two laptops, and a file one developer's global
 * excludes sweep up is a file this scan does not read there. The tracked half is
 * not: it is whatever the index says, identically everywhere. This replaces the
 * struck *"a host in an untracked file"* clause rather than leaving the bound
 * list one clause shorter than the truth.
 *
 * THROWS RATHER THAN RETURNING AN EMPTY LIST, in six places now, and each refusal
 * names what it could not obtain. A scan that could not look must never read as
 * an all-clear — that is the failure mode this whole guard is shaped around, and
 * returning `[]` here, OR QUIETLY FALLING BACK TO THE TRACKED LISTING when the
 * second call fails, would make every assertion downstream pass over a tree
 * nothing ever opened. The six: each listing's own `execFileSync` catch; an
 * empty TRACKED half (an unreadable or empty index over a full worktree is not a
 * population, and the untracked half alone must not stand in for the whole
 * tree); both halves empty; a nested repository the untracked listing could not
 * recurse into; and a population carrying no scanned extension.
 */
function splitPopulation(root) {
  const run = (args, half) => {
    let listing
    try {
      listing = execFileSync('git', ['-C', root, 'ls-files', ...args, '-z'], { encoding: 'utf8', maxBuffer: 256 * 1024 * 1024, stdio: ['ignore', 'pipe', 'pipe'] })
    } catch (error) {
      throw new Error(`forbidden font host scan could not look: \`${['git', 'ls-files', ...args].join(' ')}\` — the ${half} of the population — failed in ${root} (${String(error)}), and an unobtainable population must never read as all-clear. It must NOT degrade to the other listing.`)
    }
    return listing.split('\0').filter((entry) => entry !== '')
  }
  // Two calls, two refusals. The second inherits the first's rule rather than
  // being exempt from it: if the untracked half cannot be obtained, the scan
  // has not looked at the whole tree and says so instead of reporting the
  // tracked half as if it were everything.
  const trackedListing = run([], 'tracked half')
  const untrackedListing = run(['--others', '--exclude-standard'], 'untracked-but-not-ignored half')
  if (trackedListing.length === 0 && untrackedListing.length === 0) {
    throw new Error(`forbidden font host scan could not look: git reports no file at all in ${root}, neither tracked nor untracked-but-not-ignored`)
  }
  // THE TRACKED HALF BEING EMPTY IS ITS OWN REFUSAL, and it is here because the
  // widening nearly deleted it. Before Story 15.2b this walk threw on an empty
  // `git ls-files`; folding that into the both-halves-empty check above would let
  // a repository with an empty or unreadable INDEX over a full worktree proceed,
  // reporting `tracked: 0` and scanning the untracked half as if it were the
  // tree. That is a scan that could not look reading as an all-clear by another
  // door, with only `POPULATION_FLOOR` behind it.
  if (trackedListing.length === 0) {
    throw new Error(`forbidden font host scan could not look: git tracks no file at all in ${root}. An empty or unreadable index over a worktree that still has files in it is not a population, and the untracked-but-not-ignored half must never be reported as if it were the whole tree.`)
  }
  // A NESTED REPOSITORY IS LISTED AS A DIRECTORY AND NEVER RECURSED INTO, so its
  // files would go unread while this guard claimed whole-tree coverage — the
  // exact collapse Story 15.2b exists to close, arriving through the half that
  // story added. Measured: `git ls-files --others --exclude-standard` emits an
  // untracked nested repository as a single entry with a TRAILING SLASH and
  // lists none of the files inside it; that entry then fails `insidePopulation`'s
  // `isFile()` test and is dropped SILENTLY. Refuse instead, naming the path.
  // Only entries inside the declared trees are refused: this guard's claim is
  // bounded by `SCANNED_ROOTS`, so a nested repository it never claimed to read
  // is not its failure to report.
  const unrecursed = untrackedListing.filter((path) => path.endsWith('/') && insideTheWalk(path))
  if (unrecursed.length > 0) {
    throw new Error(`forbidden font host scan could not look: git listed ${unrecursed.join(', ')} in ${root} as an untracked NESTED REPOSITORY — a single directory entry it does not recurse into — so the files inside it were never read. A scan that skipped a subtree must not report the rest as a clean whole tree. Either track or ignore that repository, or scan it separately.`)
  }
  const keep = insidePopulation(root)
  // Order-stable and duplicate-free IN BOTH HALVES. The tracked half is deduped
  // against ITSELF as well as the untracked half against it: during an
  // unresolved merge `git ls-files` emits a conflicted path once per stage, so a
  // tracked half taken verbatim would read those files two or three times,
  // inflate the reported count and duplicate every finding in them — and
  // `files === tracked + untracked` would still hold, so the widening's own
  // invariant cannot see it.
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
  if (files.length === 0) throw new Error(`forbidden font host scan could not look: no file in ${root} under ${SCANNED_ROOTS.join(', ')} — tracked or untracked-but-not-ignored — carries a scanned extension (${SCANNED_EXTENSIONS.join(' ')})`)
  return { files, tracked: tracked.length, untracked: untracked.length }
}

/** The flat population, tracked and untracked-but-not-ignored alike. See `splitPopulation`. */
export function scannedPopulation(root) {
  return splitPopulation(root).files
}

/**
 * THE POPULATION FLOOR. A count written next to the thing it counts stops being
 * true the moment the thing grows, so this is a FLOOR rather than an equality —
 * but a floor is what turns "the scan found nothing" into a claim.
 *
 * MEASURED AT STORY 15.2b, 2026-09-05: **631** files under `SCANNED_ROOTS`
 * carry a scanned extension — the number `npm run scan:font-hosts` prints,
 * re-measured over this tree rather than estimated. THE MEASUREMENT IS THE
 * TOTAL, 631, and it is the total on purpose: under Story 15.2b's widening both
 * halves are read, so one population is what there is to count.
 *
 * THE SPLIT IS A PRE-COMMIT OBSERVATION, NOT A RECORDED MEASUREMENT, and is
 * written down as one so it does not read as a false claim a week later. When
 * the figure was taken the split was `630 tracked + 1 untracked-but-not-ignored`,
 * the 1 being this story's own new test file. The moment this story is committed
 * that same 631 becomes `631 + 0` — the split moves with the index while the
 * total does not, which is exactly why the total is what is recorded here.
 *
 * (An earlier draft of this comment recorded "MEASURED AT STORY 8.5: 579
 * tracked files", which was correct when written and had gone stale by 631; an
 * earlier draft than that recorded 1,058, matching neither the scanned
 * population nor the repository-wide extension count of 704. Both were
 * corrected to the measurement rather than carried forward. A count written
 * beside the thing it counts is a claim, and this one was checked.)
 *
 * THE VALUE DOES NOT MOVE, AND THAT IS A DECISION RATHER THAN AN OVERSIGHT.
 * 400 is a floor, not a census: it exists to catch a walk that collapsed, not
 * to fence in growth, and raising it because the population grew would be
 * tuning a guard to whatever it currently reports. Measured at Story 15.2b's
 * plan gate with a positive control: NO assertion anywhere in this repository
 * pins this population to an exact number — both consumers use
 * `toBeGreaterThan(POPULATION_FLOOR)` — so nothing else had to move with the
 * measurement either.
 */
export const POPULATION_FLOOR = 400

export function scanForbiddenFontHosts(root, { floor = POPULATION_FLOOR } = {}) {
  const { files, tracked, untracked } = splitPopulation(root)
  const findings = []
  for (const path of files) {
    // TOCTOU: `insidePopulation` stat'd this path, and the read happens later.
    // An UNTRACKED file — an editor temporary, a build scratch file, exactly the
    // half Story 15.2b added — is far likelier to vanish in between than a
    // tracked one. A file that no longer exists carries no host, so ENOENT is
    // skipped; anything else is a read this scan could not do, and a guard must
    // never read a broken read as a clean file.
    let source
    try {
      source = readFileSync(join(root, path), 'utf8')
    } catch (error) {
      if (error && error.code === 'ENOENT') continue
      throw error
    }
    for (const occurrence of occurrencesIn(source, extname(path).toLowerCase())) findings.push({ file: path, ...occurrence })
  }
  // THE SPLIT IS A VALUE, NOT A PRINTED STRING (Story 15.2b). `files === tracked
  // + untracked` is the widening's own invariant, and a test can only read it
  // off the result — inferring it from the CLI sentence would be an assertion
  // about a message rather than about the walk.
  return { files: files.length, tracked, untracked, floor, findings }
}

export function assertNoForbiddenFontHosts(root, options = {}) {
  const result = scanForbiddenFontHosts(root, options)
  if (result.files < result.floor) {
    throw new Error(`forbidden font host scan read only ${result.files} files, under its floor of ${result.floor} — refusing to report a clean population over one this small, because a scan that read almost nothing is indistinguishable from a scan that found nothing`)
  }
  if (result.findings.length > 0) {
    const named = result.findings.map((finding) => `  ${finding.file}:${finding.line}: ${finding.host} (${finding.half})\n    ${finding.text}`).join('\n')
    // THE TWO HALVES GET TWO SENTENCES, because they are two different findings.
    // A reader told "this host is forbidden" about a host the product legitimately
    // fetches from would either delete a working call site or delete the guard.
    const forbidden = result.findings.some((finding) => finding.half === 'forbidden')
    const declaredOnly = result.findings.some((finding) => finding.half === 'declared-only')
    throw new Error(`forbidden font host in scanned source (${result.findings.length} occurrence(s) across ${result.files} files scanned):\n${named}\n`
      + (forbidden
        ? `FORBIDDEN OUTRIGHT: ${FORBIDDEN_FONT_HOSTS.map((entry) => entry.host).join(' and ')} are never reached. D-16.3 measured that the `
          + 'stylesheet endpoint serves woff2 — which the engine refuses by design — split by unicode-range into per-script subsets '
          + 'that would embed partial coverage into a document naming the whole family. '
        : '')
      + (declaredOnly
        ? 'DECLARED-ONLY: this host is one the designer legitimately fetches from, and it may be spelled in exactly one module — '
          + 'src/font-source.ts, the single declared home of every allowed host (D-16.4). Do not add a second fetch site; call that one. '
        : '')
      + 'A commented-out host fails too — comments are stripped from the EXEMPTION, not from the scan. If a line must name a host, '
      + `declare it in code with the marker \`${DECLARATION_MARKER}\` on that same line.`)
  }
  return result
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const root = join(dirname(fileURLToPath(import.meta.url)), '..', '..')
  const result = assertNoForbiddenFontHosts(root)
  // The population size is REPORTED, not merely used (AC4). A scan whose only
  // evidence is "it passed" says nothing about what it looked at. The claim is
  // bounded on purpose (D-8.5.5): about the scanned population, never about
  // what leaves the machine.
  // AND THE SENTENCE NO LONGER SAYS "tracked" (Story 15.2b). It said so
  // truthfully for as long as the walk read `git ls-files` alone; the moment
  // the untracked-but-not-ignored half joined the population, the word became
  // the guard's own stale claim about itself. The split is printed rather than
  // summed away, because "630 tracked + 1 untracked" is what says the scan read
  // the files this story just wrote.
  console.log(`forbidden font host scan: ${result.findings.length} occurrence(s) in ${result.files} source files under ${relative(process.cwd(), root) || '.'} `
    + `(${result.tracked} tracked + ${result.untracked} untracked-but-not-ignored, floor ${result.floor}). `
    + 'This bounds the SCANNED POPULATION only; it is not a claim that no request leaves the machine.')
}
