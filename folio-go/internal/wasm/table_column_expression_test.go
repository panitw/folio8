package wasm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestEngineColumnExpressionExactBytesHistoryAndRefusals(t *testing.T) {
	input, err := os.ReadFile("../../../fixtures/statement-1/input.folio")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock())
	if _, err := engine.Load(input); err != nil {
		t.Fatal(err)
	}
	const id = "e8"
	columns, err := engine.TableColumns(id)
	if err != nil {
		t.Fatal(err)
	}
	columnID := columns.Table.Columns[0].ID
	command := func(binding string) []byte {
		data, _ := json.Marshal(map[string]any{"kind": "updateTableColumnExpression", "version": 1, "id": id, "columnId": columnID, "binding": binding})
		return data
	}
	serialized := func() []byte {
		t.Helper()
		data, _, err := engine.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	for _, binding := range []string{`{{upper(row.trn_code)}}`, `{{formatDate(row.date, "dd/MM/yyyy")}}`, `{{formatNumber(row.amount * 1.07, "#,##0.00")}}`, " Code: {{row.trn_code}}\r\n", "literal", "", strings.Repeat("x", 256)} {
		t.Run(binding, func(t *testing.T) {
			before := serialized()
			previous, err := engine.TableColumns(id)
			if err != nil {
				t.Fatal(err)
			}
			start := engine.Snapshot()
			committed, err := engine.Apply(command(binding))
			if err != nil || committed.Revision != start.Revision+1 {
				t.Fatalf("commit: %#v, %v", committed, err)
			}
			after := serialized()
			projected, err := engine.TableColumns(id)
			if err != nil || projected.Table.Columns[0].Binding != binding {
				t.Fatalf("projection changed expression: %#v %v", projected, err)
			}
			fresh := NewEngine(testClock())
			if _, err := fresh.Load(after); err != nil {
				t.Fatal(err)
			}
			reopened, _ := fresh.TableColumns(id)
			again, _, _ := fresh.Serialize()
			if reopened.Table.Columns[0].Binding != binding || !bytes.Equal(after, again) {
				t.Fatal("reopen changed exact formula bytes")
			}
			if _, err := engine.Undo(); err != nil || !bytes.Equal(serialized(), before) {
				t.Fatalf("formula was not one undo step: %v", err)
			}
			stable := engine.Snapshot()
			undo, redo := append([][]byte(nil), engine.undo...), append([][]byte(nil), engine.redo...)
			if _, err := engine.Apply(command(previous.Table.Columns[0].Binding)); err != nil || !reflect.DeepEqual(stable, engine.Snapshot()) {
				t.Fatalf("unchanged text changed revision: %v", err)
			}
			invalid := [][]byte{
				command(`{{upper(}}`), command(`{{unknown(row.date)}}`), command(`{{upper(7)}}`), command(`{{row.amount > 1.07}}`), command(strings.Repeat("é", 129)),
				[]byte(fmt.Sprintf(`{"kind":"updateTableColumnExpression","version":1,"id":%q,"columnId":%q}`, id, columnID)),
			}
			for _, raw := range []string{"null", "true", "9", "[]", "{}"} {
				invalid = append(invalid, []byte(fmt.Sprintf(`{"kind":"updateTableColumnExpression","version":1,"id":%q,"columnId":%q,"binding":%s}`, id, columnID, raw)))
			}
			for _, refused := range invalid {
				if _, err := engine.Apply(refused); err == nil {
					t.Fatalf("invalid expression accepted: %s", refused)
				}
				if !bytes.Equal(serialized(), before) || !reflect.DeepEqual(stable, engine.Snapshot()) || !slices.EqualFunc(undo, engine.undo, bytes.Equal) || !slices.EqualFunc(redo, engine.redo, bytes.Equal) {
					t.Fatal("refusal or no-op changed document, revision or history")
				}
			}
			if _, err := engine.Redo(); err != nil || !bytes.Equal(serialized(), after) {
				t.Fatalf("redo lost exact formula: %v", err)
			}
		})
	}
}

func TestEngineColumnExpressionFooterAndAliasRefusalPreserveHistory(t *testing.T) {
	input, err := os.ReadFile("../../../fixtures/statement-1/input.folio")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock())
	if _, err := engine.Load(input); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Apply([]byte(`{"kind":"configureTableBinding","version":1,"id":"e8","collection":"transactions[]","alias":"row"}`)); err != nil {
		t.Fatal(err)
	}
	columns, _ := engine.TableColumns("e8")
	columnID := columns.Table.Columns[0].ID
	apply := func(command string) {
		t.Helper()
		if _, err := engine.Apply([]byte(command)); err != nil {
			t.Fatal(err)
		}
	}
	apply(fmt.Sprintf(`{"kind":"updateTableColumnFooter","version":1,"id":"e8","columnId":%q,"footer":"sum","footerOf":"","footerFormat":""}`, columnID))
	formula := fmt.Sprintf(`{"kind":"updateTableColumnExpression","version":1,"id":"e8","columnId":%q,"binding":"{{formatNumber(row.amount * 1.07, \"#,##0.00\")}}"}`, columnID)
	refuse := func(command string) {
		t.Helper()
		before, _, _ := engine.Serialize()
		stable := engine.Snapshot()
		undo, redo := append([][]byte(nil), engine.undo...), append([][]byte(nil), engine.redo...)
		if _, err := engine.Apply([]byte(command)); err == nil {
			t.Fatalf("invalid command accepted: %s", command)
		}
		after, _, _ := engine.Serialize()
		if !bytes.Equal(before, after) || !reflect.DeepEqual(stable, engine.Snapshot()) || !slices.EqualFunc(undo, engine.undo, bytes.Equal) || !slices.EqualFunc(redo, engine.redo, bytes.Equal) {
			t.Fatal("footer/alias refusal changed history")
		}
	}
	refuse(formula)
	refuse(fmt.Sprintf(`{"kind":"updateTableColumnExpression","version":1,"id":"e8","columnId":%q,"binding":""}`, columnID))
	apply(fmt.Sprintf(`{"kind":"updateTableColumnFooter","version":1,"id":"e8","columnId":%q,"footer":"sum","footerOf":"transactions.amount","footerFormat":""}`, columnID))
	apply(formula)
	refuse(`{"kind":"configureTableBinding","version":1,"id":"e8","collection":"transactions[]","alias":"txn"}`)
}
