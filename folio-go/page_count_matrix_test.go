package folio8

// Story 2.7, AC1/AC5: epics.md's fourth Story 2.7 acceptance criterion
// (cited by ordinal, not line range — this story's review, Finding 6)
// names documents of 1, 5, 20 and 50 pages, hashes matching on all four
// targets. page-count-20
// (matrix_test.go, matrixDocuments) carries the cross-target obligation
// — D-2.7.4's ruling on D-2.6.2's criterion. The other three sizes are
// verified BEHAVIOURALLY here, in the ordinary suite, without a
// separate matrix leg: an "implementation judgment" this story's own
// AC5 authorises explicitly ("whether all four become matrix documents
// or one document with the others as ordinary-suite subjects").
//
// Each document's page count is DETERMINISTIC BY CONSTRUCTION, not
// tuned line-wrapping (page_count_matrix_templates.go): N single-line
// content elements, each placed one content-band window below the
// last, so D-2.6.1's sliding window puts exactly one per page.

import (
	"os"
	"path/filepath"
	"testing"
)

// pageCountFixtures pairs each in-repo template constant with its
// fixture directory slug and declared page count — the single
// declarative list every test below drives, so a fifth size added
// later needs one new entry rather than four new functions.
var pageCountFixtures = []struct {
	slug  string
	json  string
	pages int
}{
	{"page-count-1", pageCount1TemplateJSON, 1},
	{"page-count-5", pageCount5TemplateJSON, 5},
	{"page-count-20", pageCount20TemplateJSON, 20},
	{"page-count-50", pageCount50TemplateJSON, 50},
}

// TestPageCountFixturesMatchTheInRepoTemplates keeps each in-repo
// template constant byte-identical to its fixtures/page-count-N/
// input.folio — multi-page's own precedent (font-text's, wrapped-
// text's, three-band-page's before it).
func TestPageCountFixturesMatchTheInRepoTemplates(t *testing.T) {
	root := repoRootFromTest(t)
	for _, f := range pageCountFixtures {
		path := filepath.Join(root, "fixtures", f.slug, "input.folio")
		onDisk, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: presence precondition: %s could not be read: %v", f.slug, path, err)
		}
		if len(onDisk) == 0 {
			t.Fatalf("%s: presence precondition: %s is empty", f.slug, path)
		}
		if string(onDisk) != f.json {
			t.Errorf("%s: the in-repo template constant and %s have DIVERGED (constant %d bytes, on disk %d bytes)",
				f.slug, path, len(f.json), len(onDisk))
		}
	}
}

// TestPageCountFixturesRenderCorrectPageNumbersEverywhere is AC1: X is
// the current page and Y the document total, correct on EVERY page,
// read off the PRODUCED content stream through the document's own
// /ToUnicode CMap (D-000.21: never off the input, never off "a
// substitution occurred").
//
// PRESENCE PRECONDITION: each document must render as exactly its
// declared page count, or nothing below says anything about X varying
// across pages (AC1's own instruction).
//
// VACUITY GUARD (AC1): the digit-count property is meaningless below
// N >= 10. page-count-20 and page-count-50 both cross the page-9-to-
// page-10 boundary; this loop checks EVERY page of EVERY document, so
// that boundary is checked by construction rather than by a
// hand-picked pair of pages.
func TestPageCountFixturesRenderCorrectPageNumbersEverywhere(t *testing.T) {
	for _, f := range pageCountFixtures {
		t.Run(f.slug, func(t *testing.T) {
			tpl, err := ParseTemplate([]byte(f.json))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			res, rerr := Render(tpl, Data("{}"), nil, testShippedFontSet())
			if rerr != nil {
				t.Fatalf("Render: %v", rerr)
			}
			b := res.Bytes
			streams := splitPageContentStreams(t, b)
			if len(streams) != f.pages {
				t.Fatalf("presence precondition: rendered %d page(s), want %d", len(streams), f.pages)
			}
			cmap := mpParseToUnicode(t, b)
			for p := 0; p < f.pages; p++ {
				want := "Page " + itoaForTest(int64(p+1)) + " of " + itoaForTest(int64(f.pages))
				found := false
				for _, run := range mpExtractRuns(t, streams[p], cmap) {
					if run.text == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("page %d does not draw %q anywhere in its content stream", p+1, want)
				}
			}
		})
	}
}
