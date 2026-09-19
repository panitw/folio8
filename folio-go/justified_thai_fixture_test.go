package folio8

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/template"
	"github.com/panitw/folio8/folio-go/internal/text"
)

// The geometry fixtures/justified-thai/ is stated in, DECLARED ONCE here
// and read by this file AND by matrix_test.go's per-leg feature guard
// (which is matrix-tagged and would otherwise carry a second copy — the
// two-literals hazard that makes a guard agree with itself while
// disagreeing with the document).
//
// Every element is chain ["Noto Sans", "Noto Sans Thai"] at 11 pt. The
// Thai face raises the ruled advance to 16,709 mp against the
// Latin-only 14,982 the other text fixtures are stated in; the
// first-baseline offset is unchanged at 11,759, because it is a maximum
// over the DECLARED chain and Noto Sans already carried it.
//
// pdfY of an element's first baseline, IN MILLIPOINTS, is
//
//	841890 - 36000 - (20000 + 1000*y) - 11759 = 774131 - 1000*y
//
// where y is the element's DECLARED y IN POINTS (0, 115, 230) and every
// other term is already in millipoints: the A4 page height 841,890 mp,
// the 36 pt top margin as 36,000 mp, the 20 pt page-header band as
// 20,000 mp, and the first-baseline offset 11,759 mp. The 1000 is the
// points-to-millipoints conversion, and it is written out because it is
// the only unit change in the line.
//
// So e1 (y = 0 pt) starts at 774,131 mp, e2 (y = 115 pt) at 659,131 and
// e3 (y = 230 pt) at 544,131 — which is what justifiedThaiBaselinesMP
// below lists.
const (
	justifiedThaiBoxWidthMP = int64(220_000)
	// justifiedThaiLeftEdgeMP is the element's left edge IN THE EMITTED
	// PDF: its declared x of 0 plus the page's 36 pt left margin, which
	// the producer applies at emission.
	justifiedThaiLeftEdgeMP = int64(36_000)
	// justifiedThaiRunLeftMP and justifiedThaiRunRightMP are the same two
	// edges on the SHIPPING RUN PATH, where coordinates are still
	// document-relative and the margin has not been applied.
	//
	// justifiedThaiRunRightMP is where EVERY justified line's last piece
	// must end, EXACTLY — asserted at the same standard as the Latin
	// case, in integer millipoints, which is what AC-TH1 requires.
	justifiedThaiRunLeftMP  = int64(0)
	justifiedThaiRunRightMP = justifiedThaiRunLeftMP + justifiedThaiBoxWidthMP

	// The atomic-run element: a 50 pt box holding a Thai run the
	// segmenter proposes no interior break opportunity inside,
	// followed by a second word.
	justifiedThaiAtomicBoxWidthMP = int64(50_000)
	// justifiedThaiAtomicFirstLine is what the packer must put on that
	// element's first line. Asserted rather than assumed, because the
	// whole case turns on this run being ATOMIC.
	//
	// ATOMIC IS A PROPERTY OF THE SEGMENTER'S ANSWER, NOT OF THE
	// WORDLIST'S CONTENTS. The shipped wordlist DOES hold "กานต์", a
	// suffix of this run — grepping for it and concluding the test is
	// broken is the wrong inference. What the case rests on, and what
	// TestJustifiedThaiAtomicRunHasNowhereToPutSlack measures directly
	// against text.Dictionary(), is that text.Opportunities proposes
	// ZERO opportunities strictly inside this run.
	justifiedThaiAtomicFirstLine = "ณัฐกานต์"
)

// justifiedThaiLines is what the packer makes of e1's two paragraphs, in
// order, with the arithmetic each line's placement is a function of.
//
// EVERY GAP HERE IS DICTIONARY-DERIVED. The element's value carries no
// space character at all — Thai writes its sentences without them — so
// each interior opportunity is a seam the shipped wordlist proposed
// (AD-25), never a run of whitespace the author typed. That is the whole
// point of this fixture beside fixtures/justified-text/, whose every gap
// is a space.
//
// gaps is the number of INTERIOR break opportunities the line holds;
// slack is the declared width less the summed piece widths; base and
// remainder are slack/gaps and slack mod gaps:
//
//	line 0  slack 13,893 over 7 gaps -> 7 x 1,984 + 5 left over
//	line 1  slack 12,771 over 6 gaps -> 6 x 2,128 + 3 left over
//	line 3  slack  7,942 over 5 gaps -> 5 x 1,588 + 2 left over
//	line 4  slack 31,757 over 9 gaps -> 9 x 3,528 + 5 left over
//
// All four remainders are NON-ZERO — a remainder of zero would make the
// ordered distribution rule vacuous, since the interesting half of the
// rule is WHICH gaps get the extra millipoint — and three of the four
// are LARGER than half the gap count, which an implementation spreading
// the remainder from the END would place at the wrong gaps.
var justifiedThaiLines = []struct {
	// pieces is the number of drawn runs the line is set as: gaps+1 when
	// the line is justified, 1 when it is ragged.
	pieces int
	// ragged records WHY a line is not justified, and the three reasons
	// are three because each answers a different question (D-7.1.5).
	ragged string
	gaps   int
	slack  int64
}{
	{pieces: 8, gaps: 7, slack: 13_893},
	{pieces: 7, gaps: 6, slack: 12_771},
	{pieces: 1, ragged: "the author ended this line by typing a break, and it is NOT the last line"},
	{pieces: 6, gaps: 5, slack: 7_942},
	{pieces: 10, gaps: 9, slack: 31_757},
	{pieces: 1, ragged: "the last line of the element, which only the line INDEX can answer"},
}

