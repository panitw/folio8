package wasm

import (
	"bytes"
	"github.com/panitw/folio8/folio-go/fonts"
	"os"
	"reflect"
	"testing"
)

func TestEngineGroupDeleteAndDuplicateAreOneHistoryStep(t *testing.T) {
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		command string
		delta   int
	}{
		{"deleteComponents", `{"kind":"deleteComponents","version":1,"ids":["e1","e2","e5"]}`, -3},
		{"duplicateComponents", `{"kind":"duplicateComponents","version":1,"ids":["e1","e2","e5"],"snap":true}`, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := NewEngine(testClock(), fonts.Shipped())
			before, err := engine.Load(input)
			if err != nil {
				t.Fatal(err)
			}
			original, _, _ := engine.Serialize()
			after, err := engine.Apply([]byte(tc.command))
			if err != nil || after.Revision != before.Revision+1 || !after.CanUndo || after.Canvas == nil || len(after.Canvas.Components) != len(before.Canvas.Components)+tc.delta {
				t.Fatalf("apply = %#v, %v", after, err)
			}
			committed, _, _ := engine.Serialize()
			if _, err := engine.Undo(); err != nil {
				t.Fatal(err)
			}
			restored, _, _ := engine.Serialize()
			if !bytes.Equal(restored, original) {
				t.Fatal("one undo did not restore the whole group")
			}
			if undone := engine.Snapshot(); undone.CanUndo {
				t.Fatal("the group command pushed more than one history entry")
			}
			if _, err := engine.Redo(); err != nil {
				t.Fatal(err)
			}
			redone, _, _ := engine.Serialize()
			if !bytes.Equal(redone, committed) {
				t.Fatal("redo did not replay the whole group")
			}
			current := engine.Snapshot()
			if _, err := engine.Apply([]byte(`{"kind":"deleteComponents","version":1,"ids":["ezmissing"]}`)); err == nil {
				t.Fatal("missing id accepted")
			}
			if !reflect.DeepEqual(current, engine.Snapshot()) {
				t.Fatal("refused group command changed snapshot or history")
			}
		})
	}
}
