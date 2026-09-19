package folio8

// justifiedThaiTemplateJSON is fixtures/justified-thai/input.folio, kept
// byte-identical to it by TestJustifiedThaiGoldenFixture (the same
// hand-sync precedent font-text, multi-script-fallback, wrapped-text,
// mandatory-break, line-spacing, justified-text and alignment-rounding
// set).
//
// IT IS THE COVERAGE fixtures/justified-text/ COULD NOT GIVE. That
// fixture is pure Latin, so every gap it justifies is a run of
// whitespace the author typed. Thai writes its sentences WITHOUT
// spaces: a Thai line's break opportunities come from the shipped
// dictionary (AD-25), and nothing in this repository named Thai and
// `justify` together until this document. The behaviour was already
// correct — this fixture is what makes it FALSIFIABLE, which is the
// same absence that cost DW-24 three stories.
//
// THREE ELEMENTS.
//
//	e1  style.align "justify", a two-paragraph Thai contract clause in
//	    a 220 pt box. Its six lines are, in order:
//	      0  seven interior gaps, justified across DICTIONARY-DERIVED
//	         opportunities - there is not one space on the line
//	      1  six interior gaps, justified
//	      2  ended by the line feed the author typed - RAGGED, and it
//	         is not the last line, so it is the case only the
//	         break-kind field can answer (D-7.1.5)
//	      3  five interior gaps, justified
//	      4  nine interior gaps, justified - four DIFFERENT gap counts
//	         across the document, so a build that hard-coded one
//	         cannot agree with all of them
//	      5  the last line - RAGGED, the case only the line INDEX can
//	         answer
//	    Every justified line carries a NON-ZERO remainder, and three of
//	    the four carry one LARGER than half the gap count - which an
//	    implementation spreading the remainder from the END would place
//	    at the wrong gaps.
//
//	e2  THE CONTROL: the same string, the same chain, the same size and
//	    the same box, with NO align at all. It packs into the same six
//	    lines at the same six intervals, one run each, at the element's
//	    own left edge. A build that ignored `justify` - or that saw
//	    Thai and quietly fell back to ragged left - would render e1
//	    exactly as this.
//
//	e3  AD-25's ATOMIC UNKNOWN RUN, and the third ragged condition the
//	    acceptance criteria were originally silent on. The segmenter
//	    proposes ZERO interior break opportunities inside the run
//	    "ณัฐกานต์" - which
//	    TestJustifiedThaiAtomicRunHasNowhereToPutSlack MEASURES against
//	    the shipped dictionary rather than assuming. Its first line is
//	    justified, is NOT the last line, is NOT ended by a mandatory
//	    break, and has POSITIVE slack - and it is still set at the
//	    element's natural start edge, because there is nowhere to put
//	    the slack.
//
//	    NOT because the wordlist lacks the letters: it DOES hold
//	    "กานต์", a suffix of that run. What the wordlist happens to
//	    contain is not the property this case rests on, and the
//	    zero-opportunity precondition is asserted with t.Fatalf so the
//	    case cannot go vacuous if the segmenter's answer ever changes.
//
// Every number here is MEASURED, not assumed - see
// TestJustifiedThaiSemanticAcceptance.
const justifiedThaiTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 220, "height": 105, "value": "คู่สัญญาตกลงรักษาความลับของข้อตกลงฉบับนี้ตลอดเวลาและจะเปิดเผยข้อมูลต่อบุคคลภายนอกเฉพาะกรณีที่กฎหมายกำหนด\nใบเสร็จรับเงินค่าน้ำประปาประจำเดือนมกราคมออกให้แก่ผู้รับบริการในประเทศไทยตามที่ระบุไว้ในเอกสารแนบท้ายสัญญา", "style": {"fontFamily": "body", "fontSize": 11, "align": "justify"}},
        {"id": "e2", "type": "text", "x": 0, "y": 115, "width": 220, "height": 105, "value": "คู่สัญญาตกลงรักษาความลับของข้อตกลงฉบับนี้ตลอดเวลาและจะเปิดเผยข้อมูลต่อบุคคลภายนอกเฉพาะกรณีที่กฎหมายกำหนด\nใบเสร็จรับเงินค่าน้ำประปาประจำเดือนมกราคมออกให้แก่ผู้รับบริการในประเทศไทยตามที่ระบุไว้ในเอกสารแนบท้ายสัญญา", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e3", "type": "text", "x": 0, "y": 230, "width": 50, "height": 40, "value": "ณัฐกานต์ ปฐพี", "style": {"fontFamily": "body", "fontSize": 11, "align": "justify"}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans", "Noto Sans Thai"]},
  "locale": "th",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+07:00",
  "version": "2.0"
}
`

// justifiedThaiDataJSON is the report data fixtures/justified-thai/
// renders against. The document binds nothing, for the same reason
// fixtures/justified-text/ binds nothing: justification is a property of
// the declared box and the packed line, and bound data would only add a
// way for the fixture to move for reasons that are not its subject.
const justifiedThaiDataJSON = `{}`
