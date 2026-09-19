import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { blankComments } from '../scripts/forbidden-font-hosts.mjs'

// STORY 15.2b — WHAT THE BUILD EMITS MUST BE A SUBSET OF WHAT `.gitignore`
// IGNORES, AND THE ONE EXCEPTION MUST BE NAMED AS AN EXCEPTION.
//
// WHY THIS FILE EXISTS AT ALL, AND WHY IT ARRIVES WITH THAT STORY RATHER THAN
// LATER. Story 15.2b widened the font-host scans' population from `git ls-files`
// to `git ls-files` UNION `git ls-files --others --exclude-standard`, so an
// untracked file is read the moment it is written. That closed a real hole —
// every file a story wrote used to be invisible to the guard for the story's
// whole life — and opened a small trap in the same motion: the build writes
// into `folio-designer/src/generated/`, those artifacts are kept out of git ONE
// FILENAME AT A TIME rather than by folder, and the NEXT generated file
// somebody adds now walks straight into the scan and reds a guard they never
// touched.
//
// AND IT IS NOT HYPOTHETICAL. Measured across the entire populated generated
// tree there is EXACTLY ONE occurrence of any scanned font host — the
// family-index host, in the generated header COMMENT of `font-index.ts`. The
// scan reads raw text and exempts only a marker written IN CODE, so that file
// would be a `declared-only` finding the moment its ignore line lapsed, while
// `build-font-index.mjs`, which composes that comment from `familyIndexHost` in
// `src/font-source.ts`, spells the host zero times. THE GENERATOR PASSES THE
// SCAN AND ITS OUTPUT WOULD NOT. (The emitted stylesheet is sometimes described
// as a second carrier; measured, `runtime-fonts.css` carries no host at all —
// its `url()`s are local — so the snapshot is the sole carrier.)
//
// WHY THE EMITTED SET IS READ OFF THE EMITTERS RATHER THAN DECLARED HERE. A
// hand-maintained list of generated artifacts would be a THIRD place for the
// truth to live, and the thing it is meant to catch — somebody adding an
// emission — is exactly the moment they would forget to update it. The
// emitter's own source and `git check-ignore` are both authorities that cannot
// be forgotten. `build-wasm.mjs` cannot be imported (it runs `go env GOROOT`
// and the whole build at module top level), so its SOURCE TEXT is the subject.
//
// AND THE TEXT IS COMMENT-BLANKED FIRST, with the blanker the font-host scan
// already exports: a raw-text scan counts a commented-out `writeFileSync` as a
// live emission.

const here = path.dirname(fileURLToPath(import.meta.url))
const repoRoot = path.join(here, '..', '..')
const scriptsDir = path.join(here, '..', 'scripts')
const generated = 'folio-designer/src/generated'

/** The one emitted artifact deliberately kept OUT of `.gitignore` and tracked instead. */
const DELIBERATELY_TRACKED = 'pdfjs-assets.ts'

/**
 * Every path under `src/generated/` an emitter writes, extracted from its
 * comment-blanked source.
 *
 * The two shapes the emitters use, and the only two:
 *   `join(generatedDir, '<entry>')`                    — `build-wasm.mjs`
 *   `join(designerRoot, 'src', 'generated'[, '<entry>'])` — the directory
 *      binding in `build-wasm.mjs`, and `build-font-index.mjs`'s file.
 */
