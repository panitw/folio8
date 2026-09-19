package bind

// Story 3.3: the collection seam (R1), the four-state discrimination
// (R8), and the null-as-zero-observation arithmetic (R7), exercised
// end to end through exprResolver — the SAME type text.go's Resolve
// uses — and, for sum()/count()/avg(), through the real
// expr.Parse/Check/Eval pipeline, so these tests prove the whole seam
// as authors will actually reach it ("{{sum(transactions.amount)}}"),
// not just CollectionLength/ProjectCollection in isolation.

import (
	"errors"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/expr"
)

// mustDecodeAggregate is decodeguard_test.go's mustDecode, restated
// here rather than shared, to keep this file's fixtures self-contained
// and independently readable.
func mustDecodeAggregate(t *testing.T, js string) Value {
	t.Helper()
	v, err := DecodeData([]byte(js))
	if err != nil {
		t.Fatalf("DecodeData(%s): %v", js, err)
	}
	return v
}

// evalAggregateExpr parses, checks and evaluates src against data
// (root-relative — R3, no row/params involved) — the whole pipeline a
// "{{...}}" placeholder actually drives.
func evalAggregateExpr(t *testing.T, src string, data Value) (expr.Value, []expr.Caveat, error) {
	t.Helper()
	e, perr := expr.Parse(src)
	if perr != nil {
		t.Fatalf("Parse(%q): %v", src, perr)
	}
	if cerr := expr.Check(e); cerr != nil {
		t.Fatalf("Check(%q): %v", src, cerr)
	}
	resolver := exprResolver{scope: NewScope(data, Value{Kind: KindObject}), elementID: "e1"}
	return expr.Eval(e, resolver, testFormatContext(), "e1")
}

// --- AC1-AC6: the seam itself ---

// TestProjectCollectionPreservesDataOrder is AC2: exactly one Value
// per element, in DATA ORDER — never sorted, deduplicated or filtered.
// The fixture is order-distinguishable (strictly decreasing), so any
// reordering, dedup, or filtering would be visible.
func TestProjectCollectionPreservesDataOrder(t *testing.T) {
	data := mustDecodeAggregate(t, `{"t":[{"a":30},{"a":10},{"a":20},{"a":10}]}`)
	r := exprResolver{scope: NewScope(data, Value{Kind: KindObject}), elementID: "e1"}
	got, err := r.ProjectCollection([]string{"t", "a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"30", "10", "20", "10"}
	if len(got) != len(want) {
		t.Fatalf("got %d elements, want %d: %v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Kind != expr.KindNumber || got[i].Num.Coefficient != mustAtoi64(t, w) {
			t.Errorf("element %d: got %#v, want numeric %s (order/identity must be preserved, never sorted or deduplicated)", i, got[i], w)
		}
	}
}

func mustAtoi64(t *testing.T, s string) int64 {
	t.Helper()
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			t.Fatalf("mustAtoi64(%q): not a plain digit string", s)
		}
		n = n*10 + int64(c-'0')
	}
	return n
}

// TestCollectionLengthNeverConsultsProjectedField is AC3/R5: count()
// STRUCTURALLY calls CollectionLength and nothing else. Proven, not by
// reading the source, but by a fixture whose elements are missing the
// very field sum()/avg() over the same path would need: if
// CollectionLength ever inspected an element's field, this fixture
// would make it error; it does not.
func TestCollectionLengthNeverConsultsProjectedField(t *testing.T) {
	data := mustDecodeAggregate(t, `{"t":[{"other":"1"},{},{"other":"2"}]}`)
	r := exprResolver{scope: NewScope(data, Value{Kind: KindObject}), elementID: "e1"}
	n, err := r.CollectionLength([]string{"t"})
	if err != nil {
		t.Fatalf("CollectionLength must succeed even though every element is missing \"amount\": %v", err)
	}
	if n != 3 {
		t.Fatalf("got %d, want 3", n)
	}
	// R5, stated verbatim in this test per AC13's own instruction: count
	// is a property of the collection; sum and avg are properties of a
	// projection over it. The same collection's projection over "amount"
	// (a field no element here has) must fail — proving count's success
	// above was not a fluke of a lenient projection.
	if _, err := r.ProjectCollection([]string{"t", "amount"}); err == nil {
		t.Fatal("presence precondition: ProjectCollection over a field no element has should fail — if it didn't, count succeeding wouldn't demonstrate anything")
	}
}

