package folio8_test

// D-2.6.4: pin fixtures/expected-breaks/expected_breaks.json in the
// ORDINARY suite, not only behind //go:build matrix.
//
// THE GAP THIS CLOSES, and it is not the feedback delay. D-2.4.3's Thai
// BREAK sign-off binds to this file's bytes: the record a reader writes
// names a sha256, and expected_breaks_signoff_matrix_test.go re-computes
// that sha256 and refuses a record that no longer matches. That binding
// is the whole mechanism by which "a human read these labels" stays true
// as the repository moves.
//
// But that test is matrix-tagged, so it compiles on no ordinary run. A
// file whose only digest binding lives behind a build tag can be EDITED
// BETWEEN GATES with the ordinary suite fully green — which means the
// SIGN-OFF'S SUBJECT can drift while the sign-off still reads as valid.
// The record answers "did someone read these labels"; it cannot answer
// "are these the labels they read" unless something checks continuously.
//
// MEASURED CORRECTION TO D-2.6.4'S OWN PREMISE (D-000.49 — a record that
// misstates a risk is a defect, in either direction). D-2.6.4 says the
// file's "only digest pin is matrix-tagged today". Measured at this
// story's baseline by grepping the digest across folio-go/, fixtures/
// and docs/: the literal a545e042…9324de occurs at ZERO sites. The
// matrix-tagged test does not pin the digest either — it COMPUTES the
// digest at run time and compares it to the sign-off record's field, and
// that record (fixtures/expected-breaks/break-signoff.json) does not
// exist. So today the file is pinned NOWHERE, and this is its FIRST
// digest pin rather than its second. The remedy D-2.6.4 ordered is
// unchanged; only its premise is sharpened.
//
// WHY THIS IS NOT FOLDED INTO goldenDigestRecord. That record is
// expected.pdf-shaped: every entry is hashed by reading
// fixtures/<dir>/expected.pdf, and its completeness half scans the
// declared scope for PDF digests. This artifact is not a golden PDF and
// has no expected.pdf, so an entry there would assert something about a
// file that does not exist.
//
// NOTHING IN THE REPOSITORY CAN WRITE THIS FILE. fixtures/expected-breaks/
// README.md states, and Story 2.6's creator re-verified, that there is
// deliberately no generator for this directory. This test is what keeps
// that true against a hand edit rather than against a generator.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// expectedBreaksDigest is the committed sha256 of
// fixtures/expected-breaks/expected_breaks.json, written out here as a
// SECOND LITERAL (D-2.3.4's mechanism, applied to a non-PDF oracle).
//
// The number is not read from the file it checks. That is the point: the
// cheapest response to a hash mismatch is to accept the new bytes, and
// accepting them is invisible in a diff when only one copy of the number
// exists. With a literal here, an edit to the break labels must ALSO
// edit this line — and that edit is exactly the conversation this pin is
// for, because the Thai break sign-off binds to the bytes this digest
// names.
const expectedBreaksDigest = "f0848d4e89b950aae8c8fc4e21af37ff56f602f42075786575b00a7531cfb19e"

// TestExpectedBreaksVectorIsPinnedInTheOrdinarySuite is D-2.6.4.
//
// PRESENCE PRECONDITIONS (D-000.21 sharpened). A digest comparison alone
// would catch a truncated file, but it would catch it as "the digest
// moved" — indistinguishable from a legitimate re-label. These assert
// that the artifact still CARRIES THE PROPERTY the sign-off is about — a
// populated set of break-labelled items — so a failure says which of the
// two happened.
func TestExpectedBreaksVectorIsPinnedInTheOrdinarySuite(t *testing.T) {
	path := filepath.Join(repoRootForByteNeutrality(t), "fixtures", "expected-breaks", "expected_breaks.json")

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("presence precondition: %s could not be read: %v — the break sign-off's subject is not there, and a digest claim about a missing file is vacuous", path, err)
	}
	if len(body) == 0 {
		t.Fatal("presence precondition: fixtures/expected-breaks/expected_breaks.json is empty")
	}

	var doc struct {
		Items []struct {
			ID             string   `json:"id"`
			Text           string   `json:"text"`
			Words          []string `json:"words"`
			ExpectedBreaks []int    `json:"expectedBreaks"`
		} `json:"items"`
	}
	if uerr := json.Unmarshal(body, &doc); uerr != nil {
		t.Fatalf("presence precondition: %s is not valid JSON: %v", path, uerr)
	}
	if len(doc.Items) == 0 {
		t.Fatal("presence precondition: the break vector declares ZERO items — \"its digest is unchanged\" would still hold of an emptied oracle, and the sign-off would be bound to nothing")
	}

	// The property the sign-off judges is the LABELLING. An item with no
	// `words` carries no judgment for a reader to have made.
	labelled := 0
	for _, it := range doc.Items {
		if len(it.Words) > 0 && strings.TrimSpace(it.Text) != "" {
			labelled++
		}
	}
	if labelled == 0 {
		t.Fatal("presence precondition: no item carries both a non-empty `text` and a non-empty `words` labelling — the break vector holds no judgment the sign-off could bind to")
	}
	t.Logf("subject: fixtures/expected-breaks/expected_breaks.json — %d items, %d carrying a break labelling", len(doc.Items), labelled)

	sum := sha256.Sum256(body)
	got := hex.EncodeToString(sum[:])
	if got != expectedBreaksDigest {
		t.Fatalf(
			"fixtures/expected-breaks/expected_breaks.json now hashes to\n  %s\nbut the committed digest is\n  %s\n\n"+
				"THE BREAK LABELS HAVE BEEN EDITED. D-2.4.3's Thai BREAK sign-off binds to these exact bytes, "+
				"and a review sheet may be with a reader right now — an edit invalidates their reading whether "+
				"or not the sign-off record has been written yet.\n\n"+
				"Do NOT simply paste the new digest in. If the labels are genuinely wrong, correct them, update "+
				"this literal, and RE-REQUEST the reading: the fixture is the oracle and the engine follows it, "+
				"never the reverse (D-000.26 refined — the sign-off binds to the break-opportunity vector, which "+
				"is this file).\n\n"+
				"If you did not mean to change this file, revert it: there is deliberately no generator for "+
				"fixtures/expected-breaks/, so nothing in the repository writes it and every change to it is a "+
				"hand edit.",
			got, expectedBreaksDigest)
	}
}
