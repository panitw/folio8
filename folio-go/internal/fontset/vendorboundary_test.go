package fontset_test

import (
	"encoding/binary"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boxesandglue/textshape/ot"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
	"github.com/panitw/folio8/folio-go/internal/fontset"
	"github.com/panitw/folio8/folio-go/internal/geom"
)

// This file is the EXECUTABLE form of internal/fontset/vendor-boundary.md
// (Story 2.3a, AC8). Its job is that the document cannot drift away from
// the artifact it describes: every load-bearing claim there is DERIVED
// from the vendor here rather than restated (D-000.14), and a future
// textshape that changes a default fails these tests instead of silently
// falsifying a table.
//
// Every assertion names its subject in its failure message (D-000.26):
// the face, the accessor, the exact table removed.

// vendorBoundaryDoc is the enumeration this file makes claims about,
// relative to this package's directory. It is named as a constant so
// that a test asserting properties of a document which has been renamed
// or deleted FAILS rather than passes (AC8.5).
const vendorBoundaryDoc = "vendor-boundary.md"

// textshapeVersion is the single dependency version the enumeration is a
// claim about. Asserted against go.mod below, so the document's
// prominent version statement is checkable rather than decorative.
const textshapeVersion = "v0.0.15"

// robotoPath is the audit's subject throughout: unitsPerEm 2048, 1,294
// glyphs, OS/2 version 4 with a real sCapHeight of 1456.
func robotoPath() string { return filepath.Join("..", "..", "testdata", "fonts", "Roboto-Regular.ttf") }

func robotoBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(robotoPath())
	if err != nil {
		t.Fatalf("read the audit's subject face %s: %v", robotoPath(), err)
	}
	if len(data) == 0 {
		t.Fatalf("presence precondition: the audit's subject face %s is empty", robotoPath())
	}
	return data
}

// stripTableRecord removes exactly one 16-byte record from the table
// directory and decrements numTables. Every surviving record keeps its
// ORIGINAL ABSOLUTE OFFSET: the records after the removed one move back
// 16 bytes within the directory, the 16 bytes vacated at the directory's
// end are zeroed, and the file's length is unchanged. The stripped
// table's own bytes stay where they were, unreferenced.
//
// The shape matters and is not interchangeable with a truncation. An
// earlier version shifted the file's tail back by 16 bytes instead;
// every surviving offset then pointed 16 bytes past its table, and the
// face reported 14 glyphs for EVERY mutation — a number that looks like
// a measurement and is an artifact of the mutation. Recorded because the
// wrong helper produced PLAUSIBLE output, which is the failure mode this
// whole file is about.
func stripTableRecord(t *testing.T, src []byte, tag string) []byte {
	t.Helper()
	out := make([]byte, len(src))
	copy(out, src)
	numTables := int(binary.BigEndian.Uint16(out[4:6]))
	for i := 0; i < numTables; i++ {
		off := 12 + i*16
		if string(out[off:off+4]) != tag {
			continue
		}
		copy(out[off:12+(numTables-1)*16], out[off+16:12+numTables*16])
		for j := 12 + (numTables-1)*16; j < 12+numTables*16; j++ {
			out[j] = 0
		}
		binary.BigEndian.PutUint16(out[4:6], uint16(numTables-1))
		return out
	}
	t.Fatalf("the subject face %s carries no %q table record to remove, so no assertion about its absence can be made", robotoPath(), tag)
	return nil
}

// parseStripped strips one table and returns the re-parsed font AND face.
// It asserts the stripped font STILL PARSES first (AC8.1): a strip that
// produced an unparseable font would make every row below pass
// vacuously, for the wrong reason.
func parseStripped(t *testing.T, tag string) (*ot.Font, *ot.Face) {
	t.Helper()
	stripped := stripTableRecord(t, robotoBytes(t), tag)
	font, err := ot.ParseFont(stripped, 0)
	if err != nil {
		t.Fatalf("presence precondition: %s with its %q record removed no longer parses (%v) — every claim about what the vendor RETURNS for an absent %q would then be a claim about an unparseable font", robotoPath(), tag, err, tag)
	}
	if font.HasTable(tagFor(t, tag)) {
		t.Fatalf("presence precondition: %s still reports HasTable(%q) after that record was removed — the strip did not take effect, so a substituted value could not be distinguished from a real one", robotoPath(), tag)
	}
	face, ferr := ot.NewFace(font)
	if ferr != nil {
		t.Fatalf("ot.NewFace on %s with %q removed: %v", robotoPath(), tag, ferr)
	}
	return font, face
}

func tagFor(t *testing.T, tag string) ot.Tag {
	t.Helper()
	if len(tag) != 4 {
		t.Fatalf("table tag %q is not four bytes", tag)
	}
	return ot.MakeTag(tag[0], tag[1], tag[2], tag[3])
}

