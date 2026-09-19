import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const sourceDir = path.dirname(fileURLToPath(import.meta.url))
const designerRoot = path.dirname(sourceDir)
const production = fs.readdirSync(sourceDir, { recursive: true })
  .filter((entry): entry is string => typeof entry === 'string' && /\.(?:ts|tsx|css)$/.test(entry) && !/\.test\.(?:ts|tsx)$/.test(entry))
  .map((entry) => path.join(sourceDir, entry))
const e2e = fs.readdirSync(path.join(designerRoot, 'e2e'), { recursive: true })
  .filter((entry): entry is string => typeof entry === 'string' && /\.(?:ts|tsx)$/.test(entry))
  .map((entry) => path.join(designerRoot, 'e2e', entry))
const tests = fs.readdirSync(sourceDir, { recursive: true })
  .filter((entry): entry is string => typeof entry === 'string' && /\.test\.(?:ts|tsx)$/.test(entry) && entry !== 'canvas-authority-contract.test.ts')
  .map((entry) => path.join(sourceDir, entry))

const prohibited = [
  /(?:CanvasRenderingContext2D|\bctx)\.measureText/,
  /\b(?:getBoundingClientRect|getClientRects)\s*\(/,
  /\b(?:offset(?:Width|Height|Left|Top|Parent)|client(?:Width|Height|Left|Top)|scroll(?:Width|Height|Left|Top))\b/,
  /\boffset[XY]\b/,
  /\bResizeObserver\b/,
  // STORY 8.4a REPAIRED THIS RULE, WHICH HAD BEEN DEAD SINCE 7bfb076.
  // `violations()` used to rewrite EVERY `document.fonts` prefix in every file
  // to a throwaway token before applying any pattern, so this line could not
  // match anything at all: `document.fonts.add(face)` read as
  // `fontReadinessOnly.add(face)`. Three things confirmed it was dead rather
  // than merely broad — the token appeared nowhere else in the repository, the
  // mutation block below proved eleven other prohibitions and not this one, and
  // the rewrite had been appended as a drive-by unblock in a story about sample
  // data. The rewrite is now scoped to `document.fonts.ready` ALONE (readiness
  // is not measurement, and e2e/engine-worker.spec.ts legitimately awaits it),
  // so every other member — `add`, `delete`, `clear`, `load`, `check`, and
  // iteration — is caught here, and the proof that it is caught is in this
  // file rather than assumed.
  /\bdocument\.fonts\b/,
  // The other half of the same door, which was never guarded at all: a face can
  // be registered without touching `document.fonts` by name. Story 8.4a's seam
  // is carved out below, by file name and only while it still spells the
  // function that earns the exception.
  /\bnew FontFace\b/,
  /\bdevicePixelRatio\b/,
  /\b(?:Range|document\.createRange|getSelection|Selection)\s*\(/,
  /\bgetComputedStyle\s*\(/,
  /(?:white-space|text-wrap|overflow-wrap|word-break|line-clamp|text-align)\s*:\s*(?:normal|wrap|balance|pretty|anywhere|break-word|justify|\d+)/,
  // D-7.4.2 §5. The canvas paint is APPROXIMATE and, since Story 7.4, may be
  // a deliberately truncated prefix; pagination is EXACT and comes from
  // layout.Paginate. Deriving a height, a page count or a window count from
  // the paint's line count would make a truncated paint shorten the reported
  // column, and Story 7.6 would then draw the wrong number of sheets — the
  // canvas lying about pagination. The independence holds in Go today by
  // construction (nothing there reads CanvasTextPaint), but the data sits
  // right here in `line.advance`, one plausible line of designer code away.
  // A positive test would not catch that line being written; this does.
  /\b(?:textPaint|paint)\??\.lines\.length\b/,
  /\blines\.length\s*[*/]|[*/]\s*\blines\.length\b/,
  // Story 7.6 / AC2, and the same guard one step further along. A window's
  // COUNT may not come from the paint; a window's POSITION may not come from
  // arithmetic at all. The window height multiplied by an index is the closed form
  // internal/layout/paginate.go forbids by name — the window advances to the
  // top of the first item that did not fit, never by a fixed height — and it
  // is measurably wrong: 110 millipoints per window adrift on a column of
  // round 728pt spacing, and eleven windows where the engine says two on a
  // column with a declared gap. The origins are PROJECTED
  // (contentWindowOrigins); multiplying is the one plausible line of designer
  // code that would quietly replace them, in either operand order.
  /\b(?:contentWindowHeight|windowHeight)\s*\*|\*\s*[\w.?]*\b(?:contentWindowHeight|windowHeight)\b/,
  // STORY 11.3 / AC2, AND THE SHARP END OF I-2: NO SYNTHETIC BOLD OR OBLIQUE
  // ON PAINTED DOCUMENT TEXT. The engine resolves a real cut and names it on
  // the fragment; a `font-weight` or `font-style` applied to the painted text
  // asks the browser to smear a face it already drew correctly, which is the
  // one thing the emit path has always refused. Until this story the canvas did
  // exactly that, from `component.bold` — THE REQUESTED FLAG — and after 11.2
  // it did it ON TOP OF the real bold cut.
  //
  // ⚠ SCOPED TO THE SURFACE, NOT TO THE VALUE, and that is the whole design of
  // these two rules. The sibling CSS rule above (`white-space: normal`, …)
  // scopes by VALUE because its forbidden values appear nowhere legitimate.
  // These do: the chrome writes `font-weight: 500` in seven rules and `600` in
  // two, and `.property-fx` is deliberately italic. The invariant is not "this
  // value is wrong" but "this property may not be applied to painted document
  // text" — the page/chrome line DESIGN.md already draws — so the scope is the
  // `.canvas-text-*` surface and the JSX elements that carry it.
  //
  // ⚠ AND IT REACHES TWO DIFFERENT SPELLINGS, BECAUSE THE DEFECT WAS WRITTEN IN
  // BOTH AND A VALUE PATTERN REACHED NEITHER. `App.css` read
  // `font-weight: var(--text-font-weight)` and `App.tsx` wrote
  // `'--text-font-weight': component.bold ? 700 : 400`; a
  // `font-weight:\s*(bold|\d+)` pattern matches neither of the two lines this
  // story deleted, and would have shipped green over the defect it exists for.
  // The two rules are separate entries so each has its own red proof.
  //
  // ⚠ NEITHER IS A BAN ON THE CUSTOM PROPERTY'S NAME. A rename defeats a name
  // ban, and the pair is what closes that: the custom property can only BECOME
  // a weight through a CSS declaration on the painted surface, which the first
  // rule catches whatever the value is called.
  //
  // RULE 15 — the painted-document CSS surface, LONGHAND. Any `.canvas-text*`
  // rule whose body declares either property, by any value: a literal, a
  // `var()`, or an inherited custom property.
  //
  // ⚠ THE SELECTOR PART IS `[^{}]*`, NOT `[^\n{}]*`. It was the latter, and a
  // grouped selector broken across lines — `.canvas-text-paint,\n.other { … }`
  // — walked straight through it. In CSS the text between the previous rule's
  // `}` and this rule's `{` IS the selector list, so excluding braces is the
  // correct bound and excluding newlines was an accident of this file happening
  // to be written one rule per line.
  /\.canvas-text[^{}]*\{[^}]*\bfont-(?:weight|style)\s*:/,
  // RULE 16 — the same surface, SHORTHAND, which the longhand rule cannot see.
  // `font: bold 12px/1 sans-serif` sets `font-weight` without the word
  // `font-weight` appearing anywhere, and this is not hypothetical: the
  // `.property-toggle-unavailable` rule THIS STORY ADDED uses the `font`
  // shorthand precisely because it resets a weight.
  //
  // SCOPED TO THE VALUE HERE, and that is not a contradiction of the rule above.
  // The shorthand's other job — `font: var(--type-band-tab)` on
  // `.canvas-text-truncated` — is a legitimate type token on a chrome strip and
  // is in the file today; banning the shorthand outright would red it. What is
  // forbidden is a weight or a slope EXPRESSED in the shorthand, so the value is
  // scanned for exactly those: the four keywords, and a `[1-9]00` weight (a
  // length is `12px`, never a bare `700`). The value scan stops at `;` or `}`,
  // so a keyword elsewhere in the block cannot answer for it.
  /\.canvas-text[^{}]*\{[^}]*\bfont\s*:[^;}]*(?:\b(?:bold(?:er)?|lighter|italic|oblique)\b|\b[1-9]00\b)/,
  // RULE 17 — the same surface in JSX: the inline style object on an element
  // whose className names one of the canvas text spans. Any property whose name
  // carries a weight or a slope — `fontWeight`, `font-style`, or a custom
  // property spelling either — is the same prohibition arriving by the other
  // door. `fontFamily` and `--text-font-size` are deliberately untouched: the
  // family IS how the resolved cut reaches the browser, and a size is not a
  // weight.
  //
  // ⚠ BOUNDED BY THE OPENING TAG (`[^>]*`), NOT BY THE FIRST `}`. It was
  // `[^}]*`, which stopped at the first closing brace inside the style object —
  // and EVERY canvas-text style object in App.tsx already contains a
  // conditional-spread `{}` (`...(component.color === undefined ? {} : {…})`),
  // so a weight written AFTER one was invisible in the very file this rule
  // guards. A JSX inline style lives inside one opening tag, so the tag is the
  // honest bound.
  //
  // ⚠ AND THE className IS NOT REQUIRED TO BE A STRING LITERAL. It was
  // `className="canvas-text…"`, which a template literal
  // (`` className={`canvas-text-paint ${x}`} ``) defeated. Matching `className=`
  // and then the class name anywhere in the same tag covers the literal, the
  // template literal, a ternary and a helper call alike.
  /className=[^>]*canvas-text[^>]*style=\{\{[^>]*font-?(?:weight|style)/i,
]

// ===========================================================================
// THE AD-17 DISPLAY-PAINT TEST — THREE CONDITIONS, ALL REQUIRED.
// Ruled by the engineering lead at Story 14.9's plan gate, 2026-09-10, and
// written HERE rather than only in that story's spec: a spec is read once, and
// this file's comments are the one place in this repository demonstrated to be
// read years later.
// ===========================================================================
//
// THE QUESTION IT SETTLES. Story 14.9 draws a table on the canvas — the bound
// collection, the real header labels, one row of binding placeholders — and is
// the first thing in the designer to paint a string with the browser that ALSO
// APPEARS IN THE PDF. Is that allowed under AD-17?
//
// THE AXIS THAT WAS REFUSED, VERBATIM, because it is the wrong rule and would
// be reached for again:
//
//   "That axis is a coincidence, not the rule, and the spec must not state it
//   as one. The reason every string the canvas paints today has been safe was
//   never that it does not print — it is that NOTHING DEPENDS ON ITS MEASURED
//   EXTENT. Were the rule 'chrome only', it would forbid something harmless
//   and permit something dangerous the moment someone painted a chrome string
//   whose width drove a layout decision."
//
// THE TEST. Text may be painted by the browser on the canvas only where:
//
//   1. THE RECTANGLE IS THE ENGINE'S. Every coordinate and extent bounding the
//      painted text comes from the engine's projection. No browser measurement
//      contributes to the box.
//   2. THE BROWSER MAKES NO BREAK DECISION. Wrapping disabled
//      (`white-space: pre`/`nowrap`); overflow clipped or ellipsised BY CSS,
//      never re-flowed.
//   3. NOTHING FLOWS BACK. No quantity derived from the painted text — width,
//      height, line count, overflow state, scroll extent — reaches geometry, a
//      page count, a band-fit decision, a command, or a projection field.
//
//   "If all three hold the paint is display-only and AD-17 permits it whether
//   or not the string also prints. If any one fails, the text must arrive as
//   pre-measured, pre-broken runs from the measure API."
//
// WHY THIS IS A SCOPED PERMISSION AND NOT A WAIVER — CONDITION 3 IS ALREADY
// ENFORCED, BY THIS FILE:
//
//   "canvas-authority-contract.test.ts scans src/ and e2e/ for every route by
//   which a browser-measured quantity could be OBTAINED AT ALL —
//   getBoundingClientRect, offset*, client*, scroll*, ResizeObserver,
//   getComputedStyle, Range. A VALUE THAT CANNOT BE OBTAINED CANNOT FLOW BACK."
//
// That is the `prohibited` list above, applied to production `src/**`, the unit
// tests and `e2e/**` alike — `page.evaluate` bodies included, because the scan
// is plain regex over whole file text.
//
// THE MARKER. Every element painting text under this exception carries the
// class `.canvas-display-paint`, and that class is ALSO the mechanism for
// condition 2: its rule in App.css is `min-width: 0; overflow: hidden;
// white-space: pre; text-overflow: ellipsis`. It is named for the PROPERTY —
// display-only paint — and NOT for the feature that first needed it, so that a
// later author grepping the class lands on this rule and the next candidate for
// the exception recognises its own case. `.canvas-table-header-label` would
// have said nothing about the rule it lives under.
//
// NO COMPUTED ELLIPSIS. CSS `text-overflow` is condition-2 compliant; a
// JavaScript truncation that measures the string or its box is not, and there
// is none anywhere in this feature.
//
// THE PERMITTED RESIDUE is exactly what epics.md's Story 5.13 AC already
// scopes: A LABEL MAY CLIP WHERE THE PDF WRAPS. That is the allowed text-only
// inaccuracy, and never a geometry error. The canvas stays "explicitly
// approximate about text only".
//
// ⚠ THE PRECEDENT IS BOUNDED, AND THIS PARAGRAPH IS THE BOUND. This ruling
// carries a later paint ONLY where all three conditions hold the same way they
// hold here. Two cases are explicitly NOT carried and each needs a NEW RULING:
//
//   (a) any use that fails condition 1 or condition 2 MECHANICALLY — a box the
//       browser sized from content, or text the browser was allowed to wrap or
//       re-flow; and
//   (b) any use where condition 3 is ASSERTED rather than covered by this
//       scan — if the measured quantity can be obtained at all, "we do not use
//       it" is a promise, not an enforcement, and this ruling does not reach it.
//
// ⚠ AND IT SAYS NOTHING ABOUT THE CHROME NOTICES THAT PREDATE IT.
// `.canvas-image-placeholder`, `.canvas-text-truncated` and the no-columns
// notice are fixed English sentences in a placeholder frame; they wrap, they
// carry no engine string, and they claim no exception. They were safe before
// this ruling for condition 3's reason alone — nothing depends on their
// measured extent — which is precisely the reasoning the refused axis above
// mistook for "chrome never prints".

// STORY 8.2. THE SECOND LOCK ON "THE BROWSER HOLDS NO ENGINE RULE".
//
// A chain edit is refused by the engine in the engine's own sentence, and the
// panel's only job is to place it. The failure mode that closes off is a
// TypeScript COPY of one of those rules — a duplicate-name check, an
// empty-chain check, an orphan check, a message table — which would pass every
// behavioural test the panel has, because "the error shows" is satisfied
// equally well by a local copy as by the engine's answer. App.test.tsx's
// anti-pre-emption assertions prove a command is still DISPATCHED for each of
// those cases; this scan proves the SENTENCES are not here.
//
// Each literal is a fragment of a real refusal in
// folio-go/component_commands.go: `a font chain named %q already exists`,
// `font chain %q is still named by ...`, `removing that entry would leave font
// chain %q with no entries`, `a font chain must declare at least one entry`,
// `entry index is out of range`, `font chain name exceeds the projection
// bound`, `no font chain named %q is declared`.
//
// SCOPE: PRODUCTION SOURCE ONLY, and deliberately so. A test that asserts the
// rendered refusal is `===` to the engine's `message` must be able to WRITE
// that message as a fixture — that is the evidence, not a copy of the rule —
// so the unit-test and e2e corpora are not scanned for these. Production is
// where a rule would have to live to reach an author.
//
// AND COMMENTS ARE STRIPPED FIRST. These sentences are ordinary English, and
// scanning raw file text made the guard collide with prose that has nothing to
// do with an engine rule: `createComponent` ALREADY EXISTS on the channel, the
// sheet gap IS DECLARED here as a number. Two unrelated comments were reworded
// to get the first version of this scan green, which is the guard editing the
// codebase rather than the codebase answering to the guard — and it would have
// happened again on every future `is declared` or `is out of range`. A COPY of
// an engine rule has to be a string or a template literal to reach an author,
// so those are the only places worth looking, and the two comments above have
// been restored to their original wording as this fix's own proof.
const refusalVocabulary = [
  /already exists/,
  /is still named by/,
  /with no entries/,
  /must declare at least one entry/,
  /is out of range/,
  /exceeds the projection bound/,
  /is declared/,
  // The rest of component_commands.go's font-chain vocabulary. These were
  // omitted while the scan read raw text because several of them are also
  // plausible English; with comments stripped they cost nothing to add, and
  // leaving them out left five refusals a browser copy could have used.
  /font chain entries are required/,
  /must be a non-empty string/,
  /declares more entries than the projection bound/,
  /declares more font chains than the projection bound/,
  // STORY 11.4 — ROUTE C GAVE THE ENTRY DECODER SEVEN NEW SENTENCES, and every
  // one of them is a rule about what a chain entry may be. They are exactly the
  // shape a browser-side copy takes: a pick builder that "helpfully" validated
  // its own entries before sending them would reproduce these one for one, pass
  // every behavioural test the control has, and quietly become a second
  // authority over the format — the failure this whole scan exists for.
  //
  // ⚠ `/must be a string array/` LEFT THIS LIST WITH THE SENTENCE IT NAMED.
  // `entries` and `tail` are no longer string arrays, so the engine no longer
  // says that; the first row below is the sentence that replaced it. Keeping
  // the retired one would have been a pattern that can never fire, which reads
  // as coverage and is not.
  /must be an array of font chain entries/,
  /is not a key a font chain entry may carry/,
  /never an assets key/,
  /must name the face it is/,
  /is present and null/,
  /must be a string naming a face/,
  /write no key at all rather than an empty string/,
  /names this entry's OWN base face/,
]

// withoutComments removes line and block comments while leaving string and
// template literals intact. It is a character scanner rather than a regex
// because a regex cannot tell `// a comment` from the `//` inside a URL string
// — and getting that backwards would make the guard vacuous in exactly the
// place it matters. Quotes are checked before comment openers, so an
// apostrophe inside a comment never opens a string and a `//` inside a string
// never opens a comment.
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

function refusalViolations(files: readonly string[]): string[] {
  return files.flatMap((file) => {
    const source = withoutComments(fs.readFileSync(file, 'utf8'))
    const name = path.relative(designerRoot, file)
    return refusalVocabulary.filter((pattern) => pattern.test(source)).map((pattern) => `${name}: ${pattern}`)
  })
}

// scanned is what every prohibition is actually applied to: the file's own
// text with the approved, individually justified exceptions removed and its
// COMMENTS STRIPPED. Comment stripping arrived with Story 8.4a and for the
// same reason it was already applied to the refusal vocabulary — these
// patterns are describable in English, and a guard that reddens on the prose
// explaining what not to write ends up editing the codebase instead of
// answering to it. `does not catch ordinary English in a comment` proves both
// directions.
//
// The pointer-input exception runs FIRST, on raw text, because its seam was
// written against the file as it stands and it asserts that seam is still
// there.
//
// STORY 17.6 adds the painted-border readback exception LAST. It is a pure
// additional transform: identity for every file but one, and inside that one
// it rewrites a single spelling inside a single matched region. Nothing about
// how the other prohibitions compose changes — in particular the three
// ARITHMETIC rules at the end of `prohibited`
// (`textPaint?.lines.length`, `lines.length` in a `*` or `/`, and
// `contentWindowHeight`/`windowHeight` in a `*`) still apply everywhere,
// including inside the exempted block, and
// `keeps every other prohibition live inside the exempt block` is the proof.
function scanned(file: string, source: string): string {
  return withoutApprovedPaintedBorderReadback(file, withoutApprovedRuntimeFaceRegistration(file, withoutComments(withoutApprovedLocalPointerInput(file, source))))
}

function violations(files: readonly string[]): string[] {
  return files.flatMap((file) => {
    const source = scanned(file, fs.readFileSync(file, 'utf8'))
    const name = path.relative(designerRoot, file)
    return prohibited.filter((pattern) => pattern.test(source)).map((pattern) => `${name}: ${pattern}`)
  })
}

describe('canvas projection authority contract', () => {
  it('scans a non-vacuous production, unit-test, and e2e corpus for browser measurement authority', () => {
    expect(production.length).toBeGreaterThan(10)
    expect(tests.length).toBeGreaterThan(10)
    expect(e2e.length).toBeGreaterThan(3)
    expect(violations([...production, ...tests, ...e2e])).toEqual([])
  })

  it('keeps the engine\'s own refusal vocabulary out of every production source file', () => {
    expect(production.length).toBeGreaterThan(10)
    expect(refusalViolations(production)).toEqual([])
  })

  it('turns a TypeScript copy of any engine refusal red', () => {
    // One realistic line per literal — the shape a browser-side rule would
    // actually take — so the scan is proved by its own reds, not assumed.
    expect(refusalForSource('if (chains.some((chain) => chain.name === name)) return `a font chain named "${name}" ' + 'already exists`')).not.toEqual([])
    expect(refusalForSource('const orphan = `font chain "${name}" ' + 'is still named by ${ids.join(", ")}`')).not.toEqual([])
    expect(refusalForSource('const empty = `removing that entry would leave font chain "${name}" ' + 'with no entries`')).not.toEqual([])
    expect(refusalForSource('const none = "a font chain ' + 'must declare at least one entry"')).not.toEqual([])
    expect(refusalForSource('const range = "entry index ' + 'is out of range"')).not.toEqual([])
    expect(refusalForSource('const bound = "font chain name ' + 'exceeds the projection bound"')).not.toEqual([])
    expect(refusalForSource('const missing = `no font chain named "${name}" ' + 'is declared`')).not.toEqual([])
    expect(refusalForSource('const required = "font chain ' + 'entries are required"')).not.toEqual([])
    expect(refusalForSource('const shape = "font chain entries ' + 'must be an array of font chain entries"')).not.toEqual([])
    expect(refusalForSource('const key = `"${key}" ' + 'is not a key a font chain entry may carry"`')).not.toEqual([])
    expect(refusalForSource('const asset = "a font chain entry written by a command names a FACE, ' + 'never an assets key"')).not.toEqual([])
    expect(refusalForSource('const none = "a font chain entry object ' + 'must name the face it is"')).not.toEqual([])
    expect(refusalForSource('const nulled = `"bold" ' + 'is present and null"`')).not.toEqual([])
    expect(refusalForSource('const typed = `"bold" ' + 'must be a string naming a face"`')).not.toEqual([])
    expect(refusalForSource('const empty = "for a cut this family does not have, ' + 'write no key at all rather than an empty string"')).not.toEqual([])
    expect(refusalForSource('const self = `"bold" ' + 'names this entry\'s OWN base face"`')).not.toEqual([])
    expect(refusalForSource('const face = "a font chain entry ' + 'must be a non-empty string"')).not.toEqual([])
    expect(refusalForSource('const many = "a font chain ' + 'declares more entries than the projection bound"')).not.toEqual([])
    expect(refusalForSource('const chains = "document ' + 'declares more font chains than the projection bound"')).not.toEqual([])
  })

  // THE OTHER HALF, and the one that keeps the guard from editing the prose it
  // is scanning. These are the two REAL comments in this repository that the
  // raw-text version of this scan reddened, restored verbatim.
  it('does not catch ordinary English in a comment', () => {
    expect(refusalForSource('// be created there rather than refused. `createComponent` already exists on\n// the channel, already carries the band NAME')).toEqual([])
    expect(refusalForSource('// between two sheets is declared HERE as a number and written out as a custom')).toEqual([])
    expect(refusalForSource('/* the index is out of range for this band, so nothing is drawn */')).toEqual([])
    // But a string on the same line as a comment is still read.
    expect(refusalForSource('const message = "already exists" // a comment')).not.toEqual([])
    // And a comment marker inside a STRING does not hide what follows it.
    expect(refusalForSource('const url = "https://example.test/x"; const message = "is out of range"')).not.toEqual([])
  })

  it('allows only the non-document reduced-motion media rule', () => {
    const css = fs.readFileSync(path.join(sourceDir, 'App.css'), 'utf8')
    expect([...css.matchAll(/@media\s*\(([^)]+)\)/g)].map((match) => match[1])).toEqual(['prefers-reduced-motion: reduce'])
  })

  it('turns realistic measurement, CSS, range, and event-coordinate mutations red', () => {
    expect(violationsForSource(`const width = ${['CanvasRenderingContext2D', 'measureText'].join('.')}("x")`)).not.toEqual([])
    expect(violationsForSource('.paint { white-space: normal }')).not.toEqual([])
    expect(violationsForSource(`const x = event.${['offset', 'X'].join('')}`)).not.toEqual([])
    expect(violationsForSource('const style = getComputedStyle(node)')).not.toEqual([])
    expect(violationsForSource(`const range = ${['document', 'createRange'].join('.') }()`)).not.toEqual([])
    expect(violationsForSource('.paint { text-align: justify }')).not.toEqual([])
    expect(violationsForSource('const height = component.textPaint.lines.length * line.advance')).not.toEqual([])
    expect(violationsForSource('const windows = Math.ceil(paint.lines.length / perPage)')).not.toEqual([])
    expect(violationsForSource('const top = canvas.contentWindowHeight * index')).not.toEqual([])
    expect(violationsForSource('const top = index * canvas.contentWindowHeight')).not.toEqual([])
    expect(violationsForSource('const top = sheet * windowHeight')).not.toEqual([])
    // STORY 11.3 / AC2 — THE TWO SPELLINGS, EACH ITS OWN ROW. The first is the
    // line App.css carried until this story, `var()` value and all; the second
    // is the line App.tsx carried, where the property is a CUSTOM property and
    // the word `font-weight` never appears in a `property: value` position at
    // all. A value-shaped pattern reddens neither.
    expect(violationsForSource('.canvas-text-paint { font-size: var(--text-font-size); font-weight: var(--text-font-weight); }')).not.toEqual([])
    expect(violationsForSource('<span className="canvas-text-paint" style={{ \'--text-font-weight\': component.bold ? 700 : 400 }} />')).not.toEqual([])
    // AND THE FOUR EVASIONS REVIEW MEASURED AS GREEN AGAINST THE FIRST FORM OF
    // THESE RULES. Each is a real way the deleted defect comes back, each was
    // silent, and each has its own row so a later retune cannot quietly reopen
    // one of them.
    //
    // EACH ROW NAMES THE RULE IT MUST WAKE, the way the Story 17.6 block below
    // does: `.not.toEqual([])` cannot tell three live rules from two, and these
    // three overlap enough that one could cover for another's deletion.
    const [longhand, shorthand, inlineStyle] = [prohibited[14], prohibited[15], prohibited[16]]
    for (const [line, rule] of [
      // (i) the `font` SHORTHAND, which carries a weight with no `font-weight`
      // anywhere in it. Only the shorthand rule can answer this.
      ['.canvas-text-paint { font: bold 12px/1 sans-serif }', shorthand],
      // (ii) a GROUPED SELECTOR broken across lines, which a newline-excluding
      // selector bound walked straight through.
      ['.canvas-text-paint,\n.other { font-weight: 700 }', longhand],
      // (iii) a weight written AFTER a nested `{}` in the style object — and
      // every canvas-text style object in App.tsx already contains a
      // conditional spread, so this was the most natural reintroduction point
      // in the very file the rule guards.
      ['<span className="canvas-text-paint" style={{ ...(a ? {} : { fontFamily: f }), fontWeight: 700 }} />', inlineStyle],
      // (iv) a TEMPLATE-LITERAL className, which a string-literal pattern missed.
      ['<span className={`canvas-text-paint ${x}`} style={{ fontWeight: 700 }} />', inlineStyle],
      // And the original two spellings, pinned to their rules for the same reason.
      ['.canvas-text-paint { font-weight: var(--text-font-weight); }', longhand],
      ['<span className="canvas-text-paint" style={{ \'--text-font-weight\': component.bold ? 700 : 400 }} />', inlineStyle],
    ] as const) {
      expect(violationsForSource(line).map(String), line).toContain(String(rule))
    }
  })

  // STORY 11.3 / AC2. THE PROHIBITION IN BOTH DIRECTIONS, because a rule that
  // reddens the chrome is not scoped and a rule that misses the mutation is not
  // a prohibition. The reds are the two lines this story deleted plus the
  // plausible ways they could come back; the greens are every legitimate use of
  // the same two properties that is already in this repository.
  it('turns a synthetic weight or slope on painted document text red, and leaves the chrome alone', () => {
    // REDS — the painted surface, in CSS, by every value spelling.
    for (const line of [
      '.canvas-text-paint { font-weight: var(--text-font-weight); }',
      '.canvas-text-paint { font-style: var(--text-font-style); }',
      '.canvas-text-paint { font-weight: 700 }',
      '.canvas-text-paint { font-weight: bold }',
      '.canvas-text-fragment { font-style: italic }',
      '.canvas-text-line { font-weight:700 }',
      // The rule reached through a descendant or a grouped selector, which is
      // the same surface written a different way.
      '.canvas-component .canvas-text-fragment span { font-weight: 600 }',
      '.some-chrome, .canvas-text-paint { font-style: oblique }',
      // The shorthand, in every spelling that carries a weight or a slope.
      '.canvas-text-paint { font: bold 12px/1 sans-serif }',
      '.canvas-text-paint { font: italic 12px/1 sans-serif }',
      '.canvas-text-paint { font: 700 12px/1 sans-serif }',
      '.canvas-text-fragment { font: oblique 700 12px/1 sans-serif }',
      // The selector list broken across lines, in both orders.
      '.canvas-text-paint,\n.other-chrome { font-weight: 700 }',
      '.other-chrome,\n.canvas-text-paint { font: bolder 12px sans-serif }',
    ]) {
      expect(violationsForSource(line), line).not.toEqual([])
    }
    // REDS — the painted surface, in JSX, by custom property and by the React
    // property name alike.
    for (const line of [
      '<span className="canvas-text-paint" style={{ \'--text-font-weight\': component.bold ? 700 : 400 }} />',
      '<span className="canvas-text-paint" aria-hidden="true" style={{ \'--text-font-style\': component.italic ? \'italic\' : \'normal\' }} />',
      '<span className="canvas-text-fragment" style={{ fontWeight: 700 }} />',
      '<span className="canvas-text-line" style={{ fontStyle: \'italic\' }} />',
      // AFTER a nested `{}` — the shape the real paint span already has.
      '<span className="canvas-text-paint" style={{ ...(component.color === undefined ? {} : { \'--text-ink\': component.color }), fontWeight: 700 }} />',
      '<span className="canvas-text-paint" aria-hidden="true" style={{ ...(a ? {} : { b: 1 }), \'--text-font-style\': component.italic ? \'italic\' : \'normal\' }} />',
      // A className that is not a string literal.
      '<span className={`canvas-text-paint ${zoomClass}`} style={{ fontWeight: 700 }} />',
      '<span className={clsx(\'canvas-text-fragment\', extra)} style={{ fontStyle: \'italic\' }} />',
      // A RENAMED CUSTOM PROPERTY DEFEATS THE JSX RULE AND NOT THE PAIR: the
      // value can only become a weight through a CSS declaration on the same
      // surface, and rule 15 is standing there.
      '.canvas-text-paint { font-weight: var(--tfw) }',
    ]) {
      expect(violationsForSource(line), line).not.toEqual([])
    }
    // GREENS — the repository's own legitimate uses, quoted verbatim from the
    // files they live in. These are the nine chrome rules a value-scoped ban
    // would have reddened, the pasted-HTML fixtures that are INPUT DATA rather
    // than a rule, and the neighbouring test that names both properties while
    // asserting their ABSENCE from an @font-face descriptor.
    for (const line of [
      '.mode-switch .mode-active { background: var(--color-active); color: var(--color-ink-high); font-weight: 500; }',
      '.panel-tab-active { border-bottom-color: var(--color-select); color: var(--color-ink-high); font-weight: 500; }',
      '.component-identity-name { color: var(--color-ink-high); font: var(--type-body-em); font-weight: 500; text-transform: capitalize; }',
      '.property-fx { flex: none; padding: 0 3px; color: var(--color-ink-ghost); font: var(--type-mono); font-style: italic; }',
      '.property-toggle[aria-pressed="true"] { background: var(--color-active); font-weight: 600; }',
      '.property-segment[aria-pressed="true"] { background: var(--color-active); font-weight: 500; }',
      '.property-add-fonts-label { color: var(--color-select-bright); font-weight: 500; }',
      '.font-browser-card .font-browser-family { font: var(--type-body-em); font-weight: 500; }',
      '.font-browser-confirm { padding: var(--space-2) var(--space-6); font-weight: 500; }',
      '\'text/html\': \'<p style="font-weight:700;font-family:Georgia">Clause 1.</p>\'',
      'expect(rule).not.toContain(\'font-weight\')',
      'expect(rule).not.toContain(\'font-style\')',
      // The painted surface's family and size, which are the engine's own
      // answers and must never be caught by a rule aimed at weight.
      '.canvas-text-fragment { left: var(--text-fragment-x); top: 0; font-family: \'Noto Sans\', sans-serif; }',
      '.canvas-text-paint { font-size: var(--text-font-size); }',
      '<span className="canvas-text-fragment" style={{ fontFamily: shippedFaceFamily(fragment.face) }} />',
      // THE SHORTHAND CARRYING A TYPE TOKEN, which is in this file today on
      // `.canvas-text-truncated` — a chrome strip, not painted document text —
      // and which the shorthand rule must not red. This is the green that keeps
      // rule 16 scoped to a weight rather than to the property.
      '.canvas-text-truncated { padding: 2px var(--space-2); font: var(--type-band-tab); letter-spacing: var(--tracking-band-tab); }',
      // The story's own third-state rule, which uses the shorthand to reset a
      // weight — legitimately, because it is CHROME and names no canvas-text
      // surface at all.
      '.property-toggle-unavailable, .property-toggle-unavailable[aria-pressed="true"] { border-style: dashed; font: var(--type-body-em); }',
    ]) {
      expect(violationsForSource(line), line).toEqual([])
    }
    // AND THE REAL FILES, WHICH IS THE CLAIM THAT ACTUALLY MATTERS: the four
    // sources that carry every green above stay green as they stand on disk.
    for (const name of ['App.css', 'App.tsx', 'App.test.tsx', 'font-catalogue.test.ts']) {
      const file = [...production, ...tests].filter((candidate) => path.basename(candidate) === name)
      expect(file, name).toHaveLength(1)
      expect(violations(file), name).toEqual([])
    }
  })

  // STORY 8.4a. THE MUTATION PROOFS THE `document.fonts` RULE NEVER HAD —
  // which is the defect being repaired, not a decoration on it. Before this,
  // deleting that rule outright left every assertion in this file green, so
  // the rule was indistinguishable from its own absence.
  it('turns runtime font registration red, allows readiness, and allows it only in the one approved seam', () => {
    expect(violationsForSource('document.fonts.add(face)')).not.toEqual([])
    expect(violationsForSource('document.fonts.delete(face)')).not.toEqual([])
    expect(violationsForSource('await document.fonts.load("12px x")')).not.toEqual([])
    expect(violationsForSource('if (document.fonts.check("12px x")) paint()')).not.toEqual([])
    expect(violationsForSource('const face = new FontFace(family, bytes)')).not.toEqual([])
    // READINESS STAYS LEGAL, in the one place that actually uses it and
    // anywhere else: it registers nothing and measures nothing.
    expect(violationsForFile('e2e/engine-worker.spec.ts', 'await document.fonts.ready')).toEqual([])
    // AND THE PROSE TAX IS GONE. A comment that names the mechanism is not the
    // mechanism, in either file.
    expect(violationsForSource('// the seam calls document.fonts.add once per carried face')).toEqual([])
    expect(violationsForSource('/* never write new FontFace outside that module */')).toEqual([])
  })

  // The seam exception is scoped to a file AND to the function inside it. Both
  // halves are proved: the same line is legal there and illegal everywhere
  // else, and it stops being legal there the moment the function it is
  // attached to is gone.
  it('scopes the registration exception to the named seam and to the function that earns it', () => {
    const seam = 'src/embedded-face-registry.ts'
    const registration = 'export function registerCarriedFaces(keys) {\n  const face = new FontFace(family, bytes)\n  document.fonts.add(face)\n}\n'
    expect(violationsForFile(seam, registration)).toEqual([])
    expect(violationsForFile('src/some-canvas-component.tsx', registration)).not.toEqual([])
    // OUTSIDE the approved function, inside the approved file.
    expect(violationsForFile(seam, `const stray = new FontFace(family, bytes)\n${registration}`)).not.toEqual([])
  })

  // AND THE CARVE-OUT WAIVES THE TWO FONT SPELLINGS ONLY. The first form of
  // this exception deleted the whole function body from the scanned text,
  // which silently waived every OTHER prohibition inside the one function
  // allowed to touch fonts at all — an AD-17 hole in exactly the place the
  // contract is thinnest. These are the reds that prove it is not waived now,
  // and each is the same line that is red in any ordinary module.
  it('keeps every non-font prohibition live inside the approved seam', () => {
    const seam = 'src/embedded-face-registry.ts'
    const inside = (line: string) => `export function registerCarriedFaces(keys) {\n  ${line}\n}\n`
    for (const line of [
      'const style = getComputedStyle(node)',
      'const w = node.offsetWidth',
      'const observer = new ResizeObserver(() => paint())',
      'const dpr = devicePixelRatio',
      'const height = component.textPaint.lines.length * line.advance',
      `const width = ${['CanvasRenderingContext2D', 'measureText'].join('.')}("x")`,
    ]) {
      expect(violationsForFile(seam, inside(line))).not.toEqual([])
    }
    // The two that ARE waived, on the same lines, in the same function.
    expect(violationsForFile(seam, inside('document.fonts.add(new FontFace(family, bytes))'))).toEqual([])
  })
})

// STORY 14.9 — THE DISPLAY-PAINT EXCEPTION, CHECKED RATHER THAN ONLY WRITTEN.
//
// The three-condition ruling above is the deliverable, and a comment can rot.
// These rows hold the two halves of it that are mechanical: the marker class
// exists on both sides (a paint claiming the exception, and the CSS rule that
// makes condition 2 true of it), and that rule still spells condition 2. They
// do NOT try to prove condition 3 — that is the corpus scan above, which is the
// whole point of the ruling being scoped to this file.
describe('the display-paint exception is marked, and its marker carries condition 2 (Story 14.9)', () => {
  const marker = 'canvas-display-paint'
  const appCss = fs.readFileSync(path.join(sourceDir, 'App.css'), 'utf8')
  const appTsx = fs.readFileSync(path.join(sourceDir, 'App.tsx'), 'utf8')

  it('names the marker for the property, not for the feature that first needed it', () => {
    // ⚠ ANCHORED ON `className=`, NOT ON RAW FILE TEXT. It was
    // `expect(appTsx).toContain(marker)`, which THIS FILE'S OWN COMMENT BLOCK
    // above already satisfies — an instrument whose silence is its answer, and
    // measured green with the class stripped from all six paint sites. The
    // per-ELEMENT pin, over the rendered DOM, is in canvas-table-paint.test.tsx;
    // this row only holds the two file-level facts.
    expect(appTsx).toMatch(new RegExp(`className=[^>]*${marker}`))
    expect(appCss).toContain(`.${marker} {`)
    // ⚠ AND THE PROPERTY IS ASSERTED OF THE CLASS NAMES THE CODE ACTUALLY USES,
    // not of the local literal four lines up — that pair could only fail if
    // someone edited this test's own string, which is a tautology wearing an
    // assertion's clothes. These read the real spellings out of App.css.
    const declared = [...appCss.matchAll(/\.(canvas-display-[a-z-]+)/g)].map((match) => match[1] as string)
    expect(declared, 'App.css must declare at least one display-paint class').toContain(marker)
    for (const name of new Set(declared)) {
      // Named for the PROPERTY, never for the feature that first needed it:
      // `.canvas-table-header-label` would say nothing about the rule it lives
      // under.
      expect(name, name).not.toMatch(/table|column|header|chip/)
      // And no `canvas-text` substring — rules 15/16/17 above forbid
      // font-weight/font-style anywhere on that surface, and the header labels
      // this exception exists for need a weight.
      expect(name, name).not.toContain('canvas-text')
    }
  })

  it('spells condition 2 in the marker\'s own rule — wrapping off, clipped, CSS ellipsis only', () => {
    const rule = appCss.match(new RegExp(`\\.${marker}\\s*\\{([^}]*)\\}`))
    expect(rule, 'App.css must declare a `.canvas-display-paint` rule').toBeTruthy()
    const body = (rule as RegExpMatchArray)[1] as string
    expect(body).toMatch(/white-space:\s*(?:pre|nowrap)\b/)
    expect(body).toMatch(/overflow:\s*hidden\b/)
    // `text-overflow` is a CSS decision about glyphs that nothing reads back.
    // A JS truncation that measured the string or its box would not be, and
    // there is none: the `prohibited` scan above is what forbids the measuring
    // half, and this only records that the clipping is CSS's.
    expect(body).toMatch(/text-overflow:\s*ellipsis\b/)
  })

  it('leaves the whole prohibition list live on the file that carries the exception', () => {
    // The exception is a NAMING convention, not a carve-out in the scan: unlike
    // Story 8.4a's seam and Story 13.2's, nothing about `.canvas-display-paint`
    // removes a single pattern from `App.tsx`. Proved by planting each of the
    // measurement routes inside a marked element and watching the scan wake.
    // ⚠ THE RULES ARE LOOKED UP BY THEIR OWN SOURCE, NOT BY ORDINAL. Every
    // other block in this file writes `prohibited[14]` and friends, and each is
    // correct today — but inserting or reordering one pattern silently
    // re-points a row at a DIFFERENT rule, and the row would go on passing
    // while proving something else. `ruleFor` fails loudly instead, and
    // asserting exactly one match is what makes the lookup a pin.
    const ruleFor = (source: string) => {
      const found = prohibited.filter((pattern) => pattern.source === source)
      expect(found, `no single prohibited pattern has the source ${source}`).toHaveLength(1)
      return found[0] as RegExp
    }
    const offsets = ruleFor('\\b(?:offset(?:Width|Height|Left|Top|Parent)|client(?:Width|Height|Left|Top)|scroll(?:Width|Height|Left|Top))\\b')
    const rects = ruleFor('\\b(?:getBoundingClientRect|getClientRects)\\s*\\(')
    const computed = ruleFor('\\bgetComputedStyle\\s*\\(')
    for (const [line, rule] of [
      ['<span className="canvas-display-paint" style={{ width: node.offsetWidth }} />', offsets],
      ['<span className="canvas-display-paint" style={{ width: box.getBoundingClientRect().width }} />', rects],
      ['<span className="canvas-display-paint" style={{ width: getComputedStyle(cell).width }} />', computed],
      ['<span className="canvas-display-paint">{text.slice(0, columnWidth / cell.scrollWidth)}</span>', offsets],
    ] as const) {
      expect(violationsForSource(line).map(String), line).toContain(String(rule))
    }
    // And App.tsx as it actually stands, carrying the exception, is clean.
    expect(violations([path.join(sourceDir, 'App.tsx')])).toEqual([])
  })
})

// STORY 13.2 — THE FIT MEASUREMENT'S EXCEPTION, PROVED BY RUNNING THE SCAN.
//
// `Fit width` and `Fit page` are the first thing in the designer that needs the
// browser's own answer to "how big is this box". AD-17 does not forbid that
// outright; it forbids it going unnamed. So the exception admits TWO property
// spellings — fit-page's available height is not derivable from the width and
// the page's aspect ratio, so one name was never enough — inside ONE function
// in ONE file, and every row below runs the scan rather than reading the regex.
//
// The rows that matter most are the negative ones. A widening that slipped past
// would be invisible: `clientLeft`, `clientTop` and every `offset*` spelling
// were red in `src/preview/` before this story and are asserted red after it,
// which is the difference between two named rewrites and a group made lazy.
describe('the fit measurement\'s exception is two names in one function (Story 13.2)', () => {
  const previewRelative = path.join('src', 'preview', 'pdf-viewer.tsx')
  const previewSource = fs.readFileSync(path.join(designerRoot, previewRelative), 'utf8')
  // The line the plants are injected after, so each one lands INSIDE the seam.
  const anchor = '  if (!host) return { width: 0, height: 0 }\n'
  const group = String(prohibited[2])

  it('admits the two fit names inside the seam, in the file as committed', () => {
    expect(previewSource).toContain(anchor)
    expect(violationsForFile(previewRelative, previewSource)).toEqual([])
    // Positive control that the file really does take both readings: an
    // exception over a measurement that is not there proves nothing.
    expect(previewSource).toMatch(/host\.client(?:Width|Height)\b/)
    expect(previewSource.match(/host\.client(?:Width|Height)\b/g) ?? []).toHaveLength(2)
  })

  it('leaves every neighbouring measurement red INSIDE the seam', () => {
    for (const [line, rule] of [
      ['const l = host.clientLeft', prohibited[2]],
      ['const t = host.clientTop', prohibited[2]],
      ['const w = host.offsetWidth', prohibited[2]],
      ['const h = host.offsetHeight', prohibited[2]],
      ['const p = host.offsetParent', prohibited[2]],
      ['const rect = host.getBoundingClientRect()', prohibited[1]],
      ['const observer = new ResizeObserver(() => undefined)', prohibited[4]],
      ['const style = getComputedStyle(host)', prohibited[9]],
      ['const dpr = devicePixelRatio', prohibited[7]],
    ] as const) {
      const planted = previewSource.replace(anchor, `${anchor}${line}\n`)
      expect(planted).not.toEqual(previewSource)
      expect(violationsForFile(previewRelative, planted).map(String)).toContain(String(rule))
    }
  })

  it('leaves the two admitted names red OUTSIDE the seam in the same file', () => {
    for (const stray of ['const w = host.clientWidth', 'const h = host.clientHeight']) {
      const planted = `${previewSource}\n${stray}\n`
      expect(violationsForFile(previewRelative, planted).map(String)).toEqual([group])
    }
    // The control the reds are measured against: the same file without them.
    expect(violationsForFile(previewRelative, previewSource)).toEqual([])
  })

  it('leaves the two admitted names red in every other file, preview directory included', () => {
    for (const line of ['const w = node.clientWidth', 'const h = node.clientHeight']) {
      // A production module outside `src/preview/` entirely.
      expect(violationsForFile('src/component-command.ts', line).map(String)).toEqual([group])
      // AND ANOTHER FILE IN THE SAME DIRECTORY. The carve-out is one file, not
      // the folder — the distinction `e2e/` already turns on at ROW 3 below.
      expect(violationsForFile(path.join('src', 'preview', 'freshness.ts'), line).map(String)).toEqual([group])
      expect(violationsForFile(path.join('src', 'preview', 'diagnostic-presenter.tsx'), line).map(String)).toEqual([group])
    }
  })

  it('fails the exception\'s OWN assertion when the measurement seam is gone', () => {
    // `function ` prefixes the DECLARATION only, so this renames the seam and
    // not its call site — `String.replace` substitutes the first occurrence.
    const renamed = previewSource.replace('function measuredViewerBox(host', 'function measuredViewerArea(host')
    expect(renamed).not.toEqual(previewSource)
    // Pinned to the `toMatch` assertion, so a path error or a future TypeError
    // cannot pass for the seam having died with its reason.
    expect(() => violationsForFile(previewRelative, renamed)).toThrow(/to match/)
    // And a SECOND copy of the seam is refused too: the lazy match would
    // otherwise select the prepended one and bound the waiver around it.
    // Pinned to the exactly-once length check for the reason its sibling three
    // lines above gives: a bare `toThrow()` here is satisfied by ANY throw, so
    // a path error or an unrelated TypeError would read as the second copy
    // having been refused.
    const doubled = `${previewSource}\nfunction measuredViewerBox(host: HTMLDivElement | null): PreviewBox {\n  return { width: 0, height: 0 }\n}\n`
    expect(() => violationsForFile(previewRelative, doubled)).toThrow(/to have a length of 1/)
    expect(() => violationsForFile(previewRelative, previewSource)).not.toThrow()
  })

  it('leaves the transient scroll waiver exactly where it was, directory-wide', () => {
    const scrolling = 'const top = host.scrollTop + host.scrollLeft'
    // In the file that also holds the fit seam the seam must still be there for
    // the exception to run at all, so the plant is appended to the real source;
    // in the sibling, which has no seam, it stands alone.
    expect(violationsForFile(previewRelative, `${previewSource}\n${scrolling}\n`)).toEqual([])
    expect(violationsForFile(path.join('src', 'preview', 'freshness.ts'), scrolling)).toEqual([])
    // And still red outside the directory, which is what makes it a waiver.
    expect(violationsForFile('src/component-command.ts', scrolling).map(String)).toEqual([group])
  })
})

// STORY 17.6. ONE TEST PER ROW OF THE STORY'S I/O MATRIX — NINE ROWS, NINE
// TESTS — driven through `violationsForFile`, the harness that addresses the
// scan BY NAME so a file-scoped exception can be proved to hold there and to
// hold nowhere else.
//
// The point of the whole block is that a failing alarm cannot get louder. The
// corpus scan stood red on this one spec for weeks (DW-152), and while it was
// red a second violation anywhere in `src/` or `e2e/` changed the failure's
// CONTENTS and not its STATUS — and no gate reads contents. These are the
// reds that prove it can fire again.
describe('the AD-17 corpus scan can see a NEW violation (Story 17.6)', () => {
  const exemptRelative = path.join('e2e', 'e9-5-border-no-ink.spec.ts')
  const exemptSource = fs.readFileSync(path.join(designerRoot, exemptRelative), 'utf8')
  // Referenced, not re-spelt: if `prohibited`'s entry is retuned these rows
  // must fail for a behavioural reason, never a spelling one.
  const measurement = String(prohibited[9])
  const workflow = fs.readFileSync(path.join(designerRoot, '..', '.github', 'workflows', 'ci.yml'), 'utf8')

  // ROW 1 — the scan, after the exception, against the repo as committed.
  it('finds no violation at all in the repo as committed', () => {
    const exempt = e2e.filter((file) => path.basename(file) === 'e9-5-border-no-ink.spec.ts')
    expect(exempt).toHaveLength(1)
    expect(violations(exempt)).toEqual([])
    expect(violations([...production, ...tests, ...e2e])).toEqual([])
  })

  // ROW 2 — a NEW getComputedStyle in PRODUCTION. The `production` arm is the
  // one that also carries `.css`, so the plant goes in a real `.ts` module it
  // actually looks at.
  it('reds on a NEW getComputedStyle planted in production source', () => {
    expect(production.some((file) => path.basename(file) === 'component-command.ts')).toBe(true)
    expect(violationsForFile('src/component-command.ts', 'const style = getComputedStyle(node)').map(String)).toEqual([measurement])
  })

  // ROW 3 — a NEW getComputedStyle in a DIFFERENT e2e spec. The exception is
  // one FILE, not the folder: no `e2e/**` was waived.
  it('reds on a getComputedStyle planted in another e2e spec — the exception is one file, not the folder', () => {
    expect(e2e.some((file) => path.basename(file) === 'application-shell.spec.ts')).toBe(true)
    expect(violationsForFile('e2e/application-shell.spec.ts', 'const style = getComputedStyle(box)').map(String)).toEqual([measurement])
  })

  // ROW 4 — a SECOND getComputedStyle in the exempt file, OUTSIDE the named
  // block. The exception is one BLOCK, not the file.
  it('reds on a second getComputedStyle in the exempt file, outside the named block', () => {
    const stray = `${exemptSource}\nconst strayStyle = getComputedStyle(document.body)\n`
    expect(stray).not.toEqual(exemptSource)
    expect(violationsForFile(exemptRelative, stray).map(String)).toEqual([measurement])
    // The control the red is measured against: the same file WITHOUT the stray.
    expect(violationsForFile(exemptRelative, exemptSource)).toEqual([])
  })

  // ROW 5 — the exempt seam renamed or deleted. The exception asserts its own
  // reason, so the carve-out cannot outlive the thing it exempts.
  it('fails the exception’s OWN assertion when the exempt seam is gone', () => {
    const renamed = exemptSource.replace('page.evaluate(', 'page.evaluateHandle(')
    expect(renamed).not.toEqual(exemptSource)
    // Pinned to the `toMatch` assertion: a bare `.toThrow()` would pass on a
    // path error or any future refactor's TypeError.
    expect(() => violationsForFile(exemptRelative, renamed)).toThrow(/to match/)
    expect(() => violationsForFile(exemptRelative, exemptSource)).not.toThrow()
  })

  // ROW 6 — every OTHER prohibition inside the exempt block. Only the one
  // spelling is rewritten; the block is not waived. The last line is one of
  // the three ARITHMETIC rules, which are the easiest part of this guard to
  // disturb by accident and are proved live here.
  it('keeps every other prohibition live inside the exempt block', () => {
    const readback = '    const style = getComputedStyle(box)\n'
    expect(exemptSource).toContain(readback)
    // EACH PLANT NAMES THE RULE IT MUST WAKE. `.not.toEqual([])` cannot tell
    // thirteen live rules from twelve, and the three ARITHMETIC rules are the
    // ones the lead flagged as easiest to disturb by accident. They are
    // planted SEPARATELY here because the obvious single line
    // (`textPaint.lines.length * advance`) matches rules 12 AND 13 at once —
    // measured: rule 12 could be deleted outright with 18/18 still passing,
    // because rule 13 covered for it. Each of the three now has a plant only
    // it can answer.
    for (const [line, rule] of [
      ['const w = box.offsetWidth', prohibited[2]],
      ['const rect = box.getBoundingClientRect()', prohibited[1]],
      ['const observer = new ResizeObserver(() => undefined)', prohibited[4]],
      ['const dpr = devicePixelRatio', prohibited[7]],
      [`const width = ${['CanvasRenderingContext2D', 'measureText'].join('.')}("x")`, prohibited[0]],
      // Rule 12 ALONE — no adjacent operator, so rule 13 cannot answer for it.
      ['const height = component.textPaint.lines.length', prohibited[11]],
      // Rule 13 ALONE — no `textPaint`/`paint` prefix, so rule 12 cannot.
      ['const n = lines.length * 2', prohibited[12]],
      // Rule 14 ALONE.
      ['const top = canvas.contentWindowHeight * index', prohibited[13]],
    ] as const) {
      const planted = exemptSource.replace(readback, `${readback}    ${line}\n`)
      expect(planted).not.toEqual(exemptSource)
      expect(violationsForFile(exemptRelative, planted).map(String)).toContain(String(rule))
    }
  })

  // ROW 7 — the population. It did not shrink, and the arms are still the
  // asymmetric ones they were: `production` carries `.css`, `tests` and `e2e`
  // are `.ts`/`.tsx` only.
  it('leaves the scanned population and its three non-vacuity floors where they were', () => {
    expect(production.length).toBeGreaterThan(10)
    expect(tests.length).toBeGreaterThan(10)
    expect(e2e.length).toBeGreaterThan(3)
    expect(production.filter((file) => file.endsWith('.css')).length).toBeGreaterThan(0)
    expect(tests.filter((file) => !/\.tsx?$/.test(file))).toEqual([])
    expect(e2e.filter((file) => !/\.tsx?$/.test(file))).toEqual([])
    // THE FLOORS ABOVE CANNOT SEE A SHRINK, WHICH IS THE FAILURE MODE THIS
    // STORY EXISTS TO PREVENT — an exception that "works" by scanning fewer
    // files. Measured: narrowing the `production` filter from `.ts|.tsx|.css`
    // to `.ts|.css` drops all 8 `.tsx` files, INCLUDING `App.tsx` — the canvas
    // projection itself, the file AD-17 is most about, and the very file the
    // `placementPoint` carve-out below exists for — leaving 50 files, still
    // over the floor of 10, with every other assertion in this row passing.
    // 18/18 green over a corpus missing the code the rule is about.
    //
    // These are the counts enumerated at 995ec5c. `toBeGreaterThanOrEqual`,
    // not `toBe`, so ordinary growth never churns the guard while any shrink
    // reddens and has to be raised deliberately.
    expect(production.length).toBeGreaterThanOrEqual(58)
    expect(tests.length).toBeGreaterThanOrEqual(51)
    expect(e2e.length).toBeGreaterThanOrEqual(15)
    expect(production.filter((file) => file.endsWith('.tsx')).length).toBeGreaterThanOrEqual(8)
    expect(production.filter((file) => file.endsWith('.css')).length).toBeGreaterThanOrEqual(3)
    // AND NO e2e SPEC MAY BE EXCLUDED BY NAME. Excluding one by name is an
    // established idiom in this very file (`tests` excludes this file at :15),
    // so the e2e arm is compared against an INDEPENDENT walk of the directory
    // rather than against its own filter.
    const e2eOnDisk = fs.readdirSync(path.join(designerRoot, 'e2e'), { recursive: true })
      .filter((entry): entry is string => typeof entry === 'string' && /\.tsx?$/.test(entry))
    expect(new Set(e2e.map((file) => path.relative(path.join(designerRoot, 'e2e'), file)))).toEqual(new Set(e2eOnDisk))
    expect(e2e.filter((file) => path.basename(file) === 'e9-5-border-no-ink.spec.ts')).toHaveLength(1)
  })

  // ROW 8 — `npm test` runs whole. No test is excluded by name any more,
  // neither by the package script nor by the workflow step.
  it('excludes no test by name — the designer suite runs whole', () => {
    const scripts = (JSON.parse(fs.readFileSync(path.join(designerRoot, 'package.json'), 'utf8')) as { scripts: Record<string, string> }).scripts
    // Positive control that this really is the script the suite runs under.
    expect(scripts.test).toContain('vitest run')
    expect(scripts.test).not.toMatch(/\s-t\s/)
    expect(workflow).toMatch(/npx vitest run\n/)
    // Every spelling of a name filter, not just `-t`.
    expect(workflow).not.toMatch(/vitest run[^\n]*(?:-t\b|--testNamePattern|--exclude)/)
    expect(scripts.test).not.toMatch(/--testNamePattern|--exclude/)
    // AND THE THIRD FILE THAT CAN REMOVE A TEST FROM THE RUN. `package.json`
    // and `ci.yml` are not the only doors: adding `exclude` to vite's `test`
    // block drops a file from the suite without touching either — and the file
    // it would most usefully drop is THIS one, taking the AD-17 corpus scan
    // and all nine of these rows with it, silently and green.
    const viteConfig = fs.readFileSync(path.join(designerRoot, 'vite.config.ts'), 'utf8')
    expect(viteConfig).toMatch(/include:\s*\[/)
    expect(viteConfig).not.toMatch(/exclude/)
  })

  // ROW 9 — ci.yml. The quarantine is gone; the Go one, which reports an
  // honestly unmet exercise floor and is never to be "fixed", is untouched.
  it('carries no quarantined designer job in ci.yml, and leaves folio-go-known-red alone', () => {
    // POSITIVE CONTROLS FIRST, so the two absences below are real silence and
    // not a failed read of the wrong file.
    expect(workflow).toMatch(/^ {2}folio-designer:$/m)
    expect(workflow).toMatch(/^ {2}folio-go-known-red:$/m)
    expect(workflow).toMatch(/KNOWN_RED_TEST: "\^TestCorpusMeetsP6ExerciseFloors\$"/)
    expect(workflow).not.toMatch(/folio-designer-known-red/)
    expect(workflow).not.toMatch(/DESIGNER_KNOWN_RED/)
  })
})

function violationsForSource(source: string): RegExp[] { return violationsForFile('src/an-ordinary-module.ts', source) }
// The same scan a real file gets, addressed by NAME, so an exception that is
// scoped to a file can be proved to hold there and to hold nowhere else.
function violationsForFile(file: string, source: string): RegExp[] { return prohibited.filter((pattern) => pattern.test(scanned(path.join(designerRoot, file), source))) }
function refusalForSource(source: string): RegExp[] { return refusalVocabulary.filter((pattern) => pattern.test(withoutComments(source))) }

// THE TWO SCOPED EXCEPTIONS TO THE FONT RULES, each written the way
// withoutApprovedLocalPointerInput is: narrow to a named owner, and asserted
// to still have the thing that earns it, so the carve-out cannot outlive its
// reason.
function withoutApprovedRuntimeFaceRegistration(file: string, source: string): string {
  // READINESS IS NOT REGISTRATION AND NOT MEASUREMENT, and it is legal
  // everywhere. e2e/engine-worker.spec.ts awaits it so that in-flight requests
  // for the BUILD-TIME faces are not counted as offline failures; nothing is
  // measured, nothing is added, and no layout waits on it.
  const readiness = source.replace(/document\.fonts\.ready\b/g, 'fontReadinessOnly')
  // THE ONE SEAM THAT MAY REGISTER A FACE WHILE A DOCUMENT IS OPEN (Story
  // 8.4a). A face the DOCUMENT carries exists only inside that document, so it
  // cannot be declared at build time; it is fetched over the engine's own
  // `asset` operation and added to the page's font set for as long as that
  // document is open.
  //
  // THE CARVE-OUT IS TWO SPELLINGS INSIDE ONE FUNCTION, NOT THE FUNCTION.
  // It was written as a deletion of the whole function body, which read as
  // "the exception is the FUNCTION, not the file" but actually waived EVERY
  // prohibition inside it — `getComputedStyle`, `offsetWidth`,
  // `ResizeObserver`, `devicePixelRatio` and the pagination-from-paint rules
  // included — inside the one function in the designer that is allowed
  // anywhere near fonts. That is an AD-17 hole, and it is the shape the
  // sibling carve-out below already avoided. Only `new FontFace` and
  // `document.fonts` are neutralised here; every other prohibition still
  // applies inside `registerCarriedFaces`, proved by
  // `keeps every non-font prohibition live inside the approved seam`. The
  // scope is still bounded by the function: text outside it is untouched, and
  // if the function is renamed or removed the exception dies with it.
  // canvas-font-stack.test.ts separately asserts this is the only site in the
  // whole designer.
  if (path.basename(file) === 'embedded-face-registry.ts') {
    const seam = /export function registerCarriedFaces\([\s\S]*?\n}\n/
    expect(readiness).toMatch(seam)
    return readiness.replace(seam, (body) => body.replace(/new FontFace\b/g, 'approvedSeamFaceConstruction').replace(/document\.fonts\b/g, 'approvedSeamFontSet'))
  }
  // THE DETECTOR'S OWN FIXTURES. canvas-font-stack.test.ts is the test that
  // proves runtime registration happens in exactly one place, and it cannot
  // prove its scanner detects a mechanism without spelling that mechanism. The
  // exception is narrowed to the two font spellings — every other prohibition
  // still applies to that file — and it holds only while the detector is
  // actually there.
  if (path.basename(file) === 'canvas-font-stack.test.ts') {
    expect(readiness).toMatch(/function registersAFaceAtRuntime\(source: string\): boolean/)
    return readiness.replace(/new FontFace\b/g, 'faceDetectorFixture').replace(/document\.fonts\b/g, 'fontSetDetectorFixture')
  }
  return readiness
}

function withoutApprovedLocalPointerInput(file: string, source: string): string {
  if (file.includes(`${path.sep}preview${path.sep}`)) {
    // PDF viewer scroll is deliberately transient viewer navigation, never a
    // document/canvas measurement. Keep that exception narrow to this owner.
    const transient = source.replace(/scroll(?:Width|Height|Left|Top)\b/g, 'viewerTransientState')
    // STORY 13.2 — THE FIT MEASUREMENT, AND THE ONLY TWO NAMES IT CONSUMES.
    //
    // `Fit width` and `Fit page` need the scroll container's real pixel box.
    // The available height is NOT derivable from the width and the page's
    // aspect ratio, so fit-page forces a SECOND name, and each of the two is
    // spelled on its own `.replace` below rather than by widening the
    // `client(?:Width|Height|Left|Top)` group — widening the group would waive
    // `clientLeft` and `clientTop` in the same stroke, silently and with
    // nothing to notice it. Those two, every `offset*` spelling and
    // `getBoundingClientRect` all stay red inside this directory, and the rows
    // in `the fit measurement's exception is two names in one function` RUN the
    // scan against each rather than reading this regex back.
    //
    // A fit scale is not the fidelity constant `previewOversample` is. That
    // constant exists precisely BECAUSE the display's pixel ratio is banned and
    // an image can be oversampled by a fixed multiple instead; a container's
    // pixel size has no constant that can stand in for it, and AD-17's own
    // answer for that case is a named, scoped exception rather than a guess.
    //
    // THE CARVE-OUT IS ONE FUNCTION IN ONE FILE, not this directory. It is
    // written in the shape of the `embedded-face-registry.ts` sibling above —
    // scoped by `path.basename`, bounded to a matched region rather than to the
    // file, and asserting the seam that earns it is still present, so the
    // exception dies with its reason instead of outliving it. This exception is
    // the one carve-out in this file that asserted no seam at all; now it does.
    if (path.basename(file) !== 'pdf-viewer.tsx') return transient
    const seam = /function measuredViewerBox\(host: HTMLDivElement \| null\): PreviewBox \{[\s\S]*?\n}\n/
    expect(transient).toMatch(seam)
    // AND EXACTLY ONCE, for the reason the Story 17.6 sibling gives: a second
    // copy of the seam prepended above this one would otherwise be the region
    // the lazy match selects, waiving a reading this story exists to bound.
    expect(transient.match(new RegExp(seam, 'g')) ?? []).toHaveLength(1)
    return transient.replace(seam, (region) => region.replace(/\bclientWidth\b/g, 'viewerFitContainerWidth').replace(/\bclientHeight\b/g, 'viewerFitContainerHeight'))
  }
  if (path.basename(file) !== 'App.tsx') return source
  // The sole approved pointer coordinate is isolated to a named transient
  // proposal helper. It is not DOM measurement and never reaches paint.
  const seam = /export function placementPoint\(event: Pick<MouseEvent,[\s\S]*?\n}\nfunction pageStyle/
  expect(source).toMatch(seam)
  return source.replace(seam, 'function pageStyle')
}

// STORY 17.6. THE INSTRUMENT MAY READ WHAT THE BROWSER PAINTED; THE PRODUCT
// MAY NOT.
//
// AD-17's subject is the PRODUCT: the canvas gets every text metric from the
// engine and never measures. `e2e/e9-5-border-no-ink.spec.ts` is a Playwright
// assertion that reads the borders the page actually painted and compares the
// RESOLVED ink against an exact expected list of one. That is an instrument
// measuring the product's output — the opposite of the product measuring
// itself — and rewriting it to read the projection's own declared border back
// to itself would make both sides of the assertion move together, so it would
// pass through the very E9-5 defect it was written for.
//
// THE CARVE-OUT IS ONE SPELLING INSIDE ONE NAMED BLOCK IN ONE NAMED FILE, and
// it is deliberately narrower than the `document.fonts.ready` rewrite above,
// which is repo-wide and scoped to no owner at all. It is written in the shape
// of the `embedded-face-registry.ts` sibling: scoped by `path.basename`,
// bounded to a matched region rather than to the file, asserting the seam that
// earns it is still present so the exception dies with its reason, and
// rewriting ONLY `getComputedStyle`. Every other prohibition — `offsetWidth`,
// `getBoundingClientRect`, `ResizeObserver`, `devicePixelRatio` and the three
// pagination-arithmetic rules — is still live inside the block, and a SECOND
// `getComputedStyle` anywhere else in the same file is still red.
//
// This is what makes the corpus scan an alarm again. It stood red on this one
// file for weeks (DW-152), and while it was red a new violation anywhere in
// the designer changed the failure's CONTENTS and not its STATUS.
function withoutApprovedPaintedBorderReadback(file: string, source: string): string {
  if (path.basename(file) !== 'e9-5-border-no-ink.spec.ts') return source
  // THE SEAM IS THE WHOLE INSTRUMENT, NOT JUST THE READBACK. An earlier form
  // of this ended at `}).sort())` — the close of `page.evaluate` — which meant
  // the carve-out survived the assertion being gutted: replacing
  // `expect.poll(...).toEqual([...])` with a bare `await page.evaluate(...)`
  // left the seam matching and the whole suite green, so the one file allowed
  // to call `getComputedStyle` kept that permission while asserting nothing.
  // MEASURED, not reasoned: 18/18 passed with the comparison removed. The
  // reason for the exception is that this compares RESOLVED ink against an
  // exact expected list, so the comparison is now part of what must still be
  // there for the exception to hold.
  const seam = /expect\.poll\(\(\) => page\.evaluate\(\(\) => Array\.from\(document\.querySelectorAll\('\.canvas-box'\)\)[\s\S]*?\}\)\.sort\(\)\)\)\.toEqual\(\[[^\]]*\]\)/
  expect(source).toMatch(seam)
  // AND EXACTLY ONCE. A second `.canvas-box` readback block prepended above
  // this one would otherwise be the region the lazy match selected, waiving a
  // `getComputedStyle` the story exists to catch.
  expect(source.match(new RegExp(seam, 'g')) ?? []).toHaveLength(1)
  return source.replace(seam, (region) => region.replace(/\bgetComputedStyle\b/g, 'approvedPaintedBorderReadback'))
}
