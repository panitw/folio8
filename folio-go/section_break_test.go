package folio8

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/designer"
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
)

// sectionBreakTestDoc is spec-section-break's engine witness: a 200x150pt
// page (content height 110pt) with a two-column table at y 0 bound to
// items[], a "Legend" text element at y 80, and "Page {{page}} of {{pages}}"
// in the footer. contentKeys is appended inside the content band (for
// example `, "sectionBreak": 75`); extraElements is appended to its elements.
//
// Measured geometry: the header row is 10pt and every data row 10.896pt, and
// the content origin is 20pt down the page, so a break at 75 is the line
// 95pt down the page. Five rows end above it, six cross it.
func sectionBreakTestDoc(contentKeys, extraElements string) string {
	return fmt.Sprintf(`{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8},
        "columns": [
          {"id": "e2", "label": "A", "width": 80, "bind": "{{row.a}}"},
          {"id": "e3", "label": "B", "width": 80, "bind": "{{row.b}}"}
        ]},
      {"id": "e5", "type": "text", "x": 0, "y": 80, "width": 180, "height": 12, "value": "Legend", "style": {"fontFamily": "latin", "fontSize": 8}}%s
    ]%s},
    "pageFooter": {"elements": [{"id": "e4", "type": "text", "x": 0, "y": 0, "width": 180, "height": 8, "value": "Page {{page}} of {{pages}}", "style": {"fontFamily": "latin", "fontSize": 6}}], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"latin": ["Noto Sans"]},
  "locale": "en",
  "nextId": 20,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "utcOffset": "+00:00",
  "version": "4.1"
}
`, extraElements, contentKeys)
}

const sectionBreakAt75 = `, "sectionBreak": 75`

// sectionBreakWithout removes the legend element, leaving the table alone:
// the independent oracle for where the above-line content ends.
func sectionBreakWithout(doc, element string) string {
	start := strings.Index(doc, `{"id": "`+element+`"`)
	end := strings.Index(doc[start:], "}}") + start + 2
	return strings.Replace(doc[:start]+doc[end:], "]},\n      \n", "]}\n", 1)
}

func sectionBreakPages(t *testing.T, doc string, rows int) ([]pagemodel.Page, []Diagnostic) {
	t.Helper()
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	return barcodePages(t, tpl, multiRowTableData(rows, -1))
}

