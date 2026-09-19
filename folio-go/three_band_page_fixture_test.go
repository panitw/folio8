package folio8

// Story 2.5's fixture, fixtures/three-band-page/ — the document with
// content in ALL THREE bands and four pairwise-distinct geometric
// inputs, and the assertions that make a band-composition defect
// visible.
//
// AC6 bans the CONSERVATION form. `headerHeight + contentHeight +
// footerHeight == usableHeight` is NOT asserted here and must never be.
// D-000.33: "an additivity or conservation law is satisfied trivially
// by a degenerate partition", and D-000.36 measured that the obvious
// remedy — a non-emptiness precondition — ALSO stays green, because
// additivity is preserved by ANY MONOTONE boundary function. A
// conservation assertion over band heights can detect a SATURATING
// error only. Four independent literals over four distinct inputs
// detect a SUBSTITUTION (a footer height read where a header height
// belongs) and a SWAP, which is the error class this story can
// actually commit.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/layout"
)

// The fixture's page setup, restated here as the literals the
// assertions below compare against. Every one is HAND-DERIVED from
// fixtures/three-band-page/input.folio, and the arithmetic that
// produced it is written out beside it. Nothing here is computed from
// the code under test.
//
// From input.folio, converted to millipoints by AD-2's one fixed-point
// unit (points x 1000):
//
//	page.size            A4          -> height 841890 mp
//	page.margin.top      30          ->  30000 mp
//	page.margin.bottom   42          ->  42000 mp
//	page.margin.left     36          ->  36000 mp
//	pageHeader.height    18          ->  18000 mp
//	pageFooter.height    24          ->  24000 mp
//
// PAIRWISE DISTINCT, which is the whole point of choosing them:
// 30000, 42000, 18000, 24000 — no two equal. In every pre-2.5 fixture
// these read 36000, 36000, 20000, 20000, so a swap of either pair
// produced identical bytes and no assertion over them could fail.
const (
	tbPageHeightMP   geom.Length = 841890
	tbMarginTopMP    geom.Length = 30000
	tbMarginBottomMP geom.Length = 42000
	tbMarginLeftMP   geom.Length = 36000
	tbHeaderHeightMP geom.Length = 18000
	tbFooterHeightMP geom.Length = 24000

	// pageHeader band origin: the top of the printable column.
	tbHeaderOriginMP geom.Length = 0

	// content band origin = pageHeaderHeight
	//                     = 18000
	tbContentOriginMP geom.Length = 18000

	// content band height = pageHeight - marginTop - marginBottom
	//                       - pageHeaderHeight - pageFooterHeight
	//                     = 841890 - 30000 - 42000 - 18000 - 24000
	//                     = 727890
	tbContentHeightMP geom.Length = 727890

	// pageFooter band origin = pageHeight - marginTop - marginBottom
	//                          - pageFooterHeight
	//                        = 841890 - 30000 - 42000 - 24000
	//                        = 745890
	// (identically: contentOrigin + contentHeight = 18000 + 727890.)
	tbFooterOriginMP geom.Length = 745890
)

func parseThreeBandPageTemplate(t *testing.T) *Template {
	t.Helper()
	tpl, err := ParseTemplate([]byte(threeBandPageTemplateJSON))
	if err != nil {
		t.Fatalf("ParseTemplate(threeBandPageTemplateJSON): %v", err)
	}
	return tpl
}

func renderThreeBandPage(t *testing.T) []byte {
	t.Helper()
	res, err := Render(parseThreeBandPageTemplate(t), Data("{}"), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("Render(three-band-page): %v", err)
	}
	return res.Bytes
}

