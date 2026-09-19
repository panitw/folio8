//go:build matrix

// declared_variants_signoff_matrix_test.go is Story 11.5's attestation
// gate, and like its four precedents it is a TEST rather than a log
// entry.
//
// IT IS GREEN NOW, AND IT SHIPPED RED. Story 11.5 froze
// fixtures/declared-variants/ before any human had looked at the page.
// That is permitted on the condition D-2.3.5 set for
// fixtures/shaped-text/ and D-11.5.1 re-applied here as arm [A]: the
// obligation is tracked by a test that FAILS until a person writes the
// record, because an obligation held only in a register is an obligation
// nobody trips over. Arm [B] — closing the story and filing the reading
// as a deferral — leaves a tree that says "done" and a register that says
// "not really", a state indistinguishable from its opposite by looking.
//
// The story shipped this test failing and HALTED with expected.pdf a
// candidate. Panit Wechasil read the page on 2026-09-06 and the record
// landed in fixtures/declared-variants/signoff.json, which discharged the
// halt. The mechanism below is unchanged and is not a historical note: it
// goes RED again, by construction, the moment expected.pdf is re-recorded
// — which is the whole point of binding a reading to a digest.
//
// THE OBLIGATION IS DOCTRINAL, NOT ARCHITECTURAL, and its descent is
// D-000.22 → D-2.3.5 (D-11.5.2). AD-21 says nothing about human
// attestation and D-4.7.1 is scoped to the statement family; the
// distinction is load-bearing rather than pedantic, because an
// architectural invariant cannot be deferred, so had the obligation lived
// in AD-21 this red-gate arm would have been forced rather than chosen.
//
// THE RED WAS TRANSIENT, AND D-11.5.1 DREW THE LINE BY NAME.
// TestCorpusMeetsP6ExerciseFloors and its P6g subtest are PERMANENT
// mandated reds — a floor nobody has met. This one cleared the moment the
// owner read the page and the record was written. A permanent red teaches
// everyone to ignore a number; a transient one is a countdown, and this
// countdown ran to zero inside the story that started it. The matrix
// suite is back to TWO named failures, both permanent; a third whose name
// is not one of those two is a hard stop.
//
// WHY THIS FIXTURE NEEDED ITS OWN READING rather than riding on any
// existing record. Every other attested artifact in this repository was
// judged for a property of SHAPING or PLACEMENT — Thai marks, break
// opportunities, a statement's readable layout. The judgment owed here is
// a different one and no existing record touches it: is line 2 a REAL
// BOLD FACE, or a regular face thickened? That is a question about which
// font program reached the page, and it is answerable only by eye.
//
// IT IS A SEPARATE FILE from fixtures/statement-signoff.json on purpose:
// that record's digests map is slug-set-checked against the four
// statement fixtures, so a non-statement digest entering it would break
// that check rather than extend it.
//
// NO AGENT INVENTS THIS RECORD. Writing `reader`, `date` or `examined`
// on someone else's behalf is a fabricated attestation and it is the
// single failure the whole mechanism exists to prevent (D-000.28: a claim
// written before the event it asserts is false from birth). The record
// that exists says on its face how it was produced — a verbatim
// transcription of the reader's own words and judgment, made at the
// reader's explicit instruction — so that a later reader can weigh it
// rather than having to guess.
package folio8

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// declaredVariantsSignOffFileName is the record the owner writes into
// fixtures/declared-variants/ once they have looked at expected.pdf.
const declaredVariantsSignOffFileName = "signoff.json"

