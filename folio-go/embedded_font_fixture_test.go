package folio8

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtures/embedded-font/ is Story 8.3's artifact and Story 8.4's subject:
// THE FIRST `.folio` IN THIS REPOSITORY THAT CARRIES A FONT FACE, and now the
// first whose page is DRAWN with one.
//
// WHAT IT RED-PROVES: that a face can travel inside a template — stored,
// loaded, round-tripped, projected — through the same `assets` mechanism
// images already use, that a document declaring one declares format version
// 2.0, and (Story 8.4) that the renderer draws from it, on four targets, with
// no such face installed on the machine and none supplied for it in the
// FontSet.
//
// THE DOCUMENT'S TEXT IS PURE THAI, AND THAT IS THE WHOLE MEASUREMENT
// (D-8.4.4b). Measured from the shipped cmaps: NotoSans-Regular covers ZERO
// codepoints in U+0E00–U+0E7F, and NotoSansThai-Regular covers 87 of them. The
// chain is ["Noto Sans", <the carried face>], so every rune on this page falls
// through the shipped entry and can only be drawn by the face the document
// carries. Story 8.3's version drew LATIN — which the carried Thai face also
// covers — so its bytes could not tell a correct implementation from an inert
// one, and its digest observed nothing. A PURE-THAI string is the sharp
// witness; a mixed one is not.
//
// IT SHIPPED NO expected.pdf UNTIL STORY 8.4, and that was correct: an
// `expected.pdf` is a human-attested artifact under AD-21/D-4.7.1, and Story
// 8.3 could not produce the one that matters — a page drawn WITH the embedded
// face. Recording a page drawn with the SHIPPED face under the name
// "embedded-font" would have attested the wrong thing. Story 8.4 produces that
// page, so the golden ships and is registered in goldenDigestRecord, which
// now holds 23 entries.
//
// THE COUNT ASSERTIONS ARE GONE, DELIBERATELY (Story 8.4, TRAP 1). This file
// and matrix_test.go both used to assert that the render embeds EXACTLY ONE
// font program, as the honest statement of the interim state. With a pure-Thai
// document on this chain a CORRECT Story 8.4 also embeds exactly one program —
// the carried Thai face instead of the shipped Latin one — so the count passes
// identically before and after and certifies the opposite of what it says.
// Both are IDENTITY assertions now: WHICH face reached the page, read off the
// produced bytes.

// embeddedFontAssetBytes is the face the fixture carries: the SHIPPED Noto
// Sans Thai, embedded a second time as an ASSET rather than supplied through
// the FontSet. NO NEW BINARY ENTERS THE REPOSITORY — this is the same
// folio-go/fonts/notosansthai/NotoSansThai-Regular.ttf the module already
// commits, reached through the test binary's own embed (testfont_embed_test.go)
// so the bytes travel to every cross-target leg, js/wasm included.
//
// Thai deliberately, and since Story 8.4 THAI TEXT ON THE PAGE deliberately:
// the shipped Latin face the chain names first covers none of it, so the face
// the document carries is the only face that can draw this page. Story 8.3
// drew Latin here, on the opposite reasoning — that the carried face was the
// one the page did NOT need — which was right for a story that rendered from
// it nowhere and is exactly wrong for one that does.
func embeddedFontAssetBytes() []byte { return testShippedNotoSansThai }

// embeddedFontAssetKey is the key the format's own rule produces: the
// lowercase hex SHA-256 of the DECODED bytes. It is derived here rather than
// written down, because a written-down digest is a second authority on what
// the key is, and this fixture exists partly to show there is only one.
func embeddedFontAssetKey() string {
	sum := sha256.Sum256(embeddedFontAssetBytes())
	return fmt.Sprintf("%x", sum)
}

// embeddedFontTemplateJSON is fixtures/embedded-font/input.folio, BUILT rather
// than transcribed.
//
// Every other matrix document keeps its template as a hand-copied Go string
// constant. This one cannot: its asset is ~47 KB of base64, and a hand-copied
// constant of that size is a diff nobody reads and a drift nobody sees. So the
// document is generated from the shipped bytes by the format's own rules —
// SHA-256 for the key, canonical 76-column wrapping for the data — and
// TestEmbeddedFontFixtureMatchesInputFolio8 pins the committed file against it.
// The tie is the same one every other fixture has; only its direction is
// reversed (the constant is derived, the file is checked), and that is
// strictly stronger, because the derivation is the format's rule rather than a
// human's copy of its output.
// embeddedFontLicenceText and embeddedFontCopyright are the two keys Story
// 8.6 made REQUIRED of an asset a chain names, and neither is written down
// here: the text is the committed `LICENSE-OFL.txt` beside the face, and the
// copyright is that file's own first line. Copying either into a Go literal
// would create the second authority the whole rule exists to prevent — a
// document could then state terms the bytes beside it contradict, and nothing
// would red.
func embeddedFontLicenceText() string { return strings.TrimRight(testShippedNotoSansThaiLicence, "\n") }

