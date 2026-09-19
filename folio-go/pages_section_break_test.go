package folio8

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/layout"
	"github.com/panitw/folio8/folio-go/internal/pagemodel"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// SPEC-multi-pages story 5: a section break on every designed page (CAP-6)
// and Page Break off (CAP-9). The multi-page-statement fixture's page 2 holds
// ed (0–18pt), ee (24–38), ef (40–54), eg (110–111) and eh (114–128).

func TestALaterPagesBreakLoadsAndRoundTrips(t *testing.T) {
	src := editMultiPage(t, func(d *template.Document) {
		d.Pages[1].SectionBreak = pts(60)
		d.Pages[1].SectionBreakAnchor = template.Presence[bool]{Set: true, Value: false}
	})
	tpl := multiPageTemplate(t, src)
	if offset, ok := declaredSectionBreak(tpl, 1); !ok || offset != 60000 || sectionBreakAnchored(tpl, 1) {
		t.Fatalf("page 2's break %d %v anchored %v", offset, ok, sectionBreakAnchored(tpl, 1))
	}
	if _, ok := declaredSectionBreak(tpl, 0); ok {
		t.Fatal("page 1 gained a break")
	}
	saved, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if string(saved) != src {
		t.Fatalf("a later-page break does not round-trip byte-identically:\n%s", saved)
	}
	if !strings.Contains(src, `"sectionBreak": 60`) {
		t.Fatal("precondition: the source declares pages[1].sectionBreak")
	}
}

