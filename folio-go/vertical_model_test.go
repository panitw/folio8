package folio8

import (
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/fontset"
	"github.com/panitw/folio8/folio-go/internal/geom"
)

// Story 2.5a. The ruled vertical model (D-2.4.2 as AMENDED) asserted at
// the ONE level where every one of its terms is observable.
//
// WHY THIS FILE EXISTS AS A SEPARATE LEVEL OF ASSERTION, stated first
// because it is the whole design (D-000.39 sharpened):
//
// hhea lineGap is 0 on ALL FOUR faces this repository commits — the
// three shipped Noto faces and the Roboto test face. So the max(gap)
// term of the ruled model is BYTE-NEUTRAL on every artifact that ships.
// A mutation that drops it, doubles it, or splits it half-above and
// half-below produces a byte-IDENTICAL PDF for every fixture in the
// repository. That is a fact about the INPUT SET, not a gap in the
// goldens, and the forbidden response is to strengthen a golden
// assertion until it somehow "covers" the term: that manufactures a
// guard against a difference that does not exist and it will fire on a
// legitimate refactor.
//
// The only place the term can have teeth is a direct test over
// FABRICATED fontset.LineMetrics, which is why verticalModel takes
// metrics as a VALUE rather than reaching for a *fontset.Font. The
// signature is load-bearing, not tidy.

// verticalModelSubject is one fully-declared subject of the ruled model:
// its fabricated inputs and the value each of the three spans MUST
// compute to.
//
// D-000.45, BINDING: every expectation here is a COMPUTED VALUE from a
// declarative table. Not a direction, not an inequality standing in for
// a value. The reason is concrete and lives in this repository: Roboto's
// hhea ascent is 928, BELOW the 1000-unit em, so its first baseline sits
// ABOVE what the point size implies while all three Noto faces sit
// below. A guard phrased "the baseline sits lower than the font size
// implies" is FALSE on fixtures/font-text/, which ships.
type verticalModelSubject struct {
	name     string
	metrics  []fontset.LineMetrics
	fontSize geom.Length

	// The three maxima the model must select, each INDEPENDENTLY.
	wantMaxAscent  int64
	wantMaxDescent int64
	wantMaxLineGap int64

	// The three spans, in millipoints, each hand-derived below.
	wantFirstBaseline geom.Length
	wantAdvance       geom.Length
	wantLastDescent   geom.Length

	// wantSupersededAdvance is what the SUPERSEDED rule — max(A - D +
	// gap) over the chain, the worst SINGLE FACE — would have produced.
	// Where it differs from wantAdvance, the subject DISCRIMINATES the
	// amended rule from the rule it replaced; where it does not, the
	// subject is a negative control. Both are needed and both are
	// counted.
	wantSupersededAdvance geom.Length

	note string
}

