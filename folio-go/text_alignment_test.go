package folio8

import (
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
	"github.com/panitw/folio8/folio-go/internal/text"
)

// alignedTemplate is one band holding four text elements that differ ONLY in
// their committed alignment: same value, same face, same size, same box. Any
// difference in where they are drawn is therefore the alignment rule and
// nothing else.
const alignedTemplateJSON = `{"version":"1.0","page":{"size":"A4","orientation":"portrait","margin":{"top":36,"right":36,"bottom":36,"left":36}},"bands":{"pageHeader":{"height":20,"elements":[]},"content":{"elements":[
{"id":"e1","type":"text","x":10,"y":0,"width":200,"height":40,"value":"Total","style":{"fontFamily":"body","fontSize":12}},
{"id":"e2","type":"text","x":10,"y":50,"width":200,"height":40,"value":"Total","style":{"fontFamily":"body","fontSize":12,"align":"center"}},
{"id":"e3","type":"text","x":10,"y":100,"width":200,"height":40,"value":"Total","style":{"fontFamily":"body","fontSize":12,"align":"right"}},
{"id":"e4","type":"text","x":10,"y":150,"width":200,"height":40,"value":"Total","style":{"fontFamily":"body","fontSize":12,"valign":"bottom"}},
{"id":"e5","type":"text","x":10,"y":200,"width":200,"height":40,"value":"Total","style":{"fontFamily":"body","fontSize":12,"valign":"middle"}}
]},"pageFooter":{"height":20,"elements":[]}},"fonts":{"body":["Roboto-Regular"]},"locale":"en","utcOffset":"+00:00","assets":{},"nextId":6}`

func TestTextAlignmentDistributesSlackOnly(t *testing.T) {
	for _, c := range []struct {
		name                  string
		align                 string
		box, line, wantOffset geom.Length
	}{
		{"unset is the start edge", "", 200_000, 50_000, 0},
		{"left is the start edge", "left", 200_000, 50_000, 0},
		{"right takes all the slack", "right", 200_000, 50_000, 150_000},
		{"center halves the slack", "center", 200_000, 50_000, 75_000},
		{"an exact half rounds to even, down", "center", 200_000, 199_999, 0},
		{"an exact half rounds to even, up", "center", 200_000, 199_997, 2},
		{"a line that exactly fills has no slack", "center", 200_000, 200_000, 0},
		{"an overflowing line keeps the start edge", "right", 200_000, 260_000, 0},
		{"an undeclared width has no box to align in", "right", 0, 50_000, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := textAlignOffset(c.align, c.box, c.line); got != c.wantOffset {
				t.Errorf("textAlignOffset(%q, %d, %d) = %d, want %d", c.align, c.box, c.line, got, c.wantOffset)
			}
		})
	}
	for _, c := range []struct {
		name                   string
		valign                 string
		box, block, wantOffset geom.Length
	}{
		{"unset is the top edge", "", 40_000, 14_000, 0},
		{"top is the top edge", "top", 40_000, 14_000, 0},
		{"bottom takes all the slack", "bottom", 40_000, 14_000, 26_000},
		{"middle halves the slack", "middle", 40_000, 14_000, 13_000},
		{"an exact half rounds to even, down", "middle", 40_000, 39_999, 0},
		{"an exact half rounds to even, up", "middle", 40_000, 39_997, 2},
		{"a block taller than its box keeps the top edge", "middle", 40_000, 60_000, 0},
		{"an undeclared height has no box to align in", "bottom", 0, 14_000, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := textValignOffset(c.valign, c.box, c.block); got != c.wantOffset {
				t.Errorf("textValignOffset(%q, %d, %d) = %d, want %d", c.valign, c.box, c.block, got, c.wantOffset)
			}
		})
	}
}

// TestTextBlockHeightIsTheRuledModelsOwnExtent pins the one derivation valign
// depends on: the spans the packer and the page-splitter already use, never a
// re-measurement of the text.
func TestTextBlockHeightIsTheRuledModelsOwnExtent(t *testing.T) {
	vm := verticalMetrics{FirstBaseline: 11_000, Advance: 14_000, LastDescent: 3_000}
	for _, c := range []struct {
		lines int
		want  geom.Length
	}{{0, 0}, {1, 14_000}, {2, 28_000}, {3, 42_000}} {
		if got := textBlockHeight(c.lines, vm); got != c.want {
			t.Errorf("textBlockHeight(%d) = %d, want %d", c.lines, got, c.want)
		}
	}
}

