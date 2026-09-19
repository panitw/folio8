package expr

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// Eval walks e against resolver, computing a Value (AC12-AC18), and
// alongside it every Caveat (Story 3.3, DECISION-5) the walk produced
// — non-error conditions the render survives (avg()-on-empty, R9).
// caveats is nil whenever none occurred; it is never a non-nil empty
// slice (D-2.8.6's "empty is nil, one representation", applied here so
// a caller can compare without special-casing the length-0 case).
// elementID names the binding site in every located error this
// produces, matching AD-14's convention throughout the rest of the
// codebase.
//
// Public entry validates the complete tree through Check. Recursive calls
// share that validation and one evaluation budget, including the selected
// branch's resolver and formatting work.
// fc (Story 3.4, R1/AC1) is the document's formatting context — locale
// tag plus fixed UTC offset — needed only by formatDate/formatNumber,
// but threaded through every recursive Eval call so a nested call
// (inside if()'s selected branch, for instance) can reach it too. It
// is a plain value, never read from package state (AD-1).
func Eval(e Expr, resolver Resolver, fc FormatContext, elementID string) (val Value, caveats []Caveat, err error) {
	if _, ok := resolver.(*budgetResolver); !ok {
		return EvalWithBudget(e, resolver, fc, elementID, NewBudget())
	}
	budget := expressionBudget(resolver)
	defer func() {
		if err == nil && val.Kind == KindNumber {
			err = validateDecimal(val.Num)
		}
		if err == nil && val.Kind == KindString {
			err = budget.Charge(len(val.Str))
		}
		if err != nil {
			var located *LocatedError
			if !errors.As(err, &located) {
				err = at(location(e), fmt.Errorf("expr: element %s: %w", elementID, err))
			}
		}
	}()
	if err = budget.Charge(1); err != nil {
		return
	}
	switch n := e.(type) {
	case *PathExpr:
		if resolver.(*budgetResolver).Resolver == nil {
			return Value{}, nil, fmt.Errorf("data resolver is unavailable")
		}
		val, err = resolver.Resolve(n.Segments)
		return
	case *StringLit:
		return Value{Kind: KindString, Str: n.Value}, nil, nil
	case *NumberLit:
		d, err := NewDecimal(n.Literal)
		return Value{Kind: KindNumber, Num: d}, nil, err
	case *BoolLit:
		return Value{Kind: KindBool, Bool: n.Value}, nil, nil
	case *NullLit:
		return Value{Kind: KindNull}, nil, nil
	case *GroupExpr:
		return Eval(n.Inner, resolver, fc, elementID)
	case *UnaryExpr:
		v, c, err := Eval(n.Operand, resolver, fc, elementID)
		if err != nil {
			return Value{}, nil, err
		}
		if v.Kind != KindNumber {
			return Value{}, nil, fmt.Errorf("unary %s operand must be a number, got %s", n.Op, v.Kind)
		}
		if n.Op == "-" {
			v.Num, err = decimalResult(new(big.Int).Neg(big.NewInt(v.Num.Coefficient)), v.Num.Exponent)
		}
		return v, c, err
	case *BinaryExpr:
		return evalBinary(n, resolver, fc, elementID)
	case *ConditionalExpr:
		return evalConditional(n.Condition, n.Then, n.Else, n.Raw, resolver, fc, elementID)
	case *CallExpr:
		return evalCall(n, resolver, fc, elementID)
	default:
		return Value{}, nil, fmt.Errorf("unrecognised expression node %T", e)
	}
}

func evalCall(call *CallExpr, resolver Resolver, fc FormatContext, elementID string) (Value, []Caveat, error) {
	entry, ok := lookupFunc(call.Name)
	if !ok {
		return Value{}, nil, fmt.Errorf(
			"expr: element %s: unknown function %q — the eight legal names are %s: %s",
			elementID, call.Name, strings.Join(LegalFunctionNames(), ", "), call.Raw,
		)
	}
	if len(call.Args) != entry.arity {
		return Value{}, nil, fmt.Errorf(
			"expr: element %s: %s() takes %d argument(s), got %d: %s",
			elementID, entry.name, entry.arity, len(call.Args), call.Raw,
		)
	}

	switch entry.name {
	case "upper", "lower":
		return evalUpperLower(entry.name, call, resolver, fc, elementID)
	case "if":
		return evalIf(call, resolver, fc, elementID)
	case "sum":
		return evalSum(call, resolver, elementID)
	case "count":
		v, err := evalCount(call, resolver, elementID)
		return v, nil, err
	case "avg":
		return evalAvg(call, resolver, elementID)
	case "formatDate":
		return evalFormatDate(call, resolver, fc, elementID)
	case "formatNumber":
		return evalFormatNumber(call, resolver, fc, elementID)
	default:
		// Unreachable given functionTable's own entries (table.go):
		// every entry is handled above (AC16's structural half,
		// table_derivational_test.go, asserts this by AST). Kept as a
		// located error, not a panic, per AD-14's "never a panic".
		return Value{}, nil, fmt.Errorf("expr: element %s: internal: %q has no evaluator", elementID, entry.name)
	}
}

