package folio8

import (
	"strings"
	"testing"
)

// DW-28, and the reason these arms are written HERE rather than beside
// the branch they exercise.
//
// internal/pdf's glyph-positioning refusal (textdoc.go's
// appendShapedRun) had been exercised since Story 2.3 by handing the
// emitter a SYNTHETIC run. Both that test and the branch's own comment
// stated, as measured fact, that the branch is "UNREACHABLE through the
// render path with the shipped set and cannot be red-proved through it".
//
// THAT CLAIM WAS FALSE, and it was load-bearing: it is why no
// render-path test was ever built, and it is what a reader hitting the
// refusal in production would have been told when they went looking.
// Story 2.3 measured ITS OWN SAMPLES and reported on THE SHIPPED SET —
// two different populations. The samples happened to contain no Thai
// sequence whose marks the shaper displaces vertically; ordinary Thai
// does, and the shipped Noto Sans Thai gives those marks a non-zero
// YOffset.
//
// So this file exists to hold the emitter to a REAL DOCUMENT, through
// ParseTemplate + Render — the same public entry point a caller uses —
// rather than through a hand-built run.
//
// STORY 8.0 RE-POINTED IT, IT DID NOT REPLACE IT (D-7.8.7: re-point,
// never delete). The characterization landed first, deliberately, so
// that the fix would have something to move; these are the same three
// arms, now stating what the engine does AFTER the fix:
//
//	arm A  ทั้งสิ้น RENDERS, and its content stream carries the rise.
//	arm B  สัญญา, the control, is UNCHANGED and still green.
//	arm C  the owner's clause RENDERS.
//
// The two properties arm A used to carry — a refused render emits ZERO
// bytes, and the message an author reads is pinned VERBATIM — did not
// go anywhere. They moved onto the refusal that is LEFT, which is the
// rounding boundary: see TestAStackedMarkWhoseRiseRoundsAwayIsStillRefused.

// thaiStackedMarksTemplate is the smallest document that exercises the
// vertical offset: one text element whose value is the single word
// ทั้งสิ้น ("in total" / "altogether"), in which ั and ้ both sit above ท
// and the face resolves the pair with a GPOS y-displacement.
//
// It is deliberately ONE WORD. A whole clause reaches the same code (see
// the owner's-document arm below), but a one-word document keeps the
// subset small enough that the CID in the pinned message is stable.
//
// THE TRIGGER IS A NON-ZERO YOffset, NOT MARK STACKING. ที่, ป้ำ and ปั
// each stack two marks over one base and have rendered since Story 2.3,
// because Noto Sans Thai resolves those pairs with a GSUB lowered-form
// substitution at ZERO offset. ที่ appears in fixtures/shaped-text, in
// all four statement-* fixtures and in fixtures/justified-thai. A reader
// who takes "two stacked marks" as the predicate will read those
// goldens as contradicting this file; they do not.
const thaiStackedMarksTemplate = `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 40,
       "value": "ทั้งสิ้น", "style": {"fontFamily": "body", "fontSize": 12}}
    ]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans", "Noto Sans Thai"]},
  "locale": "th",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36},
           "orientation": "portrait", "size": "A4"},
  "utcOffset": "+07:00",
  "version": "1.0"
}`

// thaiRiseRoundsAwayTemplate is the SAME document at fontSize 0.008, and
// it is the whole reason the fail-closed branch narrowed rather than
// vanished.
//
// With Ts in place every ShapedGlyph field is expressible, so the only
// thing left for a refusal to catch is the ROUNDING BOUNDARY: an offset
// the shaper really asked for whose rise scales to zero.
// ScaleRound(8, -57, 1000) == 0, so emitting the rise would drop the
// offset silently — the healthy output and the broken output would be
// the same bytes.
//
// It is REACHABLE, and the population that is measured over is
// "documents the shipped parser accepts": fontSize has no positivity
// floor at parse (parse.go's decodePoints), so this loads and arrives at
// the emitter. At any fontSize of 1 pt or more, |rise| >= |YOffset|
// millipoints and is never zero, so no ordinary document reaches it.
//
// It is written out rather than derived from the const above by a string
// replacement, because the message pinned below names a CID, and a CID
// is a property of the subset this exact document produces.
const thaiRiseRoundsAwayTemplate = `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 40,
       "value": "ทั้งสิ้น", "style": {"fontFamily": "body", "fontSize": 0.008}}
    ]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans", "Noto Sans Thai"]},
  "locale": "th",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36},
           "orientation": "portrait", "size": "A4"},
  "utcOffset": "+07:00",
  "version": "1.0"
}`

