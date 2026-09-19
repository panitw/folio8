package forbiddenimportsfixture

import "runtime"

// violatingRuntimeCaller calls runtime.Caller, which Story 2.1's
// RuleRuntimeCaller forbids anywhere under folio-go/internal/ (AC2,
// V1): a render-path package must never inspect its own call stack.
func violatingRuntimeCaller() {
	_, _, _, _ = runtime.Caller(0)
}
