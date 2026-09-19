package folio8

import (
	"slices"

	"github.com/panitw/folio8/folio-go/internal/expr"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// SerializeTemplate returns the engine's canonical .folio bytes for t.
//
// The Template remains opaque: callers receive bytes, not the internal
// document or a field-by-field representation of it. This is the only save
// seam used by the browser worker (AD-15/AD-16).
func SerializeTemplate(t *Template) ([]byte, error) {
	if t == nil {
		return nil, errNilTemplate
	}
	return template.SerializeDocumentWithMinimumVersion(t.doc, expressionMinimumVersion(t.doc))
}

// Parse the actual expressions, so quoted operator punctuation and ordinary
// paths never accidentally raise the saved format requirement.
//
// Two expression-derived requirements exist, highest first:
//
//   - 3.3 (template.TextNumberExpressionVersion): a text expression — a text
//     element's value or a table column's bind — whose static kinds include
//     number ({{1}}, {{count(items)}}, {{a + b}}). A number in text prints as
//     its exact decimal from 3.3 on. A plain path whose kind depends on data
//     ({{row.amount}}) cannot be detected and raises nothing (disclosed in
//     folio-format.md).
//   - 2.0: formula syntax or boolean/null literals in any expression container.
func expressionMinimumVersion(doc *template.Document) string {
	formula := func(raw string) bool { e, err := expr.Parse(raw); return err == nil && expr.UsesFormulas(e) }
	number := func(raw string) bool {
		e, err := expr.Parse(raw)
		return err == nil && slices.Contains(expr.KnownKinds(e), expr.KindNumber)
	}
	textPlaceholders := func(raw string, test func(string) bool) bool {
		_, parts, _, err := expr.ScanPlaceholders(raw)
		if err != nil {
			return false
		}
		for _, part := range parts {
			if !part.Reserved && test(part.Inner) {
				return true
			}
		}
		return false
	}
	usesFormula := false
	for _, band := range doc.ElementBands() {
		for _, el := range band.Elements {
			if el.VisibleIf.Set && !el.VisibleIf.Null && formula(el.VisibleIf.Value) {
				usesFormula = true
			}
			if el.Value.Set && !el.Value.Null {
				if textPlaceholders(el.Value.Value, number) {
					return template.TextNumberExpressionVersion
				}
				if textPlaceholders(el.Value.Value, formula) {
					usesFormula = true
				}
			}
			if el.Table.Set && !el.Table.Null {
				for _, col := range el.Table.Value.Columns {
					if textPlaceholders(col.Bind, number) {
						return template.TextNumberExpressionVersion
					}
					if textPlaceholders(col.Bind, formula) {
						usesFormula = true
					}
				}
			}
		}
	}
	if usesFormula {
		return "2.0"
	}
	return ""
}
