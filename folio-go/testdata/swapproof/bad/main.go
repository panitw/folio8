// Package main is AC19's negative fixture (D-1.7.5): Data and Params
// are DEFINED types with the same underlying type ([]byte), so
// swapping them accidentally at a Render call site must be a compile
// error, never a runtime one. This file is built directly (never via
// "./..." — testdata/ is excluded from that pattern) by
// TestParamsDataSwapDoesNotTypeCheck in folio-go/paramsswap_test.go,
// which asserts it fails to build and names the assignability.
package main

import "github.com/panitw/folio8/folio-go"

func main() {
	var t *folio8.Template
	var d folio8.Data
	var p folio8.Params
	var f folio8.FontSet
	// Swapped: p where Data belongs, d where Params belongs.
	_, _ = folio8.Render(t, p, d, f)
}
