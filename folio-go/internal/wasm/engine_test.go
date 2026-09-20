package wasm

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
	"github.com/panitw/folio8/folio-go/internal/designer"
)

func TestEngineTableCollectionBindingOwnsOneHistoryStep(t *testing.T) {
	input, err := os.ReadFile("../../../fixtures/statement-1/input.folio")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	loaded, err := engine.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	before, _, _ := engine.Serialize()
	command := []byte(`{"kind":"bindTableCollection","version":1,"id":"e8","segments":["report","entries"]}`)
	bound, err := engine.Apply(command)
	if err != nil || bound.Revision != loaded.Revision+1 || !bound.CanUndo || bound.CanRedo {
		t.Fatalf("collection bind history = %#v, %v", bound, err)
	}
	if got := canvasComponentByID(t, bound.Canvas, "e8").TableBind; got == nil || *got != "report.entries[]" {
		t.Fatalf("collection paint = %v", got)
	}
	after, _, _ := engine.Serialize()
	want := bytes.Replace(before, []byte(`"bind": "transactions[]"`), []byte(`"bind": "report.entries[]"`), 1)
	if !bytes.Equal(after, want) {
		t.Fatal("collection bind changed alias, column expressions, or other document state")
	}
	if _, err := engine.Apply([]byte(`{"kind":"bindTableCollection","version":1,"id":"e8","segments":["a.b"]}`)); err == nil {
		t.Fatal("ambiguous key succeeded")
	}
	stable, err := engine.Apply(command)
	if err != nil || stable.Revision != bound.Revision || stable.CanUndo != bound.CanUndo || stable.CanRedo != bound.CanRedo {
		t.Fatalf("repeated bind or refusal changed history = %#v, %v", stable, err)
	}
	undone, err := engine.Undo()
	if err != nil || undone.CanUndo || !undone.CanRedo {
		t.Fatalf("one undo = %#v, %v", undone, err)
	}
	restored, _, _ := engine.Serialize()
	if !bytes.Equal(restored, before) {
		t.Fatal("one undo did not restore the original bytes")
	}
	stable, err = engine.Apply([]byte(`{"kind":"bindTableCollection","version":1,"id":"e8","segments":["transactions"]}`))
	if err != nil || stable.Revision != undone.Revision || !stable.CanRedo || stable.CanUndo {
		t.Fatalf("same bind consumed redo = %#v, %v", stable, err)
	}
	redone, err := engine.Redo()
	if err != nil || !redone.CanUndo || redone.CanRedo {
		t.Fatalf("redo = %#v, %v", redone, err)
	}
	restored, _, _ = engine.Serialize()
	if !bytes.Equal(restored, after) {
		t.Fatal("redo did not restore the collection binding")
	}
}

func TestEngineCollectionFooterRebaseIsOneHistoryStep(t *testing.T) {
	input, err := os.ReadFile("../../testdata/commands/table-collection-footers.folio")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"bindTableCollection", "configureTableBinding"} {
		t.Run(kind, func(t *testing.T) {
			command := func(collection string) []byte {
				payload := map[string]any{"kind": kind, "version": 1, "id": "e1"}
				if kind == "bindTableCollection" {
					payload["segments"] = strings.Split(strings.TrimSuffix(collection, "[]"), ".")
				} else {
					payload["collection"], payload["alias"] = collection, "txn"
				}
				encoded, _ := json.Marshal(payload)
				return encoded
			}
			engine := NewEngine(testClock(), fonts.Shipped())
			loaded, err := engine.Load(input)
			if err != nil {
				t.Fatal(err)
			}
			before, _, _ := engine.Serialize()
			bound, err := engine.Apply(command("report.entries[]"))
			if err != nil || bound.Revision != loaded.Revision+1 || !bound.CanUndo || bound.CanRedo {
				t.Fatalf("collection/footer command = %#v, %v", bound, err)
			}
			after, _, _ := engine.Serialize()
			want := bytes.Replace(before, []byte(`"bind": "account.transactions[]"`), []byte(`"bind": "report.entries[]"`), 1)
			want = bytes.ReplaceAll(want, []byte(`"footerOf": "account.transactions.`), []byte(`"footerOf": "report.entries.`))
			if !bytes.Equal(after, want) {
				t.Fatalf("collection/footer update changed unrelated canonical bytes: %s", after)
			}
			if _, err := engine.Apply(command(strings.Repeat("a", 246) + "[]")); err == nil {
				t.Fatal("over-limit explicit footer source succeeded")
			} else {
				var failure *designer.ComponentCommandError
				if !errors.As(err, &failure) || failure.ElementID != "e1" || failure.DataPath != "column.footerOf" {
					t.Fatalf("footer overflow was not located: %v", err)
				}
			}
			stable, err := engine.Apply(command("report.entries[]"))
			if err != nil || stable.Revision != bound.Revision || stable.CanUndo != bound.CanUndo || stable.CanRedo != bound.CanRedo {
				t.Fatalf("no-op or footer refusal changed history: %#v, %v", stable, err)
			}
			undone, err := engine.Undo()
			if err != nil || undone.CanUndo || !undone.CanRedo {
				t.Fatalf("one undo = %#v, %v", undone, err)
			}
			restored, _, _ := engine.Serialize()
			if !bytes.Equal(restored, before) {
				t.Fatal("one undo did not restore the collection and explicit footer prefixes")
			}
			stable, err = engine.Apply(command("account.transactions[]"))
			if err != nil || stable.Revision != undone.Revision || stable.CanUndo || !stable.CanRedo {
				t.Fatalf("same collection consumed redo: %#v, %v", stable, err)
			}
			if _, err := engine.Redo(); err != nil {
				t.Fatal(err)
			}
			restored, _, _ = engine.Serialize()
			if !bytes.Equal(restored, after) {
				t.Fatal("redo did not restore the collection and explicit footer prefixes")
			}
		})
	}
}

func TestEngineLoadAndSerializeRoundTripsCanonicalBytes(t *testing.T) {
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	nonCanonical := append([]byte("\n  "), input...)
	snapshot, err := engine.Load(nonCanonical)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.DocumentState != "loaded" || snapshot.ByteLength != len(input) {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	got, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, input) {
		t.Fatal("worker engine serialization did not come from the Go serializer")
	}
	got[0] ^= 1
	again, _, err := engine.Serialize()
	if err != nil || !bytes.Equal(again, input) {
		t.Fatal("Serialize exposed aliased authoritative bytes")
	}
}

func TestEngineParameterReferencesAreARevisionCorrelatedProjection(t *testing.T) {
	engine := NewEngine(testClock(), fonts.Shipped())
	input, err := os.ReadFile("../../testdata/example/first-pdf.folio")
	if err != nil {
		t.Fatal(err)
	}
	input = bytes.Replace(input, []byte("{{customer.name}}"), []byte("{{params.reportDate}} {{params.reportDate}}"), 1)
	if _, err := engine.Load(input); err != nil {
		t.Fatal(err)
	}
	references, revision, err := engine.ParameterReferences()
	if err != nil || !reflect.DeepEqual(references, []string{"reportDate"}) || revision != engine.Snapshot().Revision {
		t.Fatalf("parameter references = %#v/%d, err=%v", references, revision, err)
	}
}

func TestEngineEmptyParameterReferencesRemainAnArrayForWorkerTransport(t *testing.T) {
	engine := NewEngine(testClock(), fonts.Shipped())
	input, err := os.ReadFile("../../testdata/example/first-pdf.folio")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Load(input); err != nil {
		t.Fatal(err)
	}
	references, _, err := engine.ParameterReferences()
	if err != nil {
		t.Fatal(err)
	}
	if references == nil || len(references) != 0 {
		t.Fatalf("empty parameter references must be a non-nil array for JSON transport, got %#v", references)
	}
}

