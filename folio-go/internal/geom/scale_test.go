package geom

import (
	"math"
	"testing"
)

func TestScaleRound(t *testing.T) {
	cases := []struct {
		name     string
		v        Length
		num, den int64
		want     Length
	}{
		// Half-to-even vs. half-away-from-zero disagree on these four —
		// this is what RP-3 (round-half-away-from-zero mutation) reddens.
		{"half rounds down to even (2)", 5, 1, 2, 2},
		{"half rounds down to even (-2)", -5, 1, 2, -2},
		{"half rounds up to even (8)", 15, 1, 2, 8},
		{"half rounds up to even (-8)", -15, 1, 2, -8},

		// Exact cases (no rounding decision to make).
		{"exact positive", 10, 1, 2, 5},
		{"exact negative", -10, 1, 2, -5},
		{"exact zero", 0, 1, 2, 0},
		{"identity", 841890, 1, 1, 841890},

		// Non-half fractional cases (ordinary rounding, not a tie).
		{"rounds down, not a tie", 7, 1, 4, 2},     // 1.75 -> 2, sanity check below
		{"rounds nearest below half", 9, 1, 4, 2},  // 2.25 -> 2
		{"rounds nearest above half", 11, 1, 4, 3}, // 2.75 -> 3

		// Another even/odd half-tie pair away from zero, to further pin
		// the rounding mode down (3.5 -> 4 even; 4.5 -> 4 even).
		{"half rounds up to even (4)", 7, 1, 2, 4},
		{"half rounds down to even (4)", 9, 1, 2, 4},

		// Boundary rows (this story's QA review, Blocker 3): the earlier
		// implementation negated math.MinInt64 directly, which overflows
		// silently in two's complement (-math.MinInt64 == math.MinInt64)
		// and returned the wrong sign. num=1, den=2 exercises the
		// negation path at both ends of int64's asymmetric range, plus
		// -1, 999, 1000 and -1000 as the story's review specifically
		// asked for.
		{"MinInt64 boundary", Length(math.MinInt64), 1, 2, Length(-4611686018427387904)},
		{"MaxInt64 boundary", Length(math.MaxInt64), 1, 2, Length(4611686018427387904)},
		{"-1 boundary", -1, 1, 2, 0},
		{"999 boundary", 999, 1, 2, 500},
		{"1000 boundary", 1000, 1, 2, 500},
		{"-1000 boundary", -1000, 1, 2, -500},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ScaleRound(c.v, c.num, c.den)
			if got != c.want {
				t.Errorf("ScaleRound(%d, %d, %d) = %d, want %d", c.v, c.num, c.den, got, c.want)
			}
		})
	}
}

func TestScaleRoundPanicsOnZeroDenominator(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on den == 0, got none")
		}
	}()
	ScaleRound(1, 1, 0)
}

// TestScaleRoundPanicsOnMinInt64Denominator is the review's second
// measured Finding-6 case: ScaleRound(1000, 1, math.MinInt64) used to
// return -1 (exact answer 0) because negating den == math.MinInt64
// overflowed silently. It now panics instead of returning a wrong
// answer, consistent with this function's den == 0 panic.
func TestScaleRoundPanicsOnMinInt64Denominator(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on den == math.MinInt64, got none")
		}
	}()
	ScaleRound(1000, 1, math.MinInt64)
}

// TestScaleRoundPanicsOnMultiplicationOverflow is the review's first
// measured Finding-6 case: ScaleRound(math.MaxInt64/2, 4, 2) used to
// return -2 (silently wrapped) because v*num overflowed int64
// undetected. It now panics instead.
func TestScaleRoundPanicsOnMultiplicationOverflow(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on v*num overflow, got none")
		}
	}()
	ScaleRound(Length(math.MaxInt64/2), 4, 2)
}

// TestScaleRoundPanicsOnSecondMultiplicationOverflowCase mirrors the
// review's second overflow example directly.
func TestScaleRoundPanicsOnSecondMultiplicationOverflowCase(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on v*num overflow, got none")
		}
	}()
	ScaleRound(1000000, math.MaxInt64/1000, 1)
}

// TestScaleRoundPanicsOnDivisionOverflow is the den == -1,
// v*num == math.MinInt64 edge case: the true answer (+2^63) has no int64
// representation at all.
func TestScaleRoundPanicsOnDivisionOverflow(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on v*num/den overflow, got none")
		}
	}()
	ScaleRound(Length(math.MinInt64), 1, -1)
}
