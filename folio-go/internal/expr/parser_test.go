package expr

import (
	"strings"
	"testing"
)

// TestParseAcceptedForms is AC3: the grammar accepts a bare dotted
// path, a function call over comma-separated arguments, a
// double-quoted string literal, a number literal, and nesting to at
// least one level.
func TestParseAcceptedForms(t *testing.T) {
	cases := []string{
		"a",
		"a.b",
		"a.b.c",
		`upper(a)`,
		`if(a, "x", "y")`,
		`sum(a.b)`,
		`"a string"`,
		`""`,
		"123",
		"-5",
		"1.50",
		"1e10",
		"-1.5e-10",
		`formatNumber(sum(t.amount), "#,##0.00")`, // AC3's own nesting example
	}
	for _, c := range cases {
		if _, err := Parse(c); err != nil {
			t.Errorf("Parse(%q): expected success, got error: %v", c, err)
		}
	}
}

// TestParseRejectedForms is AC3/AC19: every syntax-invalid case named
// by the ACs, enumerated in one table test.
func TestParseRejectedForms(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"unbalanced_paren_missing_close", `sum(a`},
		{"unbalanced_paren_extra_close", `sum(a))`},
		{"trailing_comma", `sum(a,)`},
		{"unterminated_string", `upper("a`},
		{"empty_expression", ``},
		{"empty_expression_whitespace_only", `   `},
		{"unsupported_operator", `a == b`},
		{"digit_leading_path_segment", `a.1b`},
		{"interior_whitespace", `a b`},
		{"array_index", `a[0]`},
		{"comma_at_top_level", `a, b`},
		{"bare_string_arg_where_call_expected_ok_but_dangling_paren", `upper(`},
		// QA Finding 3 (Blocker): well past maxCallDepth, so this must
		// be rejected as an ordinary located syntax error rather than
		// recursing without bound.
		{"excessive_call_nesting_depth", strings.Repeat("a(", maxCallDepth+36)},
		// QA Finding 11 (Minor): an escaped quote is not supported —
		// this must be rejected with an explicit message, not silently
		// mis-parsed into a confusing downstream error.
		{"escaped_quote_in_string", `upper("a\"b")`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Parse(c.src); err == nil {
				t.Errorf("Parse(%q): expected a syntax error, got none", c.src)
			}
		})
	}
}

// TestParseExcessiveCallNestingIsLocatedError is QA Finding 3's
// (Blocker) direct red-proof: a nested-call chain reproducing the
// reviewer's real trigger (~1.6MB, ~800,000 nested calls) through the
// same public Parse entry both folio8.ParseTemplate (load) and
// bind.BindText (render) call. Before maxCallDepth existed this input
// drove unbounded recursion into a goroutine stack overflow — a
// runtime *throw*, not a panic, so recover() cannot catch it and the
// process exits with no error, no element id, and no diagnostic
// (D-000.13: the assertion here is on the ERROR's content, never on
// mere survival — a test that only checked "no crash" would not
// distinguish a bounded rejection from a lucky, unbounded success).
func TestParseExcessiveCallNestingIsLocatedError(t *testing.T) {
	src := strings.Repeat("a(", 800000)
	_, err := Parse(src)
	if err == nil {
		t.Fatal("Parse: expected a located depth-limit error, got none (process would otherwise be at risk of a stack overflow)")
	}
	if !strings.Contains(err.Error(), "source bytes") {
		t.Errorf("Parse error = %q, want it to name the depth limit specifically", err.Error())
	}
	if !strings.Contains(err.Error(), "position") {
		t.Errorf("Parse error = %q, want a position, like every other syntax error", err.Error())
	}
}

// TestParseEscapedQuoteIsAnExplicitError is QA Finding 11's (Minor)
// direct red-proof: before this fix, `"a\"b"` produced a message about
// unrelated trailing content or a missing comma — the lexer silently
// stopped at the bogus closing quote and let a LATER stage misreport
// the cause. Now the error names the real defect directly.
func TestParseEscapedQuoteIsAnExplicitError(t *testing.T) {
	_, err := Parse(`"a\"b"`)
	if err == nil {
		t.Fatal("expected an error: a quote cannot appear inside a string literal, escaped or not")
	}
	if !strings.Contains(err.Error(), "escape sequence") {
		t.Errorf(`Parse error = %q, want it to name "escape sequences" directly, not a downstream symptom`, err.Error())
	}
}

