//go:build matrix

// shaped_signoff_matrix_test.go is Story 2.3's pending-obligation
// mechanism, and it is a FAILING TEST rather than a note on purpose.
//
// D-2.3.5 permits `fixtures/shaped-text/` to be frozen with its Thai
// semantic sign-off outstanding, under three binding conditions, of
// which the first is: "the pending obligation is a FAILING TEST, never a
// log entry... gated as the matrix legs are — so the story commits green
// and the GATE cannot pass. A note is optional; a red gate is not."
//
// That is exactly what the `matrix` build tag buys. The routine
// `go test ./...` every story runs does not compile this file, so Story
// 2.3 commits green; the Epic 2 boundary gate, which is the run that
// carries `-tags=matrix`, does compile it and cannot pass until a Thai
// reader has actually looked at the rendered page and recorded what they
// saw. `go vet -tags=matrix ./...` still compiles it every story, which
// is how a deferral of this kind stays honest rather than rotting.
//
// The second binding condition is the anti-rot property and lives in
// assertSignOffMatchesFrozenHash below: the record names the digest it
// signed off, so re-recording the fixture AUTOMATICALLY invalidates the
// sign-off and demands a fresh look. Without it a sign-off silently
// carries over to bytes no human ever saw — the stale-provenance failure
// this project already closed once on derived fonts.
//
// THE BOUNDARY THIS DOES NOT CROSS (D-2.3.5, stated so it is not
// re-derived): D-000.22's MACHINE-CHECKABLE HALF IS NEVER DEFERRABLE.
// Only its irreducibly-human half is, and only against a mechanical
// blocker. Weight class, name records, TJ adjustment values, axis pins,
// byte-identity across targets — all asserted AT RECORDING, with no
// deferral available, and all of them are asserted today. Only "does
// this Thai read correctly to someone who reads Thai" may be pending,
// because nothing substitutes for it. The evidence for that split is
// this project's own: the Thin-Chinese defect was caught by a
// machine-checkable property, not by a human look.
package folio8

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// thaiSignOffFileName is the record a Thai reader writes into
// fixtures/shaped-text/ once they have looked at expected.pdf.
const thaiSignOffFileName = "thai-signoff.json"

// thaiSignOff is the sign-off record's schema. Every field is required
// and every field is load-bearing: a record missing the reader's name is
// unattributable, one missing the date cannot be aged, one missing what
// was examined cannot be distinguished from a rubber stamp, and one
// missing the digest is the stale-provenance defect D-2.3.5's second
// condition exists to close.
type thaiSignOff struct {
	// Reader is the human who looked. A name, not a role and not an
	// agent: D-000.22's whole point is that this is the one claim no
	// machine and no agent can make on someone else's behalf.
	Reader string `json:"reader"`
	// Date is when they looked, ISO-8601 (YYYY-MM-DD).
	Date string `json:"date"`
	// Examined is what they actually looked at and what they saw —
	// specific enough that a later reader can tell it apart from
	// "looked fine".
	Examined string `json:"examined"`
	// SHA256 is the fixture digest this sign-off applies to. It MUST
	// equal fixtures/shaped-text/expected.json's sha256. A re-record
	// moves that digest and so invalidates this record by construction.
	SHA256 string `json:"sha256"`
}

