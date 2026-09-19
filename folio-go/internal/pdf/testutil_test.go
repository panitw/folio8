package pdf

import (
	"os"
	"path/filepath"
	"testing"
)

// repoRootFromTest finds the folio8 repository root by walking up from the
// current working directory (Go test binaries run with cwd set to the
// package directory) until it finds a directory containing both
// folio-go/ and fixtures/. This is how tests reach repo-root fixtures/ by
// relative path without go:embed (D-000.5, AD-21) — a non-test helper
// importing os under internal/ would violate AD-1 and Story 1.3's lint, so
// this lives only in a _test.go file.
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
			t.Fatalf("could not find repo root (a directory containing both folio-go/ and fixtures/) walking up from %s", dir)
		}
		dir = parent
	}
}

func isRepoRoot(dir string) bool {
	folio8Go, err1 := os.Stat(filepath.Join(dir, "folio-go"))
	fixtures, err2 := os.Stat(filepath.Join(dir, "fixtures"))
	return err1 == nil && folio8Go.IsDir() && err2 == nil && fixtures.IsDir()
}
