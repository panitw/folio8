package text

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// TestAC10ComputedBreaksMatchS4Basis is AC10, D-2.1.1's guardrail: the
// computed break opportunities are asserted against the checked-in
// S4-basis fixture (fixtures/thai-break-corpus/computed_breaks.json,
// produced by cmd/genbreaks AFTER this story's hand review — Trap 1's
// ordering: review first, check in second, assert third) on the
// ordinary (non-heavy) suite, on every target — including this
// package's js/wasm leg (folio-go/wasmleg_test.go), which is what makes
// the Epic 2 gate's deferred linux/amd64 and linux/arm64 legs
// (D-000.4/D-2.1.1) genuinely exercise this story's work rather than
// merely proving the code compiles there.
//
// This test answers ONLY "does this target compute the same breaks as
// the fixture?" — never "are the breaks right" (that is P1-P3,
// asserted separately in corpus_test.go and never derived from this
// file; Trap 1). A target computing different breaks fails this test
// (AC10's binding property), not a warning.
//
// V10's vacuity guard: this asserts a NON-ZERO, computed break count
// exists somewhere in the fixture (proving the file is not empty or a
// trivial all-zero stub) and that every item in the live corpus has a
// corresponding fixture entry (proving this test is not silently
// comparing against a shorter, stale file).
func TestAC10ComputedBreaksMatchS4Basis(t *testing.T) {
	items := loadCorpus(t)
	fixture := loadComputedBreaksFixture(t)

	if len(fixture) == 0 {
		t.Fatal("V10: computed_breaks.json fixture is empty")
	}
	if len(fixture) != len(items) {
		t.Fatalf("V10: fixture has %d entries, corpus has %d items — the fixture is stale (run `cd folio-go && go run ./cmd/genbreaks` and commit the result)", len(fixture), len(items))
	}

	dict := Dictionary()
	var totalBreaks int
	var mismatches int
	for _, it := range items {
		want, ok := fixture[it.ID]
		if !ok {
			t.Errorf("AC10: corpus item %s has no entry in computed_breaks.json — fixture is stale", it.ID)
			continue
		}
		got, _ := ComputeBreaks(dict, it.Text, false)
		totalBreaks += len(got)

		sort.Ints(got)
		sortedWant := append([]int(nil), want...)
		sort.Ints(sortedWant)
		// Normalize nil vs. empty-non-nil (both mean "no breaks") before
		// comparing — reflect.DeepEqual treats []int{} and []int(nil)
		// as UNEQUAL, which would make every zero-break item a false
		// AC10 violation (a genuine Go gotcha this test nearly shipped
		// with — this story's dev record).
		if len(got) == 0 {
			got = nil
		}
		if len(sortedWant) == 0 {
			sortedWant = nil
		}
		if !reflect.DeepEqual(got, sortedWant) {
			t.Errorf("AC10 VIOLATION: item %s (%q) computed breaks %v, fixture (S4 basis) says %v — this target computes different breaks than the fixture", it.ID, it.Text, got, sortedWant)
			mismatches++
		}
	}

	if totalBreaks == 0 {
		t.Fatal("V10: every item computed zero breaks — the fixture (or the corpus) is vacuous")
	}
	if mismatches == 0 {
		t.Logf("AC10: this target (see PROBE-TARGET log lines in the same run) reproduces the S4-basis fixture exactly across all %d corpus items (%d total break positions)", len(items), totalBreaks)
	}
}

func loadComputedBreaksFixture(t *testing.T) map[string][]int {
	t.Helper()
	root := repoRootFromTest(t)
	path := filepath.Join(root, "fixtures", "thai-break-corpus", "computed_breaks.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read computed_breaks.json: %v", err)
	}
	var m map[string][]int
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal computed_breaks.json: %v", err)
	}
	return m
}
