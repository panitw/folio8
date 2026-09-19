package folio8

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/designer"
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// SPEC-multi-pages story 2: the addPage, deletePage and setPageBreak engine
// commands, their shape changes (D-G.1) and their refusals.

func applyPageCommand(t *testing.T, tpl *Template, command string) designer.CanvasProjection {
	t.Helper()
	projection, err := applyComponentCommand(tpl, []byte(command))
	if err != nil {
		t.Fatalf("%s: %v", command, err)
	}
	return projection
}

func refusePageCommand(t *testing.T, tpl *Template, command, wantPath string) {
	t.Helper()
	before, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	_, err = applyComponentCommand(tpl, []byte(command))
	var failure *designer.ComponentCommandError
	if !errors.As(err, &failure) {
		t.Fatalf("%s: err = %v, want a located refusal", command, err)
	}
	if failure.DataPath != wantPath {
		t.Errorf("%s: refused at %q, want %q (%s)", command, failure.DataPath, wantPath, failure.Message)
	}
	after, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf("%s: a refused command changed the document", command)
	}
}

func reparse(t *testing.T, tpl *Template) *template.Document {
	t.Helper()
	saved, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	d, err := template.ParseDocument(saved)
	if err != nil {
		t.Fatalf("the saved bytes do not load: %v\n%s", err, saved)
	}
	return d
}

func elementIDs(elements []template.Element) []string {
	var out []string
	for _, el := range elements {
		out = append(out, string(el.ID))
	}
	return out
}

func TestAddPageToAOnePageDocumentMovesItsContentAndBreakIntoPageOne(t *testing.T) {
	tpl := multiPageTemplate(t, sectionBreakStatementTemplateJSON)
	original := reparse(t, tpl)
	if original.PageCount() != 1 || !original.Bands.Content.SectionBreak.Set || len(original.Bands.Content.Elements) == 0 {
		t.Fatal("fixture precondition: a one-page document with elements and a section break")
	}
	projection := applyPageCommand(t, tpl, `{"kind":"addPage","version":1}`)
	d := reparse(t, tpl)
	if d.PageCount() != 2 || len(d.Bands.Content.Elements) != 0 || d.Bands.Content.SectionBreak.Set {
		t.Fatalf("pages = %d, bands.content elements = %d: want two pages and an empty bands.content", d.PageCount(), len(d.Bands.Content.Elements))
	}
	if strings.Join(elementIDs(d.Pages[0].Elements), ",") != strings.Join(elementIDs(original.Bands.Content.Elements), ",") || d.Pages[0].SectionBreak != original.Bands.Content.SectionBreak {
		t.Error("page 1 does not hold the old content and its break")
	}
	if len(d.Pages[1].Elements) != 0 || !d.Pages[1].PageBreak {
		t.Error("the new page is not empty with Page Break on")
	}
	if d.Version != "4.1" {
		t.Errorf("version = %s, want 4.1", d.Version)
	}
	if len(projection.PageBreaks) != 2 || !projection.PageBreaks[0] || !projection.PageBreaks[1] {
		t.Errorf("projected pageBreaks = %v", projection.PageBreaks)
	}
}

