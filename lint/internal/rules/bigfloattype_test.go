package rules

import (
	"path/filepath"
	"strings"
	"testing"
)

// assertBigFloatVisited mirrors assertVisited (floattyped_test.go) for
// BigFloatTypeStats: checked BY NAME, from the checker's own reported
// stats, so a run that visited only part of the tree cannot pass by
// coincidentally reporting the same zero findings a whole-tree run
// reports.
func assertBigFloatVisited(t *testing.T, stats BigFloatTypeStats, want ...string) {
	t.Helper()
	seen := map[string]bool{}
	for _, d := range stats.DirsVisited {
		seen[d] = true
	}
	for _, w := range want {
		if !seen[w] {
			t.Fatalf("vacuity guard: checker's own stats did not report visiting directory %q, got %v", w, stats.DirsVisited)
		}
	}
}

// TestBigFloatTypeCoverageStatementWording is AC17: the coverage
// witness's documentation must say, IN THOSE WORDS (D-000.23), that
// this rule covers the two named types and not the class, and that it
// is not counted as coverage for AD-23's property — Layer 1 is.
func TestBigFloatTypeCoverageStatementWording(t *testing.T) {
	for _, want := range []string{
		"math/big.Float", "math/big.Rat",
		"these two types, not the class",
		"not counted as coverage",
		"Layer 1",
	} {
		if !strings.Contains(BigFloatTypeCoverageStatement, want) {
			t.Errorf("BigFloatTypeCoverageStatement does not contain %q: %s", want, BigFloatTypeCoverageStatement)
		}
	}
}

// TestBigFloatTypeProductionScan is AC21: the real folio-go module
// root, asserted to report zero findings — expected to pass on the
// first run (F1 measured big.Float/big.Rat at zero sites repo-wide),
// with the AC16 vacuity witness making that green mean something.
func TestBigFloatTypeProductionScan(t *testing.T) {
	root := repoRootFromTest(t)
	moduleRoot := filepath.Join(root, "folio-go")

	findings, stats, err := ScanBigFloatTypes(moduleRoot, false)
	if err != nil {
		t.Fatalf("scan folio-go module root %s: %v", moduleRoot, err)
	}

	assertBigFloatVisited(t, stats, ".", "internal/bind")
	if stats.FilesParsed == 0 {
		t.Fatal("vacuity guard: checker's own stats report 0 files parsed under the folio-go module root")
	}
	if stats.TypedExprs == 0 {
		t.Fatal("vacuity guard: checker's own stats report 0 expressions successfully typed under the folio-go module root")
	}

	if len(findings) > 0 {
		var msgs []string
		for _, f := range findings {
			msgs = append(msgs, f.Rule+": "+f.Message)
			if f.Rule != RuleNoBigFloatType {
				t.Errorf("finding at %s carries rule id %q, want %q", f.Path, f.Rule, RuleNoBigFloatType)
			}
		}
		t.Fatalf(
			"math/big.Float or math/big.Rat found under the folio-go module root (AD-23) — this is a "+
				"defect found, not a guard problem:\n%s",
			strings.Join(msgs, "\n"))
	}
}

// TestBigFloatTypeTestScopeInventory is AC15's file-scope clause,
// verified rather than merely stated (QA review Finding 2, Blocker):
// _test.go files under the folio-go module root are IN SCOPE, matching
// ScanFloatTypedValues' own TestFloatTypedTestScopeInventory
// (floattyped_test.go) — "a type-aware rule that did not [walk
// _test.go files] would be strictly weaker in file scope than the
// guard it strengthens — a silent regression dressed as a fix." No
// production caller passed includeTests:true before this fix, so a
// math/big.Float or math/big.Rat in any _test.go file shipped
// undetected by BOTH guards (the syntactic one keys on
// float32/float64 identifiers and cannot see it either).
//
// This is an INVENTORY, not a bare zero-assertion dressed as one, the
// same shape as TestFloatTypedTestScopeInventory: it currently holds
// ZERO sanctioned sites, because F1 measured zero occurrences of the
// two banned types anywhere repo-wide — production code and tests
// alike. If a _test.go file ever legitimately needs one, it is added
// here, deliberately, in the same diff that introduces it — never
// silently absorbed.
func TestBigFloatTypeTestScopeInventory(t *testing.T) {
	root := repoRootFromTest(t)
	moduleRoot := filepath.Join(root, "folio-go")

	got, stats, err := ScanBigFloatTypes(moduleRoot, true)
	if err != nil {
		t.Fatalf("scan folio-go module root %s with tests: %v", moduleRoot, err)
	}

	assertBigFloatVisited(t, stats, ".", "internal/bind")
	if stats.FilesParsed == 0 {
		t.Fatal("vacuity guard: checker's own stats report 0 files parsed under the folio-go module root (tests included)")
	}
	if stats.TypedExprs == 0 {
		t.Fatal("vacuity guard: checker's own stats report 0 expressions successfully typed under the folio-go module root (tests included)")
	}
	for _, d := range stats.DirsVisited {
		if strings.HasPrefix(d, "..") {
			t.Errorf("coverage witness leaked a path outside the scanned root: %q — the go-build-cache filter should have excluded this", d)
		}
	}

	if len(got) != 0 {
		var msgs []string
		for _, f := range got {
			msgs = append(msgs, f.Rule+": "+f.Message)
		}
		t.Fatalf(
			"math/big.Float or math/big.Rat found under a _test.go file in the folio-go module root "+
				"(AD-23, AC15) — this is a defect found, not a guard problem; add the sanctioned site to "+
				"this inventory deliberately if it is intentional:\n%s",
			strings.Join(msgs, "\n"))
	}
}

