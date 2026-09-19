package arch

// Story 3.2's own structural guards: AC2 (D-3.2.2's mechanical half:
// internal/expr's own import set resolves only to the standard library
// or this module), AC6 (no exported registration path over the closed
// eight-entry table), AC7/AC8 (the table's own AST set-equality and
// C1's discharge), AC22 (D-3.2.1's forcing function: exactly one
// "Decimal" type declaration in the module, and it is in internal/expr),
// and AC23 (DW-8's other half: parseBindingPath and isValidIdent are
// absent from the module once internal/expr exists).
//
// Every count here reuses walkGoFiles (R5, D-000.14): AST, never text,
// never a filtered pipe (F4's grep trap: a text grep for "Decimal"
// also finds internal/template/ids.go:70's unrelated base36ToDecimal).

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// exprPkgRelDir is internal/expr's directory relative to moduleRoot(t).
const exprPkgRelDir = "internal/expr"

// ---------------------------------------------------------------------
// AC2 — internal/expr's import set is stdlib + first-party only
// (D-3.2.2's mechanical half; QA Finding 1, Blocker: this guard did
// not exist before this fix)
// ---------------------------------------------------------------------

// firstPartyModulePrefix is this module's own import path prefix — the
// only non-stdlib prefix AC2 permits inside internal/expr.
const firstPartyModulePrefix = "github.com/panitw/folio8/folio-go/"

// isStdlibOrFirstPartyImportPath reports whether path resolves to the
// standard library or to this module (R4, D-000.23). It touches
// neither go.mod nor the network: a standard-library import path's
// first path segment never contains a dot — domain-qualified module
// paths ("github.com/...", "golang.org/x/...") always do — which is
// the ordinary Go convention every import-path classifier relies on;
// the explicit module-prefix check covers the one first-party case a
// bare "no dot" test would (correctly, but coincidentally) also allow.
func isStdlibOrFirstPartyImportPath(path string) bool {
	if strings.HasPrefix(path, firstPartyModulePrefix) {
		return true
	}
	first := path
	if i := strings.Index(path, "/"); i >= 0 {
		first = path[:i]
	}
	return !strings.Contains(first, ".")
}

// exprImportViolationsFromFile extracts every import in one already-
// parsed internal/expr file and returns any that resolve outside the
// standard library and this module. Both the shipped guard and its
// red-proof call this SAME function (QA Finding 4, Major, forced this
// discipline onto AC22's own red-proof after a hand-duplicated copy
// was found decoupled from the guard it claimed to prove; AC2's guard
// is built with that lesson applied from the start, not duplicated).
func exprImportViolationsFromFile(rel string, file *ast.File) []string {
	var violations []string
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if !isStdlibOrFirstPartyImportPath(path) {
			violations = append(violations, fmt.Sprintf("%s: %q", rel, path))
		}
	}
	return violations
}

