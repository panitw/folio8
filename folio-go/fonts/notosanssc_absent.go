//go:build nocjkface

// THE TRIMMED HALF OF THE PAIR (spec-deferred-offline-cache, CAP-6).
//
// Under `-tags nocjkface` this file replaces notosanssc.go, the go:embed
// directive for notosanssc/NotoSansSC-Regular.ttf is never compiled, and
// the face's 10,595,932 raw bytes — 4.72 MiB of the designer's 8.11 MiB
// Brotli engine sidecar — are absent from the binary rather than merely
// unreferenced by it. A go:embed var is package-scope, so no linker
// dead-code elimination can remove it and no filter applied at the
// Shipped() call site can either: the only way not to carry the bytes is
// not to compile the directive, which is what this pair buys.
//
// ONE BUILD SETS THIS TAG AND ONE ONLY: the designer's engine wasm, via
// ENGINE_BUILD_FLAGS in folio-designer/scripts/wasm-vcs-stamp.mjs. That
// build receives the face from JavaScript instead — fetched once per
// session from the deferred release asset that already carries the
// identical bytes, and installed through the wasm host's installFace
// entry point into the FontSet internal/wasm holds.
//
// NOTHING IS REMOVED FROM THE SHIPPED CONTRACT BY THIS FILE'S EXISTENCE.
// notosanssc/ keeps its binary, its licence and its NOTICE; the untagged
// Shipped() keeps all eleven keys; folio-js, folio-dotnet, the six CJK
// golden fixtures and the eleven-face documentation are untouched.

package fonts

// buildTaggedFaces adds nothing under this tag. It is a nil map rather
// than an empty literal because Shipped()'s merge is a range, and a
// range over nil is the idiomatic no-op.
func buildTaggedFaces() map[string][]byte { return nil }
