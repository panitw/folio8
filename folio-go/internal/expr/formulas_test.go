package expr

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestFormulaAcceptanceValuesAndScales(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"1 / 3", "0.3333"}, {"2 / 3", "0.6667"}, {"1.00 / 3", "0.333333"}, {"1 / 8", "0.1250"},
		{"1 / 32", "0.0312"}, {"3 / 32", "0.0938"}, {"-1 / 32", "-0.0312"}, {"-3 / 32", "-0.0938"}, {"1 / -32", "-0.0312"},
		{"1e3 / 3", "333.3333"}, {"1e-3 / 3", "0.0003333"}, {"1 / 100000", "0.0000"}, {"-1 / 100000", "0.0000"}, {"-3 / 50000", "-0.0001"},
		{"12 / 3 / 2", "2.00000000"}, {"0.1+0.2", "0.3"}, {"1.00 + 2.0", "3.00"}, {"1.00-2.0", "-1.00"}, {"1.0 * 2.00", "2.000"},
		{"-5 % 2", "-1"}, {"5 % -2", "1"}, {"5.00 % 2.0", "1.00"}, {"+1.00", "1.00"}, {"-(1.00)", "-1.00"}, {"--1", "1"},
		{"-9223372036854775808", "-9223372036854775808"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			got := mustEval(t, tc.source, nil)
			want, err := NewDecimal(tc.want)
			if err != nil {
				t.Fatal(err)
			}
			if got.Kind != KindNumber || got.Num != want {
				t.Fatalf("got %+v, want %+v", got, want)
			}
		})
	}
}
func TestFormulaComparisonsPrecedenceAndLaziness(t *testing.T) {
	for _, tc := range []struct {
		source string
		want   bool
	}{
		{"1.0 != 1.00", false}, {"true != false", true}, {"null != null", false}, {"null != true", true}, {"null != 0", true}, {`"null" != null`, true},
		{"2+3*4 > 10", true}, {"(2+3)*4>19", true}, {"2+3*4>15", false}, {"10-3-2>4", true}, {"10-3-2>6", false}, {"12/3/2>3", false},
		{"5%2!=0", true}, {"-5%2<0", true}, {"5%-2>0", true}, {"0.1+0.2>=0.3", true}, {"1/100000>0", false}, {"1/3>0.33332", false}, {"1.00/3>0.33332", true},
		{"true?true:false", true}, {"if(false,true,false)", false}, {"null?true:false", false}, {"if(null,true,false)", false},
		{"true ? true : missingFlag", true}, {"if(false,missingFlag,true)", true}, {"true?true:1/0>1", true}, {"true ? false : true ? false : true", false},
		{"1 < 2 != 3 > 4", true}, {"true ? (false ? false : true) : false", true},
	} {
		t.Run(tc.source, func(t *testing.T) {
			got := mustEval(t, tc.source, nil)
			if got.Kind != KindBool || got.Bool != tc.want {
				t.Fatalf("got %+v, want %v", got, tc.want)
			}
		})
	}
	for _, op := range []string{">", "<", ">=", "<=", "!="} {
		for _, amount := range []int64{19999, 20000, 25000} {
			got := mustEval(t, "loanAmount "+op+" 20000", mapResolver{"loanAmount": {Kind: KindNumber, Num: Decimal{Coefficient: amount}}})
			want := false
			switch op {
			case ">":
				want = amount > 20000
			case "<":
				want = amount < 20000
			case ">=":
				want = amount >= 20000
			case "<=":
				want = amount <= 20000
			case "!=":
				want = amount != 20000
			}
			if got.Bool != want {
				t.Fatalf("%d %s 20000 = %v", amount, op, got)
			}
		}
	}
	for _, v := range []Value{{Kind: KindNull}, {Kind: KindString}, {Kind: KindString, Str: "a"}, {Kind: KindBool}, {Kind: KindNumber}} {
		got := mustEval(t, "customer.middleName != null", mapResolver{"customer.middleName": v})
		if got.Bool != (v.Kind != KindNull) {
			t.Fatalf("null comparison: %+v", got)
		}
	}
}
func TestFormulaStaticBothBranchesAndConsumerKinds(t *testing.T) {
	for _, src := range []string{`true != 1`, `false < true`, `"x" > 1`, `"x" != 1`, `true+1`, `null*2`, `-false`, `false?upper(1):"ok"`, `if(false,upper(1),"ok")`, `sum(true)`, `count(null)`, `formatNumber(1,false)`, `sum(count(items))`, `(flag?1:"x")!=1`, `1!=(flag?1:"x")`, `if(flag,1,"x")!=1`, `false?bad():true`} {
		e, err := Parse(src)
		if err != nil {
			t.Fatalf("syntax %q: %v", src, err)
		}
		err = Check(e)
		var located *LocatedError
		if !errors.As(err, &located) {
			t.Fatalf("%q must have located static error, got %v", src, err)
		}
	}
	for _, src := range []string{`loanAmount+fee`, `if(flag,1,fallback)`, `flag?1:fallback`, `"true"`, `"false"`, `"null"`, `true`, `false`, `null`, `(true)`} {
		e, err := Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		err = CheckCondition(e)
		valid := src == "true" || src == "false" || src == "null" || src == "(true)"
		if (err == nil) != valid {
			t.Fatalf("CheckCondition(%s)=%v", src, err)
		}
	}
	for _, src := range []string{`true ? "Yes" : "No"`, `null`, `true`, `false`, `1`, `count(items)`, `a + b`, `flag ? 1 : "x"`, `flag ? 1 : false`} {
		e, _ := Parse(src)
		err := CheckText(e)
		valid := src != `true` && src != `false` && src != `flag ? 1 : false`
		if (err == nil) != valid {
			t.Fatalf("CheckText(%s)=%v", src, err)
		}
	}
	for _, src := range []string{`sum((items.amount))`, `formatNumber((1),("0"))`} {
		e, err := Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		if err = Check(e); err != nil {
			t.Fatalf("group-transparent constraints: %v", err)
		}
	}
}
func TestFormulaSyntaxAndLocations(t *testing.T) {
	for _, src := range []string{"true.field", "true()", "false.x", "null()", "1<2<3", "1!=2!=3", "a == b", "a === b", "a !== b", "a && b", "a || b", "!a", "x?y", "loanAmount >"} {
		if _, err := Parse(src); err == nil {
			t.Fatalf("accepted %s", src)
		}
	}
	for _, src := range []string{"trueFlag", "falseValue", "nullable", "True", "record.true", "params.null"} {
		e, err := Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := e.(*PathExpr); !ok {
			t.Fatalf("%s is %T", src, e)
		}
	}
	e, err := Parse("  true ? false : upper(1)")
	if err != nil {
		t.Fatal(err)
	}
	var located *LocatedError
	if err = Check(e); !errors.As(err, &located) || located.Offset != 23 {
		t.Fatalf("offset=%+v err=%v", located, err)
	}
	_, err = Parse(`"ไทย" != `)
	if !errors.As(err, &located) || located.Offset != len(`"ไทย" != `) {
		t.Fatalf("UTF-8 source offset: %+v", located)
	}
}
func TestFormulaArithmeticFailuresAreLocated(t *testing.T) {
	for _, src := range []string{"1/0", "1%0", "9223372036854775807+1", "-(-9223372036854775808)", "1000000000000000/1", "1e100000*1e1", "1e-100000/1"} {
		e, err := Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = Eval(e, nil, testFC(), "formula")
		var located *LocatedError
		if !errors.As(err, &located) {
			t.Fatalf("%s: expected located failure, got %v", src, err)
		}
	}
	e, _ := Parse("a+1")
	_, _, err := Eval(e, mapResolver{"a": {Kind: KindNumber, Num: Decimal{Coefficient: 1, Exponent: math.MaxInt}}}, testFC(), "formula")
	if err == nil {
		t.Fatal("invalid resolver Decimal accepted")
	}
	e, _ = Parse("a/b")
	v, _, err := Eval(e, mapResolver{"a": {Kind: KindNumber, Num: Decimal{Exponent: 100000}}, "b": {Kind: KindNumber, Num: Decimal{Coefficient: 1, Exponent: -99996}}}, testFC(), "formula")
	if err != nil || v.Num != (Decimal{Exponent: -100000}) {
		t.Fatalf("valid wide division shift: %+v %v", v, err)
	}
}
func TestFormulaResourceLimits(t *testing.T) {
	for _, src := range []string{strings.Repeat("x", maxSourceBytes+1), strings.Repeat("(", 65) + "true" + strings.Repeat(")", 65), strings.Repeat("+", 65) + "1"} {
		if _, err := Parse(src); err == nil {
			t.Fatal("unbounded parse accepted")
		}
	}
	if _, err := Parse(strings.Repeat("x", maxSourceBytes)); err != nil {
		t.Fatal(err)
	}
	var tree func(int) Expr
	tree = func(depth int) Expr {
		if depth == 0 {
			return &NumberLit{Literal: "1"}
		}
		return &BinaryExpr{Op: "+", Left: tree(depth - 1), Right: tree(depth - 1)}
	}
	if err := Check(tree(11)); err != nil {
		t.Fatalf("4095 nodes: %v", err)
	}
	if err := Check(&UnaryExpr{Op: "+", Operand: tree(11)}); err != nil {
		t.Fatalf("4096 nodes rejected: %v", err)
	}
	if err := Check(&UnaryExpr{Op: "+", Operand: &UnaryExpr{Op: "+", Operand: tree(11)}}); err == nil {
		t.Fatal("4097 nodes accepted")
	}
	if _, err := Parse(strings.Repeat("(", 63) + "true" + strings.Repeat(")", 63)); err != nil {
		t.Fatalf("depth 64 rejected: %v", err)
	}
	if err := Check(tree(12)); err == nil {
		t.Fatal("8191 nodes accepted")
	}
	cycle := &GroupExpr{}
	cycle.Inner = cycle
	for _, e := range []Expr{nil, (*CallExpr)(nil), cycle, &UnaryExpr{Op: "-"}, &CallExpr{Name: "if", Args: make([]Expr, 100000)}, &StringLit{Value: strings.Repeat("s", maxSourceBytes+1)}, &NumberLit{Literal: strings.Repeat("0", maxSourceBytes+1)}, &PathExpr{Segments: []string{strings.Repeat("x", maxSourceBytes+1)}}} {
		if _, _, err := Eval(e, nil, testFC(), "formula"); err == nil {
			t.Fatalf("malformed tree accepted: %T", e)
		}
	}
	e, _ := Parse("upper(lower(upper(x)))")
	_, _, err := Eval(e, mapResolver{"x": {Kind: KindString, Str: strings.Repeat("a", 200000)}}, testFC(), "formula")
	if err == nil || !strings.Contains(err.Error(), "work units") {
		t.Fatalf("shared string budget: %v", err)
	}
	budget := NewBudget()
	if err := budget.Charge(maxEvaluationWork); err != nil {
		t.Fatal(err)
	}
	if err := budget.Charge(1); err == nil {
		t.Fatal("budget overrun accepted")
	}
}

