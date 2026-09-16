package folio8

// STORY 12.3 — the three table properties the engine has always rendered and
// no command could write: headerHeight, altRowBackground and the headerStyle
// block. One test per row of the story's own I/O matrix, plus the writer
// census that keeps the claim about WHO authors headerStyle true.
//
// This file is declared in table_behaviour_suite_test.go's
// declaredTableBehaviourSuite: it is named `table_*_test.go`, and that guard
// compares its literal list against `filepath.Glob("table_*_test.go")` in BOTH
// directions, so a new file of that name reds the guard on the commit that
// adds it unless the list moves with it.

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio8-go/internal/designer"
	"github.com/panitw/folio8/folio8-go/internal/template"
)

// theWorkedExampleTable is the table this file edits: `e2` in the content band
// of the worked example, which already declares `style.fontFamily: body` and
// `style.fontSize: 8` and NO headerStyle at all. That combination is what makes
// the fallback observable — a cleared header field has somewhere real to fall
// through to.
const theWorkedExampleTable = "e2"

func headerStyleFixture(t *testing.T) *Template {
	t.Helper()
	tpl := componentTemplate(t)
	view, err := tableColumns(tpl, theWorkedExampleTable)
	if err != nil {
		t.Fatalf("fixture precondition: the worked example must project %s as a table: %v", theWorkedExampleTable, err)
	}
	if view.HeaderFontSize != 0 || view.HeaderFontSizeResolved != 8000 || view.HeaderFontFamilyResolved != "body" {
		t.Fatalf("fixture precondition: %s must declare no headerStyle and a table style of 8pt body, got %#v", theWorkedExampleTable, view)
	}
	return tpl
}

func applyToTable(t *testing.T, tpl *Template, command string) error {
	t.Helper()
	_, err := applyComponentCommand(tpl, []byte(command))
	return err
}

func mustApplyToTable(t *testing.T, tpl *Template, command string) {
	t.Helper()
	if err := applyToTable(t, tpl, command); err != nil {
		t.Fatalf("apply %s: %v", command, err)
	}
}

func canonicalBytes(t *testing.T, tpl *Template) []byte {
	t.Helper()
	encoded, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	return encoded
}

func projectTable(t *testing.T, tpl *Template) designer.TableColumnsProjection {
	t.Helper()
	view, err := tableColumns(tpl, theWorkedExampleTable)
	if err != nil {
		t.Fatalf("project %s: %v", theWorkedExampleTable, err)
	}
	return view
}

// refusalLeavesTheDocumentAlone is the shape every refusal row shares: the
// command errors, the error is LOCATED (an element id and a data path, not a
// bare sentence), and the canonical bytes are byte-identical afterwards.
func refusalLeavesTheDocumentAlone(t *testing.T, tpl *Template, command, wantPath string) {
	t.Helper()
	before := canonicalBytes(t, tpl)
	err := applyToTable(t, tpl, command)
	if err == nil {
		t.Fatalf("apply %s: expected a refusal", command)
	}
	located, ok := err.(*designer.ComponentCommandError)
	if !ok {
		t.Fatalf("apply %s: error is %T, not a located *ComponentCommandError — an unlocated refusal reaches the author as ENGINE_REJECTED with no field to look at", command, err)
	}
	if located.DataPath != wantPath {
		t.Errorf("apply %s: refusal is located at %q, want %q", command, located.DataPath, wantPath)
	}
	if after := canonicalBytes(t, tpl); !bytes.Equal(before, after) {
		t.Errorf("apply %s: a refused command changed the canonical bytes", command)
	}
}

// refusalSaysWhy is refusalLeavesTheDocumentAlone plus the REASON.
//
// A refusal whose MESSAGE is unasserted can be deleted outright and replaced by
// an unrelated fallback that happens to share its DataPath, with every test
// still green. That is not hypothetical: deleting setTableHeaderHeight's `op`
// guard at this tree left both of its tests passing, because the command then
// fell through to "headerHeight must be a positive length" — a different
// refusal, for a different reason, located at the same table.headerHeight.
// Pinning the sentence is what makes the guard falsifiable.
func refusalSaysWhy(t *testing.T, tpl *Template, command, wantPath, wantMessage string) {
	t.Helper()
	refusalLeavesTheDocumentAlone(t, tpl, command, wantPath)
	located, ok := applyToTable(t, tpl, command).(*designer.ComponentCommandError)
	if !ok {
		t.Fatalf("apply %s: expected a located refusal", command)
	}
	if located.Message != wantMessage {
		t.Errorf("apply %s: refused with %q, want %q — the guard this story added must be the one that fired, not a fallback that shares its DataPath", command, located.Message, wantMessage)
	}
}

// ---------------------------------------------------------------------------
// The I/O matrix, one test per row.
// ---------------------------------------------------------------------------

func TestSetHeaderStyleFieldWritesItAndProjectsBothMembers(t *testing.T) {
	tpl := headerStyleFixture(t)
	mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"`+theWorkedExampleTable+`","field":"fontSize","op":"set","value":14}`)
	view := projectTable(t, tpl)
	if view.HeaderFontSize != 14000 || view.HeaderFontSizeResolved != 14000 {
		t.Errorf("committed/resolved fontSize = %d/%d, want 14000/14000", view.HeaderFontSize, view.HeaderFontSizeResolved)
	}
	if !bytes.Contains(canonicalBytes(t, tpl), []byte(`"headerStyle"`)) {
		t.Error("the document does not carry a headerStyle key after one was authored")
	}
}

// THE ROW THIS WHOLE STORY TURNS ON. A cleared field's RESOLVED member must
// read the table's own style — 8pt here, from the fixture — and it must do so
// because the ENGINE cascaded it, not because anything recomputed it. The
// committed member goes back to absent in the same breath, which is what keeps
// "clearable back to absent" meaningful.
func TestClearHeaderStyleFieldFallsBackToTheTablesOwnStyle(t *testing.T) {
	tpl := headerStyleFixture(t)
	mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"`+theWorkedExampleTable+`","field":"fontSize","op":"set","value":14}`)
	mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"`+theWorkedExampleTable+`","field":"background","op":"set","value":"#101010"}`)
	mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"`+theWorkedExampleTable+`","field":"fontSize","op":"clear"}`)
	view := projectTable(t, tpl)
	if view.HeaderFontSize != 0 {
		t.Errorf("committed fontSize = %d after a clear, want 0 (absent)", view.HeaderFontSize)
	}
	if view.HeaderFontSizeResolved != 8000 {
		t.Errorf("resolved fontSize = %d, want the table's own style.fontSize of 8000", view.HeaderFontSizeResolved)
	}
	encoded := canonicalBytes(t, tpl)
	if bytes.Contains(encoded, []byte(`"fontSize": 14`)) {
		t.Error("the cleared fontSize is still in the canonical bytes")
	}
	// The zero Presence removes the KEY. An explicit null would still be a key
	// in the file — different bytes, an undo entry, and a raised format version.
	if bytes.Contains(encoded, []byte(`"fontSize": null`)) {
		t.Error("clearing wrote an explicit null rather than removing the key")
	}
	// And the sibling that was NOT cleared is untouched, so this is a clear of
	// one field rather than of the block.
	if view.HeaderBackground != "#101010" {
		t.Errorf("committed background = %q, want the sibling field left alone", view.HeaderBackground)
	}
}

// AND WITH NO TABLE STYLE TO FALL BACK TO, the resolved member is the format's
// documented default rather than nothing.
func TestClearedHeaderStyleFieldFallsBackToTheDocumentedDefault(t *testing.T) {
	tpl := headerStyleFixture(t)
	mustApplyToTable(t, tpl, `{"kind":"updateComponentProperties","version":1,"ids":["`+theWorkedExampleTable+`"],"changes":{"fontSize":{"op":"clear"}}}`)
	mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"`+theWorkedExampleTable+`","field":"fontSize","op":"set","value":14}`)
	mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"`+theWorkedExampleTable+`","field":"fontSize","op":"clear"}`)
	if got := projectTable(t, tpl).HeaderFontSizeResolved; got != int64(defaultFontSizePt) {
		t.Errorf("resolved fontSize with no table style = %d, want the engine's default %d", got, defaultFontSizePt)
	}
}

func TestClearingTheLastHeaderStyleFieldRemovesTheBlock(t *testing.T) {
	tpl := headerStyleFixture(t)
	mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"`+theWorkedExampleTable+`","field":"align","op":"set","value":"center"}`)
	if !bytes.Contains(canonicalBytes(t, tpl), []byte(`"headerStyle"`)) {
		t.Fatal("precondition: the block must exist before the clear that removes it")
	}
	mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"`+theWorkedExampleTable+`","field":"align","op":"clear"}`)
	encoded := canonicalBytes(t, tpl)
	if bytes.Contains(encoded, []byte(`"headerStyle"`)) {
		t.Errorf("clearing the last field left a headerStyle key behind:\n%s", encoded)
	}
	// An empty object is the failure this row exists for, and it is a different
	// failure from a leftover key with a value in it.
	if bytes.Contains(encoded, []byte(`"headerStyle": {}`)) {
		t.Error("clearing the last field left an empty headerStyle object")
	}
}

func TestSetAndClearTheAlternatingRowBackground(t *testing.T) {
	tpl := headerStyleFixture(t)
	mustApplyToTable(t, tpl, `{"kind":"setTableAltRowBackground","version":1,"id":"`+theWorkedExampleTable+`","op":"set","value":"#DDEEFF"}`)
	if got := projectTable(t, tpl).AltRowBackground; got != "#DDEEFF" {
		t.Errorf("altRowBackground = %q, want #DDEEFF", got)
	}
	if !bytes.Contains(canonicalBytes(t, tpl), []byte(`"altRowBackground": "#DDEEFF"`)) {
		t.Error("altRowBackground did not reach the canonical bytes")
	}
	mustApplyToTable(t, tpl, `{"kind":"setTableAltRowBackground","version":1,"id":"`+theWorkedExampleTable+`","op":"clear"}`)
	if got := projectTable(t, tpl).AltRowBackground; got != "" {
		t.Errorf("altRowBackground = %q after a clear, want absent", got)
	}
	if encoded := canonicalBytes(t, tpl); bytes.Contains(encoded, []byte(`"altRowBackground"`)) {
		t.Errorf("clearing left the key in the document:\n%s", encoded)
	}
}

