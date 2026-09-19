package layout

import "github.com/panitw/folio8/folio-go/internal/geom"

// ColumnGeometry is one column's resolved position (Story 4.1, AC1/AC2):
// its x-origin, page-absolute-minus-band exactly like any other
// band-relative quantity in this package, and its width, taken VERBATIM
// from the template's declaration and never adjusted to fit content
// (AD-24 — "nothing negotiates").
type ColumnGeometry struct {
	X, Width geom.Length
}

// TableGeometry is a table's resolved column layout.
//
// R4/AC1 1c: there is deliberately NO width field here. A table's total
// width is AD-13's derived quantity — "the sum of its column widths… it
// is never stored as an independent field" — so the compiler itself is
// the anchor: nothing can read a stale total, because there is nowhere
// for one to be cached. Width() below is the only way to ask for it, and
// it recomputes the sum every call.
type TableGeometry struct {
	Columns []ColumnGeometry
}

// Width returns the table's total width: the sum of its columns'
// widths, computed fresh from Columns every call (AD-13).
func (g TableGeometry) Width() geom.Length {
	var total geom.Length
	for _, c := range g.Columns {
		total += c.Width
	}
	return total
}

// ColumnWidths lays out columns left-to-right from startX, a column's
// x-origin being the running sum of every preceding column's declared
// width. It is given ONLY the declared widths (D-000.16: "the signal
// rides on the value") — never a label, a measured glyph advance or a
// font — which is what makes AC2's property ("column widths are never
// negotiated against content") true by construction rather than by
// convention: there is no parameter here through which content could
// reach the computation.
func ColumnWidths(startX geom.Length, widths []geom.Length) TableGeometry {
	cols := make([]ColumnGeometry, len(widths))
	x := startX
	for i, w := range widths {
		cols[i] = ColumnGeometry{X: x, Width: w}
		x += w
	}
	return TableGeometry{Columns: cols}
}
