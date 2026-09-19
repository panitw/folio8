package folio8

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestREADMEExampleBlockMatchesSource makes the README's "reproduced here
// verbatim" claim TRUE BY CHECK rather than by promise.
//
// It was not. The dead `err == nil &&` conjunct was genuinely fixed in
// example_test.go — with a comment explaining that log.Fatal above makes
// it unreachable — and README.md kept shipping the old line, under a
// sentence asserting the block was verbatim. So the artifact AC9 actually
// names, the one an integrator reads, carried the defect the story
// reported as fixed, plus a now-false guarantee about itself. Nothing
// diffed the two, so it would have drifted again.
//
// This is D-1.4.10's fence/golden byte-identity mechanism, applied to the
// one fenced block in this repo that claims to be a copy of a source file.
func TestREADMEExampleBlockMatchesSource(t *testing.T) {
	root := repoRootFromTest(t)

	readmePath := filepath.Join(root, "folio-go", "README.md")
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read %s: %v", readmePath, err)
	}
	sourcePath := filepath.Join(root, "folio-go", "example_test.go")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read %s: %v", sourcePath, err)
	}
	// Assert both operands are non-empty BEFORE comparing: two empty
	// reads compare equal, and would report agreement between two files
	// neither of which was examined.
	if len(readme) == 0 || len(source) == 0 {
		t.Fatalf("refusing to compare: README %d bytes, example_test.go %d bytes", len(readme), len(source))
	}

	const fence = "```go\n"
	start := strings.Index(string(readme), fence)
	if start < 0 {
		t.Fatalf("%s contains no ```go fenced block at all — the verbatim claim has nothing to point at", readmePath)
	}
	rest := string(readme)[start+len(fence):]
	end := strings.Index(rest, "\n```")
	if end < 0 {
		t.Fatalf("%s's ```go block is never closed", readmePath)
	}
	block := rest[:end]

	want := strings.TrimRight(string(source), "\n")
	if block != want {
		t.Fatalf(
			"README.md's fenced Go block has DRIFTED from example_test.go, while README.md still claims "+
				"the block is reproduced verbatim.\n\n"+
				"The README is the artifact an integrator actually reads (AC9, DW-9), so a stale copy there "+
				"is the defect, not a cosmetic mismatch. Re-copy example_test.go into the fence.\n\n"+
				"--- README block (%d bytes) ---\n%s\n\n--- example_test.go (%d bytes) ---\n%s",
			len(block), block, len(want), want,
		)
	}
}