func TestSetTheHeaderHeightInMillipoints(t *testing.T) {
	tpl := headerStyleFixture(t)
	mustApplyToTable(t, tpl, `{"kind":"setTableHeaderHeight","version":1,"id":"`+theWorkedExampleTable+`","height":18}`)
	if got := projectTable(t, tpl).HeaderHeight; got != 18000 {
		t.Errorf("headerHeight = %d, want 18000 millipoints", got)
	}
	if !bytes.Contains(canonicalBytes(t, tpl), []byte(`"headerHeight": 18`)) {
		t.Error("headerHeight did not reach the canonical bytes in points")
	}
}

// headerHeight IS REQUIRED, so a clear is refused rather than honoured. No
// clear affordance is rendered anywhere, which makes this reachable only from a
// hand-built command — and that is exactly why the engine has to refuse it: the
// document that came back would not load.
func TestTheHeaderHeightCannotBeCleared(t *testing.T) {
	tpl := headerStyleFixture(t)
	refusalSaysWhy(t, tpl, `{"kind":"setTableHeaderHeight","version":1,"id":"`+theWorkedExampleTable+`","op":"clear"}`, "table.headerHeight", "headerHeight is required: it accepts neither a clear nor a null")
}

// A TABLE'S PROJECTED HEIGHT IS ITS headerHeight (projectedSize), so growing it
// can push the table out of a band that caps vertically. The check is
// containComponent — the same predicate updateTableColumn's width case asks,
// one new call site.
func TestAHeaderHeightThatOverflowsItsBandIsRefused(t *testing.T) {
	tpl := componentTemplate(t)
	before, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	created, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"table","band":"pageHeader","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatalf("create a table in the page header: %v", err)
	}
	table := newProjectedComponent(t, before, created)
	// The page header is 60pt tall in this fixture, so 40pt fits and 100pt
	// cannot. Both arms, so the refusal is not simply "every height is refused".
	if err := applyToTable(t, tpl, `{"kind":"setTableHeaderHeight","version":1,"id":"`+table.ID+`","height":40}`); err != nil {
		t.Fatalf("a header height that fits its band must be accepted: %v", err)
	}
	encoded := canonicalBytes(t, tpl)
	err = applyToTable(t, tpl, `{"kind":"setTableHeaderHeight","version":1,"id":"`+table.ID+`","height":100}`)
	located, ok := err.(*designer.ComponentCommandError)
	if !ok {
		t.Fatalf("a header height taller than its band produced %T, want a located refusal", err)
	}
	if located.DataPath != "table.headerHeight" || located.ElementID != table.ID {
		t.Errorf("refusal located at %q/%q, want table.headerHeight on %s", located.ElementID, located.DataPath, table.ID)
	}
	if after := canonicalBytes(t, tpl); !bytes.Equal(encoded, after) {
		t.Error("a refused header height changed the canonical bytes")
	}
}

func TestAMalformedAlternatingRowColourIsRefused(t *testing.T) {
	tpl := headerStyleFixture(t)
	refusalLeavesTheDocumentAlone(t, tpl, `{"kind":"setTableAltRowBackground","version":1,"id":"`+theWorkedExampleTable+`","op":"set","value":"not-a-colour"}`, "table.altRowBackground")
}

func TestAnUnknownHeaderStyleFieldIsRefused(t *testing.T) {
	tpl := headerStyleFixture(t)
	// `padding` and a bare `border` are refused by the SAME gate as a nonsense
	// name: the arm's closed set is the only door, and neither is in it.
	//
	// ⚠ `bold` and `italic` LEFT THIS LIST AT STORY 11.2 and are in the
	// closed set now — resolveHeaderStyle cascades both, so a header style
	// declaring either is read by something that draws (FR57, AC2). They are
	// asserted below instead, where a NON-BOOLEAN value for them is refused
	// at the field itself rather than at the gate.
	//
	// ⚠ A BARE `border` STAYS ON THIS LIST AT STORY 14.8, AND THAT IS THE
	// SHAPE DECISION MADE OBSERVABLE. The story made the header border
	// authorable as three FLAT DOTTED attributes — `border.width`,
	// `border.color`, `border.edges` — and deliberately NOT as one member
	// carrying an object, because a `{id, field, op, value}` command is one
	// field and one op, so a block `set` could only be built by re-sending the
	// two attributes the author never touched. `border` naming the whole block
	// is therefore still not a thing a command can say, and this row is what
	// keeps that true.
	//
	// THE NEAR MISSES ARE HERE FOR THE SAME REASON. `borderWidth` is the
	// element-level command's camelCase spelling, which would produce the path
	// `table.headerStyle.borderWidth` — a key no document has, which is DW-333;
	// `border.notAKey` and `padding.top` are the dotted form pointed at
	// something that is not in the set. All three are refused at the gate, so
	// the dot is not a wildcard.
	for _, field := range []string{"paddingTop", "border", "padding", "notAField", "borderWidth", "borderColor", "borderEdges", "border.notAKey", "border.", "padding.top"} {
		refusalLeavesTheDocumentAlone(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"`+theWorkedExampleTable+`","field":"`+field+`","op":"set","value":"x"}`, "table.headerStyle")
	}
	// The two booleans take their own arm, so `"x"` is refused by the VALUE
	// check and locates at the field — a different door from the gate above,
	// and asserting it is what keeps the two doors distinguishable.
	for _, field := range []string{"bold", "italic"} {
		refusalLeavesTheDocumentAlone(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"`+theWorkedExampleTable+`","field":"`+field+`","op":"set","value":"x"}`, "table.headerStyle."+field)
	}
	// And the twelve that ARE in it are all reachable, so the closed set is not
	// simply refusing everything.
	for _, field := range tableHeaderStyleFields {
		if err := applyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"`+theWorkedExampleTable+`","field":"`+field+`","op":"clear"}`); err != nil {
			t.Errorf("clearing the declared field %q was refused: %v", field, err)
		}
	}
}

// TestTheHeaderStyleBooleanArmActuallyStoresWhatItWasGiven is P4: the
// two booleans Story 11.2 added to the closed set take their OWN arm
// above the string default, and nothing observed what that arm WROTE.
// Measured by mutation: inverting the stored value (`Value: !flag`) left
// the whole suite green — every other assertion about them checks only
// that the field is reachable or that a bad value is refused.
//
// It reads the value back out of the RE-PARSED CANONICAL BYTES rather
// than off the in-memory template, so the assertion covers the command,
// the serializer and the loader in one statement — and `false` is
// asserted as well as `true`, because an arm that stored a constant
// would satisfy either one alone.
func TestTheHeaderStyleBooleanArmActuallyStoresWhatItWasGiven(t *testing.T) {
	for _, tc := range []struct {
		field string
		value string
		read  func(template.Style) template.Presence[bool]
	}{
		{"bold", "true", func(st template.Style) template.Presence[bool] { return st.Bold }},
		{"bold", "false", func(st template.Style) template.Presence[bool] { return st.Bold }},
		{"italic", "true", func(st template.Style) template.Presence[bool] { return st.Italic }},
		{"italic", "false", func(st template.Style) template.Presence[bool] { return st.Italic }},
	} {
		t.Run(tc.field+"="+tc.value, func(t *testing.T) {
			tpl := headerStyleFixture(t)
			mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"`+theWorkedExampleTable+`","field":"`+tc.field+`","op":"set","value":`+tc.value+`}`)
			reloaded, err := ParseTemplate(canonicalBytes(t, tpl))
			if err != nil {
				t.Fatalf("the command produced bytes that do not load: %v", err)
			}
			var found bool
			for _, el := range reloaded.doc.Bands.Content.Elements {
				if string(el.ID) != theWorkedExampleTable {
					continue
				}
				found = true
				hs := el.Table.Value.HeaderStyle
				if !hs.Set || hs.Null {
					t.Fatalf("the command left no headerStyle block to carry %s", tc.field)
				}
				got := tc.read(hs.Value)
				if !got.Set || got.Null {
					t.Fatalf("headerStyle.%s did not survive the round trip: %+v", tc.field, got)
				}
				if want := tc.value == "true"; got.Value != want {
					t.Fatalf("headerStyle.%s round-tripped as %v, want %v — the command's own arm is storing something other than what it was given", tc.field, got.Value, want)
				}
			}
			if !found {
				t.Fatalf("fixture precondition: element %s is not in the content band, so nothing above was asserted", theWorkedExampleTable)
			}
		})
	}
}

