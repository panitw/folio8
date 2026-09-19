package expr

// This file is FR18/AD-9/C1's closed table: the single package-level
// literal, in one file, of the eight and ONLY eight functions the
// expression language will ever have (AC5). Adding a ninth requires
// editing both this literal and AC7's expected-set guard, in the same
// diff, or CI is red (AC8, C1) — that is the whole point of counting
// them here, in one place, rather than letting each function register
// itself.
//
// AC6: no exported function anywhere in internal/expr adds to,
// mutates, or replaces this table — closed at COMPILE time, not by
// convention. functionTable is unexported, is never returned by
// reference from an exported function, and there is no exported
// "Register" of any kind (internal/expr_arch_test.go asserts this by
// AST, over internal/expr's own exported declarations).

// argKind is a per-argument constraint Check (check.go) can decide
// WITHOUT any data — Decision 3's "literal-argument kind" half, never
// its "path-argument value kind" half (that is explicitly NOT this
// story's obligation; it is owed at evaluation by evalCall's own
// switch, below — FuncEntry no longer carries an OwningStory field,
// see AC16/Arm A's removal note further down this file).
type argKind int

const (
	// argAny places no static constraint on this argument at all.
	argAny argKind = iota
	// argNotLiteral requires a collection path (possibly grouped).
	argNotLiteral
	argCondition
	argNumber
	argString
	argInstant
	// argStringLiteral requires this argument to be, syntactically, a
	// string literal — never a path, a call, or a number literal.
	// formatDate/formatNumber's pattern argument (D-1.4.1) is always a
	// literal pattern, never data-dependent (AC10's
	// `formatNumber(x, 123)` example: the pattern is present but is
	// the WRONG kind of literal).
	argStringLiteral
)

// returnKind marks a table entry's declared RETURN type (AC9: "each of
// the three aggregation entries declares a Decimal-typed signature …
// declaring the table honestly requires referencing Decimal — a
// stringly-typed table is a defect on its own merits"). Each variant
// embeds the real Go type it names, rather than a string label, so the
// table's own declarations are checkable by the Go compiler, not by
// convention: sum/count/avg's entries below literally construct a
// returnDecimal{}, which only type-checks because Decimal (decimal.go)
// exists and is spelled correctly.
type returnKind interface{ isReturnKind() }

type returnDecimal struct{ zero Decimal } // sum, count, avg
type returnString struct{}                // formatDate, formatNumber, upper, lower
type returnAny struct{}                   // if — whichever branch is selected

func (returnDecimal) isReturnKind() {}
func (returnString) isReturnKind()  {}
func (returnAny) isReturnKind()     {}

// FuncEntry is one row of the closed eight-entry table.
//
// Story 3.4/AC16 (RULED ARM A): this struct used to also carry
// `implemented bool` and `owningStory string`, distinguishing a
// computed function from a registered-but-not-yet-implemented one.
// That distinction is REMOVED, not merely retired to false/inert,
// because the table is closed at eight (C1) and this story flips the
// last two entries — so the "unimplemented" population goes to ZERO,
// on schedule, and a mechanism asserting "is this entry implemented"
// would from that point on assert a proposition that can never again
// be false (D-000.9 with the polarity inverted: a guard that cannot
// fail, reported as coverage). Every one of the eight entries below
// now computes; TestImplementedEntriesMatchEvalCallSwitch
// (table_derivational_test.go) and the behavioural witness in
// table_behavioral_test.go assert the OBLIGATION that they do,
// derivationally, rather than re-reading a flag this story just set.
type funcEntry struct {
	name  string
	arity int
	args  []argKind // len(args) == arity
	ret   returnKind
}

// functionTable is FR18's closed eight, keyed by name (AC5). Ordering
// here is source order, not significant to any guard.
var functionTable = [8]funcEntry{
	{name: "sum", arity: 1, args: []argKind{argNotLiteral}, ret: returnDecimal{}},
	{name: "count", arity: 1, args: []argKind{argNotLiteral}, ret: returnDecimal{}},
	{name: "avg", arity: 1, args: []argKind{argNotLiteral}, ret: returnDecimal{}},
	{name: "formatDate", arity: 2, args: []argKind{argInstant, argStringLiteral}, ret: returnString{}},
	{name: "formatNumber", arity: 2, args: []argKind{argNumber, argStringLiteral}, ret: returnString{}},
	{name: "upper", arity: 1, args: []argKind{argString}, ret: returnString{}},
	{name: "lower", arity: 1, args: []argKind{argString}, ret: returnString{}},
	{name: "if", arity: 3, args: []argKind{argCondition, argAny, argAny}, ret: returnAny{}},
}

// LegalFunctionNames returns the eight legal names, in table order —
// used only to compose AC11's "the eight legal names" error text, so
// that text can never drift from the table it names (AD-13).
func LegalFunctionNames() []string {
	names := make([]string, 0, len(functionTable))
	for _, e := range functionTable {
		names = append(names, e.name)
	}
	return names
}

// lookupFunc finds name's table entry.
func lookupFunc(name string) (funcEntry, bool) {
	for _, e := range functionTable {
		if e.name == name {
			return e, true
		}
	}
	return funcEntry{}, false
}
