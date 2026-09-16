package folio8

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio8-go/internal/designer"
	"github.com/panitw/folio8/folio8-go/internal/template"
)

// TestBrowserAuthoredRoundTripWitness is deliberately dormant during the
// ordinary Go suite. Story 6.7's compiled Playwright proof creates a fresh
// directory, writes the opaque worker/download bytes there, and reruns this
// exact public-library test with FOLIO8_ROUNDTRIP_DIR set. Keeping the native
// half here means the browser cannot replace parse, serialization, template
// inspection, or rendering with a TypeScript lookalike.
func TestBrowserAuthoredRoundTripWitness(t *testing.T) {
	dir := os.Getenv("FOLIO8_ROUNDTRIP_DIR")
	if dir == "" {
		t.Skip("browser/native witness is supplied only by the Story 6.7 Playwright run")
	}

	golden := roundTripInputs(t, dir, "golden")
	alternate := roundTripInputs(t, dir, "alternate")
	assertRoundTripChain(t, "golden", golden)
	assertRoundTripChain(t, "alternate", alternate)
	assertCustomerStatementFacts(t, golden.template)

	if bytes.Equal(golden.template, alternate.template) {
		t.Fatal("alternate saved template copies the golden saved bytes")
	}
	assertDifferentGoOwnedShapes(t, golden.template, alternate.template)
	assertHandEditedNativeTemplate(t)
}

type roundTripWitness struct {
	template   []byte
	data       []byte
	params     []byte
	browserPDF []byte
}

func roundTripInputs(t *testing.T, dir, name string) roundTripWitness {
	t.Helper()
	read := func(suffix string) []byte {
		path := filepath.Join(dir, name+suffix)
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if len(b) == 0 {
			t.Fatalf("%s is empty", path)
		}
		return b
	}
	return roundTripWitness{
		template:   read(".folio"),
		data:       read(".data.json"),
		params:     read(".params.json"),
		browserPDF: read(".browser.pdf"),
	}
}

func assertRoundTripChain(t *testing.T, name string, witness roundTripWitness) {
	t.Helper()
	tpl, err := ParseTemplate(witness.template)
	if err != nil {
		t.Fatalf("%s: native ParseTemplate(saved bytes): %v", name, err)
	}
	canonical, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatalf("%s: native SerializeTemplate(saved bytes): %v", name, err)
	}
	if !bytes.Equal(canonical, witness.template) {
		t.Fatalf("%s: saved bytes are not the exact canonical bytes returned by the engine", name)
	}

	// testShippedFontSet is byte-parity checked against fonts.Shipped() by the
	// external-package shipped-face test; package folio8 test files cannot
	// import fonts without a Go import cycle.
	first, err := Render(tpl, Data(witness.data), Params(witness.params), testShippedFontSet())
	if err != nil {
		t.Fatalf("%s: native production Render: %v", name, err)
	}
	second, err := Render(tpl, Data(witness.data), Params(witness.params), testShippedFontSet())
	if err != nil {
		t.Fatalf("%s: repeated native production Render: %v", name, err)
	}
	if !bytes.Equal(first.Bytes, second.Bytes) {
		t.Fatalf("%s: repeated native production bytes differ: %s != %s", name, fingerprint(first.Bytes), fingerprint(second.Bytes))
	}
	if !bytes.Equal(first.Bytes, witness.browserPDF) {
		t.Fatalf("%s: admitted browser PDF and native production PDF differ: browser=%s native=%s", name, fingerprint(witness.browserPDF), fingerprint(first.Bytes))
	}
	if len(first.Diagnostics) != 0 {
		t.Fatalf("%s: authored success witness returned diagnostics: %+v", name, first.Diagnostics)
	}
}

