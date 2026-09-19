import { describe, expect, it } from 'vitest'
import { bandBoundaryCeiling, boundaryOffset, proposedBandHeight } from './band-boundary'
import { BAND_CONTENT_WINDOW_MARGIN, type CanvasProjection } from './engine-protocol'

// The pure arithmetic behind the boundary drag, in millipoints. Nothing here
// touches the DOM, so every claim is a statement about numbers the engine
// projected and a pointer delta — which is the whole reason the arithmetic
// lives outside App.tsx.
//
// A4 portrait at 36pt margins: 769.89pt of printable column, partitioned by
// the three bands. header 20 + content 719.89 + footer 30 = 769.89.
const bands: CanvasProjection['bands'] = [
  { name: 'pageHeader', x: 36_000, y: 36_000, width: 523_276, height: 20_000 },
  { name: 'content', x: 36_000, y: 56_000, width: 523_276, height: 719_890 },
  { name: 'pageFooter', x: 36_000, y: 775_890, width: 523_276, height: 30_000 },
]
const innerH = 769_890

describe('bandBoundaryCeiling', () => {
  it('is the printable column less the OTHER capping band, one millipoint short', () => {
    // Go's bandContentWindowCeiling(other, innerH) = innerH - other - 1, asked
    // of each band in turn. The numbers are written out rather than derived,
    // so a rearrangement of the function that happens to agree with itself
    // still has to agree with these.
    expect(bandBoundaryCeiling(bands, 'pageHeader')).toBe(769_890 - 30_000 - 1)
    expect(bandBoundaryCeiling(bands, 'pageFooter')).toBe(769_890 - 20_000 - 1)
    expect(bandBoundaryCeiling(bands, 'pageHeader')).toBe(739_889)
  })

  it('sums the projected heights rather than deriving the column from the page', () => {
    // The whole column is the three heights, by isCanvas's contiguity
    // invariant. A ceiling that read a page height and two margins would be
    // band placement, which is internal/layout's alone.
    expect(bands.reduce((total, band) => total + band.height, 0)).toBe(innerH)
    // Move the CONTENT band's height and the ceiling moves with it: the sum is
    // real, not a constant that happens to match.
    const shorter = bands.map((band) => band.name === 'content' ? { ...band, height: 619_890 } : band)
    expect(bandBoundaryCeiling(shorter, 'pageHeader')).toBe(669_890 - 30_000 - 1)
  })

  it('consumes the mirrored margin rather than a bare literal', () => {
    // The one input that is NOT projected. If the engine ever stopped requiring
    // a strictly positive content region, this is the number that would move,
    // and engine-bounds-mirror.test.ts is what makes it move on both sides.
    expect(BAND_CONTENT_WINDOW_MARGIN).toBe(1)
    expect(bandBoundaryCeiling(bands, 'pageHeader') + BAND_CONTENT_WINDOW_MARGIN).toBe(innerH - 30_000)
  })
})

