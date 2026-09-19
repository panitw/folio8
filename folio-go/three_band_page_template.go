package folio8

// threeBandPageTemplateJSON is Story 2.5's three-band composition
// fixture, kept BYTE-IDENTICAL to fixtures/three-band-page/input.folio
// by hand (font-text's and wrapped-text's precedent —
// TestThreeBandPageGoldenFixture asserts the two are equal before it
// asserts anything else).
//
// WHAT THIS DOCUMENT'S CONTENT CAN EXPRESS, which is the point of it
// (D-000.36's corollary: the subject matters as much as the assertion).
// Story 2.5's creator measured, by injecting a wrong band origin into
// each of the three bands in turn and running the whole suite, that:
//
//	page header origin moved  ->  detected by ZERO tests
//	content     origin moved  ->  detected by 5 golden tests
//	page footer origin moved  ->  detected by 1 golden test
//
// The cause was the SUBJECT, not the assertions: all six existing
// fixtures have an EMPTY pageHeader band, and all six share one page
// setup in which marginTop == marginBottom (36 and 36) and
// pageHeader.height == pageFooter.height (20 and 20) — so any
// SUBSTITUTION among the four geometric inputs, and any SWAP of a pair,
// is invisible in the bytes.
//
// This document fixes both halves:
//
//   - ALL THREE BANDS carry a text element, each with a distinct,
//     identifiable literal string, so a band mix-up shows up in the
//     rendered TEXT and not only in a coordinate.
//   - The four geometric inputs are PAIRWISE DISTINCT:
//     margin.top 30, margin.bottom 42, pageHeader.height 18,
//     pageFooter.height 24 — no two equal, so no substitution among
//     them survives.
//   - e2 sits at a NON-ZERO band-relative y (120), so "band origin" and
//     "element y" cannot be conflated by a test that happens to pass
//     when both are zero.
//
// What it CANNOT express, stated so nobody reads more into it than is
// there: it is single-page (pagination is Story 2.6), all-Latin (it
// deliberately creates NO Thai reading judgment — that obligation is
// D-2.3.5's and is bound to fixtures/shaped-text), single-face, and
// carries no image, no binding and no line wrapping. It says nothing
// about leading, shaping or breaking; those are Story 2.4's fixtures.
const threeBandPageTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 500, "height": 20, "value": "CONTENT BAND FIRST ELEMENT", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "text", "x": 0, "y": 120, "width": 500, "height": 20, "value": "CONTENT BAND SECOND ELEMENT", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {
      "elements": [
        {"id": "e3", "type": "text", "x": 0, "y": 6, "width": 500, "height": 16, "value": "FOOTER BAND ONLY", "style": {"fontFamily": "body", "fontSize": 8}}
      ],
      "height": 24
    },
    "pageHeader": {
      "elements": [
        {"id": "e4", "type": "text", "x": 0, "y": 4, "width": 500, "height": 16, "value": "HEADER BAND ONLY", "style": {"fontFamily": "body", "fontSize": 9}}
      ],
      "height": 18
    }
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 5,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 36, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
