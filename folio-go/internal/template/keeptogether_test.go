package template

import (
	"strings"
	"testing"
)

// keepTogetherDocIn builds a one-element document with the element
// declared in the named band, carrying the given raw attribute text
// (e.g. `"keepTogether": "signature", `) on the element itself.
//
// The band is a PARAMETER because the whole point of the refusal under
// test is that the same element text is legal in one band and refused in
// the other two — a fixture that could only express the content band
// would assert nothing about the refusal.
func keepTogetherDocIn(band, elementType, attr string) string {
	elem := `{"id": "e1", "type": "` + elementType + `", "x": 0, "y": 0, "width": 200, "height": 40, ` + attr + `"value": "v"}`
	if elementType == "table" {
		elem = `{"id": "e1", "type": "table", "x": 0, "y": 0, ` + attr +
			`"bind": "rows[]", "as": "row", "headerHeight": 20, "columns": [{"id": "e2", "label": "L", "width": 100, "bind": "{{row.v}}"}]}`
	}
	bands := map[string]string{
		"content":    `"content": {"elements": [` + elem + `]}`,
		"pageHeader": `"pageHeader": {"elements": [` + elem + `], "height": 20}`,
		"pageFooter": `"pageFooter": {"elements": [` + elem + `], "height": 20}`,
	}
	if _, ok := bands[band]; !ok {
		panic("keepTogetherDocIn: unknown band " + band)
	}
	for _, b := range []string{"content", "pageHeader", "pageFooter"} {
		if b == band {
			continue
		}
		switch b {
		case "content":
			bands[b] = `"content": {"elements": []}`
		default:
			bands[b] = `"` + b + `": {"elements": [], "height": 20}`
		}
	}
	return `{
  "assets": {},
  "bands": {` + bands["content"] + `, ` + bands["pageFooter"] + `, ` + bands["pageHeader"] + `},
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

// keepTogetherKeyForms are the two spellings of DECLARING the key that
// the two refusals below must treat identically.
//
// The refusals are about the KEY, not its value: no reading of
// `"keepTogether": null` on a page-header element or on a table is ever
// honoured, so accepting it there would be a silent no-op that also
// raised the saved version to 1.2. Before this was fixed the refusals
// lived inside the non-null arm and every null form loaded clean, and
// these tests only ever passed a non-null string — which is why the
// null form is enumerated here rather than assumed to follow.
var keepTogetherKeyForms = []struct {
	label string
	attr  string
}{
	{"tag", `"keepTogether": "signature", `},
	{"null", `"keepTogether": null, `},
}

// TestKeepTogetherIsContentBandOnly is FR51's scope, enforced at LOAD
// rather than ignored at render.
//
// A page header and a page footer are repeated verbatim on every page
// and never reach the paginator as column items at all, so a tag there
// could never be honoured. Accepting it silently would leave an author
// looking at a block that never groups, with nothing to read.
func TestKeepTogetherIsContentBandOnly(t *testing.T) {
	for _, form := range keepTogetherKeyForms {
		if _, err := ParseDocument([]byte(keepTogetherDocIn("content", "text", form.attr))); err != nil {
			t.Fatalf("the control must load — the %s form on a content-band text element is legal (null means ungrouped): %v", form.label, err)
		}
	}
	for _, band := range []string{"pageHeader", "pageFooter"} {
		for _, form := range keepTogetherKeyForms {
			t.Run(band+"/"+form.label, func(t *testing.T) {
				_, err := ParseDocument([]byte(keepTogetherDocIn(band, "text", form.attr)))
				if err == nil {
					t.Fatalf("the keepTogether key in its %s form on a %s element must be a load error — the key can never be honoured there whatever its value", form.label, band)
				}
				var le *LoadError
				if !asLoadError(err, &le) {
					t.Fatalf("want a located LoadError, got %T: %v", err, err)
				}
				if le.Field != "keepTogether" {
					t.Errorf("LoadError.Field = %q, want %q — the error must name the field", le.Field, "keepTogether")
				}
				if le.ElementID != "e1" {
					t.Errorf("LoadError.ElementID = %q, want %q — the error must name the element", le.ElementID, "e1")
				}
			})
		}
	}
}

// TestKeepTogetherIsNotValidOnATable is the second refusal, and its
// reason is structural rather than stylistic: layout.ColumnItem.Group is
// ONE key per item and a table's rows already carry theirs, so a tagged
// table would need a second grouping model.
func TestKeepTogetherIsNotValidOnATable(t *testing.T) {
	for _, form := range keepTogetherKeyForms {
		t.Run(form.label, func(t *testing.T) {
			_, err := ParseDocument([]byte(keepTogetherDocIn("content", "table", form.attr)))
			if err == nil {
				t.Fatalf("the keepTogether key in its %s form on a table must be a load error — a table can never carry a second grouping identity whatever the key's value", form.label)
			}
			var le *LoadError
			if !asLoadError(err, &le) {
				t.Fatalf("want a located LoadError, got %T: %v", err, err)
			}
			if le.Field != "keepTogether" || le.ElementID != "e1" {
				t.Errorf("LoadError = {Field: %q, ElementID: %q}, want the field and the element named", le.Field, le.ElementID)
			}
		})
	}
	// The control: the SAME table without the key loads, so the refusal
	// is about the key and not about the table document being malformed.
	if _, err := ParseDocument([]byte(keepTogetherDocIn("content", "table", ""))); err != nil {
		t.Fatalf("the same table without the key must load: %v", err)
	}
}

// TestKeepTogetherRefusesAnEmptyTag: the value NAMES a group. An empty
// string names none, and would silently join every other ""-tagged
// element in the document into one union extent.
func TestKeepTogetherRefusesAnEmptyTag(t *testing.T) {
	_, err := ParseDocument([]byte(keepTogetherDocIn("content", "text", `"keepTogether": "", `)))
	if err == nil {
		t.Fatal("an empty keepTogether tag must be a load error")
	}
	if !strings.Contains(err.Error(), "keepTogether") {
		t.Errorf("the error must name the field, got: %v", err)
	}
}

// TestKeepTogetherPresenceRoundTrips pins the three-way Presence
// polarity for this key: absent, explicit null and present-with-value
// are three distinct states that survive parse and serialize.
func TestKeepTogetherPresenceRoundTrips(t *testing.T) {
	for _, c := range []struct {
		label    string
		attr     string
		wantSet  bool
		wantNull bool
		wantVal  string
		wantJSON string
	}{
		{"absent", "", false, false, "", ""},
		{"null", `"keepTogether": null, `, true, true, "", `"keepTogether": null`},
		{"present", `"keepTogether": "signature", `, true, false, "signature", `"keepTogether": "signature"`},
	} {
		t.Run(c.label, func(t *testing.T) {
			d, err := ParseDocument([]byte(keepTogetherDocIn("content", "text", c.attr)))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := d.Bands.Content.Elements[0].KeepTogether
			if got.Set != c.wantSet || got.Null != c.wantNull || got.Value != c.wantVal {
				t.Fatalf("KeepTogether = %+v, want {Set:%v Null:%v Value:%q}", got, c.wantSet, c.wantNull, c.wantVal)
			}
			out, err := SerializeDocument(d)
			if err != nil {
				t.Fatalf("serialize: %v", err)
			}
			if c.wantJSON == "" {
				if strings.Contains(string(out), "keepTogether") {
					t.Fatalf("an absent key must emit nothing:\n%s", out)
				}
				return
			}
			if !strings.Contains(string(out), c.wantJSON) {
				t.Fatalf("want %s in the emission:\n%s", c.wantJSON, out)
			}
		})
	}
}

// asLoadError is errors.As with this package's own error type, spelled
// once so the assertions above read as assertions.
func asLoadError(err error, target **LoadError) bool {
	le, ok := err.(*LoadError)
	if !ok {
		return false
	}
	*target = le
	return true
}
