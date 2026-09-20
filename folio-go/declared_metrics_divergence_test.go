//go:build !nocjkface

package folio8

import "testing"

// TestTenFaceRenderDivergesWithoutTheDeclaredMetrics is the non-vacuity
// half of the pair, and it is two guards in one.
//
//  1. IT IS THE MEASUREMENT OF THE DEFECT. Without the declared metrics a
//     ten-face engine lays every CJK-tailed document out against a shorter
//     chain, and the corpus below contains no CJK text whatsoever. If this
//     test ever goes green — if the renders start matching without the
//     metrics — then the tagged half is proving nothing, because the thing
//     it corrects would no longer be happening.
//
//  2. IT IS THE GUARD ON EVERY OTHER CONSUMER. folio-js, folio-dotnet, the
//     CLI, the goldens and any caller with a genuinely partial FontSet get
//     THIS build, and their behaviour must be exactly what it always was:
//     a chain member that was not supplied contributes nothing. A metrics
//     table that answered unconditionally would change their output, and
//     it would red here first.
func TestTenFaceRenderDivergesWithoutTheDeclaredMetrics(t *testing.T) {
	outcome := compareTenAgainstEleven(t)
	identical, different := outcome.identical, outcome.different
	if len(different) == 0 {
		t.Fatalf(
			"every corpus document rendered IDENTICALLY with ten faces and with eleven, in the build that declares NO "+
				"line metrics for the missing face.\nThat is the premise the tagged parity test rests on, and it has "+
				"stopped being true: either a chain member no longer constrains the vertical model (in which case the "+
				"declared metrics are dead code and should go), or this corpus no longer contains a document whose chain "+
				"names the CJK face.\nidentical: %v",
			identical,
		)
	}
	// AND THE DIVERGENCE IS NOT MERELY "SOMETHING SOMEWHERE": the document
	// that was actually measured and reported must be one of them, or the
	// pair has drifted off the case it was written for.
	measured := false
	for _, name := range different {
		if name == "inline/two-line-latin-paragraph" {
			measured = true
		}
	}
	if !measured {
		t.Fatalf("the measured two-line Latin paragraph no longer diverges; the corpus has drifted off the reported case. different: %v", different)
	}
	t.Logf("without declared metrics: %d of %d corpus documents render differently (%v); identical: %v", len(different), len(different)+len(identical), different, identical)
}
