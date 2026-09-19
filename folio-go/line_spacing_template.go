package folio8

// lineSpacingTemplateJSON is fixtures/line-spacing/input.folio, kept
// byte-identical to it by TestLineSpacingGoldenFixture (the same
// hand-sync precedent font-text, multi-script-fallback, wrapped-text and
// mandatory-break set).
//
// It is Story 7.2's golden, and it exists because the story's own
// byte-neutrality guard over the pre-7.2 corpus is UNFALSIFIABLE without
// it: measured at this story's baseline, not one committed fixture
// declared `style.lineSpacing` at all — because the key did not exist —
// so no recorded byte in the repository could tell a build that honours
// an author's leading from one that silently ignores it.
//
// IT IS ALSO THE FIRST COMMITTED DOCUMENT DECLARING `"version": "1.1"`.
// That is not decoration: under D-1.4.13 a document declares the lowest
// version its own content requires, and this one requires 1.1 because it
// sets `lineSpacing`. A fixture that declared 1.0 while using a 1.1 key
// would be the exact misdeclaration D-7.2.1 exists to end.
//
// Four elements, each carrying a property nothing else in the corpus can
// express:
//
//	e1  NO lineSpacing at all. THE CONTROL, and it is what makes every
//	    other element's numbers mean something: its two baselines are one
//	    RULED advance apart (14,982 mp), so a build that scaled the wrong
//	    thing, or scaled everything, disagrees with it first.
//
//	e2  lineSpacing 1.5 -> an advance of 22,473 mp. THE AC ELEMENT for
//	    "Advance scales". Its FIRST baseline must sit at exactly the same
//	    offset below its own top edge as e1's does below e1's — that is
//	    the two-model split (D-2.5a/DW-15) observed in the artifact
//	    rather than asserted in a unit test.
//
//	e3  lineSpacing 0.6 -> an advance of 8,989 mp, which is LESS than the
//	    first-baseline offset of 11,759. THE TIGHT-LEADING ELEMENT: one
//	    line's baseline sits below the next line's top, so the line boxes
//	    overlap. This is the geometry the designer canvas used to refuse
//	    outright — blanking the entire projection — and D-7.2.2 deleted
//	    that clause. Three lines rather than two, so the overlap is
//	    interior and not merely terminal.
//
//	e4  a TABLE carrying lineSpacing on `style` AND on `headerStyle`.
//	    D-7.1.3's "every caller, no carve-out" is a claim about code that
//	    nothing in the corpus could contradict without this element. Its
//	    second column's bound value carries a line feed (Story 7.1), so
//	    the first data row is TWO lines and its height is therefore a
//	    function of the scaled advance — the only way a table's leading
//	    is observable in the produced bytes at all. `headerStyle` sets a
//	    DIFFERENT ratio from `style` and inherits `fontFamily` from it,
//	    so the cascade is exercised in both directions at once.
//
// Every number here is MEASURED, not assumed — see
// TestLineSpacingSemanticAcceptance.
const lineSpacingTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 40, "value": "Ruled\nleading", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e2", "type": "text", "x": 0, "y": 60, "width": 200, "height": 60, "value": "Open\nleading", "style": {"fontFamily": "body", "fontSize": 11, "lineSpacing": 1.5}},
        {"id": "e3", "type": "text", "x": 0, "y": 140, "width": 200, "height": 60, "value": "Tight\nleading\nthrice", "style": {"fontFamily": "body", "fontSize": 11, "lineSpacing": 0.6}},
        {"id": "e4", "type": "table", "x": 0, "y": 240, "bind": "clauses[]", "as": "clause", "headerHeight": 20,
          "columns": [
            {"id": "e5", "label": "No.", "width": 40, "bind": "{{clause.no}}"},
            {"id": "e6", "label": "Clause", "width": 200, "bind": "{{clause.text}}"}
          ],
          "style": {"fontFamily": "body", "fontSize": 11, "lineSpacing": 1.5},
          "headerStyle": {"fontSize": 11, "lineSpacing": 0.6}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 7,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+07:00",
  "version": "1.1"
}
`

// lineSpacingDataJSON is the report data fixtures/line-spacing/ renders
// against. The first clause's value carries a line feed (Story 7.1) so
// that row is two lines tall and its height is a function of the scaled
// advance; the second is one line, so the two rows have different
// heights and a build that ignored the leading would have to get both
// wrong in the same direction to still agree.
const lineSpacingDataJSON = `{"clauses":[{"no":"1","text":"First clause\nsecond line"},{"no":"2","text":"Second clause"}]}`