// TestExprImportSetIsStdlibOrFirstPartyOnly is AC2 — D-3.2.2's
// "mechanical half": every import in every non-_test.go file under
// internal/expr resolves to the standard library or to this module.
// This is what turns D-3.2.2 ("CEL and every general-purpose
// expression library REJECTED... the reason is the numeric model, not
// the parser") from a memory into a fact: the next author who adds
// `require github.com/google/cel-go` and imports it from internal/expr
// trips this guard.
//
// Coverage witness (D-000.23, R4), stated in these words: this guard
// covers internal/expr's own import set, not the module's — go.mod
// already requires github.com/boxesandglue/textshape (F9), so a
// module-wide reading of "no third-party dependency" would be a false
// red for a reason that has nothing to do with this story.
func TestExprImportSetIsStdlibOrFirstPartyOnly(t *testing.T) {
	root := moduleRoot(t)
	fset := token.NewFileSet()
	var filesParsed int
	var violations []string
	err := walkGoFiles(fset, root, func(rel string, file *ast.File) error {
		if filepath.ToSlash(filepath.Dir(rel)) != exprPkgRelDir || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		filesParsed++
		violations = append(violations, exprImportViolationsFromFile(rel, file)...)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if filesParsed == 0 {
		t.Fatal("presence precondition (D-000.9): zero non-_test.go files found under internal/expr — nothing was checked")
	}
	for _, v := range violations {
		t.Errorf("internal/expr imports outside the standard library and this module: %s — D-3.2.2 rejected every general-purpose expression library on its numeric model (not its parser), and this guard is what keeps that a fact rather than a memory", v)
	}
}

// TestExprImportSetRedProof is AC2's own red-proof (D-000.30), built to
// avoid Finding 4's shape from the start: it injects a non-first-party
// import into an in-memory parse of internal/expr source and confirms
// exprImportViolationsFromFile — the EXACT function
// TestExprImportSetIsStdlibOrFirstPartyOnly calls, never a duplicate —
// reports it.
func TestExprImportSetRedProof(t *testing.T) {
	const src = `package expr

import (
	"fmt"

	"github.com/google/cel-go/cel"
)

var _ = fmt.Sprintf
var _ = cel.NewEnv
`
	fset := token.NewFileSet()
	file, perr := parser.ParseFile(fset, "table.go", src, 0)
	if perr != nil {
		t.Fatalf("the injected source failed to parse: %v", perr)
	}
	rel := filepath.Join(exprPkgRelDir, "table.go")
	got := exprImportViolationsFromFile(rel, file)
	if len(got) == 0 {
		t.Fatal("RED-PROOF FAILED: a non-first-party import (github.com/google/cel-go/cel) was not observed as a violation")
	}
	found := false
	for _, v := range got {
		if strings.Contains(v, "github.com/google/cel-go/cel") {
			found = true
		}
	}
	if !found {
		t.Fatalf("red-proof reported violations %v, want one naming github.com/google/cel-go/cel", got)
	}
	t.Logf("red-proof: a non-first-party import is observed as a violation: %v — AC2's guard would catch it", got)
}

// ---------------------------------------------------------------------
// AC7/AC8 — the closed eight-entry function table
// ---------------------------------------------------------------------

// extractFunctionTableNamesFromFile extracts every "name: "..."" string
// found inside a package-level var named "functionTable"'s composite
// literal, from one already-parsed file.
func extractFunctionTableNamesFromFile(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, vname := range vs.Names {
				if vname.Name != "functionTable" || i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.CompositeLit)
				if !ok {
					continue
				}
				for _, elt := range lit.Elts {
					entry, ok := elt.(*ast.CompositeLit)
					if !ok {
						continue
					}
					for _, field := range entry.Elts {
						kv, ok := field.(*ast.KeyValueExpr)
						if !ok {
							continue
						}
						key, ok := kv.Key.(*ast.Ident)
						if !ok || key.Name != "name" {
							continue
						}
						bl, ok := kv.Value.(*ast.BasicLit)
						if !ok || bl.Kind != token.STRING {
							continue
						}
						if unquoted, err := strconv.Unquote(bl.Value); err == nil {
							names = append(names, unquoted)
						}
					}
				}
			}
		}
	}
	return names
}

// exprEightFunctionNames is the closed set AC5 declares — pinned here,
// independently of internal/expr/table.go's own source, so this guard
// actually compares two independently-stated lists rather than
// tautologically re-deriving one from the other.
var exprEightFunctionNames = map[string]bool{
	"sum": true, "count": true, "avg": true,
	"formatDate": true, "formatNumber": true,
	"upper": true, "lower": true, "if": true,
}

