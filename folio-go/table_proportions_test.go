package folio8

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/designer"
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/template"
)

func newProportionalTable(t *testing.T) (*Template, string) {
	t.Helper()
	tpl := componentTemplate(t)
	tpl.doc.Bands.Content.Elements = nil
	tpl.doc.Bands.PageHeader.Elements = nil
	tpl.doc.Bands.PageFooter.Elements = nil
	before, _ := canvas(tpl)
	after, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	return tpl, newProjectedComponent(t, before, after).ID
}

func TestProportionalCommandsStructureTotalAndAtomicRefusals(t *testing.T) {
	tpl, id := newProportionalTable(t)
	view, err := tableColumns(tpl, id)
	if err != nil || view.Sizing != "proportion" || len(view.Columns) != 1 || view.Columns[0].Proportion != "1" || view.TotalWidth != projectedBands(t, tpl)["content"].Width {
		t.Fatalf("starter: %+v %v", view, err)
	}
	mustApplyToTable(t, tpl, fmt.Sprintf(`{"kind":"setTableWidth","version":1,"id":%q,"value":"500"}`, id))
	for n := 1; n < 3; n++ {
		mustApplyToTable(t, tpl, fmt.Sprintf(`{"kind":"addTableColumn","version":1,"id":%q,"index":%d}`, id, n))
	}
	view, _ = tableColumns(tpl, id)
	for i, w := range []int64{166667, 166667, 166666} {
		if view.Columns[i].Width != w || view.Columns[i].Proportion != "1" {
			t.Fatalf("rounding: %+v", view)
		}
	}
	middle := view.Columns[1].ID
	mustApplyToTable(t, tpl, fmt.Sprintf(`{"kind":"updateTableColumn","version":1,"id":%q,"columnId":%q,"field":"proportion","value":"2"}`, id, middle))
	assertWidths := func(want []int64) {
		t.Helper()
		view, err = tableColumns(tpl, id)
		if err != nil {
			t.Fatal(err)
		}
		canvas, err := canvas(tpl)
		if err != nil {
			t.Fatal(err)
		}
		component := componentByID(t, canvas, id)
		for i, w := range want {
			if view.Columns[i].Width != w || component.Columns[i].Width != w {
				t.Fatalf("editor/canvas widths: %+v %+v", view.Columns, component.Columns)
			}
		}
		if component.Width != view.TotalWidth {
			t.Fatal("canvas lost authored total")
		}
	}
	assertWidths([]int64{125000, 250000, 125000})
	mustApplyToTable(t, tpl, fmt.Sprintf(`{"kind":"setTableWidth","version":1,"id":%q,"value":"400"}`, id))
	assertWidths([]int64{100000, 200000, 100000})
	for _, tc := range []struct{ field, value string }{
		{"proportion", `""`}, {"proportion", `"0"`}, {"proportion", `"-1"`}, {"proportion", `"bad"`}, {"proportion", `"1.0001"`}, {"proportion", `"9223372036854775.807"`},
		{"total", `""`}, {"total", `"0"`}, {"total", `"bad"`}, {"total", `"1.0001"`}, {"total", `"524"`}, {"total", `"0.001"`},
	} {
		t.Run(tc.field+tc.value, func(t *testing.T) {
			before := canonicalBytes(t, tpl)
			command := fmt.Sprintf(`{"kind":"updateTableColumn","version":1,"id":%q,"columnId":%q,"field":"proportion","value":%s}`, id, middle, tc.value)
			if tc.field == "total" {
				command = fmt.Sprintf(`{"kind":"setTableWidth","version":1,"id":%q,"value":%s}`, id, tc.value)
			}
			if _, err := applyComponentCommand(tpl, []byte(command)); err == nil || !strings.Contains(err.Error(), "folio8:") && !strings.Contains(err.Error(), "template:") {
				t.Fatalf("missing located refusal: %v", err)
			}
			_, refusal := applyComponentCommand(tpl, []byte(command))
			var componentErr *designer.ComponentCommandError
			var renderErr *RenderError
			located := errors.As(refusal, &componentErr) && componentErr.ElementID != "" || errors.As(refusal, &renderErr) && renderErr.Diagnostic.ElementID != ""
			if !located {
				t.Fatalf("refusal lost structured element identity: %v", refusal)
			}
			if !bytes.Equal(before, canonicalBytes(t, tpl)) {
				t.Fatal("refusal changed document/IDs")
			}
		})
	}
	// Removing a column keeps the other weights and total, including emptiness.
	mustApplyToTable(t, tpl, fmt.Sprintf(`{"kind":"removeTableColumn","version":1,"id":%q,"columnId":%q}`, id, view.Columns[2].ID))
	view, _ = tableColumns(tpl, id)
	if view.TotalWidth != 400000 || view.Columns[0].Proportion != "1" || view.Columns[1].Proportion != "2" {
		t.Fatalf("remove: %+v", view)
	}
	for _, col := range view.Columns {
		mustApplyToTable(t, tpl, fmt.Sprintf(`{"kind":"removeTableColumn","version":1,"id":%q,"columnId":%q}`, id, col.ID))
	}
	view, _ = tableColumns(tpl, id)
	if view.TotalWidth != 400000 || len(view.Columns) != 0 {
		t.Fatalf("empty: %+v", view)
	}
	empty := canonicalBytes(t, tpl)
	tpl, err = ParseTemplate(empty)
	if err != nil {
		t.Fatal(err)
	}
	view, err = tableColumns(tpl, id)
	if err != nil || view.TotalWidth != 400000 || len(view.Columns) != 0 || !bytes.Equal(empty, canonicalBytes(t, tpl)) {
		t.Fatalf("reopen empty table lost total: %+v %v", view, err)
	}
	mustApplyToTable(t, tpl, fmt.Sprintf(`{"kind":"addTableColumn","version":1,"id":%q,"index":0}`, id))
	assertWidths([]int64{400000})
	canonical := canonicalBytes(t, tpl)
	reloaded, err := ParseTemplate(canonical)
	if err != nil {
		t.Fatal(err)
	}
	again, err := tableColumns(reloaded, id)
	if err != nil || !reflect.DeepEqual(view, again) {
		t.Fatalf("reopen: %+v %v", again, err)
	}
}

