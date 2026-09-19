// Package geom declares the module's single geometric scalar type.
//
// internal/geom is the ONLY package that may declare a geometric scalar
// type (AD-2). Its non-test files import nothing — not even the standard
// library — so that no geometric value can ever pick up float formatting,
// wall-clock, randomness or any other non-deterministic behaviour through a
// transitive import. If a 128-bit intermediate is ever needed for scaling,
// that is a later story's problem and a later decision.
package geom

// Length is a measurement expressed in millipoints: one thousandth of a PDF
// point, where one point is 1/72 inch. Every position, advance and
// dimension anywhere in this module is a Length. Storing measurements as an
// integer number of millipoints instead of a floating-point number of
// points is what makes the module's output byte-reproducible (AD-2): a
// millipoint value has an exact, unambiguous decimal text form, and integer
// arithmetic on it never depends on the host's floating-point
// implementation.
type Length int64

// Rect is an axis-aligned rectangle expressed in millipoints.
type Rect struct {
	X, Y, W, H Length
}
