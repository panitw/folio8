package embedfontfixture

import "embed"

// A glob argument (Finding 5, QA review).
//
//go:embed fonts/*
var Fonts embed.FS
