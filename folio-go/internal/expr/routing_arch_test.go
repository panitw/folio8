package expr

// Story 3.3, R4/AC8/AC14, discharging D-3.1a.4's own follow-up "owed
// to 3.3 by name": the reducer inventory (D-3.1a.3,
// internal/reducer_inventory_arch_test.go) is a declaration-shape
// set-equality check over the MODULE — it asserts SumDecimals and
// AvgDecimals exist, in the right package, with the right signature.
// It does NOT assert anything CALLS them (D-3.1a.4's own correction of
// this story's earlier over-claim). F6, measured at this story's
// creation: an inline big.Int accumulator dropped into evalSum's body
// instead of a call to SumDecimals passes every guard that existed
// before this file — the reducer inventory, the float-ban guards, the
// function table's closed-eight check — none of them look at evalSum's
// BODY at all.
//
// This file is the positive assertion that closes that gap (D-000.59's
// shape — assert the OBLIGATION, not the event that happens to satisfy
// it today): evalSum's body must CONTAIN a call to SumDecimals, and
// evalAvg's must contain a call to AvgDecimals, checked by AST over
// aggregate.go — and each red-proof demonstrates, on a MUTATED SCRATCH
// COPY (never the committed file, same discipline as every other
// red-proof in this module), that an inline-accumulator rewrite is
// exactly what this assertion catches.

import (
	"go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"os"
	"strings"
	"testing"
)

// funcCallsName AST-scans a function declaration named fnName inside
// src for a CallExpr whose callee is the bare identifier calleeName —
// e.g. evalSum calling SumDecimals(...). Returns false, false if fnName
// itself was never found (a presence-precondition failure the caller
// must check separately from "found the function, but it doesn't call
// calleeName").
func funcCallsName(t *testing.T, filename string, src []byte, fnName, calleeName string) (found, calls bool) {
	t.Helper()
	fset := gotoken.NewFileSet()
	f, err := goparser.ParseFile(fset, filename, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != fnName || fn.Body == nil {
			return true
		}
		found = true
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			ident, ok := call.Fun.(*ast.Ident)
			if ok && ident.Name == calleeName {
				calls = true
			}
			return true
		})
		return false
	})
	return found, calls
}

// TestSumRoutesThroughSumDecimals is AC8/R4's positive routing
// assertion: evalSum's body must call SumDecimals — never a second,
// hand-rolled accumulator, however correct one might look.
func TestSumRoutesThroughSumDecimals(t *testing.T) {
	src, err := os.ReadFile("aggregate.go")
	if err != nil {
		t.Fatalf("read aggregate.go: %v", err)
	}
	found, calls := funcCallsName(t, "aggregate.go", src, "evalSum", "SumDecimals")
	if !found {
		t.Fatal("presence precondition (D-000.9): evalSum was not found in aggregate.go")
	}
	if !calls {
		t.Fatal("AC8/R4 VIOLATED: evalSum's body does not call SumDecimals — sum() must route through the one kernel D-1.4.1/DW-7 requires every caller (this expression, and Story 4.5's table footer) to share")
	}
}

// TestAvgRoutesThroughAvgDecimals is AC14/R4's counterpart for avg().
func TestAvgRoutesThroughAvgDecimals(t *testing.T) {
	src, err := os.ReadFile("aggregate.go")
	if err != nil {
		t.Fatalf("read aggregate.go: %v", err)
	}
	found, calls := funcCallsName(t, "aggregate.go", src, "evalAvg", "AvgDecimals")
	if !found {
		t.Fatal("presence precondition (D-000.9): evalAvg was not found in aggregate.go")
	}
	if !calls {
		t.Fatal("AC14/R4 VIOLATED: evalAvg's body does not call AvgDecimals — avg() must route through the one kernel D-1.4.1/DW-7 requires every caller to share")
	}
}