// TestThreeBandPagePartitionIsPinnedByValue is AC6: the band partition
// asserted as FOUR INDEPENDENT HAND-DERIVED LITERALS over four
// pairwise-distinct geometric inputs, never as a sum-to-whole.
//
// PRESENCE PRECONDITION (D-000.21 sharpened): before comparing any
// origin, it asserts the four geometric INPUTS actually read as the
// distinct values the fixture declares. Comparing derived numbers
// against literals is meaningless if the inputs the derivation saw were
// not the inputs the literals were derived from — and, specifically,
// four inputs that were accidentally equal (the pre-2.5 state) would
// make every substitution invisible while every assertion still passed.
func TestThreeBandPagePartitionIsPinnedByValue(t *testing.T) {
	g, err := pageGeometryOf(parseThreeBandPageTemplate(t))
	if err != nil {
		t.Fatalf("pageGeometryOf: %v", err)
	}

	// (1) The inputs are what the literals were derived from.
	for _, c := range []struct {
		name string
		got  geom.Length
		want geom.Length
	}{
		{"page height", g.Height, tbPageHeightMP},
		{"margin.top", g.MarginTop, tbMarginTopMP},
		{"margin.bottom", g.MarginBottom, tbMarginBottomMP},
		{"margin.left", g.MarginLeft, tbMarginLeftMP},
		{"pageHeader.height", g.PageHeaderHeight, tbHeaderHeightMP},
		{"pageFooter.height", g.PageFooterHeight, tbFooterHeightMP},
	} {
		if c.got != c.want {
			t.Fatalf("presence precondition: fixture's %s reads %d mp, want %d mp — the literals below were hand-derived from the fixture's declared setup and mean nothing against a different one", c.name, c.got, c.want)
		}
	}

	// (2) The four geometric inputs are PAIRWISE DISTINCT. Without this
	// the assertions in (3) are correct assertions over a subject that
	// cannot express a substitution (D-000.36's corollary: the subject
	// matters as much as the assertion).
	distinct := map[geom.Length]string{}
	for _, c := range []struct {
		name string
		v    geom.Length
	}{
		{"margin.top", g.MarginTop},
		{"margin.bottom", g.MarginBottom},
		{"pageHeader.height", g.PageHeaderHeight},
		{"pageFooter.height", g.PageFooterHeight},
	} {
		if prior, clash := distinct[c.v]; clash {
			t.Fatalf("AC7 violated: %s and %s are both %d mp — the four geometric inputs must be PAIRWISE DISTINCT, or a substitution among them is invisible in the bytes (measured: every pre-2.5 fixture had marginTop == marginBottom and pageHeader.height == pageFooter.height)", prior, c.name, c.v)
		}
		distinct[c.v] = c.name
	}

	// (3) The four independent numbers, each against its hand-derived
	// literal. NOT a sum, NOT a conservation law (D-000.33).
	origins := layout.Origins(g)
	if origins.PageHeader != tbHeaderOriginMP {
		t.Errorf("page-header band origin = %d mp, want %d mp (the top of the printable column). Story 2.5's creator measured that a WRONG page-header origin was detected by ZERO tests in this repository before this fixture existed", origins.PageHeader, tbHeaderOriginMP)
	}
	if origins.Content != tbContentOriginMP {
		t.Errorf("content band origin = %d mp, want %d mp (= pageHeader.height, 18000)", origins.Content, tbContentOriginMP)
	}
	if origins.PageFooter != tbFooterOriginMP {
		t.Errorf("page-footer band origin = %d mp, want %d mp (= 841890 - 30000 - 42000 - 24000)", origins.PageFooter, tbFooterOriginMP)
	}
	if h := layout.ContentHeight(g); h != tbContentHeightMP {
		t.Errorf("content band height = %d mp, want %d mp (= 841890 - 30000 - 42000 - 18000 - 24000)", h, tbContentHeightMP)
	}

	// (3b) MEASURED, and reported rather than assumed (AC6: "if any of the
	// four does not redden, that is a finding, not a formality").
	//
	// AC6's red-proof 2 — swap marginTop and marginBottom — was run in two
	// forms, because "swap" has two sites:
	//
	//   at the READING site (render.go's pageGeometryOf, MarginTop fed from
	//   page.margin.bottom and vice versa): REDDENS. Caught by (1) above,
	//   by all four content-stream Y literals, and by the golden digest.
	//   This is the real substitution defect, and it is closed.
	//
	//   inside the DERIVATION (ContentHeight's own expression, written
	//   `g.Height - g.MarginBottom - g.MarginTop - …`): STAYS GREEN, on all
	//   three tests. It stays green because it is not a defect: subtraction
	//   of a commutative sum is reassociation, `a-b-c == a-c-b` exactly, so
	//   no number and no byte changes. That is D-000.33's observation one
	//   level up — a partition's boundary arithmetic cannot distinguish two
	//   terms that enter it only as a SUM — and it is why the four margins
	//   are additionally pinned INDIVIDUALLY by literal in (1) and here,
	//   rather than trusted to the derived numbers.
	//
	// margin.top is pinned once more here, standing apart from (1)'s input
	// table, because it is the one margin a renderer's flip consumes
	// directly: it reaches output bytes on its own, not only through the
	// content height.
	if g.MarginTop != tbMarginTopMP {
		t.Errorf("page-model margin.top = %d mp, want %d mp", g.MarginTop, tbMarginTopMP)
	}

	// (4) The three origins are strictly increasing, so a DEGENERATE
	// partition — any two bands sharing an origin, or a band collapsing
	// to zero — is a failure rather than a silent pass.
	if !(origins.PageHeader < origins.Content && origins.Content < origins.PageFooter) {
		t.Errorf("band origins are not strictly increasing: pageHeader=%d content=%d pageFooter=%d — two bands sharing an origin, or a band of zero height, is a degenerate partition and must fail",
			origins.PageHeader, origins.Content, origins.PageFooter)
	}
}

