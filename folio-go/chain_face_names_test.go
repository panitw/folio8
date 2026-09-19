package folio8

import (
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/template"
)

// This file pins `chainFaceNames` (render.go) — THE one boundary between the
// document's chain and the render path's face-name list.
//
// IT LIVES HERE, IN `package folio8`, BECAUSE NOWHERE ELSE CAN. The function's
// consequences are consequences at Render and Validate, and both are exported
// from this package; internal/template is a different package that cannot call
// either, so its tests can pin the FORMAT's behaviour and nothing about the
// render path's. Story 8.3's first draft of the comment on chainFaceNames
// claimed these properties were "pinned by test (fonts_embedded_test.go)" —
// they were not, and could not have been. Deleting the `Embedded()` arm
// outright left the whole Go suite green.
//
// WHAT THE BOUNDARY DOES CHANGED AT STORY 8.4, AND SO DID THESE PINS. Until
// 8.4 an embedded entry was DROPPED here; now it is mapped to the reserved
// face name embeddedFaceName derives from its ASSET KEY, and the fontCache
// resolves that name from the document's own assets. The two mutation pins
// below are re-pointed at the new boundary rather than retired, because the
// defects they catch are the same two defects — they have only swapped which
// side of the boundary is correct.
//
// TWO MUTATIONS, because they are caught in different places and knowing which
// is which is the difference between coverage and the feeling of it:
//   - DROPPING the embedded arm — a straight regression to the 8.3 boundary —
//     shortens the chain. The unit half below catches it on length and order;
//     the end-to-end halves catch it because the document then has no face
//     that covers Thai at all.
//   - Emitting the BARE `entry.AssetKey` instead of the derived name — a
//     64-character digest reaching the render path as an unqualified face name,
//     which is what would let a caller's FontSet shadow a document's carried
//     face — is caught by TestEmbeddedFaceNameIsDerivedFromTheAssetKey and by
//     TestEmbeddedFaceWinsOverACollidingFontSetKey.

// embeddedChainDoc is a one-text-element document whose `body` chain is
// `chain` verbatim, carrying the same font asset fixtures/embedded-font/ does
// so an `{"asset": …}` entry has something real to name.
func embeddedChainDoc(t *testing.T, chain string) string {
	t.Helper()
	return embeddedFontsBlockDoc(t, `"body": `+chain)
}

// embeddedFontsBlockDoc is the same splice for a WHOLE fonts block, so a test
// can give one document several chains. embeddedChainDoc is the one-chain
// case of it.
func embeddedFontsBlockDoc(t *testing.T, fontsBody string) string {
	t.Helper()
	source := embeddedFontTemplateJSON()
	// Spliced between the two keys that BRACKET the fonts block in canonical
	// (sorted) order — `bands` before it, `locale` after it — rather than by
	// counting braces. Brace counting over a document carrying ~47 KB of
	// base64 is a second JSON parser written badly; the sort order is a
	// property AD-9 guarantees.
	const open, next = "  \"fonts\": {", "  \"locale\":"
	start := strings.Index(source, open)
	end := strings.Index(source, next)
	if start < 0 || end < 0 || end < start {
		t.Fatalf("fixture assumption violated: the generated document's fonts block is not bracketed by %q and %q", open, next)
	}
	return source[:start] + "  \"fonts\": {" + fontsBody + "},\n" + source[end:]
}

// embeddedChainDocText is embeddedChainDoc with the drawn text substituted
// too. The fixture's own text is pure Thai (Story 8.4, D-8.4.4b), which is the
// SHARP witness for "the carried face is the only one that can draw this" and
// is exactly the wrong input for an ORDER assertion — the carried face covers
// both scripts, so a chain's order only shows up in mixed text.
func embeddedChainDocText(t *testing.T, chain, text string) string {
	t.Helper()
	const quoted = `"value": "สัญญา"`
	source := embeddedChainDoc(t, chain)
	if !strings.Contains(source, quoted) {
		t.Fatalf("fixture assumption violated: the generated document does not carry %s", quoted)
	}
	return strings.Replace(source, quoted, `"value": "`+text+`"`, 1)
}

// nonFontAssetKey and nonFontAssetData are the three-by-two-pixel PNG several
// tests in this package already carry, under the key the format's own rule
// produces for it. It is reused rather than minted so no new binary — not even
// an inline one — enters the repository for DW-83's sake.
const nonFontAssetKey = "5a05ad01e89c143b7061b0c93450566568d38a23da9b9c5c9dfe449016433078"

