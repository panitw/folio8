package folio8_test

// TestValidatePredictsRender is AC2 (D-3.7.1's guardrail; "do not
// re-open" item 1): Validate's verdict agrees with Render's, on the
// SAME inputs, for a table of cases this test owns.
//
// The visibleIf-gated row below is written from a MEASUREMENT taken
// against HEAD at Story 3.7's creation (task 5), not from the Story
// 1.8 ruling's own paraphrase: that ruling anticipated a case where
// "data-dependence makes prediction impossible" and the diagnostic
// "degrades to a Warning." Measured at HEAD (render.go's
// collectBandTextRuns and buildPageModel's image-asset loop, both
// carrying Story 3.5's own R2/AC7(b) comments): EVERY element's own
// binding, expression and asset validation runs UNCONDITIONALLY,
// regardless of its visibleIf verdict — visibility gates only whether
// a run/diagnostic is APPENDED to the output, never whether validation
// itself runs. Story 3.5 closed the gap the earlier Story 1.8 ruling
// anticipated: there is no remaining case at HEAD where a hidden
// element's own failure is invisible to Validate or degrades to a
// Warning — a hidden element's broken binding is exactly as fatal as a
// visible one's, in both Render and Validate alike. This row measures
// and pins that, rather than asserting the anticipated-but-superseded
// Warning behaviour.
import (
	"errors"
	"strings"
	"testing"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
)

const validateWellFormedTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 500, "height": 20, "value": "{{name}}", "style": {"fontFamily": "body", "fontSize": 14}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// validateVisibleIfLiteralTemplateJSON is D-3.5.1's case: a literal
// (non-boolean) visibleIf condition, which folio8_expr_validate.go
// rejects at PARSE time (measured at HEAD) — it never reaches Render
// as a *Template at all, so "both reject" here means ParseTemplate
// itself fails identically for the bytes Validate is handed.
const validateVisibleIfLiteralTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 500, "height": 20, "value": "{{name}}", "visibleIf": "42", "style": {"fontFamily": "body", "fontSize": 14}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// validateHiddenBrokenBindingTemplateJSON is the visibleIf-gated row's
// fixture: e1 is hidden whenever "flag" is false, and its OWN value
// binds to a path ("missing.path") absent from every data set this
// test supplies. Measured at HEAD: this errors in BOTH Render and
// Validate REGARDLESS of "flag"'s value, because collectBandTextRuns
// validates every element's binding before consulting its visibility
// verdict (render.go, Story 3.5 R2/AC7(b)).
const validateHiddenBrokenBindingTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 500, "height": 20, "value": "{{missing.path}}", "visibleIf": "flag", "style": {"fontFamily": "body", "fontSize": 14}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