func assertDifferentGoOwnedShapes(t *testing.T, golden, alternate []byte) {
	t.Helper()
	parse := func(name string, raw []byte) designer.CanvasProjection {
		tpl, err := ParseTemplate(raw)
		if err != nil {
			t.Fatalf("%s: ParseTemplate: %v", name, err)
		}
		projection, err := canvasWithTextPaint(tpl, testShippedFontSet())
		if err != nil {
			t.Fatalf("%s: CanvasWithTextPaint: %v", name, err)
		}
		return projection
	}
	left, right := parse("golden", golden), parse("alternate", alternate)
	shape := func(c designer.CanvasProjection) map[string]int {
		out := map[string]int{}
		for _, component := range c.Components {
			out[fmt.Sprintf("%s/%s", component.Band, component.Type)]++
		}
		return out
	}
	if fmt.Sprint(shape(left)) == fmt.Sprint(shape(right)) {
		t.Fatalf("Go-owned canvas facts show no structural difference: %v", shape(left))
	}
}

// assertCustomerStatementFacts reads the parsed Go-owned model, rather than
// selectors or a TypeScript projection. It keeps the browser witness tied to
// the actual report named in the Epic: image/logo, account framing, five
// transaction columns, and generated-date/page footer content.
func assertCustomerStatementFacts(t *testing.T, raw []byte) {
	t.Helper()
	tpl, err := ParseTemplate(raw)
	if err != nil {
		t.Fatalf("parse Customer Account Statement: %v", err)
	}
	doc := tpl.doc
	if len(doc.Assets) == 0 {
		t.Fatal("Customer Account Statement has no embedded logo/image asset")
	}
	var image, table bool
	for _, element := range doc.Bands.PageHeader.Elements {
		if element.Type == "image" {
			image = true
		}
	}
	for _, element := range doc.Bands.Content.Elements {
		if element.Type != "table" {
			continue
		}
		table = true
		if !element.Table.Set || len(element.Table.Value.Columns) != 5 {
			t.Fatalf("statement table has %d columns, want five", len(element.Table.Value.Columns))
		}
	}
	if !image {
		t.Fatal("Customer Account Statement has no logo/image in page header")
	}
	if !table {
		t.Fatal("Customer Account Statement has no transaction table")
	}
	text := func(elements []template.Element) string {
		var values []string
		for _, e := range elements {
			if e.Value.Set {
				values = append(values, e.Value.Value)
			}
		}
		return strings.Join(values, "\n")
	}
	content, footer := text(doc.Bands.Content.Elements), text(doc.Bands.PageFooter.Elements)
	for _, want := range []string{"customer.name", "account.number", "period.from"} {
		if !strings.Contains(content, want) {
			t.Fatalf("statement content omits %q: %q", want, content)
		}
	}
	for _, want := range []string{"params.generatedDate", "Page {{page}} of {{pages}}"} {
		if !strings.Contains(footer, want) {
			t.Fatalf("statement footer omits %q: %q", want, footer)
		}
	}
}

func assertHandEditedNativeTemplate(t *testing.T) {
	t.Helper()
	// This canonical, hand-authored report is intentionally read only by this
	// native suite. The Playwright source never uploads or opens this pathname.
	templatePath := filepath.Join("testdata", "template", "golden", "worked-example.json")
	template, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read hand-edited template: %v", err)
	}
	tpl, err := ParseTemplate(template)
	if err != nil {
		t.Fatalf("parse hand-edited template: %v", err)
	}
	canonical, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatalf("serialize hand-edited template: %v", err)
	}
	if !bytes.Equal(template, canonical) {
		t.Fatal("hand-edited template is not canonical; this is not the required canonical native witness")
	}
	data, err := os.ReadFile(filepath.Join("..", "fixtures", "statement-1", "data.json"))
	if err != nil {
		t.Fatalf("read hand-edited data: %v", err)
	}
	params := []byte(`{"generatedDate":"2026-08-27"}`)
	first, err := Render(tpl, Data(data), Params(params), testShippedFontSet())
	if err != nil {
		t.Fatalf("render hand-edited template: %v", err)
	}
	second, err := Render(tpl, Data(data), Params(params), testShippedFontSet())
	if err != nil {
		t.Fatalf("re-render hand-edited template: %v", err)
	}
	if len(first.Bytes) == 0 || !bytes.Equal(first.Bytes, second.Bytes) {
		t.Fatalf("hand-edited template is not deterministically renderable: first=%s second=%s", fingerprint(first.Bytes), fingerprint(second.Bytes))
	}
}

func fingerprint(b []byte) string {
	sum := sha256.Sum256(b)
	return fmt.Sprintf("len=%d sha256=%x", len(b), sum)
}
