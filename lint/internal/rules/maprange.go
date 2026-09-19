package rules

import (
	"fmt"
	"go/ast"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// RuleMapRange is this guard's stable rule id (AC4).
const RuleMapRange = "map-range"

// EscapeHatch is AC15's idiom, named verbatim in every failure message.
const EscapeHatch = "for _, k := range slices.Sorted(maps.Keys(m)) { v := m[k]; … }"

// MapRangeStats reports what ScanMapRange actually examined, taken from
// the scanner's own execution rather than a second, independent walk
// (Major 5, this story's QA review): a vacuity guard built by re-deriving
// "there were files under this root" a different way cannot see a
// scanner that silently does nothing, because a dead first statement
// injected into ScanMapRange never touches a second, unrelated walk.
// TypedRangeStmts — a count of *ast.RangeStmt subjects whose type DID
// resolve — is the specific statistic that would have made Blocker 1
// (D-1.3.11) visible: a checker that resolves nothing still reports zero
// findings, but it also types zero range statements.
type MapRangeStats struct {
	DirsVisited     []string
	FilesParsed     int
	TypedRangeStmts int
}

// ScanMapRange is the AC1 pure checker for D-1.3.5's total ban on
// ranging a map in any non-test .go file under the target directory
// (AC14). Detection is exact and type-based: it flags an *ast.RangeStmt
// whose subject resolves, via go/types, to a map type — never a
// syntactic guess (RP-6). No reachability or dataflow analysis is
// attempted (AC14: the hazard statement, not the mechanism). `_test.go`
// files are outside the ban (D-1.3.5) and are never loaded in the first
// place (packages.Config.Tests is false below). Any directory named
// testdata is excluded (AC2) — golang.org/x/tools/go/packages' list
// driver applies the same "go tool ignores testdata for package matching"
// convention `go build`/`go vet` already do.
//
// D-1.3.11: the dependency-free go/types path this checker originally
// shipped with was withdrawn. It type-checked one source directory at a
// time with a tolerant importer that substituted an empty types.Package
// for any sibling internal/ package it was not also given — so a map
// type declared in one package and ranged in another (internal/pdf
// already imports internal/geom) resolved to "no information" and was
// silently never reported. D-1.3.6's binding invariant (a) is "exact
// detection with no false positives"; a best-effort path that documents
// its own inexactness is not that. This checker now resolves the whole
// target subtree as one coherent package graph via go/packages, with
// full type information for every package it loads and everything it
// imports (invariant (b) still holds: golang.org/x/tools is lint's own
// dependency and never touches folio-go's module graph). And per the
// same finding's fix: an *ast.RangeStmt subject whose type cannot be
// resolved is now a hard error, not a silent skip — see the walkErr
// handling below. A checker that can fall through to "zero findings"
// when it does not understand the code is indistinguishable from a
// clean scan; this one fails the build instead.
func ScanMapRange(root string) ([]Finding, MapRangeStats, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Dir:   root,
		Tests: false, // D-1.3.5: _test.go files are outside the ban.
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, MapRangeStats{}, fmt.Errorf("load packages under %s: %w", root, err)
	}

	// Fail loudly (D-1.3.11) rather than silently treating a package
	// that failed to load or type-check as zero findings: packages.Load
	// itself can return a nil top-level error while individual packages
	// carry list/parse/type errors in their own Errors field.
	var loadErrs []string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			loadErrs = append(loadErrs, fmt.Sprintf("%s: %s", p.PkgPath, e.Error()))
		}
	})
	if len(loadErrs) > 0 {
		return nil, MapRangeStats{}, fmt.Errorf(
			"type information unavailable under %s — failing loudly rather than silently reporting zero findings (D-1.3.11): %s",
			root, strings.Join(loadErrs, "; "))
	}

	var findings []Finding
	var stats MapRangeStats
	dirsSeen := map[string]bool{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			fname := pkg.Fset.Position(file.Pos()).Filename
			rel, rerr := filepath.Rel(root, fname)
			if rerr != nil {
				return nil, MapRangeStats{}, fmt.Errorf("relativize %s to %s: %w", fname, root, rerr)
			}
			stats.FilesParsed++
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
				rs, ok := n.(*ast.RangeStmt)
				if !ok {
					return true
				}
				tv, ok := pkg.TypesInfo.Types[rs.X]
				if !ok || tv.Type == nil {
					pos := pkg.Fset.Position(rs.Pos())
					walkErr = fmt.Errorf(
						"%s:%d: could not resolve the type of this range statement's subject — "+
							"failing loudly rather than silently skipping it (D-1.3.11)", rel, pos.Line)
					return false
				}
				stats.TypedRangeStmts++
				if _, isMap := tv.Type.Underlying().(*types.Map); isMap {
					pos := pkg.Fset.Position(rs.Pos())
					findings = append(findings, Finding{
						Path: rel, Rule: RuleMapRange, Line: pos.Line,
						Message: fmt.Sprintf("%s:%d: range over a map value is forbidden (AD-1, NFR1.d) — use instead: %s",
							rel, pos.Line, EscapeHatch),
					})
				}
				return true
			})
			if walkErr != nil {
				return nil, MapRangeStats{}, walkErr
			}
		}
	}
	return findings, stats, nil
}
