package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFindRepoRoot is Finding 17's minimum fix (this story's QA review):
// findRepoRoot is a third, near-identical copy of the "walk up until a
// directory holds both folio-go/ and lint/" pattern also duplicated in
// lint/internal/rules/testutil_test.go and
// lint/internal/manifest/manifest_test.go — but unlike those two, this
// copy is production code with no test file at all, and it is the one
// that decides where genmanifest writes the committed MANIFEST.md. This
// does not consolidate the three copies (that would be a larger,
// out-of-scope refactor); it closes the specific gap the finding named:
// zero coverage on the one copy that writes a committed file.
func TestFindRepoRoot(t *testing.T) {
	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("findRepoRoot: %v", err)
	}

	folio8Go, err1 := os.Stat(filepath.Join(root, "folio-go"))
	lintDir, err2 := os.Stat(filepath.Join(root, "lint"))
	if err1 != nil || !folio8Go.IsDir() {
		t.Errorf("resolved root %q does not contain a folio-go/ directory", root)
	}
	if err2 != nil || !lintDir.IsDir() {
		t.Errorf("resolved root %q does not contain a lint/ directory", root)
	}

	// The path genmanifest actually writes to must exist under this
	// root and be exactly lint/MANIFEST.md — the concern Finding 17
	// raised: a drift between this resolution and the test package's
	// own repoRootFromTest would surface as a confusing "out of date"
	// failure rather than a clear root-mismatch one.
	wantManifest := filepath.Join(root, "lint", "MANIFEST.md")
	if _, err := os.Stat(wantManifest); err != nil {
		t.Errorf("expected %s to exist: %v", wantManifest, err)
	}
}
