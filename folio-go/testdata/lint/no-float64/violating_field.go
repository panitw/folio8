// Package nofloat64fixture is a retained violating fixture for the
// no-float64 rule (D-1.3.3; AC13's sibling table row for this rule).
// It stands permanently in the tree, is never compiled — testdata/ is
// invisible to the go command (F-4) — and is syntactically valid Go
// with forbidden semantics (AC5): a struct field typed float64.
package nofloat64fixture

// violatingField declares a float64 field — forbidden by AD-23 anywhere
// under folio-go/internal/.
type violatingField struct {
	x float64
}
