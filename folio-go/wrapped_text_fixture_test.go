package folio8

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/text"
)

// wrappedTextExpectedLines is AC15(a), stated as LITERALS before the
// fixture's hash is frozen, because "the line count is whatever it came
// out as" would certify nothing.
//
// Every number here was MEASURED first (Story 2.4's dev record) against
// the shipped chain at 11 pt, and each element's full text was measured
// to be WIDER than its box — otherwise "it wrapped" would be satisfied
// by an input that fitted all along:
//
//	e1  Latin  full 331,683 mp against a 150,000 mp box -> 3 lines
//	e2  Thai   full 192,830 mp against a 150,000 mp box -> 2 lines
//	e3  CJK    full 198,000 mp against a 150,000 mp box -> 2 lines
//	e4  span   full  46,585 mp against a  20,000 mp box -> 2 lines
var wrappedTextExpectedLines = map[string]int{
	"e1": 3,
	"e2": 2,
	"e3": 2,
	"e4": 2,
}

// renderWrappedText renders the fixture through the public entry point,
// exactly as a caller would.
func renderWrappedText(t *testing.T) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(wrappedTextTemplateJSON))
	if err != nil {
		t.Fatalf("parse wrapped-text template: %v", err)
	}
	res, err := Render(tpl, Data(wrappedTextDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render wrapped-text: %v", err)
	}
	return res.Bytes
}

// linesByOrigin groups emitted runs into lines by their Tm y
// translation. Every run on one line shares a y; a wrapped element emits
// one distinct y per line.
//
// Read off the PRODUCED PDF, never off the renderer's intermediate
// state: the property AC15 asserts is about the document that ships.
func linesByOrigin(runs []emittedRun) []int64 {
	seen := map[int64]bool{}
	var ys []int64
	for _, r := range runs {
		if !seen[r.OriginYMilli] {
			seen[r.OriginYMilli] = true
			ys = append(ys, r.OriginYMilli)
		}
	}
	sort.Slice(ys, func(i, j int) bool { return ys[i] > ys[j] })
	return ys
}

