package template

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/diag"
)

// This file is Story 8.3's evidence: the I/O matrix, every row, for a font
// that travels inside the template.
//
// WHAT IT DOES NOT COVER, deliberately and still: rendering FROM an embedded
// face. Story 8.4 built that, in `package folio8` — this package structurally
// cannot reach Render or Validate, so what belongs here is the FORMAT's half
// and nothing else. The half that is this package's, and that Story 8.4 must
// not have "fixed", is D-1.8.1 as amended: the load path neither resolves an
// embedded entry to a face nor refuses a chain for holding one. That is pinned
// below (TestLoadNeitherResolvesNorRefusesAnEmbeddedEntry), because a negative
// assertion carries a test's evidentiary burden.

// embeddedFontKey is the SHA-256 of embeddedFontBytes below, and the key the
// documents in this file use. It is the same 156-byte hand-built sfnt
// maximalFixture carries: a version tag, a three-entry table directory and
// three 32-byte tables. It is a FIXTURE AND NOT A FACE, and that is enough,
// because the load-time check on a recognised font media type is STRUCTURAL
// (checkSfnt) — nothing in this package parses a face for glyphs.
const embeddedFontKey = "cbd7a24e64e08aba9da4edd9343b9eaa629e7c26e722eedf68fd5efe217dbedc"

const embeddedFontData = `[
        "AAEAAAADACAABAAQY21hcAAAAAAAAAA8AAAAIGdseWYAAAAAAAAAXAAAACBoZWFkAAAAAAAAAHwA",
        "AAAgQ01BUERBVEFDTUFQREFUQUNNQVBEQVRBQ01BUERBVEFHTFlGREFUQUdMWUZEQVRBR0xZRkRB",
        "VEFHTFlGREFUQUhFQUREQVRBSEVBRERBVEFIRUFEREFUQUhFQUREQVRB"
      ]`

