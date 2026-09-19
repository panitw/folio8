package forbiddenimportsfixture

import . "math"

// violatingDotImport is Finding 15's own named hazard: a dot import
// makes Pow(10, 2) a bare, unqualified call — never an
// *ast.SelectorExpr — so RuleMathSelector's alias-resolved matching
// cannot see it at all. RuleDotImport (the engineering lead's WIDER
// ruling: ban dot imports outright, not just patch the math rule) must
// report this file regardless of which package is dot-imported.
func violatingDotImport() float64 {
	return Pow(10, 2)
}
