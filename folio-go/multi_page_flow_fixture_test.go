package folio8

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
	"github.com/panitw/folio8/folio-go/internal/template"
)

const multiPageFlowFixtureDir = "multi-page-flow"

// The fixture's marker texts, one per part of the flow.
const (
	multiPageFlowLegend       = "Codes: DEP deposit, WDL withdrawal, TRF transfer, FEE fee, INT interest"
	multiPageFlowCharges      = "Service charges"
	multiPageFlowChargesNote  = "Charges are debited on the last business day of the month."
	multiPageFlowAcknowledged = "Acknowledged by the account holder"
	multiPageFlowSignature    = "Account holder signature"
)

func renderMultiPageFlow(t *testing.T) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(multiPageFlowTemplateJSON))
	if err != nil {
		t.Fatalf("parse multi-page-flow fixture: %v", err)
	}
	res, err := Render(tpl, Data(multiPageFlowDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render multi-page-flow fixture: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Fatalf("the multi-page-flow fixture must render with NO diagnostics; got %+v", res.Diagnostics)
	}
	return res.Bytes
}

// multiPageFlowAssertPages is the per-leg feature guard: four pages, the
// header and "Page N of 4" on every page, the legend on page 2, the charges
// heading on page 3 and the charges note and signature on page 4, each once.
func multiPageFlowAssertPages(t *testing.T, raw []byte, fail func(string, ...any)) {
	pages := statementPageRuns(t, raw)
	if len(pages) != 4 {
		fail("the PDF has %d pages, want 4", len(pages))
		return
	}
	wantOn := map[string]int{multiPageFlowLegend: 1, multiPageFlowCharges: 2, multiPageFlowChargesNote: 3, multiPageFlowAcknowledged: 3, multiPageFlowSignature: 3}
	for i, runs := range pages {
		header, footer := 0, false
		seen := map[string]int{}
		for _, run := range runs {
			switch run.Text {
			case "STATEMENT OF ACCOUNT":
				header++
			case fmt.Sprintf("Page %d of 4", i+1):
				footer = true
			}
			if _, ok := wantOn[run.Text]; ok {
				seen[run.Text]++
			}
		}
		if header != 1 || !footer {
			fail("page %d: header drawn %d times, footer %q present %v", i+1, header, fmt.Sprintf("Page %d of 4", i+1), footer)
		}
		for text, page := range wantOn {
			if want := map[bool]int{true: 1, false: 0}[i == page]; seen[text] != want {
				fail("page %d draws %q %d times, want %d", i+1, text, seen[text], want)
			}
		}
	}
}

func TestMultiPageFlowGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", multiPageFlowFixtureDir)
	for _, c := range []struct{ file, want string }{
		{"input.folio", multiPageFlowTemplateJSON},
		{"data.json", multiPageFlowDataJSON},
	} {
		got, err := os.ReadFile(filepath.Join(dir, c.file))
		if err != nil {
			t.Fatalf("read %s: %v", c.file, err)
		}
		if string(got) != c.want {
			t.Fatalf("fixtures/%s/%s has drifted from its Go constant (multi_page_flow_template.go)", multiPageFlowFixtureDir, c.file)
		}
	}
	tpl, err := ParseTemplate([]byte(multiPageFlowTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := template.SerializeDocument(tpl.doc)
	if err != nil {
		t.Fatal(err)
	}
	if string(saved) != multiPageFlowTemplateJSON {
		t.Fatalf("input.folio does not save back byte-for-byte:\n%s", saved)
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
	if got := sha256Hex(renderMultiPageFlow(t)); got != fixture.SHA256 {
		t.Fatalf("golden fixture mismatch: got sha256 %s, want %s (fixtures/%s). Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change. Do not regenerate the fixture to make this pass.", got, fixture.SHA256, multiPageFlowFixtureDir)
	}
}

// multiPageFlowAlone renders designed page `page` of the fixture alone, as a
// one-page document with its own break and the same data.
func multiPageFlowAlone(t *testing.T, page int) []pagemodel.Page {
	t.Helper()
	doc, err := template.ParseDocument([]byte(multiPageFlowTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	doc.Bands.Content = doc.Pages[page].Band
	doc.Pages = nil
	src, err := template.SerializeDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := ParseTemplate(src)
	if err != nil {
		t.Fatal(err)
	}
	pages, diags := barcodePages(t, tpl, multiPageFlowDataJSON)
	if len(diags) != 0 {
		t.Fatalf("page %d alone: diags %+v", page+1, diags)
	}
	return pages
}

// flowDrawn is a page's content drawing, without the shared header and
// footer, and without the runs and rects `skip` names.
func flowDrawn(p pagemodel.Page, skip func(text string) bool, skipRect func(r pagemodel.Rect) bool) ([][3]any, []pagemodel.Rect) {
	runs, rects := contentDrawn(p)
	var keptRuns [][3]any
	for _, r := range runs {
		if !skip(r[0].(string)) {
			keptRuns = append(keptRuns, r)
		}
	}
	var keptRects []pagemodel.Rect
	for _, r := range rects {
		if !skipRect(r) {
			keptRects = append(keptRects, r)
		}
	}
	return keptRuns, keptRects
}

// TestMultiPageFlowSemanticAcceptance checks the golden's claim against the
// page model, with oracles from other renders: each designed page laid out
// alone as a one-page document.
func TestMultiPageFlowSemanticAcceptance(t *testing.T) {
	tpl, err := ParseTemplate([]byte(multiPageFlowTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	pages, diags := barcodePages(t, tpl, multiPageFlowDataJSON)
	if len(diags) != 0 || len(pages) != 4 {
		t.Fatalf("pages %d diags %+v, want 4 pages and no diagnostics", len(pages), diags)
	}
	if tpl.doc.Pages[1].PageBreak || tpl.doc.Pages[2].PageBreak {
		t.Fatal("precondition: pages 2 and 3 declare Page Break off")
	}
	none := func(string) bool { return false }
	noRect := func(pagemodel.Rect) bool { return false }
	page3Text := func(text string) bool { return text == multiPageFlowAcknowledged || text == multiPageFlowSignature }
	signatureLine := func(r pagemodel.Rect) bool { return r.W == 200000 && r.H == 1000 }

	// CAP-6, page 1: its unanchored legend follows its own rows, exactly as
	// page 1 alone draws it, on output pages 1 and 2.
	page1 := multiPageFlowAlone(t, 0)
	if len(page1) != 2 {
		t.Fatalf("page 1 alone renders %d pages, want 2", len(page1))
	}
	for i := range page1 {
		gotRuns, gotRects := flowDrawn(pages[i], none, noRect)
		wantRuns, wantRects := flowDrawn(page1[i], none, noRect)
		if !reflect.DeepEqual(gotRuns, wantRuns) || !reflect.DeepEqual(gotRects, wantRects) {
			t.Errorf("output page %d differs from page 1 alone", i+1)
		}
	}

	// CAP-9 doesn't fit, and CAP-6 page 2: page 2 needs two output pages, so it
	// starts a new one, and draws exactly what it draws alone with its own break.
	page2 := multiPageFlowAlone(t, 1)
	if len(page2) != 2 {
		t.Fatalf("page 2 alone renders %d pages, want 2", len(page2))
	}
	for i := range page2 {
		gotRuns, gotRects := flowDrawn(pages[2+i], page3Text, signatureLine)
		wantRuns, wantRects := flowDrawn(page2[i], none, noRect)
		if !reflect.DeepEqual(gotRuns, wantRuns) || !reflect.DeepEqual(gotRects, wantRects) {
			t.Errorf("output page %d differs from page 2 alone:\n got %v\nwant %v", 3+i, gotRuns, wantRuns)
		}
	}
	if on, _ := headingBaseline(pages, multiPageFlowCharges); !reflect.DeepEqual(on, []int{2}) {
		t.Errorf("the charges heading is on %v, want output page 3", on)
	}

	// CAP-9 fits: page 3's block is on output page 4, every item moved down by
	// the same distance, which is where page 2's content ends there. That end
	// is the section-break measure — the lowest drawn item bottom, not a
	// declared box — so it is the bottom of the note's one 8pt line: its y of
	// 410pt plus that line's 10.896pt height, 420.896pt down the window.
	page3 := multiPageFlowAlone(t, 2)
	if len(page3) != 1 {
		t.Fatalf("page 3 alone renders %d pages, want 1", len(page3))
	}
	var moved []geom.Length
	for _, text := range []string{multiPageFlowAcknowledged, multiPageFlowSignature} {
		on, y := headingBaseline(pages, text)
		_, declared := headingBaseline(page3, text)
		if !reflect.DeepEqual(on, []int{3}) {
			t.Fatalf("%q is on %v, want output page 4", text, on)
		}
		moved = append(moved, y-declared)
	}
	var lineY, declaredLineY geom.Length
	for _, r := range pages[3].Rects {
		if signatureLine(r) {
			lineY = r.Y
		}
	}
	for _, r := range page3[0].Rects {
		if signatureLine(r) {
			declaredLineY = r.Y
		}
	}
	moved = append(moved, lineY-declaredLineY)
	if moved[0] != 420896 || moved[1] != moved[0] || moved[2] != moved[0] {
		t.Errorf("page 3's items moved by %v, want all by 420896 (page 2's end on output page 4)", moved)
	}
	multiPageFlowAssertPages(t, renderMultiPageFlow(t), t.Errorf)
}

// TestMultiPageFlowRendersIdenticallyInAFreshProcess renders the golden in a
// fresh OS process and compares it with this process's render.
func TestMultiPageFlowRendersIdenticallyInAFreshProcess(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), subprocessMultiPageFlowEnvVar+"=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("subprocess render failed: %v\n%s", err, stderr.String())
	}
	if here := renderMultiPageFlow(t); !bytes.Equal(stdout.Bytes(), here) {
		off, window := firstDivergence(stdout.Bytes(), here)
		t.Fatalf("fresh-process render differs at byte %d: %s", off, window)
	}
}
