package expr

import (
	"fmt"
	"strings"
)

type kindSet uint8

const (
	nullKind    kindSet = 1 << KindNull
	stringKind  kindSet = 1 << KindString
	numberKind  kindSet = 1 << KindNumber
	boolKind    kindSet = 1 << KindBool
	unknownKind kindSet = 1 << 4
)

// Check validates both branches without reading data.
func Check(e Expr) error {
	if err := preflight(e); err != nil {
		return err
	}
	return check(e)
}
func check(e Expr) error {
	var err error
	switch n := e.(type) {
	case *PathExpr:
		if len(n.Segments) == 0 {
			err = fmt.Errorf("empty data path")
		}
	case *NumberLit:
		p := parser{src: n.Literal}
		var tok token
		tok, err = p.lexNumber()
		if err == nil && tok.end != len(n.Literal) {
			err = fmt.Errorf("invalid number literal %q", n.Literal)
		}
		if err == nil {
			_, err = NewDecimal(n.Literal)
		}
	case *StringLit, *BoolLit, *NullLit:
	case *CallExpr:
		return checkCall(n)
	case *GroupExpr:
		return check(n.Inner)
	case *UnaryExpr:
		if n.Op != "+" && n.Op != "-" {
			err = fmt.Errorf("unknown unary operator %q", n.Op)
		} else {
			err = requireKind(n.Operand, numberKind, "unary "+n.Op+" operand must be a number")
		}
	case *BinaryExpr:
		switch n.Op {
		case "+", "-", "*", "/", "%", ">", "<", ">=", "<=":
			if err = requireKind(n.Left, numberKind, n.Op+" operand must be a number"); err == nil {
				err = requireKind(n.Right, numberKind, n.Op+" operand must be a number")
			}
		case "!=":
			err = checkInequality(n.Left, n.Right)
		default:
			err = fmt.Errorf("unknown binary operator %q", n.Op)
		}
	case *ConditionalExpr:
		err = requireKind(n.Condition, boolKind|nullKind, "conditional condition must be a boolean or null")
	default:
		err = fmt.Errorf("unrecognised expression node %T", e)
	}
	if err != nil {
		return at(location(e), err)
	}
	for _, child := range Children(e) {
		if err := check(child); err != nil {
			return err
		}
	}
	return nil
}

// CheckCondition and CheckText preserve each consumer's strict output rules,
// checking each statically known branch even if it would not be selected.
// Text admits a number (owner decision 2026-09-13): it prints as its exact
// decimal (Decimal.Text). Booleans stay refused in text.
func CheckCondition(e Expr) error {
	if err := Check(e); err != nil {
		return err
	}
	return requireKind(e, boolKind|nullKind, "condition must be a boolean or null (no truthiness)")
}
func CheckText(e Expr) error {
	if err := Check(e); err != nil {
		return err
	}
	return requireKind(e, stringKind|numberKind|nullKind, "text expression must return a string, a number or null (a boolean is never coerced)")
}
func requireKind(e Expr, allowed kindSet, label string) error {
	switch n := Ungroup(e).(type) {
	case *ConditionalExpr:
		if err := requireKind(n.Then, allowed, label); err != nil {
			return err
		}
		return requireKind(n.Else, allowed, label)
	case *CallExpr:
		if n.Name == "if" && len(n.Args) == 3 {
			if err := requireKind(n.Args[1], allowed, label); err != nil {
				return err
			}
			return requireKind(n.Args[2], allowed, label)
		}
	}
	kinds := possibleKinds(e)
	if kinds&unknownKind == 0 && kinds&allowed == 0 {
		return at(location(e), fmt.Errorf("%s: %s", label, e.Text()))
	}
	return nil
}
func possibleKinds(e Expr) kindSet {
	switch n := e.(type) {
	case *StringLit:
		return stringKind
	case *NumberLit, *UnaryExpr:
		return numberKind
	case *BoolLit:
		return boolKind
	case *NullLit:
		return nullKind
	case *GroupExpr:
		return possibleKinds(n.Inner)
	case *BinaryExpr:
		if n.Op == "!=" || n.Op == ">" || n.Op == "<" || n.Op == ">=" || n.Op == "<=" {
			return boolKind
		}
		return numberKind
	case *ConditionalExpr:
		return possibleKinds(n.Then) | possibleKinds(n.Else)
	case *CallExpr:
		if n.Name == "if" && len(n.Args) == 3 {
			return possibleKinds(n.Args[1]) | possibleKinds(n.Args[2])
		}
		entry, ok := lookupFunc(n.Name)
		if !ok {
			return unknownKind
		}
		switch entry.ret.(type) {
		case returnDecimal:
			if n.Name == "avg" {
				return numberKind | nullKind
			}
			return numberKind
		case returnString:

			return stringKind
		}
	}
	return unknownKind
}

