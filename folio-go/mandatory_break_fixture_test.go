package folio8

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// mandatoryBreakExpectedLines is stated as LITERALS before the fixture's
// hash is frozen, because "the line count is whatever it came out as"
// would certify nothing.
//
// Every number was MEASURED first against the shipped Noto Sans at 11 pt
// (see mandatory_break_template.go for each element's role):
//
//	e1  "Yours\nsincerely"        full 74,305 mp against a 200,000 mp box -> 2 lines
//	e2  "Clause 1.\n\nClause 2."  full 92,664 mp against a 200,000 mp box -> 3 lines
//	e3  "first\nsecond word"      full 86,625 mp against a  40,000 mp box -> 2 lines
//
// e1 AND e2 FIT THEIR BOXES OUTRIGHT. That is deliberate and it is the
// opposite of fixtures/wrapped-text/'s premise: there, every element had
// to be WIDER than its box or "it wrapped" was vacuous. Here, a value
// too wide for its box would break for want of room and would say
// nothing about a break the author typed (D-7.1.7, finding 2).
var mandatoryBreakExpectedLines = map[string]int{
	"e1": 2,
	"e2": 3,
	"e3": 2,
}

// mandatoryBreakEmptyLines names, per element, the indexes of the lines
// that must be EMPTY. e2's line 1 is the paragraph gap — a line box with
// nothing in it, which draws no run at all and is therefore invisible to
// a baseline count. It is asserted on the layout, and its consequence
// (a two-advance gap between e2's two DRAWN baselines) is asserted on
// the produced PDF.
var mandatoryBreakEmptyLines = map[string][]int{
	"e2": {1},
}

// mandatoryBreakAdvanceMP and mandatoryBreakBaselinesMP are the fixture's
// ruled vertical numbers, DECLARED ONCE here and read by both this file
// and matrix_test.go's per-leg feature guard (which is matrix-tagged and
// would otherwise carry a second copy of them — the two-literals hazard
// that makes a guard agree with itself while disagreeing with the
// document).
//
// Hand-derived, never read back off a render: see
// TestMandatoryBreakSemanticAcceptance for the full derivation.
const mandatoryBreakAdvanceMP = int64(14982)

// mandatoryBreakBaselinesMP is every DRAWN baseline, top of page first,
// grouped by the element that emits it. e2 contributes only two, because
// its middle line is empty and draws nothing — the gap between its two
// entries is what makes that line observable at all.
var mandatoryBreakBaselinesMP = []struct {
	id string
	ys []int64
}{
	{"e1", []int64{774131, 759149}},
	{"e2", []int64{714131, 684167}},
	{"e3", []int64{634131, 619149}},
}

// mandatoryBreakDrawnBaselines flattens the table above into the order
// linesByOrigin reports, and mandatoryBreakAssertBaselines is the shared
// per-element check both the ordinary suite and every matrix leg run.
func mandatoryBreakDrawnBaselines() []int64 {
	var out []int64
	for _, el := range mandatoryBreakBaselinesMP {
		out = append(out, el.ys...)
	}
	return out
}

// mandatoryBreakAssertBaselines checks the drawn baselines PER ELEMENT,
// which a total cannot: a regression that moved a baseline out of e1 and
// into e3 preserves the count, the ordering and the six values' sum.
// Each element's own inter-baseline interval is checked too — one
// advance for e1 and e3, TWO for e2, whose empty line occupies one full
// Advance and draws nothing.
//
// It reports through fail(format, args...) so the ordinary suite can use
// t.Errorf and the matrix legs t.Fatalf, without a second copy of the
// arithmetic.
func mandatoryBreakAssertBaselines(ys []int64, fail func(string, ...any)) {
	want := mandatoryBreakDrawnBaselines()
	if len(ys) != len(want) {
		fail("the render occupies %d distinct drawn baselines %v, want %d %v (e1=2 + e2=2 drawn of 3 + e3=2)", len(ys), ys, len(want), want)
		return
	}
	at := 0
	for _, el := range mandatoryBreakBaselinesMP {
		for i, wantY := range el.ys {
			if ys[at+i] != wantY {
				fail("%s's drawn baseline %d is at pdfY %d, want the hand-derived %d", el.id, i, ys[at+i], wantY)
			}
		}
		// e2's paragraph gap is TWO advances; every other element's is
		// one. Derived from the declared table rather than restated, so
		// the two cannot disagree.
		for i := 1; i < len(el.ys); i++ {
			wantGap := el.ys[i-1] - el.ys[i]
			if gap := ys[at+i-1] - ys[at+i]; gap != wantGap {
				fail("%s: the interval between drawn baselines %d and %d is %d mp, want %d", el.id, i-1, i, gap, wantGap)
			}
			if wantGap != mandatoryBreakAdvanceMP && wantGap != 2*mandatoryBreakAdvanceMP {
				fail("test-data defect: %s's declared interval %d mp is neither one advance (%d) nor two", el.id, wantGap, mandatoryBreakAdvanceMP)
			}
		}
		at += len(el.ys)
	}
}

