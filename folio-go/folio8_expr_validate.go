package folio8

// This file is ParseTemplate's Story 3.2 obligation (R1, R3, AC19,
// AC21): every "{{ }}" expression in the document is parsed and
// statically checked here, at load, and D-1.4.1's footerOf/
// footerFormat derivation runs here too — both forced up to this
// module-root package by F2 (internal/template can never import
// internal/expr).

import (
	"fmt"
	"strings"

	"github.com/panitw/folio8/folio-go/internal/expr"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// validateAndDeriveExpressions walks doc's three bands (AC19: every
// "{{ }}" occurrence in the document, not only footer-related ones)
// and returns D-1.4.1's derived-footer results, keyed by column id
// (R2: never written back into doc itself).
func validateAndDeriveExpressions(doc *template.Document) (map[template.ElementID]expr.DerivedFooter, error) {
	derived := map[template.ElementID]expr.DerivedFooter{}
	for _, band := range doc.ElementBands() {
		for _, el := range band.Elements {
			// visibleIf is a common field on every element kind (QA
			// Finding 12, Minor): the field table (folio-format.md,
			// "visibleIf | *Optional.* An expression; …") and this
			// module's own fixtures (fixtures_test.go:298,
			// "visibleIf": "customer.hasTransactions") agree it holds
			// a BARE expression, unlike a text element's `value` or a
			// table column's `bind` — neither of which wraps it in
			// "{{ }}". checkVisibleIfExpression (below) parses and
			// Checks it directly for exactly that reason:
			// checkTextExpressions would scan for "{{ }}" occurrences
			// inside it and find none, passing silently no matter what
			// the string said — a vacuous check, not a real one. Story
			// 3.5 drives visibility from this exact field; a malformed
			// visibleIf (a bare operator, AC19's own named case) must
			// fail at load, the same as a malformed text value or
			// table bind, not load clean and surface only when 3.5
			// starts evaluating it.
			if el.VisibleIf.Set && !el.VisibleIf.Null {
				if err := checkVisibleIfExpression(el.VisibleIf.Value, el.ID); err != nil {
					// Story 3.6, AC4/AC8: FR41's "invalid expression"
					// mode — visibleIf is validated by the SAME
					// expr.Parse/expr.Check machinery checkTextExpressions
					// uses for a text value, so a failure here is the
					// same failure MODE, coded the same way.
					return nil, newRenderError(DiagCodeExpressionInvalid, string(el.ID), "visibleIf", err)
				}
			}
			// Story 3.5, AC4/DECISION-1 (ruled): conditional/data-driven
			// FORMATTING is out of scope forever (AD-24 draws the line
			// at "turns components on and off", never "changes how a
			// component looks") — checked here, alongside visibleIf,
			// because a style field carrying "{{ }}" LOOKS honoured
			// (D-2.4.1/D-3.1.1's precedent: a declaration that looks
			// honoured and silently is not is a load error) and today
			// loads clean and renders inertly (style fields are
			// otherwise entirely unvalidated). See
			// checkStyleHasNoPlaceholders' own doc comment for the
			// scope fence this check does NOT cross.
			if el.Style.Set && !el.Style.Null {
				if err := checkStyleHasNoPlaceholders(el.Style.Value, el.ID, "style"); err != nil {
					return nil, err
				}
			}
			switch el.Type {
			case template.ElementText:
				if el.Value.Set && !el.Value.Null {
					if err := checkTextExpressions(el.Value.Value, el.ID); err != nil {
						// Story 3.6, AC4/AC8, R9: FR41's "invalid
						// expression" mode.
						return nil, newRenderError(DiagCodeExpressionInvalid, string(el.ID), "value", err)
					}
				}
			case template.ElementBarcode, template.ElementQRCode:
				if el.Value.Set && !el.Value.Null {
					if err := checkCodeValue(el); err != nil {
						return nil, err
					}
				}
			case template.ElementTable:
				if el.Table.Set {
					// Story 3.5 finisher review, Finding 4 (Major):
					// table.altRowBackground is a style/appearance
					// field (folio-format.md, FR28: "Colour for
					// alternating rows") and D-3.5.2's ruling covers
					// "any style string field" — not "any style.*
					// string field" — so it belongs under the same
					// fence as style.background, checked the same way,
					// with the same message shape. See
					// checkStyleHasNoPlaceholders' doc comment for the
					// completeness test (TestStyleStringFieldPopulationMatchesSchema,
					// folio8_expr_validate_test.go) that now keeps this
					// call in sync with the schema instead of a silent
					// hand list.
					if alt := el.Table.Value.AltRowBackground; alt.Set && !alt.Null {
						if err := checkStyleStringHasNoPlaceholder(el.ID, "table.altRowBackground", alt.Value); err != nil {
							return nil, err
						}
					}
					// Story 4.1 (owner ruling): headerStyle is a SECOND
					// Style attach point on the same document element,
					// reusing template.Style verbatim — so it goes
					// through the SAME checkStyleHasNoPlaceholders call
					// el.Style already does, above. Its own string
					// fields (align/background/fontFamily/valign/
					// border.color/border.edges) are already covered by
					// TestStyleStringFieldPopulationMatchesSchema via
					// reflectStyleStringFields("Style", ...) — this is
					// a second CALL SITE for the same schema fields,
					// not a new field the population test needs to
					// learn about.
					if hs := el.Table.Value.HeaderStyle; hs.Set && !hs.Null {
						if err := checkStyleHasNoPlaceholders(hs.Value, el.ID, "headerStyle"); err != nil {
							return nil, err
						}
					}
					if err := validateTableColumns(el.Table.Value, derived); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return derived, nil
}

// checkTextExpressions parses and statically checks every non-reserved
// "{{ }}" occurrence in text (AC19, AC10, AC11): a syntax error, a
// wrong arity, an unknown function name, or a wrong-kind literal
// argument is a load error naming elementID and preserving the located
// expression cause before any source excerpt.
func checkTextExpressions(text string, elementID template.ElementID) error {
	_, placeholders, _, serr := expr.ScanPlaceholders(text)
	if serr != nil {
		return fmt.Errorf("folio8: ParseTemplate: element %s: %s", elementID, serr)
	}
	for index, ph := range placeholders {
		if ph.Reserved {
			// AC4/AD-4: {{page}}/{{pages}} are short-circuited before
			// any parse attempt, at load exactly as at render.
			continue
		}
		e, perr := expr.Parse(ph.Inner)
		if perr != nil {
			return fmt.Errorf("folio8: ParseTemplate: element %s: value expression placeholder %d: %w", elementID, index+1, perr)
		}
		if cerr := expr.CheckText(e); cerr != nil {
			return fmt.Errorf("folio8: ParseTemplate: element %s: value placeholder %d: %w", elementID, index+1, cerr)
		}
	}
	return nil
}

// checkVisibleIfExpression parses and statically checks a visibleIf
// field's raw value as a BARE expression (QA Finding 12, Minor) — no
// "{{ }}" wrapping, and so no AD-4 reserved-token short-circuit
// either: "page"/"pages" are reserved only as the interpolation
// SYNTAX "{{page}}"/"{{pages}}", which visibleIf never contains; a
// visibleIf naming a bare "page" path (an unlikely condition, since
// AD-4 means it can only ever resolve against ordinary top-level data,
// never a page-number slot) is not a reservation conflict at all.
func checkVisibleIfExpression(raw string, elementID template.ElementID) error {
	e, perr := expr.Parse(raw)
	if perr != nil {
		return fmt.Errorf("folio8: ParseTemplate: element %s: visibleIf expression: %w", elementID, perr)
	}
	if cerr := expr.CheckCondition(e); cerr != nil {
		return fmt.Errorf("folio8: ParseTemplate: element %s: visibleIf: %w", elementID, cerr)
	}
	return nil
}

// checkStyleStringHasNoPlaceholder rejects a "{{ }}" placeholder inside
// one string-valued style/appearance field, naming the element, the
// field's full dotted path (as it appears in the JSON — e.g.
// "style.background", "table.altRowBackground") and the offending
// placeholder text. The one function both checkStyleHasNoPlaceholders
// (element-level style.*) and validateAndDeriveExpressions'
// ElementTable case (table.altRowBackground) call — a single
// mechanism, not two independently written copies of "does this string
// contain a placeholder" (D-000.38).
func checkStyleStringHasNoPlaceholder(elementID template.ElementID, fieldPath, value string) error {
	if value == "" {
		return nil
	}
	_, placeholders, _, serr := expr.ScanPlaceholders(value)
	if serr != nil {
		// An unterminated "{{" is itself evidence of an attempted
		// placeholder — reported as this same rejection rather
		// than expr's own syntax-error text, since style never
		// supports interpolation of any shape (well-formed or
		// not).
		return fmt.Errorf(
			"folio8: ParseTemplate: element %s: %s %q looks like it contains a \"{{ }}\" placeholder (%s) — "+
				"conditional/data-driven styling is not supported: a component's condition turns it on or off "+
				"(visibleIf), it never changes how the component looks",
			elementID, fieldPath, value, serr,
		)
	}
	if len(placeholders) > 0 {
		return fmt.Errorf(
			"folio8: ParseTemplate: element %s: %s %q must not contain a \"{{%s}}\" placeholder — "+
				"conditional/data-driven styling is not supported: a component's condition turns it on or off "+
				"(visibleIf), it never changes how the component looks",
			elementID, fieldPath, value, placeholders[0].Inner,
		)
	}
	return nil
}

// checkStyleHasNoPlaceholders rejects a "{{ }}" placeholder inside any
// STRING-valued field of an ELEMENT's style block, naming the element,
// the field and the offending placeholder text. table.altRowBackground
// — the table extension's own style/appearance field, outside
// element.style entirely — is checked separately, by the same
// mechanism (checkStyleStringHasNoPlaceholder), from
// validateAndDeriveExpressions' ElementTable case.
//
// THIS IS NOT STYLE-FIELD VALIDATION, and must never be read as such
// (D-000.24: a check that reads as broader than it is is worse than an
// admitted hole). Hex colours, alignment tokens and font-family names
// remain entirely unvalidated, deliberately — that hole is unchanged
// by this story. The ONLY property this checks is "does this string
// contain a placeholder", using expr.ScanPlaceholders — the module's
// ONE tokenizer for "{{ }}" (AD-13, the same mechanism bind.Resolve
// itself calls) — never a strings.Contains("{{") spelling match: if
// ScanPlaceholders says a string carries a placeholder, it is one, by
// the same authority the render path uses to find one.
//
// Ruled scope (DECISION-1), CORRECTED (Story 3.5 finisher review,
// Finding 4 / Major — D-000.46: this comment previously claimed "every
// style field a document can declare TODAY is checked" while
// table.altRowBackground loaded clean, a false map inside the very
// check D-3.5.2 ruled on). What this function checks is exactly six
// element.style.* string fields: align, background, fontFamily,
// valign, border.color, border.edges. TestStyleStringFieldPopulationMatchesSchema
// (folio8_expr_validate_test.go) DERIVES this population from the
// schema by reflection — every Presence[string]/Presence[[]string]
// field on template.Style, template.Border, template.TableExt and
// template.Column — rather than trusting a hand-written list (D-000.67:
// a presence precondition is itself population-keyed, and the fact
// that the original sweep missed table.altRowBackground is the
// evidence a hand list is not enough). That test's own documented
// exclusions name every schema field this check deliberately does NOT
// cover, and why. If a legitimate style value is ever found that must
// legitimately contain "{{", that is the assumption underlying this
// ruling and belongs back to the lead — never resolved by weakening
// this check.
func checkStyleHasNoPlaceholders(st template.Style, elementID template.ElementID, fieldPrefix string) error {
	if st.Align.Set && !st.Align.Null {
		if err := checkStyleStringHasNoPlaceholder(elementID, fieldPrefix+".align", st.Align.Value); err != nil {
			return err
		}
	}
	if st.Background.Set && !st.Background.Null {
		if err := checkStyleStringHasNoPlaceholder(elementID, fieldPrefix+".background", st.Background.Value); err != nil {
			return err
		}
	}
	if st.Color.Set && !st.Color.Null {
		if err := checkStyleStringHasNoPlaceholder(elementID, fieldPrefix+".color", st.Color.Value); err != nil {
			return err
		}
	}
	if st.FontFamily.Set && !st.FontFamily.Null {
		if err := checkStyleStringHasNoPlaceholder(elementID, fieldPrefix+".fontFamily", st.FontFamily.Value); err != nil {
			return err
		}
	}
	if st.Valign.Set && !st.Valign.Null {
		if err := checkStyleStringHasNoPlaceholder(elementID, fieldPrefix+".valign", st.Valign.Value); err != nil {
			return err
		}
	}
	if st.Border.Set && !st.Border.Null {
		b := st.Border.Value
		if b.Color.Set && !b.Color.Null {
			if err := checkStyleStringHasNoPlaceholder(elementID, fieldPrefix+".border.color", b.Color.Value); err != nil {
				return err
			}
		}
		if b.Edges.Set && !b.Edges.Null {
			for _, edge := range b.Edges.Value {
				if err := checkStyleStringHasNoPlaceholder(elementID, fieldPrefix+".border.edges", edge); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// validateTableColumns checks every column's bind (same static rules
// as checkTextExpressions) and, for a column requesting a sum/avg
// footer with footerOf omitted, runs D-1.4.1's derivation (AC21):
// derivable, the resolved value is recorded in derived; not derivable,
// a load error naming the column id (AC21's rejection arm — a bind
// like {{if(row.x, row.a, row.b)}} does not name a single numeric
// source the way the two derivable shapes do).
//
// footer: "count" is special (D-1.4.1): it always counts the table's
// own bound collection, never a per-column numeric source, so no
// derivation is attempted for it at all — and footerOf present
// alongside footer: "count" is already a load error at the template
// layer (AC43 check 1, internal/template/parse_bands.go).
func validateTableColumns(tbl template.TableExt, derived map[template.ElementID]expr.DerivedFooter) error {
	alias := resolvedRowAlias(tbl.As)
	collection := strings.TrimSuffix(tbl.Bind, "[]")

	for _, col := range tbl.Columns {
		if err := checkTextExpressions(col.Bind, col.ID); err != nil {
			// Story 3.6, AC4/AC8, R9: FR41's "invalid expression" mode
			// — a column's bind uses the same static-check machinery a
			// text element's value does.
			return newRenderError(DiagCodeExpressionInvalid, string(col.ID), "bind", err)
		}

		if !col.Footer.Set || col.FooterOf.Set {
			continue
		}
		if col.Footer.Value != "sum" && col.Footer.Value != "avg" {
			continue // "count" never derives a numeric source
		}

		result, derivable, err := expr.DeriveFooterOf(col.Bind, alias, collection)
		if err != nil {
			return fmt.Errorf("folio8: ParseTemplate: column %s: %s", col.ID, err)
		}
		if !derivable {
			// Story 3.6, R8/DW-6: TABLE_FOOTER_SOURCE_UNRESOLVED — a
			// column requesting a sum/avg footer with footerOf omitted
			// and a bind that is not one of D-1.4.1's two derivable
			// shapes.
			return newRenderError(DiagCodeTableFooterSourceUnresolved, string(col.ID), "", fmt.Errorf(
				"folio8: ParseTemplate: column %s: footer %q requires footerOf, and bind %q is not one of D-1.4.1's two derivable shapes — "+
					"name the numeric source explicitly with footerOf",
				col.ID, col.Footer.Value, col.Bind,
			))
		}
		derived[col.ID] = result
	}
	return nil
}
