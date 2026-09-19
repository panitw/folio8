package folio8

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/fontset"
	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/template"
)

// TestCanvasMeasuresWithTheEmbeddedFace is Story 8.4's AC5, stated as VERIFIED
// rather than deleted (D-8.4.1a).
//
// WHY THIS IS A TEST AND NOT A FEATURE. addCanvasTextPaint (page_setup.go)
// already calls the render path's own fontChain, shapeSegments,
// chainVerticalModel and positionSegments — AD-17's "the canvas consumes the
// IDENTICAL advance the renderer does … the browser never measures text". So
// the moment the render path can resolve an embedded entry, the canvas can
// too, and the measurement half of AC5 arrives with the seam rather than with
// any canvas code of its own.
//
// THAT IS EXACTLY WHY IT NEEDS A PIN. "Shared today" and "shared tomorrow" are
// different claims, and the difference is one refactor wide. There is ONE
// thing the two paths do not share — each builds its own fontCache
// (predictDocument at render.go, addCanvasTextPaint at page_setup.go) — and if
// the canvas's were built without the document, every assertion below would
// fail while the PDF stayed perfect. That is the mutation this test is
// red-proved against: replace newDocumentFontCache(t) with newFontCache() in
// page_setup.go and this reddens on its own.
//
// WHAT IS NOT CLAIMED HERE. The canvas MEASURES with the embedded face; the
// browser cannot PAINT with it, because the designer has no CSS family for a
// carried face at all and falls through to `sans-serif`. That is Story 8.4a
// (DW-35), and the disclosure is carried by
// folio-designer/src/canvas-font-stack.test.ts rather than by a comment.
func TestCanvasMeasuresWithTheEmbeddedFace(t *testing.T) {
	tpl, err := ParseTemplate([]byte(embeddedFontTemplateJSON()))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fs := testShippedFontSet()

	projection, err := canvasWithTextPaint(tpl, fs)
	if err != nil {
		t.Fatalf("canvas projection: %v", err)
	}
	// A DEGRADED CHAIN IS HOW THIS FAILS SILENTLY, and it has to be excluded
	// before anything below means anything. addCanvasTextPaint disposes of an
	// element it cannot shape — an empty paint, and on carries — so a canvas
	// that could not see the carried face returns a perfectly well-formed
	// projection with nothing in it, and a comparison over zero fragments
	// passes. Two independent witnesses, because they fail differently:
	// ContentWindowCountIsExact is the ENGINE's own report that no chain
	// degraded (page_setup.go's `exact` term; this document has no bound table
	// and no conditional visibility, so a degraded chain is the only cause
	// that can be present), and the fragment count below is the fact itself.
	if !projection.ContentWindowCountIsExact {
		t.Fatal("the engine reports an inexact window count for a document with no table and no conditional visibility — its font chain degraded, so the embedded entry did not reach the canvas's own fontCache")
	}

	data := emptyBindValue(t)
	runs, err := collectTextRuns(tpl, data, data, fs, newDocumentFontCache(tpl))
	if err != nil {
		t.Fatalf("shipping run collection: %v", err)
	}
	if len(runs) == 0 {
		t.Fatal("presence precondition: the PDF path produced no text runs, so nothing below is compared")
	}

	// THE WITNESS THAT THIS DOCUMENT EXERCISES THE SUBJECT AT ALL. Every run
	// must be drawn with the face the document CARRIES — not the shipped Latin
	// face its chain names first, which covers none of this Thai. Without this
	// the whole test would keep passing over a document whose embedded entry
	// was never consulted, which is the vacuous version of it.
	wantFace := embeddedFaceName(embeddedFontAssetKey())
	for _, run := range runs {
		if run.face != wantFace {
			t.Fatalf("a run is drawn with face %q, want the carried face %q — this document is not exercising the embedded path", run.face, wantFace)
		}
	}

	byElement := make(map[string][]textRunSource)
	for _, run := range runs {
		byElement[run.elementID] = append(byElement[run.elementID], run)
	}
	bandY := make(map[string]int64)
	for _, band := range projection.Bands {
		bandY[band.Name] = band.Y
	}

	// The comparison is canvas_text_paint_test.go's own — term for term, not a
	// looser restatement of it — so the two tests cannot drift into asserting
	// different strengths of the same claim.
	cache := newDocumentFontCache(tpl)
	compared := 0
	for _, component := range projection.Components {
		if component.Type != "text" || component.TextPaint == nil {
			continue
		}
		wantRuns := byElement[component.ID]
		if len(wantRuns) == 0 {
			t.Fatalf("component %s has paint but no PDF-path runs", component.ID)
		}
		for lineIndex, line := range component.TextPaint.Lines {
			var lineRuns []textRunSource
			for _, run := range wantRuns {
				if run.lineIndex == lineIndex {
					lineRuns = append(lineRuns, run)
				}
			}
			if len(lineRuns) != len(line.Fragments) {
				t.Fatalf("component %s line %d has %d canvas fragments and %d PDF runs", component.ID, lineIndex, len(line.Fragments), len(lineRuns))
			}
			var width geom.Length
			for fragmentIndex, run := range lineRuns {
				runWidth, werr := shippingRunWidth(run, fs, cache)
				if werr != nil {
					t.Fatal(werr)
				}
				width += runWidth
				bandOrigin := bandY[component.Band] - projection.MarginTop
				if line.Top != int64(run.y)-bandOrigin || line.Baseline != int64(run.y+run.baselineOffset)-bandOrigin {
					t.Errorf("component %s line %d origin/baseline diverges from the PDF path: %#v versus y=%d baseline=%d", component.ID, lineIndex, line, run.y, run.y+run.baselineOffset)
				}
				fragment := line.Fragments[fragmentIndex]
				if fragment.Text != run.text || fragment.X != int64(run.x) {
					t.Errorf("component %s line %d fragment %d = %#v, want the PDF path's text=%q x=%d", component.ID, lineIndex, fragmentIndex, fragment, run.text, run.x)
				}
				compared++
			}
			// The ADVANCE half. A fragment origin that matched while the line
			// advance did not would still stack the second line wrongly, and
			// the advance is what the embedded face's own hhea metrics feed.
			if line.Width != int64(width) {
				t.Errorf("component %s line %d width = %d, want the PDF path's shaped width %d", component.ID, lineIndex, line.Width, width)
			}
			if line.Advance <= 0 {
				t.Errorf("component %s line %d advance = %d — the vertical model derived nothing from the carried face", component.ID, lineIndex, line.Advance)
			}
		}
	}
	if compared == 0 {
		t.Fatal("vacuity guard: no fragment was compared, so this test asserted nothing")
	}
	t.Logf("canvas/PDF measurement agreement witness — %d fragments compared, all drawn with the carried face %q", compared, wantFace)
}

