package nofloat64fixture

// length stands in for internal/geom.Length so this fixture does not
// need to import the real package to prove the compliant shape: an
// int64-backed value type with no float64 or float32 anywhere.
type length int64

// compliant uses only int64 and the length stand-in — the near-miss that
// must NOT be reported, proving the guard does not over-fire on ordinary
// integer arithmetic.
func compliant(x int64) length {
	return length(x)
}
