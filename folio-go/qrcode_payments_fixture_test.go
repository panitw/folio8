package folio8

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/barcode"
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
)

const qrcodePaymentsFixtureDir = "qrcode-payments"

func renderQRCodePayments(t *testing.T) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(qrcodePaymentsTemplateJSON))
	if err != nil {
		t.Fatalf("parse qrcode fixture: %v", err)
	}
	res, err := Render(tpl, Data(qrcodePaymentsDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render qrcode fixture: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Fatalf("the qrcode fixture must render with NO diagnostics; got %+v", res.Diagnostics)
	}
	return res.Bytes
}

// qrcodePaymentsWantRects is how many filled rectangles the four symbols
// encode to: one per horizontal run of dark modules.
func qrcodePaymentsWantRects() int {
	total := 0
	for _, s := range qrcodePaymentsSymbols {
		level, _ := barcode.ParseECLevel(s.level)
		sym, err := barcode.EncodeQR([]byte(s.resolved), level)
		if err != nil {
			return -1
		}
		fit, ok := barcode.FitQR(sym.Size, geom.Length(s.boxW), geom.Length(s.boxH))
		if !ok {
			return -1
		}
		total += len(barcode.QRRects(sym, fit))
	}
	return total
}

// qrcodePaymentsAssertRects is the per-leg feature guard: the PDF must carry
// exactly one filled rectangle per module run of the four symbols, so a
// target that drew nothing, or drew a different symbol, fails its own leg.
func qrcodePaymentsAssertRects(raw []byte, fail func(string, ...any)) {
	want := qrcodePaymentsWantRects()
	if got := bytes.Count(raw, []byte(" re f\n")); got != want {
		fail("the PDF carries %d filled rectangles, want the %d module runs of the fixture's four QR codes", got, want)
	}
}

func TestQRCodePaymentsGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", qrcodePaymentsFixtureDir)
	for _, c := range []struct{ file, want string }{
		{"input.folio", qrcodePaymentsTemplateJSON},
		{"data.json", qrcodePaymentsDataJSON},
	} {
		got, err := os.ReadFile(filepath.Join(dir, c.file))
		if err != nil {
			t.Fatalf("read %s: %v", c.file, err)
		}
		if string(got) != c.want {
			t.Fatalf("fixtures/%s/%s has drifted from its Go constant (qrcode_payments_template.go)", qrcodePaymentsFixtureDir, c.file)
		}
	}
	fixture := loadExpectedFixture(t, filepath.Join(dir, "expected.json"))
	if fixture.Folio8GoVersion == "" || fixture.GoToolchain == "" || !isSHA256HexString(fixture.SHA256) {
		t.Fatalf("expected.json is incomplete: %+v", fixture)
	}
	expectedPDF, err := os.ReadFile(filepath.Join(dir, "expected.pdf"))
	if err != nil {
		t.Fatalf("read expected.pdf: %v", err)
	}
	if onDisk := sha256Hex(expectedPDF); onDisk != fixture.SHA256 {
		t.Fatalf("expected.pdf's sha256 %s does not match expected.json's %s", onDisk, fixture.SHA256)
	}
	if got := sha256Hex(renderQRCodePayments(t)); got != fixture.SHA256 {
		t.Fatalf("golden fixture mismatch: got sha256 %s, want %s (fixtures/%s). Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change. Do not regenerate the fixture to make this pass.", got, fixture.SHA256, qrcodePaymentsFixtureDir)
	}
}

// TestQRCodePaymentsSemanticAcceptance reads each QR code's rects back out of
// the page model and checks they are exactly the modules of its resolved
// string's encoding at its declared level, square, whole-millipoint and
// centred in its box.
func TestQRCodePaymentsSemanticAcceptance(t *testing.T) {
	tpl, err := ParseTemplate([]byte(qrcodePaymentsTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	pages, diags := barcodePages(t, tpl, qrcodePaymentsDataJSON)
	if len(diags) != 0 || len(pages) != 1 {
		t.Fatalf("pages %d diags %+v", len(pages), diags)
	}
	rects := pages[0].Rects
	bands, err := documentBands(tpl)
	if err != nil {
		t.Fatal(err)
	}
	elements := tpl.doc.Bands.Content.Elements
	if len(elements) != len(qrcodePaymentsSymbols) {
		t.Fatalf("fixture has %d content elements, the symbol table %d", len(elements), len(qrcodePaymentsSymbols))
	}
	for i, s := range qrcodePaymentsSymbols {
		el := elements[i]
		if string(el.ID) != s.id {
			t.Fatalf("element %d is %s, the symbol table names %s", i, el.ID, s.id)
		}
		level, _ := barcode.ParseECLevel(s.level)
		sym, err := barcode.EncodeQR([]byte(s.resolved), level)
		if err != nil {
			t.Fatal(err)
		}
		fit, _ := barcode.FitQR(sym.Size, geom.Length(s.boxW), geom.Length(s.boxH))
		n := len(barcode.QRRects(sym, fit))
		if len(rects) < n {
			t.Fatalf("%s: %d rects left, want %d", s.id, len(rects), n)
		}
		var mine []pagemodel.Rect
		mine, rects = rects[:n], rects[n:]
		module, originX, originY := assertRectsEncode(t, mine, sym, geom.Length(s.boxW), geom.Length(s.boxH))
		// Centred: the symbol's top-left module sits at the element's page
		// position plus FitQR's offsets, which leave equal space either side.
		if wantX, wantY := el.X+fit.OffsetX, bands[1].origin+el.Y+fit.OffsetY; originX != wantX || originY != wantY {
			t.Errorf("%s: symbol origin (%d, %d), want the centred (%d, %d)", s.id, originX, originY, wantX, wantY)
		}
		if module < barcode.QRMinModuleWidth {
			t.Errorf("%s: module %d mp is below 0.5 mm", s.id, module)
		}
		if sym.Level.String() != s.level {
			t.Errorf("%s: encoded at %s, declared %s", s.id, sym.Level, s.level)
		}
	}
	if len(rects) != 0 {
		t.Fatalf("%d rects are left over after the four QR codes", len(rects))
	}
	qrcodePaymentsAssertRects(renderQRCodePayments(t), t.Errorf)
}
