package folio8

// Story 2.8's FR44 clip and diagnostic channel, exercised through the
// PUBLIC folio8.Render/folio8.RenderTo.
//
// AC1/AC2/AC3 are built on SYNTHETIC TEMPLATES for the same reason
// render_overflow_test.go's D-2.6.5 cases are: a committed golden that a
// human reads should look like a document a human would recognise, and
// the "declared width exceeded" pathology needs its OWN narrow/wide
// pair to prove "never reflowed, never dropped" — no committed fixture
// carries a wide-box control for the same text. `wrapped-text` e4
// (finding 4's real subject) is still exercised directly, once, so the
// synthetic cases are never the ONLY evidence FR44 fires on a real
// fixture (D-000.50).
//
// AC6/AC7 assert the clip's actual PDF bytes and the diagnostic
// channel's ordering/emptiness/agreement guarantees (D-2.8.6).

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// clipAtomicWord is finding 2's horizontal probe word, reused: a single
// 34-rune token with NO break opportunity anywhere inside it, so any
// box narrower than its shaped width overflows by construction — never
// by an unbreakableValues declaration, which wrapped-text/e4 already
// covers elsewhere. Two subjects, two reasons a run can be atomic
// (D-000.50).
const clipAtomicWord = "Supercalifragilisticexpialidocious"

// clipAtomicTemplate declares ONE text element holding clipAtomicWord at
// 12pt, in a box of the given width (points), followed by a SIBLING
// text element ("e2") whose own position must never move regardless of
// whether "e1" overflows (AD-24: "siblings never move, because nothing
// in a band ever reflows").
func clipAtomicTemplate(widthPt int) string {
	return fmt.Sprintf(`{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 20, "y": 40, "width": %d, "height": 40, "value": %q, "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "text", "x": 20, "y": 120, "width": 200, "height": 40, "value": "sibling", "style": {"fontFamily": "body", "fontSize": 12}}
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
`, widthPt, clipAtomicWord)
}

// clipNarrowTemplate is 20pt wide — far narrower than clipAtomicWord's
// shaped width at 12pt — so e1 overflows (finding 2's horizontal probe:
// identical word, a 20pt box painted all 34 glyphs before this story).
var clipNarrowTemplate = clipAtomicTemplate(20)

// clipWideTemplate is the NEGATIVE CONTROL (D-000.34/D-000.36): the
// SAME word in a box wide enough to hold it on one line with room to
// spare — 600pt, finding 2's own control width. Nothing about e1's
// glyphs, line count or break positions may differ between this
// template's render and clipNarrowTemplate's, because AD-24 forbids
// content ever reflowing on account of its own overflow.
var clipWideTemplate = clipAtomicTemplate(600)

