package folio8

import (
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/panitw/folio8/folio-go/internal/designer"
)

// STORY 14.9 — THE CANVAS DRAWS THE TABLE IT WILL PRINT, and this file is the
// behavioural half of the projection that lets it.
//
// The wire record next door (canvas_projection_wire_test.go) pins the KEY SET
// on both sides of the seam. This file pins the VALUES: that each key carries
// what the engine will actually use, that the two resolved alignments come from
// the two DIFFERENT cascades the renderer runs, and that nothing which loads
// and paints today is newly refused.
//
// Every assertion carries a `probed == 0`-style precondition, in the shape
// component_properties_test.go's tableBind check uses, so none of them can pass
// vacuously on a document that happens to have no table in it.

// canvasSplitAlignTableTemplateJSON is R2's fixture, and its whole job is to
// make the two resolved alignments DIFFER.
//
// The table declares `style.align: "center"` and `headerStyle.align: "right"`.
// resolveHeaderStyle takes headerStyle.align first, so a column that declares
// no `align` of its own resolves to "right" in the HEADER row and "center" in
// the DATA row. Without a fixture in which they differ, both keys could be
// populated from ONE cascade and every other assertion in this file would still
// pass — the both-sides-move-together shape, and the exact reading R2 refused.
//
// Column e3 declares no `align`: it is the one that splits.
// Column e4 declares `align: "left"`: the column's own value is the most
// specific of the three and wins over BOTH fallbacks, so its two keys agree.
// Column e5 is the tolerated-degenerate column — width 0, an empty label and an
// empty bind, all three legal at load and all three drawn rather than refused.
const canvasSplitAlignTableTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Opening balance", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "table", "x": 0, "y": 1440, "as": "row", "bind": "transactions[]", "headerHeight": 16,
         "columns": [
           {"id": "e3", "label": "Date", "width": 80, "bind": "{{row.date}}"},
           {"id": "e4", "label": "Amount", "width": 60, "align": "left", "bind": "{{row.amount}}"},
           {"id": "e5", "label": "", "width": 0, "bind": ""}
         ],
         "style": {"fontFamily": "body", "fontSize": 8, "align": "center"},
         "headerStyle": {"align": "right"}}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 24
    },
    "pageHeader": {
      "elements": [],
      "height": 18
    }
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 7,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

func canvasTableColumnsOf(t *testing.T, projection designer.CanvasProjection, id string) []designer.CanvasTableColumn {
	t.Helper()
	for _, component := range projection.Components {
		if component.ID == id {
			return component.Columns
		}
	}
	t.Fatalf("component %s is not in the projection", id)
	return nil
}

// TestCanvasTableColumnsCarryTheDeclaredValues is the plain half: the label,
// the width in MILLIPOINTS (unconverted — the wire unit) and the binding, each
// verbatim from the document.
func TestCanvasTableColumnsCarryTheDeclaredValues(t *testing.T) {
	projection := projectWithPaint(t, parseWindowCountTemplate(t, canvasSplitAlignTableTemplateJSON))
	columns := canvasTableColumnsOf(t, projection, "e2")
	if len(columns) != 3 {
		t.Fatalf("fixture precondition: want the table's three declared columns, got %d — %#v", len(columns), columns)
	}
	for index, want := range []designer.CanvasTableColumn{
		{ID: "e3", Label: "Date", Width: 80000, Bind: "{{row.date}}"},
		{ID: "e4", Label: "Amount", Width: 60000, Bind: "{{row.amount}}"},
		{ID: "e5", Label: "", Width: 0, Bind: ""},
	} {
		got := columns[index]
		if got.ID != want.ID || got.Label != want.Label || got.Width != want.Width || got.Bind != want.Bind {
			t.Errorf("column %d projects id=%q label=%q width=%d bind=%q, want id=%q label=%q width=%d bind=%q", index, got.ID, got.Label, got.Width, got.Bind, want.ID, want.Label, want.Width, want.Bind)
		}
	}
	// The declared order is the drawn order: the canvas lays the tracks out
	// left to right in this sequence, so a reordering here would move a
	// heading over the wrong column.
	if columns[0].ID != "e3" || columns[2].ID != "e5" {
		t.Errorf("the projected columns are not in the document's declared order: %#v", columns)
	}
}