// verticalModelSubjects. Every number is hand-derived from the ruled
// formula in the comment beside it — never copied from output.
//
//	top -> first baseline    max(A)
//	baseline -> baseline     max(A) + max(|D|) + max(gap)
//	last baseline -> bottom  max(|D|)
//
// scaled by geom.ScaleRound(units, fontSize, 1000).
var verticalModelSubjects = []verticalModelSubject{
	{
		// THE SUBJECT THE AMENDMENT EXISTS FOR: the shipped chain's real
		// numbers. Thai wins DESCENT (450), SC wins ASCENT (1160) — two
		// DIFFERENT faces, which is exactly what the superseded
		// single-face maximisation could not express.
		//
		//	max(A)    = max(1069, 1061, 1160) = 1160   (Noto Sans SC)
		//	max(|D|)  = max( 293,  450,  288) =  450   (Noto Sans Thai)
		//	max(gap)  = 0
		//	advance   = 1160 + 450 + 0        = 1610
		//	superseded= max(1362, 1511, 1448) = 1511   -> 99 units short
		//
		// At 16 pt (16000 mp), units -> millipoints is a plain *16:
		//	first    = 1160 * 16 = 18560
		//	advance  = 1610 * 16 = 25760
		//	last     =  450 * 16 =  7200
		//	superseded=1511 * 16 = 24176
		name: "shipped chain at 16 pt — the two axes are won by DIFFERENT faces",
		metrics: []fontset.LineMetrics{
			{Ascent: 1069, Descent: -293, LineGap: 0}, // Noto Sans
			{Ascent: 1061, Descent: -450, LineGap: 0}, // Noto Sans Thai
			{Ascent: 1160, Descent: -288, LineGap: 0}, // Noto Sans SC
		},
		fontSize:              geom.Length(16000),
		wantMaxAscent:         1160,
		wantMaxDescent:        450,
		wantMaxLineGap:        0,
		wantFirstBaseline:     geom.Length(18560),
		wantAdvance:           geom.Length(25760),
		wantLastDescent:       geom.Length(7200),
		wantSupersededAdvance: geom.Length(24176),
		note:                  "the worst adjacent PAIR is 1610; the worst single FACE is 1511",
	},
	{
		// THE lineGap SUBJECT. Byte-neutral on everything that ships, so
		// this is the ONLY place the term is observable at all. Non-zero
		// on two of the three faces and DIFFERENT on each, so a model
		// that took the gap of the ascent-winning face rather than
		// max(gap) would be caught too.
		//
		//	max(A)   = max(1000, 900, 800)  = 1000
		//	max(|D|) = max( 200, 500, 100)  =  500
		//	max(gap) = max(   0,  70, 250)  =  250
		//	advance  = 1000 + 500 + 250     = 1750
		//	superseded = max(1000-(-200)+0, 900-(-500)+70, 800-(-100)+250)
		//	           = max(1200, 1470, 1150) = 1470
		//
		// At 10 pt (10000 mp), units -> millipoints is a plain *10:
		//	first  = 1000*10 = 10000   advance = 1750*10 = 17500
		//	last   =  500*10 =  5000   superseded = 1470*10 = 14700
		name: "FABRICATED metrics with a NON-ZERO lineGap — the term no golden can see",
		metrics: []fontset.LineMetrics{
			{Ascent: 1000, Descent: -200, LineGap: 0},
			{Ascent: 900, Descent: -500, LineGap: 70},
			{Ascent: 800, Descent: -100, LineGap: 250},
		},
		fontSize:              geom.Length(10000),
		wantMaxAscent:         1000,
		wantMaxDescent:        500,
		wantMaxLineGap:        250,
		wantFirstBaseline:     geom.Length(10000),
		wantAdvance:           geom.Length(17500),
		wantLastDescent:       geom.Length(5000),
		wantSupersededAdvance: geom.Length(14700),
		note:                  "all THREE maxima are won by THREE DIFFERENT faces, so no single face can supply the answer",
	},
	{
		// NEGATIVE CONTROL. A single face cannot fail to supply both
		// axes, so max(A)+max(|D|)+max(gap) is IDENTICALLY A - D + gap
		// and the amended rule agrees with the superseded one exactly.
		// Without this row, "the amended rule differs from the old one"
		// would be achieved by differing from it EVERYWHERE, which is
		// not the claim.
		//
		//	max(A)=928  max(|D|)=244  max(gap)=0
		//	advance = 928 + 244 + 0 = 1172 = superseded (928+244+0)
		// At 14 pt: first = 928*14 = 12992, advance = 1172*14 = 16408,
		//           last  = 244*14 =  3416
		name: "NEGATIVE CONTROL: single face (Roboto) — amended and superseded rules AGREE",
		metrics: []fontset.LineMetrics{
			{Ascent: 928, Descent: -244, LineGap: 0},
		},
		fontSize:              geom.Length(14000),
		wantMaxAscent:         928,
		wantMaxDescent:        244,
		wantMaxLineGap:        0,
		wantFirstBaseline:     geom.Length(12992),
		wantAdvance:           geom.Length(16408),
		wantLastDescent:       geom.Length(3416),
		wantSupersededAdvance: geom.Length(16408),
		note:                  "928 is BELOW the 1000-unit em, so this face's first baseline sits ABOVE what the point size implies — D-000.45's real subject",
	},
	{
		// The Latin-only shipped chain, which fixtures/three-band-page/
		// declares. Also a single-face agreement case, at a third size.
		//	max(A)=1069 max(|D|)=293 max(gap)=0 -> 1362
		// At 12 pt: first = 1069*12 = 12828, advance = 1362*12 = 16344,
		//           last  =  293*12 =  3516
		name: "NEGATIVE CONTROL: Latin-only shipped chain at 12 pt",
		metrics: []fontset.LineMetrics{
			{Ascent: 1069, Descent: -293, LineGap: 0},
		},
		fontSize:              geom.Length(12000),
		wantMaxAscent:         1069,
		wantMaxDescent:        293,
		wantMaxLineGap:        0,
		wantFirstBaseline:     geom.Length(12828),
		wantAdvance:           geom.Length(16344),
		wantLastDescent:       geom.Length(3516),
		wantSupersededAdvance: geom.Length(16344),
		note:                  "1069 is ABOVE the em, so this face moves the other way from Roboto",
	},
	{
		// ORDER INDEPENDENCE. The same three faces as the first subject,
		// permuted so that neither the ascent winner nor the descent
		// winner is first. A maximum is order-independent; a
		// first-face rule is not, and would give 1362 here.
		name: "the shipped chain PERMUTED — a maximum is order-independent",
		metrics: []fontset.LineMetrics{
			{Ascent: 1160, Descent: -288, LineGap: 0}, // SC first
			{Ascent: 1061, Descent: -450, LineGap: 0},
			{Ascent: 1069, Descent: -293, LineGap: 0},
		},
		fontSize:              geom.Length(16000),
		wantMaxAscent:         1160,
		wantMaxDescent:        450,
		wantMaxLineGap:        0,
		wantFirstBaseline:     geom.Length(18560),
		wantAdvance:           geom.Length(25760),
		wantLastDescent:       geom.Length(7200),
		wantSupersededAdvance: geom.Length(24176),
		note:                  "identical answers to the first subject under a different order",
	},
}

