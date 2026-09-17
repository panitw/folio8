package folio8

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio8-go/internal/designer"
	"github.com/panitw/folio8/folio8-go/internal/fontset"
	"github.com/panitw/folio8/folio8-go/internal/geom"
	"github.com/panitw/folio8/folio8-go/internal/template"
)

// This file carries Story 7.2's direct assertions — the ones the feature
// would otherwise satisfy only BY CONSTRUCTION. Applying the ratio at one
// line inside verticalModel makes almost everything here true
// automatically; that is exactly the argument for asserting it rather
// than relying on it, because "true by construction" survives only as
// long as the construction does.
//
// The artifact-level half lives in line_spacing_fixture_test.go.

// lineSpacingProbeMetrics is the shipped Noto Sans face's hhea, spelled
// as fabricated metrics so the arithmetic is reachable without a font —
// the same shape TestVerticalModelArithmeticOverFabricatedMetrics uses,
// and the only way the ratio's own error paths are constructible at all.
var lineSpacingProbeMetrics = []fontset.LineMetrics{{Ascent: 1069, Descent: -293, LineGap: 0}}

// TestLineSpacingScalesAdvanceAndNothingElse is the story's central
// claim, asserted at the seam it is made at: FirstBaseline and
// LastDescent must be BIT-IDENTICAL to the unscaled model at every ratio,
// while Advance scales.
//
// This is the D-2.5a / DW-15 two-model split. Story 2.5a exists solely
// because the two models were once conflated, and a ratio that reached
// FirstBaseline would move every multi-line element's top edge and make
// every neighbour appear to jump.
func TestLineSpacingScalesAdvanceAndNothingElse(t *testing.T) {
	const fontSize = geom.Length(11000)
	base, err := verticalModel([]string{"probe"}, lineSpacingProbeMetrics, fontSize, defaultLineSpacing)
	if err != nil {
		t.Fatalf("unscaled verticalModel: %v", err)
	}
	if int64(base.FirstBaseline) != lineSpacingFirstBaselineMP || int64(base.Advance) != lineSpacingRuledAdvanceMP || int64(base.LastDescent) != lineSpacingLastDescentMP {
		t.Fatalf("the probe's unscaled model is (%d, %d, %d), want the hand-derived (%d, %d, %d) — every case below is stated against it", base.FirstBaseline, base.Advance, base.LastDescent, lineSpacingFirstBaselineMP, lineSpacingRuledAdvanceMP, lineSpacingLastDescentMP)
	}

	cases := []struct {
		ratio       int64
		wantAdvance geom.Length
		note        string
	}{
		{1000, geom.Length(lineSpacingRuledAdvanceMP), "the neutral ratio reproduces the ruled advance exactly"},
		{1500, geom.Length(lineSpacingOpenAdvanceMP), "14982*1500/1000 is exact — no rounding at all"},
		{600, geom.Length(lineSpacingTightAdvanceMP), "14982*600 = 8,989,200; the remainder 200 is below half, so round-half-to-even keeps 8989"},
		{1, 15, "14982*1 = 14982; q 14, r 982, and 982 > 500 rounds up to 15"},
		{template.MaxLineSpacingThousandths, 14982000, "the stated sanity ceiling still scales exactly"},
	}
	for _, c := range cases {
		got, err := verticalModel([]string{"probe"}, lineSpacingProbeMetrics, fontSize, c.ratio)
		if err != nil {
			t.Errorf("ratio %d: %v", c.ratio, err)
			continue
		}
		if got.Advance != c.wantAdvance {
			t.Errorf("ratio %d: Advance = %d, want %d (%s)", c.ratio, got.Advance, c.wantAdvance, c.note)
		}
		// THE ASSERTION THIS TEST EXISTS FOR.
		if got.FirstBaseline != base.FirstBaseline {
			t.Errorf("ratio %d: FirstBaseline = %d, want %d BIT-IDENTICAL to the unscaled model — lineSpacing must never move an element's first baseline", c.ratio, got.FirstBaseline, base.FirstBaseline)
		}
		if got.LastDescent != base.LastDescent {
			t.Errorf("ratio %d: LastDescent = %d, want %d BIT-IDENTICAL to the unscaled model", c.ratio, got.LastDescent, base.LastDescent)
		}
	}

	// The tight case is the one the designer canvas used to refuse: the
	// advance is BELOW the first-baseline offset, so consecutive line
	// boxes overlap. Stated as an assertion rather than a remark, because
	// a range that quietly stopped admitting it would leave every other
	// assertion here green.
	tight, err := verticalModel([]string{"probe"}, lineSpacingProbeMetrics, fontSize, 600)
	if err != nil {
		t.Fatalf("tight ratio: %v", err)
	}
	if !(tight.FirstBaseline > tight.Advance) {
		t.Errorf("at ratio 0.6 the first-baseline offset (%d) is not greater than the advance (%d) — this fixture no longer exercises tight leading at all", tight.FirstBaseline, tight.Advance)
	}
}

