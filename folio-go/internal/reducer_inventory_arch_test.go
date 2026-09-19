package arch

// Story 3.1a's AC23/AC24: the reducer inventory tripwire that makes
// Story 3.3 unable to skip routing sum/avg through the exact kernel
// this story builds (internal/expr's SumDecimals/AvgDecimals — moved
// there from internal/bind at Story 3.2/D-3.2.1; F12, Story 3.3: this
// comment named the pre-3.2 location and was never updated).
//
// D-3.1a.3 (the ruling this file exists to satisfy): the tripwire's
// location clause is RELATIONAL — "the same package as the Decimal type
// declaration" — never a literal path such as "internal/bind". Story
// 3.2 moves Decimal to internal/expr (DW-8) and the reducers travel
// with it; a literal-path tripwire would become a Story 3.2 EDIT, and a
// tripwire whose expected value must be edited is one that gets edited
// wrongly. Stated relationally, this file survives that move with zero
// changes, AND gains a second real failure mode: it also fails if
// Decimal moves and the reducers are left behind, which is a genuine
// way to end up with two accumulators.
//
// This is pure AST, no type-checking (go/types), matching every other
// guard in this package — the predicate ("a function shaped exactly
// like []Decimal -> (Decimal, error)") is entirely a matter of syntax
// once qualified references are resolved through each file's own
// import aliases, never by literal source text.

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
)

// decimalReducer is one function the scan found reducing a sequence of
// Decimal to (Decimal, error).
type decimalReducer struct {
	Name   string
	PkgDir string // relative to the scanned root; "." is the root package
	File   string // relative to the scanned root
	Line   int
}

// decimalReducerInventoryStats reports what reducerDecimalInventory
// actually examined, from the scanner's OWN execution — see
// noFloat64Stats' doc comment (this package) for why a second,
// independently-derived walk cannot be trusted as a vacuity guard: it
// cannot see a scanner that silently does nothing.
type decimalReducerInventoryStats struct {
	FilesParsed int
	DeclFound   bool
	// DeclPkgDir is the directory (relative to the scanned root) that
	// declares "type Decimal", once DeclFound is true.
	DeclPkgDir string
}

// reducerImportAliases maps each import's local name (its explicit
// alias, or the last path segment when unaliased) to its full import
// path, mirroring lint's importAliases — so a qualified reference
// (bind.Decimal, or a renamed import) is matched by the package it
// actually resolves to, never by literal source text.
func reducerImportAliases(file *ast.File) map[string]string {
	aliases := map[string]string{}
	for _, imp := range file.Imports {
		p := strings.Trim(imp.Path.Value, `"`)
		name := p
		if idx := strings.LastIndex(p, "/"); idx != -1 {
			name = p[idx+1:]
		}
		if imp.Name != nil {
			name = imp.Name.Name
		}
		aliases[name] = p
	}
	return aliases
}

// countFields counts the effective number of parameters/results a
// *ast.FieldList's Field slice describes: an unnamed field is one slot,
// a named field groups len(Names) slots under one type (Go's
// "a, b int" shorthand).
func countFields(fields []*ast.Field) int {
	n := 0
	for _, f := range fields {
		if len(f.Names) == 0 {
			n++
		} else {
			n += len(f.Names)
		}
	}
	return n
}

// flattenFieldTypes expands a *ast.FieldList's Field slice into one
// ast.Expr per effective slot, repeating a grouped field's Type once
// per name it covers.
func flattenFieldTypes(fields []*ast.Field) []ast.Expr {
	var out []ast.Expr
	for _, f := range fields {
		cnt := 1
		if len(f.Names) > 0 {
			cnt = len(f.Names)
		}
		for i := 0; i < cnt; i++ {
			out = append(out, f.Type)
		}
	}
	return out
}

