package folio8

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio8-go/internal/geom"

	"github.com/panitw/folio8/folio8-go/internal/bind"
	"github.com/panitw/folio8/folio8-go/internal/expr"
	"github.com/panitw/folio8/folio8-go/internal/template"
)

func formulaTemplate(t *testing.T, condition string) *Template {
	t.Helper()
	encoded, _ := json.Marshal(condition)
	tpl, err := ParseTemplate([]byte(visTextTemplateJSON(`"visibleIf": ` + string(encoded) + `, `)))
	if err != nil {
		t.Fatal(err)
	}
	return tpl
}
func TestBooleanFormulasRenderThresholdAndLiteralConditions(t *testing.T) {
	for _, tc := range []struct {
		formula, data string
		want          int
	}{
		{"loanAmount>20000", `{"loanAmount":25000}`, 2}, {"loanAmount>20000", `{"loanAmount":20000}`, 1}, {"loanAmount>20000", `{"loanAmount":19999}`, 1},
		{"true", `{}`, 2}, {"false", `{}`, 1}, {"null", `{}`, 1}, {"(true)", `{}`, 2}, {"true", `{"true":false}`, 2},
		{"vip ? true : (blocked ? false : loanAmount>20000)", `{"vip":false,"blocked":false,"loanAmount":25000}`, 2},
		{"vip ? true : (blocked ? false : loanAmount>20000)", `{"vip":false,"blocked":true,"loanAmount":25000}`, 1},
		{"vip ? true : (blocked ? false : loanAmount>20000)", `{"vip":true}`, 2},
		{"loanAmount + fee > 20000", `{"loanAmount":19999,"fee":2}`, 2},
		{"true ? true : missingFlag", `{}`, 2},
	} {
		t.Run(tc.formula+tc.data, func(t *testing.T) {
			tpl := formulaTemplate(t, tc.formula)
			runs, _ := totalRunsImages(visBuildPages(t, tpl, tc.data))
			if runs != tc.want {
				t.Fatalf("runs=%d want %d", runs, tc.want)
			}
		})
	}
	tpl, err := ParseTemplate([]byte(visTextTemplateJSON(`"visibleIf": null, `)))
	if err != nil {
		t.Fatal(err)
	}
	runs, _ := totalRunsImages(visBuildPages(t, tpl, `{}`))
	if runs != 2 {
		t.Fatalf("JSON null hid element")
	}
}
func TestBooleanFormulaConditionCaveatsSurviveHiddenElementsOnce(t *testing.T) {
	for _, condition := range []string{"avg(items.amount)!=0", "avg(items.amount)!=null", "true ? avg(items.amount)!=null : false"} {
		tpl := formulaTemplate(t, condition)
		data, _ := bind.DecodeData([]byte(`{"items":[]}`))
		params, _ := decodeParams(nil)
		pages, _, _, diags, err := buildPageModel(tpl, data, params, testFontSet())
		if err != nil {
			t.Fatal(err)
		}
		wantRuns := 1
		if condition == "avg(items.amount)!=0" {
			wantRuns = 2
		}
		runs := 0
		for _, page := range pages {
			runs += len(page.Runs)
		}
		if runs != wantRuns {
			t.Fatalf("%s: runs=%d want %d", condition, runs, wantRuns)
		}
		count := 0
		for _, diag := range diags {
			if diag.Code == DiagCodeEmptyAverage {
				count++
				if diag.ElementID != "e1" {
					t.Fatalf("wrong location %+v", diag)
				}
			}
		}
		if count != 1 {
			t.Fatalf("%s: %+v", condition, diags)
		}
	}
	tpl := formulaTemplate(t, "true ? true : avg(items.amount)!=null")
	data, _ := bind.DecodeData([]byte(`{}`))
	params, _ := decodeParams(nil)
	_, _, _, diags, err := buildPageModel(tpl, data, params, testFontSet())
	if err != nil || len(diags) != 0 {
		t.Fatalf("unselected caveat: %+v %v", diags, err)
	}
}
func TestBooleanFormulaCommandErrorsKeepCauseAndAtomicity(t *testing.T) {
	tpl := formulaTemplate(t, "true")
	before, _ := SerializeTemplate(tpl)
	for _, condition := range []string{"loanAmount >", "flag ? 1 : true", "false ? upper(1) : true"} {
		raw, _ := json.Marshal(map[string]any{"kind": "updateComponentProperties", "version": 1, "ids": []string{"e1", "e2"}, "changes": map[string]any{"x": map[string]any{"op": "set", "value": 12}, "visibleIf": map[string]any{"op": "set", "value": condition}}})
		_, err := applyComponentCommand(tpl, raw)
		var renderErr *RenderError
		var located *expr.LocatedError
		if !errors.As(err, &renderErr) || !errors.As(err, &located) || renderErr.Diagnostic.Code != DiagCodeExpressionInvalid || renderErr.Diagnostic.ElementID != "e1" || renderErr.Diagnostic.DataPath != "visibleIf" {
			t.Fatalf("lost formula diagnostic: %v", err)
		}
		after, _ := SerializeTemplate(tpl)
		if !bytes.Equal(before, after) {
			t.Fatal("invalid formula partially applied properties")
		}
	}
	raw := []byte(`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"visibleIf":{"op":"null"}}}`)
	if _, err := applyComponentCommand(tpl, raw); err != nil {
		t.Fatal(err)
	}
}
func TestBooleanFormulaRuntimeErrorsKeepCodeFieldAndOffset(t *testing.T) {
	for _, tc := range []struct{ formula, data, code string }{
		{"amount>0", `{}`, DiagCodeBindingPathAbsent}, {"amount/0>0", `{"amount":1}`, DiagCodeExpressionInvalid}, {"amount>0", `{"amount":"x"}`, DiagCodeExpressionInvalid},
	} {
		tpl := formulaTemplate(t, tc.formula)
		_, err := Render(tpl, Data(tc.data), nil, testFontSet())
		var diagnosed *RenderError
		var located *expr.LocatedError
		if !errors.As(err, &diagnosed) || diagnosed.Diagnostic.Code != tc.code || !errors.As(err, &located) {
			t.Fatalf("%s: %v", tc.formula, err)
		}
	}
}
func TestBooleanFormulaVersionUsesAllExpressionContainers(t *testing.T) {
	for _, bandName := range []string{"pageHeader", "content", "pageFooter"} {
		for _, container := range []string{"visibleIf", "value", "column"} {
			tpl := formulaTemplate(t, "flag")
			el := tpl.doc.Bands.Content.Elements[0]
			el.VisibleIf = template.Presence[string]{}
			switch container {
			case "visibleIf":
				el.VisibleIf = template.Presence[string]{Set: true, Value: "true"}
			case "value":
				el.Value = template.Presence[string]{Set: true, Value: `{{true?"Yes":"No"}}`}
			case "column":
				el.Type = template.ElementTable
				el.Width = template.Presence[geom.Length]{}
				el.Value = template.Presence[string]{}
				el.Table = template.Presence[template.TableExt]{Set: true, Value: template.TableExt{Columns: []template.Column{{Bind: `{{null}}`}}}}
			}
			tpl.doc.Bands.Content.Elements = nil
			switch bandName {
			case "pageHeader":
				tpl.doc.Bands.PageHeader.Elements = []template.Element{el}
			case "content":
				tpl.doc.Bands.Content.Elements = []template.Element{el}
			case "pageFooter":
				tpl.doc.Bands.PageFooter.Elements = []template.Element{el}
			}
			saved, err := SerializeTemplate(tpl)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(saved, []byte(`"version": "2.0"`)) || tpl.doc.Version != "1.0" {
				t.Fatalf("%s %s: version requirement or source mutation", bandName, container)
			}
		}
	}
	for _, value := range []string{`{{"true ? 1+2 : null"}}`, `{{record.true}}`, `{{upper(name)}}`} {
		tpl := formulaTemplate(t, "flag")
		tpl.doc.Bands.Content.Elements[0].Value.Value = value
		saved, _ := SerializeTemplate(tpl)
		if !bytes.Contains(saved, []byte(`"version": "1.0"`)) {
			t.Fatalf("false version trigger %s", value)
		}
	}
	tpl := formulaTemplate(t, "true")
	tpl.doc.Version = "2.7"
	saved, _ := SerializeTemplate(tpl)
	if !bytes.Contains(saved, []byte(`"version": "2.7"`)) {
		t.Fatal("loaded version lowered")
	}
}
func TestBooleanFormulaReferencesPreviewAndFooterRestrictions(t *testing.T) {
	tpl := formulaTemplate(t, "params.enabled ? params.amount + 1 > 0 : params.fallback != null")
	refs, err := ParameterReferences(tpl)
	if err != nil || !reflect.DeepEqual(refs, []string{"amount", "enabled", "fallback"}) {
		t.Fatalf("refs=%v %v", refs, err)
	}
	for _, tc := range []struct{ formula, want string }{
		{"true", "{}"}, {"false", "{}"}, {"null", "{}"}, {"amount>20000", `{"amount":0}`}, {"amount/divisor>1", `{"amount":0,"divisor":1}`}, {"amount!=true", `{"amount":true}`}, {"amount != null", `{"amount":""}`}, {"amount/params.divisor>1", `{"amount":0}`},
	} {
		got, err := standInData(formulaTemplate(t, tc.formula))
		if err != nil || string(got) != tc.want {
			t.Fatalf("preview %s = %s %v", tc.formula, got, err)
		}
	}
	for _, formula := range []string{"amount/(divisor-divisor)>1", "flag ? amount>1 : upper(amount)!=null"} {
		_, err := standInData(formulaTemplate(t, formula))
		if err == nil || !strings.Contains(err.Error(), "sample data") {
			t.Fatalf("unsafe preview %s: %v", formula, err)
		}
	}
	for _, binding := range []string{`{{(row.amount)}}`, `{{formatNumber(row.amount+1,"0")}}`, `{{true ? row.amount : row.other}}`, `Prefix {{true ? row.amount : row.other}}`} {
		_, derived, err := expr.DeriveFooterOf(binding, "row", "items")
		if err != nil || derived {
			t.Fatalf("expanded footer syntax %s: %v", binding, err)
		}
		_, migrated, used, err := expr.RewriteRowBinding(binding, "row", "record")
		if err != nil || migrated || !used {
			t.Fatalf("alias use lost %s: %v %v %v", binding, migrated, used, err)
		}
	}
}

