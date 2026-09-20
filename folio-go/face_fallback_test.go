package folio8

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/bind"
)

// This file is the I/O & Edge-Case Matrix of "Resolve a face once, and stop
// demanding a font set nobody reads", one test per row, plus the two
// properties the matrix cannot express as a row: determinism, and the line
// box agreeing with the face that actually paints it.
//
// ⚠ EVERY SUBSTITUTION CASE HERE USES A SINGLE-ENTRY CHAIN WHERE IT CAN,
// AND THAT IS THE POINT. An earlier attempt at this story passed its whole
// matrix while the capability's headline case — a document naming ONE brand
// face, rendered on a host that lacks it — still refused, because every test
// happened to use a two-entry chain with a present member and so never
// reached the vertical model's empty-metrics arm. A green suite over
// everything except the thing the story exists for.

// brandChainTemplateJSON is the story's Success signal in miniature: a
// document whose ONLY chain entry is a face no renderer in this repository
// has. Latin text, so the shipped set can cover it — the question this
// fixture asks is about SUPPLY, never about script coverage.
const brandChainTemplateJSON = `{
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
  "fonts": {"body": ["Brand Face"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// presentThenAbsentChainTemplateJSON is matrix rows 2 and 3's shape: one
// member the caller supplies and one they do not. Its text is chosen per
// test through the binding.
const presentThenAbsentChainTemplateJSON = `{
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
  "fonts": {"body": ["Noto Sans", "Brand Face"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// substitutingTableTemplateJSON is finding #7's shape: a table whose column
// is shaped ONCE PER ROW through a chain naming an absent face. The
// per-(element, distinct rune) rule is a claim about the memo that outlives
// the shapeSegments call, and only a multi-row table can measure it.
const substitutingTableTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "items[]", "headerHeight": 20,
         "style": {"fontFamily": "body", "fontSize": 9},
         "columns": [{"id": "e2", "label": "Label", "width": 180, "bind": "{{row.v}}"}]}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Brand Face"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

func parseFixture(t *testing.T, source string) *Template {
	t.Helper()
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return tpl
}

// allEmbeddedTemplateJSON is CAP-7's document: a chain whose EVERY entry is
// an asset the document carries.
//
// It is derived from fixtures/embedded-font by dropping the shipped face
// from the front of the chain, and the derivation is the whole reason it
// exists. That fixture's chain is ["Noto Sans", <asset>], so supplying
// "Noto Sans" legitimately CHANGES the page — an absent chain member
// contributes no line metrics and a present one does — and the strong claim
// ("the same bytes as rendering it with any font set whatsoever") is simply
// false over it. Stripping the shipped entry is what makes the claim true,
// and what makes a test of it worth writing.
func allEmbeddedTemplateJSON(t *testing.T) string {
	t.Helper()
	const chainHead = "      \"Noto Sans\",\n      {"
	source := embeddedFontTemplateJSON()
	if !strings.Contains(source, chainHead) {
		t.Fatal("fixture precondition: fixtures/embedded-font's chain no longer begins with the shipped face, so this derivation is stale")
	}
	return strings.Replace(source, chainHead, "      {", 1)
}

// TestAnAllEmbeddedDocumentNeedsNoFontSet is CAP-7 and the acceptance
// criterion behind issue #1: the font set contributes NOTHING to a document
// that carries every face it names, so it cannot change a byte — and the
// call is not refused before resolution has been attempted.
//
// THE SIX FONT SETS ARE THE POINT, AND EACH ONE CARRIES A DIFFERENT
// CLAIM. Empty and nil prove the refusal is gone. The shipped set proves
// the bytes do not move when a real set is present. Garbage bytes and a
// zero-byte face prove the engine never PARSES what it never consults — a
// set is data the resolver may ignore, not an input it validates, and the
// zero-byte face is the workaround issue #1's reporter is living with. A
// face the chain never names proves the same thing from the other side.
func TestAnAllEmbeddedDocumentNeedsNoFontSet(t *testing.T) {
	source := allEmbeddedTemplateJSON(t)
	tpl := parseFixture(t, source)

	baseline, err := Render(tpl, Data(`{}`), nil, FontSet{})
	if err != nil {
		t.Fatalf("an all-embedded document refused an empty font set: %v", err)
	}
	if len(baseline.Bytes) == 0 {
		t.Fatal("an all-embedded document rendered no bytes")
	}

	for _, tc := range []struct {
		name  string
		fonts FontSet
	}{
		{"nil", nil},
		{"the shipped set", testShippedFontSet()},
		{"garbage bytes", FontSet{"Noto Sans": []byte("not a font at all")}},
		{"a zero-byte face", FontSet{"Noto Sans": {}}},
		{"a face the chain never names", FontSet{"Roboto": testShippedRoboto}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Render(tpl, Data(`{}`), nil, tc.fonts)
			if err != nil {
				t.Fatalf("render with %s: %v", tc.name, err)
			}
			if !bytes.Equal(got.Bytes, baseline.Bytes) {
				t.Errorf("render with %s produced %d bytes, empty set produced %d — the font set reached a document that consults none of it", tc.name, len(got.Bytes), len(baseline.Bytes))
			}
		})
	}
}

// TestValidateAcceptsAnEmptyFontSet is the same claim on the predictor: an
// argument check on Validate that Render does not make would predict a
// refusal Render would not produce.
func TestValidateAcceptsAnEmptyFontSet(t *testing.T) {
	source := allEmbeddedTemplateJSON(t)
	diags, err := Validate([]byte(source), Data(`{}`), nil, FontSet{})
	if err != nil {
		t.Fatalf("Validate refused an empty font set for an all-embedded document: %v", err)
	}
	for _, d := range diags {
		if d.Code == DiagCodeTextFaceAbsent {
			t.Errorf("Validate reported %s for a document that carries every face it names: %s", d.Code, d.Message)
		}
	}
}

// TestAChainThatCoversTheRuneIsUnaffected is matrix row 2: an absent member
// beside a present one that DOES cover the rune is the format's standing
// tolerance, and neither selector may disturb it — no diagnostic, and the
// same bytes either way.
func TestAChainThatCoversTheRuneIsUnaffected(t *testing.T) {
	tpl := parseFixture(t, presentThenAbsentChainTemplateJSON)
	strict, err := Render(tpl, Data(`{"name":"Hello"}`), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("strict render: %v", err)
	}
	lenient, err := Render(tpl, Data(`{"name":"Hello"}`), nil, testShippedFontSet(), FaceFallbackSubstitute)
	if err != nil {
		t.Fatalf("lenient render: %v", err)
	}
	for _, d := range append(append([]Diagnostic{}, strict.Diagnostics...), lenient.Diagnostics...) {
		if d.Code == DiagCodeTextFaceSubstituted || d.Code == DiagCodeTextFaceAbsent {
			t.Errorf("a chain that covers its runes reported %s: %s", d.Code, d.Message)
		}
	}
	if len(strict.Bytes) == 0 || len(lenient.Bytes) == 0 {
		t.Error("a chain that covers its runes did not render under one of the selectors")
	}
	// ⚠ NO BYTE-IDENTITY CLAIM HERE, AND THE ABSENCE IS DELIBERATE. The
	// chain has an absent member, so under lenient ANY pool face may end
	// up painted into this element and the line box is sized to hold one
	// (chainLineMetrics' pool arm). That is a LEADING difference, not a
	// coverage one: the same runes are drawn by the same face. Asserting
	// byte-identity here would pin the opposite of what the vertical
	// model is required to do. Byte-identity between the selectors is
	// asserted where it IS true and load-bearing —
	// TestALenientFullySuppliedChainIsByteIdenticalToStrict, over a chain
	// with no absent member, which is every document a caller has today.
}

// TestTheDefaultStillRefusesAnAbsentFace is matrix row 3, and it is the
// whole of D1: today's behaviour is what a caller who asks for nothing
// gets, so no existing integrator changes behaviour.
func TestTheDefaultStillRefusesAnAbsentFace(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{"a single-entry chain", brandChainTemplateJSON},
		{"a chain with a present member that does not cover the rune", presentThenAbsentChainTemplateJSON},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tpl := parseFixture(t, tc.source)
			// Thai text for the two-entry case: "Noto Sans" IS supplied
			// and covers none of it, so coverage genuinely fails while a
			// chain member is absent.
			_, err := Render(tpl, Data(`{"name":"สัญญา"}`), nil, testShippedFontSet())
			var renderErr *RenderError
			if !errors.As(err, &renderErr) || renderErr.Diagnostic.Code != DiagCodeTextFaceAbsent {
				t.Fatalf("expected %s, got %v", DiagCodeTextFaceAbsent, err)
			}
			if !strings.Contains(renderErr.Diagnostic.Message, `"Brand Face"`) {
				t.Errorf("the refusal does not name the absent face: %s", renderErr.Diagnostic.Message)
			}
		})
	}
}

