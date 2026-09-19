import { BAND_CONTENT_WINDOW_MARGIN, CAPPING_BANDS, type CanvasProjection, type CappingBand } from './engine-protocol'

// STORY 12.5. THE ARITHMETIC BEHIND A BAND-BOUNDARY DRAG, in the same
// millipoints Go projected, and nothing else.
//
// WHAT THIS MODULE IS NOT. It is not a model of the document, it measures no
// DOM, and it re-derives no band origin: `internal/layout` owns band placement
// (AD-24) and keeps it for the whole gesture. What moves while a boundary is
// dragged is ONE PROPOSED LINE over ONE projected number — the browser may show
// where a gesture has got to; it may not compute what the document would
// become. Everything else waits for the engine's re-projection at release.
//
// THE TWO BOUNDS IT HOLDS, and why a third is deliberately missing:
//
//   - A FLOOR AT 0. A band height is never negative in ANY document — that is a
//     property of the FIELD, and Story 17.4 item 9 authorises clamping it.
//   - THE CONTENT-WINDOW CEILING, mirrored. A property of the LAYOUT, permitted
//     under DW-36's condition: it CONSUMES the engine's own declaration
//     (BAND_CONTENT_WINDOW_MARGIN) rather than re-spelling it, and
//     engine-bounds-mirror.test.ts reads it doing so.
//   - NO STRAND FLOOR. Computable from the projection, and deliberately not
//     built (12.1's Q4, upheld by 12.5's R1). The discriminator: CLAMP A
//     GESTURE AT BOUNDS THAT CARRY NO INFORMATION; SEND, AND LET THE ENGINE
//     REFUSE, WHERE THE REFUSAL NAMES SOMETHING THE AUTHOR NEEDS. A ceiling
//     says only "no further", which a stopped pointer already says. The strand
//     refusal carries an ElementID — it tells the author WHICH component is in
//     the way — and a silent drag limit would destroy exactly that.

function clamp(value: number, low: number, high: number): number { return Math.min(Math.max(value, low), Math.max(low, high)) }

// bandBoundaryCeiling is Go's bandContentWindowCeiling, asked of the
// projection. It reads NOTHING but band heights: `innerH` is their sum, which
// is exact because isCanvas enforces contiguity (`paint.y === prior.y +
// prior.height`) over exactly three bands, so the three heights partition the
// printable column — the same partition internal/layout's BandOrigins states.
// Summing what layout already derived is not a second derivation of it, and it
// is the reason no margin, page height or origin is touched here.
export function bandBoundaryCeiling(bands: CanvasProjection['bands'], band: CappingBand): number {
  // The OTHER capping band is taken from the shared list rather than named, so
  // this file holds no copy of it (engine-bounds-mirror.test.ts's census).
  const other = CAPPING_BANDS.find((candidate) => candidate !== band)
  let innerH = 0
  let otherHeight = 0
  for (const projected of bands) {
    innerH += projected.height
    if (projected.name === other) otherHeight = projected.height
  }
  return innerH - otherHeight - BAND_CONTENT_WINDOW_MARGIN
}

// proposedBandHeight turns pointer travel into a candidate height. `dy` is
// DOWNWARD travel in millipoints, already through canvasDisplay.documentDelta,
// and `limit` is supplied by the caller — this function holds no bound of its
// own beyond the floor.
//
// THE SIGN FLIP IS THE WHOLE OF THE FOOTER CASE. Both interactive boundaries
// are the TOP edge of the band below them, but the height they govern is
// measured in opposite directions: dragging the header/content boundary DOWN
// makes the page header taller, and dragging the content/footer boundary down
// makes the page footer SHORTER. A footer grows as the pointer rises.
//
// THE RESULT IS ROUNDED TO A WHOLE MILLIPOINT, and that is load-bearing rather
// than tidy. `dy` reaches here as canvasDisplay.documentDelta(px, zoom) * 1000,
// and at any zoom but 1 that product is a float artifact rather than an
// integer: at zoom 1.1 a −36px drag gives −32726.999999999996, and App.tsx's
// `points()` — which assumes whole millipoints — spells the result
// "27.273.00000000000364". That is not a JSON number, so command-json's
// jsonNumber replaces it with `null`, the command goes out as
// `"height":null`, the engine refuses it, and the author has already been
// reading the malformed string in the on-canvas readout for the whole gesture.
// Every other geometry factory rounds at the same seam (component-command.ts's
// `millipoints`); this is that seam for a band height.
export function proposedBandHeight(band: CappingBand, originalHeight: number, dy: number, limit: number): number {
  return Math.round(clamp(originalHeight + (band === 'pageFooter' ? -dy : dy), 0, limit))
}

// boundaryOffset is where the proposed line is painted, relative to the top of
// the band the handle sits on — which is where the boundary is today. It is the
// clamped travel, so the line stops exactly where the proposal stops rather
// than running on past a bound the release would not honour.
//
// The two subtractions are written out rather than one of them negated: `-0`
// is a real JavaScript value, and negating a zero difference would hand the
// stylesheet a signed zero for the commonest offset there is.
export function boundaryOffset(band: CappingBand, originalHeight: number, proposedHeight: number): number {
  return band === 'pageFooter' ? originalHeight - proposedHeight : proposedHeight - originalHeight
}
