package bind

// Story 2.7, AC3: "no page namespace exists, and none can be added" —
// enforced structurally, over the subject that CAN express the
// violation (D-000.50). The expression language does not exist yet
// (Epic 3), so there is nothing to grep for "page" IN — a guard over an
// empty set cannot fail, which is not coverage. The subject that DOES
// exist today is exprResolver's resolution-root dispatch itself (the
// fence in BindText's own doc comment, text.go — cited by enclosing
// function rather than line number, Story 2.7's review, Finding 7: the
// line-number pointer here went stale once already, from that story's
// own edit): a "page" namespace comes into existence precisely when
// "page" becomes a FOURTH rootKind.
//
// RE-POINTED A SECOND TIME (Story 3.3 finisher pass, Finding 1 — a
// Blocker, ruled by the engineering lead). The property this file
// checks SPLITS INTO TWO, and they are answered by TWO DIFFERENT
// INSTRUMENTS, never blurred together:
//
//  1. "Can a root name be introduced anywhere other than a
//     declaration?" NO, and the COMPILER says so — not this file. Story
//     3.3's original re-point (R2) turned rootName/rootDesc into a
//     shared helper's return values, but they stayed bare strings, so a
//     dispatch that bypassed selectRoot entirely and called lookupBound
//     with a literal "page" argument still COMPILED and still PASSED
//     this file's guard. The finisher pass makes rootName/rootDesc one
//     defined type, rootKind (text.go): lookupBound's parameter is now
//     rootKind, not string, so `lookupBound(..., "page")` — an untyped
//     string constant — is not ASSIGNABLE to it and does not compile.
//     This is a compile-time property. It is proven by a real build
//     attempt with the compiler's own verbatim error recorded (in the
//     story's Delivery Log, D-000.24) — it is NOT a test in this file,
//     and must never be dressed as one (a `go build` failure cannot be
//     "asserted" by a `go test` run without either skipping the whole
//     package or building a throwaway module, both of which would
//     obscure the very property being demonstrated).
//  2. "Is the set of declared roots closed?" Answered here, by AST
//     SET-EQUALITY over every rootKind COMPOSITE LITERAL in this
//     package — the DECLARED set (declaredResolutionRootNames, text.go)
//     compared, BOTH DIRECTIONS, against the OBSERVED set (D-2.5.1: the
//     same shape internal/template/closedsets.go's align sets and
//     byte_neutrality_test.go's declaredEpic2GateObligations use;
//     `closedAligns` became `closedStyleAligns`/`closedColumnAligns`
//     when Story 7.3 split the one shared set in two).
//     This REPLACES the previous selectRoot-return-statement scan and
//     is STRICTLY WIDER: it sees a fourth root wherever a rootKind is
//     constructed with that name, whether or not selectRoot ever
//     returns it and whether or not it ever reaches lookupBound — the
//     exact generality the selectRoot-keyed proxy did not have (that
//     proxy is why Finding 1 was possible: a caller that never went
//     through selectRoot was invisible to it).
//
// Never a count (D-2.5.1: "never a count, never in a test name").
import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// packageSourceFiles lists every .go file in this package's own
// directory — production AND test files alike (Story 3.3 finisher
// pass: rootKind is unexported, so package bind's own directory is the
// ONLY place in the module a rootKind composite literal can be
// written at all; scanning test files too closes the same class of gap
// this story's review already found once — Finding 2's "a file the old
// scan never looked at").
func packageSourceFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		files = append(files, name)
	}
	if len(files) == 0 {
		t.Fatal("presence precondition: no .go files found in this package directory")
	}
	slices.Sort(files) // deterministic order, D-1.3.5
	return files
}