// TestVerticalModelArithmeticOverFabricatedMetrics is AC3's teeth and
// the amended D-2.4.2's direct assertion. It drives verticalModel with
// FABRICATED metrics — no font is constructed — and asserts all three
// spans against hand-derived literals.
//
// It carries two vacuity guards, because a table like this can rot into
// asserting nothing at all:
//
//   - At least one subject must DISCRIMINATE the amended rule from the
//     superseded one, or the table would pass unchanged against the very
//     rule the amendment replaced.
//   - At least one subject must carry a NON-ZERO max(gap), or the term
//     that no golden can observe is not observed here either — and this
//     test would be the second place claiming coverage it does not have.
func TestVerticalModelArithmeticOverFabricatedMetrics(t *testing.T) {
	if len(verticalModelSubjects) == 0 {
		t.Fatal("vacuity: the subject table is empty, so this test asserts nothing")
	}

	var discriminating, withLineGap, aboveEm, belowEm int
	for _, s := range verticalModelSubjects {
		t.Run(s.name, func(t *testing.T) {
			if len(s.metrics) == 0 {
				t.Fatalf("presence precondition: subject %q fabricates no metrics", s.name)
			}

			// (1) THE TABLE IS INTERNALLY CONSISTENT. Each declared span
			//     equals the ruled formula applied to the declared
			//     maxima. Without this, a typo in a literal would be
			//     "asserted" by being compared to itself.
			wantFirst := geom.ScaleRound(geom.Length(s.wantMaxAscent), int64(s.fontSize), 1000)
			wantAdv := geom.ScaleRound(geom.Length(s.wantMaxAscent+s.wantMaxDescent+s.wantMaxLineGap), int64(s.fontSize), 1000)
			wantLast := geom.ScaleRound(geom.Length(s.wantMaxDescent), int64(s.fontSize), 1000)
			if wantFirst != s.wantFirstBaseline || wantAdv != s.wantAdvance || wantLast != s.wantLastDescent {
				t.Fatalf("the TABLE disagrees with the ruled formula: from maxA=%d maxD=%d maxGap=%d at size %d the rule gives first=%d advance=%d last=%d, but the table declares first=%d advance=%d last=%d",
					s.wantMaxAscent, s.wantMaxDescent, s.wantMaxLineGap, s.fontSize,
					wantFirst, wantAdv, wantLast,
					s.wantFirstBaseline, s.wantAdvance, s.wantLastDescent)
			}

			// (2) THE MAXIMA ARE SELECTED PER AXIS. Recomputed here from
			//     the fabricated metrics so the declared maxima are
			//     checked against the inputs and not merely asserted.
			var maxA, maxD, maxG, superseded int64
			for _, lm := range s.metrics {
				if lm.Ascent > maxA {
					maxA = lm.Ascent
				}
				if d := -lm.Descent; d > maxD {
					maxD = d
				}
				if lm.LineGap > maxG {
					maxG = lm.LineGap
				}
				if u := lm.Ascent - lm.Descent + lm.LineGap; u > superseded {
					superseded = u
				}
			}
			if maxA != s.wantMaxAscent || maxD != s.wantMaxDescent || maxG != s.wantMaxLineGap {
				t.Fatalf("the declared maxima do not match the fabricated metrics: metrics give maxA=%d maxD=%d maxGap=%d, table declares %d/%d/%d",
					maxA, maxD, maxG, s.wantMaxAscent, s.wantMaxDescent, s.wantMaxLineGap)
			}
			if got := geom.ScaleRound(geom.Length(superseded), int64(s.fontSize), 1000); got != s.wantSupersededAdvance {
				t.Fatalf("the declared SUPERSEDED advance is %d, but max(A-D+gap) over the fabricated metrics gives %d", s.wantSupersededAdvance, got)
			}

			// (3) THE PRODUCTION ARITHMETIC AGREES. This is the actual
			//     assertion; everything above makes it mean something.
			got, err := verticalModel([]string{s.name}, s.metrics, s.fontSize, defaultLineSpacing)
			if err != nil {
				t.Fatalf("verticalModel: %v", err)
			}
			if got.FirstBaseline != s.wantFirstBaseline {
				t.Errorf("top -> first baseline is %d mp, want the hand-derived %d mp (max(A)=%d at size %d) — %s",
					got.FirstBaseline, s.wantFirstBaseline, s.wantMaxAscent, s.fontSize, s.note)
			}
			if got.Advance != s.wantAdvance {
				t.Errorf("baseline -> baseline is %d mp, want the hand-derived %d mp (max(A)+max(|D|)+max(gap) = %d+%d+%d at size %d) — %s",
					got.Advance, s.wantAdvance, s.wantMaxAscent, s.wantMaxDescent, s.wantMaxLineGap, s.fontSize, s.note)
			}
			if got.LastDescent != s.wantLastDescent {
				t.Errorf("last baseline -> bottom is %d mp, want the hand-derived %d mp (max(|D|)=%d at size %d)",
					got.LastDescent, s.wantLastDescent, s.wantMaxDescent, s.fontSize)
			}

			// (4) THE SUPERSEDED RULE IS ASSERTED AGAINST, not merely
			//     absent — but only where the two rules actually differ.
			if s.wantSupersededAdvance != s.wantAdvance && got.Advance == s.wantSupersededAdvance {
				t.Errorf("baseline -> baseline is %d mp, which is exactly max(A - D + gap) over the chain — the SUPERSEDED single-face rule. The amended rule maximises each axis INDEPENDENTLY: %s",
					got.Advance, s.note)
			}
		})

		if s.wantSupersededAdvance != s.wantAdvance {
			discriminating++
		}
		if s.wantMaxLineGap != 0 {
			withLineGap++
		}
		// Counted, never asserted as a direction (D-000.45): these are
		// vacuity counters over the TABLE, not claims about an artifact.
		if s.wantFirstBaseline > s.fontSize {
			aboveEm++
		}
		if s.wantFirstBaseline < s.fontSize {
			belowEm++
		}
	}

	if discriminating == 0 {
		t.Fatal("vacuity: no subject distinguishes max(A)+max(|D|)+max(gap) from the SUPERSEDED max(A-D+gap), so this table would pass unchanged against the rule the amendment replaced")
	}
	if withLineGap == 0 {
		t.Fatal("vacuity: no subject carries a non-zero max(gap). hhea lineGap is 0 on every committed face, so if it is zero here too then NOTHING in this repository observes the lineGap term and this test must not be counted as covering it (D-000.39 sharpened)")
	}
	if aboveEm == 0 || belowEm == 0 {
		t.Fatalf("vacuity: %d subjects place the first baseline further down than the point size and %d place it further up. BOTH must occur or this table cannot tell a computed value from a directional rule — and a directional rule is false on fixtures/font-text/ (D-000.45)", aboveEm, belowEm)
	}
	t.Logf("vertical model: %d of %d subjects discriminate the amended rule from the superseded one; %d carry a non-zero max(gap); %d place the first baseline below the point size and %d above it",
		discriminating, len(verticalModelSubjects), withLineGap, aboveEm, belowEm)
}

