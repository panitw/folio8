package rules

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestNoCompressorProductionScan is AC10's production caller: the real
// folio-go/ tree (not merely internal/ — AC10 says "no file under
// folio-go/") must show zero findings. Non-vacuous per D-000.9: the
// scanner's own reported FilesSeen must be non-zero.
func TestNoCompressorProductionScan(t *testing.T) {
	root := repoRootFromTest(t)
	folio8GoDir := filepath.Join(root, "folio-go")

	findings, stats, err := ScanNoCompressorImports(folio8GoDir)
	if err != nil {
		t.Fatalf("scan folio-go/: %v", err)
	}
	if stats.FilesSeen == 0 {
		t.Fatal("vacuity guard: scanner's own stats report 0 files seen under folio-go/ (D-000.9)")
	}
	if len(findings) > 0 {
		var msgs []string
		for _, f := range findings {
			msgs = append(msgs, f.Message)
		}
		t.Fatalf("forbidden compressor/image-decoder imports found under folio-go/ (AC10, D-1.8.1):\n%s", strings.Join(msgs, "\n"))
	}
}

// TestNoCompressorRedProof is RP-5: a real compress/zlib import, used,
// in a real non-test file must redden this guard — question 2 of AC30
// (D-000.9 extended) matters here because this guard is green at
// baseline for the "vacuous" reason that nothing imports a compressor
// yet (M-8). A control (zero-content violating file omitted here since
// the fixture is itself the mutation, per the forbidden-imports guard's
// established shape) is the production scan above staying green before
// this fixture is added to it — it isn't, since this fixture lives
// under testdata/, which walkGoFiles always excludes from any OTHER
// scan by directory name, so the production scan can never accidentally
// pick it up.
func TestNoCompressorRedProof(t *testing.T) {
	root := repoRootFromTest(t)

	t.Run("compressor import reddens with RuleNoCompressor", func(t *testing.T) {
		dir := filepath.Join(root, "folio-go", "testdata", "lint", "no-compressor", "violating-compressor")
		findings, stats, err := ScanNoCompressorImports(dir)
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		if stats.FilesSeen == 0 {
			t.Fatal("vacuity guard: 0 files seen in the violating fixture directory")
		}
		found := false
		for _, f := range findings {
			if f.Rule == RuleNoCompressor {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected a %s finding, got: %+v", RuleNoCompressor, findings)
		}
	})

	t.Run("image decoder import reddens with RuleNoImageDecoder", func(t *testing.T) {
		dir := filepath.Join(root, "folio-go", "testdata", "lint", "no-compressor", "violating-decoder")
		findings, stats, err := ScanNoCompressorImports(dir)
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		if stats.FilesSeen == 0 {
			t.Fatal("vacuity guard: 0 files seen in the violating fixture directory")
		}
		found := false
		for _, f := range findings {
			if f.Rule == RuleNoImageDecoder {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected a %s finding, got: %+v", RuleNoImageDecoder, findings)
		}
	})
}