// TestVendorSubstitutesTheExactDefaultsTheEnumerationNames is AC8.1.
// For each table the enumeration covers, it asserts the vendor accessor
// returns the EXACT substituted value Table 1 names — not merely "some
// default". If a future textshape changes one, this fails and the
// document is corrected from the artifact rather than from memory.
func TestVendorSubstitutesTheExactDefaultsTheEnumerationNames(t *testing.T) {
	// First, the intact readings the substitutions are measured against.
	// Without these, "CapHeight() == 1900 when OS/2 is absent" could not
	// be distinguished from "CapHeight() is 1900 for this face anyway".
	intactBytes := robotoBytes(t)
	intactFont, perr := ot.ParseFont(intactBytes, 0)
	if perr != nil {
		t.Fatalf("parse the intact subject face %s: %v", robotoPath(), perr)
	}
	intact, ferr := ot.NewFace(intactFont)
	if ferr != nil {
		t.Fatalf("ot.NewFace on the intact subject face %s: %v", robotoPath(), ferr)
	}
	const (
		wantUpem      = 2048
		wantGlyphs    = 1294
		wantPSName    = "Roboto-Regular"
		wantAscender  = 1900
		wantDescender = -500
		wantCapHeight = 1456
		// Story 2.3a Finding 6 (Nit). The intact advance for gid 36, so
		// that "HorizontalAdvance(36) == Upem() when hhea/hmtx is
		// absent" is a claim about a SUBSTITUTION rather than a claim
		// that happens to hold for this face anyway. Deliberately
		// different from wantUpem (2048); the assertion below pins that
		// difference so a future subject face or gid whose real advance
		// equalled upem cannot make those two rows self-satisfying.
		wantAdvance36 = 1839
	)
	if got := int(intact.Upem()); got != wantUpem {
		t.Fatalf("intact %s: Upem() = %d, want %d — the audit's subject is not the face this file was written against", robotoPath(), got, wantUpem)
	}
	if got := intactFont.NumGlyphs(); got != wantGlyphs {
		t.Fatalf("intact %s: NumGlyphs() = %d, want %d", robotoPath(), got, wantGlyphs)
	}
	if got := intact.PostscriptName(); got != wantPSName {
		t.Fatalf("intact %s: PostscriptName() = %q, want %q", robotoPath(), got, wantPSName)
	}
	if got := int(intact.Ascender()); got != wantAscender {
		t.Fatalf("intact %s: Ascender() = %d, want %d", robotoPath(), got, wantAscender)
	}
	if got := int(intact.Descender()); got != wantDescender {
		t.Fatalf("intact %s: Descender() = %d, want %d", robotoPath(), got, wantDescender)
	}
	if got := int(intact.CapHeight()); got != wantCapHeight {
		t.Fatalf("intact %s: CapHeight() = %d, want %d", robotoPath(), got, wantCapHeight)
	}
	// Read ONCE and reuse. The read is a float-typed value expression and
	// therefore a sanctioned entry in lint's no-float-typed-value
	// test-scope inventory; taking it twice would book two entries for
	// one fact.
	intactAdvance36 := int64(intact.HorizontalAdvance(36))
	if intactAdvance36 != wantAdvance36 {
		t.Fatalf("intact %s: HorizontalAdvance(36) = %d, want %d", robotoPath(), intactAdvance36, wantAdvance36)
	}
	// The discrimination itself, asserted rather than assumed: the two
	// operands of the hhea/hmtx rows below must DIFFER on the intact
	// face, or those rows would pass without the vendor substituting
	// anything.
	if intactAdvance36 == int64(intact.Upem()) {
		t.Fatalf("intact %s: HorizontalAdvance(36) == Upem() == %d — the hhea/hmtx subtests below assert that an ABSENT table makes the advance equal upem, and that assertion cannot discriminate on a face where it is already true", robotoPath(), int64(intact.Upem()))
	}

	t.Run("head absent substitutes upem 1000 and a fabricated bounding box", func(t *testing.T) {
		_, face := parseStripped(t, "head")
		if got := int(face.Upem()); got != 1000 {
			t.Errorf("%s with %q removed: Upem() = %d, want the vendor's substituted 1000 (\"Default for CFF\", ot/metrics.go); the real value is %d", robotoPath(), "head", got, wantUpem)
		}
		xMin, yMin, xMax, yMax := face.BBox()
		if int(xMin) != 0 || int(yMin) != -200 || int(xMax) != 1000 || int(yMax) != 800 {
			t.Errorf("%s with %q removed: BBox() = (%d,%d,%d,%d), want the vendor's substituted (0,-200,1000,800)", robotoPath(), "head", xMin, yMin, xMax, yMax)
		}
	})

	t.Run("name absent substitutes the literal string Unknown", func(t *testing.T) {
		_, face := parseStripped(t, "name")
		if got := face.PostscriptName(); got != "Unknown" {
			t.Errorf("%s with %q removed: PostscriptName() = %q, want the vendor's substituted %q — a well-formed string every downstream assertion accepts", robotoPath(), "name", got, "Unknown")
		}
	})

	t.Run("OS/2 absent substitutes the ascender for the cap height", func(t *testing.T) {
		_, face := parseStripped(t, "OS/2")
		if got, want := int(face.CapHeight()), int(face.Ascender()); got != want {
			t.Errorf("%s with %q removed: CapHeight() = %d, want it to equal Ascender() = %d — the substitution this audit exists for", robotoPath(), "OS/2", got, want)
		}
		if got := int(face.CapHeight()); got != wantAscender {
			t.Errorf("%s with %q removed: CapHeight() = %d, want the intact ascender %d", robotoPath(), "OS/2", got, wantAscender)
		}
	})

	t.Run("hhea absent substitutes 800 and -200 and the upem advance", func(t *testing.T) {
		_, face := parseStripped(t, "hhea")
		if got := int(face.Ascender()); got != 800 {
			t.Errorf("%s with %q removed: Ascender() = %d, want the vendor's substituted 800", robotoPath(), "hhea", got)
		}
		if got := int(face.Descender()); got != -200 {
			t.Errorf("%s with %q removed: Descender() = %d, want the vendor's substituted -200", robotoPath(), "hhea", got)
		}
		if got, want := int64(face.HorizontalAdvance(36)), int64(face.Upem()); got != want {
			t.Errorf("%s with %q removed: HorizontalAdvance(36) = %d, want the vendor's substituted upem %d", robotoPath(), "hhea", got, want)
		}
	})

	t.Run("hmtx absent substitutes the upem advance", func(t *testing.T) {
		_, face := parseStripped(t, "hmtx")
		if got, want := int64(face.HorizontalAdvance(36)), int64(face.Upem()); got != want {
			t.Errorf("%s with %q removed: HorizontalAdvance(36) = %d, want the vendor's substituted upem %d", robotoPath(), "hmtx", got, want)
		}
	})

	t.Run("maxp absent or short substitutes a glyph count of zero", func(t *testing.T) {
		stripped := stripTableRecord(t, robotoBytes(t), "maxp")
		font, err := ot.ParseFont(stripped, 0)
		if err != nil {
			t.Fatalf("presence precondition: %s with %q removed no longer parses: %v", robotoPath(), "maxp", err)
		}
		if got := font.NumGlyphs(); got != 0 {
			t.Errorf("%s with %q removed: NumGlyphs() = %d, want the vendor's substituted 0 — the value that produces a negative slice length downstream", robotoPath(), "maxp", got)
		}
	})

	t.Run("cmap absent is reported honestly", func(t *testing.T) {
		_, face := parseStripped(t, "cmap")
		if face.Cmap() != nil {
			t.Errorf("%s with %q removed: Cmap() returned non-nil, want nil — this row is classified HONEST, and folio8's decision not to require the table rests on that classification", robotoPath(), "cmap")
		}
	})
}