func TestNullIsRefusedByAllThreeArms(t *testing.T) {
	tpl := headerStyleFixture(t)
	refusalLeavesTheDocumentAlone(t, tpl, `{"kind":"setTableAltRowBackground","version":1,"id":"`+theWorkedExampleTable+`","op":"null"}`, "table.altRowBackground")
	refusalLeavesTheDocumentAlone(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"`+theWorkedExampleTable+`","field":"color","op":"null"}`, "table.headerStyle.color")
	refusalSaysWhy(t, tpl, `{"kind":"setTableHeaderHeight","version":1,"id":"`+theWorkedExampleTable+`","op":"null"}`, "table.headerHeight", "headerHeight is required: it accepts neither a clear nor a null")
}

// AND THE GATE THE SEVEN COLUMN ARMS SHARE, repeated here because these three
// repeat it: not found, and not a table.
func TestTheThreeArmsShareTheTableGate(t *testing.T) {
	tpl := headerStyleFixture(t)
	for _, command := range []string{
		`{"kind":"setTableHeaderHeight","version":1,"id":"nope","height":18}`,
		`{"kind":"setTableAltRowBackground","version":1,"id":"nope","op":"clear"}`,
		`{"kind":"updateTableHeaderStyle","version":1,"id":"nope","field":"align","op":"clear"}`,
	} {
		refusalLeavesTheDocumentAlone(t, tpl, command, "table.id")
	}
	// AND A MALFORMED COMMAND AGAINST A MISSING ID IS STILL "table.id".
	//
	// Well-formed ops alone could not see this: the three arms reached the
	// shared gate at DIFFERENT points, so `{"id":"nope","op":"bogus"}` was
	// refused at table.altRowBackground — a refusal naming a field on an
	// element that does not exist, pointing the author at the wrong thing. The
	// ordering is now the same in all three arms, and this row is what holds it
	// there.
	for _, command := range []string{
		`{"kind":"setTableHeaderHeight","version":1,"id":"nope","op":"bogus"}`,
		`{"kind":"setTableAltRowBackground","version":1,"id":"nope","op":"bogus"}`,
		`{"kind":"updateTableHeaderStyle","version":1,"id":"nope","field":"align","op":"bogus"}`,
		// A field name outside the seven, on an element that is not there:
		// still the element, not the field.
		`{"kind":"updateTableHeaderStyle","version":1,"id":"nope","field":"notAField","op":"clear"}`,
		// And a surplus key, on an element that is not there.
		`{"kind":"setTableHeaderHeight","version":1,"id":"nope","height":18,"surplus":1}`,
	} {
		refusalLeavesTheDocumentAlone(t, tpl, command, "table.id")
	}
	// A component that exists and is not a table takes the second sentence.
	before, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	created, err := applyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"rect","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	rect := newProjectedComponent(t, before, created)
	refusalLeavesTheDocumentAlone(t, tpl, `{"kind":"setTableHeaderHeight","version":1,"id":"`+rect.ID+`","height":18}`, "table.id")
}

// A DOCUMENT NOBODY EDITED SERIALIZES TO THE SAME BYTES. This story adds no
// key, removes none, and raises no version, so opening the editor on a table
// and touching nothing must be free.
func TestATableWhoseHeaderIsNotEditedSerializesIdentically(t *testing.T) {
	tpl := headerStyleFixture(t)
	before := canonicalBytes(t, tpl)
	if _, err := tableColumns(tpl, theWorkedExampleTable); err != nil {
		t.Fatal(err)
	}
	if after := canonicalBytes(t, tpl); !bytes.Equal(before, after) {
		t.Error("projecting a table for the editor changed the document")
	}
	// And the fixture on disk is unchanged by the round trip, which is what
	// "no corpus digest moves" means for this one document.
	source, err := os.ReadFile(filepath.Join("testdata", "template", "golden", "worked-example.json"))
	if err != nil {
		t.Fatal(err)
	}
	reparsed, err := ParseTemplate(source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonicalBytes(t, reparsed), before) {
		t.Error("the fixture no longer round-trips to the same bytes")
	}
}

// RE-SETTING A VALUE TO WHAT IT ALREADY IS IS A SILENT SUCCESS, not an error.
// The canonical-bytes short-circuit that turns it into "no revision, no undo
// entry" lives one layer up in folio8-go/internal/wasm/engine.go's Apply, which runs it BEFORE
// pushUndo and install; what is asserted here is the half this layer owns —
// the command is accepted and the bytes do not move.
func TestReSettingAHeaderStyleFieldToItsCurrentValueIsASilentNoOp(t *testing.T) {
	tpl := headerStyleFixture(t)
	mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"`+theWorkedExampleTable+`","field":"fontSize","op":"set","value":14}`)
	settled := canonicalBytes(t, tpl)
	if err := applyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"`+theWorkedExampleTable+`","field":"fontSize","op":"set","value":14}`); err != nil {
		t.Fatalf("re-setting a value to itself must be a silent success, got %v", err)
	}
	if after := canonicalBytes(t, tpl); !bytes.Equal(settled, after) {
		t.Error("re-setting a value to itself moved the bytes")
	}
	// Clearing an already-absent field is the same shape from the other side.
	mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"`+theWorkedExampleTable+`","field":"color","op":"clear"}`)
	if after := canonicalBytes(t, tpl); !bytes.Equal(settled, after) {
		t.Error("clearing an already-absent field moved the bytes")
	}
}

// EVERY FIELD VALIDATES THROUGH THE SAME PREDICATE THE LOADER ASKS. The
// consequence of getting this wrong is not a bad message: a command door that
// admitted what the file door refuses would stamp out a document the designer
// cannot reopen.
func TestHeaderStyleValuesAreJudgedByTheLoadersOwnPredicates(t *testing.T) {
	tpl := headerStyleFixture(t)
	for _, refused := range []struct{ field, value, path string }{
		{"align", `"justify"`, "table.headerStyle.align"},
		{"valign", `"centre"`, "table.headerStyle.valign"},
		{"color", `"#GGGGGG"`, "table.headerStyle.color"},
		{"background", `"rebeccapurple"`, "table.headerStyle.background"},
		{"fontFamily", `"no-such-chain"`, "table.headerStyle.fontFamily"},
		{"fontSize", `0`, "table.headerStyle.fontSize"},
		{"fontSize", `-3`, "table.headerStyle.fontSize"},
		{"lineSpacing", `"1.5"`, "table.headerStyle.lineSpacing"},
	} {
		refusalLeavesTheDocumentAlone(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"`+theWorkedExampleTable+`","field":"`+refused.field+`","op":"set","value":`+refused.value+`}`, refused.path)
	}
	// The accepting arm for each, so the refusals above are about the VALUE and
	// not about the field.
	for _, accepted := range []struct{ field, value string }{
		{"align", `"center"`},
		{"valign", `"middle"`},
		{"color", `"#112233"`},
		{"background", `"#445566"`},
		{"fontFamily", `"body"`},
		{"fontSize", `14`},
		{"lineSpacing", `1.5`},
	} {
		if err := applyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"`+theWorkedExampleTable+`","field":"`+accepted.field+`","op":"set","value":`+accepted.value+`}`); err != nil {
			t.Errorf("a legal %s value was refused: %v", accepted.field, err)
		}
	}
	view := projectTable(t, tpl)
	if view.HeaderAlign != "center" || view.HeaderValign != "middle" || view.HeaderColor != "#112233" || view.HeaderBackground != "#445566" || view.HeaderFontFamily != "body" || view.HeaderFontSize != 14000 || view.HeaderLineSpacing != 1500 {
		t.Errorf("the seven accepted values did not all land: %#v", view)
	}
	// And the resolved twin of a SET field is that field: the cascade prefers
	// headerStyle over the table's own style, which is the whole point of it.
	if view.HeaderAlignResolved != "center" || view.HeaderFontSizeResolved != 14000 {
		t.Errorf("resolved members do not follow a committed one: %#v", view)
	}
}

// ---------------------------------------------------------------------------
// The writer census.
// ---------------------------------------------------------------------------

// declaredHeaderStyleAssigners is every non-test function in the module ROOT
// (package folio8) that ASSIGNS to a HeaderStyle-rooted expression, or takes a
// writable pointer into one.
//
// THE PHRASING MATTERS AND BOTH OBVIOUS PHRASINGS ARE FALSE. "Nothing writes
// headerStyle" was already false before this story: renameFontChain rewrites
// HeaderStyle.FontFamily when a font chain is renamed, and it is the ONE such
// site — verified at D-12.3.0's own tree with `git show 71627a5`, where the
// ruling's "two sites" was an over-count when it was written rather than drift
// since. And "two writers" is false in the other direction now that 12.3 exists.
// What is true, and what this list says, is that the set is CLOSED and every
// member is named.
var declaredHeaderStyleAssigners = []string{
	"cleanupEmptyHeaderStyle", // 12.3 — drops a block whose last field was cleared
	"headerStyleFor",          // 12.3 — materialises the block and hands out the pointer
	"renameFontChain",         // 8.1 — rewrites HeaderStyle.FontFamily when a chain is renamed
}

// declaredHeaderStyleCommands is the far narrower claim the story actually
// makes: exactly ONE command AUTHORS the header style, and it is the arm added
// for it. renameFontChain is not in this list because it authors nothing — it
// follows a rename the author made somewhere else.
var declaredHeaderStyleCommands = []string{"updateTableHeaderStyle"}

func TestOnlyTheDeclaredSitesWriteAHeaderStyle(t *testing.T) {
	assigners, callers := map[string]bool{}, map[string]bool{}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	scanned := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		scanned++
		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, name, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				switch n := node.(type) {
				case *ast.AssignStmt:
					for _, lhs := range n.Lhs {
						if namesHeaderStyle(lhs) {
							assigners[fn.Name.Name] = true
						}
					}
				case *ast.UnaryExpr:
					if n.Op == token.AND && namesHeaderStyle(n.X) {
						assigners[fn.Name.Name] = true
					}
				case *ast.CallExpr:
					if ident, ok := n.Fun.(*ast.Ident); ok && ident.Name == "headerStyleFor" {
						callers[fn.Name.Name] = true
					}
				}
				return true
			})
		}
	}
	if scanned == 0 {
		t.Fatal("vacuity guard: the census parsed ZERO non-test files in the module root, so it proves nothing")
	}
	if got := sortedFunctionNames(assigners); !reflect.DeepEqual(got, declaredHeaderStyleAssigners) {
		t.Errorf("the functions assigning to a HeaderStyle are\n\t%v\nand the declared set is\n\t%v — a new writer is a new place a header style can change, and this list is where that becomes a reviewed line rather than a silent one", got, declaredHeaderStyleAssigners)
	}
	// headerStyleFor is the only function that hands out a writable pointer into
	// the block, so its caller set IS the set of commands that author one.
	if got := sortedFunctionNames(callers); !reflect.DeepEqual(got, declaredHeaderStyleCommands) {
		t.Errorf("the functions taking a writable header style are\n\t%v\nand the declared set is\n\t%v", got, declaredHeaderStyleCommands)
	}
	// Non-vacuity: the matcher finds a write in a synthetic source, so an empty
	// result above would have been a finding rather than a pass.
	if !matcherFindsAHeaderStyleWrite(t, "package p\nfunc w(e *E) { e.Table.Value.HeaderStyle = x }\n") {
		t.Error("the census matcher does not recognise a plain HeaderStyle assignment")
	}
	if matcherFindsAHeaderStyleWrite(t, "package p\nfunc r(e *E) bool { return e.Table.Value.HeaderStyle.Set }\n") {
		t.Error("the census matcher counts a READ as a write, which would make the declared set meaningless")
	}
}

