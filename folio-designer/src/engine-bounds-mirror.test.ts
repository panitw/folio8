import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { BANDS_CAPPING_VERTICALLY, CAPPING_BANDS, LOCALE_TAGS, SCALAR_BINDING_COMPONENT_TYPES, type CappingBand } from './engine-protocol'

// D-7.4.5 / DW-25, AS WIDENED BY STORY 7.5.
//
// THE STANDING OBLIGATION, in its current form: ANY invariant duplicated
// across the Go/TypeScript boundary moves in ONE commit, with a test that
// reads both sides. It used to read "the size caps move together", and that
// wording was the defect — DW-25 audited and closed the four SIZE CAPS, and
// band containment, a different invariant that merely lives in the same file,
// was left untied and drifted anyway. An audit closes only what it measured.
//
// `engine-protocol.ts` hand-copies the canvas projection bounds
// `folio-go/page_setup.go` declares, and the band-containment predicate
// `folio-go/component_commands.go` declares. There is no shared source and no
// codegen, so the only thing standing between the files is this test: it
// reads BOTH sides and asserts they agree. A comment asking the next person
// to remember is precisely what this replaces.
//
// WHAT THE TIE COMPARES, AND WHAT IT DOES NOT. It compares LITERALS, not
// quantities. Go's `len()` counts BYTES; TypeScript's `.length` counts UTF-16
// CODE UNITS, so for non-ASCII the browser side is the more permissive of the
// two string bounds. That asymmetry is pre-existing, is safe in that
// direction — Go refuses first, so nothing unrepresentable crosses — and is
// recorded here rather than "fixed", because equal numerals is exactly the
// property that keeps the two sides from drifting apart.
//
// Go's per-line `maxCanvasTextFragments` is deliberately absent: the browser
// counts fragments CUMULATIVELY across a component, which is a different
// quantity, and pairing them would assert a tie that does not exist.
//
// NOT EVERY MIRROR COMES FROM ONE GO FILE. Story 7.4 projects `lineSpacing`
// across the channel, whose range is declared in
// `folio-go/internal/template/linespacing.go` and whose maximum that file
// explicitly calls "A STATED SANITY CEILING" — a number somebody will adjust.
// Raising it in Go alone would leave the browser silently dropping every
// snapshot of such a document, so its pair is tied here too, read from ITS
// OWN source rather than assumed to live beside the others.

const sourceDir = path.dirname(fileURLToPath(import.meta.url))
const goSources = {
  pageSetup: path.resolve(sourceDir, '../../folio-go/page_setup.go'),
  lineSpacing: path.resolve(sourceDir, '../../folio-go/internal/template/linespacing.go'),
  // Story 7.5's mirror is a PREDICATE, not a numeral, and it lives in a third
  // Go file. Reading it here is the point: a tie list that only ever reaches
  // the files it already knew about is the audit's blind spot restated.
  componentCommands: path.resolve(sourceDir, '../../folio-go/component_commands.go'),
  // Story 12.2's mirror is a CLOSED SET OF STRINGS, and it lives in a fourth Go
  // file. AD-12's four locale tags are spelled exactly once in Go — as the
  // right-hand sides of four named constants in this file — and exactly once
  // here, in engine-protocol.ts's LOCALE_TAGS. Nothing but the describe block
  // at the foot of this file stands between them.
  locale: path.resolve(sourceDir, '../../folio-go/internal/template/locale.go'),
  // Story 14.4's mirror needs a FIFTH source, and needs it for the same reason
  // the locale one needed a fourth: the rule it ties is spelled across two Go
  // files. `component_commands.go` states the gate as `element.Type !=
  // template.ElementText`; the constant that names the kind is declared here,
  // in the closed five of `ElementType`. Resolving the identifier through its
  // own declaration is what makes the comparison a claim about the RULE rather
  // than about a spelling.
  templateModel: path.resolve(sourceDir, '../../folio-go/internal/template/model.go'),
  // Story 14.7b's mirror needs a SIXTH source, and it is the first one that is
  // not a FORMAT package: `historyLimit` is declared in the WASM SHELL — still
  // inside `folio-go`, and still the same module as every source above it —
  // because the undo/redo stacks are a property of a live wasm session rather
  // than of the document format. The table editor's Cancel is a compensating sequence of N
  // `undo` operations, so the depth of that history is a bound the browser side
  // now has to know — and `historyLimit` appears in Go at exactly two places
  // (its declaration and `appendBounded`) and nowhere in TypeScript until this
  // story mirrored it.
  history: path.resolve(sourceDir, '../../folio-go/internal/wasm/engine.go'),
} as const
const tsPath = path.join(sourceDir, 'engine-protocol.ts')
// Story 7.6's THIRD consumer of the band-containment tie. The drag clamp used
// to cap every band vertically with its own inline rule — a fourth spelling
// of an invariant three files already state — and lifting the content band in
// Go and in the protocol while leaving it clamped here would have shipped a
// column reachable by command and not by hand. It reads the list; this test
// reads it reading the list.
const dragClampPath = path.join(sourceDir, 'resize-anchor.ts')
// Story 12.5's mirrored bound, and the THIRD kind of thing this file ties: not
// a numeral declared on both sides, and not a list, but ONE TERM lifted out of
// a Go expression. `bandContentWindowCeiling` is `innerH - other - 1`; every
// input to it is projected, and the `- 1` is not. DW-36's standing condition —
// a browser-side bound must CONSUME the engine's declaration and be caught
// doing so — is what makes the drag clamp above legal, and it is what makes
// this one legal too.
const bandBoundaryPath = path.join(sourceDir, 'band-boundary.ts')

type GoSource = keyof typeof goSources
type Pair = Readonly<{ go: string; source: GoSource; ts: string; sites: ReadonlyArray<RegExp> }>

// Each pair names a Go constant, the Go FILE it is declared in, its
// TypeScript mirror, and the validator sites that must actually CONSUME the
// mirror. Without the site regexes a hoisted constant could sit in the file
// unused while the validator kept a stale inline literal, and the tie would
// pass while the canvas blanked.
const pairs: ReadonlyArray<Pair> = [
  { go: 'maxCanvasBodyText', source: 'pageSetup', ts: 'MAX_CANVAS_BODY_TEXT', sites: [/boundedString\('value', MAX_CANVAS_BODY_TEXT\)/, /fragment\.text\.length <= MAX_CANVAS_BODY_TEXT\b/] },
  { go: 'maxCanvasBodyTextLines', source: 'pageSetup', ts: 'MAX_CANVAS_BODY_TEXT_LINES', sites: [/value\.lines\.length > MAX_CANVAS_BODY_TEXT_LINES\b/] },
  { go: 'maxCanvasBodyTextFragments', source: 'pageSetup', ts: 'MAX_CANVAS_BODY_TEXT_FRAGMENTS', sites: [/fragments <= MAX_CANVAS_BODY_TEXT_FRAGMENTS\b/] },
  { go: 'maxCanvasPropertyString', source: 'pageSetup', ts: 'MAX_CANVAS_PROPERTY_STRING', sites: [/boundedString\(key, MAX_CANVAS_PROPERTY_STRING\)/] },
  { go: 'MinLineSpacingThousandths', source: 'lineSpacing', ts: 'MIN_LINE_SPACING_THOUSANDTHS', sites: [/component\.lineSpacing < MIN_LINE_SPACING_THOUSANDTHS\b/] },
  { go: 'MaxLineSpacingThousandths', source: 'lineSpacing', ts: 'MAX_LINE_SPACING_THOUSANDTHS', sites: [/component\.lineSpacing > MAX_LINE_SPACING_THOUSANDTHS\b/] },
  // Story 8.1. maxCanvasFontFamilies had a TypeScript mirror and no pair from
  // the day it was written — the one-sided edit this list exists to catch,
  // sitting inside the list itself. It is tied here alongside the per-chain
  // entry bound the same story added, so the projection's font half is not
  // half-tied.
  { go: 'maxCanvasFontFamilies', source: 'pageSetup', ts: 'MAX_ENGINE_FONT_FAMILIES', sites: [/value\.fontFamilies\.length > MAX_ENGINE_FONT_FAMILIES\b/] },
  { go: 'maxCanvasFontChainEntries', source: 'componentCommands', ts: 'MAX_ENGINE_FONT_CHAIN_ENTRIES', sites: [/chain\.entries\.length <= MAX_ENGINE_FONT_CHAIN_ENTRIES\b/] },
  // STORY 14.4 / D-14.4.Q4. The binding-string bound had a mirror on both sides
  // and no row here from the day either was written — the one-sided edit this
  // list exists to catch, sitting beside the list. It is in scope because it is
  // the SAME SUBJECT as the scalar-binding tie below (Go/TS agreement on
  // binding validation) in the SAME FILE that story was already extending.
  { go: 'maxCanvasBindingString', source: 'pageSetup', ts: 'MAX_ENGINE_BINDING_LENGTH', sites: [/component\.binding\.length > MAX_ENGINE_BINDING_LENGTH\b/] },
]

