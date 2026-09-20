package wasm

import (
	"os"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/fonts"
)

// TestEmbedFontsSettingUndoesLikeEveryOtherDocumentChange is the undo row of
// spec-font-sources-and-embedding CAP-2's I/O matrix, proved at the layer that
// actually owns undo. Engine.undo holds canonical BYTE snapshots, so a setting
// that lives in those bytes undoes BY CONSTRUCTION — which is a claim worth
// measuring rather than asserting, because a setting held anywhere else (engine
// state, a projection field with no document behind it) would not.
//
// BOTH HALVES ARE CHECKED: the bytes and the PROJECTION. The panel's checkbox
// is seeded from the projection alone, so a document that undid correctly while
// the projection kept the undone value would leave the author looking at a
// setting the file does not carry.
func TestEmbedFontsSettingUndoesLikeEveryOtherDocumentChange(t *testing.T) {
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	loaded, err := engine.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Canvas == nil || !loaded.Canvas.EmbedFonts {
		t.Fatal("the shipped worked example declares no embedFonts key, so it must project as embedding")
	}
	before, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(before), "embedFonts") {
		t.Fatalf("precondition: the fixture already carries an embedFonts key:\n%s", before)
	}

	off, err := engine.Apply([]byte(`{"kind":"setDocumentEmbedFonts","version":1,"embedFonts":false}`))
	if err != nil {
		t.Fatalf("turning embedding off was refused: %v", err)
	}
	if off.Canvas == nil || off.Canvas.EmbedFonts {
		t.Fatal("the projection still says the document embeds after the setting was turned off")
	}
	if !off.CanUndo || off.CanRedo {
		t.Fatalf("history after the setting changed = %#v, want canUndo and no canRedo", off)
	}
	afterSet, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(afterSet), `"embedFonts": false`) {
		t.Fatalf("the setting did not reach the document bytes:\n%s", afterSet)
	}

	undone, err := engine.Undo()
	if err != nil {
		t.Fatal(err)
	}
	if undone.Canvas == nil || !undone.Canvas.EmbedFonts {
		t.Fatal("undo restored the document but not the projection the panel reads")
	}
	restored, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != string(before) {
		t.Fatalf("undo did not return the document to the bytes it had:\n--- got ---\n%s\n--- want ---\n%s", restored, before)
	}

	// AND REDO PUTS IT BACK, so the setting is an ordinary history event in
	// both directions rather than a one-way write undo happens to erase.
	redone, err := engine.Redo()
	if err != nil {
		t.Fatal(err)
	}
	if redone.Canvas == nil || redone.Canvas.EmbedFonts {
		t.Fatal("redo did not restore the setting the author chose")
	}
	again, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(afterSet) {
		t.Fatalf("redo did not return the document to the bytes the command produced:\n--- got ---\n%s\n--- want ---\n%s", again, afterSet)
	}
}
