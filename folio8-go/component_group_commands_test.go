package folio8

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/panitw/folio8/folio8-go/internal/template"
)

func elementsByID(tpl *Template) map[string]struct {
	band    string
	element template.Element
} {
	out := map[string]struct {
		band    string
		element template.Element
	}{}
	for name, band := range map[string]template.Band{bandPageHeader: tpl.doc.Bands.PageHeader, bandContent: tpl.doc.Bands.Content, bandPageFooter: tpl.doc.Bands.PageFooter} {
		for _, element := range band.Elements {
			out[string(element.ID)] = struct {
				band    string
				element template.Element
			}{name, element}
		}
	}
	return out
}

func TestDeleteComponentsRemovesAGroupAcrossBands(t *testing.T) {
	tpl := componentTemplate(t)
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"deleteComponents","version":1,"ids":["e5","e1","e2"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(projection.Components) != 0 || len(elementsByID(tpl)) != 0 {
		t.Fatalf("group delete left components: projection=%d template=%v", len(projection.Components), elementsByID(tpl))
	}
	if _, err := ParseTemplate(canonicalBytes(t, tpl)); err != nil {
		t.Fatal(err)
	}
}

func TestMultiComponentCommandsRefuseBadIDsWithoutMutation(t *testing.T) {
	for _, command := range []string{
		`{"kind":"deleteComponents","version":1,"ids":[]}`,
		`{"kind":"deleteComponents","version":1,"ids":null}`,
		`{"kind":"deleteComponents","version":1,"ids":"e1"}`,
		`{"kind":"deleteComponents","version":1,"ids":[""]}`,
		`{"kind":"deleteComponents","version":1,"ids":["e1","e1"]}`,
		`{"kind":"deleteComponents","version":1,"ids":["e1","ezmissing"]}`,
		`{"kind":"deleteComponents","version":1}`,
		`{"kind":"deleteComponents","version":1,"ids":["e1"],"snap":false}`,
		`{"kind":"duplicateComponents","version":1,"ids":[],"snap":false}`,
		`{"kind":"duplicateComponents","version":1,"ids":["e1","e1"],"snap":false}`,
		`{"kind":"duplicateComponents","version":1,"ids":["e1","ezmissing"],"snap":false}`,
		`{"kind":"duplicateComponents","version":1,"ids":["e1"]}`,
		`{"kind":"duplicateComponents","version":1,"ids":["e1"],"snap":"no"}`,
	} {
		t.Run(command, func(t *testing.T) {
			tpl := componentTemplate(t)
			before := canonicalBytes(t, tpl)
			if _, err := applyComponentCommand(tpl, []byte(command)); err == nil {
				t.Fatal("command accepted")
			}
			if !bytes.Equal(before, canonicalBytes(t, tpl)) {
				t.Fatal("refused command mutated the template")
			}
		})
	}
}

func TestDuplicateComponentsCopiesAGroupFromOneCounter(t *testing.T) {
	tpl := componentTemplate(t)
	original := elementsByID(tpl)
	if tpl.doc.NextID != 6 {
		t.Fatalf("precondition: nextId=%d", tpl.doc.NextID)
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"duplicateComponents","version":1,"ids":["e1","e2","e5"],"snap":false}`)); err != nil {
		t.Fatal(err)
	}
	after := elementsByID(tpl)
	// e1 -> e6; table e2 -> e7 with columns e8, e9; e5 -> ea. e5 spans the
	// footer's full width, so its +6pt copy cannot fit and keeps the source
	// position.
	for source, copyID := range map[string]string{"e1": "e6", "e2": "e7", "e5": "ea"} {
		copied, ok := after[copyID]
		if !ok {
			t.Fatalf("copy %s of %s missing: %v", copyID, source, after)
		}
		src := original[source]
		offset := int64(6000)
		if source == "e5" {
			offset = 0
		}
		if copied.band != src.band || int64(copied.element.X) != int64(src.element.X)+offset || int64(copied.element.Y) != int64(src.element.Y)+offset || copied.element.Type != src.element.Type {
			t.Fatalf("copy %s of %s = band %s at (%d,%d), source band %s at (%d,%d)", copyID, source, copied.band, copied.element.X, copied.element.Y, src.band, src.element.X, src.element.Y)
		}
		if unchanged := after[source]; fmt.Sprintf("%#v", unchanged) != fmt.Sprintf("%#v", src) {
			t.Fatalf("source %s changed", source)
		}
	}
	columns := after["e7"].element.Table.Value.Columns
	if len(columns) != 2 || columns[0].ID != "e8" || columns[1].ID != "e9" {
		t.Fatalf("table copy columns = %#v", columns)
	}
	if tpl.doc.NextID != 11 {
		t.Fatalf("nextId=%d, want 11", tpl.doc.NextID)
	}
	if _, err := ParseTemplate(canonicalBytes(t, tpl)); err != nil {
		t.Fatal(err)
	}
}

func TestDuplicateComponentsOfOneMatchesDuplicateComponent(t *testing.T) {
	for _, snap := range []string{"true", "false"} {
		for _, id := range []string{"e1", "e2", "e5"} {
			single, group := componentTemplate(t), componentTemplate(t)
			if _, err := applyComponentCommand(single, []byte(fmt.Sprintf(`{"kind":"duplicateComponent","version":1,"id":%q,"snap":%s}`, id, snap))); err != nil {
				t.Fatal(err)
			}
			if _, err := applyComponentCommand(group, []byte(fmt.Sprintf(`{"kind":"duplicateComponents","version":1,"ids":[%q],"snap":%s}`, id, snap))); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(canonicalBytes(t, single), canonicalBytes(t, group)) {
				t.Fatalf("%s snap=%s: group duplicate diverged from single duplicate", id, snap)
			}
		}
	}
}

func TestDuplicateComponentsFallsBackToTheSourcePositionWhenTheOffsetDoesNotFit(t *testing.T) {
	tpl := componentTemplate(t)
	// Footer band is 30pt tall; e5 at y=8 is 10pt tall. Move it flush with the
	// bottom so a +6pt copy would leave the band.
	tpl.doc.Bands.PageFooter.Elements[0].Y = 20000
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"duplicateComponents","version":1,"ids":["e5"],"snap":false}`)); err != nil {
		t.Fatal(err)
	}
	copied := elementsByID(tpl)["e6"]
	if copied.band != bandPageFooter || copied.element.X != 0 || copied.element.Y != 20000 {
		t.Fatalf("copy = %s (%d,%d), want footer at source position", copied.band, copied.element.X, copied.element.Y)
	}
}