// TestBareCollectionPathToSumIsLocatedErrorAtEvaluation is AC6: a bare
// collection path (no trailing projected field) passed to sum()/avg()
// is a located Error at EVALUATION — Check cannot decide this without
// data (a path's shape alone does not say whether it names a
// collection).
func TestBareCollectionPathToSumIsLocatedErrorAtEvaluation(t *testing.T) {
	data := mustDecodeAggregate(t, `{"t":[{"a":"1"},{"a":"2"}]}`)
	_, perr := expr.Parse("sum(t)")
	if perr != nil {
		t.Fatalf("Parse(\"sum(t)\"): unexpected syntax error: %v", perr)
	}
	e, _ := expr.Parse("sum(t)")
	if cerr := expr.Check(e); cerr != nil {
		t.Fatalf("Check(\"sum(t)\") must succeed — a bare collection path is only decidable against DATA: %v", cerr)
	}
	_, _, err := evalAggregateExpr(t, "sum(t)", data)
	if err == nil {
		t.Fatal("AC6 VIOLATED: sum() over a bare collection path (no projected field) must be a located Error at evaluation")
	}
}

// --- AC12/AC12a: the declarative table and its option-1 red-proof ---

func TestAggregateDeclarativeTable(t *testing.T) {
	cases := []struct {
		name       string
		json       string
		wantSum    expr.Decimal
		wantCount  int64
		wantAvgNum bool // false only for the zero-length row (Warning, no number)
		wantAvg    expr.Decimal
	}{
		{
			name:       "ordinary projection",
			json:       `{"t":[{"a":10.00},{"a":20.00},{"a":30.00}]}`,
			wantSum:    expr.Decimal{Coefficient: 6000, Exponent: -2},
			wantCount:  3,
			wantAvgNum: true,
			wantAvg:    expr.Decimal{Coefficient: 20000000, Exponent: -6}, // 20.000000
		},
		{
			// R7: null is a ZERO OBSERVATION. sum=1+0+3=4, count=3
			// (null counts), avg=4/3=1.3333 at scale (0+4).
			name:       "null is a zero observation",
			json:       `{"t":[{"a":1},{"a":null_placeholder},{"a":3}]}`,
			wantSum:    expr.Decimal{Coefficient: 4, Exponent: 0},
			wantCount:  3,
			wantAvgNum: true,
			wantAvg:    expr.Decimal{Coefficient: 13333, Exponent: -4},
		},
		{
			name:       "all-null, N=3",
			json:       `{"t":[{"a":null_placeholder},{"a":null_placeholder},{"a":null_placeholder}]}`,
			wantSum:    expr.Decimal{Coefficient: 0, Exponent: 0},
			wantCount:  3,
			wantAvgNum: true,
			wantAvg:    expr.Decimal{Coefficient: 0, Exponent: -4}, // 0.0000
		},
		{
			name:       "zero-length collection",
			json:       `{"t":[]}`,
			wantSum:    expr.Decimal{Coefficient: 0, Exponent: 0},
			wantCount:  0,
			wantAvgNum: false,
		},
		{
			// Story 3.3 finisher pass, Finding 10: R7 extended ONE LEVEL
			// UP (text.go's file-level comment on splitCollectionPath) —
			// a null COLLECTION PATH itself (as opposed to a null
			// element inside a real array, the "null is a zero
			// observation" row above) is ALSO one zero observation:
			// CollectionLength reports 1, ProjectCollection reports one
			// KindNull element. Before this pass this was asserted only
			// at the resolver level
			// (TestFourCollectionStatesDiscriminated/"explicit null
			// collection") and never through sum/count/avg — this row
			// closes that gap. sum=0, count=1, avg=0/1=0.0000 at scale
			// (max operand scale 0 + avgExtraScale 4).
			name:       "null collection path (the path itself is null, not an element within it)",
			json:       `{"t":null_placeholder}`,
			wantSum:    expr.Decimal{Coefficient: 0, Exponent: 0},
			wantCount:  1,
			wantAvgNum: true,
			wantAvg:    expr.Decimal{Coefficient: 0, Exponent: -4},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// null_placeholder isn't valid JSON on its own — substitute
			// literal "null" so each case's json field above reads
			// self-documenting in a gofmt'd table without vet
			// complaining about a bare identifier.
			js := strings.ReplaceAll(c.json, "null_placeholder", "null")
			data := mustDecodeAggregate(t, js)

			sumV, _, err := evalAggregateExpr(t, "sum(t.a)", data)
			if err != nil {
				t.Fatalf("sum: unexpected error: %v", err)
			}
			if sumV.Num != c.wantSum {
				t.Errorf("sum: got %+v, want %+v", sumV.Num, c.wantSum)
			}

			countV, _, err := evalAggregateExpr(t, "count(t)", data)
			if err != nil {
				t.Fatalf("count: unexpected error: %v", err)
			}
			if countV.Num.Coefficient != c.wantCount || countV.Num.Exponent != 0 {
				t.Errorf("count: got %+v, want {%d,0}", countV.Num, c.wantCount)
			}

			// AC4: THE LENGTH INVARIANT, asserted on EVERY fixture here,
			// including the all-null one.
			r := exprResolver{scope: NewScope(data, Value{Kind: KindObject}), elementID: "e1"}
			n, lerr := r.CollectionLength([]string{"t"})
			if lerr != nil {
				t.Fatalf("CollectionLength: unexpected error: %v", lerr)
			}
			projected, perr := r.ProjectCollection([]string{"t", "a"})
			if perr != nil {
				t.Fatalf("ProjectCollection: unexpected error: %v", perr)
			}
			if len(projected) != n {
				// Story 3.3 finisher pass (Finding 7): this was t.Fatalf,
				// which aborted the subtest before the avg assertion
				// below ever ran — the AC12a option-1 mutant's actual
				// blast radius therefore included this invariant but
				// NEVER exercised the one assertion (avg) that is the
				// length invariant's own reason for existing (R10).
				// t.Errorf lets the subtest continue: the fixture is
				// still usable after a length mismatch, and both facts
				// get to redden independently.
				t.Errorf("AC4 LENGTH INVARIANT VIOLATED: len(ProjectCollection)=%d != CollectionLength=%d", len(projected), n)
			}

			avgV, avgCaveats, aerr := evalAggregateExpr(t, "avg(t.a)", data)
			if !c.wantAvgNum {
				if aerr != nil {
					t.Fatalf("avg over zero-length: expected a Caveat, not an Error: %v", aerr)
				}
				if avgV.Kind != expr.KindNull {
					t.Errorf("avg over zero-length: expected an empty (KindNull) result, got %#v", avgV)
				}
				if len(avgCaveats) != 1 || avgCaveats[0].Kind != expr.CaveatEmptyAverage {
					t.Errorf("avg over zero-length: expected exactly one CaveatEmptyAverage, got %#v", avgCaveats)
				}
				return
			}
			if aerr != nil {
				t.Fatalf("avg: unexpected error: %v", aerr)
			}
			if len(avgCaveats) != 0 {
				t.Errorf("avg: unexpected caveats on a non-empty collection: %#v", avgCaveats)
			}
			if avgV.Num != c.wantAvg {
				t.Errorf("avg: got %+v, want %+v", avgV.Num, c.wantAvg)
			}
		})
	}
}