// renderMandatoryBreak renders the fixture through the public entry
// point, exactly as a caller would.
func renderMandatoryBreak(t *testing.T) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(mandatoryBreakTemplateJSON))
	if err != nil {
		t.Fatalf("parse mandatory-break template: %v", err)
	}
	res, err := Render(tpl, Data(mandatoryBreakDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render mandatory-break: %v", err)
	}
	return res.Bytes
}

// TestMandatoryBreakSemanticAcceptance is D-000.22's semantic acceptance
// step for fixtures/mandatory-break/, performed AT FIRST RECORDING —
// "a wrong first recording is not a bug that gets caught later, it is a
// bug that gets RATIFIED later".
//
// The numbers below are hand-derived from the committed face, never read
// back off the render:
//
//	A4 is 841,890 mp tall; margins 36 pt; pageHeader height 20 pt, so
//	the content band's origin is 20,000 mp.
//	Noto Sans: hhea ascent 1069, descent -293, lineGap 0.
//	    FirstBaseline = 1069 * 11 = 11,759
//	    Advance       = (1069 + 293 + 0) * 11 = 14,982
//	pdfY of an element's FIRST baseline
//	    = 841890 - 36000 - (20000 + y) - 11759 = 774131 - y
//	and each subsequent DRAWN line is one Advance lower.
//
//	e1 y=0       2 lines:  774131, 759149
//	e2 y= 60000  3 lines:  714131, (empty), 684167   <- TWO advances apart
//	e3 y=140000  2 lines:  634131, 619149
func TestMandatoryBreakSemanticAcceptance(t *testing.T) {
	b := renderMandatoryBreak(t)
	assertWellFormedPDF(t, "mandatory-break golden fixture render", b, 1)

	runs := readEmittedRuns(t, b)
	if len(runs) == 0 {
		t.Fatal("the mandatory-break render emitted no text runs, so every assertion below would be vacuous")
	}
	ys := linesByOrigin(runs)

	// (a) THE DRAWN BASELINES, EXACTLY, AND PER ELEMENT. An empty line
	//     draws nothing, so the artifact carries SIX baselines for seven
	//     lines — and the seventh's existence is what e2's two-advance
	//     interval proves. Checked per element rather than as one flat
	//     list, because a regression that moved a baseline out of one
	//     element and into another preserves the total.
	mandatoryBreakAssertBaselines(ys, t.Errorf)

	// THE VACUITY GUARD THAT MAKES (a) MEAN SOMETHING. A packer that
	// declined every typed break emits ONE baseline per element —
	// three in total, all of them at an element's own first-baseline
	// position, so every "is it in the right place" assertion above
	// would still hold for the three that remained.
	if len(ys) <= len(mandatoryBreakExpectedLines) {
		t.Fatalf("the render occupies only %d baselines for %d elements — no typed break was taken, and this fixture is certifying a document that ate the author's line breaks", len(ys), len(mandatoryBreakExpectedLines))
	}

	// (b) THE DISCRIMINATION THAT MAKES (a)'S INTERVALS MEAN SOMETHING.
	//     e2's two drawn baselines must be strictly further apart than
	//     e1's and e3's — that difference is the only way a line nobody
	//     drew is observable in the produced bytes, and it is what
	//     D-7.1.2's height claim comes to in practice. Read off the
	//     ARTIFACT, and compared between elements rather than against a
	//     literal, so a uniform change to the advance cannot satisfy it.
	// (a) has already REPORTED a wrong baseline count, but it reports
	//     through t.Errorf and execution continues. Indexing six baselines
	//     out of a shorter slice would PANIC, and a panic in one test takes
	//     the whole package's test binary down with it — every other test in
	//     folio-go would stop reporting, which is the DW-23 shape (one
	//     signal swallowing another) in miniature. Measured at closure: with
	//     the D-7.1.1 exemption neutered this test panicked rather than
	//     failed. Stop at the report instead.
	if len(ys) != len(mandatoryBreakDrawnBaselines()) {
		return
	}
	e1Gap, e2Gap, e3Gap := ys[0]-ys[1], ys[2]-ys[3], ys[4]-ys[5]
	if e1Gap != e3Gap {
		t.Errorf("e1's and e3's baseline intervals differ (%d vs %d) — both hold two ordinary consecutive lines", e1Gap, e3Gap)
	}
	if e2Gap != 2*e1Gap {
		t.Errorf("e2's drawn baselines are %d mp apart against e1's %d — the paragraph gap must be exactly TWICE an ordinary interval, or the empty line is not occupying its full Advance", e2Gap, e1Gap)
	}

	// (c) The declared face is embedded and named.
	if !containsFontFile2(b) {
		t.Fatal("the mandatory-break render carries no FontFile2 — the fixture would certify nothing about embedding")
	}
	if !strings.Contains(string(b), "NotoSans") {
		t.Error("the render does not name the embedded face NotoSans")
	}

	// (d) AC17's /ToUnicode section sizes.
	assertToUnicodeSectionsUnderCap(t, b)

	t.Logf("mandatory-break semantic acceptance: %d text runs across %d drawn baselines %v; e2's paragraph gap shows as a %d mp interval against e1's ordinary %d (ruled advance %d)", len(runs), len(ys), ys, e2Gap, e1Gap, mandatoryBreakAdvanceMP)
}

