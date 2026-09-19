package fonts

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// robotoCut is one Roboto face that exists TWICE in this repository — once
// embedded here and once committed on the designer side — and must be the
// same bytes in both places.
type robotoCut struct {
	// Key is the fonts.Shipped() key this cut is filed under.
	Key string
	// Dir and File locate the cut, relative to folio-go/fonts/ on this
	// side and folio-designer/public/fonts/ on the other. The two trees
	// use the SAME directory and file names on purpose: a mirror whose
	// two halves are addressed by one pair of strings cannot drift by
	// someone editing one path and not the other.
	Dir  string
	File string
	// MinPlausibleBytes is this cut's OWN non-vacuity floor, sitting
	// comfortably below its recorded size and far above anything a
	// truncated read or a broken embed would produce. It is per-cut
	// rather than one shared constant because the cuts differ by 22 KB
	// and a floor set by the smallest of them would stop discriminating
	// for the largest.
	MinPlausibleBytes int
	// RecordedBytes is the size measured at Story 11.1's plan gate,
	// quoted in the failure message so a reader sees how far short a
	// short read fell.
	RecordedBytes int
}

// robotoCuts is every Roboto face folio-go ships. Story 16.8 shipped the
// first; Story 11.1 added the other three.
//
// D-B EXTENDS THE ONE-CUT DISCIPLINE TO THEM RATHER THAN SCOPING IT TO
// REGULAR. D-A's consequences left 11.1 an explicit fork: carry
// TestShippedRobotoMatchesDesignerCatalogue's one-cut rule into the bold
// and italic cuts, or rule that the rule was only ever about Regular.
// The rule is carried. The hazard it names does not care about weight —
// two cuts of "Roboto Bold" under one name would make a bold run render
// differently depending on which build produced the document, in exactly
// the way two cuts of "Roboto" would, and the browser now gets its own
// copy of each of these files, so there are two halves to keep identical
// per cut instead of one.
var robotoCuts = []robotoCut{
	{
		Key: "Roboto", Dir: "roboto", File: "Roboto-Regular.ttf",
		MinPlausibleBytes: 300_000, RecordedBytes: 355_956,
	},
	{
		Key: "Roboto Bold", Dir: "roboto-bold", File: "Roboto-Bold.ttf",
		MinPlausibleBytes: 300_000, RecordedBytes: 358_188,
	},
	{
		Key: "Roboto Italic", Dir: "roboto-italic", File: "Roboto-Italic.ttf",
		MinPlausibleBytes: 320_000, RecordedBytes: 375_320,
	},
	{
		Key: "Roboto Bold Italic", Dir: "roboto-bolditalic", File: "Roboto-BoldItalic.ttf",
		MinPlausibleBytes: 320_000, RecordedBytes: 378_148,
	},
}