// Both spellings a Go declaration can take: a file-scope `const NAME = N` and
// a member of a grouped `const ( … )` block, which is how linespacing.go
// declares its pair.
function goConstant(source: string, name: string): string | undefined {
  return source.match(new RegExp(`^(?:const[ \\t]+|[ \\t]+)${name} = (\\d+)$`, 'm'))?.[1]
}

function tsConstant(source: string, name: string): string | undefined {
  return source.match(new RegExp(`^export const ${name} = (\\d+)$`, 'm'))?.[1]
}

describe('canvas projection bounds mirror', () => {
  const sources: Record<GoSource, string> = { pageSetup: fs.readFileSync(goSources.pageSetup, 'utf8'), lineSpacing: fs.readFileSync(goSources.lineSpacing, 'utf8'), componentCommands: fs.readFileSync(goSources.componentCommands, 'utf8'), locale: fs.readFileSync(goSources.locale, 'utf8'), templateModel: fs.readFileSync(goSources.templateModel, 'utf8'), history: fs.readFileSync(goSources.history, 'utf8') }
  const goValue = (pair: Pair) => goConstant(sources[pair.source], pair.go)
  const ts = fs.readFileSync(tsPath, 'utf8')

  it('finds every declared pair on both sides of the channel', () => {
    // Non-vacuity. A regex that quietly stops matching is the exact failure
    // mode a tie assertion exists to prevent, so the found set is asserted
    // whole before any equality is claimed from it.
    expect(pairs.map((pair) => [pair.go, goValue(pair)])).toEqual(pairs.map((pair) => [pair.go, expect.stringMatching(/^\d+$/)]))
    expect(pairs.map((pair) => [pair.ts, tsConstant(ts, pair.ts)])).toEqual(pairs.map((pair) => [pair.ts, expect.stringMatching(/^\d+$/)]))
    // The count, and the fact that EVERY Go source a NUMERAL pair names is
    // actually read: a pair list that quietly lost its only linespacing.go
    // member would otherwise leave this file reading one source and still
    // calling itself the tie. (`goSources` itself now has SIX entries — the
    // locale and template-model files are read by the predicate describes at
    // the foot of this file, and `folio-go/internal/wasm/engine.go` by Story 14.7b's history-limit
    // describe, none of them by a numeral pair in THIS list.)
    expect(pairs).toHaveLength(9)
    // The NUMERAL pairs read three of the SIX sources: componentCommands
    // carries the band-containment predicate tie below AND, since Story 8.1,
    // the per-chain entry bound the font-chain commands and the projection
    // share. `locale` and `templateModel` carry no numeral and appear only in
    // the predicate describes, and `history` by the history-limit describe at the
    // foot of this file.
    expect(new Set(pairs.map((pair) => pair.source))).toEqual(new Set(['pageSetup', 'lineSpacing', 'componentCommands']))
    // AND THE NUMBER OF SOURCES IS ASSERTED RATHER THAN NARRATED, because the
    // sentence above had already gone stale once: it said FIVE while `goSources`
    // held six. A seventh entry now has to face this line.
    expect(Object.keys(goSources)).toHaveLength(6)
  })

  it('holds every Go bound and its TypeScript mirror at the same number', () => {
    expect(pairs.map((pair) => `${pair.go}=${goValue(pair)}`)).toEqual(pairs.map((pair) => `${pair.go}=${tsConstant(ts, pair.ts)}`))
    // The derivations, pinned as numerals so a silent joint edit of both
    // files still has to face the arithmetic recorded in the Go comments.
    expect(goConstant(sources.pageSetup, 'maxCanvasBodyTextLines')).toBe('1920')
    expect(goConstant(sources.pageSetup, 'maxCanvasBodyTextFragments')).toBe('65536')
    expect(goConstant(sources.pageSetup, 'maxCanvasBodyText')).toBe('1048576')
    expect(goConstant(sources.pageSetup, 'maxCanvasPropertyString')).toBe('512')
    expect(goConstant(sources.lineSpacing, 'MinLineSpacingThousandths')).toBe('1')
    expect(goConstant(sources.lineSpacing, 'MaxLineSpacingThousandths')).toBe('1000000')
    expect(goConstant(sources.pageSetup, 'maxCanvasBindingString')).toBe('256')
  })

  it('consumes every mirrored constant at the validator site it bounds', () => {
    for (const pair of pairs) for (const site of pair.sites) expect(ts).toMatch(site)
  })

  it('keeps body text and identifiers on separate bounds on both sides', () => {
    // The split is the whole point (D-7.4.2 §3): the element VALUE must not
    // be governed by the identifier bound on either side of the channel.
    expect(sources.pageSetup).toMatch(/if len\(element\.Value\.Value\) > maxCanvasBodyText \{/)
    const identifiers = ts.match(/\[([^[\]]*)\]\.every\(optionalString\)/)?.[1] ?? ''
    expect(identifiers).toContain("'fontFamily'")
    expect(identifiers).not.toContain("'value'")
  })

  it('turns a one-sided edit of any pair red', () => {
    // The red-proof: mutate the TypeScript literal alone, exactly as a
    // careless raise-the-number change would, and the comparison must fail.
    for (const pair of pairs) {
      const drifted = ts.replace(new RegExp(`^export const ${pair.ts} = (\\d+)$`, 'm'), `export const ${pair.ts} = 7`)
      expect(drifted).not.toBe(ts)
      expect(tsConstant(drifted, pair.ts)).not.toBe(goValue(pair))
    }
    // And the same in the other direction, from the Go side — once per Go
    // SOURCE, because the linespacing.go pair is declared in a grouped const
    // block and a regex that only understood the file-scope spelling would
    // pass this test while tying nothing.
    const driftedLines = sources.pageSetup.replace(/^const maxCanvasBodyTextLines = (\d+)$/m, 'const maxCanvasBodyTextLines = 256')
    expect(driftedLines).not.toBe(sources.pageSetup)
    expect(goConstant(driftedLines, 'maxCanvasBodyTextLines')).not.toBe(tsConstant(ts, 'MAX_CANVAS_BODY_TEXT_LINES'))
    const driftedCeiling = sources.lineSpacing.replace(/^(\s+)MaxLineSpacingThousandths = (\d+)$/m, '$1MaxLineSpacingThousandths = 2000000')
    expect(driftedCeiling).not.toBe(sources.lineSpacing)
    expect(goConstant(driftedCeiling, 'MaxLineSpacingThousandths')).not.toBe(tsConstant(ts, 'MAX_LINE_SPACING_THOUSANDTHS'))
  })
})

// Go declares the list through named constants, so the identifiers are
// resolved before comparison: `[]string{bandPageHeader, bandPageFooter}` and
// `['pageHeader', 'pageFooter']` are the same claim spelled two ways, and it
// is the CLAIM that has to match, not the spelling.
function goBandsCappingVertically(source: string): ReadonlyArray<string> {
  const names = new Map<string, string>()
  for (const match of source.matchAll(/^[ \t]*(band[A-Za-z]+)\s+= "([^"]+)"$/gm)) names.set(match[1] as string, match[2] as string)
  const list = source.match(/^var bandsCappingVertically = \[\]string\{([^}]*)\}$/m)?.[1]
  if (list === undefined) return []
  return list.split(',').map((entry) => entry.trim()).filter((entry) => entry.length > 0).map((entry) => names.get(entry) ?? entry)
}

