package folio8

import (
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/designer"
)

// This file pins `projectFontChainEntry` (page_setup.go) — the EMBEDDED
// branch, which had no Go test at all when Story 8.3 first landed.
//
// WHY THAT WAS DANGEROUS, MEASURED. Deleting `out.Family = entry.AssetKey`
// left the whole Go suite green. With it gone, a font asset carrying no
// `font.family` projects `family: ""`, and the designer's own guard rejects
// exactly that (`isFontChainEntry`: `if (assetKey.length > 0) return
// family.length > 0`) — so `isCanvas` returns false, `parseInbound` returns
// undefined, `engine-client` calls `worker.terminate()`, and the canvas is
// permanently blank with no element id and nothing to attribute it to. That is
// precisely the failure mode D-8.2.8 and DW-82 exist to prevent, shipping
// green.
//
// The TypeScript side of this contract is tested in engine-protocol.test.ts
// and the key sets are tied together by canvas_projection_wire_test.go. What
// neither of those can see is whether GO EVER PRODUCES the shape the guard
// requires — a hand-written TypeScript fixture proves the guard accepts a
// well-formed entry, not that the engine emits one. This file is that half.

// canvasChainDoc is a one-text-element document carrying one font asset whose
// `font` record carries `displayKeys` — the record's DISPLAY half, written as
// the inner text of a JSON object with no braces — and whose `body` chain names
// the shipped face and then the asset.
//
// THE REQUIRED LICENCE KEYS ARE ALWAYS SPLICED IN, and the signature changed
// from "the whole record verbatim" to "the display half" for that reason
// (Story 8.6). The chain here NAMES the asset, so the document is an embedded
// face and its record must state its terms or the document does not load at
// all. Every case below is about what the PROJECTION does with a missing,
// null or empty family — a question that is only reachable on a document that
// loads, so the licence half is supplied rather than left to each case to
// remember. Passing an empty displayKeys is the "no display identity at all"
// case, which is as close to the old "no font record at all" row as a
// referenced asset can now come.
func canvasChainDoc(t *testing.T, displayKeys string) string {
	t.Helper()
	record := "{" + requiredLicenceKeys
	if displayKeys != "" {
		record += ", " + displayKeys
	}
	record += "}"
	source := embeddedFontTemplateJSON()
	// Replace the generated document's own `font` record, which is bracketed
	// in canonical (sorted) key order by `data` before it and `mediaType`
	// after it. Sorted order is a property AD-9 guarantees, so this is a
	// bracket rather than a brace count over ~47 KB of base64.
	const open, next = "      \"font\": {", "      \"mediaType\":"
	start := strings.Index(source, open)
	end := strings.Index(source, next)
	if start < 0 || end < 0 || end < start {
		t.Fatalf("fixture assumption violated: the generated document's font record is not bracketed by %q and %q", open, next)
	}
	return source[:start] + "      \"font\": " + record + ",\n" + source[end:]
}

// projectedChainEntries runs a document through the REAL projection entry
// point — the one that reaches the browser — and returns the `body` chain's
// entries.
func projectedChainEntries(t *testing.T, source string) []designer.CanvasFontChainEntry {
	t.Helper()
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	projection, err := canvas(tpl)
	if err != nil {
		t.Fatalf("Canvas: %v", err)
	}
	for _, chain := range projection.FontChains {
		if chain.Name == "body" {
			return chain.Entries
		}
	}
	t.Fatalf("the projection carries no chain named body: %+v", projection.FontChains)
	return nil
}

