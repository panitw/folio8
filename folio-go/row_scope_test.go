package folio8

import (
	"testing"

	"github.com/panitw/folio8/folio-go/internal/bind"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// TestRowAliasDefaultsToRowAtResolutionTime is source AC2, exercised
// through the ACTUAL production defaulting function (resolvedRowAlias,
// render.go) rather than a test that merely hardcodes "row": a
// repeating region whose "as" is absent (TableExt.As.Set == false)
// resolves its row scope's alias to "row", and the defaulting happens
// at RESOLUTION time — resolvedRowAlias is called here with an ABSENT
// Presence, exactly as a table element with no "as" key parses to
// (parse_bands.go/model.go are unchanged by this story: "as" stays
// Presence-absent, never defaulted at load).
func TestRowAliasDefaultsToRowAtResolutionTime(t *testing.T) {
	var as template.Presence[string] // zero value: Set == false, AS parsed today
	alias := resolvedRowAlias(as)
	if alias != "row" {
		t.Fatalf("AC2: an absent \"as\" must default the alias to \"row\", got %q", alias)
	}

	data := bind.Value{Kind: bind.KindObject}
	row, err := bind.DecodeData([]byte(`{"amount": "10.00"}`))
	if err != nil {
		t.Fatalf("DecodeData: %v", err)
	}
	scope := bind.NewScope(data, bind.Value{Kind: bind.KindObject}).WithRow(row, alias)

	got, _, _, rerr := bind.Resolve("{{row.amount}}", scope, testFormatContext(), "e2")
	if rerr != nil {
		t.Fatalf("AC2: unexpected error: %v", rerr)
	}
	if got != "10.00" {
		t.Fatalf("AC2: {{row.amount}} must resolve to the current row's field via the DEFAULTED alias, got %q", got)
	}
}

// TestRowAliasHonoursDeclaredAs is AC1's counterpart to the AC2 test
// above, through the same production defaulting function: a DECLARED
// "as" is used verbatim, never overridden by the default.
func TestRowAliasHonoursDeclaredAs(t *testing.T) {
	as := template.Presence[string]{Set: true, Value: "transaction"}
	alias := resolvedRowAlias(as)
	if alias != "transaction" {
		t.Fatalf("AC1: a declared \"as\" must be used verbatim, got %q", alias)
	}
}
