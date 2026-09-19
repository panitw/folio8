package folio8

import (
	"bytes"
	"os"
	"testing"
)

func TestSerializeTemplateUsesCanonicalEngineSerializer(t *testing.T) {
	input, err := os.ReadFile("testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := ParseTemplate(input)
	if err != nil {
		t.Fatal(err)
	}
	got, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, input) {
		t.Fatal("SerializeTemplate did not return the canonical engine serialization")
	}
	if bytes.Equal(got, append([]byte("\n  "), input...)) {
		t.Fatal("SerializeTemplate retained non-canonical caller bytes")
	}
}
