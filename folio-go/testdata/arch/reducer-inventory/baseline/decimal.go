// Package reducerinventoryfixture is a retained AST-only fixture for
// the reducer inventory's red-proof (AC23/AC24, D-3.1a.3, D-000.30). It
// is never built as part of folio-go: it lives under testdata/, which
// the go tool excludes from package matching, and it stands in for
// internal/bind's real shape (a Decimal type plus the two reducer
// functions) without depending on it, so this fixture never needs
// editing when Story 3.2 moves the real Decimal to internal/expr.
package reducerinventoryfixture

// Decimal is a stand-in for bind.Decimal — the real fields play no part
// in a purely structural (AST-only, no type-checking) inventory scan.
type Decimal struct {
	Coefficient int64
	Exponent    int
}
