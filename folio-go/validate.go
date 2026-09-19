package folio8

import (
	"fmt"

	"github.com/panitw/folio8/folio-go/internal/bind"
)

// This file declares Validate (Story 3.7, D-3.7.1, FR42). It is
// deliberately its OWN file, importing NONE of "os", "time", "net",
// "math/rand" — the same file-scoped shape render_entry.go documents
// for Render/RenderTo (AC12), extended by this story to a THIRD file:
// AC12(b) widens lint's findRenderDeclaringFiles to the closed set
// {Render, RenderTo, Validate}, and this file is exactly the new
// surface that widening exists to cover.
//
// Validate is a DRY-RUN PREDICTOR of Render, not a second rule system
// (decision log :4189, binding): every located Error Render can
// produce for a given (b, d, p, f), Validate produces too — for the
// SAME inputs — and NO RENDER IS ATTEMPTED (no page model is composed,
// no font is embedded as a PDF object, no byte of a PDF is ever
// produced). It shares its implementation with Render via
// predictDocument (render.go) rather than re-implementing the checks
// (D-000.42): Validate calls predictDocument directly and stops there,
// discarding the page model, embedded faces and image XObjects
// predictDocument also returns — those three values exist only to feed
// internal/pdf.SerializeTextDocument, which Validate's call graph never
// reaches (AC1, TestValidateNeverReachesRenderOrInternalPDF,
// render_arch_test.go).
//
// D-1.1.c / C1: this is the ONE public entry point FR42 gets. A
// second, structural-only validation entry point is deliberately NOT
// added — the doc comment below (the empty-Data caveat) costs nothing
// and two public entry points covering one FR is exactly the surface
// growth those decisions exist to prevent.
//
// THE USABILITY TRAP (D-3.7.1's own guardrail, AC2). Validate predicts
// Render FOR THE INPUTS GIVEN. A caller that passes an empty Data (or
// one missing the paths a template's placeholders bind to) gets
// absent-path Errors back from Validate — and those are CORRECT
// PREDICTIONS OF A RENDER WITH EMPTY DATA, not defects in the
// template. Pass the SAME sample data Render will eventually receive,
// or Validate will correctly, and unhelpfully, report that your data
// is missing. TestValidatePredictsRender's final subtest,
// "D-3.7.1's trap: empty Data yields absent-path Errors that are
// correct predictions of a render with the same empty Data"
// (validate_test.go), pins this by asserting Render, called with the
// identical empty Data, fails the same way. (QA Finding 8, this
// story's review: this comment previously named a test,
// TestValidatePredictsRenderIncludingEmptyDataCaveat, that does not
// exist anywhere in the repo.)
func Validate(b []byte, d Data, p Params, f FontSet) ([]Diagnostic, error) {
	t, err := ParseTemplate(b)
	if err != nil {
		return nil, err
	}
	data, err := bind.DecodeData(d)
	if err != nil {
		return nil, fmt.Errorf("folio8: Validate: %w", err)
	}
	params, err := decodeParams(p)
	if err != nil {
		return nil, err
	}
	if _, derr := resolveDocumentDate(params); derr != nil {
		return nil, derr
	}
	_, _, _, diags, perr := predictDocument(t, data, params, f)
	if perr != nil {
		return nil, perr
	}
	return diags, nil
}