func TestAddPageInsertsAfterTheNamedPageOrAtTheEnd(t *testing.T) {
	tpl := multiPageTemplate(t, multiPageStatementTemplateJSON)
	applyPageCommand(t, tpl, `{"kind":"addPage","version":1}`)
	d := reparse(t, tpl)
	if d.PageCount() != 3 || len(d.Pages[2].Elements) != 0 || len(d.Pages[1].Elements) == 0 {
		t.Fatalf("without after: want the empty page appended as page 3")
	}
	// After the last page appends, exactly as no after does.
	applyPageCommand(t, tpl, `{"kind":"addPage","version":1,"after":2}`)
	d = reparse(t, tpl)
	if d.PageCount() != 4 || len(d.Pages[3].Elements) != 0 || !d.Pages[3].PageBreak || len(d.Pages[1].Elements) == 0 {
		t.Fatalf("after the last index: want an empty page appended as page 4")
	}
	applyPageCommand(t, tpl, `{"kind":"deletePage","version":1,"page":3}`)
	// Mark old page 3 so its new position is visible.
	applyPageCommand(t, tpl, `{"kind":"setPageBreak","version":1,"page":2,"pageBreak":false}`)
	applyPageCommand(t, tpl, `{"kind":"addPage","version":1,"after":1}`)
	d = reparse(t, tpl)
	if d.PageCount() != 4 {
		t.Fatalf("pages = %d, want 4", d.PageCount())
	}
	if len(d.Pages[2].Elements) != 0 || !d.Pages[2].PageBreak {
		t.Error("the page added after page 2 is not an empty page 3 with Page Break on")
	}
	if d.Pages[3].PageBreak {
		t.Error("old page 3 did not become page 4")
	}
}

func TestDeletePageRemovesThePageAndEveryElementOnIt(t *testing.T) {
	tpl := multiPageTemplate(t, multiPageStatementTemplateJSON)
	applyPageCommand(t, tpl, `{"kind":"addPage","version":1,"after":0}`)
	// Three pages: the statement, an empty page, the terms.
	terms := elementIDs(reparse(t, tpl).Pages[2].Elements)
	applyPageCommand(t, tpl, `{"kind":"deletePage","version":1,"page":1}`)
	d := reparse(t, tpl)
	if d.PageCount() != 2 || strings.Join(elementIDs(d.Pages[1].Elements), ",") != strings.Join(terms, ",") {
		t.Fatal("deleting page 2 did not make page 3 the new page 2")
	}
	projection := applyPageCommand(t, tpl, `{"kind":"deletePage","version":1,"page":1}`)
	d = reparse(t, tpl)
	if d.PageCount() != 1 || d.Pages != nil || len(d.Bands.Content.Elements) == 0 {
		t.Fatal("deleting down to one page did not return the one-page shape")
	}
	for _, id := range terms {
		for _, el := range d.Bands.Content.Elements {
			if string(el.ID) == id {
				t.Errorf("element %s of the deleted page survived", id)
			}
		}
		for _, component := range projection.Components {
			if component.ID == id {
				t.Errorf("element %s of the deleted page is still projected", id)
			}
		}
	}
	if len(projection.PageBreaks) != 1 || len(projection.ContentWindowPages) == 0 {
		t.Errorf("a one-page projection carries pageBreaks %v", projection.PageBreaks)
	}
}

func TestDeletingDownToOnePageKeepsPageOnesSectionBreak(t *testing.T) {
	tpl := multiPageTemplate(t, sectionBreakStatementTemplateJSON)
	breakBefore := reparse(t, tpl).Bands.Content.SectionBreak
	applyPageCommand(t, tpl, `{"kind":"addPage","version":1}`)
	applyPageCommand(t, tpl, `{"kind":"deletePage","version":1,"page":1}`)
	d := reparse(t, tpl)
	if d.Pages != nil || d.Bands.Content.SectionBreak != breakBefore {
		t.Fatalf("the break is not back in bands.content: %+v", d.Bands.Content.SectionBreak)
	}
}

func TestDeletingPageOneTakesItsBreakAndMakesPageTwoTheSinglePage(t *testing.T) {
	tpl := multiPageTemplate(t, sectionBreakStatementTemplateJSON)
	applyPageCommand(t, tpl, `{"kind":"addPage","version":1}`)
	applyPageCommand(t, tpl, `{"kind":"setPageBreak","version":1,"page":1,"pageBreak":false}`)
	applyPageCommand(t, tpl, `{"kind":"deletePage","version":1,"page":0}`)
	d := reparse(t, tpl)
	if d.Pages != nil || len(d.Bands.Content.Elements) != 0 || d.Bands.Content.SectionBreak.Set {
		t.Fatal("old page 2 (empty, no break) did not become the single page")
	}
	saved, _ := SerializeTemplate(tpl)
	if strings.Contains(string(saved), "pageBreak") {
		t.Error("a one-page document still writes a pageBreak")
	}
}