// namesHeaderStyle reports whether expr is rooted at a `.HeaderStyle` selector
// — `x.HeaderStyle`, `x.HeaderStyle.Value`, `x.HeaderStyle.Value.FontFamily`
// and so on all count, because every one of them writes into the block.
func namesHeaderStyle(expr ast.Expr) bool {
	for {
		sel, ok := expr.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		if sel.Sel.Name == "HeaderStyle" {
			return true
		}
		expr = sel.X
	}
}

func matcherFindsAHeaderStyleWrite(t *testing.T, source string) bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "synthetic.go", source, 0)
	if err != nil {
		t.Fatalf("parse the synthetic source: %v", err)
	}
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		if assign, ok := node.(*ast.AssignStmt); ok {
			for _, lhs := range assign.Lhs {
				if namesHeaderStyle(lhs) {
					found = true
				}
			}
		}
		return true
	})
	return found
}

func sortedFunctionNames(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	slices.Sort(out)
	return out
}

// ---------------------------------------------------------------------------
// REVIEW PATCH SET (2026-09-05). The four findings below are all the same
// shape: a claim the story makes that nothing in the tree could falsify.
// ---------------------------------------------------------------------------

// headerCascadeDocument is a hand-authored table that declares exactly the two
// style blocks a row asks it to and nothing else, so a resolved member can be
// watched moving from level to level. It is hand-authored rather than driven
// off the worked example because two of the things these rows need — a SECOND
// font chain, and a headerStyle carrying a field no command can author — are
// reachable only from the file door.
//
// tableStyle and headerStyle are whole JSON fragments including their trailing
// comma, or "" for "the block is not there at all".
func headerCascadeDocument(t *testing.T, tableStyle, headerStyle string) *Template {
	t.Helper()
	doc := `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "rows[]", "as": "row", "headerHeight": 20,
          ` + tableStyle + headerStyle + `
          "columns": [{"id": "e2", "label": "A", "width": 100, "align": "left", "bind": "{{row.a}}"}]}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Roboto-Regular"], "display": ["Roboto-Bold"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	tpl, err := ParseTemplate([]byte(doc))
	if err != nil {
		t.Fatalf("parse the cascade fixture: %v", err)
	}
	return tpl
}

// THE FOUR RESOLVED MEMBERS NOTHING ASSERTED, AND THE THREE THAT WERE.
//
// Measured before this test existed: HeaderBackgroundResolved, HeaderColorResolved,
// HeaderLineSpacingResolved and HeaderValignResolved appeared in exactly one
// production file and in ZERO `_test.go` files, while HeaderFontSizeResolved,
// HeaderAlignResolved and HeaderFontFamilyResolved each appeared in one. Those
// four carry AC3 — *the editor shows the value the document will actually use,
// because the engine sent it* — and wiring any of them straight to the
// COMMITTED value, or to a constant, left the whole suite green.
//
// Each row walks BOTH legs of the cascade so a member wired to either end is
// caught: put the value on the TABLE's own style and the resolved twin must
// follow it; then put a different value on `headerStyle` and the resolved twin
// must follow THAT instead. A member wired to the committed field fails the
// first leg; one wired to the table's own style fails the second.
func TestEveryResolvedProjectionMemberFollowsBothLegsOfTheCascade(t *testing.T) {
	rows := []struct {
		field       string
		member      string
		tableStyle  string
		headerValue string
		wantTable   string
		wantHeader  string
		read        func(designer.TableColumnsProjection) string
	}{
		{"fontFamily", "HeaderFontFamilyResolved", `"fontFamily": "body"`, `"display"`, "body", "display",
			func(v designer.TableColumnsProjection) string { return v.HeaderFontFamilyResolved }},
		{"fontSize", "HeaderFontSizeResolved", `"fontSize": 9`, `14`, "9000", "14000",
			func(v designer.TableColumnsProjection) string { return strconv.FormatInt(v.HeaderFontSizeResolved, 10) }},
		{"lineSpacing", "HeaderLineSpacingResolved", `"lineSpacing": 1.5`, `2`, "1500", "2000",
			func(v designer.TableColumnsProjection) string {
				return strconv.FormatInt(v.HeaderLineSpacingResolved, 10)
			}},
		// ⚠ wantTable IS "" FOR THE CHROME FOUR (this row and the border
		// trio below), and that is SPEC-table-rules §1 rather than a
		// gap. `background` and `border` are the only members on this
		// cascade with NO `style` leg any more: a table's own
		// `style.background`/`style.border` paint the table's BOX, and
		// `headerStyle`'s are the only declaration that reaches a header
		// cell. So with nothing on headerStyle the resolved twin
		// correctly answers "nothing resolves" — the projection's
		// ordinary spelling of absence — and leg two, which is what the
		// author is actually authoring, is unchanged.
		{"background", "HeaderBackgroundResolved", `"background": "#eeeeee"`, `"#101010"`, "", "#101010",
			func(v designer.TableColumnsProjection) string { return v.HeaderBackgroundResolved }},
		{"color", "HeaderColorResolved", `"color": "#1b2a4a"`, `"#c81e1e"`, "#1b2a4a", "#c81e1e",
			func(v designer.TableColumnsProjection) string { return v.HeaderColorResolved }},
		{"valign", "HeaderValignResolved", `"valign": "middle"`, `"bottom"`, "middle", "bottom",
			func(v designer.TableColumnsProjection) string { return v.HeaderValignResolved }},
		{"align", "HeaderAlignResolved", `"align": "center"`, `"right"`, "center", "right",
			func(v designer.TableColumnsProjection) string { return v.HeaderAlignResolved }},
		// STORY 11.3's TWO. Leg one puts `true` on the table's own style and
		// leg two puts `false` on headerStyle, so the second leg is the one a
		// member wired to `style.bold` cannot pass — the direction matters for
		// a bool, because a leg-two value of `true` would also be produced by
		// a member that simply echoed leg one.
		{"bold", "HeaderBoldResolved", `"bold": true`, `false`, "true", "false",
			func(v designer.TableColumnsProjection) string { return strconv.FormatBool(v.HeaderBoldResolved) }},
		{"italic", "HeaderItalicResolved", `"italic": true`, `false`, "true", "false",
			func(v designer.TableColumnsProjection) string { return strconv.FormatBool(v.HeaderItalicResolved) }},
		// STORY 14.8's THREE, AND THEY WALK A DIFFERENT CASCADE FROM THEIR NINE
		// SIBLINGS — which is exactly why they are worth walking. The nine above
		// fall through FIELD BY FIELD, so leg one's value survives on every
		// field the header does not name. The border does not: resolveHeaderStyle
		// takes `headerStyle.border` WHOLE, so leg two's `headerStyle` border
		// declares the ONE attribute under test and the other two resolve to the
		// FORMAT's defaults rather than to the table's.
		//
		// The leg-one table style therefore declares a full border whose every
		// attribute differs from the format's default (3pt / #445566 / two
		// edges), and leg two's expected value is the attribute the command just
		// wrote. A member wired to the committed field fails leg one; one wired
		// to `style.border` fails leg two, exactly as for the nine.
		{"border.width", "HeaderBorderWidthResolved", `"border": {"width": 3, "color": "#445566", "edges": ["top", "bottom"]}`, `1.5`, "", "1500",
			func(v designer.TableColumnsProjection) string { return v.HeaderBorderWidthResolved }},
		{"border.color", "HeaderBorderColorResolved", `"border": {"width": 3, "color": "#445566", "edges": ["top", "bottom"]}`, `"#c81e1e"`, "", "#c81e1e",
			func(v designer.TableColumnsProjection) string { return v.HeaderBorderColorResolved }},
		// AND THE EDGES ROW IS THE ONE THAT PINS THE CANONICAL ORDER AS WELL AS
		// THE CASCADE: the command sends `["left","top"]` and the projection must
		// answer `top,left`, because the browser's guard admits only the format's
		// own order and Go is the side that fixes it.
		{"border.edges", "HeaderBorderEdgesResolved", `"border": {"width": 3, "color": "#445566", "edges": ["top", "bottom"]}`, `["left", "top"]`, "", "top,left",
			func(v designer.TableColumnsProjection) string { return v.HeaderBorderEdgesResolved }},
	}
	// THE TIE DW-240 WAS MISSING, AND THE DURABLE HALF OF ITS FIX. Nothing
	// related `tableHeaderStyleFields` to this projection's member list, which
	// is exactly why nobody noticed that the command layer accepted nine
	// header-style fields while the projection carried seven pairs: a weight
	// an author could write and could not read back. `rows` above is a
	// hand-maintained list and was itself the second copy of that asymmetry,
	// so it is pinned to the engine's own closed set here rather than counted.
	// A tenth field added to `tableHeaderStyleFields` now reds THIS test until
	// its pair and its row exist.
	fields := make([]string, 0, len(rows))
	for _, row := range rows {
		fields = append(fields, row.field)
	}
	slices.Sort(fields)
	want := slices.Clone(tableHeaderStyleFields)
	slices.Sort(want)
	if !slices.Equal(fields, want) {
		t.Errorf("this test walks the header-style fields\n\t%v\nand the engine's closed set is\n\t%v — a field a command can author with no row here is a field whose resolved projection member nothing follows, which is DW-240 repeating", fields, want)
	}
	for _, row := range rows {
		t.Run(row.member, func(t *testing.T) {
			// LEG ONE: the value lives on the table's own style and there is no
			// headerStyle at all, so the resolved twin must read the table's.
			tpl := headerCascadeDocument(t, `"style": {`+row.tableStyle+`},`, "")
			view, err := tableColumns(tpl, "e1")
			if err != nil {
				t.Fatalf("project the table: %v", err)
			}
			if got := row.read(view); got != row.wantTable {
				t.Errorf("%s = %q with the value on the table's own style, want %q — the resolved member is not reading the engine's cascade", row.member, got, row.wantTable)
			}
			// And the COMMITTED twin beside it is still absent, which is what
			// makes the two members different members rather than one repeated.
			// "false" JOINS "" AND "0" AS A SPELLING OF ABSENT, and that is
			// the disclosed collapse rather than a hole in this check: a bool
			// on a wire whose key set is pinned exactly in both directions has
			// no third value to put an absence in, so committed-absent and
			// committed-`false` are the same member value
			// (TableColumnsProjection's own comment states the limit). Leg two
			// below is where the two booleans are still discriminating, and it
			// sets headerStyle to `false` over a table style of `true`.
			if committed := committedTwin(t, view, row.field); committed != "" && committed != "0" && committed != "false" {
				t.Errorf("committed %s = %q with nothing on headerStyle, want absent", row.field, committed)
			}
			// LEG TWO: headerStyle now declares a DIFFERENT value, so the
			// resolved twin must abandon the table's style for it.
			if _, err := applyComponentCommand(tpl, []byte(`{"kind":"updateTableHeaderStyle","version":1,"id":"e1","field":"`+row.field+`","op":"set","value":`+row.headerValue+`}`)); err != nil {
				t.Fatalf("set headerStyle.%s: %v", row.field, err)
			}
			view, err = tableColumns(tpl, "e1")
			if err != nil {
				t.Fatalf("re-project the table: %v", err)
			}
			if got := row.read(view); got != row.wantHeader {
				t.Errorf("%s = %q with headerStyle.%s declared, want %q — headerStyle must beat the table's own style", row.member, got, row.field, row.wantHeader)
			}
			// AND BACK: clearing it returns the resolved twin to the table's
			// own value, which is the row of the I/O matrix this whole story
			// turns on, asserted here once per member rather than once total.
			if _, err := applyComponentCommand(tpl, []byte(`{"kind":"updateTableHeaderStyle","version":1,"id":"e1","field":"`+row.field+`","op":"clear"}`)); err != nil {
				t.Fatalf("clear headerStyle.%s: %v", row.field, err)
			}
			view, err = tableColumns(tpl, "e1")
			if err != nil {
				t.Fatalf("re-project the table after the clear: %v", err)
			}
			if got := row.read(view); got != row.wantTable {
				t.Errorf("%s = %q after the clear, want the table's own %q back", row.member, got, row.wantTable)
			}
		})
	}
}

// TestTheProjectionCarriesAPairForEveryHeaderStyleFieldACommandCanWrite is the
// OTHER half of DW-240's tie, and the durable one: it reads the projection's
// member list off the STRUCT rather than off a hand-maintained list, so it
// holds even for a field nobody remembered to add a cascade row for.
//
// DW-240 WAS AN ASYMMETRY NOTHING COULD SEE. `tableHeaderStyleFields` (the
// authoring half) went to nine at Story 11.2 while TableColumnsProjection kept
// seven Header*/Header*Resolved pairs, and no test in this repository related
// the two — so a header weight an author could write was a header weight the
// panel could not read back, silently, for a whole story. The two members were
// the cheap half of the fix; this is the half that stops the tenth field
// repeating it.
//
// IT IS READ FROM THE JSON TAGS, NOT THE Go NAMES, because the tags are what
// the browser's exact-key guard pins and therefore what "a member" means on
// this seam. `headerHeight` is the one `header*` key with no `Resolved` twin,
// and that is asserted rather than skipped: the struct's own comment calls the
// singleton deliberate, so the test states which key it is instead of letting
// any unpaired key through.
func TestTheProjectionCarriesAPairForEveryHeaderStyleFieldACommandCanWrite(t *testing.T) {
	tags := map[string]bool{}
	value := reflect.TypeOf(designer.TableColumnsProjection{})
	for i := 0; i < value.NumField(); i++ {
		tag, _, _ := strings.Cut(value.Field(i).Tag.Get("json"), ",")
		if tag != "" {
			tags[tag] = true
		}
	}
	// POSITIVE CONTROL: the reflection actually read something, and read the
	// key set the wire record already pins. A typo'd tag name would otherwise
	// make every absence below vacuous.
	if len(tags) != len(tableColumnsProjectionWireKeys) {
		t.Fatalf("read %d json tags off TableColumnsProjection and the recorded wire set has %d — this test is measuring the wrong struct", len(tags), len(tableColumnsProjectionWireKeys))
	}
	var paired, unpaired []string
	for tag := range tags {
		field, ok := strings.CutPrefix(tag, "header")
		// A tag spelled EXACTLY `header` would take `field[:1]` on an empty
		// string and PANIC — a test that crashes instead of reporting, on the
		// one input this loop cannot name. It is not reachable today and it is
		// guarded rather than argued: the guard costs a line, and "unreachable"
		// is the claim that ages worst in this repository.
		if !ok || field == "" || strings.HasSuffix(tag, "Resolved") {
			continue
		}
		name := strings.ToLower(field[:1]) + field[1:]
		if tags[tag+"Resolved"] {
			paired = append(paired, name)
			continue
		}
		unpaired = append(unpaired, name)
	}
	// BOTH LISTS ARE SORTED, because both are built by ranging a MAP and Go
	// randomises that order deliberately. `paired` was sorted and `unpaired` was
	// not, so a second unpaired key would have made this test's verdict depend
	// on the run — green sometimes, red sometimes, over the same code.
	slices.Sort(paired)
	slices.Sort(unpaired)
	want := slices.Clone(tableHeaderStyleFields)
	slices.Sort(want)
	if !slices.Equal(paired, want) {
		t.Errorf("TableColumnsProjection carries a committed/resolved pair for\n\t%v\nand the fields a command may author are\n\t%v — a field on one side only is a value an author can write and cannot read back (DW-240), or a member the engine sends that no command can produce", paired, want)
	}
	if !slices.Equal(unpaired, []string{"height"}) {
		t.Errorf("the header keys with no resolved twin are %v, want exactly [height] — headerHeight is the one deliberate singleton (it is required by the format, so it is never absent and committed IS resolved); any other unpaired header key is a missing pair", unpaired)
	}
}

// committedTwin reads the COMMITTED member beside a resolved one, as a string,
// so the test above can assert the pair are genuinely two members.
func committedTwin(t *testing.T, view designer.TableColumnsProjection, field string) string {
	t.Helper()
	switch field {
	case "fontFamily":
		return view.HeaderFontFamily
	case "fontSize":
		return strconv.FormatInt(view.HeaderFontSize, 10)
	case "lineSpacing":
		return strconv.FormatInt(view.HeaderLineSpacing, 10)
	case "background":
		return view.HeaderBackground
	case "color":
		return view.HeaderColor
	case "valign":
		return view.HeaderValign
	case "align":
		return view.HeaderAlign
	case "bold":
		return strconv.FormatBool(view.HeaderBold)
	case "italic":
		return strconv.FormatBool(view.HeaderItalic)
	// ⚠ A MISSING ARM HERE DOES NOT RED — the switch used to fall out and
	// return "", which is one of the three spellings of ABSENT the caller
	// accepts, so a new field's leg-one check would have MEASURED NOTHING
	// rather than failed. Vacuity, not failure. The t.Fatalf below is what
	// turns the omission into a report, and these three arms are why it has
	// never fired.
	case "border.width":
		return view.HeaderBorderWidth
	case "border.color":
		return view.HeaderBorderColor
	case "border.edges":
		return view.HeaderBorderEdges
	}
	t.Fatalf("no committed twin for %q", field)
	return ""
}

// cleanupEmptyHeaderStyle's PROTECTION FOR A BLOCK THAT STILL DECLARES
// SOMETHING THE COMMAND WAS NOT ASKED ABOUT.
//
// The predicate consults Border, Padding and Extra as well as the fields the
// panel authors, and nothing tested that. Shortening it left every test green
// while an author who had hand-written a header border lost it the moment they
// touched header alignment — the block would go from "declares align and border"
// to "declares border" to, wrongly, absent.
//
// ⚠ ITS TITLE SAID "the last AUTHORABLE field" AND MEANT "the last field a
// COMMAND CAN WRITE", WHICH STOPPED BEING THE SAME THING AT STORY 14.8. The
// border IS authorable now, one attribute at a time. What this test still pins is
// unchanged and is what matters: a command asked about ALIGNMENT must not delete
// a border it was not asked about. The border here is hand-authored in FULL, so
// the collapse Story 14.8 added to cleanupEmptyHeaderStyle — which drops an
// EMPTY `border: {}` — cannot fire on it, and the two behaviours are
// distinguishable rather than one masking the other.
//
// Precedent for the shape: line_spacing_test.go's
// TestLineSpacingClearedFromTheOnlyStyleFieldStrandsNoStyleBlock, which pins
// the same predicate's other direction on element.Style.
func TestClearingTheLastAuthorableHeaderFieldKeepsAHandAuthoredBorder(t *testing.T) {
	tpl := headerCascadeDocument(t, "", `"headerStyle": {"align": "center", "border": {"edges": ["bottom"], "color": "#112233", "width": 1}},`)
	if got := projectHeaderCascade(t, tpl).HeaderAlign; got != "center" {
		t.Fatalf("precondition: the hand-authored align must project, got %q", got)
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"updateTableHeaderStyle","version":1,"id":"e1","field":"align","op":"clear"}`)); err != nil {
		t.Fatalf("clear headerStyle.align: %v", err)
	}
	encoded, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	// The cleared field is gone.
	if bytes.Contains(encoded, []byte(`"align": "center"`)) {
		t.Errorf("the cleared align survived:\n%s", encoded)
	}
	// AND THE BLOCK IS STILL THERE, because it still declares something.
	if !bytes.Contains(encoded, []byte(`"headerStyle"`)) {
		t.Errorf("clearing the last AUTHORABLE field dropped a headerStyle that still declares a border:\n%s", encoded)
	}
	if !bytes.Contains(encoded, []byte(`"border"`)) {
		t.Errorf("the hand-authored header border was deleted by a command that was asked about alignment:\n%s", encoded)
	}
	// And the document still loads, which is what "survives in the canonical
	// bytes" has to mean for a block the loader will read back.
	if _, err := ParseTemplate(encoded); err != nil {
		t.Fatalf("the surviving document no longer loads: %v", err)
	}
}

