package folio8

import (
	"bytes"
	"errors"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio8-go/internal/geom"
	"github.com/panitw/folio8/folio8-go/internal/pagemodel"
)

// The rendering library guide (docs/rendering-library.md) embeds the example
// templates under docs/examples verbatim and states what each one renders.
// These tests keep those statements true: every example is rendered here and
// its page count, placement and diagnostic codes are asserted, every example
// file must appear verbatim in the guide, and every exported identifier of the
// folio8 and fonts packages must be named in the guide.

func docsDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRootFromTest(t), "docs")
}

func readDocsExample(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(docsDir(t), "examples", name))
	if err != nil {
		t.Fatalf("read example %s: %v", name, err)
	}
	if len(b) == 0 {
		t.Fatalf("example %s is empty", name)
	}
	return string(b)
}

func docsExamplePages(t *testing.T, templateName, dataName string) ([]pagemodel.Page, []Diagnostic) {
	t.Helper()
	tpl, err := ParseTemplate([]byte(readDocsExample(t, templateName)))
	if err != nil {
		t.Fatalf("ParseTemplate(%s): %v", templateName, err)
	}
	return barcodePages(t, tpl, readDocsExample(t, dataName))
}

func docsRender(t *testing.T, templateSource, dataName string) Result {
	t.Helper()
	tpl, err := ParseTemplate([]byte(templateSource))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	res, err := Render(tpl, Data(readDocsExample(t, dataName)), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return res
}

// runTop returns the output pages (zero-based) drawing a run with exactly
// this source text, and the run's y on the last of them.
func runTop(pages []pagemodel.Page, text string) ([]int, geom.Length) {
	return legendOn(pages, text)
}

func docsCodes(diags []Diagnostic) []string {
	codes := []string{}
	for _, d := range diags {
		codes = append(codes, d.Severity.String()+" "+d.Code+" "+d.ElementID)
	}
	return codes
}

func requireNoDiagnostics(t *testing.T, name string, diags []Diagnostic) {
	t.Helper()
	if len(diags) != 0 {
		t.Fatalf("%s: unexpected diagnostics %v", name, docsCodes(diags))
	}
}

func TestDocsExampleFirstPDF(t *testing.T) {
	pages, diags := docsExamplePages(t, "first-pdf.folio", "first-pdf.data.json")
	requireNoDiagnostics(t, "first-pdf", diags)
	if on, _ := runTop(pages, "Hello, Ada Lovelace!"); len(pages) != 1 || len(on) != 1 {
		t.Fatalf("first-pdf: %d pages, greeting on %v", len(pages), on)
	}
	tpl, err := ParseTemplate([]byte(readDocsExample(t, "first-pdf.folio")))
	if err != nil {
		t.Fatal(err)
	}
	data := Data(readDocsExample(t, "first-pdf.data.json"))
	res, err := Render(tpl, data, nil, testShippedFontSet())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(res.Bytes, []byte("%PDF-")) {
		t.Fatalf("Render did not return a PDF")
	}
	var buf bytes.Buffer
	if _, err := RenderTo(&buf, tpl, data, nil, testShippedFontSet()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), res.Bytes) {
		t.Fatalf("RenderTo wrote different bytes from Render")
	}
}

func TestDocsExampleFormulaVisibility(t *testing.T) {
	above, diags := docsExamplePages(t, "formula-visibility.folio", "formula-visibility.above.json")
	requireNoDiagnostics(t, "above", diags)
	below, diags := docsExamplePages(t, "formula-visibility.folio", "formula-visibility.below.json")
	requireNoDiagnostics(t, "below", diags)
	for _, c := range []struct {
		pages []pagemodel.Page
		text  string
		want  int
	}{
		{above, "Total 25,150.50", 1}, {above, "Manual review required", 1}, {above, "Tier: large", 1},
		{below, "Total 20,150.50", 1}, {below, "Manual review required", 0}, {below, "Tier: standard", 1},
	} {
		if on, _ := runTop(c.pages, c.text); len(on) != c.want {
			t.Errorf("%q drawn on %d pages, want %d", c.text, len(on), c.want)
		}
	}
}

