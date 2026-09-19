package template

import (
	"strings"
	"testing"
)

// TestDecodeLineSpacingDomain is D-7.2.3's range, asserted as the domain
// it actually is rather than as a typographic opinion: a whole number of
// thousandths in [1, 1000000], nothing rounded, nothing clamped.
func TestDecodeLineSpacingDomain(t *testing.T) {
	for _, c := range []struct {
		literal string
		want    int64
		note    string
	}{
		{"1", 1000, "the neutral ratio"},
		{"1.5", 1500, "the worked case"},
		{"0.001", 1, "the floor — no 1.0 minimum: tight leading is what a filed contract is set in"},
		{"0.6", 600, "genuinely tight, and legal"},
		{"1000", 1000000, "the stated sanity ceiling"},
		{"1000.0", 1000000, "a non-canonical spelling of the ceiling is accepted and normalised"},
		{"1.500", 1500, "trailing zeros are a spelling, not a value"},
		{"1e0", 1000, "exponent notation is legal JSON and legal here"},
	} {
		got, err := DecodeLineSpacing(c.literal)
		if err != nil {
			t.Errorf("DecodeLineSpacing(%q) = error %v, want %d (%s)", c.literal, err, c.want, c.note)
			continue
		}
		if got != c.want {
			t.Errorf("DecodeLineSpacing(%q) = %d, want %d (%s)", c.literal, got, c.want, c.note)
		}
	}

	for _, c := range []struct {
		literal string
		reason  string
		note    string
	}{
		{"0", "range", "zero is below the floor"},
		{"-1", "range", "negative is below the floor"},
		{"-0.5", "range", "a negative ratio is not a ratio"},
		{"1000.001", "range", "one thousandth past the ceiling"},
		{"1.0005", "decimal places", "four decimal places is not a whole number of thousandths"},
		{"0.0001", "decimal places", "below the floor AND inexact — the exactness check runs first"},
	} {
		_, err := DecodeLineSpacing(c.literal)
		if err == nil {
			t.Errorf("DecodeLineSpacing(%q) was accepted; %s", c.literal, c.note)
			continue
		}
		if !strings.Contains(err.Error(), c.reason) {
			t.Errorf("DecodeLineSpacing(%q) = %q, want a reason mentioning %q (%s)", c.literal, err.Error(), c.reason, c.note)
		}
	}
}

// TestLineSpacingRoundTripsInItsAuthoredSpelling is AD-9's
// edit-and-edit-back byte identity for the new key at both ends of its
// domain: thousandths through appendPoints is the same ×1000 decimal path
// decodePoints read it with, so there is no second number formatter for
// the two to disagree about.
func TestLineSpacingRoundTripsInItsAuthoredSpelling(t *testing.T) {
	for _, c := range []struct{ authored, canonical string }{
		{"1", "1"},
		{"1.5", "1.5"},
		{"0.875", "0.875"},
		{"0.001", "0.001"},
		{"1000", "1000"},
		{"1.500", "1.5"},   // normalised, and stably so
		{"1e0", "1"},       // ditto
		{"1000.0", "1000"}, // ditto
	} {
		doc := lineSpacingRoundTripDoc(c.authored)
		d, err := ParseDocument([]byte(doc))
		if err != nil {
			t.Errorf("parse %s: %v", c.authored, err)
			continue
		}
		out, err := SerializeDocument(d)
		if err != nil {
			t.Errorf("serialize %s: %v", c.authored, err)
			continue
		}
		want := `"lineSpacing": ` + c.canonical
		if !strings.Contains(string(out), want) {
			t.Errorf("authored %s serialized without %s:\n%s", c.authored, want, out)
		}
		// A second pass must be a fixed point.
		d2, err := ParseDocument(out)
		if err != nil {
			t.Errorf("reparse %s: %v", c.authored, err)
			continue
		}
		out2, err := SerializeDocument(d2)
		if err != nil {
			t.Errorf("reserialize %s: %v", c.authored, err)
			continue
		}
		if string(out2) != string(out) {
			t.Errorf("authored %s is not a fixed point after one canonicalisation:\n%s\nvs\n%s", c.authored, out, out2)
		}
	}
}

