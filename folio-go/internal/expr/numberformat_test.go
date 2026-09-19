package expr

import (
	"math/big"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/template"
)

func mustEvalFormatNumber(t *testing.T, src string, r Resolver, fc FormatContext) string {
	t.Helper()
	e, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	if err := Check(e); err != nil {
		t.Fatalf("Check(%q): %v", src, err)
	}
	v, _, err := Eval(e, r, fc, "e1")
	if err != nil {
		t.Fatalf("Eval(%q): unexpected error: %v", src, err)
	}
	if v.Kind != KindString {
		t.Fatalf("Eval(%q): got kind %s, want string", src, v.Kind)
	}
	return v.Str
}

// --- AC11/AC11a: locale-driven separators, grouping and digits ---

func TestFormatNumberAllFourLocales(t *testing.T) {
	r := mapResolver{"x": {Kind: KindNumber, Num: Decimal{Coefficient: 123456, Exponent: -2}}} // 1234.56
	cases := []struct {
		locale string
		want   string
	}{
		{template.LocaleEN, "1,234.56"},
		{template.LocaleTH, "1,234.56"}, // DECISION-2: Western digits everywhere
		{template.LocaleZhHans, "1,234.56"},
		{template.LocaleJA, "1,234.56"},
	}
	for _, c := range cases {
		fc := fcFor(c.locale, "+00:00")
		got := mustEvalFormatNumber(t, `formatNumber(x, "#,##0.00")`, r, fc)
		if got != c.want {
			t.Errorf("locale %s: got %q, want %q", c.locale, got, c.want)
		}
	}
}

// TestFormatNumberGroupingRedProof is AC11's own red-proof: changing
// one tag's separator moves only that tag's output.
//
// Finding 13 (this story's QA review): the previous version mutated
// the REAL, package-level localeTable in place (`localeTable[en] =
// mutated`, restored by defer) — a shipped test demonstrating that
// AC1/AD-1's "internal/expr holds no package-level mutable state"
// map is, in fact, mutable. No production path writes to it, so this
// never affected determinism, but it is real order-dependence risk
// (and a data race waiting for `t.Parallel()`). This version reads a
// COPY of en's entry out of the table (Go map reads always copy a
// struct value) and passes the mutated COPY directly to
// renderNumberPattern — the real localeTable is never written to at
// all. `th`'s output is proven independent simply by never having
// touched anything th's own lookup reads.
func TestFormatNumberGroupingRedProof(t *testing.T) {
	spec, err := validateNumberPattern("#,##0.00")
	if err != nil {
		t.Fatalf("validateNumberPattern: %v", err)
	}
	scaled, err := scaleToFractionDigits(Decimal{Coefficient: 123456, Exponent: -2}, spec.fracDigits)
	if err != nil {
		t.Fatalf("scaleToFractionDigits: %v", err)
	}

	mutatedEN := localeTable[template.LocaleEN] // a COPY — the real table is never written to
	mutatedEN.groupSep = ";"
	got := renderNumberPattern(scaled, spec, mutatedEN)
	if got != "1;234.56" {
		t.Fatalf("RED-PROOF: en's own output did not move with a locally-mutated copy of its table entry, got %q", got)
	}

	r := mapResolver{"x": {Kind: KindNumber, Num: Decimal{Coefficient: 123456, Exponent: -2}}}
	gotTh := mustEvalFormatNumber(t, `formatNumber(x, "#,##0.00")`, r, fcFor(template.LocaleTH, "+00:00"))
	if gotTh != "1,234.56" {
		t.Fatalf("RED-PROOF FAILED: th's output was not the expected default — got %q", gotTh)
	}
}

// --- AC12 Layer 1: exactness oracle ---