// justifiedThaiAtomicLines is the atomic-run element, and it is AC-TH2's
// subject: its FIRST line is justified, is not the last, and was not
// ended by a mandatory break — and is still drawn as ONE run at the
// element's natural start edge, because the segmenter proposes no
// interior break opportunity inside that run to put slack into.
var justifiedThaiAtomicLines = []struct {
	pieces int
	ragged string
}{
	{pieces: 1, ragged: "AD-25's atomic unknown run: ZERO interior break opportunities, so there is nowhere to put the slack"},
	{pieces: 1, ragged: "the last line of the element"},
}

// justifiedThaiBaselinesMP is every DRAWN baseline, top of page first,
// grouped by the element that emits it. e1 (justified) and e2 (the
// control, no align at all) pack into the SAME six lines at the SAME six
// intervals: only the horizontal placement differs, which is what makes
// the run counts below evidence about justification rather than about
// wrapping.
var justifiedThaiBaselinesMP = []struct {
	id string
	ys []int64
}{
	{"e1", []int64{774131, 757422, 740713, 724004, 707295, 690586}},
	{"e2", []int64{659131, 642422, 625713, 609004, 592295, 575586}},
	{"e3", []int64{544131, 527422}},
}

// justifiedThaiRunsPerBaseline is the per-line run count the three
// elements produce, in the order justifiedThaiBaselinesMP lists them.
// THIS IS THE VACUITY GUARD, and it is the whole reason the control
// element exists: every line of this document fits its box, so a build
// that ignored `justify` — or one that saw a Thai run and quietly fell
// back to ragged left, which is the failure this fixture was added to
// make visible — would emit exactly these fourteen baselines with one
// run each, which is precisely what e2 already is.
func justifiedThaiRunsPerBaseline() []int {
	out := make([]int, 0, 14)
	for _, ln := range justifiedThaiLines {
		out = append(out, ln.pieces)
	}
	for range justifiedThaiLines {
		out = append(out, 1)
	}
	for _, ln := range justifiedThaiAtomicLines {
		out = append(out, ln.pieces)
	}
	return out
}

// justifiedThaiAssertGeometry checks the drawn baselines and the
// per-line run counts of a rendered justified-thai stream. It reports
// through fail(format, args...) so the ordinary suite can use t.Errorf
// and the matrix legs t.Fatalf, without a second copy of the arithmetic.
//
// It takes *testing.T as well as fail, because the CONTROL PREMISE it
// asserts before the discriminating relation (justifiedThaiControlPremise)
// is a precondition and not a finding: if e2 has stopped being e1's
// control, the relation below is not weaker evidence, it is no evidence,
// and continuing to report against it would be misleading.
func justifiedThaiAssertGeometry(t *testing.T, runs []emittedRun, fail func(string, ...any)) {
	t.Helper()
	ys := linesByOrigin(runs)
	var want []int64
	for _, el := range justifiedThaiBaselinesMP {
		want = append(want, el.ys...)
	}
	if len(ys) != len(want) {
		fail("the render occupies %d distinct drawn baselines %v, want %d %v", len(ys), ys, len(want), want)
		return
	}
	at := 0
	for _, el := range justifiedThaiBaselinesMP {
		for i, wantY := range el.ys {
			if ys[at+i] != wantY {
				fail("%s: drawn baseline %d is at pdfY %d, want the hand-derived %d", el.id, i, ys[at+i], wantY)
			}
		}
		at += len(el.ys)
	}

	counts := make(map[int64]int, len(ys))
	xs := make(map[int64][]int64, len(ys))
	for _, r := range runs {
		counts[r.OriginYMilli]++
		xs[r.OriginYMilli] = append(xs[r.OriginYMilli], r.OriginXMilli)
	}
	wantCounts := justifiedThaiRunsPerBaseline()
	// THE TWO FIXTURE TABLES MUST AGREE IN LENGTH, and this is checked
	// rather than assumed because wantCounts is indexed by the baseline
	// loop below: a disagreement would be an index-out-of-range PANIC
	// that kills the whole test binary — every matrix leg using this as
	// its pre-comparison guard included — instead of failing one
	// assertion with a message naming the cause.
	if len(wantCounts) != len(want) {
		fail("justifiedThaiRunsPerBaseline() lists %d run counts and justifiedThaiBaselinesMP %d baselines — the two fixture tables disagree in length, so the per-line run counts below cannot be stated at all", len(wantCounts), len(want))
		return
	}
	for i, y := range ys {
		if counts[y] != wantCounts[i] {
			fail("baseline %d (pdfY %d) carries %d drawn runs, want %d — a justified Thai line is drawn as one run PER PIECE across its DICTIONARY-DERIVED gaps, and a ragged one as a single run", i, y, counts[y], wantCounts[i])
		}
		positions := append([]int64(nil), xs[y]...)
		sort.Slice(positions, func(a, b int) bool { return positions[a] < positions[b] })
		if len(positions) == 0 {
			continue
		}
		if positions[0] != justifiedThaiLeftEdgeMP {
			fail("baseline %d (pdfY %d) starts at x %d, want the element's own left edge %d — justification distributes slack BETWEEN pieces, never before the line", i, y, positions[0], justifiedThaiLeftEdgeMP)
		}
		for j := 1; j < len(positions); j++ {
			if positions[j] <= positions[j-1] {
				fail("baseline %d (pdfY %d) draws pieces at %v, which is not strictly ascending", i, y, positions)
				break
			}
		}
	}

	// THE PREMISE THE RELATION BELOW RESTS ON, ASSERTED FIRST. "e1 draws
	// more runs than e2" is evidence about justification ONLY IF the two
	// elements carry the SAME string in the SAME box at the SAME size.
	// Nothing else in this file pinned that, so a drift in e2's value
	// would have made the relation vacuous while leaving it green.
	justifiedThaiControlPremise(t)

	// THE DISCRIMINATING RELATION, asserted between the two elements
	// rather than against a literal: e1 must draw MORE runs than e2 even
	// though the two carry the same Thai string in the same box. A
	// silent ragged-left fallback for Thai cannot satisfy this.
	e1, e2 := 0, 0
	for i, y := range ys {
		switch {
		case i < len(justifiedThaiLines):
			e1 += counts[y]
		case i < 2*len(justifiedThaiLines):
			e2 += counts[y]
		}
	}
	if e2 != len(justifiedThaiLines) {
		fail("the control element e2 draws %d runs across %d lines, want one per line — it declares no alignment at all", e2, len(justifiedThaiLines))
	}
	if e1 <= e2 {
		fail("the justified element draws %d runs and the unaligned control %d — this fixture is certifying a document whose declared Thai justification was ignored", e1, e2)
	}
}