// thaiUnstackedTemplate is the NON-VACUITY CONTROL, and it is the whole
// reason the arms above prove anything about vertically displaced marks
// rather than about Thai.
//
// สัญญา ("contract") is the same script, the same face, the same font
// size and the same box. Its only difference is that no glyph in it
// carries a vertical offset. If this document ever stops rendering, the
// arms above stop being evidence about the offset and the failure is
// somewhere else entirely.
//
// UNCHANGED BY STORY 8.0, deliberately: a control that had to be edited
// to keep up with the fix was never a control.
const thaiUnstackedTemplate = `{
  "assets": {},
  "bands": {
    "content": {"elements": [
      {"id": "e1", "type": "text", "x": 0, "y": 0, "width": 200, "height": 40,
       "value": "สัญญา", "style": {"fontFamily": "body", "fontSize": 12}}
    ]},
    "pageFooter": {"elements": [], "height": 20},
    "pageHeader": {"elements": [], "height": 20}
  },
  "fonts": {"body": ["Noto Sans", "Noto Sans Thai"]},
  "locale": "th",
  "nextId": 2,
  "page": {"margin": {"bottom": 36, "left": 36, "right": 36, "top": 36},
           "orientation": "portrait", "size": "A4"},
  "utcOffset": "+07:00",
  "version": "1.0"
}`

// thaiOwnersClause is the contractor-liability clause from a real Thai
// contract, reported by the owner against the shipped designer (DW-28).
// It is not constructed to trip anything — it tripped the refusal
// because ordinary Thai legal prose contains ทั้งสิ้น, and ครั้ง, ทั้งนี้
// and ตั้งแต่ elsewhere.
//
// fixtures/thai-stacked-marks/ is built from THIS const, and
// TestThaiStackedMarksFixtureCarriesTheOwnersClause asserts the golden
// still carries it verbatim — so the fixture cannot drift away from the
// document that motivated it.
const thaiOwnersClause = "การที่ผู้รับจ้างเป็นผู้ประกอบธุรกิจทวงถามหนี้ เป็นผู้มีความรู้ ความชำนาญ " +
	"มีความสามารถเป็นพิเศษในงานที่รับจ้าง จึงทราบและเข้าใจในพระราชบัญญัติ การทวงถามหนี้ พ.ศ. 2558 " +
	"และกฎหมายอื่น ๆ ที่เกี่ยวข้องกับกิจการงานที่มารับจ้างตามที่ระบุอยู่ในสัญญาจ้างเป็นอย่างดี " +
	"ผู้รับจ้างย่อมต้องรับผิดเป็นการส่วนตัวทั้งสิ้น"

// thaiRiseRoundsAwayMessage is the message pinned VERBATIM, because the
// point of pinning it is that this is the text a document author reads
// when their document will not render. A reworded refusal is a changed
// product, not a changed internal, and it should have to be edited here
// deliberately.
//
// STORY 8.0 REWROTE THE REASON CLAUSE, AND THE REWRITE IS THE POINT. The
// old text said the offset was refused "which a TJ array cannot express"
// — true of the branch that refused every offset, and FALSE of the one
// that is left: a TJ array is no longer why the glyph is refused, a rise
// that rounded to zero is. Shipping a canonical statement that
// misdescribes its own condition is exactly the failure D-8.0.1 exists
// to stop.
//
// The second half is kept WORD FOR WORD from the message this narrows,
// because it stays true and it is the sentence that explains why
// refusing beats degrading.
const thaiRiseRoundsAwayMessage = "internal/pdf: face Noto Sans Thai: CID 3 carries a non-zero " +
	"vertical offset (-57 thousandths of an em) that scales to a text rise of zero at this run's " +
	"font size (8 millipoints). Emitting the glyph without its offset would place it wrongly with " +
	"no observable difference in the output bytes, so this fails rather than degrades."

