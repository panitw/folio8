// Package forbiddenimportsfixture is a retained fixture tree for the
// forbidden-imports rule (D-1.3.1, D-1.3.10, AC13). Never compiled —
// testdata/ is invisible to the go command — and syntactically valid Go
// with forbidden semantics (AC5).
package forbiddenimportsfixture

import "time"

func violatingTimeImport() time.Time {
	return time.Time{}
}
