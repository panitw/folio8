package arch

// AC23's production caller and AC24's red-proof, captured BEFORE the
// kernel exists (D-000.30 — the window shuts the moment the obligation
// is wired). See reducer_inventory_arch_test.go for the checker itself
// and D-3.1a.3 for why its location clause is relational.

import (
	"path/filepath"
	"testing"
)

// realModuleImportPath is folio-go's own module path (go.mod:1),
// spelled once here rather than re-derived, so the production caller
// can resolve a qualified reference (bind.Decimal) to a directory.
const realModuleImportPath = "github.com/panitw/folio8/folio-go"

// TestDecimalReducerInventoryIsExactlySumAndAvg is AC23: the set of
// functions in the folio-go module that reduce a sequence of Decimal to
// (Decimal, error) is exactly {SumDecimals, AvgDecimals}, and both are
// declared in the SAME package as the Decimal type declaration itself
// (D-3.1a.3 — relational, not the literal path "internal/bind", because
// Story 3.2 moves Decimal to internal/expr and the reducers travel with
// it).
//
// This is a set-equality-plus-location assertion, not an extinction
// guard (D-3.2.1's own note applies verbatim here): "nothing named
// Decimal survives" would be false by construction since the type must
// exist somewhere, and the same is true of these two reducers once the
// kernel exists. Location is the property being checked, not presence.
func TestDecimalReducerInventoryIsExactlySumAndAvg(t *testing.T) {
	got, stats, err := reducerDecimalInventory(moduleRoot(t), realModuleImportPath)
	if err != nil {
		t.Fatalf("reducer inventory over the folio-go module: %v", err)
	}

	// Vacuity guard (D-000.9): from the scanner's OWN reported stats,
	// confirm it actually parsed files and found the Decimal
	// declaration, so this cannot pass by walking zero files.
	if stats.FilesParsed == 0 {
		t.Fatal("vacuity guard: scanner's own stats report 0 files parsed under the folio-go module root")
	}
	if !stats.DeclFound {
		t.Fatal("vacuity guard: scanner's own stats report the \"Decimal\" type declaration was never found")
	}

	want := map[string]bool{"SumDecimals": true, "AvgDecimals": true}
	for _, v := range decimalReducerViolations(got, want, stats.DeclPkgDir) {
		t.Error(v)
	}
}

// fixtureModuleImportPath is a synthetic module path for the retained
// AST-only fixtures below — none of them are ever built as part of any
// real Go module (they live under testdata/), so this need only be
// self-consistent, never resolvable.
const fixtureModuleImportPath = "example.invalid/reducer-inventory-fixture"

func reducerInventoryFixtureRoot(t *testing.T, variant string) string {
	t.Helper()
	return filepath.Join(filepath.Dir(moduleRoot(t)), "folio-go", "testdata", "arch", "reducer-inventory", variant)
}

// TestDecimalReducerInventoryFixtureBaselineIsExactlyTwo is a sanity
// check on the fixture tree itself: pointed at the retained baseline —
// Decimal plus exactly SumDecimals and AvgDecimals — the scan finds
// exactly those two and nothing else, before either red-proof direction
// below is exercised.
func TestDecimalReducerInventoryFixtureBaselineIsExactlyTwo(t *testing.T) {
	root := reducerInventoryFixtureRoot(t, "baseline")
	got, stats, err := reducerDecimalInventory(root, fixtureModuleImportPath)
	if err != nil {
		t.Fatalf("scan fixture tree %s: %v", root, err)
	}
	if stats.FilesParsed == 0 {
		t.Fatal("vacuity guard: checker's own stats report 0 files parsed in the fixture tree")
	}
	if len(got) != 2 {
		t.Fatalf("fixture baseline sanity check: expected exactly 2 reducers, got %d: %+v", len(got), got)
	}
}