// embeddedFontDoc builds a whole `.folio` with one asset and one chain, so
// every row of the matrix is a real ParseDocument over real bytes rather than
// a hand-built Document that skipped the loader.
//
// assetBody is the asset object's inner text (so a row can vary mediaType, the
// data or the font record); chainBody is the chain array verbatim (so a row
// can put any entry shape in it, legal or not).
func embeddedFontDoc(assetBody, chainBody string) string {
	return `{
  "assets": {
    "` + embeddedFontKey + `": {` + assetBody + `
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

// fontAssetBody is a well-formed font asset with the full `font` record.
const fontAssetBody = `
      "data": ` + embeddedFontData + `,
      "font": {
        "copyright": "Copyright 2026 The Folio Fixture Authors",
        "family": "Maximal Sans",
        "licence": "SIL Open Font License 1.1",
        "licenceText": "This fixture face is licensed under the SIL Open Font License, Version 1.1. (The whole text travels here in a real document; a fixture states enough to be non-empty.)",
        "source": "hand-built 156-byte sfnt — a fixture, not a face",
        "style": "Regular"
      },
      "mediaType": "font/ttf"`

// plainFontAssetBody carries no `font` record at all.
//
// SINCE STORY 8.6 IT IS ONLY LEGAL UNREFERENCED. An asset a chain names by
// {"asset": key} must state its terms — licence, licenceText and copyright —
// so this body is paired with unreferencedChain below, never with
// embeddedChain. The referenced arm of exactly this body is the REFUSAL that
// TestAReferencedFaceMustStateItsTerms asserts.
const plainFontAssetBody = `
      "data": ` + embeddedFontData + `,
      "mediaType": "font/ttf"`

// fontAssetBodyOfType is fontAssetBody with the media type varied and the
// licence record kept whole. A row that varies the media type is asking about
// RECOGNITION, so it must not accidentally also be asking about the licence
// rule — before Story 8.6 those rows carried no `font` record at all and
// passed for a reason that no longer exists.
func fontAssetBodyOfType(mediaType string) string {
	return strings.Replace(fontAssetBody, `"mediaType": "font/ttf"`, `"mediaType": "`+mediaType+`"`, 1)
}

const embeddedChain = `["Noto Sans", {"asset": "` + embeddedFontKey + `"}]`

// unreferencedChain names no asset at all, so the document's font asset is an
// ORPHAN — present, preserved, and outside Story 8.6's licence requirement,
// which is scoped to an asset a chain actually names. Every row below whose
// subject is the RECORD's presence semantics rather than the reference rule
// uses this chain, so a partial or absent record is still a legal document to
// round-trip.
const unreferencedChain = `["Noto Sans"]`

// requireLoadError asserts a document is refused with a *LoadError at exactly
// field — INCLUDING the entry index — and with the general code. The exact
// field is the assertion, not a substring of the message: the field is what a
// consumer discriminates on (D-7.8.1: no code is minted here, so the Field is
// the whole diagnosis), and a message check would pass on a refusal that had
// lost the index.
func requireLoadError(t *testing.T, doc, field string) *LoadError {
	t.Helper()
	_, err := ParseDocument([]byte(doc))
	if err == nil {
		t.Fatalf("expected a load error at field %s, got none", field)
	}
	var le *LoadError
	if !errors.As(err, &le) {
		t.Fatalf("expected a *LoadError at field %s, got %T: %v", field, err, err)
	}
	if le.Field != field {
		t.Fatalf("load error field = %q, want %q (reason: %s)", le.Field, field, le.Reason)
	}
	if le.Code != diag.CodeTemplateFieldInvalid {
		t.Fatalf("load error code = %q, want the general %q — no font-specific code is minted by this story (D-7.8.1)", le.Code, diag.CodeTemplateFieldInvalid)
	}
	return le
}

// canonicalFixedPoint is the round trip, BOTH directions, over one source.
//
// It returns the canonical bytes and asserts, on the way, that they are a
// fixed point: Serialize(Parse(canonical)) == canonical, and
// Parse(Serialize(d)) == d as the same statement read the other way. The
// documents in this file are written for READABILITY (compact bands, one-line
// chains), so comparing a hand-written source to the serializer's output would
// assert the author's formatting rather than the format's canonical form — the
// canonical bytes the loader itself produces are what the property is about.
func canonicalFixedPoint(t *testing.T, source string) string {
	t.Helper()
	d, err := ParseDocument([]byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	d2, err := ParseDocument(out)
	if err != nil {
		t.Fatalf("re-parse the canonical bytes: %v", err)
	}
	out2, err := SerializeDocument(d2)
	if err != nil {
		t.Fatalf("re-serialize: %v", err)
	}
	if string(out2) != string(out) {
		t.Fatalf("Serialize(Parse(b)) != b over canonical bytes:\n--- got ---\n%s\n--- want ---\n%s", out2, out)
	}
	return string(out)
}

// TestEmbeddedEntryRoundTripsBothDirections is the matrix's first two rows,
// and it pins the fixed point in BOTH directions, because either one alone is
// satisfiable by a serializer that is wrong in a way the other would catch.
func TestEmbeddedEntryRoundTripsBothDirections(t *testing.T) {
	source := embeddedFontDoc(fontAssetBody, embeddedChain)

	d, err := ParseDocument([]byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	chain, ok := d.Fonts.Chain("body")
	if !ok || len(chain) != 2 {
		t.Fatalf("chain body = %#v, ok = %v — want two entries", chain, ok)
	}
	if chain[0].Embedded() || chain[0].Face != "Noto Sans" {
		t.Errorf("entry 0 = %#v, want the named face Noto Sans", chain[0])
	}
	if !chain[1].Embedded() || chain[1].AssetKey != embeddedFontKey || chain[1].Face != "" {
		t.Errorf("entry 1 = %#v, want the embedded asset %s and no face", chain[1], embeddedFontKey)
	}

	// The `font` record, all four keys, into the model.
	asset := d.Assets[embeddedFontKey]
	if !asset.Font.Set || asset.Font.Null {
		t.Fatalf("asset font record = %#v, want it present", asset.Font)
	}
	rec := asset.Font.Value
	for _, want := range []struct {
		key   string
		got   Presence[string]
		value string
	}{
		{"family", rec.Family, "Maximal Sans"},
		{"licence", rec.Licence, "SIL Open Font License 1.1"},
		{"source", rec.Source, "hand-built 156-byte sfnt — a fixture, not a face"},
		{"style", rec.Style, "Regular"},
	} {
		if !want.got.Set || want.got.Null || want.got.Value != want.value {
			t.Errorf("font.%s = %#v, want %q", want.key, want.got, want.value)
		}
	}

	// The fixed point, both directions, plus the CANONICAL SPELLING of the two
	// things this story added — asserted as bytes, because "it round-trips"
	// is satisfied by any spelling that is stable, including a wrong one.
	canonical := canonicalFixedPoint(t, source)
	wantEntry := "\n      {\n        \"asset\": \"" + embeddedFontKey + "\"\n      }\n"
	if !strings.Contains(canonical, wantEntry) {
		t.Errorf("the embedded entry is not emitted as a one-key object through writeObject:\n%s", canonical)
	}
	// `font` sorts between `data` and `mediaType`, and its own six keys come
	// back sorted — the whole record, spelled out, so a key order that drifted
	// would be visible here rather than merely stable. `licenceText` sorting
	// immediately after `licence` is the one adjacency worth reading twice:
	// they are a prefix pair, and a sort that put them the other way round
	// would still look alphabetical at a glance.
	wantRecord := `"font": {
        "copyright": "Copyright 2026 The Folio Fixture Authors",
        "family": "Maximal Sans",
        "licence": "SIL Open Font License 1.1",
        "licenceText": "This fixture face is licensed under the SIL Open Font License, Version 1.1. (The whole text travels here in a real document; a fixture states enough to be non-empty.)",
        "source": "hand-built 156-byte sfnt — a fixture, not a face",
        "style": "Regular"
      },
      "mediaType": "font/ttf"`
	if !strings.Contains(canonical, wantRecord) {
		t.Errorf("the font record is not emitted in sorted key order between data and mediaType:\n%s", canonical)
	}
}

// TestFontRecordKeysRoundTripIndividually walks the record's keys one at a
// time, in all three presence states, because the whole-record test above
// cannot distinguish "absent" from "explicitly null" — and a refusal written
// only in the non-null branch lets `"family": null` past every guard.
// fontRecordKeys is the record's six keys, named once. Every per-key test
// below is driven off THIS list rather than off `family` alone, because the
// argument for the three-way presence handling is a PER-KEY argument — "a
// refusal written only in the non-null branch lets `null` past every guard" is
// as true of `licence` as of `family`, and a test that only ever nulls
// `family` proves it for one sixth of the record.
//
// It is also the guard against the record growing a seventh key with no
// coverage: TestFontRecordKeyListIsTheWholeRecord below reflects over
// FontRecord and fails if this list and the struct disagree.
var fontRecordKeys = []string{"copyright", "family", "licence", "licenceText", "source", "style"}

// fontAssetWithRecord builds an asset body carrying `record` verbatim as its
// `font` value, so a case can put any JSON there — legal or not.
func fontAssetWithRecord(record string) string {
	return "\n      \"data\": " + embeddedFontData + ",\n      \"font\": " + record + ",\n      \"mediaType\": \"font/ttf\""
}

// TestFontRecordKeyListIsTheWholeRecord binds fontRecordKeys to FontRecord
// itself, so a seventh key added to the model without a line here is caught
// rather than silently uncovered by every test below.
//
// THE COMPARISON IS CASE-FOLDED, and that is a concession worth naming: the
// model has no struct tags (the writer and the decoder spell the JSON keys),
// so reflection can only offer the Go field name. `LicenceText` and
// `licenceText` are the same key under a fold and nothing else here collides,
// so folding is exact for this struct — but it is the reason a field named
// `Licence_Text` would slip past, and if the record ever grows one, this
// binding needs a real name map rather than a fold.
func TestFontRecordKeyListIsTheWholeRecord(t *testing.T) {
	typ := reflect.TypeOf(FontRecord{})
	var presenceFields []string
	for i := range typ.NumField() {
		f := typ.Field(i)
		if f.Type == reflect.TypeOf(Presence[string]{}) {
			presenceFields = append(presenceFields, strings.ToLower(f.Name))
		}
	}
	sort.Strings(presenceFields)
	want := make([]string, 0, len(fontRecordKeys))
	for _, key := range fontRecordKeys {
		want = append(want, strings.ToLower(key))
	}
	sort.Strings(want)
	if !reflect.DeepEqual(presenceFields, want) {
		t.Fatalf("FontRecord's optional string keys are %v and fontRecordKeys names %v — every key the record carries must be driven through the presence tests below", presenceFields, want)
	}
}

// TestFontRecordKeysRoundTripIndividually walks EVERY key through all three
// presence states — absent, explicitly null, present with a value — plus the
// record-level states, and asserts each is a canonical fixed point. That is
// what makes "an explicit null is not absence" a property of the record rather
// than of one field of it.
func TestFontRecordKeysRoundTripIndividually(t *testing.T) {
	// Record-level states first: the key absent altogether, null, and empty.
	for _, tc := range []struct {
		name   string
		record string
	}{
		{"absent record", ""},
		{"null record", "null"},
		{"empty record", "{}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := "\n      \"data\": " + embeddedFontData + ",\n      \"mediaType\": \"font/ttf\""
			if tc.record != "" {
				body = fontAssetWithRecord(tc.record)
			}
			canonicalFixedPoint(t, embeddedFontDoc(body, unreferencedChain))
		})
	}

	// Then every KEY, in each of its three states, one key at a time — so a
	// key that round-trips only because a sibling was present cannot hide.
	for _, key := range fontRecordKeys {
		for _, tc := range []struct {
			state string
			value string
		}{
			{"present", `"a value"`},
			{"explicitly null", `null`},
			{"empty string", `""`},
		} {
			t.Run(key+"/"+tc.state, func(t *testing.T) {
				d, err := ParseDocument([]byte(embeddedFontDoc(fontAssetWithRecord(`{"`+key+`": `+tc.value+`}`), unreferencedChain)))
				if err != nil {
					t.Fatalf("parse: %v", err)
				}
				got := fontRecordField(t, d.Assets[embeddedFontKey], key)
				if !got.Set {
					t.Fatalf("%s: parsed to an ABSENT Presence — a key that was written must never read back as missing", key)
				}
				if wantNull := tc.value == "null"; got.Null != wantNull {
					t.Errorf("%s: Null = %v, want %v — absence, an explicit null and a value are three states, not two", key, got.Null, wantNull)
				}
				canonicalFixedPoint(t, embeddedFontDoc(fontAssetWithRecord(`{"`+key+`": `+tc.value+`}`), unreferencedChain))
			})
		}

		// And the ABSENT arm, per key: writing the other three must leave this
		// one absent, and absent must cost no byte on the way out.
		t.Run(key+"/absent", func(t *testing.T) {
			var pairs []string
			for _, other := range fontRecordKeys {
				if other != key {
					pairs = append(pairs, `"`+other+`": "a value"`)
				}
			}
			source := embeddedFontDoc(fontAssetWithRecord(`{`+strings.Join(pairs, ", ")+`}`), unreferencedChain)
			d, err := ParseDocument([]byte(source))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := fontRecordField(t, d.Assets[embeddedFontKey], key); got.Set {
				t.Errorf("%s: a key nobody wrote parsed as PRESENT (%#v)", key, got)
			}
			// Scoped to the RECORD's own keys, read off the emitted JSON.
			// A bare strings.Contains over the whole document would have
			// been satisfied by the text element's own `"style"` key —
			// measured: it was, and the check passed for the wrong
			// reason on three keys and failed on the fourth.
			canonical := canonicalFixedPoint(t, source)
			emitted := emittedFontRecordKeys(t, canonical)
			if slices.Contains(emitted, key) {
				t.Errorf("%s: an absent key reappeared on serialize — the record emitted %v", key, emitted)
			}
			for _, other := range fontRecordKeys {
				if other != key && !slices.Contains(emitted, other) {
					t.Errorf("%s: emitting one key dropped a sibling — the record emitted %v", key, emitted)
				}
			}
		})
	}

	// An unknown key inside the record rides through opaquely (D-1.4.9),
	// unlike an unknown key in a chain ENTRY, where the object IS the
	// discriminant and an unknown key is a load error.
	t.Run("an unknown key rides through", func(t *testing.T) {
		canonicalFixedPoint(t, embeddedFontDoc(fontAssetWithRecord(`{"family": "Maximal Sans", "weight": 700}`), unreferencedChain))
	})
}

// fontRecordField reads one key's Presence off a parsed asset by NAME, so the
// table above drives the model as well as the bytes. Reflection rather than a
// switch, for the same reason fontRecordKeys is bound to the struct: a fifth
// key must not be able to acquire a silently missing case here.
func fontRecordField(t *testing.T, asset Asset, key string) Presence[string] {
	t.Helper()
	if !asset.Font.Set || asset.Font.Null {
		t.Fatalf("the asset carries no font record (%#v)", asset.Font)
	}
	name := strings.ToUpper(key[:1]) + key[1:]
	value := reflect.ValueOf(asset.Font.Value).FieldByName(name)
	if !value.IsValid() {
		t.Fatalf("FontRecord has no field %s — fontRecordKeys and the model disagree", name)
	}
	got, ok := value.Interface().(Presence[string])
	if !ok {
		t.Fatalf("FontRecord.%s is not a Presence[string]", name)
	}
	return got
}

// emittedFontRecordKeys reads the `font` record's keys off SERIALIZED bytes,
// sorted. Off the JSON rather than off a substring search, because the
// document carries an element `style` block whose key name collides with the
// record's own `style` — a text search cannot tell the two apart, and the one
// that matters here is the record's.
func emittedFontRecordKeys(t *testing.T, canonical string) []string {
	t.Helper()
	var doc struct {
		Assets map[string]struct {
			Font map[string]json.RawMessage `json:"font"`
		} `json:"assets"`
	}
	if err := json.Unmarshal([]byte(canonical), &doc); err != nil {
		t.Fatalf("unmarshal the canonical bytes: %v", err)
	}
	asset, ok := doc.Assets[embeddedFontKey]
	if !ok {
		t.Fatalf("the canonical bytes carry no asset %s", embeddedFontKey)
	}
	keys := make([]string, 0, len(asset.Font))
	for key := range asset.Font {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// TestFontRecordRefusesANonString is the null-vs-wrong-type pair, PER KEY. The
// pairing is the point: `null` is accepted for every key (above) and a wrong
// type is refused for every key (here), so neither answer can be inferred from
// the other, and no key gets one without the other.
func TestFontRecordRefusesANonString(t *testing.T) {
	for _, key := range fontRecordKeys {
		for _, value := range []string{`3`, `true`, `["a value"]`, `{"a": "value"}`} {
			t.Run(key+"/"+value, func(t *testing.T) {
				source := embeddedFontDoc(fontAssetWithRecord(`{"`+key+`": `+value+`}`), embeddedChain)
				requireLoadError(t, source, "assets."+embeddedFontKey+".font."+key)
			})
		}
	}

	// And the record itself, where the value is not an object at all.
	for _, value := range []string{`"Maximal Sans"`, `7`, `["Maximal Sans"]`} {
		t.Run("the record is "+value, func(t *testing.T) {
			requireLoadError(t, embeddedFontDoc(fontAssetWithRecord(value), embeddedChain), "assets."+embeddedFontKey+".font")
		})
	}
}

// TestTwoChainsNamingOneAssetDeduplicate is the matrix's dedup row. There is
// no dedup PASS to write — Assets is a Go map keyed by the digest, so two
// chains naming the same face are one entry by construction — and this is the
// test that says so rather than a comment claiming it.
func TestTwoChainsNamingOneAssetDeduplicate(t *testing.T) {
	source := `{
  "assets": {
    "` + embeddedFontKey + `": {` + fontAssetBody + `
    }
  },
  "bands": {
    "content": {"elements": []},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {
    "body": [
      {
        "asset": "` + embeddedFontKey + `"
      }
    ],
    "heading": [
      {
        "asset": "` + embeddedFontKey + `"
      }
    ]
  },
  "locale": "en",
  "nextId": 1,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "2.0"
}`
	d, err := ParseDocument([]byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(d.Assets) != 1 {
		t.Fatalf("assets = %d entries, want 1 — two chains naming one key must be one asset", len(d.Assets))
	}
	canonical := canonicalFixedPoint(t, source)
	if strings.Count(canonical, `"`+embeddedFontKey+`": {`) != 1 {
		t.Fatalf("the asset key is emitted more than once:\n%s", canonical)
	}
	// Both chains still name it, so dedup is storage-level and not a rewrite
	// of the chains that referenced it.
	if strings.Count(canonical, `"asset": "`+embeddedFontKey+`"`) != 2 {
		t.Fatalf("both chains must still carry their own entry:\n%s", canonical)
	}
}

// TestAddingAFontAssetMovesNoImageAsset is the matrix's emission-order row,
// measured rather than argued: an asset's byte position is a total function of
// its key (writeAssets sorts, and writeObject sorts again), so inserting one
// can only shift what sorts AFTER it.
//
// The two keys are chosen so the FONT sorts BEFORE the image — the direction
// that would move the image if the property did not hold. Choosing them the
// other way round would have made this test pass on an echoing serializer.
func TestAddingAFontAssetMovesNoImageAsset(t *testing.T) {
	// A SECOND font asset, whose digest sorts BEFORE the image's. The 156-byte
	// sfnt this file otherwise uses hashes to c…, which sorts AFTER the image
	// and so could not observe a moved one; this variant differs only in its
	// table contents and hashes to 3….
	const lowFontKey = "35573263bb78c4a0b0866ff63489bcfeb36b56ac2abe42206967541ba829eea7"
	const lowFontBody = `
      "data": [
        "AAEAAAADACAABAAQY21hcAAAAAAAAAA8AAAAIGdseWYAAAAAAAAAXAAAACBoZWFkAAAAAAAAAHwA",
        "AAAgRklYVFVSRTFGSVhUVVJFMUZJWFRVUkUxRklYVFVSRTFGSVhUVVJFMUZJWFRVUkUxRklYVFVS",
        "RTFGSVhUVVJFMUZJWFRVUkUxRklYVFVSRTFGSVhUVVJFMUZJWFRVUkUx"
      ],
      "font": {
        "family": "Low Sans"
      },
      "mediaType": "font/ttf"`
	const imageKey = "5a05ad01e89c143b7061b0c93450566568d38a23da9b9c5c9dfe449016433078"
	const imageBody = `
      "data": [
        "iVBORw0KGgoAAAANSUhEUgAAAAMAAAACCAIAAAASFvFNAAAAGElEQVR42mL6z8DAAMZMEOo/AwMg",
        "AAD//zwUBf/NjsW5AAAAAElFTkSuQmCC"
      ],
      "mediaType": "image/png"`
	if !(lowFontKey < imageKey) {
		t.Fatalf("fixture assumption violated: the font key %s must sort BEFORE the image key %s, or this test cannot observe a moved image", lowFontKey, imageKey)
	}

	doc := func(assets string) string {
		return `{
  "assets": {` + assets + `
  },
  "bands": {
    "content": {"elements": []},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 1,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}`
	}
	imageOnly := doc("\n    \"" + imageKey + "\": {" + imageBody + "\n    }")
	both := doc("\n    \"" + lowFontKey + "\": {" + lowFontBody + "\n    },\n    \"" + imageKey + "\": {" + imageBody + "\n    }")

	imageObject := func(source string) string {
		t.Helper()
		d, err := ParseDocument([]byte(source))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		out, err := SerializeDocument(d)
		if err != nil {
			t.Fatalf("serialize: %v", err)
		}
		i := strings.Index(string(out), `"`+imageKey+`"`)
		if i < 0 {
			t.Fatalf("the image asset is missing from the output:\n%s", out)
		}
		return string(out)[i:]
	}
	if got, want := imageObject(both), imageObject(imageOnly); got != want {
		t.Fatalf("adding a font asset changed the image asset's emitted bytes:\n--- with a font ---\n%s\n--- without ---\n%s", got, want)
	}
}

// TestAbsentAssetKeyIsALocatedLoadError is the matrix's absent-key row: the
// field names the CHAIN, the INDEX and lands on `.asset`, and the key itself
// is the reported value.
func TestAbsentAssetKeyIsALocatedLoadError(t *testing.T) {
	const missing = "0000000000000000000000000000000000000000000000000000000000000000"
	source := embeddedFontDoc(fontAssetBody, `["Noto Sans", {"asset": "`+missing+`"}]`)
	le := requireLoadError(t, source, "fonts.body[1].asset")
	if le.Value != missing {
		t.Errorf("load error value = %q, want the offending key %q", le.Value, missing)
	}
	if !strings.Contains(le.Reason, "assets") {
		t.Errorf("load error reason %q does not tell the author where the key should have been", le.Reason)
	}
}

// TestBadChainEntryShapeIsALocatedLoadError is the matrix's bad-shape row,
// over every shape the format refuses. The field is asserted with its INDEX in
// every case, and the index is deliberately not 0 in most of them: a refusal
// that hard-coded [0], or that lost the index entirely (the pre-8.3 behaviour,
// where the whole chain collapsed to one error at `fonts.body`), passes an
// index-0 case and fails these.
func TestBadChainEntryShapeIsALocatedLoadError(t *testing.T) {
	for _, tc := range []struct {
		name  string
		chain string
		field string
	}{
		{"a number", `["Noto Sans", 7]`, "fonts.body[1]"},
		{"an array", `["Noto Sans", ["Noto Sans"]]`, "fonts.body[1]"},
		{"null", `["Noto Sans", null]`, "fonts.body[1]"},
		{"true", `["Noto Sans", true]`, "fonts.body[1]"},
		{"an empty object", `["Noto Sans", {}]`, "fonts.body[1]"},
		// ⚠ `{"face": "Noto Sans"}` USED TO BE THIS ROW, AND STORY 11.2
		// MADE IT VALID: the object form's `face` discriminant is now a
		// legal entry. The genuine defect the row was standing for — an
		// entry object that names NO kind at all — is a sibling with no
		// discriminant beside it, so that is what the row asserts now.
		{"an object with no discriminant", `["Noto Sans", {"bold": "Noto Sans Bold"}]`, "fonts.body[1]"},
		// And the other direction, or "exactly one of" is asserted in
		// neither: BOTH discriminants in one object is the same defect.
		{"an object with both discriminants", `["Noto Sans", {"face": "Noto Sans", "asset": "` + embeddedFontKey + `"}]`, "fonts.body[1]"},
		{"an object with an extra key", `["Noto Sans", {"asset": "` + embeddedFontKey + `", "weight": 700}]`, "fonts.body[1]"},
		{"a non-string asset value", `["Noto Sans", {"asset": 7}]`, "fonts.body[1].asset"},
		{"a null asset value", `["Noto Sans", {"asset": null}]`, "fonts.body[1].asset"},
		{"an empty asset value", `["Noto Sans", {"asset": ""}]`, "fonts.body[1].asset"},
		// BOTH shapes of the empty face name, per D-8.3.2's guardrail. They
		// are not the same case. `["Noto Sans", ""]` is the one that
		// matters: before Story 8.3 it LOADED AND RENDERED — resolveRuneFace
		// silently skipped the empty entry and drew with Noto Sans, which is
		// the silent substitution AD-8 forbids by name. `[""]` alone never
		// rendered at all; it reached the existing no-usable-entry error. A
		// test of the second alone would pass while proving nothing about
		// the case whose observable behaviour actually changed.
		{"an empty face name beside a usable one", `["Noto Sans", ""]`, "fonts.body[1]"},
		{"an empty face name alone", `[""]`, "fonts.body[0]"},
		{"a bad entry at index 2", `["Noto Sans", "Noto Sans Thai", 7]`, "fonts.body[2]"},
		{"a bad entry at index 0", `[7, "Noto Sans"]`, "fonts.body[0]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireLoadError(t, embeddedFontDoc(fontAssetBody, tc.chain), tc.field)
		})
	}
}

// TestTheThreeObjectShapeRefusalsAreTOLDAPART is P6: an empty object,
// both discriminants and an unrecognised key are three DIFFERENT
// authoring mistakes at the same field, and a message that cannot tell
// them apart sends all three authors looking for the same thing.
//
// TestBadChainEntryShapeIsALocatedLoadError above asserts the FIELD
// only, so it passes whatever the sentence says — a refusal asserted
// only by its address is not asserted (D-000.25). This pins that each
// one names its own defect, and that each still carries the shared
// grammar so the author is told what they MAY write.
func TestTheThreeObjectShapeRefusalsAreTOLDAPART(t *testing.T) {
	reasons := map[string]string{}
	for _, tc := range []struct{ name, chain, mustSay string }{
		{"no discriminant", `[{"bold": "Noto Sans Bold"}]`, "neither"},
		{"both discriminants", `[{"face": "Noto Sans", "asset": "` + embeddedFontKey + `"}]`, "BOTH"},
		{"an unknown key", `[{"face": "Noto Sans", "weight": 700}]`, `"weight"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			le := requireLoadError(t, embeddedFontDoc(fontAssetBody, tc.chain), "fonts.body[0]")
			if !strings.Contains(le.Reason, tc.mustSay) {
				t.Errorf("the %s refusal %q does not say which defect this is (wanted it to mention %s)", tc.name, le.Reason, tc.mustSay)
			}
			// Every one still tells the author what they MAY write.
			for _, key := range append([]string{"face", "asset"}, fontChainVariantKeys()...) {
				if !strings.Contains(le.Reason, `"`+key+`"`) {
					t.Errorf("the %s refusal %q does not name the legal key %q", tc.name, le.Reason, key)
				}
			}
			reasons[tc.name] = le.Reason
		})
	}
	// And they are genuinely three sentences, not one repeated.
	if len(reasons) == 3 {
		for a, ra := range reasons {
			for b, rb := range reasons {
				if a < b && ra == rb {
					t.Errorf("the %q and %q refusals are byte-identical, so the author cannot tell which mistake they made:\n%s", a, b, ra)
				}
			}
		}
	}
	// The unknown-key refusal NAMES the offending key, which is what
	// makes decodeFontChainEntry's sorted scan load-bearing rather than
	// inert: an object with two unrecognised keys always reports the
	// first in byte order.
	le := requireLoadError(t, embeddedFontDoc(fontAssetBody, `[{"face": "Noto Sans", "weight": 700, "axis": 1}]`), "fonts.body[0]")
	if !strings.Contains(le.Reason, `"axis"`) {
		t.Errorf("with two unrecognised keys the refusal %q does not name the first in byte order (\"axis\")", le.Reason)
	}
}

