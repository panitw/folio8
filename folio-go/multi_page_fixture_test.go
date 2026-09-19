package folio8

// Story 2.6's fixture, fixtures/multi-page/ — the FIRST document in the
// repository with more than one page, and the assertions that make a
// pagination defect visible.
//
// D-000.50 is why this fixture exists at all: measured across all seven
// pre-2.6 fixtures, ZERO are multi-page, one has a populated page header and
// two a populated page footer. A correct assertion that content spilling past
// the content band produces a second page is VACUOUS over every one of them,
// and vacuous invisibly, because the assertion itself reads as sound. The
// question comes before the assertion, not after it.
//
// D-000.45 is why nothing here asserts a direction. "A longer document has
// more pages" is monotone and is satisfied by a paginator that returns 2, 5
// or 900. Every expectation below is a HAND-DERIVED INTEGER with its
// arithmetic written out beside it.
//
// D-000.33 is why the line->page partition is pinned by VALUE and both parts
// are asserted non-empty. "Every line appears on exactly one page" is
// satisfied by the degenerate partition that puts all 29 lines on page 1 —
// which is precisely the pre-2.6 behaviour, i.e. THE DEFECT.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// The fixture's page setup, restated as the literals the assertions below
// compare against. Every one is HAND-DERIVED from
// fixtures/multi-page/input.folio and the arithmetic is written out.
//
//	page.size            A4          -> 595276 x 841890 mp
//	page.margin.top      30          ->  30000 mp
//	page.margin.bottom   42          ->  42000 mp
//	page.margin.left     36          ->  36000 mp
//	page.margin.right    54          ->  54000 mp
//	pageHeader.height    18          ->  18000 mp
//	pageFooter.height    24          ->  24000 mp
//
// SIX PAIRWISE-DISTINCT geometric inputs: 30000, 42000, 36000, 54000, 18000,
// 24000. No two equal, so no substitution among them and no swap of a pair
// can survive (Story 2.5's finding, one story on).
const (
	mpPageHeightMP   geom.Length = 841890
	mpMarginTopMP    geom.Length = 30000
	mpMarginBottomMP geom.Length = 42000
	mpMarginLeftMP   geom.Length = 36000
	mpMarginRightMP  geom.Length = 54000
	mpHeaderHeightMP geom.Length = 18000
	mpFooterHeightMP geom.Length = 24000

	// content band origin = pageHeaderHeight = 18000
	mpContentOriginMP geom.Length = 18000

	// content band height = 841890 − 30000 − 42000 − 18000 − 24000
	//                     = 727890
	mpContentHeightMP geom.Length = 727890

	// page footer origin = contentOrigin + contentHeight
	//                    = 18000 + 727890 = 745890
	mpFooterOriginMP geom.Length = 745890

	// The vertical model for chain ["Noto Sans"] at 24pt, which is exactly
	// twice the 12pt model (the scale is linear and exact in millipoints):
	//	max(ascent)  1069/1000 em -> 25656 mp at 24pt
	//	advance      1362/1000 em -> 32688 mp at 24pt   (lineGap is 0)
	//	max(descent)  293/1000 em ->  7032 mp at 24pt
	mpFirstBaselineMP geom.Length = 25656
	mpAdvanceMP       geom.Length = 32688
	mpLastDescentMP   geom.Length = 7032
)

// THE PARTITION, hand-derived.
//
// Line i occupies the column from `contentOrigin + i*advance` to
// `contentOrigin + i*advance + firstBaseline + lastDescent` — D-2.6.1's
// "baseline − max(ascent) .. baseline + max(descent)", where firstBaseline IS
// max(ascent). With advance == firstBaseline+lastDescent == 32688, line i
// spans exactly [18000 + i*32688, 18000 + (i+1)*32688].
//
// Window 0 is [18000 .. 745890], i.e. 727890 tall. Line i fits ENTIRELY iff
//
//	(i+1) * 32688 <= 727890   ->   i + 1 <= 22.27...   ->   i <= 21
//
// so lines 0..21 — TWENTY-TWO lines — fall on page 1. Line 22 spans
// 737136..769824, which is NOT contained in window 0, so it starts window 1
// at its own top, 737136. The remaining lines 22..28 — SEVEN — fall on
// page 2, and the document is TWO pages.
//
// Note what makes this a real test rather than a restatement: the pre-2.6
// code put all 29 lines on one page and drew lines 22..28 BELOW THE BOTTOM
// EDGE OF THE SHEET, at negative PDF Y, with no error and no warning.
const (
	mpTotalContentLines = 29
	mpLinesOnPage1      = 22
	mpLinesOnPage2      = 7
	mpTotalPages        = 2
)

