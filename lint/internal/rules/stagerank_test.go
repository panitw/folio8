package rules

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestStageRankProductionScan is D-1.3.3's PRODUCTION caller: it points
// the pure checker at the real folio-go/internal/ tree and asserts ZERO
// findings, failing on a scan error separately from, and BEFORE, the
// zero-findings assertion (D-1.3.3 amended, RP-3b — a tree that cannot
// be read must never be silently treated as "zero findings").
//
// D-000.16's own stated precondition for the table is "green today".
// This test is that precondition, executed rather than asserted in
// prose.
//
// VACUITY GUARD (D-000.9), and it reads ScanStageRank's OWN returned
// stats rather than re-walking the tree a second, independent way
// (Major 5's precedent): a dead scanner produces exactly the
// zero-findings all-clear a healthy one does, and a second walk built
// the old way would keep reporting healthy numbers after
// `if true { return nil, stats, nil }` were injected into the checker.
// The guard names the five packages that make this scan non-trivial —
// `layout` and `pagemodel` are the packages this story CREATES, and the
// arrow between `text` and `fontset` is the one D-000.16's ranks were
// corrected for — plus a non-zero count of first-party import edges
// actually compared. `diag` was added by Story 3.6's finisher (Finding
// 14, QA review): AC1 requires `internal/diag`'s zero-first-party-import
// property to be asserted by TWO independent guards (an in-package AST
// scan plus this lint scan); without `diag` named here, `ScanStageRank`
// reporting a violation there was verified to redden, but nothing
// asserted the scan had ever actually ENTERED the package — a future
// change that caused the walk to skip it would go slack silently.
func TestStageRankProductionScan(t *testing.T) {
	root := repoRootFromTest(t)
	internalDir := filepath.Join(root, "folio-go", "internal")

	findings, stats, err := ScanStageRank(internalDir)
	if err != nil {
		t.Fatalf("scan folio-go/internal/: %v", err)
	}

	for _, pkg := range []string{"layout", "pagemodel", "pdf", "text", "fontset", "diag"} {
		if !slices.Contains(stats.PackagesVisited, pkg) {
			t.Fatalf("vacuity guard: the scanner's own stats do not report visiting package %q — a scan that never entered it reports the same zero findings a healthy scan does (D-000.9). Visited: %v",
				pkg, stats.PackagesVisited)
		}
	}
	if stats.FilesSeen == 0 {
		t.Fatal("vacuity guard: the scanner's own stats report 0 .go files parsed under folio-go/internal/")
	}
	if stats.FirstPartyImports == 0 {
		t.Fatal("vacuity guard: the scanner's own stats report 0 first-party internal import edges examined — the rank comparison never ran even once, so zero findings proves nothing")
	}

	if len(findings) > 0 {
		var msgs []string
		for _, f := range findings {
			msgs = append(msgs, f.Message)
		}
		t.Fatalf("stage-rank violations under folio-go/internal/ (D-000.16):\n%s", strings.Join(msgs, "\n"))
	}
}