func TestProportionalStructuralRefusalsAreLocatedAndAtomic(t *testing.T) {
	for _, kind := range []string{"remove", "move", "add"} {
		t.Run(kind, func(t *testing.T) {
			tpl, id := newProportionalTable(t)
			if kind != "add" {
				for n := 1; n < 4; n++ {
					mustApplyToTable(t, tpl, fmt.Sprintf(`{"kind":"addTableColumn","version":1,"id":%q,"index":%d}`, id, n))
				}
				view, _ := tableColumns(tpl, id)
				for _, column := range view.Columns[2:] {
					mustApplyToTable(t, tpl, fmt.Sprintf(`{"kind":"updateTableColumn","version":1,"id":%q,"columnId":%q,"field":"proportion","value":"3"}`, id, column.ID))
				}
			}
			total := "0.004"
			if kind == "add" {
				total = "0.001"
			}
			mustApplyToTable(t, tpl, fmt.Sprintf(`{"kind":"setTableWidth","version":1,"id":%q,"value":%q}`, id, total))
			view, _ := tableColumns(tpl, id)
			command := fmt.Sprintf(`{"kind":"removeTableColumn","version":1,"id":%q,"columnId":%q}`, id, view.Columns[0].ID)
			wantID := view.Columns[0].ID
			switch kind {
			case "remove":
				wantID = view.Columns[1].ID
			case "move":
				command = fmt.Sprintf(`{"kind":"moveTableColumn","version":1,"id":%q,"columnId":%q,"toIndex":3}`, id, wantID)
			case "add":
				wantID = fmt.Sprintf("e%d", tpl.doc.NextID)
				command = fmt.Sprintf(`{"kind":"addTableColumn","version":1,"id":%q,"index":1}`, id)
			}
			before, nextID := canonicalBytes(t, tpl), tpl.doc.NextID
			_, err := applyComponentCommand(tpl, []byte(command))
			var failure *RenderError
			if !errors.As(err, &failure) || failure.Diagnostic.ElementID != wantID || failure.Diagnostic.DataPath != "column.proportion" || !strings.Contains(err.Error(), "zero width") {
				t.Fatalf("lost located allocation refusal: %+v / %v", failure, err)
			}
			if tpl.doc.NextID != nextID || !bytes.Equal(before, canonicalBytes(t, tpl)) {
				t.Fatal("structural refusal changed bytes or allocated IDs")
			}
		})
	}
}

func TestProportionalColumnsRenderExactlyLikeResolvedPointGeometry(t *testing.T) {
	cols := `[{"id":"e2","label":"A","bind":"{{row.a}}","proportion":1},{"id":"e3","label":"B","bind":"{{row.b}}","proportion":2},{"id":"e4","label":"C","bind":"{{row.c}}","proportion":1}]`
	source := strings.Replace(tableHeaderDoc(`{"fontFamily":"latin","fontSize":12,"border":{"width":1,"color":"#000000"}}`, cols), `"type": "table"`, `"type": "table", "width": 500`, 1)
	source = strings.Replace(source, `"nextId": 4`, `"nextId": 5`, 1)
	proportional, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	data := `{"items":[{"a":"left","b":"middle","c":"right"}]}`
	pages := tablePagesForTest(t, source, data)
	if len(pages) != 1 || len(pages[0].Rects) < 3 {
		t.Fatalf("expected drawn table geometry: %+v", pages)
	}
	for i, w := range []geom.Length{125000, 250000, 125000} {
		if pages[0].Rects[i].W != w {
			t.Fatalf("PDF page model width %d=%d want %d", i, pages[0].Rects[i].W, w)
		}
	}
	wantPDF, err := Render(proportional, Data(data), nil, testShippedFontSet())
	if err != nil {
		t.Fatal(err)
	}
	point, err := ParseTemplate(canonicalBytes(t, proportional))
	if err != nil {
		t.Fatal(err)
	}
	el := &point.doc.Bands.Content.Elements[0]
	widths, err := template.TableColumnWidths(*el)
	if err != nil {
		t.Fatal(err)
	}
	el.Width = template.Presence[geom.Length]{}
	for i, w := range widths {
		el.Table.Value.Columns[i].Width = w
		el.Table.Value.Columns[i].Proportion = template.Presence[int64]{}
	}
	gotPDF, err := Render(point, Data(data), nil, testShippedFontSet())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wantPDF.Bytes, gotPDF.Bytes) {
		t.Fatal("resolved proportional PDF differs from exact point geometry")
	}
}
