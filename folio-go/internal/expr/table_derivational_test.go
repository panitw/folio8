package expr

// This file replaces TestThreeImplementedFunctions/
// TestFiveUnimplementedFunctions (Story 3.1/3.2's hard-coded name
// lists, F11/D-3.1a.3, AC29): "a guard whose expected value must be
// edited is one that gets edited wrongly" — those two tests named
// {"upper","lower","if"} and {"sum","count","avg","formatDate",
// "formatNumber"} verbatim, so Story 3.3's own edit (three of the five
// flip) and Story 3.4's (the remaining two flip) each required editing
// the guard in the same diff as the thing it guards, exactly the shape
// D-3.1a.3 warns against.
//
// CORRECTED (Story 3.4, F1/AC16). This file used to claim it "survives
// 3.3's edit (three names move across) and 3.4's (two more) with NO
// edit to this file at all." That was FALSE for one of its three
// guards: TestUnimplementedEntriesHaveNoEvalCallBranch carried its own
// presence precondition (D-000.9) asserting its unimplemented
// population was nonzero — and Story 3.4 flipping the last two entries
// drives that population to ZERO, on schedule, per the roadmap. D-000.9
// and D-000.59 collide at a scheduled zero: a presence precondition is
// itself a population-keyed assertion, so when the schedule empties its
// population the precondition fires as a false alarm at exactly the
// moment the guard it protects becomes correctly vacuous. RULED (AC16,
// DECISION-1, Arm A): the machinery this precondition protected —
// funcEntry.implemented, funcEntry.owningStory, eval.go's
// "if !entry.implemented" branch, and this test together with its
// precondition — is REMOVED, not kept inert, because the table is
// closed at eight (C1): the unimplemented population cannot refill,
// so keeping the mechanism (even inert) would leave something
// asserting a proposition that can never again be false.
//
// TestImplementedEntriesMatchEvalCallSwitch (below) is the ONE
// survivor, restated as plain set equality (every functionTable entry
// has an evalCall switch branch and vice versa) now that "implemented"
// is not a distinguishing property of any entry — every one of the
// eight is. Its own presence precondition (`len(tableImplemented) ==
// 0`) STAYS SAFE forever: that population goes to eight, never to
// zero. TestAllTableEntriesActuallyCompute
// (table_behavioral_test.go) is AC16's OTHER half — the OBLIGATION
// that a registered function actually computes, never merely re-read
// as the EVENT "all eight are implemented" (D-000.59).

import (
	"go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"os"
	"testing"
)

// evalCallSwitchNames AST-scans eval.go's evalCall function for its
// "switch entry.name { case "x", "y": … }" statement, returning every
// string literal any case clause matches (excluding "default", which
// carries no string literal at all and is evalCall's own
// unreachable-in-practice safety net, eval.go's own comment).
func evalCallSwitchNames(t *testing.T) map[string]bool {
	t.Helper()
	src, err := os.ReadFile("eval.go")
	if err != nil {
		t.Fatalf("read eval.go: %v", err)
	}
	fset := gotoken.NewFileSet()
	f, err := goparser.ParseFile(fset, "eval.go", src, 0)
	if err != nil {
		t.Fatalf("parse eval.go: %v", err)
	}

	names := map[string]bool{}
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "evalCall" {
			return true
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sw, ok := n.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			tag, ok := sw.Tag.(*ast.SelectorExpr)
			if !ok || tag.Sel.Name != "name" {
				return true // not the "switch entry.name" statement
			}
			found = true
			for _, stmt := range sw.Body.List {
				cc, ok := stmt.(*ast.CaseClause)
				if !ok || cc.List == nil {
					continue // cc.List == nil is the "default" clause
				}
				for _, expr := range cc.List {
					lit, ok := expr.(*ast.BasicLit)
					if !ok || lit.Kind != gotoken.STRING {
						t.Fatalf("evalCall's switch matches a non-literal case expression %#v — this scan cannot derive the implemented set from it", expr)
					}
					v := lit.Value
					if len(v) >= 2 {
						v = v[1 : len(v)-1]
					}
					names[v] = true
				}
			}
			return false
		})
		return true
	})
	if !found {
		t.Fatal("presence precondition (D-000.9): evalCall's \"switch entry.name\" statement was not found — the scan found nothing to disagree with")
	}
	return names
}

// TestImplementedEntriesMatchEvalCallSwitch is AC16's STRUCTURAL half
// (Story 3.4, restated now that every functionTable entry computes):
// functionTable's name set must equal evalCall's own switch-case set,
// in BOTH directions — a table entry with no branch is caught (it
// would hit evalCall's own "has no evaluator" internal error at
// runtime; this test catches it at build time instead), and a stray
// switch branch for a name absent from the table is caught too (dead
// code nothing in the table can ever select).
func TestImplementedEntriesMatchEvalCallSwitch(t *testing.T) {
	switchNames := evalCallSwitchNames(t)

	tableNames := map[string]bool{}
	for _, e := range functionTable {
		tableNames[e.name] = true
	}

	for name := range tableNames {
		if !switchNames[name] {
			t.Errorf("%s: functionTable has this entry, but evalCall's switch has no case for it", name)
		}
	}
	for name := range switchNames {
		if !tableNames[name] {
			t.Errorf("%s: evalCall's switch has a case for this, but functionTable has no entry for it", name)
		}
	}
	// This precondition is SAFE FOREVER (unlike the removed
	// TestUnimplementedEntriesHaveNoEvalCallBranch's — see this file's
	// top comment): functionTable is closed at eight (C1), so this
	// population goes to eight and never to zero.
	if len(tableNames) == 0 {
		t.Fatal("presence precondition (D-000.9): zero table entries found — the comparison above would pass vacuously")
	}
}
