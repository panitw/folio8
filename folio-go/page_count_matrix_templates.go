package folio8

// Story 2.7, AC5 — epics.md's fourth Story 2.7 acceptance criterion,
// cited by ORDINAL rather than line range (this story's review,
// Finding 6: every epics.md:NNN-NNN line citation this story shipped
// was wrong — position-bound citations restale): documents of 1, 5, 20
// and 50 pages, spanning the page-9-to-page-10 digit boundary (finding
// 8, story creation: no existing fixture could express a page number
// whose digit count changes between page 9 and page 10 -- these four
// exist to fix that).
//
// EACH DOCUMENT'S SHAPE. N single-line content elements, each placed
// at y = i*728pt -- one content-band window height (727.89pt, this
// geometry's ContentHeight) plus a small margin -- so EVERY element
// lands in its own pagination window (D-2.6.1's sliding-window rule:
// "window N+1 begins at the top of the first item that did not fit in
// window N"). This is deterministic BY CONSTRUCTION, not tuned line-
// wrapping: N elements always produce exactly N pages, independent of
// font metrics. The page footer carries "Page {{page}} of {{pages}}"
// -- this story's construct -- and the page header a fixed literal, so
// a header/footer mix-up is readable in the text.
//
// Kept BYTE-IDENTICAL to fixtures/page-count-N/input.folio by hand,
// the same discipline multi_page_template.go documents: a test
// (page_count_matrix_test.go) asserts the pairs are equal before it
// asserts anything else.

const pageCount1TemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Marker 1", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {
      "elements": [
        {"id": "e2", "type": "text", "x": 0, "y": 6, "width": 480, "height": 16, "value": "Page {{page}} of {{pages}}", "style": {"fontFamily": "body", "fontSize": 8}}
      ],
      "height": 24
    },
    "pageHeader": {
      "elements": [
        {"id": "e3", "type": "text", "x": 0, "y": 4, "width": 480, "height": 16, "value": "PAGE COUNT MATRIX FIXTURE", "style": {"fontFamily": "body", "fontSize": 9}}
      ],
      "height": 18
    }
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 4,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

const pageCount5TemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Marker 1", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "text", "x": 0, "y": 728, "width": 200, "height": 20, "value": "Marker 2", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e3", "type": "text", "x": 0, "y": 1456, "width": 200, "height": 20, "value": "Marker 3", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e4", "type": "text", "x": 0, "y": 2184, "width": 200, "height": 20, "value": "Marker 4", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e5", "type": "text", "x": 0, "y": 2912, "width": 200, "height": 20, "value": "Marker 5", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {
      "elements": [
        {"id": "e6", "type": "text", "x": 0, "y": 6, "width": 480, "height": 16, "value": "Page {{page}} of {{pages}}", "style": {"fontFamily": "body", "fontSize": 8}}
      ],
      "height": 24
    },
    "pageHeader": {
      "elements": [
        {"id": "e7", "type": "text", "x": 0, "y": 4, "width": 480, "height": 16, "value": "PAGE COUNT MATRIX FIXTURE", "style": {"fontFamily": "body", "fontSize": 9}}
      ],
      "height": 18
    }
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 8,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