func TestSetPageBreakWritesTheValueOnALaterPage(t *testing.T) {
	tpl := multiPageTemplate(t, multiPageStatementTemplateJSON)
	projection := applyPageCommand(t, tpl, `{"kind":"setPageBreak","version":1,"page":1,"pageBreak":false}`)
	saved, _ := SerializeTemplate(tpl)
	if !strings.Contains(string(saved), `"pageBreak": false`) {
		t.Error("the saved bytes do not state pageBreak false")
	}
	if len(projection.PageBreaks) != 2 || !projection.PageBreaks[0] || projection.PageBreaks[1] {
		t.Errorf("projected pageBreaks = %v, want [true false]", projection.PageBreaks)
	}
}

func TestPageCommandsRefuseWhatTheyCannotDo(t *testing.T) {
	one := multiPageTemplate(t, sectionBreakStatementTemplateJSON)
	refusePageCommand(t, one, `{"kind":"deletePage","version":1,"page":0}`, "bands.content")
	refusePageCommand(t, one, `{"kind":"setPageBreak","version":1,"page":0,"pageBreak":false}`, "bands.content.pageBreak")
	refusePageCommand(t, one, `{"kind":"addPage","version":1,"after":1}`, "pages")

	many := multiPageTemplate(t, multiPageStatementTemplateJSON)
	refusePageCommand(t, many, `{"kind":"setPageBreak","version":1,"page":0,"pageBreak":false}`, "pages[0].pageBreak")
	refusePageCommand(t, many, `{"kind":"setPageBreak","version":1,"page":1,"pageBreak":null}`, "pages[1].pageBreak")
	refusePageCommand(t, many, `{"kind":"setPageBreak","version":1,"page":2,"pageBreak":true}`, "pages")
	refusePageCommand(t, many, `{"kind":"deletePage","version":1,"page":-1}`, "pages")
	refusePageCommand(t, many, `{"kind":"deletePage","version":1,"page":null}`, "pages")
	refusePageCommand(t, many, `{"kind":"deletePage","version":1}`, "pages")
	refusePageCommand(t, many, `{"kind":"addPage","version":1,"after":null}`, "pages")
	refusePageCommand(t, many, `{"kind":"addPage","version":1,"after":0,"extra":1}`, "pages")
}

func TestContentComponentsProjectTheirDesignedPage(t *testing.T) {
	tpl := multiPageTemplate(t, multiPageStatementTemplateJSON)
	projection := shippedProjection(t, tpl)
	d := reparse(t, tpl)
	want := map[string]int{}
	for page, content := range d.Pages {
		for _, el := range content.Elements {
			want[string(el.ID)] = page
		}
	}
	laterPage := 0
	for _, component := range projection.Components {
		if component.Band != bandContent {
			if component.Page != 0 {
				t.Errorf("%s component %s projects page %d, want 0", component.Band, component.ID, component.Page)
			}
			continue
		}
		if component.Page != want[component.ID] {
			t.Errorf("component %s projects page %d, want %d", component.ID, component.Page, want[component.ID])
		}
		if component.Page == 1 {
			laterPage++
		}
	}
	if laterPage == 0 {
		t.Fatal("fixture precondition: no page-2 component was projected")
	}
	for _, component := range shippedProjection(t, multiPageTemplate(t, sectionBreakStatementTemplateJSON)).Components {
		if component.Page != 0 {
			t.Errorf("a one-page document projects component %s on page %d", component.ID, component.Page)
		}
	}
}

// ---------------------------------------------------------------------------
// SPEC-multi-pages story 3: elements on a specific page. createComponent,
// dropComponent and moveComponents take an optional target `page`.

func moveToPage(ids []string, ref, dx, dy string, page int) string {
	return strings.Replace(string(windowMoveIntent(ids, ref, dx, dy, false)), `"constrainToWindow":true`, fmt.Sprintf(`"constrainToWindow":true,"page":%d`, page), 1)
}

