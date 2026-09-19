package forbiddenimportsfixture

import "math"

// compliantMathAbs calls one of AD-1's seven allow-listed math
// functions and must NOT be reported.
func compliantMathAbs(x float64) float64 {
	return math.Abs(x)
}
