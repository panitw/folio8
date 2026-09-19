package folio8

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// The number-in-text-binding spec (owner decision 2026-09-13, revising
// AD-14 for numbers in text only): a number resolving in a TEXT binding
// prints as its exact decimal in text elements and table data cells, with
// no grouping, locale or rounding. Footer cells are always formatted by
// formatNumber (unchanged).

func numberInTextDoc() string {
	text := func(id string, y int, value string) string {
		return fmt.Sprintf(`{"id": %q, "type": "text", "x": 0, "y": %d, "width": 180, "height": 10, "value": %q, "style": {"fontFamily": "latin", "fontSize": 8}}`, id, y, value)
	}
	return `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      ` + text("e1", 0, "{{v}}") + `,
      ` + text("e2", 12, "Total: {{ten}} THB") + `,
      ` + text("e3", 24, "Rows {{count(items)}}") + `,
      ` + text("e4", 36, "Sum {{a + b}}") + `,
      {"id": "e5", "type": "table", "x": 0, "y": 50, "bind": "items[]", "headerHeight": 10,
        "style": {"fontFamily": "latin", "fontSize": 8},
        "columns": [
          {"id": "e6", "label": "Amount", "width": 80, "bind": "{{row.amount}}", "footer": "sum"}
        ]}
    ]},
    "pageFooter": {"elements": [], "height": 10},
    "pageHeader": {"elements": [], "height": 10}
  },
  "fonts": {"latin": ["Noto Sans"]},
  "locale": "en",
  "nextId": 7,
  "page": {"margin": {"bottom": 10, "left": 10, "right": 10, "top": 10}, "orientation": "portrait", "size": {"width": 200, "height": 150}},
  "utcOffset": "+00:00",
  "version": "3.3"
}
`
}

func TestRenderPrintsNumbersInTextAsExactDecimals(t *testing.T) {
	tpl, err := ParseTemplate([]byte(numberInTextDoc()))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	data := `{"v": 1234.50, "ten": 10, "a": 1.5, "b": 2, "items": [{"amount": 99.9}, {"amount": 0.10}, {"amount": 1}]}`
	res, err := Render(tpl, Data(data), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	pages := pageTextsOf(t, res.Bytes)
	// Whole runs: one per text element or cell. "101.00" is the sum
	// footer, which formatNumber formats (unchanged).
	for _, want := range []string{"1234.50", "Total: 10 THB", "Rows 3", "Sum 3.5", "99.9", "0.10", "101.00"} {
		if !pageContains(pages, 0, want) {
			t.Errorf("the page does not draw the whole run %q; drawn runs: %q", want, pages[0])
		}
	}
}

func TestRenderBooleanInTextStaysLocatedError(t *testing.T) {
	tpl, err := ParseTemplate([]byte(numberInTextDoc()))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	_, err = Render(tpl, Data(`{"v": true, "ten": 10, "a": 1, "b": 2, "items": []}`), nil, testShippedFontSet())
	if err == nil || !strings.Contains(err.Error(), "e1") || !strings.Contains(err.Error(), "bool") {
		t.Fatalf("a boolean in text must be a located Error naming e1, got: %v", err)
	}
}

// TestNoDataPreviewPrintsNumberStandIns is the statement acceptance shape
// without sample data: a path used both bare and under formatNumber takes
// the zero stand-in and the preview renders.
func TestNoDataPreviewPrintsNumberStandIns(t *testing.T) {
	tpl := standInTemplate(t, standInTextElement("e1", `{{header.balance_available}}`)+`, `+
		standInTextElementAt("e2", 30, `{{formatNumber(header.balance_available, "#,##0.00")}}`))
	got := standInBytes(t, tpl)
	if string(got) != `{"header":{"balance_available":0}}` {
		t.Fatalf("stand-in document = %s", got)
	}
	if _, err := Render(tpl, Data(got), nil, testShippedFontSet()); err != nil {
		t.Fatalf("preview render: %v", err)
	}
}

func standInTextElementAt(id string, y int, value string) string {
	return fmt.Sprintf(`{"id": %q, "type": "text", "x": 0, "y": %d, "width": 460, "height": 20, "value": %q, "style": {"fontFamily": "body", "fontSize": 12}}`, id, y, value)
}

// TestTextNumberExpressionRaisesVersion33 is the save-time floor: a text
// expression statically known to return a number raises the document to
// 3.3; a plain path (kind depends on data) raises nothing.
func TestTextNumberExpressionRaisesVersion33(t *testing.T) {
	for _, tc := range []struct{ value, want string }{
		{`{{1}}`, "3.3"},
		{`Rows {{count(items)}}`, "3.3"},
		{`{{a + b}}`, "3.3"},
		{`{{avg(items.x)}}`, "3.3"},
		{`{{flag ? 1 : "none"}}`, "3.3"},
		{`{{v}}`, "1.0"},
		{`{{formatNumber(v, "0")}}`, "1.0"},
		{`{{true ? "Yes" : "No"}}`, "2.0"},
	} {
		tpl := formulaTemplate(t, "flag")
		tpl.doc.Bands.Content.Elements[0].Value.Value = tc.value
		saved, err := SerializeTemplate(tpl)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(saved, []byte(`"version": "`+tc.want+`"`)) {
			t.Errorf("%s: saved document does not declare %s", tc.value, tc.want)
		}
	}

	tpl, err := ParseTemplate([]byte(strings.Replace(numberInTextDoc(), `"version": "3.3"`, `"version": "1.0"`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	for i := range tpl.doc.Bands.Content.Elements[:4] {
		tpl.doc.Bands.Content.Elements[i].Value.Value = "{{v}}"
	}
	saved, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	// The fixture's table has no total width and no 3.x key, so its only
	// requirement is the loaded 1.0.
	if !bytes.Contains(saved, []byte(`"version": "1.0"`)) {
		t.Fatalf("a plain-path column bind must not raise to 3.3:\n%s", saved)
	}
	tpl.doc.Bands.Content.Elements[4].Table.Value.Columns[0].Bind = "{{row.amount * 2}}"
	saved, err = SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(saved, []byte(`"version": "3.3"`)) {
		t.Fatalf("a number-kind column bind must raise to 3.3:\n%s", saved)
	}
}