// TestAnUnknownEntryKeyIsRefusedByTheCLOSEDSetAndSaysSo is the extra-key
// row's MESSAGE, asserted because the subject did not move but THE GROUND
// DID: `{"asset": …, "weight": 700}` was refused for breaking "exactly one
// key" until Story 11.2 and is refused for not being in the CLOSED key set
// now. requireLoadError checks the Field only, so the row above cannot see
// that change — a refusal asserted only by its address is not asserted
// (D-000.25).
//
// It pins the closed set's own members appearing in the sentence, DERIVED
// from the parser's enumeration rather than spelled here, so the assertion
// tracks the set instead of a wording.
func TestAnUnknownEntryKeyIsRefusedByTheCLOSEDSetAndSaysSo(t *testing.T) {
	le := requireLoadError(t,
		embeddedFontDoc(fontAssetBody, `["Noto Sans", {"asset": "`+embeddedFontKey+`", "weight": 700}]`),
		"fonts.body[1]")
	for _, key := range append([]string{"face", "asset"}, fontChainVariantKeys()...) {
		if !strings.Contains(le.Reason, `"`+key+`"`) {
			t.Errorf("the refusal %q does not name the legal key %q — the message must be derived from the closed set, not hand-written", le.Reason, key)
		}
	}
}

// TestASelfReferentialVariantSaysWHYAndSaysItFromTheCLOSEDSet is DW-241's
// MESSAGE, and it needs its own test for the reason the unknown-key row does:
// requireLoadError asserts the FIELD only, so the two rows added to
// TestADefectiveVariantIsRefusedAtTheSiblingItself cannot see the wording at
// all (D-000.25).
//
// It pins two things. First, the closed set's members appear in the sentence
// DERIVED from the parser's own enumeration rather than spelled here, so the
// three keys are never written down a second time. Second, the sentence says
// WHY — that such an entry declares nothing and does it silently — because
// "that is not allowed" sends an author to delete a key without ever learning
// that the cut they wanted has to be NAMED.
//
// ⚠ IT ASSERTS ON Error(), NOT ON THE Reason FIELD, and the difference is the
// whole point of the test. Error() is what the author reads, and it cuts Reason
// at loadErrorReasonRunes with a `…`. Asserting on the raw field passed happily
// over a 917-rune sentence whose rendered form stopped BEFORE the remedy and
// BEFORE "only the base is privileged" — a test that cannot see what the author
// sees is a test of something else.
func TestASelfReferentialVariantSaysWHYAndSaysItFromTheCLOSEDSet(t *testing.T) {
	for _, tc := range []struct{ name, chain, field string }{
		{"the face arm", `["Noto Sans", {"face": "Roboto", "bold": "Roboto"}]`, "fonts.body[1].bold"},
		{"the embedded arm", `[{"asset": "` + embeddedFontKey + `", "boldItalic": "` + embeddedFontKey + `"}]`, "fonts.body[0].boldItalic"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			le := requireLoadError(t, embeddedFontDoc(fontAssetBody, tc.chain), tc.field)
			read := le.Error()
			for _, key := range fontChainVariantKeys() {
				if !strings.Contains(read, `"`+key+`"`) {
					t.Errorf("the refusal %q does not name the closed set's member %q — the message must be DERIVED from the enumeration, not hand-written", read, key)
				}
			}
			// It says what is wrong, not merely that something is.
			for _, phrase := range []string{"OWN base", "silently", "no key at all"} {
				if !strings.Contains(read, phrase) {
					t.Errorf("the refusal %q never says %q, so an author is told a rule and not a reason", read, phrase)
				}
			}
			// AND IT NAMES THE ONE AXIS. Without this sentence the next reader
			// concludes that no two variants may agree, which is wider than the
			// narrowing that was ruled.
			if !strings.Contains(read, "only the base is privileged") {
				t.Errorf("the refusal %q does not say that only the BASE is privileged, so it reads as a ban on any two variants agreeing", read)
			}
			// NON-VACUITY, and it is what makes the four assertions above a
			// measurement rather than a hope: the rendered message really is
			// truncated, so landing the load-bearing clauses inside the window
			// was necessary and remains necessary.
			if !strings.Contains(read, loadErrorElision) {
				t.Errorf("the rendered refusal is no longer truncated, so this test no longer proves the load-bearing clauses come FIRST: %q", read)
			}
		})
	}
}

