package folio8

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/panitw/folio8/folio-go/internal/designer"
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// SPEC-multi-pages story 2: the three page commands. Each is ONE command, so
// the wasm engine records it as one undo entry. Page indexes on the wire are
// 0-based, the same numbering `pages[i]` uses in the file and in refusals.
//
// THE SHAPE RULE (D-G.1). A document holds its content in Bands.Content while
// it has one page, and in Pages — every page, page 1 included — once it has
// two or more. The commands below move content between the two shapes
// exactly when the page count crosses one, so the saved bytes always follow
// the story-1 rules.

// pagesPath locates a refusal about the page list as a whole.
const pagesPath = "pages"

// pageShape is a template's content, saved for rollback.
type pageShape struct {
	content template.Band
	pages   []template.ContentPage
}

func savePageShape(t *Template) pageShape {
	return pageShape{content: t.doc.Bands.Content, pages: append([]template.ContentPage(nil), t.doc.Pages...)}
}

func (s pageShape) restore(t *Template) {
	t.doc.Bands.Content = s.content
	t.doc.Pages = s.pages
}

// contentPagesForEdit returns the document's pages as a fresh list, page 1
// taken from Bands.Content when the document is in the one-page shape.
func contentPagesForEdit(t *Template) []template.ContentPage {
	if len(t.doc.Pages) > 0 {
		return append([]template.ContentPage(nil), t.doc.Pages...)
	}
	content := t.doc.Bands.Content
	return []template.ContentPage{{Band: template.Band{Elements: content.Elements, SectionBreak: content.SectionBreak, SectionBreakAnchor: content.SectionBreakAnchor}, PageBreak: true}}
}

// installContentPages writes pages back in the shape its count requires.
func installContentPages(t *Template, pages []template.ContentPage) {
	if len(pages) == 1 {
		page := pages[0].Band
		t.doc.Bands.Content.Elements = page.Elements
		t.doc.Bands.Content.SectionBreak = page.SectionBreak
		t.doc.Bands.Content.SectionBreakAnchor = page.SectionBreakAnchor
		t.doc.Pages = nil
		return
	}
	// Page 1's Page Break does not apply; it is always held as true.
	pages[0].PageBreak = true
	t.doc.Bands.Content.Elements = []template.Element{}
	t.doc.Bands.Content.SectionBreak = template.Presence[geom.Length]{}
	t.doc.Bands.Content.SectionBreakAnchor = template.Presence[bool]{}
	t.doc.Pages = pages
}

// projectPageEdit projects t after an edit, restoring shape on failure.
func projectPageEdit(t *Template, previous pageShape) (designer.CanvasProjection, error) {
	projection, err := canvas(t)
	if err != nil {
		previous.restore(t)
		return designer.CanvasProjection{}, err
	}
	return projection, nil
}

// pageIndexField reads a 0-based page index. Null, a non-integer and an index
// outside the document are refused, located at the page list.
func pageIndexField(raw map[string]json.RawMessage, name string, count int) (int, error) {
	if value, ok := raw[name]; ok && string(bytes.TrimSpace(value)) == "null" {
		return 0, componentFailure("", pagesPath, fmt.Sprintf("%s must be a page index", name))
	}
	index, err := commandInt(raw, name)
	if err != nil {
		return 0, componentFailure("", pagesPath, err.Error())
	}
	if index < 0 || index >= count {
		return 0, componentFailure("", pagesPath, fmt.Sprintf("%s %d names no page — this document has %d page(s), indexed from 0", name, index, count))
	}
	return index, nil
}

// SPEC-multi-pages story 3: createComponent, dropComponent and moveComponents
// take an optional target `page`. Absent, the command is exactly today's (the
// designer omits it for page 1), so it is counted as a field only when given.
func optionalPageField(t *Template, raw map[string]json.RawMessage) (int, bool, error) {
	if _, ok := raw["page"]; !ok {
		return 0, false, nil
	}
	page, err := pageIndexField(raw, "page", t.doc.PageCount())
	return page, true, err
}

