package folio8

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// The ruled vertical model fixtures/line-spacing/ is stated in, DECLARED
// ONCE here and read by this file AND by matrix_test.go's per-leg feature
// guard (which is matrix-tagged and would otherwise carry a second copy —
// the two-literals hazard that makes a guard agree with itself while
// disagreeing with the document).
//
// Chain ["Noto Sans"] at 11 pt. hhea ascent 1069, descent -293, lineGap 0:
//
//	FirstBaseline = 1069 * 11 = 11,759   <- NOT scaled by lineSpacing
//	ruled Advance = 1362 * 11 = 14,982
//	LastDescent   =  293 * 11 =  3,223   <- NOT scaled by lineSpacing
//
// Every scaled advance is ScaleRound(14982, r, 1000), hand-computed:
//
//	r = 1500 -> 14982*1500 = 22,473,000; /1000 -> 22,473 exactly
//	r =  600 -> 14982* 600 =  8,989,200; /1000 -> q 8,989 r 200, and
//	            200 < 500, so round-half-to-even keeps 8,989
const (
	lineSpacingFirstBaselineMP = int64(11759)
	lineSpacingRuledAdvanceMP  = int64(14982)
	lineSpacingLastDescentMP   = int64(3223)
	lineSpacingOpenAdvanceMP   = int64(22473)
	lineSpacingTightAdvanceMP  = int64(8989)
)

// lineSpacingBaselinesMP is every DRAWN baseline, top of page first,
// grouped by the element that emits it, with the interval that element's
// own declared ratio requires.
//
// pdfY of an element's first baseline is
// 841890 - 36000 - (20000 + y) - 11759 = 774131 - y, and each subsequent
// line is one of THAT element's advances lower. The table's rows are
// derived in TestLineSpacingSemanticAcceptance's doc comment.
var lineSpacingBaselinesMP = []struct {
	id string
	// wantAdvance is the interval this element's own consecutive drawn
	// baselines must sit at; 0 for an element that draws only one.
	wantAdvance int64
	ys          []int64
}{
	{"e1", lineSpacingRuledAdvanceMP, []int64{774131, 759149}},
	{"e2", lineSpacingOpenAdvanceMP, []int64{714131, 691658}},
	{"e3", lineSpacingTightAdvanceMP, []int64{634131, 625142, 616153}},
	{"e4 header", 0, []int64{534131}},
	{"e4 row 1", lineSpacingOpenAdvanceMP, []int64{514131, 491658}},
	{"e4 row 2", 0, []int64{476676}},
}

func lineSpacingDrawnBaselines() []int64 {
	var out []int64
	for _, el := range lineSpacingBaselinesMP {
		out = append(out, el.ys...)
	}
	return out
}

// lineSpacingAssertBaselines checks the drawn baselines PER ELEMENT,
// which a flat list cannot: a regression that moved a baseline out of e2
// and into e3 preserves the count, the ordering and the six values' sum.
// Each element's own inter-baseline interval is checked against the ratio
// that element declares, so "the advance scaled" is asserted three
// different ways in one document rather than once.
//
// It reports through fail(format, args...) so the ordinary suite can use
// t.Errorf and the matrix legs t.Fatalf, without a second copy of the
// arithmetic.
func lineSpacingAssertBaselines(ys []int64, fail func(string, ...any)) {
	want := lineSpacingDrawnBaselines()
	if len(ys) != len(want) {
		fail("the render occupies %d distinct drawn baselines %v, want %d %v", len(ys), ys, len(want), want)
		return
	}
	at := 0
	for _, el := range lineSpacingBaselinesMP {
		for i, wantY := range el.ys {
			if ys[at+i] != wantY {
				fail("%s: drawn baseline %d is at pdfY %d, want the hand-derived %d", el.id, i, ys[at+i], wantY)
			}
		}
		for i := 1; i < len(el.ys); i++ {
			if gap := ys[at+i-1] - ys[at+i]; gap != el.wantAdvance {
				fail("%s: the interval between drawn baselines %d and %d is %d mp, want %d — the declared lineSpacing was not honoured here", el.id, i-1, i, gap, el.wantAdvance)
			}
		}
		at += len(el.ys)
	}

	// THE VACUITY GUARD THAT MAKES ALL OF THE ABOVE MEAN SOMETHING. Every
	// element of this document fits its box and every baseline is drawn,
	// so a build that IGNORED lineSpacing entirely would emit exactly this
	// many baselines — just evenly spaced at the ruled advance. What it
	// could not do is emit THREE DIFFERENT intervals. Asserted as a
	// relation between the elements rather than against a literal, so a
	// uniform change to the leading rule cannot satisfy it.
	if len(ys) == len(want) {
		ruled, open, tight := ys[0]-ys[1], ys[2]-ys[3], ys[4]-ys[5]
		if !(tight < ruled && ruled < open) {
			fail("e3's %d mp, e1's %d mp and e2's %d mp intervals are not strictly increasing — this fixture is certifying a document whose declared line spacing was ignored", tight, ruled, open)
		}
	}
}

