import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import {
  HOST_FONT_ACCESS_APIS,
  HOST_FONT_DECLARATION_MARKER,
  POPULATION_FLOOR,
  SCANNED_ROOTS,
  assertNoHostFontAccess,
  occurrencesIn,
  scanHostFontAccess,
  scannedPopulation,
} from '../scripts/host-font-access.mjs'

// STORY 16.2, AC4's SECOND HALF — NO CODE PATH IN THIS DESIGNER ENUMERATES,
// READS OR FEATURE-DETECTS HOST FONTS.
//
// WHY THE ASSERTION IS SOURCE-LEVEL AND OVER THE WHOLE TREE. The claim is about
// the DESIGNER, not about one module: a test that only checked `font-store.ts`
// would pass while `App.tsx` called the API. So the scan reads every tracked
// source file under the designer, and this file drives the REAL scanner rather
// than a copy of its logic.
//
// AND IT IS RED-PROVED BY DELETING THE GUARD, NEVER BY FALSIFYING A CONDITION.
// These spellings appear ZERO times in this repository, so "it passed" is not
// evidence of anything on its own — a scan that read nothing, matched nothing,
// or exempted everything would pass identically. Three tests below are the
// evidence instead: a positive control, a population floor, and the comment
// direction. Emptying `HOST_FONT_ACCESS_APIS`, or deleting
// `assertNoHostFontAccess`, reds them.
//
// THE CLAIM IS BOUNDED (D-8.5.5). A green here means none of these spellings
// appears in the scanned population. It is NOT a proof that no host font is ever
// read — a source scan cannot see a name assembled at runtime or a call inside a
// dependency — and no test here is written as if it were. (This sentence used to
// end "or anything in an untracked file". STORY 15.2b made that clause false:
// the walk now unions `git ls-files --others --exclude-standard` into the
// listing, so the untracked-but-not-ignored tree is read. The two remaining
// clauses are still true and still written.)

const here = path.dirname(fileURLToPath(import.meta.url))
const repoRoot = path.join(here, '..', '..')