// TestACrossNamespaceVariantIsALocatedLoadError is AD-8's namespace match,
// asserted in BOTH directions. A sibling naming the other namespace is the
// substitution AD-8 forbids, arriving inside a single entry where no
// precedence rule can see it — an `asset` entry whose bold is a face name
// would draw a shipped face for a document that thinks it carries its own,
// and a `face` entry whose bold is an assets key would do the reverse.
func TestACrossNamespaceVariantIsALocatedLoadError(t *testing.T) {
	for _, tc := range []struct {
		name   string
		chain  string
		field  string
		reason string
	}{
		{
			"an embedded entry whose variant is a face name",
			`[{"asset": "` + embeddedFontKey + `", "bold": "Noto Sans Bold"}]`,
			"fonts.body[0].bold",
			"assets",
		},
		{
			"a face entry whose variant is an assets key",
			`["Noto Sans", {"face": "Noto Sans", "bold": "` + embeddedFontKey + `"}]`,
			"fonts.body[1].bold",
			"assets",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			le := requireLoadError(t, embeddedFontDoc(fontAssetBody, tc.chain), tc.field)
			if !strings.Contains(le.Reason, tc.reason) {
				t.Errorf("refusal %q does not tell the author which namespace applies here", le.Reason)
			}
		})
	}
}