// TestCanvasVerticalModelUsesTheEmbeddedFacesOwnMetrics is the half the
// fragment comparison above cannot see on a one-line document: that the line
// advance the canvas reports is derived from the CARRIED face's hhea metrics
// and not from the shipped face the chain names first.
//
// The two faces have different vertical metrics, so a canvas that resolved
// only the shipped entry would report a different advance — and on a
// single-line element nothing else would move, which is exactly how this would
// otherwise ship unnoticed.
func TestCanvasVerticalModelUsesTheEmbeddedFacesOwnMetrics(t *testing.T) {
	tpl, err := ParseTemplate([]byte(embeddedFontTemplateJSON()))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fs := testShippedFontSet()
	chain := []string{"Noto Sans", embeddedFaceName(embeddedFontAssetKey())}
	const fontSize = geom.Length(12_000)

	withCarried, err := chainVerticalModel(chain, fontSize, defaultLineSpacing, fs, newDocumentFontCache(tpl))
	if err != nil {
		t.Fatalf("vertical model over the carried chain: %v", err)
	}
	// The same chain against a cache that cannot see the document: the
	// embedded entry supplies nothing and only "Noto Sans" constrains the
	// model. This is the pre-Story-8.4 answer, and the two must differ or the
	// assertion above is measuring the shipped face under another name.
	shippedOnly, err := chainVerticalModel(chain, fontSize, defaultLineSpacing, fs, newFontCache())
	if err != nil {
		t.Fatalf("vertical model over the shipped entry alone: %v", err)
	}
	if withCarried == shippedOnly {
		t.Fatalf("the carried face contributed nothing to the vertical model: %#v", withCarried)
	}

	projection, err := canvasWithTextPaint(tpl, fs)
	if err != nil {
		t.Fatalf("canvas projection: %v", err)
	}
	found := false
	for _, component := range projection.Components {
		if component.TextPaint == nil {
			continue
		}
		for _, line := range component.TextPaint.Lines {
			found = true
			if line.Advance != int64(withCarried.Advance) {
				t.Errorf("canvas line advance = %d, want %d — the canvas's vertical model does not include the carried face (it reports %d without it)",
					line.Advance, withCarried.Advance, shippedOnly.Advance)
			}
		}
	}
	if !found {
		t.Fatal("vacuity guard: the projection carries no painted line, so nothing was compared")
	}
}

