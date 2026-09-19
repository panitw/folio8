package expr

// Caveat is a package-local, typed non-error condition Eval can report
// alongside a successfully-resolved Value (Story 3.3, DECISION-5 /
// R9). AD-14 draws a hard line between an error (the render cannot
// proceed) and a diagnostic (the render proceeded, with a caveat worth
// surfacing) — EXPERIENCE.md:145, verbatim. avg() over a
// present-but-empty collection is exactly the second case: Story 4.2's
// own AC requires an empty-collection table to render successfully, so
// the kernel's honest "cannot average 0 operands" error (D-3.1a.2, the
// kernel is UNCHANGED by this story) must not abort the render here —
// it must become a caveat instead, with the aggregate resolving to
// empty.
//
// Caveat is NOT a folio8.Diagnostic: internal/expr may not import the
// module-root folio8 package (that rank is backwards), and Story 3.6
// owns the general diagnostic-code registry (internal/diag) — this
// type mints no code, decides no severity and constructs nothing
// folio8-shaped. It is the payload a higher-ranked caller (internal/bind,
// then folio8's own render path) has enough information to turn into
// one, without this package needing to know how.
type Caveat struct {
	// Kind is the closed set of conditions that can produce a Caveat.
	// Closed the same way functionTable is closed (table.go): adding a
	// second kind is a direction change, not a one-line edit.
	Kind CaveatKind

	// Path is the collection's dotted path exactly as the author wrote
	// it — the same convention bind.Substitution.Path already uses for
	// a bare path (text.go), so a caller presenting both together never
	// has to reconcile two different path spellings.
	Path string
}

// CaveatKind is Caveat's closed discriminant.
type CaveatKind int

const (
	// CaveatEmptyAverage: avg() was evaluated against a collection that
	// resolved (per R8's four-state discrimination) to present and
	// empty — never against an absent or wrong-kind collection, both of
	// which remain located Errors (R8), and never against an
	// all-null collection, which is a real average (R7.3), not this
	// caveat.
	CaveatEmptyAverage CaveatKind = iota
)

func (k CaveatKind) String() string {
	switch k {
	case CaveatEmptyAverage:
		return "empty-average"
	default:
		return "unknown-caveat"
	}
}