// TestOption1RedProofProjectCollectionOmitsNulls is AC12a: THE named
// mutant for the owner's null ruling — "implement option 1:
// ProjectCollection omits null elements" — reddening the length
// invariant (AC4) and the avg assertion, and NOTHING else: sum is
// byte-identical either way (a null contributes 0 under the owner's
// ruling; option 1 simply omits it — adding zero changes nothing), so
// only avg (whose divisor is len(projected)) can see the difference.
// This is the mutant AC4's own red-proof rests on — AC21's
// contiguous-slice mutant ALSO reddens AC4 (AC21a), so it is NOT
// independent evidence; this one is.
func TestOption1RedProofProjectCollectionOmitsNulls(t *testing.T) {
	data := mustDecodeAggregate(t, `{"t":[{"a":1},{"a":null},{"a":3}]}`)
	r := exprResolver{scope: NewScope(data, Value{Kind: KindObject}), elementID: "e1"}

	trueProjected, err := r.ProjectCollection([]string{"t", "a"})
	if err != nil {
		t.Fatalf("ProjectCollection: unexpected error: %v", err)
	}
	trueLen, err := r.CollectionLength([]string{"t"})
	if err != nil {
		t.Fatalf("CollectionLength: unexpected error: %v", err)
	}
	if len(trueProjected) != trueLen {
		t.Fatalf("presence precondition: the REAL implementation must satisfy AC4 before this red-proof means anything (got %d, want %d)", len(trueProjected), trueLen)
	}

	// mutantProjected simulates "option 1: ProjectCollection omits null
	// elements" — the owner's REJECTED arm — over the SAME data.
	var mutantProjected []expr.Value
	for _, v := range trueProjected {
		if v.Kind == expr.KindNull {
			continue // option 1's own omission
		}
		mutantProjected = append(mutantProjected, v)
	}

	// Blast radius check 1: AC4's length invariant REDDENS under the
	// mutant (2 != 3) — exactly as AC12a requires.
	if len(mutantProjected) == trueLen {
		t.Fatal("presence precondition: the mutant fixture must actually omit at least one null to demonstrate anything")
	}

	// Blast radius check 2: sum is BYTE-IDENTICAL under both arms —
	// nulls contribute 0 either way, so the mutant is invisible to sum.
	trueDecimals, err := decimalsFromProjectionForTest(trueProjected)
	if err != nil {
		t.Fatalf("unexpected error converting the real projection: %v", err)
	}
	mutantDecimals, err := decimalsFromProjectionForTest(mutantProjected)
	if err != nil {
		t.Fatalf("unexpected error converting the mutant projection: %v", err)
	}
	trueSum, err := expr.SumDecimals(trueDecimals)
	if err != nil {
		t.Fatalf("SumDecimals(true): %v", err)
	}
	mutantSum, err := expr.SumDecimals(mutantDecimals)
	if err != nil {
		t.Fatalf("SumDecimals(mutant): %v", err)
	}
	if trueSum != mutantSum {
		t.Fatalf("presence precondition VIOLATED: sum must be identical under both arms (it is what makes avg the ONLY discriminator) — true=%+v mutant=%+v", trueSum, mutantSum)
	}

	// Blast radius check 3: avg DIFFERS — the mutant's divisor is 2, the
	// true divisor is 3. This IS the owner's ruling being enforced: only
	// avg can see the option-1 mutation.
	trueAvg, err := expr.AvgDecimals(trueDecimals)
	if err != nil {
		t.Fatalf("AvgDecimals(true): %v", err)
	}
	mutantAvg, err := expr.AvgDecimals(mutantDecimals)
	if err != nil {
		t.Fatalf("AvgDecimals(mutant): %v", err)
	}
	if trueAvg == mutantAvg {
		t.Fatal("RED-PROOF FAILED: avg is identical under both arms — the option-1 mutation is invisible even to avg, so AC4's length invariant would have no independent teeth")
	}
	t.Logf("red-proof: option-1 (omit nulls) leaves sum identical (%+v) but changes avg (true=%+v, mutant=%+v) and the length invariant (true len=%d, mutant len=%d) — exactly AC12a's required blast radius", trueSum, trueAvg, mutantAvg, trueLen, len(mutantProjected))
}