// TestVendorFabricatesForAnOutOfDomainGlyphID is the enumeration's third
// state — fabricated, distinct from substituted. GetAdvanceWidth does
// not default for an out-of-range glyph id; it returns lastAdvanceWidth,
// a real number belonging to a different glyph. This is why the integer
// path folio8 now uses bounds-checks every id before handing it over.
func TestVendorFabricatesForAnOutOfDomainGlyphID(t *testing.T) {
	data := robotoBytes(t)
	font, err := ot.ParseFont(data, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", robotoPath(), err)
	}
	numGlyphs := font.NumGlyphs()
	if numGlyphs <= 0 {
		t.Fatalf("presence precondition: %s reports %d glyphs, so there is no out-of-range id to ask about", robotoPath(), numGlyphs)
	}
	hmtx, herr := ot.ParseHmtxFromFont(font)
	if herr != nil {
		t.Fatalf("ot.ParseHmtxFromFont on %s: %v", robotoPath(), herr)
	}

	const outOfRange = 65535
	if outOfRange < numGlyphs {
		t.Fatalf("presence precondition: glyph id %d is INSIDE this %d-glyph face, so asking for it proves nothing about out-of-domain behaviour", outOfRange, numGlyphs)
	}
	got := hmtx.GetAdvanceWidth(outOfRange)
	if got == 0 {
		t.Fatalf("%s: GetAdvanceWidth(%d) on a %d-glyph face returned 0 — the enumeration classifies this row FABRICATED because it returns a plausible NON-zero advance; if the vendor now reports absence, reclassify the row in %s", robotoPath(), outOfRange, numGlyphs, vendorBoundaryDoc)
	}
	last := hmtx.GetAdvanceWidth(ot.GlyphID(numGlyphs - 1))
	if got != last {
		t.Errorf("%s: GetAdvanceWidth(%d) = %d, want it to equal the last in-range glyph's advance %d — the enumeration names lastAdvanceWidth as the fabricated value", robotoPath(), outOfRange, got, last)
	}
}

