//go:build nocjkface

package folio8

import "testing"

// TestTenFaceRenderIsByteIdenticalUnderTheTag is the property the whole of
// CAP-6 rests on, asserted in the build that actually ships to the
// designer.
//
// Every document in the corpus names the CJK face in its chain and reaches
// none of it. Rendered by an engine holding ten faces and the absent
// face's three hhea integers, each must produce EXACTLY the bytes an
// eleven-face engine produces — which is what the CLI, folio-js and
// folio-dotnet produce. Not "close", not "same page count": the same PDF.
//
// ⚠ ITS NON-VACUITY LIVES IN THE OTHER BUILD. Ten faces and eleven faces
// agreeing here would be unremarkable if they agreed everywhere, so
// declared_metrics_divergence_test.go asserts — untagged, where no metrics
// are declared — that these very documents DO diverge. Read the two
// together; neither is worth much alone.
// ⚠ HOW TO RUN IT, AND WHY IT IS NOT `go test -tags nocjkface ./`. Most of
// this package's suite is ABOUT the eleven-face shipped set — the golden
// instances, the PostScript-name census, the starter's declared cuts, the
// Pan-CJK coverage rows — and every one of them correctly fails under a tag
// whose whole purpose is that `fonts.Shipped()` returns ten. The tagged build
// is the designer's wasm, not a second configuration of the library, so the
// tag is run over the packages that have something to say about it:
//
//	go test -tags nocjkface ./fonts/ ./internal/fontset/ ./internal/wasm/
//	go test -tags nocjkface ./ -run 'TestTenFaceRender|TestCJKTextStillRefuses'
//
// The second line is this file and its twin. Stated here rather than left to
// a reader to discover from a wall of unrelated red.
func TestTenFaceRenderIsByteIdenticalUnderTheTag(t *testing.T) {
	outcome := compareTenAgainstEleven(t)
	identical, different := outcome.identical, outcome.different
	if len(different) > 0 {
		t.Fatalf(
			"%d of %d corpus documents rendered DIFFERENTLY with ten faces plus declared line metrics than with "+
				"eleven: %v.\nNone of them contains a CJK codepoint, so this is the designer disagreeing with the CLI, "+
				"folio-js and folio-dotnet about ordinary Latin and Thai documents — the byte-identity this spec says "+
				"survives untouched.\nIf the difference is the LEADING, the declared metrics in "+
				"internal/fontset/declared_metrics.go are wrong or are not reaching chainLineMetrics. If it is something "+
				"else, a missing chain member makes a SECOND difference nobody has accounted for: report it rather than "+
				"widening this test.\nidentical: %v",
			len(different), len(different)+len(identical), different, identical,
		)
	}
	if len(identical) == 0 {
		t.Fatal("the corpus is empty, so this test asserted nothing")
	}
	t.Logf("byte-identical with ten faces + declared metrics: %d documents (%v)", len(identical), identical)
}
