package arch

// Story 2.5's architectural guards, in one file because they are one
// property seen from four sides: band composition lives in
// internal/layout, produces internal/pagemodel, and neither of them
// knows what a PDF is.
//
// Every guard here is AST-based and imports no first-party package —
// which is why the `arch` package sits at the stage-rank table's
// rankNoStage: it observes the tree and is downstream of nothing.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------
// Shared parsing helpers
// ---------------------------------------------------------------------

// parsedFile is one .go file with its AST and the path it was read from.
type parsedFile struct {
	rel  string
	file *ast.File
	fset *token.FileSet
}

// parseDirNonTest parses every non-test .go file DIRECTLY in dir (not
// recursively — each caller here is asking about one package). It fails
// the test on a read or parse error rather than returning an empty slice,
// because "the directory could not be read" and "the directory is clean"
// must never produce the same answer (D-1.3.3 amended).
func parseDirNonTest(t *testing.T, dir string) []parsedFile {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	var out []parsedFile
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		f, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			t.Fatalf("parse %s: %v", path, perr)
		}
		out = append(out, parsedFile{rel: name, file: f, fset: fset})
	}
	return out
}

// exprString renders an expression back to source-like text, for the
// operand tests below. It handles only the shapes those tests care about
// and returns "" for anything else, which reads as "not a match" rather
// than as a match.
func exprString(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return exprString(v.X) + "." + v.Sel.Name
	case *ast.CallExpr:
		return exprString(v.Fun) + "(...)"
	case *ast.ParenExpr:
		return exprString(v.X)
	case *ast.IndexExpr:
		return exprString(v.X) + "[...]"
	case *ast.StarExpr:
		return "*" + exprString(v.X)
	}
	return ""
}

// moduleRoot resolves folio-go/ from this package's own directory
// (folio-go/internal/), the same way TestNoFloat64UnderModule does.
func moduleRoot(t *testing.T) string {
	t.Helper()
	internalRoot, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve internal/ root: %v", err)
	}
	return filepath.Dir(internalRoot)
}

// ---------------------------------------------------------------------
// AC1 — internal/pagemodel exists and names no PDF concept
// ---------------------------------------------------------------------

// pdfConceptSubstrings is the closed list AC1 enumerates, spelled as
// lower-case substrings matched against every identifier the package
// declares: "no identifier in the package names a PDF object reference,
// a resource dictionary, a content-stream operator, or a font PROGRAM".
//
// It is deliberately a substring list rather than an exact-name list,
// the same fail-safe reasoning isBannedImportPath uses in lint's
// forbidden-imports rule: "ResourceName" and "ImageResourceDict" are
// both the thing being excluded, and an exact-match list is the erosion
// path.
var pdfConceptSubstrings = []string{
	"resource",      // a resource dictionary or its keys
	"xobject",       // a PDF XObject
	"objref",        // a PDF object reference
	"objectnumber",  // ditto
	"dict",          // any PDF dictionary
	"contentstream", // a content stream
	"operator",      // a content-stream operator
	"fontfile",      // /FontFile2 — an embedded font PROGRAM
	"fontprogram",   // ditto, spelled out
	"subsettag",     // the subset tag is a property of an embedded program
	"flatedecode",   // a PDF stream filter
	"dctdecode",     // ditto
	"pdf",           // the format itself, named anywhere
}

