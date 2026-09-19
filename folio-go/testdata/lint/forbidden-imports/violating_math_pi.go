package forbiddenimportsfixture

import "math"

// violatingMathPi references the float-valued constant math.Pi, which
// AC12's value-kind test forbids even though it is not a call.
var violatingMathPi = math.Pi
