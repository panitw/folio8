package rules

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestMapRangeProductionScan is the AC17 production caller: it scans
// the real folio-go/internal/ tree and asserts zero findings,
// non-vacuously — from the scanner's OWN reported MapRangeStats (Major
// 5, this story's QA review), not a second, independent re-walk. A
// second, independent walk cannot see a dead scanner: injecting
// `if true { return nil, MapRangeStats{}, nil }` as ScanMapRange's first
// statement never touches an unrelated walk built the same way the
// checker itself walks, but it DOES zero out the checker's own reported
// stats — which is exactly what this guard now asserts against.
//
// TypedRangeStmts is NOT asserted non-zero here: F-7 measured that every
// `range` site under folio-go/internal/ today is inside a `_test.go`
// file (D-1.3.5 exempts test files, so ScanMapRange never loads them —
// packages.Config.Tests is false), so the real tree legitimately types
// zero range statements in non-test files right now. That statistic IS
// asserted non-zero in TestMapRangeFixtureScan below, where the fixture
// tree's non-test range statements — including Blocker 1's (D-1.3.11)
// retained cross-package regression — give it something real to count.
// Warrant: F-7, measured — zero map ranges under internal/ today, so
// this restriction is being taken before a caller exists.
func TestMapRangeProductionScan(t *testing.T) {
	root := repoRootFromTest(t)
	internalDir := filepath.Join(root, "folio-go", "internal")

	findings, stats, err := ScanMapRange(internalDir)
	if err != nil {
		t.Fatalf("scan folio-go/internal/: %v", err)
	}

	visitedGeom, visitedPDF := false, false
	for _, dir := range stats.DirsVisited {
		if dir == "geom" {
			visitedGeom = true
		}
		if dir == "pdf" {
			visitedPDF = true
		}
	}
	if !visitedGeom || !visitedPDF {
		t.Fatalf("vacuity guard: scanner's own stats did not report visiting package directories \"geom\" and \"pdf\", got %v", stats.DirsVisited)
	}
	if stats.FilesParsed == 0 {
		t.Fatal("vacuity guard: scanner's own stats report 0 files parsed under internal/")
	}

	if len(findings) > 0 {
		var msgs []string
		for _, f := range findings {
			msgs = append(msgs, f.Message)
		}
		t.Fatalf("map range(s) found under internal/ (D-1.3.5, NFR1.d):\n%s", strings.Join(msgs, "\n"))
	}
}

// TestMapRangeUnderModule is Story 2.2's D-2.2.3 extension: a SECOND
// production caller on the SAME checker (ScanMapRange itself is
// unchanged, and the slices.Sorted(maps.Keys(...)) escape hatch is
// unchanged) — the exact D-1.6.7 move ("the render path's extent
// starts in package folio8, so the hazard does too"), the identical
// precedent folio-go/internal/arch_test.go's TestNoFloat64UnderModule
// already applied to the no-float64 guard.
//
// Measured, at creation (B15, V4): TestMapRangeProductionScan above
// only ever scans folio-go/internal/, so "internal/ must never range
// the FontSet" was VACUOUSLY true — no internal/ package can even name
// folio8.FontSet without an import cycle, and FontSet is declared and
// consumed entirely at the module root (fontset.go, render.go), outside
// that root. Extending the scan to the WHOLE MODULE is what makes this
// guard non-vacuous for the file that actually touches a FontSet.
//
// TestMapRangeProductionScan (internal/-scoped) is kept UNCHANGED —
// two callers, one checker, exactly as D-1.6.7 left the float64 guard.
func TestMapRangeUnderModule(t *testing.T) {
	root := repoRootFromTest(t)
	moduleRoot := filepath.Join(root, "folio-go")

	findings, stats, err := ScanMapRange(moduleRoot)
	if err != nil {
		t.Fatalf("scan module root %s: %v", moduleRoot, err)
	}

	// Vacuity guard (D-000.9 + D-000.13), from the scanner's OWN
	// reported stats: confirm it actually visited the module-root
	// package (where render.go and fontset.go live, dir ".") and
	// "internal", not just the directory TestMapRangeProductionScan
	// already covers on its own.
	visitedModuleRootPkg, visitedInternal := false, false
	for _, dir := range stats.DirsVisited {
		if dir == "." {
			visitedModuleRootPkg = true
		}
		if dir == "internal" || strings.HasPrefix(dir, "internal"+string(filepath.Separator)) || strings.HasPrefix(dir, "internal/") {
			visitedInternal = true
		}
	}
	if !visitedModuleRootPkg {
		t.Fatalf("vacuity guard: scanner's own stats did not report visiting the module-root package (\".\"), got %v", stats.DirsVisited)
	}
	if !visitedInternal {
		t.Fatalf("vacuity guard: scanner's own stats did not report visiting \"internal\" from the module root, got %v", stats.DirsVisited)
	}
	if stats.FilesParsed == 0 {
		t.Fatal("vacuity guard: scanner's own stats report 0 files parsed under the module root")
	}

	if len(findings) > 0 {
		// D-000.13: report BY RULE ID AND MESSAGE, never by exit status.
		// This loop previously collected f.Message alone and never
		// referenced f.Rule at all, so the Dev Agent Record's claim that
		// the guard "fired by rule id (map-range) and message" was not
		// supported by anything this test does. A finding's rule id is
		// the stable, greppable half of its identity — the half a reader
		// uses to find the rule that produced it — and it is precisely
		// the half that was missing.
		var msgs []string
		for _, f := range findings {
			msgs = append(msgs, f.Rule+": "+f.Path+": "+f.Message)
			if f.Rule != RuleMapRange {
				t.Errorf(
					"finding at %s carries rule id %q, want %q — a finding emitted by this scan under "+
						"another rule id would be unattributable to the guard that produced it",
					f.Path, f.Rule, RuleMapRange,
				)
			}
		}
		t.Fatalf(
			"map range(s) found under the WHOLE MODULE (AD-1, NFR1.d, D-2.2.3) — this is a defect found, "+
				"not a guard problem; fix the range, do not narrow the scan:\n%s",
			strings.Join(msgs, "\n"),
		)
	}
}

