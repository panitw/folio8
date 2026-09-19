// Package nobigfloattype is the retained fixture tree for the
// no-bigfloat-type rule (Story 3.1a). It is never built as part of any
// real Go module: it lives under testdata/, which the go tool excludes
// from package matching, and it exists only to be pointed at by this
// rule's red-proof.
package nobigfloattype

import "math/big"

// CompliantBigInt uses exactly the big.Int accumulator AD-23's kernel
// blesses (internal/expr's SumDecimals/AvgDecimals, D-3.2.1 — corrected
// Story 3.3 finisher pass, Finding 19: a fifth instance of F12's
// stale-citation class, still naming internal/bind after the move) —
// present so this fixture tree is not vacuously compliant by being
// empty, and so the scan is shown NOT to fire on every math/big
// identifier, only on the two banned ones (Float and Rat).
func CompliantBigInt(a, b int64) *big.Int {
	x := big.NewInt(a)
	y := big.NewInt(b)
	return x.Add(x, y)
}
