package folio8

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestMatrixDocumentSlugsAreRegisteredInCI closes a drift that has
// already happened once.
//
// matrix_test.go carries `//go:build matrix`, so its matrixDocuments
// list executes only at an epic boundary gate. .github/workflows/
// matrix.yml carries a SECOND, hand-written copy of the same list — the
// `docs="..."` line and the per-target upload-artifact paths. Nothing
// connected the two, so a document registered in one and not the other
// was invisible: Story 2.2 added multi-script-fallback to matrixDocuments
// and never to the workflow, and CI went on publishing four documents'
// worth of legs while comparing three, reporting green. That is Story
// 1.2's own Finding 8 recurring one story later, which is what a
// hand-maintained duplicate list does.
//
// This test is deliberately UNTAGGED — it runs in the ordinary
// `go test ./...` every story runs, not at the gate — and it reads both
// lists from SOURCE, because the tagged list is not compiled into this
// binary. Reading source rather than the compiled value is the weaker
// mechanism; running in every story is what makes it worth having.
//
// The Go side is read by go/parser (MatrixDocumentSlugsFromSource), not
// by pattern-matching text: a regex over the source was measured to be
// defeated by a single-line composite literal, and defeated the sibling
// obligation guard in the same stroke. The workflow side stays a regex
// because matrix.yml is YAML holding a shell line — there is no grammar
// here to parse, and the `docs="…"` assignment has one spelling that a
// missing match reports as a failure rather than as an empty list.
func TestMatrixDocumentSlugsAreRegisteredInCI(t *testing.T) {
	root := repoRootFromTest(t)

	declared, elements, err := MatrixDocumentSlugsFromSource(filepath.Join(root, "folio-go", "matrix_test.go"))
	if err != nil {
		t.Fatalf("read matrixDocuments from matrix_test.go: %v", err)
	}
	if len(declared) == 0 || elements == 0 {
		t.Fatal("vacuity guard: found no matrixDocuments entries in matrix_test.go — this test would pass on an empty comparison")
	}
	// N-of-N witness: every literal element yielded a slug. This guard and
	// TestEpic2GateObligationsMatchTheDeclaredSet now share ONE reader,
	// and that is deliberate — they previously shared a REGEX, which was
	// worse: it made them one guard wearing two names, and a single-line
	// composite literal (gofmt-clean, compiling under -tags matrix) was
	// measured invisible to both at once (Story 2.5 review, Finding 1).
	// The shared reader parses the Go grammar, where the literal has no
	// second spelling, and FAILS on an entry it cannot read rather than
	// letting the set shrink underneath the comparison (D-000.36).
	if len(declared) != elements {
		t.Fatalf("vacuity guard: read %d slugs from %d matrixDocuments entries — a registered document is missing from the comparison", len(declared), elements)
	}

	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "matrix.yml"))
	if err != nil {
		t.Fatalf("read matrix.yml: %v", err)
	}
	docsRe := regexp.MustCompile(`(?m)^\s*docs="([^"]+)"`)
	dm := docsRe.FindStringSubmatch(string(workflow))
	if dm == nil {
		t.Fatal("could not find the docs=\"...\" list in .github/workflows/matrix.yml")
	}
	inCI := strings.Fields(dm[1])
	if len(inCI) == 0 {
		t.Fatal("vacuity guard: the workflow's docs list is empty")
	}

	if !equalSets(declared, inCI) {
		t.Fatalf(
			"matrixDocuments and .github/workflows/matrix.yml's docs list disagree.\n"+
				"  matrix_test.go: %v\n  matrix.yml:     %v\n"+
				"A document in one list and not the other is a matrix leg nobody compares, reported as green.",
			sorted(declared), sorted(inCI),
		)
	}

	// ...and every document must have an upload-artifact path on every
	// target, or its hashes never reach the comparison job at all.
	targets := []string{"darwin-arm64", "linux-amd64", "linux-arm64", "js-wasm"}
	for _, slug := range declared {
		for _, target := range targets {
			want := fmt.Sprintf("hash.%s.%s.txt", target, slug)
			if !strings.Contains(string(workflow), want) {
				t.Errorf("matrix.yml declares no upload-artifact path %s — that leg's hash never reaches the comparison job", want)
			}
		}
	}

	t.Logf("matrix registration witness — %d documents read from %d matrixDocuments literal entries, registered in both matrixDocuments and matrix.yml, across %d targets", len(declared), elements, len(targets))
}

func equalSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x, y := sorted(a), sorted(b)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
