package folio8

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/panitw/folio8/folio8-go/internal/designer"
	"github.com/panitw/folio8/folio8-go/internal/geom"
	"github.com/panitw/folio8/folio8-go/internal/layout"
	"github.com/panitw/folio8/folio8-go/internal/template"
)

// This file is the engine half of spec-section-break (CAP-2 to CAP-5): the
// content band's optional `sectionBreak`, the load checks it needs, the
// keepTogether split it forces, and the section-aware pagination both of
// buildPageModel's passes run.
//
// THE RULE. Elements declared at or below the break form the section. The
// items above it are paginated on their own first. If they end at or above
// the line on their last page, the section is drawn on that page; otherwise
// on a page added after it. Either way every section element sits at its
// declared y, so the section's first page carries its own shift (zero), and
// a section that does not fit continues under the ordinary rules.
//
// UNANCHORED (CAP-7, `sectionBreakAnchor: false`). When the content above ends
// below the line, the section is instead moved as one block by D before it is
// paginated: down by E - line on that same page when its extent still fits,
// otherwise up so the line is the top of a new page's window. When the content
// above ends at or above the line it is exactly the anchored rule.
//
// THE BYTE-IDENTITY FAST PATH. When the above-line items fit one page and end
// at or above the line, the document is paginated exactly as it is without
// the key — one layout pass over every item — so its PDF is unchanged (CAP-3).

// sectionBreakDataPath locates a band-level section-break diagnostic.
const sectionBreakDataPath = "bands.content.sectionBreak"

// sectionBreakAnchorDataPath locates a diagnostic about the break's Anchor.
const sectionBreakAnchorDataPath = "bands.content.sectionBreakAnchor"

// sectionBreakPath locates a diagnostic about page's key: under
// `bands.content` for a one-page document, `pages[i]` otherwise
// (SPEC-multi-pages CAP-6: every designed page holds its own break).
func sectionBreakPath(t *Template, page int, key string) string {
	return t.doc.PageField(page) + "." + key
}

// contentBandOf is designed page's content band.
func contentBandOf(t *Template, page int) *template.Band {
	return t.doc.ContentBands()[page]
}

// sectionBreakSplit is one document's section break, resolved for pagination.
// The zero value is "no break", and paginates exactly as before.
type sectionBreakSplit struct {
	// line is the break in page space: the content origin plus the offset.
	line geom.Length
	// members holds the ids of the content-band elements declared at or below
	// the break. Looked up, never ranged.
	members map[string]bool
	// unanchored is spec-section-break CAP-7: `sectionBreakAnchor: false`.
	unanchored bool
}

// sectionBreakOf resolves designed page's section break against g. A page
// without a break, or whose section is empty, gets the zero value.
func sectionBreakOf(t *Template, g layout.PageGeometry, page int) sectionBreakSplit {
	offset, ok := declaredSectionBreak(t, page)
	if !ok {
		return sectionBreakSplit{}
	}
	out := sectionBreakSplit{line: layout.Origins(g).Content + offset, unanchored: !sectionBreakAnchored(t, page)}
	for _, el := range contentBandOf(t, page).Elements {
		if el.Y < offset {
			continue
		}
		if out.members == nil {
			out.members = map[string]bool{}
		}
		out.members[string(el.ID)] = true
	}
	return out
}

// declaredSectionBreak returns designed page's break offset, if it has one.
func declaredSectionBreak(t *Template, page int) (geom.Length, bool) {
	if t == nil || t.doc == nil || page < 0 || page >= t.doc.PageCount() {
		return 0, false
	}
	sb := contentBandOf(t, page).SectionBreak
	if !sb.Set || sb.Null {
		return 0, false
	}
	return sb.Value, true
}

// sectionBreakAnchored reports designed page's break Anchor setting (CAP-7):
// true unless the page declares `sectionBreakAnchor: false`.
func sectionBreakAnchored(t *Template, page int) bool {
	if t == nil || t.doc == nil || page < 0 || page >= t.doc.PageCount() {
		return true
	}
	a := contentBandOf(t, page).SectionBreakAnchor
	return !a.Set || a.Null || a.Value
}

