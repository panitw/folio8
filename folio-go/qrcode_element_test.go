package folio8

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/barcode"
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
)

// qrcodeTestEMVCo is an EMVCo Merchant-Presented Mode shaped payload (tag 30
// bill payment, CRC tag 63). Its CRC is illustrative: the element encodes the
// finished string and never checks it.
const qrcodeTestEMVCo = "00020101021230810016A00000067701011201150994000123456780214INV2026000001030900000000153037645406150.005802TH62100706INV01263049A3F"

// qrcodeTestTemplate is a one-qrcode document: e1 at (20, 12) in the content
// band. level "" leaves errorCorrection absent.
func qrcodeTestTemplate(t *testing.T, value, level, width, height, extra string) string {
	t.Helper()
	quoted, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if level != "" {
		extra += `"errorCorrection": "` + level + `", `
	}
	return `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "qrcode", "x": 20, "y": 12, "width": ` + width + `, "height": ` + height + `, ` + extra + `"value": ` + string(quoted) + `}
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

// renderQRCodeWitness renders a square-box qrcode document successfully, for
// the diagnostic census's Warning witnesses.
func renderQRCodeWitness(t *testing.T, value, level, side, data string) Result {
	t.Helper()
	tpl, err := ParseTemplate([]byte(qrcodeTestTemplate(t, value, level, side, side, "")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := Render(tpl, Data(data), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return result
}

// assertRectsEncode checks that rects — one qrcode's page-model rectangles,
// in emission order — are black, lie on one whole-millipoint module grid of
// the largest fitting size, and cover exactly the dark modules of want's
// encoding. It returns the module width and the symbol's top-left corner.
func assertRectsEncode(t *testing.T, rects []pagemodel.Rect, want barcode.QRSymbol, boxW, boxH geom.Length) (geom.Length, geom.Length, geom.Length) {
	t.Helper()
	if len(rects) == 0 {
		t.Fatal("no modules drawn")
	}
	module := boxW
	if boxH < module {
		module = boxH
	}
	module /= geom.Length(want.Size + 2*barcode.QRQuietZoneModules)
	// Module (0,0) is the top-left finder's corner, always dark, and the
	// first run emitted.
	originX, originY := rects[0].X, rects[0].Y
	covered := make([]bool, len(want.Modules))
	for i, r := range rects {
		if !r.HasFill || r.Fill != (pagemodel.Color{}) || r.HasStroke {
			t.Fatalf("rect %d is not a black fill: %+v", i, r)
		}
		if r.H != module || r.W%module != 0 || (r.X-originX)%module != 0 || (r.Y-originY)%module != 0 {
			t.Fatalf("rect %d is off the %d mp module grid: %+v", i, module, r)
		}
		y := int((r.Y - originY) / module)
		for x := int((r.X - originX) / module); x < int((r.X-originX+r.W)/module); x++ {
			if x < 0 || y < 0 || x >= want.Size || y >= want.Size || covered[y*want.Size+x] || !want.Dark(x, y) {
				t.Fatalf("rect %d covers module (%d,%d), which is not a distinct dark module of the expected symbol", i, x, y)
			}
			covered[y*want.Size+x] = true
		}
	}
	for i, dark := range want.Modules {
		if dark != covered[i] {
			t.Fatalf("module (%d,%d): dark %v, drawn %v — the rects do not encode the expected symbol", i%want.Size, i/want.Size, dark, covered[i])
		}
	}
	return module, originX, originY
}

func TestQRCodeRendersItsResolvedString(t *testing.T) {
	for _, c := range []struct {
		name, value, level, data, resolved string
	}{
		{"EMVCo payload, default level", "{{payload}}", "", `{"payload":"` + qrcodeTestEMVCo + `"}`, qrcodeTestEMVCo},
		{"Thai UTF-8 at Q", "ชำระเงิน {{ref}}", "Q", `{"ref":"INV-0001"}`, "ชำระเงิน INV-0001"},
	} {
		t.Run(c.name, func(t *testing.T) {
			tpl, err := ParseTemplate([]byte(qrcodeTestTemplate(t, c.value, c.level, "120", "120", "")))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			pages, diags := barcodePages(t, tpl, c.data)
			if len(diags) != 0 || len(pages) != 1 {
				t.Fatalf("pages %d diags %+v", len(pages), diags)
			}
			level := barcode.ECLevelM
			if c.level != "" {
				level, _ = barcode.ParseECLevel(c.level)
			}
			want, err := barcode.EncodeQR([]byte(c.resolved), level)
			if err != nil {
				t.Fatal(err)
			}
			module, originX, _ := assertRectsEncode(t, pages[0].Rects, want, 120000, 120000)
			if left := originX - 20000; left < 4*module || left != geom.ScaleRound(120000-module*geom.Length(want.Size), 1, 2) {
				t.Errorf("symbol is not centred with a 4-module quiet zone: left %d, module %d", left, module)
			}
		})
	}
}

func TestQRCodeLevelsAreHonoured(t *testing.T) {
	prev := 0
	for _, level := range []string{"L", "M", "Q", "H"} {
		tpl, err := ParseTemplate([]byte(qrcodeTestTemplate(t, qrcodeTestEMVCo, level, "200", "200", "")))
		if err != nil {
			t.Fatalf("%s: parse: %v", level, err)
		}
		pages, diags := barcodePages(t, tpl, `{}`)
		if len(diags) != 0 {
			t.Fatalf("%s: diags %+v", level, diags)
		}
		l, _ := barcode.ParseECLevel(level)
		want, err := barcode.EncodeQR([]byte(qrcodeTestEMVCo), l)
		if err != nil {
			t.Fatal(err)
		}
		assertRectsEncode(t, pages[0].Rects, want, 200000, 200000)
		if want.Version < prev {
			t.Errorf("level %s uses version %d, smaller than the lower level's %d", level, want.Version, prev)
		}
		prev = want.Version
	}
}

func TestQRCodeNonSquareBoxIsSizedToTheShortSideAndCentred(t *testing.T) {
	tpl, err := ParseTemplate([]byte(qrcodeTestTemplate(t, "folio8", "", "200", "80", "")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	pages, diags := barcodePages(t, tpl, `{}`)
	if len(diags) != 0 {
		t.Fatalf("diags %+v", diags)
	}
	want, _ := barcode.EncodeQR([]byte("folio8"), barcode.ECLevelM)
	module, originX, originY := assertRectsEncode(t, pages[0].Rects, want, 200000, 80000)
	if module != 80000/geom.Length(want.Size+8) {
		t.Fatalf("module %d, want the 80 pt side's %d", module, 80000/geom.Length(want.Size+8))
	}
	symbol := module * geom.Length(want.Size)
	if originX-20000 != geom.ScaleRound(200000-symbol, 1, 2) {
		t.Errorf("not centred horizontally: left %d", originX-20000)
	}
	canvas, err := canvasWithTextPaint(tpl, testShippedFontSet())
	if err != nil {
		t.Fatalf("canvas: %v", err)
	}
	paint := findCanvasComponent(t, canvas, "e1").QRCode
	if paint == nil || paint.Rects[0].Y != int64(geom.ScaleRound(80000-symbol, 1, 2)) || paint.Rects[0].X != int64(originX-20000) {
		t.Fatalf("not centred vertically, or the canvas disagrees: %+v", paint)
	}
	// The page model is the canvas paint translated by the element origin.
	top := originY - geom.Length(paint.Rects[0].Y)
	if len(paint.Rects) != len(pages[0].Rects) {
		t.Fatalf("canvas %d rects, render %d", len(paint.Rects), len(pages[0].Rects))
	}
	for i, r := range paint.Rects {
		pr := pages[0].Rects[i]
		if geom.Length(r.X)+20000 != pr.X || geom.Length(r.Y)+top != pr.Y || geom.Length(r.Width) != pr.W || geom.Length(r.Height) != pr.H {
			t.Fatalf("canvas rect %d %+v differs from render rect %+v", i, r, pr)
		}
	}
}

func TestQRCodeEmptyOrNullDrawsNothing(t *testing.T) {
	for _, c := range []struct{ value, data string }{
		{"{{ref}}", `{"ref":null}`},
		{"{{ref}}", `{"ref":""}`},
		{"", `{}`},
	} {
		tpl, err := ParseTemplate([]byte(qrcodeTestTemplate(t, c.value, "H", "120", "120", "")))
		if err != nil {
			t.Fatalf("%q: parse: %v", c.value, err)
		}
		pages, diags := barcodePages(t, tpl, c.data)
		if len(diags) != 0 || len(pages[0].Rects) != 0 {
			t.Errorf("%q with %s: rects %d diags %+v, want nothing", c.value, c.data, len(pages[0].Rects), diags)
		}
	}
}

func TestQRCodeLoadRefusals(t *testing.T) {
	for _, c := range []struct {
		name, value, level, code string
	}{
		{"static text over version-40 capacity", strings.Repeat("x", 1274) + "{{ref}}", "H", DiagCodeTemplateFieldInvalid},
		{"reserved page token", "{{pages}}", "", DiagCodeTemplateFieldInvalid},
		{"invalid expression", "{{a +}}", "", DiagCodeExpressionInvalid},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseTemplate([]byte(qrcodeTestTemplate(t, c.value, c.level, "120", "120", "")))
			var re *RenderError
			if !errors.As(err, &re) || re.Diagnostic.Code != c.code || re.Diagnostic.ElementID != "e1" {
				t.Fatalf("want a located %s on e1, got %v", c.code, err)
			}
		})
	}
	// Exactly at capacity still loads.
	if _, err := ParseTemplate([]byte(qrcodeTestTemplate(t, strings.Repeat("x", 1273), "H", "300", "300", ""))); err != nil {
		t.Fatalf("1273 static bytes fit version 40 at H: %v", err)
	}
	for _, bad := range []string{`"errorCorrection": "X", `, `"errorCorrection": null, `, `"style": {}, `} {
		if _, err := ParseTemplate([]byte(qrcodeTestTemplate(t, "folio8", "", "120", "120", bad))); err == nil {
			t.Errorf("%s must be refused at load", bad)
		}
	}
}

func TestQRCodeWarnings(t *testing.T) {
	for _, c := range []struct {
		name, value, level, side, data, code string
		drawn                                bool
	}{
		{"too long from data", "{{ref}}", "H", "300", `{"ref":"` + strings.Repeat("x", 1274) + `"}`, DiagCodeQRCodeTooLong, false},
		{"module below 0.5 mm", "folio8", "M", "30", `{}`, DiagCodeQRCodeModuleTooSmall, true},
		{"cannot fit", "folio8", "M", "0.02", `{}`, DiagCodeQRCodeDoesNotFit, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			tpl, err := ParseTemplate([]byte(qrcodeTestTemplate(t, c.value, c.level, c.side, c.side, "")))
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

func TestHiddenQRCodeDrawsAndWarnsNothing(t *testing.T) {
	tpl, err := ParseTemplate([]byte(qrcodeTestTemplate(t, "{{ref}}", "H", "300", "300", `"visibleIf": "show", `)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	pages, diags := barcodePages(t, tpl, `{"show":false,"ref":"`+strings.Repeat("x", 1274)+`"}`)
	if len(diags) != 0 || len(pages[0].Rects) != 0 {
		t.Fatalf("a hidden qrcode drew %d rects with %+v", len(pages[0].Rects), diags)
	}
}

func TestQRCodeCommandsAndCanvas(t *testing.T) {
	tpl, err := ParseTemplate([]byte(qrcodeTestTemplate(t, "1", "", "120", "120", "")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"dropComponent","version":1,"type":"qrcode","x":100,"y":300,"snap":false}`))
	if err != nil {
		t.Fatalf("drop: %v", err)
	}
	dropped := projection.Components[len(projection.Components)-1]
	if dropped.Type != "qrcode" || dropped.Width != 72000 || dropped.Height != 72000 || dropped.Value == nil || *dropped.Value != qrcodeStarterValue {
		t.Fatalf("dropped qrcode = %+v", dropped)
	}
	painted, err := canvasWithTextPaint(tpl, testShippedFontSet())
	if err != nil {
		t.Fatalf("canvas: %v", err)
	}
	if p := findCanvasComponent(t, painted, dropped.ID).QRCode; p == nil || p.ModuleWidth < int64(barcode.QRMinModuleWidth) {
		t.Errorf("the starter QR code must paint at 0.5 mm or more: %+v", p)
	}

	for _, cmd := range []string{
		`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"errorCorrection":{"op":"set","value":"H"}}}`,
		`{"kind":"bindComponentScalar","version":1,"id":"e1","segments":["ref"]}`,
	} {
		if _, err := applyComponentCommand(tpl, []byte(cmd)); err != nil {
			t.Fatalf("%s: %v", cmd, err)
		}
	}
	saved, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"errorCorrection": "H"`, `"version": "4.0"`, `"value": "{{ref}}"`} {
		if !strings.Contains(string(saved), want) {
			t.Fatalf("saved document lacks %s:\n%s", want, saved)
		}
	}
	canvas, err := canvasWithTextPaint(tpl, testShippedFontSet())
	if err != nil {
		t.Fatalf("canvas: %v", err)
	}
	e1 := findCanvasComponent(t, canvas, "e1")
	if e1.Binding == nil || *e1.Binding != "ref" {
		t.Fatalf("binding = %v, want ref", e1.Binding)
	}
	if e1.Authored == nil || e1.Authored.ErrorCorrection.State != "value" || *e1.Authored.ErrorCorrection.Value != "H" {
		t.Fatalf("authored errorCorrection = %+v", e1.Authored)
	}
	// A bound value draws illustrative modules: the placeholder as 0123456789.
	want, _ := layoutQRCode("e1", illustrativeBarcodePlaceholder, barcode.ECLevelH, 120000, 120000)
	if e1.QRCode == nil || e1.QRCodeUnavailable != nil || len(e1.QRCode.Rects) != len(want.rects) {
		t.Fatalf("canvas paint %+v unavailable %v, want the illustrative symbol's %d rects", e1.QRCode, e1.QRCodeUnavailable, len(want.rects))
	}
	if canvas.ContentWindowCountIsExact {
		t.Error("a bound content-band qrcode must mark the window count inexact")
	}

	// Escapes decode; clearing the level removes the key.
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"expression":{"op":"set","value":"A\\r{{ref}}"},"errorCorrection":{"op":"clear"}}}`)); err != nil {
		t.Fatalf("set expression and clear level: %v", err)
	}
	saved, _ = SerializeTemplate(tpl)
	if strings.Contains(string(saved), "errorCorrection") || !strings.Contains(string(saved), `"value": "A\r{{ref}}"`) {
		t.Fatalf("want no errorCorrection and a stored carriage return:\n%s", saved)
	}

	for _, bad := range []string{
		`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"errorCorrection":{"op":"set","value":"X"}}}`,
		`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"errorCorrection":{"op":"null"}}}`,
		`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"background":{"op":"set","value":"#ff0000"}}}`,
		`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"value":{"op":"set","value":"a\\tb"}}}`,
	} {
		if _, err := applyComponentCommand(tpl, []byte(bad)); err == nil {
			t.Errorf("command must be refused: %s", bad)
		}
	}
}

