package folio8

import (
	"errors"
	"strings"
	"testing"
)

// spec-deferred-offline-cache CAP-7, and this file exists because the
// STORY IS A FORK, not a new diagnostic.
//
// Two conditions used to arrive at one place — `found == false` in
// shapeSegments — and leave it as one TEXT_MISSING_GLYPH Warning with
// the rune dropped:
//
//	rune uncovered  +  some chain member was ABSENT   -> refuse, naming it
//	rune uncovered  +  every chain member was PRESENT -> the Warning
//
// The engine cannot ask whether the absent face would have covered the
// rune, which is exactly why the absence must be reported rather than
// assumed harmless: the second arm KNOWS the document's chain is
// incomplete, the first only knows it was not given everything the
// document asked for.
//
// EVERY ROW BELOW CARRIES A NEGATIVE CONTROL, and that is the whole
// design of this file rather than a habit. A test that only asserted the
// refusal would still pass if the fork collapsed the other way — if
// EVERY uncovered rune refused, failing legitimately unsupported text —
// and a test that only asserted the Warning would pass if the refusal
// were deleted. Each row therefore pins BOTH sides of the one condition
// it names.
//
// The documents are synthetic and built here. None of the committed
// golden fixtures may reach this path — they all supply the full font
// set — and TestNoCommittedFixtureReachesTheAbsentFaceRefusal at the
// bottom of this file is the direct statement of that claim rather than
// an inference from the suite being green.

// faceAbsentDoc builds a one-element document whose chain and text are
// the two variables every row of the matrix turns on.
func faceAbsentDoc(chain, text string) string {
	return `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 500, "height": 40, "value": ` + text + `, "style": {"fontFamily": "body", "fontSize": 14}}
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
  "version": "1.0"
}
`
}

// faceAbsentBoldDoc is faceAbsentDoc with `style.bold` on the element, so
// the chain's declared variant is actually requested.
func faceAbsentBoldDoc(chain, text string) string {
	return strings.Replace(faceAbsentDoc(chain, text), `"fontSize": 14`, `"fontSize": 14, "bold": true`, 1)
}

// fontSetWithout is testShippedFontSet() minus the named faces — the
// partial `FontSet` a folio-js or folio-dotnet caller can supply today
// and the designer's wasm host will supply once the CJK face is
// deferred. It is built by SUBTRACTION from the shipped set so that a
// face added to that set cannot silently stop being supplied here.
func fontSetWithout(t *testing.T, names ...string) FontSet {
	t.Helper()
	fs := testShippedFontSet()
	for _, n := range names {
		if _, ok := fs[n]; !ok {
			t.Fatalf("precondition: %q is not in the shipped set, so removing it withholds nothing", n)
		}
		delete(fs, n)
	}
	return fs
}

// renderFaceAbsentDoc parses and renders source, returning whatever came
// back. It asserts nothing: every row of the matrix asserts a DIFFERENT
// outcome over the same two calls.
func renderFaceAbsentDoc(t *testing.T, source string, fs FontSet) (Result, error) {
	t.Helper()
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("a document naming an unsupplied face must still LOAD — the supply question belongs to Render: %v", err)
	}
	return Render(tpl, Data(`{}`), nil, fs)
}

// assertFaceAbsentRefusal is the full shape of the refusal, asserted in
// one place: the coded error, its location, the named face, and NO PDF.
func assertFaceAbsentRefusal(t *testing.T, res Result, err error, face string) {
	t.Helper()
	if err == nil {
		t.Fatalf("the render SUCCEEDED where a face the chain declares was never supplied — that is the silently-missing-text outcome byte-identity exists to prevent; diagnostics: %+v", res.Diagnostics)
	}
	if len(res.Bytes) != 0 {
		t.Errorf("a refused render returned %d PDF bytes", len(res.Bytes))
	}
	var refusal *RenderError
	if !errors.As(err, &refusal) {
		t.Fatalf("the refusal must reach the caller as a *RenderError carrying a code: %T %v", err, err)
	}
	d := refusal.Diagnostic
	if d.Code != DiagCodeTextFaceAbsent {
		t.Errorf("code = %q, want %q (never TEXT_MISSING_GLYPH, which means something else); error = %v", d.Code, DiagCodeTextFaceAbsent, err)
	}
	if d.Severity != SeverityError {
		t.Errorf("severity = %s, want Error", d.Severity)
	}
	if d.ElementID != "e1" {
		t.Errorf("ElementID = %q, want e1 — the refusal must be located at the element", d.ElementID)
	}
	if d.DataPath != "style.fontFamily" {
		t.Errorf("DataPath = %q, want style.fontFamily — the field the chain was resolved from", d.DataPath)
	}
	if want := `face "` + face + `" is not present in the supplied FontSet`; !strings.Contains(d.Message, want) {
		t.Errorf("the message does not name the ABSENT FACE (%q missing), which is the one thing that tells the caller what to supply:\n\t%s", want, d.Message)
	}
}

