//go:build matrix

// expected_breaks_signoff_matrix_test.go is Story 2.4's pending human
// obligation, and — like Story 2.3's before it — it is a FAILING TEST
// rather than a note, because D-2.3.5's first binding condition says the
// pending obligation must be one: "gated as the matrix legs are — so the
// story commits green and the GATE cannot pass. A note is optional; a
// red gate is not."
//
// WHY THIS IS A SECOND RECORD AND NOT AN EXTENSION OF THE FIRST
// (D-2.4.3, ruled). "Does this Thai READ correctly" and "do these BREAK
// POINTS fall correctly" are two distinct human judgments. One sign-off
// covering both is the conflation D-000.23 warns about, with an added
// hazard the ruling names directly: a reader signing off on legibility
// would not know they were also certifying break placement.
//
// So this record is bound to its OWN digest — the sha256 of
// fixtures/expected-breaks/expected_breaks.json — and the Thai READING
// sign-off's own artifact and digest (see fixtures/shaped-text/README.md's
// provenance for its current value) are not touched, read, or extended by
// this file. A re-record of either fixture must invalidate EXACTLY ONE of
// the two sign-offs and not the other. That is the property, and it is why
// the schema and the binding are duplicated here rather than shared:
// sharing them would couple the two invalidations back together.
//
// CORRECTED by the finisher (Story 2.6 finisher, Finding 6): this comment
// used to name the reading sign-off's digest as a literal, 5964aad0…c92e00f
// — which had gone stale (shaped-text/expected.pdf was re-recorded by
// Story 2.5a; its digest is now 6c040ef7…c6c85370) and named a file,
// fixtures/shaped-text/thai-signoff.json, that has never existed. Both were
// comment-only and read by no live assertion. Named by pointer rather than
// by a second copy of a value that restales exactly like this one did
// (Story 2.3a's finding on position- and value-bound citations).
//
// After this story the Epic 2 gate owes THREE things: the four-target
// matrix legs, the Thai RENDERING sign-off, and this one. That is
// sanctioned and stated, not smuggled.
//
// THE BOUNDARY THIS DOES NOT CROSS. D-000.22's machine-checkable half is
// never deferrable, and none of it is deferred here. That the labels are
// self-consistent with their own segmentation, that the engine
// reproduces every one of them exactly, that both polarities are present
// and neither is vacuous — all asserted AT RECORDING, untagged, on every
// target, in internal/text/s4_expected_test.go. Only "would a Thai
// reader put the line breaks in these places" is pending, because
// nothing substitutes for it.
package folio8

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// breakSignOffFileName is the record a Thai reader writes into
// fixtures/expected-breaks/ once they have read the labels.
const breakSignOffFileName = "break-signoff.json"

// breakSignOff is this record's schema. It is deliberately NOT the
// shaped-text record's type: the two answer different questions, bind to
// different digests, and must be able to invalidate independently.
type breakSignOff struct {
	// Reader is the human who read the labels. A name, not a role and
	// not an agent — the labels in the fixture were authored by an
	// agent, which is exactly why this field cannot be.
	Reader string `json:"reader"`
	// Date is when they read them, ISO-8601 (YYYY-MM-DD).
	Date string `json:"date"`
	// Examined is what they checked and what they concluded — specific
	// enough that a later reader can tell it from "looked fine".
	Examined string `json:"examined"`
	// SHA256 is the sha256 of expected_breaks.json as signed off. Any
	// edit to the fixture moves it and invalidates this record by
	// construction.
	SHA256 string `json:"sha256"`
}

