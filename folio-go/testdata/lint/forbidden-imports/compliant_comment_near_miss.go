package forbiddenimportsfixture

// compliantCommentNearMiss reproduces internal/geom/scale.go:31's exact
// shape (F-6, measured at f9c27b3): the only math.Round anywhere under
// internal/ is inside a comment, alongside five math.MinInt64 comment
// mentions in the same real file. A regex over source text would flag
// every one of these; an AST walk (this guard) must not, because
// comments are never part of the AST's expressions (AC12, AC13, RP-5d).
//
// no call to math.Round in this function. math.MinInt64 is mentioned
// here, and here (math.MinInt64), and again (math.MinInt64), a fourth
// time (math.MinInt64), and a fifth (math.MinInt64) — all in comments.
func compliantCommentNearMiss(x int64) int64 {
	return x
}