func renderClipTemplate(t *testing.T, tplJSON string) Result {
	t.Helper()
	tpl, err := ParseTemplate([]byte(tplJSON))
	if err != nil {
		t.Fatalf("presence precondition: the synthetic template does not parse: %v", err)
	}
	res, err := Render(tpl, Data("{}"), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	return res
}

// TestWidthOverflowDetectedPerElement is AC1: a per-element record
// naming the element id, carrying the axis, the declared bound and the
// measured extent — asserted on the FIELDS, never on "something was
// detected" (D-000.21, D-2.6.5's guardrail applied one story on).
//
// Subject population (D-000.50): wrapped-text/e4 (a real, committed
// subject) AND a synthetic Latin atomic token — a single-subject
// assertion here would tie the property to the Thai dictionary path.
func TestWidthOverflowDetectedPerElement(t *testing.T) {
	t.Run("wrapped-text e4, the real committed subject", func(t *testing.T) {
		tpl, err := ParseTemplate([]byte(wrappedTextTemplateJSON))
		if err != nil {
			t.Fatalf("presence precondition: %v", err)
		}
		res, err := Render(tpl, Data(wrappedTextDataJSON), nil, testShippedFontSet())
		if err != nil {
			t.Fatalf("Render() error: %v", err)
		}
		if len(res.Bytes) == 0 {
			t.Fatal("AC1/AC6: the render must still succeed, returning bytes, exactly as before this story")
		}
		if len(res.Diagnostics) != 1 {
			t.Fatalf("want exactly 1 Diagnostic (only e4 overflows horizontally in this fixture, finding 4), got %d: %+v", len(res.Diagnostics), res.Diagnostics)
		}
		d := res.Diagnostics[0]
		// DW-18 RETIRED (Story 3.6, AC6): severityUnset now occupies the
		// zero value, so this check genuinely distinguishes "correctly
		// set to Warning" from "never set" — an omitted Severity field
		// would compare as severityUnset, not SeverityWarning, and this
		// assertion would (correctly) fail.
		if d.Severity != SeverityWarning {
			t.Errorf("Severity = %v, want SeverityWarning (AD-14: clipped content is a Warning, never fatal)", d.Severity)
		}
		if d.Code != DiagCodeTextClippedWidth {
			t.Errorf("Code = %q, want %q", d.Code, DiagCodeTextClippedWidth)
		}
		if d.ElementID != "e4" {
			t.Errorf("ElementID = %q, want %q — an UNLOCATED diagnostic is what AD-10 exists to prevent", d.ElementID, "e4")
		}
		if !strings.Contains(d.Message, "e4") {
			t.Errorf("Message %q does not name the element id", d.Message)
		}
	})

	t.Run("synthetic Latin atomic token, the negative-control pairing", func(t *testing.T) {
		res := renderClipTemplate(t, clipNarrowTemplate)
		if len(res.Diagnostics) != 1 {
			t.Fatalf("want exactly 1 Diagnostic, got %d: %+v", len(res.Diagnostics), res.Diagnostics)
		}
		d := res.Diagnostics[0]
		if d.ElementID != "e1" {
			t.Errorf("ElementID = %q, want %q", d.ElementID, "e1")
		}
		// DW-18 RETIRED (Story 3.6, AC6) — see the comment on the
		// equivalent check above; the Severity half is real coverage now.
		if d.Code != DiagCodeTextClippedWidth || d.Severity != SeverityWarning {
			t.Errorf("got Code=%q Severity=%v, want Code=%q Severity=Warning", d.Code, d.Severity, DiagCodeTextClippedWidth)
		}

		// The WIDE control must overflow NOTHING — this is the vacuity
		// guard finding 2 calls out by name: without it, a detector that
		// fired unconditionally would still pass the row above.
		wide := renderClipTemplate(t, clipWideTemplate)
		if len(wide.Diagnostics) != 0 {
			t.Fatalf("negative control: the WIDE box must overflow nothing, got %d Diagnostic(s): %+v", len(wide.Diagnostics), wide.Diagnostics)
		}
	})
}

// TestDetectWidthOverflowRecordsTheBoundAndExtent is AC1's internal
// half: the axis, the declared bound and the measured extent — fields
// the public Diagnostic (AD-14's closed five-field shape) does not
// carry, but which the record driving BOTH the diagnostic and the clip
// decision must, or "clip at the box boundary" has nothing to clip
// against.
func TestDetectWidthOverflowRecordsTheBoundAndExtent(t *testing.T) {
	lines := []wrappedLine{{from: 0, to: 5, width: 24585}}
	got, overflows := detectWidthOverflow("e4", lines, 20000)
	if !overflows {
		t.Fatal("want overflow, got none")
	}
	if got.elementID != "e4" || got.declaredWidth != 20000 || got.measuredWidth != 24585 {
		t.Errorf("got %+v, want {elementID:e4 declaredWidth:20000 measuredWidth:24585}", got)
	}

	// Negative control: a line that fits exactly must not overflow —
	// "<=" not "<", or a box measured to the millipoint would clip
	// content that fits it exactly.
	if _, overflows := detectWidthOverflow("e5", []wrappedLine{{width: 20000}}, 20000); overflows {
		t.Error("a line exactly as wide as the box must NOT overflow")
	}

	// No declared width (boxWidth == 0, collectBandTextRuns' own
	// convention) never overflows — there is no bound to measure
	// against.
	if _, overflows := detectWidthOverflow("e6", []wrappedLine{{width: 999999}}, 0); overflows {
		t.Error("an element with no declared width must never overflow")
	}
}

// TestOverflowingContentNeverReflowedNeverDropped is AC2, source AC1's
// third consequent in full: glyph sequence, line count and break
// positions identical to a wide-box render of the SAME text, and every
// sibling's position byte-for-byte unchanged.
//
// Both halves are required (finding 2): an assertion that the glyphs
// survive, alone, is satisfied by the code as it stood BEFORE this
// story and would ship as a tautology.
func TestOverflowingContentNeverReflowedNeverDropped(t *testing.T) {
	narrow := renderClipTemplate(t, clipNarrowTemplate)
	wide := renderClipTemplate(t, clipWideTemplate)

	narrowRuns := readEmittedRuns(t, narrow.Bytes)
	wideRuns := readEmittedRuns(t, wide.Bytes)

	narrowE1 := runsForResource(narrowRuns)
	wideE1 := runsForResource(wideRuns)

	if len(narrowE1) != 1 || len(wideE1) != 1 {
		t.Fatalf("presence precondition: want exactly 1 BT..ET run for e1 in each render (one line, one face), got narrow=%d wide=%d", len(narrowE1), len(wideE1))
	}
	if !reflect.DeepEqual(narrowE1[0].CIDs, wideE1[0].CIDs) {
		t.Errorf("glyph sequence differs between the overflowing (narrow-box) and non-overflowing (wide-box) renders of the SAME text:\n narrow: %v\n wide:   %v\nContent must never be re-broken or substituted because its own box is too small.", narrowE1[0].CIDs, wideE1[0].CIDs)
	}
	if narrowE1[0].OriginXMilli != wideE1[0].OriginXMilli || narrowE1[0].OriginYMilli != wideE1[0].OriginYMilli {
		t.Errorf("e1's own origin moved between renders: narrow=(%d,%d) wide=(%d,%d) — clipping is about paint, never about position",
			narrowE1[0].OriginXMilli, narrowE1[0].OriginYMilli, wideE1[0].OriginXMilli, wideE1[0].OriginYMilli)
	}

	// The SIBLING (e2) must be byte-for-byte unchanged: same origin,
	// same CIDs, in both renders — AD-24's "siblings never move, because
	// nothing in a band ever reflows", now checked against an element
	// that actually overflows rather than merely asserted in prose.
	narrowE2 := lastRun(narrowRuns)
	wideE2 := lastRun(wideRuns)
	if narrowE2.OriginXMilli != wideE2.OriginXMilli || narrowE2.OriginYMilli != wideE2.OriginYMilli {
		t.Errorf("sibling e2 moved: narrow=(%d,%d) wide=(%d,%d)", narrowE2.OriginXMilli, narrowE2.OriginYMilli, wideE2.OriginXMilli, wideE2.OriginYMilli)
	}
	if !reflect.DeepEqual(narrowE2.CIDs, wideE2.CIDs) {
		t.Errorf("sibling e2's glyphs differ between renders — nothing about e1 overflowing may touch e2 at all")
	}
}

// runsForResource returns every run but the last (e1's own runs; e2 is
// always emitted last since it is the second and final element in
// clipAtomicTemplate's authored order).
func runsForResource(runs []emittedRun) []emittedRun {
	if len(runs) == 0 {
		return nil
	}
	return runs[:len(runs)-1]
}

func lastRun(runs []emittedRun) emittedRun {
	return runs[len(runs)-1]
}

// clipHeightVariantTemplate is AC3's forward guard subject: ONE text
// element whose declared height is the only thing that varies between
// two calls. singleLine models shaped-text's own geometry (16pt,
// height:24 — D-2.8.1's finding 5: 7.3% UNDER the ruled line box, which
// is exactly the shape that WOULD visibly clip descenders if height
// ever became a clip bound) with wide/narrow box options so both a
// single-line and a multi-line element are covered by one template.
func clipHeightVariantTemplate(height, width int, singleLine bool) string {
	value := "wraps across more than one line because the box is deliberately narrow"
	if singleLine {
		value = "one line"
	}
	return fmt.Sprintf(`{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 20, "y": 40, "width": %d, "height": %d, "value": %q, "style": {"fontFamily": "body", "fontSize": 16}}
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
`, width, height, value)
}

// TestDeclaredHeightIsInert is AC3: the OD-2 fence, made mechanical. A
// text element's declared height must remain read by NOTHING — changing
// ONLY it must change NO emitted byte. Keyed on the PURPOSE ("height
// does not affect output"), not on the proxy "no code reads el.Height",
// which a future refactor could satisfy while breaking the property
// (D-000.15).
//
// Subjects: shaped-text-shaped geometry (single line, 16pt, height:24 —
// the exact case that WOULD visibly clip descenders under a vertical
// clip bound, finding 5) and a genuinely multi-line element.
func TestDeclaredHeightIsInert(t *testing.T) {
	for _, row := range []struct {
		name       string
		width      int
		singleLine bool
	}{
		{"single-line, shaped-text geometry", 200, true},
		{"multi-line, wraps across several lines", 90, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			for _, heights := range [][2]int{
				{24, 999999}, // finding 5's under-declared box vs an absurdly tall one
				{24, 1},      // vs an absurdly SHORT one — would clip everything if height ever bound
				{1000, 5},
			} {
				a := renderClipTemplate(t, clipHeightVariantTemplate(heights[0], row.width, row.singleLine))
				b := renderClipTemplate(t, clipHeightVariantTemplate(heights[1], row.width, row.singleLine))
				if !bytes.Equal(a.Bytes, b.Bytes) {
					t.Errorf("height %d vs height %d: rendered bytes differ (%d vs %d bytes) — a text element's declared height must be inert (D-2.8.1); nothing about it may reach emitted output",
						heights[0], heights[1], len(a.Bytes), len(b.Bytes))
				}
				if !reflect.DeepEqual(a.Diagnostics, b.Diagnostics) {
					t.Errorf("height %d vs height %d: Diagnostics differ (%+v vs %+v) — height must not influence the WIDTH-only overflow diagnostic either",
						heights[0], heights[1], a.Diagnostics, b.Diagnostics)
				}
			}
		})
	}
}

