package pdf

import (
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
)

// appendRectContentStream appends one bracketed content-stream sequence
// per page-model Rect (Story 4.1, AC3/AC6): the page model's first vector
// primitive, drawn BEFORE any text or image content, so a cell's
// background and border sit behind its label rather than over it.
//
// Fill, when present, is one "re f" path: PDF's rectangle operator
// already describes the whole box, so there is nothing to decompose into
// edges. Stroke draws only the EDGES the template asked for
// (style.border.edges) — PDF's own rectangle stroke ("re S") always
// strokes all four sides, so a stroked SUBSET is built from explicit
// "... m ... l" subpaths and closed off by a single "S": a stroke
// operator paints every subpath the current path holds, so N
// one-segment subpaths plus one S draws N independent lines.
//
// q/Q brackets the fill half and the stroke half INDEPENDENTLY, so a
// Rect with only a fill (or only a stroke) never leaves an unused
// colour or line width in the graphics state for whatever the next
// primitive draws — the same discipline textdoc.go's clip already uses
// for BT/ET (D-1.8's convention).
func appendRectContentStream(dst []byte, page pagemodel.Page) []byte {
	for _, r := range page.Rects {
		pdfX := page.MarginLeft + r.X
		pdfX2 := pdfX + r.W

		// Both derived from flipY (AD-24/D-1.8.10 — the module's one and
		// only coordinate inverter): the box's TOP edge (drawnHeight 0,
		// since r.Y already names the box's top) and its BOTTOM edge
		// (drawnHeight 0, topY argument shifted down by r.H) — the same
		// "bottom edge via flipY(..., Y, height)" shape imagedoc.go
		// already uses for a placement's own bottom, spelled as two
		// calls because a stroked edge needs both corners, not only the
		// bottom-left origin a fill's "re" alone would need.
		topY := flipY(page.Height, page.MarginTop, r.Y, 0)
		bottomY := flipY(page.Height, page.MarginTop, r.Y+r.H, 0)

		if r.HasFill {
			dst = append(dst, "q\n"...)
			dst = appendColorChannels(dst, r.Fill)
			dst = append(dst, " rg\n"...)
			dst = appendLength(dst, pdfX)
			dst = append(dst, ' ')
			dst = appendLength(dst, bottomY)
			dst = append(dst, ' ')
			dst = appendLength(dst, r.W)
			dst = append(dst, ' ')
			dst = appendLength(dst, r.H)
			dst = append(dst, " re f\nQ\n"...)
		}

		if r.HasStroke && (r.Edges.Top || r.Edges.Right || r.Edges.Bottom || r.Edges.Left) {
			dst = append(dst, "q\n"...)
			dst = appendColorChannels(dst, r.Stroke)
			dst = append(dst, " RG\n"...)
			dst = appendLength(dst, r.StrokeWidth)
			dst = append(dst, " w\n"...)
			if r.Edges.Top {
				dst = appendEdge(dst, pdfX, topY, pdfX2, topY)
			}
			if r.Edges.Bottom {
				dst = appendEdge(dst, pdfX, bottomY, pdfX2, bottomY)
			}
			if r.Edges.Left {
				dst = appendEdge(dst, pdfX, bottomY, pdfX, topY)
			}
			if r.Edges.Right {
				dst = appendEdge(dst, pdfX2, bottomY, pdfX2, topY)
			}
			dst = append(dst, "S\n"...)
			dst = append(dst, "Q\n"...)
		}
	}
	return dst
}

// appendEdge appends one "x1 y1 m x2 y2 l" subpath — a single stroked
// line segment, one of a Rect's up-to-four edges.
//
// OPERANDS PRECEDE THE OPERATOR. PDF content streams are postfix, like
// every other operator this package emits ("<w> w", "<r> <g> <b> RG",
// "<x> <y> <w> <h> re f"). This function shipped prefix — "m x1 y1 l x2
// y2" — from Story 4.1 until Story 9.1, which is not a cosmetic
// difference: a reader binds each pair of numbers to the operator that
// FOLLOWS them, so the leading "m" ran with an empty operand stack and
// every subsequent pair bound one operator late. Four edges came out as
// a moveto/lineto chain zig-zagging between opposite corners — the
// familiar "crossed box". It was never caught because the byte-level
// tests count "m " occurrences (how many subpaths) rather than reading
// the operand order, and no golden was ever opened and looked at with a
// stroked edge in it.
func appendEdge(dst []byte, x1, y1, x2, y2 geom.Length) []byte {
	dst = appendLength(dst, x1)
	dst = append(dst, ' ')
	dst = appendLength(dst, y1)
	dst = append(dst, " m "...)
	dst = appendLength(dst, x2)
	dst = append(dst, ' ')
	dst = appendLength(dst, y2)
	dst = append(dst, " l\n"...)
	return dst
}

// appendColorChannels appends "<r> <g> <b>", the rg/RG operator's three
// decimal operands, converting each 0..255 page-model channel with the
// module's one round-half-to-even scaler (AC5, this story's D2):
// geom.ScaleRound(channel, 1000, 255) produces a thousandths-scaled
// value — exactly appendLength's own input convention — so this
// introduces no new numeric representation and no float anywhere on the
// path (AD-1/AD-3/AD-23). This is the ONLY site under internal/pdf that
// converts a colour channel; both fill and stroke route through it.
func appendColorChannels(dst []byte, c pagemodel.Color) []byte {
	dst = appendLength(dst, geom.ScaleRound(geom.Length(c.R), 1000, 255))
	dst = append(dst, ' ')
	dst = appendLength(dst, geom.ScaleRound(geom.Length(c.G), 1000, 255))
	dst = append(dst, ' ')
	dst = appendLength(dst, geom.ScaleRound(geom.Length(c.B), 1000, 255))
	return dst
}
