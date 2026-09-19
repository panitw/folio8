package folio8

import (
	"fmt"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// This file is Story 11.2's acceptance suite: `style.bold` and
// `style.italic` resolved to REAL weighted and sloped faces, per rune,
// through the DECLARED chain (FR57) — with no synthetic emboldening or
// obliquing anywhere, and with nothing inferred, parsed or constructed
// from a face name.

// styleFaceDoc builds a one-text-element document over an arbitrary
// chain literal and style block, so every case below differs only in the
// two things the story is about.
func styleFaceDoc(chain, style, value string) string {
	return fmt.Sprintf(`{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 480, "height": 40, "value": %s, "style": %s}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": %s},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "2.0"
}
`, value, style, chain)
}

// renderedFacesAndDiags is the END-TO-END observation every case here
// makes: the faces the page model actually carries, in drawing order,
// and the diagnostics the render produced. It goes through
// buildPageModel rather than through shapeSegments directly, because a
// resolution that worked at the seam and never reached the page would
// pass a seam-level assertion and ship a wrong page.
func renderedFacesAndDiags(t *testing.T, source string) ([]string, []Diagnostic) {
	t.Helper()
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	pages, _, _, diags, err := buildPageModel(tpl, mustDecodeData(t, `{}`), mustDecodeParams(t), testShippedFontSet())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var faces []string
	for _, p := range pages {
		for _, r := range p.Runs {
			faces = append(faces, r.Face)
		}
	}
	if len(faces) == 0 {
		t.Fatalf("precondition: the document produced no text runs, so no face assertion below means anything")
	}
	return faces, diags
}

func requireFaces(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("drew %d run(s) %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("run %d drew in %q, want %q (all runs: %v)", i, got[i], want[i], got)
		}
	}
}

// baselineOffsetOf is the element's LEADING, read off the page model.
// It is a function of the chain and the font size and of nothing that
// was drawn, so it can be compared across documents whose text differs —
// which is what lets every leading assertion below be output-against-
// output rather than a second derivation of the vertical model.
func baselineOffsetOf(t *testing.T, source string) geom.Length {
	t.Helper()
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	pages, _, _, _, err := buildPageModel(tpl, mustDecodeData(t, `{}`), mustDecodeParams(t), testShippedFontSet())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, p := range pages {
		for _, r := range p.Runs {
			return r.BaselineOffset
		}
	}
	t.Fatal("precondition: the document produced no run, so it has no leading to read")
	return 0
}

func styleFaceWarnings(diags []Diagnostic) []Diagnostic {
	var out []Diagnostic
	for _, d := range diags {
		if d.Code == DiagCodeTextStyleFaceUndeclared {
			out = append(out, d)
		}
	}
	return out
}

// TestADeclaredVariantIsShapedAndMeasuredFromItsOwnFace is AC1: the
// glyphs come from the declared variant's own metrics, never the base
// face's.
//
// The width assertion is what makes it about METRICS and not only about
// a name: Roboto Bold is a different program with different advances, so
// a resolver that named the bold face and measured the regular one would
// name-check green here and lay out wrong.
func TestADeclaredVariantIsShapedAndMeasuredFromItsOwnFace(t *testing.T) {
	const chain = `[{"face": "Roboto", "bold": "Roboto Bold", "italic": "Roboto Italic", "boldItalic": "Roboto Bold Italic"}]`
	for _, tc := range []struct {
		name  string
		style string
		want  string
	}{
		{"regular", `{"fontFamily": "body", "fontSize": 12}`, "Roboto"},
		{"bold", `{"fontFamily": "body", "fontSize": 12, "bold": true}`, "Roboto Bold"},
		{"italic", `{"fontFamily": "body", "fontSize": 12, "italic": true}`, "Roboto Italic"},
		{"bold and italic", `{"fontFamily": "body", "fontSize": 12, "bold": true, "italic": true}`, "Roboto Bold Italic"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			faces, diags := renderedFacesAndDiags(t, styleFaceDoc(chain, tc.style, `"Hamburgefonstiv"`))
			requireFaces(t, faces, tc.want)
			if w := styleFaceWarnings(diags); len(w) != 0 {
				t.Fatalf("a declared variant still warned: %+v", w)
			}
		})
	}

	// AC1's measurement half, stated as a difference: the four cuts do
	// not all measure the same, so the engine cannot be reading one face
	// for every request.
	widths := map[string]int{}
	for _, tc := range []struct{ name, style string }{
		{"regular", `{"fontFamily": "body", "fontSize": 12}`},
		{"bold", `{"fontFamily": "body", "fontSize": 12, "bold": true}`},
	} {
		tpl, err := ParseTemplate([]byte(styleFaceDoc(chain, tc.style, `"Hamburgefonstiv"`)))
		if err != nil {
			t.Fatal(err)
		}
		el := tpl.doc.Bands.Content.Elements[0]
		base, styled, err := fontChain(tpl, el)
		if err != nil {
			t.Fatal(err)
		}
		cache := newDocumentFontCache(tpl)
		segs, _, err := shapeSegments("e1", base, styled, el.Value.Value, testShippedFontSet(), cache, breaksAreConsumed)
		if err != nil {
			t.Fatal(err)
		}
		widths[tc.name] = int(measureRuneRange(segs, 0, len([]rune(el.Value.Value)), 12_000))
	}
	if widths["regular"] == widths["bold"] {
		t.Fatalf("the bold and regular cuts measured identically (%d) — the declared variant's own metrics are not being read", widths["regular"])
	}
}

