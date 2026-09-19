import { describe, expect, it } from 'vitest'
import { clampPreviewScale, fitPreviewScale, MAX_PREVIEW_SCALE, MIN_PREVIEW_SCALE, PREVIEW_ZOOM_CHOICES, steppedPreviewScale, typedPreviewPage, typedPreviewZoom } from './viewer-navigation'

// STORY 13.2 — ONE TEST PER ARITHMETIC ROW OF THE STORY'S I/O MATRIX.
//
// Every expectation below is a LITERAL. Recomputing the answer with the same
// division the code under test uses would pass over a fit that divides the
// wrong way round, which is the one mistake this arithmetic can actually make.
describe('the PDF viewer\'s navigation arithmetic', () => {
  it('fits a page to the container\'s width', () => {
    // 800 usable px, a page 600 CSS px wide at scale 1.
    // `4 / 3` rather than `800 / 600`: the same number, written so the row does
    // not restate the division the code under test performs (D-11.2.2).
    expect(fitPreviewScale('width', { width: 600, height: 900 }, { width: 800, height: 500 })).toBe(4 / 3)
    // The height is not consulted at all on this arm — a page ten times too
    // tall for the box still fits its width.
    expect(fitPreviewScale('width', { width: 600, height: 9000 }, { width: 800, height: 500 })).toBe(4 / 3)
  })

  it('fits a whole page inside the container\'s box', () => {
    // 800x500 around a 600x900 page: 800/600 = 1.3333, 500/900 = 0.5555. The
    // SMALLER wins, or the page would not be whole.
    expect(fitPreviewScale('page', { width: 600, height: 900 }, { width: 800, height: 500 })).toBe(5 / 9)
    // And the other way round, so this cannot pass by always taking the height.
    expect(fitPreviewScale('page', { width: 600, height: 300 }, { width: 300, height: 900 })).toBe(0.5)
  })

  it('leaves the scale alone when the container cannot be measured', () => {
    for (const box of [{ width: 0, height: 0 }, { width: 800, height: 0 }, { width: 0, height: 500 }, { width: -800, height: 500 }, { width: Number.NaN, height: 500 }, { width: Number.POSITIVE_INFINITY, height: 500 }]) {
      expect(fitPreviewScale('width', { width: 600, height: 900 }, box)).toBeUndefined()
      expect(fitPreviewScale('page', { width: 600, height: 900 }, box)).toBeUndefined()
    }
    // A page of no size is the same answer, and for the same reason: there is
    // no scale that could be handed to the rasterizer instead.
    expect(fitPreviewScale('width', { width: 0, height: 900 }, { width: 800, height: 500 })).toBeUndefined()
    expect(fitPreviewScale('page', { width: 600, height: Number.NaN }, { width: 800, height: 500 })).toBeUndefined()
  })

  it('re-resolves a fit for whatever page is now showing', () => {
    // The same fit rule, the same box, two pages of different sizes — which is
    // what "fit persists across a page change" has to mean to be worth having.
    const box = { width: 800, height: 500 }
    expect(fitPreviewScale('width', { width: 600, height: 900 }, box)).toBe(4 / 3)
    expect(fitPreviewScale('width', { width: 400, height: 900 }, box)).toBe(2)
  })

  it('steps the zoom from wherever the scale actually stands, inside the bounds', () => {
    expect(steppedPreviewScale(1, 1)).toBe(1.1)
    expect(steppedPreviewScale(1, -1)).toBe(0.9)
    // From a RESOLVED fit scale, which is the case the story adds: 1.333333 is
    // not a listed percentage and stepping from it is the whole point of
    // writing the resolved value back into the view state.
    expect(steppedPreviewScale(4 / 3, 1)).toBe(1.433333)
    // The bounds hold at both ends.
    expect(steppedPreviewScale(MIN_PREVIEW_SCALE, -1)).toBe(0.5)
    expect(steppedPreviewScale(MAX_PREVIEW_SCALE, 1)).toBe(2)
    expect(steppedPreviewScale(Number.NaN, 1)).toBeUndefined()
  })

  // A FIT SCALE IS NOT INSIDE THE ZOOM BOUNDS, AND A STEP FROM ONE USED TO GO
  // BACKWARDS. `fitPreviewScale` is deliberately unclamped and the viewer writes
  // the resolved fit into `state.scale`, so both numbers below are scales the
  // stepper really is handed. Clamping alone then inverted the control.
  it('refuses a step that would move the scale the wrong way, and still clamps every step that would not', () => {
    // The two fits, resolved against real viewer boxes and asserted first so the
    // rows beneath are stepping from a scale this module actually produces.
    expect(fitPreviewScale('page', { width: 816, height: 1056 }, { width: 900, height: 320 })).toBe(0.30303030303030304)
    expect(fitPreviewScale('width', { width: 100, height: 100 }, { width: 900, height: 320 })).toBe(9)
    // `Zoom out PDF` at 30%: the step landed on 0.20303…, which clamped UP to
    // 0.5 and enlarged the page by 65% under a button that says zoom out.
    expect(steppedPreviewScale(0.30303030303030304, -1)).toBeUndefined()
    // `Zoom in PDF` at 900%: the step landed on 9.1, which clamped DOWN to 2.
    expect(steppedPreviewScale(9, 1)).toBeUndefined()
    // The OTHER direction from each of those two scales is a step towards the
    // bounds, so it is allowed and clamps exactly as it always did.
    expect(steppedPreviewScale(0.30303030303030304, 1)).toBe(0.5)
    expect(steppedPreviewScale(9, -1)).toBe(2)
    // A scale sitting ON a bound is not outside it: pressing further at either
    // end still answers with the bound rather than with "no change".
    expect(steppedPreviewScale(MIN_PREVIEW_SCALE, -1)).toBe(0.5)
    expect(steppedPreviewScale(MAX_PREVIEW_SCALE, 1)).toBe(2)
  })

  // THE RESOLVED-VALUE GUARD IS REACHABLE, WHICH IS NOT OBVIOUS. `measurable`
  // passes a denormal happily — 5e-324 is finite and greater than zero — and the
  // division that follows is what overflows. Without the guard on the RESOLVED
  // number an Infinity would reach the rasterizer through a box that measured.
  it('refuses a fit whose own division overflows to Infinity', () => {
    const denormal = { width: 5e-324, height: 5e-324 }
    const huge = { width: 1e300, height: 1e300 }
    expect(fitPreviewScale('width', denormal, huge)).toBeUndefined()
    expect(fitPreviewScale('page', denormal, huge)).toBeUndefined()
  })

  // THE ROUNDING, PINNED AT EACH PLACE IT HAPPENS.
  it('strips the float noise a stepped scale arrives at, at each site that strips it', () => {
    // The case the module comment names. MEASURED, and it is worth recording
    // that the comment overstates it: `1 - 0.1 - 0.1` is EXACTLY 0.8 in
    // IEEE-754 doubles, as is `1 + -2 * 0.1`, so this row pins the arithmetic's
    // answer and not the rounding. The two rows below pin the rounding.
    expect(steppedPreviewScale(1, -2)).toBe(0.8)
    // `+` pressed at 70% really computes 0.7 + 0.1 = 0.7999999999999999, which
    // would otherwise be written back into the view state and keep accumulating.
    expect(steppedPreviewScale(0.7, 1)).toBe(0.8)
    // And `clampPreviewScale`'s own rounding, reached DIRECTLY rather than
    // through a step, because that is the only way to tell the two sites apart:
    // 0.7 + 0.6 is 1.2999999999999998.
    expect(clampPreviewScale(0.7 + 0.6)).toBe(1.3)
  })

  it('clamps a typed zoom into the bounds and refuses one that is not a number', () => {
    expect(typedPreviewZoom('250')).toBe(2)
    expect(typedPreviewZoom('10')).toBe(0.5)
    expect(typedPreviewZoom('150')).toBe(1.5)
    expect(typedPreviewZoom(' 75 % ')).toBe(0.75)
    for (const rejected of ['abc', '', '   ', '-100', '1e3', '12.5.1', '%']) expect(typedPreviewZoom(rejected)).toBeUndefined()
    // Zero is refused rather than clamped: it is the one value that would make
    // a rasterizer draw nothing at all, so it never becomes 50% by accident.
    expect(typedPreviewZoom('0')).toBeUndefined()
  })

  it('accepts a typed page in range and refuses every entry outside it', () => {
    expect(typedPreviewPage('7', 34)).toBe(7)
    expect(typedPreviewPage(' 34 ', 34)).toBe(34)
    for (const rejected of ['0', '35', 'abc', '', '-1', '2.5']) expect(typedPreviewPage(rejected, 34)).toBeUndefined()
    // With no count yet — the document is still opening — only the lower bound
    // can be checked, and it still is.
    expect(typedPreviewPage('35', undefined)).toBe(35)
    expect(typedPreviewPage('0', undefined)).toBeUndefined()
  })

  it('refuses any scale a rasterizer could not use, and clamps the rest', () => {
    expect(clampPreviewScale(1.5)).toBe(1.5)
    expect(clampPreviewScale(9)).toBe(MAX_PREVIEW_SCALE)
    expect(clampPreviewScale(0.01)).toBe(MIN_PREVIEW_SCALE)
    for (const refused of [0, -1, Number.NaN, Number.POSITIVE_INFINITY]) expect(clampPreviewScale(refused)).toBeUndefined()
  })

  it('offers the percentages the zoom control names, and only those', () => {
    // The list is asserted rather than assumed: the select's options and the
    // "is this scale one of the listed choices" test both read this one array.
    expect([...PREVIEW_ZOOM_CHOICES]).toEqual([0.5, 0.75, 1, 1.5, 2])
    for (const choice of PREVIEW_ZOOM_CHOICES) expect(clampPreviewScale(choice)).toBe(choice)
  })
})
