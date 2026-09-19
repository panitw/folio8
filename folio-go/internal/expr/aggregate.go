package expr

import "fmt"

// This file is Story 3.3's evaluator for the three aggregate functions
// (R4, R5): sum() and avg() route through the SAME kernel
// (SumDecimals/AvgDecimals, reduce.go) DW-7/D-1.4.1 requires every
// aggregate caller — this expression, and Story 4.5's table footer —
// to share; count() never touches that kernel at all, calling only
// CollectionLength (R5's "count is a property of the collection; sum
// and avg are properties of a projection over it").
//
// routing_arch_test.go's AST scan is what makes the calls below an
// OBLIGATION, not a convention it happens to be honoured by today
// (F6/D-3.1a.4): it fails if evalSum's body stops containing a call to
// SumDecimals, or evalAvg's stops containing a call to AvgDecimals,
// however the surrounding code is refactored.
//
// STATED BOUND (Story 3.3 finisher pass, Finding 12, D-000.24): that
// scan proves the call is PRESENT, not that its RESULT is used — a
// deliberately vestigial `_, _ = SumDecimals(decimals)` alongside a
// second, real accumulator would satisfy it while computing the wrong
// answer through the wrong path. This is a stated, accepted bound, not
// built against: a scan that also traced the call's result into the
// function's return value would need a call-graph/data-flow analysis
// this file's own tests deliberately do not build (D-3.1a.4's ruling
// against a disproportionate routing instrument, applied the same way
// here as it was to the reducer inventory).

// aggregateOperandPath is AC6's shared precondition for all three
// aggregates: the single argument must be a bare data path — never a
// literal (already forbidden statically by argNotLiteral, check.go)
// and never a nested call — because a collection can only be named by
// a path; nothing else in the grammar denotes one.
func aggregateOperandPath(name string, call *CallExpr, elementID string) (*PathExpr, error) {
	p, ok := Ungroup(call.Args[0]).(*PathExpr)
	if !ok {
		return nil, fmt.Errorf(
			"expr: element %s: %s() operand must be a data path naming a collection, got %s: %s",
			elementID, name, call.Args[0].Text(), call.Raw,
		)
	}
	return p, nil
}

// decimalsFromProjection converts ProjectCollection's raw per-element
// Values into the []Decimal the kernel takes, applying R7's owner
// ruling: an explicit null element is a ZERO OBSERVATION, resolving to
// the additive identity {Coefficient: 0, Exponent: 0} — the SAME value
// D-3.1a.2 already gives SumDecimals(nil) — never a zero carrying the
// column's own scale or the projection's maximum scale (AC12).
func decimalsFromProjection(projected []Value, name, path, elementID string) ([]Decimal, error) {
	out := make([]Decimal, len(projected))
	for i, v := range projected {
		switch v.Kind {
		case KindNumber:
			out[i] = v.Num
		case KindNull:
			out[i] = Decimal{Coefficient: 0, Exponent: 0}
		default:
			// Unreachable in practice: ProjectCollection (internal/bind)
			// already screens every element's projected field to Number
			// or Null before returning success at all (AC19) — kept as
			// a located error, never a panic (AD-14), in case that
			// contract is ever broken by a future ProjectCollection
			// implementation.
			return nil, fmt.Errorf(
				"expr: element %s: %s(%s): internal: projected element %d resolved to a %s, neither number nor null",
				elementID, name, path, i, v.Kind,
			)
		}
	}
	return out, nil
}

// KernelOverflowError wraps a bound-breach error from SumDecimals or
// AvgDecimals (reduce.go: the alignment-spread bound, the int64
// narrowing, avg's own result-exponent bound) with the AD-14 payload
// the kernel itself cannot attach (reduce.go's own F2 note: "a
// []Decimal alone genuinely does not know its own element id") — the
// calling element id, the collection path exactly as authored, and the
// operand count. It implements Unwrap so errors.Is/errors.As reach the
// kernel's own error unchanged (AC26): a caller — Story 3.6, attaching
// an AD-14 registry code — matches this TYPE, never the kernel's
// message text, which AD-14 forbids.
type KernelOverflowError struct {
	ElementID string
	Path      string
	Operands  int
	Err       error
}

func (e *KernelOverflowError) Error() string {
	return fmt.Sprintf("expr: element %s: %s: %d operand(s): %s", e.ElementID, e.Path, e.Operands, e.Err)
}

func (e *KernelOverflowError) Unwrap() error { return e.Err }

