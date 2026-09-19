package expr

import "testing"

// TestLegalFunctionNamesIsExactlyEight is AC5, checked at the package's
// own public surface rather than by reaching into functionTable
// (AC7/table.go's own AST guard, internal/expr_arch_test.go, is what
// asserts the LITERAL is exactly eight — this test additionally pins
// the exported accessor built on top of it).
func TestLegalFunctionNamesIsExactlyEight(t *testing.T) {
	want := map[string]bool{
		"sum": true, "count": true, "avg": true,
		"formatDate": true, "formatNumber": true,
		"upper": true, "lower": true, "if": true,
	}
	got := LegalFunctionNames()
	if len(got) != 8 {
		t.Fatalf("LegalFunctionNames() has %d entries, want 8: %v", len(got), got)
	}
	for _, name := range got {
		if !want[name] {
			t.Errorf("unexpected function name %q", name)
		}
		delete(want, name)
	}
	if len(want) != 0 {
		t.Errorf("missing function names: %v", want)
	}
}

// TestThreeImplementedFunctions/TestFiveUnimplementedFunctions moved to
// table_derivational_test.go (Story 3.3, AC29, F11/D-3.1a.3): those two
// tests were hard-coded name lists that this story's own edit (three
// names flip) and Story 3.4's (the remaining two) would each have had
// to edit in the same diff as the thing being guarded — "a guard whose
// expected value must be edited is one that gets edited wrongly."
// TestImplementedEntriesMatchEvalCallSwitch derives the same coverage
// from eval.go's own switch statement instead.

// TestAggregationEntriesDeclareDecimalReturn is AC9: sum/count/avg's
// table entries declare a Decimal-typed signature (a compile-time
// property — this test merely observes it, since a stringly-typed
// table would fail to compile this assertion at all).
func TestAggregationEntriesDeclareDecimalReturn(t *testing.T) {
	for _, name := range []string{"sum", "count", "avg"} {
		e, _ := lookupFunc(name)
		if _, ok := e.ret.(returnDecimal); !ok {
			t.Errorf("%s: expected a Decimal-typed return declaration, got %T", name, e.ret)
		}
	}
}
