package folio8

// barcodeThaiBillPaymentTemplateJSON is
// fixtures/barcode-thai-bill-payment/input.folio, kept byte-identical to it by
// TestBarcodeThaiBillPaymentGoldenFixture (line-spacing's hand-sync
// precedent).
//
// It is the barcode element's golden (spec-barcode-qr-elements CAP-1): a
// Thai bill-payment payload whose static part carries the biller's tax id and
// three carriage-return separators, and whose suffix, references and amount
// come from the data. It is the first committed document declaring
// `"version": "4.0"`, because it is the first carrying a barcode.
//
// It draws no text, so no face reaches the page: every byte that can differ
// between targets is the integer bar geometry.
const barcodeThaiBillPaymentTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "barcode", "x": 0, "y": 0, "width": 400, "height": 50, "value": "|0994000123456{{suffix}}\r{{ref1}}\r{{ref2}}\r{{amount}}"}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+07:00",
  "version": "4.0"
}
`

// barcodeThaiBillPaymentDataJSON is the record the fixture renders, kept
// byte-identical to fixtures/barcode-thai-bill-payment/data.json.
const barcodeThaiBillPaymentDataJSON = `{"amount":"150000","ref1":"1234567890","ref2":"0000000001","suffix":"01"}
`

// barcodeThaiBillPaymentResolved is the exact string the bars must decode to:
// the value above with the data substituted, carriage returns included.
const barcodeThaiBillPaymentResolved = "|099400012345601\r1234567890\r0000000001\r150000"