// exprIsDecimal reports whether e denotes the Decimal type declared at
// declPkgDir/declImportPath — a bare identifier only when the
// expression's own file sits in the SAME package directory as the
// declaration, and a qualified reference (pkg.Decimal) otherwise,
// resolved through that file's own import aliases, never by source
// text. Factored out of the pass-2 closure (QA review Finding 4) so the
// slice-alias pre-pass below can resolve the SAME identity question a
// declared alias's right-hand side names.
func exprIsDecimal(e ast.Expr, dir string, aliases map[string]string, declPkgDir, declImportPath string) bool {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name == "Decimal" && dir == declPkgDir
	case *ast.SelectorExpr:
		if x.Sel.Name != "Decimal" {
			return false
		}
		pkgIdent, ok := x.X.(*ast.Ident)
		if !ok {
			return false
		}
		path, known := aliases[pkgIdent.Name]
		return known && path == declImportPath
	default:
		return false
	}
}

// paramElemType reports the element type of a parameter spelled as a
// slice ([]Decimal) or as a variadic parameter (...Decimal) — the two
// AST shapes Go uses for "a sequence of Decimal" as a single formal
// parameter. QA review Finding 4 (Major): the original matcher only
// recognised *ast.ArrayType, so a variadic parameter — the single
// likeliest spelling for a function table's own sum(...) — walked
// straight past it, being an *ast.Ellipsis node instead.
func paramElemType(t ast.Expr) (ast.Expr, bool) {
	switch x := t.(type) {
	case *ast.ArrayType:
		if x.Len != nil {
			return nil, false // a fixed-size array, not a slice
		}
		return x.Elt, true
	case *ast.Ellipsis:
		return x.Elt, true
	default:
		return nil, false
	}
}

// pkgImportPathForDir resolves a scan-relative package directory ("."
// for the root package) to its full import path under moduleImportPath.
func pkgImportPathForDir(dir, moduleImportPath string) string {
	if dir == "." {
		return moduleImportPath
	}
	return moduleImportPath + "/" + dir
}

