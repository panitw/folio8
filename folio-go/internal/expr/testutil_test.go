package expr

// repoRootFromTest and mustReadFile mirror internal/template's helpers
// of the same name (D-000.5/AD-21's shared pattern, duplicated because
// it lives only in _test.go files, package-local by Go's own rules).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/template"
)

// testFC is the ordinary `en`/UTC formatting context most tests that
// do not themselves exercise formatDate/formatNumber's locale
// behaviour thread through Eval — Story 3.4's FormatContext parameter
// (R1) is mandatory on every Eval call, and this is the shared,
// explicit, non-zero value tests use when the locale is not the point
// of the test (AC5b: the ZERO value is deliberately unusable, so it is
// never used here as a stand-in for "don't care").
func testFC() FormatContext {
	return FormatContext{Locale: template.LocaleEN, UTCOffset: "+00:00"}
}

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
