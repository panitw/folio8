package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolverMethodSetCoverageStatementWording is D-000.23-style
// labelling: the coverage witness must say, in those words, what this
// rule covers and why the AST-only sibling guard cannot.
func TestResolverMethodSetCoverageStatementWording(t *testing.T) {
	for _, want := range []string{
		"full",
		"resolved signature",
		"embedded interface",
	} {
		if !strings.Contains(ResolverMethodSetCoverageStatement, want) {
			t.Errorf("ResolverMethodSetCoverageStatement does not contain %q: %s", want, ResolverMethodSetCoverageStatement)
		}
	}
}

// TestResolverMethodSetProductionScan is AC5/AC22: the real
// internal/expr.Resolver interface, asserted to report zero findings.
func TestResolverMethodSetProductionScan(t *testing.T) {
	root := repoRootFromTest(t)
	exprDir := filepath.Join(root, "folio-go", "internal", "expr")

	findings, stats, err := ScanResolverMethodSet(exprDir)
	if err != nil {
		t.Fatalf("scan %s: %v", exprDir, err)
	}
	if stats.FilesParsed == 0 {
		t.Fatal("vacuity guard: checker's own stats report 0 files parsed")
	}
	if stats.MethodsSeen != 3 {
		t.Fatalf("vacuity guard: expected exactly 3 methods observed on the real Resolver, got %d", stats.MethodsSeen)
	}
	if len(findings) != 0 {
		t.Fatalf("expr.Resolver must be exactly the closed set with no findings, got %+v", findings)
	}
}

// TestResolverMethodSetCompliantFixtureIsClean is the negative control
// (D-000.50): an exactly-matching fixture Resolver must report zero
// findings — the rule fires on real drift, not on any Resolver-shaped
// interface it happens to see.
func TestResolverMethodSetCompliantFixtureIsClean(t *testing.T) {
	root := repoRootFromTest(t)
	fixtureDir := filepath.Join(root, "folio-go", "testdata", "lint", "resolver-method-set", "compliant")

	findings, stats, err := ScanResolverMethodSet(fixtureDir)
	if err != nil {
		t.Fatalf("scan %s: %v", fixtureDir, err)
	}
	if stats.MethodsSeen != 3 {
		t.Fatalf("vacuity guard: expected 3 methods observed on the compliant fixture, got %d", stats.MethodsSeen)
	}
	if len(findings) != 0 {
		t.Fatalf("compliant fixture must report zero findings, got %+v", findings)
	}
}

// TestResolverMethodSetRedProofWidenedSignature is AC5's red-proof
// (evasion 1, Finding 2): ProjectCollection widened with an
// offset/limit parameter, under its OWN unchanged name, must trip this
// rule even though the AST-only name-list guard
// (TestExprResolverMethodSetIsClosed, folio-go/internal/expr_arch_test.go)
// cannot see it — a name list has no notion of a parameter.
func TestResolverMethodSetRedProofWidenedSignature(t *testing.T) {
	root := repoRootFromTest(t)
	fixtureDir := filepath.Join(root, "folio-go", "testdata", "lint", "resolver-method-set", "widened-signature")

	findings, _, err := ScanResolverMethodSet(fixtureDir)
	if err != nil {
		t.Fatalf("scan %s: %v", fixtureDir, err)
	}
	if len(findings) == 0 {
		t.Fatal("RED-PROOF FAILED: a ProjectCollection widened with offset/limit produced zero findings — " +
			"AC5's own evasion (a same-named method growing a positional argument) went undetected")
	}
	found := false
	for _, f := range findings {
		if strings.Contains(f.Message, "ProjectCollection") && strings.Contains(f.Message, "resolved signature") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a finding naming ProjectCollection's widened resolved signature, got %+v", findings)
	}
}

// TestResolverMethodSetRedProofEmbeddedInterface is AC22's red-proof
// (evasion 2, Finding 2): a fourth method arriving through an EMBEDDED
// interface field must trip this rule even though BOTH the AST-only
// name-list guard and its own captured red-proof
// (TestExprResolverMethodSetRedProofFourthMethod) pass silently on it —
// an *ast.Field for an embedded interface has zero Names, but
// go/types' *types.Interface method-set expansion has no such blind
// spot (methods contributed by embedding are part of the set by
// definition).
func TestResolverMethodSetRedProofEmbeddedInterface(t *testing.T) {
	root := repoRootFromTest(t)
	fixtureDir := filepath.Join(root, "folio-go", "testdata", "lint", "resolver-method-set", "embedded-interface")

	findings, stats, err := ScanResolverMethodSet(fixtureDir)
	if err != nil {
		t.Fatalf("scan %s: %v", fixtureDir, err)
	}
	if stats.MethodsSeen != 4 {
		t.Fatalf("presence precondition: expected the embedded interface's method to be counted (4 total), got %d — the mutation did not take", stats.MethodsSeen)
	}
	found := false
	for _, f := range findings {
		if strings.Contains(f.Message, "PageIndex") {
			found = true
		}
	}
	if !found {
		t.Fatalf("RED-PROOF FAILED: a fourth method (PageIndex) contributed through an embedded interface "+
			"produced no finding naming it — AC22's own evasion (an embedded interface field, which has no "+
			"ast.Field.Names for an AST walk to see) went undetected, got %+v", findings)
	}
}

// TestResolverMethodSetMissingMethodReportsFinding is the mirror case:
// a Resolver missing one of the three required methods must be
// reported too, not only an unexpected extra one.
func TestResolverMethodSetMissingMethodReportsFinding(t *testing.T) {
	dir := t.TempDir()
	src := `package incomplete

type Value struct{}

type Resolver interface {
	Resolve(path []string) (Value, error)
	CollectionLength(path []string) (int, error)
}
`
	if err := os.WriteFile(filepath.Join(dir, "incomplete.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module incomplete\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("write fixture go.mod: %v", err)
	}

	findings, _, err := ScanResolverMethodSet(dir)
	if err != nil {
		t.Fatalf("scan %s: %v", dir, err)
	}
	found := false
	for _, f := range findings {
		if strings.Contains(f.Message, "missing required method") && strings.Contains(f.Message, "ProjectCollection") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a finding for the missing ProjectCollection method, got %+v", findings)
	}
}
