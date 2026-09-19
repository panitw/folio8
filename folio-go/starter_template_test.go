package folio8_test

// STORY 11.3 / P7 — THE SHIPPED STARTER TEMPLATE, PARSED BY SOMETHING THAT RUNS.
//
// ⚠ NOTHING THAT RUNS PARSED THIS FILE. Measured before this test existed:
// `git grep starter.folio` returns five non-doc consumers, and the only one
// that reads its BYTES is `folio-go/wasm/cmd/engine/main_test.go`, which is
// `//go:build js && wasm` — absent from `go list ./...` and never built by any
// gate on this machine. `startup-sequence.test.ts` stubs the fetch with
// `new Uint8Array([1,2,3])`; `font-embed-boundary.spec.ts` reads it but is
// COMPILED only; `verify-offline-release.mjs` class-checks that some asset ends
// with `.folio`; the build fingerprints it into gitignored generated output.
//
// SO THE FILE COULD SHIP BROKEN AND EVERY GATE WOULD STAY GREEN: `"1.0"` beside
// object-form entries (the "version that lies" this epic already caught once),
// or a variant typo'd `"Roboto-Bold"` naming a face `fonts.Shipped()` does not
// supply, and nothing would say a word. This is the reader that runs.
//
// IT IS AN EXTERNAL TEST PACKAGE because it imports `folio-go/fonts`, and
// `fonts` imports `folio8` — inside `package folio8` that is an import cycle.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
	"github.com/panitw/folio8/folio-go/internal/designer"
)

const starterTemplatePath = "../folio-designer/public/templates/starter.folio"

// starterChainName is the chain the starter declares and every text element in
// a new document names. Read as a constant rather than discovered, because a
// rename is a change this test should make somebody think about.
const starterChainName = "Roboto"

func loadStarterTemplate(t *testing.T) ([]byte, *folio8.Template) {
	t.Helper()
	raw, err := os.ReadFile(starterTemplatePath)
	if err != nil {
		t.Fatalf("read the shipped starter template: %v", err)
	}
	tpl, err := folio8.ParseTemplate(raw)
	if err != nil {
		t.Fatalf("the shipped starter template does not load: %v", err)
	}
	return raw, tpl
}

