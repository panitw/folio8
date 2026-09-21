import { execFileSync } from 'node:child_process'
import crypto from 'node:crypto'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// THE JOIN NOTHING IN THIS CODEBASE PERFORMS.
//
// `scripts/build-wasm.mjs` holds the two halves of the browser's font identity
// in two places that never meet. Its `assets` object maps OPAQUE SLOT NAMES
// (`sans`, `sansCjk`, `sansThai`, `mono`, `plexSans`, `plexSansThai`) to source
// file paths and carries no family name at all; its `runtime-fonts.css`
// template maps FAMILY NAMES to those slots by hand, with no loop. Nothing
// composes the two, so nothing had ever been able to say WHICH FILE IS BEHIND
// A FAMILY NAME — and the browser declares two vocabularies over that join:
// the design system's three IBM Plex families, which every `--type-*` token
// resolves through, and the engine's three face names, which the canvas asks
// for because AD-17 makes the browser a rasterizer only.
//
// This file performs the join and pins its result. Three claims:
//
//   1. THE TWO VOCABULARIES ARE SEPARATE BY DESIGN. Story 8.4b declared both
//      over the SAME three files — a deliberate INTERVAL, in which the IBM
//      Plex names were IBM Plex in name only. Story 8.4c ended it: one rule
//      per family, each over a distinct file, no family sharing bytes with
//      another. It was six rules over six files then and is THIRTEEN over
//      thirteen since Story 11.1 — the invariant is one-file-per-family, not
//      the count. A file reached by two family names is now a defect rather
//      than the expected state, and the assertion below is where that is
//      caught.
//
//   2. THE FACE THE ENGINE MEASURED. Every family named after a shipped face
//      is declared from bytes IDENTICAL to the bytes `folio-go/fonts/fonts.go`
//      embeds under that same face name. AD-17 makes the browser a rasterizer
//      only; a family called `Noto Sans` that is not the engine's `Noto Sans`
//      is worse than no family at all, because it fails silently.
//
//   3. THE BYTES ARE THE FACE THE NAME CLAIMS. A family name in a stylesheet
//      is an assertion about bytes, and until Story 8.4c nothing in this
//      repository could check it: `IBM Plex Mono` was Noto Sans SC, a CJK sans
//      with no monospacing, and every gate stayed green. The guard below opens
//      each declared file and reads its own `name` table, so the name over the
//      rule and the name inside the file have to agree — plus, for the two
//      families whose job is more than a typeface, that the mono face is
//      fixed-pitch and the Thai face covers the strings from the recorded
//      Thai rendering defect.
//
// WHY THE GENERATOR SOURCE AND NOT ITS OUTPUT. `src/generated/runtime-fonts.css`
// and `src/generated/runtime/` are gitignored and only exist after `build:wasm`,
// so asserting against them would make this guard's strength depend on build
// order — and a missing file is the classic way a guard goes quietly vacuous.
// Every file read here is a tracked source, in either language.
const here = path.dirname(fileURLToPath(import.meta.url))
const designerRoot = path.join(here, '..')
const generatorPath = path.join(designerRoot, 'scripts', 'build-wasm.mjs')
const engineFontsDir = path.join(designerRoot, '..', 'folio-go', 'fonts')
const enginePath = path.join(engineFontsDir, 'fonts.go')
// THE OTHER HALF OF `Shipped()` (spec-deferred-offline-cache, story 5): the
// `//go:build !nocjkface` file that embeds the CJK face and merges it in. See
// `shippedFaceNames` for why this half and not its twin.
const buildTaggedFacesPath = path.join(here, '..', '..', 'folio-go', 'fonts', 'notosanssc.go')
const buildTaggedFacesSource = fs.readFileSync(buildTaggedFacesPath, 'utf8')
// `lint`'s asset licence gate, READ AS SOURCE rather than restated here.
// `manifest.ResolveAssets` (AC25, AD-26) only requires a `LICENSE*`, only
// requires a `NOTICE*`, and only writes a `lint/MANIFEST.md` row for a file
// whose extension is in ITS OWN `fontExtensions` list — so that list is the
// exact boundary of what the licence gate can SEE. Restating the list on this
// side would make the guard agree with a copy of the gate rather than with the
// gate, and the two would drift apart silently the first time either moved.
const licenceGatePath = path.join(designerRoot, '..', 'lint', 'internal', 'manifest', 'manifest.go')
// THE POPULATION THE MAGIC-BYTE SWEEP RUNS OVER — widened at Story 8.4h (AC7,
// D-8.5.2) from `folio-designer/public/fonts` to THE WHOLE TRACKED REPOSITORY.
//
// The narrow version could only ever have caught a mis-suffixed font dropped
// into the designer's own font tree, while the licence gate it mirrors
// (`manifest.ResolveAssets`) has walked the ENTIRE repository since Story 3.6.
// So the guard was blind in exactly the direction the gate was not: a `.woff2`
// committed under `folio-go/`, under `lint/testdata/`, or anywhere else was
// invisible to the gate BECAUSE OF ITS EXTENSION and invisible to this guard
// BECAUSE OF ITS PATH — the two blind spots composing into a font that ships
// with no LICENSE required, no NOTICE required and no `lint/MANIFEST.md` row.
// D-8.5.2 sends the extension-class guard repo-wide for that reason.
//
// THIS WIDENS THE GUARD'S DIRECTORY REACH, NOT ITS ASSET-CLASS REACH. The
// licence gate's own walk was already repo-wide and already filters by
// extension, so no non-font asset is newly subjected to the font allowlist by
// this change (Design Note 5).
const repoRoot = path.join(designerRoot, '..')

// The DESIGN SYSTEM's own three families — tokens.css's `--font-sans`,
// `--font-mono` and `--font-page`, through which every `--type-*` token
// resolves. They are spelled here because they are a fact about THIS
// repository's chrome, fixed by DESIGN.md and not derived from anything.
// Everything else the generator declares is claimed to be an ENGINE face name,
// and that claim is checked against `fonts.Shipped()` below rather than
// assumed. Note that this is emphatically NOT a family -> face mapping: no
// entry here says which engine face any of these corresponds to, and the pair
// structure below is derived from the generator's own slots instead.
const chromeFamilies = ['IBM Plex Sans', 'IBM Plex Mono', 'IBM Plex Sans Thai'] as const

// STORY 16.8'S ADDITION. Roboto is the first face this repository ships
// under BOTH vocabularies at once: it is a `font-catalogue.json` catalogue
// face (Story 8.5, `AVAILABLE LOCALLY` in the family control) AND, as of
// this story, an engine-shipped face (`fonts.Shipped()`), so the browser's
// `@font-face` for it comes from the CATALOGUE emitter's templated rule
// (`scripts/build-wasm.mjs`'s `catalogueFaces.map(...)` loop), never from a
// hand-written rule — a hand-written seventh rule naming the SAME family the
// catalogue already declares would be the exact duplicate-`@font-face`
// hazard `font-catalogue.json`'s own collision guard exists to refuse
// (`catalogueFamilies.has(entry.family)` in build-wasm.mjs). The catalogue's
// per-face rule is therefore invisible to `declaredFamilies`/
// `familySourcePaths` below, which see the THIRTEEN hand-written rules only, so
// the two ties this story's own boundary calls out — "does the browser have
// a face for it" and "are the two Roboto files byte-identical" — are made
// here explicitly, against the catalogue path, rather than assumed to fall
// out of the hand-written-rule machinery built for the other three faces.
const catalogueEngineRobotoFile = 'public/fonts/roboto/Roboto-Regular.ttf'

/**
 * CSS family names `font-catalogue.json` declares, read as data rather than
 * re-derived from build-wasm.mjs's loop.
 *
 * ⚠ IT RETURNS THE DERIVED CUT NAMES, NOT THE ROWS' `family` FIELD
 * (spec-install-all-face-cuts, story 3). The two were the same list while the
 * catalogue held one upright Regular per family; a row now carries a `style`
 * and the emitted `@font-face` names it `<family> <cut>`, so the question this
 * helper answers — "which families does the CATALOGUE's templated rule already
 * declare, so the hand-written-rule parse below must not require one" — is
 * about the derived name. Returning the base family would answer `Inter` for a
 * row emitted as `Inter Bold` and would leave a genuine cut name unmatched.
 *
 * It is a SET-LIKE list: `Inter` appears once here even though four rows carry
 * it, because a family's four rows derive four distinct CSS names and only the
 * Regular's is the bare family.
 */
function catalogueDeclaredFamilies(): ReadonlyArray<string> {
  const catalogue = JSON.parse(fs.readFileSync(path.join(designerRoot, 'font-catalogue.json'), 'utf8')) as ReadonlyArray<{ family: string; style: string }>
  return catalogue.map((entry) => entry.style === 'Regular' ? entry.family : `${entry.family} ${entry.style === 'BoldItalic' ? 'Bold Italic' : entry.style}`)
}

// withoutComments strips line and block comments while leaving string and
// template literals intact, so the parses below answer to the CODE that emits
// the stylesheet rather than to prose that merely spells a rule.
//
// IT IS DUPLICATED FROM canvas-font-stack.test.ts (which in turn duplicates it
// from canvas-authority-contract.test.ts), DELIBERATELY. Importing it from
// another test file would register that file's whole suite a second time under
// this one, and hoisting it into a shared non-test module would put a test
// helper into `src/` — the very production corpus those suites scan. Each guard
// stays independently self-contained: this file must be able to redden on its
// own, without depending on another suite's helper staying correct.
//
// A character scanner rather than a regex because a regex cannot tell
// `// a comment` from the `//` inside a URL string, and getting that backwards
// makes the guard vacuous exactly where it matters.
//
// WHY THIS IS LOAD-BEARING HERE, measured: with the three engine-named rules
// commented out — left in the file as comment text — every test in this file
// stayed green while the emitted stylesheet dropped to three rules, which is
// the exact state that shipped the reported Thai overlap. Red-proof below.
function withoutComments(source: string): string {
  let out = ''
  let index = 0
  let quote: string | undefined
  while (index < source.length) {
    const char = source[index] as string
    if (quote !== undefined) {
      out += char
      if (char === '\\') { out += source[index + 1] ?? ''; index += 2; continue }
      if (char === quote) quote = undefined
      index++
      continue
    }
    if (char === '"' || char === '\'' || char === '`') { quote = char; out += char; index++; continue }
    if (char === '/' && source[index + 1] === '/') { while (index < source.length && source[index] !== '\n') index++; continue }
    if (char === '/' && source[index + 1] === '*') { index += 2; while (index < source.length && !(source[index] === '*' && source[index + 1] === '/')) index++; index += 2; continue }
    out += char
    index++
  }
  return out
}

/**
 * The shape `familySourcePaths` yields for a rule whose `assets` slot did not
 * resolve. It is NOT a file path, and must never be treated as one: two rules
 * naming the same unresolvable slot would otherwise group into a well-formed
 * "pair" and satisfy the interval guard while resolving to nothing at all.
 */
const sentinelPrefix = '<no assets slot named '
const isSentinel = (file: string) => file.startsWith(sentinelPrefix)

/**
 * The `assets` half: slot name -> source file path, relative to the designer
 * root, for every slot fingerprinted out of `public/fonts`. The other slots
 * (`wasm`, `wasmExec`, `starter`) are fingerprinted from build products rather
 * than from a committed path and are deliberately not matched.
 */
