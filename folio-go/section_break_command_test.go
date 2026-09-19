package folio8

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/designer"
)

// spec-section-break CAP-1 / CAP-6, the designer story: the two commands and
// the straddle refusals every geometry command raises. sectionBreakTestDoc's
// page is 200 x 150pt with 10pt margins and 10pt header and footer bands, so
// the content band is 110pt tall. e1 is a table at y 0 whose declared box is
// its 10pt header; e5 is a 12pt legend at y 80.

// sectionBreakCommandDoc is sectionBreakTestDoc with an optional break and one
// more text element, e6, at y 60 (60-72pt), above a break at 75.
func sectionBreakCommandDoc(t *testing.T, contentKeys string) *Template {
	t.Helper()
	doc := sectionBreakTestDoc(contentKeys, `,
      {"id": "e6", "type": "text", "x": 0, "y": 60, "width": 60, "height": 12, "value": "Note", "style": {"fontFamily": "latin", "fontSize": 8}}`)
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	return tpl
}

// sectionBreakRefusal applies command and demands a located refusal that
// leaves the document byte-identical.
func sectionBreakRefusal(t *testing.T, tpl *Template, command string, apply func(*Template, []byte) (designer.CanvasProjection, error)) *designer.ComponentCommandError {
	t.Helper()
	before, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	_, applyErr := apply(tpl, []byte(command))
	if applyErr == nil {
		t.Fatalf("command unexpectedly succeeded: %s", command)
	}
	after, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("a refused command mutated the document: %s", command)
	}
	var failure *designer.ComponentCommandError
	if !errors.As(applyErr, &failure) {
		t.Fatalf("refusal for %s is %T (%v), want a located *ComponentCommandError", command, applyErr, applyErr)
	}
	return failure
}

func componentApply(tpl *Template, command []byte) (designer.CanvasProjection, error) {
	return applyComponentCommand(tpl, command)
}

func pageSetupApply(tpl *Template, command []byte) (designer.CanvasProjection, error) {
	return applyPageSetupCommand(tpl, command)
}

func TestSetSectionBreakPlacesSnapsAndProjects(t *testing.T) {
	tpl := sectionBreakCommandDoc(t, "")
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"setSectionBreak","version":1,"offset":40,"snap":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if projection.SectionBreak == nil || *projection.SectionBreak != 42000 {
		t.Fatalf("a snapped placement at 40pt must land on the 6pt grid at 42pt, got %v", projection.SectionBreak)
	}
	saved, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), `"sectionBreak": 42`) || !strings.Contains(string(saved), `"version": "4.1"`) {
		t.Fatalf("saved document lacks the break or its version:\n%s", saved)
	}
	for _, component := range projection.Components {
		if component.Band == bandContent && component.BelowSectionBreak == nil {
			t.Fatalf("content component %s carries no membership", component.ID)
		}
	}
	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"sectionBreak":42000`)) || !bytes.Contains(encoded, []byte(`"belowSectionBreak":true`)) {
		t.Fatalf("the wire carries neither the offset nor the membership: %s", encoded)
	}
}

func TestSetSectionBreakTypedValueIsNotSnapped(t *testing.T) {
	tpl := sectionBreakCommandDoc(t, sectionBreakAt75)
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"setSectionBreak","version":1,"offset":52.5,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	if *projection.SectionBreak != 52500 {
		t.Fatalf("a typed 52.5pt must be written exactly, got %d", *projection.SectionBreak)
	}
}

func TestSetSectionBreakRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, command, element string
	}{
		{"through an element", `{"kind":"setSectionBreak","version":1,"offset":85,"snap":false}`, "e5"},
		{"through a table header", `{"kind":"setSectionBreak","version":1,"offset":5,"snap":false}`, "e1"},
		{"snapped through an element", `{"kind":"setSectionBreak","version":1,"offset":65,"snap":true}`, "e6"},
		{"at the content band's bottom", `{"kind":"setSectionBreak","version":1,"offset":110,"snap":false}`, ""},
		{"below the content band", `{"kind":"setSectionBreak","version":1,"offset":400,"snap":false}`, ""},
		{"at the top", `{"kind":"setSectionBreak","version":1,"offset":0,"snap":false}`, ""},
		{"negative", `{"kind":"setSectionBreak","version":1,"offset":-6,"snap":false}`, ""},
		{"missing snap", `{"kind":"setSectionBreak","version":1,"offset":40}`, ""},
		{"surplus key", `{"kind":"setSectionBreak","version":1,"offset":40,"snap":false,"band":"content"}`, ""},
		{"a string offset", `{"kind":"setSectionBreak","version":1,"offset":"40","snap":false}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			failure := sectionBreakRefusal(t, sectionBreakCommandDoc(t, sectionBreakAt75), tc.command, componentApply)
			if failure.ElementID != tc.element {
				t.Fatalf("refusal names %q, want %q (%s)", failure.ElementID, tc.element, failure.Message)
			}
			if failure.DataPath != sectionBreakDataPath {
				t.Fatalf("refusal DataPath = %q, want %q", failure.DataPath, sectionBreakDataPath)
			}
		})
	}
}

