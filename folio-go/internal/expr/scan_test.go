package expr

import "testing"

// TestIsReservedIsExactlyPageAndPages is AD-4/QA Finding 10 (Minor):
// pins the reserved set to exactly {page, pages}, the same discipline
// AC7 already applies to the function table. Before this fix,
// ReservedPlaceholders was an exported, mutable map — any importer
// could grow or shrink AD-4's fence at runtime with nothing to catch
// it; this guard is what would now catch a THIRD entry (or the loss of
// an existing one) landing in reservedPlaceholders, whether by an
// in-package edit or (now that the map is unexported) a mistaken
// future re-export.
func TestIsReservedIsExactlyPageAndPages(t *testing.T) {
	if len(reservedPlaceholders) == 0 {
		t.Fatal("presence precondition (D-000.9): reservedPlaceholders is empty — nothing to check")
	}
	want := map[string]bool{"page": true, "pages": true}
	if len(reservedPlaceholders) != len(want) {
		t.Fatalf("reservedPlaceholders has %d entries, want exactly %d: %v", len(reservedPlaceholders), len(want), reservedPlaceholders)
	}
	for k := range want {
		if !reservedPlaceholders[k] {
			t.Errorf("reservedPlaceholders is missing %q", k)
		}
	}
	for k := range reservedPlaceholders {
		if !want[k] {
			t.Errorf("reservedPlaceholders has an unexpected third entry %q — AD-4 reserves exactly \"page\" and \"pages\"", k)
		}
	}
}

// TestIsReservedAgreesWithTheMap confirms IsReserved — the only way in
// or out of reservedPlaceholders now that it is unexported — actually
// reads the same set this file declares, for both members and a
// non-member.
func TestIsReservedAgreesWithTheMap(t *testing.T) {
	for name := range reservedPlaceholders {
		if !IsReserved(name) {
			t.Errorf("IsReserved(%q) = false, want true", name)
		}
	}
	if IsReserved("balance") {
		t.Error(`IsReserved("balance") = true, want false — "balance" is an ordinary data path, not reserved`)
	}
}
