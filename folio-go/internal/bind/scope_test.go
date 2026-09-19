package bind

import (
	"strings"
	"testing"
)

// TestScopeRowAliasResolvesCurrentRow is source AC1: a repeating
// region declaring {"bind": "transactions[]", "as": "transaction"} —
// a scope whose current row is one element of that collection resolves
// "{{transaction.<field>}}" to THAT row's value.
//
// D-000.50's subject requirement: the collection must have at least
// two rows with DIFFERENT values for the same field, and the assertion
// is made against BOTH rows — a single-row fixture cannot distinguish
// "resolved the current row" from "resolved the only row".
func TestScopeRowAliasResolvesCurrentRow(t *testing.T) {
	data := mustDecode(t, `{"customer": {"name": "Ada"}}`)
	row1 := mustDecode(t, `{"amount": "10.00", "payee": "Alpha"}`)
	row2 := mustDecode(t, `{"amount": "20.00", "payee": "Beta"}`)

	scope1 := NewScope(data, noParams).WithRow(row1, "transaction")
	got1, _, _, err1 := Resolve("{{transaction.amount}} {{transaction.payee}}", scope1, testFormatContext(), "e2")
	if err1 != nil {
		t.Fatalf("row1: unexpected error: %v", err1)
	}
	if got1 != "10.00 Alpha" {
		t.Fatalf("AC1: row1 must resolve to its own fields, got %q", got1)
	}

	scope2 := NewScope(data, noParams).WithRow(row2, "transaction")
	got2, _, _, err2 := Resolve("{{transaction.amount}} {{transaction.payee}}", scope2, testFormatContext(), "e2")
	if err2 != nil {
		t.Fatalf("row2: unexpected error: %v", err2)
	}
	if got2 != "20.00 Beta" {
		t.Fatalf("AC1: row2 must resolve to its own fields, got %q", got2)
	}
	if got1 == got2 {
		t.Fatalf("AC1: two rows with different field values produced identical output %q — the current row was not actually consulted", got1)
	}
}

// TestScopeRowAliasFieldAbsentIsLocatedError is AC1's AD-14 clause: a
// field absent from the row is an error naming the row path and the
// element id — the same three-case discipline (absent/null/wrong-kind)
// lookupBound already gives every other root, now proved for "row".
func TestScopeRowAliasFieldAbsentIsLocatedError(t *testing.T) {
	data := mustDecode(t, `{}`)
	row := mustDecode(t, `{"amount": "10.00"}`)
	scope := NewScope(data, noParams).WithRow(row, "transaction")

	_, _, _, err := Resolve("{{transaction.payee}}", scope, testFormatContext(), "e2")
	if err == nil {
		t.Fatal("AC1: a field absent from the row must be a located Error")
	}
	if !strings.Contains(err.Error(), "transaction.payee") || !strings.Contains(err.Error(), "e2") {
		t.Errorf("error must name the row path (with the AUTHOR'S alias) and the element id, got: %v", err)
	}
}

// TestScopeRowAliasBareIsLocatedError is Review Finding 3's repair: a
// bare row alias, no dot ("{{transaction}}"), returns an early error —
// text.go's row branch, mirroring AC17's bare-"params" rule
// (text_test.go: TestBindTextParamsBareIsLocatedError) — that shipped
// with no test and no AC of its own. The row branch's bare-alias arm
// is an early return that never reaches lookupBound, the same shape
// Story 2.5's review flagged as able to defeat
// TestBindResolutionRootsAreClosed's closed-set guard (a rootName the
// guard never observes because the call site doesn't exist). It does
// NOT defeat the guard here: the row branch's OTHER arm (len(path) >=
// 2, exercised by TestScopeRowAliasResolvesCurrentRow and friends)
// independently supplies the literal "row" rootName to lookupBound, so
// the guard's [data params row] observation does not depend on this
// bare-alias sub-case ever executing — it is simply unassserted by any
// AC0-AC8 obligation, now closed by this test.
func TestScopeRowAliasBareIsLocatedError(t *testing.T) {
	data := mustDecode(t, `{}`)
	row := mustDecode(t, `{"amount": "10.00"}`)
	scope := NewScope(data, noParams).WithRow(row, "transaction")

	_, _, _, err := Resolve("{{transaction}}", scope, testFormatContext(), "e2")
	if err == nil {
		t.Fatal("a bare row alias must be a located Error")
	}
	if !strings.Contains(err.Error(), "e2") {
		t.Errorf("error must name the element id, got: %v", err)
	}
	if !strings.Contains(err.Error(), "namespace") {
		t.Errorf("error should say the alias is a namespace, not a value, got: %v", err)
	}
}

// TestScopeRowAliasDefaultsToRow is source AC2: a repeating region
// whose "as" is absent defaults its alias to "row" at RESOLUTION time
// — TableExt.As itself stays Presence-absent (parse_bands.go/model.go
// are unchanged by this story; the default is applied by whichever
// caller builds the Scope, never at load).
func TestScopeRowAliasDefaultsToRow(t *testing.T) {
	data := mustDecode(t, `{}`)
	row := mustDecode(t, `{"amount": "10.00"}`)
	scope := NewScope(data, noParams).WithRow(row, "row") // caller applies the AC2 default

	got, _, _, err := Resolve("{{row.amount}}", scope, testFormatContext(), "e2")
	if err != nil {
		t.Fatalf("AC2: unexpected error: %v", err)
	}
	if got != "10.00" {
		t.Fatalf("AC2: the defaulted alias %q must resolve to the row's fields, got %q", "row", got)
	}
}