func TestDocsExampleBarcodeAndQRCode(t *testing.T) {
	pages, diags := docsExamplePages(t, "barcode-qrcode.folio", "barcode-qrcode.data.json")
	requireNoDiagnostics(t, "barcode-qrcode", diags)
	if len(pages) != 1 || len(pages[0].Rects) == 0 {
		t.Fatalf("barcode-qrcode: %d pages", len(pages))
	}
	drawn := len(pages[0].Rects)

	pages, diags = docsExamplePages(t, "barcode-qrcode.folio", "barcode-qrcode.unencodable.json")
	if got := docsCodes(diags); len(got) != 1 || got[0] != "Warning "+DiagCodeBarcodeUnencodable+" e1" {
		t.Fatalf("unencodable: diagnostics %v, want one BARCODE_UNENCODABLE warning on e1", got)
	}
	if len(pages) != 1 || len(pages[0].Rects) == 0 || len(pages[0].Rects) == drawn {
		rects := 0
		if len(pages) > 0 {
			rects = len(pages[0].Rects)
		}
		t.Fatalf("unencodable: %d pages; the QR codes must still draw (rects %d, with the barcode %d)", len(pages), rects, drawn)
	}
	res := docsRender(t, readDocsExample(t, "barcode-qrcode.folio"), "barcode-qrcode.unencodable.json")
	if len(res.Bytes) == 0 || len(res.Diagnostics) != 1 {
		t.Fatalf("unencodable render: %d bytes, %d diagnostics", len(res.Bytes), len(res.Diagnostics))
	}
}

func TestDocsExampleSectionBreaks(t *testing.T) {
	anchored := readDocsExample(t, "section-break.folio")
	unanchored := readDocsExample(t, "section-break-unanchored.folio")

	// Uncrossed: identical bytes to the same document with no break.
	withoutBreak := strings.Replace(anchored, `,
      "sectionBreak": 75`, "", 1)
	if withoutBreak == anchored {
		t.Fatal("fixture precondition: sectionBreak key not found")
	}
	for name, src := range map[string]string{"anchored": anchored, "unanchored": unanchored} {
		if !bytes.Equal(docsRender(t, src, "section-break.5-rows.json").Bytes, docsRender(t, withoutBreak, "section-break.5-rows.json").Bytes) {
			t.Errorf("%s: an uncrossed break changed the PDF", name)
		}
	}

	declared, _ := docsExamplePages(t, "section-break.folio", "section-break.5-rows.json")
	_, legendY := runTop(declared, "Legend")

	pages, diags := docsExamplePages(t, "section-break.folio", "section-break.7-rows.json")
	requireNoDiagnostics(t, "anchored", diags)
	if on, y := runTop(pages, "Legend"); len(pages) != 2 || len(on) != 1 || on[0] != 1 || y != legendY {
		t.Fatalf("anchored, crossed: %d pages, legend on %v at %d; want 2 pages, legend on page 2 at its declared %d", len(pages), on, y, legendY)
	}
	if got := requirePageXOfY(t, docsRender(t, anchored, "section-break.7-rows.json").Bytes); got != 2 {
		t.Fatalf("anchored footer counts %d pages", got)
	}

	pages, diags = docsExamplePages(t, "section-break-unanchored.folio", "section-break.7-rows.json")
	requireNoDiagnostics(t, "unanchored", diags)
	_, lastRowY := runTop(pages, "Item 7")
	on, y := runTop(pages, "Legend")
	// Seven 10.896pt rows under a 10pt header end at 86.272pt, 11.272pt past the break at 75.
	if len(pages) != 1 || len(on) != 1 || y != legendY+11272 || y <= lastRowY {
		t.Fatalf("unanchored, crossed: %d pages, legend on %v at %d; want 1 page, legend at %d", len(pages), on, y, legendY+11272)
	}
}

