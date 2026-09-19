package folio8

// Story 2.6 finisher's fix for Finding 5 (AC Conformance / Major): AC2
// requires "for each row of a declared table of (page geometry, content
// extent) -> expected page count, the RENDERED DOCUMENT'S /Count and its
// number of /Type /Page objects both equal the declared integer."
//
// Before this fix that composition was verified end to end at exactly ONE
// point, N=2:
//
//   - internal/layout/paginate_test.go's declared table (geometry, content
//     extent) -> page count is excellent, but asserts len(plan.Pages), a Go
//     VALUE, never PDF bytes.
//   - internal/pdf/passtwo_pagecount_test.go's declared table asserts
//     /Count and /Type /Page against PRODUCED bytes, but its input is a
//     synthetic inputPages COUNT, not (geometry, content extent) — it
//     proves pass two is faithful to whatever pass one hands it, not that
//     pass one and pass two AGREE when wired together.
//   - multi_page_fixture_test.go asserts /Count on a rendered document, but
//     only for N=2.
//
// A defect in the SEAM between Paginate and SerializeTextDocument that
// manifested only at N=1, N=3 or N=0 was invisible to every test in the
// repository. This file closes that: it renders through the PUBLIC
// folio8.Render — pagination AND serialization AND everything between them
// — for the three page counts nothing else in the tree exercises.
//
// THE GEOMETRY IS THE SAME AS fixtures/multi-page/'s (multi_page_template.go
// derives it): page height 841890mp, margin.top 30000, margin.bottom 42000,
// pageHeader.height 18000, pageFooter.height 24000 -> content window
// 727890mp; at Noto Sans 24pt the per-line advance is 32688mp, so the
// window holds 22 lines (the same "(i+1)*32688 <= 727890 -> i <= 21"
// arithmetic multi_page_template.go's docblock already states).
//
// THE SENTENCE COUNTS BELOW WERE MEASURED, NOT HAND-DERIVED CHARACTER BY
// CHARACTER: word wrapping depends on word boundaries the per-line-advance
// arithmetic alone does not determine (that arithmetic fixes the VERTICAL
// rule; it says nothing about how many words of THIS text fit one line's
// WIDTH). Each row was chosen with headroom on both sides of its nearest
// page-count boundary — 15 repetitions renders one page and the next
// boundary (measured) is at 20; 40 repetitions renders three pages, between
// measured boundaries at 30 (two pages) and 60 (four pages) — so an
// unrelated, small change to line breaking cannot flip a row by accident.
// The zero-content row needs no measurement: an empty content band is
// AC2's own declared "no content at all is ONE page" case.

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"
)

// composeMultiPageTemplate builds a synthetic .folio document sharing
// fixtures/multi-page/'s geometry, with its content element's value set to
// sentenceCount repetitions of one fixed sentence (or no content element at
// all when sentenceCount is 0).
func composeMultiPageTemplate(sentenceCount int) string {
	const sentence = "The quick brown fox jumps over the lazy dog. "
	elements := "[]"
	if sentenceCount > 0 {
		value := ""
		for i := 0; i < sentenceCount; i++ {
			value += sentence
		}
		// %q is safe JSON-string escaping for this content: every byte in
		// `sentence` is a plain printable ASCII letter, space, or period,
		// none of which Go's %q or JSON's string syntax treats specially.
		elements = fmt.Sprintf(`[{"id": "e1", "type": "text", "x": 0, "y": 0, "width": 480, "height": 700, "value": %q, "style": {"fontFamily": "body", "fontSize": 24}}]`, value)
	}
	return `{
  "assets": {},
  "bands": {
    "content": {"elements": ` + elements + `},
    "pageFooter": {"elements": [], "height": 24},
    "pageHeader": {"elements": [], "height": 18}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

// readDeclaredCount parses the integer following the FIRST "] /Count "
// marker in b, the same technique internal/pdf/passtwo_pagecount_test.go
// uses (Story 2.6 finisher, Finding 16: parse the integer, do not match a
// brittle substring like "] /Count 3 " that only works because the emitter
// happens to write a trailing space).
func readDeclaredCount(t *testing.T, b []byte) int {
	t.Helper()
	marker := []byte("] /Count ")
	idx := bytes.Index(b, marker)
	if idx < 0 {
		t.Fatal("presence precondition: no \"] /Count \" in the produced bytes")
	}
	rest := b[idx+len(marker):]
	end := bytes.IndexByte(rest, ' ')
	if end < 0 {
		t.Fatal("malformed /Count in the produced bytes")
	}
	n, err := strconv.Atoi(string(rest[:end]))
	if err != nil {
		t.Fatalf("unparseable /Count %q: %v", rest[:end], err)
	}
	return n
}

// TestMultiPageComposedPageCountFromADeclaredTable is Finding 5's remedy:
// AC2's composition — geometry PLUS content extent, all the way through to
// PRODUCED BYTES — asserted at every page count the repository's other
// tests do not already cover end to end (N=2 is multi_page_fixture_test.go's
// alone).
func TestMultiPageComposedPageCountFromADeclaredTable(t *testing.T) {
	table := []struct {
		name          string
		sentenceCount int
		wantPages     int
	}{
		{"content that fits one page", 15, 1},
		{"a zero-content document is one page", 0, 1},
		{"content spanning three windows", 40, 3},
	}

	for _, row := range table {
		t.Run(row.name, func(t *testing.T) {
			tpl, err := ParseTemplate([]byte(composeMultiPageTemplate(row.sentenceCount)))
			if err != nil {
				t.Fatalf("presence precondition: the synthetic template does not even parse: %v", err)
			}
			res, err := Render(tpl, Data("{}"), nil, testShippedFontSet())
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			b := res.Bytes
			if len(b) == 0 {
				t.Fatal("presence precondition: Render returned zero bytes")
			}

			// Site 1: the number of /Type /Page objects.
			if got := countPageObjects(b); got != row.wantPages {
				t.Errorf("AC2: rendered %d /Type /Page object(s) for %d sentence(s) of content; the declared value is %d",
					got, row.sentenceCount, row.wantPages)
			}

			// Site 2: /Count in the page tree, independently (D-000.33 —
			// two independent sites, not a conservation law satisfied by
			// a serializer that gets both wrong the same way).
			if got := readDeclaredCount(t, b); got != row.wantPages {
				t.Errorf("AC2: /Count is %d for %d sentence(s) of content; the declared value is %d",
					got, row.sentenceCount, row.wantPages)
			}
		})
	}
}
