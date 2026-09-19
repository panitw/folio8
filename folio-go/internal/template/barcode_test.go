package template

import (
	"strings"
	"testing"
)

func barcodeDoc(element string) []byte {
	return []byte(`{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        ` + element + `
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`)
}

func TestBarcodeLoadsAndSavesAs40(t *testing.T) {
	d, err := ParseDocument(barcodeDoc(`{"id": "e1", "type": "barcode", "x": 0, "y": 0, "width": 300, "height": 50, "value": "|0994000123456{{suffix}}\r{{ref1}}"}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	el := d.Bands.Content.Elements[0]
	if el.Type != ElementBarcode || !el.Value.Set || el.Value.Value != "|0994000123456{{suffix}}\r{{ref1}}" {
		t.Fatalf("decoded element = %+v", el)
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if !strings.Contains(string(out), `"version": "4.0"`) {
		t.Fatalf("a barcode document must declare 4.0:\n%s", out)
	}
	if !strings.Contains(string(out), `"value": "|0994000123456{{suffix}}\r{{ref1}}"`) {
		t.Fatalf("the carriage return must be written as a JSON escape:\n%s", out)
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
		t.Fatalf("a barcode document does not round-trip byte-identically:\n%s\n---\n%s", out, out2)
	}
}

func TestBarcodeNullValueLoads(t *testing.T) {
	d, err := ParseDocument(barcodeDoc(`{"id": "e1", "type": "barcode", "x": 0, "y": 0, "width": 300, "height": 50, "value": null}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if v := d.Bands.Content.Elements[0].Value; !v.Set || !v.Null {
		t.Fatalf("value = %+v, want present null", v)
	}
}

func TestBarcodeRefusals(t *testing.T) {
	for _, c := range []struct {
		name, element, field string
	}{
		{"missing value", `{"id": "e1", "type": "barcode", "x": 0, "y": 0, "width": 300, "height": 50}`, "value"},
		{"style block", `{"id": "e1", "type": "barcode", "x": 0, "y": 0, "width": 300, "height": 50, "value": "1", "style": {"background": "#ff0000"}}`, "style"},
		{"empty style block", `{"id": "e1", "type": "barcode", "x": 0, "y": 0, "width": 300, "height": 50, "value": "1", "style": {}}`, "style"},
		{"null style", `{"id": "e1", "type": "barcode", "x": 0, "y": 0, "width": 300, "height": 50, "value": "1", "style": null}`, "style"},
		{"font key", `{"id": "e1", "type": "barcode", "x": 0, "y": 0, "width": 300, "height": 50, "value": "1", "style": {"fontFamily": "body"}}`, "style"},
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

func TestDocumentWithoutBarcodeKeepsItsVersion(t *testing.T) {
	d, err := ParseDocument(barcodeDoc(`{"id": "e1", "type": "rect", "x": 0, "y": 0, "width": 300, "height": 50}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if !strings.Contains(string(out), `"version": "1.0"`) {
		t.Fatalf("a document without a barcode must keep 1.0:\n%s", out)
	}
}