// tbExpectedTm is the hand-computed content-stream placement of every
// text run this document emits, in the order the runs are produced
// (documentBands' authored order: pageHeader, content, pageFooter).
//
// internal/pdf places a baseline at
//
//	pdfX = marginLeft + run.X
//	pdfY = flipY(pageHeight, marginTop, run.Y, run.BaselineOffset)
//	     = pageHeight - marginTop - run.Y - run.BaselineOffset
//
// and run.Y is layout.PlaceInBand(bandOrigin, element.y) — a
// translation.
//
// THE FOURTH TERM IS NO LONGER run.FontSize (Story 2.5a). It is the
// ruled vertical model's first span: max(hhea ascent) over the
// element's DECLARED chain, scaled to the point size. This document
// declares ["Noto Sans"] alone, whose hhea ascent is 1069/1000 em, so
// each size gets its own offset:
//
//	size  9 pt -> 1069 *  9 =  9621 mp
//	size 12 pt -> 1069 * 12 = 12828 mp
//	size  8 pt -> 1069 *  8 =  8552 mp
//
// THREE DISTINCT OFFSETS for the three sizes, which is what keeps the
// "four distinct placements" assertion below non-vacuous after the
// change (AC14 / D-000.34: a test built on a bug dies with the bug,
// silently, because it goes on passing).
//
// Working each one out, in millipoints, and then in the
// points-with-trimmed-zeros form appendLength emits:
//
//	e4  pageHeader  y=4    size 9   run.Y = 0      + 4000   =   4000
//	    pdfY = 841890 - 30000 -   4000 -  9621 = 798269 -> "798.269"
//	e1  content     y=0    size 12  run.Y = 18000  + 0      =  18000
//	    pdfY = 841890 - 30000 -  18000 - 12828 = 781062 -> "781.062"
//	e2  content     y=120  size 12  run.Y = 18000  + 120000 = 138000
//	    pdfY = 841890 - 30000 - 138000 - 12828 = 661062 -> "661.062"
//	e3  pageFooter  y=6    size 8   run.Y = 745890 + 6000   = 751890
//	    pdfY = 841890 - 30000 - 751890 -  8552 =  51448 -> "51.448"
//
//	pdfX = 36000 + 0 = 36000 -> "36" for all four.
//
// Three DISTINCT band origins are represented (0, 18000, 745890) and
// each of the three bands contributed at least one run — without which
// every band-origin assertion is vacuous, which is exactly the state
// Finding 2 measured the repository to be in.
var tbExpectedTm = []struct {
	element string
	band    string
	tm      string
	text    string
}{
	{"e4", "pageHeader", "1 0 0 1 36 798.269 Tm", "HEADER BAND ONLY"},
	{"e1", "content", "1 0 0 1 36 781.062 Tm", "CONTENT BAND FIRST ELEMENT"},
	{"e2", "content", "1 0 0 1 36 661.062 Tm", "CONTENT BAND SECOND ELEMENT"},
	{"e3", "pageFooter", "1 0 0 1 36 51.448 Tm", "FOOTER BAND ONLY"},
}

