package template

import (
	"os"
	"path/filepath"
	"testing"
)

// repoRootFromTest mirrors lint/internal/rules' helper of the same name
// (D-000.5/AD-21's shared pattern, duplicated because it lives only in
// _test.go files in two separate modules): it walks up from the test
// binary's cwd until it finds a directory containing both folio-go/ and
// lint/.
func repoRootFromTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if isRepoRootDir(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root walking up from %s", dir)
		}
		dir = parent
	}
}

func isRepoRootDir(dir string) bool {
	fg, err1 := os.Stat(filepath.Join(dir, "folio-go"))
	l, err2 := os.Stat(filepath.Join(dir, "lint"))
	return err1 == nil && fg.IsDir() && err2 == nil && l.IsDir()
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}
