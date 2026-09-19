package pkgb

import "example.invalid/reducer-inventory-fixture/pkga"

// SumDecimals and AvgDecimals are LEFT BEHIND here after a simulated
// move of Decimal to pkga — exactly the hazard D-3.1a.3's relational
// location clause exists to catch: the scan finds both reducers (via
// the qualified reference pkga.Decimal, proving the resolution path
// works) but must report PkgDir "pkgb" while Decimal's own DeclPkgDir
// is "pkga", reddening AC23.
func SumDecimals(items []pkga.Decimal) (pkga.Decimal, error) {
	return pkga.Decimal{}, nil
}

func AvgDecimals(items []pkga.Decimal) (pkga.Decimal, error) {
	return pkga.Decimal{}, nil
}
