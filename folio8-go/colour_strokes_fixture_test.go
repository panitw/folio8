package folio8

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio8-go/internal/geom"
	"github.com/panitw/folio8/folio8-go/internal/pagemodel"
	"github.com/panitw/folio8/folio8-go/internal/template"
)

const colourStrokesFixtureDir = "colour-strokes"

// colourStrokesFills and colourStrokesStrokes are the colours the document
// declares, by the PDF operator that must paint each: `rg` for a fill or a
// text ink, `RG` for a stroke. Every one differs from #000000.
var (
	colourStrokesFills   = []string{"#1B2A4A", "#FFF4D6", "#6A1B9A", "#37474F", "#FFFFFF", "#E3F2FD"}
	colourStrokesStrokes = []string{"#C81E1E", "#2E7D32", "#1565C0", "#8E24AA", "#00838F"}
)

func renderColourStrokes(t *testing.T) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(colourStrokesTemplateJSON))
	if err != nil {
		t.Fatalf("parse colour-strokes fixture: %v", err)
	}
	res, err := Render(tpl, Data(colourStrokesDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render colour-strokes fixture: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Fatalf("the colour-strokes fixture must render with NO diagnostics; got %+v", res.Diagnostics)
	}
	return res.Bytes
}

var colourOperatorRE = regexp.MustCompile(`(?m)^(\S+) (\S+) (\S+) (rg|RG)$`)

// colourStrokesAssertColours is the per-leg feature guard, read off the PDF
// bytes: one page, and every declared colour set by the operator that paints
// it — and no colour operator setting black, which would mean a declared
// colour fell back to the default somewhere.
func colourStrokesAssertColours(t *testing.T, raw []byte, fail func(string, ...any)) {
	t.Helper()
	streams := splitPageContentStreams(t, raw)
	if len(streams) != 1 {
		fail("the PDF has %d pages, want 1", len(streams))
		return
	}
	seen := map[string]bool{}
	for _, m := range colourOperatorRE.FindAllStringSubmatch(streams[0], -1) {
		var ch [3]int64
		for i := 0; i < 3; i++ {
			v, ok := colourOperandThousandths(m[1+i])
			if !ok {
				fail("colour operand %q is not a decimal of at most three places", m[1+i])
				return
			}
			ch[i] = v
		}
		key := m[4] + " " + strconv.FormatInt(ch[0], 10) + " " + strconv.FormatInt(ch[1], 10) + " " + strconv.FormatInt(ch[2], 10)
		seen[key] = true
		if ch == [3]int64{} {
			fail("a %s operator sets black; every colour this document declares differs from #000000", m[4])
		}
	}
	want := func(op, hex string) string {
		c, ok := parseHexColor(hex)
		if !ok {
			fail("test colour %s is not #RRGGBB", hex)
			return ""
		}
		return op + " " + strconv.FormatInt(int64(geom.ScaleRound(geom.Length(c.R), 1000, 255)), 10) + " " +
			strconv.FormatInt(int64(geom.ScaleRound(geom.Length(c.G), 1000, 255)), 10) + " " +
			strconv.FormatInt(int64(geom.ScaleRound(geom.Length(c.B), 1000, 255)), 10)
	}
	for _, hex := range colourStrokesFills {
		if key := want("rg", hex); key == "" || !seen[key] {
			fail("no rg operator paints %s", hex)
		}
	}
	for _, hex := range colourStrokesStrokes {
		if key := want("RG", hex); key == "" || !seen[key] {
			fail("no RG operator strokes %s", hex)
		}
	}
}

// colourOperandThousandths reads a content-stream colour operand — the
// emitter's exact decimal spelling of a thousandths count, such as "0.106",
// "0.29" or "1" — as that count, with no floating point (AD-23).
func colourOperandThousandths(s string) (int64, bool) {
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" || len(frac) > 3 {
		return 0, false
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil || w < 0 {
		return 0, false
	}
	f := int64(0)
	if frac != "" {
		f, err = strconv.ParseInt(frac+strings.Repeat("0", 3-len(frac)), 10, 64)
		if err != nil {
			return 0, false
		}
	}
	return w*1000 + f, true
}

func TestColourStrokesGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", colourStrokesFixtureDir)
	for _, c := range []struct{ file, want string }{
		{"input.folio", colourStrokesTemplateJSON},
		{"data.json", colourStrokesDataJSON},
	} {
		got, err := os.ReadFile(filepath.Join(dir, c.file))
		if err != nil {
			t.Fatalf("read %s: %v", c.file, err)
		}
		if string(got) != c.want {
			t.Fatalf("fixtures/%s/%s has drifted from its Go constant (colour_strokes_template.go)", colourStrokesFixtureDir, c.file)
		}
	}
	tpl, err := ParseTemplate([]byte(colourStrokesTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := template.SerializeDocument(tpl.doc)
	if err != nil {
		t.Fatal(err)
	}
	if string(saved) != colourStrokesTemplateJSON {
		t.Fatalf("input.folio does not save back byte-for-byte:\n%s", saved)
	}
	if !strings.Contains(colourStrokesTemplateJSON, `"version": "3.1"`) {
		t.Error("the fixture must declare format version 3.1 (it declares table rules)")
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
	if got := sha256Hex(renderColourStrokes(t)); got != fixture.SHA256 {
		t.Fatalf("golden fixture mismatch: got sha256 %s, want %s (fixtures/%s). Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change. Do not regenerate the fixture to make this pass.", got, fixture.SHA256, colourStrokesFixtureDir)
	}
}

// TestColourStrokesSemanticAcceptance checks what the golden declares against
// the page model: the heading's partial border, the inks, and the table's
// alternating fills.
func TestColourStrokesSemanticAcceptance(t *testing.T) {
	tpl, err := ParseTemplate([]byte(colourStrokesTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	pages, diags := barcodePages(t, tpl, colourStrokesDataJSON)
	if len(diags) != 0 || len(pages) != 1 {
		t.Fatalf("pages %d diags %+v, want 1 page and no diagnostics", len(pages), diags)
	}
	page := pages[0]
	colour := func(hex string) pagemodel.Color {
		c, ok := parseHexColor(hex)
		if !ok {
			t.Fatalf("test colour %s is not #RRGGBB", hex)
		}
		return c
	}

	// e1: a filled box stroked on its bottom and left edges only.
	var heading []pagemodel.Rect
	for _, r := range page.Rects {
		if r.HasStroke && r.Stroke == colour("#C81E1E") {
			heading = append(heading, r)
		}
	}
	if len(heading) != 1 {
		t.Fatalf("%d rects stroke #C81E1E, want the heading's one", len(heading))
	}
	h := heading[0]
	if !h.HasFill || h.Fill != colour("#FFF4D6") {
		t.Errorf("the heading box fill is %+v (HasFill %v), want #FFF4D6", h.Fill, h.HasFill)
	}
	if !h.Edges.Bottom || !h.Edges.Left || h.Edges.Top || h.Edges.Right {
		t.Errorf("the heading box strokes edges %+v, want bottom and left only", h.Edges)
	}

	// Inks: the heading, the note, the header row and the cells.
	inks := map[string]pagemodel.Color{
		"Colour and strokes": colour("#1B2A4A"),
		"Item":               colour("#FFFFFF"),
		"Colour":             colour("#FFFFFF"),
		"Qty":                colour("#FFFFFF"),
		"Navy heading":       colour("#37474F"),
	}
	noteFound := false
	for _, run := range page.Runs {
		if want, ok := inks[run.SourceText]; ok {
			if !run.HasColor || run.Color != want {
				t.Errorf("run %q ink %+v (HasColor %v), want %+v", run.SourceText, run.Color, run.HasColor, want)
			}
			delete(inks, run.SourceText)
		}
		if strings.HasPrefix(run.SourceText, "Above:") {
			noteFound = true
			if !run.HasColor || run.Color != colour("#6A1B9A") {
				t.Errorf("the note's ink is %+v, want #6A1B9A", run.Color)
			}
		}
	}
	if !noteFound {
		t.Error("the note run (\"Above: …\") was not found")
	}
	if len(inks) != 0 {
		t.Errorf("runs not found: %v", inks)
	}

	// The rect and the line are stroked; six rows put three alternate fills
	// on the page; the rules are drawn.
	count := func(pred func(pagemodel.Rect) bool) int {
		n := 0
		for _, r := range page.Rects {
			if pred(r) {
				n++
			}
		}
		return n
	}
	for _, hex := range []string{"#2E7D32", "#1565C0", "#00838F"} {
		if count(func(r pagemodel.Rect) bool { return r.HasStroke && r.Stroke == colour(hex) }) != 1 {
			t.Errorf("want exactly one rect stroked %s", hex)
		}
	}
	if n := count(func(r pagemodel.Rect) bool { return r.HasFill && r.Fill == colour("#E3F2FD") }); n != 9 {
		t.Errorf("%d cells filled #E3F2FD, want 9 (three columns on collection indexes 1, 3 and 5)", n)
	}
	// Three header cells filled #1B2A4A: the heading's #1B2A4A is ink on
	// a run, not a rect fill, so only the header row can satisfy this.
	if n := count(func(r pagemodel.Rect) bool { return r.HasFill && r.Fill == colour("#1B2A4A") }); n != 3 {
		t.Errorf("%d rects filled #1B2A4A, want 3 (the header row's three cells)", n)
	}
	// Eight rules: two column boundaries, and six row boundaries (header
	// and the six rows, with none on the table's bottom edge).
	if n := count(func(r pagemodel.Rect) bool { return r.HasStroke && r.Stroke == colour("#8E24AA") }); n != 8 {
		t.Errorf("%d rules stroked #8E24AA, want 8", n)
	}

	colourStrokesAssertColours(t, renderColourStrokes(t), t.Errorf)
}

// TestColourStrokesRendersIdenticallyInAFreshProcess renders the golden in a
// fresh OS process and compares it with this process's render.
func TestColourStrokesRendersIdenticallyInAFreshProcess(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), subprocessColourStrokesEnvVar+"=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("subprocess render failed: %v\n%s", err, stderr.String())
	}
	if here := renderColourStrokes(t); !bytes.Equal(stdout.Bytes(), here) {
		off, window := firstDivergence(stdout.Bytes(), here)
		t.Fatalf("fresh-process render differs at byte %d: %s", off, window)
	}
}
