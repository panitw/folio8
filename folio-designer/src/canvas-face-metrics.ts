// THE ONE RULE THAT MAKES THE BROWSER'S BASELINE THE ENGINE'S BASELINE.
//
// `.canvas-text-line` in App.css places a painted line by the engine's own
// baseline and then lifts it by one em:
//
//     top: var(--text-line-baseline); line-height: 1;
//     transform: translateY(calc(-1 * var(--text-font-size)));
//
// That arithmetic is only correct if the browser puts the glyph baseline
// exactly one em below the line box's top, and CSS inline layout does not.
// With `line-height: 1` the baseline lands at (1 + A - D) / 2 em, where A and
// D are the ASCENT AND DESCENT THE BROWSER READ OUT OF THE FACE — so every
// painted line was drawn (1 - A + D) / 2 em TOO HIGH, and that error is a
// property of the font rather than of the layout.
//
// IT WAS INVISIBLE FOR AS LONG AS THE BUILD'S OWN FACES WERE THE ONLY ONES
// MEASURED, and that is why it survived. Over the 120 faces this build ships
// the error runs 0.048 em to 0.235 em, and for each of them it stays inside
// the headroom the face leaves above its own ink, so the text sat a little
// high and never crossed the element's top edge. A DOCUMENT'S OWN face need
// not be so kind: TH Sarabun New declares hhea ascent 0.844 and descent
// -0.457, which is both the largest error of any face measured here (0.3065
// em) and the smallest headroom (0.177 em above the ink of Thai text with
// stacked vowels and tone marks), so its heading painted 0.13 em clear of the
// box the PDF draws it inside. A regression test over a SHIPPED face cannot
// see any of this; the guard that covers it must use a tight-ascent face.
//
// SO THE METRICS ARE OVERRIDDEN RATHER THAN THE ARITHMETIC CORRECTED. With
// ascent 100% and descent 0% the baseline sits at (1 + 1 - 0) / 2 = ONE EM
// below the line box top for every face, which is exactly what the transform
// above already assumes, and the correction needs no per-face number to reach
// the stylesheet. The alternative — projecting each resolved face's A and D
// onto the canvas and computing the offset — would put a second copy of the
// vertical model in TypeScript, in the units of whichever table the browser
// happened to consult (hhea, OS/2 typo, or usWin, which differ BY PLATFORM
// for one identical face), and the engine's model reads hhea alone.
//
// NOTHING IS RESHAPED. These descriptors feed line-box geometry only: glyph
// advances, kerning and the shaping of every run are untouched, which is what
// keeps AD-17 true — the browser stays a rasterizer of the engine's own
// positions, and the engine's positions are what moved back under the ink.
//
// THE RESIDUE, STATED. Chrome rounds the overridden ascent to a whole device
// pixel, so a painted baseline may still sit up to 1px off the engine's at any
// one zoom. That is a rasterization difference of the kind the canvas already
// lives with, not a layout error that scales with the font size.
//
// ⚠ THIS IS THE CANVAS'S ALONE. It belongs to the population that paints the
// PAGE — the carried faces `embedded-face-family.ts` names and the shipped and
// catalogue rules `scripts/build-wasm.mjs` emits. The font browser's specimens
// (`preview-face-family.ts`) and every chrome surface must keep their faces'
// REAL metrics: a specimen exists to show an author what a typeface does, and
// a design system's text is laid out by the browser rather than placed by the
// engine. `preview-face-registry.ts` is the one caller that opts out, and it
// says so.

// The descriptors in the shape `new FontFace` takes, for the faces a document
// carries.
export const canvasFaceMetricOverride: Readonly<Record<string, string>> = { ascentOverride: '100%', descentOverride: '0%', lineGapOverride: '0%' }

// The same three descriptors in the shape an `@font-face` rule takes, for the
// faces the build ships. `scripts/build-wasm.mjs` cannot import this module —
// it is a Node script emitting CSS, and the two spellings are different
// vocabularies rather than one value used twice — so the spellings are tied
// instead by `canvas-face-metrics.test.ts`, which reads the emitted stylesheet
// and asserts every canvas rule carries these three descriptors and that no
// chrome rule does.
export const canvasFaceMetricOverrideCss = 'ascent-override: 100%; descent-override: 0%; line-gap-override: 0%;'

// The descriptor set for a population that must keep its face's own metrics.
// Spelled as a named export rather than an inline `{}` so a reader at the call
// site sees a DECISION and lands on the rule above.
export const naturalFaceMetrics: Readonly<Record<string, string>> = {}