// TestAvgRoutingRedProofInlineAccumulator is AC14's captured red-proof
// (Story 3.3 finisher pass, Finding 14): the `avg` mirror of
// TestSumRoutingRedProofInlineAccumulator below, which AC14 asked for
// explicitly ("the same positive routing assertion AND captured
// red-proof shape as AC8") and which shipped without it. Textual
// removal of the routing call, same AST-only discipline as the sum
// red-proof (never compiled or run).
func TestAvgRoutingRedProofInlineAccumulator(t *testing.T) {
	src, err := os.ReadFile("aggregate.go")
	if err != nil {
		t.Fatalf("read aggregate.go: %v", err)
	}
	marker := "\tavg, err := AvgDecimals(decimals)\n"
	if !strings.Contains(string(src), marker) {
		t.Fatalf("presence precondition: aggregate.go no longer contains the expected \"AvgDecimals(decimals)\" call line — this red-proof's injection point is stale")
	}
	inline := "\tinlineTotal := 0\n" +
		"\tfor _, d := range decimals {\n" +
		"\t\tinlineTotal += int(d.Coefficient)\n" +
		"\t}\n" +
		"\tavg, err := Decimal{Coefficient: int64(inlineTotal) / int64(len(decimals)), Exponent: 0}, error(nil)\n"
	mutated := []byte(strings.Replace(string(src), marker, inline, 1))

	found, calls := funcCallsName(t, "aggregate.go", mutated, "evalAvg", "AvgDecimals")
	if !found {
		t.Fatal("presence precondition: mutated evalAvg was not found by the scan")
	}
	if calls {
		t.Fatal("RED-PROOF FAILED: the mutated source still shows evalAvg calling AvgDecimals — the injection did not remove the routing call it was meant to remove")
	}
	t.Log("red-proof: an inline accumulator replacing the AvgDecimals(...) call is observed as NOT calling AvgDecimals — TestAvgRoutesThroughAvgDecimals would redden on this source, exactly as AC14 requires")
}

// TestSumRoutingRedProofInlineAccumulator is AC8's captured red-proof
// (D-000.30, F6): a SCRATCH COPY of aggregate.go with evalSum's call to
// SumDecimals replaced by an inline big.Int accumulation loop — the
// EXACT hazard F6 measured passing every OTHER guard in the module —
// must fail TestSumRoutesThroughSumDecimals' own assertion. This
// window shuts the moment sum() is wired (D-000.30); it is captured
// here, permanently, as a red-proof rather than a one-time manual
// demonstration, so the assertion's teeth are re-proven on every run,
// not just believed once.
func TestSumRoutingRedProofInlineAccumulator(t *testing.T) {
	src, err := os.ReadFile("aggregate.go")
	if err != nil {
		t.Fatalf("read aggregate.go: %v", err)
	}
	marker := "\ttotal, err := SumDecimals(decimals)\n"
	if !strings.Contains(string(src), marker) {
		t.Fatalf("presence precondition: aggregate.go no longer contains the expected \"SumDecimals(decimals)\" call line — this red-proof's injection point is stale")
	}
	// CORRECTED (Story 3.3 finisher pass, Finding 13). The injected text
	// below is a TEXTUAL REMOVAL of the routing call, nothing more: it
	// is parsed by go/parser (this red-proof is AST-only, never compiled
	// or run — same discipline as every other scratch-copy red-proof in
	// this module) and asserted NOT to contain a call to SumDecimals.
	// The previous version of this comment claimed the replacement was
	// "a SYNTACTICALLY VALID, honest inline big.Int accumulator" that
	// "would, if actually compiled and run, compute the SAME correct
	// answer as SumDecimals" — both false: `bigInt`/`bigIntFromInt64`
	// are not identifiers this package declares or imports (the real
	// spellings are `big.Int`/`big.NewInt`, and `aggregate.go` does not
	// import "math/big" at all), and the replacement ignores
	// `d.Exponent` entirely, so it would not compute the same answer for
	// any mixed-exponent operand set — including AC12's own
	// 10.00/20.00/30.00 corpus. This red-proof's teeth do not depend on
	// the replacement's arithmetic being correct: it exists to prove
	// TestSumRoutesThroughSumDecimals catches the ABSENCE of the routing
	// call, and it does (verified separately, by hand, with a real
	// compiling exponent-aware accumulator dropped into a scratch copy
	// of the actual file — TestSumRoutesThroughSumDecimals reddened on
	// that too).
	inline := "\tinlineTotal := 0\n" +
		"\tfor _, d := range decimals {\n" +
		"\t\tinlineTotal += int(d.Coefficient)\n" +
		"\t}\n" +
		"\ttotal, err := Decimal{Coefficient: int64(inlineTotal), Exponent: 0}, error(nil)\n"
	mutated := []byte(strings.Replace(string(src), marker, inline, 1))

	found, calls := funcCallsName(t, "aggregate.go", mutated, "evalSum", "SumDecimals")
	if !found {
		t.Fatal("presence precondition: mutated evalSum was not found by the scan")
	}
	if calls {
		t.Fatal("RED-PROOF FAILED: the mutated source still shows evalSum calling SumDecimals — the injection did not remove the routing call it was meant to remove")
	}
	t.Log("red-proof: an inline big.Int accumulator replacing the SumDecimals(...) call is observed as NOT calling SumDecimals — TestSumRoutesThroughSumDecimals would redden on this source, exactly as AC8 requires")
}

