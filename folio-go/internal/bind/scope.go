package bind

// Scope carries every resolution root reachable while resolving one
// placeholder (AC0): DATA and PARAMS are always present; ROW is
// optional, set only while resolving inside a repeating region's row
// scope (AD-11, Story 3.1). BindText/BindTextSpans (text.go) construct
// a Scope with no row set — the existing, pre-3.1 behaviour, byte-
// identical (AC0/AC7). A future story that generates rows (4.2) calls
// WithRow to activate one.
//
// The row's ALIAS (the author's "as" spelling, or the literal "row"
// when the region omits it, AC2) is carried on the Scope as ordinary
// data used for dispatch and for error text — never as a lookupBound
// rootKind. lookupBound's rootKind parameter is a named struct type
// (text.go), not a string, so there is no code path by which the
// author's own alias spelling could be passed there even as an
// argument type error, let alone smuggled past as a literal (Story 3.3
// finisher pass, Finding 1). D-3.1.1 records this: the root-CLASS name
// is always kindRow, whose own name field is the literal "row".
type Scope struct {
	data, params Value

	rowSet   bool
	row      Value
	rowAlias string
}

// NewScope builds a Scope with no row active — the "wrapper" scope
// AC0 requires BindText/BindTextSpans to construct.
func NewScope(data, params Value) Scope {
	return Scope{data: data, params: params}
}

// WithRow returns a copy of s with a row scope active, addressed by
// alias (the region's declared "as", or "row" when omitted — callers
// apply that default, not this function). Selection inside the
// resolver is by first-path-segment equality against alias, evaluated
// after params and before the data root (D-3.1.1).
func (s Scope) WithRow(row Value, alias string) Scope {
	s.rowSet = true
	s.row = row
	s.rowAlias = alias
	return s
}
