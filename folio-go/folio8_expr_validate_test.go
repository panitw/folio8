package folio8

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/template"
)

// footerDocJSON builds a minimal single-column table document whose
// column requests a "sum" footer with footerOf OMITTED, so ParseTemplate
// must attempt D-1.4.1's derivation. bindExpr is the column's raw
// "bind" value (already including the surrounding "{{ }}"), alias is
// the table's declared row alias ("as"; omitted when empty, defaulting
// to "row" per D-3.1.1).
func footerDocJSON(alias, bindExpr string) string {
	asField := ""
	if alias != "" {
		asField = `"as": "` + alias + `", `
	}
	return `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e2", "type": "table", "x": 0, "y": 0, ` + asField + `"bind": "transactions[]", "headerHeight": 14,
          "columns": [
            {"id": "e3", "label": "Amount", "width": 80, "bind": "` + bindExpr + `", "footer": "sum"}
          ]}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

// TestParseTemplateDerivesFooterOfShape1 is AC21's derivable arm,
// shape 1: a bare row-scoped path. Asserts the RESOLVED VALUE, not
// merely that the document loaded (D-000.59's own anti-vacuity
// clause).
func TestParseTemplateDerivesFooterOfShape1(t *testing.T) {
	tpl, err := ParseTemplate([]byte(footerDocJSON("row", `{{row.amount}}`)))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	got, ok := tpl.derivedFooters["e3"]
	if !ok {
		t.Fatal("expected a derived footerOf for column e3")
	}
	if got.FooterOf != "transactions.amount" {
		t.Errorf("FooterOf = %q, want transactions.amount", got.FooterOf)
	}
	if got.HasFooterFormat {
		t.Errorf("shape 1 must not derive a footerFormat, got %+v", got)
	}
}

// TestParseTemplateDerivesFooterOfShape2 is AC21's derivable arm,
// shape 2, using the canonical golden's own alias/pattern (D-1.4.10):
// footerFormat also resolves to the derived default.
func TestParseTemplateDerivesFooterOfShape2(t *testing.T) {
	tpl, err := ParseTemplate([]byte(footerDocJSON("transaction", `{{formatNumber(transaction.amount, \"#,##0.00\")}}`)))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	got, ok := tpl.derivedFooters["e3"]
	if !ok {
		t.Fatal("expected a derived footerOf for column e3")
	}
	if got.FooterOf != "transactions.amount" {
		t.Errorf("FooterOf = %q, want transactions.amount", got.FooterOf)
	}
	if !got.HasFooterFormat || got.FooterFormat != "#,##0.00" {
		t.Errorf("expected footerFormat to default to #,##0.00, got %+v", got)
	}
}

// TestParseTemplateRejectsNonDerivableFooterBind is AC21's rejection
// arm: a footer requested with footerOf omitted, over a bind that is
// neither derivable shape, is a load error naming the column id.
func TestParseTemplateRejectsNonDerivableFooterBind(t *testing.T) {
	_, err := ParseTemplate([]byte(footerDocJSON("row", `{{if(row.x, row.a, row.b)}}`)))
	if err == nil {
		t.Fatal("expected a load error: bind is not one of D-1.4.1's two derivable shapes")
	}
	if !strings.Contains(err.Error(), "e3") {
		t.Errorf("error must name the column id, got: %v", err)
	}
}

// TestParseTemplateFooterOfDerivationNeverWritesBack is R2/D-1.4.3's
// P3 fixed point: Serialize(Parse(b)) == b must still hold, byte for
// byte, for a document whose footer derives footerOf rather than
// declaring it — the derived value is resolved ALONGSIDE the document,
// never written into it.
//
// QA Finding 2 (Blocker): the earlier version of this test compared
// SerializeDocument(doc) against itself, and separately inspected two
// documents ParseTemplate's derivation never touched — a fresh
// template.ParseDocument(original) before the derivation ran, and
// another fresh template.ParseDocument(original) after. A write-back
// inside validateTableColumns lands in the *Document ParseTemplate
// itself parsed and walked (validateAndDeriveExpressions receives that
// exact pointer, and TableExt.Columns is a slice — a value receiver
// still shares its backing array), which neither comparison could ever
// see. Fixed by serializing tpl.doc — the actual document the
// derivation walked over — and comparing it against an independently
// derived canonical form.
func TestParseTemplateFooterOfDerivationNeverWritesBack(t *testing.T) {
	original := []byte(footerDocJSON("row", `{{row.amount}}`))

	// canonical is P3's fixed point, established independently of the
	// derivation path: an ordinary parse+serialize of original, which
	// never calls expr.DeriveFooterOf and so can never write anything
	// back. This also pins the fixture's own starting shape.
	canonicalDoc, err := template.ParseDocument(original)
	if err != nil {
		t.Fatalf("template.ParseDocument: %v", err)
	}
	canonical, err := template.SerializeDocument(canonicalDoc)
	if err != nil {
		t.Fatalf("SerializeDocument (canonical): %v", err)
	}
	if strings.Contains(string(canonical), `"footerOf"`) {
		t.Fatalf("fixture must omit footerOf by construction, got:\n%s", canonical)
	}

	// tpl.doc is the ACTUAL *template.Document ParseTemplate's
	// derivation walked over — the one place a write-back would show.
	tpl, err := ParseTemplate(original)
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	derived, err := template.SerializeDocument(tpl.doc)
	if err != nil {
		t.Fatalf("SerializeDocument (tpl.doc): %v", err)
	}
	if string(derived) != string(canonical) {
		t.Fatalf("Serialize(ParseTemplate(b).doc) != Serialize(Parse(b)) — P3 violated by a write-back (R2):\nderived:   %s\ncanonical: %s", derived, canonical)
	}
	if strings.Contains(string(derived), `"footerOf"`) {
		t.Fatalf("derivation must NEVER be written back into the document (R2): serialized output contains \"footerOf\":\n%s", derived)
	}
}

// TestParseTemplateRejectsSyntaxErrorInTextValue is AC19 through the
// public API: a syntax error inside "{{ }}" fails at LOAD (ParseTemplate),
// naming the element id and the offending text.
func TestParseTemplateRejectsSyntaxErrorInTextValue(t *testing.T) {
	const tplJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 20, "value": "{{nosuch(a + b)}}", "style": {"fontFamily": "body", "fontSize": 14}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	_, err := ParseTemplate([]byte(tplJSON))
	if err == nil {
		t.Fatal("expected a load error for a syntax-invalid expression")
	}
	if !strings.Contains(err.Error(), "e1") {
		t.Errorf("error must name the element id, got: %v", err)
	}
	// QA Finding 14 (Minor): AC19 requires the offending expression
	// text VERBATIM, not merely "an error occurred somewhere in e1" —
	// this is the half of the epic's third Given that makes the
	// diagnostic useful to a template author.
	if !strings.Contains(err.Error(), "nosuch(a + b)") {
		t.Errorf("error must carry the offending expression text %q, got: %v", "nosuch(a + b)", err)
	}
}

// TestParseTemplateAcceptsNumberKindTextValue: since the number-in-text
// spec (2026-09-13) a number-kind text expression loads; a boolean one
// stays a load error.
func TestParseTemplateAcceptsNumberKindTextValue(t *testing.T) {
	doc := func(value string) []byte {
		return []byte(`{
  "assets": {},
  "bands": {
    "content": {"elements": [{"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 20, "value": "` + value + `", "style": {"fontFamily": "body", "fontSize": 14}}]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "3.3"
}`)
	}
	for _, value := range []string{"{{a + b}}", "{{1}}", "{{count(items)}}"} {
		if _, err := ParseTemplate(doc(value)); err != nil {
			t.Errorf("%s must load: %v", value, err)
		}
	}
	if _, err := ParseTemplate(doc("{{true}}")); err == nil {
		t.Error("{{true}} must stay a load error")
	}
}

// TestParseTemplateRejectsSyntaxErrorInVisibleIf is QA Finding 12
// (Minor) through the public API: visibleIf is a BARE expression (no
// "{{ }}" — folio-format.md's field table, and fixtures_test.go:298's
// "visibleIf": "customer.hasTransactions"), never walked by
// validateAndDeriveExpressions before this fix, so a malformed
// visibleIf loaded clean. It must now fail at LOAD, naming the element
// id, exactly like a malformed text value.
func TestParseTemplateRejectsSyntaxErrorInVisibleIf(t *testing.T) {
	const tplJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 20, "value": "static text", "visibleIf": "a + b", "style": {"fontFamily": "body", "fontSize": 14}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	_, err := ParseTemplate([]byte(tplJSON))
	if err == nil {
		t.Fatal("expected a load error: visibleIf's grammar has no operators, same as any other expression")
	}
	if !strings.Contains(err.Error(), "e1") {
		t.Errorf("error must name the element id, got: %v", err)
	}
	if !strings.Contains(err.Error(), "visibleIf") {
		t.Errorf("error should identify visibleIf as the offending field, got: %v", err)
	}
}

// TestParseTemplateAcceptsBareVisibleIfExpression confirms the
// positive case: an ordinary bare-path visibleIf, exactly the shape
// fixtures_test.go's own golden already uses, must still load.
func TestParseTemplateAcceptsBareVisibleIfExpression(t *testing.T) {
	const tplJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 20, "value": "static text", "visibleIf": "customer.hasTransactions", "style": {"fontFamily": "body", "fontSize": 14}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	if _, err := ParseTemplate([]byte(tplJSON)); err != nil {
		t.Fatalf("expected the bare-path visibleIf to load, got: %v", err)
	}
}

// TestParseTemplateRejectsOversizedNumberLiteralAtLoad is R3/QA
// Finding 7 (Major) through the public API: a numeric literal whose
// bounds NewDecimal would reject must fail at LOAD (ParseTemplate),
// not survive to Render. Before this fix, this exact document loaded
// with no error at all — the failure mode R3/F3 were written to keep
// apart.
func TestParseTemplateRejectsOversizedNumberLiteralAtLoad(t *testing.T) {
	const tplJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 20, "value": "{{upper(12345678901234567890123456789)}}", "style": {"fontFamily": "body", "fontSize": 14}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	_, err := ParseTemplate([]byte(tplJSON))
	if err == nil {
		t.Fatal("expected a load error: a 29-digit coefficient exceeds NewDecimal's own bound and must not survive to render")
	}
	if !strings.Contains(err.Error(), "e1") {
		t.Errorf("error must name the element id, got: %v", err)
	}
}

// TestParseTemplateRejectsWrongArityAtLoad is AC10 through the public
// API: sum(a, b) is a located parse error at LOAD, not an
// unimplemented-function error at evaluation.
func TestParseTemplateRejectsWrongArityAtLoad(t *testing.T) {
	const tplJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 20, "value": "{{sum(a, b)}}", "style": {"fontFamily": "body", "fontSize": 14}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	_, err := ParseTemplate([]byte(tplJSON))
	if err == nil {
		t.Fatal("expected a load error: sum() takes 1 argument")
	}
	if !strings.Contains(err.Error(), "e1") {
		t.Errorf("error must name the element id, got: %v", err)
	}
	// QA Finding 14 (Minor): the offending text, verbatim.
	if !strings.Contains(err.Error(), "sum(a, b)") {
		t.Errorf("error must carry the offending expression text %q, got: %v", "sum(a, b)", err)
	}
}

// TestParseTemplateRejectsExcessiveCallNestingAtLoad is QA Finding 3's
// (Blocker) load-entry-point half: reproduces the reviewer's real
// trigger — a single text element whose value nests ~800,000 function
// calls, ~1.6MB, an unremarkable size for a .folio file — through the
// actual public API, folio8.ParseTemplate, load's real entry point.
// Before internal/expr's maxCallDepth existed this input drove
// unbounded recursion into an unrecoverable runtime stack overflow;
// now it must surface as an ordinary located load error naming the
// element id, like every other rejected form.
func TestParseTemplateRejectsExcessiveCallNestingAtLoad(t *testing.T) {
	deepValue, err := json.Marshal("{{" + strings.Repeat("a(", 800000) + "}}")
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	tplJSON := `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 20, "value": ` + string(deepValue) + `, "style": {"fontFamily": "body", "fontSize": 14}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	_, err = ParseTemplate([]byte(tplJSON))
	if err == nil {
		t.Fatal("expected a load error: excessive call nesting must be rejected, not overflow the stack")
	}
	if !strings.Contains(err.Error(), "e1") {
		t.Errorf("error must name the element id, got: %v", err)
	}
	if !strings.Contains(err.Error(), "source bytes") {
		t.Errorf("error must name the depth limit specifically, got: %v", err)
	}
}

// TestParseTemplateAcceptsCanonicalGolden is F3's forcing case, proved
// directly: the canonical worked example binds a formatNumber(...)
// call (D-1.4.1 shape 2, now fully implemented as of Story 3.4) and
// must still load, deriving its footer as before.
func TestParseTemplateAcceptsCanonicalGolden(t *testing.T) {
	tpl, err := LoadTemplate("testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatalf("the canonical golden must still load under Story 3.2's load-time expression validation: %v", err)
	}
	got, ok := tpl.derivedFooters["e4"]
	if !ok {
		t.Fatal("expected the golden's own footer column (e4) to derive footerOf")
	}
	if got.FooterOf != "transactions.amount" {
		t.Errorf("FooterOf = %q, want transactions.amount", got.FooterOf)
	}
	if !got.HasFooterFormat || got.FooterFormat != "#,##0.00" {
		t.Errorf("expected footerFormat to default to #,##0.00, got %+v", got)
	}
}

// TestParseTemplateRejectsLiteralVisibleIf is Story 3.5's AC6/
// DECISION-2: a bare literal condition can NEVER resolve to a
// boolean — the grammar has no boolean literal — so it is rejected at
// LOAD, closing the asymmetry with if()'s own condition slot
// (argNotLiteral). Both a number literal and a string literal are
// checked, matching the story's own two named subjects.
func TestParseTemplateRejectsLiteralVisibleIf(t *testing.T) {
	tplJSON := func(literal string) string {
		return `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 20, "value": "static text", "visibleIf": ` + literal + `, "style": {"fontFamily": "body", "fontSize": 14}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	}

	cases := []struct {
		name    string
		literal string // JSON string VALUE for visibleIf: e.g. "\"42\"" encodes the bare expression 42
	}{
		{name: "number literal", literal: `"42"`},
		{name: "string literal", literal: `"\"hello\""`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseTemplate([]byte(tplJSON(c.literal)))
			if err == nil {
				t.Fatalf("expected a load error: a bare literal condition can never be a boolean")
			}
			if !strings.Contains(err.Error(), "e1") {
				t.Errorf("error must name the element id, got: %v", err)
			}
			if !strings.Contains(err.Error(), "boolean or null") {
				t.Errorf("error must identify the literal as the defect, got: %v", err)
			}
		})
	}
}

// TestIfConditionStillRejectsLiteralAfterVisibleIfSharesThePredicate is
// AC6's red-proof second half: if()'s OWN literal-condition rejection
// must still fire after checkVisibleIfExpression starts sharing
// expr.IsLiteralExpr with it — proving the hoisted predicate serves
// BOTH call sites rather than one having quietly regressed.
func TestIfConditionStillRejectsLiteralAfterVisibleIfSharesThePredicate(t *testing.T) {
	const tplJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 20, "value": "{{if(42, \"a\", \"b\")}}", "style": {"fontFamily": "body", "fontSize": 14}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	_, err := ParseTemplate([]byte(tplJSON))
	if err == nil {
		t.Fatal("expected a load error: if()'s condition slot must still reject a bare literal")
	}
	if !strings.Contains(err.Error(), "must be a boolean or null") {
		t.Errorf("error must carry if()'s own argNotLiteral wording, got: %v", err)
	}
}

// TestParseTemplateRejectsVisibleIfOnTableColumn is AC3: a load error
// naming the column id, closing the live spec/code divergence measured
// at this story's creation (a column-level visibleIf loaded clean,
// absorbed opaquely into Column.Extra).
func TestParseTemplateRejectsVisibleIfOnTableColumn(t *testing.T) {
	const tplJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e2", "type": "table", "x": 0, "y": 0, "bind": "transactions[]", "headerHeight": 14,
          "columns": [
            {"id": "e3", "label": "Amount", "width": 80, "bind": "{{transaction.amount}}", "visibleIf": "transaction.isVisible"}
          ]}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	_, err := ParseTemplate([]byte(tplJSON))
	if err == nil {
		t.Fatal("expected a load error: visibility applies to elements only, never a table column")
	}
	if !strings.Contains(err.Error(), "e3") {
		t.Errorf("error must name the offending column id, got: %v", err)
	}
	if !strings.Contains(err.Error(), "elements only") {
		t.Errorf("error should state the reason (visibility applies to elements only), got: %v", err)
	}
}

