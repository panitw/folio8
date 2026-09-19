// Package main is AC19b's residual-gap fixture (illustrative,
// D-1.7.5): an EXPLICIT conversion between the two defined types still
// compiles. The guardrail proves the ACCIDENTAL swap is a compile
// error; it does not and cannot stop a deliberate cast — this fixture
// exists so that fact is measured, not merely asserted in prose.
package main

import "github.com/panitw/folio8/folio-go"

func main() {
	var t *folio8.Template
	var d folio8.Data
	var p folio8.Params
	var f folio8.FontSet
	_, _ = folio8.Render(t, folio8.Data(p), folio8.Params(d), f)
}
