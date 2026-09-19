package embedfontfixture

import _ "embed"

// A non-font embed is fine — only font-extensioned paths trip this
// guard.
//
//go:embed data.json
var data []byte
