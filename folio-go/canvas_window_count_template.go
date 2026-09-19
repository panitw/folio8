package folio8

// Story 7.5 — the FIRST multi-page CANVAS fixtures.
//
// WHY THEY HAD TO BE NEW. No canvas or projection fixture in this repository
// was multi-page, and none COULD be: containComponent's one-window cap made a
// template that placed anything below the foot of page one unauthorable, and
// multi_page_fixture_test.go, statement_fixture_test.go and
// page_count_matrix_test.go never call Canvas or CanvasWithTextPaint at all.
// Every canvas assertion about pagination was therefore vacuous by
// construction, which is how the closed form these two fixtures exist to
// refuse could have survived unnoticed.
//
// WHY fixtures/page-count-{1,5,20,50} COULD NOT BE REUSED. Their elements sit
// exactly one window apart, which makes `ceil(lowestBottom / H)` and the true
// slide count COINCIDE at every N. They exercise the code without
// discriminating it. Discriminating needs a GAP.
//
// THE GEOMETRY, shared by both constants and identical to the page-count
// matrix's: A4 portrait, margins {top 30, right 54, bottom 42, left 36}, page
// header 18, page footer 24. So one content window is
//
//	841890 − 30000 − 42000 − 18000 − 24000 = 727890 millipoints (727.89pt)
//
// which internal/layout's ContentHeight derives and CanvasProjection reports
// as contentWindowHeight.

