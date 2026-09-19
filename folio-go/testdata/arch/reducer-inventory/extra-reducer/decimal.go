// Package reducerinventoryfixture, extra-reducer variant: AC24's FIRST
// red-proof direction. Identical to the baseline fixture's Decimal type;
// see baseline/decimal.go for why the fields are irrelevant here.
package reducerinventoryfixture

type Decimal struct {
	Coefficient int64
	Exponent    int
}
