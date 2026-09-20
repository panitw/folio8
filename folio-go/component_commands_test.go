package folio8

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/panitw/folio8/folio-go/internal/designer"
	"github.com/panitw/folio8/folio-go/internal/fontset"
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/template"
)

func TestBindTableCollectionPreservesEverythingElse(t *testing.T) {
	input, err := os.ReadFile("../fixtures/statement-1/input.folio")
	if err != nil {
		t.Fatal(err)
	}
	for _, alias := range []string{"explicit", "absent"} {
		t.Run(alias, func(t *testing.T) {
			source := input
			if alias == "absent" {
				source = bytes.Replace(source, []byte(`, "as": "txn"`), nil, 1)
				source = bytes.ReplaceAll(source, []byte("txn."), []byte("row."))
			}
			for _, segments := range [][]string{{"items"}, {"report", "transactions"}, {strings.Repeat("a", 65)}, {strings.Repeat("a", 120)}, {strings.Repeat("a", 254)}, strings.Split(strings.Repeat("a.", 126)+"a", "."), {strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("d", 59)}} {
				tpl, err := ParseTemplate(source)
				if err != nil {
					t.Fatal(err)
				}
				before, _ := SerializeTemplate(tpl)
				keys, _ := json.Marshal(segments)
				projection, err := applyComponentCommand(tpl, []byte(`{"kind":"bindTableCollection","version":1,"id":"e8","segments":`+string(keys)+`}`))
				if err != nil {
					t.Fatal(err)
				}
				collection := strings.Join(segments, ".") + "[]"
				if got := canvasComponentByID(projection, "e8"); got == nil || got.TableBind == nil || *got.TableBind != collection {
					t.Fatalf("collection projection = %#v, want %q", got, collection)
				}
				after, _ := SerializeTemplate(tpl)
				want := bytes.Replace(before, []byte(`"bind": "transactions[]"`), []byte(`"bind": "`+collection+`"`), 1)
				if !bytes.Equal(after, want) {
					t.Fatalf("collection pick changed other document bytes: %s", after)
				}
				if _, err := tableColumns(tpl, "e8"); err != nil {
					t.Fatalf("collection with derived/absent footer sources became uneditable: %v", err)
				}
				configured, _ := ParseTemplate(source)
				aliasValue := "txn"
				if alias == "absent" {
					aliasValue = ""
				}
				if _, err := applyComponentCommand(configured, tableCollectionEditCommand("configureTableBinding", "e8", collection, aliasValue)); err != nil {
					t.Fatalf("Configure columns rejected picker-admitted collection %q: %v", collection, err)
				}
				configuredBytes, _ := SerializeTemplate(configured)
				if !bytes.Equal(after, configuredBytes) {
					t.Fatal("picker and Configure columns produced different documents")
				}
			}
		})
	}
}

func TestBindTableCollectionRefusalsAreTransactional(t *testing.T) {
	input, err := os.ReadFile("../fixtures/statement-1/input.folio")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, id, segments string }{
		{"missing", "e8", ""}, {"null", "e8", "null"}, {"empty", "e8", "[]"},
		{"scalar", "e8", `"items"`}, {"typed", "e8", `[42]`}, {"null-key", "e8", `[null]`},
		{"dot", "e8", `["a.b"]`}, {"empty-key", "e8", `[""]`},
		{"unicode", "e8", `["สวัสดี"]`}, {"control", "e8", `["line\nbreak"]`},
		{"array-index", "e8", `["items[0]","rows"]`}, {"array-marker", "e8", `["items[]"]`},
		{"params", "e8", `["params"]`}, {"params-child", "e8", `["params","items"]`},
		{"long-key", "e8", `["` + strings.Repeat("a", 255) + `"]`},
		{"long-path", "e8", `["` + strings.Repeat("a", 64) + `","` + strings.Repeat("b", 64) + `","` + strings.Repeat("c", 64) + `","` + strings.Repeat("d", 60) + `"]`},
		{"long-segmented-path", "e8", `["a"` + strings.Repeat(`,"a"`, 127) + `]`},
		{"missing-table", "ez", `["items"]`}, {"text", "e5", `["items"]`}, {"image", "e1", `["items"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tpl, err := ParseTemplate(input)
			if err != nil {
				t.Fatal(err)
			}
			before, _ := SerializeTemplate(tpl)
			command := `{"kind":"bindTableCollection","version":1,"id":"` + tc.id + `"`
			if tc.segments != "" {
				command += `,"segments":` + tc.segments
			}
			command += `}`
			if _, err := applyComponentCommand(tpl, []byte(command)); err == nil {
				t.Fatalf("unsupported collection succeeded: %s", command)
			} else if tc.segments != "" {
				var failure *designer.ComponentCommandError
				if !errors.As(err, &failure) || failure.ElementID != tc.id || (tc.id == "e8" && failure.DataPath != "table.collection") {
					t.Fatalf("collection refusal was not located at its table and field: %v", err)
				}
			}
			after, _ := SerializeTemplate(tpl)
			if !bytes.Equal(before, after) {
				t.Fatal("refusal changed canonical document")
			}
		})
	}
	for _, extra := range []string{`,"alias":"new"`, `,"collection":"other[]"`, `,"columnId":"e9"`} {
		tpl, _ := ParseTemplate(input)
		before, _ := SerializeTemplate(tpl)
		if _, err := applyComponentCommand(tpl, []byte(`{"kind":"bindTableCollection","version":1,"id":"e8","segments":["items"]`+extra+`}`)); err == nil {
			t.Fatalf("extra collection command field succeeded: %s", extra)
		}
		after, _ := SerializeTemplate(tpl)
		if !bytes.Equal(before, after) {
			t.Fatal("extra-field refusal changed canonical document")
		}
	}
}

func tableCollectionEditCommand(kind, id, collection, alias string) []byte {
	command := map[string]any{"kind": kind, "version": 1, "id": id}
	if kind == "bindTableCollection" {
		command["segments"] = strings.Split(strings.TrimSuffix(collection, "[]"), ".")
	} else {
		command["collection"], command["alias"] = collection, alias
	}
	encoded, _ := json.Marshal(command)
	return encoded
}

func TestTableCollectionCommandsPreserveRelativeFooterSources(t *testing.T) {
	input, err := os.ReadFile("testdata/commands/table-collection-footers.folio")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"bindTableCollection", "configureTableBinding"} {
		for _, alias := range []string{"txn", ""} {
			for _, collection := range []string{"items[]", "report.entries[]", "account.transactions[]"} {
				t.Run(kind+"/"+alias+"/"+collection, func(t *testing.T) {
					source := input
					if alias == "" {
						source = bytes.Replace(source, []byte(`, "as": "txn"`), nil, 1)
						source = bytes.ReplaceAll(source, []byte("txn."), []byte("row."))
					}
					tpl, err := ParseTemplate(source)
					if err != nil {
						t.Fatal(err)
					}
					before, _ := SerializeTemplate(tpl)
					if _, err := applyComponentCommand(tpl, tableCollectionEditCommand(kind, "e1", collection, alias)); err != nil {
						t.Fatal(err)
					}
					after, _ := SerializeTemplate(tpl)
					want := bytes.Replace(before, []byte(`"bind": "account.transactions[]"`), []byte(`"bind": "`+collection+`"`), 1)
					want = bytes.ReplaceAll(want, []byte(`"footerOf": "account.transactions.`), []byte(`"footerOf": "`+strings.TrimSuffix(collection, "[]")+`.`))
					if !bytes.Equal(after, want) {
						t.Fatalf("collection change modified more than collection/footer prefixes: %s", after)
					}
					view, err := tableColumns(tpl, "e1")
					if err != nil || view.Columns[0].Footer != "sum" || view.Columns[1].Footer != "avg" || view.Columns[2].FooterOf != "" || view.Columns[3].FooterOf != "" || view.Columns[4].FooterOf != "" {
						t.Fatalf("footer sources lost their explicit/derived/absent forms: %#v, %v", view, err)
					}
				})
			}
		}
	}
}

func TestTableCollectionFooterSourceBoundIsTransactional(t *testing.T) {
	input, err := os.ReadFile("testdata/commands/table-collection-footers.folio")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"bindTableCollection", "configureTableBinding"} {
		for _, tc := range []struct {
			name       string
			rootLength int
			shortFirst bool
			accept     bool
		}{
			{"exact-source-bound", 245, false, true},
			{"source-overflow", 246, false, false},
			{"later-source-overflow", 248, true, false},
		} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				source := input
				if tc.shortFirst {
					source = bytes.Replace(source, []byte("account.transactions.totals.net"), []byte("account.transactions.net"), 1)
				}
				tpl, err := ParseTemplate(source)
				if err != nil {
					t.Fatal(err)
				}
				before, _ := SerializeTemplate(tpl)
				collection := strings.Repeat("a", tc.rootLength) + "[]"
				_, err = applyComponentCommand(tpl, tableCollectionEditCommand(kind, "e1", collection, "txn"))
				if tc.accept {
					if err != nil {
						t.Fatal(err)
					}
					view, err := tableColumns(tpl, "e1")
					if err != nil || len(view.Columns[0].FooterOf) != 256 {
						t.Fatalf("exact source bound did not stay editable: %#v, %v", view, err)
					}
				} else {
					var failure *designer.ComponentCommandError
					if !errors.As(err, &failure) || failure.ElementID != "e1" || failure.DataPath != "column.footerOf" {
						t.Fatalf("over-limit source lacks a located refusal: %v", err)
					}
					after, _ := SerializeTemplate(tpl)
					if !bytes.Equal(after, before) {
						t.Fatal("source overflow changed the document")
					}
				}
			})
		}
	}
}

func componentTemplate(t *testing.T) *Template {
	t.Helper()
	b, err := os.ReadFile("testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := ParseTemplate(b)
	if err != nil {
		t.Fatal(err)
	}
	return tpl
}

func newProjectedComponent(t *testing.T, before, after designer.CanvasProjection) designer.CanvasComponent {
	t.Helper()
	known := map[string]bool{}
	for _, component := range before.Components {
		known[component.ID] = true
	}
	for _, component := range after.Components {
		if !known[component.ID] {
			return component
		}
	}
	t.Fatal("command projection did not add a component")
	return designer.CanvasComponent{}
}

func pointLiteral(value int64) string {
	whole, fraction := value/1000, value%1000
	if fraction == 0 {
		return strconv.FormatInt(whole, 10)
	}
	return strconv.FormatInt(whole, 10) + "." + strings.TrimRight(fmt.Sprintf("%03d", fraction), "0")
}

func TestComponentCommandsCreateAllClosedKindsAndKeepOrder(t *testing.T) {
	tpl := componentTemplate(t)
	for _, kind := range []string{"text", "image", "table", "line", "rect"} {
		before, err := canvas(tpl)
		if err != nil {
			t.Fatal(err)
		}
		projection, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"`+kind+`","band":"content","x":12,"y":12,"width":72,"height":24,"snap":false}`))
		if err != nil {
			t.Fatalf("create %s: %v", kind, err)
		}
		component := newProjectedComponent(t, before, projection)
		if component.Type != kind || component.Band != "content" || component.ID == "" {
			t.Fatalf("create %s projection = %#v", kind, component)
		}
		if kind == "table" && (component.Resizable || component.X != 0 || component.Width != projectedBands(t, tpl)["content"].Width || component.Height != 24000) {
			t.Fatalf("table projection must be derived and non-resizable: %#v", component)
		}
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"widget","band":"content","x":12,"y":12,"width":72,"height":24,"snap":false}`)); err == nil {
		t.Fatal("sixth component kind unexpectedly succeeded")
	}
}

func TestComponentCommandsSnapContainAndFailureAreTransactional(t *testing.T) {
	tpl := componentTemplate(t)
	before, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	beforeProjection, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"text","band":"content","x":3,"y":3,"width":72,"height":24,"snap":true}`)); err != nil {
		t.Fatal(err)
	}
	projection, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	created := newProjectedComponent(t, beforeProjection, projection)
	if created.X != 6000 || created.Y != 6000 {
		t.Fatalf("snap = %#v, want 6000 millipoints", created)
	}
	afterCreate, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, afterCreate) {
		t.Fatal("successful component command did not change canonical bytes")
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"moveComponent","version":1,"id":"`+created.ID+`","x":999999,"y":0,"snap":false}`)); err == nil {
		t.Fatal("out-of-band move unexpectedly succeeded")
	}
	afterFailure, err := SerializeTemplate(tpl)
	if err != nil || !bytes.Equal(afterCreate, afterFailure) {
		t.Fatalf("failed command changed canonical bytes: %v", err)
	}
}

func TestBindComponentScalarOwnsRootExpressionAndPaintProjection(t *testing.T) {
	tpl := componentTemplate(t)
	before, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"bindComponentScalar","version":1,"id":"e1","segments":["customer","name"]}`))
	if err != nil {
		t.Fatal(err)
	}
	component := componentByID(t, projection, "e1")
	if component.Binding == nil || *component.Binding != "customer.name" {
		t.Fatalf("binding paint = %#v, want customer.name", component.Binding)
	}
	canonical, err := SerializeTemplate(tpl)
	if err != nil || bytes.Equal(before, canonical) || !bytes.Contains(canonical, []byte(`"value": "{{customer.name}}"`)) {
		t.Fatalf("canonical scalar binding = %s, err=%v", canonical, err)
	}
	for _, command := range [][]byte{
		[]byte(`{"kind":"bindComponentScalar","version":1,"id":"e1","segments":["params","name"]}`),
		[]byte(`{"kind":"bindComponentScalar","version":1,"id":"e1","segments":["page"]}`),
		[]byte(`{"kind":"bindComponentScalar","version":1,"id":"e2","segments":["customer","name"]}`),
		[]byte(`{"kind":"bindComponentScalar","version":1,"id":"e1","segments":["not-valid"]}`),
	} {
		if _, err := applyComponentCommand(tpl, command); err == nil {
			t.Fatalf("invalid scalar bind unexpectedly succeeded: %s", command)
		}
		after, err := SerializeTemplate(tpl)
		if err != nil || !bytes.Equal(canonical, after) {
			t.Fatalf("rejected scalar bind changed canonical bytes: %s, err=%v", command, err)
		}
	}
}

func TestPlacedImageStartsEmptyAndSurvivesTheRoundTripAndRender(t *testing.T) {
	tpl := componentTemplate(t)
	before, _ := canvas(tpl)
	placed, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"image","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	// The box is placed but unfilled: nothing to paint, and no reason given,
	// because nothing has gone wrong — the author simply has not chosen a
	// file. That is what tells the designer to draw its empty placeholder
	// rather than one of ImageUnavailable's two failure texts.
	component := newProjectedComponent(t, before, placed)
	if component.Image != nil || component.ImageUnavailable != nil {
		t.Fatalf("placed image projected image=%#v unavailable=%s, want an empty box", component.Image, describeStringPtr(component.ImageUnavailable))
	}
	canonical, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	// No stowaway: an empty box embeds no bytes anywhere in the document.
	if !bytes.Contains(canonical, []byte(`"asset": null`)) {
		t.Fatalf("canonical bytes do not declare the empty asset: %s", canonical)
	}
	if !bytes.Contains(canonical, []byte(`"assets": {}`)) {
		t.Fatalf("placing an empty image added a document asset: %s", canonical)
	}
	// The canonical bytes still load, and the element is still an image.
	reloaded, err := ParseTemplate(canonical)
	if err != nil {
		t.Fatalf("a document with an empty image box did not load: %v", err)
	}
	again, err := SerializeTemplate(reloaded)
	if err != nil || !bytes.Equal(canonical, again) {
		t.Fatalf("empty image box did not round-trip: err=%v", err)
	}
	// Render draws nothing for it and completes without a caveat: an unfilled
	// box is an authoring state, not a defect in the document.
	data, err := os.ReadFile(filepath.Join("..", "fixtures", "statement-1", "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Render(reloaded, Data(data), Params(`{"generatedDate":"2026-08-27"}`), testShippedFontSet())
	if err != nil {
		t.Fatalf("a document with an empty image box did not render: %v", err)
	}
	// ⚠ THIS EXPECTATION MOVED AT STORY 11.2, AND IT MOVED FOR A REASON
	// THAT IS NOT ABOUT IMAGES. `worked-example.json` — the fixture
	// componentTemplate builds from — declares `style.bold` on element
	// `e1`, against a `body` chain of bare face names that declares no
	// bold variant. Until Story 11.2 `style.bold` was read by nothing
	// that draws, so the render said nothing about it. It is read now,
	// and an entry with no declared variant renders in its own base face
	// and SAYS SO — one TEXT_STYLE_FACE_UNDECLARED Warning per distinct
	// rune of that element (FR57, AC3).
	//
	// So the assertion is narrowed rather than deleted: this test is
	// about an unfilled image box, and what it must still prove is that
	// the EMPTY BOX contributes nothing. Every diagnostic present must be
	// the pre-existing bold condition on e1; a diagnostic from any other
	// element, or of any other code, is the failure this line was written
	// to catch and still catches.
	for _, d := range result.Diagnostics {
		if d.Code == DiagCodeTextStyleFaceUndeclared && d.ElementID == "e1" {
			continue
		}
		t.Fatalf("empty image box reported diagnostics beyond e1's declared-bold Warning: %#v", result.Diagnostics)
	}
	// THE EXACT COUNT, not merely "some". AC3 coalesces to one Warning
	// per (element, DISTINCT RUNE), so e1's own text fixes this number —
	// and asserting only "more than zero" would let a duplicate-warning
	// regression (one per occurrence, or one per row) pass here.
	// Re-derived from the fixture rather than written down, so it moves
	// with the fixture's text and not with a maintainer's memory.
	wantWarnings := 0
	seen := map[rune]bool{}
	for _, r := range workedExampleElementText(t, "e1") {
		if !seen[r] {
			seen[r] = true
			wantWarnings++
		}
	}
	if wantWarnings == 0 {
		t.Fatal("fixture precondition: element e1 has no text, so it can earn no Warning and the assertion above proves nothing")
	}
	if len(result.Diagnostics) != wantWarnings {
		t.Fatalf("empty image box render reported %d diagnostics, want exactly %d — one per distinct rune of e1's bold text, never one per occurrence: %#v",
			len(result.Diagnostics), wantWarnings, result.Diagnostics)
	}
}

// workedExampleElementText reads one element's authored `value` out of
// the golden source, so the expected Warning count above is DERIVED from
// the fixture rather than pinned to a number that goes stale silently.
func workedExampleElementText(t *testing.T, id string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "template", "golden", "worked-example.json"))
	if err != nil {
		t.Fatalf("read golden source: %v", err)
	}
	tpl, err := ParseTemplate(b)
	if err != nil {
		t.Fatalf("parse golden source: %v", err)
	}
	for _, band := range []([]template.Element){
		tpl.doc.Bands.PageHeader.Elements,
		tpl.doc.Bands.Content.Elements,
		tpl.doc.Bands.PageFooter.Elements,
	} {
		for _, el := range band {
			if string(el.ID) == id {
				return el.Value.Value
			}
		}
	}
	t.Fatalf("fixture precondition: worked-example.json has no element %q", id)
	return ""
}

func TestBindComponentScalarPreservesDecodedSegmentsAndRejectsTypedBindings(t *testing.T) {
	tpl := componentTemplate(t)
	before, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	// This is legal command grammar even though a picker would withhold an
	// observed collection. D-6.2.1 keeps sample runtime kind out of command
	// legality; AD-14 reports any incompatible runtime value later.
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"bindComponentScalar","version":1,"id":"e1","segments":["items"]}`)); err != nil {
		t.Fatalf("sample-independent collection-shaped path was rejected: %v", err)
	}
	collectionCanonical, err := SerializeTemplate(tpl)
	if err != nil || !bytes.Contains(collectionCanonical, []byte(`"value": "{{items}}"`)) {
		t.Fatalf("collection-shaped path was not canonically retained: %s, err=%v", collectionCanonical, err)
	}
	_, renderErr := Render(tpl, Data(`{"items":[],"transactions":[]}`), Params(`{}`), testShippedFontSet())
	if renderErr == nil || !strings.Contains(renderErr.Error(), "element e1") || !strings.Contains(renderErr.Error(), `"items"`) || !strings.Contains(renderErr.Error(), "array") {
		t.Fatalf("runtime collection kind must use the AD-14 located diagnostic, got %v", renderErr)
	}
	for _, command := range [][]byte{
		[]byte(`{"kind":"bindComponentScalar","version":1,"id":"e1","segments":["a.b"]}`),
		[]byte(`{"kind":"bindComponentScalar","version":1,"id":"e1","segments":["line\\nbreak"]}`),
		[]byte(`{"kind":"bindComponentScalar","version":1,"id":"e1","segments":["สวัสดี"]}`),
		[]byte(`{"kind":"bindComponentScalar","version":1,"id":"e1","segments":[""]}`),
		[]byte(`{"kind":"bindComponentScalar","version":1,"id":"e1","segments":["params","name"]}`),
		[]byte(`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"value":{"op":"set","value":"{{customer.name}}"}}}`),
	} {
		if _, err := applyComponentCommand(tpl, command); err == nil {
			t.Fatalf("ambiguous or typed binding unexpectedly succeeded: %s", command)
		}
		after, err := SerializeTemplate(tpl)
		if err != nil || !bytes.Equal(collectionCanonical, after) {
			t.Fatalf("rejected command changed canonical bytes: %s, err=%v", command, err)
		}
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"value":{"op":"set","value":"literal text"}}}`)); err != nil {
		t.Fatalf("literal text edit was rejected: %v", err)
	}
	after, err := SerializeTemplate(tpl)
	if err != nil || bytes.Equal(before, after) || !bytes.Contains(after, []byte(`"value": "literal text"`)) {
		t.Fatalf("literal edit was not canonical: %s, err=%v", after, err)
	}
}

func componentByID(t *testing.T, projection designer.CanvasProjection, id string) designer.CanvasComponent {
	t.Helper()
	for _, component := range projection.Components {
		if component.ID == id {
			return component
		}
	}
	t.Fatalf("component %q is absent from projection", id)
	return designer.CanvasComponent{}
}

func TestComponentCommandsRejectTableResizeAndPreserveTableGeometry(t *testing.T) {
	tpl := componentTemplate(t)
	beforeProjection, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	table := newProjectedComponent(t, beforeProjection, projection)
	before, _ := SerializeTemplate(tpl)
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"resizeComponent","version":1,"id":"`+table.ID+`","width":72,"height":24,"snap":false}`)); err == nil || !strings.Contains(err.Error(), "derived geometry") {
		t.Fatalf("table resize error = %v", err)
	}
	after, _ := SerializeTemplate(tpl)
	if !bytes.Equal(before, after) || strings.Contains(string(after), `"width": 72`) {
		t.Fatalf("table resize changed canonical state: %s", after)
	}
}

// removeStarterColumn keeps existing point-table fixtures deliberately empty.
// New proportional behavior is exercised by table_proportions_test.go.
func removeStarterColumn(t *testing.T, tpl *Template, id string) {
	t.Helper()
	view, err := tableColumns(tpl, id)
	if err != nil || len(view.Columns) != 1 {
		t.Fatalf("starter column = %#v, err=%v", view, err)
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"removeTableColumn","version":1,"id":"`+id+`","columnId":"`+view.Columns[0].ID+`"}`)); err != nil {
		t.Fatal(err)
	}
	_, _, _, element, _ := findComponent(tpl, id)
	element.Width = template.Presence[geom.Length]{} // point fixture

}

func TestTableColumnCommandsAreClosedCanonicalAndDerived(t *testing.T) {
	tpl := componentTemplate(t)
	before, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	table := newProjectedComponent(t, before, projection)
	removeStarterColumn(t, tpl, table.ID)
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"addTableColumn","version":1,"id":"`+table.ID+`","index":0}`)); err != nil {
		t.Fatal(err)
	}
	view, err := tableColumns(tpl, table.ID)
	if err != nil || len(view.Columns) != 1 || view.Columns[0].Width != 72000 || view.Columns[0].Align != "left" || !view.Columns[0].RowFieldEditable {
		t.Fatalf("column projection = %#v, err=%v", view, err)
	}
	column := view.Columns[0]
	for _, command := range [][]byte{
		[]byte(`{"kind":"updateTableColumn","version":1,"id":"` + table.ID + `","columnId":"` + column.ID + `","field":"header","value":"Amount"}`),
		[]byte(`{"kind":"updateTableColumn","version":1,"id":"` + table.ID + `","columnId":"` + column.ID + `","field":"width","value":96}`),
		[]byte(`{"kind":"updateTableColumn","version":1,"id":"` + table.ID + `","columnId":"` + column.ID + `","field":"align","value":"right"}`),
		[]byte(`{"kind":"addTableColumn","version":1,"id":"` + table.ID + `","index":1}`),
	} {
		if _, err := applyComponentCommand(tpl, command); err != nil {
			t.Fatalf("apply %s: %v", command, err)
		}
	}
	view, err = tableColumns(tpl, table.ID)
	if err != nil || len(view.Columns) != 2 || view.Columns[0].Header != "Amount" || view.Columns[0].Width != 96000 || view.Columns[0].Align != "right" {
		t.Fatalf("edited projection = %#v, err=%v", view, err)
	}
	canvas, err := canvas(tpl)
	if err != nil || componentByID(t, canvas, table.ID).Width != 168000 {
		t.Fatalf("derived table width = %#v, err=%v", canvas, err)
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"moveTableColumn","version":1,"id":"`+table.ID+`","columnId":"`+view.Columns[1].ID+`","toIndex":0}`)); err != nil {
		t.Fatal(err)
	}
	view, _ = tableColumns(tpl, table.ID)
	if view.Columns[0].ID == column.ID {
		t.Fatal("move did not preserve engine ordered columns")
	}
	canonical, err := SerializeTemplate(tpl)
	if err != nil || bytes.Contains(canonical, []byte(`"tableWidth"`)) {
		t.Fatalf("canonical table geometry leaked a width: %s, err=%v", canonical, err)
	}
}

func TestTableColumnRejectionsDoNotMutate(t *testing.T) {
	tpl := componentTemplate(t)
	before, _ := canvas(tpl)
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	table := newProjectedComponent(t, before, projection)
	removeStarterColumn(t, tpl, table.ID)
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"addTableColumn","version":1,"id":"`+table.ID+`","index":0}`)); err != nil {
		t.Fatal(err)
	}
	view, _ := tableColumns(tpl, table.ID)
	canonical, _ := SerializeTemplate(tpl)
	for _, command := range [][]byte{
		[]byte(`{"kind":"updateTableColumn","version":1,"id":"` + table.ID + `","columnId":"` + view.Columns[0].ID + `","field":"width","value":0}`),
		[]byte(`{"kind":"updateTableColumn","version":1,"id":"` + table.ID + `","columnId":"` + view.Columns[0].ID + `","field":"align","value":"justify"}`),
		[]byte(`{"kind":"removeTableColumn","version":1,"id":"` + table.ID + `","columnId":"missing"}`),
		[]byte(`{"kind":"addTableColumn","version":1,"id":"` + table.ID + `","index":3}`),
	} {
		if _, err := applyComponentCommand(tpl, command); err == nil {
			t.Fatalf("invalid command succeeded: %s", command)
		}
		after, _ := SerializeTemplate(tpl)
		if !bytes.Equal(canonical, after) {
			t.Fatalf("rejection mutated canonical bytes: %s", command)
		}
	}
}

func TestTableColumnCommandsAreTransactionalAtThePublicSeam(t *testing.T) {
	tpl := componentTemplate(t)
	before, _ := canvas(tpl)
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":500,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	table := newProjectedComponent(t, before, projection)
	removeStarterColumn(t, tpl, table.ID)
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"moveComponent","version":1,"id":"`+table.ID+`","x":500,"y":0,"snap":false}`)); err != nil {
		t.Fatal(err)
	}
	canonical, _ := SerializeTemplate(tpl)
	// addTableColumn reaches containment only after adding the candidate column;
	// direct callers must still retain their original canonical template.
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"addTableColumn","version":1,"id":"`+table.ID+`","index":0}`)); err == nil {
		t.Fatal("out-of-band add unexpectedly succeeded")
	}
	after, _ := SerializeTemplate(tpl)
	if !bytes.Equal(canonical, after) {
		t.Fatal("rejected add mutated the public caller template")
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"addTableColumn","version":1,"id":"`+table.ID+`","index":0}`)); err == nil {
		t.Fatal("a rejected candidate consumed an id or changed later command behavior")
	}
}

