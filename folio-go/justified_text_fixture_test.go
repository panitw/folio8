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

// The geometry fixtures/justified-text/ is stated in, DECLARED ONCE here
// and read by this file AND by matrix_test.go's per-leg feature guard
// (which is matrix-tagged and would otherwise carry a second copy — the
// two-literals hazard that makes a guard agree with itself while
// disagreeing with the document).
//
// Both elements are chain ["Noto Sans"] at 11 pt in a 200 pt box, so the
// vertical model is the one fixtures/line-spacing/ derives:
//
//	FirstBaseline = 11,759   Advance = 14,982   LastDescent = 3,223
//
// pdfY of an element's first baseline is
// 841890 - 36000 - (20000 + y) - 11759 = 774131 - y.
const (
	justifiedBoxWidthMP = int64(200_000)
	// justifiedLeftEdgeMP is the element's left edge IN THE EMITTED PDF:
	// its declared x of 0 plus the page's 36 pt left margin, which the
	// producer applies at emission.
	justifiedLeftEdgeMP = int64(36_000)
	// justifiedRunLeftMP and justifiedRunRightMP are the same two edges on
	// the SHIPPING RUN PATH, where coordinates are still document-relative
	// and the margin has not been applied: the element declares x 0 and a
	// 200 pt width.
	//
	// justifiedRunRightMP is where EVERY justified line's last piece must
	// end, exactly. It is a consequence of basing the slack on the summed
	// piece widths rather than on the packer's single line measurement,
	// and it is the property a piece-wise rounding error would break by a
	// millipoint or two — small enough to look like nothing and large
	// enough to differ between targets.
	justifiedRunLeftMP  = int64(0)
	justifiedRunRightMP = justifiedRunLeftMP + justifiedBoxWidthMP
)

// justifiedLines is what the packer makes of the fixture's paragraph, in
// order, with the arithmetic each line's placement is a function of.
//
// gaps is the number of INTERIOR break opportunities the line holds;
// slack is the declared width less the summed piece widths; base and
// remainder are slack/gaps and slack mod gaps. Every one of these is
// hand-derived from the numbers below rather than read back off a render:
//
//	line 0  slack  3,034 over 7 gaps -> 7 x   433 + 3 left over
//	line 2  slack  1,494 over 5 gaps -> 5 x   298 + 4 left over
//	line 3  slack 16,839 over 7 gaps -> 7 x 2,405 + 4 left over
//
// A remainder of zero at every line would make the ordered distribution
// rule vacuous — the interesting half of the rule is which gaps get the
// extra millipoint — so all three carry one, and two of them carry a
// remainder LARGER than half the gap count, which a "spread the remainder
// from the end" implementation would place at the wrong gaps.
var justifiedLines = []struct {
	// pieces is the number of drawn runs the line is set as: gaps+1 when
	// the line is justified, 1 when it is ragged.
	pieces int
	// ragged records WHY a line is not justified, and the three reasons
	// are three because each answers a different question (D-7.1.5).
	ragged string
	gaps   int
	slack  int64
}{
	{pieces: 8, gaps: 7, slack: 3_034},
	{pieces: 1, ragged: "the author ended this line by typing a break, and it is NOT the last line"},
	{pieces: 6, gaps: 5, slack: 1_494},
	{pieces: 8, gaps: 7, slack: 16_839},
	{pieces: 1, ragged: "the last line of the element, which only the line INDEX can answer"},
}

// justifiedBaselinesMP is every DRAWN baseline, top of page first,
// grouped by the element that emits it. e1 (justified) and e2 (the
// control, no align at all) pack into the SAME five lines at the SAME
// five intervals: only the horizontal placement differs, which is what
// makes the run counts below evidence about justification rather than
// about wrapping.
var justifiedBaselinesMP = []struct {
	id string
	ys []int64
}{
	{"e1", []int64{774131, 759149, 744167, 729185, 714203}},
	{"e2", []int64{674131, 659149, 644167, 629185, 614203}},
}

