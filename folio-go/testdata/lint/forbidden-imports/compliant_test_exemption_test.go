package forbiddenimportsfixture

import (
	"embed"
	"os"
	"path/filepath"
	"testing"
)

//go:embed compliant_test_exemption_test.go
var compliantEmbedFS embed.FS

// compliantTestExemption uses exactly the four imports D-1.3.1's
// `_test.go` exemption permits — os, testing, path/filepath, embed —
// and must NOT be reported.
func compliantTestExemption(t *testing.T) {
	t.Helper()
	_ = os.Getenv("X")
	_ = filepath.Join("a", "b")
}
