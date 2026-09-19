// Story 10.1's own suite: style.color — the ink a text-bearing element
// prints in. The format carried no such field before this story; text
// took the PDF's own initial fill and there was nothing to set.
package folio8

import (
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/pagemodel"
)

func inkTemplateJSON(styleFields string) string {
	return `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 30, "value": "Inked", "style": {` + styleFields + `"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "text", "x": 0, "y": 100, "width": 200, "height": 20, "value": "Plain", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

func TestStyleColorInksEveryRunOfItsElementAndNoOther(t *testing.T) {
	pages := boxPages(t, inkTemplateJSON(`"color": "#c81e1e", `))
	inked, plain := 0, 0
	for _, run := range pages[0].Runs {
		switch {
		case run.HasColor:
			inked++
			if run.Color != (pagemodel.Color{R: 0xc8, G: 0x1e, B: 0x1e}) {
				t.Errorf("inked run carries %+v, want #c81e1e", run.Color)
			}
		default:
			plain++
		}
	}
	if inked == 0 || plain == 0 {
		t.Fatalf("runs: %d inked, %d plain — want e1's runs inked and e2's left alone", inked, plain)
	}
}

// An element that declares no colour must carry NO colour, not black:
// the difference is a colour operator in the content stream, and it is
// what keeps every document written before this field byte-identical.
func TestAnUndeclaredColorIsAbsentRatherThanBlack(t *testing.T) {
	for _, run := range boxPages(t, inkTemplateJSON(``))[0].Runs {
		if run.HasColor {
			t.Fatalf("a run carries a colour (%+v) on a document that declares none", run.Color)
		}
	}
}

func TestANullColorIsNoColor(t *testing.T) {
	for _, run := range boxPages(t, inkTemplateJSON(`"color": null, `))[0].Runs {
		if run.HasColor {
			t.Errorf("an explicitly null color inked a run %+v — present-null means no declaration", run.Color)
		}
	}
}

func TestAMalformedColorIsALocatedLoadError(t *testing.T) {
	requireColourLoadError(t, inkTemplateJSON(`"color": "red", `), "e1", "style.color")
}

// style.color is a string-valued style field, so Story 3.5's fence
// covers it by construction: no colour-by-data, in any style field.
func TestAPlaceholderInStyleColorIsALoadError(t *testing.T) {
	_, err := ParseTemplate([]byte(inkTemplateJSON(`"color": "{{customer.brand}}", `)))
	if err == nil {
		t.Fatal("a {{ }} placeholder in style.color loaded successfully")
	}
	if !strings.Contains(err.Error(), "style.color") {
		t.Errorf("error %q does not name style.color", err)
	}
}

// A table's cells take the table's own cascade, exactly as their
// background and padding already do.
func TestATableCascadesItsInkToHeaderAndCells(t *testing.T) {
	doc := `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "rows[]", "as": "row", "headerHeight": 20,
          "style": {"color": "#1b2a4a", "fontFamily": "body", "fontSize": 10},
          "headerStyle": {"color": "#c81e1e"},
          "columns": [{"id": "e2", "label": "A", "width": 100, "align": "left", "bind": "{{row.a}}"}]}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	pages, _, _, _, err := buildPageModel(tpl, mustDecodeData(t, `{"rows":[{"a":"one"}],"customer":{"flag":false}}`), mustDecodeParams(t), testFontSet())
	if err != nil {
		t.Fatalf("buildPageModel: %v", err)
	}
	header, body := 0, 0
	for _, run := range pages[0].Runs {
		if !run.HasColor {
			t.Fatalf("a table run carries no ink: %q", run.SourceText)
		}
		switch run.Color {
		case pagemodel.Color{R: 0xc8, G: 0x1e, B: 0x1e}:
			header++
		case pagemodel.Color{R: 0x1b, G: 0x2a, B: 0x4a}:
			body++
		default:
			t.Errorf("run %q carries %+v, which is neither the header's nor the body's ink", run.SourceText, run.Color)
		}
	}
	if header == 0 || body == 0 {
		t.Errorf("header runs = %d, body runs = %d — headerStyle.color must win for the header and style.color for the cells", header, body)
	}
}

