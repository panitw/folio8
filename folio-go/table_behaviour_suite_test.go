package folio8

// Story 4.7, AC9: the golden does NOT supersede the table behaviour
// suite.
//
// D-4.7.0's obligation 2, stated in full so a later reader does not have
// to re-derive it from a story file: the four statement goldens and the
// table behaviour suite are TWO INDEPENDENT PRODUCERS OF THE SAME
// ANSWERS. The golden pins bytes and cannot say whether they are right;
// the behaviour suite asserts properties and cannot say whether the
// bytes moved. Neither replaces the other, and that pairing is the
// construction that has reliably worked all programme. Do not retire,
// thin, or fold the behaviour suite into the goldens.
//
// A CORRECTION TO A FIGURE THAT HAS BEEN CIRCULATING. D-4.7.0 records
// that Story 4.6's mutation "reddened 46 top-level tests, all in the
// table behaviour suite". That is a MUTATION BLAST RADIUS — what one
// mutation happened to redden — and not the size of the population this
// guard protects. Measured at this story's baseline (df8cbcc), counting
// top-level `func Test…` in package folio8:
//
//	table_render_test.go        17   Story 4.1
//	table_render_row_test.go    12   Story 4.2
//	table_pagination_test.go     8   Story 4.3
//	table_header_repeat_test.go 14   Story 4.4
//	table_footer_test.go        28   Story 4.5
//	table_row_clip_test.go      11   Story 4.6
//	                            --
//	                            90
//
// plus 25 more in supporting internal packages. A guard written against
// 46 would protect a number that was never the population — so this
// guard is written against the SET OF FILE NAMES (D-2.5.1's shape:
// adding or removing a member is a one-line reviewable diff, never a
// rename), and never against a count.
//
// WHAT THIS CATCHES, AND WHAT IT DOES NOT — stated honestly, because a
// guard oversold is worse than none. It catches DELETION. It does NOT
// catch ROT: tests kept in place but hollowed out. Nothing mechanical in
// this project catches that, and pretending otherwise would be exactly
// the "checkbox nobody can fail" this story was warned against. The real
// protection against rot is the per-observable deletion screen (D-000.85)
// applied by every future story that touches table code, and the six
// files are named here so a later reader knows what population that
// screen must still redden.
//
// AND A SECOND LIMITATION, ON THE SET-EQUALITY HALF, recorded for the
// same reason (this story's review, Finding 12). The "on disk and not
// declared" direction observes the population by GLOBBING
// `table_*_test.go`. That is a NAMING CONVENTION, not the population: a
// table behaviour test file added as `statement_table_pagination_test.go`
// or `row_clip_test.go` is invisible to this guard and can never trigger
// that arm. The direction that matters for the job this guard was given —
// the six declared files must not silently disappear — is unaffected and
// is red-proved. The advertised phrase is "set equality over the
// population"; what is implemented is set equality over the files the
// convention names, and a reader is owed that difference. Any future
// table behaviour file that does not begin `table_` must be added to
// declaredTableBehaviourSuite BY HAND, because nothing here will notice
// its absence.

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// declaredTableBehaviourSuite is THE LIST — the second producer, by
// name. Removing a file from table behaviour coverage is a one-line
// diff to this list, made deliberately and reviewed, rather than a
// silent deletion nothing notices.
var declaredTableBehaviourSuite = []string{
	"table_proportions_test.go",          // Proportional authoring — exact allocation, structure, refusals and PDF geometry
	"table_render_test.go",               // Story 4.1 — cell rendering, header row, column geometry
	"table_render_row_test.go",           // Story 4.2 — row scope, per-row binding, empty collections
	"table_pagination_test.go",           // Story 4.3 — row-atomic pagination
	"table_header_repeat_test.go",        // Story 4.4 — the header repeats on continuation pages
	"table_footer_test.go",               // Story 4.5 — footer aggregates, orphan avoidance
	"table_row_clip_test.go",             // Story 4.6 — a row taller than the page
	"table_alternating_row_test.go",      // Story 4.8 — odd collection-index alternate fills
	"table_header_style_test.go",         // Story 12.3 — the header/alt-row authoring commands and the headerStyle writer census
	"table_ruled_form_test.go",           // SPEC-table-rules — the per-page frame and rules, the per-page floor and its push, perimeter ownership, the two table commands and the packed label projection
	"table_rules_test.go",                // SPEC-table-rules — the table's own box, the boundary-addressed interior rules, the ruled area's floor and the packed column label
	"table_header_align_padding_test.go", // spec-table-cell-padding-header-align-info — per-column header alignment and the table's own cell padding
}

