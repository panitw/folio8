package template

import (
	"strings"
	"testing"
)

func TestQRCodeLoadsAndSavesAs40(t *testing.T) {
	d, err := ParseDocument(barcodeDoc(`{"id": "e1", "type": "qrcode", "x": 0, "y": 0, "width": 120, "height": 120, "errorCorrection": "H", "value": "ชำระเงิน {{ref}}"}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	el := d.Bands.Content.Elements[0]
	if el.Type != ElementQRCode || el.Value.Value != "ชำระเงิน {{ref}}" || !el.ErrorCorrection.Set || el.ErrorCorrection.Value != "H" {
		t.Fatalf("decoded element = %+v", el)
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	for _, want := range []string{`"version": "4.0"`, `"errorCorrection": "H"`, `"type": "qrcode"`} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("saved document lacks %s:\n%s", want, out)
		}
	}
	again, err := ParseDocument(out)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	out2, err := SerializeDocument(again)
	if err != nil {
		t.Fatalf("reserialize: %v", err)
	}
	if string(out) != string(out2) {
		t.Fatalf("a qrcode document does not round-trip byte-identically:\n%s\n---\n%s", out, out2)
	}
}

func TestQRCodeWithoutLevelSavesWithoutTheKey(t *testing.T) {
	d, err := ParseDocument(barcodeDoc(`{"id": "e1", "type": "qrcode", "x": 0, "y": 0, "width": 120, "height": 120, "value": "folio8"}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if d.Bands.Content.Elements[0].ErrorCorrection.Set {
		t.Fatal("an absent errorCorrection must stay absent")
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if strings.Contains(string(out), "errorCorrection") || !strings.Contains(string(out), `"version": "4.0"`) {
		t.Fatalf("want 4.0 and no errorCorrection key:\n%s", out)
	}
}

func TestQRCodeRefusals(t *testing.T) {
	for _, c := range []struct {
		name, element, field string
	}{
		{"unknown level", `{"id": "e1", "type": "qrcode", "x": 0, "y": 0, "width": 120, "height": 120, "value": "1", "errorCorrection": "X"}`, "errorCorrection"},
		{"lower-case level", `{"id": "e1", "type": "qrcode", "x": 0, "y": 0, "width": 120, "height": 120, "value": "1", "errorCorrection": "m"}`, "errorCorrection"},
		{"null level", `{"id": "e1", "type": "qrcode", "x": 0, "y": 0, "width": 120, "height": 120, "value": "1", "errorCorrection": null}`, "errorCorrection"},
		{"numeric level", `{"id": "e1", "type": "qrcode", "x": 0, "y": 0, "width": 120, "height": 120, "value": "1", "errorCorrection": 1}`, "errorCorrection"},
		{"level on a text", `{"id": "e1", "type": "text", "x": 0, "y": 0, "width": 120, "height": 120, "value": "1", "errorCorrection": "M"}`, "errorCorrection"},
		{"level on a barcode", `{"id": "e1", "type": "barcode", "x": 0, "y": 0, "width": 120, "height": 120, "value": "1", "errorCorrection": "M"}`, "errorCorrection"},
		{"missing value", `{"id": "e1", "type": "qrcode", "x": 0, "y": 0, "width": 120, "height": 120}`, "value"},
		{"style block", `{"id": "e1", "type": "qrcode", "x": 0, "y": 0, "width": 120, "height": 120, "value": "1", "style": {}}`, "style"},
		{"null style", `{"id": "e1", "type": "qrcode", "x": 0, "y": 0, "width": 120, "height": 120, "value": "1", "style": null}`, "style"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseDocument(barcodeDoc(c.element))
			le, ok := err.(*LoadError)
			if !ok {
				t.Fatalf("want a located *LoadError, got %T: %v", err, err)
			}
			if le.Field != c.field || le.ElementID != "e1" {
				t.Fatalf("refusal = field %q element %q, want %q e1", le.Field, le.ElementID, c.field)
			}
		})
	}
}

func TestQRErrorCorrectionSet(t *testing.T) {
	for _, s := range QRErrorCorrectionTokens {
		if !IsQRErrorCorrection(s) {
			t.Errorf("%q must be admitted", s)
		}
	}
	for _, s := range []string{"", "l", "X", "LM"} {
		if IsQRErrorCorrection(s) {
			t.Errorf("%q must be refused", s)
		}
	}
	if !IsQRErrorCorrection(QRErrorCorrectionDefault) {
		t.Error("the default must be a member")
	}
}