// TestLineSpacingRefusesAnOverflowingProduct is D-7.2.4's guard, proved
// at the seam: geom.ScaleRound PANICS on int64 product overflow, and a Go
// panic aborts the package's whole test binary — every other test in
// folio8-go then silently stops reporting, which is a suite-wide blindfold
// rather than a crash. So the precondition is discharged BEFORE the call,
// and this test would not merely fail without the guard, it would take
// the binary down (measured, by neutering the guard).
//
// It pins the route THE RATIO opens, and only that one. The route through
// an unbounded authored `fontSize` — verticalModel's own `scale` closure,
// reachable with no lineSpacing declared at all — is still open and is
// recorded as DW-26, not closed here: bounding fontSize is a
// format-domain decision on a second field that D-7.2.4 keeps out of this
// story.
func TestLineSpacingRefusesAnOverflowingProduct(t *testing.T) {
	// 1362 units at 10,000,000,000,000 millipoints is a ruled advance of
	// 13,620,000,000,000; at the ceiling ratio the product is
	// 13,620,000,000,000,000,000 — past int64's 9,223,372,036,854,775,807.
	// Spelled as an integer literal: AD-23 forbids a floating-point
	// literal anywhere under the module root, `1e13` included.
	_, err := verticalModel([]string{"probe"}, lineSpacingProbeMetrics, geom.Length(10_000_000_000_000), template.MaxLineSpacingThousandths)
	if err == nil {
		t.Fatal("a lineSpacing whose product with the ruled advance overflows int64 must be a returned error, never a panic reaching geom.ScaleRound")
	}
	if !strings.Contains(err.Error(), "overflow") {
		t.Errorf("the error must say what went wrong, got %q", err.Error())
	}
	// The ratio is quoted in the AUTHOR'S units, not the engine's: someone
	// who typed 1000 must not have to convert 1000000 thousandths back.
	for _, want := range []string{"probe", "line spacing 1000 "} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error must name %q so the failure is locatable and readable, got %q", want, err.Error())
		}
	}
}

// TestLineSpacingRefusesAZeroResolvedAdvance is D-7.2.3's guardrail: the
// real lower-bound failure is not load-time checkable, because a
// load-time check cannot see the font size.
//
// geom.ScaleRound returns 0 whenever v*num < den/2 — measured:
// ScaleRound(400, 1, 1000) is 0 — so a small face at a small ratio yields
// zero-height lines, which layout cannot draw and the canvas correctly
// refuses. It is checked WHERE BOTH OPERANDS EXIST, as a DISTINCT
// condition from the load-time range with its own error. Raising the
// load-time minimum to prevent it would only move the blindness.
func TestLineSpacingRefusesAZeroResolvedAdvance(t *testing.T) {
	// units 400 at 1000 millipoints is a ruled advance of exactly 400 mp;
	// at the floor ratio, 400*1/1000 rounds to 0.
	metrics := []fontset.LineMetrics{{Ascent: 400, Descent: 0, LineGap: 0}}
	_, err := verticalModel([]string{"tinyface"}, metrics, geom.Length(1000), template.MinLineSpacingThousandths)
	if err == nil {
		t.Fatal("a resolved advance of zero must be a located error, not a zero-height line")
	}
	for _, want := range []string{"tinyface", "400", "line spacing 0.001 "} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error must name the element's chain, the RESOLVED size, and the ratio in the author's own units, missing %q in %q", want, err.Error())
		}
	}
	// It must NOT be the range code's condition: the value 1 is inside
	// the declared domain and was accepted at load.
	if err := template.ValidateLineSpacingThousandths(template.MinLineSpacingThousandths); err != nil {
		t.Fatalf("the floor ratio must be ACCEPTED at load — the zero-advance failure is a different condition at a different stage: %v", err)
	}
}

// TestNeutralRatioNeverRefusesADegeneracyItDidNotCause is P1's pin, and
// it is a BYTE-NEUTRALITY assertion wearing an error-path costume.
//
// The zero-advance guard must refuse only a zero THE RATIO CAUSED. A
// ruled advance that is already non-positive comes from an unbounded
// authored `fontSize` — DW-26, which this story deliberately leaves open
// — is reachable with no `lineSpacing` declared anywhere, and rendered
// fine at this story's baseline. A guard that refused it would make the
// NEUTRAL ratio start rejecting documents that carry no lineSpacing at
// all, which the contract's "absent produces byte-identical output to
// today" forbids outright.
//
// Both halves are pinned, because either alone is satisfiable by a guard
// that is simply absent or simply unconditional.
func TestNeutralRatioNeverRefusesADegeneracyItDidNotCause(t *testing.T) {
	// (a) PASS THROUGH: the ruled advance is already zero (font size 0),
	//     and no lineSpacing is declared. This must behave exactly as it
	//     did before the ratio existed.
	vm, err := verticalModel([]string{"probe"}, lineSpacingProbeMetrics, 0, defaultLineSpacing)
	if err != nil {
		t.Fatalf("the neutral ratio refused a degeneracy it did not cause: %v\n\nA document declaring no lineSpacing must render exactly as it did at this story's baseline; a zero ruled advance is DW-26's unbounded fontSize, not this story's to refuse.", err)
	}
	if vm.Advance != 0 {
		t.Errorf("Advance = %d, want the unscaled model's own 0 handed back untouched", vm.Advance)
	}

	// (b) STILL REFUSED: a POSITIVE ruled advance driven to zero BY the
	//     ratio. Without this half, (a) would be satisfied by deleting
	//     the guard entirely.
	if _, err := verticalModel([]string{"tinyface"}, []fontset.LineMetrics{{Ascent: 400}}, geom.Length(1000), template.MinLineSpacingThousandths); err == nil {
		t.Error("a positive ruled advance driven to zero BY the ratio must still be refused — that IS the condition this guard exists for")
	}

	// (c) And the whole existing corpus still renders, which is the only
	//     assertion that can catch a guard whose predicate is subtly
	//     wrong in a direction (a) and (b) do not probe.
	for _, doc := range []string{mandatoryBreakTemplateJSON, lineSpacingTemplateJSON} {
		if _, err := ParseTemplate([]byte(doc)); err != nil {
			t.Errorf("a committed fixture stopped parsing: %v", err)
		}
	}
}

