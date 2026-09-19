package rules

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFloatTypedProductionScan is the AC2 production caller: the real
// folio-go tree, Tests false, asserted to report zero findings.
//
// MEASURED TRANSITION, RECORDED WITH BOTH NUMBERS (AC5's red-proof).
// Before Story 2.3a's fix to internal/fontset/fontset.go, this exact
// invocation — ScanFloatTypedValues(<repo>/folio-go, false) — reported
// FOUR findings in ONE file: fontset.go:328 and :329 (AdvanceForRune's
// vendor call and the read of its result) and :565 and :566 (the same
// pair inside Subset, which builds the PDF /W width table on every
// render that draws text). After the fix it reports ZERO. The syntactic
// guard in folio-go/internal/arch_test.go reported zero on BOTH sides of
// that transition, which is the gap D-000.25 named.
//
// The vacuity guard reads the checker's OWN returned stats (Major 5 of
// Story 1.3's QA review), never a second, independently-derived walk:
// injecting `if true { return nil, FloatTypedStats{}, nil }` as
// ScanFloatTypedValues' first statement leaves an unrelated re-walk
// untouched but zeroes every assertion below.
func TestFloatTypedProductionScan(t *testing.T) {
	root := repoRootFromTest(t)
	moduleRoot := filepath.Join(root, "folio-go")

	findings, stats, err := ScanFloatTypedValues(moduleRoot, false)
	if err != nil {
		t.Fatalf("scan folio-go module root %s: %v", moduleRoot, err)
	}

	assertVisited(t, stats, ".", "internal/fontset")
	if stats.FilesParsed == 0 {
		t.Fatal("vacuity guard: checker's own stats report 0 files parsed under the folio-go module root")
	}
	// The statistic that would make "a checker that resolved nothing"
	// visible: a loader that produced no type information reports zero
	// findings exactly as a clean tree does, and reports zero typed
	// expressions, which a clean tree never does.
	if stats.TypedExprs == 0 {
		t.Fatal("vacuity guard: checker's own stats report 0 expressions successfully typed under the folio-go module root")
	}

	if len(findings) > 0 {
		var msgs []string
		for _, f := range findings {
			msgs = append(msgs, f.Rule+": "+f.Message)
			if f.Rule != RuleNoFloatTypedValue {
				t.Errorf("finding at %s carries rule id %q, want %q", f.Path, f.Rule, RuleNoFloatTypedValue)
			}
		}
		t.Fatalf(
			"float-typed value expression(s) found under the folio-go module root (AD-23) — this is a defect found, "+
				"not a guard problem; decline the accessor, do not narrow the scan:\n%s",
			strings.Join(msgs, "\n"))
	}
}

