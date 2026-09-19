package folio8

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
)

// SPEC-table-rules' I/O matrix, one test per row.
//
// EVERY DOCUMENT HERE IS THE SAME THREE-COLUMN TABLE and the cases differ only
// in the keys under test, so a rect count or a line position that moves can be
// attributed to the key that moved it rather than to the fixture.
//
// THE RECT ORDER collectBandTableRuns emits, which every assertion below reads
// against: the header row's cells (one per column), then each data row's cells,
// then the footer's if there is one, then — LAST — the table's own box and the
// interior rules, in one source. The box is last so the frame and the rules
// draw OVER the row fills rather than under them.
func ruledTableDoc(tableKeys string, columns int) string {
	cols := make([]string, 0, columns)
	for i := 0; i < columns; i++ {
		cols = append(cols, fmt.Sprintf(`{"id": "e%d", "label": "H%d", "width": 60, "bind": "{{row.a}}"}`, i+2, i))
	}
	return `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "as": "row", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8}` + tableKeys + `,
        "columns": [` + strings.Join(cols, ",\n          ") + `]}
    ]},
    "pageFooter": {"elements": [], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"latin": ["Noto Sans"]},
  "locale": "en",
  "nextId": 9,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 300, "height": 400}},
  "utcOffset": "+00:00",
  "version": "3.1"
}
`
}

// verticalRules / horizontalRules select the two line primitives a rules block
// emits. A rule is a zero-WIDTH rect stroked on its left edge (a vertical) or a
// zero-HEIGHT rect stroked on its top edge (a horizontal) — the line primitive
// internal/pdf/rectdoc.go already had, never a new page-model kind.
func verticalRules(rects []pagemodel.Rect) []pagemodel.Rect {
	var out []pagemodel.Rect
	for _, r := range rects {
		if r.HasStroke && r.W == 0 && r.Edges.Left && !r.Edges.Top {
			out = append(out, r)
		}
	}
	return out
}

func horizontalRules(rects []pagemodel.Rect) []pagemodel.Rect {
	var out []pagemodel.Rect
	for _, r := range rects {
		if r.HasStroke && r.H == 0 && r.Edges.Top && !r.Edges.Left {
			out = append(out, r)
		}
	}
	return out
}

// MATRIX ROW "Ruled columns only": three columns, two interior boundaries, two
// vertical lines, none on the table's own left or right edge, and no
// horizontals at all.
func TestRulesBetweenColumnsDrawsOneLinePerInteriorBoundary(t *testing.T) {
	pages := tablePagesForTest(t, ruledTableDoc(`, "rules": {"between": ["columns"], "width": 0.5}`, 3), `{"items": [{"a":"x"},{"a":"y"}]}`)
	rects := pages[0].Rects
	verticals := verticalRules(rects)
	if len(verticals) != 2 {
		t.Fatalf("got %d vertical rule(s) for a three-column table, want 2 — one per INTERIOR boundary, and never one on the table's own edge", len(verticals))
	}
	// The two interior boundaries of three 60pt columns starting at x=0.
	for i, want := range []geom.Length{60_000, 120_000} {
		if verticals[i].X != want {
			t.Errorf("vertical rule %d is at x=%d, want %d (the boundary between columns %d and %d)", i, verticals[i].X, want, i, i+1)
		}
	}
	// NEITHER EDGE. 0 is the table's left edge and 180,000 its right; a rule at
	// either would be the line the box's own border draws.
	for i, r := range verticals {
		if r.X == 0 || r.X == 180_000 {
			t.Errorf("vertical rule %d sits on the table's own edge (x=%d) — the perimeter belongs to style.border, and the box is the only thing that draws it", i, r.X)
		}
	}
	if got := len(horizontalRules(rects)); got != 0 {
		t.Errorf("got %d horizontal rule(s) with between: [\"columns\"], want 0", got)
	}
}