// TestShippedRobotoMatchesDesignerCatalogue is Story 16.8's machine-checked
// form of "THERE IS EXACTLY ONE ROBOTO" — its own Always clause, restated
// here as a run rather than a comment — GENERALISED AT STORY 11.1 from the
// single Regular pair it was written for to a table over all four Roboto
// cuts (D-B).
//
// WHY THIS TEST EXISTS, NAMED PLAINLY. Two cuts of Roboto already sat in
// this repository under one name, 180 KB apart, before this story: the
// designer's own catalogue face (`folio-designer/public/fonts/roboto/
// Roboto-Regular.ttf`, `font-catalogue.json`'s `"roboto"` entry, sha256
// e688a215…, 347.6 KiB) and a test fixture (`folio-go/testdata/fonts/
// Roboto-Regular.ttf`, sha256 79e85140…, 167.7 KiB) used only to exercise
// the Apache-2.0 branch of internal/fontset's licence-signature table.
// Shipping the wrong one would not fail a build — it would render,
// silently, and every document naming "Roboto" would disagree
// pixel-for-pixel depending on which binary produced it. That is worse
// than a build failure, so it is checked by a run rather than left to a
// comment ("a comment is not a measurement").
//
// WHAT "CATALOGUE" MEANS FOR THE THREE NEW CUTS, SINCE THE TEST NAME NOW
// OVERSTATES IT. Only the Regular has a `font-catalogue.json` entry, and
// the three new cuts deliberately have none: `font-catalogue.test.ts`
// asserts every catalogue face is upright Regular 400, so a bold cut
// cannot honestly be one and the catalogue stays Regular-only. What each
// new cut DOES have is a committed designer-side copy under the same
// directory name, delivered to the browser by an `@font-face` rule rather
// than by the catalogue. That copy is what this test compares against, and
// it is the half that would otherwise drift: the engine and the browser
// each hold their own bytes for every shipped face.
//
// WHY THE COMPARISON IS AGAINST THE DESIGNER'S FILE, READ FRESH, RATHER
// THAN A LITERAL DIGEST RESTATED HERE. A hardcoded sha256 on this side
// could be "kept honest" by editing one string the moment either file
// changed, which is exactly the laundering font-binary-identity.test.ts's
// own digest ties (on the designer side) are written to resist. Reading
// the designer's committed file at test time means a byte in either copy
// moving independently is what turns this test red — not a copy of a
// number moving in lockstep with it.
func TestShippedRobotoMatchesDesignerCatalogue(t *testing.T) {
	shipped := Shipped()

	// THE NEGATIVE CONTROL, READ ONCE AND RE-CHECKED AGAINST EVERY CUT.
	// folio-go/testdata/fonts/Roboto-Regular.ttf IS A DIFFERENT CUT and
	// must stay different — from all four faces, not only from the
	// Regular. This is not a defect to fix; it is a fixture for
	// internal/fontset's Apache-2.0 licence-signature test. But if it ever
	// became byte-identical to any shipped Roboto, the sentence above
	// stating they differ would be quietly false, so that premise is
	// checked too, once per cut.
	testFixturePath := filepath.Join("..", "testdata", "fonts", "Roboto-Regular.ttf")
	testFixtureBytes, err := os.ReadFile(testFixturePath)
	if err != nil {
		t.Fatalf("could not read the test-fixture Roboto at %s: %v", testFixturePath, err)
	}
	if len(testFixtureBytes) == 0 {
		t.Fatalf("the negative control at %s read as ZERO bytes — every 'these differ' check below would be vacuous", testFixturePath)
	}
	testFixtureDigest := sha256.Sum256(testFixtureBytes)

	checked := 0
	for _, cut := range robotoCuts {
		t.Run(cut.Key, func(t *testing.T) {
			shippedBytes, ok := shipped[cut.Key]
			if !ok {
				t.Fatalf(
					"fonts.Shipped() carries no %q key. Story 16.8 requires the engine to ship %q and "+
						"Story 11.1 requires the other three Roboto cuts alongside it; a cut named here and "+
						"absent there ships with no one-cut discipline at all.",
					cut.Key, "Roboto",
				)
			}

			// THE DESIGNER'S COMMITTED COPY, READ AS THE SOURCE OF TRUTH.
			designerPath := filepath.Join("..", "..", "folio-designer", "public", "fonts", cut.Dir, cut.File)
			designerBytes, err := os.ReadFile(designerPath)
			if err != nil {
				t.Fatalf("could not read the designer's %s at %s: %v", cut.Key, designerPath, err)
			}

			// NON-VACUITY BEFORE COMPARISON, on BOTH operands and with
			// this cut's own floor. Two failed or truncated reads
			// comparing near-empty to near-empty is how an identity check
			// reports IDENTICAL for files that were never really read.
			if len(shippedBytes) < cut.MinPlausibleBytes {
				t.Fatalf(
					"fonts.Shipped()[%q] is only %d bytes, short of this cut's %d-byte floor (recorded size "+
						"%d) — the embed is reading something other than the shipped face",
					cut.Key, len(shippedBytes), cut.MinPlausibleBytes, cut.RecordedBytes,
				)
			}
			if len(designerBytes) < cut.MinPlausibleBytes {
				t.Fatalf(
					"%s is only %d bytes, short of this cut's %d-byte floor (recorded size %d) — the "+
						"designer-side copy is truncated or is not the file this row names",
					designerPath, len(designerBytes), cut.MinPlausibleBytes, cut.RecordedBytes,
				)
			}

			shippedDigest := sha256.Sum256(shippedBytes)
			designerDigest := sha256.Sum256(designerBytes)
			if shippedDigest != designerDigest {
				t.Fatalf(
					"folio-go's embedded %q (sha256 %s, %d bytes) is NOT byte-identical to the designer's "+
						"copy at %s (sha256 %s, %d bytes). THERE IS EXACTLY ONE ROBOTO PER CUT: two cuts under "+
						"one name would make a document naming %q render differently depending on which build "+
						"produced it. If this fired, the embedded bytes were copied from the wrong source — "+
						"re-copy them byte-for-byte from the designer's committed file rather than "+
						"re-downloading or re-deriving them.",
					cut.Key, hex.EncodeToString(shippedDigest[:]), len(shippedBytes),
					designerPath, hex.EncodeToString(designerDigest[:]), len(designerBytes), cut.Key,
				)
			}

			// The negative control, re-checked for THIS cut.
			if testFixtureDigest == shippedDigest {
				t.Fatalf(
					"folio-go/testdata/fonts/Roboto-Regular.ttf (the Apache-2.0 licence-signature fixture) is "+
						"now byte-identical to the shipped %q (sha256 %s). This test's own header comment says "+
						"the two are different cuts by design — if that has changed, the comment above needs "+
						"correcting along with this assertion, not the other way round.",
					cut.Key, hex.EncodeToString(shippedDigest[:]),
				)
			}

			// THE WITNESS IS INCREMENTED HERE, INSIDE THE SUBTEST AND
			// AFTER THE LAST ASSERTION, NOT IN THE LOOP BODY. Outside the
			// closure it would count loop ITERATIONS, which the range
			// itself already guarantees — the mismatch below would then be
			// unreachable by construction and the witness would witness
			// nothing. Here it counts subtests that actually reached the
			// end of their assertions, so a cut that returns early or
			// t.Fatalf's is one the count is missing.
			checked++
		})
	}

	// Coverage witness (D-000.9): a run that examined nothing must not
	// report the same "no failures" a healthy run reports.
	if checked != len(robotoCuts) {
		t.Fatalf("coverage witness: checked %d of %d Roboto cuts", checked, len(robotoCuts))
	}
	if checked == 0 {
		t.Fatal("coverage witness: robotoCuts is EMPTY, so this test asserted nothing")
	}

	// WHAT THIS TEST DELIBERATELY DOES NOT DO, STATED RATHER THAN LEFT AS
	// AN ABSENCE. It does not scan fonts.Shipped() for keys that "look
	// like" Roboto and demand a row for each. Doing so would mean reading
	// a family out of a Shipped() key — a `strings.HasPrefix(key,
	// "Roboto")` — and D-B forecloses that shape outright, in tests as
	// well as in production: the keys are readable strings, not an
	// encoding, and the one place a family may be read from is the face's
	// own name table, sfnt name ID 1 — which shippedFaceSpecs.Family
	// asserts against the binary rather than being. The
	// residual gap is therefore real and small: a FIFTH Roboto cut added
	// to Shipped() and to no list here would ship without the one-cut
	// discipline. It would still fail shipped_faces_ext_test.go and
	// shipped_faces_test.go (both compare Shipped() against their tables
	// in both directions) and lint's fonts-asset rules, so it cannot ship
	// unnoticed — only unmirrored, and adding its row here is one line.
}