// TestFloatTypedTestScopeInventory is the AC2 test-scope caller: the
// same checker, Tests true, over the same module.
//
// THIS IS AN INVENTORY, NOT AN EXEMPTION, AND THE DIFFERENCE IS THE
// POINT. Nothing is excused by name. Adding a float-typed expression to
// any _test.go file under folio-go fails this test; removing one fails
// it too. D-2.1.3's and D-000.15's rotting-list objection does not
// attach, because a named exemption grows silently while an enumeration
// on the page cannot.
//
// The inventory holds FIVE SITES across THREE FILES, and each is
// legitimate for its own, separately recorded reason:
//
//   - shaping_expectations_test.go is the negative control for
//     TestShapedExpectationsObservability. It reads the vendor accessor
//     DELIBERATELY, because reproducing the pre-2.3 path exactly is what
//     makes it a control. Deleting it would delete the control.
//
//   - internal/fontset/fontset_test.go's
//     TestSubsetPinnedInstancesProduceDifferentTags is D-2.2.4's sanctioned V5
//     tag-discrimination test, which calls the vendor's PinAxisLocation
//     with the untyped constant 700. That constant takes the parameter's
//     type, so it is a float-typed value expression with no banned
//     identifier and no fractional literal anywhere — an *ast.BasicLit
//     of kind INT. It is the second, independent blind spot in the
//     syntactic guard (D-2.2.4 correction), and it is why the comments
//     that claimed AD-23 made PinAxisLocation unreachable were false.
//
//   - internal/fontset/vendorboundary_test.go is the THIRD FILE, and it
//     was added BY Story 2.3a rather than inherited. Recording how it
//     got here matters, because the story's own AC2 predicted this list
//     would still hold exactly two entries afterwards. It carries THREE
//     of the five sites: the `hhea` row, the `hmtx` row, and the intact
//     anchor those two are measured against (Story 2.3a Finding 6),
//     which is read once and reused precisely so it books one entry
//     rather than two.
//
//     What happened is the inventory working. AC8 requires the
//     enumeration's load-bearing rows to be DERIVED from the vendor
//     rather than restated, and two of those rows are claims about
//     (*ot.Face).HorizontalAdvance itself: that it returns the face's
//     unitsPerEm when `hhea` or `hmtx` is absent. Asserting anything at
//     all about a function with a fractional return type requires a
//     value expression of that type — there is no integer spelling of
//     that claim, and that asymmetry IS the row (ot.ParseHmtxFromFont,
//     the integer path, reports an error where the accessor
//     substitutes). So AC8's derivation obligation and AC2's predicted
//     count were in tension, and the tension resolves in favour of
//     deriving the claim.
//
//     The entry was NOT quietly absorbed: adding it turned this test
//     red, it was read, and it is written down here with its reason —
//     which is the whole difference between an inventory and an
//     exemption. Anyone who disagrees can see exactly what was added and
//     why, in a diff.
//
// Narrowing this rule to non-test files was considered and rejected: the
// existing syntactic guard walks _test.go files, so a type-aware rule
// that did not would be strictly weaker in file scope than the guard it
// strengthens — a silent regression dressed as a fix.
//
// The comparison is BY SITE — (file, rule, LINE) — never by count
// (D-1.3.3 amended): a scan finding the right NUMBER of wrong things
// must still fail.
//
// CORRECTION, Story 2.3a Finding 2 (Major). This test used to compare by
// (file, rule) and its comment claimed, as AC2 did, that "adding a
// float-typed expression to any _test.go file fails the build". THAT WAS
// FALSE for any file already on the list: every additional site inside an
// enumerated file was invisible, and so was removing one of two. The gap
// was live, not hypothetical — vendorboundary_test.go already carried TWO
// sites (the `hhea` and `hmtx` rows below) while the list could name only
// one, so the tree held four sanctioned sites and this test asserted
// three and reported ok. The comment and the assertion now agree, and the
// thing they agree on is the stronger of the two readings.
func TestFloatTypedTestScopeInventory(t *testing.T) {
	root := repoRootFromTest(t)
	moduleRoot := filepath.Join(root, "folio-go")

	got, stats, err := ScanFloatTypedValues(moduleRoot, true)
	if err != nil {
		t.Fatalf("scan folio-go module root %s with tests: %v", moduleRoot, err)
	}

	assertVisited(t, stats, ".", "internal/fontset")
	if stats.FilesParsed == 0 {
		t.Fatal("vacuity guard: checker's own stats report 0 files parsed under the folio-go module root (tests included)")
	}
	if stats.TypedExprs == 0 {
		t.Fatal("vacuity guard: checker's own stats report 0 expressions successfully typed under the folio-go module root (tests included)")
	}

	// Five SITES across three files. Each is sanctioned for the reason
	// given beside it; the line numbers pin them individually so a sixth
	// cannot arrive unnoticed.
	want := []findingSite{
		// The pre-2.3 negative control for TestShapedExpectationsObservability.
		{Path: "shaping_expectations_test.go", Rule: RuleNoFloatTypedValue, Line: 357},
		// D-2.2.4's sanctioned V5 tag-discrimination test: PinAxisLocation(…, 700).
		{Path: "internal/fontset/fontset_test.go", Rule: RuleNoFloatTypedValue, Line: 529},
		// The INTACT anchor for gid 36's advance (Story 2.3a Finding 6),
		// read once and reused, so the two rows below assert a
		// substitution rather than a coincidence.
		{Path: "internal/fontset/vendorboundary_test.go", Rule: RuleNoFloatTypedValue, Line: 178},
		// The `hhea` row: HorizontalAdvance(36) when hhea is absent.
		{Path: "internal/fontset/vendorboundary_test.go", Rule: RuleNoFloatTypedValue, Line: 226},
		// The `hmtx` row: the same claim with hmtx absent. This is the
		// site the file-keyed inventory could not represent.
		{Path: "internal/fontset/vendorboundary_test.go", Rule: RuleNoFloatTypedValue, Line: 233},
	}
	assertExactFindingSites(t, got, want)
}