// assertEveryTextRiseIsRestored is arms A and C's restoration check, and
// it is written this way rather than as a substring scan because the
// obvious spelling — strings.Contains(res.Bytes, "0 Ts\n") — is wrong
// twice over.
//
// It is UNANCHORED: any operand whose last character is the digit 0
// satisfies it, so "-10 Ts\n" reads as a restore. And it scans the WHOLE
// FILE, embedded FontFile2 programs included, where arbitrary binary can
// spell any operator by chance.
//
// This walks the PAGE CONTENT STREAMS (splitPageContentStreams follows
// each page object's own /Contents reference, so a font program is never
// visited) and reads the operand of every Ts in every BT..ET block, which
// is the only form in which "the run gave the rise back" is actually
// checkable. It returns the number of runs that carried a rise, so a
// caller can refuse to pass vacuously.
func assertEveryTextRiseIsRestored(t *testing.T, raw []byte) int {
	t.Helper()
	risen := 0
	for _, content := range splitPageContentStreams(t, raw) {
		for i, block := range strings.Split(content, "BT\n")[1:] {
			end := strings.Index(block, "ET\n")
			if end == -1 {
				t.Fatalf("run %d: unterminated BT block", i)
			}
			block = block[:end]
			ops := textRiseOperands(t, i, block)
			if len(ops) == 0 {
				continue
			}
			risen++
			if ops[len(ops)-1] != "0" {
				t.Fatalf("run %d ends at text rise %s, not 0 — rise is a persistent text-state parameter that survives ET, and an unclipped uncoloured run has no q/Q bracket to restore it, so whatever drew next would be displaced.\nrun: %q", i, ops[len(ops)-1], block)
			}
		}
	}
	return risen
}

// TestThaiStackedMarksReachThePageThroughTheRenderPath is arm A,
// re-pointed by Story 8.0 from "is refused" to "renders".
//
// This is the same document, through the same public entry point, with
// the same shipped font set — the characterization arm turned over. It
// asserts more than "no error": the document must produce BYTES, and
// those bytes must carry the rise operator, because a build that
// silently dropped the offset would also return no error.
func TestThaiStackedMarksReachThePageThroughTheRenderPath(t *testing.T) {
	tpl, err := ParseTemplate([]byte(thaiStackedMarksTemplate))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Render(tpl, Data("{}"), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("ordinary Thai whose marks the shaper displaces vertically must RENDER (DW-28): %v", err)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("the render returned no error and no bytes, so it witnesses nothing")
	}

	// ScaleRound(12000, -57, 1000) = -684 millipoints. The sign passes
	// through untouched: YOffset is the shaper's y-up delta and Ts is
	// y-up in text space, so nothing between render.go's producer and
	// the operand negates it.
	//
	// Looked for in the PAGE CONTENT STREAM, not in the file: a golden
	// also carries embedded font programs, and arbitrary binary can
	// spell any operator by chance.
	const wantRise = "-0.684 Ts\n"
	found := false
	for _, content := range splitPageContentStreams(t, res.Bytes) {
		if strings.Contains(content, wantRise) {
			found = true
		}
	}
	if !found {
		t.Fatalf("no page content stream carries %q — the mark reached the page at the WRONG height, which is the one failure that produces no error at all", wantRise)
	}

	// ...and every run that sets a rise must give it back: text rise is
	// a persistent text-state parameter that survives ET.
	if risen := assertEveryTextRiseIsRestored(t, res.Bytes); risen == 0 {
		t.Fatal("no run in this document carries a text rise, so the restoration check asserted nothing")
	}
}