const emittedPaths = (source: string): string[] => {
  const blanked = blankComments(source, '.mjs')
  const found = [
    ...[...blanked.matchAll(/join\(\s*generatedDir\s*,\s*'([^']+)'\s*\)/g)].map((match) => match[1]),
    ...[...blanked.matchAll(/join\(\s*designerRoot\s*,\s*'src'\s*,\s*'generated'\s*(?:,\s*'([^']+)'\s*)?\)/g)].map((match) => match[1]),
  ]
  // The bare directory binding captures nothing and emits nothing.
  return found.filter((entry): entry is string => entry !== undefined)
}

/** A path segment the source computes at run time, so this file cannot name it. */
const DYNAMIC = '\u0000dynamic'

/**
 * `join(...)` arguments resolved to path SEGMENTS relative to the designer root,
 * or `undefined` when the base is a binding this resolver does not know.
 *
 * A non-literal argument becomes `DYNAMIC` rather than aborting the resolution:
 * `join(outputDir, directory)` still tells us the write lands somewhere under
 * `src/generated/runtime/`, which is all the coverage check below needs.
 */
const resolveJoin = (args: string, bindings: ReadonlyMap<string, readonly string[]>): string[] | undefined => {
  const parts = args.split(',').map((part) => part.trim()).filter((part) => part !== '')
  const base = parts.length > 0 ? bindings.get(parts[0]) : undefined
  if (base === undefined) return undefined
  const segments = [...base]
  for (const part of parts.slice(1)) {
    const literal = /^'([^']*)'$/.exec(part)
    if (literal === null) segments.push(DYNAMIC)
    else if (literal[1] === '..') segments.pop()
    else segments.push(literal[1])
  }
  return segments
}

/**
 * Every `const NAME = join(...)` in the source, resolved to segments relative to
 * the designer root — `designerRoot` itself seeded as the empty path.
 *
 * WHY BINDINGS AND NOT TWO HARDCODED NAMES. The extractor above reads two
 * literal shapes, and the blindness detector used to be keyed on the same two
 * name spellings — so `const emitDir = join(designerRoot, 'src', 'generated')`
 * followed by `writeFileSync(join(emitDir, 'seventh.ts'), …)` was neither
 * extracted NOR reported: the closed set silently missed an emission and the
 * detector meant to catch that silently missed it too. Resolving the bindings is
 * what makes an alias visible, whatever it is called.
 *
 * Resolution runs to a fixed point because `outputDir = join(generatedDir,
 * 'runtime')` can only resolve after `generatedDir` has. A base built by a call
 * (`join(dirname(fileURLToPath(import.meta.url)), '..')`) is deliberately left
 * unresolved — `designerRoot` is seeded instead, and any OTHER such base yields
 * `undefined`, which the write-site check reads as "not known to be in the
 * generated tree" rather than as an emission.
 */
const resolveDirectoryBindings = (blanked: string): Map<string, readonly string[]> => {
  const bindings = new Map<string, readonly string[]>([['designerRoot', []]])
  const declarations = [...blanked.matchAll(/(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*join\(([^()]*)\)/g)]
  for (let pass = 0; pass <= declarations.length; pass++) {
    let changed = false
    for (const [, name, args] of declarations) {
      if (bindings.has(name)) continue
      const segments = resolveJoin(args, bindings)
      if (segments === undefined) continue
      bindings.set(name, segments)
      changed = true
    }
    if (!changed) break
  }
  return bindings
}

/** The top-level entry under `src/generated/` a resolved path lands in, if any. */
const generatedEntryOf = (segments: readonly string[] | undefined): string | undefined =>
  segments !== undefined && segments[0] === 'src' && segments[1] === 'generated' ? segments[2] : undefined

/** Every destination a `writeFileSync`/`copyFileSync` line could be writing to. */
const destinationsOn = (line: string, bindings: ReadonlyMap<string, readonly string[]>): (readonly string[] | undefined)[] => [
  ...[...line.matchAll(/join\(([^()]*)\)/g)].map((match) => resolveJoin(match[1], bindings)),
  // A bare binding as the destination: `writeFileSync(generatedPath, …)`,
  // `copyFileSync(wasmExec, gluePath)`.
  ...[...line.matchAll(/[A-Za-z_$][\w$]*/g)].map((match) => bindings.get(match[0])),
]

/**
 * Write sites that land in the generated tree in a shape the extractor above did
 * NOT read, so the extraction cannot go quietly blind.
 *
 * THE EXTRACTED SET IS CHECKED AS A CLOSED SET, NOT MERELY FED TO A SUBSET TEST
 * — a subset assertion over an extraction that silently stopped matching is
 * green over nothing. This is the other half of that.
 *
 * ⚠ IT IS KEYED ON WRITE SITES, NOT ON LINES MENTIONING THE TREE. An earlier
 * shape filtered lines by the two binding spellings and by `join(`, which made
 * it blind to exactly the case it existed to catch (an unrecognised directory
 * binding, see `resolveDirectoryBindings`) and, worse, would report a script
 * that merely READS the generated tree — `scripts/wasm-vcs-stamp.test.mjs` does
 * — as an unparsed emission. Only `writeFileSync`/`copyFileSync` can introduce
 * an artifact, so only they are examined; a destination that resolves under an
 * entry the extractor already found (`src/generated/runtime/…`) is covered by
 * that entry.
 *
 * What it still cannot see, stated rather than implied: a destination assembled
 * by string concatenation rather than `join`, and a base bound through a call
 * this resolver does not evaluate.
 */
const unparsedReferences = (source: string): string[] => {
  const blanked = blankComments(source, '.mjs')
  const bindings = resolveDirectoryBindings(blanked)
  const extracted = new Set(emittedPaths(source))
  const reported: string[] = []
  blanked.split('\n').forEach((raw, index) => {
    const line = raw.trim()
    if (!/\b(?:writeFileSync|copyFileSync)\s*\(/.test(line)) return
    for (const segments of destinationsOn(line, bindings)) {
      const entry = generatedEntryOf(segments)
      // `undefined` — the write is not in the generated tree, or is the
      // directory itself, which emits no artifact.
      if (entry === undefined) continue
      if (entry !== DYNAMIC && extracted.has(entry)) continue
      reported.push(`${index + 1}: ${line}`)
      return
    }
  })
  return [...new Set(reported)]
}

const ignored = (relative: string): boolean => {
  try {
    execFileSync('git', ['-C', repoRoot, 'check-ignore', '-q', '--', relative], { stdio: ['ignore', 'pipe', 'pipe'] })
    return true
  } catch (error) {
    // rc 1 is "not ignored", which is an answer. Anything else is git failing,
    // and a guard must never read a broken tool as a clean result.
    if ((error as { status?: number }).status === 1) return false
    throw error
  }
}

const tracked = (relative: string): boolean => {
  try {
    execFileSync('git', ['-C', repoRoot, 'ls-files', '--error-unmatch', '--', relative], { stdio: ['ignore', 'pipe', 'pipe'] })
    return true
  } catch (error) {
    if ((error as { status?: number }).status === 1) return false
    throw error
  }
}

/**
 * The path to hand `git check-ignore`, WITH A TRAILING SLASH FOR A DIRECTORY
 * EMISSION.
 *
 * MEASURED 2026-09-05 in a scratch repository whose only pattern is `dir/`:
 * with the directory ABSENT, `git check-ignore -q -- a/dir` exits **1 — not
 * ignored** — while `a/dir/` exits **0**. With the directory present both exit
 * 0. A `dir/` pattern matches only a directory, and with nothing on disk git has
 * no way to know the path is one unless the slash says so.
 *
 * ⚠ THIS IS NOT TIDINESS, AND THE SLASH MUST NOT BE "SIMPLIFIED" AWAY. `runtime`
 * is a directory emission — `const outputDir = join(generatedDir, 'runtime')` in
 * `build-wasm.mjs` — and `.gitignore` ignores it as `…/generated/runtime/`. Probed
 * without the slash on any tree where `npm run build:wasm` has not yet run, it
 * comes back unignored and this file reds for a reason with nothing to do with
 * the code under test. CI never saw it only because `build:wasm` precedes
 * `vitest` there. The positive control is measured in its own arm below: with the
 * directory absent, a name nothing ignores still exits 1 WITH the slash, so the
 * slash cannot turn an unignored path into an ignored one.
 */
const ignoreProbePath = (entry: string): string => `${generated}/${entry}${entry.includes('.') ? '' : '/'}`

/**
 * The assertion itself, over an arbitrary emitted set, so the red-proof below
 * can run the REAL check against a mutated copy of the source rather than
 * against a re-implementation of it.
 */
const offendersIn = (emitted: readonly string[]): string[] => emitted
  .filter((entry) => entry !== DELIBERATELY_TRACKED)
  .filter((entry) => !ignored(ignoreProbePath(entry)))

/**
 * Every script under `scripts/` that could emit into the generated tree,
 * DISCOVERED rather than listed.
 *
 * WHY IT IS A GLOB. This file's own header argues that a hand-maintained list of
 * generated artifacts is a third place for the truth to live; a hand-maintained
 * list of EMITTERS is the same defect one level up. Pinned to two filenames, a
 * third script under `scripts/` writing into `src/generated/` was read by
 * nothing and reddened nothing. The membership test runs over COMMENT-BLANKED
 * source for the same reason the extraction does: a script that only mentions
 * the generated tree in prose is not an emitter.
 */
const emitters: ReadonlyArray<readonly [string, string]> = fs.readdirSync(scriptsDir)
  .filter((name) => name.endsWith('.mjs'))
  .sort()
  .map((name) => [name, fs.readFileSync(path.join(scriptsDir, name), 'utf8')] as const)
  .filter(([, source]) => /generatedDir|'src'\s*,\s*'generated'|src\/generated\//.test(blankComments(source, '.mjs')))

const sourceOf = (name: string): string => {
  const found = emitters.find(([emitter]) => emitter === name)
  if (found === undefined) throw new Error(`${name} is not among the discovered emitters (${emitters.map(([emitter]) => emitter).join(', ')}), so the discovery below is not finding what this file is about`)
  return found[1]
}

const buildWasmSource = sourceOf('build-wasm.mjs')
const buildFontIndexSource = sourceOf('build-font-index.mjs')

describe('everything the build emits into src/generated/ is ignored, except the one tracked on purpose', () => {
  // THE CLOSED SET. Not "at least these" — exactly these, so a SEVENTH emission
  // reds here naming itself, and so does an extraction that quietly stopped
  // matching and returned five.
  // THE EMITTER SET IS DISCOVERED, AND THE DISCOVERY HAS ITS OWN CONTROL. A glob
  // that matched nothing would make every assertion below vacuous, so the two
  // scripts this file is actually about are asserted to be in what it found.
  it('discovers the scripts that write into the generated tree, rather than naming two by hand', () => {
    const names = emitters.map(([name]) => name)
    expect(names, 'the emitter glob must find the two scripts this file exists for').toEqual(expect.arrayContaining(['build-wasm.mjs', 'build-font-index.mjs']))
    expect(names.length, 'a glob that found nothing would make every assertion in this file vacuous').toBeGreaterThanOrEqual(2)
    // AND IT IS A GLOB OVER THE DIRECTORY, so a third emitter is picked up the
    // day it is written. Measured: every `.mjs` under `scripts/` whose
    // comment-blanked source references the generated tree is read.
    const everyScript = fs.readdirSync(scriptsDir).filter((name) => name.endsWith('.mjs'))
    expect(everyScript.length, 'the glob must be reading the real scripts directory').toBeGreaterThan(names.length)
    for (const [name, source] of emitters) {
      expect(blankComments(source, '.mjs'), `${name} was discovered as an emitter, so its blanked source must really reference the generated tree`).toMatch(/generatedDir|'src'\s*,\s*'generated'|src\/generated\//)
    }
  })

  it('extracts exactly the eight emissions the build writes, from the emitters own source', () => {
    const emitted = emitters.flatMap(([, source]) => emittedPaths(source)).sort()
    expect(emitted, 'the emitted set is read from the emitters, so a new artifact appears here the moment it is written. If this reds with a new name, that artifact needs a .gitignore line (or a deliberate decision to track it, like pdfjs-assets.ts).').toEqual([
      'documentation-assets.ts',
      'example-assets.ts',
      'font-catalogue.ts',
      'font-index.ts',
      'offline-assets.ts',
      'pdfjs-assets.ts',
      'runtime',
      'runtime-fonts.css',
    ])
    // AND `build-font-index.mjs` IS READ TOO, or `font-index.ts` — the one
    // emitted file that really does carry a scanned host — would be missed.
    expect(emittedPaths(buildFontIndexSource), 'the font-index emitter is a second source of emissions and must be read').toEqual(['font-index.ts'])
  })

  it('leaves no reference to the generated tree unparsed, so the extraction cannot go blind', () => {
    for (const [name, source] of emitters) {
      expect(unparsedReferences(source), `${name} writes into ${generated}/ in a shape this test cannot read, so the extracted set above may be missing an artifact. Teach the extractor the shape, or use one it already knows.`).toEqual([])
    }
    // POSITIVE CONTROL 1 — A NESTED EMISSION, a shape neither extractor regex
    // reads. Without a control the green above is satisfied by a detector that
    // reports nothing.
    const nested = `${buildWasmSource}\nwriteFileSync(join(generatedDir, 'nested', 'deep.ts'), '')\n`
    expect(emittedPaths(nested), 'the extractor genuinely cannot read this shape — that is what makes it the control').not.toContain('nested')
    expect(unparsedReferences(nested).join('\n'), 'a path shape the extractor cannot read must be named, never silently skipped').toContain("join(generatedDir, 'nested', 'deep.ts')")

    // POSITIVE CONTROL 2 — AN ALIASED DIRECTORY BINDING, which is the shape the
    // previous detector was blind to in BOTH directions: not extracted, and not
    // reported either, so the closed set quietly lost an emission. The alias is
    // a different NAME for the same directory, so nothing about it is exotic —
    // it is one rename away at any time.
    const aliased = `${buildWasmSource}\nconst emitDir = join(designerRoot, 'src', 'generated')\nwriteFileSync(join(emitDir, 'seventh.ts'), '')\n`
    expect(emittedPaths(aliased), 'the extractor genuinely cannot read the aliased binding — that is what makes it the control').not.toContain('seventh.ts')
    expect(unparsedReferences(aliased).join('\n'), 'a write through an unrecognised directory binding must be named, never silently missed').toContain("join(emitDir, 'seventh.ts')")

    // AND THE DETECTOR DISCRIMINATES: a script that only READS the generated
    // tree emits nothing and must not be reported. `scripts/wasm-vcs-stamp.test.mjs`
    // is the real instance — it reads the emitted wasm out of
    // `src/generated/runtime/` — and a detector keyed on "mentions the tree"
    // rather than on write sites reported it.
    const readsOnly = "const runtimeDirectory = join(designerRoot, 'src', 'generated', 'runtime')\nconst bytes = readFileSync(join(runtimeDirectory, 'folio8-engine.wasm'))\n"
    expect(unparsedReferences(readsOnly), 'reading the generated tree introduces no artifact and must not be reported as one').toEqual([])
  })

  // THE TRAILING SLASH ON A DIRECTORY PROBE, MEASURED RATHER THAN ASSERTED IN
  // PROSE. `runtime` is a directory emission and `.gitignore` ignores it as
  // `…/generated/runtime/`; probed without the slash on a tree where the build
  // has not run, git answers "not ignored" and the subset assertion reds for a
  // reason unconnected to the code. This arm reproduces the measurement in a
  // throwaway repository WITH THE DIRECTORY ABSENT, which is the only state in
  // which the two probes disagree.
  it('probes a directory emission with a trailing slash, because check-ignore answers differently without one', () => {
    const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-check-ignore-'))
    try {
      // The same settings the other two Story 15.2b test files pin on every
      // scratch repository (R2): `core.hooksPath=/dev/null` so an `init.templateDir`
      // that installs hooks cannot run somebody's script here, and
      // `init.defaultBranch` so `git init` prints no advisory. Nothing is
      // committed in this arm, so there is no commit to pass `--no-verify`.
      execFileSync('git', ['-C', scratch, '-c', 'core.hooksPath=/dev/null', '-c', 'init.defaultBranch=main', 'init', '-q'], { stdio: ['ignore', 'pipe', 'pipe'] })
      fs.writeFileSync(path.join(scratch, '.gitignore'), 'a/dir/\n')
      const probe = (relative: string): number => {
        try {
          execFileSync('git', ['-C', scratch, 'check-ignore', '-q', '--', relative], { stdio: ['ignore', 'pipe', 'pipe'] })
          return 0
        } catch (error) {
          return (error as { status?: number }).status ?? -1
        }
      }
      expect(fs.existsSync(path.join(scratch, 'a', 'dir')), 'the directory must be ABSENT — that is the whole state under test').toBe(false)
      expect(probe('a/dir'), 'without the slash and without the directory on disk, git reports a `dir/` pattern as NOT ignored').toBe(1)
      expect(probe('a/dir/'), 'with the slash, the same pattern matches').toBe(0)
      // THE POSITIVE CONTROL FOR THE SLASH ITSELF: it cannot manufacture an
      // ignore. A name nothing ignores stays unignored with or without it.
      expect(probe('a/other'), 'a name nothing ignores is not ignored').toBe(1)
      expect(probe('a/other/'), 'and the trailing slash does not make it so').toBe(1)
      // AND ONCE THE DIRECTORY EXISTS, both agree — which is why this only ever
      // bit a tree where `npm run build:wasm` had not run.
      fs.mkdirSync(path.join(scratch, 'a', 'dir'), { recursive: true })
      expect(probe('a/dir')).toBe(0)
      expect(probe('a/dir/')).toBe(0)
      // AND THE HELPER THIS FILE USES ADDS THE SLASH FOR A DIRECTORY EMISSION
      // AND ONLY FOR ONE.
      expect(ignoreProbePath('runtime')).toBe(`${generated}/runtime/`)
      expect(ignoreProbePath('font-index.ts')).toBe(`${generated}/font-index.ts`)
    } finally {
      fs.rmSync(scratch, { recursive: true, force: true })
    }
  })

  // THE SUBSET RELATION, MEASURED AGAINST `.gitignore` VIA GIT RATHER THAN BY
  // READING THE FILE. `git check-ignore` is the authority on what is ignored;
  // parsing `.gitignore` here would be a second implementation of git's rules.
  it('proves every emitted artifact is git-ignored, and names the deliberate exception rather than omitting it', () => {
    const emitted = emitters.flatMap(([, source]) => emittedPaths(source))
    expect(offendersIn(emitted), `these artifacts are emitted into ${generated}/ but nothing ignores them. Since Story 15.2b the font-host scans read untracked files, so an unignored emission enters the scan the moment the build writes it and reds a guard its author never touched. Add a line to .gitignore's WASM-build-output block, or justify tracking it the way ${DELIBERATELY_TRACKED} is justified.`).toEqual([])

    // AND THE EXCEPTION IS ASSERTED AS AN EXCEPTION. Omitting it from the list
    // above would leave the reader unable to tell a deliberate decision from an
    // oversight, which is the whole failure mode this file is about.
    expect(emitted, `${DELIBERATELY_TRACKED} must still be an emission — the exception only means anything while the thing it excepts is real`).toContain(DELIBERATELY_TRACKED)
    expect(ignored(ignoreProbePath(DELIBERATELY_TRACKED)), `${DELIBERATELY_TRACKED} is the ONE generated artifact deliberately kept out of .gitignore: it stays in Vite's asset graph and is committed. If this reds, someone added an ignore line for it.`).toBe(false)
    expect(tracked(`${generated}/${DELIBERATELY_TRACKED}`), `${DELIBERATELY_TRACKED} is tracked on purpose, which is the only reason it is in the font-host scan's population`).toBe(true)
  })

  // THE RED-PROOF, TAKEN BY ADDING A SEVENTH EMISSION TO A COPY OF THE SOURCE.
  // Never to the real file: the emitters run in `npm run build`, and a scratch
  // artifact left behind by an interrupted prover would poison every later run.
  it('reds naming the artifact, when a seventh emission arrives without an ignore line', () => {
    const seventh = 'seventh-artifact.ts'
    const mutated = `${buildWasmSource}\nwriteFileSync(join(generatedDir, '${seventh}'), 'export const seventh = 1\\n')\n`
    const emitted = [...emittedPaths(mutated), ...emitters.filter(([name]) => name !== 'build-wasm.mjs').flatMap(([, source]) => emittedPaths(source))]

    expect(emitted, 'the extractor must see the new emission at all, or the check below is vacuous').toContain(seventh)
    // THE REAL CHECK, over the mutated set: it fails, and it NAMES the artifact.
    expect(offendersIn(emitted)).toEqual([seventh])
    // And the six real emissions are still clean under the same call, so the
    // red discriminates rather than merely firing.
    expect(offendersIn(emitted).filter((entry) => entry !== seventh)).toEqual([])
    // A commented-out emission is NOT one, which is why the source is blanked
    // before extraction rather than read raw.
    expect(emittedPaths(`${buildWasmSource}\n// writeFileSync(join(generatedDir, '${seventh}'), '')\n`)).not.toContain(seventh)
  })
})
