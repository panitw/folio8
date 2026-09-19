// This file is Story 3.5's own obligation (R4): a bare-expression
// evaluator over the SAME resolution seam Resolve (text.go) already
// uses, for a caller that needs the raw expr.Value a condition resolved
// to — never coerced to a string — rather than a bound TEXT result.
//
// Every OTHER bind.* entry point (BindText, BindTextSpans, Resolve) is
// text-span shaped: it scans for "{{ }}" placeholders and writes
// whatever each one resolves to as text (text.go's Resolve: a string
// verbatim, a number as its exact decimal, null as empty, and anything
// else a wrong-kind Error).
// A visibility condition is a BARE expression with no "{{ }}" wrapping
// (folio8_expr_validate.go's checkVisibleIfExpression exists separately
// for exactly this reason: routing it through the text path "would
// scan for {{ }} occurrences inside it and find none, passing silently
// no matter what the string said"), and its result must stay a JSON
// boolean/null all the way back to the caller (D-3.2.3) rather than
// being forced through the string coercion every text-shaped entry
// point applies.
package bind

import (
	"fmt"

	"github.com/panitw/folio8/folio-go/internal/expr"
)

// EvaluateCondition parses and statically checks src — a bare
// expression, not a "{{ }}"-wrapped interpolation — then evaluates it
// against scope, returning the resolved expr.Value UNCOERCED, plus any
// Caveat the walk produced.
//
// This is a new function on bind, not a new method on expr.Resolver
// (D-3.3.1/S7: the seam is exactly Resolve + CollectionLength +
// ProjectCollection, closed and enforced by
// lint/internal/rules/resolvermethodset.go) — it reuses the SAME
// exprResolver adapter Resolve (text.go) already builds, so a
// condition and a text placeholder dispatch to data/params/row through
// one traversal, never two.
func EvaluateCondition(src string, scope Scope, fc expr.FormatContext, elementID string) (expr.Value, []expr.Caveat, error) {
	e, perr := parseAndCheck(src, elementID)
	if perr != nil {
		return expr.Value{}, nil, perr
	}
	if err := expr.CheckCondition(e); err != nil {
		return expr.Value{}, nil, fmt.Errorf("bind: element %s: visibleIf: %w", elementID, err)
	}
	budget := expr.NewBudget()
	resolver := exprResolver{scope: scope, elementID: elementID, budget: budget}
	value, caveats, err := expr.EvalWithBudget(e, resolver, fc, elementID, budget)
	if err == nil {
		_, err = expr.ConditionValue(value, "visibleIf", src, elementID)
		err = expr.WithLocation(e, err)
	}
	return value, caveats, err
}

// parseAndCheck is the parse + static-check half every bare-expression
// entry point in this file shares (D-000.9: one traversal, one error
// shape, never two spellings of the same two steps).
func parseAndCheck(src, elementID string) (expr.Expr, error) {
	e, perr := expr.Parse(src)
	if perr != nil {
		return nil, fmt.Errorf(
			"bind: element %s: invalid expression: %w",
			elementID, perr,
		)
	}
	if cerr := expr.Check(e); cerr != nil {
		return nil, fmt.Errorf("bind: element %s: %w", elementID, cerr)
	}
	return e, nil
}

// EvaluateValue is this file's second bare-expression entry point, and
// it is the general one this file's own header describes: "a caller that
// needs the raw expr.Value a[n expression] resolved to — never coerced
// to a string". It parses, statically checks and evaluates src against
// scope through the SAME exprResolver seam Resolve (text.go) and
// EvaluateCondition already use, and returns the resolved expr.Value
// uncoerced, plus any Caveat the walk produced.
//
// General value evaluation has no boolean consumer constraint.
//
// Its one caller today is Story 4.5's table footer (folio8's
// table_render.go, footerCellExprText): D-1.4.1 rules that a footer
// whose footerFormat is "absent and underived" renders UNFORMATTED, and
// the only way to render a Decimal at its own scale through the closed
// pattern grammar is to know that scale — which means evaluating the
// aggregate, as a NUMBER, before the display expression is synthesised.
// Routing that through this function rather than through a private
// evaluator is what keeps the footer's value on the one aggregate
// evaluation (Story 4.5's AC4/DW-7) instead of manufacturing a second.
func EvaluateValue(src string, scope Scope, fc expr.FormatContext, elementID string) (expr.Value, []expr.Caveat, error) {
	e, perr := parseAndCheck(src, elementID)
	if perr != nil {
		return expr.Value{}, nil, perr
	}
	budget := expr.NewBudget()
	resolver := exprResolver{scope: scope, elementID: elementID, budget: budget}
	return expr.EvalWithBudget(e, resolver, fc, elementID, budget)
}

// PathAbsentError distinguishes missing report data from formula failures.
type PathAbsentError struct {
	Path string
	Err  error
}

func (e *PathAbsentError) Error() string { return e.Err.Error() }
func (e *PathAbsentError) Unwrap() error { return e.Err }
