package folio8_test

// STORY 11.4 / AC1 — WHAT A PICK LEAVES IN THE DOCUMENT, ASSERTED ON A REAL
// DOCUMENT AND NOT ON A COMMAND PAYLOAD.
//
// ⚠ WHY THIS FILE EXISTS AT ALL. The designer suite inspects the command BYTES
// a pick sends and nothing in it inspects a resulting document — the closest
// thing, `App.test.tsx`'s `robotoChain`, is a hand-built projection rather than
// a document the designer produced. So "the pick declares the cuts" was
// assertable on one side of the wire only, and the half that matters to an
// author — the file they save and reopen — was unreached.
//
// IT IS AN EXTERNAL TEST PACKAGE because it imports `folio-go/fonts`, and
// `fonts` imports `folio8`; inside `package folio8` that is an import cycle. It
// is the same reason `starter_template_test.go` is external, and this file
// copies that file's discipline: every expectation is DERIVED from the document
// the commands produced, never restated beside it.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
	"github.com/panitw/folio8/folio-go/internal/designer"
)

// pickBaseDocument is a document that declares ONE chain the picked family is
// not, so a pick is a CHANGE rather than a first declaration. That is the
// obligation DW-238's discharge left behind: the control must write the
// variants at the moment it constructs the chain, because nothing later will.
// A test that only ever set the family once could not see a pick that wrote a
// bare face name.
func pickBaseDocument(value string) string {
	return `{
  "assets": {},
  "bands": {
    "content": {"elements": [{"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 40, "value": ` + value + `, "style": {"fontFamily": "body", "fontSize": 12}}]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}`
}

// pickCommands is what the family control sends when an author picks a family
// this release already ships: the chain is DECLARED first, naming the shipped
// face and the cuts that family has, and only then may the property name it —
// `canvas.fontFamilies` is the closed set `style.fontFamily` may reference, so
// the engine forces the order. Two commands and two undo entries, never one.
//
// The `entries` payloads below are the bytes `shipped-face-cuts.ts` produces
// for those two families; `App.test.tsx` asserts the browser emits exactly
// these, and this file asserts what the engine does with them.
func pickCommands(family, entries string) []string {
	return []string{
		`{"kind":"addFontChain","version":1,"name":"` + family + `","entries":` + entries + `}`,
		`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"fontFamily":{"op":"set","value":"` + family + `"}}}`,
	}
}

// robotoPickEntries is the `entries` payload the family control sends for a
// Roboto pick: the shipped face with its cuts, followed by the SAME proposed
// fallback tail the embed path computes — Roboto's catalogue `scripts` is
// `["latin"]`, so the shipped faces for thai and cjk follow it, each declaring
// the cuts it has.
//
// `App.test.tsx` asserts the browser emits exactly these bytes; this file
// asserts what the engine does with them.
const robotoPickEntries = `[` +
	`{"face":"Roboto","bold":"Roboto Bold","italic":"Roboto Italic","boldItalic":"Roboto Bold Italic"},` +
	`{"face":"Noto Sans Thai","bold":"Noto Sans Thai Bold"},` +
	`"Noto Sans SC"]`

