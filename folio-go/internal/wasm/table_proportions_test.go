package wasm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	folio8 "github.com/panitw/folio8/folio-go"
)

func TestProportionAuthoringHistoryAndRefusals(t *testing.T) {
	input, err := os.ReadFile("../../../folio-designer/public/templates/starter.folio")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock())
	if _, err := engine.Load(input); err != nil {
		t.Fatal(err)
	}
	apply := func(command string) Snapshot {
		t.Helper()
		snap, err := engine.Apply([]byte(command))
		if err != nil {
			t.Fatalf("%s: %v", command, err)
		}
		return snap
	}
	serialized := func() []byte {
		t.Helper()
		b, _, err := engine.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	created := apply(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`)
	id := created.Canvas.Components[0].ID
	view, err := engine.TableColumns(id)
	if err != nil {
		t.Fatal(err)
	}
	if view.Table.Sizing != "proportion" || view.Table.Columns[0].Proportion != "1" {
		t.Fatalf("starter=%+v", view)
	}
	states := [][]byte{serialized()}
	commands := []string{
		fmt.Sprintf(`{"kind":"setTableWidth","version":1,"id":%q,"value":"500"}`, id),
		fmt.Sprintf(`{"kind":"addTableColumn","version":1,"id":%q,"index":1}`, id),
		fmt.Sprintf(`{"kind":"updateTableColumn","version":1,"id":%q,"columnId":%q,"field":"proportion","value":"2"}`, id, view.Table.Columns[0].ID),
		fmt.Sprintf(`{"kind":"setTableWidth","version":1,"id":%q,"value":"400"}`, id),
		fmt.Sprintf(`{"kind":"removeTableColumn","version":1,"id":%q,"columnId":%q}`, id, view.Table.Columns[0].ID),
	}
	for _, command := range commands {
		apply(command)
		states = append(states, serialized())
	}
	for i := len(states) - 2; i >= 0; i-- {
		if _, err := engine.Undo(); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(serialized(), states[i]) {
			t.Fatalf("undo %d did not restore total, weights and columns", i)
		}
	}
	before := engine.Snapshot()
	beforeBytes := serialized()
	for _, command := range []string{
		fmt.Sprintf(`{"kind":"setTableWidth","version":1,"id":%q,"value":"999"}`, id),
		fmt.Sprintf(`{"kind":"updateTableColumn","version":1,"id":%q,"columnId":%q,"field":"proportion","value":"0"}`, id, view.Table.Columns[0].ID),
	} {
		if _, err := engine.Apply([]byte(command)); err == nil {
			t.Fatal("invalid command accepted")
		}
		if !reflect.DeepEqual(engine.Snapshot(), before) || !bytes.Equal(beforeBytes, serialized()) {
			t.Fatal("refusal changed document, revision or redo history")
		}
	}
	for i := 1; i < len(states); i++ {
		if _, err := engine.Redo(); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(serialized(), states[i]) {
			t.Fatalf("redo %d lost sizing state", i)
		}
	}
	reloaded := NewEngine(testClock())
	if _, err := reloaded.Load(serialized()); err != nil {
		t.Fatal(err)
	}
	want, _ := engine.TableColumns(id)
	got, err := reloaded.TableColumns(id)
	if err != nil || !reflect.DeepEqual(want.Table, got.Table) {
		t.Fatalf("reopen changed proportions: %+v %+v %v", want, got, err)
	}
}

func TestProportionStructuralRefusalsPreserveLocatedHistory(t *testing.T) {
	for _, kind := range []string{"remove", "move", "add"} {
		t.Run(kind, func(t *testing.T) {
			input, err := os.ReadFile("../../../folio-designer/public/templates/starter.folio")
			if err != nil {
				t.Fatal(err)
			}
			engine := NewEngine(testClock())
			if _, err := engine.Load(input); err != nil {
				t.Fatal(err)
			}
			apply := func(command string) Snapshot {
				t.Helper()
				snap, err := engine.Apply([]byte(command))
				if err != nil {
					t.Fatalf("%s: %v", command, err)
				}
				return snap
			}
			saved := func() []byte {
				t.Helper()
				data, _, err := engine.Serialize()
				if err != nil {
					t.Fatal(err)
				}
				return data
			}
			created := apply(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`)
			id := created.Canvas.Components[0].ID
			if kind != "add" {
				for n := 1; n < 4; n++ {
					apply(fmt.Sprintf(`{"kind":"addTableColumn","version":1,"id":%q,"index":%d}`, id, n))
				}
				view, _ := engine.TableColumns(id)
				for _, col := range view.Table.Columns[2:] {
					apply(fmt.Sprintf(`{"kind":"updateTableColumn","version":1,"id":%q,"columnId":%q,"field":"proportion","value":"3"}`, id, col.ID))
				}
			}
			beforeTotal := saved()
			total := "0.004"
			if kind == "add" {
				total = "0.001"
			}
			apply(fmt.Sprintf(`{"kind":"setTableWidth","version":1,"id":%q,"value":%q}`, id, total))
			view, _ := engine.TableColumns(id)
			before := saved()
			// Keep both directions of history populated at the refusal boundary.
			apply(fmt.Sprintf(`{"kind":"updateTableColumn","version":1,"id":%q,"columnId":%q,"field":"header","value":"After"}`, id, view.Table.Columns[0].ID))
			afterHeader := saved()
			if _, err := engine.Undo(); err != nil {
				t.Fatal(err)
			}
			snapshot := engine.Snapshot()
			if !snapshot.CanUndo || !snapshot.CanRedo {
				t.Fatal("refusal fixture needs both history directions")
			}
			wantID := view.Table.Columns[0].ID
			command := fmt.Sprintf(`{"kind":"removeTableColumn","version":1,"id":%q,"columnId":%q}`, id, wantID)
			switch kind {
			case "remove":
				wantID = view.Table.Columns[1].ID
			case "move":
				command = fmt.Sprintf(`{"kind":"moveTableColumn","version":1,"id":%q,"columnId":%q,"toIndex":3}`, id, wantID)
			case "add":
				var document struct {
					NextID int64 `json:"nextId"`
				}
				if err := json.Unmarshal(before, &document); err != nil {
					t.Fatal(err)
				}
				wantID = fmt.Sprintf("e%d", document.NextID)
				command = fmt.Sprintf(`{"kind":"addTableColumn","version":1,"id":%q,"index":1}`, id)
			}
			_, err = engine.Apply([]byte(command))
			var failure *folio8.RenderError
			if !errors.As(err, &failure) || failure.Diagnostic.ElementID != wantID || failure.Diagnostic.DataPath != "column.proportion" || !strings.Contains(err.Error(), "zero width") {
				t.Fatalf("lost located allocation refusal: %+v / %v", failure, err)
			}
			if !bytes.Equal(before, saved()) || !reflect.DeepEqual(snapshot, engine.Snapshot()) {
				t.Fatal("refusal changed bytes, IDs, revision or history")
			}
			if _, err := engine.Redo(); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(afterHeader, saved()) {
				t.Fatal("refusal disturbed redo entry")
			}
			if _, err := engine.Undo(); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, saved()) {
				t.Fatal("refusal disturbed undo entry")
			}
			if _, err := engine.Undo(); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(beforeTotal, saved()) {
				t.Fatal("refusal inserted a hidden undo entry")
			}
		})
	}
}
