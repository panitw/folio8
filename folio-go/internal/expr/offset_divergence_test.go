package expr

import (
	"testing"

	"github.com/panitw/folio8/folio-go/internal/template"
)

// STORY 12.2 / D-12.C: THE ANTI-DIVERGENCE GUARD FOR `utcOffset`.
//
// Two predicates read the same document field from two packages:
// internal/template's IsUTCOffset decides whether the file LOADS, and this
// package's parseUTCOffsetMinutes decides whether every formatDate in it
// RENDERS. They are not one function and cannot be — template (rank 2) cannot
// import expr (rank 3) — so nothing but this test stands between them.
//
// WHY A GUARD ABOUT A `template` PREDICATE LIVES IN `expr`. This is the only
// package from which BOTH sides can be called: parseUTCOffsetMinutes is
// unexported here, and expr already imports template (decimal.go, locale.go),
// while the reverse import is forbidden by the stage-rank wall. The same test
// written in internal/template would not compile — it could not name
// parseUTCOffsetMinutes at all — so the guard is sited on the only side of the
// wall that can see both, deliberately, and not because it is "about" expr.
//
// WHAT THE GAP COST, MEASURED. Before Story 12.2 the loader's pattern was
// `^[+-][0-9]{2}:[0-9]{2}$`: a document declaring `"+99:99"` LOADED, and then
// every formatDate in it failed at render with `expr: invalid UTC offset
// "+99:99"` — a refusal the author reached only by rendering, attributable to
// nothing they could see. D-12.C repaired the loader's predicate rather than
// leaving the command to range-check on top of it, and this test is what keeps
// the two from drifting apart again.
//
// `Z` IS THE ONE EXPECTED ASYMMETRY, AND IT IS DECLARED HERE BY NAME RATHER
// THAN FILTERED OUT OF THE CANDIDATE SET. A guard that quietly dropped it would
// hide the very fact D-12.C uncovered. parseUTCOffsetMinutes serves TWO
// populations with one function: the document's own `utcOffset` field
// (the parseUTCOffsetMinutes call in evalFormatDate, which passes fc.UTCOffset)
// and the offsets embedded in RFC 3339 timestamps arriving in report DATA (the
// two calls that pass m[8], in ParseRFC3339 and instantMsFromValue). LINE
// NUMBERS ARE DELIBERATELY NOT USED HERE: this comment block is itself long
// enough to move them, and a stale citation inside prose invoking D-000.10
// against accurate-sounding claims is the defect that prose is about.
//
// `Z` is RFC 3339's canonical UTC spelling and the data caller requires it; the
// document field must refuse it on ONE ground — it is not `±HH:MM`, the syntax
// folio-format.md's `utcOffset` field-table row states, so excluding it
// implements the
// format exactly as excluding `+99:99` does. The loader admits `-00:00`
// precisely because that IS `±HH:MM`. Syntax is the whole test. So the function
// correctly admits the UNION of what its two callers need, and the loader
// correctly admits less.

// utcOffsetAsymmetry is the ONE SEMANTIC asymmetry: a value that means a real
// offset to one predicate and is refused by the other on purpose. D-12.C
// declares it by name rather than letting the candidate set quietly omit it,
// because a guard that filters `Z` out hides the very fact that decision
// uncovered.
var utcOffsetAsymmetry = map[string]string{
	"Z": "RFC 3339's canonical UTC spelling. parseUTCOffsetMinutes must admit it for offsets embedded in report DATA (the m[8] calls in ParseRFC3339 and instantMsFromValue); the loader must refuse it on one ground — it is not ±HH:MM, the syntax folio-format.md's `utcOffset` field-table row states, so excluding it implements the format exactly as excluding +99:99 does. The loader admits -00:00 precisely because that IS ±HH:MM; syntax is the whole test (D-12.C, D-12.C.4).",
}