// MATRIX ROW "Frame, not grid": a table's own `style.border` strokes ONE rect
// — the table's box — and no cell rect carries it.
//
// This is the row the rest of the matrix is built on, and it is the whole of
// SPEC-table-rules §1. Before it, a table was excluded by name from the element
// box painter and its border was stamped onto every header, data and footer
// cell, so a 1pt border printed a full grid whose weight followed the row
// count. The assertion is therefore a COUNT and a NEGATIVE, not a position: one
// stroked rect, and every other rect unstroked. A position-only assertion would
// stay green if the frame were drawn correctly AND every cell kept its stroke.
func TestATableStyleBorderStrokesTheBoxAndNoCell(t *testing.T) {
	pages := tablePagesForTest(t, ruledTableDoc(`, "style": {"fontFamily": "latin", "fontSize": 8, "border": {"width": 1}}`, 3), `{"items": [{"a":"x"},{"a":"y"}]}`)
	rects := pages[0].Rects
	stroked := make([]pagemodel.Rect, 0, len(rects))
	for _, r := range rects {
		if r.HasStroke {
			stroked = append(stroked, r)
		}
	}
	if len(stroked) != 1 {
		t.Fatalf("got %d stroked rect(s) for a table declaring style.border, want exactly 1 — the box. More than one means the border is still cell chrome and the frame is a grid", len(stroked))
	}
	box := stroked[0]
	// Three 60pt columns from x=0: the box spans the table's full width.
	if box.X != 0 || box.W != 180_000 {
		t.Errorf("box is x=%d w=%d, want x=0 w=180000 — the frame must span the whole table, not one cell", box.X, box.W)
	}
	// Header (10pt) plus two data rows: taller than the header alone, which is
	// what a box collapsed onto the header row would measure.
	if box.H <= 10_000 {
		t.Errorf("box height is %d, want more than the 10000 header — the frame must enclose the rows, not just the header", box.H)
	}
	if !box.Edges.Top || !box.Edges.Right || !box.Edges.Bottom || !box.Edges.Left {
		t.Errorf("box edges are %+v, want all four — an undeclared `edges` is the whole perimeter", box.Edges)
	}
	if box.StrokeWidth != 1000 {
		t.Errorf("box StrokeWidth = %d, want 1000 (1pt)", box.StrokeWidth)
	}
}

// MATRIX ROW "Line drawn once": each interior boundary carries EXACTLY one
// stroke, not one per adjacent cell. This is the defect the boundary
// vocabulary replaces — under the old cell-chrome model two neighbours each
// owned the edge they shared and nothing de-duplicated them.
func TestEveryInteriorBoundaryCarriesExactlyOneStroke(t *testing.T) {
	pages := tablePagesForTest(t, ruledTableDoc(`, "rules": {"between": ["columns", "rows"]}`, 3), `{"items": [{"a":"x"},{"a":"y"},{"a":"z"}]}`)
	rects := pages[0].Rects
	atX := map[geom.Length]int{}
	for _, r := range verticalRules(rects) {
		atX[r.X]++
	}
	for x, n := range atX {
		if n != 1 {
			t.Errorf("the column boundary at x=%d carries %d strokes, want exactly 1", x, n)
		}
	}
	atY := map[geom.Length]int{}
	for _, r := range horizontalRules(rects) {
		atY[r.Y]++
	}
	for y, n := range atY {
		if n != 1 {
			t.Errorf("the row boundary at y=%d carries %d strokes, want exactly 1", y, n)
		}
	}
	// Three rows and a header make THREE interior row boundaries: header/row0,
	// row0/row1, row1/row2. The last row's bottom is a boundary between a row
	// and nothing, so it carries none.
	if got := len(atY); got != 3 {
		t.Errorf("got %d distinct row boundaries for a header plus three rows, want 3 — the bottom-most row has no bottom rule", got)
	}
}

// MATRIX ROW "Header boundary": `rows` is declared AND headerStyle.border
// strokes its bottom, so that one coordinate carries the header's line and not
// a second one from the rules.
func TestTheHeadersOwnBottomBorderWinsOverARowRule(t *testing.T) {
	withHeaderBorder := tablePagesForTest(t,
		ruledTableDoc(`, "headerStyle": {"border": {"edges": ["bottom"], "width": 1}}, "rules": {"between": ["rows"]}`, 3),
		`{"items": [{"a":"x"},{"a":"y"}]}`)
	// Without the header's border the SAME document rules that boundary, which
	// is what makes the skip observable rather than a coincidence of counting.
	withoutHeaderBorder := tablePagesForTest(t,
		ruledTableDoc(`, "rules": {"between": ["rows"]}`, 3),
		`{"items": [{"a":"x"},{"a":"y"}]}`)

	skipped := horizontalRules(withHeaderBorder[0].Rects)
	drawn := horizontalRules(withoutHeaderBorder[0].Rects)
	if len(drawn) != len(skipped)+1 {
		t.Fatalf("with the header's bottom border the rules draw %d line(s) and without it %d; want exactly one fewer with it — the header's border wins that boundary", len(skipped), len(drawn))
	}
	headerBottom := withHeaderBorder[0].Rects[0].Y + withHeaderBorder[0].Rects[0].H
	for _, r := range skipped {
		if r.Y == headerBottom {
			t.Errorf("a rule was drawn at the header boundary (y=%d) that headerStyle.border already strokes — two strokes at one coordinate is the defect this model removes", r.Y)
		}
	}
}

