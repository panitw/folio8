package folio8

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestNoRealToUnicodeSectionExceedsTheCap is DW-14's corpus-wide
// property, added on the engineering lead's ruling. The chunking fix
// (internal/pdf.buildToUnicodeCMap) is inert against every fixture
// shipped today — D1 measured the largest /ToUnicode section anywhere
// in the repository at 45 entries, on the two-page `multi-page`
// fixture, and `page-count-20` carries only 32 (fewer than a two-page
// document: distinct content drives section size, not page count) —
// so at cap 100 the chunking branch is never taken by anything this
// test can see TODAY.
//
// That inertness is not merely "a change with no witness" — it is
// scheduled to end at Story 4.7, the five-column transaction table at
// 1/5/20/50 pages carrying Latin, Thai and CJK, which is very likely
// the first input to cross 100 entries AND the commit that freezes the
// golden hashes across four targets. If the chunking were subtly
// wrong, 4.7 would record the wrong bytes as permanently correct
// forever, and every guard this programme owns would then defend them.
//
// This test is what makes that landing automatic rather than something
// a human has to remember: it reads every COMMITTED GOLDEN PDF this
// repository ships (fixtures/*/expected.pdf — the real corpus, not a
// synthetic stand-in) and asserts no /ToUnicode beginbfchar section
// exceeds the cap. It is green today at max 45 with zero code
// required, and becomes load-bearing automatically the instant a
// future fixture (4.7's, or any other) crosses 100.
//
// Anti-vacuity (D-000.9): it reports, via t.Logf, the OBSERVED MAXIMUM
// section size and the number of fixtures/sections actually examined —
// never merely "no violation" — so the jump from 45 to something over
// 100 is a number a human sees at the recording story, not a silent
// pass/fail flip that could also mean "found nothing to look at".
//
// Red-proof (D-000.9/D-000.75, reviewer's M4 reproduced as a permanent
// test rather than a one-off manual probe): pointing walkToUnicodeCorpus
// — the SAME walk this test calls, not a stand-in helper — at a
// t.TempDir() carrying one hand-built 101-entry fixture reddens it. See
// TestNoRealToUnicodeSectionExceedsTheCapRedProof, below.
func TestNoRealToUnicodeSectionExceedsTheCap(t *testing.T) {
	root := repoRootFromTest(t)
	fixturesDir := filepath.Join(root, "fixtures")
	result, err := walkToUnicodeCorpus(fixturesDir)
	if err != nil {
		t.Fatalf("walkToUnicodeCorpus(%s): %v", fixturesDir, err)
	}
	for _, v := range result.violations {
		t.Errorf("fixture %q: a /ToUnicode section carries %d entries, over the %d cap", v.fixture, v.size, bfcharCapForTest)
	}
	if result.fixturesExamined == 0 {
		t.Fatal("presence precondition: found zero fixtures/*/expected.pdf files — this test examined nothing")
	}
	if result.fixturesWithSections == 0 {
		t.Fatal("presence precondition: found zero /ToUnicode sections across the whole corpus — this test examined nothing")
	}
	t.Logf("DW-14 real-corpus witness: examined %d fixture(s), %d carrying at least one /ToUnicode section, %d section(s) total; observed maximum section size = %d (cap %d)",
		result.fixturesExamined, result.fixturesWithSections, result.sectionsExamined, result.observedMax, bfcharCapForTest)
}

