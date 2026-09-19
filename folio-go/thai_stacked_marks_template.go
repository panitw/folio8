package folio8

// thaiStackedMarksTemplateJSON is fixtures/thai-stacked-marks/input.folio,
// kept byte-identical to it by TestThaiStackedMarksGoldenFixture (the
// same hand-sync precedent font-text, multi-script-fallback,
// wrapped-text, mandatory-break, line-spacing, justified-text,
// justified-thai and alignment-rounding set).
//
// IT IS THE FIRST DOCUMENT IN THIS REPOSITORY THAT CARRIES A GLYPH THE
// SHAPER GIVES A NON-ZERO YOffset, and until Story 8.0 no document could
// be: internal/pdf refused such a glyph outright, so a fixture holding
// one would have had no bytes to record. That is why the corpus went
// twenty-one goldens without one, and why "ordinary Thai does not
// render" was found in production rather than by a test (DW-28).
//
// THE TRIGGER IS A NON-ZERO YOffset, NOT MARK STACKING. ที่, ป้ำ and ปั
// each stack two marks over one base and have rendered since Story 2.3,
// because Noto Sans Thai resolves those pairs with a GSUB lowered-form
// substitution at zero offset. ั followed by a tone mark is the pair the
// face resolves by a GPOS y-displacement instead, and that displacement
// is what had nowhere to go. ที่ already appears in
// fixtures/shaped-text, in all four statement-* fixtures and in
// fixtures/justified-thai — so a reader who takes "two stacked marks"
// as the predicate will read those goldens as contradicting this one.
//
// TWO ELEMENTS, and the second is the one that makes the first evidence.
//
//	e1  THE SUBJECT: the contractor-liability clause from the owner's
//	    real Thai contract, verbatim — the document that was pasted into
//	    the shipped designer, drawn correctly on the canvas, and then
//	    refused by the PDF stage with `Render failure · ENGINE_REJECTED`
//	    (DW-28, HIGH). It is not constructed to trip anything: ordinary
//	    Thai legal prose contains ทั้งสิ้น, and ครั้ง, ทั้งนี้ and ตั้งแต่
//	    elsewhere. Its runs carry text-rise operators, and every one of
//	    them is restored to `0 Ts` before the run's ET, because rise is a
//	    persistent text-state parameter that survives ET.
//
//	e2  THE CONTROL, and the byte-identity half stated inside the
//	    document itself: สัญญา ("contract") is the same script, the same
//	    chain, the same size and the same box, and not one of its glyphs
//	    carries a vertical offset. Its run emits NO Ts operator at all —
//	    the same bytes it would have emitted before Story 8.0. A build
//	    that set the rise unconditionally, or that left a rise set across
//	    a run boundary, would show up here and nowhere else in this
//	    document.
//
// The clause is reproduced from folio-go/thai_mark_stacking_test.go,
// which characterized the refusal before this story fixed it. The two
// copies are asserted equal by TestThaiStackedMarksFixtureCarriesTheOwnersClause,
// so the fixture cannot drift away from the document that motivated it.
const thaiStackedMarksTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 100, "value": "การที่ผู้รับจ้างเป็นผู้ประกอบธุรกิจทวงถามหนี้ เป็นผู้มีความรู้ ความชำนาญ มีความสามารถเป็นพิเศษในงานที่รับจ้าง จึงทราบและเข้าใจในพระราชบัญญัติ การทวงถามหนี้ พ.ศ. 2558 และกฎหมายอื่น ๆ ที่เกี่ยวข้องกับกิจการงานที่มารับจ้างตามที่ระบุอยู่ในสัญญาจ้างเป็นอย่างดี ผู้รับจ้างย่อมต้องรับผิดเป็นการส่วนตัวทั้งสิ้น", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "text", "x": 0, "y": 110, "width": 400, "height": 30, "value": "สัญญา", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans", "Noto Sans Thai"]},
  "locale": "th",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+07:00",
  "version": "1.0"
}
`

// thaiStackedMarksDataJSON is the report data fixtures/thai-stacked-marks/
// renders against. The document binds nothing: what it witnesses is a
// property of the SHAPER'S output for a literal string, and bound data
// would only add a way for the fixture to move for reasons that are not
// its subject.
const thaiStackedMarksDataJSON = `{}`
