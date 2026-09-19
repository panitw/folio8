package rules

import (
	"path/filepath"
	"testing"
)

// TestEmbedFontProductionScan is AC3's production caller: scans the
// real folio-go/internal/ tree and asserts zero findings — no package
// under internal/ embeds font data (AD-8's Rule). Coverage witness
// first (D-000.9): zero files parsed is a failure distinct from "zero
// findings, healthy".
func TestEmbedFontProductionScan(t *testing.T) {
	root := repoRootFromTest(t)
	internalDir := filepath.Join(root, "folio-go", "internal")

	findings, stats, err := ScanEmbedFont(internalDir)
	if err != nil {
		t.Fatalf("scan folio-go/internal/: %v", err)
	}
	if stats.FilesParsed == 0 {
		t.Fatal("ScanEmbedFont parsed zero files under folio-go/internal/ — coverage witness failed (D-000.9): a scanner that looked at nothing must not report the same 'zero findings' as a healthy run")
	}
	if len(findings) > 0 {
		var msgs []string
		for _, f := range findings {
			msgs = append(msgs, f.Message)
		}
		t.Fatalf("go:embed directive(s) naming a font file found under folio-go/internal/ (AD-8, AC3):\n%v", msgs)
	}
}

// TestEmbedFontFixtureScan is AC3's fixture caller: the retained
// fixture tree at folio-go/testdata/lint/embed-font/ (never under
// folio-go/internal/) must report exactly the named finding, by file
// and rule.
func TestEmbedFontFixtureScan(t *testing.T) {
	root := repoRootFromTest(t)
	fixtureRoot := filepath.Join(root, "folio-go", "testdata", "lint", "embed-font")

	got, stats, err := ScanEmbedFont(fixtureRoot)
	if err != nil {
		t.Fatalf("scan fixture tree %s: %v", fixtureRoot, err)
	}
	if stats.FilesParsed == 0 {
		t.Fatal("vacuity guard: ScanEmbedFont parsed zero files under the fixture tree")
	}

	want := []Finding{
		{Path: "violating.go", Rule: RuleEmbedFont},
		// Finding 5 (QA review): the four evading shapes the shipped
		// guard could not see — a bare directory argument, a glob, the
		// "all:" prefix, and a quoted argument with an internal space.
		{Path: filepath.Join("dirembed", "dirembed.go"), Rule: RuleEmbedFont},
		{Path: filepath.Join("globembed", "globembed.go"), Rule: RuleEmbedFont},
		{Path: filepath.Join("allembed", "allembed.go"), Rule: RuleEmbedFont},
		{Path: "quoted.go", Rule: RuleEmbedFont},
	}
	gotSet := map[[2]string]bool{}
	for _, f := range got {
		gotSet[[2]string{f.Path, f.Rule}] = true
	}
	wantSet := map[[2]string]bool{}
	for _, f := range want {
		wantSet[[2]string{f.Path, f.Rule}] = true
	}
	for k := range wantSet {
		if !gotSet[k] {
			t.Errorf("expected finding not reported: file=%s rule=%s", k[0], k[1])
		}
	}
	for k := range gotSet {
		if !wantSet[k] {
			t.Errorf("unexpected finding reported: file=%s rule=%s", k[0], k[1])
		}
	}
}