// TestPageModelNamesNoPDFConcept is AC1: internal/pagemodel declares the
// page model, and AD-5's "the page model knows nothing about PDF" is
// asserted over its identifiers rather than described in its doc comment.
//
// PRESENCE PRECONDITION (D-000.9, D-000.21 sharpened): before asserting
// that nothing in the package names a PDF concept, it asserts the package
// actually DECLARES at least one exported type carrying at least one
// field. Without that, an empty package — or a deleted one — produces the
// same all-clear a healthy one does.
func TestPageModelNamesNoPDFConcept(t *testing.T) {
	dir := filepath.Join(moduleRoot(t), "internal", "pagemodel")
	files := parseDirNonTest(t, dir)
	if len(files) == 0 {
		t.Fatalf("presence precondition: no non-test .go file under %s — internal/pagemodel must exist (AC1)", dir)
	}

	type namedIdent struct {
		name string
		pos  token.Pos
		what string
		fset *token.FileSet
		rel  string
	}
	var idents []namedIdent
	exportedTypesWithFields := 0
	fieldsSeen := 0

	for _, pf := range files {
		// Imports: only internal/geom is permitted first-party. (The
		// import DIRECTION is the stage-rank rule's job; this asserts the
		// stricter content property Finding 6 names — a page model that
		// embedded internal/template values would satisfy every rank check
		// and still put document-model concepts into a renderer's input.)
		for _, imp := range pf.file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if !strings.Contains(path, "/folio-go/internal/") {
				continue
			}
			if strings.HasSuffix(path, "/internal/geom") {
				continue
			}
			t.Errorf("%s: internal/pagemodel imports %q; AD-5 permits internal/geom and nothing else first-party — the page model carries resolved geometry and glyph runs, never document-model or renderer types",
				pf.rel, path)
		}

		for _, decl := range pf.file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				idents = append(idents, namedIdent{ts.Name.Name, ts.Name.Pos(), "type", pf.fset, pf.rel})
				st, ok := ts.Type.(*ast.StructType)
				if !ok || st.Fields == nil {
					continue
				}
				n := 0
				for _, field := range st.Fields.List {
					for _, fn := range field.Names {
						idents = append(idents, namedIdent{fn.Name, fn.Pos(), "field", pf.fset, pf.rel})
						n++
						fieldsSeen++
					}
					// The field's TYPE is an identifier too: a field of a
					// renderer type is exactly the AC1 red-proof's shape.
					if ts.Name.IsExported() {
						if s := exprString(field.Type); s != "" {
							idents = append(idents, namedIdent{s, field.Pos(), "field type", pf.fset, pf.rel})
						}
					}
				}
				if ts.Name.IsExported() && n > 0 {
					exportedTypesWithFields++
				}
			}
		}
	}

	if exportedTypesWithFields == 0 || fieldsSeen == 0 {
		t.Fatalf("presence precondition (D-000.9): internal/pagemodel declares %d exported struct type(s) with fields and %d field(s) in total — with none, the identifier assertions below pass vacuously on an empty package",
			exportedTypesWithFields, fieldsSeen)
	}

	for _, id := range idents {
		lower := strings.ToLower(id.name)
		for _, banned := range pdfConceptSubstrings {
			if strings.Contains(lower, banned) {
				pos := id.fset.Position(id.pos)
				t.Errorf("%s:%d: internal/pagemodel's %s %q names a PDF concept (%q). AD-5: \"the page model knows nothing about PDF\" — its types name only geometry, glyph runs (font identity + glyph ids + positions) and images. A face NAME and a glyph id belong here; a resource dictionary, an object reference, a content-stream operator and an embedded font PROGRAM do not (those are internal/pdf's EmbeddedFace and ImageXObject, and they stay there)",
					id.rel, pos.Line, id.what, id.name, banned)
			}
		}
	}
}

// ---------------------------------------------------------------------
// AC4 — AD-5's arrow, asserted BY NAME as well as by rank
// ---------------------------------------------------------------------

