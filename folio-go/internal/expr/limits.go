package expr

import (
	"errors"
	"fmt"
	"reflect"
)

// LocatedError preserves the typed cause and the source-relative UTF-8 byte offset.
type LocatedError struct {
	Offset int
	Err    error
}

func (e *LocatedError) Error() string { return fmt.Sprintf("at position %d: %s", e.Offset, e.Err) }
func (e *LocatedError) Unwrap() error { return e.Err }
func at(offset int, err error) error {
	if err == nil {
		return nil
	}
	var located *LocatedError
	if errors.As(err, &located) {
		return err
	}
	return &LocatedError{Offset: offset, Err: err}
}
func location(e Expr) int {
	switch n := e.(type) {
	case *PathExpr:
		return n.Offset
	case *StringLit:
		return n.Offset
	case *NumberLit:
		return n.Offset
	case *BoolLit:
		return n.Offset
	case *NullLit:
		return n.Offset
	case *CallExpr:
		return n.Offset
	case *GroupExpr:
		return n.Offset
	case *UnaryExpr:
		return n.Offset
	case *BinaryExpr:
		return n.Offset
	case *ConditionalExpr:
		return n.Offset
	default:
		return 0
	}
}

// preflight is iterative so malformed or cyclic caller-built trees never reach
// recursive checking/evaluation. Shared subtrees count once per occurrence.
func preflight(root Expr) error {
	type visit struct {
		node  Expr
		depth int
		leave bool
	}
	stack := []visit{{node: root, depth: 1}}
	active := make(map[Expr]bool)
	nodes := 0
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch v.node.(type) {
		case *PathExpr, *StringLit, *NumberLit, *BoolLit, *NullLit, *CallExpr, *GroupExpr, *UnaryExpr, *BinaryExpr, *ConditionalExpr:
		default:
			return at(0, fmt.Errorf("malformed expression: unsupported node %T", v.node))
		}
		if reflect.ValueOf(v.node).IsNil() {
			return at(0, fmt.Errorf("malformed expression: nil node"))
		}
		if v.leave {
			delete(active, v.node)
			continue
		}
		if active[v.node] {
			return at(location(v.node), fmt.Errorf("cyclic expression tree"))
		}
		nodes++
		if nodes > maxASTNodes || v.depth > maxCallDepth {
			return at(location(v.node), fmt.Errorf("expression exceeds 4096 nodes or depth 64"))
		}
		if len(v.node.Text()) > maxSourceBytes {
			return at(location(v.node), fmt.Errorf("expression exceeds %d source bytes", maxSourceBytes))
		}
		switch n := v.node.(type) {
		case *NumberLit:
			if len(n.Literal) > maxSourceBytes {
				return at(n.Offset, fmt.Errorf("number literal exceeds source bound"))
			}
		case *StringLit:
			if len(n.Value) > maxSourceBytes {
				return at(n.Offset, fmt.Errorf("string literal exceeds source bound"))
			}
		case *PathExpr:
			if len(n.Segments) > maxSourceBytes {
				return at(n.Offset, fmt.Errorf("path exceeds source bound"))
			}
			size := 0
			for _, segment := range n.Segments {
				if len(segment) > maxSourceBytes-size {
					return at(n.Offset, fmt.Errorf("path exceeds source bound"))
				}
				size += len(segment) + 1
			}
		case *CallExpr:
			if len(n.Name) > maxSourceBytes {
				return at(n.Offset, fmt.Errorf("function name exceeds source bound"))
			}
		}
		active[v.node] = true
		stack = append(stack, visit{node: v.node, leave: true})
		children := Children(v.node)
		if len(children) > maxASTNodes-nodes {
			return at(location(v.node), fmt.Errorf("expression exceeds 4096 nodes"))
		}
		for _, child := range children {
			stack = append(stack, visit{node: child, depth: v.depth + 1})
		}
	}
	return nil
}

const maxEvaluationWork = 1_000_000

// Budget is shared by an expression's recursion and its concrete data resolver.
// Third-party resolvers remain responsible for bounding their own external work.
type Budget struct{ remaining int }

func NewBudget() *Budget { return &Budget{remaining: maxEvaluationWork} }
func (b *Budget) Charge(units int) error {
	if b == nil {
		return nil
	}
	if units < 0 || units > b.remaining {
		return fmt.Errorf("expression evaluation exceeds %d work units", maxEvaluationWork)
	}
	b.remaining -= units
	return nil
}

type budgetResolver struct {
	Resolver
	budget *Budget
}

func expressionBudget(r Resolver) *Budget {
	if b, ok := r.(*budgetResolver); ok {
		return b.budget
	}
	return nil
}

// EvalWithBudget shares the caller's budget without widening the closed Resolver interface.
func EvalWithBudget(e Expr, r Resolver, fc FormatContext, id string, budget *Budget) (Value, []Caveat, error) {
	if budget == nil {
		budget = NewBudget()
	}
	if err := Check(e); err != nil {
		return Value{}, nil, err
	}
	return Eval(e, &budgetResolver{Resolver: r, budget: budget}, fc, id)
}

// WithLocation attaches the expression's available source offset to a consumer
// failure without replacing an already located evaluation cause.
func WithLocation(e Expr, err error) error { return at(location(e), err) }