// canvasWindowCountGapTemplateJSON is the DISCRIMINATING fixture: one line of
// text at the top of the column, and a second element declared TEN WINDOWS
// BELOW it with nothing in between.
//
// The right answer is TWO. The window advances to the top of the first item
// that did not fit, so the gap costs nothing: page one holds the text, page
// two starts at the rect. The closed form paginate.go forbids by name answers
// ELEVEN — nine of them blank pages that no document has — so this fixture
// red-proves that exact spelling rather than merely exercising the count.
//
// The rect is STYLED, and Story 7.9 is why. It used to carry no style, on the
// since-corrected claim that the canvas counts a column "as the canvas paints
// it" and therefore occupies a window for a box the printed document has
// nothing on. The render path places an element box only where a background
// or a border is declared (element_box.go), so an unstyled rect ten windows
// down produced a canvas that drew a sheet the document does not print — and
// the canvas's own createComponent gives a new rect a border for the same
// reason, so an unstyled one was never the authored norm either. A background
// keeps the gap real on BOTH sides, which is what this fixture is for.
const canvasWindowCountGapTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Opening balance", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "rect", "x": 0, "y": 7280, "width": 200, "height": 20, "style": {"background": "#eeeeee"}}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 24
    },
    "pageHeader": {
      "elements": [],
      "height": 18
    }
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// canvasWindowCountControlTemplateJSON is the NEGATIVE CONTROL: three
// elements one window apart, at y = 0, 728 and 1456 points.
//
// Both routes answer THREE here — the slide and the closed form agree,
// exactly as they agree on every page-count-N fixture. It is in the suite for
// two reasons: without it the discriminating test could pass by always
// answering two, and with it the record says out loud WHY the forbidden
// spelling survived this long. One-window spacing hides the defect.
const canvasWindowCountControlTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Window one", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "text", "x": 0, "y": 728, "width": 200, "height": 20, "value": "Window two", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e3", "type": "text", "x": 0, "y": 1456, "width": 200, "height": 20, "value": "Window three", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 24
    },
    "pageHeader": {
      "elements": [],
      "height": 18
    }
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// canvasWindowCountOversizedTemplateJSON declares a content component TALLER
// THAN ONE WINDOW — 900pt in a 727.89pt column. Story 7.5 newly makes such a
// component authorable, so layout.Paginate's OverflowError is newly reachable
// from the canvas, and the projection must degrade the COUNT rather than fail
// and blank the canvas.
const canvasWindowCountOversizedTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "rect", "x": 0, "y": 0, "width": 200, "height": 900, "style": {"background": "#eeeeee"}}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 24
    },
    "pageHeader": {
      "elements": [],
      "height": 18
    }
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// canvasWindowCountBoundTableTemplateJSON is the FLOOR, recorded rather than
// discovered later: a table bound to a collection the canvas has never been
// given. projectedSize returns a table's header height and no rows, because
// the canvas has no data — so the column the canvas paints is one header tall
// however many hundred rows the finished statement runs to.
const canvasWindowCountBoundTableTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "table", "x": 0, "y": 0, "as": "row", "bind": "transactions[]", "headerHeight": 16, "columns": [{"id": "e2", "label": "Date", "width": 80, "align": "left", "bind": "{{row.date}}"}], "style": {"fontFamily": "body", "fontSize": 8}}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 24
    },
    "pageHeader": {
      "elements": [],
      "height": 18
    }
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// canvasWindowCountUnshapedTextTemplateJSON is the floor's THIRD cause, the
// one neither a table nor an over-tall box produces: a content text element
// that HAS a value and no style.fontFamily to resolve a chain from. fontChain
// refuses it, addCanvasTextPaint degrades that one element to an empty paint
// — the pre-existing, deliberate behaviour, because such a document is
// structurally valid and still loadable — and the extents its lines would
// have contributed are simply missing from the column the count measures.
//
// The second element is shaped normally, so the fixture discriminates: the
// column is counted, it is just counted short.
const canvasWindowCountUnshapedTextTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "No chain resolves this"},
        {"id": "e2", "type": "text", "x": 0, "y": 40, "width": 200, "height": 20, "value": "Shaped normally", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 24
    },
    "pageHeader": {
      "elements": [],
      "height": 18
    }
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// canvasWindowCountUnshapedHeaderTemplateJSON is the SAME defect in a band the
// window count does not measure. The page header carries the text with no
// resolvable chain; the content band is an ordinary, exactly counted column.
//
// The floor flag is a statement about the CONTENT COLUMN, so this template
// must report it false. Without the `band.name == bandContent` guard on the
// degradation site the whole Go suite still passed, and every document with an
// unshapeable header title would have told the author that it prints more
// pages than are drawn — a false claim, on a count that is exact.
const canvasWindowCountUnshapedHeaderTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Shaped normally", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 24
    },
    "pageHeader": {
      "elements": [
        {"id": "e2", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "No chain resolves this"}
      ],
      "height": 18
    }
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// canvasWindowCountGroupedTemplateJSON is Story 7.9's COUNT discriminator:
// the case where a declared keep-together group changes how many windows the
// content column occupies, not merely where one of them begins.
//
// THE ARITHMETIC, on this file's shared geometry (one window = 727890
// millipoints). A body line sits at the top of the column. A two-member group
// straddles the first window's ceiling — `e2`'s text at y 700 is inside it,
// `e3`'s ruled line at y 740 … 760 is past it — and an UNTAGGED tail element
// sits at y 1440.
//
//	TAGGED:   the group does not fit window one, so the window slides to the
//	          group's EARLIEST top, 700000. Window two is then
//	          [700000, 1427890), which the tail at 1440000 misses, so it
//	          starts a THIRD window at 1440000. Count 3, origins
//	          [0 700000 1440000].
//	UNTAGGED: `e2` fits window one on its own and only `e3` falls out, so
//	          window two begins at 740000 and spans [740000, 1467890) — which
//	          the tail at 1440000 fits. Count 2, origins [0 740000].
//
// So the tags move the COUNT (3 against 2) and every origin after the first.
// Before Story 7.9 the canvas built its column items ungrouped and answered
// the untagged numbers for the tagged document, while reporting the count
// EXACT — a confidently wrong claim, which is worse than the silence Story
// 7.6 replaced.
//
// The ruled line is a `rect` and it is STYLED on purpose. A styled box is one
// the render path also places (element_box.go), so the two sides are
// comparable item for item — and because the group's members are one TEXT
// element and one NON-TEXT element, the equality cannot pass unless BOTH of
// the canvas's column-item arms carry their groups. Tagging only the text arm
// leaves `e3` loose and the count falls back to 2.
const canvasWindowCountGroupedTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Opening balance", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "text", "x": 0, "y": 700, "width": 240, "height": 20, "keepTogether": "signature", "value": "Signed for the Company", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e3", "type": "rect", "x": 0, "y": 740, "width": 240, "height": 20, "keepTogether": "signature", "style": {"background": "#000000"}},
        {"id": "e4", "type": "text", "x": 0, "y": 1440, "width": 240, "height": 20, "value": "Continued overleaf", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 24
    },
    "pageHeader": {
      "elements": [],
      "height": 18
    }
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 5,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.2"
}
`

// canvasWindowCountGroupedUngroupedTemplateJSON is the twin: the document
// above with its two `"keepTogether": "signature", ` tags REMOVED and nothing
// else changed, asserted mechanically by
// TestCanvasGroupedTwinDiffersOnlyByTheTags.
//
// It is the control, on fixtures/keep-together/'s own pattern. "The canvas
// counts three windows" is a fact many unrelated implementations produce;
// "three HERE and two THERE, for a pair differing in exactly two tags" is
// not. It declares 1.2 as well: a version is never LOWERED to make a twin
// look tidier, and the pair must differ in exactly one respect.
const canvasWindowCountGroupedUngroupedTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Opening balance", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "text", "x": 0, "y": 700, "width": 240, "height": 20, "value": "Signed for the Company", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e3", "type": "rect", "x": 0, "y": 740, "width": 240, "height": 20, "style": {"background": "#000000"}},
        {"id": "e4", "type": "text", "x": 0, "y": 1440, "width": 240, "height": 20, "value": "Continued overleaf", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 24
    },
    "pageHeader": {
      "elements": [],
      "height": 18
    }
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 5,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.2"
}
`