// TestTableBehaviourSuiteIsNotSupersededByTheGolden asserts every
// declared file is present and carries top-level tests.
//
// "Present" alone would be satisfied by an empty file, which is
// precisely the rot shape this guard cannot catch in general — so it at
// least refuses the degenerate case it CAN see, and reports the
// per-file top-level test count it found (D-000.9: report the number, so
// a collapse from 28 to 1 is visible to a human reading the log even
// though no assertion fails on it).
func TestTableBehaviourSuiteIsNotSupersededByTheGolden(t *testing.T) {
	root := repoRootFromTest(t)
	moduleRoot := filepath.Join(root, "folio-go")

	if len(declaredTableBehaviourSuite) == 0 {
		t.Fatal("vacuity guard: the declared table behaviour suite is empty, so this guard protects nothing")
	}

	// The declared set, present on disk.
	counts := map[string]int{}
	for _, name := range declaredTableBehaviourSuite {
		path := filepath.Join(moduleRoot, name)
		src, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("declared table behaviour file %s is MISSING (%v).\n\n"+
				"The four statement goldens (fixtures/statement-*) do NOT supersede this suite: a golden pins "+
				"bytes and cannot say whether they are right, and the behaviour tests assert properties and "+
				"cannot say whether the bytes moved. If this file was genuinely retired, remove its line from "+
				"declaredTableBehaviourSuite in the same commit, with the ruling that authorised it (D-2.5.1).",
				name, err)
			continue
		}
		n := countTopLevelTestFuncs(string(src))
		counts[name] = n
		if n == 0 {
			t.Errorf("declared table behaviour file %s carries ZERO top-level tests — it is present but empty, which is indistinguishable from deletion for every purpose this guard serves", name)
		}
	}

	// ...and the declared set is a SET: no duplicates.
	seen := map[string]bool{}
	for _, name := range declaredTableBehaviourSuite {
		if seen[name] {
			t.Errorf("declaredTableBehaviourSuite lists %s twice", name)
		}
		seen[name] = true
	}

	// SET EQUALITY, IN BOTH DIRECTIONS, and this half is what makes the
	// guard's own deletion screen possible: removing a name from the
	// declared list while leaving the file on disk must REDDEN. Without
	// it, quietly shortening the list would be invisible — the guard
	// would simply check fewer things and still report green, which is
	// the exact shape D-2.5.1 replaced ("a count in a name rots on every
	// future member; assert the SET").
	observed, gerr := filepath.Glob(filepath.Join(moduleRoot, "table_*_test.go"))
	if gerr != nil {
		t.Fatalf("glob table_*_test.go: %v", gerr)
	}
	if len(observed) == 0 {
		t.Fatal("vacuity guard: the glob found no table_*_test.go files at all, so set equality below would compare two empty sets")
	}
	for _, path := range observed {
		name := filepath.Base(path)
		if name == "table_behaviour_suite_test.go" {
			continue // this guard itself, not a member of the population it guards
		}
		if !seen[name] {
			t.Errorf(
				"%s is a table behaviour test file on disk and is NOT in declaredTableBehaviourSuite.\n\n"+
					"Add it as a ONE-LINE entry naming the story it belongs to. The declared list is the "+
					"record of what the SECOND PRODUCER consists of; a file missing from it is one a future "+
					"reader will not know to keep (D-2.5.1).",
				name)
		}
	}

	names := make([]string, 0, len(counts))
	for n := range counts {
		names = append(names, n)
	}
	sort.Strings(names)
	total := 0
	for _, n := range names {
		total += counts[n]
	}
	t.Logf("second-producer witness — %d declared table behaviour files present, %d top-level tests between them: %v",
		len(names), total, func() []string {
			out := make([]string, len(names))
			for i, n := range names {
				out[i] = n + "=" + itoaForTest(int64(counts[n]))
			}
			return out
		}())
}

// countTopLevelTestFuncs counts `func TestX(` declarations at column 0.
// It is a text count and says nothing about what those tests assert —
// which is exactly the limitation stated in this file's header, kept
// visible rather than papered over.
func countTopLevelTestFuncs(src string) int {
	n := 0
	for _, line := range splitLinesForTest(src) {
		if len(line) > len("func Test") && line[:len("func Test")] == "func Test" {
			n++
		}
	}
	return n
}

func splitLinesForTest(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}