func sectionBreakRender(t *testing.T, doc string, rows int) Result {
	t.Helper()
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	res, err := Render(tpl, Data(multiRowTableData(rows, -1)), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return res
}

// legendOn returns every page the legend is drawn on, and its baseline.
func legendOn(pages []pagemodel.Page, text string) (onPages []int, y geom.Length) {
	for p, pg := range pages {
		for _, r := range pg.Runs {
			if r.SourceText == text {
				onPages = append(onPages, p)
				y = r.Y
				break
			}
		}
	}
	return onPages, y
}

// tableEnd is the oracle for E: the lowest rect bottom on the last page of a
// table-only render, and the content origin (the header rect's top on page 1).
func tableEnd(pages []pagemodel.Page) (origin, end geom.Length) {
	origin = pages[0].Rects[0].Y
	for _, r := range pages[0].Rects {
		if r.Y < origin {
			origin = r.Y
		}
	}
	for _, r := range pages[len(pages)-1].Rects {
		if r.Y+r.H > end {
			end = r.Y + r.H
		}
	}
	return origin, end
}

// requirePageXOfY asserts that every page draws "Page p of n", which is also
// the witness that pass A (the count {{pages}} prints) and pass B (the pages
// drawn) agree.
func requirePageXOfY(t *testing.T, b []byte) int {
	t.Helper()
	streams := splitPageContentStreams(t, b)
	cmap := mpParseToUnicode(t, b)
	for p := range streams {
		want := "Page " + itoaForTest(int64(p+1)) + " of " + itoaForTest(int64(len(streams)))
		found := false
		for _, run := range mpExtractRuns(t, streams[p], cmap) {
			if run.text == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("page %d of %d does not draw %q", p+1, len(streams), want)
		}
	}
	if got := readDeclaredCount(t, b); got != len(streams) {
		t.Errorf("/Count is %d for %d page content streams", got, len(streams))
	}
	return len(streams)
}

// TestSectionBreakLandingMatchesTheOracleForEveryRowCount is CAP-2 and CAP-4
// over every row count from 0 to 16, against an oracle built from a
// DIFFERENT render — the table alone, with no break and no legend.
func TestSectionBreakLandingMatchesTheOracleForEveryRowCount(t *testing.T) {
	doc := sectionBreakTestDoc(sectionBreakAt75, "")
	tableOnly := sectionBreakWithout(sectionBreakTestDoc("", ""), "e5")
	reference, _ := sectionBreakPages(t, sectionBreakTestDoc("", ""), 0)
	_, declaredY := legendOn(reference, "Legend")
	var notCrossed, crossedOnLast, endsAboveLater int
	for rows := 0; rows <= 16; rows++ {
		t.Run(fmt.Sprintf("rows=%d", rows), func(t *testing.T) {
			oracle, _ := sectionBreakPages(t, tableOnly, rows)
			origin, end := tableEnd(oracle)
			line := origin + 75000
			wantPage := len(oracle)
			if end <= line {
				wantPage = len(oracle) - 1
			}

			pages, _ := sectionBreakPages(t, doc, rows)
			onPages, y := legendOn(pages, "Legend")
			if len(onPages) != 1 {
				t.Fatalf("legend drawn on pages %v, want exactly one page", onPages)
			}
			if onPages[0] != wantPage {
				t.Fatalf("legend on page %d, want %d (table ends at %d on its page %d, line %d)", onPages[0]+1, wantPage+1, end, len(oracle), line)
			}
			if y != declaredY {
				t.Errorf("legend baseline %d, want its declared position %d", y, declaredY)
			}
			if want := max(len(oracle), wantPage+1); len(pages) != want {
				t.Errorf("%d pages, want %d", len(pages), want)
			}
			landing := pages[wantPage]
			for _, r := range landing.Runs {
				if strings.Contains(r.SourceText, "W-") && wantPage == len(oracle) {
					t.Errorf("added page %d carries row text %q above the section", wantPage+1, r.SourceText)
				}
			}
			for _, r := range landing.Rects {
				if r.Y+r.H > line {
					t.Errorf("a rect on the landing page ends at %d, below the line %d — it would overlap the section", r.Y+r.H, line)
				}
			}
			switch {
			case wantPage == 0:
				notCrossed++
			case wantPage == len(oracle):
				crossedOnLast++
			default:
				endsAboveLater++
			}
			requirePageXOfY(t, sectionBreakRender(t, doc, rows).Bytes)
		})
	}
	if notCrossed == 0 || crossedOnLast == 0 || endsAboveLater == 0 {
		t.Fatalf("coverage witness: not crossed %d, crossed %d, ends above the line on a later page %d — each must be reached", notCrossed, crossedOnLast, endsAboveLater)
	}
}

// TestSectionBreakNamedCases pins the matrix rows at measured row counts.
func TestSectionBreakNamedCases(t *testing.T) {
	doc := sectionBreakTestDoc(sectionBreakAt75, "")
	for _, c := range []struct {
		label       string
		rows        int
		legendPage  int
		totalPages  int
		addedPageOK bool
	}{
		{"not crossed: five rows end above the line on page 1", 5, 0, 1, false},
		{"crossed on the last page: the sixth row passes the line", 6, 1, 2, true},
		{"ends above the line later: rows fill page 1 and end above it on page 2", 12, 1, 2, false},
		{"crossed on page 2", 15, 2, 3, true},
	} {
		t.Run(c.label, func(t *testing.T) {
			pages, _ := sectionBreakPages(t, doc, c.rows)
			onPages, _ := legendOn(pages, "Legend")
			if len(pages) != c.totalPages || len(onPages) != 1 || onPages[0] != c.legendPage {
				t.Fatalf("pages %d, legend on %v; want %d pages with the legend on page %d", len(pages), onPages, c.totalPages, c.legendPage+1)
			}
			if c.addedPageOK {
				for _, r := range pages[c.legendPage].Runs {
					if r.SourceText != "Legend" && !strings.Contains(r.SourceText, "Page") {
						t.Errorf("added page carries %q besides the section and the footer", r.SourceText)
					}
				}
				if len(pages[c.legendPage].Rects) != 0 {
					t.Errorf("added page carries %d rects above the section", len(pages[c.legendPage].Rects))
				}
			}
			if got := requirePageXOfY(t, sectionBreakRender(t, doc, c.rows).Bytes); got != c.totalPages {
				t.Errorf("PDF has %d pages, want %d", got, c.totalPages)
			}
		})
	}
}

// TestSectionBreakNotCrossedIsByteIdentical is CAP-3: when nothing crosses the
// line, the document with the key and the same document without it render
// identical PDFs. An empty section is identical even when rows cross.
func TestSectionBreakNotCrossedIsByteIdentical(t *testing.T) {
	without := sectionBreakTestDoc("", "")
	for _, rows := range []int{0, 1, 5} {
		with := sectionBreakRender(t, sectionBreakTestDoc(sectionBreakAt75, ""), rows).Bytes
		plain := sectionBreakRender(t, without, rows).Bytes
		if !bytes.Equal(with, plain) {
			off, window := firstDivergence(with, plain)
			t.Errorf("rows=%d: the break changed a document nothing crosses (first difference at byte %d: %s)", rows, off, window)
		}
	}
	// Sanity: a crossing document does differ, so the comparison above can fail.
	if bytes.Equal(sectionBreakRender(t, sectionBreakTestDoc(sectionBreakAt75, ""), 6).Bytes, sectionBreakRender(t, without, 6).Bytes) {
		t.Fatal("a crossed break rendered identically to no break — the identity comparison proves nothing")
	}
	// Empty section: the break at 100 lies below the legend (80..92), so
	// nothing is at or below it and no page is added, crossed or not.
	for _, rows := range []int{6, 15} {
		with := sectionBreakRender(t, sectionBreakTestDoc(`, "sectionBreak": 100`, ""), rows).Bytes
		if !bytes.Equal(with, sectionBreakRender(t, without, rows).Bytes) {
			t.Errorf("rows=%d: an empty section changed the output", rows)
		}
	}
}

// TestSectionBreakFloorCountsTowardWhereContentEnds: a table's minHeight
// floor crossing the line is a crossing, though its rows do not. The legend
// sits beside the table's columns, so no floor push is involved.
func TestSectionBreakFloorCountsTowardWhereContentEnds(t *testing.T) {
	beside := func(minHeight string) string {
		doc := sectionBreakTestDoc(sectionBreakAt75, "")
		doc = strings.Replace(doc, `"headerHeight": 10,`, `"headerHeight": 10, "minHeight": `+minHeight+`,`, 1)
		return strings.Replace(doc, `"x": 0, "y": 80, "width": 180`, `"x": 165, "y": 80, "width": 15`, 1)
	}
	for _, c := range []struct {
		minHeight string
		want      int
	}{
		{"70", 0}, // floored bottom 90pt from the content top: at or above 95
		{"80", 1}, // floored bottom 100pt: crosses
	} {
		pages, _ := sectionBreakPages(t, beside(c.minHeight), 1)
		onPages, _ := legendOn(pages, "Legend")
		if len(onPages) != 1 || onPages[0] != c.want {
			t.Errorf("minHeight %s: legend on pages %v, want page %d", c.minHeight, onPages, c.want+1)
		}
	}
}

// TestSectionBreakFloorPushCountsTowardWhereContentEnds: an above-line
// element under a floored table ends where the floor pushes it. Here the
// floor (40pt) and the element's own unpushed bottom are both above the
// break at 45, and only its pushed bottom crosses it.
func TestSectionBreakFloorPushCountsTowardWhereContentEnds(t *testing.T) {
	doc := func(elementX string) string {
		pushed := `,
      {"id": "e6", "type": "text", ` + elementX + `, "y": 22, "height": 10, "value": "Pushed", "style": {"fontFamily": "latin", "fontSize": 8}}`
		d := sectionBreakTestDoc(`, "sectionBreak": 45`, pushed)
		return strings.Replace(d, `"headerHeight": 10,`, `"headerHeight": 10, "minHeight": 40,`, 1)
	}
	under, _ := sectionBreakPages(t, doc(`"x": 0, "width": 150`), 1)
	onPages, _ := legendOn(under, "Legend")
	if len(onPages) != 1 || onPages[0] != 1 || len(under) != 2 {
		t.Fatalf("element under the floor: %d pages, legend on %v; want the legend on an added page 2", len(under), onPages)
	}
	// Precondition: the same element beside the table is not pushed, and then
	// nothing crosses — so the push alone is what moved the section.
	beside, _ := sectionBreakPages(t, doc(`"x": 165, "width": 15`), 1)
	onPages, _ = legendOn(beside, "Legend")
	if len(onPages) != 1 || onPages[0] != 0 || len(beside) != 1 {
		t.Fatalf("precondition: element beside the floor: %d pages, legend on %v; want one page", len(beside), onPages)
	}
}

// TestSectionBreakSharedPageKeepsTheSectionsFramesAndPushes: when the section
// shares the above-line content's last page, its table's frame and the floor
// push on the element below that table are exactly what they are when the
// section renders on page 1.
func TestSectionBreakSharedPageKeepsTheSectionsFramesAndPushes(t *testing.T) {
	const legend = `{"id": "e5", "type": "text", "x": 0, "y": 80, "width": 180, "height": 12, "value": "Legend", "style": {"fontFamily": "latin", "fontSize": 8}}`
	doc := func(minHeight string) string {
		section := `{"id": "e6", "type": "table", "x": 0, "y": 62, "bind": "extra[]", "headerHeight": 10` + minHeight + `,
        "style": {"fontFamily": "latin", "fontSize": 8, "border": {"width": 1}},
        "columns": [{"id": "e7", "label": "C", "width": 80, "bind": "{{row.a}}"}]},
      {"id": "e8", "type": "text", "x": 0, "y": 76, "width": 150, "height": 10, "value": "Below", "style": {"fontFamily": "latin", "fontSize": 8}}`
		d := sectionBreakTestDoc(`, "sectionBreak": 60`, "")
		if !strings.Contains(d, legend) {
			t.Fatal("fixture precondition: the legend element was not found")
		}
		return strings.Replace(d, legend, section, 1)
	}
	data := func(rows int) string {
		return strings.TrimSuffix(multiRowTableData(rows, -1), "}") + `,"extra":[]}`
	}
	pagesOf := func(src string, rows int) []pagemodel.Page {
		tpl, err := ParseTemplate([]byte(src))
		if err != nil {
			t.Fatalf("ParseTemplate: %v", err)
		}
		pages, _ := barcodePages(t, tpl, data(rows))
		return pages
	}
	origin, _ := tableEnd(pagesOf(doc(`, "minHeight": 25`), 0))
	line := origin + 60000
	section := func(pg pagemodel.Page) ([]pagemodel.Rect, geom.Length, bool) {
		var rects []pagemodel.Rect
		for _, r := range pg.Rects {
			if r.Y >= line {
				rects = append(rects, r)
			}
		}
		for _, r := range pg.Runs {
			if r.SourceText == "Below" {
				return rects, r.Y, true
			}
		}
		return rects, 0, false
	}

	alone := pagesOf(doc(`, "minHeight": 25`), 0)
	shared := pagesOf(doc(`, "minHeight": 25`), 12)
	if len(alone) != 1 || len(shared) != 2 {
		t.Fatalf("precondition: %d pages alone, %d shared; want 1 and 2", len(alone), len(shared))
	}
	aloneRects, aloneY, okA := section(alone[0])
	sharedRects, sharedY, okS := section(shared[1])
	if !okA || !okS {
		t.Fatalf("precondition: the element below the section table is missing (alone %v, shared page 2 %v)", okA, okS)
	}
	hasRow := false
	for _, r := range shared[1].Runs {
		if strings.Contains(r.SourceText, "W-") {
			hasRow = true
		}
	}
	if !hasRow {
		t.Fatal("precondition: page 2 carries no above-line row, so it is not a shared page")
	}
	floored := false
	for _, r := range aloneRects {
		if r.H >= 25000 {
			floored = true
		}
	}
	if !floored {
		t.Fatalf("precondition: no section rect is floored to 25pt: %+v", aloneRects)
	}
	_, unpushedY, _ := section(pagesOf(doc(""), 0)[0])
	if unpushedY == aloneY {
		t.Fatal("precondition: the floor does not push the element below the section table")
	}

	if fmt.Sprint(sharedRects) != fmt.Sprint(aloneRects) {
		t.Errorf("section rects on the shared page differ from page 1:\nshared %+v\nalone  %+v", sharedRects, aloneRects)
	}
	if sharedY != aloneY {
		t.Errorf("the pushed element sits at %d on the shared page, want %d as on page 1", sharedY, aloneY)
	}
}

// TestSectionBreakTallSectionContinues: a section holding a growing table
// continues onto later pages under the ordinary rules, and every row is drawn
// exactly once.
func TestSectionBreakTallSectionContinues(t *testing.T) {
	second := `,
      {"id": "e6", "type": "table", "x": 0, "y": 95, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8},
        "columns": [
          {"id": "e7", "label": "C", "width": 80, "bind": "{{row.a}}"},
          {"id": "e8", "label": "D", "width": 80, "bind": "{{row.b}}"}
        ]}`
	doc := sectionBreakTestDoc(sectionBreakAt75, second)
	const rows = 12
	pages, _ := sectionBreakPages(t, doc, rows)
	if len(pages) < 3 {
		t.Fatalf("presence precondition: %d pages — the section table must run onto later pages", len(pages))
	}
	count := 0
	for _, pg := range pages {
		for _, r := range pg.Runs {
			if strings.Contains(r.SourceText, "W-") {
				count++
			}
		}
	}
	if count != 4*rows {
		t.Errorf("%d row cells drawn, want %d — each of two tables' rows exactly once", count, 4*rows)
	}
	requirePageXOfY(t, sectionBreakRender(t, doc, rows).Bytes)
}

// TestSectionBreakSplitsAKeepTogetherGroup: the break wins over keepTogether.
func TestSectionBreakSplitsAKeepTogetherGroup(t *testing.T) {
	signature := `,
      {"id": "e6", "type": "text", "x": 0, "y": 60, "width": 180, "height": 10, "value": "Signed", "keepTogether": "sig", "style": {"fontFamily": "latin", "fontSize": 8}}`
	doc := strings.Replace(sectionBreakTestDoc(sectionBreakAt75, signature), `"value": "Legend",`, `"value": "Legend", "keepTogether": "sig",`, 1)
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	idx := keepTogetherTags(tpl)
	above, below := idx.keepTogetherGroup("e6").Key, idx.keepTogetherGroup("e5").Key
	if above == below {
		t.Fatalf("both sides of a split group share the key %+v", above)
	}
	if tag, ok := keepTogetherTagOf(above); !ok || tag != "sig" {
		t.Errorf("above side's key %+v does not read back as tag sig", above)
	}
	if tag, ok := keepTogetherTagOf(below); !ok || tag != "sig" {
		t.Errorf("below side's key %+v does not read back as tag sig", below)
	}

	for _, rows := range []int{1, 6} {
		res := sectionBreakRender(t, doc, rows)
		var found []Diagnostic
		for _, d := range res.Diagnostics {
			if d.Code == DiagCodeSectionBreakSplitsKeepTogether {
				found = append(found, d)
			}
		}
		if len(found) != 1 || !strings.Contains(found[0].Message, `"sig"`) || found[0].Severity != SeverityWarning || found[0].ElementID != "e5" {
			t.Errorf("rows=%d: want one Warning naming group sig at e5, got %+v", rows, found)
		}
	}
	// Without a break the same group is whole and nothing is reported.
	res := sectionBreakRender(t, strings.Replace(doc, sectionBreakAt75, "", 1), 1)
	for _, d := range res.Diagnostics {
		if d.Code == DiagCodeSectionBreakSplitsKeepTogether {
			t.Errorf("no break, yet %+v", d)
		}
	}
}

// TestSectionBreakLoadErrors is CAP-5's refusals through the public door.
func TestSectionBreakLoadErrors(t *testing.T) {
	for _, c := range []struct {
		label, doc, code, elementID, dataPath string
	}{
		{"break at 0", sectionBreakTestDoc(`, "sectionBreak": 0`, ""), DiagCodeSectionBreakInvalid, "", "bands.content.sectionBreak"},
		{"negative break", sectionBreakTestDoc(`, "sectionBreak": -5`, ""), DiagCodeSectionBreakInvalid, "", "bands.content.sectionBreak"},
		{"break at the content height", sectionBreakTestDoc(`, "sectionBreak": 110`, ""), DiagCodeSectionBreakInvalid, "", "bands.content.sectionBreak"},
		{"break past the content height", sectionBreakTestDoc(`, "sectionBreak": 400`, ""), DiagCodeSectionBreakInvalid, "", "bands.content.sectionBreak"},
		{"a string break", sectionBreakTestDoc(`, "sectionBreak": "400"`, ""), DiagCodeSectionBreakInvalid, "", "bands.content.sectionBreak"},
		{"duplicate key", sectionBreakTestDoc(`, "sectionBreak": 75, "sectionBreak": 76`, ""), DiagCodeSectionBreakInvalid, "", "bands.content.sectionBreak"},
		{"on the page header", strings.Replace(sectionBreakTestDoc("", ""), `"pageHeader": {"elements": [], "height": 10}`, `"pageHeader": {"elements": [], "height": 10, "sectionBreak": 5}`, 1), DiagCodeSectionBreakInvalid, "", "bands.pageHeader.sectionBreak"},
		{"on the page footer", strings.Replace(sectionBreakTestDoc("", ""), `], "height": 10},
    "pageHeader"`, `], "height": 10, "sectionBreak": 5},
    "pageHeader"`, 1), DiagCodeSectionBreakInvalid, "", "bands.pageFooter.sectionBreak"},
		{"a text box across the break", sectionBreakTestDoc(`, "sectionBreak": 85`, ""), DiagCodeSectionBreakStraddled, "e5", ""},
		{"a table header across the break", sectionBreakTestDoc(`, "sectionBreak": 5`, ""), DiagCodeSectionBreakStraddled, "e1", ""},
	} {
		t.Run(c.label, func(t *testing.T) {
			if !strings.Contains(c.doc, "sectionBreak") {
				t.Fatal("fixture precondition: the document carries no sectionBreak")
			}
			_, err := ParseTemplate([]byte(c.doc))
			var re *RenderError
			if !errors.As(err, &re) {
				t.Fatalf("want a *RenderError, got %T %v", err, err)
			}
			d := re.Diagnostic
			if d.Code != c.code || d.ElementID != c.elementID || d.DataPath != c.dataPath || d.Severity != SeverityError {
				t.Errorf("got %+v, want code %s element %q data path %q", d, c.code, c.elementID, c.dataPath)
			}
		})
	}
	// The boundary is legal: a box ending exactly on the line, and a table
	// whose rows run past it.
	if _, err := ParseTemplate([]byte(sectionBreakTestDoc(`, "sectionBreak": 92`, ""))); err != nil {
		t.Errorf("a legend ending exactly on the break must load: %v", err)
	}
	if _, err := ParseTemplate([]byte(sectionBreakTestDoc(`, "sectionBreak": 10`, ""))); err != nil {
		t.Errorf("a table header ending exactly on the break must load: %v", err)
	}
}

// TestSectionBreakDocumentRoundTripsThroughThePublicDoor: saved, the document
// declares 4.1 and reparses to the same bytes.
func TestSectionBreakDocumentRoundTripsThroughThePublicDoor(t *testing.T) {
	tpl, err := ParseTemplate([]byte(strings.Replace(sectionBreakTestDoc(sectionBreakAt75, ""), `"version": "4.1"`, `"version": "1.0"`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), `"version": "4.1"`) || !strings.Contains(string(saved), `"sectionBreak": 75`) {
		t.Fatalf("saved document lacks 4.1 or its break:\n%s", saved)
	}
	again, err := ParseTemplate(saved)
	if err != nil {
		t.Fatal(err)
	}
	resaved, err := SerializeTemplate(again)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved, resaved) {
		t.Fatal("a saved section-break document does not round-trip byte-identically")
	}
}

// TestSectionBreakLeavesTheCanvasPaginationUnchanged: the canvas keeps plain
// pagination, so a document's projection with a break equals its projection
// without one in every pagination-derived field. UPDATED ON PURPOSE by the
// designer story (CAP-6): the projection now CARRIES the break — its offset
// and each content component's membership — and those two additions are the
// only difference allowed.
func TestSectionBreakLeavesTheCanvasPaginationUnchanged(t *testing.T) {
	project := func(doc string) designer.CanvasProjection {
		t.Helper()
		tpl, err := ParseTemplate([]byte(doc))
		if err != nil {
			t.Fatal(err)
		}
		projection, err := canvasWithTextPaint(tpl, testShippedFontSet())
		if err != nil {
			t.Fatalf("CanvasWithTextPaint: %v", err)
		}
		return projection
	}
	with, without := project(sectionBreakTestDoc(sectionBreakAt75, "")), project(sectionBreakTestDoc("", ""))
	if with.SectionBreak == nil || *with.SectionBreak != 75000 {
		t.Fatalf("the projection must carry the break offset 75000, got %v", with.SectionBreak)
	}
	if without.SectionBreak != nil {
		t.Fatalf("a document without a break must project no sectionBreak, got %v", *without.SectionBreak)
	}
	members := map[string]bool{}
	for index, component := range with.Components {
		if component.Band != bandContent {
			if component.BelowSectionBreak != nil {
				t.Fatalf("%s is not a content component and must carry no membership", component.ID)
			}
			continue
		}
		if component.BelowSectionBreak == nil {
			t.Fatalf("content component %s carries no membership in a document with a break", component.ID)
		}
		members[component.ID] = *component.BelowSectionBreak
		with.Components[index].BelowSectionBreak = nil
	}
	if members["e1"] || !members["e5"] {
		t.Fatalf("membership = %v, want the table above and the legend below", members)
	}
	for _, component := range without.Components {
		if component.BelowSectionBreak != nil {
			t.Fatalf("a document without a break projected membership on %s", component.ID)
		}
	}
	with.SectionBreak = nil
	a, err := json.Marshal(with)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(without)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		off, window := firstDivergence(a, b)
		t.Fatalf("the break changed the canvas projection beyond its offset and membership (first difference at byte %d: %s)", off, window)
	}
}