// observedRootKindNames AST-scans src for every rootKind{...} COMPOSITE
// LITERAL (the type declared in text.go) and reports the string-literal
// value of its "name" field — keyed (`rootKind{name: "page", ...}`) or
// positional (`rootKind{"page", "..."}`, name is field 0) alike.
//
// A zero-value literal with no elements at all (`rootKind{}` — used
// once, in collectionSubPath's own namespace-error return, where no
// root was ever selected) declares NO name and is silently skipped: it
// is not a root construction, it is an aborted-return placeholder that
// is never passed to lookupBound.
//
// A composite literal that DOES supply a name position/key but whose
// value is not a string literal (a variable, a call, a const
// expression) is reported as dynamicCallSites rather than silently
// skipped — the exact shape a smuggled root name would take if
// rootKind's own "name" field were ever assigned from something other
// than a literal.
func observedRootKindNames(t *testing.T, filename string, src []byte) (names []string, dynamicCallSites int) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}
	ast.Inspect(f, func(n ast.Node) bool {
		cl, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		ident, ok := cl.Type.(*ast.Ident)
		if !ok || ident.Name != "rootKind" {
			return true
		}
		if len(cl.Elts) == 0 {
			// Zero-value placeholder — declares no root name.
			return true
		}

		var nameVal ast.Expr
		keyed := false
		for _, elt := range cl.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				keyed = true
				if id, ok := kv.Key.(*ast.Ident); ok && id.Name == "name" {
					nameVal = kv.Value
				}
			}
		}
		if !keyed {
			// Positional form: rootKind's field 0 is "name".
			nameVal = cl.Elts[0]
		}
		if nameVal == nil {
			// Keyed form that omitted "name" entirely — its zero value
			// ("") is not a declared root; report it as a name so the
			// closed-set comparison below catches it rather than
			// silently accepting an implicit empty root.
			names = append(names, "")
			return true
		}
		lit, ok := nameVal.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			dynamicCallSites++
			return true
		}
		v := lit.Value
		if len(v) >= 2 {
			v = v[1 : len(v)-1]
		}
		names = append(names, v)
		return true
	})
	return names, dynamicCallSites
}

// observedRootKindNamesAcrossPackage scans every .go file in this
// package's directory, aggregating names and dynamicCallSites across
// all of them.
//
// overrideFile/overrideSrc substitute IN-MEMORY content for the one
// named file (used by the red-proof below to scan a MUTATED copy of a
// file without ever writing it to disk); every other file is read
// fresh. Pass "" / nil to scan the package exactly as committed.
func observedRootKindNamesAcrossPackage(t *testing.T, overrideFile string, overrideSrc []byte) (names []string, dynamicCallSites int) {
	t.Helper()
	for _, name := range packageSourceFiles(t) {
		src := overrideSrc
		if name != overrideFile || overrideSrc == nil {
			var err error
			src, err = os.ReadFile(name)
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
		}
		n, d := observedRootKindNames(t, name, src)
		names = append(names, n...)
		dynamicCallSites += d
	}
	return names, dynamicCallSites
}

// TestBindResolutionRootsAreClosed is AC3's/AC7's structural guard for
// property 2 above (the closed-set property). It does NOT check
// property 1 (whether a root can be introduced outside a declaration)
// — that is now a compile-time property with no test-shaped form; see
// this file's own top comment and the story's Delivery Log for its
// separately-recorded compiler proof.
//
// PRESENCE PRECONDITION (D-000.9/D-000.21): the scan must find at least
// one rootKind composite literal, or "the observed set equals the
// declared set" is satisfied vacuously by an empty scan finding nothing
// to disagree with.
func TestBindResolutionRootsAreClosed(t *testing.T) {
	observed, dynamic := observedRootKindNamesAcrossPackage(t, "", nil)
	if dynamic > 0 {
		t.Fatalf("%d rootKind composite literal(s) supply a NON-LITERAL name field — a computed root name "+
			"is exactly how a fourth resolution root would be smuggled past a literal scan; make it a literal "+
			"or extend this guard to resolve it", dynamic)
	}
	if len(observed) == 0 {
		t.Fatal("presence precondition (D-000.9): zero rootKind composite literals found — \"the observed set " +
			"equals the declared set\" would be satisfied vacuously by an empty scan")
	}

	declaredSet := map[string]bool{}
	for _, r := range declaredResolutionRootNames {
		declaredSet[r] = true
	}
	observedSet := map[string]bool{}
	for _, r := range observed {
		observedSet[r] = true
	}

	for _, d := range declaredResolutionRootNames {
		if !observedSet[d] {
			t.Errorf("declared resolution root %q is never constructed as a rootKind — either "+
				"remove it from declaredResolutionRootNames or restore its rootKind declaration", d)
		}
	}
	for r := range observedSet {
		if !declaredSet[r] {
			t.Errorf("OBSERVED resolution root %q (a rootKind composite literal's name field) is not in "+
				"declaredResolutionRootNames %v — a FOURTH resolution root has appeared. If this is deliberate, "+
				"it is a direction change under AD-4 (\"no page namespace exists, and none can be added\") "+
				"and needs the engineering lead's ruling, not a one-line edit to this list",
				r, declaredResolutionRootNames)
		}
	}

	wantSorted := append([]string(nil), declaredResolutionRootNames...)
	slices.Sort(wantSorted)
	gotSorted := append([]string(nil), observed...)
	slices.Sort(gotSorted)
	t.Logf("declared resolution roots: %v; observed at %d rootKind composite literal(s): %v", wantSorted, len(observed), gotSorted)
}