func renderLineSpacing(t *testing.T) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(lineSpacingTemplateJSON))
	if err != nil {
		t.Fatalf("parse line-spacing template: %v", err)
	}
	res, err := Render(tpl, Data(lineSpacingDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render line-spacing: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Fatalf("the line-spacing fixture must render with NO diagnostics; got %+v", res.Diagnostics)
	}
	return res.Bytes
}

// TestLineSpacingSemanticAcceptance is D-000.22's semantic acceptance
// step for fixtures/line-spacing/, performed AT FIRST RECORDING — "a
// wrong first recording is not a bug that gets caught later, it is a bug
// that gets RATIFIED later".
//
// The numbers are hand-derived from the committed face, never read back
// off the render:
//
//	A4 is 841,890 mp tall; margins 36 pt; pageHeader height 20 pt, so the
//	content band's origin is 20,000 mp. pdfY of an element's FIRST
//	baseline is 841890 - 36000 - (20000 + y) - 11759 = 774131 - y.
//
//	e1 y=      0  no ratio    774131, 759149                (-14,982)
//	e2 y= 60,000  1.5         714131, 691658                (-22,473)
//	e3 y=140,000  0.6         634131, 625142, 616153        (- 8,989)
//
//	e4 y=240,000 is the table. Its header row occupies headerHeight
//	20,000 from the table top, valign top, so the label's own top is
//	240,000 and its baseline 251,759 -> pdfY 534,131. Row 1 starts at
//	260,000; its two lines sit one BODY advance apart, so their baselines
//	are 271,759 and 294,232 -> pdfY 514,131 and 491,658. Row 1's height is
//	FirstBaseline + 1*Advance + LastDescent = 11759 + 22473 + 3223 =
//	37,455, so row 2 starts at 297,455, baseline 309,214 -> pdfY 476,676.
//	THE ROW HEIGHT IS WHERE THE TABLE'S OWN RATIO BECOMES OBSERVABLE: at
//	the ruled advance row 2 would start 7,491 mp higher.
func TestLineSpacingSemanticAcceptance(t *testing.T) {
	b := renderLineSpacing(t)
	assertWellFormedPDF(t, "line-spacing golden fixture render", b, 1)

	runs := readEmittedRuns(t, b)
	if len(runs) == 0 {
		t.Fatal("the line-spacing render emitted no text runs, so every assertion below would be vacuous")
	}
	ys := linesByOrigin(runs)

	// (a) THE DRAWN BASELINES, EXACTLY, PER ELEMENT, each against its own
	//     declared ratio — plus the strictly-increasing vacuity guard.
	lineSpacingAssertBaselines(ys, t.Errorf)

	// (b) THE FIRST BASELINE OF EVERY ELEMENT IS EXACTLY WHERE IT WOULD
	//     BE WITH NO RATIO AT ALL. This is the two-model split (D-2.5a /
	//     DW-15) read off the ARTIFACT: e1 declares no spacing, e2
	//     declares 1.5 and e3 declares 0.6, and all three first baselines
	//     satisfy the SAME rule 774131 - y. If lineSpacing had reached
	//     FirstBaseline, e2's and e3's top edges would have moved and
	//     every neighbour below them would appear to jump.
	//
	//     Derived from e1's OBSERVED first baseline rather than from a
	//     literal, so this stays an assertion about the relation between
	//     the three elements even if the shipped face ever changes.
	if len(ys) == len(lineSpacingDrawnBaselines()) {
		firstOf := map[string]int64{"e1": ys[0], "e2": ys[2], "e3": ys[4]}
		for _, c := range []struct {
			id string
			y  geom.Length
		}{{"e2", 60000}, {"e3", 140000}} {
			want := firstOf["e1"] - int64(c.y)
			if got := firstOf[c.id]; got != want {
				t.Errorf(
					"%s's FIRST baseline is at pdfY %d, want %d (e1's %d less its own y offset of %d) — lineSpacing must scale Advance and NOTHING else, so a ratio can never move an element's top edge",
					c.id, got, want, firstOf["e1"], c.y,
				)
			}
		}
	}

	// (c) The declared face is embedded and named.
	if !containsFontFile2(b) {
		t.Fatal("the line-spacing render carries no FontFile2 — the fixture would certify nothing about embedding")
	}
	if !strings.Contains(string(b), "NotoSans") {
		t.Error("the render does not name the embedded face NotoSans")
	}

	// (d) AC17's /ToUnicode section sizes.
	assertToUnicodeSectionsUnderCap(t, b)

	t.Logf("line-spacing semantic acceptance: %d text runs across %d drawn baselines %v; ruled interval %d mp, widened %d, tightened %d (first-baseline offset %d, which the tightened advance is BELOW — the overlapping line boxes tight leading is)", len(runs), len(ys), ys, lineSpacingRuledAdvanceMP, lineSpacingOpenAdvanceMP, lineSpacingTightAdvanceMP, lineSpacingFirstBaselineMP)
}