const nonFontAssetData = `"iVBORw0KGgoAAAANSUhEUgAAAAMAAAACCAIAAAASFvFNAAAAGElEQVR42mL6z8DAAMZMEOo/AwMg", "AAD//zwUBf/NjsW5AAAAAElFTkSuQmCC"`

// nonFontChainDoc adds that PNG to the document's assets map and hands back a
// document whose `body` chain is `chain` — so a chain entry can name it.
//
// The asset is a REAL, VALID PNG under a REAL image media type. Rewriting the
// carried FONT's mediaType to `image/png` would not do: image/png is a
// recognised image type, so the loader sniffs the bytes and refuses the
// document at load — which is a different defect entirely, and not the one
// DW-83 is about. DW-83's document is valid: it carries a perfectly good
// image and names it where a face belongs.
func nonFontChainDoc(t *testing.T, chain, text string) string {
	t.Helper()
	return withNonFontAsset(t, embeddedChainDocText(t, chain, text))
}

// withNonFontAsset is the assets-map half of nonFontChainDoc on its own, for a
// test that builds its own fonts block rather than a single `body` chain.
func withNonFontAsset(t *testing.T, source string) string {
	t.Helper()
	const anchor = "  \"assets\": {\n"
	at := strings.Index(source, anchor)
	if at < 0 {
		t.Fatal("fixture assumption violated: the generated document has no assets block")
	}
	// Inserted FIRST because "5a05…" sorts before the font asset's "c945…",
	// keeping the document in the canonical key order AD-9 guarantees.
	// A `font` record ON A PNG reads like a contradiction, and it is exactly
	// what the format now requires — which is worth stating rather than
	// leaving as a puzzle. Story 8.6's rule keys off the CHAIN ENTRY, not off
	// the bytes: an asset a chain names by {"asset": key} is a declared
	// embedded face and must state its terms, and the loader asks that
	// BEFORE anything asks what the bytes are. D-1.8.1's promise is intact
	// downstream of it — a document that states its terms and then names a
	// PNG still LOADS and still fails at RENDER, which is the property every
	// test below measures.
	png := "    \"" + nonFontAssetKey + "\": {\n      \"data\": [" + nonFontAssetData + "],\n      \"font\": {" + requiredLicenceKeys + "},\n      \"mediaType\": \"image/png\"\n    },\n"
	return source[:at+len(anchor)] + png + source[at+len(anchor):]
}

// TestChainFaceNamesMapsEveryEntryAndKeepsOrder is the unit-level half: the
// function itself, over every mixture, asserting BOTH that an embedded entry
// becomes its derived name and that nothing else moves.
//
// The order assertion is not decoration. A converter written as "walk the
// entries" is trivially order-preserving; one written as "partition, then
// concatenate" is not, and the two are indistinguishable from a chain whose
// embedded entry happens to sit last. Every case here puts one somewhere else.
func TestChainFaceNamesMapsEveryEntryAndKeepsOrder(t *testing.T) {
	face := template.FaceEntry
	asset := template.AssetEntry
	k1, k2 := embeddedFaceName("k1"), embeddedFaceName("k2")
	for _, tc := range []struct {
		name  string
		chain []template.FontChainEntry
		want  []string
	}{
		{"no entries", nil, []string{}},
		{"named faces only", []template.FontChainEntry{face("A"), face("B"), face("C")}, []string{"A", "B", "C"}},
		{"embedded first", []template.FontChainEntry{asset("k1"), face("A"), face("B")}, []string{k1, "A", "B"}},
		{"embedded in the middle", []template.FontChainEntry{face("A"), asset("k1"), face("B")}, []string{"A", k1, "B"}},
		{"embedded last", []template.FontChainEntry{face("A"), face("B"), asset("k1")}, []string{"A", "B", k1}},
		{"two embedded, interleaved", []template.FontChainEntry{asset("k1"), face("A"), asset("k2"), face("B")}, []string{k1, "A", k2, "B"}},
		{"embedded only", []template.FontChainEntry{asset("k1"), asset("k2")}, []string{k1, k2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, styled := chainFaceNames(tc.chain, template.FontStyleRegular)
			if styled != nil {
				t.Fatalf("a chain asked for no variant returned a styled list: %v", styled)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("chainFaceNames(%v) = %v, want %v", tc.chain, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("chainFaceNames(%v) = %v, want %v — the document's authored order is not preserved", tc.chain, got, tc.want)
				}
			}
			// The BARE asset key must never appear as a face name. If it did,
			// the render path would carry a 64-character digest in the same
			// namespace a caller's FontSet keys live in, and the two could
			// shadow one another — the collision AD-8 keeps a document's
			// carried face and a caller's supplied face out of.
			for _, name := range got {
				if name == "k1" || name == "k2" {
					t.Fatalf("an embedded entry's BARE asset key reached the render path as a face name: %v", got)
				}
			}
		})
	}
}

