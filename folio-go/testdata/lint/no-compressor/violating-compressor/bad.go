package violating

import "compress/zlib"

// Deliberately invalid semantics fixture (RP-5): a real compress/zlib
// import, used, in a real non-test file — valid syntax, forbidden
// semantics (D-000.13).
func UseZlib() {
	_ = zlib.NewWriter
}
