package template

import (
	"reflect"
	"strings"
	"testing"
)

// tableWidthDoc, tableHeightDoc (AC1, instrument 1a) are TEST-OWNED
// fixture documents (D-000.68's anchor: the JSON is authored here, not
// derived from the parser's own field list). Each declares a "width" (or
// "height") key on a table element, which parse_bands.go:158-164
// already rejects at load — measured at story creation to have ZERO
// tests (D4) despite being AC1's single strongest existing guarantee.
const tableWidthDoc = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {
          "bind": "items[]",
          "columns": [{"bind": "{{row.n}}", "id": "e2", "label": "N", "width": 40}],
          "headerHeight": 14,
          "id": "e1",
          "type": "table",
          "width": 500,
          "x": 0,
          "y": 0
        }
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 3,
  "page": {
    "margin": {"bottom": 36, "left": 36, "right": 36, "top": 36},
    "orientation": "portrait",
    "size": "A4"
  },
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

const tableHeightDoc = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {
          "bind": "items[]",
          "columns": [{"bind": "{{row.n}}", "id": "e2", "label": "N", "width": 40}],
          "headerHeight": 14,
          "height": 300,
          "id": "e1",
          "type": "table",
          "x": 0,
          "y": 0
        }
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 3,
  "page": {
    "margin": {"bottom": 36, "left": 36, "right": 36, "top": 36},
    "orientation": "portrait",
    "size": "A4"
  },
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// tableValidDoc is tableWidthDoc/tableHeightDoc with neither "width" nor
// "height" — the control that must load cleanly, so a caller can tell
// "rejected because of width/height" apart from "rejected for some other
// reason" (D-000.9: an all-clear must not be indistinguishable from
// "could not look").
const tableValidDoc = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {
          "bind": "items[]",
          "columns": [{"bind": "{{row.n}}", "id": "e2", "label": "N", "width": 40}],
          "headerHeight": 14,
          "id": "e1",
          "type": "table",
          "x": 0,
          "y": 0
        }
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 3,
  "page": {
    "margin": {"bottom": 36, "left": 36, "right": 36, "top": 36},
    "orientation": "portrait",
    "size": "A4"
  },
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// TestTableDeclaringWidthIsRejected is AC1 instrument 1a's width half.
//
// Discriminating mutation (run and recorded in the story's Delivery
// Log): delete the "if el.Type == ElementTable { if wok {...} }" branch
// at parse_bands.go:158 — the document then loads cleanly and "width"
// lands on Element.Width. This test reddens under that mutation.
func TestTableTotalWithAbsoluteColumnsIsRejected(t *testing.T) {
	_, err := ParseDocument([]byte(tableWidthDoc))
	if err == nil {
		t.Fatal("a table total with absolute columns must be a load error; got nil error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "e2") {
		t.Errorf("error must name the column id \"e2\": %v", err)
	}
	if !strings.Contains(msg, "width") {
		t.Errorf("error must name the field \"width\": %v", err)
	}
}

// TestTableDeclaringHeightIsRejected is AC1 instrument 1a's height half.
func TestTableDeclaringHeightIsRejected(t *testing.T) {
	_, err := ParseDocument([]byte(tableHeightDoc))
	if err == nil {
		t.Fatal("a table declaring \"height\" must be a load error (AD-13, AC1); got nil error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "e1") {
		t.Errorf("error must name the element id \"e1\": %v", err)
	}
	if !strings.Contains(msg, "height") {
		t.Errorf("error must name the field \"height\": %v", err)
	}
}

// TestTableWithNeitherWidthNorHeightLoads is the D-000.9 control: the
// identical fixture minus the offending key must load cleanly, so the
// two tests above are known to be testing the width/height rejection
// specifically and not some unrelated fixture defect.
func TestTableWithNeitherWidthNorHeightLoads(t *testing.T) {
	if _, err := ParseDocument([]byte(tableValidDoc)); err != nil {
		t.Fatalf("a table declaring neither width nor height must load: %v", err)
	}
}

// wantTableExtFields, wantColumnFields (AC1 instrument 1b) are
// TEST-OWNED literals (R5, D-000.68): TableExt's and Column's field sets
// are PERMANENT through the MVP (R5), so a literal — not a derivation —
// is the right anchor. Extra is excluded by name on Column (finisher
// fix, Story 4.1 review Finding 15: TableExt itself carries no Extra
// field at all — model.go's passthrough carrier for unknown keys
// (AD-9/D-1.4.9) is declared only on Column — so fieldNameSet's
// "Extra" skip below is inert for TableExt and this is not a second
// exclusion). HeaderStyle is a RULED, deliberate exception to R5's
// "TableExt's field set is permanent through the MVP": Story 4.1's own
// Delivery Log
// records the owner's ruling that a table gets a headerStyle block, so
// the author controls the header's appearance — landing in 4.1 rather
// than being deferred, because 4.1 is already the story that attaches
// style to the header row.
//
// Rules and MinHeight are the SECOND ruled exception, and they arrive
// together because they are one capability: SPEC-table-rules splits a
// table's perimeter from its interior, and a rule that stops at the last
// row instead of at the box's bottom border is not the thing the spec
// describes. Both are recorded as owner-ruled in that spec's frozen
// Intent block, exactly as HeaderStyle is recorded in Story 4.1's
// Delivery Log.
var wantTableExtFields = map[string]bool{
	"Bind": true, "As": true, "Columns": true, "HeaderHeight": true, "AltRowBackground": true,
	"HeaderStyle": true, "Rules": true, "MinHeight": true,
}

var wantColumnFields = map[string]bool{
	"ID": true, "Label": true, "Width": true, "Proportion": true, "Align": true, "Bind": true,
	"Footer": true, "FooterOf": true, "FooterFormat": true,
	// Added by the table-cell-padding/header-align spec (2026-09-13): an
	// optional per-column header alignment, its own closed set, raising 3.2.
	"HeaderAlign": true,
}

// fieldNameSet reflects over typ's exported field names, explicitly
// excluding "Extra" (documented passthrough carrier, never a schema
// field).
func fieldNameSet(t *testing.T, typ reflect.Type) map[string]bool {
	t.Helper()
	got := map[string]bool{}
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		if name == "Extra" {
			continue
		}
		got[name] = true
	}
	if len(got) == 0 {
		t.Fatal("vacuity guard (D-000.9): reflected zero fields")
	}
	return got
}

// assertFieldSetsEqual is a set-equality assertion in BOTH directions,
// per AC1 1b: missing an expected field AND an unexpected extra field
// (e.g. a renamed one) both fail it.
func assertFieldSetsEqual(t *testing.T, typeName string, want, got map[string]bool) {
	t.Helper()
	compared := 0
	for name := range want {
		compared++
		if !got[name] {
			t.Errorf("%s: expected field %q not found in the reflected set %v", typeName, name, got)
		}
	}
	for name := range got {
		compared++
		if !want[name] {
			t.Errorf("%s: unexpected field %q in the reflected set (not in %v) — R5's field set is permanent", typeName, name, want)
		}
	}
	if compared == 0 {
		t.Fatal("vacuity guard (D-000.9): zero comparisons made")
	}
}

// TestTableExtFieldSetIsPermanent is AC1 instrument 1b's TableExt half.
//
// Discriminating mutation (run and recorded): add `Width geom.Length` to
// TableExt — reds, naming the added field. Renaming an existing field
// (e.g. Columns -> Cols) also reds: the guard compares the SET, not a
// spelling.
func TestTableExtFieldSetIsPermanent(t *testing.T) {
	got := fieldNameSet(t, reflect.TypeOf(TableExt{}))
	assertFieldSetsEqual(t, "TableExt", wantTableExtFields, got)
}

// TestColumnFieldSetIsPermanent is AC1 instrument 1b's Column half.
func TestColumnFieldSetIsPermanent(t *testing.T) {
	got := fieldNameSet(t, reflect.TypeOf(Column{}))
	assertFieldSetsEqual(t, "Column", wantColumnFields, got)
}

// headerStyleBadValignDoc carries BOTH a "style" block and a
// "headerStyle" block with an invalid "valign" — this is Story 4.1
// finisher fix, review Finding 5: every load error previously raised
// INSIDE headerStyle named the field "style.<x>", sending a template
// author who mistyped inside headerStyle to the wrong block. Without
// the sibling "style" block present, a wrongly-located message would
// be indistinguishable from a correctly-located one (both would
// contain "valign"); the sibling block is what makes the assertion
// below discriminate.
const headerStyleBadValignDoc = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {
          "bind": "items[]",
          "columns": [{"bind": "{{row.n}}", "id": "e2", "label": "N", "width": 40}],
          "headerHeight": 14,
          "id": "e1",
          "style": {"valign": "top"},
          "headerStyle": {"valign": "sideways"},
          "type": "table",
          "x": 0,
          "y": 0
        }
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 3,
  "page": {
    "margin": {"bottom": 36, "left": 36, "right": 36, "top": 36},
    "orientation": "portrait",
    "size": "A4"
  },
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// TestHeaderStyleLoadErrorsNameHeaderStyleNotStyle is the finisher's
// fix for Story 4.1 review Finding 5 (Major): a malformed field inside
// a table's "headerStyle" block must be located at "headerStyle.<x>",
// never at "style.<x>" (its sibling attach point on the same element).
//
// Discriminating mutation: revert decodeStyle's fieldPrefix threading
// (hardcode "style" in its newLoadError calls again) — reds, because
// the message would then name "style.valign" while this element's OWN
// "style" block is a valid, different value ("top"), so the message
// would point a template author at the wrong block entirely.
func TestHeaderStyleLoadErrorsNameHeaderStyleNotStyle(t *testing.T) {
	_, err := ParseDocument([]byte(headerStyleBadValignDoc))
	if err == nil {
		t.Fatal("a headerStyle.valign outside the closed set must be a load error; got nil error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "headerStyle.valign") {
		t.Errorf("error must name the field as \"headerStyle.valign\" (not its sibling \"style.valign\"): %v", err)
	}
	if strings.Contains(msg, "field style.valign") {
		t.Errorf("error wrongly located the headerStyle field at \"style.valign\": %v", err)
	}
}