// TestTheTwoChainSlicesAreAligned is P12's guard, at the PRODUCER.
//
// metricsFaceNames and shapeSegments both index `styled` with an index
// derived from `base`/`chain`, and neither checks the lengths — the
// alignment is a stated precondition, not a runtime guard, because a
// guard here would be one for a case no acceptance criterion
// demonstrates. What makes that safe is that chainFaceNames is the
// pair's ONLY producer and builds both in one pass, so this pins the
// producer rather than adding a check to every consumer.
//
// Both halves of the contract are asserted: nil for a regular request
// (nothing is restyled, so there is nothing to carry), and exact
// same-length same-order for every other request — including entries
// that declare NO variant, whose slot must still be present and empty
// rather than skipped, which is the mistake that would silently shift
// every later index by one.
func TestTheTwoChainSlicesAreAligned(t *testing.T) {
	face := template.FaceEntry
	asset := template.AssetEntry
	withBold := func(name, bold string) template.FontChainEntry {
		return template.FontChainEntry{Face: name, Bold: bold}
	}
	for _, chain := range [][]template.FontChainEntry{
		nil,
		{face("A")},
		{face("A"), face("B"), face("C")},
		{withBold("A", "A Bold"), face("B")},
		{face("A"), withBold("B", "B Bold")},
		{asset("k1"), withBold("B", "B Bold"), asset("k2")},
		{withBold("A", "A Bold"), withBold("B", "B Bold")},
	} {
		for _, want := range []template.FontStyle{
			template.FontStyleRegular,
			template.FontStyleBold,
			template.FontStyleItalic,
			template.FontStyleBoldItalic,
		} {
			base, styled := chainFaceNames(chain, want)
			if len(base) != len(chain) {
				t.Fatalf("base list has %d entries for a %d-entry chain", len(base), len(chain))
			}
			if want == template.FontStyleRegular {
				if styled != nil {
					t.Fatalf("a regular request produced a styled list %v — nil is the contract, and every consumer branches on it", styled)
				}
				continue
			}
			if len(styled) != len(base) {
				t.Fatalf("styled list has %d entries against %d base entries for chain %v — the index that ties them would read the wrong entry's variant", len(styled), len(base), chain)
			}
			// And the correspondence is by INDEX, entry for entry: an
			// entry declaring nothing for this request contributes an
			// EMPTY slot, never a skipped one.
			for i, entry := range chain {
				wantVariant := entry.Variant(want)
				if wantVariant != "" && entry.Embedded() {
					wantVariant = embeddedFaceName(wantVariant)
				}
				if styled[i] != wantVariant {
					t.Fatalf("styled[%d] = %q for entry %+v at style %d, want %q", i, styled[i], entry, want, wantVariant)
				}
			}
		}
	}
}

// TestEmbeddedFaceNameIsDerivedFromTheAssetKey pins D-8.4.1 at the derivation
// itself: the name comes from the ASSET KEY and from nothing else.
//
// It is a separate test from the mapping above because the mapping would pass
// under a derivation that read `font.family` — the entry would still be
// converted, still in order, and still not the bare key. That derivation is
// precisely the one AD-8 forbids: a document's "Inter" and a caller's "Inter"
// would collide in one namespace.
func TestEmbeddedFaceNameIsDerivedFromTheAssetKey(t *testing.T) {
	key := embeddedFontAssetKey()
	name := embeddedFaceName(key)
	if !strings.Contains(name, key) {
		t.Fatalf("the embedded face name %q does not carry its asset key %q", name, key)
	}
	if name == key {
		t.Fatal("the embedded face name is the bare asset key — it must be qualified, or it shares a namespace with the caller's FontSet keys")
	}
	// The document's own `font.family` is "Noto Sans Thai", and a shipped face
	// of that exact name is in the FontSet. The derived name must contain
	// neither, or the two could substitute for one another.
	for _, family := range []string{"Noto Sans Thai", "Noto Sans"} {
		if strings.Contains(name, family) {
			t.Errorf("the embedded face name %q is derived from font.family %q — AD-8/D-8.4.1: the ASSET KEY decides", name, family)
		}
	}
	// Length first: under the pre-8.4 boundary this is EMPTY, and indexing it
	// would panic and take the whole test binary down with it — which hides
	// every other failure the same mutation causes.
	got, _ := chainFaceNames([]template.FontChainEntry{template.AssetEntry(key)}, template.FontStyleRegular)
	if len(got) != 1 || got[0] != name {
		t.Errorf("the boundary and the derivation disagree about an embedded entry's face name: chainFaceNames = %v, want [%s]", got, name)
	}
}

