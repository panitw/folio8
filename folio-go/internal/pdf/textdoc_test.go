package pdf

import "github.com/panitw/folio8/folio-go/internal/pagemodel"

import (
	"bytes"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// fakeFace is a minimal, synthetic EmbeddedFace for exercising
// SerializeTextDocument's structure without needing a real subset —
// internal/fontset's own tests (fontset_test.go) already exercise real
// subsetting; this file's job is the PDF assembly layer alone.
func fakeFace(name string) EmbeddedFace {
	return EmbeddedFace{
		Name:          name,
		Program:       []byte("FAKE-TRUETYPE-PROGRAM-BYTES-" + name),
		Tag:           "ABCDEF",
		NumGlyphs:     3,
		WidthForGlyph: map[uint16]int64{0: 0, 1: 500, 2: 600},
		ToUnicode: []CIDText{
			{CID: 1, Text: "A"},
			{CID: 2, Text: "B"},
		},
	}
}

func onePageWithText(face string) pagemodel.Page {
	return pagemodel.Page{
		Runs: []pagemodel.TextRun{
			{Face: face, SourceText: "AB", X: 0, Y: 0, FontSize: 12000, Glyphs: []pagemodel.ShapedGlyph{
				{CID: 1, XAdvance: 500},
				{CID: 2, XAdvance: 600},
			}},
		},
		Width:      595276,
		Height:     841890,
		MarginTop:  36000,
		MarginLeft: 36000,
	}
}

// TestSerializeTextDocumentEmbedsOneFontFileForTwoPages is AC9,
// asserted BEHAVIOURALLY: "a two-page document using one face embeds
// exactly one FontFile2." internal/pdf has no notion of "the same
// template rendered across multiple pages" (no internal/layout exists
// until Story 2.5, so real pagination is out of this story's scope) —
// this test exercises the mechanism this property actually depends on:
// SerializeTextDocument receives the resolved face set ONCE and embeds
// each entry exactly once regardless of how many pagemodel.Pages reference
// it, which is the "never per page" half of AC9. The "one subsetting
// call" half — that internal/fontset.Font.Subset is called exactly
// once per face, over the document-wide union of runes, not per
// element — is asserted behaviourally at the folio8 package level by
// TestRenderSubsetsOneFacePerDocumentNotPerElement (render_test.go),
// which renders a real two-element, two-band document through the
// public Render path (Finding 6, QA review: the previous version of
// this comment claimed that half was "asserted via code review", which
// is not an assertion; it never reached renderDocument at all — the
// two-PAGE half genuinely remains out of reach until Story 2.5's
// pagination exists, and stays deferred here on that basis, honestly).
func TestSerializeTextDocumentEmbedsOneFontFileForTwoPages(t *testing.T) {
	faces := map[string]EmbeddedFace{"Body": fakeFace("Body")}
	pages := []pagemodel.Page{onePageWithText("Body"), onePageWithText("Body")}

	out, err := SerializeTextDocument(pages, faces, nil, nil)
	if err != nil {
		t.Fatalf("SerializeTextDocument: %v", err)
	}

	count := bytes.Count(out, []byte("/FontFile2 "))
	if count != 1 {
		t.Fatalf("two-page document using one face embeds %d FontFile2 references, want exactly 1 (AC9)", count)
	}
	if bytes.Count(out, []byte("/Type /Page")) < 2 {
		t.Fatalf("expected at least two /Type /Page objects (a two-page document), got fewer — test setup is broken")
	}
	if !bytes.Contains(out, []byte(fakeFace("Body").Program)) {
		t.Fatal("output does not contain the face's program bytes at all")
	}
	// The program bytes must appear EXACTLY ONCE — embedding it per page
	// would duplicate the (potentially large) font program, defeating
	// AC9's whole point.
	if got := bytes.Count(out, []byte(fakeFace("Body").Program)); got != 1 {
		t.Fatalf("face program bytes appear %d times, want exactly 1 (AC9: never per page)", got)
	}
}

// TestSerializeTextDocumentMissingFaceIsLocatedError exercises the
// content-stream builder's own guard: a run naming a face absent from
// the face set is a located error, never a silently-emitted .notdef
// stream.
func TestSerializeTextDocumentMissingFaceIsLocatedError(t *testing.T) {
	pages := []pagemodel.Page{onePageWithText("DoesNotExist")}
	_, err := SerializeTextDocument(pages, map[string]EmbeddedFace{}, nil, nil)
	if err == nil {
		t.Fatal("expected an error for a run naming a face absent from the face set")
	}
}

// TestSerializeTextDocumentUnknownCIDIsLocatedError exercises the
// per-glyph guard in appendShapedRun: a CID with no width in the
// embedded face is a located error, never a silently-emitted .notdef.
// Story 2.3 re-keyed this from runes onto CIDs, because there is no
// rune -> CID route into a content stream any more (AC4).
func TestSerializeTextDocumentUnknownCIDIsLocatedError(t *testing.T) {
	face := fakeFace("Body")
	// CID 9 is beyond fakeFace's base block and its (empty) extras.
	pages := []pagemodel.Page{{
		Runs: []pagemodel.TextRun{{Face: "Body", SourceText: "Z", X: 0, Y: 0, FontSize: 12000, Glyphs: []pagemodel.ShapedGlyph{
			{CID: 9, XAdvance: 500},
		}}},
		Width:  595276,
		Height: 841890,
	}}
	_, err := SerializeTextDocument(pages, map[string]EmbeddedFace{"Body": face}, nil, nil)
	if err == nil {
		t.Fatal("expected an error for a CID with no width in the embedded face")
	}
}

// TestShapedRunExpressesAYOffsetAsATextRise is Story 8.0's re-pointing
// of TestShapedRunFailsClosedOnYOffset (D-7.8.7: re-point, never
// delete). The synthetic run and its non-vacuity leg are the same two
// runs; what they assert is turned over.
//
// The comment that stood here said: "Measured across all three shipped
// faces at Story 2.3: YOffset is 0 for every glyph of every sample, so
// no production input triggers this and the branch has NO available
// red-proof through a rendered document." That was false. Story 2.3
// measured its own samples and reported on the shipped set — two
// different populations — and ordinary Thai whose marks the shaper
// displaces vertically reaches the emitter through the public entry
// point on the shipped Noto Sans Thai (thai_mark_stacking_test.go,
// fixtures/thai-stacked-marks/, DW-28).
//
// The synthetic run keeps its own value: it exercises the rise without
// depending on a face, so it survives any change to the shipped set.
func TestShapedRunExpressesAYOffsetAsATextRise(t *testing.T) {
	face := fakeFace("Body")
	run := pagemodel.TextRun{Face: "Body", SourceText: "A", FontSize: 12000, Glyphs: []pagemodel.ShapedGlyph{
		{CID: 1, XAdvance: 500, YOffset: 37},
	}}
	got, err := appendShapedRun(nil, run, face)
	if err != nil {
		t.Fatalf("a glyph carrying a non-zero YOffset must be EXPRESSED, not refused: %v", err)
	}
	// ScaleRound(12000, 37, 1000) = 444 millipoints. The sign passes
	// through untouched: YOffset is y-up and so is Ts.
	if string(got) != "0.444 Ts\n<0001> Tj\n0 Ts\n" {
		t.Fatalf("want the rise set, the glyph shown, and the rise given back; got %q", got)
	}

	// ...and the same run with YOffset zeroed must emit with NO rise at
	// all, so the test above cannot pass for an unrelated reason.
	run.Glyphs[0].YOffset = 0
	plain, err := appendShapedRun(nil, run, face)
	if err != nil {
		t.Fatalf("the same run with YOffset 0 must emit cleanly, got: %v", err)
	}
	if string(plain) != "<0001> Tj\n" {
		t.Fatalf("a zero-offset run must emit today's exact bytes with no Ts operator, got %q", plain)
	}
}

// TestShapedRunFailsClosedOnARiseThatRoundsAway is the fail-closed
// branch, NARROWED rather than deleted.
//
// With Ts in place every ShapedGlyph field is expressible, so the only
// condition left is the rounding boundary: an offset the shaper really
// asked for whose rise scales to zero. Emitting "0 Ts" there would drop
// the offset silently — the healthy output and the broken output would
// be the same bytes.
//
// It is reachable through a real document, not only here: fontSize has
// no positivity floor at parse, and a stacked-mark document at
// fontSize 0.008 reaches this branch on the shipped face
// (thai_mark_stacking_test.go's
// TestAStackedMarkWhoseRiseRoundsAwayIsStillRefused).
func TestShapedRunFailsClosedOnARiseThatRoundsAway(t *testing.T) {
	face := fakeFace("Body")
	// ScaleRound(8, -57, 1000) = 0 — the offset is real and the rise is
	// not.
	run := pagemodel.TextRun{Face: "Body", SourceText: "A", FontSize: 8, Glyphs: []pagemodel.ShapedGlyph{
		{CID: 1, XAdvance: 500, YOffset: -57},
	}}
	got, err := appendShapedRun(nil, run, face)
	if err == nil {
		t.Fatalf("an offset whose rise rounds to zero must be a located error, not a silently dropped offset; got %q", got)
	}
	if got != nil {
		t.Fatalf("a refusal must return no bytes, got %q", got)
	}

	// NON-VACUITY: the same offset at an ordinary font size must emit.
	// Without this leg the assertion above would pass for a build that
	// refused every offset again.
	run.FontSize = 12000
	if _, err := appendShapedRun(nil, run, face); err != nil {
		t.Fatalf("the same offset at 12 pt must emit cleanly (ScaleRound(12000, -57, 1000) = -684), got: %v", err)
	}
}

// TestShapedRunRoundsTheRiseHalfToEven is the assertion that makes this
// story's four-target matrix obligation mean something.
//
// EVERY rise the shipped documents produce divides exactly by 1000
// (12000x-57, 12000x-2, 12000x-59, 12000x37, 8x-57), so on all of them
// geom.ScaleRound and a plain truncating `int64(FontSize)*YOffset/1000`
// return the identical value — measured: swapping ScaleRound for
// truncation produced ZERO test failures across the whole suite. The
// round-half-to-even rule is the STATED justification for registering
// this document on all four targets, and until this test nothing in the
// tree observed it.
//
// Both cases below sit exactly on a tie, which is the only place the two
// rules can disagree:
//
//	11500 x -57 = -655_500;  q = -655, |r| = 500 = |den|/2, q odd  -> -656
//	                         truncation would give                    -655
//	10500 x -59 = -619_500;  q = -619, |r| = 500 = |den|/2, q odd  -> -620
//	                         truncation would give                    -619
//
// Asserted on the EXACT BYTES rather than on the Length, because the
// operand is what crosses to another target.
func TestShapedRunRoundsTheRiseHalfToEven(t *testing.T) {
	face := fakeFace("Body")
	cases := []struct {
		name     string
		fontSize int64
		yOffset  int64
		want     string
		truncate string // what the discarded truncating rule would have emitted
	}{
		{"a tie rounding away from zero to an even quotient", 11500, -57, "-0.656 Ts\n<0001> Tj\n0 Ts\n", "-0.655"},
		{"a second, independent tie", 10500, -59, "-0.62 Ts\n<0001> Tj\n0 Ts\n", "-0.619"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := pagemodel.TextRun{Face: "Body", SourceText: "A", FontSize: geom.Length(tc.fontSize), Glyphs: []pagemodel.ShapedGlyph{
				{CID: 1, XAdvance: 500, YOffset: tc.yOffset},
			}}
			got, err := appendShapedRun(nil, run, face)
			if err != nil {
				t.Fatalf("appendShapedRun: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("rise rounding:\n got  %q\n want %q", got, tc.want)
			}
			// NON-VACUITY, stated rather than implied: this test is only
			// worth having because the discarded rule produces a
			// DIFFERENT operand, and that operand must be absent.
			if bytes.Contains(got, []byte(tc.truncate)) {
				t.Fatalf("the emitted rise %q carries the truncating rule's operand %q — the rise must be rounded half to even (AD-1/AD-23), which is what makes all four AD-21 targets agree on it", got, tc.truncate)
			}
		})
	}
}

// TestZeroOffsetRunIsByteIdenticalToItsRisenTwin is Story 8.0's whole
// byte-identity claim, asserted rather than assumed — and it is
// TestColouredRunBracketsItsInk's fourth move, transposed onto the rise:
// build the stream with the condition ABSENT, flip one field on the SAME
// run, then delete the new bytes from the positive output and assert
// byte-equality with the negative output.
//
// This is what says the 21 goldens committed before this story cannot
// move. The claim is not "the Ts path is small"; it is that a run whose
// every glyph carries zero offset takes no part of it.
func TestZeroOffsetRunIsByteIdenticalToItsRisenTwin(t *testing.T) {
	faces := map[string]EmbeddedFace{"Body": fakeFace("Body")}
	page := func(run pagemodel.TextRun) pagemodel.Page {
		return pagemodel.Page{Width: 600000, Height: 800000, Runs: []pagemodel.TextRun{run}}
	}
	run := pagemodel.TextRun{Face: "Body", SourceText: "AB", FontSize: 12000, X: 1000, Y: 2000, Glyphs: []pagemodel.ShapedGlyph{
		{CID: 1, XAdvance: 500},
		{CID: 2, XAdvance: 600},
	}}

	plain, err := buildTextContentStream(page(run), faces)
	if err != nil {
		t.Fatalf("buildTextContentStream: %v", err)
	}
	if bytes.Contains(plain, []byte(" Ts")) {
		t.Errorf("a zero-offset run emitted a text-rise operator: %q", plain)
	}

	// ONE FIELD FLIPS, on both glyphs, so the run is a single segment.
	run.Glyphs[0].YOffset, run.Glyphs[1].YOffset = 37, 37
	risen, err := buildTextContentStream(page(run), faces)
	if err != nil {
		t.Fatalf("buildTextContentStream (risen): %v", err)
	}
	if !bytes.Contains(risen, []byte("0.444 Ts\n")) {
		t.Errorf("a risen run must set the rise, got %q", risen)
	}
	if !bytes.Contains(risen, []byte("0 Ts\nET\n")) {
		t.Errorf("a risen run must give the rise back before its ET — rise survives ET, and an unclipped uncoloured run has no q/Q bracket to restore it — got %q", risen)
	}

	// THE MOVE THAT MATTERS. Delete exactly the bytes the rise added and
	// what is left must be the zero-offset stream, byte for byte.
	stripped := bytes.ReplaceAll(risen, []byte("0.444 Ts\n"), nil)
	stripped = bytes.ReplaceAll(stripped, []byte("0 Ts\n"), nil)
	if !bytes.Equal(stripped, plain) {
		t.Errorf("giving a run a vertical offset changed more than its rise:\n risen-minus-rise %q\n plain            %q", stripped, plain)
	}
}

// TestShapedRunSplitsOnEveryChangeOfRise pins the run-splitting rule on
// a sequence no shipped document happens to produce: offsets 0, -57,
// -57, 0. Three segments in index order, the two middle glyphs sharing
// ONE Ts, and the rise given back before the run ends.
//
// The adjustment term that sits before a glyph leads the segment that
// glyph opens; Ts moves no pen, so a term crossing a segment boundary
// changes nothing about where the glyphs land.
func TestShapedRunSplitsOnEveryChangeOfRise(t *testing.T) {
	face := fakeFace("Body")
	run := pagemodel.TextRun{Face: "Body", SourceText: "AAAA", FontSize: 12000, Glyphs: []pagemodel.ShapedGlyph{
		{CID: 1, XAdvance: 500},
		{CID: 1, XAdvance: 500, YOffset: -57},
		{CID: 1, XAdvance: 500, YOffset: -57},
		{CID: 1, XAdvance: 500},
	}}
	got, err := appendShapedRun(nil, run, face)
	if err != nil {
		t.Fatalf("appendShapedRun: %v", err)
	}
	const want = "<0001> Tj\n-0.684 Ts\n<00010001> Tj\n0 Ts\n<0001> Tj\n"
	if string(got) != want {
		t.Fatalf("run splitting:\n got  %q\n want %q", got, want)
	}
}

// TestShapedRunMovesStraightFromOneRiseToTheNext covers the transition
// the sequence above cannot: rises[start] != current with BOTH of them
// non-zero.
//
// fixtures/thai-stacked-marks/ passes through a restoring `0 Ts` between
// its -0.684 and -0.708 segments, because a zero-offset glyph happens to
// sit between them, and TestShapedRunSplitsOnEveryChangeOfRise's
// 0,-57,-57,0 sequence never leaves zero either. So without this case the
// emitter could be restoring to 0 before EVERY rise change — emitting a
// byte nothing asked for, on a path no golden covers — and every other
// assertion in the tree would still pass.
//
// Ts SETS the rise, it does not accumulate one, so moving straight from
// -0.684 to -0.708 is correct and the intervening restore would be
// noise.
func TestShapedRunMovesStraightFromOneRiseToTheNext(t *testing.T) {
	face := fakeFace("Body")
	run := pagemodel.TextRun{Face: "Body", SourceText: "AA", FontSize: 12000, Glyphs: []pagemodel.ShapedGlyph{
		{CID: 1, XAdvance: 500, YOffset: -57},
		{CID: 1, XAdvance: 500, YOffset: -59},
	}}
	got, err := appendShapedRun(nil, run, face)
	if err != nil {
		t.Fatalf("appendShapedRun: %v", err)
	}
	const want = "-0.684 Ts\n<0001> Tj\n-0.708 Ts\n<0001> Tj\n0 Ts\n"
	if string(got) != want {
		t.Fatalf("a non-zero to non-zero rise change:\n got  %q\n want %q", got, want)
	}
	// Said explicitly, because it is the property this case exists for:
	// exactly ONE restore, at the end.
	if n := bytes.Count(got, []byte("0 Ts\n")); n != 1 {
		t.Fatalf("want exactly one restoring `0 Ts`, at the end; got %d in %q", n, got)
	}
}

// TestZeroAdjustmentRunEmitsTj is AC6/AC14's compatibility clause,
// asserted on the emitted bytes rather than assumed: a run in which
// every adjustment computes to zero emits a bare Tj hex string —
// exactly the bytes folio8 emitted before Story 2.3 — and a run carrying
// a real offset or a kerned advance emits a TJ array instead.
func TestZeroAdjustmentRunEmitsTj(t *testing.T) {
	face := fakeFace("Body")

	plain := pagemodel.TextRun{Face: "Body", SourceText: "AB", FontSize: 12000, Glyphs: []pagemodel.ShapedGlyph{
		{CID: 1, XAdvance: 500},
		{CID: 2, XAdvance: 600},
	}}
	got, err := appendShapedRun(nil, plain, face)
	if err != nil {
		t.Fatalf("appendShapedRun: %v", err)
	}
	if string(got) != "<00010002> Tj\n" {
		t.Fatalf("a zero-adjustment run must emit today's exact Tj bytes, got %q", got)
	}

	kerned := pagemodel.TextRun{Face: "Body", SourceText: "AB", FontSize: 12000, Glyphs: []pagemodel.ShapedGlyph{
		{CID: 1, XAdvance: 460}, // /W says 500: GPOS kerned it by -40
		{CID: 2, XAdvance: 600},
	}}
	got, err = appendShapedRun(nil, kerned, face)
	if err != nil {
		t.Fatalf("appendShapedRun (kerned): %v", err)
	}
	if !bytes.HasPrefix(got, []byte("[<0001>40<0002>")) {
		t.Fatalf("a kerned run must emit a TJ array carrying the +40 adjustment, got %q", got)
	}

	offset := pagemodel.TextRun{Face: "Body", SourceText: "AB", FontSize: 12000, Glyphs: []pagemodel.ShapedGlyph{
		{CID: 1, XAdvance: 500},
		{CID: 2, XAdvance: 600, XOffset: -29},
	}}
	got, err = appendShapedRun(nil, offset, face)
	if err != nil {
		t.Fatalf("appendShapedRun (offset): %v", err)
	}
	if !bytes.Contains(got, []byte("29<0002>")) {
		t.Fatalf("an x-offset run must emit the +29 pre-glyph adjustment, got %q", got)
	}
}

// TestAppendXrefGeneralHandlesArbitraryObjectCounts is a basic sanity
// check on the generalised xref writer (document.go's appendXref stays
// fixed at exactly Story 1.1's four objects, untouched per AC14a).
func TestAppendXrefGeneralHandlesArbitraryObjectCounts(t *testing.T) {
	offsets := []int{0, 10, 200, 3000, 40000, 500000, 6000000}
	out := appendXrefGeneral(nil, offsets)
	if !bytes.HasPrefix(out, []byte("xref\n0 7\n")) {
		t.Fatalf("unexpected xref header: %q", out[:20])
	}
	lines := bytes.Split(bytes.TrimSuffix(out[len("xref\n0 7\n"):], nil), []byte("\n"))
	// 7 offsets -> 7 entries, each 20 bytes (19 chars + trailing \n via Split).
	if len(lines) < 7 {
		t.Fatalf("expected at least 7 xref entry lines, got %d", len(lines))
	}
}

// TestAssetKeyEscapeIsIdentity is AC18a: asset keys are 64 lowercase hex
// characters, a strict subset of pdfNameEscape's kept set
// ([A-Za-z0-9_-]), so escaping is the identity and distinct keys stay
// distinct — asserted directly rather than left to be re-derived (the
// font path's resource-name collision hazard, Story 1.5 QA Finding 23,
// cannot arise here).
func TestAssetKeyEscapeIsIdentity(t *testing.T) {
	key := "5a05ad01e89c143b7061b0c93450566568d38a23da9b9c5c9dfe449016433078"
	if got := pdfNameEscape(key); got != key {
		t.Fatalf("pdfNameEscape(%q) = %q, want the identity (AC18a)", key, got)
	}
}

// TestColouredRunBracketsItsInk is Story 10.1's byte-level assertion: a
// run carrying an ink emits a fill-colour operator inside its own q/Q
// bracket, so the colour cannot leak into whatever draws next; a run
// carrying none emits no colour operator at all, which is what leaves
// every document written before style.color existed byte-identical.
func TestColouredRunBracketsItsInk(t *testing.T) {
	faces := map[string]EmbeddedFace{"Body": fakeFace("Body")}
	run := pagemodel.TextRun{Face: "Body", SourceText: "A", FontSize: 12000, X: 1000, Y: 2000, Glyphs: []pagemodel.ShapedGlyph{{CID: 1, XAdvance: 500}}}

	plain, err := buildTextContentStream(pagemodel.Page{Width: 600000, Height: 800000, Runs: []pagemodel.TextRun{run}}, faces)
	if err != nil {
		t.Fatalf("buildTextContentStream: %v", err)
	}
	if bytes.Contains(plain, []byte(" rg")) || bytes.Contains(plain, []byte("q\n")) {
		t.Errorf("an uncoloured run emitted a colour operator or a bracket: %q", plain)
	}

	run.HasColor, run.Color = true, pagemodel.Color{R: 255, G: 0, B: 0}
	coloured, err := buildTextContentStream(pagemodel.Page{Width: 600000, Height: 800000, Runs: []pagemodel.TextRun{run}}, faces)
	if err != nil {
		t.Fatalf("buildTextContentStream (coloured): %v", err)
	}
	if !bytes.HasPrefix(coloured, []byte("q\n1 0 0 rg\nBT\n")) {
		t.Errorf("a coloured run must set its fill inside its own bracket, before BT, got %q", coloured)
	}
	if !bytes.HasSuffix(coloured, []byte("ET\nQ\n")) {
		t.Errorf("a coloured run must close its bracket after ET, got %q", coloured)
	}
	// The ONE difference between the two streams is the bracket and the
	// operator: the text itself is emitted identically.
	if got := bytes.ReplaceAll(bytes.ReplaceAll(coloured, []byte("q\n1 0 0 rg\n"), nil), []byte("ET\nQ\n"), []byte("ET\n")); !bytes.Equal(got, plain) {
		t.Errorf("colouring a run changed more than its ink:\n coloured-minus-ink %q\n plain              %q", got, plain)
	}
}