// TestThreeBandPageSemanticAcceptance is AC7's machine-checkable
// acceptance at first recording (D-000.22), and D-2.3.5's rule that
// this half is NEVER deferrable. It runs before the hash is compared
// and does not consult it: a hash over bytes nobody has read is Story
// 1.1's "two empty files are byte-identical".
func TestThreeBandPageSemanticAcceptance(t *testing.T) {
	b := renderThreeBandPage(t)

	// Well-formed, one page, and NOT blank.
	assertWellFormedPDF(t, "three-band-page", b, 1)
	if !containsFontFile2(b) {
		t.Fatal("three-band-page output contains no /FontFile2 — a hash over a document that embedded no face would certify nothing about band composition either")
	}

	// Every run's emitted placement equals its hand-computed literal AND
	// carries its own band's distinct, identifiable string, decoded back
	// through the document's own /ToUnicode CMap. Pairing the two in one
	// assertion is the point: a band mix-up then shows up as the WRONG
	// TEXT AT A GIVEN PLACE, which a coordinate-only assertion cannot
	// distinguish from a coordinate that merely moved.
	//
	// Presence precondition first: without it, "every expected run
	// matched" passes on an empty stream of runs.
	runs := extractDrawnRuns(t, b)
	if len(runs) != len(tbExpectedTm) {
		t.Fatalf("presence precondition: the content stream carries %d drawn text runs, want exactly %d (one per text element: e4 header, e1+e2 content, e3 footer). Got: %+v",
			len(runs), len(tbExpectedTm), runs)
	}
	for i, want := range tbExpectedTm {
		if runs[i].tm != want.tm {
			t.Errorf("run %d (%s, %s band): content-stream placement is %q, want the hand-computed %q",
				i, want.element, want.band, runs[i].tm, want.tm)
		}
		if runs[i].text != want.text {
			t.Errorf("run %d (%s, %s band) at %q extracts as %q, want %q — each band's content must be identifiable, so a band mix-up is visible in the rendered TEXT and not only in a coordinate",
				i, want.element, want.band, runs[i].tm, runs[i].text, want.text)
		}
	}

	// Exactly THREE distinct band origins are represented among the
	// emitted runs, and each band contributed at least one — without
	// which every band-origin assertion over this document is vacuous,
	// which is exactly the state Finding 2 measured the repository to be
	// in (all six pre-2.5 fixtures have an EMPTY pageHeader band).
	distinctPlacement := map[string]bool{}
	byBand := map[string]int{}
	for i, want := range tbExpectedTm {
		distinctPlacement[runs[i].tm] = true
		byBand[want.band]++
	}
	if len(distinctPlacement) != 4 {
		t.Errorf("the four runs produced %d distinct placements, want 4 — two runs sharing a placement would hide a collapsed band", len(distinctPlacement))
	}
	for _, band := range []string{"pageHeader", "content", "pageFooter"} {
		if byBand[band] == 0 {
			t.Errorf("band %q contributed ZERO runs — every band-origin assertion over this document would be vacuous", band)
		}
	}
}

// drawnRun is one BT/ET text-showing block: the absolute text matrix it
// was placed with, and the text it extracts as.
type drawnRun struct {
	tm   string
	text string
}

