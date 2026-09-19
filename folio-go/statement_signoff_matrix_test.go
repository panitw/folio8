//go:build matrix

// statement_signoff_matrix_test.go is Story 4.7's human semantic
// acceptance step, and it is a FAILING TEST rather than a note on
// purpose.
//
// WHY THIS IS THE ONLY NON-CIRCULAR CHECK ON THIS RECORDING, stated
// first because everything else in this file follows from it.
//
// The four statement goldens are compared, by every other guard this
// story ships, against files THIS STORY JUST PRODUCED. That comparison
// is circular on the first pass and proves nothing until the bytes move.
// Cross-target byte identity across darwin/arm64, linux/amd64,
// linux/arm64 and js/wasm proves DETERMINISM, not correctness — this
// project's own words, from the multi-page incident: "a deterministically
// wrong file is byte-identical to itself." Measured at this story's
// baseline, NO committed golden in this repository contained a table at
// all, so nothing recorded before today could tell a correct table from
// a broken one, and Story 4.6's unconditional-clip mutation reddened
// ZERO goldens.
//
// So the only thing standing between this commit and four permanently
// defended wrong documents is a person opening them and reading them.
// D-000.22, verbatim: "A hash tells you nobody moved the furniture. It
// does not tell you the furniture was ever in the right room."
//
// THE MECHANISM IS RATIFIED AND COPIED, NOT REDESIGNED. D-000.22 +
// D-000.44 + D-000.53 + D-2.3.5 / D-000.26 (refined) already specify it,
// and it ships twice already — shaped_signoff_matrix_test.go (Thai
// READING) and expected_breaks_signoff_matrix_test.go (Thai BREAKS).
// This is the third instance of the SAME form: a JSON record naming the
// reader, the date, what was examined and the digests signed; every
// field required, because an unattributed, undated or unspecific
// sign-off is indistinguishable from no sign-off at all; enforced by a
// test gated behind the `matrix` build tag, so the story commits green
// while THE GATE CANNOT PASS (D-2.3.5's first binding condition: "the
// pending obligation is a FAILING TEST, never a log entry").
//
// ONE RECORD OVER FOUR DIGESTS, AND INVALIDATED IN WHOLE. This is the
// one thing this instance does that its two predecessors do not, and it
// is the engineering lead's ruling on this story, on two grounds:
//
//   - D-000.41's dilution argument. Four records for ONE act of human
//     attention overstates the attention and trains the signer that
//     re-approval is routine.
//   - OVER-INVALIDATING IS SAFE; UNDER-INVALIDATING IS NOT. Four
//     separate records would let three attestations survive a change
//     that was systemic — and a font or layout change that moves one of
//     these documents is nearly always evidence about all four. A stale
//     attestation that still reads valid is precisely the artifact this
//     epic has spent itself hunting.
//
// So assertStatementSignOffMatchesFrozenHashes below is the four-digest
// sibling of shaped_signoff_matrix_test.go's
// assertSignOffMatchesFrozenHash, and it fails the WHOLE record if ANY
// ONE digest has moved, or if the record names a different SET of
// documents than the four registered here.
//
// THE BOUNDARY THIS DOES NOT CROSS, stated so it is not re-derived:
// D-000.22's MACHINE-CHECKABLE HALF IS NEVER DEFERRABLE. Page counts,
// the {page -> row indices} partition, header repetition, the footer
// aggregate's value and page, the params-supplied date, the frozen Thai
// break, GPOS inside a cell, the logo's single definition, structural
// validity and cross-target byte identity are ALL asserted at recording,
// in the ordinary suite, with no deferral available — see
// statement_semantics_test.go. Only "does this document READ correctly
// to a person" may be pending, because nothing substitutes for it.
//
// D-000.41's OWN CHECK, AND WHAT THE SECOND ATTESTATION IS FOR. One
// scheduled item is known to be capable of moving these bytes shortly
// after this sign-off is recorded: DW-13's adoption (font-stream
// compression), which is the owner's decision, batched at the Epic 4
// close. The sign-off is requested ANYWAY, and the reasoning is recorded
// here rather than left for the owner to discover:
//
//   - DW-13's adoption is a DECISION, not a scheduled event. Waiting on
//     it is waiting on a distant owner that may never arrive.
//   - Story 4.7 IS the C4 gate. Closing the gate with its own flagship
//     artifact unattested is the wrong shape: "frozen with sign-off
//     pending" is a licence for a TRANSIENT state, not for the gate's
//     own TERMINAL one.
//   - And the dilution worry INVERTS on inspection. If DW-13 is adopted,
//     the re-record is a deliberate byte-moving change that should NOT
//     alter appearance — so the second attestation is the check that
//     catches an unintended VISUAL side-effect of it. The second
//     signature answers a question the first cannot; the owner is not
//     being asked to sign twice for the same thing.
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