// TestLineSpacingGoldenFixture is the byte-identity half: the live render
// must reproduce fixtures/line-spacing/expected.pdf exactly.
//
// It runs AFTER the semantic assertions above in file order and in intent
// (D-000.22): a hash frozen before anyone checked what it contained
// certifies only that the bytes have not changed.
func TestLineSpacingGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", "line-spacing")

	inputPath := filepath.Join(dir, "input.folio")
	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read %s: %v", inputPath, err)
	}
	if string(inputBytes) != lineSpacingTemplateJSON {
		t.Fatalf(
			"%s has drifted from folio-go/lineSpacingTemplateJSON (line_spacing_template.go) — the two are "+
				"supposed to be byte-identical (kept in sync by hand, per mandatory-break's precedent)",
			inputPath,
		)
	}

	fixture := loadExpectedFixture(t, filepath.Join(dir, "expected.json"))
	if fixture.Folio8GoVersion == "" {
		t.Fatal("fixture is missing folio8GoVersion")
	}
	if fixture.GoToolchain == "" {
		t.Fatal("fixture is missing goToolchain")
	}
	if !isSHA256HexString(fixture.SHA256) {
		t.Fatalf("fixture sha256 %q is not 64 lower-case hex characters", fixture.SHA256)
	}

	b := renderLineSpacing(t)
	got := sha256Hex(b)

	expectedPDF, err := os.ReadFile(filepath.Join(dir, "expected.pdf"))
	if err != nil {
		t.Fatalf("read expected.pdf: %v", err)
	}
	if onDisk := sha256Hex(expectedPDF); onDisk != fixture.SHA256 {
		t.Fatalf(
			"fixtures/line-spacing/expected.pdf's own sha256 (%s) does not match expected.json's recorded sha256 (%s) — the fixture's two halves have drifted apart",
			onDisk, fixture.SHA256,
		)
	}
	if got != fixture.SHA256 {
		t.Fatalf(
			"golden fixture mismatch: got sha256 %s, want %s (fixtures/line-spacing). Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change. Do not regenerate the fixture to make this pass.",
			got, fixture.SHA256,
		)
	}
}

// TestLineSpacingFixtureDeclaresTheVersionItsContentRequires pins the
// half of D-1.4.13 this fixture is the corpus's only witness to: it uses
// a 1.1 key, so it declares 1.1 — and re-serializing it neither raises it
// further (the library's ceiling is not a document's version) nor lowers
// it.
func TestLineSpacingFixtureDeclaresTheVersionItsContentRequires(t *testing.T) {
	if !strings.Contains(lineSpacingTemplateJSON, `"version": "1.1"`) {
		t.Fatal("the line-spacing fixture sets style.lineSpacing, so it must DECLARE 1.1 — a document that uses a 1.1 key while declaring 1.0 is the misdeclaration D-7.2.1 exists to end")
	}
	d, err := template.ParseDocument([]byte(lineSpacingTemplateJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := template.SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if !strings.Contains(string(out), `"version": "1.1"`) {
		t.Fatalf("re-serializing the fixture must keep 1.1:\n%s", out)
	}
}