// TestADefectiveVariantIsRefusedAtTheSiblingItself covers the remaining
// per-sibling refusals: each lands on `fonts.<chain>[<i>].<key>`, never on
// the entry, so the author is sent to the half that is wrong.
func TestADefectiveVariantIsRefusedAtTheSiblingItself(t *testing.T) {
	for _, tc := range []struct {
		name  string
		chain string
		field string
	}{
		{"an empty variant", `["Noto Sans", {"face": "Noto Sans", "bold": ""}]`, "fonts.body[1].bold"},
		{"a boolean variant", `["Noto Sans", {"face": "Noto Sans", "bold": true}]`, "fonts.body[1].bold"},
		{"a null variant", `["Noto Sans", {"face": "Noto Sans", "italic": null}]`, "fonts.body[1].italic"},
		{"a numeric boldItalic", `["Noto Sans", {"face": "Noto Sans", "boldItalic": 700}]`, "fonts.body[1].boldItalic"},
		{"an empty face discriminant", `["Noto Sans", {"face": "", "bold": "Noto Sans Bold"}]`, "fonts.body[1].face"},
		// D-11.2.11 / DW-241, ONE ROW PER ARM. A variant naming its entry's own
		// base declares nothing and renders bold-as-regular while bypassing the
		// Warning that exists to announce exactly that state — the one outcome
		// AC3 exists to prevent. The comparand is the ARM's own discriminant:
		// `entry.Face` on a face entry, `entry.AssetKey` on an embedded one.
		{"a face entry whose variant names its own face", `["Noto Sans", {"face": "Roboto", "bold": "Roboto"}]`, "fonts.body[1].bold"},
		{"an embedded entry whose variant names its own asset", `[{"asset": "` + embeddedFontKey + `", "italic": "` + embeddedFontKey + `"}]`, "fonts.body[0].italic"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireLoadError(t, embeddedFontDoc(fontAssetBody, tc.chain), tc.field)
		})
	}
	// The wrong-typed case says WHICH MEANING APPLIES HERE. `bold` is a
	// boolean on `style` and a FACE on a chain entry, and an author who
	// writes `true` has reached for the other one.
	le := requireLoadError(t, embeddedFontDoc(fontAssetBody, `["Noto Sans", {"face": "Noto Sans", "bold": true}]`), "fonts.body[1].bold")
	if !strings.Contains(le.Reason, "boolean") || !strings.Contains(le.Reason, "style") {
		t.Errorf("refusal %q does not distinguish a chain entry's `bold` from style's boolean of the same name", le.Reason)
	}
}

// TestChainThatIsNotAnArrayStillFailsAtTheChain keeps the pre-8.3 refusal for
// the one defect that has no entry index to name: the chain itself is not an
// array, so there is no entry to locate.
//
// It asserts the REASON as well as the field, which the Field-only version of
// this test did not. The message read "must be an array of strings" until
// Story 8.3, and that became false the moment an entry could be an object —
// unpinned wording in a refusal is wording that goes stale silently and sends
// the author to fix the one thing that was not wrong.
func TestChainThatIsNotAnArrayStillFailsAtTheChain(t *testing.T) {
	// `null` is NOT in this list, and its absence is measured rather than an
	// oversight: decodeArrayRaw has always read a JSON null as an empty array
	// (decodehelpers.go), so `"body": null` loaded as an empty chain before
	// Story 8.3 and still does. Widening that here would be a behaviour change
	// this story has no mandate for; it is asserted below instead, so the
	// tolerance is pinned rather than merely inherited.
	for _, chain := range []string{`"Noto Sans"`, `7`, `true`, `{"asset": "` + embeddedFontKey + `"}`} {
		le := requireLoadError(t, embeddedFontDoc(fontAssetBody, chain), "fonts.body")
		if !strings.Contains(le.Reason, "must be an array of font chain entries") {
			t.Errorf("chain %s: reason = %q, want it to name what a chain must be", chain, le.Reason)
		}
		if strings.Contains(le.Reason, "array of strings") {
			t.Errorf("chain %s: reason = %q still claims a chain is an array of STRINGS — an entry may be an object since Story 8.3", chain, le.Reason)
		}
	}
	// The inherited tolerance, stated: a null chain is an EMPTY chain, not a
	// refusal, exactly as it was before this story.
	d, err := ParseDocument([]byte(embeddedFontDoc(fontAssetBody, `null`)))
	if err != nil {
		t.Fatalf("a null chain loaded as an empty chain before Story 8.3 and must still: %v", err)
	}
	if chain, ok := d.Fonts.Chain("body"); ok || len(chain) != 0 {
		t.Errorf("a null chain = %#v, ok = %v — want an empty chain that Fonts.Chain declines to name", chain, ok)
	}
}

