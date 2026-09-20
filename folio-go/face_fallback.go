package folio8

import "fmt"

// This file declares the render path's ONE resolution-mode selector and
// the single place a caller's variadic becomes a value.
//
// WHY A VARIADIC. Render, RenderTo and Validate are public API frozen at
// folio-go/v1.0.0 (AD-22): every existing caller must keep compiling and
// keep producing the same bytes. Go offers exactly one shape for an
// optional trailing argument, and this is it. The zero value of the type
// is FaceFallbackStrict, so `Render(t, d, p, f)` — every call site that
// exists today — asks for today's behaviour by construction rather than
// by a default written down somewhere and trusted.
//
// WHY IT IS NOT A FIELD ON FontSet (D1). FontSet is map[string][]byte and
// every caller in three languages constructs one. Making it a struct to
// carry a mode would break all of them for a property that is not about
// the fonts at all: it is about what the RENDER does when the fonts run
// out.
//
// ⚠ DELIBERATELY NO String() METHOD. package folio8's root files may not
// declare a second String on a non-Severity receiver:
// TestFolio8MethodNamesAreInjective (render_arch_test.go) keys methods by
// name across the call graph, and a second String makes that map lossy.
// A caller who wants to print one prints the constant's name themselves.

// FaceFallback selects what a render does with a chain entry naming a
// face the renderer was never given.
//
// The three outcomes are ONE RULE, not three checks:
//
//   - an entry that RESOLVES renders, and needs no font set at all;
//   - an entry that cannot resolve WITH CANDIDATES AVAILABLE is either
//     refused (FaceFallbackStrict) or painted in a coverage-resolved
//     face the renderer does hold and reported (FaceFallbackSubstitute);
//   - an entry that cannot resolve WITH NO CANDIDATE AT ALL is
//     DiagCodeTextFaceAbsent under either selector.
//
// No argument check pre-empts that rule before resolution has been
// attempted, in any binding.
type FaceFallback uint8

const (
	// FaceFallbackStrict refuses a rune that no present member of its
	// chain covers when a member of that chain was never supplied, with
	// DiagCodeTextFaceAbsent. It is the ZERO VALUE, so it is what every
	// caller written before this selector existed asks for, and it is
	// what the designer's engine build depends on: page_setup.go catches
	// that code to learn which face the browser must fetch.
	FaceFallbackStrict FaceFallback = iota

	// FaceFallbackSubstitute paints such a rune in a face the renderer
	// WAS given — the document's own embedded assets first, then the
	// supplied FontSet, each in face-name order, first face that covers
	// the rune winning — and emits a DiagCodeTextFaceSubstituted Warning
	// naming the element, the rune, the face requested and the face
	// painted. A renderer holding nothing that covers the rune still
	// refuses with DiagCodeTextFaceAbsent.
	FaceFallbackSubstitute
)

// errTooManyFaceFallbacks and errUnknownFaceFallback are the two ways the
// optional selector can be malformed. Both are REFUSALS rather than
// clamps, and the reasoning belongs at the site: a caller who passed two
// selectors, or a value outside the closed set, has a bug in the code
// that computed it, and silently picking one of them for them hides that
// bug behind bytes that look fine. The engine's whole contract is that a
// render says what it did.
//
// They are sentinels so a caller — and a test — can match them with
// errors.Is rather than by comparing prose.
var (
	errTooManyFaceFallbacks = fmt.Errorf("folio8: at most one FaceFallback may be supplied")
	errUnknownFaceFallback  = fmt.Errorf("folio8: unknown FaceFallback")
)

// resolveFaceFallback is the SINGLE place the variadic becomes a value.
// Every public entry point and every internal seam that takes the
// variadic routes through it, so the internal path refuses exactly what
// the public path refuses rather than clamping to the last element.
func resolveFaceFallback(fallback []FaceFallback) (FaceFallback, error) {
	switch len(fallback) {
	case 0:
		return FaceFallbackStrict, nil
	case 1:
	default:
		return FaceFallbackStrict, fmt.Errorf("%w, got %d", errTooManyFaceFallbacks, len(fallback))
	}
	switch fallback[0] {
	case FaceFallbackStrict, FaceFallbackSubstitute:
		return fallback[0], nil
	default:
		return FaceFallbackStrict, fmt.Errorf("%w: %d", errUnknownFaceFallback, uint8(fallback[0]))
	}
}