function tsBandsCappingVertically(source: string): ReadonlyArray<string> {
  const list = source.match(/^export const BANDS_CAPPING_VERTICALLY = \[([^\]]*)\]$/m)?.[1]
  if (list === undefined) return []
  return list.split(',').map((entry) => entry.trim().replace(/^'|'$/g, '')).filter((entry) => entry.length > 0)
}

// STORY 7.5's MIRROR, and the first one here that ties a PREDICATE rather
// than a numeral.
//
// The invariant: which bands cap a component's vertical extent. Go enforces
// it on the COMMAND path (containComponent) and TypeScript enforces it again
// on the PROJECTION path (isCanvas) — and the asymmetry is what makes a
// one-sided edit dangerous. Go's canvasComponents does not re-check
// containment when projecting, so Go already projects an out-of-band element
// happily and TypeScript alone kills it: lifting the cap in Go without
// lifting it here would ship a story that is invisible in the running app,
// because parseInbound would return undefined, terminate the worker and blank
// the canvas for exactly the documents the story exists to make authorable.
describe('band containment mirror', () => {
  const go = fs.readFileSync(goSources.componentCommands, 'utf8')
  const ts = fs.readFileSync(tsPath, 'utf8')
  const dragClamp = fs.readFileSync(dragClampPath, 'utf8')

  it('reads a non-empty list from every side', () => {
    // Non-vacuity first: a regex that quietly stops matching would make every
    // equality below true and meaningless.
    expect(goBandsCappingVertically(go)).toEqual(['pageHeader', 'pageFooter'])
    expect(tsBandsCappingVertically(ts)).toEqual(['pageHeader', 'pageFooter'])
  })

  it('agrees on which bands cap a component vertically', () => {
    expect(goBandsCappingVertically(go)).toEqual(tsBandsCappingVertically(ts))
    // The content band is on NEITHER list, which is the whole of Story 7.5:
    // a column has no height to be inside of.
    expect(goBandsCappingVertically(go)).not.toContain('content')
    expect(tsBandsCappingVertically(ts)).not.toContain('content')
  })

  it('consumes the list at the site it governs, in all three consumers', () => {
    // A list nothing reads would tie dead declarations together while the
    // real gates kept their own inline spellings.
    expect(go).toMatch(/if !outside && slices\.Contains\(bandsCappingVertically, band\.Name\) \{/)
    expect(go).toMatch(/func containEdgeY\(band designer\.CanvasBand, value, limit geom\.Length\) geom\.Length \{\n\tif !slices\.Contains\(bandsCappingVertically, band\.Name\) \{/)
    expect(ts).toMatch(/BANDS_CAPPING_VERTICALLY\.includes\(component\.band as string\) && !\(box\.y \+ box\.height <= band\.height\)/)
    // The DRAG CLAMP (DW-36), which reads the list from engine-protocol.ts
    // rather than restating it, and gates the ONE vertical limit both of its
    // vertical clamps consume.
    expect(dragClamp).toMatch(/import \{ BANDS_CAPPING_VERTICALLY, type CanvasProjection \} from '\.\/engine-protocol'/)
    expect(dragClamp).toMatch(/^ {2}const limitHeight = limit && BANDS_CAPPING_VERTICALLY\.includes\(limit\.band\) \? limit\.height : Number\.POSITIVE_INFINITY$/m)
  })

  // STORY 12.1's REVIEW ADDED TWO MORE SPELLINGS OF THIS LIST AND THIS IS WHERE
  // THEY WERE PUT BACK.
  //
  // The band-height command takes a BAND as a parameter, so it needed the list
  // as a type and the panel needed it as something to iterate. Both were first
  // written out again — `SettableBand` in band-height-command.ts and
  // `settableBands` in App.tsx — which made four and five copies of a list whose
  // entire safety property is that this file reads it on both sides. A copy
  // outside the census is the only kind that can go stale unnoticed, and the
  // cost of this one going stale is the cost of every other copy of it: the
  // browser drops the snapshot, terminates the worker and blanks the canvas.
  it('hands the SAME list to the type and the array the command path consumes', () => {
    // The array is the same object, not an equal one: a member added to
    // BANDS_CAPPING_VERTICALLY is in CAPPING_BANDS by identity.
    expect(CAPPING_BANDS).toBe(BANDS_CAPPING_VERTICALLY)
    expect([...CAPPING_BANDS]).toEqual(goBandsCappingVertically(go))
    // AND THE UNION, TIED THE SAME WAY THE ARRAYS ARE: by reading the source.
    // A type cannot be enumerated at run time, so the DERIVATION is read out of
    // engine-protocol.ts instead — CappingBand must be the projection's own
    // band-name union (which canvas_projection_wire_test.go pins against Go)
    // minus the one band with no height to cap with — and the union that
    // derivation produces is then computed from that same union's source text
    // and compared to Go's list. A third band added in Go reaches the projection
    // union, reaches CappingBand through Exclude, and reds here.
    expect(ts).toMatch(/^export type CappingBand = Exclude<CanvasProjection\['bands'\]\[number\]\['name'\], 'content'>$/m)
    const projectedBandNames = ts.match(/^[ \t]+bands: ReadonlyArray<Readonly<\{ name: ([^;]+);/m)?.[1]
    expect(projectedBandNames).toBeDefined()
    const derivedUnion = (projectedBandNames ?? '').split('|').map((entry) => entry.trim().replace(/^'|'$/g, '')).filter((entry) => entry !== '' && entry !== 'content')
    expect(derivedUnion.sort()).toEqual([...goBandsCappingVertically(go)].sort())
    // The type-level half of the same claim, which tsc checks and vitest cannot:
    // a Record keyed by the union must carry EXACTLY the union's members, so a
    // member gained or lost on either side stops this file compiling.
    const everyCappingBand: Record<CappingBand, true> = { pageHeader: true, pageFooter: true }
    expect(Object.keys(everyCappingBand).sort()).toEqual([...goBandsCappingVertically(go)].sort())
    // And the consumers hold no copy of their own: they import these two.
    const app = fs.readFileSync(path.join(sourceDir, 'App.tsx'), 'utf8')
    const factory = fs.readFileSync(path.join(sourceDir, 'band-height-command.ts'), 'utf8')
    expect(app).toContain('CAPPING_BANDS')
    expect(app).not.toMatch(/\['pageHeader', 'pageFooter'\]/)
    expect(factory).toMatch(/^import type \{ CappingBand \} from '\.\/engine-protocol'$/m)
    expect(factory).not.toMatch(/'pageHeader' \| 'pageFooter'/)
  })

  it('keeps the HORIZONTAL cap universal in every consumer', () => {
    // The column is unbounded vertically, never horizontally — so neither
    // side may guard its x check by band.
    expect(go).toMatch(/outside := x < 0 \|\| y < 0 \|\| width < 0 \|\| height < 0 \|\| x > geom\.Length\(band\.Width\) \|\| width > geom\.Length\(band\.Width\)-x$/m)
    expect(ts).toMatch(/^ {4}if \(!\(box\.x \+ box\.width <= band\.width\)\) return false$/m)
    // The drag clamp's width limit takes the band's size unconditionally,
    // with no band in the condition at all.
    expect(dragClamp).toMatch(/^ {2}const limitWidth = limit \? limit\.width : Number\.POSITIVE_INFINITY$/m)
  })

  it('turns a one-sided edit of the predicate red', () => {
    const driftedTs = ts.replace(/^export const BANDS_CAPPING_VERTICALLY = \[([^\]]*)\]$/m, "export const BANDS_CAPPING_VERTICALLY = ['pageHeader', 'content', 'pageFooter']")
    expect(driftedTs).not.toBe(ts)
    expect(tsBandsCappingVertically(driftedTs)).not.toEqual(goBandsCappingVertically(go))
    const driftedGo = go.replace(/^var bandsCappingVertically = \[\]string\{([^}]*)\}$/m, 'var bandsCappingVertically = []string{bandPageHeader}')
    expect(driftedGo).not.toBe(go)
    expect(goBandsCappingVertically(driftedGo)).not.toEqual(tsBandsCappingVertically(ts))
    // And the third consumer's own drift: a drag clamp that stopped reading
    // the list and re-stated the rule inline is the shape DW-36 named — "a
    // fourth spelling of the tie" — and it would be invisible to every
    // assertion above, because both declarations would still agree.
    const driftedClamp = dragClamp.replace(/const limitHeight = limit && BANDS_CAPPING_VERTICALLY\.includes\(limit\.band\) \? limit\.height : Number\.POSITIVE_INFINITY/, 'const limitHeight = limit ? limit.height : Number.POSITIVE_INFINITY')
    expect(driftedClamp).not.toBe(dragClamp)
    expect(driftedClamp).not.toMatch(/BANDS_CAPPING_VERTICALLY\.includes\(limit\.band\)/)
  })
})

// STORY 12.5. THE CONTENT-WINDOW CEILING, MIRRORED FOR A GESTURE AND FOR
// NOTHING ELSE.
//
// The band-height PANEL still holds no bound (12.1's D-12.1-Q4, narrowed rather
// than deleted by 12.5's R1): a typed field has 17.4's "consistency with
// typing" to be consistent with, so an unmirrored clamp beside a keystroke that
// sends-and-is-refused would make one box behave two ways. A canvas boundary
// has no typed counterpart, so that property is vacuous there — and the OTHER
// half of 17.4's objection, a quietly-drifting copy of the engine's rule, is
// answered here rather than by prohibition. This describe is what makes the
// copy not quiet.
describe('content-window ceiling mirror', () => {
  const go = fs.readFileSync(goSources.pageSetup, 'utf8')
  const ts = fs.readFileSync(tsPath, 'utf8')
  const boundary = fs.readFileSync(bandBoundaryPath, 'utf8')

  // WRAP-FRAGILE AND LOUD ABOUT IT (D-000.27). Both extractions are
  // line-anchored against one gofmt/prettier spelling, so a reformat that
  // splits either line must produce a RED here rather than a vacuous pass —
  // which is what the non-vacuity row below is for, and why every equality
  // after it reads through these two helpers rather than through raw regexes.
  const goMargin = (source: string): string | undefined =>
    source.match(/^func bandContentWindowCeiling\(other, innerH geom\.Length\) geom\.Length \{\n\treturn innerH - other - (\d+)\n\}$/m)?.[1]
  const tsMargin = (source: string): string | undefined =>
    source.match(/^export const BAND_CONTENT_WINDOW_MARGIN = (\d+)$/m)?.[1]

  it('reads the margin from both sides, or fails rather than passing vacuously', () => {
    expect(goMargin(go), 'page_setup.go no longer declares bandContentWindowCeiling where this test can read it; re-derive the extraction rather than deleting the tie').toMatch(/^\d+$/)
    expect(tsMargin(ts), 'engine-protocol.ts no longer declares BAND_CONTENT_WINDOW_MARGIN on one line').toMatch(/^\d+$/)
  })

  it('holds the engine\'s strict-positivity margin and its mirror at the same number', () => {
    expect(tsMargin(ts)).toBe(goMargin(go))
    // Pinned as a numeral too, so a silent joint edit still has to face the
    // reason recorded in both files: a geom.Length is an integer count of
    // millipoints and the content region must be STRICTLY positive.
    expect(goMargin(go)).toBe('1')
  })

  it('keeps the ceiling the PREDICATE\'s own on the Go side', () => {
    // bandsLeaveContentWindow is written in terms of the ceiling rather than
    // restating the arithmetic, which is what makes one number worth mirroring
    // at all. A second spelling in Go would leave this mirror tied to the
    // spelling nobody checks against.
    expect(go).toMatch(/^\treturn header >= 0 && footer >= 0 && header <= bandContentWindowCeiling\(footer, innerH\)$/m)
  })

  it('consumes the mirror at the one site that bounds a gesture', () => {
    // A constant nothing reads would tie two dead declarations together while
    // the clamp kept an inline literal — the same failure the pairs table's
    // `sites` column exists to prevent.
    expect(boundary).toMatch(/^import \{ BAND_CONTENT_WINDOW_MARGIN, CAPPING_BANDS, type CanvasProjection, type CappingBand \} from '\.\/engine-protocol'$/m)
    expect(boundary).toMatch(/^  return innerH - otherHeight - BAND_CONTENT_WINDOW_MARGIN$/m)
    // AND THE CENSUS STAYS CLOSED: the new module holds no copy of the capping
    // band list either, it reads CAPPING_BANDS.
    expect(boundary).toContain('CAPPING_BANDS')
    expect(boundary).not.toMatch(/\['pageHeader', 'pageFooter'\]/)
  })

  it('turns a one-sided edit of the margin red', () => {
    const driftedTs = ts.replace(/^export const BAND_CONTENT_WINDOW_MARGIN = (\d+)$/m, 'export const BAND_CONTENT_WINDOW_MARGIN = 0')
    expect(driftedTs).not.toBe(ts)
    expect(tsMargin(driftedTs)).not.toBe(goMargin(go))
    const driftedGo = go.replace(/^\treturn innerH - other - (\d+)$/m, '\treturn innerH - other - 2')
    expect(driftedGo).not.toBe(go)
    expect(goMargin(driftedGo)).not.toBe(tsMargin(ts))
    // And the consumer's own drift, which both declarations agreeing would
    // hide: a ceiling that stopped reading the mirror and restated the
    // strictness inline is the shape DW-36 named.
    const driftedBoundary = boundary.replace(/return innerH - otherHeight - BAND_CONTENT_WINDOW_MARGIN/, 'return innerH - otherHeight - 1')
    expect(driftedBoundary).not.toBe(boundary)
    expect(driftedBoundary).not.toMatch(/^  return innerH - otherHeight - BAND_CONTENT_WINDOW_MARGIN$/m)
  })
})

// Go declares the rule as a disjunction inside the property command's length
// arm; TypeScript declares it as a list the inspector reads. Both are resolved
// to a set of KEY NAMES before comparison, because it is the CLAIM — which
// fields the engine refuses at or below zero — that has to match, not the
// spelling.
function goPositiveLengthFields(source: string): ReadonlyArray<string> {
  const clause = source.match(/^\t+if \((key == "\w+"(?: \|\| key == "\w+")*)\) && length <= 0 \{$/m)?.[1]
  if (clause === undefined) return []
  return [...clause.matchAll(/key == "(\w+)"/g)].map((match) => match[1] as string)
}

function tsPositiveLengthFields(source: string): ReadonlyArray<string> {
  const list = source.match(/^export const POSITIVE_LENGTH_FIELDS: ReadonlyArray<PropertyField> = \[([^\]]*)\]$/m)?.[1]
  if (list === undefined) return []
  return list.split(',').map((entry) => entry.trim().replace(/^'|'$/g, '')).filter((entry) => entry.length > 0)
}

// STORY 17.4's MIRROR, and the second one here that ties a PREDICATE.
//
// The invariant: which property keys the engine refuses at or below zero. Go
// enforces it on the COMMAND path; the inspector's ARROW STEP reads it to know
// where to stop, so that a keypress never proposes a value the command path
// will refuse. The asymmetry is what makes a one-sided edit dangerous in BOTH
// directions: adding a key in Go alone would let an arrow step a field into a
// refusal the author reads as a mysteriously rejected nudge, and dropping one
// in Go alone would leave the panel clamping a field the engine no longer
// bounds. `x` and `y` are on NEITHER list, which is the reason the tie is over
// a list at all rather than over "the numeric fields" — their own floor comes
// from `containComponent` instead, tied in the describe below.
describe('positive length rule mirror', () => {
  const go = fs.readFileSync(goSources.componentCommands, 'utf8')
  const ts = fs.readFileSync(path.join(sourceDir, 'component-property-command.ts'), 'utf8')
  const panel = fs.readFileSync(path.join(sourceDir, 'App.tsx'), 'utf8')

  it('reads a non-empty list from every side', () => {
    // Non-vacuity first: a regex that quietly stops matching would make the
    // equality below true and meaningless.
    expect(goPositiveLengthFields(go)).toEqual(['width', 'height', 'fontSize', 'borderWidth'])
    expect(tsPositiveLengthFields(ts)).toEqual(['width', 'height', 'fontSize', 'borderWidth'])
  })

  it('agrees on which property keys must stay positive', () => {
    expect(goPositiveLengthFields(go)).toEqual(tsPositiveLengthFields(ts))
    // x and y are on neither side of THIS rule. That does NOT make them
    // unbounded, which is what an earlier reading concluded: they are bounded
    // below at the band origin by `containComponent`, tied in the describe
    // below. What these four assertions pin is only that x and y are not
    // subject to the strictly tighter `> 0` rule.
    expect(goPositiveLengthFields(go)).not.toContain('x')
    expect(tsPositiveLengthFields(ts)).not.toContain('x')
    expect(goPositiveLengthFields(go)).not.toContain('y')
    expect(tsPositiveLengthFields(ts)).not.toContain('y')
  })

  it('consumes the list at the site it governs, and the line-spacing pair beside it', () => {
    // A list nothing reads would tie a dead declaration to a live Go rule
    // while the step kept its own inline spelling.
    expect(go).toMatch(/if \(key == "width" \|\| key == "height" \|\| key == "fontSize" \|\| key == "borderWidth"\) && length <= 0 \{\n\t+return fmt\.Errorf\("%s must be positive", key\)$/m)
    expect(panel).toMatch(/POSITIVE_LENGTH_FIELDS\.includes\(field\) \? 1 : /)
    // The step's OTHER bound comes from the already-tied line-spacing pair
    // rather than from two fresh literals in the panel.
    expect(panel).toMatch(/field === 'lineSpacing' \? MIN_LINE_SPACING_THOUSANDTHS :/)
    expect(panel).toMatch(/field === 'lineSpacing' \? MAX_LINE_SPACING_THOUSANDTHS :/)
  })

  it('turns a one-sided edit of the rule red', () => {
    const driftedTs = ts.replace(/^export const POSITIVE_LENGTH_FIELDS: ReadonlyArray<PropertyField> = \[([^\]]*)\]$/m, "export const POSITIVE_LENGTH_FIELDS: ReadonlyArray<PropertyField> = ['width', 'height', 'fontSize']")
    expect(driftedTs).not.toBe(ts)
    expect(tsPositiveLengthFields(driftedTs)).not.toEqual(goPositiveLengthFields(go))
    const driftedGo = go.replace(/if \(key == "width" \|\| key == "height" \|\| key == "fontSize" \|\| key == "borderWidth"\) && length <= 0 \{/, 'if (key == "width" || key == "height" || key == "fontSize" || key == "borderWidth" || key == "x") && length <= 0 {')
    expect(driftedGo).not.toBe(go)
    expect(goPositiveLengthFields(driftedGo)).not.toEqual(tsPositiveLengthFields(ts))
  })
})

// THE SECOND BOUND ON THE PROPERTY PATH, added at Story 17.4's review.
//
// `updateComponentPropertiesInPlace` does not stop at the `> 0` rule above: it
// calls `containComponent` on every id after applying the changes
// (`component_commands.go:880`), and that predicate opens by refusing NEGATIVE
// geometry. So `x` and `y` — absent from the `> 0` list, and therefore read as
// "unbounded" when this story was planned — have a floor of ZERO on exactly the
// path an arrow step uses. The panel mirrors that floor through
// `ORIGIN_FLOOR_FIELDS` rather than restating a literal.
//
// ⚠ WHAT THIS TIE DELIBERATELY DOES NOT COVER: the same predicate also bounds
// x, y, width and height ABOVE, against the band extents. The panel does not
// clamp to those, so a step at the band edge still reaches the engine's own
// located refusal. The ceiling is PER-COMPONENT — two components of equal width
// at different x have different width ceilings — so a selection-wide clamp
// needs a ruling this story does not carry. It is recorded as an open question
// in the story's Spec Change Log, and named here so the omission is visible
// rather than inferred from silence.
function goNonNegativeGeometryFields(source: string): ReadonlyArray<string> {
  const clause = source.match(/^\toutside := ((?:\w+ < 0 \|\| )+)/m)?.[1]
  if (clause === undefined) return []
  return [...clause.matchAll(/(\w+) < 0/g)].map((match) => match[1] as string)
}

function tsOriginFloorFields(source: string): ReadonlyArray<string> {
  const list = source.match(/^export const ORIGIN_FLOOR_FIELDS: ReadonlyArray<PropertyField> = \[([^\]]*)\]$/m)?.[1]
  if (list === undefined) return []
  return list.split(',').map((entry) => entry.trim().replace(/^'|'$/g, '')).filter((entry) => entry.length > 0)
}

describe('origin floor mirror', () => {
  const go = fs.readFileSync(goSources.componentCommands, 'utf8')
  const ts = fs.readFileSync(path.join(sourceDir, 'component-property-command.ts'), 'utf8')
  const panel = fs.readFileSync(path.join(sourceDir, 'App.tsx'), 'utf8')

  it('reads the negative-geometry refusal from Go and the floor list from the panel', () => {
    // Non-vacuity: a regex that quietly stopped matching would make every
    // comparison below true and meaningless.
    expect(goNonNegativeGeometryFields(go)).toEqual(['x', 'y', 'width', 'height'])
    expect(tsOriginFloorFields(ts)).toEqual(['x', 'y'])
  })

  it('mirrors exactly the fields whose floor comes from containment alone', () => {
    // Go refuses four fields below zero. Two of them, width and height, are
    // already held to a STRICTLY TIGHTER floor by the `> 0` rule, so the panel
    // clamps them there instead; the remaining two are the origin-floor list.
    // Stated as a partition, so a field moving between the two Go rules cannot
    // leave both TypeScript lists satisfied.
    expect([...tsOriginFloorFields(ts), ...tsPositiveLengthFields(ts)].filter((field) => goNonNegativeGeometryFields(go).includes(field)).sort())
      .toEqual([...goNonNegativeGeometryFields(go)].sort())
    expect(tsOriginFloorFields(ts).filter((field) => tsPositiveLengthFields(ts).includes(field))).toEqual([])
  })

  it('consumes the list at the site it governs', () => {
    expect(panel).toMatch(/ORIGIN_FLOOR_FIELDS\.includes\(field\) \? 0 : undefined$/m)
  })

  it('turns a one-sided edit of the rule red', () => {
    const driftedTs = ts.replace(/^export const ORIGIN_FLOOR_FIELDS: ReadonlyArray<PropertyField> = \[([^\]]*)\]$/m, "export const ORIGIN_FLOOR_FIELDS: ReadonlyArray<PropertyField> = ['x']")
    expect(driftedTs).not.toBe(ts)
    expect(tsOriginFloorFields(driftedTs)).not.toEqual(tsOriginFloorFields(ts))
    const driftedGo = go.replace(/^\toutside := x < 0 \|\| y < 0 \|\| /m, '\toutside := x < 0 || ')
    expect(driftedGo).not.toBe(go)
    expect(goNonNegativeGeometryFields(driftedGo)).not.toEqual(goNonNegativeGeometryFields(go))
  })
})

// Go declares AD-12's closed locale set through named constants, so the
// identifiers are resolved before comparison — exactly as
// goBandsCappingVertically does for the band list. `[]string{LocaleEN, LocaleTH,
// LocaleZhHans, LocaleJA}` and `['en', 'th', 'zh-Hans', 'ja']` are the same
// CLAIM spelled two ways, and it is the claim that has to match, not the text.
function goLocaleTags(source: string): ReadonlyArray<string> {
  const names = new Map<string, string>()
  for (const match of source.matchAll(/^[ \t]*(Locale[A-Za-z]+)\s+= "([^"]+)"$/gm)) names.set(match[1] as string, match[2] as string)
  const list = source.match(/^var LocaleTags = \[\]string\{([^}]*)\}$/m)?.[1]
  if (list === undefined) return []
  return list.split(',').map((entry) => entry.trim()).filter((entry) => entry.length > 0).map((entry) => names.get(entry) ?? entry)
}

