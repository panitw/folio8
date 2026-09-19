package embedfontfixture

import _ "embed"

// A quoted argument containing a space (Finding 5, QA review) — legal
// Go; a naive strings.Fields split breaks it into two tokens on the
// internal space.
//
//go:embed "shipped face.ttf"
var ShippedFace []byte
