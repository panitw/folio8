// Package arch holds architectural fitness tests that apply across every
// package under folio-go/internal/, rather than to any one of them. It has
// no non-test files: it exists purely to run assertions like "no float64
// anywhere under internal/" that a single package's own test file cannot
// see past its own directory.
package arch

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ruleNoFloat64 is this guard's stable rule id (AC4): findings carry it so
// a fixture scan's expected set can assert by file and rule, never by
// count (AC1, D-1.3.3 amended).
const ruleNoFloat64 = "no-float64"

// finding is one violation, with its path relative to the scanned target
// directory (AC4) and this guard's stable rule id.
type finding struct {
	path string
	rule string
	msg  string
}

// findFloatOccurrences walks a parsed file for any appearance at all of
// the identifier "float64" or "float32", plus any untyped floating-point
// literal (token.FLOAT). Flagging the bare identifier wherever it occurs
// in the AST — rather than only at specific declaration positions —
// subsumes every type-position shape in one pass: func parameters,
// results and type parameters; struct fields, including nested,
// array/slice/map/pointer/variadic and anonymous-struct forms; interface
// method signatures; type declarations and aliases; and explicit
// conversions such as float64(x), whose callee is itself an *ast.Ident
// named "float64".
//
// This replaces an earlier version that type-asserted expr.(*ast.Ident)
// only at f.Type / decl.Type for a fixed set of declaration kinds, and so
// caught only the four narrowest shapes: this story's own QA review
// (Blocker 2(b) / Major 7) measured nine further shapes that walk missed
// entirely, including a bare float64(x) conversion — which is exactly
// what let a float64 conversion plus float formatting reach an output
// byte with this guard staying green.
func findFloatOccurrences(fset *token.FileSet, file *ast.File, path string) []string {
	var findings []string
	ast.Inspect(file, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.Ident:
			if v.Name == "float64" || v.Name == "float32" {
				p := fset.Position(v.Pos())
				findings = append(findings, path+":"+itoa(p.Line)+": identifier "+v.Name)
			}
		case *ast.BasicLit:
			if v.Kind == token.FLOAT {
				p := fset.Position(v.Pos())
				findings = append(findings, path+":"+itoa(p.Line)+": untyped floating-point literal "+v.Value)
			}
		}
		return true
	})
	return findings
}

func itoa(n int) string {
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

// walkGoFiles walks root and calls visit for every .go file found, with
// its parsed AST and its path relative to root (AC4: findings must carry
// a path relative to the scanned target directory). It skips any
// directory literally named "testdata", at any depth, by directory name
// (AC2) — not by path prefix and not by a list of specific paths — and,
// since Story 1.6's second, module-wide caller (AC5a, D-1.6.7), any
// directory whose name starts with "." (a dot-directory), at any depth,
// BY CATEGORY: never by naming a specific one such as ".matrix-build"
// — "a name on a list is the rot pattern" (AC5a). The root itself is
// never skipped by this rule even if its own name happens to start
// with "." (WalkDir's first callback IS the root; only entries found
// WHILE walking are subject to the dot-directory skip), which matters
// not at all in practice here since neither caller ever points this at
// a dot-named root, but keeps the predicate exactly "skip a
// dot-directory encountered during the walk", nothing more.
func walkGoFiles(fset *token.FileSet, root string, visit func(rel string, file *ast.File) error) error {
	first := true
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			isRoot := first
			first = false
			if d.Name() == "testdata" || (!isRoot && strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, parser.AllErrors)
		if perr != nil {
			return perr
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		return visit(rel, file)
	})
}

// noFloat64Stats reports what scanNoFloat64 actually examined, from the
// scanner's own execution (Major 5, this story's QA review): the
// production caller's vacuity guard used to re-derive "packages
// visited" and "declarations seen" with a second, independent call to
// walkGoFiles — which cannot see a dead scanner, because injecting
// `if true { return nil, nil }` as scanNoFloat64's first statement never
// touches that unrelated second walk. Reading these counts from
// scanNoFloat64's own return value closes that gap: the same mutation
// now zeroes out PackagesVisited and DeclsSeen too.
type noFloat64Stats struct {
	packagesVisited map[string]bool
	declsSeen       int
}

// scanNoFloat64 is the AC1 pure checker for this rule: a function over a
// target directory returning (findings, error), with no *testing.T
// parameter, no hard-coded root and no repo-root discovery inside it.
func scanNoFloat64(root string) ([]finding, noFloat64Stats, error) {
	fset := token.NewFileSet()
	var findings []finding
	stats := noFloat64Stats{packagesVisited: map[string]bool{}}
	err := walkGoFiles(fset, root, func(rel string, file *ast.File) error {
		pkgDir := filepath.Dir(rel)
		stats.packagesVisited[pkgDir] = true
		if pkgDir != "." {
			ast.Inspect(file, func(n ast.Node) bool {
				switch n.(type) {
				case *ast.FuncDecl, *ast.TypeSpec, *ast.ValueSpec:
					stats.declsSeen++
				}
				return true
			})
		}
		for _, msg := range findFloatOccurrences(fset, file, rel) {
			findings = append(findings, finding{path: rel, rule: ruleNoFloat64, msg: msg})
		}
		return nil
	})
	return findings, stats, err
}