// mpExpectedContentY is the declared PDF user-space Y of each content line,
// per page, derived from the constants above and NOT from the code under
// test.
//
//	pdfY = pageHeight − marginTop − placedY − baselineOffset
//
// and a line's placedY on its own page is `contentOrigin + k*advance` where k
// is its index WITHIN that page — because the window slides to begin at the
// first line that did not fit, so the first line of every page sits at the
// content band's top. That identity is itself a consequence of the ruled
// model and is worth seeing spelled out.
func mpExpectedContentY(indexWithinPage int) geom.Length {
	placedY := mpContentOriginMP + geom.Length(int64(indexWithinPage))*mpAdvanceMP
	return mpPageHeightMP - mpMarginTopMP - placedY - mpFirstBaselineMP
}

const (
	// header: placedY 4000 (band origin 0 + el.y 4000), baselineOffset 9621
	//	841890 − 30000 − 4000 − 9621 = 798269
	mpHeaderPDFYMP geom.Length = 798269

	// footer: placedY 751890 (footer origin 745890 + el.y 6000),
	// baselineOffset 8552
	//	841890 − 30000 − 751890 − 8552 = 51448
	mpFooterPDFYMP geom.Length = 51448

	mpHeaderText = "HEADER REPEATED ON EVERY PAGE"
	mpFooterText = "FOOTER REPEATED ON EVERY PAGE"
)

// mpDrawnRun is one BT/ET block: its placement operator and the text it
// extracts to through the document's OWN /ToUnicode CMap.
type mpDrawnRun struct {
	tm   string
	text string
}

// mpParseToUnicode decodes the document's /ToUnicode CMap, tolerating an
// entry that maps ONE CID to SEVERAL code points.
//
// WHY A SEPARATE PARSER RATHER THAN three_band_page_fixture_test.go's.
// parseToUnicodeCMap requires every bfchar destination to be a single BMP
// code point and FAILS otherwise. This fixture's prose contains "first",
// "file" and "fit", so the shaper draws the `fi` LIGATURE — one glyph whose
// source text is two runes ("00660069"). That is a correct and desirable
// property of the subject: it is exactly the glyph-vs-rune distinction
// Story 2.3 exists for, and it means the subset is built from what is DRAWN
// rather than from what was typed.
//
// The fixture was NOT rewritten to avoid the ligature. Distorting the
// document so an existing helper can read it would be fitting the subject to
// the instrument — and D-000.50's whole lesson is that the subject is what
// decides whether an assertion means anything.
//
// IT IS SINGLE-FACE ONLY, AND IT NOW REFUSES A MULTI-FACE DOCUMENT
// RATHER THAN LYING ABOUT ONE (D-4.7.6).
//
// THE DEFECT. This function merges EVERY embedded face's beginbfchar
// section into ONE cid -> text map. CIDs are per-face: face A's CID 3 and
// face B's CID 3 are different glyphs, and the loop below lets whichever
// section comes last in the file win. On a document with more than one
// embedded face the recovered text is therefore GARBAGE — measured during
// Story 4.7 on the Customer Account Statement, which embeds Noto Sans,
// Noto Sans Thai and Noto Sans SC: "Customer: Ada Lovelace" came back as
// "ไustomerะบdaบศovelace".
//
// WHY A FATAL AND NOT A COMMENT. Every caller before Story 4.7 was
// single-face, so the defect had never shown, and the failure mode is
// PLAUSIBLE-LOOKING TEXT rather than an error — a future caller gets a
// string, not a crash, and may well assert against it. There are eight
// call sites. A comment protects none of them; counting font resources
// is cheap and fails closed.
//
// THE TWO REPLACEMENTS, so the fatal is actionable (D-000.37):
// toUnicodeForResources (shaped_fixture_test.go) returns one CMap PER
// FONT RESOURCE, and parseContentStreamRuns returns each run with the
// resource that drew it. Composing the two decodes a multi-face document
// correctly; statement_semantics_test.go's statementPageRuns is the
// worked example.
func mpParseToUnicode(t *testing.T, b []byte) map[uint16]string {
	t.Helper()
	if _, type0s := resourceType0Objects(t, b); len(type0s) > 1 {
		names := make([]string, 0, len(type0s))
		for name := range type0s {
			names = append(names, name)
		}
		sort.Strings(names)
		t.Fatalf(
			"mpParseToUnicode was called on a document that embeds %d font faces (%v), and it CANNOT decode one correctly.\n\n"+
				"It merges every face's /ToUnicode beginbfchar section into a single cid -> text map, but CIDs are "+
				"PER FACE: the sections collide and the last one in the file wins. The result is not an error — it is "+
				"plausible-looking, confidently WRONG text. Measured on Story 4.7's Customer Account Statement, which "+
				"embeds three faces: \"Customer: Ada Lovelace\" decoded as \"ไustomerะบdaบศovelace\".\n\n"+
				"Use the per-resource instruments instead: toUnicodeForResources(t, b) gives one CMap per font resource, "+
				"and parseContentStreamRuns(t, stream) gives each run with the resource that drew it. Compose them — "+
				"statement_semantics_test.go's statementPageRuns is the worked example.",
			len(type0s), names,
		)
	}
	out := map[uint16]string{}
	src := string(b)
	idx := 0
	for {
		start := strings.Index(src[idx:], "beginbfchar")
		if start == -1 {
			break
		}
		start += idx + len("beginbfchar")
		stop := strings.Index(src[start:], "endbfchar")
		if stop == -1 {
			t.Fatal("document carries a beginbfchar with no endbfchar")
		}
		section := src[start : start+stop]
		idx = start + stop
		for _, line := range strings.Split(section, "\n") {
			var fields []string
			for _, f := range strings.Fields(line) {
				fields = append(fields, strings.Trim(f, "<>"))
			}
			if len(fields) != 2 || len(fields[0]) != 4 {
				continue
			}
			cid, ok := parseHex16(fields[0])
			if !ok {
				t.Fatalf("unrecognised /ToUnicode CID %q", fields[0])
			}
			var sb strings.Builder
			for i := 0; i+4 <= len(fields[1]); i += 4 {
				cp, ok := parseHex16(fields[1][i : i+4])
				if !ok {
					t.Fatalf("unrecognised /ToUnicode destination %q", fields[1])
				}
				sb.WriteRune(rune(cp))
			}
			out[cid] = sb.String()
		}
	}
	if len(out) == 0 {
		t.Fatal("presence precondition: the document carries no /ToUnicode bfchar entries — every text assertion built on decoding them would pass vacuously")
	}
	return out
}

