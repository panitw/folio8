package folio8

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/barcode"
	"github.com/panitw/folio8/folio-go/internal/bind"
	"github.com/panitw/folio8/folio-go/internal/designer"
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
)

// barcodeTestTemplate is a one-barcode document: e1 at (20, 12) in the
// content band, with the given value, box and extra element keys.
func barcodeTestTemplate(t *testing.T, value, width, height, extra string) string {
	t.Helper()
	quoted, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "barcode", "x": 20, "y": 12, "width": ` + width + `, "height": ` + height + `, ` + extra + `"value": ` + string(quoted) + `}
    ]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "4.0"
}
`
}

// renderBarcodeWitness renders a barcode document successfully, for the
// diagnostic census's Warning witnesses.
func renderBarcodeWitness(t *testing.T, value, width, data string) Result {
	t.Helper()
	tpl, err := ParseTemplate([]byte(barcodeTestTemplate(t, value, width, "50", "")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := Render(tpl, Data(data), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return result
}

func barcodePages(t *testing.T, tpl *Template, data string) ([]pagemodel.Page, []Diagnostic) {
	t.Helper()
	d, err := bind.DecodeData([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	p, err := decodeParams(nil)
	if err != nil {
		t.Fatal(err)
	}
	pages, _, _, diags, err := buildPageModel(tpl, d, p, testShippedFontSet())
	if err != nil {
		t.Fatalf("buildPageModel: %v", err)
	}
	return pages, diags
}

// runsFromRects reads a symbol's module runs back out of the page model:
// bar widths and the gaps between bars, each divided by the shared module
// width. Every rect must be black, full height and a whole number of modules.
func runsFromRects(t *testing.T, rects []pagemodel.Rect, height geom.Length) ([]int, geom.Length) {
	t.Helper()
	if len(rects) == 0 {
		t.Fatal("no bars drawn")
	}
	module := rects[0].W
	for _, r := range rects {
		if r.W < module {
			module = r.W
		}
	}
	var runs []int
	for i, r := range rects {
		if !r.HasFill || r.Fill != (pagemodel.Color{}) || r.HasStroke {
			t.Fatalf("bar %d is not a black fill: %+v", i, r)
		}
		if r.H != height {
			t.Fatalf("bar %d is %d mp tall, want the box height %d", i, r.H, height)
		}
		if r.W%module != 0 {
			t.Fatalf("bar %d width %d is not a whole number of %d mp modules", i, r.W, module)
		}
		if i > 0 {
			prev := rects[i-1]
			gap := r.X - (prev.X + prev.W)
			if gap <= 0 || gap%module != 0 {
				t.Fatalf("gap before bar %d is %d mp, not a whole number of modules", i, gap)
			}
			runs = append(runs, int(gap/module))
		}
		runs = append(runs, int(r.W/module))
	}
	return runs, module
}

func TestBarcodeThaiBillPaymentRendersItsResolvedString(t *testing.T) {
	const value = "|0994000123456{{suffix}}\r{{ref1}}\r{{ref2}}\r{{amount}}"
	const data = `{"suffix":"01","ref1":"1234567890","ref2":"INV2026","amount":"150000"}`
	const resolved = "|099400012345601\r1234567890\rINV2026\r150000"
	tpl, err := ParseTemplate([]byte(barcodeTestTemplate(t, value, "300", "50", "")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	pages, diags := barcodePages(t, tpl, data)
	if len(diags) != 0 {
		t.Fatalf("diagnostics = %+v, want none", diags)
	}
	if len(pages) != 1 {
		t.Fatalf("%d pages", len(pages))
	}
	runs, module := runsFromRects(t, pages[0].Rects, 50000)
	values, err := barcode.Encode(resolved)
	if err != nil {
		t.Fatal(err)
	}
	want := barcode.Runs(values)
	if len(runs) != len(want) {
		t.Fatalf("drawn %d runs, want %d", len(runs), len(want))
	}
	for i := range want {
		if runs[i] != want[i] {
			t.Fatalf("run %d is %d modules, want %d — the bars do not encode %q", i, runs[i], want[i], resolved)
		}
	}
	// Module width is the largest whole millipoint that fits with quiet zones.
	total := geom.Length(barcode.Modules(want) + 2*barcode.QuietZoneModules)
	if module != geom.Length(300000)/total {
		t.Errorf("module width %d, want %d", module, geom.Length(300000)/total)
	}
	rects := pages[0].Rects
	left := rects[0].X - 20000
	right := 20000 + 300000 - (rects[len(rects)-1].X + rects[len(rects)-1].W)
	if left < 10*module || right < 10*module {
		t.Errorf("quiet zones %d / %d mp are below 10 modules (%d mp)", left, right, 10*module)
	}
	if d := left - right; d < -1 || d > 1 {
		t.Errorf("symbol is not centred: left %d, right %d", left, right)
	}

	result, err := Render(tpl, Data(data), nil, testShippedFontSet())
	if err != nil || len(result.Bytes) == 0 || len(result.Diagnostics) != 0 {
		t.Fatalf("Render: %v, %d bytes, %+v", err, len(result.Bytes), result.Diagnostics)
	}
}

func TestBarcodeEmptyOrNullDrawsNothing(t *testing.T) {
	for _, c := range []struct{ value, data string }{
		{"{{ref}}", `{"ref":null}`},
		{"", `{}`},
	} {
		tpl, err := ParseTemplate([]byte(barcodeTestTemplate(t, c.value, "300", "50", "")))
		if err != nil {
			t.Fatalf("%q: parse: %v", c.value, err)
		}
		pages, diags := barcodePages(t, tpl, c.data)
		if len(diags) != 0 || len(pages[0].Rects) != 0 {
			t.Errorf("%q: rects %d diags %+v, want nothing", c.value, len(pages[0].Rects), diags)
		}
	}
}

func TestBarcodeAbsentPathIsALocatedError(t *testing.T) {
	tpl, err := ParseTemplate([]byte(barcodeTestTemplate(t, "{{missing}}", "300", "50", "")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = Render(tpl, Data(`{}`), nil, testShippedFontSet())
	var re *RenderError
	if !errors.As(err, &re) || re.Diagnostic.Code != DiagCodeBindingPathAbsent || re.Diagnostic.ElementID != "e1" {
		t.Fatalf("want a located BINDING_PATH_ABSENT on e1, got %v", err)
	}
}

func TestBarcodeStaticLoadRefusals(t *testing.T) {
	for _, c := range []struct {
		name, value, code string
	}{
		{"non-ASCII static text", "ก{{ref}}", DiagCodeTemplateFieldInvalid},
		{"non-ASCII trailing text", "{{ref}}é", DiagCodeTemplateFieldInvalid},
		{"reserved page token", "{{page}}", DiagCodeTemplateFieldInvalid},
		{"invalid expression", "{{a +}}", DiagCodeExpressionInvalid},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseTemplate([]byte(barcodeTestTemplate(t, c.value, "300", "50", "")))
			var re *RenderError
			if !errors.As(err, &re) || re.Diagnostic.Code != c.code || re.Diagnostic.ElementID != "e1" {
				t.Fatalf("want a located %s on e1, got %v", c.code, err)
			}
		})
	}
}

func TestBarcodeWarnings(t *testing.T) {
	for _, c := range []struct {
		name, value, width, data, code string
		drawn                          bool
	}{
		{"unencodable data", "{{ref}}", "300", `{"ref":"ก"}`, DiagCodeBarcodeUnencodable, false},
		{"module below 0.25 mm", "1234567890", "60", `{}`, DiagCodeBarcodeModuleTooSmall, true},
		{"cannot fit", "1234567890", "0.1", `{}`, DiagCodeBarcodeDoesNotFit, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			tpl, err := ParseTemplate([]byte(barcodeTestTemplate(t, c.value, c.width, "50", "")))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			pages, diags := barcodePages(t, tpl, c.data)
			if len(diags) != 1 || diags[0].Code != c.code || diags[0].Severity != SeverityWarning || diags[0].ElementID != "e1" {
				t.Fatalf("diagnostics = %+v, want one located %s Warning", diags, c.code)
			}
			if drawn := len(pages[0].Rects) > 0; drawn != c.drawn {
				t.Errorf("drawn = %v, want %v", drawn, c.drawn)
			}
			if _, err := Render(tpl, Data(c.data), nil, testShippedFontSet()); err != nil {
				t.Errorf("a data Warning must not stop the render: %v", err)
			}
		})
	}
}

func TestHiddenBarcodeDrawsAndWarnsNothing(t *testing.T) {
	tpl, err := ParseTemplate([]byte(barcodeTestTemplate(t, "{{ref}}", "300", "50", `"visibleIf": "show", `)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	pages, diags := barcodePages(t, tpl, `{"show":false,"ref":"ก"}`)
	if len(diags) != 0 || len(pages[0].Rects) != 0 {
		t.Fatalf("a hidden barcode drew %d rects with %+v", len(pages[0].Rects), diags)
	}
}

func TestBarcodeEscapesRoundTrip(t *testing.T) {
	for _, c := range []struct{ stored, shown string }{
		{"|0994000123456{{suffix}}\r{{ref1}}", "|0994000123456{{suffix}}\n{{ref1}}"},
		{"a\\b\nc", `a\\b\nc`},
		{`{{"x\y"}}` + "\r", `{{"x\y"}}` + "\n"},
	} {
		if got := encodeBarcodeEscapes(c.stored); got != c.shown {
			t.Errorf("encode(%q) = %q, want %q", c.stored, got, c.shown)
		}
		back, err := decodeBarcodeEscapes(c.shown)
		if err != nil || back != c.stored {
			t.Errorf("decode(%q) = %q, %v; want %q", c.shown, back, err, c.stored)
		}
	}
	// A typed `\r` and a CRLF pair each still decode to ONE carriage return.
	for _, shown := range []string{`a\rb`, "a\r\nb"} {
		if back, err := decodeBarcodeEscapes(shown); err != nil || back != "a\rb" {
			t.Errorf("decode(%q) = %q, %v; want %q", shown, back, err, "a\rb")
		}
	}
	for _, bad := range []string{`a\tb`, `trailing\`} {
		if _, err := decodeBarcodeEscapes(bad); err == nil {
			t.Errorf("decode(%q) must be refused", bad)
		}
	}
}

func findCanvasComponent(t *testing.T, projection designer.CanvasProjection, id string) designer.CanvasComponent {
	t.Helper()
	for _, c := range projection.Components {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("component %s is not projected", id)
	return designer.CanvasComponent{}
}

func TestBarcodeCommandsAndCanvas(t *testing.T) {
	tpl, err := ParseTemplate([]byte(barcodeTestTemplate(t, "1", "300", "50", "")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"dropComponent","version":1,"type":"barcode","x":100,"y":100,"snap":false}`))
	if err != nil {
		t.Fatalf("drop: %v", err)
	}
	dropped := projection.Components[len(projection.Components)-1]
	if dropped.Type != "barcode" || dropped.Width != 216000 || dropped.Height != 48000 || dropped.Value == nil || *dropped.Value != barcodeStarterValue {
		t.Fatalf("dropped barcode = %+v", dropped)
	}

	// The designer sends escapes; the document stores characters.
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"expression":{"op":"set","value":"|0994000123456{{suffix}}\\r{{ref1}}"}}}`)); err != nil {
		t.Fatalf("set expression: %v", err)
	}
	saved, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), `"value": "|0994000123456{{suffix}}\r{{ref1}}"`) {
		t.Fatalf("the carriage return must be stored as a JSON escape:\n%s", saved)
	}
	if !strings.Contains(string(saved), `"version": "4.0"`) {
		t.Fatalf("a barcode document must save as 4.0:\n%s", saved)
	}
	canvas, err := canvasWithTextPaint(tpl, testShippedFontSet())
	if err != nil {
		t.Fatalf("canvas: %v", err)
	}
	e1 := findCanvasComponent(t, canvas, "e1")
	if e1.Value == nil || *e1.Value != "|0994000123456{{suffix}}\n{{ref1}}" {
		t.Fatalf("the canvas must show the escaped value, got %v", e1.Value)
	}

	// A bound value draws illustrative bars: each placeholder is 0123456789,
	// which is exactly a render whose data supplies that string.
	if e1.Barcode == nil {
		t.Fatalf("no canvas paint for a bound barcode: unavailable=%v", e1.BarcodeUnavailable)
	}
	pages, _ := barcodePages(t, tpl, `{"suffix":"0123456789","ref1":"0123456789"}`)
	var rects []pagemodel.Rect
	for _, r := range pages[0].Rects {
		if r.X >= 20000 && r.X < 320000 && r.H == 50000 {
			rects = append(rects, r)
		}
	}
	if len(rects) != len(e1.Barcode.Bars) {
		t.Fatalf("canvas has %d bars, the render %d", len(e1.Barcode.Bars), len(rects))
	}
	for i, bar := range e1.Barcode.Bars {
		if bar.X+20000 != int64(rects[i].X) || bar.Width != int64(rects[i].W) {
			t.Fatalf("canvas bar %d = %+v, render rect = %+v", i, bar, rects[i])
		}
	}

	// Refusals: a bad escape, and a style property.
	for _, bad := range []string{
		`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"value":{"op":"set","value":"a\\tb"}}}`,
		`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"background":{"op":"set","value":"#ff0000"}}}`,
		`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"fontSize":{"op":"set","value":12}}}`,
	} {
		if _, err := applyComponentCommand(tpl, []byte(bad)); err == nil {
			t.Errorf("command must be refused: %s", bad)
		}
	}

	// Binding from the Data panel.
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"bindComponentScalar","version":1,"id":"e1","segments":["ref"]}`)); err != nil {
		t.Fatalf("bind: %v", err)
	}
	canvas, err = canvasWithTextPaint(tpl, testShippedFontSet())
	if err != nil {
		t.Fatalf("canvas: %v", err)
	}
	if b := findCanvasComponent(t, canvas, "e1").Binding; b == nil || *b != "ref" {
		t.Fatalf("binding = %v, want ref", b)
	}
}