// TestABareEntryNeverConstructsAVariantName is the DW-233 tripwire, and
// it is the single most load-bearing test in this story.
//
// The chain entry is the bare string "Roboto". The supplied FontSet DOES
// contain "Roboto Bold". The element declares bold. The correct answer
// is Roboto REGULAR plus a Warning, because FR57 resolves "per rune
// through the DECLARED chain" and this chain declares nothing.
//
// DO NOT DELETE OR WEAKEN THIS TEST, AND THIS IS WHY — it is not here
// because a register asked for it. Measured at 3ad4ede: no pinned golden
// renders a bold- or italic-declaring document (`grep -rn '"bold"' fixtures/`
// → 0 across all 30 fixture dirs), and nothing pins worked-example.json's
// rendered BYTES. So an implementation that fell back to constructing
// `entry.Face + " Bold"` would silently switch a face, fire no Warning,
// move no golden, and leave every other test in the tree green. This
// test is the only thing standing between the repository and a silent
// face swap that nothing else can see.
func TestABareEntryNeverConstructsAVariantName(t *testing.T) {
	if _, ok := testShippedFontSet()["Roboto Bold"]; !ok {
		t.Fatal("precondition: the FontSet does not supply \"Roboto Bold\", so a name-constructing resolver would fail for the wrong reason and this test would pass vacuously")
	}
	faces, diags := renderedFacesAndDiags(t,
		styleFaceDoc(`["Roboto"]`, `{"fontFamily": "body", "fontSize": 12, "bold": true}`, `"Ab"`))
	requireFaces(t, faces, "Roboto")
	w := styleFaceWarnings(diags)
	if len(w) == 0 {
		t.Fatal("an entry with no declared variant rendered its base face SILENTLY — AC3's Warning is the whole record that the weight was lost")
	}
	for _, d := range w {
		if d.ElementID != "e1" {
			t.Errorf("Warning is not located at the element: %+v", d)
		}
		if !strings.Contains(d.Message, "Roboto") {
			t.Errorf("Warning %q does not name the face the rune was actually drawn in", d.Message)
		}
	}
	// One per (element, DISTINCT rune), never one per occurrence.
	if len(w) != 2 {
		t.Fatalf("expected one Warning per distinct rune of \"Ab\", got %d: %+v", len(w), w)
	}
}

// TestPermanentAbsenceRendersTheEntrysOwnBaseFaceAndWarns is AC3 over
// the two faces the shipped set has no cut for. They are not a
// hypothetical: Noto Sans SC ships Regular only, and Noto Sans Thai
// ships Regular and Bold and no italic, so no chain an author can write
// makes these go away.
func TestPermanentAbsenceRendersTheEntrysOwnBaseFaceAndWarns(t *testing.T) {
	for _, tc := range []struct {
		name  string
		chain string
		style string
		value string
		want  string
	}{
		{
			"a CJK rune with no bold cut",
			`[{"face": "Noto Sans", "bold": "Noto Sans Bold"}, "Noto Sans SC"]`,
			`{"fontFamily": "body", "fontSize": 12, "bold": true}`,
			`"中"`,
			"Noto Sans SC",
		},
		{
			"a Thai rune with no italic cut",
			`[{"face": "Noto Sans", "italic": "Noto Sans Italic"}, {"face": "Noto Sans Thai", "bold": "Noto Sans Thai Bold"}]`,
			`{"fontFamily": "body", "fontSize": 12, "italic": true}`,
			`"ก"`,
			"Noto Sans Thai",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			faces, diags := renderedFacesAndDiags(t, styleFaceDoc(tc.chain, tc.style, tc.value))
			requireFaces(t, faces, tc.want)
			w := styleFaceWarnings(diags)
			if len(w) != 1 {
				t.Fatalf("want exactly one Warning, got %d: %+v", len(w), w)
			}
			if !strings.Contains(w[0].Message, tc.want) {
				t.Errorf("Warning %q does not name the face actually drawn (%s)", w[0].Message, tc.want)
			}
		})
	}
}