// evalUpperLower is AC12: upper()/lower() evaluate per Go's
// strings.ToUpper/ToLower. A non-string operand — including a value
// resolved from data of the wrong kind, a number literal, or a null —
// is a located error, never a coerced stringification (AD-14's
// wrong-kind case, never a coercion, AD-14 verbatim).
func evalUpperLower(name string, call *CallExpr, resolver Resolver, fc FormatContext, elementID string) (Value, []Caveat, error) {
	v, caveats, err := Eval(call.Args[0], resolver, fc, elementID)
	if err != nil {
		return Value{}, nil, err
	}
	if v.Kind != KindString {
		return Value{}, nil, fmt.Errorf("%s() operand must be a string, got %s (never coerced): %s", name, v.Kind, call.Raw)
	}
	if err := expressionBudget(resolver).Charge(len(v.Str)); err != nil {
		return Value{}, nil, err
	}
	s := v.Str
	if name == "upper" {
		s = strings.ToUpper(s)
	} else {
		s = strings.ToLower(s)
	}
	return Value{Kind: KindString, Str: s}, caveats, nil
}

// evalIf is AC13/AC14 and the owner's ruling on if(null, …): if(cond,
// then, else), arity exactly 3 (table.go). cond must resolve to a
// JSON boolean — NO truthiness in any form (AD-14: a wrong-kind value
// is an Error, never a coercion). Two of AD-14's three presence cases
// diverge deliberately, and the divergence IS the point, tested as a
// pair so it cannot drift apart silently:
//
//   - An ABSENT path as cond is a LOCATED ERROR carrying the path —
//     this falls out of resolver.Resolve's own contract (ast.go) with
//     no special-casing here at all: Eval(call.Args[0], …) below
//     simply returns whatever error the resolver produced for an
//     absent path, unchanged.
//   - An EXPLICIT JSON null as cond takes the ELSE branch, SILENTLY —
//     no error, no diagnostic, no warning (OWNER DECISION, this
//     story). The trade-off (a reader cannot distinguish a hidden
//     section from one that was never there) was presented and chosen
//     deliberately, over the alternative of a Warning. This is the one
//     behaviour in the engine that produces no signal at all; it is
//     documented in folio-format.md and has its own dedicated,
//     findable test (TestIfNullConditionIsSilentlyFalse, eval_test.go)
//     for exactly that reason.
//
// AC14: only the SELECTED branch is evaluated — a function call that
// would ERROR, called from the branch that was NOT selected, must not
// surface at all (originally proved with an unimplemented function as
// the example, back when formatDate/formatNumber were registered but
// not yet computing; any erroring call demonstrates the same
// short-circuit today). This is not a special case here either: the
// unselected call.Args[1]/[2] element is simply never passed to Eval.
//
// A Caveat from cond's OWN evaluation (e.g. avg() used, unusually, as
// a condition and landing on the empty-average caveat before erroring
// on its non-boolean kind) still propagates — evalIf never discards a
// caveat it collected on the way to a result, selected branch or not.
func evalIf(call *CallExpr, resolver Resolver, fc FormatContext, elementID string) (Value, []Caveat, error) {
	return evalConditional(call.Args[0], call.Args[1], call.Args[2], call.Raw, resolver, fc, elementID)
}
func evalConditional(condition, then, otherwise Expr, raw string, resolver Resolver, fc FormatContext, elementID string) (Value, []Caveat, error) {
	cond, caveats, err := Eval(condition, resolver, fc, elementID)
	if err != nil {
		return Value{}, nil, err
	}
	selected, err := ConditionValue(cond, "if()/ternary condition", raw, elementID)
	if err != nil {
		return Value{}, nil, at(location(condition), err)
	}
	branch := otherwise
	if selected {
		branch = then
	}
	value, branchCaveats, err := Eval(branch, resolver, fc, elementID)
	return value, appendCaveats(caveats, branchCaveats), err
}

// ConditionValue applies D-3.2.3's owner-ruled axis for interpreting v
// as a boolean condition: JSON true/false decide the boolean directly;
// an explicit JSON null is silently false (no diagnostic, of any
// severity); any other kind — a string or a number, however falsy it
// looks ("", 0, "false") — is a located error, because AD-14 admits no
// truthiness.
//
// This is the ONE place that axis is decided. evalIf (above) and
// Story 3.5's element visibility (internal/bind.EvaluateCondition's
// caller, in package folio8) both need exactly this rule and must never
// acquire two independently-written copies of it (D-000.38) — the
// obvious wrong implementation of either feature is a falsy-check, and
// nothing in the grammar itself forbids writing one by hand at a
// second call site.
//
// label names the caller's own condition slot for the error text
// (e.g. "if() condition", "visibleIf"); raw is the offending
// expression's own source text, verbatim, as the author wrote it.
func ConditionValue(v Value, label, raw, elementID string) (bool, error) {
	switch v.Kind {
	case KindBool:
		return v.Bool, nil
	case KindNull:
		return false, nil // OWNER RULING (D-3.2.3): silent false, no diagnostic.
	default:
		return false, fmt.Errorf(
			"expr: element %s: %s must be a boolean, got %s (no truthiness — AD-14): %s",
			elementID, label, v.Kind, raw,
		)
	}
}

// appendCaveats concatenates a and b, preserving D-2.8.6's "empty is
// nil, one representation": nil in, nil out, never a non-nil empty
// slice manufactured along the way.
func appendCaveats(a, b []Caveat) []Caveat {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	out := make([]Caveat, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)
	return out
}
