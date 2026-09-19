package expr

import (
	"math"
	"strings"
	"testing"
)

// mustDecimal is a test-only helper: parse a literal via NewDecimal and
// fail the test immediately on error, so every reducer test's corpus
// reads as a plain list of literal strings.
func mustDecimal(t *testing.T, literal string) Decimal {
	t.Helper()
	d, err := NewDecimal(literal)
	if err != nil {
		t.Fatalf("NewDecimal(%q): %v", literal, err)
	}
	return d
}

func mustDecimals(t *testing.T, literals ...string) []Decimal {
	t.Helper()
	out := make([]Decimal, len(literals))
	for i, l := range literals {
		out[i] = mustDecimal(t, l)
	}
	return out
}

// --- AC1: the kernel is arithmetic-only -------------------------------

// TestSumDecimalsEmptyReturnsIdentity is D-3.1a.2: SumDecimals(nil) (and
// any empty slice) returns the additive identity, never an error. This
// is the ASYMMETRIC half of D-3.1a.2 — see
// TestAvgDecimalsEmptyReturnsError for the other half.
func TestSumDecimalsEmptyReturnsIdentity(t *testing.T) {
	got, err := SumDecimals(nil)
	if err != nil {
		t.Fatalf("SumDecimals(nil): unexpected error: %v", err)
	}
	want := Decimal{Coefficient: 0, Exponent: 0}
	if got != want {
		t.Fatalf("SumDecimals(nil) = %+v, want the identity %+v", got, want)
	}

	got, err = SumDecimals([]Decimal{})
	if err != nil {
		t.Fatalf("SumDecimals([]Decimal{}): unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("SumDecimals([]Decimal{}) = %+v, want the identity %+v", got, want)
	}
}

// TestAvgDecimalsEmptyReturnsError is D-3.1a.2's other half: unlike sum,
// avg has no identity element, so an empty input is a located error, not
// a silently invented zero.
func TestAvgDecimalsEmptyReturnsError(t *testing.T) {
	if _, err := AvgDecimals(nil); err == nil {
		t.Fatal("AvgDecimals(nil): expected an error (avg has no identity element), got nil")
	}
	if _, err := AvgDecimals([]Decimal{}); err == nil {
		t.Fatal("AvgDecimals([]Decimal{}): expected an error, got nil")
	}
}

// --- AC2/AC3: alignment, big.Int accumulation, single narrowing -------

// TestSumDecimalsAlignsToMinimumExponent is AC3: the result carries the
// MAXIMUM operand scale (the MINIMUM operand exponent), and trailing
// zeros are never lost — D-1.6.1's AC2 distinctness ("1.50" != "1.5")
// survives addition.
func TestSumDecimalsAlignsToMinimumExponent(t *testing.T) {
	items := mustDecimals(t, "1.50", "2.5", "0.001")
	got, err := SumDecimals(items)
	if err != nil {
		t.Fatalf("SumDecimals: unexpected error: %v", err)
	}
	// 1.500 + 2.500 + 0.001 = 4.001, at the minimum operand exponent -3.
	want := Decimal{Coefficient: 4001, Exponent: -3}
	if got != want {
		t.Fatalf("SumDecimals(1.50, 2.5, 0.001) = %+v, want %+v", got, want)
	}
}

// TestSumDecimalsSingleOperandIsIdentity confirms a one-item sum simply
// returns that operand's own scale untouched — the alignment loop's
// degenerate case.
func TestSumDecimalsSingleOperandIsIdentity(t *testing.T) {
	item := mustDecimal(t, "1.50")
	got, err := SumDecimals([]Decimal{item})
	if err != nil {
		t.Fatalf("SumDecimals: unexpected error: %v", err)
	}
	if got != item {
		t.Fatalf("SumDecimals([1.50]) = %+v, want %+v", got, item)
	}
}

// --- AC4: avg's declared division scale, and its tie case -------------

// TestAvgDecimalsScaleIsMaxOperandScalePlusFour is AC4: the division
// scale is (maximum operand scale + 4), declared once as a property of
// the operation.
func TestAvgDecimalsScaleIsMaxOperandScalePlusFour(t *testing.T) {
	// "1.50" and "2.50" both carry scale 2 (exponent -2); their average
	// must be reported at scale 2+4=6 (exponent -6).
	items := mustDecimals(t, "1.50", "2.50")
	got, err := AvgDecimals(items)
	if err != nil {
		t.Fatalf("AvgDecimals: unexpected error: %v", err)
	}
	if got.Exponent != -6 {
		t.Fatalf("AvgDecimals(1.50, 2.50).Exponent = %d, want -6 (max operand scale 2, plus 4)", got.Exponent)
	}
	// (1.50+2.50)/2 = 2.00, at scale 6: coefficient 2000000.
	want := Decimal{Coefficient: 2_000_000, Exponent: -6}
	if got != want {
		t.Fatalf("AvgDecimals(1.50, 2.50) = %+v, want %+v", got, want)
	}
}

// TestAvgDecimalsRoundsHalfToEven exercises AC4's tie case explicitly,
// both directions (ties DOWN to an already-even neighbour, ties UP to
// an even neighbour from an odd one), so the half-to-even half is
// exercised rather than merely assumed.
//
// Both cases use n=32 operands, all zero except one, at exponent -2, so
// SumDecimals' total.Coefficient is exactly the one nonzero operand's
// coefficient and total.Exponent is -2 (avg's exponent is then -6).
// AvgDecimals' numerator is total.Coefficient * 10^avgExtraScale
// (10^4); dividing by n=32:
//   - coefficient 1: numerator 10000, quotient 312 (EVEN), remainder 16
//     (2*16 == 32 == n: an exact tie) — must stay 312.
//   - coefficient 3: numerator 30000, quotient 937 (ODD), remainder 16
//     (an exact tie again) — must round UP to 938.
//
// Both were computed independently in Python and are asserted as
// literal constants here, not re-derived from the production code.
func TestAvgDecimalsRoundsHalfToEven(t *testing.T) {
	const n = 32

	buildItems := func(firstCoefficient int64) []Decimal {
		items := make([]Decimal, n)
		for i := range items {
			items[i] = Decimal{Coefficient: 0, Exponent: -2}
		}
		items[0] = Decimal{Coefficient: firstCoefficient, Exponent: -2}
		return items
	}

	cases := []struct {
		name             string
		firstCoefficient int64
		want             Decimal
	}{
		{"tie resolves DOWN to an already-even quotient", 1, Decimal{Coefficient: 312, Exponent: -6}},
		{"tie resolves UP from an odd quotient to the even neighbour", 3, Decimal{Coefficient: 938, Exponent: -6}},
		// QA review Finding 12 (Minor): no negative operand appeared
		// anywhere in this story's corpus or unit tests, leaving the
		// whole sign path (roundQuotientAwayFromZero's q.Sign() < 0
		// branch, QuoRem's sign-of-dividend remainder, and the
		// negative half of the half-to-even tie) unexercised. These
		// two cases mirror the positive pair above with the sign
		// flipped; both computed independently (by hand and
		// cross-checked in Python, matching the reviewer's own
		// -30000/32 -> q=-937, r=-16, tie, odd -> -938 computation)
		// and asserted as literal constants, not re-derived from the
		// production code.
		{"negative tie resolves DOWN (magnitude) to an already-even quotient", -1, Decimal{Coefficient: -312, Exponent: -6}},
		{"negative tie resolves UP (magnitude) from an odd quotient to the even neighbour", -3, Decimal{Coefficient: -938, Exponent: -6}},
	}
	for _, c := range cases {
		got, err := AvgDecimals(buildItems(c.firstCoefficient))
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", c.name, err)
		}
		if got != c.want {
			t.Fatalf("%s: AvgDecimals(...) = %+v, want %+v", c.name, got, c.want)
		}
	}
}

