package folio8

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// The geometry fixtures/alignment-rounding/ is stated in, DECLARED ONCE
// here and read by this file AND by matrix_test.go's per-leg feature
// guard (which is matrix-tagged and would otherwise carry a second copy).
//
// Chain ["Noto Sans"] at 11 pt throughout, so the vertical model is the
// one fixtures/line-spacing/ derives: FirstBaseline 11,759, Advance
// 14,982, LastDescent 3,223, and a one-line block is 14,982 tall.
//
// A4 is 841,890 mp tall; margins 36 pt; pageHeader height 20 pt, so the
// content band's origin is 20,000 mp and pdfY of a top-aligned element's
// first baseline is 841890 - 36000 - (20000 + y) - 11759 = 774131 - y.
//
//	e1  y=      0  align center                       774131
//	e2  y= 30,000  valign middle, height 40.001       774131 - 30000 - ScaleRound(25019,1,2)
//	                                                = 774131 - 30000 - 12510 = 731621
//	e3  y= 80,000  valign bottom, height 40           774131 - 80000 - 25018 = 669113
//	e4  y=130,000  the table. tableTop is 150,000; its header cell's
//	    content box is the full 24,001 headerHeight, so the label's own
//	    top is 150000 + ScaleRound(24001-14982,1,2) = 150000 + 4510 and
//	    its baseline 166,269 -> pdfY 639,621. Rows begin at 174,001, so
//	    row 1's first baseline is 185,760 -> pdfY 620,130, and every
//	    later line is one Advance lower.
var alignmentRoundingRunsMP = []struct {
	x, y int64
	what string
}{
	{115392, 774131, "e1 Centred — align center, the tie rounded UP to even"},
	{36000, 731621, "e2 Middled — valign middle, the tie rounded UP to even"},
	{36000, 669113, "e3 Bottomed — valign bottom, unrounded and equally undeclared until now"},
	{56916, 639621, "e4 header Qty — align center (tie UP to even) and valign middle (tie UP)"},
	{96003, 639621, "e4 header Clause — left, the control for the centred label beside it"},
	{96003, 620130, "row 1 clause line 1"},
	{62856, 605148, "row 1 qty 3 — centred, and on the row's SECOND line slot"},
	{96003, 605148, "row 1 clause line 2"},
	{96003, 590166, "row 1 clause line 3"},
	{96003, 575184, "row 1 clause line 4"},
	{59710, 560202, "row 2 qty 12 — a DIFFERENT odd slack from 3's"},
	{96003, 560202, "row 2 clause"},
	{62856, 545220, "row 3 qty 7"},
	{96003, 545220, "row 3 clause line 1"},
	{96003, 530238, "row 3 clause line 2"},
	{62856, 515256, "footer count 3 — centred, in the footer cell's own code"},
}

// alignmentRoundingAssertRuns checks every drawn run's origin against the
// hand-derived table above. It reports through fail(format, args...) so
// the ordinary suite can use t.Errorf and the matrix legs t.Fatalf,
// without a second copy of the arithmetic.
func alignmentRoundingAssertRuns(runs []emittedRun, fail func(string, ...any)) {
	got := make([][2]int64, 0, len(runs))
	for _, r := range runs {
		got = append(got, [2]int64{r.OriginXMilli, r.OriginYMilli})
	}
	sort.Slice(got, func(a, b int) bool {
		if got[a][1] != got[b][1] {
			return got[a][1] > got[b][1]
		}
		return got[a][0] < got[b][0]
	})
	if len(got) != len(alignmentRoundingRunsMP) {
		fail("the render draws %d runs %v, want %d", len(got), got, len(alignmentRoundingRunsMP))
		return
	}
	for i, want := range alignmentRoundingRunsMP {
		if got[i][0] != want.x || got[i][1] != want.y {
			fail("run %d (%s) is at (%d, %d), want the hand-derived (%d, %d)", i, want.what, got[i][0], got[i][1], want.x, want.y)
		}
	}

	// THE VACUITY GUARD, and it is what makes the sixteen literals above
	// mean something. Asserted as RELATIONS between the runs rather than
	// against further literals, so a uniform change to the rounding rule
	// cannot satisfy it:
	//
	//  - the centred text element does NOT sit at the page's left edge;
	//  - the two centred qty values are at DIFFERENT x, because their
	//    slacks differ — a build that centred nothing would put both at
	//    the column's own left edge;
	//  - row 1's qty cell is on the row's SECOND line slot, which is
	//    neither the first nor the last: that is the integer line-count
	//    split, and it is the only construct in the repository reaching it.
	if got[0][0] == 36000 {
		fail("the centred text element is drawn at the page's left edge — this fixture is certifying a document whose declared alignment was ignored")
	}
	qty := map[int64]bool{}
	for _, r := range got {
		if r[0] != 96003 && r[1] < 639621 {
			qty[r[0]] = true
		}
	}
	if len(qty) < 2 {
		fail("the centred column's cells are drawn at %d distinct x positions, want at least 2 — a build that centred nothing would draw them all at the column's left edge", len(qty))
	}
	var clause1 []int64
	for _, r := range got {
		if r[0] == 96003 && r[1] <= 620130 && r[1] >= 575184 {
			clause1 = append(clause1, r[1])
		}
	}
	if len(clause1) != 4 {
		fail("row 1's clause cell occupies %d lines, want 4 — the row needs at least three spare line slots for `middle` to differ from both `top` and `bottom`", len(clause1))
		return
	}
	if got[6][1] != clause1[1] {
		fail("row 1's qty cell sits on the line slot at pdfY %d; `middle` over 3 spare slots is slot 3/2 = 1, which is pdfY %d (top would be %d, bottom %d)", got[6][1], clause1[1], clause1[0], clause1[3])
	}
}

