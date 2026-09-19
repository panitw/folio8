package folio8

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtures/thai-stacked-marks/ is the golden for Story 8.0: a glyph the
// shaper gives a NON-ZERO YOffset, expressed in the PDF with the
// text-rise operator Ts.
//
// It is the corpus's twenty-second document and the FIRST that could
// ever have held such a glyph. Until this story internal/pdf refused one
// outright — a hard Render error and ZERO bytes — so a fixture carrying
// one would have had nothing to record. That is why the first
// twenty-one goldens go without one, and it is why "ordinary Thai does
// not render" was found by the owner pasting a real contract into the
// shipped designer rather than by a test (DW-28, HIGH).
//
// The three rises this document emits, all measured, none assumed:
//
//	YOffset -2  at 12 pt -> -0.024 Ts   (line 0)
//	YOffset -57 at 12 pt -> -0.684 Ts   (line 4)
//	YOffset -59 at 12 pt -> -0.708 Ts   (line 4)
//
// The last two land on the SAME CID, one after the other, which is what
// makes this document evidence that the rise is a property of a glyph
// OCCURRENCE and not of a glyph id.

// thaiStackedMarksRises is the set of rise operands this document must
// emit, declared once and read by every assertion below AND by
// matrix_test.go's per-leg feature guard (which is matrix-tagged and
// would otherwise carry a second copy — the two-literals hazard that
// makes a guard agree with itself while disagreeing with the document).
var thaiStackedMarksRises = []string{"-0.024 Ts\n", "-0.684 Ts\n", "-0.708 Ts\n"}

// thaiStackedMarksControlHex is e2 — สัญญา, the control — as the emitter
// draws it: five glyphs, no vertical offset on any of them, and
// therefore NO Ts operator in its run at all.
const thaiStackedMarksControlHex = "<0052>-7<0011>7<003c003c0023>"

func renderThaiStackedMarks(t *testing.T) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(thaiStackedMarksTemplateJSON))
	if err != nil {
		t.Fatalf("parse thai-stacked-marks template: %v", err)
	}
	res, err := Render(tpl, Data(thaiStackedMarksDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render thai-stacked-marks: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Fatalf("the thai-stacked-marks fixture must render with NO diagnostics; got %+v", res.Diagnostics)
	}
	return res.Bytes
}

// TestThaiStackedMarksFixtureCarriesTheOwnersClause binds the fixture to
// the document that motivated it.
//
// thaiOwnersClause is the contractor-liability clause from the owner's
// real Thai contract, and thai_mark_stacking_test.go's characterization
// arm reads the SAME const. If the fixture's value ever drifts away from
// it, the golden stops being a recording of the reported failure and
// becomes a recording of whatever it drifted into — which is precisely
// the substitution D-000.22 warns gets RATIFIED rather than caught.
func TestThaiStackedMarksFixtureCarriesTheOwnersClause(t *testing.T) {
	if !strings.Contains(thaiStackedMarksTemplateJSON, thaiOwnersClause) {
		t.Fatalf("fixtures/thai-stacked-marks/ no longer carries the owner's clause verbatim — the golden must record the document DW-28 reported, not a paraphrase of it.\nwant to contain: %s", thaiOwnersClause)
	}
	// ...and the CONTROL must still be there, or the document stops
	// witnessing the byte-identity half inside itself.
	if !strings.Contains(thaiStackedMarksTemplateJSON, `"value": "สัญญา"`) {
		t.Fatal(`fixtures/thai-stacked-marks/ has lost its control element สัญญา — without a zero-offset run in the SAME document, nothing here says the Ts path is entered only when YOffset != 0`)
	}
}