func TestTableColumnProjectionCapRejectsThe129thCommandWithoutMutation(t *testing.T) {
	tpl := componentTemplate(t)
	before, _ := canvas(tpl)
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	table := newProjectedComponent(t, before, projection)
	removeStarterColumn(t, tpl, table.ID)
	for index := 0; index < maxTableColumns; index++ {
		if _, err := applyComponentCommand(tpl, []byte(`{"kind":"addTableColumn","version":1,"id":"`+table.ID+`","index":`+strconv.Itoa(index)+`}`)); err != nil {
			t.Fatalf("add %d: %v", index+1, err)
		}
		view, err := tableColumns(tpl, table.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := applyComponentCommand(tpl, []byte(`{"kind":"updateTableColumn","version":1,"id":"`+table.ID+`","columnId":"`+view.Columns[index].ID+`","field":"width","value":0.001}`)); err != nil {
			t.Fatalf("shrink %d: %v", index+1, err)
		}
	}
	view, err := tableColumns(tpl, table.ID)
	if err != nil || len(view.Columns) != maxTableColumns {
		t.Fatalf("128-column projection = %#v, err=%v", view, err)
	}
	canonical, _ := SerializeTemplate(tpl)
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"addTableColumn","version":1,"id":"`+table.ID+`","index":128}`)); err == nil {
		t.Fatal("129th editor-unprojectable column unexpectedly succeeded")
	}
	after, _ := SerializeTemplate(tpl)
	if !bytes.Equal(canonical, after) {
		t.Fatal("129th column rejection mutated canonical bytes")
	}
}

func TestTableDataBindingAndFooterCommandsAreCanonicalAndTransactional(t *testing.T) {
	tpl := componentTemplate(t)
	before, _ := canvas(tpl)
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	table := newProjectedComponent(t, before, projection)
	removeStarterColumn(t, tpl, table.ID)
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"addTableColumn","version":1,"id":"`+table.ID+`","index":0}`)); err != nil {
		t.Fatal(err)
	}
	view, err := tableColumns(tpl, table.ID)
	if err != nil {
		t.Fatal(err)
	}
	column := view.Columns[0]
	for _, command := range [][]byte{
		[]byte(`{"kind":"configureTableBinding","version":1,"id":"` + table.ID + `","collection":"transactions[]","alias":"transaction"}`),
		[]byte(`{"kind":"updateTableColumnBinding","version":1,"id":"` + table.ID + `","columnId":"` + column.ID + `","field":"amount"}`),
		[]byte(`{"kind":"updateTableColumnFooter","version":1,"id":"` + table.ID + `","columnId":"` + column.ID + `","footer":"sum","footerOf":"","footerFormat":""}`),
	} {
		if _, err := applyComponentCommand(tpl, command); err != nil {
			t.Fatalf("apply %s: %v", command, err)
		}
	}
	view, err = tableColumns(tpl, table.ID)
	if err != nil || view.Collection != "transactions[]" || view.Alias != "transaction" || view.Columns[0].Binding != "{{transaction.amount}}" || view.Columns[0].Footer != "sum" {
		t.Fatalf("data projection = %#v, err=%v", view, err)
	}
	canonical, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(canonical, []byte(`"as": "transaction"`)) || !bytes.Contains(canonical, []byte(`"bind": "{{transaction.amount}}"`)) || !bytes.Contains(canonical, []byte(`"footer": "sum"`)) {
		t.Fatalf("canonical data table omitted configured fields: %s", canonical)
	}
	if _, err := ParseTemplate(canonical); err != nil {
		t.Fatalf("canonical reload: %v", err)
	}
	for _, rejected := range [][]byte{
		[]byte(`{"kind":"configureTableBinding","version":1,"id":"` + table.ID + `","collection":"params.items[]","alias":"row"}`),
		[]byte(`{"kind":"updateTableColumnBinding","version":1,"id":"` + table.ID + `","columnId":"` + column.ID + `","field":"bare row"}`),
		[]byte(`{"kind":"updateTableColumnFooter","version":1,"id":"` + table.ID + `","columnId":"` + column.ID + `","footer":"count","footerOf":"transactions.amount","footerFormat":""}`),
	} {
		if _, err := applyComponentCommand(tpl, rejected); err == nil {
			t.Fatalf("rejected command succeeded: %s", rejected)
		}
		after, _ := SerializeTemplate(tpl)
		if !bytes.Equal(canonical, after) {
			t.Fatalf("rejection changed canonical bytes: %s", rejected)
		}
	}
}

func TestTableAliasMigrationReservedRootsAndStrictEnvelope(t *testing.T) {
	tpl := componentTemplate(t)
	before, _ := canvas(tpl)
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	table := newProjectedComponent(t, before, projection)
	removeStarterColumn(t, tpl, table.ID)
	for index := 0; index < 3; index++ {
		if _, err := applyComponentCommand(tpl, []byte(`{"kind":"addTableColumn","version":1,"id":"`+table.ID+`","index":`+strconv.Itoa(index)+`}`)); err != nil {
			t.Fatal(err)
		}
	}
	view, _ := tableColumns(tpl, table.ID)
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"configureTableBinding","version":1,"id":"`+table.ID+`","collection":"transactions[]","alias":"transaction"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"updateTableColumnBinding","version":1,"id":"`+table.ID+`","columnId":"`+view.Columns[0].ID+`","field":"params.value"}`)); err != nil {
		t.Fatal(err)
	}
	// A formatNumber row expression is a legal persisted source and must migrate
	// without a browser parser; params remains a distinct root and stays put.
	_, _, _, tableElement, err := findComponent(tpl, table.ID)
	if err != nil {
		t.Fatal(err)
	}
	tableElement.Table.Value.Columns[1].Bind = `{{formatNumber(transaction.amount, "#,##0.00")}}`
	tableElement.Table.Value.Columns[2].Bind = `{{params.value}}`
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"configureTableBinding","version":1,"id":"`+table.ID+`","collection":"transactions[]","alias":"item"}`)); err != nil {
		t.Fatal(err)
	}
	view, err = tableColumns(tpl, table.ID)
	if err != nil || view.Alias != "item" || view.Columns[0].Binding != "{{item.params.value}}" || view.Columns[1].Binding != `{{formatNumber(item.amount, "#,##0.00")}}` || view.Columns[2].Binding != "{{params.value}}" {
		t.Fatalf("migrated projection = %#v, err=%v", view, err)
	}
	canonical, _ := SerializeTemplate(tpl)
	for _, alias := range []string{"params", "page", "pages"} {
		if _, err := applyComponentCommand(tpl, []byte(`{"kind":"configureTableBinding","version":1,"id":"`+table.ID+`","collection":"transactions[]","alias":"`+alias+`"}`)); err == nil {
			t.Fatalf("reserved alias %q succeeded", alias)
		}
		after, _ := SerializeTemplate(tpl)
		if !bytes.Equal(canonical, after) {
			t.Fatalf("reserved alias %q mutated bytes", alias)
		}
	}
	if _, err := applyComponentCommand(tpl, append([]byte(`{"kind":"configureTableBinding","version":1,"id":"`+table.ID+`","collection":"transactions[]","alias":"sale"}`), []byte(` {}`)...)); err == nil {
		t.Fatal("concatenated command succeeded")
	}
	if _, err := ParseTemplate(canonical); err != nil {
		t.Fatalf("migrated reload: %v", err)
	}
}

func TestDropComponentUsesGoHalfOpenBandHitTesting(t *testing.T) {
	tpl := componentTemplate(t)
	initial, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		band string
		y    int64
	}{
		{"pageHeader", initial.Bands[0].Y},
		{"content", initial.Bands[1].Y},
		{"pageFooter", initial.Bands[2].Y},
	} {
		before, _ := canvas(tpl)
		command := []byte(`{"kind":"dropComponent","version":1,"type":"text","x":36,"y":` + pointLiteral(want.y) + `,"snap":false}`)
		after, err := applyComponentCommand(tpl, command)
		if err != nil {
			t.Fatalf("drop %s: %v", want.band, err)
		}
		if got := newProjectedComponent(t, before, after).Band; got != want.band {
			t.Fatalf("drop at %d resolved %s, want %s", want.y, got, want.band)
		}
	}
	before, _ := SerializeTemplate(tpl)
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"dropComponent","version":1,"type":"text","x":35.999,"y":36,"snap":false}`)); err == nil {
		t.Fatal("drop on the left page edge unexpectedly succeeded")
	}
	after, _ := SerializeTemplate(tpl)
	if !bytes.Equal(before, after) {
		t.Fatal("rejected drop changed canonical bytes")
	}
}

func imageDropTemplate(t *testing.T, width, height geom.Length) *Template {
	t.Helper()
	tpl := componentTemplate(t)
	tpl.doc.Page.SizeIsName = false
	tpl.doc.Page.SizeName = ""
	tpl.doc.Page.SizeCustom = template.PageSize{Width: width + 72000, Height: 842000}
	tpl.doc.Bands.Content.Elements = nil
	for _, band := range []*template.Band{&tpl.doc.Bands.PageHeader, &tpl.doc.Bands.PageFooter} {
		band.Elements = nil
		band.Height.Value = height
	}
	return tpl
}

func TestImageDropsFitAtTheirOriginInHeaderAndFooter(t *testing.T) {
	for _, bandName := range []string{"pageHeader", "pageFooter"} {
		for _, snap := range []bool{false, true} {
			for _, tc := range []struct {
				name               string
				x, y               int64
				unsnapped, snapped [4]int64
			}{
				{"interior", 8000, 7000, [4]int64{8000, 7000, 96000, 48000}, [4]int64{6000, 6000, 96000, 48000}},
				{"midpoint", 12000, 30500, [4]int64{12000, 30500, 96000, 30500}, [4]int64{12000, 30000, 96000, 31000}},
				{"bottom", 12000, 60999, [4]int64{12000, 60999, 96000, 1}, [4]int64{12000, 60000, 96000, 1000}},
				{"right", 196999, 7000, [4]int64{196999, 7000, 1, 48000}, [4]int64{192000, 6000, 5000, 48000}},
				{"bottom right", 196999, 60999, [4]int64{196999, 60999, 1, 1}, [4]int64{192000, 60000, 5000, 1000}},
				{"snap overflows only width", 100000, 7000, [4]int64{100000, 7000, 96000, 48000}, [4]int64{102000, 6000, 95000, 48000}},
			} {
				t.Run(fmt.Sprintf("%s/snap=%t/%s", bandName, snap, tc.name), func(t *testing.T) {
					tpl := imageDropTemplate(t, 197000, 61000)
					before, _ := canvas(tpl)
					band := projectedBands(t, tpl)[bandName]
					command := fmt.Sprintf(`{"kind":"dropComponent","version":1,"type":"image","x":%s,"y":%s,"snap":%t}`, pointLiteral(band.X+tc.x), pointLiteral(band.Y+tc.y), snap)
					after, err := applyComponentCommand(tpl, []byte(command))
					if err != nil {
						t.Fatal(err)
					}
					want := tc.unsnapped
					if snap {
						want = tc.snapped
					}
					image := newProjectedComponent(t, before, after)
					if image.Band != bandName || [4]int64{image.X, image.Y, image.Width, image.Height} != want {
						t.Fatalf("image = %#v, want %s %v", image, bandName, want)
					}
					if len(after.Components) != len(before.Components)+1 || !reflect.DeepEqual(before.Bands, after.Bands) {
						t.Fatal("image placement changed a band or added more than one component")
					}
				})
			}
			t.Run(fmt.Sprintf("%s/snap=%t/grid boundary", bandName, snap), func(t *testing.T) {
				tpl := imageDropTemplate(t, 96000, 48000)
				before, _ := canvas(tpl)
				band := projectedBands(t, tpl)[bandName]
				after, err := applyComponentCommand(tpl, []byte(fmt.Sprintf(`{"kind":"dropComponent","version":1,"type":"image","x":%s,"y":%s,"snap":%t}`, pointLiteral(band.X+95999), pointLiteral(band.Y+47999), snap)))
				if err != nil {
					t.Fatal(err)
				}
				want := [4]int64{95999, 47999, 1, 1}
				if snap {
					want = [4]int64{90000, 42000, 6000, 6000}
				}
				image := newProjectedComponent(t, before, after)
				if [4]int64{image.X, image.Y, image.Width, image.Height} != want {
					t.Fatalf("image at the grid boundary = %#v, want %v", image, want)
				}
			})
		}
	}
}

func TestImageDropsResizeToUndersizedBands(t *testing.T) {
	for _, bandName := range []string{"pageHeader", "pageFooter"} {
		for _, snap := range []bool{false, true} {
			for _, size := range []struct{ width, height geom.Length }{{95000, 61000}, {197000, 47000}, {1000, 1}} {
				t.Run(fmt.Sprintf("%s/snap=%t/%dx%d", bandName, snap, size.width, size.height), func(t *testing.T) {
					tpl := imageDropTemplate(t, size.width, size.height)
					band := projectedBands(t, tpl)[bandName]
					before, _ := canvas(tpl)
					after, err := applyComponentCommand(tpl, []byte(fmt.Sprintf(`{"kind":"dropComponent","version":1,"type":"image","x":%s,"y":%s,"snap":%t}`, pointLiteral(band.X), pointLiteral(band.Y), snap)))
					if err != nil {
						t.Fatal(err)
					}
					image := newProjectedComponent(t, before, after)
					if image.X != 0 || image.Y != 0 || image.Width != min(96000, int64(size.width)) || image.Height != min(48000, int64(size.height)) {
						t.Fatalf("resized image = %#v", image)
					}
					if !reflect.DeepEqual(before.Bands, after.Bands) {
						t.Fatal("image placement grew a band")
					}
				})
			}
		}
	}
}

func TestImageDropKeepsContentAndHalfOpenHitTesting(t *testing.T) {
	for _, snap := range []bool{false, true} {
		t.Run(fmt.Sprintf("snap=%t", snap), func(t *testing.T) {
			tpl := imageDropTemplate(t, 197000, 61000)
			bands := projectedBands(t, tpl)
			content, header, footer := bands["content"], bands["pageHeader"], bands["pageFooter"]
			for _, tc := range []struct {
				y    int64
				band string
			}{
				{header.Y + header.Height, "content"},
				{content.Y + content.Height - 1, "content"},
				{content.Y + content.Height, "pageFooter"},
			} {
				before, _ := canvas(tpl)
				after, err := applyComponentCommand(tpl, []byte(fmt.Sprintf(`{"kind":"dropComponent","version":1,"type":"image","x":%s,"y":%s,"snap":%t}`, pointLiteral(content.X), pointLiteral(tc.y), snap)))
				if err != nil {
					t.Fatal(err)
				}
				image := newProjectedComponent(t, before, after)
				if image.Band != tc.band || image.Width != 96000 || image.Height != 48000 {
					t.Fatalf("boundary image = %#v, want %s", image, tc.band)
				}
				if tc.y == content.Y+content.Height-1 && (image.Y < content.Height-3000 || image.Y+image.Height <= content.Height) {
					t.Fatalf("content image was pulled up into the first window: %#v", image)
				}
			}
			for _, point := range [][2]int64{
				{content.X + content.Width - 1, content.Y}, // Content retains strict horizontal containment.
				{header.X - 1, header.Y}, {header.X, header.Y - 1},
				{header.X + header.Width, header.Y}, {footer.X, footer.Y + footer.Height},
			} {
				before, _ := SerializeTemplate(tpl)
				_, err := applyComponentCommand(tpl, []byte(fmt.Sprintf(`{"kind":"dropComponent","version":1,"type":"image","x":%s,"y":%s,"snap":%t}`, pointLiteral(point[0]), pointLiteral(point[1]), snap)))
				if err == nil {
					t.Fatalf("drop at (%d,%d) unexpectedly succeeded", point[0], point[1])
				}
				if after, _ := SerializeTemplate(tpl); !bytes.Equal(before, after) {
					t.Fatal("rejected image drop changed canonical bytes")
				}
			}
		})
	}
}

func TestImagePaletteFitKeepsExplicitGeometryStrict(t *testing.T) {
	for _, bandName := range []string{"pageHeader", "pageFooter"} {
		for _, snap := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/snap=%t", bandName, snap), func(t *testing.T) {
				tpl := imageDropTemplate(t, 197000, 61000)
				before, _ := canvas(tpl)
				created, err := applyComponentCommand(tpl, []byte(fmt.Sprintf(`{"kind":"createComponent","version":1,"type":"image","band":%q,"x":0,"y":0,"width":96,"height":48,"snap":false}`, bandName)))
				if err != nil {
					t.Fatal(err)
				}
				id := newProjectedComponent(t, before, created).ID
				band := projectedBands(t, tpl)[bandName]
				for _, command := range []string{
					fmt.Sprintf(`{"kind":"createComponent","version":1,"type":"image","band":%q,"x":0,"y":30,"width":96,"height":48,"snap":%t}`, bandName, snap),
					fmt.Sprintf(`{"kind":"moveComponent","version":1,"id":%q,"x":0,"y":30,"snap":%t}`, id, snap),
					fmt.Sprintf(`{"kind":"resizeComponent","version":1,"id":%q,"width":96,"height":72,"snap":%t}`, id, snap),
					fmt.Sprintf(`{"kind":"setComponentBounds","version":1,"id":%q,"x":0,"y":30,"width":96,"height":48,"snap":%t}`, id, snap),
					fmt.Sprintf(`{"kind":"dropComponent","version":1,"type":"rect","x":%s,"y":%s,"snap":%t}`, pointLiteral(band.X), pointLiteral(band.Y+band.Height-1), snap),
				} {
					canonical, _ := SerializeTemplate(tpl)
					if _, err := applyComponentCommand(tpl, []byte(command)); err == nil {
						t.Fatalf("out-of-bounds command unexpectedly succeeded: %s", command)
					}
					if after, _ := SerializeTemplate(tpl); !bytes.Equal(canonical, after) {
						t.Fatalf("rejected command changed canonical bytes: %s", command)
					}
				}
			})
		}
	}
}

func TestSetComponentBoundsMovesOriginAndSizeInOneCommand(t *testing.T) {
	tpl := componentTemplate(t)
	before, _ := canvas(tpl)
	createdProjection, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"rect","band":"content","x":36,"y":36,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	created := newProjectedComponent(t, before, createdProjection)
	// A north-west drag: the origin and the size move together, which is the
	// whole reason this command exists next to move and resize.
	bounded, err := applyComponentCommand(tpl, []byte(`{"kind":"setComponentBounds","version":1,"id":"`+created.ID+`","x":24.005,"y":12.006,"width":84.007,"height":48.008,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	component := newProjectedComponent(t, before, bounded)
	if component.X != 24005 || component.Y != 12006 || component.Width != 84007 || component.Height != 48008 {
		t.Fatalf("bounds units = (%d,%d,%d,%d), want (24005,12006,84007,48008)", component.X, component.Y, component.Width, component.Height)
	}
	// A TALL CONTENT Y, ACCEPTED. This seam moves origin and size together
	// and its refusal map below carries no y or height overflow probe at
	// all, so the clause Story 7.5 lifted had zero coverage here on either
	// side of the change. 2400pt is roughly three and a half windows down a
	// content band 679.89pt tall.
	tall, err := applyComponentCommand(tpl, []byte(`{"kind":"setComponentBounds","version":1,"id":"`+created.ID+`","x":0,"y":2400,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatalf("bounds three windows below the band foot were refused: %v", err)
	}
	if component = newProjectedComponent(t, before, tall); component.Y != 2400000 {
		t.Fatalf("tall bounds y = %d, want 2400000", component.Y)
	}
	if roundTripped := reloadedComponent(t, tpl, created.ID); roundTripped.Y != 2400000 {
		t.Fatalf("canonical bytes carried y = %d, want 2400000", roundTripped.Y)
	}
	canonical, _ := SerializeTemplate(tpl)
	for name, command := range map[string]string{
		"missing height":  `{"kind":"setComponentBounds","version":1,"id":"` + created.ID + `","x":0,"y":0,"width":72,"snap":false}`,
		"zero width":      `{"kind":"setComponentBounds","version":1,"id":"` + created.ID + `","x":0,"y":0,"width":0,"height":24,"snap":false}`,
		"outside theband": `{"kind":"setComponentBounds","version":1,"id":"` + created.ID + `","x":-1,"y":0,"width":72,"height":24,"snap":false}`,
		"past band width": `{"kind":"setComponentBounds","version":1,"id":"` + created.ID + `","x":0,"y":0,"width":100000,"height":24,"snap":false}`,
		"unknown id":      `{"kind":"setComponentBounds","version":1,"id":"nope","x":0,"y":0,"width":72,"height":24,"snap":false}`,
	} {
		if _, err := applyComponentCommand(tpl, []byte(command)); err == nil {
			t.Fatalf("%s unexpectedly succeeded", name)
		}
		if after, _ := SerializeTemplate(tpl); !bytes.Equal(canonical, after) {
			t.Fatalf("rejected %s changed canonical bytes", name)
		}
	}
}

func TestSnapDoesNotPushAnEdgeDragOutOfItsBand(t *testing.T) {
	tpl := componentTemplate(t)
	before, _ := canvas(tpl)
	createdProjection, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"rect","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	created := newProjectedComponent(t, before, createdProjection)
	var band designer.CanvasBand
	for _, candidate := range createdProjection.Bands {
		if candidate.Name == "content" {
			band = candidate
		}
	}
	literal := func(millipoints int64) string {
		return fmt.Sprintf("%d.%03d", millipoints/1000, millipoints%1000)
	}
	// The far edge of the band, to the millipoint. Nearest-grid snapping can
	// round this away from the band, and rounding alone must not turn a legal
	// drag into a refusal the designer shows as a bounce back.
	edgeX, edgeY := band.Width-created.Width, band.Height-created.Height
	moved, err := applyComponentCommand(tpl, []byte(`{"kind":"moveComponent","version":1,"id":"`+created.ID+`","x":`+literal(edgeX)+`,"y":`+literal(edgeY)+`,"snap":true}`))
	if err != nil {
		t.Fatalf("edge move with snapping was refused: %v", err)
	}
	component := newProjectedComponent(t, before, moved)
	if component.X%designer.GridIncrement != 0 || component.Y%designer.GridIncrement != 0 {
		t.Fatalf("pulled-back origin = (%d,%d), want grid multiples", component.X, component.Y)
	}
	// THE X HALF ONLY, since Story 7.5. The pull-back is a rescue of the
	// grid's own rounding on the axis that still has an edge to be pulled
	// back to; the content band has no bottom edge any more, so a snapped Y
	// past `edgeY` is the position the author asked for and stays there.
	if component.X+component.Width > band.Width {
		t.Fatalf("pulled-back geometry (%d,%d,%d,%d) leaves band width %d", component.X, component.Y, component.Width, component.Height, band.Width)
	}
	if edgeX-component.X >= designer.GridIncrement {
		t.Fatalf("pull-back moved x %d further than one grid step from %d", component.X, edgeX)
	}
	// Same for a bounds drag that lands its far edges exactly on the band.
	bounded, err := applyComponentCommand(tpl, []byte(`{"kind":"setComponentBounds","version":1,"id":"`+created.ID+`","x":`+literal(edgeX)+`,"y":`+literal(edgeY)+`,"width":`+literal(created.Width)+`,"height":`+literal(created.Height)+`,"snap":true}`))
	if err != nil {
		t.Fatalf("edge bounds with snapping was refused: %v", err)
	}
	component = newProjectedComponent(t, before, bounded)
	if component.X+component.Width > band.Width {
		t.Fatalf("pulled-back bounds (%d,%d,%d,%d) leave band width %d", component.X, component.Y, component.Width, component.Height, band.Width)
	}
	// The pull-back rescues the grid's own rounding and nothing else: a
	// caller asking for geometry a whole grid step outside is still refused.
	canonical, _ := SerializeTemplate(tpl)
	for name, command := range map[string]string{
		"a grid step past the right edge": `{"kind":"moveComponent","version":1,"id":"` + created.ID + `","x":` + literal(edgeX+designer.GridIncrement) + `,"y":0,"snap":true}`,
		"far past the right edge":         `{"kind":"setComponentBounds","version":1,"id":"` + created.ID + `","x":` + literal(edgeX+100*designer.GridIncrement) + `,"y":0,"width":72,"height":24,"snap":true}`,
	} {
		if _, err := applyComponentCommand(tpl, []byte(command)); err == nil {
			t.Fatalf("%s unexpectedly succeeded", name)
		}
		if after, _ := SerializeTemplate(tpl); !bytes.Equal(canonical, after) {
			t.Fatalf("rejected %s changed canonical bytes", name)
		}
	}
	// A grid step past what used to be the bottom edge, which was in that
	// refusal map until Story 7.5 and is now an ordinary placement. The Y arm
	// of the pull-back assertions above went vacuous when the cap lifted —
	// nothing pulls a content Y back any more — so THIS is what carries the
	// Y axis's discriminating power now: far below the foot of page one, the
	// drag is accepted, still snaps to the grid, and is what the canonical
	// bytes carry back on the next load.
	far := band.Height*3 + 4321
	dropped, err := applyComponentCommand(tpl, []byte(`{"kind":"moveComponent","version":1,"id":"`+created.ID+`","x":0,"y":`+literal(far)+`,"snap":true}`))
	if err != nil {
		t.Fatalf("a content drag three windows below the band was refused: %v", err)
	}
	component = newProjectedComponent(t, before, dropped)
	if component.Y%designer.GridIncrement != 0 {
		t.Fatalf("snapped y = %d, want a grid multiple", component.Y)
	}
	if component.Y <= band.Height {
		t.Fatalf("snapped y = %d, want a position below the one-window band height %d", component.Y, band.Height)
	}
	roundTripped := reloadedComponent(t, tpl, created.ID)
	if roundTripped.Y != component.Y {
		t.Fatalf("canonical bytes carried y = %d, want the placed %d", roundTripped.Y, component.Y)
	}
}

// reloadedComponent serializes tpl, parses the bytes back and returns the
// named component as the fresh projection sees it — a full round trip through
// the canonical form, which is the only way to show that a placement PERSISTS
// rather than merely being accepted by one command.
func reloadedComponent(t *testing.T, tpl *Template, id string) designer.CanvasComponent {
	t.Helper()
	canonical, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := ParseTemplate(canonical)
	if err != nil {
		t.Fatalf("canonical bytes did not load back: %v", err)
	}
	projection, err := canvas(reloaded)
	if err != nil {
		t.Fatal(err)
	}
	for _, component := range projection.Components {
		if component.ID == id {
			return component
		}
	}
	t.Fatalf("component %q is absent from the reloaded projection", id)
	return designer.CanvasComponent{}
}

func TestComponentMoveResizeDeleteAreExactAndMonotonic(t *testing.T) {
	tpl := componentTemplate(t)
	before, _ := canvas(tpl)
	createdProjection, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"rect","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	created := newProjectedComponent(t, before, createdProjection)
	moved, err := applyComponentCommand(tpl, []byte(`{"kind":"moveComponent","version":1,"id":"`+created.ID+`","x":1.001,"y":2.002,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	component := newProjectedComponent(t, before, moved)
	if component.X != 1001 || component.Y != 2002 {
		t.Fatalf("move units = (%d,%d), want (1001,2002)", component.X, component.Y)
	}
	resized, err := applyComponentCommand(tpl, []byte(`{"kind":"resizeComponent","version":1,"id":"`+created.ID+`","width":73.003,"height":25.004,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	component = newProjectedComponent(t, before, resized)
	if component.Width != 73003 || component.Height != 25004 {
		t.Fatalf("resize units = (%d,%d), want (73003,25004)", component.Width, component.Height)
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"deleteComponent","version":1,"id":"`+created.ID+`"}`)); err != nil {
		t.Fatal(err)
	}
	beforeNext, _ := canvas(tpl)
	next, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"line","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	if newProjectedComponent(t, beforeNext, next).ID == created.ID {
		t.Fatal("component id was reused after deletion")
	}
}

// projectedBands is the three CanvasBands of a template, by name, so a test
// can probe containComponent with the same rectangles the command path hands
// it rather than with numbers it invented.
func projectedBands(t *testing.T, tpl *Template) map[string]designer.CanvasBand {
	t.Helper()
	projection, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	bands := make(map[string]designer.CanvasBand, len(projection.Bands))
	for _, band := range projection.Bands {
		bands[band.Name] = band
	}
	if len(bands) != 3 {
		t.Fatalf("projection carries %d bands, want 3", len(bands))
	}
	return bands
}

// TestBandContainmentRefusalsCarryTheirExactMessages CHARACTERIZES the one
// message every band-extent refusal in the designer command path produces,
// by FULL-STRING equality, before Story 7.5 splits the predicate that
// produces it.
//
// Until this test existed, `grep "must stay within" --include="*_test.go"`
// returned nothing: "the message is unchanged" was a claim no test could
// contradict. A story that splits first and asserts afterwards is asserting
// whatever it happened to produce, so this lands first and on purpose.
//
// WHY FULL-STRING EQUALITY AND NEVER strings.Contains. This same file's
// unrelated column refusal reads "footerOf must stay within the table
// collection" — a substring assertion on "must stay within" is satisfied by
// a message about table collections, which is the opposite of a
// characterization.
func TestBandContainmentRefusalsCarryTheirExactMessages(t *testing.T) {
	tpl := componentTemplate(t)
	bands := projectedBands(t, tpl)
	// Spelled out here rather than assembled from the production format
	// string: a characterization that reuses the code it characterizes
	// asserts nothing at all.
	want := map[string]string{
		"pageHeader": "folio8: component geometry must stay within pageHeader",
		"content":    "folio8: component geometry must stay within content",
		"pageFooter": "folio8: component geometry must stay within pageFooter",
	}
	exactly := func(name, probe string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s in %s was accepted; want the refusal %q", probe, name, want[name])
		}
		if err.Error() != want[name] {
			t.Fatalf("%s in %s = %q, want exactly %q", probe, name, err.Error(), want[name])
		}
	}
	type probe struct {
		name                string
		x, y, width, height geom.Length
	}
	// REPRESENTATIONAL refusals. A negative coordinate is not a statement
	// about how tall a band is, so it is refused in every band and stays
	// refused in every band.
	for _, name := range []string{"pageHeader", "content", "pageFooter"} {
		for _, p := range []probe{
			{"negative x", -1, 0, 6000, 6000},
			{"negative y", 0, -1, 6000, 6000},
			{"negative width", 0, 0, -1, 6000},
			{"negative height", 0, 0, 6000, -1},
		} {
			exactly(name, p.name, containComponent(bands[name], p.x, p.y, p.width, p.height))
		}
	}
	// The HORIZONTAL cap, in the content band: a column is unbounded
	// vertically, never horizontally.
	content := bands["content"]
	exactly("content", "x past the band width", containComponent(content, geom.Length(content.Width)+1, 0, 6000, 6000))
	exactly("content", "width past the band width", containComponent(content, 0, 0, geom.Length(content.Width)+1, 6000))
	// The BAND-CAPACITY refusals, in the two repeating bands. A page header
	// is exactly one page tall because that is what repeating means.
	for _, name := range []string{"pageHeader", "pageFooter"} {
		band := bands[name]
		exactly(name, "y past the band height", containComponent(band, 0, geom.Length(band.Height)+1, 6000, 6000))
		exactly(name, "height past the band height", containComponent(band, 0, 0, 6000, geom.Length(band.Height)+1))
	}
	// And the clause Story 7.5 LIFTED, asserted from the other side. This
	// test landed with these two lines reading `exactly(...)`, red-proved,
	// and one commit later they read this — which is the whole reason they
	// were written first. The content band is a COLUMN: a Y past one
	// window's worth of height is a position on a later page, not geometry
	// outside the document.
	if err := containComponent(content, 0, geom.Length(content.Height)*11, 6000, 6000); err != nil {
		t.Fatalf("a content y eleven windows down = %v, want acceptance", err)
	}
	if err := containComponent(content, 0, 0, 6000, geom.Length(content.Height)*11); err != nil {
		t.Fatalf("a content box eleven windows tall = %v, want acceptance", err)
	}
}

// TestJavaScriptSafeGeometryBoundRefusalsCarryTheirExactMessages is the
// SEPARATE, SURVIVING upper bound: it is a different path, with a different
// message, upstream of containComponent, and Story 7.5's split does not
// touch it. After the lift it is the only thing bounding a content
// component's Y, so it is characterized here for the same reason as the
// band messages — nothing asserted it before.
func TestJavaScriptSafeGeometryBoundRefusalsCarryTheirExactMessages(t *testing.T) {
	tpl := componentTemplate(t)
	before, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	createdProjection, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"rect","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	created := newProjectedComponent(t, before, createdProjection)
	// One millipoint past Number.MAX_SAFE_INTEGER, as the command's own
	// three-decimal point literal.
	const past = "9007199254740.992"
	_, err = applyComponentCommand(tpl, []byte(`{"kind":"moveComponent","version":1,"id":"`+created.ID+`","x":0,"y":`+past+`,"snap":false}`))
	if want := "folio8: component.y: y exceeds the JavaScript-safe geometry bound"; err == nil || err.Error() != want {
		t.Fatalf("move past the JavaScript-safe bound = %v, want exactly %q", err, want)
	}
	// The projection-time sibling of the same bound, which is a different
	// message and must stay one: the command path refuses first, so this is
	// the backstop that keeps an unrepresentable coordinate from reaching
	// the JSON/JS boundary at all.
	stranded := componentTemplate(t)
	if len(stranded.doc.Bands.Content.Elements) == 0 {
		t.Fatal("fixture has no content element to strand")
	}
	stranded.doc.Bands.Content.Elements[0].X = geom.Length(designer.MaxCanvasMillipoints) + 1
	_, err = canvas(stranded)
	if want := "folio8: component exceeds the JavaScript-safe geometry bound"; err == nil || err.Error() != want {
		t.Fatalf("projection of an unrepresentable coordinate = %v, want exactly %q", err, want)
	}
}

// TestTheColumnLiftIsExercisedAtTheCommandSurface closes the gap between the
// clause Story 7.5 moved and the surface the story is ABOUT.
//
// Two separate weaknesses live here, and neither shows up as a red test:
//
//  1. TestBandContainmentRefusalsCarryTheirExactMessages calls
//     containComponent DIRECTLY. That is the right shape for characterizing a
//     predicate, but the story's expectations are written at the command
//     surface ("setComponentBounds / moveComponent / createComponent"), and
//     the refusal an author actually sees is a *ComponentCommandError whose
//     Message and DataPath are set by the eleven call sites, not by the
//     predicate. Only the pageHeader property path asserted that wrapping.
//
//  2. containEdgeY has FOUR call sites and only one of them — moveComponent's
//     y — was reachable from any test. Reverting the other three (
//     dropComponent's y, and setComponentBounds' height and y) to the
//     unconditional containEdge leaves the ENTIRE Go suite green, which was
//     measured, not assumed. Those three are the sites whose pre-clamp gate
//     the lift WIDENS: a drag that was refused outright before now passes the
//     probe, so a containEdge left behind would quietly pull the component
//     back onto page one instead of refusing it — and in the bounds case
//     would also collapse its height to zero, because the positivity guard
//     has already run by then.
func TestTheColumnLiftIsExercisedAtTheCommandSurface(t *testing.T) {
	tpl := componentTemplate(t)
	bands := projectedBands(t, tpl)
	content, header, footer := bands["content"], bands["pageHeader"], bands["pageFooter"]
	before, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	refusedAs := func(name string, err error, wantMessage string) {
		t.Helper()
		var failure *designer.ComponentCommandError
		if !errors.As(err, &failure) {
			t.Fatalf("%s = %v, want a component command failure", name, err)
		}
		if failure.Message != wantMessage {
			t.Fatalf("%s message = %q, want exactly %q", name, failure.Message, wantMessage)
		}
		if failure.DataPath != "component.geometry" {
			t.Fatalf("%s data path = %q, want component.geometry", name, failure.DataPath)
		}
	}

	// MATRIX ROW 1 NAMES createComponent, and nothing created a tall content
	// component: the largest content y anywhere else in these tests is 40pt.
	tall := content.Height*4 + 4321
	createdProjection, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"rect","band":"content","x":0,"y":`+pointLiteral(tall)+`,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatalf("createComponent four windows below the content top was refused: %v", err)
	}
	created := newProjectedComponent(t, before, createdProjection)
	if created.Y != tall {
		t.Fatalf("created y = %d, want the requested %d", created.Y, tall)
	}
	if roundTripped := reloadedComponent(t, tpl, created.ID); roundTripped.Y != tall {
		t.Fatalf("canonical bytes carried y = %d, want %d", roundTripped.Y, tall)
	}

	// THE COMMAND-SURFACE MESSAGES, by full-string equality, for the two
	// bands the property-panel test does not reach. The column is unbounded
	// vertically and never horizontally, so content still refuses on x.
	_, err = applyComponentCommand(tpl, []byte(`{"kind":"moveComponent","version":1,"id":"`+created.ID+`","x":`+pointLiteral(content.Width+1000)+`,"y":0,"snap":false}`))
	refusedAs("a content x past the band width", err, "folio8: component geometry must stay within content")

	footerProjection, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"rect","band":"pageFooter","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	inFooter := newProjectedComponent(t, createdProjection, footerProjection)
	canonical, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	_, err = applyComponentCommand(tpl, []byte(`{"kind":"moveComponent","version":1,"id":"`+inFooter.ID+`","x":0,"y":`+pointLiteral(footer.Height+1000)+`,"snap":false}`))
	refusedAs("a pageFooter y past the band height", err, "folio8: component geometry must stay within pageFooter")
	if after, _ := SerializeTemplate(tpl); !bytes.Equal(canonical, after) {
		t.Fatal("a refused pageFooter move changed the canonical bytes")
	}

	// dropComponent's Y PULL-BACK, at a page point one point above the foot
	// of the content band. hitTestBand's rectangle is still one page tall, so
	// this is the lowest a drop can land — and the dropped box hangs past the
	// band foot, which is the whole point. With containEdgeY the component
	// keeps the grid position the author dropped it at; with the
	// unconditional containEdge it is pulled back inside page one instead.
	dropPageY := content.Y + content.Height - 1000
	beforeDrop, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	droppedProjection, err := applyComponentCommand(tpl, []byte(`{"kind":"dropComponent","version":1,"type":"rect","x":`+pointLiteral(content.X)+`,"y":`+pointLiteral(dropPageY)+`,"snap":true}`))
	if err != nil {
		t.Fatalf("a snapped drop at the foot of the content band was refused: %v", err)
	}
	dropped := newProjectedComponent(t, beforeDrop, droppedProjection)
	if dropped.Y%designer.GridIncrement != 0 {
		t.Fatalf("dropped y = %d, want a grid multiple", dropped.Y)
	}
	if dropped.Y+dropped.Height <= content.Height {
		t.Fatalf("dropped box (y %d + height %d) was pulled back inside the one-window band height %d", dropped.Y, dropped.Height, content.Height)
	}

	// setComponentBounds' HEIGHT and Y pull-backs, on a tall content
	// component with snapping on and no coordinate on the grid. The mutant
	// here is the expensive one: containEdge(height, band.Height-y) sees a
	// negative limit and floorToGrid returns 0, so the component is committed
	// with no height at all — the positivity guard at the top of the command
	// ran long before.
	boundsY := content.Height*3 + 500
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"setComponentBounds","version":1,"id":"`+created.ID+`","x":0.5,"y":`+pointLiteral(boundsY)+`,"width":72.5,"height":24.5,"snap":true}`)); err != nil {
		t.Fatalf("a snapped bounds drag three windows down was refused: %v", err)
	}
	bounded := reloadedComponent(t, tpl, created.ID)
	if bounded.Height <= 0 {
		t.Fatalf("snapped bounds committed height = %d; the pull-back collapsed the component", bounded.Height)
	}
	if bounded.Height%designer.GridIncrement != 0 || bounded.Y%designer.GridIncrement != 0 {
		t.Fatalf("snapped bounds (y %d, height %d) are not grid multiples", bounded.Y, bounded.Height)
	}
	if bounded.Y <= content.Height {
		t.Fatalf("snapped bounds y = %d was pulled back inside the one-window band height %d", bounded.Y, content.Height)
	}

	// AND THE OTHER DIRECTION, which nothing exercised either: in a band that
	// DOES cap, containEdgeY must still clamp. A pageHeader component whose
	// far edge is off the grid snaps PAST the band foot, and the pull-back is
	// what rescues it — a containEdgeY that returned its input unchanged in
	// every band would make this command a refusal instead.
	beforeHeader, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	headerProjection, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"rect","band":"pageHeader","x":0,"y":0,"width":72,"height":25,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	inHeader := newProjectedComponent(t, beforeHeader, headerProjection)
	headerEdgeY := header.Height - inHeader.Height
	if headerEdgeY%designer.GridIncrement == 0 {
		t.Fatalf("this test needs an off-grid edge to say anything; edgeY %d is already a grid multiple", headerEdgeY)
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"moveComponent","version":1,"id":"`+inHeader.ID+`","x":0,"y":`+pointLiteral(headerEdgeY)+`,"snap":true}`)); err != nil {
		t.Fatalf("an edge move inside the pageHeader was refused; the pull-back did not rescue the grid's rounding: %v", err)
	}
	movedInHeader := reloadedComponent(t, tpl, inHeader.ID)
	if movedInHeader.Y+movedInHeader.Height > header.Height {
		t.Fatalf("pageHeader component (y %d + height %d) left its band height %d; containEdgeY did not clamp", movedInHeader.Y, movedInHeader.Height, header.Height)
	}
	if headerEdgeY-movedInHeader.Y >= designer.GridIncrement {
		t.Fatalf("pageHeader pull-back moved y to %d, more than one grid step from the edge %d", movedInHeader.Y, headerEdgeY)
	}
}