// utcOffsetEvaluatorLaxity is A FINDING THIS GUARD FOUND, recorded rather than
// filtered out, and deliberately NOT merged into utcOffsetAsymmetry above: it
// is not a semantic disagreement about what an offset means, it is
// strconv.Atoi accepting a sign inside the hour field.
//
//	parseUTCOffsetMinutes("++7:00") == 420 and ("+-7:00") == -420
//
// because the function checks s[0], s[3] and the RANGES but hands s[1:3] to
// strconv.Atoi, which accepts "+7" and "-7".
//
// IT IS UNREACHABLE IN PRODUCTION, in both populations, which is why Story 12.2
// records it instead of repairing it (the repair is outside D-12.C's scope and
// would be a behaviour change to a function serving report data):
//
//   - the DATA path never feeds it — rfc3339Pattern's own capture group is
//     `(Z|[+-]\d{2}:\d{2})`, so the two inner fields are guaranteed digits;
//   - the DOCUMENT path never feeds it — since Story 12.2 the loader's
//     IsUTCOffset gates it, and both strings are refused there.
//
// The direction is also the harmless one: the evaluator is laxer than the
// loader, so no document that LOADS can fail to render because of it. The
// dangerous direction is asserted below with no exemptions at all.
var utcOffsetEvaluatorLaxity = map[string]string{
	"++7:00": "strconv.Atoi accepts a leading sign, so the hour field \"+7\" parses as 7. Unreachable: rfc3339Pattern guarantees digits on the data path, and IsUTCOffset refuses it on the document path.",
	"+-7:00": "the same strconv.Atoi laxity with a negative inner sign, yielding -420 minutes. Unreachable for the same two reasons.",
}

// utcOffsetCandidates spans the shapes the two predicates could plausibly
// disagree about: legal offsets across the whole range, the out-of-range values
// the old loader admitted, the syntactic near-misses, `Z`, and the two strings
// that exposed the strconv laxity above.
var utcOffsetCandidates = []string{
	"+00:00", "-00:00", "+07:00", "-07:00", "-05:30", "+14:00", "-12:00", "+23:59",
	"+99:99", "+24:00", "-24:00", "+00:60", "+30:00", "-99:00",
	"Z", "z", "UTC", "+7:00", "+0700", "07:00", "", "+07:00 ", " +07:00", "+07:0a", "++7:00", "+-7:00", "+07-00",
}

// TestLoaderAdmitsNothingTheEvaluatorCannotParse is THE LOAD-BEARING HALF, and
// it takes no exemptions at all — not `Z`, not anything. A value the document
// admits and the evaluator refuses is a file that opens, saves and then renders
// no dates at all, with the refusal reaching the author only at render time and
// attributable to nothing they can see. That is precisely the defect D-12.C
// closed, and it is never an acceptable exception.
func TestLoaderAdmitsNothingTheEvaluatorCannotParse(t *testing.T) {
	if len(utcOffsetCandidates) == 0 {
		t.Fatal("presence precondition (D-000.9): the candidate set is empty, so every assertion below is vacuous")
	}
	admitted := 0
	for _, candidate := range utcOffsetCandidates {
		if !template.IsUTCOffset(candidate) {
			continue
		}
		admitted++
		if _, err := parseUTCOffsetMinutes(candidate); err != nil {
			t.Errorf("the loader ADMITS %q and the evaluator refuses it (%v) — a document declaring it opens and then renders no dates at all (D-12.C)", candidate, err)
		}
	}
	// Non-vacuity: a candidate set the loader refuses entirely would make the
	// loop above pass while asserting nothing.
	if admitted == 0 {
		t.Fatal("the loader admitted none of the candidates, so this test asserted nothing")
	}
	// AND THE VALUE THE STORY IS ABOUT, named rather than left to the loop: it
	// used to be admitted here and refused there.
	if template.IsUTCOffset("+99:99") {
		t.Error("the loader admits +99:99 again — the exact value D-12.C repaired")
	}
}