// TestRenderTimeLeadingErrorsNameTheElement is P2's pin: the AC requires
// "a located error names the element and the resolved size", and the
// element id is contributed ONLY by the callers' wraps — verticalModel
// itself knows the chain and the size but not which element declared
// them. Asserted through the PUBLIC parse+render path, at BOTH the text
// element site and the table site, because deleting either wrap leaves
// every seam-level test in this file green.
func TestRenderTimeLeadingErrorsNameTheElement(t *testing.T) {
	// A font size whose ruled advance times the ceiling ratio overflows
	// int64: 1362 units at 10,000,000,000,000 millipoints is a ruled
	// advance of 13,620,000,000,000, and x1000000 is past int64's max.
	const hugeFontSize = `"fontSize": 10000000000, "lineSpacing": 1000`

	for _, c := range []struct {
		label string
		doc   string
	}{
		{"text element", lineSpacingHugeTextTemplate(hugeFontSize)},
		{"table header label", lineSpacingHugeTableTemplate(`"fontSize": 11`, hugeFontSize)},
		{"table body row", lineSpacingHugeTableTemplate(hugeFontSize, `"fontSize": 11`)},
	} {
		t.Run(c.label, func(t *testing.T) {
			tpl, err := ParseTemplate([]byte(c.doc))
			if err != nil {
				t.Fatalf("the document must LOAD — the range check passes, and the failure is a render-time one: %v", err)
			}
			_, err = Render(tpl, Data(`{"rows":[{"v":"x"}]}`), nil, testShippedFontSet())
			if err == nil {
				t.Fatal("an overflowing leading product must fail the render, not panic and not succeed")
			}
			if !strings.Contains(err.Error(), "element e1") {
				t.Errorf("the render error does not LOCATE the element (want \"element e1\"), got: %v", err)
			}
			if !strings.Contains(err.Error(), "overflow") {
				t.Errorf("the render error does not say what went wrong, got: %v", err)
			}
		})
	}

	// The same, for the OTHER render-time leading condition: a resolved
	// advance of zero must name the element and the resolved size too.
	//
	// The header-label case is here deliberately. A header label is always
	// ONE line, so its Advance never enters the drawn geometry, and it is
	// tempting to read the resulting refusal as over-strict and to pass
	// defaultLineSpacing at that site instead. That would be exactly the
	// carve-out D-7.1.3 forbids: one rule for one property, at every
	// caller. The refusal is the specified behaviour, so it is pinned
	// rather than left to be "fixed" later.
	for _, c := range []struct {
		label string
		doc   string
	}{
		{"text element", lineSpacingHugeTextTemplate(`"fontSize": 0.001, "lineSpacing": 0.001`)},
		{"table header label", lineSpacingHugeTableTemplate(`"fontSize": 11`, `"fontSize": 0.3, "lineSpacing": 0.001`)},
	} {
		t.Run("zero advance, "+c.label, func(t *testing.T) {
			tpl, err := ParseTemplate([]byte(c.doc))
			if err != nil {
				t.Fatalf("the document must LOAD — the range check passes: %v", err)
			}
			if _, err := Render(tpl, Data(`{"rows":[{"v":"x"}]}`), nil, testShippedFontSet()); err == nil {
				t.Error("a resolved advance of zero must fail the render")
			} else {
				if !strings.Contains(err.Error(), "element e1") {
					t.Errorf("the zero-advance render error does not LOCATE the element, got: %v", err)
				}
				if !strings.Contains(err.Error(), "resolves to an advance of 0") {
					t.Errorf("the zero-advance render error does not name the resolved size, got: %v", err)
				}
			}
		})
	}
}

// TestLineSpacingOutOfRangeIsALocatedCodedLoadError is the AC for the
// load path: below the floor, above the ceiling, and more than three
// decimal places are all refused, all located, and all carry
// TEMPLATE_FIELD_INVALID — never TEMPLATE_MALFORMED, whose message
// the WASM host replaces wholesale.
func TestLineSpacingOutOfRangeIsALocatedCodedLoadError(t *testing.T) {
	for _, c := range []struct {
		label string
		value string
		field string
		want  string
	}{
		{"zero", `"lineSpacing": 0`, "style", "style.lineSpacing"},
		{"negative", `"lineSpacing": -1`, "style", "style.lineSpacing"},
		{"above the ceiling", `"lineSpacing": 1000.001`, "style", "style.lineSpacing"},
		{"four decimal places", `"lineSpacing": 1.0005`, "style", "style.lineSpacing"},
		{"not a number", `"lineSpacing": "1.5"`, "style", "style.lineSpacing"},
		// null: encoding/json no-ops a JSON null into a json.Number, so
		// without an explicit refusal the author is told `invalid numeric
		// literal ""` — a message about the engine's parser rather than
		// about anything they wrote. Covered on BOTH attachment points.
		{"null", `"lineSpacing": null`, "style", "style.lineSpacing"},
		{"null on a headerStyle", `"lineSpacing": null`, "headerStyle", "headerStyle.lineSpacing"},
		{"on a headerStyle", `"lineSpacing": 0`, "headerStyle", "headerStyle.lineSpacing"},
	} {
		t.Run(c.label+" in "+c.field, func(t *testing.T) {
			_, err := ParseTemplate([]byte(lineSpacingRejectTemplate(c.field, c.value)))
			if err == nil {
				t.Fatal("expected a load error")
			}
			var re *RenderError
			if !errors.As(err, &re) {
				t.Fatalf("error is not transported as *RenderError: %T %v", err, err)
			}
			if re.Diagnostic.Code != DiagCodeTemplateFieldInvalid {
				t.Errorf("code = %q, want %q — an uncoded load error becomes TEMPLATE_MALFORMED and its message is destroyed before the author sees it", re.Diagnostic.Code, DiagCodeTemplateFieldInvalid)
			}
			if re.Diagnostic.Code == DiagCodeTemplateMalformed {
				t.Error("the code must NOT be TEMPLATE_MALFORMED")
			}
			if re.Diagnostic.Severity != SeverityError {
				t.Errorf("severity = %s, want Error", re.Diagnostic.Severity)
			}
			if re.Diagnostic.ElementID != "e1" {
				t.Errorf("the error must LOCATE the element, got ElementID %q", re.Diagnostic.ElementID)
			}
			// The FIELD locates which of the two attachment points it
			// was: a mistyped headerStyle value must not send the author
			// to its sibling `style` block.
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("the error must name %q, got %q", c.want, err.Error())
			}
			if strings.Contains(err.Error(), `invalid numeric literal ""`) {
				t.Errorf("the message describes the engine's own parser rather than what the author wrote: %q", err.Error())
			}
		})
	}
}

