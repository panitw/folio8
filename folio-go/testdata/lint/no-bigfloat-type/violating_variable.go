package nobigfloattype

import "math/big"

// VariableOfBannedType is the AC18 "variable of the type" case: x is
// declared with big.Rat — exercising the OTHER banned type, so both
// Float and Rat are proven reachable across this fixture, not just one.
// big.Rat is banned for a DIFFERENT reason from big.Float (D-3.1a.1): it
// is exact, so Layer 1's behavioural oracle cannot catch it — it is
// wrong because it dodges AD-23's defined division scale and
// round-half-to-even rule, not because it is imprecise.
func VariableOfBannedType() {
	var x big.Rat
	_ = x
}