// TestFloatTypedFixtureScan is the first half of AC3's red-proof: the
// checker, pointed at the retained fixture tree, reports EXACTLY the
// violating file and NOT the compliant one — by file and rule.
//
// The second half lives in folio-go's own suite, where the syntactic
// scanner lives: folio-go/internal/arch_blindspot_test.go points
// findFloatOccurrences at this same violating fixture file and asserts
// it reports zero.
func TestFloatTypedFixtureScan(t *testing.T) {
	root := repoRootFromTest(t)
	fixtureRoot := filepath.Join(root, "folio-go", "testdata", "lint", "no-float-typed-value")

	got, stats, err := ScanFloatTypedValues(fixtureRoot, false)
	if err != nil {
		t.Fatalf("scan fixture tree %s: %v", fixtureRoot, err)
	}
	if stats.TypedExprs == 0 {
		t.Fatal("vacuity guard: checker's own stats report 0 expressions successfully typed in the fixture tree — a checker that resolved nothing reports zero findings exactly as a clean tree does")
	}
	if stats.FilesParsed < 2 {
		t.Fatalf("vacuity guard: the fixture tree carries a violating file AND a compliant file; the checker reports %d files parsed, so one half of the red-proof was not read", stats.FilesParsed)
	}

	want := []Finding{
		{Path: "violating_inferred.go", Rule: RuleNoFloatTypedValue},
	}
	assertExactFindings(t, got, want)
}

// TestFloatTypedFindingNamesResolvedTypeAndPosition is AC1's message
// requirement: a finding must name the RESOLVED TYPE and the
// expression's position, not "a float was found". A message that does
// not name the type leaves the reader unable to tell an inferred float
// from a spelled one, which is the entire distinction this rule exists
// to draw.
//
// The expected type name is spelled out here as a literal, independently
// of the production code that builds the message, so that changing what
// the checker reports moves only one side.
func TestFloatTypedFindingNamesResolvedTypeAndPosition(t *testing.T) {
	root := repoRootFromTest(t)
	fixtureRoot := filepath.Join(root, "folio-go", "testdata", "lint", "no-float-typed-value")

	got, _, err := ScanFloatTypedValues(fixtureRoot, false)
	if err != nil {
		t.Fatalf("scan fixture tree %s: %v", fixtureRoot, err)
	}
	if len(got) == 0 {
		t.Fatal("presence precondition: the fixture scan reported no findings at all, so there is no message to make claims about")
	}

	// The vendor accessor the violating fixture calls resolves to this
	// type. Assembled from parts so this test file does not itself
	// contain the identifier as a single token — folio-go's own
	// module-wide syntactic guard walks _test.go files, and this file is
	// in lint, but keeping the two modules' fixtures legible about what
	// they do and do not spell is the point of AC3's fixture.
	wantType := "float" + "32"

	f := got[0]
	if !strings.Contains(f.Message, wantType) {
		t.Errorf("finding message does not name the resolved type %q: %s", wantType, f.Message)
	}
	if !strings.Contains(f.Message, "violating_inferred.go:") {
		t.Errorf("finding message does not name the expression's file and line: %s", f.Message)
	}
	if f.Line <= 0 {
		t.Errorf("finding carries no line number: %+v", f)
	}
	if f.Path != "violating_inferred.go" {
		t.Errorf("finding path %q is not relative to the scanned root", f.Path)
	}
}

// TestFloatTypedScanFailsLoudlyOnAnUnloadableTree is the AC1
// "never zero findings on a tree that did not type-check" assertion,
// red-proved against a directory that is not a Go package tree at all.
// A checker that returns (nil, zero, nil) for a tree it could not read
// is indistinguishable from a clean scan.
func TestFloatTypedScanFailsLoudlyOnAnUnloadableTree(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-directory")

	findings, _, err := ScanFloatTypedValues(missing, false)
	if err == nil {
		t.Fatalf("scanning a non-existent tree %s returned no error and %d findings — a scan that cannot read its target must fail, never report zero", missing, len(findings))
	}
}