// TestLineSpacingErrorMessageSurvivesTheWasmReportingRule mirrors, in
// this package's own terms, the rule that actually decides whether AC7
// is delivered: wasm/cmd/engine's reportableMessage replaces a
// diagnostic's message with "The template could not be processed" for
// TEMPLATE_MALFORMED and for that code ALONE, and passes every other
// code's message through bounded at 512 bytes.
//
// The rule itself lives in a js/wasm-only main package, so it cannot be
// called from here. What is asserted here is the PROPERTY that rule turns
// on — the code is not TEMPLATE_MALFORMED, and the message is short
// enough to survive the bound intact — so a future change routing this
// condition back through the generic load-error path reddens on the Go
// side, where the tests actually run. The js/wasm side asserts the same
// end to end (wasm/cmd/engine/main_test.go).
func TestLineSpacingErrorMessageSurvivesTheWasmReportingRule(t *testing.T) {
	_, err := ParseTemplate([]byte(lineSpacingRejectTemplate("style", `"lineSpacing": 1000.001`)))
	if err == nil {
		t.Fatal("expected a load error")
	}
	var re *RenderError
	if !errors.As(err, &re) {
		t.Fatalf("not a *RenderError: %T", err)
	}
	if re.Diagnostic.Code == DiagCodeTemplateMalformed {
		t.Fatal("TEMPLATE_MALFORMED is the ONE code whose message the WASM host replaces — this condition must never carry it")
	}
	if n := len(re.Diagnostic.Message); n == 0 || n > 512 {
		t.Errorf("message is %d bytes; it must be non-empty and inside the host's 512-byte bound to reach the author intact", n)
	}
	if !strings.Contains(re.Diagnostic.Message, "e1") {
		t.Errorf("the message must name the element so the author can act on it, got %q", re.Diagnostic.Message)
	}
}

// TestLineSpacingIsRefusedIdenticallyByFileAndProperty is D-7.2.3's "one
// validation function": the property command must refuse a value for the
// SAME reason the loader does, and it must do so by calling the same
// function rather than by restating its bounds.
func TestLineSpacingIsRefusedIdenticallyByFileAndProperty(t *testing.T) {
	for _, value := range []string{"0", "-1", "1000.001", "1.0005"} {
		fileErr := func() string {
			_, err := ParseTemplate([]byte(lineSpacingRejectTemplate("style", `"lineSpacing": `+value)))
			if err == nil {
				t.Fatalf("%s: the file path accepted an out-of-domain value", value)
			}
			return err.Error()
		}()
		propErr := func() string {
			tpl := mustParseLineSpacingTemplate(t, "")
			_, err := applyComponentCommand(tpl, []byte(`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"lineSpacing":{"op":"set","value":`+value+`}}}`))
			if err == nil {
				t.Fatalf("%s: the property command accepted an out-of-domain value", value)
			}
			return err.Error()
		}()
		// The REASON is the shared half; the surrounding location is not.
		reason := lineSpacingReasonOf(t, value)
		if !strings.Contains(fileErr, reason) {
			t.Errorf("%s: the load error does not carry the shared reason %q: %s", value, reason, fileErr)
		}
		if !strings.Contains(propErr, reason) {
			t.Errorf("%s: the property command's error does not carry the shared reason %q: %s", value, reason, propErr)
		}
	}

	// And the positive control: a legal value is accepted through the
	// command and reaches the document.
	tpl := mustParseLineSpacingTemplate(t, "")
	if _, err := applyComponentCommand(tpl, []byte(`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"lineSpacing":{"op":"set","value":1.5}}}`)); err != nil {
		t.Fatalf("a legal lineSpacing must be accepted through the property command: %v", err)
	}
	out, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if !strings.Contains(string(out), `"lineSpacing": 1.5`) {
		t.Fatalf("the command's value did not reach the document:\n%s", out)
	}
	// And the document it wrote declares the version that value needs.
	if !strings.Contains(string(out), `"version": "1.1"`) {
		t.Fatalf("a document the inspector gave a lineSpacing must serialize declaring 1.1:\n%s", out)
	}
}