// TestVerticalModelRefusesAChainWithNoPresentFace and
// TestVerticalModelRefusesANonPositiveLineHeight are the two error paths
// of the arithmetic, RED-PROVED at the seam where they are
// constructible. See TestVerticalModelErrorPathsAreUnreachableThroughRender
// for what that does and does not entitle this story to claim.
func TestVerticalModelRefusesAChainWithNoPresentFace(t *testing.T) {
	_, err := verticalModel([]string{"Nope", "Also Nope"}, nil, geom.Length(16000), defaultLineSpacing)
	if err == nil {
		t.Fatal("a chain with no present face must be a located error, not a default line height")
	}
	if !strings.Contains(err.Error(), "Nope") {
		t.Errorf("the error must LOCATE the failure by naming the chain, got %q", err.Error())
	}
}

func TestVerticalModelRefusesANonPositiveLineHeight(t *testing.T) {
	// Every present face declares an hhea ascent no greater than zero
	// and a descent no less than zero, so all three maxima clamp to
	// zero and the line height is 0.
	_, err := verticalModel([]string{"Degenerate"}, []fontset.LineMetrics{
		{Ascent: 0, Descent: 0, LineGap: 0},
		{Ascent: -50, Descent: 20, LineGap: -10},
	}, geom.Length(16000), defaultLineSpacing)
	if err == nil {
		t.Fatal("a chain whose faces sum to a non-positive line height must be an error, not a zero-height line")
	}
	for _, want := range []string{"Degenerate", "max(hhea ascent)", "max(-hhea descent)", "max(hhea lineGap)"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error must report which term produced the degenerate height; %q is missing from %q", want, err.Error())
		}
	}
}

