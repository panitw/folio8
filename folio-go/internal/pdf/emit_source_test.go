package pdf

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ruleNumericFormatting is this guard's stable rule id (AC4).
const ruleNumericFormatting = "numeric-formatting"

// numericFormattingFinding is one violation, with its path relative to
// the scanned target directory (AC4) and this guard's stable rule id.
type numericFormattingFinding struct {
	path string
	rule string
	msg  string
}

// isForbiddenStrconvSelector reports whether name is one of the strconv
// functions D-1.1.b's guardrail names: the Format* family (including
// FormatFloat), Itoa, and the Append* family (including AppendFloat — the
// exact call this story's QA review used to demonstrate the hole: a text
// list that enumerated strconv.FormatFloat but not strconv.AppendFloat let
// a float64 conversion plus float formatting reach an output byte with
// every guard, the golden hash, vet and gofmt all silent — Blocker 2(a)).
func isForbiddenStrconvSelector(name string) bool {
	return name == "Itoa" || strings.HasPrefix(name, "Format") || strings.HasPrefix(name, "Append")
}

// isForbiddenFmtSelector reports whether name is a fmt function that
// formats or prints a value — "any fmt formatting call", D-1.1.b's
// guardrail text verbatim. There is no verb allow-list to keep in step
// with new fmt.* additions: none of these calls belong in internal/pdf
// outside numbers.go at all, so every formatting/printing entry point is
// forbidden regardless of which verbs it is given.
func isForbiddenFmtSelector(name string) bool {
	switch name {
	case "Sprint", "Sprintf", "Sprintln",
		"Fprint", "Fprintf", "Fprintln",
		"Print", "Printf", "Println",
		"Errorf", "Appendf", "Append":
		return true
	default:
		return false
	}
}

// importAliases maps each import's local name (its explicit alias, or the
// last path segment when unaliased) to its full import path, so a call
// can be matched by the package it actually resolves to rather than by
// literal source text.
func importAliases(file *ast.File) map[string]string {
	aliases := map[string]string{}
	for _, imp := range file.Imports {
		p := strings.Trim(imp.Path.Value, `"`)
		name := p
		if idx := strings.LastIndex(p, "/"); idx != -1 {
			name = p[idx+1:]
		}
		if imp.Name != nil {
			name = imp.Name.Name
		}
		aliases[name] = p
	}
	return aliases
}

func itoa10(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// numericFormattingStats reports what scanNumericFormatting actually
// examined, from the scanner's own execution (Major 5, this story's QA
// review) — see internal/arch_test.go's identically-named-in-spirit
// fix for why a second, independently-derived walk cannot be trusted as
// a vacuity guard: injecting a dead first statement into
// scanNumericFormatting would zero out its own reported stats but never
// touch an unrelated walk built the old way.
type numericFormattingStats struct {
	filesChecked int
}

// scanNumericFormatting is the AC1 pure checker for this rule, narrowed
// per D-1.3.2 to a single rule: within the given target directory
// (production: exactly folio-go/internal/pdf/; fixture: a tree standing
// in for it), every file except numbers.go — _test.go files included, no
// exemption (the shipped no-exemption choice is kept) — is forbidden from
// calling strconv.Format*/Itoa/Append* or a fmt formatting function. Any
// directory named testdata is excluded (AC2). D-1.3.2 deleted the
// second, module-wide half outright: nothing outside internal/pdf writes
// an output byte (AD-5), so that half protected nothing while costing
// AD-3's diagnostic carve-out (Story 1.4's fmt.Errorf, F-2).
//
// Matching resolves each call's import alias and its selector name via
// go/ast on the parsed call expression, not literal source text or
// regular expressions (D-1.1.b, Blocker 2(a)).
func scanNumericFormatting(root string) ([]numericFormattingFinding, numericFormattingStats, error) {
	fset := token.NewFileSet()
	var findings []numericFormattingFinding
	var stats numericFormattingStats

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if filepath.Base(path) == "numbers.go" {
			return nil // the one file this guardrail exempts
		}
		stats.filesChecked++

		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}

		aliases := importAliases(file)
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkgIdent, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			pkgPath, known := aliases[pkgIdent.Name]
			if !known {
				return true
			}
			pos := fset.Position(call.Pos())
			switch {
			case pkgPath == "strconv" && isForbiddenStrconvSelector(sel.Sel.Name):
				findings = append(findings, numericFormattingFinding{
					path: rel, rule: ruleNumericFormatting,
					msg: rel + ":" + itoa10(pos.Line) + ": strconv." + sel.Sel.Name,
				})
			case pkgPath == "fmt" && isForbiddenFmtSelector(sel.Sel.Name):
				findings = append(findings, numericFormattingFinding{
					path: rel, rule: ruleNumericFormatting,
					msg: rel + ":" + itoa10(pos.Line) + ": fmt." + sel.Sel.Name,
				})
			}
			return true
		})
		return nil
	})
	return findings, stats, err
}