// TestSectionBreakStatementRendersIdenticallyInAFreshProcess renders the
// golden in a fresh OS process and compares it with this process's render.
func TestSectionBreakStatementRendersIdenticallyInAFreshProcess(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), subprocessSectionBreakStatementEnvVar+"=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("subprocess render failed: %v\n%s", err, stderr.String())
	}
	if here := renderSectionBreakStatement(t); !bytes.Equal(stdout.Bytes(), here) {
		off, window := firstDivergence(stdout.Bytes(), here)
		t.Fatalf("fresh-process render differs at byte %d: %s", off, window)
	}
}

// sectionBreakWarningDoc is sectionBreakTestDoc with a second table, e6, at
// y 95 — below the break, so it belongs to the section with the legend —
// bound to wall[]. footer adds a count footer to its first column.
func sectionBreakWarningDoc(footer bool) string {
	foot := ""
	if footer {
		foot = `, "footer": "count"`
	}
	return sectionBreakTestDoc(sectionBreakAt75, `,
      {"id": "e6", "type": "table", "x": 0, "y": 95, "bind": "wall[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8},
        "columns": [
          {"id": "e7", "label": "C", "width": 80, "bind": "{{row.a}}"`+foot+`},
          {"id": "e8", "label": "D", "width": 80, "bind": "{{row.b}}"}
        ]}`)
}