// ⚠ AND THE SAME CLAIM AGAINST AN *EMPTY* HAND-AUTHORED BORDER, WHICH THE TEST
// ABOVE CANNOT SEE.
//
// WHY THE GUARD ABOVE MISSED THIS: it hand-authors the border in FULL —
// `{"edges": ["bottom"], "color": "#112233", "width": 1}` — so Story 14.8's
// empty-`border` collapse could never fire on it no matter which field the
// command cleared, and the test stayed green over a real deletion. A FIXTURE
// MORE COMPLETE THAN THE DEFECT'S PRECONDITION IS A GUARD THAT CANNOT SEE IT.
// The next person will reach for the same fully-populated fixture; the border
// here is deliberately EMPTY, which is the only state the collapse can act on.
//
// AND `"border": {}` IS A MEANINGFUL DOCUMENT, NOT DEBRIS. resolveHeaderStyle's
// arm is `case hasHeader && header.Border.Set && !header.Border.Null`, so a
// present-but-empty header border wins the cascade WHOLE and paints the resolved
// 0.5pt black on all four edges; writeBorder emits `{}` for it, so it survives a
// load and a save as a fixed point. This fixture therefore declares NO table
// border, which makes the resolved trio the discriminator: 500/#000000/all-four
// while the empty block stands, and ""/""/"" — nothing painted at all — the
// instant it is deleted. The byte search alone would be weaker; the PDF changing
// is the defect.
//
// UNGATED, this collapse ran on every one of the twelve fields' clears, so this
// command — about BACKGROUND — deleted the border and then the whole headerStyle.
// RED PROOF: drop `strings.HasPrefix(clearedField, "border.")` from
// cleanupEmptyHeaderStyle and this test fails on all three assertions.
func TestClearingANonBorderHeaderFieldKeepsAnEmptyHandAuthoredBorder(t *testing.T) {
	tpl := headerCascadeDocument(t, "", `"headerStyle": {"background": "#445566", "border": {}},`)
	// PRECONDITION: the empty border block already wins the cascade whole, and
	// the table declares none of its own, so every resolved value below is the
	// header's block answering.
	before := projectHeaderCascade(t, tpl)
	if before.HeaderBorderWidthResolved != "500" || before.HeaderBorderColorResolved != "#000000" || before.HeaderBorderEdgesResolved != "top,right,bottom,left" {
		t.Fatalf("precondition: an empty header border must resolve to the renderer's own defaults, got %#v", before)
	}
	// A COMMAND ABOUT BACKGROUND. It names no border attribute at all.
	mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"e1","field":"background","op":"clear"}`)
	encoded := canonicalBytes(t, tpl)
	if bytes.Contains(encoded, []byte(`"background"`)) {
		t.Errorf("the cleared background survived:\n%s", encoded)
	}
	if !bytes.Contains(encoded, []byte(`"border"`)) {
		t.Errorf("a command about background deleted a hand-authored empty header border:\n%s", encoded)
	}
	if !bytes.Contains(encoded, []byte(`"headerStyle"`)) {
		t.Errorf("a command about background dropped a headerStyle that still declares a border:\n%s", encoded)
	}
	// AND THE RENDERED ANSWER IS UNCHANGED, which is the half that makes this a
	// correctness claim rather than a tidiness one.
	if after := projectHeaderCascade(t, tpl); after.HeaderBorderWidthResolved != "500" || after.HeaderBorderColorResolved != "#000000" || after.HeaderBorderEdgesResolved != "top,right,bottom,left" {
		t.Errorf("clearing the background changed what the header row paints: now %q/%q/%q, want the unchanged 500/#000000/top,right,bottom,left", after.HeaderBorderWidthResolved, after.HeaderBorderColorResolved, after.HeaderBorderEdgesResolved)
	}
	if _, err := ParseTemplate(encoded); err != nil {
		t.Fatalf("the surviving document no longer loads: %v", err)
	}
}

func projectHeaderCascade(t *testing.T, tpl *Template) designer.TableColumnsProjection {
	t.Helper()
	view, err := tableColumns(tpl, "e1")
	if err != nil {
		t.Fatalf("project e1: %v", err)
	}
	return view
}

// A CLEAR AGAINST AN EXPLICITLY-NULL headerStyle IS A NO-OP, and a no-op must
// not move the bytes. headerStyleFor replaces the null with a real block and
// cleanupEmptyHeaderStyle then removes the key outright, so `"headerStyle":
// null` became no headerStyle at all — a byte change and an undo entry for a
// command that removed a field which was already absent.
func TestClearingAFieldOfAnExplicitlyNullHeaderStyleMovesNoBytes(t *testing.T) {
	tpl := headerCascadeDocument(t, "", `"headerStyle": null,`)
	before, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if !bytes.Contains(before, []byte(`"headerStyle": null`)) {
		t.Fatalf("precondition: the fixture must carry an explicit null headerStyle:\n%s", before)
	}
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"updateTableHeaderStyle","version":1,"id":"e1","field":"align","op":"clear"}`)); err != nil {
		t.Fatalf("clear headerStyle.align on a null block: %v", err)
	}
	after, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatalf("re-serialize: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("a clear of an already-absent field changed the document:\nbefore\n%s\nafter\n%s", before, after)
	}
}

// setTableHeaderHeight's ARITY REFUSAL IS LOCATED, like every other refusal in
// this change. It returned componentFields' bare fmt.Errorf, which reaches the
// author as ENGINE_REJECTED with no element and no field to look at — the one
// unlocated refusal across the three arms, and the exact thing
// refusalLeavesTheDocumentAlone's own comment warns about.
func TestSetTheHeaderHeightLocatesItsArityRefusal(t *testing.T) {
	tpl := headerStyleFixture(t)
	const want = "setTableHeaderHeight takes exactly kind, version, id and height"
	// A surplus key.
	refusalSaysWhy(t, tpl, `{"kind":"setTableHeaderHeight","version":1,"id":"`+theWorkedExampleTable+`","height":18,"surplus":1}`, "table.headerHeight", want)
	// And a missing one.
	refusalSaysWhy(t, tpl, `{"kind":"setTableHeaderHeight","version":1,"id":"`+theWorkedExampleTable+`"}`, "table.headerHeight", want)
}

// ---------------------------------------------------------------------------
// STORY 14.8 — THE HEADER BORDER'S OWN I/O MATRIX, AT THE WIRE-SHAPE LAYER.
//
// The four rows below are the ones only Go can answer, because each is a claim
// about the BYTES a command leaves behind rather than about what the panel
// shows: one attribute per command, a second attribute leaving the first alone,
// a clear of one, and the clear of the last collapsing two nested blocks. The
// two refusal rows are here for the same reason — a refusal is located or it is
// not, and only the engine knows.
//
// EVERY ONE IS RED-PROVED BY DELETING THE BEHAVIOUR UNDER TEST, never by
// reverting the story: an absence claim ("no colour was written") passes far too
// easily under a revert, because a revert removes the command that would have
// written one.
// ---------------------------------------------------------------------------

// A table whose OWN border is fully declared and whose header declares none.
//
// SPEC-table-rules §1 CHANGED WHAT THE TABLE'S OWN BORDER DOES here, and the
// fixture is kept as it was so the change is visible: the table's border now
// paints the table's BOX and the header inherits NOTHING from it. What the four
// rows below assert is unaffected — they are claims about the BYTES a command
// leaves behind (one attribute per command, a second leaving the first alone, a
// clear of one, the clear of the last collapsing two nested blocks) — and the
// resolved-projection preconditions now read "the header declares and resolves
// no border at all" rather than "the header inherits the table's".
func headerBorderDocument(t *testing.T, headerStyle string) *Template {
	t.Helper()
	return headerCascadeDocument(t, `"style": {"border": {"width": 3, "color": "#445566", "edges": ["top", "bottom"]}},`, headerStyle)
}

// ROW 1 — the FIRST attribute authored. It materialises the block, it writes
// EXACTLY the attribute named, and it seeds neither of the other two. That last
// clause is the whole of "Arm C" at the storage layer: a colour or an edge list
// appearing in the file here would be a value the author never chose, written on
// their behalf to soften a cascade change the panel discloses in words instead.
func TestAuthoringOneHeaderBorderAttributeWritesOnlyThatAttribute(t *testing.T) {
	tpl := headerBorderDocument(t, "")
	// PRECONDITION: the header inherits NOTHING — the table's own border is
	// the box's now (SPEC-table-rules §1).
	before := projectHeaderCascade(t, tpl)
	if before.HeaderBorderWidthResolved != "" || before.HeaderBorderColorResolved != "" || before.HeaderBorderEdgesResolved != "" {
		t.Fatalf("precondition: the header must resolve no border of its own, got %#v", before)
	}
	if before.HeaderBorderWidth != "" || before.HeaderBorderColor != "" || before.HeaderBorderEdges != "" {
		t.Fatalf("precondition: the header must declare no border of its own, got %#v", before)
	}
	mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"e1","field":"border.width","op":"set","value":1.5}`)
	encoded := canonicalBytes(t, tpl)
	if !bytes.Contains(encoded, []byte(`"headerStyle"`)) || !bytes.Contains(encoded, []byte(`"width": 1.5`)) {
		t.Fatalf("the authored width did not reach the file:\n%s", encoded)
	}
	// THE TWO ATTRIBUTES NOBODY TOUCHED ARE ABSENT FROM THE HEADER'S BLOCK.
	// Asserted on the PROJECTION's committed members rather than by string
	// search, because the table's own border legitimately carries both and a
	// byte-level search for "#445566" would find it there.
	view := projectHeaderCascade(t, tpl)
	if view.HeaderBorderWidth != "1500" {
		t.Errorf("committed headerBorder.width = %q, want \"1500\"", view.HeaderBorderWidth)
	}
	if view.HeaderBorderColor != "" || view.HeaderBorderEdges != "" {
		t.Errorf("authoring the width seeded the other two attributes: colour %q, edges %q — the command must carry exactly what the author chose", view.HeaderBorderColor, view.HeaderBorderEdges)
	}
	// AND THE CASCADE HAS CHANGED HANDS. The two untouched attributes now
	// resolve to the FORMAT's defaults, not to the table's border, because
	// resolveHeaderStyle takes the header's block whole. This is the fact the
	// panel has to state in words, and it is asserted here so the words are
	// describing something real.
	if view.HeaderBorderColorResolved != "#000000" || view.HeaderBorderEdgesResolved != "top,right,bottom,left" {
		t.Errorf("after authoring one attribute the untouched two resolve to %q / %q, want the format's own #000000 / all four edges — the table's border must stop contributing entirely", view.HeaderBorderColorResolved, view.HeaderBorderEdgesResolved)
	}
	if _, err := ParseTemplate(encoded); err != nil {
		t.Fatalf("the document a single border attribute produced no longer loads: %v", err)
	}
}

