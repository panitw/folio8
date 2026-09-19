package folio8

// wrappedTextTemplateJSON is fixtures/wrapped-text/input.folio, kept
// byte-identical to it by TestWrappedTextGoldenFixture (the same
// hand-sync precedent font-text and multi-script-fallback set).
//
// It exists to exercise Story 2.4's line breaking in all three scripts
// plus the declared-unbreakable mechanism, in ONE document:
//
//	e1  Latin, 150 pt box at 11 pt -> 3 lines, breaking at whitespace.
//	e2  Thai, 150 pt box -> 2 lines, breaking at dictionary boundaries.
//	e3  CJK, 150 pt box -> 2 lines, breaking between Han characters.
//	e4  the declared span, 20 pt box -> 2 lines. THIS IS THE
//	    DISCRIMINATING ELEMENT: its box (20,000 mp) is deliberately
//	    NARROWER than the bound value "ศรีสุข" measures
//	    (24,585 mp), so without the unbreakableValues declaration the
//	    surname SPLITS into two lines at the dictionary seam between
//	    its two halves, and with it the value stays whole and overflows
//	    its box visibly. Both polarities are asserted; the "declared"
//	    case alone would be vacuous (AC7).
//
// Every one of those line counts is MEASURED, not assumed — see
// TestWrappedTextSemanticAcceptance.
const wrappedTextTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 150, "height": 60, "value": "The quick brown fox jumps over the lazy dog near the river bank", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e2", "type": "text", "x": 0, "y": 80, "width": 150, "height": 60, "value": "ใบเสร็จรับเงินค่าน้ำประปาประจำเดือนมกราคม", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e3", "type": "text", "x": 0, "y": 160, "width": 150, "height": 60, "value": "结算单共三页请核对每一行的金额与日期", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e4", "type": "text", "x": 0, "y": 240, "width": 20, "height": 40, "value": "ผู้รับ {{customer.name}}", "style": {"fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans", "Noto Sans Thai", "Noto Sans SC"]},
  "locale": "th",
  "nextId": 5,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "unbreakableValues": ["customer.name"],
  "utcOffset": "+07:00",
  "version": "1.0"
}
`

// wrappedTextDataJSON is the report data fixtures/wrapped-text/ renders
// against. customer.name is D-2.1.6's worked case: a surname that is
// character-for-character two common dictionary words.
const wrappedTextDataJSON = `{"customer":{"name":"ศรีสุข"}}`
