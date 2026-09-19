package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVisibilityComputationSignatureCoverageStatementWording is
// D-000.23-style labelling: the coverage witness must say, in those
// words, what this rule pins and why a reachability analysis was
// declined in its favour.
func TestVisibilityComputationSignatureCoverageStatementWording(t *testing.T) {
	for _, want := range []string{
		"parameter type list",
		"page-derived or not",
		"D-000.68",
	} {
		if !strings.Contains(VisibilityComputationSignatureCoverageStatement, want) {
			t.Errorf("VisibilityComputationSignatureCoverageStatement does not contain %q: %s", want, VisibilityComputationSignatureCoverageStatement)
		}
	}
}

// TestVisibilityComputationSignatureProductionScan is AC9's real
// anchor: the real folio-go package's computeVisibility, asserted to
// report zero findings against the literal four-parameter list this
// test owns.
func TestVisibilityComputationSignatureProductionScan(t *testing.T) {
	root := repoRootFromTest(t)
	folio8Dir := filepath.Join(root, "folio-go")

	findings, stats, err := ScanVisibilityComputationSignature(folio8Dir)
	if err != nil {
		t.Fatalf("scan %s: %v", folio8Dir, err)
	}
	if stats.FilesParsed == 0 {
		t.Fatal("vacuity guard: checker's own stats report 0 files parsed")
	}
	if stats.ParamsSeen != 4 {
		t.Fatalf("vacuity guard: expected exactly 4 parameters observed on the real computeVisibility, got %d", stats.ParamsSeen)
	}
	if len(findings) != 0 {
		t.Fatalf("computeVisibility must match the closed parameter list with no findings, got %+v", findings)
	}
}

// visibilitySignatureFixtureModule writes a minimal, self-contained Go
// module at dir with two helper packages named "bind" and "expr" (so
// types.RelativeTo prints them exactly as the real production package
// does — by short package name, not import path) and a "folio8" package
// declaring computeVisibility with the given extra source appended
// verbatim after the closed-set declaration. This lets each red-proof
// below stay a single, self-contained file rather than depending on a
// checked-in fixture tree (same technique as
// TestResolverMethodSetMissingMethodReportsFinding).
func visibilitySignatureFixtureModule(t *testing.T, funcSrc string) string {
	t.Helper()
	dir := t.TempDir()

	write := func(rel, content string) {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}

	write("go.mod", "module visibilitysignaturefixture\n\ngo 1.25\n")
	write("bind/bind.go", "package bind\n\ntype Value struct{}\n")
	write("expr/expr.go", "package expr\n\ntype FormatContext struct{}\n")
	write("main.go", `package folio8

import (
	"visibilitysignaturefixture/bind"
	"visibilitysignaturefixture/expr"
)

type template struct{}

type bandWithOrigin struct {
	band   template
	origin int
}

type visibilityVerdicts map[string]bool

// Keeps both imports "used" regardless of which types funcSrc itself
// references, so a fixture that changes fc's type away from
// expr.FormatContext (the changed-type red-proof) still compiles.
var _ = bind.Value{}
var _ = expr.FormatContext{}

`+funcSrc+"\n")

	return dir
}

// TestVisibilityComputationSignatureCompliantFixtureIsClean is the
// negative control (D-000.50): an exactly-matching fixture must report
// zero findings — the rule fires on real drift, not on any
// computeVisibility-shaped function it happens to see.
func TestVisibilityComputationSignatureCompliantFixtureIsClean(t *testing.T) {
	dir := visibilitySignatureFixtureModule(t, `func computeVisibility(bands []bandWithOrigin, data, params bind.Value, fc expr.FormatContext) (visibilityVerdicts, error) {
	return nil, nil
}`)

	findings, stats, err := ScanVisibilityComputationSignature(dir)
	if err != nil {
		t.Fatalf("scan %s: %v", dir, err)
	}
	if stats.ParamsSeen != 4 {
		t.Fatalf("vacuity guard: expected 4 parameters observed on the compliant fixture, got %d", stats.ParamsSeen)
	}
	if len(findings) != 0 {
		t.Fatalf("compliant fixture must report zero findings, got %+v", findings)
	}
}

// TestVisibilityComputationSignatureRedProofExtraParameter is AC9's
// red-proof for the exact evasion the ruling named: a widened
// parameter list, even with an ordinary, non-page-derived type
// (int), must trip this rule — the property is "this computation's
// inputs are closed," not "no page-derived value can reach it."
func TestVisibilityComputationSignatureRedProofExtraParameter(t *testing.T) {
	dir := visibilitySignatureFixtureModule(t, `func computeVisibility(bands []bandWithOrigin, data, params bind.Value, fc expr.FormatContext, pageCount int) (visibilityVerdicts, error) {
	return nil, nil
}`)

	findings, stats, err := ScanVisibilityComputationSignature(dir)
	if err != nil {
		t.Fatalf("scan %s: %v", dir, err)
	}
	if stats.ParamsSeen != 5 {
		t.Fatalf("presence precondition: expected the widened parameter to be counted (5 total), got %d — the mutation did not take", stats.ParamsSeen)
	}
	if len(findings) == 0 {
		t.Fatal("RED-PROOF FAILED: computeVisibility widened with a fifth (pageCount int) parameter produced zero findings")
	}
	found := false
	for _, f := range findings {
		if strings.Contains(f.Message, "takes 5 parameters") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a finding naming the widened 5-parameter list, got %+v", findings)
	}
}

// TestVisibilityComputationSignatureRedProofChangedType is the mirror
// case: the parameter COUNT stays 4, but one type changes (fc's type
// widened to bind.Value instead of expr.FormatContext) — a count check
// alone would miss this.
func TestVisibilityComputationSignatureRedProofChangedType(t *testing.T) {
	dir := visibilitySignatureFixtureModule(t, `func computeVisibility(bands []bandWithOrigin, data, params, fc bind.Value) (visibilityVerdicts, error) {
	return nil, nil
}`)

	findings, stats, err := ScanVisibilityComputationSignature(dir)
	if err != nil {
		t.Fatalf("scan %s: %v", dir, err)
	}
	if stats.ParamsSeen != 4 {
		t.Fatalf("presence precondition: expected 4 parameters, got %d", stats.ParamsSeen)
	}
	found := false
	for _, f := range findings {
		if strings.Contains(f.Message, "parameter 4") && strings.Contains(f.Message, `want "expr.FormatContext"`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("RED-PROOF FAILED: expected a finding naming parameter 4's changed type, got %+v", findings)
	}
}

// TestVisibilityComputationSignatureNotFoundReportsError is the mirror
// of ScanResolverMethodSet's own "declaration never found" guard: a
// package with no computeVisibility at all must fail loudly, never
// silently report zero findings and look identical to a clean scan.
func TestVisibilityComputationSignatureNotFoundReportsError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module empty\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package folio8\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	_, _, err := ScanVisibilityComputationSignature(dir)
	if err == nil {
		t.Fatal("expected an error when computeVisibility is absent, got nil")
	}
	if !strings.Contains(err.Error(), "was never found") {
		t.Fatalf("expected a 'never found' error, got: %v", err)
	}
}