func TestSectionBreakCommandsTakeAnOptionalPage(t *testing.T) {
	tpl := multiPageTemplate(t, multiPageStatementTemplateJSON)
	before := reparse(t, tpl)

	applyPageCommand(t, tpl, `{"kind":"setSectionBreak","version":1,"offset":60,"snap":false,"page":1}`)
	after := reparse(t, tpl)
	if !after.Pages[1].SectionBreak.Set || after.Pages[1].SectionBreak.Value != 60000 {
		t.Fatalf("page 2's break %+v, want 60pt", after.Pages[1].SectionBreak)
	}
	if !reflect.DeepEqual(before.Pages[0], after.Pages[0]) {
		t.Fatal("setting page 2's break changed page 1")
	}

	applyPageCommand(t, tpl, `{"kind":"setSectionBreakAnchor","version":1,"anchor":false,"page":1}`)
	if d := reparse(t, tpl); !d.Pages[1].SectionBreakAnchor.Set || d.Pages[1].SectionBreakAnchor.Value || d.Pages[0].SectionBreakAnchor.Set {
		t.Fatalf("anchor: page 2 %+v, page 1 %+v", d.Pages[1].SectionBreakAnchor, d.Pages[0].SectionBreakAnchor)
	}

	// Page 1 has no break of its own: page-1 commands are refused at page 1.
	failure := sectionBreakRefusal(t, tpl, `{"kind":"removeSectionBreak","version":1}`, componentApply)
	if failure.DataPath != "pages[0].sectionBreak" {
		t.Errorf("page-1 remove refused at %q", failure.DataPath)
	}

	// An unknown page is refused at pages, the document unchanged.
	for _, command := range []string{
		`{"kind":"removeSectionBreak","version":1,"page":7}`,
		`{"kind":"setSectionBreak","version":1,"offset":60,"snap":false,"page":2}`,
		`{"kind":"setSectionBreakAnchor","version":1,"anchor":true,"page":-1}`,
		`{"kind":"removeSectionBreak","version":1,"page":null}`,
	} {
		if failure := sectionBreakRefusal(t, tpl, command, componentApply); failure.DataPath != pagesPath {
			t.Errorf("%s refused at %q, want %q", command, failure.DataPath, pagesPath)
		}
	}

	// A break through a page-2 element is refused naming it, on page 2's key.
	failure = sectionBreakRefusal(t, tpl, `{"kind":"setSectionBreak","version":1,"offset":30,"snap":false,"page":1}`, componentApply)
	if failure.ElementID != "ee" || failure.DataPath != "pages[1].sectionBreak" {
		t.Errorf("straddle refusal %+v", failure)
	}

	// Straddles are judged by the page the element sits on or lands on.
	sectionBreakRefusal(t, tpl, `{"kind":"moveComponent","version":1,"id":"ee","x":0,"y":55,"snap":false}`, componentApply)
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"moveComponent","version":1,"id":"e6","x":0,"y":55,"snap":false}`)); err != nil {
		t.Errorf("a page-1 element moved across page 2's break offset was refused: %v", err)
	}
	refusePageCommand(t, multiPageTemplateWithPage2Break(t), moveToPage([]string{"e5"}, "e5", "0", "55", 1), "component.geometry")

	applyPageCommand(t, tpl, `{"kind":"removeSectionBreak","version":1,"page":1}`)
	if d := reparse(t, tpl); d.Pages[1].SectionBreak.Set || d.Pages[1].SectionBreakAnchor.Set {
		t.Fatal("removing page 2's break left a key behind")
	}
}

func multiPageTemplateWithPage2Break(t *testing.T) *Template {
	t.Helper()
	return multiPageTemplate(t, editMultiPage(t, func(d *template.Document) { d.Pages[1].SectionBreak = pts(60) }))
}

func TestAPageChangeRefusesToStrandALaterPagesBreak(t *testing.T) {
	tpl := multiPageTemplate(t, editMultiPage(t, func(d *template.Document) { d.Pages[1].SectionBreak = pts(600) }))
	failure := sectionBreakRefusal(t, tpl, `{"kind":"setBandHeight","version":1,"band":"pageHeader","height":200,"snap":false}`, componentApply)
	if failure.DataPath != "pages[1].sectionBreak" || !strings.Contains(failure.Message, "on page 2") {
		t.Errorf("refusal %+v", failure)
	}
}

func TestAMultiPageProjectionCarriesOneBreakPerPage(t *testing.T) {
	tpl := multiPageTemplate(t, editMultiPage(t, func(d *template.Document) {
		d.Pages[1].SectionBreak = pts(60)
		d.Pages[1].SectionBreakAnchor = template.Presence[bool]{Set: true, Value: false}
	}))
	projection := shippedProjection(t, tpl)
	if projection.SectionBreak != nil || projection.SectionBreakAnchor != nil {
		t.Fatal("a multi-page projection carries the one-page pair")
	}
	if len(projection.SectionBreaks) != 2 || projection.SectionBreaks[0] != nil || projection.SectionBreaks[1] == nil || *projection.SectionBreaks[1] != 60000 || !reflect.DeepEqual(projection.SectionBreakAnchors, []bool{true, false}) {
		t.Fatalf("breaks %v anchors %v", projection.SectionBreaks, projection.SectionBreakAnchors)
	}
	raw, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"sectionBreaks":[null,60000]`) || !strings.Contains(string(raw), `"sectionBreakAnchors":[true,false]`) || strings.Contains(string(raw), `"sectionBreak":`) {
		t.Fatalf("wire keys: %s", raw)
	}
	pageOf := contentPageIndex(tpl)
	for _, component := range projection.Components {
		if component.Band != bandContent {
			if component.BelowSectionBreak != nil {
				t.Errorf("%s outside content carries belowSectionBreak", component.ID)
			}
			continue
		}
		switch page := pageOf[component.ID]; {
		case page == 0 && component.BelowSectionBreak != nil:
			t.Errorf("page-1 component %s carries belowSectionBreak with no page-1 break", component.ID)
		case page == 1 && (component.BelowSectionBreak == nil || *component.BelowSectionBreak != (component.Y >= 60000)):
			t.Errorf("page-2 component %s membership %v", component.ID, component.BelowSectionBreak)
		}
	}

	// A multi-page document with no break at all still sends the per-page pair.
	bare := shippedProjection(t, multiPageTemplate(t, multiPageStatementTemplateJSON))
	if !reflect.DeepEqual(bare.SectionBreaks, []*int64{nil, nil}) || !reflect.DeepEqual(bare.SectionBreakAnchors, []bool{true, true}) {
		t.Fatalf("no-break projection %v %v", bare.SectionBreaks, bare.SectionBreakAnchors)
	}
	// A one-page projection keeps its pair exactly and never the per-page one.
	one := shippedProjection(t, sectionBreakCommandDoc(t, sectionBreakAt75))
	if one.SectionBreak == nil || one.SectionBreaks != nil || one.SectionBreakAnchors != nil {
		t.Fatalf("one-page projection %v %v %v", one.SectionBreak, one.SectionBreaks, one.SectionBreakAnchors)
	}
}

