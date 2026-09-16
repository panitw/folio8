package folio8

import (
	"bytes"
	"errors"
	"testing"

	"github.com/panitw/folio8/folio8-go/internal/designer"
)

// The header cell resolves columns[].headerAlign first; the data cell never
// consults it. Column width 100pt, align right, headerAlign center.
func TestColumnHeaderAlignMovesTheHeaderOnly(t *testing.T) {
	cols := `[{"id": "e2", "label": "A", "width": 100, "align": "right", "headerAlign": "center", "bind": "{{row.a}}"}]`
	doc := tableHeaderDocFull(`{"fontFamily": "latin"}`, "", cols, 20)
	pages := tablePagesForTest(t, doc, `{"items": [{"a": "1"}]}`)
	if len(pages[0].Runs) != 2 {
		t.Fatalf("got %d runs, want 2 (header + data)", len(pages[0].Runs))
	}
	headerX, dataX := int64(pages[0].Runs[0].X), int64(pages[0].Runs[1].X)
	if headerX <= 0 || headerX >= 60000 {
		t.Errorf("header X = %d, want centred inside the 100pt column (between 0 and 60000)", headerX)
	}
	if dataX <= 90000 {
		t.Errorf("data X = %d, want right-aligned (> 90000) — headerAlign must not reach a data cell", dataX)
	}

	// The same column without headerAlign puts the header where the data is.
	plain := tableHeaderDocFull(`{"fontFamily": "latin"}`, "", `[{"id": "e2", "label": "A", "width": 100, "align": "right", "bind": "{{row.a}}"}]`, 20)
	plainPages := tablePagesForTest(t, plain, `{"items": [{"a": "1"}]}`)
	if got := int64(plainPages[0].Runs[0].X); got <= 90000 {
		t.Errorf("header X without headerAlign = %d, want right-aligned with its column", got)
	}
}

// The canvas projection's HeaderAlign uses the same helper as the renderer.
func TestCanvasTableColumnsHonourColumnHeaderAlign(t *testing.T) {
	cols := `[{"id": "e2", "label": "A", "width": 100, "align": "right", "headerAlign": "center", "bind": "{{row.a}}"}]`
	tpl, err := ParseTemplate([]byte(tableHeaderDocFull(`{"fontFamily": "latin"}`, `{"align": "left"}`, cols, 20)))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, element, err := findComponent(tpl, "e1")
	if err != nil {
		t.Fatal(err)
	}
	columns, err := canvasTableColumns(*element)
	if err != nil {
		t.Fatal(err)
	}
	if columns[0].HeaderAlign != "center" || columns[0].CellAlign != "right" {
		t.Fatalf("canvas column resolves headerAlign=%q cellAlign=%q, want center/right", columns[0].HeaderAlign, columns[0].CellAlign)
	}
}

// The Table Editor presses what PRINTS while headerAlign is unset, so the
// projection's headerAlignResolved must be the engine's header cascade: a column
// with no align under headerStyle.align center resolves center, not left.
func TestTableColumnsProjectHeaderAlignResolved(t *testing.T) {
	for _, tc := range []struct {
		column, want string
	}{
		// No align of its own: the table-wide header alignment prints.
		{`{"id": "e2", "label": "A", "width": 50, "bind": "{{row.a}}"}`, "center"},
		// The column's own align wins over headerStyle.align.
		{`{"id": "e2", "label": "A", "width": 50, "align": "right", "bind": "{{row.a}}"}`, "right"},
		// An explicit headerAlign wins over both.
		{`{"id": "e2", "label": "A", "width": 50, "align": "right", "headerAlign": "left", "bind": "{{row.a}}"}`, "left"},
	} {
		tpl, err := ParseTemplate([]byte(tableHeaderDocFull(`{"fontFamily": "latin", "align": "right"}`, `{"align": "center"}`, "["+tc.column+"]", 20)))
		if err != nil {
			t.Fatal(err)
		}
		view, err := tableColumns(tpl, "e1")
		if err != nil {
			t.Fatal(err)
		}
		if got := view.Columns[0].HeaderAlignResolved; got != tc.want {
			t.Errorf("%s: headerAlignResolved = %q, want %q", tc.column, got, tc.want)
		}
	}
	// With no headerStyle.align the table's own style.align is the fallback.
	plain, err := ParseTemplate([]byte(tableHeaderDocFull(`{"fontFamily": "latin", "align": "right"}`, "", `[{"id": "e2", "label": "A", "width": 50, "bind": "{{row.a}}"}]`, 20)))
	if err != nil {
		t.Fatal(err)
	}
	if view, err := tableColumns(plain, "e1"); err != nil || view.Columns[0].HeaderAlignResolved != "right" {
		t.Fatalf("resolved = %#v, err=%v, want right from style.align", view.Columns, err)
	}
}