// TestTheLatinRunKeepsItsBoldWhileTheAbsentOneDoesNot is the same
// document read the other way: absence is per ENTRY, so one element can
// carry both answers at once. A resolver that gave up on the whole
// element the moment one entry lacked a variant would pass the two cases
// above and fail this one.
func TestTheLatinRunKeepsItsBoldWhileTheAbsentOneDoesNot(t *testing.T) {
	faces, diags := renderedFacesAndDiags(t, styleFaceDoc(
		`[{"face": "Noto Sans", "bold": "Noto Sans Bold"}, "Noto Sans SC"]`,
		`{"fontFamily": "body", "fontSize": 12, "bold": true}`,
		`"A中B"`))
	requireFaces(t, faces, "Noto Sans Bold", "Noto Sans SC", "Noto Sans Bold")
	w := styleFaceWarnings(diags)
	if len(w) != 1 {
		t.Fatalf("want exactly the CJK rune's Warning, got %d: %+v", len(w), w)
	}
}

// TestTheChainIsNeverWALKEDForWeight is the trap this design exists to
// avoid. Entry A covers the rune and declares no bold; entry B, later in
// the chain, does. The answer is A's base face — LOSING THE WEIGHT IS A
// SMALLER LIE THAN CHANGING THE TYPEFACE, and hunting down the chain for
// something bold is the substitution AD-8 forbids by name.
func TestTheChainIsNeverWALKEDForWeight(t *testing.T) {
	faces, diags := renderedFacesAndDiags(t, styleFaceDoc(
		`["Roboto", {"face": "Noto Sans", "bold": "Noto Sans Bold"}]`,
		`{"fontFamily": "body", "fontSize": 12, "bold": true}`,
		`"A"`))
	requireFaces(t, faces, "Roboto")
	if len(styleFaceWarnings(diags)) != 1 {
		t.Fatalf("the entry that covered the rune declares no bold, so exactly one Warning is owed: %+v", diags)
	}
}

// TestACrossVariantCollisionLOADSANDRENDERS is DW-241's other direction at
// the RENDER end, and it is not optional (D-11.2.11).
//
// Story 11.4 made a variant naming its entry's own base a load error. The
// comparison is against THAT ARM'S OWN DISCRIMINANT and nothing else, so one
// face declared as both the bold and the italic cut is a real declaration and
// stays legal — and it must actually DRAW, not merely parse. Without this the
// narrowing quietly becomes "no two variants may agree", which is wider than
// what was ruled and is the failure a load-side test alone cannot see.
//
// ⚠ THE ASSERTION IS THE FACE, AND THE WARNING COUNT IS ZERO. A declared cut
// is a declared cut: an element asking for italic against this entry must be
// drawn in the face the entry names for italic, with no absence Warning — that
// is what makes this the LEGITIMATE spelling of the pattern the guard refuses,
// rather than a defect the guard happened to let through (D-11.3.7).
func TestACrossVariantCollisionLOADSANDRENDERS(t *testing.T) {
	const collided = `[{"face": "Roboto", "bold": "Roboto Bold", "italic": "Roboto Bold"}]`
	for _, tc := range []struct{ name, style, want string }{
		{"bold takes the declared cut", `{"fontFamily": "body", "fontSize": 12, "bold": true}`, "Roboto Bold"},
		{"italic takes the SAME declared cut", `{"fontFamily": "body", "fontSize": 12, "italic": true}`, "Roboto Bold"},
		// And the base is still the base: the collision decorates the entry, it
		// does not replace it.
		{"unstyled still draws the base", `{"fontFamily": "body", "fontSize": 12}`, "Roboto"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			faces, diags := renderedFacesAndDiags(t, styleFaceDoc(collided, tc.style, `"A"`))
			requireFaces(t, faces, tc.want)
			if w := styleFaceWarnings(diags); len(w) != 0 {
				t.Fatalf("a DECLARED cut owes no absence Warning, got %+v", w)
			}
		})
	}
	// THE VACUITY GUARD, AND IT IS THE ONE THAT MATTERS HERE: the two faces
	// this test discriminates between must actually be different, or "italic
	// drew Roboto Bold" would be satisfied by an engine that ignored the
	// declaration entirely and drew the base.
	base, _ := renderedFacesAndDiags(t, styleFaceDoc(collided, `{"fontFamily": "body", "fontSize": 12}`, `"A"`))
	italic, _ := renderedFacesAndDiags(t, styleFaceDoc(collided, `{"fontFamily": "body", "fontSize": 12, "italic": true}`, `"A"`))
	if len(base) != 1 || len(italic) != 1 || base[0] == italic[0] {
		t.Fatalf("the styled and unstyled renders drew the same face (%v / %v), so the assertions above cannot tell a declaration from an absence", base, italic)
	}
	// AND THE NEAREST DEFECT IS STILL REFUSED, so this green is a statement
	// about the collision rather than about the guard being absent.
	if _, err := ParseTemplate([]byte(styleFaceDoc(`[{"face": "Roboto", "bold": "Roboto", "italic": "Roboto Bold"}]`, `{"fontFamily": "body", "fontSize": 12}`, `"A"`))); err == nil {
		t.Fatal("a variant naming its own base loaded; the collision above proves nothing about a check that is not there")
	}
}