func TestEngineTableColumnsAreRevisionCorrelatedAndHistoryOwned(t *testing.T) {
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	if _, err := engine.Load(input); err != nil {
		t.Fatal(err)
	}
	created, err := engine.Apply([]byte(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	var tableID string
	for _, component := range created.Canvas.Components {
		if component.Type == "table" {
			tableID = component.ID
		}
	}
	if tableID == "" {
		t.Fatal("table was not projected")
	}
	// This lifecycle starts with an explicitly emptied authored table.
	starter, err := engine.TableColumns(tableID)
	if err != nil || len(starter.Table.Columns) != 1 {
		t.Fatalf("starter column = %#v, err=%v", starter, err)
	}
	if _, err := engine.Apply([]byte(`{"kind":"removeTableColumn","version":1,"id":"` + tableID + `","columnId":"` + starter.Table.Columns[0].ID + `"}`)); err != nil {
		t.Fatal(err)
	}
	before := engine.Snapshot()
	after, err := engine.Apply([]byte(`{"kind":"addTableColumn","version":1,"id":"` + tableID + `","index":0}`))
	if err != nil || after.Revision != before.Revision+1 || !after.CanUndo {
		t.Fatalf("add history = %#v, err=%v", after, err)
	}
	projection, err := engine.TableColumns(tableID)
	if err != nil || projection.Revision != after.Revision || len(projection.Table.Columns) != 1 {
		t.Fatalf("projection = %#v, err=%v", projection, err)
	}
	if _, err := engine.Undo(); err != nil {
		t.Fatal(err)
	}
	undone, err := engine.TableColumns(tableID)
	if err != nil || len(undone.Table.Columns) != 0 || undone.Revision == projection.Revision {
		t.Fatalf("undo projection = %#v, err=%v", undone, err)
	}
	beforeRejected := engine.Snapshot()
	if _, err := engine.Apply([]byte(`{"kind":"addTableColumn","version":1,"id":"` + tableID + `","index":9}`)); err == nil {
		t.Fatal("invalid table command succeeded")
	}
	if afterRejected := engine.Snapshot(); !reflect.DeepEqual(afterRejected, beforeRejected) {
		t.Fatalf("rejection changed history/revision: %#v != %#v", afterRejected, beforeRejected)
	}
}

func TestEngineTableCreationAndStarterColumnUndoRedoAtomically(t *testing.T) {
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	loaded, err := engine.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	before, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	created, err := engine.Apply([]byte(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":500,"y":1500.123,"width":72,"height":24,"snap":true}`))
	if err != nil || created.Revision != loaded.Revision+1 || !created.CanUndo || created.CanRedo {
		t.Fatalf("creation history = %#v, err=%v", created, err)
	}
	known := map[string]bool{}
	for _, component := range loaded.Canvas.Components {
		known[component.ID] = true
	}
	var table designer.CanvasComponent
	for _, component := range created.Canvas.Components {
		if !known[component.ID] {
			table = component
		}
	}
	columns, err := engine.TableColumns(table.ID)
	if err != nil || len(columns.Table.Columns) != 1 || columns.Revision != created.Revision || columns.Table.Columns[0].Width != table.Width || table.X != 0 || table.Y != 1500000 {
		t.Fatalf("creation column and geometry = %#v/%#v, err=%v", table, columns, err)
	}
	canonical, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	undone, err := engine.Undo()
	if err != nil || undone.CanUndo || !undone.CanRedo {
		t.Fatalf("one undo must remove the entire creation: %#v, err=%v", undone, err)
	}
	undoBytes, _, err := engine.Serialize()
	if err != nil || !bytes.Equal(before, undoBytes) {
		t.Fatalf("undo did not restore the pre-creation bytes: %v", err)
	}
	if _, err := engine.TableColumns(table.ID); err == nil {
		t.Fatal("undone table still has an editor projection")
	}
	redone, err := engine.Redo()
	if err != nil || !redone.CanUndo || redone.CanRedo {
		t.Fatalf("redo history = %#v, err=%v", redone, err)
	}
	redoBytes, _, err := engine.Serialize()
	if err != nil || !bytes.Equal(canonical, redoBytes) {
		t.Fatalf("redo changed canonical IDs or geometry: %v", err)
	}
	again, err := engine.TableColumns(table.ID)
	if err != nil || !reflect.DeepEqual(columns.Table, again.Table) || !reflect.DeepEqual(created.Canvas, redone.Canvas) {
		t.Fatalf("redo changed the table/column projection: %v", err)
	}
}

func TestEngineRenderMatchesTheNativeProductionPathByteForByte(t *testing.T) {
	fixtures := []struct{ name, template, data, params string }{
		{"simple", "../../testdata/example/first-pdf.folio", `{"customer":{"name":"Ada"}}`, `{"preview":null}`},
		// This is a genuine five-page, table/text, multi-script shipped-font
		// document. A one-page ASCII fixture cannot detect pagination or font
		// path divergence, so both inputs remain required parity subjects.
		{"multipage-text-font", "../../../fixtures/statement-5/input.folio", "../../../fixtures/statement-5/data.json", "../../../fixtures/statement-5/params.json"},
		// spec-section-break CAP-6: the golden statement whose legend moves to
		// an added page. Preview (the wasm engine) must equal the native render.
		{"section-break-statement", "../../../fixtures/section-break-statement/input.folio", "../../../fixtures/section-break-statement/data.json", `{}`},
		// spec-section-break CAP-7: the unanchored golden, whose legend is
		// pushed down on page 1. Preview must equal the native render.
		{"section-break-unanchored", "../../../fixtures/section-break-unanchored/input.folio", "../../../fixtures/section-break-unanchored/data.json", `{}`},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			input, err := os.ReadFile(fixture.template)
			if err != nil {
				t.Fatal(err)
			}
			data, params := []byte(fixture.data), []byte(fixture.params)
			if strings.HasSuffix(fixture.data, ".json") {
				data, err = os.ReadFile(fixture.data)
				if err != nil {
					t.Fatal(err)
				}
			}
			if strings.HasSuffix(fixture.params, ".json") {
				params, err = os.ReadFile(fixture.params)
				if err != nil {
					t.Fatal(err)
				}
			}
			// BOTH SIDES OF THE PARITY READ THE SAME SET, AND IT IS THE
			// ELEVEN-FACE ONE UNDER EVERY BUILD (spec-deferred-offline-cache).
			// This test asks whether the engine's render equals the production
			// render of the same bytes — a question about the RENDERER, not
			// about which faces a build embeds. Left on fonts.Shipped(), it
			// would silently change subject under `-tags nocjkface` and refuse
			// the CJK fixtures for a reason that has nothing to do with parity.
			faces := elevenFaceSet(t)
			engine := NewEngine(testClock(), faces)
			if _, err := engine.Load(input); err != nil {
				t.Fatal(err)
			}
			canonical, snapshot, err := engine.Serialize()
			if err != nil {
				t.Fatal(err)
			}
			wantTemplate, err := folio8.ParseTemplate(canonical)
			if err != nil {
				t.Fatal(err)
			}
			want, err := folio8.Render(wantTemplate, folio8.Data(data), folio8.Params(params), faces)
			if err != nil {
				t.Fatal(err)
			}
			got, evidence, err := engine.Render(canonical, data, params)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want.Bytes) || evidence.Revision != snapshot.Revision {
				t.Fatalf("wasm render diverged from production: revision=%d want=%d", evidence.Revision, snapshot.Revision)
			}
			wantDigest := sha256.Sum256(want.Bytes)
			if evidence.PDFSHA256 != fmt.Sprintf("%x", wantDigest) {
				t.Fatalf("digest = %q", evidence.PDFSHA256)
			}
			// STORY 13.3 — THE TWO RENDER FACTS THE BROWSER'S EVIDENCE RAIL PRINTS.
			//
			// ⚠ THE VERSION IS COMPARED AGAINST THE PACKAGE CONSTANT, WHICH IS ONLY
			// A REAL COMPARISON WHILE THE CONSTANT IS NON-EMPTY. Blanking
			// `folio8.Version` would satisfy `evidence.Version == folio8.Version`
			// tautologically while `isPreview` in the browser rejects `version: ''`
			// — no preview would render at all, with the whole Go suite green. So
			// the constant is checked for content first, and the projection against
			// the constant second. A literal expectation here would be worse than
			// either: a version invented for display is the exact defect the
			// evidence surface exists to prevent, and a literal would agree with an
			// invented one.
			if folio8.Version == "" {
				t.Fatal("folio8.Version is empty, which makes the comparison below vacuous and makes every browser preview unrenderable")
			}
			if evidence.Version != folio8.Version {
				t.Fatalf("engine version = %q, want %q", evidence.Version, folio8.Version)
			}
			// THE ELAPSED NUMBER IS THE CLOCK'S, READ ONCE EACH SIDE OF THE RENDER.
			// The engine takes its clock from NewEngine, and testClock advances a
			// fixed step per reading, so exactly one step is the only answer that
			// proves both readings happened. An `int64` nobody assigns is 0, and a
			// clock read once, or read around more than the render, is a multiple.
			if evidence.ElapsedMs != testClockStepMs {
				t.Fatalf("elapsed = %d ms, want %d: the clock was not read exactly once on each side of Render", evidence.ElapsedMs, testClockStepMs)
			}
			// AND BOTH SURVIVE THE WIRE ENCODING, INCLUDING A ZERO. `omitempty` on
			// either field would drop a legitimate `0 ms` render and an empty
			// version, and JSON is the only place that choice is observable.
			encoded, err := json.Marshal(RenderResult{})
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{`"elapsedMs":0`, `"version":""`} {
				if !strings.Contains(string(encoded), key) {
					t.Fatalf("zero-valued render reply %s omits %s", encoded, key)
				}
			}
			identity, identityRevision, err := engine.PreviewIdentity(data, params)
			// This is deliberately not a same-helper comparison: the engine must
			// supply precisely the complete shipped set to the public identity
			// contract. Omitting any production face in Engine.PreviewIdentity
			// therefore disagrees with this independently assembled expectation.
			wantIdentity := designer.PreviewIdentity(canonical, folio8.Data(data), folio8.Params(params), faces)
			if err != nil || identity != wantIdentity || evidence.Identity != wantIdentity || identityRevision != snapshot.Revision {
				t.Fatalf("identity evidence = %q/%d, render = %q, want=%q, err=%v", identity, identityRevision, evidence.Identity, wantIdentity, err)
			}
			for face, program := range faces {
				changed := append([]byte(nil), program...)
				changed[0] ^= 1
				mutated := elevenFaceSet(t)
				mutated[face] = changed
				if designer.PreviewIdentity(canonical, folio8.Data(data), folio8.Params(params), mutated) == wantIdentity {
					t.Fatalf("shipped face %q did not affect preview identity", face)
				}
			}
			if _, _, err := engine.Render(append(canonical, ' '), []byte(`{}`), []byte(`{}`)); err == nil {
				t.Fatal("stale/noncanonical template render unexpectedly succeeded")
			}
		})
	}
}

func TestEngineRejectedLoadIsTransactional(t *testing.T) {
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	before, err := engine.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	beforeBytes, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	invalid := bytes.Replace(input, []byte(`"top": 36`), []byte(`"top": 800`), 1)
	if _, err := engine.Load(invalid); err == nil {
		t.Fatal("invalid projection load unexpectedly succeeded")
	}
	after := engine.Snapshot()
	afterBytes, _, err := engine.Serialize()
	if err != nil || !reflect.DeepEqual(after, before) || !bytes.Equal(afterBytes, beforeBytes) {
		t.Fatalf("rejected load changed engine: before=%#v after=%#v", before, after)
	}
}

func TestEngineRejectsUnknownCommandWithoutChangingDocument(t *testing.T) {
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	before, err := engine.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Apply([]byte(`{"kind":"unknown"}`)); err == nil {
		t.Fatal("unknown command unexpectedly succeeded")
	}
	after := engine.Snapshot()
	if after != before {
		t.Fatalf("unknown command changed state: before=%#v after=%#v", before, after)
	}
}

func TestEngineCommitsComponentChangesThroughGoOwnedCommandChannel(t *testing.T) {
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	before, err := engine.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	after, err := engine.Apply([]byte(`{"kind":"createComponent","version":1,"type":"text","band":"content","x":12,"y":12,"width":72,"height":24,"snap":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != before.Revision+1 || after.ByteLength <= before.ByteLength || after.Canvas == nil || len(after.Canvas.Components) == 0 {
		t.Fatalf("commit snapshot = %#v, before = %#v", after, before)
	}
}

func TestEnginePageSetupRevisionAndProjectionChangeTogether(t *testing.T) {
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	before, err := engine.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	after, err := engine.Apply([]byte(`{"kind":"pageSetup","version":1,"preset":"Letter","orientation":"landscape","width":1,"height":1,"margin":{"top":36,"right":36,"bottom":36,"left":36}}`))
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != before.Revision+1 || after.Canvas == nil || after.Canvas.Width != 792000 || after.Canvas.Height != 612000 {
		t.Fatalf("page setup snapshot = %#v", after)
	}
}

func TestEnginePropertyBatchAdvancesOneRevisionOrLeavesEverythingUntouched(t *testing.T) {
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	before, err := engine.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	after, err := engine.Apply([]byte(`{"kind":"updateComponentProperties","version":1,"ids":["e1","e5"],"changes":{"y":{"op":"set","value":12}}}`))
	if err != nil || after.Revision != before.Revision+1 || after.Canvas == nil {
		t.Fatalf("property batch = %#v, %v", after, err)
	}
	bytesBeforeFailure, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Apply([]byte(`{"kind":"updateComponentProperties","version":1,"ids":["e1","missing"],"changes":{"y":{"op":"set","value":18}}}`)); err == nil {
		t.Fatal("bad target unexpectedly succeeded")
	}
	bytesAfterFailure, snapshotAfterFailure, err := engine.Serialize()
	if err != nil || snapshotAfterFailure.Revision != after.Revision || !bytes.Equal(bytesBeforeFailure, bytesAfterFailure) {
		t.Fatal("rejected property batch changed engine state")
	}
}

func TestEngineUndoRedoOwnsCommittedCanonicalHistoryAndResetsOnLoad(t *testing.T) {
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	loaded, err := engine.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	before, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	committed, err := engine.Apply([]byte(`{"kind":"createComponent","version":1,"type":"text","band":"content","x":12,"y":12,"width":72,"height":24,"snap":true}`))
	if err != nil {
		t.Fatal(err)
	}
	after, _, err := engine.Serialize()
	if err != nil || bytes.Equal(before, after) {
		t.Fatal("accepted command did not create a distinct canonical state")
	}
	undone, err := engine.Undo()
	if err != nil || undone.Revision != committed.Revision+1 {
		t.Fatalf("undo = %#v, %v", undone, err)
	}
	got, _, err := engine.Serialize()
	if err != nil || !bytes.Equal(got, before) {
		t.Fatal("undo did not restore engine canonical bytes")
	}
	redone, err := engine.Redo()
	if err != nil || redone.Revision != undone.Revision+1 {
		t.Fatalf("redo = %#v, %v", redone, err)
	}
	got, _, err = engine.Serialize()
	if err != nil || !bytes.Equal(got, after) {
		t.Fatal("redo did not restore engine canonical bytes")
	}
	if _, err := engine.Undo(); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Apply([]byte(`{"kind":"createComponent","version":1,"type":"rect","band":"content","x":100,"y":100,"width":72,"height":24,"snap":true}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Redo(); !errors.Is(err, ErrNoRedo) {
		t.Fatalf("divergent command retained redo branch: %v", err)
	}
	if _, err := engine.Load(input); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Undo(); !errors.Is(err, ErrNoUndo) {
		t.Fatalf("load retained undo history: %v", err)
	}
	if engine.Snapshot().Revision <= loaded.Revision {
		t.Fatal("revision was not monotonic")
	}
}

func TestEngineScalarBindingIsOneCanonicalUndoableMutation(t *testing.T) {
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	loaded, err := engine.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	before, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	bound, err := engine.Apply([]byte(`{"kind":"bindComponentScalar","version":1,"id":"e1","segments":["customer","name"]}`))
	if err != nil || bound.Revision != loaded.Revision+1 || !bound.CanUndo || bound.Canvas == nil {
		t.Fatalf("scalar bind snapshot = %#v, err=%v", bound, err)
	}
	component := canvasComponentByID(t, bound.Canvas, "e1")
	if component.Binding == nil || *component.Binding != "customer.name" {
		t.Fatalf("engine binding projection = %#v", component.Binding)
	}
	after, _, err := engine.Serialize()
	if err != nil || bytes.Equal(before, after) || !bytes.Contains(after, []byte(`"value": "{{customer.name}}"`)) {
		t.Fatalf("accepted bind was not canonical: %s, err=%v", after, err)
	}
	undone, err := engine.Undo()
	if err != nil || undone.Revision != bound.Revision+1 {
		t.Fatalf("undo scalar bind = %#v, err=%v", undone, err)
	}
	got, _, err := engine.Serialize()
	if err != nil || !bytes.Equal(got, before) {
		t.Fatalf("undo did not restore pre-bind bytes: %v", err)
	}
	redone, err := engine.Redo()
	if err != nil || redone.Revision != undone.Revision+1 {
		t.Fatalf("redo scalar bind = %#v, err=%v", redone, err)
	}
	got, _, err = engine.Serialize()
	if err != nil || !bytes.Equal(got, after) {
		t.Fatalf("redo did not restore scalar binding: %v", err)
	}
	stable := engine.Snapshot()
	if _, err := engine.Apply([]byte(`{"kind":"bindComponentScalar","version":1,"id":"e1","segments":["params","name"]}`)); err == nil {
		t.Fatal("params scalar bind unexpectedly succeeded")
	}
	if engine.Snapshot().Revision != stable.Revision {
		t.Fatal("rejected scalar bind advanced revision")
	}
}

func TestEngineScalarBindingRoundTripsAndNoOpPreservesHistoryBranches(t *testing.T) {
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	if _, err := engine.Load(input); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Apply([]byte(`{"kind":"bindComponentScalar","version":1,"id":"e1","segments":["customer","name"]}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Apply([]byte(`{"kind":"bindComponentScalar","version":1,"id":"e1","segments":["account","number"]}`)); err != nil {
		t.Fatal(err)
	}
	undone, err := engine.Undo()
	if err != nil || !undone.CanRedo {
		t.Fatalf("undo before no-op = %#v, err=%v", undone, err)
	}
	beforeNoOp, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	noOp, err := engine.Apply([]byte(`{"kind":"bindComponentScalar","version":1,"id":"e1","segments":["customer","name"]}`))
	if err != nil || noOp.Revision != undone.Revision || !noOp.CanRedo {
		t.Fatalf("same binding must preserve revision and redo: %#v, err=%v", noOp, err)
	}
	afterNoOp, _, err := engine.Serialize()
	if err != nil || !bytes.Equal(beforeNoOp, afterNoOp) {
		t.Fatalf("same binding changed canonical bytes: %v", err)
	}
	if _, err := engine.Redo(); err != nil {
		t.Fatalf("no-op binding discarded redo: %v", err)
	}
	canonical, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	reloaded := NewEngine(testClock(), fonts.Shipped())
	loaded, err := reloaded.Load(canonical)
	if err != nil || loaded.Canvas == nil || canvasComponentByID(t, loaded.Canvas, "e1").Binding == nil {
		t.Fatalf("saved canonical binding did not survive load: %#v, err=%v", loaded, err)
	}
}

func canvasComponentByID(t *testing.T, canvas *designer.CanvasProjection, id string) designer.CanvasComponent {
	t.Helper()
	for _, component := range canvas.Components {
		if component.ID == id {
			return component
		}
	}
	t.Fatalf("component %q is absent from wasm canvas", id)
	return designer.CanvasComponent{}
}

func TestEngineNoOpDoesNotChangeHistoryRevisionOrRedo(t *testing.T) {
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	if _, err := engine.Load(input); err != nil {
		t.Fatal(err)
	}
	command := []byte(`{"kind":"pageSetup","version":1,"preset":"custom","orientation":"landscape","width":300.125,"height":400.5,"margin":{"top":10,"right":11.5,"bottom":12,"left":13}}`)
	changed, err := engine.Apply(command)
	if err != nil || !changed.CanUndo || changed.CanRedo {
		t.Fatalf("first command = %#v, %v", changed, err)
	}
	undone, err := engine.Undo()
	if err != nil || undone.CanUndo || !undone.CanRedo {
		t.Fatalf("undo = %#v, %v", undone, err)
	}
	stable, err := engine.Apply([]byte(`{"kind":"pageSetup","version":1,"preset":"A4","orientation":"portrait","width":0,"height":0,"margin":{"top":36,"right":36,"bottom":36,"left":36}}`))
	if err != nil {
		t.Fatal(err)
	}
	if stable.Revision != undone.Revision || stable.CanUndo != undone.CanUndo || stable.CanRedo != undone.CanRedo {
		t.Fatalf("no-op changed history evidence: before=%#v after=%#v", undone, stable)
	}
	redone, err := engine.Redo()
	if err != nil || redone.Revision != stable.Revision+1 || !redone.CanUndo || redone.CanRedo {
		t.Fatalf("no-op cleared redo or changed revision: %#v, %v", redone, err)
	}
}

func TestEngineDuplicateIsACommittedGoCommand(t *testing.T) {
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	before, err := engine.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	after, err := engine.Apply([]byte(`{"kind":"duplicateComponent","version":1,"id":"e1","snap":true}`))
	if err != nil || after.Revision != before.Revision+1 || after.Canvas == nil || len(after.Canvas.Components) != len(before.Canvas.Components)+1 {
		t.Fatalf("duplicate = %#v, %v", after, err)
	}
	if after.Canvas.Components[len(after.Canvas.Components)-1].ID == "e1" {
		t.Fatal("duplicate retained its source opaque id")
	}
}

// ---------------------------------------------------------------------------
// STORY 8.1: THE FONT-CHAIN COMMANDS AS ENGINE TRANSACTIONS.
//
// fontChainEngineDocJSON declares THREE chains and names two of them from
// elements in different bands, including a table's headerStyle. It is the
// repo's first byte-level pin on multi-chain emission (Design Notes R2).
const fontChainEngineDocJSON = `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e7", "type": "text", "x": 0, "y": 0, "width": 100, "height": 20, "value": "content", "style": {"fontFamily": "body"}},
      {"id": "e9", "type": "table", "x": 0, "y": 30, "bind": "items[]", "headerHeight": 20,
        "columns": [{"id": "e10", "label": "Date", "width": 100, "bind": "{{row.a}}"}],
        "headerStyle": {"fontFamily": "body"}}
    ]},
    "pageFooter": {"elements": [{"id": "e12", "type": "text", "x": 0, "y": 0, "width": 100, "height": 20, "value": "footer", "style": {"fontFamily": "heading"}}], "height": 20},
    "pageHeader": {"elements": [{"id": "e2", "type": "text", "x": 0, "y": 0, "width": 100, "height": 20, "value": "header", "style": {"fontFamily": "body"}}], "height": 20}
  },
  "fonts": {"body": ["Noto Sans", "Noto Sans Thai"], "heading": ["Noto Sans"], "unused": ["Noto Sans SC"]},
  "locale": "en",
  "nextId": 39,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}`

func fontChainEngine(t *testing.T) *Engine {
	t.Helper()
	engine := NewEngine(testClock(), fonts.Shipped())
	if _, err := engine.Load([]byte(fontChainEngineDocJSON)); err != nil {
		t.Fatal(err)
	}
	return engine
}

func fontChainFamilies(t *testing.T, snapshot Snapshot) []string {
	t.Helper()
	if snapshot.Canvas == nil {
		t.Fatal("snapshot carries no canvas")
	}
	return snapshot.Canvas.FontFamilies
}

// TestEngineFontChainCommandsAdvanceExactlyOneRevisionAndOneUndoStep is AC1:
// every accepted chain command is ONE committed mutation, by construction —
// Apply's single pushUndo — and a refused one commits nothing.
func TestEngineFontChainCommandsAdvanceExactlyOneRevisionAndOneUndoStep(t *testing.T) {
	for _, command := range []string{
		`{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans"]}`,
		`{"kind":"renameFontChain","version":1,"name":"body","to":"brand"}`,
		`{"kind":"deleteFontChain","version":1,"name":"unused"}`,
		`{"kind":"addFontChainEntry","version":1,"name":"body","index":0,"face":"Noto Sans SC"}`,
		`{"kind":"moveFontChainEntry","version":1,"name":"body","from":0,"to":1}`,
		`{"kind":"removeFontChainEntry","version":1,"name":"body","index":0}`,
	} {
		engine := fontChainEngine(t)
		before := engine.Snapshot()
		after, err := engine.Apply([]byte(command))
		if err != nil {
			t.Fatalf("%s: %v", command, err)
		}
		if after.Revision != before.Revision+1 || !after.CanUndo || after.CanRedo || after.Canvas == nil {
			t.Fatalf("%s snapshot = %#v", command, after)
		}
		undone, err := engine.Undo()
		if err != nil {
			t.Fatalf("%s undo: %v", command, err)
		}
		if undone.CanUndo {
			t.Fatalf("%s pushed more than one undo entry", command)
		}
		restored, _, err := engine.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		fresh := fontChainEngine(t)
		original, _, err := fresh.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(restored, original) {
			t.Fatalf("%s: one undo did not restore the document", command)
		}
	}
}

// TestEngineFontChainRenameUndoesTheMapAndTheElementsTogether is AC2: the map
// key and all four references move in one entry, so one undo restores every
// one of them. A rename that pushed twice, or that carried the elements in a
// second transaction, would fail here rather than at render.
func TestEngineFontChainRenameUndoesTheMapAndTheElementsTogether(t *testing.T) {
	engine := fontChainEngine(t)
	before, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	renamed, err := engine.Apply([]byte(`{"kind":"renameFontChain","version":1,"name":"body","to":"brand"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := fontChainFamilies(t, renamed); !reflect.DeepEqual(got, []string{"brand", "heading", "unused"}) {
		t.Fatalf("families after rename = %#v", got)
	}
	after, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(after, []byte(`"body"`)) {
		t.Fatal("the old chain name survives somewhere in the renamed document")
	}
	if bytes.Count(after, []byte(`"fontFamily": "brand"`)) != 3 {
		t.Fatalf("renamed document carries %d brand references, want the three style/headerStyle bearers", bytes.Count(after, []byte(`"fontFamily": "brand"`)))
	}
	undone, err := engine.Undo()
	if err != nil || undone.CanUndo {
		t.Fatalf("undo = %#v, %v — a rename is ONE history entry", undone, err)
	}
	restored, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, restored) {
		t.Fatal("one undo did not restore the fonts map AND the elements together")
	}
}

func TestEngineRefusedFontChainCommandLeavesByteRevisionAndHistoryUntouched(t *testing.T) {
	engine := fontChainEngine(t)
	committed, err := engine.Apply([]byte(`{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans"]}`))
	if err != nil {
		t.Fatal(err)
	}
	before, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	for _, refused := range []string{
		`{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans"]}`,
		`{"kind":"deleteFontChain","version":1,"name":"body"}`,
		`{"kind":"removeFontChainEntry","version":1,"name":"heading","index":0}`,
		`{"kind":"renameFontChain","version":1,"name":"body","to":"heading"}`,
	} {
		if _, err := engine.Apply([]byte(refused)); err == nil {
			t.Fatalf("%s unexpectedly succeeded", refused)
		}
		after, snapshot, err := engine.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) || snapshot.Revision != committed.Revision || snapshot.CanUndo != committed.CanUndo || snapshot.CanRedo != committed.CanRedo {
			t.Fatalf("%s changed engine state: %#v", refused, snapshot)
		}
	}
}

// TestEngineFontChainRenameOutAndBackIsByteIdentical is Design Notes R2's
// claim, measured on a MULTI-CHAIN document: a chain's emitted position is a
// total function of its key and its entries are the slice, so renaming out and
// back must restore the canonical bytes exactly — fonts map AND bands, since a
// rename that failed to restore a style.fontFamily would move the bands bytes
// instead. Two commands, two revisions, two undo steps.
func TestEngineFontChainRenameOutAndBackIsByteIdentical(t *testing.T) {
	engine := fontChainEngine(t)
	loaded := engine.Snapshot()
	original, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	out, err := engine.Apply([]byte(`{"kind":"renameFontChain","version":1,"name":"body","to":"zbrand"}`))
	if err != nil {
		t.Fatal(err)
	}
	moved, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(original, moved) {
		t.Fatal("the rename did not move the canonical bytes; the round trip would prove nothing")
	}
	back, err := engine.Apply([]byte(`{"kind":"renameFontChain","version":1,"name":"zbrand","to":"body"}`))
	if err != nil {
		t.Fatal(err)
	}
	final, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, final) {
		t.Fatal("rename out and back did not restore the canonical bytes exactly")
	}
	if out.Revision != loaded.Revision+1 || back.Revision != out.Revision+1 {
		t.Fatalf("revisions = %d, %d, %d — two commands are two revisions", loaded.Revision, out.Revision, back.Revision)
	}
	if _, err := engine.Undo(); err != nil {
		t.Fatal(err)
	}
	second, err := engine.Undo()
	if err != nil || second.CanUndo {
		t.Fatalf("second undo = %#v, %v — two commands are two undo steps", second, err)
	}
}

// TestEngineFontChainNoOpChangesNothing is the bytes.Equal short-circuit at
// the chain path: a move that reorders nothing is valid and commits nothing.
func TestEngineFontChainNoOpChangesNothing(t *testing.T) {
	engine := fontChainEngine(t)
	committed, err := engine.Apply([]byte(`{"kind":"addFontChain","version":1,"name":"caption","entries":["Noto Sans"]}`))
	if err != nil {
		t.Fatal(err)
	}
	before, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	stable, err := engine.Apply([]byte(`{"kind":"moveFontChainEntry","version":1,"name":"body","from":1,"to":1}`))
	if err != nil {
		t.Fatal(err)
	}
	after, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || stable.Revision != committed.Revision || stable.CanUndo != committed.CanUndo || stable.CanRedo != committed.CanRedo {
		t.Fatalf("a no-op chain command changed engine state: %#v", stable)
	}
}

// TestEngineProjectsTheChainsThemselvesNotOnlyTheirNames is AC5: the canvas
// projection carries the entries, so a move or a remove is observable to the
// designer at all. FontChains[i].Name == FontFamilies[i] is asserted, because
// the browser's guard drops the whole snapshot if it stops holding.
func TestEngineProjectsTheChainsThemselvesNotOnlyTheirNames(t *testing.T) {
	engine := fontChainEngine(t)
	snapshot, err := engine.Apply([]byte(`{"kind":"moveFontChainEntry","version":1,"name":"body","from":0,"to":1}`))
	if err != nil {
		t.Fatal(err)
	}
	chains := snapshot.Canvas.FontChains
	names := make([]string, 0, len(chains))
	for _, chain := range chains {
		names = append(names, chain.Name)
	}
	if !reflect.DeepEqual(names, fontChainFamilies(t, snapshot)) {
		t.Fatalf("FontChains names = %#v, FontFamilies = %#v", names, snapshot.Canvas.FontFamilies)
	}
	// Story 8.3: an entry is a projected object. Both of these are named
	// faces, so both carry an empty AssetKey.
	if !reflect.DeepEqual(chains[0].Entries, []designer.CanvasFontChainEntry{{Face: "Noto Sans Thai"}, {Face: "Noto Sans"}}) {
		t.Fatalf("projected body chain = %#v, want the reordered entries", chains[0].Entries)
	}
}

// TestEngineFontChainMoveIsFollowedByTheFolio8Bytes is the I/O matrix's "Move
// an entry" row read literally: "`.folio` entry order follows verbatim". The
// in-memory slice was already asserted, but the slice is not the claim — the
// claim is about the SERIALIZED document, and the only byte-level chain
// assertions in this file were the rename round trip (which restores the
// original bytes, so it cannot see an entry order at all) and the no-op
// (from:1,to:1, which by construction moves nothing). A real reorder had no
// byte-level pin anywhere.
//
// The expected block is spelled out rather than derived, because deriving it
// from the same slice the command wrote would assert only that the serializer
// agrees with itself. writeStringArray preserves the slice order and
// writeFonts sorts only the KEYS, so this is what canonical form looks like.
func TestEngineFontChainMoveIsFollowedByTheFolio8Bytes(t *testing.T) {
	engine := fontChainEngine(t)
	before, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	authored := "\"body\": [\n      \"Noto Sans\",\n      \"Noto Sans Thai\"\n    ]"
	if !bytes.Contains(before, []byte(authored)) {
		t.Fatalf("fixture precondition: the authored chain order is not in the loaded bytes\n%s", before)
	}
	if _, err := engine.Apply([]byte(`{"kind":"moveFontChainEntry","version":1,"name":"body","from":0,"to":1}`)); err != nil {
		t.Fatal(err)
	}
	after, _, err := engine.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	reordered := "\"body\": [\n      \"Noto Sans Thai\",\n      \"Noto Sans\"\n    ]"
	if !bytes.Contains(after, []byte(reordered)) {
		t.Fatalf("the moved entry order did not reach the .folio bytes\n%s", after)
	}
	if bytes.Contains(after, []byte(authored)) {
		t.Fatal("the authored entry order survived the move in the .folio bytes")
	}
	// The OTHER chains are untouched, and the keys are still sorted: a move is
	// an edit to one slice, not a re-emission of the map.
	if !bytes.Equal(bytes.Replace(after, []byte(reordered), []byte(authored), 1), before) {
		t.Fatal("a move changed more of the document than the one chain's entry order")
	}
}

// TestEngineApplyRefusesADuplicateKeyOnEitherRoutingBranch covers the wasm
// path's OWN last-wins read, which is a third door onto the same defect and
// adds no check of its own: engine.go unmarshals into `struct{ Kind string }`
// to choose between the page-setup and component decoders, and that unmarshal
// routes on the LAST "kind" exactly as the two module decoders do.
//
// Both directions are asserted. A duplicate that would route a pageSetup
// command into the component decoder is refused, and so is one going the other
// way — routing correctly by accident is not the property.
func TestEngineApplyRefusesADuplicateKeyOnEitherRoutingBranch(t *testing.T) {
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, probe := range []struct {
		name string
		// component reports whether this payload is routed to the COMPONENT
		// door. Each door raises its refusal in its own shape, so the two are
		// distinguished here rather than asserted as one thing.
		component bool
		command   string
	}{
		{
			// Routed to ApplyComponentCommand by the LAST kind, having named
			// pageSetup first.
			name:      "a component command wearing a page-setup kind first",
			component: true,
			command:   `{"kind":"pageSetup","version":1,"id":"e1","kind":"deleteComponent"}`,
		},
		{
			// Routed to ApplyPageSetupCommand by the last kind.
			name:      "a page-setup command wearing a component kind first",
			component: false,
			command:   `{"kind":"deleteComponent","version":1,"preset":"A4","orientation":"portrait","width":0,"height":0,"margin":{"top":10,"right":10,"bottom":10,"left":10},"kind":"pageSetup"}`,
		},
		{
			name:      "a duplicate nested inside a component command",
			component: true,
			command:   `{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"value":{"op":"set","value":"FIRST","value":"SECOND"}}}`,
		},
	} {
		t.Run(probe.name, func(t *testing.T) {
			engine := NewEngine(testClock(), fonts.Shipped())
			before, err := engine.Load(input)
			if err != nil {
				t.Fatal(err)
			}
			_, err = engine.Apply([]byte(probe.command))
			if err == nil {
				t.Fatal("duplicate-key bytes were accepted through the wasm engine")
			}
			var failure *designer.ComponentCommandError
			switch {
			case probe.component:
				// The component door: a ComponentCommandError, which the host
				// maps to COMPONENT_INVALID. Anything else falls through to
				// ENGINE_REJECTED with no location at all.
				if !errors.As(err, &failure) {
					t.Fatalf("refusal is not a ComponentCommandError, so the host reports it unlocated: %v", err)
				}
				if failure.ElementID != "" {
					t.Fatalf("refusal named the element %q, which the duplicate has made untrustworthy", failure.ElementID)
				}
			default:
				// The page-setup door: NOT a ComponentCommandError, because
				// engineFailure matches that type first and would answer
				// COMPONENT_INVALID for a page-setup command. The prefix below
				// is what routes it to PAGE_SETUP_INVALID instead.
				if errors.As(err, &failure) {
					t.Fatalf("the page-setup door raised a ComponentCommandError, so the host reports COMPONENT_INVALID: %v", err)
				}
				if !strings.HasPrefix(err.Error(), "folio8: page.") {
					t.Fatalf("error = %q, want the `folio8: page.` prefix the host answers PAGE_SETUP_INVALID for", err.Error())
				}
			}
			if after := engine.Snapshot(); after != before {
				t.Fatalf("a refused command changed state: before=%#v after=%#v", before, after)
			}
		})
	}
}

func TestEngineGroupMovePreviewAtomicHistoryAndRevisionFence(t *testing.T) {
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	if _, err := engine.Load(input); err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, position := range []string{"12.125", "84.375"} {
		before := engine.Snapshot()
		added, err := engine.Apply([]byte(`{"kind":"createComponent","version":1,"type":"rect","band":"content","x":` + position + `,"y":12.225,"width":24,"height":12,"snap":false}`))
		if err != nil {
			t.Fatal(err)
		}
		known := map[string]bool{}
		for _, component := range before.Canvas.Components {
			known[component.ID] = true
		}
		for _, component := range added.Canvas.Components {
			if !known[component.ID] {
				ids = append(ids, component.ID)
			}
		}
	}
	command := func(dx string, revision uint64) []byte {
		return []byte(fmt.Sprintf(`{"kind":"moveComponents","version":1,"ids":[%q,%q],"referenceId":%q,"dx":%s,"dy":0,"snap":false,"expectedRevision":%d}`, ids[0], ids[1], ids[0], dx, revision))
	}
	original, before, _ := engine.Serialize()
	preview, err := engine.GroupMovePreview(command("2.125", before.Revision))
	if err != nil || preview.DX != 2125 || preview.Revision != before.Revision {
		t.Fatalf("preview=%+v, %v", preview, err)
	}
	if !reflect.DeepEqual(engine.Snapshot(), before) {
		t.Fatal("preview changed history or snapshot")
	}
	zero, err := engine.Apply(command("0", before.Revision))
	if err != nil || !reflect.DeepEqual(zero, before) {
		t.Fatal("zero changed revision/history")
	}
	moved, err := engine.Apply(command("2.125", before.Revision))
	if err != nil || moved.Revision != before.Revision+1 {
		t.Fatalf("move=%+v, %v", moved, err)
	}
	committed, _, _ := engine.Serialize()
	if _, err = engine.Apply(command("3", before.Revision)); err == nil {
		t.Fatal("stale revision accepted")
	}
	if _, err = engine.GroupMovePreview(command("3", before.Revision)); err == nil {
		t.Fatal("stale preview accepted")
	}
	if _, err = engine.Undo(); err != nil {
		t.Fatal(err)
	}
	restored, _, _ := engine.Serialize()
	if !bytes.Equal(restored, original) {
		t.Fatal("one undo did not restore all members")
	}
	if _, err = engine.Redo(); err != nil {
		t.Fatal(err)
	}
	redone, _, _ := engine.Serialize()
	if !bytes.Equal(redone, committed) {
		t.Fatal("redo did not replay whole group")
	}
	current := engine.Snapshot()
	invalid := bytes.Replace(command("1", current.Revision), []byte(fmt.Sprintf("%q", ids[1])), []byte(`"ezmissing"`), 1)
	if _, err = engine.Apply(invalid); err == nil {
		t.Fatal("missing member accepted")
	}
	if !reflect.DeepEqual(current, engine.Snapshot()) {
		t.Fatal("invalid member changed snapshot/history")
	}
}

func TestEngineFreshTableDuplicateHistoryPreservesIndependentColumnIDs(t *testing.T) {
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	loaded, err := engine.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	created, err := engine.Apply([]byte(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":0,"y":24,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, component := range loaded.Canvas.Components {
		known[component.ID] = true
	}
	var sourceID string
	for _, component := range created.Canvas.Components {
		if !known[component.ID] {
			sourceID = component.ID
		}
		known[component.ID] = true
	}
	source, err := engine.TableColumns(sourceID)
	if err != nil || len(source.Table.Columns) != 1 {
		t.Fatalf("source = %#v, err=%v", source, err)
	}
	before, _, _ := engine.Serialize()
	duplicated, err := engine.Apply([]byte(fmt.Sprintf(`{"kind":"duplicateComponent","version":1,"id":%q,"snap":true}`, sourceID)))
	if err != nil || duplicated.Revision != created.Revision+1 {
		t.Fatalf("duplicate did not commit once: %#v, err=%v", duplicated, err)
	}
	var duplicateID string
	for _, component := range duplicated.Canvas.Components {
		if !known[component.ID] {
			duplicateID = component.ID
		}
	}
	copy, err := engine.TableColumns(duplicateID)
	if err != nil || len(copy.Table.Columns) != 1 || copy.Table.Columns[0].ID == source.Table.Columns[0].ID || copy.Table.Columns[0].Width != source.Table.Columns[0].Width {
		t.Fatalf("duplicate column = %#v, err=%v", copy, err)
	}
	canonical, _, _ := engine.Serialize()
	reloaded := NewEngine(testClock(), fonts.Shipped())
	if _, err := reloaded.Load(canonical); err != nil {
		t.Fatalf("duplicated table did not reload: %v", err)
	}
	if _, err := engine.Undo(); err != nil {
		t.Fatal(err)
	}
	undone, _, _ := engine.Serialize()
	if !bytes.Equal(before, undone) {
		t.Fatal("one undo did not remove the duplicate and its column")
	}
	if _, err := engine.Redo(); err != nil {
		t.Fatal(err)
	}
	redone, _, _ := engine.Serialize()
	if !bytes.Equal(canonical, redone) {
		t.Fatal("redo changed the allocated duplicate IDs")
	}
	if _, err := engine.Apply([]byte(fmt.Sprintf(`{"kind":"updateTableColumn","version":1,"id":%q,"columnId":%q,"field":"header","value":"Independent copy"}`, duplicateID, copy.Table.Columns[0].ID))); err != nil {
		t.Fatal(err)
	}
	unchanged, err := engine.TableColumns(sourceID)
	if err != nil || !reflect.DeepEqual(source.Table, unchanged.Table) {
		t.Fatalf("editing the duplicate changed the original: %v", err)
	}
	if _, err := engine.Undo(); err != nil {
		t.Fatal(err)
	}
	editUndone, _, _ := engine.Serialize()
	if !bytes.Equal(canonical, editUndone) {
		t.Fatal("undoing the independent column edit changed duplication")
	}
}

func TestEngineColumnAuthoringSplitBindingClearAndReopenHistory(t *testing.T) {
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	loaded, err := engine.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	apply := func(command string) Snapshot {
		t.Helper()
		snapshot, err := engine.Apply([]byte(command))
		if err != nil {
			t.Fatalf("%s: %v", command, err)
		}
		return snapshot
	}
	serialized := func() []byte {
		t.Helper()
		data, _, err := engine.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	created := apply(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`)
	known := map[string]bool{}
	for _, component := range loaded.Canvas.Components {
		known[component.ID] = true
	}
	var id string
	for _, component := range created.Canvas.Components {
		if !known[component.ID] {
			id = component.ID
		}
	}
	starter, err := engine.TableColumns(id)
	if err != nil || len(starter.Table.Columns) != 1 {
		t.Fatalf("starter = %#v, err=%v", starter, err)
	}
	beforeAdd := serialized()
	added := apply(fmt.Sprintf(`{"kind":"addTableColumn","version":1,"id":%q,"index":1}`, id))
	columns, err := engine.TableColumns(id)
	if err != nil || len(columns.Table.Columns) != 2 || added.Revision != created.Revision+1 {
		t.Fatalf("split add = %#v, err=%v", columns, err)
	}
	first, second := columns.Table.Columns[0], columns.Table.Columns[1]
	if first.Width+second.Width != starter.Table.Columns[0].Width || first.Width != second.Width+(starter.Table.Columns[0].Width%2) || first.ID != starter.Table.Columns[0].ID {
		t.Fatalf("split changed geometry or original id: %#v", columns)
	}
	afterAdd := serialized()
	if _, err := engine.Undo(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(serialized(), beforeAdd) {
		t.Fatal("one undo did not restore both the starter width and column count")
	}
	if _, err := engine.Redo(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(serialized(), afterAdd) {
		t.Fatal("redo did not restore widths and allocated column id")
	}
	apply(fmt.Sprintf(`{"kind":"configureTableBinding","version":1,"id":%q,"collection":"transactions[]","alias":"txn"}`, id))
	bindCommand := func(field string) string {
		return fmt.Sprintf(`{"kind":"updateTableColumnBinding","version":1,"id":%q,"columnId":%q,"field":%q}`, id, first.ID, field)
	}
	beforeBind := engine.Snapshot()
	bound := apply(bindCommand("customer.name"))
	columns, err = engine.TableColumns(id)
	if err != nil || bound.Revision != beforeBind.Revision+1 || columns.Table.Columns[0].Binding != "{{txn.customer.name}}" || columns.Table.Columns[0].RowField != "customer.name" || !columns.Table.Columns[0].RowFieldEditable {
		t.Fatalf("typed binding = %#v, err=%v", columns, err)
	}
	boundBytes := serialized()
	beforeRefusal := engine.Snapshot()
	for _, invalid := range []string{bindCommand("customer..name"), fmt.Sprintf(`{"kind":"addTableColumn","version":1,"id":%q,"index":9}`, id)} {
		if _, err := engine.Apply([]byte(invalid)); err == nil {
			t.Fatalf("invalid command succeeded: %s", invalid)
		}
		if !bytes.Equal(serialized(), boundBytes) || !reflect.DeepEqual(engine.Snapshot(), beforeRefusal) {
			t.Fatal("refusal changed bytes, revision, or history")
		}
	}
	cleared := apply(bindCommand(""))
	columns, err = engine.TableColumns(id)
	if err != nil || cleared.Revision != bound.Revision+1 || columns.Table.Columns[0].Binding != "" || columns.Table.Columns[0].RowField != "" || !columns.Table.Columns[0].RowFieldEditable {
		t.Fatalf("clear = %#v, err=%v", columns, err)
	}
	clearedBytes := serialized()
	if _, err := engine.Undo(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(serialized(), boundBytes) {
		t.Fatal("clear was not one undo step")
	}
	if _, err := engine.Redo(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(serialized(), clearedBytes) {
		t.Fatal("redo did not restore cleared binding")
	}
	apply(bindCommand("date"))
	configuredBytes := serialized()
	if _, err := engine.Undo(); err != nil {
		t.Fatal(err)
	}
	beforeNoop := engine.Snapshot()
	if !beforeNoop.CanRedo {
		t.Fatal("no-op test requires redo history")
	}
	if snapshot := apply(bindCommand("")); !reflect.DeepEqual(snapshot, beforeNoop) {
		t.Fatal("no-op clear changed history or revision")
	}
	if _, err := engine.Redo(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(serialized(), configuredBytes) {
		t.Fatal("no-op clear lost redo history")
	}
	columns, err = engine.TableColumns(id)
	if err != nil {
		t.Fatal(err)
	}
	reopened := NewEngine(testClock(), fonts.Shipped())
	if _, err := reopened.Load(configuredBytes); err != nil {
		t.Fatal(err)
	}
	again, err := reopened.TableColumns(id)
	if err != nil || !reflect.DeepEqual(columns.Table, again.Table) {
		t.Fatalf("reopen changed widths or editable bindings: %#v, err=%v", again, err)
	}
	persisted, _, err := reopened.Serialize()
	if err != nil || !bytes.Equal(persisted, configuredBytes) {
		t.Fatalf("reopen changed saved bytes: %v", err)
	}
}

func columnAuthoringEngine(t *testing.T) (*Engine, string, string) {
	t.Helper()
	input, err := os.ReadFile("../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(testClock(), fonts.Shipped())
	loaded, err := engine.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	created, err := engine.Apply([]byte(`{"kind":"createComponent","version":1,"type":"table","band":"content","x":0,"y":0,"width":72,"height":24,"snap":false}`))
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, component := range loaded.Canvas.Components {
		known[component.ID] = true
	}
	for _, component := range created.Canvas.Components {
		if !known[component.ID] {
			columns, err := engine.TableColumns(component.ID)
			if err != nil {
				t.Fatal(err)
			}
			return engine, component.ID, columns.Table.Columns[0].ID
		}
	}
	t.Fatal("created table not found")
	return nil, "", ""
}

func TestEngineTableColumnBindingLimitRefusalPreservesHistory(t *testing.T) {
	engine, id, columnID := columnAuthoringEngine(t)
	configure := func(alias string) []byte {
		return []byte(fmt.Sprintf(`{"kind":"configureTableBinding","version":1,"id":%q,"collection":"transactions[]","alias":%q}`, id, alias))
	}
	bind := func(length int) []byte {
		return []byte(fmt.Sprintf(`{"kind":"updateTableColumnBinding","version":1,"id":%q,"columnId":%q,"field":%q}`, id, columnID, strings.Repeat("f", length)))
	}
	longAlias := strings.Repeat("a", 64)
	for _, command := range [][]byte{configure(longAlias), bind(187)} {
		if _, err := engine.Apply(command); err != nil {
			t.Fatal(err)
		}
	}
	columns, err := engine.TableColumns(id)
	if err != nil || len(columns.Table.Columns[0].Binding) != 256 {
		t.Fatalf("exact-boundary binding = %#v, err=%v", columns, err)
	}
	refuseUnchanged := func(command []byte) {
		t.Helper()
		before := engine.Snapshot()
		beforeBytes, _, _ := engine.Serialize()
		if _, err := engine.Apply(command); err == nil {
			t.Fatalf("invalid command succeeded: %s", command)
		}
		afterBytes, _, _ := engine.Serialize()
		if !reflect.DeepEqual(before, engine.Snapshot()) || !bytes.Equal(beforeBytes, afterBytes) {
			t.Fatal("refusal changed bytes/revision/history")
		}
		if _, err := engine.TableColumns(id); err != nil {
			t.Fatalf("refusal stranded the table editor: %v", err)
		}
	}
	refuseUnchanged(bind(188))
	refuseUnchanged(bind(192))
	if _, err := engine.Undo(); err != nil {
		t.Fatal(err)
	}
	if !engine.Snapshot().CanRedo {
		t.Fatal("redo precondition missing")
	}
	refuseUnchanged(bind(188))
	if _, err := engine.Redo(); err != nil {
		t.Fatal(err)
	}
	for _, command := range [][]byte{configure(""), bind(192)} {
		if _, err := engine.Apply(command); err != nil {
			t.Fatal(err)
		}
	}
	refuseUnchanged(configure(longAlias))
}

func TestEngineClearDerivedAggregateBindingRefusesUntilSourceIsExplicit(t *testing.T) {
	for _, aggregate := range []string{"sum", "avg"} {
		t.Run(aggregate, func(t *testing.T) {
			engine, id, columnID := columnAuthoringEngine(t)
			bind := func(field string) []byte {
				return []byte(fmt.Sprintf(`{"kind":"updateTableColumnBinding","version":1,"id":%q,"columnId":%q,"field":%q}`, id, columnID, field))
			}
			footer := func(source string) []byte {
				return []byte(fmt.Sprintf(`{"kind":"updateTableColumnFooter","version":1,"id":%q,"columnId":%q,"footer":%q,"footerOf":%q,"footerFormat":"0.00"}`, id, columnID, aggregate, source))
			}
			for _, command := range [][]byte{bind("amount"), footer("")} {
				if _, err := engine.Apply(command); err != nil {
					t.Fatal(err)
				}
			}
			before := engine.Snapshot()
			beforeBytes, _, _ := engine.Serialize()
			if _, err := engine.Apply(bind("")); err == nil {
				t.Fatal("clear silently removed the aggregate source")
			}
			afterBytes, _, _ := engine.Serialize()
			if !reflect.DeepEqual(before, engine.Snapshot()) || !bytes.Equal(beforeBytes, afterBytes) {
				t.Fatal("refused clear changed bytes/revision/history")
			}
			if _, err := engine.Apply(footer("items.amount")); err != nil {
				t.Fatal(err)
			}
			explicitBytes, _, _ := engine.Serialize()
			before = engine.Snapshot()
			cleared, err := engine.Apply(bind(""))
			if err != nil || cleared.Revision != before.Revision+1 {
				t.Fatalf("clear with explicit source: %v", err)
			}
			columns, err := engine.TableColumns(id)
			if err != nil || columns.Table.Columns[0].Binding != "" || columns.Table.Columns[0].Footer != aggregate || columns.Table.Columns[0].FooterOf != "items.amount" || columns.Table.Columns[0].FooterFormat != "0.00" {
				t.Fatalf("clear altered aggregate: %#v, err=%v", columns, err)
			}
			clearedBytes, _, _ := engine.Serialize()
			if _, err := engine.Undo(); err != nil {
				t.Fatal(err)
			}
			undone, _, _ := engine.Serialize()
			if !bytes.Equal(undone, explicitBytes) {
				t.Fatal("undo failed to restore the binding in one step")
			}
			if _, err := engine.Redo(); err != nil {
				t.Fatal(err)
			}
			redone, _, _ := engine.Serialize()
			if !bytes.Equal(redone, clearedBytes) {
				t.Fatal("redo changed the explicit aggregate")
			}
		})
	}
}
