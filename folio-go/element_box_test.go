// Story 9.1's own suite: an element's style.background and style.border
// reach the page model. Before this story they reached nothing outside a
// table's cell chrome — see element_box.go's doc comment for the trace.
package folio8

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/designer"
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/layout"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// boxTemplateJSON builds a three-band document whose content element e1
// carries styleFields verbatim (a style block's inner fields, e.g.
// `"background": "#112233", `). e2 is an unstyled sibling, so every
// assertion about "the boxes" is also an assertion that an element
// declaring no box contributes none.
func boxTemplateJSON(styleFields string) string {
	return `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 12, "y": 20, "width": 200, "height": 30, "value": "Boxed", "style": {` + styleFields + `"fontFamily": "body", "fontSize": 12}},
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

// boxPages renders doc through the SAME function Render() calls
// internally, so what these tests read is the page model that becomes
// the PDF, not a parallel derivation of it.
func boxPages(t *testing.T, doc string) []pagemodel.Page {
	t.Helper()
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	pages, _, _, _, err := buildPageModel(tpl, mustDecodeData(t, `{"customer":{"flag":false}}`), mustDecodeParams(t), testFontSet())
	if err != nil {
		t.Fatalf("buildPageModel: %v", err)
	}
	return pages
}

// contentBandOrigin re-derives where the content band starts from the
// template's own geometry, independently of the collector under test.
func contentBandOrigin(t *testing.T, doc string) geom.Length {
	t.Helper()
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	g, err := pageGeometryOf(tpl)
	if err != nil {
		t.Fatalf("pageGeometryOf: %v", err)
	}
	return layout.Origins(g).Content
}

func TestElementBackgroundFillsTheDeclaredBox(t *testing.T) {
	doc := boxTemplateJSON(`"background": "#112233", `)
	pages := boxPages(t, doc)
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(pages))
	}
	if len(pages[0].Rects) != 1 {
		t.Fatalf("page rects = %d, want exactly 1 (e1's box; e2 declares none)", len(pages[0].Rects))
	}
	rect := pages[0].Rects[0]
	want := pagemodel.Rect{
		X: 12000, Y: contentBandOrigin(t, doc) + 20000, W: 200000, H: 30000,
		HasFill: true, Fill: pagemodel.Color{R: 0x11, G: 0x22, B: 0x33},
	}
	if rect != want {
		t.Errorf("e1's box = %+v, want %+v — the box is the element's DECLARED rectangle, placed in its band", rect, want)
	}
}

func TestElementBorderStrokesWithTheTableCellDefaults(t *testing.T) {
	// No width, no colour, no edges: the defaults a table cell already
	// resolves through this same builder — 0.5pt, #000000, all four edges.
	rect := boxPages(t, boxTemplateJSON(`"border": {}, `))[0].Rects[0]
	if rect.HasFill {
		t.Errorf("a border-only box has a fill (%+v) — a declaration the element never made", rect.Fill)
	}
	if !rect.HasStroke || rect.Stroke != (pagemodel.Color{}) || rect.StrokeWidth != 500 {
		t.Errorf("border defaults = {stroke:%v color:%+v width:%d}, want {true #000000 500}", rect.HasStroke, rect.Stroke, rect.StrokeWidth)
	}
	if (rect.Edges != pagemodel.RectEdges{Top: true, Right: true, Bottom: true, Left: true}) {
		t.Errorf("border edges = %+v, want all four", rect.Edges)
	}

	declared := boxPages(t, boxTemplateJSON(`"border": {"color": "#c81e1e", "width": 2, "edges": ["bottom"]}, `))[0].Rects[0]
	if !declared.HasStroke || declared.Stroke != (pagemodel.Color{R: 0xc8, G: 0x1e, B: 0x1e}) || declared.StrokeWidth != 2000 {
		t.Errorf("declared border = {stroke:%v color:%+v width:%d}, want {true #c81e1e 2000}", declared.HasStroke, declared.Stroke, declared.StrokeWidth)
	}
	if (declared.Edges != pagemodel.RectEdges{Bottom: true}) {
		t.Errorf("declared edges = %+v, want bottom alone", declared.Edges)
	}
}

func TestElementBoxCarriesBothFillAndStrokeAtOnce(t *testing.T) {
	rect := boxPages(t, boxTemplateJSON(`"background": "#0b1120", "border": {"color": "#ffffff"}, `))[0].Rects[0]
	if !rect.HasFill || !rect.HasStroke {
		t.Fatalf("box = {fill:%v stroke:%v}, want both — one rectangle carries both declarations", rect.HasFill, rect.HasStroke)
	}
}

func TestHiddenElementDrawsNoBox(t *testing.T) {
	doc := `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 30, "value": "Hidden", "visibleIf": "customer.flag", "style": {"background": "#112233", "fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	// mustDecodeData supplies customer.flag = false, so e1 is hidden.
	if rects := boxPages(t, doc)[0].Rects; len(rects) != 0 {
		t.Errorf("a hidden element contributed %d rect(s): %+v — AD-24 makes it absent from the page model, box included", len(rects), rects)
	}
}

// parse_bands.go makes width and height required on every non-table
// element, so "no rectangle" reaches the renderer only as a zero or
// negative one — which the loader accepts.
func TestBoxWithoutADeclaredRectangleDrawsNothing(t *testing.T) {
	doc := `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 0, "value": "Unsized", "style": {"background": "#112233", "fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	if rects := boxPages(t, doc)[0].Rects; len(rects) != 0 {
		t.Errorf("an element whose declared rectangle has no area drew %d rect(s) — there is no rectangle to draw", len(rects))
	}
}

func TestRectAndLineElementsDrawTheirBox(t *testing.T) {
	doc := `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "rect", "x": 0, "y": 0, "width": 120, "height": 40, "style": {"background": "#1b2a4a", "border": {"color": "#ffffff", "width": 1}}},
        {"id": "e2", "type": "line", "x": 0, "y": 60, "width": 300, "height": 1, "style": {"background": "#000000"}}
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
	rects := boxPages(t, doc)[0].Rects
	if len(rects) != 2 {
		t.Fatalf("rect+line drew %d rect(s), want 2 — before Story 9.2 neither kind reached the page model at all", len(rects))
	}
	if !rects[0].HasFill || !rects[0].HasStroke {
		t.Errorf("the rect element = {fill:%v stroke:%v}, want both", rects[0].HasFill, rects[0].HasStroke)
	}
	if !rects[1].HasFill || rects[1].H != 1000 || rects[1].W != 300000 {
		t.Errorf("the line element = {fill:%v w:%d h:%d}, want a filled bar 300000 x 1000 — its declared height IS its thickness", rects[1].HasFill, rects[1].W, rects[1].H)
	}
}

