package wasm

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/template"
)

// SPEC-multi-pages story 1: engine commands that address an element by id
// work on a multi-page document. Each of update, move and delete on a page-2
// element changes only that element, and one undo restores the bytes.
func TestIdAddressedCommandsWorkOnALaterPageElementAndUndo(t *testing.T) {
	input, err := os.ReadFile("../../../fixtures/multi-page-statement/input.folio")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock())
	if _, err := engine.Load(input); err != nil {
		t.Fatal(err)
	}
	if saved, _, err := engine.Serialize(); err != nil || !bytes.Equal(saved, input) {
		t.Fatalf("the multi-page file does not save back byte-for-byte (err %v)", err)
	}
	parse := func(b []byte) *template.Document {
		t.Helper()
		d, err := template.ParseDocument(b)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	const target = "ee"
	indexOf := func(d *template.Document) int {
		for i, el := range d.Pages[1].Elements {
			if string(el.ID) == target {
				return i
			}
		}
		return -1
	}
	for _, c := range []struct {
		command string
		check   func(before, after *template.Document)
	}{
		{`{"kind":"updateComponentProperties","version":1,"ids":["ee"],"changes":{"value":{"op":"set","value":"1. Changed terms."}}}`, func(before, after *template.Document) {
			if got := after.Pages[1].Elements[indexOf(after)].Value.Value; got != "1. Changed terms." {
				t.Errorf("value = %q", got)
			}
		}},
		{`{"kind":"moveComponent","version":1,"id":"ee","x":0,"y":60,"snap":false}`, func(before, after *template.Document) {
			el := after.Pages[1].Elements[indexOf(after)]
			if el.X != 0 || el.Y != 60000 {
				t.Errorf("moved to %d,%d", el.X, el.Y)
			}
		}},
		{`{"kind":"deleteComponent","version":1,"id":"ee"}`, func(before, after *template.Document) {
			if indexOf(after) != -1 || len(after.Pages[1].Elements) != len(before.Pages[1].Elements)-1 {
				t.Error("the element was not deleted from page 2")
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
		b, a := parse(before), parse(after)
		if a.PageCount() != 2 || len(a.Bands.Content.Elements) != 0 {
			t.Fatalf("%s: the document left the multi-page shape", c.command)
		}
		c.check(b, a)
		// Nothing else changed: page 1, the bands and page 2's other elements.
		if !reflect.DeepEqual(b.Pages[0], a.Pages[0]) || !reflect.DeepEqual(b.Bands, a.Bands) {
			t.Errorf("%s changed page 1 or the bands", c.command)
		}
		others := func(d *template.Document) []template.Element {
			var out []template.Element
			for _, el := range d.Pages[1].Elements {
				if string(el.ID) != target {
					out = append(out, el)
				}
			}
			return out
		}
		if !reflect.DeepEqual(others(b), others(a)) {
			t.Errorf("%s changed another page-2 element", c.command)
		}
		if _, err := engine.Undo(); err != nil {
			t.Fatal(err)
		}
		if undone, _, _ := engine.Serialize(); !bytes.Equal(undone, before) {
			t.Fatalf("one undo did not restore the bytes before %s", c.command)
		}
		// Re-apply so the next command runs on the changed document, except
		// the delete, which is last.
		if _, err := engine.Redo(); err != nil {
			t.Fatal(err)
		}
	}
}

// SPEC-multi-pages story 2: every page command is one undo entry, and one undo
// restores the exact bytes — across the one-page and pages shapes.
func TestPageCommandsAreOneUndoEntryEachAndRestoreTheBytes(t *testing.T) {
	input, err := os.ReadFile("../../../fixtures/multi-page-statement/input.folio")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock())
	if _, err := engine.Load(input); err != nil {
		t.Fatal(err)
	}
	parse := func() *template.Document {
		t.Helper()
		saved, _, err := engine.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		d, err := template.ParseDocument(saved)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	for _, c := range []struct {
		command string
		check   func(d *template.Document)
	}{
		// Two pages to one: page 2 goes, the one-page shape returns.
		{`{"kind":"deletePage","version":1,"page":1}`, func(d *template.Document) {
			if d.Pages != nil || len(d.Bands.Content.Elements) == 0 {
				t.Error("deleting page 2 of 2 did not return the one-page shape")
			}
		}},
		// One page to two: the content moves into pages[0].
		{`{"kind":"addPage","version":1}`, func(d *template.Document) {
			if d.PageCount() != 2 || len(d.Bands.Content.Elements) != 0 || len(d.Pages[1].Elements) != 0 || !d.Pages[1].PageBreak {
				t.Error("adding a page did not write the pages shape with an empty page 2")
			}
		}},
		{`{"kind":"addPage","version":1,"after":0}`, func(d *template.Document) {
			if d.PageCount() != 3 || len(d.Pages[1].Elements) != 0 {
				t.Error("adding after page 1 did not insert an empty page 2")
			}
		}},
		// After the last index appends, as no after does.
		{`{"kind":"addPage","version":1,"after":2}`, func(d *template.Document) {
			if d.PageCount() != 4 || len(d.Pages[3].Elements) != 0 || !d.Pages[3].PageBreak {
				t.Error("adding after the last page did not append an empty page 4")
			}
		}},
		{`{"kind":"setPageBreak","version":1,"page":1,"pageBreak":false}`, func(d *template.Document) {
			if d.Pages[1].PageBreak {
				t.Error("Page Break was not cleared")
			}
		}},
		// Page 1 goes; old page 2 becomes page 1.
		{`{"kind":"deletePage","version":1,"page":0}`, func(d *template.Document) {
			if d.PageCount() != 3 || len(d.Pages[0].Elements) != 0 {
				t.Error("deleting page 1 did not make page 2 the first page")
			}
		}},
	} {
		// Each row builds on the document the previous row left (it is redone
		// after its undo check).
		before, _, err := engine.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		if c.command == `{"kind":"addPage","version":1}` {
			// The previous row left one page.
			if parse().Pages != nil {
				t.Fatal("precondition: one page")
			}
		}
		if _, err := engine.Apply([]byte(c.command)); err != nil {
			t.Fatalf("%s: %v", c.command, err)
		}
		c.check(parse())
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
	// The last page is never deleted.
	for parse().PageCount() > 1 {
		if _, err := engine.Apply([]byte(`{"kind":"deletePage","version":1,"page":0}`)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := engine.Apply([]byte(`{"kind":"deletePage","version":1,"page":0}`)); err == nil {
		t.Error("the engine deleted the only page")
	}
}

// SPEC-multi-pages story 3: a move to another page, and a create onto one, are
// one undo entry each, and one undo restores the bytes.
func TestCrossPageMoveAndPagedCreateAreOneUndoEntryEach(t *testing.T) {
	input, err := os.ReadFile("../../../fixtures/multi-page-statement/input.folio")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock())
	loaded, err := engine.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	pageOf := func(id string) int {
		t.Helper()
		saved, _, err := engine.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		d, err := template.ParseDocument(saved)
		if err != nil {
			t.Fatal(err)
		}
		for page, content := range d.Pages {
			for _, el := range content.Elements {
				if string(el.ID) == id {
					return page
				}
			}
		}
		return -1
	}
	positionOf := func(id string) (int64, int64) {
		t.Helper()
		saved, _, err := engine.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		d, err := template.ParseDocument(saved)
		if err != nil {
			t.Fatal(err)
		}
		for _, content := range d.Pages {
			for _, el := range content.Elements {
				if string(el.ID) == id {
					return int64(el.X), int64(el.Y)
				}
			}
		}
		t.Fatalf("no element %s", id)
		return 0, 0
	}
	move := fmt.Sprintf(`{"kind":"moveComponents","version":1,"ids":["e5"],"referenceId":"e5","dx":0,"dy":200,"snap":false,"expectedRevision":%d,"constrainToWindow":true,"page":1}`, loaded.Revision)
	preview, err := engine.GroupMovePreview([]byte(move))
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Revision != loaded.Revision || preview.DX != 0 || preview.DY != 200000 {
		t.Fatalf("preview %+v, want revision %d, dx 0, dy 200000", preview, loaded.Revision)
	}
	if _, err := engine.Apply([]byte(move)); err != nil {
		t.Fatal(err)
	}
	if pageOf("e5") != 1 {
		t.Fatal("e5 did not move to page 2")
	}
	// The commit applies exactly the previewed translation: e5 was at 0,0.
	if x, y := positionOf("e5"); int64(x) != preview.DX || int64(y) != preview.DY {
		t.Fatalf("committed e5 at %d,%d, but the preview accepted dx %d dy %d", x, y, preview.DX, preview.DY)
	}
	if _, err := engine.Undo(); err != nil {
		t.Fatal(err)
	}
	if undone, _, _ := engine.Serialize(); !bytes.Equal(undone, input) {
		t.Fatal("one undo did not restore the bytes after a cross-page move")
	}
	if _, err := engine.Apply([]byte(`{"kind":"createComponent","version":1,"type":"text","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false,"page":1}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Undo(); err != nil {
		t.Fatal(err)
	}
	if undone, _, _ := engine.Serialize(); !bytes.Equal(undone, input) {
		t.Fatal("one undo did not restore the bytes after a create onto page 2")
	}
}