// TestRecognisedFontMediaTypeWithWrongBytesIsALoadError is the matrix's
// lying-file row. font/ttf is RECOGNISED, so bytes that are not sfnt are
// refused at load — reader-independent, exactly as a recognised image type
// whose bytes are not that image is.
func TestRecognisedFontMediaTypeWithWrongBytesIsALoadError(t *testing.T) {
	// "not an sfnt at all, but a valid, non-empty, correctly-keyed asset" —
	// the digest is of these bytes, so the only thing wrong with the document
	// is that it lies about the container.
	const notSfntKey = "9878aff9a8f523afac3198c3b7b8c00a0a2e89dfb02b77bd03178c9670473a11"
	const notSfntData = `[
        "VGhpcyBpcyBub3QgYSBmb250LiBJdCBpcyBqdXN0IHNvbWUgdGV4dCB0aGF0IGlzIGxvbmcgZW5v",
        "dWdoIHRvIHdyYXAgb250byB0d28gY2Fub25pY2FsIGxpbmVzLg=="
      ]`
	source := `{
  "assets": {
    "` + notSfntKey + `": {
      "data": ` + notSfntData + `,
      "mediaType": "font/ttf"
    }
  },
  "bands": {
    "content": {"elements": []},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 1,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}`
	le := requireLoadError(t, source, "assets."+notSfntKey+".data")
	if !strings.Contains(le.Reason, "sfnt") {
		t.Errorf("load error reason %q does not say what was wrong with the bytes", le.Reason)
	}
	// And a TRUNCATED real sfnt — a correct version tag and a correct table
	// directory over a file that stops in the middle of the first table. This
	// is the case a version-tag-only sniff lets straight through, and it is
	// the reason checkSfnt reads the directory's offsets at all: without it
	// the truncation surfaces far from the loader, inside a shaper.
	const truncatedKey = "2e93003babae68f5bffbd0c519392d7d410f7ee77b1fae2d61d654578f307683"
	truncated := strings.Replace(source, notSfntKey, truncatedKey, 1)
	truncated = strings.Replace(truncated, notSfntData, `[
        "AAEAAAADACAABAAQY21hcAAAAAAAAAA8AAAAIGdseWYAAAAAAAAAXAAAACBoZWFkAAAAAAAAAHwA",
        "AAAgQ01BUERB"
      ]`, 1)
	le = requireLoadError(t, truncated, "assets."+truncatedKey+".data")
	if !strings.Contains(le.Reason, "truncated") {
		t.Errorf("a truncated sfnt must be diagnosed as truncated, got: %s", le.Reason)
	}
}

// TestCollectionUnderASingleFaceMediaTypeIsALoadError closes a hole the
// version-tag check had while `checkSfnt` returned nil for 'ttcf'.
//
// THE HOLE, MEASURED. `mediaType` is AUTHOR-DECLARED. Nothing inspects the
// bytes before the media type is read, so `{"mediaType": "font/ttf"}` over TTC
// bytes IS recognised, DOES reach checkSfnt, and used to load clean — while
// folio-format.md promises a recognised type whose bytes are not that format is
// a load error. The old branch justified itself with "no recognised media type
// reaches here with it anyway", which was an unmeasured negative and false.
//
// The pairing is what makes this a rule rather than a rejection: the SAME bytes
// under `font/collection` — an unrecognised type — load clean and are preserved
// verbatim, on D-1.8.1's amended path. So a document that really means to carry
// a collection has a legal spelling, and only the MISLABELLED one is refused.
func TestCollectionUnderASingleFaceMediaTypeIsALoadError(t *testing.T) {
	const ttcKey = "1ae2b2a5570037299d094fbac8406acc533bd9d126b129f0954826829b325c09"
	const ttcData = `[
        "dHRjZgABAAAAAAABAAAAEAABAAAAAQAQAAQAAGNtYXAAAAAAAAAALAAAACBDTUFQREFUQUNNQVBE",
        "QVRBQ01BUERBVEFDTUFQREFUQQ=="
      ]`
	doc := func(mediaType string) string {
		return `{
  "assets": {
    "` + ttcKey + `": {
      "data": ` + ttcData + `,
      "mediaType": "` + mediaType + `"
    }
  },
  "bands": {
    "content": {"elements": []},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 1,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}`
	}

	// Both RECOGNISED single-face types refuse it, and the message names what
	// the bytes actually are rather than only that they are wrong.
	for _, mediaType := range []string{"font/ttf", "font/otf"} {
		t.Run(mediaType, func(t *testing.T) {
			le := requireLoadError(t, doc(mediaType), "assets."+ttcKey+".data")
			if !strings.Contains(le.Reason, "collection") {
				t.Errorf("reason = %q, want it to name the bytes as a collection", le.Reason)
			}
			if !strings.Contains(le.Reason, "font/collection") {
				t.Errorf("reason = %q, want it to name the media type the author should have declared", le.Reason)
			}
		})
	}

	// And the honest spelling loads clean, unread and preserved.
	t.Run("font/collection", func(t *testing.T) {
		if _, err := ParseDocument([]byte(doc("font/collection"))); err != nil {
			t.Fatalf("font/collection is UNRECOGNISED, so its bytes are never inspected and it must load clean (D-1.8.1 amended), got: %v", err)
		}
		canonicalFixedPoint(t, doc("font/collection"))
	})

	// The direct check, so the rule is pinned at the predicate too: the same
	// bytes are recognised-and-refused under one type and never looked at
	// under the other.
	raw, err := decodeBase64Asset([]string{"dHRjZgABAAAAAAABAAAAEAABAAAAAQAQAAQAAGNtYXAAAAAAAAAALAAAACBDTUFQREFUQUNNQVBE", "QVRBQ01BUERBVEFDTUFQREFUQQ=="})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if recognised, ferr := decodeRecognisedFont("font/ttf", raw); !recognised || ferr == nil {
		t.Errorf("decodeRecognisedFont(font/ttf, ttc) = recognised %v, err %v — want recognised with an error", recognised, ferr)
	}
	if recognised, ferr := decodeRecognisedFont("font/collection", raw); recognised || ferr != nil {
		t.Errorf("decodeRecognisedFont(font/collection, ttc) = recognised %v, err %v — want unrecognised and never inspected", recognised, ferr)
	}
}