// TestOtNewFaceHasNoFailureMode derives Table 3: ot.NewFace returned
// err == nil for every table-removal case measured, INCLUDING the one
// that panics. That is the evidence for labelling fontset.New's
// ot.NewFace error branch UNREACHABLE rather than counting it as
// coverage (D-000.24).
//
// maxp is deliberately excluded from the loop and asserted separately
// below, because it does not return at all.
func TestOtNewFaceHasNoFailureMode(t *testing.T) {
	for _, tag := range []string{"head", "hhea", "hmtx", "name", "OS/2", "cmap"} {
		stripped := stripTableRecord(t, robotoBytes(t), tag)
		font, err := ot.ParseFont(stripped, 0)
		if err != nil {
			t.Fatalf("presence precondition: %s with %q removed no longer parses: %v", robotoPath(), tag, err)
		}
		if _, ferr := ot.NewFace(font); ferr != nil {
			t.Errorf("ot.NewFace on %s with %q removed returned %v — the enumeration records that it has NO failure mode, and fontset.New's error branch is labelled unreachable on that basis. If this fires, that label is stale and must be corrected, not deleted", robotoPath(), tag, ferr)
		}
	}
}

// TestVendorPanicsOnAMissingMaxpWithoutFolio8Guard is the red-proof that
// D-2.3a.1's guard is LOAD-BEARING rather than defensive decoration. It
// asserts the hazard directly at the vendor: ot.NewFace, handed a font
// whose maxp record has been removed, PANICS.
//
// Mechanism: NumGlyphs() returns 0, hhea's numberOfHMetrics is 1294, and
// ot.ParseHmtx reaches make([]int16, 0-1294). Measured at 431a6a5 the
// same panic came out of folio8.Render itself, through
// fontset.New -> ot.NewFace, on caller-supplied FontSet bytes. That path
// is closed — TestFolio8DeclinesEverySubstitutionAtIngestion asserts the
// located error folio8 returns instead — so this test asserts the hazard
// at the only layer where it still exists. If textshape ever fixes it,
// this test fails and folio8's guard becomes belt-and-braces rather than
// load-bearing; say so then, do not delete the guard.
func TestVendorPanicsOnAMissingMaxpWithoutFolio8Guard(t *testing.T) {
	stripped := stripTableRecord(t, robotoBytes(t), "maxp")
	font, err := ot.ParseFont(stripped, 0)
	if err != nil {
		t.Fatalf("presence precondition: %s with %q removed no longer parses: %v", robotoPath(), "maxp", err)
	}
	if got := font.NumGlyphs(); got != 0 {
		t.Fatalf("presence precondition: %s with %q removed reports %d glyphs, not 0 — the panic below depends on the zero", robotoPath(), "maxp", got)
	}

	panicked := func() (p bool) {
		defer func() {
			if r := recover(); r != nil {
				p = true
			}
		}()
		_, _ = ot.NewFace(font)
		return false
	}()

	if !panicked {
		t.Fatalf("ot.NewFace on %s with %q removed did NOT panic. The enumeration records this as the loud instance of the class D-2.3a.1 closed; if the vendor no longer panics here, correct %s from the artifact", robotoPath(), "maxp", vendorBoundaryDoc)
	}
}

