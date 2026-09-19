package bind

import (
	"strings"
	"testing"
)

func mustDecode(t *testing.T, jsonData string) Value {
	t.Helper()
	v, err := DecodeData([]byte(jsonData))
	if err != nil {
		t.Fatalf("DecodeData(%s): %v", jsonData, err)
	}
	return v
}

// noParams is the "no runtime values supplied" params root most
// existing BindText tests use — they exercise the data root only, and
// AC16's absent-params behaviour is pinned separately.
var noParams = Value{Kind: KindObject}

// TestAD14Triple is D-1.6.2/AC11's retained fixture triple: AC8, AC9
// and AC10 proved over ONE template text and ONE data path
// ("customer.name"), differing only in the data. Rows 1 and 2 are the
// proof (absent vs explicit null, opposite outcomes) — a future
// refactor collapsing Presence to two states must turn one of them
// red.
func TestAD14Triple(t *testing.T) {
	const text = "Statement for {{customer.name}}"
	const elementID = "e1"

	t.Run("row1_absent_is_error", func(t *testing.T) {
		data := mustDecode(t, `{}`)
		_, err := BindText(text, data, noParams, testFormatContext(), elementID)
		if err == nil {
			t.Fatal("AC8: absent path must be an Error")
		}
		if !strings.Contains(err.Error(), "customer.name") || !strings.Contains(err.Error(), elementID) {
			t.Errorf("error must name both the data path and the element id, got: %v", err)
		}
	})

	t.Run("row2_null_renders_empty", func(t *testing.T) {
		data := mustDecode(t, `{"customer": {"name": null}}`)
		got, err := BindText(text, data, noParams, testFormatContext(), elementID)
		if err != nil {
			t.Fatalf("AC9: explicit null must not be an error, got: %v", err)
		}
		if got != "Statement for " {
			t.Errorf("AC9: null must render as empty, got %q", got)
		}
	})

	// Owner decision 2026-09-13 (revising AD-14 for numbers in text only):
	// a number prints as its exact decimal; a boolean stays a wrong-kind
	// Error, never coerced.
	t.Run("row3_number_prints_exact_decimal", func(t *testing.T) {
		data := mustDecode(t, `{"customer": {"name": 123}}`)
		got, err := BindText(text, data, noParams, testFormatContext(), elementID)
		if err != nil {
			t.Fatalf("a JSON number bound into text must print, got: %v", err)
		}
		if got != "Statement for 123" {
			t.Errorf("got %q, want %q", got, "Statement for 123")
		}
	})

	t.Run("row4_boolean_is_error_no_coercion", func(t *testing.T) {
		data := mustDecode(t, `{"customer": {"name": true}}`)
		_, err := BindText(text, data, noParams, testFormatContext(), elementID)
		if err == nil {
			t.Fatal("AC10: a JSON boolean bound into a text element must be an Error, never coerced")
		}
		if !strings.Contains(err.Error(), "bool") || !strings.Contains(err.Error(), elementID) {
			t.Errorf("error should name the boolean kind and the element, got: %v", err)
		}
	})
}

// TestBindTextPrintsNumbersAsExactDecimals is the spec's I/O matrix at the
// bind layer: data numbers and computed numbers share one printer.
func TestBindTextPrintsNumbersAsExactDecimals(t *testing.T) {
	data := mustDecode(t, `{"v": 1234.50, "i": 2067071865, "n": -3.5, "e": 1e3, "z": -0, "zs": 0.000,
		"a": 1.5, "b": 2, "items": [{"x": 1}, {"x": 2}, {"x": 3}], "ten": 10}`)
	for _, tc := range []struct{ text, want string }{
		{"{{v}}", "1234.50"},
		{"{{i}}", "2067071865"},
		{"{{n}}", "-3.5"},
		{"{{e}}", "1000"},
		{"{{z}}", "0"},
		{"{{zs}}", "0.000"},
		{"{{count(items)}}", "3"},
		{"{{sum(items.x)}}", "6"},
		{"{{a + b}}", "3.5"},
		{"{{1}}", "1"},
		{"Total: {{ten}} THB", "Total: 10 THB"},
	} {
		got, err := BindText(tc.text, data, noParams, testFormatContext(), "e1")
		if err != nil {
			t.Fatalf("%s: %v", tc.text, err)
		}
		if got != tc.want {
			t.Errorf("%s = %q, want %q", tc.text, got, tc.want)
		}
	}
	for _, text := range []string{"{{a / 0}}", "{{true}}"} {
		if _, err := BindText(text, data, noParams, testFormatContext(), "e1"); err == nil {
			t.Errorf("%s must stay a located Error", text)
		}
	}
}