func TestRemoveSectionBreak(t *testing.T) {
	tpl := sectionBreakCommandDoc(t, sectionBreakAt75)
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"removeSectionBreak","version":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if projection.SectionBreak != nil {
		t.Fatal("a removed break is still projected")
	}
	saved, _ := SerializeTemplate(tpl)
	if strings.Contains(string(saved), "sectionBreak") {
		t.Fatalf("a removed break is still in the file:\n%s", saved)
	}
	sectionBreakRefusal(t, tpl, `{"kind":"removeSectionBreak","version":1}`, componentApply)
	sectionBreakRefusal(t, sectionBreakCommandDoc(t, sectionBreakAt75), `{"kind":"removeSectionBreak","version":1,"offset":75}`, componentApply)
}

// TestGeometryCommandsRefuseASectionBreakStraddle is the I/O matrix's element
// rows: every command that can change an element's vertical extent refuses
// one that would leave it across the line, names it, and changes nothing.
func TestGeometryCommandsRefuseASectionBreakStraddle(t *testing.T) {
	for _, tc := range []struct {
		name, command, element string
	}{
		{"move", `{"kind":"moveComponent","version":1,"id":"e5","x":0,"y":70,"snap":false}`, "e5"},
		{"resize", `{"kind":"resizeComponent","version":1,"id":"e6","width":60,"height":30,"snap":false}`, "e6"},
		{"bounds", `{"kind":"setComponentBounds","version":1,"id":"e6","x":0,"y":66,"width":60,"height":12,"snap":false}`, "e6"},
		{"property y", `{"kind":"updateComponentProperties","version":1,"ids":["e5"],"changes":{"y":{"op":"set","value":70}}}`, "e5"},
		{"property height", `{"kind":"updateComponentProperties","version":1,"ids":["e6"],"changes":{"height":{"op":"set","value":20}}}`, "e6"},
		{"create", `{"kind":"createComponent","version":1,"type":"rect","band":"content","x":0,"y":70,"width":24,"height":24,"snap":false}`, "*"},
		{"drop", `{"kind":"dropComponent","version":1,"type":"rect","x":10,"y":90,"snap":false}`, "*"},
		{"duplicate", `{"kind":"duplicateComponent","version":1,"id":"e6","snap":false}`, "*"},
		{"duplicate group", `{"kind":"duplicateComponents","version":1,"ids":["e6"],"snap":false}`, "*"},
		{"header height", `{"kind":"setTableHeaderHeight","version":1,"id":"e1","height":80}`, "e1"},
		{"group move", string(moveIntent([]string{"e5", "e6"}, "e5", "0", "6", false)), "e6"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			failure := sectionBreakRefusal(t, sectionBreakCommandDoc(t, sectionBreakAt75), tc.command, componentApply)
			// "*" is a newly allocated element: any id, but one.
			named := failure.ElementID == tc.element || tc.element == "*" && failure.ElementID != "" && failure.ElementID != "e5" && failure.ElementID != "e6"
			if !named || !strings.Contains(failure.Message, "section break") {
				t.Fatalf("refusal = %q on %q, want one naming %s and the section break", failure.Message, failure.ElementID, tc.element)
			}
		})
	}
}

// And the same commands, kept on one side of the line, are accepted: the
// check refuses crossing, not movement.
func TestGeometryCommandsOnOneSideOfTheBreakAreAccepted(t *testing.T) {
	for _, command := range []string{
		`{"kind":"moveComponent","version":1,"id":"e5","x":0,"y":90,"snap":false}`,
		`{"kind":"moveComponent","version":1,"id":"e6","x":0,"y":63,"snap":false}`,
		`{"kind":"setComponentBounds","version":1,"id":"e6","x":0,"y":0,"width":60,"height":75,"snap":false}`,
		`{"kind":"createComponent","version":1,"type":"rect","band":"content","x":0,"y":75,"width":24,"height":24,"snap":false}`,
		`{"kind":"setTableHeaderHeight","version":1,"id":"e1","height":75}`,
		string(moveIntent([]string{"e5"}, "e5", "0", "6", false)),
	} {
		if _, err := applyComponentCommand(sectionBreakCommandDoc(t, sectionBreakAt75), []byte(command)); err != nil {
			t.Fatalf("%s: %v", command, err)
		}
	}
	// And with no break at all, the formerly straddling edits pass.
	if _, err := applyComponentCommand(sectionBreakCommandDoc(t, ""), []byte(`{"kind":"moveComponent","version":1,"id":"e5","x":0,"y":70,"snap":false}`)); err != nil {
		t.Fatal(err)
	}
}