// TestExprFunctionTableIsExactlyEight is AC7: the registered names,
// extracted by AST from internal/expr's own functionTable literal,
// are exactly FR18's eight — never zero (D-000.9's presence
// precondition: an extractor that finds nothing must not read as "the
// set matched").
func TestExprFunctionTableIsExactlyEight(t *testing.T) {
	root := moduleRoot(t)
	fset := token.NewFileSet()
	var filesParsed int
	var found bool
	var names []string
	err := walkGoFiles(fset, root, func(rel string, file *ast.File) error {
		filesParsed++
		if filepath.ToSlash(filepath.Dir(rel)) != exprPkgRelDir || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		got := extractFunctionTableNamesFromFile(file)
		if got != nil {
			found = true
			names = append(names, got...)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if filesParsed == 0 {
		t.Fatal("vacuity guard (D-000.9): zero files parsed under the module root")
	}
	if !found {
		t.Fatal("AC7 presence precondition: the functionTable literal was never found — extraction found nothing")
	}
	if len(names) == 0 {
		t.Fatal("AC7 presence precondition (D-000.9): zero entries extracted from functionTable")
	}
	if len(names) != len(exprEightFunctionNames) {
		t.Errorf("functionTable has %d entries, want %d: %v", len(names), len(exprEightFunctionNames), names)
	}
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			t.Errorf("duplicate function name %q in functionTable", n)
		}
		seen[n] = true
		if !exprEightFunctionNames[n] {
			t.Errorf("unexpected function name %q in functionTable — FR18's eight are %v", n, exprEightFunctionNames)
		}
	}
	for want := range exprEightFunctionNames {
		if !seen[want] {
			t.Errorf("functionTable is missing %q", want)
		}
	}
}

// TestExprFunctionTableRedProofNinthEntry is AC8: C1's requirement,
// discharged concretely. A ninth entry, injected into a SCRATCH COPY
// of internal/expr/table.go's source (never the committed file, same
// discipline as bind's TestBindResolutionRootsClosureRedProof), must
// change what TestExprFunctionTableIsExactlyEight's own extraction
// observes — proving that adding a function to the table without also
// touching this guard's expected set is exactly what goes red.
func TestExprFunctionTableRedProofNinthEntry(t *testing.T) {
	root := moduleRoot(t)
	path := filepath.Join(root, exprPkgRelDir, "table.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	marker := `{name: "if", arity: 3, args: []argKind{argCondition, argAny, argAny}, ret: returnAny{}},`
	if !strings.Contains(string(src), marker) {
		t.Fatalf("presence precondition: table.go no longer contains the expected \"if\" entry line — this red-proof's injection point is stale")
	}
	injected := `{name: "frobnicate", arity: 1, args: []argKind{argAny}, ret: returnString{}},`
	mutated := strings.Replace(string(src), marker, marker+"\n\t"+injected, 1)

	fset := token.NewFileSet()
	file, perr := parser.ParseFile(fset, "table.go", mutated, 0)
	if perr != nil {
		t.Fatalf("the mutated source failed to parse — the injection is malformed: %v", perr)
	}
	got := extractFunctionTableNamesFromFile(file)

	found9 := false
	for _, n := range got {
		if n == "frobnicate" {
			found9 = true
		}
	}
	if !found9 {
		t.Fatalf("presence precondition: mutation was supposed to add \"frobnicate\" but extraction over the mutated source observed %v", got)
	}
	if len(got) != len(exprEightFunctionNames) {
		t.Logf("red-proof: mutated functionTable now has %d entries (%v) — TestExprFunctionTableIsExactlyEight's own len(names) != 8 check would fail on this source, exactly as AC8 requires", len(got), got)
		return
	}
	t.Fatal("RED-PROOF FAILED: a ninth table entry did not change the extracted count — AC7's guard would not catch it")
}

// ---------------------------------------------------------------------
// AC22 (Story 3.3) — no per-page vocabulary, asserted by SET EQUALITY
// over expr.Resolver's method set, not a name list (a name list dies
// to the first synonym — AD-4's page/pages fence).
// ---------------------------------------------------------------------

// exprResolverMethodNames is the closed set R1/AC22 declares — pinned
// here, independently of internal/expr/ast.go's own source, exactly
// the same "two independently-stated lists" discipline
// exprEightFunctionNames above uses.
var exprResolverMethodNames = map[string]bool{
	"Resolve": true, "CollectionLength": true, "ProjectCollection": true,
}

// extractResolverInterfaceMethods AST-extracts the method names of the
// interface type named "Resolver" declared in file, or nil if this
// file does not declare one.
func extractResolverInterfaceMethods(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != "Resolver" {
				continue
			}
			iface, ok := ts.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}
			for _, m := range iface.Methods.List {
				for _, n := range m.Names {
					names = append(names, n.Name)
				}
			}
		}
	}
	return names
}

