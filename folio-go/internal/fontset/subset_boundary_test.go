package fontset

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSubsetRejectsGlyphIDsTheFaceDoesNotHave is Story 2.3 finisher's
// Finding 8 guard — the REACHABLE half of the vendor boundary AC5 opened
// when it re-keyed Subset on glyph ids rather than runes.
//
// Before this story, Subset took runes and looked them up through the
// face's own cmap, so an id the face did not have was not expressible.
// Now the caller supplies ids directly, and the vendor does not report
// an out-of-range one — it FABRICATES a glyph. Measured at v0.0.15
// against fonts/notosans/NotoSans-Regular.ttf (4,515 glyphs),
// Subset([]uint16{65535}) returned a complete mapping and a 460-byte
// program with no error at all. Silent wrong bytes from a call that
// reports success is the one failure mode this module exists to make
// impossible.
//
// The negative leg is the point of the test, not decoration: an
// in-range id at the very top of the face's numbering must still be
// ACCEPTED, so the guard cannot be satisfied by rejecting everything.
func TestSubsetRejectsGlyphIDsTheFaceDoesNotHave(t *testing.T) {
	f, err := New("roboto", testFontBytes(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	n := f.face.Font.NumGlyphs()
	if n <= 1 {
		t.Fatalf("test face reports %d glyphs, too few to have both an in-range and an out-of-range id", n)
	}
	last := uint16(n - 1)
	beyond := uint16(n)
	t.Logf("SUBJECT: folio-go/testdata/fonts/Roboto-Regular.ttf, NumGlyphs=%d (valid ids 0..%d)", n, last)

	rejected := []struct {
		name  string
		input []uint16
	}{
		{"one id just past the end", []uint16{beyond}},
		{"the uint16 ceiling", []uint16{65535}},
		{"a valid id followed by an invalid one", []uint16{last, beyond}},
		{"an invalid id followed by a valid one", []uint16{beyond, last}},
	}
	for _, tc := range rejected {
		sub, serr := f.Subset(tc.input)
		if serr == nil {
			t.Errorf(
				"%s: Subset(%v) succeeded on a face with %d glyphs, returning a %d-byte program. "+
					"The subsetter fabricated a glyph the face does not have, and the document would "+
					"embed it while reporting success",
				tc.name, tc.input, n, len(sub.Program),
			)
			continue
		}
		if !strings.Contains(serr.Error(), "does not exist in this face") {
			t.Errorf("%s: Subset(%v) failed with %q, which is not the located out-of-range diagnostic", tc.name, tc.input, serr)
		}
	}

	// NEGATIVE LEG: the highest VALID id must still be accepted, or the
	// guard above would be satisfied by an off-by-one that rejects a
	// legitimate glyph.
	sub, err := f.Subset([]uint16{last})
	if err != nil {
		t.Fatalf("Subset([%d]) rejected the face's HIGHEST VALID glyph id: %v — the bound is off by one", last, err)
	}
	if _, ok := sub.GlyphForSource[last]; !ok {
		t.Fatalf("Subset([%d]) accepted the face's highest valid glyph id but returned no mapping for it", last)
	}
	t.Logf("%d of %d out-of-range inputs rejected; the highest valid id (%d) still accepted", len(rejected), len(rejected), last)
}

// TestSubsetIsInvariantUnderDuplicateGlyphIDs closes the coverage
// regression Story 2.3 introduced (finisher, Finding 9, part two).
//
// shapedGlyphIDs deduplicates before calling Subset, so after AC5's
// re-key no test in this package passed a slice containing duplicates
// and the dedup `continue` branch measured count 0. The PRODUCTION path
// does pass duplicates — render.go appends EVERY shaped glyph id across
// the whole document, and any repeated letter repeats its id — so
// invariance under multiplicity became a live property with no unit
// coverage. The pre-change rune-based tests exercised it only by
// accident, through "Hello, World! 0123456789" repeating l, o and space.
//
// The assertion is byte-identity against the deduplicated call, which is
// stronger than "it did not error".
//
// WHAT THIS TEST DOES AND DOES NOT PROVE — stated because the red-proof
// came back GREEN and that result is informative rather than a failure
// to report. Disabling folio8's own `seen` dedup inside Subset does NOT
// redden this test. Measured directly against the vendor at v0.0.15
// (folio-go/testdata/fonts/Roboto-Regular.ttf, the same 10 shaped ids
// expanded to 40 with duplicates): calling subset.Input.AddGlyphs with
// the duplicate-bearing slice produced NumOutputGlyphs 22 and a 7,672-byte
// program, byte-for-byte identical to the deduplicated call. The vendor
// deduplicates into its own glyph set.
//
// So invariance under multiplicity is enforced TWICE — once here, once
// below the seam — and this test asserts the PROPERTY the render path
// actually depends on (render.go appends every shaped glyph id in the
// document, duplicates included) rather than attributing it to folio8's
// branch. folio8's dedup is defence in depth against a vendor contract
// change, which is worth having and is worth not overclaiming: this is a
// forward guard on that contract, not a red-proved statement about the
// `continue` branch, and D-000.24 says to label it rather than credit it.
func TestSubsetIsInvariantUnderDuplicateGlyphIDs(t *testing.T) {
	f, err := New("roboto", testFontBytes(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	unique := shapedGlyphIDs(t, f, "Hello, World!")
	if len(unique) < 2 {
		t.Fatalf("shaping produced %d distinct glyphs, too few to build a duplicate-bearing input", len(unique))
	}

	// Interleave every id with a repeat of the FIRST id, and append the
	// whole unique run again — so duplicates appear adjacent, separated,
	// and as a wholesale repetition.
	var withDupes []uint16
	for _, g := range unique {
		withDupes = append(withDupes, g, unique[0], g)
	}
	withDupes = append(withDupes, unique...)
	if len(withDupes) <= len(unique) {
		t.Fatalf("test bug: the duplicate-bearing input (%d ids) is no longer than the unique one (%d)", len(withDupes), len(unique))
	}

	base, err := f.Subset(unique)
	if err != nil {
		t.Fatalf("Subset(unique): %v", err)
	}
	if len(base.Program) == 0 {
		t.Fatal("Subset(unique) produced no program bytes — the comparison below would pass vacuously")
	}
	dup, err := f.Subset(withDupes)
	if err != nil {
		t.Fatalf("Subset(withDupes): %v", err)
	}

	if !bytes.Equal(base.Program, dup.Program) {
		t.Errorf(
			"a %d-id input carrying duplicates produced a different program (%d B) from its %d-id "+
				"deduplicated form (%d B) — Subset is no longer invariant under multiplicity, which the "+
				"render path relies on because it appends every shaped glyph id in the document",
			len(withDupes), len(dup.Program), len(unique), len(base.Program),
		)
	}
	if base.Tag != dup.Tag {
		t.Errorf("tags diverged under duplication: unique=%s withDupes=%s", base.Tag, dup.Tag)
	}
	if base.NumGlyphs != dup.NumGlyphs {
		t.Errorf("output glyph counts diverged under duplication: unique=%d withDupes=%d", base.NumGlyphs, dup.NumGlyphs)
	}
	t.Logf("%d unique ids vs %d ids with duplicates: byte-identical programs, tag %s", len(unique), len(withDupes), base.Tag)
}

// TestSubsetDoesNotSortItsInput is Finding 9 part one, and it exists
// because TestPermutationInvariance CANNOT prove what AC5 claims for it.
//
// That test permutes Subset's input and observes identical output, and
// concludes Subset does not sort. But the invariance it observes is
// produced by the VENDOR, two frames below the call:
// subset.Plan.createCompactMapping sorts the glyph set itself. A
// defensive sort added inside fontset.Subset would therefore NOT redden
// TestPermutationInvariance — the output is identical either way. It is
// a vendor forward-guard, not a proof about folio8's code, and AC5's
// claim that the re-key "preserves AC8a's discriminating power" was
// overstated: the discriminating power against a folio8-side sort was
// never there.
//
// A source scan is the only mechanism that can actually catch the
// reintroduction, so that is what this is. It resolves import aliases
// and matches on the sort/slices packages by PATH rather than by literal
// text, the same discipline internal/pdf's numeric-formatting guard
// uses, and it scopes itself to the Subset method's own body by walking
// the AST rather than by grepping the file.
//
// D-1.5.7, verbatim on why the ban exists at all: a sort here would make
// permuted-then-sorted inputs the same input, so AC8a's proof would pass
// regardless of whether the property held.
func TestSubsetDoesNotSortItsInput(t *testing.T) {
	const file = "fontset.go"
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	fset := token.NewFileSet()
	parsed, perr := parser.ParseFile(fset, file, src, parser.ParseComments)
	if perr != nil {
		t.Fatalf("parse %s: %v", file, perr)
	}

	// Resolve local import names to full paths, so an aliased import
	// cannot slip a sort past a name match.
	aliases := map[string]string{}
	for _, imp := range parsed.Imports {
		p := strings.Trim(imp.Path.Value, `"`)
		name := p
		if i := strings.LastIndex(p, "/"); i >= 0 {
			name = p[i+1:]
		}
		if imp.Name != nil {
			name = imp.Name.Name
		}
		aliases[name] = p
	}

	var body *ast.BlockStmt
	ast.Inspect(parsed, func(n ast.Node) bool {
		fd, ok := n.(*ast.FuncDecl)
		if !ok || fd.Name.Name != "Subset" || fd.Recv == nil {
			return true
		}
		body = fd.Body
		return false
	})
	// VACUITY GUARD: if the method is ever renamed or moved, this test
	// must fail rather than silently scan nothing. That is the exact
	// hard-coded-name trap this project has hit before.
	if body == nil {
		t.Fatalf("no method named Subset found in %s — this guard scanned nothing and proves nothing", file)
	}

	calls := 0
	var found []string
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		calls++
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkgIdent, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		path := aliases[pkgIdent.Name]
		if path != "sort" && path != "slices" {
			return true
		}
		if !strings.Contains(strings.ToLower(sel.Sel.Name), "sort") {
			return true
		}
		pos := fset.Position(call.Pos())
		found = append(found, filepath.Base(pos.Filename)+":"+itoa(pos.Line)+": "+path+"."+sel.Sel.Name)
		return true
	})

	if calls == 0 {
		t.Fatalf("Subset's body contains no call expressions at all — this guard scanned nothing")
	}
	if len(found) > 0 {
		t.Fatalf(
			"fontset.Subset sorts its input (D-1.5.7, AC8a forbids it):\n  %s\n\n"+
				"TestPermutationInvariance will NOT catch this — the vendor's own "+
				"subset.Plan.createCompactMapping sorts the glyph set below this call, so a sort here "+
				"produces identical output and that test stays green. This scan is the only mechanism "+
				"that can see it.",
			strings.Join(found, "\n  "),
		)
	}
	t.Logf("%d call expressions in fontset.Subset scanned, 0 sort calls (imports resolved by path, not by name)", calls)
}

// itoa renders a small non-negative int without strconv, matching the
// no-frills style the other source-scanning guards in this repo use.
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