// TestAPartialStyleMatchIsAnAbsence: bold+italic against an entry
// declaring only `bold` resolves to the BASE face and warns. Choosing
// the bold cut would be a nearest-fit search — the inference this design
// refuses.
func TestAPartialStyleMatchIsAnAbsence(t *testing.T) {
	faces, diags := renderedFacesAndDiags(t, styleFaceDoc(
		`[{"face": "Roboto", "bold": "Roboto Bold"}]`,
		`{"fontFamily": "body", "fontSize": 12, "bold": true, "italic": true}`,
		`"A"`))
	requireFaces(t, faces, "Roboto")
	if len(styleFaceWarnings(diags)) != 1 {
		t.Fatalf("a partial match is an absence and owes a Warning: %+v", diags)
	}
}

// TestADeclaredVariantThatCannotDrawTheRuneFallsBackToItsOwnBaseFace:
// coverage chose the entry on its BASE face, so the declared variant is
// not guaranteed to carry the glyph. That is an absence too — the
// entry's own base face and the same Warning — rather than a second
// fallback policy invented for the case.
//
// Noto Sans SC covers U+4E2D and Noto Sans Thai does not, so the entry
// is chosen on its base face and its declared bold cannot draw the rune.
//
// ⚠ THE FACES ARE THIS WAY ROUND SO THE LEADING ASSERTION CAN
// DISCRIMINATE, and that is measured, not aesthetic. Noto Sans SC's hhea
// ascent is the TALLER of the two, so an implementation that let the
// declared variant REPLACE its base face in the metrics chain would drop
// the taller face — the one the glyphs actually came from — and shorten
// the line. With the faces the other way round the max is the same
// either way and the assertion would be vacuous; the guard below pins
// that they really do differ.
func TestADeclaredVariantThatCannotDrawTheRuneFallsBackToItsOwnBaseFace(t *testing.T) {
	const styled = `[{"face": "Noto Sans SC", "bold": "Noto Sans Thai"}]`
	faces, diags := renderedFacesAndDiags(t, styleFaceDoc(styled,
		`{"fontFamily": "body", "fontSize": 12, "bold": true}`, `"\u4e2d"`))
	requireFaces(t, faces, "Noto Sans SC")
	w := styleFaceWarnings(diags)
	if len(w) != 1 {
		t.Fatalf("a variant that cannot draw the rune is an absence and owes a Warning: %+v", diags)
	}
	if !strings.Contains(w[0].Message, "Noto Sans SC") {
		t.Errorf("Warning %q does not name the base face the rune fell back to", w[0].Message)
	}

	// AND THE LEADING FOLLOWS THE FACES THAT MAY DRAW, not the declared
	// variant alone. Both controls are UNSTYLED documents over plain
	// chains, so this compares one engine output against another and
	// re-derives no arithmetic.
	const plainBoth = `["Noto Sans SC", "Noto Sans Thai"]`
	const plainVariantOnly = `["Noto Sans Thai"]`
	const noStyle = `{"fontFamily": "body", "fontSize": 12}`

	both := baselineOffsetOf(t, styleFaceDoc(plainBoth, noStyle, `"\u4e2d"`))
	variantOnly := baselineOffsetOf(t, styleFaceDoc(plainVariantOnly, noStyle, `"\u0e01"`))
	if both == variantOnly {
		t.Fatalf("vacuity guard: the two faces yield the same leading (%d), so this assertion cannot tell a metrics chain that DROPPED the base face from one that kept it", both)
	}
	got := baselineOffsetOf(t, styleFaceDoc(styled, `{"fontFamily": "body", "fontSize": 12, "bold": true}`, `"\u4e2d"`))
	if got != both {
		t.Fatalf("the styled element's leading is %d, want %d — the metrics chain must cover EVERY face that may draw, and the glyphs came from the base face this leading has dropped (variant-only leading would be %d)", got, both, variantOnly)
	}
}