// TestParseTemplateRejectsPlaceholderInStyleField is DECISION-1
// (ruled): a "{{ }}" placeholder inside a style string field is a
// located load error, naming the element and the field — NOT style-
// field validation in general (a colour's #RRGGBB shape is checked by the
// loader itself, internal/template's IsHexColour).
func TestParseTemplateRejectsPlaceholderInStyleField(t *testing.T) {
	const tplJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 20, "value": "static text",
          "style": {"fontFamily": "body", "fontSize": 14, "background": "{{if(customer.overdue, \"#FF0000\", \"#00FF00\")}}"}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	_, err := ParseTemplate([]byte(tplJSON))
	if err == nil {
		t.Fatal("expected a load error: conditional/data-driven styling is not supported")
	}
	if !strings.Contains(err.Error(), "e1") {
		t.Errorf("error must name the element id, got: %v", err)
	}
	if !strings.Contains(err.Error(), "background") {
		t.Errorf("error must name the offending style field, got: %v", err)
	}
}

// TestParseTemplateAcceptsOrdinaryStyleValues is the companion negative
// case: ordinary style values with no placeholder must still load clean.
// A colour must be #RRGGBB (checked by the loader since 2026-09-17), so
// the one here is well-formed; everything else in the block is still
// unvalidated beyond its own closed set.
func TestParseTemplateAcceptsOrdinaryStyleValues(t *testing.T) {
	const tplJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 20, "value": "static text",
          "style": {"fontFamily": "body", "fontSize": 14, "background": "#ABCDEF", "align": "left"}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	if _, err := ParseTemplate([]byte(tplJSON)); err != nil {
		t.Fatalf("ordinary style values must load; expected no error, got: %v", err)
	}
}

