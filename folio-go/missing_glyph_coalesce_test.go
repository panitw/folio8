package folio8_test

// TestMissingGlyphWarningsCoalesceByDistinctRunePerElement is AC7
// (D-3.7.3, OVERRULING the creator's presentation-layer recommendation):
// the missing-glyph Warning is coalesced IN THE ENGINE to one per
// (element, distinct rune), in FIRST-OCCURRENCE order — never one per
// occurrence, and never de-duplicated by map iteration (AD-1, D-2.8.6:
// the diagnostics slice's order is a determinism guarantee).
//
// The fixture interleaves two distinct uncovered runes (Thai "ก" and
// "ข", neither covered by the single-face "Noto Sans" chain) among
// covered ASCII runes, with "ก" occurring first and twice, and "ข"
// occurring second and once — so a map-ranged implementation (which
// would still get the COUNT right, 2 distinct runes) can be told apart
// from the required linear-scan, first-occurrence-order
// implementation only by checking the ORDER, which this test does.
//
// D-3.7.8 (this story's review, Finding 7 / the engineering lead's
// follow-up ruling): the ordering assertion above is a PROBABILISTIC
// backstop, not the deterministic guard — over a two-distinct-rune
// population, Go's randomised map-iteration start reproduces the
// "correct" order roughly HALF the time, so a map-ranged
// de-duplication reddens this file's own ordering check on only
// ~5 of 10 runs (measured in review). The DETERMINISTIC catch, every
// run, is `lint`'s TestMapRangeUnderModule
// (lint/internal/rules/maprange_test.go), which fires on the bare
// `range` over the de-duplication map itself:
// "map-range: render.go:1075: range over a map value is forbidden
// (AD-1, NFR1.d)". Neither this file, D-3.7.3 nor the Delivery Log
// named that guard before this correction — see the decision log's
// D-3.7.8 entry.
//
// TestMissingGlyphWarningsCoalescingIsDeterministicAcrossRepeatedRuns
// below is this file's OWN deterministic guard, added for the same
// reason: it asserts AD-1's real property (same input, same output)
// by running the coalescing many times and comparing every result to
// the first, rather than pinning a single golden order. Its
// discriminating power (1 − 2⁻ⁿ over n runs) does not depend on the
// population being small — unlike the ordering assertion above, which
// is only as strong as the interleaving this one fixture happens to
// construct.

import (
	"strings"
	"testing"

	folio8 "github.com/panitw/folio8/folio-go"
	"github.com/panitw/folio8/folio-go/fonts"
)

