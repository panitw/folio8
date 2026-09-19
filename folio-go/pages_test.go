package folio8

import (
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/designer"
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// SPEC-multi-pages story 1: the engine half of designed pages — render order,
// the summed page count, an empty page, the projection's window pages and
// page-located load errors.

func multiPageTemplate(t *testing.T, src string) *Template {
	t.Helper()
	tpl, err := ParseTemplate([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return tpl
}

func replaceMultiPage(t *testing.T, old, new string) string {
	t.Helper()
	if !strings.Contains(multiPageStatementTemplateJSON, old) {
		t.Fatalf("fixture precondition: %q not found", old)
	}
	return strings.Replace(multiPageStatementTemplateJSON, old, new, 1)
}

// editMultiPage returns the multi-page fixture's canonical source after edit.
// New elements may use ids from ei upward: nextId is raised past them.
func editMultiPage(t *testing.T, edit func(d *template.Document)) string {
	t.Helper()
	d, err := template.ParseDocument([]byte(multiPageStatementTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	d.NextID = 40
	edit(d)
	out, err := template.SerializeDocument(d)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func pts(v int64) template.Presence[geom.Length] {
	return template.Presence[geom.Length]{Set: true, Value: geom.Length(v * 1000)}
}

func multiPageElement(d *template.Document, page int, id string) *template.Element {
	for i := range d.Pages[page].Elements {
		if string(d.Pages[page].Elements[i].ID) == id {
			return &d.Pages[page].Elements[i]
		}
	}
	panic("fixture precondition: no element " + id + " on page " + strconv.Itoa(page))
}

// multiPageBox is a styled rect, a column item on the render and canvas paths.
func multiPageBox(id string, y, height int64) template.Element {
	return template.Element{ID: template.ElementID(id), Type: template.ElementRect, Y: geom.Length(y * 1000), Width: pts(200), Height: pts(height),
		Style: template.Presence[template.Style]{Set: true, Value: template.Style{Background: template.Presence[string]{Set: true, Value: "#000000"}}}}
}

func multiPageText(id string, y int64, value string) template.Element {
	return template.Element{ID: template.ElementID(id), Type: template.ElementText, Y: geom.Length(y * 1000), Width: pts(300), Height: pts(14),
		Value: template.Presence[string]{Set: true, Value: value},
		Style: template.Presence[template.Style]{Set: true, Value: template.Style{FontFamily: template.Presence[string]{Set: true, Value: "body"}, FontSize: pts(10)}}}
}

func shippedProjection(t *testing.T, tpl *Template) designer.CanvasProjection {
	t.Helper()
	projection, err := canvasWithTextPaint(tpl, testShippedFontSet())
	if err != nil {
		t.Fatalf("CanvasWithTextPaint: %v", err)
	}
	return projection
}

const multiPageLastPageEnd = `"pageBreak": true
    }
  ],`

func TestAnEmptyDesignedPageIsOneHeaderAndFooterOnlyPage(t *testing.T) {
	tpl := multiPageTemplate(t, replaceMultiPage(t, multiPageLastPageEnd, `"pageBreak": true
    },
    {
      "elements": [],
      "pageBreak": true
    }
  ],`))
	res, err := Render(tpl, Data(multiPageStatementDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatal(err)
	}
	pages := statementPageRuns(t, res.Bytes)
	if len(pages) != 5 {
		t.Fatalf("%d pages, want 5", len(pages))
	}
	var texts []string
	for _, run := range pages[4] {
		texts = append(texts, run.Text)
	}
	want := []string{"STATEMENT OF ACCOUNT", "Folio Example Bank (synthetic)", "Synthetic sample data - not a real account", "Page 5 of 5"}
	if !reflect.DeepEqual(texts, want) {
		t.Fatalf("the empty page draws %q, want only the header and footer %q", texts, want)
	}
	for i := range pages {
		found := false
		for _, run := range pages[i] {
			found = found || run.Text == fmt.Sprintf("Page %d of 5", i+1)
		}
		if !found {
			t.Errorf("page %d does not read Page %d of 5", i+1, i+1)
		}
	}
}

// SPEC-multi-pages story 5 (CAP-9): Page Break off is honoured. The fixture's
// page 2 is 128pt tall and page 1's rows leave room for it on output page 3,
// so it joins that page and the PDF has three pages.
func TestAPageBreakOffPageFollowsThePreviousPagesContent(t *testing.T) {
	tpl := multiPageTemplate(t, replaceMultiPage(t, `"pageBreak": true`, `"pageBreak": false`))
	if tpl.doc.Pages[1].PageBreak {
		t.Fatal("precondition: page 2 loaded with Page Break on")
	}
	pages, diags := barcodePages(t, tpl, multiPageStatementDataJSON)
	onPages, _ := headingBaseline(pages, multiPageStatementPage2Heading)
	if len(diags) != 0 || len(pages) != 3 || len(onPages) != 1 || onPages[0] != 2 {
		t.Fatalf("pages %d, page 2's heading on %v, diags %+v; want 3 pages with the heading on page 3", len(pages), onPages, diags)
	}
}

func TestTheCanvasProjectsEachWindowsDesignedPage(t *testing.T) {
	// No data: page 1's table is one header tall, so each page is one window.
	projection := shippedProjection(t, multiPageTemplate(t, multiPageStatementTemplateJSON))
	if projection.ContentWindowCount != 2 || !reflect.DeepEqual(projection.ContentWindowOrigins, []int64{0, 0}) || !reflect.DeepEqual(projection.ContentWindowPages, []int{0, 1}) {
		t.Fatalf("count %d origins %v pages %v, want 2 [0 0] [0 1]", projection.ContentWindowCount, projection.ContentWindowOrigins, projection.ContentWindowPages)
	}
	if projection.ContentWindowCountIsExact {
		t.Error("page 1 has a bound table, so the count cannot be exact")
	}

	// A page-1 element far down the column gives page 1 a second window; the
	// origins stay page-local, so page 2's window still begins at 0.
	far := multiPageTemplate(t, editMultiPage(t, func(d *template.Document) {
		d.Pages[0].Elements = append(d.Pages[0].Elements, multiPageText("ei", 900, "Far below"))
	}))
	projection = shippedProjection(t, far)
	origins, pages := projection.ContentWindowOrigins, projection.ContentWindowPages
	if projection.ContentWindowCount != 3 || len(origins) != 3 || !reflect.DeepEqual(pages, []int{0, 0, 1}) || origins[0] != 0 || origins[1] <= 0 || origins[2] != 0 {
		t.Fatalf("count %d origins %v pages %v, want 3 windows [0 >0 0] on pages [0 0 1]", projection.ContentWindowCount, origins, pages)
	}

	// A page-2 element taller than one window degrades the count to one
	// window per page.
	tall := multiPageTemplate(t, editMultiPage(t, func(d *template.Document) {
		d.Pages[1].Elements = append(d.Pages[1].Elements, multiPageBox("ei", 0, 700))
	}))
	projection = shippedProjection(t, tall)
	if projection.ContentWindowCount != 2 || !reflect.DeepEqual(projection.ContentWindowOrigins, []int64{0, 0}) || !reflect.DeepEqual(projection.ContentWindowPages, []int{0, 1}) || projection.ContentWindowCountIsExact {
		t.Fatalf("degraded: count %d origins %v pages %v exact %v, want 2 [0 0] [0 1] false", projection.ContentWindowCount, projection.ContentWindowOrigins, projection.ContentWindowPages, projection.ContentWindowCountIsExact)
	}

	// A one-page document projects all zeros, one per window.
	one := projectWithPaint(t, parseWindowCountTemplate(t, canvasWindowCountControlTemplateJSON))
	if len(one.ContentWindowPages) != int(one.ContentWindowCount) || one.ContentWindowCount < 2 {
		t.Fatalf("one-page control: count %d pages %v", one.ContentWindowCount, one.ContentWindowPages)
	}
	for _, page := range one.ContentWindowPages {
		if page != 0 {
			t.Fatalf("one-page control projects pages %v, want all zeros", one.ContentWindowPages)
		}
	}
	// The zero projection (no paint) is one window per page too.
	if bare, err := canvas(multiPageTemplate(t, multiPageStatementTemplateJSON)); err != nil || !reflect.DeepEqual(bare.ContentWindowPages, []int{0, 1}) || !reflect.DeepEqual(bare.ContentWindowOrigins, []int64{0, 0}) {
		t.Fatalf("Canvas: pages %v origins %v err %v", bare.ContentWindowPages, bare.ContentWindowOrigins, err)
	}
}

func TestPageLoadErrorsNameThePage(t *testing.T) {
	src := multiPageStatementTemplateJSON
	onePage := src[:strings.Index(src, `"pages": [`)] + `"pages": [{"elements": []}],
  "utcOffset": "+07:00",
  "version": "4.1"
}
`
	for _, c := range []struct {
		name, src, code, path, reason string
	}{
		{"one page", onePage, DiagCodePagesInvalid, "pages", "has 1 entries"},
		{"first-page break out of range", replaceMultiPage(t, `"pages": [
    {
      "elements": [`, `"pages": [
    {
      "sectionBreak": 0,
      "elements": [`), DiagCodeSectionBreakInvalid, "pages[0].sectionBreak", ""},
		// Story 5: a later page's break is checked against that page alone.
		{"later-page break out of range", replaceMultiPage(t, `"pageBreak": true`, `"pageBreak": true, "sectionBreak": 5000`), DiagCodeSectionBreakInvalid, "pages[1].sectionBreak", "at or below the bottom"},
		// ee runs 24–38pt, across a page-2 break at 30.
		{"later-page straddle", replaceMultiPage(t, `"pageBreak": true`, `"pageBreak": true, "sectionBreak": 30`), DiagCodeSectionBreakStraddled, "pages[1].sectionBreak", "element ee"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseTemplate([]byte(c.src))
			var re *RenderError
			if !errors.As(err, &re) {
				t.Fatalf("got %v, want a *RenderError", err)
			}
			if re.Diagnostic.Code != c.code || re.Diagnostic.DataPath != c.path {
				t.Fatalf("code %q path %q, want %q at %q (%v)", re.Diagnostic.Code, re.Diagnostic.DataPath, c.code, c.path, err)
			}
			if !strings.Contains(err.Error(), c.reason) {
				t.Fatalf("error %q does not say %q", err, c.reason)
			}
		})
	}
}

func TestCommandsThatCreateIntoContentTargetThePageOne(t *testing.T) {
	tpl := multiPageTemplate(t, multiPageStatementTemplateJSON)
	band, _, err := bandByName(tpl, bandContent)
	if err != nil {
		t.Fatal(err)
	}
	if band != &tpl.doc.Pages[0].Band {
		t.Fatal("the content band a create targets is not page 1's")
	}
	found, _, _, element, err := findComponent(tpl, "eh")
	if err != nil {
		t.Fatal(err)
	}
	if found != &tpl.doc.Pages[1].Band || string(element.ID) != "eh" {
		t.Fatal("findComponent did not find a page-2 element on page 2's band")
	}
	if len(contentElements(tpl)) != len(tpl.doc.Pages[0].Elements)+len(tpl.doc.Pages[1].Elements) {
		t.Fatal("contentElements does not see every page")
	}
}

func TestPageOnesSectionBreakDoesNotConstrainALaterPage(t *testing.T) {
	tpl := multiPageTemplate(t, editMultiPage(t, func(d *template.Document) {
		d.Pages[0].SectionBreak = pts(300)
	}))
	// ee is 14pt tall, so at y 295 it crosses 300.
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"moveComponent","version":1,"id":"ee","x":0,"y":295,"snap":false}`)); err != nil {
		t.Fatalf("a page-2 element moved across page 1's break offset was refused: %v", err)
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"moveComponent","version":1,"id":"e6","x":0,"y":295,"snap":false}`)); err == nil {
		t.Fatal("control: a page-1 element moved across its own break was accepted")
	}
}

func TestSectionBreakCommandsOnAMultiPageDocumentUsePageOne(t *testing.T) {
	tpl := multiPageTemplate(t, multiPageStatementTemplateJSON)
	var failure *designer.ComponentCommandError
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"removeSectionBreak","version":1}`)); !errors.As(err, &failure) || failure.DataPath != "pages[0].sectionBreak" {
		t.Fatalf("refusal %v, want one located at pages[0].sectionBreak", err)
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"setSectionBreak","version":1,"offset":300,"snap":false}`)); err != nil {
		t.Fatal(err)
	}
	saved, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	d, err := template.ParseDocument(saved)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Pages[0].SectionBreak.Set || d.Pages[0].SectionBreak.Value != 300000 || d.Bands.Content.SectionBreak.Set || len(d.Bands.Content.Elements) != 0 {
		t.Fatalf("saved break pages[0] %+v, bands.content %+v", d.Pages[0].SectionBreak, d.Bands.Content)
	}
}