// TestDuplicateDoesNotCopyTheKeepTogetherTag is D-7.7.10 (DW-48), asserted
// rather than left incidental.
//
// duplicateComponent's `clone := *element` is a WHOLE-STRUCT copy, so every
// field of template.Element rides along by default and only the ones written
// afterwards differ. Before Story 7.9 that included the keep-together tag —
// and Epic 7 ships no designer control anywhere to view, set or clear one
// (file-only authoring is the stated scope boundary), so a duplicated
// signature block would silently enlarge a group the author cannot reach,
// moving a page break for a reason nothing in the product explains.
//
// Nothing in this repository asserted ANY field-copy behaviour of duplicate
// before this test: wasm/engine_test.go's TestEngineDuplicateIsACommittedGoCommand
// claims only that the id changed. That absence is why the tag rode along
// unnoticed.
func TestDuplicateDoesNotCopyTheKeepTogetherTag(t *testing.T) {
	tpl, err := ParseTemplate([]byte(canvasWindowCountGroupedTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	// The tag is refused outside the content band and on a table, so the
	// subject must be a content-band element of another kind.
	const tagged = "e2"
	original, ok := contentElementByID(tpl, tagged)
	if !ok || !original.KeepTogether.Set || original.KeepTogether.Null || original.KeepTogether.Value != "signature" {
		t.Fatalf("precondition: %s must carry a keepTogether tag for this test to say anything: %#v", tagged, original.KeepTogether)
	}
	before, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := applyComponentCommand(tpl, []byte(`{"kind":"duplicateComponent","version":1,"id":"`+tagged+`","snap":false}`))
	if err != nil {
		t.Fatalf("duplicateComponent: %v", err)
	}
	copyID := newProjectedComponent(t, before, projection).ID
	if copyID == tagged {
		t.Fatal("the duplicate must be a NEW element, not the original")
	}

	clone, ok := contentElementByID(tpl, copyID)
	if !ok {
		t.Fatalf("the duplicate %s is not in the content band", copyID)
	}
	// ABSENT, not an explicit null. `Set: true, Null: true` serializes back
	// as `"keepTogether": null`, which is still the key appearing in the file
	// and still raises the document's required format version — so a clear
	// that produced one would not be "no tag" at all.
	if clone.KeepTogether.Set || clone.KeepTogether.Null || clone.KeepTogether.Value != "" {
		t.Fatalf("the duplicate carries keepTogether %#v; a copy must join NO group, and its absence must be the zero Presence rather than an explicit null", clone.KeepTogether)
	}
	// The rest of the copy is untouched, so the assertion above is about the
	// TAG and not about a duplicate that silently lost its other fields.
	if clone.Type != original.Type || clone.Value != original.Value || clone.Width != original.Width || clone.Height != original.Height {
		t.Fatalf("the duplicate diverged from its original beyond the tag: %#v against %#v", clone, original)
	}

	// AND THE ORIGINAL IS UNCHANGED. A clear applied to the wrong struct — or
	// through a pointer into the band's slice — would drop the author's own
	// declaration while the copy looked correct.
	after, ok := contentElementByID(tpl, tagged)
	if !ok {
		t.Fatalf("the original %s disappeared from the content band", tagged)
	}
	if after.KeepTogether != original.KeepTogether {
		t.Fatalf("the original's keepTogether changed from %#v to %#v", original.KeepTogether, after.KeepTogether)
	}
	// At the bytes, which is where the author's file actually lives: exactly
	// the two tags the document was authored with, and no `null` spelling.
	canonical, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(canonical, []byte(`"keepTogether": "signature"`)); n != 2 {
		t.Fatalf("the serialized document carries %d keepTogether tags, want the two it was authored with", n)
	}
	if bytes.Contains(canonical, []byte(`"keepTogether": null`)) {
		t.Fatal("the duplicate wrote an explicit null keepTogether; the field must be ABSENT")
	}
}

// contentElementByID finds a content-band element by id, reading the
// TEMPLATE's own element rather than the CanvasProjection — the projection
// carries no keepTogether field at all, so a test that probed it could not see
// the tag whether it was copied or not.
func contentElementByID(tpl *Template, id string) (template.Element, bool) {
	for _, el := range tpl.doc.Bands.Content.Elements {
		if string(el.ID) == id {
			return el, true
		}
	}
	return template.Element{}, false
}

// ---------------------------------------------------------------------------
// STORY 8.1: THE FONT-CHAIN COMMAND VOCABULARY.
//
// fontChainDocJSON is a THREE-chain document with fontFamily attached at both
// of its two attachment points, across all three bands, so the delete
// refusal's document order (pageHeader, content, pageFooter) and its
// headerStyle arm are both reachable:
//
//	body    — named by e2 (pageHeader style), e7 (content style),
//	          e9 (content table headerStyle) and e11 (pageFooter style)
//	heading — named by e12 (pageFooter style) only
//	unused  — named by nothing
//
// (Ids are base-36 counters behind the "e", so e11 is 37 and e12 is 38: e11 is
// the free id below the document's nextId of 39, and e13 — which is 39 — would
// have collided with it.)
//
// e11 EXISTS FOR THE pageFooter ARM, and it is not decoration: measured by
// deleting Bands.PageFooter.Elements from fontChainBands, the whole folio-go
// suite stayed green without it — the footer band was walked by nothing any
// test could see, because the only footer element named a chain nothing ever
// deleted or renamed. The AC asks the orphaning-delete refusal to name "an
// element in each of the three bands and a table's headerStyle", and this is
// the element that makes the third band one of them.
const fontChainDocJSON = `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e7", "type": "text", "x": 0, "y": 0, "width": 100, "height": 20, "value": "content", "style": {"fontFamily": "body"}},
      {"id": "e9", "type": "table", "x": 0, "y": 30, "bind": "items[]", "headerHeight": 20,
        "columns": [{"id": "e10", "label": "Date", "width": 100, "bind": "{{row.a}}"}],
        "headerStyle": {"fontFamily": "body"}}
    ]},
    "pageFooter": {"elements": [
      {"id": "e12", "type": "text", "x": 0, "y": 0, "width": 100, "height": 20, "value": "footer", "style": {"fontFamily": "heading"}},
      {"id": "e11", "type": "text", "x": 120, "y": 0, "width": 100, "height": 20, "value": "page", "style": {"fontFamily": "body"}}
    ], "height": 20},
    "pageHeader": {"elements": [{"id": "e2", "type": "text", "x": 0, "y": 0, "width": 100, "height": 20, "value": "header", "style": {"fontFamily": "body"}}], "height": 20}
  },
  "fonts": {"body": ["Noto Sans", "Noto Sans Thai"], "heading": ["Noto Sans"], "unused": ["Noto Sans SC"]},
  "locale": "en",
  "nextId": 39,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}`

func fontChainTemplate(t *testing.T) *Template {
	t.Helper()
	tpl, err := ParseTemplate([]byte(fontChainDocJSON))
	if err != nil {
		t.Fatal(err)
	}
	return tpl
}

// fontChainRefusal applies command, requires it to be refused as a located
// *ComponentCommandError, and requires the document to be byte-identical
// afterwards — every matrix row's "document unmutated" clause, asserted rather
// than assumed, because a partially applied rename is exactly the failure the
// one-transaction shape exists to prevent.
func fontChainRefusal(t *testing.T, tpl *Template, command string) *designer.ComponentCommandError {
	t.Helper()
	before, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	_, applyErr := applyComponentCommand(tpl, []byte(command))
	if applyErr == nil {
		t.Fatalf("command unexpectedly succeeded: %s", command)
	}
	after, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("a refused font chain command mutated the document: %s", command)
	}
	var failure *designer.ComponentCommandError
	if !errors.As(applyErr, &failure) {
		t.Fatalf("refusal for %s is %T (%v), want *ComponentCommandError", command, applyErr, applyErr)
	}
	if failure.ElementID != "" {
		t.Errorf("font chain refusal carries elementId %q; a chain command is not addressed to an element", failure.ElementID)
	}
	return failure
}

func fontChainAccepted(t *testing.T, tpl *Template, command string) designer.CanvasProjection {
	t.Helper()
	projection, err := applyComponentCommand(tpl, []byte(command))
	if err != nil {
		t.Fatalf("%s: %v", command, err)
	}
	return projection
}

func fontChainOf(t *testing.T, tpl *Template, name string) []string {
	t.Helper()
	chain, ok := tpl.doc.Fonts[name]
	if !ok {
		t.Fatalf("font chain %q is not declared", name)
	}
	// The command path can express only face-name entries (Story 8.6
	// owns the embedding command), so this helper keeps returning the
	// []string every assertion in this file is written against. An
	// EMBEDDED entry reaching here would be a defect, and it fails
	// loudly rather than flattening to "" — a silent empty face name is
	// exactly what a join-and-compare assertion cannot see.
	names := make([]string, 0, len(chain))
	for i, entry := range chain {
		if entry.Embedded() {
			t.Fatalf("font chain %q entry %d is an embedded-face entry; no command in this package can create one", name, i)
		}
		names = append(names, entry.Face)
	}
	return names
}

func fontFamilyOf(t *testing.T, tpl *Template, id string) string {
	t.Helper()
	_, _, _, element, err := findComponent(tpl, id)
	if err != nil {
		t.Fatalf("element %s: %v", id, err)
	}
	if element.Type == template.ElementTable && element.Table.Set && !element.Table.Null && element.Table.Value.HeaderStyle.Set {
		return element.Table.Value.HeaderStyle.Value.FontFamily.Value
	}
	if !element.Style.Set || element.Style.Null {
		return ""
	}
	return element.Style.Value.FontFamily.Value
}

func TestFontChainAddDeclaresAChainAndProjectsIt(t *testing.T) {
	tpl := fontChainTemplate(t)
	projection := fontChainAccepted(t, tpl, `{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans","Noto Sans Thai"]}`)
	if got := fontChainOf(t, tpl, "caption"); !reflect.DeepEqual(got, []string{"Noto Sans", "Noto Sans Thai"}) {
		t.Fatalf("caption chain = %#v", got)
	}
	if !reflect.DeepEqual(projection.FontFamilies, []string{"body", "caption", "heading", "unused"}) {
		t.Fatalf("projection families = %#v, want the declared chains, sorted", projection.FontFamilies)
	}
	// FontChains[i].Name == FontFamilies[i] by construction: assert it, since
	// the browser's guard rejects the whole snapshot if it ever stops holding.
	names := make([]string, 0, len(projection.FontChains))
	for _, chain := range projection.FontChains {
		names = append(names, chain.Name)
	}
	if !reflect.DeepEqual(names, projection.FontFamilies) {
		t.Fatalf("FontChains names = %#v, FontFamilies = %#v", names, projection.FontFamilies)
	}
	// Story 8.3: an entry is a projected OBJECT, not a string. A named
	// face carries its name in Face and nothing else — an empty AssetKey
	// is the discriminant the designer reads, and asserting the whole
	// struct (rather than only Face) is what keeps a family or style
	// leaking onto a non-embedded entry visible here.
	if !reflect.DeepEqual(projection.FontChains[1].Entries, []designer.CanvasFontChainEntry{{Face: "Noto Sans"}, {Face: "Noto Sans Thai"}}) {
		t.Fatalf("projected caption entries = %#v", projection.FontChains[1].Entries)
	}
}

func TestFontChainAddRefusesATakenName(t *testing.T) {
	failure := fontChainRefusal(t, fontChainTemplate(t), `{"kind":"addFontChain","version":1,"name":"heading","entries":["Noto Sans"]}`)
	if failure.Message != `a font chain named "heading" already exists` || failure.DataPath != "fonts.heading" {
		t.Fatalf("duplicate refusal = %q at %q", failure.Message, failure.DataPath)
	}
}

func TestFontChainAddRefusesAChainWithNoEntries(t *testing.T) {
	failure := fontChainRefusal(t, fontChainTemplate(t), `{"kind":"addFontChain","version":1,"name":"caption","entries":[]}`)
	if failure.Message != "a font chain must declare at least one entry" {
		t.Fatalf("empty-entries refusal = %q", failure.Message)
	}
}

func TestFontChainCommandsRefuseAnEmptyName(t *testing.T) {
	// commandString already refuses an absent or empty author string; the
	// chain commands reuse it rather than restating the rule.
	for _, probe := range []struct{ command, want string }{
		{`{"kind":"addFontChain","version":1,"name":"","entries":["Noto Sans"]}`, "folio8: name must be a non-empty string"},
		{`{"kind":"renameFontChain","version":1,"name":"body","to":""}`, "folio8: to must be a non-empty string"},
		{`{"kind":"deleteFontChain","version":1,"name":""}`, "folio8: name must be a non-empty string"},
		{`{"kind":"addFontChainEntry","version":1,"name":"body","index":0,"face":""}`, "folio8: face must be a non-empty string"},
	} {
		if failure := fontChainRefusal(t, fontChainTemplate(t), probe.command); failure.Message != probe.want {
			t.Errorf("%s refusal = %q, want %q", probe.command, failure.Message, probe.want)
		}
	}
}

func TestFontChainRenameCarriesEveryElementNamingIt(t *testing.T) {
	tpl := fontChainTemplate(t)
	fontChainAccepted(t, tpl, `{"kind":"renameFontChain","version":1,"name":"body","to":"brand"}`)
	if _, declared := tpl.doc.Fonts["body"]; declared {
		t.Fatal("the old chain key survived the rename")
	}
	if got := fontChainOf(t, tpl, "brand"); !reflect.DeepEqual(got, []string{"Noto Sans", "Noto Sans Thai"}) {
		t.Fatalf("renamed chain = %#v, want the entries carried verbatim", got)
	}
	// Every reference: a style.fontFamily bearer in each of the three bands,
	// and the table's headerStyle.fontFamily.
	for _, id := range []string{"e2", "e7", "e9", "e11"} {
		if got := fontFamilyOf(t, tpl, id); got != "brand" {
			t.Errorf("element %s names %q after the rename", id, got)
		}
	}
	if got := fontFamilyOf(t, tpl, "e12"); got != "heading" {
		t.Errorf("element e12 named a chain the rename did not touch: %q", got)
	}
}

func TestFontChainRenameRefusesATakenDestination(t *testing.T) {
	failure := fontChainRefusal(t, fontChainTemplate(t), `{"kind":"renameFontChain","version":1,"name":"body","to":"heading"}`)
	if failure.Message != `a font chain named "heading" already exists` {
		t.Fatalf("rename-onto-taken refusal = %q", failure.Message)
	}
}

func TestFontChainDeleteRemovesAnUnreferencedChain(t *testing.T) {
	tpl := fontChainTemplate(t)
	projection := fontChainAccepted(t, tpl, `{"kind":"deleteFontChain","version":1,"name":"unused"}`)
	if _, declared := tpl.doc.Fonts["unused"]; declared {
		t.Fatal("the deleted chain is still declared")
	}
	if !reflect.DeepEqual(projection.FontFamilies, []string{"body", "heading"}) {
		t.Fatalf("projection families after delete = %#v", projection.FontFamilies)
	}
}

// TestFontChainDeleteRefusesAChainAnElementStyleNames is the style.fontFamily
// arm of the orphaning-delete refusal, and asserts the document-order id list.
func TestFontChainDeleteRefusesAChainAnElementStyleNames(t *testing.T) {
	failure := fontChainRefusal(t, fontChainTemplate(t), `{"kind":"deleteFontChain","version":1,"name":"body"}`)
	if failure.Message != `font chain "body" is still named by e2, e7, e9, e11` || failure.DataPath != "fonts.body" {
		t.Fatalf("orphaning-delete refusal = %q at %q", failure.Message, failure.DataPath)
	}
}

// TestFontChainDeleteRefusesAChainOnlyAHeaderStyleNames proves the SECOND
// attachment point on its own. The style-arm test above would stay green if
// fontChainReferences never looked at a table's headerStyle at all — that
// population is exactly the one a style-only test never reaches.
func TestFontChainDeleteRefusesAChainOnlyAHeaderStyleNames(t *testing.T) {
	tpl := fontChainTemplate(t)
	// Move every style.fontFamily off "body" first — in all three bands,
	// including the footer's — leaving the table's headerStyle as the chain's
	// only remaining reference.
	fontChainAccepted(t, tpl, `{"kind":"updateComponentProperties","version":1,"ids":["e2","e7","e11"],"changes":{"fontFamily":{"op":"set","value":"heading"}}}`)
	if refs := fontChainReferences(tpl, "body"); !reflect.DeepEqual(refs, []string{"e9"}) {
		t.Fatalf("references to body = %#v, want the table's headerStyle alone", refs)
	}
	failure := fontChainRefusal(t, tpl, `{"kind":"deleteFontChain","version":1,"name":"body"}`)
	if failure.Message != `font chain "body" is still named by e9` {
		t.Fatalf("headerStyle-only refusal = %q", failure.Message)
	}
}

func TestFontChainRemoveEntryRefusesToEmptyAChain(t *testing.T) {
	failure := fontChainRefusal(t, fontChainTemplate(t), `{"kind":"removeFontChainEntry","version":1,"name":"heading","index":0}`)
	if failure.Message != `removing that entry would leave font chain "heading" with no entries` {
		t.Fatalf("last-entry refusal = %q", failure.Message)
	}
}

func TestFontChainEntryCommandsEditTheChainInPlace(t *testing.T) {
	tpl := fontChainTemplate(t)
	// A face this build's FontSet does not ship is ACCEPTED: the format's
	// standing tolerance (render.go's resolveRuneFace skips an absent member).
	fontChainAccepted(t, tpl, `{"kind":"addFontChainEntry","version":1,"name":"body","index":1,"face":"Nonesuch Display"}`)
	if got := fontChainOf(t, tpl, "body"); !reflect.DeepEqual(got, []string{"Noto Sans", "Nonesuch Display", "Noto Sans Thai"}) {
		t.Fatalf("after insert = %#v", got)
	}
	fontChainAccepted(t, tpl, `{"kind":"moveFontChainEntry","version":1,"name":"body","from":0,"to":2}`)
	if got := fontChainOf(t, tpl, "body"); !reflect.DeepEqual(got, []string{"Nonesuch Display", "Noto Sans Thai", "Noto Sans"}) {
		t.Fatalf("after move = %#v", got)
	}
	fontChainAccepted(t, tpl, `{"kind":"removeFontChainEntry","version":1,"name":"body","index":1}`)
	if got := fontChainOf(t, tpl, "body"); !reflect.DeepEqual(got, []string{"Nonesuch Display", "Noto Sans"}) {
		t.Fatalf("after remove = %#v, want the order of the rest preserved", got)
	}
}

func TestFontChainEntryCommandsRefuseAnIndexOutOfRange(t *testing.T) {
	for _, command := range []string{
		`{"kind":"addFontChainEntry","version":1,"name":"body","index":3,"face":"Noto Sans"}`,
		`{"kind":"addFontChainEntry","version":1,"name":"body","index":-1,"face":"Noto Sans"}`,
		`{"kind":"moveFontChainEntry","version":1,"name":"body","from":0,"to":2}`,
		`{"kind":"moveFontChainEntry","version":1,"name":"body","from":2,"to":0}`,
		`{"kind":"removeFontChainEntry","version":1,"name":"body","index":2}`,
	} {
		if failure := fontChainRefusal(t, fontChainTemplate(t), command); failure.Message != "entry index is out of range" {
			t.Errorf("%s refusal = %q", command, failure.Message)
		}
	}
}

func TestFontChainCommandsRefuseAnUndeclaredChain(t *testing.T) {
	for _, command := range []string{
		`{"kind":"renameFontChain","version":1,"name":"missing","to":"brand"}`,
		`{"kind":"deleteFontChain","version":1,"name":"missing"}`,
		`{"kind":"addFontChainEntry","version":1,"name":"missing","index":0,"face":"Noto Sans"}`,
		`{"kind":"moveFontChainEntry","version":1,"name":"missing","from":0,"to":0}`,
		`{"kind":"removeFontChainEntry","version":1,"name":"missing","index":0}`,
	} {
		failure := fontChainRefusal(t, fontChainTemplate(t), command)
		if failure.Message != `no font chain named "missing" is declared` || failure.DataPath != "fonts.missing" {
			t.Errorf("%s refusal = %q at %q", command, failure.Message, failure.DataPath)
		}
	}
}

func TestFontChainAddRefusesANameOverTheProjectionBound(t *testing.T) {
	// Refused AT THE COMMAND, located, rather than left to canvasFontChains'
	// unlocated bare error later in the same transaction.
	long := strings.Repeat("f", maxCanvasPropertyString+1)
	failure := fontChainRefusal(t, fontChainTemplate(t), `{"kind":"addFontChain","version":1,"name":"`+long+`","entries":["Noto Sans"]}`)
	if failure.Message != "font chain name exceeds the projection bound" {
		t.Fatalf("over-long name refusal = %q", failure.Message)
	}
	edge := strings.Repeat("f", maxCanvasPropertyString)
	tpl := fontChainTemplate(t)
	fontChainAccepted(t, tpl, `{"kind":"addFontChain","version":1,"name":"`+edge+`","entries":["Noto Sans"]}`)
}

func TestFontChainAddRefusesMoreChainsThanTheProjectionBound(t *testing.T) {
	tpl := fontChainTemplate(t)
	for len(tpl.doc.Fonts) < maxCanvasFontFamilies {
		fontChainAccepted(t, tpl, fmt.Sprintf(`{"kind":"addFontChain","version":1,"name":"c%04d","entries":["Noto Sans"]}`, len(tpl.doc.Fonts)))
	}
	failure := fontChainRefusal(t, tpl, `{"kind":"addFontChain","version":1,"name":"onetoomany","entries":["Noto Sans"]}`)
	if failure.Message != "document declares more font chains than the projection bound" {
		t.Fatalf("over-count refusal = %q", failure.Message)
	}
}

func TestFontChainEntryCountIsBoundedAtTheCommand(t *testing.T) {
	tpl := fontChainTemplate(t)
	entries := make([]string, maxCanvasFontChainEntries+1)
	for i := range entries {
		entries[i] = fmt.Sprintf("%q", fmt.Sprintf("face-%d", i))
	}
	failure := fontChainRefusal(t, tpl, `{"kind":"addFontChain","version":1,"name":"deep","entries":[`+strings.Join(entries, ",")+`]}`)
	if failure.Message != "a font chain declares more entries than the projection bound" {
		t.Fatalf("over-deep chain refusal = %q", failure.Message)
	}
	fontChainAccepted(t, tpl, `{"kind":"addFontChain","version":1,"name":"deep","entries":[`+strings.Join(entries[:maxCanvasFontChainEntries], ",")+`]}`)
	if failure := fontChainRefusal(t, tpl, `{"kind":"addFontChainEntry","version":1,"name":"deep","index":0,"face":"Noto Sans"}`); failure.Message != "a font chain declares more entries than the projection bound" {
		t.Fatalf("over-deep insert refusal = %q", failure.Message)
	}
}

// TestFontChainRenameMovesTheDefaultANewTextElementAdopts records
// defaultFontFamily's coupling to the chain KEYS rather than leaving it
// unmeasured: D-4.1.1 rejected an alphabetical default on exactly this hazard,
// and a rename is now a way an author can move it.
func TestFontChainRenameMovesTheDefaultANewTextElementAdopts(t *testing.T) {
	tpl := fontChainTemplate(t)
	if got := defaultFontFamily(tpl); got != "body" {
		t.Fatalf("default family = %q, want the sorted-first declared chain", got)
	}
	fontChainAccepted(t, tpl, `{"kind":"renameFontChain","version":1,"name":"body","to":"zbody"}`)
	if got := defaultFontFamily(tpl); got != "heading" {
		t.Fatalf("default family after the rename = %q, want the new sorted-first chain", got)
	}
	before, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	projection := fontChainAccepted(t, tpl, `{"kind":"createComponent","version":1,"type":"text","band":"content","x":12,"y":60,"width":72,"height":24,"snap":false}`)
	created := newProjectedComponent(t, before, projection)
	if created.FontFamily == nil || *created.FontFamily != "heading" {
		t.Fatalf("a text element placed after the rename adopted %v", created.FontFamily)
	}
}

// TestEmptyFontChainIsInvisibleToTheProjectionAndRefusedByTheProperty is the
// departed population of the five open-coded guards Fonts.Chain replaced: a
// declared-but-empty chain. decodeFonts accepts one at load (this story does
// NOT narrow the loader), so it must still be invisible to the projection and
// still refused by the fontFamily property command — and it must stay
// deletable, because "declared" and "nameable" are deliberately different
// questions.
func TestEmptyFontChainIsInvisibleToTheProjectionAndRefusedByTheProperty(t *testing.T) {
	tpl, err := ParseTemplate([]byte(strings.Replace(fontChainDocJSON, `"unused": ["Noto Sans SC"]`, `"unused": []`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if chain, ok := tpl.doc.Fonts.Chain("unused"); ok || chain != nil {
		t.Fatalf("Fonts.Chain accepted an empty chain: %#v", chain)
	}
	if _, declared := tpl.doc.Fonts["unused"]; !declared {
		t.Fatal("the loader dropped the empty chain; this story does not narrow decodeFonts")
	}
	projection, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(projection.FontFamilies, []string{"body", "heading"}) {
		t.Fatalf("an empty chain reached the projection: %#v", projection.FontFamilies)
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"updateComponentProperties","version":1,"ids":["e2"],"changes":{"fontFamily":{"op":"set","value":"unused"}}}`)); err == nil {
		t.Fatal("the property command accepted an empty chain")
	}
	// Still deletable: an empty chain a .folio in the wild carries must not
	// become unreachable to every command at once.
	fontChainAccepted(t, tpl, `{"kind":"deleteFontChain","version":1,"name":"unused"}`)
}

// TestFontChainAddRefusesEveryMalformedEntryList covers the addFontChain
// refusals the matrix names but no test reached: entries absent, entries that
// are not a string array, an entry that is the empty string, and an entry over
// the projection's identifier bound. Each is proved by DELETING its guard —
// the cheapest screen for a subject the suite never reaches at all, which is
// what every one of these was before this test existed.
func TestFontChainAddRefusesEveryMalformedEntryList(t *testing.T) {
	long := strings.Repeat("f", maxCanvasPropertyString+1)
	for _, probe := range []struct{ command, want string }{
		// componentFields counts an exact arity, so "entries absent" must
		// still be four fields: a MISSPELLED key, which is the shape the
		// mistake actually takes.
		{`{"kind":"addFontChain","version":1,"name":"caption","faces":["Noto Sans"]}`, "font chain entries are required"},
		// STORY 11.4 MOVED TWO OF THESE SENTENCES AND NO VERDICT. `entries` is
		// no longer an array of strings — it is an array of the format's own
		// chain entries — so a message saying "string array" would send the
		// author to fix the one thing that was not wrong. The INPUTS and the
		// refusals are the same;
		// TestRouteCWidenedAFieldsShapeAndNoDoorsVERDICT is what asserts that.
		{`{"kind":"addFontChain","version":1,"name":"caption","entries":"Noto Sans"}`, "font chain entries must be an array of font chain entries"},
		{`{"kind":"addFontChain","version":1,"name":"caption","entries":[7]}`, "a font chain entry must be " + fontChainEntryShape},
		{`{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans",""]}`, "a font chain entry must be a non-empty string"},
		{`{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans","` + long + `"]}`, "font chain entry exceeds the projection bound"},
	} {
		failure := fontChainRefusal(t, fontChainTemplate(t), probe.command)
		if failure.Message != probe.want {
			t.Errorf("refusal = %q, want %q\n  for %s", failure.Message, probe.want, probe.command)
		}
		if failure.DataPath != "fonts.caption" {
			t.Errorf("refusal is located at %q, want fonts.caption\n  for %s", failure.DataPath, probe.command)
		}
	}
}

// TestFontChainRenameRefusesADestinationOverTheProjectionBound is the `to`
// half of fontChainName's length guard. The `name` half has its own test; this
// one is the argument a rename-shaped command reaches and an add-shaped one
// never does.
func TestFontChainRenameRefusesADestinationOverTheProjectionBound(t *testing.T) {
	long := strings.Repeat("f", maxCanvasPropertyString+1)
	failure := fontChainRefusal(t, fontChainTemplate(t), `{"kind":"renameFontChain","version":1,"name":"body","to":"`+long+`"}`)
	if failure.Message != "font chain name exceeds the projection bound" {
		t.Fatalf("over-long destination refusal = %q", failure.Message)
	}
	edge := strings.Repeat("f", maxCanvasPropertyString)
	fontChainAccepted(t, fontChainTemplate(t), `{"kind":"renameFontChain","version":1,"name":"body","to":"`+edge+`"}`)
}

// TestFontChainRefusalDataPathSurvivesTheHostsCut is the located-ness half of
// the over-long-name refusal, and the one case where locating the name is the
// entire point. The host cuts DataPath at maxComponentDataPathBytes and its
// bounded() slices by BYTES, so a name of multi-byte runes would arrive at the
// author split through the middle of a character unless it is cut here first.
func TestFontChainRefusalDataPathSurvivesTheHostsCut(t *testing.T) {
	// 257 two-byte runes: over maxCanvasPropertyString, so the command refuses
	// it, and far over the DataPath bound, so the path must be trimmed.
	long := strings.Repeat("é", maxCanvasPropertyString/2+1)
	if len(long) <= maxCanvasPropertyString {
		t.Fatalf("fixture precondition: the name must exceed the identifier bound, got %d bytes", len(long))
	}
	failure := fontChainRefusal(t, fontChainTemplate(t), `{"kind":"addFontChain","version":1,"name":"`+long+`","entries":["Noto Sans"]}`)
	if failure.Message != "font chain name exceeds the projection bound" {
		t.Fatalf("over-long name refusal = %q", failure.Message)
	}
	if len(failure.DataPath) > maxComponentDataPathBytes {
		t.Fatalf("DataPath is %d bytes; the host cuts at %d and would take this one apart itself", len(failure.DataPath), maxComponentDataPathBytes)
	}
	if !utf8.ValidString(failure.DataPath) {
		t.Fatalf("DataPath is not valid UTF-8: %q", failure.DataPath)
	}
	if !strings.HasPrefix(failure.DataPath, "fonts.") {
		t.Fatalf("DataPath = %q, want the fonts.<name> shape even trimmed", failure.DataPath)
	}
	// And a name that FITS is not trimmed: the guard must not cost every
	// ordinary refusal its path.
	if failure := fontChainRefusal(t, fontChainTemplate(t), `{"kind":"deleteFontChain","version":1,"name":"missing"}`); failure.DataPath != "fonts.missing" {
		t.Fatalf("an ordinary refusal lost its path: %q", failure.DataPath)
	}
}

// TestFontChainOrphanMessageTrimsOnAWholeIdBoundary tests fontChainOrphanMessage
// DIRECTLY, because the two delete refusals that reach it in the wild carry
// lists of about fifty bytes against a five-hundred-byte budget and so take
// only its fall-through. Its trimming branch, its " and N more" suffix, its
// whole-id guarantee and its "%d elements" fallback are all reachable — an
// author can declare hundreds of elements naming one chain — and all four were
// unexecuted.
func TestFontChainOrphanMessageTrimsOnAWholeIdBoundary(t *testing.T) {
	ids := make([]string, 200)
	for i := range ids {
		ids[i] = fmt.Sprintf("element-%03d", i)
	}
	message := fontChainOrphanMessage("body", ids)
	if len(message) > maxComponentFailureMessageBytes {
		t.Fatalf("the message is %d bytes; the host cuts at %d", len(message), maxComponentFailureMessageBytes)
	}
	prefix := `font chain "body" is still named by `
	body, ok := strings.CutPrefix(message, prefix)
	if !ok {
		t.Fatalf("message = %q, want the %q shape", message, prefix)
	}
	listed, suffix, ok := strings.Cut(body, " and ")
	if !ok || !strings.HasSuffix(suffix, " more") {
		t.Fatalf("a trimmed list must close with \" and N more\": %q", message)
	}
	remaining, err := strconv.Atoi(strings.TrimSuffix(suffix, " more"))
	if err != nil {
		t.Fatalf("the suffix does not carry a count: %q", suffix)
	}
	// THE WHOLE-ID GUARANTEE, which is the point of trimming here rather than
	// letting the host cut: every id named is named in full and in order, and
	// the count accounts for exactly the ones that were left out.
	named := strings.Split(listed, ", ")
	if len(named)+remaining != len(ids) {
		t.Fatalf("%d ids named plus %d more is not the %d that reference the chain", len(named), remaining, len(ids))
	}
	for i, id := range named {
		if id != ids[i] {
			t.Fatalf("id %d in the message is %q, want the whole id %q — the trim cut through an id", i, id, ids[i])
		}
	}
	// One more id would have to fit for the trim to be honest about where it
	// stopped: the message must be at the budget, not arbitrarily short.
	if len(message)+len(", "+ids[len(named)]) <= maxComponentFailureMessageBytes-len(" and 0 more") {
		t.Fatalf("the message stopped at %d bytes with room for another id", len(message))
	}
}

// TestFontChainOrphanMessageFitsEveryNameTheCommandAccepts is the negative
// budget. fontChainName admits a name up to maxCanvasPropertyString, which
// alone exceeds the width the host cuts a message to, so without a trim on the
// NAME every branch of fontChainOrphanMessage overruns and the host cuts
// through the middle of it — defeating the whole-id guarantee the function's
// own doc comment makes. Measured at both the edge and past it, and with
// multi-byte runes, where the host's byte cut would split a character.
func TestFontChainOrphanMessageFitsEveryNameTheCommandAccepts(t *testing.T) {
	for _, probe := range []struct {
		name  string
		chain string
		ids   []string
	}{
		{"the longest name the command accepts", strings.Repeat("f", maxCanvasPropertyString), []string{"e2", "e7"}},
		{"a long name of multi-byte runes", strings.Repeat("é", maxCanvasPropertyString/2), []string{"e2", "e7"}},
		{"a long name of characters %q escapes", strings.Repeat(`"`, maxCanvasPropertyString), []string{"e2", "e7"}},
		{"a long name and a long id list", strings.Repeat("f", maxCanvasPropertyString), []string{strings.Repeat("i", 200), strings.Repeat("j", 200)}},
		{"an ordinary name", "body", []string{"e2", "e7"}},
	} {
		t.Run(probe.name, func(t *testing.T) {
			message := fontChainOrphanMessage(probe.chain, probe.ids)
			if len(message) > maxComponentFailureMessageBytes {
				t.Fatalf("the message is %d bytes; the host cuts at %d and the cut lands inside the name", len(message), maxComponentFailureMessageBytes)
			}
			if !utf8.ValidString(message) {
				t.Fatalf("the message is not valid UTF-8: %q", message)
			}
			// Whatever it had to give up, it never gives up the count: an
			// author told "still named by" and nothing else has been told
			// nothing.
			if !strings.Contains(message, "is still named by ") {
				t.Fatalf("message = %q", message)
			}
		})
	}
	// The fallback: not even one id fits beside a name this long, and the
	// count is all that is left to say.
	message := fontChainOrphanMessage(strings.Repeat("f", maxCanvasPropertyString), []string{strings.Repeat("i", 200), strings.Repeat("j", 200)})
	if !strings.HasSuffix(message, " 2 elements") {
		t.Fatalf("message = %q, want the \"%%d elements\" fallback", message)
	}
}

// componentFailureHostBounds reads the widths the wasm host actually cuts a
// ComponentCommandError to, out of the host's own source.
var componentFailureHostBounds = regexp.MustCompile(`bounded\(componentErr\.(Message|DataPath), (\d+)\)`)

// TestComponentFailureBoundsMatchTheHostsOwnLiterals ties
// maxComponentFailureMessageBytes and maxComponentDataPathBytes to the numbers
// they were hand-copied from. This is the one-sided-constant defect this story
// fixed for maxCanvasFontFamilies on the designer's side of the seam, and it
// applies here for a reason peculiar to Go: wasm/cmd/engine is
// //go:build js && wasm, so `go test ./...` never compiles it and no ordinary
// test can see those literals at all. Reading the source is the mechanism
// canvas_projection_wire_test.go already uses for engine-protocol.ts, and it
// is the mechanism available here.
//
// If it goes red: the host was retuned and these constants were not, which
// means a message this module trimmed to fit is now cut again by the host —
// through the middle of an element id, which is precisely the outcome
// fontChainOrphanMessage exists to prevent.
func TestComponentFailureBoundsMatchTheHostsOwnLiterals(t *testing.T) {
	path := filepath.Join(repoRootFromTest(t), "folio-go", "wasm", "cmd", "engine", "main.go")
	source, err := os.ReadFile(path)
	if err != nil {
		// Not a skip. The other side of this seam is part of the contract,
		// and a missing one is a finding, not an excuse.
		t.Fatalf("read the wasm host: %v", err)
	}
	found := map[string]int{}
	for _, match := range componentFailureHostBounds.FindAllStringSubmatch(string(source), -1) {
		width, err := strconv.Atoi(match[2])
		if err != nil {
			t.Fatalf("host bound %q is not a number", match[2])
		}
		found[match[1]] = width
	}
	for _, pair := range []struct {
		field string
		here  int
	}{
		{"Message", maxComponentFailureMessageBytes},
		{"DataPath", maxComponentDataPathBytes},
	} {
		width, ok := found[pair.field]
		if !ok {
			t.Fatalf("the host no longer bounds componentErr.%s where this test can read it; if engineFailure was restructured, re-derive this extraction rather than deleting the check", pair.field)
		}
		if width != pair.here {
			t.Errorf("the host cuts componentErr.%s at %d and this module trims to %d — one side of the seam moved and the other did not", pair.field, width, pair.here)
		}
	}
}

// ---------------------------------------------------------------------------
// STORY 8.6: THE PICK, THE DEDUPE, THE DROP — AND DW-80.

// embedCommand builds the pick the designer sends. Written out here rather
// than assembled from a helper because THE FIELD COUNT IS PART OF THE CONTRACT
// (componentFields(raw, 12) counts kind and version too), and a builder that
// quietly omitted a key would move the refusal this file is measuring.
func embedCommand(t *testing.T, chain string, face []byte, tail string) string {
	t.Helper()
	return embedCommandDeclaring(t, chain, face, tail, "OFL-1.1")
}

// embedCommandDeclaring is the same pick with the declared SPDX id under the
// caller's control. Story 16.1b needs it: the licence field stopped being
// inert the moment fontset.RefuseContradictedLicence began reading the bytes
// beside it, so a test embedding Roboto — whose own name table names the
// Apache License — must say Apache-2.0 or be measuring a refusal it did not
// mean to provoke.
func embedCommandDeclaring(t *testing.T, chain string, face []byte, tail string, licence string) string {
	t.Helper()
	return `{"kind":"embedFontFamily","version":1,"name":` + quoteForCommand(t, chain) +
		`,"family":"Noto Sans Thai","style":"Regular","licence":` + quoteForCommand(t, licence) +
		`,"licenceText":"This Font Software is licensed under the SIL Open Font License, Version 1.1."` +
		`,"copyright":"Copyright 2022 The Noto Project Authors","source":"catalogue"` +
		`,"mediaType":"font/ttf","data":"` + base64.StdEncoding.EncodeToString(face) + `","tail":` + tail + `}`
}

func quoteForCommand(t *testing.T, value string) string {
	t.Helper()
	out, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// embeddedKeyOf is the format's own rule for the key, derived rather than
// written down — the same reason embeddedFontAssetKey() derives the fixture's.
func embeddedKeyOf(face []byte) string { return fmt.Sprintf("%x", sha256.Sum256(face)) }

// TestEmbedFontFamilyWritesTheAssetAndDeclaresTheChain is AC1 and AC6 at the
// command surface: ONE command puts the bytes in the document, writes the
// whole record — licence text and copyright included — and declares a chain
// naming the asset by key with the proposed tail behind it.
func TestEmbedFontFamilyWritesTheAssetAndDeclaresTheChain(t *testing.T) {
	tpl := fontChainTemplate(t)
	face := testShippedNotoSansThai
	key := embeddedKeyOf(face)

	// ⚠ THE TAIL IS WRITTEN IN THE OBJECT FORM, and it has to be. A mutation
	// that rebuilt every tail entry as template.FaceEntry(e.Face) — dropping
	// EVERY declared cut on EVERY tail entry — left the whole Go suite green,
	// because nothing on this side ever read a tail entry's cuts back. A tail
	// of bare strings cannot see that regression at all.
	fontChainAccepted(t, tpl, embedCommand(t, "Noto Sans Thai", face,
		`[{"face":"Noto Sans","bold":"Noto Sans Bold","italic":"Noto Sans Italic","boldItalic":"Noto Sans Bold Italic"},"Noto Sans SC"]`))

	asset, ok := tpl.doc.Assets[key]
	if !ok {
		t.Fatalf("the picked face was not stored under its own content hash %s", key)
	}
	if asset.MediaType != "font/ttf" {
		t.Errorf("mediaType = %q, want font/ttf", asset.MediaType)
	}
	if !asset.Font.Set || asset.Font.Null {
		t.Fatal("the asset carries no font record at all")
	}
	// AC6 SPELLED OUT, KEY BY KEY. A record that carried the display half and
	// dropped the terms is the failure this asserts against, and it is the one
	// a writer would produce by re-using Story 8.3's four-key record.
	for _, want := range []struct {
		name  string
		value template.Presence[string]
	}{
		{"family", asset.Font.Value.Family},
		{"style", asset.Font.Value.Style},
		{"licence", asset.Font.Value.Licence},
		{"licenceText", asset.Font.Value.LicenceText},
		{"copyright", asset.Font.Value.Copyright},
		{"source", asset.Font.Value.Source},
	} {
		if !want.value.Set || want.value.Null || want.value.Value == "" {
			t.Errorf("the recorded font carries no %s (%#v)", want.name, want.value)
		}
	}

	chain, ok := tpl.doc.Fonts["Noto Sans Thai"]
	if !ok {
		t.Fatal("the pick declared no chain")
	}
	if len(chain) != 3 || !chain[0].Embedded() || chain[0].AssetKey != key {
		t.Fatalf("chain = %#v — want the picked face first, named by asset key", chain)
	}
	if chain[1].Face != "Noto Sans" || chain[2].Face != "Noto Sans SC" {
		t.Errorf("the proposed tail did not survive: %#v", chain[1:])
	}
	// AND NEITHER DID ITS DECLARED CUTS. The tail is where a pick carries what
	// the engine ships for the scripts the embedded face does not cover, so a
	// tail that arrives stripped of its cuts silently un-bolds every fallback
	// run in the document — with no warning anywhere, because an entry that
	// declares no cut is a legal entry.
	for _, want := range []struct {
		at    int
		style template.FontStyle
		cut   string
	}{
		{1, template.FontStyleBold, "Noto Sans Bold"},
		{1, template.FontStyleItalic, "Noto Sans Italic"},
		{1, template.FontStyleBoldItalic, "Noto Sans Bold Italic"},
		// D-A, and ORDINARY: Noto Sans SC has no cut at any weight, so the
		// entry declares none and stays a bare face name.
		{2, template.FontStyleBold, ""},
	} {
		if got := chain[want.at].Variant(want.style); got != want.cut {
			t.Errorf("tail entry %d declares %v = %q, want %q", want.at, want.style, got, want.cut)
		}
	}
	// NON-VACUITY of the "" row above, and of the whole block: at least one cut
	// really did reach the model, so a writer that dropped every one of them
	// could not pass this test by agreeing with the empty expectations.
	if chain[1].Bold == "" {
		t.Fatal("no tail cut reached the model at all, so every expectation above was vacuous")
	}
	// AND THE PICKED ENTRY ITSELF DECLARES NO CUT, which is the claim the
	// designer test used to make with an assertion that could not fail — it
	// inspected the TOP LEVEL of the command payload, where no variant key has
	// ever been able to appear (D-11.2.8). The entry is built inside this
	// function from the bytes it hashed, and here is where it can be read back.
	// One embedded face is one face: a pick carries a single upright Regular
	// and an entry may declare only a cut the document actually holds.
	for _, style := range []template.FontStyle{template.FontStyleBold, template.FontStyleItalic, template.FontStyleBoldItalic} {
		if got := chain[0].Variant(style); got != "" {
			t.Errorf("the embedded entry declares %v = %q; a pick embeds ONE face and has no cut to declare", style, got)
		}
	}
	// AND THE DOCUMENT IS STILL A DOCUMENT. applyFontChainCommand reparses its
	// own canonical bytes, so this also proves the writer cannot emit a record
	// its own parser rejects — the whole reason the command validates the
	// three required keys up front.
	if _, err := ParseTemplate(mustSerialize(t, tpl)); err != nil {
		t.Fatalf("the document the pick produced does not load: %v", err)
	}
}

// TestEmbedFontFamilyRefusesAFaceThatCannotStateItsTerms is the guard that
// keeps the writer and the parser from disagreeing. Each row empties ONE
// required key, so a guard written for only some of them is visible.
//
// WHAT IT ASSERTS, AND WHY IT IS NOT A MESSAGE SUBSTRING. It used to check
// `strings.Contains(failure.Message, missing)`, which was a check that could
// not fail: the refusal sentence for a missing `licence` contains the word
// "licence", and so does the one for `licenceText` — so the licenceText row
// passed whether the licenceText guard fired or the licence guard did, and the
// six rows did not distinguish between themselves at all. It now asserts the
// exact message the field's own guard emits, which no other field's guard can
// produce.
//
// The DataPath is asserted too, but separately and for a different reason: a
// chain command is not addressed to an element, so `fonts.<name>` is the only
// location it carries, and it is the same for every row. It says the refusal
// is located; the message says WHICH key.
func TestEmbedFontFamilyRefusesAFaceThatCannotStateItsTerms(t *testing.T) {
	face := testShippedNotoSansThai
	for _, missing := range []string{"family", "style", "licence", "licenceText", "copyright", "source"} {
		for _, spelling := range []struct {
			name  string
			value string
		}{
			// Emptied rather than deleted: componentFields counts the keys, so
			// deleting one would be refused on ARITY and the row would pass
			// without ever reaching the rule it is about.
			{"empty", `""`},
			// And BLANK, because the parser refuses whitespace-only terms and
			// a command that admitted them would produce a document its own
			// reparse rejects — unlocated, as "font chains did not pass format
			// validation", which tells the author nothing.
			{"blank", `" \n "`},
		} {
			t.Run(missing+"/"+spelling.name, func(t *testing.T) {
				var fields map[string]json.RawMessage
				if err := json.Unmarshal([]byte(embedCommand(t, "Noto Sans Thai", face, `[]`)), &fields); err != nil {
					t.Fatal(err)
				}
				fields[missing] = json.RawMessage(spelling.value)
				rebuilt, err := json.Marshal(fields)
				if err != nil {
					t.Fatal(err)
				}
				failure := fontChainRefusal(t, fontChainTemplate(t), string(rebuilt))
				if want := "folio8: " + missing + " must be a non-empty string"; failure.Message != want {
					t.Errorf("refusal message = %q, want %q — the message has to identify WHICH key was short, and a substring check cannot: every one of these sentences contains the word \"licence\"", failure.Message, want)
				}
				if failure.DataPath != "fonts.Noto Sans Thai" {
					t.Errorf("the refusal is located at %q, want the chain the pick names", failure.DataPath)
				}
			})
		}
	}
}

// TestEmbedFontFamilyRePickStoresNoSecondCopy is AC2: the content hash
// decides. A second pick of a family already embedded declares no second
// asset, no second chain, and leaves the canonical bytes untouched — which is
// what makes it push no undo entry (asserted end-to-end in wasm/).
func TestEmbedFontFamilyRePickStoresNoSecondCopy(t *testing.T) {
	tpl := fontChainTemplate(t)
	face := testShippedNotoSansThai
	command := embedCommand(t, "Noto Sans Thai", face, `["Noto Sans"]`)

	fontChainAccepted(t, tpl, command)
	first := mustSerialize(t, tpl)

	fontChainAccepted(t, tpl, command)
	second := mustSerialize(t, tpl)

	if !bytes.Equal(first, second) {
		t.Fatal("a re-pick of an already-embedded family moved the document's canonical bytes")
	}
	if len(tpl.doc.Assets) != 1 {
		t.Errorf("the document carries %d assets, want 1 — the same bytes must store once", len(tpl.doc.Assets))
	}
	// A re-pick under a DIFFERENT chain name is still the same face, so it is
	// still the same no-op: the key is what the document is asked about, never
	// the name the author happened to type.
	fontChainAccepted(t, tpl, embedCommand(t, "Brand", face, `["Noto Sans"]`))
	if _, exists := tpl.doc.Fonts["Brand"]; exists {
		t.Error("a re-pick of an already-embedded face declared a SECOND chain; the existing one is what the author is offered")
	}
}

// TestEmbedFontFamilyRePicksAfterTheChainWasDeleted is the third row of the
// matrix, and it is the one that separates "dedupe on the ASSET" from "dedupe
// on the CHAIN": the asset is still in the document, nothing names it, and the
// pick must re-declare the chain WITHOUT storing the bytes again.
func TestEmbedFontFamilyRePicksAfterTheChainWasDeleted(t *testing.T) {
	tpl := fontChainTemplate(t)
	face := testShippedNotoSansThai
	key := embeddedKeyOf(face)
	command := embedCommand(t, "Noto Sans Thai", face, `["Noto Sans"]`)
	fontChainAccepted(t, tpl, command)

	// Un-name it WITHOUT letting the drop collect it: a second chain holds the
	// key while the first is deleted, so the asset survives to be re-found.
	tpl.doc.Fonts["keeper"] = []template.FontChainEntry{template.AssetEntry(key)}
	fontChainAccepted(t, tpl, `{"kind":"deleteFontChain","version":1,"name":"Noto Sans Thai"}`)
	if _, ok := tpl.doc.Assets[key]; !ok {
		t.Fatal("precondition: the asset a second chain still names must not have been collected")
	}
	delete(tpl.doc.Fonts, "keeper")

	fontChainAccepted(t, tpl, command)
	if len(tpl.doc.Assets) != 1 {
		t.Errorf("the re-pick stored %d assets, want 1 — the bytes were already carried", len(tpl.doc.Assets))
	}
	chain, ok := tpl.doc.Fonts["Noto Sans Thai"]
	if !ok || len(chain) == 0 || chain[0].AssetKey != key {
		t.Fatalf("the re-pick did not re-declare a chain naming the existing key: %#v", chain)
	}
}

// TestAssetKeyReferencedSeesAFontChain IS DW-80, STATED AS A TEST.
//
// RED-PROVED BY DELETING THE FONT ARM of assetKeyReferenced, not by
// falsifying the image condition: the defect was that the walk never read
// t.doc.Fonts at all and answered false for every font asset, so the mutation
// that must red is the one that restores that state. Falsifying the IMAGE
// condition would red this too, and would prove nothing about the font arm.
func TestAssetKeyReferencedSeesAFontChain(t *testing.T) {
	tpl := fontChainTemplate(t)
	face := testShippedNotoSansThai
	key := embeddedKeyOf(face)
	fontChainAccepted(t, tpl, embedCommand(t, "Noto Sans Thai", face, `[]`))

	if !assetKeyReferenced(tpl, key) {
		t.Fatal("assetKeyReferenced answered FALSE for an asset a live font chain names (DW-80) — the orphan drop built on this answer deletes a face the document is still drawing with")
	}
	// The negative arm, so the positive one is not vacuous: an unrelated key
	// is still unreferenced.
	if assetKeyReferenced(tpl, strings.Repeat("0", 64)) {
		t.Error("assetKeyReferenced answered TRUE for a key nothing names")
	}
}

// TestRemovingTheLastNamingEntryDropsTheFace is AC5, and
// TestAFaceASecondChainStillNamesIsRetained below is the arm DW-80 would have
// got wrong. They are written as a pair because either alone is satisfied by
// an implementation that is wrong in the other direction — "always delete"
// passes the first, "never delete" passes the second.
func TestRemovingTheLastNamingEntryDropsTheFace(t *testing.T) {
	tpl := fontChainTemplate(t)
	face := testShippedNotoSansThai
	key := embeddedKeyOf(face)
	fontChainAccepted(t, tpl, embedCommand(t, "Noto Sans Thai", face, `["Noto Sans"]`))
	if _, ok := tpl.doc.Assets[key]; !ok {
		t.Fatal("precondition: the pick stored no asset")
	}

	fontChainAccepted(t, tpl, `{"kind":"removeFontChainEntry","version":1,"name":"Noto Sans Thai","index":0}`)

	if _, ok := tpl.doc.Assets[key]; ok {
		t.Error("the face nothing names any longer was left in the document — a file must not accumulate megabytes of faces nothing draws with")
	}
	// SCOPED, NEVER A SWEEP. The document's other assets are untouched, which
	// is what distinguishes this from a garbage collection pass.
	if len(tpl.doc.Fonts["Noto Sans Thai"]) != 1 {
		t.Errorf("the chain itself was disturbed beyond the removed entry: %#v", tpl.doc.Fonts["Noto Sans Thai"])
	}
}

func TestAFaceASecondChainStillNamesIsRetained(t *testing.T) {
	tpl := fontChainTemplate(t)
	face := testShippedNotoSansThai
	key := embeddedKeyOf(face)
	fontChainAccepted(t, tpl, embedCommand(t, "Noto Sans Thai", face, `["Noto Sans"]`))

	// A SECOND chain over the same key. Declared directly because no command
	// authors a second embedded entry over an existing key — embedFontFamily
	// deliberately no-ops when one already names it (AC2).
	tpl.doc.Fonts["alsoThai"] = []template.FontChainEntry{template.AssetEntry(key), template.FaceEntry("Noto Sans")}

	fontChainAccepted(t, tpl, `{"kind":"removeFontChainEntry","version":1,"name":"Noto Sans Thai","index":0}`)

	if _, ok := tpl.doc.Assets[key]; !ok {
		t.Fatal("a face a SECOND chain still names was deleted — this is the arm DW-80's broken walk would have got wrong, and the deletion direction is the dangerous one")
	}
}

// TestDeletingAChainDropsTheFacesOnlyItNamed applies AC5 to the other action
// that un-names an entry. Deleting a chain un-names every entry in it at once;
// the collection is the same, scoped to exactly those entries.
func TestDeletingAChainDropsTheFacesOnlyItNamed(t *testing.T) {
	tpl := fontChainTemplate(t)
	face := testShippedNotoSansThai
	key := embeddedKeyOf(face)
	fontChainAccepted(t, tpl, embedCommand(t, "Noto Sans Thai", face, `["Noto Sans"]`))

	fontChainAccepted(t, tpl, `{"kind":"deleteFontChain","version":1,"name":"Noto Sans Thai"}`)
	if _, ok := tpl.doc.Assets[key]; ok {
		t.Error("deleting the only chain naming a face left the face behind")
	}
}

// TestEmbedFontFamilyRefusesAChainNameTheDocumentAlreadyTakes is the branch
// that stops a pick from SILENTLY REPLACING a chain the author already has.
//
// WHY IT NEEDS ITS OWN TEST, MEASURED: deleting the collision branch left the
// whole folio-go suite green. Every other embed test either dedupes first
// (the key is already named, so the command no-ops before it ever looks at the
// name) or picks a free name — so the one path that reaches
// `t.doc.Fonts[name] = entries` over an EXISTING chain was reachable by no
// test at all. Without the branch that assignment discards the old chain's
// entries, and because embedFontFamily never calls dropUnnamedFontAssets, a
// face the discarded chain named is left behind as an orphan nothing can
// reach: the document silently loses a chain and silently keeps the bytes.
//
// A NEW FACE, DELIBERATELY. The bytes are Noto Sans rather than the Thai face
// the other embed tests use, so the document does not already carry this key
// and the dedupe short-circuit cannot answer first — the command must reach
// the name question for this test to be about the name question.
func TestEmbedFontFamilyRefusesAChainNameTheDocumentAlreadyTakes(t *testing.T) {
	tpl := fontChainTemplate(t)
	face := testShippedNotoSans
	key := embeddedKeyOf(face)
	if _, exists := tpl.doc.Assets[key]; exists {
		t.Fatal("precondition: the document already carries this face, so the dedupe short-circuit would answer before the name is ever considered")
	}
	before := append([]template.FontChainEntry(nil), tpl.doc.Fonts["body"]...)

	// fontChainRefusal also asserts the document is byte-identical afterwards,
	// which is the strongest statement of "the existing chain is unchanged" —
	// a partially applied pick is exactly what the one-transaction shape
	// exists to prevent.
	failure := fontChainRefusal(t, tpl, embedCommand(t, "body", face, `["Noto Sans SC"]`))
	if !strings.Contains(failure.Message, `a font chain named "body" already exists`) {
		t.Errorf("the refusal does not say the name is taken: %s", failure.Message)
	}
	if failure.DataPath != "fonts.body" {
		t.Errorf("the refusal is located at %q, want fonts.body — the author has to be told WHICH name is taken", failure.DataPath)
	}
	// Stated directly as well, because the byte comparison above would also be
	// satisfied by a command that refused for some unrelated reason before
	// touching anything.
	if !slices.Equal(tpl.doc.Fonts["body"], before) {
		t.Errorf("the existing chain was disturbed by a refused pick: %#v, want %#v", tpl.doc.Fonts["body"], before)
	}
	if _, exists := tpl.doc.Assets[key]; exists {
		t.Error("a refused pick stored the face anyway")
	}
}

// ---------------------------------------------------------------------------
// STORY 16.0: THE SAME BYTES, THE TWO DOORS, THE ONE PREDICATE.

// TestVariableFaceIsRefusedAtTheCommandAndAtIngestionOverTheSameBytes is the
// engineering lead's guardrail from this story's plan gate, written as a test
// rather than as a note.
//
// D-16.6 measured the disagreement: embedFontFamily's only structural gate was
// checkSfnt, which does not look at `fvar`, while fontset.New refuses `fvar`
// outright. A pick of a variable face therefore wrote a `.folio` that saved
// cleanly and failed at RENDER — the one outcome D-8.4d.1 and D-16.1 both
// promise cannot happen, and reachable today with none of Epic 16 built.
//
// ONE BYTE SLICE, BOTH DOORS, IN ONE TEST. Two tests over two fixtures in two
// packages would re-create exactly the disjointness this guard exists to
// close: they could both pass while the two sites had drifted to disagreeing
// about which bytes are variable. Feeding `face` to both is the whole point.
//
// RED-PROVED BY DELETION, not by falsifying a condition (recorded 2026-09-03,
// wd folio-go, `go test -run 'VariableFace|Fvar|EmbedFontFamily' ./...`):
// deleting the fontset.RefuseVariableFace call in embedFontFamily reds the
// command half; deleting the variableFaceError call in fontset.New reds the
// ingestion half. Inverting a predicate would only have proved arm order.
func TestVariableFaceIsRefusedAtTheCommandAndAtIngestionOverTheSameBytes(t *testing.T) {
	// THE SAME SLICE reaches both doors. It is not re-read, re-decoded or
	// copied between them.
	face := testNotoSansThaiVariableFontBytes
	const chain = "Noto Sans Thai VF"

	// Door one: the command. fontChainRefusal also asserts the document is
	// byte-identical afterwards.
	tpl := fontChainTemplate(t)
	assetsBefore := len(tpl.doc.Assets)
	failure := fontChainRefusal(t, tpl, embedCommand(t, chain, face, `["Noto Sans"]`))

	// Door two: the renderer, over those same bytes.
	_, ingestErr := fontset.New(chain, face)
	if ingestErr == nil {
		t.Fatal("fontset.New accepted a variable face; the renderer's own guard has been moved or removed, and a hand-written .folio is now unguarded")
	}

	// ONE PREDICATE, NOT TWO AUTHORITIES. componentFailure carries the
	// helper's sentence through unaltered, so the command's message is the
	// renderer's message. A second implementation that merely agreed today
	// would not survive this equality.
	if failure.Message != ingestErr.Error() {
		t.Errorf("the two refusals are not the same sentence, so they are not the same code:\n command: %s\n  render: %s", failure.Message, ingestErr.Error())
	}
	// And it is the message that carries the author's next action (D-2.2.4:
	// "a caller hitting this needs an action, not a refusal").
	for _, want := range []string{"`fvar`", "fonttools varLib.instancer"} {
		if !strings.Contains(failure.Message, want) {
			t.Errorf("the command's refusal does not mention %s: %s", want, failure.Message)
		}
	}
	if failure.DataPath != fontChainPath(chain) {
		t.Errorf("the refusal is not located at the chain: dataPath = %q, want %q", failure.DataPath, fontChainPath(chain))
	}

	// "RETURNED AN ERROR" IS A WEAKER CLAIM THAN "WROTE NOTHING". The asset is
	// content-addressed, so the key the pick would have used is derivable
	// without the pick having happened.
	if _, exists := tpl.doc.Assets[embeddedKeyOf(face)]; exists {
		t.Error("the refused variable face was written to t.doc.Assets anyway")
	}
	if len(tpl.doc.Assets) != assetsBefore {
		t.Errorf("the asset map moved from %d to %d entries on a refused pick", assetsBefore, len(tpl.doc.Assets))
	}
	if _, exists := tpl.doc.Fonts[chain]; exists {
		t.Error("the refused variable face declared its chain anyway")
	}
}

// TestStaticFaceIsStillEmbeddedAtBothDoors is the over-broadness control. A
// guard that refused every face would pass every assertion above, so the same
// two doors are asked about a face that carries no `fvar` at all and both must
// still admit it. Nothing about today's behaviour for a static face changes.
func TestStaticFaceIsStillEmbeddedAtBothDoors(t *testing.T) {
	face := testRobotoFontBytes
	const chain = "Roboto"

	tpl := fontChainTemplate(t)
	// DECLARED Apache-2.0, and that is Story 16.1b's doing rather than a
	// detail: Roboto's own name table reads "Licensed under the Apache
	// License, Version 2.0", so the OFL-1.1 this pick used to declare is now
	// a CONTRADICTION and would be refused by the licence door two lines
	// below the `fvar` one. Declaring the licence the bytes actually name
	// keeps this test measuring what it is named for — that a STATIC face
	// still gets through — rather than a refusal from a different guard.
	fontChainAccepted(t, tpl, embedCommandDeclaring(t, chain, face, `["Noto Sans"]`, "Apache-2.0"))
	if _, exists := tpl.doc.Assets[embeddedKeyOf(face)]; !exists {
		t.Fatal("a static face was not stored under its content hash")
	}
	if _, exists := tpl.doc.Fonts[chain]; !exists {
		t.Fatal("a static face did not declare its chain")
	}
	if _, err := fontset.New(chain, face); err != nil {
		t.Fatalf("fontset.New refused a static face: %v", err)
	}
}

// ---------------------------------------------------------------------------
// STORY 16.1b: THE LICENCE TIE IS REACHABLE FROM THE COMMAND.
//
// The three outcomes are asserted against the door itself in
// internal/fontset/licencesignature_test.go — over a variable face, among
// others, which routed through here would be masked by the `fvar` refusal
// firing first. What THIS test proves is the one thing that file cannot: that
// embedFontFamily actually calls it, and that a refusal at this site writes
// NOTHING.
//
// The subject is a STATIC face for exactly that reason (D-16.R.9's call-site
// note). Roboto is recorded Apache-2.0 and its own record 13 says so, so
// declaring it OFL-1.1 is a true contradiction over real committed bytes and
// needs no mislabelled binary in the tree.
//
// RED-PROVED BY DELETING THE GUARD, not by falsifying a condition (recorded
// 2026-09-03, wd folio-go, `go test -run 'ContradictedLicence|StaticFace' ./...`):
// removing the fontset.RefuseContradictedLicence call from embedFontFamily
// reds this test. A falsified condition would only have proved arm order.
func TestEmbedFontFamilyRefusesAFaceWhoseOwnBytesContradictTheDeclaredLicence(t *testing.T) {
	face := testRobotoFontBytes
	const chain = "Roboto"

	tpl := fontChainTemplate(t)
	assetsBefore := len(tpl.doc.Assets)
	failure := fontChainRefusal(t, tpl, embedCommandDeclaring(t, chain, face, `["Noto Sans"]`, "OFL-1.1"))

	// BOTH SIDES NAMED, or the author cannot tell whether the catalogue row
	// is wrong or the binary is the wrong binary.
	for _, want := range []string{chain, "OFL-1.1", "Apache License"} {
		if !strings.Contains(failure.Message, want) {
			t.Errorf("the command's refusal does not name %s: %s", want, failure.Message)
		}
	}
	if failure.DataPath != fontChainPath(chain) {
		t.Errorf("the refusal is not located at the chain: dataPath = %q, want %q", failure.DataPath, fontChainPath(chain))
	}

	// "RETURNED AN ERROR" IS A WEAKER CLAIM THAN "WROTE NOTHING", and the
	// whole point of siting this before t.doc.Assets is the second one.
	if _, exists := tpl.doc.Assets[embeddedKeyOf(face)]; exists {
		t.Error("the refused face was written to t.doc.Assets anyway")
	}
	if len(tpl.doc.Assets) != assetsBefore {
		t.Errorf("the asset map moved from %d to %d entries on a refused pick", assetsBefore, len(tpl.doc.Assets))
	}
	if _, exists := tpl.doc.Fonts[chain]; exists {
		t.Error("the refused face declared its chain anyway")
	}
}

// TestEmbedFontFamilyStillRefusesAFaceOverTheSupportedSizeWithALocatedMessage
// keeps a REFUSAL A REFUSAL. notosanssc (10,595,932 bytes) is the one face the
// D-16.6 probe rejected, correctly, at this bound — it is not in the pickable
// catalogue, so the browser run cannot reach it, and nothing in Story 16.0 may
// sweep it into the boundary reporting it changed.
//
// The bound is DERIVED at component_commands.go's maxComponentAssetBytes
// ((engineProtocolMaxPayloadBytes - maxComponentAssetPayloadOverheadBytes) *
// 3 / 4) and the literal 6288384 appears nowhere in code. This test reads the
// derivation rather than restating its arithmetic.
func TestEmbedFontFamilyStillRefusesAFaceOverTheSupportedSizeWithALocatedMessage(t *testing.T) {
	const chain = "Oversize"
	oversize := make([]byte, maxComponentAssetBytes+1)
	copy(oversize, testRobotoFontBytes)

	tpl := fontChainTemplate(t)
	failure := fontChainRefusal(t, tpl, embedCommand(t, chain, oversize, `["Noto Sans"]`))
	want := fmt.Sprintf("face exceeds the %d-byte supported size", maxComponentAssetBytes)
	if failure.Message != want {
		t.Errorf("the size refusal reads %q, want %q", failure.Message, want)
	}
	if failure.DataPath != fontChainPath(chain) {
		t.Errorf("the size refusal is not located at the chain: dataPath = %q", failure.DataPath)
	}
}

// elementValue reads one element's `value` straight out of the canonical bytes,
// so "the victim is unchanged" is asserted against the DOCUMENT rather than
// against a projection the same defect could have shaped.
func elementValue(t *testing.T, tpl *Template, band, id string) string {
	t.Helper()
	canonical, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Bands map[string]struct {
			Elements []struct {
				ID    string `json:"id"`
				Value string `json:"value"`
			} `json:"elements"`
		} `json:"bands"`
	}
	if err := json.Unmarshal(canonical, &document); err != nil {
		t.Fatal(err)
	}
	for _, element := range document.Bands[band].Elements {
		if element.ID == id {
			return element.Value
		}
	}
	t.Fatalf("the fixture no longer carries %s in %s, so this test asserts nothing; re-derive the fixture rather than deleting the check", id, band)
	return ""
}

// TestApplyComponentCommandRefusesDuplicateKeysAtEveryLevel hands the ENGINE the
// bytes. That is the point of it: an encoder test would go green the moment a
// future encoder regresses, because it tests the fix rather than the property.
//
// Every payload below was ACCEPTED at the baseline this replaces, and the first
// one is the executed proof that gives the story its name — it NAMES e1 in the
// page header and MUTATES e5 in the page footer, returning a nil error.
func TestApplyComponentCommandRefusesDuplicateKeysAtEveryLevel(t *testing.T) {
	for _, probe := range []struct {
		name    string
		command string
		level   string
	}{
		{
			name:    "top level ids and changes",
			command: `{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"value":{"op":"set","value":"FIRST"}},"ids":["e5"],"changes":{"value":{"op":"set","value":"SECOND"}}}`,
			level:   "$",
		},
		{
			name:    "the changes object",
			command: `{"kind":"updateComponentProperties","version":1,"ids":["e5"],"changes":{"value":{"op":"set","value":"FIRST"},"value":{"op":"set","value":"SECOND"}}}`,
			level:   "$.changes",
		},
		{
			name:    "the operation object",
			command: `{"kind":"updateComponentProperties","version":1,"ids":["e5"],"changes":{"value":{"op":"set","value":"FIRST","value":"SECOND"}}}`,
			level:   "$.changes.value",
		},
		{
			name:    "a repeated op inside the operation object",
			command: `{"kind":"updateComponentProperties","version":1,"ids":["e5"],"changes":{"value":{"op":"clear","op":"set","value":"SECOND"}}}`,
			level:   "$.changes.value",
		},
		{
			// The version gate reads the LAST key, so appending a second one
			// admitted a version:0 command. Asserted directly rather than left
			// to be repaired by accident at the top level: a fix nothing
			// asserts can regress in silence.
			name:    "a repeated version",
			command: `{"kind":"updateComponentProperties","version":0,"ids":["e5"],"changes":{"value":{"op":"set","value":"SECOND"}},"version":1}`,
			level:   "$",
		},
		{
			// Arity is a COINCIDENCE, not a check. deleteComponent and
			// deleteFontChain both carry three top-level keys, so
			// componentFields passed and dispatch landed in the wrong handler;
			// only that handler's own field names stopped it, which stops
			// nothing in general.
			name:    "a same-arity kind escalation",
			command: `{"kind":"deleteComponent","version":1,"id":"e5","kind":"deleteFontChain"}`,
			level:   "$",
		},
		{
			// Arrays are not a special case anywhere else and must not be one
			// here. The scan reaches an object nested inside one.
			name:    "an object inside an array",
			command: `{"kind":"bindComponentScalar","version":1,"id":"e5","segments":["a"],"probe":[{"k":1,"k":2}]}`,
			level:   "$.probe[0]",
		},
	} {
		t.Run(probe.name, func(t *testing.T) {
			tpl := componentTemplate(t)
			before, err := SerializeTemplate(tpl)
			if err != nil {
				t.Fatal(err)
			}
			victim := elementValue(t, tpl, "pageFooter", "e5")
			_, err = applyComponentCommand(tpl, []byte(probe.command))
			if err == nil {
				t.Fatal("duplicate-key bytes were accepted; a command that names one thing changed another")
			}
			var failure *designer.ComponentCommandError
			if !errors.As(err, &failure) {
				// A bare fmt.Errorf surfaces at the host as ENGINE_REJECTED
				// with no location at all, which is what every other refusal on
				// this decode path does and what this one must not.
				t.Fatalf("refusal is not a ComponentCommandError, so it reaches the host unlocated: %v", err)
			}
			if failure.ElementID != "" {
				// A duplicate key means the named id is exactly the thing that
				// cannot be trusted: the executed baseline named e1 and changed
				// e5, so naming any id names the wrong one.
				t.Fatalf("refusal named the element %q, which the duplicate has made untrustworthy", failure.ElementID)
			}
			if failure.DataPath != componentCommandPath {
				t.Fatalf("DataPath = %q, want the document-scoped %q", failure.DataPath, componentCommandPath)
			}
			if !strings.Contains(failure.Message, probe.level) {
				t.Fatalf("message %q does not name the level %s the duplicate is at", failure.Message, probe.level)
			}
			if got := elementValue(t, tpl, "pageFooter", "e5"); got != victim {
				t.Fatalf("the element a last-wins resolution would have mutated changed from %q to %q", victim, got)
			}
			after, err := SerializeTemplate(tpl)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("a refused command changed the canonical bytes")
			}
		})
	}
}

// TestApplyComponentCommandStillAcceptsUnambiguousBytes is the other direction,
// and it is what stops the guard above from being satisfied by refusing
// everything. A duplicate key inside a STRING VALUE is not a duplicate key.
func TestApplyComponentCommandStillAcceptsUnambiguousBytes(t *testing.T) {
	tpl := componentTemplate(t)
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"updateComponentProperties","version":1,"ids":["e5"],"changes":{"value":{"op":"set","value":"a\",\"value\":\"b"}}}`)); err != nil {
		t.Fatalf("a value whose TEXT looks like a repeated key was refused: %v", err)
	}
	if got, want := elementValue(t, tpl, "pageFooter", "e5"), `a","value":"b`; got != want {
		t.Fatalf("value = %q, want %q", got, want)
	}
	// Repeated keys in SIBLING objects are not duplicates either: `op` appears
	// once per operation object, and every multi-field command relies on it.
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"updateComponentProperties","version":1,"ids":["e5"],"changes":{"value":{"op":"set","value":"x"},"align":{"op":"set","value":"center"}}}`)); err != nil {
		t.Fatalf("sibling objects sharing a key name were refused: %v", err)
	}
}

