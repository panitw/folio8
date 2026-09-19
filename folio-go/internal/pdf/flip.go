package pdf

import "github.com/panitw/folio8/folio-go/internal/geom"

// flipY is AD-24's "exactly one function" that inverts a coordinate
// (D-1.8.10, confirming AC17/AC17a): every content-stream Y coordinate
// this module ever emits — text baselines (textdoc.go) and image
// placements (imagedoc.go) alike — is derived from this function, and
// nowhere else in the module computes pageHeight-minus-something to
// place something.
//
// It converts a TOP-DOWN, page-local Y (the offset from the page's
// printable top edge to the TOP of whatever is being placed; this is
// how Y reads in the `.folio` document model, per textdoc.go's TextRun
// doc comment) into PDF user space's BOTTOM-UP Y (the coordinate of the
// BOTTOM of that same thing) — what every placement operator (Tm's ty,
// or cm's f for an image) actually needs.
//
// D-1.8.10, verbatim, on why the guard for this function is POSITIVE
// rather than a search for minus signs: "assert that ALL content-stream
// coordinate emission routes through the one flip function... not that
// 'nobody else writes a minus sign', which is unfalsifiable." See
// TestContentStreamYCoordinatesRouteThroughFlipY (flip_test.go) for that
// guard. By CONVENTION, every local variable holding a value obtained
// from this function — or passed directly as this function's return
// value — to a content-stream numeric emitter is named with a trailing
// "Y" (pdfX/pdfY, imgX/imgY, ...); the guard enforces that convention.
func flipY(pageHeight, marginTop, topY, drawnHeight geom.Length) geom.Length {
	return pageHeight - marginTop - topY - drawnHeight
}