// starterRobotoChain reads the shipped starter's own Roboto chain off disk.
//
// It is READ rather than restated because it is the comparand for "a pick must
// not write a chain with less script coverage than the path it replaced", and a
// restated copy would agree with a regression the day someone updated both.
func starterRobotoChain(t *testing.T) []json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(starterTemplatePath)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Fonts map[string][]json.RawMessage `json:"fonts"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	chain, ok := document.Fonts["Roboto"]
	if !ok || len(chain) == 0 {
		t.Fatalf("starter.folio declares no Roboto chain, so the comparison below would be vacuous: %v", document.Fonts)
	}
	return chain
}

// canonicalChain renders a chain as one comparable string with every entry's
// object keys sorted, so the comparison is about what the entries SAY and not
// about the order two writers happened to emit their keys in.
func canonicalChain(t *testing.T, entries []json.RawMessage) string {
	t.Helper()
	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		var any interface{}
		if err := json.Unmarshal(entry, &any); err != nil {
			t.Fatalf("chain entry %s is not JSON: %v", entry, err)
		}
		// json.Marshal sorts an object's keys, which is exactly the
		// canonicalisation wanted here.
		out, err := json.Marshal(any)
		if err != nil {
			t.Fatal(err)
		}
		parts = append(parts, string(out))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func applyAll(t *testing.T, tpl *folio8.Template, commands ...string) {
	t.Helper()
	for _, command := range commands {
		if _, err := designer.ApplyComponentCommand(tpl, []byte(command)); err != nil {
			t.Fatalf("%s: %v", command, err)
		}
	}
}

// documentChains reads the chains back out of the SERIALIZED document — the
// bytes an author would save and reopen — rather than out of the in-memory
// template, so what is asserted is what the file carries.
func documentChains(t *testing.T, tpl *folio8.Template) map[string][]json.RawMessage {
	t.Helper()
	out, err := folio8.SerializeTemplate(tpl)
	if err != nil {
		t.Fatalf("serialize the picked document: %v", err)
	}
	var document struct {
		Assets map[string]json.RawMessage   `json:"assets"`
		Fonts  map[string][]json.RawMessage `json:"fonts"`
	}
	if err := json.Unmarshal(out, &document); err != nil {
		t.Fatalf("decode the picked document: %v", err)
	}
	// AC1's "no asset is embedded, and the document's assets map is untouched",
	// asserted on the file itself.
	if len(document.Assets) != 0 {
		t.Errorf("picking a family the release SHIPS wrote %d asset(s) into the document; naming a face is not carrying it (D-16.5)", len(document.Assets))
	}
	// AND IT STILL LOADS. A pick that wrote a variant naming its own base would
	// author a document this story's own parse narrowing refuses (D-11.2.11) —
	// the product would produce a file it cannot reopen.
	if _, err := folio8.ParseTemplate(out); err != nil {
		t.Fatalf("the document a pick produced does not load again: %v", err)
	}
	return document.Fonts
}

// TestAPickOfAShippedFamilyDeclaresThatFamilysCutsInTheDocument is AC1 and AC2
// end to end, on the document and then on the render.
func TestAPickOfAShippedFamilyDeclaresThatFamilysCutsInTheDocument(t *testing.T) {
	tpl, err := folio8.ParseTemplate([]byte(pickBaseDocument(`"Hamburgefonstiv"`)))
	if err != nil {
		t.Fatalf("parse the base document: %v", err)
	}
	applyAll(t, tpl, pickCommands("Roboto", robotoPickEntries)...)

	chains := documentChains(t, tpl)
	entries, declared := chains["Roboto"]
	if !declared || len(entries) != 3 {
		t.Fatalf("the pick declared %v; want one chain named Roboto carrying the picked face and its proposed fallback tail", chains)
	}
	// ⚠ AND THE WHOLE CHAIN IS THE STARTER'S OWN, ENTRY FOR ENTRY. The declare
	// path replaced an embed that computed a proposed fallback tail, and the
	// first cut of it shipped with NO tail at all: a Roboto pick wrote one entry
	// where the shipped starter's Roboto chain has three, so latin kept working
	// and every Thai and CJK run in the document silently lost its fallback. A
	// pick must never yield a chain with less script coverage than the path it
	// replaced, and for Roboto — `scripts: ["latin"]` — the answer is exactly
	// what `starter.folio` already declares. That file is the comparand rather
	// than a list restated here, so the two cannot drift.
	if got, want := canonicalChain(t, entries), canonicalChain(t, starterRobotoChain(t)); got != want {
		t.Errorf("a Roboto pick writes\n  %s\nand starter.folio declares\n  %s", got, want)
	}
	// EVERY NAME THE ENTRY CARRIES IS A `fonts.Shipped()` KEY, and every variant
	// differs from the base. Both are read off the FILE, so this cannot become a
	// second copy of the table the browser holds.
	shipped := fonts.Shipped()
	var entry struct {
		Face       string `json:"face"`
		Asset      string `json:"asset"`
		Bold       string `json:"bold"`
		Italic     string `json:"italic"`
		BoldItalic string `json:"boldItalic"`
	}
	if err := json.Unmarshal(entries[0], &entry); err != nil {
		t.Fatalf("the pick wrote an entry that is not an object: %s", entries[0])
	}
	if entry.Asset != "" {
		t.Fatalf("the pick wrote an EMBEDDED entry (%q); a shipped family is named, never carried", entry.Asset)
	}
	if _, ok := shipped[entry.Face]; !ok {
		t.Errorf("the pick names the base face %q, which fonts.Shipped() does not supply", entry.Face)
	}
	cuts := 0
	for _, cut := range []struct{ key, name string }{{"bold", entry.Bold}, {"italic", entry.Italic}, {"boldItalic", entry.BoldItalic}} {
		if cut.name == "" {
			continue
		}
		cuts++
		if _, ok := shipped[cut.name]; !ok {
			t.Errorf("the pick declares %s = %q, which fonts.Shipped() does not supply", cut.key, cut.name)
		}
		if cut.name == entry.Face {
			t.Errorf("the pick declares %s naming its OWN base face %q — this story's parse check makes that a load error, so the pick would author a document the product cannot reopen", cut.key, entry.Face)
		}
	}
	// NON-VACUITY. Every check above is over a list, and a pick that wrote a
	// bare `"Roboto"` would satisfy all of them in silence — which is the exact
	// defect this story exists to close.
	if cuts == 0 {
		t.Fatal("the pick declared NO cut at all, so every assertion above was vacuous: a re-picked family still cannot bold")
	}

	// AC2 — AND IT ACTUALLY BOLDS, WITH NO WARNING. The expected face is read
	// back off the document rather than restated: whatever the pick declared as
	// this family's bold cut is what a bolded run must be drawn in.
	applyAll(t, tpl, `{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"bold":{"op":"set","value":true}}}`)
	projection, err := designer.CanvasWithTextPaint(tpl, fonts.Shipped())
	if err != nil {
		t.Fatalf("project the paint: %v", err)
	}
	faces := paintedFaces(projection)
	if len(faces) == 0 {
		t.Fatal("the bolded element painted no fragment, so the face assertion below would be vacuous")
	}
	for _, face := range faces {
		if face != entry.Bold {
			t.Errorf("a bolded element painted in %q, want the cut the pick DECLARED, %q", face, entry.Bold)
		}
	}
	res, err := folio8.Render(tpl, folio8.Data(`{}`), folio8.Params(`{}`), fonts.Shipped())
	if err != nil {
		t.Fatalf("render the picked document: %v", err)
	}
	for _, d := range res.Diagnostics {
		if d.Code == folio8.DiagCodeTextStyleFaceUndeclared {
			t.Errorf("a DECLARED cut earned an absence Warning: %s", d.Message)
		}
	}
}

// TestAPickOfNotoSansSCDeclaresNoCutAndThatIsORDINARY is D-A exercised as the
// permanent shipped condition it is, not as an edge case.
//
// `Noto Sans SC`'s Regular alone is ~10.6 MB, so three instances were ruled out
// of the offline payload — the family has no face at any other weight and never
// will. An entry for it therefore declares nothing, stays a bare face name, and
// a bolded CJK run draws SC Regular and earns AC3's Warning. That is the ruled
// answer, and the Warning is the product telling the truth rather than a defect.
//
// ⚠ THIS IS A SYNTHETIC PICK, AND SAYING SO IS PART OF THE TEST. The family
// control CANNOT make this gesture: D-11.1.5 leaves `Noto Sans`, `Noto Sans
// Thai` and `Noto Sans SC` out of the designer's catalogue entirely, so the
// dropdown never offers them and no author can reach this path through the UI
// as it ships. What is exercised here is the ENGINE'S half of the contract —
// what a cut-less entry does to a render — driven by the commands the control
// would send if it did offer the family. Read as an end-to-end path it would
// overclaim; read as the engine's answer for a family that declares nothing, it
// is exactly the permanent condition D-A describes, and it is the same answer
// the SC entry in every REAL pick's fallback tail gets.
func TestAPickOfNotoSansSCDeclaresNoCutAndThatIsORDINARY(t *testing.T) {
	tpl, err := folio8.ParseTemplate([]byte(pickBaseDocument(`"中文"`)))
	if err != nil {
		t.Fatalf("parse the base document: %v", err)
	}
	applyAll(t, tpl, pickCommands("Noto Sans SC", `["Noto Sans SC"]`)...)

	chains := documentChains(t, tpl)
	entries := chains["Noto Sans SC"]
	if len(entries) != 1 {
		t.Fatalf("the pick declared %v; want one chain named Noto Sans SC with one entry", chains)
	}
	// A FAMILY WITH NO CUT SERIALISES AS A BARE STRING, which is the canonical
	// shape for a variant-free entry — and it is what keeps such a document at
	// version 1.0 rather than raising it for writing the same chain a second way.
	var bare string
	if err := json.Unmarshal(entries[0], &bare); err != nil || bare != "Noto Sans SC" {
		t.Fatalf("the entry is %s, want the bare face name a variant-free entry canonicalises to", entries[0])
	}

	applyAll(t, tpl, `{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"bold":{"op":"set","value":true}}}`)
	projection, err := designer.CanvasWithTextPaint(tpl, fonts.Shipped())
	if err != nil {
		t.Fatalf("project the paint: %v", err)
	}
	faces := paintedFaces(projection)
	if len(faces) == 0 {
		t.Fatal("the bolded CJK element painted no fragment, so the face assertion below would be vacuous")
	}
	for _, face := range faces {
		if face != "Noto Sans SC" {
			t.Errorf("a bolded CJK run painted in %q, want the entry's own base face — no weight is ever synthesized", face)
		}
	}
	// AND THE ABSENCE IS ANNOUNCED. Rendering the base face silently would be
	// the failure AC3 exists to prevent; the Warning is what makes it honest.
	res, err := folio8.Render(tpl, folio8.Data(`{}`), folio8.Params(`{}`), fonts.Shipped())
	if err != nil {
		t.Fatalf("render the CJK document: %v", err)
	}
	warned := 0
	for _, d := range res.Diagnostics {
		if d.Code == folio8.DiagCodeTextStyleFaceUndeclared {
			warned++
			if !strings.Contains(d.Message, "Noto Sans SC") {
				t.Errorf("the absence Warning %q does not name the face the run was actually drawn in", d.Message)
			}
		}
	}
	if warned == 0 {
		t.Fatalf("bolding a run against an entry that declares no bold earned NO Warning: %+v", res.Diagnostics)
	}
}