// TestExprResolverMethodSetIsClosed is Story 3.3's AC22: expr.Resolver
// declares EXACTLY {Resolve, CollectionLength, ProjectCollection} —
// never a fourth method (a page-scoped variant, under any spelling,
// AD-4) — checked by AST set equality against exprResolverMethodNames,
// in the SAME instrument (this file, walkGoFiles over the whole module
// root) as TestExprFunctionTableIsExactlyEight above.
func TestExprResolverMethodSetIsClosed(t *testing.T) {
	root := moduleRoot(t)
	fset := token.NewFileSet()
	var filesParsed int
	var found bool
	var names []string
	err := walkGoFiles(fset, root, func(rel string, file *ast.File) error {
		filesParsed++
		if filepath.ToSlash(filepath.Dir(rel)) != exprPkgRelDir || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		got := extractResolverInterfaceMethods(file)
		if got != nil {
			found = true
			names = append(names, got...)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if filesParsed == 0 {
		t.Fatal("vacuity guard (D-000.9): zero files parsed under the module root")
	}
	if !found {
		t.Fatal("AC22 presence precondition: the Resolver interface declaration was never found")
	}
	if len(names) == 0 {
		t.Fatal("AC22 presence precondition (D-000.9): zero methods extracted from the Resolver interface")
	}
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			t.Errorf("duplicate method name %q in Resolver's method set", n)
		}
		seen[n] = true
		if !exprResolverMethodNames[n] {
			t.Errorf("unexpected method %q on expr.Resolver — AC22's closed set is %v; a page-scoped "+
				"aggregate variant under any spelling is a direction change under AD-4, not a one-line edit here", n, exprResolverMethodNames)
		}
	}
	for want := range exprResolverMethodNames {
		if !seen[want] {
			t.Errorf("expr.Resolver is missing method %q", want)
		}
	}
}

// TestExprResolverMethodSetRedProofFourthMethod is AC22's own red-proof
// (D-000.52), same discipline as TestExprFunctionTableRedProofNinthEntry
// above: inject a FOURTH method onto a SCRATCH COPY of ast.go's
// Resolver interface declaration and confirm the extraction above
// would observe it (and so TestExprResolverMethodSetIsClosed would
// redden), never touching the committed file.
func TestExprResolverMethodSetRedProofFourthMethod(t *testing.T) {
	root := moduleRoot(t)
	path := filepath.Join(root, exprPkgRelDir, "ast.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	marker := "ProjectCollection(path []string) ([]Value, error)"
	if !strings.Contains(string(src), marker) {
		t.Fatalf("presence precondition: ast.go no longer contains the expected ProjectCollection method line — this red-proof's injection point is stale")
	}
	injected := "PageCollectionLength(path []string) (int, error)"
	mutated := strings.Replace(string(src), marker, marker+"\n\t"+injected, 1)

	fset := token.NewFileSet()
	file, perr := parser.ParseFile(fset, "ast.go", mutated, 0)
	if perr != nil {
		t.Fatalf("the mutated source failed to parse — the injection is malformed: %v", perr)
	}
	got := extractResolverInterfaceMethods(file)

	found4 := false
	for _, n := range got {
		if n == "PageCollectionLength" {
			found4 = true
		}
	}
	if !found4 {
		t.Fatalf("presence precondition: mutation was supposed to add \"PageCollectionLength\" but extraction over the mutated source observed %v", got)
	}
	for _, n := range got {
		if !exprResolverMethodNames[n] {
			t.Logf("red-proof: mutated Resolver interface now has method %q outside the closed set %v — TestExprResolverMethodSetIsClosed's own comparison would fail on this source, exactly as AC22 requires", n, exprResolverMethodNames)
			return
		}
	}
	t.Fatal("RED-PROOF FAILED: a fourth Resolver method did not appear outside the declared closed set — AC22's guard would not catch it")
}

// ---------------------------------------------------------------------
// AC6 — no exported registration path over the closed table
// ---------------------------------------------------------------------

// TestExprTableHasNoExportedRegistrationPath is AC6: "closed" means
// closed at COMPILE time. functionTable and funcEntry are both
// unexported identifiers (table.go) — Go's own export rule already
// makes them unreachable from outside internal/expr — and this guard
// checks that fact holds (nothing renamed functionTable/funcEntry to
// an exported spelling), that no exported declaration's name reads as
// a registration/mutation entry point, AND (QA Finding 9, Minor: the
// name check alone is a deny-list an exported Install/Add/Define/
// SetFunction/Extend/WithFunction would pass straight through) that no
// exported function's BODY assigns to, indexes into for assignment, or
// append()s onto functionTable — the property AC6's own words name
// directly ("adds to, mutates, or replaces the table") and an AST can
// decide without relying on what the function happens to be called.
func TestExprTableHasNoExportedRegistrationPath(t *testing.T) {
	root := moduleRoot(t)
	fset := token.NewFileSet()
	var filesParsed int
	var exported []string
	var mutators []string
	err := walkGoFiles(fset, root, func(rel string, file *ast.File) error {
		if filepath.ToSlash(filepath.Dir(rel)) != exprPkgRelDir || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		filesParsed++
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.IsExported() {
					exported = append(exported, d.Name.Name)
					if exportedFuncMutatesFunctionTable(d) {
						mutators = append(mutators, d.Name.Name)
					}
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.ValueSpec:
						for _, n := range s.Names {
							if n.IsExported() {
								exported = append(exported, n.Name)
							}
						}
					case *ast.TypeSpec:
						if s.Name.IsExported() {
							exported = append(exported, s.Name.Name)
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if filesParsed == 0 {
		t.Fatal("vacuity guard: zero non-test files found under internal/expr")
	}
	if len(exported) == 0 {
		t.Fatal("vacuity guard: zero exported declarations found under internal/expr — nothing to check")
	}
	for _, name := range exported {
		if name == "functionTable" || name == "FunctionTable" || name == "funcEntry" || name == "FuncEntry" {
			t.Errorf("internal/expr exports %q — the closed table (or its entry type) must never be reachable from outside the package", name)
		}
		lower := strings.ToLower(name)
		if strings.Contains(lower, "register") || strings.Contains(lower, "addfunction") {
			t.Errorf("internal/expr exports %q — AC6 forbids any exported registration path over the closed table", name)
		}
	}
	for _, name := range mutators {
		t.Errorf("internal/expr exports %q, whose body assigns to or append()s onto functionTable — AC6 forbids any exported function that adds to, mutates, or replaces the closed table, regardless of its name", name)
	}
}

// exportedFuncMutatesFunctionTable reports whether fd's body contains
// an assignment whose left-hand side references the identifier
// "functionTable" (the whole array, or one indexed element), or a call
// to the builtin append() whose first argument does. Either shape
// would let an exported function add to, mutate, or replace the
// closed table (AC6) regardless of what the function is named — the
// gap TestExprTableHasNoExportedRegistrationPath's name-based check
// alone cannot see (QA Finding 9, Minor).
func exportedFuncMutatesFunctionTable(fd *ast.FuncDecl) bool {
	if fd.Body == nil {
		return false
	}
	referencesFunctionTable := func(e ast.Expr) bool {
		switch v := e.(type) {
		case *ast.Ident:
			return v.Name == "functionTable"
		case *ast.IndexExpr:
			id, ok := v.X.(*ast.Ident)
			return ok && id.Name == "functionTable"
		default:
			return false
		}
	}
	mutates := false
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range v.Lhs {
				if referencesFunctionTable(lhs) {
					mutates = true
				}
			}
		case *ast.CallExpr:
			if id, ok := v.Fun.(*ast.Ident); ok && id.Name == "append" && len(v.Args) > 0 {
				if referencesFunctionTable(v.Args[0]) {
					mutates = true
				}
			}
		}
		return true
	})
	return mutates
}

// TestExprTableMutationRedProof is AC6's own red-proof (QA Finding 9):
// an exported function whose NAME gives no hint at all (no "register",
// no "add") but whose body assigns into functionTable, injected into
// an in-memory parse of internal/expr source, must be caught by
// exportedFuncMutatesFunctionTable — proving the structural check
// catches what the name-based check alone would miss.
func TestExprTableMutationRedProof(t *testing.T) {
	const src = `package expr

func Extend(e funcEntry) {
	functionTable[0] = e
}
`
	fset := token.NewFileSet()
	file, perr := parser.ParseFile(fset, "table.go", src, 0)
	if perr != nil {
		t.Fatalf("the injected source failed to parse: %v", perr)
	}
	var found bool
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != "Extend" {
			continue
		}
		found = true
		if !exportedFuncMutatesFunctionTable(fd) {
			t.Fatal("RED-PROOF FAILED: an exported function assigning to functionTable[0] was not detected as a mutator")
		}
	}
	if !found {
		t.Fatal("presence precondition: the injected \"Extend\" function declaration was not found in the parsed source")
	}
	// Control: the innocuous name itself ("Extend") would NOT have
	// been caught by the old name-based deny-list (it contains
	// neither "register" nor "addfunction"), confirming this red-proof
	// exercises the NEW structural half, not the pre-existing one.
	if strings.Contains(strings.ToLower("Extend"), "register") || strings.Contains(strings.ToLower("Extend"), "addfunction") {
		t.Fatal("test fixture error: \"Extend\" must NOT match the name-based deny-list, or this red-proof does not isolate the structural check")
	}
}

// ---------------------------------------------------------------------
// AC22 — D-3.2.1's forcing function: exactly one "Decimal" declaration
// ---------------------------------------------------------------------

// decimalDeclLocations finds every package directory (relative to
// root) under which a top-level "type Decimal" is declared. It is a
// thin, override-free call onto decimalDeclLocationsFrom — the ONE
// extraction body both the production guard and its red-proof share
// (QA Finding 4, Major: an earlier version hand-duplicated this body a
// second time for the red-proof alone, so crippling the shipped
// extraction left the red-proof green and self-congratulatory; fixed
// by threading the override through this function instead of copying
// it, exactly as the finding's suggested resolution asked).
func decimalDeclLocations(root string) (locations []string, filesParsed int, err error) {
	return decimalDeclLocationsFrom(root, "", nil)
}

// decimalDeclLocationsFrom is decimalDeclLocations' actual extraction
// body. When overrideRel is non-empty, the one file whose root-relative
// path matches it is parsed from overrideSrc instead of read from disk
// — used only by TestDecimalUniquenessRedProof, via
// decimalDeclLocationsWithOverride below, to inject a second "Decimal"
// declaration into a real package without writing to disk. The
// production guard (decimalDeclLocations) always calls this with an
// empty override, so both callers run the identical scan-and-extract
// logic — the property Finding 4 required.
func decimalDeclLocationsFrom(root, overrideRel string, overrideSrc []byte) (locations []string, filesParsed int, err error) {
	fset := token.NewFileSet()
	err = walkGoFiles(fset, root, func(rel string, file *ast.File) error {
		filesParsed++
		useFile := file
		if overrideRel != "" && filepath.ToSlash(rel) == filepath.ToSlash(overrideRel) {
			f2, perr := parser.ParseFile(fset, rel, overrideSrc, 0)
			if perr != nil {
				return perr
			}
			useFile = f2
		}
		dir := filepath.ToSlash(filepath.Dir(rel))
		for _, decl := range useFile.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name.Name != "Decimal" {
					continue
				}
				locations = append(locations, dir)
			}
		}
		return nil
	})
	return locations, filesParsed, err
}