// TestDuplicateKeyRefusalCarriesNoElementID asserts the refusal's own shape at
// the module boundary: a duplicate key makes the named id untrustworthy, so no
// id is named, and the message fits inside the width the host will cut it to.
// Mapping that onto a host RESPONSE is a different question, asserted by
// TestDuplicateKeyRefusalArrivesAtTheHostAsComponentInvalid below.
func TestDuplicateKeyRefusalCarriesNoElementID(t *testing.T) {
	tpl := componentTemplate(t)
	_, err := applyComponentCommand(tpl, []byte(`{"kind":"deleteComponent","version":1,"id":"e1","id":"e5"}`))
	var failure *designer.ComponentCommandError
	if !errors.As(err, &failure) {
		t.Fatalf("want a ComponentCommandError, got %v", err)
	}
	if failure.ElementID != "" || failure.DataPath != "command" {
		t.Fatalf("ElementID = %q, DataPath = %q; want an empty id and a document-scoped path", failure.ElementID, failure.DataPath)
	}
	if len(failure.Message) > maxComponentFailureMessageBytes {
		t.Fatalf("message is %d bytes, which the host would cut at %d", len(failure.Message), maxComponentFailureMessageBytes)
	}
}

// componentInvalidHostBranch reads the wasm host's OWN mapping of a
// *ComponentCommandError onto a response. Anchored on the errors.As that opens
// the branch, because `return response{` is the whole file's idiom and an
// unanchored match would read some other branch and compare it to this record —
// a green test asserting the wrong thing.
var componentInvalidHostBranch = regexp.MustCompile(`(?s)var componentErr \*designer\.ComponentCommandError\s*\n\s*if errors\.As\(err, &componentErr\) \{(.*?)\n\t\}`)