// TestWrappedTextSemanticAcceptance is D-000.22's semantic acceptance
// step for fixtures/wrapped-text/, performed AT FIRST RECORDING. Every
// property here is machine-checkable and none of it is deferrable
// (D-2.3.5): only the break-placement judgment is pending, and that is
// bound to the expected-breaks fixture, not to this one (D-2.4.3, DN-4
// confirmed — no third sign-off).
func TestWrappedTextSemanticAcceptance(t *testing.T) {
	b := renderWrappedText(t)
	assertWellFormedPDF(t, "wrapped-text golden fixture render", b, 1)

	runs := readEmittedRuns(t, b)
	if len(runs) == 0 {
		t.Fatal("the wrapped-text render emitted no text runs, so every assertion below would be vacuous")
	}
	ys := linesByOrigin(runs)

	// (a) The document occupies the expected number of lines in total.
	// Counted as DISTINCT baselines, which is the artifact-level
	// consequence of wrapping.
	wantTotal := 0
	for _, n := range wrappedTextExpectedLines {
		wantTotal += n
	}
	if len(ys) != wantTotal {
		t.Errorf("the render occupies %d distinct baselines %v, want %d (e1=3 + e2=2 + e3=2 + e4=2)", len(ys), ys, wantTotal)
	}

	// THE VACUITY GUARD THAT MAKES (a) MEAN SOMETHING. An unwrapped
	// engine emits ONE baseline per element — four in total. If this
	// fixture ever reports four, wrapping has silently stopped
	// happening and every other assertion here would still pass.
	if len(ys) <= len(wrappedTextExpectedLines) {
		t.Fatalf("the render occupies only %d baselines for %d elements — nothing wrapped, and this fixture is certifying an unwrapped document", len(ys), len(wrappedTextExpectedLines))
	}

	// (d) THE BASELINES THE ARTIFACT ACTUALLY CARRIES ARE EVENLY SPACED
	//     WITHIN AN ELEMENT BY THE RULED ADVANCE.
	//
	// REPAIRED BY STORY 2.5a UNDER D-000.44. What stood here was the
	// purest D-000.21 instance in the module: it called the production
	// lineAdvance, compared the answer to a literal, and then — having
	// already computed `ys`, the distinct baselines of the produced
	// document, three statements above — USED ONLY THEIR COUNT AND
	// THREW THE VALUES AWAY. It asserted on the INPUT while being named
	// for semantic acceptance, and its own comment made a claim about
	// the ARTIFACT ("the baselines are evenly spaced") that nothing
	// checked against the artifact.
	//
	// The ruled advance (D-2.4.2 as AMENDED), hand-derived:
	//
	//	max(A)   = 1160 (Noto Sans SC)
	//	max(|D|) =  450 (Noto Sans Thai)   <- a DIFFERENT face
	//	max(gap) =    0
	//	units    = 1160 + 450 + 0 = 1610
	//	at 11 pt: 1610 * 11 = 17710 mp
	//
	// The SUPERSEDED rule — max(A - D + gap), the worst single face —
	// gave 1511 * 11 = 16621, which is the literal that used to stand
	// here. It is retained below as the hypothesis this assertion
	// discriminates against.
	const wantAdvance = geom.Length(17710)
	const supersededAdvance = geom.Length(16621)

	// NON-DEGENERACY PRECONDITION (D-000.33). "Evenly spaced" is
	// satisfied TRIVIALLY by an element with one baseline, and by a
	// document whose elements each have one. The spacing claim means
	// nothing until at least one element contributes two consecutive
	// baselines, so that is established first and the count of
	// intervals actually compared is reported.
	intervals := 0
	base := 0
	for _, id := range []string{"e1", "e2", "e3", "e4"} {
		n := wrappedTextExpectedLines[id]
		if base+n > len(ys) {
			t.Fatalf("presence precondition: the render carries %d baselines, too few to cover %s's %d lines starting at index %d", len(ys), id, n, base)
		}
		// ys is sorted DESCENDING in pdfY, i.e. top of page first, so
		// each successive line within an element is `advance` LOWER.
		for i := base + 1; i < base+n; i++ {
			gap := geom.Length(ys[i-1] - ys[i])
			if gap == supersededAdvance {
				t.Errorf("%s: the observed spacing between baselines %d and %d is %d mp, which is exactly max(hhea A - D + gap) over the chain — the SUPERSEDED worst-single-face rule. The amended rule maximises each axis independently: max(A)=1160 (Noto Sans SC) + max(|D|)=450 (Noto Sans Thai) = 1610 units = %d mp at 11 pt",
					id, i-1, i, gap, wantAdvance)
			} else if gap != wantAdvance {
				t.Errorf("%s: the observed spacing between baselines %d and %d is %d mp, want the hand-derived %d mp (D-2.4.2 amended)", id, i-1, i, gap, wantAdvance)
			}
			intervals++
		}
		base += n
	}
	if intervals == 0 {
		t.Fatal("vacuity: no element in this fixture contributed two consecutive baselines, so the spacing assertion above compared nothing (D-000.33)")
	}

	// THE PRODUCTION FUNCTION AGREES WITH WHAT THE ARTIFACT CARRIES.
	//
	// LABELLED REDUNDANT (D-000.42), NOT COUNTED AS COVERAGE. This
	// comparison has no teeth that the artifact assertion above and
	// TestVerticalModelArithmeticOverFabricatedMetrics do not already
	// have: any change that moved lineAdvance would redden both of
	// them first. It is kept as cheap belt-and-braces that ties the
	// artifact back to the symbol that produced it — and it is
	// explicitly NOT one of the checks this fixture claims as coverage.
	got, err := lineAdvance([]string{"Noto Sans", "Noto Sans Thai", "Noto Sans SC"}, geom.Length(11000), defaultLineSpacing, testShippedFontSet(), newFontCache())
	if err != nil {
		t.Fatalf("lineAdvance: %v", err)
	}
	if got != wantAdvance {
		t.Errorf("REDUNDANT CHECK: the fixture's leading is %d millipoints, want %d (D-2.4.2 amended)", got, wantAdvance)
	}

	// (e) The declared page size and the embedded faces are what the
	// template asked for.
	if !containsFontFile2(b) {
		t.Fatal("the wrapped-text render carries no FontFile2 — the fixture would certify nothing about embedding")
	}
	for _, want := range []string{"NotoSans", "NotoSansThai", "NotoSansSC"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("the render does not name the embedded face %q; all three shipped faces must appear, since the fixture uses all three scripts", want)
		}
	}

	// (f) AC17's /ToUnicode section sizes.
	assertToUnicodeSectionsUnderCap(t, b)

	t.Logf("wrapped-text semantic acceptance: %d text runs across %d baselines %v; %d OBSERVED inter-baseline intervals all equal the ruled %d mp (superseded rule would give %d)", len(runs), len(ys), ys, intervals, wantAdvance, supersededAdvance)
}