// mpExtractRuns returns one run per BT/ET block of ONE PAGE'S content stream,
// in emission order, decoding each block's CIDs back to text through the
// document's own CMap — the
// property a READER actually gets, not the property the renderer was asked
// for (D-000.21: assert on the artifact that carries the property).
//
// IT TAKES ONE STREAM, NOT THE WHOLE DOCUMENT, and that is load-bearing on a
// multi-page document: the header's Tm operator is BYTE-IDENTICAL on every
// page — that is the property AC3 asserts — so asking "does the document
// contain this Tm" cannot tell you WHICH page it is on. Matching globally
// counted the header twice on a two-page document and reported 29 content
// lines on a page that carries 22.
func mpExtractRuns(t *testing.T, stream string, cidToText map[uint16]string) []mpDrawnRun {
	t.Helper()
	var out []mpDrawnRun
	for _, block := range strings.Split(stream, "BT\n")[1:] {
		end := strings.Index(block, "ET\n")
		if end == -1 {
			continue
		}
		var run mpDrawnRun
		var sb strings.Builder
		for _, line := range strings.Split(block[:end], "\n") {
			line = strings.TrimSpace(line)
			if strings.HasSuffix(line, " Tm") {
				run.tm = line
				continue
			}
			// TWO SHAPES, and missing the second is a silent hole rather
			// than a failure. internal/pdf emits `[<hex>-kern<hex>] TJ` for
			// a run that carries kerning adjustments and plain `<hex> Tj`
			// for one that does not — and which shape a run gets depends on
			// its TEXT. An extractor that handled only the array form
			// returned "" for every unkerned run, which reads exactly like
			// "this run drew nothing" and would let a footer that had
			// silently stopped being drawn pass as present.
			isTJ := strings.HasPrefix(line, "[") && strings.HasSuffix(line, "] TJ")
			isTj := strings.HasPrefix(line, "<") && strings.HasSuffix(line, "> Tj")
			if !isTJ && !isTj {
				continue
			}
			rest := line
			for {
				open := strings.Index(rest, "<")
				if open == -1 {
					break
				}
				shut := strings.Index(rest[open:], ">")
				if shut == -1 {
					break
				}
				hexRun := rest[open+1 : open+shut]
				for i := 0; i+4 <= len(hexRun); i += 4 {
					cid, ok := parseHex16(hexRun[i : i+4])
					if !ok {
						t.Fatalf("content stream carries a malformed CID %q", hexRun[i:i+4])
					}
					text, known := cidToText[cid]
					if !known {
						t.Fatalf("content stream emits CID %d, which the document's /ToUnicode CMap does not map — the text is unextractable", cid)
					}
					sb.WriteString(text)
				}
				rest = rest[open+shut:]
			}
		}
		run.text = sb.String()
		if run.tm != "" {
			out = append(out, run)
		}
	}
	if len(out) == 0 {
		t.Fatal("presence precondition: no BT/ET text block found in this page's content stream")
	}
	return out
}