// assertRendersClean is the negative control every refusal row needs: the
// same document, the same text, nothing withheld.
func assertRendersClean(t *testing.T, res Result, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("the control render failed, so the row above proves nothing about WHY the other one did: %v", err)
	}
	if len(res.Bytes) == 0 {
		t.Error("the control render produced no PDF bytes")
	}
	if len(res.Diagnostics) != 0 {
		t.Errorf("the control render produced diagnostics: %+v", res.Diagnostics)
	}
}

// TestAbsentChainMemberAnotherEntryCoversRendersSilently is matrix row 1,
// and it is the row that stops this story from being a blunt "refuse
// whenever anything is missing".
//
// The chain names a face the caller did not supply, and the text is
// entirely covered by the entry that WAS supplied. The absent member's
// work was done by another entry, so the document is not short of
// anything: it renders, with NO diagnostic at all — the format's
// standing tolerance of an absent chain member, unchanged.
func TestAbsentChainMemberAnotherEntryCoversRendersSilently(t *testing.T) {
	source := faceAbsentDoc(`["Noto Sans", "Noto Sans SC"]`, `"Ada"`)

	res, err := renderFaceAbsentDoc(t, source, fontSetWithout(t, "Noto Sans SC"))
	assertRendersClean(t, res, err)

	// THE NEGATIVE CONTROL IS THE OTHER DIRECTION HERE, because the row
	// itself is the clean one: the SAME chain and the SAME withheld face
	// with text the present entry cannot cover must refuse. Without this
	// the test above would pass against an engine that had simply lost
	// the refusal.
	han, hanErr := renderFaceAbsentDoc(t, faceAbsentDoc(`["Noto Sans", "Noto Sans SC"]`, `"汉"`), fontSetWithout(t, "Noto Sans SC"))
	assertFaceAbsentRefusal(t, han, hanErr, "Noto Sans SC")
}

// TestAbsentChainMemberNoEntryCoversRefuses is matrix row 2 — the
// story's central case, and the one a folio-js or folio-dotnet caller
// with a partial font set reaches today.
//
// Before CAP-7 this shipped a PDF with the Han text silently gone under
// a TEXT_MISSING_GLYPH Warning.
func TestAbsentChainMemberNoEntryCoversRefuses(t *testing.T) {
	source := faceAbsentDoc(`["Noto Sans", "Noto Sans SC"]`, `"汉"`)

	res, err := renderFaceAbsentDoc(t, source, fontSetWithout(t, "Noto Sans SC"))
	assertFaceAbsentRefusal(t, res, err, "Noto Sans SC")

	// THE NEGATIVE CONTROL: the same document, the same text, the face
	// supplied. If this ever fails the refusal above is firing on the
	// chain rather than on the absence.
	control, cerr := renderFaceAbsentDoc(t, source, testShippedFontSet())
	assertRendersClean(t, control, cerr)
}

// TestChainWithNoPresentMemberRefusesWithTheSameCode is matrix row 3: the
// same fault with nothing left to fall back to.
//
// It reaches the refusal at the FIRST RUNE, in shapeSegments, rather than
// one level up in the vertical model where it used to land — and it
// carries the same code either way, which is the point of coding both:
// an author who omitted the only face and one who omitted one of several
// read one diagnostic, not two.
func TestChainWithNoPresentMemberRefusesWithTheSameCode(t *testing.T) {
	source := faceAbsentDoc(`["Noto Sans SC"]`, `"汉"`)

	res, err := renderFaceAbsentDoc(t, source, fontSetWithout(t, "Noto Sans SC"))
	assertFaceAbsentRefusal(t, res, err, "Noto Sans SC")

	control, cerr := renderFaceAbsentDoc(t, source, testShippedFontSet())
	assertRendersClean(t, control, cerr)
}