/**
 * `git`, inside a throwaway repository, with every setting pinned that could
 * make one of these arms fail or HANG on somebody else's laptop (R2).
 *
 * `user.name`/`user.email` because a machine with no configured identity cannot
 * commit. `commit.gpgsign=false` because a machine with signing on either fails
 * the commit or blocks on a passphrase prompt. `core.hooksPath=/dev/null`
 * because a global `core.hooksPath`, or an `init.templateDir` that installs
 * hooks, would run somebody's own pre-commit script inside this scratch
 * repository — the commits below also pass `--no-verify`, the same refusal said
 * twice on purpose. `init.defaultBranch` because otherwise `git init` prints an
 * advisory on every run.
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
 * non-repository it is the FIRST call — the tracked listing — that fails, so the
 * refusal names the TRACKED half and the `--others` refusal is never reached.
 * An arm written that way would stay green through a `--others` call rewritten
 * to catch and fall back to the tracked listing, which is exactly the collapse
 * Story 15.2b exists to close, and this guard is the one that already had the
 * hole once. Making only the second call fail is the only way to put the second
 * refusal on the path the proof exercises.
 *
 * `PATH` is restored in `finally`, and the real `git` is resolved BEFORE the
 * shim is prepended so the shim's `exec` cannot recurse into itself.
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

describe('no code path in the designer reaches for host-installed fonts', () => {
  // THE SPELLINGS ARE PINNED, so a list quietly emptied to make a red go away
  // reds HERE instead. This is the assertion that makes every clean result
  // below mean something.
  it('forbids the Local Font Access surface by every spelling it goes by', () => {
    // Each is spelled on a line carrying the marker in code, which is how this
    // test file names what it forbids without being a violation of it. A marker
    // in a comment would declare nothing — see the comment-direction test.
    const declared = [
      { name: 'queryLocalFonts', declaration: 'folio8:host-font-declaration' },
      { name: 'navigator.fonts', declaration: 'folio8:host-font-declaration' },
      { name: 'local-fonts permission', declaration: 'folio8:host-font-declaration' },
      { name: 'FontData', declaration: 'folio8:host-font-declaration' },
    ]
    expect(HOST_FONT_ACCESS_APIS.map((api) => api.name)).toEqual(declared.map((api) => api.name))
    expect(HOST_FONT_DECLARATION_MARKER).toBe('folio8:host-font-declaration')
    for (const api of HOST_FONT_ACCESS_APIS) expect(api.declaration).toBe(HOST_FONT_DECLARATION_MARKER)
  })

  // THE REAL SCAN, OVER THE REAL TREE. The population is asserted first, because
  // a finding count is only meaningful next to what was read.
  it('reads a real population of the designer and finds no host-font access', () => {
    const result = scanHostFontAccess(repoRoot)
    expect(result.files, `the scan read only ${result.files} files, which cannot support a claim about this designer`).toBeGreaterThan(POPULATION_FLOOR)
    expect(result.findings, 'the designer must never enumerate, read or feature-detect a font installed on the machine (SPEC-fonts Non-goal: "No host fonts")').toEqual([])
    // The two halves account for the whole population and nothing is counted
    // twice (Story 15.2b). ⚠ THIS IS NOT THE DELTA PROOF: on a committed tree
    // the untracked half is 0 and this reduces to `n === n + 0`, true and proof
    // of nothing. The falsifiable version lives in the hermetic arm below.
    expect(result.files).toBe(result.tracked + result.untracked)
    // AND IT SPANS EVERY TREE IT CLAIMS TO. The expectation is DERIVED from
    // `SCANNED_ROOTS` rather than written out here, so adding a root the walk
    // cannot reach reds, and so does reaching a directory no root declares.
    const reached = [...new Set(scannedPopulation(repoRoot).map((file) => file.split('/').slice(0, 2).join('/')))].sort()
    expect(reached, 'every declared scan root must contribute at least one file, and nothing outside them may').toEqual([...SCANNED_ROOTS].sort())
  })

  // AND THE GUARD READS ITS OWN FILE, WHICH IS WHY EVERY SPELLING ABOVE IS
  // SPLIT.
  //
  // A guard proved once, before the file that drives it existed, is not a guard
  // that is still reading. This asserts the scanned population CONTAINS this
  // test file, so the whole-tree claim covers the one file in the tree most
  // likely to spell the forbidden API — and so the day somebody "fixes" a red
  // here by excluding the test file from the walk, this reds instead of going
  // quiet. It is also the standing reason the expected values in this file are
  // written split rather than whole.
  it('scans its own file, so the guard is not exempt from the rule it enforces', () => {
    expect(scannedPopulation(repoRoot), 'the host-font guard must be inside the population it scans').toContain('folio-designer/src/host-font-access.test.ts')
    // AND SO IS THE SCANNER IT DRIVES, for the same reason.
    expect(scannedPopulation(repoRoot)).toContain('folio-designer/scripts/host-font-access.mjs')
  })

  // 1 — THE POSITIVE CONTROL. Without this, every assertion above is satisfied
  // by a scanner that matches nothing at all.
  it('fails, naming the file and the line, on a source file that calls the API', () => {
    const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-host-font-'))
    try {
      const git = scratchGit(scratch)
      git('init', '-q')
      const tree = path.join(scratch, 'folio-designer', 'src')
      fs.mkdirSync(tree, { recursive: true })
      fs.writeFileSync(path.join(tree, 'clean.ts'), 'export const ok = 1\n')
      fs.writeFileSync(path.join(tree, 'offender.ts'), 'const ok = 1\nconst faces = await window.query' + 'LocalFonts()\n')
      git('add', '-A')

      const result = scanHostFontAccess(scratch, { floor: 1 })
      // THE SPELLING IS SPLIT HERE FOR THE SAME REASON IT IS SPLIT ABOVE. This
      // file is inside the scanned population (asserted below), and the scan
      // reads RAW text, so writing the API whole in an expected value would make
      // this file a real occurrence and red the whole-tree scan against itself.
      // Splitting it is the file's own established technique; excluding this
      // file from the scan would be the weaker fix, because a guard that does
      // not read itself is a guard nobody is holding.
      expect(result.findings.map((finding) => `${finding.file}:${finding.line}:${finding.api}`)).toEqual(['folio-designer/src/offender.ts:2:query' + 'LocalFonts'])
      // The thrown message NAMES the file and the line, not merely a count.
      expect(() => assertNoHostFontAccess(scratch, { floor: 1 })).toThrow(/folio-designer\/src\/offender\.ts:2/)
      // And the clean file is not reported, so the matcher discriminates.
      expect(result.findings.map((finding) => finding.file)).not.toContain('folio-designer/src/clean.ts')
    } finally {
      fs.rmSync(scratch, { recursive: true, force: true })
    }
  })

  // AND IT DISCRIMINATES AGAINST A REAL NEIGHBOUR IN THIS REPOSITORY. PDF.js's
  // `standardFontDataUrl` contains the interface name this API exposes, and a
  // substring scan would report the PDF preview as a host-font reader. This is
  // a MEASURED collision, not a precaution: that option is passed at
  // `src/preview/pdf-viewer.tsx`.
  it('does not read a font-shaped identifier that is not this API', () => {
    const neighbours = 'const options = { standardFontDataUrl: url, cMapUrl: url }\nconst family = "local-fonts-are-not-read-here"\n'
    expect(occurrencesIn(neighbours, '.ts')).toEqual([])
    // AND THE REAL NEIGHBOUR, READ AS ITSELF RATHER THAN QUOTED. The page font
    // set is the registry a document's OWN carried faces are registered into,
    // which is the exact opposite of reading a font off the machine — and the
    // one module in this build that touches it must stay legal. Its source is
    // the corpus, so a widened pattern that swept it up reds here.
    const registry = fs.readFileSync(path.join(here, 'embedded-face-registry.ts'), 'utf8')
    expect(registry, 'the neighbour corpus must be the module that really does register faces').toContain('registerCarriedFaces')
    expect(occurrencesIn(registry, '.ts')).toEqual([])
  })

  // 2 — THE POPULATION FLOOR. A scan that read almost nothing is
  // indistinguishable from a scan that found nothing, and it must not be
  // allowed to report the second.
  it('refuses to report a clean population over an implausibly small one', () => {
    const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-host-font-floor-'))
    try {
      const git = scratchGit(scratch)
      git('init', '-q')
      const tree = path.join(scratch, 'folio-designer', 'src')
      fs.mkdirSync(tree, { recursive: true })
      fs.writeFileSync(path.join(tree, 'only.ts'), 'export const ok = 1\n')
      git('add', '-A')
      // Clean by content, and still refused, because one file is not a population.
      expect(scanHostFontAccess(scratch, { floor: 1 }).findings).toEqual([])
      expect(() => assertNoHostFontAccess(scratch)).toThrow(/under its floor/)
    } finally {
      fs.rmSync(scratch, { recursive: true, force: true })
    }
  })

  // 3 — THE COMMENT DIRECTION, which is the half that is easy to get backwards.
  // The SCAN reads raw text, so a commented-out call still fails. What is
  // stripped is the EXEMPTION: a marker written inside a comment declares
  // nothing, because a comment is exactly where somebody parks a call they mean
  // to make later.
  it('fails on a commented-out call, and refuses a declaration written in a comment', () => {
    const spelling = 'query' + 'LocalFonts'
    expect(occurrencesIn(`// const faces = await ${spelling}()\n`, '.ts')).toHaveLength(1)
    // A marker in a comment on the same line exempts NOTHING.
    expect(occurrencesIn(`// ${spelling}() ${HOST_FONT_DECLARATION_MARKER}\n`, '.ts')).toHaveLength(1)
    // In code, on the same line, it does — which is how the scanner and this
    // file are able to name what they forbid.
    expect(occurrencesIn(`const forbidden = { api: '${spelling}', declaration: '${HOST_FONT_DECLARATION_MARKER}' }\n`, '.ts')).toEqual([])
  })

  // ─────────────────────────────────────────────────────────────────────────
  // STORY 15.2b — THIS GUARD HAD ALREADY BEEN FOOLED BY THE HOLE IT NOW CLOSES.
  //
  // Read `POPULATION_FLOOR`'s docblock in `scripts/host-font-access.mjs`: the
  // figure it had to correct was measured while Story 16.2's own six new files
  // were still untracked, so the walk — which read `git ls-files` — could not
  // see them. The scanner's own refusal has said *"an unobtainable population
  // must never read as all-clear"* since the day it shipped, AND IT STILL HAD
  // THE HOLE. This arm is the one that would have caught it.
  //
  // HERMETIC, for the same reason the sibling scan's is: after the widening, an
  // untracked file spelling this API is exactly what the guard catches, so
  // planting one in this repository would redden every concurrent story's build
  // gate, and a committed fixture cannot serve because THE PROPERTY UNDER TEST
  // IS BEING UNTRACKED.
  // ─────────────────────────────────────────────────────────────────────────
  it('reads an UNTRACKED file and reds on a Local Font Access call in one', () => {
    const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-host-font-untracked-'))
    try {
      // `-c user.name/user.email/commit.gpgsign` because a machine with signing
      // configured either fails the commit or hangs on a passphrase prompt.
      const git = scratchGit(scratch)
      git('init', '-q')
      const tree = path.join(scratch, 'folio-designer', 'src')
      fs.mkdirSync(tree, { recursive: true })
      fs.writeFileSync(path.join(tree, 'committed-clean.ts'), 'export const ok = 1\n')
      git('add', '-A')
      git('commit', '-q', '--no-verify', '-m', 'one clean committed file')
      // NEVER STAGED — and the spelling is split for the same reason every
      // other expected value in this file is: this file is itself inside the
      // scanned population.
      fs.writeFileSync(path.join(tree, 'never-staged.ts'), 'const ok = 1\nconst faces = await window.query' + 'LocalFonts()\n')

      // (a) THE POSITIVE CONTROL ON THE POPULATION ITSELF, first: without it a
      // temp directory swept up by someone's global gitignore reads as a pass.
      // One binding, two assertions: each `scannedPopulation` call spawns two
      // git subprocesses, and reading the same list twice buys nothing.
      const population = scannedPopulation(scratch)
      expect(population, 'the untracked file must be IN the population — that is the widening').toContain('folio-designer/src/never-staged.ts')
      expect(population).toContain('folio-designer/src/committed-clean.ts')

      // (b) AND THE SCAN REDS, NAMING path:line.
      expect(() => assertNoHostFontAccess(scratch, { floor: 1 })).toThrow(/folio-designer\/src\/never-staged\.ts:2/)

      // (c) THE DELTA, ASSERTED WHERE IT IS FALSIFIABLE. On this repository the
      // untracked half is empty, so a clean-tree sum proves nothing.
      const result = scanHostFontAccess(scratch, { floor: 1 })
      expect(result.tracked).toBe(1)
      expect(result.untracked, 'the never-staged file is the untracked half').toBe(1)
      expect(result.files).toBe(result.tracked + result.untracked)
    } finally {
      fs.rmSync(scratch, { recursive: true, force: true })
    }
  })

  // AND THE SECOND GIT CALL IS NOT EXEMPT FROM THE FIRST'S RULE. A failing or
  // unobtainable `--others` listing must throw, never silently degrade to the
  // tracked list: the whole point of the widening is that "clean" starts
  // meaning "I looked", and a quiet fallback would restore the exact collapse
  // it was written to close — a scan that could not look reading as all-clear.
  it('refuses, rather than falling back to the tracked listing, when a listing cannot be obtained', () => {
    const notARepository = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-host-font-others-'))
    try {
      // ⚠ THIS ARM IS ABOUT THE **TRACKED** HALF, AND SAYING SO IS THE POINT.
      // Measured: in a directory git knows nothing about the FIRST call fails,
      // so this refusal is the tracked half's. It used to be labelled the
      // `--others` proof, which it never was — the `--others` proof is the arm
      // below, where only the second call is made to fail.
      expect(() => scannedPopulation(notARepository)).toThrow(/could not look/)
      expect(() => scannedPopulation(notARepository)).toThrow(/tracked half/)
    } finally {
      fs.rmSync(notARepository, { recursive: true, force: true })
    }
  })

  // AND THE SECOND GIT CALL IS NOT EXEMPT FROM THE FIRST'S RULE, PROVED
  // BEHAVIOURALLY RATHER THAN BY GREPPING THE SCANNER'S OWN SOURCE. This guard
  // is the one that stated the principle in its message and still had the hole,
  // so an arm here that only re-read the message would repeat the original
  // mistake in test form.
  it('refuses, naming the untracked half, when ONLY the --others listing cannot be obtained', () => {
    const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-host-font-others-fail-'))
    try {
      const git = scratchGit(scratch)
      git('init', '-q')
      const tree = path.join(scratch, 'folio-designer', 'src')
      fs.mkdirSync(tree, { recursive: true })
      fs.writeFileSync(path.join(tree, 'committed-clean.ts'), 'export const ok = 1\n')
      git('add', '-A')
      git('commit', '-q', '--no-verify', '-m', 'one clean committed file')
      fs.writeFileSync(path.join(tree, 'never-staged.ts'), 'export const authored = 1\n')

      // THE CONTROL: with the real git both halves are obtainable, so a shim
      // that broke everything could not pass as one that broke only `--others`.
      const population = scannedPopulation(scratch)
      expect(population).toContain('folio-designer/src/committed-clean.ts')
      expect(population, 'the untracked half is really being read before it is broken').toContain('folio-designer/src/never-staged.ts')

      withFailingUntrackedListing(() => {
        // It THROWS — it does not report the tracked half as if it were the
        // tree. That is the whole property.
        expect(() => scannedPopulation(scratch)).toThrow(/could not look/)
        expect(() => scannedPopulation(scratch)).toThrow(/untracked-but-not-ignored half/)
        expect(() => scannedPopulation(scratch)).toThrow(/ls-files --others/)
        expect(() => scannedPopulation(scratch)).toThrow(/must NOT degrade to the other listing/)
        expect(() => assertNoHostFontAccess(scratch, { floor: 1 })).toThrow(/could not look/)
      })

      // AND `PATH` IS BACK, so no later arm scans through a broken git.
      expect(scannedPopulation(scratch)).toContain('folio-designer/src/never-staged.ts')
    } finally {
      fs.rmSync(scratch, { recursive: true, force: true })
    }
  })

  // AND EACH HALF STILL NAMES ITSELF IN THE SCANNER'S TEXT. SECONDARY only: a
  // string present in a source file proves nothing about which line executes.
  it('names both halves in its refusals', () => {
    const source = fs.readFileSync(path.join(here, '..', 'scripts', 'host-font-access.mjs'), 'utf8')
    for (const half of ['tracked half', 'untracked-but-not-ignored half']) expect(source, `the refusal must be able to name the ${half}`).toContain(half)
  })

  // THE UNTRACKED HALF IS `--exclude-standard`, AND THAT FLAG IS ASSERTED
  // BEHAVIOURALLY HERE RATHER THAN LEFT TO THE SIBLING SCAN.
  //
  // Measured before this arm existed: deleting `--exclude-standard` from this
  // scanner's `--others` call reddened NOTHING. Its hermetic arm builds a
  // scratch repository with no `.gitignore` at all, so the flag has nothing to
  // do there, and on the real tree the extra ignored build output happens to
  // spell none of the four Local Font Access APIs. A flag no test can see is a
  // flag the next refactor deletes — and deleting it would pull the whole
  // ignored build tree into the population the day one of those artifacts
  // spelled one of these names.
  it('reads untracked files that are not ignored, and leaves ignored ones out', () => {
    const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-host-font-exclude-'))
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
      // new generated artifact reds a guard nobody touched. The spelling is
      // split for the reason every expected value in this file is: this file is
      // itself inside the scanned population.
      fs.writeFileSync(path.join(tree, 'generated', 'emitted.ts'), 'const faces = await window.query' + 'LocalFonts()\n')

      const population = scannedPopulation(scratch)
      expect(population, 'an untracked file nobody ignored is read').toContain('folio-designer/src/authored.ts')
      expect(population, 'an untracked file the ignore list names is NOT read').not.toContain('folio-designer/src/generated/emitted.ts')
      const result = scanHostFontAccess(scratch, { floor: 1 })
      expect(result.untracked, 'exactly one of the two untracked files joins the population').toBe(1)
      expect(result.files).toBe(result.tracked + result.untracked)
      // AND THE IGNORED FILE'S CALL IS NOT A FINDING, because it was never read.
      // Without `--exclude-standard` it would be one, which is what makes this
      // the behavioural proof of the flag rather than a restatement of it.
      expect(result.findings, 'an ignored build artifact must not be scanned at all').toEqual([])
    } finally {
      fs.rmSync(scratch, { recursive: true, force: true })
    }
  })

  // AN EMPTY TRACKED HALF IS ITS OWN REFUSAL. Folded into a both-halves-empty
  // check, a repository with an empty or unreadable INDEX over a full worktree
  // would proceed, report `tracked: 0`, and treat the untracked half as the
  // tree, with only the population floor behind it.
  it('refuses a repository whose index is empty, rather than reporting the untracked half as the tree', () => {
    const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-host-font-no-index-'))
    try {
      const git = scratchGit(scratch)
      git('init', '-q')
      const tree = path.join(scratch, 'folio-designer', 'src')
      fs.mkdirSync(tree, { recursive: true })
      for (const name of ['a.ts', 'b.ts', 'c.ts']) fs.writeFileSync(path.join(tree, name), 'export const ok = 1\n')

      expect(() => scannedPopulation(scratch)).toThrow(/could not look/)
      expect(() => scannedPopulation(scratch)).toThrow(/git tracks no file at all/)
      // THE CONTROL: stage one file and the same tree is a population again.
      git('add', 'folio-designer/src/a.ts')
      expect(scannedPopulation(scratch)).toContain('folio-designer/src/a.ts')
    } finally {
      fs.rmSync(scratch, { recursive: true, force: true })
    }
  })

  // AN UNTRACKED NESTED REPOSITORY IS A SUBTREE THIS WALK CANNOT READ, AND IT
  // MUST SAY SO RATHER THAN DROP IT. Measured: `git ls-files --others
  // --exclude-standard` reports one as a single entry with a trailing slash and
  // lists none of the files inside it; that entry then fails the population
  // filter's `isFile()` test and disappears silently.
  it('refuses an untracked nested repository it cannot recurse into, naming the path', () => {
    const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-host-font-nested-'))
    try {
      const git = scratchGit(scratch)
      git('init', '-q')
      const tree = path.join(scratch, 'folio-designer', 'src')
      fs.mkdirSync(tree, { recursive: true })
      fs.writeFileSync(path.join(tree, 'committed-clean.ts'), 'export const ok = 1\n')
      git('add', '-A')
      git('commit', '-q', '--no-verify', '-m', 'one clean committed file')
      // THE CONTROL, before the nested repository exists.
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
})
