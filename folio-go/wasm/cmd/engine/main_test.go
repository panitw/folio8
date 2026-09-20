//go:build js && wasm

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
	"github.com/panitw/folio8/folio-go/internal/wasm"
)

// This is compiled for the real js/wasm host every story. Browser execution is
// owned by the D-000.4 Epic 5 boundary cadence; the native wasm.Engine test
// covers the same canonical fixture on every ordinary Go run.
func TestWasmHostRoundTripsCanonicalFixture(t *testing.T) {
	input, err := os.ReadFile("../../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := wasm.NewEngine(elapsedClock(), fonts.Shipped())
	nonCanonical := append([]byte("\n  "), input...)
	loaded := dispatch(engine, request{Operation: "load", PayloadBase64: base64.StdEncoding.EncodeToString(nonCanonical)})
	if !loaded.OK || loaded.Snapshot.DocumentState != "loaded" {
		t.Fatalf("load response = %#v", loaded)
	}
	serialized := dispatch(engine, request{Operation: "serialize"})
	if !serialized.OK {
		t.Fatalf("serialize response = %#v", serialized)
	}
	got, err := base64.StdEncoding.DecodeString(serialized.BytesBase64)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, input) {
		t.Fatal("wasm host serialization bypassed canonical Go serialization")
	}
}

