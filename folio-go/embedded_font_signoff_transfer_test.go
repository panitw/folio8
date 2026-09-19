package folio8

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/template"
	"github.com/panitw/folio8/folio-go/internal/text"
)

// D-8.4.8's ASSERTION. fixtures/embedded-font/signoff.json is not a human
// reading: it TRANSFERS the owner's reading of fixtures/thai-stacked-marks/
// onto a second document, and the ground it transfers on is that the two
// documents' shaped runs are the same computation over the same bytes. That
// claim is a claim about the code, so it is asserted here rather than asserted
// in prose inside the record that depends on it.
//
// WHAT MAKES THIS TEST NON-VACUOUS, stated because on its own it does NOT
// look it. Both sides are LIVE: this is the both-sides-move-together shape,
// and a shaping regression that hit the embedded path and the FontSet path
// identically would leave it green. It is non-vacuous ONLY because the chain
// terminates in a FROZEN LITERAL somewhere else:
//
//	fixtures/thai-stacked-marks/expected.pdf is byte-pinned at
//	d5077f3346e10abb17ec69d2d6e2a975d02524d6e2eebcbec3b85ff30ca48eb1,
//	fixtures/thai-stacked-marks/signoff.json names that digest as the bytes
//	the owner actually read, and goldenDigestRecord re-hashes the artifact
//	on every ordinary run.
//
// So a regression that moved both live runs together would move the anchor's
// bytes too, redden the digest guard, invalidate the reading by construction,
// and LAPSE the transfer. That is the whole mechanism, and it is why
// embedded_font_signoff_matrix_test.go asserts the lapse condition explicitly
// rather than merely asserting the transferred record exists.
//
// PRE-SUBSET IS LOAD-BEARING, NOT AN IMPLEMENTATION DETAIL. text.ShapedGlyph's
// GlyphID is documented as "the glyph index in the SOURCE face's numbering —
// not a subset glyph id and not a PDF CID". Subsetting renumbers glyph ids
// from each document's own subset plan, and the two documents necessarily
// subset differently (different resource name, derived from the asset key —
// AD-7). A post-subset comparison would therefore be comparing two
// renumberings, which asserts nothing about the shaping the owner's eye
// judged. shapeSegments is the deepest layer that still speaks the source
// face's numbering, so the comparison is made THERE and nowhere downstream.

// fixtureShapedRun is one element's shaped run, pre-subset, flattened across
// the face segments shapeSegments produced. faceNames is kept alongside the
// glyphs because the two sides of the assertion below MUST differ in it: that
// difference is the entire point of the transfer.
type fixtureShapedRun struct {
	glyphs    []text.ShapedGlyph
	faceNames []string
	text      string
}