// TestLenientPaintsAPoolFaceAndSaysSo is matrix row 4 — the story's Success
// signal. The SINGLE-ENTRY chain is deliberate: it reaches the vertical
// model's empty-metrics arm, which is the arm the first attempt at this
// story left refusing.
func TestLenientPaintsAPoolFaceAndSaysSo(t *testing.T) {
	tpl := parseFixture(t, brandChainTemplateJSON)
	res, err := Render(tpl, Data(`{"name":"Hi"}`), nil, testShippedFontSet(), FaceFallbackSubstitute)
	if err != nil {
		t.Fatalf("a lenient render of a single-entry absent chain was refused: %v", err)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("a lenient render produced no bytes")
	}
	var warnings []Diagnostic
	for _, d := range res.Diagnostics {
		if d.Code == DiagCodeTextFaceSubstituted {
			warnings = append(warnings, d)
		}
	}
	// "Hi" is two distinct runes, so two Warnings: the rule is per
	// (element, DISTINCT RUNE), not per element and not per glyph.
	if len(warnings) != 2 {
		t.Fatalf("expected one Warning per distinct rune (2), got %d: %+v", len(warnings), res.Diagnostics)
	}
	for _, d := range warnings {
		if d.Severity != SeverityWarning {
			t.Errorf("substitution reported as %s, not a Warning", d.Severity)
		}
		if d.ElementID != "e1" {
			t.Errorf("Warning names element %q, want e1", d.ElementID)
		}
		// The four things the message must name: the element, the rune,
		// the face requested and the face painted.
		if !strings.Contains(d.Message, "element e1") {
			t.Errorf("message does not name the element: %s", d.Message)
		}
		if !strings.Contains(d.Message, `"Brand Face"`) {
			t.Errorf("message does not name the face requested: %s", d.Message)
		}
		if !strings.Contains(d.Message, "U+00") {
			t.Errorf("message does not name the rune: %s", d.Message)
		}
		if !strings.Contains(d.Message, "painted in ") {
			t.Errorf("message does not name the face painted: %s", d.Message)
		}
	}
}