// thaiStackedMarksAssertRises is this document's SEMANTIC GUARD,
// DECLARED ONCE here and read by this file's own test AND by
// matrix_test.go's per-leg feature guard (which is matrix-tagged and
// would otherwise carry a second copy — the two-literals hazard that
// makes a guard agree with itself while disagreeing with the document).
//
// It asserts four things about the emitted content stream:
//
//  1. every declared rise is present, as an operand of Ts;
//  2. no rise is left set when its run ends — text rise is a PERSISTENT
//     text-state parameter that survives ET, and buildTextContentStream
//     brackets a run in q/Q only when it is clipped or coloured, so a run
//     that walked away with the rise set would displace whatever drew
//     next;
//  3. the control run emits NO Ts at all; and
//  4. at least one segment at a non-zero rise takes the `<hex> Tj` fast
//     path, which is what says the rise machinery did not quietly force
//     every segment into a TJ array.
//
// fatalf is the caller's own failure reporter, so a matrix leg can name
// its target in the message.
func thaiStackedMarksAssertRises(t *testing.T, raw []byte, fatalf func(format string, args ...any)) {
	t.Helper()

	streams := splitPageContentStreams(t, raw)
	if len(streams) != 1 {
		fatalf("thai-stacked-marks is a one-page document; got %d page content stream(s)", len(streams))
		return
	}
	content := streams[0]

	for _, rise := range thaiStackedMarksRises {
		if !strings.Contains(content, rise) {
			fatalf("the emitted content stream carries no %q — the shaper's vertical offset did not reach the page", rise)
			return
		}
	}

	// (2) EVERY RUN BEGINS AND ENDS AT RISE ZERO.
	blocks := strings.Split(content, "BT\n")[1:]
	if len(blocks) == 0 {
		fatalf("the emitted content stream carries no BT block — this guard would assert nothing about a document that drew no text")
		return
	}
	risenBlocks, controlBlocks, tjAtRise := 0, 0, 0
	for i, block := range blocks {
		end := strings.Index(block, "ET\n")
		if end == -1 {
			fatalf("run %d: unterminated BT block", i)
			return
		}
		block = block[:end]

		ops := textRiseOperands(t, i, block)
		if len(ops) == 0 {
			if strings.Contains(block, thaiStackedMarksControlHex) {
				controlBlocks++
			}
			continue
		}
		risenBlocks++
		if ops[len(ops)-1] != "0" {
			fatalf("run %d ends at text rise %s, not 0 — rise survives ET and this run has no q/Q bracket (it is neither clipped nor coloured), so the next thing drawn would be displaced.\nrun: %q", i, ops[len(ops)-1], block)
			return
		}
		if ops[0] == "0" {
			fatalf("run %d opens by setting the rise to 0 — every run already begins at rise 0, so this is a byte nothing asked for.\nrun: %q", i, block)
			return
		}
		// (4) A NON-ZERO-rise segment whose own terms are all zero must
		// still take the `<hex> Tj` fast path.
		//
		// Split on " Ts\n" yields one piece per segment BOUNDARY, so
		// pieces[k] is the segment opened by the k-th Ts and ops[k-1] is
		// that Ts's operand. Both must be read: a piece opened by a
		// RESTORING `0 Ts` is a segment at rise zero, and counting one
		// of those would let this check pass with no non-zero-rise
		// segment on the fast path at all — which is the opposite of
		// what it claims. And only the piece's FIRST show-text operator
		// is its own; a `> Tj` further along belongs to a later segment
		// at a different rise.
		for k, seg := range strings.Split(block, " Ts\n")[1:] {
			if k >= len(ops) || ops[k] == "0" {
				continue
			}
			first := seg
			if nl := strings.Index(first, "\n"); nl != -1 {
				first = first[:nl]
			}
			if strings.HasPrefix(first, "<") && strings.HasSuffix(first, "> Tj") {
				tjAtRise++
			}
		}
	}
	if risenBlocks == 0 {
		fatalf("no run in this document carries a text rise — the fixture no longer witnesses its own subject")
		return
	}
	if controlBlocks == 0 {
		fatalf("the control run %s was not found without a Ts operator — without it this document says nothing about the zero-offset path", thaiStackedMarksControlHex)
		return
	}
	if tjAtRise == 0 {
		fatalf("no segment at a non-zero rise took the `<hex> Tj` fast path — the segment emitter must choose Tj vs TJ by the SAME rule at every rise, and this document contains a segment whose terms are all zero")
		return
	}

	t.Logf("thai-stacked-marks witness — %d run(s) carry a text rise and every one restores 0 Ts before its ET; %d control run(s) carry none; %d risen segment(s) took the Tj fast path", risenBlocks, controlBlocks, tjAtRise)
}

// TestThaiStackedMarksEmitsTheRise is the semantic half, and it runs
// BEFORE the digest assertion in file order and in intent (D-000.22): a
// hash frozen before anyone checked what it contained certifies only
// that the bytes have not changed.
func TestThaiStackedMarksEmitsTheRise(t *testing.T) {
	thaiStackedMarksAssertRises(t, renderThaiStackedMarks(t), t.Fatalf)
}

// textRiseOperands returns, in order, the operand of every Ts operator
// in one BT..ET block. It fatals on a Ts with no operand rather than
// skipping it: a scan that silently ignores what it cannot read reports
// a clean run for a broken one (D-000.36).
func textRiseOperands(t *testing.T, run int, block string) []string {
	t.Helper()
	var ops []string
	fields := strings.Fields(block)
	for i, f := range fields {
		if f != "Ts" {
			continue
		}
		if i == 0 {
			t.Fatalf("run %d: a Ts operator with no operand before it: %q", run, block)
		}
		ops = append(ops, fields[i-1])
	}
	return ops
}

// TestThaiStackedMarksGoldenFixture is the byte-identity half: the live
// render must reproduce fixtures/thai-stacked-marks/expected.pdf exactly,
// and the committed input.folio must still be byte-identical to the const
// this package renders (the hand-sync precedent font-text,
// multi-script-fallback, wrapped-text, mandatory-break, line-spacing,
// justified-text, justified-thai and alignment-rounding set).
func TestThaiStackedMarksGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", "thai-stacked-marks")

	inputPath := filepath.Join(dir, "input.folio")
	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read %s: %v", inputPath, err)
	}
	if string(inputBytes) != thaiStackedMarksTemplateJSON {
		t.Fatalf(
			"%s has drifted from folio-go/thaiStackedMarksTemplateJSON (thai_stacked_marks_template.go) — the two are "+
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

	b := renderThaiStackedMarks(t)
	got := sha256Hex(b)

	expectedPDF, err := os.ReadFile(filepath.Join(dir, "expected.pdf"))
	if err != nil {
		t.Fatalf("read expected.pdf: %v", err)
	}
	if onDisk := sha256Hex(expectedPDF); onDisk != fixture.SHA256 {
		t.Fatalf(
			"fixtures/thai-stacked-marks/expected.pdf's own sha256 (%s) does not match expected.json's recorded sha256 (%s) — the fixture's two halves have drifted apart",
			onDisk, fixture.SHA256,
		)
	}
	if got != fixture.SHA256 {
		t.Fatalf(
			"golden fixture mismatch: got sha256 %s, want %s (fixtures/thai-stacked-marks). Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change. Do not regenerate the fixture to make this pass.",
			got, fixture.SHA256,
		)
	}
}
