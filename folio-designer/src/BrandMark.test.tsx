import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { BrandMark } from './BrandMark'

// The house convention for a test that reads its own source tree — twenty-odd
// sibling suites spell it exactly this way (`design-contract.test.ts`,
// `canvas-authority-contract.test.ts`, `command-json-soleness.test.ts`, …).
const sourceDir = path.dirname(fileURLToPath(import.meta.url))
const brandMarkPath = path.join(sourceDir, 'BrandMark.tsx')
const appCssPath = path.join(sourceDir, 'App.css')
const tokensCssPath = path.join(sourceDir, 'tokens.css')

// Every production `.ts`/`.tsx` under `src`, excluding test files. The scans
// below are claims about the PRODUCTION corpus: a hand-written second copy of
// the mark, or a third call site, is what they have to be able to see.
const productionSources = (): ReadonlyArray<string> =>
  (fs.readdirSync(sourceDir, { recursive: true }) as ReadonlyArray<string>)
    .filter((entry) => /\.tsx?$/.test(entry) && !/\.test\.tsx?$/.test(entry) && !/(^|\/)__/.test(entry))
    .map((entry) => path.join(sourceDir, entry))
    .filter((file) => fs.statSync(file).isFile())

// ⚠ COMMENTS ARE STRIPPED BEFORE EVERY SOURCE SCAN BELOW, and that is the whole
// point rather than a detail. DW-358 records that `design-contract.test.ts:87`
// reads raw text, so a hex inside a CSS comment reds it — and naming a token's
// value in a comment is this codebase's prevailing style, so that guard is a
// tripwire under its own house style. Reproducing the defect in a new guard
// would be writing the registered finding a second time.
//
// The `[^:]` guard in the line-comment arm keeps `https://` out of the jaws of
// the stripper. It is a lexer approximation, not a parser: a `//` inside a
// string literal that is not preceded by `:` would still be cut. Every scan
// that uses it is paired with a control asserting the CODE survived stripping,
// so an over-eager strip cannot quietly empty the corpus and pass.
const stripComments = (source: string) => source
  .replace(/\/\*[\s\S]*?\*\//g, '')
  .replace(/(^|[^:"'`\\])\/\/[^\n]*/gm, '$1')

// THE EXPECTATIONS BELOW ARE LITERALS, NEVER RECOMPUTED FROM THE COMPONENT'S OWN
// FORMULA. A derived expectation is vacuous: it would agree with whatever the
// component happened to compute, including a lookup table of the mockup's
// integers, which is precisely the mutation AC3 exists to forbid.
const marks = (size: 18 | 22) => {
  const { container } = render(<BrandMark size={size} />)
  const svg = container.querySelector('svg')
  const rects = container.querySelectorAll('rect')
  if (!svg) throw new Error('BrandMark rendered no <svg>')
  return { svg, outer: rects[0], inner: rects[1], count: rects.length }
}

describe('the product wears its own mark, drawn from resources/logo.png', () => {
  it('draws the document bar mark at 18 from the logo\'s own ratios', () => {
    const { svg, outer, inner, count } = marks(18)
    expect(count, 'exactly two rects — the outline and the block').toBe(2)
    expect(svg).toHaveAttribute('width', '18')
    expect(svg).toHaveAttribute('height', '18')
    expect(svg).toHaveAttribute('viewBox', '0 0 18 18')
    // 18 × 80/784 = 1.837 stroke, inset by half of it.
    expect(outer).toHaveAttribute('x', '0.918')
    expect(outer).toHaveAttribute('y', '0.918')
    expect(outer).toHaveAttribute('width', '16.163')
    expect(outer).toHaveAttribute('height', '16.163')
    // 18 × 244/784, 208/784, 296/784, 369/784.
    expect(inner).toHaveAttribute('x', '5.602')
    expect(inner).toHaveAttribute('y', '4.776')
    expect(inner).toHaveAttribute('width', '6.796')
    expect(inner).toHaveAttribute('height', '8.472')
  })

  it('draws the load screen mark at 22 from the same rule, scaled', () => {
    const { svg, outer, inner, count } = marks(22)
    expect(count).toBe(2)
    expect(svg).toHaveAttribute('width', '22')
    expect(svg).toHaveAttribute('height', '22')
    expect(svg).toHaveAttribute('viewBox', '0 0 22 22')
    expect(outer).toHaveAttribute('x', '1.122')
    expect(outer).toHaveAttribute('y', '1.122')
    expect(outer).toHaveAttribute('width', '19.755')
    expect(outer).toHaveAttribute('height', '19.755')
    expect(inner).toHaveAttribute('x', '6.847')
    expect(inner).toHaveAttribute('y', '5.837')
    expect(inner).toHaveAttribute('width', '8.306')
    expect(inner).toHaveAttribute('height', '10.355')
  })

  // THE REVERSAL OF STORY 14.5's CONSTANT STROKE, ASSERTED AS A DIFFERENCE
  // RATHER THAN AS TWO NUMBERS. The logo draws the stroke as a fixed fraction of
  // the box (80/784), so the two sizes MUST disagree. Pinning 1.837 and 2.245
  // alone would stay green if someone reintroduced a per-size lookup table with
  // those two values in it; asserting that the ratio is preserved is what says
  // one rule produced both.
  it('scales the stroke with the box, as the logo draws it', () => {
    expect(marks(18).outer).toHaveAttribute('stroke-width', '1.837')
    expect(marks(22).outer).toHaveAttribute('stroke-width', '2.245')
    const ratio = (size: 18 | 22) => Number(marks(size).outer.getAttribute('stroke-width')) / size
    expect(ratio(18), 'the stroke is 80/784 of the box at every size').toBeCloseTo(80 / 784, 4)
    expect(ratio(22)).toBeCloseTo(80 / 784, 4)
    expect(marks(18).outer.getAttribute('stroke-width'), 'a stroke that did not scale would be the mockup rule, not the logo').not.toBe(marks(22).outer.getAttribute('stroke-width'))
  })

  // THE COMPONENT'S CONSTANTS ARE THE ASSET'S OWN PIXELS, asserted against the
  // numbers this suite independently writes above. If someone re-measures the
  // logo and changes MARK/STROKE/INNER_*, these literals must be re-derived with
  // them — which is the point: the asset is the authority, and a silent edit to
  // the constants is a silent change of brand.
  it('keeps the logo\'s measured integers as the component\'s constants', () => {
    const source = fs.readFileSync(brandMarkPath, 'utf8')
    for (const [name, value] of [['MARK', 784], ['STROKE', 80], ['INNER_WIDTH', 296], ['INNER_HEIGHT', 369], ['INNER_X', 244], ['INNER_Y', 208]] as const) {
      expect(stripComments(source), `${name} is measured from resources/logo.png`).toMatch(new RegExp(`const ${name} = ${value}\\b`))
    }
  })

  // THE MIDDLE LINK OF THE COLOUR CHAIN. The chain is
  // `--color-brand` → `.brand-mark { color: … }` → `currentColor`, and this
  // class attribute is the join. Without this assertion the attribute can be
  // deleted and the whole unit suite stays green while the mark silently
  // inherits whatever colour its container happens to carry.
  it('carries the class that is the only join between the token and currentColor', () => {
    expect(marks(18).svg.getAttribute('class'), 'the .brand-mark rule reaches the SVG through this attribute and no other route').toBe('brand-mark')
    expect(marks(22).svg.getAttribute('class')).toBe('brand-mark')
  })

  it('takes its colour only as currentColor, at both sizes', () => {
    for (const size of [18, 22] as const) {
      const { svg, outer, inner } = marks(size)
      expect(svg).toHaveAttribute('fill', 'none')
      expect(outer).toHaveAttribute('fill', 'none')
      expect(outer).toHaveAttribute('stroke', 'currentColor')
      expect(inner).toHaveAttribute('fill', 'currentColor')
      expect(inner).not.toHaveAttribute('stroke')
    }
  })

  it('contributes nothing to the accessible tree', () => {
    const { svg } = marks(18)
    expect(svg).toHaveAttribute('aria-hidden', 'true')
    expect(svg).not.toHaveAttribute('role')
    expect(svg).not.toHaveAttribute('aria-label')
    expect(svg).not.toHaveAttribute('aria-labelledby')
    expect(svg.querySelector('title')).toBeNull()
  })

  // AC2's `BrandMark.tsx` half. This is a CONSTRUCTED absence claim — nothing in
  // the tree reads a `.tsx` file for colour literals today — so it is written
  // with controls in BOTH directions, because a scan that only ever reddens has
  // not been shown to discriminate.
  it('writes no colour literal in the component, and the scan that says so discriminates', () => {
    const source = fs.readFileSync(brandMarkPath, 'utf8')
    const literal = /#[0-9a-fA-F]{3,8}|\b(?:rgba?|hsla?)\(/
    const scan = (text: string) => literal.test(stripComments(text))

    // ANTI-VACUITY FOR THE STRIPPER ITSELF: stripping must remove commentary
    // and leave the code. If it emptied the file every assertion below would
    // pass over nothing.
    expect(stripComments(source), 'the stripper must leave the code it is meant to scan').toContain('stroke="currentColor"')
    expect(stripComments(source), 'the stripper must actually remove commentary').not.toContain('THE GEOMETRY RULE')

    // NEGATIVE CONTROL — the file as shipped.
    expect(scan(source), 'BrandMark.tsx must spell no colour literal; the token reaches it as currentColor').toBe(false)

    // THE TWO-WAY CONTROL ON ONE LITERAL. The SAME `#87F0FF` must be clean in a
    // comment and red in an attribute — a scan that cannot tell those apart is
    // DW-358 rewritten, which is the defect this story criticises.
    expect(scan(`${source}\n// the token is #87F0FF`), 'a hex NAMED IN A COMMENT is documentation, not a colour literal').toBe(false)
    expect(scan(`${source}\n/* the token is #87F0FF */`), 'a hex in a block comment is documentation too').toBe(false)
    expect(scan(source.replace('stroke="currentColor"', 'stroke="#87F0FF"')), 'the same hex in an ATTRIBUTE is the violation, and must red').toBe(true)

    // POSITIVE CONTROLS — the scan fires on every spelling it claims to catch,
    // including the `rgba(`/`hsla(` pair the incumbent App.css regex cannot
    // match (DW-358). Injected as code, not as a comment.
    for (const injected of ['#87F0FF', '#87f0ff', '#fff', 'rgb(', 'rgba(', 'hsl(', 'hsla(']) {
      expect(scan(`${source}\nconst injected = "${injected}"`), `the scan must fire on ${injected} in code, or it is vacuous`).toBe(true)
    }
  })

  // AC3's structural half. One drawing means one place the mark is drawn — and
  // the scan must survive a copy spelled with a different element.
  //
  // ⚠ IT DOES NOT KEY ON `<rect`. A second drawing built from `<path>`,
  // `<polygon>` or two bordered `<div>`s is exactly what AC3 forbids and would
  // walk straight past a `<rect`-only scan. Two arms instead:
  //
  //  (1) THE MARK'S OWN GEOMETRY IN AN ATTRIBUTE POSITION — the half-stroke
  //      inset and the two outer sizes the rule produces, plus the 22px inner
  //      block's three-decimal values. Any copy of THIS mark has to reproduce
  //      these numbers whatever element carries them. Attribute position, not
  //      bare text: the mockup rule's `16.5` also read as "Story 16.5", which
  //      appears in prose across twenty files in this codebase (measured), so a
  //      bare-number scan was a permanent false alarm. The logo's own values
  //      collide with no story number, but the attribute anchor is KEPT — the
  //      hazard was the technique, not the one number that exposed it, and the
  //      prose control below still proves the anchor is doing the work.
  //  (2) THE TWO ELEMENTS THAT DRAW A FILLED SQUARE BLOCK. The house icon set is
  //      `<path>`/`<circle>` only (`paletteGlyphs` in App.tsx), so a `<rect>` or
  //      `<polygon>` appearing anywhere else in production is square-shaped news.
  //
  // Both arms were measured against the whole production corpus and match
  // BrandMark.tsx alone, so neither is carrying a pre-existing false positive.
  it('is the only place the mark is drawn, whatever element a copy might use', () => {
    const geometryInMarkup = /\b(?:x|y|cx|cy|width|height|points|d)\s*=\s*["{]\s*"?\s*(?:0\.918|16\.163|1\.122|19\.755|6\.796|8\.472|10\.355)\b/
    const squareElement = /<(?:rect|polygon)\b/
    const drawn = productionSources().filter((file) => {
      const code = stripComments(fs.readFileSync(file, 'utf8'))
      return geometryInMarkup.test(code) || squareElement.test(code)
    })
    expect(drawn.map((file) => path.relative(sourceDir, file)), 'a second drawing of the mark in production means it is two drawings, not one rule').toEqual(['BrandMark.tsx'])
    // NON-VACUITY, IN BOTH DIRECTIONS. The arms fire on a hand-written copy
    // spelled with any of the three plausible elements …
    expect(geometryInMarkup.test('<rect x="0.918" y="0.918" width="19.755" height="19.755" />')).toBe(true)
    expect(geometryInMarkup.test('<path d="6.796 8.472 L0 0" />'), 'a copy drawn as a path is still a copy').toBe(true)
    expect(squareElement.test('<polygon points="0,0 18,0 18,18 0,18" />')).toBe(true)
    // … and stay quiet on the house icon set and on prose, which is what keeps
    // this guard from becoming a standing false alarm.
    expect(geometryInMarkup.test('<path d="M3.5 4.5V3h9v1.5" />'), 'a paletteGlyphs path is not a copy of the mark').toBe(false)
    expect(geometryInMarkup.test('// STORY 16.5 changed the verb'), 'prose naming Story 16.5 is not geometry').toBe(false)
    expect(squareElement.test('<circle cx="10.5" cy="6.25" r="1" />'), 'the house glyphs use path and circle, which stay permitted').toBe(false)
  })

  // AC3's OTHER half, and the constraint the spec argues hardest for: TWO SIZES,
  // NOWHERE ELSE (`DESIGN.md:436-438`). `size` is typed `number`, so the type
  // system permits a third call site at any value — including 13 on a manifest
  // row, which the Boundaries section forbids by name because the mockup's 13px
  // shape there is the CJK row's in-progress marker, not a brand instance.
  // Putting the mark there would make the product's brand read as "loading".
  // Nothing else in the tree can see that, so this scan is its only fence.
  it('is used at exactly two sites, at the two declared sizes, and nowhere else', () => {
    const sites = productionSources().flatMap((file) => {
      const code = stripComments(fs.readFileSync(file, 'utf8'))
      return [...code.matchAll(/<BrandMark\b[^/>]*?\bsize=\{(\d+)\}/g)].map((match) => `${path.relative(sourceDir, file)} @ ${match[1]}`)
    })
    expect(sites.sort(), 'the mark has exactly two sizes and exactly two homes — 18 in the document bar, 22 on the load screen, nowhere else').toEqual(['App.tsx @ 18', 'LoadScreen.tsx @ 22'])
    // AND NO OTHER SPELLING OF THE PROP. A third site written `size={n}` or
    // `size={SOME_CONST}` would not be counted above, so the bare element
    // occurrences are counted independently and must agree.
    const anyUse = productionSources().flatMap((file) => {
      const code = stripComments(fs.readFileSync(file, 'utf8'))
      return [...code.matchAll(/<BrandMark\b/g)].map(() => path.relative(sourceDir, file))
    })
    expect(anyUse.sort(), 'every use of the mark must be one of the two declared sites, whatever spelling the size prop takes').toEqual(['App.tsx', 'LoadScreen.tsx'])
  })

  // The colour lives in CSS, so it cannot be proven behaviourally here: jsdom
  // parses no stylesheet. It is pinned by source text instead, and Verification
  // names it as not behaviourally proven. `e2e/brand-mark.spec.ts` carries the
  // only assertion that resolves the cascade, and that spec does not run here.
  it('pins the one App.css rule that is the mark\'s only source of colour', () => {
    const appCss = fs.readFileSync(appCssPath, 'utf8')
    expect(appCss, 'the mark takes its colour from .brand-mark { color: var(--color-brand) } and nowhere else').toMatch(/\.brand-mark\s*\{[^}]*color:\s*var\(--color-brand\)/)
    expect(appCss).toMatch(/\.brand-lockup\s*\{/)
    // Case-insensitive ON PURPOSE: `tokens.css` spells the value UPPERCASE
    // today, and a later re-casing of the token file is not a colour change.
    expect(fs.readFileSync(tokensCssPath, 'utf8'), 'compared case-insensitively: a re-cased token file is not a colour change').toMatch(/--color-brand:\s*#87f0ff/i)
  })
})
