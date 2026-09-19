//go:build matrix

// embedded_font_signoff_matrix_test.go is D-8.4.8's gate over the corpus's
// ONLY TRANSFERRED READING, and like the four human-reading gates beside it,
// it is a FAILING TEST rather than a register entry — an obligation held only
// in a register is an obligation nobody trips over (D-2.3.5).
//
// WHAT IT GATES IS NOT THE SAME THING THEY GATE, AND THE DIFFERENCE IS THE
// WHOLE FILE. shaped_signoff_matrix_test.go, expected_breaks_signoff_matrix_test.go,
// statement_signoff_matrix_test.go and thai_stacked_marks_signoff_matrix_test.go
// each hold open "a person must look at this page". Nobody is going to look at
// fixtures/embedded-font/expected.pdf: D-8.4.8 ruled that the owner's reading
// of fixtures/thai-stacked-marks/ TRANSFERS onto it, because the two pages'
// shaped runs are provably the same computation over the same bytes
// (TestEmbeddedFontShapedRunEqualsAttestedControl asserts exactly that,
// pre-subset). So fixtures/embedded-font/signoff.json carries a DIFFERENT
// SCHEMA on purpose — `kind: "transferred-reading"`, a NOT_A_HUMAN_READING
// statement, and NO `reader`, NO `date` and NO `examined`, because inventing
// any of the three would be a fabricated attestation.
//
// WHAT THIS GATE HOLDS OPEN IS THE LAPSE CONDITION, which is the part of
// D-8.4.8 most easily lost. The equality the transfer rests on is
// LIVE-VS-LIVE: on its own it is the both-sides-move-together shape and would
// survive a shaping regression that hit the embedded path and the FontSet path
// alike. It is non-vacuous ONLY because fixtures/thai-stacked-marks/expected.pdf
// is byte-pinned and the owner's sign-off names that digest — THAT FROZEN
// LITERAL IS WHAT THE CHAIN TERMINATES IN. If that golden is ever re-recorded
// the borrowed reading covers bytes nobody looked at, the transfer LAPSES, and
// both fixtures need a real human reading.
//
// A gate that only checked the record exists would let the transfer silently
// outlive its own basis. This one goes RED on a missing record, on a stale
// covered digest, on a moved anchor, on an anchor sign-off that no longer
// binds, and on any attempt to dress the record up as a human reading.
package folio8

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// embeddedFontTransferredReadingFileName is the record D-8.4.8 authorised and
// the orchestrator wrote. It shares thai_stacked_marks_signoff_matrix_test.go's
// file name and nothing else.
const embeddedFontTransferredReadingFileName = "signoff.json"

// thaiStackedMarksAnchorDigest is THE FROZEN LITERAL, and it is written out
// here rather than read from any file — the same second-literal mechanism
// D-2.3.4 requires of goldenDigestRecord, for the same reason. The cheapest
// available response to a lapsed transfer is to paste the new digest into
// fixtures/embedded-font/signoff.json, and that edit is invisible in a diff
// when only one copy of the number exists. With a second copy here, a lapse
// cannot be papered over without changing a file whose entire subject is the
// lapse.
//
// This is the digest fixtures/thai-stacked-marks/expected.pdf hashed to when
// the owner read that page on 2026-08-31, and it is what
// fixtures/thai-stacked-marks/signoff.json certifies.
//
// NOTE ON SCOPE: this literal sits OUTSIDE goldenDigestSearchScope
// (byte_neutrality_test.go), which covers fixtures/ and that file alone. That
// is deliberate, not an oversight — the completeness half exists to find sites
// a re-record must UPDATE TOGETHER, and this one must emphatically not be
// updated together with anything. It is here to go red.
const thaiStackedMarksAnchorDigest = "d5077f3346e10abb17ec69d2d6e2a975d02524d6e2eebcbec3b85ff30ca48eb1"

