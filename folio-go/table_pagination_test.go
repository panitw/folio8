package folio8

// Story 4.3: the repository's first MULTI-PAGE table fixture (R8 lifts
// 4.2's own "every table fits on one page" fence, here and only here).
//
// These tests exercise the REAL row-generating pipeline
// (collectBandTableRuns/collectBandTextRuns, table_render.go) feeding
// layout.Paginate through the SAME chromeRowGroup()/lineRowGroup()
// direct-field-lookup methods paginateDocument/contentColumnItems use in
// production (render.go/page_number.go) — this file's own itemsForTest
// helper duplicates only the "index into the caller's own slice" wiring
// paginateDocument already has, never Paginate's placement logic, so a
// green result here is a statement about the SHIPPED grouping mechanism.

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/layout"
)

// multiRowTableDoc is a small custom page size (R8's fixture, chosen
// small so a modest row count already crosses a page boundary) with one
// two-column table bound to `items[]`. footerPageCount, when true, adds
// a page-header/page-footer "Page {{page}} of {{pages}}" construct —
// AC3's own witness needs both passes to see the SAME table.
func multiRowTableDoc(footerPageCount bool) string {
	footerElements := `[]`
	if footerPageCount {
		// Finding 3 (this story's finisher review): "pf1" violated the
		// schema's own id pattern (^e[0-9a-z]+$) and never even parsed —
		// the true branch was dead AND broken. "e4" is free (the table's
		// own elements use e1/e2/e3) and nextId (5) already exceeds it.
		footerElements = `[{"id": "e4", "type": "text", "x": 0, "y": 0, "width": 180, "height": 8, "value": "Page {{page}} of {{pages}}", "style": {"fontFamily": "latin", "fontSize": 6}}]`
	}
	return fmt.Sprintf(`{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8},
        "columns": [
          {"id": "e2", "label": "A", "width": 80, "bind": "{{row.a}}"},
          {"id": "e3", "label": "B", "width": 80, "bind": "{{row.b}}"}
        ]}
    ]},
    "pageFooter": {"elements": %s, "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"latin": ["Noto Sans"]},
  "locale": "en",
  "nextId": 5,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`, footerElements)
}

// tableRowJSON is one bound row: A/B carry an "RxW-" marker (this
// story's own row-identity witness pattern, matching
// TestDataRowIdentityIsConsistentAndDistinct) so a run's own SourceText
// names the row it claims, independent of rowIndex bookkeeping.
type tableRowJSON struct {
	A string `json:"a"`
	B string `json:"b"`
}

// multiRowTableData builds `rows` bound rows. wrapRow, if >= 0, gets a
// long value in column "a" so that row wraps to several physical lines
// (AC1's "at least one row wrapped" precondition) — narrow enough (80pt
// at 8pt Noto Sans) that eight ~5-letter marked words will not fit one
// line.
func multiRowTableData(rows, wrapRow int) string {
	items := make([]tableRowJSON, rows)
	for i := range items {
		marker := fmt.Sprintf("R%dW-", i)
		a := marker + "x"
		if i == wrapRow {
			a = marker + "Alpha " + marker + "Bravo " + marker + "Charlie " + marker + "Delta " + marker + "Echo " + marker + "Foxtrot " + marker + "Golf " + marker + "Hotel"
		}
		items[i] = tableRowJSON{A: a, B: marker + "b"}
	}
	b, err := json.Marshal(map[string]any{"items": items})
	if err != nil {
		panic(err) // test-fixture construction only; a marshal failure here is a test bug, not reachable through Render
	}
	return string(b)
}

// itemsForTest mirrors paginateDocument's/contentColumnItems' own
// "index into the caller's slice" wiring for a CONTENT-band-only table
// (this file's fixtures declare no page-header/page-footer table and no
// images), carrying each item's Group via the SAME
// chromeRowGroup()/lineRowGroup() methods production code calls. It is
// NOT a second implementation of Paginate's placement logic — only of
// the bookkeeping that turns collectBandTableRuns/collectBandTextRuns'
// output into layout.ColumnItems, which is what lets RectRef/TextRunRef
// be traced straight back to tableRects[i]/contentRuns[i] in the
// assertions below (unlike contentColumnItems' own PHASE-A rects, which
// deliberately carry dummy refs — see that function's own doc comment).
func itemsForTest(contentRuns []textRunSource, tableRects []tableRectSource) []layout.ColumnItem {
	var items []layout.ColumnItem
	for i, r := range tableRects {
		items = append(items, layout.ColumnItem{
			ElementID: r.elementID, Top: r.top, Bottom: r.bottom,
			Rects: []layout.RectRef{layout.RectRef(i)},
			Group: r.chromeRowGroup(),
		})
	}
	for i := 0; i < len(contentRuns); i++ {
		j := i
		item := layout.ColumnItem{
			ElementID: contentRuns[i].elementID,
			Top:       contentRuns[i].itemTop, Bottom: contentRuns[i].itemBottom,
			Group: contentRuns[i].lineRowGroup(),
		}
		for j < len(contentRuns) &&
			contentRuns[j].elementID == contentRuns[i].elementID &&
			contentRuns[j].lineIndex == contentRuns[i].lineIndex {
			item.Runs = append(item.Runs, layout.TextRunRef(j))
			j++
		}
		items = append(items, item)
		i = j - 1
	}
	return items
}