// justifiedRunsPerBaseline is the per-line run count the two elements
// produce, in the order justifiedBaselinesMP lists them. THIS IS THE
// VACUITY GUARD, and it is the whole reason the control element exists:
// every line of this document fits its box, so a build that ignored
// `justify` entirely would emit exactly these ten baselines — just one
// run each, at the left edge, which is precisely what e2 already is.
// What such a build could not do is make e1 differ from e2 at all.
func justifiedRunsPerBaseline() []int {
	out := make([]int, 0, 10)
	for _, ln := range justifiedLines {
		out = append(out, ln.pieces)
	}
	for range justifiedLines {
		out = append(out, 1)
	}
	return out
}

// justifiedAssertGeometry checks the drawn baselines and the per-line run
// counts of a rendered justified-text stream. It reports through
// fail(format, args...) so the ordinary suite can use t.Errorf and the
// matrix legs t.Fatalf, without a second copy of the arithmetic.
func justifiedAssertGeometry(runs []emittedRun, fail func(string, ...any)) {
	ys := linesByOrigin(runs)
	var want []int64
	for _, el := range justifiedBaselinesMP {
		want = append(want, el.ys...)
	}
	if len(ys) != len(want) {
		fail("the render occupies %d distinct drawn baselines %v, want %d %v", len(ys), ys, len(want), want)
		return
	}
	at := 0
	for _, el := range justifiedBaselinesMP {
		for i, wantY := range el.ys {
			if ys[at+i] != wantY {
				fail("%s: drawn baseline %d is at pdfY %d, want the hand-derived %d", el.id, i, ys[at+i], wantY)
			}
		}
		at += len(el.ys)
	}

	counts := make(map[int64]int, len(ys))
	xs := make(map[int64][]int64, len(ys))
	for _, r := range runs {
		counts[r.OriginYMilli]++
		xs[r.OriginYMilli] = append(xs[r.OriginYMilli], r.OriginXMilli)
	}
	wantCounts := justifiedRunsPerBaseline()
	for i, y := range ys {
		if counts[y] != wantCounts[i] {
			fail("baseline %d (pdfY %d) carries %d drawn runs, want %d — a justified line is drawn as one run PER PIECE, and a ragged one as a single run", i, y, counts[y], wantCounts[i])
		}
		positions := append([]int64(nil), xs[y]...)
		sort.Slice(positions, func(a, b int) bool { return positions[a] < positions[b] })
		if len(positions) == 0 {
			continue
		}
		if positions[0] != justifiedLeftEdgeMP {
			fail("baseline %d (pdfY %d) starts at x %d, want the element's own left edge %d — justification distributes slack BETWEEN pieces, never before the line", i, y, positions[0], justifiedLeftEdgeMP)
		}
		for j := 1; j < len(positions); j++ {
			if positions[j] <= positions[j-1] {
				fail("baseline %d (pdfY %d) draws pieces at %v, which is not strictly ascending", i, y, positions)
				break
			}
		}
	}

	// THE DISCRIMINATING RELATION, asserted between the two elements
	// rather than against a literal: e1 must draw MORE runs than e2 even
	// though the two carry the same string in the same box. A uniform
	// change to the placement rule cannot satisfy this.
	e1, e2 := 0, 0
	for i, y := range ys {
		if i < len(justifiedLines) {
			e1 += counts[y]
		} else {
			e2 += counts[y]
		}
	}
	if e2 != len(justifiedLines) {
		fail("the control element e2 draws %d runs across %d lines, want one per line — it declares no alignment at all", e2, len(justifiedLines))
	}
	if e1 <= e2 {
		fail("the justified element draws %d runs and the unaligned control %d — this fixture is certifying a document whose declared justification was ignored", e1, e2)
	}
}

func renderJustifiedText(t *testing.T) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(justifiedTemplateJSON))
	if err != nil {
		t.Fatalf("parse justified-text template: %v", err)
	}
	res, err := Render(tpl, Data(justifiedDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render justified-text: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Fatalf("the justified-text fixture must render with NO diagnostics; got %+v", res.Diagnostics)
	}
	return res.Bytes
}