// sectionBreakWarningData binds six items[] rows (enough to cross the line,
// so the section moves to an added page) and one wall[] row whose first cell
// holds `words` short words.
func sectionBreakWarningData(words int) string {
	items := make([]tableRowJSON, 6)
	for i := range items {
		items[i] = tableRowJSON{A: fmt.Sprintf("R%dW-x", i), B: fmt.Sprintf("R%dW-b", i)}
	}
	var wall strings.Builder
	for w := 0; w < words; w++ {
		fmt.Fprintf(&wall, "Q%02d ", w)
	}
	b, err := json.Marshal(map[string]any{"items": items, "wall": []tableRowJSON{{A: wall.String(), B: "wall"}}})
	if err != nil {
		panic(err)
	}
	return string(b)
}

// sectionBreakWarningPages renders doc against data, requires the section to
// have moved to document page 2 and the wall row to start on document page 3
// (the section's own SECOND page), and returns the page count and diagnostics.
func sectionBreakWarningPages(t *testing.T, doc, data string) (int, []Diagnostic) {
	t.Helper()
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	pages, diags := barcodePages(t, tpl, data)
	if on, _ := legendOn(pages, "Legend"); len(on) != 1 || on[0] != 1 {
		t.Fatalf("presence precondition: the legend is on pages %v, want only index 1 — the section must start on an added page so its own page index differs from the document's", on)
	}
	wallOn := -1
	for p, pg := range pages {
		for _, r := range pg.Runs {
			if strings.Contains(r.SourceText, "Q00") {
				wallOn = p
				break
			}
		}
		if wallOn >= 0 {
			break
		}
	}
	if wallOn != 2 {
		t.Fatalf("presence precondition: the wall row starts on page index %d, want 2 (document page 3, the section's own page 2)", wallOn)
	}
	return len(pages), diags
}

