package folio8

import (
	"strings"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/layout"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// This file is the engine's one reading of SPEC-multi-pages' designed pages.
// Every content-element reader goes through contentElements, contentPageIndex
// or firstContentBand, never through Bands.Content: a multi-page document
// holds its content in Document.Pages and leaves Bands.Content empty.

// contentPagesSplit is the document's designed pages, resolved for
// pagination: which page each content item belongs to, each page's own
// section break, and each page's Page Break setting.
type contentPagesSplit struct {
	// pageOf maps a content element id to its page. Nil for a one-page
	// document, whose every item is page 0.
	pageOf map[string]int
	// breaks holds one section break per designed page; the zero value is
	// "no break" (SPEC-multi-pages CAP-6).
	breaks []sectionBreakSplit
	// pageBreaks holds each designed page's Page Break; page 1's is always
	// true (CAP-9).
	pageBreaks []bool
}

// contentPagesOf resolves t's designed pages against g.
func contentPagesOf(t *Template, g layout.PageGeometry) contentPagesSplit {
	count := t.doc.PageCount()
	split := contentPagesSplit{breaks: make([]sectionBreakSplit, count), pageBreaks: make([]bool, count)}
	for page := range split.breaks {
		split.breaks[page] = sectionBreakOf(t, g, page)
		split.pageBreaks[page] = page == 0 || t.doc.Pages[page].PageBreak
	}
	if count > 1 {
		split.pageOf = contentPageIndex(t)
	}
	return split
}

// partition splits items by designed page, keeping each page's items in
// their incoming order. A one-page document's items are returned as they are.
func (s contentPagesSplit) partition(items []layout.ColumnItem) [][]layout.ColumnItem {
	perPage := make([][]layout.ColumnItem, len(s.breaks))
	if len(perPage) == 1 {
		perPage[0] = items
		return perPage
	}
	for _, it := range items {
		page := s.pageOf[it.ElementID]
		perPage[page] = append(perPage[page], it)
	}
	return perPage
}

// contentPagesPlan is the whole document's pagination: each designed page
// paginated as its own column (with its own section break) and the output
// pages joined in page order. A page with Page Break on starts a new output
// page; one with Page Break off may instead share the previous page's last
// output page (CAP-9), so one output page can carry two designed pages.
type contentPagesPlan struct {
	// Pagination holds the joined output pages, with Suppressed and Clipped
	// page numbers restated in the document's output page numbering.
	layout.Pagination
	plans []sectionPlan
	// slots lists, per output page, the designed pages drawn on it and each
	// one's local page in its own plan, in page order.
	slots  [][]designedSlot
	pageOf map[string]int
}

// designedSlot is one designed page's local page drawn on an output page.
type designedSlot struct {
	page, local int
}

// outputShiftFor is the window shift for elementID's content on output page.
func (p contentPagesPlan) outputShiftFor(page int, elementID string) geom.Length {
	slots := p.slots[page]
	designed := p.pageOf[elementID]
	for _, slot := range slots {
		if slot.page == designed {
			return p.plans[slot.page].shiftFor(slot.local, elementID)
		}
	}
	return p.plans[slots[0].page].shiftFor(slots[0].local, elementID)
}

// paginateContentPages is the ONE pagination both of buildPageModel's passes
// run, so {{pages}} sums every designed page's output pages exactly as they
// are rendered. A one-page document is paginated exactly as before.
//
// PAGE BREAK OFF (CAP-9). Page N > 1 with Page Break off is paginated alone,
// with its own break, as one block. When page N−1 occupies more than one
// output page and its content has an end E on its last output page (nothing
// clipped there), and the block paginates to exactly one output page with
// nothing clipped whose extent below the window top fits between E and the
// window bottom, the block is moved down rigidly so its window top sits at E
// and joins that output page. Otherwise, exactly as with Page Break on, it
// starts a new output page. It never lands on an earlier output page.
func paginateContentPages(g layout.PageGeometry, items []layout.ColumnItem, split contentPagesSplit) (contentPagesPlan, []Diagnostic, error) {
	out := contentPagesPlan{pageOf: split.pageOf}
	var diags []Diagnostic
	perPage := split.partition(items)
	for page, pageItems := range perPage {
		plan, pageDiags, err := paginateWithSectionBreak(g, pageItems, split.breaks[page])
		if err != nil {
			return contentPagesPlan{}, nil, err
		}
		offset := len(out.Pages)
		if page > 0 && !split.pageBreaks[page] {
			if d, ok := pageBreakOffShift(g, perPage[page-1], out.plans[page-1], pageItems, plan); ok {
				translateSinglePagePlan(&plan, d)
				offset--
				out.Pages[offset] = mergePageAssignments(out.Pages[offset], plan.Pages[0])
				out.slots[offset] = append(out.slots[offset], designedSlot{page: page, local: 0})
				out.appendRestated(plan, offset)
				out.plans = append(out.plans, plan)
				diags = append(diags, pageDiags...)
				continue
			}
		}
		for local := range plan.Pages {
			out.slots = append(out.slots, []designedSlot{{page: page, local: local}})
		}
		out.Pages = append(out.Pages, plan.Pages...)
		out.appendRestated(plan, offset)
		out.plans = append(out.plans, plan)
		diags = append(diags, pageDiags...)
	}
	return out, diags, nil
}

// appendRestated appends plan's suppressions and clips, restated from the
// plan's local page numbering to output pages starting at offset.
func (p *contentPagesPlan) appendRestated(plan sectionPlan, offset int) {
	for _, s := range plan.Suppressed {
		s.Page += offset
		p.Suppressed = append(p.Suppressed, s)
	}
	for _, c := range plan.Clipped {
		c.Page += offset
		p.Clipped = append(p.Clipped, c)
	}
}

// pageBreakOffShift decides CAP-9 for one Page Break off page: the distance d
// its block moves down from its declared window top, and whether it joins the
// previous page's last output page at all.
func pageBreakOffShift(g layout.PageGeometry, prevItems []layout.ColumnItem, prev sectionPlan, items []layout.ColumnItem, plan sectionPlan) (geom.Length, bool) {
	// No overflow, no pull: a previous page that fits one output page is
	// followed by a new output page, as with Page Break on.
	if len(prev.Pages) < 2 {
		return 0, false
	}
	// An empty page adds nothing, so it never adds an output page; it needs no
	// E, and is placed nowhere.
	if len(items) == 0 {
		return 0, true
	}
	end, clipped := designedPageEnd(g, prevItems, prev)
	if clipped {
		return 0, false
	}
	origins := layout.Origins(g)
	if len(plan.Pages) != 1 || len(plan.Clipped) != 0 {
		return 0, false
	}
	extent, blockClipped := designedPageEnd(g, items, plan)
	if blockClipped || end+(extent-origins.Content) > origins.PageFooter {
		return 0, false
	}
	return end - origins.Content, true
}

// designedPageEnd is where one designed page's content ends on its last
// output page, in page space — the section-break "where content ends"
// measure (aboveLineEnd), below-line section included: the lowest bottom of
// any item on that page under its own shift, counting a table row's
// displacement, a floor push and a table's floored slice. clipped reports a
// group clipped on that page, which has no usable end.
func designedPageEnd(g layout.PageGeometry, items []layout.ColumnItem, plan sectionPlan) (geom.Length, bool) {
	last := len(plan.Pages) - 1
	for _, c := range plan.Clipped {
		if c.Page == last {
			return 0, true
		}
	}
	pa := plan.Pages[last]
	runs := make(map[layout.TextRunRef]bool, len(pa.ContentRuns))
	for _, r := range pa.ContentRuns {
		runs[r] = true
	}
	images := make(map[layout.ImageRef]bool, len(pa.ContentImages))
	for _, r := range pa.ContentImages {
		images[r] = true
	}
	rects := make(map[layout.RectRef]bool, len(pa.ContentRects))
	for _, r := range pa.ContentRects {
		rects[r] = true
	}
	end := layout.Origins(g).Content
	for _, it := range items {
		onLast := (len(it.Runs) > 0 && runs[it.Runs[0]]) || (len(it.Images) > 0 && images[it.Images[0]]) || (len(it.Rects) > 0 && rects[it.Rects[0]])
		if !onLast {
			continue
		}
		bottom := it.Bottom - plan.shiftFor(last, it.ElementID) + elementPushFor(pa.ElementPush, it.ElementID)
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

// translateSinglePagePlan moves a one-page plan down by d as a rigid block:
// every window shift (the page's and its section's) drops by d, a repeated
// header's own shift with it, and each page-space table slice moves down by d.
// Row displacements and floor pushes are relative and stay as they are; clip
// bounds are column space and stay as they are.
func translateSinglePagePlan(plan *sectionPlan, d geom.Length) {
	if d == 0 {
		return
	}
	pa := plan.Pages[0]
	pa.Shift -= d
	if len(pa.HeaderRepeats) > 0 {
		repeats := append([]layout.TableHeaderRepeat(nil), pa.HeaderRepeats...)
		for i := range repeats {
			repeats[i].Shift -= d
		}
		pa.HeaderRepeats = repeats
	}
	if len(pa.TableSlices) > 0 {
		slices := append([]layout.TableSlice(nil), pa.TableSlices...)
		for i := range slices {
			slices[i].Top += d
			slices[i].Bottom += d
		}
		pa.TableSlices = slices
	}
	pages := append([]layout.PageAssignment(nil), plan.Pages...)
	pages[0] = pa
	plan.Pages = pages
	if plan.sectionShift != nil {
		shifts := append([]geom.Length(nil), plan.sectionShift...)
		shifts[0] -= d
		plan.sectionShift = shifts
	}
}

// contentElements returns every designed page's content elements, in page
// order. For a one-page document it is that page's own slice.
func contentElements(t *Template) []template.Element {
	bands := t.doc.ContentBands()
	if len(bands) == 1 {
		return bands[0].Elements
	}
	var out []template.Element
	for _, band := range bands {
		out = append(out, band.Elements...)
	}
	return out
}

// firstContentBand is page 1's content band: the band commands that create
// or drop into `content` target when they name no page.
func firstContentBand(t *Template) *template.Band {
	return t.doc.ContentBands()[0]
}

// contentPageIndex maps every content element's id to its designed page's
// index. Ids are unique document-wide, so the map is total over the content
// elements. Looked up, never ranged.
func contentPageIndex(t *Template) map[string]int {
	out := map[string]int{}
	for page, band := range t.doc.ContentBands() {
		for _, el := range band.Elements {
			out[string(el.ID)] = page
		}
	}
	return out
}
