// Package numericformattingfixture is a retained violating fixture for
// the numeric-formatting rule (D-1.3.2, D-1.3.3): a non-numbers.go file
// inside pdf/, the tree standing in for internal/pdf (Blocker 4, this
// story's QA review: the fixture root itself now IS the scope claim
// under test — see ../template/version.go's doc comment for the other
// half). Never compiled — testdata/ is invisible to the go command
// (F-4) — and syntactically valid Go with forbidden semantics (AC5).
package numericformattingfixture

import "strconv"

func violating(n int) string {
	return strconv.Itoa(n)
}
