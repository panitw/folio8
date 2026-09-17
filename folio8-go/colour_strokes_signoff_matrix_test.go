//go:build matrix

// colour_strokes_signoff_matrix_test.go is the attestation gate for
// fixtures/colour-strokes/ (SPEC-client-libraries story 3, DW-147), and like
// declared_variants_signoff_matrix_test.go, its precedent, it is a TEST
// rather than a log entry.
//
// IT SHIPS RED. The story recorded this fixture's golden and froze its hash
// before the owner had looked at the page, on the condition the
// declared-variants precedent set (D-11.5.1, arm [A]): the obligation is
// tracked by a test that FAILS until the reading is recorded, because an
// obligation held only in a register is one nobody trips over. It is a
// TRANSIENT red — a countdown that must reach zero before the
// folio8-go/v1.0.0 tag, which DW-147 gates — and not one of the suite's
// permanent mandated reds.
//
// WHY THIS FIXTURE NEEDS ITS OWN READING. Every other attested artifact was
// judged for shaping, placement or which face reached the page. The
// judgment owed here is about COLOUR: does each ink, fill and stroke land
// on the element and the edges the document declares, in the colour it
// declares? A swapped fill and stroke, a border on the wrong edges, or a
// header drawn in the cells' ink all produce a deterministic, byte-identical
// PDF that is wrong, and only a person looking at the page sees that.
//
// NO AGENT INVENTS THIS RECORD. Writing `reader`, `date` or `examined` on
// someone else's behalf is a fabricated attestation (D-000.28). The file is
// written only on the owner's explicit instruction, in
// fixtures/declared-variants/signoff.json's shape, and says on its face how
// it was produced.
package folio8

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// colourStrokesSignOffFileName is the record written into
// fixtures/colour-strokes/ once the owner has examined expected.pdf.
const colourStrokesSignOffFileName = "signoff.json"

// TestColourStrokesSemanticSignOffIsRecorded is the attestation gate. It
// reuses thaiSignOff's schema, as the declared-variants gate does, and binds
// the record to the LIVE digest of expected.pdf, so a re-record invalidates
// a reading of bytes nobody has looked at.
func TestColourStrokesSemanticSignOffIsRecorded(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", colourStrokesFixtureDir)
	path := filepath.Join(dir, colourStrokesSignOffFileName)

	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Fatalf(
			"fixtures/colour-strokes/%s does not exist, so the owner's sign-off of the colour golden "+
				"(DW-147, SPEC-client-libraries story 3) is still OUTSTANDING.\n\n"+
				"This is not a broken test. The story recorded this fixture's golden before anyone had "+
				"looked at the page, on the condition that the obligation is tracked by a failing test "+
				"(the declared-variants precedent, D-11.5.1 arm [A]). It is a transient red that must "+
				"clear before the folio8-go/v1.0.0 tag.\n\n"+
				"THE OWNER'S STEP: open fixtures/colour-strokes/expected.pdf and check that\n"+
				"  1. the heading is dark navy (#1B2A4A) on a pale cream band (#FFF4D6), with a red\n"+
				"     (#C81E1E) stroke on the band's BOTTOM and LEFT edges and none on its top or right;\n"+
				"  2. the note under it is purple (#6A1B9A);\n"+
				"  3. the box is outlined in green (#2E7D32) and the line is blue (#1565C0);\n"+
				"  4. the table has a navy header row (#1B2A4A) with white labels, dark grey cell text\n"+
				"     (#37474F), purple lines (#8E24AA) between columns and rows, a teal outer frame\n"+
				"     (#00838F), and a pale blue fill (#E3F2FD) on the second, fourth and sixth rows.\n\n"+
				"Then %s is written, by the owner or on the owner's explicit instruction, as:\n"+
				"  {\n"+
				"    \"reader\":   \"<name>\",\n"+
				"    \"date\":     \"<YYYY-MM-DD>\",\n"+
				"    \"examined\": \"<what was looked at and what was seen>\",\n"+
				"    \"sha256\":   \"<the sha256 from expected.json, unchanged>\"\n"+
				"  }\n"+
				"and goldenDigestRecord (byte_neutrality_test.go) declares it as a \"signoff\" site.\n\n"+
				"If the colours are WRONG, the fixture is re-recorded instead.\n\n"+
				"NO AGENT MAY WRITE THIS FILE UNASKED. reader, date and examined are claims about a person "+
				"having looked, and one written on their behalf is a fabricated attestation.",
			colourStrokesSignOffFileName, colourStrokesSignOffFileName,
		)
	}
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var rec thaiSignOff
	if uerr := json.Unmarshal(raw, &rec); uerr != nil {
		t.Fatalf("fixtures/colour-strokes/%s is not valid JSON: %v", colourStrokesSignOffFileName, uerr)
	}
	for _, f := range []struct{ name, value string }{
		{"reader", rec.Reader},
		{"date", rec.Date},
		{"examined", rec.Examined},
		{"sha256", rec.SHA256},
	} {
		if strings.TrimSpace(f.value) == "" {
			t.Errorf(
				"fixtures/colour-strokes/%s has an empty %q. Every field is required: an unattributed, "+
					"undated or unspecific sign-off is indistinguishable from no sign-off",
				colourStrokesSignOffFileName, f.name,
			)
		}
	}
	if t.Failed() {
		return
	}
	if _, perr := time.Parse("2006-01-02", strings.TrimSpace(rec.Date)); perr != nil {
		t.Fatalf(
			"fixtures/colour-strokes/%s has date %q, which is not ISO-8601 (YYYY-MM-DD): %v",
			colourStrokesSignOffFileName, rec.Date, perr,
		)
	}

	pdf, rerr := os.ReadFile(filepath.Join(dir, "expected.pdf"))
	if rerr != nil {
		t.Fatalf("read fixtures/colour-strokes/expected.pdf: %v", rerr)
	}
	if len(pdf) == 0 {
		t.Fatal("fixtures/colour-strokes/expected.pdf is empty — there is nothing here for a reading to be a reading of")
	}
	live := sha256Hex(pdf)
	if strings.TrimSpace(rec.SHA256) != live {
		t.Fatalf(
			"the sign-off in fixtures/colour-strokes/%s names digest\n  %s\nbut fixtures/colour-strokes/expected.pdf now hashes to\n  %s\n\n"+
				"The fixture has been RE-RECORDED since it was signed off, so the sign-off applies to bytes "+
				"nobody has looked at. Re-open expected.pdf, judge the colours again, and update both the "+
				"digest and what was examined. Do NOT simply paste the new digest in.",
			colourStrokesSignOffFileName, rec.SHA256, live,
		)
	}

	t.Logf(
		"colour-strokes semantic sign-off present: reader %q, date %s, digest %s",
		rec.Reader, rec.Date, rec.SHA256,
	)
}