func sectionBreakDiagsWithCode(diags []Diagnostic, code string) []Diagnostic {
	var out []Diagnostic
	for _, d := range diags {
		if d.Code == code {
			out = append(out, d)
		}
	}
	return out
}

// TestSectionBreakClippedRowWarningNamesTheDocumentPage: a row clipped inside
// a moved section is reported on the DOCUMENT page it is drawn on (3), not
// the section's own page number (2).
func TestSectionBreakClippedRowWarningNamesTheDocumentPage(t *testing.T) {
	_, diags := sectionBreakWarningPages(t, sectionBreakWarningDoc(false), sectionBreakWarningData(60))
	got := sectionBreakDiagsWithCode(diags, DiagCodeTableRowClippedHeight)
	if len(got) != 1 {
		t.Fatalf("want one %s, got %+v", DiagCodeTableRowClippedHeight, diags)
	}
	if got[0].ElementID != "e6" || !strings.Contains(got[0].Message, "placed alone on page 3 and CLIPPED") {
		t.Errorf("clipped-row warning does not name document page 3 for e6: %+v", got[0])
	}
}

// TestSectionBreakHeaderRepeatAndFooterOrphanWarningsInTheSection: the
// section's header-repeat suppression names the DOCUMENT page (3), and the
// section's own footer-orphan diagnostics are reported at all.
func TestSectionBreakHeaderRepeatAndFooterOrphanWarningsInTheSection(t *testing.T) {
	_, diags := sectionBreakWarningPages(t, sectionBreakWarningDoc(true), sectionBreakWarningData(40))
	sup := sectionBreakDiagsWithCode(diags, DiagCodeTableHeaderRepeatSuppressed)
	if len(sup) != 1 {
		t.Fatalf("want one %s, got %+v", DiagCodeTableHeaderRepeatSuppressed, diags)
	}
	if sup[0].ElementID != "e6" || !strings.Contains(sup[0].Message, "could not be drawn on page 3 —") {
		t.Errorf("header-repeat warning does not name document page 3 for e6: %+v", sup[0])
	}
	orphan := sectionBreakDiagsWithCode(diags, DiagCodeTableFooterOrphanSuppressed)
	if len(orphan) != 1 || orphan[0].ElementID != "e6" || orphan[0].Severity != SeverityWarning {
		t.Errorf("want one %s Warning for e6 from the section's pagination, got %+v", DiagCodeTableFooterOrphanSuppressed, diags)
	}
}

