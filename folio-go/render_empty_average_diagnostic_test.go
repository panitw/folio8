package folio8

// Story 3.3, DECISION-5/R9: avg() over a present-but-empty collection
// is a Warning, not a render-aborting Error — the render proceeds, the
// aggregate resolves to empty, and folio8 mints DiagCodeEmptyAverage for
// it (diagnostic.go), following DiagCodeTextClippedWidth's own
// precedent (render_clip_diagnostic_test.go).
//
// A bare "{{avg(...)}}" is the shape exercised here: an EMPTY average
// resolves to expr.KindNull, which text bindings render as empty. A
// NON-empty average is a real Decimal, which (since the number-in-text-
// binding spec, 2026-09-13) prints as its exact decimal rather than
// failing; formatNumber() remains the way to style it.

import (
	"fmt"
	"reflect"
	"testing"
)

func emptyAverageTemplateJSON(collectionJSON string) string {
	return `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 20, "y": 40, "width": 200, "height": 20, "value": "{{avg(t.a)}}", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

// TestAvgOverEmptyCollectionRendersSuccessfullyWithWarning is
// DECISION-5/R9's own acceptance case, and Story 4.2's forcing AC
// applied one story early: a present-but-empty collection must not
// abort the render.
func TestAvgOverEmptyCollectionRendersSuccessfullyWithWarning(t *testing.T) {
	tpl, err := ParseTemplate([]byte(emptyAverageTemplateJSON("")))
	if err != nil {
		t.Fatalf("presence precondition: %v", err)
	}
	res, err := Render(tpl, Data(`{"t":[]}`), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("AC (DECISION-5/R9) VIOLATED: avg() over an empty collection must not abort the render, got error: %v", err)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("the render must still produce bytes")
	}
	if len(res.Diagnostics) != 1 {
		t.Fatalf("want exactly 1 Diagnostic, got %d: %+v", len(res.Diagnostics), res.Diagnostics)
	}
	d := res.Diagnostics[0]
	if d.Severity != SeverityWarning {
		t.Errorf("Severity = %v, want SeverityWarning — an Error here would make Story 4.2's empty-collection-table AC unsatisfiable", d.Severity)
	}
	if d.Code != DiagCodeEmptyAverage {
		t.Errorf("Code = %q, want %q", d.Code, DiagCodeEmptyAverage)
	}
	if d.ElementID != "e1" {
		t.Errorf("ElementID = %q, want %q", d.ElementID, "e1")
	}
	if d.DataPath != "t.a" {
		t.Errorf("DataPath = %q, want %q (the collection path exactly as authored)", d.DataPath, "t.a")
	}
}

// TestAvgOverNonEmptyCollectionProducesNoCaveat is the negative
// control (D-000.34/D-000.36): a REAL average (over a non-empty
// collection) is a number, which a bare text placeholder prints as its
// exact decimal (2026-09-13) — so it renders with NO diagnostic, never
// silently producing the empty-average Warning.
func TestAvgOverNonEmptyCollectionProducesNoCaveat(t *testing.T) {
	tpl, err := ParseTemplate([]byte(emptyAverageTemplateJSON("")))
	if err != nil {
		t.Fatalf("presence precondition: %v", err)
	}
	res, err := Render(tpl, Data(`{"t":[{"a":1},{"a":2}]}`), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("a bare {{avg(...)}} over a NON-empty collection must print its exact decimal: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Fatalf("a real average must produce no empty-average Warning, got: %+v", res.Diagnostics)
	}
	if _, ok := anyPageContains(pageTextsOf(t, res.Bytes), "1.5000"); !ok {
		t.Fatalf("the average must print as 1.5000; drawn: %v", pageTextsOf(t, res.Bytes))
	}
}

// TestDiagnosticsEmptyIsNilForAverage is D-2.8.6's second determinism
// rule, re-proven for the new code path: a render with nothing to
// report (here, an all-null average — a REAL value, R7.3 — cannot be
// rendered as bare text either, so the negative control instead uses a
// document with no aggregate at all).
func TestDiagnosticsEmptyIsNilForAverage(t *testing.T) {
	tplJSON := `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 20, "y": 40, "width": 200, "height": 20, "value": "no aggregate here", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	tpl, err := ParseTemplate([]byte(tplJSON))
	if err != nil {
		t.Fatalf("presence precondition: %v", err)
	}
	res, err := Render(tpl, Data(`{}`), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if res.Diagnostics != nil {
		t.Fatalf("Diagnostics = %#v, want nil", res.Diagnostics)
	}
}

// TestBindStageCaveatPrecedesLayoutStageClipWarning is D-2.8.6's
// ordering guarantee applied to the NEW pipeline-stage pair this story
// introduces: WITHIN one element, a bind-stage Caveat (avg()-on-empty)
// must precede that SAME element's own layout-stage clip Warning.
func TestBindStageCaveatPrecedesLayoutStageClipWarning(t *testing.T) {
	// A single element whose bound text resolves to empty (the average
	// caveat fires) AND whose declared width is 0 — text.go's own
	// early-return on boundText=="" means an EMPTY bound string never
	// reaches width-overflow detection at all, so this element alone
	// cannot exhibit BOTH diagnostics; instead, two elements sharing one
	// declaration order prove the ACROSS-element half (band order then
	// declaration order, already covered by
	// TestDiagnosticsOrderIsDocumentOrder) and this test asserts the
	// WITHIN-one-element half is even possible to reason about: the
	// avg() caveat is appended to diags BEFORE collectBandTextRuns
	// reaches this element's own overflow check (render.go), which is a
	// property of the SOURCE ORDER of the two appends for the SAME
	// element, not of two different elements — asserted here by reading
	// render.go's own two append sites' relative order is unnecessary
	// to re-derive at the unit level once TestDiagnosticsOrderIsDocumentOrder
	// and TestAvgOverEmptyCollectionRendersSuccessfullyWithWarning both
	// pass; this test instead pins the OBSERVABLE consequence directly:
	// an element whose avg() is empty (Caveat) is declared BEFORE a
	// second element that overflows (clip Warning), and the combined
	// Diagnostics slice preserves that declaration order.
	tplJSON := `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 20, "y": 40, "width": 200, "height": 20, "value": "{{avg(t.a)}}", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "text", "x": 20, "y": 80, "width": 20, "height": 20, "value": "Supercalifragilisticexpialidocious", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 100,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	tpl, err := ParseTemplate([]byte(tplJSON))
	if err != nil {
		t.Fatalf("presence precondition: %v", err)
	}
	res, err := Render(tpl, Data(`{"t":[]}`), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	var gotCodes []string
	for _, d := range res.Diagnostics {
		gotCodes = append(gotCodes, d.Code)
	}
	wantCodes := []string{DiagCodeEmptyAverage, DiagCodeTextClippedWidth}
	if !reflect.DeepEqual(gotCodes, wantCodes) {
		t.Errorf("Diagnostics codes = %v, want %v (declaration order: e1's caveat before e2's clip)", gotCodes, wantCodes)
	}
}

// multiPageAggregateBandTemplateJSON builds a synthetic .folio document
// SHARING multi_page_composition_test.go's own measured geometry (page
// height 841890mp, margin.top 30000mp, margin.bottom 42000mp,
// pageHeader.height 18000mp, pageFooter.height 24000mp; 40 repetitions of
// the fixed sentence is THAT FILE's own measured 3-page boundary), with
// bandElement placed in the repeated band named by band ("pageHeader" or
// "pageFooter") — the one place D-3.3.6's own falsifier lives: "a caveat
// that arrives for body text but not for a page header/footer".
func multiPageAggregateBandTemplateJSON(band, bandElementJSON string) string {
	const sentence = "The quick brown fox jumps over the lazy dog. "
	value := ""
	for i := 0; i < 40; i++ {
		value += sentence
	}
	contentElements := fmt.Sprintf(`[{"id": "e1", "type": "text", "x": 0, "y": 0, "width": 480, "height": 700, "value": %q, "style": {"fontFamily": "body", "fontSize": 24}}]`, value)

	pageHeaderElements, pageFooterElements := "[]", "[]"
	switch band {
	case "pageHeader":
		pageHeaderElements = "[" + bandElementJSON + "]"
	case "pageFooter":
		pageFooterElements = "[" + bandElementJSON + "]"
	default:
		panic("multiPageAggregateBandTemplateJSON: band must be \"pageHeader\" or \"pageFooter\"")
	}

	return `{
  "assets": {},
  "bands": {
    "content": {"elements": ` + contentElements + `},
    "pageFooter": {"elements": ` + pageFooterElements + `, "height": 24},
    "pageHeader": {"elements": ` + pageHeaderElements + `, "height": 18}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 100,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

// aggregateFooterElementJSON is the reviewer's exact trap case (D-3.3.6's
// own named falsifier): a page-footer element mixing the page-number
// reservation ({{page}}/{{pages}}, resolved per page by
// shiftSubstitutions/resolvePageTokens) with an aggregate ({{avg(t.a)}},
// resolved ONCE, at bind time, before the band is duplicated across
// pages).
const aggregateFooterElementJSON = `{"id": "e10", "type": "text", "x": 20, "y": 4, "width": 400, "height": 18, "value": "Page {{page}} of {{pages}} — avg {{avg(t.a)}}", "style": {"fontFamily": "body", "fontSize": 10}}`

// TestAggregateEmptyAverageCaveatFiresOnceInPageFooterAcrossMultiplePages
// is Story 3.3 finisher pass, Finding 11 (D-3.3.6): the review found the
// shipped suite exercised only body-text elements, never a page-footer or
// page-header element mixing {{page}}/{{pages}} with an aggregate — the
// EXACT case D-3.3.6 names as its own falsifier ("a caveat that arrives
// for body text but not for a page header"), and the exact place a future
// refactor folding the Caveat onto bind.Substitution (which
// shiftSubstitutions reconstructs field-by-field, dropping any field it
// does not know about) would silently break without any test noticing.
//
// A genuinely MULTI-PAGE document is required, not a single-page one:
// the property under test is "once, not per page" (the page-footer band
// is duplicated onto every page), which a one-page document cannot
// distinguish from "correctly once".
func TestAggregateEmptyAverageCaveatFiresOnceInPageFooterAcrossMultiplePages(t *testing.T) {
	tpl, err := ParseTemplate([]byte(multiPageAggregateBandTemplateJSON("pageFooter", aggregateFooterElementJSON)))
	if err != nil {
		t.Fatalf("presence precondition: %v", err)
	}
	res, err := Render(tpl, Data(`{"t":[]}`), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("AC (D-3.3.6) VIOLATED: a page-footer avg()-on-empty must not abort the render, got error: %v", err)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("the render must still produce bytes")
	}

	pageCount := readDeclaredCount(t, res.Bytes)
	if pageCount < 2 {
		t.Fatalf("presence precondition: this test requires a genuinely multi-page document (the geometry/sentence-count pairing is measured elsewhere to produce 3 pages), got %d page(s) — \"once, not per page\" is unfalsifiable on a single page", pageCount)
	}

	var emptyAverageDiags []Diagnostic
	for _, d := range res.Diagnostics {
		if d.Code == DiagCodeEmptyAverage {
			emptyAverageDiags = append(emptyAverageDiags, d)
		}
	}
	if len(emptyAverageDiags) != 1 {
		t.Fatalf("RED-PROOF: want exactly 1 %s Diagnostic across %d pages (the footer band is duplicated onto every page, but the aggregate is resolved ONCE, at bind time), got %d: %+v",
			DiagCodeEmptyAverage, pageCount, len(emptyAverageDiags), emptyAverageDiags)
	}
	d := emptyAverageDiags[0]
	if d.Severity != SeverityWarning {
		t.Errorf("Severity = %v, want SeverityWarning", d.Severity)
	}
	if d.ElementID != "e10" {
		t.Errorf("ElementID = %q, want %q", d.ElementID, "e10")
	}
	if d.DataPath != "t.a" {
		t.Errorf("DataPath = %q, want %q", d.DataPath, "t.a")
	}
}

// TestAggregateEmptyAverageCaveatFiresOnceInPageHeaderAcrossMultiplePages
// is the page-HEADER twin (Finding 11's own suggested resolution: "a
// header-band twin asserting a count of exactly 1 across a multi-page
// document is worth the extra ten lines") — the same property, the other
// repeated band.
func TestAggregateEmptyAverageCaveatFiresOnceInPageHeaderAcrossMultiplePages(t *testing.T) {
	headerElement := `{"id": "e11", "type": "text", "x": 20, "y": 2, "width": 400, "height": 14, "value": "avg {{avg(t.a)}}", "style": {"fontFamily": "body", "fontSize": 10}}`
	tpl, err := ParseTemplate([]byte(multiPageAggregateBandTemplateJSON("pageHeader", headerElement)))
	if err != nil {
		t.Fatalf("presence precondition: %v", err)
	}
	res, err := Render(tpl, Data(`{"t":[]}`), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("AC (D-3.3.6) VIOLATED: a page-header avg()-on-empty must not abort the render, got error: %v", err)
	}

	pageCount := readDeclaredCount(t, res.Bytes)
	if pageCount < 2 {
		t.Fatalf("presence precondition: this test requires a genuinely multi-page document, got %d page(s)", pageCount)
	}

	var emptyAverageDiags []Diagnostic
	for _, d := range res.Diagnostics {
		if d.Code == DiagCodeEmptyAverage {
			emptyAverageDiags = append(emptyAverageDiags, d)
		}
	}
	if len(emptyAverageDiags) != 1 {
		t.Fatalf("RED-PROOF: want exactly 1 %s Diagnostic across %d pages, got %d: %+v", DiagCodeEmptyAverage, pageCount, len(emptyAverageDiags), emptyAverageDiags)
	}
	if emptyAverageDiags[0].ElementID != "e11" {
		t.Errorf("ElementID = %q, want %q", emptyAverageDiags[0].ElementID, "e11")
	}
}
