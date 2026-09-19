// Package maprangefixture is a retained fixture tree for the map-range
// rule (D-1.3.5, AC16). Never compiled — testdata/ is invisible to the
// go command — and syntactically valid Go with forbidden semantics
// (AC5).
package maprangefixture

// violatingMapRange ranges a map value in a non-test file — forbidden
// anywhere under folio-go/internal/ (D-1.3.5, NFR1.d).
func violatingMapRange(m map[string]int) int {
	total := 0
	for _, v := range m {
		total += v
	}
	return total
}