// ---------------------------------------------------------------------
// AC18 (Story 3.3 finisher pass, Finding 3) — "the absent and wrong-kind
// arms go red without the kernel being called at all", PROVEN rather
// than asserted (D-000.9). internal/bind's ProjectCollection/
// CollectionLength cannot call the kernel at all (a different package,
// never imported for that purpose) — see aggregate_test.go's own
// corrected comment. The only place the property is actually decided
// is HERE: evalSum/evalAvg, which call BOTH resolver.ProjectCollection
// and the kernel, in that order.
// ---------------------------------------------------------------------

// resolverErrorGuardPrecedesKernelCall AST-parses the function named
// fnName in src and reports (found, guarded):
//
//   - found is true if fnName was located at all.
//   - guarded is true if a TOP-LEVEL statement assigns from a call to
//     resolverMethod (a selector call, e.g. resolver.ProjectCollection),
//     the VERY NEXT top-level statement is an "if" whose body contains
//     a return statement (the error-check guard), and that guard's own
//     source position is STRICTLY BEFORE the first call to kernelFunc
//     anywhere in the function body.
//
// This is a STRUCTURAL instrument — a declaration-shape check, not a
// runtime call-graph one (same honesty class D-3.1a.4 already applies
// to the reducer inventory): it proves the kernel call is not
// TEXTUALLY reachable without first passing the error-check-and-return
// on resolverMethod's own result, which — combined with Go's ordinary
// sequential control flow within one function — is what "the kernel is
// never called on this path" reduces to. It is the "equivalent
// instrument" AC18 asks for in place of a runtime call-counter, which
// has no natural attachment point here: SumDecimals/AvgDecimals are
// plain package-level functions with no injection seam, and giving them
// one purely for testability would either violate D-3.1a.2 (the kernel
// is unchanged by this story) or require an indirection that would
// itself defeat TestSumRoutesThroughSumDecimals'/TestAvgRoutesThroughAvgDecimals's
// own AST-literal-call scan above.
func resolverErrorGuardPrecedesKernelCall(t *testing.T, filename string, src []byte, fnName, resolverMethod, kernelFunc string) (found, guarded bool) {
	t.Helper()
	fset := gotoken.NewFileSet()
	f, err := goparser.ParseFile(fset, filename, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}

	var guardPos gotoken.Pos
	var kernelPos gotoken.Pos

	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != fnName || fn.Body == nil {
			return true
		}
		found = true

		stmts := fn.Body.List
		for i, stmt := range stmts {
			assign, ok := stmt.(*ast.AssignStmt)
			if !ok {
				continue
			}
			callsMethod := false
			for _, rhs := range assign.Rhs {
				call, ok := rhs.(*ast.CallExpr)
				if !ok {
					continue
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == resolverMethod {
					callsMethod = true
				}
			}
			if !callsMethod || i+1 >= len(stmts) {
				continue
			}
			ifStmt, ok := stmts[i+1].(*ast.IfStmt)
			if !ok {
				continue
			}
			for _, s := range ifStmt.Body.List {
				if _, ok := s.(*ast.ReturnStmt); ok {
					guardPos = ifStmt.Pos()
				}
			}
			if guardPos != gotoken.NoPos {
				break
			}
		}

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == kernelFunc {
				if kernelPos == gotoken.NoPos || call.Pos() < kernelPos {
					kernelPos = call.Pos()
				}
			}
			return true
		})
		return false
	})

	if !found {
		return false, false
	}
	if guardPos == gotoken.NoPos || kernelPos == gotoken.NoPos {
		return true, false
	}
	return true, guardPos < kernelPos
}