// TestSumDecimalsMixedSignOperands is the other half of QA review
// Finding 12: SumDecimals' own sign path (a negative operand aligned
// and accumulated alongside positive ones) had no corpus coverage
// either. -1.500 + 2.500 + 0.001 = 1.001, at the minimum operand
// exponent -3 (independently computed by hand).
func TestSumDecimalsMixedSignOperands(t *testing.T) {
	items := mustDecimals(t, "-1.50", "2.5", "0.001")
	got, err := SumDecimals(items)
	if err != nil {
		t.Fatalf("SumDecimals: unexpected error: %v", err)
	}
	want := Decimal{Coefficient: 1001, Exponent: -3}
	if got != want {
		t.Fatalf("SumDecimals(-1.50, 2.5, 0.001) = %+v, want %+v", got, want)
	}
}

// --- AC5: accumulator overflow is a located Error, never a widening ---

// TestSumDecimalsOverflowIsLocatedError is AC5's red-proof: a corpus
// that overflows int64 only AFTER accumulation (each operand fits on
// its own — constructed directly, bypassing NewDecimal's existing
// per-literal bound entirely, per AC5's own instruction) is a located
// error naming the operation, the operand count, and the bound
// breached — never a wrap, never a truncation.
func TestSumDecimalsOverflowIsLocatedError(t *testing.T) {
	items := []Decimal{
		{Coefficient: math.MaxInt64, Exponent: 0},
		{Coefficient: math.MaxInt64, Exponent: 0},
	}
	_, err := SumDecimals(items)
	if err == nil {
		t.Fatal("SumDecimals: expected an overflow error accumulating two MaxInt64 coefficients, got nil")
	}
	if !strings.Contains(err.Error(), "sum") {
		t.Errorf("overflow error does not name the operation (\"sum\"): %v", err)
	}
	if !strings.Contains(err.Error(), "2 operand") {
		t.Errorf("overflow error does not name the operand count: %v", err)
	}
	if !strings.Contains(err.Error(), "does not fit int64") {
		t.Errorf("overflow error does not name the bound breached: %v", err)
	}
}

