package folio8

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio8-go/internal/expr"
	"github.com/panitw/folio8/folio8-go/internal/fontset"
	"github.com/panitw/folio8/folio8-go/internal/pdf"
	"github.com/panitw/folio8/folio8-go/internal/template"
)

// testFormatContext is the ordinary `en`/UTC formatting context this
// package's own tests thread through bind.BindTextSpans/Resolve
// wherever they call it directly rather than through the public
// Render/ParseTemplate path (Story 3.4's mandatory FormatContext
// parameter, R1).
func testFormatContext() expr.FormatContext {
	return expr.NewFormatContext(template.LocaleEN, "+00:00")
}

// subprocessEnvVar, when set to "1", makes TestMain render the FONTLESS
// document to stdout and exit instead of running the test suite. This is
// how the determinism test in this file re-executes the test binary as a
// fresh OS process rather than comparing two calls inside one process (a
// same-process comparison would pass on shared memoised state, which is
// exactly what AC8 rules out).
//
// This selector keeps rendering Story 1.1's fixed rectangle via
// internal/pdf.Serialize() directly, NOT via Render — Render now
// requires a non-nil template (AC14b), and fixtures/minimal-rect/ is
// reclassified (AC14a, D-1.5.9) as pinning internal/pdf's fontless
// emission path specifically. AC10a: "the subprocess seam KEEPS the
// fontless path" — so fixtures/minimal-rect/ does not lose its
// four-target verification.
const subprocessEnvVar = "FOLIO8_SUBPROCESS_RENDER"

// subprocessFontEnvVar is AC10a's SECOND selector: it renders the
// template+font document (fontTestTemplateJSON, testRobotoFontBytes)
// through the public Render path and writes the bytes to stdout. The
// cross-target matrix (matrix_test.go) drives both selectors and
// verifies AC10b's vacuity guard before comparing any hash: the child's
// output must actually contain a FontFile2, or the whole exercise
// certifies nothing (F-6).
const subprocessFontEnvVar = "FOLIO8_SUBPROCESS_RENDER_FONT"

// subprocessImageEnvVar is AC22's THIRD selector (D-1.8.6, joining the
// two above — it replaces neither): it renders imageTestTemplateJSON
// (an embedded, real, supported PNG asset drawn by one image element)
// through the public Render path and writes the bytes to stdout. The
// matrix harness (matrix_test.go) verifies AC23's vacuity guard — the
// child's output must actually contain an image XObject
// (/Subtype /Image) — before comparing any hash, the same shape AC10b
// already established for the font path.
const subprocessImageEnvVar = "FOLIO8_SUBPROCESS_RENDER_IMAGE"

// subprocessMultiScriptEnvVar is Story 2.2's FOURTH selector (AC8,
// joining the three above — it replaces none of them): it renders
// multiScriptTestTemplateJSON, a document whose ONE text element mixes
// Latin, Thai and CJK runes against a three-member fallback chain,
// through the public Render path, using the REAL shipped face set
// (`github.com/panitw/folio8/folio8-go/fonts`.Shipped()) rather than a
// synthetic FontSet — this is the fixture that actually exercises
// AC7's per-(face,pinned-instance) goldens across all four targets. The
// matrix harness verifies its OWN feature guard on every leg (AC8,
// V6): the captured stream must contain THREE distinct, INSTANCED
// (fvar-free) embedded programs, not merely "a FontFile2" — the same
// reasoning AC10b/AC23 already apply to the font and image paths.
const subprocessMultiScriptEnvVar = "FOLIO8_SUBPROCESS_RENDER_MULTISCRIPT"

// multiScriptTestTemplateJSON is Story 2.2's AC8 fixture document: one
// text element, one three-member fallback chain naming all three
// shipped faces in AD-8's declared order (Latin, Thai, CJK), and a
// value mixing all three scripts in a single run — "Ada" (Latin,
// resolves to Noto Sans), "ก" (Thai, resolves to Noto Sans Thai, since
// Noto Sans has no glyph for it) and "汉" (a Han ideograph, resolves to
// Noto Sans SC). This is D-1.8.6's precedent: a NEW document, added
// beside fontTestTemplateJSON/imageTestTemplateJSON, never replacing
// either (D-2.2-D5).
const multiScriptTestTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 500, "height": 20, "value": "Ada ก 汉", "style": {"fontFamily": "body", "fontSize": 14}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {
    "body": ["Noto Sans", "Noto Sans Thai", "Noto Sans SC"]
  },
  "locale": "en",
  "nextId": 2,
  "page": {
    "margin": {"bottom": 36, "left": 36, "right": 36, "top": 36},
    "orientation": "portrait",
    "size": "A4"
  },
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// subprocessShapedTextEnvVar is Story 2.3's FIFTH selector (joining the
// four above — it replaces none of them): it renders
// shapedTextTemplateJSON, the document whose text was chosen
// specifically so that a WRONG shaping answer is a different hash,
// through the public Render path against the real shipped face set.
//
// AC11 is why it exists at all. Every pre-2.3 golden was measured to be
// blind to shaping: its text shapes to itself, so a correct
// implementation and one that never calls the shaper produce identical
// bytes. A fifth fixture recorded over equally blind text would look
// like coverage in every report while being unable to fail when this
// story regresses — D-000.9's exact shape.
const subprocessShapedTextEnvVar = "FOLIO8_SUBPROCESS_RENDER_SHAPEDTEXT"

// subprocessWrappedTextEnvVar is Story 2.4's SIXTH selector (joining the
// five above, replacing none): fixtures/wrapped-text/, the line-breaking
// document.
//
// It needs its own selector for the same reason shaped-text did. Every
// earlier fixture FITS its box — measured, all ten text elements, the
// widest at 26% of its width — so a matrix comparing four legs of any of
// them would agree perfectly while certifying nothing about wrapping: a
// build that never broke a line would produce identical bytes. This is
// the only registered document whose elements are measured to overflow
// their boxes, and therefore the only one whose legs can disagree if
// line breaking is not reproducible across targets.
const subprocessWrappedTextEnvVar = "FOLIO8_SUBPROCESS_RENDER_WRAPPEDTEXT"

// subprocessThreeBandEnvVar is Story 2.5's SEVENTH selector (joining the
// six above, replacing none): fixtures/three-band-page/, the band
// composition document.
//
// It needs its own selector because no existing document can express a
// band-composition defect. Measured by injection at Story 2.5's
// creation: moving the PAGE HEADER band's origin was detected by ZERO
// tests in the repository, because all six recorded fixtures have an
// EMPTY pageHeader band — and all six share one page setup in which
// marginTop == marginBottom and pageHeader.height == pageFooter.height,
// so a SWAP of any pair among the four geometric inputs is invisible in
// the bytes. This is the only registered document with content in all
// three bands and four pairwise-distinct geometric inputs, and therefore
// the only one whose legs can disagree if band composition is not
// reproducible across targets.
const subprocessThreeBandEnvVar = "FOLIO8_SUBPROCESS_RENDER_THREEBAND"

// subprocessMultiPageEnvVar is Story 2.6's EIGHTH selector, rendering
// fixtures/multi-page/ through the public Render path in a FRESH process.
//
// It exists for the same reason the seven before it do: a golden recorded
// from one process pins whatever that process happened to do, including a map
// iteration order. The multi-page document is the first with more than one
// page, so it is the first whose PAGE OBJECT ORDER could vary between
// processes — which is exactly the property a second process can falsify and
// a second call in the same process cannot.
const subprocessMultiPageEnvVar = "FOLIO8_SUBPROCESS_RENDER_MULTIPAGE"

