// Package layout places a document's bands on the page and produces the
// page model (AD-5, AD-24). It is the stage between the bound document
// and any renderer, and it knows nothing about any output format:
// stage-rank 7, so it may not import internal/pdf (rank 8) — AD-5's one
// arrow, the one the spine names first and the one lint's `stage-rank`
// rule exists to keep absent.
//
// AD-24, verbatim, and it is directly load-bearing here: "An element's
// x/y is relative to its band's top-left corner, never to the page.
// Bands are placed on the page by internal/layout alone."
//
// Two properties this package holds and the arithmetic below makes
// structural:
//
//   - PLACEMENT IS A TRANSLATION, NEVER AN INVERSION (D-2.0.4). An
//     element's page Y is `band origin + element.Y` and nothing more.
//     Increasing an element's band-relative Y moves it DOWN the page,
//     because both this package and the page model are top-left origin
//     with Y increasing downward. The single inversion into a bottom-up
//     output space belongs to that renderer and happens in exactly one
//     function there (for PDF: internal/pdf's flipY, D-1.8.10). That
//     guard is scoped to internal/pdf and CANNOT see an inversion
//     introduced here, so this package asserts the property positively
//     in its own tests rather than relying on it.
//
//   - NOTHING NEGOTIATES (AD-24). Band geometry is a function of the
//     PAGE SETUP ALONE. ContentHeight below cannot consult the content
//     band's elements, because it is never given them: its only
//     parameter is PageGeometry. The content band does not grow to fit,
//     and content that does not fit overflows visibly — clipping and
//     overflow diagnostics are Story 2.8's (FR44, AD-14).
//
// The signal rides on the value, never through an import (D-000.16):
// everything this package needs arrives as a parameter.
package layout

import (
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
)

// PageGeometry is the COMPLETE set of geometric inputs band composition
// is allowed to see. It is deliberately a closed struct of page setup
// and nothing else: there is no field here through which a measured
// element could reach the derivation, which is how AD-24's "nothing
// negotiates" is enforced structurally rather than by a comment.
//
// All values are geom.Length millipoints (AD-2's one fixed-point unit).
// Heights are the DECLARED band heights from `bands.pageHeader.height`
// and `bands.pageFooter.height`; the content band declares no height —
// `bands.content.height` is a LOAD ERROR (internal/template's
// parse_bands.go, Story 1.4), because storing it would be a second
// source of truth for a derived quantity (AD-13's sibling rule).
type PageGeometry struct {
	Width, Height                      geom.Length
	MarginTop, MarginBottom            geom.Length
	MarginLeft, MarginRight            geom.Length
	PageHeaderHeight, PageFooterHeight geom.Length
}

// ContentHeight is THE one function that derives the content band's
// height (`folio-format.md`: "derived by one function"; D-2.0.4: "guard
// it positively at 2.5"):
//
//	pageHeight − marginTop − marginBottom − pageHeaderHeight − pageFooterHeight
//
// Its inputs are PAGE GEOMETRY ONLY. It does not receive, and cannot
// consult, the content band's elements or their measured sizes — see the
// package comment: AD-24's "nothing negotiates" is what makes a folio8
// report's boxes absolute and its output predictable.
//
// Integer arithmetic on geom.Length (int64 millipoints) throughout: no
// division, no rounding, no float (AD-23), so this quantity is exact and
// identical on every target.
func ContentHeight(g PageGeometry) geom.Length {
	return g.Height - g.MarginTop - g.MarginBottom - g.PageHeaderHeight - g.PageFooterHeight
}

// BandOrigins is where each band's own element-relative Y=0 sits,
// measured DOWNWARD from the page's top PRINTABLE edge (inside the top
// margin).
//
// The three are strictly increasing for any page whose bands have
// positive height, and the partition is exact: PageFooter is exactly
// Content plus the content band's derived height, so no millipoint of
// the printable column belongs to two bands or to none.
type BandOrigins struct {
	PageHeader geom.Length
	Content    geom.Length
	PageFooter geom.Length
}

// Origins resolves the three band origins from the page setup.
//
// The page-footer origin is expressed as the content band's origin plus
// its DERIVED height, rather than as a second subtraction from the page
// height. Both spellings produce the same integer, and this one is the
// honest one: it routes every content-height need through
// ContentHeight (AC5's positive "exactly one function"), and it says
// out loud that the footer starts exactly where the content band ends.
func Origins(g PageGeometry) BandOrigins {
	contentOrigin := g.PageHeaderHeight
	contentHeight := ContentHeight(g)
	return BandOrigins{
		PageHeader: 0,
		Content:    contentOrigin,
		PageFooter: contentOrigin + contentHeight,
	}
}

// PlaceInBand is AD-24's placement, and it is the whole of it: an
// element's page-absolute Y is its band's origin plus its own
// band-relative Y.
//
// It is a named function rather than a `+` written at each call site so
// that "band placement is a TRANSLATION, not an INVERSION" (D-2.0.4) is
// a property of one function that tests can hold, instead of a claim
// about arithmetic scattered across a caller. Nothing here subtracts a
// coordinate from a page height; nothing here negates. Moving an
// element down its band moves it down the page.
func PlaceInBand(bandOrigin, elementY geom.Length) geom.Length {
	return bandOrigin + elementY
}

// ComposePage produces the finished page model (AD-5) from content whose
// coordinates are ALREADY page-absolute — placed through PlaceInBand —
// plus the page geometry a renderer needs to reach its own space.
//
// It carries the page's own dimensions and its top/left margins because
// page-model coordinates are offsets from the printable corner, not from
// the paper corner. It does not carry band heights or band origins:
// bands are this package's business and stop here, which is what AD-24's
// "bands are placed on the page by internal/layout alone" means once
// there is a page model to place them into.
func ComposePage(g PageGeometry, runs []pagemodel.TextRun, images []pagemodel.ImagePlacement, rects []pagemodel.Rect) pagemodel.Page {
	return pagemodel.Page{
		Runs:       runs,
		Images:     images,
		Rects:      rects,
		Width:      g.Width,
		Height:     g.Height,
		MarginTop:  g.MarginTop,
		MarginLeft: g.MarginLeft,
	}
}