func TestBandAndPageChangesRefuseToStrandTheSectionBreak(t *testing.T) {
	failure := sectionBreakRefusal(t, sectionBreakCommandDoc(t, sectionBreakAt75), `{"kind":"setBandHeight","version":1,"band":"pageFooter","height":50,"snap":false}`, componentApply)
	if failure.DataPath != sectionBreakDataPath || !strings.Contains(failure.Message, "section break") {
		t.Fatalf("band-height refusal = %+v, want one naming the section break", failure)
	}
	if _, err := applyComponentCommand(sectionBreakCommandDoc(t, sectionBreakAt75), []byte(`{"kind":"setBandHeight","version":1,"band":"pageFooter","height":30,"snap":false}`)); err != nil {
		t.Fatalf("a footer that still leaves room above the break was refused: %v", err)
	}
	failure = sectionBreakRefusal(t, sectionBreakCommandDoc(t, sectionBreakAt75), `{"kind":"pageSetup","version":1,"preset":"custom","orientation":"portrait","width":200,"height":100,"margin":{"top":10,"right":10,"bottom":10,"left":10}}`, pageSetupApply)
	if failure.DataPath != sectionBreakDataPath || !strings.Contains(failure.Message, "section break") {
		t.Fatalf("page-setup refusal = %+v, want one naming the section break", failure)
	}
}

// spec-section-break CAP-7: the Anchor toggle is one command, written only as
// `false`, projected only as `false`, and removed with the break.
func TestSetSectionBreakAnchor(t *testing.T) {
	tpl := sectionBreakCommandDoc(t, sectionBreakAt75)
	anchored, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if anchored.SectionBreakAnchor != nil {
		t.Fatalf("an anchored break must project no sectionBreakAnchor, got %v", *anchored.SectionBreakAnchor)
	}
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"setSectionBreakAnchor","version":1,"anchor":false}`))
	if err != nil {
		t.Fatal(err)
	}
	if projection.SectionBreakAnchor == nil || *projection.SectionBreakAnchor {
		t.Fatalf("an unanchored break must project sectionBreakAnchor false, got %v", projection.SectionBreakAnchor)
	}
	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"sectionBreakAnchor":false`)) {
		t.Fatalf("the wire does not carry the Anchor: %s", encoded)
	}
	saved, _ := SerializeTemplate(tpl)
	if !strings.Contains(string(saved), `"sectionBreakAnchor": false`) || !strings.Contains(string(saved), `"version": "4.1"`) {
		t.Fatalf("saved document lacks the Anchor or its version:\n%s", saved)
	}
	if _, err := ParseTemplate(saved); err != nil {
		t.Fatalf("the saved unanchored document does not reload: %v", err)
	}

	projection, err = applyComponentCommand(tpl, []byte(`{"kind":"setSectionBreakAnchor","version":1,"anchor":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if projection.SectionBreakAnchor != nil {
		t.Fatal("a re-anchored break still projects sectionBreakAnchor")
	}
	if saved, _ := SerializeTemplate(tpl); strings.Contains(string(saved), "sectionBreakAnchor") {
		t.Fatalf("a re-anchored break still writes the key:\n%s", saved)
	}

	for _, command := range []string{
		`{"kind":"setSectionBreakAnchor","version":1}`,
		`{"kind":"setSectionBreakAnchor","version":1,"anchor":"false"}`,
		`{"kind":"setSectionBreakAnchor","version":1,"anchor":null}`,
		`{"kind":"setSectionBreakAnchor","version":1,"anchor":false,"offset":75}`,
	} {
		failure := sectionBreakRefusal(t, sectionBreakCommandDoc(t, sectionBreakAt75), command, componentApply)
		if failure.DataPath != sectionBreakAnchorDataPath {
			t.Errorf("%s: refusal DataPath = %q, want %q", command, failure.DataPath, sectionBreakAnchorDataPath)
		}
	}
	failure := sectionBreakRefusal(t, sectionBreakCommandDoc(t, ""), `{"kind":"setSectionBreakAnchor","version":1,"anchor":false}`, componentApply)
	if !strings.Contains(failure.Message, "no section break") {
		t.Errorf("anchoring a document without a break: %q", failure.Message)
	}
}

func TestRemoveSectionBreakClearsTheAnchor(t *testing.T) {
	tpl := sectionBreakCommandDoc(t, `, "sectionBreak": 75, "sectionBreakAnchor": false`)
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"removeSectionBreak","version":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if projection.SectionBreak != nil || projection.SectionBreakAnchor != nil {
		t.Fatal("a removed break still projects its offset or Anchor")
	}
	saved, _ := SerializeTemplate(tpl)
	if strings.Contains(string(saved), "sectionBreak") {
		t.Fatalf("a removed break left a key in the file:\n%s", saved)
	}
	if _, err := ParseTemplate(saved); err != nil {
		t.Fatalf("the document without its break does not reload: %v", err)
	}
}