// TestProjectedEmbeddedEntryCarriesTheDiscriminantAndTheRecord asserts the
// whole projected entry, not one field of it. Asserting the struct is what
// keeps a family or style LEAKING onto a named face visible: the browser
// rejects that too (`a named face carries no family and no style`), and it
// blanks the canvas the same way an absent family does.
func TestProjectedEmbeddedEntryCarriesTheDiscriminantAndTheRecord(t *testing.T) {
	key := embeddedFontAssetKey()
	for _, tc := range []struct {
		name   string
		record string
		want   designer.CanvasFontChainEntry
	}{
		{
			name:   "family and style from the record",
			record: `"family": "Noto Sans Thai", "style": "Regular"`,
			want:   designer.CanvasFontChainEntry{AssetKey: key, Family: "Noto Sans Thai", Style: "Regular"},
		},
		{
			// The style is genuinely optional, and an absent one projects
			// empty — which the browser accepts, unlike an empty family.
			name:   "family only",
			record: `"family": "Noto Sans Thai"`,
			want:   designer.CanvasFontChainEntry{AssetKey: key, Family: "Noto Sans Thai"},
		},
		{
			// THE FALLBACK, and the whole reason this file exists. No
			// `font.family`, so the ASSET KEY is projected as the family. The
			// engine decides what the panel shows; the browser is never handed
			// an empty name it would have to invent a rule for.
			name:   "no family — the asset key is the fallback",
			record: `"style": "Regular"`,
			want:   designer.CanvasFontChainEntry{AssetKey: key, Family: key, Style: "Regular"},
		},
		{
			// STORY 8.6 REPLACED TWO ROWS WITH THIS ONE, and the reason is
			// the rule and not the tidying. "No font record at all" and "an
			// empty record" were both legal documents when the record was
			// wholly optional; on a chain-NAMED asset neither is a document
			// any more — it is refused at load — so keeping them would have
			// been keeping two rows that assert what the projection does with
			// a document the loader never yields. What survives of them is
			// the reachable case: a record that carries its TERMS and no
			// display identity whatever, which is where the asset-key
			// fallback still has to hold.
			name:   "the required terms and no display identity at all",
			record: "",
			want:   designer.CanvasFontChainEntry{AssetKey: key, Family: key},
		},
		{
			// An explicit null is a legal, round-trippable spelling in the
			// FILE, and it means nothing to DISPLAY — so it falls back the
			// same way absence does. The distinction survives in the document
			// (Presence round-trips it); it simply does not reach the panel.
			name:   "an explicitly null family",
			record: `"family": null, "style": "Regular"`,
			want:   designer.CanvasFontChainEntry{AssetKey: key, Family: key, Style: "Regular"},
		},
		{
			// An empty-STRING family is a name the panel cannot draw, so it
			// falls back too. Without this the browser's `family.length > 0`
			// rejects the snapshot.
			name:   "an empty-string family",
			record: `"family": "", "style": "Regular"`,
			want:   designer.CanvasFontChainEntry{AssetKey: key, Family: key, Style: "Regular"},
		},
		{
			name:   "an explicitly null style",
			record: `"family": "Noto Sans Thai", "style": null`,
			want:   designer.CanvasFontChainEntry{AssetKey: key, Family: "Noto Sans Thai"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := projectedChainEntries(t, canvasChainDoc(t, tc.record))
			if len(entries) != 2 {
				t.Fatalf("projected %d entries, want 2: %+v", len(entries), entries)
			}
			// The NAMED face first — asserted whole, so a display string
			// leaking onto it is caught here rather than in the browser.
			if want := (designer.CanvasFontChainEntry{Face: "Noto Sans"}); entries[0] != want {
				t.Errorf("named-face entry = %+v, want %+v — a named face's name IS its identity and it carries no family or style", entries[0], want)
			}
			if entries[1] != tc.want {
				t.Errorf("embedded entry = %+v, want %+v", entries[1], tc.want)
			}
			// The invariant the designer's guard turns on, stated directly so
			// its violation is named rather than inferred from a struct diff.
			if entries[1].Family == "" {
				t.Error("an embedded entry projected an EMPTY family — the designer's guard rejects that, parseInbound returns undefined, the worker is terminated and the canvas goes permanently blank (D-8.2.8, DW-82)")
			}
			if (entries[1].Face == "") == (entries[1].AssetKey == "") {
				t.Errorf("exactly one of Face and AssetKey must be non-empty, got %+v", entries[1])
			}
		})
	}
}