// TestParseTemplateRejectsPlaceholderInAltRowBackground is Story 3.5
// finisher review, Finding 4 (Major): D-3.5.2's ruling covers "any
// style string field", not "any style.* string field", and
// table.altRowBackground (FR28, folio-format.md's ONLY colour field
// outside element.style) loaded clean before this fix — the exact
// worked example D-3.5.2 itself gave ("a colour as 'whatever
// if(overdue, red, black) says'").
func TestParseTemplateRejectsPlaceholderInAltRowBackground(t *testing.T) {
	const tplJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e2", "type": "table", "x": 0, "y": 0, "bind": "transactions[]", "headerHeight": 14,
          "altRowBackground": "{{if(customer.overdue, \"#FF0000\", \"#00FF00\")}}",
          "columns": [
            {"id": "e3", "label": "Amount", "width": 80, "bind": "{{transaction.amount}}"}
          ]}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	_, err := ParseTemplate([]byte(tplJSON))
	if err == nil {
		t.Fatal("expected a load error: table.altRowBackground must not carry a \"{{ }}\" placeholder, same fence as element.style")
	}
	if !strings.Contains(err.Error(), "e2") {
		t.Errorf("error must name the offending element id, got: %v", err)
	}
	if !strings.Contains(err.Error(), "altRowBackground") {
		t.Errorf("error must name the offending field, got: %v", err)
	}
}