// shapedTextTemplateJSON is Story 2.3's AC10 fixture document
// (fixtures/shaped-text/input.folio, kept byte-identical to it by
// TestShapedTextGoldenFixture). Every element is chosen for what it
// makes observable:
//
//	e1  Thai tall consonants with marks — GSUB selects a LOWERED mark
//	    form; the naive answer collides with the ascender. THE defect.
//	e2  A real Thai name, plus two plain-Thai negative controls.
//	e3  "น้ำ า" — a merged SARA AM cluster AND a standalone SARA AA in
//	    ONE document. Both end in the same glyph and need different
//	    /ToUnicode answers, which is what forces CIDs to be allocated
//	    per (glyph, text) rather than per glyph.
//	e4  Latin ligatures (ffi, fi) and kern pairs (AV, Wo, To).
//	e5  CJK, the negative control: identical glyphs, all advances 1000.
//	e6  A mixed run, so shaping is exercised per FACE-SEGMENT (AC8) —
//	    the fallback chain still decides the face, shaping runs inside
//	    what it chose, never across a boundary.
//	e7  "AV ก" — a KERNING Latin segment followed by another face's
//	    segment. Added by Story 2.3's finisher (Blocker 1). e4 is
//	    shape-observable but is the LAST segment of its element, so its
//	    cursor is never consumed; e6 is a mixed run but "Ada " carries no
//	    kern, so it agreed by accident. Between them the golden was green
//	    over a live defect — text drawn kerned and placed unkerned. e7 is
//	    the case neither could produce: the Thai segment's origin moves by
//	    the A/V kern, so a regression to unkerned segment cursors is a
//	    different hash.
const shapedTextTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 500, "height": 24, "value": "ปั ฟั ที่ ป้ำ", "style": {"fontFamily": "body", "fontSize": 16}},
        {"id": "e2", "type": "text", "x": 0, "y": 28, "width": 500, "height": 24, "value": "ณัฐวุฒิ เกิด กรุงเทพ", "style": {"fontFamily": "body", "fontSize": 16}},
        {"id": "e3", "type": "text", "x": 0, "y": 56, "width": 500, "height": 24, "value": "น้ำ า", "style": {"fontFamily": "body", "fontSize": 16}},
        {"id": "e4", "type": "text", "x": 0, "y": 84, "width": 500, "height": 24, "value": "office fi AV Wo. To,", "style": {"fontFamily": "body", "fontSize": 16}},
        {"id": "e5", "type": "text", "x": 0, "y": 112, "width": 500, "height": 24, "value": "结算单，共３页", "style": {"fontFamily": "body", "fontSize": 16}},
        {"id": "e6", "type": "text", "x": 0, "y": 140, "width": 500, "height": 24, "value": "Ada ปั 结", "style": {"fontFamily": "body", "fontSize": 16}},
        {"id": "e7", "type": "text", "x": 0, "y": 168, "width": 500, "height": 24, "value": "AV ก", "style": {"fontFamily": "body", "fontSize": 16}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {
    "body": ["Noto Sans", "Noto Sans Thai", "Noto Sans SC"]
  },
  "locale": "th",
  "nextId": 8,
  "page": {
    "margin": {"bottom": 36, "left": 36, "right": 36, "top": 36},
    "orientation": "portrait",
    "size": "A4"
  },
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// imageTestAssetKey is the SHA-256 (lowercase hex) of imageTestPNGBytes
// below — generated by Go's image/png encoder and independently
// cross-checked with Python's hashlib/base64 during story creation.
const imageTestAssetKey = "5a05ad01e89c143b7061b0c93450566568d38a23da9b9c5c9dfe449016433078"

// imageTestTemplateJSON is a `.folio` document with one image element
// in content, referencing a real, supported, non-square 3x2 8-bit RGB
// PNG asset (AC27a: non-square, so it exercises AC13's binding-axis
// choice) — its data is the canonical 76-column base64 split of
// imageTestPNGBytes.
const imageTestTemplateJSON = `{
  "assets": {
    "` + imageTestAssetKey + `": {
      "data": [
        "iVBORw0KGgoAAAANSUhEUgAAAAMAAAACCAIAAAASFvFNAAAAGElEQVR42mL6z8DAAMZMEOo/AwMg",
        "AAD//zwUBf/NjsW5AAAAAElFTkSuQmCC"
      ],
      "mediaType": "image/png"
    }
  },
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "image", "asset": "` + imageTestAssetKey + `", "x": 0, "y": 0, "width": 100, "height": 60}
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
  "fonts": {},
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
  "version": "1.0"
}
`

// componentAssetImportKey is the SHA-256 (lowercase hex) of the source
// image bytes setComponentAsset installs below — the same picture
// component_asset_command_test.go's png1x1Gray() decodes (Go's own
// internal/template/fixtures_test.go fixture: a real, valid 1x1 grayscale
// PNG).
const componentAssetImportKey = "541581d3ab4d47c46ce5bcfbe86f9e9369f425b41df11decff572d259fa22c65"

// componentAssetImportSecondAssetKey is the SHA-256 (lowercase hex) of
// jpeg4x4Bytes() (render_image_test.go's own real, decodable 4x4 JPEG
// fixture) — the SECOND asset in componentAssetImportBaseTemplateJSON
// below, on element e2. e2 is never touched by the command under test, so
// this key survives it untouched, alongside componentAssetImportKey
// (Finding 7, review of 2026-08-29: this second, DIFFERENT asset is what
// makes the fixture's sorted-key claim genuinely falsifiable — see that
// constant's doc comment).
const componentAssetImportSecondAssetKey = "a3beda078fd65550fb477583f62a56b17fcb89a881b22606c1790cede7f9640a"

// componentAssetImportBaseTemplateJSON is the STARTING document
// TestComponentAssetImportCommandReproducesTheFixtureInput applies the
// real setComponentAsset command to (fixture_test.go). Unlike
// imageTestTemplateJSON (one image, one asset) it carries TWO image
// elements/assets: e1 references imageTestAssetKey's 3x2 RGB PNG (the
// same starting point image-embed/ and the pre-Finding-7 version of this
// fixture both used) and e2 references componentAssetImportSecondAssetKey's
// JPEG. The command below targets e1 ONLY — e2's asset is never
// referenced by the mutation, so D-5.13.3's scoped orphan-collection must
// leave it alone while e1's OLD asset (imageTestAssetKey, referenced by
// nothing else) is correctly collected. The result therefore carries TWO
// surviving assets under two DIFFERENT keys, which is what lets the
// golden's sorted-key claim actually be falsified: with only one
// surviving asset (the shape before this fix), reversing the sort order
// in serialize.go has nothing to reorder and the golden stays green.
const componentAssetImportBaseTemplateJSON = `{
  "assets": {
    "` + imageTestAssetKey + `": {
      "data": [
        "iVBORw0KGgoAAAANSUhEUgAAAAMAAAACCAIAAAASFvFNAAAAGElEQVR42mL6z8DAAMZMEOo/AwMg",
        "AAD//zwUBf/NjsW5AAAAAElFTkSuQmCC"
      ],
      "mediaType": "image/png"
    },
    "` + componentAssetImportSecondAssetKey + `": {
      "data": ["/9j/2wCEAAMCAgMCAgMDAwMEAwMEBQgFBQQEBQoHBwYIDAoMDAsKCwsNDhIQDQ4RDgsLEBYQERMUFRUVDA8XGBYUGBIUFRQBAwQEBQQFCQUFCRQNCw0UFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFP/AABEIAAQABAMBIgACEQEDEQH/xAGiAAABBQEBAQEBAQAAAAAAAAAAAQIDBAUGBwgJCgsQAAIBAwMCBAMFBQQEAAABfQECAwAEEQUSITFBBhNRYQcicRQygZGhCCNCscEVUtHwJDNicoIJChYXGBkaJSYnKCkqNDU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6g4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2drh4uPk5ebn6Onq8fLz9PX29/j5+gEAAwEBAQEBAQEBAQAAAAAAAAECAwQFBgcICQoLEQACAQIEBAMEBwUEBAABAncAAQIDEQQFITEGEkFRB2FxEyIygQgUQpGhscEJIzNS8BVictEKFiQ04SXxFxgZGiYnKCkqNTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqCg4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2dri4+Tl5ufo6ery8/T19vf4+fr/2gAMAwEAAhEDEQA/ALHw9/Z+8Ff8Iva/8Sz9R6D2rpP+GfvBX/QM/Uf4V0fw9/5Fe1/z2FdJX5lis0x3t5/vpbvqzuyHOsy/srDf7RP4I/afY//Z"],
      "mediaType": "image/jpeg"
    }
  },
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "image", "asset": "` + imageTestAssetKey + `", "x": 0, "y": 0, "width": 100, "height": 60},
        {"id": "e2", "type": "image", "asset": "` + componentAssetImportSecondAssetKey + `", "x": 0, "y": 70, "width": 40, "height": 40}
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
  "fonts": {},
  "locale": "en",
  "nextId": 3,
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
  "version": "1.0"
}
`

// componentAssetImportTemplateJSON is fixtures/component-asset-import's
// input.folio, captured verbatim (AC in Story 5.13's Task 5): the CANONICAL
// output of one real setComponentAsset command (AD-9's digest-as-key, 76-col
// wrap, sorted keys, insert-if-absent, repoint-with-orphan-collection) run
// against componentAssetImportBaseTemplateJSON above, replacing element e1's
// asset with png1x1Gray(). It is not hand-authored to resemble that output —
// TestComponentAssetImportCommandReproducesTheFixtureInput (fixture_test.go)
// re-runs the command and asserts byte-identity against this constant on
// every ordinary `go test ./...`, so this string can never silently drift
// from what the command actually produces. It carries TWO assets — e1's new
// one (componentAssetImportKey) and e2's untouched one
// (componentAssetImportSecondAssetKey), never repointed or collected by
// this command — which is what makes the sorted-key ordering below (5 < a)
// an actual, falsifiable claim rather than a single-entry map with nothing
// to order (Finding 7, review of 2026-08-29).
const componentAssetImportTemplateJSON = `{
  "assets": {
    "541581d3ab4d47c46ce5bcfbe86f9e9369f425b41df11decff572d259fa22c65": {
      "data": [
        "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAAAAAA6fptVAAAADklEQVR4nGJqAAQAAP//AIYAg0ye",
        "IEsAAAAASUVORK5CYII="
      ],
      "mediaType": "image/png"
    },
    "a3beda078fd65550fb477583f62a56b17fcb89a881b22606c1790cede7f9640a": {
      "data": [
        "/9j/2wCEAAMCAgMCAgMDAwMEAwMEBQgFBQQEBQoHBwYIDAoMDAsKCwsNDhIQDQ4RDgsLEBYQERMU",
        "FRUVDA8XGBYUGBIUFRQBAwQEBQQFCQUFCRQNCw0UFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQU",
        "FBQUFBQUFBQUFBQUFBQUFBQUFBQUFP/AABEIAAQABAMBIgACEQEDEQH/xAGiAAABBQEBAQEBAQAA",
        "AAAAAAAAAQIDBAUGBwgJCgsQAAIBAwMCBAMFBQQEAAABfQECAwAEEQUSITFBBhNRYQcicRQygZGh",
        "CCNCscEVUtHwJDNicoIJChYXGBkaJSYnKCkqNDU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hp",
        "anN0dXZ3eHl6g4SFhoeIiYqSk5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV",
        "1tfY2drh4uPk5ebn6Onq8fLz9PX29/j5+gEAAwEBAQEBAQEBAQAAAAAAAAECAwQFBgcICQoLEQAC",
        "AQIEBAMEBwUEBAABAncAAQIDEQQFITEGEkFRB2FxEyIygQgUQpGhscEJIzNS8BVictEKFiQ04SXx",
        "FxgZGiYnKCkqNTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqCg4SFhoeIiYqS",
        "k5SVlpeYmZqio6Slpqeoqaqys7S1tre4ubrCw8TFxsfIycrS09TV1tfY2dri4+Tl5ufo6ery8/T1",
        "9vf4+fr/2gAMAwEAAhEDEQA/ALHw9/Z+8Ff8Iva/8Sz9R6D2rpP+GfvBX/QM/Uf4V0fw9/5Fe1/z",
        "2FdJX5lis0x3t5/vpbvqzuyHOsy/srDf7RP4I/afY//Z"
      ],
      "mediaType": "image/jpeg"
    }
  },
  "bands": {
    "content": {
      "elements": [
        {
          "asset": "541581d3ab4d47c46ce5bcfbe86f9e9369f425b41df11decff572d259fa22c65",
          "height": 60,
          "id": "e1",
          "type": "image",
          "width": 100,
          "x": 0,
          "y": 0
        },
        {
          "asset": "a3beda078fd65550fb477583f62a56b17fcb89a881b22606c1790cede7f9640a",
          "height": 40,
          "id": "e2",
          "type": "image",
          "width": 40,
          "x": 0,
          "y": 70
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
  "fonts": {},
  "locale": "en",
  "nextId": 3,
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
  "version": "1.0"
}
`

// subprocessComponentAssetImportEnvVar is Story 5.13's SIXTEENTH selector
// (joining the fifteen above — it replaces none of them): it renders
// componentAssetImportTemplateJSON — fixtures/component-asset-import's
// input.folio, the captured canonical output of one real setComponentAsset
// command, not a hand-authored document — through the public Render path
// and writes the bytes to stdout. Unlike subprocessImageEnvVar's
// image-embed fixture, this selector's fixture pins the AUTHORING
// COMMAND's output (AD-9's digest-as-key insertion, 76-column wrap, sorted
// keys, repoint-with-orphan-collection), not merely a document that already
// names an asset.
const subprocessComponentAssetImportEnvVar = "FOLIO8_SUBPROCESS_RENDER_COMPONENTASSETIMPORT"

// fontTestTemplateJSON is a `.folio` document with one text element in
// each of pageHeader, content and pageFooter, all resolving to the same
// face via one fallback chain ("body" -> ["Roboto-Regular"]) — enough to
// exercise AC9's "union of glyphs the whole document uses, collected
// once" across more than one element and more than one band.
const fontTestTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 400, "height": 20, "value": "Hello, World!", "style": {"fontFamily": "body", "fontSize": 14}}
      ]
    },
    "pageFooter": {
      "elements": [
        {"id": "e2", "type": "text", "x": 0, "y": 0, "width": 400, "height": 16, "value": "Page footer 0123456789", "style": {"fontFamily": "body", "fontSize": 9}}
      ],
      "height": 20
    },
    "pageHeader": {
      "elements": [],
      "height": 20
    }
  },
  "fonts": {
    "body": ["Roboto-Regular"]
  },
  "locale": "en",
  "nextId": 3,
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
  "version": "1.0"
}
`

// testFontSet returns a FontSet carrying Story 1.5's one small Latin
// test face (AC26), keyed to match fontTestTemplateJSON's fonts chain.
func testFontSet() FontSet {
	return FontSet{"Roboto-Regular": testRobotoFontBytes}
}

// parseFontTestTemplate parses fontTestTemplateJSON, failing the test on
// any parse error.
func parseFontTestTemplate(t *testing.T) *Template {
	t.Helper()
	tpl, err := ParseTemplate([]byte(fontTestTemplateJSON))
	if err != nil {
		t.Fatalf("parse fontTestTemplateJSON: %v", err)
	}
	return tpl
}

// TestRenderSubsetsOneFacePerDocumentNotPerElement is Finding 6's fix
// (QA review): AC9's "one subsetting call" half was previously asserted
// only "via code review" in a synthetic textdoc_test.go fixture that
// never reached renderDocument. This test exercises the REAL public
// path — Render, on fontTestTemplateJSON, whose content and pageFooter
// bands each carry a text element referencing the SAME face ("body" ->
// "Roboto-Regular") — and asserts the property behaviourally two ways:
//
//  1. Exactly one /FontFile2 stream in the output, even though two
//     elements in two different bands reference the face.
//  2. The embedded subset's tag is the DOCUMENT-WIDE union closure's
//     tag, not either element's own smaller, per-element closure — a
//     regression that moved subsetting inside the per-run loop (the
//     exact hazard AC9 names) would still embed exactly one FontFile2
//     per RUN, so check 1 alone would not catch it; the tag comparison
//     does, because a per-element subset's closure (and therefore its
//     tag) differs from the union's.
func TestRenderSubsetsOneFacePerDocumentNotPerElement(t *testing.T) {
	tpl := parseFontTestTemplate(t)
	fs := testFontSet()

	outRes, err := Render(tpl, Data("{}"), nil, fs)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out := outRes.Bytes
	if !containsFontFile2(out) {
		t.Fatal("rendered document has no /FontFile2 — vacuity guard failed before this test's real assertions could mean anything (AC10b)")
	}

	n := bytes.Count(out, []byte("/FontFile2 "))
	if n != 1 {
		t.Fatalf("AC9: expected exactly one /FontFile2 reference for one face used by two elements in two bands, got %d", n)
	}

	font, ferr := fontset.New("Roboto-Regular", testRobotoFontBytes)
	if ferr != nil {
		t.Fatalf("fontset.New: %v", ferr)
	}
	// Story 2.3, AC5: the subset is keyed on the glyphs the renderer
	// DRAWS, so the comparison sets are built by shaping each element's
	// text, exactly as renderDocument does.
	elem1Sub, err := font.Subset(shapedGlyphIDsForTest(t, font, "Hello, World!"))
	if err != nil {
		t.Fatalf("Subset(elem1 alone): %v", err)
	}
	elem2Sub, err := font.Subset(shapedGlyphIDsForTest(t, font, "Page footer 0123456789"))
	if err != nil {
		t.Fatalf("Subset(elem2 alone): %v", err)
	}
	unionSub, err := font.Subset(shapedGlyphIDsForTest(t, font, "Hello, World!Page footer 0123456789"))
	if err != nil {
		t.Fatalf("Subset(union): %v", err)
	}
	if elem1Sub.Tag == unionSub.Tag || elem2Sub.Tag == unionSub.Tag {
		t.Fatalf(
			"test fixture assumption violated: a per-element subset's tag must differ from the union's for this "+
				"assertion to be discriminating (elem1=%q elem2=%q union=%q)",
			elem1Sub.Tag, elem2Sub.Tag, unionSub.Tag,
		)
	}

	baseFontUnion := "/BaseFont /" + unionSub.Tag + "+"
	if !bytes.Contains(out, []byte(baseFontUnion)) {
		t.Fatalf("rendered document's embedded /BaseFont does not carry the document-wide union tag %q — AC9's "+
			"'one subsetting call per document, over the union of glyphs the whole document uses' is violated", unionSub.Tag)
	}
	for _, perElement := range []struct {
		label string
		sub   *fontset.Subset
	}{{"elem1", elem1Sub}, {"elem2", elem2Sub}} {
		needle := "/BaseFont /" + perElement.sub.Tag + "+"
		if bytes.Contains(out, []byte(needle)) {
			t.Fatalf("rendered document's embedded /BaseFont carries %s's OWN per-element tag %q instead of the "+
				"document-wide union tag — subsetting is happening per element, not once per document (AC9)",
				perElement.label, perElement.sub.Tag)
		}
	}
}

// shapedGlyphIDsForTest returns the source glyph ids font draws for
// text, deduplicated by first occurrence — the input shape Subset takes
// after Story 2.3 re-keyed it off runes (AC5).
func shapedGlyphIDsForTest(t *testing.T, font *fontset.Font, text string) []uint16 {
	t.Helper()
	glyphs, err := font.Shaper().Shape(text)
	if err != nil {
		t.Fatalf("Shape(%q): %v", text, err)
	}
	if len(glyphs) == 0 {
		t.Fatalf("Shape(%q) produced no glyphs", text)
	}
	seen := map[uint16]bool{}
	var out []uint16
	for _, g := range glyphs {
		if seen[g.GlyphID] {
			continue
		}
		seen[g.GlyphID] = true
		out = append(out, g.GlyphID)
	}
	return out
}

// containsFontFile2 is AC10b's binding vacuity guard, applied wherever
// this file or matrix_test.go needs to confirm a rendered document
// actually embeds a font before comparing any bytes: "the child render
// on the font path must be asserted to actually contain a FontFile2
// before any byte comparison runs... same output ⇒ instance" (D-000.9).
func containsFontFile2(b []byte) bool {
	return bytes.Contains(b, []byte("/FontFile2"))
}

// containsImageXObject is AC23's binding vacuity guard (D-1.8.6), the
// image-path mirror of containsFontFile2 above: a rendered document
// must be confirmed to actually contain an image XObject
// (/Subtype /Image) before any byte comparison runs.
func containsImageXObject(b []byte) bool {
	return bytes.Contains(b, []byte("/Subtype /Image"))
}

// toolchainEnvVar, when set to "1", makes TestMain write the toolchain
// that built this binary to stdout and exit, instead of running the test
// suite. Story 1.2's matrix harness (AC3, D-1.2.4) uses this to witness
// each target binary's toolchain before comparing any hash: the host's
// `go version` describes the machine, not the artifact, and in CI the
// four legs are built on three different runners — only
// runtime.Version() compiled into the binary itself is an honest
// witness. Same seam as subprocessEnvVar, one more env-gated mode, both
// exiting before m.Run() is ever reached so neither touches os/exec or
// pipes — which is what keeps this branch wasm-safe.
const toolchainEnvVar = "FOLIO8_SUBPROCESS_TOOLCHAIN"

func TestMain(m *testing.M) {
	if os.Getenv(toolchainEnvVar) == "1" {
		os.Stdout.WriteString(runtime.Version())
		os.Exit(0)
	}
	if os.Getenv(subprocessEnvVar) == "1" {
		writeToStdoutOrDie(pdf.Serialize())
	}
	if os.Getenv(subprocessFontEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(fontTestTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data("{}"), nil, testFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessImageEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(imageTestTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data("{}"), nil, nil)
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessComponentAssetImportEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(componentAssetImportTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data("{}"), nil, nil)
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessShapedTextEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(shapedTextTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data("{}"), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessWrappedTextEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(wrappedTextTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data(wrappedTextDataJSON), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessThreeBandEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(threeBandPageTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data("{}"), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessMultiPageEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(multiPageTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data("{}"), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessMultiScriptEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(multiScriptTestTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data("{}"), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessPageCount20EnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(pageCount20TemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data("{}"), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessHiddenImageEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(hiddenImageTestTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data(`{"flag": false}`), nil, testFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessAlternatingRowsEnvVar) == "1" {
		b, err := renderAlternatingRowsFixture()
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(b)
	}
	if os.Getenv(subprocessMandatoryBreakEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(mandatoryBreakTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data(mandatoryBreakDataJSON), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessLineSpacingEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(lineSpacingTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data(lineSpacingDataJSON), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessJustifiedTextEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(justifiedTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data(justifiedDataJSON), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessJustifiedThaiEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(justifiedThaiTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data(justifiedThaiDataJSON), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessAlignmentRoundingEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(alignmentRoundingTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data(alignmentRoundingDataJSON), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessThaiStackedMarksEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(thaiStackedMarksTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data(thaiStackedMarksDataJSON), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessColourStrokesEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(colourStrokesTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data(colourStrokesDataJSON), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessMultiPageFlowEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(multiPageFlowTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data(multiPageFlowDataJSON), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessMultiPageStatementEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(multiPageStatementTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data(multiPageStatementDataJSON), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessSectionBreakUnanchoredEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(sectionBreakUnanchoredTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data(sectionBreakUnanchoredDataJSON), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessSectionBreakStatementEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(sectionBreakStatementTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data(sectionBreakStatementDataJSON), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessBarcodeThaiBillPaymentEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(barcodeThaiBillPaymentTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data(barcodeThaiBillPaymentDataJSON), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessQRCodePaymentsEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(qrcodePaymentsTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data(qrcodePaymentsDataJSON), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessDeclaredVariantsEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(declaredVariantsTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		// Data("{}") — the document binds nothing, and Render refuses a
		// nil report body outright rather than reading it as "no data".
		// Spelled the way this document's two other call sites spell it
		// (declared_variants_fixture_test.go, missing_glyph_corpus_test.go),
		// so all three read as one idiom.
		res, err := Render(tpl, Data("{}"), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessEmbeddedFontEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(embeddedFontTemplateJSON()))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data(`{}`), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	if os.Getenv(subprocessKeepTogetherEnvVar) == "1" {
		tpl, err := ParseTemplate([]byte(keepTogetherTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data(keepTogetherDataJSON), nil, testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	// Story 4.7's ELEVENTH..FOURTEENTH selectors: the Customer Account
	// Statement at 1, 5, 20 and 50 pages. Four selectors rather than one
	// parameterised selector because matrixDocuments' capture contract is
	// one env var per document, and because js/wasm has no argv the Docker
	// and wasm runners could carry a page count in.
	for _, sel := range statementSubprocessSelectors {
		if os.Getenv(sel.env) != "1" {
			continue
		}
		tpl, err := ParseTemplate([]byte(statementTemplateJSON))
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		res, err := Render(tpl, Data(statementDataJSON(sel.rows)), Params(statementParamsJSON), testShippedFontSet())
		if err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		writeToStdoutOrDie(res.Bytes)
	}
	os.Exit(m.Run())
}

// subprocessMandatoryBreakEnvVar is Story 7.1's selector, rendering
// fixtures/mandatory-break/ — the first document in this repository
// whose text or bound data carries a line feed — in a FRESH process,
// for the same reason multi-page and page-count-20 needed one: a golden
// recorded from one process pins whatever that process happened to do.
const subprocessMandatoryBreakEnvVar = "FOLIO8_SUBPROCESS_RENDER_MANDATORYBREAK"

// subprocessLineSpacingEnvVar is Story 7.2's selector, rendering
// fixtures/line-spacing/ — the first document in this repository that
// declares a line spacing at all, and the first declaring format
// version 1.1 — in a FRESH process, for the same reason mandatory-break
// needed one: a golden recorded from one process pins whatever that
// process happened to do.
const subprocessLineSpacingEnvVar = "FOLIO8_SUBPROCESS_RENDER_LINESPACING"

// subprocessJustifiedTextEnvVar is Story 7.3's selector, rendering
// fixtures/justified-text/ — the first document in this repository that
// is justified at all, and the first declaring format version 2.0 — in a
// FRESH process, for the same reason line-spacing needed one: a golden
// recorded from one process pins whatever that process happened to do.
const subprocessJustifiedTextEnvVar = "FOLIO8_SUBPROCESS_RENDER_JUSTIFIEDTEXT"

// subprocessJustifiedThaiEnvVar is Story 7.3's Thai-coverage selector,
// rendering fixtures/justified-thai/ — the first document in this
// repository whose justified content has no SPACES in it, so that every
// gap its slack is distributed into is a seam the shipped dictionary
// proposed (AD-25) rather than a run of whitespace the author typed —
// in a FRESH process, for the same reason justified-text needed one.
const subprocessJustifiedThaiEnvVar = "FOLIO8_SUBPROCESS_RENDER_JUSTIFIEDTHAI"

// subprocessAlignmentRoundingEnvVar is Story 7.3's second selector,
// rendering fixtures/alignment-rounding/ — DW-24's closing document, the
// first that declares `align: "center"` or `valign` at all and therefore
// the first that reaches any branch halving a slack with
// geom.ScaleRound. It is the one document in the corpus whose whole
// reason to exist is a rounding rule, which is why running it on every
// target is not a formality.
const subprocessAlignmentRoundingEnvVar = "FOLIO8_SUBPROCESS_RENDER_ALIGNMENTROUNDING"

// subprocessKeepTogetherEnvVar is Story 7.7's selector, rendering
// fixtures/keep-together/ — the first document in this repository whose
// column is broken by an AUTHOR'S OWN DECLARATION (FR51) rather than by
// the four pagination rules alone, and the first declaring format
// version 1.2 — in a FRESH process, for the same reason
// alignment-rounding needed one: a golden recorded from one process pins
// whatever that process happened to do. It matters here specifically
// because this story changes what the paginator is GIVEN, and a page
// assignment is the one quantity a whole document's bytes hang off.
const subprocessKeepTogetherEnvVar = "FOLIO8_SUBPROCESS_RENDER_KEEPTOGETHER"

// subprocessThaiStackedMarksEnvVar is Story 8.0's selector, rendering
// fixtures/thai-stacked-marks/ — the first document in this repository
// carrying a glyph the shaper gives a non-zero YOffset, and therefore
// the first whose content stream holds a text-rise operator at all — in
// a FRESH process, for the same reason keep-together needed one: a
// golden recorded from one process pins whatever that process happened
// to do. It matters here specifically because the rise operand is
// derived by geom.ScaleRound from the run's font size and spelled out by
// appendLength, and both halves of that are what AD-21's four targets
// exist to hold to one answer.
const subprocessThaiStackedMarksEnvVar = "FOLIO8_SUBPROCESS_RENDER_THAISTACKEDMARKS"

// subprocessEmbeddedFontEnvVar is Story 8.3's selector, rendering
// fixtures/embedded-font/ — THE FIRST DOCUMENT IN THIS REPOSITORY THAT CARRIES
// A FONT FACE — in a FRESH process, for the same reason keep-together and
// thai-stacked-marks needed one: a golden recorded from one process pins
// whatever that process happened to do.
//
// STORY 8.3 REGISTERED IT FOR A NEGATIVE REASON — the carried face reached the
// loader on every target and the page on none — and Story 8.4 inverted that.
// The document's text is pure Thai now, the shipped Latin face the chain names
// first covers none of it, and what the four legs certify is that a font
// program decoded out of the document's own base64, subset and embedded,
// produces identical bytes on all four targets. The fixture's extraGuard
// asserts WHICH face reached the page, by identity: a target that drew with
// the shipped face, or with none, fails its own leg rather than merely
// diverging from the other three.
const subprocessEmbeddedFontEnvVar = "FOLIO8_SUBPROCESS_RENDER_EMBEDDEDFONT"

// subprocessDeclaredVariantsEnvVar is Story 11.5's selector, rendering
// fixtures/declared-variants/ — THE FIRST DOCUMENT IN THIS REPOSITORY
// THAT DECLARES BOLD OR ITALIC AT ALL, and the first whose chain entry
// declares its three cuts in the object form Story 11.2 introduced — in
// a FRESH process, for the same reason embedded-font and
// thai-stacked-marks needed one: a golden recorded from one process
// pins whatever that process happened to do.
//
// EVERY LEG RENDERS FROM THE COMMITTED TEMPLATE CONST, IN A FRESH
// PROCESS, AND THAT IS THE INVARIANT. The four legs render
// declaredVariantsTemplateJSON — the same const
// fixtures/declared-variants/input.folio is kept byte-identical to — so
// no leg can certify a document the repository does not carry. There IS
// an in-process render of that same const (renderDeclaredVariants, in
// declared_variants_fixture_test.go), and the untagged golden test
// hashes its bytes; what the matrix adds is the FRESH PROCESS on four
// targets, because a golden recorded from one process pins whatever that
// process happened to do.
//
// It matters here specifically because face resolution is where the
// four targets could quietly disagree: chainFaceNames maps an entry to
// a base name and a styled name at ONE boundary, and a target that read
// the variant differently — or constructed one — would embed a
// different font program, subset differently and hash differently, with
// nothing in the corpus before this document able to say so.
const subprocessDeclaredVariantsEnvVar = "FOLIO8_SUBPROCESS_RENDER_DECLAREDVARIANTS"

// subprocessBarcodeThaiBillPaymentEnvVar renders
// fixtures/barcode-thai-bill-payment/ — the first document carrying a barcode —
// in a fresh process, from the committed template const.
const subprocessBarcodeThaiBillPaymentEnvVar = "FOLIO8_SUBPROCESS_RENDER_BARCODETHAIBILLPAYMENT"

// subprocessSectionBreakStatementEnvVar renders
// fixtures/section-break-statement/ — the first document carrying a section
// break — in a fresh process, from the committed template const.
const subprocessSectionBreakStatementEnvVar = "FOLIO8_SUBPROCESS_RENDER_SECTIONBREAKSTATEMENT"

// subprocessSectionBreakUnanchoredEnvVar renders
// fixtures/section-break-unanchored/ — the first document carrying an
// unanchored section break — in a fresh process, from the committed
// template const.
const subprocessSectionBreakUnanchoredEnvVar = "FOLIO8_SUBPROCESS_RENDER_SECTIONBREAKUNANCHORED"

// subprocessMultiPageStatementEnvVar renders fixtures/multi-page-statement/ —
// the first document written in the multi-page `pages` shape — in a fresh
// process, from the committed template const.
const subprocessMultiPageStatementEnvVar = "FOLIO8_SUBPROCESS_RENDER_MULTIPAGESTATEMENT"

// subprocessMultiPageFlowEnvVar renders fixtures/multi-page-flow/ — the first
// document with a section break on a later page and Page Break off — in a
// fresh process, from the committed template const.
const subprocessMultiPageFlowEnvVar = "FOLIO8_SUBPROCESS_RENDER_MULTIPAGEFLOW"

// subprocessColourStrokesEnvVar renders fixtures/colour-strokes/ — the first
// document declaring text colour and coloured strokes (DW-147) — in a fresh
// process, from the committed template const.
const subprocessColourStrokesEnvVar = "FOLIO8_SUBPROCESS_RENDER_COLOURSTROKES"

// subprocessQRCodePaymentsEnvVar renders fixtures/qrcode-payments/ — the
// first document carrying a qrcode — in a fresh process, from the committed
// template const.
const subprocessQRCodePaymentsEnvVar = "FOLIO8_SUBPROCESS_RENDER_QRCODEPAYMENTS"

// subprocessPageCount20EnvVar is Story 2.7's NINTH selector, rendering
// fixtures/page-count-20/ — the {{page}}/{{pages}} matrix document — in
// a FRESH process, for the same reason multi-page needed one: a golden
// recorded from one process pins whatever that process happened to do.
// 20 pages is the smallest of this story's four page counts that spans
// the page-9-to-page-10 digit boundary (finding 8, story creation), so
// it is the one registered for cross-target comparison; the other three
// (1, 5, 50 pages) are verified behaviourally in the ordinary suite
// (page_count_matrix_test.go) without a separate matrix leg.
const subprocessPageCount20EnvVar = "FOLIO8_SUBPROCESS_RENDER_PAGECOUNT20"

// subprocessHiddenImageEnvVar is Story 3.5's TENTH selector, rendering
// hiddenImageTestTemplateJSON in a FRESH process. Story 3.5 finisher
// review, Finding 1 (Blocker): a hidden image element's `/XObject`
// stayed embedded in the PDF even though pagemodel.Page.Images
// correctly showed zero images (AC1/AC2) — assetKeys/pdfImages were
// built from every image run BEFORE visibility was consulted. AD-21
// binds every feature to ship its cross-target golden, and this is the
// first document able to falsify the fix in the direction that matters:
// its rendered output must NOT contain an image XObject on any target,
// not merely on the host this story was fixed and measured on
// (matrix_test.go's requireNoImageXObject is this document's
// extraGuard). A fresh process, for the same reason multi-page and
// page-count-20 needed one: a golden recorded from one process pins
// whatever that process happened to do.
const subprocessHiddenImageEnvVar = "FOLIO8_SUBPROCESS_RENDER_HIDDENIMAGE"

// hiddenImageTestTemplateJSON reuses imageTestTemplateJSON's own
// non-square 3x2 PNG asset (imageTestAssetKey) on an image element
// carrying `visibleIf`, plus an always-visible text sibling on the same
// face fontTestTemplateJSON already exercises ("body" -> Roboto-
// Regular) — so the rendered output has real, observable content
// (a FontFile2) alongside the asset that must NOT appear.
const hiddenImageTestTemplateJSON = `{
  "assets": {
    "` + imageTestAssetKey + `": {
      "data": [
        "iVBORw0KGgoAAAANSUhEUgAAAAMAAAACCAIAAAASFvFNAAAAGElEQVR42mL6z8DAAMZMEOo/AwMg",
        "AAD//zwUBf/NjsW5AAAAAElFTkSuQmCC"
      ],
      "mediaType": "image/png"
    }
  },
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "image", "asset": "` + imageTestAssetKey + `", "x": 0, "y": 0, "width": 100, "height": 60, "visibleIf": "flag"},
        {"id": "e2", "type": "text", "x": 0, "y": 100, "width": 200, "height": 20, "value": "Always visible", "style": {"fontFamily": "body", "fontSize": 12}}
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
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 4,
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
  "version": "1.0"
}
`

// writeToStdoutOrDie writes b to stdout in full and exits 0, or reports a
// short/failed write and exits 1. A short write here would otherwise be
// indistinguishable from a real determinism failure: the parent process
// would see truncated bytes and report a byte-offset divergence that
// points at the renderer, when the actual fault is this pipe (this
// story's QA review, Nit 25).
func writeToStdoutOrDie(b []byte) {
	n, werr := os.Stdout.Write(b)
	if werr != nil {
		os.Stderr.WriteString("write to stdout: " + werr.Error())
		os.Exit(1)
	}
	if n != len(b) {
		os.Stderr.WriteString("short write to stdout: wrote " + strconv.Itoa(n) + " of " + strconv.Itoa(len(b)) + " bytes")
		os.Exit(1)
	}
	os.Exit(0)
}

// assertWellFormedPDF performs toolchain-independent structural
// validation of the classic (non-stream) PDF this story's document is:
// it re-derives the document's self-described structure from its own
// bytes — no PDF-reading dependency, consistent with AC2 — and checks
// internal consistency, rather than checking for a handful of literal
// substrings.
//
// This story's QA review measured that the previous version (four
// checks: non-empty, header prefix, %%EOF suffix, exactly one
// "/Type /Page " substring) accepted a 118-byte stub with no catalog, no
// page tree, no content stream, no xref table and no trailer — every test
// that used it, including AC5's own validity test, passed on that stub
// (Major 4). It is also this function that AC8's vacuity guard 1 depends
// on to keep two identical failures from comparing equal, and — with
// Major 4's fix — it now doubles as Minor 18's PDF self-consistency
// assertions and Nit 26's fix for a page-count check that used to depend
// on an incidental trailing space rather than an actual dictionary
// boundary.
func assertWellFormedPDF(t *testing.T, label string, b []byte, wantPages int) {
	t.Helper()

	if len(b) == 0 {
		t.Fatalf("%s: output is empty", label)
	}
	if !bytes.HasPrefix(b, []byte("%PDF-1.7")) {
		t.Fatalf("%s: output does not start with %%PDF-1.7", label)
	}
	if !bytes.HasSuffix(b, []byte("%%EOF\n")) {
		t.Fatalf("%s: output does not end with %%%%EOF", label)
	}

	// THE EXPECTED PAGE COUNT IS A PARAMETER, and this is Story 2.6's
	// repair of a real process defect rather than a tidy-up.
	//
	// This assertion used to read `pageCount != 1`, hard-coded. That made
	// the ONE well-formedness checker in the module UNCALLABLE on a
	// multi-page document: it would have fatalled on the very property that
	// made the document interesting. So when Story 2.6 recorded the first
	// multi-page golden, this checker was simply never invoked on it — and
	// the golden shipped with an unresolvable page tree, past its hash and
	// past a bespoke acceptance step assembled from checks a broken file
	// satisfies.
	//
	// The checker existed. It could not be pointed at the subject. That is
	// worse than a missing check, because the 18 call sites elsewhere make
	// the module look covered.
	//
	// THE `!= wantPages` COMPARISON IS KEPT, NOT DELETED (D-000.34). On the
	// seven single-page fixtures it is load-bearing — a render that
	// suddenly emitted two pages must still fail — and deleting it to
	// accommodate the multi-page case would have destroyed a live assertion
	// silently, which is exactly what D-000.34 is about. Every pre-2.6 call
	// site passes 1 and is unchanged in meaning.
	pageCount := countPageObjects(b)
	if pageCount != wantPages {
		t.Fatalf("%s: expected exactly %d /Type /Page object(s), found %d", label, wantPages, pageCount)
	}

	if !bytes.Contains(b, []byte("/Type /Catalog")) {
		t.Fatalf("%s: no /Type /Catalog object found", label)
	}

	xrefOffset := assertStartxrefPointsAtXref(t, label, b)
	assertXrefEntriesPointAtTheirObjects(t, label, b, xrefOffset)
	assertStreamLengthsAreExact(t, label, b)
	assertIndirectRefsAreDelimited(t, label, b)
}

// nonStreamRegions returns the byte ranges of b that are NOT inside a stream
// body, by walking the same /Length -> "stream\n" -> body -> "endstream"
// structure assertStreamLengthsAreExact validates.
//
// WHY THE SCOPING IS NECESSARY AND WHY IT IS SOUND. Stream bodies are
// arbitrary binary — subsetted font programs and image bytes — and can
// contain any sequence, including "R" followed by a digit. A token check run
// over the whole file would be a false-positive generator, and a false
// positive in a guard is an attack on the guard (D-000.15): the first one
// gets the guard suppressed. The scoping is EXACT rather than heuristic
// because every stream in this module declares its length up front, and
// assertStreamLengthsAreExact — which runs immediately before this — has
// already fatalled if any declared length did not land exactly on
// "endstream". So the regions are known, not guessed.
//
// UNDECLARED COUPLING, NAMED (Story 2.6 finisher, Finding 8 — Minor, not
// a reason to drop this guard). This function is a SECOND, INDEPENDENT
// copy of assertStreamLengthsAreExact's walk (/Length -> "stream\n" ->
// body -> "endstream"), differing only in what happens on a malformed
// condition: that function t.Fatalf's, this one silently `break`s. The
// soundness argument above depends on (a) assertWellFormedPDF calling
// assertStreamLengthsAreExact immediately before this, and (b) the two
// walks staying byte-for-byte equivalent — NEITHER is asserted by the
// type system or by a test. Measured at the time of writing: region/body
// separation is exact on every committed golden (0 bytes of stream body
// leak into a scanned region, the /Length1 font shape included), and even
// a TOTAL scoping failure would produce 0 false positives on today's
// corpus — so this is not a live D-000.15 false-positive generator. But
// if a future edit relaxes one walk (e.g. adds "stream\r\n" support, or
// widens the 40-byte window) without the other, `break` fires early and
// the ENTIRE REMAINDER OF THE FILE, font programs included, silently
// becomes a "non-stream region". If you touch either walk, touch both.
func nonStreamRegions(b []byte) [][]byte {
	var out [][]byte
	cursor := 0
	idx := 0
	for {
		const kw = "/Length "
		rel := bytes.Index(b[idx:], []byte(kw))
		if rel == -1 {
			break
		}
		pos := idx + rel + len(kw)
		end := pos
		for end < len(b) && b[end] >= '0' && b[end] <= '9' {
			end++
		}
		if end == pos {
			break
		}
		declared, err := strconv.Atoi(string(b[pos:end]))
		if err != nil {
			break
		}
		const streamKW = "stream\n"
		streamRel := bytes.Index(b[end:], []byte(streamKW))
		if streamRel == -1 || streamRel > 40 {
			break
		}
		bodyStart := end + streamRel + len(streamKW)
		bodyEnd := bodyStart + declared
		if bodyEnd > len(b) {
			break
		}
		out = append(out, b[cursor:bodyStart])
		cursor = bodyEnd
		idx = bodyEnd
	}
	out = append(out, b[cursor:])
	return out
}

// refTokenRE matches an indirect reference token together with the ONE byte
// that follows it, so the follower can be inspected.
var refTokenRE = regexp.MustCompile(`\d+ 0 R(.|\n)`)

// isPDFDelimiter reports whether c may legally terminate a token: ISO 32000-1
// Table 1 (white space) and Table 2 (delimiters).
func isPDFDelimiter(c byte) bool {
	switch c {
	case 0x00, '\t', '\n', '\f', '\r', ' ':
		return true
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}

// assertIndirectRefsAreDelimited is Story 2.6's artifact-level guard on the
// defect that shipped a golden no viewer would open.
//
// IT IS KEYED ON THE DEFECT'S SHAPE, NOT ON /Kids (D-000.15: key a guard on
// its purpose, never on a proxy). The purpose is "an indirect reference must
// be a TOKEN a parser can terminate". The bug happened to be in /Kids, but
// /Kids is NOT the only ref array the format emits today — CORRECTED by the
// finisher (Finding 3): /DescendantFonts is a second one, live in most
// goldens. What is true is narrower: /Kids is the only ref array that is
// ever MULTI-ELEMENT today, which is a fact about the current feature set,
// not about the code. Story 2.7's page numbering and Epic 4's tables add
// ref arrays that can be multi-element, as would any /Annots or name tree.
//
// FORWARD GUARD, LABELLED (D-000.24). For /Kids this has a real red-proof:
// the actual recorded bytes "[8 0 R10 0 R]" redden it — reproduced again by
// the finisher (Blocker 1 / Major 2 red-proof) after routing document.go's
// page tree and textdoc.go's /DescendantFonts through appendRefArray /
// writeRefArray, confirming both remaining sites moved no golden. For
// /DescendantFonts and any future ref array there is still no red-proof
// available, because neither is ever multi-element yet. That half remains a
// forward guard on a hazard the code can commit but has not, and it is NOT
// counted as proven coverage.
func assertIndirectRefsAreDelimited(t *testing.T, label string, b []byte) {
	t.Helper()

	regions := nonStreamRegions(b)
	if len(regions) == 0 {
		t.Fatalf("%s: the non-stream scoping produced no regions, so this check would look nowhere", label)
	}
	refs := 0
	for _, region := range regions {
		for _, m := range refTokenRE.FindAllSubmatch(region, -1) {
			refs++
			follower := m[1][0]
			if !isPDFDelimiter(follower) {
				t.Errorf("%s: the indirect reference %q is followed by %q, which is neither white space nor a PDF delimiter.\n\n"+
					"A reference must be a TOKEN a parser can terminate. \"8 0 R10 0 R\" tokenizes as one unknown "+
					"token \"R10\", so NEITHER reference resolves — that is how a two-page document shipped with an "+
					"EMPTY page tree behind a correct-looking /Count, and a viewer showed \"page 0 of 2\".",
					label, bytes.TrimRight(m[0], string(follower)), string(follower))
			}
		}
	}
	// PRESENCE PRECONDITION: a document with no indirect references at all
	// would satisfy "every reference is delimited" by having none.
	if refs == 0 {
		t.Fatalf("%s: no indirect reference found outside stream data — every PDF this module emits has at least a /Parent and a /Contents, so finding none means the scoping is broken and this check looked nowhere", label)
	}
}

// countPageObjects counts occurrences of "/Type /Page" that are not the
// start of "/Type /Pages" — a substring match alone (as the previous
// version of this file relied on) cannot tell "/Type /Page " from
// "/Type /Page>>" apart from "/Type /Pages", except by accident of a
// trailing space (Nit 26).
func countPageObjects(b []byte) int {
	needle := []byte("/Type /Page")
	count := 0
	idx := 0
	for {
		rel := bytes.Index(b[idx:], needle)
		if rel == -1 {
			return count
		}
		pos := idx + rel
		end := pos + len(needle)
		if end < len(b) && b[end] == 's' { // "/Type /Pages", not a page object
			idx = end
			continue
		}
		count++
		idx = end
	}
}

// assertStartxrefPointsAtXref parses "startxref\n<offset>\n" and checks
// that offset genuinely indexes to the start of the "xref" keyword, and
// returns that offset.
func assertStartxrefPointsAtXref(t *testing.T, label string, b []byte) int {
	t.Helper()

	const kw = "startxref\n"
	six := bytes.LastIndex(b, []byte(kw))
	if six == -1 {
		t.Fatalf("%s: no startxref keyword found", label)
	}
	rest := b[six+len(kw):]
	eol := bytes.IndexByte(rest, '\n')
	if eol == -1 {
		t.Fatalf("%s: startxref value is not newline-terminated", label)
	}
	offsetStr := string(rest[:eol])
	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		t.Fatalf("%s: startxref value %q is not an integer: %v", label, offsetStr, err)
	}
	if offset < 0 || offset >= len(b) || !bytes.HasPrefix(b[offset:], []byte("xref")) {
		t.Fatalf("%s: startxref offset %d does not point at the xref keyword", label, offset)
	}
	return offset
}

// assertXrefEntriesPointAtTheirObjects parses the classic xref table
// starting at xrefOffset and checks that every in-use entry's offset
// genuinely lands on "N 0 obj" for its own object number.
//
// Repaired by Story 2.6a (AC8, D-000.50): before the repair this function
// parsed exactly ONE xref subsection and returned, without ever checking
// whether the bytes following its "count" entries were the "trailer"
// keyword or a second, unexamined subsection — "one of something encoded
// as an invariant", the same class as Finding 1 and Finding 2. It now
// loops over every subsection the table declares and fatals if what
// follows the last one's entries is neither "trailer" nor a further
// well-formed "start count" header, instead of silently stopping.
//
// Story 2.6a review, Blocker 2 (fixed by the finisher): the FIRST version
// of this repair checked for "trailer" before ever consuming a subsection
// header, so a table declaring ZERO subsections returned cleanly without
// one assertion — the exact defect class this repair exists to remove,
// reintroduced by the repair itself (D-000.48). It now fatals if "trailer"
// is reached with no subsection yet examined.
//
// Story 2.6a review, Finding 8 (fixed by the finisher): the FIRST version
// also required each subsequent subsection to start EXACTLY where the
// previous one ended, which rejects the legal non-contiguous form ISO
// 32000-1 §7.5.4 explicitly permits. It now only rejects a subsection
// that OVERLAPS an object number a previous subsection already covered
// (start < the running next-object-number), and accepts a gap.
//
// This repo's serializer emits exactly one subsection by construction (a
// single contiguous object range), so no COMMITTED GOLDEN can express any
// of this — every golden here has exactly one subsection (Story 2.6a,
// Measured findings 3). But D-000.50 asks whether ANY subject can express
// the defect, not merely a committed one: the four
// TestAssertXrefEntriesPointAtTheirObjects* tests below build small
// in-memory literal buffers (no fixture, no emitted byte) that do, so this
// guard is PROVEN (D-000.50, D-000.24), not forward — each of the two
// review findings above is red-proofed by literally reverting this
// function to its prior shape and confirming the corresponding test fails.
func assertXrefEntriesPointAtTheirObjects(t *testing.T, label string, b []byte, xrefOffset int) {
	t.Helper()

	const xrefKW = "xref\n"
	xrefBody := b[xrefOffset:]
	if !bytes.HasPrefix(xrefBody, []byte(xrefKW)) {
		t.Fatalf("%s: xref section does not start with %q", label, xrefKW)
	}
	xrefBody = xrefBody[len(xrefKW):]

	firstSubsection := true
	nextObjNum := 0
	for {
		if bytes.HasPrefix(xrefBody, []byte("trailer")) {
			if firstSubsection {
				t.Fatalf("%s: xref table declares no subsections before %q", label, "trailer")
			}
			return
		}

		headerEnd := bytes.IndexByte(xrefBody, '\n')
		if headerEnd == -1 {
			t.Fatalf("%s: xref subsection header is not newline-terminated", label)
		}
		fields := strings.Fields(string(xrefBody[:headerEnd]))
		if len(fields) != 2 {
			t.Fatalf("%s: xref subsection header %q does not have two fields", label, xrefBody[:headerEnd])
		}
		start, err1 := strconv.Atoi(fields[0])
		count, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil || count < 1 {
			t.Fatalf("%s: unexpected xref subsection header %q", label, xrefBody[:headerEnd])
		}
		if firstSubsection && start != 0 {
			t.Fatalf("%s: first xref subsection must start at object 0, header %q", label, xrefBody[:headerEnd])
		}
		if !firstSubsection && start < nextObjNum {
			t.Fatalf("%s: xref subsection header %q overlaps object %d from the previous subsection", label, xrefBody[:headerEnd], nextObjNum-1)
		}
		firstSubsection = false

		entries := xrefBody[headerEnd+1:]
		for i := 0; i < count; i++ {
			objNum := start + i
			entryStart := i * 20
			if entryStart+20 > len(entries) {
				t.Fatalf("%s: xref entry %d is truncated", label, objNum)
			}
			entry := entries[entryStart : entryStart+20]
			if entry[10] != ' ' || entry[16] != ' ' || entry[18] != ' ' || entry[19] != '\n' {
				t.Fatalf("%s: xref entry %d %q is not exactly 20 bytes in the expected shape", label, objNum, entry)
			}
			offStr := string(entry[0:10])
			genStr := string(entry[11:16])
			kind := entry[17]
			off, oerr := strconv.Atoi(offStr)
			_, gerr := strconv.Atoi(genStr)
			if oerr != nil || gerr != nil {
				t.Fatalf("%s: xref entry %d %q has a non-numeric offset or generation field", label, objNum, entry)
			}
			switch kind {
			case 'f':
				if objNum != 0 {
					t.Fatalf("%s: xref entry %d is a free entry, but only object 0 should be", label, objNum)
				}
			case 'n':
				if objNum == 0 {
					t.Fatalf("%s: xref entry 0 must be the free entry, got in-use", label)
				}
				want := strconv.Itoa(objNum) + " 0 obj"
				if off < 0 || off+len(want) > len(b) || string(b[off:off+len(want)]) != want {
					t.Fatalf("%s: xref entry %d claims offset %d, but %q was not found there", label, objNum, off, want)
				}
			default:
				t.Fatalf("%s: xref entry %d has an unrecognised entry kind %q", label, objNum, string(kind))
			}
		}
		nextObjNum = start + count
		xrefBody = entries[count*20:]
	}
}

// xrefNegativeCaseEnvVar selects which malformed xref buffer a re-exec of
// this test binary should run TestXrefEntriesRejectsMalformedSubprocess
// against, when set. The parent tests below launch a CHILD process for
// this rather than calling assertXrefEntriesPointAtTheirObjects directly
// against a `t.Run` subtest, because a subtest that fails via t.Fatalf
// unconditionally fails its parent test — and the whole `go test`
// invocation — with no supported way to catch and swallow that failure
// in-process (verified empirically: wrapping the fataling call in
// `t.Run` and checking only the returned bool still reports the outer
// test, and the package run, as FAILED). Re-executing as a subprocess
// (the same technique subprocessEnvVar and its siblings above already use
// for a different reason — isolating process state) means the FATAL
// happens in a process whose own exit status this file never asks `go
// test` to interpret; only the exit CODE, inspected by the parent test
// exactly once, decides this file's own PASS/FAIL.
const xrefNegativeCaseEnvVar = "FOLIO8_XREF_NEGATIVE_CASE"