// TestLenientRendersThaiInThai is CAP-4's own success criterion, and it is
// what makes "coverage-resolved, per rune" a claim rather than a phrase: a
// substitute that cannot draw the script it replaced has substituted
// nothing, it has produced tofu.
func TestLenientRendersThaiInThai(t *testing.T) {
	tpl := parseFixture(t, brandChainTemplateJSON)
	res, err := Render(tpl, Data(`{"name":"สัญญา"}`), nil, testShippedFontSet(), FaceFallbackSubstitute)
	if err != nil {
		t.Fatalf("lenient render of Thai text: %v", err)
	}
	if len(res.Diagnostics) == 0 {
		t.Fatal("a substituted render reported nothing")
	}
	for _, d := range res.Diagnostics {
		if d.Code == DiagCodeTextFaceSubstituted && !strings.Contains(d.Message, "Noto Sans Thai") {
			t.Errorf("a Thai rune was painted in something other than the Thai face: %s", d.Message)
		}
	}
}

// pageSlotFooterTemplateJSON puts a {{page}} slot on the absent chain. It
// exists because the page-number machinery shapes through a SECOND path
// into the same seam: digitTableRun (page_number.go) shapes "0123456789"
// and HARD-FAILS with an internal error unless it gets back exactly one
// face segment of ten glyphs.
//
// Substitution has to keep that invariant. It does, and not by accident:
// substituteFace is a pure function of the rune, so all ten digits resolve
// to the same pool face and coalesce into one segment. This fixture is
// what keeps that true rather than assumed.
const pageSlotFooterTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 500, "height": 20, "value": "{{name}}", "style": {"fontFamily": "body", "fontSize": 14}}
      ]
    },
    "pageFooter": {
      "elements": [
        {"id": "e2", "type": "text", "x": 0, "y": 0, "width": 500, "height": 14, "value": "{{page}}", "style": {"fontFamily": "body", "fontSize": 9}}
      ],
      "height": 20
    },
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Brand Face"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// TestLenientShapesAPageNumberSlot is the page-number seam, which no other
// case in this file reaches: digitTableRun refuses anything but one
// ten-glyph segment, so a substitution that resolved two digits to two
// different faces would abort the render with an internal error rather
// than produce a wrong page.
func TestLenientShapesAPageNumberSlot(t *testing.T) {
	tpl := parseFixture(t, pageSlotFooterTemplateJSON)
	res, err := Render(tpl, Data(`{"name":"Hi"}`), nil, testShippedFontSet(), FaceFallbackSubstitute)
	if err != nil {
		t.Fatalf("a lenient render of a page-number slot was refused: %v", err)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("no bytes")
	}
	// AND IT IS ON THE RECORD. The digit table itself shapes with an empty
	// element id and its diagnostics are discarded by design
	// (digitTableRun's own comment), so the record has to come from the
	// footer element's own bound text — which is where it does come from:
	// the slot's reservation text is a digit, reported against e2, naming
	// the face every one of the ten digits is painted in.
	saw := false
	for _, d := range res.Diagnostics {
		if d.Code == DiagCodeTextFaceSubstituted && d.ElementID == "e2" {
			saw = true
		}
	}
	if !saw {
		t.Errorf("the page number was painted in a substitute face and nothing said so: %+v", res.Diagnostics)
	}

	// The default is unmoved: the same document still refuses.
	if _, err := Render(tpl, Data(`{"name":"Hi"}`), nil, testShippedFontSet()); err == nil {
		t.Error("the page-slot document rendered under the default selector")
	}
}