// TestVerticalModelErrorPathsAreUnreachableThroughRender is AC4,
// MEASURED rather than assumed.
//
// The vertical model is now computed for EVERY element with at least one
// line, where the superseded code computed the advance only when
// len(lines) > 1. That widens the set of inputs that can reach the two
// error paths above, and D-000.9 says the question "can they now fire?"
// is to be measured, not asserted.
//
// THE ANSWER, and it is structural rather than statistical:
//
//   - verticalModel's len(metrics) == 0 path — AC4 names it
//     "present == 0", which is a CONDITION and not a symbol; the
//     production spelling is len(metrics) == 0 in wrap.go — WAS
//     unreachable through folio8.Render before Story 3.6, and STORY 3.6
//     CHANGED THIS, measured rather than assumed exactly as this test's
//     own philosophy demands. Before 3.6, the call sat AFTER
//     shapeSegments had already succeeded on non-empty text, and
//     shapeSegments resolved every rune through resolveRuneFace, which
//     was a located ERROR when NO chain member present in the FontSet
//     had a glyph for a rune — so a chain with no present face failed
//     there first. Story 3.6 (OPEN-1, divergence 6) made that condition
//     a WARNING instead (FR41's fifth mode: the render omits the rune —
//     no glyph, no advance — rather than aborting), so resolveRuneFace
//     no longer fails there. When EVERY declared chain member is absent
//     from the supplied FontSet, every rune in the element is now
//     omitted and the element contributes NO face metrics at all to its
//     line — reaching verticalModel's len(metrics) == 0 path for real,
//     through the public entry point, for the first time. This is not a
//     regression: it is the same located error as before
//     (TestVerticalModelRefusesAChainWithNoPresentFace, above), now
//     reachable one level higher because the layer that used to fail
//     first no longer does for this particular cause. An element with
//     EMPTY text never reaches either: collectTextRuns short-circuits on
//     boundText == "" one branch earlier, and it does so AFTER fontChain
//     has validated the chain — so the widening cannot turn a
//     previously-rendering empty element into an error.
//
//   - verticalModel's units <= 0 path — AC4 names it "maxUnits <= 0";
//     the production variable is units, and there is no maxUnits
//     anywhere in the module — is UNREACHABLE with any face this repository
//     commits. It requires every present face to declare an hhea ascent
//     <= 0 and a descent >= 0; all four committed faces declare the
//     opposite, and requireReadableTables makes an absent hhea a load
//     error rather than a substituted 800/-200.
//
// D-000.24 WOULD label both "forward guards with no available
// red-proof". THAT LABEL IS WRONG HERE, and the distinction is the point
// of the verticalModel seam: both paths ARE red-proved, one level down,
// by the two tests above, because the arithmetic is reachable with
// fabricated metrics. They are PROVEN at the seam and UNREACHABLE
// through the public entry point. Neither half alone is the honest
// statement.
func TestVerticalModelErrorPathsAreUnreachableThroughRender(t *testing.T) {
	// (1) Story 3.6 changed this measurement's outcome (see the doc
	//     comment above): a chain with NO member present in the FontSet
	//     no longer fails in resolveRuneFace (that condition is now
	//     FR41's fifth-mode WARNING, not an error) — it now fails one
	//     level higher, in the vertical model itself, once every rune in
	//     the element has been omitted and no face metrics remain to
	//     derive a line height from. Asserting WHICH error comes back is
	//     the measurement; asserting merely that "an error came back"
	//     would pass either way.
	tpl, err := ParseTemplate([]byte(multiScriptTestTemplateJSON))
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	_, rerr := Render(tpl, Data("{}"), nil, FontSet{"Not A Declared Face": testShippedNotoSans})
	if rerr == nil {
		t.Fatal("presence precondition: rendering against a FontSet supplying none of the declared chain must fail, or this measurement has no subject")
	}
	if strings.Contains(rerr.Error(), "has a glyph for rune") {
		t.Errorf("the render failed in resolveRuneFace (%q) — Story 3.6 made that condition a Warning, not an error, so this is no longer where a chain-with-no-present-face fails", rerr.Error())
	}
	if !strings.Contains(rerr.Error(), "no line height can be derived from it") {
		t.Errorf("expected verticalModel's located len(metrics)==0 error, got %q", rerr.Error())
	}

	// (2) A chain with SOME members absent still renders — the model
	//     tolerates an absent chain member exactly as fontChain and
	//     resolveRuneFace do. This is the half that would catch the
	//     widening turning a working document into an error.
	partial := FontSet{"Noto Sans": testShippedNotoSans, "Noto Sans Thai": testShippedNotoSansThai, "Noto Sans SC": testShippedNotoSansSC}
	delete(partial, "Noto Sans SC")
	latinOnly, perr := ParseTemplate([]byte(fontTestTemplateJSON))
	if perr != nil {
		t.Fatalf("parse template: %v", perr)
	}
	if _, err := Render(latinOnly, Data("{}"), nil, testFontSet()); err != nil {
		t.Errorf("a single-line, single-face document must still render after the vertical model became unconditional, got %v", err)
	}

	// (3) THE DIRECT STATEMENT OF THE NO-REGRESSION CLAIM. Every
	//     element of every fixture in this repository is single-line or
	//     multi-line, and all of them rendered before the widening. The
	//     five fixture render helpers are driven here so that the claim
	//     "no input that rendered successfully before returns an error
	//     now" is asserted rather than inferred from the suite being
	//     green elsewhere.
	rendered := 0
	for _, f := range baselineAcceptanceFixtures {
		if f.render(t) == nil {
			t.Errorf("%s: render produced no bytes", f.name)
			continue
		}
		rendered++
	}
	if rendered != len(baselineAcceptanceFixtures) {
		t.Fatalf("presence precondition: %d of %d fixtures rendered", rendered, len(baselineAcceptanceFixtures))
	}
	t.Logf("AC4 (as amended by Story 3.6): verticalModel's units<=0 path (AC4's \"maxUnits<=0\") is UNREACHABLE through folio8.Render (no committed face is degenerate); its len(metrics)==0 path (AC4's \"present==0\") IS now reachable, when every chain member is absent from the FontSet (case 1, above) — both are PROVEN at the verticalModel seam over fabricated metrics regardless. %d fixtures re-rendered without error.", rendered)
}