function tsLocaleTags(source: string): ReadonlyArray<string> {
  const list = source.match(/^export const LOCALE_TAGS = \[([^\]]*)\] as const$/m)?.[1]
  if (list === undefined) return []
  return list.split(',').map((entry) => entry.trim().replace(/^'|'$/g, '')).filter((entry) => entry.length > 0)
}

// STORY 12.2's MIRROR, and the second here that ties a CLOSED SET rather than a
// numeral.
//
// The invariant: which locale tags a `.folio` document may declare. Go enforces
// it on the FILE path (parse.go, through template.IsLocale) and on the COMMAND
// path (setDocumentLocale, through the same predicate), and TypeScript enforces
// it again on the PROJECTION path (isCanvas) while OFFERING it in the panel.
// The offering is what makes a one-sided edit dangerous in a new way: the two
// existing selects in PAGE SETUP — preset and orientation — hardcode their
// options with no tie to Go at all, and copying that here would have made
// AD-12's four tags a fourth un-tied spelling in the one story whose whole
// subject is a set with one authority.
//
// What a stale copy costs is the same as every other one on this boundary: a
// tag Go projects and this side does not list makes isCanvas return false,
// parseInbound return undefined, and engine-client terminate the worker — the
// canvas permanently blank, with no element id and nothing to attribute it to.
describe('locale tag mirror', () => {
  const go = fs.readFileSync(goSources.locale, 'utf8')
  const ts = fs.readFileSync(tsPath, 'utf8')

  it('reads a non-empty list from every side', () => {
    // Non-vacuity first: a regex that quietly stops matching would make every
    // equality below true and meaningless.
    expect(goLocaleTags(go)).toEqual(['en', 'th', 'zh-Hans', 'ja'])
    expect(tsLocaleTags(ts)).toEqual(['en', 'th', 'zh-Hans', 'ja'])
    // And the RUNTIME array, not only its source text: the guard, the panel and
    // the command factory all read this object.
    expect([...LOCALE_TAGS]).toEqual(tsLocaleTags(ts))
  })

  it('agrees on the closed set a document may declare', () => {
    expect(goLocaleTags(go)).toEqual(tsLocaleTags(ts))
    // ORDER IS PART OF THE CLAIM. Go pins LocaleTags' exact sequence
    // (TestLocaleTagsExactOrder) because the refusal messages are joined from
    // it and the panel lists its options in it; comparing as sets would let the
    // two sides disagree about what the author sees first.
    expect(goLocaleTags(go)).not.toEqual([])
    expect([...LOCALE_TAGS].sort()).toEqual([...goLocaleTags(go)].sort())
  })

  it('consumes the list at the sites it governs, in every consumer', () => {
    // A list nothing reads would tie dead declarations together while the real
    // gates kept their own inline spellings.
    expect(go).toMatch(/^func IsLocale\(s string\) bool \{ return closedLocales\[s\] \}$/m)
    expect(ts).toMatch(/if \(!LOCALE_TAGS\.includes\(value\.locale as LocaleTag\)\) return false/)
    expect(ts).toMatch(/^export type LocaleTag = \(typeof LOCALE_TAGS\)\[number\]$/m)
  })

  it('lets no consumer hold a copy of its own', () => {
    // THE CENSUS IS THE WHOLE OF src/, NOT A LIST OF FILES SOMEONE REMEMBERED.
    // It used to grep exactly two files, App.tsx and the command factory, while
    // the comment above claimed "everything on this side reads this array" and
    // "a copy outside that census is the only kind that can go stale
    // unnoticed" — so the one case it worried about, a NEW file spelling a tag
    // for itself, was precisely the case it could not see. A named-file list is
    // the defect; a directory walk is the fix.
    //
    // `zh-Hans` is the probe because it is the one tag nothing in this codebase
    // spells for another reason — `en`, `th` and `ja` appear in fixtures, font
    // names, language attributes and ordinary English, and would make this
    // assertion fire on text that is not a second copy of the set.
    //
    // engine-protocol.ts is the ONE exemption, because it is the authority.
    // This file is exempt too: the assertions here quote the expected list, and
    // a guard that reddened on its own expectation would be unwritable.
    const scanned = fs.readdirSync(sourceDir, { recursive: true })
      .filter((entry): entry is string => typeof entry === 'string' && /\.(ts|tsx)$/.test(entry))
      .filter((entry) => entry !== 'engine-protocol.ts' && entry !== 'engine-bounds-mirror.test.ts')
    // Non-vacuity: a walk that returned nothing, or a handful, would make the
    // loop below pass while looking like a census. The tree carried well over a
    // hundred .ts/.tsx files when this was written.
    expect(scanned.length).toBeGreaterThan(80)
    expect(scanned).toContain('App.tsx')
    expect(scanned).toContain('document-settings-command.ts')
    const holders = scanned.filter((entry) => /'zh-Hans'/.test(fs.readFileSync(path.join(sourceDir, entry), 'utf8')))
    expect(holders).toEqual([])
    // And the two consumers read the authority rather than merely not spelling
    // it — "holds no copy" is satisfied vacuously by a file that uses no tags
    // at all, so the positive half is asserted too.
    const app = fs.readFileSync(path.join(sourceDir, 'App.tsx'), 'utf8')
    const factory = fs.readFileSync(path.join(sourceDir, 'document-settings-command.ts'), 'utf8')
    expect(app).toContain('LOCALE_TAGS')
    expect(factory).toMatch(/^import type \{ LocaleTag \} from '\.\/engine-protocol'$/m)
    // And the Go side holds exactly one spelling too: the tag literal appears
    // only as the right-hand side of its constant, never in closedLocales or
    // LocaleTags, which are both built from the constants. Null-safe, so a
    // literal that has been RENAMED reports what is missing instead of
    // "expected null to have length 1".
    expect(go.match(/"zh-Hans"/g) ?? []).toEqual(['"zh-Hans"'])
  })

  it('turns a one-sided edit of the set red', () => {
    const driftedGo = go.replace(/^\tLocaleJA     = "ja"$/m, '\tLocaleJA     = "ja-JP"')
    expect(driftedGo).not.toBe(go)
    expect(goLocaleTags(driftedGo)).not.toEqual(tsLocaleTags(ts))
    const driftedTs = ts.replace(/^export const LOCALE_TAGS = \[([^\]]*)\] as const$/m, "export const LOCALE_TAGS = ['en', 'th', 'zh-Hans'] as const")
    expect(driftedTs).not.toBe(ts)
    expect(tsLocaleTags(driftedTs)).not.toEqual(goLocaleTags(go))
  })
})