// TestExactlyOneDecimalDeclarationInTheModule is AC22/D-3.2.1: exactly
// one "Decimal" type declaration exists in the module, and it is in
// internal/expr.
func TestExactlyOneDecimalDeclarationInTheModule(t *testing.T) {
	root := moduleRoot(t)
	locations, filesParsed, err := decimalDeclLocations(root)
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if filesParsed == 0 {
		t.Fatal("vacuity guard: zero files parsed under the module root")
	}
	if len(locations) == 0 {
		t.Fatal("presence precondition: no \"type Decimal\" declaration found anywhere in the module")
	}
	if len(locations) != 1 {
		t.Fatalf("expected exactly ONE \"Decimal\" declaration in the module, found %d: %v", len(locations), locations)
	}
	if locations[0] != exprPkgRelDir {
		t.Fatalf("the module's one \"Decimal\" declaration lives in %q, want %q", locations[0], exprPkgRelDir)
	}
}

// TestDecimalDeclarationScanSkipsTestdataDecoys is AC22's own witness:
// four decoy "type Decimal" declarations live under
// testdata/arch/reducer-inventory/ (F4) — Go excludes testdata/ from
// the build, so they are genuinely not a second Decimal in the module,
// and walkGoFiles' own testdata skip (internal/arch_test.go:115) is
// why TestExactlyOneDecimalDeclarationInTheModule does not see them.
// This test makes that fact explicit rather than implicit: it asserts
// the walk never visits a path containing a "testdata" segment at all.
func TestDecimalDeclarationScanSkipsTestdataDecoys(t *testing.T) {
	root := moduleRoot(t)
	fset := token.NewFileSet()
	var visited int
	err := walkGoFiles(fset, root, func(rel string, _ *ast.File) error {
		visited++
		for _, seg := range strings.Split(filepath.ToSlash(rel), "/") {
			if seg == "testdata" {
				t.Fatalf("walkGoFiles visited a path under a \"testdata\" directory: %s — the skip is not load-bearing here, it is broken", rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if visited == 0 {
		t.Fatal("vacuity guard: zero files visited")
	}
	// Presence precondition on the decoys themselves: if this fixture
	// tree is ever deleted, this test would pass vacuously (nothing to
	// skip), so confirm the decoys still exist on disk (unparsed by
	// this check, deliberately — the point is that the SCAN never
	// reaches them, not that they are absent).
	decoyDir := filepath.Join(root, "testdata", "arch", "reducer-inventory")
	info, statErr := os.Stat(decoyDir)
	if statErr != nil || !info.IsDir() {
		t.Fatalf("presence precondition: %s must exist (F4's decoy fixtures) for this witness to mean anything: %v", decoyDir, statErr)
	}
}

// TestDecimalUniquenessRedProof is D-3.2.1's own addition to the
// ruling: since the testdata decoys CANNOT red-prove the
// Decimal-uniqueness half (walkGoFiles skips them by design), this
// test supplies the proof the decoys cannot: it declares a SECOND
// "type Decimal" inside an in-memory mutated copy of a REAL package's
// source (never written to disk) and observes decimalDeclLocations
// over that mutated tree reporting two locations.
func TestDecimalUniquenessRedProof(t *testing.T) {
	root := moduleRoot(t)
	targetRel := filepath.Join("internal", "bind", "value.go")
	targetPath := filepath.Join(root, targetRel)
	src, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read %s: %v", targetPath, err)
	}
	mutated := string(src) + "\n\n// zzRedProofSecondDecimal is D-3.2.1's red-proof: a second \"Decimal\"\n// type declaration, injected only into this in-memory copy.\ntype zzRedProofSecondDecimalMarker struct{}\ntype Decimal struct{ x int }\n"

	fset := token.NewFileSet()
	if _, perr := parser.ParseFile(fset, "value.go", mutated, 0); perr != nil {
		t.Fatalf("the mutated source failed to parse — the injection is malformed: %v", perr)
	}

	// Re-scan the whole module, but with targetRel's content overridden
	// in memory — mirrors bind's own override-one-file pattern
	// (resolution_roots_arch_test.go's observedResolutionRootsAcrossPackage).
	locations, filesParsed, scanErr := decimalDeclLocationsWithOverride(root, targetRel, []byte(mutated))
	if scanErr != nil {
		t.Fatalf("mutated scan: %v", scanErr)
	}
	if filesParsed == 0 {
		t.Fatal("vacuity guard: zero files parsed")
	}
	if len(locations) < 2 {
		t.Fatalf("RED-PROOF FAILED: injecting a second \"type Decimal\" into a REAL package did not produce a second observed location: %v", locations)
	}
	t.Logf("red-proof: a second real \"Decimal\" declaration is observed at %v — the uniqueness guard would catch it", locations)
}

// decimalDeclLocationsWithOverride is decimalDeclLocations, except
// exactly one file's content is substituted in memory rather than read
// from disk — used only by the red-proof above, never by the
// production guard, which scans the tree exactly as committed. It is
// now a thin call onto decimalDeclLocationsFrom, the SAME body
// decimalDeclLocations itself calls (Finding 4's fix): there is no
// second, hand-copied extraction here for a future edit to silently
// diverge from.
func decimalDeclLocationsWithOverride(root, overrideRel string, overrideSrc []byte) (locations []string, filesParsed int, err error) {
	return decimalDeclLocationsFrom(root, overrideRel, overrideSrc)
}

// ---------------------------------------------------------------------
// AC23 — DW-8's other half: parseBindingPath/isValidIdent are absent
// ---------------------------------------------------------------------

// TestParseBindingPathAndIsValidIdentAreAbsent is AC23: an extinction
// guard over AST-extracted function declarations — the correct
// instrument here (unlike AC22, where D-3.2.1 rules location is the
// property and extinction is wrong, since Decimal must exist
// somewhere): these two functions must exist NOWHERE in the module
// once internal/expr's parser replaces them (D-1.6.5, DW-8).
func TestParseBindingPathAndIsValidIdentAreAbsent(t *testing.T) {
	root := moduleRoot(t)
	fset := token.NewFileSet()
	var filesParsed int
	found := map[string][]string{} // name -> "rel:line"
	err := walkGoFiles(fset, root, func(rel string, file *ast.File) error {
		filesParsed++
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv != nil {
				continue
			}
			if fd.Name.Name == "parseBindingPath" || fd.Name.Name == "isValidIdent" {
				pos := fset.Position(fd.Pos())
				found[fd.Name.Name] = append(found[fd.Name.Name], fmt.Sprintf("%s:%d", rel, pos.Line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if filesParsed == 0 {
		t.Fatal("vacuity guard: zero files parsed under the module root")
	}
	for _, name := range []string{"parseBindingPath", "isValidIdent"} {
		if locs, ok := found[name]; ok {
			t.Errorf("%s must be ABSENT from the module (DW-8, AC23) — still found at %v", name, locs)
		}
	}
}
