package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file is Story 3.4's AC2: "lint's forbidden-imports scan over
// folio-go/internal/ reports FilesSeen INCLUDING EVERY NEW FILE this
// story adds — asserted by name, not by a count that could pass while
// the new file was skipped — and finds zero findings." It is a
// coverage proof over the EXISTING guard, not a new fence (D-000.24):
// no new rule is added here.

// story34NewFiles is every file Story 3.4 added under folio-go/internal/
// (AD-12's locale table, formatDate, formatNumber and their supporting
// grammars), by path RELATIVE TO folio-go/internal/ — repaired against
// Finding 14 (this story's QA review, three gaps):
//  1. The original list only named internal/expr's 8 non-test files;
//     internal/template/locale.go — a new file this story adds under
//     internal/ — was unreachable because the walk root itself was
//     hardcoded to internal/expr.
//  2. No new `_test.go` sibling was named, though ScanForbiddenImports
//     polices test files too (the math-selector rule has no test
//     exemption at all).
//  3. (Fixed in forbiddenimports.go itself, not here:)
//     ForbiddenImportsStats now carries FilesSeenNames, so this
//     witness reads the SCANNER'S OWN record instead of re-deriving an
//     independent walk that could disagree with a scanner which
//     silently skipped something.
//
// Matches the story's own File List exactly: 13 files under
// internal/expr, 2 under internal/template, 1 under internal/bind.
var story34NewFiles = []string{
	"expr/formatcontext.go",
	"expr/locale.go",
	"expr/locale_test.go",
	"expr/tenpow.go",
	"expr/datepattern.go",
	"expr/datepattern_test.go",
	"expr/numberpattern.go",
	"expr/calendar.go",
	"expr/formatdate.go",
	"expr/formatdate_test.go",
	"expr/numberformat.go",
	"expr/numberformat_test.go",
	"expr/table_behavioral_test.go",
	"template/locale.go",
	"template/locale_test.go",
	"bind/formatcontext_test.go",
}

// TestForbiddenImportsScanSeesStory34Files is AC2's own-name coverage
// assertion: the REAL production scan over folio-go/internal/ (the
// exact root TestForbiddenImportsProductionScan above uses, not a
// narrower or independently-walked subdirectory) must report, in its
// OWN FilesSeenNames, every file in story34NewFiles — named, not
// merely counted.
func TestForbiddenImportsScanSeesStory34Files(t *testing.T) {
	root := repoRootFromTest(t)
	internalDir := filepath.Join(root, "folio-go", "internal")

	_, stats, err := ScanForbiddenImports(internalDir)
	if err != nil {
		t.Fatalf("scan %s: %v", internalDir, err)
	}
	if len(stats.FilesSeenNames) == 0 {
		t.Fatal("presence precondition (D-000.9): zero files seen under internal/ (FilesSeenNames)")
	}

	seen := map[string]bool{}
	for _, name := range stats.FilesSeenNames {
		seen[name] = true
	}
	for _, name := range story34NewFiles {
		if !seen[name] {
			t.Errorf("forbidden-imports scan's own FilesSeenNames did not include %s — coverage witness failed for a named file, not merely a count", name)
		}
	}
}

// TestForbiddenImportsRedProofOnStory34File is AC2's own red-proof,
// captured over a SCRATCH COPY of one of this story's real new files
// (never the committed file): injecting "time" must reden naming that
// file, and injecting math.Pow must reden naming "Pow" — reverted by
// construction (the scratch copy is never written back).
func TestForbiddenImportsRedProofOnStory34File(t *testing.T) {
	root := repoRootFromTest(t)
	src := filepath.Join(root, "folio-go", "internal", "expr", "calendar.go")
	orig, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}

	// Mutation 1: add "import \"time\"".
	withTime := strings.Replace(string(orig), "import \"fmt\"", "import (\n\t\"fmt\"\n\t\"time\"\n)\n\nvar _ = time.Now", 1)
	if withTime == string(orig) {
		t.Fatal("presence precondition: the injection point (\"import \\\"fmt\\\"\") was not found — this red-proof is stale")
	}
	dir1 := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir1, "calendar.go"), []byte(withTime), 0o644); err != nil {
		t.Fatalf("write scratch file: %v", err)
	}
	findings1, _, err := ScanForbiddenImports(dir1)
	if err != nil {
		t.Fatalf("scan scratch dir 1: %v", err)
	}
	found := false
	for _, f := range findings1 {
		if strings.Contains(f.Message, "calendar.go") && strings.Contains(f.Message, `"time"`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("RED-PROOF FAILED: injecting \"time\" into a scratch copy of calendar.go was not observed as a forbidden import naming the file, got: %v", findings1)
	}

	// Mutation 2: add a math.Pow call.
	withPow := strings.Replace(string(orig), "import \"fmt\"",
		"import (\n\t\"fmt\"\n\t\"math\"\n)\n\nvar _ = math.Pow(10, 2)", 1)
	dir2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir2, "calendar.go"), []byte(withPow), 0o644); err != nil {
		t.Fatalf("write scratch file: %v", err)
	}
	findings2, _, err := ScanForbiddenImports(dir2)
	if err != nil {
		t.Fatalf("scan scratch dir 2: %v", err)
	}
	foundPow := false
	for _, f := range findings2 {
		if strings.Contains(f.Message, "Pow") {
			foundPow = true
		}
	}
	if !foundPow {
		t.Fatalf("RED-PROOF FAILED: injecting math.Pow into a scratch copy of calendar.go was not observed as a forbidden math-selector call naming \"Pow\", got: %v", findings2)
	}
}
