// Package nofloattypedvalue is the retained fixture tree for the
// no-float-typed-value rule (Story 2.3a, AC3). It is never built as part
// of folio-go: it lives under testdata/, which the go tool excludes from
// package matching, and it exists only to be pointed at by the two
// halves of this rule's red-proof.
package nofloattypedvalue

import "github.com/boxesandglue/textshape/ot"

// TruncatedVendorAdvance is the violating site, and the whole point of
// this file is what it does NOT contain: no banned type identifier
// anywhere, not even in a comment, and no fractional literal. The
// vendor accessor's return type is inferred, so the conversion below
// reads as pure integer code to a scanner that matches on spelling.
//
// A type-aware scan reports it. The syntactic scanner in
// folio-go/internal/arch_test.go reports zero on this file, and that
// contrast is asserted in both modules rather than described in prose.
func TruncatedVendorAdvance(f *ot.Face, gid ot.GlyphID) int64 {
	return int64(f.HorizontalAdvance(gid))
}