// TestParseTemplateRejectsPlaceholderInHeaderStyle is Story 4.1's
// second Style attach point going through the SAME fence as
// element.style and table.altRowBackground above: headerStyle reuses
// template.Style verbatim, so a "{{ }}" placeholder inside one of its
// string fields is a load error, not a silently-inert value.
//
// The fixture carries a "style" block as well as "headerStyle"
// (finisher fix, Story 4.1 review Finding 5): without a sibling
// "style" block, a message that (bug) names "style.background"
// instead of "headerStyle.background" would satisfy this test just as
// well as the correct one — the ambiguity Finding 5 found was
// invisible precisely because the original fixture had no "style"
// block to be confused with. Asserting "headerStyle" by name, not
// merely "background", is what makes the assertion discriminate.
func TestParseTemplateRejectsPlaceholderInHeaderStyle(t *testing.T) {
	const tplJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e2", "type": "table", "x": 0, "y": 0, "bind": "transactions[]", "headerHeight": 14,
          "style": {"background": "#EFEFEF"},
          "headerStyle": {"background": "{{if(customer.overdue, \"#FF0000\", \"#00FF00\")}}"},
          "columns": [
            {"id": "e3", "label": "Amount", "width": 80, "bind": "{{transaction.amount}}"}
          ]}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	_, err := ParseTemplate([]byte(tplJSON))
	if err == nil {
		t.Fatal("expected a load error: table.headerStyle.background must not carry a \"{{ }}\" placeholder, same fence as element.style")
	}
	if !strings.Contains(err.Error(), "e2") {
		t.Errorf("error must name the offending element id, got: %v", err)
	}
	if !strings.Contains(err.Error(), "headerStyle.background") {
		t.Errorf("error must name the offending block AND field as \"headerStyle.background\" — a message naming the sibling \"style.background\" must NOT satisfy this, got: %v", err)
	}
}