// TestFolio8DeclinesEverySubstitutionAtIngestion is AC8.2 and D-2.3a.1's
// live guard: loading each stripped font through fontset.New yields the
// outcome Table 2 records, and the assertion is on the ERROR'S TEXT —
// that it names the font and the table — not merely that an error
// occurred. An error that fires for an unrelated reason would satisfy
// "err != nil" perfectly.
func TestFolio8DeclinesEverySubstitutionAtIngestion(t *testing.T) {
	const faceName = "Roboto-Regular"

	// The intact face must load, or every "stripping X makes it fail"
	// row below is satisfied by a face that never loaded in the first
	// place.
	if _, err := fontset.New(faceName, robotoBytes(t)); err != nil {
		t.Fatalf("presence precondition: the intact subject face %s does not load through fontset.New (%v), so no claim below distinguishes a missing table from a broken face", robotoPath(), err)
	}

	for _, tag := range []string{"head", "maxp", "hhea", "hmtx", "OS/2"} {
		stripped := stripTableRecord(t, robotoBytes(t), tag)
		_, err := fontset.New(faceName, stripped)
		if err == nil {
			t.Errorf("fontset.New on %s with %q removed returned no error — a table folio8 reads was absent and the vendor's substituted value was accepted (D-2.3a.1)", robotoPath(), tag)
			continue
		}
		msg := err.Error()
		if !strings.Contains(msg, faceName) {
			t.Errorf("fontset.New on %s with %q removed: error %q does not name the face %q — a diagnostic that does not say WHICH face of a FontSet failed is not located", robotoPath(), tag, msg, faceName)
		}
		if !strings.Contains(msg, tag) {
			t.Errorf("fontset.New on %s with %q removed: error %q does not name the missing table — D-2.3a.1 requires a located error naming the face AND the table", robotoPath(), tag, msg)
		}
	}

	// The two ruled tolerations. These are NOT oversights: each is a
	// disposition ruled elsewhere, and asserting them here is what stops
	// a later tightening reversing a ruling silently.
	t.Run("name is tolerated, because Story 2.2 ruled a nameless program loadable", func(t *testing.T) {
		stripped := stripTableRecord(t, robotoBytes(t), "name")
		f, err := fontset.New(faceName, stripped)
		if err != nil {
			t.Fatalf("fontset.New on %s with %q removed returned %v — D-2.3a.1 says verbatim \"Do not require `name`\"; requiring it reverses Story 2.2's ruled disposition", robotoPath(), "name", err)
		}
		if got := f.PostScriptName(); got != "" {
			t.Errorf("fontset.New on %s with %q removed: PostScriptName() = %q, want \"\" — observably absent, never the vendor's plausible %q", robotoPath(), "name", got, "Unknown")
		}
	})

	t.Run("cmap is tolerated, because requiring it would break the fallback chain", func(t *testing.T) {
		stripped := stripTableRecord(t, robotoBytes(t), "cmap")
		f, err := fontset.New(faceName, stripped)
		if err != nil {
			t.Fatalf("fontset.New on %s with %q removed returned %v — the vendor reports a missing cmap HONESTLY (Cmap() == nil) and folio8 nil-checks it; requiring the table would turn a face the fallback chain can skip into a load error that makes the whole FontSet unloadable", robotoPath(), "cmap", err)
		}
		if f.HasGlyph('H') {
			t.Errorf("fontset.New on %s with %q removed: HasGlyph('H') = true, want false — the chain's skip depends on this being observably false, not on an error", robotoPath(), "cmap")
		}
	})

	// Story 2.3a Finding 5 (Nit). The subtest above pins the TOLERATION;
	// this one pins the CONSEQUENCE that makes the toleration
	// defensible. The deviation from D-2.3a.1's literal text ("every
	// table folio8 actually reads", and cmap IS read) is justified by a
	// CHAIN-LEVEL claim — a chain whose first face carries no cmap falls
	// through to the next face and still renders — and a single-face
	// assertion cannot measure that claim. Without this, the toleration
	// was pinned and its justification was only narrated.
	t.Run("a chain whose first face has no cmap falls through and still renders", func(t *testing.T) {
		const fallbackName = "Noto Sans"
		tmplPath := filepath.Join("..", "..", "..", "fixtures", "font-text", "input.folio")
		raw, rerr := os.ReadFile(tmplPath)
		if rerr != nil {
			t.Fatalf("read the template this assertion renders, %s: %v", tmplPath, rerr)
		}

		// Build the two-face chain by widening the fixture's own
		// single-face chain, so the document under test differs from the
		// committed fixture in exactly one respect.
		const singleChain = `["Roboto-Regular"]`
		twoChain := `["` + faceName + `","` + fallbackName + `"]`
		if !strings.Contains(string(raw), singleChain) {
			t.Fatalf("presence precondition: %s no longer declares the single-face chain %s, so this test cannot widen it to a two-face chain", tmplPath, singleChain)
		}
		twoFaceRaw := []byte(strings.Replace(string(raw), singleChain, twoChain, 1))

		tmpl, terr := folio8.ParseTemplate(twoFaceRaw)
		if terr != nil {
			t.Fatalf("parse the two-face variant of %s: %v", tmplPath, terr)
		}

		fallbackBytes := fonts.Shipped()[fallbackName]
		if len(fallbackBytes) == 0 {
			t.Fatalf("presence precondition: fonts.Shipped() carries no bytes for the fallback face %q, so a fall-through could not be distinguished from an empty chain", fallbackName)
		}
		strippedFirst := stripTableRecord(t, robotoBytes(t), "cmap")
		if len(strippedFirst) == 0 {
			t.Fatalf("presence precondition: stripping %q from %s produced no bytes", "cmap", robotoPath())
		}

		// Anti-vacuity: the FIRST face must genuinely be unable to serve
		// the glyph, or a successful render proves nothing about
		// falling through.
		first, ferr := fontset.New(faceName, strippedFirst)
		if ferr != nil {
			t.Fatalf("the cmap-stripped first face must still LOAD for this to be a fall-through rather than a load failure: %v", ferr)
		}
		if first.HasGlyph('H') {
			t.Fatalf("anti-vacuity: the cmap-stripped first face reports HasGlyph('H') = true, so the render below would not exercise a fall-through at all")
		}

		outRes, err := folio8.Render(tmpl, folio8.Data("{}"), folio8.Params("{}"),
			folio8.FontSet{faceName: strippedFirst, fallbackName: fallbackBytes})
		if err != nil {
			t.Fatalf("rendering a chain [%s, %s] whose FIRST face has no cmap returned %v — requiring cmap in requiredTables would convert this rendering document into an unloadable FontSet, reversing Story 2.2's ruled fall-through rather than tightening it", faceName, fallbackName, err)
		}
		out := outRes.Bytes
		if len(out) == 0 {
			t.Fatalf("rendering the two-face chain returned no error but produced 0 bytes — a document that renders must produce bytes")
		}

		// The contrast that gives the number meaning: a required table
		// missing on the same first face makes the whole FontSet
		// unloadable, 0 bytes. cmap does not. Both operands measured
		// here, neither quoted.
		osStripped := stripTableRecord(t, robotoBytes(t), "OS/2")
		badRes, berr := folio8.Render(tmpl, folio8.Data("{}"), folio8.Params("{}"),
			folio8.FontSet{faceName: osStripped, fallbackName: fallbackBytes})
		bad := badRes.Bytes
		if berr == nil {
			t.Fatalf("control: the same chain with a REQUIRED table (OS/2) missing on the first face rendered %d bytes without error, so this test cannot distinguish a tolerated table from a required one", len(bad))
		}
		if len(bad) != 0 {
			t.Errorf("control: the refused render produced %d bytes, want 0", len(bad))
		}
		t.Logf("cmap-stripped first face: chain renders %d bytes, err=nil; OS/2-stripped first face: %d bytes, err=%v", len(out), len(bad), berr)
	})
}