// Go declares the gate against a NAMED CONSTANT, so the identifier is resolved
// through its own declaration before comparison: `template.ElementText` and
// `'text'` are the same claim spelled two ways, and it is the CLAIM that has to
// match. `internal/template/model.go` declares the closed five; a rename there
// alone must not be able to make this tie pass while meaning something else.
function goElementTypes(model: string): ReadonlyMap<string, string> {
  const names = new Map<string, string>()
  for (const match of model.matchAll(/^\t(Element[A-Za-z]+)\s+ElementType = "([^"]+)"$/gm)) names.set(match[1] as string, match[2] as string)
  return names
}

// WRAP-FRAGILE AND LOUD ABOUT IT (D-000.27). The gate is matched together with
// the refusal text it emits — repo-wide grep puts that string at exactly one
// site — so a reformat, a moved gate or a reworded refusal produces a RED here
// rather than a vacuous pass. That is what the non-vacuity `it` below is for.
function goScalarBindingTypes(commands: string, model: string): ReadonlyArray<string> {
  const gate = commands.match(/^\tif ((?:element\.Type != template\.Element[A-Za-z]+(?: && )?)+) \{\n\t\treturn designer\.CanvasProjection\{\}, componentFailure\(id, "component\.id", "only [a-z, ]+ components can receive a scalar binding"\)$/m)?.[1]
  if (gate === undefined) return []
  const types = goElementTypes(model)
  const resolved = [...gate.matchAll(/template\.(Element[A-Za-z]+)/g)].map((match) => types.get(match[1] as string))
  return resolved.some((kind) => kind === undefined) ? [] : resolved as string[]
}

function tsScalarBindingTypes(source: string): ReadonlyArray<string> {
  const list = source.match(/^export const SCALAR_BINDING_COMPONENT_TYPES: ReadonlyArray<CanvasComponentType> = \[([^\]]*)\]$/m)?.[1]
  if (list === undefined) return []
  return list.split(',').map((entry) => entry.trim().replace(/^'|'$/g, '')).filter((entry) => entry.length > 0)
}