// boldBrandChainTemplateJSON asks for a weight the substitute cannot have:
// the styled chain is indexed by CHAIN ENTRY, and a pool face is not one.
const boldBrandChainTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 500, "height": 20, "value": "{{name}}", "style": {"fontFamily": "body", "fontSize": 14, "bold": true}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Brand Face"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// TestASubstitutedRuneReportsItsDroppedWeightToo: a substituted face has
// no declared variant and none may be inferred, so the requested bold is
// lost. A reader given only the substitution Warning would be told the
// typeface changed and not that the weight went with it.
func TestASubstitutedRuneReportsItsDroppedWeightToo(t *testing.T) {
	tpl := parseFixture(t, boldBrandChainTemplateJSON)
	res, err := Render(tpl, Data(`{"name":"Hi"}`), nil, testShippedFontSet(), FaceFallbackSubstitute)
	if err != nil {
		t.Fatalf("lenient render: %v", err)
	}
	substituted, undeclared := 0, 0
	for _, d := range res.Diagnostics {
		switch d.Code {
		case DiagCodeTextFaceSubstituted:
			substituted++
		case DiagCodeTextStyleFaceUndeclared:
			undeclared++
			if d.ElementID != "e1" {
				t.Errorf("the weight Warning names element %q, want e1", d.ElementID)
			}
		}
	}
	// Two distinct runes, each reported once under each code.
	if substituted != 2 {
		t.Errorf("substitution Warnings = %d, want 2", substituted)
	}
	if undeclared != 2 {
		t.Errorf("dropped-weight Warnings = %d, want 2 — the requested bold was lost in silence: %+v", undeclared, res.Diagnostics)
	}
}

// TestLenientWithNoCandidateStillRefuses is matrix row 5: the selector asks
// for a SUBSTITUTE, not for the rune to be dropped, so a renderer holding
// nothing that draws it refuses exactly as strict does.
func TestLenientWithNoCandidateStillRefuses(t *testing.T) {
	tpl := parseFixture(t, brandChainTemplateJSON)
	for _, tc := range []struct {
		name  string
		text  string
		fonts FontSet
	}{
		// Nothing at all to substitute FROM.
		{"an empty pool", "Hi", FontSet{}},
		// A full pool, and not one face in it draws this rune. The
		// substitute is COVERAGE-resolved, so "the renderer holds
		// faces" is not the same claim as "the renderer holds a
		// candidate".
		{"a pool that covers nothing of this text", "\U0001F600", testShippedFontSet()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Render(tpl, Data(`{"name":"`+tc.text+`"}`), nil, tc.fonts, FaceFallbackSubstitute)
			var renderErr *RenderError
			if !errors.As(err, &renderErr) || renderErr.Diagnostic.Code != DiagCodeTextFaceAbsent {
				t.Fatalf("expected %s with %s, got %v", DiagCodeTextFaceAbsent, tc.name, err)
			}
		})
	}
}

// TestLenientPrefersTheEmbeddedFace is matrix row 6 and D2's first arm: the
// document's own asset was chosen deliberately by the author, so it is
// searched before anything the host happened to have.
func TestLenientPrefersTheEmbeddedFace(t *testing.T) {
	// The embedded-font fixture carries Noto Sans Thai as an asset. The
	// element's OWN chain is pointed at a face nobody supplies, and the
	// asset is moved to a SECOND chain — which is what puts it in the
	// substitution pool without letting it resolve the rune directly.
	// Both it and the supplied "Noto Sans Thai" then cover the text.
	original := "  \"fonts\": {\n    \"body\": [\n      \"Noto Sans\",\n      {\n        \"asset\": \"" + embeddedFontAssetKey() + "\"\n      }\n    ]\n  },"
	replacement := "  \"fonts\": {\n    \"body\": [\n      \"Brand Face\"\n    ],\n    \"carried\": [\n      {\n        \"asset\": \"" + embeddedFontAssetKey() + "\"\n      }\n    ]\n  },"
	source := embeddedFontTemplateJSON()
	if !strings.Contains(source, original) {
		t.Fatal("fixture precondition: fixtures/embedded-font's fonts block has moved, so this derivation is stale")
	}
	source = strings.Replace(source, original, replacement, 1)
	tpl := parseFixture(t, source)
	res, err := Render(tpl, Data(`{}`), nil, testShippedFontSet(), FaceFallbackSubstitute)
	if err != nil {
		t.Fatalf("lenient render: %v", err)
	}
	found := false
	for _, d := range res.Diagnostics {
		if d.Code != DiagCodeTextFaceSubstituted {
			continue
		}
		found = true
		if !strings.Contains(d.Message, `painted in embedded "Noto Sans Thai"`) {
			t.Errorf("the supplied face won over the document's own carried one: %s", d.Message)
		}
	}
	if !found {
		t.Fatalf("no substitution was reported: %+v", res.Diagnostics)
	}
}

