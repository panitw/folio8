package folio8

import (
	"errors"
	"fmt"
	"github.com/panitw/folio8/folio8-go/internal/bind"

	"github.com/panitw/folio8/folio8-go/internal/layout"
	"github.com/panitw/folio8/folio8-go/internal/template"
)

// This file is Story 3.6's D-3.6.3, RULED ARM A: AD-14 requires "every
// failure mode named in FR41 has a code" and a code is a FIELD of
// Diagnostic — a bare fmt.Errorf has no fields. So FR41's four Error
// modes (malformed template, unresolvable binding, invalid expression,
// unlayoutable content) must produce a Diagnostic, and an aborting one
// must carry SeverityError. Arm A reconciles that with Go's ordinary
// error-return convention rather than departing from either: it wraps
// each existing, pre-3.6 error type, so no existing error type,
// message or errors.As target changes (AC8).
//
// This is FORCED, not preferred (D-3.6.3's own grounds): arm B (leave
// SeverityError unexercised) needed both a scope cut and an AD-14
// amendment with no reason on the table to amend it, and arm C
// (construct SeverityError but keep it caller-unreachable) is
// D-000.9's rejected shape at the type level.

// RenderError is the first publicly constructible SeverityError value
// in this module's history (AC8). It carries the AD-14 Diagnostic AC4's
// four Error modes require, WITHOUT replacing the underlying error a
// caller may already be matching on with errors.As:
//
//   - Error() returns the wrapped error's OWN message, byte-for-byte —
//     printing a RenderError looks exactly like printing what it wraps,
//     because AC8 requires "no existing error type, message or
//     errors.As target changes";
//   - Unwrap() returns the wrapped error, so errors.As/errors.Is walks
//     straight through to it — a *layout.OverflowError,
//     *template.LoadError or expr.KernelOverflowError inside a
//     RenderError is exactly as recoverable as it always was.
//
// A caller wanting the STABLE CODE (AC9: "match on the code, never on
// message text") uses errors.As(err, &renderErr) and reads
// renderErr.Diagnostic.Code — never Message, and never the wrapped
// error's own type, which stays an implementation detail this type
// does not require a caller to know about.
type RenderError struct {
	// Diagnostic is AD-14's SeverityError-carrying value: Code names
	// which of FR41's four Error modes this is; ElementID and DataPath
	// are populated where the failure mode has them (AD-10); Message
	// duplicates Err.Error() — present so a Diagnostic value taken in
	// isolation (e.g. logged, or read off Result.Diagnostics-shaped
	// tooling) is still self-describing, never because it is parsed
	// (Diagnostic.Message's own doc comment).
	Diagnostic Diagnostic

	// Err is the underlying, PRE-EXISTING error this Diagnostic
	// describes, unchanged from what this module returned before Story
	// 3.6.
	Err error
}

// Error returns Err's own message, unchanged — printing a RenderError
// is indistinguishable from printing what it wraps (AC8).
func (e *RenderError) Error() string { return e.Err.Error() }

// Unwrap exposes Err to errors.As/errors.Is, so every pre-existing
// target (*layout.OverflowError, *template.LoadError,
// expr.KernelOverflowError) keeps resolving through this wrapper.
func (e *RenderError) Unwrap() error { return e.Err }

// newRenderError builds a RenderError: a SeverityError Diagnostic
// carrying code/elementID/dataPath, wrapping err. err must be non-nil.
func newRenderError(code, elementID, dataPath string, err error) *RenderError {
	return &RenderError{
		Diagnostic: Diagnostic{
			Severity:  SeverityError,
			Code:      code,
			ElementID: elementID,
			DataPath:  dataPath,
			Message:   err.Error(),
		},
		Err: err,
	}
}

// wrapTemplateError is ParseTemplate's boundary for the load path:
// internal/template may not import the module root (AD-1) and so cannot
// construct a Diagnostic itself.
//
// A *template.LoadError always carries a Code now (Story 7.8, D-7.8.1):
// diag.CodeTemplateFieldInvalid by default, supplied by newLoadError
// itself, or one of the three overriding specific codes — the
// footer-source ones. Whichever it is,
// it is kept. Until Story 7.8 the general population arrived UNCODED and
// was bucketed under DiagCodeTemplateMalformed, whose message the WASM
// host replaces wholesale — so every located field error was destroyed
// before its author could read it. That boundary rule is unchanged and
// still correct for what it now names.
//
// DiagCodeTemplateMalformed remains FR41's "malformed template" mode
// (AC4/AC8) and is what a NON-LoadError load failure becomes: bytes that
// are not a JSON object, an unreadable value under an unknown key, or a
// MAJOR version the library cannot load — the failures that have no
// field to name, and whose messages can quote the offending document
// back.
func wrapTemplateError(err error) error {
	if le, ok := err.(*template.LoadError); ok {
		// The zero-Code fallback is retained for a LoadError value
		// constructed outside this package's two constructors; neither
		// constructor can produce one.
		code := DiagCodeTemplateMalformed
		if le.Code != "" {
			code = string(le.Code)
		}
		if code == DiagCodeSectionBreakInvalid || code == DiagCodePagesInvalid {
			// spec-section-break and SPEC-multi-pages: located at the band or
			// the page, which has no element id — the field path is its
			// location.
			return newRenderError(code, le.ElementID, le.Field, err)
		}
		return newRenderError(code, le.ElementID, "", err)
	}
	return newRenderError(DiagCodeTemplateMalformed, "", "", err)
}

// Table allocation errors identify a table total or a particular column's
// proportion, including failures reached through structural Canvas projection.
func wrapTableWidthError(err error) error {
	wrapped := wrapTemplateError(err)
	if le, ok := err.(*template.LoadError); ok {
		failure := wrapped.(*RenderError)
		failure.Diagnostic.DataPath = "column." + le.Field
		if le.Field == "width" {
			failure.Diagnostic.DataPath = "table.width"
		}
	}
	return wrapped
}

// wrapOverflowError is layout.Paginate's boundary for FR41's
// "unlayoutable content" mode (AC4/AC8, R9): an element (a text line
// or an image) taller than the content window it must fit inside
// (internal/layout.OverflowError, FR44/D-2.6.1).
//
// AC8's "no existing error type, message or errors.As target changes"
// applies to the MESSAGE too: both of this function's call sites
// previously returned fmt.Errorf("folio8: Render: %w", err) directly, so
// wrapped is built the SAME way here before being handed to
// newRenderError — Error() on the result is byte-identical to what
// Render returned before this story, and errors.As still resolves
// *layout.OverflowError through the extra Unwrap hop (RenderError ->
// the fmt-wrapped error -> the OverflowError).
func wrapOverflowError(err error) error {
	wrapped := fmt.Errorf("folio8: Render: %w", err)
	elementID := ""
	if oe, ok := err.(*layout.OverflowError); ok {
		elementID = oe.ElementID
	}
	return newRenderError(DiagCodeContentUnlayoutable, elementID, "", wrapped)
}

func expressionRuntimeError(elementID, field string, err error) error {
	code := DiagCodeExpressionInvalid
	var absent *bind.PathAbsentError
	if errors.As(err, &absent) {
		code = DiagCodeBindingPathAbsent
		err = fmt.Errorf("%s: %w", field, err)
		field = absent.Path
	}
	return newRenderError(code, elementID, field, err)
}