func TestDocsExampleDesignedPages(t *testing.T) {
	on, diags := docsExamplePages(t, "designed-pages.folio", "designed-pages.data.json")
	requireNoDiagnostics(t, "page break on", diags)
	pagesOn, approvedOnY := runTop(on, "Approved by")
	if len(on) != 3 || len(pagesOn) != 1 || pagesOn[0] != 2 {
		t.Fatalf("Page Break on: %d output pages, Approved by on %v; want 3 pages, on page 3", len(on), pagesOn)
	}

	off, diags := docsExamplePages(t, "designed-pages-page-break-off.folio", "designed-pages.data.json")
	requireNoDiagnostics(t, "page break off", diags)
	pagesOff, y := runTop(off, "Approved by")
	_, lastRowY := runTop(off, "Item 12")
	// The last row's run starts 10.896pt above its row bottom; page 2 starts directly under it.
	if len(off) != 2 || len(pagesOff) != 1 || pagesOff[0] != 1 || y != lastRowY+10896 {
		t.Fatalf("Page Break off: %d output pages, Approved by on %v at %d; want 2 pages, on page 2 at %d", len(off), pagesOff, y, lastRowY+10896)
	}
	if approvedOnY >= y {
		t.Fatalf("Page Break on should draw Approved by at its declared position (%d), above %d", approvedOnY, y)
	}
	for name, file := range map[string]string{"on": "designed-pages.folio", "off": "designed-pages-page-break-off.folio"} {
		want := map[string]int{"on": 3, "off": 2}[name]
		if got := requirePageXOfY(t, docsRender(t, readDocsExample(t, file), "designed-pages.data.json").Bytes); got != want {
			t.Errorf("Page Break %s: footers count %d pages, want %d", name, got, want)
		}
	}
}

func TestDocsExampleRuledTable(t *testing.T) {
	pages, diags := docsExamplePages(t, "ruled-table.folio", "ruled-table.data.json")
	requireNoDiagnostics(t, "ruled-table", diags)
	if len(pages) != 1 {
		t.Fatalf("ruled-table: %d pages", len(pages))
	}
	rects := pages[0].Rects
	if v, h := len(verticalRules(rects)), len(horizontalRules(rects)); v != 1 || h != 3 {
		t.Fatalf("ruled-table: %d vertical and %d horizontal rules, want 1 and 3", v, h)
	}
	frame := false
	for _, r := range rects {
		frame = frame || (r.HasStroke && r.W == 180000 && r.H == 90000)
	}
	if !frame {
		t.Fatalf("ruled-table: no 180x90pt frame drawn to the minHeight floor")
	}

	_, err := ParseTemplate([]byte(readDocsExample(t, "ruled-table-unplaceable.folio")))
	var re *RenderError
	if !errors.As(err, &re) || re.Diagnostic.Code != DiagCodeTableMinHeightUnplaceable || re.Diagnostic.ElementID != "e1" {
		t.Fatalf("ruled-table-unplaceable: err %v, want TABLE_MIN_HEIGHT_UNPLACEABLE on e1", err)
	}
}

// guideTwins are the guide's two published forms: the Markdown source and the
// HTML page the designer bundles. Both must carry every example and name every
// export, so neither can fall behind the other.
var guideTwins = []string{"rendering-library.md", "rendering-library.html"}

func readGuide(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(docsDir(t), name))
	if err != nil {
		t.Fatalf("read guide %s: %v", name, err)
	}
	if len(b) == 0 {
		t.Fatalf("guide %s is empty", name)
	}
	return string(b)
}

var htmlTag = regexp.MustCompile(`<[^>]*>`)

// guideText is what a reader sees: the Markdown as written, or the HTML page
// with its tags removed and its entities decoded.
func guideText(t *testing.T, name string) string {
	t.Helper()
	text := readGuide(t, name)
	if strings.HasSuffix(name, ".html") {
		text = html.UnescapeString(htmlTag.ReplaceAllString(text, ""))
	}
	return text
}

// guideWords is guideText for word matching: each HTML tag becomes a space, so
// adjacent table cells such as <td>UTCOffset</td><td>string</td> stay separate
// words. guideText joins them without one, which keeps code blocks verbatim.
func guideWords(t *testing.T, name string) string {
	t.Helper()
	text := readGuide(t, name)
	if strings.HasSuffix(name, ".html") {
		text = html.UnescapeString(htmlTag.ReplaceAllString(text, " "))
	}
	return text
}