// TestCanvasDegradesRatherThanAbortingOnANonFontChainEntry pins D-7.4.2 on the
// element arm Story 8.4 made reachable.
//
// THE CONDITION IS NEW AND IT COMES FROM DOCUMENT CONTENT. Before this story a
// chain entry naming a non-font asset resolved to nothing anywhere; since it,
// coverage resolution refuses it, located — which is DW-83's whole point on
// the render path, and exactly the wrong answer on the canvas. The designer is
// the ONE surface on which an author can repair such an entry, and a
// projection that returns an error opens no document at all.
//
// So the capability refusal is tolerated HERE, per element, in the shape
// addCanvasTextPaint's own fontChain arm a few lines above already uses: an
// empty paint, the column flagged as degraded, and on to the next element.
//
// THE SCOPING SENTENCE THAT USED TO END THIS COMMENT WAS WRONG, and D-8.4.12
// says so: the tolerance was scoped to the capability error
// (template.UnsupportedFontMediaTypeError) on the stated ground that
// everything else was "a genuine internal shaping fault … not a document
// property". The population an error-type gate excludes contains document
// properties — see the test below, whose carried face is one. The gate is now
// POSITIONAL: a fault resolving a face the element's own chain names degrades
// that element whatever type carries it. This document still exercises the
// same arm; it is simply no longer the only one that reaches it.
//
// RED-PROOF: restore the unconditional `return` in addCanvasTextPaint's
// shapeSegments arm and this test fails on the projection error.
func TestCanvasDegradesRatherThanAbortingOnANonFontChainEntry(t *testing.T) {
	source := nonFontChainDoc(t, `["Noto Sans", {"asset": "`+nonFontAssetKey+`"}]`, "สัญญา")
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("the document must LOAD: %v", err)
	}
	fs := testShippedFontSet()

	// The geometry-only projection never shaped anything and always worked;
	// asserting it here is what makes the paint half's failure specific.
	if _, gerr := canvas(tpl); gerr != nil {
		t.Fatalf("the geometry projection must succeed: %v", gerr)
	}

	projection, err := canvasWithTextPaint(tpl, fs)
	if err != nil {
		t.Fatalf("a document the FORMAT calls valid must still open in the designer — the surface on which its chain can be repaired: %v", err)
	}
	found := false
	for _, component := range projection.Components {
		if component.ID != "e1" {
			continue
		}
		found = true
		if component.TextPaint == nil {
			t.Fatal("the element carries no paint at all — it must carry an EMPTY paint, which is what the canvas draws as nothing")
		}
		if len(component.TextPaint.Lines) != 0 {
			t.Errorf("the element painted %d line(s) from a chain entry the engine refuses to draw with", len(component.TextPaint.Lines))
		}
	}
	if !found {
		t.Fatal("vacuity guard: the projection carries no component e1")
	}
	// AND IT SAYS SO. A silently-degraded element and a well-measured one are
	// indistinguishable downstream, which is why the honesty flag exists.
	if projection.ContentWindowCountIsExact {
		t.Error("the projection reports an EXACT window count for a document whose only text element could not be shaped — the degradation is not disclosed")
	}

	// The render path is unchanged: this document is still refused there,
	// located. The canvas tolerance is a canvas decision and must not have
	// leaked into Render.
	if _, rerr := Render(tpl, Data(`{}`), nil, fs); rerr == nil {
		t.Error("Render must still refuse this document (DW-83) — the canvas's tolerance leaked onto the render path")
	}
}