// validateSectionBreak refuses, at load, a break a page's content band cannot
// honour (SECTION_BREAK_INVALID, located at the page's key) and an element
// whose declared box lies on both sides of its own page's break
// (SECTION_BREAK_STRADDLED, located at the element). Every designed page is
// checked against its own elements, in page order. It lives here, not in
// internal/template, because the range check needs layout.ContentHeight —
// validateTableMinHeights' reason.
func validateSectionBreak(t *Template) error {
	if t == nil || t.doc == nil {
		return nil
	}
	for page := range t.doc.ContentBands() {
		if err := validatePageSectionBreak(t, page); err != nil {
			return err
		}
	}
	return nil
}

// validatePageSectionBreak is validateSectionBreak for one designed page.
func validatePageSectionBreak(t *Template, page int) error {
	band := contentBandOf(t, page)
	breakPath := sectionBreakPath(t, page, "sectionBreak")
	offset, ok := declaredSectionBreak(t, page)
	if !ok {
		// The parser already refuses an Anchor with no break; this is the
		// same rule for a template that reached here some other way.
		if band.SectionBreakAnchor.Set {
			anchorPath := sectionBreakPath(t, page, "sectionBreakAnchor")
			return newRenderError(DiagCodeSectionBreakInvalid, "", anchorPath,
				fmt.Errorf("folio8: %s: declared without a sectionBreak — the Anchor setting qualifies a section break; add the break or remove this key", anchorPath))
		}
		return nil
	}
	if offset <= 0 {
		return newRenderError(DiagCodeSectionBreakInvalid, "", breakPath,
			fmt.Errorf("folio8: %s: %spt is at or above the content band's top — a section break must lie inside the content band, greater than 0", breakPath, template.FormatPoints(offset)))
	}
	if g, err := pageGeometryOf(t); err == nil {
		if height := layout.ContentHeight(g); offset >= height {
			return newRenderError(DiagCodeSectionBreakInvalid, "", breakPath,
				fmt.Errorf("folio8: %s: %spt is at or below the bottom of the content band (a content height of %spt) — move the break up, or give the content band more room", breakPath, template.FormatPoints(offset), template.FormatPoints(height)))
		}
	}
	for _, el := range band.Elements {
		top, bottom := sectionBreakDeclaredBox(el)
		if top < offset && bottom > offset {
			// A one-page document keeps today's located result; a later
			// page's straddle also names the page's break path.
			dataPath, where := "", ""
			if t.doc.PageCount() > 1 {
				dataPath, where = breakPath, " ("+breakPath+")"
			}
			return newRenderError(DiagCodeSectionBreakStraddled, string(el.ID), dataPath,
				fmt.Errorf("folio8: element %s: its declared box runs from %spt to %spt, across the section break at %spt%s — every element must lie wholly above or wholly below the break; move or resize the element, or move the break", el.ID, template.FormatPoints(top), template.FormatPoints(bottom), template.FormatPoints(offset), where))
		}
	}
	return nil
}

// sectionBreakDeclaredBox is the vertical extent of el's declared box, the
// extent the straddle rule reads. A table's box is its header row — its rows
// and its minHeight floor may run past the line. An element with no usable
// declared height is a point at its y.
func sectionBreakDeclaredBox(el template.Element) (top, bottom geom.Length) {
	top, bottom = el.Y, el.Y
	if el.Type == template.ElementTable {
		if el.Table.Set && !el.Table.Null {
			bottom = el.Y + el.Table.Value.HeaderHeight
		}
		return top, bottom
	}
	if el.Height.Set && !el.Height.Null && el.Height.Value > 0 {
		bottom = el.Y + el.Height.Value
	}
	return top, bottom
}

// ---------------------------------------------------------------------------
// spec-section-break CAP-1 / CAP-6: THE DESIGNER'S COMMANDS AND REFUSALS.
//
// In the file the break stays the content band's `sectionBreak` key; on the
// canvas it is placed, dragged, nudged, typed and deleted like an element.
// Every one of those gestures is ONE command below, so each is one undo entry.
// Every geometry command that could leave an element across the line asks
// refuseSectionBreakStraddle before it installs anything, so a refused edit
// names the element in the way and leaves the document unchanged.

// sectionBreakStraddles reports whether el's declared box would lie on both
// sides of a break at offset — the load rule, asked of a candidate.
func sectionBreakStraddles(el template.Element, offset geom.Length) bool {
	top, bottom := sectionBreakDeclaredBox(el)
	return top < offset && bottom > offset
}