// Table-level padding insets header and data text from the column's left
// edge; a declared headerStyle.padding takes the header row over.
func TestTablePaddingInsetsHeaderAndDataCells(t *testing.T) {
	cols := `[{"id": "e2", "label": "A", "width": 100, "bind": "{{row.a}}"}]`
	pages := tablePagesForTest(t, tableHeaderDocFull(`{"fontFamily": "latin", "padding": {"left": 3, "right": 3}}`, "", cols, 20), `{"items": [{"a": "1"}]}`)
	if got := [2]int64{int64(pages[0].Runs[0].X), int64(pages[0].Runs[1].X)}; got != [2]int64{3000, 3000} {
		t.Errorf("header/data X = %v, want both inset 3pt (3000)", got)
	}
	override := tablePagesForTest(t, tableHeaderDocFull(`{"fontFamily": "latin", "padding": {"left": 3, "right": 3}}`, `{"padding": {"left": 0}}`, cols, 20), `{"items": [{"a": "1"}]}`)
	if got := [2]int64{int64(override[0].Runs[0].X), int64(override[0].Runs[1].X)}; got != [2]int64{0, 3000} {
		t.Errorf("with headerStyle.padding header/data X = %v, want [0 3000]", got)
	}

	tpl, err := ParseTemplate([]byte(tableHeaderDocFull(`{"fontFamily": "latin", "padding": {"left": 3}}`, `{"padding": {"left": 0}}`, cols, 20)))
	if err != nil {
		t.Fatal(err)
	}
	view, err := tableColumns(tpl, "e1")
	if err != nil {
		t.Fatal(err)
	}
	if view.PaddingLeft != "3000" || view.PaddingRight != "" || !view.PaddingHeaderOverride {
		t.Errorf("projection padding = %q/%q override=%v, want 3000/\"\"/true", view.PaddingLeft, view.PaddingRight, view.PaddingHeaderOverride)
	}

	// The CANVAS projection carries the table's declared padding too, which is
	// what the canvas insets its headings and cells by.
	padded, err := ParseTemplate([]byte(tableHeaderDocFull(`{"fontFamily": "latin", "padding": {"left": 3, "right": 3}}`, "", cols, 20)))
	if err != nil {
		t.Fatal(err)
	}
	canvas, err := canvas(padded)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, component := range canvas.Components {
		if component.ID != "e1" {
			continue
		}
		found = true
		if component.PaddingLeft == nil || *component.PaddingLeft != 3000 || component.PaddingRight == nil || *component.PaddingRight != 3000 {
			t.Errorf("canvas table padding = %v/%v, want 3000/3000", component.PaddingLeft, component.PaddingRight)
		}
	}
	if !found {
		t.Fatal("canvas projection has no component e1")
	}
}

// headerAlign is set-only through updateTableColumn, validated against the
// loader's closed set, and projected as committed; padding goes through the
// existing element property command.
func TestTableEditorHeaderAlignAndPaddingCommands(t *testing.T) {
	tpl := componentTemplate(t)
	before, _ := canvas(tpl)
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	table := newProjectedComponent(t, before, projection)
	view, err := tableColumns(tpl, table.ID)
	if err != nil || len(view.Columns) == 0 {
		t.Fatalf("projection = %#v, err=%v", view, err)
	}
	column := view.Columns[0]
	if column.HeaderAlign != "" || view.PaddingLeft != "" || view.PaddingRight != "" || view.PaddingHeaderOverride {
		t.Fatalf("a new table must project no headerAlign and no padding: %#v", view)
	}

	canonical, _ := SerializeTemplate(tpl)
	for _, value := range []string{`"justify"`, `""`, `null`, `1`} {
		_, err := applyComponentCommand(tpl, []byte(`{"kind":"updateTableColumn","version":1,"id":"`+table.ID+`","columnId":"`+column.ID+`","field":"headerAlign","value":`+value+`}`))
		var located *designer.ComponentCommandError
		if !errors.As(err, &located) || located.DataPath != "column.headerAlign" {
			t.Fatalf("headerAlign %s: err = %v, want a refusal located at column.headerAlign", value, err)
		}
		if after, _ := SerializeTemplate(tpl); !bytes.Equal(canonical, after) {
			t.Fatalf("headerAlign %s refusal mutated the document", value)
		}
	}

	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"updateTableColumn","version":1,"id":"`+table.ID+`","columnId":"`+column.ID+`","field":"headerAlign","value":"center"}`)); err != nil {
		t.Fatal(err)
	}
	view, _ = tableColumns(tpl, table.ID)
	if view.Columns[0].HeaderAlign != "center" || view.Columns[0].Align != "left" {
		t.Fatalf("after set, column projects headerAlign=%q align=%q", view.Columns[0].HeaderAlign, view.Columns[0].Align)
	}
	out, _ := SerializeTemplate(tpl)
	if !bytes.Contains(out, []byte(`"headerAlign": "center"`)) || !bytes.Contains(out, []byte(`"version": "3.2"`)) {
		t.Fatalf("document must carry headerAlign at 3.2:\n%s", out)
	}

	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"updateComponentProperties","version":1,"ids":["`+table.ID+`"],"changes":{"paddingLeft":{"op":"set","value":3},"paddingRight":{"op":"set","value":3}}}`)); err != nil {
		t.Fatal(err)
	}
	view, _ = tableColumns(tpl, table.ID)
	if view.PaddingLeft != "3000" || view.PaddingRight != "3000" {
		t.Fatalf("padding projects %q/%q, want 3000/3000", view.PaddingLeft, view.PaddingRight)
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"updateComponentProperties","version":1,"ids":["`+table.ID+`"],"changes":{"paddingLeft":{"op":"clear"},"paddingRight":{"op":"clear"}}}`)); err != nil {
		t.Fatal(err)
	}
	view, _ = tableColumns(tpl, table.ID)
	_, _, _, element, err := findComponent(tpl, table.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.PaddingLeft != "" || view.PaddingRight != "" || (element.Style.Set && element.Style.Value.Padding.Set) {
		t.Fatalf("cleared padding must drop the empty block: %q/%q style=%#v", view.PaddingLeft, view.PaddingRight, element.Style)
	}
}
