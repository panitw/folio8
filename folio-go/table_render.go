// This file is Story 4.1's header-row rendering: a table's column
// geometry, its header cells' vector chrome (border/padding/
// background), and its column labels — the header LABEL is the ONLY
// text this story produces (AC9: zero data rows).
//
// SCOPE FENCE, stated once here rather than at every function: rows,
// cell text wrapping, pagination across pages, the repeated header on
// continuation pages, footer aggregates and alternating row shading are
// explicitly NOT this file's job (4.2-4.8, per the story's own AC9
// table). This file produces exactly one thing per table: its header
// row, once, wherever the table's own Y places it.
package folio8

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/panitw/folio8/folio-go/internal/bind"
	"github.com/panitw/folio8/folio-go/internal/expr"
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/layout"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
	"github.com/panitw/folio8/folio-go/internal/template"
	"github.com/panitw/folio8/folio-go/internal/text"
)

// effectiveFooterOf resolves col's numeric source for a sum/avg footer
// (Story 4.5): the EXPLICIT footerOf when the column carries one, else
// D-1.4.1's DERIVED value from doc.derivedFooters (Story 3.2's
// derivation, first consumed here — Finding 1, this story's creation
// probe). Never re-derived (D-4.2.2): load-time validation
// (folio8_expr_validate.go's validateTableColumns) already ran
// expr.DeriveFooterOf and is the ONLY caller of that function; this
// looks its answer up.
func effectiveFooterOf(doc *Template, col template.Column) (string, bool) {
	if col.FooterOf.Set {
		return col.FooterOf.Value, true
	}
	if d, ok := doc.derivedFooters[col.ID]; ok && d.FooterOf != "" {
		return d.FooterOf, true
	}
	return "", false
}

// effectiveFooterFormat resolves col's display pattern: EXPLICIT
// footerFormat when set, else D-1.4.1's derived pattern (shape 2 only
// — shape 1 derives no format at all), else false ("absent and
// underived").
func effectiveFooterFormat(doc *Template, col template.Column) (string, bool) {
	if col.FooterFormat.Set {
		return col.FooterFormat.Value, true
	}
	if d, ok := doc.derivedFooters[col.ID]; ok && d.HasFooterFormat {
		return d.FooterFormat, true
	}
	return "", false
}

// footerCellExprText builds the "{{ }}" text that computes col's footer
// VALUE (Story 4.5, AC4/DW-7): the SAME aggregate evaluation as an
// author-written {{sum(...)}}/{{avg(...)}}/{{count(...)}} expression,
// because it IS one — synthesised and handed to bind.Resolve, the one
// display-text function every other cell already uses (D-1.4.1),
// rather than a second, parallel evaluator. D-000.79 §2: this is the
// developer's mechanism choice for "routes through the same
// evaluation" — a second route that provably reached the same kernel
// would also satisfy AC4, but this one makes the identity a
// STRUCTURAL fact (it is literally the same parser/evaluator entry
// point) rather than an argued one.
//
// formatNumber is ALWAYS applied — never a bare {{sum(...)}}. When this
// was written bind.Resolve rejected a number in text outright; since the
// number-in-text-binding spec (2026-09-13) a bare number prints as its
// exact decimal, but the footer keeps formatNumber so footerFormat and
// the unformatted-scale pattern below stay the one styling route.
//
// THE "ABSENT AND UNDERIVED" CASE, CORRECTED (this story's review,
// Blocker 1). D-1.4.1 rules it verbatim: "Absent and underived, the
// footer renders UNFORMATTED". This story's first pass used a fixed
// pattern literal "0" there and argued the grammar admitted nothing
// better — the argument was half right and the conclusion was wrong.
// "0" is ZERO fraction digits, so a true total of 30.85 rendered "31":
// a silently altered money figure on the one path AD-23 exists to keep
// exact. The grammar genuinely cannot SPELL "the value's own scale"
// (its fraction part is drawn from '0' alone, never '#'), but it can be
// COMPUTED from the value — which is what expr.UnformattedPattern does,
// and what the bind.EvaluateValue call below exists to feed it. The
// aggregate is evaluated ONCE as a NUMBER, through the SAME
// bind→expr→SumDecimals/AvgDecimals seam the display expression itself
// then re-enters (AC4/DW-7 is unaffected: this adds no second
// arithmetic, only a second traversal of the one that already exists),
// its own scale becomes the pattern, and the value renders at its own
// precision with no grouping and no zero-padding. The one residual —
// a scale beyond the grammar's own maxPatternFractionDigits ceiling —
// is documented on UnformattedPattern itself and binds an explicit
// author-written footerFormat exactly as hard.
//
// collectionEmpty (AC9, D-3.1a.2, Story 4.2's own empty-collection AC)
// is the one exception to "always wrap in formatNumber": avg() over an
// empty collection resolves to expr.KindNull (a Caveat, not an error —
// evalAvg's own R9/DECISION-5 guard, aggregate.go), and
// evalFormatNumber HARD-ERRORS on any non-KindNumber operand
// ("operand must be a number, got null (never coerced)",
// numberformat.go) — wrapping it would turn AC9's Warning-and-empty-
// cell into a render-aborting Error, which is exactly the AD-14
// violation AC9 exists to forbid. So when the table's bound collection
// is empty, an avg footer's expression text omits formatNumber
// entirely and resolves bare: bind.Resolve's own KindNull handling
// ("AD-14: an explicit JSON null renders as empty, never an error")
// takes it from there, and the Caveat it collects is what becomes
// AC9's Warning (diagnosticFromCaveat, reusing DiagCodeEmptyAverage —
// D-000.65: no new code). sum/count are unaffected: SumDecimals(nil)
// and CollectionLength(empty) both resolve to a real KindNumber zero
// (D-3.1a.2's own two-zeros ruling), never KindNull, so they always
// go through formatNumber exactly like the non-empty case.
func footerCellExprText(doc *Template, tbl template.TableExt, col template.Column, collectionEmpty bool, scope bind.Scope, fc expr.FormatContext) (string, error) {
	if col.Footer.Value == "avg" && collectionEmpty {
		footerOf, ok := effectiveFooterOf(doc, col)
		if !ok {
			return "", fmt.Errorf("folio8: Render: internal error: column %s: footer %q has no resolved footerOf at render time", col.ID, col.Footer.Value)
		}
		return "{{avg(" + footerOf + ")}}", nil
	}
	var inner string
	switch col.Footer.Value {
	case "count":
		// D-1.4.1/"Things the schema and record could not resolve" #4:
		// count's operand is the table's own bound collection, never a
		// per-column source — footerOf is a load error alongside it
		// (parse_bands.go's own AC43 check 1).
		inner = "count(" + strings.TrimSuffix(tbl.Bind, "[]") + ")"
	case "sum", "avg":
		footerOf, ok := effectiveFooterOf(doc, col)
		if !ok {
			// Unreachable: validateTableColumns (folio8_expr_validate.go)
			// already rejects a sum/avg footer whose footerOf is
			// neither explicit nor derivable
			// (DiagCodeTableFooterSourceUnresolved) before render ever
			// runs. Kept as a located error, never a panic (AD-14), in
			// case that load-time contract is ever broken.
			return "", fmt.Errorf("folio8: Render: internal error: column %s: footer %q has no resolved footerOf at render time", col.ID, col.Footer.Value)
		}
		inner = col.Footer.Value + "(" + footerOf + ")"
	default:
		// Unreachable: parse_bands.go's closedFooterKinds already
		// rejects anything outside {sum, count, avg} at load time.
		return "", fmt.Errorf("folio8: Render: internal error: column %s: footer %q is not one of sum, count, avg", col.ID, col.Footer.Value)
	}
	pattern, ok := effectiveFooterFormat(doc, col)
	if !ok {
		// D-1.4.1's "absent and underived" arm: UNFORMATTED, which is
		// the value's own scale — see this function's doc comment for
		// why that has to be computed rather than written down.
		v, _, verr := bind.EvaluateValue(inner, scope, fc, string(col.ID))
		if verr != nil {
			return "", fmt.Errorf("folio8: Render: column %s: footer: %w", col.ID, verr)
		}
		if v.Kind != expr.KindNumber {
			// AD-14's wrong-kind Error, never coerced and never a panic
			// (R7/D-000.65: an existing shape, no new code). Reachable
			// only if an aggregate ever resolves to a non-number here;
			// sum/count always resolve to a KindNumber, and the one
			// KindNull case (avg over an empty collection) returned
			// above before reaching this line.
			return "", fmt.Errorf(
				"folio8: Render: column %s: footer %q resolved to a %s, not a number (never coerced)",
				col.ID, col.Footer.Value, v.Kind,
			)
		}
		pattern = expr.UnformattedPattern(v.Num)
	}
	return "{{formatNumber(" + inner + ", " + strconv.Quote(pattern) + ")}}", nil
}

