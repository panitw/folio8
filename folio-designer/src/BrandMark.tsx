// ONE DRAWING, ONE RULE, TWO SIZES — NOW TAKEN FROM THE BRAND ASSET ITSELF.
//
// The mark is a square outline containing a smaller solid block. It appears at
// exactly two sizes — 22px on the load screen, 18px in the document bar
// (`DESIGN.md:436-438`, which adds "Nowhere else") — and it is ONE component
// parameterised by `size`, never two drawings wearing one function signature.
//
// THE SOURCE OF TRUTH IS `resources/logo.png`, AND IT SUPERSEDES THE MOCKUPS.
// Story 14.5 derived this rule from `Main.dc.html`/`Load.dc.html`, which were
// approximations drawn before the brand asset existed; its comment warned
// against "fixing" the formula back to those mockups' integers, and that warning
// still stands — the mockups are not the authority. The logo is, and the numbers
// below are measured from it rather than eyeballed against it.
//
// THE MEASUREMENT, so it can be replayed. Decompose `resources/logo.png`
// (3000×929, RGBA) and take the cyan ink alone — the white "Folio8" wordmark
// beside it is not part of the mark. The mark's outer edge is a 784px square at
// (72,72); a vertical cut through its centre gives an 80px stroke, then the
// inner block, then an 80px stroke; the inner block's own bounding box is
// 296×369 at (244,208) relative to the outer edge. Those five integers over 784
// are the rule, and they are written below as exact fractions so that no
// decimal drift enters between the asset and the component.
//
// WHAT CHANGED FROM THE MOCKUP-DERIVED RULE, stated so the diff is legible:
//   stroke   0.083–0.068 of the box (a constant 1.5px)  →  0.102, and it SCALES
//   inner w  0.333 → 0.378      inner h  0.444 → 0.471
//   inner x  0.333 → 0.311      inner y  0.278 → 0.265
// The logo's mark is squarer-shouldered and more heavily drawn than the mockups
// made it look, which is most visible in the stroke.
//
// THE STROKE NOW SCALES, and that is the substantive reversal. Story 14.5 held
// it at 1.5px "because the mockups do not scale it"; the logo draws it as a
// fixed FRACTION of the box, so a mark at 22px is drawn more heavily than one at
// 18px, exactly as enlarging the asset would. The consequence is that neither
// size lands the stroke on a whole pixel (1.837 at 18, 2.245 at 22) — it did not
// before either, at 1.5 — so the edges antialias. That is the faithful rendering
// of a scaling rule at small sizes, not a defect to round away: snapping to 2px
// at both sizes would reintroduce a per-site lookup table, which is the one
// thing this component exists not to be.
//
// SIZED BY SVG ATTRIBUTES, NOT BY A CSS CLASS, which departs from every other
// glyph in this app (`.palette-icon { width: var(--icon-size) }`). The reason is
// testability: vitest runs in jsdom, which parses no stylesheet and computes no
// layout, so correctness placed in CSS has zero executable coverage here. As
// `width`/`height`/`viewBox`/`x`/`y` attributes the parameterisation is a
// literal assertion instead of an unprovable claim.
//
// COLOUR ARRIVES ONLY AS `currentColor`, set by `.brand-mark` in App.css from
// `var(--color-select)`. No colour literal is written in this file.
//
// DECORATIVE: `aria-hidden="true"`, no `role`, no `<title>`, no accessible name
// of its own — so the mark and the wordmark beside it announce the product name
// once, not twice.
//
// INLINE SVG, NEVER A FILE ON DISK (D-14.0.1, restated by D-13.6.7): the offline
// release's cache manifest counts emitted files and `vite.config.ts` sets
// `assetsInlineLimit: 0`. This story's asset-slot cost is zero.

// Three decimal places: enough to carry 6.796 and 8.472 without emitting a
// float tail that would make the expected attribute strings unreadable.
const round3 = (value: number) => Math.round(value * 1000) / 1000

// The logo's own pixels, as measured above. MARK is the outer edge of the
// square; every other constant is a length within it. Kept as integers over a
// shared denominator rather than pre-divided decimals so that re-measuring the
// asset is a direct comparison against these six numbers.
const MARK = 784
const STROKE = 80
const INNER_WIDTH = 296
const INNER_HEIGHT = 369
const INNER_X = 244
const INNER_Y = 208

export function BrandMark({ size }: { size: number }) {
  // The rect is stroked centred on its path, so insetting by half the stroke
  // lands the outer EDGE on the box — which is what the 784px square measures.
  const stroke = (size * STROKE) / MARK
  const outer = round3(size - stroke)
  const inset = round3(stroke / 2)
  const innerWidth = round3((size * INNER_WIDTH) / MARK)
  const innerHeight = round3((size * INNER_HEIGHT) / MARK)
  const innerX = round3((size * INNER_X) / MARK)
  const innerY = round3((size * INNER_Y) / MARK)
  return <svg aria-hidden="true" className="brand-mark" width={size} height={size} viewBox={`0 0 ${size} ${size}`} fill="none">
    <rect x={inset} y={inset} width={outer} height={outer} fill="none" stroke="currentColor" strokeWidth={round3(stroke)} />
    <rect x={innerX} y={innerY} width={innerWidth} height={innerHeight} fill="currentColor" />
  </svg>
}
