package expr

// SumDecimals and AvgDecimals are Story 3.1a's exact decimal reduction
// KERNEL (AC1): pure arithmetic over []Decimal, reusing decimal.go's
// existing bounds and big.Int narrowing pattern. These are NOT
// expression-language functions — no arity checking, no collection
// resolution, no diagnostic codes, no function-table entry (Story 3.3
// owns all of that, and the closed eight-entry table). D-3.1a.3's
// tripwire (reducer_inventory_arch_test.go, folio-go/internal/) asserts
// these two, and only these two, live here — in the same package as
// Decimal itself — so it survives Story 3.2's move of Decimal to
// internal/expr with no edit. It is a declaration-shape set-equality
// check, not a call-graph one: it does NOT assert anything calls
// SumDecimals/AvgDecimals, so it cannot by itself force a future
// sum()/avg() to route through them (QA review Finding 11; D-3.1a.4
// corrects the story's earlier over-claim on this point).
//
// F2 (Flag F2, this story): the errors below carry no AD-14 diagnostic
// code, no located data path and no element id. internal/diag does not
// exist until Story 3.6 (absence-diag-package), and a []Decimal alone
// genuinely does not know its own element id — inventing a placeholder
// would be worse than omitting it. These errors are located ONLY as far
// as the kernel itself can locate them (the operand index/count and the
// bound breached); the eventual caller in internal/expr attaches the
// full AD-14 payload. This comment must not be read as claiming AD-14
// conformance it does not have (D-000.24).

import (
	"fmt"
	"math/big"
)

// avgExtraScale is D-3.1a.1's declared division scale offset for
// AvgDecimals: it divides at (maximum operand scale + 4). Declared ONCE
// here, as a property of the operation, and never derived from a
// corpus (D-000.32) or from any caller's data. The specific value 4 is
// ILLUSTRATIVE and carried at the charter's own MEDIUM confidence
// (Flag F3): Story 3.4's assertion that no formatNumber pattern it
// accepts can request more fractional digits than avg produces is what
// keeps this honest. If 3.4 finds such a pattern, this constant moves —
// that is the design working, not a regression here.
const avgExtraScale = 4

// tenPow computes 10^n as a big.Int. n is always small and non-negative
// at every call site below (avgExtraScale, a compile-time constant), so
// this never faces the unbounded-shift hazard AC6 guards against —
// that hazard lives entirely in SumDecimals' alignment step, which is
// bounded BEFORE any shift the size of an operand's own exponent
// spread is computed.
func tenPow(n int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}

// SumDecimals reduces a sequence of Decimal to their exact sum (AC1).
//
// D-3.1a.2: SumDecimals(nil) — and SumDecimals of any empty slice —
// returns the ADDITIVE IDENTITY {Coefficient: 0, Exponent: 0}, never an
// error. Sum has an identity element; an empty sum is defined. This is
// NOT a ruling on what the expression-language sum() does on an empty
// COLLECTION (Story 3.3's own question, Flag F5) — it is the kernel's
// own arithmetic identity, answered once here so every caller (the
// expression sum(), and D-1.4.1's table-footer evaluation, which must
// use this same kernel) gets the same zero at the same scale rather
// than inventing its own.
//
// AC2/AC3: operands are aligned to their common MINIMUM exponent
// (equivalently, the MAXIMUM operand scale — D-1.6.1's AC2 distinctness,
// "1.50" != "1.5", survives addition because the result's Exponent is
// never coarser than any operand's), accumulated in big.Int (already
// blessed and imported, decimal.go's own "math/big" import — F12,
// Story 3.3: prefer this symbol reference over a line cite, which has
// already gone stale here twice — exact, no intermediate rounding),
// and narrowed EXACTLY ONCE, at the end, through the SAME IsInt64()
// pattern NewDecimal's own coefficient narrowing already uses
// (decimal.go). No checked-add helper is written: there is one
// narrowing site, and it is this one.
//
// AC6: the alignment spread (maximum exponent minus minimum exponent)
// is checked against maxDecimalExponentMagnitude (decimal.go) BEFORE
// any shift is computed. Computing a shift whose magnitude is itself
// what the bound exists to forbid — e.g. a request to align across a
// spread of tens of millions — is exactly the cost this ordering
// defends against; discovering the shift was too large only after
// building it would already have paid that cost.
//
// CORRECTION (QA review Finding 3, Major). Exponent is a plain Go int,
// and Decimal is an exported struct with exported fields any caller can
// construct directly, bypassing NewDecimal's own per-literal bound
// entirely (this file's own tests do, at AC5's red-proofs). The spread
// used to be computed as plain "maxExp - minExp": for extreme operand
// exponents (math.MinInt and math.MaxInt together, the reviewer's own
// reproduction) that subtraction itself WRAPS, landing back inside the
// bound and sailing the check right past a request the bound exists to
// forbid — a silent wrong answer, not a rejection. The spread is now
// computed in math/big (already imported, exact, cannot wrap), so the
// bound check is correct for every representable Exponent, not merely
// for spreads that happen to fit back in range after wrapping.
func SumDecimals(items []Decimal) (Decimal, error) {
	if len(items) == 0 {
		return Decimal{Coefficient: 0, Exponent: 0}, nil
	}

	minExp := items[0].Exponent
	maxExp := items[0].Exponent
	for _, it := range items[1:] {
		if it.Exponent < minExp {
			minExp = it.Exponent
		}
		if it.Exponent > maxExp {
			maxExp = it.Exponent
		}
	}

	spread := new(big.Int).Sub(big.NewInt(int64(maxExp)), big.NewInt(int64(minExp)))
	if spread.Cmp(big.NewInt(maxDecimalExponentMagnitude)) > 0 {
		return Decimal{}, fmt.Errorf(
			"expr: sum: alignment spread %s between operand exponents (min %d, max %d) exceeds %d",
			spread.String(), minExp, maxExp, maxDecimalExponentMagnitude,
		)
	}

	total := new(big.Int)
	for _, it := range items {
		shift := it.Exponent - minExp // always >= 0, and now known <= spread <= the bound
		term := big.NewInt(it.Coefficient)
		if shift > 0 {
			term.Mul(term, tenPow(shift))
		}
		total.Add(total, term)
	}

	if !total.IsInt64() {
		return Decimal{}, fmt.Errorf(
			"expr: sum: accumulated coefficient %s does not fit int64 (%d operand(s), aligned to exponent %d)",
			total.String(), len(items), minExp,
		)
	}

	return Decimal{Coefficient: total.Int64(), Exponent: minExp}, nil
}

