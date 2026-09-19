package layout

// internal/layout's own tests, and Finding 5 is why they exist rather
// than leaning on internal/pdf's flip guard.
//
// D-2.0.4 expects band placement to be "a translation, not an
// inversion", and calls Story 2.5 "its first real test against a
// genuinely different caller". Measured: that expectation is NOT
// automatically discharged. internal/pdf's
// TestContentStreamYCoordinatesRouteThroughFlipY scans internal/pdf
// ONLY — "no function in this package other than flipY may…" — so a
// coordinate inversion introduced inside internal/layout is outside its
// scan root entirely and it would never fire. AD-24 is therefore
// asserted POSITIVELY here, by value, in the package that could break
// it.

import (
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
)

// tbGeometry is fixtures/three-band-page/'s page setup, restated in
// millipoints. Its four geometric inputs are PAIRWISE DISTINCT — 30000,
// 42000, 18000, 24000 — which is what lets the assertions below detect a
// substitution among them. Every pre-2.5 fixture read 36000, 36000,
// 20000, 20000, where a swap of either pair produces identical output.
func tbGeometry() PageGeometry {
	return PageGeometry{
		Width:            595276,
		Height:           841890,
		MarginTop:        30000,
		MarginBottom:     42000,
		MarginLeft:       36000,
		MarginRight:      36000,
		PageHeaderHeight: 18000,
		PageFooterHeight: 24000,
	}
}

// TestPlaceInBandIsATranslationNotAnInversion is AD-24 and D-2.0.4's
// "a translation, not an inversion", asserted as the property that
// distinguishes the two rather than as an absence of minus signs.
//
// The distinguishing property is ORDER: under a translation, increasing
// an element's band-relative Y moves it FURTHER DOWN the page, and the
// distance between two elements is preserved exactly. Under an inversion
// the order reverses. A guard that searched for arithmetic could be
// satisfied by code that inverts with an addition; this cannot.
func TestPlaceInBandIsATranslationNotAnInversion(t *testing.T) {
	const origin geom.Length = 745890 // the fixture's page-footer origin

	// (a) Order is preserved: down the band is down the page.
	prev := PlaceInBand(origin, 0)
	for _, y := range []geom.Length{1, 1000, 6000, 120000, 727889} {
		got := PlaceInBand(origin, y)
		if got <= prev {
			t.Fatalf("PlaceInBand(%d, %d) = %d, which is not greater than the previous placement %d — placement must be a TRANSLATION: increasing an element's band-relative Y moves it DOWN the page. An INVERSION would reverse this order (D-2.0.4, AD-24)",
				origin, y, got, prev)
		}
		prev = got
	}

	// (b) Distance is preserved exactly: the offset from the band's own
	// zero is the element's own Y, unchanged. An inversion, or any
	// scaling, breaks this.
	base := PlaceInBand(origin, 0)
	for _, y := range []geom.Length{0, 1, 4000, 6000, 120000} {
		if got := PlaceInBand(origin, y) - base; got != y {
			t.Errorf("PlaceInBand(%d, %d) sits %d mp below the band's own zero, want exactly %d mp — band placement adds the band origin and does nothing else", origin, y, got, y)
		}
	}

	// (c) The band's own zero IS the band origin: no hidden offset.
	if got := PlaceInBand(origin, 0); got != origin {
		t.Errorf("PlaceInBand(%d, 0) = %d, want %d — an element at its band's top-left corner sits exactly at the band origin (AD-24: \"an element's x/y is relative to its band's top-left corner\")", origin, got, origin)
	}

	// (d) NOTHING is subtracted from a page height. A flip would produce
	// a placement that DECREASES as the page grows taller; a translation
	// is independent of page height altogether, which is exactly why
	// PlaceInBand is not given one.
	tall := tbGeometry()
	tall.Height *= 2
	if Origins(tall).PageHeader != Origins(tbGeometry()).PageHeader {
		t.Error("the page-header band origin moved when the page grew taller — it is the top of the printable column and depends on nothing else")
	}
	if Origins(tall).Content != Origins(tbGeometry()).Content {
		t.Error("the content band origin moved when the page grew taller — it is pageHeaderHeight and depends on nothing else")
	}
}