// TestVersionForSaveIsRaisedOnlyByContent pins D-1.4.13 at the one
// function that expresses it, including the retrofit D-7.2.1 obliged:
// Epic 10 shipped style.color and bumped nothing, so colour-bearing
// documents declared 1.0 while requiring 1.1.
func TestVersionForSaveIsRaisedOnlyByContent(t *testing.T) {
	for _, c := range []struct {
		label  string
		style  string
		loaded string
		want   string
	}{
		{"neither key, 1.0 in", "", "1.0", "1.0"},
		{"lineSpacing, 1.0 in", `"lineSpacing": 1.5, `, "1.0", "1.1"},
		{"color, 1.0 in", `"color": "#112233", `, "1.0", "1.1"},
		{"explicit null color still declares the key", `"color": null, `, "1.0", "1.1"},
		{"neither key, 1.1 in — never lowered", "", "1.1", "1.1"},
		{"neither key, 1.9 in — never lowered", "", "1.9", "1.9"},
		{"lineSpacing, 1.9 in — the higher wins", `"lineSpacing": 1.5, `, "1.9", "1.9"},
		// MAJOR 0 is LOADABLE: parseVersion admits major >= 0 and
		// checkVersionLoadable refuses only major > SupportedMajor. A 0.x
		// document that introduces no 1.1 key must round-trip verbatim.
		// Raising it to the baseVersion FLOOR would rewrite a field the
		// document owns, on a save that changed nothing (AD-9).
		{"neither key, 0.9 in — a floor is not a demand", "", "0.9", "0.9"},
		{"lineSpacing, 0.9 in — real content still raises", `"lineSpacing": 1.5, `, "0.9", "1.1"},
		// Story 7.3. `align: "justify"` extends a CLOSED SET, which
		// D-1.4.12 makes a MAJOR change, so it is the first content in
		// the format's history that raises the MAJOR component. The
		// three 1.0 alignment values raise nothing: `align` is not a
		// new key, only one of its members is new.
		{"justify alone, 1.0 in", `"align": "justify", `, "1.0", "2.0"},
		{"center does not raise — the key is old, one value is new", `"align": "center", `, "1.0", "1.0"},
		{"left does not raise", `"align": "left", `, "1.0", "1.0"},
		{"justify, 1.1 in — the MAJOR still wins", `"align": "justify", `, "1.1", "2.0"},
		{"justify, 2.1 in — never lowered", `"align": "justify", `, "2.1", "2.1"},
		{"neither key, 2.1 in — never lowered", "", "2.1", "2.1"},
	} {
		t.Run(c.label, func(t *testing.T) {
			doc := strings.Replace(lineSpacingRoundTripDocWithStyle(c.style), `"version": "1.0"`, `"version": "`+c.loaded+`"`, 1)
			d, err := ParseDocument([]byte(doc))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := versionForSave(d.Version, d); got != c.want {
				t.Errorf("versionForSave(%q) = %q, want %q", c.loaded, got, c.want)
			}
		})
	}

	// Story 7.7: the ELEMENT-LEVEL probe. `keepTogether` is not a style
	// key, so a rule expressed only through styleVersionRank would miss
	// it entirely — the same shape of miss style.color made, one level
	// out. These go through their own builder because the style builder
	// cannot express an element key at all.
	for _, c := range []struct {
		label  string
		attr   string
		loaded string
		want   string
	}{
		{"no keepTogether, 1.0 in", "", "1.0", "1.0"},
		{"keepTogether, 1.0 in", `"keepTogether": "signature", `, "1.0", "1.2"},
		{"keepTogether, 1.1 in — the higher wins", `"keepTogether": "signature", `, "1.1", "1.2"},
		{"explicit null keepTogether still declares the key", `"keepTogether": null, `, "1.0", "1.2"},
		{"keepTogether, 1.9 in — never lowered", `"keepTogether": "signature", `, "1.9", "1.9"},
		{"keepTogether, 2.0 in — never lowered", `"keepTogether": "signature", `, "2.0", "2.0"},
	} {
		t.Run(c.label, func(t *testing.T) {
			doc := strings.Replace(keepTogetherRoundTripDocWithAttr(c.attr), `"version": "1.0"`, `"version": "`+c.loaded+`"`, 1)
			d, err := ParseDocument([]byte(doc))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := versionForSave(d.Version, d); got != c.want {
				t.Errorf("versionForSave(%q) = %q, want %q", c.loaded, got, c.want)
			}
		})
	}

	// THE ORDERING CASE FOR THE ELEMENT-LEVEL PROBE, and it is the one a
	// probe that ran only when no style rank had been found would fail:
	// the FIRST element carries `keepTogether` (1.2) and a LATER one
	// carries `align: "justify"` (2.0), so the maximum must be 2.0 —
	// and, in the other order, the 2.0 must not be lowered by a later
	// 1.2. A probe that overwrote `highest` instead of raising it fails
	// the second direction.
	dk, err := ParseDocument([]byte(keepTogetherThenJustifyDoc))
	if err != nil {
		t.Fatalf("parse keepTogether-then-justify doc: %v", err)
	}
	if got := versionRequiredByContent(dk); got != majorFeatureVersion {
		t.Errorf("keepTogether on an earlier element and justify on a later one requires %q, want %q — the element-level probe must participate in the MAXIMUM, never replace it", got, majorFeatureVersion)
	}
	dj, err := ParseDocument([]byte(justifyThenKeepTogetherDoc))
	if err != nil {
		t.Fatalf("parse justify-then-keepTogether doc: %v", err)
	}
	if got := versionRequiredByContent(dj); got != majorFeatureVersion {
		t.Errorf("justify on an earlier element and keepTogether on a later one requires %q, want %q", got, majorFeatureVersion)
	}
	// And the pair that fixes the RANK ORDER itself: 1.2 must beat 1.1,
	// in both authoring orders, or inserting rankKeepTogether renumbered
	// the iota block into a different meaning than it reads as.
	dl, err := ParseDocument([]byte(lineSpacingThenKeepTogetherDoc))
	if err != nil {
		t.Fatalf("parse lineSpacing-then-keepTogether doc: %v", err)
	}
	if got := versionRequiredByContent(dl); got != keepTogetherVersion {
		t.Errorf("lineSpacing on an earlier element and keepTogether on a later one requires %q, want %q", got, keepTogetherVersion)
	}

	// The rule must reach BOTH attachment points, not only element.style:
	// a table that sets lineSpacing on headerStyle alone still requires
	// 1.1, and a version that missed it would misdeclare exactly the way
	// style.color did.
	d, err := ParseDocument([]byte(lineSpacingHeaderStyleDoc))
	if err != nil {
		t.Fatalf("parse headerStyle doc: %v", err)
	}
	if got := versionRequiredByContent(d); got != minorFeatureVersion {
		t.Errorf("a document whose only 1.1 key is on headerStyle requires %q, want %q", got, minorFeatureVersion)
	}

	// Story 7.3 asserted here that the SECOND attachment point reaches
	// the 2.0 rule too: justifyHeaderStyleDoc — a TABLE's
	// headerStyle.align: "justify" — had to LOAD and had to require
	// 2.0. Story 7.8 INVERTS that. The document is now refused at load,
	// so it never becomes a *Document and versionRequiredByContent
	// never sees it; the 2.0 raise is closed by construction rather than
	// by a version rule (see the version half's own test,
	// TestATableStyleJustifyIsRefusedBeforeAnyVersionIsComputed).
	//
	// WHICH OF THE TWO OPTIONS WAS TAKEN, and why (the story required
	// this to be stated): the const was NEITHER deleted NOR rewritten
	// onto a text element. It is KEPT EXACTLY AS IT WAS — still a
	// table, still carrying headerStyle.align: "justify" — and
	// REPURPOSED from an acceptance fixture into the refusal fixture
	// asserted just below. Rewriting it onto a text element would have
	// destroyed the only justify-on-a-table document in this file and
	// left the inverted assertion nothing to refuse; deleting it would
	// have dropped the falsified test rather than inverting it, which
	// the story forbids. The property the const was built to prove —
	// that a rule walking only element.style misses the second
	// attachment point, exactly as style.color was missed — is still
	// live for every 1.1 key, and lineSpacingHeaderStyleDoc above keeps
	// proving it for headerStyle.
	if _, err := ParseDocument([]byte(justifyHeaderStyleDoc)); err == nil {
		t.Fatal("a table's headerStyle.align: \"justify\" must now be REFUSED at load — no *Document exists for versionRequiredByContent to raise to 2.0 (Story 7.8, DW-29)")
	} else if !strings.Contains(err.Error(), "headerStyle.align") || !strings.Contains(err.Error(), "e1") {
		t.Errorf("the refusal must name the element and the field, got: %v", err)
	}

	// THE ORDERING CASE, and it is the one a first-hit implementation
	// fails. The document's FIRST styled element sets lineSpacing (1.1)
	// and a LATER one sets align: justify (2.0). versionRequiredByContent
	// must report the HIGHEST requirement in the document, not the first
	// one it walks into — reporting 1.1 here would ship a file whose own
	// 1.x reader must refuse to draw it while its version says it may.
	dm, err := ParseDocument([]byte(minorThenMajorDoc))
	if err != nil {
		t.Fatalf("parse minor-then-major doc: %v", err)
	}
	if got := versionRequiredByContent(dm); got != majorFeatureVersion {
		t.Errorf("lineSpacing on an earlier element and justify on a later one requires %q, want %q — the rule must take the MAXIMUM over every attachment point, never the first hit", got, majorFeatureVersion)
	}
	// And in the other order, so the result is not an artifact of which
	// element happens to come first.
	dr, err := ParseDocument([]byte(majorThenMinorDoc))
	if err != nil {
		t.Fatalf("parse major-then-minor doc: %v", err)
	}
	if got := versionRequiredByContent(dr); got != majorFeatureVersion {
		t.Errorf("justify on an earlier element and lineSpacing on a later one requires %q, want %q", got, majorFeatureVersion)
	}

	// And an unparseable loaded version is returned untouched rather than
	// invented over — it reached here only through checkVersionLoadable.
	if got := versionForSave("not-a-version", d); got != "not-a-version" {
		t.Errorf("versionForSave over an unparseable version = %q, want it returned untouched", got)
	}
}

