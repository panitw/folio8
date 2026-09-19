package folio8

import (
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/pagemodel"
)

// contentBandPageSlotTemplate places {{page}} in a CONTENT-band
// element — D-2.7.3's red-proof subject, a REAL template rather than an
// argument (the ruling's own instruction). It is deliberately minimal:
// one content-band text element, no header or footer at all, so the
// only thing that could make this document render is a construct
// resolving somewhere Y is not yet independent of layout.
const contentBandPageSlotTemplate = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 480, "height": 40, "value": "You are on page {{page}}.", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 24
    },
    "pageHeader": {
      "elements": [],
      "height": 18
    }
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// TestPageSlotInContentBandIsALocatedError is D-2.7.3's red-proof: a
// REAL template placing {{page}} in a content-band element, not an
// argument. The construct is fenced to the page-header/page-footer
// bands because ONLY there is Y independent of the construct's own
// width (finding 2, story creation) — in the content band this is a
// fixed point AD-24 forbids negotiating, so it is a located template
// error naming the element, never a silent literal and never a
// best-effort render.
func TestPageSlotInContentBandIsALocatedError(t *testing.T) {
	tpl, err := ParseTemplate([]byte(contentBandPageSlotTemplate))
	if err != nil {
		t.Fatalf("presence precondition: the template does not parse: %v", err)
	}
	_, rerr := Render(tpl, Data("{}"), nil, testShippedFontSet())
	if rerr == nil {
		t.Fatal("Render succeeded on a content-band {{page}} construct — D-2.7.3 requires this to be a " +
			"located template error, since the page count this construct needs depends on this band's own " +
			"layout, which AD-24 does not permit resolving by negotiation")
	}
	if !strings.Contains(rerr.Error(), "e1") {
		t.Errorf("error does not name the offending element (e1): %v", rerr)
	}
	// This story's review, Finding 11: strings.Contains(rerr.Error(), "page")
	// is near-free in a codebase whose errors routinely say "page header",
	// "page count", "page geometry" — assert on D-2.7.3's actual fence
	// phrase instead, so this test can distinguish the fence firing from
	// an unrelated error that merely mentions "page".
	const fencePhrase = "resolves only in the page header and page footer bands"
	if !strings.Contains(rerr.Error(), fencePhrase) {
		t.Errorf("error does not carry D-2.7.3's fence phrase %q: %v", fencePhrase, rerr)
	}
	t.Logf("located error, as required: %v", rerr)
}

// TestPageSlotInContentBandPagesIsAlsoALocatedError is the same fence
// applied to {{pages}} — Y needs no reservation, but it is STILL only
// meaningful once pagination has run, which content-band layout cannot
// presuppose either.
func TestPageSlotInContentBandPagesIsAlsoALocatedError(t *testing.T) {
	src := strings.Replace(contentBandPageSlotTemplate, "{{page}}", "{{pages}}", 1)
	tpl, err := ParseTemplate([]byte(src))
	if err != nil {
		t.Fatalf("presence precondition: the template does not parse: %v", err)
	}
	_, rerr := Render(tpl, Data("{}"), nil, testShippedFontSet())
	if rerr == nil {
		t.Fatal("Render succeeded on a content-band {{pages}} construct — D-2.7.3 fences both reserved tokens to the page-header/page-footer bands")
	}
	// This story's review, Finding 11: the previous version asserted
	// only rerr == nil, which passes on ANY render failure — a parse
	// error, a font error, a geometry error — and cannot distinguish
	// D-2.7.3's fence from an unrelated fault. Assert the fence's own
	// identity, same phrase as the {{page}} sibling test above.
	const fencePhrase = "resolves only in the page header and page footer bands"
	if !strings.Contains(rerr.Error(), fencePhrase) {
		t.Errorf("error does not carry D-2.7.3's fence phrase %q: %v", fencePhrase, rerr)
	}
	if !strings.Contains(rerr.Error(), "e1") {
		t.Errorf("error does not name the offending element (e1): %v", rerr)
	}
	t.Logf("located error, as required: %v", rerr)
}

