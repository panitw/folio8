package folio8

// keepTogetherTemplateJSON is fixtures/keep-together/input.folio, kept
// byte-identical to it by TestKeepTogetherGoldenFixture.
//
// IT IS A DISCRIMINATOR, NOT A DEMONSTRATION (Story 7.7, FR51). The
// document is authored so that its signature block lands ASTRIDE the
// first content window's ceiling: with the tags, the block moves whole
// to page 2; without them, its first element stays behind on page 1.
// The two renders differ in bytes, and keepTogetherUngroupedTemplateJSON
// below is the twin that makes that difference measurable rather than
// asserted.
//
// THE ARITHMETIC IS NOT RESTATED HERE. Its single copy is the table in
// fixtures/keep-together/README.md ("The arithmetic"), which gives every
// element's measured, band-relative extent against the 729.890 pt
// content window. It lived in both places once and the two copies
// drifted apart — and away from the fixture — so this comment gives the
// SHAPE and the README gives the numbers.
//
// The shape: the body text (e1) ends far above the window's ceiling, so
// nothing about the block's behaviour depends on where the body happens
// to break. Of the three signature elements, e2 falls inside window one
// and e3 and e4 fall past it.
//
// UNGROUPED that is a severed signature block: the name prints at the
// foot of page 1 and the rule and the date print at the head of page 2,
// which is the defect FR51 exists to end. GROUPED, the union extent is
// far short of a whole window, so the group is not over-tall — and it
// does not fit window one, so the window slides to the group's EARLIEST
// top and all three members ride to page 2 at their own declared
// positions. No sibling moves; the body text is untouched.
//
// IT DECLARES 1.2, the version Story 7.7 introduced (D-7.7.2), because
// it actually carries the key. Its ungrouped twin declares 1.2 as well:
// version is never LOWERED, and keeping the two documents different in
// exactly one respect — the three tags — is the whole point of the pair.
const keepTogetherTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 523, "height": 660, "value": "1. This deed of appointment is made between Northgate Holdings Limited, a company incorporated in England and Wales, and the person whose name appears beneath the ruled line at the foot of this instrument. 2. The parties agree that every obligation recorded in the schedule attached to this deed is owed severally, and that no waiver of any term shall be inferred from a failure to enforce it on an earlier occasion. 3. Notice under this deed is given in writing to the registered office of the party to be served, and is treated as received on the second working day after the day on which it was posted. 4. The law of England and Wales governs this deed and every dispute arising out of it, and the courts of England and Wales have exclusive jurisdiction over any such dispute. 5. Nothing in this deed creates a partnership between the parties, nor authorises either of them to act as the agent of the other in any dealing with a third party. 6. Each party shall keep the terms of this deed confidential at all times during its currency, and for a period of three years after the day on which it has come to an end. 7. Where a sum falls due under this deed and is not paid on the day it is due, interest accrues on that sum at four per cent above the base rate from time to time in force. 8. A person who is not a party to this deed has no right to enforce any of its terms, and the parties may vary or rescind it without the consent of any such person. 9. If a court holds any provision of this deed to be invalid, that provision is severed and the remainder of the deed continues in force as though the severed words had never been written. 10. The schedule referred to in clause 2 may be amended only in writing signed by both parties, and an amendment made in any other manner has no effect on the obligations recorded in it. 11. Neither party is liable for a failure to perform an obligation under this deed where that failure is caused by an event outside its reasonable control and it has told the other party promptly. 12. The rights and remedies recorded in this deed are cumulative, and none of them excludes a right or a remedy that either party would otherwise have at law or in equity. 13. Any assignment of a right under this deed requires the prior written consent of the other party, and consent may not be withheld unreasonably where the assignee is a member of the same group. 14. Time is not of the essence in the performance of any obligation under this deed unless one party has served a notice on the other making time of the essence for that obligation. 15. This deed records the whole of the agreement between the parties on its subject matter, and it replaces every earlier draft, understanding and assurance relating to that subject matter. 16. A notice served under clause 3 is not validly served if it is sent only by electronic means, whatever the parties may have done in the course of dealing with one another before that date. 17. Each party shall pay its own costs of the negotiation and execution of this deed, and neither may recover those costs from the other under any provision of it. 18. The obligations recorded in clauses 6 and 7 survive the ending of this deed, however it ends, and remain enforceable against each party after the day on which it ends. 19. Nothing in this deed obliges either party to enter into a further agreement of any kind, and neither may treat a statement of intention as an offer capable of acceptance. 20. This deed may be executed in counterparts, each of which when executed is an original, and all of which together constitute one and the same instrument.", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e2", "type": "text", "x": 0, "y": 706, "width": 240, "height": 16, "keepTogether": "signature", "value": "Signed for the Company", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e3", "type": "rect", "x": 0, "y": 734, "width": 240, "height": 1, "keepTogether": "signature", "style": {"background": "#000000"}},
        {"id": "e4", "type": "text", "x": 0, "y": 740, "width": 240, "height": 16, "keepTogether": "signature", "value": "Date: 31 August 2026", "style": {"fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 5,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.2"
}
`

// keepTogetherUngroupedTemplateJSON is keepTogetherTemplateJSON with the
// three `"keepTogether": "signature", ` tags REMOVED and nothing else
// changed — asserted mechanically by
// TestKeepTogetherTwinDiffersOnlyByTheTags, so the pair cannot drift
// into differing for some second reason and quietly stop discriminating.
//
// It is the control. A test that only asserted "the grouped render puts
// the block on page 2" would pass under an implementation that put the
// block on page 2 for some unrelated reason (a mis-measured body, an
// off-by-one window). Asserting the DIFFERENCE between these two
// documents cannot.
const keepTogetherUngroupedTemplateJSON = `{
  "assets": {},
  "bands": {
    "content": {
      "elements": [
        {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 523, "height": 660, "value": "1. This deed of appointment is made between Northgate Holdings Limited, a company incorporated in England and Wales, and the person whose name appears beneath the ruled line at the foot of this instrument. 2. The parties agree that every obligation recorded in the schedule attached to this deed is owed severally, and that no waiver of any term shall be inferred from a failure to enforce it on an earlier occasion. 3. Notice under this deed is given in writing to the registered office of the party to be served, and is treated as received on the second working day after the day on which it was posted. 4. The law of England and Wales governs this deed and every dispute arising out of it, and the courts of England and Wales have exclusive jurisdiction over any such dispute. 5. Nothing in this deed creates a partnership between the parties, nor authorises either of them to act as the agent of the other in any dealing with a third party. 6. Each party shall keep the terms of this deed confidential at all times during its currency, and for a period of three years after the day on which it has come to an end. 7. Where a sum falls due under this deed and is not paid on the day it is due, interest accrues on that sum at four per cent above the base rate from time to time in force. 8. A person who is not a party to this deed has no right to enforce any of its terms, and the parties may vary or rescind it without the consent of any such person. 9. If a court holds any provision of this deed to be invalid, that provision is severed and the remainder of the deed continues in force as though the severed words had never been written. 10. The schedule referred to in clause 2 may be amended only in writing signed by both parties, and an amendment made in any other manner has no effect on the obligations recorded in it. 11. Neither party is liable for a failure to perform an obligation under this deed where that failure is caused by an event outside its reasonable control and it has told the other party promptly. 12. The rights and remedies recorded in this deed are cumulative, and none of them excludes a right or a remedy that either party would otherwise have at law or in equity. 13. Any assignment of a right under this deed requires the prior written consent of the other party, and consent may not be withheld unreasonably where the assignee is a member of the same group. 14. Time is not of the essence in the performance of any obligation under this deed unless one party has served a notice on the other making time of the essence for that obligation. 15. This deed records the whole of the agreement between the parties on its subject matter, and it replaces every earlier draft, understanding and assurance relating to that subject matter. 16. A notice served under clause 3 is not validly served if it is sent only by electronic means, whatever the parties may have done in the course of dealing with one another before that date. 17. Each party shall pay its own costs of the negotiation and execution of this deed, and neither may recover those costs from the other under any provision of it. 18. The obligations recorded in clauses 6 and 7 survive the ending of this deed, however it ends, and remain enforceable against each party after the day on which it ends. 19. Nothing in this deed obliges either party to enter into a further agreement of any kind, and neither may treat a statement of intention as an offer capable of acceptance. 20. This deed may be executed in counterparts, each of which when executed is an original, and all of which together constitute one and the same instrument.", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e2", "type": "text", "x": 0, "y": 706, "width": 240, "height": 16, "value": "Signed for the Company", "style": {"fontFamily": "body", "fontSize": 11}},
        {"id": "e3", "type": "rect", "x": 0, "y": 734, "width": 240, "height": 1, "style": {"background": "#000000"}},
        {"id": "e4", "type": "text", "x": 0, "y": 740, "width": 240, "height": 16, "value": "Date: 31 August 2026", "style": {"fontFamily": "body", "fontSize": 11}}
      ]
    },
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans"]},
  "locale": "en",
  "nextId": 5,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36}, "orientation": "portrait", "size": "A4"},
  "utcOffset": "+00:00",
  "version": "1.2"
}
`

// keepTogetherDataJSON is the report data fixtures/keep-together/ renders
// against. The document binds nothing: FR51 is a property of the
// TEMPLATE's declared geometry, and a fixture whose break depended on
// bound data would be measuring two things at once.
const keepTogetherDataJSON = `{}`