// TestWasmHostSanitizesTemplateDiagnostics carries BOTH arms of the
// boundary rule, because Story 7.8 moved a population across it.
//
// ARM ONE — the destruction rule itself, on the population that still
// carries it. TEMPLATE_MALFORMED's message is replaced wholesale
// because that message may quote the offending document back, and a
// large or hostile one would be reflected instead of described. The
// fixture is re-pointed, not the rule: it used to be a well-formed
// object with a 2048-byte value, which reached here as an UNCODED
// *LoadError and so was bucketed under TEMPLATE_MALFORMED along with
// every located field error in the format. D-7.8.1 stopped bucketing
// LoadErrors there, so a document that is merely WRONG no longer
// exercises this rule; a document that is not a document does.
//
// ARM TWO — and it exists because arm one alone would have been a green
// test measuring the residue. When a change moves a population OUT of a
// guard's scope, the guard's test must be re-pointed at a member still
// in scope AND the departed population asserted under its new
// treatment. The departed population is the parseable-but-invalid
// document, and its new treatment is D-7.8.5's: the message SURVIVES,
// so the author is finally told which element and which field — and the
// fragment of their own document quoted back inside it is bounded IN
// RUNES and visibly elided, which is what makes surviving safe.
func TestWasmHostSanitizesTemplateDiagnostics(t *testing.T) {
	t.Run("unparseable-bytes-are-described-not-reflected", func(t *testing.T) {
		engine := wasm.NewEngine(elapsedClock(), fonts.Shipped())
		malicious := `{"version":"1.0","page":"` + string(bytes.Repeat([]byte("x"), 2048)) + `"`
		got := dispatch(engine, request{Operation: "load", PayloadBase64: base64.StdEncoding.EncodeToString([]byte(malicious))})
		if got.OK || got.DiagnosticCode != folio8.DiagCodeTemplateMalformed || got.Message != "The template could not be processed" {
			t.Fatalf("diagnostic = %#v", got)
		}
		if len(got.Message) > 512 || bytes.Contains([]byte(got.Message), []byte("xxx")) {
			t.Fatalf("unsafe message = %q", got.Message)
		}
	})

	t.Run("parseable-but-invalid-keeps-its-message-bounded-and-elided", func(t *testing.T) {
		// A WELL-FORMED document whose `style` key holds 2048 Thai
		// characters — the exact shape that regressed when D-7.8.1
		// landed alone. Thai and not "xxx" on purpose: a bound that
		// counted BYTES would pass on ASCII while handing this author a
		// third of the budget, and would split a rune on the way out.
		const thai = "\u0e01"
		value := strings.Repeat(thai, 2048)
		doc := `{"assets":{},"bands":{"content":{"elements":[{"id":"e1","type":"text","x":0,"y":0,"width":200,"height":40,"value":"v","style":"` + value + `"}]},"pageFooter":{"elements":[],"height":20},"pageHeader":{"elements":[],"height":20}},"fonts":{"body":["Noto Sans"]},"locale":"en","nextId":2,"page":{"margin":{"bottom":36,"left":36,"right":36,"top":36},"orientation":"portrait","size":"A4"},"utcOffset":"+00:00","version":"1.0"}`
		engine := wasm.NewEngine(elapsedClock(), fonts.Shipped())
		got := dispatch(engine, request{Operation: "load", PayloadBase64: base64.StdEncoding.EncodeToString([]byte(doc))})
		if got.OK {
			t.Fatalf("a non-object style must fail the load: %#v", got)
		}
		if got.DiagnosticCode != folio8.DiagCodeTemplateFieldInvalid {
			t.Fatalf("code = %q, want %q", got.DiagnosticCode, folio8.DiagCodeTemplateFieldInvalid)
		}
		if got.Message == "The template could not be processed" {
			t.Fatal("the message was destroyed: a parseable document's located refusal must reach its author (D-7.8.1)")
		}
		for _, want := range []string{"style", "e1"} {
			if !strings.Contains(got.Message, want) {
				t.Fatalf("the message must name %q, got %q", want, got.Message)
			}
		}
		reflected := strings.Count(got.Message, thai)
		if reflected > 84 {
			t.Fatalf("the host reported %d runes of the author's own document; the bound is 84 (D-7.8.5)", reflected)
		}
		if reflected <= 28 {
			t.Fatalf("only %d runes survived: the bound is counting BYTES, not runes (D-7.8.5)", reflected)
		}
		if !strings.Contains(got.Message, "\u2026") {
			t.Fatalf("the elided message must say so, got %q", got.Message)
		}
		if !utf8.ValidString(got.Message) {
			t.Fatalf("the reported message is not valid UTF-8 — a bound split a rune: %q", got.Message)
		}
		t.Logf("bounded reflection at the host: %d bytes, %d author runes, code=%s", len(got.Message), reflected, got.DiagnosticCode)
	})

	t.Run("a-multi-fragment-runaway-still-fits-the-host-window", func(t *testing.T) {
		// The leg above spends ONE fragment budget. This one spends
		// three at once: claimID passes a raw element id as ElementID,
		// again as Value, and again inside Reason via
		// validateElementID's %q. Four per-fragment rune bounds share
		// no budget, so the assembled sentence measured 1142 bytes —
		// and bounded() here is a raw BYTE slice value[:512], which
		// delivered it to the author cut mid-rune with no elision
		// marker. Thai on purpose, for the same reason as above.
		const thai = "\u0e01"
		id := strings.Repeat(thai, 2048)
		doc := `{"assets":{},"bands":{"content":{"elements":[{"id":"` + id + `","type":"text","x":0,"y":0,"width":200,"height":40,"value":"v"}]},"pageFooter":{"elements":[],"height":20},"pageHeader":{"elements":[],"height":20}},"fonts":{"body":["Noto Sans"]},"locale":"en","nextId":2,"page":{"margin":{"bottom":36,"left":36,"right":36,"top":36},"orientation":"portrait","size":"A4"},"utcOffset":"+00:00","version":"1.0"}`
		engine := wasm.NewEngine(elapsedClock(), fonts.Shipped())
		got := dispatch(engine, request{Operation: "load", PayloadBase64: base64.StdEncoding.EncodeToString([]byte(doc))})
		if got.OK {
			t.Fatalf("an id that is not an id must fail the load: %#v", got)
		}
		if len(got.Message) > 512 {
			t.Fatalf("the host reported %d bytes, past its own 512-byte window", len(got.Message))
		}
		if !utf8.ValidString(got.Message) {
			t.Fatalf("the reported message is not valid UTF-8 — the host's byte slice split a rune: %q", got.Message)
		}
		if !strings.Contains(got.Message, "\u2026") {
			t.Fatalf("a message this long was truncated somewhere and must say so, got %q", got.Message)
		}
		t.Logf("multi-fragment runaway at the host: %d bytes, code=%s", len(got.Message), got.DiagnosticCode)
	})
}