// ROW 2 — the SECOND attribute authored leaves the first alone.
func TestAuthoringASecondHeaderBorderAttributeLeavesTheFirstAlone(t *testing.T) {
	tpl := headerBorderDocument(t, `"headerStyle": {"border": {"width": 1.5}},`)
	mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"e1","field":"border.color","op":"set","value":"#c81e1e"}`)
	view := projectHeaderCascade(t, tpl)
	if view.HeaderBorderWidth != "1500" {
		t.Errorf("committed headerBorder.width = %q after authoring the COLOUR, want the stored \"1500\" untouched", view.HeaderBorderWidth)
	}
	if view.HeaderBorderColor != "#c81e1e" {
		t.Errorf("committed headerBorder.color = %q, want #c81e1e", view.HeaderBorderColor)
	}
	if view.HeaderBorderEdges != "" {
		t.Errorf("committed headerBorder.edges = %q, want absent — two commands must not add up to three attributes", view.HeaderBorderEdges)
	}
}

// ROW 3 — clearing ONE attribute removes it, leaves its siblings, and leaves a
// border that is still drawn.
func TestClearingOneHeaderBorderAttributeKeepsTheRest(t *testing.T) {
	tpl := headerBorderDocument(t, `"headerStyle": {"border": {"width": 1.5, "color": "#c81e1e"}},`)
	mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"e1","field":"border.color","op":"clear"}`)
	view := projectHeaderCascade(t, tpl)
	if view.HeaderBorderColor != "" {
		t.Errorf("committed headerBorder.color = %q after a clear, want absent", view.HeaderBorderColor)
	}
	if view.HeaderBorderWidth != "1500" {
		t.Errorf("committed headerBorder.width = %q, want the \"1500\" the clear was not asked about", view.HeaderBorderWidth)
	}
	// STILL DRAWN: the block survives with one attribute, so the cleared colour
	// falls to the FORMAT's default rather than back to the table's — the
	// takeover is not undone by clearing one of three.
	if view.HeaderBorderEdgesResolved == "" {
		t.Error("clearing one attribute stopped the border being painted at all")
	}
	if view.HeaderBorderColorResolved != "#000000" {
		t.Errorf("resolved headerBorder.color = %q after the clear, want the format's #000000 — the header still owns its border as a whole", view.HeaderBorderColorResolved)
	}
	encoded := canonicalBytes(t, tpl)
	if bytes.Contains(encoded, []byte(`"#c81e1e"`)) {
		t.Errorf("the cleared colour survived in the bytes:\n%s", encoded)
	}
	// "the header's border block survived" is asserted through the PROJECTION
	// above and never by searching the bytes for `"border"`: this fixture's
	// TABLE declares one too, so that search would pass whatever happened to
	// the header's.
	if !bytes.Contains(encoded, []byte(`"headerStyle"`)) {
		t.Errorf("clearing one of two attributes dropped the whole headerStyle:\n%s", encoded)
	}
}

