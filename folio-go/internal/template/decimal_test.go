package template

import (
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// TestDecodePointsNormalisesNonCanonicalInput is AC27: "36.0000" -> 36000
// millipoints, "1e3" -> 1000000 millipoints (1000 points); legality is a
// property of the value, not the spelling.
func TestDecodePointsNormalisesNonCanonicalInput(t *testing.T) {
	cases := []struct {
		literal string
		want    int64 // millipoints
	}{
		{"36", 36000},
		{"36.0000", 36000},
		{"1e3", 1000000},
		{"1E3", 1000000},
		{"72.5", 72500},
		{"-5.5", -5500},
		{"0", 0},
	}
	for _, c := range cases {
		got, err := decodePoints(c.literal)
		if err != nil {
			t.Errorf("decodePoints(%q): unexpected error: %v", c.literal, err)
			continue
		}
		if int64(got) != c.want {
			t.Errorf("decodePoints(%q) = %d, want %d", c.literal, int64(got), c.want)
		}
	}
}

// TestDecodePointsRejectsExtraDecimals is AC28: a value that is not an
// exact whole number of millipoints is a load error — nothing is
// rounded. Both polarities: 36.0000 legal, 72.5001 a load error.
func TestDecodePointsRejectsExtraDecimals(t *testing.T) {
	if _, err := decodePoints("36.0000"); err != nil {
		t.Errorf("36.0000 must be legal (exactly 36000 millipoints): %v", err)
	}
	if _, err := decodePoints("72.5001"); err == nil {
		t.Error("72.5001 must be a load error (more than three decimal places)")
	}
}

// TestDecodePointsOverflow is AC27: int64 millipoint overflow is a load
// error, never a wrap.
func TestDecodePointsOverflow(t *testing.T) {
	if _, err := decodePoints("99999999999999999999999999"); err == nil {
		t.Error("expected an overflow load error")
	}
}

// TestParseDecimalExponentDoesNotHang is D-1.6.6/AC4/AC4a: the shipped
// exponent accumulator wrapped silently on an absurd exponent, and the
// wrapped value reached big.Int.Exp's repeated squaring in decodePoints,
// which HUNG the process. Measured pre-fix (this story's Dev Notes, M-1,
// and reconfirmed by temporarily restoring the baseline decimal.go and
// running this exact literal under `go test -timeout=15s`): "hung;
// killed by -timeout at 15.436s", with ~74 math/big.nat.sqr/karatsubaSqr
// frames rooted at decodePoints (decimal.go:48) — reproducing M-1's
// trace exactly. Post-fix, every case below returns an error in well
// under a second (measured: ~0.5s total for all four).
func TestParseDecimalExponentDoesNotHang(t *testing.T) {
	cases := []string{
		"1e99999999999999999999",  // exponent accumulator overflow (Fix 1)
		"1e9223372036854775808",   // one past int64 max: also Fix 1
		"1e-99999999999999999999", // negative polarity, same overflow
	}
	for _, c := range cases {
		_, err := decodePoints(c)
		if err == nil {
			t.Errorf("decodePoints(%q): expected a located error, got nil (this must never hang or wrap)", c)
		}
	}
}

// TestSplitJSONNumberExponentBound is AC4a-ii and AC4b: a documented
// magnitude bound, rejected BEFORE any big.Int scaling is attempted.
// Tested at the bound and one past it, on both polarities (binding,
// AC4b; negative side added per QA Finding 9, this story's review,
// Minor — the bound is applied before `if neg { n = -n }`, so the
// symmetry was real but previously unpinned) — the specific number
// (MaxSplitExponentMagnitude) is illustrative.
//
// QA Finding 7 (this story's review, Minor): MaxSplitExponentMagnitude
// was previously 1,000,000, ten times the actual wider consumer bound
// (100,000); this test's literals now match the corrected constant.
func TestSplitJSONNumberExponentBound(t *testing.T) {
	if _, err := parseDecimalExponent("100000"); err != nil {
		t.Errorf("exponent magnitude at the bound (%d) must be legal: %v", MaxSplitExponentMagnitude, err)
	}
	if _, err := parseDecimalExponent("100001"); err == nil {
		t.Errorf("exponent magnitude one past the bound (%d) must be a located error", MaxSplitExponentMagnitude+1)
	}
	if _, err := parseDecimalExponent("-100000"); err != nil {
		t.Errorf("exponent magnitude at the bound, negative side (-%d) must be legal: %v", MaxSplitExponentMagnitude, err)
	}
	if _, err := parseDecimalExponent("-100001"); err == nil {
		t.Errorf("exponent magnitude one past the bound, negative side (-%d) must be a located error", MaxSplitExponentMagnitude+1)
	}
	// 1e2000000 was Story 1.4's own control (M-2): geom.Length's
	// millipoint overflow check already caught it correctly in 0.07s.
	// It now fails earlier still — at the shared splitter, before any
	// big.Int scaling — because its exponent magnitude (2,000,000)
	// exceeds MaxSplitExponentMagnitude (100,000, corrected by Finding
	// 7). Still an error either way; this asserts it stays an error
	// post-fix. (Nit 14, this story's review: this specific assertion
	// cannot itself fail from removing the magnitude-bound fix alone —
	// decodePoints' own millipoint overflow check already caught
	// 1e2000000 before this story existed — so it is retained
	// deliberately as a regression control for Story 1.4's own case
	// (M-2), not as coverage of this bound; the two assertions above
	// carry that.)
	if _, err := decodePoints("1e2000000"); err == nil {
		t.Error("decodePoints(\"1e2000000\") must still be a located error")
	}
}

// TestAppendPointsSpelling is AC25's on-disk spelling: trailing zeros
// trimmed, no trailing '.', no '-0', no exponent.
func TestAppendPointsSpelling(t *testing.T) {
	cases := []struct {
		millipoints int64
		want        string
	}{
		{36000, "36"},
		{72500, "72.5"},
		{0, "0"},
		{-36000, "-36"},
		{1, "0.001"},
		{-1, "-0.001"},
	}
	for _, c := range cases {
		got := string(appendPoints(nil, geom.Length(c.millipoints)))
		if got != c.want {
			t.Errorf("appendPoints(%d) = %q, want %q", c.millipoints, got, c.want)
		}
	}
}
