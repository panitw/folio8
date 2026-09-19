package expr

import (
	"strings"
	"testing"
)

// TestNewDecimalPreservesLiteralPrecision is AC2's minimum table:
// distinct literals stay distinct, including "1.50" != "1.5".
func TestNewDecimalPreservesLiteralPrecision(t *testing.T) {
	cases := []struct {
		literal string
		want    Decimal
	}{
		{"1.50", Decimal{150, -2}},
		{"1.5", Decimal{15, -1}},
		{"36", Decimal{36, 0}},
		{"1e3", Decimal{1, 3}},
	}
	for _, c := range cases {
		got, err := NewDecimal(c.literal)
		if err != nil {
			t.Errorf("NewDecimal(%q): unexpected error: %v", c.literal, err)
			continue
		}
		if got != c.want {
			t.Errorf("NewDecimal(%q) = %+v, want %+v", c.literal, got, c.want)
		}
	}
}

// TestNewDecimalCoefficientBound is AC3: only significant digits count
// (leading zeros stripped, trailing zeros never stripped). A 20-digit
// literal with no leading zeros is an error; a value whose only
// significant digit is preceded by nineteen leading zeros is legal.
func TestNewDecimalCoefficientBound(t *testing.T) {
	got, err := NewDecimal("0.00000000000000000001")
	if err != nil {
		t.Fatalf("NewDecimal(\"0.00000000000000000001\"): unexpected error: %v", err)
	}
	want := Decimal{Coefficient: 1, Exponent: -20}
	if got != want {
		t.Fatalf("NewDecimal(\"0.00000000000000000001\") = %+v, want %+v", got, want)
	}

	if _, err := NewDecimal("12345678901234567890"); err == nil {
		t.Fatal("NewDecimal(\"12345678901234567890\"): expected an error (20 significant digits, exceeds int64)")
	}
}

// TestNewDecimalAllZeros is AC3a: "0.00" strips to the EMPTY digit
// string and must yield {0,-2} — not an error, not a lost scale. This
// is the case a naive implementation gets wrong in both directions
// (erroring on empty digits, or silently returning exponent 0).
func TestNewDecimalAllZeros(t *testing.T) {
	cases := []struct {
		literal string
		want    Decimal
	}{
		{"0.00", Decimal{0, -2}},
		{"0.0", Decimal{0, -1}},
		{"0", Decimal{0, 0}},
		{"-0.00", Decimal{0, -2}},
	}
	for _, c := range cases {
		got, err := NewDecimal(c.literal)
		if err != nil {
			t.Errorf("NewDecimal(%q): unexpected error: %v", c.literal, err)
			continue
		}
		if got != c.want {
			t.Errorf("NewDecimal(%q) = %+v, want %+v", c.literal, got, c.want)
		}
	}
}

// TestNewDecimalExponentBound is AC4b: Decimal applies its OWN tighter
// exponent check, on top of the shared splitter's wider one. Tested at
// the bound and one past it (binding), on BOTH the positive and
// negative side (QA Finding 9, this story's review, Minor: only the
// positive side had a test — the check is a two-sided comparison whose
// lower arm was unexercised).
func TestNewDecimalExponentBound(t *testing.T) {
	if _, err := NewDecimal("1e100000"); err != nil {
		t.Errorf("exponent magnitude at Decimal's own bound (%d) must be legal: %v", maxDecimalExponentMagnitude, err)
	}
	if _, err := NewDecimal("1e100001"); err == nil {
		t.Errorf("exponent magnitude one past Decimal's own bound (%d) must be a located error", maxDecimalExponentMagnitude+1)
	}
	if _, err := NewDecimal("1e-100000"); err != nil {
		t.Errorf("exponent magnitude at Decimal's own bound, negative side (-%d) must be legal: %v", maxDecimalExponentMagnitude, err)
	}
	if _, err := NewDecimal("1e-100001"); err == nil {
		t.Errorf("exponent magnitude one past Decimal's own bound, negative side (-%d) must be a located error", maxDecimalExponentMagnitude+1)
	}
}

// TestNewDecimalConstructedExponentBoundNoENotation is QA Finding 3
// (this story's review, Major): the bound was previously checked
// against the literal's raw "e" exponent, never against the
// CONSTRUCTED Decimal.Exponent (exp - len(fracPart)). A literal with no
// "e" notation at all never touched the old check regardless of how
// negative its constructed Exponent became. Measured pre-fix: "0." +
// 199999 zeros + "1" produced {Coefficient:1, Exponent:-200000} with
// err == nil, twice the bound. Tested at the constructed bound and one
// past it (binding, AC4b).
func TestNewDecimalConstructedExponentBoundNoENotation(t *testing.T) {
	// fracPart = (maxDecimalExponentMagnitude-1) zeros + "1", so
	// len(fracPart) == maxDecimalExponentMagnitude and the constructed
	// exponent (0 - len(fracPart)) lands exactly at the bound.
	atBound := "0." + strings.Repeat("0", maxDecimalExponentMagnitude-1) + "1"
	got, err := NewDecimal(atBound)
	if err != nil {
		t.Fatalf("constructed exponent magnitude at the bound (-%d) must be legal: %v", maxDecimalExponentMagnitude, err)
	}
	if got.Exponent != -maxDecimalExponentMagnitude {
		t.Fatalf("NewDecimal(atBound).Exponent = %d, want %d", got.Exponent, -maxDecimalExponentMagnitude)
	}

	onePast := "0." + strings.Repeat("0", maxDecimalExponentMagnitude) + "1"
	if _, err := NewDecimal(onePast); err == nil {
		t.Fatal("constructed exponent magnitude one past the bound must be a located error (no \"e\" notation ever reached the old raw-exp check)")
	}
}

// TestNewDecimalConstructedExponentBoundStacked is QA Finding 3's other
// half: a literal that stacks a fractional part on top of an explicit
// exponent, where neither half alone crosses the bound but the
// CONSTRUCTED exponent (exp - len(fracPart)) does. Measured pre-fix:
// "0." + 999999 zeros + "1e-100000" produced Exponent:-1100000 with
// err == nil.
func TestNewDecimalConstructedExponentBoundStacked(t *testing.T) {
	// len(fracPart) = maxDecimalExponentMagnitude-50; combined with the
	// explicit "e-50", the constructed exponent lands exactly at the
	// bound: -50 - (maxDecimalExponentMagnitude-50) = -maxDecimalExponentMagnitude.
	fracZeros := maxDecimalExponentMagnitude - 51
	atBound := "0." + strings.Repeat("0", fracZeros) + "1e-50"
	if _, err := NewDecimal(atBound); err != nil {
		t.Fatalf("stacked frac+exponent at the constructed bound must be legal: %v", err)
	}

	onePast := "0." + strings.Repeat("0", fracZeros+1) + "1e-50"
	if _, err := NewDecimal(onePast); err == nil {
		t.Fatal("stacked frac+exponent one past the constructed bound must be a located error (neither the raw exponent -50 nor fracPart's length alone crosses the bound)")
	}
}

// TestNewDecimalRejectsAbsurdExponentQuickly is D-1.6.6's guarantee
// carried into this consumer: even though Decimal's own bound (100,000)
// is well inside the shared splitter's bound (1,000,000), an absurd
// exponent that would have wrapped the old accumulator is rejected here
// too — through the shared, already-fixed splitter — never a hang.
func TestNewDecimalRejectsAbsurdExponentQuickly(t *testing.T) {
	if _, err := NewDecimal("1e99999999999999999999"); err == nil {
		t.Fatal("expected a located error, not a hang or a silent wrap")
	}
}