// TestCanvasTableColumnsResolveBothRowsAlignments is R2, and it is the reason
// this projection carries TWO alignment keys rather than one.
//
// ⚠ IT ASSERTS WHICH ROW CONSUMES WHICH KEY, NOT MERELY THAT BOTH EXIST.
// SWAPPING `headerAlign` and `cellAlign` in canvasTableColumns MUST RED THIS
// TEST — a presence assertion cannot see a swap, and two keys populated from
// one cascade pass everything else in this file.
func TestCanvasTableColumnsResolveBothRowsAlignments(t *testing.T) {
	projection := projectWithPaint(t, parseWindowCountTemplate(t, canvasSplitAlignTableTemplateJSON))
	columns := canvasTableColumnsOf(t, projection, "e2")
	if len(columns) < 2 {
		t.Fatalf("fixture precondition: R2 needs a column that declares no align and one that does, got %#v", columns)
	}
	split := columns[0]
	// THE TWO GENUINELY DIFFER, and this is the precondition the whole test
	// rests on: if the fixture ever stopped splitting them, every assertion
	// below would still pass with both keys fed from one cascade.
	if split.HeaderAlign == split.CellAlign {
		t.Fatalf("fixture precondition: column %q resolves the same alignment for both rows (%q), so this test cannot tell two cascades from one", split.ID, split.HeaderAlign)
	}
	// headerStyle.align wins the HEADER row (resolveHeaderStyle); the table's
	// own style.align wins the DATA row (resolveBodyStyle, which has no
	// headerStyle arm at all — D-000.76's fence).
	if split.HeaderAlign != "right" {
		t.Errorf("column %q resolves headerAlign=%q; the table declares headerStyle.align \"right\", which is what the PDF's header row prints", split.ID, split.HeaderAlign)
	}
	if split.CellAlign != "center" {
		t.Errorf("column %q resolves cellAlign=%q; the table declares style.align \"center\" and no headerStyle reaches a data row, which is what the PDF's data rows print", split.ID, split.CellAlign)
	}
	// A column's OWN align is the most specific of the three and wins over
	// both fallbacks, so this one agrees with itself — and that it agrees is
	// not the same claim as the split above.
	own := columns[1]
	if own.HeaderAlign != "left" || own.CellAlign != "left" {
		t.Errorf("column %q declares align \"left\" and resolves headerAlign=%q cellAlign=%q; a column's own align is the most specific declaration and wins over both row fallbacks", own.ID, own.HeaderAlign, own.CellAlign)
	}
}

// TestCanvasTableColumnsFallBackToLeftWithNoDeclaredAlignment pins the third
// arm of both cascades — the documented default — on a table that declares
// neither style.align nor headerStyle.align. Without it, "right"/"center" above
// could both be coming from a hardcoded fallback of the wrong kind, which is
// exactly what TableColumns' own align derivation does (`align := "left"`, then
// the column) and what this projection must NOT do.
func TestCanvasTableColumnsFallBackToLeftWithNoDeclaredAlignment(t *testing.T) {
	projection := projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountBoundTableTemplateJSON))
	columns := canvasTableColumnsOf(t, projection, "e1")
	if len(columns) == 0 {
		t.Fatal("fixture precondition: the bound-table fixture projected no columns, so the default-alignment assertion said nothing")
	}
	for _, column := range columns {
		if column.HeaderAlign != "left" || column.CellAlign != "left" {
			t.Errorf("column %q resolves headerAlign=%q cellAlign=%q on a table that declares no alignment anywhere; both cascades default to \"left\"", column.ID, column.HeaderAlign, column.CellAlign)
		}
	}
}

// TestCanvasTableColumnsAreAbsentRatherThanEmpty covers two rows of the matrix
// at once: a table with `columns: []` (legal at load, renders as nothing) emits
// no member, and neither does any non-table component.
//
// ABSENCE, NOT AN EMPTY ARRAY. The member is `omitempty`, the browser's guard
// admits its absence, and the canvas states in words that the table has no
// columns yet rather than drawing an empty frame.
func TestCanvasTableColumnsAreAbsentRatherThanEmpty(t *testing.T) {
	projection := projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountColumnlessTableTemplateJSON))
	tables, others := 0, 0
	for _, component := range projection.Components {
		if component.Type == "table" {
			tables++
			if component.Columns != nil {
				t.Errorf("a table declaring `columns: []` projected %d column(s): %#v", len(component.Columns), component.Columns)
			}
			// The bind is still projected: an empty column list says nothing
			// about the table's binding, and the chip still names it.
			if component.TableBind == nil {
				t.Errorf("a columnless table projected no tableBind: %#v", component)
			}
			continue
		}
		others++
		if component.Columns != nil {
			t.Errorf("a %s component carries columns: %#v", component.Type, component.Columns)
		}
	}
	if tables == 0 {
		t.Fatal("presence precondition: the columnless fixture projected no table at all, so the empty-columns assertion said nothing")
	}
	if others == 0 {
		t.Fatal("presence precondition: the columnless fixture projected no non-table component, so the cross-type assertion said nothing")
	}
}

