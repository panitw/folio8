package rules

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestForbiddenImportsProductionScan is the AC1 production caller: it
// scans the real folio-go/internal/ tree and asserts zero findings,
// failing on a scan error separately from, and before, the zero-findings
// assertion (AC5, RP-3b). Non-vacuous per AC12's requirement that "the
// production scan over the real tree is green on the shipped suite" —
// internal/pdf/numbers_test.go and internal/geom/scale_test.go both
// import math today (F-6), so a scan that visited nothing would not
// exercise the selector logic at all. The vacuity guard reads the
// scanner's OWN reported ForbiddenImportsStats (Major 5, this story's
// QA review), not a second, independently-derived walk — see
// MapRangeStats' doc comment for why a second walk cannot be trusted:
// injecting a dead first statement into ScanForbiddenImports would zero
// out its own reported stats but never touch an unrelated walk built the
// old way.
func TestForbiddenImportsProductionScan(t *testing.T) {
	root := repoRootFromTest(t)
	internalDir := filepath.Join(root, "folio-go", "internal")

	findings, stats, err := ScanForbiddenImports(internalDir)
	if err != nil {
		t.Fatalf("scan folio-go/internal/: %v", err)
	}

	if stats.FilesSeen == 0 {
		t.Fatal("vacuity guard: scanner's own stats report 0 files seen under internal/")
	}
	filesWithMathImport := countFilesImporting(t, internalDir, "math")
	if filesWithMathImport == 0 {
		t.Fatal("vacuity guard: expected at least one file under internal/ importing \"math\" (F-6) — the production scan did not exercise the selector logic")
	}

	if len(findings) > 0 {
		var msgs []string
		for _, f := range findings {
			msgs = append(msgs, f.Message)
		}
		t.Fatalf("forbidden imports/selectors found under internal/ (AD-1):\n%s", strings.Join(msgs, "\n"))
	}
}

