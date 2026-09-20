import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

import { canvasFaceMetricOverride, canvasFaceMetricOverrideCss, naturalFaceMetrics } from './canvas-face-metrics'

// THE TIE BETWEEN THE TWO SPELLINGS OF ONE DECISION.
//
// `canvas-face-metrics.ts` states the override once, in two vocabularies: the
// `new FontFace` descriptor OBJECT the faces a document carries are registered
// with, and the `@font-face` DESCRIPTOR LIST the faces the build ships are
// declared with. `scripts/build-wasm.mjs` cannot import the TypeScript module —
// it is a Node script emitting CSS — so it spells the CSS half a second time,
// and these are the guards that stop the two drifting.
//
// WHY THIS READS THE GENERATOR AND NOT ITS OUTPUT, for the reason
// `canvas-font-stack.test.ts` states at length: `src/generated/runtime-fonts.css`
// is gitignored and exists only after `build:wasm`, so asserting against it
// would make this guard's strength depend on build order, and a missing file is
// the classic way a guard goes quietly vacuous. The generator is a tracked
// source.
//
// AND WHY THE NEGATIVE HALF IS THE HALF THAT MATTERS. Overriding a face's
// ascent and descent is right for the page, where the ENGINE decides where a
// baseline sits and the browser only rasterizes there, and wrong everywhere
// else: the design system's own text is laid out BY the browser, and a font
// browser specimen exists to show an author what a typeface really does. The
// test that the three IBM Plex rules carry no override is therefore not
// symmetry for its own sake — it is the assertion that the canvas's correction
// has not leaked into the chrome.
const here = path.dirname(fileURLToPath(import.meta.url))
const generatorPath = path.join(here, '..', 'scripts', 'build-wasm.mjs')
const generatorSource = fs.readFileSync(generatorPath, 'utf8')

// The design system's three families, which are laid out by the browser and
// must keep their faces' real metrics.
const chromeFamilies = ['IBM Plex Sans', 'IBM Plex Mono', 'IBM Plex Sans Thai']

/**
 * Every `@font-face` rule the generator's source spells, template holes and all.
 *
 * ⚠ A RULE'S OWN BRACES ARE NOT ITS ONLY BRACES. The generator writes these
 * rules as template literals, so `${assets.sans}` and `${face.cssFamily}` sit
 * inside them and a `[^}]*` body stops at the first hole rather than at the end
 * of the rule — which silently truncated every rule before the descriptors this
 * file exists to find. The hole is matched as a unit instead.
 */
function generatorFontFaceRules(): ReadonlyArray<string> {
  return [...generatorSource.matchAll(/@font-face \{(?:\$\{[^}]*\}|[^{}])*\}/g)].map(([rule]) => rule)
}

describe('the canvas face metric override', () => {
  it('states the same three descriptors in both vocabularies', () => {
    // The CSS half is parsed rather than compared as a string, so the two are
    // tied by VALUE: a reordering is not a drift, a changed percentage is.
    const fromCss = Object.fromEntries(canvasFaceMetricOverrideCss.split(';').map((part) => part.trim()).filter((part) => part !== '').map((part) => {
      const [property, value] = part.split(':').map((half) => half.trim())
      return [property.replace(/-([a-z])/g, (_, letter: string) => letter.toUpperCase()), value]
    }))
    expect(fromCss).toEqual(canvasFaceMetricOverride)
  })

  it('makes the browser put the baseline exactly where .canvas-text-line assumes it is', () => {
    // WHAT THE NUMBERS MEAN, so a later author changing one has to mean it.
    // With `line-height: 1` the browser puts a glyph baseline at
    // (1 + A - D) / 2 em below the line box top, and `.canvas-text-line` lifts
    // the box by one em on the assumption that the baseline is one em down.
    // The transform is correct exactly when (1 + A - D) / 2 === 1.
    const ascent = Number.parseFloat(canvasFaceMetricOverride.ascentOverride) / 100
    const descent = Number.parseFloat(canvasFaceMetricOverride.descentOverride) / 100
    expect((1 + ascent - descent) / 2).toBe(1)
  })

  it('leaves a face registered under the natural metrics untouched', () => {
    // `new FontFace(family, bytes, {})` is the browser's own default: the
    // face's real ascent, descent and line gap. Stated as a checked emptiness
    // so the opt-out cannot quietly acquire a descriptor.
    expect(Object.keys(naturalFaceMetrics)).toEqual([])
  })

  it('overrides every face the generator declares for the page', () => {
    const rules = generatorFontFaceRules()
    // The generator declares thirteen hand-written rules plus one templated
    // catalogue rule. A regex that matched fewer would make every assertion
    // below vacuous, which is the failure mode this count exists to refuse.
    expect(rules.length).toBe(14)
    const canvasRules = rules.filter((rule) => !chromeFamilies.some((family) => rule.includes(`font-family: '${family}'`)))
    expect(canvasRules.length).toBe(11)
    for (const rule of canvasRules) expect(rule).toContain('${canvasFaceMetricOverrideCss}')
  })

  it('leaves the design system’s own families with their real metrics', () => {
    const rules = generatorFontFaceRules()
    const chromeRules = rules.filter((rule) => chromeFamilies.some((family) => rule.includes(`font-family: '${family}'`)))
    expect(chromeRules.length).toBe(chromeFamilies.length)
    for (const rule of chromeRules) {
      expect(rule).not.toContain('canvasFaceMetricOverrideCss')
      expect(rule).not.toContain('ascent-override')
      expect(rule).not.toContain('descent-override')
      expect(rule).not.toContain('line-gap-override')
    }
  })

  it('spells the CSS constant in the generator exactly as the module states it', () => {
    // The generator's own literal, read out of its source and compared to the
    // module's. This is the drift the two-vocabulary split makes possible.
    const declared = generatorSource.match(/const canvasFaceMetricOverrideCss = '([^']*)'/)
    expect(declared?.[1]).toBe(canvasFaceMetricOverrideCss)
  })
})