// STORY 14.4's MIRROR, and the THIRD here that ties a PREDICATE.
//
// The invariant: which component kinds may receive a SCALAR binding. Go
// enforces it on the COMMAND path (`bindComponentScalar`, after `findComponent`
// and before any mutation); TypeScript enforces it again on the PROJECTION path
// (`isCanvas`), and — since this story — states it a third time in the DATA
// PANEL, before the command is sent at all.
//
// ⚠ THIS TIE DOES NOT REGISTER A NEW COPY; IT REGISTERS ONE THAT WAS ALREADY
// SHIPPED. `engine-protocol.ts` has re-derived Go's rule inline since it was
// written, untied and outside the mirror census, in the harshest failure mode
// on this boundary: a one-sided Go edit makes `isCanvas` return false,
// `parseInbound` return undefined and the canvas go permanently blank — not a
// wrong tooltip, a dead editor. So the choice this story faced was never "one
// copy or two"; it was "two copies untied, or one constant tied". The pre-flight
// was gated on this describe existing, not the other way round.
//
// ⚠ WHAT IS TIED IS THE COMPONENT-TYPE GATE AND NOTHING ELSE. Whether a picked
// PATH yields a scalar at render is runtime data decided by `internal/bind`, and
// D-6.2.1 deliberately keeps sample runtime kind out of command legality
// (`component_commands_test.go`: `segments:["items"]` on an empty collection is
// ACCEPTED and canonicalised). Nothing here may grow into "is this path
// bindable" without crossing that authority boundary.
describe('scalar binding legality mirror', () => {
  const go = fs.readFileSync(goSources.componentCommands, 'utf8')
  const model = fs.readFileSync(goSources.templateModel, 'utf8')
  const ts = fs.readFileSync(tsPath, 'utf8')
  const panel = fs.readFileSync(path.join(sourceDir, 'DataPanel.tsx'), 'utf8')
  const app = fs.readFileSync(path.join(sourceDir, 'App.tsx'), 'utf8')

  it('reads the gate from Go and the list from the browser side, or fails rather than passing vacuously', () => {
    // Non-vacuity FIRST, as a separate row, because an extraction that quietly
    // stopped matching would make every equality below true and meaningless.
    // The five element kinds are asserted whole: a rename in model.go alone
    // must report what moved rather than silently resolving to nothing.
    // P10. MEMBERSHIP, NOT AN ORDERED WHOLE. Asserting the exact five-element
    // list in order would red on a legitimate gofmt reorder or on a SIXTH
    // element kind, with a message claiming the extraction had broken — false
    // in both cases. `toMatchObject` admits extra kinds and ignores order;
    // what it still refuses is a RENAMED or missing `ElementText`, which is
    // the only change that can silently unmake this tie.
    expect(Object.fromEntries(goElementTypes(model)), 'internal/template/model.go no longer declares the ElementType constants this tie reads — ElementText above all — in a spelling this extractor understands; RE-DERIVE the extraction rather than deleting the tie. A REORDERED block or a NEW eighth element kind is not a failure of this row; only a changed SPELLING is.').toMatchObject({ ElementText: 'text', ElementImage: 'image', ElementTable: 'table', ElementLine: 'line', ElementRect: 'rect', ElementBarcode: 'barcode', ElementQRCode: 'qrcode' })
    expect(goScalarBindingTypes(go, model), 'component_commands.go no longer spells bindComponentScalar\'s type gate — or its refusal text — where this test can read it; RE-DERIVE the extraction rather than deleting the tie, because the pre-flight in DataPanel.tsx is only legal while this tie holds').toEqual(['text', 'barcode', 'qrcode'])
    expect(tsScalarBindingTypes(ts), 'engine-protocol.ts no longer declares SCALAR_BINDING_COMPONENT_TYPES on one line in the spelling this extractor reads').toEqual(['text', 'barcode', 'qrcode'])
    // And the RUNTIME array, not only its source text: the projection guard,
    // the inspector and the data panel all read this object.
    expect([...SCALAR_BINDING_COMPONENT_TYPES]).toEqual(tsScalarBindingTypes(ts))
  })

  it('agrees on which component kinds may receive a scalar binding', () => {
    expect(tsScalarBindingTypes(ts)).toEqual(goScalarBindingTypes(go, model))
    // THE NEGATIVE HALF, and it is not decoration: `table` is the kind most
    // likely to be added here by mistake, because a Table legally takes a
    // binding through `configureTableBinding` / `updateTableColumnBinding` —
    // a DIFFERENT value and a DIFFERENT command. A constant named for
    // bindability in general would have been wrong on exactly that row.
    for (const kind of ['table', 'line', 'rect', 'image']) {
      expect(goScalarBindingTypes(go, model)).not.toContain(kind)
      expect(tsScalarBindingTypes(ts)).not.toContain(kind)
    }
  })

  it('consumes the list at every site that judges a component kind', () => {
    // A constant nothing reads would tie a dead declaration to a live Go rule
    // while each judgement kept its own inline spelling — the failure the
    // pairs table's `sites` column exists to prevent, applied to a predicate.
    //
    // 1. THE PROJECTION GUARD (shipped long before this story, registered by it).
    expect(ts).toMatch(/if \(!SCALAR_BINDING_COMPONENT_TYPES\.includes\(component\.type as CanvasComponentType\) && component\.binding !== undefined\) return false/)
    // 2. THE DATA PANEL'S PRE-FLIGHT, the site this story added. Matched as the
    //    whole gate EXPRESSION rather than as a mention of the name, so a panel
    //    that kept the import and re-spelled the test inline still reds.
    expect(panel).toMatch(/^import \{ SCALAR_BINDING_COMPONENT_TYPES, type CanvasComponentType \} from '\.\/engine-protocol'$/m)
    expect(panel).toMatch(/const bindableKind = selectedComponentType !== undefined && SCALAR_BINDING_COMPONENT_TYPES\.includes\(selectedComponentType\)/)
    // 3. THE INSPECTOR'S BINDING SECTION, gated by the same array.
    expect(app).toMatch(/const scalarBindable = single !== undefined && SCALAR_BINDING_COMPONENT_TYPES\.includes\(single\.type\)/)
    // AND NO CONSUMER HOLDS A COPY OF ITS OWN. The drift this story invites is
    // a kind test re-spelled as a bare comparison against the literal, which
    // both declarations agreeing would hide completely.
    expect(panel).not.toMatch(/[=!]==\s*'text'/)
    // P6. `single?.type === 'line'` / `'image'` / `'table'` is how App.tsx
    // spells every OTHER kind test, so the optional-chained form is the most
    // likely re-spelling and the pattern must reach it. The earlier
    // `single\.type` version did not.
    expect(app).not.toMatch(/single\??\.type [=!]==\s*'text'/)
  })

  it('turns a one-sided edit of the gate red', () => {
    // FROM GO, by widening it — the edit that would make the panel refuse a
    // pick the engine has started accepting.
    const widenedGo = go.replace(/^\tif element\.Type != template\.ElementText && element\.Type != template\.ElementBarcode && element\.Type != template\.ElementQRCode \{$/m, '\tif element.Type != template.ElementText && element.Type != template.ElementBarcode && element.Type != template.ElementImage {')
    expect(widenedGo).not.toBe(go)
    expect(goScalarBindingTypes(widenedGo, model)).not.toEqual(tsScalarBindingTypes(ts))
    // FROM GO, by deleting it — which must red the NON-VACUITY row, not the
    // agreement one, so the maintainer is told the extraction stopped reading
    // rather than that the two sides disagree.
    const deletedGo = go.replace(/^\tif element\.Type != template\.ElementText && element\.Type != template\.ElementBarcode && element\.Type != template\.ElementQRCode \{\n\t\treturn designer\.CanvasProjection\{\}, componentFailure\(id, "component\.id", "only text, barcode and qrcode components can receive a scalar binding"\)\n\t\}\n/m, '')
    expect(deletedGo).not.toBe(go)
    expect(goScalarBindingTypes(deletedGo, model)).toEqual([])
    // FROM GO, by renaming the kind constant out from under the gate: the
    // resolution through model.go is what catches this, and it too must land
    // on the non-vacuity row.
    const renamedModel = model.replace(/^\tElementText(\s+)ElementType = "text"$/m, '\tElementProse$1ElementType = "text"')
    expect(renamedModel).not.toBe(model)
    expect(goScalarBindingTypes(go, renamedModel)).toEqual([])
    // FROM TYPESCRIPT.
    const driftedTs = ts.replace(/^export const SCALAR_BINDING_COMPONENT_TYPES: ReadonlyArray<CanvasComponentType> = \[([^\]]*)\]$/m, "export const SCALAR_BINDING_COMPONENT_TYPES: ReadonlyArray<CanvasComponentType> = ['text', 'table']")
    expect(driftedTs).not.toBe(ts)
    expect(tsScalarBindingTypes(driftedTs)).not.toEqual(goScalarBindingTypes(go, model))
    // AND FROM THE CONSUMER — the drift this story specifically invites, which
    // BOTH DECLARATIONS AGREEING WOULD HIDE: a panel that stops reading the
    // constant and re-spells the rule inline, leaving it hoisted but unused.
    // If this stayed green the mirror would be decorative.
    const driftedPanel = panel.replace(/const bindableKind = selectedComponentType !== undefined && SCALAR_BINDING_COMPONENT_TYPES\.includes\(selectedComponentType\)/, "const bindableKind = selectedComponentType === 'text'")
    expect(driftedPanel).not.toBe(panel)
    expect(driftedPanel).not.toMatch(/const bindableKind = selectedComponentType !== undefined && SCALAR_BINDING_COMPONENT_TYPES\.includes\(selectedComponentType\)/)
    expect(driftedPanel).toMatch(/[=!]==\s*'text'/)
  })
})

