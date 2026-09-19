// Package folio8 is the public entry point of the folio-go rendering
// library.
package folio8

import (
	"os"

	"github.com/panitw/folio8/folio-go/internal/expr"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// Template is a parsed, canonicalised `.folio` document (AC1). It is an
// OPAQUE handle wrapping internal/template.Document — deliberately NOT
// a type alias (Epic 1 boundary gate finding): an alias made every
// exported field of internal/template.Document, and all 32 exported
// declarations reachable from it, part of the public surface, and let
// a caller construct a Template field-by-field, bypassing
// ParseTemplate entirely — sidestepping asset-key validation, the
// version rules, the eight closed sets, exact-decimal discipline and
// nextId monotonicity. Construction is only through LoadTemplate/
// ParseTemplate; there are no exported fields and no accessors are
// shipped pre-emptively (add them only when a real need appears — a
// MINOR addition later, versus a MAJOR break if the surface had to
// close after being opened). See testdata/templateopaque/ for the
// compile-time proof that composite-literal construction of Template
// does not type-check.
//
// A defined type without the "=" (type Template template.Document)
// would NOT close this hole: it still exposes every exported field of
// template.Document and still permits composite-literal construction
// (folio8.Template{Version: "1.0"} would compile). Do not take that
// route.
type Template struct {
	doc *template.Document

	// derivedFooters is Story 3.2/D-1.4.1's footerOf/footerFormat
	// derivation result (R1, R2), keyed by column id — resolved
	// ALONGSIDE doc, never written back into it (R2: internal/
	// template/serialize.go emits "footerOf" whenever it is SET, so
	// storing a derived value on doc would break D-1.4.3's P3 fixed
	// point for every document that legitimately omits footerOf).
	// Column ids are document-unique (AD-10), so a flat map keyed by
	// column id alone is unambiguous.
	derivedFooters map[template.ElementID]expr.DerivedFooter
}

// LoadTemplate reads path from disk and parses it as a `.folio`
// document (AC1). Per D-1.4.6: "So LoadTemplate(path string) and
// ParseTemplate(b []byte) live in package folio8 at the module root,
// where os is permitted; internal/template never sees a path — it
// takes []byte and returns a *Template or an error. AD-1 is untouched
// because no internal/ package imports os." LoadTemplate delegates to
// ParseTemplate — "this is honest factoring rather than scope growth."
func LoadTemplate(path string) (*Template, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseTemplate(b)
}

// ParseTemplate parses b as a `.folio` document (AC1). It is the sole
// bridge into internal/template's parser (AD-9: "internal/template
// owns both the parser and the serializer").
//
// Story 3.2 (R1, forced by F2: internal/template, stage rank 2, can
// never import internal/expr, stage rank 3 — D-1.6.1's pre-commitment
// — so this validation cannot live in internal/template itself):
// ParseTemplate is ALSO where every "{{ }}" expression in the document
// (text elements' `value`, table columns' `bind`) is parsed and
// statically checked (expr.Parse, expr.Check — syntax, arity, unknown
// function names, literal-argument kind; R3: "syntax and arity at
// load; execution at evaluation" — never a full evaluation, which
// needs report data that does not exist yet), and where D-1.4.1's
// footerOf/footerFormat derivation runs for every table column that
// requests a sum/avg footer without naming footerOf explicitly
// (AC21). A syntax error, an unknown/mis-aritied function, or a
// non-derivable bind on a column that needed derivation are all load
// errors from this public entry point — never deferred to Render.
func ParseTemplate(b []byte) (*Template, error) {
	doc, err := template.ParseDocument(b)
	if err != nil {
		// Story 3.6, AC4/AC8/R8: wraps as *RenderError, carrying
		// DiagCodeTemplateMalformed (FR41's "malformed template" mode)
		// or, for DW-6's two named conditions, the coded value
		// internal/template's own newLoadErrorCoded attached.
		return nil, wrapTemplateError(err)
	}
	derived, err := validateAndDeriveExpressions(doc)
	if err != nil {
		return nil, err
	}
	t := &Template{doc: doc, derivedFooters: derived}
	// SPEC-table-rules §3: a table whose `minHeight` exceeds the content
	// window can never be placed, and that is decidable from the document
	// alone — so it is refused HERE, at the same public boundary the
	// expression checks are refused at, and for the same layering reason
	// (internal/template may not import internal/layout).
	if err := validateTableMinHeights(t); err != nil {
		return nil, err
	}
	// spec-section-break CAP-5: the break's range against the derived
	// content height, and no element on both sides of it — refused here for
	// the same layering reason.
	if err := validateSectionBreak(t); err != nil {
		return nil, err
	}
	return t, nil
}
