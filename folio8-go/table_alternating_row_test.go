package folio8

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio8-go/internal/pagemodel"
)

var alternatingBase = pagemodel.Color{R: 0x11, G: 0x22, B: 0x33}
var alternatingFill = pagemodel.Color{R: 0xDD, G: 0xEE, B: 0xFF}

func alternatingTableDoc(baseBackground, altBackground string) string {
	background := ""
	if baseBackground != "" {
		background = fmt.Sprintf(`, "background": %q`, baseBackground)
	}
	return fmt.Sprintf(`{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8%s},
        "headerStyle": {"background": "#445566"},
        "altRowBackground": %q,
        "columns": [
          {"id": "e2", "label": "A", "width": 60, "bind": "{{row.a}}"},
          {"id": "e3", "label": "B", "width": 60, "bind": "{{row.b}}"}
        ]}
    ]},
    "pageFooter": {"elements": [], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"latin": ["Noto Sans"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "utcOffset": "+00:00",
  "version": "1.0"
}`, background, altBackground)
}

func fiveAlternatingRowsData(prefix string) string {
	return fmt.Sprintf(`{"items":[
    {"a":%q,"b":"0"},{"a":%q,"b":"1"},{"a":%q,"b":"2"},
    {"a":%q,"b":"3"},{"a":%q,"b":"4"}
  ]}`, prefix+"0", prefix+"1", prefix+"2", prefix+"3", prefix+"4")
}

// dataRowRectGroups slices out the five data rows' cell rects. boxRects
// is 1 when the table declares a `style.background` or `style.border` —
// SPEC-table-rules §1 makes that the table's OWN BOX, one extra rect
// appended after every row's — and 0 when it declares neither.
func dataRowRectGroups(t *testing.T, pages []pagemodel.Page, columns, boxRects int) [][]pagemodel.Rect {
	t.Helper()
	if len(pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(pages))
	}
	rects := pages[0].Rects
	if len(rects) != columns*6+boxRects {
		t.Fatalf("got %d rects, want %d (one header plus five data rows, %d columns each, plus %d box rect(s))", len(rects), columns*6+boxRects, columns, boxRects)
	}
	groups := make([][]pagemodel.Rect, 5)
	for row := range groups {
		// The frame's fill, when the table declares style.background, is
		// drawn BEFORE the table's first rect (SPEC-table-rules).
		start := boxRects + columns*(row+1)
		groups[row] = rects[start : start+columns]
	}
	return groups
}

func assertRowFills(t *testing.T, groups [][]pagemodel.Rect, alt pagemodel.Color) {
	t.Helper()
	for row, cells := range groups {
		for column, rect := range cells {
			if row%2 == 1 {
				if !rect.HasFill || rect.Fill != alt {
					t.Errorf("row %d column %d fill = {present:%v color:%+v}, want alternate %+v", row, column, rect.HasFill, rect.Fill, alt)
				}
				continue
			}
			// SPEC-table-rules §1: `altRowBackground` is the ONLY
			// declaration that fills a data cell, so an even row is
			// unfilled whether or not the table declares a
			// `style.background` — that now paints the table's own box.
			if rect.HasFill {
				t.Errorf("row %d column %d is filled %+v; only altRowBackground may fill a data cell", row, column, rect.Fill)
			}
		}
	}
}

func TestAlternatingRowBackgroundAppliesToOddCollectionIndexes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		base     string
		boxRects int
	}{
		{name: "a table style.background no longer reaches an even row", base: "#112233", boxRects: 1},
		{name: "even rows remain unfilled without a body background"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pages := tablePagesForTest(t, alternatingTableDoc(tc.base, "#DDEEFF"), fiveAlternatingRowsData("R"))
			assertRowFills(t, dataRowRectGroups(t, pages, 2, tc.boxRects), alternatingFill)
		})
	}
}

func TestAlternatingRowChoiceDependsOnTemplateAndCollectionIndexOnly(t *testing.T) {
	bluePages := tablePagesForTest(t, alternatingTableDoc("#112233", "#DDEEFF"), fiveAlternatingRowsData("A"))
	redPages := tablePagesForTest(t, alternatingTableDoc("#112233", "#CC0000"), fiveAlternatingRowsData("A"))
	changedDataPages := tablePagesForTest(t, alternatingTableDoc("#112233", "#DDEEFF"), fiveAlternatingRowsData("Z"))

	blueGroups := dataRowRectGroups(t, bluePages, 2, 1)
	redGroups := dataRowRectGroups(t, redPages, 2, 1)
	changedGroups := dataRowRectGroups(t, changedDataPages, 2, 1)
	for row := range blueGroups {
		for column := range blueGroups[row] {
			if blueGroups[row][column].HasFill != changedGroups[row][column].HasFill || blueGroups[row][column].Fill != changedGroups[row][column].Fill {
				t.Errorf("row %d column %d changed its fill when unrelated cell values changed", row, column)
			}
			if row%2 == 0 && blueGroups[row][column].HasFill {
				t.Errorf("row %d column %d is filled %+v; an even row takes no fill now that style.background paints the table's box", row, column, blueGroups[row][column].Fill)
			}
		}
	}
	if blueGroups[1][0].Fill == redGroups[1][0].Fill {
		t.Fatal("changing only altRowBackground did not change the odd row's page-model fill")
	}

	render := func(t *testing.T, alt string) []byte {
		t.Helper()
		tpl, err := ParseTemplate([]byte(alternatingTableDoc("#112233", alt)))
		if err != nil {
			t.Fatalf("ParseTemplate: %v", err)
		}
		res, err := Render(tpl, Data(fiveAlternatingRowsData("A")), nil, testShippedFontSet())
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		return res.Bytes
	}
	if bytes.Equal(render(t, "#DDEEFF"), render(t, "#CC0000")) {
		t.Fatal("templates differing only in altRowBackground produced byte-identical PDFs")
	}
}