// STORY 14.7b's MIRROR: HOW DEEP THE ENGINE'S UNDO HISTORY IS.
//
// The invariant: `historyLimit`. It is the FOURTH kind of thing this file ties —
// not a numeral both sides validate against, not a list, not a term lifted out
// of an expression, but a CAPACITY the browser has to reason about because the
// table editor's Cancel is a compensating sequence of N `undo` operations rather
// than a transaction.
//
// ⚠ AND THE ASYMMETRY IS WHAT MAKES A ONE-SIDED EDIT DANGEROUS HERE. Go does
// not REFUSE at the limit — `appendBounded` shifts the stack down and drops the
// oldest entry, returning no error and putting nothing on the wire — so an
// over-run is SILENT. If Go's limit were lowered without lowering this mirror,
// the footer would keep offering Cancel for a count the engine can no longer
// reach, the sequence would land part-way, and the dialog would have closed
// claiming a discard it did not complete. Nothing in the running app would say
// so; only this test stands between the two numbers.
//
// The bound is consumed at the two sites that REFUSE, which is DW-36's standing
// condition restated: a browser-side bound must consume the engine's
// declaration and be caught doing so.
describe('engine history limit mirror', () => {
  const go = fs.readFileSync(goSources.history, 'utf8')
  const ts = fs.readFileSync(tsPath, 'utf8')
  const dialog = fs.readFileSync(path.join(sourceDir, 'TableEditor.tsx'), 'utf8')
  const app = fs.readFileSync(path.join(sourceDir, 'App.tsx'), 'utf8')

  it('reads a declared historyLimit from folio-go/internal/wasm/engine.go at all', () => {
    // NON-VACUITY, AND IT IS DELIBERATELY BLIND TO THE NUMBER. A RENAME of the
    // Go constant would make `goConstant` return `undefined`, and an equality
    // between two `undefined`s is a tie that compares nothing while passing. So
    // the DECLARATION is asserted to exist, by its own spelling, before any
    // number is read off it.
    expect(go).toMatch(/^const historyLimit = \d+$/m)
    expect(goConstant(go, 'historyLimit')).toMatch(/^\d+$/)
    expect(tsConstant(ts, 'MAX_ENGINE_HISTORY_ENTRIES')).toMatch(/^\d+$/)
    // AND THE ENFORCEMENT IS STILL THE RING BUFFER, which is the half of this
    // invariant that is not a number: a refusal instead of an eviction would
    // make the mirror unnecessary, and an eviction that stopped reading
    // `historyLimit` would make it a dead declaration.
    expect(go).toMatch(/^\tif len\(history\) == historyLimit \{$/m)
    expect(go).toMatch(/^\t\tcopy\(history, history\[1:\]\)$/m)
  })

  it('holds the Go history depth and its TypeScript mirror at the same number', () => {
    expect(goConstant(go, 'historyLimit')).toBe(tsConstant(ts, 'MAX_ENGINE_HISTORY_ENTRIES'))
    // Pinned as a numeral for the reason the derivations above are: a silent
    // joint edit of both files still has to face the number recorded here.
    expect(goConstant(go, 'historyLimit')).toBe('100')
  })

  it('consumes the mirror at both sites that refuse a discard above it', () => {
    // THE FOOTER'S DISABLE. This is the site the author sees, and it is where a
    // stale inline literal would be invisible.
    expect(dialog).toMatch(/^import \{ MAX_ENGINE_HISTORY_ENTRIES, type TableColumns \} from '\.\/engine-protocol'$/m)
    expect(dialog).toMatch(/^ {2}const overHistoryBound = editCount > MAX_ENGINE_HISTORY_ENTRIES$/m)
    // `fileBusy` JOINED THIS EXPRESSION, and the tie reads the whole of it
    // rather than the bound alone: `cancelTableEditor` refuses while a save or an
    // export is in flight, and a button that omitted the flag looked available
    // during one and swallowed the click.
    expect(dialog).toMatch(/disabled=\{busy \|\| fileBusy \|\| overHistoryBound\}/)
    // AND THE LOOP'S OWN REFUSAL, which must not depend on a disabled button: a
    // greyed control is a UI fact, and the compensating sequence is not allowed
    // to trust one.
    expect(app).toMatch(/^ {4}if \(count > MAX_ENGINE_HISTORY_ENTRIES\) return$/m)
  })

  it('reds on a one-sided number edit AND on a deleted consumption', () => {
    // The number, from each side in turn.
    const driftedTs = ts.replace(/^export const MAX_ENGINE_HISTORY_ENTRIES = (\d+)$/m, 'export const MAX_ENGINE_HISTORY_ENTRIES = 7')
    expect(driftedTs).not.toBe(ts)
    expect(tsConstant(driftedTs, 'MAX_ENGINE_HISTORY_ENTRIES')).not.toBe(goConstant(go, 'historyLimit'))
    const driftedGo = go.replace(/^const historyLimit = (\d+)$/m, 'const historyLimit = 7')
    expect(driftedGo).not.toBe(go)
    expect(goConstant(driftedGo, 'historyLimit')).not.toBe(tsConstant(ts, 'MAX_ENGINE_HISTORY_ENTRIES'))
    // The RENAME, which the non-vacuity clause above is the one that catches.
    const renamed = go.replace(/^const historyLimit = (\d+)$/m, 'const undoDepth = $1')
    expect(renamed).not.toBe(go)
    expect(goConstant(renamed, 'historyLimit')).toBeUndefined()
    expect(renamed).not.toMatch(/^const historyLimit = \d+$/m)
    // ⚠ AND THE ONE THIS FILE'S OWN COMMENT SAYS THE `sites` COLUMN EXISTS FOR:
    // deleting the CONSUMPTION while both numbers still agree. A constant that
    // matches Go and is read by nothing bounds nothing, and the tie would pass
    // while Cancel offered a discard the engine cannot deliver.
    const inlined = dialog.replace('editCount > MAX_ENGINE_HISTORY_ENTRIES', 'editCount > 100')
    expect(inlined).not.toBe(dialog)
    expect(inlined).not.toMatch(/^ {2}const overHistoryBound = editCount > MAX_ENGINE_HISTORY_ENTRIES$/m)
    const unguarded = app.replace('if (count > MAX_ENGINE_HISTORY_ENTRIES) return', 'if (count > 100) return')
    expect(unguarded).not.toBe(app)
    expect(unguarded).not.toMatch(/^ {4}if \(count > MAX_ENGINE_HISTORY_ENTRIES\) return$/m)
  })
})