// TestTheShippedStarterDeclaresOnlyCutsTheEngineSupplies is the typo guard, and
// the reason this file exists at all. Every face name the starter's chain names
// — the entries' own faces AND their declared style variants — is intersected
// with `fonts.Shipped()`'s keys, so `"Roboto-Bold"` for `"Roboto Bold"` is a red
// here instead of a Warning nobody sees in a document nobody has opened yet.
//
// D-11.2.11 IS CHECKED HERE TOO, one story before its load error exists: a
// variant naming its own base face is a placeholder pretending to be a
// declaration, and 11.4 will refuse it at parse. Ship one now and 11.4 breaks
// the starter.
func TestTheShippedStarterDeclaresOnlyCutsTheEngineSupplies(t *testing.T) {
	_, tpl := loadStarterTemplate(t)
	projection, err := designer.Canvas(tpl)
	if err != nil {
		t.Fatalf("project the shipped starter: %v", err)
	}
	shipped := fonts.Shipped()
	var chain *designer.CanvasFontChain
	for i := range projection.FontChains {
		if projection.FontChains[i].Name == starterChainName {
			chain = &projection.FontChains[i]
		}
	}
	if chain == nil {
		t.Fatalf("the starter declares no chain named %q; its chains are %v", starterChainName, projection.FontFamilies)
	}
	// NON-VACUITY, FIRST AND BY MEASUREMENT. Every assertion below is over a
	// list, and an empty list satisfies all of them. The starter declares three
	// entries and at least one variant; a file that lost its variants would
	// otherwise pass this test in silence, which is the exact failure it exists
	// to prevent.
	if len(chain.Entries) != 3 {
		t.Fatalf("the starter's %q chain projects %d entries, want 3", starterChainName, len(chain.Entries))
	}
	declared := 0
	for i, entry := range chain.Entries {
		if entry.AssetKey != "" {
			t.Fatalf("entry %d is an EMBEDDED entry; the starter carries no assets and its variants are FontSet face names (AD-8): %+v", i, entry)
		}
		if _, ok := shipped[entry.Face]; !ok {
			t.Errorf("entry %d names the base face %q, which fonts.Shipped() does not supply", i, entry.Face)
		}
		for _, variant := range []struct{ key, name string }{
			{"bold", entry.Bold}, {"italic", entry.Italic}, {"boldItalic", entry.BoldItalic},
		} {
			if variant.name == "" {
				continue
			}
			declared++
			if _, ok := shipped[variant.name]; !ok {
				t.Errorf("entry %d (%q) declares %s = %q, which fonts.Shipped() does not supply — a document declaring a cut the engine cannot draw paints the base face and warns, for every author who opens a new file", i, entry.Face, variant.key, variant.name)
			}
			if variant.name == entry.Face {
				t.Errorf("entry %d (%q) declares %s naming its OWN BASE FACE — that is D-11.2.11's placeholder, and Story 11.4's parse check turns it into a load error on the shipped starter", i, entry.Face, variant.key)
			}
		}
	}
	if declared < 1 {
		t.Fatalf("the starter's chain declares NO style variant at all, so every assertion above was vacuous — a new document cannot bold (D-11.0.1)")
	}
	// AND THE PROJECTED VARIANTS ARE THE ONES THE FILE DECLARES, read back off
	// the file's own bytes rather than restated here, so this cannot drift into
	// a second copy of the starter's contents.
	var document struct {
		Fonts map[string][]json.RawMessage `json:"fonts"`
	}
	if err := json.Unmarshal(mustReadStarter(t), &document); err != nil {
		t.Fatalf("decode the starter's fonts block: %v", err)
	}
	if got := len(document.Fonts[starterChainName]); got != len(chain.Entries) {
		t.Errorf("the file declares %d entries in %q and the projection carries %d", got, starterChainName, len(chain.Entries))
	}
	for i, entry := range document.Fonts[starterChainName] {
		for _, key := range []string{"bold", "italic", "boldItalic"} {
			var object map[string]string
			if json.Unmarshal(entry, &object) != nil {
				continue // a bare string entry declares nothing, which is legal
			}
			projected := map[string]string{"bold": chain.Entries[i].Bold, "italic": chain.Entries[i].Italic, "boldItalic": chain.Entries[i].BoldItalic}[key]
			if object[key] != projected {
				t.Errorf("entry %d declares %s = %q in the file and projects %q", i, key, object[key], projected)
			}
		}
	}
}

// TestTheShippedStarterDeclaresTheVersionItsContentRequires is the OTHER way
// this file could ship broken with every gate green: object-form entries beside
// `"version": "1.0"`. `fontsRequireMajor` raises any document with an
// object-form entry to 2.0, so the serializer would stamp 2.0 on the first
// save — and the committed file would have been claiming a version no 1.x
// reader can honour, which is the "version that lies" this epic already caught
// once at the predicate level.
//
// IT IS ASSERTED AS AN AGREEMENT, NOT AGAINST A LITERAL: whatever version the
// engine derives from this content is the version the file must declare, so the
// check survives the starter gaining or losing a feature.
func TestTheShippedStarterDeclaresTheVersionItsContentRequires(t *testing.T) {
	raw, tpl := loadStarterTemplate(t)
	canonical, err := folio8.SerializeTemplate(tpl)
	if err != nil {
		t.Fatalf("serialize the shipped starter: %v", err)
	}
	declared, derived := versionOf(t, raw), versionOf(t, canonical)
	if declared != derived {
		t.Errorf("the shipped starter declares version %q and its own content requires %q — the committed file claims a version its content does not honour, and the first save would rewrite it", declared, derived)
	}
	// Positive control: the read really found a version, so an equality between
	// two empty strings cannot pass for agreement.
	if declared == "" {
		t.Fatal("read no version out of the shipped starter")
	}
}

