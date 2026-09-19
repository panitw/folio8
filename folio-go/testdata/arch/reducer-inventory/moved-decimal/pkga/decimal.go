// Package pkga is the "Decimal moved here" side of the moved-decimal
// fixture (QA review Finding 5): D-3.1a.3's second real failure mode —
// Decimal declared in one package, with the reducers left behind in a
// DIFFERENT one (pkgb) — reached only through a qualified reference.
package pkga

// Decimal is a stand-in for bind.Decimal, matching the other
// reducer-inventory fixtures — the real fields play no part in a
// purely structural (AST-only, no type-checking) inventory scan.
type Decimal struct {
	Coefficient int64
	Exponent    int
}