// TestWasmHostReportsTheTableJustifyRefusalIntact is Story 7.8's AC4,
// asserted at the only place it is actually decided: the wasm boundary.
//
// A designer author opens a hand-edited `.folio` whose table carries
// `style.align: "justify"`. Before this story that document LOADED,
// paid a MAJOR version bump and drew every cell at the start edge with
// no diagnostic at all. Refusing it at load is only half the fix — an
// UNCODED refusal would have arrived here as TEMPLATE_MALFORMED, whose
// message reportableMessage replaces with "The template could not be
// processed", and the author would be told nothing about which element
// or which field. Every Go-side assertion would still have been green.
//
// This is why D-7.8.1 had to be settled before the story could be
// written, and it is the shape of
// TestWasmHostReportsTheLineSpacingRefusalIntact above.
func TestWasmHostReportsTheTableJustifyRefusalIntact(t *testing.T) {
	const doc = `{"assets":{},"bands":{"content":{"elements":[{"id":"e1","type":"table","x":0,"y":0,"bind":"rows[]","as":"row","headerHeight":20,"columns":[{"id":"e2","label":"L","width":100,"bind":"{{row.v}}"}],"style":{"fontFamily":"body","fontSize":11,"align":"justify"}}]},"pageFooter":{"elements":[],"height":20},"pageHeader":{"elements":[],"height":20}},"fonts":{"body":["Noto Sans"]},"locale":"en","nextId":3,"page":{"margin":{"bottom":36,"left":36,"right":36,"top":36},"orientation":"portrait","size":"A4"},"utcOffset":"+00:00","version":"2.0"}`
	engine := wasm.NewEngine(elapsedClock(), fonts.Shipped())
	got := dispatch(engine, request{Operation: "load", PayloadBase64: base64.StdEncoding.EncodeToString([]byte(doc))})
	if got.OK {
		t.Fatalf("a table carrying style.align: \"justify\" must fail the load: %#v", got)
	}
	if got.DiagnosticCode != folio8.DiagCodeTemplateFieldInvalid {
		t.Fatalf("code = %q, want %q", got.DiagnosticCode, folio8.DiagCodeTemplateFieldInvalid)
	}
	if got.Message == "The template could not be processed" {
		t.Fatal("the message was replaced by the malformed-template placeholder — the author is told neither which element nor which field")
	}
	for _, want := range []string{"e1", "style.align", "left, center, right"} {
		if !strings.Contains(got.Message, want) {
			t.Fatalf("the message must name %q, got %q", want, got.Message)
		}
	}
	if strings.Contains(got.Message, "right, justify") {
		t.Fatalf("the message must not name justify among a table's legal values, got %q", got.Message)
	}

	// The SAME value on a TEXT element still loads. Without this leg the
	// assertions above are equally consistent with a blanket ban, which
	// Story 7.3, Story 7.4 and two shipped goldens forbid.
	const textDoc = `{"assets":{},"bands":{"content":{"elements":[{"id":"e1","type":"text","x":0,"y":0,"width":200,"height":40,"value":"v","style":{"fontFamily":"body","fontSize":11,"align":"justify"}}]},"pageFooter":{"elements":[],"height":20},"pageHeader":{"elements":[],"height":20}},"fonts":{"body":["Noto Sans"]},"locale":"en","nextId":2,"page":{"margin":{"bottom":36,"left":36,"right":36,"top":36},"orientation":"portrait","size":"A4"},"utcOffset":"+00:00","version":"2.0"}`
	textEngine := wasm.NewEngine(elapsedClock(), fonts.Shipped())
	textGot := dispatch(textEngine, request{Operation: "load", PayloadBase64: base64.StdEncoding.EncodeToString([]byte(textDoc))})
	if !textGot.OK {
		t.Fatalf("a TEXT element's style.align: \"justify\" must still load (FR47): %#v", textGot)
	}
	t.Logf("table refusal at the host: code=%s message=%q; the same value on a text element loaded", got.DiagnosticCode, got.Message)
}

