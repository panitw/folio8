package folio8

// qrcodePaymentsTemplateJSON is fixtures/qrcode-payments/input.folio, kept
// byte-identical to it by TestQRCodePaymentsGoldenFixture (the barcode
// fixture's hand-sync precedent).
//
// It is the qrcode element's golden (spec-barcode-qr-elements CAP-2): four QR
// codes on one page, one at each error-correction level. e1 carries a bound
// EMVCo Thai QR Payment payload with no `errorCorrection` key (the default
// M); e2 the same payload at H; e3 a Thai UTF-8 string with a bound reference
// at Q; e4 a static reference at L in a non-square box.
//
// It draws no text, so no face reaches the page: every byte that can differ
// between targets is the integer module geometry.
const qrcodePaymentsTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "qrcode", "x": 0, "y": 0, "width": 120, "height": 120, "value": "{{payload}}"},
        {"id": "e2", "type": "qrcode", "x": 160, "y": 0, "width": 120, "height": 120, "errorCorrection": "H", "value": "{{payload}}"},
        {"id": "e3", "type": "qrcode", "x": 320, "y": 0, "width": 120, "height": 120, "errorCorrection": "Q", "value": "ชำระเงิน {{ref}}"},
        {"id": "e4", "type": "qrcode", "x": 0, "y": 160, "width": 200, "height": 80, "errorCorrection": "L", "value": "INV-2026-000001"}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "th",
  "nextId": 5,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+07:00",
  "version": "4.0"
}
`

// qrcodePaymentsDataJSON is the record the fixture renders, kept
// byte-identical to fixtures/qrcode-payments/data.json.
const qrcodePaymentsDataJSON = `{"payload":"00020101021230810016A00000067701011201150994000123456780214INV2026000001030900000000153037645406150.005802TH62100706INV01263049A3F","ref":"INV-2026-000001"}
`

// qrcodePaymentsSymbols is each QR code's exact resolved string and level,
// in element order: what its modules must decode to.
var qrcodePaymentsSymbols = []struct {
	id, resolved, level string
	boxW, boxH          int64
}{
	{"e1", "00020101021230810016A00000067701011201150994000123456780214INV2026000001030900000000153037645406150.005802TH62100706INV01263049A3F", "M", 120000, 120000},
	{"e2", "00020101021230810016A00000067701011201150994000123456780214INV2026000001030900000000153037645406150.005802TH62100706INV01263049A3F", "H", 120000, 120000},
	{"e3", "ชำระเงิน INV-2026-000001", "Q", 120000, 120000},
	{"e4", "INV-2026-000001", "L", 200000, 80000},
}