func parseMultiPageTemplate(t *testing.T) *Template {
	t.Helper()
	tpl, err := ParseTemplate([]byte(multiPageTemplateJSON))
	if err != nil {
		t.Fatalf("ParseTemplate(multiPageTemplateJSON): %v", err)
	}
	return tpl
}

func renderMultiPage(t *testing.T) []byte {
	t.Helper()
	res, err := Render(parseMultiPageTemplate(t), Data("{}"), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("Render(multi-page): %v", err)
	}
	return res.Bytes
}

// mpKidsArrayRE and mpContentsRefRE are this file's OWN, narrow regexes for
// resolving the page tree — a value duplicate of matrix_test.go's
// matrixKidsArrayRE (this file carries no "matrix" build tag, so it cannot
// reference matrix_test.go's symbols; see matrixKidsArrayRE's own docblock
// for why the two files cannot share one copy).
var mpKidsArrayRE = regexp.MustCompile(`/Type /Pages /Kids \[([^\]]*)\] /Count (\d+)`)
var mpContentsRefRE = regexp.MustCompile(`/Contents (\d+) 0 R`)

// splitPageContentStreams returns each page's content stream, in the page
// tree's DECLARED order (its /Kids array).
//
// IT FOLLOWS REFERENCES, IT DOES NOT SCAN BYTES FOR TEXT (Story 2.6
// finisher, Finding 12). The previous version split the ENTIRE document on
// every ">>\nstream\n" boundary — including inside embedded font programs —
// and kept any resulting chunk containing the ASCII bytes "BT\n", on the
// premise that a font program would never happen to contain that triple.
// True on every fixture measured, but LUCK rather than scoping: a future
// subsetted program containing those three bytes would manufacture a
// phantom "page", reddening AC3/AC4/AC8 for a reason unconnected to
// pagination. This version instead resolves the page tree's /Kids array (a
// REFERENCE, the same technique requirePageTreeResolves in matrix_test.go
// and TestEveryGoldenPDFResolvesItsPageTree in
// golden_structural_validity_test.go use for the same reason) and follows
// each page object's OWN /Contents reference to its stream — a font
// program is never even visited, because nothing ever points a /Contents
// reference at one.
//
// folio8 never compresses a content stream (R4 stays shut, D-2.0.4), so the
// resolved stream bodies are readable in the bytes as written.
func splitPageContentStreams(t *testing.T, b []byte) []string {
	t.Helper()

	m := mpKidsArrayRE.FindSubmatch(b)
	if m == nil {
		t.Fatal("presence precondition: no page tree of the form \"/Type /Pages /Kids [...] /Count N\" found")
	}
	fields := bytes.Fields(m[1])
	if len(fields)%3 != 0 {
		t.Fatalf("/Kids array %q does not tokenize into whole \"N 0 R\" triples", m[1])
	}

	objs := pdfObjects(t, b)
	var out []string
	for i := 0; i+2 < len(fields); i += 3 {
		num, err := strconv.Atoi(string(fields[i]))
		if err != nil {
			t.Fatalf("unparseable page object number %q in /Kids", fields[i])
		}
		pageBody, ok := objs[num]
		if !ok {
			t.Fatalf("the page tree references object %d, which is never defined in the file", num)
		}
		cm := mpContentsRefRE.FindSubmatch(pageBody)
		if cm == nil {
			t.Fatalf("page object %d carries no /Contents reference", num)
		}
		contentNum, err := strconv.Atoi(string(cm[1]))
		if err != nil {
			t.Fatalf("unparseable /Contents object number %q for page %d", cm[1], num)
		}
		contentBody, ok := objs[contentNum]
		if !ok {
			t.Fatalf("page %d's /Contents references object %d, which is never defined in the file", num, contentNum)
		}
		s, ok := streamBody(contentBody)
		if !ok {
			t.Fatalf("object %d (page %d's /Contents) carries no stream body", contentNum, num)
		}
		out = append(out, string(s))
	}
	if len(out) == 0 {
		t.Fatal("presence precondition: the document carries no pages — every per-page assertion below would iterate over nothing")
	}
	return out
}

// PageContentStreams is splitPageContentStreams, reachable from
// package folio8_test.
//
// It exists for the same reason MatrixDocumentSlugsFromSource is
// exported, and the reasoning is worth repeating rather than
// re-discovered: byte_neutrality_test.go is package folio8_test, and a
// guard there that needs to look at what the emitter WROTE must look at
// the page content streams and nothing else. Re-implementing a stream
// walk over there would be a second parser with a second set of
// assumptions — and the assumption that matters is precisely the one a
// naive whole-file scan gets wrong: an embedded FontFile2 is arbitrary
// binary, so any operator's bytes can occur inside one by chance. This
// walker follows each page object's OWN /Contents reference, so a font
// program is never even visited.
//
// It is test-only source, so nothing is added to the module's public
// API.
func PageContentStreams(t *testing.T, b []byte) []string {
	t.Helper()
	return splitPageContentStreams(t, b)
}