// TestWasmHostReportsTheLineSpacingRefusalIntact is Story 7.2's AC7 at
// the only place it is actually decided. reportableMessage replaces a
// diagnostic's message with "The template could not be processed" for
// TEMPLATE_MALFORMED and for that code ALONE — so an UNCODED lineSpacing
// load error would be destroyed here, before the author ever saw which
// element or which range it was about, and every Go-side assertion would
// still be green. The coded load error (TEMPLATE_FIELD_INVALID since
// D-7.8.2 retired the field's own code) is what makes the AC reachable;
// this is where that is observable.
func TestWasmHostReportsTheLineSpacingRefusalIntact(t *testing.T) {
	engine := wasm.NewEngine(elapsedClock(), fonts.Shipped())
	doc := `{"assets":{},"bands":{"content":{"elements":[{"id":"e1","type":"text","x":0,"y":0,"width":200,"height":40,"value":"v","style":{"fontFamily":"body","fontSize":11,"lineSpacing":1000.001}}]},"pageFooter":{"elements":[],"height":20},"pageHeader":{"elements":[],"height":20}},"fonts":{"body":["Noto Sans"]},"locale":"en","nextId":2,"page":{"margin":{"bottom":36,"left":36,"right":36,"top":36},"orientation":"portrait","size":"A4"},"utcOffset":"+00:00","version":"1.0"}`
	got := dispatch(engine, request{Operation: "load", PayloadBase64: base64.StdEncoding.EncodeToString([]byte(doc))})
	if got.OK {
		t.Fatalf("an out-of-range lineSpacing must fail the load: %#v", got)
	}
	if got.DiagnosticCode != folio8.DiagCodeTemplateFieldInvalid {
		t.Fatalf("code = %q, want %q", got.DiagnosticCode, folio8.DiagCodeTemplateFieldInvalid)
	}
	if got.Message == "The template could not be processed" {
		t.Fatal("the message was replaced by the malformed-template placeholder — the author is told nothing about which element or which range")
	}
	for _, want := range []string{"e1", "lineSpacing"} {
		if !strings.Contains(got.Message, want) {
			t.Fatalf("the message must name %q, got %q", want, got.Message)
		}
	}
}

