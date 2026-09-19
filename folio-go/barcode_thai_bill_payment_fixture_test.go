package folio8

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/barcode"
	"github.com/panitw/folio8/folio-go/internal/geom"
)

const barcodeThaiBillPaymentFixtureDir = "barcode-thai-bill-payment"

func renderBarcodeThaiBillPayment(t *testing.T) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(barcodeThaiBillPaymentTemplateJSON))
	if err != nil {
		t.Fatalf("parse barcode fixture: %v", err)
	}
	res, err := Render(tpl, Data(barcodeThaiBillPaymentDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render barcode fixture: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Fatalf("the barcode fixture must render with NO diagnostics; got %+v", res.Diagnostics)
	}
	return res.Bytes
}

// barcodeThaiBillPaymentWantBars is how many bars the resolved payload
// encodes to: three per symbol and four for STOP.
func barcodeThaiBillPaymentWantBars() int {
	values, err := barcode.Encode(barcodeThaiBillPaymentResolved)
	if err != nil {
		return -1
	}
	return len(barcode.Runs(values))/2 + 1
}

// barcodeThaiBillPaymentAssertBars is the per-leg feature guard: the PDF must
// carry exactly one filled rectangle per encoded bar, so a target that drew
// nothing, or drew a different symbol, fails its own leg.
func barcodeThaiBillPaymentAssertBars(raw []byte, fail func(string, ...any)) {
	want := barcodeThaiBillPaymentWantBars()
	if got := bytes.Count(raw, []byte(" re f\n")); got != want {
		fail("the PDF carries %d filled rectangles, want the %d bars of %q", got, want, barcodeThaiBillPaymentResolved)
	}
}

func TestBarcodeThaiBillPaymentGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", barcodeThaiBillPaymentFixtureDir)
	for _, c := range []struct{ file, want string }{
		{"input.folio", barcodeThaiBillPaymentTemplateJSON},
		{"data.json", barcodeThaiBillPaymentDataJSON},
	} {
		got, err := os.ReadFile(filepath.Join(dir, c.file))
		if err != nil {
			t.Fatalf("read %s: %v", c.file, err)
		}
		if string(got) != c.want {
			t.Fatalf("fixtures/%s/%s has drifted from its Go constant (barcode_thai_bill_payment_template.go)", barcodeThaiBillPaymentFixtureDir, c.file)
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
	if got := sha256Hex(renderBarcodeThaiBillPayment(t)); got != fixture.SHA256 {
		t.Fatalf("golden fixture mismatch: got sha256 %s, want %s (fixtures/%s). Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change. Do not regenerate the fixture to make this pass.", got, fixture.SHA256, barcodeThaiBillPaymentFixtureDir)
	}
}

func TestBarcodeThaiBillPaymentSemanticAcceptance(t *testing.T) {
	tpl, err := ParseTemplate([]byte(barcodeThaiBillPaymentTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	pages, diags := barcodePages(t, tpl, barcodeThaiBillPaymentDataJSON)
	if len(diags) != 0 || len(pages) != 1 {
		t.Fatalf("pages %d diags %+v", len(pages), diags)
	}
	runs, module := runsFromRects(t, pages[0].Rects, 50000)
	values, err := barcode.Encode(barcodeThaiBillPaymentResolved)
	if err != nil {
		t.Fatal(err)
	}
	want := barcode.Runs(values)
	if len(runs) != len(want) {
		t.Fatalf("drawn %d runs, want %d", len(runs), len(want))
	}
	for i := range want {
		if runs[i] != want[i] {
			t.Fatalf("run %d is %d modules, want %d", i, runs[i], want[i])
		}
	}
	if module < barcode.MinModuleWidth {
		t.Errorf("module %d mp is below the scannable minimum", module)
	}
	total := geom.Length(barcode.Modules(want) + 2*barcode.QuietZoneModules)
	if module != geom.Length(400000)/total {
		t.Errorf("module %d mp, want the largest fit %d", module, geom.Length(400000)/total)
	}
	rects := pages[0].Rects
	if rects[0].X < 10*module || 400000-(rects[len(rects)-1].X+rects[len(rects)-1].W) < 10*module {
		t.Error("a quiet zone is below 10 modules")
	}
	barcodeThaiBillPaymentAssertBars(renderBarcodeThaiBillPayment(t), t.Errorf)
}