// TestAlignedTextElementsMoveInsideTheirDeclaredBox is the production-path
// assertion: the same string, in the same box, drawn at three horizontal and
// three vertical alignments, lands where the slack rule says it lands.
func TestAlignedTextElementsMoveInsideTheirDeclaredBox(t *testing.T) {
	tpl, err := ParseTemplate([]byte(alignedTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	data := emptyBindValue(t)
	runs, err := collectTextRuns(tpl, data, data, testFontSet(), newFontCache())
	if err != nil {
		t.Fatal(err)
	}
	byElement := make(map[string]textRunSource, len(runs))
	for _, run := range runs {
		if _, seen := byElement[run.elementID]; seen {
			t.Fatalf("element %s produced more than one run; this fixture is meant to be one line, one face", run.elementID)
		}
		byElement[run.elementID] = run
	}
	if len(byElement) != 5 {
		t.Fatalf("presence precondition: got runs for %d elements, want 5", len(byElement))
	}
	width, err := shippingRunWidth(byElement["e1"], testFontSet(), newFontCache())
	if err != nil {
		t.Fatal(err)
	}
	const box = geom.Length(200_000)
	slack := box - width
	if slack <= 0 {
		t.Fatalf("presence precondition: %q measures %d in a %d box, leaving no slack to distribute", "Total", width, box)
	}
	left := byElement["e1"].x
	if got, want := byElement["e2"].x, left+geom.ScaleRound(slack, 1, 2); got != want {
		t.Errorf("centred x = %d, want %d", got, want)
	}
	if got, want := byElement["e3"].x, left+slack; got != want {
		t.Errorf("right-aligned x = %d, want %d", got, want)
	}

	// Vertical: e1/e4/e5 sit at declared y of 0/150/200 in the same band, so
	// each one's own top is its declared origin plus its valign offset.
	vm, err := chainVerticalModel([]string{"Roboto-Regular"}, 12_000, defaultLineSpacing, testFontSet(), newFontCache())
	if err != nil {
		t.Fatal(err)
	}
	block := textBlockHeight(1, vm)
	const boxHeight = geom.Length(40_000)
	if got, want := byElement["e4"].y, byElement["e1"].y+150_000+(boxHeight-block); got != want {
		t.Errorf("bottom-aligned y = %d, want %d", got, want)
	}
	if got, want := byElement["e5"].y, byElement["e1"].y+200_000+geom.ScaleRound(boxHeight-block, 1, 2); got != want {
		t.Errorf("middle-aligned y = %d, want %d", got, want)
	}
}

// TestCanvasPaintMatchesTheShippingRunPathUnderAlignment is the parity claim
// that matters for the designer: the canvas draws an aligned element exactly
// where the PDF producer draws it, from the same committed style, with no
// second alignment rule in the browser.
func TestCanvasPaintMatchesTheShippingRunPathUnderAlignment(t *testing.T) {
	tpl, err := ParseTemplate([]byte(alignedTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	projection, err := canvasWithTextPaint(tpl, testFontSet())
	if err != nil {
		t.Fatal(err)
	}
	data := emptyBindValue(t)
	runs, err := collectTextRuns(tpl, data, data, testFontSet(), newFontCache())
	if err != nil {
		t.Fatal(err)
	}
	shipping := make(map[string]textRunSource, len(runs))
	for _, run := range runs {
		shipping[run.elementID] = run
	}
	bandY := make(map[string]int64)
	for _, band := range projection.Bands {
		bandY[band.Name] = band.Y
	}
	checked := 0
	for _, component := range projection.Components {
		paint := component.TextPaint
		if paint == nil || len(paint.Lines) == 0 {
			t.Fatalf("component %s has no paint to compare", component.ID)
		}
		run, ok := shipping[component.ID]
		if !ok {
			t.Fatalf("component %s has paint but no shipping run", component.ID)
		}
		bandOrigin := bandY[component.Band] - projection.MarginTop
		if got, want := paint.Lines[0].Fragments[0].X, int64(run.x); got != want {
			t.Errorf("component %s canvas x = %d, want the shipping run's %d", component.ID, got, want)
		}
		if got, want := paint.Lines[0].Top, int64(run.y)-bandOrigin; got != want {
			t.Errorf("component %s canvas top = %d, want the shipping run's %d", component.ID, got, want)
		}
		checked++
	}
	if checked != 5 {
		t.Fatalf("compared %d components, want 5", checked)
	}
}

// TestCanvasProjectionCarriesTheDeclaredFamiliesAndTheDefaultSize covers the
// two values the designer's typography controls are built from: the closed
// set style.fontFamily may name in this document, and the size the producer
// uses when an element commits none.
func TestCanvasProjectionCarriesTheDeclaredFamiliesAndTheDefaultSize(t *testing.T) {
	tpl, err := ParseTemplate([]byte(`{"version":"1.0","page":{"size":"A4","orientation":"portrait","margin":{"top":36,"right":36,"bottom":36,"left":36}},"bands":{"pageHeader":{"height":20,"elements":[]},"content":{"elements":[]},"pageFooter":{"height":20,"elements":[]}},"fonts":{"heading":["Roboto-Regular"],"body":["Roboto-Regular"]},"locale":"en","utcOffset":"+00:00","assets":{},"nextId":1}`))
	if err != nil {
		t.Fatal(err)
	}
	projection, err := canvas(tpl)
	if err != nil {
		t.Fatal(err)
	}
	if len(projection.FontFamilies) != 2 || projection.FontFamilies[0] != "body" || projection.FontFamilies[1] != "heading" {
		t.Errorf("font families = %#v, want the declared chains, sorted", projection.FontFamilies)
	}
	for _, name := range projection.FontFamilies {
		if !knownFontFamily(tpl, name) {
			t.Errorf("projected family %q is not one a fontFamily command would accept", name)
		}
	}
	if projection.DefaultFontSize != int64(defaultFontSizePt) {
		t.Errorf("default font size = %d, want the producer's own %d", projection.DefaultFontSize, defaultFontSizePt)
	}
}

// justifyTemplateJSON is one band holding TWO text elements: e1, which
// is justified, and e2, which declares no align at all and is the
// CONTROL. They are the same string in the same box at the same size in
// the same face, so any difference between them is the justification
// rule and nothing else — which is exactly what
// TestCanvasPaintMatchesTheShippingRunPathUnderJustification's vacuity
// guard compares, fragment count against fragment count.
//
// What it covers is the PRODUCTION path end to end — parse, collect the
// shipping runs, project the canvas — over a multi-line justified
// element whose interior lines are drawn as several pieces and whose
// last line is ragged.
//
// What it does NOT cover, deliberately: the three ragged conditions, the
// no-declared-width case and the overflow case. Those cannot be reached
// from one rendered document at once, so they are driven directly
// against the shared rule by
// TestJustifiedLinePiecesLeavesEveryRaggedCaseAtTheStartEdge and by
// TestJustifiedLastLineThatIsAlsoMandatoryBreakEndedIsRaggedByEitherCondition.
//
// It declares 2.0 because `align: "justify"` is what requires it.
const justifyTemplateJSON = `{"version":"2.0","page":{"size":"A4","orientation":"portrait","margin":{"top":36,"right":36,"bottom":36,"left":36}},"bands":{"pageHeader":{"height":20,"elements":[]},"content":{"elements":[
{"id":"e1","type":"text","x":10,"y":0,"width":120,"height":60,"value":"alpha beta gamma delta epsilon","style":{"fontFamily":"body","fontSize":12,"align":"justify"}},
{"id":"e2","type":"text","x":10,"y":70,"width":120,"height":60,"value":"alpha beta gamma delta epsilon","style":{"fontFamily":"body","fontSize":12}}
]},"pageFooter":{"height":20,"elements":[]}},"fonts":{"body":["Roboto-Regular"]},"locale":"en","utcOffset":"+00:00","assets":{},"nextId":3}`

// justifyProbe is one REAL packed line of REAL shaped text, together
// with everything the shared justification rule takes, and the summed
// width of the pieces that rule itself divides the line into.
//
// It exists so the remainder rule can be exercised at a CHOSEN slack
// over a CHOSEN number of gaps without any of the arithmetic being
// restated in the test: the box width is the pieces' own natural sum
// plus the slack under test, so `slack = boxWidth - Σ pieceWidths` —
// the rule's own definition — comes out at exactly that number.
type justifyProbe struct {
	segs     []faceSegment
	ops      []text.Opportunity
	ln       wrappedLine
	index    int
	count    int
	fontSize geom.Length
	// natural is Σ of the pieces' own measured widths, by
	// measureRuneRange, which is positionSegments' cursor arithmetic.
	natural geom.Length
}

// justifyProbeLineWithGaps searches a genuinely packed paragraph for an
// interior, optional-ended line that the PRODUCTION rule divides into
// exactly gaps+1 pieces. The gap count is read off justifiedLinePieces
// itself rather than counted here, so the probe cannot disagree with the
// rule it is about to interrogate.
func justifyProbeLineWithGaps(t *testing.T, gaps int) justifyProbe {
	t.Helper()
	fs, cache := testShippedFontSet(), newFontCache()
	const value = "alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu nu xi omicron"
	segs, _, err := shapeSegments("e1", []string{"Noto Sans"}, nil, value, fs, cache, breaksAreConsumed)
	if err != nil {
		t.Fatal(err)
	}
	ops := text.Opportunities(text.Dictionary(), value, nil)
	const fontSize = geom.Length(11_000)
	// Wide enough that every candidate line has slack, so the piece
	// partition comes back and its gap count can be read.
	const wide = geom.Length(1 << 30)
	for packWidth := geom.Length(40_000); packWidth <= 400_000; packWidth += 2_000 {
		lines := packLines(segs, ops, len([]rune(value)), fontSize, packWidth)
		for i := 0; i < len(lines)-1; i++ {
			if lines[i].endedBy == text.BreakMandatory {
				continue
			}
			pieces := justifiedLinePieces(alignJustify, lines[i], i, len(lines), segs, ops, fontSize, wide)
			if len(pieces) != gaps+1 {
				continue
			}
			var natural geom.Length
			for _, piece := range pieces {
				natural += measureRuneRange(segs, piece.from, piece.to, fontSize)
			}
			return justifyProbe{segs: segs, ops: ops, ln: lines[i], index: i, count: len(lines), fontSize: fontSize, natural: natural}
		}
	}
	t.Fatalf("presence precondition: no interior, optional-ended line of the probe paragraph divides into %d gaps at any packing width, so the case below cannot be stated over real shaped text", gaps)
	return justifyProbe{}
}

// grants drives the PRODUCTION rule at a box width leaving exactly
// `slack`, and recovers what each gap ACTUALLY received from the
// returned piece positions: a gap's grant is the distance between a
// piece's start and the previous piece's natural end. Nothing about the
// division is recomputed here.
//
// It returns nil for a line the rule left ragged.
func (p justifyProbe) grants(t *testing.T, slack int64) []int64 {
	t.Helper()
	box := p.natural + geom.Length(slack)
	pieces := justifiedLinePieces(alignJustify, p.ln, p.index, p.count, p.segs, p.ops, p.fontSize, box)
	if pieces == nil {
		return nil
	}
	width := func(piece justifiedPiece) geom.Length {
		return measureRuneRange(p.segs, piece.from, piece.to, p.fontSize)
	}
	if pieces[0].offset != 0 {
		t.Errorf("the first piece is at %d, want the element's own start edge 0", pieces[0].offset)
	}
	out := make([]int64, 0, len(pieces)-1)
	for i := 1; i < len(pieces); i++ {
		out = append(out, int64(pieces[i].offset-(pieces[i-1].offset+width(pieces[i-1]))))
	}
	last := pieces[len(pieces)-1]
	if got := last.offset + width(last); got != box {
		t.Errorf("the last piece ends at %d, want the declared width %d exactly", got, box)
	}
	return out
}

// TestJustifiedLinePiecesRemainderRule drives the PRODUCTION remainder
// rule — justifiedLinePieces itself, over really shaped, really packed
// text — and reads each gap's granted amount back out of the piece
// positions it returns. Nothing here recomputes base or the remainder:
// a test that transcribed the division would stay green against ANY
// implementation of it, including a deleted one.
//
// The three worked examples are the ones the story's Design Notes and
// the I/O matrix name: slack 7 over 3 gaps gives 3, 2, 2; slack 6 over
// 3 gives 2, 2, 2; slack 2 over 3 gives 1, 1, 0 — and a gap
// legitimately receiving nothing is not a defect. The single-gap
// boundary and the no-slack case are stated beside them.
//
// EVERY PAIR BELOW IS REALISED THROUGH REAL SHAPED TEXT. The gap count
// is whatever the packer and the rule between them produce for a line
// of the probe paragraph, and the slack is set by declaring a box that
// is exactly that many millipoints wider than the pieces' own summed
// width — which is the rule's own definition of slack, so no pair had
// to be asserted through anything narrower.
func TestJustifiedLinePiecesRemainderRule(t *testing.T) {
	for _, c := range []struct {
		name  string
		gaps  int
		slack int64
		want  []int64
	}{
		{"the matrix's own 7 over 3", 3, 7, []int64{3, 2, 2}},
		{"6 over 3 divides exactly", 3, 6, []int64{2, 2, 2}},
		{"2 over 3 leaves the last gap nothing", 3, 2, []int64{1, 1, 0}},
		{"5 over 2", 2, 5, []int64{3, 2}},
		{"one gap takes the whole slack", 1, 7, []int64{7}},
		{"a line with no slack is ragged", 3, 0, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			probe := justifyProbeLineWithGaps(t, c.gaps)
			got := probe.grants(t, c.slack)
			if c.want == nil {
				if got != nil {
					t.Fatalf("slack %d over %d gaps distributed %v; a line with no slack must be set at the natural start edge", c.slack, c.gaps, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("slack %d over %d gaps came back RAGGED; every assertion below would be vacuous", c.slack, c.gaps)
			}
			if len(got) != c.gaps {
				t.Fatalf("the rule drew %d pieces' worth of gaps, want %d", len(got), c.gaps)
			}
			// THE AMOUNTS, as granted by the production rule.
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("slack %d over %d gaps granted %v, want %v", c.slack, c.gaps, got, c.want)
				}
			}
			// THE ORDER, asserted as a property rather than against the
			// literals above: the additional millipoints go to the
			// EARLIEST gaps in ascending position along the line, so the
			// grants never rise and never differ by more than one.
			var total int64
			for i, g := range got {
				total += g
				if g < 0 {
					t.Errorf("gap %d was granted %d; nothing may distribute a negative", i, g)
				}
				if i > 0 {
					if got[i-1] < g {
						t.Errorf("gap %d was granted %d and the EARLIER gap %d only %d — the remainder goes to the first gaps in ascending position along the line", i, g, i-1, got[i-1])
					}
					if got[i-1]-g > 1 {
						t.Errorf("gap %d was granted %d and gap %d %d — every gap receives the same base and at most one additional millipoint", i-1, got[i-1], i, g)
					}
				}
			}
			// AND THE TOTAL IS EXACT: no float, no discarded remainder.
			if total != c.slack {
				t.Errorf("the granted amounts sum to %d, want the slack %d exactly", total, c.slack)
			}
		})
	}
}

// TestJustifiedLinePiecesLeavesEveryRaggedCaseAtTheStartEdge drives the
// shared rule directly over the packer's own output, because three of
// these five conditions cannot be reached from any single rendered
// document at once.
func TestJustifiedLinePiecesLeavesEveryRaggedCaseAtTheStartEdge(t *testing.T) {
	fs, cache := testShippedFontSet(), newFontCache()
	const value = "alpha beta gamma delta epsilon zeta\neta theta iota kappa lambda mu nu xi"
	segs, _, err := shapeSegments("e1", []string{"Noto Sans"}, nil, value, fs, cache, breaksAreConsumed)
	if err != nil {
		t.Fatal(err)
	}
	ops := text.Opportunities(text.Dictionary(), value, nil)
	const fontSize = geom.Length(11_000)
	const boxWidth = geom.Length(120_000)
	lines := packLines(segs, ops, len([]rune(value)), fontSize, boxWidth)
	if len(lines) < 3 {
		t.Fatalf("presence precondition: the probe packs into %d lines, want at least 3", len(lines))
	}

	sawJustified, sawMandatory := false, false
	for i, ln := range lines {
		pieces := justifiedLinePieces(alignJustify, ln, i, len(lines), segs, ops, fontSize, boxWidth)
		switch {
		case i == len(lines)-1:
			if pieces != nil {
				t.Errorf("the LAST line was justified; it must be set at the natural start edge, and the condition is DERIVED FROM THE INDEX because a line no break ended carries the zero value BreakOptional")
			}
		case ln.endedBy == text.BreakMandatory:
			sawMandatory = true
			if pieces != nil {
				t.Errorf("line %d was ended by a mandatory break and must be ragged even though it is not the last line", i)
			}
		default:
			if pieces != nil {
				sawJustified = true
			}
		}
	}
	if !sawMandatory {
		t.Fatal("presence precondition: the probe contains no mandatory-break-ended line, so the second ragged condition is untested here")
	}
	if !sawJustified {
		t.Fatal("presence precondition: no line was justified at all, so every assertion above is vacuous")
	}

	// (a) NO DECLARED WIDTH: no box to justify to.
	if pieces := justifiedLinePieces(alignJustify, lines[0], 0, len(lines), segs, ops, fontSize, 0); pieces != nil {
		t.Error("an element with no declared width has no box to justify to and must be set at its natural start edge")
	}
	// (b) NOT JUSTIFIED AT ALL: the three older alignments never reach
	//     the piece path, which is what keeps the corpus byte-identical.
	for _, align := range []string{"", "left", "center", "right"} {
		if pieces := justifiedLinePieces(align, lines[0], 0, len(lines), segs, ops, fontSize, boxWidth); pieces != nil {
			t.Errorf("align %q reached the justified piece path", align)
		}
	}
	// (c) NO INTERIOR OPPORTUNITY: nowhere to place the slack. Stated
	//     because the acceptance criteria are silent on it and AD-25
	//     makes it reachable — an unknown Thai run is atomic and offers
	//     no interior opportunity at all.
	if pieces := justifiedLinePieces(alignJustify, lines[0], 0, len(lines), segs, nil, fontSize, boxWidth); pieces != nil {
		t.Error("a line with zero interior break opportunities has nowhere to put slack and must be set at the natural start edge")
	}
	// (d) NO SLACK: a line that meets or exceeds its declared width keeps
	//     the start edge, where FR44's clip-and-warn applies unchanged.
	if pieces := justifiedLinePieces(alignJustify, lines[0], 0, len(lines), segs, ops, fontSize, 1); pieces != nil {
		t.Error("an overflowing justified line must keep the natural start edge; nothing may distribute a negative")
	}
}

// TestJustifiedLastLineThatIsAlsoMandatoryBreakEndedIsRaggedByEitherCondition
// covers the I/O matrix's "Both conditions at once" row, which no other
// test distinguishes: TestJustifiedLinePiecesLeavesEveryRaggedCaseAtTheStartEdge
// takes its `i == len(lines)-1` arm FIRST, so a line that is both the last
// line AND mandatory-break-ended is never told apart from a last line that
// merely is not.
//
// WHAT packLines ACTUALLY PRODUCES, MEASURED HERE RATHER THAN ASSUMED. A
// trailing line feed does NOT hand back a final line whose endedBy is
// BreakMandatory. Mandatory breaks are SEPARATORS: k of them yield k+1
// lines (D-7.1.2), so a value ending ON a break gets a trailing EMPTY line
// that the break separates from nothing, and that line — ended by no break
// at all — carries the zero value BreakOptional. The assertions below
// measure exactly that on the probe, so the claim is this suite's and not
// a comment's. The combination is therefore NOT reachable through
// packLines' own output, which is precisely why the rule must still be
// asked the question directly: a shape no packer emits today is a shape
// nothing would notice regressing.
//
// So the probe presents a REAL packer-produced mandatory-break-ended line
// at the last index, and walks all four cells of the two conditions. The
// independence the matrix row demands is what the four cells assert: each
// condition alone is sufficient (cells 2 and 3), so neither can be being
// inferred from the other, and the fourth cell — neither condition, same
// probe, same box — comes back JUSTIFIED, which is what stops the other
// three from passing vacuously.
func TestJustifiedLastLineThatIsAlsoMandatoryBreakEndedIsRaggedByEitherCondition(t *testing.T) {
	fs, cache := testShippedFontSet(), newFontCache()
	// Ends ON a mandatory break, so the trailing-empty-line rule is
	// exercised as well as the mid-value break.
	const value = "alpha beta gamma delta epsilon zeta\neta theta iota kappa lambda mu nu xi\n"
	segs, _, err := shapeSegments("e1", []string{"Noto Sans"}, nil, value, fs, cache, breaksAreConsumed)
	if err != nil {
		t.Fatal(err)
	}
	ops := text.Opportunities(text.Dictionary(), value, nil)
	const fontSize = geom.Length(11_000)
	const boxWidth = geom.Length(120_000)
	lines := packLines(segs, ops, len([]rune(value)), fontSize, boxWidth)
	if len(lines) < 3 {
		t.Fatalf("presence precondition: the probe packs into %d lines, want at least 3", len(lines))
	}

	// The measured trailing-break behaviour, asserted rather than assumed.
	last := lines[len(lines)-1]
	if last.endedBy != text.BreakOptional {
		t.Fatalf("presence precondition: the probe's final line carries endedBy %v; this test's whole premise is that a value ending ON a mandatory break gets a trailing line ended by NO break, carrying the zero value", last.endedBy)
	}
	if last.from != last.to {
		t.Fatalf("presence precondition: the probe's final line is %q, want the EMPTY line a trailing separator leaves behind (D-7.1.2)", string([]rune(value)[last.from:last.to]))
	}

	// gapsAndSlack re-derives, by the rule's own definition, what a line
	// would be justified with: the interior opportunities are the gaps,
	// and the slack is the box less the SUMMED piece widths. It exists so
	// the preconditions below can prove a nil result is attributable to
	// the ragged conditions and to nothing else.
	gapsAndSlack := func(ln wrappedLine) (int, geom.Length) {
		gaps := 0
		var summed geom.Length
		from := ln.from
		for _, op := range ops {
			if op.LineEnd <= ln.from || op.LineEnd >= ln.to {
				continue
			}
			gaps++
			summed += measureRuneRange(segs, from, op.LineEnd, fontSize)
			from = op.LineEnd
		}
		summed += measureRuneRange(segs, from, ln.to, fontSize)
		return gaps, boxWidth - summed
	}

	// A line that would otherwise justify and IS mandatory-break-ended,
	// and one that would otherwise justify and is NOT. Both must be
	// interior, so that "last line" is a property of the index this test
	// supplies rather than of the line itself.
	mandatory, optional := -1, -1
	for i := 0; i < len(lines)-1; i++ {
		gaps, slack := gapsAndSlack(lines[i])
		if gaps < 1 || slack <= 0 {
			continue
		}
		if lines[i].endedBy == text.BreakMandatory && mandatory < 0 {
			mandatory = i
		}
		if lines[i].endedBy == text.BreakOptional && optional < 0 {
			optional = i
		}
	}
	if mandatory < 0 {
		t.Fatal("presence precondition: no interior mandatory-break-ended line with gaps and positive slack, so the both-at-once cell below could pass for the wrong reason")
	}
	if optional < 0 {
		t.Fatal("presence precondition: no interior optional-ended line with gaps and positive slack, so the control cell below could pass for the wrong reason")
	}
	mandGaps, mandSlack := gapsAndSlack(lines[mandatory])
	optGaps, optSlack := gapsAndSlack(lines[optional])
	t.Logf("probe: line %d is mandatory-break-ended with %d gaps and %d slack; line %d is optional-ended with %d gaps and %d slack", mandatory, mandGaps, mandSlack, optional, optGaps, optSlack)

	// CELL 1 — BOTH CONDITIONS AT ONCE. The mandatory-break-ended line is
	// presented as the final line of the element. Ragged.
	if pieces := justifiedLinePieces(alignJustify, lines[mandatory], mandatory, mandatory+1, segs, ops, fontSize, boxWidth); pieces != nil {
		t.Errorf("a line that is BOTH the last line and mandatory-break-ended was justified into %d pieces; it must be set at the element's natural start edge", len(pieces))
	}
	// CELL 2 — MANDATORY BREAK ALONE. The same line, not last. Ragged, so
	// the mandatory-break condition is NOT inferred from the index.
	if pieces := justifiedLinePieces(alignJustify, lines[mandatory], mandatory, len(lines), segs, ops, fontSize, boxWidth); pieces != nil {
		t.Errorf("line %d is mandatory-break-ended and was justified into %d pieces when it is not the last line; the break-kind condition is being inferred from the index", mandatory, len(pieces))
	}
	// CELL 3 — LAST LINE ALONE. A line no break ended, presented as the
	// final line. Ragged, so the last-line condition is NOT inferred from
	// the break-kind field, whose zero value would answer it wrongly.
	if pieces := justifiedLinePieces(alignJustify, lines[optional], optional, optional+1, segs, ops, fontSize, boxWidth); pieces != nil {
		t.Errorf("line %d carries the zero value BreakOptional and was justified into %d pieces when it is the last line; the last-line condition is being inferred from the break-kind field instead of DERIVED FROM THE INDEX", optional, len(pieces))
	}
	// CELL 4 — NEITHER CONDITION, same probe and same box. Justified, and
	// that is what makes the three nils above mean something.
	pieces := justifiedLinePieces(alignJustify, lines[optional], optional, len(lines), segs, ops, fontSize, boxWidth)
	if pieces == nil {
		t.Fatalf("presence precondition: line %d has %d gaps and %d slack and is neither last nor mandatory-break-ended, yet came back ragged — every assertion above is vacuous", optional, optGaps, optSlack)
	}
	if len(pieces) != optGaps+1 {
		t.Fatalf("presence precondition: line %d was drawn as %d pieces, want %d for %d interior gaps", optional, len(pieces), optGaps+1, optGaps)
	}
}

// TestJustifiedLineWithExactlyOneGapTakesTheWholeSlack is the boundary
// the general rule would hide: with one gap, base is the whole slack and
// the remainder is zero.
func TestJustifiedLineWithExactlyOneGapTakesTheWholeSlack(t *testing.T) {
	fs, cache := testShippedFontSet(), newFontCache()
	const value = "alpha beta gamma"
	segs, _, err := shapeSegments("e1", []string{"Noto Sans"}, nil, value, fs, cache, breaksAreConsumed)
	if err != nil {
		t.Fatal(err)
	}
	ops := text.Opportunities(text.Dictionary(), value, nil)
	const fontSize = geom.Length(11_000)
	// A box wide enough for "alpha beta" and not for the third word.
	boxWidth := measureRuneRange(segs, 0, 11, fontSize) + 4_000
	lines := packLines(segs, ops, len([]rune(value)), fontSize, boxWidth)
	if len(lines) != 2 {
		t.Fatalf("presence precondition: packed into %d lines, want 2", len(lines))
	}
	pieces := justifiedLinePieces(alignJustify, lines[0], 0, len(lines), segs, ops, fontSize, boxWidth)
	if len(pieces) != 2 {
		t.Fatalf("line 0 was drawn as %d pieces, want 2 (one interior gap)", len(pieces))
	}
	w0 := measureRuneRange(segs, pieces[0].from, pieces[0].to, fontSize)
	w1 := measureRuneRange(segs, pieces[1].from, pieces[1].to, fontSize)
	slack := boxWidth - (w0 + w1)
	if slack <= 0 {
		t.Fatalf("presence precondition: no slack to distribute (%d)", slack)
	}
	if pieces[0].offset != 0 {
		t.Errorf("the first piece is at %d, want the element's own start edge 0", pieces[0].offset)
	}
	if got := pieces[1].offset - w0; got != slack {
		t.Errorf("the single gap received %d, want the whole slack %d", got, slack)
	}
	if got := pieces[1].offset + w1; got != boxWidth {
		t.Errorf("the last piece ends at %d, want the declared width %d exactly", got, boxWidth)
	}
}

// TestCanvasPaintMatchesTheShippingRunPathUnderJustification is the
// parity claim that matters for the designer once a line is drawn as
// several pieces: the canvas must carry the SAME number of fragments per
// line, with the same text and the same x, as the PDF producer's runs.
//
// It is the guard that catches a PDF-only justification. The browser
// never justifies anything — `text-align: justify` is contractually
// banned across every production, unit and e2e source — so if these two
// disagree the canvas is simply wrong.
func TestCanvasPaintMatchesTheShippingRunPathUnderJustification(t *testing.T) {
	tpl, err := ParseTemplate([]byte(justifyTemplateJSON))
	if err != nil {
		t.Fatal(err)
	}
	projection, err := canvasWithTextPaint(tpl, testFontSet())
	if err != nil {
		t.Fatal(err)
	}
	data := emptyBindValue(t)
	runs, err := collectTextRuns(tpl, data, data, testFontSet(), newFontCache())
	if err != nil {
		t.Fatal(err)
	}
	byElement := map[string][]textRunSource{}
	for _, run := range runs {
		byElement[run.elementID] = append(byElement[run.elementID], run)
	}
	bandY := map[string]int64{}
	for _, band := range projection.Bands {
		bandY[band.Name] = band.Y
	}
	fragments := map[string]int{}
	for _, component := range projection.Components {
		paint := component.TextPaint
		if paint == nil {
			t.Fatalf("component %s has no paint", component.ID)
		}
		wantRuns := byElement[component.ID]
		if len(wantRuns) == 0 {
			t.Fatalf("component %s has paint but no shipping runs", component.ID)
		}
		for lineIndex, line := range paint.Lines {
			var lineRuns []textRunSource
			for _, run := range wantRuns {
				if run.lineIndex == lineIndex {
					lineRuns = append(lineRuns, run)
				}
			}
			if len(lineRuns) != len(line.Fragments) {
				t.Fatalf("component %s line %d projects %d fragments, want the shipping run count %d — a justified line is one fragment PER PIECE", component.ID, lineIndex, len(line.Fragments), len(lineRuns))
			}
			bandOrigin := bandY[component.Band] - projection.MarginTop
			for i, run := range lineRuns {
				fragment := line.Fragments[i]
				if fragment.Text != run.text || fragment.X != int64(run.x) {
					t.Errorf("component %s line %d fragment %d = %#v, want the shipping run text=%q x=%d", component.ID, lineIndex, i, fragment, run.text, run.x)
				}
				if line.Top != int64(run.y)-bandOrigin {
					t.Errorf("component %s line %d top = %d, want the shipping run's %d", component.ID, lineIndex, line.Top, int64(run.y)-bandOrigin)
				}
			}
			fragments[component.ID] += len(line.Fragments)
		}
	}
	// THE VACUITY GUARD: e1 and e2 are the same string in the same box and
	// differ only in `align`, so a projection that ignored justification
	// would give them the same fragment count.
	if fragments["e1"] <= fragments["e2"] {
		t.Fatalf("the justified component projects %d fragments and the unaligned control %d — the canvas is not showing what the producer prints", fragments["e1"], fragments["e2"])
	}
	t.Logf("canvas/PDF parity under justification: e1 projects %d word-grained fragments against the control's %d", fragments["e1"], fragments["e2"])
}

// TestJustifiedPieceBoundaryStraddlingAPageSlotIsALocatedError is the
// tripwire this story owes. positionSegments returns a LOCATED ERROR,
// never a panic, when a {{page}} reservation cannot be expressed as one
// contiguous glyph range — and splitting a justified line into pieces
// WIDENS the set of inputs that can reach that path, because a piece
// boundary is now a run boundary too.
//
// REACHABILITY, stated rather than assumed: the construct is unreachable
// through the shipped set. `{{page}}` resolves to digits with no interior
// break opportunity, and a piece boundary is always AT a break
// opportunity, so no opportunity can fall strictly inside a reservation.
// The path is therefore checked here over a fabricated straddle, exactly
// as its own doc comment says such a path must be: a public entry point
// must not let an internal panic cross it.
func TestJustifiedPieceBoundaryStraddlingAPageSlotIsALocatedError(t *testing.T) {
	fs, cache := testShippedFontSet(), newFontCache()
	const value = "alpha beta"
	segs, _, err := shapeSegments("e1", []string{"Noto Sans"}, nil, value, fs, cache, breaksAreDrawn)
	if err != nil {
		t.Fatal(err)
	}
	// A reservation spanning the space — i.e. straddling exactly where a
	// justified piece boundary falls.
	slots := []pageSlotSpan{{from: 3, to: 8}}
	_, perr := positionSegments(segs, 0, 5, 0, 0, 11_000, 11_000, slots)
	if perr == nil {
		t.Fatal("a {{page}} reservation straddling a piece boundary must be a LOCATED error")
	}
	if !strings.Contains(perr.Error(), "straddles") || !strings.Contains(perr.Error(), "folio8: Render") {
		t.Errorf("the error must locate the straddle rather than merely fail: %v", perr)
	}
}
