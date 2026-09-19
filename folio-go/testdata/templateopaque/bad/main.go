// Command bad is the red fixture for
// TestTemplateCompositeLiteralDoesNotTypeCheck (folio-go/templateopaque_test.go,
// Epic 1 boundary gate finding): it attempts to construct a
// folio8.Template by composite literal, field by field, from outside
// package folio8 — exactly the side door the alias
// (`type Template = template.Document`) used to leave open, letting a
// caller bypass ParseTemplate's asset-key validation, version rules,
// closed sets, exact-decimal discipline and nextId monotonicity. This
// must NOT type-check: folio8.Template exports no fields, so
// "Version" (a field that only ever existed on the aliased
// internal/template.Document) is unknown to the compiler here.
package main

import "github.com/panitw/folio8/folio-go"

func main() {
	tpl := folio8.Template{Version: "1.0"}
	_ = tpl
}