// TestProjectedEntryStringsAreBounded closes the other half of the bound. Only
// the FACE arm was exercised before (canvas_body_text_bounds_test.go's "fonts
// chain entry" row); Family and Style are equally on the wire, the browser
// bounds all four, and a bound applied to one field of four is a bound on
// nothing.
//
// The AssetKey arm is deliberately absent and that is not an oversight: a key
// is 64 hex characters by the format's own load rule (isSHA256HexKey), so no
// loadable document can carry one over the bound — a test would have to build
// a Document the loader refuses, which asserts something about a state the
// engine cannot be in.
func TestProjectedEntryStringsAreBounded(t *testing.T) {
	long := strings.Repeat("f", maxCanvasPropertyString+1)
	atLimit := strings.Repeat("f", maxCanvasPropertyString)

	for _, tc := range []struct {
		name    string
		record  string
		refused bool
	}{
		{"family at the limit", `"family": "` + atLimit + `"`, false},
		{"family over the limit", `"family": "` + long + `"`, true},
		{"style at the limit", `"family": "Noto Sans Thai", "style": "` + atLimit + `"`, false},
		{"style over the limit", `"family": "Noto Sans Thai", "style": "` + long + `"`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tpl, err := ParseTemplate([]byte(canvasChainDoc(t, tc.record)))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			_, cerr := canvas(tpl)
			if tc.refused {
				if cerr == nil {
					t.Fatal("a projected string over the bound must be REFUSED with a stated reason, never silently cut")
				}
				if !strings.Contains(cerr.Error(), "font chain entry exceeds the projection bound") {
					t.Errorf("Canvas refused with %q, want the projection-bound reason", cerr)
				}
				return
			}
			if cerr != nil {
				t.Fatalf("a projected string AT the bound must be accepted, got: %v", cerr)
			}
		})
	}
}

// STORY 11.3 / DW-239 — THE DECLARED STYLE VARIANTS, PROJECTED.
//
// The panel could not see that a chain declares a bold cut, so it could not
// honour the epic's rule that an ABSENT cut is stated rather than shown as a
// reachable on-state. These three keys are that read-back, and this is the Go
// half of the same contract the file above already keeps for `family`/`style`:
// engine-protocol.test.ts proves the browser's guard ACCEPTS the shape, and
// only a test here can prove the engine EMITS it.
//
// WHAT IT MUST BE, AND THE TRAP IT MUST NOT FALL INTO. The projection copies
// what the DOCUMENT declares, verbatim (D-11.2.1 / D-11.2.2). Every row below
// therefore names its expected value as a literal the fixture also spells,
// which is the only way a `Face + " Bold"` implementation is visibly wrong; a
// row asserting `entry.Bold != ""` would pass over one.
//
// AND THE NAMESPACES DO NOT CROSS (AD-8). A `face` entry's variants are FontSet
// FACE NAMES; an `asset` entry's are `assets` KEYS. The embedded row asserts
// the projected `bold` is the second asset's KEY — not its family, not its
// style, not a name derived from either.

// variantFixtureSecondKey and variantFixtureSecondData are a SECOND hand-built
// 156-byte sfnt and its digest, copied from internal/template's own variant
// fixtures, because a chain entry with a variant ASSET KEY needs a document
// carrying two faces — the loader refuses a variant naming no asset, and that
// refusal is itself pinned in that package.
const variantFixtureSecondKey = "35573263bb78c4a0b0866ff63489bcfeb36b56ac2abe42206967541ba829eea7"

