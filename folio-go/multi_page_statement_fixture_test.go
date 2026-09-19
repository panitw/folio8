package folio8

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
	"github.com/panitw/folio8/folio-go/internal/template"
)

const multiPageStatementFixtureDir = "multi-page-statement"

// multiPageStatementPage2Heading is the first text of the second designed page.
const multiPageStatementPage2Heading = "Terms and conditions"

func renderMultiPageStatement(t *testing.T) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(multiPageStatementTemplateJSON))
	if err != nil {
		t.Fatalf("parse multi-page-statement fixture: %v", err)
	}
	res, err := Render(tpl, Data(multiPageStatementDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render multi-page-statement fixture: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Fatalf("the multi-page-statement fixture must render with NO diagnostics; got %+v", res.Diagnostics)
	}
	return res.Bytes
}

// multiPageStatementAssertPages is the per-leg feature guard: four pages, the
// header and "Page N of 4" on every page, and page 2's heading on page 4 only.
func multiPageStatementAssertPages(t *testing.T, raw []byte, fail func(string, ...any)) {
	pages := statementPageRuns(t, raw)
	if len(pages) != 4 {
		fail("the PDF has %d pages, want 4", len(pages))
		return
	}
	for i, runs := range pages {
		header, footer, heading := 0, false, 0
		for _, run := range runs {
			switch run.Text {
			case "STATEMENT OF ACCOUNT":
				header++
			case fmt.Sprintf("Page %d of 4", i+1):
				footer = true
			case multiPageStatementPage2Heading:
				heading++
			}
		}
		if header != 1 || !footer {
			fail("page %d: header drawn %d times, footer %q present %v", i+1, header, fmt.Sprintf("Page %d of 4", i+1), footer)
		}
		if want := map[bool]int{true: 1, false: 0}[i == 3]; heading != want {
			fail("page %d draws page 2's heading %d times, want %d", i+1, heading, want)
		}
	}
}

func TestMultiPageStatementGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", multiPageStatementFixtureDir)
	for _, c := range []struct{ file, want string }{
		{"input.folio", multiPageStatementTemplateJSON},
		{"data.json", multiPageStatementDataJSON},
	} {
		got, err := os.ReadFile(filepath.Join(dir, c.file))
		if err != nil {
			t.Fatalf("read %s: %v", c.file, err)
		}
		if string(got) != c.want {
			t.Fatalf("fixtures/%s/%s has drifted from its Go constant (multi_page_statement_template.go)", multiPageStatementFixtureDir, c.file)
		}
	}
	// The hand-written two-page file saves back byte-for-byte.
	tpl, err := ParseTemplate([]byte(multiPageStatementTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := template.SerializeDocument(tpl.doc)
	if err != nil {
		t.Fatal(err)
	}
	if string(saved) != multiPageStatementTemplateJSON {
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
	if got := sha256Hex(renderMultiPageStatement(t)); got != fixture.SHA256 {
		t.Fatalf("golden fixture mismatch: got sha256 %s, want %s (fixtures/%s). Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change. Do not regenerate the fixture to make this pass.", got, fixture.SHA256, multiPageStatementFixtureDir)
	}
}

// headingBaseline returns the pages carrying text and its baseline.
func headingBaseline(pages []pagemodel.Page, text string) (onPages []int, y geom.Length) {
	for i, p := range pages {
		for _, r := range p.Runs {
			if r.SourceText == text {
				onPages = append(onPages, i)
				y = r.Y
			}
		}
	}
	return onPages, y
}

// TestMultiPageStatementSemanticAcceptance checks the golden's claim against
// the page model, with an oracle from another render: page 2's elements laid
// out alone as a one-page document.
func TestMultiPageStatementSemanticAcceptance(t *testing.T) {
	tpl, err := ParseTemplate([]byte(multiPageStatementTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	pages, diags := barcodePages(t, tpl, multiPageStatementDataJSON)
	if len(diags) != 0 || len(pages) != 4 {
		t.Fatalf("pages %d diags %+v, want 4 pages and no diagnostics", len(pages), diags)
	}
	for i, p := range pages {
		rows := 0
		for _, r := range p.Runs {
			if r.SourceText == "Salary deposit" {
				rows++
			}
		}
		if (i < 3) != (rows > 0) {
			t.Errorf("page %d carries %d deposit rows", i+1, rows)
		}
	}
	onPages, y := headingBaseline(pages, multiPageStatementPage2Heading)
	if len(onPages) != 1 || onPages[0] != 3 {
		t.Fatalf("page 2's heading is on pages %v, want page 4 only", onPages)
	}

	doc, err := template.ParseDocument([]byte(multiPageStatementTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	if !doc.Pages[1].PageBreak {
		t.Error("precondition: page 2 declares Page Break on")
	}
	doc.Bands.Content.Elements = doc.Pages[1].Elements
	doc.Pages = nil
	alone, err := template.SerializeDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	aloneTpl, err := ParseTemplate(alone)
	if err != nil {
		t.Fatal(err)
	}
	alonePages, _ := barcodePages(t, aloneTpl, multiPageStatementDataJSON)
	aloneOn, declaredY := headingBaseline(alonePages, multiPageStatementPage2Heading)
	if len(alonePages) != 1 || len(aloneOn) != 1 {
		t.Fatalf("page 2 alone: %d pages, heading on %v", len(alonePages), aloneOn)
	}
	if y != declaredY {
		t.Errorf("page 2's heading baseline on page 4 is %d, want its declared %d", y, declaredY)
	}
	multiPageStatementAssertPages(t, renderMultiPageStatement(t), t.Errorf)
}

// TestMultiPageStatementRendersIdenticallyInAFreshProcess renders the golden
// in a fresh OS process and compares it with this process's render.
func TestMultiPageStatementRendersIdenticallyInAFreshProcess(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), subprocessMultiPageStatementEnvVar+"=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("subprocess render failed: %v\n%s", err, stderr.String())
	}
	if here := renderMultiPageStatement(t); !bytes.Equal(stdout.Bytes(), here) {
		off, window := firstDivergence(stdout.Bytes(), here)
		t.Fatalf("fresh-process render differs at byte %d: %s", off, window)
	}
}