// TestCanvasDegradesRatherThanAbortingOnAnUnreadableCarriedFace is the
// INVERSION of the characterization test that stood here, performed under
// D-8.4.12's ruling. It is not a new test: the fixture, the precondition and
// the surfaces asserted are the ones the characterization already carried —
// only the expected disposition of the paint projection is turned around.
//
// WHAT IT USED TO RECORD. Story 8.4 changed a failure mode CONDITIONALLY, and
// D-8.4.9(a) is the obligation not to change a failure mode without naming
// what asserts it. Measured then: reverting the fix to an unconditional
// `return` reddened exactly ONE test
// (TestCanvasDegradesRatherThanAbortingOnANonFontChainEntry, above), and
// dropping the scoping altogether — degrading unconditionally — reddened
// NOTHING in the whole suite. So the changed half was pinned and the RETAINED
// half was not pinned at all. D-8.4.12 named that, and not the abort, as the
// finding: WHEN A RULING SPLITS A POPULATION, THE RETAINED HALF IS THE ONE
// NOTHING TESTS.
//
// WHY IT INVERTED. The scoping's stated premise was false about its own
// population. checkSfnt is explicit that it validates the table directory and
// never a table's contents, leaving that to the shaper — so the carried face
// built below, a STRUCTURALLY VALID sfnt whose one declared table has no
// contents, LOADS, and then aborted the entire projection at fontset.New.
// That is a document property; it has an author repair (fix or replace the
// chain entry); and the surface the repair happens on is the one the abort
// closed. The same D-7.4.2 shape Story 8.4 fixed on the neighbouring arm,
// surviving on this one because the gate enumerated error TYPES instead of
// asking where the fault arose.
//
// WHAT DID NOT MOVE, AND WHY THAT IS THE POINT. The ParseTemplate Fatal below
// is kept VERBATIM. It guards the PRECONDITION rather than the behaviour —
// that the format calls this document valid — so it is independent of which
// way the arm points: a future story moving the check into the loader is found
// there whether this projection degrades or aborts. Guarding the precondition
// and not only the behaviour is the part of the previous author's work worth
// repeating, along with its declining to ratify its own measurement: widening
// on a test's authority would have been the wrong call, and referring it up
// with the measurement attached is what produced the ruling.
//
// THE RETAINED ABORT HAS ITS OWN ASSERTION NOW, and it is a different document
// entirely: TestCanvasStillAbortsOnAHostFontSetFaceThatWillNotParse, below.
//
// RED-PROOF: restore the errors.As allowlist (or any gate that excludes
// fontset.New's failure) in addCanvasTextPaint's shapeSegments arm and this
// test fails on the projection error.
// unreadableCarriedFaceDoc is the D-8.4.12 fixture, lifted out of the test
// below so the stamp's own unit test can reach the SAME document rather than a
// look-alike. It returns the document source and the asset key, whose derived
// face name (embeddedFaceName) is what a fontCache is keyed by.
//
// The bytes are a structurally valid sfnt: the single-face version tag, one
// declared table, a whole table directory, and that table's (offset, length)
// inside the file — everything checkSfnt looks at. Its CONTENTS are absent,
// which is everything checkSfnt deliberately does not look at. So the document
// LOADS and the face fails later, at fontset.New, which is the whole reason
// this population exists.
func unreadableCarriedFaceDoc(t *testing.T) (source, assetKey string) {
	t.Helper()
	raw := make([]byte, 28)
	binary.BigEndian.PutUint32(raw[0:], 0x00010000)
	binary.BigEndian.PutUint16(raw[4:], 1)   // numTables
	binary.BigEndian.PutUint16(raw[6:], 16)  // searchRange
	copy(raw[12:16], "cmap")                 // the one declared table
	binary.BigEndian.PutUint32(raw[20:], 28) // offset: end of file
	binary.BigEndian.PutUint32(raw[24:], 0)  // length: zero

	key := fmt.Sprintf("%x", sha256.Sum256(raw))
	src := embeddedChainDocText(t, `["Noto Sans", {"asset": "`+key+`"}]`, "สัญญา")
	const anchor = "  \"assets\": {\n"
	at := strings.Index(src, anchor)
	if at < 0 {
		t.Fatal("fixture assumption violated: the generated document has no assets block")
	}
	// The `font` record is not decoration here: the chain NAMES this asset, so
	// since Story 8.6 it is an embedded face and the document does not LOAD
	// without its terms. This population's whole point is a document that
	// loads and fails later, so the licence half has to be satisfied for the
	// later failure to be reachable at all.
	blob := "    \"" + key + "\": {\n      \"data\": [\"" + base64.StdEncoding.EncodeToString(raw) + "\"],\n      \"font\": {" + requiredLicenceKeys + "},\n      \"mediaType\": \"font/ttf\"\n    },\n"
	return src[:at+len(anchor)] + blob + src[at+len(anchor):], key
}

