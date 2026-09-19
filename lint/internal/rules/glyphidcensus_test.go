package rules

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// TestGlyphIdentifierCensus is DW-16's replacement owner (D-000.73).
//
// WHY THIS EXISTS. `pagemodel.ShapedGlyph.CID` carries two different
// kinds of value under one name: in the allocator's base block it IS a
// subset glyph id, which any renderer holding the font can resolve; in
// the default block it is a SYNTHETIC identifier minted so PDF's
// /ToUnicode CMap can map one identifier to one text, and it indexes a
// table the page model deliberately does not carry. Nothing in the type
// tells the two apart. AD-5 says keeping PDF out of the page model is
// "what keeps PNG/SVG/HTML renderers possible later", and this field
// partially defeats that.
//
// DW-16 sat open for three epics behind an owner that reads "the first
// non-PDF renderer story" — a story that does not exist and is not
// scheduled anywhere through Epic 6 (measured, not grepped: Story 5.10's
// preview consumes the real PDF via pdfjs-dist, and Story 5.9's canvas
// paints pre-broken lines of text and never touches TextRun.Glyphs).
// D-000.73 ruled the class rather than the instance: AN OWNER THAT IS A
// STORY-THAT-MAY-NEVER-EXIST IS REPLACED BY A MECHANISM THAT FIRES ON
// THE REAL CONDITION. This test is that mechanism. Its two censuses are
// the two conditions under which DW-16's pricing actually changes:
//
//   - A THIRD PRODUCER means the "option 1 is a local change" argument
//     has moved again, as it already moved once unnoticed at Story 2.7.
//   - A READ FROM OUTSIDE internal/pdf AND THE COPIER IS THE FORCING
//     FUNCTION ARRIVING. A non-PDF consumer of this field is precisely
//     the event DW-16 has been waiting for; when it lands, this test is
//     red and the entry re-opens with a real deadline.
//
// ANCHOR (D-000.68). Two, and neither is the code's own spelling. The
// censuses below are TEST-OWNED LITERALS: growth of either set requires
// editing this file, which says in its own text that the set is closed
// — that is the discriminator against stating it relationally (D-3.1a.3
// is relational because its set is expected to move; this one is not).
// Field identity is resolved through GO/TYPES to the single *types.Var
// declared by internal/pagemodel, never by matching the name "CID". The
// name match is not a hypothetical weakness here: internal/text declares
// its OWN ShapedGlyph with its own CID field, and package folio8 reads
// `.CID` off pdf.CIDText. A spelling-based instrument reports three
// producers and eight consumers on this tree. The type checker reports
// two and two.
//
// PLACEMENT. This lives in lint rather than in folio-go because lint
// already type-checks the whole folio-go module through packages.Load
// (D-1.3.11), and because D-3.7.9's lesson is explicit that AST-only
// scans over folio-go/ are the weaker instrument "wherever a lint-side
// equivalent is affordable". It is also in the CI job that is green and
// independently scheduled (ci.yml's lint job declares no `needs:`).
func TestGlyphIdentifierCensus(t *testing.T) {
	// The closed sets. Editing either of these is the deliberate,
	// reviewable act DW-16 asks for — never a drive-by.
	wantProducers := []string{
		"folio-go/render.go:buildShapedPDFRuns",         // the allocator: mints base AND synthetic values
		"folio-go/page_number.go:resolvePageRunForPage", // Story 2.7: a COPIER, reads CIDs the allocator minted
	}
	wantReaderPkgs := []string{
		"github.com/panitw/folio8/folio-go",              // the copier's own read path (page_number.go)
		"github.com/panitw/folio8/folio-go/internal/pdf", // the only legitimate consumer
	}

	root := repoRootFromTest(t)
	moduleRoot := filepath.Join(root, "folio-go")

	field, pkgs := loadShapedGlyphCIDField(t, moduleRoot)
	producers, readers, sites := censusShapedGlyphCID(t, pkgs, field)

	// VACUITY GUARD (D-000.9). A census that visited nothing reports the
	// same "matches the expected set" a healthy one does when the
	// expected set is also empty. Assert the scan actually found sites
	// before comparing sets, and fail on zero rather than pass.
	if sites == 0 {
		t.Fatalf("the census resolved zero references to pagemodel.ShapedGlyph.CID across the whole folio-go module — that is a broken scan, not a clean tree (D-000.9)")
	}

	slices.Sort(producers)
	slices.Sort(readers)
	slices.Sort(wantProducers)
	slices.Sort(wantReaderPkgs)

	// Reported BOTH ways and BY NAME, never as a count: "2 producers,
	// want 2" hides a swap, and a count does not tell the reader which
	// site to look at (D-000.68).
	reportSetDiff(t, "producer", producers, wantProducers,
		"a third construction site of pagemodel.ShapedGlyph re-prices DW-16 — the entry's cost argument has already moved once unnoticed (Story 2.7). Add it here deliberately, and re-read DW-16 before you do")
	reportSetDiff(t, "reader package", readers, wantReaderPkgs,
		"a read of ShapedGlyph.CID from outside internal/pdf and the copier IS DW-16's forcing function arriving: a non-PDF consumer cannot resolve the synthetic values and cannot detect that it is holding one. Do not silence this by widening the set — route it to the engineering lead")
}

