package expr

// Expr is one node of a parsed expression tree (AC1, AC3): a bare
// dotted path, a function call, a string literal, or a number literal.
// It carries no evaluation logic of its own — Eval (eval.go) walks the
// tree and Check (check.go) validates it statically; a node's only
// job is to remember what the author wrote and where.
type Expr interface {
	// Text returns the exact source text this node was parsed from
	// (AC19's "the offending expression text, verbatim, as the author
	// wrote it"; AC15's "the offending expression text").
	Text() string

	exprNode()
}

// PathExpr is a bare dotted path (AC3): "ident ( '.' ident )*",
// preserving 1.6's grammar verbatim (D-1.6.5). Segments is the path
// split on ".", in order.
type PathExpr struct {
	Offset   int // source-relative UTF-8 byte offset
	Segments []string
	Raw      string
}

func (p *PathExpr) Text() string { return p.Raw }
func (*PathExpr) exprNode()      {}

// StringLit is a double-quoted string literal (AC3). Value is the
// literal's content with the surrounding quotes removed; Raw is the
// exact source text including the quotes.
type StringLit struct {
	Offset int // source-relative UTF-8 byte offset
	Value  string
	Raw    string
}

func (s *StringLit) Text() string { return s.Raw }
func (*StringLit) exprNode()      {}

// NumberLit is a JSON number literal (AC3): sign, digits, optional
// fraction, optional exponent — the same shape
// internal/template.SplitJSONNumber accepts, since NewDecimal is what
// eventually consumes it (eval.go). Literal is the literal exactly as
// written; Raw is identical to Literal (a number literal is never
// wrapped in anything else).
type NumberLit struct {
	Offset  int // source-relative UTF-8 byte offset
	Literal string
	Raw     string
}

func (n *NumberLit) Text() string { return n.Raw }
func (*NumberLit) exprNode()      {}

// CallExpr is a function call over comma-separated arguments (AC3),
// nesting to any depth (AC3's minimum is one level;
// formatNumber(sum(t.amount), "#,##0.00") parses). Name is the bare
// function identifier as written — Check (check.go) is what resolves
// it against the closed eight-entry table (table.go); Parse itself
// has no notion of which names are legal, so the grammar and the
// closed set stay two separably-testable properties (AC1 vs AC5-AC11).
type CallExpr struct {
	Offset int // source-relative UTF-8 byte offset
	Name   string
	Args   []Expr
	Raw    string
}

func (c *CallExpr) Text() string { return c.Raw }
func (*CallExpr) exprNode()      {}

// Formula nodes preserve grouping syntax for footer derivation and byte locations.
type BoolLit struct {
	Value  bool
	Raw    string
	Offset int
}

func (n *BoolLit) Text() string { return n.Raw }
func (*BoolLit) exprNode()      {}

type NullLit struct {
	Raw    string
	Offset int
}

func (n *NullLit) Text() string { return n.Raw }
func (*NullLit) exprNode()      {}

type GroupExpr struct {
	Inner  Expr
	Raw    string
	Offset int
}

func (n *GroupExpr) Text() string { return n.Raw }
func (*GroupExpr) exprNode()      {}

type UnaryExpr struct {
	Op      string
	Operand Expr
	Raw     string
	Offset  int
}

func (n *UnaryExpr) Text() string { return n.Raw }
func (*UnaryExpr) exprNode()      {}

type BinaryExpr struct {
	Op          string
	Left, Right Expr
	Raw         string
	Offset      int
}

func (n *BinaryExpr) Text() string { return n.Raw }
func (*BinaryExpr) exprNode()      {}

type ConditionalExpr struct {
	Condition, Then, Else Expr
	Raw                   string
	Offset                int
}

func (n *ConditionalExpr) Text() string { return n.Raw }
func (*ConditionalExpr) exprNode()      {}

// Children enumerates every child in authored order. Callers must preflight
// caller-built trees with Check before recursively walking them.
func Children(e Expr) []Expr {
	switch n := e.(type) {
	case *CallExpr:
		return n.Args
	case *GroupExpr:
		return []Expr{n.Inner}
	case *UnaryExpr:
		return []Expr{n.Operand}
	case *BinaryExpr:
		return []Expr{n.Left, n.Right}
	case *ConditionalExpr:
		return []Expr{n.Condition, n.Then, n.Else}
	default:
		return nil
	}
}