// tableRectSource is one table's header-row vector chrome (Story 4.1,
// R1/AC3/AC6): one pagemodel.Rect per column cell, ALWAYS populated
// when the table has at least one column — even a fully style-less
// table's cells are represented, carrying HasFill==false and
// HasStroke==false, so the header row still occupies its own page
// space (D-2.6.5's own rule: an item that occupies space must not be
// empty) without drawing anything (R6).
//
// It mirrors imageRunSource's shape (band + a page-absolute extent),
// deliberately: this is a SECOND, PARALLEL content kind alongside text
// runs and image placements, not a variant of either.
type tableRectSource struct {
	band        int
	elementID   string
	top, bottom geom.Length
	rects       []pagemodel.Rect

	// isDataRow / rowIndex — Story 4.2, DECISION-2 (owner ruling): a
	// carried identity for "which bound-collection row produced this
	// chrome group", so Story 4.3 ("a row moves whole to the next
	// page") can group this story's output by DIRECT FIELD LOOKUP
	// rather than by reconstructing membership from element ids,
	// extents or emission order — exactly the reconstruction the
	// ruling names as where a wrapped row silently becomes two.
	//
	// isDataRow is false (and rowIndex meaningless) for the header's
	// own chrome group (Story 4.1, unchanged): a header is not a row.
	// For a data row's chrome group (Story 4.2), isDataRow is true and
	// rowIndex is the row's 0-based position in the bound collection.
	//
	// This story asserts only that the identity itself is correct
	// (TestDataRowIdentityIsConsistentAndDistinct) — it asserts nothing
	// about pagination (AC7). Story 4.3 is this field's one named
	// consumer.
	isDataRow bool
	rowIndex  int

	// isHeaderRow — Story 4.3: the header's OWN grouping identity,
	// parallel to isDataRow/rowIndex above but for the one header row a
	// table has. AC5 extends "a row moves whole to the next page" to the
	// header without special-casing it: the header's chrome and its
	// column labels are one group, exactly as a data row's chrome and
	// its physical lines are one group. Set true ONLY on the header's own
	// tableRectSource (isDataRow stays false there, unchanged); never set
	// alongside isDataRow.
	isHeaderRow bool

	// isFooterRow — Story 4.5: parallel to isHeaderRow/isDataRow above,
	// for the one footer row a table with at least one `footer` column
	// has. Set true ONLY on the footer's own tableRectSource; never
	// alongside isDataRow or isHeaderRow. A row-TYPE tag, kept separate
	// from whatever layout.ItemGroupKey.Index this row's group is
	// carrying for pagination purposes (see chromeRowGroup) precisely so
	// a future consumer keying off row identity (isDataRow/rowIndex —
	// Story 4.8's alternating shading, epics.md: "follows row index in
	// the collection") never mistakes the footer for a row, regardless
	// of the orphan-tie mechanism's own bookkeeping.
	isFooterRow bool

	// frame — SPEC-table-rules: the table's own frame, interior rules and
	// floor, shared by every row source of one table (nil when the table
	// declares none of them). It is NOT a pagination item: the frame is
	// built per page slice AFTER pagination (table_frame.go), because a
	// slice's extent is known only once pages are assigned.
	frame *tableFrame
}

// chromeRowGroup derives this rect source's layout.ItemGroup — Story 4.3's
// grouping identity — by DIRECT FIELD LOOKUP from isDataRow/isHeaderRow/
// rowIndex, never by reconstruction (D-4.2.2, R3). Not grouped
// (layout.ItemGroup{}) for a table with neither: unreachable in practice
// (every tableRectSource is either the header's own or a data row's,
// never neither), but stated rather than assumed.
func (r tableRectSource) chromeRowGroup() layout.ItemGroup {
	switch {
	case r.isHeaderRow:
		return layout.ItemGroup{Present: true, Key: layout.ItemGroupKey{ElementID: r.elementID, IsHeader: true}}
	case r.isFooterRow:
		// Story 4.5: Index -1 is a sentinel no real data row ever
		// carries — see textRunSource.lineRowGroup's matching case for
		// the full rationale (paginateWithFooterOrphanFix, table_footer.go).
		return layout.ItemGroup{Present: true, Key: layout.ItemGroupKey{ElementID: r.elementID, Index: footerGroupIndex}}
	case r.isDataRow:
		return layout.ItemGroup{Present: true, Key: layout.ItemGroupKey{ElementID: r.elementID, Index: r.rowIndex}}
	default:
		return layout.ItemGroup{}
	}
}

// resolvedHeaderStyle is a table's header-row style, cascaded ONCE per
// table (AC3/AC4, and this story's owner-ruled headerStyle addition):
// for every field, `headerStyle.<field>` wins when the table declares
// one AND that field is set within it; otherwise the table's own
// `style.<field>` wins when set; otherwise the field's documented
// default (folio-format.md's Style table, unchanged by this story
// except fontFamily — see the story's Delivery Log). `columns[].align`
// is resolved separately, per column, and still wins over both
// (AC4's own grounds, extended one level: the column's field is the
// most specific of the three).
type resolvedHeaderStyle struct {
	hasFontFamily bool
	fontFamily    string
	fontSize      geom.Length

	// lineSpacing is Story 7.2's leading ratio in thousandths,
	// cascaded exactly as fontSize is: headerStyle wins, then the
	// table's own style, then the neutral default. D-7.1.3 — every
	// caller, no carve-out; one rule for one property.
	lineSpacing int64

	hasBorder bool
	border    template.Border

	hasBackground bool
	background    string

	// Story 10.1: the header row's ink, cascaded exactly as its
	// background is — headerStyle.color wins over style.color.
	inkStyle template.Style

	// Story 11.2 / FR57: the header row's WEIGHT and SLOPE, cascaded in
	// the exact spelling their siblings use. They are here rather than
	// on resolvedBodyStyle for D-000.76's reason: headerStyle governs
	// the header row ALONE, and the body and footer rows cascade from
	// the table's own `style` through fontChain, unchanged.
	bold   bool
	italic bool

	// inkField is WHICH of the two the cascade actually took, carried
	// beside the value so a malformed colour can be located. The error
	// site used to emit the literal "headerStyle.color/style.color",
	// which names both fields and identifies neither: AD-14's contract is
	// that callers match on the CODE — so nothing broke — but AD-14's
	// other half is that a diagnostic LOCATES, and a person handed both
	// names has to guess which file line to edit. The switch below
	// already knows the answer; it simply threw it away. Empty only when
	// no arm was taken, in which case styleInk produces no ink and never
	// reads it.
	inkField string

	padding template.Padding

	valign string // "top", "middle" or "bottom" — never empty

	// alignFallback is what a column without its OWN `align` uses
	// (AC4): headerStyle.align, else style.align, else "left".
	alignFallback string
}

