package folio8

// multiPageTemplateJSON is Story 2.6's multi-page composition fixture, kept
// BYTE-IDENTICAL to fixtures/multi-page/input.folio by hand (font-text's,
// wrapped-text's and three-band-page's precedent —
// TestMultiPageGoldenFixtureMatchesTheInRepoTemplate asserts the two are
// equal before it asserts anything else).
//
// WHAT THIS DOCUMENT'S CONTENT CAN EXPRESS, which is the point of it
// (D-000.50: ask whether any subject can express the defect BEFORE writing
// the guard; the question comes before the assertion, not after it).
//
// Measured across all seven pre-2.6 fixtures by laying them out with the
// production functions and comparing each one's lowest content bottom
// against its derived content height:
//
//	more than one page of content   0 of 7
//	a POPULATED pageHeader band     1 of 7  (three-band-page only)
//	a POPULATED pageFooter band     2 of 7  (three-band-page, font-text)
//
// The tightest fixture, wrapped-text, uses 37.7% of one content band and no
// fixture is within 454pt of the page boundary. So NO EXISTING FIXTURE CAN
// EXPRESS THIS STORY'S DEFECT: an assertion that content spilling past the
// content band produces a second page is vacuous over every one of them, and
// vacuous INVISIBLY, because the assertion itself reads as sound.
//
// This document fixes that:
//
//   - IT PAGINATES. Its single content element wraps to 29 lines at 24pt on
//     the Noto Sans chain, against a content window that holds 22. Lines
//     0-21 fall on page 1 and lines 22-28 on page 2, so the element's lines
//     STRADDLE a page boundary — the case D-2.6.1's "no line is ever split"
//     invariant is about, and the case no other fixture in the repository
//     contains.
//   - BOTH RUNNING BANDS ARE POPULATED, each with a DISTINCT literal string
//     ("HEADER REPEATED ON EVERY PAGE" / "FOOTER REPEATED ON EVERY PAGE"),
//     so a band mix-up shows up in the rendered TEXT and not only in a
//     coordinate, and a header drawn where a footer belongs is readable in
//     the golden by a human.
//   - THE SIX GEOMETRIC INPUTS ARE PAIRWISE DISTINCT: margin.top 30,
//     margin.bottom 42, margin.left 36, margin.right 54, pageHeader.height
//     18, pageFooter.height 24. No two are equal, so no SUBSTITUTION among
//     them and no SWAP of a pair survives — Story 2.5's finding, which was
//     that four mutually-equal inputs made a moved page-header band
//     invisible to every test in the repository.
//
// WHAT IT CANNOT EXPRESS, stated so nobody reads more into it than is there:
//
//   - It is ALL-LATIN by construction, so it creates NO reading judgment and
//     NO human sign-off obligation (three-band-page's precedent). The Thai
//     obligations are D-2.3.5's and D-2.4.3's and are bound to
//     fixtures/shaped-text and fixtures/expected-breaks respectively.
//   - It is SINGLE-FACE and carries no image, no binding and no fallback, so
//     it says nothing about shaping, coverage resolution or subsetting.
//   - It contains no element taller than the content window, so it does NOT
//     exercise FR44's overflow diagnostic (layout.OverflowError). That case
//     is covered in internal/layout/paginate_test.go, where it can be stated
//     as an error assertion rather than as bytes.
//   - It has exactly ONE content element, so it says nothing about how two
//     absolutely-positioned siblings interleave across a window boundary.
//     internal/layout/paginate_test.go covers that.
//   - It does NOT contain {{page}} or {{pages}}. Those are RESERVED for
//     Story 2.7 (D-1.6.5, AC18) and their pass-through is asserted on a
//     separate in-test document, so this golden is not re-recorded when 2.7
//     implements them.
const multiPageTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 480, "height": 700, "value": "This statement covers the period from the first of January to the thirty-first of March. Every page of it carries the same running header and the same running footer, so a reader handed only the second sheet can still tell what they are holding and who issued it. The content below is one single text element. It is not divided into paragraphs, sections or boxes, and nothing in it has been repositioned in order to make it fit. It simply runs on until it reaches the bottom of the content band, and then it continues at the top of the next page, at exactly the leading it had before. No line is ever split across the boundary between two pages. A line that would otherwise fall half on one sheet and half on the next is placed whole on the next sheet instead, and the small band of whitespace left behind at the foot of the earlier page is correct typesetting rather than a defect. A bank statement cannot ship a half line. That single rule is what makes this document safe to print, safe to file, and safe to read aloud from. Nothing on any page negotiates for space with anything else, and no element ever moves sideways or shuffles upward to close a gap that pagination opened.", "style": {"fontFamily": "body", "fontSize": 24}}
      ]
    },
    "pageFooter": {
      "elements": [
        {"id": "e2", "type": "text", "x": 0, "y": 6, "width": 480, "height": 16, "value": "FOOTER REPEATED ON EVERY PAGE", "style": {"fontFamily": "body", "fontSize": 8}}
      ],
      "height": 24
    },
    "pageHeader": {
      "elements": [
        {"id": "e3", "type": "text", "x": 0, "y": 4, "width": 480, "height": 16, "value": "HEADER REPEATED ON EVERY PAGE", "style": {"fontFamily": "body", "fontSize": 9}}
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