// TestMultiPageGoldenFixtureMatchesTheInRepoTemplate is the precondition
// every other assertion in this file rests on: the constant compiled into the
// test binary and the committed fixture are THE SAME DOCUMENT. Without it,
// every assertion below describes a document nobody ships.
func TestMultiPageGoldenFixtureMatchesTheInRepoTemplate(t *testing.T) {
	path := filepath.Join(repoRootFromTest(t), "fixtures", "multi-page", "input.folio")
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("presence precondition: %s could not be read: %v", path, err)
	}
	if len(onDisk) == 0 {
		t.Fatal("presence precondition: fixtures/multi-page/input.folio is empty")
	}
	if string(onDisk) != multiPageTemplateJSON {
		t.Errorf("multiPageTemplateJSON and fixtures/multi-page/input.folio have DIVERGED.\n"+
			"in-repo constant: %d bytes\non disk:          %d bytes\n"+
			"They are kept byte-identical by hand (font-text's, wrapped-text's and three-band-page's "+
			"precedent). Every assertion in this file is about the constant; the golden is recorded from "+
			"the constant; the fixture is what a human reads. All three must be one document.",
			len(multiPageTemplateJSON), len(onDisk))
	}
}

// TestMultiPageRendersTheDeclaredNumberOfPages is AC2 at the document level.
//
// PRESENCE PRECONDITION: it first asserts the document's content genuinely
// EXCEEDS one content band. Without that, "it renders 2 pages" would be an
// assertion about a document that cannot express the defect — D-000.50's
// whole point.
func TestMultiPageRendersTheDeclaredNumberOfPages(t *testing.T) {
	// (1) The geometry is what the literals were derived from.
	g, err := pageGeometryOf(parseMultiPageTemplate(t))
	if err != nil {
		t.Fatalf("pageGeometryOf: %v", err)
	}
	for _, c := range []struct {
		name      string
		got, want geom.Length
	}{
		{"page height", g.Height, mpPageHeightMP},
		{"margin.top", g.MarginTop, mpMarginTopMP},
		{"margin.bottom", g.MarginBottom, mpMarginBottomMP},
		{"margin.left", g.MarginLeft, mpMarginLeftMP},
		{"margin.right", g.MarginRight, mpMarginRightMP},
		{"pageHeader.height", g.PageHeaderHeight, mpHeaderHeightMP},
		{"pageFooter.height", g.PageFooterHeight, mpFooterHeightMP},
	} {
		if c.got != c.want {
			t.Fatalf("presence precondition: the fixture's %s reads %d mp, want %d mp — the literals below were hand-derived from the declared setup and mean nothing against a different one", c.name, c.got, c.want)
		}
	}

	// (2) The six geometric inputs are PAIRWISE DISTINCT, or a substitution
	// among them is invisible however correct the assertions are.
	seen := map[geom.Length]string{}
	for _, c := range []struct {
		name string
		v    geom.Length
	}{
		{"margin.top", mpMarginTopMP}, {"margin.bottom", mpMarginBottomMP},
		{"margin.left", mpMarginLeftMP}, {"margin.right", mpMarginRightMP},
		{"pageHeader.height", mpHeaderHeightMP}, {"pageFooter.height", mpFooterHeightMP},
	} {
		if other, dup := seen[c.v]; dup {
			t.Fatalf("presence precondition: %s and %s are both %d — equal inputs make a SUBSTITUTION between them invisible in the bytes", other, c.name, c.v)
		}
		seen[c.v] = c.name
	}

	// (3) THE DOCUMENT ACTUALLY OVERFLOWS ONE CONTENT BAND. This is the
	// D-000.50 precondition: 0 of the 7 pre-2.6 fixtures could satisfy it.
	lowestBottom := mpContentOriginMP + geom.Length(int64(mpTotalContentLines))*mpAdvanceMP
	windowBottom := mpContentOriginMP + mpContentHeightMP
	if lowestBottom <= windowBottom {
		t.Fatalf("presence precondition: the fixture's content bottom is %d, which FITS the window ending at %d — this document cannot express a pagination defect", lowestBottom, windowBottom)
	}

	// (4) The declared page count, at two independent sites in the bytes.
	//
	// CORRECTED by the finisher (Finding 16): the /Count check used to
	// match the substring "] /Count 2 " literally — which reintroduces
	// the incidental-trailing-space pattern Nit 26 removed from
	// countPageObjects (render_test.go), succeeds only because the
	// emitter happens to write "/Count 2 >>" with a space, and hard-codes
	// "2" while the failure message interpolated mpTotalPages, so
	// changing that constant would make the assertion and its own
	// diagnostic disagree. readDeclaredCount (multi_page_composition_test.go)
	// parses the integer instead.
	b := renderMultiPage(t)
	if got := bytes.Count(b, []byte("<< /Type /Page /Parent ")); got != mpTotalPages {
		t.Errorf("the document emits %d /Type /Page objects; the declared value is %d", got, mpTotalPages)
	}
	if got := readDeclaredCount(t, b); got != mpTotalPages {
		t.Errorf("the page tree carries /Count %d; the declared value is %d", got, mpTotalPages)
	}

	// (5) NO TEXT IS DRAWN OFF THE SHEET. This is the defect itself,
	// asserted directly on the artifact: at the baseline of this story the
	// same document shape drew 62%% of its lines at a NEGATIVE PDF Y.
	for _, stream := range splitPageContentStreams(t, b) {
		for _, line := range strings.Split(stream, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasSuffix(line, " Tm") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			if strings.HasPrefix(fields[len(fields)-2], "-") {
				t.Errorf("a text placement is emitted at a NEGATIVE PDF Y (%q) — that is text drawn below the bottom edge of the sheet, which is the defect this story fixes", line)
			}
		}
	}
}

