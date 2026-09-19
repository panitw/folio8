package nofloattypedvalue

import "github.com/boxesandglue/textshape/ot"

// IntegerVendorMetrics reads integer-typed vendor accessors only. It is
// here so the rule is shown NOT to fire on every vendor call — a rule
// that reported every call into the dependency would be a denylist on
// the dependency, not a guard on a type.
func IntegerVendorMetrics(f *ot.Face) (int64, int64) {
	return int64(f.Ascender()), int64(f.Descender())
}

// IntegerArithmetic is ordinary integer code sharing the file's package
// so the compliant half is not vacuously compliant by being empty.
func IntegerArithmetic(a, b int64) int64 {
	return a*1000 + b
}