// TestThePageNumberDigitTableIsShapedFromTheSlotRunsOwnFace is P2's
// pairing, and it is a correctness hazard rather than a cosmetic one.
//
// buildPageNumberSlot takes the FACE NAME from the run carrying the
// {{page}} reservation and the CIDs and advances from the digit-table
// run, and resolvePageRunForPage injects those digits with NO
// RE-SHAPING. If the two were shaped from different faces, one font
// program's glyph ids would be drawn out of another's — arbitrary wrong
// glyphs in a page number, reported by nothing.
//
// Style variants put that identity at risk in a way it had never been:
// the element's runes resolve through the styled list, so any
// separately-derived chain handed to digitTableRun can land elsewhere.
// The observable here is end-to-end — the substituted digit's ADVANCE
// comes from the digit table, while Face comes from the slot's run — so
// a divergence shows up as the two disagreeing.
func TestThePageNumberDigitTableIsShapedFromTheSlotRunsOwnFace(t *testing.T) {
	pageNumberDoc := func(chain, style string) string {
		return `{
  "assets": {},
  "bands": {
    "content": {"elements": []},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [
      {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 20, "value": "{{page}}", "style": ` + style + `}
    ], "height": 20}
  },
  "fonts": {"body": ` + chain + `},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "2.0"
}`
	}
	// digitAdvanceAndFace reads the SUBSTITUTED page-number digit: its
	// advance came from the digit table, its Face from the slot's run.
	digitAdvanceAndFace := func(t *testing.T, source string) (geom.Length, string) {
		t.Helper()
		tpl, err := ParseTemplate([]byte(source))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		pages, _, _, _, err := buildPageModel(tpl, mustDecodeData(t, `{}`), mustDecodeParams(t), testShippedFontSet())
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		for _, p := range pages {
			for _, r := range p.Runs {
				if len(r.Glyphs) == 0 {
					continue
				}
				return geom.Length(r.Glyphs[0].XAdvance), r.Face
			}
		}
		t.Fatal("precondition: the page-number element produced no glyph, so there is nothing to compare")
		return 0, ""
	}

	const plain = `{"fontFamily": "body", "fontSize": 12}`
	const bold = `{"fontFamily": "body", "fontSize": 12, "bold": true}`

	// The two controls are UNSTYLED documents over bare chains, so each
	// one's digit advance is unambiguously that one face's.
	regularAdv, regularFace := digitAdvanceAndFace(t, pageNumberDoc(`["Roboto"]`, plain))
	boldAdv, boldFace := digitAdvanceAndFace(t, pageNumberDoc(`["Roboto Bold"]`, plain))
	if regularFace != "Roboto" || boldFace != "Roboto Bold" {
		t.Fatalf("precondition: the controls drew in %q and %q, want Roboto and Roboto Bold", regularFace, boldFace)
	}
	if regularAdv == boldAdv {
		t.Fatalf("vacuity guard: Roboto and Roboto Bold advance their digits identically (%d), so this test cannot tell which face shaped the digit table", regularAdv)
	}

	gotAdv, gotFace := digitAdvanceAndFace(t, pageNumberDoc(`[{"face": "Roboto", "bold": "Roboto Bold"}]`, bold))
	if gotFace != "Roboto Bold" {
		t.Fatalf("the slot's run drew in %q, want Roboto Bold", gotFace)
	}
	if gotAdv != boldAdv {
		t.Fatalf("the substituted page-number digit advances %d, but the run's own face (%s) advances %d — the digit table was shaped from a DIFFERENT face than the run it feeds, so its CIDs belong to another font program (Roboto's own advance is %d)",
			gotAdv, gotFace, boldAdv, regularAdv)
	}
}

