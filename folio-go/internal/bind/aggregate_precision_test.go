package bind

// AC9-AC11: sum() is exact at the EVALUATION layer (AD-23), AC16:
// avg()'s round-half-to-even, and AC24-AC26: overflow/bound breaches
// wrapped with %w and errors.Is/As recoverable — all exercised through
// the same expr.Parse/Check/Eval pipeline aggregate_test.go uses, so
// these prove the whole seam, not the kernel in isolation (the kernel
// itself is unchanged, D-3.1a.2, and already has its own tests,
// internal/expr/reduce_test.go).

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/expr"
)

// TestSumIsExactOnD00061CorpusA is AC9: the hand-computed total is
// asserted against D-000.61's DISCRIMINATING corpus A —
// 12345678901234.56 + 32x0.01 — measured in that decision to be
// discriminating under float64 (unlike corpus C, seven small 2dp
// amounts, measured byte-identical under float64 and therefore useless
// as a red-proof subject). 12345678901234.56 + 32*0.01 =
// 12345678901234.88 exactly.
func TestSumIsExactOnD00061CorpusA(t *testing.T) {
	var elems []string
	elems = append(elems, `{"a":12345678901234.56}`)
	for i := 0; i < 32; i++ {
		elems = append(elems, `{"a":0.01}`)
	}
	js := `{"t":[` + strings.Join(elems, ",") + `]}`
	data := mustDecodeAggregate(t, js)

	v, _, err := evalAggregateExpr(t, "sum(t.a)", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := expr.Decimal{Coefficient: 1234567890123488, Exponent: -2}
	if v.Num != want {
		t.Fatalf("got %+v, want %+v (12345678901234.88)", v.Num, want)
	}
	// AC10/D-3.3.7 (finisher pass, correcting this comment): AD-23's
	// two-layer guard forbids float64 being COMMITTED anywhere under
	// folio-go/ — including inside a test — so the live demonstration
	// cannot land here. It is NOT merely cited, though: the engineering
	// lead ruled that a mutation red-proof is a measurement, not an
	// artifact, and it WAS performed (working-tree mutation of
	// SumDecimals, measured, reverted — never committed) in this
	// story's finisher pass, then pinned as a permanent, dependency-free
	// demonstration in hashmatrix/floatdiscrimination/ (outside AD-23's
	// scope by construction, same rationale as hashmatrix/probe/'s own
	// doc comment). See D-3.3.7 for the measured figures: the honest
	// value-level float64 mutant misses this exact total by one satang
	// in the corpus's declared (forward) order and lands on it by
	// coincidence in reverse — so the order-invariance assertion
	// reddens unconditionally and the value assertion only in the
	// declared order, both recorded there.
}

// TestSumOrderInvarianceOnD00061CorpusA is AC9's order-invariance
// half: original, reversed and shuffled order must yield the IDENTICAL
// Decimal.
func TestSumOrderInvarianceOnD00061CorpusA(t *testing.T) {
	amounts := []string{"12345678901234.56"}
	for i := 0; i < 32; i++ {
		amounts = append(amounts, "0.01")
	}
	buildJSON := func(order []string) string {
		var elems []string
		for _, a := range order {
			elems = append(elems, `{"a":`+a+`}`)
		}
		return `{"t":[` + strings.Join(elems, ",") + `]}`
	}

	original := append([]string(nil), amounts...)
	reversed := make([]string, len(amounts))
	for i, a := range amounts {
		reversed[len(amounts)-1-i] = a
	}
	// A fixed, non-trivial shuffle — deterministic (AD-1 forbids
	// math/rand under internal/, and a fixed permutation is exactly as
	// good a witness for order-invariance as a random one).
	shuffled := []string{amounts[16], amounts[0], amounts[32], amounts[1], amounts[31]}
	shuffled = append(shuffled, amounts[2:16]...)
	shuffled = append(shuffled, amounts[17:31]...)

	var results []expr.Decimal
	for _, order := range [][]string{original, reversed, shuffled} {
		data := mustDecodeAggregate(t, buildJSON(order))
		v, _, err := evalAggregateExpr(t, "sum(t.a)", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		results = append(results, v.Num)
	}
	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			t.Fatalf("AC9 VIOLATED: order %d produced %+v, want the same %+v as the original order", i, results[i], results[0])
		}
	}
}