// extractDrawnRuns reads the document's single uncompressed content
// stream and returns one drawnRun per BT/ET block, in emission order,
// decoding each block's CIDs back to text through the document's OWN
// /ToUnicode CMap.
//
// Going through /ToUnicode rather than comparing against the source
// string directly is deliberate: it asserts the property a reader
// actually gets (the text extracts correctly) rather than the property
// the renderer was asked for (D-000.21 — assert on the artifact that
// carries the property). folio8 never compresses a content stream (R4
// stays shut, D-2.0.4), so both are readable in the bytes as written.
func extractDrawnRuns(t *testing.T, b []byte) []drawnRun {
	t.Helper()
	cidToRune := parseToUnicodeCMap(t, b)
	if len(cidToRune) == 0 {
		t.Fatal("presence precondition: the document carries no /ToUnicode bfchar entries — every text assertion built on decoding them would pass vacuously")
	}

	var out []drawnRun
	src := string(b)
	for _, block := range strings.Split(src, "BT\n")[1:] {
		end := strings.Index(block, "ET\n")
		if end == -1 {
			continue
		}
		block = block[:end]

		var run drawnRun
		for _, line := range strings.Split(block, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasSuffix(line, " Tm") {
				run.tm = line
				continue
			}
			if !strings.HasPrefix(line, "[<") || !strings.HasSuffix(line, "] TJ") {
				continue
			}
			// Concatenate every <hex> segment in the TJ array, skipping
			// the inter-segment kern adjustments.
			var sb strings.Builder
			rest := line
			for {
				open := strings.Index(rest, "<")
				if open == -1 {
					break
				}
				close := strings.Index(rest[open:], ">")
				if close == -1 {
					break
				}
				hex := rest[open+1 : open+close]
				for i := 0; i+4 <= len(hex); i += 4 {
					cid, ok := parseHex16(hex[i : i+4])
					if !ok {
						t.Fatalf("content stream carries a malformed CID %q", hex[i:i+4])
					}
					r, known := cidToRune[cid]
					if !known {
						t.Fatalf("content stream emits CID %d, which the document's /ToUnicode CMap does not map — the text is unextractable", cid)
					}
					sb.WriteRune(r)
				}
				rest = rest[open+close:]
			}
			run.text = sb.String()
		}
		if run.tm != "" {
			out = append(out, run)
		}
	}
	return out
}

// parseToUnicodeCMap reads the document's `<cid> <utf16be>` bfchar
// entries. Only the single-BMP-code-point form folio8 emits is handled;
// anything else fails the test rather than being skipped, so an
// unrecognised entry can never silently shrink the map.
func parseToUnicodeCMap(t *testing.T, b []byte) map[uint16]rune {
	t.Helper()
	out := map[uint16]rune{}
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
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var fields []string
			for _, f := range strings.Fields(line) {
				fields = append(fields, strings.Trim(f, "<>"))
			}
			if len(fields) != 2 || len(fields[0]) != 4 {
				t.Fatalf("unrecognised /ToUnicode bfchar entry %q", line)
			}
			cid, ok := parseHex16(fields[0])
			if !ok {
				t.Fatalf("unrecognised /ToUnicode CID %q", fields[0])
			}
			if fields[1] == "" {
				// A glyph that contributes no text of its own (the
				// cluster-text rule, D-2.3-Q1). Real information, and
				// this all-Latin document should carry none of them.
				out[cid] = 0
				continue
			}
			if len(fields[1]) != 4 {
				t.Fatalf("/ToUnicode entry for CID %d is %q; this fixture is all-Latin and must map each CID to exactly one BMP code point", cid, fields[1])
			}
			cp, ok := parseHex16(fields[1])
			if !ok {
				t.Fatalf("unrecognised /ToUnicode destination %q", fields[1])
			}
			out[cid] = rune(cp)
		}
	}
	return out
}