// TestANewDocumentFromTheStarterCanActuallyBold is AC5, end to end and on the
// REAL file: create a text element, set bold through the command layer, and
// read the face the ENGINE resolved off the paint fragment. It is the one
// assertion that fails if any link in the chain is wrong — the file's variant,
// the parser, the resolver, or the shipped face set — and it names the face
// rather than merely asserting the fragment is non-empty.
func TestANewDocumentFromTheStarterCanActuallyBold(t *testing.T) {
	_, tpl := loadStarterTemplate(t)
	if _, err := designer.ApplyComponentCommand(tpl, []byte(`{"kind":"createComponent","version":1,"type":"text","band":"content","x":40,"y":40,"width":200,"height":24,"snap":false}`)); err != nil {
		t.Fatalf("create a text element in a new document: %v", err)
	}
	set := `{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"value":{"op":"set","value":"Heading"},"fontFamily":{"op":"set","value":"` + starterChainName + `"},"bold":{"op":"set","value":true}}}`
	if _, err := designer.ApplyComponentCommand(tpl, []byte(set)); err != nil {
		t.Fatalf("bold the new element: %v", err)
	}
	projection, err := designer.CanvasWithTextPaint(tpl, fonts.Shipped())
	if err != nil {
		t.Fatalf("project the paint: %v", err)
	}
	faces := paintedFaces(projection)
	if len(faces) == 0 {
		t.Fatal("the bolded element painted no fragment at all, so the face assertion below would be vacuous")
	}
	// THE FACE IS DERIVED FROM THE FILE, not restated: whatever the starter
	// declares as Roboto's bold cut is what a bolded Latin run must be drawn in.
	want := declaredBoldOf(t, starterChainName)
	for _, face := range faces {
		if face != want {
			t.Errorf("a bolded element in a new document painted in %q, want the chain's declared bold cut %q — pressing B in a brand-new file must reach a REAL bold face, never a synthesised one", face, want)
		}
	}
}

func mustReadStarter(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(starterTemplatePath)
	if err != nil {
		t.Fatalf("read the shipped starter template: %v", err)
	}
	return raw
}

func versionOf(t *testing.T, document []byte) string {
	t.Helper()
	var decoded struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(document, &decoded); err != nil {
		t.Fatalf("decode a version out of %d bytes: %v", len(document), err)
	}
	return decoded.Version
}

// declaredBoldOf reads the chain's first entry's declared bold cut out of the
// FILE, so the paint assertion compares the engine's answer against the
// document rather than against a literal repeated from it.
func declaredBoldOf(t *testing.T, chainName string) string {
	t.Helper()
	var document struct {
		Fonts map[string][]json.RawMessage `json:"fonts"`
	}
	if err := json.Unmarshal(mustReadStarter(t), &document); err != nil {
		t.Fatalf("decode the starter's fonts block: %v", err)
	}
	// An entry is EITHER a bare string or an object (folio-format.md's two
	// shapes), and only the object form can declare a variant, so a failed
	// object decode is a legal entry that declares none rather than an error.
	for _, entry := range document.Fonts[chainName] {
		var object struct {
			Bold string `json:"bold"`
		}
		if json.Unmarshal(entry, &object) == nil && object.Bold != "" {
			return object.Bold
		}
	}
	t.Fatalf("the starter's %q chain declares no bold cut, so a new document cannot bold at all", chainName)
	return ""
}

func paintedFaces(projection designer.CanvasProjection) []string {
	var faces []string
	for _, component := range projection.Components {
		if component.TextPaint == nil {
			continue
		}
		for _, line := range component.TextPaint.Lines {
			for _, fragment := range line.Fragments {
				if strings.TrimSpace(fragment.Text) == "" {
					continue
				}
				faces = append(faces, fragment.Face)
			}
		}
	}
	return faces
}
