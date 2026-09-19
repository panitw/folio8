package folio8

// mandatoryBreakTemplateJSON is fixtures/mandatory-break/input.folio,
// kept byte-identical to it by TestMandatoryBreakGoldenFixture (the
// same hand-sync precedent font-text, multi-script-fallback and
// wrapped-text set).
//
// It is Story 7.1's golden, and it exists because AC6's byte-identity
// guard over the pre-7.1 corpus is UNFALSIFIABLE without it: measured
// at this story's baseline, not one committed fixture's text or bound
// data contained a line feed, so no recorded byte in the repository
// could tell a document that honours a typed break from one that
// silently eats it.
//
// Three elements, each carrying a property nothing else in the corpus
// can express:
//
//	e1  "Yours\nsincerely" in a 200 pt box -> 2 lines. THE AC1 ELEMENT.
//	    Its whole value MEASURES 74,305 mp against a 200,000 mp box, so
//	    it FITS — which is the entire point (D-7.1.7, finding 2): the
//	    packer's "does everything that is left fit?" short-circuit
//	    bypasses the opportunity list altogether, and a fixture whose
//	    text was too wide for its box would never reach it and would
//	    certify a break that was taken for want of room rather than
//	    because the author typed it.
//
//	e2  "Clause 1.\n\nClause 2." in a 200 pt box -> 3 lines, THE MIDDLE
//	    ONE EMPTY. A paragraph gap, inexpressible before this story.
//	    The empty line draws nothing, so the artifact carries TWO
//	    baselines for this element separated by TWO advances — which is
//	    how "an empty line occupies one full Advance" is observable in
//	    the produced bytes at all (D-7.1.2).
//
//	e3  the declared span, "{{customer.note}}" in a 40 pt box -> 2
//	    lines. THIS IS THE DISCRIMINATING ELEMENT (D-7.1.1), and it
//	    discriminates in BOTH directions at once because its bound
//	    value holds a line feed AND a space:
//
//	      * the line feed is STRICTLY INTERIOR to the atomic span
//	        atomicSpansFor builds for "customer.note", so it is
//	        suppressed unless the mandatory kind is exempted at the
//	        filter site — flip that test and this element becomes ONE
//	        line;
//	      * "second word" measures 66,220 mp against a 40,000 mp box
//	        while "second" alone measures 36,971, so WITHOUT the
//	        unbreakableValues declaration the packer would break at the
//	        space and this element would become THREE lines. With it,
//	        the value stays whole and overflows visibly (AC11), which
//	        is the existing DiagCodeTextClippedWidth Warning and no new
//	        code.
//
//	    Either assertion alone cannot tell "the exemption works" from
//	    "span suppression broke". Both are asserted, and the two
//	    failures are distinguishable.
//
// THE LINE FEED ARRIVES THROUGH DATA, NOT THROUGH TEMPLATE LITERAL
// TEXT, AND THAT IS LOAD-BEARING (D-7.1.1's correction).
// bind.Substitution's Start/End bracket ONLY what a placeholder
// substituted, so a line feed typed into an element's own literal text
// is never inside an atomic span at all and the collision cannot arise
// for it. A fixture that put the \n in literal text would pass
// vacuously. e1 and e2 do carry literal line feeds — they are the AC1
// and paragraph-gap elements, and they make no claim about spans.
//
// Every line count here is MEASURED, not assumed — see
// TestMandatoryBreakSemanticAcceptance.
const mandatoryBreakTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 40, "value": "Yours\nsincerely", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e2", "type": "text", "x": 0, "y": 60, "width": 200, "height": 60, "value": "Clause 1.\n\nClause 2.", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e3", "type": "text", "x": 0, "y": 140, "width": 40, "height": 40, "value": "{{customer.note}}", "style": {"fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "unbreakableValues": ["customer.note"],
  "utcOffset": "+07:00",
  "version": "1.0"
}
`

// mandatoryBreakDataJSON is the report data fixtures/mandatory-break/
// renders against.
//
// customer.note is D-7.1.1's worked case: ONE declared-unbreakable
// value carrying BOTH a line feed and a space, so the fixture can tell
// the two failure directions apart. The line feed must survive the
// declaration; the space must not.
const mandatoryBreakDataJSON = `{"customer":{"note":"first\nsecond word"}}`