func countFilesImporting(t *testing.T, root, importPath string) int {
	t.Helper()
	count := 0
	err := walkGoFiles(root, func(rel string, file *ast.File, fset *token.FileSet) error {
		for _, imp := range file.Imports {
			if strings.Trim(imp.Path.Value, `"`) == importPath {
				count++
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return count
}

// TestForbiddenImportsFixtureScan is the AC1 fixture caller, red-proving
// AC13's ten fixtures at folio-go/testdata/lint/forbidden-imports/
// (never under folio-go/internal/, F-10): a non-test file and a
// `_test.go` file each importing time (D-1.3.1: two, not one); a call to
// a math function outside the seven; a reference to math.Pi; a file
// importing subpackages of three banned paths (Finding 6, this story's
// QA review: math/rand/v2, net/http, net/url, os/exec); and a dot
// import of "math" (Finding 15, Story 3.4's QA review: a dot import
// makes Pow(10, 2) unqualified, evading RuleMathSelector entirely — only
// RuleDotImport catches it, and RuleMathSelector must NOT also fire for
// the same file) — all reported — plus a `_test.go` file using the
// exemption's exact four imports, a file using math.MaxInt64, a file
// calling math.Abs, and a file whose comments name
// math.Round/math.MinInt64 (reproducing internal/geom/scale.go:31's
// shape) — none reported.
func TestForbiddenImportsFixtureScan(t *testing.T) {
	root := repoRootFromTest(t)
	fixtureRoot := filepath.Join(root, "folio-go", "testdata", "lint", "forbidden-imports")

	got, _, err := ScanForbiddenImports(fixtureRoot)
	if err != nil {
		t.Fatalf("scan fixture tree %s: %v", fixtureRoot, err)
	}

	want := []Finding{
		{Path: "violating_time_import.go", Rule: RuleForbiddenImports},
		{Path: "violating_time_import_test.go", Rule: RuleForbiddenImports},
		{Path: "violating_subpackage_imports.go", Rule: RuleForbiddenImports},
		{Path: "violating_math_call.go", Rule: RuleMathSelector},
		{Path: "violating_math_pi.go", Rule: RuleMathSelector},
		{Path: "violating_runtime_caller.go", Rule: RuleRuntimeCaller}, // Story 2.1, AC2, V1
		{Path: "violating_dot_import.go", Rule: RuleDotImport},         // Finding 15
	}
	assertExactFindings(t, got, want)
}

// TestForbiddenImportsDotImportEvadesMathSelectorAloneButNotDotImport
// is Finding 15's own named hazard, asserted directly rather than only
// implied by the fixture-scan count above: violating_dot_import.go's
// `. "math"` call to Pow must be caught by RuleDotImport, and it must
// NOT ALSO be caught by RuleMathSelector — proving RuleMathSelector's
// alias-resolution genuinely cannot see a dot-imported call (the exact
// gap the engineering lead's ruling named), and that RuleDotImport is
// the instrument closing it, not a coincidental second hit.
func TestForbiddenImportsDotImportEvadesMathSelectorAloneButNotDotImport(t *testing.T) {
	root := repoRootFromTest(t)
	fixtureRoot := filepath.Join(root, "folio-go", "testdata", "lint", "forbidden-imports")

	got, _, err := ScanForbiddenImports(fixtureRoot)
	if err != nil {
		t.Fatalf("scan fixture tree %s: %v", fixtureRoot, err)
	}

	var rulesForDotImportFile []string
	for _, f := range got {
		if f.Path == "violating_dot_import.go" {
			rulesForDotImportFile = append(rulesForDotImportFile, f.Rule)
		}
	}
	if len(rulesForDotImportFile) == 0 {
		t.Fatal("violating_dot_import.go produced no findings at all — RuleDotImport did not fire")
	}
	for _, r := range rulesForDotImportFile {
		if r == RuleMathSelector {
			t.Errorf("violating_dot_import.go was ALSO caught by RuleMathSelector (%v) — the fixture no longer demonstrates the evasion Finding 15 named", rulesForDotImportFile)
		}
	}
	if len(rulesForDotImportFile) != 1 || rulesForDotImportFile[0] != RuleDotImport {
		t.Fatalf("violating_dot_import.go: got rules %v, want exactly [%s]", rulesForDotImportFile, RuleDotImport)
	}
}

// TestForbiddenImportsMessageContent is AC10's message-content
// assertion (Finding 11, this story's QA review): AC10 requires the
// failure to name the file, the offending import or selector, and AD-1's
// allow-listed numeric surface, but no test previously inspected
// Finding.Message or Finding.Line for this rule — assertExactFindings
// discards both. Each literal below is written independently of the
// production constants it is checked against (the same discipline
// Finding 2's fix established for EscapeHatch), so a drift in either the
// message template or allowedNumericSurface reddens this test.
func TestForbiddenImportsMessageContent(t *testing.T) {
	const wantSurface = "+ - * /, comparison, and Sqrt, Floor, Ceil, Round, Trunc, Abs, Mod"
	if allowedNumericSurface != wantSurface {
		t.Fatalf("allowedNumericSurface drifted from the literal this test independently spells out:\n got:  %q\n want: %q", allowedNumericSurface, wantSurface)
	}

	root := repoRootFromTest(t)
	fixtureRoot := filepath.Join(root, "folio-go", "testdata", "lint", "forbidden-imports")

	got, _, err := ScanForbiddenImports(fixtureRoot)
	if err != nil {
		t.Fatalf("scan fixture tree %s: %v", fixtureRoot, err)
	}

	messageFor := func(path, rule string) string {
		t.Helper()
		for _, f := range got {
			if f.Path == path && f.Rule == rule {
				return f.Message
			}
		}
		t.Fatalf("no finding for path=%s rule=%s", path, rule)
		return ""
	}

	// Banned import: names the file, the import, and the allow-listed surface.
	bannedMsg := messageFor("violating_time_import.go", RuleForbiddenImports)
	if !strings.Contains(bannedMsg, "violating_time_import.go") {
		t.Errorf("banned-import message missing file name: %q", bannedMsg)
	}
	if !strings.Contains(bannedMsg, `"time"`) {
		t.Errorf("banned-import message missing offending import: %q", bannedMsg)
	}
	if !strings.Contains(bannedMsg, wantSurface) {
		t.Errorf("banned-import message missing allow-listed numeric surface: %q", bannedMsg)
	}

	// math call outside the seven: names the file, the selector, and the surface.
	callMsg := messageFor("violating_math_call.go", RuleMathSelector)
	if !strings.Contains(callMsg, "violating_math_call.go") {
		t.Errorf("math-call message missing file name: %q", callMsg)
	}
	if !strings.Contains(callMsg, wantSurface) {
		t.Errorf("math-call message missing allow-listed numeric surface: %q", callMsg)
	}

	// math.Pi non-call reference: this exact clause was unmet before
	// Finding 11's fix — the non-call message previously omitted the
	// allow-listed numeric surface entirely.
	piMsg := messageFor("violating_math_pi.go", RuleMathSelector)
	if !strings.Contains(piMsg, "violating_math_pi.go") {
		t.Errorf("math.Pi message missing file name: %q", piMsg)
	}
	if !strings.Contains(piMsg, "math.Pi") {
		t.Errorf("math.Pi message missing offending selector: %q", piMsg)
	}
	if !strings.Contains(piMsg, wantSurface) {
		t.Errorf("math.Pi message missing allow-listed numeric surface (Finding 11): %q", piMsg)
	}
}

// TestRenderEntryFileHasNoForbiddenImports is Story 1.6's AC12/AC13
// (D-1.6.3): "the file declaring Render and RenderTo imports none of
// os, time, net, math/rand" is asserted here by REUSING
// ScanForbiddenImports — never a newly invented scanner (AC13) —
// pointed at whichever files under folio-go/ this guard's target set
// names (findFolio8GoScanTargets below), not at a hard-coded path
// (filepath.WalkDir, which ScanForbiddenImports/walkGoFiles are built
// on, visits a non-directory root exactly once, so this is the
// existing checker at file scope, not a second checker).
//
// QA Finding 1 (Story 1.7's review, Blocker): the original version
// hard-coded folio-go/render_entry.go as the target. Moving func Render
// back into folio8.go (which imports "os"), leaving render_entry.go in
// place holding only type Data and a doc comment, left this test
// PASSING — the guard asserted a filename, never that the file
// declares Render. D-1.6.3's own justification ("fails the moment
// someone adds a convenience os.Getenv to the render entry point") was
// false the moment the entry point moved, which is exactly what Story
// 1.7 does when it adds RenderTo. Story 1.7 fixed this by deriving the
// target set from an AST lookup for the name set {Render, RenderTo}
// (later {Render, RenderTo, Validate} at Story 3.7, AC12(b)).
//
// QA Finding 4 (Story 3.7's review, Major) / D-3.7.9(b): that
// NAME-DRIVEN anchor was itself an anchor the code could move — a
// fourth public function (e.g. Preview) reading the environment built
// and passed every test, because nothing scans a file until its
// declared name is added to the literal set first. The engineering
// lead's ruling: do NOT widen the name set further; INVERT the
// population. Every non-test .go file directly under folio-go/ is now
// scanned UNLESS it is named in allowedWorldReadingFiles below, each
// entry carrying its reason — so a brand-new file (Preview or
// anything else) is fenced THE DAY IT IS CREATED, by default, with no
// test edit required. Only a file deliberately added to the allowlist
// may import the world; the burden of proof is now on the exception,
// not on remembering to name the rule.
//
// Vacuity guard (AC25/D-000.9, sharpened by D-000.13 — Finding 1):
// findFolio8GoScanTargets itself fails the test if every non-test file
// under folio-go/ is allowlisted, so a run that finds nothing to scan
// cannot read as "zero findings, pass". For each target file,
// filesSeen == 1 is asserted explicitly, and a missing file is already
// a walk error surfaced by ScanForbiddenImports itself (D-1.3.3
// amended: a target that cannot be read is never silently "zero
// findings").
func TestRenderEntryFileHasNoForbiddenImports(t *testing.T) {
	root := repoRootFromTest(t)
	targets := findFolio8GoScanTargets(t, root)

	for _, target := range targets {
		findings, stats, err := ScanForbiddenImports(target)
		if err != nil {
			t.Fatalf("scan %s: %v (AC12: every non-allowlisted file under folio-go/ must exist and be readable)", target, err)
		}
		if stats.FilesSeen != 1 {
			t.Fatalf("vacuity guard: expected exactly 1 file seen (%s itself), got %d — stats: %+v", target, stats.FilesSeen, stats)
		}
		if len(findings) > 0 {
			var msgs []string
			for _, f := range findings {
				msgs = append(msgs, f.Message)
			}
			t.Fatalf("AC12: %s must import none of os/time/net/math/rand (or be added to allowedWorldReadingFiles WITH a reason), found:\n%s", target, strings.Join(msgs, "\n"))
		}
	}
}

// TestFindRenderDeclaringFilesExcludesFolio8Go is AC12(b)'s NON-FIRING
// CONTROL (D-000.68, D-3.7.9(b): "the same instrument as 3.6's
// add-with-pin mutation"). folio8.go legitimately imports "os" for
// LoadTemplate (D-1.4.6's os boundary is the package, not this file),
// so it must be — and stay — the one entry in allowedWorldReadingFiles,
// and findFolio8GoScanTargets must NEVER return it. A guard that (by
// accident of a future refactor) started scanning folio8.go would turn
// a legitimate import into a false positive (D-000.15's erosion path);
// this control catches that before it ever reaches ScanForbiddenImports.
func TestFindRenderDeclaringFilesExcludesFolio8Go(t *testing.T) {
	if _, ok := allowedWorldReadingFiles["folio8.go"]; !ok {
		t.Fatalf("allowedWorldReadingFiles no longer names folio8.go — LoadTemplate's legitimate \"os\" import has no exemption and TestRenderEntryFileHasNoForbiddenImports will now (correctly) fail on it; this control exists to catch the OPPOSITE mistake, not to be silenced by this one")
	}
	root := repoRootFromTest(t)
	targets := findFolio8GoScanTargets(t, root)
	for _, f := range targets {
		if strings.HasSuffix(f, string(filepath.Separator)+"folio8.go") {
			t.Fatalf("findFolio8GoScanTargets returned folio8.go, which is allowlisted and legitimately imports \"os\" — the guard has been widened into a false positive: %v", targets)
		}
	}
}

// allowedWorldReadingFiles is the test-owned EXCEPTION list D-3.7.9(b)
// requires (QA Finding 4): every non-test .go file directly under
// folio-go/ is forbidden os/time/net/math/rand UNLESS it is named
// here, WITH its reason. This is the closed set today; adding a
// second entry is a deliberate, reviewable edit, exactly as adding to
// the old pureEntryPointNames literal was — the difference is that an
// OMISSION here (forgetting to allowlist a new legitimately
// world-reading file) fails the build loudly, where an omission from
// the old name-driven list failed silently.
var allowedWorldReadingFiles = map[string]string{
	"folio8.go": "declares LoadTemplate, which reads a template from a caller-supplied file path — D-1.4.6's os boundary is the package, not this file",
}

// findFolio8GoScanTargets locates every non-test .go file directly
// under folio-go/ (the module-root package) that is NOT named in
// allowedWorldReadingFiles — D-3.7.9(b)'s inverted anchor, replacing
// Story 3.7's name-driven findRenderDeclaringFiles (QA Finding 4): the
// population is now "everything, minus a reviewed exception list",
// never "everything matching a name someone remembered to add".
//
// Vacuity guard (D-000.9/D-000.13): every file allowlisted (i.e. zero
// scan targets) is a hard failure here, not a scan that silently found
// nothing to check — the caller must never be able to read an empty
// result as "the property holds".
func findFolio8GoScanTargets(t *testing.T, root string) []string {
	t.Helper()
	folio8GoDir := filepath.Join(root, "folio-go")
	entries, err := os.ReadDir(folio8GoDir)
	if err != nil {
		t.Fatalf("read %s: %v", folio8GoDir, err)
	}

	var files []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if _, allowed := allowedWorldReadingFiles[name]; allowed {
			continue
		}
		files = append(files, filepath.Join(folio8GoDir, name))
	}

	if len(files) == 0 {
		t.Fatalf("vacuity guard: every non-test file directly under %s is allowlisted in allowedWorldReadingFiles — AC12's property is unassertable (D-000.9/D-000.13)", folio8GoDir)
	}
	sort.Strings(files)
	return files
}