// shapedRunFromCommittedFixture shapes one element of a fixture's COMMITTED
// input.folio — read from disk, not from a Go constant mirroring it — because
// the sign-off record names fixture paths, and a constant is a second
// authority on what the fixture says.
func shapedRunFromCommittedFixture(t *testing.T, fixtureDir, elementID string) fixtureShapedRun {
	t.Helper()
	root := repoRootFromTest(t)
	path := filepath.Join(root, "fixtures", fixtureDir, "input.folio")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	tpl, err := ParseTemplate(source)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	var el template.Element
	found := false
	for _, candidate := range tpl.doc.Bands.Content.Elements {
		if string(candidate.ID) == elementID {
			el = candidate
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("presence precondition: fixtures/%s/input.folio has no content element %q — the transfer names it, so its absence makes every assertion below vacuous", fixtureDir, elementID)
	}

	chain, styled, err := fontChain(tpl, el)
	if err != nil {
		t.Fatalf("fixtures/%s element %s: font chain: %v", fixtureDir, elementID, err)
	}
	// newDocumentFontCache, NOT newFontCache: the embedded side resolves its
	// face out of the document, and a cache that could not see the document
	// would silently skip the carried entry and shape nothing.
	cache := newDocumentFontCache(tpl).forChain(el.Style.Value.FontFamily.Value)
	segs, diags, err := shapeSegments(elementID, chain, styled, el.Value.Value, testShippedFontSet(), cache, breaksAreConsumed)
	if err != nil {
		t.Fatalf("fixtures/%s element %s: shapeSegments: %v", fixtureDir, elementID, err)
	}
	if len(diags) != 0 {
		t.Fatalf("fixtures/%s element %s shaped with %d diagnostic(s), so the run is not a clean one to compare: %+v", fixtureDir, elementID, len(diags), diags)
	}

	run := fixtureShapedRun{text: el.Value.Value}
	for _, seg := range segs {
		for range seg.glyphs {
			run.faceNames = append(run.faceNames, seg.face)
		}
		run.glyphs = append(run.glyphs, seg.glyphs...)
	}
	return run
}

// TestEmbeddedFontShapedRunEqualsAttestedControl is the assertion
// fixtures/embedded-font/signoff.json names in its `transfer.asserted_by`
// field, and it is what that record's whole claim rests on.
//
// THE TWO SIDES DIFFER IN HOW THE FACE ARRIVES, AND IN NOTHING ELSE THAT
// SHAPES. fixtures/thai-stacked-marks/'s e2 gets Noto Sans Thai BY NAME
// THROUGH THE FontSet; fixtures/embedded-font/'s e1 gets the identical font
// program BY ASSET KEY OUT OF THE DOCUMENT, with no such face supplied in its
// FontSet at all. Everything a shaped run is a function of is the same: the
// same string, the same chain name "body", 12 pt, width 400, x 0. The two
// elements differ only in y and height, which is placement, not shaping.
//
// If those two paths ever produce different glyph ids or different offsets,
// the transferred reading is attesting a page nobody looked at and this test
// is where that is found.
func TestEmbeddedFontShapedRunEqualsAttestedControl(t *testing.T) {
	control := shapedRunFromCommittedFixture(t, "thai-stacked-marks", "e2")
	carried := shapedRunFromCommittedFixture(t, "embedded-font", "e1")

	// VACUITY GUARD. Two empty runs are equal, and so are two runs over
	// different strings that both failed to shape.
	if len(control.glyphs) == 0 {
		t.Fatal("vacuity guard: the attested control shaped to zero glyphs, so the equality below asserts nothing")
	}
	if control.text != carried.text {
		t.Fatalf("the two elements no longer draw the same string, so nothing about the shaped runs can transfer:\n\tcontrol (thai-stacked-marks e2): %q\n\tcarried (embedded-font e1):     %q", control.text, carried.text)
	}

	// THE DIFFERENCE THAT IS THE POINT. Asserted before the equality, because
	// an equality between two runs that both came through the FontSet by name
	// would be trivially true and would certify nothing about the embedded
	// path.
	fs := testShippedFontSet()
	for i, name := range control.faceNames {
		if _, supplied := fs[name]; !supplied {
			t.Fatalf("control glyph %d resolved to face %q, which the FontSet does not supply — the control side must arrive BY NAME THROUGH THE FontSet", i, name)
		}
	}
	wantCarriedFace := embeddedFaceName(embeddedFontAssetKey())
	for i, name := range carried.faceNames {
		if name != wantCarriedFace {
			t.Fatalf("carried glyph %d resolved to face %q, want %q — the carried side must arrive BY ASSET KEY OUT OF THE DOCUMENT", i, name, wantCarriedFace)
		}
		if _, supplied := fs[name]; supplied {
			t.Fatalf("the carried face name %q IS in the FontSet, so this side did not come out of the document after all", name)
		}
	}

	// THE EQUALITY ITSELF: glyph ids and x/y offsets, in the SOURCE face's
	// numbering.
	if len(carried.glyphs) != len(control.glyphs) {
		t.Fatalf("the carried face shaped %q into %d glyph(s); the attested control shaped it into %d. The transferred reading at fixtures/embedded-font/signoff.json claims these are the same computation, and they are not.", control.text, len(carried.glyphs), len(control.glyphs))
	}
	for i := range control.glyphs {
		want, got := control.glyphs[i], carried.glyphs[i]
		if got.GlyphID != want.GlyphID || got.XOffset != want.XOffset || got.YOffset != want.YOffset {
			t.Errorf(
				"glyph %d differs between the attested control and the carried face:\n"+
					"\tcontrol (thai-stacked-marks e2, by name through the FontSet): gid %d, offset (%d, %d)\n"+
					"\tcarried (embedded-font e1, by asset key out of the document): gid %d, offset (%d, %d)\n\n"+
					"D-8.4.8's transfer rests on these being identical. If this is a deliberate change, "+
					"fixtures/embedded-font/signoff.json no longer transfers anything and BOTH fixtures need "+
					"a real human reading — no agent may write one.",
				i, want.GlyphID, want.XOffset, want.YOffset, got.GlyphID, got.XOffset, got.YOffset)
		}
	}
	if t.Failed() {
		return
	}
	t.Logf("D-8.4.8: %d pre-subset glyphs identical in id and x/y offset across %q; the control resolved through the FontSet as %v, the carried run out of the document as %q",
		len(control.glyphs), control.text, control.faceNames[:1], wantCarriedFace)
}

// transferredReading is the schema of a TRANSFERRED reading record
// (D-8.4.8), and it is DELIBERATELY NOT thaiSignOff's.
//
// It has no `reader`, no `date` and no `examined`, because no human has
// looked at the file it covers. Reusing the human schema would have made
// the two indistinguishable to every reader and every guard, and a
// reconstructed record that does not say it is reconstructed is the
// failure this run has flagged before. The fields it DOES carry are what
// make it checkable: what kind of record it is, the plain statement that
// it is not a human reading, the digest it covers, and the anchor whose
// reading it borrows.
type transferredReading struct {
	Kind             string `json:"kind"`
	NotAHumanReading string `json:"NOT_A_HUMAN_READING"`
	SHA256           string `json:"sha256"`
	Transfer         struct {
		Basis         string `json:"basis"`
		FromFixture   string `json:"from_fixture"`
		FromElement   string `json:"from_element"`
		ToElement     string `json:"to_element"`
		AnchorSignoff string `json:"anchor_signoff"`
		AnchorSHA256  string `json:"anchor_sha256"`
		AssertedBy    string `json:"asserted_by"`
		GrantedBy     string `json:"granted_by"`
	} `json:"transfer"`
}

// transferredReadingKind is the value that marks a record as transferred
// rather than read. It is spelled once, here, because the matrix gate and
// this file's ordinary-suite check must not drift on it.
const transferredReadingKind = "transferred-reading"

// assertTransferredReadingIsRealAndStillBinding is the checker
// embedded_font_signoff_matrix_test.go's gate and this file's
// ordinary-suite guard SHARE, defined here rather than in the
// matrix-tagged file so there is exactly one of it: an untagged file is
// compiled into both binaries, a tagged one into only the matrix binary.
//
// It asserts four things, and the fourth is the one most likely to be
// dropped:
//
//  1. PRESENCE and shape — the file exists, parses, and says what kind of
//     record it is.
//  2. IT DOES NOT IMPERSONATE A HUMAN READING — `kind` is
//     "transferred-reading", the NOT_A_HUMAN_READING statement is
//     present, and `reader`, `date` and `examined` are ABSENT. Their
//     absence is checked on the RAW JSON, not on the struct above, which
//     would silently ignore them: an agent adding any of the three would
//     be fabricating an attestation, and this is where that is caught.
//  3. STILL BINDING — the digest it covers equals wantCovered, recomputed
//     from the live fixture.
//  4. THE TRANSFER HAS NOT LAPSED — `transfer.anchor_sha256` equals
//     wantAnchor, recomputed from the LIVE anchor fixture, and the anchor
//     sign-off it names still binds to that same value. The transfer is a
//     borrowed reading: it is worth exactly as much as the anchor's
//     reading of the anchor's bytes, so if those bytes move the record
//     covers nothing and BOTH fixtures need a real human reading.
func assertTransferredReadingIsRealAndStillBinding(t *testing.T, root, relPath, wantCovered, wantAnchor string) {
	t.Helper()
	path := filepath.Join(root, relPath)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("read %s: %v", relPath, err)
		return
	}
	var rec transferredReading
	if uerr := json.Unmarshal(raw, &rec); uerr != nil {
		t.Errorf("%s is not valid JSON: %v", relPath, uerr)
		return
	}

	// (1) and (2).
	if rec.Kind != transferredReadingKind {
		t.Errorf("%s has kind %q, want %q. This record must be unmistakable as a transferred reading rather than a human one.", relPath, rec.Kind, transferredReadingKind)
	}
	if strings.TrimSpace(rec.NotAHumanReading) == "" {
		t.Errorf("%s carries no NOT_A_HUMAN_READING statement. A record that does not say nobody looked at the file is indistinguishable from one that claims somebody did.", relPath)
	}
	var loose map[string]any
	if uerr := json.Unmarshal(raw, &loose); uerr != nil {
		t.Errorf("%s is not a JSON object: %v", relPath, uerr)
		return
	}
	for _, forbidden := range []string{"reader", "date", "examined"} {
		if _, present := loose[forbidden]; present {
			t.Errorf("%s carries a %q field. A TRANSFERRED reading must not claim one: no human has looked at the file it covers, and NO AGENT MAY WRITE reader, date or examined in any sign-off record. Remove the field — do not fill it in.", relPath, forbidden)
		}
	}

	// (3) STILL BINDING to the bytes it covers.
	if rec.SHA256 != wantCovered {
		t.Errorf(
			"%s covers digest %s, but fixtures/embedded-font now hashes to %s.\n\n"+
				"The record is STALE: it applies to bytes that no longer exist. Re-recording the covered "+
				"fixture does not invalidate the ANCHOR's reading, so the transfer may still be available — "+
				"but this record must be rewritten deliberately, and the shaped-run equality re-established, "+
				"not have a new digest pasted into it.",
			relPath, rec.SHA256, wantCovered)
	}

	// (4) THE LAPSE CONDITION.
	if rec.Transfer.AnchorSHA256 != wantAnchor {
		t.Errorf(
			"%s anchors its transfer on digest %s, but %s/expected.pdf now hashes to %s.\n\n"+
				"THE TRANSFER HAS LAPSED (D-8.4.8). This record borrows a human reading of the anchor's "+
				"bytes; those bytes moved, so the reading covers a page nobody looked at and it transfers "+
				"NOTHING. Both fixtures now need a real human reading. Do NOT update this digest to make "+
				"the test green — that would re-assert an attestation over unread bytes, which is the "+
				"fabrication D-000.28 and D-2.3.5 both forbid.",
			relPath, rec.Transfer.AnchorSHA256, rec.Transfer.FromFixture, wantAnchor)
		return
	}
	// AND THE ANCHOR'S OWN READING MUST STILL BIND TO IT. A transfer whose
	// anchor sign-off has itself gone stale is a chain that terminates in
	// nothing, which is the one thing this record cannot survive.
	anchorPath := filepath.Join(root, filepath.FromSlash(rec.Transfer.AnchorSignoff))
	anchorRaw, aerr := os.ReadFile(anchorPath)
	if aerr != nil {
		t.Errorf("%s names %s as its anchor sign-off, and that file could not be read: %v. The transfer has nothing to terminate in.", relPath, rec.Transfer.AnchorSignoff, aerr)
		return
	}
	// The anchor's schema is read inline rather than through thaiSignOff,
	// which lives in a //go:build matrix file and so is not compiled into
	// the ordinary test binary this checker must also serve.
	var anchor struct {
		Reader   string `json:"reader"`
		Date     string `json:"date"`
		Examined string `json:"examined"`
		SHA256   string `json:"sha256"`
	}
	if uerr := json.Unmarshal(anchorRaw, &anchor); uerr != nil {
		t.Errorf("%s is not valid JSON: %v", rec.Transfer.AnchorSignoff, uerr)
		return
	}
	if anchor.SHA256 != wantAnchor {
		t.Errorf("%s names digest %s, but %s transfers from it on %s. The anchor's own reading is stale, so there is nothing to borrow.", rec.Transfer.AnchorSignoff, anchor.SHA256, relPath, wantAnchor)
	}
	if strings.TrimSpace(anchor.Reader) == "" || strings.TrimSpace(anchor.Examined) == "" {
		t.Errorf("%s is the human reading %s borrows, and it names no reader or nothing examined. A transfer from an empty attestation transfers nothing.", rec.Transfer.AnchorSignoff, relPath)
	}
}

