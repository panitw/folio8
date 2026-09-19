package rules

import (
	"fmt"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// RuleVisibilityComputationSignatureClosed is this guard's stable rule id.
const RuleVisibilityComputationSignatureClosed = "visibility-computation-signature-closed"

// VisibilityComputationSignatureCoverageStatement is D-000.23-style
// labelling: this rule pins computeVisibility's resolved PARAMETER
// TYPE LIST to a literal — nothing else, and in particular it makes no
// claim about which of those types page state could reach through.
//
// Story 3.5 finisher review, Finding 2 (Major), ruled by the engineering
// lead: AC9's own text asked for "a signature that cannot receive
// page-derived state — a compile-time anchor," but no type system
// forbids adding a parameter to a function, and a distinct input type
// does not close that either (pageCount is an int; nothing stops
// someone adding "pageCount int" to a struct or a signature). The
// achievable — and STRONGER — instrument is set-equality on the whole
// parameter list: ANY input growth reddens this rule, page-derived or
// not, because that is the real property AD-24/AD-4 need: this
// computation's inputs are CLOSED. A reachability analysis ("can a
// page-derived value flow into this call") was explicitly declined —
// it rots, and answers a narrower question.
const VisibilityComputationSignatureCoverageStatement = "this rule pins computeVisibility's " +
	"resolved parameter type list to a literal this test owns — any growth of the input set " +
	"reddens it, page-derived or not, because closing the input set outright is a stronger and " +
	"cheaper property than proving no page-derived value can reach the function today " +
	"(D-000.68: pin to a literal when the set is permanent — nothing scheduled in Epics 4-6 " +
	"grows visibility's inputs, since AD-24 forbids row-level visibility outright)"

// expectedVisibilityComputationParams is AC9's compile-time anchor,
// restated as data: computeVisibility's parameter types, IN ORDER,
// rendered with types.RelativeTo(the defining package) so a
// same-package type prints unqualified and every cross-package type
// prints qualified by its short package name. Growing this list —
// whether the new parameter is page-derived or not — reddens the scan.
var expectedVisibilityComputationParams = []string{
	"[]bandWithOrigin",
	"bind.Value",
	"bind.Value",
	"expr.FormatContext",
}

// visibilityComputationFuncName is the function this rule pins. A
// variable, not a literal repeated at each call site, so every message
// and the production scan below stay in sync with a single rename.
const visibilityComputationFuncName = "computeVisibility"

// VisibilityComputationSignatureStats is D-000.9's coverage witness: a
// checker that never found computeVisibility must not silently report
// zero findings and look identical to a clean scan.
type VisibilityComputationSignatureStats struct {
	FilesParsed int
	ParamsSeen  int
}

// ScanVisibilityComputationSignature loads the package declared in dir
// WITH TYPE INFORMATION and asserts computeVisibility's resolved
// parameter type list is EXACTLY expectedVisibilityComputationParams
// (Story 3.5 finisher review, Finding 2 / Major). The production caller
// points dir at folio-go's own module root, where package folio8
// declares computeVisibility (render_visibility.go); a fixture
// red-proof points it at a synthetic module shaped the same way.
//
// Placed in lint, not a folio-go arch test, for the same reason
// ScanResolverMethodSet is (D-1.3.11 as extended by Story 3.3): a
// go/types-resolved signature needs the dependency this module already
// carries, and folio-go's own arch tests stay dependency-free (D-1.3.6).
func ScanVisibilityComputationSignature(dir string) ([]Finding, VisibilityComputationSignatureStats, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Dir: dir,
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, VisibilityComputationSignatureStats{}, fmt.Errorf("load package at %s: %w", dir, err)
	}

	var loadErrs []string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			loadErrs = append(loadErrs, fmt.Sprintf("%s: %s", p.PkgPath, e.Error()))
		}
	})
	if len(loadErrs) > 0 {
		return nil, VisibilityComputationSignatureStats{}, fmt.Errorf(
			"type information unavailable loading %s — failing loudly rather than "+
				"silently reporting zero findings (D-1.3.11): %s",
			dir, strings.Join(loadErrs, "; "))
	}

	var stats VisibilityComputationSignatureStats
	var findings []Finding
	found := false

	for _, pkg := range pkgs {
		stats.FilesParsed += len(pkg.Syntax)
		if pkg.Types == nil {
			continue
		}
		obj := pkg.Types.Scope().Lookup(visibilityComputationFuncName)
		if obj == nil {
			continue
		}
		fn, ok := obj.(*types.Func)
		if !ok {
			continue
		}
		sig, ok := fn.Type().(*types.Signature)
		if !ok {
			continue
		}
		found = true

		// A custom qualifier, not types.RelativeTo: RelativeTo prints a
		// non-local type's FULL IMPORT PATH ("github.com/.../internal/
		// bind.Value"), which would make expectedVisibilityComputationParams
		// an import-path literal — exactly the kind of anchor that moves
		// under an unrelated refactor (a package relocated one directory
		// deeper). Printing by the package's declared short NAME instead
		// ("bind.Value") is what the production signature and every
		// synthetic fixture package already agree on, and is what the
		// literal below is written against.
		local := pkg.Types
		qualifier := func(other *types.Package) string {
			if other == local {
				return ""
			}
			return other.Name()
		}
		params := sig.Params()
		got := make([]string, params.Len())
		for i := 0; i < params.Len(); i++ {
			got[i] = types.TypeString(params.At(i).Type(), qualifier)
		}
		stats.ParamsSeen = len(got)

		if len(got) != len(expectedVisibilityComputationParams) {
			findings = append(findings, Finding{
				Path: "render_visibility.go",
				Rule: RuleVisibilityComputationSignatureClosed,
				Message: fmt.Sprintf(
					"%s takes %d parameters (%s), want exactly %d (%s) — AC9's inputs are closed; a "+
						"widened parameter list is a direction change under AD-4, whether or not the new "+
						"parameter is itself page-derived. %s",
					visibilityComputationFuncName, len(got), strings.Join(got, ", "),
					len(expectedVisibilityComputationParams), strings.Join(expectedVisibilityComputationParams, ", "),
					VisibilityComputationSignatureCoverageStatement,
				),
			})
			continue
		}
		for i, want := range expectedVisibilityComputationParams {
			if got[i] != want {
				findings = append(findings, Finding{
					Path: "render_visibility.go",
					Rule: RuleVisibilityComputationSignatureClosed,
					Message: fmt.Sprintf(
						"%s parameter %d has resolved type %q, want %q (AC9)",
						visibilityComputationFuncName, i+1, got[i], want,
					),
				})
			}
		}
	}
	if !found {
		return nil, VisibilityComputationSignatureStats{}, fmt.Errorf(
			"%s was never found at %s", visibilityComputationFuncName, dir)
	}

	sort.Slice(findings, func(i, j int) bool { return findings[i].Message < findings[j].Message })
	return findings, stats, nil
}