// TestStageRankFixtureScan is D-1.3.3's FIXTURE caller over the RETAINED
// VIOLATING FIXTURE at folio-go/testdata/lint/stage-rank/ (never under
// folio-go/internal/, F-10). It asserts exactly the named findings BY
// FILE AND RULE, never by count (AC1, RP-3c) — matching neither a subset
// nor a superset, so deleting an expected finding fails on the "expected
// not reported" half and inventing one fails on the "unexpected
// reported" half.
//
// The fixture covers four violation shapes and three compliant shapes:
//
//	layout/       -> pdf        HIGHER rank — AD-5's OWN ARROW, named
//	                            explicitly. D-000.23 read the other way:
//	                            a guard written for a CLASS must still be
//	                            shown to cover the INSTANCE it exists for.
//	expr/         -> bind       HIGHER rank — D-1.6.1's pre-commitment.
//	pagemodel/    -> diag       EQUAL rank — the branch a plain
//	                            "forward-only" reading would miss.
//	unrankedstage/              NO rank at all — the fail-safe default,
//	                            firing with no import present.
//	template/     -> geom       lower rank: NOT reported.
//	pdf/          -> pagemodel  lower rank: NOT reported. This is the
//	                            arrow Story 2.5 actually adds, so the
//	                            table must not accidentally forbid it.
//	(root file)                 rankNoStage with no first-party import:
//	                            NOT reported.
func TestStageRankFixtureScan(t *testing.T) {
	root := repoRootFromTest(t)
	fixtureRoot := filepath.Join(root, "folio-go", "testdata", "lint", "stage-rank")

	got, stats, err := ScanStageRank(fixtureRoot)
	if err != nil {
		t.Fatalf("scan fixture tree %s: %v", fixtureRoot, err)
	}
	if stats.FilesSeen == 0 {
		t.Fatalf("vacuity guard: the scanner's own stats report 0 files parsed under %s — the retained fixture is missing, and an empty tree produces the same empty finding set a compliant one does", fixtureRoot)
	}

	want := []Finding{
		{Path: filepath.Join("layout", "violating_pdf_import.go"), Rule: RuleStageRank},
		{Path: filepath.Join("expr", "violating_bind_import.go"), Rule: RuleStageRank},
		{Path: filepath.Join("pagemodel", "violating_equal_rank.go"), Rule: RuleStageRank},
		{Path: filepath.Join("unrankedstage", "violating_unranked.go"), Rule: RuleStageRank},
	}
	assertExactFindings(t, got, want)
}

// TestStageRankMessageNamesAD5sArrow is the message half of the fixture
// caller: a finding that fires but says nothing useful is a tripwire
// whose remedy nobody can execute (D-000.37 — "a tripwire's failure
// message is executable by a human"). The layout -> pdf finding must
// name both endpoints, both ranks, and the rule that a stage passes what
// a later stage needs rather than importing it.
func TestStageRankMessageNamesAD5sArrow(t *testing.T) {
	root := repoRootFromTest(t)
	fixtureRoot := filepath.Join(root, "folio-go", "testdata", "lint", "stage-rank")

	got, _, err := ScanStageRank(fixtureRoot)
	if err != nil {
		t.Fatalf("scan fixture tree %s: %v", fixtureRoot, err)
	}

	wantPath := filepath.Join("layout", "violating_pdf_import.go")
	var msg string
	for _, f := range got {
		if f.Path == wantPath {
			msg = f.Message
			break
		}
	}
	if msg == "" {
		t.Fatalf("presence precondition: no finding reported for %s — the message assertions below would pass vacuously on an empty string", wantPath)
	}
	for _, want := range []string{`"layout"`, `"pdf"`, "rank 7", "rank 8", "STRICTLY LOWER", "rides on the VALUE"} {
		if !strings.Contains(msg, want) {
			t.Errorf("layout -> pdf finding message does not contain %q; got:\n%s", want, msg)
		}
	}
}

// TestStageRankUnrankedMessageNamesASymbolThatExists is the sibling of
// TestStageRankMessageNamesAD5sArrow, for the OTHER message this rule
// emits.
//
// It exists because the unranked-package message shipped naming
// `stageRanks` — a symbol that appears nowhere in the repository. The
// table is `stageRankTable`. The file path was right, so the cost was
// small, but D-000.37 is exactly the ruling this violated: "a tripwire's
// failure message is executable by a human" and "the remedy text is
// maintained as code, not as prose." Only the layout -> pdf message had
// its content held by a test; this branch had none, so a wrong
// identifier in it was invisible.
//
// So this asserts more than a spelling. It reads stagerank.go's own
// source and requires that the identifier the remedy names is actually
// DECLARED there. A rename of the table that leaves the message behind
// fails here, which a string-equality assertion would not have done —
// it would merely have moved the stale name from the message into the
// test.
func TestStageRankUnrankedMessageNamesASymbolThatExists(t *testing.T) {
	root := repoRootFromTest(t)
	fixtureRoot := filepath.Join(root, "folio-go", "testdata", "lint", "stage-rank")

	got, _, err := ScanStageRank(fixtureRoot)
	if err != nil {
		t.Fatalf("scan fixture tree %s: %v", fixtureRoot, err)
	}

	wantPath := filepath.Join("unrankedstage", "violating_unranked.go")
	var msg string
	for _, f := range got {
		if f.Path == wantPath {
			msg = f.Message
			break
		}
	}
	if msg == "" {
		t.Fatalf("presence precondition: no finding reported for %s — the message assertions below would pass vacuously on an empty string", wantPath)
	}

	// The remedy must say WHERE to act, WHAT to edit, and that silence is
	// not an option.
	const wantSymbol = "stageRankTable"
	for _, want := range []string{
		`"unrankedstage"`,
		"lint/internal/rules/stagerank.go",
		wantSymbol,
		"never a pass",
		"known ranks:",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("unranked-package finding message does not contain %q; got:\n%s", want, msg)
		}
	}

	// ...and the symbol it names must exist. This is the half that would
	// have caught the shipped defect: `stageRanks` satisfied every
	// substring check a human would think to write, and resolved to
	// nothing.
	srcPath := filepath.Join(root, "lint", "internal", "rules", "stagerank.go")
	src, rerr := os.ReadFile(srcPath)
	if rerr != nil {
		t.Fatalf("read %s: %v", srcPath, rerr)
	}
	if !strings.Contains(string(src), "var "+wantSymbol+" = ") {
		t.Fatalf("the remedy names %q, but %s declares no such variable — a tripwire whose remedy points at a symbol that does not exist is not executable by a human (D-000.37)", wantSymbol, srcPath)
	}
}