// TestCanvasTableColumnsRefuseNothingTheDocumentAlreadyPaints is the fence
// around the derivations this story copied from TableColumns WITHOUT copying
// its gate. That projection hard-errors on `width <= 0`, on more than 128
// columns and on a bind that fails rootCollectionPath; a canvas-projection
// error blanks the WHOLE designer, so reusing the gate would newly kill
// documents that currently draw.
func TestCanvasTableColumnsRefuseNothingTheDocumentAlreadyPaints(t *testing.T) {
	// A zero width and a NEGATIVE width, an empty label and an empty bind —
	// every one of them legal at load today. The negative column is paired
	// with a wider one so the table's projected width (the column sum) stays
	// non-negative, which is a pre-existing component-level rule and not this
	// member's business.
	source := strings.Replace(canvasSplitAlignTableTemplateJSON,
		`{"id": "e5", "label": "", "width": 0, "bind": ""}`,
		`{"id": "e5", "label": "", "width": 0, "bind": ""},
           {"id": "e6", "label": "Negative", "width": -5, "bind": "{{row.x}}"}`, 1)
	if source == canvasSplitAlignTableTemplateJSON {
		t.Fatal("fixture precondition: the negative-width column was not spliced into the fixture, so this test measures the unmodified document")
	}
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("a zero-width and a negative-width column must still LOAD: %v", err)
	}
	projection, err := canvas(tpl)
	if err != nil {
		t.Fatalf("a zero-width or negative-width column was refused a canvas projection, which blanks the whole designer for a document that paints today: %v", err)
	}
	columns := canvasTableColumnsOf(t, projection, "e2")
	widths := map[string]int64{}
	for _, column := range columns {
		widths[column.ID] = column.Width
	}
	if got, ok := widths["e5"]; !ok || got != 0 {
		t.Errorf("the zero-width column projects width %d (present: %v); it must be carried verbatim, not dropped and not clamped", got, ok)
	}
	if got, ok := widths["e6"]; !ok || got != -5000 {
		t.Errorf("the negative-width column projects width %d (present: %v); it must be carried verbatim, not dropped and not clamped", got, ok)
	}
	// An empty label and an empty bind are DECLARED EMPTY, not absent: the
	// renderer builds the header rect and skips the glyphs, and the canvas
	// matches it.
	for _, column := range columns {
		if column.ID == "e5" && (column.Label != "" || column.Bind != "") {
			t.Errorf("the empty-label, empty-bind column projects label=%q bind=%q", column.Label, column.Bind)
		}
	}
}