func renderAlignmentRounding(t *testing.T) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(alignmentRoundingTemplateJSON))
	if err != nil {
		t.Fatalf("parse alignment-rounding template: %v", err)
	}
	res, err := Render(tpl, Data(alignmentRoundingDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render alignment-rounding: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Fatalf("the alignment-rounding fixture must render with NO diagnostics; got %+v", res.Diagnostics)
	}
	return res.Bytes
}

func alignmentRoundingMeasure(t *testing.T, s string) geom.Length {
	t.Helper()
	segs, _, err := shapeSegments("probe", []string{"Noto Sans"}, nil, s, testShippedFontSet(), newFontCache(), breaksAreDrawn)
	if err != nil {
		t.Fatalf("shape %q: %v", s, err)
	}
	return measureRuneRange(segs, 0, len([]rune(s)), 11_000)
}

// TestAlignmentRoundingSlacksAreOdd is the assertion without which this
// whole fixture would be decorative.
//
// geom.ScaleRound(slack, 1, 2) and a plain truncating slack/2 AGREE on
// every EVEN slack. A centred fixture whose boxes happened to leave even
// slack would therefore satisfy DW-24's literal text — "a document
// reaching every rounding site" — while detecting none of the mutations
// DW-24's own falsifier runs, and nothing else in the repository would
// notice.
//
// ODDNESS ALONE IS NOT ENOUGH EITHER, which is the whole reason this
// test asserts a residue rather than a parity. An odd slack does take
// the exact-half tie — but half of all odd slacks break that tie
// DOWNWARD to even, and downward is the very number truncation
// produces, so such a site agrees with truncation too. The
// discriminating condition is `slack ≡ 3 (mod 4)`: only then does the
// tie round UP, away from truncation, and only then does mutating the
// site to `slack / 2` move a golden byte.
//
// So every slack this document leaves at a rounding site is 3 (mod 4)
// on purpose — three of the boxes are declared in thousandths of a
// point to make it so — and that is CHECKED here rather than left to
// luck, which is what deferred-work.md's DW-24 closing note,
// fixtures/alignment-rounding/README.md and
// alignment_rounding_template.go all say this test does.
func TestAlignmentRoundingSlacksAreOdd(t *testing.T) {
	const oneLineBlock = geom.Length(14_982) // FirstBaseline 11,759 + LastDescent 3,223
	for _, c := range []struct {
		site  string
		box   geom.Length
		inner geom.Length
	}{
		{"text_alignment.go textAlignOffset — e1 align center", 200_000, alignmentRoundingMeasure(t, "Centred")},
		{"text_alignment.go textValignOffset — e2 valign middle", 40_001, oneLineBlock},
		{"table_render.go header cell align center — Qty", 60_003, alignmentRoundingMeasure(t, "Qty")},
		{"table_render.go header cell valign middle", 24_001, oneLineBlock},
		{"table_render.go body cell align center — 3 and 7", 60_003, alignmentRoundingMeasure(t, "3")},
		{"table_render.go body cell align center — 12", 60_003, alignmentRoundingMeasure(t, "12")},
		{"table_render.go footer cell align center — the count 3", 60_003, alignmentRoundingMeasure(t, "3")},
	} {
		slack := c.box - c.inner
		if slack <= 0 {
			t.Errorf("%s: slack is %d — a box with no slack reaches no rounding at all", c.site, slack)
			continue
		}
		if residue := int64(slack) % 4; residue != 3 {
			t.Errorf(
				"%s: slack is %d, which is %d (mod 4) — every slack at a rounding site must be 3 (mod 4). "+
					"An EVEN slack takes no tie at all, and a slack that is 1 (mod 4) halves DOWN to even, "+
					"which is exactly the number a truncating slack/2 also produces — either way this site "+
					"would stop distinguishing half-to-even from truncation and its golden coverage would "+
					"be vacuous",
				c.site, slack, residue,
			)
			continue
		}
		// And the discrimination is asserted against the RULE rather
		// than against a literal: at 3 (mod 4) the tie breaks UPWARD to
		// even, so ScaleRound must differ from the truncating slack/2 a
		// mutation would put in its place — by exactly one millipoint.
		if half, truncated := geom.ScaleRound(slack, 1, 2), slack/2; half != truncated+1 {
			t.Errorf(
				"%s: ScaleRound(%d, 1, 2) = %d and the truncating slack/2 gives %d — they must differ by one, "+
					"or mutating this site to truncation would move no golden byte",
				c.site, slack, half, truncated,
			)
		}
	}
	if alignmentRoundingMeasure(t, "3") == alignmentRoundingMeasure(t, "12") {
		t.Error("the two qty values shape to the same width, so the centred body cell rounds one slack rather than two")
	}
}