func TestFormulaBoundaryWhitespaceKeepsSourceOffsets(t *testing.T) {
	for _, boundary := range []string{"\n", "\r\n", "\u00a0", "\n\u00a0\t"} {
		source := boundary + "true" + boundary
		e, err := Parse(source)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		if location(e) != len(boundary) {
			t.Fatalf("offset %d, want %d", location(e), len(boundary))
		}
		v, _, err := Eval(e, nil, testFC(), "e1")
		if err != nil || !v.Bool {
			t.Fatalf("boundary value: %+v %v", v, err)
		}
		_, err = Parse(boundary + "flag >" + boundary)
		var located *LocatedError
		if !errors.As(err, &located) || located.Offset != 2*len(boundary)+len("flag >") {
			t.Fatalf("boundary error offset: %v", err)
		}
	}
}
func TestFormulaUnsupportedValueWrapperReturnsLocatedError(t *testing.T) {
	wrapper := struct{ Expr }{Expr: &BoolLit{Value: true}}
	for _, e := range []Expr{wrapper, &wrapper} {
		_, _, err := Eval(e, nil, testFC(), "e1")
		var located *LocatedError
		if !errors.As(err, &located) || !strings.Contains(err.Error(), "unsupported node") {
			t.Fatalf("wrapper: %v", err)
		}
	}
}