// pdfEscapedEmbeddedFaceName is the derived face name as internal/pdf spells
// it in a resource dictionary and a content stream. pdfNameEscape keeps
// [A-Za-z0-9_-] and drops everything else, so the derivation's separator goes
// and the asset key's 64 hex characters stay — which is what makes the
// produced PDF itself a witness to WHICH face drew the page, by identity and
// never by a count of embedded programs.
func pdfEscapedEmbeddedFaceName(assetKey string) string {
	var b strings.Builder
	for _, r := range embeddedFaceName(assetKey) {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// renderEmbeddedChainDoc parses and renders one of embeddedChainDoc's
// variants against the SHIPPED FontSet alone, and returns the bytes.
func renderEmbeddedChainDoc(t *testing.T, source string) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Render(tpl, Data(`{}`), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return res.Bytes
}

// TestEmbeddedEntryRendersFromTheDocumentsOwnBytes is the end-to-end half of
// the same claim, and it is the pin that reddens if the embedded arm of
// chainFaceNames is dropped.
//
// The document's text is Thai, the FontSet is the SHIPPED set, and the chain
// is ["Noto Sans", <the carried face>]. Measured from the shipped cmaps:
// NotoSans-Regular covers ZERO codepoints in U+0E00–U+0E7F. So the only face
// that can draw this text is the one the document CARRIES — and if the
// embedded entry contributed nothing, every rune would be uncovered, would be
// omitted from the output, and would raise a missing-glyph Warning.
//
// That is the assertion: a clean render, no diagnostics, and drawn glyphs.
// "It rendered" alone is satisfied by a render that dropped every rune.
func TestEmbeddedEntryRendersFromTheDocumentsOwnBytes(t *testing.T) {
	source := embeddedChainDoc(t, `["Noto Sans", {"asset": "`+embeddedFontAssetKey()+`"}]`)
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Render(tpl, Data(`{}`), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("a chain whose only covering entry is the carried face must render: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Fatalf("rendering from the carried face produced diagnostics — every rune fell through the chain uncovered: %+v", res.Diagnostics)
	}
	// The face the page is drawn with is the DOCUMENT's, named by its derived
	// key, and that key reaches the produced PDF as the font resource name.
	if !strings.Contains(string(res.Bytes), pdfEscapedEmbeddedFaceName(embeddedFontAssetKey())) {
		t.Error("the produced PDF names no font resource derived from the carried asset's key — the page was not drawn with the document's own face")
	}
	// And the shipped Latin face the chain names FIRST drew nothing, because
	// it covers none of this text — so it was never subset and never embedded.
	if strings.Contains(string(res.Bytes), "+NotoSans-Regular") {
		t.Error("the shipped Latin face reached the page, though it covers none of the drawn text")
	}
}

// TestEmbeddedFaceWinsOverACollidingFontSetKey pins the PRECEDENCE rule the
// derived name's namespace rests on: a caller cannot shadow a document's
// carried face by filing bytes under the same string.
//
// The FontSet handed in below carries the shipped LATIN Noto Sans under the
// embedded face's own derived name. If the FontSet arm were consulted first —
// or at all — the Thai text would be drawn with a Latin face that covers none
// of it, and every rune would raise a missing-glyph Warning.
func TestEmbeddedFaceWinsOverACollidingFontSetKey(t *testing.T) {
	source := embeddedChainDoc(t, `[{"asset": "`+embeddedFontAssetKey()+`"}]`)
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fs := FontSet{}
	for name, data := range testShippedFontSet() {
		fs[name] = data
	}
	fs[embeddedFaceName(embeddedFontAssetKey())] = fs["Noto Sans"]
	res, err := Render(tpl, Data(`{}`), nil, fs)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Fatalf("a caller's FontSet entry shadowed the document's own carried face: %+v", res.Diagnostics)
	}
}

// TestChainResolvesInTheDeclaredOrder is the ORDER half at the render
// surface, and since Story 8.4 it has teeth it could not have before: the
// carried face covers BOTH scripts, so which entry sits first decides which
// face draws the Latin.
//
// ["Noto Sans", <carried Thai>] draws Latin with the shipped face and Thai
// with the carried one. [<carried Thai>, "Noto Sans"] draws BOTH with the
// carried one, because the first entry that covers a rune wins (AD-8's Rule).
// The two must therefore differ — and a converter that dropped everything
// after the first embedded entry, or that sorted the chain, would make them
// agree.
func TestChainResolvesInTheDeclaredOrder(t *testing.T) {
	const mixed = "A สัญญา"
	shippedFirst := renderEmbeddedChainDoc(t, embeddedChainDocText(t, `["Noto Sans", {"asset": "`+embeddedFontAssetKey()+`"}]`, mixed))
	carriedFirst := renderEmbeddedChainDoc(t, embeddedChainDocText(t, `[{"asset": "`+embeddedFontAssetKey()+`"}, "Noto Sans"]`, mixed))
	if string(shippedFirst) == string(carriedFirst) {
		t.Fatal("reversing the chain changed nothing — coverage resolution is not walking the document's authored order")
	}
	// The witness that the difference is the one claimed: with the carried
	// face first, the shipped Latin face is not embedded at all.
	if strings.Contains(string(carriedFirst), "+NotoSans-Regular") {
		t.Error("the shipped Latin face reached the page even though the carried face precedes it and covers every rune")
	}
	if !strings.Contains(string(shippedFirst), "+NotoSans-Regular") {
		t.Error("the shipped Latin face did NOT reach the page even though it precedes the carried face — the Latin text was drawn with the wrong face")
	}
}

// TestNonFontAssetIsAcceptedAtLoad is DW-83's first half, and it is the one
// this story must NOT "fix". A chain entry may name an asset that is not a
// font: the loader checks that the key is present in the assets map and
// nothing more, because D-1.8.1 as amended preserves an unrecognised or
// wrong-kind media type at load and errors at render.
// TestANonFontAssetNAMEDBYACHAINMustAlsoStateItsTerms pins the CONSEQUENCE
// Story 8.6 registered in DW-138 and its Spec Change Log — the one place the
// embedded-face record's new rule reaches past the record itself.
//
// requireEmbeddedFaceLicence (internal/template/parse.go) keys off THE CHAIN
// ENTRY, not off the bytes: `{"asset": key}` is the document DECLARING an
// embedded face, and the loader asks whether that face states its terms before
// anything asks what the bytes are. So a chain entry naming an `image/png`
// asset with no `font` record is now refused AT LOAD — where before Story 8.6
// it loaded clean and failed only at render.
//
// THIS TEST EXISTS BECAUSE THE CHANGE WAS OTHERWISE ONLY VISIBLE AS A FIXTURE
// EDIT. `withNonFontAsset` gained a `font` record so the D-1.8.1 tests below
// could still reach their own subjects; that made the new rule a PRECONDITION
// of those tests and asserted it nowhere. The widest consequence of the story
// was the one arm nothing measured.
//
// D-1.8.1 IS NOT WEAKENED, and the pair below is what says so: this document
// is refused for having no terms, and the one directly beneath it — identical
// but for the record — still LOADS and still fails at RENDER. The new rule is
// a gate in front of D-1.8.1's path, never a replacement for it.
func TestANonFontAssetNamedByAChainMustAlsoStateItsTerms(t *testing.T) {
	const anchor = "  \"assets\": {\n"
	source := embeddedChainDocText(t, `["Noto Sans", {"asset": "`+nonFontAssetKey+`"}]`, "สัญญา")
	at := strings.Index(source, anchor)
	if at < 0 {
		t.Fatal("fixture assumption violated: the generated document has no assets block")
	}
	// The SAME PNG withNonFontAsset splices, minus the `font` record — so the
	// two documents differ in exactly the thing under test.
	png := "    \"" + nonFontAssetKey + "\": {\n      \"data\": [" + nonFontAssetData + "],\n      \"mediaType\": \"image/png\"\n    },\n"
	source = source[:at+len(anchor)] + png + source[at+len(anchor):]

	_, err := ParseTemplate([]byte(source))
	if err == nil {
		t.Fatal("a chain entry naming an asset that states no terms must be REFUSED at load, whatever the asset's media type — the entry is what declares an embedded face")
	}
	// LOCATED AT BOTH HALVES. The field is the asset record, which is where the
	// fix goes; the message names the chain entry, which is what made this
	// asset an embedded face. Either coordinate alone sends the reader
	// somewhere they cannot act.
	if want := "assets." + nonFontAssetKey + ".font.licence"; !strings.Contains(err.Error(), want) {
		t.Errorf("the refusal does not locate the asset record %s: %v", want, err)
	}
	if !strings.Contains(err.Error(), "fonts.body[1]") {
		t.Errorf("the refusal does not name the chain entry that makes the asset an embedded face: %v", err)
	}

	// AND THE CONTROL, in the same test so the pair cannot drift apart: add the
	// terms and nothing else, and the document is back on D-1.8.1's path.
	withTerms := strings.Replace(source, "\"data\": ["+nonFontAssetData+"],\n      \"mediaType\": \"image/png\"", "\"data\": ["+nonFontAssetData+"],\n      \"font\": {"+requiredLicenceKeys+"},\n      \"mediaType\": \"image/png\"", 1)
	if withTerms == source {
		t.Fatal("fixture assumption violated: the control's font record was not spliced in")
	}
	if _, cerr := ParseTemplate([]byte(withTerms)); cerr != nil {
		t.Fatalf("D-1.8.1 as amended: an asset that STATES its terms and is not a font must still LOAD and fail only at render, got a load error: %v", cerr)
	}
}

func TestNonFontAssetIsAcceptedAtLoad(t *testing.T) {
	if _, err := ParseTemplate([]byte(nonFontChainDoc(t, `["Noto Sans", {"asset": "`+nonFontAssetKey+`"}]`, "สัญญา"))); err != nil {
		t.Fatalf("a chain entry naming a non-font asset must LOAD (D-1.8.1 as amended): %v", err)
	}
}

// TestNonFontAssetDrawnErrorsAtRenderAndAtValidate is DW-83's second and third
// halves, and the third is the one D-8.3.5 warns is the one that disappears.
//
// The document's text is Thai and the chain's other entry is the shipped Latin
// Noto Sans, which covers none of it — so coverage resolution MUST reach the
// entry naming the image, and that is the moment the renderer is asked to draw
// with something it cannot read.
//
// THE ERROR MUST LOCATE ITSELF at all three coordinates AC4 names: the chain,
// the entry index, and the asset key. A message naming only the asset key
// cannot be acted on when one chain is shared by many elements.
//
// AND VALIDATE MUST RETURN THE IDENTICAL ERROR — asserted by string equality
// and by the diagnostic count, so removing the Validate arm reddens THIS test
// on its own. This is chain_face_names_test.go's own shape, kept deliberately
// rather than the image precedent's (render_image_test.go asserts Render only,
// which is exactly the omission D-8.3.5 names).
func TestNonFontAssetDrawnErrorsAtRenderAndAtValidate(t *testing.T) {
	source := nonFontChainDoc(t, `["Noto Sans", {"asset": "`+nonFontAssetKey+`"}]`, "สัญญา")

	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("the document must LOAD: %v", err)
	}
	_, rerr := Render(tpl, Data(`{}`), nil, testShippedFontSet())
	if rerr == nil {
		t.Fatal("a chain entry naming a non-font asset must fail at Render once something must draw from it (DW-83)")
	}
	for _, want := range []string{
		"element e1",                // the element that needed the face
		`font chain "body" entry 1`, // WHICH entry of WHICH chain
		"asset " + nonFontAssetKey,  // and which asset
		`cannot render font media type "image/png"`,
		"the document is valid", // a capability limit, never a format error
	} {
		if !strings.Contains(rerr.Error(), want) {
			t.Errorf("the render error does not locate itself (%q missing): %v", want, rerr)
		}
	}

	diags, verr := Validate([]byte(source), Data(`{}`), nil, testShippedFontSet())
	if verr == nil {
		t.Fatal("Validate must PREDICT Render (D-1.8.1 amended): Render refuses this document, so Validate must too")
	}
	if verr.Error() != rerr.Error() {
		t.Errorf("Validate and Render disagree about the same document:\n\tValidate: %v\n\tRender:   %v", verr, rerr)
	}
	if len(diags) != 0 {
		t.Errorf("Validate returned %d diagnostic(s) alongside a hard error: %+v", len(diags), diags)
	}
}

// TestNonFontAssetNeverDrawnRendersClean is the OTHER side of the same rule,
// and it is what makes the error's placement a decision rather than an
// accident: the decode is lazy, at the point of use, so an entry no rune ever
// resolves to costs the render nothing and complains about nothing.
//
// The text here is LATIN, which the chain's first entry covers completely, so
// coverage resolution never reaches the entry naming the image. The chain is
// still walked for the vertical model — chainLineMetrics visits every entry —
// and the tolerance there (fontCache.metricsFace) is what keeps this clean.
func TestNonFontAssetNeverDrawnRendersClean(t *testing.T) {
	source := nonFontChainDoc(t, `["Noto Sans", {"asset": "`+nonFontAssetKey+`"}]`, "Latin only")
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Render(tpl, Data(`{}`), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("a chain entry naming a non-font asset that nothing draws from must render clean: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Fatalf("an entry nothing draws from produced diagnostics: %+v", res.Diagnostics)
	}
	if _, verr := Validate([]byte(source), Data(`{}`), nil, testShippedFontSet()); verr != nil {
		t.Fatalf("Validate must predict this clean render, got: %v", verr)
	}
}

// TestChainOfOnlyUnusableEntriesProducesTheExistingLocatedError keeps the pin
// Story 8.3 wrote, re-pointed at the condition that can still reach it.
//
// A chain of nothing but embedded entries is now perfectly renderable, so the
// document that used to produce this error no longer can. What still can is a
// chain whose every entry is a face NOTHING supplies — the pre-existing
// empty-metrics path, deliberately not widened by this story. The assertion is
// here so that if a later story does widen it, the change is deliberate.
func TestChainOfOnlyUnusableEntriesProducesTheExistingLocatedError(t *testing.T) {
	source := embeddedChainDoc(t, `["No Such Face"]`)

	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("a chain naming an unsupplied face must LOAD: %v", err)
	}
	_, rerr := Render(tpl, Data(`{}`), nil, testShippedFontSet())
	if rerr == nil {
		t.Fatal("a chain with no usable entry must fail at Render — nothing can draw the text")
	}
	for _, want := range []string{"element e1", "none of the fallback chain's faces"} {
		if !strings.Contains(rerr.Error(), want) {
			t.Errorf("the render error does not locate itself (%q missing): %v", want, rerr)
		}
	}

	diags, verr := Validate([]byte(source), Data(`{}`), nil, testShippedFontSet())
	if verr == nil {
		t.Fatal("Validate must PREDICT Render (D-1.8.1 amended): Render refuses this document, so Validate must too")
	}
	if verr.Error() != rerr.Error() {
		t.Errorf("Validate and Render disagree about the same document:\n\tValidate: %v\n\tRender:   %v", verr, rerr)
	}
	if len(diags) != 0 {
		t.Errorf("Validate returned %d diagnostic(s) alongside a hard error: %+v", len(diags), diags)
	}
}

// TestTheLocatedErrorNamesTheChainTheElementDrawsThrough pins the COORDINATES
// AC4 asks for against the case that can get them wrong: one asset key named
// by TWO chains.
//
// The address is (chain, entry index, asset key), and the face name the render
// path carries is derived from the ASSET KEY ALONE (AD-8/D-8.4.1 — the key
// decides, and it must, or a document's face and a caller's could collide). So
// one asset key is one face name however many chains name it, and the index
// behind that name must not answer with whichever chain happened to be visited
// first: an author sent to `fonts.aaa[1]` to repair an element that draws
// through `fonts.body` is sent to the wrong line of their own document.
//
// Both chains here are byte-identical apart from their names, so the ONLY
// thing that can make the message name "body" is reading the chain the element
// actually draws through.
func TestTheLocatedErrorNamesTheChainTheElementDrawsThrough(t *testing.T) {
	const chain = `["Noto Sans", {"asset": "` + nonFontAssetKey + `"}]`
	source := withNonFontAsset(t, embeddedFontsBlockDoc(t, `"aaa": `+chain+`, "body": `+chain))

	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("the document must LOAD: %v", err)
	}
	_, rerr := Render(tpl, Data(`{}`), nil, testShippedFontSet())
	if rerr == nil {
		t.Fatal("a chain entry naming a non-font asset must fail at Render once something must draw from it (DW-83)")
	}
	if !strings.Contains(rerr.Error(), `font chain "body" entry 1`) {
		t.Errorf("the error does not name the chain element e1 draws through: %v", rerr)
	}
	// The witness that the assertion above is not passing by accident: the
	// OTHER chain, which no element draws through, must not be named at all.
	if strings.Contains(rerr.Error(), `font chain "aaa"`) {
		t.Errorf("the error names a chain the element does not draw through — the author is sent to the wrong entry: %v", rerr)
	}

	diags, verr := Validate([]byte(source), Data(`{}`), nil, testShippedFontSet())
	if verr == nil || verr.Error() != rerr.Error() {
		t.Errorf("Validate must return the identical error:\n\tValidate: %v\n\tRender:   %v", verr, rerr)
	}
	if len(diags) != 0 {
		t.Errorf("Validate returned %d diagnostic(s) alongside a hard error: %+v", len(diags), diags)
	}
}