// A malformed altRowBackground is refused at LOAD, located at the table and
// the field, before any row is reached (owner ruling, 2026-09-17).
func TestAlternatingRowBackgroundUsesExistingLocatedColourError(t *testing.T) {
	requireColourLoadError(t, alternatingTableDoc("#112233", "not-a-colour"), "e1", "altRowBackground")
}

func alternatingPaginatedFooterDoc() string {
	doc := footerFixtureDoc("count", false)
	doc = strings.Replace(doc,
		`"style": {"fontFamily": "latin", "fontSize": 8}`,
		// NO table `style.background`: SPEC-table-rules §1 makes it the
		// table's own box, which is a single rect spanning a
		// twenty-row, three-page table and therefore a different
		// pagination question from the one this fixture is for.
		`"style": {"fontFamily": "latin", "fontSize": 8}, "headerStyle": {"background": "#445566"}, "altRowBackground": "#DDEEFF"`, 1)
	return doc
}

// TestAlternatingRowBackgroundIsGeometryAndPaginationNeutral is the permanent
// relational witness for AD-13/AD-24.  It renders the exact same 20-row,
// three-page table with and without the optional field, then proves that the
// final page model has the same page partition, every run, and every
// non-fill rectangle field.  Fill presence/colour are intentionally the only
// permitted difference: they are the feature's whole output contract.
func TestAlternatingRowBackgroundIsGeometryAndPaginationNeutral(t *testing.T) {
	const rows = 20
	withAlternate := alternatingPaginatedFooterDoc()
	withoutAlternate := strings.Replace(withAlternate, `, "altRowBackground": "#DDEEFF"`, "", 1)
	data := footerFixtureData(rows)

	withPages := tablePagesForTest(t, withAlternate, data)
	withoutPages := tablePagesForTest(t, withoutAlternate, data)
	if len(withPages) < 3 {
		t.Fatalf("presence precondition: alternate fixture has %d pages, want at least 3", len(withPages))
	}
	if len(withPages) != len(withoutPages) {
		t.Fatalf("page partition changed: with alternate=%d pages, without=%d", len(withPages), len(withoutPages))
	}

	fillDifferences := 0
	for pageIndex := range withPages {
		withPage, withoutPage := withPages[pageIndex], withoutPages[pageIndex]
		if withPage.Width != withoutPage.Width || withPage.Height != withoutPage.Height ||
			withPage.MarginTop != withoutPage.MarginTop || withPage.MarginLeft != withoutPage.MarginLeft {
			t.Errorf("page %d geometry changed: with=%+v without=%+v", pageIndex, withPage, withoutPage)
		}
		if len(withPage.Rects) != len(withoutPage.Rects) {
			t.Fatalf("page %d rectangle cardinality changed: with alternate=%d, without=%d", pageIndex, len(withPage.Rects), len(withoutPage.Rects))
		}
		if len(withPage.Runs) != len(withoutPage.Runs) {
			t.Fatalf("page %d run cardinality changed: with alternate=%d, without=%d", pageIndex, len(withPage.Runs), len(withoutPage.Runs))
		}
		if !reflect.DeepEqual(withPage.Runs, withoutPage.Runs) {
			t.Errorf("page %d text runs changed when only altRowBackground was declared", pageIndex)
		}
		for rectIndex := range withPage.Rects {
			withRect, withoutRect := withPage.Rects[rectIndex], withoutPage.Rects[rectIndex]
			if withRect.HasFill != withoutRect.HasFill || withRect.Fill != withoutRect.Fill {
				fillDifferences++
			}
			withRect.HasFill, withRect.Fill = false, pagemodel.Color{}
			withoutRect.HasFill, withoutRect.Fill = false, pagemodel.Color{}
			if withRect != withoutRect {
				t.Errorf("page %d rectangle %d changed outside HasFill/Fill: with=%+v without=%+v", pageIndex, rectIndex, withRect, withoutRect)
			}
		}
	}
	if fillDifferences != 30 {
		t.Errorf("fill differences = %d, want 30 alternate cells (10 odd rows × 3 columns)", fillDifferences)
	}
}