// TestResolvePageRunForPageUsesPreMeasuredGlyphsNotReshaping is AC2's
// required red-proof (D-2.7.1's "the glyphs substituted are
// pre-measured"): resolvePageRunForPage must SELECT among DigitCID's
// ten pre-shaped values, never derive a CID by any other route.
//
// resolvePageRunForPage's own SIGNATURE is half the proof: it takes no
// FontSet and no fontCache, so it cannot consult a font even if it
// wanted to. This test is the executed half (D-000.52) — a
// deliberately WRONG DigitCID table (values no real shaper would ever
// produce) is injected, and the substituted output is asserted to carry
// exactly those wrong values. If the function instead derived a CID
// independently of DigitCID, this would fail with the REAL CIDs
// showing up instead of the injected ones — the failure a re-introduced
// re-shape would produce.
func TestResolvePageRunForPageUsesPreMeasuredGlyphsNotReshaping(t *testing.T) {
	const digitAdvance = int64(572) // Noto Sans, measured at this story's creation
	slot := pagemodel.PageNumberSlot{
		GlyphLo:      2,
		GlyphHi:      4, // digits(Y) == 2: room for two digits
		DigitsY:      2,
		DigitAdvance: digitAdvance,
		DigitCID: [10]uint16{
			0: 9990, 1: 9991, 2: 9992, 3: 9993, 4: 9994,
			5: 9995, 6: 9996, 7: 9997, 8: 9998, 9: 9999,
		},
	}
	run := pagemodel.TextRun{
		Face: "test-face",
		Glyphs: []pagemodel.ShapedGlyph{
			{CID: 1, XAdvance: 100},          // "P"
			{CID: 2, XAdvance: 100},          // "a" (stand-in for "Page " prefix glyphs)
			{CID: 0, XAdvance: digitAdvance}, // reserved digit slot [2:4)
			{CID: 0, XAdvance: digitAdvance},
			{CID: 3, XAdvance: 100}, // " of 99" suffix glyph
		},
		PageSlots: []pagemodel.PageNumberSlot{slot},
	}

	// Page 7: one digit, right-aligned -- glyph index 2 is skipped
	// (slack), glyph index 3 carries digit 7's INJECTED CID.
	resolved := resolvePageRunForPage(run, 7)
	if resolved.PageSlots != nil {
		t.Error("resolvePageRunForPage must clear PageSlots on the resolved run — pass two must never see it")
	}
	if len(resolved.Glyphs) != 4 {
		t.Fatalf("page 7 (1 digit) in a 2-digit reservation: got %d glyphs, want 4 (2 prefix + 1 digit + 1 suffix)", len(resolved.Glyphs))
	}
	if resolved.Glyphs[2].CID != slot.DigitCID[7] {
		t.Errorf("page 7's drawn digit CID = %d, want the INJECTED table value %d — a real shaper would "+
			"never produce 9997; if this shows a DIFFERENT number, the function re-derived a CID instead "+
			"of selecting DigitCID[7]", resolved.Glyphs[2].CID, slot.DigitCID[7])
	}
	if resolved.Glyphs[2].XOffset != digitAdvance {
		t.Errorf("page 7's slack shift = %d, want exactly one digit-advance (%d) — D-2.7.2's right alignment", resolved.Glyphs[2].XOffset, digitAdvance)
	}
	if got, want := resolved.Glyphs[2].XAdvance, digitAdvance+digitAdvance; got != want {
		t.Errorf("page 7's shifted digit XAdvance = %d, want %d (slack + one digit-advance, so the suffix lands where the 2-digit canonical layout already put it)", got, want)
	}
	if resolved.Glyphs[3].CID != 3 {
		t.Errorf("the suffix glyph (originally CID 3, after the reserved slot) must be UNTOUCHED by resolution; got CID %d", resolved.Glyphs[3].CID)
	}

	// Page 42: two digits, fills the reservation exactly -- no slack,
	// both injected CIDs, neither carries an offset.
	resolved42 := resolvePageRunForPage(run, 42)
	if len(resolved42.Glyphs) != 5 {
		t.Fatalf("page 42 (2 digits) in a 2-digit reservation: got %d glyphs, want 5", len(resolved42.Glyphs))
	}
	if resolved42.Glyphs[2].CID != slot.DigitCID[4] || resolved42.Glyphs[3].CID != slot.DigitCID[2] {
		t.Errorf("page 42's digits = (%d,%d), want the injected table values for 4 and 2 (%d,%d)",
			resolved42.Glyphs[2].CID, resolved42.Glyphs[3].CID, slot.DigitCID[4], slot.DigitCID[2])
	}
	if resolved42.Glyphs[2].XOffset != 0 {
		t.Errorf("page 42 fills the reservation exactly — no slack, so no offset; got XOffset=%d", resolved42.Glyphs[2].XOffset)
	}

	// A run with no PageSlots passes through byte-for-byte -- the
	// no-op path every pre-2.7 document relies on.
	plain := pagemodel.TextRun{Face: "test-face", Glyphs: []pagemodel.ShapedGlyph{{CID: 1, XAdvance: 100}}}
	if got := resolvePageRunForPage(plain, 5); len(got.Glyphs) != 1 || got.Glyphs[0] != plain.Glyphs[0] {
		t.Errorf("a run with an empty PageSlots must pass through unchanged; got %+v", got)
	}
}