// pageSetupHostFallback reads the OTHER door's mapping: the host's prefix test
// and the code it answers with. Anchored on the prefix literal the page-setup
// door deliberately emits, so this cannot match some unrelated response.
var pageSetupHostFallback = regexp.MustCompile(`(?s)strings\.HasPrefix\(message, "folio8: page\."\).*?DiagnosticCode: "PAGE_SETUP_INVALID"`)

// TestDuplicateKeyRefusalArrivesAtTheHostAsComponentInvalid is the acceptance
// criterion's own wording — "when it surfaces at the wasm host, then it arrives
// as COMPONENT_INVALID with an EMPTY ElementID" — and it needs both halves,
// because neither is sufficient on its own.
//
// The FIRST half is executed: the refusal really is a *ComponentCommandError
// carrying an empty ElementID, so whatever the host does with that field, it is
// handed nothing.
//
// The SECOND half is read from the host's source, for the same reason
// TestComponentFailureBoundsMatchTheHostsOwnLiterals reads it: wasm/cmd/engine
// is a main package built for js/wasm, its own tests carry `//go:build js &&
// wasm` and do not run in this gate, and nothing else ties this module's
// refusal to the code the browser is actually answered by. The existing
// tripwire pins Message and DataPath and deliberately NOT ElementID — this is
// the leg it leaves open.
//
// If it goes red: the host stopped mapping a component refusal onto
// COMPONENT_INVALID, or stopped passing the module's own ElementID through, and
// the panel is now being told an id the duplicate has made untrustworthy.
func TestDuplicateKeyRefusalArrivesAtTheHostAsComponentInvalid(t *testing.T) {
	tpl := componentTemplate(t)
	_, err := applyComponentCommand(tpl, []byte(`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"value":{"op":"set","value":"FIRST"}},"ids":["e5"],"changes":{"value":{"op":"set","value":"SECOND"}}}`))
	var failure *designer.ComponentCommandError
	if !errors.As(err, &failure) {
		t.Fatalf("want a ComponentCommandError, got %v", err)
	}
	if failure.ElementID != "" {
		t.Fatalf("ElementID = %q; the host would report an id the duplicate has made untrustworthy", failure.ElementID)
	}

	path := filepath.Join(repoRootFromTest(t), "folio-go", "wasm", "cmd", "engine", "main.go")
	source, err := os.ReadFile(path)
	if err != nil {
		// Not a skip. The host is the other side of this seam and a missing one
		// is a finding, not an excuse.
		t.Fatalf("read the wasm host: %v", err)
	}
	branch := componentInvalidHostBranch.FindStringSubmatch(string(source))
	if branch == nil {
		t.Fatal("wasm/cmd/engine/main.go no longer maps a *designer.ComponentCommandError where this test can read it; if the host was restructured, re-derive this extraction rather than deleting the check")
	}
	// Whitespace-insensitive: gofmt aligns these struct fields, so pinning the
	// exact column spacing would redden this test the day an unrelated field
	// with a longer name joins the literal. What is being asserted is the
	// mapping, not the alignment.
	for _, want := range []*regexp.Regexp{
		regexp.MustCompile(`DiagnosticCode:\s*"COMPONENT_INVALID"`),
		regexp.MustCompile(`ElementID:\s*bounded\(componentErr\.ElementID, 128\)`),
	} {
		if !want.MatchString(branch[1]) {
			t.Errorf("the host's component-refusal branch no longer matches %s:\n%s", want, branch[1])
		}
	}

	// THE SECOND DOOR'S CODE, read from the same file. A page-setup refusal is
	// raised as a plain error opening with the `folio8: page.` prefix precisely
	// so it lands in this fallback instead of the component branch above, and
	// the fallback is where PAGE_SETUP_INVALID is decided.
	if !pageSetupHostFallback.MatchString(string(source)) {
		t.Error("wasm/cmd/engine/main.go no longer answers PAGE_SETUP_INVALID for a message opening with the `folio8: page.` prefix; if the host was restructured, re-derive this extraction rather than deleting the check — a page-setup refusal now reports some other code")
	}
	if !strings.HasPrefix(pageSetupFailurePrefix, "folio8: page.") {
		t.Errorf("the page-setup door's own prefix is %q, which the host's fallback above does not test for", pageSetupFailurePrefix)
	}
	// ORDER IS PART OF THE MAPPING. The *RenderError branch answers with a
	// registered diagnostic code, and a ComponentCommandError is not a
	// *RenderError — but if the two were ever reordered around a shared
	// interface, this refusal would surface as something else entirely.
	componentAt := strings.Index(string(source), "var componentErr *designer.ComponentCommandError")
	renderAt := strings.Index(string(source), "var renderErr *folio8.RenderError")
	if componentAt < 0 || renderAt < 0 {
		t.Fatal("wasm/cmd/engine/main.go no longer declares both error branches where this test can find them; re-derive this extraction rather than deleting the check")
	}
	if componentAt > renderAt {
		t.Error("the host now matches *RenderError before *ComponentCommandError, so a component refusal no longer arrives as COMPONENT_INVALID")
	}
}