func TestPageOnesSectionBreakRendersAsItDoesAlone(t *testing.T) {
	const note = "Closing note"
	multi := multiPageTemplate(t, editMultiPage(t, func(d *template.Document) {
		d.Pages[0].SectionBreak = pts(600)
		d.Pages[0].Elements = append(d.Pages[0].Elements, multiPageText("ei", 620, note))
	}))
	d, err := template.ParseDocument([]byte(editMultiPage(t, func(d *template.Document) {
		d.Pages[0].SectionBreak = pts(600)
		d.Pages[0].Elements = append(d.Pages[0].Elements, multiPageText("ei", 620, note))
	})))
	if err != nil {
		t.Fatal(err)
	}
	d.Bands.Content = d.Pages[0].Band
	d.Pages = nil
	aloneSrc, err := template.SerializeDocument(d)
	if err != nil {
		t.Fatal(err)
	}
	alone := multiPageTemplate(t, string(aloneSrc))

	multiPages, _ := barcodePages(t, multi, multiPageStatementDataJSON)
	alonePages, _ := barcodePages(t, alone, multiPageStatementDataJSON)
	multiOn, multiY := headingBaseline(multiPages, note)
	aloneOn, aloneY := headingBaseline(alonePages, note)
	if len(aloneOn) != 1 || !reflect.DeepEqual(multiOn, aloneOn) || multiY != aloneY {
		t.Fatalf("below-break note on pages %v at %d, alone on %v at %d", multiOn, multiY, aloneOn, aloneY)
	}
	if headingOn, _ := headingBaseline(multiPages, multiPageStatementPage2Heading); len(multiPages) != len(alonePages)+1 || !reflect.DeepEqual(headingOn, []int{len(alonePages)}) {
		t.Fatalf("%d pages with page 2's heading on %v, want %d pages with it on the last", len(multiPages), headingOn, len(alonePages)+1)
	}

	projection := shippedProjection(t, multi)
	pageOf := contentPageIndex(multi)
	for _, component := range projection.Components {
		if component.Band != bandContent {
			continue
		}
		if pageOf[component.ID] == 1 && component.BelowSectionBreak != nil {
			t.Errorf("page-2 component %s carries belowSectionBreak", component.ID)
		}
		if component.ID == "ei" && (component.BelowSectionBreak == nil || !*component.BelowSectionBreak) {
			t.Error("page 1's below-break note is not marked below the break")
		}
	}
}