// ROW 4 — clearing the LAST attribute collapses TWO nested blocks, and this is
// the one that reds if cleanupEmptyHeaderStyle's empty-`border` mirror is
// deleted. Without it the file keeps `"border": {}`, whose `Border.Set` is still
// true, so the whole-style check can never fire and `headerStyle` is pinned
// alive forever.
func TestClearingTheLastHeaderBorderAttributeCollapsesBorderAndHeaderStyle(t *testing.T) {
	tpl := headerBorderDocument(t, `"headerStyle": {"border": {"width": 1.5}},`)
	mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"e1","field":"border.width","op":"clear"}`)
	encoded := canonicalBytes(t, tpl)
	if bytes.Contains(encoded, []byte(`"border": {}`)) {
		t.Errorf("an empty border block survived the clear:\n%s", encoded)
	}
	if bytes.Contains(encoded, []byte(`"headerStyle"`)) {
		t.Errorf("clearing the header style's only member left a headerStyle key:\n%s", encoded)
	}
	// THE HEADER RESOLVES NO BORDER AT ALL, which is what makes the collapse
	// a behaviour rather than tidiness: before SPEC-table-rules the header
	// inherited the table's border back; now the table's border is the box's,
	// so what comes back is the absence — and a member still claiming
	// 3pt/#445566/top,bottom here would mean the collapse never happened and
	// the stale header block is still being read.
	view := projectHeaderCascade(t, tpl)
	if view.HeaderBorderWidthResolved != "" || view.HeaderBorderColorResolved != "" || view.HeaderBorderEdgesResolved != "" {
		t.Errorf("after clearing the last attribute the header resolves %#v, want no border at all", view)
	}
	if _, err := ParseTemplate(encoded); err != nil {
		t.Fatalf("the collapsed document no longer loads: %v", err)
	}
}

// AND THE INNER COLLAPSE ON ITS OWN, so the test above cannot be satisfied by
// the outer one alone: a header style that declares something ELSE keeps its
// headerStyle key and must still lose the emptied border.
func TestClearingTheLastBorderAttributeCollapsesTheBorderInsideALivingHeaderStyle(t *testing.T) {
	// NO BORDER ON THE TABLE'S OWN STYLE HERE, deliberately: the byte search
	// below is for the word `border` anywhere in the file, and a table border
	// would satisfy it and make the assertion meaningless.
	tpl := headerCascadeDocument(t, "", `"headerStyle": {"align": "center", "border": {"width": 1.5}},`)
	mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"e1","field":"border.width","op":"clear"}`)
	encoded := canonicalBytes(t, tpl)
	if bytes.Contains(encoded, []byte(`"border"`)) {
		t.Errorf("the emptied border survived inside a headerStyle that still declares align:\n%s", encoded)
	}
	if !bytes.Contains(encoded, []byte(`"align": "center"`)) {
		t.Errorf("the collapse took the align it was not asked about:\n%s", encoded)
	}
}

// A clear of a border attribute against a document with NO headerStyle at all
// moves no bytes — the short-circuit ahead of headerStyleFor, asserted for the
// nested path as well as the flat one.
func TestClearingAHeaderBorderAttributeThatIsAlreadyAbsentMovesNoBytes(t *testing.T) {
	tpl := headerBorderDocument(t, "")
	before := canonicalBytes(t, tpl)
	for _, field := range []string{"border.width", "border.color", "border.edges"} {
		mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"e1","field":"`+field+`","op":"clear"}`)
	}
	if after := canonicalBytes(t, tpl); !bytes.Equal(before, after) {
		t.Errorf("clearing three already-absent border attributes changed the document:\nbefore\n%s\nafter\n%s", before, after)
	}
	// AND AGAINST A PRESENT headerStyle WITH NO border, which takes the other
	// branch: headerStyleFor materialises nothing, the clear arm's own guard
	// declines to create a border, and the collapse finds nothing to drop.
	present := headerBorderDocument(t, `"headerStyle": {"align": "center"},`)
	beforePresent := canonicalBytes(t, present)
	mustApplyToTable(t, present, `{"kind":"updateTableHeaderStyle","version":1,"id":"e1","field":"border.edges","op":"clear"}`)
	if after := canonicalBytes(t, present); !bytes.Equal(beforePresent, after) {
		t.Errorf("clearing an absent border inside a present headerStyle changed the document:\nbefore\n%s\nafter\n%s", beforePresent, after)
	}
}

// THE TWO REFUSAL ROWS, LOCATED AT THE DOTTED PATH. The dot is the whole reason
// the field names are spelled the way they are: `table.headerStyle.border.width`
// is a path the document actually has, where the camelCase alternative would
// have located at `table.headerStyle.borderWidth`, a key no file carries
// (DW-333).
func TestAMalformedHeaderBorderValueIsRefusedAtItsOwnDottedPath(t *testing.T) {
	tpl := headerBorderDocument(t, "")
	// A NEGATIVE WIDTH — the loader's own rule (ISO 32000-1 §8.4.3.2), restated
	// at this door only so the author gets a LOCATED sentence instead of an
	// unlocated parse failure off the round-trip.
	refusalSaysWhy(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"e1","field":"border.width","op":"set","value":-1}`,
		"table.headerStyle.border.width",
		"border.width must not be negative: a PDF line width is non-negative (ISO 32000-1 8.4.3.2); use 0 for the thinnest line")
	// A COLOUR THE FILE DOOR CANNOT CHECK. `internal/template` may not import
	// parseHexColor (AD-1), so a malformed border colour LOADS and surfaces at
	// render; refusing it here is what keeps the author's own edit located.
	refusalSaysWhy(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"e1","field":"border.color","op":"set","value":"red"}`,
		"table.headerStyle.border.color", "border.color must be a #RRGGBB colour")
	// AND THE EMPTY EDGE ARRAY, which is not "no border" but a stroke with no
	// side to draw. Clearing the attribute is how an author says the other thing.
	refusalSaysWhy(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"e1","field":"border.edges","op":"set","value":[]}`,
		"table.headerStyle.border.edges", "border.edges must be a non-empty string array")
	// AND AN EDGE NAME THE LOADER'S CLOSED SET DOES NOT CARRY. This arm validated
	// no name at all, so `["middle"]` was ADMITTED here, mutated the document, and
	// the refusal arrived from ParseTemplate on wasm's round-trip naming no
	// element and no field — the unlocated failure the width and colour arms
	// restate the loader's own rules to avoid. template.IsBorderEdge reads the
	// very map parse_bands.go reads, so this door cannot drift from that one.
	refusalSaysWhy(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"e1","field":"border.edges","op":"set","value":["middle"]}`,
		"table.headerStyle.border.edges", "border.edges must name only top, right, bottom, left: middle is not one of them")
	// A LEGAL NAME BESIDE AN ILLEGAL ONE IS STILL REFUSED — the check is per
	// name, not "at least one is known".
	refusalSaysWhy(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"e1","field":"border.edges","op":"set","value":["top","Bottom"]}`,
		"table.headerStyle.border.edges", "border.edges must name only top, right, bottom, left: Bottom is not one of them")
	// That the document did not move is asserted by refusalSaysWhy itself, which
	// runs refusalLeavesTheDocumentAlone first — and it is the load-bearing half
	// here: the old arm mutated the template and only then failed, downstream.
}

