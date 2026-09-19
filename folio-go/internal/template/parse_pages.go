package template

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/panitw/folio8/folio-go/internal/diag"
)

// This file decodes SPEC-multi-pages' top-level `pages` array (D-G.1). A
// document with two or more designed pages lists every page, page 1 included,
// in `pages`, and its `bands.content` holds no elements. A one-page document
// keeps the `bands.content` shape and has no `pages` key.

// pagesKey is the top-level key.
const pagesKey = "pages"

// pageBreakKey is a page entry's Page Break setting.
const pageBreakKey = "pageBreak"

// isPageField reports whether field is a `pages[i]` location, the bandField
// decodeElement carries for a page entry's element.
func isPageField(field string) bool {
	return strings.HasPrefix(field, pagesKey+"[")
}

// pageField is the location of page entry i.
func pageField(i int) string {
	return fmt.Sprintf("%s[%d]", pagesKey, i)
}

// decodePages decodes `pages` and checks it against the content band already
// decoded into content. Ids are claimed through the one document-wide ctx, so
// an id repeated on a later page is refused at that page.
func decodePages(ctx *parseCtx, raw json.RawMessage, content Band) ([]ContentPage, error) {
	items, err := decodeArrayRaw(raw)
	if err != nil {
		return nil, newLoadErrorCoded(pagesKey, "", string(raw), "must be an array of page objects", diag.CodePagesInvalid)
	}
	if len(items) < 2 {
		return nil, newLoadErrorCoded(pagesKey, "", string(raw), fmt.Sprintf("has %d entries — a multi-page document lists at least two pages; a one-page document declares its content in bands.content and has no pages key", len(items)), diag.CodePagesInvalid)
	}
	if len(content.Elements) > 0 {
		return nil, newLoadErrorCoded(contentBandField, "", "", "declares elements beside a pages array — a multi-page document lists every page, page 1 included, in pages, and bands.content holds no elements", diag.CodePagesInvalid)
	}
	if content.SectionBreak.Set || content.SectionBreakAnchor.Set {
		key := sectionBreakKey
		if !content.SectionBreak.Set {
			key = sectionBreakAnchorKey
		}
		return nil, newLoadErrorCoded(contentBandField+"."+key, "", "", "declared beside a pages array — a multi-page document declares a page's section break on its pages entry", diag.CodePagesInvalid)
	}

	pages := make([]ContentPage, 0, len(items))
	for i, item := range items {
		page, err := decodeContentPage(ctx, i, item)
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	if err := refuseKeepTogetherAcrossPages(pages); err != nil {
		return nil, err
	}
	return pages, nil
}

// decodeContentPage decodes one `pages` entry. Its key set is closed.
func decodeContentPage(ctx *parseCtx, i int, raw json.RawMessage) (ContentPage, error) {
	field := pageField(i)
	obj, err := decodeObjectMap(raw)
	if err != nil {
		return ContentPage{}, newLoadErrorCoded(field, "", string(raw), "must be an object with elements and, optionally, pageBreak, sectionBreak and sectionBreakAnchor", diag.CodePagesInvalid)
	}
	if keys := unexpectedKeys(obj, map[string]bool{"elements": true, pageBreakKey: true, sectionBreakKey: true, sectionBreakAnchorKey: true}); len(keys) > 0 {
		return ContentPage{}, newLoadErrorCoded(field+"."+keys[0], "", keys[0], "is not a key a page may carry — a page holds exactly elements, pageBreak, sectionBreak and sectionBreakAnchor", diag.CodePagesInvalid)
	}

	elemsRaw, ok := obj["elements"]
	if !ok {
		return ContentPage{}, newLoadErrorCoded(field+".elements", "", "", "missing required field", diag.CodePagesInvalid)
	}
	elems, err := decodeElements(ctx, field, elemsRaw)
	if err != nil {
		return ContentPage{}, err
	}

	page := ContentPage{PageBreak: true}
	page.Elements = elems

	if pbRaw, ok := obj[pageBreakKey]; ok {
		pbField := field + "." + pageBreakKey
		if n := countTopLevelKey(raw, pageBreakKey); n > 1 {
			return ContentPage{}, newLoadErrorCoded(pbField, "", string(pbRaw), "declared more than once", diag.CodePagesInvalid)
		}
		v, err := decodeBoolRaw(pbRaw)
		if err != nil || rawIsNull(pbRaw) {
			return ContentPage{}, newLoadErrorCoded(pbField, "", string(pbRaw), "must be true or false", diag.CodePagesInvalid)
		}
		// Page 1's value is ignored and dropped on save: Page Break does not
		// apply to the first page.
		if i > 0 {
			page.PageBreak = v
		}
	}

	// SPEC-multi-pages CAP-6: every page may declare its own section break,
	// decoded exactly as page 1's; its range and straddle checks run per page
	// in package folio8 (validateSectionBreak).
	sectionBreak, anchor, err := decodeSectionBreakKeys(obj, raw, field, map[string]bool{}, false)
	if err != nil {
		return ContentPage{}, err
	}
	page.SectionBreak, page.SectionBreakAnchor = sectionBreak, anchor
	return page, nil
}

// refuseKeepTogetherAcrossPages refuses a keepTogether tag used on two pages:
// a group keeps its members on one output page, and a designed page always
// starts its own.
func refuseKeepTogetherAcrossPages(pages []ContentPage) error {
	firstPage := map[string]int{}
	for i, page := range pages {
		for _, el := range page.Elements {
			if !el.KeepTogether.Set || el.KeepTogether.Null {
				continue
			}
			tag := el.KeepTogether.Value
			first, seen := firstPage[tag]
			if !seen {
				firstPage[tag] = i
				continue
			}
			if first != i {
				return newLoadErrorCoded(pageField(i), string(el.ID), tag,
					fmt.Sprintf("keepTogether group %q has members on %s and %s — a keep-together group cannot span pages; give the members on one page a different tag", tag, pageField(first), pageField(i)),
					diag.CodePagesInvalid)
			}
		}
	}
	return nil
}