func TestAssetAndFontChainWalksSeeALaterPage(t *testing.T) {
	shared := png1x1Gray()
	key := sha256Hex(shared)
	image := func(id string, y int64) template.Element {
		return template.Element{ID: template.ElementID(id), Type: template.ElementImage, Y: geom.Length(y * 1000), Width: pts(72), Height: pts(24), Asset: template.Presence[string]{Set: true, Value: key}}
	}
	tpl := multiPageTemplate(t, editMultiPage(t, func(d *template.Document) {
		d.Assets[key] = template.Asset{Data: []string{base64.StdEncoding.EncodeToString(shared)}, MediaType: "image/png"}
		d.Pages[0].Elements = append(d.Pages[0].Elements, image("ei", 400))
		d.Pages[1].Elements = append(d.Pages[1].Elements, image("ej", 200))
		d.Fonts["terms"] = []template.FontChainEntry{template.FaceEntry("Noto Sans")}
		ee := multiPageElement(d, 1, "ee")
		style := ee.Style.Value
		style.FontFamily = template.Presence[string]{Set: true, Value: "terms"}
		ee.Style.Value = style
	}))

	if _, err := applyComponentCommand(tpl, setAssetCommand("ei", "image/png", png3x2RGB)); err != nil {
		t.Fatal(err)
	}
	if _, ok := tpl.doc.Assets[key]; !ok {
		t.Fatal("replacing page 1's image dropped the asset page 2's image still names")
	}

	fontChainAccepted(t, tpl, `{"kind":"renameFontChain","version":1,"name":"terms","to":"legal"}`)
	if _, _, _, ee, err := findComponent(tpl, "ee"); err != nil || ee.Style.Value.FontFamily.Value != "legal" {
		t.Fatalf("renaming the chain did not update page 2's element (err %v)", err)
	}
	if failure := fontChainRefusal(t, tpl, `{"kind":"deleteFontChain","version":1,"name":"legal"}`); failure.Message != `font chain "legal" is still named by ee` {
		t.Fatalf("delete refusal = %q", failure.Message)
	}
}