// TestContentVersionNeverExceedsTheLibraryCeiling states the relation the
// three version constants currently only IMPLY by coincidence.
//
// versionForSave stamps versionRequiredByContent onto documents it saves.
// If a future key ever raised that above SupportedVersion, this library
// would write files ITS OWN LOADER refuses — checkVersionLoadable rejects
// a higher MAJOR outright — and it would do so silently, because nothing
// compares the two. The failure would surface as "the engine cannot
// reopen what it just wrote", which is the worst possible place to
// discover it.
//
// THE ENUMERATION IS HAND-MAINTAINED, AND SAYING SO IS THE POINT (DW-81,
// closed by Story 8.3). This comment used to claim the bound was "derived from
// the constants themselves rather than from a list someone remembered to
// extend". That was false in both halves: the constant list below is a literal
// slice, and the document-shape loops under it are three hand-written
// builders. A trigger added without a matching loop is SILENTLY NEVER CHECKED
// — which is the exact failure this file exists to prevent, so a comment
// claiming otherwise was worse than no comment at all.
//
// The obligation, stated plainly for whoever adds the fourth trigger: ADD ITS
// BUILDER LOOP IN THE SAME COMMIT. Story 8.3's font trigger is the third, and
// it is the one that found this.
func TestContentVersionNeverExceedsTheLibraryCeiling(t *testing.T) {
	ceilingMajor, ceilingMinor, err := parseVersion(SupportedVersion)
	if err != nil {
		t.Fatalf("SupportedVersion %q does not parse: %v", SupportedVersion, err)
	}
	if ceilingMajor != SupportedMajor {
		t.Errorf("SupportedVersion %q declares MAJOR %d but SupportedMajor is %d — checkVersionLoadable and versionForSave would disagree about what this library can load", SupportedVersion, ceilingMajor, SupportedMajor)
	}
	for _, v := range []string{baseVersion, minorFeatureVersion, keepTogetherVersion, majorFeatureVersion} {
		major, minor, perr := parseVersion(v)
		if perr != nil {
			t.Errorf("%q does not parse: %v", v, perr)
			continue
		}
		if major > ceilingMajor || (major == ceilingMajor && minor > ceilingMinor) {
			t.Errorf("the content rule can return %q, which is ABOVE the library's own ceiling %q — this library would write documents its own loader refuses", v, SupportedVersion)
		}
	}
	// baseVersion must also be the FLOOR: a document requiring nothing
	// must not be stamped above the lowest version the format has.
	if baseVersion != "1.0" {
		t.Errorf("baseVersion = %q, want the format's own lowest version 1.0 — a document that needs nothing new must declare it", baseVersion)
	}
	// And the rule's own output is inside the bound for every document
	// shape it distinguishes, not merely for the constants in isolation.
	for _, style := range []string{"", `"lineSpacing": 1.5, `, `"color": "#112233", `, `"align": "justify", `} {
		d, perr := ParseDocument([]byte(lineSpacingRoundTripDocWithStyle(style)))
		if perr != nil {
			t.Fatalf("parse: %v", perr)
		}
		got := versionRequiredByContent(d)
		major, minor, _ := parseVersion(got)
		if major > ceilingMajor || (major == ceilingMajor && minor > ceilingMinor) {
			t.Errorf("versionRequiredByContent returned %q, above the ceiling %q", got, SupportedVersion)
		}
	}
	// Story 7.7's shape lives on the ELEMENT, not in the style block, so
	// it needs its own builder here or this enumeration goes vacuous for
	// the one key that is not a style key.
	for _, attr := range []string{"", `"keepTogether": "signature", `, `"keepTogether": null, `} {
		d, perr := ParseDocument([]byte(keepTogetherRoundTripDocWithAttr(attr)))
		if perr != nil {
			t.Fatalf("parse: %v", perr)
		}
		got := versionRequiredByContent(d)
		major, minor, _ := parseVersion(got)
		if major > ceilingMajor || (major == ceilingMajor && minor > ceilingMinor) {
			t.Errorf("versionRequiredByContent returned %q, above the ceiling %q", got, SupportedVersion)
		}
	}
	// Story 8.3's shape lives on neither the style block nor the element:
	// it is the document-level `fonts` map, which no element walk reaches
	// at all. A THIRD builder, for the same reason keepTogether needed a
	// second one — and the failure mode this loop guards against is not
	// "the probe is wrong" but "the probe is never walked".
	//
	// It asserts BOTH DIRECTIONS, and the pair is the point (D-1.4.13):
	// an embedded-face ENTRY requires 2.0, and a font ASSET that no chain
	// references requires nothing — such a document loads and renders
	// correctly on a 1.x reader, so raising it would orphan a document
	// from readers that can in fact read it.
	//
	// STORY 8.6: BOTH ASSET-BEARING ARMS CARRY THE FULL LICENCE RECORD, and
	// carrying it in the UNREFERENCED arm is deliberate rather than tidy. It
	// makes the two arms differ in exactly ONE thing — the chain entry — so
	// what the pair measures is still the entry and not the record. The
	// record's own keys reach no version rule at all: they can only appear on
	// an asset, and an asset alone raises nothing.
	for _, tc := range []struct {
		name string
		doc  string
		want string
	}{
		{"no font asset, no embedded entry", embeddedFontVersionDoc(false, false), baseVersion},
		{"a font ASSET the chain does not name", embeddedFontVersionDoc(true, false), baseVersion},
		{"an embedded-face ENTRY", embeddedFontVersionDoc(true, true), majorFeatureVersion},
		// spec-barcode-qr-elements: the ELEMENT TYPE is the trigger, so its
		// builder swaps the element's type rather than adding a key. The
		// text twin is the control that keeps the corpus where it is.
		{"a text element (barcode control)", barcodeVersionDoc("text"), baseVersion},
		{"a barcode element", barcodeVersionDoc("barcode"), barcodeVersion},
		// A LATER proportional table must not lower the barcode's rank.
		{"a barcode then a proportional table", barcodeThenProportionalTableDoc, barcodeVersion},
	} {
		d, perr := ParseDocument([]byte(tc.doc))
		if perr != nil {
			t.Fatalf("%s: parse: %v", tc.name, perr)
		}
		got := versionRequiredByContent(d)
		if got != tc.want {
			t.Errorf("%s: versionRequiredByContent = %q, want %q", tc.name, got, tc.want)
		}
		major, minor, _ := parseVersion(got)
		if major > ceilingMajor || (major == ceilingMajor && minor > ceilingMinor) {
			t.Errorf("%s: versionRequiredByContent returned %q, above the ceiling %q", tc.name, got, SupportedVersion)
		}
	}
}

