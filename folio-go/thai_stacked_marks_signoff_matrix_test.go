//go:build matrix

// thai_stacked_marks_signoff_matrix_test.go is Story 8.0's
// pending-obligation mechanism, and like its two precedents it is a
// FAILING TEST rather than a log entry.
//
// Story 8.0 froze fixtures/thai-stacked-marks/ before any human had
// read the page. That is permitted on the same condition D-2.3.5 set
// for fixtures/shaped-text/: the obligation is tracked by a test that
// is RED until a person writes the record, because an obligation held
// only in a register is an obligation nobody trips over.
//
// WHY THIS FIXTURE NEEDED ITS OWN READING, rather than riding on
// fixtures/shaped-text/thai-signoff.json. That record attests marks
// placed by a GSUB LOWERED-FORM SUBSTITUTION — one glyph, zero vertical
// offset. This fixture's subject is marks placed by a GPOS Y-DISPLACEMENT
// and emitted through PDF's text-rise operator, which nothing in the
// corpus had ever judged before Story 8.0 wrote the first `Ts` bytes in
// this project's history. Two different mechanisms can both be correct
// in the shaper and only one of them be correct on the page, so the
// existing attestation says nothing about this one.
//
// It is a SEPARATE FILE from fixtures/statement-signoff.json on purpose:
// that record's digests map is slug-set-checked against the four
// statement fixtures, so a non-statement digest entering it would break
// that check rather than extend it.
package folio8

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stackedMarksSignOffFileName is the record a Thai reader writes into
// fixtures/thai-stacked-marks/ once they have looked at expected.pdf.
const stackedMarksSignOffFileName = "signoff.json"

// TestThaiStackedMarksSemanticSignOffIsRecorded is the red gate. It
// reuses thaiSignOff's schema deliberately: the four fields are
// load-bearing for the same reasons there, and a second schema would be
// a second place for the rule to drift.
func TestThaiStackedMarksSemanticSignOffIsRecorded(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", "thai-stacked-marks")
	path := filepath.Join(dir, stackedMarksSignOffFileName)

	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Fatalf(
			"fixtures/thai-stacked-marks/%s does not exist, so Story 8.0's human semantic acceptance "+
				"step is still OUTSTANDING.\n\n"+
				"This is not a broken test. Story 8.0 froze this fixture's hash before a Thai reader had "+
				"looked at it, on the condition that the obligation is tracked by a failing test rather "+
				"than a register entry. This is that test.\n\n"+
				"To resolve it: open fixtures/thai-stacked-marks/expected.pdf, read the Thai, and confirm "+
				"that every tone mark sits on the consonant it belongs to — %q is the stacked-mark case "+
				"this story exists to fix, where ั and ้ both sit above ท and are placed by a vertical "+
				"displacement the emitter had previously REFUSED rather than drawn. Then write %s as:\n"+
				"  {\n"+
				"    \"reader\":   \"<your name>\",\n"+
				"    \"date\":     \"<YYYY-MM-DD>\",\n"+
				"    \"examined\": \"<what you looked at and what you saw>\",\n"+
				"    \"sha256\":   \"<the sha256 from expected.json, unchanged>\"\n"+
				"  }\n\n"+
				"If the marks sit WRONG, the fixture is re-recorded instead — that is the outcome this "+
				"gate exists to make possible, and it is far cheaper than the reverse.",
			stackedMarksSignOffFileName, "ทั้งสิ้น", stackedMarksSignOffFileName,
		)
	}
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var rec thaiSignOff
	if uerr := json.Unmarshal(raw, &rec); uerr != nil {
		t.Fatalf("fixtures/thai-stacked-marks/%s is not valid JSON: %v", stackedMarksSignOffFileName, uerr)
	}

	// Prove the record carries its fields before treating it as
	// evidence. An empty string in any of them is a record that exists
	// without saying anything, which satisfies a naive existence check
	// while asserting nothing.
	for _, f := range []struct{ name, value string }{
		{"reader", rec.Reader},
		{"date", rec.Date},
		{"examined", rec.Examined},
		{"sha256", rec.SHA256},
	} {
		if strings.TrimSpace(f.value) == "" {
			t.Errorf(
				"fixtures/thai-stacked-marks/%s has an empty %q. Every field is required: an "+
					"unattributed, undated or unspecific sign-off is indistinguishable from no sign-off",
				stackedMarksSignOffFileName, f.name,
			)
		}
	}
	if t.Failed() {
		return
	}

	// The digest binds the record to the bytes that were read. A
	// re-record moves that digest and so invalidates this record BY
	// CONSTRUCTION, which is the stale-provenance defect D-2.3.5's
	// second condition exists to close.
	assertSignOffMatchesFrozenHash(t, dir, rec)
	t.Logf(
		"Thai stacked-mark sign-off present: reader %q, date %s, digest %s",
		rec.Reader, rec.Date, rec.SHA256,
	)
}
