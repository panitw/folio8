package folio8

// Story 2.6's AC8 built {{page}} and {{pages}} as a RESERVATION with no
// resolution: the two tests below pinned that "not yet" as a NEGATIVE
// property, deliberately, so a developer implementing pagination could
// not slip the one-line substitution in without a test noticing
// (Story 2.6's own file comment: "layout.Pagination literally
// enumerates the pages — and substituting it into a footer string is
// ONE LINE").
//
// STORY 2.7 IS THAT IMPLEMENTATION, and the file's own Story 2.6
// comment authorises exactly this inversion: "at which point this
// document's rendering legitimately changes and this test legitimately
// becomes 2.7's to rewrite." TestReservedPagePlaceholdersPassThroughOnEveryPage
// is renamed and inverted below to assert the RESOLVED text.
//
// D-000.34 governs what happens to TestReservedPlaceholderSetIsUnchanged.
// Its POSITIVE half — "{{page}}/{{pages}} pass through unchanged" — is
// now FALSE and is not weakened into "the resolved text appears": that
// rewrite is green on a renderer that has quietly stopped binding data
// entirely (AC3/AC4's hazard, named in this story's brief). The positive
// property this file used to carry — "page/pages are a CLOSED, DECLARED
// set, not ordinary data paths" — now lives structurally, in
// internal/bind's declared resolution-root set (AC3,
// TestBindResolutionRootsAreClosed in internal/bind), compared against
// the OBSERVED set in both directions, the same shape closedsets.go and
// declaredEpic2GateObligations already use for "this absence must stay
// absent" (D-2.5.1's mechanism). The NEGATIVE half here — a non-reserved
// placeholder must still resolve-or-error — SURVIVES UNCHANGED and still
// reddens: it is the only thing standing between "every placeholder now
// resolves correctly" and "every placeholder now resolves because
// nothing is checked any more."
//
// WHY IT ASSERTS OFF THE PRODUCED CONTENT STREAM AND NOT OFF THE INPUT.
// D-000.21 sharpened: assert on the artifact that carries the property. The
// property is what a READER of the PDF sees, so the literal characters are
// recovered from the drawn glyphs through the document's own /ToUnicode CMap.

import (
	"strings"
	"testing"
)

// reservedPlaceholderFooterTemplate is the multi-page fixture's geometry and
// content with ONE change: the page footer's value carries both reserved
// placeholders, in the shape a template author would naturally write.
//
// It is a SEPARATE document rather than an edit to fixtures/multi-page/ so
// that the committed golden was not re-recorded when Story 2.7 implemented
// these placeholders — fixtures/multi-page/'s footer stayed the fixed
// literal "FOOTER REPEATED ON EVERY PAGE" throughout, and this document
// is still the one that exercises {{page}}/{{pages}}. Two pages, so
// digits(Y) == 1 and every page's own digit count already equals Y's —
// this file's assertions therefore say nothing about D-2.7.2's
// right-alignment slack; the 20/50-page matrix documents (page_number_test.go)
// are where the digit-count boundary is exercised.
const reservedPlaceholderFooterTemplate = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 480, "height": 700, "value": "This statement covers the period from the first of January to the thirty-first of March. Every page of it carries the same running header and the same running footer, so a reader handed only the second sheet can still tell what they are holding and who issued it. The content below is one single text element. It is not divided into paragraphs, sections or boxes, and nothing in it has been repositioned in order to make it fit. It simply runs on until it reaches the bottom of the content band, and then it continues at the top of the next page, at exactly the leading it had before. No line is ever split across the boundary between two pages. A line that would otherwise fall half on one sheet and half on the next is placed whole on the next sheet instead, and the small band of whitespace left behind at the foot of the earlier page is correct typesetting rather than a defect. A bank statement cannot ship a half line. That single rule is what makes this document safe to print, safe to file, and safe to read aloud from. Nothing on any page negotiates for space with anything else, and no element ever moves sideways or shuffles upward to close a gap that pagination opened.", "style": {"fontFamily": "body", "fontSize": 24}}
      ]
    },
    "pageFooter": {
      "elements": [
        {"id": "e2", "type": "text", "x": 0, "y": 6, "width": 480, "height": 16, "value": "Page {{page}} of {{pages}}", "style": {"fontFamily": "body", "fontSize": 8}}
      ],
      "height": 24
    },
    "pageHeader": {
      "elements": [
        {"id": "e3", "type": "text", "x": 0, "y": 4, "width": 480, "height": 16, "value": "HEADER REPEATED ON EVERY PAGE", "style": {"fontFamily": "body", "fontSize": 9}}
      ],
      "height": 18
    }
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// TestReservedPagePlaceholdersResolveOnEveryPage is AC1/AC4: the
// inversion of Story 2.6's AC8 pass-through test. X is the current page
// and Y the document total, correct on EVERY page, read off the
// PRODUCED content stream through the document's own /ToUnicode CMap —
// never off the input, and never off "a substitution occurred" (AC1's
// own instruction: asserting on the input or on the page model does not
// carry the property).
//
// PRESENCE PRECONDITION: the document must render as N >= 2 pages
// before the assertion means anything, and this fixture's own footer
// text is what forces the hazard Story 2.6 built the negative control
// around — pagination hands the implementer a page index.
func TestReservedPagePlaceholdersResolveOnEveryPage(t *testing.T) {
	tpl, err := ParseTemplate([]byte(reservedPlaceholderFooterTemplate))
	if err != nil {
		t.Fatalf("presence precondition: the template does not parse: %v", err)
	}

	if !strings.Contains(reservedPlaceholderFooterTemplate, "{{page}}") ||
		!strings.Contains(reservedPlaceholderFooterTemplate, "{{pages}}") {
		t.Fatal("presence precondition: the template carries neither reserved placeholder")
	}

	res, rerr := Render(tpl, Data("{}"), nil, testShippedFontSet())
	if rerr != nil {
		t.Fatalf("Render: %v", rerr)
	}
	b := res.Bytes

	streams := splitPageContentStreams(t, b)
	if len(streams) < 2 {
		t.Fatalf("presence precondition: the document rendered as %d page(s); this assertion needs at least 2 to say anything about X varying across pages", len(streams))
	}

	cmap := mpParseToUnicode(t, b)
	for p, stream := range streams {
		want := "Page " + itoaForTest(int64(p+1)) + " of " + itoaForTest(int64(len(streams)))
		found := false
		var seen []string
		for _, run := range mpExtractRuns(t, stream, cmap) {
			seen = append(seen, run.text)
			if run.text == want {
				found = true
				break
			}
			if strings.Contains(run.text, "{{page}}") || strings.Contains(run.text, "{{pages}}") {
				t.Errorf("page %d still draws an UNRESOLVED reserved placeholder: %q — Story 2.7 resolves "+
					"{{page}}/{{pages}} in the page header and page footer bands", p+1, run.text)
			}
		}
		if !found {
			t.Errorf("page %d does not draw %q anywhere in its content stream (drew: %v)", p+1, want, seen)
		}
	}
}