// firstPartyImportGraph maps each package directory under internal/ to
// the set of internal packages it imports, over EVERY .go file including
// tests. Test files are included deliberately: an arrow laundered through
// a test helper is still an arrow, and D-1.3.1's precedent is not to
// grant an exemption pre-emptively.
func firstPartyImportGraph(t *testing.T, internalDir string) map[string]map[string]bool {
	t.Helper()
	graph := map[string]map[string]bool{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(internalDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, rerr := filepath.Rel(internalDir, path)
		if rerr != nil {
			return rerr
		}
		pkg := filepath.ToSlash(filepath.Dir(rel))
		if slash := strings.Index(pkg, "/"); slash != -1 {
			pkg = pkg[:slash]
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}
		if graph[pkg] == nil {
			graph[pkg] = map[string]bool{}
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			const marker = "/folio-go/internal/"
			idx := strings.Index(p, marker)
			if idx == -1 {
				continue
			}
			target := p[idx+len(marker):]
			if slash := strings.Index(target, "/"); slash != -1 {
				target = target[:slash]
			}
			if target == pkg {
				continue // the external-test-package idiom, not an arrow
			}
			graph[pkg][target] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", internalDir, err)
	}
	return graph
}

// TestInternalLayoutDoesNotReachInternalPDF is AD-5's arrow asserted BY
// NAME, and it is deliberately REDUNDANT with lint's `stage-rank` rule.
//
// D-000.23: a guard written for a CLASS is not evidence about the
// INSTANCE. A rank table is a class guard, and a future edit to that
// table could permit this one arrow without anyone noticing that AD-5 was
// what the number meant. This test names the arrow, so editing the ranks
// is not enough to lose it.
//
// TRANSITIVE, not direct: an arrow laundered through a third package is
// the same arrow.
//
// PRESENCE PRECONDITION: internal/layout must have at least one
// first-party import before "internal/pdf is not among them" means
// anything — a package that imports nothing satisfies the assertion
// vacuously.
func TestInternalLayoutDoesNotReachInternalPDF(t *testing.T) {
	internalDir := filepath.Join(moduleRoot(t), "internal")
	graph := firstPartyImportGraph(t, internalDir)

	direct, ok := graph["layout"]
	if !ok {
		t.Fatal("presence precondition: no package directory \"layout\" found under folio-go/internal/ — AD-5's arrow is unassertable (AC2 creates this package)")
	}
	if len(direct) == 0 {
		t.Fatal("presence precondition (D-000.9): internal/layout has ZERO first-party imports — \"internal/pdf is not among them\" is satisfied vacuously by a package that imports nothing")
	}

	// Transitive reachability, breadth-first over the first-party graph.
	seen := map[string]bool{"layout": true}
	queue := []string{"layout"}
	pathTo := map[string]string{"layout": "internal/layout"}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		targets := make([]string, 0, len(graph[cur]))
		for target := range graph[cur] {
			targets = append(targets, target)
		}
		sort.Strings(targets) // deterministic witness path
		for _, target := range targets {
			if seen[target] {
				continue
			}
			seen[target] = true
			pathTo[target] = pathTo[cur] + " -> internal/" + target
			queue = append(queue, target)
		}
	}

	if seen["pdf"] {
		t.Fatalf(
			"AD-5 VIOLATED: internal/layout reaches internal/pdf (%s).\n\n"+
				"The spine's first line of grounding, verbatim: \"One dependency arrow matters more than "+
				"the rest: there is none from internal/layout to internal/pdf. That absence is what keeps "+
				"PNG/SVG/HTML renderers possible later (AD-5), and it is precisely the arrow a well-meaning "+
				"commit will try to add.\"\n\n"+
				"The remedy is never to relax the rank table: pass what layout needs as a PARAMETER, or move "+
				"the value into internal/pagemodel (rank 1), which both stages may see.",
			pathTo["pdf"])
	}
}

// ---------------------------------------------------------------------
// AC5 — the content band's height is derived by exactly one function
// ---------------------------------------------------------------------

// bandHeightOperandNames are the field/variable names that identify a
// DECLARED BAND HEIGHT. A subtraction reaching one of these is the
// content-height derivation, wherever it is written.
var bandHeightOperandNames = map[string]bool{
	"PageHeaderHeight": true,
	"PageFooterHeight": true,
	"pageHeaderHeight": true,
	"pageFooterHeight": true,
	"headerHeight":     true,
	"footerHeight":     true,
	"usableHeight":     true,
}

// subtractionOperands returns every operand appearing anywhere in a
// chain of `-` expressions rooted at e, flattened. `a - b - c - d`
// parses as `((a-b)-c)-d`, so a shallow look at Op's two children misses
// most of the chain — this is the same flattening internal/pdf's
// flip-routing guard performs, for the same reason.
func subtractionOperands(e ast.Expr) []ast.Expr {
	be, ok := e.(*ast.BinaryExpr)
	if !ok || be.Op != token.SUB {
		return nil
	}
	out := subtractionOperands(be.X)
	if out == nil {
		out = []ast.Expr{be.X}
	}
	out = append(out, be.Y)
	return out
}

// operandNamesIn returns the trailing identifier of each operand, so
// `g.PageFooterHeight` contributes "PageFooterHeight".
func operandNamesIn(operands []ast.Expr) []string {
	var names []string
	for _, op := range operands {
		switch v := op.(type) {
		case *ast.Ident:
			names = append(names, v.Name)
		case *ast.SelectorExpr:
			names = append(names, v.Sel.Name)
		}
	}
	return names
}

// TestContentHeightIsDerivedByExactlyOneFunction is AC5, in D-1.8.10's
// POSITIVE shape — "assert that ALL ... routes through the one function,
// not that 'nobody else writes a minus sign', which is unfalsifiable".
//
// Here the positive form is exact rather than conventional: the content
// band's height is the only quantity in internal/layout derived by
// subtracting BAND HEIGHTS, so "every site that needs a content height
// obtains it from ContentHeight" is checkable as "no function other than
// ContentHeight subtracts a band height".
//
// It also asserts AD-24's "nothing negotiates" STRUCTURALLY: ContentHeight
// takes exactly one parameter, of type PageGeometry. It is not given the
// content band's elements, so it cannot consult them, and the content
// band can never grow to fit.
//
// PRESENCE PRECONDITION: ContentHeight must be declared exactly once AND
// called from at least one non-test site, or this passes on a function
// nobody calls.
func TestContentHeightIsDerivedByExactlyOneFunction(t *testing.T) {
	dir := filepath.Join(moduleRoot(t), "internal", "layout")
	files := parseDirNonTest(t, dir)
	if len(files) == 0 {
		t.Fatalf("presence precondition: no non-test .go file under %s (AC2 creates internal/layout)", dir)
	}

	declarations := 0
	callSites := 0
	var findings []string

	for _, pf := range files {
		for _, decl := range pf.file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			if fd.Name.Name == "ContentHeight" {
				declarations++
				if fd.Type.Params == nil || len(fd.Type.Params.List) != 1 {
					t.Errorf("%s: ContentHeight must take exactly one parameter (the page geometry); AD-24's \"nothing negotiates\" is enforced by what it CANNOT be handed", pf.rel)
				} else if got := exprString(fd.Type.Params.List[0].Type); got != "PageGeometry" {
					t.Errorf("%s: ContentHeight's parameter is %q, want \"PageGeometry\" — its inputs are page geometry ONLY; it must not receive the content band's elements or their measured sizes (AD-24)", pf.rel, got)
				}
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if ok {
					if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "ContentHeight" {
						callSites++
					}
					return true
				}
				be, ok := n.(*ast.BinaryExpr)
				if !ok || be.Op != token.SUB {
					return true
				}
				for _, name := range operandNamesIn(subtractionOperands(be)) {
					if bandHeightOperandNames[name] {
						pos := pf.fset.Position(be.Pos())
						findings = append(findings, pf.rel+":"+itoaArch(pos.Line)+": function "+fd.Name.Name+" subtracts band height "+name+" directly")
						return false
					}
				}
				return true
			})
		}
	}

	if declarations != 1 {
		t.Fatalf("AC5: internal/layout declares ContentHeight %d times, want exactly 1 — the content band's height is derived BY ONE FUNCTION (folio-format.md; storing or re-deriving it is a second source of truth for a derived quantity)", declarations)
	}
	if callSites == 0 {
		t.Fatal("presence precondition (D-000.9): ContentHeight is called from ZERO non-test sites inside internal/layout — \"every site that needs a content height obtains it from that function\" is satisfied vacuously when no site needs one")
	}
	if len(findings) > 0 {
		t.Fatalf("AC5 violation — the content band's height must be derived by ContentHeight and nowhere else:\n%s", strings.Join(findings, "\n"))
	}
}