// TestXrefEntriesRejectsMalformedSubprocess is not a test in its own
// right and asserts nothing when run ordinarily — it t.Skips immediately
// unless xrefNegativeCaseEnvVar is set, which only the re-exec launched
// by TestAssertXrefEntriesPointAtTheirObjectsRejectsZeroSubsections and
// TestAssertXrefEntriesPointAtTheirObjectsRejectsOverlappingSubsections
// does. Its own t.Fatalf, when the case under test correctly rejects its
// buffer, ends this SEPARATE process with a non-zero exit code — which is
// the signal the parent test reads.
func TestXrefEntriesRejectsMalformedSubprocess(t *testing.T) {
	which := os.Getenv(xrefNegativeCaseEnvVar)
	if which == "" {
		t.Skip("only runs when re-executed as a subprocess with " + xrefNegativeCaseEnvVar + " set")
	}

	switch which {
	case "zero-subsection":
		buf := []byte("%PDF-1.7\nxref\ntrailer\n<< /Size 1 >>\nstartxref\n9\n%%EOF\n")
		xrefOffset := bytes.Index(buf, []byte("xref\n"))
		if xrefOffset == -1 {
			t.Fatal("test buffer does not contain the xref keyword")
		}
		assertXrefEntriesPointAtTheirObjects(t, "probe", buf, xrefOffset)
	case "overlap":
		const obj1Body = "1 0 obj\n<< >>\nendobj\n"
		header := "%PDF-1.7\n"
		body := header + obj1Body
		obj1Offset := len(header)

		// Subsection 1: "0 2" covers objects 0 and 1, so the next free
		// object number is 2. Subsection 2: "1 1" re-claims object 1 —
		// an overlap, not a gap.
		xref := "xref\n" +
			"0 2\n" + xrefEntryBytes(0, 'f') + xrefEntryBytes(obj1Offset, 'n') +
			"1 1\n" + xrefEntryBytes(obj1Offset, 'n') +
			"trailer\n<< /Size 6 >>\n"
		xrefOffset := len(body)
		full := []byte(body + xref)
		assertXrefEntriesPointAtTheirObjects(t, "probe", full, xrefOffset)
	case "bad-offset-in-second-subsection":
		const obj1Body = "1 0 obj\n<< >>\nendobj\n"
		const obj5Body = "5 0 obj\n<< >>\nendobj\n"
		header := "%PDF-1.7\n"
		body := header + obj1Body + obj5Body
		obj1Offset := len(header)

		// Subsection 1: "0 2", well-formed. Subsection 2: "5 1", but its
		// entry claims an offset that does NOT land on "5 0 obj" — the
		// exact wrong-offset shape the per-entry check exists to catch,
		// now inside the SECOND subsection rather than the first, so this
		// proves that check applies uniformly rather than only to
		// whatever the loop happens to examine first.
		xref := "xref\n" +
			"0 2\n" + xrefEntryBytes(0, 'f') + xrefEntryBytes(obj1Offset, 'n') +
			"5 1\n" + xrefEntryBytes(999999, 'n') +
			"trailer\n<< /Size 6 >>\n"
		xrefOffset := len(body)
		full := []byte(body + xref)
		assertXrefEntriesPointAtTheirObjects(t, "probe", full, xrefOffset)
	default:
		t.Fatalf("unknown %s value %q", xrefNegativeCaseEnvVar, which)
	}
}