func TestCanvasDegradesRatherThanAbortingOnAnUnreadableCarriedFace(t *testing.T) {
	source, _ := unreadableCarriedFaceDoc(t)

	// THE PRECONDITION THAT MAKES THIS A D-7.4.2 QUESTION AT ALL: the format
	// calls this document valid. If a future story moves the check into the
	// loader, this Fatal is where that is found, and the abort below stops
	// being reachable from document content.
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("PRECONDITION CHANGED: this document no longer LOADS, so the abort below is no longer reachable from document content and this characterization is stale — re-read D-8.4.9(a) before deleting it: %v", err)
	}
	// The geometry-only projection never shaped anything and always worked;
	// asserting it here is what makes the paint half's failure specific.
	if _, gerr := canvas(tpl); gerr != nil {
		t.Fatalf("the geometry projection must succeed: %v", gerr)
	}

	projection, perr := canvasWithTextPaint(tpl, testShippedFontSet())
	if perr != nil {
		t.Fatalf("a document the FORMAT calls valid must still open in the designer — the surface on which its chain entry can be repaired (D-8.4.12): %v", perr)
	}
	// The degrade is the same shape the neighbouring arm's test asserts, term
	// for term, because D-8.4.12's whole content is that these two documents
	// belong to ONE population: an EMPTY paint (not a missing one), and the
	// element's absent extents disclosed on the projection.
	found := false
	for _, component := range projection.Components {
		if component.ID != "e1" {
			continue
		}
		found = true
		if component.TextPaint == nil {
			t.Fatal("the element carries no paint at all — it must carry an EMPTY paint, which is what the canvas draws as nothing")
		}
		if len(component.TextPaint.Lines) != 0 {
			t.Errorf("the element painted %d line(s) from a chain whose only usable face cannot be read", len(component.TextPaint.Lines))
		}
	}
	if !found {
		t.Fatal("vacuity guard: the projection carries no component e1")
	}
	if projection.ContentWindowCountIsExact {
		t.Error("the projection reports an EXACT window count for a document whose only text element could not be shaped — the degradation is not disclosed")
	}

	// AND THE RENDER PATH DOES NOT MOVE. Render refusing this document is not
	// in question — a page drawn with an unreadable face would be wrong — so
	// it is asserted here to keep the two surfaces' dispositions visible side
	// by side. The widening is a CANVAS decision and must not have leaked onto
	// the render path, which is the same thing the neighbouring arm's test
	// checks for its own document.
	if _, rerr := Render(tpl, Data(`{}`), nil, testShippedFontSet()); rerr == nil {
		t.Error("Render must refuse a document whose only usable face cannot be read")
	}
}