// TestEmbeddedFontTransferredReadingDoesNotImpersonateAHumanReading runs the
// transferred record's schema, impersonation and LAPSE checks in the ORDINARY
// suite.
//
// embedded_font_signoff_matrix_test.go is the gate — it is what an epic
// boundary cannot pass with red — but a record going stale between matrix runs
// would otherwise be invisible until the next gate run, and the highest-stakes
// failure here (an agent filling in `reader`, `date` or `examined`) is a source
// edit that needs no re-record at all to happen. So it is caught continuously,
// the same move TestExpectedBreaksVectorIsPinnedInTheOrdinarySuite makes for
// expected_breaks.json (D-2.6.4).
func TestEmbeddedFontTransferredReadingDoesNotImpersonateAHumanReading(t *testing.T) {
	root := repoRootFromTest(t)
	assertTransferredReadingIsRealAndStillBinding(t, root,
		filepath.Join("fixtures", "embedded-font", "signoff.json"),
		fixturePDFDigest(t, root, "embedded-font"),
		fixturePDFDigest(t, root, "thai-stacked-marks"),
	)
}

// fixturePDFDigest re-hashes a fixture's committed expected.pdf. It reads the
// ARTIFACT and never expected.json, because a re-record moves both together
// and the digest of the bytes is the only thing an attestation can bind to.
func fixturePDFDigest(t *testing.T, root, dir string) string {
	t.Helper()
	path := filepath.Join(root, "fixtures", dir, "expected.pdf")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(body) == 0 {
		t.Fatalf("presence precondition: %s is empty — two empty files are byte-identical", path)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