// decimalsFromProjectionForTest mirrors internal/expr/aggregate.go's
// unexported decimalsFromProjection (R7's null-to-identity rule)
// without importing it (this file is package bind, testing the SEAM
// bind exposes — the conversion itself is expr's, exercised directly
// here only to compute the two comparison sums/averages this red-proof
// needs).
func decimalsFromProjectionForTest(projected []expr.Value) ([]expr.Decimal, error) {
	out := make([]expr.Decimal, len(projected))
	for i, v := range projected {
		switch v.Kind {
		case expr.KindNumber:
			out[i] = v.Num
		case expr.KindNull:
			out[i] = expr.Decimal{Coefficient: 0, Exponent: 0}
		default:
			return nil, errors.New("unexpected non-numeric, non-null projected value")
		}
	}
	return out, nil
}

// --- AC17-AC19a: four states before the kernel, and per-element defects ---

// CORRECTED (Story 3.3 finisher pass, Finding 3): the "noopCounter"
// this comment used to name never existed anywhere in the module — the
// paragraph below was reasoning, not an observation, which is exactly
// what AC18's "proven, not asserted" exists to rule out (D-000.9).
//
// What IS true, and is a property of THIS package's own import graph
// rather than of runtime behaviour: internal/bind's ProjectCollection
// and CollectionLength never call expr.SumDecimals/expr.AvgDecimals —
// this file does not import a symbol that could reach them, and
// grepping this package for "SumDecimals"/"AvgDecimals" outside test
// files returns nothing. So at THIS layer, the kernel genuinely cannot
// be called, by construction, and no instrument beyond that fact is
// meaningful to build here.
//
// The property AC18 actually cares about — that the EXPRESSION layer's
// evalSum/evalAvg (internal/expr/aggregate.go), which DO call the
// kernel, never reach it when ProjectCollection returns an error — is
// proven with a real structural instrument at internal/expr/
// routing_arch_test.go's TestKernelCallIsGuardedByProjectCollectionError,
// not here.
func TestFourCollectionStatesDiscriminated(t *testing.T) {
	t.Run("present and empty", func(t *testing.T) {
		data := mustDecodeAggregate(t, `{"t":[]}`)
		r := exprResolver{scope: NewScope(data, Value{Kind: KindObject}), elementID: "e1"}
		n, err := r.CollectionLength([]string{"t"})
		if err != nil || n != 0 {
			t.Fatalf("got (%d, %v), want (0, nil)", n, err)
		}
		projected, err := r.ProjectCollection([]string{"t", "a"})
		if err != nil || len(projected) != 0 {
			t.Fatalf("got (%v, %v), want ([], nil)", projected, err)
		}
	})
	t.Run("absent from data", func(t *testing.T) {
		data := mustDecodeAggregate(t, `{"other":1}`)
		r := exprResolver{scope: NewScope(data, Value{Kind: KindObject}), elementID: "e1"}
		if _, err := r.CollectionLength([]string{"t"}); err == nil {
			t.Fatal("CollectionLength over an absent path must be a located Error")
		}
		if _, err := r.ProjectCollection([]string{"t", "a"}); err == nil {
			t.Fatal("ProjectCollection over an absent path must be a located Error")
		}
	})
	t.Run("explicit null collection", func(t *testing.T) {
		// R7 extended one level up (see text.go's file-level comment):
		// a null COLLECTION path is one zero observation, never zero
		// elements and never an error.
		data := mustDecodeAggregate(t, `{"t":null}`)
		r := exprResolver{scope: NewScope(data, Value{Kind: KindObject}), elementID: "e1"}
		n, err := r.CollectionLength([]string{"t"})
		if err != nil || n != 1 {
			t.Fatalf("got (%d, %v), want (1, nil)", n, err)
		}
		projected, err := r.ProjectCollection([]string{"t", "a"})
		if err != nil || len(projected) != 1 || projected[0].Kind != expr.KindNull {
			t.Fatalf("got (%#v, %v), want ([{Kind:KindNull}], nil)", projected, err)
		}
	})
	t.Run("present, not an array", func(t *testing.T) {
		data := mustDecodeAggregate(t, `{"t":"not an array"}`)
		r := exprResolver{scope: NewScope(data, Value{Kind: KindObject}), elementID: "e1"}
		if _, err := r.CollectionLength([]string{"t"}); err == nil {
			t.Fatal("CollectionLength over a non-array value must be a located Error")
		}
		if _, err := r.ProjectCollection([]string{"t", "a"}); err == nil {
			t.Fatal("ProjectCollection over a non-array value must be a located Error")
		}
	})
}

