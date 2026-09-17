package folio8

import (
	"errors"
	"strings"
	"testing"
)

// requireColourLoadError asserts that source is refused at LOAD with
// TEMPLATE_FIELD_INVALID, located at elementID (skipped when "") and naming
// field — and that Validate, which loads the same bytes first, refuses it
// identically, so no render can start.
func requireColourLoadError(t *testing.T, source, elementID, field string) {
	t.Helper()
	check := func(door string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: a colour that is not #RRGGBB at %s loaded without error", door, field)
		}
		var re *RenderError
		if !errors.As(err, &re) {
			t.Fatalf("%s: error is %T (%v), want a located *RenderError", door, err, err)
		}
		if re.Diagnostic.Code != DiagCodeTemplateFieldInvalid {
			t.Errorf("%s: code = %q, want %q", door, re.Diagnostic.Code, DiagCodeTemplateFieldInvalid)
		}
		if re.Diagnostic.Severity != SeverityError {
			t.Errorf("%s: severity = %s, want Error", door, re.Diagnostic.Severity)
		}
		if elementID != "" && re.Diagnostic.ElementID != elementID {
			t.Errorf("%s: ElementID = %q, want %q", door, re.Diagnostic.ElementID, elementID)
		}
		if !strings.Contains(re.Diagnostic.Message, field) {
			t.Errorf("%s: message %q does not name the field %s", door, re.Diagnostic.Message, field)
		}
		if !strings.Contains(re.Diagnostic.Message, "#RRGGBB") {
			t.Errorf("%s: message %q does not say what a colour must be", door, re.Diagnostic.Message)
		}
	}
	_, err := ParseTemplate([]byte(source))
	check("ParseTemplate", err)
	_, verr := Validate([]byte(source), Data(`{}`), nil, testFontSet())
	check("Validate", verr)
}

// colourLoadDoc is a one-element document: a hidden text element (so a
// render would never draw it) when table is false, or an empty-bound table
// (so a render would draw no row) when it is true. elementFields is spliced
// into the element.
func colourLoadDoc(table bool, elementFields string) string {
	element := `{"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 30, "value": "Hidden", "visibleIf": "false", ` + elementFields + `}`
	if table {
		element = `{"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "rows[]", "as": "row", "headerHeight": 20, ` + elementFields + `,
          "columns": [{"id": "e2", "label": "A", "width": 100, "bind": "{{row.a}}"}]}`
	}
	return `{
  "assets": {},
  "bands": {
    "content": {"elements": [` + element + `]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "4.1"
}
`
}

// TestEveryColourFieldIsCheckedAtLoad is the owner's 2026-09-17 ruling as
// a table: every colour field the format has is refused at load when it is
// not #RRGGBB, located at that field, whether or not a render would ever
// draw it — and the same document with a well-formed colour loads.
func TestEveryColourFieldIsCheckedAtLoad(t *testing.T) {
	const style = `"fontFamily": "body", "fontSize": 10`
	for _, c := range []struct {
		name   string
		table  bool
		fields func(colour string) string
		field  string
	}{
		{"style.color", false, func(c string) string { return `"style": {"color": "` + c + `", ` + style + `}` }, "style.color"},
		{"style.background", false, func(c string) string { return `"style": {"background": "` + c + `", ` + style + `}` }, "style.background"},
		{"style.border.color", false, func(c string) string {
			return `"style": {"border": {"color": "` + c + `", "edges": ["top"]}, ` + style + `}`
		}, "style.border.color"},
		{"table style.color", true, func(c string) string { return `"style": {"color": "` + c + `", ` + style + `}` }, "style.color"},
		{"table style.background", true, func(c string) string { return `"style": {"background": "` + c + `", ` + style + `}` }, "style.background"},
		{"table style.border.color", true, func(c string) string { return `"style": {"border": {"color": "` + c + `"}, ` + style + `}` }, "style.border.color"},
		{"headerStyle.color", true, func(c string) string { return `"style": {` + style + `}, "headerStyle": {"color": "` + c + `"}` }, "headerStyle.color"},
		{"headerStyle.background", true, func(c string) string { return `"style": {` + style + `}, "headerStyle": {"background": "` + c + `"}` }, "headerStyle.background"},
		{"headerStyle.border.color", true, func(c string) string {
			return `"style": {` + style + `}, "headerStyle": {"border": {"color": "` + c + `"}}`
		}, "headerStyle.border.color"},
		{"altRowBackground", true, func(c string) string { return `"style": {` + style + `}, "altRowBackground": "` + c + `"` }, "altRowBackground"},
		{"rules.color", true, func(c string) string {
			return `"style": {` + style + `}, "rules": {"between": ["columns"], "color": "` + c + `"}`
		}, "rules.color"},
	} {
		t.Run(c.name, func(t *testing.T) {
			for _, bad := range []string{"#12345", "red", "", "#1234567", "#GG0000", "{{row.colour}}"} {
				requireColourLoadError(t, colourLoadDoc(c.table, c.fields(bad)), "e1", c.field)
				_, err := ParseTemplate([]byte(colourLoadDoc(c.table, c.fields(bad))))
				byData := strings.Contains(err.Error(), "data-driven styling is not supported")
				if wantByData := strings.Contains(bad, "{{"); byData != wantByData {
					t.Errorf("%q: message %q names data-driven styling = %v, want %v", bad, err, byData, wantByData)
				}
			}
			for _, good := range []string{"#1b2a4a", "#C81E1E"} {
				if _, err := ParseTemplate([]byte(colourLoadDoc(c.table, c.fields(good)))); err != nil {
					t.Errorf("a well-formed colour %s at %s failed to load: %v", good, c.field, err)
				}
			}
		})
	}
}
