package folio8

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
)

const sectionBreakStatementFixtureDir = "section-break-statement"

// sectionBreakStatementLegendMarker is the Latin head of the legend heading.
// The heading is bilingual, so it is drawn as one run per face and the Thai
// half is a run of its own; the Latin run is the one that is searched for.
const sectionBreakStatementLegendMarker = "Transaction codes"

// legendHeadingOn returns the pages the legend heading's Latin run is drawn
// on, and its baseline.
func legendHeadingOn(pages []pagemodel.Page) (onPages []int, y geom.Length) {
	for p, pg := range pages {
		for _, r := range pg.Runs {
			if strings.HasPrefix(r.SourceText, sectionBreakStatementLegendMarker) {
				onPages = append(onPages, p)
				y = r.Y
				break
			}
		}
	}
	return onPages, y
}

func renderSectionBreakStatement(t *testing.T) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(sectionBreakStatementTemplateJSON))
	if err != nil {
		t.Fatalf("parse section-break fixture: %v", err)
	}
	res, err := Render(tpl, Data(sectionBreakStatementDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render section-break fixture: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Fatalf("the section-break fixture must render with NO diagnostics; got %+v", res.Diagnostics)
	}
	return res.Bytes
}

// sectionBreakStatementAssertLegend is the per-leg feature guard: two pages,
// the legend heading drawn exactly once and on the last page, and "Page X of
// 2" on both — so a target that left the legend under the rows fails its own
// leg.
//
// The document embeds two faces, so its text is decoded through the
// per-resource /ToUnicode CMaps (statementPageRuns), never a merged map.
func sectionBreakStatementAssertLegend(t *testing.T, raw []byte, fail func(string, ...any)) {
	pages := statementPageRuns(t, raw)
	if len(pages) != 2 {
		fail("the PDF has %d pages, want 2", len(pages))
		return
	}
	legend := 0
	for p, runs := range pages {
		onPage, pageNumber := false, false
		want := "Page " + itoaForTest(int64(p+1)) + " of 2"
		for _, run := range runs {
			if strings.HasPrefix(run.Text, sectionBreakStatementLegendMarker) {
				onPage = true
			}
			if run.Text == want {
				pageNumber = true
			}
		}
		if onPage {
			legend++
			if p != len(pages)-1 {
				fail("the legend is drawn on page %d, not the last page", p+1)
			}
		}
		if !pageNumber {
			fail("page %d does not draw %q", p+1, want)
		}
	}
	if legend != 1 {
		fail("the legend heading is drawn on %d pages, want exactly 1", legend)
	}
}

func TestSectionBreakStatementGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", sectionBreakStatementFixtureDir)
	for _, c := range []struct{ file, want string }{
		{"input.folio", sectionBreakStatementTemplateJSON},
		{"data.json", sectionBreakStatementDataJSON},
	} {
		got, err := os.ReadFile(filepath.Join(dir, c.file))
		if err != nil {
			t.Fatalf("read %s: %v", c.file, err)
		}
		if string(got) != c.want {
			t.Fatalf("fixtures/%s/%s has drifted from its Go constant (section_break_statement_template.go)", sectionBreakStatementFixtureDir, c.file)
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
	if got := sha256Hex(renderSectionBreakStatement(t)); got != fixture.SHA256 {
		t.Fatalf("golden fixture mismatch: got sha256 %s, want %s (fixtures/%s). Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change. Do not regenerate the fixture to make this pass.", got, fixture.SHA256, sectionBreakStatementFixtureDir)
	}
}

// TestSectionBreakStatementSemanticAcceptance checks the golden's own claim
// against the page model: forty rows cross the line on page 1, so the legend
// is alone on an added page 2 at exactly its page-1 position in the same
// document rendered with five rows.
func TestSectionBreakStatementSemanticAcceptance(t *testing.T) {
	tpl, err := ParseTemplate([]byte(sectionBreakStatementTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	pages, diags := barcodePages(t, tpl, sectionBreakStatementDataJSON)
	if len(diags) != 0 || len(pages) != 2 {
		t.Fatalf("pages %d diags %+v, want 2 pages and no diagnostics", len(pages), diags)
	}
	onPages, y := legendHeadingOn(pages)
	if len(onPages) != 1 || onPages[0] != 1 {
		t.Fatalf("legend on pages %v, want page 2 only", onPages)
	}
	rows := 0
	for _, r := range pages[0].Runs {
		if r.SourceText == "DEP" || r.SourceText == "WDL" || r.SourceText == "TRF" || r.SourceText == "FEE" || r.SourceText == "INT" {
			rows++
		}
	}
	if rows != 40 {
		t.Fatalf("page 1 carries %d rows, want all 40 — the fixture must cross the line on page 1", rows)
	}
	for _, r := range pages[1].Runs {
		if r.SourceText == "DEP" || r.SourceText == "WDL" {
			t.Errorf("page 2 carries row text %q above the legend", r.SourceText)
		}
	}
	short, _ := barcodePages(t, tpl, fiveRowsOf(t, sectionBreakStatementDataJSON))
	shortPages, shortY := legendHeadingOn(short)
	if len(short) != 1 || len(shortPages) != 1 || shortPages[0] != 0 {
		t.Fatalf("with five rows: %d pages, legend on %v; want one page carrying it", len(short), shortPages)
	}
	if y != shortY {
		t.Errorf("legend baseline on the added page is %d, want its declared position %d", y, shortY)
	}
	sectionBreakStatementAssertLegend(t, renderSectionBreakStatement(t), t.Errorf)
}

// fiveRowsOf keeps the first five transactions of the fixture's record.
func fiveRowsOf(t *testing.T, data string) string {
	t.Helper()
	start := 0
	for i := 0; i < 6; i++ {
		next := strings.Index(data[start+1:], `{"amount":`)
		if next < 0 {
			t.Fatal("fixture precondition: fewer than six transactions")
		}
		start += 1 + next
	}
	// start is the sixth transaction; cut the array there.
	return data[:start-1] + "]}"
}