// Ungroup makes parentheses transparent to function argument constraints.
func Ungroup(e Expr) Expr {
	for {
		n, ok := e.(*GroupExpr)
		if !ok {
			return e
		}
		e = n.Inner
	}
}

// UsesFormulas reports the AST-derived unreleased 2.0 expression requirement.
func UsesFormulas(e Expr) bool {
	switch e.(type) {
	case *BoolLit, *NullLit, *GroupExpr, *UnaryExpr, *BinaryExpr, *ConditionalExpr:
		return true
	}
	for _, child := range Children(e) {
		if UsesFormulas(child) {
			return true
		}
	}
	return false
}

// Kind is the discriminant of a resolved Value (evaluation-time
// result), and separately of a Presence-carrying lookup outcome.
type Kind int

const (
	KindNull Kind = iota
	KindString
	KindNumber
	KindBool
)

func (k Kind) String() string {
	switch k {
	case KindNull:
		return "null"
	case KindString:
		return "string"
	case KindNumber:
		return "number"
	case KindBool:
		return "bool"
	default:
		return "unknown"
	}
}

// Value is an expression's evaluated result: a scalar, deliberately —
// no function in the closed eight-entry table produces or consumes a
// collection value directly; sum/count/avg (Story 3.3) reduce a
// collection to a scalar, and CollectionLength/ProjectCollection below
// are the seam that reaches one, never Value itself. AD-14's null case
// is a Kind, not a separate sentinel: an
// explicit JSON null is a legal Value (KindNull), never itself an
// evaluation error — only an ABSENT path is (see Resolver).
type Value struct {
	Kind Kind
	Str  string
	Num  Decimal
	Bool bool
}

// Resolver looks up a dotted path against whatever data tree the
// caller owns (evaluation time only — Parse/Check never resolve a
// path against data). internal/expr never sees internal/bind's own
// Value tree: bind (stage rank 4) imports expr (rank 3) to reuse this
// interface, never the reverse (D-1.6.1, F2) — so Resolver is exactly
// the seam a caller's own data model crosses through, defined here at
// the lower rank and implemented at the higher one.
//
// AD-14's three cases are split across Resolve's two return values,
// deliberately asymmetric: an ABSENT path is reported as a non-nil
// err — already the caller's own fully-worded, located error (e.g.
// bind's "data path %q is absent from the report data"), which Eval
// propagates completely unchanged, because only the resolver's own
// root (data/params/row, in bind's case) knows how to word it. An
// explicit JSON NULL is not an error at all: it is the ordinary
// Value{Kind: KindNull}, err == nil — the owner's if() ruling (D-000.x,
// this story) depends on that: a null CONDITION is a legal value that
// evaluates to the else branch, silently, and can only do that if
// resolving it did not already fail.
// CollectionLength and ProjectCollection (Story 3.3, R1) are the
// COLLECTION seam, split from Resolve into two methods rather than
// widening Value with an array kind: Value stays scalar, deliberately
// (its own doc comment above), and a collection value never crosses
// this interface at all — only an int (a count) or a []Value (one
// scalar per element) ever does.
//
// Neither method takes a range, offset, index or limit (R1): every
// call is over the WHOLE collection a path names, never a page or a
// window of it — the mechanism AD-11's "never over the rows on the
// current page" is built on (R3).
//
// path is root-relative in the same sense Resolve's path is — resolved
// against whichever root (params/row/data) the FIRST segment selects —
// except that AD-11 forces an aggregate to bypass the row root even
// when the row scope's own declared alias equals path's first segment
// (R3): the only root a collection ever resolves through, once row is
// excluded, is params or the document root, never a narrower row scope.
type Resolver interface {
	Resolve(path []string) (Value, error)

	// CollectionLength reports the number of elements in the
	// collection path names (AC1, AC3): count()'s only operation.
	// count is STRUCTURALLY unable to reach a projected value (AC3) —
	// this method's signature has no way to return one.
	CollectionLength(path []string) (int, error)

	// ProjectCollection returns exactly one Value per element of the
	// collection path's array prefix names, in DATA ORDER — never
	// sorted, deduplicated or filtered (AC2) — projecting the
	// remaining segments of path against each element in turn. sum()
	// and avg() are its only two callers.
	ProjectCollection(path []string) ([]Value, error)
}