// resolveHeaderStyle cascades el's table style and headerStyle exactly
// once (called once per table element).
func resolveHeaderStyle(el template.Element) resolvedHeaderStyle {
	var base, header template.Style
	if el.Style.Set && !el.Style.Null {
		base = el.Style.Value
	}
	hasHeader := false
	if el.Table.Set && el.Table.Value.HeaderStyle.Set && !el.Table.Value.HeaderStyle.Null {
		header = el.Table.Value.HeaderStyle.Value
		hasHeader = true
	}

	r := resolvedHeaderStyle{valign: "top", alignFallback: "left"}

	switch {
	case hasHeader && header.FontFamily.Set && !header.FontFamily.Null:
		r.hasFontFamily, r.fontFamily = true, header.FontFamily.Value
	case base.FontFamily.Set && !base.FontFamily.Null:
		r.hasFontFamily, r.fontFamily = true, base.FontFamily.Value
	}

	r.fontSize = defaultFontSizePt
	switch {
	case hasHeader && header.FontSize.Set && !header.FontSize.Null:
		r.fontSize = header.FontSize.Value
	case base.FontSize.Set && !base.FontSize.Null:
		r.fontSize = base.FontSize.Value
	}

	r.lineSpacing = defaultLineSpacing
	switch {
	case hasHeader && header.LineSpacing.Set && !header.LineSpacing.Null:
		r.lineSpacing = header.LineSpacing.Value
	case base.LineSpacing.Set && !base.LineSpacing.Null:
		r.lineSpacing = base.LineSpacing.Value
	}

	// ⚠ THE CHROME PAIR HAS NO `style` ARM, AND IT IS THE ONLY PAIR ON
	// THIS CASCADE THAT HAS NONE (SPEC-table-rules §1).
	//
	// Every other field here describes the TEXT INSIDE a cell, so it
	// cascades headerStyle -> style -> default like a text property
	// should. `border` and `background` describe the CHROME AROUND a
	// box, and a table's own `style.border`/`style.background` now paint
	// the TABLE'S OWN BOX — once, around the perimeter — exactly as
	// every other element type's do. Letting them also cascade in here
	// would stroke the frame a second time along the header row's four
	// sides, which is the doubled-stroke defect the split exists to
	// remove.
	//
	// `headerStyle.border`/`headerStyle.background` are UNCHANGED and
	// keep their whole meaning: they are what draws the header's fill
	// and the rule under it, and they remain the ONLY declaration that
	// puts chrome on a header cell.
	if hasHeader && header.Border.Set && !header.Border.Null {
		r.hasBorder, r.border = true, header.Border.Value
	}
	if hasHeader && header.Background.Set && !header.Background.Null {
		r.hasBackground, r.background = true, header.Background.Value
	}

	// `&& !header.Color.Null` is what makes this arm match its EIGHT
	// siblings above and below. Without it, `headerStyle: {"color": null}`
	// WON the cascade with a null and stopped the fall-through to
	// style.color, so a table declaring `style.color: #c81e1e` printed its
	// header in black — measured at 2 ink operations against 3 for the
	// same table with headerStyle absent or `{}`. The background and
	// border arms on the same table already fall through on an explicit
	// null; folio-format.md already states that rule; the code was the
	// outlier, so the fix is here and neither document changes.
	switch {
	case hasHeader && header.Color.Set && !header.Color.Null:
		r.inkStyle.Color, r.inkField = header.Color, "headerStyle.color"
	case base.Color.Set:
		r.inkStyle.Color, r.inkField = base.Color, "style.color"
	}

	// `.Set && !.Null` on BOTH arms, matching the nine siblings around
	// them — the spelling the `color` arm above records a live defect for
	// omitting. An explicit `headerStyle: {"bold": null}` must fall
	// through to `style.bold`, not win with a null.
	switch {
	case hasHeader && header.Bold.Set && !header.Bold.Null:
		r.bold = header.Bold.Value
	case base.Bold.Set && !base.Bold.Null:
		r.bold = base.Bold.Value
	}

	switch {
	case hasHeader && header.Italic.Set && !header.Italic.Null:
		r.italic = header.Italic.Value
	case base.Italic.Set && !base.Italic.Null:
		r.italic = base.Italic.Value
	}

	switch {
	case hasHeader && header.Padding.Set && !header.Padding.Null:
		r.padding = header.Padding.Value
	case base.Padding.Set && !base.Padding.Null:
		r.padding = base.Padding.Value
	}

	switch {
	case hasHeader && header.Valign.Set && !header.Valign.Null:
		r.valign = header.Valign.Value
	case base.Valign.Set && !base.Valign.Null:
		r.valign = base.Valign.Value
	}

	switch {
	case hasHeader && header.Align.Set && !header.Align.Null:
		r.alignFallback = header.Align.Value
	case base.Align.Set && !base.Align.Null:
		r.alignFallback = base.Align.Value
	}

	return r
}

// resolvedBodyStyle is a table's DATA-ROW style, cascaded ONCE per
// table (Story 4.2, AC5; D-000.76's ruling that headerStyle governs
// the header row ONLY): for every field, the table's own
// `style.<field>` wins when set, otherwise the field's documented
// default. There is deliberately NO headerStyle arm here — that is the
// whole point of AC5's fence. `columns[].align` is resolved separately
// per column and still wins over `alignFallback`, exactly as it does
// for the header (AC4, extended one level).
//
// fontFamily/fontSize are NOT carried here: a data row's font chain
// cascades from `style` alone, which is exactly fontChain's own
// cascade (render.go) — reused verbatim so a data cell's "no
// resolvable fontFamily" error is the SAME message the existing
// font-resolution failure produces, never a third spelling (AC5's own
// grounds, D-000.65).
type resolvedBodyStyle struct {
	// NO border/background MEMBERS, and their absence is the point
	// (SPEC-table-rules §1): a data or footer cell carries no chrome from
	// the element's own style, because that declaration paints the
	// TABLE'S OWN BOX. There is nothing here for a cell builder to read,
	// which is what makes the old grid unreachable rather than merely
	// unused.

	// Story 10.1: a data cell's ink, from the table's own style alone —
	// the same arm every other body-cell property cascades through.
	inkStyle template.Style

	padding template.Padding

	valign string // "top", "middle" or "bottom" — never empty

	alignFallback string
}

// resolveBodyStyle cascades el's table style ALONE (never headerStyle)
// exactly once per table (Story 4.2, AC5).
func resolveBodyStyle(el template.Element) resolvedBodyStyle {
	var base template.Style
	if el.Style.Set && !el.Style.Null {
		base = el.Style.Value
	}

	r := resolvedBodyStyle{valign: "top", alignFallback: "left"}

	// SPEC-table-rules §1: a data cell carries NO chrome from the
	// element's own style. `style.border` and `style.background` paint
	// the TABLE'S BOX now, so consuming them here would stamp the frame
	// onto every cell of every row — which is the grid this change
	// replaces, and the reason a 1pt frame used to get thicker as rows
	// were added. The interior lines are `table.rules`, drawn ONCE per
	// boundary from the table's own geometry (collectBandTableRuns),
	// never per cell.
	//
	// `table.altRowBackground` is UNAFFECTED and still fills its rows:
	// it is a row property with no perimeter meaning, and it never
	// belonged to the box.
	// `.Null` beside `.Set`, matching every other arm in this function and
	// the header cascade above. ⚠ THIS CHANGES NOTHING OBSERVABLE TODAY:
	// styleInk opens with `!st.Color.Set || st.Color.Null`, so a null
	// yielded no ink anyway, and this arm is single — there is no second
	// arm to fall through to. It is spelled correctly here because the
	// correctness was ACCIDENTAL AND NON-LOCAL, holding only because a
	// different function re-checks null. resolveHeaderStyle, same file and
	// same property, just demonstrated what this spelling costs when a
	// second arm exists; the next arm or the next copy would inherit a live
	// defect with nothing to announce it.
	if base.Color.Set && !base.Color.Null {
		r.inkStyle.Color = base.Color
	}
	if base.Padding.Set && !base.Padding.Null {
		r.padding = base.Padding.Value
	}
	if base.Valign.Set && !base.Valign.Null {
		r.valign = base.Valign.Value
	}
	if base.Align.Set && !base.Align.Null {
		r.alignFallback = base.Align.Value
	}
	return r
}

// columnAlign is the LAST STEP of the alignment cascade, and the one step both
// rows share: a column's OWN `align` is the most specific of the three
// declarations and wins over whichever row fallback its caller resolved
// (AC4's grounds, extended one level). The fallback differs per row —
// resolveHeaderStyle's for a header cell, resolveBodyStyle's for a data or
// footer cell — and that difference is the caller's, not this function's.
//
// IT IS A FUNCTION BECAUSE STORY 14.9 GAINED A FIFTH CALLER OUTSIDE THE
// RENDERER. The canvas projection now carries a resolved alignment per column
// per row (page_setup.go, canvasComponents), and this three-line pattern was
// open-coded at four sites in this file. A fifth copy in a different file is
// exactly the drift 14.8's Part 4 requirement names: one source shared with the
// renderer, because a projection that MIRRORS the cascade will drift and the
// failure mode is a canvas that lies about print while every test passes.
//
// `columns[].align` is a three-value closed set (ColumnAlignTokens), so this
// never returns `justify` unless the fallback it was handed already was one —
// and TableStyleAlignTokens refuses `justify` on a table's own style at load.
func columnAlign(fallback string, col template.Column) string {
	if col.Align.Set && !col.Align.Null {
		return col.Align.Value
	}
	return fallback
}

// columnHeaderAlign is columnAlign for a HEADER cell, and only a header cell:
// `columns[].headerAlign` wins, then the column's own `align`, then the header
// row's fallback. Data and footer cells never consult it. The renderer's header
// row and the canvas projection's HeaderAlign both call it, so the canvas
// cannot align a heading differently from the PDF.
func columnHeaderAlign(fallback string, col template.Column) string {
	if col.HeaderAlign.Set && !col.HeaderAlign.Null {
		return col.HeaderAlign.Value
	}
	return columnAlign(fallback, col)
}

// paddingEdges returns the four padding insets, each independently
// defaulting to zero when its own field is absent (AC3, R6).
func paddingEdges(p template.Padding) (top, right, bottom, left geom.Length) {
	if p.Top.Set && !p.Top.Null {
		top = p.Top.Value
	}
	if p.Right.Set && !p.Right.Null {
		right = p.Right.Value
	}
	if p.Bottom.Set && !p.Bottom.Null {
		bottom = p.Bottom.Value
	}
	if p.Left.Set && !p.Left.Null {
		left = p.Left.Value
	}
	return
}