// TestCanvasTableColumnStringsClipRatherThanRefuseTheDocument is the
// amendment's own test (Spec Change Log 1): the two bounded column strings
// CLIP and the column is still drawn.
//
// ⚠ WHY THIS IS NOT A STYLE CHOICE. `decodeColumn` caps neither `label` nor
// `bind`, so this document LOADS AND PRINTS — step-04 measured the golden
// fixture with a 600-byte column label rendering a real 64,123-byte PDF. An
// abort here would take a template that prints correctly and blank the
// designer's canvas until reload, because a canvas-projection error makes
// isCanvas false, parseInbound undefined and PROTOCOL_INVALID terminate the
// worker with no respawn. A document the engine prints is a document the canvas
// draws — which is the matrix's own answer, two rows away, for a zero or
// negative column width.
func TestCanvasTableColumnStringsClipRatherThanRefuseTheDocument(t *testing.T) {
	long := strings.Repeat("x", 600)
	source := strings.Replace(canvasSplitAlignTableTemplateJSON,
		`{"id": "e3", "label": "Date", "width": 80, "bind": "{{row.date}}"}`,
		`{"id": "e3", "label": "`+long+`", "width": 80, "bind": "{{row.`+long+`}}"}`, 1)
	if source == canvasSplitAlignTableTemplateJSON {
		t.Fatal("fixture precondition: the over-bound label was not spliced in, so this test measures the unmodified document")
	}
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("a 600-byte column label and bind must still LOAD — decodeColumn caps neither: %v", err)
	}
	projection, err := canvas(tpl)
	if err != nil {
		t.Fatalf("a 600-byte column label aborted the canvas projection, which terminates the worker for a document that prints: %v", err)
	}
	columns := canvasTableColumnsOf(t, projection, "e2")
	// THE COLUMN IS STILL THERE, AND THAT IS HALF THE RULING. Dropping it would
	// put the chip's "3 columns" above two drawn headers — systematically wrong
	// about STRUCTURE, which is the harm AD-17 exists to prevent.
	if len(columns) != 3 {
		t.Fatalf("the over-bound column was dropped: want the table's three declared columns, got %d — %#v", len(columns), columns)
	}
	if columns[0].ID != "e3" {
		t.Fatalf("the over-bound column is no longer first: %#v", columns)
	}
	if got := len(columns[0].Label); got != maxCanvasPropertyString {
		t.Errorf("the 600-byte label projected %d bytes, want it clipped to %d", got, maxCanvasPropertyString)
	}
	if got := len(columns[0].Bind); got != maxCanvasPropertyString {
		t.Errorf("the 600-byte bind projected %d bytes, want it clipped to %d", got, maxCanvasPropertyString)
	}
	// The two columns that were already inside the bound are untouched: a clip
	// that shortened everything would be a different defect.
	if columns[1].Label != "Amount" || columns[1].Bind != "{{row.amount}}" {
		t.Errorf("a column inside the bound was altered: %#v", columns[1])
	}
}

// TestCanvasTableColumnClipLandsOnARuneBoundary is the half an ASCII probe
// cannot see, and the spec now requires it by name.
//
// TWO UNITS DISAGREE ACROSS THIS SEAM. The projection bounds BYTES
// (`len(column.Label)`); the browser's guard bounds UTF-16 CODE UNITS
// (`column.label.length`). A Thai or CJK label is exactly where they diverge —
// 200 characters is 600 bytes and 200 UTF-16 units — so this asserts the
// clipped value is valid UTF-8, cut between runes, inside the byte bound, AND
// inside the guard's own UTF-16 bound, computed rather than argued.
func TestCanvasTableColumnClipLandsOnARuneBoundary(t *testing.T) {
	// 200 Thai characters: three bytes each, so 600 bytes and 200 UTF-16 units.
	// The bound falls at byte 512, which is INSIDE the 171st character —
	// 170*3 = 510 — so a byte cut would emit a partial rune. That the bound
	// does not land on a boundary is the precondition this test needs.
	label := strings.Repeat("ก", 200)
	if len(label) != 600 || utf8.RuneCountInString(label) != 200 {
		t.Fatalf("fixture precondition: want a 600-byte, 200-rune label, got %d bytes and %d runes", len(label), utf8.RuneCountInString(label))
	}
	if utf8.RuneStart(label[maxCanvasPropertyString]) {
		t.Fatal("fixture precondition: the bound already falls on a rune boundary in this fixture, so a byte cut and a rune cut would agree and this test could not tell them apart")
	}
	clipped := clipCanvasPropertyString(label)
	if !utf8.ValidString(clipped) {
		t.Fatalf("the clipped label is not valid UTF-8 (%d bytes) — encoding/json would mangle or reject it, turning a display concern into the fatal one this clipping removes", len(clipped))
	}
	if len(clipped) > maxCanvasPropertyString {
		t.Fatalf("the clipped label is %d bytes, over the projection bound of %d", len(clipped), maxCanvasPropertyString)
	}
	// It keeps as much as a rune boundary allows: a clip that threw away a
	// whole extra character would be a different, quieter defect.
	if want := maxCanvasPropertyString - 2; len(clipped) != want {
		t.Errorf("the clipped label is %d bytes, want %d — the largest rune-aligned prefix inside the bound", len(clipped), want)
	}
	if got := utf8.RuneCountInString(clipped); got != 170 {
		t.Errorf("the clipped label holds %d runes, want 170", got)
	}
	if !strings.HasPrefix(label, clipped) {
		t.Error("the clipped label is not a prefix of the declared one")
	}
	// AND THE BROWSER'S OWN BOUND, IN THE BROWSER'S OWN UNIT. isCanvas checks
	// `column.label.length <= MAX_CANVAS_PROPERTY_STRING`, which is UTF-16 code
	// units. Computing it here is what makes "the guard accepts the clipped
	// value" a measurement rather than an assumption.
	if units := len(utf16.Encode([]rune(clipped))); units > maxCanvasPropertyString {
		t.Errorf("the clipped label is %d UTF-16 code units, over the browser guard's bound of %d", units, maxCanvasPropertyString)
	}
	// The same value, all the way through a real document, so the clip is
	// proved where it actually runs and not only in its helper.
	source := strings.Replace(canvasSplitAlignTableTemplateJSON, `"label": "Date"`, `"label": "`+label+`"`, 1)
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("a 200-character Thai column label must still LOAD: %v", err)
	}
	projection, err := canvas(tpl)
	if err != nil {
		t.Fatalf("a 200-character Thai column label aborted the canvas projection: %v", err)
	}
	columns := canvasTableColumnsOf(t, projection, "e2")
	if len(columns) != 3 || columns[0].Label != clipped {
		t.Fatalf("the projected label is not the rune-aligned clip: %d column(s), first label %d bytes", len(columns), len(columns[0].Label))
	}
	if !utf8.ValidString(string(mustMarshal(t, projection))) {
		t.Error("the marshalled projection is not valid UTF-8")
	}
}

