// This file is Story 9.1's whole engine obligation: an element's
// `style.background` and `style.border` finally reach the page model.
//
// Before this story they reached NOTHING outside a table. The format
// carried them, parse_bands.go decoded them, folio8_expr_validate.go
// checked them for placeholders, serialize.go wrote them back — and the
// only consumer on any render path was table_render.go, which resolves
// the TABLE element's own style into per-cell chrome (resolveHeaderStyle
// / resolveBodyStyle, `base = el.Style.Value`). A text, image, rect or
// line element's background and border were inert, and rect and line
// reached the page model not at all (render_visibility.go's "LATENT, not
// broken": their verdicts were computed and unconsumed).
//
// The shape of the fix is deliberately NOT new machinery. A box is a
// rect group with a band, an element id and a vertical extent — which is
// exactly tableRectSource, the carrier Story 4.1 already built and that
// paginateDocument, contentColumnItems and the page assembler already
// know how to place, repeat, shift and clip. So this file produces
// tableRectSource values through buildCellRectWithBackgroundField, the
// SAME builder that draws a table cell's chrome, and adds no second
// implementation of "what does style.border mean" (D-000.38's rule,
// applied to geometry rather than to placeholders).
//
// What keeps AD-21's corpus a witness rather than a casualty: an element
// with neither background nor border produces no source at all, so a
// document that declares none — every golden in the corpus — reaches
// paginateDocument with a byte-identical rect slice.
package folio8