// TestMultiPageHeaderAndFooterAppearOnEveryPage is AC3.
//
// The header and footer strings are DISTINCT, IDENTIFIABLE LITERALS, so a
// band mix-up shows up in the rendered TEXT and not only in a coordinate.
//
// PRESENCE PRECONDITION: it asserts both bands are non-empty in the INPUT
// first, so the assertion cannot pass by finding nothing on either page.
func TestMultiPageHeaderAndFooterAppearOnEveryPage(t *testing.T) {
	tpl := parseMultiPageTemplate(t)
	if n := len(tpl.doc.Bands.PageHeader.Elements); n == 0 {
		t.Fatal("presence precondition: the fixture's pageHeader band is EMPTY — \"the header appears on every page\" is satisfied by finding nothing on every page")
	}
	if n := len(tpl.doc.Bands.PageFooter.Elements); n == 0 {
		t.Fatal("presence precondition: the fixture's pageFooter band is EMPTY — same vacuity, on the other band")
	}
	if mpHeaderText == mpFooterText {
		t.Fatal("presence precondition: the header and footer literals are identical, so a band mix-up would be invisible in the rendered text")
	}

	b := renderMultiPage(t)
	cmap := mpParseToUnicode(t, b)
	streams := splitPageContentStreams(t, b)
	if len(streams) != mpTotalPages {
		t.Fatalf("the document has %d text-bearing pages; the declared value is %d", len(streams), mpTotalPages)
	}

	wantHeaderTm := "1 0 0 1 36 " + formatMPForTm(mpHeaderPDFYMP) + " Tm"
	wantFooterTm := "1 0 0 1 36 " + formatMPForTm(mpFooterPDFYMP) + " Tm"

	for p, stream := range streams {
		headers, footers := 0, 0
		for _, r := range mpExtractRuns(t, stream, cmap) {
			switch r.text {
			case mpHeaderText:
				headers++
				if r.tm != wantHeaderTm {
					t.Errorf("page %d: the header is placed at %q; the declared placement is %q.\n"+
						"Hand-derived: 841890 − 30000 − 4000 − 9621 = 798269 mp. The header must sit at the "+
						"SAME Y on every page — page 34 is as complete as page 1.", p+1, r.tm, wantHeaderTm)
				}
			case mpFooterText:
				footers++
				if r.tm != wantFooterTm {
					t.Errorf("page %d: the footer is placed at %q; the declared placement is %q.\n"+
						"Hand-derived: 841890 − 30000 − 751890 − 8552 = 51448 mp.", p+1, r.tm, wantFooterTm)
				}
			}
		}
		if headers != 1 {
			t.Errorf("page %d carries %d header runs; it must carry exactly 1", p+1, headers)
		}
		if footers != 1 {
			t.Errorf("page %d carries %d footer runs; it must carry exactly 1", p+1, footers)
		}
	}
}