func TestDuplicateComponentsClearsTheKeepTogetherTag(t *testing.T) {
	tpl, err := ParseTemplate([]byte(canvasWindowCountGroupedTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	before, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"duplicateComponents","version":1,"ids":["e2"],"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	copyID := newProjectedComponent(t, before, projection).ID
	clone, ok := contentElementByID(tpl, copyID)
	if !ok || clone.KeepTogether.Set || clone.KeepTogether.Null {
		t.Fatalf("copy %s keepTogether = %#v", copyID, clone.KeepTogether)
	}
	if original, _ := contentElementByID(tpl, "e2"); original.KeepTogether.Value != "signature" {
		t.Fatalf("original lost its tag: %#v", original.KeepTogether)
	}
}

func TestDuplicateComponentsPreflightsEveryIDWithoutMutation(t *testing.T) {
	const maxID = int64(1<<63 - 1)
	tpl := componentTemplate(t)
	// e1 needs one id, table e2 needs three: four in all.
	tpl.doc.NextID = maxID - 3
	before := canonicalBytes(t, tpl)
	command := []byte(`{"kind":"duplicateComponents","version":1,"ids":["e1","e2"],"snap":false}`)
	if _, err := applyComponentCommand(tpl, command); err == nil {
		t.Fatal("accepted too few ids for the whole group")
	}
	if !bytes.Equal(before, canonicalBytes(t, tpl)) {
		t.Fatal("refusal mutated the template")
	}
	tpl.doc.NextID = maxID - 4
	if _, err := applyComponentCommand(tpl, command); err != nil {
		t.Fatal(err)
	}
	if tpl.doc.NextID != maxID {
		t.Fatalf("nextId=%d", tpl.doc.NextID)
	}
}
