import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import {
  DECLARATION_MARKER,
  DECLARED_ONLY_FONT_HOSTS,
  FORBIDDEN_FONT_HOSTS,
  SCANNED_FONT_HOSTS,
  POPULATION_FLOOR,
  SCANNED_EXTENSIONS,
  SCANNED_ROOTS,
  assertNoForbiddenFontHosts,
  blankComments,
  exemptLineNumbers,
  occurrencesIn,
  scanForbiddenFontHosts,
  scannedPopulation,
} from '../scripts/forbidden-font-hosts.mjs'

// STORY 8.5, AC4. THE SCAN THAT WOULD PASS VACUOUSLY, AND THE THREE THINGS
// THAT STOP IT (Design Note 2).
//
// The two forbidden hosts appear ZERO times in the trees this scan reads, so a
// scan introduced now is green BEFORE it is correct, and it would stay green if
// it scanned nothing at all, matched nothing at all, or exempted everything.
// "It passed" is therefore not evidence of anything here. What this file
// establishes instead:
//
//   1. A POSITIVE CONTROL — a fixture that really does contain a host, asserted
//      to fail, so the matcher is shown to match.
//   2. A POPULATION FLOOR — the scan reports how many files it read and refuses
//      to report a clean population over an implausibly small one, so a scan
//      that stopped looking reds instead of passing.
//   3. THE COMMENT DIRECTION — a host inside a comment STILL FAILS. Comments
//      are stripped from the EXEMPTION, never from the scan, and deleting that
//      stripping step reds a named test below on its own message.
//
// AND THE CLAIM IS BOUNDED (D-8.5.5). What a green here means is that no
// forbidden host appears in the scanned population. It is never a claim that no
// request leaves the machine, and no test here should ever be written as if it
// were.

const here = path.dirname(fileURLToPath(import.meta.url))
const repoRoot = path.join(here, '..', '..')
const hosts = FORBIDDEN_FONT_HOSTS.map((entry) => entry.host)
const declaredOnly = DECLARED_ONLY_FONT_HOSTS.map((entry) => entry.host)

/**
 * `git`, inside a throwaway repository, with every setting pinned that could
 * make one of these arms fail or HANG on somebody else's laptop (R2).
 *
 * `user.name`/`user.email` because a machine that has never configured an
 * identity cannot commit. `commit.gpgsign=false` because a machine with signing
 * on either fails the commit or blocks on a passphrase prompt.
 * `core.hooksPath=/dev/null` because a global `core.hooksPath`, or an
 * `init.templateDir` that installs hooks, would run somebody's own pre-commit
 * script inside this scratch repository — and the commits below also pass
 * `--no-verify`, which is the same refusal said twice on purpose.
 * `init.defaultBranch` because otherwise `git init` emits an advisory on every
 * run. A test that hangs on someone else's laptop is a test that gets deleted.
 */
const scratchGit = (repository: string) => (...args: string[]) => execFileSync('git', [
  '-C', repository,
  '-c', 'user.name=folio8 15.2b',
  '-c', 'user.email=15.2b@example.test',
  '-c', 'commit.gpgsign=false',
  '-c', 'core.hooksPath=/dev/null',
  '-c', 'init.defaultBranch=main',
  ...args,
], { stdio: ['ignore', 'pipe', 'pipe'] })

/**
 * Runs `body` with a `git` on `PATH` that delegates to the real one for
 * everything EXCEPT an invocation carrying `--others`, which exits non-zero.
 *
 * WHY A SHIM AND NOT SIMPLY A DIRECTORY GIT DOES NOT KNOW. Measured: in a
 * non-repository the FIRST call — the tracked listing — is the one that fails,
 * so the refusal that comes back names the TRACKED half and the `--others`
 * refusal is never reached at all. An arm written that way asserts the
 * pre-existing throw and would stay green through a `--others` call rewritten to
 * catch and fall back to the tracked listing, which is precisely the collapse
 * Story 15.2b exists to close. Making ONLY the second call fail is the only way
 * to put the second refusal on the path the proof exercises.
 *
 * `PATH` is restored in `finally`, and the real `git` is resolved BEFORE the
 * shim is prepended so the shim's own `exec` cannot recurse into itself.
 */
const withFailingUntrackedListing = <T>(body: () => T): T => {
  const shimDirectory = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-git-shim-'))
  const realGit = execFileSync('/bin/sh', ['-c', 'command -v git'], { encoding: 'utf8' }).trim()
  const shim = path.join(shimDirectory, 'git')
  fs.writeFileSync(shim, `#!/bin/sh\ncase " $* " in\n  *' --others '*)\n    echo 'folio8 15.2b shim: the --others listing is forced to fail' >&2\n    exit 128\n    ;;\nesac\nexec ${JSON.stringify(realGit)} "$@"\n`)
  fs.chmodSync(shim, 0o755)
  const originalPath = process.env.PATH
  process.env.PATH = `${shimDirectory}${path.delimiter}${originalPath ?? ''}`
  try {
    return body()
  } finally {
    if (originalPath === undefined) delete process.env.PATH
    else process.env.PATH = originalPath
    fs.rmSync(shimDirectory, { recursive: true, force: true })
  }
}

