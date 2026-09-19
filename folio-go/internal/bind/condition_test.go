package bind

import (
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/expr"
)

// TestEvaluateConditionRejectsLiteral is Story 3.5 finisher review,
// Finding 10 (Minor) / D-3.5.1's own tripwire: EvaluateCondition is a
// condition slot, so it must apply the SAME literal rejection
// checkVisibleIfExpression enforces at load — not only at load — so a
// future second caller reaching this function directly cannot silently
// bypass D-3.5.1. Unreachable through the public API today (a literal
// visibleIf never survives ParseTemplate), so this test calls
// EvaluateCondition directly, the one place the property is checkable
// at all.
func TestEvaluateConditionRejectsLiteral(t *testing.T) {
	scope := NewScope(Value{}, Value{})
	fc := expr.NewFormatContext("en", "+00:00")

	for _, src := range []string{"42", `"hello"`} {
		_, _, err := EvaluateCondition(src, scope, fc, "e1")
		if err == nil {
			t.Fatalf("EvaluateCondition(%q): expected a literal-rejection error, got nil", src)
		}
		if !strings.Contains(err.Error(), "must be a boolean or null") {
			t.Errorf("EvaluateCondition(%q): expected the literal-rejection wording, got: %v", src, err)
		}
	}
}

// TestEvaluateConditionAcceptsOrdinaryPath is the negative control: an
// ordinary data-path condition must not be caught by the new literal
// check.
func TestEvaluateConditionAcceptsOrdinaryPath(t *testing.T) {
	data, err := DecodeData([]byte(`{"customer": {"flag": true}}`))
	if err != nil {
		t.Fatalf("DecodeData: %v", err)
	}
	scope := NewScope(data, Value{})
	fc := expr.NewFormatContext("en", "+00:00")

	val, _, everr := EvaluateCondition("customer.flag", scope, fc, "e1")
	if everr != nil {
		t.Fatalf("EvaluateCondition(customer.flag): unexpected error: %v", everr)
	}
	if val.Kind != expr.KindBool || !val.Bool {
		t.Errorf("EvaluateCondition(customer.flag): got %+v, want boolean true", val)
	}
}