// TestPerElementDefectsAC19a is AC19a: ONE fixture — a single
// projection holding one null element and one field-absent element —
// asserting a NUMBER for the first and a located Error for the
// second, proving the seam discriminates the two states a sloppy
// resolver conflates ({"amount": null} vs {}).
func TestPerElementDefectsAC19a(t *testing.T) {
	data := mustDecodeAggregate(t, `{"t":[{"amount":null},{}]}`)
	r := exprResolver{scope: NewScope(data, Value{Kind: KindObject}), elementID: "e1"}

	// Element 0 alone: a legal, non-error null observation.
	only0 := mustDecodeAggregate(t, `{"t":[{"amount":null}]}`)
	r0 := exprResolver{scope: NewScope(only0, Value{Kind: KindObject}), elementID: "e1"}
	got0, err := r0.ProjectCollection([]string{"t", "amount"})
	if err != nil {
		t.Fatalf("element with explicit null field: unexpected error: %v", err)
	}
	if len(got0) != 1 || got0[0].Kind != expr.KindNull {
		t.Fatalf("element with explicit null field: got %#v, want one KindNull value", got0)
	}

	// Both elements together: the absent field on element 1 must stop
	// the projection with a located Error naming the element index.
	_, err = r.ProjectCollection([]string{"t", "amount"})
	if err == nil {
		t.Fatal("a field absent from an element must be a located Error, never coerced or skipped")
	}
	if !strings.Contains(err.Error(), "1") {
		t.Errorf("error must carry the zero-based element index (1), got: %v", err)
	}
	if !strings.Contains(err.Error(), "amount") {
		t.Errorf("error must carry the projected field path, got: %v", err)
	}
	if !strings.Contains(err.Error(), "e1") {
		t.Errorf("error must carry the element id, got: %v", err)
	}
	if !strings.Contains(err.Error(), "t") {
		t.Errorf("error must carry the collection path, got: %v", err)
	}
}

