package rules

import (
	"fmt"
	"go/ast"
	"go/types"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// RuleNoBigFloatType is this guard's stable rule id.
const RuleNoBigFloatType = "no-bigfloat-type"

// BigFloatTypeCoverageStatement is D-000.23's required labelling,
// spelled out as an exported string so a test can assert on it directly
// rather than on a doc comment nobody re-reads. Per D-000.23: a
// denylist's coverage witness must say what it covers, in those words,
// and never claim to cover the class.
const BigFloatTypeCoverageStatement = "this rule covers math/big.Float and math/big.Rat by resolved " +
	"type identity — these two types, not the class of arbitrary-precision or binary-floating-point " +
	"types — and it is not counted as coverage for AD-23's property; Layer 1 (the behavioural " +
	"exactness oracle over SumDecimals/AvgDecimals) is"

// bannedBigMathTypes is Layer 2's DENYLIST, and the whole of it
// (D-3.1a.1, D-000.23): math/big.Float and math/big.Rat, by resolved
// type identity (Obj().Pkg().Path() + Obj().Name()), never by source
// text. big.Float is banned because it IS binary floating point,
// semantically, implemented over integers (its mantissa is a
// []Word/uint — verified against Go 1.26.0's math/big source; NO type-
// SHAPE check can catch it, only a type-IDENTITY one). big.Rat is
// banned for a DIFFERENT reason: it is exact, so Layer 1's behavioural
// oracle cannot catch it — it is wrong because it carries an unrounded
// rational and thereby dodges AD-23's defined division scale and
// round-half-to-even rule, which is a ruled property, not an
// implementation detail.
var bannedBigMathTypes = map[[2]string]bool{
	{"math/big", "Float"}: true,
	{"math/big", "Rat"}:   true,
}

// BigFloatTypeStats reports what ScanBigFloatTypes actually examined,
// taken from the scanner's OWN execution (D-000.9, D-000.23's "coverage
// witness"), mirroring FloatTypedStats' doc comment: a checker that
// obtains no type information reports zero findings exactly as a clean
// scan does, and TypedExprs is the statistic that tells the two apart.
type BigFloatTypeStats struct {
	// DirsVisited lists, by name and relative to the scanned root, every
	// directory a parsed file was found in ("." is the root package).
	DirsVisited []string
	// FilesParsed counts the syntax files the loader handed back.
	FilesParsed int
	// TypedExprs counts expressions whose type resolved (value AND type
	// positions both — a variable, field, parameter or result of a
	// banned type is a TYPE position, not a value expression, so this
	// rule cannot restrict itself to tv.IsValue() the way the
	// float-typed-VALUE rule does).
	TypedExprs int
}

// ScanBigFloatTypes is Layer 2 (D-3.1a.1): a narrow type-identity
// DENYLIST forbidding math/big.Float and math/big.Rat anywhere under
// root — scoped to the folio-go MODULE ROOT (D-3.1a.1 correction,
// verified against ScanFloatTypedValues' own shipped production caller,
// which scans the module root and asserts the public root package
// visited BY NAME), never the repository root: hashmatrix/ is
// deliberately a SEPARATE module so the float guards exclude it by
// construction (D-000.6 amendment), and its retained FMA probe needs
// floats and is the one place they are correct.
//
// Matched by RESOLVED TYPE IDENTITY (Obj().Pkg().Path() + Obj().Name()),
// never by source text: an alias, a dot-import, a renamed import, a type
// parameter instantiated at one of the two banned types, and a
// variable, field, parameter or result of that type all resolve the
// same and all trip it (AC14). This is why every ast.Expr go/types
// recorded a type for is inspected, not only value expressions —
// unlike ScanFloatTypedValues (which looks only at VALUES: an inferred
// float64 arithmetic result), a variable declared `var x big.Float` and
// never used again is exactly the kind of site this rule must still
// catch, and its type only appears in a TYPE position.
//
// Structurally identical to ScanFloatTypedValues (packages.Load, the
// same D-1.3.11 loud-failure discipline, the same dedup-by-(path,line)),
// because it is the same class of guard answering a different
// question; see that function's doc comment for the shared reasoning
// this one does not repeat.
func ScanBigFloatTypes(root string, includeTests bool) ([]Finding, BigFloatTypeStats, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Dir:   root,
		Tests: includeTests,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, BigFloatTypeStats{}, fmt.Errorf("load packages under %s: %w", root, err)
	}

	var loadErrs []string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			loadErrs = append(loadErrs, fmt.Sprintf("%s: %s", p.PkgPath, e.Error()))
		}
	})
	if len(loadErrs) > 0 {
		return nil, BigFloatTypeStats{}, fmt.Errorf(
			"type information unavailable under %s — failing loudly rather than silently reporting zero findings (D-1.3.11): %s",
			root, strings.Join(loadErrs, "; "))
	}

	var stats BigFloatTypeStats
	dirsSeen := map[string]bool{}
	filesSeen := map[string]bool{}
	seen := map[[2]string]bool{}
	var findings []Finding

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			fname := pkg.Fset.Position(file.Pos()).Filename
			rel, rerr := filepath.Rel(root, fname)
			if rerr != nil {
				return nil, BigFloatTypeStats{}, fmt.Errorf("relativize %s to %s: %w", fname, root, rerr)
			}
			rel = filepath.ToSlash(rel)
			if rel == ".." || strings.HasPrefix(rel, "../") {
				// QA review Finding 2: at includeTests=true,
				// packages.Load hands back synthesized test-main
				// packages backed by files under the Go build cache
				// (e.g. a path like
				// "../../../Library/Caches/go-build/04/..."), which
				// sit entirely outside root. They are
				// compiler-generated, not source under this scan's
				// target tree, and would otherwise pollute the AC16
				// coverage witness with cache paths instead of real
				// ones. Skipped by construction — by "outside root",
				// never by naming a specific cache path.
				continue
			}
			if !filesSeen[rel] {
				filesSeen[rel] = true
				stats.FilesParsed++
			}
			dir := filepath.ToSlash(filepath.Dir(rel))
			if !dirsSeen[dir] {
				dirsSeen[dir] = true
				stats.DirsVisited = append(stats.DirsVisited, dir)
			}

			var walkErr error
			ast.Inspect(file, func(n ast.Node) bool {
				if walkErr != nil {
					return false
				}
				expr, ok := n.(ast.Expr)
				if !ok {
					return true
				}
				tv, recorded := pkg.TypesInfo.Types[expr]
				if !recorded {
					return true
				}
				pos := pkg.Fset.Position(expr.Pos())
				if tv.IsBuiltin() {
					// Same exclusion ScanFloatTypedValues documents:
					// go/types leaves Type Invalid for a built-in
					// function identifier unless the call is a
					// conversion. Excluded by CATEGORY, before the
					// unresolved-type check below.
					return true
				}
				if tv.Type == nil || tv.Type == types.Typ[types.Invalid] {
					walkErr = fmt.Errorf(
						"%s:%d:%d: go/types recorded this expression but could not resolve its type — "+
							"failing loudly rather than silently skipping it (D-1.3.11)",
						rel, pos.Line, pos.Column)
					return false
				}
				stats.TypedExprs++

				named, ok := resolveNamedType(tv.Type)
				if !ok || named.Obj().Pkg() == nil {
					return true
				}
				key := [2]string{named.Obj().Pkg().Path(), named.Obj().Name()}
				if !bannedBigMathTypes[key] {
					return true
				}

				dedupKey := [2]string{rel, itoa(pos.Line)}
				if seen[dedupKey] {
					return true
				}
				seen[dedupKey] = true
				findings = append(findings, Finding{
					Path: rel,
					Rule: RuleNoBigFloatType,
					Line: pos.Line,
					Message: fmt.Sprintf(
						"%s:%d:%d: this expression has type %s.%s (resolved by go/types, never by source "+
							"text — an alias, a dot-import or a renamed import all resolve the same) — "+
							"AD-23 forbids it under folio-go: it is binary floating point implemented over "+
							"integers with no float field to detect structurally (Float), or it dodges "+
							"AD-23's defined division scale and round-half-to-even rule (Rat). %s",
						rel, pos.Line, pos.Column, key[0], key[1], BigFloatTypeCoverageStatement,
					),
				})
				return true
			})
			if walkErr != nil {
				return nil, BigFloatTypeStats{}, walkErr
			}
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Line < findings[j].Line
	})
	sort.Strings(stats.DirsVisited)
	return findings, stats, nil
}