// TestParseTemplateAcceptsOrdinaryAltRowBackground is the companion
// negative case: an ordinary altRowBackground value with no
// placeholder — including a plain literal hex colour, which this check
// does NOT validate — must still load clean.
func TestParseTemplateAcceptsOrdinaryAltRowBackground(t *testing.T) {
	const tplJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e2", "type": "table", "x": 0, "y": 0, "bind": "transactions[]", "headerHeight": 14,
          "altRowBackground": "#EFEFEF",
          "columns": [
            {"id": "e3", "label": "Amount", "width": 80, "bind": "{{transaction.amount}}"}
          ]}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	if _, err := ParseTemplate([]byte(tplJSON)); err != nil {
		t.Fatalf("altRowBackground remains otherwise unvalidated; expected no error, got: %v", err)
	}
}

// styleStringFieldExclusions is the ONLY hand-written list this
// completeness test carries, and it is an EXCLUSION list, not an
// inclusion list (Story 3.5 finisher review, Finding 4 / Major,
// D-000.67): every Presence[string]/Presence[[]string] field on
// template.Style, template.Border, template.TableExt and
// template.Column is a CANDIDATE by construction (reflectStyleStringFields
// below finds it automatically); a field is excluded from
// checkStyleHasNoPlaceholders/table.altRowBackground's coverage only by
// being named HERE, with a reason — so a schema field that is neither
// checked nor excluded fails the test loudly instead of silently
// passing uncovered, which is exactly how table.altRowBackground went
// unchecked before this fix.
var styleStringFieldExclusions = map[string]string{
	"TableExt.As": "a row alias identifier (the table's own {{ }}-free binding namespace), " +
		"not an appearance property — rejecting a placeholder here under \"conditional/data-driven " +
		"styling is not supported\" would be the wrong message for the wrong reason",
	"Column.Align": "governed by its own closed-set check (parse_bands.go: must be one of " +
		"left/center/right) — a placeholder value is already rejected, just not under this message",
	"Column.HeaderAlign": "governed by its own closed-set check (parse_bands.go: ColumnHeaderAlignTokens, " +
		"left/center/right) — a placeholder value is already rejected, just not under this message",
	"Column.Footer":   "a closed enum (\"count\"/\"sum\"/\"avg\"), not a colour or appearance value",
	"Column.FooterOf": "a column-id reference, not an appearance property",
	"Column.FooterFormat": "a number/date format pattern (Story 3.4), not a colour — \"conditional " +
		"formatting\" in AC4's sense is about appearance, not about which locale pattern a footer uses",
}