// runXrefNegativeCaseSubprocess re-executes this test binary pinned to
// TestXrefEntriesRejectsMalformedSubprocess with xrefNegativeCaseEnvVar
// set to which, and reports whether the child exited non-zero (i.e.
// whether assertXrefEntriesPointAtTheirObjects fatalled on that buffer).
func runXrefNegativeCaseSubprocess(t *testing.T, which string) (fatalled bool, output []byte) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestXrefEntriesRejectsMalformedSubprocess$", "-test.v")
	cmd.Env = append(os.Environ(), xrefNegativeCaseEnvVar+"="+which)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return false, out
	}
	if _, ok := err.(*exec.ExitError); ok {
		return true, out
	}
	t.Fatalf("could not even launch the subprocess for xref negative case %q: %v\n%s", which, err, out)
	return false, out
}

// TestAssertXrefEntriesPointAtTheirObjectsRejectsZeroSubsections is the
// Story 2.6a review's Blocker 2 red-proof, committed rather than declared
// unavailable (D-000.24, D-000.30): before this repair,
// assertXrefEntriesPointAtTheirObjects checked for the "trailer" keyword
// BEFORE ever consuming a single subsection header, so an xref table that
// declared zero subsections returned cleanly without making one
// assertion — the exact "silent truncation to nothing" class this story
// exists to remove, reintroduced by the story's own repair of a different
// instance of the same class (D-000.48). Verified against a `804eea1`
// worktree: this buffer FATALs there too (on the "does not have two
// fields" branch, by accident of the trailer body reading as one field),
// so this red-proof discriminates the SAME direction as the review's
// throwaway probe, now committed instead of discarded.
func TestAssertXrefEntriesPointAtTheirObjectsRejectsZeroSubsections(t *testing.T) {
	fatalled, out := runXrefNegativeCaseSubprocess(t, "zero-subsection")
	if !fatalled {
		t.Fatalf("assertXrefEntriesPointAtTheirObjects accepted a zero-subsection xref table without ever examining a subsection; it must fatal. Subprocess output:\n%s", out)
	}
}