// TestAlignmentRoundingSemanticAcceptance is D-000.22's semantic
// acceptance step for fixtures/alignment-rounding/, performed AT FIRST
// RECORDING.
func TestAlignmentRoundingSemanticAcceptance(t *testing.T) {
	b := renderAlignmentRounding(t)
	assertWellFormedPDF(t, "alignment-rounding golden fixture render", b, 1)

	runs := readEmittedRuns(t, b)
	if len(runs) == 0 {
		t.Fatal("the alignment-rounding render emitted no text runs, so every assertion below would be vacuous")
	}
	alignmentRoundingAssertRuns(runs, t.Errorf)

	// The centred positions are also DERIVED here, from the same rule the
	// producer applies, so the literals above are pinned to the rule and
	// not merely to yesterday's output.
	for _, c := range []struct {
		contentX, contentW geom.Length
		text               string
		want               int64
	}{
		{36_000, 200_000, "Centred", 115392},
		{36_000, 60_003, "Qty", 56916},
		{36_000, 60_003, "3", 62856},
		{36_000, 60_003, "12", 59710},
	} {
		got := c.contentX + geom.ScaleRound(c.contentW-alignmentRoundingMeasure(t, c.text), 1, 2)
		if int64(got) != c.want {
			t.Errorf("the centring rule places %q at %d, but the fixture records %d", c.text, got, c.want)
		}
	}

	if !containsFontFile2(b) {
		t.Fatal("the alignment-rounding render carries no FontFile2")
	}
	if !strings.Contains(string(b), "NotoSans") {
		t.Error("the render does not name the embedded face NotoSans")
	}
	assertToUnicodeSectionsUnderCap(t, b)

	t.Logf("alignment-rounding semantic acceptance: %d text runs reaching the centred text element, the centred header/body/footer cells, both vertical rounds and the integer line-slot split", len(runs))
}

// TestAlignmentRoundingFixtureDeclaresTheVersionItsContentRequires: this
// document uses no 1.1 key and no 2.0 value, so it declares — and keeps
// declaring — 1.0. It is the corpus's witness that the format's three
// live versions coexist, beside line-spacing's 1.1 and justified-text's
// 2.0, and that neither the library's ceiling nor a sibling fixture's
// version leaks into a document that requires neither.
func TestAlignmentRoundingFixtureDeclaresTheVersionItsContentRequires(t *testing.T) {
	if !strings.Contains(alignmentRoundingTemplateJSON, `"version": "1.0"`) {
		t.Fatal("the alignment-rounding fixture uses no 1.1 key and no 2.0 value, so it must declare 1.0")
	}
	d, err := template.ParseDocument([]byte(alignmentRoundingTemplateJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := template.SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if !strings.Contains(string(out), `"version": "1.0"`) {
		t.Fatalf("re-serializing the fixture must keep 1.0, never raise it to the library's ceiling:\n%s", out)
	}
}

// TestAlignmentRoundingGoldenFixture is the byte-identity half. It runs
// AFTER the semantic assertions above in file order and in intent
// (D-000.22).
func TestAlignmentRoundingGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", "alignment-rounding")

	inputPath := filepath.Join(dir, "input.folio")
	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read %s: %v", inputPath, err)
	}
	if string(inputBytes) != alignmentRoundingTemplateJSON {
		t.Fatalf(
			"%s has drifted from folio-go/alignmentRoundingTemplateJSON (alignment_rounding_template.go) — the two are "+
				"supposed to be byte-identical (kept in sync by hand, per line-spacing's precedent)",
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

	b := renderAlignmentRounding(t)
	got := sha256Hex(b)

	expectedPDF, err := os.ReadFile(filepath.Join(dir, "expected.pdf"))
	if err != nil {
		t.Fatalf("read expected.pdf: %v", err)
	}
	if onDisk := sha256Hex(expectedPDF); onDisk != fixture.SHA256 {
		t.Fatalf(
			"fixtures/alignment-rounding/expected.pdf's own sha256 (%s) does not match expected.json's recorded sha256 (%s) — the fixture's two halves have drifted apart",
			onDisk, fixture.SHA256,
		)
	}
	if got != fixture.SHA256 {
		t.Fatalf(
			"golden fixture mismatch: got sha256 %s, want %s (fixtures/alignment-rounding). Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change. Do not regenerate the fixture to make this pass.",
			got, fixture.SHA256,
		)
	}
}