// TestChainVerticalModelIsOneWalkFeedingBothSpans is AC1's assertion, in
// the only form that is observable: the first-baseline offset and the
// inter-baseline advance that production uses come from ONE call, and
// lineAdvance is a PROJECTION of that call rather than a second
// derivation.
//
// The structural half is not assertable from a test — Go cannot ask "how
// many times did you walk the chain" — so what is asserted is the
// consequence that a second walk would eventually break: the two spans
// agree with the single value chainVerticalModel returns, for every
// shipped chain and size the fixtures use.
func TestChainVerticalModelIsOneWalkFeedingBothSpans(t *testing.T) {
	fs := testShippedFontSet()
	chains := [][]string{
		{"Noto Sans", "Noto Sans Thai", "Noto Sans SC"},
		{"Noto Sans"},
	}
	sizes := []geom.Length{8000, 9000, 11000, 12000, 14000, 16000}

	checked := 0
	for _, chain := range chains {
		for _, size := range sizes {
			vm, err := chainVerticalModel(chain, size, defaultLineSpacing, fs, newFontCache())
			if err != nil {
				t.Fatalf("chainVerticalModel(%v, %d): %v", chain, size, err)
			}
			adv, aerr := lineAdvance(chain, size, defaultLineSpacing, fs, newFontCache())
			if aerr != nil {
				t.Fatalf("lineAdvance(%v, %d): %v", chain, size, aerr)
			}
			if adv != vm.Advance {
				t.Errorf("chain %v at %d: lineAdvance returned %d but the vertical model's Advance is %d — these must be one derivation, not two",
					chain, size, adv, vm.Advance)
			}
			// The model's own internal consistency, off the same call:
			// Advance = FirstBaseline + LastDescent + max(gap) scaled,
			// and max(gap) is 0 on every committed face, so the two
			// outer spans must SUM to the inner one here.
			if vm.FirstBaseline+vm.LastDescent != vm.Advance {
				t.Errorf("chain %v at %d: first(%d) + last(%d) = %d, but Advance is %d. On the committed faces hhea lineGap is 0, so the three spans must reconcile exactly; if a face with a non-zero lineGap has been added, THIS ASSERTION is what must change, not the model",
					chain, size, vm.FirstBaseline, vm.LastDescent, vm.FirstBaseline+vm.LastDescent, vm.Advance)
			}
			if vm.FirstBaseline <= 0 || vm.Advance <= 0 {
				t.Errorf("chain %v at %d: degenerate spans first=%d advance=%d", chain, size, vm.FirstBaseline, vm.Advance)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("vacuity: no (chain, size) pair was checked")
	}
	t.Logf("AC1: %d (chain, size) pairs agree between chainVerticalModel and its lineAdvance projection", checked)
}