// pageOneTableCopy is page 1's table e7 under a new id, keeping only the
// description column, renamed to columnID.
func pageOneTableCopy(d *template.Document, id, columnID string) template.Element {
	el := *multiPageElement(d, 0, "e7")
	ext := el.Table.Value
	column := ext.Columns[2]
	column.ID = template.ElementID(columnID)
	ext.Columns = []template.Column{column}
	el.Table.Value = ext
	el.ID = template.ElementID(id)
	return el
}

func TestReadWalksSeeALaterPage(t *testing.T) {
	setValue := func(value string) func(*template.Document) {
		return func(d *template.Document) {
			multiPageElement(d, 1, "ee").Value = template.Presence[string]{Set: true, Value: value}
		}
	}
	for _, c := range []struct {
		name  string
		edit  func(*template.Document)
		check func(t *testing.T, src string)
	}{
		{"parameter references", setValue("Branch {{params.branch}}"), func(t *testing.T, src string) {
			refs, err := ParameterReferences(multiPageTemplate(t, src))
			if err != nil || !reflect.DeepEqual(refs, []string{"branch"}) {
				t.Fatalf("references %v err %v, want [branch]", refs, err)
			}
		}},
		{"stand-in data", setValue("Version {{terms.version}}"), func(t *testing.T, src string) {
			data, err := standInData(multiPageTemplate(t, src))
			if err != nil || !strings.Contains(string(data), `"terms"`) {
				t.Fatalf("stand-in data %s err %v, want a terms path", data, err)
			}
		}},
		{"invalid expression", setValue("{{formatNumber(}}"), func(t *testing.T, src string) {
			if _, err := ParseTemplate([]byte(src)); err == nil {
				t.Fatal("an invalid expression on page 2 loaded")
			}
		}},
		{"too-tall minHeight", func(d *template.Document) {
			table := pageOneTableCopy(d, "ei", "ej")
			ext := table.Table.Value
			ext.MinHeight = pts(10000)
			table.Table.Value = ext
			d.Pages[1].Elements = append(d.Pages[1].Elements, table)
		}, func(t *testing.T, src string) {
			_, err := ParseTemplate([]byte(src))
			var re *RenderError
			if !errors.As(err, &re) || re.Diagnostic.Code != DiagCodeTableMinHeightUnplaceable || re.Diagnostic.ElementID != "ei" {
				t.Fatalf("got %v, want TABLE_MIN_HEIGHT_UNPLACEABLE naming ei", err)
			}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			c.check(t, editMultiPage(t, c.edit))
		})
	}
}