// TestCanvasStillAbortsOnAHostFontSetFaceThatWillNotParse is the assertion the
// RETAINED half of D-8.4.12 has never had, and writing it is the finding —
// not the abort it pins.
//
// WHY IT IS THE FINDING. Story 8.4 split a population and pinned one half;
// D-8.4.12 re-split the same population on a different axis. Both times the
// half that CHANGED was measured and the half that was KEPT was not: before
// this test, mutating addCanvasTextPaint's shapeSegments arm to degrade
// unconditionally reddened NOTHING in the entire Go suite, so nothing in the
// repository could tell the difference between "the abort is scoped" and
// "there is no abort". The general lesson D-8.4.12 records is that when a
// ruling splits a population, THE RETAINED HALF IS THE ONE NOTHING TESTS.
//
// WHY THIS DOCUMENT, AND NOT AN INTERNAL SHAPING FAULT. A host-supplied
// FontSet face that will not parse is the case that DISCRIMINATES THE TWO
// AXES (D-8.4.12 guardrail 6), which is exactly what makes it the right
// witness for what survives:
//
//	an ERROR-TYPE gate       — sweeps it in, and would degrade
//	the ATTRIBUTABILITY axis — leaves it out, and aborts
//
// The failure below is the SAME failure the test above tolerates, at the same
// door, of the same shape (fontset.New refusing bytes it cannot read). The
// ONLY thing that differs is WHO SUPPLIED THE FACE — this one is the host
// application's, so no edit an author makes on the canvas repairs it, and no
// document is locked out of its own repair by refusing to open. A test that
// distinguishes two runs differing in one variable is what makes the axis a
// measured claim rather than a stated one.
//
// IT IS DELIBERATELY NOT ABSORBED. Whether a caller's unreadable face should
// have a better answer than an aborted projection is a separate question with
// a separate owner; D-8.4.12 says so and says do not answer it here.
//
// RED-PROOF (this is the mutation that used to redden nothing): change the
// gate in addCanvasTextPaint's shapeSegments arm to degrade unconditionally —
// delete the `if !errors.As(...)` and its return — and this test fails on the
// nil projection error.
func TestCanvasStillAbortsOnAHostFontSetFaceThatWillNotParse(t *testing.T) {
	const hostFace = "Host Supplied Face"
	source := embeddedChainDocText(t, `["`+hostFace+`"]`, "สัญญา")
	tpl, err := ParseTemplate([]byte(source))
	if err != nil {
		t.Fatalf("the document must LOAD — nothing about it is malformed; its chain simply names a face the CALLER supplies: %v", err)
	}

	fs := testShippedFontSet()
	fs[hostFace] = []byte("these bytes are not a font")

	// THE PRECONDITION THAT MAKES THE COMPARISON MEAN ANYTHING: the caller's
	// face fails at the same door, and in the same way, as the carried face
	// the test above now degrades for. If fontset.New ever started accepting
	// these bytes this test would pass while measuring nothing at all.
	if _, ferr := fontset.New(hostFace, fs[hostFace]); ferr == nil {
		t.Fatal("precondition: fontset.New accepted the fixture's bytes, so this document no longer exercises a face that will not parse")
	}

	// The geometry-only projection never shaped anything and always worked;
	// asserting it here is what makes the paint half's failure specific.
	if _, gerr := canvas(tpl); gerr != nil {
		t.Fatalf("the geometry projection must succeed: %v", gerr)
	}

	_, perr := canvasWithTextPaint(tpl, fs)
	if perr == nil {
		t.Fatal("the projection succeeded over a face THE CALLER supplied and this build cannot read. D-8.4.12 widened the canvas on the ATTRIBUTABILITY axis and explicitly did NOT absorb this case: a FontSet face is the host application's, not the document's, and no edit on the canvas repairs it. If the abort is to go, that is a ruling, not a scoping tweak")
	}
	// It aborts with the element located, which is the one thing that makes
	// the abort actionable.
	if !strings.Contains(perr.Error(), "canvas text element e1") {
		t.Errorf("the projection aborted without naming the element that caused it: %v", perr)
	}
	// AND IT IS THE AXIS THAT DECIDED IT, asserted directly rather than
	// inferred from the abort. A stamp on a caller's face would mean the
	// discriminator had collapsed back into "any face that will not parse" —
	// an error-type gate wearing a positional name — and every assertion above
	// would still pass on the day it did.
	var carried *template.CarriedFaceError
	if errors.As(perr, &carried) {
		t.Error("a face the CALLER's FontSet supplied was stamped as one the DOCUMENT carries: the attributability axis has collapsed into an error-type gate, and the next caller-supplied fault will degrade a document silently")
	}
}

