package wasm

import (
	"bytes"
	"os"
	"reflect"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/template"
)

// SPEC-multi-pages story 5: each Section Break command on a later page is one
// undo entry, changes only that page, and one undo restores the bytes. A
// command without `page` still targets page 1.
func TestLaterPageSectionBreakCommandsAreOneUndoEntryEach(t *testing.T) {
	input, err := os.ReadFile("../../../fixtures/multi-page-statement/input.folio")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock())
	if _, err := engine.Load(input); err != nil {
		t.Fatal(err)
	}
	parse := func(b []byte) *template.Document {
		t.Helper()
		d, err := template.ParseDocument(b)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	for _, c := range []struct {
		command string
		check   func(before, after *template.Document)
	}{
		{`{"kind":"setSectionBreak","version":1,"offset":200,"snap":false,"page":1}`, func(before, after *template.Document) {
			if !after.Pages[1].SectionBreak.Set || after.Pages[1].SectionBreak.Value != 200000 || !reflect.DeepEqual(before.Pages[0], after.Pages[0]) {
				t.Error("page 2's break was not set alone")
			}
		}},
		{`{"kind":"setSectionBreakAnchor","version":1,"anchor":false,"page":1}`, func(before, after *template.Document) {
			if !after.Pages[1].SectionBreakAnchor.Set || after.Pages[1].SectionBreakAnchor.Value || !reflect.DeepEqual(before.Pages[0], after.Pages[0]) {
				t.Error("page 2's Anchor was not cleared alone")
			}
		}},
		{`{"kind":"setSectionBreak","version":1,"offset":300,"snap":false}`, func(before, after *template.Document) {
			if !after.Pages[0].SectionBreak.Set || after.Pages[0].SectionBreak.Value != 300000 || !reflect.DeepEqual(before.Pages[1], after.Pages[1]) {
				t.Error("a command without page did not set page 1's break alone")
			}
		}},
		{`{"kind":"removeSectionBreak","version":1,"page":1}`, func(before, after *template.Document) {
			if after.Pages[1].SectionBreak.Set || after.Pages[1].SectionBreakAnchor.Set || !reflect.DeepEqual(before.Pages[0], after.Pages[0]) {
				t.Error("page 2's break was not removed alone")
			}
		}},
	} {
		before, _, err := engine.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Apply([]byte(c.command)); err != nil {
			t.Fatalf("%s: %v", c.command, err)
		}
		after, _, err := engine.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		c.check(parse(before), parse(after))
		if _, err := engine.Undo(); err != nil {
			t.Fatal(err)
		}
		if undone, _, _ := engine.Serialize(); !bytes.Equal(undone, before) {
			t.Fatalf("one undo did not restore the bytes before %s", c.command)
		}
		if _, err := engine.Redo(); err != nil {
			t.Fatal(err)
		}
	}
	// An unknown page is refused and changes nothing.
	before, _, _ := engine.Serialize()
	if _, err := engine.Apply([]byte(`{"kind":"removeSectionBreak","version":1,"page":7}`)); err == nil {
		t.Error("a remove on page 8 of 2 was accepted")
	}
	if after, _, _ := engine.Serialize(); !bytes.Equal(after, before) {
		t.Error("a refused command changed the document")
	}
}