func TestBooleanFormulaTableBindingRoundTripAndRuntimeErrors(t *testing.T) {
	cols := `[{"id":"e2","label":"Status","width":100,"bind":"{{row.amount > 20000 ? \"High\" : \"Low\"}}"}]`
	raw := threeColumnTableDoc(`{"fontFamily":"latin"}`, cols)
	tpl, err := ParseTemplate([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(saved, []byte(`"version": "2.0"`)) {
		t.Fatal("table formula version missing")
	}
	loaded, err := ParseTemplate(saved)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ data, want string }{{`{"items":[{"amount":25000}]}`, "High"}, {`{"items":[{"amount":20000}]}`, "Low"}} {
		pages, _, _, _, err := buildPageModel(loaded, mustDecodeData(t, tc.data), mustDecodeParams(t), testShippedFontSet())
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, page := range pages {
			for _, run := range page.Runs {
				if run.SourceText == tc.want {
					found = true
				}
			}
		}
		if !found {
			t.Fatalf("missing %s table formula output", tc.want)
		}
	}
	for _, tc := range []struct{ data, code string }{{`{"items":[{}]}`, DiagCodeBindingPathAbsent}, {`{"items":[{"amount":"wrong"}]}`, DiagCodeExpressionInvalid}} {
		_, err := Render(loaded, Data(tc.data), nil, testShippedFontSet())
		var located *RenderError
		var cause *expr.LocatedError
		if !errors.As(err, &located) || located.Diagnostic.ElementID != "e2" || located.Diagnostic.Code != tc.code || !errors.As(err, &cause) {
			t.Fatalf("table error: %v", err)
		}
	}
	// A non-migratable formula's row alias must refuse atomically.
	before, _ := SerializeTemplate(loaded)
	_, err = applyComponentCommand(loaded, []byte(`{"kind":"configureTableBinding","version":1,"id":"e1","collection":"items[]","alias":"record"}`))
	after, _ := SerializeTemplate(loaded)
	if err == nil || !strings.Contains(err.Error(), "cannot migrate") || !bytes.Equal(before, after) {
		t.Fatalf("alias formula refusal was not atomic: %v", err)
	}
}

func TestBooleanFormulaReviewPreviewCandidatesAndLazyChecks(t *testing.T) {
	for _, tc := range []struct{ condition, value string }{
		{`x != (flag ? 1 : "a")`, ""},
		{`a != b`, `{{formatDate(a,"yyyy")}} {{formatNumber(b,"0")}}`},
		{`true ? true : 9223372036854775807 + 1 != 0`, ""},
	} {
		tpl := formulaTemplate(t, tc.condition)
		if tc.value != "" {
			tpl.doc.Bands.Content.Elements[0].Value.Value = tc.value
		}
		data, err := standInData(tpl)
		if err != nil {
			t.Fatalf("%s: %v", tc.condition, err)
		}
		if _, err := Render(tpl, Data(data), nil, testFontSet()); err != nil {
			t.Fatalf("preview %s with %s: %v", tc.condition, data, err)
		}
		if strings.HasPrefix(tc.condition, "x !=") && !bytes.Contains(data, []byte(`"x":null`)) {
			t.Fatalf("mixed kinds did not select null: %s", data)
		}
		if tc.condition == "a != b" && (!bytes.Contains(data, []byte(`"a":0`)) || !bytes.Contains(data, []byte(`"b":0`))) {
			t.Fatalf("common number candidate missing: %s", data)
		}
	}
	for _, tc := range []struct{ condition, value, field string }{
		{`1/(amount-amount)>0`, "", "visibleIf"},
		{`9223372036854775807 + 1 != 0`, "", "visibleIf"},
		{`true`, `{{formatNumber(1/(amount-amount),"0")}}`, "value"},
	} {
		tpl := formulaTemplate(t, tc.condition)
		if tc.value != "" {
			tpl.doc.Bands.Content.Elements[0].Value.Value = tc.value
		}
		_, err := standInData(tpl)
		if err == nil || !strings.Contains(err.Error(), "element e1 "+tc.field) || !strings.Contains(err.Error(), "supply sample data") {
			t.Fatalf("lost preview site: %v", err)
		}
	}
}

func TestBooleanFormulaReviewConditionWarningsKeepDocumentOrder(t *testing.T) {
	tpl := formulaTemplate(t, "true")
	body := func(path string) template.Presence[string] {
		return template.Presence[string]{Set: true, Value: "{{avg(" + path + ".amount)}}"}
	}
	condition := func(path string) template.Presence[string] {
		return template.Presence[string]{Set: true, Value: "avg(" + path + ".amount)!=0"}
	}
	e1 := &tpl.doc.Bands.Content.Elements[0]
	e1.Value = body("first")
	e2 := &tpl.doc.Bands.Content.Elements[1]
	e2.VisibleIf, e2.Value = condition("secondCondition"), body("second")
	header := *e1
	header.ID = "e3"
	header.VisibleIf, header.Value = condition("headerCondition"), body("header")
	footer := *e1
	footer.ID = "e4"
	footer.VisibleIf, footer.Value = condition("footerCondition"), body("footer")
	tpl.doc.Bands.PageHeader.Elements = []template.Element{header}
	tpl.doc.Bands.PageFooter.Elements = []template.Element{footer}
	data := mustDecodeData(t, `{"first":[],"secondCondition":[],"second":[],"headerCondition":[],"header":[],"footerCondition":[],"footer":[]}`)
	_, _, _, diags, err := buildPageModel(tpl, data, mustDecodeParams(t), testFontSet())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, diag := range diags {
		got = append(got, diag.ElementID+":"+diag.DataPath)
	}
	want := []string{"e3:headerCondition.amount", "e3:header.amount", "e1:first.amount", "e2:secondCondition.amount", "e2:second.amount", "e4:footerCondition.amount", "e4:footer.amount"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("warning order = %v, want %v", got, want)
	}
}

func TestBooleanFormulaReviewConsumerLocationsAndPlaceholderIdentity(t *testing.T) {
	for _, source := range []string{"  flag", "  true ? flag : false"} {
		tpl := formulaTemplate(t, source)
		_, err := Render(tpl, Data(`{"flag":1}`), nil, testFontSet())
		var located *expr.LocatedError
		if !errors.As(err, &located) || located.Offset < 2 {
			t.Fatalf("condition location: %v", err)
		}
	}
	for _, source := range []string{"  flag", "  true ? flag : null"} {
		// Booleans stay wrong-kind in text; a number now prints (2026-09-13).
		for _, value := range []string{"true", "false"} {
			tpl := formulaTemplate(t, "true")
			tpl.doc.Bands.Content.Elements[0].Value.Value = `{{"ok"}} {{` + source + `}}`
			_, err := Render(tpl, Data(`{"flag":`+value+`}`), nil, testFontSet())
			var located *expr.LocatedError
			if !errors.As(err, &located) || located.Offset < 2 || !strings.Contains(err.Error(), "placeholder 2") {
				t.Fatalf("text location/placeholder: %v", err)
			}
		}
	}
	tpl := formulaTemplate(t, "true")
	tpl.doc.Bands.Content.Elements[0].Value.Value = `{{"ok"}} {{ ` + strings.Repeat(" ", 600) + `flag >}}`
	serialized, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ParseTemplate(serialized)
	var located *expr.LocatedError
	if !errors.As(err, &located) || !strings.Contains(err.Error(), "placeholder 2") || !strings.Contains(err.Error(), "unexpected end") {
		t.Fatalf("load placeholder identity: %v", err)
	}
}

func TestBooleanFormulaReviewMissingPathRetainsConsumerField(t *testing.T) {
	for _, field := range []string{"visibleIf", "value", "bind"} {
		tpl := formulaTemplate(t, "true")
		data := `{}`
		switch field {
		case "visibleIf":
			tpl.doc.Bands.Content.Elements[0].VisibleIf.Value = "missing"
		case "value":
			tpl.doc.Bands.Content.Elements[0].Value.Value = "{{missing}}"
		case "bind":
			var err error
			tpl, err = ParseTemplate([]byte(threeColumnTableDoc(`{"fontFamily":"latin"}`, `[{"id":"e2","label":"Status","width":100,"bind":"{{missing}}"}]`)))
			if err != nil {
				t.Fatal(err)
			}
			data = `{"items":[{}]}`
		}
		_, err := Render(tpl, Data(data), nil, testShippedFontSet())
		var diagnostic *RenderError
		if !errors.As(err, &diagnostic) || diagnostic.Diagnostic.Code != DiagCodeBindingPathAbsent || diagnostic.Diagnostic.DataPath != "missing" || !strings.Contains(diagnostic.Diagnostic.Message, field+":") {
			t.Fatalf("%s missing-path context: %v", field, err)
		}
	}
}