// TestACarriedFaceFailureIsStampedOnEveryConsultationIncludingTheMemo closes
// the hole a memoized failure opens in a stamp applied at a door.
//
// THE HOLE. fontCache.get memoizes an embedded face's failure so a document
// with many elements does not re-base64 and re-parse the same asset once per
// element. A stamp applied only to the value RETURNED on the first
// consultation, and not to the value STORED, would hand the second element an
// unstamped error — so element 1 degrades, element 2 aborts the projection,
// and the disposition of one condition depends on how many elements happen to
// name the chain. That is invisible in every single-element fixture in this
// file.
//
// AND IT IS ASSERTED OVER BOTH ERROR SHAPES the embedded arm can produce,
// because "every error out of this arm is stamped" is the claim that makes the
// set closed by construction — the property an errors.As allowlist could not
// have:
//
//	decodeAt refuses the media type -> *template.UnsupportedFontMediaTypeError
//	                                   (the ONE type the old allowlist named)
//	fontset.New refuses the bytes   -> the type the old allowlist excluded,
//	                                   which is the defect D-8.4.12 repairs
//
// RED-PROOF: store the unstamped `err` in c.failedEmbedded (revert the memo
// half of the stamp) and the second consultation of each document fails here.
func TestACarriedFaceFailureIsStampedOnEveryConsultationIncludingTheMemo(t *testing.T) {
	unreadable, unreadableKey := unreadableCarriedFaceDoc(t)
	for _, tc := range []struct {
		name     string
		source   string
		assetKey string
	}{
		{"media type this build cannot read", nonFontChainDoc(t, `["Noto Sans", {"asset": "`+nonFontAssetKey+`"}]`, "สัญญา"), nonFontAssetKey},
		{"structurally valid sfnt this build cannot parse", unreadable, unreadableKey},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tpl, err := ParseTemplate([]byte(tc.source))
			if err != nil {
				t.Fatalf("the document must LOAD: %v", err)
			}
			cache := newDocumentFontCache(tpl).forChain("body")
			name := embeddedFaceName(tc.assetKey)
			fs := testShippedFontSet()

			first, ferr := cache.get(name, fs)
			if ferr == nil {
				t.Fatalf("precondition: the carried face resolved, so this fixture measures nothing (got %v)", first)
			}
			second, serr := cache.get(name, fs)
			if serr == nil {
				t.Fatalf("precondition: the SECOND consultation resolved a face the first refused (got %v)", second)
			}
			// The memo is what the second call must have come through — if it
			// re-derived the error the hole would not exist here and this test
			// would be measuring the first call twice.
			if len(cache.failedEmbedded) != 1 {
				t.Fatalf("precondition: the failure was not memoized (%d entries), so the second consultation did not exercise the memo", len(cache.failedEmbedded))
			}
			for i, err := range []error{ferr, serr} {
				var carried *template.CarriedFaceError
				if !errors.As(err, &carried) {
					t.Errorf("consultation %d returned an UNSTAMPED error (%T: %v) — the canvas would abort the whole projection for it while the other consultation degraded one element", i+1, err, err)
				}
			}
			// AND THE STAMP CHANGED NO TEXT — asserted against what it
			// WRAPS, not merely against its own second self. This is what
			// keeps every located string the render path already prints
			// byte-identical to what it printed before the stamp existed,
			// and it is the property that made the carrier safe to add to
			// an arm every render already walks.
			var stamped *template.CarriedFaceError
			if errors.As(ferr, &stamped) && stamped.Error() != stamped.Err.Error() {
				t.Errorf("the stamp added text of its own:\nstamped: %v\nwrapped: %v", stamped, stamped.Err)
			}
			if ferr.Error() != serr.Error() {
				t.Errorf("the two consultations print differently:\n first: %v\nsecond: %v", ferr, serr)
			}
			if !strings.Contains(ferr.Error(), "body") {
				t.Errorf("the stamp swallowed the located address the chain-scoped error carries: %v", ferr)
			}
		})
	}
}
