package folio8_test

// This file covers Story 3.7's date-related ACs that are exercised
// in-process, against the public API directly (AC9, AC10). AC8's
// SOURCE_DATE_EPOCH-through-the-environment assertion and AC11's
// fourth case are cmd/folio8's own (main_subprocess_test.go, a real
// subprocess) — this file's job is the two properties that do NOT
// involve the CLI or the environment at all.

import (
	"errors"
	"strings"
	"testing"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
)

// TestRenderWithParamsDocumentDateSetsCreationAndModDate is AC9: the
// SAME date arrives when a caller supplies documentDate directly
// through Params, with NO environment involved at all — proving the
// wiring is "a caller like any other" (AD-7's own words), not a
// privileged path only cmd/folio8 can reach.
func TestRenderWithParamsDocumentDateSetsCreationAndModDate(t *testing.T) {
	tpl, err := folio8.ParseTemplate([]byte(validateWellFormedTemplateJSON))
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	params := folio8.Params(`{"documentDate": "2023-06-15T12:00:00Z"}`)
	res, err := folio8.Render(tpl, folio8.Data(`{"name": "Jane"}`), params, fonts.Shipped())
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	b := res.Bytes
	if !strings.Contains(string(b), "/CreationDate") || !strings.Contains(string(b), "/ModDate") {
		t.Fatalf("output does not carry both date keys")
	}
	want := "D:20230615120000+00'00'"
	if !strings.Contains(string(b), want) {
		t.Errorf("output does not carry the expected formatted date %q; got:\n%s", want, b)
	}
}

// TestDocumentDateInvalidIsLocatedErrorOnBothPaths is AC10: a
// present-but-non-RFC-3339 documentDate value is a located Error, and
// BOTH Render and Validate reject it, with a *RenderError carrying
// DiagCodeDocumentDateInvalid.
func TestDocumentDateInvalidIsLocatedErrorOnBothPaths(t *testing.T) {
	b := []byte(validateWellFormedTemplateJSON)
	data := folio8.Data(`{"name": "Jane"}`)
	params := folio8.Params(`{"documentDate": "not-a-timestamp"}`)
	fs := fonts.Shipped()

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

	_, verr := folio8.Validate(b, data, params, fs)
	var vre *folio8.RenderError
	if !errors.As(verr, &vre) {
		t.Fatalf("Validate() error = %v, want a *RenderError", verr)
	}
	if vre.Diagnostic.Code != folio8.DiagCodeDocumentDateInvalid {
		t.Errorf("Validate() code = %q, want %q", vre.Diagnostic.Code, folio8.DiagCodeDocumentDateInvalid)
	}
}

// TestDocumentDateInvalid_MutationAcceptingVerbatim is AC10's own named
// mutation, RUN as a real code mutation elsewhere (see the story's
// Delivery Log); this permanent test is the guard that mutation
// reddens: a malformed documentDate string must never reach
// /CreationDate verbatim. Asserted here as the negative of the above:
// the malformed value's own text must never appear in output, because
// there IS no output — the render/validate call errors before any
// bytes exist.
func TestDocumentDateInvalidNeverReachesOutput(t *testing.T) {
	tpl, err := folio8.ParseTemplate([]byte(validateWellFormedTemplateJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	params := folio8.Params(`{"documentDate": "not-a-timestamp"}`)
	res, rerr := folio8.Render(tpl, folio8.Data(`{"name": "Jane"}`), params, fonts.Shipped())
	if rerr == nil {
		t.Fatalf("Render() unexpectedly succeeded with an invalid documentDate; bytes = %d", len(res.Bytes))
	}
}

// TestDocumentDateYearBoundIsEnforced is QA Finding 14's own pin
// (this story's review, Minor): R5 records the shared civil calendar
// as "bounded to years 1-9999" but nothing enforced the lower bound —
// documentDate: "0000-01-01T00:00:00Z" round-tripped through the
// calendar and reached /CreationDate as D:00000101000000+00'00',
// outside ISO 32000-1's representable calendar. Both edges are pinned:
// year 0000 is rejected, and the boundary years 0001 and 9999 (already
// covered structurally by the exhaustive round-trip measurement, but
// never asserted through documentDate specifically) are accepted.
func TestDocumentDateYearBoundIsEnforced(t *testing.T) {
	fs := fonts.Shipped()
	tpl, err := folio8.ParseTemplate([]byte(validateWellFormedTemplateJSON))
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	data := folio8.Data(`{"name": "Jane"}`)

	t.Run("year 0000 is a located error", func(t *testing.T) {
		params := folio8.Params(`{"documentDate": "0000-01-01T00:00:00Z"}`)
		_, rerr := folio8.Render(tpl, data, params, fs)
		var rre *folio8.RenderError
		if !errors.As(rerr, &rre) {
			t.Fatalf("Render() error = %v, want a *RenderError for year 0000", rerr)
		}
		if rre.Diagnostic.Code != folio8.DiagCodeDocumentDateInvalid {
			t.Errorf("code = %q, want %q", rre.Diagnostic.Code, folio8.DiagCodeDocumentDateInvalid)
		}
	})

	for _, year := range []string{"0001", "9999"} {
		year := year
		t.Run("year "+year+" is accepted", func(t *testing.T) {
			params := folio8.Params(`{"documentDate": "` + year + `-06-15T12:00:00Z"}`)
			res, rerr := folio8.Render(tpl, data, params, fs)
			if rerr != nil {
				t.Fatalf("Render() error for year %s: %v", year, rerr)
			}
			want := "D:" + year + "0615120000+00'00'"
			if !strings.Contains(string(res.Bytes), want) {
				t.Errorf("output does not contain %q for year %s", want, year)
			}
		})
	}
}