// TestEmbeddedFontTransferredReadingHoldsIsTheGate is the red gate.
//
// It re-hashes both artifacts itself rather than trusting either expected.json,
// because a re-record that moved a PDF and its JSON together is exactly the
// movement this gate exists to notice.
func TestEmbeddedFontTransferredReadingHolds(t *testing.T) {
	root := repoRootFromTest(t)
	relPath := filepath.Join("fixtures", "embedded-font", embeddedFontTransferredReadingFileName)
	path := filepath.Join(root, relPath)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf(
			"fixtures/embedded-font/%s does not exist, so the corpus's only Thai-bearing golden with no "+
				"human reading is covered by NOTHING.\n\n"+
				"This is not a broken test. fixtures/embedded-font/expected.pdf draws Thai from a face the "+
				"document carries, and D-8.4.8 ruled that the owner's reading of "+
				"fixtures/thai-stacked-marks/ TRANSFERS onto it rather than a second reading being "+
				"commissioned. That ruling produced a record; this gate holds it open.\n\n"+
				"DO NOT RESOLVE THIS BY WRITING A HUMAN SIGN-OFF. No agent may write \"reader\", \"date\" "+
				"or \"examined\" in any record under fixtures/. If the transferred record is genuinely "+
				"gone, restore it from D-8.4.8 or ask the owner to read the page.",
			embeddedFontTransferredReadingFileName,
		)
	}

	// THE TWO LIVE DIGESTS, RE-HASHED HERE (fixturePDFDigest reads the
	// ARTIFACT, never expected.json: a re-record moves both together, and the
	// point of this gate is to see the movement).
	covered := fixturePDFDigest(t, root, "embedded-font")
	anchor := fixturePDFDigest(t, root, "thai-stacked-marks")

	// THE LAPSE CONDITION, ASSERTED AGAINST THE FROZEN LITERAL FIRST. This is
	// the leg that survives somebody "fixing" the record: even with
	// transfer.anchor_sha256 rewritten to whatever the golden now hashes to,
	// this comparison still fails.
	if anchor != thaiStackedMarksAnchorDigest {
		t.Fatalf(
			"fixtures/thai-stacked-marks/expected.pdf now hashes to %s; the owner's reading of "+
				"2026-08-31 was of %s.\n\n"+
				"THE TRANSFER HAS LAPSED (D-8.4.8). fixtures/embedded-font/signoff.json is not a human "+
				"reading — it borrows that one — and a borrowed reading of bytes that no longer exist "+
				"covers nothing. BOTH fixtures now need a real human reading, and no agent may write "+
				"either of them.\n\n"+
				"DO NOT UPDATE THE LITERAL IN THIS FILE TO MAKE THIS GREEN. The literal is the frozen "+
				"terminus the whole chain rests on; updating it re-asserts an attestation over unread "+
				"bytes, which is the fabrication D-000.28 forbids. If the re-record was deliberate, the "+
				"correct outcome is that this gate is RED until a person has looked.",
			anchor, thaiStackedMarksAnchorDigest)
	}

	// The record itself: schema, non-impersonation, both bindings. Shared
	// verbatim with the ordinary suite's guard in byte_neutrality_test.go, so
	// the rule cannot drift between the two places it is enforced.
	assertTransferredReadingIsRealAndStillBinding(t, root, relPath, covered, anchor)
	if t.Failed() {
		return
	}

	// THE ASSERTION THE RECORD RESTS ON MUST EXIST. `transfer.asserted_by`
	// names a test; a record pointing at a test somebody deleted is a record
	// whose ground has silently gone, and the deletion would be invisible
	// here without this.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	var rec transferredReading
	if uerr := json.Unmarshal(raw, &rec); uerr != nil {
		t.Fatalf("%s is not valid JSON: %v", relPath, uerr)
	}
	assertNamedTestExists(t, root, relPath, rec.Transfer.AssertedBy)

	t.Logf(
		"D-8.4.8: the transferred reading holds. Covered %s (fixtures/embedded-font), anchored on %s "+
			"(fixtures/thai-stacked-marks, read by a human 2026-08-31), asserted by %s.",
		covered, anchor, rec.Transfer.AssertedBy)
}

// assertNamedTestExists checks that a named Go test function is declared
// somewhere in this module's test sources. It is a source scan rather than a
// reflective lookup because the test being named may be in another package or
// behind another build tag.
func assertNamedTestExists(t *testing.T, root, relPath, name string) {
	t.Helper()
	if strings.TrimSpace(name) == "" {
		t.Errorf("%s names no asserting test in transfer.asserted_by. The transfer's whole ground is an asserted equality; a record that does not name the assertion cannot be checked against it.", relPath)
		return
	}
	needle := "func " + name + "("
	moduleRoot := filepath.Join(root, "folio-go")
	found := false
	scanned := 0
	werr := filepath.WalkDir(moduleRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		scanned++
		if strings.Contains(string(src), needle) {
			found = true
		}
		return nil
	})
	if werr != nil {
		t.Fatalf("walk %s: %v", moduleRoot, werr)
	}
	if scanned == 0 {
		t.Fatal("vacuity guard: the walk read 0 test files, so \"the asserting test exists\" says nothing")
	}
	if !found {
		t.Errorf(
			"%s names %s as the assertion its transfer rests on, and no such test function is declared "+
				"anywhere in folio-go. The ground for the transferred reading has gone; either restore "+
				"the assertion or the record covers nothing.",
			relPath, name)
	}
}