func TestAlternatingRowBackgroundContinuesAcrossPagesAndExcludesHeaderFooter(t *testing.T) {
	const rows = 20
	doc := alternatingPaginatedFooterDoc()
	data := footerFixtureData(rows)
	plan, _, sources := paginateContentTableForTest(t, doc, data)
	if len(plan.Pages) < 3 {
		t.Fatalf("presence precondition: got %d pages, want at least 3", len(plan.Pages))
	}

	discriminatingBoundary := false
	for pageIndex, page := range plan.Pages {
		first := -1
		for _, ref := range page.ContentRects {
			source := sources[ref]
			if !source.isDataRow {
				continue
			}
			if first == -1 {
				first = source.rowIndex
			}
			// Even rows carry NO fill (SPEC-table-rules §1: only
			// altRowBackground fills a data cell); odd rows carry the
			// alternate. The parity is still the whole assertion.
			wantFill := source.rowIndex%2 == 1
			for column, rect := range source.rects {
				if rect.HasFill != wantFill || (wantFill && rect.Fill != alternatingFill) {
					t.Errorf("page %d row %d column %d fill = {present:%v color:%+v}, want filled=%v from collection parity", pageIndex, source.rowIndex, column, rect.HasFill, rect.Fill, wantFill)
				}
			}
		}
		if pageIndex > 0 && first%2 == 1 {
			discriminatingBoundary = true
		}
	}
	if !discriminatingBoundary {
		t.Fatal("presence precondition: no continuation page begins on an odd collection index; page-local reset would be indistinguishable")
	}

	header := pagemodel.Color{R: 0x44, G: 0x55, B: 0x66}
	seenHeader, seenFooter := 0, 0
	for _, source := range sources {
		switch {
		case source.isHeaderRow:
			seenHeader++
			for _, rect := range source.rects {
				if !rect.HasFill || rect.Fill != header {
					t.Errorf("source header fill = {present:%v color:%+v}, want header %+v", rect.HasFill, rect.Fill, header)
				}
			}
		case source.isFooterRow:
			seenFooter++
			for _, rect := range source.rects {
				// SPEC-table-rules §1: a footer cell takes no chrome from
				// the element's own style either. The property this arm
				// guards is the one it always guarded — a footer is NOT
				// an alternating data row — and it is now spelled as
				// "never the alternate colour" rather than "always the
				// body colour".
				if rect.HasFill {
					t.Errorf("source footer fill = {present:%v color:%+v}, want no fill at all", rect.HasFill, rect.Fill)
				}
			}
		}
	}
	if seenHeader != 1 || seenFooter != 1 {
		t.Fatalf("source groups: header=%d footer=%d, want one of each", seenHeader, seenFooter)
	}

	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	pages, _, _, _, err := buildPageModel(tpl, mustDecodeData(t, data), mustDecodeParams(t), testShippedFontSet())
	if err != nil {
		t.Fatalf("buildPageModel: %v", err)
	}
	if len(pages) != len(plan.Pages) {
		t.Fatalf("buildPageModel pages=%d, pagination pages=%d", len(pages), len(plan.Pages))
	}
	for pageIndex, page := range pages {
		headerCells := 0
		for _, rect := range page.Rects {
			if rect.HasFill && rect.Fill == header {
				headerCells++
			}
		}
		if headerCells != 3 {
			t.Errorf("page %d carries %d header-colour cells, want 3 (original/repeated header retained)", pageIndex, headerCells)
		}
	}
	// The source assertions above prove the footer's assigned colour before
	// pagination. Count the final page-model rectangles as well: of 20 rows ×
	// three cells, the 30 at odd collection indexes are the alternate colour
	// and NOTHING else in this document is filled except the header
	// (counted separately above) — the even rows and the footer take no
	// fill at all now that `style.background` paints the table's box.
	// This catches a pagination-stage mix-up that recolours the footer after
	// collectBandTableRuns has returned.
	baseCells, alternateCells := 0, 0
	for _, page := range pages {
		for _, rect := range page.Rects {
			switch {
			case rect.HasFill && rect.Fill == alternatingBase:
				baseCells++
			case rect.HasFill && rect.Fill == alternatingFill:
				alternateCells++
			}
		}
	}
	if baseCells != 0 {
		t.Errorf("final page-model carries %d body-colour cells, want 0 — no cell takes the element's own style.background", baseCells)
	}
	if alternateCells != 30 {
		t.Errorf("final page-model carries %d alternate-colour cells, want 30 (10 odd data rows × 3 columns)", alternateCells)
	}

	res, err := Render(tpl, Data(data), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	assertWellFormedPDF(t, "alternating paginated footer", res.Bytes, len(plan.Pages))
}