// embeddedFontVersionDoc builds the three documents the loop above walks:
// with or without a font ASSET, and with or without a chain ENTRY naming it.
// The asset is the same 156-byte hand-built sfnt maximalFixture carries (see
// fixtures_test.go) — a fixture, not a face; the load-time check on
// `font/ttf` is structural, and nothing here renders.
//
// referenced == true with embedded == false is not a case: an entry can only
// name an asset the document carries, and decodeFonts refuses one that does
// not, so the combination is unrepresentable rather than untested.
func embeddedFontVersionDoc(asset, referenced bool) string {
	const key = "cbd7a24e64e08aba9da4edd9343b9eaa629e7c26e722eedf68fd5efe217dbedc"
	assets := "{}"
	if asset {
		assets = `{
    "` + key + `": {
      "data": [
        "AAEAAAADACAABAAQY21hcAAAAAAAAAA8AAAAIGdseWYAAAAAAAAAXAAAACBoZWFkAAAAAAAAAHwA",
        "AAAgQ01BUERBVEFDTUFQREFUQUNNQVBEQVRBQ01BUERBVEFHTFlGREFUQUdMWUZEQVRBR0xZRkRB",
        "VEFHTFlGREFUQUhFQUREQVRBSEVBRERBVEFIRUFEREFUQUhFQUREQVRB"
      ],
      "font": {
        "copyright": "Copyright 2026 The Folio Fixture Authors",
        "family": "Maximal Sans",
        "licence": "SIL Open Font License 1.1",
        "licenceText": "This fixture face is licensed under the SIL Open Font License, Version 1.1.",
        "style": "Regular"
      },
      "mediaType": "font/ttf"
    }
  }`
	}
	chain := `["Noto Sans"]`
	if referenced {
		chain = `["Noto Sans", {"asset": "` + key + `"}]`
	}
	return `{
  "assets": ` + assets + `,
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 40, "value": "v", "style": {"fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ` + chain + `},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "2.0"
}`
}

