package folio8_test

// Story 2.2, AC4: the missing-glyph diagnostic is COVERAGE-based,
// evaluated per rune against each face's cmap in chain order — never a
// proxy such as locale or "preferred face for the script" (D-2.2-D4).
// These tests exercise the REAL public Render path against the REAL
// shipped faces (fonts.Shipped()), not a synthetic fixture, so a
// regression here is a regression an integrator would actually hit.

import (
	"strings"
	"testing"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
)

const multiScriptTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 500, "height": 20, "value": "{{name}}", "style": {"fontFamily": "body", "fontSize": 14}}
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

// TestCoverageBasedFallbackSpansAllThreeShippedFaces is AC7/AC8's own
// premise made concrete: a SINGLE text element mixing Latin, Thai and
// CJK runes must actually embed all three shipped faces, each covering
// only the runes it has glyphs for — never "the whole element uses
// whichever chain member came first" (the pre-Story-2.2 behaviour).
func TestCoverageBasedFallbackSpansAllThreeShippedFaces(t *testing.T) {
	tpl, err := folio8.ParseTemplate([]byte(multiScriptTemplateJSON))
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}

	// "Ada" (Latin, Noto Sans) + "ก" (Thai, Noto Sans Thai) + "汉" (a
	// Han ideograph, Noto Sans SC) — one line genuinely mixing scripts,
	// exactly AD-8's "a template names a family plus an ordered
	// fallback chain, tried left to right per glyph" scenario.
	data := folio8.Data(`{"name": "Ada ก 汉"}`)
	res, err := folio8.Render(tpl, data, folio8.Params(`{}`), fonts.Shipped())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	pdfBytes := res.Bytes

	// V6's vacuity guard, applied to all three faces at once: a render
	// that silently dropped to one face (or produced tofu) would still
	// "succeed" — so assert the resource NAMES for all three faces
	// actually appear as Font resources, not just that /FontFile2
	// appears at all (which one face alone would also satisfy).
	// Resource/BaseFont names have spaces stripped (measured: "Noto
	// Sans" -> "NotoSans" in the Resources dict and /BaseFont), so
	// match on the sanitized form.
	s := string(pdfBytes)
	for _, want := range []string{"NotoSans", "NotoSansThai", "NotoSansSC"} {
		if !strings.Contains(s, want) {
			t.Errorf("rendered PDF does not reference face %q as a resource/BaseFont name — the fallback chain did not actually reach it", want)
		}
	}
	if n := strings.Count(s, "/FontFile2"); n < 3 {
		t.Errorf("expected at least 3 distinct /FontFile2 embeddings (one per shipped face actually used), got %d", n)
	}
}