func TestWasmHostReportsEngineAuthoredRenderMessages(t *testing.T) {
	starter, err := os.ReadFile("../../../../folio-designer/public/templates/starter.folio")
	if err != nil {
		t.Fatal(err)
	}
	encode := func(b []byte) string { return base64.StdEncoding.EncodeToString(b) }
	place := func(changes string) (*wasm.Engine, []byte) {
		engine := wasm.NewEngine(elapsedClock(), fonts.Shipped())
		if loaded := dispatch(engine, request{Operation: "load", PayloadBase64: encode(starter)}); !loaded.OK {
			t.Fatalf("load = %#v", loaded)
		}
		created := dispatch(engine, request{Operation: "command", PayloadBase64: encode([]byte(`{"kind":"createComponent","version":1,"type":"text","band":"content","x":40,"y":40,"width":200,"height":24,"snap":false}`))})
		if !created.OK {
			t.Fatalf("create = %#v", created)
		}
		if changes != "" {
			changed := dispatch(engine, request{Operation: "command", PayloadBase64: encode([]byte(`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":` + changes + `}`))})
			if !changed.OK {
				t.Fatalf("change = %#v", changed)
			}
		}
		serialized := dispatch(engine, request{Operation: "serialize"})
		if !serialized.OK {
			t.Fatalf("serialize = %#v", serialized)
		}
		out, err := base64.StdEncoding.DecodeString(serialized.BytesBase64)
		if err != nil {
			t.Fatal(err)
		}
		return engine, out
	}
	data := encode([]byte(`{"amount":10000}`))
	params := encode([]byte(`{}`))

	// A number bound into text renders as its exact decimal (2026-09-13).
	numberEngine, numbered := place(`{"expression":{"op":"set","value":"{{amount}}"}}`)
	got := dispatch(numberEngine, request{Operation: "render", TemplateBase64: encode(numbered), DataBase64: data, ParamsBase64: params})
	if !got.OK {
		t.Fatalf("number render = %#v", got)
	}
	// A statically number-kind text value commits and saves at 3.3.
	if _, literal := place(`{"expression":{"op":"set","value":"{{1}}"}}`); !strings.Contains(string(literal), `"version": "3.3"`) {
		t.Fatalf("a {{1}} text value must save at 3.3:\n%s", literal)
	}

	// A wrong-kind (boolean) binding carries its own diagnostic code; the
	// message must name the element and the reason rather than a fixed
	// placeholder.
	boundEngine, bound := place(`{"expression":{"op":"set","value":"{{flag}}"}}`)
	got = dispatch(boundEngine, request{Operation: "render", TemplateBase64: encode(bound), DataBase64: encode([]byte(`{"flag":true}`)), ParamsBase64: params})
	if got.OK || got.DiagnosticCode == "" || !strings.Contains(got.Message, "bool") || got.ElementID != "e1" {
		t.Fatalf("bind render = %#v", got)
	}

	// A render failure with no diagnostic code of its own still reports what
	// the engine said instead of collapsing to "the engine rejected".
	unfontedEngine, unfonted := place(`{"fontFamily":{"op":"clear"}}`)
	got = dispatch(unfontedEngine, request{Operation: "render", TemplateBase64: encode(unfonted), DataBase64: data, ParamsBase64: params})
	if got.OK || got.DiagnosticCode != "ENGINE_REJECTED" || !strings.Contains(got.Message, "style.fontFamily") {
		t.Fatalf("unfonted render = %#v", got)
	}
	if len(got.Message) > 512 {
		t.Fatalf("unbounded message = %d bytes", len(got.Message))
	}
}

func TestTableColumnsRequestRequiresTheExactSelectionEnvelope(t *testing.T) {
	engine := wasm.NewEngine(elapsedClock(), fonts.Shipped())
	payload := base64.StdEncoding.EncodeToString([]byte(`{"id":"e7"}`))
	for _, in := range []request{
		{Operation: "table-columns", TemplateBase64: base64.StdEncoding.EncodeToString([]byte("template")), PayloadBase64: payload},
		{Operation: "table-columns", DataBase64: base64.StdEncoding.EncodeToString([]byte("sample")), PayloadBase64: payload},
		{Operation: "table-columns", ParamsBase64: base64.StdEncoding.EncodeToString([]byte("params")), PayloadBase64: payload},
		{Operation: "table-columns", PayloadBase64: base64.StdEncoding.EncodeToString([]byte(`{"id":"e7","bind":"row.amount"}`))},
		{Operation: "table-columns", PayloadBase64: base64.StdEncoding.EncodeToString([]byte(`{"id":"e7","footer":"sum"}`))},
		{Operation: "table-columns", PayloadBase64: base64.StdEncoding.EncodeToString([]byte(`{"id":"e7","sample":"data"}`))},
		{Operation: "table-columns", PayloadBase64: base64.StdEncoding.EncodeToString([]byte(`{"id":"e7"}{"id":"e8"}`))},
		{Operation: "table-columns", PayloadBase64: base64.StdEncoding.EncodeToString([]byte(`{"id":"` + string(bytes.Repeat([]byte("e"), 129)) + `"}`))},
	} {
		out := dispatch(engine, in)
		if out.OK || out.DiagnosticCode != "WASM_INPUT_INVALID" {
			t.Fatalf("table envelope = %#v", out)
		}
	}
}

