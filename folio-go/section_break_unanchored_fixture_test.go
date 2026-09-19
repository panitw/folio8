package folio8

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

const sectionBreakUnanchoredFixtureDir = "section-break-unanchored"

func renderSectionBreakUnanchored(t *testing.T) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(sectionBreakUnanchoredTemplateJSON))
	if err != nil {
		t.Fatalf("parse section-break-unanchored fixture: %v", err)
	}
	res, err := Render(tpl, Data(sectionBreakUnanchoredDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render section-break-unanchored fixture: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Fatalf("the section-break-unanchored fixture must render with NO diagnostics; got %+v", res.Diagnostics)
	}
	return res.Bytes
}

// sectionBreakUnanchoredAssertLegend is the per-leg feature guard: one page,
// the legend heading drawn exactly once, and "Page 1 of 1" — so a target that
// moved the legend to an added page, or left it under the rows twice, fails
// its own leg.
func sectionBreakUnanchoredAssertLegend(t *testing.T, raw []byte, fail func(string, ...any)) {
	pages := statementPageRuns(t, raw)
	if len(pages) != 1 {
		fail("the PDF has %d pages, want 1", len(pages))
		return
	}
	legend, pageNumber := 0, false
	for _, run := range pages[0] {
		if strings.HasPrefix(run.Text, sectionBreakStatementLegendMarker) {
			legend++
		}
		if run.Text == "Page 1 of 1" {
			pageNumber = true
		}
	}
	if legend != 1 {
		fail("the legend heading is drawn %d times, want exactly 1", legend)
	}
	if !pageNumber {
		fail("page 1 does not draw %q", "Page 1 of 1")
	}
}

func TestSectionBreakUnanchoredGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", sectionBreakUnanchoredFixtureDir)
	for _, c := range []struct{ file, want string }{
		{"input.folio", sectionBreakUnanchoredTemplateJSON},
		{"data.json", sectionBreakUnanchoredDataJSON},
	} {
		got, err := os.ReadFile(filepath.Join(dir, c.file))
		if err != nil {
			t.Fatalf("read %s: %v", c.file, err)
		}
		if string(got) != c.want {
			t.Fatalf("fixtures/%s/%s has drifted from its Go constant (section_break_unanchored_template.go)", sectionBreakUnanchoredFixtureDir, c.file)
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
	if got := sha256Hex(renderSectionBreakUnanchored(t)); got != fixture.SHA256 {
		t.Fatalf("golden fixture mismatch: got sha256 %s, want %s (fixtures/%s). Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change. Do not regenerate the fixture to make this pass.", got, fixture.SHA256, sectionBreakUnanchoredFixtureDir)
	}
}

// TestSectionBreakUnanchoredSemanticAcceptance checks the golden's own claim
// against the page model, with an oracle from two OTHER renders: the legend's
// declared baseline (five rows, nothing crossed) and where the rows end.
func TestSectionBreakUnanchoredSemanticAcceptance(t *testing.T) {
	tpl, err := ParseTemplate([]byte(sectionBreakUnanchoredTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	pages, diags := barcodePages(t, tpl, sectionBreakUnanchoredDataJSON)
	if len(diags) != 0 || len(pages) != 1 {
		t.Fatalf("pages %d diags %+v, want 1 page and no diagnostics", len(pages), diags)
	}
	onPages, y := legendHeadingOn(pages)
	if len(onPages) != 1 || onPages[0] != 0 {
		t.Fatalf("legend on pages %v, want page 1 only", onPages)
	}
	rows := 0
	for _, r := range pages[0].Runs {
		switch r.SourceText {
		case "DEP", "WDL", "TRF", "FEE", "INT":
			rows++
		}
	}
	if rows != 35 {
		t.Fatalf("page 1 carries %d rows, want all 35", rows)
	}
	// The table's header rect is 38pt below the content origin; the line is
	// 390pt below it; the rows end at the lowest rect bottom.
	top, end := pages[0].Rects[0].Y, geom.Length(0)
	for _, r := range pages[0].Rects {
		top = min(top, r.Y)
		end = max(end, r.Y+r.H)
	}
	line := top - 38000 + 390000
	if end <= line {
		t.Fatalf("precondition: the rows end at %d, not past the line %d", end, line)
	}
	short, _ := barcodePages(t, tpl, fiveRowsOf(t, sectionBreakUnanchoredDataJSON))
	shortPages, declaredY := legendHeadingOn(short)
	if len(short) != 1 || len(shortPages) != 1 {
		t.Fatalf("with five rows: %d pages, legend on %v", len(short), shortPages)
	}
	if want := declaredY + (end - line); y != want {
		t.Errorf("legend baseline %d, want %d — its declared %d pushed by the %d the rows run past the line", y, want, declaredY, end-line)
	}
	anchored, err := ParseTemplate([]byte(strings.Replace(sectionBreakUnanchoredTemplateJSON, `,
      "sectionBreakAnchor": false`, "", 1)))
	if err != nil {
		t.Fatal(err)
	}
	if anchoredPages, _ := barcodePages(t, anchored, sectionBreakUnanchoredDataJSON); len(anchoredPages) != 2 {
		t.Errorf("precondition: anchored, the same document has %d pages, want 2 — the push must be the unanchored rule's doing", len(anchoredPages))
	}
	sectionBreakUnanchoredAssertLegend(t, renderSectionBreakUnanchored(t), t.Errorf)
}

// TestSectionBreakUnanchoredRendersIdenticallyInAFreshProcess renders the
// golden in a fresh OS process and compares it with this process's render.
func TestSectionBreakUnanchoredRendersIdenticallyInAFreshProcess(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), subprocessSectionBreakUnanchoredEnvVar+"=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("subprocess render failed: %v\n%s", err, stderr.String())
	}
	if here := renderSectionBreakUnanchored(t); !bytes.Equal(stdout.Bytes(), here) {
		off, window := firstDivergence(stdout.Bytes(), here)
		t.Fatalf("fresh-process render differs at byte %d: %s", off, window)
	}
}