// TestMissingGlyphMessageSpellsACarriedFaceForAHuman pins D-000.37 against the
// spelling Story 8.4 created.
//
// The face name the render path carries for a carried face is the RESERVED,
// asset-key-derived one, and it must stay exactly that everywhere it resolves
// a face, keys the cache, names a run or reaches a PDF resource dictionary. It
// is the wrong thing to show a person: `no face in chain [Noto Sans,
// asset:c94562c1…64 hex characters…] covers U+6F22` reads as though the author
// mistyped a font name, and the digest is not a thing they can act on.
//
// So the MESSAGE PATH — and only the message path — spells an embedded entry
// by the display identity the asset itself carries. font.family is display
// identity by D-8.4.1's own words; using it HERE is exactly what it is for,
// and using it to resolve a face is what that decision forbids.
func TestMissingGlyphMessageSpellsACarriedFaceForAHuman(t *testing.T) {
	key := embeddedFontAssetKey()
	source := embeddedChainDocText(t, `["Noto Sans", {"asset": "`+key+`"}]`, "สัญญา 漢")
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Render(tpl, Data(`{}`), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var missing []Diagnostic
	for _, d := range res.Diagnostics {
		if d.Code == DiagCodeTextMissingGlyph {
			missing = append(missing, d)
		}
	}
	if len(missing) == 0 {
		t.Fatal("presence precondition: no missing-glyph diagnostic was produced, so nothing below is asserted")
	}
	for _, d := range missing {
		if strings.Contains(d.Message, key) {
			t.Errorf("the author-facing diagnostic prints the 64-hex asset digest: %s", d.Message)
		}
		if strings.Contains(d.Message, embeddedFaceNamePrefix) {
			t.Errorf("the author-facing diagnostic prints the render path's reserved internal face name: %s", d.Message)
		}
		if !strings.Contains(d.Message, `embedded "Noto Sans Thai"`) {
			t.Errorf("the author-facing diagnostic does not spell the carried face by its own display identity: %s", d.Message)
		}
		// The rest of the sentence still does its job.
		if !strings.Contains(d.Message, "Noto Sans,") || !strings.Contains(d.Message, "U+6F22") {
			t.Errorf("the diagnostic stopped naming the chain that was searched or the rune: %s", d.Message)
		}
	}
	// AND THE RESOLUTION NAMESPACE DID NOT MOVE. The reserved name is what the
	// produced PDF names its font resource, which is the identity witness the
	// fixture's golden rests on.
	if !strings.Contains(string(res.Bytes), pdfEscapedEmbeddedFaceName(key)) {
		t.Error("the reserved face name no longer reaches the PDF resource dictionary — the spelling change escaped the message path")
	}
}

// TestANonFontAssetRefusesWhereAnUnsuppliedFaceSkips pins an asymmetry that is
// real, deliberate and was undocumented and untested until this test.
//
// Both arms below name a chain of THREE entries whose LAST entry covers every
// rune drawn. They differ only in what the middle entry is:
//
//   - a chain entry naming a NON-FONT ASSET refuses the whole render at that
//     entry, even though a later entry covers the text;
//   - a chain entry naming a face the FontSet does not supply is silently
//     SKIPPED, and the later entry draws.
//
// That is not an inconsistency to repair. A non-font asset is a DOCUMENT
// DEFECT that travels inside the file — it is wrong on every machine, forever,
// and D-1.8.1 as amended puts its refusal at the moment something must draw
// with it. An unsupplied face is a DEPLOYMENT CONDITION: the same document is
// correct on a host that supplies it, and AD-8's chain exists precisely so a
// document survives one. The rule is written down in folio-format.md; this
// test is what stops either arm flipping unnoticed.
func TestANonFontAssetRefusesWhereAnUnsuppliedFaceSkips(t *testing.T) {
	t.Run("a non-font asset refuses even though a later entry covers", func(t *testing.T) {
		source := nonFontChainDoc(t, `["Noto Sans", {"asset": "`+nonFontAssetKey+`"}, "Noto Sans Thai"]`, "สัญญา")
		tpl, err := ParseTemplate([]byte(source))
		if err != nil {
			t.Fatalf("the document must LOAD: %v", err)
		}
		_, rerr := Render(tpl, Data(`{}`), nil, testShippedFontSet())
		if rerr == nil {
			t.Fatal("a chain entry naming a non-font asset must refuse, even though the entry after it covers every rune")
		}
		if !strings.Contains(rerr.Error(), `font chain "body" entry 1`) {
			t.Errorf("the refusal does not name the offending entry: %v", rerr)
		}
	})

	t.Run("an unsupplied face is skipped and the later entry draws", func(t *testing.T) {
		source := embeddedChainDoc(t, `["Noto Sans", "No Such Face At All", "Noto Sans Thai"]`)
		tpl, err := ParseTemplate([]byte(source))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		res, err := Render(tpl, Data(`{}`), nil, testShippedFontSet())
		if err != nil {
			t.Fatalf("a chain entry naming a face the FontSet does not supply must be SKIPPED, not refused: %v", err)
		}
		if len(res.Diagnostics) != 0 {
			t.Fatalf("the skipped entry produced diagnostics — the later entry did not draw: %+v", res.Diagnostics)
		}
	})
}