// TestDuplicateKeyScanStaysSynchronisedPastItsDepthBound covers the scanner's
// own failure mode rather than the defect it hunts.
//
// The walk is recursive and Decoder.Token() streams to ANY depth — measured,
// 20 000 nested arrays tokenise cleanly — so the walk is bounded. Returning at
// that bound WITHOUT consuming the value it declined to enter leaves the
// decoder positioned INSIDE that value, and the parent object's loop then reads
// the value's own nested tokens as if they were the parent's keys. The result
// is a duplicate reported at a path that does not exist, or a real duplicate
// missed, or a key token that is not a string aborting the scan silently.
//
// The bound is encoding/json's own nesting limit, so the payload below is one
// level past what the caller's ordinary decode will accept: the scan must hand
// it on intact rather than mangling the walk.
func TestDuplicateKeyScanStaysSynchronisedPastItsDepthBound(t *testing.T) {
	deep := strings.Repeat("[", maxCommandKeyScanDepth+1) + strings.Repeat("]", maxCommandKeyScanDepth+1)

	// A REAL duplicate at the top level, sitting AFTER a value too deep to walk.
	// A desynchronised decoder never reaches it.
	if at, key, found := duplicateKeyPathForTest(t, `{"a":`+deep+`,"a":1}`); !found || at != "$" || key != "a" {
		t.Fatalf("the duplicate after an over-deep value was reported as (%q, %q, %v); the scan lost its place inside that value", at, key, found)
	}

	// And NO duplicate where there is none. With the decoder left inside the
	// deep value, the parent loop reads `[`-delimiters as keys and the scan
	// reports whatever it stumbles into.
	if at, key, found := duplicateKeyPathForTest(t, `{"a":`+deep+`,"b":1}`); found {
		t.Fatalf("a duplicate was invented at (%q, %q) in bytes that declare none", at, key)
	}

	// The whole command still refuses, because the ordinary decode rejects the
	// depth — the scan hands the question on rather than answering it.
	tpl := componentTemplate(t)
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"deleteComponent","version":1,"id":`+deep+`}`)); err == nil {
		t.Fatal("bytes nested past encoding/json's own limit were accepted")
	}
}

// duplicateKeyPathForTest exercises the walk directly, because what is under
// test is the decoder's POSITION after it — a property no end-to-end refusal
// can distinguish from a lucky guess.
func duplicateKeyPathForTest(t *testing.T, payload string) (string, string, bool) {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(payload))
	dec.UseNumber()
	first, err := dec.Token()
	if err != nil {
		t.Fatalf("payload does not tokenise: %v", err)
	}
	return scanForDuplicateKey(dec, first, "$", 0)
}

// TestComponentPropertyNullReportsTheCauseAndTheField is the DW-32 door's own
// user-visible outcome, and it had no coverage: every assertion about the
// `null` an unparseable draft encodes to sat on the PAGE-SETUP door, while
// DW-32 was filed against the property encoder.
//
// These are the exact bytes the designer now produces when the author types
// something that is not a JSON number into a numeric property. The refusal must
// name the FIELD and the CAUSE — a located message the panel can put beside the
// box — not the generic parse failure the malformed bytes used to earn, and not
// the overflow the mixed branch used to report.
func TestComponentPropertyNullReportsTheCauseAndTheField(t *testing.T) {
	for _, probe := range []struct {
		field    string
		wantPath string
		wantMsg  string
	}{
		{field: "width", wantPath: "component.width", wantMsg: "must be a number"},
		{field: "x", wantPath: "component.x", wantMsg: "must be a number"},
		{field: "fontSize", wantPath: "component.fontSize", wantMsg: "must be a number"},
	} {
		t.Run(probe.field, func(t *testing.T) {
			tpl := componentTemplate(t)
			before, err := SerializeTemplate(tpl)
			if err != nil {
				t.Fatal(err)
			}
			_, err = applyComponentCommand(tpl, []byte(`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"`+probe.field+`":{"op":"set","value":null}}}`))
			var failure *designer.ComponentCommandError
			if !errors.As(err, &failure) {
				t.Fatalf("want a located ComponentCommandError, got %v", err)
			}
			if failure.DataPath != probe.wantPath {
				t.Fatalf("DataPath = %q, want %q — the panel puts the message beside the field this names", failure.DataPath, probe.wantPath)
			}
			if !strings.Contains(failure.Message, probe.wantMsg) {
				t.Fatalf("message = %q, want it to name the cause %q", failure.Message, probe.wantMsg)
			}
			// The cause the mixed branch used to report for these exact bytes.
			if strings.Contains(failure.Message, "overflows millipoints") {
				t.Fatalf("message = %q, which still blames overflow for a value that is not a number", failure.Message)
			}
			after, err := SerializeTemplate(tpl)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("a refused property command changed the canonical bytes")
			}
		})
	}

	// THE OTHER DIRECTION, on the same door: a genuine overflow still reports
	// overflow. Fixing a message must not delete the detector, and this door
	// shares parseMillipoints with page setup.
	tpl := componentTemplate(t)
	_, err := applyComponentCommand(tpl, []byte(`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"width":{"op":"set","value":99999999999999999999}}}`))
	var overflow *designer.ComponentCommandError
	if !errors.As(err, &overflow) {
		t.Fatalf("want a located ComponentCommandError, got %v", err)
	}
	if !strings.Contains(overflow.Message, "overflows millipoints") {
		t.Fatalf("message = %q, want it to still report an overflow", overflow.Message)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// STORY 11.4 — ROUTE C: `entries` AND `tail` CARRY THE FORMAT'S OWN CHAIN ENTRY
// ─────────────────────────────────────────────────────────────────────────────

// TestRouteCWidenedAFieldsShapeAndNoDoorsVERDICT is Story 15.2a's inherited
// obligation, made testable rather than asserted in prose.
//
// 15.2a's norm — "the accept-set of both doors is narrowed or unchanged at
// every input; no draft may be accepted that was refused before" — binds the
// JUDGMENT OF EXISTING INPUT SHAPES, not the set of expressible commands. Read
// as a bar on the product ever gaining a capability it would be a permanent
// freeze on the command surface, which its own "Valid command, unchanged"
// matrix row contradicts. So the obligation this story inherits is: for every
// command shape that exists today, the accept/refuse VERDICT is unchanged at
// every input.
//
// ⚠ IT IS DELIBERATELY NOT A TEST THAT THE NEW SHAPE IS ACCEPTED. A case
// showing `{"face": …, "bold": …}` going through proves nothing about whether
// widening a field's SHAPE softened a VERDICT. Every row below is an input that
// was REFUSED before route C and must still be refused after it — an arity
// violation, which is a judgment about a field set that already existed, and
// each malformed-entry-list row from the test above.
func TestRouteCWidenedAFieldsShapeAndNoDoorsVERDICT(t *testing.T) {
	long := strings.Repeat("f", maxCanvasPropertyString+1)
	// ARITY, ON AN EXISTING FIELD SET AND IN BOTH DIRECTIONS. componentFields
	// counts an exact arity and route C moved no constant: addFontChain is
	// still 4 and embedFontFamily still 12. These refusals are UNLOCATED — the
	// count is checked before any field is read, so there is no chain name to
	// name — which is why they are asserted here rather than through
	// fontChainRefusal.
	for _, command := range []string{
		`{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans"],"extra":1}`,
		`{"kind":"addFontChain","version":1,"name":"caption"}`,
		`{"kind":"embedFontFamily","version":1,"name":"c","family":"F","style":"Regular","licence":"OFL-1.1","licenceText":"t","copyright":"c","mediaType":"font/ttf","data":"AA==","tail":[]}`,
	} {
		tpl := fontChainTemplate(t)
		before, err := SerializeTemplate(tpl)
		if err != nil {
			t.Fatal(err)
		}
		if _, applyErr := applyComponentCommand(tpl, []byte(command)); applyErr == nil {
			t.Errorf("an arity violation was accepted after route C: %s", command)
		}
		after, err := SerializeTemplate(tpl)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Errorf("a refused command mutated the document: %s", command)
		}
	}
	for _, command := range []string{
		// THE MALFORMED ENTRY LISTS, unchanged inputs to the letter.
		`{"kind":"addFontChain","version":1,"name":"caption","faces":["Noto Sans"]}`,
		`{"kind":"addFontChain","version":1,"name":"caption","entries":"Noto Sans"}`,
		`{"kind":"addFontChain","version":1,"name":"caption","entries":[7]}`,
		`{"kind":"addFontChain","version":1,"name":"caption","entries":[]}`,
		`{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans",""]}`,
		`{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans","` + long + `"]}`,
		// AND THE NAME RULES THE ENTRY SHAPE NEVER TOUCHED.
		`{"kind":"addFontChain","version":1,"name":"body","entries":["Noto Sans"]}`,
		`{"kind":"addFontChain","version":1,"name":"","entries":["Noto Sans"]}`,
	} {
		fontChainRefusal(t, fontChainTemplate(t), command)
	}
	// POSITIVE CONTROL: the door is not simply refusing everything. The one
	// command shape that was accepted before route C is accepted after it, and
	// it is the SAME bytes — a chain of bare face-name strings.
	fontChainAccepted(t, fontChainTemplate(t), `{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans","Noto Sans Thai"]}`)
}

// TestAddFontChainDeclaresTheCutsAPickNAMES is route C's capability half: a
// chain entry a command writes may declare the cuts its family has.
//
// It reads the entry back off the MODEL through FontChainEntry.Variant, which
// is the closed set's own authority (model.go's fontChainVariants), so the tie
// between the wire's key spellings and the fields they land in is asserted
// rather than assumed to hold because both lists were typed the same day.
func TestAddFontChainDeclaresTheCutsAPickNAMES(t *testing.T) {
	tpl := fontChainTemplate(t)
	fontChainAccepted(t, tpl, `{"kind":"addFontChain","version":1,"name":"caption","entries":[{"face":"Roboto","bold":"Roboto Bold","italic":"Roboto Italic","boldItalic":"Roboto Bold Italic"},{"face":"Noto Sans Thai","bold":"Noto Sans Thai Bold"},"Noto Sans SC"]}`)
	chain, ok := tpl.doc.Fonts["caption"]
	if !ok || len(chain) != 3 {
		t.Fatalf("the pick declared %#v, want three entries", chain)
	}
	for _, want := range []struct {
		at    int
		face  string
		style template.FontStyle
		cut   string
	}{
		{0, "Roboto", template.FontStyleBold, "Roboto Bold"},
		{0, "Roboto", template.FontStyleItalic, "Roboto Italic"},
		{0, "Roboto", template.FontStyleBoldItalic, "Roboto Bold Italic"},
		{1, "Noto Sans Thai", template.FontStyleBold, "Noto Sans Thai Bold"},
		{1, "Noto Sans Thai", template.FontStyleItalic, ""},
		// D-A, AND IT IS ORDINARY BEHAVIOUR: Noto Sans SC has no cut at any
		// weight, so the entry declares none and stays a bare face name.
		{2, "Noto Sans SC", template.FontStyleBold, ""},
		{2, "Noto Sans SC", template.FontStyleBoldItalic, ""},
	} {
		entry := chain[want.at]
		if entry.Face != want.face {
			t.Fatalf("entry %d names %q, want %q", want.at, entry.Face, want.face)
		}
		if entry.Embedded() {
			t.Fatalf("entry %d is an EMBEDDED entry; a command writes FACE names and never an asset sibling", want.at)
		}
		if got := entry.Variant(want.style); got != want.cut {
			t.Errorf("entry %d (%q) declares %v = %q, want %q", want.at, want.face, want.style, got, want.cut)
		}
	}
	// AND THE DOCUMENT CARRIES NO ASSET. Naming a shipped family is not
	// embedding it: that is the whole of what Story 11.4 changed about a pick.
	if len(tpl.doc.Assets) != 0 {
		t.Errorf("naming a shipped family wrote %d assets; a pick that names must embed nothing", len(tpl.doc.Assets))
	}
	// NON-VACUITY of the "" rows above: at least one cut really was declared,
	// so a decoder that dropped every variant could not pass this test.
	if chain[0].Bold == "" {
		t.Fatal("no cut reached the model at all, so every empty-string row above was vacuous")
	}
}

// TestACommandMayNotWriteAnAssetSiblingOrAnUnknownKey is the guarantee the
// `assetKeyReferenced` out-of-scope ruling rests on, asserted rather than
// remembered.
//
// Before route C the guarantee was the wire type: `[]string` had nowhere to put
// an asset. Now it is this refusal, so it is the thing that must be tested — an
// invariant whose mechanism changed and whose test did not is an invariant
// nobody is checking.
func TestACommandMayNotWriteAnAssetSiblingOrAnUnknownKey(t *testing.T) {
	for _, probe := range []struct {
		command, mustSay string
		// A refusal about the ENTRY tells the author what an entry may be. A
		// refusal about one variant's VALUE does not, because the shape was
		// never the thing that was wrong.
		statesShape bool
	}{
		// THE ASSET ARM, ON BOTH DOORS AND IN BOTH POSITIONS.
		{`{"kind":"addFontChain","version":1,"name":"caption","entries":[{"asset":"9f86d0"}]}`, "never an assets key", true},
		{`{"kind":"addFontChain","version":1,"name":"caption","entries":[{"face":"Roboto","asset":"9f86d0"}]}`, "never an assets key", true},
		// A FOURTH VARIANT KEY. The set is CLOSED, and the closure is what keeps
		// an unknown key inside an entry a refusal rather than a decoration.
		{`{"kind":"addFontChain","version":1,"name":"caption","entries":[{"face":"Roboto","weight":700}]}`, "CLOSED", true},
		{`{"kind":"addFontChain","version":1,"name":"caption","entries":[{"face":"Roboto","oblique":"Roboto Italic"}]}`, "CLOSED", true},
		// A NON-STRING VARIANT — `bold` is a boolean on `style` and a FACE here.
		// It is NOT the closed-set refusal: the key is a member, the value is
		// the wrong kind of thing, and telling this author the set is closed
		// would send them to fix the one thing that was not wrong.
		{`{"kind":"addFontChain","version":1,"name":"caption","entries":[{"face":"Roboto","bold":true}]}`, `"bold" must be a string naming a face`, true},
		// AN OBJECT WITH NO DISCRIMINANT AT ALL.
		{`{"kind":"addFontChain","version":1,"name":"caption","entries":[{"bold":"Roboto Bold"}]}`, "must name the face it is", true},
		// AN EXPLICIT EMPTY CUT. Absence is an absent KEY: an empty string names
		// no face and the loader refuses it, so a command that let it through
		// would author a document the product cannot reopen.
		{`{"kind":"addFontChain","version":1,"name":"caption","entries":[{"face":"Roboto","bold":""}]}`, "write no key at all", false},
		// AND THE TAIL TAKES EVERY ONE OF THE SAME RULES, because it is the same
		// decoder. `data` is one byte of nothing, which never gets read: the
		// tail is refused before the face is decoded.
		{`{"kind":"embedFontFamily","version":1,"name":"c","family":"F","style":"Regular","licence":"OFL-1.1","licenceText":"t","copyright":"c","source":"s","mediaType":"font/ttf","data":"AA==","tail":[{"asset":"9f86d0"}]}`, "never an assets key", true},
	} {
		failure := fontChainRefusal(t, fontChainTemplate(t), probe.command)
		if !strings.Contains(failure.Message, probe.mustSay) {
			t.Errorf("refusal = %q, want it to mention %q\n  for %s", failure.Message, probe.mustSay, probe.command)
		}
		// Every ENTRY-shaped refusal still tells the author what they MAY write.
		if probe.statesShape && !strings.Contains(failure.Message, fontChainEntryShape) {
			t.Errorf("refusal = %q never states the legal entry shape\n  for %s", failure.Message, probe.command)
		}
	}
	// THE SHAPE SENTENCE NAMES THE CLOSED SET AND THE ONE DISCRIMINANT A
	// COMMAND MAY WRITE, and never `asset`.
	for _, key := range []string{"face", "bold", "italic", "boldItalic"} {
		if !strings.Contains(fontChainEntryShape, `"`+key+`"`) {
			t.Errorf("the legal-shape sentence never names %q", key)
		}
	}
	if strings.Contains(fontChainEntryShape, `"asset"`) {
		t.Error("the legal-shape sentence offers `asset`, which no command may write")
	}
}

// TestTheCommandDoorRefusesEveryNULLShapedHoleInAnEntryObject is the review
// finding that made route C real rather than nominal, and every row below was
// MEASURED as accepted before the decoder read a key map.
//
// Route C was chosen over a new command kind and over a widened arity for ONE
// reason above the cost ones: it makes "a variant key is a face name, never an
// assets key" a thing the DECODER enforces instead of a rule someone remembers.
// A `*string` struct field with DisallowUnknownFields could not carry that
// weight. A JSON null decodes to a nil pointer, so `{"asset":null}` named the
// key and read as absent, and the guarantee had a null-shaped hole straight
// through it. Two more holes came with it: a null CUT was dropped silently
// while the loader refuses it with a located load error, and encoding/json
// matches field names case-INSENSITIVELY, so `{"FACE":…}` and `{"BOLDITALIC":…}`
// were accepted where the loader refuses them.
//
// Each arm is its own row, and each is mutation-proved by DELETING its own
// guard rather than by deleting the decoder: one fix covering four findings is
// a claim, not a demonstration.
func TestTheCommandDoorRefusesEveryNULLShapedHoleInAnEntryObject(t *testing.T) {
	for _, probe := range []struct{ name, command, mustSay string }{
		{
			"a null asset key still names the asset arm",
			`{"kind":"addFontChain","version":1,"name":"caption","entries":[{"face":"Roboto","asset":null}]}`,
			"never an assets key",
		},
		{
			"a null asset key with no face beside it",
			`{"kind":"addFontChain","version":1,"name":"caption","entries":[{"asset":null}]}`,
			"never an assets key",
		},
		{
			"a null cut is a value, not an absence",
			`{"kind":"addFontChain","version":1,"name":"caption","entries":[{"face":"Roboto","bold":null}]}`,
			`"bold" is present and null`,
		},
		{
			"a null face names no face",
			`{"kind":"addFontChain","version":1,"name":"caption","entries":[{"face":null}]}`,
			`"face" is present and null`,
		},
		{
			"the discriminant is CASE-SENSITIVE",
			`{"kind":"addFontChain","version":1,"name":"caption","entries":[{"FACE":"Roboto"}]}`,
			"CASE-SENSITIVE",
		},
		{
			"and so is every cut key",
			`{"kind":"addFontChain","version":1,"name":"caption","entries":[{"face":"Roboto","BOLDITALIC":"Roboto Bold Italic"}]}`,
			"CASE-SENSITIVE",
		},
		{
			"and the tail takes the same rules, because it is the same decoder",
			`{"kind":"embedFontFamily","version":1,"name":"c","family":"F","style":"Regular","licence":"OFL-1.1","licenceText":"t","copyright":"c","source":"s","mediaType":"font/ttf","data":"AA==","tail":[{"face":"Noto Sans Thai","bold":null}]}`,
			`"bold" is present and null`,
		},
	} {
		t.Run(probe.name, func(t *testing.T) {
			failure := fontChainRefusal(t, fontChainTemplate(t), probe.command)
			if !strings.Contains(failure.Message, probe.mustSay) {
				t.Errorf("refusal = %q, want it to say %q", failure.Message, probe.mustSay)
			}
		})
	}
	// THE THREE REFUSALS ARE THREE DIFFERENT SENTENCES. An unrecognised key, a
	// value of the wrong type and a null are three different author mistakes,
	// and a decoder that collapses them into one sentence has told the author
	// only that something is wrong.
	distinct := map[string]string{}
	for _, command := range []string{
		`{"kind":"addFontChain","version":1,"name":"caption","entries":[{"face":"Roboto","weight":700}]}`,
		`{"kind":"addFontChain","version":1,"name":"caption","entries":[{"face":"Roboto","bold":true}]}`,
		`{"kind":"addFontChain","version":1,"name":"caption","entries":[{"face":"Roboto","bold":null}]}`,
		`{"kind":"addFontChain","version":1,"name":"caption","entries":[{"face":"Roboto","bold":""}]}`,
	} {
		message := fontChainRefusal(t, fontChainTemplate(t), command).Message
		if seen, ok := distinct[message]; ok {
			t.Errorf("two different defects share one refusal %q:\n  %s\n  %s", message, seen, command)
		}
		distinct[message] = command
	}
	// POSITIVE CONTROL, and it is the near-miss rather than an easy one: the
	// legitimate spellings of every arm above are ACCEPTED. Without it a
	// decoder that refused every object would pass every row.
	fontChainAccepted(t, fontChainTemplate(t), `{"kind":"addFontChain","version":1,"name":"caption","entries":[{"face":"Roboto","bold":"Roboto Bold","boldItalic":"Roboto Bold Italic"}]}`)
}

// TestARepeatedKeyInsideAChainEntryIsRefusedAtTheDoor is the fourth arm of the
// same finding, and it is here to record that the answer is NOT in the entry
// decoder.
//
// refuseDuplicateCommandKeys token-scans the WHOLE command at every depth —
// arrays and nested objects are not special cases anywhere else and are not
// special cases there — so a repeated key inside one entry object is refused
// before addFontChain runs, with the path of the object that carries it. A
// second duplicate check inside commandFontChainEntry would be a second answer
// to a solved question, and two answers can disagree. This test is what keeps
// the reuse honest: if the door's scan ever stopped descending into `entries`,
// nothing else would notice.
func TestARepeatedKeyInsideAChainEntryIsRefusedAtTheDoor(t *testing.T) {
	for _, probe := range []struct{ command, at string }{
		{`{"kind":"addFontChain","version":1,"name":"caption","entries":[{"face":"A","bold":"B","bold":"C"}]}`, "$.entries[0]"},
		{`{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans",{"face":"A","face":"B"}]}`, "$.entries[1]"},
		{`{"kind":"embedFontFamily","version":1,"name":"c","family":"F","style":"Regular","licence":"OFL-1.1","licenceText":"t","copyright":"c","source":"s","mediaType":"font/ttf","data":"AA==","tail":[{"face":"A","italic":"B","italic":"C"}]}`, "$.tail[0]"},
	} {
		tpl := fontChainTemplate(t)
		_, err := applyComponentCommand(tpl, []byte(probe.command))
		if err == nil {
			t.Fatalf("a repeated key inside a chain entry was accepted, last-wins and silently: %s", probe.command)
		}
		if !strings.Contains(err.Error(), "twice at "+probe.at) {
			t.Errorf("refusal = %q, want it to locate the repeat at %s", err.Error(), probe.at)
		}
	}
}

// TestASelfReferentialVariantThroughTheCOMMANDDoorIsRefused closes the gap
// between the two doors' WORDING on a defect they already agreed the verdict of.
//
// Before Story 11.4's review a command writing `{"face":"Roboto","bold":"Roboto"}`
// was refused only by applyFontChainCommand's reparse, which surfaces as the
// UNLOCATED "font chains did not pass format validation" — a sentence that
// names neither the chain, the entry, nor the key, and leaves an author who
// wrote one cut wrong staring at their whole font section. The loader is still
// the authority for the rule; the command door now states it where the author
// can act on it.
func TestASelfReferentialVariantThroughTheCOMMANDDoorIsRefused(t *testing.T) {
	for _, probe := range []struct{ name, command string }{
		{"entries", `{"kind":"addFontChain","version":1,"name":"caption","entries":[{"face":"Roboto","bold":"Roboto"}]}`},
		{"a tail entry", `{"kind":"embedFontFamily","version":1,"name":"c","family":"F","style":"Regular","licence":"OFL-1.1","licenceText":"t","copyright":"c","source":"s","mediaType":"font/ttf","data":"AA==","tail":[{"face":"Noto Sans Thai","italic":"Noto Sans Thai"}]}`},
	} {
		t.Run(probe.name, func(t *testing.T) {
			failure := fontChainRefusal(t, fontChainTemplate(t), probe.command)
			for _, phrase := range []string{"OWN base face", "only the base is privileged"} {
				if !strings.Contains(failure.Message, phrase) {
					t.Errorf("refusal = %q does not say %q", failure.Message, phrase)
				}
			}
			if failure.DataPath == "" {
				t.Error("the command door's self-reference refusal is unlocated, which is the whole defect it replaces")
			}
		})
	}
	// AND THE NARROWING IS THE ONE AXIS. A cross-variant collision names the
	// same face twice and never names the base — legal at the loader
	// (D-11.2.11) and it must stay legal here, or the command door has
	// quietly become stricter than the format.
	fontChainAccepted(t, fontChainTemplate(t), `{"kind":"addFontChain","version":1,"name":"caption","entries":[{"face":"Roboto","bold":"X","italic":"X"}]}`)
}

// TestTheCommandDoorsCutSetIsTheFormatsCutSet ties the command layer's local
// enumeration of the closed variant set to the format's authority.
//
// The authority is internal/template's fontChainVariants, declared "THE
// authority for the closed variant set" — and it is UNEXPORTED, so the command
// door (package folio8) cannot import it and holds its own projection. Two
// hand-kept lists that agree only because they were typed the same day is the
// drift this repo has been bitten by before, so this test reads the Go source
// as text, which is the established idiom for a two-language / two-package list
// tie (canvas-font-stack.test.ts parses fonts.go the same way).
func TestTheCommandDoorsCutSetIsTheFormatsCutSet(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("internal", "template", "model.go"))
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile(`var fontChainVariants = \[\]struct \{[\s\S]*?\n\}\{\n([\s\S]*?)\n\}`).FindSubmatch(source)
	if block == nil {
		t.Fatal("fontChainVariants is no longer declared the way this test reads it; re-derive the parse before trusting a green")
	}
	var authority []string
	for _, row := range regexp.MustCompile(`\{"([^"]+)",`).FindAllSubmatch(block[1], -1) {
		authority = append(authority, string(row[1]))
	}
	// NON-VACUITY: a parse that found nothing would make every comparison below
	// trivially true.
	if len(authority) == 0 {
		t.Fatal("read no variant keys out of model.go, so the tie below is vacuous")
	}
	if got := commandFontChainCutKeys(); !slices.Equal(got, authority) {
		t.Errorf("the command door writes %v; the format's authority is %v", got, authority)
	}
	// AND THE FIELDS, not only the names: a key that lands in the wrong field
	// would satisfy the list tie and still write bold into italic. Each key is
	// driven through the real door and read back through the model's own
	// Variant accessor, which is derived from the same authority.
	for _, tc := range []struct {
		key   string
		style template.FontStyle
	}{
		{"bold", template.FontStyleBold},
		{"italic", template.FontStyleItalic},
		{"boldItalic", template.FontStyleBoldItalic},
	} {
		if !slices.Contains(authority, tc.key) {
			t.Fatalf("%q is not in the format's authority, so this row asserts nothing", tc.key)
		}
		tpl := fontChainTemplate(t)
		fontChainAccepted(t, tpl, `{"kind":"addFontChain","version":1,"name":"caption","entries":[{"face":"Roboto","`+tc.key+`":"A Cut"}]}`)
		if got := tpl.doc.Fonts["caption"][0].Variant(tc.style); got != "A Cut" {
			t.Errorf("%q landed in the field for %v as %q, want it to carry the cut", tc.key, tc.style, got)
		}
	}
}

