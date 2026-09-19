package template

import (
	"strings"
	"testing"
)

func tableDoc(columnJSON string) []byte {
	return []byte(`{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {
          "as": "row",
          "bind": "transactions[]",
          "columns": [
` + columnJSON + `
          ],
          "headerHeight": 14,
          "id": "e2",
          "type": "table",
          "x": 0,
          "y": 0
        }
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 20
    },
    "pageHeader": {
      "elements": [],
      "height": 20
    }
  },
  "fonts": {},
  "locale": "en",
  "nextId": 4,
  "page": {
    "margin": {
      "bottom": 36,
      "left": 36,
      "right": 36,
      "top": 36
    },
    "orientation": "portrait",
    "size": "A4"
  },
  "utcOffset": "+00:00",
  "version": "1.0"
}
`)
}

// TestFooterOfWithCountIsLoadError is AC43 check 1 (D-1.4.2): footerOf
// present with footer: "count" -> load error (pure field presence).
func TestFooterOfWithCountIsLoadError(t *testing.T) {
	col := `            {
              "bind": "{{row.amount}}",
              "footer": "count",
              "footerOf": "transactions.amount",
              "id": "e3",
              "label": "Amount",
              "width": 80
            }`
	_, err := ParseDocument(tableDoc(col))
	if err == nil {
		t.Fatal(`expected a load error for footerOf present with footer: "count"`)
	}
}

// TestFooterOfOrFormatWithoutFooterIsLoadError is AC43 check 2.
func TestFooterOfOrFormatWithoutFooterIsLoadError(t *testing.T) {
	cases := []string{
		`            {
              "bind": "{{row.amount}}",
              "footerOf": "transactions.amount",
              "id": "e3",
              "label": "Amount",
              "width": 80
            }`,
		`            {
              "bind": "{{row.amount}}",
              "footerFormat": "#,##0.00",
              "id": "e3",
              "label": "Amount",
              "width": 80
            }`,
	}
	for i, col := range cases {
		if _, err := ParseDocument(tableDoc(col)); err == nil {
			t.Fatalf("case %d: expected a load error for footerOf/footerFormat present with no footer", i)
		}
	}
}

// TestFooterOfPrefixCheck is AC43 check 3: footerOf must be prefixed by
// the table's collection path + ".".
func TestFooterOfPrefixCheck(t *testing.T) {
	good := `            {
              "bind": "{{row.amount}}",
              "footer": "sum",
              "footerOf": "transactions.amount",
              "id": "e3",
              "label": "Amount",
              "width": 80
            }`
	if _, err := ParseDocument(tableDoc(good)); err != nil {
		t.Fatalf("expected footerOf prefixed by the table's collection path to load, got: %v", err)
	}

	bad := `            {
              "bind": "{{row.amount}}",
              "footer": "sum",
              "footerOf": "wrong.amount",
              "id": "e3",
              "label": "Amount",
              "width": 80
            }`
	if _, err := ParseDocument(tableDoc(bad)); err == nil {
		t.Fatal("expected a load error for footerOf not prefixed by the table's collection path")
	}
}

// TestFooterWithoutFooterOfLoads is AC44, RE-FRAMED at Story 3.2
// (Decision 1, this story): this is no longer a known gap pending a
// future story — it is a DELIBERATE LAYER BOUNDARY. F2 proves
// internal/template (stage rank 2) can never import internal/expr
// (rank 3), so template.ParseDocument itself can never check whether a
// footer's bind is derivable, and it never will: that check, and
// D-1.4.1's footerOf/footerFormat derivation, live at the module root,
// in folio8.ParseTemplate (folio-go/folio8_expr_validate.go's
// validateAndDeriveExpressions/validateTableColumns, invoked from
// ParseTemplate — folio-go/folio8.go). template.ParseDocument staying
// permissive here is that boundary's OTHER side, not a leftover hole:
// internal/template's own callers (this package's own tests included)
// get exactly the schema-level checks D-1.4.2 assigned to this layer
// (AC43) and nothing about bind-shape derivability, which belongs one
// layer up. See TestParseTemplateRejectsNonDerivableFooterBind and
// TestParseTemplateDerivesFooterOfShape1/Shape2 (folio-go's own
// folio8_expr_validate_test.go) for the STRENGTHENED assertion added at
// the root, alongside this one — this fixture is kept, not relocated,
// because it remains a true statement about THIS layer (Decision 1,
// this story: deleting it would unpin template's own permissiveness
// and let a later story change the layering unnoticed).
func TestFooterWithoutFooterOfLoads(t *testing.T) {
	col := `            {
              "bind": "{{row.amount}}",
              "footer": "sum",
              "id": "e3",
              "label": "Amount",
              "width": 80
            }`
	if _, err := ParseDocument(tableDoc(col)); err != nil {
		t.Fatalf("a footer with no footerOf must load today (D-1.4.2, Story 3.2 derives it later): %v", err)
	}
}

// TestFooterKeyOrderIsByteOrder is AC42/M-4: footerFormat sorts before
// footerOf ('F' 0x46 < 'O' 0x4F).
func TestFooterKeyOrderIsByteOrder(t *testing.T) {
	col := `            {
              "align": "right",
              "bind": "{{row.amount}}",
              "footer": "sum",
              "footerFormat": "#,##0.00",
              "footerOf": "transactions.amount",
              "id": "e3",
              "label": "Amount",
              "width": 80
            }`
	d, err := ParseDocument(tableDoc(col))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	fIdx := strings.Index(string(out), `"footerFormat"`)
	oIdx := strings.Index(string(out), `"footerOf"`)
	if fIdx == -1 || oIdx == -1 || fIdx > oIdx {
		t.Fatalf("expected footerFormat to sort before footerOf, got:\n%s", out)
	}
}
