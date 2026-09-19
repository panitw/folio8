// Package floatdiscrimination is Story 3.3's AC10 demonstration
// (D-3.3.7), landed here rather than under folio-go/ because there is
// no location under that module where a landed, executing float64
// mutant can live: folio-go/internal/arch_test.go's Layer 1 walks
// EVERY .go file under folio-go/, including _test.go, flagging the
// bare identifier float64/float32 and any token.FLOAT literal, with no
// allowlist mechanism at all; lint/internal/rules/floattyped.go's Layer
// 2 is pointed at the folio-go module root in both scopes. Both name
// folio-go/ positively and have nothing to exempt (D-3.1a.1 corrected,
// D-000.24: no exemption is ever added to either layer).
//
// hashmatrix/ is the established escape hatch: hashmatrix/probe/main.go
// (Story 1.2, AC8) already states the rationale this package reuses —
// no require, no replace, no go.work entry naming folio-go, and never
// imported by it, so it sits outside AD-23's scope BY CONSTRUCTION.
// This package adds NO dependency on folio-go (go.mod is untouched):
// the zero-dependency property is the legality argument, not a
// convenience, so D-000.61's corpus A is restated here as plain
// (coefficient, exponent) integer pairs rather than imported as
// folio-go/internal/expr.Decimal values.
//
// The exact total is pinned as the SAME literal
// folio-go/internal/bind/aggregate_precision_test.go's
// TestSumIsExactOnD00061CorpusA asserts —
// {Coefficient: 1234567890123488, Exponent: -2} — two INDEPENDENT
// producers agreeing on one pinned side, never two live computations
// compared against each other (D-000.19's shape).
package floatdiscrimination

import (
	"math"
	"testing"
)

// corpusACoefficients restates D-000.61's discriminating corpus A —
// 12345678901234.56 + 32x0.01, all at exponent -2 — as plain int64
// coefficients, in the corpus's own DECLARED order (the large amount
// first).
func corpusACoefficients() []int64 {
	out := make([]int64, 0, 33)
	out = append(out, 1234567890123456)
	for i := 0; i < 32; i++ {
		out = append(out, 1)
	}
	return out
}

// corpusAExponent is every operand's shared exponent (2 decimal
// places, e.g. "0.01" and "12345678901234.56" alike).
const corpusAExponent = -2

// exactCoefficient is the pinned exact total at corpusAExponent:
// 12345678901234.56 + 32*0.01 = 12345678901234.88 exactly.
const exactCoefficient int64 = 1234567890123488

// sumValueLevelFloat64 is the HONEST mutant D-000.61 (extension)
// requires: each operand's own DECIMAL VALUE — coefficient x
// 10^exponent, e.g. the number 0.01 itself, not a pre-scaled integer —
// is formed as a float64 and accumulated as a dollar amount, then the
// total is re-quantised back to a coefficient at exponent at the end.
// 0.01 has no exact binary float64 representation; this is what a
// naive float64 implementation of sum() would actually do.
func sumValueLevelFloat64(coefficients []int64, exponent int) int64 {
	scale := math.Pow(10, float64(exponent))
	var total float64
	for _, c := range coefficients {
		total += float64(c) * scale
	}
	return int64(math.Round(total / scale))
}

// sumCoefficientLevelFloat64 is D-000.61 (extension)'s NEGATIVE
// CONTROL: every operand here already shares one exponent, so summing
// the ALREADY-ALIGNED integer coefficients through float64 — never
// forming the decimal value 0.01 itself — is exact for any coefficient
// below 2^53 (~9.007e15; corpusA's largest coefficient is ~1.23e15).
// This introduces the float64 TYPE without the float64 ERROR, and must
// never be mistaken for AC10's demonstration (D-000.61 extension's own
// named false-alarm shape).
func sumCoefficientLevelFloat64(coefficients []int64) int64 {
	var total float64
	for _, c := range coefficients {
		total += float64(c)
	}
	return int64(total)
}

// reversed returns a new slice with in's elements in reverse order,
// never mutating in.
func reversed(in []int64) []int64 {
	out := make([]int64, len(in))
	for i, v := range in {
		out[len(in)-1-i] = v
	}
	return out
}

// TestValueLevelFloat64MutantMissesExactAnswerInDeclaredOrder is AC10's
// value-assertion half, measured live against folio-go's own
// SumDecimals in this story's finisher pass (working-tree mutation,
// reverted before commit — never landed under folio-go/) before being
// pinned here as a permanent, dependency-free demonstration: forward
// (declared) order produced {Coefficient: 1234567890123487}, one
// satang short of the exact {1234567890123488}.
func TestValueLevelFloat64MutantMissesExactAnswerInDeclaredOrder(t *testing.T) {
	got := sumValueLevelFloat64(corpusACoefficients(), corpusAExponent)
	if got == exactCoefficient {
		t.Fatalf("presence precondition: D-3.3.7 measured the forward-order value-level float64 mutant "+
			"missing the exact answer by one satang, but this run got the exact value %d — the corpus has "+
			"stopped discriminating and AC10's demonstration needs a new corpus", got)
	}
	t.Logf("forward (declared) order: got %d, want the exact %d — miss confirmed, as measured", got, exactCoefficient)
}

// TestValueLevelFloat64MutantOrderInvarianceReddens is AC10's
// order-invariance half: SumDecimals guarantees the SAME total
// regardless of operand order (float64 addition is not associative,
// so a naive accumulator does not). The reversed order is measured to
// land on the exact answer by coincidence (D-3.3.7) — this is D-000.61's
// own founding lesson recurring one layer up — so only the
// order-invariance property, not the value comparison alone, reddens
// unconditionally under this mutant.
func TestValueLevelFloat64MutantOrderInvarianceReddens(t *testing.T) {
	fwd := sumValueLevelFloat64(corpusACoefficients(), corpusAExponent)
	rev := sumValueLevelFloat64(reversed(corpusACoefficients()), corpusAExponent)
	if fwd == rev {
		t.Fatalf("RED-PROOF FAILED: forward (%d) and reversed (%d) order produced the SAME total under "+
			"the honest float64 mutant — this corpus no longer demonstrates float64's order-dependence", fwd, rev)
	}
	t.Logf("forward %d != reversed %d (reversed measured to land on the exact %d by coincidence) — "+
		"order-invariance reddens under this mutant, exactly as SumDecimals' own order-invariance test "+
		"requires it NOT to", fwd, rev, exactCoefficient)
}

// TestCoefficientLevelFloat64MutantIsExactBothOrders is D-000.61
// (extension)'s negative control, as an EXECUTING ASSERTION rather
// than a test comment that can decay silently without anyone noticing:
// the coefficient-level mutant must be exact and order-invariant in
// BOTH orders, or the corpus and the mutant have both drifted from
// what D-000.61 (extension) actually measured.
func TestCoefficientLevelFloat64MutantIsExactBothOrders(t *testing.T) {
	fwd := sumCoefficientLevelFloat64(corpusACoefficients())
	rev := sumCoefficientLevelFloat64(reversed(corpusACoefficients()))
	if fwd != exactCoefficient {
		t.Fatalf("presence precondition: coefficient-level mutant, forward order, got %d, want exact %d — "+
			"the negative control itself broke", fwd, exactCoefficient)
	}
	if rev != exactCoefficient {
		t.Fatalf("presence precondition: coefficient-level mutant, reversed order, got %d, want exact %d — "+
			"the negative control itself broke", rev, exactCoefficient)
	}
}