// refuseSectionBreakStraddle refuses a candidate content-band element whose
// declared box would lie across its own page's break. It is a no-op for a
// page without a break and for an element of any other band.
func refuseSectionBreakStraddle(t *Template, bandName string, candidate template.Element, path string) error {
	if bandName != bandContent {
		return nil
	}
	// SPEC-multi-pages: a page's break constrains only that page's elements.
	// An id not yet in the index is a new element, created into page 1.
	return refuseSectionBreakStraddleOnPage(t, bandName, contentPageIndex(t)[string(candidate.ID)], candidate, path)
}

// refuseSectionBreakStraddleOnPage is refuseSectionBreakStraddle for a
// candidate that will sit on `page` — a create onto a page, or a move to one
// (story 3). The page it lands on is the page whose break judges it.
func refuseSectionBreakStraddleOnPage(t *Template, bandName string, page int, candidate template.Element, path string) error {
	if bandName != bandContent {
		return nil
	}
	offset, ok := declaredSectionBreak(t, page)
	if !ok || !sectionBreakStraddles(candidate, offset) {
		return nil
	}
	top, bottom := sectionBreakDeclaredBox(candidate)
	return componentFailure(string(candidate.ID), path, fmt.Sprintf("%s would run from %spt to %spt, across the section break at %spt%s — every element must lie wholly above or wholly below the break; move or resize the element, or move the break", candidate.ID, template.FormatPoints(top), template.FormatPoints(bottom), template.FormatPoints(offset), onPageSuffix(t, page)))
}

// onPageSuffix names a page in a refusal sentence, only when the document has
// more than one, so a one-page document's sentences are unchanged.
func onPageSuffix(t *Template, page int) string {
	if t.doc.PageCount() < 2 {
		return ""
	}
	return fmt.Sprintf(" on page %d", page+1)
}

// refuseSectionBreakBeyondContent refuses a page or band change that leaves
// any page's break at or below the bottom of the content band. The refusal
// names the first such break, because the break is what no longer fits.
func refuseSectionBreakBeyondContent(t *Template) error {
	g, err := canvasPageGeometry(t)
	if err != nil {
		return nil
	}
	height := layout.ContentHeight(g)
	for page := range t.doc.ContentBands() {
		offset, ok := declaredSectionBreak(t, page)
		if !ok || offset < height {
			continue
		}
		return componentFailure("", sectionBreakPath(t, page, "sectionBreak"), fmt.Sprintf("this leaves a content band %spt tall, and the section break%s at %spt would lie at or below its bottom — move the section break up first", template.FormatPoints(height), onPageSuffix(t, page), template.FormatPoints(offset)))
	}
	return nil
}

// sectionBreakCommandPage reads a section-break command's optional 0-based
// `page` (SPEC-multi-pages story 5). Absent, the command is page 1's with
// today's field count, so `fields` counts it only when given. A wrong field
// count is refused at page 1's key with usage; an unknown page at `pages`.
func sectionBreakCommandPage(t *Template, raw map[string]json.RawMessage, fields int, key, usage string) (int, error) {
	_, hasPage := raw["page"]
	if hasPage {
		fields++
	}
	if err := componentFields(raw, fields); err != nil {
		return 0, componentFailure("", sectionBreakPath(t, 0, key), usage)
	}
	if !hasPage {
		return 0, nil
	}
	return pageIndexField(raw, "page", t.doc.PageCount())
}

