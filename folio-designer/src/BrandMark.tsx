// STORY 14.5 — ONE DRAWING, ONE RULE, TWO SIZES.
//
// The mark is a square outline containing a smaller centred solid block. It
// appears at exactly two sizes — 22px on the load screen, 18px in the document
// bar (`DESIGN.md:436-438`, which adds "Nowhere else") — and it is ONE
// component parameterised by `size`, never two drawings wearing one function
// signature.
//
// THE GEOMETRY RULE:
//   stroke     = 1.5                    constant at every size
//   outer rect = x,y 0.75   w,h size − 1.5   inset by half the stroke, so the
//                                            outer EDGE lands on the box
//   inner rect = w size/3   h size×4/9   x size/3   y size×5/18
//
// ⚠ THE MOCKUP'S INTEGERS WERE COMPARED AND CONSCIOUSLY NOT MATCHED. Do not
// "fix" the formula back to them. `Main.dc.html:24-25` draws 18px outer with an
// inner 6×8; the rule reproduces that EXACTLY. `Load.dc.html:24-25` draws 22px
// outer with an inner 7×10, where the rule gives 7.333×9.778 — deltas of 0.34px
// and 0.22px. A lookup table of the mockup's two integer pairs would satisfy the
// pixels and violate the AC: that is two drawings, not one rule. The mockups'
// three shapes are not even geometrically similar (inner÷outer widths are 0.333,
// 0.318, 0.385), so no rule fits all three; the 18/22 pair fits this one, and
// the 13px shape at `Load.dc.html:78-80` is the CJK row's in-progress marker,
// not a brand instance.
//
// The stroke does NOT scale: 1.5 at both sizes, as the mockups draw it.
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

// Three decimal places: enough to carry 7.333 and 9.778 without emitting a
// float tail that would make the expected attribute strings unreadable.
const round3 = (value: number) => Math.round(value * 1000) / 1000

export function BrandMark({ size }: { size: number }) {
  const stroke = 1.5
  const outer = round3(size - stroke)
  const innerWidth = round3(size / 3)
  const innerHeight = round3((size * 4) / 9)
  const innerX = round3(size / 3)
  const innerY = round3((size * 5) / 18)
  return <svg aria-hidden="true" className="brand-mark" width={size} height={size} viewBox={`0 0 ${size} ${size}`} fill="none">
    <rect x="0.75" y="0.75" width={outer} height={outer} fill="none" stroke="currentColor" strokeWidth={stroke} />
    <rect x={innerX} y={innerY} width={innerWidth} height={innerHeight} fill="currentColor" />
  </svg>
}