// TestADeclaredVariantTheFontSetDoesNotSUPPLYFallsBackToItsOwnBaseFace is// TestADeclaredVariantTheFontSetDoesNotSUPPLYFallsBackToItsOwnBaseFace is
// the matrix's "variant names an absent FontSet face" row, and it is a
// THIRD path to the same absence — distinct from the two above and
// reached through different code.
//
//   - The no-variant-declared path (TestPermanentAbsence…) never has a
//     name to look up at all.
//   - The narrow-cmap path (TestADeclaredVariantThatCannotDrawTheRune…)
//     has a face the FontSet DOES supply, which simply lacks the glyph.
//   - THIS path has a name the FontSet does not supply, so
//     fontCache.declares answers false inside faceCovers and the name
//     never reaches a face at all.
//
// That last one is the PRE-EXISTING FontSet tolerance, and the row's
// whole claim is that it still holds for a VARIANT: a chain entry naming
// a face nobody supplied is skipped in silence, never an error, because
// the same document is correct wherever that face IS supplied (AD-8). The
// tolerance then hands off to AC3's absence arm, so absence has ONE
// policy on every path rather than three.
func TestADeclaredVariantTheFontSetDoesNotSUPPLYFallsBackToItsOwnBaseFace(t *testing.T) {
	// A plausible-looking name that this build simply does not ship. The
	// precondition guard is what keeps the test honest: if the shipped set
	// ever gained this name, the case below would quietly become the
	// declared-and-resolved case and assert nothing about tolerance.
	const unsupplied = "Roboto Condensed Bold"
	if _, ok := testShippedFontSet()[unsupplied]; ok {
		t.Fatalf("precondition: the FontSet now supplies %q, so this test no longer exercises the absent-face tolerance — pick a name it does not supply", unsupplied)
	}
	// Positive control on the same lookup: the entry's BASE face must be
	// supplied, or the fallback below would succeed for the wrong reason.
	if _, ok := testShippedFontSet()["Roboto"]; !ok {
		t.Fatal("precondition: the FontSet does not supply the entry's own base face, so the fallback below proves nothing")
	}

	faces, diags := renderedFacesAndDiags(t, styleFaceDoc(
		`[{"face": "Roboto", "bold": "`+unsupplied+`"}]`,
		`{"fontFamily": "body", "fontSize": 12, "bold": true}`,
		`"A"`))

	// The render SUCCEEDS (renderedFacesAndDiags fatals on an error) and
	// draws the entry's own base face.
	requireFaces(t, faces, "Roboto")

	w := styleFaceWarnings(diags)
	if len(w) != 1 {
		t.Fatalf("want exactly one Warning for the one distinct rune, got %d: %+v", len(w), diags)
	}
	if w[0].ElementID != "e1" {
		t.Errorf("Warning is not located at the element: %+v", w[0])
	}
	// THE MESSAGE CONTENT, not merely the address: it must name the face
	// the rune was ACTUALLY drawn in, which is the entry's base face and
	// not the unsupplied variant the document asked for.
	if !strings.Contains(w[0].Message, "Roboto") {
		t.Errorf("Warning %q does not name the base face the rune fell back to", w[0].Message)
	}
	if strings.Contains(w[0].Message, unsupplied) {
		t.Errorf("Warning %q names the face that was NOT drawn — the message reports what happened, not what was asked for", w[0].Message)
	}
}

// TestCoverageIsDecidedOnTheBaseChainNotTheStyledOne is the trap stated
// directly, and it is the one this whole two-slice shape exists for.
//
// Entry 0 is Noto Sans Thai, whose BASE face covers the Thai rune; its
// declared bold is Noto Sans SC, which does not. Entry 1 is Noto Sans
// Thai Bold, which does. If the bold name were substituted INTO the
// chain before coverage ran, entry 0 would no longer cover the rune and
// it would fall through to entry 1 — a different TYPEFACE chosen in
// order to keep the WEIGHT. The correct answer is entry 0's own base
// face, and entry 1 is never reached.
func TestCoverageIsDecidedOnTheBaseChainNotTheStyledOne(t *testing.T) {
	faces, _ := renderedFacesAndDiags(t, styleFaceDoc(
		`[{"face": "Noto Sans Thai", "bold": "Noto Sans SC"}, "Noto Sans Thai Bold"]`,
		`{"fontFamily": "body", "fontSize": 12, "bold": true}`,
		`"ก"`))
	requireFaces(t, faces, "Noto Sans Thai")
}