// contentDrawn is a page's drawing with the shared header and footer left out:
// every run that does not come from them, and every rect.
func contentDrawn(p pagemodel.Page) ([][3]any, []pagemodel.Rect) {
	shared := map[string]bool{"STATEMENT OF ACCOUNT": true, "Folio Example Bank (synthetic)": true, "Synthetic sample data - not a real account": true}
	var runs [][3]any
	for _, r := range p.Runs {
		if shared[r.SourceText] || strings.HasPrefix(r.SourceText, "Page ") {
			continue
		}
		runs = append(runs, [3]any{r.SourceText, r.X, r.Y})
	}
	return runs, p.Rects
}

// CAP-6: page 1's table crosses page 1's break, and page 2 has its own break
// with a keepTogether group it splits. Page 2's output equals rendering page 2
// alone as a one-page document with that break, and the split is reported
// once, naming page 2's group.
func TestEachPagesBreakAffectsOnlyItsOwnPage(t *testing.T) {
	edit := func(d *template.Document) {
		d.Pages[0].SectionBreak = pts(600)
		d.Pages[0].Elements = append(d.Pages[0].Elements, multiPageText("ei", 620, "Closing note"))
		d.Pages[1].SectionBreak = pts(112)
		d.Pages[1].SectionBreakAnchor = template.Presence[bool]{Set: true, Value: false}
		for _, id := range []string{"eg", "eh"} {
			multiPageElement(d, 1, id).KeepTogether = template.Presence[string]{Set: true, Value: "signature"}
		}
	}
	multi := multiPageTemplate(t, editMultiPage(t, edit))
	doc, err := template.ParseDocument([]byte(editMultiPage(t, edit)))
	if err != nil {
		t.Fatal(err)
	}
	doc.Bands.Content = doc.Pages[1].Band
	doc.Pages = nil
	aloneSrc, err := template.SerializeDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	alone := multiPageTemplate(t, string(aloneSrc))

	multiPages, multiDiags := barcodePages(t, multi, multiPageStatementDataJSON)
	alonePages, _ := barcodePages(t, alone, multiPageStatementDataJSON)
	if len(alonePages) != 1 {
		t.Fatalf("page 2 alone renders %d pages", len(alonePages))
	}
	gotRuns, gotRects := contentDrawn(multiPages[len(multiPages)-1])
	wantRuns, wantRects := contentDrawn(alonePages[0])
	if !reflect.DeepEqual(gotRuns, wantRuns) || !reflect.DeepEqual(gotRects, wantRects) {
		t.Fatalf("page 2's output differs from page 2 alone:\n got %v %v\nwant %v %v", gotRuns, gotRects, wantRuns, wantRects)
	}
	if on, _ := headingBaseline(multiPages, "Closing note"); len(on) != 1 || on[0] >= len(multiPages)-1 {
		t.Fatalf("page 1's below-break note is on %v of %d pages", on, len(multiPages))
	}
	var split []string
	for _, diag := range multiDiags {
		if diag.Code == DiagCodeSectionBreakSplitsKeepTogether {
			split = append(split, diag.ElementID)
		}
	}
	if !reflect.DeepEqual(split, []string{"eg"}) {
		t.Fatalf("split warnings name %v, want [eg]", split)
	}
	idx := keepTogetherTags(multi)
	if idx["eg"].belowBreak || !idx["eh"].belowBreak {
		t.Fatalf("page 2's group is not split at page 2's break: %+v", idx)
	}
}

// pageBreakOffProbe is the multi-page statement with page 2 replaced by one
// probe rect, `height` millipoints tall at y 0, with the given Page Break.
func pageBreakOffProbe(t *testing.T, height geom.Length, pageBreak bool) *Template {
	t.Helper()
	return multiPageTemplate(t, editMultiPage(t, func(d *template.Document) {
		probe := multiPageBox("ei", 0, 1)
		probe.Width = pts(111)
		probe.Height = template.Presence[geom.Length]{Set: true, Value: height}
		d.Pages[1].Elements = []template.Element{probe}
		d.Pages[1].PageBreak = pageBreak
	}))
}

// probeAt returns the output page and page-space top of the probe rect.
func probeAt(t *testing.T, pages []pagemodel.Page) (int, geom.Length) {
	t.Helper()
	for i, p := range pages {
		for _, r := range p.Rects {
			if r.W == 111000 {
				return i, r.Y
			}
		}
	}
	t.Fatal("the probe rect is not drawn")
	return 0, 0
}

