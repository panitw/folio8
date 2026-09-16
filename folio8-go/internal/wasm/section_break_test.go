package wasm

import (
	"bytes"
	"os"
	"testing"
)

// spec-section-break, the designer story's acceptance criterion: every
// accepted section-break command is ONE undo entry, and Undo then Redo return
// the document bytes exactly to the before and after states.
func TestSectionBreakCommandsUndoAndRedoByteForByte(t *testing.T) {
	input, err := os.ReadFile("../../../fixtures/section-break-statement/input.folio")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock())
	if _, err := engine.Load(input); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{
		// Delete the break the fixture carries.
		`{"kind":"removeSectionBreak","version":1}`,
		// Place one (snapped), drag it, nudge it, type it.
		`{"kind":"setSectionBreak","version":1,"offset":403,"snap":true}`,
		`{"kind":"setSectionBreak","version":1,"offset":370,"snap":true}`,
		`{"kind":"setSectionBreak","version":1,"offset":371,"snap":false}`,
		`{"kind":"setSectionBreak","version":1,"offset":405.5,"snap":false}`,
		// spec-section-break CAP-7: turn Anchor off, on, off again, then
		// remove the unanchored break (both keys go, and undo restores both).
		`{"kind":"setSectionBreakAnchor","version":1,"anchor":false}`,
		`{"kind":"setSectionBreakAnchor","version":1,"anchor":true}`,
		`{"kind":"setSectionBreakAnchor","version":1,"anchor":false}`,
		`{"kind":"removeSectionBreak","version":1}`,
	} {
		before, _, err := engine.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Apply([]byte(command)); err != nil {
			t.Fatalf("%s: %v", command, err)
		}
		after, _, err := engine.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(before, after) {
			t.Fatalf("%s changed nothing", command)
		}
		if _, err := engine.Undo(); err != nil {
			t.Fatal(err)
		}
		undone, _, _ := engine.Serialize()
		if !bytes.Equal(undone, before) {
			t.Fatalf("one undo did not restore the bytes before %s", command)
		}
		if _, err := engine.Redo(); err != nil {
			t.Fatal(err)
		}
		redone, _, _ := engine.Serialize()
		if !bytes.Equal(redone, after) {
			t.Fatalf("redo did not restore the bytes after %s", command)
		}
	}
	// The removed unanchored break leaves neither key behind.
	if final, _, _ := engine.Serialize(); bytes.Contains(final, []byte("sectionBreak")) {
		t.Fatal("removing an unanchored break left a section-break key in the document")
	}
	// Put a break back for the refusal below.
	if _, err := engine.Apply([]byte(`{"kind":"setSectionBreak","version":1,"offset":400,"snap":false}`)); err != nil {
		t.Fatal(err)
	}
	// A refused command (through the table's header at 38-60pt) records no
	// history and changes no byte.
	before, snapshot, _ := engine.Serialize()
	if _, err := engine.Apply([]byte(`{"kind":"setSectionBreak","version":1,"offset":50,"snap":false}`)); err == nil {
		t.Fatal("a break through the table header was accepted")
	}
	after, again, _ := engine.Serialize()
	if !bytes.Equal(before, after) || again.Revision != snapshot.Revision {
		t.Fatal("a refused section-break command changed the document or its revision")
	}
}