// parseHexColor decodes a `#RRGGBB` string into page-model channels
// (AC5/D2). It is the ONLY hex DECODER in the module. Whether a string
// is a colour at all is decided at load by internal/template's
// IsHexColour, so a loaded template never hands this a malformed one.
func parseHexColor(s string) (pagemodel.Color, bool) {
	if len(s) != 7 || s[0] != '#' {
		return pagemodel.Color{}, false
	}
	var out [3]uint8
	for i := 0; i < 3; i++ {
		hi, ok1 := hexDigit(s[1+i*2])
		lo, ok2 := hexDigit(s[2+i*2])
		if !ok1 || !ok2 {
			return pagemodel.Color{}, false
		}
		out[i] = hi<<4 | lo
	}
	return pagemodel.Color{R: out[0], G: out[1], B: out[2]}, true
}

func hexDigit(b byte) (uint8, bool) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', true
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, true
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, true
	default:
		return 0, false
	}
}

// buildHeaderCellRect builds one column cell's vector primitive (AC3,
// R1) from the table's cascaded header style. hs.hasBackground/
// hasBorder absent means "draw nothing for this half" (R6) — never a
// hardcoded header treatment (the owner ruling this story carries: the
// AUTHOR expresses the look via style/headerStyle, this function
// invents none of it).
func buildHeaderCellRect(elementID string, x, y, w, h geom.Length, hs resolvedHeaderStyle) (pagemodel.Rect, error) {
	return buildCellRect(elementID, x, y, w, h, hs.hasBackground, hs.background, hs.hasBorder, hs.border)
}

// buildCellRect builds ONE cell's vector primitive (header or data row
// alike, Story 4.1/4.2) from a resolved has*/value pair for background
// and border — never a hardcoded treatment (the owner ruling both
// stories carry: the AUTHOR expresses the look via style, this
// function invents none of it). hasBackground/hasBorder absent means
// "draw nothing for this half" — a fully style-less cell still gets a
// Rect (HasFill==false, HasStroke==false), so it still occupies its
// own page space (D-2.6.5: an item that occupies space must not be
// empty) without drawing anything.
func buildCellRect(elementID string, x, y, w, h geom.Length, hasBackground bool, background string, hasBorder bool, border template.Border) (pagemodel.Rect, error) {
	return buildCellRectWithBackgroundField(elementID, x, y, w, h, hasBackground, background,
		"style.background/headerStyle.background", hasBorder, border)
}

// THE PAINT-TIME DEFAULTS FOR A BORDER WHOSE SUB-KEYS ARE EACH INDEPENDENTLY
// OPTIONAL, IN ONE PLACE, AND STORY 14.8 IS WHY THEY ARE NAMED AT ALL.
//
// They used to be three literals inlined in buildCellRectWithBackgroundField.
// That was fine while the renderer was the only reader — and it stopped being
// fine the moment the table editor had to SHOW an author what an unset border
// attribute will actually draw. `TableColumns` projects a resolved twin beside
// every committed header-style member (D-12.3.1), and for the border trio that
// twin IS these three answers. A second copy of them in the projection, or a
// third in TypeScript, is a third thing to drift out of step with the PDF; the
// designer already learned that lesson once with `?? 500` / `?? '#000000'` on
// the canvas. So the renderer and the projection ask the same three functions.
//
// `parse_bands.go` is the authority for what may be STORED here (a negative
// width is refused at load; ZERO is accepted and is the thinnest device line
// PDF can draw, not an absent border). These functions decide only what an
// ABSENT sub-key means, which is a render question and belongs in this file.
const (
	// folio-format.md's documented defaults for an unset border sub-key.
	defaultBorderWidth geom.Length = 500
	defaultBorderColor             = "#000000"
)

func resolvedBorderWidth(border template.Border) geom.Length {
	if border.Width.Set && !border.Width.Null {
		return border.Width.Value
	}
	return defaultBorderWidth
}

func resolvedBorderColor(border template.Border) string {
	if border.Color.Set && !border.Color.Null {
		return border.Color.Value
	}
	return defaultBorderColor
}

// resolvedBorderEdges answers ALL FOUR for an absent `edges`, and an explicitly
// declared list by exactly the sides it names — so a declared `[]` resolves to
// no side at all. That is not a hole: `internal/pdf/rectdoc.go` emits a stroke
// only `if r.HasStroke && (Edges.Top || Right || Bottom || Left)`, so no side
// means nothing is painted, and it is the one shape a hand-edited document can
// use to say "this border draws nothing". A command cannot write it — the
// `border.edges` arm refuses the empty array — because clearing the attribute is
// how an author says the same thing without a block that means nothing.
func resolvedBorderEdges(border template.Border) pagemodel.RectEdges {
	if !border.Edges.Set || border.Edges.Null {
		return pagemodel.RectEdges{Top: true, Right: true, Bottom: true, Left: true}
	}
	edges := pagemodel.RectEdges{}
	for _, e := range border.Edges.Value {
		switch e {
		case "top":
			edges.Top = true
		case "right":
			edges.Right = true
		case "bottom":
			edges.Bottom = true
		case "left":
			edges.Left = true
		}
	}
	return edges
}

// buildCellRectWithBackgroundField is buildCellRect's located-field form.
// Most callers use the ordinary style cascade through buildCellRect; an
// alternating data row passes table.altRowBackground so a malformed value is
// reported against the template field that actually supplied it.
func buildCellRectWithBackgroundField(elementID string, x, y, w, h geom.Length, hasBackground bool, background, backgroundField string, hasBorder bool, border template.Border) (pagemodel.Rect, error) {
	rect := pagemodel.Rect{X: x, Y: y, W: w, H: h}

	if hasBackground {
		c, ok := parseHexColor(background)
		if !ok {
			// Unreachable for a loaded template: the loader refuses it.
			return pagemodel.Rect{}, fmt.Errorf("folio8: Render: element %s: %s %q is not a #RRGGBB colour, which the loader refuses (unreachable)", elementID, backgroundField, background)
		}
		rect.HasFill = true
		rect.Fill = c
	}

	if hasBorder {
		width := resolvedBorderWidth(border)
		colorHex := resolvedBorderColor(border)
		c, ok := parseHexColor(colorHex)
		if !ok {
			// Unreachable for a loaded template: the loader refuses it.
			return pagemodel.Rect{}, fmt.Errorf("folio8: Render: element %s: style.border.color/headerStyle.border.color %q is not a #RRGGBB colour, which the loader refuses (unreachable)", elementID, colorHex)
		}
		rect.HasStroke = true
		rect.Stroke = c
		rect.StrokeWidth = width
		rect.Edges = resolvedBorderEdges(border)
	}

	return rect, nil
}

// tableDrawsColumns is THE ONE READING of "this table has something to
// draw": a table extension carrying at least one column. A table declaring
// `"columns": []` parses — parse_bands.go imposes no minimum — and
// collectBandTableRuns below returns no rect source for it, so it reaches
// no content-column item at all.
//
// It is a predicate rather than an inlined test because page_setup.go's
// canvas window count must ask the same question of the same element, and
// a second spelling of it there would put a header rect in the counted
// column that the printed document has nothing in.
func tableDrawsColumns(el template.Element) bool {
	return el.Table.Set && len(el.Table.Value.Columns) > 0
}

