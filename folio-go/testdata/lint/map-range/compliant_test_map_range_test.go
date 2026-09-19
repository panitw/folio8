package maprangefixture

import "testing"

// compliantTestMapRange ranges a map in a _test.go file — the ban is on
// non-test files only (D-1.3.5), mirroring internal/arch_test.go's own
// map[string]bool, which stays legal.
func compliantTestMapRange(t *testing.T) {
	t.Helper()
	m := map[string]bool{"a": true}
	for k := range m {
		_ = k
	}
}