// TestReservedPagePlaceholdersResolveTwoOccurrencesInOneElement is
// Blocker 1's red-proof (this story's review), on a REAL template
// rather than an argument — the ruling's own instruction (D-2.7.3's
// precedent). It is the reviewer's exact repro: a footer with a SECOND
// {{page}} occurrence after the ordinary "Page {{page}} of {{pages}}"
// construct. Because the whole footer is ASCII, all three occurrences
// resolve to ONE face segment on ONE line — a single TextRun. Before
// this story's review fix, TextRun carried a single PageSlot rather
// than a slice, so positionSegments' per-occurrence write silently
// OVERWROTE the first slot with the second: page 1 drew "Page 0 of 2 /
// 1" (the reservation's filler "0", not "1") and page 2 drew "Page 0
// of 2 / 2" — a plausible-looking WRONG page number, with no error, on
// every page.
func TestReservedPagePlaceholdersResolveTwoOccurrencesInOneElement(t *testing.T) {
	src := strings.Replace(reservedPlaceholderFooterTemplate,
		"Page {{page}} of {{pages}}", "Page {{page}} of {{pages}} / {{page}}", 1)
	tpl, err := ParseTemplate([]byte(src))
	if err != nil {
		t.Fatalf("presence precondition: the template does not parse: %v", err)
	}

	res, rerr := Render(tpl, Data("{}"), nil, testShippedFontSet())
	if rerr != nil {
		t.Fatalf("Render: %v", rerr)
	}
	b := res.Bytes

	streams := splitPageContentStreams(t, b)
	if len(streams) < 2 {
		t.Fatalf("presence precondition: the document rendered as %d page(s); this assertion needs at least 2 to say anything about X varying across pages", len(streams))
	}

	cmap := mpParseToUnicode(t, b)
	for p, stream := range streams {
		pageStr := itoaForTest(int64(p + 1))
		total := itoaForTest(int64(len(streams)))
		want := "Page " + pageStr + " of " + total + " / " + pageStr
		found := false
		var seen []string
		for _, run := range mpExtractRuns(t, stream, cmap) {
			seen = append(seen, run.text)
			if run.text == want {
				found = true
			}
			if strings.Contains(run.text, "Page 0 of") {
				t.Errorf("page %d drew the reservation's FILLER digit %q — the exact silent mis-render "+
					"Blocker 1 names: a run carrying two {{page}} occurrences resolved only the last one",
					p+1, run.text)
			}
		}
		if !found {
			t.Errorf("page %d does not draw %q anywhere in its content stream (drew: %v) — the FIRST "+
				"{{page}} occurrence in the element must resolve to the same page number as the second",
				p+1, want, seen)
		}
	}
}

// TestReservedPlaceholderSetIsUnchanged now carries ONLY the negative
// half (D-000.34, finding 7 of this story's creation). The positive
// half — "{{page}}/{{pages}} pass through unchanged" — is gone because
// it is no longer true, and it is NOT replaced by "the resolved text
// appears here": that rewrite is green on a renderer that has quietly
// stopped binding data at all, which is exactly the gap this test
// exists to close. The positive property that survives — page/pages
// are a CLOSED set, never ordinary data paths — is asserted
// structurally by internal/bind's TestBindResolutionRootsAreClosed
// (AC3), not behaviourally here.
func TestReservedPlaceholderSetIsUnchanged(t *testing.T) {
	src := strings.Replace(reservedPlaceholderFooterTemplate,
		"Page {{page}} of {{pages}}", "{{notreserved}}", 1)
	tpl, err := ParseTemplate([]byte(src))
	if err != nil {
		t.Fatalf("template does not parse: %v", err)
	}
	if _, rerr := Render(tpl, Data("{}"), nil, testShippedFontSet()); rerr == nil {
		t.Error("a NON-reserved placeholder {{notreserved}} rendered without error against empty data — " +
			"if every placeholder now resolves without checking whether it is a known data path, data " +
			"binding is silently disabled")
	}
}