// loadShapedGlyphCIDField resolves the ONE *types.Var that is
// internal/pagemodel.ShapedGlyph's CID field, and returns it alongside
// the loaded packages. Every failure here is Fatal: a census that could
// not find its own subject must never read as a census that found
// nothing wrong (D-000.9).
func loadShapedGlyphCIDField(t *testing.T, moduleRoot string) (*types.Var, []*packages.Package) {
	t.Helper()

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Dir:   moduleRoot,
		Tests: false, // the census is over shipped code; a test may construct whatever it likes
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		t.Fatalf("load packages under %s: %v", moduleRoot, err)
	}
	var loadErrs []string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			loadErrs = append(loadErrs, fmt.Sprintf("%s: %s", p.PkgPath, e.Error()))
		}
	})
	if len(loadErrs) > 0 {
		t.Fatalf("type information unavailable under %s — failing loudly rather than reporting an empty census (D-1.3.11): %s",
			moduleRoot, strings.Join(loadErrs, "; "))
	}

	const pagemodelPath = "github.com/panitw/folio8/folio-go/internal/pagemodel"
	var field *types.Var
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		if p.PkgPath != pagemodelPath || p.Types == nil || field != nil {
			return
		}
		obj := p.Types.Scope().Lookup("ShapedGlyph")
		if obj == nil {
			return
		}
		st, ok := obj.Type().Underlying().(*types.Struct)
		if !ok {
			return
		}
		for i := 0; i < st.NumFields(); i++ {
			if st.Field(i).Name() == "CID" {
				field = st.Field(i)
				return
			}
		}
	})
	if field == nil {
		t.Fatalf("could not resolve %s.ShapedGlyph.CID through go/types — if the field was renamed, DW-16's ruling requires that rename to be a deliberate act with the entry re-read, so update this test rather than deleting it", pagemodelPath)
	}
	return field, pkgs
}

// censusShapedGlyphCID walks every non-test file of the module and
// classifies each reference to field.
//
// A PRODUCER is a function containing a composite literal whose type is
// pagemodel.ShapedGlyph. A READER is any package that mentions the field
// outside such a literal. sites counts every resolved reference, and is
// the vacuity guard's subject.
func censusShapedGlyphCID(t *testing.T, pkgs []*packages.Package, field *types.Var) (producers, readers []string, sites int) {
	t.Helper()

	shaped := field.Type() // not used for identity; kept for clarity of intent
	_ = shaped

	producerSet := map[string]bool{}
	readerSet := map[string]bool{}

	packages.Visit(pkgs, nil, func(p *packages.Package) {
		if p.TypesInfo == nil || strings.HasSuffix(p.PkgPath, ".test") {
			return
		}
		for _, file := range p.Syntax {
			path := p.Fset.Position(file.Pos()).Filename
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			rel := relToModule(path)

			// Producers: composite literals OF the struct type that
			// declares the field. Resolved by comparing the literal's
			// go/types type against the field's parent struct, so a
			// same-named type in another package cannot match.
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				lt := p.TypesInfo.TypeOf(lit)
				if lt == nil {
					return true
				}
				named, ok := lt.(*types.Named)
				if !ok || named.Obj() == nil || named.Obj().Name() != "ShapedGlyph" {
					return true
				}
				if named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != field.Pkg().Path() {
					return true // internal/text declares its own ShapedGlyph; not this one
				}
				producerSet[rel+":"+enclosingFuncName(file, lit.Pos())] = true
				return true
			})

			// Readers: every identifier that RESOLVES to this exact
			// *types.Var, minus the ones that are keys inside a
			// producer literal (those are writes, already counted).
			writeKeys := map[ast.Node]bool{}
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				for _, el := range lit.Elts {
					if kv, ok := el.(*ast.KeyValueExpr); ok {
						writeKeys[kv.Key] = true
					}
				}
				return true
			})
			for id, obj := range p.TypesInfo.Uses {
				if obj != field {
					continue
				}
				sites++
				if writeKeys[ast.Node(id)] {
					continue
				}
				readerSet[p.PkgPath] = true
			}
		}
	})

	for k := range producerSet {
		producers = append(producers, k)
	}
	for k := range readerSet {
		readers = append(readers, k)
	}
	return producers, readers, sites
}

// relToModule trims everything above folio-go/ so a finding reads the
// way the decision log writes it.
func relToModule(path string) string {
	if i := strings.Index(path, "folio-go/"); i >= 0 {
		return path[i:]
	}
	return path
}

// enclosingFuncName returns the name of the function declaration
// containing pos, or "<file scope>" if there is none.
func enclosingFuncName(file *ast.File, pos token.Pos) string {
	for _, d := range file.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fd.Pos() <= pos && pos <= fd.End() {
			return fd.Name.Name
		}
	}
	return "<file scope>"
}

// reportSetDiff reports the symmetric difference between got and want by
// name, in both directions, with remedy naming what the reader must
// actually do (D-000.37: a tripwire whose remedy is not executable by a
// human is not a tripwire).
func reportSetDiff(t *testing.T, kind string, got, want []string, remedy string) {
	t.Helper()
	for _, g := range got {
		if !slices.Contains(want, g) {
			t.Errorf("UNEXPECTED %s %q — this census is a closed set (DW-16, D-000.73). %s", kind, g, remedy)
		}
	}
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("EXPECTED %s %q is GONE — if it was legitimately removed, remove it from this test's literal in the same commit; a census that quietly over-states its own subject is the shape D-000.9 forbids", kind, w)
		}
	}
}
