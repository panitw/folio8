package folio8

// Story 2.6's FR44 overflow diagnostic, exercised through the PUBLIC
// folio8.Render on SYNTHETIC TEMPLATES.
//
// WHY SYNTHETIC TEMPLATES AND NOT A FIXTURE (D-2.6.5's guardrail, and
// D-000.50's finding): no committed fixture can express this case, and none
// should be distorted to. fixtures/multi-page/ deliberately contains no
// element taller than the content window — a golden a human is asked to read
// should be a document a human would recognise, not a pathology. The
// pathology belongs here, where it can be stated as an error assertion
// rather than as bytes.
//
// WHY BOTH SUBJECTS ARE EXERCISED. D-2.6.1's disposition is "one overflow
// rule, two subjects". Exercising only one would assert that slogan while
// only one subject could ever express it — the same vacuity D-000.50 is
// about, one level down. So there is a row for a LINE taller than the window
// (reachable with a font size larger than the content band) and a row for an
// IMAGE whose DECLARED BOX is taller than it.
//
// AND A THIRD SUBJECT ARRIVED WITH STORY 9.1 WITHOUT A ROW OF ITS OWN: an
// element BOX (style.background / style.border on a text, image, rect or
// line element) whose declared height exceeds the window. Story 7.10 adds
// it, and the row exists for the WORD as much as for the disposition.
// Measured at that story's baseline, an over-tall rect was announced to its
// author verbatim as "element e1: table is taller than the content window"
// — because collectElementBoxRects is a FOURTH, UNGROUPED tableRectSource
// that paginate.go's own comment enumerated away, and the Kind derivation
// therefore called every ungrouped Rects item a table (DW-47, D-7.10.5).
// The document contains no table. A located diagnostic that names the
// element and MISNAMES the thing sends its reader looking for a table they
// never declared, so this row asserts the word rather than only observing
// it.
//
// WHY THE ASSERTION IS ON THE FIELDS, NEVER ON "an error occurred". An
// unlocated overflow message is what D-1.8.1 exists to prevent: a template
// author is expected to act on this, so the diagnostic must name WHICH
// element and WHICH kind. "Render returned non-nil" is satisfied by a
// missing-font error, a malformed-asset error, and by a panic recovered into
// an error — none of which are this.
//
// WHY IT IS AN ERROR RATHER THAN A CLIP (D-2.6.5). D-2.6.1's original
// amendment said "clip at the window bottom", and that disposition is
// WITHDRAWN. Its own text says "FR44's overflow machinery concerns content
// exceeding its BOX, not the page edge", so invoking FR44's clip-and-continue
// behaviour for a PAGE-EDGE case contradicted the same ruling. The ground
// that makes "one rule, two subjects" literally true is that both subjects
// are DECLARATION-LEVEL, not render-time: per D-2.4.2 constraint 1 leading is
// a function of the declared font stack and font size and never of the glyphs
// that appear, and the window is page height minus declared margins and band
// heights — so "some line is taller than the window" is decidable from
// template plus fontset WITH NO DATA AT ALL, exactly as the image's declared
// box is. Two declaration-level impossibilities, one disposition.

import (
	"errors"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/layout"
)

// The synthetic geometry, shared by both rows so the two subjects differ in
// nothing but the element that overflows.
//
//	A4 height 841890, margin.top 30000, margin.bottom 42000,
//	pageHeader.height 18000, pageFooter.height 24000
//	-> content window = 841890 − 30000 − 42000 − 18000 − 24000 = 727890 mp
//	                  = 727.89 pt
const overflowContentWindowMP = 727890

// overflowLineTemplate declares ONE text element at a font size whose LINE is
// taller than the whole content window.
//
// At 900pt on the Noto Sans chain the line is `advance` = 900 * 1362/1000 =
// 1,225,800 mp tall, against a 727,890 mp window. Nothing about the DATA can
// change that — which is the point: it is decidable from the template and the
// fontset alone.
const overflowLineTemplate = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 480, "height": 20, "value": "X", "style": {"fontFamily": "body", "fontSize": 900}}
      ]
    },
    "pageFooter": {"elements": [], "height": 24},
    "pageHeader": {"elements": [], "height": 18}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// overflowImageTemplate declares ONE image element whose DECLARED BOX is