// TestBindTextAcceptsBareDottedPath is AC15.
func TestBindTextAcceptsBareDottedPath(t *testing.T) {
	data := mustDecode(t, `{"customer": {"name": "Ada Lovelace"}}`)
	got, err := BindText("Statement for {{customer.name}}", data, noParams, testFormatContext(), "e1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Statement for Ada Lovelace" {
		t.Errorf("got %q", got)
	}
}

// TestBindTextRejectsGenuineSyntaxErrors is F10's re-point, first half:
// Story 3.2 gives internal/bind a real expression grammar (D-1.6.5),
// so 1.6's blanket "anything expression-shaped is rejected" test is
// gone — these cases remain genuine SYNTAX errors under the new
// grammar too (AC19: array index, a bare comma-list, a bare operator,
// interior whitespace), and each error must still name the element id.
func TestBindTextRejectsGenuineSyntaxErrors(t *testing.T) {
	cases := []string{
		`{{a[0]}}`,
		`{{a, b}}`,
		`{{a + b}}`,
		`{{a b}}`, // interior whitespace
	}
	data := mustDecode(t, `{}`)
	for _, c := range cases {
		_, err := BindText(c, data, noParams, testFormatContext(), "e1")
		if err == nil {
			t.Errorf("BindText(%q): expected a located error", c)
			continue
		}
		if !strings.Contains(err.Error(), "e1") {
			t.Errorf("BindText(%q): error must name the element id, got: %v", c, err)
		}
	}
}

// TestBindTextRejectsExcessiveCallNestingAtRender is QA Finding 3's
// (Blocker) render-entry-point half: BindText is the render-time
// resolver, the other reachable path the finding named alongside
// folio8.ParseTemplate (load). Before internal/expr's maxCallDepth
// existed, a value nesting ~800,000 function calls (~1.6MB, an
// unremarkable size for a .folio file) drove unbounded recursion into
// an unrecoverable runtime stack overflow reachable straight from a
// render call. Now it must be an ordinary located error, like every
// other rejected form.
func TestBindTextRejectsExcessiveCallNestingAtRender(t *testing.T) {
	data := mustDecode(t, `{}`)
	deep := "{{" + strings.Repeat("a(", 800000) + "}}"
	_, err := BindText(deep, data, noParams, testFormatContext(), "e1")
	if err == nil {
		t.Fatal("BindText: expected a located error; excessive call nesting must not risk a stack overflow")
	}
	if !strings.Contains(err.Error(), "e1") {
		t.Errorf("error must name the element id, got: %v", err)
	}
	if !strings.Contains(err.Error(), "source bytes") {
		t.Errorf("error must name the depth limit specifically, got: %v", err)
	}
}

// TestBindTextArrayPathIsLocatedNotScalarError is QA Finding 16(b)
// (Minor): lookupBound's default arm — what makes an array or object
// path a located error rather than a plausible value — had NO test at
// all before this fix (a search for its message, "not a scalar value
// usable in an expression", across every _test.go file returned
// nothing). Both folio-format.md's own "### Expressions" section and
// the (out-of-story) author-facing reference explicitly claim an
// empty array [] as an if() condition is a located error; expr.Value
// has no array kind, so that claim rested entirely on this previously
// untested conversion arm.
func TestBindTextArrayPathIsLocatedNotScalarError(t *testing.T) {
	data := mustDecode(t, `{"tags": ["a", "b"]}`)
	_, err := BindText(`{{tags}}`, data, noParams, testFormatContext(), "e1")
	if err == nil {
		t.Fatal("BindText: expected a located error — an array is not a scalar value usable in an expression")
	}
	if !strings.Contains(err.Error(), "e1") {
		t.Errorf("error must name the element id, got: %v", err)
	}
	if !strings.Contains(err.Error(), "tags") {
		t.Errorf("error must name the offending path, got: %v", err)
	}
	if !strings.Contains(err.Error(), "not a scalar") {
		t.Errorf(`error must say "not a scalar", got: %v`, err)
	}
}

// TestBindTextParsesFormatNumberAndFailsOnAbsentData is F10's re-point
// (Story 3.4): formatNumber is now IMPLEMENTED (AC16), so a call over
// data that does not carry the operand path fails at EVALUATION, on
// the absent path — a DIFFERENT located error from a syntax rejection
// (D-000.9's signature-failure shape: "err != nil" alone would pass
// identically for either failure, so the message itself must be
// asserted).
func TestBindTextParsesFormatNumberAndFailsOnAbsentData(t *testing.T) {
	data := mustDecode(t, `{}`)
	_, err := BindText(`{{formatNumber(transaction.amount, "#,##0.00")}}`, data, noParams, testFormatContext(), "e1")
	if err == nil {
		t.Fatal(`expected a located error: "transaction" is absent from the data`)
	}
	if !strings.Contains(err.Error(), "e1") {
		t.Errorf("error must name the element id, got: %v", err)
	}
	if !strings.Contains(err.Error(), "transaction") {
		t.Errorf("error must name the absent path, got: %v", err)
	}
	if strings.Contains(err.Error(), "not a valid expression") {
		t.Errorf("error must NOT read as a syntax rejection — formatNumber(...) is syntactically valid: %v", err)
	}
}

// TestBindTextAcceptsBareStringLiteral: AC3's grammar accepts a
// double-quoted string literal as a standalone expression — Story 3.2
// widens the binding grammar past bare paths, and a literal is one of
// the four accepted primary forms, so "{{\"literal\"}}" now renders
// its own content rather than being rejected (the pre-3.2 behaviour).
func TestBindTextAcceptsBareStringLiteral(t *testing.T) {
	data := mustDecode(t, `{}`)
	got, err := BindText(`{{"literal"}}`, data, noParams, testFormatContext(), "e1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "literal" {
		t.Errorf("got %q, want %q", got, "literal")
	}
}

// TestBindTextReservesPageAndPages is AC18/AC19: {{page}} and
// {{pages}} pass through untouched, never resolved from data, never
// an error — even when the data contains a top-level "page" key
// (the retained fixture for the reservation).
func TestBindTextReservesPageAndPages(t *testing.T) {
	data := mustDecode(t, `{"page": "HIJACKED"}`)
	got, err := BindText("Page {{page}} of {{pages}}", data, noParams, testFormatContext(), "e1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Page {{page}} of {{pages}}" {
		t.Errorf("got %q, want the placeholders left byte-for-byte unchanged", got)
	}
}

// TestBindTextPageReservationIsRedProofable is AC19's own red-proof
// directive: with the reservation branch effectively bypassed (by
// asserting the byte-for-byte-unchanged property directly, rather than
// merely checking for absence of an error), a document containing a
// top-level "page" key silently capturing the slot would change what
// {{page}} renders — this test would go red the moment that happens.
//
// QA Nit 15 (this story's review): the original version compared
// against DATA WITHOUT a "page" key, so deleting the reservation
// branch made the second call go Absent — the test reddened on that
// unrelated error check, never on the got1 != got2 comparison its own
// message describes (D-000.13: it failed, but not for the reason it
// names). Both data documents here DO contain a top-level "page" key,
// with DIFFERENT values, so a removed reservation makes both calls
// succeed — with different text — and the comparison itself is what
// reddens, naming the actual hijack.
func TestBindTextPageReservationIsRedProofable(t *testing.T) {
	withPage := mustDecode(t, `{"page": "HIJACKED"}`)
	withOtherPage := mustDecode(t, `{"page": "SOMETHING ELSE"}`)

	got1, err1 := BindText("{{page}}", withPage, noParams, testFormatContext(), "e1")
	if err1 != nil {
		t.Fatalf("unexpected error: %v", err1)
	}
	got2, err2 := BindText("{{page}}", withOtherPage, noParams, testFormatContext(), "e1")
	if err2 != nil {
		t.Fatalf("unexpected error: %v", err2)
	}
	if got1 != got2 {
		t.Fatalf("a top-level \"page\" data key changed {{page}}'s rendering: %q vs %q", got1, got2)
	}
	if got1 != "{{page}}" {
		t.Fatalf("got %q, want the literal placeholder text", got1)
	}
}

// TestBindTextNumberCoefficientOverflowIsLocatedError is QA Finding 2
// (this story's review, Major): AsDecimal/NewDecimal previously had no
// production caller at all, so AC3/AC4's mandated "located error naming
// the data path and the element id" for an out-of-bound number literal
// existed only in internal/bind's own unit tests, never on a path real
// report data reaches. A number is always wrong-kind for a text binding
// (AC10) — that does not change here — but the literal is now validated
// via Decimal first, so a malformed one is reported as such rather than
// silently passing as an unremarkable "wrong kind" without ever
// touching the exact-decimal conversion AD-23 requires.
func TestBindTextNumberCoefficientOverflowIsLocatedError(t *testing.T) {
	// 20 significant digits: exceeds int64 (AC3), same literal
	// TestNewDecimalCoefficientBound already proves NewDecimal rejects
	// directly.
	data := mustDecode(t, `{"transaction": {"amount": 12345678901234567890}}`)
	_, err := BindText("Amount: {{transaction.amount}}", data, noParams, testFormatContext(), "e7")
	if err == nil {
		t.Fatal("expected a located error for a coefficient that does not fit int64")
	}
	if !strings.Contains(err.Error(), "e7") {
		t.Errorf("error must name the element id, got: %v", err)
	}
	if !strings.Contains(err.Error(), "transaction.amount") {
		t.Errorf("error must name the data path, got: %v", err)
	}
	if !strings.Contains(err.Error(), "coefficient") {
		t.Errorf("error must describe the coefficient-bound violation Decimal actually detected, got: %v", err)
	}
}

// TestBindTextChargesPrintedExponentZeros: printing a positive exponent
// expands into that many zeros, so it is charged to the placeholder's
// budget. 1e100000 fits alone; after a 950,000-byte string resolve in the
// same placeholder the 100,000 printed zeros exceed the budget.
func TestBindTextChargesPrintedExponentZeros(t *testing.T) {
	data := mustDecode(t, `{"v": 1e100000, "s": "`+strings.Repeat("x", 950_000)+`"}`)
	got, err := BindText("{{v}}", data, noParams, testFormatContext(), "e1")
	if err != nil {
		t.Fatalf("1e100000 alone must print within budget: %v", err)
	}
	if len(got) != 100_001 || got[0] != '1' || strings.Trim(got[1:], "0") != "" {
		t.Fatalf("1e100000 printed %d bytes, want 1 followed by 100000 zeros", len(got))
	}
	_, err = BindText(`{{s != "" ? v : v}}`, data, noParams, testFormatContext(), "e1")
	if err == nil || !strings.Contains(err.Error(), "work units") {
		t.Fatalf("printing past an exhausted budget must be refused, got: %v", err)
	}
}

// TestBindTextWellFormedNumberPrintsItsScale pins the number-in-text rule
// alongside Finding 2's fix: a well-formed number (one Decimal accepts)
// prints as its exact decimal, keeping the scale the data wrote ("1.50",
// never "1.5"), while a malformed one stays the coefficient Error above.
func TestBindTextWellFormedNumberPrintsItsScale(t *testing.T) {
	data := mustDecode(t, `{"transaction": {"amount": 1.50}}`)
	got, err := BindText("Amount: {{transaction.amount}}", data, noParams, testFormatContext(), "e7")
	if err != nil {
		t.Fatalf("a well-formed number must print, got: %v", err)
	}
	if got != "Amount: 1.50" {
		t.Errorf("got %q, want %q", got, "Amount: 1.50")
	}
}

// TestBindTextParamsRootTakesPrecedenceOverData is AC13/AC14
// (D-1.7.4), copying TestBindTextPageReservationIsRedProofable's
// corrected shape (QA Nit 15) verbatim: two DATA documents, both
// carrying a top-level "params" key with DIFFERENT values, plus one
// supplied params document. Equality alone (got1 == got2) is the
// weaker half — an implementation that simply MERGES params into the
// data root would still pass equality if both data documents were
// identical, so both checks are required, and the value check is what
// actually names the hijack were the reservation to fail.
func TestBindTextParamsRootTakesPrecedenceOverData(t *testing.T) {
	dataA := mustDecode(t, `{"params": {"reportDate": "SHADOWED-FROM-DATA-A"}}`)
	dataB := mustDecode(t, `{"params": {"reportDate": "SHADOWED-FROM-DATA-B"}}`)
	params := mustDecode(t, `{"reportDate": "2026-08-23"}`)

	got1, err1 := BindText("{{params.reportDate}}", dataA, params, testFormatContext(), "e1")
	if err1 != nil {
		t.Fatalf("unexpected error: %v", err1)
	}
	got2, err2 := BindText("{{params.reportDate}}", dataB, params, testFormatContext(), "e1")
	if err2 != nil {
		t.Fatalf("unexpected error: %v", err2)
	}
	if got1 != got2 {
		t.Fatalf("a top-level \"params\" data key changed {{params.reportDate}}'s rendering: %q vs %q", got1, got2)
	}
	if got1 != "2026-08-23" {
		t.Fatalf("AC13: {{params.reportDate}} must resolve against the SUPPLIED PARAMS document, got %q, want the params value \"2026-08-23\"", got1)
	}

	// The whitespace-tolerant spelling shadows identically (M-6): the
	// fix belongs at path-segment level, not on the trimmed literal.
	got3, err3 := BindText("{{ params.reportDate }}", dataA, params, testFormatContext(), "e1")
	if err3 != nil {
		t.Fatalf("unexpected error: %v", err3)
	}
	if got3 != "2026-08-23" {
		t.Fatalf("AC13: the spaced spelling must resolve identically, got %q", got3)
	}
}

// TestBindTextParamsMissDoesNotFallBackToData is AC13/AC14's
// complementary case, and RP-3's own target (D-1.7.4): a SUPPLIED
// params document that is missing the queried key must still produce
// AC16's absent-params error, never silently fall back to a same-named
// key sitting in report data. Two data documents, both carrying a
// top-level "params.reportDate" decoy with DIFFERENT values, plus one
// supplied params document that does NOT have "reportDate" — both
// calls must produce the located absent-params error, and neither may
// equal the decoy value (which would prove a fallback happened, naming
// the hijack).
func TestBindTextParamsMissDoesNotFallBackToData(t *testing.T) {
	dataA := mustDecode(t, `{"params": {"reportDate": "SHADOWED-FROM-DATA-A"}}`)
	dataB := mustDecode(t, `{"params": {"reportDate": "SHADOWED-FROM-DATA-B"}}`)
	params := mustDecode(t, `{"otherField": "present-but-no-reportDate"}`)

	got1, err1 := BindText("{{params.reportDate}}", dataA, params, testFormatContext(), "e1")
	if err1 == nil {
		t.Fatalf("RP-3: a params miss must not fall back to report data, got no error, value %q (report data's decoy)", got1)
	}
	if !strings.Contains(err1.Error(), "params.reportDate") || !strings.Contains(err1.Error(), "the supplied params") {
		t.Errorf("error must name the params path and the params root, got: %v", err1)
	}

	got2, err2 := BindText("{{params.reportDate}}", dataB, params, testFormatContext(), "e1")
	if err2 == nil {
		t.Fatalf("RP-3: a params miss must not fall back to report data, got no error, value %q (report data's decoy)", got2)
	}
}

// TestBindTextParamsAbsentIsLocatedError is AC16 (D-1.7.4, AD-14):
// "{{params.x}}" with no params supplied is an Error naming the path
// AND the element id — and the message must name the PARAMS root, not
// report data (M-6: "absent from the report data" would be actively
// misleading here, since the value was never sought in report data at
// all).
func TestBindTextParamsAbsentIsLocatedError(t *testing.T) {
	data := mustDecode(t, `{}`)
	_, err := BindText("{{params.reportDate}}", data, noParams, testFormatContext(), "e9")
	if err == nil {
		t.Fatal("AC16: an absent params path must be a located Error")
	}
	if !strings.Contains(err.Error(), "e9") {
		t.Errorf("error must name the element id, got: %v", err)
	}
	if !strings.Contains(err.Error(), "params.reportDate") {
		t.Errorf("error must name the params path, got: %v", err)
	}
	if !strings.Contains(err.Error(), "params") || strings.Contains(err.Error(), "report data") {
		t.Errorf("error must name the PARAMS root, not report data, got: %v", err)
	}
}

// TestBindTextParamsBareIsLocatedError is AC17: "{{params}}" bare, no
// dot, is a located error — params is a namespace, not a value.
func TestBindTextParamsBareIsLocatedError(t *testing.T) {
	data := mustDecode(t, `{}`)
	_, err := BindText("{{params}}", data, noParams, testFormatContext(), "e10")
	if err == nil {
		t.Fatal("AC17: a bare {{params}} must be a located Error")
	}
	if !strings.Contains(err.Error(), "e10") {
		t.Errorf("error must name the element id, got: %v", err)
	}
	if !strings.Contains(err.Error(), "namespace") {
		t.Errorf("error should say params is a namespace, not a value, got: %v", err)
	}
}

// TestBindTextTopLevelParamsKeyInDataIsNotAnError is AC15 (D-1.7.4,
// verbatim): "rejecting it would refuse legitimate caller data over a
// name collision the caller may not control." A top-level "params" key
// in report data is legal, ordinary, unreachable JSON — decoding and
// binding an UNRELATED placeholder against that data must not error.
func TestBindTextTopLevelParamsKeyInDataIsNotAnError(t *testing.T) {
	data := mustDecode(t, `{"params": {"reportDate": "SHADOWED"}, "customer": {"name": "Ada"}}`)
	got, err := BindText("Statement for {{customer.name}}", data, noParams, testFormatContext(), "e11")
	if err != nil {
		t.Fatalf("a top-level \"params\" key in report data must not be an error for an unrelated binding, got: %v", err)
	}
	if got != "Statement for Ada" {
		t.Errorf("got %q", got)
	}
}

// TestBindTextPageAndPagesUnaffectedByParamsRoot is AC17a (AD-4): the
// existing {{page}}/{{pages}} reservation stays byte-for-byte
// unchanged even in a document that ALSO supplies a params document —
// a params implementation that quietly reroutes the reservation branch
// reddens here.
func TestBindTextPageAndPagesUnaffectedByParamsRoot(t *testing.T) {
	data := mustDecode(t, `{"page": "HIJACKED"}`)
	params := mustDecode(t, `{"reportDate": "2026-08-23"}`)
	got, err := BindText("Page {{page}} of {{pages}}, dated {{params.reportDate}}", data, params, testFormatContext(), "e12")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Page {{page}} of {{pages}}, dated 2026-08-23" {
		t.Errorf("got %q", got)
	}
}

// TestBindTextDottedPageAndPagesPathsAreOrdinaryDataPaths is AC3's
// D-000.50 subject, asserted BEHAVIOURALLY rather than by inspecting
// BindTextSpans' call sites (this story's review, Blocker 2:
// TestBindResolutionRootsAreClosed keys its OBSERVED set on lookupBound
// call sites — a PROXY. An early-return shape identical to the
// reservation's own dispatch a few lines above it, `if path[0] ==
// "page" { record(nil); continue }`, never calls lookupBound at all, so
// the AST scan sees nothing to disagree with — yet a probe confirmed
// {{page.number}}/{{page.total}} then render without error, which is
// exactly the "page" NAMESPACE AC3 forbids).
//
// The property AC3 actually requires — "no page namespace exists, and
// none can be added" — is a statement about what {{page.<anything>}}
// RESOLVES TO, not about which internal helper produced it. This test
// asserts that directly: "page"/"pages" are reserved ONLY as bare whole
// tokens (the TRIMMED literal "page" or "pages" exactly); a DOTTED path
// under either does not match that reservation and falls through to the
// ordinary data-root lookup, which — against data carrying no top-level
// "page"/"pages" key — is the ordinary "absent from data" error. Never
// a namespace-specific resolution, and never a silent empty string.
//
// This assertion cannot be evaded by reshaping BindTextSpans' internal
// dispatch (a differently-named helper, an early return, a lookup
// table): it reads the OUTPUT, not the mechanism that produced it — the
// artifact that actually carries the property (D-000.21).
//
// RED-PROOF (D-000.52), executed by hand against the reviewer's exact
// mutation and reverted, restore confirmed by digest — see the story's
// Delivery Log for the transcript: inserting `if path[0] == "page" {
// record(nil); continue }` immediately above the params branch in
// BindTextSpans makes every case below resolve to "" instead of
// erroring, reddening this test.
func TestBindTextDottedPageAndPagesPathsAreOrdinaryDataPaths(t *testing.T) {
	empty := mustDecode(t, `{}`)
	cases := []string{"{{page.number}}", "{{page.total}}", "{{pages.total}}", "{{pages.number}}"}
	for _, text := range cases {
		t.Run(text, func(t *testing.T) {
			got, err := BindText(text, empty, noParams, testFormatContext(), "e1")
			if err == nil {
				t.Fatalf("%s resolved to %q without error against data carrying no top-level \"page\"/"+
					"\"pages\" key — a \"page\" NAMESPACE exists, which AD-4/AC3 forbid: \"no page "+
					"namespace exists and none can be added\"", text, got)
			}
			if !strings.Contains(err.Error(), "absent from the report data") {
				t.Errorf("%s: want the ordinary absent-data-path error, got a different shape: %v — a "+
					"different error here could mean a namespace-specific code path was reached instead "+
					"of the ordinary data lookup", text, err)
			}
			t.Logf("%s correctly rejected as an ordinary absent data path: %v", text, err)
		})
	}
}