// xrefEntryBytes builds one 20-byte classic xref entry in the exact shape
// assertXrefEntriesPointAtTheirObjects requires: a 10-digit offset, a
// space, a 5-digit generation, a space, an 'n' or 'f' kind byte, a space,
// and a trailing newline.
func xrefEntryBytes(offset int, kind byte) string {
	return fmt.Sprintf("%010d %05d %c \n", offset, 0, kind)
}

// TestAssertXrefEntriesPointAtTheirObjectsAcceptsNonContiguousSubsections
// is the Story 2.6a review's Finding 8 red-proof, committed: a classic
// xref table is explicitly permitted to describe non-contiguous object
// ranges (ISO 32000-1 7.5.4), so a second subsection whose "start" is
// GREATER than the previous subsection's next object number — a gap, not
// a continuation — must be accepted. Before Finding 8's fix this function
// required exact continuation (start == nextObjNum) and would have
// fatalled on this well-formed input.
func TestAssertXrefEntriesPointAtTheirObjectsAcceptsNonContiguousSubsections(t *testing.T) {
	const obj1Body = "1 0 obj\n<< >>\nendobj\n"
	const obj5Body = "5 0 obj\n<< >>\nendobj\n"
	header := "%PDF-1.7\n"
	body := header + obj1Body + obj5Body
	obj1Offset := len(header)
	obj5Offset := len(header) + len(obj1Body)

	// Subsection 1: "0 2" covers objects 0 (free) and 1 (in-use).
	// Subsection 2: "5 1" covers object 5 — a gap over objects 2..4 that
	// the first subsection never claimed and never will.
	xref := "xref\n" +
		"0 2\n" + xrefEntryBytes(0, 'f') + xrefEntryBytes(obj1Offset, 'n') +
		"5 1\n" + xrefEntryBytes(obj5Offset, 'n') +
		"trailer\n<< /Size 6 >>\n"

	xrefOffset := len(body)
	full := []byte(body + xref)

	assertXrefEntriesPointAtTheirObjects(t, "probe", full, xrefOffset)
}