func TestHeaderAndFooterBoxesRepeatOnEveryPage(t *testing.T) {
	// Two content elements far enough apart to force a second page, so the
	// repetition is observed rather than assumed.
	doc := `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e3", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "First", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e4", "type": "text", "x": 0, "y": 900, "width": 200, "height": 20, "value": "Second", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {"elements": [{"id": "e2", "type": "rect", "x": 0, "y": 4, "width": 400, "height": 2, "style": {"background": "#888888"}}], "height": 20},
    "pageHeader": {"elements": [{"id": "e1", "type": "rect", "x": 0, "y": 4, "width": 400, "height": 2, "style": {"background": "#444444"}}], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 5,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	pages := boxPages(t, doc)
	if len(pages) < 2 {
		t.Fatalf("pages = %d, want at least 2 — the repetition is untested on a one-page document", len(pages))
	}
	for i, page := range pages {
		if len(page.Rects) != 2 {
			t.Errorf("page %d carries %d rect(s), want the page-header box and the page-footer box on every page", i+1, len(page.Rects))
			continue
		}
		if page.Rects[0].Fill != (pagemodel.Color{R: 0x44, G: 0x44, B: 0x44}) || page.Rects[1].Fill != (pagemodel.Color{R: 0x88, G: 0x88, B: 0x88}) {
			t.Errorf("page %d fills = %+v, %+v, want the header's then the footer's", i+1, page.Rects[0].Fill, page.Rects[1].Fill)
		}
	}
}

// TestATableStyleIsNeverPaintedTwice keeps its name and reverses its
// arithmetic, which is the honest record of what SPEC-table-rules did.
//
// Story 9.1 asserted "a table's style paints as cell chrome ONLY, never
// a second time as an element box". SPEC-table-rules moved the
// declaration to the other side of that sentence: a table's
// `style.background` paints the table's own BOX only, never a cell. The
// invariant the test actually guards — ONCE, never twice — is unchanged,
// and it is still what fails if the two painters are ever both wired up.
func TestATableStyleIsNeverPaintedTwice(t *testing.T) {
	doc := `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "rows[]", "as": "row", "headerHeight": 20,
          "style": {"background": "#112233", "fontFamily": "body", "fontSize": 10},
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
	// The table's own frame fill, one header cell and one body cell. The
	// frame is drawn per page slice after pagination, and its FILL goes
	// before the table's first rect so row and header fills paint over it. The two
	// CELLS are built unconditionally — they carry the rows' pagination
	// identity and D-2.6.5 forbids an empty item that occupies space —
	// but neither may PAINT, and exactly one rect may.
	if got := len(pages[0].Rects); got != 3 {
		t.Fatalf("a styled table produced %d rect(s), want 3 (header cell, body cell, the table's own box)", got)
	}
	filled := 0
	for i, r := range pages[0].Rects {
		if r.HasFill {
			filled++
			if i != 0 {
				t.Errorf("rect %d carries the fill; only the FIRST rect — the table's own frame fill — may (SPEC-table-rules §1: style.background reaches no cell)", i)
			}
			if r.Fill != (pagemodel.Color{R: 0x11, G: 0x22, B: 0x33}) {
				t.Errorf("the box's fill = %+v, want #112233", r.Fill)
			}
		}
	}
	if filled != 1 {
		t.Errorf("a table declaring one style.background painted %d filled rect(s), want exactly 1 — once, never twice", filled)
	}
}

