package expr

import (
	"fmt"
	"math/big"
)

func validateDecimal(d Decimal) error {
	if d.Exponent > maxDecimalExponentMagnitude || d.Exponent < -maxDecimalExponentMagnitude {
		return fmt.Errorf("decimal exponent %d exceeds magnitude bound %d", d.Exponent, maxDecimalExponentMagnitude)
	}
	return nil
}
func decimalResult(c *big.Int, e int) (Decimal, error) {
	if err := validateDecimal(Decimal{Exponent: e}); err != nil {
		return Decimal{}, err
	}
	if !c.IsInt64() {
		return Decimal{}, fmt.Errorf("arithmetic coefficient does not fit int64")
	}
	return Decimal{Coefficient: c.Int64(), Exponent: e}, nil
}
func shiftedCoefficient(d Decimal, shift int, b *Budget) (*big.Int, error) {
	if err := validateDecimal(d); err != nil {
		return nil, err
	}
	if shift < 0 || shift > 3*maxDecimalExponentMagnitude+4 {
		return nil, fmt.Errorf("decimal shift exceeds bounds")
	}
	if err := b.Charge(shift); err != nil {
		return nil, err
	}
	c := big.NewInt(d.Coefficient)
	if shift > 0 {
		c.Mul(c, tenPow(shift))
	}
	return c, nil
}
func aligned(a, b Decimal, budget *Budget) (*big.Int, *big.Int, int, error) {
	if err := validateDecimal(a); err != nil {
		return nil, nil, 0, err
	}
	if err := validateDecimal(b); err != nil {
		return nil, nil, 0, err
	}
	e := min(a.Exponent, b.Exponent)
	x, err := shiftedCoefficient(a, a.Exponent-e, budget)
	if err != nil {
		return nil, nil, 0, err
	}
	y, err := shiftedCoefficient(b, b.Exponent-e, budget)
	return x, y, e, err
}
func decimalBinary(op string, a, b Decimal, budget *Budget) (Decimal, error) {
	if err := validateDecimal(a); err != nil {
		return Decimal{}, err
	}
	if err := validateDecimal(b); err != nil {
		return Decimal{}, err
	}
	if (op == "/" || op == "%") && b.Coefficient == 0 {
		return Decimal{}, fmt.Errorf("%s by zero", map[string]string{"/": "division", "%": "remainder"}[op])
	}
	switch op {
	case "+", "-", "%":
		x, y, e, err := aligned(a, b, budget)
		if err != nil {
			return Decimal{}, err
		}
		switch op {
		case "+":
			x.Add(x, y)
		case "-":
			x.Sub(x, y)
		case "%":
			x.Rem(x, y)
		}
		return decimalResult(x, e)
	case "*":
		return decimalResult(new(big.Int).Mul(big.NewInt(a.Coefficient), big.NewInt(b.Coefficient)), a.Exponent+b.Exponent)
	case "/":
		scale := max(-a.Exponent, -b.Exponent, 0) + 4
		if scale > maxDecimalExponentMagnitude {
			return Decimal{}, fmt.Errorf("division result exponent exceeds magnitude bound %d", maxDecimalExponentMagnitude)
		}
		shift := a.Exponent - b.Exponent + scale
		x, err := shiftedCoefficient(a, max(shift, 0), budget)
		if err != nil {
			return Decimal{}, err
		}
		y, err := shiftedCoefficient(b, max(-shift, 0), budget)
		if err != nil {
			return Decimal{}, err
		}
		q, r := new(big.Int), new(big.Int)
		q.QuoRem(x, y, r)
		twice := new(big.Int).Lsh(new(big.Int).Abs(r), 1)
		cmp := twice.Cmp(new(big.Int).Abs(y))
		if cmp > 0 || cmp == 0 && q.Bit(0) == 1 {
			// Use operand signs, including when truncation yielded zero.
			if x.Sign()*y.Sign() < 0 {
				q.Sub(q, big.NewInt(1))
			} else {
				q.Add(q, big.NewInt(1))
			}
		}
		return decimalResult(q, -scale)
	}
	return Decimal{}, fmt.Errorf("unknown arithmetic operator %q", op)
}
func evalBinary(n *BinaryExpr, r Resolver, fc FormatContext, id string) (Value, []Caveat, error) {
	a, ac, err := Eval(n.Left, r, fc, id)
	if err != nil {
		return Value{}, nil, err
	}
	b, bc, err := Eval(n.Right, r, fc, id)
	if err != nil {
		return Value{}, nil, err
	}
	caveats := appendCaveats(ac, bc)
	if n.Op == "!=" {
		if a.Kind == KindNull || b.Kind == KindNull {
			return Value{Kind: KindBool, Bool: a.Kind != b.Kind}, caveats, nil
		}
		if a.Kind != b.Kind {
			return Value{}, nil, fmt.Errorf("!= operands must have the same scalar kind, got %s and %s", a.Kind, b.Kind)
		}
		switch a.Kind {
		case KindString:
			return Value{Kind: KindBool, Bool: a.Str != b.Str}, caveats, nil
		case KindBool:
			return Value{Kind: KindBool, Bool: a.Bool != b.Bool}, caveats, nil
		}
	}
	if a.Kind != KindNumber || b.Kind != KindNumber {
		return Value{}, nil, fmt.Errorf("%s operands must be numbers, got %s and %s", n.Op, a.Kind, b.Kind)
	}
	switch n.Op {
	case "!=", ">", "<", ">=", "<=":
		x, y, _, err := aligned(a.Num, b.Num, expressionBudget(r))
		if err != nil {
			return Value{}, nil, err
		}
		cmp := x.Cmp(y)
		var v bool
		switch n.Op {
		case "!=":
			v = cmp != 0
		case ">":
			v = cmp > 0
		case "<":
			v = cmp < 0
		case ">=":
			v = cmp >= 0
		case "<=":
			v = cmp <= 0
		}
		return Value{Kind: KindBool, Bool: v}, caveats, nil
	default:
		d, err := decimalBinary(n.Op, a.Num, b.Num, expressionBudget(r))
		return Value{Kind: KindNumber, Num: d}, caveats, err
	}
}