func TestDocsGuideEmbedsEveryExampleVerbatim(t *testing.T) {
	var files []string
	err := filepath.WalkDir(filepath.Join(docsDir(t), "examples"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 20 {
		t.Fatalf("found only %d example files", len(files))
	}
	markdown := guideText(t, "rendering-library.md")
	page := guideText(t, "rendering-library.html")
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		content := strings.TrimRight(string(b), "\n")
		rel, _ := filepath.Rel(docsDir(t), path)
		if !strings.Contains(markdown, "\n"+content+"\n```") {
			t.Errorf("docs/rendering-library.md does not embed %s verbatim in a fenced block", rel)
		}
		if !strings.Contains(page, content) {
			t.Errorf("docs/rendering-library.html does not embed %s verbatim", rel)
		}
	}
}

// exportedIdentifiers lists every exported declaration of the package in dir:
// functions, types, constants, variables, methods and struct fields.
func exportedIdentifiers(t *testing.T, dir string) []string {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", dir, err)
	}
	seen := map[string]bool{}
	for name, pkg := range pkgs {
		if strings.HasSuffix(name, "_test") {
			continue
		}
		p := doc.New(pkg, "example", 0)
		values := func(vs []*doc.Value) {
			for _, v := range vs {
				for _, n := range v.Names {
					seen[n] = true
				}
			}
		}
		funcs := func(fs []*doc.Func) {
			for _, f := range fs {
				seen[f.Name] = true
			}
		}
		values(p.Consts)
		values(p.Vars)
		funcs(p.Funcs)
		for _, typ := range p.Types {
			seen[typ.Name] = true
			values(typ.Consts)
			values(typ.Vars)
			funcs(typ.Funcs)
			funcs(typ.Methods)
			for _, spec := range typ.Decl.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if st, ok := ts.Type.(*ast.StructType); ok {
					for _, field := range st.Fields.List {
						for _, n := range field.Names {
							if n.IsExported() {
								seen[n.Name] = true
							}
						}
					}
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func TestDocsGuideNamesEveryExportedIdentifier(t *testing.T) {
	root := filepath.Join(repoRootFromTest(t), "folio8-go")
	total := 0
	for _, pkg := range []string{".", "fonts"} {
		ids := exportedIdentifiers(t, filepath.Join(root, pkg))
		if len(ids) == 0 {
			t.Fatalf("package %s exports nothing — the census read the wrong directory", pkg)
		}
		total += len(ids)
		for _, name := range guideTwins {
			guide := guideWords(t, name)
			for _, id := range ids {
				if !regexp.MustCompile(`\b` + regexp.QuoteMeta(id) + `\b`).MatchString(guide) {
					t.Errorf("docs/%s does not document exported identifier %s (package %s)", name, id, pkg)
				}
			}
		}
	}
	if total < 60 {
		t.Fatalf("census found only %d identifiers", total)
	}
}

// TestDocsGuideProgramsRunAgainstTheWorkingTree compiles and runs the guide's
// two programs from a consumer module that replaces folio8-go with this
// checkout, in a directory holding the first-PDF example files.
func TestDocsGuideProgramsRunAgainstTheWorkingTree(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a separate module")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go command not available")
	}
	root := repoRootFromTest(t)
	for _, program := range []string{"first-pdf", "render-to"} {
		t.Run(program, func(t *testing.T) {
			work := t.TempDir()
			gomod := "module example.com/folio8-demo\n\ngo 1.25.0\n\nrequire github.com/panitw/folio8/folio8-go v0.0.0\n\nreplace github.com/panitw/folio8/folio8-go => " + filepath.Join(root, "folio8-go") + "\n"
			sum, err := os.ReadFile(filepath.Join(root, "folio8-go", "go.sum"))
			if err != nil {
				t.Fatal(err)
			}
			files := map[string]string{
				"go.mod":              gomod,
				"go.sum":              string(sum),
				"main.go":             readDocsExample(t, filepath.Join(program, "main.go")),
				"first-pdf.folio":     readDocsExample(t, "first-pdf.folio"),
				"first-pdf.data.json": readDocsExample(t, "first-pdf.data.json"),
			}
			for name, content := range files {
				if err := os.WriteFile(filepath.Join(work, name), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command(goBin, "run", ".")
			cmd.Dir = work
			cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off", "GOWORK=off")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go run %s: %v\n%s", program, err, out)
			}
			switch program {
			case "first-pdf":
				pdf, err := os.ReadFile(filepath.Join(work, "first-pdf.pdf"))
				if err != nil || !bytes.HasPrefix(pdf, []byte("%PDF-")) {
					t.Fatalf("first-pdf wrote no PDF: %v\n%s", err, out)
				}
			case "render-to":
				for _, want := range []string{"validate: 0 warnings", "bytes equal: true", "failing writer: folio8: RenderTo: write failed after 0 of", "is a RenderError: false"} {
					if !strings.Contains(string(out), want) {
						t.Fatalf("render-to output lacks %q:\n%s", want, out)
					}
				}
			}
		})
	}
}