// TestLenientOutputIsDeterministic: the pool is ordered, never ranged, so
// identical inputs produce identical bytes — on this machine and on any
// other. A map range here would make two renders in one process disagree.
func TestLenientOutputIsDeterministic(t *testing.T) {
	tpl := parseFixture(t, brandChainTemplateJSON)
	first, err := Render(tpl, Data(`{"name":"Hello"}`), nil, testShippedFontSet(), FaceFallbackSubstitute)
	if err != nil {
		t.Fatalf("first render: %v", err)
	}
	for i := 0; i < 8; i++ {
		again, err := Render(tpl, Data(`{"name":"Hello"}`), nil, testShippedFontSet(), FaceFallbackSubstitute)
		if err != nil {
			t.Fatalf("render %d: %v", i, err)
		}
		if !bytes.Equal(first.Bytes, again.Bytes) {
			t.Fatalf("render %d differs from the first — the substitution order is not deterministic", i)
		}
		if len(again.Diagnostics) != len(first.Diagnostics) {
			t.Fatalf("render %d reported %d diagnostics, the first reported %d", i, len(again.Diagnostics), len(first.Diagnostics))
		}
	}
}

// TestTheLineBoxAccountsForTheFacePainted is the other half of
// substitution, and the defect it closes is silent: the vertical model is
// derived from the chain's PRESENT faces, so a substitute taller than every
// one of them overflows its box with nothing reported.
//
// It asserts the model is at least as tall as the painted face's own,
// which is the direction that matters — too small clips glyphs, too large
// only spaces them.
func TestTheLineBoxAccountsForTheFacePainted(t *testing.T) {
	fs := testShippedFontSet()
	tpl := parseFixture(t, presentThenAbsentChainTemplateJSON)
	lenient := newDocumentFontCache(tpl, FaceFallbackSubstitute)
	strict := newDocumentFontCache(tpl, FaceFallbackStrict)

	painted, ok := lenient.substituteFace('ส', fs)
	if !ok {
		t.Fatal("precondition: the shipped set covers no Thai rune")
	}
	paintedVM, err := chainVerticalModel([]string{painted}, 14000, defaultLineSpacing, fs, strict)
	if err != nil {
		t.Fatalf("vertical model of the painted face: %v", err)
	}
	lenientVM, err := chainVerticalModel([]string{"Noto Sans", "Brand Face"}, 14000, defaultLineSpacing, fs, lenient)
	if err != nil {
		t.Fatalf("lenient vertical model: %v", err)
	}
	if lenientVM.Advance < paintedVM.Advance || lenientVM.FirstBaseline < paintedVM.FirstBaseline || lenientVM.LastDescent < paintedVM.LastDescent {
		t.Errorf("the lenient line box %+v is smaller than the face it may paint with, %+v (%s) — a substituted glyph would overflow with no clipping warning", lenientVM, paintedVM, painted)
	}

	// AND THE STRICT MODEL IS UNMOVED. The pool arm is gated on an absent
	// chain member AND the selector, so a caller who asks for nothing gets
	// the identical arithmetic they always did.
	strictVM, err := chainVerticalModel([]string{"Noto Sans", "Brand Face"}, 14000, defaultLineSpacing, fs, strict)
	if err != nil {
		t.Fatalf("strict vertical model: %v", err)
	}
	onlyPresent, err := chainVerticalModel([]string{"Noto Sans"}, 14000, defaultLineSpacing, fs, strict)
	if err != nil {
		t.Fatalf("present-only vertical model: %v", err)
	}
	if strictVM != onlyPresent {
		t.Errorf("the strict model moved: %+v vs %+v", strictVM, onlyPresent)
	}
}

