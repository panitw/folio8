package reducerinventoryfixture

// SumDecimals and AvgDecimals are the sanctioned pair, unchanged from
// the baseline fixture.
func SumDecimals(items []Decimal) (Decimal, error) {
	return Decimal{}, nil
}

func AvgDecimals(items []Decimal) (Decimal, error) {
	return Decimal{}, nil
}

// ProductDecimals is AC24's FIRST red-proof direction, made real: a
// THIRD function reducing a sequence of Decimal to (Decimal, error),
// added anywhere in the module. The inventory must see it and report a
// set that is no longer exactly {SumDecimals, AvgDecimals}.
func ProductDecimals(items []Decimal) (Decimal, error) {
	return Decimal{}, nil
}