// contentOnlyPage refuses a target page on a band other than content: the
// page header and footer are shared by every page and belong to none.
func contentOnlyPage(bandName, id string) error {
	return componentFailure(id, pagesPath, fmt.Sprintf("page applies only to the content band — %s is shared by every page and belongs to none", bandName))
}

// addPage inserts one empty page with Page Break on: {kind, version, after?}.
// With `after` it goes directly after that page; without it, at the end.
func addPage(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	_, hasAfter := raw["after"]
	want := 2
	if hasAfter {
		want = 3
	}
	if err := componentFields(raw, want); err != nil {
		return designer.CanvasProjection{}, componentFailure("", pagesPath, "addPage takes exactly kind, version and, optionally, after")
	}
	count := t.doc.PageCount()
	insert := count
	if hasAfter {
		after, err := pageIndexField(raw, "after", count)
		if err != nil {
			return designer.CanvasProjection{}, err
		}
		insert = after + 1
	}
	previous := savePageShape(t)
	pages := contentPagesForEdit(t)
	pages = append(pages, template.ContentPage{})
	copy(pages[insert+1:], pages[insert:])
	pages[insert] = template.ContentPage{Band: template.Band{Elements: []template.Element{}}, PageBreak: true}
	installContentPages(t, pages)
	return projectPageEdit(t, previous)
}

// deletePage removes one page and every element on it: {kind, version, page}.
// The document always keeps at least one page. A page's section break goes
// with it; deleting down to one page returns the one-page shape.
func deletePage(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 3); err != nil {
		return designer.CanvasProjection{}, componentFailure("", pagesPath, "deletePage takes exactly kind, version and page")
	}
	count := t.doc.PageCount()
	if count == 1 {
		if _, err := pageIndexField(raw, "page", count); err != nil {
			return designer.CanvasProjection{}, err
		}
		return designer.CanvasProjection{}, componentFailure("", t.doc.PageField(0), "this is the document's only page — a document always keeps at least one page")
	}
	page, err := pageIndexField(raw, "page", count)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	previous := savePageShape(t)
	pages := contentPagesForEdit(t)
	pages = append(pages[:page:page], pages[page+1:]...)
	installContentPages(t, pages)
	return projectPageEdit(t, previous)
}

// setPageBreak sets one later page's Page Break: {kind, version, page,
// pageBreak}. Page 1's does not apply, so it is refused.
func setPageBreak(t *Template, raw map[string]json.RawMessage) (designer.CanvasProjection, error) {
	if err := componentFields(raw, 4); err != nil {
		return designer.CanvasProjection{}, componentFailure("", pagesPath, "setPageBreak takes exactly kind, version, page and pageBreak")
	}
	count := t.doc.PageCount()
	page, err := pageIndexField(raw, "page", count)
	if err != nil {
		return designer.CanvasProjection{}, err
	}
	path := t.doc.PageField(page) + ".pageBreak"
	if page == 0 {
		return designer.CanvasProjection{}, componentFailure("", path, "Page Break does not apply to page 1 — the first page always starts the document")
	}
	// json.Unmarshal leaves a bool untouched for null, so null is refused
	// here rather than read as false.
	if value, ok := raw["pageBreak"]; ok && string(bytes.TrimSpace(value)) == "null" {
		return designer.CanvasProjection{}, componentFailure("", path, "pageBreak must be a boolean")
	}
	value, err := commandBool(raw, "pageBreak")
	if err != nil {
		return designer.CanvasProjection{}, componentFailure("", path, err.Error())
	}
	previous := t.doc.Pages[page].PageBreak
	t.doc.Pages[page].PageBreak = value
	projection, err := canvas(t)
	if err != nil {
		t.doc.Pages[page].PageBreak = previous
		return designer.CanvasProjection{}, err
	}
	return projection, nil
}
