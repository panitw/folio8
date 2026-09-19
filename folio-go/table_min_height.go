package folio8

import (
	"fmt"

	"github.com/panitw/folio8/folio-go/internal/geom"

	"github.com/panitw/folio8/folio-go/internal/layout"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// validateTableMinHeights refuses, AT LOAD, a table whose `minHeight` is
// taller than the content window it would have to be placed in
// (SPEC-table-rules §3).
//
// WHY LOAD AND NOT RENDER. The box's height is what a continuation page
// must accommodate, and a floor taller than the window makes the table
// unplaceable on EVERY page of the document, with no data able to change
// that answer: the page size, the margins and the two band heights are
// all declared, and so is the floor. A condition decidable from the
// document alone is answered while the author is still holding the file,
// not once per render.
//
// WHY ParseTemplate AND NOT internal/template. The comparison needs
// layout.ContentHeight over a layout.PageGeometry, and internal/template
// (stage rank 2) may not import internal/layout — the same constraint
// that already puts every expression check at this boundary
// (folio8_expr_validate.go). So this is ParseTemplate's, exactly as those
// are.
//
// SCOPED TO THE CONTENT BAND, because that is the only band the content
// window describes. A page-header or page-footer table is bounded by its
// own band height, which is a different quantity with a different
// failure mode, and inventing a refusal for it here would reject
// documents on a measurement that does not govern them.
func validateTableMinHeights(t *Template) error {
	g, err := pageGeometryOf(t)
	if err != nil {
		// A page size this library cannot resolve is a DIFFERENT
		// failure, and it already has an owner further down the pipeline
		// (pageDimensions, reached from Render). Refusing here on a
		// window that could not be computed would replace that error
		// with this one and mislead the author about which field is
		// wrong.
		return nil
	}
	window := layout.ContentHeight(g)
	if el, minHeight, ok := tableFloorAbove(t, window); ok {
		return newRenderError(DiagCodeTableMinHeightUnplaceable, string(el.ID), "",
			fmt.Errorf("folio8: element %s: minHeight is taller than the content window (%spt against a content height of %spt), so the table fits on no page — reduce minHeight, or increase the page's content height by reducing its margins or its page-header/page-footer heights",
				el.ID, template.FormatPoints(minHeight), template.FormatPoints(window)))
	}
	return nil
}

// tableFloorAbove returns the first content-band table whose minHeight is
// taller than window.
func tableFloorAbove(t *Template, window geom.Length) (template.Element, geom.Length, bool) {
	for _, el := range contentElements(t) {
		if el.Type != template.ElementTable || !el.Table.Set || el.Table.Null {
			continue
		}
		minHeight := el.Table.Value.MinHeight
		if minHeight.Set && !minHeight.Null && minHeight.Value > window {
			return el, minHeight.Value, true
		}
	}
	return template.Element{}, 0, false
}

// refuseStrandedFloor is the COMMAND door's half of the same rule (review
// item 1): a command that has just changed the page, its margins or a band
// height is refused, located and naming the table, when the content window it
// leaves is shorter than a content-band table's minHeight — otherwise the
// designer saves a document ParseTemplate then refuses to reopen. The caller
// restores its own mutation.
func refuseStrandedFloor(t *Template, path string) error {
	g, err := pageGeometryOf(t)
	if err != nil {
		return nil
	}
	window := layout.ContentHeight(g)
	if el, minHeight, ok := tableFloorAbove(t, window); ok {
		return componentFailure(string(el.ID), path, fmt.Sprintf("this leaves a content window of %spt, shorter than table %s's minHeight of %spt, so that table would fit on no page — reduce its minHeight first", template.FormatPoints(window), el.ID, template.FormatPoints(minHeight)))
	}
	return nil
}