// canvasWindowCountGroupedUnstyledTemplateJSON and its untagged twin below are
// the OTHER half of Story 7.9's four-row matrix: the same pair again with the
// ruled line's `"style": {"background": "#000000"}` REMOVED and nothing else
// changed.
//
// An unstyled rect prints nothing, and the render path knows it — element_box.
// go builds a rect source only for an element declaring a background or a
// border, so an unstyled one reaches no content-column item at all. The canvas
// used to contribute one for every non-text component whatever its style,
// which put a box in the column that the printed document has nothing in. That
// was a wrong ORIGIN while the two sides were both ungrouped, and it became a
// wrong COUNT the moment tagging made the canvas's partition matter: the group
// slid for a member the render never places.
//
//	TAGGED + UNSTYLED:   the group's only placeable member is `e2` at y 700,
//	                     which fits window one on its own, so the tail at
//	                     1440000 starts window two. Count 2.
//	UNTAGGED + UNSTYLED: the same three placeable items, the same answer.
//
// So this pair is the row that must NOT move when the tags go on — the
// discrimination the styled pair cannot make, because there the tags legitimately
// change the count.
const canvasWindowCountGroupedUnstyledTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Opening balance", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "text", "x": 0, "y": 700, "width": 240, "height": 20, "keepTogether": "signature", "value": "Signed for the Company", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e3", "type": "rect", "x": 0, "y": 740, "width": 240, "height": 20, "keepTogether": "signature"},
        {"id": "e4", "type": "text", "x": 0, "y": 1440, "width": 240, "height": 20, "value": "Continued overleaf", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 24
    },
    "pageHeader": {
      "elements": [],
      "height": 18
    }
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 5,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.2"
}
`

// canvasWindowCountGroupedUngroupedUnstyledTemplateJSON is that document with
// its two tags removed — the fourth cell of the matrix, and the control that
// says the unstyled rows agree with each other as well as with the render.
const canvasWindowCountGroupedUngroupedUnstyledTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Opening balance", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "text", "x": 0, "y": 700, "width": 240, "height": 20, "value": "Signed for the Company", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e3", "type": "rect", "x": 0, "y": 740, "width": 240, "height": 20},
        {"id": "e4", "type": "text", "x": 0, "y": 1440, "width": 240, "height": 20, "value": "Continued overleaf", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 24
    },
    "pageHeader": {
      "elements": [],
      "height": 18
    }
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 5,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.2"
}
`