// TestStatementSemanticSignOffIsRecorded is the red gate. Its failure
// message is written for the person who has to resolve it, not for the
// person who wrote it (D-000.37).
func TestStatementSemanticSignOffIsRecorded(t *testing.T) {
	root := repoRootFromTest(t)
	path := filepath.Join(root, filepath.FromSlash(statementSignOffRelPath))

	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Fatalf(
			"%s does not exist, so Story 4.7's human semantic acceptance step is still OUTSTANDING and "+
				"the Epic 4 boundary gate cannot pass.\n\n"+
				"This is not a broken test. Story 4.7 froze four table goldens — the FIRST goldens in this "+
				"repository that contain a table at all — and every other check on them is circular: they "+
				"are compared against the files this story just produced, and identical bytes on four "+
				"targets prove the renderer is deterministic, not that the statement is right.\n\n"+
				"To resolve it, open these four files and READ them:\n%s\n"+
				"The 1-page and the 50-page documents in full; for the 5- and 20-page documents, the "+
				"PAGE-BOUNDARY pages specifically — the last page before each break and the first page "+
				"after — because that is where table pagination can be wrong in a way the interior pages "+
				"cannot show.\n\n"+
				"Confirm, and say so in \"examined\":\n"+
				"  - which documents you opened;\n"+
				"  - that the transaction rows carry the values the data document declares;\n"+
				"  - that the five column headers appear on continuation pages, not only page 1;\n"+
				"  - that the total at the foot is the total of the rows above it;\n"+
				"  - that the Thai reads correctly and its marks sit on the consonants they belong to;\n"+
				"  - that the Chinese is not the wrong weight or the wrong face;\n"+
				"  - that the logo is present and is a legible mark, not a black box;\n"+
				"  - that the page numbering and the supplied date (%s) are right.\n\n"+
				statementSignOffReadingRemedy+"\n\n"+
				"Then write %s as:\n"+
				"  {\n"+
				"    \"reader\":   \"<your name>\",\n"+
				"    \"date\":     \"<YYYY-MM-DD>\",\n"+
				"    \"examined\": \"<what you looked at and what you saw>\",\n"+
				"    \"digests\": {\n%s"+
				"    }\n"+
				"  }\n\n"+
				"...taking each digest from that fixture's expected.json, unchanged. Finally add a\n"+
				"  {kind: \"signoff\", relPath: %q}\n"+
				"site to ALL FOUR entries in goldenDigestRecord (byte_neutrality_test.go), the way\n"+
				"fixtures/shaped-text's site was added after its own sign-off landed — otherwise the\n"+
				"completeness half of TestGoldenDigestAgreesAtEveryDeclaredSite will report these digests\n"+
				"as occurring at an UNDECLARED site, which is that guard working correctly.\n\n"+
				"If any of it reads WRONG, the fixtures are re-recorded instead. That is the outcome this "+
				"gate exists to make possible, and it is far cheaper than the reverse.\n\n"+
				"NOTE ON RE-ATTESTATION, so it is not a surprise: if DW-13 (font-stream compression) is "+
				"adopted at the Epic 4 close, every golden in the repository moves and this record is "+
				"correctly invalidated. The re-read is not a repeat of this one — a compression change "+
				"should not alter APPEARANCE, so the second reading is the check that catches an "+
				"unintended visual side-effect of it.",
			statementSignOffRelPath,
			statementSignOffFileList(root),
			statementGeneratedDate,
			statementSignOffRelPath,
			statementSignOffDigestTemplate(t, root),
			statementSignOffRelPath,
		)
	}
	if err != nil {
		t.Fatalf("read %s: %v", statementSignOffRelPath, err)
	}

	var rec statementSignOff
	if uerr := json.Unmarshal(raw, &rec); uerr != nil {
		t.Fatalf("%s is not valid JSON: %v", statementSignOffRelPath, uerr)
	}

	// The field checks are statementSignOffFieldProblems' (untagged, and
	// red-proved there over synthetic records — this story's review,
	// Finding 3: as first written these lines sat BELOW the os.IsNotExist
	// branch above, so they had never executed on any input at all).
	if problems := statementSignOffFieldProblems(rec); len(problems) != 0 {
		t.Errorf(
			"%s is incomplete: %s. Every field is required: an unattributed, undated or unspecific "+
				"sign-off is indistinguishable from no sign-off at all, and a record naming no digests "+
				"certifies nothing and cannot go stale.",
			statementSignOffRelPath, strings.Join(problems, "; "),
		)
		return
	}

	assertStatementSignOffMatchesFrozenHashes(t, root, rec)
	t.Logf("statement semantic sign-off present: reader %q, date %s, %d digests bound", rec.Reader, rec.Date, len(rec.Digests))
}