// reducerDecimalInventory finds the package under root that declares a
// type named "Decimal", then finds every top-level function OR method
// ANYWHERE under root whose signature reduces a single parameter — a
// slice of Decimal, a variadic Decimal, or a locally-declared slice
// ALIAS of Decimal (QA review Finding 4: "type Decimals = []Decimal" —
// resolved the same way a qualified Decimal reference is, never by
// source text) — to (Decimal, error). Matched by bare identifier only
// when the function's own file sits in the SAME package directory as
// the Decimal declaration, and by resolved import alias (never source
// text) otherwise, so a reducer left behind in another package after a
// hypothetical move is still visible to the scan (D-3.1a.3's second
// failure mode) even though it will then fail the location check below.
//
// A METHOD (a receiver-bearing FuncDecl) of this exact shape is matched
// too (QA review Finding 4): AC23's own wording — "the set of functions
// ... that reduce a sequence of Decimal to (Decimal, error)" — draws no
// distinction between a plain function and a method, and a probe tree
// confirmed a method of this shape reduced []Decimal to (Decimal,
// error) exactly as plainly as a function does.
//
// Residual, STATED gap (D-000.23 discipline — an admitted hole, not a
// silent one): a slice alias declared through a SECOND layer of
// indirection (an alias of an alias), or one resolvable only through
// full type-checking (e.g. behind a generic type parameter), is not
// resolved by this pure-AST pass. Nothing in folio-go does this today;
// if it ever does, this inventory needs a go/types-based pass to keep
// seeing it, and that gap must be closed deliberately, not discovered
// by a silent miss.
//
// moduleImportPath is the import path root corresponds to, so a
// qualified reference can be resolved to a directory without hard-coding
// folio-go's own module path — the production caller passes the real
// module path; a fixture-tree caller passes a synthetic one matching
// whatever its own fixture files import.
func reducerDecimalInventory(root, moduleImportPath string) ([]decimalReducer, decimalReducerInventoryStats, error) {
	fset := token.NewFileSet()
	var stats decimalReducerInventoryStats

	// Pass 1: locate the package directory declaring "type Decimal".
	err := walkGoFiles(fset, root, func(rel string, file *ast.File) error {
		stats.FilesParsed++
		dir := filepath.ToSlash(filepath.Dir(rel))
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Name.Name != "Decimal" {
					continue
				}
				if stats.DeclFound && stats.DeclPkgDir != dir {
					return fmt.Errorf(
						"%s: a second \"Decimal\" type declaration exists (first seen in %q) — "+
							"the reducer inventory cannot locate a unique owning package",
						rel, stats.DeclPkgDir)
				}
				stats.DeclFound = true
				stats.DeclPkgDir = dir
			}
		}
		return nil
	})
	if err != nil {
		return nil, stats, err
	}
	if !stats.DeclFound {
		return nil, stats, fmt.Errorf("no type named \"Decimal\" found anywhere under %s", root)
	}

	declImportPath := moduleImportPath
	if stats.DeclPkgDir != "." {
		declImportPath = moduleImportPath + "/" + stats.DeclPkgDir
	}

	// Pass 1.5 (QA review Finding 4): collect every package-level slice
	// ALIAS of Decimal — "type X = []Decimal" or "type X = []pkg.Decimal"
	// — declared anywhere under root, keyed by the declaring package's
	// import path and the alias's own name. ts.Assign.IsValid() is what
	// distinguishes a Go type ALIAS ("type X = Y", the same type) from a
	// type DEFINITION ("type X Y", a genuinely distinct type requiring
	// explicit conversion) — only the former denotes exactly []Decimal.
	sliceAliases := map[string]map[string]bool{}
	err = walkGoFiles(fset, root, func(rel string, file *ast.File) error {
		dir := filepath.ToSlash(filepath.Dir(rel))
		aliases := reducerImportAliases(file)
		pkgImportPath := pkgImportPathForDir(dir, moduleImportPath)
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || !ts.Assign.IsValid() {
					continue // not a "type X = Y" alias
				}
				arr, ok := ts.Type.(*ast.ArrayType)
				if !ok || arr.Len != nil {
					continue // not a slice alias
				}
				if !exprIsDecimal(arr.Elt, dir, aliases, stats.DeclPkgDir, declImportPath) {
					continue
				}
				if sliceAliases[pkgImportPath] == nil {
					sliceAliases[pkgImportPath] = map[string]bool{}
				}
				sliceAliases[pkgImportPath][ts.Name.Name] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, stats, err
	}

	// matchesSliceAliasOfDecimal reports whether t is a reference (bare,
	// same-package, or qualified through an import alias) to one of the
	// slice-of-Decimal aliases Pass 1.5 found.
	matchesSliceAliasOfDecimal := func(t ast.Expr, dir string, aliases map[string]string) bool {
		switch x := t.(type) {
		case *ast.Ident:
			return sliceAliases[pkgImportPathForDir(dir, moduleImportPath)][x.Name]
		case *ast.SelectorExpr:
			pkgIdent, ok := x.X.(*ast.Ident)
			if !ok {
				return false
			}
			path, known := aliases[pkgIdent.Name]
			return known && sliceAliases[path][x.Sel.Name]
		default:
			return false
		}
	}

	// Pass 2: collect every top-level function or method reducing a
	// sequence of Decimal to (Decimal, error), anywhere under root.
	var reducers []decimalReducer
	err = walkGoFiles(fset, root, func(rel string, file *ast.File) error {
		dir := filepath.ToSlash(filepath.Dir(rel))
		aliases := reducerImportAliases(file)

		matchesDecimal := func(e ast.Expr) bool {
			return exprIsDecimal(e, dir, aliases, stats.DeclPkgDir, declImportPath)
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Type.Params == nil || fn.Type.Results == nil {
				continue
			}
			if countFields(fn.Type.Params.List) != 1 {
				continue
			}
			paramType := fn.Type.Params.List[0].Type
			if elt, ok := paramElemType(paramType); ok {
				if !matchesDecimal(elt) {
					continue // slice/variadic of something other than Decimal
				}
			} else if !matchesSliceAliasOfDecimal(paramType, dir, aliases) {
				continue // not a slice-of-Decimal, variadic-Decimal, or slice-alias-of-Decimal parameter
			}
			if countFields(fn.Type.Results.List) != 2 {
				continue
			}
			results := flattenFieldTypes(fn.Type.Results.List)
			if !matchesDecimal(results[0]) {
				continue
			}
			errIdent, ok := results[1].(*ast.Ident)
			if !ok || errIdent.Name != "error" {
				continue
			}
			pos := fset.Position(fn.Pos())
			reducers = append(reducers, decimalReducer{
				Name: fn.Name.Name, PkgDir: dir, File: rel, Line: pos.Line,
			})
		}
		return nil
	})
	if err != nil {
		return nil, stats, err
	}

	sort.Slice(reducers, func(i, j int) bool {
		if reducers[i].PkgDir != reducers[j].PkgDir {
			return reducers[i].PkgDir < reducers[j].PkgDir
		}
		if reducers[i].File != reducers[j].File {
			return reducers[i].File < reducers[j].File
		}
		return reducers[i].Line < reducers[j].Line
	})
	return reducers, stats, nil
}

