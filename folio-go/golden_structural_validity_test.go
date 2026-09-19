package folio8_test

// EVERY COMMITTED PDF GOLDEN MUST BE A PDF.
//
// WHY THIS FILE EXISTS, and it is worth more than the one-character bug that
// produced it.
//
// Story 2.6 recorded fixtures/multi-page/expected.pdf. The file PASSED its
// golden hash and PASSED the story's whole semantic acceptance step — page
// count at two independent sites, per-page header and footer placement
// against hand-derived literals, a pinned line->page partition, no text at a
// negative Y, byte-identity across two processes. And it was NOT A PDF: it
// emitted
//
//	<< /Type /Pages /Kids [8 0 R10 0 R] /Count 2 >>
//
// with no separator between the two indirect references. A PDF tokenizer
// reads "R10" as one unknown token, so NEITHER kid resolves, the page tree is
// empty, qpdf reports "file does not contain any pages", and a viewer shows
// "page 0 of 2".
//
// EVERY ONE OF THOSE ACCEPTANCE CHECKS WAS SATISFIED BY THE BROKEN FILE, and
// the reason is worth stating exactly:
//
//   - "/Type /Page objects == 2" counted OBJECT DEFINITIONS. Both page
//     objects were emitted, correctly, and both were reachable in the xref.
//     What was broken was the /Kids ARRAY that points at them — a different
//     thing, in a different object.
//   - "/Count 2" read the integer. The integer was right. The integer is a
//     CLAIM about the kids, not a fact derived from them.
//   - Every per-page assertion parsed the CONTENT STREAMS directly, by
//     splitting the file on stream boundaries. Content streams do not care
//     whether anything references them, so header/footer placement, the line
//     partition and the negative-Y check all read exactly as they would in a
//     healthy file.
//   - Byte-identity across two processes says the output is DETERMINISTIC. A
//     deterministically wrong file is byte-identical to itself.
//
// The common shape: every check asserted a property of a PART, and the defect
// was in a REFERENCE BETWEEN parts. A hash certifies that bytes have not
// changed since they were recorded; it says nothing whatever about whether
// the bytes were right when they were recorded. D-000.22's semantic
// acceptance step is supposed to be what covers that gap — and here it did
// not, because it was assembled from checks that were all true of an
// unrenderable document.
//
// SO THE REMEDY IS NOT "one more assertion about multi-page". It is a
// STRUCTURAL VALIDITY ORACLE applied to EVERY committed golden, asserting the
// property no per-part check can see: THE DOCUMENT RESOLVES. This test is
// hermetic — it parses the page tree itself rather than shelling out — so it
// runs on every target and in every story's plain `go test ./...`, not only
// where qpdf happens to be installed.
//
// PROPOSED STANDING RULE, recorded here and raised in the story's Delivery
// Log rather than assumed: *no recorded PDF golden's hash may be trusted
// until the artifact has passed a structural validity oracle.* A recording is
// an act of acceptance (D-000.44), and accepting bytes nobody has proven are
// a document is the failure this file was written after.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/panitw/folio8/folio-go"
)

// TestEveryGoldenPDFResolvesItsPageTree is the structural oracle.
//
// It asserts, for every fixture declared in goldenDigestRecord:
//
//  1. The page tree exists and declares a /Count.
//  2. The /Kids array splits into exactly /Count references, each one
//     WELL-FORMED — this is the assertion that would have caught the bug.
//     "[8 0 R10 0 R]" splits into ONE token that matches no reference at all.
//  3. Every referenced object number is actually DEFINED in the file as
//     "N 0 obj" and carries /Type /Page, and the referenced numbers are
//     pairwise DISTINCT. A syntactically valid reference to a non-existent
//     object is still an empty page tree, and a duplicated reference is how
//     an orphaned page hides behind a correct-looking reference count.
//  4. The SET of referenced page-object numbers equals the SET of /Type
//     /Page definitions in the file — so a page object that exists but is
//     unreachable, which is the general shape of this defect, fails here
//     too. CORRECTED by the finisher (Finding 4): this used to compare
//     COUNTS, not SETS, and a probe — "[8 0 R 8 0 R]" with objects 8 and 10
//     both defined as /Type /Page — passed: len(refs) == declaredCount ==
//     2, both references resolve, and the defined-object COUNT equals 2,
//     while object 10 sits outside the referenced set entirely and page 2
//     is a silent duplicate of page 1.
func TestEveryGoldenPDFResolvesItsPageTree(t *testing.T) {
	root := repoRootForByteNeutrality(t)

	if len(goldenDigestRecord) == 0 {
		t.Fatal("vacuity guard: the golden declaration is empty, so this test checks nothing")
	}

	checked := 0
	for _, fx := range goldenDigestRecord {
		t.Run(fx.dir, func(t *testing.T) {
			path := filepath.Join(root, "fixtures", fx.dir, "expected.pdf")
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("presence precondition: %s could not be read: %v", path, err)
			}
			if len(b) == 0 {
				t.Fatal("presence precondition: the golden is empty")
			}
			if !folio8.AssertPDFPageTreeResolves(t, b, path) {
				return
			}
			checked++
		})
	}

	if checked == 0 {
		t.Error("vacuity guard: no golden was actually checked")
	}
	t.Logf("structural validity: %d committed PDF goldens parsed; every page tree resolves", checked)
}