func pageOfElement(d *template.Document, id string) (int, *template.Element) {
	for page := range d.Pages {
		for i := range d.Pages[page].Elements {
			if string(d.Pages[page].Elements[i].ID) == id {
				return page, &d.Pages[page].Elements[i]
			}
		}
	}
	return -1, nil
}

func TestCreateAndDropPlaceOnTheTargetPage(t *testing.T) {
	tpl := multiPageTemplate(t, multiPageStatementTemplateJSON)
	before := reparse(t, tpl)
	applyPageCommand(t, tpl, `{"kind":"createComponent","version":1,"type":"text","band":"content","x":100,"y":50,"width":72,"height":24,"snap":false,"page":1}`)
	d := reparse(t, tpl)
	if len(d.Pages[1].Elements) != len(before.Pages[1].Elements)+1 || strings.Join(elementIDs(d.Pages[0].Elements), ",") != strings.Join(elementIDs(before.Pages[0].Elements), ",") {
		t.Fatalf("create did not land on page 2 alone: page 1 %v, page 2 %v", elementIDs(d.Pages[0].Elements), elementIDs(d.Pages[1].Elements))
	}
	if el := d.Pages[1].Elements[len(d.Pages[1].Elements)-1]; el.X != 100000 || el.Y != 50000 {
		t.Errorf("created at %d,%d, want 100000,50000", el.X, el.Y)
	}

	canvas, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	content := canvas.Bands[1]
	applyPageCommand(t, tpl, fmt.Sprintf(`{"kind":"dropComponent","version":1,"type":"text","x":%s,"y":%s,"snap":false,"page":1}`, pointLiteral(content.X+100000), pointLiteral(content.Y+50000)))
	dropped := reparse(t, tpl)
	if len(dropped.Pages[1].Elements) != len(d.Pages[1].Elements)+1 || len(dropped.Pages[0].Elements) != len(d.Pages[0].Elements) {
		t.Fatal("drop did not land on page 2 alone")
	}
	if el := dropped.Pages[1].Elements[len(dropped.Pages[1].Elements)-1]; el.X != 100000 || el.Y != 50000 {
		t.Errorf("dropped at %d,%d, want 100000,50000", el.X, el.Y)
	}
}

func TestPagedCreateDropAndMoveRefuseWhatTheyCannotDo(t *testing.T) {
	tpl := multiPageTemplate(t, multiPageStatementTemplateJSON)
	canvas, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	header := canvas.Bands[0]
	for _, c := range []struct{ command, path string }{
		{`{"kind":"createComponent","version":1,"type":"text","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false,"page":5}`, pagesPath},
		{`{"kind":"dropComponent","version":1,"type":"text","x":100,"y":200,"snap":false,"page":5}`, pagesPath},
		{moveToPage([]string{"e5"}, "e5", "0", "10", 5), pagesPath},
		{`{"kind":"createComponent","version":1,"type":"text","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false,"page":null}`, pagesPath},
		// The header and footer belong to no page.
		{`{"kind":"createComponent","version":1,"type":"text","band":"pageHeader","x":0,"y":0,"width":72,"height":12,"snap":false,"page":1}`, pagesPath},
		{fmt.Sprintf(`{"kind":"dropComponent","version":1,"type":"text","x":%s,"y":%s,"snap":false,"page":1}`, pointLiteral(header.X+1000), pointLiteral(header.Y+1000)), pagesPath},
		{moveToPage([]string{"e1"}, "e1", "0", "10", 1), pagesPath},
		// A selection spanning pages never changes page (D-3.1).
		{moveToPage([]string{"e5", "ed"}, "e5", "0", "10", 1), pagesPath},
		// Target outside band: e5 is 523pt wide, so 100pt right leaves page 2's band.
		{moveToPage([]string{"e5"}, "e5", "100", "200", 1), "component.geometry"},
	} {
		refusePageCommand(t, tpl, c.command, c.path)
	}

	onePage := multiPageTemplate(t, sectionBreakStatementTemplateJSON)
	refusePageCommand(t, onePage, `{"kind":"createComponent","version":1,"type":"text","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false,"page":1}`, pagesPath)
}