// TestBindResolutionRootsClosureRedProof is D-000.52: a structural
// claim about a guard is worth exactly as much as its demonstration.
// This introduces a FOURTH rootKind composite literal (name "page")
// into a SCRATCH COPY of text.go's source (never the committed file)
// and asserts TestBindResolutionRootsAreClosed's own scan catches it.
//
// Story 3.3 finisher pass, correcting AC7.3/Finding 15: the previous
// version of this red-proof appended a duplicate, non-compiling
// selectRoot FUNCTION declaration and described it as "reachable ONLY
// through a path a collection-method caller would take" — a claim the
// review found unsupported (a duplicate function declaration is
// reachable from nothing; it does not compile). This version injects
// exactly the artifact the closed-set scan is keyed on — a rootKind
// composite literal — with no claim about reachability, because the
// scan's whole point (per this file's top comment) is that reachability
// no longer matters: it sees a fourth root wherever declared.
func TestBindResolutionRootsClosureRedProof(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(".", "text.go"))
	if err != nil {
		t.Fatalf("read text.go: %v", err)
	}

	mutated := injectFourthRootKind(t, src)

	observed, dynamic := observedRootKindNamesAcrossPackage(t, "text.go", mutated)
	if dynamic > 0 {
		t.Fatalf("mutation produced a non-literal rootKind name — the mutation itself is malformed")
	}

	observedSet := map[string]bool{}
	for _, r := range observed {
		observedSet[r] = true
	}
	if !observedSet["page"] {
		t.Fatalf("presence precondition: the mutation was supposed to introduce a rootKind named "+
			"\"page\", but the scan over the mutated source did not observe \"page\" among %v — the mutation "+
			"did not take", observed)
	}

	declaredSet := map[string]bool{}
	for _, r := range declaredResolutionRootNames {
		declaredSet[r] = true
	}
	reddened := false
	for r := range observedSet {
		if !declaredSet[r] {
			reddened = true
		}
	}
	if !reddened {
		t.Fatal("RED-PROOF FAILED: a mutated source declaring a fourth rootKind \"page\" did not trip the " +
			"closure check — TestBindResolutionRootsAreClosed would pass on a document that had actually " +
			"acquired a page namespace")
	}
	t.Logf("red-proof: injecting a rootKind named \"page\" is observed as %v, outside declared %v — the guard reddens", observed, declaredResolutionRootNames)
}

// injectFourthRootKind appends a syntactically valid, PACKAGE-LEVEL
// rootKind composite literal — kindPage, unconditionally named "page"
// — to a COPY of src. The scan is textual/AST-only and does not need
// this to compile (a redeclared package-level var IS in fact a build
// error, exactly like the pre-finisher-pass duplicate-function
// injection it replaces), only to parse — see this file's own top
// comment for why reachability is no longer the property being tested.
func injectFourthRootKind(t *testing.T, src []byte) []byte {
	t.Helper()
	addition := "\n\nvar zzInjectedFourthRoot = rootKind{name: \"page\", desc: \"a smuggled page namespace\"}\n"
	out := make([]byte, 0, len(src)+len(addition))
	out = append(out, src...)
	out = append(out, addition...)
	return out
}
