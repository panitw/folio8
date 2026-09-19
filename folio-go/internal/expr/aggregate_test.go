package expr

// AC21/AC21a: "the substituted-window injection red-proof" — a
// resolver returning only a CONTIGUOUS SLICE of a collection (the
// shape a per-page resolver would take, which AD-11 forbids in
// production, R3) must produce a total PROVABLY DIFFERENT from the
// hand-computed whole-collection total. bind's real exprResolver is
// proven never to take this shape (internal/bind/aggregate_test.go's
// TestAggregateBypassesRowRootEvenWhenAliasShadowsCollectionName,
// AC20) — this file's windowedResolver exists ONLY to demonstrate
// what would go wrong if it ever did, never as a production type.

import (
	"strings"
	"testing"
)

// windowedResolver simulates a per-page-scoped collection resolver:
// CollectionLength and ProjectCollection both report only the window
// [start:end) of full — INTERNALLY CONSISTENT with each other (AC4's
// length invariant holds WITHIN this resolver), which is exactly why
// AC21a notes this mutant is not independent evidence for AC4: an
// internally-consistent-but-externally-wrong resolver is invisible to
// the length invariant alone.
type windowedResolver struct {
	full       []Decimal
	start, end int
}

func (w windowedResolver) Resolve(path []string) (Value, error) {
	return Value{}, errAbsentTestPath(path)
}

func (w windowedResolver) CollectionLength(path []string) (int, error) {
	return w.end - w.start, nil
}

func (w windowedResolver) ProjectCollection(path []string) ([]Value, error) {
	out := make([]Value, 0, w.end-w.start)
	for _, d := range w.full[w.start:w.end] {
		out = append(out, Value{Kind: KindNumber, Num: d})
	}
	return out, nil
}

func errAbsentTestPath(path []string) error {
	return &pathAbsentError{path: path}
}

type pathAbsentError struct{ path []string }

func (e *pathAbsentError) Error() string {
	return "test: windowedResolver: path " + strings.Join(e.path, ".") + " is absent"
}

// TestSubstitutedWindowRedProofDiffersFromWholeCollectionTotal is
// AC21: the slice is chosen (elements 0:2 of a 4-element collection)
// so the sliced total PROVABLY DIFFERS from the hand-computed WHOLE
// total (D-000.45: a literal, never a direction) — 10.00+20.00=30.00,
// stated as the sliced total in this comment per AC21's own
// instruction, against a whole of 10.00+20.00+30.00+40.00=100.00.
func TestSubstitutedWindowRedProofDiffersFromWholeCollectionTotal(t *testing.T) {
	full := []Decimal{
		{Coefficient: 1000, Exponent: -2}, // 10.00
		{Coefficient: 2000, Exponent: -2}, // 20.00
		{Coefficient: 3000, Exponent: -2}, // 30.00
		{Coefficient: 4000, Exponent: -2}, // 40.00
	}
	wholeTotal := Decimal{Coefficient: 10000, Exponent: -2} // 100.00 — hand-computed literal.
	wantSlicedTotal := Decimal{Coefficient: 3000, Exponent: -2}

	w := windowedResolver{full: full, start: 0, end: 2}

	e, perr := Parse("sum(t.a)")
	if perr != nil {
		t.Fatalf("Parse: %v", perr)
	}
	if cerr := Check(e); cerr != nil {
		t.Fatalf("Check: %v", cerr)
	}
	v, _, err := Eval(e, w, testFC(), "e1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if v.Num == wholeTotal {
		t.Fatal("presence precondition: the injected window must NOT happen to reproduce the whole-collection total — otherwise this red-proof discriminates nothing")
	}
	if v.Num != wantSlicedTotal {
		t.Fatalf("got %+v, want the STATED sliced total %+v", v.Num, wantSlicedTotal)
	}
	t.Logf("red-proof: a resolver returning only the contiguous slice [0:2) produces %+v — provably different from the hand-computed whole-collection total %+v. AD-11 forbids this shape in production; bind's real exprResolver is proven never to take it (internal/bind's AC20 test)", v.Num, wholeTotal)

	// AC21a — HONESTY NOTE: this mutant is INTERNALLY self-consistent
	// (its own CollectionLength agrees with its own ProjectCollection
	// length), so it ALSO reddens AC4's length invariant if compared
	// against the collection's own numbers — but that is NOT
	// independent evidence for AC4: internal/bind's
	// TestOption1RedProofProjectCollectionOmitsNulls (AC12a) is the
	// mutant that supplies AC4's independent teeth, because it is the
	// one that leaves sum() UNCHANGED while still breaking the
	// invariant. This test's own length invariant, checked here,
	// merely demonstrates it is internally consistent with itself,
	// which is exactly the property that makes it invisible to a
	// length-invariant-only check in the first place.
	n, _ := w.CollectionLength([]string{"t"})
	projected, _ := w.ProjectCollection([]string{"t", "a"})
	if len(projected) != n {
		t.Fatalf("presence precondition: windowedResolver must be internally self-consistent (len(projected)=%d, CollectionLength=%d) — that is the whole point of AC21a's honesty note", len(projected), n)
	}
	if n == len(full) {
		t.Fatal("presence precondition: the window must be narrower than the true collection, or this red-proof proves nothing")
	}
}