// TestThaiWithoutStackedMarksStillRenders is the control. Without it the
// arm above passes for a document that was never going to render.
//
// UNCHANGED BY STORY 8.0.
func TestThaiWithoutStackedMarksStillRenders(t *testing.T) {
	tpl, err := ParseTemplate([]byte(thaiUnstackedTemplate))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Render(tpl, Data("{}"), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("Thai without a vertically displaced mark must render; the subject is the OFFSET, not Thai: %v", err)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("the control rendered no bytes, so it witnesses nothing")
	}
	// The control's OTHER job, from Story 8.0 on: it is the zero-offset
	// path, and the zero-offset path must emit no rise operator at all.
	// This is the byte-identity guardrail stated on a real document.
	if strings.Contains(string(res.Bytes), " Ts\n") {
		t.Fatal("a document with no vertically displaced mark emitted a text-rise operator — the Ts path must be entered ONLY when YOffset != 0, or every golden in the corpus moves")
	}
}

// TestAnOrdinaryThaiClauseRenders is arm C, re-pointed, and it is the
// arm that says why this was worth a story rather than a footnote.
//
// The value is the clause the owner pasted into the shipped designer:
// the canvas drew it, and the PDF stage refused it with
// `Render failure · ENGINE_REJECTED`. The CID is NOT pinned for this arm
// — a longer document subsets more glyphs, so which CID carries which
// offset is an artefact of the subset rather than a fact about the
// language.
func TestAnOrdinaryThaiClauseRenders(t *testing.T) {
	tpl, err := ParseTemplate([]byte(strings.Replace(thaiStackedMarksTemplate, `"ทั้งสิ้น"`, `"`+thaiOwnersClause+`"`, 1)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	res, err := Render(tpl, Data("{}"), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("an ordinary Thai contract clause must render (DW-28): %v", err)
	}
	if len(res.Bytes) == 0 {
		t.Fatal("the clause rendered no bytes, so it witnesses nothing")
	}
	// This is the largest and most segment-dense document in the suite —
	// five lines, three distinct rises — so it is the strongest place to
	// assert BOTH halves: the rise is emitted, and every run that
	// emitted one gave it back before its ET.
	risen := assertEveryTextRiseIsRestored(t, res.Bytes)
	if risen == 0 {
		t.Fatal("the clause rendered without a single text-rise operator in any page content stream — its marks reached the page at the wrong height")
	}
	t.Logf("owner's clause witness — %d run(s) carry a text rise, and every one of them restores it to 0 before its ET", risen)
}

// TestAStackedMarkWhoseRiseRoundsAwayIsStillRefused carries the two
// properties arm A used to carry, moved onto the refusal that is LEFT
// (D-7.8.7: re-point, never delete).
//
// A refused render must produce NO bytes — a half-written document would
// be worse than the refusal, because a caller that ignores the error
// would ship it — and the message an author reads is pinned verbatim.
func TestAStackedMarkWhoseRiseRoundsAwayIsStillRefused(t *testing.T) {
	tpl, err := ParseTemplate([]byte(thaiRiseRoundsAwayTemplate))
	if err != nil {
		t.Fatalf("parse: %v — a font size of 0.008 is accepted at parse (there is no positivity floor), and this arm depends on that", err)
	}
	res, err := Render(tpl, Data("{}"), nil, testShippedFontSet())
	if err == nil {
		t.Fatalf("a vertical offset that scales to a rise of ZERO must still fail closed; got %d bytes and no error", len(res.Bytes))
	}
	if got := err.Error(); !strings.Contains(got, thaiRiseRoundsAwayMessage) {
		t.Fatalf("the refusal an author reads has changed.\n got: %s\nwant it to contain: %s", got, thaiRiseRoundsAwayMessage)
	}
	// A refusal must produce NO bytes.
	if len(res.Bytes) != 0 {
		t.Fatalf("a refused render must emit no bytes, got %d", len(res.Bytes))
	}
}