// TestScopeRowNeverShadowsDocumentRoot is source AC3: when the row and
// the document root BOTH carry a key of the same name with DIFFERENT
// values, an UNQUALIFIED path naming that key returns the document
// ROOT's value — a row never shadows the root. Vacuous unless both
// carry the colliding key (per the story's own warning), so both are
// asserted, plus the nested (two-segment) case.
func TestScopeRowNeverShadowsDocumentRoot(t *testing.T) {
	data := mustDecode(t, `{"customer": "FROM-ROOT", "nested": {"customer": "FROM-ROOT-NESTED"}}`)
	row := mustDecode(t, `{"customer": "FROM-ROW", "nested": {"customer": "FROM-ROW-NESTED"}}`)
	scope := NewScope(data, noParams).WithRow(row, "transaction")

	got, _, _, err := Resolve("{{customer}}", scope, testFormatContext(), "e2")
	if err != nil {
		t.Fatalf("AC3: unexpected error: %v", err)
	}
	if got != "FROM-ROOT" {
		t.Fatalf("AC3: an unqualified colliding key must resolve to the DOCUMENT ROOT's value, got %q (want %q)", got, "FROM-ROOT")
	}

	gotNested, _, _, errNested := Resolve("{{nested.customer}}", scope, testFormatContext(), "e2")
	if errNested != nil {
		t.Fatalf("AC3 nested: unexpected error: %v", errNested)
	}
	if gotNested != "FROM-ROOT-NESTED" {
		t.Fatalf("AC3: a two-segment unqualified path colliding at its FIRST segment must still resolve to the document root, got %q (want %q)", gotNested, "FROM-ROOT-NESTED")
	}
}

// TestScopeParamsUnshadowableByRow is source AC4: params.<path> resolves
// from the parameter namespace and is shadowed by NOTHING, including a
// row — all three of report data, the current row and params carry
// params.reportDate-shaped content with THREE DIFFERENT values, and
// {{params.reportDate}} inside the row scope must return the params
// root's value. The unchanged baseline (no row scope active) is
// asserted too, so the story proves it PRESERVED the property (finding
// 6) rather than reimplemented it.
func TestScopeParamsUnshadowableByRow(t *testing.T) {
	data := mustDecode(t, `{"params": {"reportDate": "FROM-DATA"}}`)
	row := mustDecode(t, `{"params": {"reportDate": "FROM-ROW"}}`)
	params := mustDecode(t, `{"reportDate": "FROM-PARAMS"}`)

	scopeNoRow := NewScope(data, params)
	gotBaseline, _, _, errBaseline := Resolve("{{params.reportDate}}", scopeNoRow, testFormatContext(), "e1")
	if errBaseline != nil {
		t.Fatalf("baseline (no row): unexpected error: %v", errBaseline)
	}
	if gotBaseline != "FROM-PARAMS" {
		t.Fatalf("AC4 baseline: {{params.reportDate}} with no row active must still resolve from params, got %q", gotBaseline)
	}

	scopeWithRow := NewScope(data, params).WithRow(row, "transaction")
	got, _, _, err := Resolve("{{params.reportDate}}", scopeWithRow, testFormatContext(), "e2")
	if err != nil {
		t.Fatalf("AC4: unexpected error: %v", err)
	}
	if got != "FROM-PARAMS" {
		t.Fatalf("AC4: params must be shadowed by NOTHING, including a row; got %q, want the params root's value %q", got, "FROM-PARAMS")
	}
}

// TestScopeParamsUnshadowableEvenByRowAliasedParams is Review Finding
// 2's repair: the row above is aliased "transaction", so
// {{params.reportDate}}'s first path segment ("params") can never
// equal scope.rowAlias under ANY dispatch order — the row's
// FROM-ROW-shaped value in that fixture is unreachable by
// construction, so that test discriminates only params-vs-DATA (the
// pre-3.1 behaviour), never params-vs-ROW (the clause this story
// added). The one fixture that CAN discriminate is a row whose own
// alias IS "params" — the reviewer's own probe. The story originally
// forbade constructing this case while OD-1 (whether "as": "params" is
// even legal) was unruled; D-3.1.1 has since ruled it (Arm B, a
// located template error), but that prohibition is enforced by
// render.go's checkTableBindings, a layer ABOVE package bind — inside
// package bind itself, Scope.WithRow accepts any alias string, so this
// test constructs the case directly at bind level (bypassing the
// render-level ban) to prove what actually protects "params" here:
// dispatch ORDER, not the alias ban. The params branch in Resolve is
// checked before the row branch (D-3.1.1), so even a row aliased
// "params" cannot shadow the params root.
func TestScopeParamsUnshadowableEvenByRowAliasedParams(t *testing.T) {
	data := mustDecode(t, `{"params": {"reportDate": "FROM-DATA"}}`)
	row := mustDecode(t, `{"reportDate": "FROM-ROW"}`)
	params := mustDecode(t, `{"reportDate": "FROM-PARAMS"}`)

	scope := NewScope(data, params).WithRow(row, "params")
	got, _, _, err := Resolve("{{params.reportDate}}", scope, testFormatContext(), "e2")
	if err != nil {
		t.Fatalf("AC4: unexpected error: %v", err)
	}
	if got != "FROM-PARAMS" {
		t.Fatalf("AC4: params must be shadowed by NOTHING, even a row aliased %q; got %q, want the params root's value %q", "params", got, "FROM-PARAMS")
	}
}