// TestKernelCallIsGuardedByProjectCollectionError is AC18: evalSum's
// and evalAvg's calls to SumDecimals/AvgDecimals are each textually
// guarded by an immediate error-check-and-return on the PRECEDING call
// to resolver.ProjectCollection — the absent/wrong-kind collection
// states (R8) are exactly the states ProjectCollection reports as an
// error (text.go), so this proves the kernel is never reached on
// those two arms.
func TestKernelCallIsGuardedByProjectCollectionError(t *testing.T) {
	src, err := os.ReadFile("aggregate.go")
	if err != nil {
		t.Fatalf("read aggregate.go: %v", err)
	}
	for _, tc := range []struct{ fn, kernel string }{
		{"evalSum", "SumDecimals"},
		{"evalAvg", "AvgDecimals"},
	} {
		found, guarded := resolverErrorGuardPrecedesKernelCall(t, "aggregate.go", src, tc.fn, "ProjectCollection", tc.kernel)
		if !found {
			t.Fatalf("presence precondition (D-000.9): %s was not found in aggregate.go", tc.fn)
		}
		if !guarded {
			t.Fatalf("AC18 VIOLATED: %s's call to %s is not textually guarded by an immediate "+
				"error-check-and-return on ProjectCollection's own result — the absent/wrong-kind "+
				"collection states could reach the kernel", tc.fn, tc.kernel)
		}
	}
}

// TestKernelCallGuardRedProof is AC18's captured red-proof (D-000.30):
// a SCRATCH COPY of aggregate.go with evalSum's ProjectCollection
// error check REMOVED (the error is discarded instead of returned)
// must fail TestKernelCallIsGuardedByProjectCollectionError's own
// assertion — proving the guard has real teeth, not merely a shape
// that happens to be present today.
func TestKernelCallGuardRedProof(t *testing.T) {
	src, err := os.ReadFile("aggregate.go")
	if err != nil {
		t.Fatalf("read aggregate.go: %v", err)
	}
	marker := "\tprojected, err := resolver.ProjectCollection(pathExpr.Segments)\n\tif err != nil {\n\t\treturn Value{}, nil, err\n\t}\n"
	if !strings.Contains(string(src), marker) {
		t.Fatalf("presence precondition: aggregate.go's evalSum no longer contains the expected ProjectCollection error-check block — this red-proof's injection point is stale")
	}
	// Discard the error rather than returning it — the exact defect
	// this guard exists to catch: the kernel would now be reachable
	// even when ProjectCollection failed.
	mutated := []byte(strings.Replace(string(src), marker, "\tprojected, _ := resolver.ProjectCollection(pathExpr.Segments)\n", 1))

	found, guarded := resolverErrorGuardPrecedesKernelCall(t, "aggregate.go", mutated, "evalSum", "ProjectCollection", "SumDecimals")
	if !found {
		t.Fatal("presence precondition: mutated evalSum was not found by the scan")
	}
	if guarded {
		t.Fatal("RED-PROOF FAILED: removing evalSum's ProjectCollection error check still reads as guarded — the injection did not remove the guard it was meant to remove")
	}
	t.Log("red-proof: removing evalSum's error-check-and-return on ProjectCollection's result is observed as NOT guarded — TestKernelCallIsGuardedByProjectCollectionError would redden on this source, exactly as AC18 requires")
}