describe('proposedBandHeight', () => {
  it('grows the page header as the pointer descends and shrinks it as it rises', () => {
    expect(proposedBandHeight('pageHeader', 20_000, 40_000, 739_889)).toBe(60_000)
    expect(proposedBandHeight('pageHeader', 20_000, -5_000, 739_889)).toBe(15_000)
  })

  // THE SIGN FLIP, and it is the half a suite that only ever tested the header
  // would let rot: both boundaries are the top edge of the band below them, but
  // the footer's height is measured upward from the page foot.
  it('grows the page footer as the pointer RISES', () => {
    expect(proposedBandHeight('pageFooter', 30_000, -25_000, 749_889)).toBe(55_000)
    expect(proposedBandHeight('pageFooter', 30_000, 10_000, 749_889)).toBe(20_000)
  })

  it('returns the original height exactly when the pointer comes back to where it started', () => {
    // Matrix row 4: a gesture that returns to its start must produce the value
    // already in force, to the millipoint, or the send-only-if-changed test in
    // App.tsx compares two numbers that merely look alike and sends a command.
    expect(proposedBandHeight('pageHeader', 20_000, 0, 739_889)).toBe(20_000)
    expect(proposedBandHeight('pageFooter', 30_000, 0, 749_889)).toBe(30_000)
  })

  it('stops at zero rather than proposing a negative band', () => {
    // Matrix row 8. A negative height is a property of the FIELD — never legal
    // in any document — so the proposal never shows one.
    expect(proposedBandHeight('pageHeader', 20_000, -50_000, 739_889)).toBe(0)
    expect(proposedBandHeight('pageFooter', 30_000, 90_000, 749_889)).toBe(0)
  })

  it('stops at the ceiling rather than proposing a document with no content window', () => {
    // Matrix row 9. The released value is legal, so the pointer running on
    // costs the author nothing but a stopped line.
    expect(proposedBandHeight('pageHeader', 20_000, 5_000_000, 739_889)).toBe(739_889)
    expect(proposedBandHeight('pageFooter', 30_000, -5_000_000, 749_889)).toBe(749_889)
  })

  // RED PROOFS (D-000.14), each run by mutating band-boundary.ts and never the
  // expectation, and each recorded with what it reddens:
  //
  //  1. DELETE THE FLOOR — the clamp replaced by `Math.min(limit, …)`: THREE
  //     rows fail, 'stops at zero rather than proposing a negative band', this
  //     one, and boundaryOffset's 'follows the clamp rather than the raw
  //     travel'. Nine pass.
  //  2. DELETE THE CEILING — the clamp's high bound replaced by
  //     Number.POSITIVE_INFINITY: 'stops at the ceiling…' and 'follows the
  //     clamp…' fail; bandBoundaryCeiling's own describe stays GREEN, which is
  //     the point — the bound existing is not the bound being applied.
  //  3. DELETE THE SIGN FLIP — `originalHeight + dy` for both bands: 'grows the
  //     page footer as the pointer RISES' fails, and both footer clamp rows
  //     fail with it.
  //  4. DROP THE MARGIN — `innerH - otherHeight`: all three bandBoundaryCeiling
  //     rows fail, and so does engine-bounds-mirror's content-window pair.
  //  5. DROP THE Math.round — 'returns whole millipoints even when the pointer
  //     delta is a float' fails here, and App.test.tsx's zoom row fails with
  //     it, on a readout spelled "27.273.00000000000364" and a command carrying
  //     `"height":null`.
  // THE ZOOM ARTIFACT, AT THE UNIT WHERE IT IS BORN. `dy` arrives as
  // documentDelta(px, zoom) * 1000, which is an integer only at zoom 1; every
  // other rung of the ladder hands this function a float, and a float
  // millipoint reaches App.tsx's `points()` — which assumes whole millipoints —
  // as a string with two decimal points in it. That string is not a JSON
  // number, so the command would carry `"height":null`.
  it('returns whole millipoints even when the pointer delta is a float', () => {
    // The measured case: zoom 1.1, a 36px upward drag on a 60pt header.
    // documentDelta(-36, 1.1) is -32.727, and -32.727 * 1000 is
    // -32726.999999999996 in IEEE 754.
    const dy = (Math.round((-36 / 1.1) * 1000) / 1000) * 1000
    expect(Number.isInteger(dy)).toBe(false)
    const proposed = proposedBandHeight('pageHeader', 60_000, dy, 739_889)
    expect(Number.isInteger(proposed)).toBe(true)
    expect(proposed).toBe(27_273)
    // The footer takes the same delta through the sign flip and must land whole
    // there too.
    expect(Number.isInteger(proposedBandHeight('pageFooter', 60_000, dy, 749_889))).toBe(true)
    // And the offset the line is painted at inherits it, because it is a
    // difference of two whole numbers.
    expect(Number.isInteger(boundaryOffset('pageHeader', 60_000, proposed))).toBe(true)
  })

  it('is clamped at both ends by ONE expression, so neither bound can be lost alone', () => {
    // Non-vacuity for the two rows above: the clamped results are the bounds
    // themselves and not some other number that happens to be small or large.
    expect(proposedBandHeight('pageHeader', 20_000, -20_001, 739_889)).toBe(0)
    expect(proposedBandHeight('pageHeader', 20_000, 719_889, 739_889)).toBe(739_889)
    expect(proposedBandHeight('pageHeader', 20_000, 719_888, 739_889)).toBe(739_888)
  })
})

describe('boundaryOffset', () => {
  it('paints the line where the clamped proposal put it, in the band it belongs to', () => {
    // Relative to the top of the band the handle sits on, which is where the
    // boundary is today: positive is downward on screen for both bands.
    expect(boundaryOffset('pageHeader', 20_000, 60_000)).toBe(40_000)
    expect(boundaryOffset('pageHeader', 20_000, 15_000)).toBe(-5_000)
    // A footer that GREW by 25pt puts the line 25pt HIGHER.
    expect(boundaryOffset('pageFooter', 30_000, 55_000)).toBe(-25_000)
    expect(boundaryOffset('pageFooter', 30_000, 20_000)).toBe(10_000)
  })

  it('is zero exactly when the proposal is the height already in force', () => {
    expect(boundaryOffset('pageHeader', 20_000, 20_000)).toBe(0)
    expect(boundaryOffset('pageFooter', 30_000, 30_000)).toBe(0)
  })

  // The line stops where the PROPOSAL stops, which is what makes Matrix rows 8
  // and 9 visible: a line drawn from raw travel would run on past a bound the
  // release would not honour.
  it('follows the clamp rather than the raw travel', () => {
    const limit = 739_889
    expect(boundaryOffset('pageHeader', 20_000, proposedBandHeight('pageHeader', 20_000, -50_000, limit))).toBe(-20_000)
    expect(boundaryOffset('pageHeader', 20_000, proposedBandHeight('pageHeader', 20_000, 5_000_000, limit))).toBe(719_889)
  })
})
