package folio8

// alignmentRoundingTemplateJSON is fixtures/alignment-rounding/input.folio,
// kept byte-identical to it by TestAlignmentRoundingGoldenFixture.
//
// IT CLOSES DW-24. Measured at Story 7.3's baseline, across every
// `fixtures/*/input.folio`: 16 `"align": "left"`, 8 `"align": "right"`,
// ZERO `"center"` and ZERO `valign` of any value. So of the alignment
// feature's branches the corpus exercised the two that CANNOT round -
// `left` returns zero and `right` returns the slack unchanged - and none
// of the ones that halve a slack with geom.ScaleRound. Every recorded
// byte in the repository was compatible with a build that broke the
// half-to-even tie differently on a different target, which is precisely
// what AD-2/AD-3 exist to prevent and what the four-target matrix exists
// to catch.
//
// ONE DOCUMENT REACHING EVERY SITE THE RE-DERIVED ENUMERATION RETURNS.
// The enumeration is re-derived by grep at the closing revision and
// recorded in DW-24's closing note, never read off the entry's
// hand-list - which had rotted twice. A centred TEXT element does not
// cover a table's cells: those round in different code, in
// table_render.go, so the document carries both.
//
//	e1  style.align "center"        -> text_alignment.go's textAlignOffset
//	e2  style.valign "middle"       -> text_alignment.go's textValignOffset
//	e3  style.valign "bottom"       -> unrounded, and equally undeclared
//	                                   by the corpus until now
//	e4  a table whose e5 column is centred and carries a `count` footer,
//	    so the HEADER, BODY and FOOTER cell align branches all round; its
//	    headerStyle sets valign "middle" (the header cell's own vertical
//	    round) and its style sets valign "middle", which is the only way
//	    to reach the integer LINE-COUNT split a body row distributes its
//	    spare line slots by.
//
// EVERY ONE OF THOSE SLACKS IS ODD IN MILLIPOINTS, DELIBERATELY, AND
// EVERY ONE OF THEM IS 3 (MOD 4). A round-half-to-even rule and a
// truncating slack/2 agree on every EVEN slack, so a fixture whose
// slacks happened to be even would satisfy DW-24's letter and detect
// none of the mutations its own falsifier runs. An odd slack takes the
// tie - but half of the odd slacks round DOWN to even, which truncation
// also produces, so oddness alone is not enough either. A slack of
// 3 (mod 4) rounds UP, and only then does the site's golden go red when
// the rule is mutated to truncation.
//
// Three boxes are therefore declared in THOUSANDTHS of a point -
// `height` 40.001, `headerHeight` 24.001 and the centred column's
// `width` 60.003 - chosen against the shaped label and value widths so
// that all seven slacks land on 3 (mod 4).
// TestAlignmentRoundingSlacksAreOdd asserts it rather than leaving it to
// hold by luck.
//
// IT DECLARES 1.0 and must keep declaring it: it uses no 1.1 key and no
// 2.0 value. A fixture that drifted up to the library's ceiling would be
// the exact misdeclaration D-7.2.1 exists to end.
const alignmentRoundingTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 20, "value": "Centred", "style": {"fontFamily": "body", "fontSize": 11, "align": "center"}},
        {"id": "e2", "type": "text", "x": 0, "y": 30, "width": 200, "height": 40.001, "value": "Middled", "style": {"fontFamily": "body", "fontSize": 11, "valign": "middle"}},
        {"id": "e3", "type": "text", "x": 0, "y": 80, "width": 200, "height": 40, "value": "Bottomed", "style": {"fontFamily": "body", "fontSize": 11, "valign": "bottom"}},
        {"id": "e4", "type": "table", "x": 0, "y": 130, "bind": "clauses[]", "as": "clause", "headerHeight": 24.001,
          "columns": [
            {"id": "e5", "label": "Qty", "width": 60.003, "align": "center", "bind": "{{clause.qty}}", "footer": "count"},
            {"id": "e6", "label": "Clause", "width": 140, "bind": "{{clause.text}}"}
          ],
          "style": {"fontFamily": "body", "fontSize": 11, "valign": "middle"},
          "headerStyle": {"fontSize": 11, "valign": "middle"}}
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
  "version": "1.0"
}
`

// alignmentRoundingDataJSON is the report data
// fixtures/alignment-rounding/ renders against.
//
// The FIRST clause's text is long enough to wrap to FOUR lines in its
// 140 pt column while its qty cell is one line, so that row has three
// spare line slots and the table's `valign: "middle"` places the qty
// cell at slot 3/2 = 1 - the only construct in the repository that
// reaches that integer split at all. The second and third rows are
// shorter, so the row heights differ and a build that got the offset
// uniformly wrong would have to get every row wrong in the same
// direction to still agree.
//
// The qty values are "3", "12" and "7" - two distinct shaped widths, so
// the centred body cell rounds two different odd slacks rather than the
// same one three times.
const alignmentRoundingDataJSON = `{"clauses":[{"qty":"3","text":"Each party shall keep the terms of this deed confidential at all times and thereafter."},{"qty":"12","text":"Notice is given in writing."},{"qty":"7","text":"The law of England applies."}]}`
