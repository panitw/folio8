package maprangefixture

import (
	"maps"
	"slices"
)

// compliantEscapeHatch uses AC15's idiom verbatim — it ranges a slice
// (the sorted keys), not a map — and must NOT be reported.
func compliantEscapeHatch(m map[string]int) int {
	total := 0
	for _, k := range slices.Sorted(maps.Keys(m)) {
		v := m[k]
		total += v
	}
	return total
}
