import { describe, expect, it } from 'vitest'
import { proposedBounds, resizeAnchors } from './resize-anchor'

const origin = { originalX: 10_000, originalY: 20_000, originalWidth: 72_000, originalHeight: 24_000 }

describe('resize anchors', () => {
  it('offers all eight handles and a move', () => {
    expect([...resizeAnchors].sort()).toEqual(['e', 'n', 'ne', 'nw', 's', 'se', 'sw', 'w'])
  })

  it('moves the origin without touching the size', () => {
    expect(proposedBounds('move', origin, 5_000, -3_000)).toEqual({ x: 15_000, y: 17_000, width: 72_000, height: 24_000 })
  })

  it('grows south-east from a fixed origin', () => {
    expect(proposedBounds('se', origin, 8_000, 6_000)).toEqual({ x: 10_000, y: 20_000, width: 80_000, height: 30_000 })
  })

  it('spends a north-west delta on both the origin and the size', () => {
    expect(proposedBounds('nw', origin, -4_000, -5_000)).toEqual({ x: 6_000, y: 15_000, width: 76_000, height: 29_000 })
    expect(proposedBounds('nw', origin, 4_000, 5_000)).toEqual({ x: 14_000, y: 25_000, width: 68_000, height: 19_000 })
  })

  it('holds the axis an edge handle does not own', () => {
    expect(proposedBounds('w', origin, 4_000, 9_000)).toEqual({ x: 14_000, y: 20_000, width: 68_000, height: 24_000 })
    expect(proposedBounds('n', origin, 9_000, 4_000)).toEqual({ x: 10_000, y: 24_000, width: 72_000, height: 20_000 })
    expect(proposedBounds('e', origin, 4_000, 9_000)).toEqual({ x: 10_000, y: 20_000, width: 76_000, height: 24_000 })
    expect(proposedBounds('s', origin, 9_000, 4_000)).toEqual({ x: 10_000, y: 20_000, width: 72_000, height: 28_000 })
  })

  it('stops a rectangle inverting under the pointer instead of proposing a negative size', () => {
    // Red proof: without the floor, dragging the west handle past the right
    // edge would hand Go an x beyond the box and a negative width.
    expect(proposedBounds('nw', origin, 200_000, 200_000)).toEqual({ x: 81_000, y: 43_000, width: 1_000, height: 1_000 })
    expect(proposedBounds('se', origin, -200_000, -200_000)).toEqual({ x: 10_000, y: 20_000, width: 1_000, height: 1_000 })
  })

  it('runs a move into the band edge and keeps the other axis tracking the hand', () => {
    // The reported behaviour: dragging past the left edge used to hand Go a
    // negative x, which it rejected, which put the component back where the
    // drag started. It now rests against the edge and keeps its y.
    const band = { band: 'pageHeader' as const, width: 523_276, height: 669_000 }
    expect(proposedBounds('move', origin, -110_000, -100_000, band)).toEqual({ x: 0, y: 0, width: 72_000, height: 24_000 })
    expect(proposedBounds('move', origin, -110_000, 30_000, band)).toEqual({ x: 0, y: 50_000, width: 72_000, height: 24_000 })
    expect(proposedBounds('move', origin, 900_000, 30_000, band)).toEqual({ x: 451_276, y: 50_000, width: 72_000, height: 24_000 })
    expect(proposedBounds('move', origin, 20_000, 900_000, band)).toEqual({ x: 30_000, y: 645_000, width: 72_000, height: 24_000 })
  })

  it('stops a resize at the band edge instead of proposing a rectangle that leaves it', () => {
    const band = { band: 'pageHeader' as const, width: 523_276, height: 669_000 }
    expect(proposedBounds('w', origin, -30_000, 0, band)).toEqual({ x: 0, y: 20_000, width: 82_000, height: 24_000 })
    expect(proposedBounds('n', origin, 0, -30_000, band)).toEqual({ x: 10_000, y: 0, width: 72_000, height: 44_000 })
    expect(proposedBounds('se', origin, 900_000, 900_000, band)).toEqual({ x: 10_000, y: 20_000, width: 513_276, height: 649_000 })
  })

  it('leaves a component alone when no band limit is supplied', () => {
    expect(proposedBounds('move', origin, 900_000, 900_000)).toEqual({ x: 910_000, y: 920_000, width: 72_000, height: 24_000 })
  })

  // DW-36, and Story 7.6's prerequisite for dragging onto a later sheet. The
  // content band is a COLUMN, not a page-tall box: clamping a drag at its
  // height pinned every component to the first window of a document the
  // canvas now draws every window of, so the lifted column was reachable by
  // command and not by hand.
  //
  // These are the RED PROOFS for the lift. Restoring the clamp — dropping the
  // BANDS_CAPPING_VERTICALLY gate in proposedBounds — turns the two vertical
  // assertions here back into 645_000 and 649_000, exactly the pageHeader
  // numbers above, which is what makes this a twin of that test rather than a
  // second copy of it.
  it('lets a move and a south resize run past one window in the content band, and still holds the sides', () => {
    const band = { band: 'content' as const, width: 523_276, height: 669_000 }
    // Three windows down the column, well past the band's own height.
    expect(proposedBounds('move', origin, 20_000, 900_000, band)).toEqual({ x: 30_000, y: 920_000, width: 72_000, height: 24_000 })
    expect(proposedBounds('se', origin, 900_000, 900_000, band)).toEqual({ x: 10_000, y: 20_000, width: 513_276, height: 924_000 })
    // What the lift does NOT touch, asserted in the same breath: the width is
    // still capped at the band's, in the content band as in every other, and
    // the band-origin floor still keeps y at or above zero.
    expect(proposedBounds('move', origin, 900_000, 30_000, band)).toEqual({ x: 451_276, y: 50_000, width: 72_000, height: 24_000 })
    expect(proposedBounds('move', origin, -110_000, -100_000, band)).toEqual({ x: 0, y: 0, width: 72_000, height: 24_000 })
    expect(proposedBounds('n', origin, 0, -30_000, band)).toEqual({ x: 10_000, y: 0, width: 72_000, height: 44_000 })
  })

  it('keeps the page footer capped exactly as the page header is', () => {
    // The lift is keyed on band identity, so the OTHER repeating band has to
    // be shown capped too — otherwise the gate could be reading nothing and
    // every assertion above would still pass.
    const band = { band: 'pageFooter' as const, width: 523_276, height: 669_000 }
    expect(proposedBounds('move', origin, 20_000, 900_000, band)).toEqual({ x: 30_000, y: 645_000, width: 72_000, height: 24_000 })
    expect(proposedBounds('se', origin, 900_000, 900_000, band)).toEqual({ x: 10_000, y: 20_000, width: 513_276, height: 649_000 })
  })

  it('leaves a component already smaller than the floor where it is', () => {
    const thin = { originalX: 0, originalY: 0, originalWidth: 400, originalHeight: 400 }
    expect(proposedBounds('nw', thin, 900, 900)).toEqual({ x: 0, y: 0, width: 400, height: 400 })
  })
})


it('keeps a line length at its orientation-preserving floor', () => {
  const origin = { originalX: 20000, originalY: 20000, originalWidth: 72000, originalHeight: 2000 }
  expect(proposedBounds('e', origin, -100000, 30000, undefined, 2000)).toEqual({ x: 20000, y: 20000, width: 2000, height: 2000 })
  expect(proposedBounds('w', origin, 100000, 30000, undefined, 2000)).toEqual({ x: 90000, y: 20000, width: 2000, height: 2000 })
  const vertical = { ...origin, originalWidth: 2000, originalHeight: 72000 }
  expect(proposedBounds('n', vertical, 30000, 100000, undefined, 2001)).toEqual({ x: 20000, y: 89999, width: 2000, height: 2001 })
  expect(proposedBounds('s', vertical, 30000, -100000, undefined, 2001)).toEqual({ x: 20000, y: 20000, width: 2000, height: 2001 })
})