// TestMultiPageLineToPagePartitionIsPinnedByValue is AC4.
//
// The partition is asserted as a declared LINE INDEX -> PAGE INDEX table with
// each line's placement, and every page is asserted NON-EMPTY. D-000.33: a
// conservation law ("every line appears exactly once") is satisfied by the
// degenerate partition, and additivity is preserved by ANY monotone boundary
// function — so the boundary INDEX is asserted, never the sum.
func TestMultiPageLineToPagePartitionIsPinnedByValue(t *testing.T) {
	if mpLinesOnPage1+mpLinesOnPage2 != mpTotalContentLines {
		t.Fatalf("the declared partition is internally inconsistent: %d + %d != %d", mpLinesOnPage1, mpLinesOnPage2, mpTotalContentLines)
	}
	if mpLinesOnPage1 == 0 || mpLinesOnPage2 == 0 {
		t.Fatal("presence precondition: a declared partition with an empty part is degenerate — exactly what D-000.33 forbids asserting around")
	}

	b := renderMultiPage(t)
	cmap := mpParseToUnicode(t, b)
	streams := splitPageContentStreams(t, b)
	if len(streams) != mpTotalPages {
		t.Fatalf("the document has %d pages; the declared partition has %d", len(streams), mpTotalPages)
	}

	wantPerPage := []int{mpLinesOnPage1, mpLinesOnPage2}
	for p, stream := range streams {
		// The content runs on this page, in emission order, excluding the
		// two running bands.
		var contentTms []string
		for _, r := range mpExtractRuns(t, stream, cmap) {
			if r.text == mpHeaderText || r.text == mpFooterText {
				continue
			}
			contentTms = append(contentTms, r.tm)
		}
		if len(contentTms) != wantPerPage[p] {
			t.Errorf("page %d carries %d content lines; the declared partition puts %d there.\n"+
				"Hand-derived: window 0 is 727890 tall and a line is 32688, so (i+1)*32688 <= 727890 "+
				"holds for i <= 21 — 22 lines on page 1, the remaining 7 on page 2.",
				p+1, len(contentTms), wantPerPage[p])
			continue
		}
		// AC4's SECOND half — "every page in the partition is non-empty" —
		// is carried BY the check immediately above, not by a separate
		// branch: wantPerPage's declared values (22, 7) are both asserted
		// non-zero by this test's own presence precondition, so
		// len(contentTms) != wantPerPage[p] already fails before
		// len(contentTms) could ever equal a non-zero want AND be zero.
		// CORRECTED by the finisher (Finding 7): a separate "== 0" branch
		// used to sit here; wantPerPage[p] is never 0, so it was dead code
		// — reaching it required len(contentTms) == wantPerPage[p] == 0,
		// which the precondition above already rules out.
		//
		// Every line's placement, against the declared table.
		for k, tm := range contentTms {
			want := "1 0 0 1 36 " + formatMPForTm(mpExpectedContentY(k)) + " Tm"
			if tm != want {
				t.Errorf("page %d, line %d of that page: placed at %q; the declared placement is %q",
					p+1, k, tm, want)
			}
		}
	}
}

// formatMPForTm spells a millipoint value the way internal/pdf's number
// emitter does, so the declared literals above can be written as millipoints
// (the unit every other constant in this file uses) rather than as decimal
// point strings that would hide a unit error.
func formatMPForTm(v geom.Length) string {
	whole := int64(v) / 1000
	frac := int64(v) % 1000
	if frac == 0 {
		return itoaForTest(whole)
	}
	// The FRACTIONAL part is spelled unsigned: the sign already lives in
	// `whole` (or, for a value in (-1000, 0), belongs on the leading "0"
	// this function never emits — every declared Y this file formats is
	// non-negative, per AC2/AC3's own no-negative-Y assertion elsewhere in
	// this file). CORRECTED by the finisher (Finding 17): for v = -1500,
	// frac used to be itoaForTest(-1500%1000) = itoaForTest(-500) = "-500",
	// which survived padding and TrimRight("0") as "-5", producing the
	// malformed "-1.-5". abs(frac) makes the digits always [0-9]+.
	if frac < 0 {
		frac = -frac
	}
	s := itoaForTest(frac)
	for len(s) < 3 {
		s = "0" + s
	}
	s = strings.TrimRight(s, "0")
	return itoaForTest(whole) + "." + s
}