// spineRelPath is the architecture spine, relative to the repo root. It
// is the ANCHOR for TestSpineStageLadderMatchesStageRankTable below
// (D-000.68): a markdown file authored outside BOTH Go modules, which
// neither stagerank.go nor any code under test can move, rename or
// reword. Not the compiler and not the type system — the third anchor,
// a literal the test does not own but the code cannot edit.
var spineRelPath = filepath.Join(
	"_bmad-output", "planning-artifacts", "architecture",
	"architecture-folio-2026-08-23", "ARCHITECTURE-SPINE.md",
)

const (
	spineLadderBegin = "<!-- stage-rank-table:begin -->"
	spineLadderEnd   = "<!-- stage-rank-table:end -->"
)

// TestSpineStageLadderMatchesStageRankTable holds ARCHITECTURE-SPINE.md's
// stage ladder to stageRankTable, in order, both ways.
//
// WHY THIS EXISTS (Epic 3 boundary gate, Finding 2.3). The spine used to
// carry a hand-drawn mermaid dependency graph over this same rule, above
// the sentence "Anything not drawn is forbidden". Measured at the gate,
// that graph drew 24 arrows: it OMITTED 13 edges that existed and
// INVENTED 11 that did not — including `text --> fontset`, which is
// backwards. It had rotted in both directions across three epics with
// nothing asserting otherwise, because a hand-maintained second copy of
// a fact the compiler already holds is correct for exactly one commit.
// The graph is deleted. What replaced it is a ladder of the ranks
// themselves, and this test is the reason the spine is allowed to state
// that the ladder "is held in agreement with that table by a test".
// Without this test that sentence is D-000.28's anticipatory
// boilerplate: false from birth and reading identically to a true one.
//
// The table is authoritative; the ladder is a reading copy. If they
// disagree, this test is red and stageRankTable is right.
func TestSpineStageLadderMatchesStageRankTable(t *testing.T) {
	root := repoRootFromTest(t)
	spinePath := filepath.Join(root, spineRelPath)

	// D-000.9: a spine that cannot be read must never report as
	// agreement. A moved or renamed spine is a red somebody fixes
	// deliberately, never a skip.
	raw, err := os.ReadFile(spinePath)
	if err != nil {
		t.Fatalf("read the architecture spine at %s: %v — this test's anchor is that file's existence; if the spine moved, update spineRelPath deliberately rather than letting the ladder go unchecked", spinePath, err)
	}
	doc := string(raw)

	begin := strings.Index(doc, spineLadderBegin)
	end := strings.Index(doc, spineLadderEnd)
	if begin < 0 || end < 0 || end < begin {
		t.Fatalf("could not locate the fenced stage ladder in %s (begin marker %q found=%t, end marker %q found=%t) — a reformat that defeats the extractor must not read as agreement (D-000.9)",
			spinePath, spineLadderBegin, begin >= 0, spineLadderEnd, end >= 0)
	}

	got := parseSpineLadder(t, doc[begin+len(spineLadderBegin):end])

	// A second vacuity guard, on the SHAPE of what was extracted rather
	// than on the extraction succeeding: an extractor that silently
	// returns nothing produces exactly the "no differences" all-clear a
	// healthy one does when the table is also empty.
	if len(got) < 10 {
		t.Fatalf("the fenced stage ladder in %s parsed to only %d row(s): %v — the ladder must carry every stage, and a parse that collapses to near-nothing is a broken extractor, not agreement (D-000.9)",
			spinePath, len(got), got)
	}

	// Ordered equality, reported BOTH ways and by NAME. A count is a
	// lossy set (D-000.68): "11 rows, want 11" hides a swap, and
	// "1 difference" does not tell the reader which document to edit.
	want := make([]stageRank, len(stageRankTable))
	copy(want, stageRankTable)

	inDocNotInTable := diffStageRanks(got, want)
	inTableNotInDoc := diffStageRanks(want, got)
	for _, r := range inDocNotInTable {
		t.Errorf("the spine's ladder carries {%q, %d}, which stageRankTable does not — the TABLE is authoritative, so this row in %s is wrong", r.Name, r.Rank, spinePath)
	}
	for _, r := range inTableNotInDoc {
		t.Errorf("stageRankTable carries {%q, %d}, which the spine's ladder does not — add it to the fenced block in %s", r.Name, r.Rank, spinePath)
	}
	if len(inDocNotInTable) > 0 || len(inTableNotInDoc) > 0 {
		return // ordering is meaningless once the sets differ
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("stage ladder row %d is {%q, %d} in the spine but {%q, %d} in stageRankTable — the ladder reads in pipeline order and that order is part of what it documents",
				i, got[i].Name, got[i].Rank, want[i].Name, want[i].Rank)
		}
	}
}