// TestContentHeightDependsOnGeometryAlone is AD-24's "nothing
// negotiates", asserted by the only means available to a test: vary
// each geometric input and confirm the derivation responds to exactly
// those, in exactly the direction and magnitude the rule states.
//
// The structural half — that the derivation CANNOT consult the content
// band's elements, because it is never handed them — is asserted by
// internal/bandcomposition_arch_test.go's
// TestContentHeightIsDerivedByExactlyOneFunction, which checks
// ContentHeight's parameter list. The two halves together are the claim.
func TestContentHeightDependsOnGeometryAlone(t *testing.T) {
	base := tbGeometry()

	// The hand-derived literal, with the arithmetic that produced it:
	//   841890 - 30000 - 42000 - 18000 - 24000 = 727890
	const want geom.Length = 727890
	if got := ContentHeight(base); got != want {
		t.Fatalf("ContentHeight = %d mp, want %d mp", got, want)
	}

	// Each of the four subtracted terms moves the answer by exactly its
	// own delta, and no two terms are interchangeable in magnitude —
	// which is only checkable because the four inputs are distinct.
	for _, c := range []struct {
		name   string
		mutate func(*PageGeometry)
		delta  geom.Length
	}{
		{"marginTop +5000", func(g *PageGeometry) { g.MarginTop += 5000 }, -5000},
		{"marginBottom +7000", func(g *PageGeometry) { g.MarginBottom += 7000 }, -7000},
		{"pageHeaderHeight +11000", func(g *PageGeometry) { g.PageHeaderHeight += 11000 }, -11000},
		{"pageFooterHeight +13000", func(g *PageGeometry) { g.PageFooterHeight += 13000 }, -13000},
		{"pageHeight +17000", func(g *PageGeometry) { g.Height += 17000 }, 17000},
	} {
		g := tbGeometry()
		c.mutate(&g)
		if got := ContentHeight(g); got != want+c.delta {
			t.Errorf("%s: ContentHeight = %d mp, want %d mp (%d %+d)", c.name, got, want+c.delta, want, c.delta)
		}
	}

	// Width and the horizontal margins are NOT in the derivation. A
	// content height that moved with them would be reading geometry it
	// has no business in.
	for _, c := range []struct {
		name   string
		mutate func(*PageGeometry)
	}{
		{"width", func(g *PageGeometry) { g.Width += 100000 }},
		{"marginLeft", func(g *PageGeometry) { g.MarginLeft += 100000 }},
		{"marginRight", func(g *PageGeometry) { g.MarginRight += 100000 }},
	} {
		g := tbGeometry()
		c.mutate(&g)
		if got := ContentHeight(g); got != want {
			t.Errorf("%s moved the content height to %d mp; the content band's height is a function of the VERTICAL page setup only", c.name, got)
		}
	}
}

// TestOriginsPartitionThePrintableColumnExactly asserts the three
// origins by hand-derived literal and as a strictly-increasing,
// non-degenerate partition — never as a sum-to-whole (D-000.33: "an
// additivity or conservation law is satisfied trivially by a degenerate
// partition").
func TestOriginsPartitionThePrintableColumnExactly(t *testing.T) {
	o := Origins(tbGeometry())

	if o.PageHeader != 0 {
		t.Errorf("page-header band origin = %d mp, want 0 mp", o.PageHeader)
	}
	if o.Content != 18000 {
		t.Errorf("content band origin = %d mp, want 18000 mp (= pageHeaderHeight)", o.Content)
	}
	if o.PageFooter != 745890 {
		t.Errorf("page-footer band origin = %d mp, want 745890 mp (= 841890 - 30000 - 42000 - 24000)", o.PageFooter)
	}

	if !(o.PageHeader < o.Content && o.Content < o.PageFooter) {
		t.Fatalf("band origins are not strictly increasing: %d, %d, %d — two bands sharing an origin is a DEGENERATE partition and must fail, not pass", o.PageHeader, o.Content, o.PageFooter)
	}

	// A degenerate page — every band height zero — must NOT satisfy the
	// strictly-increasing property. This is the control that keeps the
	// assertion above meaningful: if it held for the degenerate case too,
	// it would be detecting nothing.
	degenerate := PageGeometry{Height: 841890, MarginTop: 30000, MarginBottom: 42000}
	d := Origins(degenerate)
	if d.PageHeader < d.Content {
		t.Errorf("control failed: with a zero-height page header, the header and content origins must COINCIDE (%d, %d) — if the strictly-increasing check passed here as well it would be detecting nothing", d.PageHeader, d.Content)
	}
}