// TestAMalformedTailIsReportedAsTheTailAndNotAsEntries is the whole reason
// commandFontChainEntries takes a `subject` argument, and nothing asserted it.
//
// One decoder serves two fields — addFontChain's `entries` and
// embedFontFamily's `tail` — which is what keeps the two doors from drifting in
// what a pick may write. The cost of sharing it is that its array-level
// refusals would name one field while the author was writing the other, sending
// someone who mistyped a fallback face to go and look at a key their command
// does not have. `subject` is the fix; this is the test that it is wired up.
func TestAMalformedTailIsReportedAsTheTailAndNotAsEntries(t *testing.T) {
	embed := func(tail string) string {
		return `{"kind":"embedFontFamily","version":1,"name":"c","family":"F","style":"Regular","licence":"OFL-1.1","licenceText":"t","copyright":"c","source":"s","mediaType":"font/ttf","data":"AA==","tail":` + tail + `}`
	}
	// ⚠ `null` is in this table because it was NOT refused when the table was
	// first written: encoding/json decodes null into a slice as a no-op, so a
	// null tail arrived as an EMPTY tail and the command was accepted. It is
	// the same null-shaped hole the entry decoder had, one level up.
	for _, tail := range []string{`"Noto Sans"`, `{"face":"Noto Sans"}`, `7`, `null`} {
		failure := fontChainRefusal(t, fontChainTemplate(t), embed(tail))
		if !strings.Contains(failure.Message, "the fallback tail ") {
			t.Errorf("refusal = %q, want it to name the fallback tail\n  for tail %s", failure.Message, tail)
		}
		if strings.Contains(failure.Message, "font chain entries must be") {
			t.Errorf("refusal = %q names `entries`, a key this command does not have\n  for tail %s", failure.Message, tail)
		}
	}
	// AND THE OTHER DIRECTION, or the row above passes for a decoder that
	// simply says "the fallback tail" whatever it is decoding.
	failure := fontChainRefusal(t, fontChainTemplate(t), `{"kind":"addFontChain","version":1,"name":"caption","entries":"Noto Sans"}`)
	if !strings.Contains(failure.Message, "font chain entries must be an array of font chain entries") {
		t.Errorf("refusal = %q, want it to name `entries`", failure.Message)
	}
	nullEntries := fontChainRefusal(t, fontChainTemplate(t), `{"kind":"addFontChain","version":1,"name":"caption","entries":null}`)
	if !strings.Contains(nullEntries.Message, "font chain entries is present and null") {
		t.Errorf("refusal = %q, want a null `entries` refused by name", nullEntries.Message)
	}
	if strings.Contains(failure.Message, "fallback tail") {
		t.Errorf("refusal = %q names the fallback tail, a key this command does not have", failure.Message)
	}
}

func TestLineBoundsSnapLengthAndPositionWithoutSnappingThickness(t *testing.T) {
	for _, tc := range []struct {
		name                                         string
		width, height, proposedWidth, proposedHeight string
		wantWidth, wantHeight                        int64
	}{
		{"horizontal", "72", "1.25", "406.422", "1.25", 408000, 1250},
		{"vertical", "1.25", "72", "1.25", "406.422", 1250, 408000},
		{"square", "10", "10", "22.2", "10", 24000, 10000},
		{"short horizontal", "72", "1.25", "1.25", "1.25", 1250, 1250},
		{"short vertical", "1.25", "72", "1.25", "1.251", 1250, 1251},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, snap := range []bool{true, false} {
				t.Run(fmt.Sprintf("snap=%t", snap), func(t *testing.T) {
					tpl := componentTemplate(t)
					before, _ := canvas(tpl)
					createdProjection, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"line","band":"content","x":6,"y":12,"width":72,"height":1,"snap":false}`))
					if err != nil {
						t.Fatal(err)
					}
					created := newProjectedComponent(t, before, createdProjection)
					// The inspector remains the author of Thickness, including
					// fractional values that are smaller than a grid interval.
					_, err = applyComponentCommand(tpl, []byte(fmt.Sprintf(`{"kind":"updateComponentProperties","version":1,"ids":[%q],"changes":{"width":{"op":"set","value":%s},"height":{"op":"set","value":%s}}}`, created.ID, tc.width, tc.height)))
					if err != nil {
						t.Fatal(err)
					}
					bounded, err := applyComponentCommand(tpl, []byte(fmt.Sprintf(`{"kind":"setComponentBounds","version":1,"id":%q,"x":7.3,"y":11.2,"width":%s,"height":%s,"snap":%t}`, created.ID, tc.proposedWidth, tc.proposedHeight, snap)))
					if err != nil {
						t.Fatal(err)
					}
					got := newProjectedComponent(t, before, bounded)
					wantX, wantY, wantW, wantH := int64(6000), int64(12000), tc.wantWidth, tc.wantHeight
					if !snap {
						wantX, wantY = 7300, 11200
						w, _ := lengthField(map[string]json.RawMessage{"width": json.RawMessage(tc.proposedWidth)}, "width")
						h, _ := lengthField(map[string]json.RawMessage{"height": json.RawMessage(tc.proposedHeight)}, "height")
						wantW, wantH = int64(w), int64(h)
					}
					if got.X != wantX || got.Y != wantY || got.Width != wantW || got.Height != wantH {
						t.Fatalf("got (%d,%d,%d,%d), want (%d,%d,%d,%d)", got.X, got.Y, got.Width, got.Height, wantX, wantY, wantW, wantH)
					}
				})
			}
		})
	}
}

func TestLineBoundsSnapAtBandEdgePreservesThickness(t *testing.T) {
	for _, tc := range []struct{ vertical, short bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		vertical := tc.vertical
		t.Run(fmt.Sprintf("vertical=%t/short=%t", vertical, tc.short), func(t *testing.T) {
			tpl := componentTemplate(t)
			before, _ := canvas(tpl)
			createdProjection, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"line","band":"pageHeader","x":0,"y":0,"width":12,"height":1,"snap":false}`))
			if err != nil {
				t.Fatal(err)
			}
			created := newProjectedComponent(t, before, createdProjection)
			var band designer.CanvasBand
			for _, candidate := range before.Bands {
				if candidate.Name == "pageHeader" {
					band = candidate
				}
			}
			w, h := int64(12000), int64(1250)
			if vertical {
				w, h = h, w
			}
			if tc.short {
				// The snapped origin leaves less space than the thickness.
				// Containment rounds length to zero, exercising the precise
				// axis fallback after containment, not only before it.
				w = band.Width%designer.GridIncrement + 1250
				h = w
				if vertical {
					h = band.Height%designer.GridIncrement + 1251
					w = h - 1
				}
			}
			_, err = applyComponentCommand(tpl, []byte(fmt.Sprintf(`{"kind":"updateComponentProperties","version":1,"ids":[%q],"changes":{"width":{"op":"set","value":%s},"height":{"op":"set","value":%s}}}`, created.ID, pointLiteral(w), pointLiteral(h))))
			if err != nil {
				t.Fatal(err)
			}
			x, y := band.Width-w, band.Height-h
			bounded, err := applyComponentCommand(tpl, []byte(fmt.Sprintf(`{"kind":"setComponentBounds","version":1,"id":%q,"x":%s,"y":%s,"width":%s,"height":%s,"snap":true}`, created.ID, pointLiteral(x), pointLiteral(y), pointLiteral(w), pointLiteral(h))))
			if err != nil {
				t.Fatal(err)
			}
			got := newProjectedComponent(t, before, bounded)
			if (!vertical && got.Height != h) || (vertical && got.Width != w) {
				t.Fatalf("thickness changed: %+v", got)
			}
			if tc.short && (got.Width != w || got.Height != h) {
				t.Fatalf("short line changed orientation or size: %+v, want %dx%d", got, w, h)
			}
			if got.X+got.Width > band.Width || got.Y+got.Height > band.Height {
				t.Fatalf("escaped band: %+v", got)
			}
		})
	}
}

func TestTablePlacementUsesFullBandWidthAndPreservesVerticalIntent(t *testing.T) {
	for _, landscape := range []bool{false, true} {
		for _, door := range []string{"dropComponent", "createComponent"} {
			for _, snap := range []bool{false, true} {
				t.Run(fmt.Sprintf("landscape=%t/%s/snap=%t", landscape, door, snap), func(t *testing.T) {
					tpl := componentTemplate(t)
					if landscape {
						tpl.doc.Page.Orientation = "landscape"
						tpl.doc.Page.Margin = template.Margin{Top: 21000, Right: 27123, Bottom: 31000, Left: 43111}
					}
					before, err := canvas(tpl)
					if err != nil {
						t.Fatal(err)
					}
					content := projectedBands(t, tpl)["content"]
					if content.Width%6000 == 0 {
						t.Fatal("fixture must exercise a non-grid-multiple width")
					}
					y := int64(15123)
					command := fmt.Sprintf(`{"kind":"dropComponent","version":1,"type":"table","x":%s,"y":%s,"snap":%t}`, pointLiteral(content.X+content.Width-1), pointLiteral(content.Y+y), snap)
					if door == "createComponent" {
						y += content.Height * 2
						command = fmt.Sprintf(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":%s,"y":%s,"width":72,"height":37,"snap":%t}`, pointLiteral(content.Width-1), pointLiteral(y), snap)
					}
					nextID := tpl.doc.NextID
					after, err := applyComponentCommand(tpl, []byte(command))
					if err != nil {
						t.Fatal(err)
					}
					wantY := y
					if snap {
						wantY = ((y + 3000) / 6000) * 6000
					}
					table := newProjectedComponent(t, before, after)
					if table.Band != "content" || table.X != 0 || table.Y != wantY || table.Width != content.Width || table.Height != 24000 || table.Resizable {
						t.Fatalf("placed table = %#v, want x=0 y=%d width=%d height=24000", table, wantY, content.Width)
					}
					for _, existing := range before.Components {
						if !reflect.DeepEqual(existing, componentByID(t, after, existing.ID)) {
							t.Fatal("creation changed an existing authored component")
						}
					}
					columns, err := tableColumns(tpl, table.ID)
					if err != nil || len(columns.Columns) != 1 {
						t.Fatalf("starter column projection = %#v, err=%v", columns, err)
					}
					column := columns.Columns[0]
					if table.ID != "e"+strconv.FormatInt(nextID, 36) || column.ID != "e"+strconv.FormatInt(nextID+1, 36) || tpl.doc.NextID != nextID+2 {
						t.Fatalf("creation IDs = %s/%s, nextId=%d", table.ID, column.ID, tpl.doc.NextID)
					}
					if column.Width != content.Width || column.Header != "" || column.Binding != "" || column.Footer != "" || !column.RowFieldEditable {
						t.Fatalf("starter column must be full-width, blank and editable: %#v", column)
					}
					_, _, _, element, err := findComponent(tpl, table.ID)
					if err != nil || !element.Width.Set || int64(element.Width.Value) != content.Width || element.Height.Set || column.Proportion != "1" {
						t.Fatalf("table stores free-box geometry: %#v, err=%v", element, err)
					}
					canonical, err := SerializeTemplate(tpl)
					if err != nil {
						t.Fatal(err)
					}
					reloaded, err := ParseTemplate(canonical)
					if err != nil {
						t.Fatal(err)
					}
					again, err := SerializeTemplate(reloaded)
					if err != nil || !bytes.Equal(canonical, again) {
						t.Fatalf("table creation did not round-trip canonically: %v", err)
					}
					reloadedCanvas, err := canvas(reloaded)
					if err != nil || !reflect.DeepEqual(table, componentByID(t, reloadedCanvas, table.ID)) {
						t.Fatalf("table geometry or column ID changed on reload: %v", err)
					}
				})
			}
		}
	}
}

func TestTableDropSnappingUsesActualHeaderHeight(t *testing.T) {
	for _, bandName := range []string{"pageHeader", "pageFooter"} {
		t.Run(bandName, func(t *testing.T) {
			tpl := imageDropTemplate(t, 197123, 64000)
			before, _ := canvas(tpl)
			band := projectedBands(t, tpl)[bandName]
			// 39pt + the real 24pt header fits. Snapping to 42pt would not,
			// so the existing edge rule must pull it back to 36pt.
			command := fmt.Sprintf(`{"kind":"dropComponent","version":1,"type":"table","x":%s,"y":%s,"snap":true}`, pointLiteral(band.X+band.Width-1), pointLiteral(band.Y+39000))
			after, err := applyComponentCommand(tpl, []byte(command))
			if err != nil {
				t.Fatal(err)
			}
			table := newProjectedComponent(t, before, after)
			if table.X != 0 || table.Y != 36000 || table.Width != band.Width || table.Height != 24000 {
				t.Fatalf("table edge snapping = %#v", table)
			}
		})
	}
}

func TestTableCreationRejectionsPreserveBytesAndBothIDs(t *testing.T) {
	const maxID = int64(1<<63 - 1)
	for _, tc := range []struct {
		name    string
		nextID  int64
		command string
	}{
		{"only one ID left", maxID - 1, `{"kind":"createComponent","version":1,"type":"table","band":"content","x":12,"y":12,"width":72,"height":24,"snap":false}`},
		{"one ID left via drop", maxID - 1, `{"kind":"dropComponent","version":1,"type":"table","x":72,"y":120,"snap":false}`},
		{"negative y", 100, `{"kind":"createComponent","version":1,"type":"table","band":"content","x":12,"y":-1,"width":72,"height":24,"snap":false}`},
		{"outside hit region", 100, `{"kind":"dropComponent","version":1,"type":"table","x":0,"y":120,"snap":true}`},
		{"header overflow", 100, `{"kind":"createComponent","version":1,"type":"table","band":"pageHeader","x":12,"y":59,"width":72,"height":24,"snap":false}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tpl := componentTemplate(t)
			tpl.doc.NextID = tc.nextID
			before, err := SerializeTemplate(tpl)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := applyComponentCommand(tpl, []byte(tc.command)); err == nil {
				t.Fatal("invalid creation succeeded")
			}
			after, err := SerializeTemplate(tpl)
			if err != nil || !bytes.Equal(before, after) || tpl.doc.NextID != tc.nextID {
				t.Fatalf("refusal mutated bytes or consumed IDs: %v", err)
			}
		})
	}
	// The last pair of usable IDs succeeds and leaves a valid nextId.
	tpl := componentTemplate(t)
	tpl.doc.NextID = maxID - 2
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":12,"y":12,"width":72,"height":24,"snap":false}`)); err != nil {
		t.Fatal(err)
	}
	canonical, err := SerializeTemplate(tpl)
	if err != nil || tpl.doc.NextID != 1<<63-1 {
		t.Fatalf("last pair of IDs failed: nextId=%d, err=%v", tpl.doc.NextID, err)
	}
	if _, err := ParseTemplate(canonical); err != nil {
		t.Fatal(err)
	}
}

func TestTableDuplicateAllocatesIndependentColumnsAndReloads(t *testing.T) {
	for _, count := range []int{1, 3} {
		t.Run(fmt.Sprintf("%d columns", count), func(t *testing.T) {
			tpl := componentTemplate(t)
			before, _ := canvas(tpl)
			created, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":40,"y":20,"width":72,"height":24,"snap":false}`))
			if err != nil {
				t.Fatal(err)
			}
			source := newProjectedComponent(t, before, created)
			columns, err := tableColumns(tpl, source.ID)
			if err != nil {
				t.Fatal(err)
			}
			if count > 1 {
				mustApplyToTable(t, tpl, fmt.Sprintf(`{"kind":"updateTableColumn","version":1,"id":%q,"columnId":%q,"field":"proportion","value":2}`, source.ID, columns.Columns[0].ID))
				for index := 1; index < count; index++ {
					mustApplyToTable(t, tpl, fmt.Sprintf(`{"kind":"addTableColumn","version":1,"id":%q,"index":%d}`, source.ID, index))
				}
			}
			columns, err = tableColumns(tpl, source.ID)
			if err != nil {
				t.Fatal(err)
			}
			before, _ = canvas(tpl)
			nextID := tpl.doc.NextID
			duplicated, err := applyComponentCommand(tpl, []byte(fmt.Sprintf(`{"kind":"duplicateComponent","version":1,"id":%q,"snap":true}`, source.ID)))
			if err != nil {
				t.Fatal(err)
			}
			duplicate := newProjectedComponent(t, before, duplicated)
			copied, err := tableColumns(tpl, duplicate.ID)
			if err != nil || len(copied.Columns) != count || duplicate.ID != "e"+strconv.FormatInt(nextID, 36) || tpl.doc.NextID != nextID+int64(count)+1 {
				t.Fatalf("duplicate IDs/columns = %#v, nextId=%d, err=%v", copied, tpl.doc.NextID, err)
			}
			for index, column := range copied.Columns {
				if column.ID != "e"+strconv.FormatInt(nextID+int64(index)+1, 36) {
					t.Fatalf("column %d retained or skipped its ID: %#v", index, column)
				}
				column.ID = columns.Columns[index].ID
				if !reflect.DeepEqual(column, columns.Columns[index]) {
					t.Fatalf("column %d changed properties while duplicating", index)
				}
			}
			unchanged, err := tableColumns(tpl, source.ID)
			if err != nil || !reflect.DeepEqual(columns, unchanged) {
				t.Fatalf("duplicate changed the source's column storage: %v", err)
			}
			canonical := canonicalBytes(t, tpl)
			reloaded, err := ParseTemplate(canonical)
			if err != nil || !bytes.Equal(canonical, canonicalBytes(t, reloaded)) {
				t.Fatalf("duplicate did not reload canonically: %v", err)
			}
			// Editing either table leaves the other table's columns alone.
			mustApplyToTable(t, tpl, fmt.Sprintf(`{"kind":"updateTableColumn","version":1,"id":%q,"columnId":%q,"field":"header","value":"Copy header"}`, duplicate.ID, copied.Columns[0].ID))
			unchanged, err = tableColumns(tpl, source.ID)
			if err != nil || !reflect.DeepEqual(columns, unchanged) {
				t.Fatalf("editing the duplicate changed the source: %v", err)
			}
			mustApplyToTable(t, tpl, fmt.Sprintf(`{"kind":"updateTableColumn","version":1,"id":%q,"columnId":%q,"field":"proportion","value":3}`, source.ID, columns.Columns[0].ID))
			edited, err := tableColumns(tpl, duplicate.ID)
			if err != nil || edited.Columns[0].Header != "Copy header" || edited.Columns[0].Width != copied.Columns[0].Width {
				t.Fatalf("source edit changed the duplicate: %#v, err=%v", edited, err)
			}
			if _, err := ParseTemplate(canonicalBytes(t, tpl)); err != nil {
				t.Fatalf("independently edited tables did not reload: %v", err)
			}
		})
	}
}

func TestTableDuplicatePreflightsEveryColumnIDWithoutMutation(t *testing.T) {
	const maxID = int64(1<<63 - 1)
	tpl := componentTemplate(t)
	before, _ := canvas(tpl)
	created, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	source := newProjectedComponent(t, before, created)
	columns, _ := tableColumns(tpl, source.ID)
	mustApplyToTable(t, tpl, fmt.Sprintf(`{"kind":"updateTableColumn","version":1,"id":%q,"columnId":%q,"field":"proportion","value":2}`, source.ID, columns.Columns[0].ID))
	mustApplyToTable(t, tpl, fmt.Sprintf(`{"kind":"addTableColumn","version":1,"id":%q,"index":1}`, source.ID))
	command := []byte(fmt.Sprintf(`{"kind":"duplicateComponent","version":1,"id":%q,"snap":false}`, source.ID))
	// This copy needs three IDs, so even two remaining slots must refuse.
	tpl.doc.NextID = maxID - 2
	canonical := canonicalBytes(t, tpl)
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := applyComponentCommand(tpl, command); err == nil {
			t.Fatal("duplicate accepted too few IDs for all its columns")
		}
		if !bytes.Equal(canonical, canonicalBytes(t, tpl)) {
			t.Fatal("duplicate refusal mutated source bytes or consumed an ID")
		}
	}
	tpl.doc.NextID = maxID - 3
	if _, err := applyComponentCommand(tpl, command); err != nil {
		t.Fatal(err)
	}
	if tpl.doc.NextID != maxID {
		t.Fatalf("last three ID slots ended at %d", tpl.doc.NextID)
	}
	if _, err := ParseTemplate(canonicalBytes(t, tpl)); err != nil {
		t.Fatal(err)
	}
}

func TestTableCreationUsesDefaultFontAndRendersPopulatedItems(t *testing.T) {
	for _, door := range []string{"createComponent", "dropComponent"} {
		t.Run(door, func(t *testing.T) {
			tpl := componentTemplate(t)
			tpl.doc.Bands.PageHeader.Elements = nil
			tpl.doc.Bands.Content.Elements = nil
			tpl.doc.Bands.PageFooter.Elements = nil
			before, _ := canvas(tpl)
			command := `{"kind":"createComponent","version":1,"type":"table","band":"content","x":50,"y":24,"width":72,"height":24,"snap":false}`
			if door == "dropComponent" {
				content := projectedBands(t, tpl)["content"]
				command = fmt.Sprintf(`{"kind":"dropComponent","version":1,"type":"table","x":%s,"y":%s,"snap":false}`, pointLiteral(content.X+50000), pointLiteral(content.Y+24000))
			}
			created, err := applyComponentCommand(tpl, []byte(command))
			if err != nil {
				t.Fatal(err)
			}
			table := newProjectedComponent(t, before, created)
			if table.FontFamily == nil || *table.FontFamily != defaultFontFamily(tpl) {
				t.Fatalf("table did not inherit the declared default font: %#v", table.FontFamily)
			}
			columns, err := tableColumns(tpl, table.ID)
			if err != nil || columns.Columns[0].Header != "" || columns.Columns[0].Binding != "" {
				t.Fatalf("font default invented column content: %#v, err=%v", columns, err)
			}
			for _, editHeader := range []bool{false, true} {
				if editHeader {
					mustApplyToTable(t, tpl, fmt.Sprintf(`{"kind":"updateTableColumn","version":1,"id":%q,"columnId":%q,"field":"header","value":"Description"}`, table.ID, columns.Columns[0].ID))
				}
				reloaded, err := ParseTemplate(canonicalBytes(t, tpl))
				if err != nil {
					t.Fatal(err)
				}
				result, err := Render(reloaded, Data(`{"items":[{},{}]}`), Params(`{}`), testShippedFontSet())
				if err != nil || !bytes.HasPrefix(result.Bytes, []byte("%PDF-")) || len(result.Diagnostics) != 0 {
					t.Fatalf("table render (edited header=%t) = %v, diagnostics=%#v", editHeader, err, result.Diagnostics)
				}
			}
		})
	}
}

func tableColumnAuthoringFixture(t *testing.T, widths []geom.Length, spare geom.Length) (*Template, string) {
	t.Helper()
	var total geom.Length
	for _, width := range widths {
		total += width
	}
	tpl := imageDropTemplate(t, total+spare, 20000)
	before, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	created, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	id := newProjectedComponent(t, before, created).ID
	_, _, _, element, err := findComponent(tpl, id)
	if err != nil {
		t.Fatal(err)
	}
	element.Width = template.Presence[geom.Length]{} // absolute-width fixture
	element.Table.Value.Columns = nil
	for i, width := range widths {
		element.Table.Value.Columns = append(element.Table.Value.Columns, template.Column{
			ID: template.AllocateElementID(tpl.doc), Label: fmt.Sprintf("Label %d", i), Width: width,
			Bind: "{{row.date}}", Align: template.Presence[string]{Set: true, Value: "right"},
			Footer: template.Presence[string]{Set: true, Value: "count"}, FooterFormat: template.Presence[string]{Set: true, Value: "0"},
		})
		tpl.doc.NextID++
	}
	canonical, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	tpl, err = ParseTemplate(canonical)
	if err != nil {
		t.Fatal(err)
	}
	return tpl, id
}

func TestAddTableColumnFitsWithoutChangingOtherColumnProperties(t *testing.T) {
	for _, tc := range []struct {
		name   string
		widths []geom.Length
		spare  geom.Length
		index  int
		want   []int64
	}{
		{"normal with exact room", []geom.Length{100000, 50000}, 72000, 1, []int64{100000, 72000, 50000}},
		{"one millipoint short uses split", []geom.Length{100000, 50000}, 71999, 2, []int64{50000, 50000, 50000}},
		{"full width starter", []geom.Length{523276}, 0, 1, []int64{261638, 261638}},
		{"crowded widest keeps odd millipoint", []geom.Length{40001, 130003, 60000}, 100, 1, []int64{40001, 65001, 65002, 60000}},
		{"first widest wins tie", []geom.Length{90001, 90001, 90000}, 0, 3, []int64{45001, 90001, 90000, 45000}},
		{"smallest split", []geom.Length{1, 2}, 0, 0, []int64{1, 1, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tpl, id := tableColumnAuthoringFixture(t, tc.widths, tc.spare)
			_, _, _, original, _ := findComponent(tpl, id)
			beforeColumns := append([]template.Column(nil), original.Table.Value.Columns...)
			nextID := tpl.doc.NextID
			canvas, err := applyComponentCommand(tpl, []byte(fmt.Sprintf(`{"kind":"addTableColumn","version":1,"id":%q,"index":%d}`, id, tc.index)))
			if err != nil {
				t.Fatal(err)
			}
			view, err := tableColumns(tpl, id)
			if err != nil {
				t.Fatal(err)
			}
			var widths []int64
			var total int64
			for _, column := range view.Columns {
				widths = append(widths, column.Width)
				total += column.Width
			}
			if !reflect.DeepEqual(widths, tc.want) {
				t.Fatalf("widths = %v, want %v", widths, tc.want)
			}
			if got := componentByID(t, canvas, id).Width; got != total {
				t.Fatalf("canvas width = %d, want %d", got, total)
			}
			_, band, _, element, _ := findComponent(tpl, id)
			if total > band.Width-int64(element.X) {
				t.Fatal("added column exceeded band budget")
			}
			newColumn := view.Columns[tc.index]
			if newColumn.Binding != "" || newColumn.RowField != "" || !newColumn.RowFieldEditable || newColumn.Align != "left" || newColumn.Footer != "" || newColumn.FooterFormat != "" {
				t.Fatalf("new column inherited authoring: %#v", newColumn)
			}
			if tpl.doc.NextID != nextID+1 {
				t.Fatalf("add allocated more than one id: %d -> %d", nextID, tpl.doc.NextID)
			}
			for i, beforeColumn := range beforeColumns {
				afterIndex := i
				if i >= tc.index {
					afterIndex++
				}
				afterColumn := element.Table.Value.Columns[afterIndex]
				afterColumn.Width = beforeColumn.Width
				if !reflect.DeepEqual(afterColumn, beforeColumn) {
					t.Fatalf("existing column %d changed beyond its width: %#v -> %#v", i, beforeColumn, afterColumn)
				}
			}
			canonical, err := SerializeTemplate(tpl)
			if err != nil {
				t.Fatal(err)
			}
			reloaded, err := ParseTemplate(canonical)
			if err != nil {
				t.Fatal(err)
			}
			again, err := tableColumns(reloaded, id)
			if err != nil || !reflect.DeepEqual(view, again) {
				t.Fatalf("reopen changed projection: %#v, err=%v", again, err)
			}
		})
	}
}

func TestAddTableColumnSplitRefusalsAreAtomic(t *testing.T) {
	for _, name := range []string{"no splittable column", "id exhaustion", "invalid index", "overflow after split"} {
		t.Run(name, func(t *testing.T) {
			widths := []geom.Length{60000, 40000}
			if name == "no splittable column" {
				widths = []geom.Length{1, 1}
			}
			tpl, id := tableColumnAuthoringFixture(t, widths, 0)
			index := len(widths)
			switch name {
			case "id exhaustion":
				tpl.doc.NextID = 1<<63 - 1
			case "invalid index":
				index++
			case "overflow after split":
				_, _, _, element, _ := findComponent(tpl, id)
				element.X = 1
			}
			before, err := SerializeTemplate(tpl)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := applyComponentCommand(tpl, []byte(fmt.Sprintf(`{"kind":"addTableColumn","version":1,"id":%q,"index":%d}`, id, index))); err == nil {
				t.Fatal("invalid add succeeded")
			}
			after, err := SerializeTemplate(tpl)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("refusal changed canonical bytes: %v", err)
			}
		})
	}
}