func TestMoveComponentsToAnotherPageSplicesTheMembers(t *testing.T) {
	tpl := multiPageTemplate(t, multiPageStatementTemplateJSON)
	applyPageCommand(t, tpl, moveToPage([]string{"e5"}, "e5", "0", "200", 1))
	d := reparse(t, tpl)
	page, el := pageOfElement(d, "e5")
	if page != 1 || el.X != 0 || el.Y != 200000 {
		t.Fatalf("e5 on page %d at %+v, want page index 1 at y 200", page, el)
	}
	if got := strings.Join(elementIDs(d.Pages[0].Elements), ","); got != "e6,e7" {
		t.Errorf("page 1 holds %s, want e6,e7", got)
	}

	// A move whose page is the members' own page is today's move.
	own := multiPageTemplate(t, multiPageStatementTemplateJSON)
	plain := multiPageTemplate(t, multiPageStatementTemplateJSON)
	applyPageCommand(t, own, moveToPage([]string{"ee"}, "ee", "0", "12", 1))
	applyPageCommand(t, plain, string(windowMoveIntent([]string{"ee"}, "ee", "0", "12", false)))
	ownBytes, _ := SerializeTemplate(own)
	plainBytes, _ := SerializeTemplate(plain)
	if !bytes.Equal(ownBytes, plainBytes) {
		t.Error("a move to the members' own page differs from the same move without page")
	}
}

func TestMoveToAnotherPageSnapsWithoutClampingInPreview(t *testing.T) {
	tpl := multiPageTemplate(t, multiPageStatementTemplateJSON)
	original, _ := SerializeTemplate(tpl)
	command := []byte(strings.Replace(moveToPage([]string{"e5"}, "e5", "0", "203", 1), `"snap":false`, `"snap":true`, 1))
	move, err := previewComponentMove(tpl, command)
	if err != nil {
		t.Fatal(err)
	}
	if move.DY != 204000 || move.DX != 0 {
		t.Errorf("preview %+v, want dy 204000 snapped to the grid", move)
	}
	// Beyond the band: the preview still answers, unclamped; only the commit refuses.
	wide, err := previewComponentMove(tpl, []byte(moveToPage([]string{"e5"}, "e5", "100", "0", 1)))
	if err != nil || wide.DX != 100000 {
		t.Errorf("preview %+v %v, want dx 100000", wide, err)
	}
	if after, _ := SerializeTemplate(tpl); !bytes.Equal(after, original) {
		t.Error("preview changed the document")
	}
}

func TestMoveToAnotherPageChecksTheTargetPagesBreak(t *testing.T) {
	src := editMultiPage(t, func(d *template.Document) {
		d.Pages[0].SectionBreak = template.Presence[geom.Length]{Set: true, Value: 300000}
	})
	// Straddle on page 1: ed (18pt tall) moved onto page 1 across its break.
	refusePageCommand(t, multiPageTemplate(t, src), moveToPage([]string{"ed"}, "ed", "0", "290", 0), "component.geometry")
	// Page 2 has no break: e5 moved onto page 2 at page 1's break offset is accepted.
	tpl := multiPageTemplate(t, src)
	applyPageCommand(t, tpl, moveToPage([]string{"e5"}, "e5", "0", "290", 1))
	if page, el := pageOfElement(reparse(t, tpl), "e5"); page != 1 || el.Y != 290000 {
		t.Errorf("e5 on page %d, want page index 1 at y 290", page)
	}
}