// TestShapedTextThaiSemanticSignOffIsRecorded is the red gate. It fails
// while the sign-off record is absent, and its message is written for
// the person who has to resolve it rather than for the person who wrote
// it.
func TestShapedTextThaiSemanticSignOffIsRecorded(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", "shaped-text")
	path := filepath.Join(dir, thaiSignOffFileName)

	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Fatalf(
			"fixtures/shaped-text/%s does not exist, so AC10's human semantic acceptance step is still "+
				"OUTSTANDING and the Epic 2 boundary gate cannot pass.\n\n"+
				"This is not a broken test. Story 2.3 froze this fixture's hash before a Thai reader had "+
				"looked at it, which D-2.3.5 permits — on condition that the obligation is tracked by a "+
				"failing test rather than a log entry. This is that test.\n\n"+
				"To resolve it: open fixtures/shaped-text/expected.pdf, read the Thai, confirm that every "+
				"vowel and tone mark sits on the consonant it belongs to (the first line, %q, is the "+
				"lowered-mark case the story exists to fix), and write %s as:\n"+
				"  {\n"+
				"    \"reader\":   \"<your name>\",\n"+
				"    \"date\":     \"<YYYY-MM-DD>\",\n"+
				"    \"examined\": \"<what you looked at and what you saw>\",\n"+
				"    \"sha256\":   \"<the sha256 from expected.json, unchanged>\"\n"+
				"  }\n\n"+
				"If the Thai reads WRONG, the fixture is re-recorded instead — that is the outcome this "+
				"gate exists to make possible, and it is cheaper than the reverse.",
			thaiSignOffFileName, "ปั ฟั ที่ ป้ำ", thaiSignOffFileName,
		)
	}
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var rec thaiSignOff
	if uerr := json.Unmarshal(raw, &rec); uerr != nil {
		t.Fatalf("fixtures/shaped-text/%s is not valid JSON: %v", thaiSignOffFileName, uerr)
	}

	// D-000.21 sharpened: prove the record carries its fields before
	// treating it as evidence. An empty string in any of these is a
	// record that exists without saying anything, which would satisfy a
	// naive existence check while asserting nothing.
	for _, f := range []struct{ name, value string }{
		{"reader", rec.Reader},
		{"date", rec.Date},
		{"examined", rec.Examined},
		{"sha256", rec.SHA256},
	} {
		if strings.TrimSpace(f.value) == "" {
			t.Errorf(
				"fixtures/shaped-text/%s has an empty %q. Every field is required: an unattributed, "+
					"undated or unspecific sign-off is indistinguishable from no sign-off at all",
				thaiSignOffFileName, f.name,
			)
		}
	}
	if t.Failed() {
		return
	}

	assertSignOffMatchesFrozenHash(t, dir, rec)
	t.Logf(
		"Thai semantic sign-off present: reader %q, date %s, digest %s",
		rec.Reader, rec.Date, rec.SHA256,
	)
}

// assertSignOffMatchesFrozenHash is D-2.3.5's second binding condition:
// the sign-off names the hash it signed off, so a later re-recording
// carries no sign-off at all rather than silently carrying one made
// against different bytes.
func assertSignOffMatchesFrozenHash(t *testing.T, dir string, rec thaiSignOff) {
	t.Helper()

	rawExpected, err := os.ReadFile(filepath.Join(dir, "expected.json"))
	if err != nil {
		t.Fatalf("read fixtures/shaped-text/expected.json: %v", err)
	}
	var frozen expectedFixture
	if uerr := json.Unmarshal(rawExpected, &frozen); uerr != nil {
		t.Fatalf("fixtures/shaped-text/expected.json is not valid JSON: %v", uerr)
	}
	if strings.TrimSpace(frozen.SHA256) == "" {
		t.Fatalf("fixtures/shaped-text/expected.json carries no sha256 to bind the sign-off to")
	}

	if rec.SHA256 != frozen.SHA256 {
		t.Fatalf(
			"the Thai sign-off in %s names digest\n  %s\nbut fixtures/shaped-text/expected.json now records\n  %s\n\n"+
				"The fixture has been RE-RECORDED since it was signed off, so the sign-off applies to bytes "+
				"nobody has looked at. This is deliberate and is D-2.3.5's anti-rot condition: binding the "+
				"sign-off to the digest makes a re-record automatically invalidate it and demand a fresh "+
				"look.\n\nRe-read the rendered Thai in expected.pdf and update both the digest and what you "+
				"examined. Do NOT simply paste the new digest in.",
			thaiSignOffFileName, rec.SHA256, frozen.SHA256,
		)
	}
}