// assertStatementSignOffMatchesFrozenHashes hashes the four committed
// artifacts and prints whatever statementSignOffGateVerdict returns.
//
// Both the COMPARISON (statementSignOffStaleness) and the WRAPPER's
// verdict-to-message step (statementSignOffGateVerdict) live in the
// untagged file beside their red-proofs (statement_golden_fixture_test.go).
// A staleness check — or a wrapper around one — that only ever runs
// behind the matrix tag has no way to be shown going red, and an inverted
// verdict would have left every test in the tree green (this story's
// review, Finding 3). What is left here is the part that genuinely needs
// the filesystem.
func assertStatementSignOffMatchesFrozenHashes(t *testing.T, root string, rec statementSignOff) {
	t.Helper()

	live := map[string]string{}
	var wantSlugs []string
	for _, f := range statementFixtures {
		wantSlugs = append(wantSlugs, f.slug)
		live[f.slug] = statementLiveDigest(t, root, f.slug)
	}

	if msg, failed := statementSignOffGateVerdict(rec, live, wantSlugs); failed {
		t.Fatal(msg)
	}
}

// statementLiveDigest hashes a fixture's committed expected.pdf AT CALL
// TIME. Read from the artifact, never from expected.json: a re-record
// that moved the PDF and its JSON together would leave a JSON-to-JSON
// comparison green.
func statementLiveDigest(t *testing.T, root, slug string) string {
	t.Helper()
	path := filepath.Join(root, "fixtures", slug, "expected.pdf")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("presence precondition: %s could not be read: %v", path, err)
	}
	if len(b) == 0 {
		t.Fatalf("presence precondition: %s is empty", path)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// statementSignOffFileList renders the four documents' paths for the
// failure message, so the reader can open them without deriving paths.
func statementSignOffFileList(root string) string {
	var b strings.Builder
	for _, f := range statementFixtures {
		b.WriteString("    fixtures/" + f.slug + "/expected.pdf   (" + itoaForTest(int64(f.pages)) + " page(s), " + itoaForTest(int64(f.rows)) + " transactions)\n")
	}
	return b.String()
}

// statementSignOffDigestTemplate renders the "digests" block, prefilled
// with the CURRENT digests, so the reader copies rather than transcribes.
func statementSignOffDigestTemplate(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	for i, f := range statementFixtures {
		comma := ","
		if i == len(statementFixtures)-1 {
			comma = ""
		}
		b.WriteString("      \"" + f.slug + "\": \"" + statementLiveDigest(t, root, f.slug) + "\"" + comma + "\n")
	}
	return b.String()
}
