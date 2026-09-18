import { copyFileSync, existsSync, mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs'
import { createHash } from 'node:crypto'
import { dirname, join } from 'node:path'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { assertNoVCSStamp, buildEngineWasm } from './wasm-vcs-stamp.mjs'
import { CATALOGUE_ASSET_PREFIX } from './offline-release-contract.mjs'
import { emitFontIndexModule } from './build-font-index.mjs'
import { buildExamples } from './build-examples.mjs'
// THE ONE sfnt `name`-TABLE READER (Story 16.1). This script used to carry
// its own hand-written `DataView` walk, and `src/font-catalogue.test.ts`
// carried a second one; Story 16.1 needed a THIRD, at runtime, and extracted
// the walk into one production module instead. Node 24 strips the types, so
// this plain-ESM build script imports the TypeScript directly.
import { faceCopyright as readFaceCopyright } from '../src/font-name-table.ts'

const designerRoot = join(dirname(fileURLToPath(import.meta.url)), '..')
const generatedDir = join(designerRoot, 'src', 'generated')
const outputDir = join(generatedDir, 'runtime')
const goRoot = execFileSync('go', ['env', 'GOROOT'], { encoding: 'utf8' }).trim()
const wasmExec = [join(goRoot, 'lib', 'wasm', 'wasm_exec.js'), join(goRoot, 'misc', 'wasm', 'wasm_exec.js')].find(existsSync)

if (!wasmExec) throw new Error(`wasm_exec.js not found below ${goRoot}`)
rmSync(outputDir, { recursive: true, force: true })
mkdirSync(outputDir, { recursive: true })
const wasmPath = join(outputDir, 'folio8-engine.wasm')
// `-buildvcs=false` CLOSES ONE INPUT: THE TREE'S STATE. Go's default stamps
// `vcs.revision`, `vcs.time` and `vcs.modified` into the binary, and it derives
// `vcs.modified` from `git status`, where a single UNTRACKED file is enough — so
// without this flag the engine wasm, and every byte figure measured over the
// bundle it dominates, is a fact about whoever last wrote a scratch file into the
// checkout (D-8.5.7). With it, a clean tree, a stray untracked file and a modified
// tracked file all produce the same bytes — measured, not assumed.
//
// IT DOES NOT MAKE THE BUNDLE A FUNCTION OF THE SOURCE ALONE. The checkout's
// absolute PATH is still an input: two copies of one commit at two paths, both
// unstamped, build to different bytes, because Go embeds absolute source paths.
// That residual is measured and recorded as DW-105; `-trimpath` is its candidate
// remedy and is deliberately not applied here.
//
// Dropping `vcs.revision` is a DELIBERATE TRADE, NOT A FREE WIN: the artifact no
// longer self-identifies its commit. It is an acceptable one because the release
// manifest already carries `releaseId` and `pageId` derived from asset hashes,
// which identify the BUNDLE more precisely than a commit does, and because AD-22
// pins an exact `toolchain` directive, making the compiler behind these bytes a
// release event rather than an ambient fact.
// The flag and the argv live in wasm-vcs-stamp.mjs so the release verifier can
// run this exact build with the flag dropped, and run it twice with the flag, as
// executable proofs rather than as prose in a delivery log.
buildEngineWasm(wasmPath)
// Enforce the flag at its point of use, not only in a test: `build:wasm` is a
// dependency of `typecheck`, `test` and `build`, so dropping the flag reddens
// every designer gate and fails fast — and this must run here, while the raw wasm
// still exists, because it is deleted after fingerprinting below.
assertNoVCSStamp(readFileSync(wasmPath), 'src/generated/runtime/folio8-engine.wasm')
const gluePath = join(outputDir, 'wasm-exec.js')
copyFileSync(wasmExec, gluePath)
// PDF.js support files are copied into the same generated, immutable runtime
// tree as wasm and fonts. The viewer imports their generated URL map so Vite
// emits and the release verifier precaches every local CMap/font asset.
const pdfjsRoot = join(designerRoot, 'node_modules', 'pdfjs-dist')
const copyPDFJSRuntime = (directory, files) => {
  const target = join(outputDir, directory)
  mkdirSync(target, { recursive: true })
  for (const file of files) copyFileSync(join(pdfjsRoot, directory.replace('pdfjs-', ''), file), join(target, file))
}
// folio8 PDFs embed their own shipped faces; these local fallback assets cover
// the four CID collections PDF.js may need to inspect and its standard sans
// family without shipping its unneeded browser-viewer resources or licences.
copyPDFJSRuntime('pdfjs-cmaps', ['Adobe-GB1-0.bcmap', 'Adobe-CNS1-0.bcmap', 'Adobe-Japan1-0.bcmap', 'Adobe-Korea1-0.bcmap'])
copyPDFJSRuntime('pdfjs-standard_fonts', ['LiberationSans-Regular.ttf', 'LiberationSans-Bold.ttf', 'LiberationSans-Italic.ttf', 'LiberationSans-BoldItalic.ttf'])
// The starter is an empty, author-owned canvas. Whitespace keeps it a
// deliberately non-canonical input; only the Go serializer determines the
// bytes the application subsequently reads and saves.
const starterPath = join(outputDir, 'starter.folio')
writeFileSync(starterPath, Buffer.concat([Buffer.from('\n  '), readFileSync(join(designerRoot, 'public', 'templates', 'starter.folio'))]))

const fingerprint = (source, label) => {
  const bytes = readFileSync(source)
  const digest = createHash('sha256').update(bytes).digest('hex')
  const extension = label.slice(label.lastIndexOf('.'))
  const stem = label.slice(0, -extension.length)
  const target = `${stem}.${digest.slice(0, 20)}${extension}`
  copyFileSync(source, join(outputDir, target))
  return target
}

// PDF.js resolves CMaps and standard fonts by appending their canonical file
// names to these bases. Give each collection one content-addressed directory
// (rather than content-addressing individual filenames), preserving that
// contract while still making the directory immutable when any copied byte
// changes.
const directoryFingerprint = (directory) => createHash('sha256').update(readdirSync(directory).sort().map((file) => `${file}:${createHash('sha256').update(readFileSync(join(directory, file))).digest('hex')}`).join('\n')).digest('hex').slice(0, 20)
const pdfjsCMapDirectory = `pdfjs-cmaps-${directoryFingerprint(join(outputDir, 'pdfjs-cmaps'))}`
const pdfjsStandardFontDirectory = `pdfjs-standard-fonts-${directoryFingerprint(join(outputDir, 'pdfjs-standard_fonts'))}`

const assets = {
  wasmExec: fingerprint(gluePath, 'wasm-exec.js'),
  wasm: fingerprint(wasmPath, 'folio8-engine.wasm'),
  starter: fingerprint(starterPath, 'starter.folio'),
  sans: fingerprint(join(designerRoot, 'public', 'fonts', 'notosans', 'NotoSans-Regular.ttf'), 'noto-sans.ttf'),
  sansCjk: fingerprint(join(designerRoot, 'public', 'fonts', 'notosanssc', 'NotoSansSC-Regular.ttf'), 'noto-sans-cjk.ttf'),
  sansThai: fingerprint(join(designerRoot, 'public', 'fonts', 'notosansthai', 'NotoSansThai-Regular.ttf'), 'noto-sans-thai.ttf'),
  mono: fingerprint(join(designerRoot, 'public', 'fonts', 'ibmplexmono', 'IBMPlexMono-Regular.ttf'), 'ibm-plex-mono.ttf'),
  plexSans: fingerprint(join(designerRoot, 'public', 'fonts', 'ibmplexsans', 'IBMPlexSans-Regular.ttf'), 'ibm-plex-sans.ttf'),
  plexSansThai: fingerprint(join(designerRoot, 'public', 'fonts', 'ibmplexsansthai', 'IBMPlexSansThai-Regular.ttf'), 'ibm-plex-sans-thai.ttf'),
  // STORY 11.1'S SEVEN WEIGHTED AND SLOPED CUTS. Each is a face of its own
  // under its own family name (D-11.1.5), so each needs its own slot, its own
  // hand-written rule below and its own `shippedFamilies` entry — and NO
  // catalogue entry, because `src/font-catalogue.test.ts` asserts every
  // catalogue face is an upright Regular 400 and a bold cut fails all four of
  // those checks. The catalogue legitimately stays Regular-only.
  //
  // ONE `fingerprint()` CALL IS ONE DIST ASSET IS ONE CACHE SLOT, and being
  // inside the engine wasm exempts nothing: the three Story 2.2 Notos are
  // `//go:embed`'d AND hold three of the hardcoded slots, because a CSS
  // `@font-face` needs a URL and the wasm's copy has none. These seven take the
  // release from 54 slots to 61 against `maximumCacheAssets` 64 — which is why
  // `warnCacheAssets` ships in this story, in src/release-payload.ts.
  //
  // THE LABELS BELOW ARE DISTINGUISHED BY A DOT, not by a prefix.
  // `generate-offline-release.mjs` finds each row's asset with
  // `url.includes('/noto-sans.')` and friends, so `/noto-sans-bold.` and
  // `/noto-sans-thai-bold.` are unreachable by the Regular faces' needles and
  // vice versa — the trailing dot is what makes that true, and dropping it
  // would let `/noto-sans.` match the bold cut's asset instead.
  sansBold: fingerprint(join(designerRoot, 'public', 'fonts', 'notosans-bold', 'NotoSans-Bold.ttf'), 'noto-sans-bold.ttf'),
  sansItalic: fingerprint(join(designerRoot, 'public', 'fonts', 'notosans-italic', 'NotoSans-Italic.ttf'), 'noto-sans-italic.ttf'),
  sansBoldItalic: fingerprint(join(designerRoot, 'public', 'fonts', 'notosans-bolditalic', 'NotoSans-BoldItalic.ttf'), 'noto-sans-bold-italic.ttf'),
  sansThaiBold: fingerprint(join(designerRoot, 'public', 'fonts', 'notosansthai-bold', 'NotoSansThai-Bold.ttf'), 'noto-sans-thai-bold.ttf'),
  robotoBold: fingerprint(join(designerRoot, 'public', 'fonts', 'roboto-bold', 'Roboto-Bold.ttf'), 'roboto-bold.ttf'),
  robotoItalic: fingerprint(join(designerRoot, 'public', 'fonts', 'roboto-italic', 'Roboto-Italic.ttf'), 'roboto-italic.ttf'),
  robotoBoldItalic: fingerprint(join(designerRoot, 'public', 'fonts', 'roboto-bolditalic', 'Roboto-BoldItalic.ttf'), 'roboto-bold-italic.ttf'),
}

// ───────────────────────────────────────────────────────────────────────────
// THE CATALOGUE (Story 8.5). The thirteen font slots above (six from Stories
// 8.4c/8.5, seven from Story 11.1) are HARDCODED BY NAME because
// each one is load-bearing under a name: `src/main.tsx` and
// `src/engine.worker.ts` import them out of `runtimeAssetUrls`,
// `generate-offline-release.mjs` finds ten of them by URL substring to build
// the S1 payload, and `src/font-binary-identity.test.ts` pins the thirteen-family
// join family by family. They are a vocabulary, not a list.
//
// THE CATALOGUE IS A LIST, and so it is driven by one. `font-catalogue.json` is
// the single place a face is declared; adding one is a directory, a NOTICE and
// a row there, never an edit in three places (Design Note 4). One hardcoded key
// per face, emitting one hand-written CSS rule per face — a list retyped in
// this script every time the catalogue grows — is the shape this deliberately
// does not take.
//
// THE EMITTED CSS SHAPE IS IDENTICAL to the six rules below it — one static
// Regular per family, `format('truetype')`, `font-display: swap`, and NO
// `font-weight` and NO `font-style` descriptor — so AC6's "no bold, no italic,
// no variable axis" stays observable from the generated file itself rather than
// from prose. `src/font-catalogue.test.ts` reads each committed binary's own
// `name` and `OS/2` tables and holds the catalogue to that.
//
// These faces reach Vite's asset graph through the `url()` in the emitted
// stylesheet alone — they are deliberately NOT added to `runtimeAssetUrls`,
// which exists for the assets application code names in an import.
const catalogue = JSON.parse(readFileSync(join(designerRoot, 'font-catalogue.json'), 'utf8'))
if (!Array.isArray(catalogue) || catalogue.length === 0) throw new Error('font-catalogue.json declares no catalogue faces')

// THE THIRTEEN FAMILY NAMES THE HAND-WRITTEN RULES BELOW DECLARE.
//
// SIX UNTIL STORY 11.1, THIRTEEN AFTER IT. Each of the seven weighted and
// sloped cuts is its OWN family name (D-11.1.5) rather than a `font-weight`
// descriptor on an existing family: a `font-weight: 700` rule under
// `Noto Sans` would break the one-static-Regular-per-family convention this
// file's rules are machine-asserted against, and would put a weight axis into
// CSS that the document format deliberately excludes. The same string is the
// `fonts.Shipped()` key, the `@font-face` family, and the family the canvas
// paints with — READABLE by design, and PARSED BY NOTHING.
//
// ⚠ THIS USED TO READ `new Set(Object.keys(assets))`, AND THAT WAS A GUARD THAT
// COULD NOT FIRE. `assets`' keys are SLOT names — `wasmExec`, `wasm`, `starter`,
// `sans`, `sansCjk`, `sansThai`, `mono`, `plexSans`, `plexSansThai` — and not one
// of them is a family name, so the half of the message promising "or over a
// family the six shipped rules already declare" was false: a catalogue entry
// declaring `Noto Sans` passed, and the browser was handed TWO `@font-face`
// rules for one family, the second silently winning. The intra-catalogue
// duplicate half worked; this half asserted nothing.
//
// IT IS A LITERAL LIST AND IT IS CHECKED AGAINST THE RULES, which is the only
// honest shape available here: the thirteen rules must spell their families as
// literals (`src/font-binary-identity.test.ts` and `src/canvas-font-stack.test.ts`
// both parse them out of this file's TEXT, and an interpolated name is invisible
// to both). So the list cannot be derived from the template — instead the
// template is checked against the list, at the point of emission below, and a
// name that falls out of either side reds there rather than here.
const shippedFamilies = ['IBM Plex Sans', 'IBM Plex Mono', 'IBM Plex Sans Thai', 'Noto Sans', 'Noto Sans Thai', 'Noto Sans SC', 'Noto Sans Bold', 'Noto Sans Italic', 'Noto Sans Bold Italic', 'Noto Sans Thai Bold', 'Roboto Bold', 'Roboto Italic', 'Roboto Bold Italic']
// AND NO NAME APPEARS TWICE, WHICH NEITHER THROW BELOW CAN SEE.
// The two emission-time throws are "every name has a rule" and "the rule count
// equals the name count". A DUPLICATED name satisfies both — it has a rule, and
// with thirteen names and thirteen rules the counts still agree — while the
// thirteenth RULE has no name in the list, so the catalogue collision guard
// (`catalogueFamilies`, a Set built from this array) never learns about it and a
// `font-catalogue.json` entry could redeclare that family. The browser would
// then be handed two `@font-face` rules for one family, the second silently
// winning: exactly the defect the collision guard exists to refuse, reached
// through the guard's own input. A Set comparison is the whole fix, and it goes
// here rather than at emission because the list is the thing that is wrong.
if (new Set(shippedFamilies).size !== shippedFamilies.length) throw new Error(`shippedFamilies names ${shippedFamilies.length} families and only ${new Set(shippedFamilies).size} of them are distinct. A duplicate satisfies both emission throws — every name still has a rule, and the counts still agree — while leaving one hand-written rule outside the catalogue's collision guard, so a catalogue face could redeclare it and the browser would get two rules for one family.`)

// A family name is interpolated UNESCAPED into a single-quoted CSS string
// (`font-family: '${face.family}'`). A quote closes it, a backslash escapes the
// closing quote, and a semicolon, brace or newline ends the declaration or the
// rule — so an unchecked name is not a typo, it is a way to write arbitrary CSS
// into `runtime-fonts.css`. Held to the printable, punctuation-free shape every
// real family name has, on the same reasoning the `id` field is held to
// `/^[a-z0-9]+$/`: a field that becomes syntax must be checked where it is read.
const familyShape = /^[A-Za-z0-9][A-Za-z0-9 .+-]*$/
// `directory` and `file` are `join()`ed into a filesystem path. A separator or a
// `..` segment reads a file from outside `public/fonts/`, which would ship bytes
// no NOTICE sits beside and no licence gate ever saw. One path segment, no dots
// leading, is the whole permitted vocabulary.
const segmentShape = /^[A-Za-z0-9][A-Za-z0-9._-]*$/

// THE SCRIPT VOCABULARY (Story 8.6), CLOSED AND SMALL. A face's declared
// coverage is what the designer proposes a fallback TAIL from: the shipped
// faces for the scripts the picked face does NOT cover, in this order. It is
// closed because an unrecognised script would silently propose no fallback for
// itself — the failure mode is a chain that draws tofu, and it would look like
// a correct pick. `font-catalogue.json`'s declaration is held to each binary's
// own `cmap` by `src/font-catalogue.test.ts`; here it is only held to the
// vocabulary.
const scriptFallbacks = { latin: 'Noto Sans', thai: 'Noto Sans Thai', cjk: 'Noto Sans SC' }
// AND THE THREE FALLBACK NAMES ARE HELD TO THE FAMILIES THAT ACTUALLY EXIST.
// These strings become CHAIN ENTRIES in the author's document — the engine
// resolves them against `fonts.Shipped()`'s own keys, and an entry naming a
// face nobody supplies is SKIPPED IN SILENCE (render.go's resolveRuneFace), so
// a typo or a rename here does not error anywhere: the proposed tail simply
// stops covering the script it was proposed for, and the chain draws tofu.
// That is the exact failure the closed `scripts` vocabulary above was justified
// by, reached from the other side.
//
// AND THE ANCHOR IS THE UPRIGHT REGULARS, NOT THE WHOLE SHIPPED SET.
//
// `shippedFamilies` was the anchor until Story 11.1 widened it from six upright
// Regulars to thirteen, seven of which are WEIGHTED OR SLOPED CUTS. Anchored
// there, `scriptFallbacks` could name `Noto Sans Bold` or `Roboto Italic` and
// the build would accept it — and a fallback is not a preference: it is the
// face every chain gets stapled behind it for the runes its picked face cannot
// draw. A bold fallback would render an entire script bold in EVERY author's
// document, silently, everywhere the tail was reached, and nothing downstream
// would flag it because a bold face is a perfectly valid face. The check that
// exists to refuse a fallback naming a face nobody supplies must not admit one
// naming the wrong face instead.
//
// It is still checked against a list that cannot drift out of the stylesheet:
// every member below is asserted to be one of `shippedFamilies`, which is
// itself held to the hand-written @font-face rules at the point of emission.
const shippedRegularFamilies = ['IBM Plex Sans', 'IBM Plex Mono', 'IBM Plex Sans Thai', 'Noto Sans', 'Noto Sans Thai', 'Noto Sans SC']
for (const family of shippedRegularFamilies) {
  if (!shippedFamilies.includes(family)) throw new Error(`shippedRegularFamilies names ${JSON.stringify(family)} and shippedFamilies does not, so the script-fallback anchor has drifted off the population that is held to the emitted stylesheet`)
}
for (const [script, family] of Object.entries(scriptFallbacks)) {
  if (!shippedRegularFamilies.includes(family)) throw new Error(`scriptFallbacks maps the script '${script}' to the face ${JSON.stringify(family)}, which is not one of the upright Regular shipped families (${shippedRegularFamilies.join(', ')}). That string becomes a chain entry in the author's document: the engine SKIPS an entry naming a face it was not given rather than failing — so a name nobody supplies silently proposes a fallback that draws nothing and the chain renders tofu — and a name that IS supplied but is a bold or italic CUT is worse, because it works: an entire script would render bold or sloped in every author's document, for every chain that reached the tail.`)
}

// AND EVERY FALLBACK FACE IS A ROW IN THE DECLARED FAMILY→CUTS MIRROR.
//
// STORY 11.4 GAVE THESE THREE NAMES A SECOND JOB AND A SECOND WAY TO GO WRONG.
// A proposed tail entry no longer only NAMES a shipped face: it declares the
// cuts that face has, looked up in `src/shipped-face-cuts.ts` — the one place
// in the tree that says which faces are one family's cuts. Both pick paths
// compute it as `shippedFamilyEntry(shipped) ?? shipped`, and that `??` is a
// SILENT DEGRADE: a fallback the mirror has no row for falls back to a bare
// face name, which is a perfectly legal chain entry that renders perfectly
// well and simply cannot bold. Nothing downstream can tell that apart from a
// family that genuinely has no cuts.
//
// So the two lists can drift, and the drift is invisible in every author's
// document: rename a family on one side and every Thai run proposed by every
// pick quietly loses its bold, for good, with a green build and a green suite.
// The check above refuses a fallback naming a face nobody SUPPLIES; this one
// refuses a fallback naming a face nobody DECLARED THE CUTS OF.
//
// The mirror is read as SOURCE TEXT rather than imported, for the reason
// `canvas-font-stack.test.ts` reads `fonts.go` as text: this is a build script
// with no TypeScript program around it, and the tie wanted is between two
// authored lists, not between two module graphs.
const mirrorSource = readFileSync(join(designerRoot, 'src', 'shipped-face-cuts.ts'), 'utf8')
const mirrorTable = /export const shippedFamilyCuts: ReadonlyArray<ShippedFamilyCuts> = \[([\s\S]*?)\n\]/.exec(mirrorSource)
if (mirrorTable === null) throw new Error("src/shipped-face-cuts.ts no longer declares shippedFamilyCuts the way build-wasm.mjs reads it, so the scriptFallbacks tie below would pass over an empty list; re-derive the parse before trusting this build")
const mirrorFamilies = [...mirrorTable[1].matchAll(/\bfamily: '([^']+)'/g)].map((row) => row[1])
if (mirrorFamilies.length === 0) throw new Error('read no families out of src/shipped-face-cuts.ts, so the scriptFallbacks tie below is vacuous')
for (const [script, family] of Object.entries(scriptFallbacks)) {
  if (!mirrorFamilies.includes(family)) throw new Error(`scriptFallbacks maps the script '${script}' to the face ${JSON.stringify(family)}, which src/shipped-face-cuts.ts declares no row for (it declares ${mirrorFamilies.join(', ')}). A pick computes a tail entry as shippedFamilyEntry(face) ?? face, so a face with no row falls back to a BARE NAME — a legal entry that renders correctly and can never bold. Every document whose ${script} fallback came from a pick would silently lose that family's cuts, with nothing anywhere to say so.`)
}

const catalogueIds = new Set()
const catalogueFamilies = new Set(shippedFamilies)
const catalogueFaces = catalogue.map((entry) => {
  for (const field of ['id', 'directory', 'file', 'family', 'licence']) {
    if (typeof entry?.[field] !== 'string' || entry[field] === '') throw new Error(`font-catalogue.json entry is missing a ${field}: ${JSON.stringify(entry)}`)
  }
  if (!Array.isArray(entry.scripts) || entry.scripts.length === 0) throw new Error(`font-catalogue.json face ${entry.id} declares no scripts; the designer proposes a fallback tail from this list, and a face that claims nothing would be given a fallback for every script including its own`)
  for (const script of entry.scripts) {
    if (!Object.hasOwn(scriptFallbacks, script)) throw new Error(`font-catalogue.json face ${entry.id} declares the script ${JSON.stringify(script)}, which is not one of ${Object.keys(scriptFallbacks).join(', ')}; an unrecognised script proposes no fallback for itself and the chain draws tofu`)
  }
  if (new Set(entry.scripts).size !== entry.scripts.length) throw new Error(`font-catalogue.json face ${entry.id} declares a script twice`)
  // The id becomes a runtime filename stem AND the token the release manifest
  // recognises a catalogue asset by, so it is held to one shape here rather
  // than trusted to stay one.
  if (!/^[a-z0-9]+$/.test(entry.id)) throw new Error(`font-catalogue.json id ${JSON.stringify(entry.id)} is not lower-case alphanumeric`)
  if (catalogueIds.has(entry.id)) throw new Error(`font-catalogue.json declares the id ${JSON.stringify(entry.id)} twice`)
  for (const [field, value] of [['directory', entry.directory], ['file', entry.file]]) {
    if (!segmentShape.test(value) || value.includes('..')) throw new Error(`font-catalogue.json face ${entry.id} declares a ${field} ${JSON.stringify(value)} that is not a single plain path segment; it is joined into a filesystem path, so a separator or a '..' would read bytes from outside public/fonts/`)
  }
  if (!familyShape.test(entry.family)) throw new Error(`font-catalogue.json face ${entry.id} declares a family ${JSON.stringify(entry.family)} carrying a character that is CSS syntax; it is interpolated unescaped into font-family: '<name>' in the emitted stylesheet`)
  if (catalogueFamilies.has(entry.family)) throw new Error(`font-catalogue.json declares the family ${JSON.stringify(entry.family)} twice, or over a family the thirteen shipped rules already declare`)
  catalogueIds.add(entry.id)
  catalogueFamilies.add(entry.family)
  if (!entry.file.endsWith('.ttf')) throw new Error(`font-catalogue.json face ${entry.id} is ${entry.file}; the emitted @font-face rule declares format('truetype') and the engine decodes only font/ttf and font/otf`)
  return { ...entry, filename: fingerprint(join(designerRoot, 'public', 'fonts', entry.directory, entry.file), `${CATALOGUE_ASSET_PREFIX}${entry.id}.ttf`) }
})

// THE COPYRIGHT LINE AND THE LICENCE TEXT, READ OFF COMMITTED BYTES (Story 8.6).
//
// A `.folio` that carries a face must state its terms — the engine refuses to
// load one that does not — so the designer has to be able to supply them at the
// moment of the pick. Neither is hand-copied into `font-catalogue.json`, and
// that is the whole point: a hand-copied licence is a SECOND authority on what
// the terms are, and the first time a binary is swapped the document would
// publish terms its own bytes contradict.
//
//   licenceText — the unmodified upstream `LICENSE*` file committed beside the
//   binary. It is the same file `manifest.ResolveAssets` (AD-26) already
//   requires and `src/font-catalogue.test.ts` already counts.
//
//   copyright — nameID 0 of the face's OWN `name` table, which is the one
//   statement of a face's provenance that cannot be edited from outside the
//   binary. Measured, not assumed: all 31 committed faces carry it, and a face
//   carrying none throws out of `faceCopyright` below rather than emitting an
//   empty string, so this stays measured as the catalogue grows.
//
// This is ~4 KB of licence text PER FACE, NOT per distinct licence:
// `licenceTextOf` below reads the `LICENSE*` file committed beside THAT face's
// own binary, and every catalogue row inlines its own copy — so today's 31
// faces emit 31 texts even though only three SPDX identifiers classify them.
// That is deliberate, and the block below is why: keying these by identifier is
// exactly what published another project's terms. (The DOCUMENT likewise
// carries one copy per embedded face — deliberately, because an asset passed on
// alone must carry its own terms — so the bundle and the document now state a
// face's terms the same way.)
// THE COPYRIGHT READER IS NOW SHARED (Story 16.1). `sfntTableDirectory`,
// `nameTableString` and `faceCopyright` used to be written out here, by hand,
// beside a byte-identical second copy in `src/font-catalogue.test.ts`. Both are
// now `src/font-name-table.ts`, which the designer also uses AT RUNTIME to read
// nameID 0 out of a face fetched seconds earlier. One walk, three callers, no
// font-parsing dependency added.
const faceCopyright = (file) => {
  try {
    return readFaceCopyright(readFileSync(file))
  } catch (error) {
    throw new Error(`${file}: ${error instanceof Error ? error.message : String(error)}`)
  }
}

// PER FACE, NEVER PER IDENTIFIER. This USED TO BE keyed by SPDX id and filled
// from whichever face reached that id first, which — measured when the
// catalogue held 21 faces — gave 17 of those 21 ANOTHER PROJECT'S licence text:
// every OFL-1.1 face emitted cascadiacode's LICENSE, "with Reserved Font Name
// Cascadia Code" and all. That inverts the whole point of the story: a document
// embedding Inter would have travelled stating terms naming Microsoft's font.
//
// "The OFL is the OFL" is FALSE OF THE FILES, and that is the trap. The SIL OFL
// carries a per-project preamble — a copyright line and a Reserved Font Name —
// so two OFL-1.1 faces ship two DIFFERENT texts, and the identifier is a
// classification of the terms, never a substitute for them.
//
// It costs bundle bytes: one ~4 KB text per face — 31 of them today — where
// keying by identifier would emit one per DISTINCT licence, which is 3 across
// that same catalogue. That is the correct trade and it is stated rather than
// left to be rediscovered — a smaller bundle is not a reason to publish the
// wrong terms. The `?url` imports below are unaffected, so no build ASSET is
// added and the release cache does not grow by one slot on account of this
// module (it is 61 since Story 11.1's seven cuts; it was 54 after Story 16.1a's batch).
const licenceTextOf = (face) => {
  const directory = join(designerRoot, 'public', 'fonts', face.directory)
  const licences = readdirSync(directory).filter((name) => name.startsWith('LICENSE'))
  // Now runs for EVERY face rather than for the first face of each identifier,
  // which is the second thing the cache was quietly costing.
  if (licences.length !== 1) throw new Error(`font-catalogue.json face ${face.id} has ${licences.length} LICENSE* files beside it (${JSON.stringify(licences)}); exactly one is the text that travels into every document embedding this face`)
  return readFileSync(join(directory, licences[0]), 'utf8').trimEnd()
}

// THE COMMITTED TIER'S `source`, INLINED FROM THE NOTICE RATHER THAN POINTING
// AT IT (D-16.R.13, DW-160).
//
// What was here until Story 16.1a:
//
//   `folio8-designer/public/fonts/<dir>/<file> — see that directory's NOTICE.md
//    for the pinned upstream release and digest`
//
// Honest, and incomplete in the one way that matters: **the recipient of a
// `.folio` does not have that NOTICE.md.** The file travels alone (CAP-2), so a
// `source` that points into this repository's tree names a fact its reader
// cannot reach. The fetched tier had the mirror-image defect — a bare mutable
// branch URL — and the two disagreed in KIND, so a reader could not tell which
// tier a face came from. Both halves are corrected in one story because a field
// whose two writers use different vocabularies is uninterpretable, not merely
// inconsistent.
//
// The shape, on BOTH tiers: the **upstream project**, the **path within it**,
// and the **fetch date**. No scheme, no host, no branch name — a
// resolvable-looking string is a promise of fetchability, and a promise that
// decays reads as broken provenance when the provenance is intact. And **no
// SHA-256**: the face is stored under its digest as its asset key, and
// restating it here would put two authorities on one fact.
//
// PARSED, NEVER RETYPED. Every value comes out of the face's own NOTICE.md —
// the same rows `src/font-catalogue.test.ts` already holds each NOTICE to — so
// a provenance record and the string a document publishes cannot drift apart.
// An unparseable NOTICE throws here rather than emitting a vaguer string,
// because a silently degraded provenance line is the failure this field's whole
// correction is about.
const committedFaceSource = (face) => {
  const notice = join(designerRoot, 'public', 'fonts', face.directory, 'NOTICE.md')
  const text = readFileSync(notice, 'utf8')
  const row = (label, pattern) => {
    const match = pattern.exec(text)
    if (match === null) throw new Error(`public/fonts/${face.directory}/NOTICE.md records no ${label}, so the '${face.family}' face cannot state where it came from in the documents that embed it (D-16.R.13). Every catalogue NOTICE carries this row and src/font-catalogue.test.ts asserts it.`)
    return match[1]
  }
  // The project is written `github.com/<owner>/<repo>` in every NOTICE; the
  // host is dropped here, where the string becomes a document field, rather
  // than in the record, where it is a true statement about where to look.
  const project = row('upstream project', /^\| Upstream project \| `([^`]+)`/m).replace(/^github\.com\//, '')
  const release = row('pinned upstream release', /^\| Upstream project \| .+release `([^`]+)` \|$/m)
  const path = row('path inside the archive', /^\| Path inside the archive \| `(\S+)` \|$/m)
  const fetched = row('fetch date', /^\| Fetched \| (\d{4}-\d{2}-\d{2}) \|$/m)
  return `${project}@${release} — ${path}, fetched ${fetched}`
}

// THE TYPED CATALOGUE MODULE. `src` could not enumerate the catalogue at all
// before this: `offline-assets.ts` exports the nine named slots and nothing
// else, and the catalogue faces reached Vite only through the `url()` in the
// emitted stylesheet. They still do for the CSS; this module adds the second
// thing the pick needs — the URL to READ THE BYTES FROM, content-addressed and
// precached exactly as `runtimeAssetUrls` assets are, so the pick reads bytes
// already on the machine and fetches nothing.
//
// NO NEW BUILD ASSET. Every `?url` import below names a file the catalogue loop
// above already fingerprinted into `src/generated/runtime/`; this module names
// them, it does not create them. The release cache's slot count is unchanged
// by this module — 61 of the 64 since Story 11.1's seven cuts, 54 after Story
// 16.1a's batch, 44 before that.
writeFileSync(join(generatedDir, 'font-catalogue.ts'),
  `// GENERATED by scripts/build-wasm.mjs from font-catalogue.json. Do not edit.\n`
  + catalogueFaces.map((face, index) => `import catalogueUrl${index} from './runtime/${face.filename}?url'`).join('\n')
  + `\n\nexport type CatalogueScript = ${Object.keys(scriptFallbacks).map((script) => JSON.stringify(script)).join(' | ')}\n\n`
  + `export type CatalogueFace = Readonly<{ id: string; family: string; style: string; licence: string; licenceText: string; copyright: string; source: string; scripts: ReadonlyArray<CatalogueScript>; url: string }>\n\n`
  + `// The shipped face that covers each script, in the order a proposed tail\n`
  + `// names them. A face's tail is the entries for the scripts it does NOT cover.\n`
  + `export const scriptFallbackFaces: ReadonlyArray<readonly [CatalogueScript, string]> = [${Object.entries(scriptFallbacks).map(([script, face]) => `[${JSON.stringify(script)}, ${JSON.stringify(face)}]`).join(', ')}]\n\n`
  + `export const catalogueFaces: ReadonlyArray<CatalogueFace> = [\n`
  + catalogueFaces.map((face, index) => `  { id: ${JSON.stringify(face.id)}, family: ${JSON.stringify(face.family)}, style: "Regular", licence: ${JSON.stringify(face.licence)}, licenceText: ${JSON.stringify(licenceTextOf(face))}, copyright: ${JSON.stringify(faceCopyright(join(designerRoot, 'public', 'fonts', face.directory, face.file)))}, source: ${JSON.stringify(committedFaceSource(face))}, scripts: [${face.scripts.map((script) => JSON.stringify(script)).join(', ')}], url: catalogueUrl${index} },`).join('\n')
  + `\n]\n`)

// THE BUNDLED DOCUMENTATION: three hand-written HTML pages from the repository's
// `docs/` tree — the rendering library guide, the `.folio` format reference and
// the expression reference — copied into the same immutable runtime tree as the
// engine, so Vite emits them under `/assets/` and the
// offline release precaches them. ONE PAGE IS ONE CACHE SLOT: these three take
// the release up by three.
//
// THE PAGES LINK TO EACH OTHER, SO THEIR FINGERPRINTS WOULD FORM A CYCLE if each
// hashed its own rewritten bytes (A's bytes name B's fingerprint, which depends
// on B's bytes, which name A's). The cycle is broken deterministically:
//   1. every cross-page link is NORMALISED to its canonical sibling name
//      (`folio-format.html#table`; a `.md` sibling link becomes `.html`);
//   2. ONE digest is taken over all the normalised pages together;
//   3. each page's fingerprint is that group digest salted with its own stem;
//   4. only then are the links rewritten to the fingerprinted names.
// A change to any page therefore renames ALL of them, which is what keeps every
// emitted URL immutable: a page whose link target was renamed is itself renamed,
// instead of keeping its URL while its bytes change.
//
// The stem and the fingerprint are joined by a DASH so the verifier's
// content-addressed URL shape holds, and `vite.config.ts` emits these names
// verbatim rather than appending a second hash, because relative links between
// the pages must name exactly the files that exist beside them.
const documentationDir = join(designerRoot, '..', 'docs')
const documentationPages = [['guide', 'rendering-library'], ['js', 'folio-js'], ['dotnet', 'folio-dotnet'], ['format', 'folio-format'], ['expressions', 'expression-reference']]
const documentationLink = new RegExp(`(\\bhref\\s*=\\s*)(["'])(?:\\./)?(${documentationPages.map(([, stem]) => stem).join('|')})\\.(?:html|md)(#[^"']*)?\\2`, 'g')
const canonicalDocumentation = documentationPages.map(([key, stem]) => {
  const source = join(documentationDir, `${stem}.html`)
  if (!existsSync(source)) throw new Error(`docs/${stem}.html is missing; folio8 Designer bundles it as a precached documentation page`)
  const text = readFileSync(source, 'utf8').replace(documentationLink, (_match, attribute, quote, target, fragment = '') => `${attribute}${quote}${target}.html${fragment}${quote}`)
  return { key, stem, text }
})
const documentationDigest = createHash('sha256').update(canonicalDocumentation.map(({ stem, text }) => `${stem}\0${text}`).join('\0')).digest('hex')
const documentationNames = new Map(canonicalDocumentation.map(({ stem }) => [stem, `${stem}-${createHash('sha256').update(`${documentationDigest}\0${stem}`).digest('hex').slice(0, 20)}.html`]))
for (const { stem, text } of canonicalDocumentation) {
  const documentationTarget = documentationNames.get(stem)
  writeFileSync(join(outputDir, documentationTarget), text.replace(documentationLink, (_match, attribute, quote, target, fragment = '') => `${attribute}${quote}${documentationNames.get(target)}${fragment}${quote}`))
}
// A MODULE OF ITS OWN, not a key in `offline-assets.ts`: the engine worker
// imports that module, and the documentation belongs to the application only.
// Every URL is exported so every page stays in Vite's asset graph even
// though the application links only the guide; the others are reached from
// the guide's own relative links.
writeFileSync(join(generatedDir, 'documentation-assets.ts'),
  `// GENERATED by scripts/build-wasm.mjs from docs/*.html. Do not edit.\n`
  + canonicalDocumentation.map(({ key, stem }) => `import ${key}Url from './runtime/${documentationNames.get(stem)}?url'`).join('\n')
  + `\n\nexport const documentationAssetUrls = { ${canonicalDocumentation.map(({ key }) => `${key}: ${key}Url`).join(', ')} } as const\n`)

// THE BUNDLED EXAMPLE TEMPLATES (startup templates). Rendered strictly with the
// engine CLI, thumbnailed from that render, and fingerprinted into the runtime
// tree behind their own generated module; see scripts/build-examples.mjs. Any
// diagnostic fails the build here.
await buildExamples({ designerRoot, generatedDir, fingerprint })

rmSync(wasmPath, { force: true })
rmSync(gluePath, { force: true })
rmSync(starterPath, { force: true })
writeFileSync(join(generatedDir, 'offline-assets.ts'), Object.entries(assets)
  .map(([key, filename]) => `import ${key}Url from './runtime/${filename}?url'`).join('\n') + `\n\nexport const runtimeAssetUrls = { ${Object.keys(assets).map((key) => `${key}: ${key}Url`).join(', ')} } as const\n`)
writeFileSync(join(generatedDir, 'pdfjs-assets.ts'), `// Keep PDF.js CMaps and standard fonts in Vite's immutable asset graph.\nexport const pdfjsRuntimeAssets = import.meta.glob('./runtime/pdfjs-*/**/*', { eager: true, query: '?url', import: 'default' })\nexport const pdfjsViewerAssets = { cMapUrl: '/assets/${pdfjsCMapDirectory}/', standardFontDataUrl: '/assets/${pdfjsStandardFontDirectory}/', cMapPacked: true } as const\n`)
// THIRTEEN RULES, TWO VOCABULARIES (Story 8.4b), NOW OVER DIFFERENT FILES (8.4c).
// The first three register the DESIGN SYSTEM's family names, which is what every
// `--type-*` token in tokens.css resolves through. The other ten register the
// ENGINE's own face names — the exact spellings `fonts.Shipped()` keys its FontSet
// by — so the canvas can ASK FOR THE FACE THE ENGINE MEASURED WITH by name (AD-17
// makes the browser a rasterizer only, and it cannot rasterize with the engine's
// face while it has no way to name it).
//
// THE ENGINE HALF WENT FROM THREE TO TEN AT STORY 11.1. `fonts.Shipped()` now
// carries eleven keys; ten of them need a hand-written rule here and the
// eleventh, `Roboto`, is declared by the CATALOGUE emitter below because it is
// also a `font-catalogue.json` face — a hand-written rule under a family the
// catalogue already declares is the duplicate-`@font-face` hazard the collision
// guard above exists to refuse. That split is what
// `src/font-binary-identity.test.ts`'s mirror guard enforces: every
// `Shipped()` key that is NOT a catalogue family must have a rule here.
//
// Story 8.4b registered both halves over THE SAME THREE FILES, a deliberate
// interval in which the IBM Plex names were IBM Plex in name only — `IBM Plex
// Mono` was Noto Sans SC, a CJK sans with no monospacing. Story 8.4c ended it:
// SIX RULES OVER SIX FILES, each family declared from bytes that call themselves
// by that family's name. Story 11.1's seven cuts extend that discipline rather
// than dilute it: thirteen rules over thirteen files, no file reached twice.
//
// The three Noto slots STAY whatever the chrome points at. The engine half
// declares them, generate-offline-release.mjs requires `/noto-sans.`,
// `/noto-sans-thai.` and `/noto-sans-cjk.` each by name, NFR7 requires CJK
// coverage in the shipped set, and the release verifier requires the CJK face to
// remain the dominant font payload. `sansCjk` in particular now backs ONE rule
// rather than two; deleting it would throw at release-build time.
//
// ONE STATIC FACE PER FAMILY, and no `font-weight`/`font-style` descriptor on
// any rule — including on the seven weighted and sloped cuts, which is the
// point of giving each cut its own family NAME. A `font-weight: 700` descriptor
// under `Noto Sans` would be the other way of doing this and it is the wrong
// one: it puts a weight axis into CSS that the document format excludes, and it
// breaks the no-descriptor convention every rule here is read against. So the
// design system's own weights and italics stay browser-synthesised from ONE
// face exactly as before, and the engine's cuts are separate families the
// canvas asks for BY NAME rather than by descriptor.
// Which file is behind which family name is pinned, family by family, by
// src/font-binary-identity.test.ts, which opens each file and reads its own
// `name` table: a family name is an assertion about bytes, and that is where it
// is checked rather than discovered by a designer squinting at glyphs. Note
// that a cut's own `name` table calls itself by its BASE family — `Noto Sans
// Bold` is name[1] `Noto Sans`, name[2] `Bold` — so the per-cut metadata claim
// is made in src/font-catalogue.test.ts against the intended subfamily and
// weight class rather than against the CSS family string.
// THE COLLISION GUARD'S LIST IS HELD TO THE RULES IT CLAIMS TO DESCRIBE.
// `shippedFamilies` above is what stops a catalogue entry redeclaring one of
// these thirteen; a name that drifts out of either side would make that guard
// silent again, in exactly the way `Object.keys(assets)` did. Checked here,
// where both the list and the template are in scope.
const shippedRules = `@font-face { font-family: 'IBM Plex Sans'; src: url('./runtime/${assets.plexSans}') format('truetype'); font-display: swap; }\n@font-face { font-family: 'IBM Plex Mono'; src: url('./runtime/${assets.mono}') format('truetype'); font-display: swap; }\n@font-face { font-family: 'IBM Plex Sans Thai'; src: url('./runtime/${assets.plexSansThai}') format('truetype'); font-display: swap; }\n@font-face { font-family: 'Noto Sans'; src: url('./runtime/${assets.sans}') format('truetype'); font-display: swap; }\n@font-face { font-family: 'Noto Sans Thai'; src: url('./runtime/${assets.sansThai}') format('truetype'); font-display: swap; }\n@font-face { font-family: 'Noto Sans SC'; src: url('./runtime/${assets.sansCjk}') format('truetype'); font-display: swap; }\n@font-face { font-family: 'Noto Sans Bold'; src: url('./runtime/${assets.sansBold}') format('truetype'); font-display: swap; }\n@font-face { font-family: 'Noto Sans Italic'; src: url('./runtime/${assets.sansItalic}') format('truetype'); font-display: swap; }\n@font-face { font-family: 'Noto Sans Bold Italic'; src: url('./runtime/${assets.sansBoldItalic}') format('truetype'); font-display: swap; }\n@font-face { font-family: 'Noto Sans Thai Bold'; src: url('./runtime/${assets.sansThaiBold}') format('truetype'); font-display: swap; }\n@font-face { font-family: 'Roboto Bold'; src: url('./runtime/${assets.robotoBold}') format('truetype'); font-display: swap; }\n@font-face { font-family: 'Roboto Italic'; src: url('./runtime/${assets.robotoItalic}') format('truetype'); font-display: swap; }\n@font-face { font-family: 'Roboto Bold Italic'; src: url('./runtime/${assets.robotoBoldItalic}') format('truetype'); font-display: swap; }\n`
for (const family of shippedFamilies) if (!shippedRules.includes(`font-family: '${family}'`)) throw new Error(`shippedFamilies names ${JSON.stringify(family)} and no hand-written @font-face rule declares it, so the catalogue's collision guard is describing a family that is not there`)
if (shippedRules.split('@font-face').length - 1 !== shippedFamilies.length) throw new Error(`the hand-written stylesheet emits ${shippedRules.split('@font-face').length - 1} rules and shippedFamilies names ${shippedFamilies.length}, so a rule exists that the catalogue's collision guard does not know about`)
writeFileSync(join(generatedDir, 'runtime-fonts.css'), shippedRules
  // AND THE CATALOGUE, one rule per declared face, emitted from the manifest
  // rather than written out. Same shape as the thirteen above, deliberately: no
  // `font-weight`, no `font-style`, one static Regular per family (AC6).
  + catalogueFaces.map((face) => `@font-face { font-family: '${face.family}'; src: url('./runtime/${face.filename}') format('truetype'); font-display: swap; }\n`).join(''))

// THE FAMILY INDEX SNAPSHOT MODULE, emitted beside the catalogue module and
// from committed data alone — no network, because an offline release build is a
// shipped gate. See scripts/build-font-index.mjs for what is committed and why.
emitFontIndexModule()
