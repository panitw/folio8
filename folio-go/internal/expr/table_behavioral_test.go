package expr

import "testing"

// This file is AC16's BEHAVIOURAL half (DECISION-1, Arm A; D-000.59):
// the OBLIGATION removing the registered-but-unimplemented machinery
// leaves in its place is that a registered function actually
// COMPUTES — never merely that "all eight are implemented", which is
// the EVENT (it would just re-read the flag this story sets, and is
// not a replacement for it).
//
// callFixtures supplies one VALID call, per functionTable entry, over
// a resolver that can satisfy it — enough for evalCall to reach that
// entry's evaluator and return successfully. The set of keys here is
// checked against functionTable's own name set below (never the
// reverse: this file does not get to invent an entry), so this
// fixture map cannot silently go stale as the table's contents change
// shape in a future story — it can only go stale by omission, which
// TestAllTableEntriesActuallyCompute's own presence check catches.
var callFixtures = map[string]struct {
	src string
	res Resolver
}{
	"sum":          {`sum(t.a)`, sliceOperandResolver{"t.a": {{Kind: KindNumber, Num: Decimal{Coefficient: 100, Exponent: -2}}}}},
	"count":        {`count(t.a)`, sliceOperandResolver{"t.a": {{Kind: KindNumber, Num: Decimal{Coefficient: 100, Exponent: -2}}}}},
	"avg":          {`avg(t.a)`, sliceOperandResolver{"t.a": {{Kind: KindNumber, Num: Decimal{Coefficient: 100, Exponent: -2}}}}},
	"formatDate":   {`formatDate(x, "d MMMM yyyy")`, mapResolver{"x": {Kind: KindString, Str: "2026-08-15T00:00:00Z"}}},
	"formatNumber": {`formatNumber(x, "#,##0.00")`, mapResolver{"x": {Kind: KindNumber, Num: Decimal{Coefficient: 123456, Exponent: -2}}}},
	"upper":        {`upper(x)`, mapResolver{"x": {Kind: KindString, Str: "a"}}},
	"lower":        {`lower(x)`, mapResolver{"x": {Kind: KindString, Str: "A"}}},
	"if":           {`if(cond, a, b)`, mapResolver{"cond": {Kind: KindBool, Bool: true}, "a": {Kind: KindString, Str: "A"}, "b": {Kind: KindString, Str: "B"}}},
}

// sliceOperandResolver is a minimal Resolver whose ProjectCollection
// and CollectionLength both serve one fixed slice keyed by the full
// dotted path of the FIRST argument's Resolve target — enough for
// sum/count/avg's own fixtures above, which never resolve a bare path
// outside a call.
type sliceOperandResolver map[string][]Value

func (r sliceOperandResolver) Resolve(path []string) (Value, error) {
	return Value{}, errAbsent(path)
}

func (r sliceOperandResolver) CollectionLength(path []string) (int, error) {
	v, ok := r[joinPath(path)]
	if !ok {
		return 0, errAbsent(path)
	}
	return len(v), nil
}

func (r sliceOperandResolver) ProjectCollection(path []string) ([]Value, error) {
	v, ok := r[joinPath(path)]
	if !ok {
		return nil, errAbsent(path)
	}
	return v, nil
}

// TestAllTableEntriesActuallyCompute is AC16's behavioural half: each
// of functionTable's entries, called with valid arguments, returns A
// VALUE AND NO ERROR.
//
// Finding 7 (this story's QA review): the previous version's presence
// precondition was `exercised != len(functionTable)`, where exercised
// is incremented exactly once per loop iteration with no `continue`
// path — an identity that can never be false (the only early exit,
// t.Fatalf, aborts the test). It also failed at D-000.9's one job:
// were functionTable ever empty, the loop would never run and `0 != 0`
// would pass vacuously. The real protection was always the per-entry
// `t.Fatalf` on a missing callFixtures key below, which does work —
// this was a labelling defect, not a coverage hole.
//
// The repair asserts a comparison that CAN fail in both directions:
// len(functionTable) == len(callFixtures), which catches both an
// entry with no fixture (already caught, per-entry, above) AND a
// stray fixture with no matching entry (which the old precondition
// could never see, since it only ever counted successful iterations
// of functionTable's own loop).
func TestAllTableEntriesActuallyCompute(t *testing.T) {
	if len(functionTable) != len(callFixtures) {
		t.Fatalf("presence precondition (D-000.9): functionTable has %d entries, callFixtures has %d — a stray fixture or a missing one", len(functionTable), len(callFixtures))
	}
	for _, entry := range functionTable {
		fx, ok := callFixtures[entry.name]
		if !ok {
			t.Fatalf("%s: no callFixtures entry — every functionTable name must have one (AC16's witness set)", entry.name)
		}
		e, perr := Parse(fx.src)
		if perr != nil {
			t.Fatalf("%s: Parse(%q): %v", entry.name, fx.src, perr)
		}
		if cerr := Check(e); cerr != nil {
			t.Fatalf("%s: Check(%q): %v", entry.name, fx.src, cerr)
		}
		_, _, err := Eval(e, fx.res, testFC(), "e1")
		if err != nil {
			t.Errorf("%s: AC16 behavioural obligation failed: valid call %q returned an error instead of computing: %v", entry.name, fx.src, err)
		}
	}
}

func joinPath(path []string) string {
	out := path[0]
	for _, p := range path[1:] {
		out += "." + p
	}
	return out
}

func errAbsent(path []string) error {
	return errAbsentPath{joinPath(path)}
}

type errAbsentPath struct{ path string }

func (e errAbsentPath) Error() string {
	return "test: path " + e.path + " is absent from sliceOperandResolver"
}
