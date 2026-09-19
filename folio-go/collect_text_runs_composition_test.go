package folio8

import (
	"testing"

	"github.com/panitw/folio8/folio-go/internal/layout"
)

// This story's review, Finding 8 (Minor): collectTextRuns (the
// pre-Story-2.7 "all bands, pass-through" implementation) has NO
// production caller after this story's two-phase restructure — only
// two shaped_fixture_test.go tests still call it, believing they
// exercise the run-collection path every document actually goes
// through. They do not: renderDocument now calls collectBandTextRuns
// THREE times (content first via contentBandResolver, to learn Y; then
// header and footer via headerFooterResolver) and concatenates as
// header, content, footer — a DIFFERENT call sequence from
// collectTextRuns' single loop over documentBands' order (header,
// content, footer, all through passthroughResolver).
//
// The two sequences happen to produce byte-identical runs for any
// document with NO {{page}}/{{pages}} construct (every pre-2.7 golden,
// which is what keeps them byte-identical): documentBands indexes
// header=0/content=1/footer=2 and renderDocument's own concatenation
// order is header/content/footer, so the CONTENT is reordered
// (collected first, positioned last) but the FINAL sequence coincides.
// That coincidence was, until this test, load-bearing and unasserted —
// if either order ever diverged, collectTextRuns' two remaining test
// callers would keep passing while every golden moved underneath them.
// This test turns the coincidence into a checked invariant.
func TestCollectTextRunsMatchesTheShippingBandComposition(t *testing.T) {
	tpl, err := ParseTemplate([]byte(multiPageTemplateJSON))
	if err != nil {
		t.Fatalf("presence precondition: the template does not parse: %v", err)
	}
	data := emptyBindValue(t)
	fs := testShippedFontSet()

	// The legacy, pre-Story-2.7 path: ONE loop over documentBands'
	// order, passthroughResolver throughout.
	legacyCache := newFontCache()
	legacy, lerr := collectTextRuns(tpl, data, data, fs, legacyCache)
	if lerr != nil {
		t.Fatalf("collectTextRuns: %v", lerr)
	}
	if len(legacy) == 0 {
		t.Fatal("presence precondition: collectTextRuns produced zero runs — this fixture must carry text in every band to say anything about composition order")
	}

	// The SHIPPING path, replicated exactly as renderDocument builds
	// it (render.go): content first (contentBandResolver, D-2.7.3's
	// fence — inert on a document with no {{page}}/{{pages}} construct),
	// then header and footer (headerFooterResolver — likewise inert
	// here), concatenated as header, content, footer.
	shipCache := newFontCache()
	bands, berr := documentBands(tpl)
	if berr != nil {
		t.Fatalf("documentBands: %v", berr)
	}
	contentRuns, _, _, cerr := collectBandTextRuns(tpl, bands, contentBandIndex, data, data, fs, shipCache, contentBandResolver, nil)
	if cerr != nil {
		t.Fatalf("collectBandTextRuns(content): %v", cerr)
	}
	// pageCount is irrelevant here (this fixture has no {{page}}/{{pages}}
	// construct, so headerFooterResolver's reservation logic never
	// fires), but a real value from Paginate is used anyway so this
	// test does not quietly depend on that being true forever.
	plan, perr := layout.Paginate(mustPageGeometry(t, tpl), contentColumnItems(contentRuns, nil, nil, nil, nil))
	if perr != nil {
		t.Fatalf("layout.Paginate: %v", perr)
	}
	headerRuns, _, _, herr := collectBandTextRuns(tpl, bands, pageHeaderBandIndex, data, data, fs, shipCache, headerFooterResolver(len(plan.Pages)), nil)
	if herr != nil {
		t.Fatalf("collectBandTextRuns(header): %v", herr)
	}
	footerRuns, _, _, ferr := collectBandTextRuns(tpl, bands, pageFooterBandIndex, data, data, fs, shipCache, headerFooterResolver(len(plan.Pages)), nil)
	if ferr != nil {
		t.Fatalf("collectBandTextRuns(footer): %v", ferr)
	}
	shipping := make([]textRunSource, 0, len(headerRuns)+len(contentRuns)+len(footerRuns))
	shipping = append(shipping, headerRuns...)
	shipping = append(shipping, contentRuns...)
	shipping = append(shipping, footerRuns...)

	if len(legacy) != len(shipping) {
		t.Fatalf("collectTextRuns produced %d run(s); the shipping composition produced %d — they must "+
			"agree, or the two remaining collectTextRuns test callers are exercising a run sequence no "+
			"document actually renders", len(legacy), len(shipping))
	}
	for i := range legacy {
		l, s := legacy[i], shipping[i]
		if l.face != s.face || l.text != s.text || l.x != s.x || l.y != s.y || l.band != s.band {
			t.Errorf("run %d diverges between collectTextRuns and the shipping composition:\n  legacy:   face=%q text=%q x=%d y=%d band=%d\n  shipping: face=%q text=%q x=%d y=%d band=%d",
				i, l.face, l.text, l.x, l.y, l.band, s.face, s.text, s.x, s.y, s.band)
		}
	}
}

// mustPageGeometry resolves tpl's page geometry via pageGeometryOf,
// fataling on error — a helper shared by no other test file because no
// other test needs it in isolation from renderDocument.
func mustPageGeometry(t *testing.T, tpl *Template) layout.PageGeometry {
	t.Helper()
	g, err := pageGeometryOf(tpl)
	if err != nil {
		t.Fatalf("pageGeometryOf: %v", err)
	}
	return g
}