// TestWrappingFollowsTheFaceActuallyUsed is AC4: the breaks are those
// of the face ACTUALLY USED, and the canvas agrees with the PDF because
// both read ONE engine measurement.
//
// The break half is stated as an existence over widths rather than at
// one hand-picked width: "the breaks are the bold face's" is exactly the
// claim that SOME declared width packs differently under the two cuts,
// and a fixed width would go vacuous the moment either face's advances
// changed. The vacuity guard is the scan finding one.
func TestWrappingFollowsTheFaceActuallyUsed(t *testing.T) {
	const chain = `[{"face": "Roboto", "bold": "Roboto Bold"}]`
	const value = `"Hamburgefonstiv Hamburgefonstiv Hamburgefonstiv"`

	paintLines := func(t *testing.T, style string, width int) int {
		t.Helper()
		source := strings.Replace(styleFaceDoc(chain, style, value), `"width": 480`, fmt.Sprintf(`"width": %d`, width), 1)
		tpl, err := ParseTemplate([]byte(source))
		if err != nil {
			t.Fatal(err)
		}
		proj, err := canvasWithTextPaint(tpl, testShippedFontSet())
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range proj.Components {
			if c.ID == "e1" && c.TextPaint != nil {
				return len(c.TextPaint.Lines)
			}
		}
		t.Fatal("precondition: element e1 projected no text paint, so the comparison below proves nothing")
		return 0
	}

	const regular = `{"fontFamily": "body", "fontSize": 12}`
	const bold = `{"fontFamily": "body", "fontSize": 12, "bold": true}`

	differedAt := -1
	for width := 120; width <= 400 && differedAt < 0; width += 4 {
		if paintLines(t, regular, width) != paintLines(t, bold, width) {
			differedAt = width
		}
	}
	if differedAt < 0 {
		t.Fatal("no declared width between 120 and 400 packed differently under Roboto and Roboto Bold — the breaks are not the face actually used")
	}

	// AND CANVAS AND PDF AGREE, at the width the breaks moved: the
	// browser never measures text, so the projected line count must equal
	// the number of distinct baselines the page model draws.
	source := strings.Replace(styleFaceDoc(chain, bold, value), `"width": 480`, fmt.Sprintf(`"width": %d`, differedAt), 1)
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	pages, _, _, _, err := buildPageModel(tpl, mustDecodeData(t, `{}`), mustDecodeParams(t), testShippedFontSet())
	if err != nil {
		t.Fatal(err)
	}
	baselines := map[int64]bool{}
	for _, p := range pages {
		for _, r := range p.Runs {
			if r.Face != "Roboto Bold" {
				t.Fatalf("the PDF path drew a bold element in %q", r.Face)
			}
			baselines[int64(r.Y+r.BaselineOffset)] = true
		}
	}
	if got, want := len(baselines), paintLines(t, bold, differedAt); got != want {
		t.Fatalf("the PDF drew %d baseline(s) where the canvas projected %d line(s) at width %d — the two are not reading one measurement", got, want, differedAt)
	}
}

// TestTheTableHeaderRowResolvesItsOwnDeclaredVariant is AC2, and it has
// its own test for a structural reason: the header arm calls
// chainFaceNames DIRECTLY (table_render.go), bypassing fontChain
// entirely. A resolution placed in fontChain alone passes every text AC
// above and silently fails this one.
//
// It also pins the cascade: headerStyle wins FOR THE HEADER ROW ALONE,
// and the body row cascades from the table's own `style`.
func TestTheTableHeaderRowResolvesItsOwnDeclaredVariant(t *testing.T) {
	doc := func(style, headerStyle string) string {
		return fmt.Sprintf(`{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "table", "x": 0, "y": 0,
         "bind": "rows[]", "headerHeight": 16,
         "style": %s,
         "headerStyle": %s,
         "columns": [{"id": "e2", "label": "Head", "bind": "{{row.v}}", "width": 200, "align": "left"}]}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": [{"face": "Roboto", "bold": "Roboto Bold", "italic": "Roboto Italic", "boldItalic": "Roboto Bold Italic"}]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "2.0"
}
`, style, headerStyle)
	}
	render := func(t *testing.T, source string) []string {
		t.Helper()
		tpl, err := ParseTemplate([]byte(source))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		pages, _, _, _, err := buildPageModel(tpl, mustDecodeData(t, `{"rows":[{"v":"Body"}]}`), mustDecodeParams(t), testShippedFontSet())
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		var faces []string
		for _, p := range pages {
			for _, r := range p.Runs {
				faces = append(faces, r.SourceText+"="+r.Face)
			}
		}
		return faces
	}

	t.Run("headerStyle.bold reaches the header row", func(t *testing.T) {
		got := render(t, doc(`{"fontFamily": "body", "fontSize": 10}`, `{"bold": true}`))
		want := []string{"Head=Roboto Bold", "Body=Roboto"}
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Fatalf("faces = %v, want %v — headerStyle governs the header row ALONE", got, want)
		}
	})

	t.Run("headerStyle wins over the table's own style for the header row", func(t *testing.T) {
		got := render(t, doc(`{"fontFamily": "body", "fontSize": 10, "bold": true}`, `{"bold": false}`))
		want := []string{"Head=Roboto", "Body=Roboto Bold"}
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Fatalf("faces = %v, want %v — headerStyle.bold wins for the HEADER ROW ALONE, and the body still cascades from style", got, want)
		}
	})

	t.Run("the cascade is per FIELD, not per block", func(t *testing.T) {
		// headerStyle declares only `bold`; `italic` falls through to the
		// table's own style, so the HEADER asks for bold AND italic while
		// the body asks for italic alone. A block-granular cascade would
		// drop style.italic for the header and draw it Roboto Bold.
		got := render(t, doc(`{"fontFamily": "body", "fontSize": 10, "italic": true}`, `{"bold": true}`))
		want := []string{"Head=Roboto Bold Italic", "Body=Roboto Italic"}
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Fatalf("faces = %v, want %v — each field cascades on its own", got, want)
		}
	})

	// ⚠ NO EXPLICIT-NULL CASE, AND THAT IS MEASURED RATHER THAN OMITTED.
	// The two arms are spelled `.Set && !.Null` to match their nine
	// siblings, but `style.bold` has NO presentNull path: parse_bands.go
	// decodes the key with decodeBoolRaw, and encoding/json reads a JSON
	// null into a bool as `false` with no error — so `"bold": null` and
	// `"bold": false` are the same parsed value and no test can tell the
	// two arms apart. The `!Null` half is carried anyway, because the
	// `color` arm in this same resolver records what its absence costs
	// the day a field DOES gain a null path.

	t.Run("the table cascades to the body with no headerStyle at all", func(t *testing.T) {
		got := render(t, doc(`{"fontFamily": "body", "fontSize": 10, "bold": true}`, `{}`))
		want := []string{"Head=Roboto Bold", "Body=Roboto Bold"}
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Fatalf("faces = %v, want %v", got, want)
		}
	})
}

