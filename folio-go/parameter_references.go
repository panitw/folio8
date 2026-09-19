package folio8

import (
	"fmt"
	"maps"
	"slices"

	"github.com/panitw/folio8/folio-go/internal/expr"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// MaxParameterReferenceNameLength is the transport and UI contract for one
// direct params.<name> segment. Keep this in step with the closed browser
// protocol bound: expression identifiers are ASCII, so bytes and characters
// are the same unit here.
const MaxParameterReferenceNameLength = 128

// ParameterReferences returns the bounded, display-only names requested by a
// canonical template. It deliberately reuses the expression parser that
// ParseTemplate already admits; callers never receive a document or expression
// model, and data/row/table paths cannot enter this projection.
func ParameterReferences(tpl *Template) ([]string, error) {
	if tpl == nil || tpl.doc == nil {
		return nil, fmt.Errorf("folio8: parameter references require a template")
	}
	refs := make(map[string]struct{})
	collect := func(raw string, elementID template.ElementID, field string) error {
		e, err := expr.Parse(raw)
		if err != nil {
			return fmt.Errorf("folio8: parameter references: element %s %s: %w", elementID, field, err)
		}
		return collectParameterPaths(e, refs, elementID, field)
	}
	for _, band := range tpl.doc.ElementBands() {
		for _, element := range band.Elements {
			// visibleIf belongs to every element, including tables. The table's
			// own columns/footer bindings remain out of this projection, but a
			// table visibility condition is evaluated in the document scope and
			// can directly request a runtime parameter.
			if element.VisibleIf.Set && !element.VisibleIf.Null {
				if err := collect(element.VisibleIf.Value, element.ID, "visibleIf"); err != nil {
					return nil, err
				}
			}
			if (element.Type != template.ElementText && !isCodeElement(element.Type)) || !element.Value.Set || element.Value.Null {
				continue
			}
			_, placeholders, _, err := expr.ScanPlaceholders(element.Value.Value)
			if err != nil {
				return nil, err
			}
			for _, placeholder := range placeholders {
				if !placeholder.Reserved {
					if err := collect(placeholder.Inner, element.ID, "value"); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	if len(refs) > 128 {
		return nil, fmt.Errorf("folio8: template references more than 128 parameters")
	}
	return slices.Sorted(maps.Keys(refs)), nil
}

func collectParameterPaths(value expr.Expr, refs map[string]struct{}, elementID template.ElementID, field string) error {
	switch expression := value.(type) {
	case *expr.PathExpr:
		// The editor owns the top-level member document. A nested production
		// path such as params.statement.reportDate therefore projects
		// "statement", allowing its complete JSON object to remain raw input.
		if len(expression.Segments) >= 2 && expression.Segments[0] == "params" {
			if len(expression.Segments[1]) > MaxParameterReferenceNameLength {
				return fmt.Errorf("folio8: parameter references: element %s %s: params.%s exceeds the %d-character name limit", elementID, field, expression.Segments[1], MaxParameterReferenceNameLength)
			}
			refs[expression.Segments[1]] = struct{}{}
		}
	}
	for _, child := range expr.Children(value) {
		if err := collectParameterPaths(child, refs, elementID, field); err != nil {
			return err
		}
	}
	return nil
}