func embeddedFontCopyright() string {
	line, _, _ := strings.Cut(testShippedNotoSansThaiLicence, "\n")
	return strings.TrimSpace(line)
}

// jsonStringLiteral quotes a value the way the serializer's own
// appendJSONString does — MINIMAL escaping, so `&`, `<`, `>` and `/` travel
// literally. encoding/json's default HTML escaping would spell the OFL's
// "PERMISSION & CONDITIONS" as `&`, the serializer would rewrite it back
// on save, and the fixture would stop being a fixed point. SetEscapeHTML(false)
// is what makes the two agree, and TestEmbeddedFontFixtureIsCanonical… is what
// proves they do rather than assuming it.
func jsonStringLiteral(value string) string {
	var out strings.Builder
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		panic("embedded-font fixture: " + err.Error())
	}
	return strings.TrimRight(out.String(), "\n")
}

func embeddedFontTemplateJSON() string {
	encoded := base64Wrapped76(embeddedFontAssetBytes())
	var data strings.Builder
	for i, line := range encoded {
		if i > 0 {
			data.WriteString(",\n")
		}
		data.WriteString("        \"" + line + "\"")
	}
	return `{
  "assets": {
    "` + embeddedFontAssetKey() + `": {
      "data": [
` + data.String() + `
      ],
      "font": {
        "copyright": ` + jsonStringLiteral(embeddedFontCopyright()) + `,
        "family": "Noto Sans Thai",
        "licence": "SIL Open Font License 1.1",
        "licenceText": ` + jsonStringLiteral(embeddedFontLicenceText()) + `,
        "source": "folio-go/fonts/notosansthai/NotoSansThai-Regular.ttf — the shipped static Regular instance; see that directory's NOTICE.md for the derivation",
        "style": "Regular"
      },
      "mediaType": "font/ttf"
    }
  },
  "bands": {
    "content": {
      "elements": [
        {
          "height": 40,
          "id": "e1",
          "style": {
            "fontFamily": "body",
            "fontSize": 12
          },
          "type": "text",
          "value": "สัญญา",
          "width": 400,
          "x": 0,
          "y": 0
        }
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 20
    },
    "pageHeader": {
      "elements": [],
      "height": 20
    }
  },
  "fonts": {
    "body": [
      "Noto Sans",
      {
        "asset": "` + embeddedFontAssetKey() + `"
      }
    ]
  },
  "locale": "en",
  "nextId": 2,
  "page": {
    "margin": {
      "bottom": 36,
      "left": 36,
      "right": 36,
      "top": 36
    },
    "orientation": "portrait",
    "size": "A4"
  },
  "utcOffset": "+00:00",
  "version": "2.0"
}
`
}

// requiredLicenceKeys is the three keys Story 8.6 made REQUIRED of any asset a
// font chain names by {"asset": key}, as the inner text of a `font` record.
//
// It is a TEST-FIXTURE spelling and not a licence: the values are short
// placeholders, because the load rule this package's tests are downstream of
// asks only that each key is present, non-null and non-empty. The one document
// in this repository that carries REAL terms is fixtures/embedded-font/, and it
// gets them from the committed LICENSE file rather than from a literal.
//
// It is here, beside the fixture, rather than in each test file, because four
// of them splice it into a document for the same reason — a test whose subject
// is the projection, the render path or a non-font asset must satisfy the
// licence rule to reach its own subject at all, and four copies of these three
// keys would be four chances to spell one of them wrong and never find out.
const requiredLicenceKeys = `"copyright": "Copyright 2026 The Folio Fixture Authors", "licence": "SIL Open Font License 1.1", "licenceText": "This fixture face is licensed under the SIL Open Font License, Version 1.1."`