// CAP-9's fits and doesn't-fit rows, measured rather than assumed: E is where
// a 1pt probe lands, the window bottom is the fresh-page window top plus the
// content height, and the probe is then exactly the room left, or 1pt more.
func TestAPageBreakOffBlockFitsUnderThePreviousPagesEndOrStartsTheNextWindow(t *testing.T) {
	onPages, diags := barcodePages(t, pageBreakOffProbe(t, 1000, true), multiPageStatementDataJSON)
	if len(diags) != 0 || len(onPages) != 4 {
		t.Fatalf("Page Break on: %d pages, diags %+v", len(onPages), diags)
	}
	freshPage, windowTop := probeAt(t, onPages)
	if freshPage != 3 {
		t.Fatalf("Page Break on: the probe is on output page %d", freshPage+1)
	}
	windowBottom := windowTop + layout.ContentHeight(mustPageGeometry(t, pageBreakOffProbe(t, 1000, true)))

	small, _ := barcodePages(t, pageBreakOffProbe(t, 1000, false), multiPageStatementDataJSON)
	page, end := probeAt(t, small)
	if len(small) != 3 || page != 2 || end <= windowTop || end >= windowBottom {
		t.Fatalf("a 1pt block: %d pages, on page %d at %d (window %d–%d)", len(small), page+1, end, windowTop, windowBottom)
	}
	room := windowBottom - end

	fits, _ := barcodePages(t, pageBreakOffProbe(t, room, false), multiPageStatementDataJSON)
	if page, y := probeAt(t, fits); len(fits) != 3 || page != 2 || y != end {
		t.Errorf("fits: %d pages, probe on page %d at %d; want 3 pages, page 3 at %d", len(fits), page+1, y, end)
	}
	// Every output page keeps its header and a right "Page X of 3".
	for i, runs := range statementPageRuns(t, renderBytes(t, pageBreakOffProbe(t, room, false))) {
		found := false
		for _, run := range runs {
			found = found || run.Text == "Page "+string(rune('1'+i))+" of 3"
		}
		if !found {
			t.Errorf("output page %d does not read Page %d of 3", i+1, i+1)
		}
	}

	tooTall, _ := barcodePages(t, pageBreakOffProbe(t, room+1000, false), multiPageStatementDataJSON)
	if page, y := probeAt(t, tooTall); len(tooTall) != 4 || page != 3 || y != windowTop {
		t.Errorf("doesn't fit: %d pages, probe on page %d at %d; want 4 pages, page 4 at the window top %d", len(tooTall), page+1, y, windowTop)
	}
}

