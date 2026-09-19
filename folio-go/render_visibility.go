// This file is Story 3.5's own obligation (FR20, AD-24 "Visibility"):
// visibleIf already shipped inert at 3.2 (modelled, parsed, serialized,
// statically checked at load) — this file is what finally reads it at
// render.
//
// AD-24, verbatim: "A condition (FR20) is evaluated during bind. A
// hidden element is absent from the PageModel and leaves no gap;
// siblings never move, because nothing in a band ever reflows."
// "Absent from the PageModel" is a claim about buildPageModel's OUTPUT
// (zero Runs/Images entries for that element), never about the
// PIPELINE: a hidden element's own font chain, bindings and
// expressions are still validated exactly as a visible element's are
// (R2) — this file only ever suppresses appending a run/image/
// diagnostic, and never skips a validation call. render.go:601-616
// (Story 2.5, QA Finding 5, Major) is the shipped precedent for what
// happens when a skip is placed too early: "the SAME broken template
// passing or failing depending on which report it was handed."
package folio8

import (
	"github.com/panitw/folio8/folio-go/internal/bind"
	"github.com/panitw/folio8/folio-go/internal/expr"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// visibilityVerdicts maps every element's id, across all three bands,
// to whether it is visible. An element with no visibleIf (absent, or
// present-null — R6's first two rows) is always visible; one is
// missing from this map only if computeVisibility was never asked
// about it, in which case isVisible's default (true) applies, matching
// "no condition declared" exactly.
type visibilityVerdicts map[template.ElementID]bool

// isVisible reports whether id is visible under v — true when v is nil
// (collectTextRuns' legacy, pre-3.5 callers, which never compute
// visibility at all: byte-identical old behaviour) or when id has no
// recorded verdict (defensive; computeVisibility always records one
// for every element it walks).
func isVisible(v visibilityVerdicts, id template.ElementID) bool {
	if v == nil {
		return true
	}
	verdict, ok := v[id]
	return !ok || verdict
}

// computeVisibility evaluates every element's visibleIf condition
// exactly once, for every element across all three bands, from the
// data/params scope ALONE.
//
// It records a verdict for ALL FIVE element kinds (template.ElementType
// is a closed set — model.go), but only TWO are consumed anywhere
// today: collectBandTextRuns (render.go) reads a text element's
// verdict, and contentColumnItems (page_number.go) / paginateDocument
// (render.go) read an image element's. table/line/rect verdicts are
// computed and sit in the returned map, unconsumed — LATENT, not
// broken (Story 3.5 finisher review, Finding 6 / Minor): neither kind
// reaches the page model at all yet (tables load and validate today,
// D2 in this story's own creation notes; line/rect placement is Epic
// 4), so there is nothing yet for a verdict to gate. When a future
// story wires table/line/rect placement, THAT story owns consulting
// isVisible for its own kind at its own consumption site — nothing
// here forces it to, so grep this comment first.
//
// R1/AC9: this is the ONE place visibility is decided, and it runs
// before ANY collection pass — before collectImageRuns and before
// PHASE A's content-band text (buildPageModel calls it first, right
// after documentBands/checkTableBindings). AC9's "cannot depend on the
// page" is enforced by THIS FUNCTION'S OWN SIGNATURE, the same
// instrument resolvePageRunForPage uses on itself (page_number.go):
// it receives bands, data and params, and NOTHING page-derived — no
// pageCount, no layout.Paginate result, nothing computed downstream of
// it — so a page-dependent visibility verdict is a compile error, not
// a review catch. fc is the document's own locale/UTC-offset
// FormatContext (Story 3.4), which is template-derived, never
// page-derived, and is threaded through only because formatDate/
// formatNumber could in principle appear inside a condition expression
// — AD-4 does not reach it.
//
// Condition semantics are reused verbatim from if() (AC5, D-3.2.3):
// JSON true/false decide the verdict directly; an explicit JSON null
// is silently hidden, no diagnostic; a path absent from the data is a
// located Error; a string or number (no truthiness) is a located
// Error. expr.ConditionValue is the ONE place that axis lives — this
// function does not re-derive it.
func computeVisibility(bands []bandWithOrigin, data, params bind.Value, fc expr.FormatContext) (visibilityVerdicts, []Diagnostic, error) {
	var diagnostics []Diagnostic
	verdicts := make(visibilityVerdicts)
	scope := bind.NewScope(data, params)
	for _, b := range bands {
		for _, el := range b.band.Elements {
			if !el.VisibleIf.Set || el.VisibleIf.Null {
				// R6, rows 1-2: no condition declared (field absent, or
				// present but explicit JSON null) both mean VISIBLE —
				// there is no expression to evaluate because none was
				// declared (folio8_expr_validate.go's own skip for this
				// exact pair).
				verdicts[el.ID] = true
				continue
			}
			val, caveats, everr := bind.EvaluateCondition(el.VisibleIf.Value, scope, fc, string(el.ID))
			if everr != nil {
				return nil, nil, expressionRuntimeError(string(el.ID), "visibleIf", everr)
			}
			visible, cerr := expr.ConditionValue(val, "visibleIf", el.VisibleIf.Value, string(el.ID))
			if cerr != nil {
				return nil, nil, expressionRuntimeError(string(el.ID), "visibleIf", cerr)
			}
			for _, caveat := range caveats {
				diagnostics = append(diagnostics, diagnosticFromCaveat(string(el.ID), caveat))
			}
			verdicts[el.ID] = visible
		}
	}
	return verdicts, diagnostics, nil
}
