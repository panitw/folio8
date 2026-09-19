package expr

import (
	"strings"
	"testing"
)

// TestCheckArity is AC10: declared arity is checked for all eight at
// parse time.
func TestCheckArity(t *testing.T) {
	cases := []struct {
		src     string
		wantErr bool
	}{
		{`sum(a, b)`, true}, // sum takes 1
		{`sum(a)`, false},
		{`if(a, "x")`, true}, // if takes 3
		{`if(a, "x", "y")`, false},
		{`upper()`, true}, // upper takes 1
		{`upper(a)`, false},
	}
	for _, c := range cases {
		e, err := Parse(c.src)
		if err != nil {
			t.Fatalf("Parse(%q): unexpected syntax error: %v", c.src, err)
		}
		cerr := Check(e)
		if (cerr != nil) != c.wantErr {
			t.Errorf("Check(%q): err=%v, wantErr=%v", c.src, cerr, c.wantErr)
		}
	}
}

// TestCheckUnknownFunction is AC11: an unknown function name is a
// located error naming the offending name and the eight legal names.
//
// QA Finding (Nit): the "eight legal names" half used to derive its
// own expectation from LegalFunctionNames() — the SAME source
// checkCall's message is built from (check.go: strings.Join(
// LegalFunctionNames(), ", ")) — so if LegalFunctionNames() ever
// shrank, message and loop would shrink together and this assertion
// would still pass; the circle closed only indirectly, through
// TestLegalFunctionNamesIsExactlyEight and AC7's own AST guard. Now
// checked against a literal, independently-stated list, the same
// discipline table_test.go and expr_arch_test.go already apply.
func TestCheckUnknownFunction(t *testing.T) {
	e, err := Parse(`frobnicate(a)`)
	if err != nil {
		t.Fatalf("unexpected syntax error: %v", err)
	}
	cerr := Check(e)
	if cerr == nil {
		t.Fatal("expected an unknown-function error")
	}
	if !strings.Contains(cerr.Error(), "frobnicate") {
		t.Errorf("error must name the offending function, got: %v", cerr)
	}
	for _, name := range []string{"sum", "count", "avg", "formatDate", "formatNumber", "upper", "lower", "if"} {
		if !strings.Contains(cerr.Error(), name) {
			t.Errorf("error must list the eight legal names (missing %q), got: %v", name, cerr)
		}
	}
}

// TestCheckLiteralArgumentKind is Decision 3/AC10's literal-argument-
// kind half: sum("hello") and formatNumber(x, 123) are both decidable
// without data and both located errors at Check time.
func TestCheckLiteralArgumentKind(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		wantErr bool
	}{
		{"sum_string_literal_operand", `sum("hello")`, true},
		{"sum_number_literal_operand", `sum(123)`, true},
		{"sum_path_operand_ok", `sum(transactions.amount)`, false},
		{"formatNumber_number_literal_pattern", `formatNumber(x, 123)`, true},
		{"formatNumber_string_literal_pattern_ok", `formatNumber(x, "#,##0.00")`, false},
		{"if_string_literal_condition", `if("x", a, b)`, true},
		{"if_number_literal_condition", `if(0, a, b)`, true},
		{"if_path_condition_ok", `if(cond, a, b)`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, err := Parse(c.src)
			if err != nil {
				t.Fatalf("Parse(%q): unexpected syntax error: %v", c.src, err)
			}
			cerr := Check(e)
			if (cerr != nil) != c.wantErr {
				t.Errorf("Check(%q): err=%v, wantErr=%v", c.src, cerr, c.wantErr)
			}
		})
	}
}

// TestCheckRecursesIntoNestedCalls confirms a defect inside a nested
// call's argument is still caught (AC3's nesting case, checked with
// the same rigour).
func TestCheckRecursesIntoNestedCalls(t *testing.T) {
	e, err := Parse(`formatNumber(sum("hello"), "#,##0.00")`)
	if err != nil {
		t.Fatalf("unexpected syntax error: %v", err)
	}
	if cerr := Check(e); cerr == nil {
		t.Fatal("expected the nested sum(\"hello\") literal-kind defect to be caught")
	}
}

// TestCheckBarePathAndLiteralsAreAlwaysWellFormed: a bare path or a
// standalone STRING literal has nothing left to check once Parse
// accepted it. A number literal is different (its own bounds are
// checked — see TestCheckRejectsOversizedNumberLiteral below), so
// "123", well within bounds, stays here as the in-bounds control case.
func TestCheckBarePathAndLiteralsAreAlwaysWellFormed(t *testing.T) {
	for _, src := range []string{"a.b.c", `"literal"`, "123"} {
		e, err := Parse(src)
		if err != nil {
			t.Fatalf("Parse(%q): unexpected syntax error: %v", src, err)
		}
		if cerr := Check(e); cerr != nil {
			t.Errorf("Check(%q): unexpected error: %v", src, cerr)
		}
	}
}

// TestCheckRejectsOversizedNumberLiteral is R3's numeric-literal half
// (QA Finding 7, Major): a literal's own bounds are decidable with no
// data at all, so Check — not Eval — must reject one that NewDecimal
// would reject. Before this fix both cases below parsed AND Checked
// clean, then failed only at evaluation (reproduced by the reviewer
// through the real public API): the failure mode R3/F3 were written
// to separate.
func TestCheckRejectsOversizedNumberLiteral(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"coefficient_overflow", "12345678901234567890123456789"},  // 29 significant digits, exceeds 19
		{"exponent_overflow", "1e999999999999999999999"},           // exceeds maxDecimalExponentMagnitude
		{"nested_in_call", `upper(12345678901234567890123456789)`}, // AC3's nesting case, checked with the same rigour
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, err := Parse(c.src)
			if err != nil {
				t.Fatalf("Parse(%q): unexpected syntax error (this is a CHECK-time property, not a parse one): %v", c.src, err)
			}
			if cerr := Check(e); cerr == nil {
				t.Errorf("Check(%q): expected a located error at load, got none — this literal must not survive to evaluation", c.src)
			}
		})
	}
}

// TestCheckRejectsLeadingZeroNumberLiteral is R3/Finding 7's second
// half: the parser's own grammar must match what ast.go's NumberLit
// doc comment already claims — "the same shape
// internal/template.SplitJSONNumber accepts" — which is JSON's number
// grammar, where a leading zero is illegal except for a lone "0".
// Before this fix "01"/"007"/"-01" all parsed and Checked clean, and
// SplitJSONNumber silently normalised "01" to "1" downstream because
// it explicitly trusts encoding/json's own upstream grammar check and
// performs none of its own.
func TestCheckRejectsLeadingZeroNumberLiteral(t *testing.T) {
	for _, src := range []string{"01", "007", "-01"} {
		if _, err := Parse(src); err == nil {
			t.Errorf("Parse(%q): expected a syntax error (leading zero is not JSON's number grammar), got none", src)
		}
	}
	// "0" itself, and "0.5", remain legal — a lone zero integer part is
	// JSON's own grammar, not a defect.
	for _, src := range []string{"0", "0.5", "-0"} {
		if _, err := Parse(src); err != nil {
			t.Errorf("Parse(%q): expected success, got error: %v", src, err)
		}
	}
}