// TestClipRestrictsOnlyTheHorizontalAxis is AC6: the emitted PDF
// content stream actually clips the overflowing element's box on X, and
// the clip's Y extent is never derived from the element's own declared
// height (D-2.8.1) — read back off the PRODUCED bytes (D-000.21), not
// off renderer internals.
func TestClipRestrictsOnlyTheHorizontalAxis(t *testing.T) {
	res := renderClipTemplate(t, clipNarrowTemplate)
	s := string(res.Bytes)

	// The clip rectangle: "<marginLeft+x> 0 <width> <pageHeight> re\nW n\n".
	// margin.left 36pt + e1.x 20pt = 56; e1's declared width is 20pt;
	// A4 height is 841.89pt exactly (pageHeightA4 = 841890 mp).
	wantRe := "56 0 20 841.89 re\nW n\n"
	if !strings.Contains(s, wantRe) {
		t.Fatalf("content stream does not contain the expected clip rectangle %q — either the clip fired at the wrong box, or FR44's clip did not fire at all.\nStream:\n%s", wantRe, s)
	}
	if got := strings.Count(s, "W n\n"); got != 1 {
		t.Errorf("W n (clip) appears %d times, want exactly 1 — only the ONE overflowing element (e1) may be clipped; e2 (the sibling) fits its box and must not be", got)
	}
	if got := strings.Count(s, "\nq\n"); got != 1 {
		t.Errorf("q (clip save) appears %d times, want exactly 1", got)
	}

	// e2 (the non-overflowing sibling) never appears inside a clip: the
	// clip rectangle string above is scoped to e1's own box width (20),
	// which is narrower than e2's (200) — a second "re\nW n" using e2's
	// width would prove the clip leaked to an element that fits.
	if strings.Contains(s, "200 841.89 re\nW n\n") {
		t.Error("a clip rectangle using e2's box width was found — the sibling that fits its box must never be clipped")
	}
}