// ---------------------------------------------------------------------
// AC2 — internal/layout alone places bands; package folio8 computes none
// ---------------------------------------------------------------------

// TestNoBandOriginArithmeticInPackageFolio8 is AC2's negative half,
// asserted structurally rather than by naming convention: after this
// story, band origins arrive in package folio8 as VALUES from
// internal/layout.Origins, and no function in package folio8 derives one.
//
// The detection is the same shape internal/pdf's flip-routing guard uses
// (D-1.8.10, Finding 3): flatten every `-` chain and test its OPERANDS,
// so a differently-named variable and a bare inline expression are caught
// as readily as a conventionally-named one.
//
// PRESENCE PRECONDITION: package folio8 must still declare Render,
// RenderTo and renderDocument (Finding 4's single document-byte
// producer), or this passes on a deleted package.
func TestNoBandOriginArithmeticInPackageFolio8(t *testing.T) {
	root := moduleRoot(t)
	files := parseDirNonTest(t, root)
	if len(files) == 0 {
		t.Fatalf("presence precondition: no non-test .go file directly under %s", root)
	}

	declared := map[string]bool{}
	layoutOriginsCalls := 0
	var findings []string

	for _, pf := range files {
		// Resolve the local alias for internal/layout, so the call below
		// is matched by the package it RESOLVES to, never by the literal
		// text "layout." (AC12's discipline).
		layoutAlias := ""
		for _, imp := range pf.file.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if !strings.HasSuffix(p, "/internal/layout") {
				continue
			}
			layoutAlias = p[strings.LastIndex(p, "/")+1:]
			if imp.Name != nil {
				layoutAlias = imp.Name.Name
			}
		}

		for _, decl := range pf.file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv != nil {
				continue
			}
			declared[fd.Name.Name] = true
			if fd.Body == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						if id, ok := sel.X.(*ast.Ident); ok && layoutAlias != "" && id.Name == layoutAlias && sel.Sel.Name == "Origins" {
							layoutOriginsCalls++
						}
					}
					return true
				}
				be, ok := n.(*ast.BinaryExpr)
				if !ok || be.Op != token.SUB {
					return true
				}
				for _, name := range operandNamesIn(subtractionOperands(be)) {
					if bandHeightOperandNames[name] || name == "MarginTop" || name == "MarginBottom" ||
						((name == "Top" || name == "Bottom") && chainMentions(be, "Margin")) {
						pos := pf.fset.Position(be.Pos())
						findings = append(findings, pf.rel+":"+itoaArch(pos.Line)+": function "+fd.Name.Name+" subtracts "+name)
						return false
					}
				}
				return true
			})
		}
	}

	for _, must := range []string{"Render", "RenderTo", "renderDocument"} {
		if !declared[must] {
			t.Fatalf("presence precondition (D-000.9): package folio8 no longer declares %q — this guard would pass on a deleted package", must)
		}
	}
	if layoutOriginsCalls == 0 {
		t.Fatal("presence precondition: package folio8 never calls internal/layout.Origins — \"band origins come from internal/layout\" is not merely unproven, it is false, and the absence-of-arithmetic assertion below would pass on a package that places no bands at all")
	}

	if len(findings) > 0 {
		t.Fatalf(
			"AC2 violation — package folio8 computes a band origin:\n%s\n\n"+
				"AD-24: \"Bands are placed on the page by internal/layout alone.\" Package folio8 reads the "+
				"page SETUP (pageGeometryOf) and asks internal/layout.Origins for the three origins; it "+
				"derives none of them itself.",
			strings.Join(findings, "\n"))
	}
}

// chainMentions reports whether any operand of the subtraction chain
// rooted at e prints with sub as a component — used to distinguish
// `page.Margin.Top` from an unrelated `.Top`.
func chainMentions(e ast.Expr, sub string) bool {
	for _, op := range subtractionOperands(e) {
		if strings.Contains(exprString(op), sub) {
			return true
		}
	}
	return false
}

func itoaArch(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