func TestAnElementBoxPaintsUnderItsOwnText(t *testing.T) {
	// Page assembly draws every rect before every run, so a background can
	// never cover the text it sits behind. Asserted on the page model
	// rather than on the PDF bytes: pagemodel.Page is where the order is
	// decided (AD-5).
	page := boxPages(t, boxTemplateJSON(`"background": "#112233", `))[0]
	if len(page.Rects) == 0 || len(page.Runs) == 0 {
		t.Fatalf("page has rects=%d runs=%d, want both", len(page.Rects), len(page.Runs))
	}
}

func TestAMalformedElementColourIsALocatedLoadError(t *testing.T) {
	requireColourLoadError(t, boxTemplateJSON(`"background": "red", `), "e1", "style.background")
}

// Story 9.2: a placed line and a placed rect are visible without the
// author styling them first. Both assertions read the PROJECTION — the
// same values the canvas paints from and the inspector shows — so a
// default that existed only in the document, or only on the canvas,
// fails here. Millipoints throughout: AD-23 forbids a float anywhere
// under the module, tests included.
func TestAPlacedLineAndRectArriveVisible(t *testing.T) {
	for _, tc := range []struct {
		kind       string
		wantFill   string
		wantEdge   string
		wantWidth  int64
		wantHeight int64
	}{
		{kind: "line", wantFill: "#000000", wantHeight: 1000},
		{kind: "rect", wantEdge: "#000000", wantWidth: 1000, wantHeight: 24000},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			tpl := componentTemplate(t)
			before, err := canvas(tpl)
			if err != nil {
				t.Fatal(err)
			}
			after, err := applyComponentCommand(tpl, []byte(`{"kind":"dropComponent","version":1,"type":"`+tc.kind+`","x":72,"y":180,"snap":false}`))
			if err != nil {
				t.Fatal(err)
			}
			placed := newProjectedComponent(t, before, after)
			if got := derefString(placed.Background); got != tc.wantFill {
				t.Errorf("%s background = %q, want %q", tc.kind, got, tc.wantFill)
			}
			if got := derefString(placed.BorderColor); got != tc.wantEdge {
				t.Errorf("%s border colour = %q, want %q", tc.kind, got, tc.wantEdge)
			}
			if got := derefLength(placed.BorderWidth); got != tc.wantWidth {
				t.Errorf("%s border width = %d millipoints, want %d", tc.kind, got, tc.wantWidth)
			}
			if placed.Height != tc.wantHeight {
				t.Errorf("%s dropped %d millipoints tall, want %d — a rule's declared height is its thickness", tc.kind, placed.Height, tc.wantHeight)
			}
		})
	}
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefLength(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// TestABorderDeclaringNoEdgesIsNotABox is E9-1's proof, and it is asserted
// AGAINST LITERALS rather than against the canvas.
//
// `style.border: {"edges": []}` declared a box, painted nothing and still
// cost a page: /Count went 1 → 2 with zero fills and zero strokes, because
// elementDeclaresBox read presence alone while internal/pdf/rectdoc.go's
// emitter already refused to stroke an empty edge set. The predicate now
// carries the emitter's own condition.
//
// ⚠ WHY NOT "the canvas agrees with the render". Both sides call
// borderPaints, so that assertion would hold however wrong the predicate
// was — a self-measuring instrument. The claims here are the printed page
// count, the absence of ink operators, and byte-identity with the document
// that declares no box at all.
func TestABorderDeclaringNoEdgesIsNotABox(t *testing.T) {
	empty := boxTemplateJSON(`"border": {"edges": [], "color": "#000000"}, `)
	pages := boxPages(t, empty)
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want 1 — a border that paints no ink must not cost a page", len(pages))
	}
	if len(pages[0].Rects) != 0 {
		t.Fatalf("page rects = %d, want 0 — the element declares no box the printed page has anything in", len(pages[0].Rects))
	}

	bytesOf := func(source string) []byte {
		t.Helper()
		tpl, err := ParseTemplate([]byte(source))
		if err != nil {
			t.Fatalf("ParseTemplate: %v", err)
		}
		res, err := Render(tpl, Data(`{}`), nil, testFontSet())
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		return res.Bytes
	}

	got := bytesOf(empty)
	if n := bytes.Count(got, []byte(" re f\n")); n != 0 {
		t.Errorf("rendered %d fill operators, want 0", n)
	}
	if n := bytes.Count(got, []byte("\nS\n")); n != 0 {
		t.Errorf("rendered %d stroke operators, want 0", n)
	}
	if n := bytes.Count(got, []byte("/Count 1")); n != 1 {
		t.Errorf("/Count 1 appears %d times, want exactly 1 — the document is one page", n)
	}
	if n := bytes.Count(got, []byte("/Count 2")); n != 0 {
		t.Errorf("/Count 2 appears %d times, want 0", n)
	}

	// The document with no box declaration at all. Byte-identity is the
	// whole claim: an edges-empty border is not merely cheap, it is inert.
	if want := bytesOf(boxTemplateJSON("")); !bytes.Equal(got, want) {
		t.Errorf("an edges-empty border changed the rendered bytes (%d vs %d) — it must be byte-identical to the same document declaring no box", len(got), len(want))
	}
}