// TestWasmHostReportsTheDuplicateKeyRefusalAsComponentInvalid is Story 15.2a's
// acceptance criterion at the only place the MAPPING is actually decided.
// engineFailure matches *designer.ComponentCommandError BEFORE *folio8.RenderError,
// and a bare fmt.Errorf on the same decode path would instead fall through to
// ENGINE_REJECTED with no location at all — which is what every other refusal
// in ApplyComponentCommand's decoder does, and what this one must not.
//
// ElementID must arrive EMPTY. A duplicate key means the id the command names
// is exactly the thing that cannot be trusted: executed at the baseline, a
// command that named e1 in the page header mutated e5 in the page footer.
//
// This file is built for js/wasm, so it runs in the wasm suite rather than in
// the ordinary Go gate; folio-go's own
// TestDuplicateKeyRefusalArrivesAtTheHostAsComponentInvalid reads this mapping
// out of main.go's source so the tie is measured on every story too.
func TestWasmHostReportsTheDuplicateKeyRefusalAsComponentInvalid(t *testing.T) {
	const doc = `{"assets":{},"bands":{"content":{"elements":[{"id":"e1","type":"text","x":0,"y":0,"width":200,"height":40,"value":"first","style":{"fontFamily":"body","fontSize":11}},{"id":"e2","type":"text","x":0,"y":60,"width":200,"height":40,"value":"second","style":{"fontFamily":"body","fontSize":11}}]},"pageFooter":{"elements":[],"height":20},"pageHeader":{"elements":[],"height":20}},"fonts":{"body":["Noto Sans"]},"locale":"en","nextId":3,"page":{"margin":{"bottom":36,"left":36,"right":36,"top":36},"orientation":"portrait","size":"A4"},"utcOffset":"+00:00","version":"2.0"}`
	engine := wasm.NewEngine(elapsedClock(), fonts.Shipped())
	if loaded := dispatch(engine, request{Operation: "load", PayloadBase64: base64.StdEncoding.EncodeToString([]byte(doc))}); !loaded.OK {
		t.Fatalf("fixture precondition: the document must load, got %#v", loaded)
	}
	// Names e1 and changes e2, by appending a second ids and a second changes.
	const command = `{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"value":{"op":"set","value":"FIRST"}},"ids":["e2"],"changes":{"value":{"op":"set","value":"SECOND"}}}`
	got := dispatch(engine, request{Operation: "command", PayloadBase64: base64.StdEncoding.EncodeToString([]byte(command))})
	if got.OK {
		t.Fatalf("duplicate-key bytes were accepted at the host: %#v", got)
	}
	if got.DiagnosticCode != "COMPONENT_INVALID" {
		t.Fatalf("code = %q, want COMPONENT_INVALID — ENGINE_REJECTED would leave the panel with no location", got.DiagnosticCode)
	}
	if got.ElementID != "" {
		t.Fatalf("ElementID = %q, want empty: a duplicate key makes the named id untrustworthy", got.ElementID)
	}
	if got.DataPath == "" {
		t.Fatal("DataPath is empty, so the refusal names neither an element nor the command")
	}
	// The document did not move. The other half of "it refuses" is that the
	// element a last-wins resolution would have mutated is still what it was.
	serialized := dispatch(engine, request{Operation: "serialize"})
	if !serialized.OK {
		t.Fatalf("serialize after a refused command: %#v", serialized)
	}
	after, err := base64.StdEncoding.DecodeString(serialized.BytesBase64)
	if err != nil {
		t.Fatalf("decode serialized bytes: %v", err)
	}
	if !bytes.Contains(after, []byte(`"second"`)) || bytes.Contains(after, []byte("SECOND")) {
		t.Fatal("the element the duplicate would have mutated did not survive the refusal")
	}
}

