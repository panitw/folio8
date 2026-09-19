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

// RuleNoFloatTypedValue is this guard's stable rule id, declared beside
// the checker in the convention this package already uses
// (RuleMapRange = "map-range", RuleNoCompressor = "no-compressor-import").
const RuleNoFloatTypedValue = "no-float-typed-value"

// FloatTypedStats reports what ScanFloatTypedValues actually examined,
// taken from the scanner's OWN execution rather than a second,
// independent walk (Major 5 of Story 1.3's QA review; the same reasoning
// MapRangeStats and folio-go's noFloat64Stats carry). A vacuity guard
// built by re-deriving "there were files under this root" a different
// way cannot see a scanner that silently does nothing: injecting
// `if true { return nil, FloatTypedStats{}, nil }` as this checker's
// first statement never touches an unrelated second walk, but it DOES
// zero every field below.
//
// TypedExprs — a count of expressions whose type actually RESOLVED — is
// the specific statistic that makes "a checker that resolved nothing"
// visible. A checker that loads packages but obtains no type information
// reports zero findings exactly as a clean scan does; it also reports
// zero typed expressions, which a clean scan never does.
type FloatTypedStats struct {
	// DirsVisited lists, by name and relative to the scanned root, every
	// directory a parsed file was found in ("." is the root package).
	DirsVisited []string
	// FilesParsed counts the syntax files the loader handed back.
	FilesParsed int
	// TypedExprs counts expressions whose type resolved.
	TypedExprs int
}

