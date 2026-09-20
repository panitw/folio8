import crypto from 'node:crypto'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { createElement } from 'react'
import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { TextPaint } from './App'
import { embeddedFaceFamily } from './embedded-face-family'
import { canvasFragmentFallbackStack, isShippedFaceName, shippedFaceFamily } from './shipped-face-family'
import type { CanvasProjection } from './engine-protocol'

// The canvas paints each engine-supplied fragment as an absolutely
// positioned span at the x the ENGINE measured. The browser therefore
// contributes rasterization only (AD-17) — but it must rasterize with the
// SAME faces the engine measured with, or every fragment's drawn width
// disagrees with the x of the fragment after it and the two collide.
//
// THE DEFECT THIS PINS, because it shipped and a person reported it.
// `scripts/build-wasm.mjs` registered the three shipped Noto faces under
// IBM PLEX family names only (the design system's vocabulary). `App.css`
// asked for them under NOTO names, which nothing declared a face for, so the
// browser fell through to generic `sans-serif` — a system Thai face with
// different metrics. Latin looked fine (the fallback's Latin is close enough
// to pass a glance); Thai overlapped at exactly the fragment boundaries,
// which sit at spaces. Reported as "letters rendered on top of each other"
// around "พระราชบัญญัติ การทวงถามหนี้".
//
// AND WHAT STORY 8.4b CHANGED. Asking under the Noto names is now CORRECT:
// the generator declares the same three files a second time under the
// engine's own face names, so the stack below names exactly what the engine
// measures with. The historical defect is preserved here because the guards
// in this file are shaped by it — the failure was silent, cosmetic-looking,
// and script-dependent, which is why every claim here is a checked one.
//
// WHY THIS TEST READS THE GENERATOR AND NOT ITS OUTPUT.
// `src/generated/runtime-fonts.css` is gitignored and only exists after
// `build:wasm`. Asserting against it would make this guard's strength
// depend on build order, and a missing file is the classic way a guard
// goes quietly vacuous. Both files read here are tracked sources.
const here = path.dirname(fileURLToPath(import.meta.url))
const generatorPath = path.join(here, '..', 'scripts', 'build-wasm.mjs')
const cssPath = path.join(here, 'App.css')
// THE ENGINE'S OWN AUTHORITY FOR A FACE NAME, read rather than restated.
// `fonts.Shipped()` is the single machine-readable enumeration of the faces
// this build measures with; there is no exported constant, JSON registry or
// generated artifact carrying the same three names. An earlier form of the
// test below hardcoded them, which meant it would have gone on passing —
// while having become false — the moment folio-go shipped a different set.
const enginePath = path.join(here, '..', '..', 'folio-go', 'fonts', 'fonts.go')
// THE OTHER HALF OF `Shipped()` (spec-deferred-offline-cache, story 5): the
// `//go:build !nocjkface` file that embeds the CJK face and merges it in. See
// `shippedFaceNames` for why this half and not its twin.
const buildTaggedFacesPath = path.join(here, '..', '..', 'folio-go', 'fonts', 'notosanssc.go')
const buildTaggedFacesSource = fs.readFileSync(buildTaggedFacesPath, 'utf8')
// The design system's own vocabulary, read rather than restated: the three
// IBM Plex families must remain named by a `--font-*` token here, or the two
// vocabularies this story deliberately keeps apart have been collapsed.
const tokensPath = path.join(here, 'tokens.css')
const catalogueJsonPath = path.join(here, '..', 'font-catalogue.json')
// The catalogue's own copy of Roboto, byte-identical (Story 16.8) to the one
// folio-go embeds as its fourth shipped face.
const catalogueEngineRobotoFile = 'public/fonts/roboto/Roboto-Regular.ttf'

/**
 * Family names `font-catalogue.json` declares, read as data.
 *
 * STORY 16.8'S OWN SEAM. Roboto is the first face this repository ships
 * under BOTH vocabularies at once: a catalogue face (`AVAILABLE LOCALLY`,
 * Story 8.5) AND, as of this story, an engine-shipped face
 * (`fonts.Shipped()`). Its browser `@font-face` comes from the CATALOGUE
 * emitter's templated rule in `scripts/build-wasm.mjs`
 * (`catalogueFaces.map(...)`), never from a seventh hand-written rule — a
 * hand-written rule naming a family the catalogue already declares is
 * exactly the duplicate `@font-face` build-wasm.mjs's own collision guard
 * (`catalogueFamilies.has(entry.family)`) refuses. `declaredFamilies` below
 * therefore cannot see it: it parses literal `font-family: '...'` text out
 * of the generator SOURCE, and the catalogue's rule is templated
 * (`font-family: '${face.family}'`), resolving to real names only once the
 * script actually runs. This function is the other side of that same gap
 * closed for `fonts.Shipped()`'s hand-written half: read the data the
 * catalogue rule is generated FROM, rather than the generated rule's own
 * unresolved source text.
 */