func TestValidatePredictsRender(t *testing.T) {
	fs := fonts.Shipped()

	t.Run("malformed document: both reject, Render never attemptable", func(t *testing.T) {
		malformed := []byte(`{ this is not valid JSON`)
		diags, err := folio8.Validate(malformed, folio8.Data("{}"), folio8.Params("{}"), fs)
		if err == nil {
			t.Fatalf("Validate() on malformed bytes: want error, got diags=%+v", diags)
		}
		// Render takes a *Template, not bytes — a malformed document
		// never produces one, so "Render agrees" here means ParseTemplate
		// itself (Render's only possible entry point for these bytes)
		// fails the same way.
		if _, perr := folio8.ParseTemplate(malformed); perr == nil {
			t.Fatal("ParseTemplate() on the same malformed bytes unexpectedly succeeded")
		}
	})

	t.Run("well-formed document with data and fonts: both clean", func(t *testing.T) {
		b := []byte(validateWellFormedTemplateJSON)
		data := folio8.Data(`{"name": "Jane Doe"}`)
		params := folio8.Params("{}")

		diags, verr := folio8.Validate(b, data, params, fs)
		if verr != nil {
			t.Fatalf("Validate() error: %v", verr)
		}
		if len(diags) != 0 {
			t.Errorf("Validate() diags = %+v, want none", diags)
		}

		tpl, perr := folio8.ParseTemplate(b)
		if perr != nil {
			t.Fatalf("ParseTemplate() error: %v", perr)
		}
		res, rerr := folio8.Render(tpl, data, params, fs)
		if rerr != nil {
			t.Fatalf("Render() error: %v", rerr)
		}
		if len(res.Diagnostics) != 0 {
			t.Errorf("Render() diagnostics = %+v, want none", res.Diagnostics)
		}
	})

	t.Run("D-3.5.1: a literal visibleIf is rejected at PARSE time, before Validate or Render can disagree", func(t *testing.T) {
		b := []byte(validateVisibleIfLiteralTemplateJSON)
		_, verr := folio8.Validate(b, folio8.Data("{}"), folio8.Params("{}"), fs)
		if verr == nil {
			t.Fatal("Validate() on a literal visibleIf: want error, got none")
		}
		if !strings.Contains(verr.Error(), "must be a boolean or null") {
			t.Errorf("Validate() error does not name the literal-visibleIf defect: %v", verr)
		}
		if _, perr := folio8.ParseTemplate(b); perr == nil {
			t.Fatal("ParseTemplate() on the same literal visibleIf unexpectedly succeeded — Render could never even be attempted on this document, which is the agreement this row asserts")
		}
	})

	t.Run("hidden element with an unresolvable binding errors in BOTH Validate and Render, regardless of visibility (measured, see file doc comment)", func(t *testing.T) {
		b := []byte(validateHiddenBrokenBindingTemplateJSON)
		tpl, perr := folio8.ParseTemplate(b)
		if perr != nil {
			t.Fatalf("ParseTemplate() error: %v", perr)
		}
		for _, flag := range []bool{true, false} {
			data := folio8.Data(`{"flag": ` + boolLit(flag) + `}`)
			params := folio8.Params("{}")

			_, verr := folio8.Validate(b, data, params, fs)
			var vre *folio8.RenderError
			if !errors.As(verr, &vre) {
				t.Fatalf("flag=%v: Validate() error = %v, want a *RenderError", flag, verr)
			}
			if vre.Diagnostic.Code != folio8.DiagCodeBindingPathAbsent {
				t.Errorf("flag=%v: Validate() code = %q, want %q", flag, vre.Diagnostic.Code, folio8.DiagCodeBindingPathAbsent)
			}

			_, rerr := folio8.Render(tpl, data, params, fs)
			var rre *folio8.RenderError
			if !errors.As(rerr, &rre) {
				t.Fatalf("flag=%v: Render() error = %v, want a *RenderError", flag, rerr)
			}
			if rre.Diagnostic.Code != folio8.DiagCodeBindingPathAbsent {
				t.Errorf("flag=%v: Render() code = %q, want %q", flag, rre.Diagnostic.Code, folio8.DiagCodeBindingPathAbsent)
			}
		}
	})

	t.Run("D-3.7.1's trap: empty Data yields absent-path Errors that are correct predictions of a render with the same empty Data", func(t *testing.T) {
		b := []byte(validateWellFormedTemplateJSON)
		emptyData := folio8.Data("{}")
		params := folio8.Params("{}")

		_, verr := folio8.Validate(b, emptyData, params, fs)
		if verr == nil {
			t.Fatal("Validate() with empty Data on a template binding {{name}}: want an absent-path error, got none")
		}
		var vre *folio8.RenderError
		if !errors.As(verr, &vre) || vre.Diagnostic.Code != folio8.DiagCodeBindingPathAbsent {
			t.Fatalf("Validate() error = %v, want a *RenderError carrying %q", verr, folio8.DiagCodeBindingPathAbsent)
		}

		tpl, perr := folio8.ParseTemplate(b)
		if perr != nil {
			t.Fatalf("ParseTemplate() error: %v", perr)
		}
		_, rerr := folio8.Render(tpl, emptyData, params, fs)
		if rerr == nil {
			t.Fatal("Render() with the SAME empty Data: want the same absent-path error, got none — Validate's rejection above must be a correct PREDICTION of this failure, not a false positive")
		}
		var rre *folio8.RenderError
		if !errors.As(rerr, &rre) || rre.Diagnostic.Code != folio8.DiagCodeBindingPathAbsent {
			t.Fatalf("Render() error = %v, want a *RenderError carrying %q", rerr, folio8.DiagCodeBindingPathAbsent)
		}
	})

	// QA Finding 12 (this story's review, Minor): AC2's own named
	// mutation ("remove one validation step from Validate that Render
	// still performs -> the agreement table reddens on that row") was
	// run against date_test.go's TestDocumentDateInvalidIsLocatedErrorOnBothPaths,
	// not against THIS table — AC2's own stated anchor. This row closes
	// that gap: it belongs in the table AC2 anchors on, so the same
	// mutation (removing Validate's resolveDocumentDate call) reddens
	// HERE too, and so does any future Validate-only step that has no
	// row of its own.
	t.Run("a present-but-invalid documentDate errors in BOTH Validate and Render, with the same code", func(t *testing.T) {
		b := []byte(validateWellFormedTemplateJSON)
		data := folio8.Data(`{"name": "Jane"}`)
		params := folio8.Params(`{"documentDate": "not-a-timestamp"}`)

		_, verr := folio8.Validate(b, data, params, fs)
		var vre *folio8.RenderError
		if !errors.As(verr, &vre) {
			t.Fatalf("Validate() error = %v, want a *RenderError", verr)
		}
		if vre.Diagnostic.Code != folio8.DiagCodeDocumentDateInvalid {
			t.Errorf("Validate() code = %q, want %q", vre.Diagnostic.Code, folio8.DiagCodeDocumentDateInvalid)
		}

		tpl, perr := folio8.ParseTemplate(b)
		if perr != nil {
			t.Fatalf("ParseTemplate() error: %v", perr)
		}
		_, rerr := folio8.Render(tpl, data, params, fs)
		var rre *folio8.RenderError
		if !errors.As(rerr, &rre) {
			t.Fatalf("Render() error = %v, want a *RenderError", rerr)
		}
		if rre.Diagnostic.Code != folio8.DiagCodeDocumentDateInvalid {
			t.Errorf("Render() code = %q, want %q", rre.Diagnostic.Code, folio8.DiagCodeDocumentDateInvalid)
		}
	})
}

func boolLit(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