// TestMissingGlyphDiagnosticFiresOnUncoveredRune is V8, UPDATED by
// Story 3.6 (divergence 6, OPEN-1 ruled): the fixture must contain a
// rune genuinely covered by NO face in the chain, and — per this
// story's ruling — the render must now SUCCEED, returning complete
// bytes, with a Warning naming the element id, the offending rune (both
// U+XXXX and its literal form) and the searched chain, never a silently
// emitted blank box and never an aborted render.
//
// This is the SYNTHETIC, test-only subject the lead's OPEN-1 ruling
// required: nothing in the committed corpus/fixture set exercises this
// path (measured at story creation: `grep -rn "no font in chain"`
// returned exactly one hit, the production site), and this story's new
// corpus-wide assertion (TestCorpusFixturesProduceNoMissingGlyphWarnings,
// missing_glyph_corpus_test.go) requires that no committed fixture ever
// does — so the subject has to be built here, inline, never added to a
// retained fixture directory.
func TestMissingGlyphDiagnosticFiresOnUncoveredRune(t *testing.T) {
	const singleFaceTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 500, "height": 20, "value": "{{name}}", "style": {"fontFamily": "body", "fontSize": 14}}
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
	tpl, err := folio8.ParseTemplate([]byte(singleFaceTemplateJSON))
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}

	// "ก" (Thai) has no glyph in Noto Sans, and the declared chain names
	// no other member to fall back to — fonts.Shipped() supplies Noto
	// Sans Thai too, but it is irrelevant here because it is not part of
	// THIS document's declared chain (AD-8: the chain is
	// document-declared).
	data := folio8.Data(`{"name": "ก"}`)
	res, err := folio8.Render(tpl, data, folio8.Params(`{}`), fonts.Shipped())
	if err != nil {
		t.Fatalf("Render() error: %v — a rune with no coverage in any chain member is a Warning (Story 3.6), not a render failure", err)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("a missing-glyph Warning must accompany a SUCCESSFUL render (AD-14): Bytes must be non-empty")
	}
	if len(res.Diagnostics) != 1 {
		t.Fatalf("want exactly 1 Diagnostic, got %d: %+v", len(res.Diagnostics), res.Diagnostics)
	}
	d := res.Diagnostics[0]
	if d.Severity != folio8.SeverityWarning {
		t.Errorf("Severity = %v, want SeverityWarning (FR41's fifth mode is the one Warning among five Error modes)", d.Severity)
	}
	if d.Code != folio8.DiagCodeTextMissingGlyph {
		t.Errorf("Code = %q, want %q", d.Code, folio8.DiagCodeTextMissingGlyph)
	}
	if d.ElementID != "e1" {
		t.Errorf("ElementID = %q, want %q", d.ElementID, "e1")
	}
	if !strings.Contains(d.Message, "e1") {
		t.Errorf("Message does not name the element id (e1): %v", d.Message)
	}
	if !strings.Contains(d.Message, `U+0E01`) {
		t.Errorf("Message does not name the offending rune (U+0E01, ก): %v", d.Message)
	}
	if !strings.Contains(d.Message, "ก") {
		t.Errorf("Message does not carry the rune's literal form (ก), only its U+XXXX form: %v", d.Message)
	}
	if !strings.Contains(d.Message, "Noto Sans") {
		t.Errorf("Message does not name the chain that was searched (D-000.37: naming the chain tells the reader what to fix): %v", d.Message)
	}
}

// TestJapaneseTextThroughPanCJKFaceDoesNotFireDiagnostic is V9's other
// direction (D-2.2-D4): Noto Sans SC is Pan-CJK, so Japanese text HAS
// coverage (kana + the shared ideograph set), and the diagnostic must
// NOT fire for it — the `ja` gap is glyph FORM quality (AC10), never a
// coverage failure, and conflating the two makes a diagnostic fire on a
// correct render.
//
// LIVE REGRESSION SUBJECT UNDER D-3.4.1/D-3.4.2 (Story 3.4, AC4,
// Finding 10, this story's QA review): this fixture's `"locale": "ja"`
// document is now covered by internal/expr's own locale-table tests
// too (locale.go / locale_test.go), but that does NOT make this test
// redundant — it is the one place a `ja` document is exercised end to
// end through the real Render/ParseTemplate path with real report
// data, proving a `ja` document actually LOADS and renders, which the
// locale table's own unit tests do not attempt. Do not delete this as
// redundant once — or because — the locale table grows more `ja`
// coverage of its own.
func TestJapaneseTextThroughPanCJKFaceDoesNotFireDiagnostic(t *testing.T) {
	const jaTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 500, "height": 20, "value": "{{greeting}}", "style": {"fontFamily": "body", "fontSize": 14}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans SC"]},
  "locale": "ja",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	tpl, err := folio8.ParseTemplate([]byte(jaTemplateJSON))
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}

	// Hiragana "こんにちは" ("hello") — genuinely Japanese, covered by
	// Noto Sans SC's shared kana + ideograph set, never routed through
	// any "is this face preferred for ja" check (D-2.2-D4 forbids that
	// proxy outright).
	data := folio8.Data(`{"greeting": "こんにちは"}`)
	if _, err := folio8.Render(tpl, data, folio8.Params(`{}`), fonts.Shipped()); err != nil {
		t.Fatalf("Japanese text through the Pan-CJK Noto Sans SC face must render without a missing-glyph diagnostic (D-2.2-D4): %v", err)
	}
}