// TestMandatoryBreakLayoutProperties asserts the per-element line
// structure the produced PDF flattens away: which runes land on which
// line, and which lines are empty.
func TestMandatoryBreakLayoutProperties(t *testing.T) {
	tpl, err := ParseTemplate([]byte(mandatoryBreakTemplateJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	layouts := elementLayouts(t, tpl, mandatoryBreakDataJSON)
	if len(layouts) == 0 {
		t.Fatal("no text elements were laid out")
	}

	wantTexts := map[string][]string{
		"e1": {"Yours", "sincerely"},
		"e2": {"Clause 1.", "", "Clause 2."},
		"e3": {"first", "second word"},
	}

	fitting := 0
	for _, el := range layouts {
		want, ok := mandatoryBreakExpectedLines[el.id]
		if !ok {
			t.Errorf("unexpected element %q in the fixture", el.id)
			continue
		}
		if len(el.lines) != want {
			t.Errorf("%s: laid out %d lines %q, want %d %q", el.id, len(el.lines), lineTexts(el), want, wantTexts[el.id])
			continue
		}
		if got := lineTexts(el); !equalStrings(got, wantTexts[el.id]) {
			t.Errorf("%s: laid out as %q, want %q", el.id, got, wantTexts[el.id])
		}

		// e1 and e2 must FIT their boxes, or their breaks could have
		// been taken for want of room (D-7.1.7, finding 2). e3 must
		// NOT, because its overflow is AC11's own case.
		if el.id == "e3" {
			if el.fullWidth <= el.box {
				t.Errorf("e3: its value measures %d against a %d box — it fits, so nothing overflows and the declaration is not discriminated", el.fullWidth, el.box)
			}
		} else {
			if el.fullWidth > el.box {
				t.Errorf("%s: its value measures %d against a %d box — it does NOT fit, so its break may have been taken for want of room rather than because the author typed it", el.id, el.fullWidth, el.box)
			}
			fitting++
		}

		for _, i := range mandatoryBreakEmptyLines[el.id] {
			ln := el.lines[i]
			if ln.from != ln.to || ln.width != 0 {
				t.Errorf("%s line %d must be EMPTY, got %+v", el.id, i, ln)
			}
		}
	}
	if fitting == 0 {
		t.Fatal("no element in this fixture fits its declared box, so packLines' short-circuit — where AC1 is won or lost — is not exercised here at all")
	}
	t.Logf("%d of this fixture's elements FIT their declared boxes and still break where the author typed a break", fitting)
}

// TestMandatoryBreakDeclarationIsLoadBearing is the fixture's
// both-directions red-proof, and it is what makes e3 worth having
// (D-7.1.1).
//
// e3's bound value carries BOTH a line feed and a space, inside ONE
// declared-unbreakable span, so the two failure directions are
// distinguishable:
//
//   - flip the kind test at opportunity.go's atomic-span filter site and
//     the LINE FEED is suppressed with everything else: e3 becomes ONE
//     line, and the first assertion below fails naming the line feed;
//   - remove the declaration instead and the SPACE breaks too: e3
//     becomes THREE lines, which is polarity 2 below.
//
// Asserting only the declared case could not tell "the exemption works"
// from "span suppression broke".
func TestMandatoryBreakDeclarationIsLoadBearing(t *testing.T) {
	tpl, err := ParseTemplate([]byte(mandatoryBreakTemplateJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Polarity 1 — DECLARED, as the fixture ships.
	declared := elementLayoutByID(t, elementLayouts(t, tpl, mandatoryBreakDataJSON), "e3")
	if len(declared.atomic) == 0 {
		t.Fatal("precondition: e3 reports no atomic span, so the declaration is not reaching the breaker at all")
	}
	runes := []rune(declared.text)
	lf := -1
	for i, r := range runes {
		if r == '\n' {
			lf = i
		}
	}
	if lf < 0 {
		t.Fatalf("precondition: e3's bound value %q carries no line feed", declared.text)
	}
	// THE FIXTURE TRAP (D-7.1.1's correction): the span must STRICTLY
	// contain the line feed, or the exemption is never exercised and
	// this element passes vacuously.
	covered := false
	for _, sp := range declared.atomic {
		if lf > sp.Start && lf < sp.End {
			covered = true
		}
	}
	if !covered {
		t.Fatalf("precondition: the line feed at rune %d is not STRICTLY inside any of e3's declared spans %+v — the atomic-span filter would never have seen it, and this element would certify nothing about D-7.1.1", lf, declared.atomic)
	}
	if got := lineTexts(declared); !equalStrings(got, []string{"first", "second word"}) {
		t.Errorf("declared: e3 laid out as %q, want [first \"second word\"] — the TYPED break must survive the declaration and the SPACE must not", got)
	}

	// Polarity 2 — UNDECLARED. Same template, same data, same box; the
	// document's unbreakableValues list emptied and nothing else.
	stripped, err := ParseTemplate([]byte(strings.Replace(
		mandatoryBreakTemplateJSON,
		`"unbreakableValues": ["customer.note"],`,
		``,
		1,
	)))
	if err != nil {
		t.Fatalf("parse stripped template: %v", err)
	}
	if len(stripped.doc.UnbreakableValues) != 0 {
		t.Fatalf("the stripped template still declares %v — the edit did not take, and polarity 2 would assert nothing", stripped.doc.UnbreakableValues)
	}
	undeclared := elementLayoutByID(t, elementLayouts(t, stripped, mandatoryBreakDataJSON), "e3")
	if len(undeclared.atomic) != 0 {
		t.Fatalf("the stripped template still produced atomic spans %+v", undeclared.atomic)
	}
	got := lineTexts(undeclared)
	if !equalStrings(got, []string{"first", "second", "word"}) {
		t.Errorf("undeclared: e3 laid out as %q, want [first second word] — without the declaration the SPACE must break too, or the declared case proves nothing", got)
	}
	if len(got) == len(lineTexts(declared)) {
		t.Errorf("declared and undeclared both produced %d lines — this element does not discriminate the declaration and the fixture needs a narrower box", len(got))
	}

	t.Logf("D-7.1.1 at the golden level: declared %q vs undeclared %q", lineTexts(declared), got)
}

// TestMandatoryBreakOverflowIsTheExistingClipAndWarn is AC11 and the
// "no new diagnostic code" half of D-7.1.3: e3's second line overflows
// its declared width, and what comes back is the EXISTING
// DiagCodeTextClippedWidth Warning beside the bytes — never a new code,
// never a fatal.
func TestMandatoryBreakOverflowIsTheExistingClipAndWarn(t *testing.T) {
	tpl, err := ParseTemplate([]byte(mandatoryBreakTemplateJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Render(tpl, Data(mandatoryBreakDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("the render produced no bytes")
	}
	clipped := 0
	for _, d := range res.Diagnostics {
		switch d.Code {
		case DiagCodeTextClippedWidth:
			if d.ElementID != "e3" {
				t.Errorf("a width-clip Warning names element %q; only e3 overflows", d.ElementID)
			}
			if d.Severity != SeverityWarning {
				t.Errorf("the width-clip diagnostic is %q, want a Warning — an overflow is never fatal", d.Severity)
			}
			clipped++
		default:
			t.Errorf("unexpected diagnostic %+v — this fixture must produce the EXISTING clip Warning and nothing else", d)
		}
	}
	if clipped != 1 {
		t.Errorf("the render produced %d width-clip Warning(s), want exactly 1 (e3's declared value does not fit its 40 pt box)", clipped)
	}
}

// TestMandatoryBreakGoldenFixture is the byte-identity half: the live
// render must reproduce fixtures/mandatory-break/expected.pdf exactly.
//
// It runs AFTER the semantic assertions above in file order and in
// intent (D-000.22): a hash frozen before anyone checked what it
// contained certifies only that the bytes have not changed.
func TestMandatoryBreakGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", "mandatory-break")

	inputPath := filepath.Join(dir, "input.folio")
	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read %s: %v", inputPath, err)
	}
	if string(inputBytes) != mandatoryBreakTemplateJSON {
		t.Fatalf(
			"%s has drifted from folio-go/mandatoryBreakTemplateJSON (mandatory_break_template.go) — the two are "+
				"supposed to be byte-identical (kept in sync by hand, per wrapped-text's precedent)",
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

	b := renderMandatoryBreak(t)
	got := sha256Hex(b)

	expectedPDF, err := os.ReadFile(filepath.Join(dir, "expected.pdf"))
	if err != nil {
		t.Fatalf("read expected.pdf: %v", err)
	}
	if onDisk := sha256Hex(expectedPDF); onDisk != fixture.SHA256 {
		t.Fatalf(
			"fixtures/mandatory-break/expected.pdf's own sha256 (%s) does not match expected.json's recorded sha256 (%s) — the fixture's two halves have drifted apart",
			onDisk, fixture.SHA256,
		)
	}
	if got != fixture.SHA256 {
		t.Fatalf(
			"golden fixture mismatch: got sha256 %s, want %s (fixtures/mandatory-break). Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change. Do not regenerate the fixture to make this pass.",
			got, fixture.SHA256,
		)
	}
}

// mandatoryBreakAdvanceIsTheRuledOne is the redundant-but-cheap tie
// between the hand-derived advance this fixture's baselines are stated
// in and the production function that produced them. LABELLED REDUNDANT
// (D-000.42) and NOT counted as this fixture's coverage: any change that
// moved lineAdvance would redden the artifact assertion first.
func TestMandatoryBreakAdvanceIsTheRuledOne(t *testing.T) {
	got, err := lineAdvance([]string{"Noto Sans"}, geom.Length(11000), defaultLineSpacing, testShippedFontSet(), newFontCache())
	if err != nil {
		t.Fatalf("lineAdvance: %v", err)
	}
	if want := geom.Length(mandatoryBreakAdvanceMP); got != want {
		t.Errorf("REDUNDANT CHECK: the fixture's leading is %d millipoints, want %d ((1069 + 293 + 0) * 11)", got, want)
	}
}