// TestPerElementWrongKindIsLocatedErrorNeverCoerced is AC19's other
// arm: a projected field present but not a number.
func TestPerElementWrongKindIsLocatedErrorNeverCoerced(t *testing.T) {
	data := mustDecodeAggregate(t, `{"t":[{"amount":"not a number"}]}`)
	r := exprResolver{scope: NewScope(data, Value{Kind: KindObject}), elementID: "e1"}
	_, err := r.ProjectCollection([]string{"t", "amount"})
	if err == nil {
		t.Fatal("a non-number projected field must be a located Error, never coerced")
	}
	if !strings.Contains(err.Error(), "string") {
		t.Errorf("error must name the field's actual kind, got: %v", err)
	}
}

// --- AC20: root-relative, proven against a shadowing alias ---

// TestAggregateBypassesRowRootEvenWhenAliasShadowsCollectionName is
// AC20/R3: a row scope whose declared alias is LITERALLY the
// collection's own name ("transactions") must not narrow
// sum(transactions.amount) to anything but the WHOLE collection,
// resolved from the data root.
func TestAggregateBypassesRowRootEvenWhenAliasShadowsCollectionName(t *testing.T) {
	data := mustDecodeAggregate(t, `{"transactions":[{"amount":10.00},{"amount":20.00},{"amount":30.00}]}`)
	// The row itself is a DIFFERENT shape entirely — if the aggregate
	// were ever resolved through the row root, this would either error
	// (no "amount" array under this row) or silently produce a
	// different, wrong total. Neither is acceptable.
	row := mustDecodeAggregate(t, `{"amount":999.99}`)
	scope := NewScope(data, Value{Kind: KindObject}).WithRow(row, "transactions")

	e, perr := expr.Parse("sum(transactions.amount)")
	if perr != nil {
		t.Fatalf("Parse: %v", perr)
	}
	if cerr := expr.Check(e); cerr != nil {
		t.Fatalf("Check: %v", cerr)
	}
	resolver := exprResolver{scope: scope, elementID: "e2"}
	v, _, err := expr.Eval(e, resolver, testFormatContext(), "e2")
	if err != nil {
		t.Fatalf("AC20 VIOLATED: sum() must resolve the whole collection from the data root even under a shadowing row alias, got error: %v", err)
	}
	want := expr.Decimal{Coefficient: 6000, Exponent: -2}
	if v.Num != want {
		t.Fatalf("AC20 VIOLATED: got %+v, want the WHOLE collection's total %+v (a row-root resolution would have produced a different, wrong number, or errored)", v.Num, want)
	}
}