// TestDecimalReducerInventoryRedProofAddingAThirdReducer is AC24's
// FIRST red-proof direction, captured before the kernel exists: adding
// a third []Decimal -> (Decimal, error) function anywhere in the module
// must redden AC23. The extra-reducer fixture adds ProductDecimals
// alongside the sanctioned pair; the scan must report all three, which
// is what makes AC23's production assertion fail against a tree shaped
// like this one.
func TestDecimalReducerInventoryRedProofAddingAThirdReducer(t *testing.T) {
	root := reducerInventoryFixtureRoot(t, "extra-reducer")
	got, stats, err := reducerDecimalInventory(root, fixtureModuleImportPath)
	if err != nil {
		t.Fatalf("scan fixture tree %s: %v", root, err)
	}
	names := map[string]bool{}
	for _, r := range got {
		names[r.Name] = true
	}
	if len(got) != 3 || !names["SumDecimals"] || !names["AvgDecimals"] || !names["ProductDecimals"] {
		t.Fatalf(
			"red-proof direction 1 (add a third reducer) did not redden: expected 3 reducers "+
				"{SumDecimals, AvgDecimals, ProductDecimals}, got %d: %+v", len(got), got)
	}

	// QA review Finding 6 (Minor): the assertions above only prove the
	// SCANNER's raw output changed shape, not that AC23's own
	// comparison logic reddens against it. Run the same comparison
	// AC23's production test uses and assert its verdict is non-empty.
	want := map[string]bool{"SumDecimals": true, "AvgDecimals": true}
	if violations := decimalReducerViolations(got, want, stats.DeclPkgDir); len(violations) == 0 {
		t.Fatal("red-proof direction 1 (add a third reducer) did not redden AC23's actual verdict (decimalReducerViolations returned empty)")
	}
}

// TestDecimalReducerInventoryRedProofRemovingAReducer is AC24's SECOND
// red-proof direction: removing one of the two reducers must also
// redden AC23, not just adding a third. The missing-reducer fixture
// keeps SumDecimals and drops AvgDecimals.
func TestDecimalReducerInventoryRedProofRemovingAReducer(t *testing.T) {
	root := reducerInventoryFixtureRoot(t, "missing-reducer")
	got, stats, err := reducerDecimalInventory(root, fixtureModuleImportPath)
	if err != nil {
		t.Fatalf("scan fixture tree %s: %v", root, err)
	}
	if len(got) != 1 || got[0].Name != "SumDecimals" {
		t.Fatalf(
			"red-proof direction 2 (remove a reducer) did not redden: expected exactly 1 reducer "+
				"(SumDecimals), got %d: %+v", len(got), got)
	}

	// QA review Finding 6 (Minor): assert on AC23's own verdict too,
	// not only on the scanner's raw output shape — see the sibling
	// comment in TestDecimalReducerInventoryRedProofAddingAThirdReducer.
	want := map[string]bool{"SumDecimals": true, "AvgDecimals": true}
	if violations := decimalReducerViolations(got, want, stats.DeclPkgDir); len(violations) == 0 {
		t.Fatal("red-proof direction 2 (remove a reducer) did not redden AC23's actual verdict (decimalReducerViolations returned empty)")
	}
}

// TestDecimalReducerInventoryRedProofDecimalMovedLeavesReducersBehind is
// QA review Finding 5 (Minor): D-3.1a.3's own stated justification for
// the relational location clause — "it also fails if someone moves
// Decimal and leaves the reducers behind" — had no fixture exercising
// it; all three existing fixtures declare Decimal and both reducers in
// the SAME package directory. The moved-decimal fixture puts Decimal in
// pkga and both reducers in pkgb (reached only through the qualified
// reference pkga.Decimal), which is exactly D-3.1a.3's second failure
// mode: the scan must still find both reducers (proving the qualified-
// reference resolution path works) but report PkgDir != DeclPkgDir for
// both, reddening AC23.
func TestDecimalReducerInventoryRedProofDecimalMovedLeavesReducersBehind(t *testing.T) {
	root := reducerInventoryFixtureRoot(t, "moved-decimal")
	got, stats, err := reducerDecimalInventory(root, fixtureModuleImportPath)
	if err != nil {
		t.Fatalf("scan fixture tree %s: %v", root, err)
	}
	if stats.DeclPkgDir != "pkga" {
		t.Fatalf("fixture sanity check: expected Decimal declared in \"pkga\", got %q", stats.DeclPkgDir)
	}

	names := map[string]bool{}
	for _, r := range got {
		names[r.Name] = true
		if r.PkgDir != "pkgb" {
			t.Errorf("reducer %s: expected PkgDir \"pkgb\" (left behind after the simulated move), got %q", r.Name, r.PkgDir)
		}
	}
	if len(got) != 2 || !names["SumDecimals"] || !names["AvgDecimals"] {
		t.Fatalf("expected both reducers found in pkgb (via the qualified reference pkga.Decimal), got %d: %+v", len(got), got)
	}

	want := map[string]bool{"SumDecimals": true, "AvgDecimals": true}
	if violations := decimalReducerViolations(got, want, stats.DeclPkgDir); len(violations) == 0 {
		t.Fatal("D-3.1a.3's second failure mode did not redden: reducers left behind in a different package than Decimal must violate AC23")
	}
}