// base64Wrapped76 reproduces the format's canonical `data` wrapping. It is a
// SECOND site for that rule, and the only reason it is tolerable is that
// TestEmbeddedFontFixtureIsCanonical below asserts the whole document is a
// serializer fixed point — so if this wrapping ever disagreed with
// splitBase64Canonical, the fixture would stop round-tripping and this file
// would go red rather than shipping a document the engine rewrites on save.
func base64Wrapped76(raw []byte) []string {
	const width = 76
	encoded := base64.StdEncoding.EncodeToString(raw)
	var out []string
	for len(encoded) > width {
		out = append(out, encoded[:width])
		encoded = encoded[width:]
	}
	return append(out, encoded)
}

// TestEmbeddedFontFixtureMatchesInputFolio8 ties the committed file to the
// template the matrix actually renders. Without it the two drift, and the
// fixture stops documenting what the matrix measured.
func TestEmbeddedFontFixtureMatchesInputFolio8(t *testing.T) {
	path := filepath.Join(repoRootFromTest(t), "fixtures", "embedded-font", "input.folio")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(raw) == 0 {
		t.Fatal("presence precondition: fixtures/embedded-font/input.folio is empty")
	}
	if string(raw) != embeddedFontTemplateJSON() {
		t.Errorf("embeddedFontTemplateJSON and fixtures/embedded-font/input.folio have DIVERGED — the matrix renders one and the repository documents the other")
	}
}

// TestEmbeddedFontFixtureIsCanonicalAndDeclaresTwoPointZero is the fixture's
// own acceptance, and it is what makes AC6 structural rather than absent.
func TestEmbeddedFontFixtureIsCanonicalAndDeclaresTwoPointZero(t *testing.T) {
	source := embeddedFontTemplateJSON()
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if string(out) != source {
		t.Fatalf("the fixture is not a serializer fixed point — the engine would rewrite it on save:\n--- got ---\n%s\n--- want ---\n%s", firstBytes(string(out)), firstBytes(source))
	}
	// The version the fixture DECLARES is the version its content REQUIRES:
	// an embedded-face entry is a 2.0 document (Story 8.3).
	if !strings.Contains(source, `"version": "2.0"`) {
		t.Error("a document declaring an embedded-face entry must declare version 2.0")
	}
	// And it really renders — from the face the document CARRIES, against a
	// FontSet that supplies no face capable of drawing a word of it.
	res, err := Render(tpl, Data(`{}`), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("the fixture rendered zero bytes")
	}
	if len(res.Diagnostics) != 0 {
		t.Fatalf("the fixture rendered with diagnostics — its runes fell through the chain uncovered: %+v", res.Diagnostics)
	}
	// THE FACE ON THE PAGE, PINNED BY IDENTITY AND NEVER BY COUNT (Story
	// 8.4, TRAP 1). A count of embedded programs is exactly 1 both before
	// this story and after it — the shipped Latin face then, the carried Thai
	// face now — so it distinguishes nothing.
	requireEmbeddedFaceDrewThePage(t, "the fixture's render", res.Bytes)
}

// requireEmbeddedFaceDrewThePage reads, off produced PDF bytes, that
// fixtures/embedded-font/'s page was drawn with the face the document CARRIES
// and with no other.
//
// It is stated in three ways, because each catches a different wrong answer:
//
//   - the font RESOURCE NAME derived from the asset key is present. Only a
//     face resolved through the document's own assets can produce it — it IS
//     the asset key, and internal/pdf spells a resource name from the caller's
//     FontSet key. This is the positive identity.
//   - the shipped LATIN face is absent. It is the chain's FIRST entry, so a
//     build that resolved by name, or that fell back when coverage failed,
//     would put it on the page.
//   - /BaseFont names the Thai program. The six-letter subset tag is stripped
//     off, so this reads the embedded program's OWN PostScript name (ISO
//     32000-1 Table 117) rather than any key the renderer chose.
//
// It is shared by this file and by matrix_test.go's per-leg guard, so the four
// cross-target legs and the in-process render assert the identical property
// rather than two drifting spellings of it.
func requireEmbeddedFaceDrewThePage(t *testing.T, label string, raw []byte) {
	t.Helper()
	body := string(raw)
	if want := pdfEscapedEmbeddedFaceName(embeddedFontAssetKey()); !strings.Contains(body, want) {
		t.Errorf("%s: no font resource named %q — the page was NOT drawn with the face the document carries", label, want)
	}
	if strings.Contains(body, "+NotoSans-Regular") {
		t.Errorf("%s: the SHIPPED Latin Noto Sans reached the page, though it covers none of this document's Thai", label)
	}
	if !strings.Contains(body, "+NotoSansThai-Regular") {
		t.Errorf("%s: no /BaseFont names NotoSansThai-Regular — the embedded program is not the Thai face the document carries", label)
	}
	// Vacuity guard AND the bound this story must not lose. The three
	// assertions above are substring scans: a PDF that embedded no font at
	// all would satisfy the negative one for free, and a PDF that embedded
	// some THIRD, unrelated face alongside the carried one would satisfy all
	// three. The count is not the identity check — TRAP 1 is right that 1 is
	// the answer both before this story and after it, so it distinguishes
	// nothing on its own — but as a BOUND beside the identity it is what the
	// pre-8.4 guard supplied, and dropping it would let an extra subset face
	// onto the page unnoticed on every cross-target leg.
	if n := len(extractAllFontFile2Programs(t, raw)); n != 1 {
		t.Errorf("%s: the render embeds %d font programs, want exactly 1 (the carried Thai face and nothing else)", label, n)
	}
}

