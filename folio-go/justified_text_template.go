package folio8

// justifiedTemplateJSON is fixtures/justified-text/input.folio, kept
// byte-identical to it by TestJustifiedTextGoldenFixture (the same
// hand-sync precedent font-text, multi-script-fallback, wrapped-text,
// mandatory-break and line-spacing set).
//
// It is Story 7.3's golden, and it exists because the story's own
// byte-neutrality guard over the pre-7.3 corpus is UNFALSIFIABLE without
// it: measured at this story's baseline, `grep -oh '"align"[^,}]*'
// fixtures/*/input.folio` returned 16 `left` and 8 `right` and nothing
// else, so no recorded byte in the repository could tell a build that
// distributes a justified line's slack from one that draws it ragged.
//
// IT IS ALSO THE FIRST COMMITTED DOCUMENT DECLARING `"version": "2.0"`.
// That is not decoration and it is not the library's ceiling leaking
// into a document: under D-1.4.13 a document declares the lowest version
// its own content requires, and this one requires 2.0 because
// `align: "justify"` EXTENDS A CLOSED SET, which D-1.4.12 makes a MAJOR
// change. A 1.x reader must refuse to draw this file, and its version
// says so.
//
// TWO ELEMENTS, IDENTICAL IN EVERY RESPECT BUT ONE.
//
//	e1  style.align "justify". Its five lines are, in order:
//	      0  seven interior gaps, justified
//	      1  ended by the line feed the author typed - RAGGED, and it is
//	         not the last line, so it is the case only the break-kind
//	         field can answer (D-7.1.5)
//	      2  five interior gaps, justified - a DIFFERENT gap count from
//	         line 0, so a build that hard-coded one cannot agree with
//	         both
//	      3  seven interior gaps, justified
//	      4  the last line - RAGGED, and it is the case only the line
//	         INDEX can answer, because a line no break ended carries the
//	         zero value BreakOptional
//
//	e2  THE CONTROL, and it is what makes e1's numbers mean something:
//	    the same string, the same face, the same size and the same box,
//	    with NO align at all. It packs into the same five lines at the
//	    same five baselines, so a build that ignored `justify` would
//	    render e1 exactly as e2 - one run per line, all at the element's
//	    own left edge. What it could not do is emit e1's eight, one, six,
//	    eight and one runs at eight distinct x positions per justified
//	    line while leaving e2 untouched.
//
// Every number here is MEASURED, not assumed - see
// TestJustifiedTextSemanticAcceptance.
const justifiedTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 80, "value": "Each party shall keep the terms of this deed confidential at all times.\nDisclosure is permitted only where the law requires it, and then only to the extent so required.", "style": {"fontFamily": "body", "fontSize": 11, "align": "justify"}},
        {"id": "e2", "type": "text", "x": 0, "y": 100, "width": 200, "height": 80, "value": "Each party shall keep the terms of this deed confidential at all times.\nDisclosure is permitted only where the law requires it, and then only to the extent so required.", "style": {"fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+07:00",
  "version": "2.0"
}
`

// justifiedDataJSON is the report data fixtures/justified-text/ renders
// against. The document binds nothing: justification is a property of
// the declared box and the packed line, and giving it bound data would
// only add a way for the fixture to move for reasons that are not its
// subject.
const justifiedDataJSON = `{}`