// TestEveryEvaluatorOnlyOffsetIsDeclared is the reverse direction: the
// evaluator is allowed to be laxer than the loader, but only for values named
// in one of the two sets above, each with its reason. An undeclared one is a
// finding, and a declared one that has stopped diverging is a stale exemption
// (D-000.10).
func TestEveryEvaluatorOnlyOffsetIsDeclared(t *testing.T) {
	declared := func(candidate string) (string, bool) {
		if reason, ok := utcOffsetAsymmetry[candidate]; ok {
			return reason, true
		}
		reason, ok := utcOffsetEvaluatorLaxity[candidate]
		return reason, ok
	}
	seen := map[string]bool{}
	for _, candidate := range utcOffsetCandidates {
		documentAdmits := template.IsUTCOffset(candidate)
		_, err := parseUTCOffsetMinutes(candidate)
		evaluatorParses := err == nil
		if documentAdmits == evaluatorParses {
			if reason, ok := declared(candidate); ok {
				t.Errorf("%q is declared as a divergence (%s) but the two predicates now AGREE on it — delete the entry rather than leave a stale exemption standing (D-000.10)", candidate, reason)
			}
			continue
		}
		// The other direction has its own test and takes no exemptions.
		if documentAdmits {
			continue
		}
		seen[candidate] = true
		if _, ok := declared(candidate); !ok {
			t.Errorf("the evaluator parses %q and the loader refuses it, and nothing declares why. Either it belongs in utcOffsetAsymmetry with a semantic reason, or in utcOffsetEvaluatorLaxity as a recorded finding — an undeclared one is how a divergence becomes invisible (D-12.C).", candidate)
		}
	}
	// Every declared exemption must have been exercised. An entry naming a
	// value the candidate set never feeds is an exemption nothing tests.
	for candidate, reason := range utcOffsetAsymmetry {
		if !seen[candidate] {
			t.Errorf("%q is declared as the expected semantic asymmetry (%s) but the candidate set never exercised it as one", candidate, reason)
		}
	}
	for candidate, reason := range utcOffsetEvaluatorLaxity {
		if !seen[candidate] {
			t.Errorf("%q is recorded as evaluator laxity (%s) but the candidate set never exercised it as one", candidate, reason)
		}
	}
	// `Z` IS THE ONE SEMANTIC ASYMMETRY, stated as a count so a second one
	// cannot be added to that map without this line being edited deliberately.
	if len(utcOffsetAsymmetry) != 1 {
		t.Errorf("utcOffsetAsymmetry declares %d semantic asymmetries; D-12.C names exactly one (Z)", len(utcOffsetAsymmetry))
	}
}

// TestZIsTheDataPathsSpellingAndNotTheDocuments pins the reason the exemption
// above gives, so the exemption cannot outlive it. If `Z` ever stops being
// required by the RFC 3339 path, or ever starts being admitted by the document
// field, this test says so instead of the exemption silently continuing to
// cover a divergence nobody wants any more.
func TestZIsTheDataPathsSpellingAndNotTheDocuments(t *testing.T) {
	if template.IsUTCOffset("Z") {
		t.Error("the loader now admits Z as a document utcOffset — it is not ±HH:MM, the syntax folio-format.md's `utcOffset` field-table row states (D-12.C)")
	}
	minutes, err := parseUTCOffsetMinutes("Z")
	if err != nil {
		t.Fatalf("parseUTCOffsetMinutes refused Z: %v — RFC 3339 report data requires it (the m[8] calls in ParseRFC3339 and instantMsFromValue)", err)
	}
	if minutes != 0 {
		t.Errorf("parseUTCOffsetMinutes(\"Z\") = %d, want 0", minutes)
	}
	// AND THE DATA PATH ITSELF, not just the helper: a Z-terminated timestamp
	// in report data must still parse. This is the population that would break
	// if someone "fixed" the asymmetry by removing Z from the helper.
	if _, err := ParseRFC3339("2026-08-15T00:00:00Z"); err != nil {
		t.Fatalf("a Z-terminated RFC 3339 timestamp in report data no longer parses: %v", err)
	}
}
