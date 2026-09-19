package embedfontfixture

import "embed"

// The "all:" prefix (Finding 5, QA review) — includes files starting
// with '.'/'_' too; irrelevant to font detection, but must not defeat
// the directory-argument resolution.
//
//go:embed all:fonts
var Fonts embed.FS
