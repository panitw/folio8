package folio8

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/panitw/folio8/folio-go/internal/designer"
	"github.com/panitw/folio8/folio-go/internal/expr"
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
	"github.com/panitw/folio8/folio-go/internal/template"
)

const maxTableColumns = 128

// committedHeaderStyle is the header style block AS THE DOCUMENT
// DECLARES IT — never cascaded, never defaulted. An absent or explicitly
// null `headerStyle` yields the zero Style, which reads as "every field
// absent", which is exactly what it means.
//
// An explicitly null FIELD inside a present block (`headerStyle: {"color":
// null}`, reachable only by hand-editing) also reads as absent here: the
// projection has one member for "what is committed" and no third state to
// put a null in. The cascade's own answer for that case still travels
// intact in the resolved member beside it, because resolveHeaderStyle
// falls through a null exactly as it falls through an absent field.
func committedHeaderStyle(table template.TableExt) template.Style {
	if !table.HeaderStyle.Set || table.HeaderStyle.Null {
		return template.Style{}
	}
	return table.HeaderStyle.Value
}

// committedStyleString reads one Presence[string] the way the projection
// spells absence: the value when it is genuinely set, "" otherwise.
func committedStyleString(value template.Presence[string]) string {
	if value.Set && !value.Null {
		return value.Value
	}
	return ""
}

// committedStyleBool is the same reading for a Presence[bool], and it is
// the site of the collapse the struct comment discloses: absent, null and
// an explicit `false` all come back `false`, because `false` is the only
// spelling of absence a bool has on a wire whose key set is pinned exactly
// in both directions. It is its own function rather than an inline
// expression so there is ONE place to change if that stops being
// acceptable.
func committedStyleBool(value template.Presence[bool]) bool {
	return value.Set && !value.Null && value.Value
}