// reflectStyleStringFields walks structType (a struct value, not a
// pointer) and returns "StructName.FieldName" for every exported field
// whose declared type is the shape Presence[string] or
// Presence[[]string] — detected STRUCTURALLY (a struct with bool Set,
// bool Null and a Value field of the right kind), not by matching the
// generic instantiation's printed name, so it is robust to Presence's
// own internals moving. "Extra" (every one of these four structs' own
// opaque-passthrough field) is skipped by name, the same way
// extraFields' own callers already treat it as not a declared field.
func reflectStyleStringFields(structName string, v any) []string {
	t := reflect.TypeOf(v)
	var found []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() || f.Name == "Extra" {
			continue
		}
		ft := f.Type
		if ft.Kind() != reflect.Struct {
			continue
		}
		setF, ok1 := ft.FieldByName("Set")
		nullF, ok2 := ft.FieldByName("Null")
		valF, ok3 := ft.FieldByName("Value")
		if !ok1 || !ok2 || !ok3 || setF.Type.Kind() != reflect.Bool || nullF.Type.Kind() != reflect.Bool {
			continue // not a Presence[T] field at all
		}
		isString := valF.Type.Kind() == reflect.String
		isStringSlice := valF.Type.Kind() == reflect.Slice && valF.Type.Elem().Kind() == reflect.String
		if !isString && !isStringSlice {
			continue // e.g. Presence[bool], Presence[geom.Length], Presence[Border]
		}
		found = append(found, structName+"."+f.Name)
	}
	return found
}