// TestAMalformedColorIsRefusedWhateverTheDataSays is E10-1's proof,
// carried to where the check now lives. The colour check once sat below
// the AC9 empty-text short-circuit and the hidden-element skip, so
// `style.color: "red"` passed or failed on the data a render was handed.
// Since colour is refused at LOAD (owner ruling, 2026-09-17) no data is
// consulted at all: each case below, which a render would skip, is
// refused before a render can start, and its well-formed twin loads.
func TestAMalformedColorIsRefusedWhateverTheDataSays(t *testing.T) {
	cases := map[string]string{
		"value binds to empty": `"value": "{{customer.blank}}"`,
		"value binds to null":  `"value": "{{customer.absent}}"`,
		"element hidden":       `"value": "Inked", "visibleIf": "customer.flag"`,
	}
	docFor := func(element, style string) string {
		return `{
  "assets": {},
  "bands": {
    "content": {"elements": [{"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 30, ` + element + `, "style": {` + style + `}}]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	}
	const data = `{"customer":{"flag":false,"blank":"","absent":null}}`
	for name, element := range cases {
		t.Run(name, func(t *testing.T) {
			requireColourLoadError(t, docFor(element, `"color": "red", "fontFamily": "body", "fontSize": 12`), "e1", "style.color")

			// The well-formed twin still loads and renders, so "refused"
			// above is evidence about the colour rather than the fixture.
			tpl, err := ParseTemplate([]byte(docFor(element, `"color": "#c81e1e", "fontFamily": "body", "fontSize": 12`)))
			if err != nil {
				t.Fatalf("the valid-colour twin failed to load: %v", err)
			}
			if _, _, _, _, err := buildPageModel(tpl, mustDecodeData(t, data), mustDecodeParams(t), testFontSet()); err != nil {
				t.Fatalf("the valid-colour twin failed to render: %v", err)
			}
		})
	}
}

// cascadeTableDoc builds a one-column bound table whose own style declares
// `color: #c81e1e` and whose headerStyle is headerStyleJSON verbatim.
func cascadeTableDoc(headerStyleJSON string) string {
	return `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "rows[]", "as": "row", "headerHeight": 20,
          "style": {"color": "#c81e1e", "background": "#eeeeee", "border": {}, "fontFamily": "body", "fontSize": 10},
          ` + headerStyleJSON + `
          "columns": [{"id": "e2", "label": "A", "width": 100, "align": "left", "bind": "{{row.a}}"}]}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

// TestANullHeaderColorFallsThroughToTheTableColour is E10-2's proof.
//
// resolveHeaderStyle's Color arm was the ONLY one of nine reading `.Set`
// without `.Null`, so `headerStyle: {"color": null}` WON the cascade with a
// null and stopped the fall-through: measured at 2 inked runs where an
// absent or empty headerStyle gave 3, with the header printing black. The
// background and border arms on the SAME table are the controls — both
// already fall through on an explicit null — and folio-format.md already
// states the correct rule, so the code was the outlier and neither document
// changes.
func TestANullHeaderColorFallsThroughToTheTableColour(t *testing.T) {
	inkOps := func(t *testing.T, headerStyleJSON string) (inked, black int) {
		t.Helper()
		tpl, err := ParseTemplate([]byte(cascadeTableDoc(headerStyleJSON)))
		if err != nil {
			t.Fatalf("ParseTemplate: %v", err)
		}
		pages, _, _, _, err := buildPageModel(tpl, mustDecodeData(t, `{"rows":[{"a":"one"},{"a":"two"}]}`), mustDecodeParams(t), testFontSet())
		if err != nil {
			t.Fatalf("buildPageModel: %v", err)
		}
		for _, run := range pages[0].Runs {
			if run.HasColor && run.Color == (pagemodel.Color{R: 0xc8, G: 0x1e, B: 0x1e}) {
				inked++
			} else {
				black++
			}
		}
		return inked, black
	}

	absent, absentBlack := inkOps(t, ``)
	empty, emptyBlack := inkOps(t, `"headerStyle": {},`)
	null, nullBlack := inkOps(t, `"headerStyle": {"color": null},`)
	if absent != 3 || absentBlack != 0 {
		t.Fatalf("headerStyle absent gives %d inked / %d black, want 3 / 0 — the fixture moved and this test's controls no longer apply", absent, absentBlack)
	}
	if empty != absent || emptyBlack != 0 {
		t.Errorf("headerStyle {} gives %d inked / %d black, want %d / 0", empty, emptyBlack, absent)
	}
	if null != absent || nullBlack != 0 {
		t.Errorf("headerStyle {\"color\": null} gives %d inked / %d black, want %d / 0 — an explicit null must fall through to style.color, exactly as headerStyle.background and headerStyle.border already do", null, nullBlack, absent)
	}

	// THE CONTROLS CHANGED SIDES AT SPEC-table-rules, and the change is
	// worth stating rather than deleting.
	//
	// They used to read "the background and border arms already fall
	// through on an explicit null" — a sibling to measure `color`'s 3
	// against. Those two arms no longer HAVE a `style` leg to fall
	// through to: §1 gives a table's `style.border`/`style.background`
	// to the table's own BOX, and `headerStyle`'s are the only
	// declaration that can put chrome on a header cell. So the control
	// now measures the other answer: a null on either arm paints NO
	// header cell and NO data cell, and the table's own box is the one
	// rect that carries the declaration — exactly ONE fill and exactly
	// ONE stroke for this document, whose `style` declares both.
	//
	// `color` is unaffected and still cascades, which is why the
	// assertion above is unchanged: colour describes the text inside a
	// cell, not the chrome around it.
	//
	// THE TWO ARMS EXPECT DIFFERENT STROKE COUNTS, and the difference is
	// a pre-existing loader behaviour this test now surfaces rather than
	// anything SPEC-table-rules did: `"border": null` does NOT decode as
	// a null Presence — decodeBorder reads the null as an empty object,
	// so the header ends up declaring a border with every sub-key
	// absent, which resolves to the format's own default (0.5pt, black,
	// all four edges) and strokes. `"background": null` really is a null
	// Presence and paints nothing. Both are asserted as measured.
	for _, control := range []struct {
		name, headerStyle       string
		wantFills, wantStrokes  int
		headerCellStrokesItsOwn bool
	}{
		{name: "background", headerStyle: `"headerStyle": {"background": null},`, wantFills: 1, wantStrokes: 1},
		{name: "border", headerStyle: `"headerStyle": {"border": null},`, wantFills: 1, wantStrokes: 2, headerCellStrokesItsOwn: true},
	} {
		tpl, err := ParseTemplate([]byte(cascadeTableDoc(control.headerStyle)))
		if err != nil {
			t.Fatalf("ParseTemplate: %v", err)
		}
		pages, _, _, _, err := buildPageModel(tpl, mustDecodeData(t, `{"rows":[{"a":"one"},{"a":"two"}]}`), mustDecodeParams(t), testFontSet())
		if err != nil {
			t.Fatalf("buildPageModel: %v", err)
		}
		// The frame is drawn per page slice after pagination: its fill is
		// the FIRST rect (under every cell fill), its stroke the LAST.
		rects := pages[0].Rects
		fills, strokes := 0, 0
		fillIndex, strokeIndex, headerIndex := 0, len(rects)-1, 0
		if len(rects) > 0 && rects[0].HasFill {
			headerIndex = 1
		}
		for i, rect := range rects {
			headerCell := i == headerIndex
			if rect.HasFill {
				fills++
				if i != fillIndex {
					t.Errorf("%s control: rect %d fills; only the table's own box may", control.name, i)
				}
			}
			if rect.HasStroke {
				strokes++
				// A DATA cell may never stroke. The header cell may,
				// but only from headerStyle's own border — never from
				// the table's style, which is the box's.
				if i != strokeIndex && !(headerCell && control.headerCellStrokesItsOwn) {
					t.Errorf("%s control: rect %d strokes; no cell may carry the element's own style.border", control.name, i)
				}
			}
		}
		if fills != control.wantFills || strokes != control.wantStrokes {
			t.Errorf("%s control: %d fills / %d stroke groups, want %d / %d — the table's own style paints the BOX and reaches no cell", control.name, fills, strokes, control.wantFills, control.wantStrokes)
		}
	}
}

// TestAMalformedTableColourIsLocatedAtItsOwnBlock verifies the load error's
// field path for a malformed table colour: a bad headerStyle.color is
// located at headerStyle.color, and a bad style.color at style.color, never
// at the sibling block. Colour is refused at load (owner ruling,
// 2026-09-17), where each block is decoded under its own field prefix, so
// this asserts that prefix — it no longer observes the render cascade.
//
// It asserts the LOCATED PATH, never the surrounding wording.
func TestAMalformedTableColourIsLocatedAtItsOwnBlock(t *testing.T) {
	for _, tc := range []struct {
		name, headerStyle, table, wantPath, wantAbsent string
	}{
		{
			name:        "headerStyle wins",
			headerStyle: `"headerStyle": {"color": "red"},`,
			wantPath:    "headerStyle.color",
			wantAbsent:  "style.color/",
		},
		{
			name:        "style is what cascaded",
			headerStyle: ``,
			table:       "bad-base",
			wantPath:    "style.color",
			wantAbsent:  "headerStyle.color",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := cascadeTableDoc(tc.headerStyle)
			if tc.table == "bad-base" {
				doc = strings.Replace(doc, `"color": "#c81e1e"`, `"color": "red"`, 1)
			}
			requireColourLoadError(t, doc, "e1", tc.wantPath)
			_, err := ParseTemplate([]byte(doc))
			msg := err.Error()
			if !strings.Contains(msg, tc.wantPath) {
				t.Errorf("the diagnostic does not name %q: %s", tc.wantPath, msg)
			}
			if strings.Contains(msg, tc.wantAbsent) {
				t.Errorf("the load error names the sibling block (%q): %s", tc.wantAbsent, msg)
			}
		})
	}
}