import (
	"fmt"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/layout"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// collectElementBoxRects walks every band's elements in document order
// and returns one tableRectSource per element that declares a box.
//
// Order is bands in documentBands' authored order (header, content,
// footer), then elements in declaration order within a band — the same
// order collectBandTableRuns walks — because that order is the emitted
// byte order.
//
// Four kinds are eligible: text, image, rect and line. A TABLE is
// excluded by name (not by omission), AND SPEC-table-rules CHANGED THE
// REASON WITHOUT CHANGING THE EXCLUSION.
//
// It used to be excluded because its style.background and style.border
// were consumed as the cell chrome Epic 4 drew. Those two keys now paint the
// table's own FRAME and nothing else — the perimeter half of the split — so
// a table does have a box, and the exclusion here would look like the thing
// to delete.
//
// IT IS NOT, AND THE REASON IS GEOMETRY, NOT POLICY. This function draws a
// DECLARED rectangle (declaredBox: both width and height present and
// positive). A table declares no height — AD-13 refuses one at load, and
// `minHeight` is a FLOOR under a derivation, not a height — and a table's
// frame closes PER PAGE, so its rectangle is known only once pagination has
// assigned each page's slice. table_frame.go's applyTableFrames draws it
// there, through the same buildCellRectWithBackgroundField builder.
func collectElementBoxRects(bands []bandWithOrigin, visible visibilityVerdicts) ([]tableRectSource, error) {
	var sources []tableRectSource
	for bandIndex, b := range bands {
		for _, el := range b.band.Elements {
			if el.Type == template.ElementTable {
				// See this function's doc comment: a table's frame is
				// real but built per page slice after pagination
				// (table_frame.go), not here where only a declared
				// rectangle is available.
				continue
			}
			if !isVisible(visible, el.ID) {
				// AD-24's Visibility clause, the same reading
				// collectBandTableRuns applies to a hidden table: absent
				// from the page model entirely, and no sibling moves.
				continue
			}
			if !elementDeclaresBox(el) {
				// BOTH halves of the rule, in one gate, so that the
				// predicate page_setup.go calls IS the condition this loop
				// applies rather than its first clause:
				//
				//   no declaration — the corpus's own path: no
				//   style.background and no style.border, so no source and
				//   no change to the emitted bytes;
				//
				//   no rectangle — a box needs one with area. parse_bands.go
				//   makes width and height REQUIRED on every non-table
				//   element, so the absent case is defensive; the reachable
				//   one is a zero or negative rectangle, which the loader
				//   accepts and which has no box to draw. Nothing is drawn
				//   and the render is otherwise unaffected — the null-asset
				//   image's precedent, where the element is present and the
				//   paint is absent.
				continue
			}
			style, hasBackground, hasBorder := elementBoxDeclaration(el)
			// sized is necessarily true here: elementDeclaresBox above is
			// the conjunction, and declaredBox is a pure function of el.
			w, h, _ := declaredBox(el)
			rect, err := buildCellRectWithBackgroundField(
				string(el.ID),
				el.X, layout.PlaceInBand(b.origin, el.Y), w, h,
				hasBackground, style.Background.Value, "style.background",
				hasBorder, style.Border.Value,
			)
			if err != nil {
				return nil, err
			}
			sources = append(sources, tableRectSource{
				band:      bandIndex,
				elementID: string(el.ID),
				top:       layout.PlaceInBand(b.origin, el.Y),
				bottom:    layout.PlaceInBand(b.origin, el.Y) + h,
				rects:     []pagemodel.Rect{rect},
				// isDataRow/isHeaderRow/isFooterRow stay false: an element
				// box is not a table row, so chromeRowGroup returns the
				// zero ItemGroup and this source paginates as its own
				// ungrouped content item — and rectIsDataRow[ref] is false
				// downstream, so no row displacement is ever applied to it.
			})
		}
	}
	return sources, nil
}

// elementBoxDeclaration is THE ONE READING of "this element declares a
// box", and the only place in the module that spells it. A box exists
// where a PRESENT, non-null `style.background` does, or where a
// `style.border` that PAINTS INK does (borderPaints below) — nothing
// else, and in particular nothing about the element's kind, its size or
// whether the canvas can see it.
//
// It returns the style beside the two flags because the builder needs
// all three, and splitting them would put the same two Presence tests in
// two places. Callers that only need the verdict use elementDeclaresBox.
func elementBoxDeclaration(el template.Element) (style template.Style, hasBackground, hasBorder bool) {
	s, ok := elementStyle(el)
	if !ok {
		return template.Style{}, false, false
	}
	return s, s.Background.Set && !s.Background.Null, borderPaints(s.Border)
}

// borderPaints is THE ONE READING of "this border puts ink on the page":
// present, non-null AND not an explicitly empty edge set.
//
// It is NOT a new policy — it is an existing condition lifted one layer.
// internal/pdf/rectdoc.go:57 emits a stroke group only under
// `HasStroke && (Top||Right||Bottom||Left)`, so the emitter has always
// known that a border declaring no edges draws nothing; the placer above
// it simply never asked. A `"border": {"edges": []}` therefore declared a
// box, painted nothing, and still cost a page, because elementDeclaresBox
// treated presence alone as a declaration.
//
// `"border": {}` is UNAFFECTED and must stay so: an absent Edges means all
// four edges (buildCellRectWithBackgroundField's default), which is one
// stroke group of ink. Only an edges array that is present, non-null and
// EMPTY paints nothing — the same three-part test every other Presence
// reader in this file applies, with the emptiness clause added.
//
// page_setup.go's applyCanvasStyle is its second caller, so the projection
// and the placer answer this question identically by construction.
func borderPaints(border template.Presence[template.Border]) bool {
	if !border.Set || border.Null {
		return false
	}
	edges := border.Value.Edges
	return !(edges.Set && !edges.Null && len(edges.Value) == 0)
}

// elementDeclaresBox is THE WHOLE PLACEMENT RULE for an element box, as a
// predicate: a present, non-null background or a border that paints ink
// (borderPaints), AND a declared rectangle with area.
// collectElementBoxRects above gates on exactly this call, so the two
// cannot drift; page_setup.go's window count is its second caller.
//
// BOTH CLAUSES ARE LOAD-BEARING, and the second one is why this predicate
// is not just elementBoxDeclaration's verdict. A styled element whose
// height is zero or negative is a rectangle the loader accepts and this
// file draws nothing for, so a caller that read only the style clause
// would place a column item the printed document has nothing in — the
// same shape of error as the unstyled rect that made this predicate
// necessary, one clause further down.
//
// WHAT IT DELIBERATELY LEAVES TO ITS CALLER, because neither is a
// property of the element alone:
//
//	the VISIBILITY verdict — collectElementBoxRects consults
//	isVisible before it asks this question at all, and the canvas has
//	no verdicts, having no data (page_setup.go registers that as a
//	cause of inexactness rather than resolving it);
//
//	the TABLE exclusion — a table never reaches this file (excluded by
//	name above), so this predicate's answer for one is meaningless and
//	page_setup.go's switch must not ask it.
//
// It exists so that "an element box is a declared style over a declared
// rectangle" is written down once. The canvas used to place every
// non-text component whatever its style, so an unstyled rect occupied a
// window the printed document does not have — invisible while the two
// answers were both ungrouped, and a confidently wrong count the moment a
// group made the canvas's partition matter. A second copy of the rule in
// page_setup.go would have been the same defect waiting on the next
// divergence.
func elementDeclaresBox(el template.Element) bool {
	_, hasBackground, hasBorder := elementBoxDeclaration(el)
	if !hasBackground && !hasBorder {
		return false
	}
	_, _, sized := declaredBox(el)
	return sized
}

// elementStyle returns el's style block, and whether it has one at all.
// A present-null style (`"style": null`) is no style, exactly as every
// other reader of a Presence treats it.
func elementStyle(el template.Element) (template.Style, bool) {
	if !el.Style.Set || el.Style.Null {
		return template.Style{}, false
	}
	return el.Style.Value, true
}

// declaredBox returns el's declared width and height, and whether BOTH
// are present, non-null and positive. A box is drawn only from a
// rectangle the author actually declared; nothing here infers one from
// measured content, which would make the box a function of the data —
// and a text element's declared height is read by nothing else in the
// renderer (D-2.8.1: the declared WIDTH is FR44's only clip bound), so
// the box neither gains nor loses a clip bound by being drawn.
func declaredBox(el template.Element) (w, h geom.Length, ok bool) {
	if !el.Width.Set || el.Width.Null || !el.Height.Set || el.Height.Null {
		return 0, 0, false
	}
	if el.Width.Value <= 0 || el.Height.Value <= 0 {
		return 0, 0, false
	}
	return el.Width.Value, el.Height.Value, true
}

// elementInk resolves a style block's `color` into page-model channels:
// Story 10.1's text ink, the one style colour that paints glyphs rather
// than a rectangle. The loader has already refused any colour that is not
// #RRGGBB (internal/template's IsHexColour, TEMPLATE_FIELD_INVALID), so
// the decode below cannot fail for a loaded template; its error arm is an
// unreachable guard and carries no public code.
//
// A style that is absent, null, or carries no `color` returns hasInk
// false, and every producer then emits no colour operator at all: the
// state every document rendered in before this field existed.
func elementInk(style template.Presence[template.Style], elementID, fieldPath string) (pagemodel.Color, bool, error) {
	if !style.Set || style.Null {
		return pagemodel.Color{}, false, nil
	}
	return styleInk(style.Value, elementID, fieldPath)
}

// styleInk is elementInk over an already-resolved style block — the form
// a table's cascade (resolveHeaderStyle / resolveBodyStyle) hands over,
// where the Presence wrapper has already been unwrapped.
func styleInk(st template.Style, elementID, fieldPath string) (pagemodel.Color, bool, error) {
	if !st.Color.Set || st.Color.Null {
		return pagemodel.Color{}, false, nil
	}
	c, ok := parseHexColor(st.Color.Value)
	if !ok {
		return pagemodel.Color{}, false, fmt.Errorf("folio8: Render: element %s: %s %q is not a #RRGGBB colour, which the loader refuses (unreachable)", elementID, fieldPath, st.Color.Value)
	}
	return c, true, nil
}