// TestStyleStringFieldPopulationMatchesSchema is Finding 4's actual
// fix to D-000.67: it DERIVES the population of string-valued
// style/appearance-shaped fields from the schema (template.Style,
// template.Border, template.TableExt, template.Column) by reflection,
// subtracts styleStringFieldExclusions (each named with a reason), and
// asserts what remains is EXACTLY the set checkStyleHasNoPlaceholders
// and the table.altRowBackground call in validateAndDeriveExpressions
// together cover. An ELEVENTH Presence[string] field added to any of
// the four structs in the future is neither checked nor excluded by
// name, so it fails this test loudly — the schema is the source of
// truth, never a hand-written inclusion list.
func TestStyleStringFieldPopulationMatchesSchema(t *testing.T) {
	var all []string
	all = append(all, reflectStyleStringFields("Style", template.Style{})...)
	all = append(all, reflectStyleStringFields("Border", template.Border{})...)
	all = append(all, reflectStyleStringFields("TableExt", template.TableExt{})...)
	all = append(all, reflectStyleStringFields("Column", template.Column{})...)

	if len(all) == 0 {
		t.Fatal("vacuity guard: reflection found zero Presence[string]/Presence[[]string] fields across all four structs — the walk itself is broken")
	}

	wantChecked := map[string]bool{
		"Style.Align":               true,
		"Style.Background":          true,
		"Style.Color":               true,
		"Style.FontFamily":          true,
		"Style.Valign":              true,
		"Border.Color":              true,
		"Border.Edges":              true,
		"TableExt.AltRowBackground": true,
	}

	var uncategorized []string
	for _, field := range all {
		checked := wantChecked[field]
		_, excluded := styleStringFieldExclusions[field]
		if checked && excluded {
			t.Errorf("%s is both checked AND excluded — pick one", field)
			continue
		}
		if !checked && !excluded {
			uncategorized = append(uncategorized, field)
		}
	}
	if len(uncategorized) > 0 {
		t.Fatalf("schema field(s) %v are neither checked by checkStyleHasNoPlaceholders/table.altRowBackground "+
			"nor named in styleStringFieldExclusions with a reason — the sweep missed them (D-000.67)", uncategorized)
	}
	for field := range wantChecked {
		found := false
		for _, f := range all {
			if f == field {
				found = true
			}
		}
		if !found {
			t.Fatalf("wantChecked names %s but reflection over the schema did not find it — the checked-field list has drifted ahead of the schema", field)
		}
	}
}
