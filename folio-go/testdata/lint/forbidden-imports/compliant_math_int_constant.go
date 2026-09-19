package forbiddenimportsfixture

import "math"

// compliantMathIntConstant references an integer-limit constant, which
// AC12's value-kind test permits.
var compliantMathIntConstant int64 = math.MaxInt64
