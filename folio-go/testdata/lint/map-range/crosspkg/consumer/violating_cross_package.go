// Package consumer ranges over map types and a map-returning function
// imported from a sibling package (producer) — the cross-package shape
// Blocker 1 (D-1.3.11) proved the withdrawn dependency-free detector
// could not see: producer.ScaleTable's *ast.RangeStmt subject resolved,
// via the tolerant importer, to an empty types.Package with no
// Underlying() information, so it fell through "type did not resolve"
// silently rather than being reported. Both functions below must be
// reported by ScanMapRange now that it resolves the whole subtree as one
// coherent package graph (golang.org/x/tools/go/packages).
package consumer

import "github.com/panitw/folio8/folio-go/testdata/lint/map-range/crosspkg/producer"

// rangeOverNamedMapType ranges over a named map type declared in
// another package, taken as a parameter.
func rangeOverNamedMapType(t producer.ScaleTable) int {
	total := 0
	for _, v := range t {
		total += v
	}
	return total
}

// rangeOverMapReturningFunction ranges directly over another package's
// map-returning function call.
func rangeOverMapReturningFunction() int {
	total := 0
	for _, v := range producer.NewScaleTable() {
		total += v
	}
	return total
}