func parseHex16(s string) (uint16, bool) {
	var v uint16
	if len(s) != 4 {
		return 0, false
	}
	for i := 0; i < 4; i++ {
		c := s[i]
		var d uint16
		switch {
		case c >= '0' && c <= '9':
			d = uint16(c - '0')
		case c >= 'a' && c <= 'f':
			d = uint16(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = uint16(c-'A') + 10
		default:
			return 0, false
		}
		v = v<<4 | d
	}
	return v, true
}

// TestThreeBandPageGoldenFixture is AD-21's golden: the recorded
// document's digest, compared only after the semantic assertions above
// have said what the bytes ARE.
func TestThreeBandPageGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", "three-band-page")

	inputPath := filepath.Join(dir, "input.folio")
	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read %s: %v", inputPath, err)
	}
	if len(inputBytes) == 0 {
		t.Fatalf("%s is empty — comparing it against the module's constant would compare two absent things", inputPath)
	}
	if string(inputBytes) != threeBandPageTemplateJSON {
		t.Fatalf("%s has drifted from folio-go/threeBandPageTemplateJSON (three_band_page_template.go) — the two are supposed to be byte-identical, kept in sync by hand (font-text's and wrapped-text's precedent)", inputPath)
	}

	fixture := loadExpectedFixture(t, filepath.Join(dir, "expected.json"))
	if fixture.Folio8GoVersion == "" {
		t.Fatal("fixture is missing folio8GoVersion")
	}
	if fixture.GoToolchain == "" {
		t.Fatal("fixture is missing goToolchain")
	}
	if !isSHA256HexString(fixture.SHA256) {
		t.Fatalf("fixture sha256 %q is not 64 lower-case hex characters (AC16)", fixture.SHA256)
	}

	expectedPDF, rerr := os.ReadFile(filepath.Join(dir, "expected.pdf"))
	if rerr != nil {
		t.Fatalf("read expected.pdf: %v", rerr)
	}
	if len(expectedPDF) == 0 {
		t.Fatal("fixtures/three-band-page/expected.pdf is empty — a digest over it would certify nothing")
	}
	if onDisk := sha256Hex(expectedPDF); onDisk != fixture.SHA256 {
		t.Fatalf("fixtures/three-band-page/expected.pdf's own sha256 (%s) does not match expected.json's recorded sha256 (%s) — the fixture's two halves have drifted apart", onDisk, fixture.SHA256)
	}

	if runtime.Version() != fixture.GoToolchain {
		t.Skipf("recorded under %s, running %s — a digest is only comparable within one toolchain (AD-21)", fixture.GoToolchain, runtime.Version())
	}

	got := sha256Hex(renderThreeBandPage(t))
	if got != fixture.SHA256 {
		t.Fatalf("golden fixture mismatch: got sha256 %s, want %s (fixtures/three-band-page). Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change. Do not regenerate the fixture to make this pass.", got, fixture.SHA256)
	}
}

// TestThreeBandPageIsByteIdenticalAcrossTwoProcesses is AC7's
// "two independent process invocations produce byte-identical output",
// exercised locally rather than only in the deferred matrix.
func TestThreeBandPageIsByteIdenticalAcrossTwoProcesses(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skip("js/wasm has no os/exec or pipes; covered by the cross-target matrix harness (folio-go/matrix_test.go)")
	}
	a := renderThreeBandInSubprocess(t, "1")
	b := renderThreeBandInSubprocess(t, "4")

	assertWellFormedPDF(t, "child A (GOMAXPROCS=1, three-band)", a, 1)
	assertWellFormedPDF(t, "child B (GOMAXPROCS=4, three-band)", b, 1)
	if !containsFontFile2(a) || !containsFontFile2(b) {
		t.Fatal("a child's output contains no FontFile2 — a match below would prove nothing (AC10b's vacuity guard, applied here)")
	}

	if !bytes.Equal(a, b) {
		offset, window := firstDivergence(a, b)
		t.Fatalf("subprocess three-band outputs differ (len a=%d, len b=%d); first divergence at byte offset %d; %s",
			len(a), len(b), offset, window)
	}
}

func renderThreeBandInSubprocess(t *testing.T, gomaxprocs string) []byte {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(),
		subprocessThreeBandEnvVar+"=1",
		"GOMAXPROCS="+gomaxprocs,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("subprocess three-band render (GOMAXPROCS=%s) failed: %v\nstderr: %s", gomaxprocs, err, stderr.String())
	}
	return stdout.Bytes()
}