// TestAvgRoundHalfToEvenAtTheSeam is AC16: round-half-to-even proven
// through avg() itself (not only at the kernel, internal/expr/
// reduce_test.go's TestAvgDecimalsRoundsHalfToEven, which this table
// reuses the exact constructed fixtures of): a tie resolving DOWN to an
// already-even quotient, a tie resolving UP from an odd quotient, and
// both again with the sign flipped. n=32 operands, all zero except one
// at exponent -2, so SumDecimals' total is exactly that one operand and
// avg's declared scale (0's own scale 2, plus avgExtraScale 4 = 6)
// lands the division on an EXACT half every time (each independently
// verified in reduce_test.go's own comment, reused here unchanged).
//
// CORRECTED (Story 3.3 finisher pass, Finding 16). This used to claim
// TestAvgRoutesThroughAvgDecimals "already proves avg() cannot
// substitute a different rounding rule without reddening" — false: that
// test only checks evalAvg's body textually CONTAINS a call to
// AvgDecimals (a vestigial, discarded call satisfies it — Finding 12),
// and says nothing about rounding. The rounding-rule red-proof is
// INHERITED from the kernel, not re-derived at this seam:
// internal/expr/reduce_test.go's TestAvgDecimalsRoundsHalfToEven
// already demonstrates round-half-away-from-zero reddening its own
// tie table, and this test reuses that SAME table's fixtures to prove
// the SAME rounding is reached THROUGH avg() — re-deriving the
// round-half-away red-proof here would test the kernel a second time,
// not the seam. (The tie fixtures themselves are genuine exact ties,
// independently verified: 0.01/32 = 0.0003125 -> 312 even; 0.03/32 =
// 0.0009375 -> 938 even.)
func TestAvgRoundHalfToEvenAtTheSeam(t *testing.T) {
	const n = 32
	buildJSON := func(firstCoefficient int64) string {
		elems := make([]string, n)
		for i := range elems {
			elems[i] = `{"a":0.00}`
		}
		elems[0] = fmt.Sprintf(`{"a":%s}`, decimalLiteral(firstCoefficient, -2))
		return `{"t":[` + strings.Join(elems, ",") + `]}`
	}

	cases := []struct {
		name             string
		firstCoefficient int64
		want             expr.Decimal
	}{
		{"tie resolves DOWN to an already-even quotient", 1, expr.Decimal{Coefficient: 312, Exponent: -6}},
		{"tie resolves UP from an odd quotient to the even neighbour", 3, expr.Decimal{Coefficient: 938, Exponent: -6}},
		{"negative tie resolves DOWN (magnitude) to an already-even quotient", -1, expr.Decimal{Coefficient: -312, Exponent: -6}},
		{"negative tie resolves UP (magnitude) from an odd quotient to the even neighbour", -3, expr.Decimal{Coefficient: -938, Exponent: -6}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data := mustDecodeAggregate(t, buildJSON(c.firstCoefficient))
			v, _, err := evalAggregateExpr(t, "avg(t.a)", data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v.Num != c.want {
				t.Fatalf("avg(...) = %+v, want %+v", v.Num, c.want)
			}
		})
	}
}

// decimalLiteral renders a JSON number literal for coefficient*10^exp —
// e.g. decimalLiteral(-3, -2) => "-0.03" — since JSON has no way to
// spell a negative-scale coefficient directly as an integer literal
// with a separate exponent field the way expr.Decimal does.
func decimalLiteral(coefficient int64, exp int) string {
	if exp >= 0 {
		return fmt.Sprintf("%de%d", coefficient, exp)
	}
	neg := coefficient < 0
	if neg {
		coefficient = -coefficient
	}
	digits := fmt.Sprintf("%d", coefficient)
	scale := -exp
	for len(digits) <= scale {
		digits = "0" + digits
	}
	intPart := digits[:len(digits)-scale]
	fracPart := digits[len(digits)-scale:]
	sign := ""
	if neg {
		sign = "-"
	}
	return sign + intPart + "." + fracPart
}

// TestSumOverflowIsLocatedAtExpressionLayer is AC24: an operand set
// whose aligned coefficient exceeds int64 produces an error, at the
// EXPRESSION layer (not only provable at the kernel), naming the
// element id, the collection path, and recoverable via errors.As —
// never by message matching (AD-14/AC26).
func TestSumOverflowIsLocatedAtExpressionLayer(t *testing.T) {
	// Two operands each near int64's own max (9223372036854775807) at
	// the same exponent: their sum overflows int64.
	data := mustDecodeAggregate(t, `{"t":[{"a":9223372036854775807},{"a":9223372036854775807}]}`)
	_, _, err := evalAggregateExpr(t, "sum(t.a)", data)
	if err == nil {
		t.Fatal("AC24 VIOLATED: an int64-overflowing sum must be a located error")
	}
	var kerr *expr.KernelOverflowError
	if !errors.As(err, &kerr) {
		t.Fatalf("AC26 VIOLATED: error must be recoverable via errors.As(*expr.KernelOverflowError), got: %v", err)
	}
	if kerr.ElementID != "e1" {
		t.Errorf("error must carry the element id, got %q", kerr.ElementID)
	}
	if !strings.Contains(kerr.Path, "t.a") {
		t.Errorf("error must carry the collection path, got %q", kerr.Path)
	}
	if kerr.Operands != 2 {
		t.Errorf("error must carry the operand count, got %d", kerr.Operands)
	}
	// AC26: the kernel's own wording survives, wrapped with %w — never
	// re-formatted into a new message.
	if kerr.Err == nil || !strings.Contains(kerr.Err.Error(), "int64") {
		t.Errorf("the kernel's own error text must survive unchanged inside the wrapper, got: %v", kerr.Err)
	}
	if !errors.Is(err, kerr.Err) {
		t.Error("AC26 VIOLATED: errors.Is(err, kerr.Err) must hold — %w wrapping, never a re-formatted message")
	}
}