// TestLineSpacingClearedFromTheOnlyStyleFieldStrandsNoStyleBlock pins
// cleanupEmptyStyle's new arm: a style block whose ONLY field is
// lineSpacing must disappear entirely when it is cleared, exactly as one
// holding only a fontSize does.
func TestLineSpacingClearedFromTheOnlyStyleFieldStrandsNoStyleBlock(t *testing.T) {
	// A document whose e1 carries NO style block at all, so the one the
	// command creates holds lineSpacing and nothing else — which is the
	// only shape that can strand an empty style.
	tpl, err := ParseTemplate([]byte(lineSpacingUnstyledTemplate))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	set := []byte(`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"lineSpacing":{"op":"set","value":1.5}}}`)
	if _, err := applyComponentCommand(tpl, set); err != nil {
		t.Fatalf("set: %v", err)
	}
	clear := []byte(`{"kind":"updateComponentProperties","version":1,"ids":["e1"],"changes":{"lineSpacing":{"op":"clear"}}}`)
	if _, err := applyComponentCommand(tpl, clear); err != nil {
		t.Fatalf("clear: %v", err)
	}
	out, err := SerializeTemplate(tpl)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if strings.Contains(string(out), "lineSpacing") {
		t.Errorf("the cleared key survived:\n%s", out)
	}
	if strings.Contains(string(out), `"style"`) {
		t.Errorf("clearing the style block's only field stranded an empty style:\n%s", out)
	}
	// AND THE VERSION STAYS 1.1, WHICH IS NOT A BUG. The set command's own
	// transaction serialized and reparsed the document while it DID carry
	// a lineSpacing, so 1.1 became the document's loaded version — and
	// D-1.4.13 says lowered NEVER. A document that once legitimately
	// declared 1.1 does not become a 1.0 document because a key was
	// deleted from it, any more than a PDF 1.7 file becomes 1.4 when the
	// feature that needed 1.7 is removed. The "still declares 1.0" half of
	// the rule is asserted in TestVersionIsRaisedByContentAndNeverLowered,
	// on a document that never carried the key at all.
	if !strings.Contains(string(out), `"version": "1.1"`) {
		t.Errorf("a version raised while the document genuinely carried a 1.1 key must never be lowered afterwards:\n%s", out)
	}
}