// tableColumns returns only the selected table's structural column paint
// state. The browser may display it but cannot use it as a template model.
func tableColumns(t *Template, tableID string) (designer.TableColumnsProjection, error) {
	if t == nil {
		return designer.TableColumnsProjection{}, errNilTemplate
	}
	if tableID == "" || len(tableID) > 128 {
		return designer.TableColumnsProjection{}, fmt.Errorf("folio8: table id is invalid")
	}
	_, _, _, element, err := findComponent(t, tableID)
	if err != nil {
		return designer.TableColumnsProjection{}, fmt.Errorf("folio8: table was not found")
	}
	if element.Type != template.ElementTable || !element.Table.Set || element.Table.Null {
		return designer.TableColumnsProjection{}, fmt.Errorf("folio8: component is not a table")
	}
	if len(element.Table.Value.Columns) > maxTableColumns {
		return designer.TableColumnsProjection{}, fmt.Errorf("folio8: table has too many columns for editor projection")
	}
	collection := element.Table.Value.Bind
	alias := "row"
	if element.Table.Value.As.Set && !element.Table.Value.As.Null {
		alias = element.Table.Value.As.Value
	}
	if collection == "" || !rootCollectionPath.MatchString(collection) || len(collection) > 256 || alias == "" || len(alias) > 64 || !boundedIdentifier.MatchString(alias) || reservedRowAlias(alias) {
		return designer.TableColumnsProjection{}, fmt.Errorf("folio8: table configuration cannot be projected")
	}
	// THE CASCADE, ASKED ONCE, HERE. resolveHeaderStyle is the engine's
	// only header cascade; calling it is what makes the resolved members
	// the engine's answer rather than the browser's guess.
	resolved := resolveHeaderStyle(*element)
	committed := committedHeaderStyle(element.Table.Value)
	widths, err := template.TableColumnWidths(*element)
	if err != nil {
		return designer.TableColumnsProjection{}, wrapTableWidthError(err)
	}
	total, _ := projectedSize(*element)
	projection := designer.TableColumnsProjection{
		Sizing:                    "points",
		TotalWidth:                int64(total),
		TableID:                   tableID,
		Collection:                collection,
		Alias:                     alias,
		HeaderHeight:              int64(element.Table.Value.HeaderHeight),
		AltRowBackground:          committedStyleString(element.Table.Value.AltRowBackground),
		HeaderFontFamily:          committedStyleString(committed.FontFamily),
		HeaderFontFamilyResolved:  resolvedHeaderFontFamily(resolved),
		HeaderFontSize:            committedLength(committed.FontSize),
		HeaderFontSizeResolved:    int64(resolved.fontSize),
		HeaderLineSpacing:         committedRatio(committed.LineSpacing),
		HeaderLineSpacingResolved: resolved.lineSpacing,
		HeaderBackground:          committedStyleString(committed.Background),
		HeaderBackgroundResolved:  resolvedHeaderBackground(resolved),
		HeaderColor:               committedStyleString(committed.Color),
		HeaderColorResolved:       committedStyleString(resolved.inkStyle.Color),
		HeaderValign:              committedStyleString(committed.Valign),
		HeaderValignResolved:      resolved.valign,
		HeaderAlign:               committedStyleString(committed.Align),
		HeaderAlignResolved:       resolved.alignFallback,
		HeaderBold:                committedStyleBool(committed.Bold),
		HeaderBoldResolved:        resolved.bold,
		HeaderItalic:              committedStyleBool(committed.Italic),
		HeaderItalicResolved:      resolved.italic,
		HeaderBorderWidth:         committedBorderWidth(committedHeaderBorder(committed).Width),
		HeaderBorderWidthResolved: resolvedHeaderBorderWidth(resolved),
		HeaderBorderColor:         committedStyleString(committedHeaderBorder(committed).Color),
		HeaderBorderColorResolved: resolvedHeaderBorderColor(resolved),
		HeaderBorderEdges:         canonicalEdgeList(committedHeaderBorder(committed).Edges),
		HeaderBorderEdgesResolved: resolvedHeaderBorderEdges(resolved),
		MinHeight:                 committedLength(element.Table.Value.MinHeight),
		RulesWidth:                committedBorderWidth(committedTableRules(element.Table.Value).Width),
		RulesWidthResolved:        resolvedTableRulesWidth(element.Table.Value),
		RulesColor:                committedStyleString(committedTableRules(element.Table.Value).Color),
		RulesColorResolved:        resolvedTableRulesColor(element.Table.Value),
		RulesBetween:              canonicalBoundaryList(committedTableRules(element.Table.Value).Between),
		Columns:                   make([]designer.TableColumnProjection, 0, len(element.Table.Value.Columns)),
	}
	if element.Style.Set && !element.Style.Null && element.Style.Value.Padding.Set && !element.Style.Value.Padding.Null {
		padding := element.Style.Value.Padding.Value
		projection.PaddingLeft = committedBorderWidth(padding.Left)
		projection.PaddingRight = committedBorderWidth(padding.Right)
	}
	projection.PaddingHeaderOverride = committed.Padding.Set && !committed.Padding.Null
	if element.Width.Set {
		projection.Sizing = "proportion"
	}
	for i, column := range element.Table.Value.Columns {
		// A label is bounded in CODE POINTS (SPEC-table-rules §4), the unit the
		// command and the browser guard both count.
		if utf8.RuneCountInString(column.Label) > 256 || widths[i] <= 0 || len(column.ID) == 0 || len(column.ID) > 128 {
			return designer.TableColumnsProjection{}, fmt.Errorf("folio8: table column cannot be projected")
		}
		align := "left"
		if column.Align.Set && !column.Align.Null {
			align = column.Align.Value
		}
		if align != "left" && align != "center" && align != "right" {
			return designer.TableColumnsProjection{}, fmt.Errorf("folio8: table column cannot be projected")
		}
		headerAlign := committedStyleString(column.HeaderAlign)
		if headerAlign != "" && !template.IsColumnHeaderAlign(headerAlign) {
			return designer.TableColumnsProjection{}, fmt.Errorf("folio8: table column cannot be projected")
		}
		footer, footerOf, footerFormat := "", "", ""
		if column.Footer.Set && !column.Footer.Null {
			footer = column.Footer.Value
		}
		if column.FooterOf.Set && !column.FooterOf.Null {
			footerOf = column.FooterOf.Value
		}
		if column.FooterFormat.Set && !column.FooterFormat.Null {
			footerFormat = column.FooterFormat.Value
		}
		if footer != "" && footer != "sum" && footer != "avg" && footer != "count" || (footer == "" && (footerOf != "" || footerFormat != "")) || (footer == "count" && footerOf != "") || len(column.Bind) > 256 || len(footerOf) > 256 || len(footerFormat) > 256 {
			return designer.TableColumnsProjection{}, fmt.Errorf("folio8: table column cannot be projected")
		}
		// A new column has no bind yet. It is deliberately editable so the
		// normal Table Editor can complete it through the Go command boundary;
		// once a non-empty expression exists, retain the stricter projection
		// rules for arbitrary/unsupported expressions.
		row := expr.RowBinding{Editable: column.Bind == ""}
		if column.Bind != "" {
			row, err = expr.ProjectRowBinding(column.Bind, alias)
			if err != nil {
				return designer.TableColumnsProjection{}, fmt.Errorf("folio8: table column cannot be projected")
			}
		}
		proportion := ""
		if column.Proportion.Set {
			proportion = template.FormatProportion(column.Proportion.Value)
		}
		projection.Columns = append(projection.Columns, designer.TableColumnProjection{ID: string(column.ID), Header: column.Label, Width: int64(widths[i]), Proportion: proportion, Align: align, HeaderAlign: headerAlign, HeaderAlignResolved: columnHeaderAlign(resolved.alignFallback, column), Binding: column.Bind, RowField: row.Field, RowFieldEditable: row.Editable, Footer: footer, FooterOf: footerOf, FooterFormat: footerFormat})
	}
	return projection, nil
}