func lineSpacingRoundTripDoc(authored string) string {
	return lineSpacingRoundTripDocWithStyle(`"lineSpacing": ` + authored + `, `)
}

func lineSpacingRoundTripDocWithStyle(style string) string {
	return `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 40, "value": "v", "style": {` + style + `"fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

// keepTogetherRoundTripDocWithAttr is lineSpacingRoundTripDocWithStyle's
// ELEMENT-LEVEL twin (Story 7.7): the attribute goes on the element
// itself, beside `id`/`type`, because `keepTogether` is not a style key
// and no style builder can express it.
// barcodeVersionDoc is a one-element document whose element is of kind,
// carrying a value and no style (a barcode refuses one).
func barcodeVersionDoc(kind string) string {
	return `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "` + kind + `", "x": 0, "y": 0, "width": 200, "height": 40, "value": "1234"}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

// barcodeThenProportionalTableDoc walks a barcode FIRST and a proportional
// table (an authored width with proportion columns) after it.
const barcodeThenProportionalTableDoc = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "barcode", "x": 0, "y": 0, "width": 200, "height": 40, "value": "1234"},
        {"id": "e2", "type": "table", "x": 0, "y": 60, "width": 200, "bind": "items[]", "headerHeight": 10,
          "columns": [{"id": "e3", "label": "A", "proportion": 1, "bind": "{{row.a}}"}, {"id": "e4", "label": "B", "proportion": 1, "bind": "{{row.b}}"}]}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 5,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