const variantFixtureSecondData = `[
        "AAEAAAADACAABAAQY21hcAAAAAAAAAA8AAAAIGdseWYAAAAAAAAAXAAAACBoZWFkAAAAAAAAAHwA",
        "AAAgRklYVFVSRTFGSVhUVVJFMUZJWFRVUkUxRklYVFVSRTFGSVhUVVJFMUZJWFRVUkUxRklYVFVS",
        "RTFGSVhUVVJFMUZJWFRVUkUxRklYVFVSRTFGSVhUVVJFMUZJWFRVUkUx"
      ]`

const variantFixtureFirstKey = "cbd7a24e64e08aba9da4edd9343b9eaa629e7c26e722eedf68fd5efe217dbedc"

const variantFixtureFirstData = `[
        "AAEAAAADACAABAAQY21hcAAAAAAAAAA8AAAAIGdseWYAAAAAAAAAXAAAACBoZWFkAAAAAAAAAHwA",
        "AAAgQ01BUERBVEFDTUFQREFUQUNNQVBEQVRBQ01BUERBVEFHTFlGREFUQUdMWUZEQVRBR0xZRkRB",
        "VEFHTFlGREFUQUhFQUREQVRBSEVBRERBVEFIRUFEREFUQUhFQUREQVRB"
      ]`

// variantChainDoc is a two-asset document whose `body` chain is written
// verbatim by the caller, so a row can put any legal entry shape in it. The
// assets map is emitted in sorted key order (AD-9), and the second key sorts
// first.
func variantChainDoc(chainBody string) string {
	return `{
  "assets": {
    "` + variantFixtureSecondKey + `": {
      "data": ` + variantFixtureSecondData + `,
      "font": {
        "copyright": "Copyright 2026 The Folio Fixture Authors",
        "family": "Second Sans",
        "licence": "SIL Open Font License 1.1",
        "licenceText": "This fixture face is licensed under the SIL Open Font License, Version 1.1.",
        "source": "hand-built 156-byte sfnt — a fixture, not a face",
        "style": "Bold"
      },
      "mediaType": "font/ttf"
    },
    "` + variantFixtureFirstKey + `": {
      "data": ` + variantFixtureFirstData + `,
      "font": {
        "copyright": "Copyright 2026 The Folio Fixture Authors",
        "family": "Maximal Sans",
        "licence": "SIL Open Font License 1.1",
        "licenceText": "This fixture face is licensed under the SIL Open Font License, Version 1.1.",
        "source": "hand-built 156-byte sfnt — a fixture, not a face",
        "style": "Regular"
      },
      "mediaType": "font/ttf"
    }
  },
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 40, "value": "v", "style": {"fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ` + chainBody + `},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "2.0"
}`
}

