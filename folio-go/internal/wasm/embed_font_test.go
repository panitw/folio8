package wasm

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
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
	engine := NewEngine(testClock(), fonts.Shipped())
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

// ---------------------------------------------------------------------------
// SPEC-INSTALL-ALL-FACE-CUTS STORY 2 AT THE HISTORY BOUNDARY.

// embedCutCommandBytes is the second half of pressing B: one variant asset key
// attached to an entry that already carries a face.
func embedCutCommandBytes(chain string, index int, cut string, face []byte) []byte {
	return []byte(`{"kind":"embedFontCut","version":1,"name":"` + chain + `"` +
		`,"index":` + strconv.Itoa(index) + `,"cut":"` + cut + `"` +
		`,"family":"Noto Sans Thai","style":"Bold","licence":"OFL-1.1"` +
		`,"licenceText":"This Font Software is licensed under the SIL Open Font License, Version 1.1."` +
		`,"copyright":"Copyright 2022 The Noto Project Authors","source":"catalogue"` +
		`,"mediaType":"font/ttf","data":"` + base64.StdEncoding.EncodeToString(face) + `"}`)
}

// TestEngineApplyCutEmbedAndPropertyIsOneUndoEntry is D-owner-1 end to end, and
// the ONLY place the claim can be measured: "pressing B is one undo entry"
// belongs to wasm.Engine.Apply's single pushUndo, not to the command door.
//
// THE DOCUMENT COMES BACK BYTE-IDENTICAL, which is the whole acceptance
// criterion — "one undo restores the document byte-for-byte". Asserting only
// that the asset is gone would be satisfied by an undo that left `bold: true`
// standing over a face the document no longer carries: the exact half-state a
// unit exists to make unreachable.
func TestEngineApplyCutEmbedAndPropertyIsOneUndoEntry(t *testing.T) {
	engine := fontChainEngine(t)
	base := embeddedCatalogueFace(t)
	baseKey := fmt.Sprintf("%x", sha256.Sum256(base))

	// The precondition: a family in the document, and a component set in it.
	// Two ordinary commands, two ordinary undo entries — this story changes
	// neither, and they are not what is being measured.
	if _, err := engine.Apply(embedFontCommand("Noto Sans Thai", base)); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Apply([]byte(`{"kind":"updateComponentProperties","version":1,"ids":["e7"],"changes":{"fontFamily":{"op":"set","value":"Noto Sans Thai"}}}`)); err != nil {
		t.Fatal(err)
	}
	before := engine.Snapshot()
	original, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}

	bold := shippedFace(t, "Noto Sans")
	boldKey := fmt.Sprintf("%x", sha256.Sum256(bold))
	unit := commandUnitJSON(
		string(embedCutCommandBytes("Noto Sans Thai", 0, "bold", bold)),
		`{"kind":"updateComponentProperties","version":1,"ids":["e7"],"changes":{"bold":{"op":"set","value":true}}}`,
	)

	applied, err := engine.Apply(unit)
	if err != nil {
		t.Fatalf("the pair was refused: %v", err)
	}
	if applied.Revision != before.Revision+1 {
		t.Fatalf("revision = %d, want exactly one past %d — pressing B is ONE committed mutation", applied.Revision, before.Revision)
	}
	if !applied.CanUndo || applied.CanRedo {
		t.Fatalf("history after the pair = %#v", applied)
	}
	if raw, _, err := engine.AssetBytes(boldKey); err != nil || len(raw) != len(bold) {
		t.Fatalf("the document does not carry the cut: %d bytes, %v", len(raw), err)
	}
	if raw, _, err := engine.AssetBytes(baseKey); err != nil || len(raw) != len(base) {
		t.Fatalf("the embedded Regular was disturbed: %d bytes, %v", len(raw), err)
	}
	committed, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(committed, []byte(`"bold": true`)) {
		t.Fatal("the property half did not land: the author pressed B and nothing was bolded")
	}

	undone, err := engine.Undo()
	if err != nil {
		t.Fatal(err)
	}
	if undone.CanUndo != true {
		t.Fatal("the two precondition commands should still be undoable behind the pair")
	}
	restored, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, original) {
		t.Fatal("ONE undo did not restore the document byte-for-byte: the embed and the property commit are one undo step or they are two")
	}
	if _, _, err := engine.AssetBytes(boldKey); err == nil {
		t.Error("ONE undo left the cut's bytes behind")
	}

	redone, err := engine.Redo()
	if err != nil {
		t.Fatal(err)
	}
	if redone.Revision != undone.Revision+1 {
		t.Fatalf("redo revision = %d, want %d", redone.Revision, undone.Revision+1)
	}
	replayed, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(replayed, committed) {
		t.Fatal("redo did not replay BOTH halves of the pair")
	}
}

// shippedFace is a second real face, distinct from the base, so the cut's bytes
// are genuinely different bytes — a bold whose digest equals the regular's is
// refused as a self-reference and would measure nothing here.
func shippedFace(t *testing.T, name string) []byte {
	t.Helper()
	face, ok := fonts.Shipped()[name]
	if !ok || len(face) == 0 {
		t.Fatalf("the shipped set carries no %s", name)
	}
	return face
}