// TestEmptyLineOccupiesOneFullScaledAdvance is Story 7.1's empty line met
// with Story 7.2's ratio: two consecutive breaks produce a line box that
// draws nothing, and it must occupy one full SCALED advance — not one
// ruled advance, and not nothing.
//
// The empty line draws no run, so it is observable in the artifact only
// as an INTERVAL: the element's two drawn baselines sit two scaled
// advances apart. textBlockHeight must count it too, because that is the
// number textValignOffset distributes an element's slack against — the
// quiet path to a wrong page break.
func TestEmptyLineOccupiesOneFullScaledAdvance(t *testing.T) {
	const ratio = 1500
	const scaledAdvance = lineSpacingOpenAdvanceMP

	tpl := mustParseLineSpacingTemplate(t, `"lineSpacing": 1.5, `)
	res, err := Render(tpl, Data(`{}`), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	ys := linesByOrigin(readEmittedRuns(t, res.Bytes))
	if len(ys) != 2 {
		t.Fatalf("the probe element draws %d baselines %v, want 2 (its middle line is empty and draws nothing)", len(ys), ys)
	}
	if gap := ys[0] - ys[1]; gap != 2*scaledAdvance {
		t.Errorf("the two drawn baselines are %d mp apart, want %d — the empty line must occupy one full SCALED advance, not a ruled one (%d) and not nothing", gap, 2*scaledAdvance, 2*lineSpacingRuledAdvanceMP)
	}

	// And the block height counts it: three lines, so two advances plus
	// the ascent and the descent.
	vm, err := chainVerticalModel([]string{"Noto Sans"}, geom.Length(11000), ratio, testShippedFontSet(), newFontCache())
	if err != nil {
		t.Fatalf("chainVerticalModel: %v", err)
	}
	want := geom.Length(2*scaledAdvance) + vm.FirstBaseline + vm.LastDescent
	if got := textBlockHeight(3, vm); got != want {
		t.Errorf("textBlockHeight(3) = %d, want %d — the empty line must be counted, or vertical alignment distributes slack against a height that is too small", got, want)
	}
}

// TestBlockHeightInheritsTheRatio is the anti-drift assertion for the
// THREE longhand copies of "a block of n lines is FirstBaseline +
// (n-1)*Advance + LastDescent" — text_alignment.go's helper, and
// table_render.go's data-row and footer-row expressions, neither of which
// reuses the helper.
//
// They inherit the ratio automatically BECAUSE it is applied inside the
// model rather than at any consumer. That is the argument for applying it
// there; it is not a reason to leave the inheritance unasserted.
func TestBlockHeightInheritsTheRatio(t *testing.T) {
	fs, cache := testShippedFontSet(), newFontCache()
	ruled, err := chainVerticalModel([]string{"Noto Sans"}, geom.Length(11000), defaultLineSpacing, fs, cache)
	if err != nil {
		t.Fatalf("ruled: %v", err)
	}
	open, err := chainVerticalModel([]string{"Noto Sans"}, geom.Length(11000), 1500, fs, newFontCache())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, lines := range []int{1, 2, 3, 7} {
		gotRuled, gotOpen := textBlockHeight(lines, ruled), textBlockHeight(lines, open)
		wantDelta := geom.Length(int64(lines-1)) * (open.Advance - ruled.Advance)
		if gotOpen-gotRuled != wantDelta {
			t.Errorf("textBlockHeight(%d) grew by %d, want exactly (%d-1) advances' worth (%d) — the height must inherit the ratio through Advance and through nothing else", lines, gotOpen-gotRuled, lines, wantDelta)
		}
	}
	if textBlockHeight(1, ruled) != textBlockHeight(1, open) {
		t.Error("a ONE-line block's height must not change with the ratio at all — it carries no Advance term")
	}
}

// TestValignReseatsTheTallerBlockAndOnlyThen is the composition the rest
// of this file leaves unasserted: lineSpacing × valign.
//
// Every other acceptance here uses the default valign (top), where the
// first baseline provably does not move. That made it easy to read the
// story's "a component's top edge stays where the author put it" as
// unconditional. It is not. textValignOffset seats the whole packed block
// inside the declared height, and the ratio makes that block taller, so
// for middle/bottom the DRAWN first baseline moves — by the full extra
// block height for bottom, and by half of it for middle.
//
// That is valign doing its job on a taller block, not the ratio reaching
// the first line: FirstBaseline itself is bit-identical in both models,
// which is asserted here alongside the movement so the two can never be
// confused again. Without this test, changing render.go to compute the
// block height from a RULED model would misplace every middle/bottom
// element under a ratio with nothing going red.
func TestValignReseatsTheTallerBlockAndOnlyThen(t *testing.T) {
	fs := testShippedFontSet()
	ruled, err := chainVerticalModel([]string{"Noto Sans"}, geom.Length(11000), defaultLineSpacing, fs, newFontCache())
	if err != nil {
		t.Fatalf("ruled: %v", err)
	}
	open, err := chainVerticalModel([]string{"Noto Sans"}, geom.Length(11000), 1500, fs, newFontCache())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if ruled.FirstBaseline != open.FirstBaseline || ruled.LastDescent != open.LastDescent {
		t.Fatalf("the ratio moved a span it must never touch: FirstBaseline %d->%d, LastDescent %d->%d", ruled.FirstBaseline, open.FirstBaseline, ruled.LastDescent, open.LastDescent)
	}

	const lines = 2
	boxHeight := geom.Length(100000)
	ruledBlock, openBlock := textBlockHeight(lines, ruled), textBlockHeight(lines, open)
	grew := openBlock - ruledBlock
	if grew <= 0 {
		t.Fatalf("the ratio must make a %d-line block taller, got a delta of %d", lines, grew)
	}

	for _, c := range []struct {
		valign string
		want   geom.Length
		why    string
	}{
		{"top", 0, "a top-seated block does not move at all, so the first baseline is exactly where the ruled model put it"},
		{"bottom", grew, "seating a taller block against the box bottom lifts its first baseline by the WHOLE extra height"},
		{"middle", geom.ScaleRound(grew, 1, 2), "centring splits the extra height, so the first baseline lifts by HALF of it"},
	} {
		t.Run(c.valign, func(t *testing.T) {
			ruledOffset := textValignOffset(c.valign, boxHeight, ruledBlock)
			openOffset := textValignOffset(c.valign, boxHeight, openBlock)
			// The offset is measured DOWN from the box top, so a smaller
			// offset under the ratio is a first baseline drawn higher.
			if got := ruledOffset - openOffset; got != c.want {
				t.Errorf("valign %q: the ratio moved the first baseline by %d, want %d — %s", c.valign, got, c.want, c.why)
			}
		})
	}
}

// TestEveryBlockHeightCopyReadsTheModelsAdvance is the source-level pin
// standing behind the arithmetic above, and it exists because ONE of the
// three copies is not artifact-observable.
//
// A table's FOOTER row is always one line — a footer value is a formatted
// aggregate with no break opportunity inside it — so its `(linesInRow-1)`
// term is always zero and no document can make its inheritance visible in
// the produced bytes. What CAN be asserted is that all three copies read
// the vertical model's own `vm.Advance` rather than deriving an advance
// of their own, which is the drift this story would otherwise be one
// refactor away from.
func TestEveryBlockHeightCopyReadsTheModelsAdvance(t *testing.T) {
	root := repoRootFromTest(t)
	for _, c := range []struct {
		file string
		want string
		what string
	}{
		{"text_alignment.go", "geom.Length(int64(lines-1))*vm.Advance + vm.FirstBaseline + vm.LastDescent", "textBlockHeight, the shared helper"},
		{"table_render.go", "rowHeight := padTopB + vm.FirstBaseline + geom.Length(int64(linesInRow-1))*vm.Advance + vm.LastDescent + padBottomB", "the data-row longhand copy"},
		{"table_render.go", "footerRowHeight := padTopB + vm.FirstBaseline + geom.Length(int64(linesInRow-1))*vm.Advance + vm.LastDescent + padBottomB", "the footer-row longhand copy"},
	} {
		b, err := os.ReadFile(filepath.Join(root, "folio8-go", c.file))
		if err != nil {
			t.Fatalf("read %s: %v", c.file, err)
		}
		if !strings.Contains(string(b), c.want) {
			t.Errorf("%s: %s no longer reads the vertical model's own Advance verbatim. Expected to find:\n\t%s\nA block height derived from anything but vm.Advance stops inheriting style.lineSpacing, and nothing else in this suite would notice.", c.file, c.what, c.want)
		}
	}
}

// TestCanvasProjectsTheSameAdvanceTheRendererUses is AC6 and the Story
// 5.9 invariant: the browser never measures text, so the canvas must
// consume the IDENTICAL advance the PDF producer does — ratio included.
func TestCanvasProjectsTheSameAdvanceTheRendererUses(t *testing.T) {
	for _, c := range []struct {
		label string
		style string
		want  int64
	}{
		{"absent", "", lineSpacingRuledAdvanceMP},
		{"widened", `"lineSpacing": 1.5, `, lineSpacingOpenAdvanceMP},
		{"tight", `"lineSpacing": 0.6, `, lineSpacingTightAdvanceMP},
	} {
		t.Run(c.label, func(t *testing.T) {
			tpl := mustParseLineSpacingTemplate(t, c.style)
			projection, err := canvasWithTextPaint(tpl, testShippedFontSet())
			if err != nil {
				t.Fatalf("CanvasWithTextPaint: %v", err)
			}
			var paint *designer.CanvasTextPaint
			for i := range projection.Components {
				if projection.Components[i].ID == "e1" {
					paint = projection.Components[i].TextPaint
				}
			}
			if paint == nil || len(paint.Lines) == 0 {
				t.Fatalf("the canvas projected no text paint for e1: %+v", paint)
			}
			for i, line := range paint.Lines {
				if line.Advance != c.want {
					t.Errorf("canvas line %d advances %d, want %d — the canvas and the PDF must consume the SAME advance", i, line.Advance, c.want)
				}
			}
			// The tight case is the one that used to be unprojectable at
			// all: the baseline sits below the next line's top.
			if c.label == "tight" && !(paint.Lines[0].Baseline > paint.Lines[0].Top+paint.Lines[0].Advance) {
				t.Errorf("the tight case no longer projects overlapping line boxes (top %d, baseline %d, advance %d), so it stops exercising what D-7.2.2 removed", paint.Lines[0].Top, paint.Lines[0].Baseline, paint.Lines[0].Advance)
			}
		})
	}
}

// TestHeaderStyleLineSpacingCascadesLikeFontSize pins D-7.1.3 at the one
// construction site whose cascade is not a direct read: headerStyle wins,
// then the table's own style, then the neutral default — exactly as
// fontSize resolves, and with no carve-out.
func TestHeaderStyleLineSpacingCascadesLikeFontSize(t *testing.T) {
	parse := func(t *testing.T, style, headerStyle string) template.Element {
		t.Helper()
		doc := lineSpacingTableTemplate(style, headerStyle)
		tpl, err := ParseTemplate([]byte(doc))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		return tpl.doc.Bands.Content.Elements[0]
	}
	for _, c := range []struct {
		label       string
		style       string
		headerStyle string
		want        int64
	}{
		{"neither", "", "", defaultLineSpacing},
		{"style only", `"lineSpacing": 1.5, `, "", 1500},
		{"headerStyle only", "", `"lineSpacing": 0.6, `, 600},
		{"headerStyle wins", `"lineSpacing": 1.5, `, `"lineSpacing": 0.6, `, 600},
	} {
		t.Run(c.label, func(t *testing.T) {
			got := resolveHeaderStyle(parse(t, c.style, c.headerStyle)).lineSpacing
			if got != c.want {
				t.Errorf("resolved header lineSpacing = %d, want %d", got, c.want)
			}
		})
	}
}

// TestVersionIsRaisedByContentAndNeverLowered is D-1.4.13's rule and
// D-7.2.1's retrofit, both halves.
func TestVersionIsRaisedByContentAndNeverLowered(t *testing.T) {
	for _, c := range []struct {
		label   string
		style   string
		loaded  string
		want    string
		because string
	}{
		// Every raise case loads 1.0, not 1.1. Loading 1.1 would let the
		// never-lower arm satisfy the assertion on its own, so the case
		// would pass even if styleNeedsMinorVersion returned false for
		// both keys — a test green for a reason its own name disclaims.
		{"neither key", "", "1.0", "1.0", "a document using no 1.1 key must keep declaring 1.0, never the library's ceiling"},
		{"lineSpacing", `"lineSpacing": 1.5, `, "1.0", "1.1", "lineSpacing is a 1.1 key"},
		{"color, retrofitted", `"color": "#1B2A4A", `, "1.0", "1.1", "Epic 10 shipped style.color and bumped nothing; documents using it declared 1.0 while requiring 1.1"},
		{"both", `"color": "#1B2A4A", "lineSpacing": 1.5, `, "1.0", "1.1", "the two coexist in one MINOR"},
	} {
		t.Run(c.label, func(t *testing.T) {
			doc := strings.Replace(lineSpacingProbeTemplate(c.style), `"version": "1.0"`, `"version": "`+c.loaded+`"`, 1)
			d, err := template.ParseDocument([]byte(doc))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			out, err := template.SerializeDocument(d)
			if err != nil {
				t.Fatalf("serialize: %v", err)
			}
			if !strings.Contains(string(out), `"version": "`+c.want+`"`) {
				t.Errorf("serialized version is not %s — %s:\n%s", c.want, c.because, out)
			}
		})
	}

	// NEVER LOWERED, in both directions that matters: a document already
	// declaring a HIGHER minor than its content needs keeps it, and one
	// declaring a higher minor than the library's own ceiling keeps that
	// too (its unknown content passes through opaquely).
	for _, loaded := range []string{"1.1", "1.4"} {
		doc := strings.Replace(lineSpacingProbeTemplate(""), `"version": "1.0"`, `"version": "`+loaded+`"`, 1)
		d, err := template.ParseDocument([]byte(doc))
		if err != nil {
			t.Fatalf("parse %s: %v", loaded, err)
		}
		out, err := template.SerializeDocument(d)
		if err != nil {
			t.Fatalf("serialize %s: %v", loaded, err)
		}
		if !strings.Contains(string(out), `"version": "`+loaded+`"`) {
			t.Errorf("version %s was lowered on save:\n%s", loaded, out)
		}
	}
}

// --- fixtures for the tests above ---------------------------------------

// lineSpacingProbeTemplate is one text element carrying a PARAGRAPH GAP
// (two consecutive line feeds -> three lines, the middle one empty) so
// one document serves both the empty-line assertion and the canvas one.
// style is spliced in verbatim, trailing comma included.
func lineSpacingProbeTemplate(style string) string {
	return `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 100, "value": "Clause 1.\n\nClause 2.", "style": {` + style + `"fontFamily": "body", "fontSize": 11}}
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

// lineSpacingUnstyledTemplate carries a text element with NO style block,
// so a property command that sets one field creates a style holding only
// that field.
const lineSpacingUnstyledTemplate = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 40, "value": "v"}
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

// lineSpacingHugeTextTemplate is one text element whose style is spliced
// in verbatim (no trailing comma), for the render-time leading guards.
func lineSpacingHugeTextTemplate(style string) string {
	return `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 40, "value": "v", "style": {"fontFamily": "body", ` + style + `}}
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

// lineSpacingHugeTableTemplate splices BOTH style blocks in verbatim (no
// trailing comma and no fixed fontSize of its own), so a caller can put
// an overflowing size on exactly one of the two and leave the other
// ordinary — which is what tells the header site's wrap apart from the
// body site's.
func lineSpacingHugeTableTemplate(style, headerStyle string) string {
	return `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "rows[]", "as": "row", "headerHeight": 20,
          "columns": [{"id": "e2", "label": "L", "width": 100, "bind": "{{row.v}}"}],
          "style": {"fontFamily": "body", ` + style + `},
          "headerStyle": {` + headerStyle + `}}
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
}

func mustParseLineSpacingTemplate(t *testing.T, style string) *Template {
	t.Helper()
	tpl, err := ParseTemplate([]byte(lineSpacingProbeTemplate(style)))
	if err != nil {
		t.Fatalf("parse probe template: %v", err)
	}
	return tpl
}

// lineSpacingRejectTemplate places raw at one of the two attachment
// points, so the same value can be shown to be refused, and located,
// at both.
func lineSpacingRejectTemplate(where, raw string) string {
	if where == "headerStyle" {
		return lineSpacingTableTemplate("", raw+", ")
	}
	return `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 40, "value": "v", "style": {` + raw + `, "fontFamily": "body", "fontSize": 11}}
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

// lineSpacingTableTemplate is one table element; style and headerStyle
// are spliced in verbatim, trailing comma included.
func lineSpacingTableTemplate(style, headerStyle string) string {
	return `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "rows[]", "as": "row", "headerHeight": 20,
          "columns": [{"id": "e2", "label": "L", "width": 100, "bind": "{{row.v}}"}],
          "style": {` + style + `"fontFamily": "body", "fontSize": 11},
          "headerStyle": {` + headerStyle + `"fontSize": 11}}
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
}

// lineSpacingReasonOf asks the shared validator itself what it says about
// value, so the comparison between the two paths is against the ONE
// function rather than against a string this test spells for itself.
func lineSpacingReasonOf(t *testing.T, value string) string {
	t.Helper()
	_, err := template.DecodeLineSpacing(value)
	if err == nil {
		t.Fatalf("the shared validator accepts %q, so there is no shared reason to compare", value)
	}
	return err.Error()
}

// TestCanvasProjectionCarriesTheDefaultLineSpacing is Story 17.3's Go half.
// The designer's line-spacing box used to spell the neutral ratio itself, as
// a hard-coded `'1'`, which made the browser a SECOND authority on a number
// this package owns. It now reads the projection, so the projection has to
// carry it — and carry the producer's own constant rather than a literal
// retyped beside it, which is why the want side of this comparison is
// `defaultLineSpacing` and not `1000`.
//
// The template below declares NO lineSpacing anywhere, which is the whole
// point: this is the number an element inherits when the document is silent.
func TestCanvasProjectionCarriesTheDefaultLineSpacing(t *testing.T) {
	tpl, err := ParseTemplate([]byte(`{"version":"1.0","page":{"size":"A4","orientation":"portrait","margin":{"top":36,"right":36,"bottom":36,"left":36}},"bands":{"pageHeader":{"height":20,"elements":[]},"content":{"elements":[{"id":"e1","type":"text","x":0,"y":0,"width":200,"height":40,"value":"Hello"}]},"pageFooter":{"height":20,"elements":[]}},"fonts":{"body":["Roboto-Regular"]},"locale":"en","utcOffset":"+00:00","assets":{},"nextId":2}`))
	if err != nil {
		t.Fatal(err)
	}
	projection, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if projection.DefaultLineSpacing != defaultLineSpacing {
		t.Errorf("default line spacing = %d, want the producer's own %d", projection.DefaultLineSpacing, defaultLineSpacing)
	}
	// And that constant is template.LineSpacingUnit, not a second spelling of
	// it: styleLineSpacing returns exactly this for an element with no style
	// block, so what the panel shows is what the measurement uses.
	if defaultLineSpacing != template.LineSpacingUnit {
		t.Errorf("defaultLineSpacing = %d, want template.LineSpacingUnit %d", defaultLineSpacing, template.LineSpacingUnit)
	}
	if len(projection.Components) != 1 {
		t.Fatalf("projected %d components, want 1", len(projection.Components))
	}
	// The element declares none, so the projection must not report one: the
	// panel's "unset" state is exactly this absence, and if the projection
	// filled it in the box would have nothing left to distinguish.
	if projection.Components[0].LineSpacing != nil {
		t.Errorf("component lineSpacing = %v, want absent for an element that declares none", *projection.Components[0].LineSpacing)
	}
	if got := styleLineSpacing(template.Presence[template.Style]{}); got != projection.DefaultLineSpacing {
		t.Errorf("styleLineSpacing on an absent style = %d, want the projected default %d", got, projection.DefaultLineSpacing)
	}
}