// assertExactFindings implements AC1's "by file and rule, never by
// count" fixture assertion (RP-3c): a scan that finds the right *number*
// of wrong things, but the wrong ones, must still fail.
func assertExactFindings(t *testing.T, got []finding, want []finding) {
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
	// Deliberately not "len(got) == len(want)" (AC1: by file and rule,
	// never by count) — a single violating file may legitimately trip a
	// rule more than once (e.g. violating_conversion.go's float64 return
	// type and float64(x) conversion are two separate AST sites for the
	// same file+rule pair).
	if len(gotSet) != len(wantSet) {
		t.Errorf("distinct (file,rule) pair count mismatch: got %d, want %d", len(gotSet), len(wantSet))
	}
}

// TestNoFloat64UnderInternal is the AC1 production caller: it scans the
// real folio-go/internal/ tree and asserts zero findings. It fails on a
// scan error separately from, and before, asserting zero findings
// (AC5, RP-3b) — the two are two statements, never collapsed into one.
//
// Guard against vacuity (guard 2): it asserts, from scanNoFloat64's OWN
// returned noFloat64Stats (Major 5, this story's QA review — a second,
// independently-derived re-walk cannot see a dead scanner), that it
// actually visited the "geom" and "pdf" package directories **by name**,
// and counts declarations only from files in a real package directory
// (excluding this test's own directory, "."), so this cannot pass by
// walking zero files, and — the review's Minor 16 — cannot pass after
// deleting either real package while the other survives.
func TestNoFloat64UnderInternal(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve internal/ root: %v", err)
	}

	findings, stats, err := scanNoFloat64(root)
	if err != nil {
		t.Fatalf("walk internal/: %v", err)
	}

	if !stats.packagesVisited["geom"] || !stats.packagesVisited["pdf"] {
		t.Fatalf("vacuity guard: scanner's own stats did not report visiting package directories \"geom\" and \"pdf\", got %v", stats.packagesVisited)
	}
	if stats.declsSeen == 0 {
		t.Fatalf("vacuity guard: scanner's own stats report 0 declarations seen across real package directories (excluding this test's own directory)")
	}

	if len(findings) > 0 {
		var msgs []string
		for _, f := range findings {
			msgs = append(msgs, f.msg)
		}
		t.Fatalf("float64/float32 found under internal/ (forbidden by AD-23):\n%s", strings.Join(msgs, "\n"))
	}
}

// TestNoFloat64FixtureScan is the AC1 fixture caller: it scans the
// retained fixture tree at folio-go/testdata/lint/no-float64/ (D-1.3.3;
// never under folio-go/internal/, F-10) and asserts exactly the named
// findings, by file and rule, matching neither a subset nor a superset.
func TestNoFloat64FixtureScan(t *testing.T) {
	internalRoot, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve internal/ root: %v", err)
	}
	fixtureRoot := filepath.Join(filepath.Dir(internalRoot), "testdata", "lint", "no-float64")

	got, _, err := scanNoFloat64(fixtureRoot)
	if err != nil {
		t.Fatalf("walk fixture tree %s: %v", fixtureRoot, err)
	}

	want := []finding{
		{path: "violating_field.go", rule: ruleNoFloat64},
		{path: "violating_conversion.go", rule: ruleNoFloat64},
	}
	assertExactFindings(t, got, want)
}

// TestNoFloat64UnderModule is Story 1.6's AC5/AC5a (D-1.6.7): the
// SECOND production caller of the existing scanNoFloat64 checker — "a
// caller is not a checker" — pointed at the WHOLE module
// (folio-go/, root and below), not only folio-go/internal/. Measured
// (M-6, this story's Dev Notes): the module root sat outside every
// existing guard, so folio8.Render could carry a float64 today with
// nothing catching it. This subsumes internal/ (and, on arrival,
// fonts/, cmd/, wasm/) rather than needing a new call site each time a
// new top-level directory appears.
//
// Four binding properties (AC5a): whole module scope; `_test.go`
// included (D-1.3.1's precedent — no exemption pre-emptively); skip
// testdata/ and any dot-directory BY CATEGORY (walkGoFiles above,
// never a specific name); keep TestNoFloat64UnderInternal, the
// existing internal/-scoped caller, unmodified — two callers, one
// checker.
func TestNoFloat64UnderModule(t *testing.T) {
	internalRoot, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve internal/ root: %v", err)
	}
	moduleRoot := filepath.Dir(internalRoot) // folio-go/

	findings, stats, err := scanNoFloat64(moduleRoot)
	if err != nil {
		t.Fatalf("walk module root %s: %v", moduleRoot, err)
	}

	// Vacuity guard (D-000.9 + extension, D-000.13): from the
	// scanner's OWN reported stats, confirm it actually visited both
	// the module-root package ("." relative to moduleRoot) and the
	// "internal" directory the other caller already covers — a run
	// that visited nothing, or only internal/, would report the same
	// zero findings a healthy run does.
	if !stats.packagesVisited["."] {
		t.Fatalf("vacuity guard: scanner's own stats did not report visiting the module-root package (\".\"), got %v", stats.packagesVisited)
	}
	if !stats.packagesVisited["internal"] {
		t.Fatalf("vacuity guard: scanner's own stats did not report visiting \"internal\" from the module root, got %v", stats.packagesVisited)
	}
	if stats.declsSeen == 0 {
		t.Fatalf("vacuity guard: scanner's own stats report 0 declarations seen")
	}

	if len(findings) > 0 {
		var msgs []string
		for _, f := range findings {
			msgs = append(msgs, f.msg)
		}
		t.Fatalf("float64/float32 found under the module root (forbidden by AD-23):\n%s", strings.Join(msgs, "\n"))
	}
}