describe('no forbidden font host reaches the scanned source', () => {
  it('forbids exactly the two Google Fonts hosts, spelled here as well as there', () => {
    // Both hosts are spelled in this test file, so this line declares them the
    // way the scanner requires: IN CODE, on the same line, with the marker.
    // A comment could not do it — see the comment-direction test below.
    expect([...hosts].sort(), 'folio8:font-host-declaration').toEqual(['fonts.googleapis.com', 'fonts.gstatic.com'])
    expect(DECLARATION_MARKER).toBe('folio8:font-host-declaration')
  })

  // THE REAL SCAN, OVER THE REAL TREE. Its population size is asserted first,
  // because the finding count is only meaningful next to what was read.
  it('reads a real population of the product source trees and finds no forbidden host', () => {
    const result = scanForbiddenFontHosts(repoRoot)
    expect(result.files, `the scan read only ${result.files} files, which cannot support a claim about this repository`).toBeGreaterThan(POPULATION_FLOOR)
    // The two halves account for the whole population and nothing is counted
    // twice (Story 15.2b). ⚠ THIS IS NOT THE DELTA PROOF: on a committed tree
    // the untracked half is 0 and this reduces to `n === n + 0`, true and proof
    // of nothing. The falsifiable version lives in the hermetic arm below.
    expect(result.files).toBe(result.tracked + result.untracked)
    expect(result.findings, 'a forbidden font host appears in the product source. The catalogue is bundled and precached (D-8.5.12).').toEqual([])
    // AND IT SPANS EVERY TREE IT CLAIMS TO, EXACTLY.
    //
    // THE EXPECTATION IS DERIVED FROM `SCANNED_ROOTS`, not a subset written out
    // here. It used to name three of the six by hand and pipe them through
    // `expect.arrayContaining`, which made the `.sort()` beside it a no-op and
    // read as an exact-set check while being a containment one: `hashmatrix`,
    // `tools` and `.github` could have dropped out of the walk entirely and
    // this stayed green. Now adding a root the walk cannot reach reds, and so
    // does reaching a directory no root declares.
    const reached = [...new Set(scannedPopulation(repoRoot).map((file) => file.split('/')[0]))].sort()
    expect(reached, 'every declared scan root must contribute at least one file, and nothing outside them may').toEqual([...SCANNED_ROOTS].sort())
    expect(SCANNED_ROOTS.length, 'the scan roots must not have collapsed to a single tree').toBeGreaterThanOrEqual(3)
    // And what it deliberately does NOT read, so the bound is a decision on the
    // record rather than an accident of a filter (D-8.5.5).
    expect([...reached]).not.toContain('_bmad-output')
    expect([...reached]).not.toContain('docs')
  })

  // 1 — THE POSITIVE CONTROL. Without this, every assertion above is satisfied
  // by a scanner that matches nothing.
  it('fails, naming the file and the line, on a source file that contains a forbidden host', () => {
    const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-font-host-'))
    try {
      const git = scratchGit(scratch)
      git('init', '-q')
      const tree = path.join(scratch, 'folio-designer', 'src')
      fs.mkdirSync(tree, { recursive: true })
      fs.writeFileSync(path.join(tree, 'clean.ts'), 'export const ok = 1\n')
      fs.writeFileSync(path.join(tree, 'offender.ts'), `const a = 1\nconst url = 'https://${hosts[0]}/css2?family=Inter'\n`)
      git('add', '-A')

      const result = scanForbiddenFontHosts(scratch, { floor: 1 })
      expect(result.findings).toEqual([{ file: 'folio-designer/src/offender.ts', host: hosts[0], half: 'forbidden', line: 2, text: `const url = 'https://${hosts[0]}/css2?family=Inter'` }])
      // The thrown message NAMES the file and the line (AC4), not merely a count.
      expect(() => assertNoForbiddenFontHosts(scratch, { floor: 1 })).toThrow(/folio-designer\/src\/offender\.ts:2/)
      // And the clean file is not reported, so the matcher discriminates.
      expect(result.findings.map((finding) => finding.file)).not.toContain('folio-designer/src/clean.ts')
    } finally {
      fs.rmSync(scratch, { recursive: true, force: true })
    }
  })

  // 2 — THE POPULATION FLOOR. A scan that read almost nothing is
  // indistinguishable from a scan that found nothing, and it must not be
  // allowed to report the second.
  it('refuses to report a clean population over an implausibly small one', () => {
    const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-font-host-floor-'))
    try {
      const git = scratchGit(scratch)
      git('init', '-q')
      const tree = path.join(scratch, 'folio-go')
      fs.mkdirSync(tree, { recursive: true })
      fs.writeFileSync(path.join(tree, 'main.go'), 'package main\n')
      git('add', '-A')

      // Clean by content — and still refused, because one file is not a population.
      expect(scanForbiddenFontHosts(scratch, { floor: 1 }).findings).toEqual([])
      expect(() => assertNoForbiddenFontHosts(scratch)).toThrow(/read only 1 files, under its floor of/)
    } finally {
      fs.rmSync(scratch, { recursive: true, force: true })
    }
  })

  // AND THE POPULATION READER REFUSES TO GO QUIET IN THE OTHER TWO DIRECTIONS
  // TOO: a directory git does not track at all, and a repository with nothing
  // in the scanned trees. An unobtainable population must never read as an
  // all-clear.
  it('throws rather than reporting an empty population it could not obtain', () => {
    const notARepository = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-font-host-nogit-'))
    const emptyRepository = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-font-host-empty-'))
    try {
      expect(() => scannedPopulation(notARepository)).toThrow(/could not look/)
      const git = scratchGit(emptyRepository)
      git('init', '-q')
      fs.mkdirSync(path.join(emptyRepository, 'docs'), { recursive: true })
      fs.writeFileSync(path.join(emptyRepository, 'docs', 'page.html'), '<!doctype html>\n')
      git('add', '-A')
      expect(() => scannedPopulation(emptyRepository)).toThrow(/carries a scanned extension/)
    } finally {
      fs.rmSync(notARepository, { recursive: true, force: true })
      fs.rmSync(emptyRepository, { recursive: true, force: true })
    }
  })

  // 3 — THE COMMENT DIRECTION, the one that is easy to get exactly backwards.
  //
  // A HOST INSIDE A COMMENT STILL FAILS. The obvious implementation of "strip
  // comments" would exempt it, which is precisely the shape someone reaching
  // for the declined middle tier would leave behind: the host parked in a
  // commented-out line, ready to be uncommented.
  it('fails on a forbidden host that appears inside a comment, in every comment form', () => {
    for (const form of [
      `// const url = 'https://${hosts[0]}/css2'`,
      `/* const url = 'https://${hosts[0]}/css2' */`,
      `const a = 1 // https://${hosts[1]}`,
    ]) {
      expect(occurrencesIn(form).length, `a forbidden host commented out as ${JSON.stringify(form)} must still be reported`).toBeGreaterThan(0)
    }
  })

  // AND THE EXEMPTION IS WHAT COMMENTS ARE STRIPPED FROM. This is the mutation
  // proof for the stripping step: make `exemptLineNumbers` read the RAW text
  // instead of the comment-blanked text and this test reds on its own message,
  // because a marker written in a comment would then exempt the host beside it.
  it('honours a declaration written in code and refuses the same declaration written in a comment', () => {
    const inCode = `const entry = { host: '${hosts[0]}', declaration: '${DECLARATION_MARKER}' }`
    const inComment = `// '${hosts[0]}' ${DECLARATION_MARKER}`
    const inCommentTrailing = `const url = 'https://${hosts[0]}/css2' // ${DECLARATION_MARKER}`

    expect(exemptLineNumbers(inCode, hosts[0]), 'a declaration written in code exempts its own line').toEqual(new Set([1]))
    expect(occurrencesIn(inCode)).toEqual([])

    expect(exemptLineNumbers(inComment, hosts[0]), 'a declaration written in a comment declares nothing — comments are stripped FROM THE EXEMPTION').toEqual(new Set())
    expect(occurrencesIn(inComment).length).toBe(1)
    expect(exemptLineNumbers(inCommentTrailing, hosts[0]), 'a trailing comment cannot exempt a live line').toEqual(new Set())
    expect(occurrencesIn(inCommentTrailing).length).toBe(1)
  })

  // THE COMMENT BLANKER ITSELF, shown to blank comments AND to leave strings
  // alone. A blanker that ate the `//` inside a URL string would strip the rest
  // of that line and make the exemption fire on text nobody wrote.
  it('blanks comments, preserves line numbering, and does not mistake a URL for a comment', () => {
    // LINE NUMBERING IS THE PROPERTY, not the exact run of spaces: the
    // exemption is computed per line and reported against the RAW text's line
    // numbers, so a blanker that dropped a newline would silently exempt the
    // wrong line. Asserted structurally so the check says what it means.
    const blanked = (source: string, extension?: string) => blankComments(source, extension).split('\n')

    expect(blanked('a\n// b\nc'), 'a line comment leaves an empty line where it was').toEqual(['a', '    ', 'c'])
    expect(blanked('a\n/* b\nc */\nd'), 'a block comment blanks every line it spans').toEqual(['a', '    ', '    ', 'd'])
    expect(blanked(`const u = 'https://example.test/x' // gone`), 'the // inside a URL string is not a comment').toEqual([`const u = 'https://example.test/x'        `])
    expect(blanked('a: 1 # note\nb: 2', '.yml')).toEqual(['a: 1       ', 'b: 2'])
    expect(blanked('a: 1 # note\nb: 2', '.ts'), 'a hash is not a comment in TypeScript').toEqual(['a: 1 # note', 'b: 2'])
    expect(blanked('<p>x</p><!-- y -->\n<p>z</p>', '.html')).toEqual(['<p>x</p>          ', '<p>z</p>'])
  })


  // ─────────────────────────────────────────────────────────────────────────
  // STORY 16.1 — THE SECOND HALF (D-16.4). The guard was AMENDED, not deleted.
  //
  // D-16.1 reversed "no live font service", so this scan's original subject is
  // gone and the naive reading is that the guard has nothing left to say. The
  // amendment is the answer: two hosts are now ALLOWED, and allowed in exactly
  // one module, so the question the scan asks changed from "is this reached at
  // all" to "is this reached from anywhere but its one declared home".
  //
  // AND IT LANDED IN THE SAME STORY AS ITS SUBJECT, deliberately. A guard that
  // arrives after the population it polices is how D-8.6.5 shipped green.
  // ─────────────────────────────────────────────────────────────────────────

  it('declares exactly the two allowed hosts as declared-only, alongside the two forbidden outright', () => {
    // Both halves are spelled in this file, so this line declares them the way
    // the scanner requires: IN CODE, on the same line, with the marker.
    expect([...declaredOnly].sort(), 'folio8:font-host-declaration').toEqual(['fonts.google.com', 'raw.githubusercontent.com'])
    expect(SCANNED_FONT_HOSTS.map((entry) => entry.half)).toEqual(['forbidden', 'forbidden', 'declared-only', 'declared-only'])
    // THE TWO HALVES ARE DISJOINT. A host in both would make the message a lie
    // whichever sentence it printed.
    expect(declaredOnly.filter((host) => hosts.includes(host))).toEqual([])
  })

  // THE POSITIVE CONTROL FOR THE NEW HALF. Without it, every assertion about
  // declared-only hosts is satisfied by a scanner that matches nothing — which
  // is exactly the vacuity the first half was written around, one host list
  // later.
  it('fails, naming the file, the line and the half, on an allowed host used outside its declaring module', () => {
    const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-font-host-declared-'))
    try {
      const git = scratchGit(scratch)
      git('init', '-q')
      const tree = path.join(scratch, 'folio-designer', 'src')
      fs.mkdirSync(tree, { recursive: true })
      // The SAME host, twice: once used, once declared. Only the used one is a
      // finding, which is what makes this a discrimination rather than a match.
      fs.writeFileSync(path.join(tree, 'second-fetch-site.ts'), `const url = 'https://${declaredOnly[0]}/google/fonts/main/ofl/x/x.ttf'\n`)
      fs.writeFileSync(path.join(tree, 'font-source.ts'), `export const host = { host: '${declaredOnly[0]}', declaration: '${DECLARATION_MARKER}' }.host\n`)
      git('add', '-A')

      const result = scanForbiddenFontHosts(scratch, { floor: 1 })
      expect(result.findings.map((finding) => finding.file)).toEqual(['folio-designer/src/second-fetch-site.ts'])
      expect(result.findings[0].half).toBe('declared-only')
      expect(() => assertNoForbiddenFontHosts(scratch, { floor: 1 })).toThrow(/folio-designer\/src\/second-fetch-site\.ts:1/)
      // AND THE TWO HALVES SAY DIFFERENT THINGS. A reader told "this host is
      // forbidden" about a host the product legitimately fetches from would
      // either delete a working call site or delete the guard.
      expect(() => assertNoForbiddenFontHosts(scratch, { floor: 1 })).toThrow(/DECLARED-ONLY/)
      expect(() => assertNoForbiddenFontHosts(scratch, { floor: 1 })).not.toThrow(/FORBIDDEN OUTRIGHT/)
    } finally {
      fs.rmSync(scratch, { recursive: true, force: true })
    }
  })

  // THE MUTATION PROOF FOR THE NEW HALF, TAKEN BY DELETING THE HALF rather than
  // by falsifying a condition: with `DECLARED_ONLY_FONT_HOSTS` empty, the same
  // offending source produces no finding at all. That is what makes the green
  // above a claim about the scanner and not about the fixture.
  it('reds only because the second half exists: removing it makes the same offender invisible', () => {
    const offender = `const url = 'https://${declaredOnly[0]}/google/fonts/main/ofl/x/x.ttf'`
    expect(occurrencesIn(offender).map((finding) => finding.host)).toEqual([declaredOnly[0]])
    // The deletion, simulated over the same matcher the scan uses: a host in
    // neither list is not a finding, whatever it is.
    expect(occurrencesIn(offender.replace(declaredOnly[0], 'example.test')).length).toBe(0)
  })

  // AND THE MARKER DIRECTION IS PRESERVED ACROSS THE NEW HALF. The scan runs
  // over RAW source while the exemption runs over COMMENT-BLANKED source, so a
  // declaration written in a comment declares nothing — for an allowed host
  // exactly as for a forbidden one. A comment is precisely where someone parks
  // a second fetch site.
  it('honours a declared-only host declared in code and refuses the same declaration written in a comment', () => {
    for (const host of declaredOnly) {
      const inCode = `const entry = { host: '${host}', declaration: '${DECLARATION_MARKER}' }`
      expect(exemptLineNumbers(inCode, host)).toEqual(new Set([1]))
      expect(occurrencesIn(inCode)).toEqual([])

      const inComment = `// '${host}' ${DECLARATION_MARKER}`
      expect(exemptLineNumbers(inComment, host)).toEqual(new Set())
      expect(occurrencesIn(inComment).length).toBe(1)

      const commentedOut = `// const url = 'https://${host}/x'`
      expect(occurrencesIn(commentedOut).length, 'a commented-out use still fails: comments are stripped from the EXEMPTION, not from the scan').toBe(1)
    }
  })

  // THE REAL SUBJECT, IN THE REAL TREE. `src/font-source.ts` is the single
  // declared home of every allowed host, and this asserts that it EXISTS and
  // really carries the declarations — a second half whose only subject was a
  // fixture would be green over a repository that had none.
  it('finds its declared subject in the real source, and reds when that subject stops declaring', () => {
    const fontSource = fs.readFileSync(path.join(here, 'font-source.ts'), 'utf8')
    for (const host of declaredOnly) {
      expect(exemptLineNumbers(fontSource, host, '.ts').size, `${host} must be declared in code in src/font-source.ts`).toBeGreaterThan(0)
      expect(occurrencesIn(fontSource, '.ts').filter((finding) => finding.host === host)).toEqual([])
    }
    // AND STRIPPING THE DECLARATION REDS IT. The marker is removed from the
    // module's own source and the same scanner is re-run over it: every host it
    // spells becomes a finding. This is the red-proof for the half, taken by
    // removing the declaration rather than by weakening the check.
    const undeclared = fontSource.split(DECLARATION_MARKER).join('undeclared')
    const findings = occurrencesIn(undeclared, '.ts')
    expect(findings.length, 'with its declarations stripped, font-source.ts must fail the scan').toBeGreaterThanOrEqual(declaredOnly.length)
    expect([...new Set(findings.map((finding) => finding.host))].sort()).toEqual([...declaredOnly].sort())
    expect(findings.every((finding) => finding.half === 'declared-only')).toBe(true)
  })

  it('scans the source extensions a font host would realistically be typed into', () => {
    for (const extension of ['.ts', '.tsx', '.js', '.mjs', '.go', '.css', '.html', '.json']) expect(SCANNED_EXTENSIONS).toContain(extension)
  })

  // ─────────────────────────────────────────────────────────────────────────
  // STORY 15.2b — THE SCAN SEES THE WHOLE TREE, NOT JUST THE TRACKED HALF.
  //
  // The walk read `git ls-files` alone, which is TRACKED FILES ONLY, and no
  // story in this project stages anything — so every file a story wrote was
  // invisible to this scan for the story's entire life. Story 12.2 added five
  // files and the scan reported clean over a tree containing none of them.
  //
  // WHY THE PROOF IS HERMETIC AND NOT A FIXTURE IN THIS REPOSITORY. After the
  // widening, an untracked file containing a forbidden host is EXACTLY what
  // this scan is designed to catch, so planting one here would redden every
  // concurrent story's build gate, and an interrupted prover would leave it
  // behind to poison every later scan. A committed fixture cannot serve
  // either: THE PROPERTY UNDER TEST IS BEING UNTRACKED, which a committed
  // fixture cannot have. So the proof runs against a throwaway repository
  // under `os.tmpdir()`, removed in `finally` — the shape four arms above
  // already use, plus one real commit.
  // ─────────────────────────────────────────────────────────────────────────

  it('reads an UNTRACKED file and reds on a forbidden host in one, which is the property the widening buys', () => {
    const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-font-host-untracked-'))
    try {
      // `-c user.name/user.email/commit.gpgsign` because a machine with signing
      // configured either fails the commit or hangs on a passphrase prompt, and
      // a test that hangs on someone else's laptop is a test that gets deleted.
      const git = scratchGit(scratch)
      git('init', '-q')
      const tree = path.join(scratch, 'folio-designer', 'src')
      fs.mkdirSync(tree, { recursive: true })
      fs.writeFileSync(path.join(tree, 'committed-clean.ts'), 'export const ok = 1\n')
      git('add', '-A')
      git('commit', '-q', '--no-verify', '-m', 'one clean committed file')
      // AND THIS ONE IS NEVER STAGED. It is what a story in this project looks
      // like for its whole life.
      fs.writeFileSync(path.join(tree, 'never-staged.ts'), `const a = 1\nconst url = 'https://${hosts[1]}/s/inter/v13/x.woff2'\n`)

      // (a) THE POSITIVE CONTROL ON THE POPULATION ITSELF, asserted before the
      // finding. Without it a temp directory swept up by someone's global
      // gitignore would produce an empty untracked half and read as a pass.
      // One binding, two assertions: `scannedPopulation` spawns two git
      // subprocesses per call, and calling it twice for two reads of the same
      // list buys nothing.
      const population = scannedPopulation(scratch)
      expect(population, 'the untracked file must be IN the population — that is the widening, and everything below is downstream of it').toContain('folio-designer/src/never-staged.ts')
      expect(population).toContain('folio-designer/src/committed-clean.ts')

      // (b) AND THE SCAN REDS, NAMING path:line rather than a count.
      expect(() => assertNoForbiddenFontHosts(scratch, { floor: 1 })).toThrow(/folio-designer\/src\/never-staged\.ts:2/)

      // (c) THE DELTA, ASSERTED WHERE IT IS FALSIFIABLE. On this repository
      // `git ls-files --others --exclude-standard` returns 0 under the scanned
      // roots, so a clean-tree "files === tracked + untracked" check is
      // `n === n + 0`: true, and proof of nothing. Here the untracked half is
      // exactly one file, so both halves of the sum carry weight.
      const result = scanForbiddenFontHosts(scratch, { floor: 1 })
      expect(result.tracked, 'the committed file is the tracked half').toBe(1)
      expect(result.untracked, 'the never-staged file is the untracked half').toBe(1)
      expect(result.files).toBe(result.tracked + result.untracked)
      expect(result.findings.map((finding) => `${finding.file}:${finding.line}`)).toEqual(['folio-designer/src/never-staged.ts:2'])
    } finally {
      fs.rmSync(scratch, { recursive: true, force: true })
    }
  })

  // AND THE SECOND GIT CALL IS NOT EXEMPT FROM THE FIRST'S RULE. A failing or
  // unobtainable `--others` listing must throw, never silently degrade to the
  // tracked list: the whole point of the widening is that "clean" starts
  // meaning "I looked", and a quiet fallback would restore the exact collapse
  // it was written to close.
  it('refuses, rather than falling back to the tracked listing, when a listing cannot be obtained', () => {
    const notARepository = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-font-host-others-'))
    try {
      // ⚠ THIS ARM IS ABOUT THE **TRACKED** HALF, AND SAYING SO IS THE POINT.
      // Measured: in a directory git knows nothing about, the FIRST call fails,
      // so this refusal is the tracked half's. It used to be labelled as the
      // `--others` proof, which it never was; the `--others` proof is the arm
      // below, where only the second call is made to fail.
      expect(() => scannedPopulation(notARepository)).toThrow(/could not look/)
      expect(() => scannedPopulation(notARepository)).toThrow(/tracked half/)
    } finally {
      fs.rmSync(notARepository, { recursive: true, force: true })
    }
  })

  // AND THE SECOND GIT CALL IS NOT EXEMPT FROM THE FIRST'S RULE, PROVED
  // BEHAVIOURALLY RATHER THAN BY GREPPING THE SCANNER'S OWN SOURCE.
  //
  // A failing `--others` listing must throw, never silently degrade to the
  // tracked list: the whole point of the widening is that "clean" starts meaning
  // "I looked", and a quiet fallback would restore the exact collapse it was
  // written to close. An arm that only pointed a non-repository at the scanner
  // asserted the tracked half's refusal and then fell back to `toContain` over
  // the scanner's source text — which stays green through a `run` rewritten to
  // catch and fall back, because source text is not behaviour. So only the
  // second call is broken here, with a `git` shim on `PATH`.
  it('refuses, naming the untracked half, when ONLY the --others listing cannot be obtained', () => {
    const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-font-host-others-fail-'))
    try {
      const git = scratchGit(scratch)
      git('init', '-q')
      const tree = path.join(scratch, 'folio-designer', 'src')
      fs.mkdirSync(tree, { recursive: true })
      fs.writeFileSync(path.join(tree, 'committed-clean.ts'), 'export const ok = 1\n')
      git('add', '-A')
      git('commit', '-q', '--no-verify', '-m', 'one clean committed file')
      fs.writeFileSync(path.join(tree, 'never-staged.ts'), 'export const authored = 1\n')

      // THE CONTROL: with the real git, this population is obtainable and holds
      // both halves. Without it a shim that broke every invocation would look
      // identical to one that broke only `--others`.
      const population = scannedPopulation(scratch)
      expect(population).toContain('folio-designer/src/committed-clean.ts')
      expect(population, 'the untracked half is really being read before it is broken').toContain('folio-designer/src/never-staged.ts')

      withFailingUntrackedListing(() => {
        // It THROWS — it does not return the tracked half as if it were the
        // tree. That is the whole property.
        expect(() => scannedPopulation(scratch)).toThrow(/could not look/)
        // AND THE REFUSAL NAMES THE HALF IT COULD NOT OBTAIN, and echoes the
        // command, so a reader is told which listing failed rather than left to
        // guess.
        expect(() => scannedPopulation(scratch)).toThrow(/untracked-but-not-ignored half/)
        expect(() => scannedPopulation(scratch)).toThrow(/ls-files --others/)
        expect(() => scannedPopulation(scratch)).toThrow(/must NOT degrade to the other listing/)
        // And the whole scan, not just the population reader, refuses.
        expect(() => assertNoForbiddenFontHosts(scratch, { floor: 1 })).toThrow(/could not look/)
      })

      // AND `PATH` IS BACK, so a later arm in this file is not scanning through
      // a broken git.
      expect(scannedPopulation(scratch)).toContain('folio-designer/src/never-staged.ts')
    } finally {
      fs.rmSync(scratch, { recursive: true, force: true })
    }
  })

  // AND EACH HALF STILL NAMES ITSELF IN THE SCANNER'S TEXT. Kept as a SECONDARY
  // check only: the two arms above are the load-bearing ones, because a string
  // present in a source file proves nothing about which line executes.
  it('names both halves in its refusals', () => {
    const source = fs.readFileSync(path.join(here, '..', 'scripts', 'forbidden-font-hosts.mjs'), 'utf8')
    for (const half of ['tracked half', 'untracked-but-not-ignored half']) expect(source, `the refusal must be able to name the ${half}`).toContain(half)
  })

  // AN EMPTY TRACKED HALF IS ITS OWN REFUSAL, and this arm exists because the
  // widening nearly deleted it. Before Story 15.2b the walk threw whenever
  // `git ls-files` was empty; folded into a both-halves-empty check, a
  // repository with an empty or unreadable INDEX over a full worktree would
  // proceed, report `tracked: 0`, and scan the untracked half as though it were
  // the tree — with only the population floor behind it.
  it('refuses a repository whose index is empty, rather than reporting the untracked half as the tree', () => {
    const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-font-host-no-index-'))
    try {
      const git = scratchGit(scratch)
      git('init', '-q')
      const tree = path.join(scratch, 'folio-designer', 'src')
      fs.mkdirSync(tree, { recursive: true })
      // A full worktree, and nothing whatsoever in the index.
      for (const name of ['a.ts', 'b.ts', 'c.ts']) fs.writeFileSync(path.join(tree, name), 'export const ok = 1\n')

      expect(() => scannedPopulation(scratch)).toThrow(/could not look/)
      expect(() => scannedPopulation(scratch)).toThrow(/git tracks no file at all/)
      // THE CONTROL: stage one file and the same tree is a population again, so
      // the refusal is about the empty index and not about the fixture.
      git('add', 'folio-designer/src/a.ts')
      expect(scannedPopulation(scratch)).toContain('folio-designer/src/a.ts')
    } finally {
      fs.rmSync(scratch, { recursive: true, force: true })
    }
  })

  // AN UNTRACKED NESTED REPOSITORY IS A SUBTREE THIS WALK CANNOT READ, AND IT
  // MUST SAY SO RATHER THAN DROP IT.
  //
  // Measured: `git ls-files --others --exclude-standard` reports an untracked
  // nested repository as ONE entry with a trailing slash and lists none of the
  // files inside it. That entry then fails the population filter's `isFile()`
  // test and disappears silently — files unread while the guard claims
  // whole-tree coverage, which is the exact collapse this story exists to close,
  // arriving through the half this story added.
  it('refuses an untracked nested repository it cannot recurse into, naming the path', () => {
    const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-font-host-nested-'))
    try {
      const git = scratchGit(scratch)
      git('init', '-q')
      const tree = path.join(scratch, 'folio-designer', 'src')
      fs.mkdirSync(tree, { recursive: true })
      fs.writeFileSync(path.join(tree, 'committed-clean.ts'), 'export const ok = 1\n')
      git('add', '-A')
      git('commit', '-q', '--no-verify', '-m', 'one clean committed file')
      // THE CONTROL, taken before the nested repository exists: this tree is
      // readable, so the refusal below is caused by the nesting and nothing else.
      expect(scannedPopulation(scratch)).toContain('folio-designer/src/committed-clean.ts')

      const nested = path.join(tree, 'vendor')
      fs.mkdirSync(nested, { recursive: true })
      scratchGit(nested)('init', '-q')
      fs.writeFileSync(path.join(nested, 'inner.ts'), 'export const inner = 1\n')

      expect(() => scannedPopulation(scratch)).toThrow(/could not look/)
      expect(() => scannedPopulation(scratch)).toThrow(/folio-designer\/src\/vendor\//)
      expect(() => scannedPopulation(scratch)).toThrow(/never read/)
    } finally {
      fs.rmSync(scratch, { recursive: true, force: true })
    }
  })

  // AND THE UNTRACKED HALF IS `--exclude-standard`, WHICH IS WHAT KEEPS THE
  // BUILD'S OWN OUTPUT OUT OF IT. Asserted behaviourally in a throwaway
  // repository rather than by reading the flag off the source: an ignored
  // untracked file must stay out of the population while a non-ignored one
  // enters it. This is the hermetic form of the `src/generated/` exemption the
  // arm below measures against the real tree.
  it('reads untracked files that are not ignored, and leaves ignored ones out', () => {
    const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-font-host-exclude-'))
    try {
      const git = scratchGit(scratch)
      git('init', '-q')
      const tree = path.join(scratch, 'folio-designer', 'src')
      fs.mkdirSync(path.join(tree, 'generated'), { recursive: true })
      fs.writeFileSync(path.join(scratch, '.gitignore'), 'folio-designer/src/generated/emitted.ts\n')
      fs.writeFileSync(path.join(tree, 'committed.ts'), 'export const ok = 1\n')
      git('add', '-A')
      git('commit', '-q', '--no-verify', '-m', 'a committed file and an ignore line')
      fs.writeFileSync(path.join(tree, 'authored.ts'), 'export const authored = 1\n')
      // What a build emits: untracked AND ignored. It must not enter, or every
      // new generated artifact reds a guard nobody touched.
      fs.writeFileSync(path.join(tree, 'generated', 'emitted.ts'), 'export const emitted = 1\n')

      const population = scannedPopulation(scratch)
      expect(population, 'an untracked file nobody ignored is read').toContain('folio-designer/src/authored.ts')
      expect(population, 'an untracked file the ignore list names is NOT read').not.toContain('folio-designer/src/generated/emitted.ts')
      const result = scanForbiddenFontHosts(scratch, { floor: 1 })
      expect(result.untracked, 'exactly one of the two untracked files joins the population').toBe(1)
      expect(result.files).toBe(result.tracked + result.untracked)
    } finally {
      fs.rmSync(scratch, { recursive: true, force: true })
    }
  })

  // THE EXEMPTION LIST, RE-CHECKED BY MEASUREMENT RATHER THAN ASSUMED.
  //
  // The widening has a side effect: the build writes into
  // `folio-designer/src/generated/`, and those artifacts are kept out of git
  // ONE FILENAME AT A TIME rather than by folder — with `pdfjs-assets.ts`
  // deliberately kept IN. Until this story they could not reach the scan
  // whatever `.gitignore` said, because they were untracked. Now the ignore
  // lines are the only thing keeping them out, so they are load-bearing and
  // are measured here instead of taken on trust.
  //
  // WHY IT MATTERS, MEASURED: across the entire populated generated tree there
  // is EXACTLY ONE occurrence of any scanned host — the family-index host, in
  // the generated header COMMENT of `font-index.ts` — and the scan reads raw
  // text while exempting only a marker written in code, so that file would be a
  // `declared-only` FINDING the moment its ignore line lapsed. The generator
  // passes the scan and its output would not. `src/build-wasm.test.ts` is what
  // stops a new emission arriving without a line.
  it('measures, rather than assumes, that the generated artifacts stay out of the widened population', () => {
    const generated = 'folio-designer/src/generated'
    const ignored = (relative: string) => {
      try {
        execFileSync('git', ['-C', repoRoot, 'check-ignore', '-q', '--', relative], { stdio: ['ignore', 'pipe', 'pipe'] })
        return true
      } catch (error) {
        // rc 1 is "not ignored", which is an answer. Anything else is git
        // failing, and a guard must never read a broken tool as a clean result.
        if ((error as { status?: number }).status === 1) return false
        throw error
      }
    }
    const tracked = (relative: string) => {
      try {
        execFileSync('git', ['-C', repoRoot, 'ls-files', '--error-unmatch', '--', relative], { stdio: ['ignore', 'pipe', 'pipe'] })
        return true
      } catch (error) {
        if ((error as { status?: number }).status === 1) return false
        throw error
      }
    }

    for (const artifact of ['runtime/folio8-engine.wasm', 'offline-assets.ts', 'runtime-fonts.css', 'font-catalogue.ts', 'font-index.ts']) {
      expect(ignored(`${generated}/${artifact}`), `${generated}/${artifact} is emitted by the build and MUST be git-ignored, or the widened scan reads a build artifact as source`).toBe(true)
      expect(tracked(`${generated}/${artifact}`), `${generated}/${artifact} must not be tracked`).toBe(false)
    }
    // THE ONE DELIBERATE EXCEPTION, asserted as an exception rather than merely
    // omitted from the list above. It has no ignore line on purpose, and it is
    // in the population BECAUSE IT IS TRACKED, exactly as it already was.
    expect(ignored(`${generated}/pdfjs-assets.ts`), 'pdfjs-assets.ts is the deliberately tracked generated artifact and must have no ignore line').toBe(false)
    expect(tracked(`${generated}/pdfjs-assets.ts`), 'pdfjs-assets.ts must be tracked, which is why it is in the population').toBe(true)

    // AND THE END-TO-END MEASUREMENT, when a local build has populated the tree:
    // the ignored artifacts are absent from the population the scan actually
    // reads, and the tracked one is present.
    const population = new Set(scannedPopulation(repoRoot))
    expect(population, 'the deliberately tracked artifact is in the population, as it already was').toContain(`${generated}/pdfjs-assets.ts`)
    for (const artifact of ['offline-assets.ts', 'runtime-fonts.css', 'font-catalogue.ts', 'font-index.ts']) {
      expect(population.has(`${generated}/${artifact}`), `${generated}/${artifact} must stay out of the population its ignore line keeps it out of`).toBe(false)
    }
  })

  // AND THE MEASURED REASON THE FONT-INDEX LINE IS LOAD-BEARING, read off the
  // emitted file: it carries exactly one scanned host, in a comment, which the
  // scan does not exempt.
  //
  // ⚠ THE SKIP IS VISIBLE, WHICH IS THE WHOLE REASON IT IS `skipIf` AND NOT AN
  // `if`. This arm can only run where a local build has emitted the file, and it
  // used to be an `if (existsSync(...))` INSIDE the arm above: on a fresh clone
  // the assertions simply did not run and the arm reported green, which is a
  // measurement that silently stops measuring — the failure mode this whole file
  // is written against. Named as a skip, a reader is told the reason instead.
  it.skipIf(!fs.existsSync(path.join(repoRoot, 'folio-designer/src/generated', 'font-index.ts')))(
    'measures the one scanned host the generated family index carries (skipped until `npm run build:wasm` has emitted src/generated/font-index.ts)',
    () => {
      const fontIndex = path.join(repoRoot, 'folio-designer/src/generated', 'font-index.ts')
      expect(fs.existsSync(fontIndex), 'the skip condition and the arm must agree on which file they are about').toBe(true)
      const occurrences = occurrencesIn(fs.readFileSync(fontIndex, 'utf8'), '.ts')
      expect(occurrences.length, 'the generated family index carries exactly one scanned host, in its header comment — which is why its ignore line is load-bearing rather than tidy').toBe(1)
      expect(occurrences[0].half).toBe('declared-only')
      expect(declaredOnly, 'and the host it carries is one of the declared-only pair, never a forbidden one').toContain(occurrences[0].host)
    },
  )
})
