package consumer

import "github.com/panitw/folio8/folio-go/testdata/lint/map-range/crosspkg/producer"

// rangeOverAliasedMapType ranges over a type alias to a map type,
// imported from another package — the third of the three shapes the QA
// review measured the withdrawn detector missed (Blocker 1, D-1.3.11).
func rangeOverAliasedMapType(t producer.Alias) int {
	total := 0
	for k := range t {
		total += len(k)
	}
	return total
}

// rangeOverAliasedMapReturningFunction ranges directly over another
// package's alias-typed map-returning function call.
func rangeOverAliasedMapReturningFunction() int {
	total := 0
	for k := range producer.NewAlias() {
		total += len(k)
	}
	return total
}