function slotSourcePaths(generator: string): Readonly<Record<string, string>> {
  const entries = [...generator.matchAll(/(\w+):\s*fingerprint\(join\(designerRoot,\s*((?:'[^']*'\s*,\s*)*'[^']*')\)\s*,/g)]
  return Object.fromEntries(entries.map((match) => [
    match[1],
    [...match[2].matchAll(/'([^']*)'/g)].map((segment) => segment[1]).join('/'),
  ]))
}

// ---------------------------------------------------------------------------
// THE LICENCE GATE'S BLIND SPOT, CLOSED AS A CLASS (D-8.4.23).
//
// `lint/internal/manifest/manifest.go`'s `ResolveAssets` walks the whole
// repository and, for every directory holding a file whose extension is in
// `fontExtensions`, HARD-FAILS the build unless a `LICENSE*` and a `NOTICE*`
// sit beside it — and writes that directory a row in `lint/MANIFEST.md`. A font
// binary carrying any OTHER extension is not merely unlicensed in the
// manifest's eyes: it is INVISIBLE to it. No LICENSE required, no NOTICE
// required, no row. That is exactly the trap `@ibm/plex-*` on npm sets, since
// those packages ship `.woff2`/`.woff` and no `.ttf` at all.
//
// D-8.4.23 ruled that making this an "Always" line in one story's spec is NOT
// ENOUGH: a spec constraint binds one story, and the next person adding a font
// by a different route is not reading that spec. What is owed is a BEHAVIOURAL
// guard — a test that fails when ANY font file reaching the runtime bundle
// carries an extension the gate does not recognise. So the check below is a
// population claim over two independent populations, not a check of the
// thirteen files that happen to be here today:
//
//   1. every font asset SLOT the generator fingerprints into the runtime
//      bundle, and
//   2. every GIT-TRACKED file in the repository the licence gate walks, WHOSE
//      OWN FIRST FOUR BYTES ARE A FONT MAGIC — so a `.woff2` renamed,
//      mis-suffixed or simply dropped in anywhere is caught by what it IS
//      rather than by what it is called or where it was put. Widened from
//      `public/fonts/` to the tracked repository at Story 8.4h (AC7, D-8.5.2).
//
// The recognised set is read out of `manifest.go` itself. If someone widens
// `fontExtensions` to admit `.woff2`, this guard widens with it — which is
// correct, because at that moment the gate really can see them.
// ---------------------------------------------------------------------------

/** The extensions `lint`'s asset licence gate recognises, read from its own source. */
function licenceGateFontExtensions(manifestGo: string): ReadonlyArray<string> {
  const declaration = /var fontExtensions = \[\]string\{([^}]*)\}/.exec(manifestGo)
  if (declaration === null) throw new Error(`no 'var fontExtensions = []string{…}' declaration in ${licenceGatePath} — the licence gate's recognised set could not be read, so this guard cannot mirror it`)
  return [...declaration[1].matchAll(/"([^"]+)"/g)].map((match) => match[1].toLowerCase())
}

/**
 * A file's extension, lowercased, or `''` where it has none.
 *
 * THE `''` CASE IS NEW AT STORY 8.4h AND IS NOT COSMETIC. The previous
 * one-liner was `file.slice(file.lastIndexOf('.'))`: for an EXTENSIONLESS file
 * `lastIndexOf` returns -1, so it yielded the file's LAST CHARACTER as its
 * "extension". Under `public/fonts` no extensionless file existed and the
 * defect was latent; over the tracked repository (AC7) there are many —
 * `LICENSE`, `NOTICE`, `Makefile`, `Dockerfile`. It could never produce a false
 * positive, because the magic-byte check still gates every report, but a
 * failure message reading `(extension 'e')` is a message that sends the reader
 * the wrong way. Anchoring on the BASENAME also stops a dotted DIRECTORY name
 * (`example.test/ufl-lib/LICENSE`) from being read as the file's extension.
 */
const extensionOf = (file: string) => {
  const base = path.basename(file)
  const dot = base.lastIndexOf('.')
  // `dot === 0` is a dotfile (`.gitignore`), which has a name and no extension.
  return dot <= 0 ? '' : base.slice(dot).toLowerCase()
}

/**
 * Font asset slots whose SOURCE FILE carries an extension the licence gate does
 * not recognise. Empty is the invariant: every font the generator copies into
 * `src/generated/runtime/` — and thence into Vite's asset graph and the release
 * bundle — is a file `manifest.ResolveAssets` will demand a LICENSE and a
 * NOTICE for.
 */
function slotsInvisibleToTheLicenceGate(generator: string, recognised: ReadonlyArray<string>): ReadonlyArray<string> {
  return Object.entries(slotSourcePaths(generator))
    .filter(([, file]) => !recognised.includes(extensionOf(file)))
    .map(([slot, file]) => `${slot} -> ${file} (extension '${extensionOf(file)}')`)
}

/**
 * The four-byte sfnt/webfont signatures. `0x00010000` is TrueType outlines,
 * `true` the legacy Apple spelling, `ttcf` a collection, `OTTO` a CFF font, and
 * `wOFF`/`wOF2` the two WOFF wrappers — the last two being precisely the
 * formats the licence gate cannot see, which is why they are listed as things
 * to RECOGNISE rather than things to ignore.
 */
const fontMagics = ['\u0000\u0001\u0000\u0000', 'true', 'ttcf', 'OTTO', 'wOFF', 'wOF2']

/** Whether a file's own first four bytes say it is a font, whatever it is named. */
function looksLikeAFontBinary(file: string): boolean {
  const head = Buffer.alloc(4)
  const handle = fs.openSync(file, 'r')
  try { fs.readSync(handle, head, 0, 4, 0) } finally { fs.closeSync(handle) }
  return fontMagics.includes(head.toString('latin1'))
}

/** Every file below `directory`, recursively. */
function filesUnder(directory: string): ReadonlyArray<string> {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const full = path.join(directory, entry.name)
    return entry.isDirectory() ? filesUnder(full) : [full]
  })
}

/**
 * Files that ARE fonts by their own bytes but carry an extension the licence
 * gate does not recognise — so they would ship with no LICENSE required, no
 * NOTICE required and no `lint/MANIFEST.md` row.
 *
 * TAKES A FILE LIST, NOT A DIRECTORY, since Story 8.4h (AC7). The population is
 * no longer "everything under one path" but "everything git tracks inside the
 * gate's own walk", and those are not the same shape: a disk walk of the repo
 * root would read the three real variable TTFs in the gitignored
 * `.font-sources/` (~20 MB) plus everything under `node_modules/`, `dist/` and
 * `src/generated/` — none of which this repository redistributes. The
 * `git ls-files` intersection is therefore load-bearing, not cosmetic
 * (Design Note 5). `filesUnder` still exists and still feeds a directory into
 * this function, which is how the discrimination proof below stays honest.
 *
 * Paths are rendered relative to `root` — repo-root-relative for the real
 * population, so a report names a file the way the licence gate would.
 */
function committedFontsTheLicenceGateCannotSee(files: ReadonlyArray<string>, recognised: ReadonlyArray<string>, root: string): ReadonlyArray<string> {
  return files
    .filter((file) => !recognised.includes(extensionOf(file)) && looksLikeAFontBinary(file))
    .map((file) => `${path.relative(root, file)} (extension '${extensionOf(file)}')`)
}

/**
 * Every file the LICENCE GATE'S OWN WALK would consider, in `root`: git-tracked
 * (so untracked scratch and gitignored caches are out, exactly as
 * `manifest.ResolveAssets` excludes them via `gitTrackedFileCount`), minus the
 * two directories that walk skips — `.git`, and any `lint` directory whose
 * parent is `testdata`.
 *
 * THROWS RATHER THAN YIELDING AN EMPTY SET, in three places: if git itself
 * fails, if it lists nothing, and if nothing survives the filter. A guard that
 * cannot look must not read as all-clear — that is D-3.6.5's own ground, and
 * the whole reason the licence gate grew its scan-error floor. An enumeration
 * that quietly returned `[]` would make the assertion below pass over a
 * repository it never read.
 *
 * Vitest runs under jsdom with cwd `folio-designer/`, so `-C <root>` is
 * mandatory, not stylistic (precedent: `scripts/verify-offline-release.mjs`).
 */
function licenceGateTrackedFiles(root: string): ReadonlyArray<string> {
  let listing: string
  try {
    // stderr is CAPTURED, not inherited: the throws-proof below deliberately
    // runs this against a non-repository, and a bare `fatal:` line printed
    // into a green suite's output trains the reader to ignore them.
    listing = execFileSync('git', ['-C', root, 'ls-files', '-z'], { encoding: 'utf8', maxBuffer: 256 * 1024 * 1024, stdio: ['ignore', 'pipe', 'pipe'] })
  } catch (error) {
    throw new Error(`git ls-files failed in ${root}: ${String(error)} — the tracked population could not be obtained, and an unobtainable population must never read as all-clear`)
  }
  const tracked = listing.split('\0').filter((entry) => entry !== '')
  if (tracked.length === 0) throw new Error(`git tracks no files at all in ${root} — refusing to report a clean population over an empty one`)

  const files = tracked
    .filter(insideTheLicenceGateWalk)
    .map((relative) => path.join(root, relative))
    // A gitlink (submodule) is listed as a path that is not a regular file, and
    // an index entry can outlive its file on disk mid-rebase. Neither is
    // something to read four bytes out of.
    .filter((file) => fs.existsSync(file) && fs.statSync(file).isFile())
  if (files.length === 0) throw new Error(`no tracked regular file in ${root} survived the licence gate's walk filter — refusing to report a clean population over an empty one`)
  return files
}

/** The two exclusions `manifest.ResolveAssets`' own `filepath.WalkDir` applies. */
function insideTheLicenceGateWalk(relative: string): boolean {
  const segments = relative.split('/')
  if (segments.includes('.git')) return false
  return !segments.some((segment, index) => segment === 'lint' && segments[index - 1] === 'testdata')
}

/**
 * The `@font-face` half: family name -> the `assets` slot its `src` interpolates.
 *
 * The spelling matched here is the generator's exact, deliberate one-line-per-rule
 * form. Drift in it does not weaken this guard quietly: the non-vacuity floors
 * below assert the parse found rules at all, and the exact map assertion
 * asserts it found ALL of them.
 */