// 800pt tall — 800,000 mp against the 727,890 mp window.
//
// The box, not the drawn image. AD-24 already scales the image to fit its
// box, so "does it fit on a page" is a question about what the TEMPLATE
// declared, which is why this is a template error with a located message
// rather than a render-time surprise. The asset is the 3x2 PNG
// fixtures/image-embed/ uses; its pixel dimensions are irrelevant here and
// that is exactly the property being asserted.
const overflowImageTemplate = `{
  "assets": {
    "5a05ad01e89c143b7061b0c93450566568d38a23da9b9c5c9dfe449016433078": {"data": ["iVBORw0KGgoAAAANSUhEUgAAAAMAAAACCAIAAAASFvFNAAAAGElEQVR42mL6z8DAAMZMEOo/AwMg", "AAD//zwUBf/NjsW5AAAAAElFTkSuQmCC"], "mediaType": "image/png"}
  },
  "bands": {
    "content": {
      "elements": [
        {"id": "e2", "type": "image", "asset": "5a05ad01e89c143b7061b0c93450566568d38a23da9b9c5c9dfe449016433078", "x": 0, "y": 0, "width": 100, "height": 800}
      ]
    },
    "pageFooter": {"elements": [], "height": 24},
    "pageHeader": {"elements": [], "height": 18}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// overflowElementBoxTemplate declares ONE rect element whose declared box
// is 800pt tall — 800,000 mp against the 727,890 mp window.
//
// style.background is what makes it reach pagination at all: without a
// background and without a border, elementDeclaresBox (element_box.go) is
// false, no tableRectSource is built, and the element is as tall as it
// likes with nothing to say about it. So the declaration below is
// load-bearing, not decoration.
const overflowElementBoxTemplate = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e3", "type": "rect", "x": 0, "y": 0, "width": 100, "height": 800, "style": {"background": "#1b2a4a"}}
      ]
    },
    "pageFooter": {"elements": [], "height": 24},
    "pageHeader": {"elements": [], "height": 18}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// TestRenderReportsAnItemThatFitsOnNoPage is FR44's located diagnostic,
// asserted through the public API on every subject.
func TestRenderReportsAnItemThatFitsOnNoPage(t *testing.T) {
	for _, row := range []struct {
		name          string
		template      string
		wantElementID string
		wantKind      string
		wantHeightMin int64  // the item must genuinely exceed the window
		forbiddenWord string // a word the message must NOT contain
	}{
		{
			name:          "a LINE taller than the content window",
			template:      overflowLineTemplate,
			wantElementID: "e1",
			wantKind:      "line",
			wantHeightMin: overflowContentWindowMP,
		},
		{
			name:          "an IMAGE whose DECLARED BOX is taller than the content window",
			template:      overflowImageTemplate,
			wantElementID: "e2",
			wantKind:      "image",
			wantHeightMin: overflowContentWindowMP,
		},
		{
			// Story 7.10 / D-7.10.5. The disposition was already
			// right at baseline; the WORD was not.
			name:          "an ELEMENT BOX whose DECLARED HEIGHT is taller than the content window",
			template:      overflowElementBoxTemplate,
			wantElementID: "e3",
			wantKind:      "box",
			wantHeightMin: overflowContentWindowMP,
			forbiddenWord: "table",
		},
	} {
		t.Run(row.name, func(t *testing.T) {
			tpl, perr := ParseTemplate([]byte(row.template))
			if perr != nil {
				t.Fatalf("presence precondition: the synthetic template does not even parse: %v", perr)
			}

			// PRESENCE PRECONDITION: the document's content window really is
			// the one the row's expectation was derived from. Without it, a
			// geometry typo would make the row assert an overflow that
			// happens for the wrong reason.
			g, gerr := pageGeometryOf(tpl)
			if gerr != nil {
				t.Fatalf("pageGeometryOf: %v", gerr)
			}
			if got := layout.ContentHeight(g); int64(got) != overflowContentWindowMP {
				t.Fatalf("presence precondition: the synthetic geometry's content window is %d mp, not the %d the expectation was derived from", got, overflowContentWindowMP)
			}

			_, err := Render(tpl, Data("{}"), nil, testShippedFontSet())
			if err == nil {
				t.Fatal("Render accepted a document containing an item that fits on no page. It must be a located diagnostic: never a straddle, never a silent clip, and never a split line.")
			}

			var overflow *layout.OverflowError
			if !errors.As(err, &overflow) {
				t.Fatalf("Render returned %v (%T); FR44's page-edge diagnostic must be a *layout.OverflowError.\n"+
					"\"an error occurred\" is also satisfied by a missing face, a malformed asset, and by a panic "+
					"recovered into an error — none of which are this.", err, err)
			}
			if overflow.ElementID != row.wantElementID {
				t.Errorf("the diagnostic names element %q; the offending element is %q — an UNLOCATED overflow message is what D-1.8.1 exists to prevent", overflow.ElementID, row.wantElementID)
			}
			if overflow.Kind != row.wantKind {
				t.Errorf("the diagnostic calls the item a %q; it is a %q. Both subjects go through ONE rule, so the kind is the only thing that distinguishes them in the message.", overflow.Kind, row.wantKind)
			}
			if int64(overflow.ItemHeight) <= row.wantHeightMin {
				t.Errorf("the diagnostic reports an item height of %d mp, which does NOT exceed the %d mp window — this row is not exercising an overflow at all", overflow.ItemHeight, row.wantHeightMin)
			}
			if int64(overflow.ContentHeight) != overflowContentWindowMP {
				t.Errorf("the diagnostic reports a content height of %d mp; the document's is %d", overflow.ContentHeight, overflowContentWindowMP)
			}

			// The message a human reads must name the element, or the located
			// half of "a located message" is not in the artifact the reader
			// actually gets.
			if !strings.Contains(err.Error(), row.wantElementID) {
				t.Errorf("the rendered error message %q does not contain the element id %q", err.Error(), row.wantElementID)
			}
			// THE KIND AT ITS REAL POSITION, never merely somewhere in
			// the string. `strings.Contains(err.Error(), row.wantKind)`
			// CANNOT FAIL here and never could: the constant advice
			// sentence every OverflowError carries contains "line" ("a
			// line is never split across a page boundary") and "box"
			// ("its declared box height"), so the check passed for every
			// row whatever Kind actually held — a vacuous assertion, not
			// a weak one, and one that would have watched D-7.10.5's own
			// misnaming go by. The message OPENS "element <id>: <kind>
			// is taller than the content window", and that opening is
			// the one place the kind is the message's SUBJECT.
			wantOpening := "element " + row.wantElementID + ": " + row.wantKind + " is taller than the content window"
			if !strings.Contains(err.Error(), wantOpening) {
				t.Errorf("the rendered error message does not open %q:\n%s\n\nThe kind must reach the reader as the message's own subject; finding it somewhere inside the advice sentence is not the same claim.", wantOpening, err.Error())
			}
			if row.forbiddenWord != "" && strings.Contains(err.Error(), row.forbiddenWord) {
				t.Errorf("the rendered error message calls this element a %q:\n%s\n\nThe document declares no %s. Kind is DERIVED (overflowKind, internal/layout/paginate.go) — fix the derivation, never the string.", row.forbiddenWord, err.Error(), row.forbiddenWord)
			}
		})
	}
}

// TestRenderAcceptsAnItemThatExactlyFitsTheWindow is the NEGATIVE CONTROL for
// the test above, and it is here because D-000.34 (extended) says a fix can
// silently destroy a negative control — a control passing is what a control
// does, so its death is invisible.
//
// Without it, an implementation that rejected EVERY document would satisfy
// every assertion in TestRenderReportsAnItemThatFitsOnNoPage except the
// element id, and the two rows would still read as a well-targeted pair.
func TestRenderAcceptsAnItemThatExactlyFitsTheWindow(t *testing.T) {
	// The multi-page fixture is the nearest document that genuinely
	// paginates, and it renders. If the overflow branch were over-eager, this
	// would fail.
	if _, err := Render(parseMultiPageTemplate(t), Data("{}"), nil, testShippedFontSet()); err != nil {
		t.Fatalf("the multi-page fixture, which paginates but contains no oversized item, was REJECTED: %v\n"+
			"The overflow diagnostic must fire only for an item that fits in NO window — not for one that "+
			"merely fails to fit the CURRENT one, which is ordinary pagination.", err)
	}
}