// TestAvgResultExponentOverflowIsLocatedAtExpressionLayer is AC25: the
// AVG-SPECIFIC result-exponent bound breach (reduce.go's own
// CORRECTION, QA Finding 10), located and %w-wrapped the same way.
func TestAvgResultExponentOverflowIsLocatedAtExpressionLayer(t *testing.T) {
	// Two operands at an extreme (but individually legal) exponent:
	// SumDecimals accepts them, but AvgDecimals' OWN result-exponent
	// bound (total.Exponent - avgExtraScale) can still breach
	// maxDecimalExponentMagnitude even though the operands' own
	// exponent did not.
	json := fmt.Sprintf(`{"t":[{"a":%s},{"a":%s}]}`, extremeExponentLiteral(-99999), extremeExponentLiteral(-99999))
	data := mustDecodeAggregate(t, json)
	_, _, err := evalAggregateExpr(t, "avg(t.a)", data)
	if err == nil {
		t.Fatal("AC25 VIOLATED: an avg() whose result exponent breaches the magnitude bound must be a located error")
	}
	var kerr *expr.KernelOverflowError
	if !errors.As(err, &kerr) {
		t.Fatalf("AC26 VIOLATED: error must be recoverable via errors.As(*expr.KernelOverflowError), got: %v", err)
	}
	if kerr.Operands != 2 {
		t.Errorf("error must carry the operand count, got %d", kerr.Operands)
	}
}

// TestSumAlignmentSpreadOverflowIsLocatedAtExpressionLayer is AC25's
// OTHER half (Story 3.3 finisher pass, Finding 5): the ALIGNMENT-SPREAD
// breach (reduce.go's SumDecimals, checked BEFORE any shift is
// computed — F4/AC24's own note that a kernel-layer proof does not
// prove the wiring), located and %w-wrapped the same way as the
// overflow and result-exponent breaches above. F4 named this arm the
// REALISTIC trigger: one high-precision operand beside an ordinary
// amount, on a money path.
//
// Both operands are individually LEGAL (each exponent's own magnitude
// is within maxDecimalExponentMagnitude) — only their SPREAD (max minus
// min) exceeds the bound, so this exercises SumDecimals' own spread
// check, never decimal.go's per-literal exponent-magnitude check
// (which a single out-of-range literal would trip instead, at parse
// time, before an aggregate is even evaluated).
func TestSumAlignmentSpreadOverflowIsLocatedAtExpressionLayer(t *testing.T) {
	// exponents 50000 and -50001: spread = 100001, one past the bound;
	// each operand's own magnitude (50000, 50001) stays within it.
	json := fmt.Sprintf(`{"t":[{"a":%s},{"a":%s}]}`, extremeExponentLiteral(50000), extremeExponentLiteral(-50001))
	data := mustDecodeAggregate(t, json)
	_, _, err := evalAggregateExpr(t, "sum(t.a)", data)
	if err == nil {
		t.Fatal("AC25 VIOLATED: an alignment spread beyond the bound must be a located error")
	}
	var kerr *expr.KernelOverflowError
	if !errors.As(err, &kerr) {
		t.Fatalf("AC26 VIOLATED: error must be recoverable via errors.As(*expr.KernelOverflowError), got: %v", err)
	}
	if kerr.ElementID != "e1" {
		t.Errorf("error must carry the element id, got %q", kerr.ElementID)
	}
	if !strings.Contains(kerr.Path, "t.a") {
		t.Errorf("error must carry the collection path, got %q", kerr.Path)
	}
	if kerr.Operands != 2 {
		t.Errorf("error must carry the operand count, got %d", kerr.Operands)
	}
	// AC26: the kernel's own "alignment spread" wording survives inside
	// the wrapper, %w-wrapped — never re-formatted, never message-matched
	// by the caller (AD-14).
	if kerr.Err == nil || !strings.Contains(kerr.Err.Error(), "alignment spread") {
		t.Errorf("the kernel's own alignment-spread wording must survive unchanged inside the wrapper, got: %v", kerr.Err)
	}
	if !errors.Is(err, kerr.Err) {
		t.Error("AC26 VIOLATED: errors.Is(err, kerr.Err) must hold — %w wrapping, never a re-formatted message")
	}
}

// extremeExponentLiteral renders "1e<exp>" — a JSON number literal
// with coefficient 1 at the given (negative) exponent, used to probe
// AvgDecimals' own result-exponent bound without needing 99999 literal
// fractional digits in the source.
func extremeExponentLiteral(exp int) string {
	return fmt.Sprintf("1e%d", exp)
}