func TestTableColumnBindingCanBeTypedAndClearedWithoutSample(t *testing.T) {
	tpl, id := tableColumnAuthoringFixture(t, []geom.Length{100000}, 0)
	view, _ := tableColumns(tpl, id)
	columnID := view.Columns[0].ID
	applyField := func(field string) error {
		_, err := applyComponentCommand(tpl, []byte(fmt.Sprintf(`{"kind":"updateTableColumnBinding","version":1,"id":%q,"columnId":%q,"field":%s}`, id, columnID, field)))
		return err
	}
	for _, alias := range []string{"", "txn"} {
		if _, err := applyComponentCommand(tpl, []byte(fmt.Sprintf(`{"kind":"configureTableBinding","version":1,"id":%q,"collection":"transactions[]","alias":%q}`, id, alias))); err != nil {
			t.Fatal(err)
		}
		if err := applyField(`"customer.name"`); err != nil {
			t.Fatal(err)
		}
		view, err := tableColumns(tpl, id)
		if err != nil {
			t.Fatal(err)
		}
		wantAlias := alias
		if wantAlias == "" {
			wantAlias = "row"
		}
		if view.Columns[0].Binding != "{{"+wantAlias+".customer.name}}" || view.Columns[0].RowField != "customer.name" || !view.Columns[0].RowFieldEditable {
			t.Fatalf("binding projection = %#v", view.Columns[0])
		}
		before, _ := SerializeTemplate(tpl)
		for _, invalid := range []string{`null`, `  null  `, `7`, `"bad path"`, `"customer..name"`, `"{{row.date}}"`, strconv.Quote(strings.Repeat("a", 193))} {
			if err := applyField(invalid); err == nil {
				t.Fatalf("invalid field accepted: %s", invalid)
			}
			after, _ := SerializeTemplate(tpl)
			if !bytes.Equal(before, after) {
				t.Fatalf("invalid field changed committed binding: %s", invalid)
			}
		}
		if err := applyField(`""`); err != nil {
			t.Fatal(err)
		}
		view, _ = tableColumns(tpl, id)
		if view.Columns[0].Binding != "" || view.Columns[0].RowField != "" || !view.Columns[0].RowFieldEditable {
			t.Fatalf("clear projection = %#v", view.Columns[0])
		}
		cleared, _ := SerializeTemplate(tpl)
		if err := applyField(`""`); err != nil {
			t.Fatal(err)
		}
		again, _ := SerializeTemplate(tpl)
		if !bytes.Equal(cleared, again) {
			t.Fatal("no-op clear changed canonical bytes")
		}
	}
}

func TestTableColumnBindingAndAliasRespectTheFullExpressionLimit(t *testing.T) {
	tpl, id := tableColumnAuthoringFixture(t, []geom.Length{100000}, 0)
	view, _ := tableColumns(tpl, id)
	columnID := view.Columns[0].ID
	longAlias := strings.Repeat("a", 64)
	configure := func(alias string) []byte {
		return []byte(fmt.Sprintf(`{"kind":"configureTableBinding","version":1,"id":%q,"collection":"transactions[]","alias":%q}`, id, alias))
	}
	bind := func(length int) []byte {
		return []byte(fmt.Sprintf(`{"kind":"updateTableColumnBinding","version":1,"id":%q,"columnId":%q,"field":%q}`, id, columnID, strings.Repeat("f", length)))
	}
	for _, command := range [][]byte{configure(longAlias), bind(187)} {
		if _, err := applyComponentCommand(tpl, command); err != nil {
			t.Fatalf("exact-limit setup: %v", err)
		}
	}
	view, err := tableColumns(tpl, id)
	if err != nil || len(view.Columns[0].Binding) != maxCanvasBindingString {
		t.Fatalf("exact-limit projection = %#v, err=%v", view, err)
	}
	before, _ := SerializeTemplate(tpl)
	for _, length := range []int{188, 192} {
		if _, err := applyComponentCommand(tpl, bind(length)); err == nil {
			t.Fatalf("%d-character field exceeded the full expression limit without refusal", length)
		}
		after, _ := SerializeTemplate(tpl)
		if !bytes.Equal(before, after) {
			t.Fatal("overlong expression mutated bytes")
		}
	}
	for _, command := range [][]byte{configure(""), bind(192)} {
		if _, err := applyComponentCommand(tpl, command); err != nil {
			t.Fatal(err)
		}
	}
	before, _ = SerializeTemplate(tpl)
	if _, err := applyComponentCommand(tpl, configure(longAlias)); err == nil {
		t.Fatal("alias migration produced an unprojectable expression")
	}
	after, _ := SerializeTemplate(tpl)
	if !bytes.Equal(before, after) {
		t.Fatal("overlong alias migration mutated bytes")
	}
	if _, err := tableColumns(tpl, id); err != nil {
		t.Fatalf("refused alias change stranded projection: %v", err)
	}
}

func TestClearTableColumnBindingKeepsAggregateSourceValidation(t *testing.T) {
	for _, aggregate := range []string{"sum", "avg"} {
		t.Run(aggregate, func(t *testing.T) {
			tpl, id := tableColumnAuthoringFixture(t, []geom.Length{100000}, 0)
			view, _ := tableColumns(tpl, id)
			columnID := view.Columns[0].ID
			footerCommand := func(source string) []byte {
				return []byte(fmt.Sprintf(`{"kind":"updateTableColumnFooter","version":1,"id":%q,"columnId":%q,"footer":%q,"footerOf":%q,"footerFormat":"0.00"}`, id, columnID, aggregate, source))
			}
			if _, err := applyComponentCommand(tpl, footerCommand("")); err != nil {
				t.Fatal(err)
			}
			before, _ := SerializeTemplate(tpl)
			clear := []byte(fmt.Sprintf(`{"kind":"updateTableColumnBinding","version":1,"id":%q,"columnId":%q,"field":""}`, id, columnID))
			if _, err := applyComponentCommand(tpl, clear); err == nil {
				t.Fatal("clear removed an aggregate's derived source")
			}
			after, _ := SerializeTemplate(tpl)
			if !bytes.Equal(before, after) {
				t.Fatal("refused clear changed binding or aggregate")
			}
			view, err := tableColumns(tpl, id)
			if err != nil || view.Columns[0].Binding != "{{row.date}}" || view.Columns[0].Footer != aggregate || view.Columns[0].FooterOf != "" {
				t.Fatalf("refused clear = %#v, err=%v", view, err)
			}
			if _, err := applyComponentCommand(tpl, footerCommand("items.date")); err != nil {
				t.Fatal(err)
			}
			if _, err := applyComponentCommand(tpl, clear); err != nil {
				t.Fatalf("clear with explicit source: %v", err)
			}
			view, err = tableColumns(tpl, id)
			if err != nil || view.Columns[0].Binding != "" || view.Columns[0].Footer != aggregate || view.Columns[0].FooterOf != "items.date" || view.Columns[0].FooterFormat != "0.00" {
				t.Fatalf("clear changed explicit aggregate: %#v, err=%v", view, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SPEC-INSTALL-ALL-FACE-CUTS STORY 2: EMBEDDING A CUT ON FIRST USE.

// embedCutCommand builds the command the designer sends when the author
// presses B on a family whose bold is held. Written out in full for
// embedCommand's reason — componentFields(raw, 13) counts kind and version
// too, so a builder that quietly dropped a key would move the refusal a test
// is measuring — and the style/licence are parameters because the two admission
// gates below read them.
func embedCutCommand(t *testing.T, chain string, index int, cut string, face []byte) string {
	t.Helper()
	return embedCutCommandDeclaring(t, chain, index, cut, face, "Bold", "OFL-1.1")
}

func embedCutCommandDeclaring(t *testing.T, chain string, index int, cut string, face []byte, style, licence string) string {
	t.Helper()
	return `{"kind":"embedFontCut","version":1,"name":` + quoteForCommand(t, chain) +
		`,"index":` + strconv.Itoa(index) + `,"cut":` + quoteForCommand(t, cut) +
		`,"family":"Noto Sans Thai","style":` + quoteForCommand(t, style) +
		`,"licence":` + quoteForCommand(t, licence) +
		`,"licenceText":"This Font Software is licensed under the SIL Open Font License, Version 1.1."` +
		`,"copyright":"Copyright 2022 The Noto Project Authors","source":"catalogue"` +
		`,"mediaType":"font/ttf","data":"` + base64.StdEncoding.EncodeToString(face) + `"}`
}

// embeddedChainTemplate is the precondition every test below shares: a chain
// whose first entry carries a face of its own, which is the only shape a cut
// can be attached to.
func embeddedChainTemplate(t *testing.T) (*Template, []byte, string) {
	t.Helper()
	tpl := fontChainTemplate(t)
	base := testShippedNotoSansThai
	fontChainAccepted(t, tpl, embedCommand(t, "Noto Sans Thai", base, `["Noto Sans SC"]`))
	return tpl, base, embeddedKeyOf(base)
}

// TestEmbedFontCutAttachesTheCutAndWritesItsOwnRecord is the story's first
// matrix row and the write half of AC1: ONE command puts a second face in the
// document, under its own content hash, carrying its OWN six-field licence
// record — and the entry that already had a face now declares the cut.
func TestEmbedFontCutAttachesTheCutAndWritesItsOwnRecord(t *testing.T) {
	tpl, _, baseKey := embeddedChainTemplate(t)
	bold := testShippedNotoSans
	boldKey := embeddedKeyOf(bold)

	fontChainAccepted(t, tpl, embedCutCommand(t, "Noto Sans Thai", 0, "bold", bold))

	chain := tpl.doc.Fonts["Noto Sans Thai"]
	if chain[0].AssetKey != baseKey {
		t.Fatalf("the BASE moved: entry.AssetKey = %q, want %q — the embedded Regular is never touched", chain[0].AssetKey, baseKey)
	}
	if chain[0].Bold != boldKey {
		t.Fatalf("entry.Bold = %q, want the cut's own content hash %q", chain[0].Bold, boldKey)
	}
	if chain[0].Italic != "" || chain[0].BoldItalic != "" {
		t.Errorf("a command asked for ONE cut and declared others: %#v", chain[0])
	}
	asset, ok := tpl.doc.Assets[boldKey]
	if !ok {
		t.Fatalf("the cut was declared but its bytes were not stored under %s — a chain naming an asset the document does not carry is a located load error", boldKey)
	}
	if asset.MediaType != "font/ttf" {
		t.Errorf("mediaType = %q, want font/ttf", asset.MediaType)
	}
	// AD-26 / I-7: A VARIANT ASSET KEY IS AN EMBEDDED FACE, so it carries the
	// terms the loader requires of one. A cut written without them puts a
	// document the engine's own parser refuses one step away.
	if !asset.Font.Set || asset.Font.Null {
		t.Fatal("the cut's asset carries no font record at all")
	}
	for _, want := range []struct {
		name  string
		value template.Presence[string]
	}{
		{"family", asset.Font.Value.Family},
		{"style", asset.Font.Value.Style},
		{"licence", asset.Font.Value.Licence},
		{"licenceText", asset.Font.Value.LicenceText},
		{"copyright", asset.Font.Value.Copyright},
		{"source", asset.Font.Value.Source},
	} {
		if !want.value.Set || want.value.Null || strings.TrimSpace(want.value.Value) == "" {
			t.Errorf("the cut's record carries no %s", want.name)
		}
	}
	if asset.Font.Value.Style.Value != "Bold" {
		t.Errorf("the cut's recorded style = %q, want Bold", asset.Font.Value.Style.Value)
	}
	// THE DOCUMENT NOW CARRIES TWO FACES AND NAMES BOTH. EmbeddedAssetKeys is
	// the model's own enumeration and is what every safety walk asks.
	if keys := chain[0].EmbeddedAssetKeys(); !slices.Equal(keys, []string{baseKey, boldKey}) {
		t.Errorf("EmbeddedAssetKeys() = %v, want the base then the cut", keys)
	}
}

// TestEmbedFontCutIsProjectedToTheCanvas closes the loop the panel reads: the
// projection is where `chainDeclaresCut` looks, so a command that wrote the
// model and did not reach the canvas would leave the absence sentence standing
// over a cut the document carries.
func TestEmbedFontCutIsProjectedToTheCanvas(t *testing.T) {
	tpl, _, _ := embeddedChainTemplate(t)
	bold := testShippedNotoSans
	projection := fontChainAccepted(t, tpl, embedCutCommand(t, "Noto Sans Thai", 0, "bold", bold))
	for _, chain := range projection.FontChains {
		if chain.Name != "Noto Sans Thai" {
			continue
		}
		if chain.Entries[0].Bold != embeddedKeyOf(bold) {
			t.Fatalf("the projected entry declares bold %q, want %q", chain.Entries[0].Bold, embeddedKeyOf(bold))
		}
		return
	}
	t.Fatal("the chain the cut was attached to is not in the projection at all")
}

// TestEmbedFontCutRefusesACutAlreadyDeclaredOverDifferentBytes is the spec's
// "an embedded face is never replaced". The asset key IS the content, so a
// silent overwrite is a silently different document — and the embedded vintage
// is frozen precisely so a no-op open/save round trip stays byte-identical.
func TestEmbedFontCutRefusesACutAlreadyDeclaredOverDifferentBytes(t *testing.T) {
	tpl, _, _ := embeddedChainTemplate(t)
	first := testShippedNotoSans
	fontChainAccepted(t, tpl, embedCutCommand(t, "Noto Sans Thai", 0, "bold", first))

	second := testRobotoFontBytes
	failure := fontChainRefusal(t, tpl, embedCutCommandDeclaring(t, "Noto Sans Thai", 0, "bold", second, "Bold", "Apache-2.0"))
	if !strings.Contains(failure.Message, "already declares a bold") {
		t.Errorf("the refusal does not say the cut is taken: %s", failure.Message)
	}
	// ⚠ THE LITERAL, NOT fontChainEntryPath's OWN OUTPUT. Every other assertion
	// about an entry path in this file computes the expectation by calling the
	// helper, so rewriting the helper to truncate the INDEX away — the exact
	// defect its own doc comment warns about — would leave all of them green.
	// One assertion has to say what the path IS.
	if failure.DataPath != "fonts.Noto Sans Thai[0]" {
		t.Errorf("the refusal is located at %q, want the ENTRY path \"fonts.Noto Sans Thai[0]\"", failure.DataPath)
	}
	if tpl.doc.Fonts["Noto Sans Thai"][0].Bold != embeddedKeyOf(first) {
		t.Error("the refused command replaced the declared cut anyway")
	}
	if _, exists := tpl.doc.Assets[embeddedKeyOf(second)]; exists {
		t.Error("a refused cut stored its bytes anyway")
	}
}

// TestEmbedFontCutNoOpsOnACutAlreadyDeclaredOverTheSameBytes is the matrix's
// "cut already declared" row: the designer plans to send the property alone,
// and this is the BACKSTOP that makes the plan safe rather than load-bearing.
//
// THE BYTE COMPARISON IS THE ASSERTION. wasm.Engine.Apply pushes no history
// entry when the canonical bytes do not move, so "no second undo entry" is a
// consequence of the document standing still and is measured that way.
func TestEmbedFontCutNoOpsOnACutAlreadyDeclaredOverTheSameBytes(t *testing.T) {
	tpl, _, _ := embeddedChainTemplate(t)
	bold := testShippedNotoSans
	command := embedCutCommand(t, "Noto Sans Thai", 0, "bold", bold)
	fontChainAccepted(t, tpl, command)
	before, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}

	fontChainAccepted(t, tpl, command)

	after, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("re-declaring the SAME cut over the SAME bytes moved the document, so it would cost a second revision and a second undo entry")
	}
}

// TestEmbedFontCutRefusesTheEntrysOwnBase is the matrix's "cut bytes equal the
// base" row. The designer never sends it; this is D-11.2.11's rule reaching the
// command door so the refusal is LOCATED, rather than arriving from the
// transaction's reparse as the unlocated "font chains did not pass format
// validation".
func TestEmbedFontCutRefusesTheEntrysOwnBase(t *testing.T) {
	tpl, base, _ := embeddedChainTemplate(t)
	failure := fontChainRefusal(t, tpl, embedCutCommand(t, "Noto Sans Thai", 0, "bold", base))
	if !strings.Contains(failure.Message, "own base") {
		t.Errorf("the refusal does not name the self-reference: %s", failure.Message)
	}
	if failure.DataPath != fontChainEntryPath("Noto Sans Thai", 0) {
		t.Errorf("the refusal is located at %q, want the entry", failure.DataPath)
	}
}

// TestEmbedFontCutRefusesAFaceEntry is AD-8 at this door: a face entry's
// variants are FontSet face names, never assets keys, and shippedFamilyEntry
// already declares them at declare time. `body` is the fixture's face-only
// chain.
func TestEmbedFontCutRefusesAFaceEntry(t *testing.T) {
	tpl := fontChainTemplate(t)
	failure := fontChainRefusal(t, tpl, embedCutCommand(t, "body", 0, "bold", testShippedNotoSans))
	for _, want := range []string{"AD-8", "FontSet face names"} {
		if !strings.Contains(failure.Message, want) {
			t.Errorf("the refusal does not name %s: %s", want, failure.Message)
		}
	}
	if failure.DataPath != fontChainEntryPath("body", 0) {
		t.Errorf("the refusal is located at %q, want the entry", failure.DataPath)
	}
}

// TestEmbedFontCutRefusesACutOutsideTheClosedSet keeps the door's vocabulary
// the format's. The refusal states what the author MAY write and does not echo
// what they did — Story 8.3's rule, and the asked-for value is unbounded text.
func TestEmbedFontCutRefusesACutOutsideTheClosedSet(t *testing.T) {
	tpl, _, _ := embeddedChainTemplate(t)
	for _, asked := range []string{"semibold", "regular", "Bold", "asset", "face"} {
		failure := fontChainRefusal(t, tpl, embedCutCommand(t, "Noto Sans Thai", 0, asked, testShippedNotoSans))
		for _, want := range commandFontChainCutKeys() {
			if !strings.Contains(failure.Message, `"`+want+`"`) {
				t.Errorf("%q: the refusal does not offer %q: %s", asked, want, failure.Message)
			}
		}
	}
}

// TestEmbedFontCutRefusesAnUnknownChainAndAnOutOfRangeIndex reuses the
// vocabulary every other chain command refuses with, which is the point: the
// chain is resolved by declaredFontChain and the index by fontChainIndex, so
// this command cannot drift from addFontChainEntry / move / remove in what an
// out-of-range index means.
func TestEmbedFontCutRefusesAnUnknownChainAndAnOutOfRangeIndex(t *testing.T) {
	tpl, _, _ := embeddedChainTemplate(t)
	missing := fontChainRefusal(t, tpl, embedCutCommand(t, "nothing", 0, "bold", testShippedNotoSans))
	if !strings.Contains(missing.Message, `no font chain named "nothing" is declared`) {
		t.Errorf("unknown chain refusal = %s", missing.Message)
	}
	for _, index := range []int{-1, 2, 99} {
		failure := fontChainRefusal(t, tpl, embedCutCommand(t, "Noto Sans Thai", index, "bold", testShippedNotoSans))
		if !strings.Contains(failure.Message, "entry index is out of range") {
			t.Errorf("index %d: refusal = %s", index, failure.Message)
		}
	}
}

// TestEmbedFontCutClearsTheSameBarAsTheBase is the spec's central constraint:
// a variant asset key clears the SAME admission gates as the entry's own face.
// A weaker door here would be a way to put an unrenderable, variable or
// mislicensed bold into a document the strict door refuses to accept.
//
// THE THREE GATES ARE EXERCISED THROUGH THE SAME FIXTURES THE PICK USES, so a
// gate deleted from this handler cannot be covered by the pick's own tests.
func TestEmbedFontCutClearsTheSameBarAsTheBase(t *testing.T) {
	at := fontChainEntryPath("Noto Sans Thai", 0)
	t.Run("a variable face", func(t *testing.T) {
		tpl, _, _ := embeddedChainTemplate(t)
		failure := fontChainRefusal(t, tpl, embedCutCommand(t, "Noto Sans Thai", 0, "bold", testNotoSansThaiVariableFontBytes))
		for _, want := range []string{"`fvar`", "fonttools varLib.instancer"} {
			if !strings.Contains(failure.Message, want) {
				t.Errorf("the refusal does not mention %s: %s", want, failure.Message)
			}
		}
		if failure.DataPath != at {
			t.Errorf("located at %q, want the entry", failure.DataPath)
		}
	})
	t.Run("bytes that contradict the declared licence", func(t *testing.T) {
		tpl, _, _ := embeddedChainTemplate(t)
		// Roboto's own name table names the Apache License, so declaring it
		// OFL-1.1 is a true contradiction over real committed bytes.
		failure := fontChainRefusal(t, tpl, embedCutCommandDeclaring(t, "Noto Sans Thai", 0, "bold", testRobotoFontBytes, "Bold", "OFL-1.1"))
		for _, want := range []string{"OFL-1.1", "Apache License"} {
			if !strings.Contains(failure.Message, want) {
				t.Errorf("the refusal does not name %s: %s", want, failure.Message)
			}
		}
		if _, exists := tpl.doc.Assets[embeddedKeyOf(testRobotoFontBytes)]; exists {
			t.Error("the refused face was written to t.doc.Assets anyway")
		}
	})
	t.Run("a blank record field", func(t *testing.T) {
		tpl, _, _ := embeddedChainTemplate(t)
		blank := strings.Replace(embedCutCommand(t, "Noto Sans Thai", 0, "bold", testShippedNotoSans),
			`"copyright":"Copyright 2022 The Noto Project Authors"`, `"copyright":"   "`, 1)
		failure := fontChainRefusal(t, tpl, blank)
		if !strings.Contains(failure.Message, "copyright must be a non-empty string") {
			t.Errorf("refusal = %s", failure.Message)
		}
	})
	t.Run("bytes this build cannot read as a face", func(t *testing.T) {
		tpl, _, _ := embeddedChainTemplate(t)
		failure := fontChainRefusal(t, tpl, embedCutCommand(t, "Noto Sans Thai", 0, "bold", []byte("not a face at all")))
		if failure.Message == "" {
			t.Error("unreadable bytes were admitted with no reason")
		}
	})
}

// TestEmbedFontCutCountsThirteenFields keeps the arity part of the contract.
// componentFields is an exact count, so this is the refusal a designer builder
// that gained or lost a key would earn — unlocated, like every other arity
// refusal, because nothing has been read yet.
func TestEmbedFontCutCountsThirteenFields(t *testing.T) {
	tpl, _, _ := embeddedChainTemplate(t)
	full := embedCutCommand(t, "Noto Sans Thai", 0, "bold", testShippedNotoSans)
	for _, command := range []string{
		strings.Replace(full, `"index":0,`, ``, 1),
		strings.Replace(full, `"source":"catalogue"`, `"source":"catalogue","extra":"x"`, 1),
	} {
		_, err := applyComponentCommand(tpl, []byte(command))
		if err == nil || !strings.Contains(err.Error(), "unknown or missing fields") {
			t.Errorf("arity refusal = %v", err)
		}
	}
}

// TestRemovingAnEntryDropsTheCutItDeclaredToo is DW-80's second edition, the
// half that keeps a document from accumulating faces nothing can reach.
//
// RED-PROVED BY DELETION (2026-09-20): restoring dropUnnamedFontAssets' old
// `entry.AssetKey`-only body leaves the bold behind and reds this test, while
// the base half stays green — which is what says the two arms are measured
// apart.
func TestRemovingAnEntryDropsTheCutItDeclaredToo(t *testing.T) {
	tpl, _, baseKey := embeddedChainTemplate(t)
	bold := testShippedNotoSans
	boldKey := embeddedKeyOf(bold)
	fontChainAccepted(t, tpl, embedCutCommand(t, "Noto Sans Thai", 0, "bold", bold))

	fontChainAccepted(t, tpl, `{"kind":"removeFontChainEntry","version":1,"name":"Noto Sans Thai","index":0}`)

	if _, ok := tpl.doc.Assets[baseKey]; ok {
		t.Error("the base face nothing names any longer was left in the document")
	}
	if _, ok := tpl.doc.Assets[boldKey]; ok {
		t.Error("the CUT nothing names any longer was left in the document — un-naming an entry un-names every key it declared, not only its base")
	}
}

// TestACutASecondChainStillNamesIsRetained is the story's last acceptance
// criterion and the DANGEROUS direction: under-reporting a reference deletes a
// live face and no compile error announces it.
//
// RED-PROVED BY DELETION (2026-09-20): narrowing assetKeyReferenced back to
// `entry.Embedded() && entry.AssetKey == key` reds this test — the second
// chain's declared bold becomes invisible and the drop takes it.
func TestACutASecondChainStillNamesIsRetained(t *testing.T) {
	tpl, _, baseKey := embeddedChainTemplate(t)
	bold := testShippedNotoSans
	boldKey := embeddedKeyOf(bold)
	fontChainAccepted(t, tpl, embedCutCommand(t, "Noto Sans Thai", 0, "bold", bold))

	// A SECOND chain declaring the SAME cut, written into the model directly:
	// embedFontFamily refuses a chain name it already holds and embedFontCut
	// needs a base entry, so no pair of commands reaches this state, and the
	// state is entirely legal in the format.
	tpl.doc.Fonts["alsoThai"] = []template.FontChainEntry{{AssetKey: baseKey, Bold: boldKey}, template.FaceEntry("Noto Sans SC")}

	fontChainAccepted(t, tpl, `{"kind":"removeFontChainEntry","version":1,"name":"Noto Sans Thai","index":0}`)

	if _, ok := tpl.doc.Assets[boldKey]; !ok {
		t.Fatal("a CUT a second chain still names was deleted")
	}
	if _, ok := tpl.doc.Assets[baseKey]; !ok {
		t.Fatal("a BASE face a second chain still names was deleted")
	}
}

// TestThePickDedupeDoesNotSeeAnotherChainsDeclaredCut is why the two
// predicates are two predicates.
//
// The safety walk had to become variant-aware; the DEDUPE must not. A family
// whose upright Regular happens to be the bytes some other family declared as
// its bold is a family this document does NOT carry, and answering "already
// here" would make the pick do nothing at all — no asset, no chain, no error,
// and a control that looks broken.
//
// RED-PROVED BY SUBSTITUTION (2026-09-20): pointing embedFontFamily's dedupe at
// assetKeyReferenced reds this test and nothing else in the suite.
func TestThePickDedupeDoesNotSeeAnotherChainsDeclaredCut(t *testing.T) {
	tpl, _, _ := embeddedChainTemplate(t)
	shared := testShippedNotoSans
	fontChainAccepted(t, tpl, embedCutCommand(t, "Noto Sans Thai", 0, "bold", shared))

	// The SAME bytes, now picked as a family's own Regular under a new name.
	fontChainAccepted(t, tpl, embedCommand(t, "Noto Sans", shared, `["Noto Sans SC"]`))

	chain, ok := tpl.doc.Fonts["Noto Sans"]
	if !ok {
		t.Fatal("the pick created no chain at all: the dedupe answered \"already in the document\" about bytes that are only some OTHER entry's cut")
	}
	if len(chain) == 0 || chain[0].AssetKey != embeddedKeyOf(shared) {
		t.Fatalf("the pick's chain does not name the picked face: %#v", chain)
	}
}

// TestTheTwoAssetPredicatesDisagreeExactlyWhereTheyShould states the split as a
// property rather than leaving it to the two behavioural tests above. An
// assertion whose two sides could be equal is not an assertion (D-11.2.8), so
// both directions are measured: they agree on a base key and disagree on a
// variant one.
func TestTheTwoAssetPredicatesDisagreeExactlyWhereTheyShould(t *testing.T) {
	tpl, _, baseKey := embeddedChainTemplate(t)
	bold := testShippedNotoSans
	boldKey := embeddedKeyOf(bold)
	fontChainAccepted(t, tpl, embedCutCommand(t, "Noto Sans Thai", 0, "bold", bold))

	if !assetKeyReferenced(tpl, baseKey) || !embeddedBaseKeyReferenced(tpl, baseKey) {
		t.Error("the two predicates disagree about a BASE key, which both must see")
	}
	if !assetKeyReferenced(tpl, boldKey) {
		t.Error("the SAFETY walk cannot see a declared cut, so the orphan drop would delete a live face")
	}
	if embeddedBaseKeyReferenced(tpl, boldKey) {
		t.Error("the DEDUPE predicate sees a declared cut, so a pick over those bytes would silently create no chain")
	}
}

// TestAnEntryPathKeepsItsIndexWhenTheNameIsTooLong is the claim
// fontChainEntryPath's doc comment makes, measured rather than asserted in
// prose: the NAME is what gets cut, never the index. A path truncated to
// `fonts.<a very long name>` would silently become the CHAIN-level path, and
// the author would be told a chain has a problem when one entry of it does.
//
// ⚠ IT IS NOT WRITTEN THROUGH THE FUNCTION'S OWN ARITHMETIC. The properties
// asserted are structural — ends in the index, fits the host's bound, still
// valid UTF-8 — so re-deriving the expected string from the helper would make
// every one of them vacuous.
func TestAnEntryPathKeepsItsIndexWhenTheNameIsTooLong(t *testing.T) {
	for _, row := range []struct {
		name string
		face string
	}{
		{"over-long ASCII", strings.Repeat("x", maxComponentDataPathBytes*2)},
		// A MULTI-BYTE NAME, because the cut is made in BYTES and a naive
		// slice would split a rune. Thai is the script this product exists
		// for, and every one of these code points is three bytes.
		{"over-long Thai", strings.Repeat("ก", maxComponentDataPathBytes)},
	} {
		t.Run(row.name, func(t *testing.T) {
			path := fontChainEntryPath(row.face, 7)
			if !strings.HasSuffix(path, "[7]") {
				t.Errorf("the index was truncated away: %q ends %q, want [7] — a path that loses its index silently becomes the chain-level path", path, path[max(0, len(path)-8):])
			}
			if len(path) > maxComponentDataPathBytes {
				t.Errorf("the path is %d bytes, past the host's %d-byte DataPath cut — the host would cut it again, and its cut does not know about the index", len(path), maxComponentDataPathBytes)
			}
			if !utf8.ValidString(path) {
				t.Errorf("the path is not valid UTF-8: %q — the cut split a rune", path)
			}
			if !strings.HasPrefix(path, "fonts.") {
				t.Errorf("the path lost its prefix: %q", path)
			}
		})
	}
	// NON-VACUITY IN BOTH DIRECTIONS: a name that FITS is not truncated at all,
	// so the assertions above are measuring the truncating arm and not an
	// arm that never runs.
	if path := fontChainEntryPath("body", 0); path != "fonts.body[0]" {
		t.Errorf("a short name was disturbed: %q", path)
	}
}