// TestDeclaredVariantsSemanticSignOffIsRecorded is the attestation gate.
// It shipped red, it is green as of the record dated 2026-09-06, and it
// reds again on the next re-record. It reuses thaiSignOff's schema
// deliberately: the four fields are load-bearing for the same reasons
// there, and a second schema would be a second place for the rule to
// drift.
//
// THE ABSENCE MESSAGE BELOW IS NOT DEAD PROSE. It is written for the next
// time this gate fires — a re-record stales the record, and whoever hits
// it then needs the same instructions the first reader did.
//
// IT BINDS TO THE LIVE DIGEST OF expected.pdf, not to expected.json's
// recorded one. The two agree today and the untagged fixture test
// asserts they must — but the thing a reading is a reading OF is the
// BYTES SOMEBODY OPENED, and binding to the sidecar would let a
// re-recorded PDF keep a stale attestation for as long as its sidecar was
// updated in the same stroke. A moved golden must invalidate the reading,
// which is D-2.3.5's second condition, and the only way to say that
// without a loophole is to hash the artifact.
func TestDeclaredVariantsSemanticSignOffIsRecorded(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", declaredVariantsFixtureDir)
	path := filepath.Join(dir, declaredVariantsSignOffFileName)

	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Fatalf(
			"fixtures/declared-variants/%s does not exist, so Story 11.5's human semantic acceptance "+
				"step is still OUTSTANDING.\n\n"+
				"This is not a broken test. Story 11.5 recorded this fixture's golden and froze its hash "+
				"before anyone had looked at the page, on the condition (D-11.5.1, arm [A]) that the "+
				"obligation is tracked by a failing test rather than a register entry. This is that test, "+
				"and it clears the moment the record lands.\n\n"+
				"To resolve it: open fixtures/declared-variants/expected.pdf. Four lines, each naming the "+
				"cut it is set in. The question is only: IS LINE 2 A REAL BOLD FACE, OR A REGULAR FACE "+
				"THICKENED? The tells, in order of reliability:\n"+
				"  1. COUNTERS — the enclosed white inside a, e, o, g. A real bold keeps them open and\n"+
				"     shaped; a smeared regular chokes them toward slits. Line 2's \"Handgloves\" against\n"+
				"     line 1's is the direct comparison.\n"+
				"  2. STEM-TO-ROUND CONTRAST — a real bold thickens vertical stems more than the thin\n"+
				"     parts of curves. A synthetic bold thickens everything uniformly and looks inflated\n"+
				"     rather than drawn.\n"+
				"  3. WIDTH — a real Roboto Bold is slightly wider; line 2 should end marginally right of\n"+
				"     line 1's comparable point, not sit exactly on top of it.\n"+
				"  4. LINES 3 AND 4 — a real italic is a DRAWN italic, not a slanted regular: check a, f\n"+
				"     and e for different letterform construction rather than the same shapes leaning.\n\n"+
				"Then write %s as:\n"+
				"  {\n"+
				"    \"reader\":   \"<your name>\",\n"+
				"    \"date\":     \"<YYYY-MM-DD>\",\n"+
				"    \"examined\": \"<what you looked at and what you saw>\",\n"+
				"    \"sha256\":   \"<the sha256 from expected.json, unchanged>\"\n"+
				"  }\n\n"+
				"If the faces are WRONG, the fixture is re-recorded instead — that is the outcome this "+
				"gate exists to make possible, and it is far cheaper than the reverse.\n\n"+
				"NO AGENT MAY WRITE THIS FILE. reader, date and examined are claims about a person "+
				"having looked, and one written on their behalf is a fabricated attestation.",
			declaredVariantsSignOffFileName, declaredVariantsSignOffFileName,
		)
	}
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var rec thaiSignOff
	if uerr := json.Unmarshal(raw, &rec); uerr != nil {
		t.Fatalf("fixtures/declared-variants/%s is not valid JSON: %v", declaredVariantsSignOffFileName, uerr)
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
				"fixtures/declared-variants/%s has an empty %q. Every field is required: an "+
					"unattributed, undated or unspecific sign-off is indistinguishable from no sign-off",
				declaredVariantsSignOffFileName, f.name,
			)
		}
	}
	if t.Failed() {
		return
	}

	// The date must be a DATE. A non-empty check passes "soon" and
	// "yesterday", neither of which can be aged, and ageing is the whole
	// reason the field exists.
	if _, perr := time.Parse("2006-01-02", strings.TrimSpace(rec.Date)); perr != nil {
		t.Fatalf(
			"fixtures/declared-variants/%s has date %q, which is not ISO-8601 (YYYY-MM-DD): %v",
			declaredVariantsSignOffFileName, rec.Date, perr,
		)
	}

	// The digest binds the record to the bytes that were READ. A
	// re-record moves it and so invalidates this record BY CONSTRUCTION,
	// which is the stale-provenance defect D-2.3.5's second condition
	// exists to close. Hashed off the artifact, not read out of the
	// sidecar — see this test's doc comment.
	pdf, rerr := os.ReadFile(filepath.Join(dir, "expected.pdf"))
	if rerr != nil {
		t.Fatalf("read fixtures/declared-variants/expected.pdf: %v", rerr)
	}
	if len(pdf) == 0 {
		t.Fatal("fixtures/declared-variants/expected.pdf is empty — there is nothing here for a reading to be a reading of")
	}
	live := sha256Hex(pdf)
	if strings.TrimSpace(rec.SHA256) != live {
		t.Fatalf(
			"the sign-off in fixtures/declared-variants/%s names digest\n  %s\nbut fixtures/declared-variants/expected.pdf now hashes to\n  %s\n\n"+
				"The fixture has been RE-RECORDED since it was signed off, so the sign-off applies to bytes "+
				"nobody has looked at. This is deliberate and is D-2.3.5's anti-rot condition: binding the "+
				"sign-off to the digest makes a re-record automatically invalidate it and demand a fresh "+
				"look.\n\nRe-open expected.pdf, judge the four cuts again, and update both the digest and "+
				"what you examined. Do NOT simply paste the new digest in.",
			declaredVariantsSignOffFileName, rec.SHA256, live,
		)
	}

	t.Logf(
		"declared-variants semantic sign-off present: reader %q, date %s, digest %s",
		rec.Reader, rec.Date, rec.SHA256,
	)
}
