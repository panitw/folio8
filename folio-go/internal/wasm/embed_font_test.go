package wasm

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"testing"

	"github.com/panitw/folio8/folio-go/fonts"
)

// STORY 8.6 AT THE HISTORY BOUNDARY.
//
// The engine-side claim of AC1 and AC2 is not about the document's contents —
// component_commands_test.go measures those — it is about HISTORY: embedding a
// face and declaring the chain naming it is ONE entry with a working undo, and
// re-picking a family already embedded is NO entry at all. Both properties
// belong to wasm.Engine.Apply (its single pushUndo, and its
// bytes.Equal(canonical, e.bytes) short-circuit), so they are asserted where
// they live rather than inferred from the command returning one projection.

// embeddedCatalogueFace is a REAL face, taken from the shipped set rather than
// hand-built: the command decodes and structurally checks the bytes before it
// will write them, so a fixture that is not an sfnt would be refused before
// this file's subject — history — was ever reached.
func embeddedCatalogueFace(t *testing.T) []byte {
	t.Helper()
	face, ok := fonts.Shipped()["Noto Sans Thai"]
	if !ok || len(face) == 0 {
		t.Fatal("the shipped set carries no Noto Sans Thai, so this test has no real face to embed")
	}
	return face
}

func embedFontCommand(chain string, face []byte) []byte {
	return []byte(`{"kind":"embedFontFamily","version":1,"name":"` + chain + `"` +
		`,"family":"Noto Sans Thai","style":"Regular","licence":"OFL-1.1"` +
		`,"licenceText":"This Font Software is licensed under the SIL Open Font License, Version 1.1."` +
		`,"copyright":"Copyright 2022 The Noto Project Authors","source":"catalogue"` +
		`,"mediaType":"font/ttf","data":"` + base64.StdEncoding.EncodeToString(face) + `","tail":["Noto Sans"]}`)
}

func loadedWorkedExample(t *testing.T) *Engine {
	t.Helper()
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock())
	if _, err := engine.Load(input); err != nil {
		t.Fatal(err)
	}
	return engine
}

// TestEngineApplyEmbedFontFamilyIsOneHistoryEventWithUndoRedo is AC1's "as one
// history entry, and one undo removes both" clause end to end.
//
// THE UNDO ASSERTION IS THE POINT, and it is asserted over BOTH halves: after
// one undo the chain is gone AND the bytes are gone. A two-command
// implementation — write the asset, then declare the chain — would pass a
// revision count of one only by accident and would leave the asset behind on
// undo, which is exactly the state a single transaction exists to make
// unreachable.
func TestEngineApplyEmbedFontFamilyIsOneHistoryEventWithUndoRedo(t *testing.T) {
	engine := loadedWorkedExample(t)
	face := embeddedCatalogueFace(t)
	key := fmt.Sprintf("%x", sha256.Sum256(face))
	before := engine.Snapshot()

	after, err := engine.Apply(embedFontCommand("Brand", face))
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != before.Revision+1 || !after.CanUndo || after.CanRedo {
		t.Fatalf("history after the pick = %#v, want revision %d, canUndo, no canRedo", after, before.Revision+1)
	}
	raw, _, err := engine.AssetBytes(key)
	if err != nil || len(raw) != len(face) {
		t.Fatalf("the picked face is not carried by the document: %d bytes, %v", len(raw), err)
	}

	undone, err := engine.Undo()
	if err != nil {
		t.Fatal(err)
	}
	if undone.Revision == after.Revision || !undone.CanRedo {
		t.Fatalf("undo snapshot = %#v", undone)
	}
	if _, _, err := engine.AssetBytes(key); err == nil {
		t.Error("ONE undo left the face behind — the asset and the chain are one mutation, so undo takes both or the document is left carrying bytes nothing names")
	}
	for _, chain := range undone.Canvas.FontChains {
		if chain.Name == "Brand" {
			t.Error("ONE undo left the chain behind")
		}
	}

	redone, err := engine.Redo()
	if err != nil {
		t.Fatal(err)
	}
	if redone.Revision != undone.Revision+1 {
		t.Fatalf("redo revision = %d, want %d", redone.Revision, undone.Revision+1)
	}
	if _, _, err := engine.AssetBytes(key); err != nil {
		t.Fatalf("redo did not put the face back: %v", err)
	}
}

// TestEngineApplyEmbedFontFamilyRePickPushesNoSecondEntry is AC2 at the
// history boundary. It rests on Apply's canonical-bytes short-circuit: the
// re-pick IS a valid command and it IS accepted, and it simply leaves the
// document where it was, so revision, undo and redo must all stand still.
//
// Asserting the byte identity as well as the revision is deliberate: a
// revision that did not move would also be produced by a command that failed
// silently, and the bytes are what say the document is the one that was asked
// for.
func TestEngineApplyEmbedFontFamilyRePickPushesNoSecondEntry(t *testing.T) {
	engine := loadedWorkedExample(t)
	face := embeddedCatalogueFace(t)

	first, err := engine.Apply(embedFontCommand("Brand", face))
	if err != nil {
		t.Fatal(err)
	}
	afterFirst, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}

	second, err := engine.Apply(embedFontCommand("Brand", face))
	if err != nil {
		t.Fatalf("a re-pick of an already-embedded family must be ACCEPTED, not refused: %v", err)
	}
	if second.Revision != first.Revision {
		t.Errorf("a re-pick advanced the revision from %d to %d — it stored nothing, so it is not a committed mutation and must not be an undo step", first.Revision, second.Revision)
	}
	afterSecond, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if string(afterSecond) != string(afterFirst) {
		t.Error("a re-pick moved the document's canonical bytes")
	}

	// ONE undo returns to the document with no face at all. If the re-pick had
	// pushed an entry, this undo would land on the identical document instead.
	undone, err := engine.Undo()
	if err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("%x", sha256.Sum256(face))
	if _, _, err := engine.AssetBytes(key); err == nil {
		t.Errorf("one undo after two picks still carries the face — the re-pick pushed a second history entry (snapshot %#v)", undone)
	}
}