// TestEveryDeclaredFacePresentKeepsTheMissingGlyphWarning is matrix row
// 4, and it is the arm that must NOT change.
//
// Every face the chain declares was supplied, and none of them draws the
// rune. The engine KNOWS the document's chain is incomplete — there is
// nothing it was not given — so the rune is dropped, the Warning is the
// sole record, and a PDF ships. Refusing this would fail legitimately
// unsupported text, which is the failure mode the fork exists to avoid.
func TestEveryDeclaredFacePresentKeepsTheMissingGlyphWarning(t *testing.T) {
	// A Latin-only chain, fully supplied, against a Thai rune.
	source := faceAbsentDoc(`["Noto Sans"]`, `"ก"`)

	res, err := renderFaceAbsentDoc(t, source, testShippedFontSet())
	if err != nil {
		t.Fatalf("a rune covered by no SUPPLIED face must stay a Warning, not become the CAP-7 refusal: %v", err)
	}
	if len(res.Bytes) == 0 {
		t.Error("the render produced no PDF bytes, so the rune was not merely dropped")
	}
	if len(res.Diagnostics) != 1 {
		t.Fatalf("want exactly one diagnostic, got %+v", res.Diagnostics)
	}
	d := res.Diagnostics[0]
	if d.Code != DiagCodeTextMissingGlyph {
		t.Errorf("code = %q, want %q — the two conditions must stay distinguishable", d.Code, DiagCodeTextMissingGlyph)
	}
	if d.Severity != SeverityWarning {
		t.Errorf("severity = %s, want Warning", d.Severity)
	}

	// THE NEGATIVE CONTROL, and it is the sharpest one in the file: the
	// SAME TEXT and the SAME UNCOVERED RUNE, differing only in whether a
	// declared face was supplied. Row 4 and row 2 are one condition apart,
	// and this pins that the condition is the supply, not the rune.
	chainWithThai := faceAbsentDoc(`["Noto Sans", "Noto Sans Thai"]`, `"ก"`)
	refused, rerr := renderFaceAbsentDoc(t, chainWithThai, fontSetWithout(t, "Noto Sans Thai"))
	assertFaceAbsentRefusal(t, refused, rerr, "Noto Sans Thai")
}

// TestAbsentStyledVariantStillFallsBackSilently is matrix row 5, and it
// is the boundary this story must not cross.
//
// An absent STYLED VARIANT is a different question from an absent BASE
// face: the entry that covers the rune is present and draws it, at the
// wrong weight. That is Story 11.2's condition, it has its own code, and
// on a chain of bare face names it is not even a diagnostic. Refusing
// here would fail every document that asks for bold from a chain that
// declares none.
func TestAbsentStyledVariantStillFallsBackSilently(t *testing.T) {
	// The chain DECLARES a bold variant, the element REQUESTS bold, and
	// the caller does not supply the bold face.
	source := faceAbsentBoldDoc(`[{"face": "Noto Sans", "bold": "Noto Sans Bold"}]`, `"Ada"`)

	res, err := renderFaceAbsentDoc(t, source, fontSetWithout(t, "Noto Sans Bold"))
	if err != nil {
		t.Fatalf("an absent STYLED VARIANT must fall back to the entry's own base face, never refuse: %v", err)
	}
	if len(res.Bytes) == 0 {
		t.Error("the render produced no PDF bytes")
	}
	// ⚠ WHAT "SILENTLY" MEANS HERE, MEASURED RATHER THAN ASSUMED. The
	// story's matrix reads "no diagnostic", and the SHIPPED engine in
	// fact emits Story 11.2's TEXT_STYLE_FACE_UNDECLARED Warning for an
	// absent variant — that is its documented absence arm (FR57), it
	// predates this story, and changing it would be an unrelated edit to
	// a shipped Warning's output. What this story owes the row is that
	// the absence of a VARIANT is never CAP-7's refusal.
	//
	// THE COUNT IS ASSERTED OUTSIDE THE LOOP, because a per-diagnostic
	// loop passes vacuously over an empty slice — it would report an
	// all-clear against the very engine whose behaviour the comment above
	// states as fact. "Ada" has three distinct runes, and the Warning is
	// coalesced per (element, distinct rune).
	if len(res.Diagnostics) != 3 {
		t.Fatalf("want one TEXT_STYLE_FACE_UNDECLARED Warning per distinct rune of \"Ada\" (3), got %d: %+v", len(res.Diagnostics), res.Diagnostics)
	}
	for _, d := range res.Diagnostics {
		if d.Code != DiagCodeTextStyleFaceUndeclared {
			t.Errorf("unexpected diagnostic from an absent styled variant — never the CAP-7 refusal: %+v", d)
		}
		if d.Severity != SeverityWarning {
			t.Errorf("severity = %s, want Warning: %+v", d.Severity, d)
		}
	}

	// THE NEGATIVE CONTROL: the same chain with the BASE face withheld
	// instead of the variant. One entry, one requested weight, and the
	// outcome turns entirely on WHICH face is missing.
	refused, rerr := renderFaceAbsentDoc(t, source, fontSetWithout(t, "Noto Sans"))
	assertFaceAbsentRefusal(t, refused, rerr, "Noto Sans")
}