// resolveNamedType unwraps at most one pointer indirection (so *big.Float
// — the shape math/big's own constructors return, e.g. big.NewFloat —
// resolves the same as a bare big.Float value) and reports the
// underlying *types.Named, if any. A type with no Named identity at all
// (a basic type, a slice, an unnamed struct, ...) can never match a
// specific package-qualified type by definition and returns ok=false.
//
// CORRECTION (QA review Finding 1, Blocker): types.Unalias is applied
// before the *types.Named assertion, and again after unwrapping a
// pointer. Since Go 1.23 (gotypesalias=1 is the default; this module
// builds with go1.26.0), go/types materialises a declared alias — e.g.
// "type Money = big.Float" — as *types.Alias, which is NOT
// *types.Named. Without unaliasing first, a value of an aliased banned
// type resolved to false here, so ScanBigFloatTypes reported ZERO
// findings with a NON-ZERO TypedExprs witness: the scan looked, found
// nothing, and was wrong — exactly the failure shape AD-23's guards
// exist to eliminate, reproduced inside the guard meant to end it.
// types.Unalias is a no-op on any type that is not itself an alias, so
// this is safe for every other case resolveNamedType already handled.
func resolveNamedType(t types.Type) (*types.Named, bool) {
	t = types.Unalias(t)
	if ptr, ok := t.(*types.Pointer); ok {
		t = types.Unalias(ptr.Elem())
	}
	named, ok := t.(*types.Named)
	return named, ok
}
