package fontset

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDeclaredLineMetricsMatchTheRealFace is the tie the declared numbers
// are worth nothing without.
//
// WHAT IT ASSERTS AND WHY IT ASSERTS IT HERE. The designer's engine
// carries three integers in place of a 10.6 MB face so that a chain
// naming that face measures the same lines with or without it. Those
// integers are only correct while they equal what the face itself says —
// one unit of drift moves every line of every CJK-tailed document by a
// fraction and breaks byte-identity with the CLI, silently, for documents
// containing no CJK at all.
//
// ⚠ IT RUNS IN THE UNTAGGED BUILD, WHICH IS THE ONLY BUILD THAT CAN RUN
// IT. Under `-tags nocjkface` the face is not embedded anywhere, so there
// is nothing to compare against; untagged it is read fresh from the
// committed binary — the same file folio-go/fonts/accounting_test.go
// independently joins to fonts.Shipped() BY BYTES, which is what makes
// "the committed binary" and "the face the module ships" the same thing
// without this package importing `fonts` (it cannot: fonts imports
// folio8, which imports this).
func TestDeclaredLineMetricsMatchTheRealFace(t *testing.T) {
	path := filepath.Join("..", "..", "fonts", "notosanssc", "NotoSansSC-Regular.ttf")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v — it is the face the declared metrics claim to describe", path, err)
	}
	// NON-VACUITY BEFORE COMPARISON: a truncated or failed read that still
	// produced a Font would compare near-nothing to near-nothing.
	if len(raw) < 10_000_000 {
		t.Fatalf("%s read as %d bytes, far under the face's ~10.6 MB", path, len(raw))
	}
	face, err := New(notoSansSCFaceName, raw)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	actual := face.LineMetrics()
	if actual != notoSansSCLineMetrics {
		t.Fatalf(
			"the declared line metrics for %q are %+v and the committed face's own are %+v.\n"+
				"These three integers are what lets the designer's engine leave the face's 10.6 MB behind and still lay "+
				"out a CJK-tailed chain exactly as production does. One unit of drift is a different PDF for documents "+
				"with no CJK in them at all. Correct the declaration in declared_metrics.go to the face's own values — "+
				"never the other way round.",
			notoSansSCFaceName, notoSansSCLineMetrics, actual,
		)
	}
	// AND THE VALUES ARE NOT A ZERO VALUE WEARING THE COSTUME OF A MATCH.
	// Two zero LineMetrics compare equal, so a constructor that silently
	// produced nothing would satisfy the comparison above.
	if notoSansSCLineMetrics.Ascent <= 0 || notoSansSCLineMetrics.Descent >= 0 {
		t.Fatalf("the declared metrics %+v are not a plausible hhea reading: Ascent must be positive and Descent negative", notoSansSCLineMetrics)
	}
}

// TestDeclaredLineMetricsAnswerForNothingElse: the lookup is keyed by one
// exact name and derives nothing from it (D-B). A prefix or suffix match
// would silently supply CJK leading to a face that merely reads like it.
func TestDeclaredLineMetricsAnswerForNothingElse(t *testing.T) {
	for _, name := range []string{"", "Noto Sans", "Noto Sans Thai", "Noto Sans SC Bold", "noto sans sc", "Roboto"} {
		if _, ok := DeclaredLineMetrics(name); ok {
			t.Errorf("DeclaredLineMetrics(%q) answered; only the exact declared face name may", name)
		}
	}
}