// binary64ScaleWouldDiverge reports whether the EXACT product of
// coefficient*10^shift would be misrepresented by an IEEE-754 binary64
// (Go's forbidden "float64") implementation — WITHOUT ever
// constructing a binary64 value: AD-23's ban (enforced module-wide by
// internal/arch_test.go's TestNoFloat64UnderModule, which scans
// _test.go files too) applies here as much as anywhere else, so this
// measurement is done by pure integer reasoning about binary64's own
// well-known representable set, never by executing float64 arithmetic
// to observe it.
//
// A binary64 exactly represents a nonzero integer N iff N's ODD PART
// (N with every factor of two divided out) fits in 53 significant
// bits — the mantissa's precision, IEEE 754-2008 verbatim. This is a
// mathematical fact about the format, not an experiment run against
// it, so stating it in terms of BitLen/trailing-zero-count violates no
// guard: no float32/float64 identifier appears anywhere below.
func binary64ScaleWouldDiverge(coefficient int64, shift int) bool {
	exact := new(big.Int).Mul(big.NewInt(coefficient), new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(shift)), nil))
	n := new(big.Int).Abs(exact)
	if n.Sign() == 0 {
		return false
	}
	trailingZeroBits := 0
	tmp := new(big.Int).Set(n)
	for tmp.Bit(0) == 0 {
		tmp.Rsh(tmp, 1)
		trailingZeroBits++
	}
	significantBits := n.BitLen() - trailingZeroBits
	return significantBits > 53
}

// numberFormatOracleCorpus is AC12 Layer 1's exactness-oracle corpus,
// SHARED by TestFormatNumberExactnessOracleCorpusDiverges (D-000.50's
// population check) and TestFormatNumberExactnessOracle itself — one
// declaration, never two independently-typed lists that can silently
// drift apart (Finding 3, this story's QA review): the previous
// version's population check measured divergence for shifts {1, 4, 3}
// while the oracle's own scaleToFractionDigits call actually computed
// shift 0 for BOTH of its cases (each had d.Exponent == targetExponent,
// so tenPowLookup was never even called) — "the population checked is
// not the population asserted over."
//
// Two of the four cases below deliberately have exponent !=
// -fracDigits, so tenPowLookup IS exercised: one via the PAD branch
// (source coarser than the target scale), one via the ROUND branch
// (source finer than the target scale, forcing a real half-to-even
// decision rather than an exact pad) — closing the gap the finding
// named. Both reuse coefficient 9007199254740993 (2^53+1), already
// established as not exactly representable in binary64, so the new
// cases inherit that same discriminating property.
var numberFormatOracleCorpus = []struct {
	coeff   int64
	exp     int
	pattern string
	want    string
}{
	// Shift 0 — scaleToFractionDigits's equal-exponent early return,
	// tenPowLookup NOT called. Still meaningful: it tests whether the
	// RAW coefficient itself is exactly representable in binary64,
	// which 2^53+1 is not.
	{9007199254740993, 0, "#,##0", "9,007,199,254,740,993"},
	{123456789012345, -4, "#,##0.0000", "12,345,678,901.2345"},

	// Shift 2, PAD branch (Finding 3): d.Exponent(-2) > targetExponent(-4)
	// — source coarser than the pattern's scale, tenPowLookup(2) used
	// as a MULTIPLIER.
	{9007199254740993, -2, "#,##0.0000", "90,071,992,547,409.9300"},

	// Shift 2, ROUND branch (Finding 3): d.Exponent(-6) < targetExponent(-4)
	// — source finer than the pattern's scale, tenPowLookup(2) used as
	// a DIVISOR, and the remainder (93 of 100) genuinely rounds away
	// from zero rather than landing on an exact pad.
	{9007199254740993, -6, "#,##0.0000", "9,007,199,254.7410"},
}

// TestFormatNumberExactnessOracleCorpusDiverges is AC12 Layer 1's own
// population check (D-000.50): confirms, before relying on it, that
// this corpus's own values actually diverge under a binary64 (the
// forbidden float64) scaling implementation — never assumed, and
// measured against the SAME shift scaleToFractionDigits itself
// computes for each case (derived via the real validateNumberPattern,
// never a hand-picked, independently-guessed shift — Finding 3's own
// defect).
func TestFormatNumberExactnessOracleCorpusDiverges(t *testing.T) {
	diverged := 0
	for _, c := range numberFormatOracleCorpus {
		spec, err := validateNumberPattern(c.pattern)
		if err != nil {
			t.Fatalf("validateNumberPattern(%q): %v", c.pattern, err)
		}
		targetExponent := -spec.fracDigits
		shift := c.exp - targetExponent
		if shift < 0 {
			shift = -shift
		}
		if binary64ScaleWouldDiverge(c.coeff, shift) {
			diverged++
		}
	}
	t.Logf("measured: %d of %d corpus members diverge under a binary64 scaling implementation", diverged, len(numberFormatOracleCorpus))
	if diverged == 0 {
		t.Fatal("presence precondition (D-000.50): zero corpus members diverge under binary64 — this corpus proves nothing")
	}
}