function familySlots(generator: string): ReadonlyArray<readonly [string, string]> {
  return [...generator.matchAll(/@font-face \{ font-family: '([^']+)'; src: url\('\.\/runtime\/\$\{assets\.(\w+)\}'\) format\('truetype'\); font-display: swap;(?: \$\{canvasFaceMetricOverrideCss\})? \}/g)]
    .map((match) => [match[1], match[2]] as const)
}

/** The join: family name -> the source file the browser will be handed for it. */
function familySourcePaths(generator: string): Readonly<Record<string, string>> {
  const slots = slotSourcePaths(generator)
  return Object.fromEntries(familySlots(generator).map(([family, slot]) => [family, slots[slot] ?? `${sentinelPrefix}${slot}>`]))
}

/**
 * The families that reach each SOURCE FILE, keyed by the file. The PAIR
 * STRUCTURE IS DERIVED, never listed: "two names over one file" is exactly a
 * file with two families, and a deleted rule, a repointed rule or a slot moved
 * onto another file all show up as a file with the wrong number. Listing the
 * pairs instead would be a second authority on which browser family
 * corresponds to which engine face — exactly the mapping table Story 8.4b's
 * verdict rejects by name.
 *
 * Grouped by the RESOLVED FILE rather than by the `assets` slot, deliberately:
 * families sharing a slot resolve to one path by construction, so a slot-level
 * check would be a tautology. The file is the layer the claim is about.
 */
function familiesPerSourceFile(generator: string): Readonly<Record<string, ReadonlyArray<string>>> {
  const grouped: Record<string, string[]> = {}
  for (const [family, file] of Object.entries(familySourcePaths(generator))) (grouped[file] ??= []).push(family)
  return grouped
}

const isChrome = (family: string) => (chromeFamilies as ReadonlyArray<string>).includes(family)

/**
 * Source files reached by MORE THAN ONE family name. Empty is the invariant:
 * the two-names-one-file interval Story 8.4b pinned ended with Story 8.4c, so
 * every family — three design-system, three engine — now has bytes of its own,
 * and a shared file means a rule has drifted onto another family's face.
 *
 * A NAMED HELPER RATHER THAN AN INLINE LOOP, deliberately: the real assertion
 * and the red-proof fixture below drive this one function, so the code path
 * that must redden in production is the code path shown to redden. Each
 * offender carries the families that reached it, so a failure prints WHAT
 * shares the file rather than just a count.
 *
 * EVERY UNRESOLVED SLOT IS REPORTED TOO. `familySourcePaths` falls back to a
 * sentinel string when a rule names an `assets` slot that does not exist, and a
 * sentinel groups like a path: a lone rule over a broken slot would otherwise
 * pass as a happily-unshared file resolving to no bytes at all.
 */
function filesReachedByMoreThanOneFamily(generator: string): ReadonlyArray<string> {
  return Object.entries(familiesPerSourceFile(generator))
    .filter(([file, families]) => isSentinel(file) || families.length > 1)
    .map(([file, families]) => `${file} (reached by: ${families.join(', ')})`)
}

/**
 * The families in `required` that NO `@font-face` rule declares at all.
 *
 * THE DELETION DIRECTION, WHICH THE SHARING CHECKER CANNOT SEE BY
 * CONSTRUCTION. `filesReachedByMoreThanOneFamily` reports files reached by MORE
 * than one family; a deleted rule produces FEWER families per file, never more,
 * so it is invisible to that checker no matter how the fixture is written. The
 * hazard is real and asymmetric: a rule "simplified" away leaves a family the
 * chrome or the canvas still ASKS FOR with no face behind it at all, and the
 * browser falls silently through to a generic — which is exactly the shape of
 * the Thai overlap this repository has on record.
 *
 * A NAMED HELPER, for the same reason as the one above: the production
 * assertion and the red-proof fixture drive this one function, so the code path
 * that must redden over the real generator is the code path shown to redden
 * over a fixture. Before this existed, deletion was caught only by an inline
 * count and an exact-map `toEqual` that no fixture exercised — the one guard
 * here never shown to discriminate.
 */
function familiesWithNoRule(generator: string, required: ReadonlyArray<string>): ReadonlyArray<string> {
  const declared = familySourcePaths(generator)
  return required.filter((family) => declared[family] === undefined)
}

/**
 * The face names `fonts.Shipped()` keys its FontSet by, in the order it writes
 * them — from BOTH of the files that contribute to it.
 *
 * ⚠ THE SET IS SPLIT ACROSS TWO FILES SINCE spec-deferred-offline-cache STORY
 * 5, and reading only `fonts.go` would now silently answer TEN. `Shipped()`
 * writes ten keys in its own literal and merges `buildTaggedFaces()` over them;
 * that function has a `//go:build` pair, and the `!nocjkface` half —
 * `notosanssc.go` — is the one every build but the designer's engine wasm
 * compiles. It is the half this reader takes, deliberately: what these ties are
 * about is the ELEVEN-FACE SHIPPED CONTRACT the browser must mirror, which the
 * spec leaves untouched, not the ten faces one build happens to embed. The
 * designer's stylesheet declares an `@font-face` rule for all eleven either
 * way, because the CJK one is exactly the deferred asset the engine is handed
 * at run time.
 */
function shippedFaceNames(fontsGo: string, taggedGo: string = buildTaggedFacesSource): ReadonlyArray<string> {
  const body = /func Shipped\(\) folio8\.FontSet \{[\s\S]*?\n\}/.exec(fontsGo)?.[0]
  if (body === undefined) throw new Error(`no Shipped() function in ${enginePath}`)
  const tagged = /func buildTaggedFaces\(\) map\[string\]\[\]byte \{[\s\S]*?\n\}/.exec(taggedGo)?.[0]
  if (tagged === undefined) throw new Error(`no buildTaggedFaces() function in ${buildTaggedFacesPath}`)
  return [...body.matchAll(/"([^"]+)":\s*\w+,/g)].map((match) => match[1]).concat([...tagged.matchAll(/"([^"]+)":\s*\w+\}/g)].map((match) => match[1]))
}

/**
 * Face name -> the file the engine embeds for it, joining `Shipped()`'s map
 * through the //go:embed directives.
 *
 * ⚠ IT READS BOTH FILES, for the reason `shippedFaceNames` does: since
 * spec-deferred-offline-cache story 5 the CJK face's embed directive lives in
 * `notosanssc.go` rather than in `fonts.go`, and a join over `fonts.go` alone
 * would report the face as having no embed rather than reporting the truth.
 */
function shippedFacePaths(fontsGo: string, taggedGo: string = buildTaggedFacesSource): Readonly<Record<string, string>> {
  const embedded = `${fontsGo}\n${taggedGo}`
  const embeds = Object.fromEntries([...embedded.matchAll(/\/\/go:embed\s+(\S+)\s*\nvar\s+(\w+)\s+\[\]byte/g)].map((match) => [match[2], match[1]]))
  const body = /func Shipped\(\) folio8\.FontSet \{[\s\S]*?\n\}/.exec(fontsGo)?.[0] ?? ''
  const tagged = /func buildTaggedFaces\(\) map\[string\]\[\]byte \{[\s\S]*?\n\}/.exec(taggedGo)?.[0] ?? ''
  const rows: Array<[string, string]> = [
    ...[...body.matchAll(/"([^"]+)":\s*(\w+),/g)].map((match): [string, string] => [match[1], embeds[match[2]] ?? `<no //go:embed for ${match[2]}>`]),
    ...[...tagged.matchAll(/"([^"]+)":\s*(\w+)\}/g)].map((match): [string, string] => [match[1], embeds[match[2]] ?? `<no //go:embed for ${match[2]}>`]),
  ]
  return Object.fromEntries(rows)
}

const digest = (file: string) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex')

/**
 * The sha256 a face's OWN `NOTICE.md` records for the file it ships — parsed
 * out of the provenance record rather than restated here.
 *
 * WHY THIS TIE AND NOT A LITERAL. Every committed face sits beside a NOTICE
 * that names its upstream release, the path inside that release, the fetch
 * date, and the digest of the artifact — the same `NOTICE*` file
 * `manifest.ResolveAssets` already requires to exist, which is why the record
 * is guaranteed to be there to read. Nothing asserted that the record was TRUE
 * of the bytes beside it. That gap is not theoretical: `declaredFamilyOfFile`
 * prefers nameID 16, which is IDENTICAL across every weight and style of a
 * family, so IBM Plex Sans **Bold**, SemiBold, Italic, or a `pyftsubset`
 * Latin-only cut all satisfy the name check while the whole chrome renders in
 * the wrong face — and each NOTICE's digest quietly becomes a false statement
 * about the file it sits next to. Hardcoding the digest on this side would
 * instead let a swap be laundered by editing one number in a test.
 *
 * Deliberately strict, and deliberately throwing rather than defaulting:
 * exactly one row must carry the digest. A NOTICE that stopped recording one —
 * or recorded two — is a provenance record this guard can no longer read, and
 * that must be loud rather than vacuous.
 */
function recordedShippedDigest(noticeFile: string): string {
  const notice = fs.readFileSync(noticeFile, 'utf8')
  const rows = [...notice.matchAll(/^\|[^|\n]*sha256 of the SHIPPED[^|\n]*\|\s*`([0-9a-f]{64})`\s*\|/gm)]
  if (rows.length !== 1) throw new Error(`${noticeFile} must record exactly one 'sha256 of the SHIPPED …' table row carrying a 64-hex digest, and records ${rows.length} — the provenance record for the binary beside it cannot be read`)
  return rows[0][1]
}

// ---------------------------------------------------------------------------
// THE SMALLEST sfnt READ THAT ANSWERS "WHAT DOES THIS FILE SAY IT IS".
//
// WHY HERE AND NOT IN `lint`. `lint/internal/rules` already owns "a font binary
// is what it claims to be" (`looksLikeSfnt`) and already reaches designer paths
// elsewhere, so it was the obvious home. It was rejected on a measured ground:
// `lint/go.mod` requires only `golang.org/x/tools`, so the check would need a
// hand-rolled Go `name`-table parser or a NEW MODULE DEPENDENCY — in the very
// module whose own dependency graph `ScanLicenceGraph` audits. On this side a
// `name`-table read is a short DataView walk with no dependency at all, and it
// sits beside the parse of the generator that decides which file to read.
//
// NO FONT LIBRARY, DELIBERATELY. Everything below is offsets from the OpenType
// spec: the table directory (12-byte header, 16-byte records), `name` (format,
// count, stringOffset, then 12-byte records), `post` (isFixedPitch at +12), and
// `cmap` subtable formats 4 and 12. Adding a parser dependency to read four
// integers would put a new package in the designer's graph to check a fact
// about three committed files.
// ---------------------------------------------------------------------------

type SfntTable = { readonly offset: number; readonly length: number }

function fontView(file: string): DataView {
  const bytes = fs.readFileSync(file)
  return new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength)
}

/** The table directory: four-character tag -> where that table starts and how long it is. */
function sfntTables(view: DataView): Readonly<Record<string, SfntTable>> {
  const version = view.getUint32(0)
  // 0x00010000 is TrueType outlines; 'true' is the legacy Apple spelling. A
  // 'ttcf' collection or an 'OTTO' CFF font is neither of the two things this
  // repository commits, and reading one as if it were is how a guard goes
  // quietly wrong rather than loudly.
  if (version !== 0x00010000 && version !== 0x74727565) throw new Error(`not a single-face sfnt: version 0x${version.toString(16).padStart(8, '0')}`)
  const tables: Record<string, SfntTable> = {}
  const count = view.getUint16(4)
  for (let index = 0; index < count; index++) {
    const record = 12 + index * 16
    let tag = ''
    for (let byte = 0; byte < 4; byte++) tag += String.fromCharCode(view.getUint8(record + byte))
    tables[tag] = { offset: view.getUint32(record + 8), length: view.getUint32(record + 12) }
  }
  return tables
}

/**
 * One `name`-table string, preferring the Windows (platform 3) UTF-16BE record
 * and falling back to the Macintosh (platform 1) single-byte one, which is the
 * only other encoding these files carry. Returns undefined when the font
 * declares no such name at all — reported by the caller rather than defaulted,
 * because a missing name and a wrong name are different defects.
 */
function nameTableString(view: DataView, tables: Readonly<Record<string, SfntTable>>, nameID: number): string | undefined {
  const name = tables['name']
  if (name === undefined) throw new Error('font has no name table')
  const count = view.getUint16(name.offset + 2)
  const storage = name.offset + view.getUint16(name.offset + 4)
  let singleByte: string | undefined
  for (let index = 0; index < count; index++) {
    const record = name.offset + 6 + index * 12
    if (view.getUint16(record + 6) !== nameID) continue
    const platform = view.getUint16(record)
    const length = view.getUint16(record + 8)
    const bytes = Buffer.from(view.buffer.slice(view.byteOffset + storage + view.getUint16(record + 10), view.byteOffset + storage + view.getUint16(record + 10) + length))
    if ((platform === 3 || platform === 0) && length % 2 === 0) return bytes.swap16().toString('utf16le')
    singleByte ??= bytes.toString('latin1')
  }
  return singleByte
}

/**
 * The family the file declares FOR ITSELF: nameID 16 (typographic family) when
 * present, else nameID 1 (family). Both IBM Plex and Noto ship Regular-only
 * static instances that carry nameID 1 and no nameID 16, so the fallback is the
 * live path here — the preference is stated because a font that DOES carry
 * nameID 16 means it by it.
 */
function declaredFamilyOfFile(file: string): string {
  const view = fontView(file)
  const tables = sfntTables(view)
  return nameTableString(view, tables, 16) ?? nameTableString(view, tables, 1) ?? '<the file declares no family name>'
}

/** `post.isFixedPitch` — non-zero exactly when the face claims to be monospaced. */
function isFixedPitch(file: string): number {
  const view = fontView(file)
  const post = sfntTables(view)['post']
  if (post === undefined) throw new Error(`${file} has no post table`)
  return view.getUint32(post.offset + 12)
}