// TestAnEmptyBorderObjectStillPaintsAllFourEdges is E9-1's guardrail in the
// other direction: `"border": {}` means ABSENT edges, which
// buildCellRectWithBackgroundField defaults to all four, so it is one
// stroke group of real ink and must keep costing what it costs. Only an
// edges array that is present, non-null and empty is inert.
func TestAnEmptyBorderObjectStillPaintsAllFourEdges(t *testing.T) {
	pages := boxPages(t, boxTemplateJSON(`"border": {}, `))
	if len(pages[0].Rects) != 1 {
		t.Fatalf("page rects = %d, want 1 — an empty border OBJECT declares all four edges", len(pages[0].Rects))
	}
	rect := pages[0].Rects[0]
	if !rect.HasStroke || (rect.Edges != pagemodel.RectEdges{Top: true, Right: true, Bottom: true, Left: true}) {
		t.Errorf("border:{} = {stroke:%v edges:%+v}, want a stroke on all four edges", rect.HasStroke, rect.Edges)
	}
}

// TestANegativeBorderWidthIsRefusedAtLoad is E9-2's proof. `-5 w` is not a
// valid PDF line width (ISO 32000-1 §8.4.3.2), and this document RENDERED
// before the repair, emitting exactly that operator.
//
// The refusal is at LOAD because border.width flows into
// buildCellRectWithBackgroundField, which table cell chrome has shared
// since Epic 4 — one authority closes both paths — and it is layering-legal
// because border.width is a geom.Length and internal/template already
// imports internal/geom.
//
// ZERO IS NOT REFUSED: it is the thinnest line PDF can draw, and the second
// half of this test is the guardrail that keeps the check from widening
// into a general geometry audit.
func TestANegativeBorderWidthIsRefusedAtLoad(t *testing.T) {
	_, err := ParseTemplate([]byte(boxTemplateJSON(`"border": {"width": -5}, `)))
	if err == nil {
		t.Fatal("a negative style.border.width loaded — it would render `-5 w`, which is not a valid PDF line width")
	}
	var renderErr *RenderError
	if !errors.As(err, &renderErr) {
		t.Fatalf("the refusal must reach the caller as a *RenderError: %T %v", err, err)
	}
	if renderErr.Diagnostic.Code != DiagCodeTemplateFieldInvalid {
		t.Errorf("Code = %q, want %q", renderErr.Diagnostic.Code, DiagCodeTemplateFieldInvalid)
	}
	if renderErr.Diagnostic.ElementID != "e1" {
		t.Errorf("ElementID = %q, want e1 — the refusal must name the element", renderErr.Diagnostic.ElementID)
	}
	if msg := err.Error(); !strings.Contains(msg, "style.border.width") {
		t.Errorf("the refusal must name the field style.border.width, got: %s", msg)
	}

	// The same check on a table's OWN cell chrome, which is the path this
	// value has been reachable through since Epic 4 rather than Epic 9.
	if _, err := ParseTemplate([]byte(negativeBorderTableTemplateJSON)); err == nil {
		t.Fatal("a negative headerStyle.border.width loaded — the table cell chrome path is the one this has been reachable through since Epic 4")
	}

	// Zero stays valid: the thinnest line, not an absent border.
	rect := boxPages(t, boxTemplateJSON(`"border": {"width": 0}, `))[0].Rects[0]
	if !rect.HasStroke || rect.StrokeWidth != 0 {
		t.Errorf("border width 0 = {stroke:%v width:%d}, want {true 0} — zero is a valid PDF line width", rect.HasStroke, rect.StrokeWidth)
	}
}

const negativeBorderTableTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "t1", "type": "table", "x": 0, "y": 0, "width": 300, "height": 100,
         "table": {"columns": [{"id": "c1", "label": "A", "width": 100}],
                   "headerStyle": {"border": {"width": -2}}},
         "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// canvasComponentOf projects doc and returns the content-band component
// named id. It reads the CanvasComponent itself rather than the wire test's
// key census, because DW-74 is live: the Go/TS wire test records the
// projection's top-level and CanvasFontChain key lists and NOT
// CanvasComponent's, so a change to WHICH per-component fields are emitted
// is exactly its blind spot. A green wire test proves nothing about the two
// tests below; the fields are therefore asserted explicitly.
func canvasComponentOf(t *testing.T, doc, id string) designer.CanvasComponent {
	t.Helper()
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	projection, err := canvas(tpl)
	if err != nil {
		t.Fatalf("Canvas: %v", err)
	}
	for _, component := range projection.Components {
		if component.ID == id {
			return component
		}
	}
	t.Fatalf("component %q is absent from the projection", id)
	return designer.CanvasComponent{}
}

// TestABorderPaintingNoInkProjectsNoBorderFields is E9-5's proof.
//
// BorderEdges carries `json:",omitempty"`, which drops an EMPTY slice — so
// `style.border: {"edges": [], "color": "#ff0000"}` projected borderWidth
// and borderColor while borderEdges vanished from the wire, and App.tsx's
// `component.borderEdges ?? boxEdges` fell back to all four. The canvas
// painted a full red border for a document that prints none.
//
// ⚠ THE FIX IS NOT REMOVING omitempty. A nil slice would then marshal to
// JSON `null` on EVERY component, `null ?? boxEdges` re-fires the full
// border for all of them, and the protocol note says the validator may
// reject the null outright — wrong in both directions. What changes is
// WHICH FIELDS ARE EMITTED: a border that paints no ink projects none of
// the three, so App.tsx's `bordered` evaluates false.
func TestABorderPaintingNoInkProjectsNoBorderFields(t *testing.T) {
	inert := canvasComponentOf(t, boxTemplateJSON(`"border": {"edges": [], "color": "#ff0000"}, `), "e1")
	if inert.BorderWidth != nil {
		t.Errorf("borderWidth = %d, want absent", *inert.BorderWidth)
	}
	if inert.BorderColor != nil {
		t.Errorf("borderColor = %q, want absent", *inert.BorderColor)
	}
	if inert.BorderEdges != nil {
		t.Errorf("borderEdges = %v, want absent", inert.BorderEdges)
	}

	// The control on the same field: a border that DOES paint still projects
	// everything it declared, so "absent" above is evidence about the ink
	// and not about the projection having stopped carrying borders.
	painting := canvasComponentOf(t, boxTemplateJSON(`"border": {"edges": ["bottom"], "color": "#ff0000", "width": 2}, `), "e1")
	if painting.BorderColor == nil || *painting.BorderColor != "#ff0000" {
		t.Errorf("borderColor = %v, want #ff0000", painting.BorderColor)
	}
	if painting.BorderWidth == nil || *painting.BorderWidth != 2000 {
		t.Errorf("borderWidth = %v, want 2000", painting.BorderWidth)
	}
	if len(painting.BorderEdges) != 1 || painting.BorderEdges[0] != "bottom" {
		t.Errorf("borderEdges = %v, want [bottom]", painting.BorderEdges)
	}
}