// setSectionBreak places or moves a page's break: {kind, version, offset,
// snap, page?}. The engine snaps (the canvas drag passes true; a typed Y
// passes false), and snapping happens before every check, so a refusal names
// the offset that would actually have been written.
func setSectionBreak(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	page, err := sectionBreakCommandPage(t, raw, 4, "sectionBreak", "setSectionBreak takes exactly kind, version, offset and snap, and optionally page")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	path := sectionBreakPath(t, page, "sectionBreak")
	proposed, err := lengthField(raw, "offset")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", path, err.Error())
	}
	snap, err := commandBool(raw, "snap")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", path, err.Error())
	}
	if snap {
		snapped, valid := snapToGrid(proposed)
		if !valid {
			return designer.CanvasProjection{}, componentFailure("", path, "the section break offset overflows grid snapping")
		}
		proposed = snapped
	}
	if proposed <= 0 {
		return designer.CanvasProjection{}, componentFailure("", path, fmt.Sprintf("a section break at %spt is at or above the content band's top — it must lie inside the content band", template.FormatPoints(proposed)))
	}
	g, err := canvasPageGeometry(t)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	if height := layout.ContentHeight(g); proposed >= height {
		return designer.CanvasProjection{}, componentFailure("", path, fmt.Sprintf("a section break at %spt is at or below the bottom of the content band (a content height of %spt) — move it up", template.FormatPoints(proposed), template.FormatPoints(height)))
	}
	band := contentBandOf(t, page)
	for _, el := range band.Elements {
		if sectionBreakStraddles(el, proposed) {
			top, bottom := sectionBreakDeclaredBox(el)
			return designer.CanvasProjection{}, componentFailure(string(el.ID), path, fmt.Sprintf("a section break at %spt would run through %s, which runs from %spt to %spt — every element must lie wholly above or wholly below the break", template.FormatPoints(proposed), el.ID, template.FormatPoints(top), template.FormatPoints(bottom)))
		}
	}
	previous := band.SectionBreak
	band.SectionBreak = template.Presence[geom.Length]{Set: true, Value: proposed}
	projection, err := canvas(t)
	if err != nil {
		band.SectionBreak = previous
		return designer.CanvasProjection{}, err
	}
	return projection, nil
}

// removeSectionBreak deletes a page's break: {kind, version, page?}. "No
// break" is the key's absence, so it is cleared to the zero Presence, never to
// null.
func removeSectionBreak(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	page, err := sectionBreakCommandPage(t, raw, 2, "sectionBreak", "removeSectionBreak takes exactly kind and version, and optionally page")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	band := contentBandOf(t, page)
	if !band.SectionBreak.Set {
		message := "this document has no section break to remove"
		if t.doc.PageCount() > 1 {
			message = fmt.Sprintf("page %d has no section break to remove", page+1)
		}
		return designer.CanvasProjection{}, componentFailure("", sectionBreakPath(t, page, "sectionBreak"), message)
	}
	previous, previousAnchor := band.SectionBreak, band.SectionBreakAnchor
	band.SectionBreak = template.Presence[geom.Length]{}
	// The Anchor qualifies the break, so it goes with it (CAP-7).
	band.SectionBreakAnchor = template.Presence[bool]{}
	projection, err := canvas(t)
	if err != nil {
		band.SectionBreak, band.SectionBreakAnchor = previous, previousAnchor
		return designer.CanvasProjection{}, err
	}
	return projection, nil
}

// setSectionBreakAnchor sets a page's break Anchor: {kind, version, anchor,
// page?}. One command, so one undo entry (CAP-7). Anchored is the default and
// the key's absence, so `true` clears the key and `false` writes it.
func setSectionBreakAnchor(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	page, err := sectionBreakCommandPage(t, raw, 3, "sectionBreakAnchor", "setSectionBreakAnchor takes exactly kind, version and anchor, and optionally page")
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	path := sectionBreakPath(t, page, "sectionBreakAnchor")
	// json.Unmarshal leaves a bool untouched for null, so null is refused
	// here rather than read as false.
	if value, ok := raw["anchor"]; ok && string(bytes.TrimSpace(value)) == "null" {
		return designer.CanvasProjection{}, componentFailure("", path, "anchor must be a boolean")
	}
	anchor, err := commandBool(raw, "anchor")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", path, err.Error())
	}
	if _, ok := declaredSectionBreak(t, page); !ok {
		message := "this document has no section break to anchor — add a Section Break first"
		if t.doc.PageCount() > 1 {
			message = fmt.Sprintf("page %d has no section break to anchor — add a Section Break first", page+1)
		}
		return designer.CanvasProjection{}, componentFailure("", path, message)
	}
	band := contentBandOf(t, page)
	previous := band.SectionBreakAnchor
	if anchor {
		band.SectionBreakAnchor = template.Presence[bool]{}
	} else {
		band.SectionBreakAnchor = template.Presence[bool]{Set: true, Value: false}
	}
	projection, err := canvas(t)
	if err != nil {
		band.SectionBreakAnchor = previous
		return designer.CanvasProjection{}, err
	}
	return projection, nil
}