// TestAssertXrefEntriesPointAtTheirObjectsRejectsOverlappingSubsections
// guards the boundary Finding 8's fix must not loosen too far: relaxing
// the continuation check from "==" to "the gap-permitting >=" must still
// reject a second subsection that re-claims an object number the first
// subsection already covered.
func TestAssertXrefEntriesPointAtTheirObjectsRejectsOverlappingSubsections(t *testing.T) {
	fatalled, out := runXrefNegativeCaseSubprocess(t, "overlap")
	if !fatalled {
		t.Fatalf("assertXrefEntriesPointAtTheirObjects accepted a second subsection that overlaps object 1 already covered by the first; it must fatal. Subprocess output:\n%s", out)
	}
}

// TestAssertXrefEntriesPointAtTheirObjectsRejectsBadOffsetInSecondSubsection
// is the second half of the Story 2.6a review's Finding 6 red-proof for
// AC8 (the first half is the non-contiguous-acceptance test above): a bad
// entry offset must still be caught when it appears in the SECOND
// subsection, proving the per-entry offset check the review's own probe
// exercised applies to every subsection the loop visits, not only
// whichever one it happens to reach first.
func TestAssertXrefEntriesPointAtTheirObjectsRejectsBadOffsetInSecondSubsection(t *testing.T) {
	fatalled, out := runXrefNegativeCaseSubprocess(t, "bad-offset-in-second-subsection")
	if !fatalled {
		t.Fatalf("assertXrefEntriesPointAtTheirObjects accepted a second subsection whose entry offset does not land on its declared object; it must fatal. Subprocess output:\n%s", out)
	}
}

// assertStreamLengthsAreExact finds every "/Length N" declaration
// followed by a stream body and checks that the body is exactly N bytes
// long, ending at a literal "endstream".
//
// COUPLING, NAMED (Story 2.6 finisher, Finding 8): nonStreamRegions
// (defined earlier in this file, and consumed by
// assertIndirectRefsAreDelimited further down) is a SECOND, INDEPENDENT
// copy of this exact walk, relying on THIS function having already
// fatalled on every malformed condition it silently `break`s on instead.
// If you change this walk's shape, change nonStreamRegions the same way,
// or its scoping silently stops covering what it claims to.
func assertStreamLengthsAreExact(t *testing.T, label string, b []byte) {
	t.Helper()

	found := 0
	idx := 0
	for {
		const kw = "/Length "
		rel := bytes.Index(b[idx:], []byte(kw))
		if rel == -1 {
			break
		}
		pos := idx + rel + len(kw)
		end := pos
		for end < len(b) && b[end] >= '0' && b[end] <= '9' {
			end++
		}
		if end == pos {
			t.Fatalf("%s: /Length at offset %d is not followed by digits", label, pos)
		}
		declared, err := strconv.Atoi(string(b[pos:end]))
		if err != nil {
			t.Fatalf("%s: /Length value %q is not an integer", label, b[pos:end])
		}

		const streamKW = "stream\n"
		streamRel := bytes.Index(b[end:], []byte(streamKW))
		if streamRel == -1 || streamRel > 40 {
			t.Fatalf("%s: no %q found shortly after /Length %d", label, streamKW, declared)
		}
		bodyStart := end + streamRel + len(streamKW)
		bodyEnd := bodyStart + declared
		if bodyEnd > len(b) {
			t.Fatalf("%s: /Length %d overruns the document", label, declared)
		}
		if !bytes.HasPrefix(b[bodyEnd:], []byte("endstream")) {
			t.Fatalf("%s: stream body does not end with endstream where /Length %d says it should", label, declared)
		}
		found++
		idx = bodyEnd
	}
	if found == 0 {
		t.Fatalf("%s: no stream object found to validate /Length against", label)
	}
}

// renderInSubprocess runs this same test binary as a fresh OS process with
// FOLIO8_SUBPROCESS_RENDER=1 set, and the given GOMAXPROCS, and returns
// what it wrote to stdout.
func renderInSubprocess(t *testing.T, gomaxprocs string) []byte {
	t.Helper()

	// -test.run=^$ explicitly selects no test function to run; the
	// subprocessEnvVar check inside TestMain above is what actually
	// routes the child process. TestMain is not itself a test function,
	// so "-test.run=^TestMain$" (the previous flag here) matched nothing
	// either — harmlessly, since the env-var short-circuit runs before
	// m.Run() either way — but it named the wrong mechanism (this
	// story's QA review, Nit 24).
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(),
		subprocessEnvVar+"=1",
		"GOMAXPROCS="+gomaxprocs,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("subprocess render (GOMAXPROCS=%s) failed: %v\nstderr: %s", gomaxprocs, err, stderr.String())
	}
	return stdout.Bytes()
}

// firstDivergence returns the byte offset of the first byte at which a and
// b differ, and a short hex window around it, for diagnosing determinism
// failures.
func firstDivergence(a, b []byte) (offset int, window string) {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i, hexWindow(a, b, i)
		}
	}
	if len(a) != len(b) {
		return n, hexWindow(a, b, n)
	}
	return -1, ""
}

func hexWindow(a, b []byte, at int) string {
	const radius = 8
	start := at - radius
	if start < 0 {
		start = 0
	}
	end := func(x []byte) int {
		e := at + radius
		if e > len(x) {
			e = len(x)
		}
		return e
	}
	var sb strings.Builder
	sb.WriteString("a=")
	sb.WriteString(hexOf(a[start:end(a)]))
	sb.WriteString(" b=")
	sb.WriteString(hexOf(b[start:end(b)]))
	return sb.String()
}

func hexOf(b []byte) string {
	const hexDigits = "0123456789abcdef"
	out := make([]byte, 0, len(b)*2)
	for _, c := range b {
		out = append(out, hexDigits[c>>4], hexDigits[c&0x0f])
	}
	return string(out)
}

