package fontset_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/fontset"
)

// THE RED-PROOF THIS PACKAGE'S OWN DOC COMMENT SAID DID NOT EXIST.
//
// ShapedGlyph's comment described YOffset as "zero for every glyph of
// every sample across all three shipped faces today", and called the
// guard over it "a FORWARD GUARD WITH NO AVAILABLE RED-PROOF, never a
// red-proved one". Both halves were false, and this file is the proof.
//
// The claim traces to Story 2.3, which measured ITS OWN SAMPLES and
// reported on THE SHIPPED SET — two different populations. It then
// propagated into four more places and stood for months, and it is why
// nobody built this test: a comment asserting a negative is what a
// reader checks before they go looking, so a wrong one protects itself.
// The owner found the consequence in production instead (DW-28).
//
// The general rule, from that episode: A COMMENT THAT ASSERTS A NEGATIVE
// — unreachable, never, impossible — CARRIES THE SAME EVIDENTIARY BURDEN
// AS A TEST, AND MUST NAME THE POPULATION IT MEASURED RATHER THAN THE
// POPULATION IT CONCLUDED ABOUT.
//
// IT LIVES IN fontset, NOT IN text, AND THE REASON IS A RULE THIS FILE
// BROKE ON ITS FIRST ATTEMPT. D-000.16 ranks the pipeline strictly
// forward — geom 0, diag 1, pagemodel 1, template 2, expr 3, bind 4,
// text 5, fontset 6, layout 7, pdf 8 — and a package may import only a
// STRICTLY LOWER rank. Written under internal/text this file imported
// fontset, rank 6 from rank 5, and `lint`'s stage-rank scan caught it.
// fontset already imports text and constructs the Shaper, so the test
// belongs on this side of the boundary; the claim it corrects still
// lives in text/shape.go's comment, which points here.
//
// It is an EXTERNAL test package reading the face from its own file
// rather than importing the fonts package, because AD-8 keeps font data
// outside internal/ and fonts imports the root package.

// shippedThai loads the shipped Noto Sans Thai from its own file. If the
// path moves this test fails loudly rather than skipping — a guard that
// silently opts out of running is the failure mode this whole file is
// about.
func shippedThai(t *testing.T) *fontset.Font {
	t.Helper()
	path := filepath.Join("..", "..", "fonts", "notosansthai", "NotoSansThai-Regular.ttf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the shipped Thai face must be readable at %s: %v", path, err)
	}
	f, err := fontset.New("Noto Sans Thai", data)
	if err != nil {
		t.Fatalf("fontset.New: %v", err)
	}
	return f
}

// TestShippedThaiGivesAMarkANonZeroYOffset falsifies the claim directly,
// on the shipped face, through the real shaper.
//
// ทั้ is the smallest sequence that reaches it: ท with ั (sara a) and ้
// (mai tho) both above it. The face resolves this pair by a GPOS
// y-displacement, so one glyph comes back displaced.
func TestShippedThaiGivesAMarkANonZeroYOffset(t *testing.T) {
	glyphs, err := shippedThai(t).Shaper().Shape("ทั้")
	if err != nil {
		t.Fatalf("Shape: %v", err)
	}
	if len(glyphs) == 0 {
		t.Fatal("shaping produced no glyphs, so this test witnesses nothing")
	}
	var displaced int
	for _, g := range glyphs {
		if g.YOffset != 0 {
			displaced++
		}
	}
	if displaced == 0 {
		t.Fatalf("no glyph of ทั้ carries a non-zero YOffset across %d glyphs — "+
			"either the shipped face changed or the shaper stopped applying GPOS "+
			"mark positioning; both are the kind of silent change this test exists to catch", len(glyphs))
	}
}

// TestShippedThaiLeavesASubstitutedStackAtZeroOffset is the control, and
// it is the one that keeps the test above honest about WHAT it proves.
//
// ที่ also stacks two marks over one base, and it comes back at zero
// offset — the face resolves that pair by a GSUB lowered-form
// substitution instead. So the trigger is a non-zero YOffset, NOT mark
// stacking. Story 8.0's epic text originally said "two stacked marks"
// and was corrected at its plan gate for exactly this reason: ที่
// already appears in five shipped fixtures, all of which render.
func TestShippedThaiLeavesASubstitutedStackAtZeroOffset(t *testing.T) {
	glyphs, err := shippedThai(t).Shaper().Shape("ที่")
	if err != nil {
		t.Fatalf("Shape: %v", err)
	}
	if len(glyphs) == 0 {
		t.Fatal("shaping produced no glyphs, so this control witnesses nothing")
	}
	for i, g := range glyphs {
		if g.YOffset != 0 {
			t.Fatalf("glyph %d of ที่ carries YOffset %d, want 0: this pair is resolved by "+
				"substitution, and if it has started carrying a displacement then the "+
				"shipped fixtures that contain ที่ are about to move", i, g.YOffset)
		}
	}
}