func assertExactNumericFormattingFindings(t *testing.T, got []numericFormattingFinding, want []numericFormattingFinding) {
	t.Helper()
	gotSet := map[[2]string]bool{}
	for _, f := range got {
		gotSet[[2]string{f.path, f.rule}] = true
	}
	wantSet := map[[2]string]bool{}
	for _, f := range want {
		wantSet[[2]string{f.path, f.rule}] = true
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

// TestNumberFormattingIsConfinedToNumbersGo is AC6's / AC7's source-level
// guard, merged with D-1.1.b's file-scoped guardrail and narrowed by
// D-1.3.2 to scope exactly folio-go/internal/pdf/ (AC7): the production
// caller scans the real internal/pdf/ tree and asserts zero, failing on a
// scan error separately from, and before, the zero-findings assertion
// (AC5, RP-3b). The vacuity guard reads the scanner's OWN reported
// numericFormattingStats (Major 5, this story's QA review), not a
// second, independently-derived filepath.WalkDir: a dead scanner that
// returns immediately would leave an unrelated second walk untouched,
// but it zeroes out the stats this test now asserts against.
func TestNumberFormattingIsConfinedToNumbersGo(t *testing.T) {
	root := repoRootFromTest(t)
	pdfDir := filepath.Join(root, "folio-go", "internal", "pdf")

	findings, stats, err := scanNumericFormatting(pdfDir)
	if err != nil {
		t.Fatalf("walk internal/pdf/: %v", err)
	}

	if stats.filesChecked == 0 {
		t.Fatal("vacuity guard: scanner's own stats report 0 files checked")
	}

	if len(findings) > 0 {
		var msgs []string
		for _, f := range findings {
			msgs = append(msgs, f.msg)
		}
		t.Fatalf("forbidden strconv/fmt formatting call(s) found outside numbers.go (AC6/AC7, D-1.1.b):\n%s", strings.Join(msgs, "\n"))
	}
}

// TestNumberFormattingFixtureScan is the AC1 fixture caller, red-proving
// AC8's both directions on retained fixtures at
// folio-go/testdata/lint/numeric-formatting/ (never under
// folio-go/internal/, F-10):
//   - pdf/violating.go, standing in for a non-numbers.go file inside
//     internal/pdf, calls strconv.Itoa and IS reported when the scanned
//     root is pdf/ — the same scope the production caller uses (AC7).
//   - template/version.go, standing in for the shape Story 1.4's
//     internal/template/version.go will take (F-2: fmt.Errorf naming
//     the declared and supported versions), lives OUTSIDE pdf/ and is
//     NOT reported when the scanned root is pdf/.
//
// Blocker 4 (this story's QA review): the previous fixture named the
// fmt.Errorf file numbers.go, which hit the guard's pre-existing,
// unrelated numbers.go-filename carve-out — identically to how it would
// have applied under the deleted module-wide rule, so the fixture proved
// nothing about D-1.3.2's actual narrowing. Renaming it to version.go
// and moving it outside pdf/ makes the scope claim itself the thing under
// test: the second t.Run below scans the fixture tree's PARENT (widening
// the root past pdf/, exactly the regression a future edit could
// introduce) and asserts template/version.go DOES appear — proving,
// red, that its absence above is caused by AC7's scope, not by a
// filename exemption.
func TestNumberFormattingFixtureScan(t *testing.T) {
	root := repoRootFromTest(t)
	fixtureRoot := filepath.Join(root, "folio-go", "testdata", "lint", "numeric-formatting")

	t.Run("scoped to pdf/, matching production", func(t *testing.T) {
		got, stats, err := scanNumericFormatting(filepath.Join(fixtureRoot, "pdf"))
		if err != nil {
			t.Fatalf("walk fixture tree %s/pdf: %v", fixtureRoot, err)
		}
		if stats.filesChecked == 0 {
			t.Fatal("vacuity guard: scanner's own stats report 0 files checked")
		}
		want := []numericFormattingFinding{
			{path: "violating.go", rule: ruleNumericFormatting},
		}
		assertExactNumericFormattingFindings(t, got, want)
	})

	t.Run("widened to the parent, template/version.go must appear (Blocker 4)", func(t *testing.T) {
		got, _, err := scanNumericFormatting(fixtureRoot)
		if err != nil {
			t.Fatalf("walk fixture tree %s: %v", fixtureRoot, err)
		}
		want := []numericFormattingFinding{
			{path: filepath.Join("pdf", "violating.go"), rule: ruleNumericFormatting},
			{path: filepath.Join("template", "version.go"), rule: ruleNumericFormatting},
		}
		assertExactNumericFormattingFindings(t, got, want)
	})
}
