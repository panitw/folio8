package rules

import (
	"fmt"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// RuleResolverMethodSetClosed is this guard's stable rule id.
const RuleResolverMethodSetClosed = "resolver-method-set-closed"

// ResolverMethodSetCoverageStatement is D-000.23-style labelling: this
// rule covers expr.Resolver's method SET (names and full signatures,
// including any method contributed through an embedded interface) —
// nothing else. Story 3.3's own AST-only guard,
// folio-go/internal/expr_arch_test.go's TestExprResolverMethodSetIsClosed,
// covers the closed NAME set and is unaffected; this rule exists
// because that one cannot (D-3.1a.1's own precedent: "a pure AST walk
// with no type information ... cannot resolve a type identity").
const ResolverMethodSetCoverageStatement = "this rule covers expr.Resolver's method set by full " +
	"resolved signature (go/types' *types.Interface method set, which expands an embedded interface " +
	"unconditionally) — an AST name-list scan cannot see a widened signature or an embedded interface " +
	"field, because an *ast.Field for an embedded interface has no Names at all"

// expectedResolverMethods is AC22's closed set (D-3.3.5), restated with
// each method's FULL signature — not just its name — so a same-named
// method whose signature WIDENED (an added offset/limit parameter,
// AC5) is caught, and so is a fourth method contributed through an
// EMBEDDED INTERFACE on Resolver's own declaration: go/types'
// *types.Interface.NumMethods()/.Method(i) expand an embedded
// interface's methods BY DEFINITION — there is no alternate spelling
// there for an embedding to hide behind, unlike an AST walk over
// ast.InterfaceType.Methods.List, which sees an embedded field as one
// *ast.Field with zero Names and nothing to extract.
//
// Signature strings are rendered with types.RelativeTo(the expr
// package) so a same-package type (Value) prints unqualified and any
// future cross-package type in a widened signature would print
// qualified — either way, an exact string compare catches drift.
var expectedResolverMethods = map[string]string{
	"Resolve":           "func(path []string) (Value, error)",
	"CollectionLength":  "func(path []string) (int, error)",
	"ProjectCollection": "func(path []string) ([]Value, error)",
}

// ResolverMethodSetStats reports what ScanResolverMethodSet actually
// examined (D-000.9's coverage witness): a checker that never resolved
// type information, or never found the Resolver declaration at all,
// must not silently report zero findings and look identical to a clean
// scan.
type ResolverMethodSetStats struct {
	FilesParsed int
	MethodsSeen int
}

// ScanResolverMethodSet loads the package declared in dir (the
// DIRECTORY that declares a "Resolver" interface — the production
// caller points this at folio-go/internal/expr; a fixture red-proof
// points it directly at a fixture directory shaped the same way) WITH
// TYPE INFORMATION, and asserts the Resolver interface's method set is
// EXACTLY expectedResolverMethods, both by name and by full signature
// (AC5, AC22, Story 3.3 finisher pass, Finding 2).
//
// Placed in lint (not folio-go/internal/expr_arch_test.go), because
// this rule needs go/types to resolve a method's actual signature and
// to expand an embedded interface's contribution to the method set —
// exactly the dependency D-1.3.11 already settled belongs in lint, not
// in a dependency-free arch test under folio-go/ (D-1.3.6).
func ScanResolverMethodSet(dir string) ([]Finding, ResolverMethodSetStats, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Dir: dir,
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, ResolverMethodSetStats{}, fmt.Errorf("load package at %s: %w", dir, err)
	}

	var loadErrs []string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			loadErrs = append(loadErrs, fmt.Sprintf("%s: %s", p.PkgPath, e.Error()))
		}
	})
	if len(loadErrs) > 0 {
		return nil, ResolverMethodSetStats{}, fmt.Errorf(
			"type information unavailable loading %s — failing loudly rather than "+
				"silently reporting zero findings (D-1.3.11): %s",
			dir, strings.Join(loadErrs, "; "))
	}

	var stats ResolverMethodSetStats
	var findings []Finding
	found := false

	for _, pkg := range pkgs {
		stats.FilesParsed += len(pkg.Syntax)
		if pkg.Types == nil {
			continue
		}
		obj := pkg.Types.Scope().Lookup("Resolver")
		if obj == nil {
			continue
		}
		tn, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}
		iface, ok := tn.Type().Underlying().(*types.Interface)
		if !ok {
			continue
		}
		found = true

		qualifier := types.RelativeTo(pkg.Types)
		seen := map[string]bool{}
		for i := 0; i < iface.NumMethods(); i++ {
			m := iface.Method(i)
			stats.MethodsSeen++
			name := m.Name()
			if seen[name] {
				findings = append(findings, Finding{
					Path:    "internal/expr/ast.go",
					Rule:    RuleResolverMethodSetClosed,
					Message: fmt.Sprintf("expr.Resolver's method set contains method %q more than once — an embedded interface may be duplicating a name", name),
				})
				continue
			}
			seen[name] = true

			sig := types.TypeString(m.Type(), qualifier)
			want, ok := expectedResolverMethods[name]
			if !ok {
				findings = append(findings, Finding{
					Path: "internal/expr/ast.go",
					Rule: RuleResolverMethodSetClosed,
					Message: fmt.Sprintf(
						"expr.Resolver has an undeclared method %q (resolved signature %s) — AC22's closed "+
							"set is %v; a page-scoped variant under ANY spelling, including one contributed "+
							"by an embedded interface, is a direction change under AD-4 (\"no page namespace "+
							"exists, and none can be added\"), not a one-line edit here. %s",
						name, sig, sortedResolverMethodNames(), ResolverMethodSetCoverageStatement,
					),
				})
				continue
			}
			if sig != want {
				findings = append(findings, Finding{
					Path: "internal/expr/ast.go",
					Rule: RuleResolverMethodSetClosed,
					Message: fmt.Sprintf(
						"expr.Resolver.%s has resolved signature %q, want %q (AC5: neither CollectionLength "+
							"nor ProjectCollection may take a range, offset, index or limit — a widened "+
							"signature under the SAME method name is exactly the evasion this rule closes)",
						name, sig, want,
					),
				})
			}
		}
		for name := range expectedResolverMethods {
			if !seen[name] {
				findings = append(findings, Finding{
					Path:    "internal/expr/ast.go",
					Rule:    RuleResolverMethodSetClosed,
					Message: fmt.Sprintf("expr.Resolver is missing required method %q", name),
				})
			}
		}
	}
	if !found {
		return nil, ResolverMethodSetStats{}, fmt.Errorf(
			"the Resolver interface declaration was never found at %s", dir)
	}

	sort.Slice(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	return findings, stats, nil
}

func sortedResolverMethodNames() []string {
	names := make([]string, 0, len(expectedResolverMethods))
	for n := range expectedResolverMethods {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
