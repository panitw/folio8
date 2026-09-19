package forbiddenimportsfixture

import "math"

// violatingMathCall calls a math function outside AD-1's seven
// allow-listed functions (AC12: allow-list, not deny-list).
func violatingMathCall(x float64) float64 {
	return math.Sin(x)
}