// TestMapRangeFixtureScan is the AC1 fixture caller, red-proving AC16's
// both polarities on retained fixtures at
// folio-go/testdata/lint/map-range/ (never under folio-go/internal/,
// F-10): a non-test file ranging a map is reported; the
// slices.Sorted(maps.Keys(m)) idiom, a range over a slice, a string, an
// integer, and a `_test.go` file ranging a map are all not reported.
//
// crosspkg/producer and crosspkg/consumer are Blocker 1's (D-1.3.11)
// retained regression: a named map type, a type alias, and a
// map-returning function, all declared in one package and ranged over
// in another. This is the exact shape the withdrawn dependency-free
// detector silently missed (proven by the QA review on a copy of the
// real tree: internal/pdf already imports internal/geom), and is now
// reported because ScanMapRange resolves the whole subtree as one
// coherent package graph.
func TestMapRangeFixtureScan(t *testing.T) {
	root := repoRootFromTest(t)
	fixtureRoot := filepath.Join(root, "folio-go", "testdata", "lint", "map-range")

	got, stats, err := ScanMapRange(fixtureRoot)
	if err != nil {
		t.Fatalf("scan fixture tree %s: %v", fixtureRoot, err)
	}

	// Non-vacuity, from the scanner's own stats (Major 5): the fixture
	// tree's non-test files carry real range statements over maps,
	// slices, strings and integers, so a scanner that silently resolved
	// nothing would report TypedRangeStmts == 0 here even though
	// TestMapRangeProductionScan legitimately cannot make this
	// assertion (F-7: no non-test range statements exist under
	// folio-go/internal/ today).
	if stats.TypedRangeStmts == 0 {
		t.Fatal("vacuity guard: scanner's own stats report 0 range statements successfully typed in the fixture tree — this is the statistic that would have caught Blocker 1 (D-1.3.11)")
	}

	want := []Finding{
		{Path: "violating_map_range.go", Rule: RuleMapRange},
		{Path: filepath.Join("crosspkg", "consumer", "violating_cross_package.go"), Rule: RuleMapRange},
		{Path: filepath.Join("crosspkg", "consumer", "violating_cross_package_alias.go"), Rule: RuleMapRange},
	}
	assertExactFindings(t, got, want)
}

// TestMapRangeFailureMessageNamesEscapeHatch is RP-7 / AC15: the
// failure message must contain the escape-hatch idiom verbatim. It
// asserts against wantIdiom, a literal string copy written independently
// of the EscapeHatch constant the production code uses to build the
// message, plus EscapeHatch == wantIdiom as a second, separate
// assertion. Blocker 2 (this story's QA review): the original test
// compared the message against the very constant that produced it
// (strings.Contains(f.Message, EscapeHatch)), so replacing EscapeHatch's
// value with anything — including "TODO" — moved both sides together
// and the test never reddened. Two independent literals close that.
func TestMapRangeFailureMessageNamesEscapeHatch(t *testing.T) {
	const wantIdiom = "for _, k := range slices.Sorted(maps.Keys(m)) { v := m[k]; … }"
	if EscapeHatch != wantIdiom {
		t.Fatalf("EscapeHatch constant drifted from the idiom this test independently spells out:\n got:  %q\n want: %q", EscapeHatch, wantIdiom)
	}

	root := repoRootFromTest(t)
	fixtureRoot := filepath.Join(root, "folio-go", "testdata", "lint", "map-range")

	got, _, err := ScanMapRange(fixtureRoot)
	if err != nil {
		t.Fatalf("scan fixture tree %s: %v", fixtureRoot, err)
	}
	found := false
	for _, f := range got {
		if strings.Contains(f.Message, wantIdiom) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a finding whose message contains the escape hatch verbatim: %q", wantIdiom)
	}
}
