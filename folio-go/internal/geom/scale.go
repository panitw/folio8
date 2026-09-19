package geom

// minInt64 is math.MinInt64, spelled as a literal so this file needs no
// import (F-3: internal/geom's non-test files import nothing).
const minInt64 = -1 << 63

// int64MulOverflows reports whether a*b cannot be represented as an
// int64. It uses the standard "multiply then divide back" check, with one
// explicit exception: math.MinInt64 * -1 wraps back to math.MinInt64
// itself in two's-complement arithmetic, which makes the divide-back
// check falsely agree that nothing overflowed, so that pair is rejected
// directly.
func int64MulOverflows(a, b int64) bool {
	if a == 0 || b == 0 {
		return false
	}
	if (a == minInt64 && b == -1) || (b == minInt64 && a == -1) {
		return true
	}
	p := a * b
	return p/b != a
}

// ScaleRound computes v*num/den, rounding the exact result to the nearest
// integer using round-half-to-even (banker's rounding), and returns it as
// a Length.
//
// The computation is performed entirely on int64: v*num and the remainder
// against den are exact integer values (guarded against overflow below),
// so "exact integer quotient" is literal — there is no floating-point
// intermediate anywhere in this function, and no call to math.Round. This
// is the module's one and only scaling function (AD-1's "no open-coded
// rounding"); every place that needs to scale a Length by a ratio calls
// this instead of writing its own arithmetic.
//
// ScaleRound panics if den is zero, if den is math.MinInt64 (whose
// magnitude has no positive int64 representation), if the exact product
// v*num overflows int64, or if the exact quotient v*num/den overflows
// int64 (only reachable when den == -1 and v*num == math.MinInt64). Each
// is a programmer error, not a runtime condition callers are expected to
// recover from — consistent with this function's pre-existing den == 0
// panic.
//
// Earlier versions of this function negated the numerator (or the final
// quotient) directly to compute an absolute value. That overflows
// silently when the value being negated is math.MinInt64, since int64's
// negative range is one wider than its positive range — the exact trap
// this story's Dev Notes warned about, and the cause of a real, measured
// defect (this story's QA review, Blocker 3): ScaleRound(math.MinInt64,
// 1, 2) returned +4611686018427387904 instead of the correct
// -4611686018427387904. This version never negates a value that could be
// math.MinInt64: it lets Go's / and % (which truncate toward zero and
// already carry the correct sign) produce the quotient and remainder
// directly, and only ever negates the remainder — which, by the
// definition of remainder, always has strictly smaller magnitude than
// den and so can never itself be math.MinInt64 once den == math.MinInt64
// has already been rejected above.
func ScaleRound(v Length, num, den int64) Length {
	if den == 0 {
		panic("geom: ScaleRound: den == 0")
	}
	if den == minInt64 {
		panic("geom: ScaleRound: den == math.MinInt64, whose magnitude is not representable as a positive int64")
	}

	vn := int64(v)
	if int64MulOverflows(vn, num) {
		panic("geom: ScaleRound: v*num overflows int64")
	}
	n := vn * num

	if den == -1 && n == minInt64 {
		panic("geom: ScaleRound: v*num/den overflows int64")
	}

	// Go's / and % truncate toward zero, so q already carries the sign
	// of the exact mathematical quotient (or is zero) and r the sign of
	// the exact remainder (or is zero) — there is no separate sign
	// bookkeeping to do, and no need to ever negate n or q wholesale.
	q := n / den
	r := n % den

	if r != 0 {
		absR := r
		if absR < 0 {
			// Safe: |r| < |den| by definition of remainder, and den is
			// never math.MinInt64 here (rejected above), so r is never
			// math.MinInt64 either.
			absR = -absR
		}
		absDen := den
		if absDen < 0 {
			absDen = -absDen // safe: den != math.MinInt64, rejected above
		}

		// Round half to even, comparing 2|r| against |den| without ever
		// computing 2|r| (which could itself overflow when |r| is large):
		// 2|r| > |den| is equivalent to |r| > |den|-|r|, and the tie case
		// (2|r| == |den|) is equivalent to |r| == |den|-|r|. The
		// subtraction cannot underflow because 0 < |r| < |den| <= maxInt64
		// (den != math.MinInt64, rejected above).
		half := absDen - absR
		if absR > half || (absR == half && q%2 != 0) {
			if q >= 0 {
				q++
			} else {
				q--
			}
			// This branch can only be taken when r != 0. q == math.MinInt64
			// is only reachable via truncating division when den == 1 (or
			// -1) and n == math.MinInt64 exactly, which leaves r == 0 — so
			// q is never math.MinInt64 (nor math.MaxInt64, symmetrically)
			// at the point q++ / q-- runs, and this can never cross an
			// int64 boundary.
		}
	}

	return Length(q)
}