// collectBandTableRuns walks one band's table elements and returns the
// header-row label runs (through the SAME shape/position pipeline
// every text element uses — D-16's forcing function: buildShapedPDFRuns
// stays the ONLY producer of pagemodel.ShapedGlyph) plus one
// tableRectSource per VISIBLE table with at least one column.
//
// A HIDDEN table (AC8, AD-24's Visibility clause) contributes nothing
// at all: no label run, no rect — checked before any geometry is even
// computed, exactly as collectBandTextRuns already does for a hidden
// text element.
func collectBandTableRuns(
	doc *Template,
	bands []bandWithOrigin,
	bandIndex int,
	data, params bind.Value,
	fc expr.FormatContext,
	fs FontSet,
	cache *fontCache,
	visible visibilityVerdicts,
) ([]textRunSource, []tableRectSource, []Diagnostic, error) {
	b := bands[bandIndex]
	var runs []textRunSource
	var rectSources []tableRectSource
	var diags []Diagnostic

	for _, el := range b.band.Elements {
		if el.Type != template.ElementTable {
			continue
		}
		if !isVisible(visible, el.ID) {
			// AD-24's Visibility clause: absent from the page model
			// entirely — no header row, no borders, no background —
			// and siblings do not move (nothing here ever adjusts
			// another element's position; band composition is a
			// TRANSLATION, never a negotiation, per internal/layout's
			// own doc comment).
			continue
		}
		if !el.Table.Set {
			continue // structurally unreachable (a table element always carries TableExt), defensive
		}
		if !tableDrawsColumns(el) {
			continue // nothing to lay out or draw
		}
		tbl := el.Table.Value
		hasAltBackground := tbl.AltRowBackground.Set && !tbl.AltRowBackground.Null
		altBackground := tbl.AltRowBackground.Value

		hs := resolveHeaderStyle(el)
		tableTop := layout.PlaceInBand(b.origin, el.Y)
		// tableBottom is the HEADER row's bottom, and it is re-derived
		// below once the labels have been packed: SPEC-table-rules §4
		// makes `headerHeight` a floor, so the row's real height is not
		// known until the packer has run. It is seeded from the declared
		// floor so the declaration reads beside tableTop.
		tableBottom := tableTop + tbl.HeaderHeight

		widths, widthErr := template.TableColumnWidths(el)
		if widthErr != nil {
			return nil, nil, nil, wrapTableWidthError(widthErr)
		}
		geometry := layout.ColumnWidths(el.X, widths)

		rects := make([]pagemodel.Rect, len(tbl.Columns))
		padTop, padRight, padBottom, padLeft := paddingEdges(hs.padding)

		var chain, styledChain, metricsChain []string
		// headerCache is the cache scoped to the HEADER's chain (see
		// fontCache.forChain): a located capability error must name the
		// chain this label draws through, and a table's header and body
		// chains can be different chains of the same document.
		headerCache := cache
		if hs.hasFontFamily {
			// Story 8.4, Task 3: THE SHARED lookup, not a hand-mirrored
			// copy of it. This site used to carry its own Fonts.Chain
			// call and its own spelling of fontChain's error, under a
			// comment saying it "mirrors fontChain's own error, verbatim
			// in shape" — which is a duplicate announcing itself. The
			// message is now owned once, by lookupFontChain (render.go),
			// and it is still plain-wrapped with this element's id and
			// still no *RenderError: the behaviour is unchanged, only its
			// number of homes.
			entries, cerr := lookupFontChain(doc, hs.fontFamily)
			if cerr != nil {
				return nil, nil, nil, fmt.Errorf("folio8: Render: element %s: %w", el.ID, cerr)
			}
			// AFTER the lookup, deliberately: on the not-found path
			// there is nothing to project, and calling it first would
			// also turn `chain` from nil into a non-nil empty slice on
			// the way to an error return.
			// AC2 / FR57: the header row's own cascaded weight and slope,
			// applied HERE. ⚠ This arm reaches chainFaceNames directly
			// and never calls fontChain, so a resolution placed in
			// fontChain alone would pass every text AC and silently skip
			// every table header.
			chain, styledChain = chainFaceNames(entries, fontStyleOf(hs.bold, hs.italic))
			headerCache = cache.forChain(hs.fontFamily)
			// AFTER headerCache, deliberately: the metrics list asks the
			// cache whether each declared variant is actually supplied,
			// so it must be the header's own scoped view of it.
			metricsChain = metricsFaceNames(chain, styledChain, fs, headerCache)
		}

		// --- SPEC-table-rules §4: the header labels go through the
		// BODY CELL'S PACKER ---
		//
		// A label used to be shaped `breaksAreDrawn` and positioned
		// directly against a vertical model that was one line BY
		// CONSTRUCTION, so a `\n` was handed to the shaper as a rune to
		// draw, no font covered it, and the author got a
		// TEXT_MISSING_GLYPH warning and one line. And a label wider than
		// its column was clipped in SILENCE — the one clip path in this
		// file that appended no diagnostic at all.
		//
		// Both are one defect: the header had a second, weaker
		// implementation of a rule the body already had. So it is packed
		// through `breaksAreConsumed` + packLines, against the column's
		// own content width, and the header row GROWS to the packed
		// height. The two modes differ ONLY in whether a U+000A earns a
		// missing-glyph warning (shapeSegments' own note) — segmentation
		// is identical — so a label holding no line feed shapes to the
		// same segments it always did.
		//
		// `headerHeight` therefore becomes a FLOOR rather than an exact
		// height, narrowed exactly as `minHeight` narrows a table's
		// extent and for the same reason: an author declaring a floor is
		// declaring the form's proportions, not overriding what the text
		// needs. The field stays REQUIRED, and the packed height is
		// settled here — at layout, before pagination — so a repeated
		// header is the same height on every page it appears on.
		type headerCell struct {
			lines     []wrappedLine
			segs      []faceSegment
			align     string
			clip      bool
			clipX     geom.Length
			clipWidth geom.Length
		}
		headerCells := make([]headerCell, len(tbl.Columns))
		headerLines := 0
		var headerVM verticalMetrics
		for i, col := range tbl.Columns {
			cg := geometry.Columns[i]
			if col.Label == "" {
				continue
			}
			if !hs.hasFontFamily {
				// Same failure mode a text element with no
				// style.fontFamily already has (R6, amended, and
				// fontChain's own error text): a non-empty label
				// needs a resolvable font, and no default exists
				// (Story 4.1's Delivery Log) — plain-wrapped, exactly
				// as fontChain's caller wraps it for a text element.
				return nil, nil, nil, fmt.Errorf("folio8: Render: element %s: has a column label but no style.fontFamily (nor headerStyle.fontFamily) to resolve a font from", el.ID)
			}

			segs, glyphDiags, serr := shapeSegments(string(col.ID), chain, styledChain, col.Label, fs, headerCache, breaksAreConsumed)
			if serr != nil {
				return nil, nil, nil, serr
			}
			diags = append(diags, glyphDiags...)
			vm, verr := chainVerticalModel(metricsChain, hs.fontSize, hs.lineSpacing, fs, headerCache)
			if verr != nil {
				// Located: the leading model knows the chain and the
				// resolved size but not which element declared them, so
				// the element id is attached here, as it already is at
				// this file's body/footer site below.
				return nil, nil, nil, fmt.Errorf("folio8: Render: element %s: %w", el.ID, verr)
			}
			// LINE METRICS ARE THE MAXIMUM ACROSS COLUMNS, never the last
			// column's: labels in different scripts resolve different faces,
			// and a Thai heading beside an English one must not be spaced by
			// whichever happened to be measured last (SPEC-table-rules §4).
			headerVM = maxVerticalMetrics(headerVM, vm)

			contentW := cg.Width - padLeft - padRight
			lines := packHeaderLabelLines(segs, col.Label, hs.fontSize, contentW)

			// THE SILENT CLIP IS RETIRED, NOT RELOCATED. What is left
			// after wrapping is residual overflow — a single run with no
			// break opportunity narrow enough — and it now reports
			// through the SAME DiagCodeTextClippedWidth a body cell has
			// always used, with the same message builder. There is no
			// longer a path in this file that clips and says nothing.
			overflow, overflows := detectWidthOverflow(string(col.ID), lines, contentW)
			if overflows {
				diags = append(diags, Diagnostic{
					Severity:  SeverityWarning,
					Code:      DiagCodeTextClippedWidth,
					ElementID: overflow.elementID,
					Message:   widthClipMessage("column", "content", overflow),
				})
			}

			if len(lines) > headerLines {
				headerLines = len(lines)
			}
			headerCells[i] = headerCell{
				lines: lines, segs: segs, align: columnHeaderAlign(hs.alignFallback, col),
				clip: overflows, clipX: cg.X + padLeft, clipWidth: contentW,
			}
		}

		// The header row is max(headerHeight, the packed labels' height +
		// padding). headerLines is 0 for a table whose every label is
		// empty — there is no packed block at all then, and the declared
		// height stands unmodified, which is what keeps a label-less
		// table byte-identical.
		headerRowHeight := tbl.HeaderHeight
		headerBlockHeight := geom.Length(0)
		if headerLines > 0 {
			headerBlockHeight = headerVM.FirstBaseline + geom.Length(int64(headerLines-1))*headerVM.Advance + headerVM.LastDescent
			// ⚠ THE FLOOR IS RAISED BY EXTRA LINES, NOT BY THE FIRST ONE,
			// AND THAT BOUNDARY IS DELIBERATE.
			//
			// SPEC-table-rules states both "the header row is
			// max(headerHeight, the packed label's height + padding)" AND
			// — in its frozen Boundaries block — that a document with no
			// `\n` in a label must render to the SAME PDF HASH, with the
			// golden corpus as the witness. Those two are in conflict on
			// the corpus that exists: the statement goldens declare
			// `headerHeight: 28` with 8pt labels and 8pt of padding each
			// side, whose ONE line already measures 28.88pt, so an
			// unconditional max moves all four signed-off goldens by
			// 0.88pt for a change that was supposed to be about WRAPPING.
			//
			// The frozen constraint wins, and the narrowing costs the
			// feature nothing: the whole point of the floor is that a
			// heading which NEEDS MORE THAN ONE LINE gets the room for
			// them. A single line that already overflows its declared
			// padding is today's behaviour, unchanged, and is a separate
			// complaint about a field the author set too small.
			if headerLines > 1 {
				if packed := padTop + headerBlockHeight + padBottom; packed > headerRowHeight {
					headerRowHeight = packed
				}
			}
		}
		tableBottom = tableTop + headerRowHeight

		for i := range tbl.Columns {
			cg := geometry.Columns[i]
			rect, rerr := buildHeaderCellRect(string(el.ID), cg.X, tableTop, cg.Width, headerRowHeight, hs)
			if rerr != nil {
				return nil, nil, nil, rerr
			}
			rects[i] = rect
		}

		if headerLines > 0 {
			headerInk, hasHeaderInk, inkErr := styleInk(hs.inkStyle, string(el.ID), hs.inkField)
			if inkErr != nil {
				return nil, nil, nil, inkErr
			}
			contentY := tableTop + padTop
			contentH := headerRowHeight - padTop - padBottom

			// The BLOCK's own vertical placement inside the padded cell,
			// unchanged in shape from the single-line version this
			// replaces — for a one-line header the block height IS
			// FirstBaseline+LastDescent, so the arithmetic is the same
			// integer it always was.
			var blockTopY geom.Length
			switch hs.valign {
			case "bottom":
				blockTopY = contentY + contentH - headerBlockHeight
			case "middle":
				blockTopY = contentY + geom.ScaleRound(contentH-headerBlockHeight, 1, 2)
			default: // "top"
				blockTopY = contentY
			}

			for li := 0; li < headerLines; li++ {
				lineTopY := blockTopY + geom.Length(int64(li))*headerVM.Advance
				for i := range tbl.Columns {
					hc := headerCells[i]
					if len(hc.lines) == 0 {
						continue
					}
					// A SHORTER CELL'S OWN SLACK, distributed by the same
					// whole-line rule a data row already uses (the body's
					// lineOffsets): a two-line heading beside a one-line
					// one puts the short label at the top, the middle or
					// the bottom of the row according to valign, never
					// half a line off it.
					slack := headerLines - len(hc.lines)
					offset := 0
					switch hs.valign {
					case "bottom":
						offset = slack
					case "middle":
						offset = slack / 2
					}
					cellLi := li - offset
					if cellLi < 0 || cellLi >= len(hc.lines) {
						continue
					}
					ln := hc.lines[cellLi]
					contentX := geometry.Columns[i].X + padLeft
					contentW := geometry.Columns[i].Width - padLeft - padRight

					var textX geom.Length
					switch hc.align {
					case "right":
						textX = contentX + contentW - ln.width
					case "center":
						textX = contentX + geom.ScaleRound(contentW-ln.width, 1, 2)
					// "left" is the header cell's start edge. Every other
					// value the load-time closed-set check already
					// rejected, and since Story 7.8 that includes
					// "justify": a TABLE's style.align and
					// headerStyle.align validate against
					// TableStyleAlignTokens.
					default:
						textX = contentX
					}

					placed, perr := positionSegments(hc.segs, ln.from, ln.to, textX, lineTopY, hs.fontSize, headerVM.FirstBaseline, nil)
					if perr != nil {
						return nil, nil, nil, perr
					}
					for j := range placed {
						placed[j].band = bandIndex
						placed[j].elementID = string(el.ID)
						// EVERY header line of one table shares lineIndex
						// 0, exactly as every column's label always has.
						// The header is ONE group (isHeaderRow /
						// isHeaderLabel), it moves whole, and its lines
						// share one extent — so merging them into one
						// ColumnItem is correct, and it is what keeps the
						// body's own nextLineIndex counter starting at 1
						// with nothing to collide with.
						placed[j].lineIndex = 0
						placed[j].itemTop = tableTop
						placed[j].itemBottom = tableBottom
						placed[j].isHeaderLabel = true
						if hasHeaderInk {
							placed[j].hasColor = true
							placed[j].color = headerInk
						}
						if hc.clip {
							placed[j].clipToBox = true
							placed[j].clipX = hc.clipX
							placed[j].clipWidth = hc.clipWidth
						}
					}
					runs = append(runs, placed...)
				}
			}
		}

		frame, frameErr := buildTableFrame(el, tbl, hs, geometry)
		if frameErr != nil {
			return nil, nil, nil, frameErr
		}

		rectSources = append(rectSources, tableRectSource{
			band:        bandIndex,
			elementID:   string(el.ID),
			top:         tableTop,
			bottom:      tableBottom,
			rects:       rects,
			isHeaderRow: true,
			frame:       frame,
		})

		// --- Story 4.2: data rows ---
		//
		// checkTableBindings (render.go) already ran, before this
		// function is ever called, and already proved tbl.Bind resolves
		// to a KindArray value (or failed the whole render) — so
		// data.Lookup below is safe to read .Arr from unconditionally.
		alias := resolvedRowAlias(tbl.As)
		collectionVal, _ := data.Lookup(tableCollectionSegments(tbl.Bind))
		items := collectionVal.Arr

		// Story 4.5, DECISION-3 (ruled by the engineering lead): a
		// footer row renders whenever at least one column declares
		// `footer` — including over an EMPTY bound collection (AC9),
		// since a footer is a property of the column's configuration,
		// not of the data.
		hasFooter := false
		for _, c := range tbl.Columns {
			if c.Footer.Set {
				hasFooter = true
				break
			}
		}

		if len(items) > 0 || hasFooter {
			// AC5: a data cell's font/border/background/padding/align/
			// valign cascade from the table's own `style` ONLY — never
			// `headerStyle` (D-000.76: header-only). fontChain is
			// reused VERBATIM (not re-implemented) so a data cell's
			// "no resolvable fontFamily" failure is the SAME message
			// the existing font-resolution failure produces, never a
			// third spelling (AC5's own grounds, D-000.65).
			bodyChain, bodyStyledChain, cerr := fontChain(doc, el)
			if cerr != nil {
				return nil, nil, nil, fmt.Errorf("folio8: Render: element %s: %w", el.ID, cerr)
			}
			// The BODY's chain, scoped for the same reason headerCache is.
			bodyCache := cache.forChain(el.Style.Value.FontFamily.Value)
			// Every face that may draw a body or footer cell, for the
			// same reason the header has one: the leading must never be
			// derived from a face the glyphs did not come from.
			bodyMetricsChain := metricsFaceNames(bodyChain, bodyStyledChain, fs, bodyCache)
			// AC3's Warning is one per (element, distinct rune), and a
			// table shapes a column ONCE PER ROW — so the memo has to
			// outlive the shapeSegments call. See coalesceStyleFaceDiags.
			var styleFaceSeen []Diagnostic
			bodyFontSize := defaultFontSizePt
			if el.Style.Set && !el.Style.Null && el.Style.Value.FontSize.Set && !el.Style.Value.FontSize.Null {
				bodyFontSize = el.Style.Value.FontSize.Value
			}
			bs := resolveBodyStyle(el)
			// Story 10.1: every data cell's ink, and the footer row's,
			// resolved ONCE per table from the same cascade bs already
			// carries — the discipline every other body-cell property
			// here follows (R2: cascade once, reuse verbatim).
			bodyInk, hasBodyInk, bodyInkErr := styleInk(bs.inkStyle, string(el.ID), "style.color")
			if bodyInkErr != nil {
				return nil, nil, nil, bodyInkErr
			}
			padTopB, padRightB, padBottomB, padLeftB := paddingEdges(bs.padding)

			// R2: ONE vertical model for the WHOLE table's body, computed
			// once outside the row loop — never recomputed per cell or
			// per row.
			// Read off el.Style directly, beside bodyFontSize and for the
			// same reason bs does not carry it: this ONE model serves the
			// body rows AND the footer row.
			vm, verr := chainVerticalModel(bodyMetricsChain, bodyFontSize, styleLineSpacing(el.Style), fs, bodyCache)
			if verr != nil {
				return nil, nil, nil, fmt.Errorf("folio8: Render: element %s: %w", el.ID, verr)
			}

			type cellResult struct {
				lines     []wrappedLine
				segs      []faceSegment
				align     string
				clip      bool
				clipX     geom.Length
				clipWidth geom.Length
			}

			rowTop := tableBottom
			// nextLineIndex starts at 1: the header's own label line
			// above always uses lineIndex 0 (unchanged), so this counter
			// — monotonically increasing across EVERY physical line of
			// EVERY row of this table — never collides with it and never
			// collides between two different rows, which is exactly what
			// keeps contentColumnItems/paginateDocument's
			// (elementID, lineIndex) grouping from merging two distinct
			// physical lines into one ColumnItem (D2).
			nextLineIndex := 1

			for rowIdx, rowVal := range items {
				scope := bind.NewScope(data, params).WithRow(rowVal, alias)

				cellResults := make([]cellResult, len(tbl.Columns))
				maxLines := 0
				for ci, col := range tbl.Columns {
					cg := geometry.Columns[ci]
					contentW := cg.Width - padLeftB - padRightB

					align := columnAlign(bs.alignFallback, col)

					// AC4: the column's bind resolves in the table's
					// ROW SCOPE — bind.Resolve, never bind.BindTextSpans
					// — so an unqualified path still resolves from the
					// document root (a row never shadows it) and
					// `params.` still resolves to the parameters,
					// shadowed by nothing (AD-11). AC4's own three AD-14
					// cases (absent -> Error, explicit null -> empty and
					// not an error, wrong kind -> Error never coerced;
					// since 2026-09-13 a number prints as its exact
					// decimal and only other wrong kinds stay Errors)
					// are ALL already implemented inside bind.Resolve
					// itself — nothing here re-implements any of them.
					//
					// Diagnostic located by COLUMN id (AC4's own
					// grounds: "columns[].id exists precisely so a
					// diagnostic can name a column") — the SAME
					// DiagCodeBindingPathAbsent code collectBandTextRuns
					// already uses for a text element's own unresolvable
					// binding (D-000.65: reuse, mint nothing).
					boundText, subs, caveats, berr := bind.Resolve(col.Bind, scope, fc, string(col.ID))
					if berr != nil {
						return nil, nil, nil, expressionRuntimeError(string(col.ID), "bind", fmt.Errorf("folio8: Render: %w", berr))
					}
					for _, c := range caveats {
						diags = append(diags, diagnosticFromCaveat(string(col.ID), c))
					}
					if boundText == "" {
						cellResults[ci] = cellResult{align: align}
						continue
					}

					segs, glyphDiags, serr := shapeSegments(string(col.ID), bodyChain, bodyStyledChain, boundText, fs, bodyCache, breaksAreConsumed)
					if serr != nil {
						return nil, nil, nil, serr
					}
					diags = coalesceStyleFaceDiags(diags, glyphDiags, &styleFaceSeen)
					totalRunes := len([]rune(boundText))

					atomic := atomicSpansFor(doc.doc.UnbreakableValues, subs)
					ops := text.Opportunities(text.Dictionary(), boundText, atomic)
					// AC2: wrap INSIDE the column's own content width —
					// packLines is the SAME packer text elements use.
					lines := packLines(segs, ops, totalRunes, bodyFontSize, contentW)

					// AC3: residual overflow (no break opportunity
					// narrow enough) is CLIPPED at the column's content
					// box, with the EXISTING DiagCodeTextClippedWidth —
					// D-2.8.1's own precedent, D-000.65: no new code.
					overflow, overflows := detectWidthOverflow(string(col.ID), lines, contentW)
					if overflows {
						diags = append(diags, Diagnostic{
							Severity:  SeverityWarning,
							Code:      DiagCodeTextClippedWidth,
							ElementID: overflow.elementID,
							Message:   widthClipMessage("column", "content", overflow),
						})
					}

					if len(lines) > maxLines {
						maxLines = len(lines)
					}
					cellResults[ci] = cellResult{
						lines: lines, segs: segs, align: align,
						clip: overflows, clipX: cg.X + padLeftB, clipWidth: contentW,
					}
				}

				// R3: a block of n lines is
				// FirstBaseline + (n-1)*Advance + LastDescent. n is at
				// least 1 even when every cell in the row is empty — a
				// data row, unlike an empty text element, is not itself
				// optional (it is one element of the bound collection),
				// so it still occupies one blank line's worth of height.
				linesInRow := maxLines
				if linesInRow < 1 {
					linesInRow = 1
				}
				rowHeight := padTopB + vm.FirstBaseline + geom.Length(int64(linesInRow-1))*vm.Advance + vm.LastDescent + padBottomB
				rowBottom := rowTop + rowHeight

				// AC5 (Story 4.2 review Finding 4): bs.valign
				// distributes a CELL's own vertical slack —
				// linesInRow minus that cell's own line count — WITHIN
				// the row, the body's analogue of the header's own
				// valign (R2/R3's shared vertical model: same three-
				// way switch, applied to a whole-line COUNT here
				// rather than a sub-line pixel remainder, since a
				// row's height is already an exact multiple of
				// vm.Advance). It never changes the row's own height
				// (still exactly linesInRow, computed above) or any
				// column's geometry (AD-13) — it only decides which of
				// the row's physical line SLOTS a shorter cell's own
				// lines occupy: "top" (the default, and the ONLY
				// behaviour before this fix) leaves them at the row's
				// first slots; "bottom" shifts them to the last
				// slots; "middle" splits the remainder, rounding down
				// (an integer LINE count, not a Length — no
				// geom.ScaleRound/binary-float concern, AD-1 holds).
				lineOffsets := make([]int, len(tbl.Columns))
				for ci := range tbl.Columns {
					slack := linesInRow - len(cellResults[ci].lines)
					if slack < 0 {
						slack = 0
					}
					switch bs.valign {
					case "bottom":
						lineOffsets[ci] = slack
					case "middle":
						lineOffsets[ci] = slack / 2
					default: // "top"
						lineOffsets[ci] = 0
					}
				}

				// DECISION-1 (ruled): a data row gets cell chrome too,
				// cascaded from `style` alone (never headerStyle,
				// never a data-driven decision — bs was resolved ONCE,
				// above, from the template alone). One tableRectSource
				// per ROW — reusing the EXACT same downstream handling
				// contentColumnItems/paginateDocument already give the
				// header's own tableRectSource, so a data row's chrome
				// becomes its own ColumnItem, spanning the row's full
				// extent (AC7), with no changes needed to either of
				// those two functions.
				cellRects := make([]pagemodel.Rect, len(tbl.Columns))
				// SPEC-table-rules §1: `table.altRowBackground` is now the
				// ONLY thing that can fill a data cell, and NOTHING can
				// stroke one. A row that is not an alternating row gets a
				// Rect with neither fill nor stroke — still built, because
				// D-2.6.5 requires an item that occupies space not to be
				// empty, and still the carrier of this row's pagination
				// identity.
				hasRowBackground := false
				rowBackground := ""
				rowBackgroundField := "table.altRowBackground"
				if rowIdx%2 == 1 && hasAltBackground {
					hasRowBackground = true
					rowBackground = altBackground
				}
				for ci := range tbl.Columns {
					cg := geometry.Columns[ci]
					rect, rerr := buildCellRectWithBackgroundField(string(el.ID), cg.X, rowTop, cg.Width, rowHeight,
						hasRowBackground, rowBackground, rowBackgroundField, false, template.Border{})
					if rerr != nil {
						return nil, nil, nil, rerr
					}
					cellRects[ci] = rect
				}
				rectSources = append(rectSources, tableRectSource{
					band:      bandIndex,
					elementID: string(el.ID),
					top:       rowTop,
					bottom:    rowBottom,
					rects:     cellRects,
					isDataRow: true,
					rowIndex:  rowIdx,
					frame:     frame,
				})

				// One physical line at a time: ALL columns' cell content
				// at line li share the SAME lineIndex (assigned once per
				// li, below) and the SAME vertical extent, exactly as
				// the header's several column labels already share
				// lineIndex 0 — this is what keeps a multi-column line
				// grouped into ONE ColumnItem (D2's "two items, same
				// extent" shape, extended to N columns on one physical
				// line rather than only two content kinds).
				for li := 0; li < linesInRow; li++ {
					lineTopY := rowTop + padTopB + geom.Length(int64(li))*vm.Advance
					lineBottom := lineTopY + vm.FirstBaseline + vm.LastDescent
					for ci := range tbl.Columns {
						cr := cellResults[ci]
						cellLi := li - lineOffsets[ci]
						if cellLi < 0 || cellLi >= len(cr.lines) {
							continue
						}
						ln := cr.lines[cellLi]
						cg := geometry.Columns[ci]
						contentX := cg.X + padLeftB
						contentW := cg.Width - padLeftB - padRightB
						measured := ln.width

						var textX geom.Length
						switch cr.align {
						case "right":
							textX = contentX + contentW - measured
						case "center":
							textX = contentX + geom.ScaleRound(contentW-measured, 1, 2)
						// "left" is the cell's start edge. Every other
						// value the load-time closed-set check already
						// rejected — including "justify" since Story
						// 7.8, which refuses it on a TABLE's own
						// style.align rather than letting it cascade
						// into a body cell through alignFallback. This
						// arm therefore catches only "left".
						default:
							textX = contentX
						}

						placed, poserr := positionSegments(cr.segs, ln.from, ln.to, textX, lineTopY, bodyFontSize, vm.FirstBaseline, nil)
						if poserr != nil {
							return nil, nil, nil, poserr
						}
						for j := range placed {
							placed[j].band = bandIndex
							placed[j].elementID = string(el.ID)
							placed[j].lineIndex = nextLineIndex
							placed[j].itemTop = lineTopY
							placed[j].itemBottom = lineBottom
							placed[j].isTableRowLine = true
							placed[j].rowIndex = rowIdx
							if hasBodyInk {
								placed[j].hasColor = true
								placed[j].color = bodyInk
							}
							if cr.clip {
								placed[j].clipToBox = true
								placed[j].clipX = cr.clipX
								placed[j].clipWidth = cr.clipWidth
							}
						}
						runs = append(runs, placed...)
					}
					nextLineIndex++
				}

				rowTop = rowBottom
			}

			// --- Story 4.5: footer row ---
			//
			// rowTop is already the bottom of the last data row (or, for
			// an empty collection, tableBottom unchanged — DECISION-3).
			// bs/bodyChain/bodyFontSize/vm/padding were all resolved
			// ONCE above (R2), before this story existed, and are
			// reused verbatim rather than re-cascaded for the footer.
			if hasFooter {
				// AC4/DW-7: no row scope — an aggregate's operand is a
				// collection path from the DOCUMENT ROOT (D-1.4.1),
				// exactly as an author-written {{sum(...)}} outside any
				// row context resolves; a footer is not one row.
				scope := bind.NewScope(data, params)

				cellResults := make([]cellResult, len(tbl.Columns))
				maxLines := 0
				for ci, col := range tbl.Columns {
					if !col.Footer.Set {
						// AC1: "a column that declares no footer carries
						// no value" — DECISION-3 gives it chrome only,
						// built unconditionally below.
						align := columnAlign(bs.alignFallback, col)
						cellResults[ci] = cellResult{align: align}
						continue
					}
					align := columnAlign(bs.alignFallback, col)

					exprText, exprErr := footerCellExprText(doc, tbl, col, len(items) == 0, scope, fc)
					if exprErr != nil {
						return nil, nil, nil, exprErr
					}

					boundText, subs, caveats, berr := bind.Resolve(exprText, scope, fc, string(col.ID))
					if berr != nil {
						// AD-14's existing wrong-kind Error path (R7/D-000.65):
						// a non-numeric value at footerOf surfaces here
						// exactly as any other expression type error
						// does — a plain wrapped error, never a new
						// diagnostic code, never a panic, never coerced.
						// (A footer value always passes through
						// formatNumber, so the number-in-text rule that
						// prints a bare number as its exact decimal never
						// reaches a footer cell.)
						return nil, nil, nil, fmt.Errorf("folio8: Render: column %s: footer: %w", col.ID, berr)
					}
					for _, c := range caveats {
						// AC9: avg() over an empty collection's Caveat
						// becomes the EXISTING DiagCodeEmptyAverage
						// Warning here, through the SAME
						// diagnosticFromCaveat every other aggregate
						// caveat already uses (D-000.65).
						diags = append(diags, diagnosticFromCaveat(string(col.ID), c))
					}
					if boundText == "" {
						cellResults[ci] = cellResult{align: align}
						continue
					}

					segs, glyphDiags, serr := shapeSegments(string(col.ID), bodyChain, bodyStyledChain, boundText, fs, cache, breaksAreConsumed)
					if serr != nil {
						return nil, nil, nil, serr
					}
					diags = coalesceStyleFaceDiags(diags, glyphDiags, &styleFaceSeen)
					totalRunes := len([]rune(boundText))

					atomic := atomicSpansFor(doc.doc.UnbreakableValues, subs)
					ops := text.Opportunities(text.Dictionary(), boundText, atomic)
					cg := geometry.Columns[ci]
					contentW := cg.Width - padLeftB - padRightB
					lines := packLines(segs, ops, totalRunes, bodyFontSize, contentW)

					overflow, overflows := detectWidthOverflow(string(col.ID), lines, contentW)
					if overflows {
						diags = append(diags, Diagnostic{
							Severity:  SeverityWarning,
							Code:      DiagCodeTextClippedWidth,
							ElementID: overflow.elementID,
							Message:   widthClipMessage("column", "content", overflow),
						})
					}
					if len(lines) > maxLines {
						maxLines = len(lines)
					}
					cellResults[ci] = cellResult{
						lines: lines, segs: segs, align: align,
						clip: overflows, clipX: cg.X + padLeftB, clipWidth: contentW,
					}
				}

				linesInRow := maxLines
				if linesInRow < 1 {
					linesInRow = 1
				}
				footerRowHeight := padTopB + vm.FirstBaseline + geom.Length(int64(linesInRow-1))*vm.Advance + vm.LastDescent + padBottomB
				footerRowBottom := rowTop + footerRowHeight

				// DECISION-3 (ruled): chrome for EVERY column,
				// unconditionally — mirrors a data row's own chrome
				// (buildCellRect above), never conditioned on whether
				// that column itself declares a footer, so a bordered
				// table's footer row never draws a broken-looking
				// partial row.
				footerCellRects := make([]pagemodel.Rect, len(tbl.Columns))
				for ci := range tbl.Columns {
					cg := geometry.Columns[ci]
					// SPEC-table-rules §1: no chrome from the element's own
					// style, exactly as a data row's cells above. The
					// footer keeps its own rect so it keeps its own
					// pagination identity (isFooterRow, below).
					rect, rerr := buildCellRect(string(el.ID), cg.X, rowTop, cg.Width, footerRowHeight, false, "", false, template.Border{})
					if rerr != nil {
						return nil, nil, nil, rerr
					}
					footerCellRects[ci] = rect
				}
				rectSources = append(rectSources, tableRectSource{
					band:        bandIndex,
					elementID:   string(el.ID),
					top:         rowTop,
					bottom:      footerRowBottom,
					rects:       footerCellRects,
					isFooterRow: true,
					frame:       frame,
				})

				for li := 0; li < linesInRow; li++ {
					lineTopY := rowTop + padTopB + geom.Length(int64(li))*vm.Advance
					lineBottom := lineTopY + vm.FirstBaseline + vm.LastDescent
					for ci := range tbl.Columns {
						cr := cellResults[ci]
						if li >= len(cr.lines) {
							continue
						}
						ln := cr.lines[li]
						cg := geometry.Columns[ci]
						contentX := cg.X + padLeftB
						contentW := cg.Width - padLeftB - padRightB
						measured := ln.width

						var textX geom.Length
						switch cr.align {
						case "right":
							textX = contentX + contentW - measured
						case "center":
							textX = contentX + geom.ScaleRound(contentW-measured, 1, 2)
						// "left" is the cell's start edge. Every other
						// value the load-time closed-set check already
						// rejected — including "justify" since Story
						// 7.8, which refuses it on a TABLE's own
						// style.align rather than letting it cascade
						// into a footer cell through alignFallback. This
						// arm therefore catches only "left".
						default:
							textX = contentX
						}

						placed, poserr := positionSegments(cr.segs, ln.from, ln.to, textX, lineTopY, bodyFontSize, vm.FirstBaseline, nil)
						if poserr != nil {
							return nil, nil, nil, poserr
						}
						for j := range placed {
							placed[j].band = bandIndex
							placed[j].elementID = string(el.ID)
							placed[j].lineIndex = nextLineIndex
							placed[j].itemTop = lineTopY
							placed[j].itemBottom = lineBottom
							placed[j].isFooterLine = true
							if hasBodyInk {
								placed[j].hasColor = true
								placed[j].color = bodyInk
							}
							if cr.clip {
								placed[j].clipToBox = true
								placed[j].clipX = cr.clipX
								placed[j].clipWidth = cr.clipWidth
							}
						}
						runs = append(runs, placed...)
					}
					nextLineIndex++
				}
			}
		}

	}

	return runs, rectSources, diags, nil
}

// packHeaderLabelLines lays out one column label through the BODY CELL'S
// PACKER (SPEC-table-rules §4): its line feeds are consumed as mandatory
// breaks and it wraps against the column's content width, exactly as a data
// cell does. It is the one header packer — the rendered header and the
// canvas projection (addCanvasTableLabelLines) both call it, so the canvas
// paints the engine's lines rather than deciding its own.
//
// A LABEL IS LITERAL TEXT, so it carries no substitutions and therefore no
// atomic spans: `unbreakableValues` names DATA paths, and no data reaches a
// label.
//
// contentW <= 0 (a column narrower than its own padding) packs mandatory
// breaks only, never one rune per line; it is clamped so the guard is this
// function's and not an accident of packLines'.
func packHeaderLabelLines(segs []faceSegment, label string, fontSize, contentW geom.Length) []wrappedLine {
	if contentW < 0 {
		contentW = 0
	}
	ops := text.Opportunities(text.Dictionary(), label, nil)
	return packLines(segs, ops, len([]rune(label)), fontSize, contentW)
}