// ZERO IS ACCEPTED, AND IT IS NOT THE SAME AS ABSENT. It is the thinnest device
// line PDF can draw (parse_bands.go says so in those words), so the command door
// must not borrow the positive-length rule its font-size sibling uses.
func TestAZeroHeaderBorderWidthIsAcceptedAndPainted(t *testing.T) {
	tpl := headerBorderDocument(t, "")
	mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"e1","field":"border.width","op":"set","value":0}`)
	encoded := canonicalBytes(t, tpl)
	if !bytes.Contains(encoded, []byte(`"width": 0`)) {
		t.Errorf("a zero border width did not reach the file:\n%s", encoded)
	}
	view := projectHeaderCascade(t, tpl)
	// THE RESOLVED PAIR IS WHERE A DECLARED ZERO IS STILL LEGIBLE. The COMMITTED
	// member cannot say it — 0 is this projection's spelling of absent, a
	// collapse TableColumnsProjection's own comment discloses — but the resolved
	// twin reads 0 for a declared zero and 500 for a genuinely absent width, so
	// the panel can still tell an author what will be drawn.
	if view.HeaderBorderWidthResolved != "0" {
		t.Errorf("resolved headerBorder.width = %q for a declared zero, want \"0\" — the format's 0.5pt default must not overwrite an explicit zero", view.HeaderBorderWidthResolved)
	}
	if view.HeaderBorderEdgesResolved != "top,right,bottom,left" {
		t.Errorf("resolved headerBorder.edges = %q, want all four — a zero-width border is still a border that is painted", view.HeaderBorderEdgesResolved)
	}
}

// "NOTHING IS PAINTED" IS SAID BY THE EDGES MEMBER AND BY NOTHING ELSE, AND THE
// REASON IS NOT THE ONE IT USED TO BE. It is not that a resolved width of 0 is
// ambiguous — since the width pair became a STRING, "" means "no border reaches
// the header row" and "0" means "a declared zero-width border", which are two
// different values. It is the SECOND way to paint nothing that only the edge
// list can report: a border that DOES resolve but whose declared `edges` names
// no side. There the width resolves to a real number and the emitter still draws
// nothing, because its gate is `HasStroke && (Top || Right || Bottom || Left)`.
// Both halves are asserted below, which is why this test has two parts.
func TestNoBorderAnywhereProjectsAnEmptyResolvedEdgeList(t *testing.T) {
	tpl := headerCascadeDocument(t, "", "")
	view := projectHeaderCascade(t, tpl)
	if view.HeaderBorderEdgesResolved != "" {
		t.Errorf("resolved headerBorder.edges = %q with no border declared anywhere, want empty — no border reaches the header row, so nothing is stroked", view.HeaderBorderEdgesResolved)
	}
	if view.HeaderBorderWidthResolved != "" || view.HeaderBorderColorResolved != "" {
		t.Errorf("with no border anywhere the width/colour resolve to %q/%q, want this projection's own absence spelling \"\"/\"\"", view.HeaderBorderWidthResolved, view.HeaderBorderColorResolved)
	}
	// AND A DECLARED `edges: []` — a hand-edited shape no command can write —
	// reaches the same empty answer, because the PDF emitter's own gate is
	// `HasStroke && (Top || Right || Bottom || Left)`. Two ways to paint
	// nothing, one member that reports it.
	none := headerCascadeDocument(t, `"style": {"border": {"width": 2, "edges": []}},`, "")
	if got := projectHeaderCascade(t, none).HeaderBorderEdgesResolved; got != "" {
		t.Errorf("a declared empty edge list resolves to %q, want empty — no side means no stroke is emitted", got)
	}
}

// A HAND-AUTHORED BORDER CARRYING AN UNKNOWN SUB-KEY SURVIVES EVERY EDIT THIS
// PANEL CAN MAKE, including the collapse. `Border.Extra` is consulted inside
// cleanupEmptyHeaderStyle's new mirror for the same reason `Style.Extra` is
// consulted outside it: an unknown key rides opaquely through a load and a save,
// and dropping a block that still carries one deletes an author's data.
func TestAnUnknownHeaderBorderSubKeySurvivesTheCollapse(t *testing.T) {
	tpl := headerBorderDocument(t, `"headerStyle": {"border": {"width": 1.5, "dash": "2 2"}},`)
	if !bytes.Contains(canonicalBytes(t, tpl), []byte(`"dash"`)) {
		t.Fatalf("precondition: the unknown sub-key must round-trip:\n%s", canonicalBytes(t, tpl))
	}
	mustApplyToTable(t, tpl, `{"kind":"updateTableHeaderStyle","version":1,"id":"e1","field":"border.width","op":"clear"}`)
	encoded := canonicalBytes(t, tpl)
	if bytes.Contains(encoded, []byte(`"width": 1.5`)) {
		t.Errorf("the cleared width survived:\n%s", encoded)
	}
	if !bytes.Contains(encoded, []byte(`"dash"`)) {
		t.Errorf("the collapse deleted an unknown sub-key it was not asked about:\n%s", encoded)
	}
	if !bytes.Contains(encoded, []byte(`"headerStyle"`)) {
		t.Errorf("the headerStyle was dropped although its border still declares something:\n%s", encoded)
	}
}

// THE PROJECTION'S RESOLVED TRIO IS THE RENDERER'S OWN ANSWER, NOT A SECOND
// IMPLEMENTATION OF IT, and this is the tie that keeps it that way. Both sides
// call resolvedBorderWidth/Color/Edges; a copy of the defaults on either side
// would drift.
//
// ⚠ FOUR ASSERTIONS, AND THEY ASSERT TWO DIFFERENT PROPERTIES. READ THIS
// BEFORE DELETING ANY OF THEM — AND IN PARTICULAR, DO NOT DELETE THE FOURTH.
//
// THE FIRST THREE ASSERT DELEGATION, AND THAT IS A REAL PROPERTY. TableColumns
// projects each resolved border member by CALLING resolvedBorderWidth/Color/
// Edges — the same three functions these assertions compare it against — so
// while the projection delegates they cannot fail for any input: both sides are
// one expression. That does not make them worthless. They are exactly what
// WOULD fail if someone later inlined a different value into the projection
// instead of delegating, which is the second-copy drift Story 14.8 extracted
// these functions to prevent, and which the designer's own `?? 500` /
// `?? '#000000'` already cost once. What they do NOT assert is the DEFAULTS:
// they say the two sides agree, never what the two sides agree ON.
//
// THE FOURTH ASSERTION IS THE ONLY ONE THAT PINS THE DEFAULTS THEMSELVES —
// 500, #000000, top,right,bottom,left — and therefore the only one that can
// fail if both sides move together. It is the reason a joint edit still has to
// face folio-format.md's documented values. DELETE THE FOURTH AND THIS TEST
// CANNOT FAIL AT ALL. Anyone reading "the first three are derived" as a licence
// to trim has it exactly backwards: the derived three are the delegation guard
// and stay, and the fourth is the coverage and stays.
//
// Checked over a border with EVERY sub-key absent, because that is the only
// input where the defaults are what answer.
func TestTheProjectedHeaderBorderDefaultsAreTheRenderersOwn(t *testing.T) {
	tpl := headerCascadeDocument(t, "", `"headerStyle": {"border": {}},`)
	view := projectHeaderCascade(t, tpl)
	empty := template.Border{}
	if want := strconv.FormatInt(int64(resolvedBorderWidth(empty)), 10); view.HeaderBorderWidthResolved != want {
		t.Errorf("resolved headerBorder.width = %q, and the renderer answers %q for the same border", view.HeaderBorderWidthResolved, want)
	}
	if want := resolvedBorderColor(empty); view.HeaderBorderColorResolved != want {
		t.Errorf("resolved headerBorder.color = %q, and the renderer answers %q for the same border", view.HeaderBorderColorResolved, want)
	}
	if want := edgeListOf(resolvedBorderEdges(empty)); view.HeaderBorderEdgesResolved != want {
		t.Errorf("resolved headerBorder.edges = %q, and the renderer answers %q for the same border", view.HeaderBorderEdgesResolved, want)
	}
	// AND THE NUMBERS THEMSELVES, PINNED ONCE, so a joint edit of both sides
	// still has to face folio-format.md's documented defaults.
	if view.HeaderBorderWidthResolved != "500" || view.HeaderBorderColorResolved != "#000000" || view.HeaderBorderEdgesResolved != "top,right,bottom,left" {
		t.Errorf("an all-absent border resolves to %q/%q/%q, want the format's documented 500/#000000/top,right,bottom,left", view.HeaderBorderWidthResolved, view.HeaderBorderColorResolved, view.HeaderBorderEdgesResolved)
	}
}
