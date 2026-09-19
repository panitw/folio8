package folio8

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/expr"
	"github.com/panitw/folio8/folio-go/internal/geom"
)

func expressionCommand(id, columnID, binding string) []byte {
	command, _ := json.Marshal(map[string]any{"kind": "updateTableColumnExpression", "version": 1, "id": id, "columnId": columnID, "binding": binding})
	return command
}

func TestTableColumnExpressionPreservesCompleteTextAndProjection(t *testing.T) {
	tpl, id := tableColumnAuthoringFixture(t, []geom.Length{100000}, 0)
	view, _ := tableColumns(tpl, id)
	columnID := view.Columns[0].ID
	for _, binding := range []string{
		"{{row.date}}", `{{upper(row.trn_code)}}`, `{{formatDate(row.date, "dd/MM/yyyy")}}`,
		`{{formatNumber(row.amount * 1.07, "#,##0.00")}}`,
		`{{if(row.amount > 0, "credit", "debit")}}`,
		"  Code: {{ upper(row.trn_code) }}\r\n", "literal text", "", strings.Repeat("x", 256), strings.Repeat("é", 128),
	} {
		t.Run(binding, func(t *testing.T) {
			if _, err := applyComponentCommand(tpl, expressionCommand(id, columnID, binding)); err != nil {
				t.Fatal(err)
			}
			view, err := tableColumns(tpl, id)
			if err != nil || view.Columns[0].Binding != binding {
				t.Fatalf("binding bytes changed: %#v, %v", view, err)
			}
			before := canonicalBytes(t, tpl)
			if _, err := applyComponentCommand(tpl, expressionCommand(id, columnID, binding)); err != nil || !bytes.Equal(before, canonicalBytes(t, tpl)) {
				t.Fatalf("unchanged binding was not a no-op: %v", err)
			}
			reopened, err := ParseTemplate(before)
			if err != nil {
				t.Fatal(err)
			}
			again, err := tableColumns(reopened, id)
			if err != nil || again.Columns[0].Binding != binding {
				t.Fatalf("reopen changed binding: %#v, %v", again, err)
			}
		})
	}
}

func TestTableColumnExpressionRefusesMalformedEnvelopeAndLocatedExpressionsAtomically(t *testing.T) {
	tpl, id := tableColumnAuthoringFixture(t, []geom.Length{100000}, 0)
	view, _ := tableColumns(tpl, id)
	columnID := view.Columns[0].ID
	before := canonicalBytes(t, tpl)
	prefix := fmt.Sprintf(`{"kind":"updateTableColumnExpression","version":1,"id":%q,"columnId":%q`, id, columnID)
	commands := []string{prefix + `}`, prefix + `,"binding":null}`, prefix + `,"binding":  null  }`, prefix + `,"binding":4}`, prefix + `,"binding":true}`, prefix + `,"binding":[]}`, prefix + `,"binding":{}}`, prefix + `,"binding":"","extra":0}`, prefix + `,"binding":"","binding":"again"}`}
	commands = append(commands, string(expressionCommand(id, columnID, strings.Repeat("x", 257))), string(expressionCommand(id, columnID, strings.Repeat("é", 129))), string(expressionCommand("missing", columnID, "")), string(expressionCommand(id, "missing", "")))
	for _, command := range commands {
		if _, err := applyComponentCommand(tpl, []byte(command)); err == nil {
			t.Fatalf("invalid command accepted: %s", command)
		}
		if !bytes.Equal(before, canonicalBytes(t, tpl)) {
			t.Fatalf("refusal changed document: %s", command)
		}
	}
	// {{row.amount > 1.07}} is boolean-kind: still refused in text. (A
	// number-kind bind such as {{row.amount * 1.07}} is legal since the
	// number-in-text spec, 2026-09-13.)
	for _, binding := range []string{`{{upper(}}`, `{{unknown(row.date)}}`, `{{upper(7)}}`, `{{row.amount > 1.07}}`} {
		_, err := applyComponentCommand(tpl, expressionCommand(id, columnID, binding))
		var diagnosed *RenderError
		var located *expr.LocatedError
		if !errors.As(err, &diagnosed) || !errors.As(err, &located) || diagnosed.Diagnostic.Code != DiagCodeExpressionInvalid || diagnosed.Diagnostic.ElementID != columnID || diagnosed.Diagnostic.DataPath != "bind" {
			t.Fatalf("expression refusal must be located: %q: %T %v", binding, err, err)
		}
		if !bytes.Equal(before, canonicalBytes(t, tpl)) {
			t.Fatalf("expression refusal changed document: %q", binding)
		}
	}
}

func TestTableColumnExpressionKeepsFooterAndAliasValidationAtomic(t *testing.T) {
	for _, footer := range []string{"sum", "avg"} {
		t.Run(footer, func(t *testing.T) {
			tpl, id := tableColumnAuthoringFixture(t, []geom.Length{100000}, 0)
			view, _ := tableColumns(tpl, id)
			columnID := view.Columns[0].ID
			configure := func(source string) {
				t.Helper()
				mustApplyToTable(t, tpl, fmt.Sprintf(`{"kind":"updateTableColumnFooter","version":1,"id":%q,"columnId":%q,"footer":%q,"footerOf":%q,"footerFormat":""}`, id, columnID, footer, source))
			}
			configure("")
			for _, binding := range []string{"", "literal", `{{formatNumber(row.amount * 1.07, "#,##0.00")}}`} {
				before := canonicalBytes(t, tpl)
				_, err := applyComponentCommand(tpl, expressionCommand(id, columnID, binding))
				var diagnosed *RenderError
				if !errors.As(err, &diagnosed) || diagnosed.Diagnostic.Code != DiagCodeTableFooterSourceUnresolved || !bytes.Equal(before, canonicalBytes(t, tpl)) {
					t.Fatalf("implicit footer should refuse %q atomically: %v", binding, err)
				}
			}
			for _, binding := range []string{`{{row.amount}}`, `{{formatNumber(row.amount, "#,##0.00")}}`} {
				if _, err := applyComponentCommand(tpl, expressionCommand(id, columnID, binding)); err != nil {
					t.Fatal(err)
				}
			}
			configure("items.amount")
			formula := `{{formatNumber(row.amount * 1.07, "#,##0.00")}}`
			if _, err := applyComponentCommand(tpl, expressionCommand(id, columnID, formula)); err != nil {
				t.Fatal(err)
			}
			before := canonicalBytes(t, tpl)
			if _, err := applyComponentCommand(tpl, []byte(fmt.Sprintf(`{"kind":"configureTableBinding","version":1,"id":%q,"collection":"items[]","alias":"txn"}`, id))); err == nil || !bytes.Equal(before, canonicalBytes(t, tpl)) {
				t.Fatalf("unsupported alias migration must refuse atomically: %v", err)
			}
			if _, err := applyComponentCommand(tpl, expressionCommand(id, columnID, `{{formatNumber(row.amount, "#,##0.00")}}`)); err != nil {
				t.Fatal(err)
			}
			mustApplyToTable(t, tpl, fmt.Sprintf(`{"kind":"configureTableBinding","version":1,"id":%q,"collection":"items[]","alias":"txn"}`, id))
			view, _ = tableColumns(tpl, id)
			if view.Columns[0].Binding != `{{formatNumber(txn.amount, "#,##0.00")}}` || view.Columns[0].FooterOf != "items.amount" {
				t.Fatalf("supported alias migration changed footer: %#v", view)
			}
		})
	}
}