// sectionBreakSplitTags returns, in page order and then authored order of
// first appearance, the keepTogether tags whose members lie on both sides of
// their own page's break, with the first member of each. A tag never spans
// pages (the loader refuses it), so each page is judged alone.
func sectionBreakSplitTags(t *Template) (tags []string, firstMember map[string]string) {
	for page := range t.doc.ContentBands() {
		pageTags, pageFirst := pageSectionBreakSplitTags(t, page)
		if len(pageTags) == 0 {
			continue
		}
		if firstMember == nil {
			firstMember = map[string]string{}
		}
		for _, tag := range pageTags {
			tags = append(tags, tag)
			firstMember[tag] = pageFirst[tag]
		}
	}
	return tags, firstMember
}

// pageSectionBreakSplitTags is sectionBreakSplitTags for one designed page.
func pageSectionBreakSplitTags(t *Template, page int) (tags []string, firstMember map[string]string) {
	offset, ok := declaredSectionBreak(t, page)
	if !ok {
		return nil, nil
	}
	above := map[string]bool{}
	below := map[string]bool{}
	firstMember = map[string]string{}
	var order []string
	for _, el := range contentBandOf(t, page).Elements {
		if !el.KeepTogether.Set || el.KeepTogether.Null || el.KeepTogether.Value == "" {
			continue
		}
		tag := el.KeepTogether.Value
		if _, seen := firstMember[tag]; !seen {
			firstMember[tag] = string(el.ID)
			order = append(order, tag)
		}
		if el.Y >= offset {
			below[tag] = true
		} else {
			above[tag] = true
		}
	}
	for _, tag := range order {
		if above[tag] && below[tag] {
			tags = append(tags, tag)
		}
	}
	return tags, firstMember
}

// sectionBreakSplitDiagnostics is one SECTION_BREAK_SPLITS_KEEP_TOGETHER
// Warning per split group, located at the group's first member. Emitted on
// every render, whether or not the section moves, because the grouping the
// author declared is not the grouping that is honoured.
func sectionBreakSplitDiagnostics(t *Template) []Diagnostic {
	tags, first := sectionBreakSplitTags(t)
	var out []Diagnostic
	for _, tag := range tags {
		out = append(out, Diagnostic{
			Severity:  SeverityWarning,
			Code:      DiagCodeSectionBreakSplitsKeepTogether,
			ElementID: first[tag],
			Message: fmt.Sprintf("folio8: Render: element %s: keepTogether group %q has members on both sides of the section break, so it is split at the line — the members above the break are kept together where they are, and the members below it are kept together with the section. Move the group's members to one side of the break, or move the break, to keep the whole group together",
				first[tag], tag),
		})
	}
	return out
}

// sectionPlan is a pagination with the section break applied. Pages above
// the break shift by Pages[p].Shift; section elements shift by
// sectionShift[p]. With no break section is nil and every element shifts by
// Pages[p].Shift, exactly as before.
type sectionPlan struct {
	layout.Pagination
	section      map[string]bool
	sectionShift []geom.Length
}

// shiftFor is the window shift for elementID's content on page.
func (p sectionPlan) shiftFor(page int, elementID string) geom.Length {
	if p.section[elementID] {
		return p.sectionShift[page]
	}
	return p.Pages[page].Shift
}

