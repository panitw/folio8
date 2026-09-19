package folio8

import (
	"fmt"
	"strings"
	"testing"
)

// tableBindTestTemplateJSON is a `.folio` document with one table
// element bound to a collection path ("transactions[]", source AC5)
// and one column whose bind stays expression-shaped (D-1.6.8's fence
// — Story 3.1 does not evaluate columns[].bind, that is 3.2).
const tableBindTestTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e2", "type": "table", "x": 0, "y": 30, "bind": "transactions[]", "headerHeight": 20,
        "as": "transaction", "style": {"fontFamily": "body", "fontSize": 9},
        "columns": [{"id": "e3", "label": "Amount", "width": 80, "bind": "{{transaction.amount}}"}]}
    ]},
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

// TestRenderTableBindNonArrayFailsRender is source AC5: a collection
// binding whose path is not an array fails the render, naming the
// bind as authored and the element id. All four non-array shapes are
// required (measured at e7f3f9c to render SUCCESSFULLY before this
// story: finding 3) — the fixture and its recorded four failures are
// this story's task 1, re-homed here.
func TestRenderTableBindNonArrayFailsRender(t *testing.T) {
	tpl, err := ParseTemplate([]byte(tableBindTestTemplateJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	for _, tc := range []struct{ name, data string }{
		{"absent", `{"customer": {"name": "Ada"}}`},
		{"scalar", `{"transactions": 7}`},
		{"object", `{"transactions": {"a": 1}}`},
		{"null", `{"transactions": null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, rerr := Render(tpl, Data(tc.data), nil, testFontSet())
			if rerr == nil {
				t.Fatalf("AC5: render SUCCEEDED on a table bind %q that is not an array (%s)", "transactions[]", tc.name)
			}
			msg := rerr.Error()
			if !strings.Contains(msg, "transactions[]") {
				t.Errorf("AC5: error must name the bind AS AUTHORED (%q); got %q", "transactions[]", msg)
			}
			if !strings.Contains(msg, "e2") {
				t.Errorf("AC5: error must name the element id; got %q", msg)
			}
			t.Logf("AC5 error: %s", msg)
		})
	}
}

// TestRenderTableBindEmptyArrayIsNotAnError is source AC5's own carve
// out: an empty array is an array, not an error. Story 4.2 owns what
// an empty collection renders as; this story only proves the render
// does not fail.
func TestRenderTableBindEmptyArrayIsNotAnError(t *testing.T) {
	tpl, err := ParseTemplate([]byte(tableBindTestTemplateJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	res, rerr := Render(tpl, Data(`{"transactions": []}`), nil, testFontSet())
	if rerr != nil {
		t.Fatalf("AC5: an empty array must not fail the render, got: %v", rerr)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("Render produced no bytes")
	}
}

// aliasCollisionTemplateJSON builds a table declaring the given "as"
// alias, colliding with a reserved name.
func aliasCollisionTemplateJSON(alias string) string {
	return `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e2", "type": "table", "x": 0, "y": 30, "bind": "transactions[]", "headerHeight": 20,
        "as": "` + alias + `",
        "columns": [{"id": "e3", "label": "Amount", "width": 80, "bind": "amount"}]}
    ]},
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

// TestRenderTableRowAliasCollidesWithReservedNameFailsRender is
// D-3.1.1's ruling on the creator's flagged OD-1 (the arm the ruling
// took): a repeating region declaring "as": "params", "as": "page" or
// "as": "pages" is a located template error naming the element —
// "params" because AD-11 forbids shadowing it, "page"/"pages" because
// AD-4 forbids that namespace forever. This is a DECLARATION-LEVEL
// check: it fails identically with NO report data supplied for the
// collection at all, proving it is data-free (same category as
// D-2.6.5/D-2.7.3).
//
// Review Finding 1 (Major): the message's static explanatory text
// enumerates all three reserved names ("params" ... "page"/"pages"
// ...) regardless of what actually collided, so asserting only
// strings.Contains(msg, alias) is satisfied by that constant text, not
// by the interpolated value — a reported wrong or constant alias would
// still pass. The fix asserts on the exact substring only a correct
// interpolation of THIS alias can produce (`row alias "<alias>"
// collides`), which the static enumeration never contains verbatim.
func TestRenderTableRowAliasCollidesWithReservedNameFailsRender(t *testing.T) {
	for _, alias := range []string{"params", "page", "pages"} {
		t.Run(alias, func(t *testing.T) {
			tpl, err := ParseTemplate([]byte(aliasCollisionTemplateJSON(alias)))
			if err != nil {
				t.Fatalf("ParseTemplate: %v", err)
			}
			// No "transactions" key at all: if the render reached the
			// collection-bind check first, it would fail on THAT
			// instead — proving this check is both reached and
			// data-free requires the alias failure specifically.
			_, rerr := Render(tpl, Data(`{}`), nil, testFontSet())
			if rerr == nil {
				t.Fatalf("a row alias %q must be a located template error naming the element", alias)
			}
			msg := rerr.Error()
			// The interpolated region: "table's row alias %q collides"
			// — this substring cannot be produced by the constant
			// enumeration text, only by %q(alias) actually matching.
			wantInterpolated := fmt.Sprintf("row alias %q collides", alias)
			if !strings.Contains(msg, wantInterpolated) {
				t.Errorf("error must name the colliding alias via interpolation (want %q); got %q", wantInterpolated, msg)
			}
			if !strings.Contains(msg, "e2") {
				t.Errorf("error must name the element id; got %q", msg)
			}
			t.Logf("alias %q error: %s", alias, msg)
		})
	}
}
