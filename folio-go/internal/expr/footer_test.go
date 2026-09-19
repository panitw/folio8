package expr

import "testing"

// TestDeriveFooterOfShape1 is D-1.4.1's shape 1: a bare row-scoped
// path.
func TestDeriveFooterOfShape1(t *testing.T) {
	got, derivable, err := DeriveFooterOf("{{row.amount}}", "row", "transactions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !derivable {
		t.Fatal("expected shape 1 to be derivable")
	}
	if got.FooterOf != "transactions.amount" {
		t.Errorf("FooterOf = %q, want transactions.amount", got.FooterOf)
	}
	if got.HasFooterFormat {
		t.Errorf("shape 1 must not derive a footerFormat, got %q", got.FooterFormat)
	}
}

// TestDeriveFooterOfShape2 is D-1.4.1's shape 2, using the CANONICAL
// golden's own worked-example.json:19 values verbatim.
func TestDeriveFooterOfShape2(t *testing.T) {
	got, derivable, err := DeriveFooterOf(`{{formatNumber(transaction.amount, "#,##0.00")}}`, "transaction", "transactions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !derivable {
		t.Fatal("expected shape 2 to be derivable")
	}
	if got.FooterOf != "transactions.amount" {
		t.Errorf("FooterOf = %q, want transactions.amount", got.FooterOf)
	}
	if !got.HasFooterFormat || got.FooterFormat != "#,##0.00" {
		t.Errorf("expected FooterFormat default #,##0.00, got %+v", got)
	}
}

// TestDeriveFooterOfRejectsOtherShapes is AC21's rejection arm at the
// derivation-function level: any other bind shape is reported as NOT
// derivable, leaving the load-error decision to the caller.
func TestDeriveFooterOfRejectsOtherShapes(t *testing.T) {
	cases := []string{
		`{{if(row.x, row.a, row.b)}}`,               // the AC21 example
		`{{upper(row.amount)}}`,                     // not one of the two shapes
		`{{other.amount}}`,                          // wrong alias
		`{{row}}`,                                   // bare alias, no field
		`prefix {{row.amount}}`,                     // surrounding literal text
		`{{row.amount}} suffix`,                     // surrounding literal text
		`{{formatNumber(other.amount, "x")}}`,       // shape 2, wrong alias
		`{{formatNumber(row.amount, count.thing)}}`, // shape 2, pattern not a literal
	}
	for _, bind := range cases {
		got, derivable, err := DeriveFooterOf(bind, "row", "transactions")
		if err != nil {
			t.Errorf("DeriveFooterOf(%q): unexpected error: %v", bind, err)
			continue
		}
		if derivable {
			t.Errorf("DeriveFooterOf(%q): expected NOT derivable, got %+v", bind, got)
		}
	}
}

// TestDeriveFooterOfIgnoresReservedTokens: {{page}}/{{pages}} are
// never derivable (AD-4).
func TestDeriveFooterOfIgnoresReservedTokens(t *testing.T) {
	_, derivable, err := DeriveFooterOf("{{page}}", "row", "transactions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if derivable {
		t.Fatal("a reserved placeholder must never be reported as derivable")
	}
}