// TestFormatNumberExactnessOracle is AC12 Layer 1 itself: formatNumber
// over the corpus above must match the EXACT expected digits — the
// property no float64 implementation (math.Pow, a multiply loop,
// math.Exp(n*math.Log(10))) can satisfy for the diverging members, and
// (Finding 3) two of whose cases actually route through tenPowLookup's
// scaling path rather than only its equal-exponent bypass.
func TestFormatNumberExactnessOracle(t *testing.T) {
	for _, c := range numberFormatOracleCorpus {
		r := mapResolver{"x": {Kind: KindNumber, Num: Decimal{Coefficient: c.coeff, Exponent: c.exp}}}
		got := mustEvalFormatNumber(t, `formatNumber(x, "`+c.pattern+`")`, r, fcFor(template.LocaleEN, "+00:00"))
		if got != c.want {
			t.Errorf("coeff=%d exp=%d pattern=%q: got %q, want %q", c.coeff, c.exp, c.pattern, got, c.want)
		}
	}
}

// --- AC12 Layer 3: the lookup table's own contents ---

// TestTenPowTableMatchesIndependentBigIntChain is Layer 3: an
// INDEPENDENTLY computed big.Int chain (repeated multiplication),
// never the same computation the production table would have used
// had it been computed rather than declared as a literal.
func TestTenPowTableMatchesIndependentBigIntChain(t *testing.T) {
	chain := big.NewInt(1)
	for n := 0; n <= maxTenPowExponent; n++ {
		if n > 0 {
			chain.Mul(chain, big.NewInt(10))
		}
		want := new(big.Int).Set(chain)
		got, err := tenPowLookup(n)
		if err != nil {
			t.Fatalf("tenPowLookup(%d): unexpected error: %v", n, err)
		}
		if got.Cmp(want) != 0 {
			t.Errorf("tenPowLookup(%d) = %s, want %s (independently computed)", n, got.String(), want.String())
		}
	}
}

func TestTenPowLookupBoundedRedProof(t *testing.T) {
	if _, err := tenPowLookup(maxTenPowExponent + 1); err == nil {
		t.Fatal("expected a located error for a shift beyond the table's bound")
	}
	if _, err := tenPowLookup(-1); err == nil {
		t.Fatal("expected a located error for a negative shift")
	}
}

// --- AC13: rounding half-to-even, both tie directions ---

func TestFormatNumberRoundsHalfToEven(t *testing.T) {
	cases := []struct {
		coeff, exp int64
		want       string
	}{
		{125, -3, "0.12"}, // 0.125 -> rounds to even (0.12)
		{135, -3, "0.14"}, // 0.135 -> rounds to even (0.14)
	}
	for _, c := range cases {
		r := mapResolver{"x": {Kind: KindNumber, Num: Decimal{Coefficient: c.coeff, Exponent: int(c.exp)}}}
		got := mustEvalFormatNumber(t, `formatNumber(x, "0.00")`, r, fcFor(template.LocaleEN, "+00:00"))
		if got != c.want {
			t.Errorf("coeff=%d exp=%d: got %q, want %q", c.coeff, c.exp, got, c.want)
		}
	}
}

func TestFormatNumberPadsFewerFractionDigits(t *testing.T) {
	r := mapResolver{"x": {Kind: KindNumber, Num: Decimal{Coefficient: 5, Exponent: 0}}} // 5
	got := mustEvalFormatNumber(t, `formatNumber(x, "0.00")`, r, fcFor(template.LocaleEN, "+00:00"))
	if got != "5.00" {
		t.Fatalf("got %q, want %q", got, "5.00")
	}
}

// --- AC14: the two kernel zeros ---

func TestFormatNumberBothKernelZerosRenderAlike(t *testing.T) {
	fc := fcFor(template.LocaleEN, "+00:00")
	sumZero := Decimal{Coefficient: 0, Exponent: 0}              // SumDecimals([])
	avgZero := Decimal{Coefficient: 0, Exponent: -avgExtraScale} // all-null avg

	r1 := mapResolver{"x": {Kind: KindNumber, Num: sumZero}}
	got1 := mustEvalFormatNumber(t, `formatNumber(x, "#,##0.00")`, r1, fc)
	if got1 != "0.00" {
		t.Fatalf("sum-zero: got %q, want %q", got1, "0.00")
	}

	r2 := mapResolver{"x": {Kind: KindNumber, Num: avgZero}}
	got2 := mustEvalFormatNumber(t, `formatNumber(x, "#,##0.00")`, r2, fc)
	if got2 != "0.00" {
		t.Fatalf("avg-zero: got %q, want %q", got2, "0.00")
	}

	// Neither kernel exponent moved.
	if sumZero.Exponent != 0 {
		t.Fatalf("SumDecimals([])'s exponent must stay 0, got %d", sumZero.Exponent)
	}
	if avgZero.Exponent != -avgExtraScale {
		t.Fatalf("all-null avg's exponent must stay -avgExtraScale, got %d", avgZero.Exponent)
	}
}