func renderJustifiedThai(t *testing.T) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(justifiedThaiTemplateJSON))
	if err != nil {
		t.Fatalf("parse justified-thai template: %v", err)
	}
	res, err := Render(tpl, Data(justifiedThaiDataJSON), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("render justified-thai: %v", err)
	}
	if len(res.Diagnostics) != 0 {
		t.Fatalf("the justified-thai fixture must render with NO diagnostics; got %+v", res.Diagnostics)
	}
	return res.Bytes
}

// justifiedThaiRunsByElement drives the SHIPPING run path and groups its
// runs by element and line, each line's runs sorted by x, with each
// run's own measured width beside it. Only there is a piece's width
// available next to its position, which is what the exact-right-edge and
// exact-slack assertions need.
func justifiedThaiRunsByElement(t *testing.T) (map[string]map[int][]textRunSource, map[string]map[int][]geom.Length) {
	t.Helper()
	tpl, err := ParseTemplate([]byte(justifiedThaiTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	data := emptyBindValue(t)
	fs := testShippedFontSet()
	runs, err := collectTextRuns(tpl, data, data, fs, newFontCache())
	if err != nil {
		t.Fatal(err)
	}
	byEl := map[string]map[int][]textRunSource{}
	for _, run := range runs {
		if byEl[run.elementID] == nil {
			byEl[run.elementID] = map[int][]textRunSource{}
		}
		byEl[run.elementID][run.lineIndex] = append(byEl[run.elementID][run.lineIndex], run)
	}
	cache := newFontCache()
	widths := map[string]map[int][]geom.Length{}
	for id, lines := range byEl {
		widths[id] = map[int][]geom.Length{}
		for i, lineRuns := range lines {
			sort.Slice(lineRuns, func(a, b int) bool { return lineRuns[a].x < lineRuns[b].x })
			ws := make([]geom.Length, len(lineRuns))
			for j, run := range lineRuns {
				w, werr := shippingRunWidth(run, fs, cache)
				if werr != nil {
					t.Fatal(werr)
				}
				ws[j] = w
			}
			widths[id][i] = ws
		}
	}
	return byEl, widths
}

// justifiedThaiElement recovers one of the fixture's declared elements
// from the parsed document, so every assertion below can state
// properties of what the DOCUMENT declares without a second
// transcription of it.
func justifiedThaiElement(t *testing.T, id string) template.Element {
	t.Helper()
	d, err := template.ParseDocument([]byte(justifiedThaiTemplateJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, el := range d.Bands.Content.Elements {
		if string(el.ID) == id {
			return el
		}
	}
	t.Fatalf("presence precondition: the fixture declares no element %q", id)
	return template.Element{}
}

// justifiedThaiElementValue recovers one element's authored string from
// the parsed document, so the assertions below can state properties of
// the TEXT (it carries no space, it carries no line feed) without a
// second transcription of it.
func justifiedThaiElementValue(t *testing.T, id string) string {
	t.Helper()
	return justifiedThaiElement(t, id).Value.Value
}

// justifiedThaiDeclaredBox recovers the two declared quantities the
// control premise turns on beside the string itself: the element's box
// width and its font size, both in millipoints.
func justifiedThaiDeclaredBox(t *testing.T, id string) (width, fontSize geom.Length) {
	t.Helper()
	el := justifiedThaiElement(t, id)
	if !el.Width.Set || el.Width.Null {
		t.Fatalf("presence precondition: element %q declares no width, so there is no box to compare", id)
	}
	if !el.Style.Set || el.Style.Null || !el.Style.Value.FontSize.Set || el.Style.Value.FontSize.Null {
		t.Fatalf("presence precondition: element %q declares no style.fontSize, so there is no size to compare", id)
	}
	return el.Width.Value, el.Style.Value.FontSize.Value
}

// justifiedThaiControlPremise asserts what makes e2 A CONTROL FOR e1
// rather than merely a second element: the same string, the same
// declared width, the same declared font size. Only then is "e1 draws
// more runs than e2" evidence that the declared justification was
// honoured; if e2's value drifted from e1's, the two would differ for
// reasons that have nothing to do with alignment and the relation would
// stay green while meaning nothing.
//
// t.Fatalf, in the precondition style this file already uses: a broken
// premise is not a smaller finding than a broken result, it is the
// absence of a result.
func justifiedThaiControlPremise(t *testing.T) {
	t.Helper()
	subject, control := justifiedThaiElementValue(t, "e1"), justifiedThaiElementValue(t, "e2")
	if subject != control {
		t.Fatalf("presence precondition: the control e2's value has drifted from the subject e1's, so the run-count relation between them is no longer evidence about justification at all:\ne1 %q\ne2 %q", subject, control)
	}
	subjectWidth, subjectSize := justifiedThaiDeclaredBox(t, "e1")
	controlWidth, controlSize := justifiedThaiDeclaredBox(t, "e2")
	if subjectWidth != controlWidth {
		t.Fatalf("presence precondition: e1 declares width %d mp and the control e2 %d — a control in a different box wraps differently, and the run counts would differ for that reason rather than for justification", int64(subjectWidth), int64(controlWidth))
	}
	if subjectSize != controlSize {
		t.Fatalf("presence precondition: e1 declares fontSize %d mp and the control e2 %d — a control at a different size wraps differently, and the run counts would differ for that reason rather than for justification", int64(subjectSize), int64(controlSize))
	}
	if int64(subjectWidth) != justifiedThaiBoxWidthMP {
		t.Fatalf("presence precondition: e1 declares width %d mp and this file's justifiedThaiBoxWidthMP says %d — the declared-once geometry has drifted from the document", int64(subjectWidth), justifiedThaiBoxWidthMP)
	}
}

// justifiedThaiPackedLines packs one of the fixture's elements exactly
// as the render path does — its DECLARED chain, size and box, read back
// off the document — and returns the packer's own lines beside the
// opportunity list they were packed against.
//
// It exists because wrappedLine.endedBy is not recoverable from a drawn
// run: "which break ended this line" is the packer's own record, and it
// is the only thing that can tell a line set ragged BY A TYPED BREAK
// from one set ragged for some other reason.
func justifiedThaiPackedLines(t *testing.T, id string) ([]wrappedLine, []text.Opportunity) {
	t.Helper()
	tpl, err := ParseTemplate([]byte(justifiedThaiTemplateJSON))
	if err != nil {
		t.Fatalf("parse justified-thai template: %v", err)
	}
	el := justifiedThaiElement(t, id)
	chain, styled, err := fontChain(tpl, el)
	if err != nil {
		t.Fatalf("element %s: %v", id, err)
	}
	value := el.Value.Value
	fs, cache := testShippedFontSet(), newFontCache()
	segs, _, err := shapeSegments(id, chain, styled, value, fs, cache, breaksAreConsumed)
	if err != nil {
		t.Fatalf("element %s: %v", id, err)
	}
	ops := text.Opportunities(text.Dictionary(), value, nil)
	width, fontSize := justifiedThaiDeclaredBox(t, id)
	return packLines(segs, ops, len([]rune(value)), fontSize, width), ops
}

// justifiedThaiInteriorOpportunities counts the break opportunities
// strictly INSIDE one packed line, by the same predicate
// justifiedLinePieces uses to cut a line into pieces
// (op.LineEnd > ln.from && op.LineEnd < ln.to). It is the quantity
// AC-TH2's ragged condition is zero of.
func justifiedThaiInteriorOpportunities(ln wrappedLine, ops []text.Opportunity) int {
	n := 0
	for _, op := range ops {
		if op.LineEnd > ln.from && op.LineEnd < ln.to {
			n++
		}
	}
	return n
}

// TestJustifiedThaiSemanticAcceptance is D-000.22's semantic acceptance
// step for fixtures/justified-thai/, performed AT FIRST RECORDING — "a
// wrong first recording is not a bug that gets caught later, it is a bug
// that gets RATIFIED later".
func TestJustifiedThaiSemanticAcceptance(t *testing.T) {
	b := renderJustifiedThai(t)
	assertWellFormedPDF(t, "justified-thai golden fixture render", b, 1)

	runs := readEmittedRuns(t, b)
	if len(runs) == 0 {
		t.Fatal("the justified-thai render emitted no text runs, so every assertion below would be vacuous")
	}
	justifiedThaiAssertGeometry(t, runs, t.Errorf)

	if !containsFontFile2(b) {
		t.Fatal("the justified-thai render carries no FontFile2 — the fixture would certify nothing about embedding")
	}
	if !strings.Contains(string(b), "NotoSansThai") {
		t.Error("the render does not name the embedded Thai face NotoSansThai — a Thai justification fixture that drew no Thai would certify nothing")
	}
	assertToUnicodeSectionsUnderCap(t, b)

	t.Logf("justified-thai semantic acceptance: %d text runs across %d drawn baselines; e1's per-line run counts are %v against the control's one per line", len(runs), len(linesByOrigin(runs)), justifiedThaiRunsPerBaseline()[:len(justifiedThaiLines)])
}

// TestJustifiedThaiIsJustifiedAcrossDictionaryGaps is AC-TH1: a Thai
// line with no space in it is justified across the opportunities the
// shipped dictionary proposed, and its right edge meets the declared
// width EXACTLY, asserted at the same standard as the Latin case and in
// integer millipoints.
//
// The no-space precondition is what makes "dictionary-derived" a
// measured property rather than a claim: if the element's value carries
// no U+0020 at all, every interior opportunity on every one of its lines
// came from AD-25's wordlist.
func TestJustifiedThaiIsJustifiedAcrossDictionaryGaps(t *testing.T) {
	value := justifiedThaiElementValue(t, "e1")
	if strings.ContainsRune(value, ' ') {
		t.Fatalf("presence precondition: e1's value carries a space, so its gaps could be whitespace breaks rather than DICTIONARY-derived ones — this test would then certify nothing about Thai:\n%q", value)
	}
	if !strings.ContainsRune(value, '\n') {
		t.Fatal("presence precondition: e1's value carries no line feed, so the mandatory-break ragged line this fixture claims to cover does not exist")
	}
	ops := text.Opportunities(text.Dictionary(), value, nil)
	if len(ops) == 0 {
		t.Fatalf("presence precondition: the shipped dictionary proposes NO break opportunity anywhere in e1's value, so there is nothing to justify across:\n%q", value)
	}

	byEl, widths := justifiedThaiRunsByElement(t)
	lines := byEl["e1"]
	if len(lines) != len(justifiedThaiLines) {
		t.Fatalf("presence precondition: e1 occupies %d lines, want %d", len(lines), len(justifiedThaiLines))
	}

	// THE CONTROL'S LINES, USED AS THE EXPECTED STRINGS BELOW. e2 is the
	// same value in the same box at the same size with no alignment at
	// all (justifiedThaiControlPremise asserts exactly that), so it packs
	// into the same lines and draws each of them as ONE run — which makes
	// its run text the line's text, straight out of production. That is
	// what lets the piece-concatenation check below have an expected
	// value that was never hand-transcribed: transcribing Thai into a
	// test is precisely how a dropped or reordered combining mark gets
	// into the EXPECTATION as well as the result.
	controlLines := byEl["e2"]
	if len(controlLines) != len(justifiedThaiLines) {
		t.Fatalf("presence precondition: the control e2 occupies %d lines, want the same %d as e1 — it cannot supply e1's expected line texts otherwise", len(controlLines), len(justifiedThaiLines))
	}
	controlText := make([]string, len(justifiedThaiLines))
	for i := range justifiedThaiLines {
		runsOnLine := controlLines[i]
		if len(runsOnLine) != 1 {
			t.Fatalf("presence precondition: the control e2's line %d is drawn as %d runs, want exactly 1 — a multi-run control cannot state the line's text as a single string", i, len(runsOnLine))
		}
		controlText[i] = runsOnLine[0].text
		if controlText[i] == "" {
			t.Fatalf("presence precondition: the control e2's line %d carries no text, so comparing e1's pieces against it would certify nothing", i)
		}
	}

	for i, want := range justifiedThaiLines {
		// THE TABLE MUST BE INTERNALLY CONSISTENT BEFORE IT IS USED. A
		// row authored as justified (no ragged reason) with gaps: 0
		// would divide by zero at the ordered-remainder rule below and
		// abort the test binary instead of reporting; say so here.
		if want.ragged == "" && want.gaps == 0 {
			t.Errorf("justifiedThaiLines row %d declares gaps: 0 with no ragged reason — a justified line has at least one interior gap, and the base/remainder arithmetic below would divide by zero on this row", i)
			continue
		}
		lineRuns := lines[i]
		lineWidths := widths["e1"][i]
		if len(lineRuns) != want.pieces {
			t.Errorf("line %d is drawn as %d pieces, want %d", i, len(lineRuns), want.pieces)
			continue
		}

		// (0) THE PIECES STILL SPELL THE LINE. lineRuns is already in
		//     ascending x (justifiedThaiRunsByElement sorts it), and
		//     splitting Thai into per-word runs is exactly where a
		//     combining mark can be dropped or reordered across a piece
		//     boundary. Without this the only thing that would notice is
		//     the golden digest, which reports "hash mismatch" and not
		//     what changed. Asserted for the ragged lines too: they are
		//     single runs, so the check is free there.
		var spelled strings.Builder
		for _, run := range lineRuns {
			spelled.WriteString(run.text)
		}
		if got := spelled.String(); got != controlText[i] {
			t.Errorf("line %d's %d pieces concatenate to %q, but the control e2 sets the same line as %q — justification MOVES the pieces of a line, it never changes what the line says", i, len(lineRuns), got, controlText[i])
		}

		if want.ragged != "" {
			if got := int64(lineRuns[0].x); got != justifiedThaiRunLeftMP {
				t.Errorf("line %d is ragged (%s) and must sit at the element's natural start edge %d, got %d", i, want.ragged, justifiedThaiRunLeftMP, got)
			}
			continue
		}

		// (a) THE RIGHT EDGE LANDS ON THE DECLARED WIDTH, EXACTLY.
		last := len(lineRuns) - 1
		if got := int64(lineRuns[last].x) + int64(lineWidths[last]); got != justifiedThaiRunRightMP {
			t.Errorf("line %d's last piece ends at x %d, want the element's declared right edge %d exactly", i, got, justifiedThaiRunRightMP)
		}

		// (b) THE DISTRIBUTED AMOUNTS SUM TO THE SLACK, EXACTLY.
		gaps := make([]int64, 0, want.gaps)
		var distributed int64
		for j := 1; j < len(lineRuns); j++ {
			gap := int64(lineRuns[j].x) - (int64(lineRuns[j-1].x) + int64(lineWidths[j-1]))
			gaps = append(gaps, gap)
			distributed += gap
		}
		if len(gaps) != want.gaps {
			t.Errorf("line %d has %d gaps, want %d", i, len(gaps), want.gaps)
			continue
		}
		if distributed != want.slack {
			t.Errorf("line %d distributed %d millipoints across its gaps, want the slack %d exactly — a discarded remainder is a right edge that misses", i, distributed, want.slack)
		}

		// (c) THE ORDERED REMAINDER RULE: base to every gap, and one
		//     extra millipoint to the first `remainder` gaps in reading
		//     order. Thai reads left to right, so ascending x IS reading
		//     order here exactly as it is for Latin.
		base := want.slack / int64(want.gaps)
		remainder := want.slack - base*int64(want.gaps)
		if remainder == 0 {
			t.Errorf("line %d's slack %d divides its %d gaps exactly — this case cannot distinguish an ordered remainder rule from an unordered one", i, want.slack, want.gaps)
		}
		for j, gap := range gaps {
			wantGap := base
			if int64(j) < remainder {
				wantGap = base + 1
			}
			if gap != wantGap {
				t.Errorf("line %d gap %d is %d, want %d (base %d, remainder %d to the FIRST %d gaps in ascending position)", i, j, gap, wantGap, base, remainder, remainder)
			}
		}
	}
}

// TestJustifiedThaiRaggedLinesAreRaggedForTheReasonsClaimed asserts the
// CAUSES justifiedThaiLines records for its two ragged rows, which the
// geometry assertions above cannot see.
//
// Drawing each of them as one run at the natural start edge is what
// EVERY ragged condition produces, so the geometry alone cannot tell
// them apart: a build that set line 2 ragged by misidentifying it as the
// last line, or by losing its interior opportunities altogether, draws
// exactly what a correct build draws. Claiming the cause in a comment
// and asserting only the shared consequence is the same defect class as
// documentation claiming a property is covered when it is not.
//
// So each row's stated cause is asserted from the packer's own record,
// and — because a zero-gap line would be ragged whatever its break kind
// or index — both rows are also asserted to HAVE interior break
// opportunities, which is what separates them from AC-TH2's case below.
func TestJustifiedThaiRaggedLinesAreRaggedForTheReasonsClaimed(t *testing.T) {
	lines, ops := justifiedThaiPackedLines(t, "e1")
	if len(lines) != len(justifiedThaiLines) {
		t.Fatalf("presence precondition: e1 packs into %d lines, want the %d justifiedThaiLines states", len(lines), len(justifiedThaiLines))
	}

	var raggedAt []int
	for i, want := range justifiedThaiLines {
		if want.ragged != "" {
			raggedAt = append(raggedAt, i)
		}
	}
	if len(raggedAt) != 2 {
		t.Fatalf("presence precondition: justifiedThaiLines records %d ragged lines, want exactly 2 — the mandatory-break one and the last one, which are the two causes this test separates", len(raggedAt))
	}
	mandatoryLine, lastLine := raggedAt[0], raggedAt[1]

	// CAUSE 1: THE AUTHOR TYPED THE BREAK. Read from wrappedLine.endedBy,
	// the field Story 7.1 wrote for exactly this consumer — and NOT from
	// the line's index, which would answer the other question.
	if got := lines[mandatoryLine].endedBy; got != text.BreakMandatory {
		t.Errorf("line %d is recorded ragged because %q, but the packer says break kind %v ended it, not text.BreakMandatory — this line is ragged for some OTHER reason and the fixture's account of it is wrong", mandatoryLine, justifiedThaiLines[mandatoryLine].ragged, got)
	}
	if mandatoryLine == len(lines)-1 {
		t.Errorf("line %d is recorded ragged as a NOT-last line ended by a typed break, but it IS the last of %d lines — the two ragged conditions are not separated by this fixture at all", mandatoryLine, len(lines))
	}

	// CAUSE 2: IT IS THE LAST LINE. Derived from the INDEX, because a
	// line no break ended carries the zero value text.BreakOptional and
	// endedBy cannot answer this.
	if lastLine != len(lines)-1 {
		t.Errorf("line %d is recorded ragged because it is THE LAST LINE, but e1 packs into %d lines, so the last one is line %d", lastLine, len(lines), len(lines)-1)
	}
	if got := lines[lastLine].endedBy; got == text.BreakMandatory {
		t.Errorf("the last line %d was ended by a mandatory break, so it satisfies BOTH ragged conditions at once and witnesses neither of them on its own", lastLine)
	}

	// AND NEITHER IS THE ZERO-GAP CASE. If a supposedly mandatory-break
	// or last line held no interior break opportunity, it would be ragged
	// under AC-TH2's condition no matter what this test asserted above,
	// and the cause named would be unfalsifiable.
	for _, i := range raggedAt {
		if interior := justifiedThaiInteriorOpportunities(lines[i], ops); interior == 0 {
			t.Errorf("ragged line %d (%s) holds ZERO interior break opportunities, so it would be set ragged by AC-TH2's zero-gap condition regardless of its break kind or its index — the cause this fixture records for it is not what is being witnessed", i, justifiedThaiLines[i].ragged)
		}
	}

	t.Logf("e1's ragged lines: line %d ended by %v at index %d of %d with %d interior opportunities; line %d ended by %v at index %d of %d with %d interior opportunities",
		mandatoryLine, lines[mandatoryLine].endedBy, mandatoryLine, len(lines), justifiedThaiInteriorOpportunities(lines[mandatoryLine], ops),
		lastLine, lines[lastLine].endedBy, lastLine, len(lines), justifiedThaiInteriorOpportunities(lines[lastLine], ops))
}

// TestJustifiedThaiAtomicRunHasNowhereToPutSlack is AC-TH2, and it is a
// TEST rather than a comment on purpose: the third ragged condition —
// a line with ZERO interior break opportunities — was the one the
// original acceptance criteria were silent on, and AD-25's atomic
// unknown Thai run is what makes it reachable at all.
//
// The case is built so that the OTHER two ragged conditions cannot
// account for the result, and each of those exclusions is asserted:
// the line is not the last (the element wraps to two), it was not ended
// by a mandatory break (the value carries no line feed), the box has a
// declared width, and the slack is POSITIVE. What is left is the gap
// count, and the precondition that it is zero is asserted with t.Fatalf
// — MEASURED against text.Dictionary() through the production
// text.Opportunities, never assumed — so this case cannot go vacuous if
// the segmenter's answer for this run ever changes.
//
// WHAT MAKES THE RUN ATOMIC IS THE SEGMENTER'S ANSWER, NOT A HOLE IN THE
// WORDLIST. The shipped wordlist does hold "กานต์", a suffix of this run;
// a reader who greps for it and concludes this test is broken has
// checked the wrong thing. The asserted property is that the segmenter
// proposes no opportunity strictly inside the run — which is what the
// justification rule reads — whatever individual entries the wordlist
// happens to contain.
func TestJustifiedThaiAtomicRunHasNowhereToPutSlack(t *testing.T) {
	value := justifiedThaiElementValue(t, "e3")
	if strings.ContainsRune(value, '\n') {
		t.Fatalf("presence precondition: e3's value carries a line feed, so a ragged line here could be the MANDATORY-BREAK condition rather than the zero-gap one:\n%q", value)
	}

	byEl, widths := justifiedThaiRunsByElement(t)
	lines := byEl["e3"]
	if len(lines) != len(justifiedThaiAtomicLines) {
		t.Fatalf("presence precondition: e3 occupies %d lines, want %d — with one line the zero-gap condition would be indistinguishable from the LAST-LINE condition", len(lines), len(justifiedThaiAtomicLines))
	}

	first := lines[0]
	if len(first) != 1 {
		t.Fatalf("e3's first line is drawn as %d runs, want exactly 1 — an atomic unknown Thai run has nowhere to place slack and must be set whole", len(first))
	}
	if got := first[0].text; got != justifiedThaiAtomicFirstLine {
		t.Fatalf("presence precondition: e3's first line reads %q, want %q — the rest of this test is stated about that run", got, justifiedThaiAtomicFirstLine)
	}

	// THE PRECONDITION THIS CASE EXISTS FOR: zero interior break
	// opportunities inside the first line's own rune range, asked of the
	// SHIPPED dictionary through the same production call the
	// justification rule reads. t.Fatalf, not t.Errorf: if the segmenter
	// ever proposes one here, the case has stopped being AD-25's atomic
	// run and must be re-chosen, not quietly passed.
	lineRunes := len([]rune(justifiedThaiAtomicFirstLine))
	interior := 0
	for _, op := range text.Opportunities(text.Dictionary(), value, nil) {
		if op.LineEnd > 0 && op.LineEnd < lineRunes {
			interior++
		}
	}
	if interior != 0 {
		t.Fatalf("presence precondition: the shipped segmenter now proposes %d interior break opportunit(ies) inside %q, so it is no longer AD-25's ATOMIC UNKNOWN RUN and this test no longer covers the zero-gap ragged condition — pick a run the segmenter proposes none inside (which is not the same as, and must not be checked by, grepping the wordlist)", interior, justifiedThaiAtomicFirstLine)
	}

	// AND THE SLACK IS POSITIVE, so the slack-only rule is not what
	// produced the ragged result either.
	slack := justifiedThaiAtomicBoxWidthMP - int64(widths["e3"][0][0])
	if slack <= 0 {
		t.Fatalf("presence precondition: e3's first line measures %d in a %d box, leaving slack %d — with no slack to place, the zero-gap condition would not be what set this line ragged", int64(widths["e3"][0][0]), justifiedThaiAtomicBoxWidthMP, slack)
	}

	// THE RESULT: the natural start edge, exactly like a last line.
	if got := int64(first[0].x); got != justifiedThaiRunLeftMP {
		t.Errorf("e3's first line is drawn at x %d, want the element's natural start edge %d — a justified line with no interior break opportunity has nowhere to put its %d millipoints of slack", got, justifiedThaiRunLeftMP, slack)
	}

	t.Logf("AC-TH2: %q is atomic under the shipped segmenter (0 interior opportunities over %d runes), is line 0 of %d, was not ended by a mandatory break, and leaves %d mp of slack in its %d mp box — and is still set at the natural start edge",
		justifiedThaiAtomicFirstLine, lineRunes, len(lines), slack, justifiedThaiAtomicBoxWidthMP)
}

// TestJustifiedThaiFixtureDeclaresTheVersionItsContentRequires pins the
// same half of D-1.4.13 fixtures/justified-text/ witnesses: the document
// uses the 2.0 alignment value, so it declares 2.0 — and re-serializing
// it neither raises it further (the library's ceiling is not a
// document's version) nor lowers it.
func TestJustifiedThaiFixtureDeclaresTheVersionItsContentRequires(t *testing.T) {
	if !strings.Contains(justifiedThaiTemplateJSON, `"version": "2.0"`) {
		t.Fatal(`the justified-thai fixture sets style.align: "justify", which extends a CLOSED SET, so it must DECLARE 2.0 — a document that uses a 2.0 value while declaring 1.x is the misdeclaration D-7.2.1 exists to end`)
	}
	d, err := template.ParseDocument([]byte(justifiedThaiTemplateJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := template.SerializeDocument(d)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if !strings.Contains(string(out), `"version": "2.0"`) {
		t.Fatalf("re-serializing the fixture must keep 2.0:\n%s", out)
	}
}

// TestJustifiedThaiGoldenFixture is the byte-identity half: the live
// render must reproduce fixtures/justified-thai/expected.pdf exactly.
//
// It runs AFTER the semantic assertions above in file order and in
// intent (D-000.22): a hash frozen before anyone checked what it
// contained certifies only that the bytes have not changed.
func TestJustifiedThaiGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	dir := filepath.Join(root, "fixtures", "justified-thai")

	inputPath := filepath.Join(dir, "input.folio")
	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read %s: %v", inputPath, err)
	}
	if string(inputBytes) != justifiedThaiTemplateJSON {
		t.Fatalf(
			"%s has drifted from folio-go/justifiedThaiTemplateJSON (justified_thai_template.go) — the two are "+
				"supposed to be byte-identical (kept in sync by hand, per line-spacing's precedent)",
			inputPath,
		)
	}

	fixture := loadExpectedFixture(t, filepath.Join(dir, "expected.json"))
	if fixture.Folio8GoVersion == "" {
		t.Fatal("fixture is missing folio8GoVersion")
	}
	if fixture.GoToolchain == "" {
		t.Fatal("fixture is missing goToolchain")
	}
	if !isSHA256HexString(fixture.SHA256) {
		t.Fatalf("fixture sha256 %q is not 64 lower-case hex characters", fixture.SHA256)
	}

	b := renderJustifiedThai(t)
	got := sha256Hex(b)

	expectedPDF, err := os.ReadFile(filepath.Join(dir, "expected.pdf"))
	if err != nil {
		t.Fatalf("read expected.pdf: %v", err)
	}
	if onDisk := sha256Hex(expectedPDF); onDisk != fixture.SHA256 {
		t.Fatalf(
			"fixtures/justified-thai/expected.pdf's own sha256 (%s) does not match expected.json's recorded sha256 (%s) — the fixture's two halves have drifted apart",
			onDisk, fixture.SHA256,
		)
	}
	if got != fixture.SHA256 {
		t.Fatalf(
			"golden fixture mismatch: got sha256 %s, want %s (fixtures/justified-thai). Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change. Do not regenerate the fixture to make this pass.",
			got, fixture.SHA256,
		)
	}
}