function catalogueDeclaredFamilies(): ReadonlyArray<string> {
  const catalogue = JSON.parse(fs.readFileSync(catalogueJsonPath, 'utf8')) as ReadonlyArray<{ family: string }>
  return catalogue.map((entry) => entry.family)
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
 * Face name -> the file `fonts.go` embeds for it, joining `Shipped()`'s map
 * through the //go:embed directives.
 *
 * DUPLICATED FROM font-binary-identity.test.ts, DELIBERATELY — the same
 * self-containment reason `withoutComments` is duplicated below: this file
 * must be able to redden on its own.
 */
function shippedFacePaths(fontsGo: string): Readonly<Record<string, string>> {
  const embeds = Object.fromEntries([...fontsGo.matchAll(/\/\/go:embed\s+(\S+)\s*\nvar\s+(\w+)\s+\[\]byte/g)].map((match) => [match[2], match[1]]))
  const body = /func Shipped\(\) folio8\.FontSet \{[\s\S]*?\n\}/.exec(fontsGo)?.[0] ?? ''
  return Object.fromEntries([...body.matchAll(/"([^"]+)":\s*(\w+),/g)].map((match) => [match[1], embeds[match[2]] ?? `<no //go:embed for ${match[2]}>`]))
}

// THE ONE MODULE ALLOWED TO REGISTER A FACE WHILE A DOCUMENT IS OPEN
// (Story 8.4a). Named here rather than described, because the claim these
// tests make is not "registration is rare" but "registration is HERE".
const runtimeRegistrationSeam = 'embedded-face-registry.ts'

// THE ONE MODULE ALLOWED TO TURN A SHIPPED FACE'S ENGINE NAME INTO A CSS
// FAMILY (Story 8.4e). Named for the same reason: the claim is not "the
// derivation is simple" but "the derivation is HERE". Its carried-face twin is
// `embedded-face-family.ts`, named by the census guard below.
const shippedDerivationSeam = 'shipped-face-family.ts'

/**
 * Family names the generator actually declares an @font-face for.
 *
 * HAND IT `withoutComments(...)` OUTPUT, ALWAYS. It reads TEXT, so a rule that
 * has been commented out reads exactly like a live one — measured, not feared:
 * commenting out the three engine-named rules and leaving them as comment text
 * left every test in this file and in font-binary-identity.test.ts green while
 * the emitted stylesheet dropped to three rules, reproducing the very
 * Thai-overlap defect these guards exist to prevent. The red-proof for that is
 * its own test below.
 */
function declaredFamilies(generator: string): ReadonlyArray<string> {
  return [...generator.matchAll(/@font-face \{ font-family: '([^']+)'/g)].map((m) => m[1])
}

/**
 * Families declared by a rule in the WHOLE exact spelling the intent fixes —
 * `src` a `./runtime/` interpolation included. `declaredFamilies` above matches
 * the rule PREFIX only, so the two differ exactly by the rules that escape the
 * family->file join in font-binary-identity.test.ts. Their equality is asserted
 * below, and is the ceiling a `>=` floor cannot state.
 *
 * TWO INTERPOLATIONS ARE WELL FORMED, NOT ONE, SINCE STORY 8.5:
 *
 *   - `${assets.<slot>}` — the six hand-written rules, whose slots
 *     `font-binary-identity.test.ts` joins family by family to a source file.
 *   - `${face.filename}` — the one rule the CATALOGUE emitter templates, looped
 *     over `font-catalogue.json`. Twenty-one faces are a list, not a
 *     vocabulary, and writing them out as twenty-one more `assets` keys is the
 *     shape Story 8.5's Design Note 4 exists to refuse.
 *
 * WHAT THE WIDENING DOES NOT GIVE UP, which is the whole reason this guard
 * exists: the `src` must still be a `./runtime/` path built from an
 * interpolated fingerprint. A rule with a LITERAL src — a family fetched from
 * an arbitrary URL rather than from the offline asset graph — is still
 * invisible to this parse, still visible to `declaredFamilies`, and still
 * reddens the equality below. The catalogue's own binaries are held to their
 * committed bytes by `src/font-catalogue.test.ts`, which is the join
 * `${assets.<slot>}` buys for the other six.
 */
function wellFormedRuleFamilies(generator: string): ReadonlyArray<string> {
  return [...generator.matchAll(/@font-face \{ font-family: '([^']+)'; src: url\('\.\/runtime\/\$\{(?:assets\.\w+|face\.filename)\}'\) format\('truetype'\); font-display: swap; \}/g)].map((m) => m[1])
}

/**
 * Every `--font-*` custom property in tokens.css, with the value it is given.
 *
 * `var(--font-sans)` inside a `--type-*` token is deliberately not matched: the
 * pattern requires the `:` of a DECLARATION, so this answers to where a family
 * is NAMED rather than to where one is referenced.
 */
function fontTokenValues(tokens: string): ReadonlyArray<readonly [string, string]> {
  return [...tokens.matchAll(/(--font-[\w-]+)\s*:\s*([^;}]+)/g)].map((m) => [m[1], m[2].trim()] as const)
}

/** Quoted families the canvas fragment rule asks for, in order. */
function requestedFamilies(css: string): ReadonlyArray<string> {
  const rule = css.split('\n').find((line) => line.startsWith('.canvas-text-fragment {'))
  if (rule === undefined) throw new Error('no .canvas-text-fragment rule in App.css')
  const declaration = /font-family:([^;]+);/.exec(rule)
  if (declaration === null) throw new Error('.canvas-text-fragment declares no font-family')
  return [...declaration[1].matchAll(/'([^']+)'/g)].map((m) => m[1])
}

/**
 * Whether a source registers a font face AT RUNTIME — the mechanism Story 8.4a
 * needs and this build has none of.
 *
 * Three spellings, because the disclosure names three: the `FontFace`
 * constructor, `document.fonts.add`, and an `@font-face` rule injected as text
 * whose `src` is a `data:` or `blob:` URL (a build-time `@font-face` points at
 * a bundled asset path, so the URL scheme is what separates the two).
 */
function registersAFaceAtRuntime(source: string): boolean {
  if (/new FontFace\b|document\.fonts\.add\b/.test(source)) return true
  // The injected-rule form: an `@font-face` and, within the same rule, a `src`
  // fed from a data/blob URL. Bounded rather than greedy so a build-time
  // `@font-face` early in a file cannot pair with an unrelated `data:` URL far
  // below it.
  return /@font-face[\s\S]{0,400}?src\s*:[^;}]{0,200}?(?:data:|blob:)/.test(source)
}

// withoutComments strips line and block comments while leaving string and
// template literals intact, so a scan over source text answers to the CODE
// rather than to the prose describing it.
//
// IT IS DUPLICATED FROM canvas-authority-contract.test.ts, DELIBERATELY, and
// the alternatives are both worse. Importing it from that file would register
// its whole suite a second time under this one; hoisting it into a shared
// non-test module would put a test helper into `src/`, where it would enter
// the very production corpus these scans walk. A character scanner rather than
// a regex because a regex cannot tell `// a comment` from the `//` inside a
// URL string, and getting that backwards makes the guard vacuous exactly where
// it matters.
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

/** Every non-test designer source, PAIRED WITH ITS NAME so a scan can say where. */
function designerSources(): ReadonlyArray<readonly [string, string]> {
  return fs.readdirSync(here, { recursive: true })
    .filter((entry): entry is string => typeof entry === 'string' && /\.(?:ts|tsx)$/.test(entry) && !/\.test\.(?:ts|tsx)$/.test(entry))
    .map((entry) => [entry, fs.readFileSync(path.join(here, entry), 'utf8')] as const)
}

// The named sources that register a face at runtime, ANSWERING TO THE CODE
// RATHER THAN TO THE PROSE. Comments are stripped before the detector sees
// them, and that is not tidiness: measured, this file's own seam module
// describes its exception in a comment that spells `new FontFace`, so a raw
// scan reported the seam as registering even after the registration had been
// taken out of it — a guard that would have stayed green through the removal
// of the thing it guards. The detector itself is untouched (it is proved
// against its own fixtures below); only what is handed to it is.
function runtimeRegistrationSites(sources: ReadonlyArray<readonly [string, string]>): string[] {
  return sources.filter(([, source]) => registersAFaceAtRuntime(withoutComments(source))).map(([name]) => name)
}

// Every CSS font-family DECLARATION position in a TypeScript source: the
// `fontFamily:` of an inline style object and the `font-family:` of a CSS
// string, with the value it is being given. It deliberately does not match
// `fontFamily?:` (a type member), `'fontFamily',` (a key in a list) or
// `'fontFamily' |` (a union member) — none of those declares anything to the
// browser, and the projection's `fontFamily` field, which names a document's
// CHAIN, is read all over this codebase without ever reaching CSS.
//
// The lookbehind is measured rather than defensive: without it the scan read
// `'property-error-fontFamily' : undefined` — a ternary on an identifier that
// merely ENDS in the word — as a declaration of `undefined`, which is a green
// guard reporting a position that does not exist and a red one the moment an
// unrelated id is renamed.
function fontFamilyDeclarations(source: string): ReadonlyArray<string> {
  return [...withoutComments(source).matchAll(/(?<![\w-])(?:font-family|fontFamily)['"\]]?\s*:\s*([^,;}\n]*)/g)].map((match) => match[1].trim())
}

// The ONLY value a font-family position may be given in designer source: the
// family derived from the asset key the ENGINE attributed this fragment to.
const assetKeyDerivedFamily = /^embeddedFaceFamily\([A-Za-z][A-Za-z0-9_]*\.assetKey\)$/

// AND THE SECOND, ADDED BY STORY 8.4e — a CLOSED SET OF EXACTLY TWO, never a
// containment. The fragment's family moves from the stylesheet INTO an inline
// style for the shipped population too, and an inline family string escapes an
// App.css-only scan without anyone editing a guard. This census is what
// notices, so it admits exactly the two derivations the engine's two
// identities have and nothing else: the shipped face's name, taken from the
// fragment's own `face` field, through the one module that decides it.
//
// ANCHORED TO `fragment` BY NAME, AND NOT TO ANY IDENTIFIER THAT ENDS IN
// `.face`. A chain ENTRY has a `face` too, so `shippedFaceFamily(entry.face)`
// is a per-COMPONENT, chain-entry-derived family — the exact evasion this
// census exists to catch, and the one the intent forbids twice over (never
// from a chain entry, never per component). A pattern admitting any bare
// identifier waved it straight through while reading as a closed set.
const shippedFaceDerivedFamily = /^shippedFaceFamily\(fragment\.face\)$/

// AND THE THIRD, ADDED BY STORY 16.3 — AND IT IS NOT ABOUT THE CANVAS AT ALL.
//
// The font browser sets each specimen in its own family, which is the one thing
// that makes the screen worth having: a name in a list is a guess, and a name
// set in its own face is not. That needs a third `font-family` position in
// designer source, and this census was previously a closed set of exactly two.
//
// WHY IT DOES NOT REOPEN WHAT THE OTHER TWO CLOSE. The two above are about the
// CANVAS, where the rule is that a fragment's family may come only from the
// ENGINE's identity for the face it measured with — never from a chain entry, a
// chain name or the document's own vocabulary. This one names nothing a document
// owns and nothing the canvas paints: `previewFaceFamily` is an injective hex
// encoding of a snapshot family name behind a `folio8-preview-` prefix, in a
// namespace disjoint from the carried one, over a face registered only while a
// modal row is on screen. A preview family reaching a canvas fragment would fail
// the two patterns above exactly as any other unapproved value does.
//
// ANCHORED TO `row.family`, never to a bare identifier. The near-miss it
// would otherwise wave through — `previewFaceFamily(entry.family)` over a
// CHAIN entry, a document's own vocabulary wearing an approved call's
// spelling — is red by the same anchoring that keeps `shippedFaceFamily(
// entry.face)` red. STORY 16.7 ADMITS IT IN A SECOND FILE: the family
// control's own `AVAILABLE LOCALLY` rows are described by `browserRows` —
// the exact function `FontBrowser.tsx`'s rows already come from — so their
// `row.family` is the same offered-catalogue vocabulary, never a document's.
const previewDerivedFamily = /^previewFaceFamily\(row\.family\)$/

// AND THE FOURTH, ADDED BY STORY 16.7, FOR A SURFACE THAT IS NOT THE CANVAS
// EITHER — a declared chain's OWN specimen, drawn on the family control's
// dropdown row.
//
// WHY THIS DOES NOT REOPEN THE HAZARD `shippedFaceDerivedFamily` GUARDS. That
// pattern is anchored to `fragment` because a chain entry has not been
// attributed by the engine to any PAINTED text, and letting one stand in for
// `fragment.face` would let a fallback the engine never chose for THIS
// glyph run collide, at the pixel, with widths the engine measured under a
// different face (the whole reason `TextPaint` trusts only its own
// fragment). `declaredEntry.face` below never reaches that path: it names
// the CSS family of one `aria-hidden` `<span>` holding FIXED sample text,
// nothing about it is measured, and there is no sibling fragment position
// for a mismatched entry to disagree with. It is the chain's own declared
// primary — Design Note 3 of Story 16.7 is that the menu and the canvas
// should agree on what a chain paints with, not a claim about a specific
// glyph run.
//
// ANCHORED TO `declaredEntry`, A NAME THIS CENSUS HAS NEVER SEEN BEFORE, and
// deliberately not to `entry` — the identifier `shippedFaceDerivedFamily`
// above poisons by name for exactly the canvas hazard this derivation does
// not carry. Two different provenances get two different names rather than
// one name doing double duty; `entry.face` stays red, unapproved, exactly as
// it did before this story.
const declaredChainShippedFamily = /^shippedFaceFamily\(declaredEntry\.face\)$/

const approvedFontFamilyDerivations = [assetKeyDerivedFamily, shippedFaceDerivedFamily, previewDerivedFamily, declaredChainShippedFamily] as const

function unapprovedFontFamilyDeclarations(source: string): ReadonlyArray<string> {
  return fontFamilyDeclarations(source).filter((value) => !approvedFontFamilyDerivations.some((approved) => approved.test(value)))
}

// A 64-character asset key, the shape the engine projects and the format's own
// rule produces (the lowercase hex SHA-256 of the face's decoded bytes).
const carriedKey = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'

// paintedFragmentFamilies renders the real component the canvas renders and
// reads the family off the DOM. It is the one assertion in this file that is
// not a text scan: what the browser is ASKED for is a rendered fact, and a
// scan of App.tsx could only ever say that a plausible line is present.
function paintedFamilies(attribution: Readonly<{ assetKey?: string; face?: string }>, registered: ReadonlySet<string>): ReadonlyArray<string> {
  const component = {
    id: 'e1', type: 'text', band: 'content', x: 0, y: 0, width: 72_000, height: 24_000, resizable: true,
    textPaint: { overflow: false, truncated: false, lines: [{ top: 0, baseline: 10_000, advance: 12_000, width: 30_000, fragments: [{ text: 'สัญญา', x: 0, ...attribution }] }] },
  } as unknown as CanvasProjection['components'][number]
  const { container } = render(createElement(TextPaint, { component, carriedFaces: registered, zoom: 1 }))
  return Array.from(container.querySelectorAll('.canvas-text-fragment')).map((node) => (node as HTMLElement).style.fontFamily)
}

function paintedFragmentFamilies(assetKey: string | undefined, registered: ReadonlySet<string>): ReadonlyArray<string> {
  return paintedFamilies(assetKey === undefined ? {} : { assetKey }, registered)
}

/** The families a painted SHIPPED-face fragment asks for, in order. */
function paintedShippedFragmentFamilies(face: string | undefined): ReadonlyArray<ReadonlyArray<string>> {
  return paintedFamilies(face === undefined ? {} : { face }, new Set()).map(familiesIn)
}

// THE FAMILY SEQUENCE A CSS font-family VALUE NAMES, quotes removed. jsdom
// re-spells single quotes as double ones when it reads a declaration back, so
// comparing the raw string would pin a CSSOM formatting detail rather than the
// claim — which is about WHICH families are asked for, and in what order.
function familiesIn(value: string): ReadonlyArray<string> {
  return value === '' ? [] : value.split(',').map((entry) => entry.trim().replace(/^['"]|['"]$/g, ''))
}

describe('the canvas paints with the faces the engine measured', () => {
  const generator = fs.readFileSync(generatorPath, 'utf8')
  const css = fs.readFileSync(cssPath, 'utf8')
  // COMMENTS STRIPPED BEFORE THE GENERATOR IS PARSED, for the same measured
  // reason the designer-source scans below strip them: the parse must answer to
  // the CODE that emits the stylesheet, not to prose that merely spells a rule.
  const declared = declaredFamilies(withoutComments(generator))
  const requested = requestedFamilies(css)
  const engineFaces = shippedFaceNames(fs.readFileSync(enginePath, 'utf8'))

  // Vacuity guard: neither side may be empty, or the assertion below
  // passes by having nothing to compare. A regex that stops matching
  // because a file's shape changed is the failure mode this catches.
  //
  // THE FLOOR IS THIRTEEN, RAISED FROM SIX BY STORY 11.1 — which raised it from
  // three at Story 8.4b. Thirteen is the true count: three design-system
  // families, the three Story 2.2 engine faces, and seven weighted and sloped
  // cuts, each over bytes of its own. A floor that sits below the real number
  // has stopped discriminating; left at six it would have survived the deletion
  // of every cut this story adds.
  it('reads a non-empty declaration set from the generator', () => {
    expect(declared.length).toBeGreaterThanOrEqual(13)
  })

  // AND A CEILING, NOT ONLY A FLOOR — closing the route a floor cannot.
  //
  // MEASURED AT STORY 8.4b'S CLOSE, and it is why this is a separate test.
  // `declaredFamilies` matches only the RULE PREFIX, so it sees any
  // `@font-face` whatever; the strict, whole-rule parse in
  // font-binary-identity.test.ts sees only rules whose `src` interpolates an
  // `assets` slot, and its `toBe(6)` and exact family->file map are blind to a
  // rule that does not. A SEVENTH rule with a LITERAL src — a family the
  // browser can be asked for, fetched from an arbitrary URL rather than from
  // the offline asset graph — was added to the generator and every test in
  // both files stayed GREEN while the emitted stylesheet carried seven rules.
  // A review finding that named this route was rejected on the ground that
  // the bound "exists one file over"; it does not, for this shape of rule.
  //
  // The bound is stated as an EQUALITY between the loose parse and the strict
  // one: every family the generator declares must come from a rule in the
  // exact spelling the intent's Always clause fixes. That is also what makes
  // the two files' parses answer to the same set of rules rather than to two
  // different ones.
  it('declares no @font-face outside the exact rule spelling the guards parse', () => {
    const wellFormed = wellFormedRuleFamilies(withoutComments(generator))
    // FOURTEEN SINCE STORY 11.1: the thirteen hand-written rules — six from
    // Stories 8.4c/8.5 and seven weighted and sloped cuts — plus the ONE
    // catalogue rule the emitter templates over `font-catalogue.json`. It is a
    // count of RULE SPELLINGS IN THE GENERATOR, not of emitted rules — the
    // catalogue's one hundred and seven are counted where they are declared,
    // in `src/font-catalogue.test.ts`, against the manifest and the binaries —
    // 31 until spec-install-all-face-cuts story 3 gave the committed tier the
    // cuts its families publish, and ONE rule spelling either way, because the
    // emitter templates over the manifest rather than writing a rule out.
    expect(wellFormed.length, `read no well-formed @font-face rules out of ${generatorPath}`).toBe(14)
    expect(
      declared,
      'the generator declares an @font-face whose src is not a `${assets.<slot>}` interpolation, so it is invisible to the '
      + 'family->file join in font-binary-identity.test.ts: the browser would be offered a family backed by bytes nothing '
      + 'in this repository can name, fingerprint or ship offline.',
    ).toEqual(wellFormed)
  })

  it('reports a family declared by a rule outside that spelling, and accepts one inside it', () => {
    const good = "@font-face { font-family: 'Noto Sans'; src: url('./runtime/${assets.sans}') format('truetype'); font-display: swap; }"
    const stray = "@font-face { font-family: 'Comic Sans MS'; src: url('https://example.invalid/c.ttf') format('truetype'); font-display: swap; }"
    expect(wellFormedRuleFamilies(good)).toEqual(['Noto Sans'])
    expect(declaredFamilies(good)).toEqual(['Noto Sans'])
    // The stray is SEEN by the loose parse and INVISIBLE to the strict one,
    // which is exactly the gap; the equality above is what turns it red.
    expect(declaredFamilies(`${good}\n${stray}`)).toEqual(['Noto Sans', 'Comic Sans MS'])
    expect(wellFormedRuleFamilies(`${good}\n${stray}`)).toEqual(['Noto Sans'])

    // AND THE CATALOGUE'S OWN SHAPE, both directions (Story 8.5). The templated
    // rule is well formed; the same rule with its `src` repointed at a live
    // font service — the shape D-8.5.12 declined and the forbidden-host scan
    // watches for — is not, and reddens the equality above.
    const catalogued = "@font-face { font-family: '${face.family}'; src: url('./runtime/${face.filename}') format('truetype'); font-display: swap; }"
    const fetched = "@font-face { font-family: '${face.family}'; src: url('https://fonts.example.invalid/${face.id}.ttf') format('truetype'); font-display: swap; }"
    expect(wellFormedRuleFamilies(catalogued)).toEqual(['${face.family}'])
    expect(wellFormedRuleFamilies(fetched)).toEqual([])
    expect(declaredFamilies(fetched)).toEqual(['${face.family}'])
  })

  it('reads a non-empty request list from the canvas rule', () => {
    expect(requested.length).toBeGreaterThanOrEqual(3)
  })

  // THE GENERATOR PARSE ANSWERS TO THE CODE, NOT TO THE PROSE.
  //
  // MEASURED, NOT FEARED. Commenting out the three engine-named `@font-face`
  // rules in `scripts/build-wasm.mjs` — leaving the text in place as a comment
  // — left every test in this file and in `font-binary-identity.test.ts` green
  // while the emitted stylesheet dropped from six rules to three, which is
  // exactly the state that shipped the reported Thai overlap. The floor of six
  // above cannot catch it on its own: a commented-out rule still counts as a
  // declaration to a raw text scan. This is the direction that proves it does
  // not any more.
  it('does not count a commented-out @font-face rule as a declared family', () => {
    const emitted = "@font-face { font-family: 'Noto Sans'; src: url('./runtime/x.ttf') format('truetype'); font-display: swap; }"
    // The live direction, so the parse is shown to find a rule at all.
    expect(declaredFamilies(withoutComments(emitted))).toEqual(['Noto Sans'])
    // Both comment forms a generator can hide a rule in.
    expect(declaredFamilies(withoutComments(`// ${emitted}`))).toEqual([])
    expect(declaredFamilies(withoutComments(`/* ${emitted} */`))).toEqual([])
    // AND THE DEFECT ITSELF: without the strip, commented-out text counted.
    expect(declaredFamilies(`// ${emitted}`)).toEqual(['Noto Sans'])
  })

  // NO CHROME TOKEN IS EDITED — CHECKED, NOT ASSERTED IN PROSE.
  //
  // Story 8.4b's whole claim is that the engine's vocabulary became nameable in
  // the browser with NO chrome token touched. Nothing checked the second half:
  // `design-contract.test.ts` pins the token NAMES and the `@import` line, not
  // their values, so repointing `--font-sans` at `'Noto Sans'` — collapsing the
  // two vocabularies this story deliberately keeps separate — would have passed
  // every existing test. tokens.css is READ here and never edited.
  it('keeps each design-system family named by a --font-* token in tokens.css', () => {
    // COMMENTS STRIPPED HERE TOO, and for the measured reason the generator
    // parse is stripped: this reads TEXT, so a token declaration that has been
    // commented out reads exactly like a live one. Measured at Story 8.4b's
    // close — commenting out the live `--font-sans` line and leaving it in the
    // file as comment text, while the replacement named no IBM Plex family,
    // left this whole file GREEN over a chrome vocabulary that had lost its
    // primary family and every `--type-*` token that resolves through it. The
    // containment half below is the direction that goes quiet, because the
    // string it looks for survives in the comment. Red-proof below.
    const fontTokens = fontTokenValues(withoutComments(fs.readFileSync(tokensPath, 'utf8')))
    // NON-VACUITY FLOOR. A reformat that stops the parse matching yields an
    // empty list over which every `not.toEqual([])` below would fail — which is
    // the point: this must redden rather than pass over an empty parse.
    expect(fontTokens.length, `read no --font-* tokens out of ${tokensPath}`).toBeGreaterThanOrEqual(3)
    for (const family of ['IBM Plex Sans', 'IBM Plex Mono', 'IBM Plex Sans Thai']) {
      expect(
        fontTokens.filter(([, value]) => value.includes(`'${family}'`)).map(([name]) => name),
        `no --font-* token in tokens.css names '${family}' any more. Story 8.4b declares the ENGINE's face names alongside `
        + 'the design system\'s; it does not replace them. A chrome token pointed at an engine face name collapses the two '
        + 'vocabularies, and every --type-* token resolves through these three.',
      ).not.toEqual([])
    }
    // AND THE OTHER DIRECTION, which is what the hazard actually looks like:
    // no chrome token may name an ENGINE face at all. Measured — the
    // containment check above alone is not enough, because repointing
    // `--font-sans` at 'Noto Sans' leaves `--font-page` still naming
    // 'IBM Plex Sans', so the three families stay present while a chrome token
    // has been collapsed onto the engine's vocabulary. The face names come from
    // `fonts.Shipped()`, not from a literal restated here.
    expect(engineFaces.length, 'the engine face names must have been read').toBe(11)
    for (const face of engineFaces) {
      expect(
        fontTokens.filter(([, value]) => value.includes(`'${face}'`)).map(([name]) => name),
        `a --font-* token in tokens.css names the ENGINE face '${face}'. The chrome and the engine keep separate `
        + 'vocabularies: Story 8.4b declares the engine\'s names for the CANVAS, and a chrome token pointed at one puts '
        + 'the design system\'s type on whichever bytes the engine happens to ship.',
      ).toEqual([])
    }

    // THE RED DIRECTION, through the same helper: a token repointed at the
    // engine's vocabulary no longer names the chrome family.
    expect(fontTokenValues("--font-sans: 'Noto Sans', system-ui, sans-serif;").filter(([, value]) => value.includes("'IBM Plex Sans'"))).toEqual([])
    expect(fontTokenValues("--font-sans: 'IBM Plex Sans', system-ui, sans-serif;").filter(([, value]) => value.includes("'IBM Plex Sans'")).length).toBe(1)

    // AND THE COMMENT DIRECTION, which is what the strip above is for. CSS has
    // one comment form and it is the one a token gets parked in. Unstripped,
    // the parked declaration counts and the containment half above passes over
    // a chrome family no live token names any more.
    const parked = "  /* --font-sans: 'IBM Plex Sans', system-ui, sans-serif; */\n  --font-sans: system-ui, sans-serif;"
    expect(fontTokenValues(withoutComments(parked)).filter(([, value]) => value.includes("'IBM Plex Sans'"))).toEqual([])
    // THE DEFECT ITSELF: without the strip, the commented-out token counted.
    expect(fontTokenValues(parked).filter(([, value]) => value.includes("'IBM Plex Sans'")).map(([name]) => name)).toEqual(['--font-sans'])
    // And the strip leaves a live declaration alone.
    expect(fontTokenValues(withoutComments(parked)).map(([name]) => name)).toEqual(['--font-sans'])
  })

  // GUARD 1, WIDENED BY STORY 8.4a. It used to say only that every family the
  // STYLESHEET asks for is declared by an `@font-face`. That was a tie between
  // two files and nothing more: a family set from TypeScript — which is
  // precisely what 8.4a introduces — escaped it entirely, so the old form
  // would have gone on passing while the canvas asked for a family nothing had
  // registered. It now ties, for a CARRIED face, the family the fragment
  // actually asks for to the ASSET THE ENGINE RESOLVED IT TO.
  //
  // THE TIE IS SCOPED TO THE CARRIED CASE, AND THAT IS STILL DELIBERATE —
  // BUT NOT FOR THE REASON IT USED TO BE. Until Story 8.4b this comment said
  // the universal form was FALSE, because for a shipped face the rule asked
  // for 'IBM Plex Sans' while the engine measured 'Noto Sans', two disjoint
  // vocabularies. THAT IS NO LONGER TRUE. 8.4b registers the same three
  // shipped files a SECOND time under the engine's own face names and points
  // the fragment rule at those names, so the shipped half now asks for
  // exactly the names the engine measures with — checked by the test below,
  // and tied to the engine's bytes by src/font-binary-identity.test.ts.
  //
  // AND STORY 8.4e UNSCOPED IT. What kept the tie to the carried half was the
  // last residual: the fragment stack was a FIXED constant naming all three
  // faces in one order, while a document may declare a chain like
  // ["Noto Sans Thai"] whose covering face is not the stack's first, and the
  // three faces' cmaps genuinely overlap (339 / 529 / 230 codepoints pairwise,
  // measured, all three covering `A` and `5`) — so the engine measured a Latin
  // run with 'Noto Sans Thai' while the browser's Latin-first stack rasterized
  // it with 'Noto Sans'. A shipped fragment now carries the engine's own
  // FontSet name for the face it was measured with, exactly as a carried one
  // carries its asset key, so the per-fragment claim is checkable for BOTH
  // populations and both are checked here.
  it('asks only for families the browser has a face for, and ties every runtime one to the engine\'s own attribution', () => {
    // (a) THE STYLESHEET HALF, unchanged: every family the fragment rule names
    // is one the generator declares an @font-face for.
    expect(requested.filter((family) => !declared.includes(family))).toEqual([])

    // (b) THE RUNTIME HALF. A fragment the engine attributed to an asset the
    // document carries asks for that asset's own derived family — not a
    // stylesheet constant, not a chain name, not the asset's `font.family`.
    expect(paintedFragmentFamilies(carriedKey, new Set([carriedKey]))).toEqual([embeddedFaceFamily(carriedKey)])
    // A fragment the engine attributed to NOTHING is a shipped face and asks
    // for nothing of its own: it falls to App.css's declared stack, checked in
    // (a). This is the shipped-face path, unchanged by this story.
    expect(paintedFragmentFamilies(undefined, new Set([carriedKey]))).toEqual([''])
    // AND THE DEGRADE PATH. An inline declaration REPLACES the rule rather
    // than extending it, so asking for a family whose bytes never arrived
    // would take the fragment off the declared stack onto the browser's
    // default. The family is asked for only once the face is registered.
    expect(paintedFragmentFamilies(carriedKey, new Set())).toEqual([''])

    // (b2) THE SHIPPED HALF OF THE SAME TIE (Story 8.4e). A fragment the
    // engine attributed to a face the BUILD ships asks for that face by the
    // engine's own name, and every family it names is one the generator
    // declares an @font-face for — which is (a)'s claim, now made about the
    // value the browser is actually handed rather than only about the
    // stylesheet. It needs no registration seam: those faces are declared at
    // build time over the engine's own bytes.
    const shipped = paintedShippedFragmentFamilies('Noto Sans Thai')
    expect(shipped).toHaveLength(1)
    expect(shipped[0]![0], 'the attributed face must be asked for FIRST, or a CSS stack\'s first-match-wins search reaches a different one for every overlapping codepoint').toBe('Noto Sans Thai')
    expect(shipped[0]!.filter((family) => family !== 'sans-serif' && !declared.includes(family))).toEqual([])

    // (c) THE OTHER END OF THE TIE: the seam registers under the SAME
    // derivation of the SAME key, through the SAME module. Two derivations
    // that merely agree today are two derivations.
    const seam = fs.readFileSync(path.join(here, runtimeRegistrationSeam), 'utf8')
    const app = fs.readFileSync(path.join(here, 'App.tsx'), 'utf8')
    // STORY 16.3 SPLIT THIS ASSERTION IN TWO WITHOUT WEAKENING IT. The
    // derivation became a PARAMETER of the seam so that the font browser's
    // preview faces can land in a namespace of their own — see
    // `preview-face-family.ts` for why they must — and the seam is still the
    // ONLY module that may touch the page's font set, which is the property the
    // exact list above guards. Both halves of the old single line are still
    // pinned: the face is constructed from the derivation of the key, AND the
    // derivation a caller gets when it asks for nothing is the DOCUMENT's.
    expect(seam).toMatch(/const family = familyFor\(assetKey\)/)
    expect(seam).toMatch(/new FontFace\(family, bytes\)/)
    expect(seam).toMatch(/familyFor: \(assetKey: string\) => string \| undefined = embeddedFaceFamily/)
    expect(seam).toContain('from \'./embedded-face-family\'')
    expect(app).toContain('from \'./embedded-face-family\'')

    // (d) AND IT CANNOT COLLIDE WITH A BUILD-TIME FAMILY. D-8.4.1's own
    // hazard: `document.fonts` is a global name-keyed registry, so a derived
    // family that happened to equal a declared one would silently substitute.
    for (const family of declared) expect(embeddedFaceFamily(carriedKey)).not.toBe(family)
  })

  // DW-35 TRIPWIRE, RE-RECORDED AT STORY 8.4b. It had two causes; cause two is
  // closed, and cause one is now HALF closed. Conflating the closed half with
  // the open one is how the open one disappears.
  //
  // CAUSE ONE, VOCABULARY LAYER (CLOSED BY STORY 8.4b). Until 8.4b the two
  // sides did not merely differ in stack ORDER — they used different NAMES for
  // the same three shipped files: the generator registered them under IBM Plex
  // family names while a chain's entries are the ENGINE's face names, so a
  // chain entry could not be used as a CSS family name AT ALL. The earlier
  // form of this comment called the fix a design-system decision above a
  // builder's authority, needing either a rename of the generated families
  // (rippling into tokens.css and design-contract.test.ts) or a face-name ->
  // CSS-family map. MEASURED FALSE, per D-8.4.14: it needed neither. 8.4b adds
  // a SECOND @font-face over each of the SAME three files under the engine's
  // own face names, so the engine's vocabulary is nameable in the browser with
  // no chrome token edited, no binary added and no mapping table built — a
  // mapping table being a second authority on which browser family is which
  // engine face, rejected by name. The browser family now IS the engine's
  // name, so there is nothing to map.
  //
  // CAUSE ONE, ATTRIBUTION LAYER (CLOSED BY STORY 8.4e). What stood here was a
  // disclosure of absence — it recorded that the fragment stack was a fixed
  // stylesheet constant with no document input, and said in its own words that
  // closing that "is a different story". It was, and this is it. A shipped
  // fragment now carries the engine's `FontSet` name for the face it was
  // measured with, the browser asks for that face FIRST and keeps the declared
  // stack as its tail, and the test below is the disclosure's positive twin:
  // the family DOES come from the projection, through exactly one named seam.
  // The record was retired under its own pre-authorisation rather than
  // softened, and both of the bounds it also carried — no `var(` in the
  // declaration, and the `requested.length >= 3` floor — are kept in the
  // replacement. That is the same move Story 8.4a made on Story 8.4's
  // registration disclosure, one guard down.
  //
  // CAUSE TWO (Story 8.4, CLOSED BY STORY 8.4a). The engine renders — and
  // measures — with a face the DOCUMENT ITSELF CARRIES, decoded out of its
  // `assets` map, and the browser had NOTHING for it: no `@font-face`, no
  // family name, no bytes at all, so the fragment stack fell straight through
  // to generic `sans-serif`. 8.4a carries each fragment's ASSET KEY through
  // the projection, fetches the bytes over the existing `asset` operation, and
  // registers a `FontFace` under a family derived from that key. The guard
  // above is the tie; the guards below are what keep the mechanism from
  // spreading.
  //
  // THE DESIGN DECISION 8.4a INHERITED WAS ALREADY MADE (D-8.4.1): a carried
  // face's CSS family name derives from its ASSET KEY, never from the asset's
  // `font.family`. AD-8 makes the asset key the resolver, and deriving from
  // `font.family` would let a document's "Inter" collide with a shipped
  // "Inter" in the browser's own font registry — AD-8's hazard, one layer down.
  //
  // THE OBSTACLE THAT WAS MEASURED AND IS NOW GONE, kept because the shape of
  // its removal is what a reader needs. It said the two sides did not merely
  // differ in stack ORDER but used different NAMES for the same shipped files,
  // so a chain entry could not be used as a CSS family name at all — and that
  // the fix therefore needed a face-name -> CSS-family mapping existing on
  // NEITHER side, or a rename of the generated families rippling into the
  // design tokens and their contract test. Story 8.4b did a THIRD thing: it
  // added a second `font-face` rule per file under the engine's own name,
  // leaving the IBM Plex rules and every token untouched. A chain entry is now
  // a usable CSS family name.
  //
  // THE ALIASING TRAP IS STILL LIVE, and is now the only reason the two halves
  // must never be collapsed: the generator's `'IBM Plex Mono'` is Noto Sans SC,
  // not a mono face. That defect belongs to Story 8.4c, which puts real IBM
  // Plex bytes behind the IBM Plex names; until then the pairing is pinned,
  // file by file, in src/font-binary-identity.test.ts.
  it('derives a shipped fragment\'s family from the engine\'s attribution, through exactly one named seam', () => {
    // (a) THE TWO BOUNDS THE RETIRED RECORD CARRIED, KEPT RATHER THAN DROPPED.
    // Retiring a disclosure of absence is not licence to lose what it also
    // happened to check. NON-VACUITY FIRST: `find(...)` yields undefined the
    // moment the rule is reformatted onto several lines, and an assertion
    // about undefined proves nothing at all.
    const rule = css.split('\n').find((line) => line.startsWith('.canvas-text-fragment {'))
    expect(rule, 'the single-line .canvas-text-fragment rule must exist').toBeDefined()
    const declaration = /font-family:([^;]+);/.exec(rule as string)?.[1]
    expect(declaration, '.canvas-text-fragment must declare a font-family').toBeDefined()
    // Every family in the FALLBACK is still a literal: no custom property, no
    // interpolation, and no way for a projected chain to reach this
    // declaration. It is now the degrade path rather than the authority, and a
    // `var(` here would empty `requestedFamilies` and quietly vacate the three
    // guards that parse this rule.
    expect(declaration as string).not.toMatch(/var\(/)
    expect(requested.length).toBeGreaterThanOrEqual(3)

    // (b) AND THE POSITIVE CLAIM THAT REPLACES IT. The stack above is no
    // longer where a shipped fragment's family comes from: the family DOES
    // come from the projection now, per fragment, derived from the face name
    // the ENGINE attributed. A stylesheet rule cannot vary per fragment and a
    // document whose chain is ["Noto Sans Thai"] needs it to.
    const attributed = paintedShippedFragmentFamilies('Noto Sans Thai')
    expect(attributed).toEqual([familiesIn(shippedFaceFamily('Noto Sans Thai') as string)])
    expect(attributed[0]![0]).toBe('Noto Sans Thai')
    // A DIFFERENT ATTRIBUTION ASKS FOR A DIFFERENT FACE FIRST — which is the
    // whole difference from a constant, stated as a difference rather than as
    // a single positive reading.
    expect(paintedShippedFragmentFamilies('Noto Sans SC')[0]![0]).toBe('Noto Sans SC')

    // (c) AND THE FALLBACK IS STILL REACHED WHERE IT SHOULD BE. A fragment the
    // engine attributed to NOTHING asks for nothing of its own and falls to
    // the rule above; a face name that is not usable as a CSS family is
    // DECLINED rather than interpolated into an inline declaration.
    expect(paintedShippedFragmentFamilies(undefined)).toEqual([[]])
    expect(shippedFaceFamily('Evil\', sans-serif; background: url(x)')).toBeUndefined()
    expect(paintedShippedFragmentFamilies('Evil\', sans-serif; background: url(x)')).toEqual([[]])

    // (d) THROUGH EXACTLY ONE NAMED SEAM, AND NOWHERE ELSE. This is the
    // scanning power of the record it replaces, re-pointed from "nothing
    // derives a fragment family from the projection" to "exactly this module
    // does". A second derivation site is the mutation it exists to catch.
    const sources = designerSources()
    expect(sources.length).toBeGreaterThan(10)
    expect(sources.map(([name]) => name)).toContain(shippedDerivationSeam)
    const callers = sources.filter(([name, source]) => name !== shippedDerivationSeam && /shippedFaceFamily\(/.test(withoutComments(source))).map(([name]) => name)
    expect(callers, `${shippedDerivationSeam} is the one module that decides a shipped face's CSS family, and App.tsx is the one place that asks it`).toEqual(['App.tsx'])
    const app = fs.readFileSync(path.join(here, 'App.tsx'), 'utf8')
    expect(app).toContain('from \'./shipped-face-family\'')
  })

  // THE FALLBACK TAIL HAS EXACTLY ONE AUTHORITY, ASSERTED ACROSS BOTH SOURCES.
  // The seam's inline value carries the declared stack as its tail so an
  // inline declaration — which REPLACES the rule rather than extending it —
  // still reaches the other shipped faces. That list is therefore spelled
  // twice: once in `.canvas-text-fragment` and once in the seam. Two spellings
  // of one list is two lists, so both are read here and asserted to be the
  // same sequence, entry for entry, in the exact CSS spelling.
  it('spells the fragment fallback stack once: the seam and the stylesheet name the same list, in the same order', () => {
    const rule = css.split('\n').find((line) => line.startsWith('.canvas-text-fragment {'))
    expect(rule, 'the single-line .canvas-text-fragment rule must exist').toBeDefined()
    const declaration = /font-family:([^;]+);/.exec(rule as string)?.[1]
    expect(declaration, '.canvas-text-fragment must declare a font-family').toBeDefined()
    const cssEntries = (declaration as string).split(',').map((entry) => entry.trim())
    expect(cssEntries.length, 'read no entries out of the .canvas-text-fragment declaration').toBeGreaterThanOrEqual(4)
    expect(
      [...canvasFragmentFallbackStack],
      'shipped-face-family.ts and App.css spell the fragment fallback stack differently. A shipped fragment would then fall through a different list from an unattributed one, and the engine-face order guarded above would hold for only one of the two.',
    ).toEqual(cssEntries)
  })

  // THE SUCCESSOR OF THE DISJOINTNESS RECORD (Story 8.4b). What stood here
  // asserted, in both directions, that the engine's face names and the
  // browser's declared families do NOT intersect — the deliberate disjointness
  // that was DW-35 cause one. 8.4b reverses exactly that, so those two
  // assertions could not be edited: they had to go red and be replaced by
  // their opposite. THE CHROME HALF DID NOT GO WITH THEM. The IBM Plex
  // families are still declared and are still what every `--type-*` token in
  // tokens.css resolves through; nothing in this story renames, removes or
  // repoints them, and the arrayContaining floor below is what would notice.
  //
  // THE ENGINE'S NAMES ARE READ, NOT RESTATED. The form this replaces compared
  // a dynamically-read `declared` against a hardcoded three-element literal —
  // which would have kept passing, while having become false, if folio-go ever
  // shipped a different FontSet. Both halves of the claim below come from
  // `fonts.Shipped()` itself.
  it('declares every engine face name, and asks the canvas for the ones the degrade stack carries', () => {
    // NON-VACUITY BEFORE ANYTHING ELSE. A parse that yields nothing makes every
    // `filter(...).toEqual([])` below pass over an empty set, which is the
    // classic way this shape of guard goes quiet.
    expect(engineFaces.length, `read no face names out of Shipped() in ${enginePath}`).toBe(11)

    // THE CHROME HALF, UNWEAKENED. The design system's vocabulary must remain
    // declared; the canvas no longer asks for it, but every type token does.
    expect(declared).toEqual(expect.arrayContaining(['IBM Plex Sans', 'IBM Plex Mono', 'IBM Plex Sans Thai']))

    // SCOPED TO THE HAND-WRITTEN HALF OF `fonts.Shipped()` FROM HERE DOWN
    // (Story 16.8). Roboto joined `fonts.Shipped()` as a face that is ALSO a
    // `font-catalogue.json` catalogue face (`AVAILABLE LOCALLY`) — its
    // `@font-face` is the catalogue emitter's own templated rule, invisible
    // to `declared`'s literal-text parse (see `catalogueDeclaredFamilies`'s
    // note above). Roboto's own ties — that the
    // catalogue declares an @font-face for it, and that the browser has the
    // SAME bytes folio-go embeds — are asserted on their own, immediately
    // after this test, the same way font-binary-identity.test.ts routes them.
    //
    // TEN SINCE STORY 11.1, three before it: the three Story 2.2 Noto faces
    // plus the seven weighted and sloped cuts. Roboto's three CUTS are on this
    // side of the split, not the catalogue side.
    //
    // ⚠ NOT BECAUSE "a bold cut cannot be a catalogue face" — that rule was
    // retired by spec-install-all-face-cuts story 3, which gave every catalogue
    // row its own `style` and the committed tier every cut its families
    // publish. Roboto's three cuts stay here because they already ship as CORE
    // release assets and as `fonts.Shipped()` keys: a catalogue row for them
    // would emit a second byte-identical dist asset apiece, and retiring these
    // would move the designer's 30/30 core-asset pin.
    const catalogueFamilySet = new Set(catalogueDeclaredFamilies())
    const handWrittenEngineFaces = engineFaces.filter((face) => !catalogueFamilySet.has(face))
    expect(handWrittenEngineFaces, 'expected the three Story 2.2 Noto faces plus Story 11.1\'s seven cuts, once the catalogue-declared half (Roboto) is set aside').toHaveLength(10)

    // THE BROWSER CAN NAME THE FACE THE ENGINE MEASURED WITH. Every
    // HAND-WRITTEN-declared face in the shipped FontSet has an @font-face of
    // its own. That those faces are declared from the ENGINE'S OWN BYTES is
    // the separate, stronger claim made in src/font-binary-identity.test.ts.
    expect(handWrittenEngineFaces.filter((face) => !declared.includes(face))).toEqual([])

    // ────────────────────────────────────────────────────────────────────
    // THE FRAGMENT FALLBACK STACK DID NOT MOVE AT STORY 11.1, AND THE TWO
    // ASSERTIONS THAT USED TO RANGE OVER `handWrittenEngineFaces` ARE NARROWED
    // TO THE STACK'S OWN POPULATION RATHER THAN DROPPED (D-11.1.17, D-11.1.19).
    //
    // WHY THE OBVIOUS EDIT IS THE WRONG ONE. `.canvas-text-fragment`'s stack is
    // the DEGRADE PATH: it is what a fragment the engine attributed to NOTHING
    // falls through. Adding the seven cuts to it would change what every
    // unattributed fragment in EVERY EXISTING DOCUMENT rasterizes with — a
    // silent rendering change to documents nobody edited, under a
    // byte-determinism regime whose premise is that output moves only when
    // input does. It would arrive disguised as a tidy-up, and it is written
    // into this story's spec as a `Never`.
    //
    // SO THE CLAIM MOVES RATHER THAN SHRINKS. What the stack is still held to:
    // (a) it names nothing the browser has no hand-written rule for, and
    // (b) the faces it does name appear in `fonts.Shipped()`'s own relative
    //     order — a SUBSEQUENCE now rather than a prefix, because the cuts sit
    //     between `Noto Sans` and `Noto Sans Thai` in the engine's own map.
    // What replaces the dropped half is (c) below: the seven faces the stack
    // does not name are reachable BY ATTRIBUTION, named first, with the whole
    // stack behind them — which is a stronger statement than stack membership,
    // because it is per fragment rather than fixed.
    const stackFaces = handWrittenEngineFaces.filter((face) => requested.includes(face))
    expect(
      requested.filter((family) => !handWrittenEngineFaces.includes(family)),
      'the .canvas-text-fragment stack names a family that is not a hand-written-declared engine face, so a fragment falling through to it asks for something the browser may have no @font-face for',
    ).toEqual([])
    expect(stackFaces, 'the degrade stack is the three Story 2.2 faces and Story 11.1 deliberately did not widen it').toEqual(['Noto Sans', 'Noto Sans Thai', 'Noto Sans SC'])

    // AND IN THE ENGINE'S OWN ORDER, which membership alone does not say.
    // The acceptance criterion and the I/O matrix both require the ORDER, and
    // for good reason: a CSS stack is a first-match-wins search per codepoint,
    // the three faces' cmaps overlap (339 / 529 / 230 codepoints pairwise,
    // measured) and all three cover `A` and `5`. Reordering the stack CJK-first
    // changes which face rasterizes every overlapping codepoint — a metric
    // change the membership assertion above waves straight through. The
    // expected order is `fonts.Shipped()`'s own source order, parsed above and
    // then narrowed to the faces the stack actually names, so this ties the
    // browser's search order to the engine's declaration order rather than to
    // a literal restated here.
    expect(
      requested,
      'the .canvas-text-fragment stack must name its engine faces in the order fonts.Shipped() writes them',
    ).toEqual(stackFaces)

    // (c) THE FACES THE STACK DOES NOT NAME ARE NOT UNREACHABLE — they reach
    // the canvas by ATTRIBUTION. This is the positive claim that replaces the
    // narrowed one, and it is asserted over exactly the faces the narrowing
    // exempted, so the exemption cannot quietly grow to cover a face nothing
    // else checks.
    const outsideTheStack = handWrittenEngineFaces.filter((face) => !requested.includes(face))
    expect(outsideTheStack, 'Story 11.1 put seven cuts outside the degrade stack; if none is outside it, this exemption is describing nothing and the narrowing above is unjustified').toHaveLength(7)
    for (const face of outsideTheStack) {
      const families = familiesIn(shippedFaceFamily(face) as string)
      expect(families[0], `${face} is not in the degrade stack, so the ONLY way the browser ever asks for it is the per-fragment derivation — which must name it FIRST`).toBe(face)
      expect(families.slice(1), `${face}'s derived value must carry the whole declared stack behind it, or a codepoint it does not cover falls to the browser default rather than to the other shipped faces`).toEqual(canvasFragmentFallbackStack.map((entry) => entry.replace(/^'|'$/g, '')))
    }

    // AND TIED TO THE THIRD AUTHORITY TOO — THE BROWSER-SIDE PREDICATE THAT
    // DECIDES WHETHER A FACE NAME CAN BE ASKED FOR AT ALL (Story 8.4e).
    // `shipped-face-family.ts` admits a name by SHAPE, and a name it declines
    // sets NO inline family: that fragment falls silently back to the fixed
    // Latin-first stylesheet stack, which is the whole defect this epic
    // closed. The three ties above cannot notice — a face named
    // `IBM_Plex_Sans`, `Noto Sans 2.0`, or one spelled in its own script, can
    // be declared, requested and first in order while the predicate refuses
    // it, and every assertion in this file stays green. So the engine's own
    // names are read against the predicate as well, and as a SET DIFFERENCE
    // rather than a count: a count is lossy (Design Note 7).
    //
    // THIS PAIR IS OVER ALL ELEVEN `engineFaces`, ROBOTO AND ALL SEVEN CUTS
    // INCLUDED: the shape predicate and the "names itself first" derivation
    // hold for them exactly as they do for the three Story 2.2 faces — nothing
    // about either rule depends on WHERE the browser's @font-face for a name
    // comes from, or on whether the degrade stack happens to name it.
    expect(
      engineFaces.filter((face) => !isShippedFaceName(face)),
      'shipped-face-family.ts declines a face name fonts.Shipped() actually ships. A fragment attributed to it would set no inline family and fall back to the fixed stack, silently, with nothing else in this file red.',
    ).toEqual([])

    // AND IN THE OTHER DIRECTION, because "the predicate says yes" is not
    // "the browser asks for that face". For every shipped face, the value the
    // one seam derives must name THAT face, and name it FIRST — the same
    // first-match-wins reason the order tie above exists.
    expect(
      engineFaces.filter((face) => familiesIn(shippedFaceFamily(face) ?? '')[0] !== face),
      'shipped-face-family.ts does not name every shipped face FIRST in the family value it derives for that face',
    ).toEqual([])
  })

  // ROBOTO'S OWN TIES, SET ASIDE FROM THE TEST ABOVE (Story 16.8). It is the
  // first face to ship under both vocabularies at once — see
  // `catalogueDeclaredFamilies`'s note — so its "does the browser have a
  // face for it" and "are the bytes the SAME bytes" claims are made here,
  // against the catalogue path, rather than folded into the hand-written
  // six-rule machinery the test above exercises.
  it('Roboto: the catalogue declares it, and its bytes are the ones fonts.Shipped() embeds', () => {
    expect(engineFaces, 'fonts.Shipped() must still carry "Roboto" for this test to check anything').toContain('Roboto')
    expect(catalogueDeclaredFamilies(), 'font-catalogue.json must still declare "Roboto" — Story 16.8 does not remove it from the catalogue').toContain('Roboto')

    // THE BYTES ARE THE SAME BYTES ("there is exactly one Roboto"). This is
    // the browser-side twin of folio-go/fonts/fonts_test.go's
    // TestShippedRobotoMatchesDesignerCatalogue and of
    // font-binary-identity.test.ts's own version of this same tie.
    const shippedPaths = shippedFacePaths(fs.readFileSync(enginePath, 'utf8'))
    const engineFile = path.join(here, '..', '..', 'folio-go', 'fonts', shippedPaths.Roboto ?? '')
    const catalogueFile = path.join(here, '..', catalogueEngineRobotoFile)
    expect(fs.existsSync(engineFile), `${engineFile} must exist`).toBe(true)
    expect(fs.existsSync(catalogueFile), `${catalogueFile} must exist`).toBe(true)
    const digest = (file: string) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex')
    expect(
      digest(catalogueFile),
      'the designer catalogue\'s Roboto and the file folio-go embeds as "Roboto" must be byte-identical — a second cut under the same name would make a document naming Roboto render differently depending on which path produced it',
    ).toBe(digest(engineFile))

    // AND THE PREDICATE/DERIVATION HOLD FOR IT, restated here as Roboto's own
    // positive case rather than inferred only from the SET DIFFERENCE
    // assertions in the test above.
    expect(isShippedFaceName('Roboto')).toBe(true)
    expect(familiesIn(shippedFaceFamily('Roboto') as string)[0]).toBe('Roboto')
  })

  // STORY 8.4a'S POSITIVE TWIN OF STORY 8.4'S DISCLOSURE OF ABSENCE.
  //
  // 8.4 recorded that NOTHING in this build registers a face at runtime, and
  // said in its own words that the assertion "will have to be deleted by 8.4a
  // rather than merely edited". It was: an absence assertion cannot survive
  // the thing whose absence it asserts. What survives is its SCANNING POWER,
  // re-pointed from "nowhere" to "exactly here". The detector is unchanged and
  // is still proved against itself below.
  //
  // WHY AN EXACT LIST RATHER THAN A COUNT. The hazard is not that registration
  // happens twice; it is that it happens somewhere with the WRONG LIFETIME.
  // `document.fonts` is a global, name-keyed registry, and the obvious place
  // to copy from — ImagePaint — is mounted once per component AND once per
  // repeated sheet, so a face registered there would be added N x M times
  // under one family and deleted by whichever instance unmounted first, while
  // another was still painting with it. Naming the seam is what makes that
  // mistake visible.
  it('registers a face at runtime in exactly one named seam and nowhere else', () => {
    // Non-vacuity: the generator really does declare the build-time faces, so
    // what follows is a statement about a corpus that has font registration in
    // it, not about an empty or unread one.
    expect(declared.length).toBeGreaterThanOrEqual(3)
    for (const declaration of declared) {
      expect(generator).toContain(`@font-face { font-family: '${declaration}'`)
    }
    // And every @font-face src in the GENERATOR is still a build-time asset,
    // never a document's bytes: the build-time path did not acquire a second
    // mechanism while the runtime one was being added.
    expect(registersAFaceAtRuntime(withoutComments(generator))).toBe(false)
    const sources = designerSources()
    expect(sources.length).toBeGreaterThan(10)
    expect(runtimeRegistrationSites(sources)).toEqual([runtimeRegistrationSeam])
  })

  // THE REPLACEMENT'S OWN RED-PROOF. An exact list is only a guard if both of
  // its failure directions actually fail, and neither can be shown by the real
  // corpus — which has, and must keep having, exactly one site.
  it('turns a second registration site red, and a seam that stops registering red too', () => {
    const seam = [runtimeRegistrationSeam, "const face = new FontFace(embeddedFaceFamily(assetKey), bytes)"] as const
    const benign = ['App.tsx', 'const style = { fontSize: 12 }'] as const
    expect(runtimeRegistrationSites([seam, benign])).toEqual([runtimeRegistrationSeam])
    // A SECOND SITE. This is the mutation the exact list exists to catch —
    // ImagePaint growing a font branch, say.
    const second = ['ImagePaint.tsx', 'document.fonts.add(face)'] as const
    expect(runtimeRegistrationSites([seam, second])).not.toEqual([runtimeRegistrationSeam])
    // AND THE SEAM ITSELF GOING QUIET, which would mean the feature had been
    // removed or renamed out from under this list.
    expect(runtimeRegistrationSites([benign])).toEqual([])
    // A MENTION IS NOT A REGISTRATION. This is the mutation that was actually
    // getting through: a module that only DESCRIBES the mechanism in a comment
    // — as the seam's own header does, and as this file does throughout — must
    // not be counted, or the seam could stop registering and stay on the list.
    expect(runtimeRegistrationSites([['a-module.ts', '// the seam calls new FontFace and document.fonts.add once per carried face']])).toEqual([])
  })

  // THE DETECTOR IS ITSELF ASSERTED, because a negative scan is only as strong
  // as the pattern behind it and a regex that matches nothing passes every
  // "no offender" assertion in this file. Each mechanism the comment above
  // names is shown to be caught, and ordinary source is shown not to be.
  it('detects each of the three runtime-registration mechanisms it claims to scan for', () => {
    for (const mechanism of [
      "const face = new FontFace('x', 'url(data:font/ttf;base64,AA)')",
      "document.fonts.add(face)",
      "style.textContent = `@font-face { font-family: 'x'; src: url(data:font/ttf;base64,AA); }`",
      "sheet.insertRule(\"@font-face { font-family: 'x'; src: url(blob:http://a/b) }\")",
    ]) {
      expect(registersAFaceAtRuntime(mechanism), mechanism).toBe(true)
    }
    for (const benign of [
      "const url = 'data:image/png;base64,AA'",
      "createObjectURL(new Blob([bytes], { type: image.mediaType }))",
      "// the generator writes an @font-face per shipped face at build time",
    ]) {
      expect(registersAFaceAtRuntime(benign), benign).toBe(false)
    }
  })

  // GUARD 2, WIDENED BY STORY 8.4a. It used to say only that no source names a
  // CHAIN ENTRY in a font-family position — a tripwire on one obvious route,
  // which an asset-key-derived family would have walked straight past, green,
  // by simply not being spelled `chain.entries[0]`. It now says the opposite
  // way round, which is the strictly stronger claim: in the canvas's own
  // paint path a font-family position may name ONE thing, the family derived
  // from the asset key the engine attributed, and every other spelling — a
  // chain entry, a chain name, an asset's `font.family`, the projected
  // `fontFamily` field, a literal — is a violation. (Story 16.7 widened the
  // census to two further surfaces that are not the canvas's paint path at
  // all, each admitted by its own narrowly-anchored pattern below — never by
  // loosening this one.)
  //
  // AND IT NO LONGER TAXES PROSE. The old form scanned raw file text, so a
  // COMMENT explaining what not to write reddened it; the two directions are
  // both proved in the test below this one.
  it('permits only a closed set of approved family derivations in a font-family position, in every designer source', () => {
    const sources = designerSources()
    expect(sources.length).toBeGreaterThan(10)
    const positions = sources.flatMap(([name, source]) => fontFamilyDeclarations(source).map((value) => `${name}: ${value}`))
    // NON-VACUITY AND THE WHOLE CLAIM IN ONE LIST, EXACT PER FILE AND IN
    // SOURCE ORDER. Two of these five are the canvas fragment's — the ONLY
    // two names that may ever paint a real glyph run, each one of the
    // engine's two attribution identities for the face it measured with. The
    // other three are not about the canvas at all: `FontFamilyProperty`'s own
    // two (Story 16.7) set one `aria-hidden` specimen `<span>` each — a face
    // already on `document.fonts` for the DECLARED-chain row, and a
    // registered preview face for the AVAILABLE LOCALLY row — and
    // `FontBrowser.tsx`'s is the same preview mechanism for its own rows. A
    // CLOSED SET, never a containment: an inline family string escapes an
    // `App.css`-only scan without anyone editing a guard, which is what this
    // list notices. `App.css`'s own literal stack is a stylesheet constant
    // and is guarded separately, above and below.
    expect(positions).toEqual([
      'App.tsx: previewFaceFamily(row.family)',
      'App.tsx: embeddedFaceFamily(declaredEntry.assetKey)',
      'App.tsx: shippedFaceFamily(declaredEntry.face)',
      'App.tsx: embeddedFaceFamily(fragment.assetKey)',
      'App.tsx: shippedFaceFamily(fragment.face)',
      'FontBrowser.tsx: previewFaceFamily(row.family)',
    ])
  })

  it('turns a document-supplied family in a font-family position red, and leaves the prose describing one alone', () => {
    // Every route a document's own vocabulary could reach CSS by, including
    // the one the old form of this guard named.
    expect(unapprovedFontFamilyDeclarations('style={{ fontFamily: chain.entries[0] }}')).not.toEqual([])
    expect(unapprovedFontFamilyDeclarations('style={{ fontFamily: entry.family }}')).not.toEqual([])
    expect(unapprovedFontFamilyDeclarations('style={{ fontFamily: component.fontFamily }}')).not.toEqual([])
    expect(unapprovedFontFamilyDeclarations('node.style.fontFamily = ""; const rule = `font-family: ${chain.name}`')).not.toEqual([])
    expect(unapprovedFontFamilyDeclarations('const css = ".x { font-family: \'IBM Plex Sans\' }"')).not.toEqual([])
    // AND THE ROUTES STORY 8.4e's SECOND APPROVED FORM OPENS, each rejected.
    // A shipped face's identity is the ENGINE's; a chain entry's `face`,
    // `family` or display spelling is the DOCUMENT's, and the near-misses are
    // where the two get confused.
    expect(unapprovedFontFamilyDeclarations('style={{ fontFamily: fragment.face }}')).not.toEqual([])
    expect(unapprovedFontFamilyDeclarations('style={{ fontFamily: shippedFaceFamily(chain.entries[0].face) }}')).not.toEqual([])
    expect(unapprovedFontFamilyDeclarations('style={{ fontFamily: shippedFaceFamily(entry.family) }}')).not.toEqual([])
    expect(unapprovedFontFamilyDeclarations('style={{ fontFamily: shippedFaceFamily(component.fontFamily) }}')).not.toEqual([])
    expect(unapprovedFontFamilyDeclarations("style={{ fontFamily: `'${fragment.face}'` }}")).not.toEqual([])
    // AND THE CROSS-WIRINGS, which are the near-misses a two-form census is
    // most likely to wave through: each seam asked with the OTHER identity,
    // and the shipped seam asked with a chain ENTRY's face rather than the
    // FRAGMENT's — a per-component, chain-derived family wearing the approved
    // call's spelling.
    expect(unapprovedFontFamilyDeclarations('style={{ fontFamily: shippedFaceFamily(fragment.assetKey) }}')).not.toEqual([])
    expect(unapprovedFontFamilyDeclarations('style={{ fontFamily: embeddedFaceFamily(fragment.face) }}')).not.toEqual([])
    expect(unapprovedFontFamilyDeclarations('style={{ fontFamily: shippedFaceFamily(entry.face) }}')).not.toEqual([])
    // AND STORY 16.3'S OWN NEAR-MISSES. The preview derivation is admitted for a
    // BROWSER ROW's family — a name out of the build-time snapshot — and for
    // nothing else. A chain entry's family, a component's `fontFamily` and a
    // fragment's attribution are all the document's or the engine's vocabulary,
    // and none of them may be laundered through this call.
    expect(unapprovedFontFamilyDeclarations('style={{ fontFamily: previewFaceFamily(entry.family) }}')).not.toEqual([])
    expect(unapprovedFontFamilyDeclarations('style={{ fontFamily: previewFaceFamily(component.fontFamily) }}')).not.toEqual([])
    expect(unapprovedFontFamilyDeclarations('style={{ fontFamily: previewFaceFamily(fragment.assetKey) }}')).not.toEqual([])
    expect(unapprovedFontFamilyDeclarations('style={{ fontFamily: embeddedFaceFamily(row.family) }}')).not.toEqual([])

    // THE APPROVED ONES, and only in their derived forms — two, and no more.
    expect(unapprovedFontFamilyDeclarations('style={{ fontFamily: embeddedFaceFamily(fragment.assetKey) }}')).toEqual([])
    expect(unapprovedFontFamilyDeclarations('style={{ fontFamily: shippedFaceFamily(fragment.face) }}')).toEqual([])
    expect(unapprovedFontFamilyDeclarations('style={{ fontFamily: previewFaceFamily(row.family) }}')).toEqual([])
    // THE NEGATIVE CASES. A scan that only ever reddens has not been shown to
    // discriminate. Prose describing the forbidden route is not the route, and
    // the projection's `fontFamily` FIELD — a document's chain name — is read
    // all over this codebase without ever reaching a CSS declaration.
    expect(unapprovedFontFamilyDeclarations('// never write fontFamily: chain.entries[0] here')).toEqual([])
    expect(unapprovedFontFamilyDeclarations('// never write fontFamily: fragment.face here — it goes through the seam')).toEqual([])
    expect(unapprovedFontFamilyDeclarations('/* fontFamily: entry.family would collide with a shipped face */')).toEqual([])
    expect(unapprovedFontFamilyDeclarations('const values = components.map((c) => committedValue(c, \'fontFamily\'))')).toEqual([])
    expect(unapprovedFontFamilyDeclarations('type Component = Readonly<{ fontFamily?: string; fontSize?: number }>')).toEqual([])
    expect(unapprovedFontFamilyDeclarations('void commit({ field: \'fontFamily\', operation: \'set\', value: name })')).toEqual([])
  })

  // The generic keyword is a last resort and must stay last. If it moved
  // ahead of a declared family the browser would never reach the real
  // face, reproducing the same defect with the stack looking correct.
  it('keeps the generic fallback last', () => {
    const rule = css.split('\n').find((line) => line.startsWith('.canvas-text-fragment {')) ?? ''
    const declaration = /font-family:([^;]+);/.exec(rule)?.[1] ?? ''
    const entries = declaration.split(',').map((entry) => entry.trim())
    expect(entries[entries.length - 1]).toBe('sans-serif')
    expect(entries.slice(0, -1).every((entry) => entry.startsWith("'"))).toBe(true)
  })
})