// TestCapHeightSubstitutionIsAssertedOnTheEmittedBytes is AC8.3, in the
// form D-2.3a.1 left available.
//
// THE LIVE HALF, asserted on the produced PDF: an intact face emits
// /CapHeight 711, and 711 is what folio8's own scaling function produces
// from the face's real sCapHeight. Presence is asserted before value
// (D-000.21 sharpened) — the PDF must carry a /CapHeight entry at all
// before the two readings can be compared.
//
// THE CLOSED HALF, and why it is no longer an emitted-byte assertion.
// Measured at 431a6a5, in an isolated worktree at that commit: rendering
// fixtures/font-text/input.folio with an OS/2-stripped Roboto produced
// 22,198 bytes, err == nil, and the bytes /CapHeight 928 — the ascender,
// not a cap height, in a document reporting success. The intact render
// at that same commit produced 22,310 bytes and /CapHeight 711.
// D-2.3a.1 closed that path: the face is now refused at ingestion, so
// THERE IS NO LONGER A PDF CARRYING 928 TO ASSERT ON. Rather than
// manufacture one, this test asserts the arithmetic that produced the
// byte — the vendor's substituted CapHeight, put through folio8's one
// scaling function — and asserts the render is refused. The 928 is
// derived here, not quoted.
func TestCapHeightSubstitutionIsAssertedOnTheEmittedBytes(t *testing.T) {
	const faceName = "Roboto-Regular"
	tmplPath := filepath.Join("..", "..", "..", "fixtures", "font-text", "input.folio")
	raw, rerr := os.ReadFile(tmplPath)
	if rerr != nil {
		t.Fatalf("read the template this assertion renders, %s: %v", tmplPath, rerr)
	}
	tmpl, terr := folio8.ParseTemplate(raw)
	if terr != nil {
		t.Fatalf("parse %s: %v", tmplPath, terr)
	}

	outRes, err := folio8.Render(tmpl, folio8.Data("{}"), folio8.Params("{}"), folio8.FontSet{faceName: robotoBytes(t)})
	if err != nil {
		t.Fatalf("render %s with the intact subject face %s: %v", tmplPath, robotoPath(), err)
	}
	out := outRes.Bytes

	// Presence precondition: the property must live in this artifact.
	const capKey = "/CapHeight "
	if !strings.Contains(string(out), capKey) {
		t.Fatalf("presence precondition: the PDF rendered from %s carries no %q entry at all, so an assertion about its VALUE would pass vacuously on a document that never declared one", tmplPath, capKey)
	}

	// The intact value, derived rather than quoted: the face's real
	// sCapHeight through folio8's one scaling function.
	intactFont, perr := ot.ParseFont(robotoBytes(t), 0)
	if perr != nil {
		t.Fatalf("parse %s: %v", robotoPath(), perr)
	}
	intactFace, ferr := ot.NewFace(intactFont)
	if ferr != nil {
		t.Fatalf("ot.NewFace on %s: %v", robotoPath(), ferr)
	}
	upem := int64(intactFace.Upem())
	wantIntact := int64(geom.ScaleRound(geom.Length(int64(intactFace.CapHeight())), 1000, upem))
	if !strings.Contains(string(out), capKey+itoa(wantIntact)+" ") {
		t.Errorf("the PDF rendered from %s with the intact %s does not contain %q — the face's real cap height %d scaled to the PDF's 1000-unit em against unitsPerEm %d", tmplPath, robotoPath(), capKey+itoa(wantIntact), intactFace.CapHeight(), upem)
	}

	// The substituted value, derived the same way from the same
	// scaling function, so the two readings are commensurable.
	_, strippedFace := parseStripped(t, "OS/2")
	wantSubstituted := int64(geom.ScaleRound(geom.Length(int64(strippedFace.CapHeight())), 1000, upem))
	if wantSubstituted == wantIntact {
		t.Fatalf("the substituted cap height scales to %d, the same value the real one does — the two readings are indistinguishable in the PDF's units, so this row records no hazard and %s must be corrected", wantSubstituted, vendorBoundaryDoc)
	}

	// And the path that would have emitted it is refused.
	strippedBytes := stripTableRecord(t, robotoBytes(t), "OS/2")
	if _, serr := folio8.Render(tmpl, folio8.Data("{}"), folio8.Params("{}"), folio8.FontSet{faceName: strippedBytes}); serr == nil {
		t.Fatalf("rendering %s with an OS/2-stripped %s SUCCEEDED. At 431a6a5 that produced /CapHeight %d — the ascender — in a document reporting success. D-2.3a.1 closed it; if it renders again, the guard is gone", tmplPath, robotoPath(), wantSubstituted)
	}
}

