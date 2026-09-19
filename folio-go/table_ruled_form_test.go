package folio8

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/designer"
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
)

// SPEC-table-rules, the per-page half: the frame, the rules and the floor are
// built per page slice AFTER pagination. ruledTableDoc's page is 300x400 with
// 10pt margins and 10pt page-header/page-footer bands, so the content window is
// 360pt tall and runs from y=10pt to y=370pt in page-model coordinates.
const (
	ruledContentTop    geom.Length = 10_000
	ruledContentBottom geom.Length = 370_000
	ruledTableWidth    geom.Length = 180_000
	ruledCellWidth     geom.Length = 60_000
)

func ruledRows(n int) string {
	items := make([]string, n)
	for i := range items {
		items[i] = fmt.Sprintf(`{"a":"r%d"}`, i)
	}
	return `{"items": [` + strings.Join(items, ",") + `]}`
}

const ruledBorder = `, "style": {"fontFamily": "latin", "fontSize": 8, "border": {"width": 1}}`

// ruledHeaderFill gives the header cells a fill, which is what makes a
// header rect — the table's own or a repeated one — findable on a page.
const ruledHeaderFill = `, "headerStyle": {"background": "#EEEEEE"}`

// headerTopOn is the top of the (filled) header cells on a page.
func headerTopOn(t *testing.T, p pagemodel.Page) geom.Length {
	t.Helper()
	top, found := geom.Length(0), false
	for _, r := range p.Rects {
		if r.W == ruledCellWidth && r.HasFill && (!found || r.Y < top) {
			top, found = r.Y, true
		}
	}
	if !found {
		t.Fatal("presence precondition: no filled header cell on this page")
	}
	return top
}

// framesOn returns every stroked rect as wide as the table with all four
// edges — the frame, and nothing else in these documents draws one.
func framesOn(p pagemodel.Page) []pagemodel.Rect {
	var out []pagemodel.Rect
	for _, r := range p.Rects {
		if r.HasStroke && r.W == ruledTableWidth && r.H > 0 && r.Edges.Top && r.Edges.Right && r.Edges.Bottom && r.Edges.Left {
			out = append(out, r)
		}
	}
	return out
}

// cellExtent is the union extent of the table's cell rects at or below y.
func cellExtent(p pagemodel.Page, y geom.Length) (top, bottom geom.Length, ok bool) {
	for _, r := range p.Rects {
		if r.W != ruledCellWidth || r.Y < y {
			continue
		}
		if !ok || r.Y < top {
			top = r.Y
		}
		if !ok || r.Y+r.H > bottom {
			bottom = r.Y + r.H
		}
		ok = true
	}
	return top, bottom, ok
}