// MATRIX ROW "Rules reach the floor": the verticals run from the box top to the
// box BOTTOM, through the empty area minHeight creates.
func TestColumnRulesRunToTheBoxBottomThroughTheRuledArea(t *testing.T) {
	pages := tablePagesForTest(t, ruledTableDoc(`, "rules": {"between": ["columns"]}, "minHeight": 200`, 3), `{"items": [{"a":"x"},{"a":"y"}]}`)
	rects := pages[0].Rects
	// The table's top is its first (header) cell's top. A 200pt floor puts
	// the slice bottom 200,000mp below it — well past the two rows.
	top := rects[0].Y
	verticals := verticalRules(rects)
	if len(verticals) != 2 {
		t.Fatalf("got %d vertical rules, want 2", len(verticals))
	}
	for i, r := range verticals {
		if r.Y != top || r.H != 200_000 {
			t.Errorf("vertical rule %d spans %d..%d; want the slice's own %d..%d — a rule that stopped at the last row would leave the form's columns hanging in mid-air", i, r.Y, r.Y+r.H, top, top+200_000)
		}
	}
}

// MATRIX ROW "Floor below content": minHeight never shrinks or clips.
func TestAFloorBelowTheContentChangesNothing(t *testing.T) {
	with := tablePagesForTest(t, ruledTableDoc(`, "style": {"fontFamily": "latin", "fontSize": 8, "border": {"width": 1}}, "minHeight": 1`, 3), `{"items": [{"a":"x"},{"a":"y"}]}`)
	without := tablePagesForTest(t, ruledTableDoc(`, "style": {"fontFamily": "latin", "fontSize": 8, "border": {"width": 1}}`, 3), `{"items": [{"a":"x"},{"a":"y"}]}`)
	if len(with[0].Rects) != len(without[0].Rects) {
		t.Fatalf("declaring a floor below the content changed the rect count: %d with, %d without", len(with[0].Rects), len(without[0].Rects))
	}
	for i := range with[0].Rects {
		if with[0].Rects[i] != without[0].Rects[i] {
			t.Errorf("rect %d moved when a 1pt floor was declared under content far taller than it: %+v vs %+v", i, with[0].Rects[i], without[0].Rects[i])
		}
	}
}

// MATRIX ROW "Floor too tall": a LOAD error naming the element, with
// SPEC-table-rules' own code.
func TestAMinHeightTallerThanTheContentWindowIsRefusedAtLoad(t *testing.T) {
	// This fixture's page is 400pt tall with 10pt margins and two 10pt bands,
	// so its content window is 360pt. 500 is past it and 300 is inside it.
	if _, err := ParseTemplate([]byte(ruledTableDoc(`, "minHeight": 300`, 3))); err != nil {
		t.Fatalf("presence precondition: a floor INSIDE the content window must load, got %v", err)
	}
	_, err := ParseTemplate([]byte(ruledTableDoc(`, "minHeight": 500`, 3)))
	if err == nil {
		t.Fatal("a minHeight taller than the content window must be refused at load — a table whose floor is taller than the window can never be placed on any page")
	}
	var renderErr *RenderError
	if !errors.As(err, &renderErr) {
		t.Fatalf("errors.As(*RenderError) failed: %T: %v", err, err)
	}
	if renderErr.Diagnostic.Code != DiagCodeTableMinHeightUnplaceable {
		t.Errorf("Code = %q, want %q", renderErr.Diagnostic.Code, DiagCodeTableMinHeightUnplaceable)
	}
	if renderErr.Diagnostic.ElementID != "e1" {
		t.Errorf("ElementID = %q, want the table element e1 — the refusal must NAME the element", renderErr.Diagnostic.ElementID)
	}
}

// A NON-POSITIVE FLOOR IS A DIFFERENT REFUSAL, and it is the loader's: `max(0,
// content)` IS `content`, which is what omitting the key already means, so a
// zero floor is a second spelling of absence.
func TestANonPositiveMinHeightIsRefusedAtLoad(t *testing.T) {
	for _, value := range []string{"0", "-5"} {
		if _, err := ParseTemplate([]byte(ruledTableDoc(`, "minHeight": `+value, 3))); err == nil {
			t.Errorf("minHeight %s loaded; a floor of zero or less is what omitting the key already means", value)
		}
	}
}

