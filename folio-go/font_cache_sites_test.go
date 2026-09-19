package folio8

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestOnlyTheTwoDocumentAwareSitesBuildAFontCache is the guard embedded_face.go
// and newFontCache's own doc comment name, and it exists because the claim they
// made was previously made for a test that cannot support it.
//
// THE CLAIM. Since Story 8.4 a fontCache built WITHOUT the document
// (newFontCache) resolves every name from the caller's FontSet and can see none
// of the faces the document carries. Production code holding a *Template must
// therefore use newDocumentFontCache, and there are exactly TWO such sites:
// predictDocument (render.go, the PDF path) and addCanvasTextPaint
// (page_setup.go, the canvas). If they disagree the canvas measures a document
// the page does not print — AD-17's whole subject.
//
// WHAT USED TO BE CLAIMED, AND WHY IT WAS NOT TRUE. embedded_face.go said
// TestCanvasMeasuresWithTheEmbeddedFace "is what reddens if a third site ever
// calls newFontCache() with a document in hand". It is not: that test exercises
// addCanvasTextPaint and nothing else, so a NEW third production site — a
// second canvas projection, a preview path, a metrics endpoint — would ship
// green past it. A comment claiming a guard that does not exist is worse than
// no comment, because the next reader stops looking.
//
// HOW THIS ONE WORKS. It parses package folio8's own NON-TEST sources and reads,
// by AST rather than by grep, which function each call to either constructor
// sits inside. Test files are excluded deliberately: newFontCache is exactly
// right for a fixture over shipped faces, and dozens of tests use it. The
// production population is what must not grow silently.
func TestOnlyTheTwoDocumentAwareSitesBuildAFontCache(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve package root: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read package root: %v", err)
	}

	fset := token.NewFileSet()
	// callers[constructor] is every enclosing function name that calls it, as
	// "file.go:funcName", so a failure says where to go.
	callers := map[string][]string{}
	declared := map[string]bool{}
	parsed := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, perr := parser.ParseFile(fset, filepath.Join(root, name), nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}
		parsed++
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fn.Name.Name == "newFontCache" || fn.Name.Name == "newDocumentFontCache" {
				declared[fn.Name.Name] = true
			}
			enclosing := fn.Name.Name
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				enclosing = "(method) " + fn.Name.Name
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, isCall := n.(*ast.CallExpr)
				if !isCall {
					return true
				}
				ident, isIdent := call.Fun.(*ast.Ident)
				if !isIdent {
					return true
				}
				if ident.Name == "newFontCache" || ident.Name == "newDocumentFontCache" {
					callers[ident.Name] = append(callers[ident.Name], name+":"+enclosing)
				}
				return true
			})
		}
	}

	// VACUITY GUARDS, all three. A scan that read nothing, or that stopped
	// finding the constructors because they were renamed, must fail loudly
	// rather than report "no offending call site".
	if parsed < 10 {
		t.Fatalf("vacuity guard: only %d non-test source file(s) parsed in package folio8 — the scan read nothing", parsed)
	}
	for _, constructor := range []string{"newFontCache", "newDocumentFontCache"} {
		if !declared[constructor] {
			t.Fatalf("vacuity guard: %s is not declared in package folio8's non-test sources — this guard is asserting nothing", constructor)
		}
	}

	// THE TWO DOCUMENT-AWARE SITES, named. Both must be present (a missing one
	// is the canvas or the page silently losing the document's carried faces),
	// and no third may appear without this test being changed deliberately.
	wantDocumentAware := []string{
		"page_setup.go:addCanvasTextPaint",
		"render.go:predictDocument",
	}
	got := slices.Clone(callers["newDocumentFontCache"])
	slices.Sort(got)
	if !slices.Equal(got, wantDocumentAware) {
		t.Errorf("the document-aware fontCache sites are %v, want exactly %v — the PDF path and the canvas must agree on which faces exist (AD-17)", got, wantDocumentAware)
	}

	// AND NO PRODUCTION CODE BUILDS A DOCUMENTLESS CACHE. There are zero such
	// call sites today, which is the strongest form this can take: any
	// production caller of newFontCache is either holding a *Template (and must
	// use newDocumentFontCache) or is a path that would silently ignore a
	// document's carried faces.
	if offenders := callers["newFontCache"]; len(offenders) != 0 {
		t.Errorf("production code calls newFontCache() at %v — a cache built without the document cannot see a face the document CARRIES; use newDocumentFontCache", offenders)
	}
}
