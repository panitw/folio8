package forbiddenimportsfixture

import "time"

// D-1.3.1: one fixture proves only half the rule — this one proves the
// _test.go exemption never covers time, math/rand or net.
func timeStillBannedInTests() time.Duration {
	return time.Second
}
