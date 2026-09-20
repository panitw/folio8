//go:build !nocjkface

package fontset

// DeclaredLineMetrics answers nothing in the default build, and that is
// the whole contract: every build but the designer's engine wasm embeds
// all eleven faces, so a chain member it was not GIVEN is a caller's
// genuine partial FontSet and keeps the long-standing tolerance —
// chainLineMetrics skips it, exactly as it always has.
//
// ⚠ THIS IS WHAT KEEPS folio-js, folio-dotnet, THE CLI AND EVERY GOLDEN
// FIXTURE BYTE-FOR-BYTE UNCHANGED. A metrics table that answered
// unconditionally would change the layout a partial-set caller gets
// today, which is shipped behaviour with its own tests
// (face_absent_refusal_test.go's matrix row 1 and its neighbours).
func DeclaredLineMetrics(string) (LineMetrics, bool) { return LineMetrics{}, false }