func renderBytes(t *testing.T, tpl *Template) []byte {
	t.Helper()
	res, err := Render(tpl, Data(multiPageStatementDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatal(err)
	}
	return res.Bytes
}

// CAP-9 "No overflow": when page 1 fits one output page, a Page Break off page
// is never pulled up; it renders as output page 2 at its declared positions.
func TestAPageBreakOffPageIsNotPulledOntoAPageThatDidNotOverflow(t *testing.T) {
	var data map[string]json.RawMessage
	if err := json.Unmarshal([]byte(multiPageStatementDataJSON), &data); err != nil {
		t.Fatal(err)
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(data["transactions"], &rows); err != nil {
		t.Fatal(err)
	}
	data["transactions"], _ = json.Marshal(rows[:5])
	short, _ := json.Marshal(data)

	on, _ := barcodePages(t, pageBreakOffProbe(t, 1000, true), string(short))
	off, _ := barcodePages(t, pageBreakOffProbe(t, 1000, false), string(short))
	onPage, onY := probeAt(t, on)
	offPage, offY := probeAt(t, off)
	if len(on) != 2 || len(off) != 2 || onPage != 1 || offPage != 1 || onY != offY {
		t.Fatalf("on: %d pages, probe page %d at %d; off: %d pages, probe page %d at %d", len(on), onPage+1, onY, len(off), offPage+1, offY)
	}
}

// CAP-9 "Empty page": an empty Page Break off page after a page that overflows
// adds no output page.
func TestAnEmptyPageBreakOffPageAddsNoOutputPage(t *testing.T) {
	tpl := multiPageTemplate(t, editMultiPage(t, func(d *template.Document) {
		d.Pages[1].Elements = []template.Element{}
		d.Pages[1].PageBreak = false
	}))
	pages, diags := barcodePages(t, tpl, multiPageStatementDataJSON)
	if len(diags) != 0 || len(pages) != 3 {
		t.Fatalf("%d pages, diags %+v; want page 1's 3 output pages only", len(pages), diags)
	}
}

// A page with Page Break off follows the previous page's content INCLUDING
// that page's below-line section: page 1's unanchored note is part of E.
func TestPageBreakOffFollowsThePreviousPagesBelowLineSection(t *testing.T) {
	withNote := func(pageBreak bool) *Template {
		return multiPageTemplate(t, editMultiPage(t, func(d *template.Document) {
			d.Pages[0].SectionBreak = pts(600)
			d.Pages[0].SectionBreakAnchor = template.Presence[bool]{Set: true, Value: false}
			d.Pages[0].Elements = append(d.Pages[0].Elements, multiPageText("ei", 620, "Closing note"))
			probe := multiPageBox("ej", 0, 1)
			probe.Width = pts(111)
			d.Pages[1].Elements = []template.Element{probe}
			d.Pages[1].PageBreak = pageBreak
		}))
	}
	pages, _ := barcodePages(t, withNote(false), multiPageStatementDataJSON)
	notePages, noteY := headingBaseline(pages, "Closing note")
	page, y := probeAt(t, pages)
	if len(notePages) != 1 || page != notePages[0] || y <= noteY {
		t.Fatalf("the probe is on page %d at %d, the note on %v at baseline %d; want the probe below the note on its page", page+1, y, notePages, noteY)
	}
}

func TestAStraddleOnALaterPageIsALoadErrorNamingThePage(t *testing.T) {
	_, err := ParseTemplate([]byte(editMultiPage(t, func(d *template.Document) { d.Pages[1].SectionBreak = pts(30) })))
	var re *RenderError
	if !errors.As(err, &re) || re.Diagnostic.Code != DiagCodeSectionBreakStraddled || re.Diagnostic.ElementID != "ee" || re.Diagnostic.DataPath != "pages[1].sectionBreak" {
		t.Fatalf("got %v", err)
	}
}

// A duplicate lands on its source's page, so that page's break judges it: eh
// (114–128pt on page 2, 200pt wide) copies 6pt down to 120–134pt.
func TestADuplicateIsJudgedByItsSourcePagesBreak(t *testing.T) {
	const duplicate = `{"kind":"duplicateComponent","version":1,"id":"eh","snap":false}`
	across := multiPageTemplate(t, editMultiPage(t, func(d *template.Document) { d.Pages[1].SectionBreak = pts(130) }))
	if failure := sectionBreakRefusal(t, across, duplicate, componentApply); failure.ElementID == "" || !strings.Contains(failure.Message, "on page 2") {
		t.Errorf("refusal %+v, want one naming the copy on page 2", failure)
	}
	// Page 1's break at 125 crosses no page-1 element and must not judge a page-2 copy.
	pageOne := multiPageTemplate(t, editMultiPage(t, func(d *template.Document) { d.Pages[0].SectionBreak = pts(125) }))
	if _, err := applyComponentCommand(pageOne, []byte(duplicate)); err != nil {
		t.Fatalf("a page-2 copy was refused by page 1's break: %v", err)
	}
	if _, err := ParseTemplate(mustSerializeTemplate(t, pageOne)); err != nil {
		t.Fatalf("the saved document does not load: %v", err)
	}
}

func mustSerializeTemplate(t *testing.T, tpl *Template) []byte {
	t.Helper()
	b, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// clippingTable is page 1's table copied under id, bound to notes[] with a
// 700pt row: its one row is taller than any window and is clipped alone on a
// fresh page.
func clippingTable(d *template.Document, id, columnID string) template.Element {
	table := pageOneTableCopy(d, id, columnID)
	ext := table.Table.Value
	ext.Bind = "notes[]"
	ext.HeaderStyle = template.Presence[template.Style]{Set: true, Value: template.Style{FontSize: pts(7)}}
	table.Table.Value = ext
	style := table.Style.Value
	style.FontSize = pts(700)
	table.Style.Value = style
	return table
}

const clippingNotesData = `{"notes":[{"description":"x"}],`

// (a) The previous page's last output page is clipped, so it has no E: a
// Page Break off page starts a new output page at the window top.
func TestPageBreakOffAfterAClippedLastPageStartsANewOutputPage(t *testing.T) {
	build := func(pageBreak bool) *Template {
		return multiPageTemplate(t, editMultiPage(t, func(d *template.Document) {
			d.Pages[0].Elements = []template.Element{clippingTable(d, "ei", "ej")}
			probe := multiPageBox("ek", 0, 1)
			probe.Width = pts(111)
			d.Pages[1].Elements = []template.Element{probe}
			d.Pages[1].PageBreak = pageBreak
		}))
	}
	data := clippingNotesData + strings.TrimPrefix(multiPageStatementDataJSON, "{")
	on, _ := barcodePages(t, build(true), data)
	off, _ := barcodePages(t, build(false), data)
	onPage, onY := probeAt(t, on)
	offPage, offY := probeAt(t, off)
	if len(on) < 2 || len(off) != len(on) || offPage != onPage || offY != onY || offPage != len(off)-1 {
		t.Fatalf("on: %d pages, probe page %d at %d; off: %d pages, probe page %d at %d", len(on), onPage+1, onY, len(off), offPage+1, offY)
	}
	// And an empty Page Break off page after it still adds no output page.
	empty := multiPageTemplate(t, editMultiPage(t, func(d *template.Document) {
		d.Pages[0].Elements = []template.Element{clippingTable(d, "ei", "ej")}
		d.Pages[1].Elements = []template.Element{}
		d.Pages[1].PageBreak = false
	}))
	alone := multiPageTemplate(t, editMultiPage(t, func(d *template.Document) {
		d.Pages[0].Elements = []template.Element{clippingTable(d, "ei", "ej")}
	}))
	emptyPages, _ := barcodePages(t, empty, data)
	onPages, _ := barcodePages(t, alone, data)
	if len(emptyPages) != len(onPages)-1 {
		t.Fatalf("an empty Page Break off page after a clipped page: %d pages, want %d", len(emptyPages), len(onPages)-1)
	}
}

// (b) A block that itself clips does not fit: it starts a new output page, as
// with Page Break on.
func TestAClippedPageBreakOffBlockStartsANewOutputPage(t *testing.T) {
	build := func(pageBreak bool) *Template {
		return multiPageTemplate(t, editMultiPage(t, func(d *template.Document) {
			d.Pages[1].Elements = []template.Element{clippingTable(d, "ei", "ej")}
			d.Pages[1].PageBreak = pageBreak
		}))
	}
	data := clippingNotesData + strings.TrimPrefix(multiPageStatementDataJSON, "{")
	on, _ := barcodePages(t, build(true), data)
	off, _ := barcodePages(t, build(false), data)
	if len(off) != len(on) || len(off) < 5 {
		t.Fatalf("a clipped Page Break off block: %d pages, Page Break on %d; want the same, at least 5", len(off), len(on))
	}
	for i := range on {
		onRuns, onRects := contentDrawn(on[i])
		offRuns, offRects := contentDrawn(off[i])
		if !reflect.DeepEqual(onRuns, offRuns) || !reflect.DeepEqual(onRects, offRects) {
			t.Errorf("output page %d differs from Page Break on", i+1)
		}
	}
}

// (c) A fitting block with its own unanchored break and an element below the
// line moves as one rigid block: above-line and below-line items move by the
// same distance.
func TestAFittingPageBreakOffBlockWithItsOwnUnanchoredBreakMovesRigidly(t *testing.T) {
	const below = "Below the line"
	build := func(pageBreak bool) *Template {
		return multiPageTemplate(t, editMultiPage(t, func(d *template.Document) {
			probe := multiPageBox("ei", 0, 1)
			probe.Width = pts(111)
			d.Pages[1].Elements = []template.Element{probe, multiPageText("ej", 50, below)}
			d.Pages[1].SectionBreak = pts(40)
			d.Pages[1].SectionBreakAnchor = template.Presence[bool]{Set: true, Value: false}
			d.Pages[1].PageBreak = pageBreak
		}))
	}
	on, _ := barcodePages(t, build(true), multiPageStatementDataJSON)
	off, _ := barcodePages(t, build(false), multiPageStatementDataJSON)
	onPage, onProbe := probeAt(t, on)
	offPage, offProbe := probeAt(t, off)
	onText, onNote := headingBaseline(on, below)
	offText, offNote := headingBaseline(off, below)
	if len(off) != 3 || offPage != 2 || onPage != 3 || !reflect.DeepEqual(offText, []int{2}) || !reflect.DeepEqual(onText, []int{3}) {
		t.Fatalf("off: %d pages, probe page %d, note %v; on: probe page %d, note %v", len(off), offPage+1, offText, onPage+1, onText)
	}
	if moved := offProbe - onProbe; moved <= 0 || offNote-onNote != moved {
		t.Fatalf("the probe moved by %d and the below-line note by %d; want the same positive distance", moved, offNote-onNote)
	}
}