// TestFloatTypedScanFailsLoudlyOnATreeThatDoesNotTypeCheck is AC1's
// SECOND loud-failure assertion, and it is the one D-1.3.11 actually
// names: *"packages.Load's nil top-level error is not sufficient"*.
//
// Story 2.3a Finding 4 (Minor) found this branch committed but
// unguarded. TestFloatTypedScanFailsLoudlyOnAnUnloadableTree above
// points the scanner at a directory that does not exist, which fails at
// the packages.Load call — precisely the branch the ruling says is not
// enough. The per-package `Errors` sweep, which is what implements the
// ruling, was exercised by nothing, so a refactor that dropped it would
// have left the suite green while the checker reported zero findings on
// a tree it could not read.
//
// The fixture parses and does not type-check. Both halves are asserted
// here rather than assumed, because a fixture that failed to PARSE would
// send the scanner down a third path and this test would silently stop
// measuring the sweep.
func TestFloatTypedScanFailsLoudlyOnATreeThatDoesNotTypeCheck(t *testing.T) {
	root := repoRootFromTest(t)
	fixture := filepath.Join(root, "folio-go", "testdata", "lint", "float-typed-untypecheckable")
	src := filepath.Join(fixture, "untypecheckable.go")

	// Precondition 1: the fixture exists and is non-empty.
	data, readErr := os.ReadFile(src)
	if readErr != nil {
		t.Fatalf("retained fixture %s: %v", src, readErr)
	}
	if len(data) == 0 {
		t.Fatalf("retained fixture %s is empty — it cannot demonstrate anything", src)
	}

	// Precondition 2: it still carries the undefined symbol that makes
	// it untypeable. If someone "fixed" the fixture, this test would
	// otherwise pass vacuously against a clean tree.
	const undefinedSymbol = "thisSymbolDoesNotExist"
	if !strings.Contains(string(data), undefinedSymbol) {
		t.Fatalf("retained fixture %s no longer contains the undefined symbol %q — it can no longer make the tree untypeable", src, undefinedSymbol)
	}

	// Precondition 3: it PARSES. This is what separates "does not
	// type-check" from "is not valid Go", and only the former reaches
	// the Errors sweep.
	if _, parseErr := parser.ParseFile(token.NewFileSet(), src, data, parser.AllErrors); parseErr != nil {
		t.Fatalf("retained fixture %s must parse cleanly and fail only at type-check; go/parser rejected it: %v", src, parseErr)
	}

	findings, stats, err := ScanFloatTypedValues(fixture, false)
	if err == nil {
		t.Fatalf("scanning the untypeable tree %s returned no error and %d findings (stats %+v) — a checker that cannot type a tree must fail loudly, never report zero (D-1.3.11)", fixture, len(findings), stats)
	}
	if findings != nil {
		t.Errorf("scan of %s returned %d findings alongside its error; the loud-failure path must return no findings", fixture, len(findings))
	}

	// The error must come from the per-package Errors sweep, not from
	// packages.Load. These are different sentences in the source and
	// only one of them satisfies D-1.3.11.
	const sweepMarker = "type information unavailable under"
	if !strings.Contains(err.Error(), sweepMarker) {
		t.Errorf("error did not come from the per-package Errors sweep (expected it to contain %q): %v", sweepMarker, err)
	}
	if !strings.Contains(err.Error(), undefinedSymbol) {
		t.Errorf("error did not name the undefined symbol %q that made the tree untypeable, so it does not locate the problem: %v", undefinedSymbol, err)
	}
}

// assertVisited checks, from the checker's own reported stats, that it
// visited each named directory — BY NAME, so a run that visited only
// part of the tree cannot pass by reporting the same zero findings a
// whole-tree run reports.
func assertVisited(t *testing.T, stats FloatTypedStats, want ...string) {
	t.Helper()
	seen := map[string]bool{}
	for _, d := range stats.DirsVisited {
		seen[d] = true
	}
	for _, w := range want {
		if !seen[w] {
			t.Fatalf("vacuity guard: checker's own stats did not report visiting directory %q, got %v", w, stats.DirsVisited)
		}
	}
}