func TestErrorCorrectionIsRefusedOnOtherKinds(t *testing.T) {
	tpl, err := ParseTemplate([]byte(barcodeTestTemplate(t, "1234", "300", "50", "")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"errorCorrection":{"op":"set","value":"M"}}}`)); err == nil {
		t.Fatal("errorCorrection must be refused on a barcode")
	}
	canvas, err := canvasWithTextPaint(tpl, testShippedFontSet())
	if err != nil {
		t.Fatal(err)
	}
	if a := findCanvasComponent(t, canvas, "e1").Authored; a == nil || a.ErrorCorrection.State != "absent" {
		t.Fatalf("a barcode's authored errorCorrection must be absent: %+v", a)
	}
}

func TestStaticQRCodeCanvasMatchesRenderAndStaysExact(t *testing.T) {
	tpl, err := ParseTemplate([]byte(qrcodeTestTemplate(t, "A\r1234", "L", "90", "90", "")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	canvas, err := canvasWithTextPaint(tpl, testShippedFontSet())
	if err != nil {
		t.Fatalf("canvas: %v", err)
	}
	e1 := findCanvasComponent(t, canvas, "e1")
	if e1.Value == nil || *e1.Value != "A\n1234" {
		t.Fatalf("the canvas must show the escaped value, got %v", e1.Value)
	}
	pages, _ := barcodePages(t, tpl, `{}`)
	if e1.QRCode == nil || len(e1.QRCode.Rects) != len(pages[0].Rects) {
		t.Fatalf("canvas paint %+v vs %d rects", e1.QRCode, len(pages[0].Rects))
	}
	top := pages[0].Rects[0].Y - geom.Length(e1.QRCode.Rects[0].Y)
	for i, r := range e1.QRCode.Rects {
		pr := pages[0].Rects[i]
		if geom.Length(r.X)+20000 != pr.X || geom.Length(r.Y)+top != pr.Y || geom.Length(r.Width) != pr.W {
			t.Fatalf("rect %d differs", i)
		}
	}
	if !canvas.ContentWindowCountIsExact {
		t.Error("a static qrcode is fully knowable; the window count should stay exact")
	}
}

func TestUnfitStaticQRCodeProjectsDoesNotFitAndNoColumnItem(t *testing.T) {
	for _, c := range []struct{ value, side, reason string }{
		{"folio8", "0.02", qrcodeUnavailableDoesNotFit},
	} {
		tpl, err := ParseTemplate([]byte(qrcodeTestTemplate(t, c.value, "", c.side, c.side, "")))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		canvas, err := canvasWithTextPaint(tpl, testShippedFontSet())
		if err != nil {
			t.Fatalf("canvas: %v", err)
		}
		e1 := findCanvasComponent(t, canvas, "e1")
		if e1.QRCode != nil || e1.QRCodeUnavailable == nil || *e1.QRCodeUnavailable != c.reason {
			t.Fatalf("paint %v unavailable %v, want no paint and %s", e1.QRCode, e1.QRCodeUnavailable, c.reason)
		}
		if canvasBarcodeIsPlaced(tpl.doc.Bands.Content.Elements[0]) {
			t.Error("a qrcode that draws nothing must add no column item")
		}
	}
	// The illustrative content of a bound value can itself be too long.
	tpl, err := ParseTemplate([]byte(qrcodeTestTemplate(t, strings.Repeat("{{a}}", 128), "H", "300", "300", "")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	canvas, err := canvasWithTextPaint(tpl, testShippedFontSet())
	if err != nil {
		t.Fatalf("canvas: %v", err)
	}
	if e1 := findCanvasComponent(t, canvas, "e1"); e1.QRCode != nil || e1.QRCodeUnavailable == nil || *e1.QRCodeUnavailable != qrcodeUnavailableTooLong {
		t.Fatalf("paint %v unavailable %v, want tooLong", e1.QRCode, e1.QRCodeUnavailable)
	}
}

func TestPageHeaderQRCodeRendersOnEveryPage(t *testing.T) {
	const doc = `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e2", "type": "rect", "x": 0, "y": 0, "width": 100, "height": 20, "style": {"background": "#eeeeee"}},
      {"id": "e3", "type": "rect", "x": 0, "y": 750, "width": 100, "height": 20, "style": {"background": "#eeeeee"}}
    ]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [
      {"id": "e1", "type": "qrcode", "x": 0, "y": 0, "width": 58, "height": 58, "errorCorrection": "L", "value": "folio8"}
    ], "height": 60}
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
	want, _ := barcode.EncodeQR([]byte("folio8"), barcode.ECLevelL)
	var firstX, firstY geom.Length
	for i, page := range pages {
		var rects []pagemodel.Rect
		for _, r := range page.Rects {
			if r.HasFill && r.Fill == (pagemodel.Color{}) {
				rects = append(rects, r)
			}
		}
		assertRectsEncode(t, rects, want, 58000, 58000)
		if i == 0 {
			firstX, firstY = rects[0].X, rects[0].Y
		} else if rects[0].X != firstX || rects[0].Y != firstY {
			t.Errorf("page %d's QR code starts at (%d, %d), page 1's at (%d, %d)", i+1, rects[0].X, rects[0].Y, firstX, firstY)
		}
	}
}

// TestRaisingQRCodeLevelOverCapacityIsRefused: a static value that fits at L
// but not at H is refused when the command raises the level, and the document
// is left unchanged (the command re-validates by reparsing).
func TestRaisingQRCodeLevelOverCapacityIsRefused(t *testing.T) {
	tpl, err := ParseTemplate([]byte(qrcodeTestTemplate(t, strings.Repeat("x", 1500), "L", "400", "400", "")))
	if err != nil {
		t.Fatalf("1500 static bytes fit version 40 at L: %v", err)
	}
	before, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"errorCorrection":{"op":"set","value":"H"}}}`)); err == nil {
		t.Fatal("raising the level to H must be refused: 1500 bytes exceed the 1273 version 40 holds at H")
	}
	after, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("a refused command changed the document:\n%s\n---\n%s", before, after)
	}
}

func TestQRCodeParameterReferences(t *testing.T) {
	tpl, err := ParseTemplate([]byte(qrcodeTestTemplate(t, "{{params.amount}}|{{ref}}", "", "120", "120", "")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got, err := ParameterReferences(tpl)
	if err != nil || len(got) != 1 || got[0] != "amount" {
		t.Fatalf("references = %#v, err=%v; want [amount] from the qrcode value", got, err)
	}
}