// TestAvgDecimalsQuotientOverflowIsLocatedError: avg's own quotient
// narrowing (distinct from sum's) can also overflow — the *10^4 scale
// widening followed by division can still exceed int64 for an
// already-near-limit coefficient with very few operands.
func TestAvgDecimalsQuotientOverflowIsLocatedError(t *testing.T) {
	items := []Decimal{{Coefficient: math.MaxInt64, Exponent: 0}}
	_, err := AvgDecimals(items)
	if err == nil {
		t.Fatal("AvgDecimals: expected an overflow error (coefficient * 10^4 exceeds int64), got nil")
	}
	if !strings.Contains(err.Error(), "does not fit int64") {
		t.Errorf("overflow error does not name the bound breached: %v", err)
	}
}

// TestAvgDecimalsResultExponentBoundIsEnforced is QA review Finding 10's
// red-proof (Minor): the reviewer's own exact reproduction. Two operands
// at Exponent -maxDecimalExponentMagnitude average to a RESULT exponent
// of -maxDecimalExponentMagnitude-avgExtraScale (-100004), a value
// NewDecimal itself would reject as out of bounds — before this fix,
// AvgDecimals minted it silently with a nil error. *Do-not-re-open*
// item 6: a bound breach is a located Error, never a silent widening.
func TestAvgDecimalsResultExponentBoundIsEnforced(t *testing.T) {
	items := []Decimal{
		{Coefficient: 1, Exponent: -maxDecimalExponentMagnitude},
		{Coefficient: 1, Exponent: -maxDecimalExponentMagnitude},
	}
	got, err := AvgDecimals(items)
	if err == nil {
		t.Fatalf("AvgDecimals: expected a result-exponent bound error, got a silent result %+v (Exponent %d breaches the %d bound NewDecimal itself enforces)", got, got.Exponent, maxDecimalExponentMagnitude)
	}
	if !strings.Contains(err.Error(), "exceeds magnitude bound") {
		t.Errorf("error does not name the bound breached: %v", err)
	}
}

// --- AC6: alignment spread is bounded BEFORE any shift is computed ----

// TestSumDecimalsAlignmentSpreadIsBoundedBeforeShifting is AC6: an
// operand pair whose exponents differ by far more than
// maxDecimalExponentMagnitude must fail IMMEDIATELY with a located
// error, never by first attempting to build the shift.
//
// The spread below (100,000,000 — a thousand times
// maxDecimalExponentMagnitude) is deliberately astronomical: were the
// bound check ever moved to AFTER the shift is computed, satisfying it
// would require constructing and multiplying by a hundred-million-digit
// power of ten — tens of megabytes of big.Int, taking long enough that
// this single test would itself blow the whole suite's run budget. A
// correctly-ordered check returns from the call below effectively
// instantly; this test relies on that contrast (via the test binary's
// own timeout) rather than importing "time" to measure it directly —
// AD-1 bans "time" even in _test.go files under folio-go/.
func TestSumDecimalsAlignmentSpreadIsBoundedBeforeShifting(t *testing.T) {
	const hugeSpread = 100_000_000 // 1000x maxDecimalExponentMagnitude
	items := []Decimal{
		{Coefficient: 1, Exponent: 0},
		{Coefficient: 1, Exponent: hugeSpread},
	}

	_, err := SumDecimals(items)
	if err == nil {
		t.Fatal("SumDecimals: expected an alignment-spread error, got nil")
	}
	if !strings.Contains(err.Error(), "alignment spread") {
		t.Errorf("error does not name the alignment spread: %v", err)
	}
	if !strings.Contains(err.Error(), "100000000") {
		t.Errorf("error does not name the measured spread: %v", err)
	}
}

// TestSumDecimalsAlignmentSpreadDoesNotOverflow is QA review Finding 3's
// red-proof (Major): the reviewer's own exact reproduction. Before the
// fix, "spread := maxExp - minExp" computed in plain int WRAPPED for
// this pair (math.MinInt and math.MaxInt together), landing back inside
// maxDecimalExponentMagnitude and sailing the AC6 bound check past the
// single most extreme possible breach — SumDecimals returned
// {Coefficient: 2, Exponent: math.MinInt} with a NIL error, an
// arithmetically meaningless value with no shift ever validated. The
// spread is now computed in math/big (exact, cannot wrap), so this
// exact input must be a located error, never a silent wrap.
func TestSumDecimalsAlignmentSpreadDoesNotOverflow(t *testing.T) {
	items := []Decimal{
		{Coefficient: 1, Exponent: math.MinInt},
		{Coefficient: 1, Exponent: math.MaxInt},
	}
	got, err := SumDecimals(items)
	if err == nil {
		t.Fatalf("SumDecimals(MinInt, MaxInt exponents): expected an alignment-spread error, got a silent result %+v (this is the reviewer's own overflow reproduction)", got)
	}
	if !strings.Contains(err.Error(), "alignment spread") {
		t.Errorf("error does not name the alignment spread: %v", err)
	}
}