// TestNamelessProgramNamesItsBaseFontSubstitutionInThePDF is D-2.3a.2's
// live guard. A nameless program still renders and /BaseFont still
// carries the FontSet key — both ruled — but the PDF now SAYS SO.
//
// The property being guarded is precisely "a reader of the PDF can tell
// whether /BaseFont reflects the embedded program or reflects us". Under
// the old silent fallback the nameless render was byte-identical in
// length to the intact one (22,310 bytes at 431a6a5, measured), which is
// the definition of indistinguishable.
func TestNamelessProgramNamesItsBaseFontSubstitutionInThePDF(t *testing.T) {
	const faceName = "Roboto-Regular"
	tmplPath := filepath.Join("..", "..", "..", "fixtures", "font-text", "input.folio")
	raw, rerr := os.ReadFile(tmplPath)
	if rerr != nil {
		t.Fatalf("read %s: %v", tmplPath, rerr)
	}
	tmpl, terr := folio8.ParseTemplate(raw)
	if terr != nil {
		t.Fatalf("parse %s: %v", tmplPath, terr)
	}

	intactRes, ierr := folio8.Render(tmpl, folio8.Data("{}"), folio8.Params("{}"), folio8.FontSet{faceName: robotoBytes(t)})
	if ierr != nil {
		t.Fatalf("render %s with the intact %s: %v", tmplPath, robotoPath(), ierr)
	}
	intact := intactRes.Bytes

	stripped := stripTableRecord(t, robotoBytes(t), "name")
	namelessRes, nerr := folio8.Render(tmpl, folio8.Data("{}"), folio8.Params("{}"), folio8.FontSet{faceName: stripped})
	if nerr != nil {
		t.Fatalf("render %s with a name-stripped %s: %v — a nameless program is ruled LOADABLE (Story 2.2, D-2.3a.1)", tmplPath, robotoPath(), nerr)
	}
	nameless := namelessRes.Bytes

	// The diagnostic marker, spelled here independently of the
	// production code that writes it.
	const marker = "% folio8: /BaseFont "
	if strings.Contains(string(intact), marker) {
		t.Errorf("the INTACT render of %s contains %q — the diagnostic must fire only for a program that declares no name record, or it is noise rather than a signal (and it would move every golden)", tmplPath, marker)
	}
	if !strings.Contains(string(nameless), marker) {
		t.Fatalf("the render of %s with a name-stripped %s does not contain %q — the substitution is silent again, and a nameless program is once more indistinguishable from a correctly-named one (D-2.3a.2)", tmplPath, robotoPath(), marker)
	}

	// It is a diagnostic, not a rejection, and /BaseFont still carries
	// the key: both halves of the ruling, asserted.
	if !strings.Contains(string(nameless), "/BaseFont /") {
		t.Errorf("the nameless render of %s carries no /BaseFont name object — /BaseFont is Required by ISO 32000-1 Table 117 and something legal must go there", tmplPath)
	}

	// And the two renders must now be distinguishable, which is the
	// whole property. At 431a6a5 they were the same length.
	if len(intact) == len(nameless) {
		t.Errorf("the intact and nameless renders of %s are both %d bytes — that equality WAS the defect (measured at 431a6a5: 22,310 bytes each)", tmplPath, len(intact))
	}
}