// TestTheTableWarningCoalescesAcrossRowsNotPerCell: AC3's Warning is one
// per (element, distinct rune), and a table shapes a column ONCE PER
// ROW. Without a memo that outlives the shapeSegments call, a
// five-hundred-row bold table reports the same column's same rune five
// hundred times — one per cell, which is not the rule the spec states.
//
// The row count is deliberately larger than the distinct-rune count, so
// a per-cell implementation cannot produce the expected total by
// coincidence.
func TestTheTableWarningCoalescesAcrossRowsNotPerCell(t *testing.T) {
	const rowCount = 6
	rows := make([]string, rowCount)
	for i := range rows {
		rows[i] = `{"v":"AB"}`
	}
	source := `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "rows[]", "headerHeight": 16,
       "style": {"fontFamily": "body", "fontSize": 9, "bold": true},
       "columns": [{"id": "e2", "label": "H", "bind": "{{row.v}}", "width": 200}]}
    ]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "2.0"
}`
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, _, _, diags, err := buildPageModel(tpl, mustDecodeData(t, `{"rows":[`+strings.Join(rows, ",")+`]}`), mustDecodeParams(t), testShippedFontSet())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	byElement := map[string]int{}
	for _, d := range styleFaceWarnings(diags) {
		byElement[d.ElementID]++
	}
	// The column is ONE element and its cells carry two distinct runes,
	// so it owes exactly two Warnings however many rows it has. Its
	// header label "H" is a third, from the same element id — the label
	// is shaped once and is a distinct rune of its own.
	if got := byElement["e2"]; got != 3 {
		t.Fatalf("the column reported %d Warnings over %d rows of \"AB\" plus the label \"H\", want 3 (one per distinct rune of that element, never one per cell): %+v", got, rowCount, diags)
	}
}

// TestADocumentDeclaringNeitherBoldNorItalicIsUnchanged is AC5 at the
// engine: the same document, rendered before and after this story's
// resolution exists, produces the same faces and no new diagnostic. The
// byte-level half of AC5 lives in internal/template (serialization) and
// in the recorded golden digests.
func TestADocumentDeclaringNeitherBoldNorItalicIsUnchanged(t *testing.T) {
	faces, diags := renderedFacesAndDiags(t,
		styleFaceDoc(`["Noto Sans", "Noto Sans Thai"]`, `{"fontFamily": "body", "fontSize": 12}`, `"Aก"`))
	requireFaces(t, faces, "Noto Sans", "Noto Sans Thai")
	if len(diags) != 0 {
		t.Fatalf("a document declaring no style produced diagnostics: %+v", diags)
	}
}

// TestBoldIsOffWhenItIsAbsentNullOrFalse pins the three-state reading.
// Off is ABSENT (the designer never writes `bold: false`), but a
// hand-authored `false` and an explicit `null` mean the same thing and
// must not request a variant — a request that resolves is not the same
// as no request, because only the former can warn.
func TestBoldIsOffWhenItIsAbsentNullOrFalse(t *testing.T) {
	for _, style := range []string{
		`{"fontFamily": "body", "fontSize": 12}`,
		`{"fontFamily": "body", "fontSize": 12, "bold": false, "italic": false}`,
		`{"fontFamily": "body", "fontSize": 12, "bold": null, "italic": null}`,
	} {
		faces, diags := renderedFacesAndDiags(t, styleFaceDoc(`["Roboto"]`, style, `"A"`))
		requireFaces(t, faces, "Roboto")
		if len(styleFaceWarnings(diags)) != 0 {
			t.Fatalf("style %s requested a variant: %+v", style, diags)
		}
	}
}