// ---------------------------------------------------------------------------
// spec-section-break CAP-7: THE UNANCHORED BREAK.

const sectionBreakUnanchoredAt75 = `, "sectionBreak": 75, "sectionBreakAnchor": false`

// sectionBreakContentBottom is the content window's bottom in page space for
// sectionBreakTestDoc: its content band is 110pt tall.
func sectionBreakContentBottom(origin geom.Length) geom.Length { return origin + 110000 }

// TestSectionBreakUnanchoredLandingMatchesTheOracleForEveryRowCount is CAP-7
// over every row count from 0 to 20, against the table-only render: ending
// at or above the line on page 1, the legend is at its declared y and the PDF
// is byte-identical to the anchored one; otherwise it follows E — moved by
// exactly E - line on the last page (down, or up when a later page's rows end
// above the line), or it is on an added page with the line at the window top.
// A push is chosen exactly for the smaller crossings on each page.
func TestSectionBreakUnanchoredLandingMatchesTheOracleForEveryRowCount(t *testing.T) {
	doc := sectionBreakTestDoc(sectionBreakUnanchoredAt75, "")
	anchored := sectionBreakTestDoc(sectionBreakAt75, "")
	tableOnly := sectionBreakWithout(sectionBreakTestDoc("", ""), "e5")
	reference, _ := sectionBreakPages(t, sectionBreakTestDoc("", ""), 0)
	_, declaredY := legendOn(reference, "Legend")
	var notCrossed, pushed, pulled, moved int
	// The largest pushed crossing and the smallest moved one, per last page.
	maxPushed, minMoved := map[int]geom.Length{}, map[int]geom.Length{}
	for rows := 0; rows <= 20; rows++ {
		t.Run(fmt.Sprintf("rows=%d", rows), func(t *testing.T) {
			oracle, _ := sectionBreakPages(t, tableOnly, rows)
			origin, end := tableEnd(oracle)
			line := origin + 75000
			last := len(oracle) - 1

			pages, _ := sectionBreakPages(t, doc, rows)
			onPages, y := legendOn(pages, "Legend")
			if len(onPages) != 1 {
				t.Fatalf("legend drawn on pages %v, want exactly one page", onPages)
			}
			switch {
			case end <= line && last == 0:
				notCrossed++
				if onPages[0] != last || y != declaredY || len(pages) != len(oracle) {
					t.Fatalf("not crossed: legend on page %d at %d of %d pages, want page %d at %d of %d", onPages[0]+1, y, len(pages), last+1, declaredY, len(oracle))
				}
				if a, b := sectionBreakRender(t, doc, rows).Bytes, sectionBreakRender(t, anchored, rows).Bytes; !bytes.Equal(a, b) {
					off, window := firstDivergence(a, b)
					t.Fatalf("not crossed, yet unanchored differs from anchored at byte %d: %s", off, window)
				}
			case onPages[0] == last:
				if end <= line {
					pulled++
				} else {
					pushed++
				}
				if want := declaredY + (end - line); y != want {
					t.Fatalf("follows E: legend baseline %d, want %d (declared %d + E %d - line %d)", y, want, declaredY, end, line)
				}
				if len(pages) != len(oracle) {
					t.Fatalf("follows E: %d pages, want %d", len(pages), len(oracle))
				}
				for _, r := range pages[last].Rects {
					if r.Y+r.H > end {
						t.Errorf("a rect on the legend's page ends at %d, below the content above's end %d", r.Y+r.H, end)
					}
				}
				maxPushed[last] = max(maxPushed[last], end)
			default:
				moved++
				if onPages[0] != last+1 || y != declaredY-75000 || len(pages) != len(oracle)+1 {
					t.Fatalf("moved: legend on page %d at %d of %d pages, want page %d at %d (line at the window top) of %d", onPages[0]+1, y, len(pages), last+2, declaredY-75000, len(oracle)+1)
				}
				for _, r := range pages[last+1].Rects {
					t.Errorf("the added page carries a rect at %d above the section", r.Y)
				}
				if v, ok := minMoved[last]; !ok || end < v {
					minMoved[last] = end
				}
			}
			if y > sectionBreakContentBottom(origin) {
				t.Errorf("legend baseline %d is below the content bottom %d", y, sectionBreakContentBottom(origin))
			}
			requirePageXOfY(t, sectionBreakRender(t, doc, rows).Bytes)
		})
	}
	if notCrossed == 0 || pushed == 0 || pulled == 0 || moved == 0 {
		t.Fatalf("coverage witness: not crossed %d, pushed %d, pulled up on a later page %d, moved %d — each must be reached", notCrossed, pushed, pulled, moved)
	}
	for page, most := range maxPushed {
		if least, ok := minMoved[page]; ok && most >= least {
			t.Errorf("on last page %d a crossing ending at %d was pushed but one ending at %d was moved", page+1, most, least)
		}
	}
}