// TestUnrecognisedFontMediaTypeLoadsClean IS THE POSITIVE CONTROL, and it is
// the arm a "closed set" implementation fails.
//
// D-1.8.1 as amended: an unrecognised mediaType is never inspected at load and
// never refused there. The document is VALID and the asset is preserved
// verbatim; the failure, if any, arrives at render and only when a render
// actually needs the face. This asserts load AND Validate AND byte-preserving
// round trip, because "it parsed" alone would not show the asset survived.
func TestUnrecognisedFontMediaTypeLoadsClean(t *testing.T) {
	for _, mediaType := range []string{"font/woff2", "font/woff", "font/collection", "application/x-font-ttf", "font/ttf-but-not-really"} {
		t.Run(mediaType, func(t *testing.T) {
			// The bytes are the same sfnt, so the ONLY thing this row varies
			// is recognition — a row whose bytes were also wrong could pass
			// for the wrong reason.
			source := embeddedFontDoc(fontAssetBodyOfType(mediaType), embeddedChain)
			d, err := ParseDocument([]byte(source))
			if err != nil {
				t.Fatalf("an unrecognised font media type must LOAD CLEAN (D-1.8.1 amended), got: %v", err)
			}
			if got := d.Assets[embeddedFontKey].MediaType; got != mediaType {
				t.Fatalf("mediaType = %q, want it preserved verbatim as %q", got, mediaType)
			}
			canonical := canonicalFixedPoint(t, source)
			if !strings.Contains(canonical, `"mediaType": "`+mediaType+`"`) {
				t.Fatalf("the unrecognised media type was not preserved verbatim:\n%s", canonical)
			}
		})
	}

	// And the bytes really are NOT inspected: an unrecognised type over
	// non-sfnt bytes loads too. If this row failed, recognition would be
	// doing nothing and the row above would be passing on the bytes alone.
	const notSfntKey = "9878aff9a8f523afac3198c3b7b8c00a0a2e89dfb02b77bd03178c9670473a11"
	source := `{
  "assets": {
    "` + notSfntKey + `": {
      "data": [
        "VGhpcyBpcyBub3QgYSBmb250LiBJdCBpcyBqdXN0IHNvbWUgdGV4dCB0aGF0IGlzIGxvbmcgZW5v",
        "dWdoIHRvIHdyYXAgb250byB0d28gY2Fub25pY2FsIGxpbmVzLg=="
      ],
      "mediaType": "font/woff2"
    }
  },
  "bands": {
    "content": {"elements": []},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {},
  "locale": "en",
  "nextId": 1,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}`
	if _, err := ParseDocument([]byte(source)); err != nil {
		t.Fatalf("an unrecognised media type must never have its bytes inspected at load, got: %v", err)
	}
}

// TestUnrecognisedFontMediaTypeErrorsOnlyAtTheRenderSurface is the other half
// of the same rule: the capability error EXISTS, and it is reachable only
// through the render-surface predicate, never through the loader.
//
// Story 8.4 widened the error's address from (asset, element) to the whole
// FontChainSite — the chain NAME and the ENTRY INDEX are what make it
// actionable, because a chain is shared by many elements and "asset K is not
// a font" does not say which of a chain's entries to go and edit. Both new
// coordinates are asserted in the MESSAGE and not only in the struct: a field
// that never reaches a reader is a field that is not carrying the error.
func TestUnrecognisedFontMediaTypeErrorsOnlyAtTheRenderSurface(t *testing.T) {
	site := FontChainSite{AssetKey: embeddedFontKey, ElementID: "e1", ChainName: "body", EntryIndex: 1}
	err := DecodeFontForRender("font/woff2", []byte("whatever"), site)
	var unsupported *UnsupportedFontMediaTypeError
	if !errors.As(err, &unsupported) {
		t.Fatalf("DecodeFontForRender over an unrecognised type = %v, want an *UnsupportedFontMediaTypeError", err)
	}
	if unsupported.MediaType != "font/woff2" || unsupported.Site != site {
		t.Errorf("the capability error does not locate itself: %#v", unsupported)
	}
	for _, want := range []string{
		"the document is valid",     // the DOCUMENT is valid; this is a capability limit
		"element e1",                // where it was asked for
		"asset " + embeddedFontKey,  // which asset
		`font chain "body" entry 1`, // WHICH ENTRY of which chain — Story 8.4's addition
	} {
		if !strings.Contains(unsupported.Error(), want) {
			t.Errorf("the capability error's message is missing %q, got: %s", want, unsupported.Error())
		}
	}
	// The element clause is OMITTED rather than printed as a hole when the
	// caller supplies its own located prefix — the render path's shape.
	unlocated := &UnsupportedFontMediaTypeError{
		Site:      FontChainSite{AssetKey: embeddedFontKey, ChainName: "body", EntryIndex: 1},
		MediaType: "font/woff2",
	}
	if strings.Contains(unlocated.Error(), "element ") {
		t.Errorf("an empty ElementID must be omitted, not printed as a hole: %s", unlocated.Error())
	}
	// A recognised type over good bytes is accepted by the same predicate.
	if err := DecodeFontForRender("font/ttf", embeddedFontRawBytes(t), site); err != nil {
		t.Errorf("DecodeFontForRender over a recognised type and valid bytes = %v, want nil", err)
	}
}