// TestALenientFullySuppliedChainIsByteIdenticalToStrict is D1 stated as a
// measurement: the only documents the selector may move are the ones that
// would otherwise have been REFUSED.
func TestALenientFullySuppliedChainIsByteIdenticalToStrict(t *testing.T) {
	tpl := parseFixture(t, missingGlyphTemplateJSON)
	strict, err := Render(tpl, Data(`{"name":"ก"}`), Params(`{}`), testShippedFontSet())
	if err != nil {
		t.Fatalf("strict: %v", err)
	}
	lenient, err := Render(tpl, Data(`{"name":"ก"}`), Params(`{}`), testShippedFontSet(), FaceFallbackSubstitute)
	if err != nil {
		t.Fatalf("lenient: %v", err)
	}
	if !bytes.Equal(strict.Bytes, lenient.Bytes) {
		t.Error("the selector moved the bytes of a document whose chain members are all supplied")
	}
	// TEXT_MISSING_GLYPH is a chain every member of which WAS supplied, so
	// the substitution arm — whose guard is the REFUSAL's guard — never
	// sees it. The rune is still dropped and still warned about.
	for _, res := range []Result{strict, lenient} {
		saw := false
		for _, d := range res.Diagnostics {
			if d.Code == DiagCodeTextMissingGlyph {
				saw = true
			}
			if d.Code == DiagCodeTextFaceSubstituted {
				t.Error("a fully supplied chain reported a substitution")
			}
		}
		if !saw {
			t.Error("the missing-glyph Warning was lost")
		}
	}
}

// TestSubstitutionIsCoalescedAcrossTableRows is finding #7: a table shapes
// a column ONCE PER ROW, so the per-(element, distinct rune) rule is a
// claim about a memo that outlives the shapeSegments call. Without it a
// five-hundred-row table emits five hundred identical Warnings per rune.
func TestSubstitutionIsCoalescedAcrossTableRows(t *testing.T) {
	tpl := parseFixture(t, substitutingTableTemplateJSON)
	res, err := Render(tpl, Data(`{"items":[{"v":"ab"},{"v":"ab"},{"v":"ab"},{"v":"ab"},{"v":"ab"}]}`), nil, testShippedFontSet(), FaceFallbackSubstitute)
	if err != nil {
		t.Fatalf("lenient table render: %v", err)
	}
	perRune := map[string]int{}
	for _, d := range res.Diagnostics {
		if d.Code == DiagCodeTextFaceSubstituted {
			perRune[d.ElementID+"|"+d.Message]++
		}
	}
	if len(perRune) == 0 {
		t.Fatalf("the table reported no substitution at all: %+v", res.Diagnostics)
	}
	// ⚠ THE BOUND IS TWO, NOT ONE, AND THAT IS THE PRECEDENT'S OWN
	// NUMBER — MEASURED, NOT ASSUMED. buildPageModel walks a table's
	// body cells in TWO passes, and the memo is per pass, so a body
	// cell's Warning appears twice for a table of any size.
	// TEXT_STYLE_FACE_UNDECLARED does exactly the same thing on exactly
	// the same fixture (five rows of "ab": two copies per rune, not
	// five), and it is shipped behaviour that predates this story.
	// Matching it is the requirement; widening the memo across passes
	// would change a shipped diagnostic's output, which is a different
	// story's change.
	//
	// WHAT THIS TEST ACTUALLY MEASURES is therefore the PER-ROW memo:
	// five rows produce two Warnings, not ten. Remove the new code from
	// coalesceFaceDiags and this reads ten.
	for key, n := range perRune {
		if n > 2 {
			t.Errorf("%d copies of one (element, rune) Warning across five rows — the cross-call memo is not carrying this code: %s", n, key)
		}
	}
}

