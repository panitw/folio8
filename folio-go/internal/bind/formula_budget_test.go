package bind

import (
	"github.com/panitw/folio8/folio-go/internal/expr"
	"strings"
	"testing"
)

func TestFormulaProjectionBudgetPrecedesAllocationAndTraversal(t *testing.T) {
	// The first row would fail as a wrong-kind projection if traversal started.
	// Only the pre-allocation row×segment charge may diagnose this attempt.
	rows := make([]Value, 10)
	rows[0] = Value{Kind: KindString, Str: "not an object"}
	data := Value{Kind: KindObject, Obj: map[string]Value{"items": {Kind: KindArray, Arr: rows}}}
	budget := expr.NewBudget()
	if err := budget.Charge(1_000_000 - 5); err != nil {
		t.Fatal(err)
	}
	resolver := exprResolver{scope: NewScope(data, Value{}), elementID: "budget", budget: budget}
	_, err := resolver.ProjectCollection([]string{"items", "amount"})
	if err == nil || !strings.Contains(err.Error(), "work units") {
		t.Fatalf("projection began before budget check: %v", err)
	}
}
func TestFormulaAggregateAlignmentSharesRecursionAndResolverBudget(t *testing.T) {
	data := mustDecode(t, `{"items":[{"amount":1e6},{"amount":0}]}`)
	evaluate := func(source string, remaining int) error {
		e, err := expr.Parse(source)
		if err != nil {
			t.Fatal(err)
		}
		budget := expr.NewBudget()
		if err := budget.Charge(1_000_000 - remaining); err != nil {
			t.Fatal(err)
		}
		resolver := exprResolver{scope: NewScope(data, Value{}), elementID: "alignment", budget: budget}
		_, _, err = expr.EvalWithBudget(e, resolver, testFormatContext(), "alignment", budget)
		return err
	}
	if err := evaluate("sum(items.amount)", 30); err != nil {
		t.Fatalf("one aggregate should fit: %v", err)
	}
	// Both sums fit independently, but their combined projection, decimal
	// alignment and node work must share this one expression's remainder.
	if err := evaluate("sum(items.amount)+sum(items.amount)", 30); err == nil || !strings.Contains(err.Error(), "work units") {
		t.Fatalf("aggregate budget was reset: %v", err)
	}
	if err := evaluate("sum(items.amount)", 17); err == nil || !strings.Contains(err.Error(), "work units") {
		t.Fatalf("alignment was not preflighted: %v", err)
	}
}

func TestFormulaConditionProductionResolverSharesBudget(t *testing.T) {
	// Share one immutable row object: the production seam must charge the number
	// of rows it projects, not the number of distinct object allocations.
	row := Value{Kind: KindObject, Obj: map[string]Value{"amount": {Kind: KindNumber, Num: "1"}}}
	rows := make([]Value, 250_000)
	for i := range rows {
		rows[i] = row
	}
	scope := NewScope(Value{Kind: KindObject, Obj: map[string]Value{"items": {Kind: KindArray, Arr: rows}}}, Value{})
	_, _, err := EvaluateCondition("sum(items.amount)>0", scope, testFormatContext(), "e1")
	if err == nil || !strings.Contains(err.Error(), "work units") {
		t.Fatalf("production resolver budget was not shared: %v", err)
	}
}