// TestTheProjectionRefusesEveryColourRenderRefuses is E10-3's proof, and it
// covers ALL THREE colour arms.
//
// applyCanvasStyle bounded each colour string's LENGTH and never its SHAPE,
// so "red", "", "rgba(1,2,3,.5)" and "var(--x)" projected verbatim and
// reached the canvas's --text-ink while Render refused the same document:
// the designer painted what the engine refuses to print.
//
// Since colour is refused at LOAD (owner ruling, 2026-09-17) no such
// document loads, so the file door is asserted first. The projection's own
// refusal stays as defence in depth and is measured on a template whose
// colour is written past the loader, together with Render's unreachable
// guard on the same template, so "both sides agree" is still measured.
//
// It REFUSES rather than dropping the field: a silent drop trades a loud
// divergence for a quiet one.
func TestTheProjectionRefusesEveryColourRenderRefuses(t *testing.T) {
	for _, bad := range []string{"red", "", "rgba(1,2,3,.5)", "var(--x)"} {
		for _, arm := range []struct {
			name, styleFields string
			poke              func(st *template.Style)
		}{
			{"color", `"color": ` + quoteJSON(bad) + `, `, func(st *template.Style) { st.Color = template.Presence[string]{Set: true, Value: bad} }},
			{"background", `"background": ` + quoteJSON(bad) + `, `, func(st *template.Style) { st.Background = template.Presence[string]{Set: true, Value: bad} }},
			{"border.color", `"border": {"color": ` + quoteJSON(bad) + `}, `, func(st *template.Style) {
				st.Border = template.Presence[template.Border]{Set: true, Value: template.Border{Color: template.Presence[string]{Set: true, Value: bad}}}
			}},
		} {
			requireColourLoadError(t, boxTemplateJSON(arm.styleFields), "e1", "style."+arm.name)

			tpl, err := ParseTemplate([]byte(boxTemplateJSON(``)))
			if err != nil {
				t.Fatalf("ParseTemplate: %v", err)
			}
			arm.poke(&tpl.doc.Bands.Content.Elements[0].Style.Value)
			if _, err := canvas(tpl); err == nil {
				t.Errorf("%s = %q projected without error — the designer would paint what Render refuses", arm.name, bad)
			}
			if _, _, _, _, err := buildPageModel(tpl, mustDecodeData(t, `{}`), mustDecodeParams(t), testFontSet()); err == nil {
				t.Errorf("%s = %q rendered without error — Render's guard no longer refuses what the loader refuses", arm.name, bad)
			}
		}
	}

	// The control: a well-formed colour still projects on all three arms.
	ok := canvasComponentOf(t, boxTemplateJSON(`"color": "#123456", "background": "#abcdef", "border": {"color": "#000000"}, `), "e1")
	if ok.Color == nil || *ok.Color != "#123456" || ok.Background == nil || *ok.Background != "#abcdef" || ok.BorderColor == nil || *ok.BorderColor != "#000000" {
		t.Errorf("a valid colour failed to project: color=%v background=%v borderColor=%v", ok.Color, ok.Background, ok.BorderColor)
	}
}

func quoteJSON(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}
