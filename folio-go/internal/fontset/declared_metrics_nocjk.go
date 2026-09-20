//go:build nocjkface

package fontset

// DeclaredLineMetrics answers for the one face this build declares but
// does not carry (spec-deferred-offline-cache, CAP-6).
//
// `-tags nocjkface` is the designer's engine wasm and nothing else. Its
// FontSet is short of "Noto Sans SC" until the browser fetches and
// installs it, and that absence must NOT change the vertical model: a
// chain naming the face must measure the same lines whether the glyphs
// have arrived or not, or the designer's PDF stops agreeing with
// production for documents containing no CJK at all.
//
// ⚠ METRICS ONLY, NEVER COVERAGE. This answers the leading arithmetic and
// nothing else: no glyph is drawn from it, no rune is reported covered by
// it, and resolveRuneFace never sees it. A CJK rune that must actually be
// PAINTED still refuses with TEXT_FACE_ABSENT, naming the face — which is
// exactly when the browser fetches it. On demand still means on demand.
func DeclaredLineMetrics(name string) (LineMetrics, bool) {
	if name == notoSansSCFaceName {
		return notoSansSCLineMetrics, true
	}
	return LineMetrics{}, false
}