// TestExpectedBreaksHumanSignOffIsRecorded is the red gate for S4's
// break labels.
func TestExpectedBreaksHumanSignOffIsRecorded(t *testing.T) {
	dir := filepath.Join(repoRootFromTest(t), "fixtures", "expected-breaks")
	fixturePath := filepath.Join(dir, "expected_breaks.json")
	path := filepath.Join(dir, breakSignOffFileName)

	fixtureBytes, ferr := os.ReadFile(fixturePath)
	if ferr != nil {
		t.Fatalf("read fixtures/expected-breaks/expected_breaks.json: %v", ferr)
	}
	sum := sha256.Sum256(fixtureBytes)
	digest := hex.EncodeToString(sum[:])

	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Fatalf(
			"fixtures/expected-breaks/%s does not exist, so Story 2.4's human hand-check of the S4 break "+
				"labels is still OUTSTANDING and the Epic 2 boundary gate cannot pass.\n\n"+
				"This is not a broken test. acceptance.md:65 requires the expected-break fixture to be "+
				"\"hand-checked once\" and then frozen, and no agent may make that claim — the labels in "+
				"expected_breaks.json were WRITTEN by an agent, so an agent confirming them would be "+
				"marking its own work.\n\n"+
				"To resolve it: open fixtures/expected-breaks/expected_breaks.json and read the `words` "+
				"field of each item — that is the label. For each one, ask only: would a Thai (or Chinese, "+
				"or Japanese) reader accept a line ending at each of those seams, and NOT inside any of the "+
				"words? Note especially thai-003/004/005/006/010, which are lexicalised compounds "+
				"labelled as ONE word each and never yet confirmed by a person; thai-007/008/009, which "+
				"are ALSO lexicalised compounds but which the 2026-08-24 hand-check split anyway (no "+
				"wordlist property distinguishes them from the five kept whole — see the fixture's "+
				"_README); thai-001/thai-002, which are deliberately labelled as two words; "+
				"cjk-004/cjk-005/cjk-007 (first pass, 2026-08-24) and cjk-001/cjk-002/cjk-003/cjk-006 "+
				"(second pass, 2026-08-25), all seven of which carry `declaredAtomic: true` and are "+
				"deliberately never split regardless of what a bare-string reading of the same "+
				"characters would propose; and cjk-008, a NEW undeclared item exercising the engine's "+
				"standard per-character CJK breaking whose label is derived from UAX #14, not a "+
				"native-speaker judgment, and is therefore NOT something this sign-off needs to "+
				"adjudicate the way the human-labelled items above are. Then write %s as:\n"+
				"  {\n"+
				"    \"reader\":   \"<your name>\",\n"+
				"    \"date\":     \"<YYYY-MM-DD>\",\n"+
				"    \"examined\": \"<what you checked and what you concluded>\",\n"+
				"    \"sha256\":   \"%s\"\n"+
				"  }\n\n"+
				"If a label is WRONG, correct expected_breaks.json instead — the fixture is the oracle and "+
				"the engine follows it, never the reverse. That outcome is the reason this gate exists.\n\n"+
				"This obligation is SEPARATE from fixtures/shaped-text/thai-signoff.json, which answers a "+
				"different question (does the Thai render legibly) and is bound to a different digest. "+
				"Resolving one does not resolve the other.",
			breakSignOffFileName, breakSignOffFileName, digest,
		)
	}
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var rec breakSignOff
	if uerr := json.Unmarshal(raw, &rec); uerr != nil {
		t.Fatalf("fixtures/expected-breaks/%s is not valid JSON: %v", breakSignOffFileName, uerr)
	}

	// D-000.21 sharpened: prove the record carries its fields before
	// treating it as evidence.
	for _, f := range []struct{ name, value string }{
		{"reader", rec.Reader},
		{"date", rec.Date},
		{"examined", rec.Examined},
		{"sha256", rec.SHA256},
	} {
		if strings.TrimSpace(f.value) == "" {
			t.Errorf(
				"fixtures/expected-breaks/%s has an empty %q. Every field is required: an unattributed, "+
					"undated or unspecific sign-off is indistinguishable from no sign-off at all",
				breakSignOffFileName, f.name,
			)
		}
	}
	if t.Failed() {
		return
	}

	// The anti-rot binding, computed over the fixture's own bytes rather
	// than read from a field inside it — this fixture records no digest
	// of itself, and a self-recorded digest could not survive its own
	// insertion anyway.
	if rec.SHA256 != digest {
		t.Fatalf(
			"the break sign-off in %s names digest\n  %s\nbut fixtures/expected-breaks/expected_breaks.json "+
				"now hashes to\n  %s\n\n"+
				"The labels have been EDITED since they were signed off, so the sign-off applies to labels "+
				"nobody has read. Binding the sign-off to the digest makes an edit automatically invalidate "+
				"it and demand a fresh read.\n\n"+
				"Re-read the changed labels and update both the digest and what you examined. Do NOT simply "+
				"paste the new digest in.",
			breakSignOffFileName, rec.SHA256, digest,
		)
	}

	t.Logf(
		"S4 break-label sign-off present: reader %q, date %s, digest %s",
		rec.Reader, rec.Date, rec.SHA256,
	)
}