// TestSectionBreakUnanchoredNamedCases pins the I/O matrix rows at measured
// row counts.
func TestSectionBreakUnanchoredNamedCases(t *testing.T) {
	doc := sectionBreakTestDoc(sectionBreakUnanchoredAt75, "")
	for _, c := range []struct {
		label      string
		rows       int
		legendPage int
		totalPages int
		pushed     bool
	}{
		{"not crossed: five rows end above the line on page 1", 5, 0, 1, false},
		{"pushed, fits: the seventh row passes the line on page 1", 7, 0, 1, true},
		{"pushed, no room: rows end near the window bottom", 9, 1, 2, false},
		// The owner's report: rows fill page 1 and end on page 2 ABOVE the
		// line's height; the legend follows them up rather than keeping its
		// page-1 y (asserted exactly by the oracle test).
		{"later page, follows the rows up: rows end above the line on page 2", 12, 1, 2, true},
		{"later page, pushed: rows fill page 1 and cross on page 2", 15, 1, 2, true},
		{"later page, no room: rows end near page 2's bottom", 18, 2, 3, false},
	} {
		t.Run(c.label, func(t *testing.T) {
			pages, _ := sectionBreakPages(t, doc, c.rows)
			onPages, _ := legendOn(pages, "Legend")
			if len(pages) != c.totalPages || len(onPages) != 1 || onPages[0] != c.legendPage {
				t.Fatalf("pages %d, legend on %v; want %d pages with the legend on page %d", len(pages), onPages, c.totalPages, c.legendPage+1)
			}
			hasRow := false
			for _, r := range pages[c.legendPage].Runs {
				if strings.Contains(r.SourceText, "W-") {
					hasRow = true
				}
			}
			// pushed means the legend shares its page with the rows.
			if hasRow != (c.pushed || c.rows == 5) {
				t.Errorf("the legend's page carries rows = %v, want %v", hasRow, c.pushed || c.rows == 5)
			}
			if c.rows == 12 {
				// The owner's report, pinned exactly: rows ending ABOVE the
				// line on page 2 put the legend at declared y + (E - line).
				oracle, _ := sectionBreakPages(t, sectionBreakWithout(sectionBreakTestDoc("", ""), "e5"), c.rows)
				origin, end := tableEnd(oracle)
				line := origin + 75000
				if len(oracle) != 2 || end > line {
					t.Fatalf("precondition: the rows end at %d on page %d, want above the line %d on page 2", end, len(oracle), line)
				}
				reference, _ := sectionBreakPages(t, sectionBreakTestDoc("", ""), 0)
				_, declaredY := legendOn(reference, "Legend")
				if _, y := legendOn(pages, "Legend"); y != declaredY+(end-line) {
					t.Errorf("legend baseline %d, want %d — directly after the rows, not its page-1 y %d", y, declaredY+(end-line), declaredY)
				}
			}
			if got := requirePageXOfY(t, sectionBreakRender(t, doc, c.rows).Bytes); got != c.totalPages {
				t.Errorf("PDF has %d pages, want %d", got, c.totalPages)
			}
		})
	}
}

// TestSectionBreakUnanchoredNotCrossedIsByteIdentical is CAP-3 for Anchor
// off: the unanchored break, the anchored break and no break render the same
// PDF when nothing crosses the line.
func TestSectionBreakUnanchoredNotCrossedIsByteIdentical(t *testing.T) {
	for _, rows := range []int{0, 1, 5} {
		plain := sectionBreakRender(t, sectionBreakTestDoc("", ""), rows).Bytes
		for _, keys := range []string{sectionBreakAt75, sectionBreakUnanchoredAt75} {
			if with := sectionBreakRender(t, sectionBreakTestDoc(keys, ""), rows).Bytes; !bytes.Equal(with, plain) {
				t.Errorf("rows=%d %s: the break changed a document nothing crosses", rows, keys)
			}
		}
	}
}

// TestSectionBreakUnanchoredFloorCrosses: a minHeight floor past the line
// pushes the section from the floor's bottom, though no row crosses.
func TestSectionBreakUnanchoredFloorCrosses(t *testing.T) {
	doc := sectionBreakTestDoc(sectionBreakUnanchoredAt75, "")
	doc = strings.Replace(doc, `"headerHeight": 10,`, `"headerHeight": 10, "minHeight": 88,`, 1)
	doc = strings.Replace(doc, `"x": 0, "y": 80, "width": 180`, `"x": 165, "y": 80, "width": 15`, 1)
	reference, _ := sectionBreakPages(t, strings.Replace(doc, `, "sectionBreakAnchor": false`, "", 1), 0)
	_, declaredY := legendOn(reference, "Legend")
	pages, _ := sectionBreakPages(t, doc, 1)
	onPages, y := legendOn(pages, "Legend")
	if len(pages) != 1 || len(onPages) != 1 || onPages[0] != 0 {
		t.Fatalf("pages %d, legend on %v; want the legend pushed on page 1", len(pages), onPages)
	}
	// The floor runs 88pt from the table's top at 0, 13pt past the line at
	// 75, and the legend (80..92) pushed by 13pt still ends above 110.
	if y != declaredY+13000 {
		t.Fatalf("legend baseline %d, want %d — pushed 13pt from the floor's bottom", y, declaredY+13000)
	}
}

