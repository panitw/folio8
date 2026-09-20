package wasm

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/panitw/folio8/folio-go/fonts"
	"os"
	"reflect"
	"strings"
	"testing"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/internal/designer"
	"github.com/panitw/folio8/folio-go/internal/expr"
)

func TestBooleanFormulasRealEngineHistoryPersistenceAndRefusal(t *testing.T) {
	original, err := os.ReadFile("../../testdata/example/first-pdf.folio")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	if _, err = engine.Load(original); err != nil {
		t.Fatal(err)
	}
	for _, formula := range []string{"loanAmount > 20000", "loanAmount + fee > 20000", "vip ? true : (blocked ? false : loanAmount >= 20000)", "true", "false", "null"} {
		before, _, _ := engine.Serialize()
		raw, _ := json.Marshal(map[string]any{"kind": "updateComponentProperties", "version": 1, "ids": []string{"e1"}, "changes": map[string]any{"visibleIf": map[string]any{"op": "set", "value": formula}}})
		if _, err := engine.Apply(raw); err != nil {
			t.Fatalf("%s: %v", formula, err)
		}
		committed, _, _ := engine.Serialize()
		var doc map[string]any
		if err := json.Unmarshal(committed, &doc); err != nil {
			t.Fatal(err)
		}
		if doc["version"] != "2.0" {
			t.Fatal("formula did not raise save requirement")
		}
		if _, err := engine.Undo(); err != nil {
			t.Fatal(err)
		}
		undone, _, _ := engine.Serialize()
		if !bytes.Equal(undone, before) {
			t.Fatal("undo lost exact pre-formula bytes")
		}
		if _, err := engine.Redo(); err != nil {
			t.Fatal(err)
		}
		redone, _, _ := engine.Serialize()
		if !bytes.Equal(redone, committed) {
			t.Fatal("redo lost formula bytes")
		}
		reloaded := NewEngine(testClock(), fonts.Shipped())
		if _, err := reloaded.Load(committed); err != nil {
			t.Fatal(err)
		}
		saved, _, _ := reloaded.Serialize()
		if !bytes.Equal(saved, committed) {
			t.Fatal("reload changed formula text")
		}
	}
	before, _, _ := engine.Serialize()
	snapshot := engine.Snapshot()
	for _, formula := range []string{"loanAmount >", "true ? upper(1) : false", "flag ? 1 : true"} {
		raw, _ := json.Marshal(map[string]any{"kind": "updateComponentProperties", "version": 1, "ids": []string{"e1"}, "changes": map[string]any{"x": map[string]any{"op": "set", "value": 12}, "visibleIf": map[string]any{"op": "set", "value": formula}}})
		_, err := engine.Apply(raw)
		var renderErr *folio8.RenderError
		var located *expr.LocatedError
		if !errors.As(err, &renderErr) || renderErr.Diagnostic.Code != folio8.DiagCodeExpressionInvalid || renderErr.Diagnostic.DataPath != "visibleIf" || !errors.As(err, &located) {
			t.Fatalf("lost cause/field for %s: %v", formula, err)
		}
		current, _, _ := engine.Serialize()
		if !bytes.Equal(current, before) || !reflect.DeepEqual(engine.Snapshot(), snapshot) {
			t.Fatal("refusal changed document/history/revision")
		}
	}
}

func TestBooleanFormulaValidOverEditorLimitIsAtomic(t *testing.T) {
	original, err := os.ReadFile("../../testdata/example/first-pdf.folio")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	if _, err := engine.Load(original); err != nil {
		t.Fatal(err)
	}
	formula := strings.Repeat(" ", 509) + "true"
	parsed, err := expr.Parse(formula)
	if err != nil {
		t.Fatal(err)
	}
	if err := expr.CheckCondition(parsed); err != nil {
		t.Fatal(err)
	}
	before, _, _ := engine.Serialize()
	snapshot := engine.Snapshot()
	raw, _ := json.Marshal(map[string]any{"kind": "updateComponentProperties", "version": 1, "ids": []string{"e1"}, "changes": map[string]any{"x": map[string]any{"op": "set", "value": 12}, "visibleIf": map[string]any{"op": "set", "value": formula}}})
	_, err = engine.Apply(raw)
	var failure *designer.ComponentCommandError
	if !errors.As(err, &failure) || failure.ElementID != "e1" || failure.DataPath != "component.visibleIf" || !strings.Contains(failure.Message, "512-byte") {
		t.Fatalf("missing field/limit context: %v", err)
	}
	after, _, _ := engine.Serialize()
	if !bytes.Equal(before, after) || !reflect.DeepEqual(snapshot, engine.Snapshot()) {
		t.Fatal("refusal changed document/history/revision")
	}
}