func TestStaticBarcodeCanvasMatchesRender(t *testing.T) {
	tpl, err := ParseTemplate([]byte(barcodeTestTemplate(t, "A\r1234", "200", "40", "")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	canvas, err := canvasWithTextPaint(tpl, testShippedFontSet())
	if err != nil {
		t.Fatalf("canvas: %v", err)
	}
	e1 := findCanvasComponent(t, canvas, "e1")
	pages, _ := barcodePages(t, tpl, `{}`)
	if e1.Barcode == nil || len(e1.Barcode.Bars) != len(pages[0].Rects) {
		t.Fatalf("canvas paint %+v vs %d rects", e1.Barcode, len(pages[0].Rects))
	}
	for i, bar := range e1.Barcode.Bars {
		if bar.X+20000 != int64(pages[0].Rects[i].X) || bar.Width != int64(pages[0].Rects[i].W) {
			t.Fatalf("bar %d differs", i)
		}
	}
	if !canvas.ContentWindowCountIsExact {
		t.Error("a static barcode is fully knowable; the window count should stay exact")
	}
}

func TestBoundBarcodeCanvasDrawsIllustrativeBarsAndIsInexact(t *testing.T) {
	tpl, err := ParseTemplate([]byte(barcodeTestTemplate(t, "{{ref}}", "300", "50", "")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	canvas, err := canvasWithTextPaint(tpl, testShippedFontSet())
	if err != nil {
		t.Fatalf("canvas: %v", err)
	}
	e1 := findCanvasComponent(t, canvas, "e1")
	if e1.Barcode == nil || e1.BarcodeUnavailable != nil {
		t.Fatalf("a bound barcode must draw illustrative bars, got paint %v unavailable %v", e1.Barcode, e1.BarcodeUnavailable)
	}
	want, _ := layoutBarcode("e1", illustrativeBarcodePlaceholder, 300000, 50000)
	if len(e1.Barcode.Bars) != len(want.bars) {
		t.Fatalf("canvas has %d bars, the illustrative content encodes %d", len(e1.Barcode.Bars), len(want.bars))
	}
	if canvas.ContentWindowCountIsExact {
		t.Error("a bound content-band barcode must mark the window count inexact")
	}
}

func TestUnfitStaticBarcodeProjectsDoesNotFitAndNoColumnItem(t *testing.T) {
	tpl, err := ParseTemplate([]byte(barcodeTestTemplate(t, "1234567890", "0.1", "50", "")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	canvas, err := canvasWithTextPaint(tpl, testShippedFontSet())
	if err != nil {
		t.Fatalf("canvas: %v", err)
	}
	e1 := findCanvasComponent(t, canvas, "e1")
	if e1.Barcode != nil || e1.BarcodeUnavailable == nil || *e1.BarcodeUnavailable != barcodeUnavailableDoesNotFit {
		t.Fatalf("paint %v unavailable %v, want no paint and doesNotFit", e1.Barcode, e1.BarcodeUnavailable)
	}
	if canvasBarcodeIsPlaced(tpl.doc.Bands.Content.Elements[0]) {
		t.Error("a barcode that draws nothing must add no column item")
	}
	if !canvas.ContentWindowCountIsExact || canvas.ContentWindowCount != 1 {
		t.Errorf("window count %d exact %v, want 1 exact", canvas.ContentWindowCount, canvas.ContentWindowCountIsExact)
	}
}

func TestPageHeaderBarcodeRendersOnEveryPage(t *testing.T) {
	const doc = `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e2", "type": "rect", "x": 0, "y": 0, "width": 100, "height": 20, "style": {"background": "#eeeeee"}},
      {"id": "e3", "type": "rect", "x": 0, "y": 750, "width": 100, "height": 20, "style": {"background": "#eeeeee"}}
    ]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [
      {"id": "e1", "type": "barcode", "x": 0, "y": 0, "width": 300, "height": 20, "value": "1234"}
    ], "height": 30}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "4.0"
}
`
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	pages, diags := barcodePages(t, tpl, `{}`)
	if len(diags) != 0 || len(pages) < 2 {
		t.Fatalf("pages %d diags %+v, want a multi-page render with no diagnostics", len(pages), diags)
	}
	values, err := barcode.Encode("1234")
	if err != nil {
		t.Fatal(err)
	}
	wantBars := len(barcode.Runs(values))/2 + 1
	for i, page := range pages {
		bars := 0
		for _, r := range page.Rects {
			if r.HasFill && r.Fill == (pagemodel.Color{}) && r.H == 20000 {
				bars++
			}
		}
		if bars != wantBars {
			t.Errorf("page %d carries %d barcode bars, want %d", i+1, bars, wantBars)
		}
	}
}
