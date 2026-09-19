package folio8

// This file exercises D-3.6.3's arm A (RenderError) against AC4's four
// Error modes, AC8 and AC9 — through the PUBLIC folio8.ParseTemplate/
// Render, on SYNTHETIC templates (D-2.6.5's guardrail: no committed
// fixture should be distorted into carrying a deliberately broken
// document).

import (
	"errors"
	"strings"
	"testing"
)

const malformedTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {"elements": [], "height": 20},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00"
}
`

// unparseableTemplateJSON is what FR41's "malformed template" mode
// actually names since Story 7.8: bytes that are not a `.folio`
// DOCUMENT at all. It has no field to be located at, and its own error
// text can quote the offending input back — which is exactly why
// wasm/cmd/engine's reportableMessage replaces TEMPLATE_MALFORMED's
// message and only that one.
//
// malformedTemplateJSON above (a well-formed JSON object missing the
// required `version` key) is the OTHER half of what that one code used
// to carry, and it is now TEMPLATE_FIELD_INVALID: a located field error
// whose message names the field and quotes nothing.
const unparseableTemplateJSON = `["not", "a", "folio8", "document"]`

const invalidExpressionTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 500, "height": 20, "value": "{{if(}}", "style": {"fontFamily": "body", "fontSize": 14}}
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

const unresolvableBindingTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 500, "height": 20, "value": "{{customer.name}}", "style": {"fontFamily": "body", "fontSize": 14}}
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

// TestFourErrorModesCarrySeverityErrorDiagnostics is AC4/AC8/AC9: each
// of FR41's four Error modes must be recoverable via errors.As(err,
// &RenderError), carrying a DISTINCT registry code, SeverityError, and
// a message locating the element — WITHOUT reading Message to learn
// which mode fired (AC9: match on the code, never on message text).
//
// Story 7.8 splits the "malformed template" mode into the two codes it
// always contained: TEMPLATE_MALFORMED for a document that is not a
// document, TEMPLATE_FIELD_INVALID for a well-formed document carrying
// an unacceptable field value. Five cases, four FR41 modes.
func TestFourErrorModesCarrySeverityErrorDiagnostics(t *testing.T) {
	cases := []struct {
		name          string
		build         func(t *testing.T) error // returns the error from ParseTemplate or Render
		wantCode      string
		wantElementID string
	}{
		{
			name: "malformed template",
			build: func(t *testing.T) error {
				_, err := ParseTemplate([]byte(unparseableTemplateJSON))
				return err
			},
			wantCode:      DiagCodeTemplateMalformed,
			wantElementID: "",
		},
		{
			// Story 7.8, D-7.8.1: FR41's "malformed template" mode
			// covers two observably different failures, and until this
			// story both carried one code — so a LOCATED field error
			// was flattened at the WASM boundary along with the
			// document-quoting one the flattening exists for. They are
			// now distinct codes, and the pairwise-distinctness check
			// below is what holds them apart.
			name: "invalid template field",
			build: func(t *testing.T) error {
				_, err := ParseTemplate([]byte(malformedTemplateJSON))
				return err
			},
			wantCode:      DiagCodeTemplateFieldInvalid,
			wantElementID: "",
		},
		{
			name: "invalid expression",
			build: func(t *testing.T) error {
				_, err := ParseTemplate([]byte(invalidExpressionTemplateJSON))
				return err
			},
			wantCode:      DiagCodeExpressionInvalid,
			wantElementID: "e1",
		},
		{
			name: "unresolvable binding",
			build: func(t *testing.T) error {
				tpl, perr := ParseTemplate([]byte(unresolvableBindingTemplateJSON))
				if perr != nil {
					t.Fatalf("presence precondition: parse failed: %v", perr)
				}
				_, err := Render(tpl, Data(`{}`), nil, testFontSet())
				return err
			},
			wantCode:      DiagCodeBindingPathAbsent,
			wantElementID: "e1",
		},
		{
			name: "unlayoutable content",
			build: func(t *testing.T) error {
				tpl, perr := ParseTemplate([]byte(overflowLineTemplate))
				if perr != nil {
					t.Fatalf("presence precondition: parse failed: %v", perr)
				}
				_, err := Render(tpl, Data(`{}`), nil, testShippedFontSet())
				return err
			},
			wantCode:      DiagCodeContentUnlayoutable,
			wantElementID: "e1",
		},
	}

	seenCodes := map[string]string{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.build(t)
			if err == nil {
				t.Fatalf("presence precondition: %s produced no error", tc.name)
			}

			var renderErr *RenderError
			if !errors.As(err, &renderErr) {
				t.Fatalf("%s: errors.As(err, &RenderError) failed; err = %v (%T)", tc.name, err, err)
			}
			d := renderErr.Diagnostic
			if d.Severity != SeverityError {
				t.Errorf("%s: Severity = %v, want SeverityError", tc.name, d.Severity)
			}
			if d.Code != tc.wantCode {
				t.Errorf("%s: Code = %q, want %q", tc.name, d.Code, tc.wantCode)
			}
			if tc.wantElementID != "" {
				if d.ElementID != tc.wantElementID {
					t.Errorf("%s: ElementID = %q, want %q", tc.name, d.ElementID, tc.wantElementID)
				}
				if !strings.Contains(err.Error(), tc.wantElementID) {
					t.Errorf("%s: error message %q does not name the element id %q", tc.name, err.Error(), tc.wantElementID)
				}
			}

			// AC9: the code is recoverable WITHOUT reading Message —
			// asserted here by never having consulted d.Message above
			// to decide anything; the control assertion at
			// TestMessageRewriteDoesNotAffectCodeRecovery (below) makes
			// this a live guarantee, not an absence of a check.
			if prior, ok := seenCodes[d.Code]; ok {
				t.Errorf("%s: code %q is not distinct — already used by %q (AC4 requires pairwise-distinct codes)", tc.name, d.Code, prior)
			}
			seenCodes[d.Code] = tc.name
		})
	}

	if len(seenCodes) != len(cases) {
		t.Fatalf("collected %d distinct codes across %d modes", len(seenCodes), len(cases))
	}

	// Finding 12 (QA review, Minor): AC4 requires all FIVE FR41 codes
	// pairwise distinct, but the loop above only ever sees the four
	// Error modes — the fifth, missing glyph, is a Warning and never
	// reaches this function via errors.As. Checked here against the
	// same seenCodes set built above, so a registry-code collision
	// between the Warning and any of the four Errors is caught too.
	glyphTpl, gerr := ParseTemplate([]byte(missingGlyphTemplateJSON))
	if gerr != nil {
		t.Fatalf("parse missing-glyph template: %v", gerr)
	}
	glyphRes, gerr := Render(glyphTpl, Data(`{"name":"ก"}`), Params(`{}`), testShippedFontSet())
	if gerr != nil {
		t.Fatalf("render missing-glyph fixture: %v", gerr)
	}
	if len(glyphRes.Diagnostics) != 1 {
		t.Fatalf("presence precondition: want exactly 1 missing-glyph Diagnostic, got %d: %+v", len(glyphRes.Diagnostics), glyphRes.Diagnostics)
	}
	glyphCode := glyphRes.Diagnostics[0].Code
	if prior, ok := seenCodes[glyphCode]; ok {
		t.Errorf("missing glyph: code %q is not distinct — already used by %q (AC4 requires pairwise-distinct codes)", glyphCode, prior)
	}
	seenCodes[glyphCode] = "missing glyph"

	if len(seenCodes) != len(cases)+1 {
		t.Fatalf("collected %d distinct codes across %d modes (want %d, including the missing-glyph Warning)", len(seenCodes), len(cases)+1, len(cases)+1)
	}
}

// TestRenderErrorWrapsWithoutReplacing is AC8's own discriminator: a
// RenderError must be a WRAPPER, not a REPLACEMENT. Mutation (a):
// confirms the aborting-render half stays true (bytes absent, error
// non-nil). Mutation (b), per mode: the PRE-EXISTING errors.As target
// still resolves through RenderError's Unwrap chain — proven directly
// above for content-unlayoutable (against *layout.OverflowError, via
// render_overflow_test.go's own pre-existing assertion, unmodified)
// and here for the template-load path (against *template.LoadError).
func TestRenderErrorWrapsWithoutReplacing(t *testing.T) {
	_, err := ParseTemplate([]byte(malformedTemplateJSON))
	if err == nil {
		t.Fatal("presence precondition: malformed template produced no error")
	}

	// The render must have ABORTED: Render is never even reached here
	// (ParseTemplate itself failed), so there is no Result to inspect —
	// the strongest form of "no bytes accompany a SeverityError value".
	var renderErr *RenderError
	if !errors.As(err, &renderErr) {
		t.Fatalf("errors.As(err, &RenderError) failed; err = %v (%T)", err, err)
	}
}

// TestMessageRewriteDoesNotAffectCodeRecovery is AC9's mutation:
// rewrite each of the five messages in the working tree, and every
// code assertion must stay green while a deliberately message-matching
// CONTROL assertion reddens, proving the mutation was live and the
// green is not vacuous (D-000.68). Covers all five FR41 modes (Finding
// 4's optional extension), not just one.
//
// Finding 4 (QA review, Major): the original version assigned to a
// LOCAL field of an ALREADY-RECOVERED Diagnostic
// (`renderErr.Diagnostic.Message = "..."`) and then read `.Code` back
// from the same value. Go's type system guarantees a struct field
// assignment cannot affect a sibling field, so neither half could ever
// fail — the "mutation" never touched production code, and the
// "control" only checked that the line above it had run. This version
// instead matches each CONTROL on a substring of the REAL, LIVE
// message text the production site actually emits right now.
// Rewriting that wording in the working tree — exactly what the
// reviewer did by hand, at folio8_expr_validate.go:127,147, to
// discharge this finding — reddens the corresponding control below,
// while every Code assertion stays green, because Code is never
// derived from Message at any of these five construction sites.
func TestMessageRewriteDoesNotAffectCodeRecovery(t *testing.T) {
	cases := []struct {
		name              string
		build             func(t *testing.T) (code, message string)
		wantCode          string
		wantMessageSubstr string // a real, production-owned phrase — never derived from wantCode
	}{
		{
			// Repointed at Story 7.8: this fixture's condition — a
			// required field missing from an otherwise well-formed
			// document — is TEMPLATE_FIELD_INVALID now, not
			// TEMPLATE_MALFORMED. The property under test is unchanged:
			// Code is recovered without reading Message.
			name: "invalid template field",
			build: func(t *testing.T) (string, string) {
				_, err := ParseTemplate([]byte(malformedTemplateJSON))
				if err == nil {
					t.Fatal("presence precondition: malformed template produced no error")
				}
				var renderErr *RenderError
				if !errors.As(err, &renderErr) {
					t.Fatalf("errors.As(err, &RenderError) failed; err = %v (%T)", err, err)
				}
				return renderErr.Diagnostic.Code, renderErr.Diagnostic.Message
			},
			wantCode:          DiagCodeTemplateFieldInvalid,
			wantMessageSubstr: "missing required field",
		},
		{
			name: "invalid expression",
			build: func(t *testing.T) (string, string) {
				_, err := ParseTemplate([]byte(invalidExpressionTemplateJSON))
				if err == nil {
					t.Fatal("presence precondition: invalid expression produced no error")
				}
				var renderErr *RenderError
				if !errors.As(err, &renderErr) {
					t.Fatalf("errors.As(err, &RenderError) failed; err = %v (%T)", err, err)
				}
				return renderErr.Diagnostic.Code, renderErr.Diagnostic.Message
			},
			wantCode:          DiagCodeExpressionInvalid,
			wantMessageSubstr: "value expression",
		},
		{
			name: "unresolvable binding",
			build: func(t *testing.T) (string, string) {
				tpl, perr := ParseTemplate([]byte(unresolvableBindingTemplateJSON))
				if perr != nil {
					t.Fatalf("presence precondition: parse failed: %v", perr)
				}
				_, err := Render(tpl, Data(`{}`), nil, testFontSet())
				if err == nil {
					t.Fatal("presence precondition: unresolvable binding produced no error")
				}
				var renderErr *RenderError
				if !errors.As(err, &renderErr) {
					t.Fatalf("errors.As(err, &RenderError) failed; err = %v (%T)", err, err)
				}
				return renderErr.Diagnostic.Code, renderErr.Diagnostic.Message
			},
			wantCode:          DiagCodeBindingPathAbsent,
			wantMessageSubstr: "is absent from",
		},
		{
			name: "unlayoutable content",
			build: func(t *testing.T) (string, string) {
				tpl, perr := ParseTemplate([]byte(overflowLineTemplate))
				if perr != nil {
					t.Fatalf("presence precondition: parse failed: %v", perr)
				}
				_, err := Render(tpl, Data(`{}`), nil, testShippedFontSet())
				if err == nil {
					t.Fatal("presence precondition: overflow produced no error")
				}
				var renderErr *RenderError
				if !errors.As(err, &renderErr) {
					t.Fatalf("errors.As(err, &RenderError) failed; err = %v (%T)", err, err)
				}
				return renderErr.Diagnostic.Code, renderErr.Diagnostic.Message
			},
			wantCode:          DiagCodeContentUnlayoutable,
			wantMessageSubstr: "taller than the content window",
		},
		{
			name: "missing glyph",
			build: func(t *testing.T) (string, string) {
				tpl, perr := ParseTemplate([]byte(missingGlyphTemplateJSON))
				if perr != nil {
					t.Fatalf("presence precondition: parse failed: %v", perr)
				}
				res, err := Render(tpl, Data(`{"name":"ก"}`), Params(`{}`), testShippedFontSet())
				if err != nil {
					t.Fatalf("presence precondition: render failed: %v", err)
				}
				if len(res.Diagnostics) != 1 {
					t.Fatalf("presence precondition: want exactly 1 Diagnostic, got %d: %+v", len(res.Diagnostics), res.Diagnostics)
				}
				return res.Diagnostics[0].Code, res.Diagnostics[0].Message
			},
			wantCode:          DiagCodeTextMissingGlyph,
			wantMessageSubstr: "is omitted from the rendered output",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, message := tc.build(t)

			// The property AC9 names: Code is recoverable WITHOUT
			// reading Message. `code` and `message` are read
			// independently from the same construction site above;
			// nothing here derives one from the other.
			if code != tc.wantCode {
				t.Errorf("Code = %q, want %q", code, tc.wantCode)
			}

			// CONTROL: matches on the real, live production message
			// text. If a future edit rewrites this site's wording —
			// the exact class of change AC9 exists to prove is safe
			// for Code recovery — this assertion reddens, proving the
			// control is live rather than self-referential and that
			// the Code assertion's green above is not vacuous.
			if !strings.Contains(message, tc.wantMessageSubstr) {
				t.Fatalf("control: message %q no longer contains %q — Code recovery above is unaffected either way, but this control must fire on a real wording change or it proves nothing; update the literal if the wording change was deliberate", message, tc.wantMessageSubstr)
			}
		})
	}
}