// decimalReducerViolations compares the scanned reducer set against
// want (the AC23-sanctioned set of names) and each reducer's PkgDir
// against declPkgDir (D-3.1a.3's location clause), returning one
// human-readable violation string per problem found — sorted, for a
// deterministic diff.
//
// QA review Finding 6 (Minor): AC23's production assertion and AC24's
// red-proofs used to run independent comparison logic — the red-proofs
// only checked the SCANNER's raw output shape (reducer count and
// names), never AC23's own verdict. A future edit to AC23's comparison
// block (e.g. dropping the count-mismatch arm) could have left both
// red-proofs green. Factoring the comparison into this one function,
// called by both the production test and its red-proofs, closes that
// gap: a red-proof now asserts the violation list itself is non-empty,
// which is asserting on AC23's actual verdict.
func decimalReducerViolations(got []decimalReducer, want map[string]bool, declPkgDir string) []string {
	var violations []string
	byName := map[string]decimalReducer{}
	for _, r := range got {
		byName[r.Name] = r
	}
	for name := range want {
		r, ok := byName[name]
		if !ok {
			violations = append(violations, fmt.Sprintf(
				"AC23 VIOLATED: expected reducer %s(...) not found anywhere in the module — "+
					"it must exist, and it must be declared in the same package as Decimal (%q)",
				name, declPkgDir))
			continue
		}
		if r.PkgDir != declPkgDir {
			violations = append(violations, fmt.Sprintf(
				"AC23 VIOLATED (D-3.1a.3): %s is declared in %q, but Decimal is declared in %q — "+
					"a reducer left behind after Decimal moves is exactly the two-accumulator hazard "+
					"this tripwire exists to catch",
				name, r.PkgDir, declPkgDir))
		}
	}
	for name, r := range byName {
		if want[name] {
			continue
		}
		violations = append(violations, fmt.Sprintf(
			"AC23 VIOLATED: unexpected reducer %s at %s:%d (package %q) reduces a sequence of Decimal "+
				"to (Decimal, error) — a second reducer must either route through the sanctioned pair "+
				"or be added to this inventory's expected set deliberately, in the same diff",
			name, r.File, r.Line, r.PkgDir))
	}
	if len(byName) != len(want) {
		violations = append(violations, fmt.Sprintf(
			"reducer count mismatch: got %d distinct name(s), want exactly %d", len(byName), len(want)))
	}
	sort.Strings(violations)
	return violations
}
