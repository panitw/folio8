package nobigfloattype

import "math/big"

// Money is a package-local alias for the banned type math/big.Float —
// QA review Finding 1 (Blocker): a declared alias such as this one
// materialises in go/types as *types.Alias since Go 1.23
// (gotypesalias=1 is this toolchain's default), which is NOT
// *types.Named. Before resolveNamedType applied types.Unalias, a value
// of this type resolved to ok=false and the scan reported it clean —
// ZERO findings with a NON-ZERO TypedExprs witness, the exact failure
// shape AD-23's guards exist to eliminate. This fixture proves the
// alias case from inside the scanned tree; the reviewer's own
// reproduction (an alias declared in a separate DEPENDENCY package,
// reached only via a qualified reference such as
// "func Balance() bfdep.Money") is recorded as a live, independently
// re-run measurement in the story's Delivery Log rather than shipped
// here, matching how AC10's forbidden float64 mutant is recorded as a
// measurement rather than committed code.
type Money = big.Float

// AliasedFloat uses ONLY the alias name — the literal text "big.Float"
// appears nowhere in this function's body — exercising AC14's "an
// alias ... resolve[s] the same and trip[s] it" clause with a real
// fixture rather than an unexercised claim.
func AliasedFloat() Money {
	var m Money
	return m
}