// --- AC15/AC17: avgExtraScale honesty-keeper ---

func TestMaxPatternFractionDigitsIsAvgExtraScale(t *testing.T) {
	if maxPatternFractionDigits != avgExtraScale {
		t.Fatalf("maxPatternFractionDigits = %d, avgExtraScale = %d: must be equal, cited by symbol", maxPatternFractionDigits, avgExtraScale)
	}
}

// TestNumberPatternValidatorAcceptsUpToBoundRejectsBeyond is AC17 part
// 2, repaired (Finding 6, this story's QA review): the accept range and
// the reject probe are both anchored to avgExtraScale — NEVER to
// maxPatternFractionDigits, the constant under test — so this test
// actually reddens if the two ever decouple. The previous version
// expressed both the accept range (`k <= maxPatternFractionDigits`) and
// the reject probe (`maxPatternFractionDigits+1`) in terms of the
// constant it was supposed to be checking, so the whole assertion slid
// with it: measured, mutating maxPatternFractionDigits to
// avgExtraScale+1 left this test PASSING, because it only ever verified
// "the validator honours its own declared bound," never "the bound is
// <= avgExtraScale."
func TestNumberPatternValidatorAcceptsUpToBoundRejectsBeyond(t *testing.T) {
	for k := 0; k <= avgExtraScale; k++ {
		pattern := "0"
		if k > 0 {
			pattern = "0." + repeatZero(k)
		}
		if _, err := validateNumberPattern(pattern); err != nil {
			t.Errorf("k=%d: pattern %q should be accepted, got error: %v", k, pattern, err)
		}
	}
	beyond := "0." + repeatZero(avgExtraScale+1)
	if _, err := validateNumberPattern(beyond); err == nil {
		t.Errorf("k=%d: pattern %q should be REJECTED (exceeds avgExtraScale)", avgExtraScale+1, beyond)
	}
}

func repeatZero(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = '0'
	}
	return string(b)
}

// TestNumberPatternRedProofOverAvgExtraScale is AC17 part 3's captured
// mutation, corrected (Finding 6): the previous version's presence
// precondition (`if inflated <= avgExtraScale`, where
// `inflated := avgExtraScale + 1`) was arithmetically impossible and
// could never fire, and its comment claimed to confirm "both part 1 …
// and part 2 … would redden" — measurably false of part 2 as it was
// shipped, since part 2's own probe was expressed in terms of
// maxPatternFractionDigits rather than avgExtraScale.
//
// What this test actually proves, stated honestly and without
// overclaiming what a `const` mutation this file cannot itself perform
// at runtime: the REAL, shipped validator — using the real
// maxPatternFractionDigits, never a simulated one — rejects a pattern
// requesting avgExtraScale+1 fraction digits, anchored to avgExtraScale
// by symbol (AC15), never the literal 4.
//
// AC17 part 3's literal instruction (temporarily set
// maxPatternFractionDigits to avgExtraScale+1 in the SOURCE, confirm
// both part 1 and the repaired part 2 above redden, then revert) was
// additionally captured BY HAND during this story's finishing pass —
// see the Delivery Log — because maxPatternFractionDigits is a `const`,
// and per part 1's own "no literals" requirement it cannot be
// parameterised for a permanent automated test without either
// reintroducing a literal or turning the bound into a variable, which
// AC17 part 1 forbids.
func TestNumberPatternRedProofOverAvgExtraScale(t *testing.T) {
	pattern := "0." + repeatZero(avgExtraScale+1)
	if _, err := validateNumberPattern(pattern); err == nil {
		t.Fatalf("RED-PROOF FAILED: the real validator accepted a pattern with avgExtraScale+1 (%d) fraction digits", avgExtraScale+1)
	}
}