func TestMoveToAnotherPageKeepsAKeepTogetherGroupWhole(t *testing.T) {
	src := editMultiPage(t, func(d *template.Document) {
		for _, id := range []string{"eg", "eh"} {
			multiPageElement(d, 1, id).KeepTogether = template.Presence[string]{Set: true, Value: "signature"}
		}
	})
	// Split a group: only eg moves.
	split := multiPageTemplate(t, src)
	refusePageCommand(t, split, moveToPage([]string{"eg"}, "eg", "0", "300", 0), pagesPath)
	_, err := applyComponentCommand(split, []byte(moveToPage([]string{"eg"}, "eg", "0", "300", 0)))
	var failure *designer.ComponentCommandError
	if !errors.As(err, &failure) || failure.ElementID != "eg" || !strings.Contains(failure.Message, `"signature"`) {
		t.Errorf("refusal %v does not name the element and the group", err)
	}

	// The whole group moves together and keeps its offsets.
	tpl := multiPageTemplate(t, src)
	applyPageCommand(t, tpl, moveToPage([]string{"eg", "eh"}, "eg", "0", "300", 0))
	d := reparse(t, tpl)
	gPage, g := pageOfElement(d, "eg")
	hPage, h := pageOfElement(d, "eh")
	if gPage != 0 || hPage != 0 || g.Y != 410000 || h.Y-g.Y != 4000 {
		t.Fatalf("group on pages %d,%d at %d,%d; want both on page index 0, eg at 410000, 4pt apart", gPage, hPage, g.Y, h.Y)
	}
}

// SPEC-multi-pages: paste onto the page under the pointer. duplicateComponents
// takes an optional target `page`.
func TestDuplicateComponentsPastesOntoTheTargetPage(t *testing.T) {
	tpl := multiPageTemplate(t, multiPageStatementTemplateJSON)
	before := reparse(t, tpl)
	sourcePage, source := pageOfElement(before, "e5")
	if sourcePage != 0 {
		t.Fatalf("fixture: e5 is on page %d, want page 1", sourcePage+1)
	}
	applyPageCommand(t, tpl, `{"kind":"duplicateComponents","version":1,"ids":["e5","e1"],"snap":false,"page":1}`)
	d := reparse(t, tpl)
	if len(d.Pages[0].Elements) != len(before.Pages[0].Elements) || len(d.Pages[1].Elements) != len(before.Pages[1].Elements)+1 {
		t.Fatalf("paste did not land on page 2 alone: page 1 %v, page 2 %v", elementIDs(d.Pages[0].Elements), elementIDs(d.Pages[1].Elements))
	}
	// Another page keeps the source's page-local position: no stair-step.
	if pasted := d.Pages[1].Elements[len(d.Pages[1].Elements)-1]; pasted.X != source.X || pasted.Y != source.Y {
		t.Errorf("pasted at %d,%d, want the source's %d,%d", pasted.X, pasted.Y, source.X, source.Y)
	}
	// A header or footer copy ignores the page and stays in its band.
	if header, footer := len(d.Bands.PageHeader.Elements)+len(d.Bands.PageFooter.Elements), len(before.Bands.PageHeader.Elements)+len(before.Bands.PageFooter.Elements); header != footer+1 {
		t.Errorf("header and footer hold %d elements, want %d", header, footer+1)
	}

	// Pasting onto the source's own page is exactly today's duplicate.
	paged := multiPageTemplate(t, multiPageStatementTemplateJSON)
	plain := multiPageTemplate(t, multiPageStatementTemplateJSON)
	applyPageCommand(t, paged, `{"kind":"duplicateComponents","version":1,"ids":["e5"],"snap":true,"page":0}`)
	applyPageCommand(t, plain, `{"kind":"duplicateComponents","version":1,"ids":["e5"],"snap":true}`)
	pagedBytes, _ := SerializeTemplate(paged)
	plainBytes, _ := SerializeTemplate(plain)
	if !bytes.Equal(pagedBytes, plainBytes) {
		t.Error("a paste onto the source's own page differs from today's duplicate")
	}

	refusePageCommand(t, multiPageTemplate(t, multiPageStatementTemplateJSON), `{"kind":"duplicateComponents","version":1,"ids":["e5"],"snap":false,"page":5}`, pagesPath)
}