// TestWasmHostReportsThePageSetupDuplicateKeyRefusalAsPageSetupInvalid is the
// SECOND door's code, and it is a different question from the first — not a
// repetition of it. engineFailure matches *designer.ComponentCommandError before
// the page-setup fallback, so a page-setup refusal raised as a plain
// componentFailure would report COMPONENT_INVALID and miss the designer's only
// code-branching page-setup consumer entirely. The two doors are asserted
// separately, one code each, because that is the only way the difference is
// observable.
func TestWasmHostReportsThePageSetupDuplicateKeyRefusalAsPageSetupInvalid(t *testing.T) {
	const doc = `{"assets":{},"bands":{"content":{"elements":[]},"pageFooter":{"elements":[],"height":20},"pageHeader":{"elements":[],"height":20}},"fonts":{"body":["Noto Sans"]},"locale":"en","nextId":1,"page":{"margin":{"bottom":36,"left":36,"right":36,"top":36},"orientation":"portrait","size":"A4"},"utcOffset":"+00:00","version":"2.0"}`
	engine := wasm.NewEngine(elapsedClock(), fonts.Shipped())
	if loaded := dispatch(engine, request{Operation: "load", PayloadBase64: base64.StdEncoding.EncodeToString([]byte(doc))}); !loaded.OK {
		t.Fatalf("fixture precondition: the document must load, got %#v", loaded)
	}
	for _, probe := range []struct {
		name    string
		command string
	}{
		{
			name:    "a repeated top-level key",
			command: `{"kind":"pageSetup","version":1,"preset":"custom","orientation":"portrait","width":300,"height":400,"margin":{"top":10,"right":10,"bottom":10,"left":10},"orientation":"landscape"}`,
		},
		{
			name:    "a repeated key in the nested margin object",
			command: `{"kind":"pageSetup","version":1,"preset":"custom","orientation":"portrait","width":300,"height":400,"margin":{"top":1,"top":2,"right":10,"bottom":10,"left":10}}`,
		},
	} {
		t.Run(probe.name, func(t *testing.T) {
			got := dispatch(engine, request{Operation: "command", PayloadBase64: base64.StdEncoding.EncodeToString([]byte(probe.command))})
			if got.OK {
				t.Fatalf("duplicate-key bytes were accepted at the page-setup door: %#v", got)
			}
			if got.DiagnosticCode != "PAGE_SETUP_INVALID" {
				t.Fatalf("code = %q, want PAGE_SETUP_INVALID — the refusal must carry the code of the door it was raised at, and COMPONENT_INVALID is the code the designer's page-setup consumer does not branch on", got.DiagnosticCode)
			}
			if got.ElementID != "" {
				t.Fatalf("ElementID = %q, want empty: a duplicate key makes any named id untrustworthy", got.ElementID)
			}
			if got.DataPath == "" {
				t.Fatal("DataPath is empty, so the refusal is unlocated")
			}
		})
	}
}