// embeddedFontRawBytes decodes embeddedFontData through this package's own
// decoder rather than a second copy of one.
func embeddedFontRawBytes(t *testing.T) []byte {
	t.Helper()
	d, err := ParseDocument([]byte(embeddedFontDoc(fontAssetBody, embeddedChain)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	raw, err := decodeBase64Asset(d.Assets[embeddedFontKey].Data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := sha256HexOf(raw); got != embeddedFontKey {
		t.Fatalf("fixture assumption violated: sha256(decoded) = %s, want %s", got, embeddedFontKey)
	}
	return raw
}

// TestEmbeddedEntryRaisesTheVersionAndAFontAssetAloneDoesNot is the matrix's
// two version rows, at the versionForSave surface rather than only at the
// internal probe — that is the surface a saved file's declared version
// actually comes from.
func TestEmbeddedEntryRaisesTheVersionAndAFontAssetAloneDoesNot(t *testing.T) {
	withEntry, err := ParseDocument([]byte(embeddedFontDoc(fontAssetBody, embeddedChain)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := versionRequiredByContent(withEntry); got != "2.0" {
		t.Errorf("a document with an embedded-face entry requires %q, want 2.0", got)
	}

	// The same asset, unreferenced. Version 1.0 declared, and it must STAY
	// 1.0: a 1.x reader loads this document and renders it correctly.
	assetOnly := strings.Replace(embeddedFontDoc(fontAssetBody, `["Noto Sans"]`), `"version": "2.0"`, `"version": "1.0"`, 1)
	d, err := ParseDocument([]byte(assetOnly))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := versionRequiredByContent(d); got != "1.0" {
		t.Errorf("a document carrying a font ASSET but naming no embedded entry requires %q, want 1.0 — the trigger is the entry (D-1.4.13)", got)
	}
	if got := versionForSave(d.Version, d); got != "1.0" {
		t.Errorf("versionForSave over an unreferenced font asset = %q, want 1.0", got)
	}
}

// TestPlainFontAssetNeedsNoRecord is the "absence costs nothing" row at the
// asset level: a font asset with no `font` record round-trips without the key
// appearing at all.
func TestPlainFontAssetNeedsNoRecord(t *testing.T) {
	source := embeddedFontDoc(plainFontAssetBody, unreferencedChain)
	d, err := ParseDocument([]byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if d.Assets[embeddedFontKey].Font.Set {
		t.Errorf("a document declaring no font record must parse to an ABSENT one, got %#v", d.Assets[embeddedFontKey].Font)
	}
	canonical := canonicalFixedPoint(t, source)
	if strings.Contains(canonical, `"font"`) {
		t.Fatalf("an absent font record must not reappear on serialize:\n%s", canonical)
	}
}

// TestLoadNeitherResolvesNorRefusesAnEmbeddedEntry is what
// TestEmbeddedEntryIsInertUntilStory84 became when Story 8.4 landed, and it
// was renamed and re-aimed rather than retired.
//
// THE CLAIM IT USED TO MAKE IS NOW FALSE. It pinned that "the render path
// acquires a call site for DecodeFontForRender in Story 8.4" as a NEGATIVE
// about the present, and 8.4 is the present: folio8.fontCache.get (render.go)
// calls it. A negative assertion whose subject has arrived is a test that
// passes while documenting the opposite of the truth.
//
// WHAT IS MEASURED NOW, and it is the half that is still true and still worth
// a test: this package's LOAD path neither resolves an embedded entry to a
// face nor refuses a chain for holding one. That is D-1.8.1 as amended —
// preserve at load, error at render — and it is the property Story 8.4 must
// not have "fixed" by tightening decodeFontChainEntry. Where an embedded
// entry's bytes are actually read is a question about `package folio8`, which
// this package structurally cannot reach; folio8/chain_face_names_test.go is
// where that half lives.
func TestLoadNeitherResolvesNorRefusesAnEmbeddedEntry(t *testing.T) {
	// A chain of NOTHING BUT an embedded entry loads. It is not refused for
	// holding no face the caller supplies — the format can express it, and
	// since Story 8.4 the renderer can draw it.
	source := embeddedFontDoc(fontAssetBody, `[{"asset": "`+embeddedFontKey+`"}]`)
	d, err := ParseDocument([]byte(source))
	if err != nil {
		t.Fatalf("a chain of only embedded entries must LOAD, got: %v", err)
	}
	chain, ok := d.Fonts.Chain("body")
	if !ok || len(chain) != 1 || !chain[0].Embedded() {
		t.Fatalf("chain = %#v, ok = %v — want one embedded entry", chain, ok)
	}
	// And the entry carries NO face name: the LOADER resolves nothing. The
	// render path derives a face name from the ASSET KEY (D-8.4.1), at render,
	// and never writes one back into the parsed document.
	if chain[0].Face != "" {
		t.Errorf("an embedded entry must not acquire a face name at load, got %q — resolution is the render path's, from the asset key", chain[0].Face)
	}
}

// ---------------------------------------------------------------------------
// STORY 8.6 — AN ASSET A CHAIN NAMES MUST STATE ITS TERMS.
//
// The rule and its scope are one thing, not two: it is REQUIRED of an asset a
// chain names, and it does not exist for one nothing names. So the two arms
// are tested together, off ONE asset body, with only the chain varying —
// otherwise a passing pair could be passing because two different documents
// happened to differ somewhere else.

// TestAReferencedFaceMustStateItsTerms is the refusal, per missing key and per
// way of missing it.
//
// EACH KEY IS TESTED THROUGH BOTH ITS ABSENT AND ITS EXPLICITLY-NULL ARM, and
// through the empty string. Presence has three states (presence.go), and a
// guard written only for the absent case would admit `"licenceText": null` — a
// document stating in as many words that it carries no terms. `""` is refused
// for the same reason: a key that is present and says nothing.
func TestAReferencedFaceMustStateItsTerms(t *testing.T) {
	const (
		copyrightKey   = `"copyright": "Copyright 2026 The Folio Fixture Authors"`
		licenceKey     = `"licence": "SIL Open Font License 1.1"`
		licenceTextKey = `"licenceText": "the whole text, verbatim"`
	)
	whole := []string{copyrightKey, licenceKey, licenceTextKey}

	// The control comes FIRST and is not a formality: every row below is a
	// mutation of this document, so a row that reddened for an unrelated
	// reason would be indistinguishable from the rule working.
	if _, err := ParseDocument([]byte(embeddedFontDoc(fontAssetWithRecord(`{`+strings.Join(whole, ", ")+`}`), embeddedChain))); err != nil {
		t.Fatalf("control: a referenced face carrying all three required keys must LOAD, got: %v", err)
	}

	for _, tc := range []struct {
		name   string
		record string
		field  string
	}{
		{"no font record at all", "", "assets." + embeddedFontKey + ".font.licence"},
		{"an explicitly null font record", "null", "assets." + embeddedFontKey + ".font.licence"},
		{"an empty font record", "{}", "assets." + embeddedFontKey + ".font.licence"},

		{"licence absent", `{` + copyrightKey + `, ` + licenceTextKey + `}`, "assets." + embeddedFontKey + ".font.licence"},
		{"licence null", `{` + copyrightKey + `, "licence": null, ` + licenceTextKey + `}`, "assets." + embeddedFontKey + ".font.licence"},
		{"licence empty", `{` + copyrightKey + `, "licence": "", ` + licenceTextKey + `}`, "assets." + embeddedFontKey + ".font.licence"},
		// BLANK IS EMPTY. A space and a newline state exactly what "" states,
		// and a guard written as `== ""` admits both — a face travelling with
		// terms that say nothing, which is the condition the rule exists to
		// prevent.
		{"licence blank", `{` + copyrightKey + `, "licence": " ", ` + licenceTextKey + `}`, "assets." + embeddedFontKey + ".font.licence"},

		{"licenceText absent", `{` + copyrightKey + `, ` + licenceKey + `}`, "assets." + embeddedFontKey + ".font.licenceText"},
		{"licenceText null", `{` + copyrightKey + `, ` + licenceKey + `, "licenceText": null}`, "assets." + embeddedFontKey + ".font.licenceText"},
		{"licenceText empty", `{` + copyrightKey + `, ` + licenceKey + `, "licenceText": ""}`, "assets." + embeddedFontKey + ".font.licenceText"},
		{"licenceText blank", `{` + copyrightKey + `, ` + licenceKey + `, "licenceText": "  \n\t "}`, "assets." + embeddedFontKey + ".font.licenceText"},

		{"copyright absent", `{` + licenceKey + `, ` + licenceTextKey + `}`, "assets." + embeddedFontKey + ".font.copyright"},
		{"copyright null", `{"copyright": null, ` + licenceKey + `, ` + licenceTextKey + `}`, "assets." + embeddedFontKey + ".font.copyright"},
		{"copyright empty", `{"copyright": "", ` + licenceKey + `, ` + licenceTextKey + `}`, "assets." + embeddedFontKey + ".font.copyright"},
		{"copyright blank", `{"copyright": "\n", ` + licenceKey + `, ` + licenceTextKey + `}`, "assets." + embeddedFontKey + ".font.copyright"},

		// A record that names the terms of a DIFFERENT face — family and
		// source present, terms absent — is the shape a writer that forgot
		// this rule would emit, so it is a row rather than an assumption.
		{"display identity only", `{"family": "Maximal Sans", "source": "somewhere", "style": "Regular"}`, "assets." + embeddedFontKey + ".font.licence"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := "\n      \"data\": " + embeddedFontData + ",\n      \"mediaType\": \"font/ttf\""
			if tc.record != "" {
				body = fontAssetWithRecord(tc.record)
			}
			le := requireLoadError(t, embeddedFontDoc(body, embeddedChain), tc.field)
			// THE ERROR LOCATES BOTH HALVES. The Field is the asset record —
			// where the fix goes — and the message names the chain entry that
			// made this asset an embedded face. Either coordinate alone sends
			// the reader to a place that cannot be acted on.
			if !strings.Contains(le.Reason, "fonts.body[1]") {
				t.Errorf("the refusal does not name the chain entry that makes the asset an embedded face: %s", le.Reason)
			}
		})
	}
}

// TestAnUnreferencedFontAssetIsNotAnEmbeddedFace is the rule's OTHER half, and
// it is the half that keeps D-1.4.13 intact.
//
// The identical asset — same bytes, same key, same absent or partial record —
// is legal when no chain names it, because then no face is being redistributed
// on its account. It loads clean, it round-trips, and it does NOT raise the
// document's version. A rule scoped to the asset rather than to the reference
// would have broken all three.
func TestAnUnreferencedFontAssetIsNotAnEmbeddedFace(t *testing.T) {
	for _, tc := range []struct {
		name   string
		record string
	}{
		{"no font record at all", ""},
		{"an explicitly null record", "null"},
		{"an empty record", "{}"},
		{"display identity and no terms", `{"family": "Maximal Sans", "style": "Regular"}`},
		{"an explicitly null licenceText", `{"licence": "SIL Open Font License 1.1", "licenceText": null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := "\n      \"data\": " + embeddedFontData + ",\n      \"mediaType\": \"font/ttf\""
			if tc.record != "" {
				body = fontAssetWithRecord(tc.record)
			}
			source := embeddedFontDoc(body, unreferencedChain)
			d, err := ParseDocument([]byte(source))
			if err != nil {
				t.Fatalf("a font asset NO chain names must load clean — it is not an embedded face: %v", err)
			}
			if _, ok := d.Assets[embeddedFontKey]; !ok {
				t.Fatal("the unreferenced asset was dropped at load; the loader preserves, it does not collect")
			}
			canonicalFixedPoint(t, source)
			// D-1.4.13: version describes the DOCUMENT, and this one is
			// legible to a 1.x reader exactly as it stands.
			if got := versionRequiredByContent(d); got != "1.0" {
				t.Errorf("an unreferenced font asset requires version %q, want 1.0 — the trigger is the ENTRY, and this rule must not have become a second one", got)
			}
		})
	}
}