// committedLength and committedRatio are committedStyleString's two
// numeric siblings: the declared value, or 0 for "the document does not
// declare one".
func committedLength(value template.Presence[geom.Length]) int64 {
	if value.Set && !value.Null {
		return int64(value.Value)
	}
	return 0
}

func committedRatio(value template.Presence[int64]) int64 {
	if value.Set && !value.Null {
		return value.Value
	}
	return 0
}

// committedBorderWidth is committedLength's STRING sibling, and it exists for
// exactly one member. It reads the SAME UNITS — integer thousandths of a point
// — and differs only in how it spells absence: "" rather than 0.
//
// It cannot be committedLength, because for a border width 0 is a LEGAL
// DECLARED VALUE. parse_bands.go: "ZERO IS VALID and stays accepted: it is the
// thinnest device line PDF can draw, not an absent border." So `{"width": 0}`
// and no `width` at all are two different documents that a numeric member
// reports identically — and the panel's "nothing here is set, so this header
// row takes the table's own border" was said about the first of them, which is
// false. Absence needs a spelling of its own here; it does not for
// committedLength's other caller, because a zero font size is not a
// declaration.
func committedBorderWidth(value template.Presence[geom.Length]) string {
	if value.Set && !value.Null {
		return strconv.FormatInt(int64(value.Value), 10)
	}
	return ""
}

// resolvedHeaderFontFamily and resolvedHeaderBackground read the two
// cascade results that carry a presence flag beside their value. Empty
// means the cascade found nothing to resolve from — a font chain is
// genuinely optional at this level (the render path raises its own
// located error where a header label actually needs one), and a header
// with no background paints none.
func resolvedHeaderFontFamily(resolved resolvedHeaderStyle) string {
	if resolved.hasFontFamily {
		return resolved.fontFamily
	}
	return ""
}

func resolvedHeaderBackground(resolved resolvedHeaderStyle) string {
	if resolved.hasBackground {
		return resolved.background
	}
	return ""
}

// committedHeaderBorder reads the header's border block AS THE DOCUMENT
// DECLARES IT. An absent or explicitly null `border` yields the zero Border,
// whose three Presence members all read as absent — which is exactly what it
// means, and what makes "clear this attribute back to absent" expressible.
func committedHeaderBorder(committed template.Style) template.Border {
	if !committed.Border.Set || committed.Border.Null {
		return template.Border{}
	}
	return committed.Border.Value
}

// THE THREE RESOLVED BORDER MEMBERS, AND ALL THREE ASK THE RENDERER'S OWN
// FUNCTIONS. resolvedBorderWidth, resolvedBorderColor and resolvedBorderEdges
// live in table_render.go beside the emitter that consumes them, so what this
// projection tells the author is what the PDF draws — by construction, not by
// two implementations agreeing. `resolved.hasBorder` is resolveHeaderStyle's own
// verdict on whether ANY border reaches the header row.
// resolvedHeaderBorderWidth is a STRING for the same reason its committed twin
// is — the pair shares one type, as every pair on this struct does — and the
// formatting happens HERE, in Go, so the browser is never the place a length
// acquires a spelling. Thousandths, unchanged; "" only when no border reaches
// the header row at all.
func resolvedHeaderBorderWidth(resolved resolvedHeaderStyle) string {
	if !resolved.hasBorder {
		return ""
	}
	return strconv.FormatInt(int64(resolvedBorderWidth(resolved.border)), 10)
}