const pageCount20TemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Marker 1", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "text", "x": 0, "y": 728, "width": 200, "height": 20, "value": "Marker 2", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e3", "type": "text", "x": 0, "y": 1456, "width": 200, "height": 20, "value": "Marker 3", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e4", "type": "text", "x": 0, "y": 2184, "width": 200, "height": 20, "value": "Marker 4", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e5", "type": "text", "x": 0, "y": 2912, "width": 200, "height": 20, "value": "Marker 5", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e6", "type": "text", "x": 0, "y": 3640, "width": 200, "height": 20, "value": "Marker 6", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e7", "type": "text", "x": 0, "y": 4368, "width": 200, "height": 20, "value": "Marker 7", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e8", "type": "text", "x": 0, "y": 5096, "width": 200, "height": 20, "value": "Marker 8", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e9", "type": "text", "x": 0, "y": 5824, "width": 200, "height": 20, "value": "Marker 9", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e10", "type": "text", "x": 0, "y": 6552, "width": 200, "height": 20, "value": "Marker 10", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e11", "type": "text", "x": 0, "y": 7280, "width": 200, "height": 20, "value": "Marker 11", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e12", "type": "text", "x": 0, "y": 8008, "width": 200, "height": 20, "value": "Marker 12", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e13", "type": "text", "x": 0, "y": 8736, "width": 200, "height": 20, "value": "Marker 13", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e14", "type": "text", "x": 0, "y": 9464, "width": 200, "height": 20, "value": "Marker 14", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e15", "type": "text", "x": 0, "y": 10192, "width": 200, "height": 20, "value": "Marker 15", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e16", "type": "text", "x": 0, "y": 10920, "width": 200, "height": 20, "value": "Marker 16", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e17", "type": "text", "x": 0, "y": 11648, "width": 200, "height": 20, "value": "Marker 17", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e18", "type": "text", "x": 0, "y": 12376, "width": 200, "height": 20, "value": "Marker 18", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e19", "type": "text", "x": 0, "y": 13104, "width": 200, "height": 20, "value": "Marker 19", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e20", "type": "text", "x": 0, "y": 13832, "width": 200, "height": 20, "value": "Marker 20", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {
      "elements": [
        {"id": "e21", "type": "text", "x": 0, "y": 6, "width": 480, "height": 16, "value": "Page {{page}} of {{pages}}", "style": {"fontFamily": "body", "fontSize": 8}}
      ],
      "height": 24
    },
    "pageHeader": {
      "elements": [
        {"id": "e22", "type": "text", "x": 0, "y": 4, "width": 480, "height": 16, "value": "PAGE COUNT MATRIX FIXTURE", "style": {"fontFamily": "body", "fontSize": 9}}
      ],
      "height": 18
    }
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 75,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`

const pageCount50TemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Marker 1", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e2", "type": "text", "x": 0, "y": 728, "width": 200, "height": 20, "value": "Marker 2", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e3", "type": "text", "x": 0, "y": 1456, "width": 200, "height": 20, "value": "Marker 3", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e4", "type": "text", "x": 0, "y": 2184, "width": 200, "height": 20, "value": "Marker 4", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e5", "type": "text", "x": 0, "y": 2912, "width": 200, "height": 20, "value": "Marker 5", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e6", "type": "text", "x": 0, "y": 3640, "width": 200, "height": 20, "value": "Marker 6", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e7", "type": "text", "x": 0, "y": 4368, "width": 200, "height": 20, "value": "Marker 7", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e8", "type": "text", "x": 0, "y": 5096, "width": 200, "height": 20, "value": "Marker 8", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e9", "type": "text", "x": 0, "y": 5824, "width": 200, "height": 20, "value": "Marker 9", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e10", "type": "text", "x": 0, "y": 6552, "width": 200, "height": 20, "value": "Marker 10", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e11", "type": "text", "x": 0, "y": 7280, "width": 200, "height": 20, "value": "Marker 11", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e12", "type": "text", "x": 0, "y": 8008, "width": 200, "height": 20, "value": "Marker 12", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e13", "type": "text", "x": 0, "y": 8736, "width": 200, "height": 20, "value": "Marker 13", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e14", "type": "text", "x": 0, "y": 9464, "width": 200, "height": 20, "value": "Marker 14", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e15", "type": "text", "x": 0, "y": 10192, "width": 200, "height": 20, "value": "Marker 15", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e16", "type": "text", "x": 0, "y": 10920, "width": 200, "height": 20, "value": "Marker 16", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e17", "type": "text", "x": 0, "y": 11648, "width": 200, "height": 20, "value": "Marker 17", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e18", "type": "text", "x": 0, "y": 12376, "width": 200, "height": 20, "value": "Marker 18", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e19", "type": "text", "x": 0, "y": 13104, "width": 200, "height": 20, "value": "Marker 19", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e20", "type": "text", "x": 0, "y": 13832, "width": 200, "height": 20, "value": "Marker 20", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e21", "type": "text", "x": 0, "y": 14560, "width": 200, "height": 20, "value": "Marker 21", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e22", "type": "text", "x": 0, "y": 15288, "width": 200, "height": 20, "value": "Marker 22", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e23", "type": "text", "x": 0, "y": 16016, "width": 200, "height": 20, "value": "Marker 23", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e24", "type": "text", "x": 0, "y": 16744, "width": 200, "height": 20, "value": "Marker 24", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e25", "type": "text", "x": 0, "y": 17472, "width": 200, "height": 20, "value": "Marker 25", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e26", "type": "text", "x": 0, "y": 18200, "width": 200, "height": 20, "value": "Marker 26", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e27", "type": "text", "x": 0, "y": 18928, "width": 200, "height": 20, "value": "Marker 27", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e28", "type": "text", "x": 0, "y": 19656, "width": 200, "height": 20, "value": "Marker 28", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e29", "type": "text", "x": 0, "y": 20384, "width": 200, "height": 20, "value": "Marker 29", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e30", "type": "text", "x": 0, "y": 21112, "width": 200, "height": 20, "value": "Marker 30", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e31", "type": "text", "x": 0, "y": 21840, "width": 200, "height": 20, "value": "Marker 31", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e32", "type": "text", "x": 0, "y": 22568, "width": 200, "height": 20, "value": "Marker 32", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e33", "type": "text", "x": 0, "y": 23296, "width": 200, "height": 20, "value": "Marker 33", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e34", "type": "text", "x": 0, "y": 24024, "width": 200, "height": 20, "value": "Marker 34", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e35", "type": "text", "x": 0, "y": 24752, "width": 200, "height": 20, "value": "Marker 35", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e36", "type": "text", "x": 0, "y": 25480, "width": 200, "height": 20, "value": "Marker 36", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e37", "type": "text", "x": 0, "y": 26208, "width": 200, "height": 20, "value": "Marker 37", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e38", "type": "text", "x": 0, "y": 26936, "width": 200, "height": 20, "value": "Marker 38", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e39", "type": "text", "x": 0, "y": 27664, "width": 200, "height": 20, "value": "Marker 39", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e40", "type": "text", "x": 0, "y": 28392, "width": 200, "height": 20, "value": "Marker 40", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e41", "type": "text", "x": 0, "y": 29120, "width": 200, "height": 20, "value": "Marker 41", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e42", "type": "text", "x": 0, "y": 29848, "width": 200, "height": 20, "value": "Marker 42", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e43", "type": "text", "x": 0, "y": 30576, "width": 200, "height": 20, "value": "Marker 43", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e44", "type": "text", "x": 0, "y": 31304, "width": 200, "height": 20, "value": "Marker 44", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e45", "type": "text", "x": 0, "y": 32032, "width": 200, "height": 20, "value": "Marker 45", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e46", "type": "text", "x": 0, "y": 32760, "width": 200, "height": 20, "value": "Marker 46", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e47", "type": "text", "x": 0, "y": 33488, "width": 200, "height": 20, "value": "Marker 47", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e48", "type": "text", "x": 0, "y": 34216, "width": 200, "height": 20, "value": "Marker 48", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e49", "type": "text", "x": 0, "y": 34944, "width": 200, "height": 20, "value": "Marker 49", "style": {"fontFamily": "body", "fontSize": 12}},
        {"id": "e50", "type": "text", "x": 0, "y": 35672, "width": 200, "height": 20, "value": "Marker 50", "style": {"fontFamily": "body", "fontSize": 12}}
      ]
    },
    "pageFooter": {
      "elements": [
        {"id": "e51", "type": "text", "x": 0, "y": 6, "width": 480, "height": 16, "value": "Page {{page}} of {{pages}}", "style": {"fontFamily": "body", "fontSize": 8}}
      ],
      "height": 24
    },
    "pageHeader": {
      "elements": [
        {"id": "e52", "type": "text", "x": 0, "y": 4, "width": 480, "height": 16, "value": "PAGE COUNT MATRIX FIXTURE", "style": {"fontFamily": "body", "fontSize": 9}}
      ],
      "height": 18
    }
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 183,
  "page": {"margin": {"bottom": 42, "left": 36, "right": 54, "top": 30}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.0"
}
`