// canvasWindowCountConditionalTemplateJSON is the register's cause (d), the
// one the canvas GENUINELY CANNOT KNOW: `e3` carries `visibleIf`, so whether
// it is placed is a property of the DATA, and the canvas has none.
//
// Measured, on the same geometry as the pair above: the canvas answers 3 with
// no data at all, while the real render answers 3 with `{"showRule": true}`
// and 2 with `{"showRule": false}` — AD-24 makes a hidden element absent with
// NO GAP, so the group's slide simply does not happen. The canvas is therefore
// a CEILING here, which is the direction the field this replaced could not
// state, and the reason its name had to change.
//
// The tag is on the fixture because grouping is how the case was found and it
// makes the divergence loud. It is not what causes it: an UNGROUPED visibleIf
// element diverges the same way, and has since Story 7.5 shipped the count.
const canvasWindowCountConditionalTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Opening balance", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "text", "x": 0, "y": 700, "width": 240, "height": 20, "keepTogether": "signature", "value": "Signed for the Company", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e3", "type": "rect", "x": 0, "y": 740, "width": 240, "height": 20, "keepTogether": "signature", "visibleIf": "showRule", "style": {"background": "#000000"}},
        {"id": "e4", "type": "text", "x": 0, "y": 1440, "width": 240, "height": 20, "value": "Continued overleaf", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 24
    },
    "pageHeader": {
      "elements": [],
      "height": 18
    }
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 5,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.2"
}
`

// THE PLACEMENT PAIRS (Story 7.9's review pass). Each is one document and a
// twin differing in EXACTLY the substring that decides whether the render
// path places a content-column item for its second element — and in nothing
// else, asserted mechanically in canvas_window_count_test.go, on
// fixtures/keep-together/'s pattern. In each pair the second element sits at
// y 1440, two windows down, so its placement is worth a whole sheet: the
// document that places it occupies two windows and its twin one. "The canvas
// answers one" is a fact many wrong implementations produce; "one HERE and
// two THERE, for a pair differing in one substring" is not.
//
// They exist because canvasElementIsPlaced's first version exempted images
// and tables UNCONDITIONALLY, on the claim that those two kinds are "always
// placed". Neither is.

// canvasWindowCountUnfilledImageTemplateJSON is an image element with NO FILE
// CHOSEN — `"asset": null`, which createComponent gives every image the
// designer drops, so this is the ordinary state of a newly placed image and
// not an exotic one. collectImageRuns returns no run for it, and it declares
// no box either, so the render path places nothing for it and the document
// prints ONE page.
//
// The asset the twin below references is declared here too and simply goes
// unreferenced (an orphan asset is preserved through parse and serialize by
// design), so the pair differs in the asset VALUE alone.
const canvasWindowCountUnfilledImageTemplateJSON = `{
  "assets": {
    "5a05ad01e89c143b7061b0c93450566568d38a23da9b9c5c9dfe449016433078": {
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
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Opening balance", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "image", "x": 0, "y": 1440, "width": 240, "height": 20, "asset": null}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 24
    },
    "pageHeader": {
      "elements": [],
      "height": 18
    }
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// canvasWindowCountFilledImageTemplateJSON is that document with a FILE
// CHOSEN and nothing else changed: the same image box now names the 3x2 PNG
// declared in its assets map, collectImageRuns returns a run for it, and the
// document prints TWO pages. It is the discriminator — without it, "the
// canvas answers one for the unfilled image" would be satisfied by a canvas
// that never counted an image at all.
const canvasWindowCountFilledImageTemplateJSON = `{
  "assets": {
    "5a05ad01e89c143b7061b0c93450566568d38a23da9b9c5c9dfe449016433078": {
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
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Opening balance", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "image", "x": 0, "y": 1440, "width": 240, "height": 20, "asset": "5a05ad01e89c143b7061b0c93450566568d38a23da9b9c5c9dfe449016433078"}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 24
    },
    "pageHeader": {
      "elements": [],
      "height": 18
    }
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// canvasWindowCountFlatRectTemplateJSON is a STYLED rect with NO HEIGHT.
// element_box.go's rule has two clauses, not one — a declared background or
// border AND a rectangle with area — and this document satisfies only the
// first. The loader accepts a zero rectangle (element_box.go's own comment
// names it as the reachable case), nothing is drawn for it, and the document
// prints ONE page. A canvas gating on the style clause alone counts a second
// window here and calls the count exact.
const canvasWindowCountFlatRectTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Opening balance", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "rect", "x": 0, "y": 1440, "width": 240, "height": 0, "style": {"background": "#000000"}}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 24
    },
    "pageHeader": {
      "elements": [],
      "height": 18
    }
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// canvasWindowCountTallRectTemplateJSON is that document with a rectangle
// that has area and nothing else changed: both clauses hold, the render path
// draws the box, and the document prints TWO pages.
const canvasWindowCountTallRectTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Opening balance", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "rect", "x": 0, "y": 1440, "width": 240, "height": 20, "style": {"background": "#000000"}}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 24
    },
    "pageHeader": {
      "elements": [],
      "height": 18
    }
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 3,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// canvasWindowCountColumnlessTableTemplateJSON is a table declaring NO
// COLUMNS. internal/template/parse_bands.go imposes no minimum, so
// `"columns": []` loads; collectBandTableRuns skips such a table outright
// ("nothing to lay out or draw"), so it reaches no content-column item and
// the document prints ONE page.
//
// Both templates in this pair carry a content-band table with a non-empty
// binding, so their projected count is NOT exact — cause (a) applies whatever
// the columns say. That is why this pair is asserted in a test of its own
// rather than as a row of the placement matrix, and why both are rendered
// against `{"transactions": []}`: an absent binding is a render error.
const canvasWindowCountColumnlessTableTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Opening balance", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "table", "x": 0, "y": 1440, "as": "row", "bind": "transactions[]", "headerHeight": 16, "columns": [], "style": {"fontFamily": "body", "fontSize": 8}}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 24
    },
    "pageHeader": {
      "elements": [],
      "height": 18
    }
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// canvasWindowCountOneColumnTableTemplateJSON is that document with ONE
// COLUMN and nothing else changed: the table now has a header row to draw, it
// contributes its header rect to the content column, and the document prints
// TWO pages.
const canvasWindowCountOneColumnTableTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Opening balance", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "table", "x": 0, "y": 1440, "as": "row", "bind": "transactions[]", "headerHeight": 16, "columns": [{"id": "e3", "label": "Date", "width": 80, "align": "left", "bind": "{{row.date}}"}], "style": {"fontFamily": "body", "fontSize": 8}}
      ]
    },
    "pageFooter": {
      "elements": [],
      "height": 24
    },
    "pageHeader": {
      "elements": [],
      "height": 18
    }
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

// canvasWindowCountStyledTextBoxTemplateJSON is E9-3's fixture: ONE
// content-band text element at y 700 with a declared height of 200 and a
// one-line value, carrying a background so it declares a box.
//
// It is the measured divergence in its smallest form. The render path
// places TWO column items for this element — its shaped line, and the
// declared box element_box.go builds for it — so it spans two windows;
// the canvas placed only the line, counted ONE window, and set
// ContentWindowCountIsExact to true over it. The comment in
// addCanvasWindowCount described that divergence while the value denied
// it.
const canvasWindowCountStyledTextBoxTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 700, "width": 200, "height": 200, "value": "One line", "style": {"fontFamily": "body", "fontSize": 12, "background": "#eeeeee"}}
      ]
    },
    "pageFooter": {"elements": [], "height": 24},
    "pageHeader": {"elements": [], "height": 18}
  },
  "fonts": {"body": ["Roboto-Regular"]},
  "locale": "en",
  "nextId": 2,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