// TestRenderToCarriesTheSelector and TestValidateCarriesTheSelector pin the
// forwarding, because dropping `fallback` from either wrapper leaves every
// other test in this file green while a lenient caller gets a refusal.
func TestRenderToCarriesTheSelector(t *testing.T) {
	tpl := parseFixture(t, brandChainTemplateJSON)
	var out bytes.Buffer
	diags, err := RenderTo(&out, tpl, Data(`{"name":"Hi"}`), nil, testShippedFontSet(), FaceFallbackSubstitute)
	if err != nil {
		t.Fatalf("RenderTo dropped the selector: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("RenderTo wrote nothing")
	}
	saw := false
	for _, d := range diags {
		if d.Code == DiagCodeTextFaceSubstituted {
			saw = true
		}
	}
	if !saw {
		t.Errorf("RenderTo returned no substitution Warning: %+v", diags)
	}
	// And omitting it is still strict.
	var strictOut bytes.Buffer
	if _, err := RenderTo(&strictOut, tpl, Data(`{"name":"Hi"}`), nil, testShippedFontSet()); err == nil {
		t.Error("RenderTo without a selector did not refuse")
	}
}

func TestValidateCarriesTheSelector(t *testing.T) {
	found, err := Validate([]byte(brandChainTemplateJSON), Data(`{"name":"Hi"}`), nil, testShippedFontSet(), FaceFallbackSubstitute)
	if err != nil {
		t.Fatalf("Validate dropped the selector: %v", err)
	}
	saw := false
	for _, d := range found {
		if d.Code == DiagCodeTextFaceSubstituted {
			saw = true
		}
	}
	if !saw {
		t.Errorf("Validate returned no substitution Warning: %+v", found)
	}
	if _, err := Validate([]byte(brandChainTemplateJSON), Data(`{"name":"Hi"}`), nil, testShippedFontSet()); err == nil {
		t.Error("Validate without a selector did not refuse")
	}
}

// TestTheSelectorIsRefusedRatherThanClamped: two selectors, or a value
// outside the closed set, is a caller bug, and silently picking one for
// them would hide it behind bytes that look fine.
func TestTheSelectorIsRefusedRatherThanClamped(t *testing.T) {
	tpl := parseFixture(t, brandChainTemplateJSON)

	_, err := Render(tpl, Data(`{"name":"Hi"}`), nil, testShippedFontSet(), FaceFallbackSubstitute, FaceFallbackStrict)
	if !errors.Is(err, errTooManyFaceFallbacks) {
		t.Errorf("two selectors: got %v, want errTooManyFaceFallbacks", err)
	}
	_, err = Render(tpl, Data(`{"name":"Hi"}`), nil, testShippedFontSet(), FaceFallback(99))
	if !errors.Is(err, errUnknownFaceFallback) {
		t.Errorf("an out-of-set selector: got %v, want errUnknownFaceFallback", err)
	}
	// Every public door, and the internal seam behind them, agree.
	if _, err := Validate([]byte(brandChainTemplateJSON), Data(`{"name":"Hi"}`), nil, testShippedFontSet(), FaceFallback(99)); !errors.Is(err, errUnknownFaceFallback) {
		t.Errorf("Validate: got %v, want errUnknownFaceFallback", err)
	}
	var out bytes.Buffer
	if _, err := RenderTo(&out, tpl, Data(`{"name":"Hi"}`), nil, testShippedFontSet(), FaceFallback(99)); !errors.Is(err, errUnknownFaceFallback) {
		t.Errorf("RenderTo: got %v, want errUnknownFaceFallback", err)
	}
	data, derr := bind.DecodeData(Data(`{"name":"Hi"}`))
	if derr != nil {
		t.Fatal(derr)
	}
	if _, _, _, _, err := buildPageModel(tpl, data, bind.Value{Kind: bind.KindObject}, testShippedFontSet(), FaceFallbackSubstitute, FaceFallbackStrict); !errors.Is(err, errTooManyFaceFallbacks) {
		t.Errorf("buildPageModel clamped where the public path refuses: %v", err)
	}
	if got, err := resolveFaceFallback(nil); err != nil || got != FaceFallbackStrict {
		t.Errorf("the absent selector is not strict: %v, %v", got, err)
	}
}

// twoCarriedFacesDoc builds a document carrying TWO faces whose DISPLAY
// names and whose MINTED names sort in OPPOSITE directions, and returns
// them in display order.
//
// THE OPPOSITION IS THE WHOLE FIXTURE. A carried face resolves under
// "asset:" plus the SHA-256 of its bytes, so a pool sorted by the minted
// name orders the document's own faces by content hash. With one carried
// face — or with two whose two orders happen to agree — an ordering
// assertion passes either way and measures nothing. Here the stub's
// family is CHOSEN from the two keys' comparison, so the two orders are
// opposite on every run whatever the hashes come out as.
func twoCarriedFacesDoc(t *testing.T) (source, first, second string) {
	t.Helper()

	// A structurally valid sfnt this build cannot parse: the version tag,
	// one declared table and a whole table directory, with no contents.
	// It LOADS, which is all this fixture needs — substitutionPool sorts
	// names and decodes nothing.
	raw := make([]byte, 28)
	binary.BigEndian.PutUint32(raw[0:], 0x00010000)
	binary.BigEndian.PutUint16(raw[4:], 1)
	binary.BigEndian.PutUint16(raw[6:], 16)
	copy(raw[12:16], "cmap")
	binary.BigEndian.PutUint32(raw[20:], 28)
	binary.BigEndian.PutUint32(raw[24:], 0)

	stubKey := fmt.Sprintf("%x", sha256.Sum256(raw))
	realKey := embeddedFontAssetKey()
	if stubKey == realKey {
		t.Fatal("fixture assumption violated: two different faces hashed to one key")
	}
	// The real face's family is the fixture's and cannot be chosen; the
	// stub's is chosen HERE, against the keys, so that display order is
	// always the reverse of minted order.
	const realFamily = "Noto Sans Thai"
	// The stub sorts FIRST by minted name -> make it sort LAST by display
	// name, and the other way round. Either way the two orders oppose.
	stubFamily := "Zzz Fixture Face"
	if stubKey > realKey {
		stubFamily = "Aaa Fixture Face"
	}
	realName, stubName := embeddedFaceName(realKey), embeddedFaceName(stubKey)
	if (realFamily < stubFamily) == (realName < stubName) {
		t.Fatalf("fixture assumption violated: display order and minted order agree (%q/%q vs %q/%q)", realFamily, stubFamily, realName, stubName)
	}

	src := embeddedFontsBlockDoc(t, `"body": [{"asset": "`+realKey+`"}], "second": [{"asset": "`+stubKey+`"}]`)
	const anchor = "  \"assets\": {\n"
	at := strings.Index(src, anchor)
	if at < 0 {
		t.Fatal("fixture assumption violated: the generated document has no assets block")
	}
	blob := "    \"" + stubKey + "\": {\n      \"data\": [\"" + base64.StdEncoding.EncodeToString(raw) + "\"],\n" +
		"      \"font\": {" + requiredLicenceKeys + ", \"family\": " + jsonStringLiteral(stubFamily) + ", \"style\": \"Regular\"},\n" +
		"      \"mediaType\": \"font/ttf\"\n    },\n"
	src = src[:at+len(anchor)] + blob + src[at+len(anchor):]

	if realFamily < stubFamily {
		return src, realName, stubName
	}
	return src, stubName, realName
}

// TestTheSubstitutionPoolIsOrderedByFaceName is D2's within-arm rule, and
// the defect it closes is that an embedded face's RESOLUTION name is
// "asset:" plus a content hash — so sorting those orders the document's own
// faces by digest, which is deterministic but is not what D2, or any doc
// comment in three languages, says.
func TestTheSubstitutionPoolIsOrderedByFaceName(t *testing.T) {
	source, firstByDisplay, secondByDisplay := twoCarriedFacesDoc(t)
	tpl := parseFixture(t, source)
	cache := newDocumentFontCache(tpl, FaceFallbackSubstitute)
	pool := cache.substitutionPool(testShippedFontSet())
	if len(pool) < 3 {
		t.Fatalf("pool too small to order: %v", pool)
	}
	// The embedded arm comes first, whole.
	embedded := 0
	for _, name := range pool {
		if !cache.isEmbedded(name) {
			break
		}
		embedded++
	}
	if embedded != 2 {
		t.Fatalf("the document's two carried faces are not at the head of the pool: %v", pool[:min(4, len(pool))])
	}
	for _, name := range pool[embedded:] {
		if cache.isEmbedded(name) {
			t.Fatalf("the two arms are interleaved: %v", pool)
		}
	}

	// ⚠ THIS IS THE ASSERTION THAT BITES. The two carried faces' display
	// order is the REVERSE of their minted order (twoCarriedFacesDoc
	// guarantees it), so sorting the embedded arm by the minted name —
	// which is what this code did before D2 was read carefully — puts
	// them the other way round and reddens here.
	if pool[0] != firstByDisplay || pool[1] != secondByDisplay {
		firstSrc, _ := cache.embedded.source(pool[0])
		secondSrc, _ := cache.embedded.source(pool[1])
		t.Errorf("the embedded arm is ordered by the minted name, not by the face name a person reads: got %s then %s",
			firstSrc.displayName(), secondSrc.displayName())
	}

	// And the supplied arm is sorted by the name the caller filed it under.
	for i := embedded + 1; i < len(pool); i++ {
		if pool[i-1] >= pool[i] {
			t.Errorf("the supplied arm is not in face-name order at %d: %v", i, pool[embedded:])
		}
	}
}

// TestTheDesignerBuildStillRefuses is the constraint D1 exists to protect:
// page_setup.go's canvas projection is how the browser learns which face to
// fetch, so it pins FaceFallbackStrict and a chain naming an uninstalled
// face still refuses with TEXT_FACE_ABSENT, naming it.
func TestTheDesignerBuildStillRefuses(t *testing.T) {
	tpl := parseFixture(t, brandChainTemplateJSON)
	_, err := canvasWithTextPaint(tpl, testShippedFontSet())
	var renderErr *RenderError
	if !errors.As(err, &renderErr) || renderErr.Diagnostic.Code != DiagCodeTextFaceAbsent {
		t.Fatalf("the canvas projection did not refuse with %s: %v", DiagCodeTextFaceAbsent, err)
	}
	if !strings.Contains(renderErr.Diagnostic.Message, `"Brand Face"`) {
		t.Errorf("the canvas refusal does not name the face to install: %s", renderErr.Diagnostic.Message)
	}
}

// renderFaceSubstitutedWitness is the diagnostic registry census's
// production witness for TEXT_FACE_SUBSTITUTED: a real Render, through the
// public API, that returns the Warning on a successful Result.
func renderFaceSubstitutedWitness(t *testing.T) Result {
	t.Helper()
	tpl, err := ParseTemplate([]byte(brandChainTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Render(tpl, Data(`{"name":"Hi"}`), Params(`{}`), testShippedFontSet(), FaceFallbackSubstitute)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