// TestJustifiedTextSemanticAcceptance is D-000.22's semantic acceptance
// step for fixtures/justified-text/, performed AT FIRST RECORDING — "a
// wrong first recording is not a bug that gets caught later, it is a bug
// that gets RATIFIED later".
func TestJustifiedTextSemanticAcceptance(t *testing.T) {
	b := renderJustifiedText(t)
	assertWellFormedPDF(t, "justified-text golden fixture render", b, 1)

	runs := readEmittedRuns(t, b)
	if len(runs) == 0 {
		t.Fatal("the justified-text render emitted no text runs, so every assertion below would be vacuous")
	}
	justifiedAssertGeometry(runs, t.Errorf)

	if !containsFontFile2(b) {
		t.Fatal("the justified-text render carries no FontFile2 — the fixture would certify nothing about embedding")
	}
	if !strings.Contains(string(b), "NotoSans") {
		t.Error("the render does not name the embedded face NotoSans")
	}
	assertToUnicodeSectionsUnderCap(t, b)

	t.Logf("justified-text semantic acceptance: %d text runs across %d drawn baselines; e1's per-line run counts are %v against the control's one per line", len(runs), len(linesByOrigin(runs)), justifiedRunsPerBaseline()[:len(justifiedLines)])
}

// TestJustifiedTextDistributesSlackExactly is the arithmetic half, read
// off the SHIPPING run path rather than off the PDF operators, because
// only there is each piece's own measured width available beside its x.
//
// It asserts the four properties the rule is: the distributed amounts sum
// to the slack EXACTLY, the gaps are base or base+1 and never anything
// else, the first `remainder` gaps in ASCENDING position along the line
// are the larger ones, and the last piece's right edge equals the
// element's declared right edge exactly.
func TestJustifiedTextDistributesSlackExactly(t *testing.T) {
	tpl, err := ParseTemplate([]byte(justifiedTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	data := emptyBindValue(t)
	fs := testShippedFontSet()
	runs, err := collectTextRuns(tpl, data, data, fs, newFontCache())
	if err != nil {
		t.Fatal(err)
	}
	byLine := map[int][]textRunSource{}
	for _, run := range runs {
		if run.elementID != "e1" {
			continue
		}
		byLine[run.lineIndex] = append(byLine[run.lineIndex], run)
	}
	if len(byLine) != len(justifiedLines) {
		t.Fatalf("presence precondition: e1 occupies %d lines, want %d", len(byLine), len(justifiedLines))
	}
	cache := newFontCache()
	for i, want := range justifiedLines {
		lineRuns := byLine[i]
		sort.Slice(lineRuns, func(a, b int) bool { return lineRuns[a].x < lineRuns[b].x })
		if len(lineRuns) != want.pieces {
			t.Errorf("line %d is drawn as %d pieces, want %d", i, len(lineRuns), want.pieces)
			continue
		}
		widths := make([]geom.Length, len(lineRuns))
		for j, run := range lineRuns {
			w, werr := shippingRunWidth(run, fs, cache)
			if werr != nil {
				t.Fatal(werr)
			}
			widths[j] = w
		}
		if want.ragged != "" {
			if got := int64(lineRuns[0].x); got != justifiedRunLeftMP {
				t.Errorf("line %d is ragged (%s) and must sit at the element's natural start edge %d, got %d", i, want.ragged, justifiedRunLeftMP, got)
			}
			continue
		}

		// (a) THE RIGHT EDGE LANDS ON THE DECLARED WIDTH, EXACTLY.
		last := len(lineRuns) - 1
		if got := int64(lineRuns[last].x) + int64(widths[last]); got != justifiedRunRightMP {
			t.Errorf("line %d's last piece ends at x %d, want the element's declared right edge %d exactly", i, got, justifiedRunRightMP)
		}

		// (b) THE DISTRIBUTED AMOUNTS SUM TO THE SLACK, EXACTLY.
		gaps := make([]int64, 0, want.gaps)
		var distributed int64
		for j := 1; j < len(lineRuns); j++ {
			gap := int64(lineRuns[j].x) - (int64(lineRuns[j-1].x) + int64(widths[j-1]))
			gaps = append(gaps, gap)
			distributed += gap
		}
		if len(gaps) != want.gaps {
			t.Errorf("line %d has %d gaps, want %d", i, len(gaps), want.gaps)
			continue
		}
		if distributed != want.slack {
			t.Errorf("line %d distributed %d millipoints across its gaps, want the slack %d exactly — a discarded remainder is a right edge that misses", i, distributed, want.slack)
		}

		// (c) THE ORDERED REMAINDER RULE: base to every gap, and one extra
		//     millipoint to the first `remainder` gaps in reading order.
		base := want.slack / int64(want.gaps)
		remainder := want.slack - base*int64(want.gaps)
		if remainder == 0 {
			t.Errorf("line %d's slack %d divides its %d gaps exactly — this case cannot distinguish an ordered remainder rule from an unordered one", i, want.slack, want.gaps)
		}
		for j, gap := range gaps {
			wantGap := base
			if int64(j) < remainder {
				wantGap = base + 1
			}
			if gap != wantGap {
				t.Errorf("line %d gap %d is %d, want %d (base %d, remainder %d to the FIRST %d gaps in ascending position)", i, j, gap, wantGap, base, remainder, remainder)
			}
		}
	}
}

// TestJustifiedTextFixtureDeclaresTheVersionItsContentRequires pins the
// half of D-1.4.13 this fixture is the corpus's only witness to: it uses
// the 2.0 alignment value, so it declares 2.0 — and re-serializing it
// neither raises it further (the library's ceiling is not a document's
// version) nor lowers it.
func TestJustifiedTextFixtureDeclaresTheVersionItsContentRequires(t *testing.T) {
	if !strings.Contains(justifiedTemplateJSON, `"version": "2.0"`) {
		t.Fatal(`the justified-text fixture sets style.align: "justify", which extends a CLOSED SET, so it must DECLARE 2.0 — a document that uses a 2.0 value while declaring 1.x is the misdeclaration D-7.2.1 exists to end`)
	}
	d, err := template.ParseDocument([]byte(justifiedTemplateJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := template.SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if !strings.Contains(string(out), `"version": "2.0"`) {
		t.Fatalf("re-serializing the fixture must keep 2.0:\n%s", out)
	}
}

// TestJustifiedTextGoldenFixture is the byte-identity half: the live
// render must reproduce fixtures/justified-text/expected.pdf exactly.
//
// It runs AFTER the semantic assertions above in file order and in intent
// (D-000.22): a hash frozen before anyone checked what it contained
// certifies only that the bytes have not changed.
func TestJustifiedTextGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", "justified-text")

	inputPath := filepath.Join(dir, "input.folio")
	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read %s: %v", inputPath, err)
	}
	if string(inputBytes) != justifiedTemplateJSON {
		t.Fatalf(
			"%s has drifted from folio-go/justifiedTemplateJSON (justified_text_template.go) — the two are "+
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

	b := renderJustifiedText(t)
	got := sha256Hex(b)

	expectedPDF, err := os.ReadFile(filepath.Join(dir, "expected.pdf"))
	if err != nil {
		t.Fatalf("read expected.pdf: %v", err)
	}
	if onDisk := sha256Hex(expectedPDF); onDisk != fixture.SHA256 {
		t.Fatalf(
			"fixtures/justified-text/expected.pdf's own sha256 (%s) does not match expected.json's recorded sha256 (%s) — the fixture's two halves have drifted apart",
			onDisk, fixture.SHA256,
		)
	}
	if got != fixture.SHA256 {
		t.Fatalf(
			"golden fixture mismatch: got sha256 %s, want %s (fixtures/justified-text). Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change. Do not regenerate the fixture to make this pass.",
			got, fixture.SHA256,
		)
	}
}