// coalesceSingleFaceTemplateJSON is shared by
// TestMissingGlyphWarningsCoalesceByDistinctRunePerElement and
// TestMissingGlyphWarningsCoalescingIsDeterministicAcrossRepeatedRuns
// (D-3.7.8) below — the same fixture, so the determinism guard is
// exercising the exact same coalescing this file's ordering assertion
// pins, not a similar-looking one.
const coalesceSingleFaceTemplateJSON = `{
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

func TestMissingGlyphWarningsCoalesceByDistinctRunePerElement(t *testing.T) {
	tpl, err := folio8.ParseTemplate([]byte(coalesceSingleFaceTemplateJSON))
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}

	// "a" and "b" are covered by Noto Sans; "ก" (U+0E01) and "ข"
	// (U+0E02) are not. "ก" occurs first (twice); "ข" occurs second
	// (once) — a 3-occurrence, 2-distinct-rune interleaving.
	data := folio8.Data(`{"name": "aกbกขb"}`)
	// fonts.Shipped() supplies Noto Sans Thai too, but it is irrelevant
	// here because it is not part of THIS document's declared chain
	// (AD-8: the chain is document-declared) — the same reasoning
	// TestMissingGlyphDiagnosticFiresOnUncoveredRune (ac4_coverage_test.go)
	// already relies on.
	res, err := folio8.Render(tpl, data, folio8.Params(`{}`), fonts.Shipped())
	if err != nil {
		t.Fatalf("Render() error: %v — a rune with no coverage in any chain member is a Warning, not a render failure", err)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("a missing-glyph Warning must accompany a SUCCESSFUL render: Bytes must be non-empty")
	}

	if len(res.Diagnostics) != 2 {
		t.Fatalf("want exactly 2 Diagnostics (one per DISTINCT uncovered rune, not one per occurrence — 3 occurrences of 2 distinct runes), got %d: %+v", len(res.Diagnostics), res.Diagnostics)
	}

	wantRuneSubstr := []string{"U+0E01", "U+0E02"} // ก occurs first, ข second
	for i, want := range wantRuneSubstr {
		d := res.Diagnostics[i]
		if d.Severity != folio8.SeverityWarning {
			t.Errorf("Diagnostics[%d].Severity = %v, want SeverityWarning", i, d.Severity)
		}
		if d.Code != folio8.DiagCodeTextMissingGlyph {
			t.Errorf("Diagnostics[%d].Code = %q, want %q", i, d.Code, folio8.DiagCodeTextMissingGlyph)
		}
		if d.ElementID != "e1" {
			t.Errorf("Diagnostics[%d].ElementID = %q, want %q", i, d.ElementID, "e1")
		}
		if !strings.Contains(d.Message, want) {
			t.Errorf("Diagnostics[%d].Message does not name %s in first-occurrence order: %v", i, want, d.Message)
		}
	}
	// Ordering assertion, explicitly: ก's diagnostic must precede ข's.
	// A map-ranged de-duplication (the AD-1 violation this guardrail
	// exists to prevent) would satisfy the count above but could not be
	// relied on to satisfy this order.
	if !strings.Contains(res.Diagnostics[0].Message, "U+0E01") || !strings.Contains(res.Diagnostics[1].Message, "U+0E02") {
		t.Fatalf("Diagnostics are not in first-occurrence order: %+v", res.Diagnostics)
	}
}

// TestMissingGlyphWarningsCoalescingIsDeterministicAcrossRepeatedRuns
// is D-3.7.8's own in-module guard (this story's review, Finding 7):
// runs the SAME coalescing, on the SAME input, at least 64 times
// in-process, and asserts every result is IDENTICAL to the first. Go
// re-randomises a map's iteration start offset PER ITERATION, not per
// process, so in-process repetition genuinely varies it — this is why
// this loop, unlike the single ordering assertion above, has
// discriminating power independent of the population being only two
// distinct runes: it asserts AD-1's real property directly (same
// input, same output), rather than pinning one golden order a
// two-element map-ranged implementation could satisfy on any given
// run by chance.
func TestMissingGlyphWarningsCoalescingIsDeterministicAcrossRepeatedRuns(t *testing.T) {
	tpl, err := folio8.ParseTemplate([]byte(coalesceSingleFaceTemplateJSON))
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	data := folio8.Data(`{"name": "aกbกขb"}`)
	fs := fonts.Shipped()

	const repeats = 64
	var first []folio8.Diagnostic
	for i := 0; i < repeats; i++ {
		res, rerr := folio8.Render(tpl, data, folio8.Params(`{}`), fs)
		if rerr != nil {
			t.Fatalf("run %d: Render() error: %v", i, rerr)
		}
		if i == 0 {
			first = res.Diagnostics
			continue
		}
		if len(res.Diagnostics) != len(first) {
			t.Fatalf("run %d: got %d Diagnostics, run 0 got %d — coalescing is NOT deterministic across repeated runs", i, len(res.Diagnostics), len(first))
		}
		for j := range first {
			if res.Diagnostics[j].Message != first[j].Message || res.Diagnostics[j].Code != first[j].Code {
				t.Fatalf("run %d: Diagnostics[%d] = %+v, run 0's Diagnostics[%d] = %+v — coalescing is NOT deterministic across repeated runs (AD-1: same input must give the same output, every run)", i, j, res.Diagnostics[j], j, first[j])
			}
		}
	}
}