// paginateWithSectionBreak is the ONE pagination both of buildPageModel's
// passes run, so the page count {{pages}} prints and the pages rendered
// always agree. With no break, or an empty section, it is exactly
// paginateWithFooterOrphanFix over every item.
func paginateWithSectionBreak(g layout.PageGeometry, items []layout.ColumnItem, sb sectionBreakSplit) (sectionPlan, []Diagnostic, error) {
	var above, below []layout.ColumnItem
	if len(sb.members) > 0 {
		for _, it := range items {
			if sb.members[it.ElementID] {
				below = append(below, it)
			} else {
				above = append(above, it)
			}
		}
	}
	if len(below) == 0 {
		plan, diags, err := paginateWithFooterOrphanFix(g, items, footerOrphanTargetsFrom(items))
		return sectionPlan{Pagination: plan}, diags, err
	}

	planA, pagesA, diagsA, err := paginateWithFooterOrphanFixPages(g, above, footerOrphanTargetsFrom(above))
	if err != nil {
		return sectionPlan{}, nil, err
	}
	shared := aboveLineEndsAtOrAbove(g, above, planA, pagesA, sb.line)
	if shared && len(planA.Pages) == 1 {
		// CAP-3's fast path: nothing moves, so paginate exactly as the
		// document without the key does.
		plan, diags, err := paginateWithFooterOrphanFix(g, items, footerOrphanTargetsFrom(items))
		return sectionPlan{Pagination: plan}, diags, err
	}

	planS, pagesS, diagsS, err := paginateWithFooterOrphanFixPages(g, below, footerOrphanTargetsFrom(below))
	if err != nil {
		return sectionPlan{}, nil, err
	}

	// spec-section-break CAP-7: an unanchored section moves by D, a
	// translation applied to the section's items BEFORE they are paginated,
	// so the window rules run on them unchanged. It moves when the above-line
	// content ends below the line, or when that content reaches a page after
	// the first — there the line follows E whether E is below or above it.
	// Pushed onto E's page when its extent fits under E; otherwise lifted so
	// the line is the next window's top. Page 1 content ending at or above
	// the line took the fast path above and is never pulled up.
	var d geom.Length
	if sb.unanchored {
		origins := layout.Origins(g)
		d = origins.Content - sb.line
		shared = false
		if end, clipped := aboveLineEnd(g, above, planA, pagesA, sb.line); !clipped {
			if extent, ok := sectionExtent(g, below, planS, pagesS, sb.line); ok && end+(extent-sb.line) <= origins.PageFooter {
				d = end - sb.line
				shared = true
			}
		}
		moved := make([]layout.ColumnItem, len(below))
		for i, it := range below {
			it.Top += d
			it.Bottom += d
			moved[i] = it
		}
		planS, _, diagsS, err = paginateWithFooterOrphanFixPages(g, moved, footerOrphanTargetsFrom(moved))
		if err != nil {
			return sectionPlan{}, nil, err
		}
		unshiftSectionPages(planS.Pages, d)
	}

	pages := append([]layout.PageAssignment(nil), planA.Pages...)
	shifts := make([]geom.Length, len(pages))
	for i := range pages {
		shifts[i] = pages[i].Shift
	}
	offset := len(pages)
	first := 0
	if shared {
		offset = len(pages) - 1
		pages[offset] = mergePageAssignments(pages[offset], planS.Pages[0])
		shifts[offset] = planS.Pages[0].Shift - d
		first = 1
	}
	for _, pa := range planS.Pages[first:] {
		pages = append(pages, pa)
		shifts = append(shifts, pa.Shift-d)
	}

	out := sectionPlan{
		Pagination: layout.Pagination{
			Pages:      pages,
			Suppressed: append([]layout.TableHeaderSuppressed(nil), planA.Suppressed...),
			Clipped:    append([]layout.TableRowClipped(nil), planA.Clipped...),
		},
		section:      sb.members,
		sectionShift: shifts,
	}
	for _, s := range planS.Suppressed {
		s.Page += offset
		out.Suppressed = append(out.Suppressed, s)
	}
	for _, c := range planS.Clipped {
		c.Page += offset
		out.Clipped = append(out.Clipped, c)
	}
	return out, append(diagsA, diagsS...), nil
}

// aboveLineEndsAtOrAbove reports whether the above-line content ends at or
// above line on its last page: the lowest page-space bottom of any item on
// that page, counting a table row's displacement, a floor push and a table's
// floored slice, is at or above the line. A group clipped on that page ran
// past the content bottom, so it crosses.
func aboveLineEndsAtOrAbove(g layout.PageGeometry, items []layout.ColumnItem, plan layout.Pagination, itemPages []int, line geom.Length) bool {
	end, clipped := aboveLineEnd(g, items, plan, itemPages, line)
	return !clipped && end <= line
}