// TestAbsentChainMemberDoesNotRefuseOnALineFeed is matrix row 6.
//
// A line feed a caller CONSUMES is absent from the drawn output by
// design, not for want of a glyph — no face has ever carried one, and
// FR46 takes it as a mandatory break. The refusal sits inside that
// existing suppression, so an absent chain member cannot turn a
// paragraph break into a failed render.
func TestAbsentChainMemberDoesNotRefuseOnALineFeed(t *testing.T) {
	source := faceAbsentDoc(`["Noto Sans", "Noto Sans SC"]`, `"Ada\nAdeline"`)

	res, err := renderFaceAbsentDoc(t, source, fontSetWithout(t, "Noto Sans SC"))
	if err != nil {
		t.Fatalf("a newline in text whose chain has an absent member must render exactly as before: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Errorf("the newline produced diagnostics: %+v", res.Diagnostics)
	}

	// THE NEGATIVE CONTROL: the same text with one Han rune added. The
	// suppression is scoped to U+000A alone and must not shelter a rune
	// the document really did ask to be drawn.
	refused, rerr := renderFaceAbsentDoc(t, faceAbsentDoc(`["Noto Sans", "Noto Sans SC"]`, `"Ada\n汉"`), fontSetWithout(t, "Noto Sans SC"))
	assertFaceAbsentRefusal(t, refused, rerr, "Noto Sans SC")
}

// TestTheAbsentFaceRefusalNamesTheFaceNotTheChain pins the one thing
// that makes the refusal actionable, and it is asserted against the
// diagnostic it could plausibly have reused.
//
// missingGlyphMessage names the WHOLE CHAIN, because that Warning tells
// its reader to edit the chain. This refusal tells its reader to SUPPLY
// A FACE, and only naming that face says which — so the absent face must
// appear, spelled as the caller spelled it in the FontSet.
func TestTheAbsentFaceRefusalNamesTheFaceNotTheChain(t *testing.T) {
	res, err := renderFaceAbsentDoc(t,
		faceAbsentDoc(`["Noto Sans", "Noto Sans Thai", "Noto Sans SC"]`, `"汉"`),
		fontSetWithout(t, "Noto Sans SC"),
	)
	assertFaceAbsentRefusal(t, res, err, "Noto Sans SC")

	var refusal *RenderError
	if !errors.As(err, &refusal) {
		t.Fatalf("not a *RenderError: %T", err)
	}
	// The absent face is named FIRST and by itself, ahead of the chain
	// the message also prints for context — a reader must not have to
	// diff two lists to find the one name that matters.
	msg := refusal.Diagnostic.Message
	nameAt := strings.Index(msg, `face "Noto Sans SC" is not present`)
	chainAt := strings.Index(msg, "[Noto Sans, Noto Sans Thai, Noto Sans SC]")
	if nameAt < 0 || chainAt < 0 {
		t.Fatalf("the message must carry both the absent face and the chain searched:\n\t%s", msg)
	}
	if nameAt > chainAt {
		t.Errorf("the message leads with the chain rather than the absent face:\n\t%s", msg)
	}
	// AND IT MUST NOT BE THE MISSING-GLYPH WARNING'S SENTENCE. That one
	// says the rune was omitted and the render went on, which is exactly
	// what did NOT happen here.
	if strings.Contains(msg, "the rune is omitted from the rendered output") {
		t.Errorf("the refusal reuses missingGlyphMessage's wording, which is false of a refused render:\n\t%s", msg)
	}
}

// TestNoCommittedFixtureReachesTheAbsentFaceRefusal states EXACTLY what it
// measures, which is narrower than the story's fifth acceptance criterion
// and deliberately says so.
//
// WHAT IT MEASURES: every baseline acceptance fixture still renders
// without reaching the refusal. That is the half this file can assert —
// the refusal returns no bytes at all, so a fixture that reached it would
// fail here with a message naming this story.
//
// WHAT IT DOES NOT MEASURE, AND MUST NOT CLAIM TO: that no golden PDF
// HASH moved. No hash is compared here, and writing a second hash
// comparison beside the ones that already own it would be a weaker copy
// of them. The hashes are pinned by the golden fixture tests themselves
// (`go test ./... -run 'Golden|Parity|Fixture'`), across three languages,
// and those are the tests that fail if a byte moves.
func TestNoCommittedFixtureReachesTheAbsentFaceRefusal(t *testing.T) {
	rendered := 0
	for _, f := range baselineAcceptanceFixtures {
		if f.render(t) == nil {
			t.Errorf("%s: render produced no bytes — a CAP-7 refusal returns none, so check whether this fixture now reaches it", f.name)
			continue
		}
		rendered++
	}
	if rendered != len(baselineAcceptanceFixtures) {
		t.Fatalf("presence precondition: %d of %d fixtures rendered", rendered, len(baselineAcceptanceFixtures))
	}
	t.Logf("CAP-7: %d baseline acceptance fixtures still render; every one supplies the full font set, so none reaches the refusal. Hash identity is asserted by the golden fixture tests, not here.", rendered)
}

// TestChainWithNoPresentMemberRefusesUnlocatedWhenNothingIsDrawable is the
// one arm of the refusal a caller receives WITHOUT an element id, and it
// had no through-Render test at all until this one.
//
// An element whose whole text is line feeds a caller CONSUMES gives
// shapeSegments nothing to refuse — the newline suppression is
// deliberate and unchanged — so a chain with no present member falls
// through to the vertical model, which has no metrics to derive a line
// height from and refuses there. That is the only path on which
// TEXT_FACE_ABSENT escapes with an empty ElementID, so the location a
// caller actually receives is what is asserted.
func TestChainWithNoPresentMemberRefusesUnlocatedWhenNothingIsDrawable(t *testing.T) {
	source := faceAbsentDoc(`["Noto Sans SC"]`, `"\n"`)

	res, err := renderFaceAbsentDoc(t, source, fontSetWithout(t, "Noto Sans SC"))
	if err == nil {
		t.Fatalf("an element with no present face must be refused however little it draws; diagnostics: %+v", res.Diagnostics)
	}
	if len(res.Bytes) != 0 {
		t.Errorf("a refused render returned %d PDF bytes", len(res.Bytes))
	}
	var refusal *RenderError
	if !errors.As(err, &refusal) {
		t.Fatalf("the refusal must reach the caller as a *RenderError: %T %v", err, err)
	}
	d := refusal.Diagnostic
	if d.Code != DiagCodeTextFaceAbsent {
		t.Errorf("code = %q, want %q — the same code whether the fault is caught in shaping or in the vertical model; error = %v", d.Code, DiagCodeTextFaceAbsent, err)
	}
	if d.Severity != SeverityError {
		t.Errorf("severity = %s, want Error", d.Severity)
	}
	// THE LOCATION A CALLER ACTUALLY RECEIVES, pinned as it is rather
	// than as it would ideally be: the vertical model is the pure
	// arithmetic and holds no element id, so this arm is located by data
	// path alone. A later story that threads an element id through will
	// redden here, which is the point.
	if d.ElementID != "" {
		t.Errorf("ElementID = %q — this arm is located by data path alone; if that changed, say so here deliberately", d.ElementID)
	}
	if d.DataPath != "style.fontFamily" {
		t.Errorf("DataPath = %q, want style.fontFamily", d.DataPath)
	}
	// AND THE MESSAGE IS STILL THE VERTICAL MODEL'S OWN, byte-for-byte:
	// coding that error must not have rewritten what it says.
	if !strings.Contains(d.Message, "no line height can be derived from it") {
		t.Errorf("the vertical model's message did not survive being coded:\n\t%s", d.Message)
	}
}

// TestAbsentChainMemberDoesNotRefuseOnAControlCharacter is the newline
// rule generalised to the characters it was always about.
//
// The refusal's justification is that the absent face MIGHT have covered
// the rune. No font has ever carried U+0009, so for a control character
// that justification is simply false — it is absent from the drawn
// output by design, exactly as U+000A is. The WARNING is untouched: a tab
// no supplied face covers still reports and still drops, as it did
// before this story.
func TestAbsentChainMemberDoesNotRefuseOnAControlCharacter(t *testing.T) {
	source := faceAbsentDoc(`["Noto Sans", "Noto Sans SC"]`, `"Ada\tAdeline"`)

	res, err := renderFaceAbsentDoc(t, source, fontSetWithout(t, "Noto Sans SC"))
	if err != nil {
		t.Fatalf("a control character must not refuse merely because a chain member is absent: %v", err)
	}
	if len(res.Bytes) == 0 {
		t.Error("the render produced no PDF bytes")
	}

	// THE NEGATIVE CONTROL, and it is the one that makes this a measured
	// claim rather than a blanket exemption: the SAME text with the full
	// font set behaves identically, so the control character's treatment
	// does not depend on the supply at all.
	control, cerr := renderFaceAbsentDoc(t, source, testShippedFontSet())
	if cerr != nil {
		t.Fatalf("the control render failed: %v", cerr)
	}
	if len(control.Diagnostics) != len(res.Diagnostics) {
		t.Errorf("withholding a face changed what a control character reports: %+v vs %+v", res.Diagnostics, control.Diagnostics)
	}
	for _, d := range res.Diagnostics {
		if d.Code != DiagCodeTextMissingGlyph {
			t.Errorf("unexpected diagnostic for a control character: %+v", d)
		}
	}

	// AND AN ORDINARY RUNE IN THE SAME DOCUMENT STILL REFUSES. Without
	// this the exemption could have swallowed the whole refusal.
	refused, rerr := renderFaceAbsentDoc(t, faceAbsentDoc(`["Noto Sans", "Noto Sans SC"]`, `"Ada\t汉"`), fontSetWithout(t, "Noto Sans SC"))
	assertFaceAbsentRefusal(t, refused, rerr, "Noto Sans SC")
}

// TestTheRefusalNamesEveryAbsentChainMember pins the message against the
// case that made naming only the first one wrong.
//
// With two members absent the engine cannot know which of them would have
// covered the rune — that ignorance is the refusal's whole reason for
// existing — so naming one sends the author to supply a face that may
// have nothing to do with the script, and to be refused again.
func TestTheRefusalNamesEveryAbsentChainMember(t *testing.T) {
	res, err := renderFaceAbsentDoc(t,
		faceAbsentDoc(`["Noto Sans Thai", "Noto Sans SC"]`, `"汉"`),
		fontSetWithout(t, "Noto Sans Thai", "Noto Sans SC"),
	)
	if err == nil {
		t.Fatalf("a chain with no present member must refuse; diagnostics: %+v", res.Diagnostics)
	}
	var refusal *RenderError
	if !errors.As(err, &refusal) {
		t.Fatalf("not a *RenderError: %T %v", err, err)
	}
	msg := refusal.Diagnostic.Message
	if want := `faces "Noto Sans Thai", "Noto Sans SC" are not present in the supplied FontSet`; !strings.Contains(msg, want) {
		t.Errorf("the message must name EVERY absent member (%q missing):\n\t%s", want, msg)
	}

	// THE NEGATIVE CONTROL: with exactly one member absent the sentence
	// stays the singular one fontCache.get already uses, so the common
	// case did not grow a plural it does not need.
	one, oneErr := renderFaceAbsentDoc(t,
		faceAbsentDoc(`["Noto Sans Thai", "Noto Sans SC"]`, `"汉"`),
		fontSetWithout(t, "Noto Sans SC"),
	)
	assertFaceAbsentRefusal(t, one, oneErr, "Noto Sans SC")
}

// tableLabelDoc is a one-table document whose single column label is the
// text under test. The header style names the `body` chain, which is what
// makes the label's own shaping reach the supplied FontSet at all.
func tableLabelDoc(chain, label string) string {
	return `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "table", "x": 0, "y": 0, "bind": "rows[]", "as": "row", "headerHeight": 28,
         "style": {"fontFamily": "body", "fontSize": 10, "padding": {"bottom": 4, "left": 3, "right": 3, "top": 4}},
         "columns": [{"id": "e2", "label": ` + label + `, "width": 200, "align": "left", "bind": "{{row.value}}"}]}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ` + chain + `},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
}

// TestCanvasTableLabelRefusesForAnAbsentFaceRatherThanBlanking is the row
// spec-deferred-offline-cache's matrix calls "Canvas table label needing
// an absent face", and it is the one the CJK-face deferral makes
// reachable.
//
// addCanvasTableLabelLines used to drop `shapeSegments`'s error on the
// floor with a bare `continue`. That was invisible while the designer's
// host passed the whole shipped set — a face was never absent — and it
// becomes a SILENTLY BLANK COLUMN LABEL the moment the designer's engine
// can be short of a face: the canvas would open, the column would lose
// its packed label, and nothing would tell the browser which face to go
// and fetch.
//
// BOTH SIDES ARE PINNED HERE. A Latin label over the same partial set
// still projects — the `continue` arms that degrade for other reasons are
// untouched — and the CJK label refuses, named and located.
func TestCanvasTableLabelRefusesForAnAbsentFaceRatherThanBlanking(t *testing.T) {
	const chain = `["Noto Sans", "Noto Sans Thai", "Noto Sans SC"]`
	partial := fontSetWithout(t, "Noto Sans SC")

	// THE NEGATIVE CONTROL FIRST: the same document, the same partial set,
	// a label whose runes the present faces cover. If this refused too,
	// the row below would be proving "the canvas refuses tables", not
	// "the canvas refuses an absent face".
	latin, err := ParseTemplate([]byte(tableLabelDoc(chain, `"Date"`)))
	if err != nil {
		t.Fatal(err)
	}
	projection, err := canvasWithTextPaint(latin, partial)
	if err != nil {
		t.Fatalf("a Latin table label over a partial font set must still project: %v", err)
	}
	if len(projection.Components) != 1 || len(projection.Components[0].Columns) != 1 {
		t.Fatalf("the control projection has no table column to carry a label: %+v", projection.Components)
	}
	if lines := projection.Components[0].Columns[0].LabelLines; len(lines) != 1 || lines[0] != "Date" {
		t.Fatalf("the control column's packed label is %q, so the row below would compare against a projection that never packed one", lines)
	}

	// THE ROW ITSELF.
	cjk, err := ParseTemplate([]byte(tableLabelDoc(chain, `"汉"`)))
	if err != nil {
		t.Fatalf("a document naming an unsupplied face must still LOAD: %v", err)
	}
	_, err = canvasWithTextPaint(cjk, partial)
	if err == nil {
		t.Fatal("the canvas projected a CJK table column label with the face absent — the label would be silently blank and nothing would name the face to fetch")
	}
	var refusal *RenderError
	if !errors.As(err, &refusal) {
		t.Fatalf("the refusal must reach the caller as a *RenderError carrying a code, or the wasm host reports it as an uncoded engine rejection: %T %v", err, err)
	}
	if refusal.Diagnostic.Code != DiagCodeTextFaceAbsent {
		t.Errorf("code = %q, want %q", refusal.Diagnostic.Code, DiagCodeTextFaceAbsent)
	}
	if want := `face "Noto Sans SC" is not present in the supplied FontSet`; !strings.Contains(refusal.Diagnostic.Message, want) {
		t.Errorf("the refusal does not name the absent face (%q missing):\n\t%s", want, refusal.Diagnostic.Message)
	}
	// The column id is the location shapeSegments was given, and it is
	// what makes the refusal point at the label rather than at the table.
	if !strings.Contains(err.Error(), "e2") {
		t.Errorf("the refusal does not locate the column it arose in: %v", err)
	}
}
