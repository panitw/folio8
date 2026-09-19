package folio8

import (
	"strings"
	"testing"
)

// bindTestTemplateJSON is a `.folio` document with one text element
// bound to report data (AC15) and one table element.
//
// Story 3.1 / finding 4: table.bind used to be "{{transactions[]}}" —
// braces and all, which is not a collection path at all
// (folio-format.md:211; D-2.4.1's "one path convention in the format,
// not two" precedent) — and the data supplied no "transactions", which
// source AC5 now makes a render error on its own. It is amended to the
// bare collection path "transactions[]" with a real array supplied
// below, while columns[0].bind STAYS expression-shaped exactly as it
// was: this test's actual purpose (D-1.6.8's field-scope fence, "a
// table's columns[].bind is never scanned by text binding") is
// unchanged and still non-vacuous, because Story 3.1 does not evaluate
// column binds either — that is Story 3.2.
//
// Story 4.2 correction (measured fresh in this run — the table did not
// declare "as" here, so the column's row-scoped "transaction.amount"
// path was about to resolve against the DATA ROOT under the default
// alias "row" and fail): "as": "transaction" is now declared so the
// column's own alias matches the bind it has always carried. This is a
// fixture correction, not a behaviour change — the same category as
// the nine table_render_test.go bind sites D3 tabulates — surfaced by
// this story because column binds are evaluated for the first time.
// For the same reason, "amount" is now a JSON NUMBER (1.00) rather
// than a string: formatNumber() (D-000.14, AD-14) never coerces, and
// the column's bind was never evaluated before this story to notice
// the mismatch.
const bindTestTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 20, "value": "Statement for {{customer.name}}", "style": {"fontFamily": "body", "fontSize": 14}},
        {"id": "e2", "type": "table", "x": 0, "y": 30, "bind": "transactions[]", "headerHeight": 20,
          "as": "transaction", "style": {"fontFamily": "body", "fontSize": 9},
          "columns": [
            {"id": "e3", "label": "Amount", "width": 80, "bind": "{{formatNumber(transaction.amount, \"#,##0.00\")}}"}
          ]}
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
  "fonts": {
    "body": ["Roboto-Regular"]
  },
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
`

// TestRenderScopeFenceIgnoresTableBind is now the COLUMN fence,
// AC20/D-1.6.8 (Story 3.1 amendment, finding 4): columns[0].bind
// carries expression-shaped "{{…}}" content (parentheses, a comma and
// quotes — three separate AC16 rejection triggers) that would fail
// AC16's grammar if it were ever scanned as TEXT-ELEMENT VALUE
// INTERPOLATION. Render succeeds anyway, because collectTextRuns skips
// every non-text element and is the ONLY call site of
// internal/bind.BindText — this is the shape M-4 found already living
// in the canonical golden fixture (worked-example.json:19), reproduced
// here as a render-time (not merely round-trip) proof.
//
// table.bind is READ, from Story 3.2 on, as a collection path (source
// AC5), never as interpolated text — so this template's table.bind is
// the bare path "transactions[]" with a real array supplied, and does
// not, on its own, exercise AC20's fence. As of Story 4.2, columns[].bind
// IS also evaluated — in the table's own ROW SCOPE (AD-11), never by
// collectTextRuns/BindText — so this test's discriminating claim is
// narrower than it once was: it proves column binds are never scanned
// by TEXT binding, not that they are unevaluated altogether. Render
// succeeding here therefore depends on BOTH the fence holding AND the
// column's own bind resolving cleanly in row scope; the two updated
// fixture corrections below (the declared alias, the numeric amount)
// are what make the latter true.
func TestRenderScopeFenceIgnoresTableBind(t *testing.T) {
	tpl, err := ParseTemplate([]byte(bindTestTemplateJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	data := Data(`{"customer": {"name": "Ada Lovelace"}, "transactions": [{"amount": 1.00}]}`)
	res, err := Render(tpl, data, nil, testFontSet())
	if err != nil {
		t.Fatalf("Render must succeed despite the table column's expression-shaped bind field: %v", err)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("Render produced no bytes")
	}
}

// TestRenderBindsTextPlaceholder is AC15/AC22-AC24 end to end through
// the public Render API.
func TestRenderBindsTextPlaceholder(t *testing.T) {
	tpl, err := ParseTemplate([]byte(minimalTemplateJSONWithText))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	data := Data(`{"customer": {"name": "Ada Lovelace"}}`)
	res, err := Render(tpl, data, nil, testFontSet())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("Render produced no bytes")
	}
}

// TestRenderAbsentPathIsLocatedError is AC8 through the public Render
// API: a binding whose path is absent from the data is an Error
// naming both the data path and the element id, and the render fails.
func TestRenderAbsentPathIsLocatedError(t *testing.T) {
	tpl, err := ParseTemplate([]byte(minimalTemplateJSONWithText))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	_, err = Render(tpl, Data(`{}`), nil, testFontSet())
	if err == nil {
		t.Fatal("expected an Error for an absent data path")
	}
}

// TestRenderNullPathRendersEmpty is AC9 through the public Render API:
// an explicit JSON null renders as empty, and is not an error.
func TestRenderNullPathRendersEmpty(t *testing.T) {
	tpl, err := ParseTemplate([]byte(minimalTemplateJSONWithText))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	_, err = Render(tpl, Data(`{"customer": {"name": null}}`), nil, testFontSet())
	if err != nil {
		t.Fatalf("explicit null must not be an error: %v", err)
	}
}

// TestRenderWrongKindIsLocatedError is AC10 through the public Render
// API, as revised on 2026-09-13: a JSON number bound into a text element
// renders (as its exact decimal), while a JSON boolean is still an Error,
// never coerced.
func TestRenderWrongKindIsLocatedError(t *testing.T) {
	tpl, err := ParseTemplate([]byte(minimalTemplateJSONWithText))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	if _, err = Render(tpl, Data(`{"customer": {"name": 123}}`), nil, testFontSet()); err != nil {
		t.Fatalf("a JSON number bound into a text element must render: %v", err)
	}
	_, err = Render(tpl, Data(`{"customer": {"name": true}}`), nil, testFontSet())
	if err == nil {
		t.Fatal("expected an Error for a JSON boolean bound into a text element")
	}
	if !strings.Contains(err.Error(), "e1") || !strings.Contains(err.Error(), "bool") {
		t.Fatalf("the boolean refusal must name the element and the kind, got: %v", err)
	}
}

// TestRenderMalformedParamsIsReportedAsParams is this story's review,
// Finding 6 (Minor): a malformed Params document was previously
// reported through the same message DecodeData produces for report
// data, so the error read "…invalid JSON report data…" for a value
// that was never sought in report data at all — M-6/AC16's exact
// rationale, one layer up from where it was originally fixed.
func TestRenderMalformedParamsIsReportedAsParams(t *testing.T) {
	tpl, err := ParseTemplate([]byte(minimalTemplateJSONWithText))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	_, err = Render(tpl, Data(`{"customer": {"name": "Ada"}}`), Params(`{oops`), testFontSet())
	if err == nil {
		t.Fatal("expected an Error for malformed params JSON")
	}
	if strings.Contains(err.Error(), "report data") {
		t.Errorf("a malformed PARAMS document must not be reported as invalid report data, got: %v", err)
	}
	if !strings.Contains(err.Error(), "params") {
		t.Errorf("error must name params, got: %v", err)
	}
}

// TestRenderTrailingGarbageInParamsIsReportedAsParams is Finding 6's
// second case: params inherit DecodeData's trailing-garbage rejection
// (AC20), but until this fix the message named report data even
// though the trailing garbage was in the params document.
func TestRenderTrailingGarbageInParamsIsReportedAsParams(t *testing.T) {
	tpl, err := ParseTemplate([]byte(minimalTemplateJSONWithText))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	_, err = Render(tpl, Data(`{"customer": {"name": "Ada"}}`), Params(`{"a":1} junk`), testFontSet())
	if err == nil {
		t.Fatal("expected an Error for trailing garbage after the params document's single top-level value")
	}
	if strings.Contains(err.Error(), "report data") {
		t.Errorf("trailing garbage in PARAMS must not be reported as invalid report data, got: %v", err)
	}
	if !strings.Contains(err.Error(), "params") {
		t.Errorf("error must name params, got: %v", err)
	}
}

// TestRenderRejectsInvalidNumberPatternAtLoad is F3's re-point (Story
// 3.4, AC10b/AC17): formatNumber is now IMPLEMENTED (AC16), and its
// pattern grammar is validated at Check/load time (AC10), not at
// evaluation — so `formatNumber(a, "x")`, whose pattern "x" is not in
// the closed number-pattern grammar, is now a LOAD error at
// ParseTemplate, never a Render-time error. The subject this test
// pins (a wrong-kind pattern is caught, with a located message)
// survives 3.2 -> 3.4; only the stage moves, per F3's own instruction
// to re-point rather than delete.
func TestRenderRejectsInvalidNumberPatternAtLoad(t *testing.T) {
	const tplJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 20, "value": "{{formatNumber(a, \"x\")}}", "style": {"fontFamily": "body", "fontSize": 14}}
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
		t.Fatal("expected a load error: \"x\" is not a valid formatNumber pattern")
	}
	if !strings.Contains(err.Error(), "e1") {
		t.Fatalf("error must name the element id, got: %v", err)
	}
	// Finding 11 (this story's QA review): every error this package
	// emits is prefixed "expr: …", which itself contains an "x" — a
	// bare Contains(err.Error(), "x") passes regardless of whether the
	// message actually names the offending pattern. Assert the quoted
	// form the error actually produces instead.
	if !strings.Contains(err.Error(), `"x"`) {
		t.Fatalf("error must name the offending pattern (quoted), got: %v", err)
	}
}

// TestRenderNullBoundTextStillValidatesFontChain is QA Finding 5 (this
// story's review, Major): an element with an unresolvable
// style.fontFamily chain must be a located error (Story 1.5 AC2/AC4)
// regardless of whether its bound text happens to be empty — the AC9
// short-circuit ("null renders as empty, not an error") must not
// suppress the element's own font-chain validation. Reproduces the
// finding's own table: the same template, unresolvable chain
// "nosuchchain", varying only the data.
func TestRenderNullBoundTextStillValidatesFontChain(t *testing.T) {
	const tplJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 20, "value": "{{customer.name}}", "style": {"fontFamily": "nosuchchain", "fontSize": 14}}
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
	tpl, err := ParseTemplate([]byte(tplJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}

	cases := []struct {
		name string
		data string
	}{
		{"null binding", `{"customer": {"name": null}}`},
		{"empty-string binding", `{"customer": {"name": ""}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Render(tpl, Data(c.data), nil, testFontSet())
			if err == nil {
				t.Fatalf("expected a located font-chain error even though the bound text is empty (data: %s)", c.data)
			}
			if !strings.Contains(err.Error(), "nosuchchain") {
				t.Errorf("error must still name the unresolvable chain, got: %v", err)
			}
		})
	}

	// Control: a non-empty binding against the same broken template
	// was already an error before this fix; confirm it still is.
	if _, err := Render(tpl, Data(`{"customer": {"name": "Ada"}}`), nil, testFontSet()); err == nil {
		t.Fatal("control: a non-empty binding against an unresolvable chain must still be an error")
	}
}

// minimalTemplateJSONWithText is minimalTemplateJSON plus one text
// element bound to "customer.name".
const minimalTemplateJSONWithText = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 20, "value": "Statement for {{customer.name}}", "style": {"fontFamily": "body", "fontSize": 14}}
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
  "fonts": {
    "body": ["Roboto-Regular"]
  },
  "locale": "en",
  "nextId": 2,
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
`