// TestParseDiagnosticsNameTheRightCharacter is QA Finding 13's (Minor)
// four fixed cases, each verified directly against the reviewer's own
// reproduction.
func TestParseDiagnosticsNameTheRightCharacter(t *testing.T) {
	t.Run("non_ASCII_is_decoded_as_a_rune_not_a_byte", func(t *testing.T) {
		_, err := Parse("é")
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "é") {
			t.Errorf(`error = %q, want it to name "é" — the actual character, not a mis-decoded byte`, err.Error())
		}
	})
	t.Run("unbalanced_paren_carries_a_position", func(t *testing.T) {
		_, err := Parse(`sum(a`)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "position") {
			t.Errorf("error = %q, want a position, like every other syntax error in this grammar", err.Error())
		}
	})
	t.Run("EOF_after_dot_reads_as_end_of_expression_not_empty_quotes", func(t *testing.T) {
		_, err := Parse(`a.`)
		if err == nil {
			t.Fatal("expected an error")
		}
		if strings.Contains(err.Error(), `got ""`) {
			t.Errorf(`error = %q, want "end of expression", not the unreadable got ""`, err.Error())
		}
		if !strings.Contains(err.Error(), "end of expression") {
			t.Errorf(`error = %q, want it to say "end of expression"`, err.Error())
		}
	})
	t.Run("newline_is_named_explicitly", func(t *testing.T) {
		_, err := Parse("f(a,\n b)")
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "newline") {
			t.Errorf(`error = %q, want it to name "newline" explicitly`, err.Error())
		}
	})
}

// TestScanPlaceholdersUnterminatedCarriesPositionAndText is QA Finding
// 13's fifth case: scan.go's unterminated-"{{" error used to carry
// neither an offset nor the offending text, unlike every other
// diagnostic in the module.
func TestScanPlaceholdersUnterminatedCarriesPositionAndText(t *testing.T) {
	_, _, _, err := ScanPlaceholders("before {{unterminated and no closing braces")
	if err == nil {
		t.Fatal("expected an unterminated-placeholder error")
	}
	if !strings.Contains(err.Error(), "position") {
		t.Errorf("error = %q, want a position", err.Error())
	}
	if !strings.Contains(err.Error(), "unterminated and no closing braces") {
		t.Errorf("error = %q, want the offending text verbatim (AC19)", err.Error())
	}
}

// TestParseOtherBackslashesSurviveLiterally confirms Finding 11's fix
// is scoped to \" specifically: any OTHER backslash is untouched,
// exactly as before — it survives literally into the literal's own
// Value/Raw, so Value/Raw stay self-consistent (a Finding 11 property
// this fix must not regress).
func TestParseOtherBackslashesSurviveLiterally(t *testing.T) {
	e, err := Parse(`"a\b"`)
	if err != nil {
		t.Fatalf(`Parse("a\b"): unexpected error: %v`, err)
	}
	lit, ok := e.(*StringLit)
	if !ok {
		t.Fatalf("expected a StringLit, got %T", e)
	}
	if lit.Value != `a\b` {
		t.Errorf(`StringLit.Value = %q, want "a\\b" (backslash preserved literally)`, lit.Value)
	}
}

// TestParsePreservesRawText is AC15/AC19: the offending expression
// text must be recoverable verbatim, as authored.
func TestParsePreservesRawText(t *testing.T) {
	e, err := Parse(`formatNumber(t.amount, "#,##0.00")`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := e.Text(); got != `formatNumber(t.amount, "#,##0.00")` {
		t.Errorf("Text() = %q, want the exact source", got)
	}
}

// TestParseTrimsOnlyOuterWhitespace mirrors 1.6's "ws? ident ws?"
// (D-1.6.5): leading/trailing whitespace around the whole expression
// is tolerated, but interior whitespace is not.
func TestParseTrimsOnlyOuterWhitespace(t *testing.T) {
	e, err := Parse("  a.b  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p, ok := e.(*PathExpr)
	if !ok {
		t.Fatalf("expected a PathExpr, got %T", e)
	}
	if len(p.Segments) != 2 || p.Segments[0] != "a" || p.Segments[1] != "b" {
		t.Errorf("Segments = %v, want [a b]", p.Segments)
	}
}

// TestParseNestingDepth confirms nesting works past the one-level
// minimum AC3 requires.
func TestParseNestingDepth(t *testing.T) {
	e, err := Parse(`if(a, upper(lower(b)), "x")`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call, ok := e.(*CallExpr)
	if !ok || call.Name != "if" {
		t.Fatalf("expected top-level if(), got %#v", e)
	}
}
