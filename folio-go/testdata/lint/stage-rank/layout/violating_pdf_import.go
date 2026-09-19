// Package layout is the RETAINED VIOLATING FIXTURE for AD-5's own
// arrow, named explicitly rather than left to fall out of the rank
// class (D-000.23 read the other way: a guard written for a CLASS must
// still be shown to cover the INSTANCE it exists for).
//
// The instance, quoted from the spine's very first line of grounding:
// "One dependency arrow matters more than the rest: there is none from
// internal/layout to internal/pdf. That absence is what keeps PNG/SVG/
// HTML renderers possible later (AD-5), and it is precisely the arrow a
// well-meaning commit will try to add."
//
// This file IS that well-meaning commit. It never compiles into the
// module (testdata/ is excluded from every Go build) and exists only to
// be parsed by ScanStageRank.
package layout

import "github.com/panitw/folio8/folio-go/internal/pdf"

// Serialize is the shape the arrow arrives in: layout reaching into the
// serializer because the value it needed was easier to import than to
// pass.
func Serialize() { _ = pdf.Version }