/** nameID 13, the licence description the face carries in its own bytes. */
function licenceDescriptionOfFile(file: string): string {
  const view = fontView(file)
  return nameTableString(view, sfntTables(view), 13) ?? '<the file declares no licence description>'
}

/**
 * The glyph id a file maps a code point to, or 0 for "not covered". Every
 * `cmap` subtable is tried and the first non-zero answer wins, so the lookup
 * does not depend on which platform/encoding a given foundry happened to
 * publish. Formats 4 (BMP) and 12 (full range) are the two these files use.
 */
function glyphForCodePoint(view: DataView, tables: Readonly<Record<string, SfntTable>>, codePoint: number): number {
  const cmap = tables['cmap']
  if (cmap === undefined) throw new Error('font has no cmap table')
  const subtables = view.getUint16(cmap.offset + 2)
  for (let index = 0; index < subtables; index++) {
    const start = cmap.offset + view.getUint32(cmap.offset + 4 + index * 8 + 4)
    const format = view.getUint16(start)
    if (format === 4) {
      if (codePoint > 0xffff) continue
      const segmentsX2 = view.getUint16(start + 6)
      const endCodes = start + 14
      const startCodes = endCodes + segmentsX2 + 2
      const deltas = startCodes + segmentsX2
      const rangeOffsets = deltas + segmentsX2
      for (let segment = 0; segment < segmentsX2 / 2; segment++) {
        if (view.getUint16(endCodes + segment * 2) < codePoint) continue
        if (view.getUint16(startCodes + segment * 2) > codePoint) break
        const rangeOffset = view.getUint16(rangeOffsets + segment * 2)
        const delta = view.getInt16(deltas + segment * 2)
        if (rangeOffset === 0) {
          const glyph = (codePoint + delta) & 0xffff
          if (glyph !== 0) return glyph
          break
        }
        const at = rangeOffsets + segment * 2 + rangeOffset + (codePoint - view.getUint16(startCodes + segment * 2)) * 2
        if (at + 1 >= view.byteLength) break
        const glyph = view.getUint16(at)
        if (glyph !== 0) return (glyph + delta) & 0xffff
        break
      }
    } else if (format === 12) {
      const groups = view.getUint32(start + 12)
      for (let group = 0; group < groups; group++) {
        const at = start + 16 + group * 12
        if (view.getUint32(at) > codePoint || view.getUint32(at + 4) < codePoint) continue
        const glyph = view.getUint32(at + 8) + (codePoint - view.getUint32(at))
        if (glyph !== 0) return glyph
      }
    }
  }
  return 0
}

/** The code points of `text` the file does NOT map to a glyph, as `U+XXXX` labels. */
function codePointsNotCovered(file: string, text: string): ReadonlyArray<string> {
  const view = fontView(file)
  const tables = sfntTables(view)
  const missing = new Set<string>()
  for (const character of text) {
    const codePoint = character.codePointAt(0) as number
    if (glyphForCodePoint(view, tables, codePoint) === 0) missing.add(`U+${codePoint.toString(16).toUpperCase().padStart(4, '0')}`)
  }
  return [...missing]
}

/**
 * Every code point in `[from, to]`, as one string — so a whole RANGE can be put
 * through `codePointsNotCovered` and the answer is about the range rather than
 * about a sample somebody chose.
 */
function codePointRange(from: number, to: number): string {
  let text = ''
  for (let codePoint = from; codePoint <= to; codePoint++) text += String.fromCodePoint(codePoint)
  return text
}

/**
 * THE CHECKER THE WHOLE STORY IS ABOUT: the families whose declared source file
 * does not, in its own `name` table, call itself by the name the stylesheet
 * declares it under. Empty means every checked family name is an assertion the
 * bytes agree with.
 *
 * Takes the generator TEXT rather than reading it, so the red-proof below can
 * drive this exact function with a fixture generator — an identity guard that
 * only ever passes has not been shown to discriminate.
 */
function familiesDeclaredFromForeignBytes(generator: string, families: ReadonlyArray<string>): ReadonlyArray<string> {
  const declared = familySourcePaths(generator)
  return families.flatMap((family) => {
    const relative = declared[family]
    if (relative === undefined) return [`${family}: no @font-face rule declares it`]
    if (isSentinel(relative)) return [`${family}: declared from ${relative}, so no bytes resolve at all`]
    const file = path.join(designerRoot, relative)
    if (!fs.existsSync(file)) return [`${family}: declared from ${relative}, which does not exist`]
    const actual = declaredFamilyOfFile(file)
    return actual === family ? [] : [`${family}: declared from ${relative}, whose name table says '${actual}'`]
  })
}

