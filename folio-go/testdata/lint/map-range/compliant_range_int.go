package maprangefixture

// compliantRangeInt uses Go's range-over-int form (available at the
// go 1.25.0 language floor).
func compliantRangeInt(n int) int {
	total := 0
	for i := range n {
		total += i
	}
	return total
}