// paginateContentTableForTest collects a content-band table's runs/rects
// through the real production functions and paginates them, returning
// enough to assert row atomicity/order directly against tableRects/
// contentRuns' own identity fields.
func paginateContentTableForTest(t *testing.T, tplJSON, dataJSON string) (plan layout.Pagination, contentRuns []textRunSource, tableRects []tableRectSource) {
	t.Helper()
	tpl, err := ParseTemplate([]byte(tplJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	bands, err := documentBands(tpl)
	if err != nil {
		t.Fatalf("documentBands: %v", err)
	}
	data := mustDecodeData(t, dataJSON)
	params := mustDecodeParams(t)
	geometry, err := pageGeometryOf(tpl)
	if err != nil {
		t.Fatalf("pageGeometryOf: %v", err)
	}
	fc := testFormatContext()
	contentRuns, tableRects, _, err = collectBandTableRuns(tpl, bands, contentBandIndex, data, params, fc, testShippedFontSet(), newFontCache(), nil)
	if err != nil {
		t.Fatalf("collectBandTableRuns: %v", err)
	}
	textRuns, _, _, err := collectBandTextRuns(tpl, bands, contentBandIndex, data, params, testShippedFontSet(), newFontCache(), contentBandResolver, nil)
	if err != nil {
		t.Fatalf("collectBandTextRuns: %v", err)
	}
	contentRuns = append(textRuns, contentRuns...)
	plan, err = layout.Paginate(geometry, itemsForTest(contentRuns, tableRects))
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	return plan, contentRuns, tableRects
}

// TestDataRowNeverSplitAcrossPages is AC1: for every page, the set of
// rowIndex values among that page's CHROME items equals the set among
// its LINE items (D-000.68: set equality by identity, never a count),
// and every run bearing a row's own content marker lands on that same
// page (the independent content anchor).
func TestDataRowNeverSplitAcrossPages(t *testing.T) {
	const rows = 20
	const wrapRow = 5
	plan, contentRuns, tableRects := paginateContentTableForTest(t, multiRowTableDoc(false), multiRowTableData(rows, wrapRow))

	if len(plan.Pages) < 2 {
		t.Fatalf("presence precondition: fixture must paginate to >= 2 pages, got %d — widen the fixture", len(plan.Pages))
	}
	wrappedLines := 0
	for _, r := range contentRuns {
		if r.isTableRowLine && r.rowIndex == wrapRow {
			wrappedLines++
		}
	}
	if wrappedLines < 2 {
		t.Fatalf("presence precondition: row %d must wrap to >= 2 physical lines, got %d run(s) carrying its marker", wrapRow, wrappedLines)
	}

	markerOf := func(text string) (row int, ok bool) {
		for i := range rows {
			marker := fmt.Sprintf("R%dW-", i)
			if containsSubstring(text, marker) {
				return i, true
			}
		}
		return 0, false
	}

	// CORE invariant, survivable under mutation (a) (no chrome at all):
	// every LINE item bearing one rowIndex lands on exactly ONE page, and
	// the marker its own SourceText carries names that same row. This
	// does not consult tableRects at all, so removing a data row's
	// chrome (mutation (a)) cannot make it vacuous — with no chrome, the
	// row's LINES are the only thing left to hold together, and this is
	// the assertion that they still do.
	contentChecked := 0
	linePages := map[int]map[int]bool{} // rowIndex -> set of pages its lines appear on
	for p, pg := range plan.Pages {
		for _, ref := range pg.ContentRuns {
			r := contentRuns[ref]
			if !r.isTableRowLine {
				continue
			}
			if linePages[r.rowIndex] == nil {
				linePages[r.rowIndex] = map[int]bool{}
			}
			linePages[r.rowIndex][p] = true
			row, ok := markerOf(r.text)
			if !ok {
				t.Fatalf("run %q carries no row marker — test fixture defect", r.text)
			}
			if row != r.rowIndex {
				t.Errorf("run %q carries marker for row %d but rowIndex = %d", r.text, row, r.rowIndex)
			}
			contentChecked++
		}
	}
	if contentChecked == 0 {
		t.Fatal("presence precondition: no table-row-line runs were examined")
	}
	for row, pages := range linePages {
		if len(pages) != 1 {
			t.Errorf("row %d's own physical lines are spread across %d distinct pages — a row's lines must all land on ONE page", row, len(pages))
		}
	}

	// SECONDARY invariant, only meaningful when the chrome exists (the
	// UNMUTATED tree, and mutation (b)'s subject): the chrome's rowIndex
	// set on a page must equal the line rowIndex set on that SAME page —
	// D-000.68's set equality, never a count. Skipped (not merely
	// vacuous-true) for a row with no chrome at all, which is exactly
	// mutation (a)'s effect and exactly why the CORE check above exists
	// independently.
	chromePage := map[int]int{}
	for p, pg := range plan.Pages {
		for _, ref := range pg.ContentRects {
			r := tableRects[ref]
			if r.isDataRow {
				chromePage[r.rowIndex] = p
			}
		}
	}
	for row, pages := range linePages {
		cp, hasChrome := chromePage[row]
		if !hasChrome {
			continue
		}
		var lp int
		for pg := range pages {
			lp = pg
		}
		if cp != lp {
			t.Errorf("row %d: chrome lands on page %d but its lines land on page %d — chrome and lines must share a page (D-000.68)", row, cp, lp)
		}
	}
	for row := range chromePage {
		if _, hasLines := linePages[row]; !hasLines {
			t.Errorf("row %d has a chrome item but no line items at all", row)
		}
	}
}

// TestTableRowsAppearExactlyOnceInDataOrderAcrossPages is AC2: every row
// of the bound collection appears on EXACTLY ONE page (no duplication,
// no omission — asserted as two SEPARATE failures, D-000.79's own
// discipline), and page p's rows all precede page p+1's (a table's own
// rows are laid out strictly downward by construction — rowTop = the
// previous row's rowBottom, table_render.go — so within one table this
// is the row-loop order already; this test pins it as an assertion
// rather than an assumption).
func TestTableRowsAppearExactlyOnceInDataOrderAcrossPages(t *testing.T) {
	const rows = 20
	plan, _, tableRects := paginateContentTableForTest(t, multiRowTableDoc(false), multiRowTableData(rows, -1))
	if len(plan.Pages) < 2 {
		t.Fatalf("presence precondition: fixture must paginate to >= 2 pages, got %d", len(plan.Pages))
	}

	// Finding 8 (this story's finisher review, Nit): a `rowPage` slice
	// used to be recorded here and read back only by an `if ... { continue
	// }` as the LAST statement of the loop body — a no-op with no effect
	// on any assertion. Removed rather than wired into a real check: each
	// row's own page is already asserted more precisely below (the union
	// check, and the per-page ordering check further down), so recording
	// it a second time here would just be a second, unused derivation.
	seenCount := make([]int, rows)
	maxRowSeenOnPage := make([]int, len(plan.Pages))
	for i := range maxRowSeenOnPage {
		maxRowSeenOnPage[i] = -1
	}
	for p, pg := range plan.Pages {
		for _, ref := range pg.ContentRects {
			r := tableRects[ref]
			if !r.isDataRow {
				continue
			}
			seenCount[r.rowIndex]++
			if r.rowIndex > maxRowSeenOnPage[p] {
				maxRowSeenOnPage[p] = r.rowIndex
			}
		}
	}
	for row := range rows {
		if seenCount[row] == 0 {
			t.Errorf("row %d is OMITTED — appears on no page", row)
		}
		if seenCount[row] > 1 {
			t.Errorf("row %d is DUPLICATED — its chrome appears %d times", row, seenCount[row])
		}
	}
	// Union of rowIndex values across all pages is exactly {0..rows-1}.
	union := map[int]bool{}
	for _, pg := range plan.Pages {
		for _, ref := range pg.ContentRects {
			r := tableRects[ref]
			if r.isDataRow {
				union[r.rowIndex] = true
			}
		}
	}
	if len(union) != rows {
		t.Errorf("union of rowIndex across pages has %d members, want %d", len(union), rows)
	}
	// Order: page p's rows all precede page p+1's.
	for p := 0; p+1 < len(plan.Pages); p++ {
		if maxRowSeenOnPage[p] == -1 || maxRowSeenOnPage[p+1] == -1 {
			continue
		}
		minNext := rows
		for _, ref := range plan.Pages[p+1].ContentRects {
			r := tableRects[ref]
			if r.isDataRow && r.rowIndex < minNext {
				minNext = r.rowIndex
			}
		}
		if maxRowSeenOnPage[p] >= minNext {
			t.Errorf("page %d's highest rowIndex (%d) is >= page %d's lowest (%d) — rows are not in the collection's order across pages", p, maxRowSeenOnPage[p], p+1, minNext)
		}
	}
}

// TestBothPaginationPassesAgreeOnRowPartition is AC3: PHASE A
// (contentColumnItems, page_number.go) and PHASE B (paginateDocument's
// own item order, render.go) must produce the SAME page count and the
// SAME per-page rowIndex partition for one table, even though the two
// builders append rects/text in different orders (D3) — because both
// now carry the SAME Group identity (chromeRowGroup/lineRowGroup),
// direct field lookup, on the SAME underlying tableRects/contentRuns.
func TestBothPaginationPassesAgreeOnRowPartition(t *testing.T) {
	const rows = 20
	tplJSON := multiRowTableDoc(false)
	dataJSON := multiRowTableData(rows, 5)

	tpl, err := ParseTemplate([]byte(tplJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	bands, err := documentBands(tpl)
	if err != nil {
		t.Fatalf("documentBands: %v", err)
	}
	data := mustDecodeData(t, dataJSON)
	params := mustDecodeParams(t)
	geometry, err := pageGeometryOf(tpl)
	if err != nil {
		t.Fatalf("pageGeometryOf: %v", err)
	}
	fc := testFormatContext()
	visible, _, err := computeVisibility(bands, data, params, fc)
	if err != nil {
		t.Fatalf("computeVisibility: %v", err)
	}
	imageRuns, err := collectImageRuns(tpl)
	if err != nil {
		t.Fatalf("collectImageRuns: %v", err)
	}
	contentTableRuns, contentTableRects, _, err := collectBandTableRuns(tpl, bands, contentBandIndex, data, params, fc, testShippedFontSet(), newFontCache(), visible)
	if err != nil {
		t.Fatalf("collectBandTableRuns: %v", err)
	}
	contentRuns, _, _, err := collectBandTextRuns(tpl, bands, contentBandIndex, data, params, testShippedFontSet(), newFontCache(), contentBandResolver, visible)
	if err != nil {
		t.Fatalf("collectBandTextRuns: %v", err)
	}
	// PHASE A's OWN order (page_number.go's predictDocument caller):
	// this band's text runs, THEN this band's table runs.
	contentRuns = append(contentRuns, contentTableRuns...)

	// PHASE A: the real production function.
	phaseAPlan, err := layout.Paginate(geometry, contentColumnItems(contentRuns, imageRuns, contentTableRects, visible, nil))
	if err != nil {
		t.Fatalf("PHASE A Paginate: %v", err)
	}

	// PHASE B: paginateDocument's OWN item order (render.go) — rects
	// first, then text — reproduced here with the SAME contentRuns/
	// contentTableRects slices phase A used, so any divergence is about
	// ORDER/GROUPING alone, not about different underlying data.
	var phaseBItems []layout.ColumnItem
	for i, r := range contentTableRects {
		phaseBItems = append(phaseBItems, layout.ColumnItem{
			ElementID: r.elementID, Top: r.top, Bottom: r.bottom,
			Rects: []layout.RectRef{layout.RectRef(i)},
			Group: r.chromeRowGroup(),
		})
	}
	for i := 0; i < len(contentRuns); i++ {
		j := i
		item := layout.ColumnItem{
			ElementID: contentRuns[i].elementID,
			Top:       contentRuns[i].itemTop, Bottom: contentRuns[i].itemBottom,
			Group: contentRuns[i].lineRowGroup(),
		}
		for j < len(contentRuns) &&
			contentRuns[j].elementID == contentRuns[i].elementID &&
			contentRuns[j].lineIndex == contentRuns[i].lineIndex {
			item.Runs = append(item.Runs, layout.TextRunRef(j))
			j++
		}
		phaseBItems = append(phaseBItems, item)
		i = j - 1
	}
	phaseBPlan, err := layout.Paginate(geometry, phaseBItems)
	if err != nil {
		t.Fatalf("PHASE B Paginate: %v", err)
	}

	if len(phaseAPlan.Pages) != len(phaseBPlan.Pages) {
		t.Fatalf("PHASE A produced %d pages, PHASE B produced %d — {{page}}/{{pages}} (D-2.7.2) would print the WRONG total", len(phaseAPlan.Pages), len(phaseBPlan.Pages))
	}
	if len(phaseAPlan.Pages) < 2 {
		t.Fatalf("presence precondition: fixture must paginate to >= 2 pages, got %d", len(phaseAPlan.Pages))
	}

	// D-000.68: SETS of pages per row, per phase — not a single
	// "last-write-wins" page number. A row whose lines are SPLIT across
	// two pages within one phase would otherwise silently report
	// whichever page its LAST line happened to land on, which can
	// coincide with the other phase's (correct) page purely by chance
	// and mask exactly the divergence this test exists to catch.
	rowPagesPhaseA := make([]map[int]bool, rows)
	rowPagesPhaseB := make([]map[int]bool, rows)
	for i := range rowPagesPhaseA {
		rowPagesPhaseA[i] = map[int]bool{}
		rowPagesPhaseB[i] = map[int]bool{}
	}
	for p, pg := range phaseAPlan.Pages {
		for _, ref := range pg.ContentRuns {
			r := contentRuns[ref]
			if r.isTableRowLine {
				rowPagesPhaseA[r.rowIndex][p] = true
			}
		}
	}
	for p, pg := range phaseBPlan.Pages {
		for _, ref := range pg.ContentRuns {
			r := contentRuns[ref]
			if r.isTableRowLine {
				rowPagesPhaseB[r.rowIndex][p] = true
			}
		}
	}
	for row := range rows {
		if len(rowPagesPhaseA[row]) != 1 {
			t.Errorf("row %d: PHASE A itself spreads this row's lines across %d distinct pages (%v) — PHASE A must keep a row whole too, or its own page count is unreliable", row, len(rowPagesPhaseA[row]), rowPagesPhaseA[row])
		}
		if len(rowPagesPhaseB[row]) != 1 {
			t.Errorf("row %d: PHASE B itself spreads this row's lines across %d distinct pages (%v)", row, len(rowPagesPhaseB[row]), rowPagesPhaseB[row])
		}
		if len(rowPagesPhaseA[row]) == 1 && len(rowPagesPhaseB[row]) == 1 {
			var pa, pb int
			for p := range rowPagesPhaseA[row] {
				pa = p
			}
			for p := range rowPagesPhaseB[row] {
				pb = p
			}
			if pa != pb {
				t.Errorf("row %d: PHASE A assigns page %d, PHASE B assigns page %d — the two passes disagree on the partition", row, pa, pb)
			}
		}
	}

	// Story 4.4, AC3: FR26's reservation is ONE derivation inside
	// layout.Paginate itself (never a second, independent copy per
	// pass, D-4.3.2/D-4.2.2), so both phases must agree not only on the
	// row partition above but on WHICH pages reserved the header's
	// height and by how much. This fixture (rows=20, wrapRow=5) is
	// known, at this story's creation, to change partition under the
	// reservation — presence-checked here so this assertion is not
	// vacuously true of a fixture the reservation never touches.
	totalDisp := func(p layout.Pagination) (pages int, disp int) {
		for _, pg := range p.Pages {
			for _, d := range pg.RowDisplacement {
				if d.ElementID == "e1" {
					pages++
					disp += int(d.Amount)
				}
			}
		}
		return pages, disp
	}
	aPages, aDisp := totalDisp(phaseAPlan)
	bPages, bDisp := totalDisp(phaseBPlan)
	if aPages == 0 {
		t.Fatal("presence precondition: this fixture must exercise FR26's reservation on at least one page, or AC3's own teeth are vacuous")
	}
	if aPages != bPages || aDisp != bDisp {
		t.Errorf("PHASE A reserves the header on %d page(s) (total %dmp), PHASE B on %d page(s) (total %dmp) — the two passes disagree on FR26's reservation itself", aPages, aDisp, bPages, bDisp)
	}
}

// STORY 4.6 REWROTE THE TWO TESTS BELOW, and it is the story Story 4.3
// named in writing when it placed them: "Story 4.6 owns the real answer
// (clip to a fresh page, report a Diagnostic); THIS test records CURRENT
// behaviour and does not endorse it."
//
// Story 4.3 pinned BOTH orderings — PHASE B (itemsForTest: rects before
// text, Kind "table") and PHASE A (the shipped path via Render: text
// before rects, Kind "line") — because the two disagreed about Kind and
// leaving one unmeasured would have hidden that. The clip makes the
// disagreement moot, and this pair now says so: the clip is keyed on the
// GROUP, whose union extent is a property of the group's members and not
// of the order the sweep visits them in, so both orderings produce the
// SAME clip on the SAME document. That agreement is asserted rather than
// assumed, which is what these two tests are still for.
// tooTallRowDoc's fontSize 200pt makes a single physical line's own
// height alone exceed the ~100,000mp content window this fixture's
// 150pt-tall page leaves (150-10-10-10-10 = 110pt content height): the
// row is unplaceable in ANY window, the residual case FR44 exists for.
// Shared by both AC4 tests below (PHASE B via itemsForTest, PHASE A via
// the public Render()) so the two arms are pinned against the SAME
// document, not two documents that merely resemble each other.
func tooTallRowDoc() string {
	return `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 200},
        "columns": [{"id": "e2", "label": "A", "width": 80, "bind": "{{row.a}}"}]}
    ]},
    "pageFooter": {"elements": [], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"latin": ["Noto Sans"]},
  "locale": "en",
  "nextId": 5,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

func TestRowTallerThanContentWindowIsClippedUnderPhaseBOrdering(t *testing.T) {
	doc := tooTallRowDoc()
	data := multiRowTableData(1, -1)

	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	bands, err := documentBands(tpl)
	if err != nil {
		t.Fatalf("documentBands: %v", err)
	}
	dataVal := mustDecodeData(t, data)
	params := mustDecodeParams(t)
	geometry, err := pageGeometryOf(tpl)
	if err != nil {
		t.Fatalf("pageGeometryOf: %v", err)
	}
	fc := testFormatContext()
	contentRuns, tableRects, _, err := collectBandTableRuns(tpl, bands, contentBandIndex, dataVal, params, fc, testShippedFontSet(), newFontCache(), nil)
	if err != nil {
		t.Fatalf("collectBandTableRuns: %v", err)
	}
	textRuns, _, _, err := collectBandTextRuns(tpl, bands, contentBandIndex, dataVal, params, testShippedFontSet(), newFontCache(), contentBandResolver, nil)
	if err != nil {
		t.Fatalf("collectBandTextRuns: %v", err)
	}
	contentRuns = append(textRuns, contentRuns...)

	// Presence precondition: the row's own chrome really is taller than
	// the content window, or the clip case is not exercised.
	contentHeight := layout.ContentHeight(geometry)
	var rowHeight int64
	for _, r := range tableRects {
		if r.isDataRow {
			rowHeight = int64(r.bottom - r.top)
		}
	}
	if rowHeight <= int64(contentHeight) {
		t.Fatalf("presence precondition: the row is %dmp tall, which FITS the %dmp window — the clip case is not exercised", rowHeight, contentHeight)
	}

	// No explicit timeout wrapper here: the sweep is a SINGLE forward
	// pass (R6), and the bounded-return assertion this story owes AC6
	// lives in TestOverTallGroupPaginationTerminatesWithinABound, which
	// wraps the PUBLIC entry point in a select/time.After.
	plan, perr := layout.Paginate(geometry, itemsForTest(contentRuns, tableRects))
	if perr != nil {
		t.Fatalf("Paginate returned %T: %v — Story 4.6 clips an over-tall ROW instead of erroring (AD-14: never fatal)", perr, perr)
	}
	if len(plan.Clipped) != 1 {
		t.Fatalf("plan.Clipped = %+v; want exactly one record for the document's one over-tall row", plan.Clipped)
	}
	c := plan.Clipped[0]
	if c.ElementID != "e1" {
		t.Errorf("clip.ElementID = %q, want %q", c.ElementID, "e1")
	}
	if c.Key != (layout.ItemGroupKey{ElementID: "e1", Index: 0}) {
		t.Errorf("clip.Key = %+v, want the table's row 0 — the row index the diagnostic names comes from here", c.Key)
	}
	if int64(c.ItemHeight) != rowHeight || c.ContentHeight != contentHeight {
		t.Errorf("clip reports %dmp against %dmp; want the row's measured height %dmp against the content window %dmp",
			c.ItemHeight, c.ContentHeight, rowHeight, contentHeight)
	}
}

// TestRowTallerThanContentWindowIsClippedThroughRender is the OTHER arm
// — the shipped path, PHASE A (contentColumnItems appends TEXT before
// RECTS), through the actual public Render(). Story 4.3 called this "the
// pin 4.6 must build its clip-and-diagnose behaviour against", and this
// is 4.6 building against it.
//
// Story 4.3's pin here was Kind "line" for a table row — a mislabelling
// it recorded and routed to this story. It is discharged BY
// CONSTRUCTION rather than by relabelling: the clip is keyed on
// ItemGroup.Present, which is exactly what Kind could not distinguish
// (measured at 45cf812: an over-tall table row and an over-tall plain
// text element were byte-identical at this entry point — both 0 bytes,
// both CONTENT_UNLAYOUTABLE, both Kind "line"). A table row no longer
// reaches OverflowError at all, so it no longer carries a Kind to be
// wrong about. See layout.OverflowError's own doc comment for the
// reachability record (AC8).
//
// AND THE AGREEMENT BETWEEN THE TWO ORDERINGS, asserted rather than
// assumed: this test and the PHASE B one above run the SAME document and
// must produce the SAME clip, because a group's union extent does not
// depend on the order its members are visited in.
func TestRowTallerThanContentWindowIsClippedThroughRender(t *testing.T) {
	tpl, err := ParseTemplate([]byte(tooTallRowDoc()))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	res, rerr := Render(tpl, Data(multiRowTableData(1, -1)), nil, testShippedFontSet())
	if rerr != nil {
		t.Fatalf("Render returned %T: %v — a document whose only row is taller than the content window now renders (AD-14: never fatal)", rerr, rerr)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("Render returned a nil error and zero bytes")
	}

	var got []Diagnostic
	for _, d := range res.Diagnostics {
		if d.Code == DiagCodeTableRowClippedHeight {
			got = append(got, d)
		}
	}
	if len(got) != 1 {
		t.Fatalf("Result.Diagnostics carries %d %s diagnostic(s); want exactly 1. All: %+v", len(got), DiagCodeTableRowClippedHeight, res.Diagnostics)
	}
	if got[0].ElementID != "e1" {
		t.Errorf("ElementID = %q, want %q", got[0].ElementID, "e1")
	}
	// The row index — the data HEAD's *layout.OverflowError never
	// carried at all (its ElementID was the TABLE's id and there was no
	// row index in the type).
	if !strings.Contains(got[0].Message, "row 0 of the bound collection") {
		t.Errorf("the message does not name the row: %s", got[0].Message)
	}
	// PHASE A and PHASE B agree. The heights below are the same ones
	// TestRowTallerThanContentWindowIsClippedUnderPhaseBOrdering reads
	// off layout.Paginate directly, under the opposite item order.
	for _, want := range []string{"272400mp", "110000mp"} {
		if !strings.Contains(got[0].Message, want) {
			t.Errorf("the message does not contain %q — PHASE A and PHASE B must clip the SAME group by the SAME arithmetic; the group's union extent does not depend on visit order.\ngot: %s", want, got[0].Message)
		}
	}
}

// headerPushedToNextPageDoc places the table far enough down its content
// band (el.Y past contentHeight-headerHeight) that the HEADER ROW itself
// does not fit page 0's window at all — AC5's own fixture shape, applied
// to the header rather than a data row.
func headerPushedToNextPageDoc() string {
	return `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e6", "type": "text", "x": 0, "y": 0, "width": 180, "height": 8, "value": "filler", "style": {"fontFamily": "latin", "fontSize": 8}},
      {"id": "e1", "type": "table", "x": 0, "y": 105, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8},
        "columns": [
          {"id": "e2", "label": "A", "width": 80, "bind": "{{row.a}}"},
          {"id": "e3", "label": "B", "width": 80, "bind": "{{row.b}}"}
        ]}
    ]},
    "pageFooter": {"elements": [], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"latin": ["Noto Sans"]},
  "locale": "en",
  "nextId": 7,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

// TestHeaderRowMovesWholeToNextPageThenRepeatsOnEveryLaterOne is AC5's
// "moves whole" half (UNCHANGED, Story 4.3) AND Story 4.4's own inverted
// fence.
//
// STORY 4.4 AUTHORISED THIS INVERSION. Before this story, this test was
// named TestHeaderRowMovesWholeToNextPageAndIsNotRepeated and its own doc
// comment ended with a live fence: "the header must appear on EXACTLY ONE
// page — repeating it is Story 4.4's job, not this story's." Per that
// fence's own text, Story 4.4 is the one authorised to invert it — see
// the story's own "One existing test this story OWNS inverting" section.
// This is that inversion: rescoped and renamed, never deleted, because
// the two "moves whole" halves below are still load-bearing and still
// pass unmutated.
//
// A header that does not fit the remaining content height moves — chrome
// AND column labels TOGETHER — to the next page, exactly like a data row
// (D2's own note: this already held by the chrome accident at 903bf8f;
// the mechanism Story 4.3 shipped holds it for a REASON, and this test's
// first two invariants below are exactly that mechanism, unmutated by
// this story). What changes is the OLD fence: the header's OWN chrome
// and labels (the ORIGINAL ColumnItems, read via ContentRects/ContentRuns)
// still land on exactly the one page they always did — that is a true
// and unchanged fact, not weakened here — but the header ALSO now
// appears, through the SEPARATE PageAssignment.HeaderRepeats channel
// (DECISION-3: never folded into ContentRects/ContentRuns), on every
// later page this table's own rows land on. The new assertion below
// checks that channel by name, so a regression that silently stopped
// repeating the header on THIS fixture (header pushed off page 0 by a
// filler element, Story 4.3's own AC5 shape) would redden here too, not
// only in table_header_repeat_test.go's own simpler fixture.
//
// FINDER'S NOTE ON TEETH (finisher fix, Story 4.4 review Finding 13,
// Minor): the two "moves whole" halves below stay GREEN when row
// atomicity is deleted outright (RP-E, the story's own red-proof),
// because this fixture's header chrome and its label happen to share an
// identical extent — the exact "chrome accident" Story 4.3 itself named
// — so this fixture cannot, on its own, discriminate a grouped header
// from an ungrouped one. That property is properly, and independently,
// guarded by
// TestPaginateHeaderGroupMovesWholeEvenWhenChromeAndLabelExtentsDiffer
// and TestPaginateRowGroupMovesWholeRegardlessOfAppendOrder (both
// reddened under RP-E). The two halves here are therefore RE-CONFIRMING
// witnesses on a realistic document shape, not this property's sole
// guard — read them that way rather than as free-standing teeth.
func TestHeaderRowMovesWholeToNextPageThenRepeatsOnEveryLaterOne(t *testing.T) {
	const rows = 60
	plan, contentRuns, tableRects := paginateContentTableForTest(t, headerPushedToNextPageDoc(), multiRowTableData(rows, -1))

	if len(plan.Pages) < 3 {
		t.Fatalf("presence precondition: fixture must paginate to >= 3 pages so 'not repeated' is observable on a page the header did NOT land on, got %d", len(plan.Pages))
	}
	// Presence precondition: page 0 carries the FILLER (so the sweep has
	// already committed to page 0 before reaching the table — otherwise
	// the "no page is ever empty" rule slides window 0 to the header's
	// own top instead of advancing the page, and the header would "move"
	// to a page that never had anything else on it, proving nothing) but
	// NO table content at all — no chrome rect, no header label.
	if len(plan.Pages[0].ContentRects) != 0 {
		t.Fatalf("presence precondition: page 0 carries %d rect(s) — the header must not fit page 0's window at all", len(plan.Pages[0].ContentRects))
	}
	for _, ref := range plan.Pages[0].ContentRuns {
		if contentRuns[ref].isHeaderLabel || contentRuns[ref].isTableRowLine {
			t.Fatalf("presence precondition: page 0 carries table content — the header must not fit page 0's window at all, or this fixture does not exercise AC5")
		}
	}

	// CORE invariant, survivable under mutation (a) (no header chrome at
	// all): every header-LABEL run lands on the SAME page as every other
	// — the header's own labels stay together even with no chrome to
	// carry them, which is what proves this is a mechanism and not the
	// chrome accident.
	headerLabelPages := map[int]bool{}
	for p, pg := range plan.Pages {
		for _, ref := range pg.ContentRuns {
			if contentRuns[ref].isHeaderLabel {
				headerLabelPages[p] = true
			}
		}
	}
	if len(headerLabelPages) == 0 {
		t.Fatal("no header-label runs were found at all — test fixture defect")
	}
	if len(headerLabelPages) != 1 {
		t.Errorf("header labels are spread across %d distinct pages; they must all land on ONE page", len(headerLabelPages))
	}
	var headerPage int
	for p := range headerLabelPages {
		headerPage = p
	}
	if headerPage == 0 {
		t.Errorf("header labels landed on page 0 — the header must not fit page 0's window at all (presence precondition failed silently)")
	}

	// SECONDARY invariant, only meaningful when the chrome exists (the
	// UNMUTATED tree, and mutation (b)'s subject): the header's chrome
	// must be on the SAME page as its labels.
	headerChromePages := map[int]bool{}
	for p, pg := range plan.Pages {
		for _, ref := range pg.ContentRects {
			if tableRects[ref].isHeaderRow {
				headerChromePages[p] = true
			}
		}
	}
	if len(headerChromePages) > 0 {
		if len(headerChromePages) != 1 || !headerChromePages[headerPage] {
			t.Errorf("header chrome pages = %v, want exactly {%d} (the same page as the header's labels)", headerChromePages, headerPage)
		}
	}

	// UNCHANGED HALF: the header's OWN, DECLARED chrome and labels — the
	// ORIGINAL ColumnItems, read via ContentRects/ContentRuns exactly as
	// Story 4.3 built them — still land on EXACTLY the one page they
	// always did. This is not "not repeated" any more (see below); it is
	// "the header's own position is still a single page", which stays
	// true regardless of FR26.
	for p := range plan.Pages {
		if p == headerPage {
			continue
		}
		if headerChromePages[p] {
			t.Errorf("page %d carries the header's OWN chrome rect (via ContentRects) — its declared position must still be exactly ONE page", p)
		}
		if headerLabelPages[p] {
			t.Errorf("page %d carries the header's OWN label (via ContentRuns) — its declared position must still be exactly ONE page", p)
		}
	}

	// STORY 4.4's INVERTED HALF: the header DOES now appear again, via
	// the SEPARATE HeaderRepeats channel, on every LATER page this
	// table's own rows land on — FR26, and the very thing the pre-4.4
	// fence forbade.
	rowPages := map[int]bool{}
	for p, pg := range plan.Pages {
		for _, ref := range pg.ContentRects {
			if tableRects[ref].isDataRow {
				rowPages[p] = true
			}
		}
	}
	if len(rowPages) < 2 {
		t.Fatalf("presence precondition: this table's rows must span >= 2 pages so 'every later page' is observable on more than one, got %d", len(rowPages))
	}
	for p := range rowPages {
		if p == headerPage {
			continue // the header's OWN page is never a "repeat" (DECISION-1)
		}
		repeated := false
		for _, rep := range plan.Pages[p].HeaderRepeats {
			if rep.ElementID == "e1" {
				repeated = true
			}
		}
		if !repeated {
			t.Errorf("page %d carries this table's own rows but no HeaderRepeats entry for e1 — FR26's repeat is missing on this continuation page", p)
		}
	}
	// And the header's OWN page must never carry a "repeat" of itself
	// (DECISION-1: a repeat captions rows on a CONTINUATION page; the
	// page holding the header's own declared position is not one).
	for _, rep := range plan.Pages[headerPage].HeaderRepeats {
		if rep.ElementID == "e1" {
			t.Errorf("page %d (the header's own declared page) carries a HeaderRepeats entry for e1 — DECISION-1 forbids a repeat on the header's own page", headerPage)
		}
	}
}

// TestMultiRowTableRendersThroughPublicRenderWithPageCountFooter is
// Finding 3 (this story's finisher review) and Major 3's coverage gap
// together: no test this story added ever called the public Render() —
// every AC1/AC2/AC3/AC5 test above stops at layout.Paginate — so a
// rendering-level regression (Finding 1) could not have been caught by
// anything in this file. This test takes the `{{page}} of {{pages}}`
// branch through Render() (the fixture's own id/nextId bug that made it
// unparseable is fixed above), and cross-checks PHASE A's page count
// (the value {{page}}/{{pages}} print, D-2.7.2) against the PRODUCED
// PDF's own /Type /Page object count and its /Count entry — two
// independent sites, D-000.33, not a conservation law.
func TestMultiRowTableRendersThroughPublicRenderWithPageCountFooter(t *testing.T) {
	table := []struct {
		rows      int
		wrapRow   int
		wantPages int
	}{
		// Story 4.4: this fixture's page count moved 3 -> 4. FR26's
		// repeated header reserves headerHeight (10,000mp) on every
		// continuation page, which is exactly the reservation this
		// story adds — a genuine behaviour change (AC2), re-measured
		// against the shipped mechanism: page 0 holds rows 0-4 (row 5
		// wraps and does not fit), page 1 holds rows 5-8 (reservation
		// active), page 2 holds rows 9-17, page 3 holds rows 18-19.
		{rows: 20, wrapRow: 5, wantPages: 4},
		{rows: 40, wrapRow: -1, wantPages: 5},
		{rows: 60, wrapRow: -1, wantPages: 7},
	}
	for _, row := range table {
		t.Run(fmt.Sprintf("rows=%d", row.rows), func(t *testing.T) {
			tplJSON := multiRowTableDoc(true)
			tpl, err := ParseTemplate([]byte(tplJSON))
			if err != nil {
				t.Fatalf("ParseTemplate: %v — the footerPageCount=true branch must parse", err)
			}
			res, err := Render(tpl, Data(multiRowTableData(row.rows, row.wrapRow)), nil, testShippedFontSet())
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if len(res.Bytes) == 0 {
				t.Fatal("presence precondition: Render returned zero bytes")
			}

			// PHASE A's own answer (the value {{page}}/{{pages}} print),
			// derived independently via layout.Paginate over the SAME
			// content-band table, mirroring what predictDocument does
			// before the footer/header bands are collected.
			plan, _, _ := paginateContentTableForTest(t, multiRowTableDoc(false), multiRowTableData(row.rows, row.wrapRow))
			if len(plan.Pages) != row.wantPages {
				t.Fatalf("presence precondition: the content-band-only pagination itself produced %d pages, want %d — widen the fixture", len(plan.Pages), row.wantPages)
			}

			if got := countPageObjects(res.Bytes); got != row.wantPages {
				t.Errorf("rendered %d /Type /Page object(s) for %d row(s); PHASE A (and this test's declared value) says %d", got, row.rows, row.wantPages)
			}
			if got := readDeclaredCount(t, res.Bytes); got != row.wantPages {
				t.Errorf("/Count is %d for %d row(s); the declared value is %d", got, row.rows, row.wantPages)
			}
		})
	}
}

// tableBesideSameYElementDoc places a table and an ORDINARY text element
// at the SAME y within the content band — Finding 1's exact shape (this
// story's finisher review): a caption, a column heading, or any other
// element level with a table is a canonical, LEGAL report layout, not a
// caller bug. The two elements do not overlap horizontally (x 0..120 for
// the table's two 60pt columns, x 130..175 for the note) — Finding 1
// measured that horizontal overlap is not even required to trigger the
// pre-fix defect, only a shared y.
func tableBesideSameYElementDoc() string {
	return `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8},
        "columns": [
          {"id": "e2", "label": "A", "width": 60, "bind": "{{row.a}}"},
          {"id": "e3", "label": "B", "width": 60, "bind": "{{row.b}}"}
        ]},
      {"id": "e4", "type": "text", "x": 130, "y": 0, "width": 45, "height": 8, "value": "Note", "style": {"fontFamily": "latin", "fontSize": 8}}
    ]},
    "pageFooter": {"elements": [], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"latin": ["Noto Sans"]},
  "locale": "en",
  "nextId": 5,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

// TestTableBesideSameYElementRenders is Finding 1's regression, at the
// Render() level (this story's finisher review): before the fix, this
// exact document returned "internal error: group ... is not contiguous
// in column order" — an ordinary, legal template made unrenderable by
// R7's contiguity check firing on a caller who did nothing wrong. The
// finisher review measured the identical document rendering successfully
// against a 903bf8f worktree, which is what makes this a REGRESSION
// rather than a pre-existing limitation. It must render here too, and a
// table beside a same-y element is exactly the coverage hole Finding 3
// names as the reason no test in this story called Render() at all.
func TestTableBesideSameYElementRenders(t *testing.T) {
	tpl, err := ParseTemplate([]byte(tableBesideSameYElementDoc()))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	if _, err := Render(tpl, Data(multiRowTableData(3, -1)), nil, testShippedFontSet()); err != nil {
		t.Fatalf("Render: %v — a table beside an ordinary element merely sharing its y must render; this is Finding 1's regression", err)
	}
}

// TestRowMadeTooTallByTypedBreaksIsTheExistingClipAndWarn is Story
// 7.1's I/O-matrix row "Over-tall row -> existing
// TABLE_ROW_CLIPPED_HEIGHT Warning", and it closes the one thing the
// story's other height tests assert only by absence.
//
// Line feeds are a NEW ROUTE to an old condition: before FR46 a row
// could only outgrow the content window through a large font size or a
// long value, and now a cell carrying enough typed breaks does it at an
// ordinary size. The behaviour must be Story 4.6's existing
// Pagination.Clipped path, UNCHANGED — the same code, the same Warning
// beside the bytes, never fatal, and NO NEW DIAGNOSTIC CODE minted for
// the new cause.
//
// It is deliberately the same document shape as tooTallRowDoc's, with
// the font size back at a readable 8 pt so that the breaks, and nothing
// else, are what make the row too tall.
func TestRowMadeTooTallByTypedBreaksIsTheExistingClipAndWarn(t *testing.T) {
	const doc = `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8},
        "columns": [{"id": "e2", "label": "A", "width": 80, "bind": "{{row.a}}"}]}
    ]},
    "pageFooter": {"elements": [], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"latin": ["Noto Sans"]},
  "locale": "en",
  "nextId": 5,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	// 20 typed breaks -> 21 lines in one cell, at an ordinary 8 pt.
	tall := strings.Repeat("R0W-x\n", 20) + "R0W-x"
	dataJSON, merr := json.Marshal(map[string]any{"items": []tableRowJSON{{A: tall, B: "R0W-b"}}})
	if merr != nil {
		t.Fatalf("marshal: %v", merr)
	}

	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}

	// NEGATIVE CONTROL FIRST: the same value with its breaks replaced by
	// spaces fits, and warns about nothing. Without it, "the row is too
	// tall" could not be told from a document that was too tall anyway.
	flat, ferr := json.Marshal(map[string]any{"items": []tableRowJSON{{A: strings.ReplaceAll(tall, "\n", " "), B: "R0W-b"}}})
	if ferr != nil {
		t.Fatalf("marshal: %v", ferr)
	}
	controlRes, cerr := Render(tpl, Data(string(flat)), nil, testShippedFontSet())
	if cerr != nil {
		t.Fatalf("control render: %v", cerr)
	}
	for _, d := range controlRes.Diagnostics {
		if d.Code == DiagCodeTableRowClippedHeight {
			t.Fatalf("presence precondition: the control document (no typed breaks) is ALREADY clipped for height (%+v) — the subject below would prove nothing", d)
		}
	}

	// THE SUBJECT.
	res, rerr := Render(tpl, Data(string(dataJSON)), nil, testShippedFontSet())
	if rerr != nil {
		t.Fatalf("Render returned %T: %v — an over-tall row is clipped and reported, never fatal (AD-14)", rerr, rerr)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("Render returned a nil error and zero bytes")
	}

	var clipped []Diagnostic
	for _, d := range res.Diagnostics {
		if d.Code == DiagCodeTableRowClippedHeight {
			clipped = append(clipped, d)
		}
	}
	if len(clipped) != 1 {
		t.Fatalf("a row made too tall by TYPED BREAKS produced %d %s diagnostic(s), want exactly 1. All: %+v", len(clipped), DiagCodeTableRowClippedHeight, res.Diagnostics)
	}
	if clipped[0].Severity != SeverityWarning {
		t.Errorf("Severity = %q, want a Warning — an over-tall row is never fatal", clipped[0].Severity)
	}
	if clipped[0].ElementID != "e1" {
		t.Errorf("ElementID = %q, want %q", clipped[0].ElementID, "e1")
	}
	if !strings.Contains(clipped[0].Message, "row 0 of the bound collection") {
		t.Errorf("the message does not name the row: %s", clipped[0].Message)
	}

	// NO NEW CODE WAS MINTED FOR THE NEW CAUSE. The only diagnostics
	// this document may produce are the ones that already existed.
	for _, d := range res.Diagnostics {
		switch d.Code {
		case DiagCodeTableRowClippedHeight, DiagCodeTextClippedWidth:
		default:
			t.Errorf("unexpected diagnostic %+v — a line feed is a new ROUTE to Story 4.6's existing condition, not a new condition", d)
		}
	}
	t.Logf("Story 4.6's path, reached by typed breaks alone at 8 pt: %s", clipped[0].Message)
}
