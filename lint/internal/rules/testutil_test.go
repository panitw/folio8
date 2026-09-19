package rules

import (
	"os"
	"path/filepath"
	"testing"
)

// repoRootFromTest finds the folio8 repository root by walking up from
// the current working directory (Go test binaries run with cwd set to
// the package directory) until it finds a directory containing both
// folio-go/ and lint/ — the same D-000.5/AD-21 pattern folio-go's own
// repoRootFromTest helpers use, duplicated here because it lives only in
// a _test.go file and lint/ is a separate module.
func repoRootFromTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if isRepoRoot(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root (a directory containing both folio-go/ and lint/) walking up from %s", dir)
		}
		dir = parent
	}
}

func isRepoRoot(dir string) bool {
	folio8Go, err1 := os.Stat(filepath.Join(dir, "folio-go"))
	lintDir, err2 := os.Stat(filepath.Join(dir, "lint"))
	return err1 == nil && folio8Go.IsDir() && err2 == nil && lintDir.IsDir()
}

// assertExactFindings implements AC1's "by file and rule, never by
// count" fixture assertion (RP-3c): a scan that finds the right *number*
// of wrong things, but the wrong ones, must still fail. It compares
// distinct (path, rule) pairs, not raw finding counts, since a single
// file may legitimately trip a rule at more than one AST site.
func assertExactFindings(t *testing.T, got []Finding, want []Finding) {
	t.Helper()
	gotSet := map[[2]string]bool{}
	for _, f := range got {
		gotSet[[2]string{f.Path, f.Rule}] = true
	}
	wantSet := map[[2]string]bool{}
	for _, f := range want {
		wantSet[[2]string{f.Path, f.Rule}] = true
	}
	for k := range wantSet {
		if !gotSet[k] {
			t.Errorf("expected finding not reported: file=%s rule=%s", k[0], k[1])
		}
	}
	for k := range gotSet {
		if !wantSet[k] {
			t.Errorf("unexpected finding reported: file=%s rule=%s", k[0], k[1])
		}
	}
	if len(gotSet) != len(wantSet) {
		t.Errorf("distinct (file,rule) pair count mismatch: got %d, want %d", len(gotSet), len(wantSet))
	}
}

// findingSite is the (path, rule, line) key assertExactFindingSites
// compares on. It exists because (path, rule) alone — what
// assertExactFindings uses — is the right shape for a FIXTURE tree,
// where the question is "did the scanner flag the violating file and
// spare the compliant one", but the WRONG shape for an INVENTORY, where
// the question is "are these the only sanctioned sites in the tree".
//
// Story 2.3a Finding 2 (Major) measured the difference: with the
// inventory keyed by file, a second float-typed expression appended to
// an already-listed file was invisible, and so was removing one of two.
// That is a per-file allowlist wearing an inventory's name. Worse, the
// gap was not hypothetical — at the moment it was found,
// internal/fontset/vendorboundary_test.go already carried TWO sites
// (the `hhea` and `hmtx` HorizontalAdvance rows) and the file-keyed
// `want` list could only represent one of them, so a real fourth site
// was standing in the tree unlisted while the test reported ok.
type findingSite struct {
	Path string
	Rule string
	Line int
}

// assertExactFindingSites is assertExactFindings' site-exact sibling:
// it compares distinct (path, rule, LINE) triples, so every individual
// AST site must be enumerated. Adding a float-typed expression anywhere
// — including inside a file already listed — fails; removing one fails;
// moving one to a different line fails.
//
// The line numbers are load-bearing and WILL churn when unrelated edits
// shift a site. That friction is deliberate, not a defect: this list is
// the register of sanctioned AD-23 violations, and it should be
// impossible to change what is in it without a human reading the diff.
func assertExactFindingSites(t *testing.T, got []Finding, want []findingSite) {
	t.Helper()
	gotSet := map[findingSite]bool{}
	for _, f := range got {
		gotSet[findingSite{Path: f.Path, Rule: f.Rule, Line: f.Line}] = true
	}
	wantSet := map[findingSite]bool{}
	for _, s := range want {
		wantSet[s] = true
	}
	for k := range wantSet {
		if !gotSet[k] {
			t.Errorf("expected finding not reported: file=%s rule=%s line=%d", k.Path, k.Rule, k.Line)
		}
	}
	for k := range gotSet {
		if !wantSet[k] {
			t.Errorf("unexpected finding reported: file=%s rule=%s line=%d — a float-typed value expression exists at a site the inventory does not sanction", k.Path, k.Rule, k.Line)
		}
	}
	if len(gotSet) != len(wantSet) {
		t.Errorf("distinct (file,rule,line) site count mismatch: got %d, want %d", len(gotSet), len(wantSet))
	}
}
