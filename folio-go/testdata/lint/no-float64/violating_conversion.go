package nofloat64fixture

// violatingConversion performs a bare float64(x) conversion — the shape
// this story's QA review (Blocker 2(b)) measured that a narrower,
// declaration-position-only guard missed entirely.
func violatingConversion(x int) float64 {
	return float64(x)
}