// A clipped row on a later page is reported with its output page number.
func TestAClippedRowOnALaterPageNamesTheOutputPage(t *testing.T) {
	tpl := multiPageTemplate(t, editMultiPage(t, func(d *template.Document) {
		table := pageOneTableCopy(d, "ei", "ej")
		table.Y = geom.Length(150000)
		ext := table.Table.Value
		ext.Bind = "notes[]"
		ext.HeaderStyle = template.Presence[template.Style]{Set: true, Value: template.Style{FontSize: pts(7)}}
		table.Table.Value = ext
		style := table.Style.Value
		style.FontSize = pts(700)
		table.Style.Value = style
		d.Pages[1].Elements = append(d.Pages[1].Elements, table)
	}))
	data := `{"notes":[{"description":"x"}],` + strings.TrimPrefix(multiPageStatementDataJSON, "{")
	res, err := Render(tpl, Data(data), nil, testShippedFontSet())
	if err != nil {
		t.Fatal(err)
	}
	pages := statementPageRuns(t, res.Bytes)
	found := false
	for _, diag := range res.Diagnostics {
		if diag.Code != DiagCodeTableRowClippedHeight {
			continue
		}
		found = true
		m := regexp.MustCompile(`on page (\d+)`).FindStringSubmatch(diag.Message)
		if m == nil {
			t.Fatalf("clip warning names no page: %s", diag.Message)
		}
		if n, _ := strconv.Atoi(m[1]); n < 4 || n > len(pages) {
			t.Fatalf("clip warning names page %d; page 2 starts on output page 4 of %d", n, len(pages))
		}
	}
	if !found {
		t.Fatalf("no clip warning: %+v", res.Diagnostics)
	}
}
