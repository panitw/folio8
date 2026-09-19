package arch

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSyntacticGuardIsBlindToAnInferredFractionalType is the second half
// of Story 2.3a's AC3 red-proof, and it asserts the gap D-000.25 named
// rather than describing it in prose.
//
// The claim: scanNoFloat64's findFloatOccurrences — this module's
// existing AD-23 guard, which matches on the SPELLING of a type
// identifier and on fractional literals — reports ZERO on a file that
// truncates a vendor accessor's fractional return value to an integer.
// The type is inferred, so neither banned identifier is ever written and
// no fractional literal appears. The guard is green on code AD-23
// forbids.
//
// The complementary half lives in the lint module, where the type-aware
// rule lives: lint/internal/rules/floattyped_test.go's
// TestFloatTypedFixtureScan points ScanFloatTypedValues at this SAME
// fixture directory and asserts it reports exactly the violating file
// and not the compliant one. Two guards, two mechanisms, one invariant —
// and this test is the one that says why the second was needed.
//
// PRESENCE PRECONDITIONS FIRST (D-000.21 sharpened: assert on the
// artifact that carries the property, and PROVE it carries it). "The
// syntactic scanner reports zero" is the expected result for an empty
// file, a missing file, and a file of comments. So before asserting
// zero, this test proves the fixture exists, is non-empty, actually
// performs the integer truncation of the vendor call, and genuinely
// contains neither banned identifier nor any fractional literal. Without
// those four, a green result here would prove nothing at all.
func TestSyntacticGuardIsBlindToAnInferredFractionalType(t *testing.T) {
	internalRoot, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve internal/ root: %v", err)
	}
	fixture := filepath.Join(filepath.Dir(internalRoot), "testdata", "lint", "no-float-typed-value", "violating_inferred.go")

	src, rerr := os.ReadFile(fixture)
	if rerr != nil {
		t.Fatalf("presence precondition: the violating fixture this test makes claims about could not be read at %s: %v", fixture, rerr)
	}
	if len(src) == 0 {
		t.Fatalf("presence precondition: the violating fixture at %s is empty — a syntactic scan of nothing reports zero for the wrong reason", fixture)
	}
	text := string(src)

	// (a) It really does truncate the vendor's fractional accessor to an
	//     integer. This is the whole subject of the assertion below.
	const wantTruncation = "int64(f.HorizontalAdvance(gid))"
	if !strings.Contains(text, wantTruncation) {
		t.Fatalf("presence precondition: the violating fixture at %s does not contain %q, so it is not the hazard this test claims the syntactic guard is blind to", fixture, wantTruncation)
	}

	// (b) Neither banned identifier appears anywhere in it, not even in
	//     a comment, so the fixture's two readings cannot be confused.
	//     The identifiers are assembled from parts here on purpose: this
	//     file is itself walked by TestNoFloat64UnderModule below, which
	//     would otherwise report this test as a violation of the very
	//     rule it exists to reason about.
	banned := []string{"float" + "32", "float" + "64"}
	for _, b := range banned {
		if strings.Contains(text, b) {
			t.Fatalf("presence precondition: the violating fixture at %s contains the identifier %q — the fixture must carry NO spelled fractional type, or it does not isolate the inferred-type blind spot", fixture, b)
		}
	}

	// (c) It contains no fractional literal either. Parsing is the exact
	//     test the scanner itself applies, so this precondition is
	//     checked the same way rather than by string matching.
	fset := token.NewFileSet()
	parsedFixture, perr := parser.ParseFile(fset, fixture, src, parser.AllErrors)
	if perr != nil {
		t.Fatalf("presence precondition: the violating fixture at %s does not parse: %v", fixture, perr)
	}

	// The assertion itself: the existing syntactic scanner, run over
	// this file, finds nothing.
	got := findFloatOccurrences(fset, parsedFixture, filepath.Base(fixture))
	if len(got) != 0 {
		t.Fatalf(
			"the syntactic guard reported %d finding(s) on %s:\n%s\n"+
				"That is not a pass. This fixture exists to demonstrate that the syntactic guard CANNOT see an "+
				"inferred fractional type; if it can now, this test's premise is stale and the fixture no longer "+
				"isolates the blind spot — fix the fixture, do not delete the assertion.",
			len(got), fixture, strings.Join(got, "\n"))
	}

	// And a control on the same mechanism, so "reports zero" is known to
	// be a property of the fixture rather than of the scanner: the
	// retained no-float64 fixture tree, scanned the same way, reports
	// more than zero. Without this, a scanner that had been broken to
	// return nil unconditionally would pass the assertion above.
	controlDir := filepath.Join(filepath.Dir(internalRoot), "testdata", "lint", "no-float64")
	controlFindings, _, cerr := scanNoFloat64(controlDir)
	if cerr != nil {
		t.Fatalf("control: scan %s: %v", controlDir, cerr)
	}
	if len(controlFindings) == 0 {
		t.Fatalf(
			"control: the syntactic scanner reported zero findings on %s, which is a tree of deliberate violations — "+
				"so its zero on the inferred-type fixture above says nothing about inferred types",
			controlDir)
	}
}