func renderDiagnostics(t *testing.T, doc, data string) []Diagnostic {
	t.Helper()
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	res, err := Render(tpl, Data(data), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return res.Diagnostics
}

func noDiagnostic(t *testing.T, diags []Diagnostic, codes ...string) {
	t.Helper()
	for _, d := range diags {
		for _, c := range codes {
			if d.Code == c {
				t.Errorf("unexpected %s: %s", d.Code, d.Message)
			}
		}
	}
}

// MATRIX ROW "Framed table across pages".
func TestAFramedTableAcrossPagesKeepsItsPageCountAndClosesAFrameOnEveryPage(t *testing.T) {
	data := ruledRows(60)
	plain := tablePagesForTest(t, ruledTableDoc(``, 3), data)
	framed := tablePagesForTest(t, ruledTableDoc(ruledBorder+ruledHeaderFill, 3), data)
	if len(plain) < 2 {
		t.Fatalf("presence precondition: the unframed table is %d page(s), want at least 2", len(plain))
	}
	if len(framed) != len(plain) {
		t.Fatalf("the framed table is %d pages and the unframed one %d; a frame must never change the page count", len(framed), len(plain))
	}
	for i, p := range framed {
		frames := framesOn(p)
		if len(frames) != 1 {
			t.Fatalf("page %d carries %d frame(s), want exactly 1 — the frame closes per page", i, len(frames))
		}
		f := frames[0]
		top, bottom, ok := cellExtent(p, f.Y)
		if !ok {
			t.Fatalf("page %d: no table cell inside the frame", i)
		}
		if f.Y != top || f.Y+f.H != bottom {
			t.Errorf("page %d: frame spans %d..%d, want that slice's own %d..%d", i, f.Y, f.Y+f.H, top, bottom)
		}
		// On EVERY page — continuation pages included — the header, repeated
		// or not, lies inside the frame: frame top = header top.
		if h := headerTopOn(t, p); f.Y != h {
			t.Errorf("page %d: frame top %d, want the header's top %d — the header lies inside the frame on every page", i, f.Y, h)
		}
	}
	noDiagnostic(t, renderDiagnostics(t, ruledTableDoc(ruledBorder, 3), data), DiagCodeTableRowClippedHeight, DiagCodeTableHeaderRepeatSuppressed)
}

// MATRIX ROW "Rules across pages".
func TestColumnRulesRunTheHeightOfEveryPagesSlice(t *testing.T) {
	data := ruledRows(60)
	doc := ruledTableDoc(ruledBorder+ruledHeaderFill+`, "rules": {"between": ["columns"]}`, 3)
	pages := tablePagesForTest(t, doc, data)
	if len(pages) < 2 {
		t.Fatalf("presence precondition: %d page(s), want at least 2", len(pages))
	}
	if plain := tablePagesForTest(t, ruledTableDoc(``, 3), data); len(plain) != len(pages) {
		t.Errorf("rules changed the page count: %d with, %d without", len(pages), len(plain))
	}
	for i, p := range pages {
		f := framesOn(p)[0]
		verticals := verticalRules(p.Rects)
		if len(verticals) != 2 {
			t.Fatalf("page %d: %d vertical rules, want 2", i, len(verticals))
		}
		header := headerTopOn(t, p)
		for _, r := range verticals {
			if r.Y != header {
				t.Errorf("page %d: rule at x=%d starts at %d, want the header's top %d — the rules run through the header, repeated or not", i, r.X, r.Y, header)
			}
			if r.Y != f.Y || r.H != f.H {
				t.Errorf("page %d: rule at x=%d spans %d..%d, want the slice's %d..%d", i, r.X, r.Y, r.Y+r.H, f.Y, f.Y+f.H)
			}
		}
	}
}

// MATRIX ROW "Floor on every page".
func TestTheFloorAppliesToTheLastPagesSliceToo(t *testing.T) {
	data := ruledRows(60)
	doc := ruledTableDoc(ruledBorder+`, "minHeight": 300`, 3)
	pages := tablePagesForTest(t, doc, data)
	if plain := tablePagesForTest(t, ruledTableDoc(``, 3), data); len(plain) != len(pages) {
		t.Fatalf("a floor that fits every page changed the page count: %d with, %d without", len(pages), len(plain))
	}
	for i, p := range pages {
		f := framesOn(p)[0]
		_, bottom, _ := cellExtent(p, f.Y)
		want := f.Y + 300_000
		if want > ruledContentBottom {
			want = ruledContentBottom
		}
		if bottom > want {
			want = bottom
		}
		if f.Y+f.H != want {
			t.Errorf("page %d: frame bottom %d, want max(rows %d, min(top+300pt, content bottom)) = %d", i, f.Y+f.H, bottom, want)
		}
	}
	last := pages[len(pages)-1]
	if f := framesOn(last)[0]; f.H < 300_000 && f.Y+f.H != ruledContentBottom {
		t.Errorf("the last page's frame is %dmp tall; the floor must reach 300pt below that slice's top", f.H)
	}
}

// MATRIX ROW "Floor capped by the page".
func TestAFloorThatCannotFitBelowTheSliceTopRunsToTheContentBottom(t *testing.T) {
	doc := strings.Replace(ruledTableDoc(ruledBorder+`, "minHeight": 350`, 3), `"x": 0, "y": 0`, `"x": 0, "y": 100`, 1)
	pages := tablePagesForTest(t, doc, ruledRows(2))
	if len(pages) != 1 {
		t.Fatalf("got %d pages; the floor must never move a table to another page", len(pages))
	}
	f := framesOn(pages[0])[0]
	if f.Y != ruledContentTop+100_000 {
		t.Fatalf("presence precondition: the frame top is %d, want %d", f.Y, ruledContentTop+100_000)
	}
	if f.Y+f.H != ruledContentBottom {
		t.Errorf("frame bottom %d, want the content bottom %d — the floor is capped, never overflows", f.Y+f.H, ruledContentBottom)
	}
}

// MATRIX ROW "Floor pushes content below".
func TestASiblingBelowAFlooredTableStartsBelowTheFlooredBottom(t *testing.T) {
	withSibling := func(keys string) string {
		doc := ruledTableDoc(keys, 3)
		anchor := "]}\n    ]},\n    \"pageFooter\""
		if !strings.Contains(doc, anchor) {
			t.Fatal("fixture shape changed: cannot place a sibling")
		}
		return strings.Replace(doc, anchor, `]},
      {"id": "e8", "type": "text", "x": 0, "y": 60, "width": 100, "height": 10, "value": "After", "style": {"fontFamily": "latin", "fontSize": 8}}
    ]},
    "pageFooter"`, 1)
	}
	data := ruledRows(2)
	plain := tablePagesForTest(t, withSibling(ruledBorder), data)
	floored := tablePagesForTest(t, withSibling(ruledBorder+`, "minHeight": 200`), data)
	if len(plain) != 1 || len(floored) != 1 {
		t.Fatalf("presence precondition: %d and %d page(s), want 1 and 1", len(plain), len(floored))
	}
	_, plainRows, _ := cellExtent(plain[0], 0)
	sibling := func(p pagemodel.Page, below geom.Length) []geom.Length {
		var ys []geom.Length
		for _, r := range p.Runs {
			if r.Y > below {
				ys = append(ys, r.Y)
			}
		}
		return ys
	}
	before := sibling(plain[0], plainRows)
	if len(before) == 0 {
		t.Fatal("presence precondition: the sibling drew no run below the table")
	}
	f := framesOn(floored[0])[0]
	if f.Y+f.H != ruledContentTop+200_000 {
		t.Fatalf("presence precondition: floored frame bottom %d, want %d", f.Y+f.H, ruledContentTop+200_000)
	}
	after := sibling(floored[0], f.Y+f.H)
	if len(after) != len(before) {
		t.Fatalf("%d sibling run(s) below the floored bottom, want %d — the sibling must start below it, not overlap it", len(after), len(before))
	}
	extension := f.Y + f.H - plainRows
	for i := range before {
		if after[i]-before[i] != extension {
			t.Errorf("sibling run %d moved by %d, want exactly the floor's extension %d", i, after[i]-before[i], extension)
		}
	}
}

// MATRIX ROW "Perimeter owned by the frame".
func TestTheFrameOwnsThePerimeterOverTheHeadersBorder(t *testing.T) {
	doc := ruledTableDoc(ruledBorder+`, "headerStyle": {"border": {"edges": ["top", "left", "right"], "width": 1}}`, 3)
	pages := tablePagesForTest(t, doc, ruledRows(2))
	f := framesOn(pages[0])[0]
	var header []pagemodel.Rect
	for _, r := range pages[0].Rects {
		if r.W == ruledCellWidth && r.Y == f.Y {
			header = append(header, r)
		}
	}
	if len(header) != 3 {
		t.Fatalf("presence precondition: %d header cells at the frame's top, want 3", len(header))
	}
	for i, r := range header {
		if r.Edges.Top {
			t.Errorf("header cell %d strokes its top edge, which is the frame's top", i)
		}
	}
	if header[0].Edges.Left {
		t.Error("the first header cell strokes the table's left edge, which is the frame's")
	}
	if header[2].Edges.Right {
		t.Error("the last header cell strokes the table's right edge, which is the frame's")
	}
	if !header[1].Edges.Left || !header[1].Edges.Right {
		t.Error("an interior header edge was suppressed; only edges ON the frame are the frame's")
	}
}

// MATRIX ROW "Two-line label": no missing-glyph warning for the line feed.
func TestATwoLineLabelDrawsNoMissingGlyph(t *testing.T) {
	doc := strings.Replace(ruledTableDoc(``, 3), `"label": "H0"`, `"label": "H0\nDATE"`, 1)
	noDiagnostic(t, renderDiagnostics(t, doc, ruledRows(1)), DiagCodeTextMissingGlyph)
}

// A ONE-LINE label that already overruns headerHeight leaves the row as
// declared — the golden invariant (Spec Change Log entry 1).
func TestAOneLineLabelTallerThanHeaderHeightLeavesTheRowUnchanged(t *testing.T) {
	pages := tablePagesForTest(t, ruledTableDoc(``, 3), ruledRows(1))
	if got := pages[0].Rects[0].H; got != 10_000 {
		t.Errorf("header row is %dmp, want the declared 10000mp", got)
	}
}

// MATRIX ROW "Mixed-script header": one line geometry for every column.
func TestAMixedScriptHeaderSharesOneLineGeometry(t *testing.T) {
	doc := ruledTableDoc(``, 3)
	doc = strings.Replace(doc, `"latin": ["Noto Sans"]`, `"latin": ["Noto Sans", "Noto Sans Thai"]`, 1)
	doc = strings.Replace(doc, `"label": "H0"`, `"label": "วันที่\nDATE"`, 1)
	doc = strings.Replace(doc, `"label": "H1"`, `"label": "Amount"`, 1)
	pages := tablePagesForTest(t, doc, ruledRows(1))
	headerBottom := pages[0].Rects[0].Y + pages[0].Rects[0].H
	first := map[int]geom.Length{}
	for _, r := range pages[0].Runs {
		if r.Y >= headerBottom {
			continue
		}
		col := int(r.X / ruledCellWidth)
		if y, ok := first[col]; !ok || r.Y < y {
			first[col] = r.Y
		}
	}
	if len(first) < 2 {
		t.Fatalf("presence precondition: header runs in %d column(s), want at least 2", len(first))
	}
	if first[0] != first[1] {
		t.Errorf("the Thai column's first baseline is %d and the English column's %d; one header shares one line geometry", first[0], first[1])
	}
	if pages[0].Rects[0].H <= 10_000 {
		t.Errorf("the two-line header did not grow (%dmp)", pages[0].Rects[0].H)
	}
}

// ---------------------------------------------------------------------------
// Commands and projection.
// ---------------------------------------------------------------------------

func minHeightCommand(op, value string) string {
	if op == "clear" {
		return `{"kind":"setTableMinHeight","version":1,"id":"` + theWorkedExampleTable + `","op":"clear"}`
	}
	return `{"kind":"setTableMinHeight","version":1,"id":"` + theWorkedExampleTable + `","op":"set","value":` + value + `}`
}

func rulesCommand(field, op, value string) string {
	if op == "clear" {
		return `{"kind":"updateTableRules","version":1,"id":"` + theWorkedExampleTable + `","field":"` + field + `","op":"clear"}`
	}
	return `{"kind":"updateTableRules","version":1,"id":"` + theWorkedExampleTable + `","field":"` + field + `","op":"set","value":` + value + `}`
}

func reloads(t *testing.T, tpl *Template) []byte {
	t.Helper()
	encoded := canonicalBytes(t, tpl)
	if _, err := ParseTemplate(encoded); err != nil {
		t.Fatalf("the document a command produced does not load: %v\n%s", err, encoded)
	}
	return encoded
}

func TestSetTableMinHeightWritesClearsAndRefuses(t *testing.T) {
	tpl := headerStyleFixture(t)
	mustApplyToTable(t, tpl, minHeightCommand("set", "120"))
	if got := projectTable(t, tpl).MinHeight; got != 120_000 {
		t.Errorf("projected minHeight = %d, want 120000", got)
	}
	encoded := reloads(t, tpl)
	if !bytes.Contains(encoded, []byte(`"minHeight": 120`)) || !bytes.Contains(encoded, []byte(`"version": "3.1"`)) {
		t.Errorf("minHeight 120 and version 3.1 did not reach the bytes:\n%s", encoded)
	}
	mustApplyToTable(t, tpl, minHeightCommand("clear", ""))
	if got := projectTable(t, tpl).MinHeight; got != 0 {
		t.Errorf("projected minHeight after clear = %d, want 0", got)
	}
	if encoded := reloads(t, tpl); bytes.Contains(encoded, []byte(`minHeight`)) {
		t.Error("a cleared minHeight left its key in the bytes")
	}
	for _, v := range []string{"0", "-5"} {
		refusalLeavesTheDocumentAlone(t, tpl, minHeightCommand("set", v), "table.minHeight")
	}
}

func TestUpdateTableRulesOrdersCollapsesAndRefuses(t *testing.T) {
	tpl := headerStyleFixture(t)
	mustApplyToTable(t, tpl, rulesCommand("between", "set", `["rows","columns"]`))
	mustApplyToTable(t, tpl, rulesCommand("width", "set", `1`))
	mustApplyToTable(t, tpl, rulesCommand("color", "set", `"#112233"`))
	view := projectTable(t, tpl)
	if view.RulesBetween != "columns,rows" || view.RulesWidth != "1000" || view.RulesWidthResolved != "1000" || view.RulesColor != "#112233" || view.RulesColorResolved != "#112233" {
		t.Errorf("projection = between %q width %q/%q color %q/%q, want columns,rows 1000/1000 #112233/#112233",
			view.RulesBetween, view.RulesWidth, view.RulesWidthResolved, view.RulesColor, view.RulesColorResolved)
	}
	encoded := reloads(t, tpl)
	if !bytes.Contains(encoded, []byte(`"columns",`)) || bytes.Index(encoded, []byte(`"columns"`)) > bytes.Index(encoded, []byte(`"rows"`)) {
		t.Errorf("between was not written in canonical order:\n%s", encoded)
	}

	mustApplyToTable(t, tpl, rulesCommand("color", "clear", ""))
	if !bytes.Contains(reloads(t, tpl), []byte(`"rules"`)) {
		t.Fatal("the rules block vanished while width and between were still set")
	}
	if view := projectTable(t, tpl); view.RulesWidthResolved != "1000" || view.RulesColorResolved != "#000000" {
		t.Errorf("resolved width/colour = %q/%q, want 1000/#000000", view.RulesWidthResolved, view.RulesColorResolved)
	}
	mustApplyToTable(t, tpl, rulesCommand("width", "clear", ""))
	if !bytes.Contains(reloads(t, tpl), []byte(`"rules"`)) {
		t.Fatal("the rules block vanished while between was still set")
	}
	mustApplyToTable(t, tpl, rulesCommand("between", "clear", ""))
	if bytes.Contains(reloads(t, tpl), []byte(`"rules"`)) {
		t.Error("clearing the last rules attribute left an empty block behind")
	}
	if view := projectTable(t, tpl); view.RulesWidthResolved != "" || view.RulesBetween != "" {
		t.Errorf("a table with no rules block projects width %q between %q, want empty", view.RulesWidthResolved, view.RulesBetween)
	}

	refusalLeavesTheDocumentAlone(t, tpl, rulesCommand("between", "set", `["diagonals"]`), "table.rules.between")
	refusalLeavesTheDocumentAlone(t, tpl, rulesCommand("between", "set", `["rows","rows"]`), "table.rules.between")
	refusalLeavesTheDocumentAlone(t, tpl, rulesCommand("width", "set", `-1`), "table.rules.width")
	refusalLeavesTheDocumentAlone(t, tpl, rulesCommand("color", "set", `"red"`), "table.rules.color")
}

// REVIEW ITEM 7: unticking the last boundary is a clear of `between`, and a
// rules block with no boundary draws nothing yet raises the version — so that
// clear removes the WHOLE block, saved width and colour included.
func TestClearingBetweenCollapsesTheWholeRulesBlock(t *testing.T) {
	tpl := headerStyleFixture(t)
	original := canonicalBytes(t, tpl)
	mustApplyToTable(t, tpl, rulesCommand("between", "set", `["columns"]`))
	mustApplyToTable(t, tpl, rulesCommand("width", "set", `1`))
	mustApplyToTable(t, tpl, rulesCommand("color", "set", `"#112233"`))
	mustApplyToTable(t, tpl, rulesCommand("between", "clear", ""))
	after := reloads(t, tpl)
	// EXACT BYTES: the document as it was before any rule was authored, except
	// that the version keeps the 3.1 the rules raised it to — a version is
	// never lowered by removing content.
	if !bytes.Contains(original, []byte(`"version": "1.0"`)) {
		t.Fatalf("presence precondition: the fixture is not version 1.0:\n%s", original)
	}
	want := bytes.Replace(original, []byte(`"version": "1.0"`), []byte(`"version": "3.1"`), 1)
	if !bytes.Equal(after, want) {
		t.Errorf("clearing between did not remove the whole rules block:\nwant:\n%s\ngot:\n%s", want, after)
	}
}

// REVIEW ITEM 11: a document holding an explicit `"rules": null` is left
// byte-identical by a clear of any rules field.
func TestClearingARulesFieldUnderAnExplicitNullBlockIsANoOp(t *testing.T) {
	tpl, err := ParseTemplate([]byte(ruledTableDoc(`, "rules": null`, 3)))
	if err != nil {
		t.Fatal(err)
	}
	before := canonicalBytes(t, tpl)
	if !bytes.Contains(before, []byte(`"rules": null`)) {
		t.Fatalf("presence precondition: the fixture does not hold rules: null:\n%s", before)
	}
	for _, field := range []string{"width", "color", "between"} {
		if _, err := applyComponentCommand(tpl, []byte(`{"kind":"updateTableRules","version":1,"id":"e1","field":"`+field+`","op":"clear"}`)); err != nil {
			t.Fatalf("clear %s: %v", field, err)
		}
		if after := canonicalBytes(t, tpl); !bytes.Equal(before, after) {
			t.Errorf("clear %s under rules: null rewrote the bytes:\n%s", field, after)
		}
	}
}

func TestAColumnLabelIsBoundedInCodePoints(t *testing.T) {
	tpl := headerStyleFixture(t)
	column := projectTable(t, tpl).Columns[0].ID
	command := func(label string) string {
		return `{"kind":"updateTableColumn","version":1,"id":"` + theWorkedExampleTable + `","columnId":"` + column + `","field":"header","value":"` + label + `"}`
	}
	thai := strings.Repeat("ก", 256) // 768 bytes: three times the old byte bound
	mustApplyToTable(t, tpl, command(thai))
	if got := projectTable(t, tpl).Columns[0].Header; got != thai {
		t.Errorf("a 256-code-point Thai label did not commit (got %d runes)", len([]rune(got)))
	}
	refusalLeavesTheDocumentAlone(t, tpl, command(thai+"ก"), "column.header")
	astral := strings.Repeat("😀", 256) // 512 UTF-16 units, 1024 bytes
	mustApplyToTable(t, tpl, command(astral))
	if got := projectTable(t, tpl).Columns[0].Header; got != astral {
		t.Errorf("a 256-code-point astral label did not commit (got %d runes)", len([]rune(got)))
	}
	refusalLeavesTheDocumentAlone(t, tpl, command(astral+"😀"), "column.header")
}

func TestTheCanvasProjectsTheEnginesPackedLabelLines(t *testing.T) {
	doc := strings.Replace(ruledTableDoc(`, "rules": {"between": ["columns"]}, "minHeight": 50`, 3), `"label": "H0"`, `"label": "H0\nDATE"`, 1)
	doc = strings.Replace(doc, `"label": "H1"`, `"label": "Opening balance carried forward"`, 1)
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	fontless, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	painted, err := canvasWithTextPaint(tpl, testShippedFontSet())
	if err != nil {
		t.Fatal(err)
	}
	for name, projection := range map[string]designer.CanvasProjection{"Canvas": fontless, "CanvasWithTextPaint": painted} {
		columns := projection.Components[0].Columns
		if got := strings.Join(columns[0].LabelLines, "|"); got != "H0|DATE" {
			t.Errorf("%s: column 0 labelLines = %q, want H0|DATE", name, got)
		}
		if columns[2].LabelLines == nil || len(columns[2].LabelLines) != 1 {
			t.Errorf("%s: column 2 labelLines = %#v, want one line", name, columns[2].LabelLines)
		}
	}
	wrapped := painted.Components[0].Columns[1].LabelLines
	if len(wrapped) < 2 || strings.Join(wrapped, " ") != "Opening balance carried forward" {
		t.Errorf("an over-wide label projects %#v; want the engine's wrapped lines, which rejoin to the label", wrapped)
	}

	view, err := tableColumns(tpl, "e1")
	if err != nil {
		t.Fatal(err)
	}
	if view.MinHeight != 50_000 || view.RulesBetween != "columns" || view.RulesWidth != "" || view.RulesWidthResolved != "500" || view.RulesColorResolved != "#000000" {
		t.Errorf("TableColumns = minHeight %d between %q width %q/%q colour %q, want 50000 columns \"\"/500 #000000",
			view.MinHeight, view.RulesBetween, view.RulesWidth, view.RulesWidthResolved, view.RulesColorResolved)
	}
}

// MATRIX ROW "Two-line label": the lines are placed "centred per align". Each
// line is aligned on its OWN width, so under `center` two lines of different
// widths share one midpoint and under `right` one right edge. A left-aligned
// control proves the check can fail: there the two midpoints must differ.
func TestEachLineOfATwoLineLabelIsAlignedOnItsOwnWidth(t *testing.T) {
	lineExtents := func(align string) (shortRun, longRun pagemodel.TextRun) {
		t.Helper()
		doc := strings.Replace(ruledTableDoc(`, "headerStyle": {"align": "`+align+`"}`, 3), `"label": "H0"`, `"label": "W\nWWWWWW"`, 1)
		pages := tablePagesForTest(t, doc, `{"items": [{"a":"x"}]}`)
		var found int
		for _, r := range pages[0].Runs {
			switch r.SourceText {
			case "W":
				shortRun, found = r, found|1
			case "WWWWWW":
				longRun, found = r, found|2
			}
		}
		if found != 3 {
			t.Fatalf("align %s: presence precondition: found runs mask %b, want both label lines as their own runs", align, found)
		}
		if longRun.Y <= shortRun.Y {
			t.Fatalf("align %s: the second line is at y=%d, not below the first at y=%d", align, longRun.Y, shortRun.Y)
		}
		return shortRun, longRun
	}
	width := func(r pagemodel.TextRun) geom.Length {
		var adv int64
		for _, g := range r.Glyphs {
			adv += g.XAdvance
		}
		return geom.Length(adv * int64(r.FontSize) / 1000)
	}
	const tolerance = 2 // rounding, in the page's length unit
	near := func(a, b geom.Length) bool { d := a - b; return d <= tolerance && d >= -tolerance }

	s, l := lineExtents("center")
	if sm, lm := s.X+width(s)/2, l.X+width(l)/2; !near(sm, lm) {
		t.Errorf("center: the lines' midpoints are %d and %d; each line must be centred on its own width", sm, lm)
	}
	s, l = lineExtents("right")
	if se, le := s.X+width(s), l.X+width(l); !near(se, le) {
		t.Errorf("right: the lines end at %d and %d; each line must end at the cell's content edge", se, le)
	}
	s, l = lineExtents("left")
	if s.X != l.X {
		t.Errorf("left: the lines start at %d and %d, want one start edge", s.X, l.X)
	}
	if sm, lm := s.X+width(s)/2, l.X+width(l)/2; near(sm, lm) {
		t.Fatalf("control: left-aligned lines of different widths share a midpoint (%d, %d), so the center check above proves nothing", sm, lm)
	}
}

// ---------------------------------------------------------------------------
// Step-4 review items.
// ---------------------------------------------------------------------------

// REVIEW ITEM 1: a command that shrinks the content window below a
// content-band table's floor is refused, located, naming the table.
func TestACommandThatStrandsAFloorIsRefused(t *testing.T) {
	load := func() *Template {
		tpl, err := ParseTemplate([]byte(ruledTableDoc(`, "minHeight": 350`, 3)))
		if err != nil {
			t.Fatal(err)
		}
		return tpl
	}
	tpl := load()
	refusalLeavesTheDocumentAlone(t, tpl, `{"kind":"setBandHeight","version":1,"band":"pageHeader","height":30,"snap":false}`, bandHeightPath("pageHeader"))
	if located, ok := applyToTable(t, tpl, `{"kind":"setBandHeight","version":1,"band":"pageFooter","height":30,"snap":false}`).(*designer.ComponentCommandError); !ok || located.ElementID != "e1" {
		t.Errorf("a footer band that strands the floor: refusal %#v, want one naming table e1", located)
	}
	if err := applyToTable(t, tpl, `{"kind":"setBandHeight","version":1,"band":"pageHeader","height":15,"snap":false}`); err != nil {
		t.Errorf("a band height that still leaves the floor room must be accepted: %v", err)
	}

	for name, command := range map[string]string{
		"margin":      `{"kind":"pageSetup","version":1,"preset":"custom","orientation":"portrait","width":300,"height":400,"margin":{"top":30,"right":10,"bottom":10,"left":10}}`,
		"size":        `{"kind":"pageSetup","version":1,"preset":"custom","orientation":"portrait","width":300,"height":380,"margin":{"top":10,"right":10,"bottom":10,"left":10}}`,
		"orientation": `{"kind":"pageSetup","version":1,"preset":"custom","orientation":"landscape","width":300,"height":400,"margin":{"top":10,"right":10,"bottom":10,"left":10}}`,
	} {
		tpl := load()
		before := canonicalBytes(t, tpl)
		_, err := applyPageSetupCommand(tpl, []byte(command))
		located, ok := err.(*designer.ComponentCommandError)
		if !ok {
			t.Errorf("%s: error is %T (%v), want a located *ComponentCommandError", name, err, err)
			continue
		}
		if located.ElementID != "e1" || located.DataPath != "table.minHeight" {
			t.Errorf("%s: refusal located at %q/%q, want e1/table.minHeight", name, located.ElementID, located.DataPath)
		}
		if after := canonicalBytes(t, tpl); !bytes.Equal(before, after) {
			t.Errorf("%s: a refused page setup changed the canonical bytes", name)
		}
	}
	if _, err := applyPageSetupCommand(load(), []byte(`{"kind":"pageSetup","version":1,"preset":"custom","orientation":"portrait","width":300,"height":400,"margin":{"top":10,"right":10,"bottom":10,"left":10}}`)); err != nil {
		t.Errorf("a page setup that keeps the window must be accepted: %v", err)
	}
}

// REVIEW ITEM 2: setTableMinHeight above the window is refused at its field.
func TestSetTableMinHeightAboveTheWindowIsRefusedAtItsField(t *testing.T) {
	tpl, err := ParseTemplate([]byte(ruledTableDoc(``, 3)))
	if err != nil {
		t.Fatal(err)
	}
	command := func(v string) string {
		return `{"kind":"setTableMinHeight","version":1,"id":"e1","op":"set","value":` + v + `}`
	}
	refusalLeavesTheDocumentAlone(t, tpl, command("361"), "table.minHeight")
	if err := applyToTable(t, tpl, command("360")); err != nil {
		t.Errorf("a floor exactly the window's height must be accepted: %v", err)
	}
}

func bandFloorDoc() string {
	return `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "as": "row", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8},
        "columns": [{"id": "e2", "label": "H", "width": 180, "bind": "{{row.a}}"}]}
    ]},
    "pageFooter": {"elements": [
      {"id": "e5", "type": "table", "x": 0, "y": 0, "bind": "none[]", "as": "row", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8, "border": {"width": 1}}, "minHeight": 300,
        "columns": [{"id": "e6", "label": "F", "width": 120, "bind": "{{row.a}}"}]}
    ], "height": 40},
    "pageHeader": {"elements": [
      {"id": "e3", "type": "table", "x": 0, "y": 0, "bind": "none[]", "as": "row", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8, "border": {"width": 1}}, "minHeight": 300,
        "columns": [{"id": "e4", "label": "P", "width": 120, "bind": "{{row.a}}"}]}
    ], "height": 40}
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

// REVIEW ITEM 3: a page-header or page-footer table's floor is capped at its
// own band's bottom, and its frame is drawn on every page.
func TestABandTablesFloorNeverPassesItsBand(t *testing.T) {
	data := strings.Replace(ruledRows(60), `{"items": [`, `{"none": [], "items": [`, 1)
	pages := tablePagesForTest(t, bandFloorDoc(), data)
	if len(pages) < 2 {
		t.Fatalf("presence precondition: %d page(s), want at least 2", len(pages))
	}
	// Page 300x400, 10pt margins, 40pt bands: the header band is 0..40pt and
	// the footer band 340..380pt in page-model coordinates.
	for i, p := range pages {
		var header, footer []pagemodel.Rect
		for _, r := range p.Rects {
			if !r.HasStroke || r.W != 120_000 || !(r.Edges.Top && r.Edges.Right && r.Edges.Bottom && r.Edges.Left) {
				continue
			}
			if r.Y < 340_000 {
				header = append(header, r)
			} else {
				footer = append(footer, r)
			}
		}
		if len(header) != 1 || len(footer) != 1 {
			t.Fatalf("page %d: %d page-header frame(s) and %d page-footer frame(s), want 1 and 1", i, len(header), len(footer))
		}
		if h := header[0]; h.Y != 0 || h.Y+h.H != 40_000 {
			t.Errorf("page %d: page-header frame spans %d..%d, want 0..40000 — its floor is capped at the band's bottom", i, h.Y, h.Y+h.H)
		}
		if f := footer[0]; f.Y != 340_000 || f.Y+f.H != 380_000 {
			t.Errorf("page %d: page-footer frame spans %d..%d, want 340000..380000", i, f.Y, f.Y+f.H)
		}
	}
}

// siblingsDoc places text siblings under ruledTableDoc's table.
func siblingsDoc(t *testing.T, keys, siblings string) string {
	t.Helper()
	doc := ruledTableDoc(keys, 3)
	anchor := "]}\n    ]},\n    \"pageFooter\""
	if !strings.Contains(doc, anchor) {
		t.Fatal("fixture shape changed: cannot place a sibling")
	}
	doc = strings.Replace(doc, `"nextId": 9`, `"nextId": 20`, 1)
	return strings.Replace(doc, anchor, "]},\n      "+siblings+"\n    ]},\n    \"pageFooter\"", 1)
}

// REVIEW ITEM 4: the floor pushes what lies BELOW the table, never what lies
// beside it.
func TestTheFloorPushesASiblingBelowButNotOneBeside(t *testing.T) {
	siblings := `{"id": "e8", "type": "text", "x": 0, "y": 60, "width": 60, "height": 10, "value": "Below", "style": {"fontFamily": "latin", "fontSize": 8}},
      {"id": "e9", "type": "text", "x": 200, "y": 60, "width": 60, "height": 10, "value": "Beside", "style": {"fontFamily": "latin", "fontSize": 8}}`
	data := ruledRows(2)
	plain := tablePagesForTest(t, siblingsDoc(t, ruledBorder, siblings), data)
	floored := tablePagesForTest(t, siblingsDoc(t, ruledBorder+`, "minHeight": 200`, siblings), data)
	_, rows, _ := cellExtent(plain[0], 0)
	f := framesOn(floored[0])[0]
	extension := f.Y + f.H - rows
	if extension <= 0 {
		t.Fatalf("presence precondition: the floor extends nothing (%d)", extension)
	}
	runsAt := func(p pagemodel.Page, below geom.Length, beside bool) []geom.Length {
		var ys []geom.Length
		for _, r := range p.Runs {
			if r.Y > below && (r.X >= 200_000) == beside {
				ys = append(ys, r.Y)
			}
		}
		return ys
	}
	belowPlain, belowFloored := runsAt(plain[0], rows, false), runsAt(floored[0], rows, false)
	besidePlain, besideFloored := runsAt(plain[0], rows, true), runsAt(floored[0], rows, true)
	if len(belowPlain) == 0 || len(besidePlain) == 0 || len(belowPlain) != len(belowFloored) || len(besidePlain) != len(besideFloored) {
		t.Fatalf("presence precondition: run counts below %d/%d, beside %d/%d", len(belowPlain), len(belowFloored), len(besidePlain), len(besideFloored))
	}
	for i := range belowPlain {
		if belowFloored[i]-belowPlain[i] != extension {
			t.Errorf("the sibling below moved %d, want the extension %d", belowFloored[i]-belowPlain[i], extension)
		}
	}
	for i := range besidePlain {
		if besideFloored[i] != besidePlain[i] {
			t.Errorf("the sibling BESIDE the table moved from %d to %d; a growing table never moves a sibling beside it", besidePlain[i], besideFloored[i])
		}
	}
}

// REVIEW ITEM 5: a repeated boundary is refused at load.
func TestADuplicateRuleBoundaryIsRefusedAtLoad(t *testing.T) {
	_, err := ParseTemplate([]byte(ruledTableDoc(`, "rules": {"between": ["columns", "columns"]}`, 3)))
	if err == nil {
		t.Fatal(`"between": ["columns","columns"] loaded; each boundary may be named at most once`)
	}
	if !strings.Contains(err.Error(), "rules.between") {
		t.Errorf("the refusal is not located at rules.between: %v", err)
	}
}

// REVIEW ITEM 8: the canvas never silently cuts a label — a long Thai line and
// many lines both survive the projection.
func TestTheCanvasProjectsLongAndManyLabelLinesWhole(t *testing.T) {
	long := strings.Repeat("ก", 300) // 900 bytes on one line
	many := strings.TrimSuffix(strings.Repeat("a\n", 40), "\n")
	doc := strings.Replace(ruledTableDoc(``, 3), `"label": "H0"`, `"label": "`+long+`"`, 1)
	doc = strings.Replace(doc, `"label": "H1"`, `"label": "`+strings.ReplaceAll(many, "\n", `\n`)+`"`, 1)
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	projection, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	columns := projection.Components[0].Columns
	if got := strings.Join(columns[0].LabelLines, ""); got != long {
		t.Errorf("a 900-byte label line projected as %d bytes; it must not be cut", len(got))
	}
	if got := len(columns[1].LabelLines); got != 40 {
		t.Errorf("a 40-line label projected %d lines; none may be dropped", got)
	}
}

// REVIEW ITEM 8: Go's label-line bounds and the browser guard's are one pair.
func TestCanvasLabelLineBoundsMatchTheDesignerGuard(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "folio-designer", "src", "engine-protocol.ts"))
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]int{
		"MAX_CANVAS_TABLE_LABEL_LINES":       maxCanvasHeaderLabelLines,
		"MAX_CANVAS_TABLE_LABEL_LINE_LENGTH": maxCanvasHeaderLabelLineBytes,
	} {
		m := regexp.MustCompile(`(?m)^export const ` + name + ` = (\d+)$`).FindSubmatch(source)
		if m == nil {
			t.Errorf("engine-protocol.ts declares no %s", name)
			continue
		}
		if got, _ := strconv.Atoi(string(m[1])); got != want {
			t.Errorf("engine-protocol.ts %s = %d, Go's bound is %d", name, got, want)
		}
	}
}

// REVIEW ITEM 15: the unplaceable-floor message speaks points.
func TestTheUnplaceableFloorMessageSpeaksPoints(t *testing.T) {
	_, err := ParseTemplate([]byte(ruledTableDoc(`, "minHeight": 500`, 3)))
	if err == nil {
		t.Fatal("presence precondition: a 500pt floor in a 360pt window must be refused")
	}
	if msg := err.Error(); !strings.Contains(msg, "500pt") || !strings.Contains(msg, "360pt") || strings.Contains(msg, " mp") {
		t.Errorf("message does not state the floor and the window in points: %s", msg)
	}
}

// REVIEW ITEM 18a (rect half): a rect sibling with a background is pushed by
// exactly the extension.
func TestTheFloorPushesARectSibling(t *testing.T) {
	sibling := `{"id": "e8", "type": "rect", "x": 0, "y": 60, "width": 50, "height": 10, "style": {"background": "#FF0000"}}`
	data := ruledRows(2)
	find := func(p pagemodel.Page) pagemodel.Rect {
		for _, r := range p.Rects {
			if r.HasFill && r.W == 50_000 {
				return r
			}
		}
		t.Fatal("presence precondition: no rect sibling on the page")
		return pagemodel.Rect{}
	}
	plain := tablePagesForTest(t, siblingsDoc(t, ruledBorder, sibling), data)
	floored := tablePagesForTest(t, siblingsDoc(t, ruledBorder+`, "minHeight": 200`, sibling), data)
	_, rows, _ := cellExtent(plain[0], 0)
	f := framesOn(floored[0])[0]
	if got, want := find(floored[0]).Y, find(plain[0]).Y+(f.Y+f.H-rows); got != want {
		t.Errorf("rect sibling Y = %d, want the plain Y plus the extension, %d", got, want)
	}
}

// REVIEW ITEM 18d: an invalid rules colour is a located error — at LOAD,
// like every other colour in the format (owner ruling, 2026-09-17).
func TestAnInvalidRulesColourIsALoadError(t *testing.T) {
	requireColourLoadError(t, ruledTableDoc(`, "rules": {"between": ["columns"], "color": "red"}`, 3), "e1", "rules.color")
}

const ruledPNGAsset = `"assets": {
    "5a05ad01e89c143b7061b0c93450566568d38a23da9b9c5c9dfe449016433078": {"data": ["iVBORw0KGgoAAAANSUhEUgAAAAMAAAACCAIAAAASFvFNAAAAGElEQVR42mL6z8DAAMZMEOo/AwMg", "AAD//zwUBf/NjsW5AAAAAElFTkSuQmCC"], "mediaType": "image/png"}
  }`

// REVIEW ITEM 18a (image half): an image sibling is pushed by exactly the
// extension.
func TestTheFloorPushesAnImageSibling(t *testing.T) {
	sibling := `{"id": "e8", "type": "image", "asset": "5a05ad01e89c143b7061b0c93450566568d38a23da9b9c5c9dfe449016433078", "x": 0, "y": 60, "width": 30, "height": 20}`
	withAsset := func(doc string) string { return strings.Replace(doc, `"assets": {}`, ruledPNGAsset, 1) }
	data := ruledRows(2)
	plain := tablePagesForTest(t, withAsset(siblingsDoc(t, ruledBorder, sibling)), data)
	floored := tablePagesForTest(t, withAsset(siblingsDoc(t, ruledBorder+`, "minHeight": 200`, sibling)), data)
	if len(plain[0].Images) != 1 || len(floored[0].Images) != 1 {
		t.Fatalf("presence precondition: %d and %d image(s), want 1 and 1", len(plain[0].Images), len(floored[0].Images))
	}
	_, rows, _ := cellExtent(plain[0], 0)
	f := framesOn(floored[0])[0]
	if got, want := floored[0].Images[0].Y, plain[0].Images[0].Y+(f.Y+f.H-rows); got != want {
		t.Errorf("image sibling Y = %d, want the plain Y plus the extension, %d", got, want)
	}
}

// REVIEW ITEM 18b: a floor that pushes a sibling onto a second page is seen by
// the page-count pass too, so a {{pages}} footer prints 2 on both pages.
func TestAFloorPushOntoANewPageIsCountedByThePagesSlot(t *testing.T) {
	sibling := `{"id": "e8", "type": "text", "x": 0, "y": 60, "width": 60, "height": 10, "value": "After", "style": {"fontFamily": "latin", "fontSize": 8}}`
	footer := `"pageFooter": {"elements": [{"id": "e9", "type": "text", "x": 0, "y": 0, "width": 100, "height": 8, "value": "{{pages}}", "style": {"fontFamily": "latin", "fontSize": 6}}], "height": 10}`
	build := func(keys string) string {
		return strings.Replace(siblingsDoc(t, ruledBorder+keys, sibling), `"pageFooter": {"elements": [], "height": 10}`, footer, 1)
	}
	render := func(doc string) [][]string {
		tpl, err := ParseTemplate([]byte(doc))
		if err != nil {
			t.Fatal(err)
		}
		res, err := Render(tpl, Data(ruledRows(2)), nil, testShippedFontSet())
		if err != nil {
			t.Fatal(err)
		}
		return pageTextsOf(t, res.Bytes)
	}
	if plain := render(build(``)); len(plain) != 1 {
		t.Fatalf("presence precondition: the unfloored document is %d page(s), want 1", len(plain))
	}
	pages := render(build(`, "minHeight": 355`))
	if len(pages) != 2 {
		t.Fatalf("the pushed document is %d page(s), want 2", len(pages))
	}
	for i, texts := range pages {
		if !slices.Contains(texts, "2") {
			t.Errorf("page %d prints %q; its {{pages}} footer must print 2", i+1, texts)
		}
	}
}