func TestWasmGroupMovePreviewAndCommitTransport(t *testing.T) {
	input, err := os.ReadFile("../../../testdata/template/golden/worked-example.json")
	if err != nil {
		t.Fatal(err)
	}
	engine := wasm.NewEngine(elapsedClock(), fonts.Shipped())
	loaded := dispatch(engine, request{Operation: "load", PayloadBase64: base64.StdEncoding.EncodeToString(input)})
	if !loaded.OK {
		t.Fatal(loaded.Message)
	}
	// The fixture's two text elements are legal references; zero preview is
	// deliberately a no-op and must remain exactly revision-correlated.
	command := []byte(`{"kind":"moveComponents","version":1,"ids":["e1"],"referenceId":"e1","dx":0,"dy":0,"snap":true,"expectedRevision":1}`)
	query := dispatch(engine, request{Operation: "group-move-preview", PayloadBase64: base64.StdEncoding.EncodeToString(command)})
	if !query.OK || query.GroupMove == nil || query.GroupMove.Revision != 1 || query.GroupMove.DX != 0 || query.GroupMove.DY != 0 {
		t.Fatalf("preview: %+v", query)
	}
	committed := dispatch(engine, request{Operation: "command", PayloadBase64: base64.StdEncoding.EncodeToString(command)})
	if !committed.OK || committed.Snapshot.Revision != 1 || committed.Snapshot.CanUndo {
		t.Fatalf("zero commit: %+v", committed)
	}
	for _, in := range []request{
		{Operation: "group-move-preview", PayloadBase64: base64.StdEncoding.EncodeToString(command), DataBase64: "e30="},
		{Operation: "group-move-preview", PayloadBase64: base64.StdEncoding.EncodeToString(bytes.Replace(command, []byte(`"expectedRevision":1`), []byte(`"expectedRevision":2`), 1))},
		{Operation: "group-move-preview", PayloadBase64: base64.StdEncoding.EncodeToString(bytes.Replace(command, []byte(`"e1"`), []byte(`"ezmissing"`), -1))},
	} {
		if result := dispatch(engine, in); result.OK {
			t.Fatal("invalid preview accepted")
		}
	}
}

func TestWasmFormulaLongSyntaxCauseSurvivesWireBound(t *testing.T) {
	input, err := os.ReadFile("../../../testdata/example/first-pdf.folio")
	if err != nil {
		t.Fatal(err)
	}
	engine := wasm.NewEngine(elapsedClock(), fonts.Shipped())
	if _, err := engine.Load(input); err != nil {
		t.Fatal(err)
	}
	command, _ := json.Marshal(map[string]any{"kind": "updateComponentProperties", "version": 1, "ids": []string{"e1"}, "changes": map[string]any{"visibleIf": map[string]any{"op": "set", "value": strings.Repeat(" ", 600) + "loanAmount >"}}})
	got := dispatch(engine, request{Operation: "command", PayloadBase64: base64.StdEncoding.EncodeToString(command)})
	if got.OK || got.DiagnosticCode != folio8.DiagCodeExpressionInvalid || got.ElementID != "e1" || got.DataPath != "visibleIf" || !strings.Contains(got.Message, "unexpected end") || !strings.Contains(got.Message, "position 612") {
		t.Fatalf("lost bounded wire cause: %+v", got)
	}
}

func TestWasmFormulaMultiplePlaceholderCauseSurvivesWireBound(t *testing.T) {
	input, err := os.ReadFile("../../../testdata/example/first-pdf.folio")
	if err != nil {
		t.Fatal(err)
	}
	engine := wasm.NewEngine(elapsedClock(), fonts.Shipped())
	if _, err := engine.Load(input); err != nil {
		t.Fatal(err)
	}
	source := `{{"ok"}} {{` + strings.Repeat(" ", 600) + "loanAmount >}}"
	var document map[string]any
	if err := json.Unmarshal(input, &document); err != nil {
		t.Fatal(err)
	}
	content := document["bands"].(map[string]any)["content"].(map[string]any)
	content["elements"].([]any)[0].(map[string]any)["value"] = source
	malformed, _ := json.Marshal(document)
	got := dispatch(engine, request{Operation: "load", PayloadBase64: base64.StdEncoding.EncodeToString(malformed)})
	if got.OK || got.DiagnosticCode != folio8.DiagCodeExpressionInvalid || got.ElementID != "e1" || got.DataPath != "value" || !strings.Contains(got.Message, "placeholder 2") || !strings.Contains(got.Message, "position 612") || !strings.Contains(got.Message, "unexpected end") {
		t.Fatalf("lost placeholder wire cause: %+v", got)
	}
}