// TestComposePageCarriesPageAbsoluteContentUnchanged asserts that
// ComposePage is a carrier, not a second placement pass (AD-4: "two
// passes, and the second one lays nothing out"). It must not adjust,
// reorder or re-origin a single run.
func TestComposePageCarriesPageAbsoluteContentUnchanged(t *testing.T) {
	g := tbGeometry()
	runs := []pagemodel.TextRun{
		{Face: "Noto Sans", SourceText: "HEADER BAND ONLY", X: 0, Y: PlaceInBand(Origins(g).PageHeader, 4000), FontSize: 9000},
		{Face: "Noto Sans", SourceText: "FOOTER BAND ONLY", X: 0, Y: PlaceInBand(Origins(g).PageFooter, 6000), FontSize: 8000},
	}
	images := []pagemodel.ImagePlacement{{AssetKey: "a", X: 1000, Y: 2000, DrawWidth: 3000, DrawHeight: 4000}}
	rects := []pagemodel.Rect{{X: 5000, Y: 6000, W: 7000, H: 8000, HasFill: true, Fill: pagemodel.Color{R: 1, G: 2, B: 3}}}

	page := ComposePage(g, runs, images, rects)

	if len(page.Runs) != len(runs) || len(page.Images) != len(images) || len(page.Rects) != len(rects) {
		t.Fatalf("ComposePage carried %d runs, %d images and %d rects, want %d, %d and %d",
			len(page.Runs), len(page.Images), len(page.Rects), len(runs), len(images), len(rects))
	}
	for i := range runs {
		// Compared field by field rather than with == : TextRun carries a
		// glyph slice, which is not comparable. The fields below are the
		// ones a second placement pass would touch.
		if page.Runs[i].X != runs[i].X || page.Runs[i].Y != runs[i].Y ||
			page.Runs[i].FontSize != runs[i].FontSize ||
			page.Runs[i].Face != runs[i].Face ||
			page.Runs[i].SourceText != runs[i].SourceText {
			t.Errorf("ComposePage altered run %d: %+v, want %+v — it carries page-absolute content, it does not place it again (AD-4)", i, page.Runs[i], runs[i])
		}
	}
	if page.Images[0] != images[0] {
		t.Errorf("ComposePage altered the image placement: %+v, want %+v", page.Images[0], images[0])
	}
	if page.Rects[0] != rects[0] {
		t.Errorf("ComposePage altered the rect: %+v, want %+v", page.Rects[0], rects[0])
	}

	// The geometry a renderer needs to reach its own space, and no band
	// geometry at all: bands stop here.
	if page.Width != g.Width || page.Height != g.Height {
		t.Errorf("page dimensions = %dx%d, want %dx%d", page.Width, page.Height, g.Width, g.Height)
	}
	if page.MarginTop != g.MarginTop || page.MarginLeft != g.MarginLeft {
		t.Errorf("page margins = top %d left %d, want top %d left %d", page.MarginTop, page.MarginLeft, g.MarginTop, g.MarginLeft)
	}

	// Placement order is emission order: the header run precedes the
	// footer run, and its Y is smaller (further up a Y-down page).
	if !(page.Runs[0].Y < page.Runs[1].Y) {
		t.Errorf("the page-header run's Y (%d) is not above the page-footer run's (%d) — on a top-left origin with Y increasing downward, the header sits at a SMALLER Y", page.Runs[0].Y, page.Runs[1].Y)
	}
}