// TestNoRealToUnicodeSectionExceedsTheCapRedProof is the mandatory
// red-proof (D-000.9/D-000.75: the party who lands a safety property
// owes the mutation) for TestNoRealToUnicodeSectionExceedsTheCap
// itself: it points walkToUnicodeCorpus — the identical function that
// test calls over the real fixtures/ directory — at a t.TempDir()
// containing exactly one hand-built expected.pdf whose beginbfchar
// section carries 101 entries, one over the cap, and asserts the walk
// REPORTS A VIOLATION. This reddens the real corpus test's own logic,
// not merely the size-extraction helper it happens to share with it
// (reproducing the reviewer's manual M4 probe as a standing test).
func TestNoRealToUnicodeSectionExceedsTheCapRedProof(t *testing.T) {
	dir := t.TempDir()
	fixtureDir := filepath.Join(dir, "zz-redproof-fixture")
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	var b strings.Builder
	b.WriteString("101 beginbfchar\n")
	for i := 0; i < 101; i++ {
		b.WriteString("<0000> <0041>\n")
	}
	b.WriteString("endbfchar\n")
	if err := os.WriteFile(filepath.Join(fixtureDir, "expected.pdf"), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := walkToUnicodeCorpus(dir)
	if err != nil {
		t.Fatalf("walkToUnicodeCorpus(%s): %v", dir, err)
	}
	if result.fixturesExamined != 1 || result.fixturesWithSections != 1 {
		t.Fatalf("presence precondition: examined %d fixture(s), %d with sections, want 1 and 1", result.fixturesExamined, result.fixturesWithSections)
	}
	if len(result.violations) != 1 {
		t.Fatalf("walkToUnicodeCorpus did not report a violation for a 101-entry section: violations = %v", result.violations)
	}
	if result.violations[0].size != 101 {
		t.Errorf("reported violation size = %d, want 101", result.violations[0].size)
	}
	t.Logf("red-proof: walkToUnicodeCorpus correctly reported a violation (%d entries, cap %d) for a hand-built over-cap fixture — TestNoRealToUnicodeSectionExceedsTheCap would fail under this same input", result.violations[0].size, bfcharCapForTest)
}

// toUnicodeViolation is one fixture's over-cap /ToUnicode section, as
// reported by walkToUnicodeCorpus.
type toUnicodeViolation struct {
	fixture string
	size    int
}

// toUnicodeCorpusResult is walkToUnicodeCorpus's full report — every
// figure TestNoRealToUnicodeSectionExceedsTheCap's own anti-vacuity
// logging needs, plus the violations (if any) it turns into t.Errorf
// calls.
type toUnicodeCorpusResult struct {
	violations           []toUnicodeViolation
	observedMax          int
	sectionsExamined     int
	fixturesWithSections int
	fixturesExamined     int
}

// walkToUnicodeCorpus reads every <fixturesDir>/*/expected.pdf and
// measures its /ToUnicode section sizes. It is the ONE walk both
// TestNoRealToUnicodeSectionExceedsTheCap (over the real fixtures/
// directory) and its red-proof (over a t.TempDir() carrying one
// hand-built over-cap fixture) call — Story 4.2 review Finding 7: a
// red-proof that only exercised toUnicodeSectionSizes on a hand-built
// string never demonstrated that THIS walk's own violation-detection
// works, so the walk itself is now the shared, parameterised subject.
func walkToUnicodeCorpus(fixturesDir string) (toUnicodeCorpusResult, error) {
	var result toUnicodeCorpusResult
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		return result, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pdfPath := filepath.Join(fixturesDir, e.Name(), "expected.pdf")
		raw, rerr := os.ReadFile(pdfPath)
		if rerr != nil {
			continue // this fixture ships no expected.pdf (e.g. a round-trip-only fixture) — nothing to examine
		}
		result.fixturesExamined++
		sizes := toUnicodeSectionSizes(raw)
		if len(sizes) > 0 {
			result.fixturesWithSections++
		}
		for _, sz := range sizes {
			result.sectionsExamined++
			if sz > bfcharCapForTest {
				result.violations = append(result.violations, toUnicodeViolation{fixture: e.Name(), size: sz})
			}
			if sz > result.observedMax {
				result.observedMax = sz
			}
		}
	}
	return result, nil
}

// bfcharCapForTest is this test file's OWN literal for the ToUnicode
// section cap (D-000.68/D-000.9): it does not read
// internal/pdf.bfcharSectionCap, so a wrong value in production cannot
// make this test agree with itself — the two are independent
// statements of the same number.
const bfcharCapForTest = 100

var toUnicodeSectionRE = regexp.MustCompile(`(?s)(\d+)\s+beginbfchar\n(.*?)endbfchar`)

// toUnicodeSectionSizes extracts each beginbfchar...endbfchar section's
// ACTUAL entry count (by counting "<...> <...>\n" lines), independent
// of what the section's own declared count says — the same measured
// approach D1's own corpus table used (each entry line ends "\n").
func toUnicodeSectionSizes(raw []byte) []int {
	ms := toUnicodeSectionRE.FindAllSubmatch(raw, -1)
	sizes := make([]int, 0, len(ms))
	for _, m := range ms {
		sizes = append(sizes, strings.Count(string(m[2]), "\n"))
	}
	return sizes
}