// TestFamilyNameHasNoCallSite is AC8.4: a LIVE GUARD, not a comment. The
// enumeration classifies (*ot.Name).FamilyName as substituted-and-never-
// called; the day someone adds a call, that row stops being true and the
// build says so.
//
// CORRECTION, Story 2.3a Finding 7 (Nit). This guard used to grep raw
// source for the text ".FamilyName(" and exclude any path ending
// "vendorboundary_test.go", because this file contains that needle in
// its own string constant. Two consequences, both now gone: a COMMENT
// anywhere in the module mentioning .FamilyName( failed the build though
// no call existed, and a genuine call added INSIDE this file was excused.
//
// Binding the exclusion to a filename is the shape this project's
// conventions warn about repeatedly — a guard that asserts a property of
// a NAME rather than the property it claims to check. So the guard now
// matches on the AST: a selector whose Sel is FamilyName, in call
// position. Comments and string literals are not selectors, so the
// needle cannot appear accidentally, no file needs excusing, and this
// file is scanned on exactly the same terms as every other.
func TestFamilyNameHasNoCallSite(t *testing.T) {
	moduleRoot := filepath.Join("..", "..")
	const accessor = "FamilyName"

	var offenders []string
	scanned := 0
	fset := token.NewFileSet()
	err := filepath.WalkDir(moduleRoot, func(path string, d os.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			// Skip testdata and dot-directories BY CATEGORY, the same
			// predicate internal/arch_test.go's walkGoFiles uses —
			// never by naming a specific directory.
			if d.Name() == "testdata" || (path != moduleRoot && strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			return perr
		}
		scanned++
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel == nil || sel.Sel.Name != accessor {
				return true
			}
			offenders = append(offenders, fset.Position(sel.Sel.Pos()).String())
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", moduleRoot, err)
	}

	// Vacuity guard: "no file contains it" is also true of a walk that
	// parsed no files.
	if scanned == 0 {
		t.Fatalf("vacuity guard: the walk of %s parsed 0 Go files, so \"no call site\" says nothing", moduleRoot)
	}

	if len(offenders) > 0 {
		t.Fatalf(
			"%s enumerates (*ot.Name).FamilyName as SUBSTITUTED (it returns %q for an absent name table, and indexes entries[1] unguarded) and records that folio8 never calls it. It is called now, at:\n  %s\nEither decline the accessor as readPostScriptName does, or correct that row from the artifact.",
			vendorBoundaryDoc, "Unknown", strings.Join(offenders, "\n  "))
	}
}

// TestVendorBoundaryDocumentExistsAndIsCited is AC8.5. A test asserting
// properties of a document that has been renamed or deleted must FAIL,
// not pass — so the document is named by path here, its presence
// asserted, and the dependency version it is a claim about checked
// against go.mod rather than trusted.
func TestVendorBoundaryDocumentExistsAndIsCited(t *testing.T) {
	info, err := os.Stat(vendorBoundaryDoc)
	if err != nil {
		t.Fatalf("the enumeration this file makes claims about is missing at internal/fontset/%s: %v — every assertion in this file cites it, and a citation to nothing is worse than no citation", vendorBoundaryDoc, err)
	}
	if info.Size() == 0 {
		t.Fatalf("internal/fontset/%s is empty", vendorBoundaryDoc)
	}

	body, rerr := os.ReadFile(vendorBoundaryDoc)
	if rerr != nil {
		t.Fatalf("read internal/fontset/%s: %v", vendorBoundaryDoc, rerr)
	}
	text := string(body)

	// The document is a claim about ONE dependency version. Assert it
	// says so, and assert that what it says matches the pin.
	if !strings.Contains(text, textshapeVersion) {
		t.Errorf("internal/fontset/%s does not state the textshape version it was measured against (%s) — the document is a claim about a specific dependency version and nothing else", vendorBoundaryDoc, textshapeVersion)
	}
	goMod, gerr := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if gerr != nil {
		t.Fatalf("read folio-go/go.mod: %v", gerr)
	}
	if !strings.Contains(string(goMod), "github.com/boxesandglue/textshape "+textshapeVersion) {
		t.Fatalf("folio-go/go.mod does not require github.com/boxesandglue/textshape %s — internal/fontset/%s is measured against a version the module no longer pins, so every row in it is unverified", textshapeVersion, vendorBoundaryDoc)
	}

	// The method must be replayable, and the three-state classification
	// is what makes the tables readable. Assert both are present rather
	// than assuming a future edit keeps them.
	for _, required := range []string{"honest", "substituted", "fabricated", "16-byte record"} {
		if !strings.Contains(text, required) {
			t.Errorf("internal/fontset/%s does not mention %q — AC4 requires the three-way classification and a replayable method statement", vendorBoundaryDoc, required)
		}
	}
}

// TestForwardHazardsWithoutARedProofAreLabelled is D-000.24's honesty
// clause, made mechanical. Two rows in the enumeration describe vendor
// substitutions that folio8 now refuses at ingestion, so there is NO
// public-entry-point path along which they can be observed reaching an
// output byte — and there was none before either, because the vendor
// subsetter refused first.
//
// THIS TEST IS ITSELF A FORWARD GUARD WITH NO AVAILABLE RED-PROOF for
// the reaching-an-output-byte claim, and says so rather than being
// counted as coverage. What it DOES assert, and can: that the vendor
// substitution exists (above), and that folio8 refuses the face. What no
// assertion in this repository establishes is what those substituted
// metrics would have done to a document, because no document can be made
// to contain them.
func TestForwardHazardsWithoutARedProofAreLabelled(t *testing.T) {
	const faceName = "Roboto-Regular"
	for _, tag := range []string{"hhea", "hmtx"} {
		stripped := stripTableRecord(t, robotoBytes(t), tag)
		if _, err := fontset.New(faceName, stripped); err == nil {
			t.Errorf(
				"fontset.New accepted %s with %q removed. The enumeration labels this row a FORWARD HAZARD WITH NO AVAILABLE RED-PROOF: the vendor substitutes (Ascender 800, Descender -200, advance == upem) but nothing in this repository can show those values reaching an output byte, because folio8 refuses the face and, before that, the vendor subsetter refused it. That label is only honest while the refusal holds.",
				robotoPath(), tag)
		}
	}
}

// itoa renders an int64 in base ten without importing a formatter, so
// this file's expected-value construction stays as plain as the
// production code it checks.
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
