package expr

import (
	"fmt"
	"math/big"
)

// This file is AC12: formatNumber's own scaling (numberformat.go) uses
// an INTEGER LOOKUP TABLE, never math.Pow (which returns float64,
// AD-23) and never tenPow (reduce.go, which COMPUTES via
// big.Int.Exp — a different instrument, kept separate deliberately so
// the two call sites can never be confused for one another).
//
// tenPowInt64 is a LITERAL, not a loop: writing each entry out by hand
// means Layer 3's own test (tenpow_test.go) can compute an
// INDEPENDENT big.Int chain (10, 100, 1000, … by repeated
// multiplication) and compare it against this literal — a different
// route to the same facts (D-000.38) — rather than checking this
// table against the very computation that would have built it, which
// would let a transcription slip agree with itself.
//
// maxTenPowExponent bounds every shift formatNumber's scaling can
// request (numberformat.go): 18 is the largest shift a Decimal's own
// int64 coefficient (at most 19 significant digits,
// maxDecimalCoefficientDigits, decimal.go) can meaningfully combine
// with while the RESULT is still reasoned about in a bounded table —
// a shift request beyond this bound is a located error, never a
// silently computed (and therefore untested) larger power.
const maxTenPowExponent = 18

var tenPowInt64 = [maxTenPowExponent + 1]int64{
	1, 10, 100, 1000, 10000, 100000, 1000000, 10000000, 100000000,
	1000000000, 10000000000, 100000000000, 1000000000000,
	10000000000000, 100000000000000, 1000000000000000,
	10000000000000000, 100000000000000000, 1000000000000000000,
}

// tenPowLookup returns 10^n as a *big.Int, purely by TABLE LOOKUP
// (AC12) — no exponentiation, no math.Pow, no loop at call time. n
// outside [0, maxTenPowExponent] is a located error, never a computed
// fallback.
func tenPowLookup(n int) (*big.Int, error) {
	if n < 0 || n > maxTenPowExponent {
		return nil, fmt.Errorf(
			"expr: formatNumber: a scale shift of %d exceeds the supported bound %d (AC12's lookup table)",
			n, maxTenPowExponent,
		)
	}
	return big.NewInt(tenPowInt64[n]), nil
}