// AN UNKNOWN BOUNDARY NAME IS A LOAD ERROR. `between` is a CLOSED set, and
// extending it later is a MAJOR version change (D-1.4.12) — so the door has to
// be shut from the commit that shipped it.
func TestAnUnknownRuleBoundaryIsRefusedAtLoad(t *testing.T) {
	_, err := ParseTemplate([]byte(ruledTableDoc(`, "rules": {"between": ["diagonals"]}`, 3)))
	if err == nil {
		t.Fatal("an unknown `between` value loaded; the set is closed at columns, rows")
	}
	if !strings.Contains(err.Error(), "columns, rows") {
		t.Errorf("the refusal does not name the closed set it enforces: %v", err)
	}
}

// MATRIX ROW "Two-line label": the header row GROWS to the packed height, which
// is the half `headerHeight`-as-a-floor buys.
func TestATwoLineLabelGrowsTheHeaderRow(t *testing.T) {
	oneLine := tablePagesForTest(t, ruledTableDoc(``, 3), `{"items": [{"a":"x"}]}`)
	twoLine := tablePagesForTest(t, strings.Replace(ruledTableDoc(``, 3), `"label": "H0"`, `"label": "H0\nSECOND"`, 1), `{"items": [{"a":"x"}]}`)
	if got, want := twoLine[0].Rects[0].H, oneLine[0].Rects[0].H; got <= want {
		t.Errorf("the header row is %dmp tall with a two-line label and %dmp with a one-line one; it must GROW — headerHeight is a floor, not an exact height", got, want)
	}
	// AND THE ROWS BELOW MOVE WITH IT, because the header's height is the data
	// rows' own origin. A header that grew without displacing them would draw
	// the second line over the first row.
	if twoLine[0].Rects[3].Y <= oneLine[0].Rects[3].Y {
		t.Errorf("the first data row is at y=%d with a two-line header and y=%d with a one-line one; the rows must follow the header down", twoLine[0].Rects[3].Y, oneLine[0].Rects[3].Y)
	}
}

// MATRIX ROW "Over-wide label": it WRAPS rather than clipping in silence, and
// the header grows to hold what it wrapped to.
func TestAnOverWideLabelWrapsAndGrowsTheHeaderRatherThanClippingInSilence(t *testing.T) {
	doc := strings.Replace(ruledTableDoc(``, 3), `"label": "H0"`, `"label": "Opening balance carried forward"`, 1)
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	res, rerr := Render(tpl, Data(`{"items": [{"a":"x"}]}`), nil, testShippedFontSet())
	if rerr != nil {
		t.Fatalf("Render: %v", rerr)
	}
	// NO CLIP, because every word of this label fits a 60pt column on its own,
	// so the packer has a break opportunity narrow enough at every step.
	for _, d := range res.Diagnostics {
		if d.Code == DiagCodeTextClippedWidth {
			t.Errorf("an over-wide label that wraps cleanly still reported %s: %s", d.Code, d.Message)
		}
	}
	pages := tablePagesForTest(t, doc, `{"items": [{"a":"x"}]}`)
	plain := tablePagesForTest(t, ruledTableDoc(``, 3), `{"items": [{"a":"x"}]}`)
	if pages[0].Rects[0].H <= plain[0].Rects[0].H {
		t.Errorf("the header row did not grow for a label that wrapped to several lines (%dmp against %dmp)", pages[0].Rects[0].H, plain[0].Rects[0].H)
	}
}

// AND WHAT IS LEFT AFTER WRAPPING IS REPORTED. A single run with no break
// opportunity narrow enough is still clipped — there is nowhere else for it to
// go — but it is no longer clipped in SILENCE, which is the header clip path's
// one difference from the body's before SPEC-table-rules.
func TestAResidualHeaderOverflowIsReportedRatherThanClippedInSilence(t *testing.T) {
	doc := strings.Replace(ruledTableDoc(``, 3), `"label": "H0"`, `"label": "Unbreakablesupercalifragilistic"`, 1)
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	res, rerr := Render(tpl, Data(`{"items": [{"a":"x"}]}`), nil, testShippedFontSet())
	if rerr != nil {
		t.Fatalf("Render: %v", rerr)
	}
	warned := 0
	for _, d := range res.Diagnostics {
		if d.Code == DiagCodeTextClippedWidth {
			warned++
			if d.ElementID != "e2" {
				t.Errorf("the clip warning names %q, want the COLUMN e2 — columns[].id exists precisely so a diagnostic can name a column", d.ElementID)
			}
		}
	}
	if warned == 0 {
		t.Fatalf("a header label with no break opportunity narrow enough was clipped and NOTHING was reported. Diagnostics: %+v", res.Diagnostics)
	}
}