func TestProjectedEntryCarriesTheDeclaredStyleVariantsVerbatim(t *testing.T) {
	for _, tc := range []struct {
		name  string
		chain string
		want  []designer.CanvasFontChainEntry
	}{
		{
			// A BARE STRING DECLARES NOTHING, and all three keys are still
			// projected — as "" — because they are never omitempty. A key that
			// appeared only for entries that happen to declare a variant is a
			// key the browser's hasExactKeys rejects for every other document,
			// and the symptom is a blank canvas.
			name:  "a bare face entry declares no variant",
			chain: `["Noto Sans"]`,
			want:  []designer.CanvasFontChainEntry{{Face: "Noto Sans"}},
		},
		{
			name:  "a face entry declaring all three",
			chain: `[{"bold": "Noto Sans Bold", "boldItalic": "Noto Sans Bold Italic", "face": "Noto Sans", "italic": "Noto Sans Italic"}]`,
			want: []designer.CanvasFontChainEntry{{
				Face: "Noto Sans", Bold: "Noto Sans Bold", Italic: "Noto Sans Italic", BoldItalic: "Noto Sans Bold Italic",
			}},
		},
		{
			// THE ROW THE STARTER TEMPLATE IS: a bold and no italic. The two
			// absences travel as "" beside a present bold, which is what lets
			// the panel state "this family has no italic face" for exactly one
			// of its two controls.
			name:  "a face entry declaring bold only",
			chain: `[{"bold": "Noto Sans Thai Bold", "face": "Noto Sans Thai"}]`,
			want:  []designer.CanvasFontChainEntry{{Face: "Noto Sans Thai", Bold: "Noto Sans Thai Bold"}},
		},
		{
			// AD-8: AN EMBEDDED ENTRY'S VARIANT IS AN ASSETS KEY. The expected
			// value is the second asset's KEY — deliberately not "Second Sans",
			// which is that asset's display family and is what a projection
			// that crossed the two namespaces would have produced.
			name:  "an embedded entry declaring a variant asset key",
			chain: `[{"asset": "` + variantFixtureFirstKey + `", "bold": "` + variantFixtureSecondKey + `"}]`,
			want: []designer.CanvasFontChainEntry{{
				AssetKey: variantFixtureFirstKey, Family: "Maximal Sans", Style: "Regular", Bold: variantFixtureSecondKey,
			}},
		},
		{
			// MIXED KINDS IN ONE CHAIN, in the document's own authored order,
			// so a projection that read the variants off the wrong entry is
			// caught rather than being invisible in a one-entry chain.
			name:  "a mixed chain keeps each entry's own variants",
			chain: `[{"bold": "Noto Sans Bold", "face": "Noto Sans"}, {"asset": "` + variantFixtureFirstKey + `", "boldItalic": "` + variantFixtureSecondKey + `"}, "Noto Sans SC"]`,
			want: []designer.CanvasFontChainEntry{
				{Face: "Noto Sans", Bold: "Noto Sans Bold"},
				{AssetKey: variantFixtureFirstKey, Family: "Maximal Sans", Style: "Regular", BoldItalic: variantFixtureSecondKey},
				{Face: "Noto Sans SC"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := projectedChainEntries(t, variantChainDoc(tc.chain))
			if len(entries) != len(tc.want) {
				t.Fatalf("projected %d entries, want %d: %+v", len(entries), len(tc.want), entries)
			}
			for i, want := range tc.want {
				if entries[i] != want {
					t.Errorf("entry %d = %+v, want %+v — the projection copies the entry's declared variants VERBATIM: no name is constructed, parsed or inferred (D-11.2.1)", i, entries[i], want)
				}
			}
		})
	}
}

// TestAProjectedVariantIsBoundedLikeEveryOtherProjectedString closes the half
// of the bound the three new fields opened. maxCanvasPropertyString is applied
// to every string on this wire, and a bound applied to four of seven fields is
// a bound on nothing — the sentence projectFontChainEntry's own comment makes.
func TestAProjectedVariantIsBoundedLikeEveryOtherProjectedString(t *testing.T) {
	long := strings.Repeat("N", maxCanvasPropertyString+1)
	atLimit := strings.Repeat("N", maxCanvasPropertyString)
	for _, tc := range []struct {
		name    string
		chain   string
		refused bool
	}{
		{"bold at the limit", `[{"bold": "` + atLimit + `", "face": "Noto Sans"}]`, false},
		{"bold over the limit", `[{"bold": "` + long + `", "face": "Noto Sans"}]`, true},
		{"italic over the limit", `[{"face": "Noto Sans", "italic": "` + long + `"}]`, true},
		{"boldItalic over the limit", `[{"boldItalic": "` + long + `", "face": "Noto Sans"}]`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tpl, err := ParseTemplate([]byte(variantChainDoc(tc.chain)))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			_, cerr := canvas(tpl)
			if tc.refused {
				if cerr == nil {
					t.Fatal("a projected variant over the bound must be REFUSED with a stated reason, never silently cut")
				}
				if !strings.Contains(cerr.Error(), "font chain entry exceeds the projection bound") {
					t.Errorf("Canvas refused with %q, want the projection-bound reason", cerr)
				}
				return
			}
			if cerr != nil {
				t.Fatalf("a projected variant AT the bound must be accepted, got: %v", cerr)
			}
		})
	}
}
