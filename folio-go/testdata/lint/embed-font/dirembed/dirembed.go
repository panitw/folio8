package embedfontfixture

import "embed"

// A bare directory argument (no wildcard) embeds its whole subtree
// (Finding 5, QA review) — the exact shape Story 2.2 ships production
// faces with.
//
//go:embed fonts
var Fonts embed.FS