// TestResolvePageRunForPageHandlesTwoSlotsInOneRun is Blocker 1's
// red-proof at the unit level (D-000.52): two {{page}} occurrences in
// one element are ASCII, so positionSegments places them in a SINGLE
// run — before this story's review, PageSlots was a scalar field and
// the second occurrence's positionSegments write silently OVERWROTE the
// first, so a run carrying two reservations resolved only the LAST one
// and drew the reservation's filler "0" for the first, on every page,
// with no error. This test constructs that run directly (two DISJOINT
// reservations, glyph ranges [1:2) and [4:5)) and asserts BOTH resolve
// correctly and independently. The full-render-level red-proof, on a
// REAL two-occurrence template, is
// TestReservedPagePlaceholdersResolveTwoOccurrencesInOneElement
// (reserved_placeholders_test.go).
func TestResolvePageRunForPageHandlesTwoSlotsInOneRun(t *testing.T) {
	const digitAdvance = int64(572)
	digitCID := [10]uint16{0: 100, 1: 101, 2: 102, 3: 103, 4: 104, 5: 105, 6: 106, 7: 107, 8: 108, 9: 109}
	slotA := pagemodel.PageNumberSlot{GlyphLo: 1, GlyphHi: 2, DigitsY: 1, DigitAdvance: digitAdvance, DigitCID: digitCID}
	slotB := pagemodel.PageNumberSlot{GlyphLo: 4, GlyphHi: 5, DigitsY: 1, DigitAdvance: digitAdvance, DigitCID: digitCID}
	run := pagemodel.TextRun{
		Face: "test-face",
		Glyphs: []pagemodel.ShapedGlyph{
			{CID: 1, XAdvance: 100},          // "P" (prefix, before slot A)
			{CID: 0, XAdvance: digitAdvance}, // slot A's reserved digit [1:2)
			{CID: 2, XAdvance: 100},          // " of N / " (between the two slots)
			{CID: 3, XAdvance: 100},          // more between-text
			{CID: 0, XAdvance: digitAdvance}, // slot B's reserved digit [4:5)
		},
		// Deliberately reversed order: resolvePageRunForPage must sort
		// by GlyphLo rather than assume PageSlots is pre-sorted.
		PageSlots: []pagemodel.PageNumberSlot{slotB, slotA},
	}

	resolved := resolvePageRunForPage(run, 3)
	if len(resolved.Glyphs) != 5 {
		t.Fatalf("got %d glyphs, want 5 (unchanged count: both slots are 1-digit reservations resolving to 1-digit pages)", len(resolved.Glyphs))
	}
	if resolved.Glyphs[1].CID != digitCID[3] {
		t.Errorf("slot A (glyph index 1) did not resolve to page 3's digit: got CID %d, want %d — "+
			"this is the exact defect Blocker 1 names: a scalar PageSlot field let the SECOND slot's "+
			"write overwrite the first, leaving slot A drawing the reservation's filler digit", resolved.Glyphs[1].CID, digitCID[3])
	}
	if resolved.Glyphs[4].CID != digitCID[3] {
		t.Errorf("slot B (glyph index 4) did not resolve to page 3's digit: got CID %d, want %d", resolved.Glyphs[4].CID, digitCID[3])
	}
	if resolved.Glyphs[0].CID != 1 || resolved.Glyphs[2].CID != 2 || resolved.Glyphs[3].CID != 3 {
		t.Errorf("the non-slot glyphs must be untouched by resolution; got %+v", resolved.Glyphs)
	}
}

// TestBuildPageNumberSlotFailsClosedOnNonUniformDigitAdvances is
// D-2.7.2's residual hazard, made concrete: "the face set is not
// frozen, and a proportional-figure face would silently break the
// reservation." buildPageNumberSlot is the render-time consequence —
// this test constructs a digit table whose advances are NOT uniform
// (a proportional-figure face's shape) and asserts a located error,
// never a silently wrong reservation.
func TestBuildPageNumberSlotFailsClosedOnNonUniformDigitAdvances(t *testing.T) {
	glyphs := make([]pagemodel.ShapedGlyph, 10)
	for d := 0; d < 10; d++ {
		glyphs[d] = pagemodel.ShapedGlyph{CID: uint16(100 + d), XAdvance: 572}
	}
	glyphs[3].XAdvance = 611 // digit '3' is wider -- a proportional-figure face

	dt := pagemodel.TextRun{Face: "proportional-test-face", Glyphs: glyphs}
	ps := pendingPageSlot{runIndex: 0, glyphLo: 5, glyphHi: 7, digitsY: 2, digitTableIndex: 1}

	_, err := buildPageNumberSlot("proportional-test-face", dt, ps)
	if err == nil {
		t.Fatal("buildPageNumberSlot must fail closed when the ten digit advances are not identical — a " +
			"silently accepted reservation would break for any page whose digits included '3'")
	}
	// This story's review, Finding 11: asserting err != nil alone is
	// satisfied by buildPageNumberSlot's OTHER error path (a digit
	// table not carrying exactly 10 glyphs) too — this test's ps.glyphLo
	// (5) and ps.glyphHi (7) are arbitrary relative to the 10-glyph run,
	// so it happens to satisfy the uniformity path today only because
	// that is the sole check the 10-glyph dt can still fail. Assert the
	// error names the uniformity mechanism specifically, so a future
	// bounds check added ahead of the uniformity loop cannot silently
	// steal this test's coverage.
	if !strings.Contains(err.Error(), "advance") {
		t.Errorf("error does not name the ADVANCE mismatch specifically (want it to mention \"advance\"): %v", err)
	}
	t.Logf("failed closed as required: %v", err)
}
