package maprangefixture

func compliantRangeSlice(s []int) int {
	total := 0
	for _, v := range s {
		total += v
	}
	return total
}