func resolvedHeaderBorderColor(resolved resolvedHeaderStyle) string {
	if !resolved.hasBorder {
		return ""
	}
	return resolvedBorderColor(resolved.border)
}

// resolvedHeaderBorderEdges is the member that carries "nothing is painted",
// and it says so by being EMPTY — for both of the two ways that happens: no
// border resolves at all, and a border whose declared `edges` names no side.
// internal/pdf/rectdoc.go's emission gate is exactly that disjunction, so an
// empty string here and no stroke in the PDF are the same condition.
func resolvedHeaderBorderEdges(resolved resolvedHeaderStyle) string {
	if !resolved.hasBorder {
		return ""
	}
	return edgeListOf(resolvedBorderEdges(resolved.border))
}

// committedTableRules reads `table.rules` AS THE DOCUMENT DECLARES IT
// (SPEC-table-rules). An absent or explicitly null block yields the zero
// TableRules, whose three Presence members all read as absent — which is
// exactly what it means, and what makes "clear these back to absent"
// expressible from the panel.
func committedTableRules(table template.TableExt) template.TableRules {
	if !table.Rules.Set || table.Rules.Null {
		return template.TableRules{}
	}
	return table.Rules.Value
}

// resolvedTableRulesWidth / resolvedTableRulesColor ask the RENDERER'S OWN
// functions, exactly as the header-border trio above does, so the panel
// cannot show a number the PDF does not draw. They are "" when the table
// declares no `rules` block at all — there is then nothing to resolve,
// and no line to state a width or a colour for.
func resolvedTableRulesWidth(table template.TableExt) string {
	if !table.Rules.Set || table.Rules.Null {
		return ""
	}
	rules := table.Rules.Value
	return strconv.FormatInt(int64(resolvedBorderWidth(template.Border{Width: rules.Width})), 10)
}

func resolvedTableRulesColor(table template.TableExt) string {
	if !table.Rules.Set || table.Rules.Null {
		return ""
	}
	rules := table.Rules.Value
	return resolvedBorderColor(template.Border{Color: rules.Color})
}

// canonicalBoundaryList is canonicalEdgeList's sibling for
// `rules.between`: the boundary names in RuleBoundaryTokens' own order,
// comma-joined, "" for none. A SEPARATE function from canonicalEdgeList
// and deliberately not a generalisation of it — an edge is one of a
// cell's four sides and a boundary is one of the table's interior seams,
// and SPEC-table-rules keeps the two vocabularies apart precisely because
// conflating them is the defect it repairs.
func canonicalBoundaryList(between template.Presence[[]string]) string {
	if !between.Set || between.Null {
		return ""
	}
	declared := map[string]bool{}
	for _, b := range between.Value {
		declared[b] = true
	}
	var out []string
	for _, token := range template.RuleBoundaryTokens {
		if declared[token] {
			out = append(out, token)
		}
	}
	return strings.Join(out, ",")
}

// canonicalEdgeList and edgeListOf are the ONE spelling of an edge set on this
// wire: the four names in the format's own order, comma-joined, and "" for none.
// The order is canonical rather than the author's, because the browser's guard
// admits a canonical list and a re-ordered one would be refused — and because a
// set has no order to preserve. An unknown name in a hand-edited document is
// dropped here exactly as the renderer's own switch drops it; the loader is the
// door that refuses it, and this projection is not a second one.
func canonicalEdgeList(edges template.Presence[[]string]) string {
	if !edges.Set || edges.Null {
		return ""
	}
	declared := pagemodel.RectEdges{}
	for _, edge := range edges.Value {
		switch edge {
		case "top":
			declared.Top = true
		case "right":
			declared.Right = true
		case "bottom":
			declared.Bottom = true
		case "left":
			declared.Left = true
		}
	}
	return edgeListOf(declared)
}

func edgeListOf(edges pagemodel.RectEdges) string {
	names := make([]string, 0, 4)
	for _, edge := range []struct {
		name string
		on   bool
	}{{"top", edges.Top}, {"right", edges.Right}, {"bottom", edges.Bottom}, {"left", edges.Left}} {
		if edge.on {
			names = append(names, edge.name)
		}
	}
	return strings.Join(names, ",")
}