// ScanFloatTypedValues is the AD-23 guard the existing syntactic one
// cannot be: a pure function over a target directory returning
// (findings, stats, error), with no *testing.T parameter, no hard-coded
// root and no repo-root discovery inside it (AC1 of Story 1.3, D-1.3.6),
// so the same function can be pointed at the real tree and at a fixture
// tree.
//
// WHY IT EXISTS (D-000.25, Finding 1). AD-23 promises "no float
// arithmetic under internal/" and folio-go/internal/arch_test.go
// delivers "no float IDENTIFIERS, and no floating-point literals". Those
// differ by exactly one thing that matters: a value whose float type is
// INFERRED. `int64(someVendorCall())` names neither banned identifier
// and contains no floating-point literal, so the syntactic scanner walks
// straight past it. Measured at 431a6a5: the syntactic guard reported
// zero under folio-go while four float-typed value expressions stood in
// internal/fontset/fontset.go.
//
// DETECTION IS BY TYPE, NEVER BY SPELLING, AND COVERS THE CLASS RATHER
// THAN THE INSTANCE (D-000.23). The predicate is exactly: an ast.Expr
// whose TypesInfo.Types[e] satisfies tv.IsValue(), and whose type's
// underlying *types.Basic carries types.IsFloat. This checker names no
// accessor, no package and no symbol — a list of known float-returning
// functions is the rotting-list pattern (D-2.1.3, D-000.15) and would
// miss the next one by construction.
//
// It resolves the target subtree as ONE coherent package graph via
// go/packages, with the same Mode ScanMapRange uses, for the reason
// D-1.3.11 gives: a one-directory-at-a-time type-check with a tolerant
// importer resolves a cross-package type to "no information" and
// silently reports nothing. golang.org/x/tools is lint's own dependency
// and never touches folio-go's module graph.
//
// includeTests selects packages.Config.Tests. AD-23's existing scope
// INCLUDES _test.go files (folio-go's walkGoFiles skips only testdata
// and dot-directories), so a rule that could not see them would be
// strictly weaker in file scope than the guard it strengthens.
//
// FAILING LOUDLY, TWO WAYS (D-1.3.11):
//
//   - packages.Load's nil top-level error is not sufficient: every
//     package's own Errors field is inspected and any entry fails the
//     scan, rather than being reported as zero findings.
//   - An expression the type checker RECORDED but could not resolve —
//     a nil or types.Invalid type in TypesInfo.Types — is a hard error,
//     not a silent skip. Note the precise subject: absence from the
//     Types map is NOT that condition and is NOT an error. go/types
//     records types only for expressions that are values or types;
//     identifiers on the left of a declaration go to Defs, a selector's
//     field name and a struct field's name are recorded nowhere. Those
//     are not unresolved expressions, they are not expressions of the
//     kind this map describes, and treating them as failures would make
//     the checker fail on every well-formed Go file in the repository.
//     The genuine "did not type-check" case is caught by the package
//     Errors sweep above, and by the Invalid check below.
//
// No reachability or dataflow analysis is attempted: AD-23 is a hazard
// statement, not a mechanism — the same posture ScanMapRange takes
// (D-1.3.5).
//
// Findings are deduplicated by (path, line). go/packages loads a package
// and its external/internal test variants separately when Tests is true,
// so one source line is legitimately visited more than once; the
// duplication is a loader artifact, not a second site.
func ScanFloatTypedValues(root string, includeTests bool) ([]Finding, FloatTypedStats, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Dir:   root,
		Tests: includeTests,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, FloatTypedStats{}, fmt.Errorf("load packages under %s: %w", root, err)
	}

	var loadErrs []string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			loadErrs = append(loadErrs, fmt.Sprintf("%s: %s", p.PkgPath, e.Error()))
		}
	})
	if len(loadErrs) > 0 {
		return nil, FloatTypedStats{}, fmt.Errorf(
			"type information unavailable under %s — failing loudly rather than silently reporting zero findings (D-1.3.11): %s",
			root, strings.Join(loadErrs, "; "))
	}

	var stats FloatTypedStats
	dirsSeen := map[string]bool{}
	filesSeen := map[string]bool{}

	// Keyed by (relative path, line) so the loader's package/test-variant
	// double-load collapses to one finding per source line. Ordering is
	// never taken from this map: findings are collected into a slice and
	// sorted explicitly below, so the same tree always produces the same
	// report in the same order.
	seen := map[[2]string]bool{}
	var findings []Finding

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			fname := pkg.Fset.Position(file.Pos()).Filename
			rel, rerr := filepath.Rel(root, fname)
			if rerr != nil {
				return nil, FloatTypedStats{}, fmt.Errorf("relativize %s to %s: %w", fname, root, rerr)
			}
			rel = filepath.ToSlash(rel)
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
					// Not an expression go/types records a type for
					// (a definition ident, a field name, a selector's
					// Sel). See the doc comment: this is not the
					// "unresolved" case.
					return true
				}
				pos := pkg.Fset.Position(expr.Pos())
				if tv.IsBuiltin() {
					// go/types documents that Types[e].Type is Invalid
					// for an identifier denoting a built-in function
					// (len, append, make, …) unless the call is a
					// conversion. That is the checker working, not the
					// checker failing, so it is excluded BY CATEGORY —
					// tv.IsBuiltin(), never a list of builtin names —
					// before the unresolved-type check below. Measured
					// at 431a6a5: `len(buf)` in
					// folio-go/internal/template/decimal.go:260 is one
					// such site. A builtin is never a value expression,
					// so it can never be the float this rule looks for.
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
				if !tv.IsValue() {
					return true
				}
				basic, isBasic := tv.Type.Underlying().(*types.Basic)
				if !isBasic || basic.Info()&types.IsFloat == 0 {
					return true
				}
				key := [2]string{rel, itoa(pos.Line)}
				if seen[key] {
					return true
				}
				seen[key] = true
				findings = append(findings, Finding{
					Path: rel,
					Rule: RuleNoFloatTypedValue,
					Line: pos.Line,
					Message: fmt.Sprintf(
						"%s:%d:%d: this value expression has floating-point type %s (resolved by go/types, not spelled in the source) — "+
							"AD-23 forbids float arithmetic under folio-go, and the syntactic no-float64 guard cannot see an inferred float",
						rel, pos.Line, pos.Column, tv.Type.String()),
				})
				return true
			})
			if walkErr != nil {
				return nil, FloatTypedStats{}, walkErr
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

// itoa renders a non-negative int in base ten, used only to build the
// deduplication key above.
func itoa(n int) string {
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