func itoaForTest(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var digits []byte
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

// TestMultiPageGoldenMatchesTheCommittedArtifact is AD-21's "every feature
// ships its golden", and it is the machine-checkable half of D-000.22's
// semantic acceptance — which D-000.44 makes non-deferrable for a RECORDING
// as much as for a re-recording.
// TestMultiPageIsAWellFormedPDF calls the module's ONE general
// well-formedness checker on the one subject that could not reach it.
//
// THIS IS THE REPAIR OF THE ACTUAL PROCESS DEFECT. assertWellFormedPDF
// existed, had 18 call sites, and hard-coded `pageCount != 1` — so it would
// have FATALLED on a multi-page document on the very property that made the
// document interesting. It was therefore never called on the first multi-page
// golden, which shipped with an unresolvable page tree.
//
// The instrument that could have looked encoded the single-page shape as an
// invariant, and was silently never invoked. Adding a new checker alongside it
// would not have fixed that; pointing the existing one at the subject does.
// Its checks are the ones the bespoke acceptance step lacked — startxref
// resolves, every xref entry points at its object, every stream length is
// exact — and they are about REACHABILITY, where every bespoke check measured
// PRESENCE.
func TestMultiPageIsAWellFormedPDF(t *testing.T) {
	assertWellFormedPDF(t, "multi-page fixture", renderMultiPage(t), mpTotalPages)

	// And on the COMMITTED bytes, not only on a fresh render: the artifact a
	// reader opens is the one that must be well formed, and it is a separate
	// file from whatever this process just produced.
	dir := filepath.Join(repoRootFromTest(t), "fixtures", "multi-page")
	committed, err := os.ReadFile(filepath.Join(dir, "expected.pdf"))
	if err != nil {
		t.Fatalf("presence precondition: the committed golden could not be read: %v", err)
	}
	assertWellFormedPDF(t, "multi-page committed golden", committed, mpTotalPages)
}

func TestMultiPageGoldenMatchesTheCommittedArtifact(t *testing.T) {
	dir := filepath.Join(repoRootFromTest(t), "fixtures", "multi-page")
	want, err := os.ReadFile(filepath.Join(dir, "expected.pdf"))
	if err != nil {
		t.Fatalf("presence precondition: fixtures/multi-page/expected.pdf could not be read: %v", err)
	}
	if len(want) == 0 {
		t.Fatal("presence precondition: fixtures/multi-page/expected.pdf is empty — two empty files are byte-identical")
	}

	got := renderMultiPage(t)
	if !bytes.Equal(got, want) {
		t.Errorf("the multi-page render no longer matches its committed golden (%d bytes produced, %d committed).\n"+
			"Under AD-22 a hash change is an INTENDED, VERSIONED event: identify the one cause, then re-record "+
			"and update every site in goldenDigestRecord. Do not re-record to make this green.", len(got), len(want))
	}

	// THE DIGEST IS DELIBERATELY NOT RESTATED HERE. goldenDigestRecord
	// (byte_neutrality_test.go) is THE LIST (D-000.47): it declares this
	// fixture's sha256 once, names every site that records it, and its
	// completeness half asserts that NO UNDECLARED site carries the value.
	// A fourth literal in this file would be exactly the undeclared site
	// that mechanism exists to make impossible.
}

// TestMultiPageRenderIsIdenticalAcrossTwoProcesses is the determinism half of
// the semantic acceptance step: the same input must produce the same bytes in
// a FRESH process, or the golden is pinning a map iteration order.
func TestMultiPageRenderIsIdenticalAcrossTwoProcesses(t *testing.T) {
	first := renderMultiPage(t)
	second := renderMultiPage(t)
	if !bytes.Equal(first, second) {
		t.Fatal("two renders in the same process disagree")
	}
	out := renderMultiPageInSubprocess(t, "1")
	if !bytes.Equal(first, out) {
		t.Errorf("the multi-page render differs between this process (%d bytes) and a fresh one (%d bytes) — a golden recorded from it would pin whatever this process happened to do", len(first), len(out))
	}
}

func renderMultiPageInSubprocess(t *testing.T, gomaxprocs string) []byte {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(),
		subprocessMultiPageEnvVar+"=1",
		"GOMAXPROCS="+gomaxprocs,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("subprocess multi-page render (GOMAXPROCS=%s) failed: %v\nstderr: %s", gomaxprocs, err, stderr.String())
	}
	if stdout.Len() == 0 {
		t.Fatal("presence precondition: the subprocess produced no bytes, so a comparison against it would prove nothing")
	}
	return stdout.Bytes()
}

// TestMultiPageToUnicodeSectionsAreUnderCap is DW-14's tripwire, extended to
// this document (AC11).
//
// DW-14: /ToUnicode emits ONE unbounded `beginbfchar` section, and ISO
// 32000-1 caps a section at 100 entries. Story 2.6's creator flagged a
// multi-page fixture as the first plausible trigger — the entry source is
// per-(glyph, cluster text) rather than per-glyph since Story 2.3, so a
// longer document has more entries.
//
// Measured here: 45 in one section, against the cap of 100. That is the
// LARGEST single section any fixture has recorded — above wrapped-text's 38
// and shaped-text's 28 — from a document that is only two pages and one
// face. The trend DW-14 tracks is confirmed.
//
// The check is a TEST rather than a note for the same reason wrapped-text's
// is: the remedy (chunking into <=100-entry sections) moves the golden hash
// of every document over the cap, so DW-14 asks that it land as a deliberate
// re-record rather than a drive-by. Crossing the cap must stop someone, not
// be noticed later.
func TestMultiPageToUnicodeSectionsAreUnderCap(t *testing.T) {
	assertToUnicodeSectionsUnderCap(t, renderMultiPage(t))
}