// KnownKinds returns conservative scalar possibilities; nil means data-dependent.
func KnownKinds(e Expr) []Kind {
	mask := possibleKinds(e)
	if mask&unknownKind != 0 {
		return nil
	}
	var out []Kind
	for _, kind := range []Kind{KindNull, KindString, KindNumber, KindBool} {
		if mask&(1<<kind) != 0 {
			out = append(out, kind)
		}
	}
	return out
}
func checkCall(call *CallExpr) error {
	entry, ok := lookupFunc(call.Name)
	if !ok {
		return at(call.Offset, fmt.Errorf("unknown function %q — the eight legal names are %s: %s", call.Name, strings.Join(LegalFunctionNames(), ", "), call.Raw))
	}
	if len(call.Args) != entry.arity {
		return at(call.Offset, fmt.Errorf("%s() takes %d argument(s), got %d: %s", entry.name, entry.arity, len(call.Args), call.Raw))
	}
	for i, arg := range call.Args {
		if err := check(arg); err != nil {
			return err
		}
		if err := checkArgKind(entry, i, arg, call.Raw); err != nil {
			return at(location(arg), err)
		}
	}
	switch call.Name {
	case "formatDate":
		lit := Ungroup(call.Args[1]).(*StringLit)
		if _, err := parseDatePattern(lit.Value); err != nil {
			return at(location(call.Args[1]), err)
		}
	case "formatNumber":
		lit := Ungroup(call.Args[1]).(*StringLit)
		if _, err := validateNumberPattern(lit.Value); err != nil {
			return at(location(call.Args[1]), err)
		}
	}
	return nil
}
func IsLiteralExpr(e Expr) bool {
	switch Ungroup(e).(type) {
	case *StringLit, *NumberLit, *BoolLit, *NullLit:
		return true
	}
	return false
}
func checkArgKind(entry funcEntry, index int, arg Expr, raw string) error {
	switch entry.args[index] {
	case argAny:
		return nil
	case argNotLiteral:
		if _, ok := Ungroup(arg).(*PathExpr); !ok {
			return fmt.Errorf("%s(): argument %d must be a data path naming a collection, got %s: %s", entry.name, index+1, arg.Text(), raw)
		}
	case argStringLiteral:
		if _, ok := Ungroup(arg).(*StringLit); !ok {
			return fmt.Errorf("%s(): argument %d must be a string literal pattern, got %s: %s", entry.name, index+1, arg.Text(), raw)
		}
	case argCondition:
		return requireKind(arg, boolKind|nullKind, entry.name+"() condition must be a boolean or null")
	case argNumber:
		return requireKind(arg, numberKind, entry.name+"() operand must be a number")
	case argString:
		return requireKind(arg, stringKind, entry.name+"() operand must be a string")
	case argInstant:
		return requireKind(arg, stringKind|numberKind, entry.name+"() operand must be a string or number")
	}
	return nil
}

func checkInequality(a, b Expr) error {
	branches := func(e Expr) []Expr {
		switch n := Ungroup(e).(type) {
		case *ConditionalExpr:
			return []Expr{n.Then, n.Else}
		case *CallExpr:
			if n.Name == "if" && len(n.Args) == 3 {
				return n.Args[1:]
			}
		}
		return nil
	}
	if choices := branches(a); choices != nil {
		for _, choice := range choices {
			if err := checkInequality(choice, b); err != nil {
				return err
			}
		}
		return nil
	}
	if choices := branches(b); choices != nil {
		for _, choice := range choices {
			if err := checkInequality(a, choice); err != nil {
				return err
			}
		}
		return nil
	}
	x, y := possibleKinds(a), possibleKinds(b)
	if x&unknownKind == 0 && y&unknownKind == 0 && x&nullKind == 0 && y&nullKind == 0 && x&y == 0 {
		return at(location(a), fmt.Errorf("!= operands must have the same scalar kind, or null"))
	}
	return nil
}