// TestBigFloatTypeFixtureScan is AC18/AC19's core red-proof: the
// checker, pointed at the retained fixture tree, reports EXACTLY the
// two violating files and NOT the compliant one — compared by (file,
// rule), never by count (D-1.3.3 amended).
func TestBigFloatTypeFixtureScan(t *testing.T) {
	root := repoRootFromTest(t)
	fixtureRoot := filepath.Join(root, "folio-go", "testdata", "lint", "no-bigfloat-type")

	got, stats, err := ScanBigFloatTypes(fixtureRoot, false)
	if err != nil {
		t.Fatalf("scan fixture tree %s: %v", fixtureRoot, err)
	}
	if stats.TypedExprs == 0 {
		t.Fatal("vacuity guard: checker's own stats report 0 expressions successfully typed in the fixture tree")
	}
	if stats.FilesParsed < 3 {
		t.Fatalf("vacuity guard: the fixture tree carries three files (one compliant, two violating); the checker reports %d files parsed", stats.FilesParsed)
	}

	want := []Finding{
		{Path: "violating_renamed_import.go", Rule: RuleNoBigFloatType},
		{Path: "violating_type_alias.go", Rule: RuleNoBigFloatType},
		{Path: "violating_variable.go", Rule: RuleNoBigFloatType},
	}
	assertExactFindings(t, got, want)
}

// TestBigFloatTypeFindingNamesResolvedTypeAndCoverage is AC14/AC17's
// message requirement: a finding names the RESOLVED type identity (not
// "a big.Float was found") and carries the coverage statement, so a
// reader who sees only the message still learns this is a narrow
// denylist rather than AD-23's whole enforcement.
func TestBigFloatTypeFindingNamesResolvedTypeAndCoverage(t *testing.T) {
	root := repoRootFromTest(t)
	fixtureRoot := filepath.Join(root, "folio-go", "testdata", "lint", "no-bigfloat-type")

	got, _, err := ScanBigFloatTypes(fixtureRoot, false)
	if err != nil {
		t.Fatalf("scan fixture tree %s: %v", fixtureRoot, err)
	}
	if len(got) == 0 {
		t.Fatal("presence precondition: the fixture scan reported no findings at all")
	}

	byPath := map[string]Finding{}
	for _, f := range got {
		byPath[f.Path] = f
	}

	renamed, ok := byPath["violating_renamed_import.go"]
	if !ok {
		t.Fatal("no finding for violating_renamed_import.go (AC18's renamed-import case)")
	}
	if !strings.Contains(renamed.Message, "math/big.Float") {
		t.Errorf("renamed-import finding does not name the resolved type math/big.Float: %s", renamed.Message)
	}

	variable, ok := byPath["violating_variable.go"]
	if !ok {
		t.Fatal("no finding for violating_variable.go (AC18's \"variable of the type\" case)")
	}
	if !strings.Contains(variable.Message, "math/big.Rat") {
		t.Errorf("variable finding does not name the resolved type math/big.Rat: %s", variable.Message)
	}

	for _, f := range got {
		if !strings.Contains(f.Message, BigFloatTypeCoverageStatement) {
			t.Errorf("finding at %s does not carry the D-000.23 coverage statement: %s", f.Path, f.Message)
		}
		if f.Line <= 0 {
			t.Errorf("finding at %s carries no line number: %+v", f.Path, f)
		}
	}
}

// TestBigFloatTypeScanFailsLoudlyOnAnUnloadableTree is AC20's first
// loud-failure path: a target that does not exist at all.
func TestBigFloatTypeScanFailsLoudlyOnAnUnloadableTree(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-directory")

	findings, _, err := ScanBigFloatTypes(missing, false)
	if err == nil {
		t.Fatalf("scanning a non-existent tree %s returned no error and %d findings", missing, len(findings))
	}
}

// TestBigFloatTypeScanFailsLoudlyOnATreeThatDoesNotTypeCheck is AC20's
// second loud-failure path (D-1.3.11: packages.Load's nil top-level
// error is not sufficient — the per-package Errors sweep is what must
// catch this). Reuses the existing float-typed-untypecheckable fixture
// (folio-go/testdata/lint/float-typed-untypecheckable) rather than
// duplicating it: the fixture's job — "a tree that parses but does not
// type-check" — is identical for both rules, and TestFloatTypedScanFailsLoudlyOnATreeThatDoesNotTypeCheck
// already asserts its two preconditions (parses cleanly; still carries
// the undefined symbol) independently.
func TestBigFloatTypeScanFailsLoudlyOnATreeThatDoesNotTypeCheck(t *testing.T) {
	root := repoRootFromTest(t)
	fixture := filepath.Join(root, "folio-go", "testdata", "lint", "float-typed-untypecheckable")

	findings, stats, err := ScanBigFloatTypes(fixture, false)
	if err == nil {
		t.Fatalf("scanning the untypeable tree %s returned no error and %d findings (stats %+v) — a checker that cannot type a tree must fail loudly, never report zero (D-1.3.11)", fixture, len(findings), stats)
	}
	if findings != nil {
		t.Errorf("scan of %s returned %d findings alongside its error; the loud-failure path must return no findings", fixture, len(findings))
	}
	const sweepMarker = "type information unavailable under"
	if !strings.Contains(err.Error(), sweepMarker) {
		t.Errorf("error did not come from the per-package Errors sweep (expected it to contain %q): %v", sweepMarker, err)
	}
}