// TestRenderIsByteIdenticalAcrossTwoProcesses is AC8: the same input
// rendered by two independently started OS processes produces
// byte-identical output. Child A runs with GOMAXPROCS=1 and child B with
// GOMAXPROCS=4, so the comparison also catches order-dependence introduced
// by concurrency (e.g. an accidental map iteration reaching an output
// byte, NFR1.d).
func TestRenderIsByteIdenticalAcrossTwoProcesses(t *testing.T) {
	if runtime.GOOS == "js" {
		// js/wasm has no os/exec and no pipes (measured, Story 1.2 F-3):
		// exec.Command fails at build-select time and cmd.Run() would
		// fail at "pipe: not implemented on js" regardless. This is not
		// a determinism gap — Story 1.2's cross-target matrix harness
		// (folio8-go/matrix_test.go, //go:build matrix) covers the same
		// property across all four targets, including js/wasm, via the
		// FOLIO8_SUBPROCESS_RENDER seam driven from outside the process
		// rather than via os/exec inside it. The skip keys on
		// runtime.GOOS == "js" specifically, never on "an exec call
		// returned an error" — a genuine exec failure on a Linux leg
		// must stay a failure (AC15).
		t.Skip("js/wasm has no os/exec or pipes; covered by the cross-target matrix harness (folio8-go/matrix_test.go)")
	}
	a := renderInSubprocess(t, "1")
	b := renderInSubprocess(t, "4")

	assertWellFormedPDF(t, "child A (GOMAXPROCS=1)", a, 1)
	assertWellFormedPDF(t, "child B (GOMAXPROCS=4)", b, 1)

	if !bytes.Equal(a, b) {
		offset, window := firstDivergence(a, b)
		t.Fatalf("subprocess outputs differ (len a=%d, len b=%d); first divergence at byte offset %d; %s",
			len(a), len(b), offset, window)
	}
}