describe('the family names the browser is given are the files the engine measured with', () => {
  // THE GENERATOR IS PARSED WITHOUT ITS COMMENTS. Both halves of the join are
  // read out of source text by regex, and comment text is source text: a rule
  // that has been commented out reads exactly like a live one. Measured — see
  // the helper's own note — commenting out the three engine-named rules left
  // this whole file green over a stylesheet that had lost half its faces.
  const generator = withoutComments(fs.readFileSync(generatorPath, 'utf8'))
  const fontsGo = fs.readFileSync(enginePath, 'utf8')

  // NON-VACUITY ON BOTH PARSED HALVES, FIRST AND SEPARATELY. Each half is
  // parsed out of a source file by regex, and a regex that stops matching
  // yields an empty object over which every assertion below passes. These two
  // are what turn "the generator was reformatted" into a red test rather than
  // a silent one.
  it('reads both halves of the generator, and neither is empty', () => {
    const slots = slotSourcePaths(generator)
    expect(Object.keys(slots).length, `read no font asset slots out of ${generatorPath}`).toBeGreaterThanOrEqual(3)
    expect(familySlots(generator).length, `read no @font-face rules out of ${generatorPath}`).toBe(13)
    expect(shippedFaceNames(fontsGo).length, `read no face names out of Shipped() in ${enginePath}`).toBe(11)
  })

  // AND NEITHER HALF COUNTS COMMENTED-OUT TEXT. The rule count above is a
  // text count, so without the strip a rule that had been commented out still
  // satisfied it — the measured mutation that left every guard here green while
  // the emitted stylesheet dropped to three rules.
  it('reads neither a commented-out @font-face rule nor a commented-out assets slot as live', () => {
    const emitted = "@font-face { font-family: 'Noto Sans'; src: url('./runtime/${assets.sans}') format('truetype'); font-display: swap; }"
    const slot = "  sans: fingerprint(join(designerRoot, 'public', 'fonts', 'notosans', 'NotoSans-Regular.ttf'), 'noto-sans.ttf'),"
    // The live direction first, so both parses are shown to find anything.
    expect(familySlots(withoutComments(emitted)).length).toBe(1)
    expect(Object.keys(slotSourcePaths(withoutComments(slot)))).toEqual(['sans'])
    // Then the two comment forms a generator can hide either half in.
    expect(familySlots(withoutComments(`// ${emitted}`)).length).toBe(0)
    expect(familySlots(withoutComments(`/* ${emitted} */`)).length).toBe(0)
    expect(Object.keys(slotSourcePaths(withoutComments(`// ${slot}`)))).toEqual([])
    // AND THE DEFECT ITSELF: unstripped, the commented-out rule counted.
    expect(familySlots(`// ${emitted}`).length).toBe(1)
  })

  // THE LICENCE GATE SEES EVERY FONT THAT SHIPS (D-8.4.23).
  //
  // Not "these thirteen files are .ttf" — that is the instance, and the instance was
  // never the risk. The risk is the next font added by a different route: the
  // obvious procurement path for IBM Plex is `@ibm/plex-*` on npm, which ships
  // `.woff2`/`.woff` and no `.ttf` at all, and a `.woff2` reaching the bundle
  // would ship with NO LICENSE REQUIRED, NO NOTICE REQUIRED AND NO
  // `lint/MANIFEST.md` ROW — AD-26's asset half silently not applying to the
  // very binaries it exists to account for. A prose constraint in one story's
  // spec cannot catch that, because the person taking the other route is not
  // reading that spec. This is the behavioural form.
  //
  // TWO POPULATIONS, BOTH DERIVED. Every font asset slot the generator
  // fingerprints into the runtime bundle, and — independently — every
  // GIT-TRACKED FILE IN THE REPOSITORY THE LICENCE GATE WALKS whose OWN FIRST
  // FOUR BYTES are a font magic, so a webfont dropped in anywhere is caught by
  // what it is rather than by what it is called, where it was put, or by
  // whether anything points at it yet.
  //
  // The second population was `folio-designer/public/fonts` until Story 8.4h
  // (AC7, D-8.5.2). MEASURED at that widening, not assumed, and measured
  // against the tree this story ships in rather than the one it started from:
  // the population grew from 18 files under `public/fonts` to 1374 tracked
  // files (1438 listed by `git ls-files`, 64 of them under `*/testdata/lint`,
  // which the gate's own walk skips), and it newly reported NOTHING. Of those
  // 1374, exactly 11 carry
  // a font-plausible extension and all 11 are `.ttf` — no `.woff`, `.woff2`,
  // `.eot`, `.otc`, `.pfb` or `.dfont` is tracked anywhere — and reading the
  // first four bytes of every one of the rest yields zero font magics. That
  // the widening is inert TODAY is a measurement; assuming it would have been
  // the very mistake D-8.5.13 names.
  it('lets the asset licence gate see every font that reaches the runtime bundle', () => {
    const recognised = licenceGateFontExtensions(fs.readFileSync(licenceGatePath, 'utf8'))

    // THE MIRROR IS NON-VACUOUS FIRST. The recognised set is read out of
    // `manifest.go` by regex, and a regex that stopped matching would yield an
    // empty set over which EVERY extension is unrecognised — loud rather than
    // silent, but the helper throws on that, so this states what was read.
    expect(recognised, `read no font extensions out of ${licenceGatePath}`).toEqual(['.ttf', '.otf', '.ttc'])

    expect(
      slotsInvisibleToTheLicenceGate(generator, recognised),
      'A font asset slot listed here is fingerprinted into `src/generated/runtime/` and thence into the release bundle, '
      + 'but carries an extension `lint/internal/manifest/manifest.go`\'s `fontExtensions` does not recognise. '
      + '`manifest.ResolveAssets` would not require a LICENSE* beside it, would not require a NOTICE* beside it, and '
      + 'would give it no row in `lint/MANIFEST.md`: the binary ships and AD-26\'s asset half does not apply to it. '
      + 'Ship the `.ttf` from the upstream release archive, not the `.woff2` from the npm package.',
    ).toEqual([])

    // THE POPULATION IS NON-VACUOUS FIRST, for the same reason the mirror is:
    // `licenceGateTrackedFiles` throws rather than returning [], but stating
    // the floor here means a future change that softened the throw into a
    // fallback still reds. The figure is a FLOOR, deliberately not an equality
    // — a count written next to the thing it counts stops being true the moment
    // the thing grows (D-8.5.4).
    const tracked = licenceGateTrackedFiles(repoRoot)
    expect(tracked.length, `read only ${tracked.length} tracked files out of ${repoRoot}`).toBeGreaterThan(500)

    // AND IT REACHES OUTSIDE THE DESIGNER'S OWN FONT TREE — the whole point of
    // AC7. Stated as directories rather than as a count, so it says what the
    // widening bought.
    const reachedDirectories = new Set(tracked.map((file) => path.relative(repoRoot, file).split('/')[0]))
    expect([...reachedDirectories].sort(), 'the widened population must span the repository the licence gate walks, not one subtree')
      .toEqual(expect.arrayContaining(['folio-designer', 'folio-go', 'lint']))
    // And it excludes what the gate excludes: gitignored caches and build
    // output are not tracked, and `*/testdata/lint` is skipped by the walk.
    expect(tracked.filter((file) => /(^|\/)(node_modules|dist)\//.test(path.relative(repoRoot, file))), 'untracked build output must not enter the population').toEqual([])
    expect(tracked.filter((file) => /(^|\/)testdata\/lint\//.test(path.relative(repoRoot, file))), 'the licence gate skips */testdata/lint, so this guard must too').toEqual([])

    expect(
      committedFontsTheLicenceGateCannotSee(tracked, recognised, repoRoot),
      'A file listed here IS a font by its own first four bytes and carries an extension the asset licence gate does not '
      + 'recognise, so it is invisible to `manifest.ResolveAssets` however it came to be committed. The generator does '
      + 'not have to point at it yet for that to be true.',
    ).toEqual([])
  })

  // AND THE SWEEP IS SHOWN TO DISCRIMINATE, both halves of it, so an empty
  // list above means "no unrecognised font" rather than "the reader stopped
  // reading". The fixture slot and the fixture bytes are the two shapes the
  // real thing would take.
  it('reports a font the licence gate cannot see, by extension and by magic bytes', () => {
    const recognised = ['.ttf', '.otf', '.ttc']
    const woff2Slot = "  plexSans: fingerprint(join(designerRoot, 'public', 'fonts', 'ibmplexsans', 'IBMPlexSans-Regular.woff2'), 'ibm-plex-sans.woff2'),\n"
    const ttfSlot = "  sans: fingerprint(join(designerRoot, 'public', 'fonts', 'notosans', 'NotoSans-Regular.ttf'), 'noto-sans.ttf'),\n"

    // The healthy direction, through the same function.
    expect(slotsInvisibleToTheLicenceGate(ttfSlot, recognised)).toEqual([])
    // And the npm route, verbatim.
    expect(slotsInvisibleToTheLicenceGate(`${ttfSlot}${woff2Slot}`, recognised)).toEqual([
      "plexSans -> public/fonts/ibmplexsans/IBMPlexSans-Regular.woff2 (extension '.woff2')",
    ])

    // THE BYTE READER. `wOF2` is a WOFF2 wrapper's own signature; `\0\0\0`
    // is a static TrueType's. A reader that answered `false` to everything
    // would make the tree sweep above pass over any tree at all.
    //
    // STILL DRIVEN THROUGH A DIRECTORY, deliberately. Story 8.4h changed the
    // sweep to take a FILE LIST rather than a directory, and a proof that
    // quietly stopped exercising a directory would go vacuous — so `filesUnder`
    // feeds this scratch directory straight in. Rewritten, never narrowed
    // (D-8.5.8b).
    const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-font-magic-'))
    try {
      const webfont = path.join(scratch, 'IBMPlexSans-Regular.woff2')
      fs.writeFileSync(webfont, Buffer.concat([Buffer.from('wOF2', 'latin1'), Buffer.alloc(16)]))
      fs.writeFileSync(path.join(scratch, 'NOTICE.md'), 'not a font\n')
      expect(looksLikeAFontBinary(webfont)).toBe(true)
      expect(looksLikeAFontBinary(path.join(scratch, 'NOTICE.md'))).toBe(false)
      expect(committedFontsTheLicenceGateCannotSee(filesUnder(scratch), recognised, scratch)).toEqual([
        "IBMPlexSans-Regular.woff2 (extension '.woff2')",
      ])
      // And a recognised extension is not reported even though it is a font.
      fs.renameSync(webfont, path.join(scratch, 'IBMPlexSans-Regular.ttf'))
      expect(committedFontsTheLicenceGateCannotSee(filesUnder(scratch), recognised, scratch)).toEqual([])
      // AN EXTENSIONLESS FILE is neither reported nor mis-described. Before
      // Story 8.4h `extensionOf` returned such a file's LAST CHARACTER, which
      // under `public/fonts` never arose and repo-wide arises constantly.
      fs.writeFileSync(path.join(scratch, 'LICENSE'), 'not a font\n')
      expect(extensionOf(path.join(scratch, 'LICENSE'))).toBe('')
      expect(committedFontsTheLicenceGateCannotSee(filesUnder(scratch), recognised, scratch)).toEqual([])
    } finally {
      fs.rmSync(scratch, { recursive: true, force: true })
    }

    // AND THE MIRROR ITSELF, shown to read a real declaration and to refuse a
    // file that carries none rather than defaulting to a permissive set.
    expect(licenceGateFontExtensions('var fontExtensions = []string{".ttf", ".otf", ".ttc"}')).toEqual(['.ttf', '.otf', '.ttc'])
    expect(() => licenceGateFontExtensions('package manifest')).toThrow(/no 'var fontExtensions/)
  })

  // AC7's OWN PROOF: THE NEW DIRECTORY REACH IS REAL, not merely declared.
  //
  // The claim the widening makes is that a font-magic file with an
  // unrecognised extension is now reported FROM ANYWHERE THE LICENCE GATE
  // WALKS, where before it was reported only from `folio-designer/public/fonts`.
  // Proved in a SCRATCH GIT REPOSITORY rather than by mutating this one: the
  // enumeration reads `git ls-files`, so a synthetic subject needs a real
  // index (the same recipe `lint/internal/manifest/manifest_test.go` uses), and
  // a proof that writes into the live tree is a proof that can leave debris.
  //
  // Population-independent by construction: nothing here depends on which
  // faces this repository happens to commit today.
  it('reports a font the licence gate cannot see from anywhere it walks, not just public/fonts', () => {
    // READ OUT OF `manifest.go`, never restated: the guard must agree with the
    // gate, not with a copy of the gate (this file's own header doctrine).
    const recognised = licenceGateFontExtensions(fs.readFileSync(licenceGatePath, 'utf8'))
    const scratchRepo = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-tracked-reach-'))
    const write = (relative: string, bytes: Buffer | string) => {
      const full = path.join(scratchRepo, relative)
      fs.mkdirSync(path.dirname(full), { recursive: true })
      fs.writeFileSync(full, bytes)
      return full
    }
    const woff2 = Buffer.concat([Buffer.from('wOF2', 'latin1'), Buffer.alloc(16)])
    try {
      execFileSync('git', ['-C', scratchRepo, 'init', '-q'])

      // OUTSIDE the designer's font tree, in three different subtrees the
      // licence gate walks — each one invisible to the pre-8.4h population.
      write('folio-go/testdata/fonts/Sneaked.woff2', woff2)
      write('lint/testdata/licence/Sneaked.woff2', woff2)
      write('folio-designer/src/assets/Sneaked.woff2', woff2)
      // And the controls: a font with a RECOGNISED extension (the gate sees it,
      // so this guard must not report it), a non-font, and an extensionless file.
      write('folio-go/fonts/Legitimate.ttf', woff2)
      write('folio-go/NOTICE.md', 'not a font\n')
      write('folio-go/LICENSE', 'not a font\n')
      // And one inside `*/testdata/lint`, which the licence gate's own walk
      // SKIPS — so this guard must skip it too, or the two disagree.
      write('folio-go/testdata/lint/embed-font/Skipped.woff2', woff2)
      // An UNTRACKED font-magic file: the gate excludes what git does not
      // track, and so must this. Written but never added.
      write('folio-go/untracked-scratch/Ignored.woff2', woff2)

      execFileSync('git', ['-C', scratchRepo, 'add',
        'folio-go/testdata/fonts/Sneaked.woff2',
        'lint/testdata/licence/Sneaked.woff2',
        'folio-designer/src/assets/Sneaked.woff2',
        'folio-go/fonts/Legitimate.ttf',
        'folio-go/NOTICE.md',
        'folio-go/LICENSE',
        'folio-go/testdata/lint/embed-font/Skipped.woff2',
      ])

      const tracked = licenceGateTrackedFiles(scratchRepo)
      expect([...committedFontsTheLicenceGateCannotSee(tracked, recognised, scratchRepo)].sort(), 'the widened guard must report a font-magic file with an unrecognised extension from ANY tracked directory the licence gate walks').toEqual([
        "folio-designer/src/assets/Sneaked.woff2 (extension '.woff2')",
        "folio-go/testdata/fonts/Sneaked.woff2 (extension '.woff2')",
        "lint/testdata/licence/Sneaked.woff2 (extension '.woff2')",
      ])

      // AND THIS IS WHAT THE WIDENING BOUGHT. The pre-8.4h population was
      // `folio-designer/public/fonts`, which does not exist in this repository
      // at all — so every one of the three above was invisible before, and the
      // narrow sweep reports nothing here however many fonts are sneaked in.
      const narrowPopulation = path.join(scratchRepo, 'folio-designer', 'public', 'fonts')
      expect(fs.existsSync(narrowPopulation), 'the pre-widening population is empty in this fixture — that is the point').toBe(false)
    } finally {
      fs.rmSync(scratchRepo, { recursive: true, force: true })
    }
  })

  // A GUARD THAT CANNOT LOOK MUST NOT READ AS ALL-CLEAR (D-3.6.5's own ground).
  // The enumeration's failure modes are throws, never empty sets — an empty
  // set would flow straight into `.toEqual([])` above and pass.
  it('throws rather than reporting a clean population when the tracked enumeration cannot be obtained', () => {
    const notARepository = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-not-a-repo-'))
    const emptyRepository = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-empty-repo-'))
    try {
      // git itself fails.
      expect(() => licenceGateTrackedFiles(notARepository)).toThrow(/git ls-files failed/)
      // git succeeds and tracks nothing.
      execFileSync('git', ['-C', emptyRepository, 'init', '-q'])
      expect(() => licenceGateTrackedFiles(emptyRepository)).toThrow(/tracks no files at all/)
      // git tracks files, but none survives the licence gate's walk filter.
      const skipped = path.join(emptyRepository, 'testdata', 'lint', 'stub.txt')
      fs.mkdirSync(path.dirname(skipped), { recursive: true })
      fs.writeFileSync(skipped, 'x\n')
      execFileSync('git', ['-C', emptyRepository, 'add', 'testdata/lint/stub.txt'])
      expect(() => licenceGateTrackedFiles(emptyRepository)).toThrow(/survived the licence gate's walk filter/)
    } finally {
      fs.rmSync(notARepository, { recursive: true, force: true })
      fs.rmSync(emptyRepository, { recursive: true, force: true })
    }
  })

  // THE WHOLE MAP, EXACTLY. Not a containment and not a count: an exact
  // `toEqual` over the composed join, so an added rule, a removed rule or a
  // rule repointed at another slot all redden here, and the failure prints the
  // family and the file rather than a number.
  it('binds every declared family to the source file it is declared from', () => {
    expect(familySourcePaths(generator)).toEqual({
      'IBM Plex Sans': 'public/fonts/ibmplexsans/IBMPlexSans-Regular.ttf',
      'IBM Plex Mono': 'public/fonts/ibmplexmono/IBMPlexMono-Regular.ttf',
      'IBM Plex Sans Thai': 'public/fonts/ibmplexsansthai/IBMPlexSansThai-Regular.ttf',
      'Noto Sans': 'public/fonts/notosans/NotoSans-Regular.ttf',
      'Noto Sans Thai': 'public/fonts/notosansthai/NotoSansThai-Regular.ttf',
      'Noto Sans SC': 'public/fonts/notosanssc/NotoSansSC-Regular.ttf',
      // STORY 11.1'S SEVEN CUTS. Each is its own family name over its own
      // directory, so the exact map below is also the statement that no cut
      // shares bytes with the Regular it was derived from.
      'Noto Sans Bold': 'public/fonts/notosans-bold/NotoSans-Bold.ttf',
      'Noto Sans Italic': 'public/fonts/notosans-italic/NotoSans-Italic.ttf',
      'Noto Sans Bold Italic': 'public/fonts/notosans-bolditalic/NotoSans-BoldItalic.ttf',
      'Noto Sans Thai Bold': 'public/fonts/notosansthai-bold/NotoSansThai-Bold.ttf',
      'Roboto Bold': 'public/fonts/roboto-bold/Roboto-Bold.ttf',
      'Roboto Italic': 'public/fonts/roboto-italic/Roboto-Italic.ttf',
      'Roboto Bold Italic': 'public/fonts/roboto-bolditalic/Roboto-BoldItalic.ttf',
    })
  })

  // THE INTERVAL, ENDED. Story 8.4b pinned three files reached by two names
  // each — the design system's and the engine's — and said in as many words
  // that Story 8.4c is what splits them. It has: thirteen rules now reach
  // thirteen distinct files, and no family shares bytes with any other. What replaced
  // the interval is stronger than it was, because a shared file is now a
  // defect rather than the expected state.
  it('gives every declared family a source file of its own, the interval Story 8.4b pinned now ended', () => {
    const perFile = familiesPerSourceFile(generator)
    // EVERY FAMILY THAT MUST BE DECLARED IS DECLARED. The sharing checker
    // below cannot see a DELETION by construction — a deleted rule produces
    // fewer families per file, never more — so the opposite direction is
    // asserted here, through a helper the red-proof fixture drives too. The
    // required set is not a list: three design-system families fixed by
    // DESIGN.md, and the engine's own face names read out of `fonts.Shipped()`
    // — EXCEPT a face the CATALOGUE also declares (Roboto, since Story 16.8):
    // that one's rule is the catalogue's own templated one, invisible to this
    // hand-written-rule parse by construction (see the note above
    // `catalogueDeclaredFamilies`), and checked instead in its own dedicated
    // assertion below.
    const catalogueFamilySet = new Set(catalogueDeclaredFamilies())
    expect(
      familiesWithNoRule(generator, [...chromeFamilies, ...shippedFaceNames(fontsGo).filter((face) => !catalogueFamilySet.has(face))]),
      'A family listed here is one the chrome or the canvas ASKS FOR and no @font-face rule declares. The browser falls '
      + 'silently through to a generic face — which is the shape of the Thai rendering defect on this repository\'s '
      + 'record, where "letters rendered on top of each other". A rule "simplified" away is the way this happens.',
    ).toEqual([])

    expect(Object.keys(perFile).length, 'the thirteen @font-face rules must reach thirteen distinct source files').toBe(13)

    // EVERY GROUPING KEY IS A REAL FILE. `familySourcePaths` falls back to a
    // sentinel string when a rule names an `assets` slot that does not
    // resolve, and a sentinel groups like a path — so a rule resolving to no
    // bytes whatsoever would otherwise read as an ordinary unshared file.
    expect(
      Object.keys(perFile).filter(isSentinel),
      'an @font-face rule names an `assets` slot that does not exist, so nothing resolves the bytes behind that family',
    ).toEqual([])

    expect(
      filesReachedByMoreThanOneFamily(generator),
      'A file listed here is reached by more than one family name. Two names over one file was the deliberate INTERVAL '
      + 'between Story 8.4b and 8.4c, and it is over: the design system asks for real IBM Plex bytes and the canvas asks '
      + 'for the engine\'s Noto bytes, and the two vocabularies are separate BY DESIGN rather than by accident. A family '
      + 'that has drifted back onto another family\'s file rasterizes with someone else\'s face and fails silently.',
    ).toEqual([])
  })

  // THE RED-PROOF FOR THE SHARING CHECKER. The real generator has, and will
  // increasingly have, no offenders at all, so the corpus itself cannot
  // exercise the failing direction. A checker that only ever passes has not
  // been shown to discriminate.
  it('reports a shared file, a repointed rule and an unresolvable slot', () => {
    const rule = (family: string, slot: string) => `@font-face { font-family: '${family}'; src: url('./runtime/\${assets.${slot}}') format('truetype'); font-display: swap; }`
    const assetsBlock = "  sans: fingerprint(join(designerRoot, 'public', 'fonts', 'notosans', 'NotoSans-Regular.ttf'), 'noto-sans.ttf'),\n"
      + "  sansThai: fingerprint(join(designerRoot, 'public', 'fonts', 'notosansthai', 'NotoSansThai-Regular.ttf'), 'noto-sans-thai.ttf'),\n"
    const fixture = (...rules: ReadonlyArray<string>) => `${assetsBlock}${rules.join('\\n')}`

    // THE FULLY DIVERGED SHAPE, so the checker is shown to pass something: one
    // family per file, no sharing anywhere. This is the shape the real
    // generator now has.
    const diverged = fixture(rule('Noto Sans', 'sans'), rule('Noto Sans Thai', 'sansThai'))
    expect(filesReachedByMoreThanOneFamily(diverged)).toEqual([])
    expect(familySourcePaths(diverged)['Noto Sans']).toBe('public/fonts/notosans/NotoSans-Regular.ttf')

    // A FILE SHARED BY TWO FAMILIES — the state Story 8.4b pinned and 8.4c
    // ended, and the state a drifted or reverted rule would restore.
    const shared = fixture(rule('IBM Plex Sans', 'sans'), rule('Noto Sans', 'sans'), rule('Noto Sans Thai', 'sansThai'))
    expect(filesReachedByMoreThanOneFamily(shared)).toEqual(['public/fonts/notosans/NotoSans-Regular.ttf (reached by: IBM Plex Sans, Noto Sans)'])

    // A PAIR REPOINTED AT DIFFERING FILES — the mutation that leaves a family
    // the canvas asks for rasterizing with someone else's bytes.
    const repointed = fixture(rule('Noto Sans', 'sansThai'), rule('Noto Sans Thai', 'sansThai'))
    expect(filesReachedByMoreThanOneFamily(repointed)).toEqual(['public/fonts/notosansthai/NotoSansThai-Regular.ttf (reached by: Noto Sans, Noto Sans Thai)'])
    expect(familySourcePaths(repointed)['Noto Sans']).toBe('public/fonts/notosansthai/NotoSansThai-Regular.ttf')

    // A RULE DELETED — the "simplification" that would leave a family the
    // canvas asks for with no face behind it at all. IT DRIVES `familiesWithNoRule`,
    // the same function the production assertion above drives, because the
    // SHARING checker cannot catch a deletion however this fixture is written:
    // a deletion yields FEWER families per file, never more. Asserting only the
    // key list here — as this fixture once did — exercised no checker at all
    // and left deletion the one direction never shown to discriminate.
    const deleted = fixture(rule('Noto Sans Thai', 'sansThai'))
    expect(Object.keys(familySourcePaths(deleted))).toEqual(['Noto Sans Thai'])
    expect(filesReachedByMoreThanOneFamily(deleted), 'the sharing checker cannot see a deletion, which is why the one below exists').toEqual([])
    expect(familiesWithNoRule(deleted, ['IBM Plex Sans', 'Noto Sans', 'Noto Sans Thai'])).toEqual(['IBM Plex Sans', 'Noto Sans'])
    // And the passing direction through the same function, so an empty list
    // from the production assertion means "all declared" rather than "the
    // helper stopped looking".
    expect(familiesWithNoRule(diverged, ['Noto Sans', 'Noto Sans Thai'])).toEqual([])

    // AND A RULE OVER A SLOT THAT DOES NOT RESOLVE. It reaches no bytes, and
    // without the sentinel check it would group like a perfectly good file.
    const unresolvable = fixture(rule('IBM Plex Sans', 'nope'), rule('Noto Sans Thai', 'sansThai'))
    expect(Object.keys(familiesPerSourceFile(unresolvable)).filter(isSentinel)).toEqual(['<no assets slot named nope>'])
    expect(filesReachedByMoreThanOneFamily(unresolvable)).toEqual(['<no assets slot named nope> (reached by: IBM Plex Sans)'])

    // AND THE PARSER ITSELF GOING BLIND, which must not read as health.
    expect(familySlots('writeFileSync(cssPath, `body { font-family: sans-serif }`)').length).toBe(0)
  })

  // THE FILE-SWAP TIE, AND THE POINT OF THE WHOLE STORY. A family called
  // `Noto Sans` is only useful because the bytes behind it are the bytes the
  // ENGINE measured `Noto Sans` with. Names are cheap; this compares files.
  //
  // SCOPED TO THE ENGINE-NAMED HALF ON PURPOSE, and permanently so. The IBM
  // Plex families are the DESIGN SYSTEM's vocabulary and are declared from IBM
  // Plex bytes, which are not and must not be any face the engine embeds —
  // asserting engine identity for them would now be false. They are checked
  // the other way instead, against their own `name` tables, in the suite
  // below. This half is the stronger tie, because for the canvas a family
  // called `Noto Sans` is only useful if its bytes are the bytes the engine
  // MEASURED `Noto Sans` with. Names are cheap; this compares files.
  it('declares each engine face from bytes identical to the ones fonts.Shipped() embeds', () => {
    const declaredPaths = familySourcePaths(generator)
    const engineNamed = Object.keys(declaredPaths).filter((family) => !isChrome(family))
    const shipped = shippedFacePaths(fontsGo)

    // THE NAME SETS ARE EQUAL, OVER THE HAND-WRITTEN HALF ONLY. Story 16.8
    // adds Roboto to `fonts.Shipped()` without a hand-written rule
    // (see the note above `catalogueDeclaredFamilies`), so `engineNamed` —
    // sourced from the hand-written rules alone — is one name short of
    // `shipped` by construction. It is excluded here, BY NAME rather than by
    // a bare count adjustment, and checked its own way immediately below:
    // add a face to `fonts.Shipped()` that is neither hand-written NOR a
    // catalogue face, or drop one of the Noto faces, and this still
    // reddens.
    //
    // ELEVEN AND TEN SINCE STORY 11.1, three and four before it. Note which
    // side each of the seven cuts landed on: `Roboto Bold`, `Roboto Italic`
    // and `Roboto Bold Italic` are HAND-WRITTEN, not catalogue-routed. Only
    // `Roboto` itself is the dual-vocabulary face; its three cuts are ordinary
    // members of the set below and are digest-tied here like every other one.
    //
    // ⚠ THE REASON RECORDED HERE WAS RETIRED BY spec-install-all-face-cuts
    // STORY 3. It read "a bold cut cannot be a catalogue face at all —
    // `font-catalogue.test.ts` holds every catalogue entry to upright Regular
    // 400", and that rule is gone: the catalogue declares a `style` per row and
    // carries every cut its 31 families publish, each held to its OWN instance.
    // A bold cut is a perfectly ordinary catalogue face now.
    //
    // ROBOTO'S THREE CUTS STAY HAND-WRITTEN FOR A DIFFERENT AND STANDING
    // REASON: they already ship as CORE release assets and as `fonts.Shipped()`
    // keys, so redeclaring them in `font-catalogue.json` would emit a second
    // byte-identical dist asset per cut, and retiring the hand-written copies
    // would move the designer's 30/30 core-asset pin. Both were refused, which
    // is why this split survives the rule that used to explain it.
    const shippedViaCatalogue = new Set(['Roboto'])
    const shippedHandWritten = Object.fromEntries(Object.entries(shipped).filter(([face]) => !shippedViaCatalogue.has(face)))
    expect([...engineNamed].sort()).toEqual(Object.keys(shippedHandWritten).sort())
    expect(Object.keys(shipped).length).toBe(11)
    expect(Object.keys(shippedHandWritten).length).toBe(10)

    // THE SEVEN CUTS ARE IN THE DIGEST LOOP, NAMED. The loop below is derived,
    // so it would silently shrink to three if a cut lost its rule or its
    // `Shipped()` key — and an empty-by-attrition loop passes. Naming them is
    // what turns "seven engine/designer pairs are compared" from a property of
    // today's parse into an assertion.
    for (const cut of ['Noto Sans Bold', 'Noto Sans Italic', 'Noto Sans Bold Italic', 'Noto Sans Thai Bold', 'Roboto Bold', 'Roboto Italic', 'Roboto Bold Italic']) {
      expect(Object.keys(shippedHandWritten), `${cut} must be one of the faces whose engine and designer bytes are compared below`).toContain(cut)
    }

    // AND SO ARE THE BYTES, face by face, over all ten hand-written faces.
    for (const face of Object.keys(shippedHandWritten)) {
      const browserFile = path.join(designerRoot, declaredPaths[face])
      const engineFile = path.join(engineFontsDir, shippedHandWritten[face])
      expect(fs.existsSync(browserFile), `${browserFile} (declared for '${face}') must exist`).toBe(true)
      expect(fs.existsSync(engineFile), `${engineFile} (embedded for '${face}') must exist`).toBe(true)
      expect(
        digest(browserFile),
        `the browser declares '${face}' from ${declaredPaths[face]}, which must be byte-identical to the file folio-go embeds under that face name (${shippedHandWritten[face]}) — AD-17 makes the browser a rasterizer only, so a same-named different face fails silently`,
      ).toBe(digest(engineFile))
    }
  })

  // ROBOTO'S OWN BYTE-IDENTITY TIE (Story 16.8), THE CATALOGUE-ROUTED TWIN OF
  // THE TEST ABOVE. "THERE IS EXACTLY ONE ROBOTO" is this story's own stated
  // risk: two cuts of Roboto already exist in this repository 180 KB apart
  // (this file, byte-identical; folio-go/testdata/fonts/Roboto-Regular.ttf,
  // a DIFFERENT cut used only as an internal/fontset licence-signature
  // fixture and never embedded). The Go-side twin of this assertion is
  // folio-go/fonts/fonts_test.go's TestShippedRobotoMatchesDesignerCatalogue;
  // this is the browser side of the same claim, so a mismatch is caught
  // whichever half of the repository a reader is looking from.
  it('ships the SAME Roboto bytes the catalogue declares — there is exactly one Roboto', () => {
    const shipped = shippedFacePaths(fontsGo)
    expect(shipped.Roboto, 'fonts.Shipped() carries no "Roboto" key, or fonts.go\'s //go:embed for it could not be read').toBeDefined()
    const catalogueFile = path.join(designerRoot, catalogueEngineRobotoFile)
    const engineFile = path.join(engineFontsDir, shipped.Roboto as string)
    expect(fs.existsSync(catalogueFile), `${catalogueFile} must exist`).toBe(true)
    expect(fs.existsSync(engineFile), `${engineFile} must exist`).toBe(true)
    expect(
      digest(catalogueFile),
      `the designer's catalogue Roboto (${catalogueEngineRobotoFile}) must be byte-identical to the file folio-go embeds as "Roboto" (${shipped.Roboto}) — a second cut under the same name would make "fontFamily": "Roboto" render differently depending on which path produced the document`,
    ).toBe(digest(engineFile))
  })
})

// THE FIRST TEST IN THIS REPOSITORY THAT CAN TELL WHETHER THE BYTES BEHIND A
// FONT NAME ARE THE FONT THAT NAME CLAIMS.
//
// Everything above joins a family name to a FILE PATH. That is a claim about
// the generator, and it was true of the defect too: `IBM Plex Mono` resolved,
// perfectly consistently, to `noto-sans-cjk.ttf` — Noto Sans SC, a 10.6 MB
// Chinese sans with no monospacing at all, drawing every number, tab label and
// brand mark in the chrome. Nothing was red. The assertions below open the
// declared file and read what the FILE says about itself.
//
// WHAT THESE GATES CAN AND CANNOT PROVE. Vitest under jsdom applies no
// stylesheet and loads no font, and this repository has no gate that executes
// a real font load or rasterizes a glyph (`test:e2e:compile` is `tsc
// --noEmit`). So this proves the BINDING — which family name resolves to which
// bytes, and what those bytes declare themselves to be. It does not prove that
// a browser visibly draws the chrome in IBM Plex.
describe('the bytes behind a chrome family name are the face that name claims', () => {
  const generator = withoutComments(fs.readFileSync(generatorPath, 'utf8'))

  // ALL THREE DESIGN-SYSTEM FAMILIES. Story 8.4c's first commit scoped this to
  // `IBM Plex Mono`, the worst of the three and the cheapest end-to-end proof
  // the pipeline worked; every chrome family now has its own IBM Plex bytes,
  // so every one of them is checked. The engine-named half is checked a
  // stronger way below — against the exact bytes `fonts.Shipped()` embeds.
  it('declares each chrome family from a file whose own name table carries that family', () => {
    expect(
      familiesDeclaredFromForeignBytes(generator, chromeFamilies),
      'A design-system family name is an assertion about bytes. Each family listed here is declared over a file that '
      + 'calls itself something else — which is exactly the shipped defect this guard exists to make observable, and '
      + 'which no gate in this repository could see before it.',
    ).toEqual([])
  })

  it('gives the mono family a genuinely monospaced face', () => {
    const file = path.join(designerRoot, familySourcePaths(generator)['IBM Plex Mono'])

    // THE DEFECT'S OWN SHAPE. `--font-mono` — and through it `--type-mono`,
    // `--type-mono-em`, `--type-numeric-lg`, `--type-brand`,
    // `--type-brand-load`, `--type-band-tab` and `--type-page-mono` — resolves
    // through this family. A face that is not fixed-pitch satisfies the name
    // check above and still draws every column of digits ragged.
    expect(isFixedPitch(file), `${file} is declared as 'IBM Plex Mono' but its post table does not claim fixed pitch`).not.toBe(0)
  })

  // AD-26 IN THE BYTES, FOR EVERY REDISTRIBUTED CHROME FACE — not for one of
  // three. AC2 says each chrome family names "a committed IBM Plex OFL binary
  // CARRYING THE OFL nameID 13 STRING", and a redistributed asset's own terms
  // travel in its `name` table as well as in the LICENSE*/NOTICE* beside it.
  // Asserting it for the mono face alone left two thirds of the claim
  // unchecked, and the two unchecked ones are the faces AC2 actually names.
  it('carries the SIL OFL in the bytes of every chrome face, not only in the LICENSE beside it', () => {
    const declared = familySourcePaths(generator)
    for (const family of chromeFamilies) {
      const file = path.join(designerRoot, declared[family])
      expect(
        licenceDescriptionOfFile(file),
        `${declared[family]} is declared as '${family}', so AD-26 requires it to be a redistributed asset carrying its `
        + 'own terms — and nameID 13 is where a font states them in its own bytes. A face whose licence description is '
        + 'absent, or is not the OFL, is not the OFL binary the licence manifest claims this row ships.',
      ).toContain('SIL Open Font License, Version 1.1')
    }
  })

  // THE PROVENANCE RECORD, MADE TRUE OF THE BYTES BESIDE IT.
  //
  // Every committed face sits next to a NOTICE.md recording the sha256 of the
  // file shipped — and nothing asserted that record was true. The engine-named
  // half was already digest-pinned (against `folio-go/fonts`, above); the
  // chrome half had no pin at all, which made it the weaker one. The gap is
  // concrete rather than theoretical: `declaredFamilyOfFile` prefers nameID 16,
  // identical across every weight and style of a family, so IBM Plex Sans
  // Bold, SemiBold, Italic or a `pyftsubset` Latin-only cut would keep every
  // other assertion in this file green while the whole chrome rendered in the
  // wrong face — and each NOTICE's digest silently became a false statement.
  //
  // ALL THIRTEEN DECLARED FACES, not just the three chrome ones. The Noto pin above
  // proves the designer's copy MIRRORS the engine's; it says nothing about
  // whether either is the file its provenance record describes. This is the
  // other question, and it costs the same to ask of all thirteen.
  it('ships every declared face at the exact bytes its own NOTICE.md records', () => {
    const declared = familySourcePaths(generator)
    expect(Object.keys(declared).length, 'nothing to pin means the generator parse went blind').toBe(13)
    for (const [family, relative] of Object.entries(declared)) {
      const file = path.join(designerRoot, relative)
      const notice = path.join(path.dirname(file), 'NOTICE.md')
      expect(fs.existsSync(notice), `${relative} is declared for '${family}' but has no NOTICE.md beside it — which manifest.ResolveAssets (AC25, AD-26) already requires`).toBe(true)
      expect(
        digest(file),
        `${relative} is declared for '${family}', and its own NOTICE.md records a different sha256 for the file it ships. `
        + 'Either the binary was swapped without amending its provenance record — a different weight, a different style, '
        + 'a subset cut, all of which keep this file\'s name-table checks green — or the record was amended without the '
        + 'binary. Both make the NOTICE a false statement about the bytes beside it.',
      ).toBe(recordedShippedDigest(notice))
    }
  })

  // AND THE RECORD READER IS SHOWN TO READ, AND TO REFUSE. A parser that
  // returned the file's own digest, or that quietly defaulted when it found no
  // row, would satisfy the assertion above over any bytes at all.
  it('reads a recorded digest out of a NOTICE, and refuses a NOTICE that records none', () => {
    const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'folio8-notice-'))
    try {
      const notice = path.join(scratch, 'NOTICE.md')
      const recorded = 'a'.repeat(64)
      fs.writeFileSync(notice, `# a face\n\n| item | value |\n|---|---|\n| **sha256 of the SHIPPED file** | \`${recorded}\` |\n`)
      expect(recordedShippedDigest(notice)).toBe(recorded)
      // The engine faces' NOTICEs spell the same row "SHIPPED (produced)".
      fs.writeFileSync(notice, `| **sha256 of the SHIPPED (produced) file** | \`${recorded}\` |\n`)
      expect(recordedShippedDigest(notice)).toBe(recorded)
      // A record with no digest row, and one with two, are both unreadable
      // rather than absent — loud, because a provenance record that cannot be
      // read is a pin that is not pinning.
      fs.writeFileSync(notice, '# a face\n\nno digest recorded here at all\n')
      expect(() => recordedShippedDigest(notice)).toThrow(/exactly one 'sha256 of the SHIPPED …' table row/)
      fs.writeFileSync(notice, `| **sha256 of the SHIPPED file** | \`${recorded}\` |\n| **sha256 of the SHIPPED file** | \`${'b'.repeat(64)}\` |\n`)
      expect(() => recordedShippedDigest(notice)).toThrow(/and records 2/)
    } finally {
      fs.rmSync(scratch, { recursive: true, force: true })
    }
  })

  // THE THAI FACE, AGAINST THE FAILURE THAT ACTUALLY HAPPENED. These two
  // strings are the ones from the rendering defect this repository recorded —
  // Thai fell through to the generic `sans-serif` and, in the report's own
  // words, "letters rendered on top of each other". Replacing the Thai chrome
  // face is the one substitution in this story that could regress something
  // visible, so the guard is written against the reported strings rather than
  // against a range a reviewer would have to trust.
  //
  // MEASURED, and why this is a floor and not the whole story: this face and
  // the shipped Noto Sans Thai map the SAME 87 of the 128 code points in
  // U+0E00–U+0E7F, with an empty set difference in both directions (the other
  // 41 are Unicode-unassigned). Coverage is what a cmap read can prove.
  // Whether marks ATTACH correctly is a GPOS/shaping question no gate here can
  // execute; the face's mark lookups are compared in its NOTICE.md.
  it('gives the page family a Thai face that covers the strings from the recorded defect', () => {
    const file = path.join(designerRoot, familySourcePaths(generator)['IBM Plex Sans Thai'])
    for (const reported of ['พระราชบัญญัติ', 'การทวงถามหนี้']) {
      expect(
        codePointsNotCovered(file, reported),
        `${file} is declared as 'IBM Plex Sans Thai' but does not map every code point of '${reported}' — one of the two `
        + 'strings from the shipped Thai rendering defect. An uncovered code point falls through to the generic '
        + 'sans-serif, which is exactly how that defect looked.',
      ).toEqual([])
    }

    // AND THE READER IS SHOWN TO FIND AN ABSENCE, so an empty list above means
    // "covered" rather than "the cmap walk stopped working". The Latin sans
    // face carries no Thai at all, which is what makes the fallback order in
    // App.css's stack load-bearing rather than decorative.
    expect(codePointsNotCovered(path.join(designerRoot, 'public/fonts/ibmplexsans/IBMPlexSans-Regular.ttf'), 'พ')).toEqual(['U+0E1E'])
    expect(codePointsNotCovered(file, 'folio8')).toEqual([])
  })

  // THE LATIN RANGE THE CHROME ACTUALLY DRAWS, AS A RANGE.
  //
  // `ibmplexsans/NOTICE.md` records zero gaps across U+0020–U+017F, measured
  // once by hand on the day the binary landed. Nothing asserted it, so it was a
  // claim about a file rather than a property of the file — and a swapped or
  // subset binary would have left the claim standing while making it false.
  // This is that claim as an executable one.
  //
  // AS SPECIFIED, "U+0020–U+017F WITH NO GAPS" IS NOT LITERALLY TRUE OF ANY
  // TEXT FACE, and asserting it verbatim would have been red on day one:
  // U+007F–U+009F are the C0/C1 CONTROL code points, and neither this face nor
  // the shipped Noto Sans maps them (measured: 33 unmapped, the same 33 in
  // both). So the no-gaps claim is asserted over the three PRINTABLE blocks the
  // range decomposes into — which is exactly what the NOTICE's own table
  // measured — and the controls are pinned separately, by PARITY with the face
  // the chrome used to be drawn in, so "unmapped" is a fact about the code
  // points rather than a hole this face happens to have.
  it('covers the whole printable Latin range the chrome draws, and differs from the shipped Noto Sans only where Unicode has no character', () => {
    const declared = familySourcePaths(generator)
    const plexSans = path.join(designerRoot, declared['IBM Plex Sans'])
    const notoSans = path.join(designerRoot, declared['Noto Sans'])

    // ASCII, Latin-1 Supplement, Latin Extended-A — the whole of what the
    // chrome draws, asserted block by block so a failure names the block.
    for (const [from, to] of [[0x0020, 0x007e], [0x00a0, 0x00ff], [0x0100, 0x017f]] as const) {
      expect(
        codePointsNotCovered(plexSans, codePointRange(from, to)),
        `${declared['IBM Plex Sans']} is declared as 'IBM Plex Sans' — the family every --type-* token in tokens.css `
        + `resolves through — and does not map every code point of U+${from.toString(16).toUpperCase().padStart(4, '0')}`
        + `–U+${to.toString(16).toUpperCase().padStart(4, '0')}. An uncovered code point in the chrome's own range falls `
        + 'through to system-ui, which is the same silent-fallback failure the Thai defect was.',
      ).toEqual([])
    }

    // AND THE REST OF THE RANGE, BY PARITY. The only code points either face
    // leaves unmapped across U+0020–U+017F are the ones the other leaves
    // unmapped too — so the swap of the chrome's Latin face away from Noto Sans
    // lost nothing in the range, and the gaps that remain are Unicode's rather
    // than this face's.
    const wholeRange = codePointRange(0x0020, 0x017f)
    const plexGaps = codePointsNotCovered(plexSans, wholeRange)
    const notoGaps = codePointsNotCovered(notoSans, wholeRange)
    expect(plexGaps).toEqual(notoGaps)

    // NON-VACUITY, BOTH WAYS. The reader must be finding coverage (an empty
    // list above would otherwise be indistinguishable from a cmap walk that
    // stopped working) and must be finding absence (or parity would be the
    // parity of two empty answers).
    expect(plexGaps.length, 'the two faces agree on nothing at all, so the cmap walk is not reading').toBeLessThan(0x0180 - 0x0020)
    expect(plexGaps.length, 'the C0/C1 controls in this range are mapped by neither face and must be found').toBeGreaterThan(0)
    expect(plexGaps).toContain('U+007F')
  })

  // THAI cmap PARITY WITH THE FACE THE ENGINE MEASURES WITH, IN BOTH
  // DIRECTIONS.
  //
  // `ibmplexsansthai/NOTICE.md` records exact parity with the shipped Noto Sans
  // Thai over U+0E00–U+0E7F, set difference empty in both directions. That was
  // the acceptance risk of this whole substitution — the one change in Story
  // 8.4c that could regress something visible — and it was discharged by a
  // measurement nothing re-ran. The two defect strings above are 20 distinct
  // code points of the 87 this face maps; this is the other 67.
  //
  // BOTH FILES ARE READ FROM THE GENERATOR'S OWN DECLARATIONS rather than named
  // as literals, so the comparison is between the face the chrome is given and
  // the face the canvas is given, whatever the generator points either at.
  it('maps exactly the Thai code points the shipped Noto Sans Thai maps, in both directions', () => {
    const declared = familySourcePaths(generator)
    const plexThai = path.join(designerRoot, declared['IBM Plex Sans Thai'])
    const notoThai = path.join(designerRoot, declared['Noto Sans Thai'])
    const thai = codePointRange(0x0e00, 0x0e7f)

    const covered = (file: string) => {
      const gaps = new Set(codePointsNotCovered(file, thai))
      return [...thai].map((character) => `U+${(character.codePointAt(0) as number).toString(16).toUpperCase().padStart(4, '0')}`).filter((label) => !gaps.has(label))
    }
    const inPlex = new Set(covered(plexThai))
    const inNoto = new Set(covered(notoThai))

    expect(
      covered(notoThai).filter((label) => !inPlex.has(label)),
      `${declared['IBM Plex Sans Thai']} is declared as 'IBM Plex Sans Thai' and does NOT map Thai code points the `
      + 'engine\'s own Thai face does. The canvas measures with the engine face and the chrome draws with this one, so a '
      + 'code point the chrome cannot draw falls through to a generic — which is exactly how "letters rendered on top of '
      + 'each other" was reported.',
    ).toEqual([])

    expect(
      covered(plexThai).filter((label) => !inNoto.has(label)),
      'the chrome face maps Thai code points the engine face does not — recorded as a difference rather than assumed '
      + 'away, because the two faces were measured to agree exactly and a divergence either way means one of them moved.',
    ).toEqual([])

    // NON-VACUITY AND DISCRIMINATION. Parity between two empty sets is not
    // parity, and the same comparison against a face with no Thai must come
    // out loudly unequal.
    expect(inPlex.size, 'neither Thai face maps anything, so the cmap walk is not reading').toBeGreaterThan(80)
    expect(inPlex.size).toBe(inNoto.size)
    const latinOnly = new Set(covered(path.join(designerRoot, declared['IBM Plex Sans'])))
    expect(covered(notoThai).filter((label) => !latinOnly.has(label)).length, 'the Latin chrome face must NOT pass the Thai parity comparison').toBeGreaterThan(80)
  })

  // THE RED-PROOF. The corpus cannot exercise the failing direction — the
  // whole point of the story is that it no longer holds — so the checker is
  // driven with a fixture generator that reinstates the exact defect: the
  // family `IBM Plex Mono` bound back at the Noto CJK file.
  it('reports a mismatch when any chrome family is bound back at the Noto file it used to share', () => {
    const rule = (family: string, slot: string) => `@font-face { font-family: '${family}'; src: url('./runtime/\${assets.${slot}}') format('truetype'); font-display: swap; }`
    const slot = (name: string, ...segments: ReadonlyArray<string>) => `  ${name}: fingerprint(join(designerRoot, ${segments.map((segment) => `'${segment}'`).join(', ')}), 'label.ttf'),\n`
    const assetsBlock = slot('sans', 'public', 'fonts', 'notosans', 'NotoSans-Regular.ttf')
      + slot('sansCjk', 'public', 'fonts', 'notosanssc', 'NotoSansSC-Regular.ttf')
      + slot('sansThai', 'public', 'fonts', 'notosansthai', 'NotoSansThai-Regular.ttf')
      + slot('mono', 'public', 'fonts', 'ibmplexmono', 'IBMPlexMono-Regular.ttf')
      + slot('plexSans', 'public', 'fonts', 'ibmplexsans', 'IBMPlexSans-Regular.ttf')
      + slot('plexSansThai', 'public', 'fonts', 'ibmplexsansthai', 'IBMPlexSansThai-Regular.ttf')

    // The healthy direction first, through the same function and over all
    // three families, so the checker is shown to pass something as well as to
    // fail something.
    const healthy = `${assetsBlock}${rule('IBM Plex Sans', 'plexSans')}${rule('IBM Plex Mono', 'mono')}${rule('IBM Plex Sans Thai', 'plexSansThai')}`
    expect(familiesDeclaredFromForeignBytes(healthy, chromeFamilies)).toEqual([])

    // AND THE DEFECT, VERBATIM: the state this repository shipped in, family
    // by family — each chrome name bound back at the engine file it shared
    // before Story 8.4c. All three at once, so a checker that discriminated
    // for only one of them would still be caught.
    const reverted = `${assetsBlock}${rule('IBM Plex Sans', 'sans')}${rule('IBM Plex Mono', 'sansCjk')}${rule('IBM Plex Sans Thai', 'sansThai')}`
    expect(familiesDeclaredFromForeignBytes(reverted, chromeFamilies)).toEqual([
      "IBM Plex Sans: declared from public/fonts/notosans/NotoSans-Regular.ttf, whose name table says 'Noto Sans'",
      "IBM Plex Mono: declared from public/fonts/notosanssc/NotoSansSC-Regular.ttf, whose name table says 'Noto Sans SC'",
      "IBM Plex Sans Thai: declared from public/fonts/notosansthai/NotoSansThai-Regular.ttf, whose name table says 'Noto Sans Thai'",
    ])
    // The mono slot is checked a second way, and that way discriminates too:
    // the CJK sans it used to resolve to is not monospaced.
    expect(isFixedPitch(path.join(designerRoot, 'public/fonts/notosanssc/NotoSansSC-Regular.ttf'))).toBe(0)

    // A rule over bytes that do not exist, and a family with no rule at all,
    // are reported rather than silently skipped — the two ways this checker
    // could go vacuous while returning an empty list.
    expect(familiesDeclaredFromForeignBytes(`${assetsBlock}${rule('IBM Plex Mono', 'nope')}`, ['IBM Plex Mono'])).toEqual([
      'IBM Plex Mono: declared from <no assets slot named nope>, so no bytes resolve at all',
    ])
    expect(familiesDeclaredFromForeignBytes(assetsBlock, chromeFamilies)).toEqual([
      'IBM Plex Sans: no @font-face rule declares it',
      'IBM Plex Mono: no @font-face rule declares it',
      'IBM Plex Sans Thai: no @font-face rule declares it',
    ])
  })

  // AND THE READER ITSELF, SHOWN TO READ. Every assertion above is only worth
  // what the `name`-table walk is worth, and a reader that returned the same
  // string for everything would satisfy them all.
  it('reads a family name out of every committed face, and tells them apart', () => {
    expect(declaredFamilyOfFile(path.join(designerRoot, 'public/fonts/ibmplexsans/IBMPlexSans-Regular.ttf'))).toBe('IBM Plex Sans')
    expect(declaredFamilyOfFile(path.join(designerRoot, 'public/fonts/ibmplexmono/IBMPlexMono-Regular.ttf'))).toBe('IBM Plex Mono')
    expect(declaredFamilyOfFile(path.join(designerRoot, 'public/fonts/ibmplexsansthai/IBMPlexSansThai-Regular.ttf'))).toBe('IBM Plex Sans Thai')
    expect(declaredFamilyOfFile(path.join(designerRoot, 'public/fonts/notosans/NotoSans-Regular.ttf'))).toBe('Noto Sans')
    expect(declaredFamilyOfFile(path.join(designerRoot, 'public/fonts/notosanssc/NotoSansSC-Regular.ttf'))).toBe('Noto Sans SC')
    expect(declaredFamilyOfFile(path.join(designerRoot, 'public/fonts/notosansthai/NotoSansThai-Regular.ttf'))).toBe('Noto Sans Thai')
  })
})