// TestWrappedTextLayoutProperties is AC15 (b), (c) and (d): the
// per-element line properties, asserted against the layout the renderer
// actually produced for each element.
func TestWrappedTextLayoutProperties(t *testing.T) {
	tpl, err := ParseTemplate([]byte(wrappedTextTemplateJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	layouts := elementLayouts(t, tpl, wrappedTextDataJSON)
	if len(layouts) == 0 {
		t.Fatal("no text elements were laid out")
	}

	dict := text.Dictionary()
	var overflowing int
	for _, el := range layouts {
		want, ok := wrappedTextExpectedLines[el.id]
		if !ok {
			t.Errorf("unexpected element %q in the fixture", el.id)
			continue
		}

		// (a) per element.
		if len(el.lines) != want {
			t.Errorf("%s: laid out %d lines, want %d", el.id, len(el.lines), want)
		}

		// PRESENCE PRECONDITION: the element's full text must be wider
		// than its box, or "it wrapped" is satisfied vacuously.
		if el.fullWidth <= el.box {
			t.Errorf("%s: its text measures %d against a %d box — it fits, so this element exercises no wrapping", el.id, el.fullWidth, el.box)
		}

		runes := []rune(el.text)
		boundary := text.ClusterBoundaries(runes)
		for i, ln := range el.lines {
			// (b) no line exceeds its box, EXCEPT where a single atomic
			// unit cannot fit. That exception is not a blanket
			// permission: it is granted only when the line carries no
			// interior break opportunity at all.
			if ln.width > el.box {
				overflowing++
				interior := 0
				for _, op := range el.ops {
					if op.LineEnd > ln.from && op.LineEnd < ln.to {
						interior++
					}
				}
				if interior != 0 {
					t.Errorf("%s line %d [%d,%d) is %d wide against a %d box AND carries %d interior break opportunities — it should have been broken, not overflowed", el.id, i, ln.from, ln.to, ln.width, el.box, interior)
				}
			}

			// (c) no line boundary falls inside a Thai character cluster.
			for _, pos := range []int{ln.from, ln.to} {
				if pos > 0 && pos < len(runes) && !boundary[pos] {
					t.Errorf("%s line %d has a boundary at rune %d, strictly inside a Thai character cluster of %q", el.id, i, pos, el.text)
				}
			}

			// (d) no line boundary falls inside a declared span.
			for _, sp := range el.atomic {
				for _, pos := range []int{ln.from, ln.to} {
					if pos > sp.Start && pos < sp.End {
						t.Errorf("%s line %d has a boundary at rune %d, strictly inside the declared unbreakable span %+v (%q)", el.id, i, pos, sp, string(runes[sp.Start:sp.End]))
					}
				}
			}
		}
		_ = dict
	}

	// The overflow path must actually be exercised by this fixture, or
	// (b)'s exception branch is dead code that no test covers.
	if overflowing == 0 {
		t.Error("no line in the fixture overflows its box, so AC11's visible-overflow rule is not exercised here — e4's box is supposed to be narrower than its declared value")
	}
}

// TestWrappedTextDeclarationIsLoadBearing is AC7's both-polarity
// red-proof at the GOLDEN level, and it is the assertion that makes the
// fixture's e4 worth having.
//
// Measured: with "customer.name" declared, e4 lays out as
// "ผู้รับ" / "ศรีสุข" — the surname whole, overflowing its 20,000 mp box
// at 24,585. With the declaration removed and NOTHING else changed, it
// lays out as "ผู้รับ" / "ศรี" / "สุข" — the surname split at the seam
// between the two common dictionary words it is spelled with, which is
// D-2.1.6's worked case exactly.
//
// Asserting only the declared case would be vacuous: it could not
// distinguish an honoured declaration from a value that never had a
// break opportunity.
func TestWrappedTextDeclarationIsLoadBearing(t *testing.T) {
	tpl, err := ParseTemplate([]byte(wrappedTextTemplateJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// Polarity 1 — DECLARED, as the fixture ships.
	declared := elementLayoutByID(t, elementLayouts(t, tpl, wrappedTextDataJSON), "e4")
	if len(declared.atomic) == 0 {
		t.Fatal("precondition: e4 reports no atomic span, so the declaration is not reaching the breaker at all")
	}
	if got := lineTexts(declared); !equalStrings(got, []string{"ผู้รับ", "ศรีสุข"}) {
		t.Errorf("declared: e4 laid out as %q, want [ผู้รับ ศรีสุข] — the declared value must stay whole", got)
	}

	// Polarity 2 — UNDECLARED. Same template, same data, same box; the
	// document's unbreakableValues list emptied and nothing else.
	stripped, err := ParseTemplate([]byte(strings.Replace(
		wrappedTextTemplateJSON,
		`"unbreakableValues": ["customer.name"],`,
		``,
		1,
	)))
	if err != nil {
		t.Fatalf("parse stripped template: %v", err)
	}
	if len(stripped.doc.UnbreakableValues) != 0 {
		t.Fatalf("the stripped template still declares %v — the edit did not take, and polarity 2 would assert nothing", stripped.doc.UnbreakableValues)
	}
	undeclared := elementLayoutByID(t, elementLayouts(t, stripped, wrappedTextDataJSON), "e4")
	if len(undeclared.atomic) != 0 {
		t.Fatalf("the stripped template still produced atomic spans %+v", undeclared.atomic)
	}
	got := lineTexts(undeclared)
	if !equalStrings(got, []string{"ผู้รับ", "ศรี", "สุข"}) {
		t.Errorf("undeclared: e4 laid out as %q, want [ผู้รับ ศรี สุข] — without the declaration the surname must split, or the declared case proves nothing", got)
	}
	if len(got) == len(lineTexts(declared)) {
		t.Errorf("declared and undeclared both produced %d lines — this element does not discriminate the declaration and the fixture needs a narrower box", len(got))
	}

	t.Logf("AC7 at the golden level: declared %q vs undeclared %q", lineTexts(declared), got)
}

// TestWrappedTextObservabilityRedProof is AC15's own red-proof: point
// this fixture's line-count assertion at fixtures/shaped-text/, every
// element of which FITS its box (M4: the widest is 26% of its box), and
// it must report one line per element and fail.
//
// A fixture that cannot fail this way is not exercising wrapping, and
// this is the test that proves fixtures/wrapped-text/ can.
func TestWrappedTextObservabilityRedProof(t *testing.T) {
	root := repoRootFromTest(t)
	raw, err := os.ReadFile(filepath.Join(root, "fixtures", "shaped-text", "input.folio"))
	if err != nil {
		t.Fatalf("read shaped-text input: %v", err)
	}
	tpl, err := ParseTemplate(raw)
	if err != nil {
		t.Fatalf("parse shaped-text: %v", err)
	}
	layouts := elementLayouts(t, tpl, `{}`)
	if len(layouts) == 0 {
		t.Fatal("shaped-text laid out no elements")
	}
	for _, el := range layouts {
		if len(el.lines) != 1 {
			t.Errorf("shaped-text %s laid out %d lines, want 1 — this fixture is supposed to FIT its boxes, which is what makes it the negative control for wrapping", el.id, len(el.lines))
		}
		if el.fullWidth > el.box {
			t.Errorf("shaped-text %s measures %d against a %d box; M4 recorded every element as fitting", el.id, el.fullWidth, el.box)
		}
	}
	t.Logf("negative control: all %d shaped-text elements occupy exactly one line, so wrapped-text's multi-line counts are a real signal", len(layouts))
}

// assertToUnicodeSectionsUnderCap is AC17. The ToUnicode CMap
// specification caps a beginbfchar section at 100 entries, and
// internal/pdf.buildToUnicodeCMap emits one unbounded section (DW-14).
//
// EXCEEDING THE CAP IS STOP-AND-ESCALATE, NOT A FIX TO MAKE HERE: the
// remedy is chunking into <=100-entry sections, and that moves the hash
// of every golden document over the cap. DW-14's own text says it "wants
// to land with a deliberate re-record rather than as a drive-by".
func assertToUnicodeSectionsUnderCap(t *testing.T, b []byte) {
	t.Helper()
	re := regexp.MustCompile(`(?s)(\d+)\s+beginbfchar(.*?)endbfchar`)
	ms := re.FindAllStringSubmatch(string(b), -1)
	if len(ms) == 0 {
		t.Fatal("the render carries no beginbfchar section, so AC17 measures nothing")
	}
	var sizes []int
	for _, m := range ms {
		sizes = append(sizes, strings.Count(m[2], "<")/2)
	}
	for i, n := range sizes {
		if n > 100 {
			t.Errorf(
				"AC17 / DW-14 TRIGGERED: beginbfchar section %d carries %d entries, over the ToUnicode "+
					"specification's 100-entry cap. STOP AND ESCALATE — the fix (chunking into <=100-entry "+
					"sections) moves the golden hash of every document over the cap, and must land as a "+
					"deliberate re-record rather than as a drive-by.",
				i, n,
			)
		}
	}
	t.Logf("AC17: %d beginbfchar sections, sizes %v, cap 100 — DW-14 not triggered", len(sizes), sizes)
}

func lineTexts(el elementLayout) []string {
	runes := []rune(el.text)
	out := make([]string, 0, len(el.lines))
	for _, ln := range el.lines {
		out = append(out, string(runes[ln.from:ln.to]))
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func elementLayoutByID(t *testing.T, layouts []elementLayout, id string) elementLayout {
	t.Helper()
	for _, el := range layouts {
		if el.id == id {
			return el
		}
	}
	t.Fatalf("element %q not found among the laid-out elements", id)
	return elementLayout{}
}

// TestWrappedTextGoldenFixture is AC15's byte-identity half: the live
// render must reproduce fixtures/wrapped-text/expected.pdf exactly.
//
// It runs AFTER the semantic assertions above in file order and in
// intent: D-000.22 requires the semantic acceptance step at first
// recording, and a hash that was frozen before anyone checked what it
// contained certifies only that the bytes have not changed.
func TestWrappedTextGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", "wrapped-text")

	inputPath := filepath.Join(dir, "input.folio")
	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read %s: %v", inputPath, err)
	}
	if string(inputBytes) != wrappedTextTemplateJSON {
		t.Fatalf(
			"%s has drifted from folio-go/wrappedTextTemplateJSON (wrapped_text_template.go) — the two are "+
				"supposed to be byte-identical (kept in sync by hand, per font-text's precedent)",
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
		t.Fatalf("fixture sha256 %q is not 64 lower-case hex characters (AC16)", fixture.SHA256)
	}

	b := renderWrappedText(t)
	got := sha256Hex(b)

	expectedPDF, err := os.ReadFile(filepath.Join(dir, "expected.pdf"))
	if err != nil {
		t.Fatalf("read expected.pdf: %v", err)
	}
	if onDisk := sha256Hex(expectedPDF); onDisk != fixture.SHA256 {
		t.Fatalf(
			"fixtures/wrapped-text/expected.pdf's own sha256 (%s) does not match expected.json's recorded sha256 (%s) — the fixture's two halves have drifted apart",
			onDisk, fixture.SHA256,
		)
	}
	if got != fixture.SHA256 {
		t.Fatalf(
			"golden fixture mismatch: got sha256 %s, want %s (fixtures/wrapped-text). Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change. Do not regenerate the fixture to make this pass.",
			got, fixture.SHA256,
		)
	}
}