// TestClipCanvasPropertyStringLeavesEveryValueInsideTheBoundAlone is the other
// direction: clipping must be a no-op for every string that fits, or it would
// silently shorten the ordinary label.
func TestClipCanvasPropertyStringLeavesEveryValueInsideTheBoundAlone(t *testing.T) {
	for _, value := range []string{"", "Date", "{{row.amount}}", strings.Repeat("x", maxCanvasPropertyString), strings.Repeat("ก", maxCanvasPropertyString/3)} {
		if got := clipCanvasPropertyString(value); got != value {
			t.Errorf("clipCanvasPropertyString shortened a %d-byte value to %d bytes", len(value), len(got))
		}
	}
	// And exactly one byte over is clipped, so the bound is where it says.
	if got := clipCanvasPropertyString(strings.Repeat("x", maxCanvasPropertyString+1)); len(got) != maxCanvasPropertyString {
		t.Errorf("a %d-byte value clipped to %d bytes", maxCanvasPropertyString+1, len(got))
	}
}

// TestColumnIdsAreUniqueDocumentWide is the PRECONDITION the browser guard's
// column dedupe rests on, checked rather than assumed (step-04 finding P6).
//
// The painter keys its header row and its representative row on the column id,
// so two columns sharing one would produce duplicate React keys. The guard
// dedupes them — matching the component-level `ids` Set it already keeps — and
// that clause is only safe to add because it can never newly refuse a document
// that ships: `parse.go`'s claimID is the one door into an element id, it
// refuses a duplicate outright, and parse_bands.go routes `columns[].id`
// through it exactly as it routes an element's own id.
func TestColumnIdsAreUniqueDocumentWide(t *testing.T) {
	for _, probe := range []struct {
		name   string
		source string
	}{
		{"two columns in one table", strings.Replace(canvasSplitAlignTableTemplateJSON, `{"id": "e4", "label": "Amount"`, `{"id": "e3", "label": "Amount"`, 1)},
		{"a column and the element that owns it", strings.Replace(canvasSplitAlignTableTemplateJSON, `{"id": "e3", "label": "Date"`, `{"id": "e2", "label": "Date"`, 1)},
	} {
		t.Run(probe.name, func(t *testing.T) {
			if probe.source == canvasSplitAlignTableTemplateJSON {
				t.Fatal("fixture precondition: the duplicate id was not spliced in, so this probe measures the unmodified document")
			}
			if _, err := ParseTemplate([]byte(probe.source)); err == nil {
				t.Fatal("a duplicate column id LOADED; the browser guard's column dedupe would then be refusing a document that ships, which is the one thing it must never do")
			} else if !strings.Contains(err.Error(), "duplicate id") {
				t.Fatalf("a duplicate column id was refused for the wrong reason: %v", err)
			}
		})
	}
	// The positive control: the same document without the splice loads, so the
	// two refusals above are the duplicate's doing and not the fixture's.
	if _, err := ParseTemplate([]byte(canvasSplitAlignTableTemplateJSON)); err != nil {
		t.Fatalf("the unmodified fixture no longer loads: %v", err)
	}
}