// sectionExtent is the section's declared extent, floor-aware: the lowest
// page-space bottom of its items when paginated from their declared offset,
// counting a table's minHeight floor, a floor push and a row displacement —
// the same measure aboveLineEnd takes of the content above. It is not the
// straddle rule's declared box (sectionBreakDeclaredBox), which stops at a
// table's header. ok is false when the section does not fit one window even
// at its declared offset, so it can fit under no pushed position either.
func sectionExtent(g layout.PageGeometry, items []layout.ColumnItem, plan layout.Pagination, itemPages []int, line geom.Length) (geom.Length, bool) {
	if len(plan.Pages) != 1 {
		return 0, false
	}
	end, clipped := aboveLineEnd(g, items, plan, itemPages, line)
	return end, !clipped
}

// unshiftSectionPages restates, for an unanchored section paginated after
// being moved by d, the two per-page quantities that the render applies to
// the items' UNMOVED coordinates: a repeated header's own Shift and a clipped
// rect's column-space bottom. TableSlices are page space and already final.
// With d zero nothing changes.
func unshiftSectionPages(pages []layout.PageAssignment, d geom.Length) {
	if d == 0 {
		return
	}
	for p := range pages {
		if len(pages[p].HeaderRepeats) > 0 {
			repeats := append([]layout.TableHeaderRepeat(nil), pages[p].HeaderRepeats...)
			for i := range repeats {
				repeats[i].Shift -= d
			}
			pages[p].HeaderRepeats = repeats
		}
		if len(pages[p].ClippedRects) > 0 {
			clips := append([]layout.RectClip(nil), pages[p].ClippedRects...)
			for i := range clips {
				clips[i].Bottom -= d
			}
			pages[p].ClippedRects = clips
		}
	}
}

// aboveLineEnd is where content ends on its last page, in page space, and
// whether a group was clipped there — in which case it ran past the content
// bottom and has no usable end.
func aboveLineEnd(g layout.PageGeometry, items []layout.ColumnItem, plan layout.Pagination, itemPages []int, line geom.Length) (geom.Length, bool) {
	last := len(plan.Pages) - 1
	for _, c := range plan.Clipped {
		if c.Page == last {
			return 0, true
		}
	}
	pa := plan.Pages[last]
	end := layout.Origins(g).Content
	for i, it := range items {
		if itemPages[i] != last {
			continue
		}
		bottom := it.Bottom - pa.Shift + elementPushFor(pa.ElementPush, it.ElementID)
		if it.Group.Present && !it.Group.Key.IsHeader && !strings.HasPrefix(it.Group.Key.ElementID, keepTogetherKeyPrefix) {
			bottom += rowDisplacementFor(pa.RowDisplacement, it.ElementID)
		}
		if bottom > end {
			end = bottom
		}
	}
	for _, sl := range pa.TableSlices {
		if sl.Bottom > end {
			end = sl.Bottom
		}
	}
	return end, false
}

// mergePageAssignments puts the section's first page onto the above-line
// content's last page. Content refs are merged in ascending order, which is
// authored emission order (both passes build items so that refs ascend within
// each kind). Every other per-page list is keyed by element or carries its
// own shift, so it is concatenated.
func mergePageAssignments(a, s layout.PageAssignment) layout.PageAssignment {
	out := a
	out.ContentRuns = mergeAscending(a.ContentRuns, s.ContentRuns)
	out.ContentImages = mergeAscending(a.ContentImages, s.ContentImages)
	out.ContentRects = mergeAscending(a.ContentRects, s.ContentRects)
	out.HeaderRepeats = append(append([]layout.TableHeaderRepeat(nil), a.HeaderRepeats...), s.HeaderRepeats...)
	out.RowDisplacement = append(append([]layout.TableRowDisplacement(nil), a.RowDisplacement...), s.RowDisplacement...)
	out.ClippedRects = append(append([]layout.RectClip(nil), a.ClippedRects...), s.ClippedRects...)
	out.TableSlices = append(append([]layout.TableSlice(nil), a.TableSlices...), s.TableSlices...)
	out.ElementPush = append(append([]layout.ElementPush(nil), a.ElementPush...), s.ElementPush...)
	return out
}

// mergeAscending merges two ascending ref lists into one ascending list. On a
// tie the above-line ref goes first.
func mergeAscending[T ~int](a, b []T) []T {
	if len(b) == 0 {
		return a
	}
	out := make([]T, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if b[j] < a[i] {
			out = append(out, b[j])
			j++
		} else {
			out = append(out, a[i])
			i++
		}
	}
	out = append(out, a[i:]...)
	return append(out, b[j:]...)
}