// TestRenderProducesAValidPDF exercises AC5 directly in-process: the
// output is non-empty, has exactly one page, and passes the well-formed
// structural checks above. Independent-reader validation (qpdf --check)
// is recorded in the Delivery Log rather than wired into this test, since
// this module intentionally has no PDF-reading dependency (AC2).
func TestRenderProducesAValidPDF(t *testing.T) {
	tpl, err := ParseTemplate([]byte(minimalTemplateJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	res, err := Render(tpl, Data("{}"), nil, FontSet{})
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	assertWellFormedPDF(t, "Render()", res.Bytes, 1)
}

// TestRenderWithNoDateInParamsOmitsCreationAndModDate is AC9, RE-SCOPED
// at Story 1.7 as AC24 (D-1.7.7) requires: neither key appears anywhere
// in the output, and no /Info dictionary is emitted at all. It also
// checks the neighbouring metadata/compression keys the Dev Notes and
// fixture README promise are absent but that nothing previously
// asserted: /Producer, /Creator and /Metadata (XMP) are the next places
// a timestamp, tool name or machine name could leak into the bytes —
// the same AD-7 hazard AC9 exists to close, one key over — and /Filter
// is the R4 compression risk this story's Dev Notes explicitly remove
// from the variable set (Story 1.6's QA review, Minor 20).
//
// Finding 12 (Story 1.6's QA review): the previous version swept ONLY
// the fontless document (an empty FontSet). F-7 explicitly flagged
// /Filter as the compression risk on the FONT path ("if this story
// compresses the FontFile2, that assertion goes red") — but as wired,
// it could never go red, because it never saw that path: the document
// with the large binary stream, where a future /Filter or /Producer
// would actually be tempting, had no metadata guard at all. This is
// table-driven over both documents.
//
// AC24 (D-1.7.7, this story): before Params existed, this test's
// "unconditional" absence assertion was true SOLELY because nothing
// could supply a date — a comment about intent, not a statement the
// test actually pinned. Now that Params exists, the property this test
// asserts is precisely AD-7's negative case, verbatim from
// epics.md:1098-1100 ("Given no date supplied by any route … Then
// /CreationDate and /ModDate are omitted") — Story 3.7's fourth AC,
// which owns the POSITIVE case (a date supplied through params DOES
// appear). Left unconditional and unrenamed, 3.7's developer would meet
// a red test whose cheapest fix is to weaken it — exactly the erosion
// this run has spent six stories preventing (D-1.7.7, binding
// rationale). The two new cases below cover both "no params at all"
// and "params supplied, but no date key in them" — 1.7 emits no PDF
// date and builds no date type; wiring an actual date through params
// into /CreationDate is Story 3.7's, not this test's.
func TestRenderWithNoDateInParamsOmitsCreationAndModDate(t *testing.T) {
	forbidden := []string{"/CreationDate", "/ModDate", "/Info", "/Producer", "/Creator", "/Metadata", "/Filter"}

	cases := []struct {
		label  string
		tpl    *Template
		fs     FontSet
		params Params
	}{
		{label: "fontless, no params", tpl: mustParseMinimalTemplate(t), fs: FontSet{}, params: nil},
		{label: "font-text, no params", tpl: parseFontTestTemplate(t), fs: testFontSet(), params: nil},
		// AC24: params NON-EMPTY, but carrying no date key at all — a
		// render whose params supply no date, deliberately, not merely
		// a render with no params argument at all.
		{label: "fontless, params present but no date key", tpl: mustParseMinimalTemplate(t), fs: FontSet{}, params: Params(`{"reportTitle": "Q3 Statement"}`)},
	}
	for _, c := range cases {
		res, err := Render(c.tpl, Data("{}"), c.params, c.fs)
		if err != nil {
			t.Fatalf("%s: Render() error: %v", c.label, err)
		}
		b := res.Bytes
		for _, key := range forbidden {
			if bytes.Contains(b, []byte(key)) {
				t.Errorf("%s: output contains %s", c.label, key)
			}
		}
	}
}

// mustParseMinimalTemplate parses minimalTemplateJSON, failing the test
// on any parse error — the fontless counterpart to parseFontTestTemplate.
func mustParseMinimalTemplate(t *testing.T) *Template {
	t.Helper()
	tpl, err := ParseTemplate([]byte(minimalTemplateJSON))
	if err != nil {
		t.Fatalf("parse minimalTemplateJSON: %v", err)
	}
	return tpl
}

// TestRenderIDEntriesAreIdentical is AC10's identity half: both /ID array
// entries are byte-identical to each other and 16 bytes each (32 hex
// characters).
func TestRenderIDEntriesAreIdentical(t *testing.T) {
	tpl, err := ParseTemplate([]byte(minimalTemplateJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	res, err := Render(tpl, Data("{}"), nil, FontSet{})
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	b := res.Bytes

	idx := bytes.Index(b, []byte("/ID ["))
	if idx == -1 {
		t.Fatal("output does not contain an /ID array")
	}
	rest := b[idx+len("/ID ["):]

	first, rest, ok := extractAngleBracketed(rest)
	if !ok {
		t.Fatal("could not parse first /ID entry")
	}
	second, _, ok := extractAngleBracketed(rest)
	if !ok {
		t.Fatal("could not parse second /ID entry")
	}

	if len(first) != 32 {
		t.Errorf("first /ID entry is %d hex characters, want 32 (16 bytes)", len(first))
	}
	if len(second) != 32 {
		t.Errorf("second /ID entry is %d hex characters, want 32 (16 bytes)", len(second))
	}
	if first != second {
		t.Errorf("/ID entries differ: %q != %q", first, second)
	}
}

func extractAngleBracketed(b []byte) (content string, rest []byte, ok bool) {
	if len(b) == 0 || b[0] != '<' {
		return "", b, false
	}
	end := bytes.IndexByte(b, '>')
	if end == -1 {
		return "", b, false
	}
	return string(b[1:end]), b[end+1:], true
}

// letterSizeTemplateJSON is minimalTemplateJSON with "size": "Letter" —
// a legally loadable value (internal/template's closedPageSizeNames
// already accepts it, Story 1.4) that this story never built real
// dimensions for. Finding 17 (QA review), retained fixture: Render must
// return a located error, not silently substitute A4 (Story 1.3's
// standing rule: a deliberately violating fixture proves it fires, and
// the fixture is retained).
const letterSizeTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": []
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
  "fonts": {},
  "locale": "en",
  "nextId": 1,
  "page": {
    "margin": {
      "bottom": 36,
      "left": 36,
      "right": 36,
      "top": 36
    },
    "orientation": "portrait",
    "size": "Letter"
  },
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// TestRenderUnrecognisedNamedPageSizeIsLocatedError is Finding 17's fix
// (QA review): a document naming a page size other than "A4" (here,
// the schema-legal but dimensionally-unimplemented "Letter") must fail
// loudly rather than silently rendering as A4.
func TestRenderUnrecognisedNamedPageSizeIsLocatedError(t *testing.T) {
	tpl, err := ParseTemplate([]byte(letterSizeTemplateJSON))
	if err != nil {
		t.Fatalf("ParseTemplate: %v", err)
	}
	_, err = Render(tpl, Data("{}"), nil, FontSet{})
	if err == nil {
		t.Fatal("expected a located error for page.size \"Letter\" (Finding 17), got nil")
	}
	if !strings.Contains(err.Error(), "Letter") {
		t.Errorf("error does not name the unrecognised size: %v", err)
	}
}

// TestRenderNilTemplateIsLocatedError is AC14b: Render(nil, f) returns a
// located error, not silently ignoring its argument the way the
// PROVISIONAL Story 1.1-1.4 signature did — "better than a public entry
// point documented as ignoring its argument" (D-1.5.9).
func TestRenderNilTemplateIsLocatedError(t *testing.T) {
	_, err := Render(nil, Data("{}"), nil, FontSet{})
	if err == nil {
		t.Fatal("expected a located error for Render(nil, ...), got nil")
	}
}

// TestRenderEmbedsSubsetFontAsType0Identity is AC5: a document using a
// subset of a Latin face is embedded as a Type0/Identity-H composite
// font carrying a FontFile2 stream and a ToUnicode CMap.
func TestRenderEmbedsSubsetFontAsType0Identity(t *testing.T) {
	tpl := parseFontTestTemplate(t)
	res, err := Render(tpl, Data("{}"), nil, testFontSet())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	b := res.Bytes
	assertWellFormedPDF(t, "font document", b, 1)

	for _, want := range []string{"/Subtype /Type0", "/Encoding /Identity-H", "/FontFile2", "/ToUnicode", "/Subtype /CIDFontType2"} {
		if !bytes.Contains(b, []byte(want)) {
			t.Errorf("output does not contain %q", want)
		}
	}
	if !containsFontFile2(b) {
		t.Fatal("output does not contain a FontFile2 (AC10b vacuity guard)")
	}
}

// TestRenderWithFontIsByteIdenticalAcrossTwoProcesses is the font-path
// counterpart to TestRenderIsByteIdenticalAcrossTwoProcesses above,
// exercised locally (not just in the matrix): two independent OS
// processes rendering the same template+font document produce
// byte-identical output, and — AC10b's vacuity guard, applied here too
// — the output is confirmed to actually contain a FontFile2 before any
// comparison runs.
func TestRenderWithFontIsByteIdenticalAcrossTwoProcesses(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skip("js/wasm has no os/exec or pipes; covered by the cross-target matrix harness (folio8-go/matrix_test.go)")
	}
	a := renderFontInSubprocess(t, "1")
	b := renderFontInSubprocess(t, "4")

	assertWellFormedPDF(t, "child A (GOMAXPROCS=1, font)", a, 1)
	assertWellFormedPDF(t, "child B (GOMAXPROCS=4, font)", b, 1)

	if !containsFontFile2(a) {
		t.Fatal("child A output does not contain a FontFile2 (AC10b vacuity guard) — a match below would prove nothing")
	}
	if !containsFontFile2(b) {
		t.Fatal("child B output does not contain a FontFile2 (AC10b vacuity guard) — a match below would prove nothing")
	}

	if !bytes.Equal(a, b) {
		offset, window := firstDivergence(a, b)
		t.Fatalf("subprocess font-document outputs differ (len a=%d, len b=%d); first divergence at byte offset %d; %s",
			len(a), len(b), offset, window)
	}
}

func renderFontInSubprocess(t *testing.T, gomaxprocs string) []byte {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(),
		subprocessFontEnvVar+"=1",
		"GOMAXPROCS="+gomaxprocs,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("subprocess font render (GOMAXPROCS=%s) failed: %v\nstderr: %s", gomaxprocs, err, stderr.String())
	}
	return stdout.Bytes()
}

// TestEmbeddedHeadTimestampsEqualSourceExactly is AC11a: the embedded
// FontFile2's head table created (offset 20) and modified (offset 28)
// equal the SOURCE face's values exactly, byte-for-byte — equality
// alone, no "or zero" latitude and no clock read (D-1.5.10 withdrew
// both from D-1.5.4). F-5's measured values for the committed test face:
// created=3304067374, modified=3573633780.
func TestEmbeddedHeadTimestampsEqualSourceExactly(t *testing.T) {
	tpl := parseFontTestTemplate(t)
	res, err := Render(tpl, Data("{}"), nil, testFontSet())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	b := res.Bytes
	if !containsFontFile2(b) {
		t.Fatal("output does not contain a FontFile2 — cannot proceed (AC10b vacuity guard)")
	}

	embeddedProgram := extractFontFile2Program(t, b)
	embCreated, embModified := headTimesFromSFNT(t, embeddedProgram)
	srcCreated, srcModified := headTimesFromSFNT(t, testRobotoFontBytes)

	if embCreated != srcCreated {
		t.Errorf("embedded head.created = %d, want source's %d (F-5, AC11a)", embCreated, srcCreated)
	}
	if embModified != srcModified {
		t.Errorf("embedded head.modified = %d, want source's %d (F-5, AC11a)", embModified, srcModified)
	}
	if srcCreated != 3304067374 {
		t.Errorf("source head.created = %d, want the measured 3304067374 (F-5) — has testdata/fonts/Roboto-Regular.ttf changed?", srcCreated)
	}
	if srcModified != 3573633780 {
		t.Errorf("source head.modified = %d, want the measured 3573633780 (F-5) — has testdata/fonts/Roboto-Regular.ttf changed?", srcModified)
	}
}

// extractFontFile2Program locates the FIRST FontFile2 stream in a
// rendered PDF and returns its raw program bytes, by locating the
// object it references and reading exactly /Length1 bytes after
// "stream\n". This is a minimal, purpose-built extractor — not a
// general PDF parser — consistent with AC2/AC13: folio8-go has no
// PDF-reading dependency, so verification code that needs to read one
// back is written by hand, the same way assertWellFormedPDF is.
func extractFontFile2Program(t *testing.T, pdfBytes []byte) []byte {
	t.Helper()
	idx := bytes.Index(pdfBytes, []byte("/FontFile2 "))
	if idx == -1 {
		t.Fatal("no /FontFile2 key found")
	}
	rest := pdfBytes[idx+len("/FontFile2 "):]
	sp := bytes.IndexByte(rest, ' ')
	if sp == -1 {
		t.Fatal("could not parse /FontFile2 object reference")
	}
	objNum, err := strconv.Atoi(string(rest[:sp]))
	if err != nil {
		t.Fatalf("could not parse /FontFile2 object number: %v", err)
	}

	marker := []byte(strconv.Itoa(objNum) + " 0 obj\n")
	objIdx := bytes.Index(pdfBytes, marker)
	if objIdx == -1 {
		t.Fatalf("could not find object %d 0 obj", objNum)
	}
	objBody := pdfBytes[objIdx:]

	const lenKW = "/Length1 "
	lenIdx := bytes.Index(objBody, []byte(lenKW))
	if lenIdx == -1 {
		t.Fatal("FontFile2 object has no /Length1")
	}
	numStart := lenIdx + len(lenKW)
	numEnd := numStart
	for numEnd < len(objBody) && objBody[numEnd] >= '0' && objBody[numEnd] <= '9' {
		numEnd++
	}
	length, err := strconv.Atoi(string(objBody[numStart:numEnd]))
	if err != nil {
		t.Fatalf("could not parse /Length1: %v", err)
	}

	const streamKW = "stream\n"
	streamIdx := bytes.Index(objBody[numEnd:], []byte(streamKW))
	if streamIdx == -1 {
		t.Fatal("FontFile2 object has no stream keyword")
	}
	bodyStart := numEnd + streamIdx + len(streamKW)
	if bodyStart+length > len(objBody) {
		t.Fatal("FontFile2 stream overruns the document")
	}
	return objBody[bodyStart : bodyStart+length]
}

// extractAllFontFile2Programs is Story 2.2's multi-face extension of
// extractFontFile2Program above: the multi-script fixture (AC8) embeds
// THREE distinct FontFile2 streams, one per shipped face actually used
// — this walks every "/FontFile2 N 0 R" occurrence (by object number,
// deduplicated, since a Type0/CIDFontType2 pair can each reference the
// FontDescriptor that carries it) and returns each program's bytes in
// ascending object-number order (deterministic — never map iteration
// order).
func extractAllFontFile2Programs(t *testing.T, pdfBytes []byte) [][]byte {
	t.Helper()

	seen := map[int]bool{}
	var objNums []int
	rest := pdfBytes
	for {
		idx := bytes.Index(rest, []byte("/FontFile2 "))
		if idx == -1 {
			break
		}
		after := rest[idx+len("/FontFile2 "):]
		sp := bytes.IndexByte(after, ' ')
		if sp == -1 {
			t.Fatal("could not parse /FontFile2 object reference")
		}
		objNum, err := strconv.Atoi(string(after[:sp]))
		if err != nil {
			t.Fatalf("could not parse /FontFile2 object number: %v", err)
		}
		if !seen[objNum] {
			seen[objNum] = true
			objNums = append(objNums, objNum)
		}
		rest = after[sp:]
	}
	if len(objNums) == 0 {
		t.Fatal("no /FontFile2 key found")
	}
	sort.Ints(objNums) // deterministic order — never rely on map/seen iteration.

	programs := make([][]byte, 0, len(objNums))
	for _, objNum := range objNums {
		marker := []byte(strconv.Itoa(objNum) + " 0 obj\n")
		objIdx := bytes.Index(pdfBytes, marker)
		if objIdx == -1 {
			t.Fatalf("could not find object %d 0 obj", objNum)
		}
		objBody := pdfBytes[objIdx:]

		const lenKW = "/Length1 "
		lenIdx := bytes.Index(objBody, []byte(lenKW))
		if lenIdx == -1 {
			t.Fatalf("object %d has no /Length1", objNum)
		}
		numStart := lenIdx + len(lenKW)
		numEnd := numStart
		for numEnd < len(objBody) && objBody[numEnd] >= '0' && objBody[numEnd] <= '9' {
			numEnd++
		}
		length, err := strconv.Atoi(string(objBody[numStart:numEnd]))
		if err != nil {
			t.Fatalf("object %d: could not parse /Length1: %v", objNum, err)
		}

		const streamKW = "stream\n"
		streamIdx := bytes.Index(objBody[numEnd:], []byte(streamKW))
		if streamIdx == -1 {
			t.Fatalf("object %d has no stream keyword", objNum)
		}
		bodyStart := numEnd + streamIdx + len(streamKW)
		if bodyStart+length > len(objBody) {
			t.Fatalf("object %d: FontFile2 stream overruns the document", objNum)
		}
		programs = append(programs, objBody[bodyStart:bodyStart+length])
	}
	return programs
}

// sfntHasTable reports whether a raw sfnt program's table directory
// names tag (a 4-byte OpenType table tag, e.g. "fvar"). Used by AC8's
// feature guard to assert an embedded program is genuinely INSTANCED —
// a variable font retains fvar/gvar/avar; PinAllAxesToDefault's whole
// purpose (Story 2.2, AC7) is to make the SUBSETTED, embedded program
// carry NONE of them.
func sfntHasTable(t *testing.T, data []byte, tag string) bool {
	t.Helper()
	if len(data) < 12 {
		t.Fatal("sfnt data too short to have a table directory")
	}
	numTables := int(data[4])<<8 | int(data[5])
	for i := 0; i < numTables; i++ {
		recStart := 12 + i*16
		if recStart+4 > len(data) {
			t.Fatal("sfnt table directory overruns data")
		}
		if string(data[recStart:recStart+4]) == tag {
			return true
		}
	}
	return false
}

// headTimesFromSFNT reads an sfnt's head table created/modified fields
// (offsets 20 and 28, LONGDATETIME) directly from its table directory —
// the same technique fontset_test.go's patchUnitsPerEm helper uses, kept
// as an independent, from-scratch read here rather than importing
// internal/fontset (this test verifies the PUBLIC Render output, not
// the internal seam).
func headTimesFromSFNT(t *testing.T, data []byte) (created, modified int64) {
	t.Helper()
	if len(data) < 12 {
		t.Fatal("sfnt data too short")
	}
	numTables := int(data[4])<<8 | int(data[5])
	for i := 0; i < numTables; i++ {
		recStart := 12 + i*16
		if recStart+16 > len(data) {
			t.Fatal("sfnt table directory overruns data")
		}
		rec := data[recStart : recStart+16]
		if string(rec[0:4]) != "head" {
			continue
		}
		off := int(rec[8])<<24 | int(rec[9])<<16 | int(rec[10])<<8 | int(rec[11])
		if off+54 > len(data) {
			t.Fatal("head table overruns data")
		}
		head := data[off : off+54]
		created = beInt64(head[20:28])
		modified = beInt64(head[28:36])
		return created, modified
	}
	t.Fatal("sfnt has no head table")
	return 0, 0
}

func beInt64(b []byte) int64 {
	var v uint64
	for _, c := range b {
		v = v<<8 | uint64(c)
	}
	return int64(v)
}

// statementSubprocessSelectors is Story 4.7's env-gated render seam, one
// entry per recorded statement document. Each renders the SAME template
// and params document with a data document of its own collection length,
// in a FRESH process — the same reason multi-page and page-count-20
// needed one: a golden recorded from one process pins whatever that
// process happened to do.
//
// The rows field is deliberately NOT read from statementFixtures'
// pages field: this list and statementFixtures are two independent
// statements of the same four collection lengths, and
// TestStatementSubprocessSelectorsCoverEveryFixture below compares them.
var statementSubprocessSelectors = []struct {
	env  string
	slug string
	rows int
}{
	{"FOLIO8_SUBPROCESS_RENDER_STATEMENT1", "statement-1", 5},
	{"FOLIO8_SUBPROCESS_RENDER_STATEMENT5", "statement-5", 95},
	{"FOLIO8_SUBPROCESS_RENDER_STATEMENT20", "statement-20", 425},
	{"FOLIO8_SUBPROCESS_RENDER_STATEMENT50", "statement-50", 1085},
}

// TestStatementSubprocessSelectorsCoverEveryFixture is the guard the
// comment above promises, and this story's review Finding 4: the comment
// cited it by name, `grep` returned exactly one hit — the comment itself
// — and the `slug` field it reconciles was read by nothing at all. A
// message that points at a mechanism which is not there is D-000.37's
// second clause exactly, and the duplication it justifies was left
// unguarded.
//
// The duplication is deliberate and sound: statementSubprocessSelectors
// and statementFixtures are TWO INDEPENDENT STATEMENTS of the same four
// collection lengths, so a typo in either is visible rather than
// self-consistent. What was missing is the thing that makes independence
// safe — something that compares them. Without it, a drift in
// `rows` would be caught only by the matrix legs, which sit behind
// //go:build matrix and do not run per-commit.
func TestStatementSubprocessSelectorsCoverEveryFixture(t *testing.T) {
	if len(statementSubprocessSelectors) == 0 || len(statementFixtures) == 0 {
		t.Fatal("vacuity guard: one of the two lists is empty, so set equality below would compare nothing")
	}
	if len(statementSubprocessSelectors) != len(statementFixtures) {
		t.Fatalf("statementSubprocessSelectors declares %d document(s) and statementFixtures declares %d — every recorded statement document needs a subprocess render seam, and a seam with no document renders nothing the matrix compares",
			len(statementSubprocessSelectors), len(statementFixtures))
	}
	seenEnv := map[string]bool{}
	for i, sel := range statementSubprocessSelectors {
		f := statementFixtures[i]
		if sel.slug != f.slug {
			t.Errorf("entry %d: the subprocess selector names %q and statementFixtures names %q — the two lists are in different orders, so every comparison below is between different documents",
				i, sel.slug, f.slug)
			continue
		}
		if sel.rows != f.rows {
			t.Errorf("%s: the subprocess selector renders a collection of %d row(s) and statementFixtures records %d.\n\n"+
				"These are two independent statements of one number and they have DRIFTED. The matrix leg would then capture a document that is not the one the golden was recorded from, and the byte comparison would fail with a message about hashes rather than about this.",
				sel.slug, sel.rows, f.rows)
		}
		if seenEnv[sel.env] {
			t.Errorf("%s: the env selector %q is used by more than one document, so two legs would capture the same render", sel.slug, sel.env)
		}
		seenEnv[sel.env] = true
	}
	t.Logf("subprocess-selector witness: %d selector(s) reconciled against statementFixtures by slug and row count: %v",
		len(statementSubprocessSelectors), func() []string {
			var out []string
			for _, sel := range statementSubprocessSelectors {
				out = append(out, sel.slug+"="+itoaForTest(int64(sel.rows)))
			}
			return out
		}())
}