// TestAPicksSAVEDVERSIONFollowsTheCUTSItDeclares is the consequence nothing
// asserted: the SAME gesture, on two families, saves two different MAJOR
// versions.
//
// A cut carries its entry into the object form, and `fontsRequireMajor` reads
// the same `SerialisesAsObject` predicate the writer does — so a Roboto pick
// stamps `2.0` on a document that was `1.0`, because no 1.x reader can decode
// the chain it now holds. A cut-less pick writes bare face names, changes
// nothing a 1.x reader could not read, and leaves the document at `1.0`.
//
// That is a real and permanent cost of picking a family with cuts, and an
// author is entitled to have it be deliberate rather than incidental. It is
// also the thing a future "just always write the object form" simplification
// would break silently, which is why it is pinned here rather than left to the
// version tests, none of which drive a PICK.
func TestAPicksSAVEDVERSIONFollowsTheCUTSItDeclares(t *testing.T) {
	for _, tc := range []struct{ name, family, entries, want string }{
		{"a family with cuts writes the object form and needs a 2.0 reader", "Roboto", robotoPickEntries, "2.0"},
		{"a family with none writes bare face names and stays readable at 1.0", "Noto Sans SC", `["Noto Sans SC"]`, "1.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tpl, err := folio8.ParseTemplate([]byte(pickBaseDocument(`"Hamburgefonstiv"`)))
			if err != nil {
				t.Fatal(err)
			}
			// THE PRECONDITION IS ASSERTED, or "it saved 1.0" would be a pass
			// for a document that was never anything else.
			if before := savedVersion(t, tpl); before != "1.0" {
				t.Fatalf("the base document is already %s; this test measures what the PICK changes", before)
			}
			applyAll(t, tpl, pickCommands(tc.family, tc.entries)...)
			if got := savedVersion(t, tpl); got != tc.want {
				t.Errorf("a %s pick saves version %q, want %q", tc.family, got, tc.want)
			}
		})
	}
}

// savedVersion reads the version off the SERIALIZED bytes — the string a reader
// of the file would act on, not a field of the in-memory model.
func savedVersion(t *testing.T, tpl *folio8.Template) string {
	t.Helper()
	out, err := folio8.SerializeTemplate(tpl)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(out, &document); err != nil {
		t.Fatal(err)
	}
	return document.Version
}