// AvgDecimals reduces a sequence of Decimal to their exact average
// (AC1), dividing at (maximum operand scale + 4) with round-half-to-even
// (AC4).
//
// D-3.1a.2: unlike SumDecimals, AvgDecimals(nil) — and any empty slice —
// returns an ERROR. Averaging has no identity element: there is
// genuinely no value to report for zero operands (division by zero).
// This asymmetry with SumDecimals IS the point (D-3.1a.2's own words):
// sum answers the empty case once, honestly, and avg refuses honestly,
// rather than both inventing a scale nobody asked for. As with sum,
// this is NOT a ruling on the expression-language avg()'s own
// empty-collection semantics (Story 3.3's question, Flag F5).
func AvgDecimals(items []Decimal) (Decimal, error) {
	if len(items) == 0 {
		return Decimal{}, fmt.Errorf("expr: avg: cannot average 0 operands")
	}

	total, err := SumDecimals(items)
	if err != nil {
		return Decimal{}, fmt.Errorf("expr: avg: %w", err)
	}

	// total.Exponent is already the maximum operand scale (SumDecimals'
	// own AC3 guarantee). Dividing total's exact value by len(items) at
	// scale (that scale + avgExtraScale) is, in units of
	// 10^(total.Exponent - avgExtraScale), exactly
	// round_half_even(total.Coefficient * 10^avgExtraScale / n) — the
	// two exponents being avgExtraScale apart by construction removes
	// any need to reason about the general alignment case here.
	n := int64(len(items))
	numerator := new(big.Int).Mul(big.NewInt(total.Coefficient), tenPow(avgExtraScale))

	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, big.NewInt(n), remainder)

	absRemainderTwice := new(big.Int).Abs(remainder)
	absRemainderTwice.Lsh(absRemainderTwice, 1)
	absDivisor := big.NewInt(n) // n > 0 always (len(items) > 0, checked above)

	switch absRemainderTwice.Cmp(absDivisor) {
	case 1:
		// Strictly more than halfway: round away from zero.
		quotient = roundQuotientAwayFromZero(quotient)
	case 0:
		// Exactly halfway: round to even (AC4's tie case).
		absQuotient := new(big.Int).Abs(quotient)
		if absQuotient.Bit(0) == 1 { // quotient is odd
			quotient = roundQuotientAwayFromZero(quotient)
		}
	}
	// case -1 (strictly less than halfway): QuoRem's truncation toward
	// zero is already the correctly-rounded result.

	if !quotient.IsInt64() {
		return Decimal{}, fmt.Errorf(
			"expr: avg: quotient coefficient %s does not fit int64 (%d operand(s))",
			quotient.String(), len(items),
		)
	}

	// CORRECTION (QA review Finding 10, Minor). total.Exponent is
	// already bounded by SumDecimals' own AC6 check, but subtracting
	// avgExtraScale can still push the RESULT's exponent past
	// maxDecimalExponentMagnitude (measured: two operands at
	// -maxDecimalExponentMagnitude average to Exponent -100004, a
	// value NewDecimal itself would reject). *Do-not-re-open* item 6's
	// principle — a bound breach is a located Error, never a silent
	// widening — applies to every bound the kernel can breach, not
	// only the alignment spread's.
	resultExponent := total.Exponent - avgExtraScale
	if resultExponent > maxDecimalExponentMagnitude || resultExponent < -maxDecimalExponentMagnitude {
		return Decimal{}, fmt.Errorf(
			"expr: avg: result exponent %d (sum exponent %d minus avgExtraScale %d) exceeds magnitude bound %d",
			resultExponent, total.Exponent, avgExtraScale, maxDecimalExponentMagnitude,
		)
	}

	return Decimal{Coefficient: quotient.Int64(), Exponent: resultExponent}, nil
}

// roundQuotientAwayFromZero increments q's magnitude by one, preserving
// its sign — the "round away from zero" half of round-half-to-even.
func roundQuotientAwayFromZero(q *big.Int) *big.Int {
	if q.Sign() < 0 {
		return new(big.Int).Sub(q, big.NewInt(1))
	}
	return new(big.Int).Add(q, big.NewInt(1))
}