// TestSectionBreakUnanchoredTallSectionStartsAtTheWindowTop: a section holding
// a growing table, moved to a new page, starts at the top of that page's
// content window and continues onto later pages, each row drawn once.
func TestSectionBreakUnanchoredTallSectionStartsAtTheWindowTop(t *testing.T) {
	second := `,
      {"id": "e6", "type": "table", "x": 0, "y": 95, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8, "border": {"width": 1}},
        "columns": [
          {"id": "e7", "label": "C", "width": 80, "bind": "{{row.a}}"},
          {"id": "e8", "label": "D", "width": 80, "bind": "{{row.b}}"}
        ]}`
	doc := sectionBreakTestDoc(sectionBreakUnanchoredAt75, second)
	// Nine rows cross the line on page 1, and the section's table (its
	// header 20pt below the line, then nine rows) is taller than what is left.
	const rows = 9
	pages, _ := sectionBreakPages(t, doc, rows)
	if len(pages) < 3 {
		t.Fatalf("presence precondition: %d pages — the section table must run onto later pages", len(pages))
	}
	origin, _ := tableEnd(pages)
	onPages, legendY := legendOn(pages, "Legend")
	if len(onPages) != 1 {
		t.Fatalf("legend on %v", onPages)
	}
	reference, _ := sectionBreakPages(t, sectionBreakTestDoc("", ""), 0)
	_, declaredY := legendOn(reference, "Legend")
	if legendY != declaredY-75000 {
		t.Errorf("legend baseline %d, want %d — the line at the window top", legendY, declaredY-75000)
	}
	count := 0
	for p, pg := range pages {
		for _, r := range pg.Runs {
			if strings.Contains(r.SourceText, "W-") {
				count++
			}
		}
		for _, r := range pg.Rects {
			if r.Y < origin || r.Y+r.H > sectionBreakContentBottom(origin) {
				t.Errorf("page %d: a rect runs from %d to %d, outside the content window %d..%d", p+1, r.Y, r.Y+r.H, origin, sectionBreakContentBottom(origin))
			}
		}
	}
	if count != 4*rows {
		t.Errorf("%d row cells drawn, want %d — each of two tables' rows exactly once", count, 4*rows)
	}
	// The section's table header ("C"/"D"): on the legend's page it is 20pt
	// below the window top (declared 95, line 75, line at the window top); on
	// every continuation page its repeat sits at the window top, so its
	// baseline is exactly 20pt higher than on the first page.
	headerBaseline := func(pg pagemodel.Page) (c, d geom.Length, ok bool) {
		var okC, okD bool
		for _, r := range pg.Runs {
			if r.SourceText == "C" {
				c, okC = r.Y, true
			}
			if r.SourceText == "D" {
				d, okD = r.Y, true
			}
		}
		return c, d, okC && okD
	}
	firstC, firstD, ok := headerBaseline(pages[onPages[0]])
	if !ok {
		t.Fatal("presence precondition: the section table's header is not on the legend's page")
	}
	continuations := 0
	for p := onPages[0] + 1; p < len(pages); p++ {
		c, d, ok := headerBaseline(pages[p])
		if !ok {
			t.Errorf("continuation page %d draws no repeated C/D header", p+1)
			continue
		}
		continuations++
		if c != firstC-20000 || d != firstD-20000 {
			t.Errorf("continuation page %d: repeated header baselines %d/%d, want %d/%d — the window top, as on the first page", p+1, c, d, firstC-20000, firstD-20000)
		}
	}
	if continuations == 0 {
		t.Fatal("presence precondition: the section table has no continuation page")
	}
	requirePageXOfY(t, sectionBreakRender(t, doc, rows).Bytes)
}

// TestSectionBreakUnanchoredClippedAboveLineMovesToANewPage: when the content
// above the line is clipped on its last page it has no usable end, so an
// unanchored section finds no room there and starts a new page with the line
// at the window top.
func TestSectionBreakUnanchoredClippedAboveLineMovesToANewPage(t *testing.T) {
	var long strings.Builder
	for w := 0; w < 600; w++ {
		fmt.Fprintf(&long, "Q%03d ", w)
	}
	data, err := json.Marshal(map[string]any{"items": []tableRowJSON{{A: long.String(), B: "R0W-b"}}})
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := ParseTemplate([]byte(sectionBreakTestDoc(sectionBreakUnanchoredAt75, "")))
	if err != nil {
		t.Fatal(err)
	}
	pages, diags := barcodePages(t, tpl, string(data))
	if got := sectionBreakDiagsWithCode(diags, DiagCodeTableRowClippedHeight); len(got) != 1 {
		t.Fatalf("want one %s, got %+v", DiagCodeTableRowClippedHeight, diags)
	}
	clippedOn := -1
	for p, pg := range pages {
		for _, r := range pg.Runs {
			if strings.HasPrefix(r.SourceText, "Q000") {
				clippedOn = p
			}
		}
	}
	if clippedOn < 0 {
		t.Fatal("presence precondition: the clipped row is not drawn")
	}
	reference, _ := sectionBreakPages(t, sectionBreakTestDoc("", ""), 0)
	origin, _ := tableEnd(reference)
	_, declaredY := legendOn(reference, "Legend")
	onPages, y := legendOn(pages, "Legend")
	if len(onPages) != 1 || onPages[0] != clippedOn+1 {
		t.Fatalf("legend on pages %v, want only page %d, the page after the clipped row", onPages, clippedOn+2)
	}
	if y != declaredY-75000 {
		t.Errorf("legend baseline %d, want %d — the line at the window top", y, declaredY-75000)
	}
	if y < origin {
		t.Errorf("legend baseline %d is above the content origin %d", y, origin)
	}
	res, err := Render(tpl, Data(string(data)), nil, testShippedFontSet())
	if err != nil {
		t.Fatal(err)
	}
	if got := requirePageXOfY(t, res.Bytes); got != len(pages) {
		t.Errorf("PDF has %d pages, page model %d", got, len(pages))
	}
}

// TestSectionBreakUnanchoredClippedRowIsCutAtTheContentBottom: a row clipped
// inside a section moved to a new page is cut at the window's bottom, not at
// a bottom still carrying the section's move.
func TestSectionBreakUnanchoredClippedRowIsCutAtTheContentBottom(t *testing.T) {
	for _, keys := range []string{sectionBreakAt75, sectionBreakUnanchoredAt75} {
		doc := strings.Replace(sectionBreakWarningDoc(false), sectionBreakAt75, keys, 1)
		doc = strings.Replace(doc, `"style": {"fontFamily": "latin", "fontSize": 8},
        "columns": [
          {"id": "e7"`, `"style": {"fontFamily": "latin", "fontSize": 8, "background": "#EEEEEE"},
        "columns": [
          {"id": "e7"`, 1)
		tpl, err := ParseTemplate([]byte(doc))
		if err != nil {
			t.Fatal(err)
		}
		pages, diags := barcodePages(t, tpl, sectionBreakWarningData(60))
		if len(sectionBreakDiagsWithCode(diags, DiagCodeTableRowClippedHeight)) != 1 {
			t.Fatalf("%s: precondition: want one clipped row, got %+v", keys, diags)
		}
		origin, _ := tableEnd(pages)
		bottom, found := geom.Length(0), false
		for _, pg := range pages[1:] {
			for _, r := range pg.Rects {
				if r.Y+r.H > bottom {
					bottom, found = r.Y+r.H, true
				}
			}
		}
		if !found || bottom != sectionBreakContentBottom(origin) {
			t.Errorf("%s: the section's lowest rect ends at %d, want the content bottom %d", keys, bottom, sectionBreakContentBottom(origin))
		}
	}
}