// parseSpineLadder reads the markdown rows of the fenced ladder into the
// same representation stageRankTable uses, so the comparison is between
// two []stageRank and not between a table and a prose blob.
//
// The first cell names the stage as the spine writes it — "`internal/geom`"
// — and the scan root as "`internal/` — the scan root itself; …". The
// leading backticked token is what carries the name in both shapes, so
// that is what is read; the trailing prose in the scan-root row is
// deliberately free text, because it is documentation and nothing should
// force it to stay one particular sentence.
func parseSpineLadder(t *testing.T, block string) []stageRank {
	t.Helper()

	var out []stageRank
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) != 2 {
			continue
		}
		nameCell := strings.TrimSpace(cells[0])
		rankCell := strings.TrimSpace(cells[1])

		// Skip the markdown header and its separator rather than
		// failing on them: they are structure, not data.
		if nameCell == "Stage" || strings.HasPrefix(nameCell, "---") {
			continue
		}

		name, ok := firstBackticked(nameCell)
		if !ok {
			t.Fatalf("stage ladder row %q has no backticked stage name in its first cell", line)
		}
		name = strings.TrimPrefix(name, "internal/")
		if name == "" {
			name = "." // the scan root row, spelled "`internal/`" in the spine
		}

		rank, err := strconv.Atoi(rankCell)
		if err != nil {
			t.Fatalf("stage ladder row %q has a non-integer rank %q: %v", line, rankCell, err)
		}
		out = append(out, stageRank{Name: name, Rank: rank})
	}
	return out
}

// firstBackticked returns the contents of the first `…` span in s.
func firstBackticked(s string) (string, bool) {
	open := strings.Index(s, "`")
	if open < 0 {
		return "", false
	}
	rest := s[open+1:]
	closeAt := strings.Index(rest, "`")
	if closeAt < 0 {
		return "", false
	}
	return rest[:closeAt], true
}

// diffStageRanks returns the rows of a that do not appear anywhere in b,
// compared as whole {name, rank} pairs so that a rank change surfaces as
// one row missing and one row added rather than as silence.
func diffStageRanks(a, b []stageRank) []stageRank {
	var out []stageRank
	for _, x := range a {
		if !slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}