func keepTogetherRoundTripDocWithAttr(attr string) string {
	return `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 40, ` + attr + `"value": "v", "style": {"fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

// keepTogetherThenJustifyDoc puts the 1.2 element key on the element the
// walk reaches FIRST and the 2.0 style value on a later one; its twin
// reverses the order. Together they pin that the element-level probe
// RAISES the running maximum rather than setting it.
const keepTogetherThenJustifyDoc = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 40, "keepTogether": "signature", "value": "a", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e2", "type": "text", "x": 0, "y": 60, "width": 200, "height": 40, "value": "b", "style": {"fontFamily": "body", "fontSize": 11, "align": "justify"}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

const justifyThenKeepTogetherDoc = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 40, "value": "a", "style": {"fontFamily": "body", "fontSize": 11, "align": "justify"}},
        {"id": "e2", "type": "text", "x": 0, "y": 60, "width": 200, "height": 40, "keepTogether": "signature", "value": "b", "style": {"fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// lineSpacingThenKeepTogetherDoc pins the ORDER OF THE TWO MINORS: 1.2
// must beat 1.1. It is the case that reddens if inserting rankKeepTogether
// into the iota block put it below rankMinorFeature.
const lineSpacingThenKeepTogetherDoc = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 40, "value": "a", "style": {"fontFamily": "body", "fontSize": 11, "lineSpacing": 1.5}},
        {"id": "e2", "type": "text", "x": 0, "y": 60, "width": 200, "height": 40, "keepTogether": "signature", "value": "b", "style": {"fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

const lineSpacingHeaderStyleDoc = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "rows[]", "as": "row", "headerHeight": 20,
          "columns": [{"id": "e2", "label": "L", "width": 100, "bind": "{{row.v}}"}],
          "headerStyle": {"lineSpacing": 1.5}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// justifyHeaderStyleDoc was lineSpacingHeaderStyleDoc's Story 7.3 twin,
// asserting that the 2.0 value on a table's headerStyle raised the
// document the same way an element's own style does. Story 7.8 refuses
// that document at load, so the const is now the REFUSAL fixture: the
// one document in this file whose justify sits on a table.
const justifyHeaderStyleDoc = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "rows[]", "as": "row", "headerHeight": 20,
          "columns": [{"id": "e2", "label": "L", "width": 100, "bind": "{{row.v}}"}],
          "headerStyle": {"align": "justify"}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// minorThenMajorDoc puts the 1.1 key on the element the walk reaches
// FIRST and the 2.0 value on a later one. A first-hit implementation
// returns 1.1 for it; the maximum rule returns 2.0.
const minorThenMajorDoc = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 40, "value": "a", "style": {"lineSpacing": 1.5, "fontFamily": "body", "fontSize": 11}},
        {"id": "e2", "type": "text", "x": 0, "y": 60, "width": 200, "height": 40, "value": "b", "style": {"align": "justify", "fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// majorThenMinorDoc is the same document with the two elements' styles
// swapped, so neither answer can be an artifact of walk order.
const majorThenMinorDoc = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 40, "value": "a", "style": {"align": "justify", "fontFamily": "body", "fontSize": 11}},
        {"id": "e2", "type": "text", "x": 0, "y": 60, "width": 200, "height": 40, "value": "b", "style": {"lineSpacing": 1.5, "fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