// TestRenderAndRenderToDiagnosticsAgree is D-2.8.6: the two entry
// points must agree exactly on a clipping document's Diagnostics, and
// one side is pinned against a literal so the pair cannot both drift
// together undetected.
func TestRenderAndRenderToDiagnosticsAgree(t *testing.T) {
	tpl, err := ParseTemplate([]byte(clipNarrowTemplate))
	if err != nil {
		t.Fatalf("presence precondition: %v", err)
	}

	res, err := Render(tpl, Data("{}"), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	var buf bytes.Buffer
	toDiags, terr := RenderTo(&buf, tpl, Data("{}"), nil, testShippedFontSet())
	if terr != nil {
		t.Fatalf("RenderTo() error: %v", terr)
	}

	if !reflect.DeepEqual(res.Diagnostics, toDiags) {
		t.Fatalf("Render's Diagnostics and RenderTo's returned Diagnostics disagree:\n Render:   %+v\n RenderTo: %+v", res.Diagnostics, toDiags)
	}

	// Presence precondition (Story 2.8 review Finding 3): the equality
	// check above cannot tell us res.Diagnostics is non-empty — if the
	// attach were suppressed, BOTH res.Diagnostics and toDiags would be
	// nil/empty together, DeepEqual would still be true, and indexing
	// [0] below would panic and black out the rest of this package's
	// test run (three other tests never got a PASS/FAIL). Fail cleanly
	// instead.
	if len(res.Diagnostics) != 1 {
		t.Fatalf("presence precondition: want exactly 1 Diagnostic, got %d: %+v", len(res.Diagnostics), res.Diagnostics)
	}

	// Pinned literal (D-2.8.6): if BOTH producers drifted together — for
	// example the DiagCodeTextClippedWidth constant's WIRE VALUE were
	// changed — a comparison built from that same constant would drift
	// right along with it and the equality check above would still
	// pass. Code is therefore pinned against the bare wire string, not
	// the constant, so a changed constant value is exactly what this
	// catches (Story 2.8 review Finding 1: comparing the constant
	// against itself caught nothing). Severity below is real coverage
	// as of Story 3.6/AC6 (DW-18 retired): severityUnset now occupies
	// the zero value, so an omitted Severity field would fail this
	// reflect.DeepEqual, not pass it silently.
	want := []Diagnostic{{
		Severity:  SeverityWarning,
		Code:      "TEXT_CLIPPED_WIDTH",
		ElementID: "e1",
		Message:   res.Diagnostics[0].Message, // message prose is not the pinned part; the other three fields are
	}}
	if !reflect.DeepEqual(res.Diagnostics, want) {
		t.Errorf("Render's Diagnostics = %+v, want %+v", res.Diagnostics, want)
	}

	// RenderTo's bytes must equal Render's bytes exactly — both entry
	// points render the SAME document through the SAME shared core.
	if !bytes.Equal(buf.Bytes(), res.Bytes) {
		t.Errorf("RenderTo wrote %d bytes, Render returned %d bytes, and they differ", buf.Len(), len(res.Bytes))
	}
}

// TestRenderToWritesCompleteBytesDespiteWarning is D-2.8.6's red-proof
// for the early-return fix: a Warning is not an error, so RenderTo must
// proceed, write COMPLETE bytes, and still return the warning — closing
// the measured breakage where a document that rendered successfully
// with something to report produced NO OUTPUT AT ALL.
func TestRenderToWritesCompleteBytesDespiteWarning(t *testing.T) {
	tpl, err := ParseTemplate([]byte(clipNarrowTemplate))
	if err != nil {
		t.Fatalf("presence precondition: %v", err)
	}

	want := renderClipTemplate(t, clipNarrowTemplate) // the Render-path answer, independently
	if len(want.Diagnostics) == 0 {
		t.Fatal("presence precondition: this template must produce at least one Diagnostic, or this test proves nothing")
	}

	var buf bytes.Buffer
	diags, terr := RenderTo(&buf, tpl, Data("{}"), nil, testShippedFontSet())
	if terr != nil {
		t.Fatalf("RenderTo() error: %v — a Warning must never surface as a non-nil error", terr)
	}
	if buf.Len() == 0 {
		t.Fatal("RenderTo wrote ZERO bytes for a document that rendered successfully with a warning to report — this is exactly the breakage D-2.8.6 closes")
	}
	if !bytes.Equal(buf.Bytes(), want.Bytes) {
		t.Errorf("RenderTo wrote %d bytes; Render produced %d bytes for the identical document — they must be identical", buf.Len(), len(want.Bytes))
	}
	if len(diags) == 0 {
		t.Error("RenderTo returned zero Diagnostics for a document that Render reports warnings for")
	}
}

// TestDiagnosticsOrderIsDocumentOrder is D-2.8.6's first determinism
// rule: band order (page header, then content, then page footer), and
// within one band, element DECLARATION order — never map order, and
// never the order collectBandTextRuns happens to be CALLED in
// (renderDocument collects content BEFORE header/footer, D-2.7.3's
// Phase A/B split, but the band order for diagnostics is still
// header/content/footer).
func TestDiagnosticsOrderIsDocumentOrder(t *testing.T) {
	const tplJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "ec1", "type": "text", "x": 0, "y": 0, "width": 20, "height": 20, "value": "Supercalifragilisticexpialidocious", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "ec2", "type": "text", "x": 0, "y": 40, "width": 20, "height": 20, "value": "Antidisestablishmentarianism", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {"elements": [
      {"id": "ef1", "type": "text", "x": 0, "y": 0, "width": 20, "height": 20, "value": "Floccinaucinihilipilification", "style": {"fontFamily": "body", "fontSize": 12}}
    ], "height": 40},
    "pageHeader": {"elements": [
      {"id": "eh1", "type": "text", "x": 0, "y": 0, "width": 20, "height": 20, "value": "Pneumonoultramicroscopicsilicovolcanoconiosis", "style": {"fontFamily": "body", "fontSize": 12}}
    ], "height": 40}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 100000,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
	res := renderClipTemplate(t, tplJSON)
	var gotOrder []string
	for _, d := range res.Diagnostics {
		gotOrder = append(gotOrder, d.ElementID)
	}
	wantOrder := []string{"eh1", "ec1", "ec2", "ef1"}
	if !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Errorf("Diagnostics element order = %v, want %v (band order header/content/footer, declaration order within a band)", gotOrder, wantOrder)
	}
}

// TestDiagnosticsEmptyIsNil is D-2.8.6's second determinism rule: a
// render that produces no Diagnostic sets Diagnostics to nil, not a
// non-nil empty slice — otherwise two renders of one document differ
// under reflect.DeepEqual while producing byte-identical PDFs.
func TestDiagnosticsEmptyIsNil(t *testing.T) {
	res := renderClipTemplate(t, clipWideTemplate)
	if res.Diagnostics != nil {
		t.Fatalf("Diagnostics = %#v, want nil (not merely len() == 0) for a render with nothing to report", res.Diagnostics)
	}
}