// evalSum is AC8-AC11: sum() routes through SumDecimals — the reducer
// inventory (D-3.1a.3) does not, by itself, force that (F6); this call
// site, and routing_arch_test.go's structural assertion of it, are
// what do.
func evalSum(call *CallExpr, resolver Resolver, elementID string) (Value, []Caveat, error) {
	pathExpr, err := aggregateOperandPath("sum", call, elementID)
	if err != nil {
		return Value{}, nil, err
	}
	projected, err := resolver.ProjectCollection(pathExpr.Segments)
	if err != nil {
		return Value{}, nil, err
	}
	preflightErr := preflightProjection(projected, expressionBudget(resolver))
	if preflightErr != nil {
		return Value{}, nil, preflightErr
	}
	decimals, err := decimalsFromProjection(projected, "sum", pathExpr.Raw, elementID)
	if err != nil {
		return Value{}, nil, err
	}
	total, err := SumDecimals(decimals)
	if err != nil {
		return Value{}, nil, &KernelOverflowError{ElementID: elementID, Path: pathExpr.Raw, Operands: len(decimals), Err: err}
	}
	return Value{Kind: KindNumber, Num: total}, nil, nil
}

// evalCount is AC12-AC13/R5: count() calls CollectionLength and
// NOTHING else — it never calls ProjectCollection, so it is
// structurally unable to reach a projected value (AC3), and an element
// missing the projected field a sum()/avg() over the same collection
// would reject still counts (R5).
func evalCount(call *CallExpr, resolver Resolver, elementID string) (Value, error) {
	pathExpr, err := aggregateOperandPath("count", call, elementID)
	if err != nil {
		return Value{}, err
	}
	n, err := resolver.CollectionLength(pathExpr.Segments)
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: KindNumber, Num: Decimal{Coefficient: int64(n), Exponent: 0}}, nil
}

// evalAvg is AC12-AC16/R9/R10: avg() routes through AvgDecimals,
// dividing by len(projected) — NEVER a second CollectionLength call
// (R10: AvgDecimals' signature is unchanged, so the divisor cannot be
// passed in; AC4's length invariant is what makes len(projected) equal
// the collection's own length).
//
// A present-but-EMPTY collection (len(decimals) == 0) is handled here,
// BEFORE calling AvgDecimals at all, rather than by calling it and
// inspecting its returned error's text: AvgDecimals(nil) and a genuine
// operand-count-dependent bound breach are both possible error
// outcomes of that same call, and AD-14 forbids distinguishing them by
// message matching. The length is already known exactly (it is
// len(decimals)), so the distinction is made on that value instead —
// never on the kernel's wording. This is R9/DECISION-5's caveat, not
// an Error: Story 4.2's own AC requires an empty-collection table to
// render, so the aggregate resolves to empty and the caller surfaces a
// Warning.
func evalAvg(call *CallExpr, resolver Resolver, elementID string) (Value, []Caveat, error) {
	pathExpr, err := aggregateOperandPath("avg", call, elementID)
	if err != nil {
		return Value{}, nil, err
	}
	projected, err := resolver.ProjectCollection(pathExpr.Segments)
	if err != nil {
		return Value{}, nil, err
	}
	preflightErr := preflightProjection(projected, expressionBudget(resolver))
	if preflightErr != nil {
		return Value{}, nil, preflightErr
	}
	decimals, err := decimalsFromProjection(projected, "avg", pathExpr.Raw, elementID)
	if err != nil {
		return Value{}, nil, err
	}
	if len(decimals) == 0 {
		return Value{Kind: KindNull}, []Caveat{{Kind: CaveatEmptyAverage, Path: pathExpr.Raw}}, nil
	}
	avg, err := AvgDecimals(decimals)
	if err != nil {
		return Value{}, nil, &KernelOverflowError{ElementID: elementID, Path: pathExpr.Raw, Operands: len(decimals), Err: err}
	}
	return Value{Kind: KindNumber, Num: avg}, nil, nil
}

// Preflight kernel alignment before allocating decimals or any powers of ten.
func preflightProjection(values []Value, budget *Budget) error {
	if err := budget.Charge(len(values)); err != nil {
		return err
	}
	minExp := 0
	found := false
	for _, v := range values {
		if v.Kind == KindNumber {
			if err := validateDecimal(v.Num); err != nil {
				return err
			}
			if !found || v.Num.Exponent < minExp {
				minExp = v.Num.Exponent
			}
			found = true
		} else if v.Kind == KindNull {
			if !found || 0 < minExp {
				minExp = 0
			}
			found = true
		}
	}
	for _, v := range values {
		e := 0
		if v.Kind == KindNumber {
			e = v.Num.Exponent
		}
		if err := budget.Charge(e - minExp); err != nil {
			return err
		}
	}
	return budget.Charge(avgExtraScale)
}