// firstBytes bounds a mismatch report: the fixture is ~64 KB, and dumping two
// copies of it into a test log describes nothing.
func firstBytes(s string) string {
	const limit = 400
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "\n… (truncated; the documents differ somewhere in " + fmt.Sprint(len(s)) + " bytes)"
}

// TestUnrecognisedFontMediaTypeIsValidToo is AC's positive control at the
// PUBLIC boundary: `Validate` must be SILENT over a document carrying a font
// asset whose media type this build does not recognise.
//
// The internal half of this is proved in internal/template
// (TestUnrecognisedFontMediaTypeLoadsClean). This is the half that matters to
// an integrator, and it is asserted rather than inferred, because `Validate`
// predicts `Render` (D-1.8.1 amended) and a `Validate` that refused what
// `Render` accepts — or accepted what `Render` refuses — would be a second
// rule system. It is the arm a "closed set of font media types" implementation
// fails.
//
// STORY 8.4 SPLIT IT IN TWO, because the document's text became Thai and the
// chain's unrecognised entry is now the only entry that could draw it. The
// rule D-1.8.1 as amended actually states has both halves in it, and only the
// second existed here before:
//
//	NOTHING DRAWS FROM IT  -> valid, silent, renders clean
//	SOMETHING MUST DRAW IT -> a located capability error, at Render and at
//	                          Validate alike
//
// A test asserting only the first half would pass on a build that never looked
// at the media type at all; one asserting only the second would pass on a
// build that refused every unrecognised type at LOAD, which is precisely what
// D-1.8.1 forbids.
func TestUnrecognisedFontMediaTypeIsValidToo(t *testing.T) {
	unrecognised := strings.Replace(embeddedFontTemplateJSON(), `"mediaType": "font/ttf"`, `"mediaType": "font/woff2"`, 1)
	if !strings.Contains(unrecognised, "font/woff2") {
		t.Fatal("fixture assumption violated: the media type was not substituted")
	}

	// HALF ONE: nothing draws from it. The Thai is replaced with Latin, which
	// the chain's FIRST entry covers completely, so coverage resolution never
	// reaches the carried entry and never asks what its bytes are.
	neverDrawn := strings.Replace(unrecognised, `"value": "สัญญา"`, `"value": "Latin only"`, 1)
	if !strings.Contains(neverDrawn, `"value": "Latin only"`) {
		t.Fatal("fixture assumption violated: the drawn text was not substituted")
	}
	diags, err := Validate([]byte(neverDrawn), Data(`{}`), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("a font asset with an unrecognised mediaType that nothing draws from must be VALID (D-1.8.1 amended), got: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("Validate must be SILENT over an unrecognised font media type, got %d diagnostic(s): %+v", len(diags), diags)
	}
	tpl, err := ParseTemplate([]byte(neverDrawn))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := Render(tpl, Data(`{}`), nil, testShippedFontSet()); err != nil {
		t.Fatalf("render: %v", err)
	}

	// HALF TWO: the fixture's own Thai, which only the carried entry could
	// draw. The document still LOADS — mediaType is an open set — and fails
	// at the moment the renderer is asked to draw with it.
	tpl, err = ParseTemplate([]byte(unrecognised))
	if err != nil {
		t.Fatalf("an unrecognised font media type must still LOAD (D-1.8.1 amended): %v", err)
	}
	_, rerr := Render(tpl, Data(`{}`), nil, testShippedFontSet())
	if rerr == nil {
		t.Fatal("a chain entry this build cannot read, that something must draw from, must fail at Render")
	}
	if !strings.Contains(rerr.Error(), `cannot render font media type "font/woff2"`) {
		t.Errorf("the render error does not name the capability limit: %v", rerr)
	}
	_, verr := Validate([]byte(unrecognised), Data(`{}`), nil, testShippedFontSet())
	if verr == nil || verr.Error() != rerr.Error() {
		t.Errorf("Validate must return the IDENTICAL error Render does:\n\tValidate: %v\n\tRender:   %v", verr, rerr)
	}
}

// TestTheFontRecordCostsAnExistingDocumentNothing is the story's last AC,
// stated as something that is actually true and actually measured.
//
// WHAT IS NOT ASSERTED HERE, AND WHY. "Re-serializing a committed fixture
// reproduces its bytes" is FALSE at HEAD and was false before this story:
// several committed `input.folio` files are hand-written for readability
// (compact bands, one-line style blocks) and are not canonical. Writing that
// assertion would have meant either changing those files — moving the very
// bytes this AC is about — or quietly restricting the population until it
// passed. Both are worse than saying what holds.
//
// WHAT IS ASSERTED. Over EVERY committed document: the serializer's output is
// a fixed point (so nothing this story added makes an existing document
// unstable), and the output carries NO `font` key for any document that did
// not declare one — an absent Presence stays absent and never reappears as an
// authored null or an empty object. The asset-bearing population is counted
// rather than named, so a fixture that gains or loses an assets map is read
// here rather than slipping past.
//
// The stronger statement — that no committed byte moved at all — is carried by
// the recorded golden PDF digests (goldenDigestRecord). Story 8.4 touched
// EXACTLY ONE fixture there, embedded-font's, deliberately and as the story
// that owns it; the other 22 are unmoved. Two artifacts of that one fixture
// moved in two different ways, and one word will not do for both:
// `expected.json`'s recorded sha256 genuinely CHANGED (db400698…e513ad became
// f533b04b…6d851832, the document's drawn text having gone from Latin to Thai),
// while `expected.pdf` is NEW — it ships for the first time, which is why the
// record holds 23 entries now and held 22 before.
func TestTheFontRecordCostsAnExistingDocumentNothing(t *testing.T) {
	dir := filepath.Join(repoRootFromTest(t), "fixtures")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read fixtures/: %v", err)
	}
	withAssets, checked := 0, 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name(), "input.folio")
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			continue // not every fixture ships an input.folio
		}
		checked++
		canonical := serializeTwice(t, entry.Name(), raw)
		if entry.Name() != "embedded-font" && strings.Contains(canonical, `"font"`) {
			t.Errorf("%s: a document that declares no font record must not acquire one on serialize", entry.Name())
		}
		if assetCount(t, raw) > 0 {
			withAssets++
		}
	}
	if checked == 0 {
		t.Fatal("vacuity guard: no fixture input.folio was read, so this test asserted nothing")
	}
	if withAssets != 7 {
		t.Errorf("fixtures carrying a non-empty assets map = %d, want 7 (the six that predate Story 8.3 plus embedded-font) — if a fixture gained or lost an asset map, say so here deliberately", withAssets)
	}
	t.Logf("byte-neutrality witness — %d committed documents serialized, %d of them carrying a non-empty assets map", checked, withAssets)
}

// serializeTwice returns a document's canonical bytes, having asserted they are
// a fixed point: Serialize(Parse(Serialize(Parse(b)))) == Serialize(Parse(b)).
func serializeTwice(t *testing.T, name string, raw []byte) string {
	t.Helper()
	once := serializeOnce(t, name, raw)
	twice := serializeOnce(t, name, []byte(once))
	if twice != once {
		t.Errorf("%s: the serializer is not a fixed point over this document", name)
	}
	return once
}

func serializeOnce(t *testing.T, name string, raw []byte) string {
	t.Helper()
	tpl, err := ParseTemplate(raw)
	if err != nil {
		t.Fatalf("%s: parse: %v", name, err)
	}
	out, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatalf("%s: serialize: %v", name, err)
	}
	return string(out)
}

// assetCount reads how many entries a committed document's `assets` map holds,
// off the JSON rather than off the parsed model, so the population this test
// reports is the one the FILE declares.
func assetCount(t *testing.T, raw []byte) int {
	t.Helper()
	var doc struct {
		Assets map[string]json.RawMessage `json:"assets"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal a committed document: %v", err)
	}
	return len(doc.Assets)
}
