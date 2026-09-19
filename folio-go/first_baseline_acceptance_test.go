package folio8

import (
	"fmt"
	"sort"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// Story 2.5a, AC6 — D-000.22's semantic acceptance step for EVERY
// fixture this story re-records, performed at the recording moment.
//
// D-000.44, BINDING: a RE-RECORDING IS A RECORDING. Measured at
// 17f5f7a, of the five goldens this story moves, exactly ONE
// (three-band-page) had a semantic check that could see the defect;
// three had no semantic acceptance test at all, and the fourth
// (wrapped-text) had one that called lineAdvance, compared it to a
// literal, then computed the observed baselines AND DISCARDED THEM. So
// D-000.22 stood recorded as discharged for fixtures that never had the
// step. This file is where it is actually paid.
//
// WHAT EVERY ASSERTION HERE READS. The baselines are recovered from the
// PRODUCED PDF's own content stream — the `ty` operand of each BT block's
// `Tm` operator, via readEmittedRuns — never from the renderer's
// intermediate state and never from the inputs (D-000.21 sharpened:
// assert on the artifact that carries the property, and prove it carries
// it).
//
// WHAT EVERY EXPECTATION IS. A COMPUTED VALUE, hand-derived below from
// the ruled formula, in the style tbExpectedTm already uses. Never a
// direction. D-000.45 is binding and its subject is in this very table:
// Roboto's hhea ascent is 928, BELOW the 1000-unit em, so font-text's
// baselines move UP while every Noto-chain baseline moves DOWN. A guard
// phrased "the baseline sits lower than the font size implies" is FALSE
// on a fixture that ships.

// baselineElement is one text element of a fixture: what the ruled model
// must compute for it, and where its baselines must land in the produced
// artifact.
type baselineElement struct {
	id         string
	band       string
	fontSizeMP int64

	// maxAscent1000 is max(hhea ascent) over the element's DECLARED
	// chain, in units of the 1000-em — the input to the model's first
	// span. Declared per element so the table states its own premises.
	maxAscent1000 int64

	// wantFirstBaselineMP is max(A) scaled to fontSizeMP: the distance
	// from the element's top to its FIRST baseline.
	wantFirstBaselineMP int64

	// wantAdvanceMP is max(A)+max(|D|)+max(gap) scaled. Zero for a
	// single-line element, where no inter-baseline distance is
	// observable and asserting one would be vacuous (D-000.33).
	wantAdvanceMP int64

	// wantBaselineY is every baseline this element emits, in
	// millipoints of PDF user space (bottom-up), top line first.
	wantBaselineY []int64
}

// baselineFixture is one re-recorded golden and everything the
// acceptance step needs to judge it.
type baselineFixture struct {
	name   string
	render func(*testing.T) []byte
	chain  []string

	// elements in the order their baselines appear DOWN the page, which
	// is descending PDF y.
	elements []baselineElement
}

// baselineAcceptanceFixtures. Page geometry, stated once because every
// derivation below uses it:
//
//	A4 is 595276 x 841890 millipoints.
//	internal/pdf places a baseline at
//	    pdfY = flipY(pageHeight, marginTop, run.Y, run.BaselineOffset)
//	         = pageHeight - marginTop - run.Y - run.BaselineOffset
//	run.Y is layout.PlaceInBand(bandOrigin, element.y) — a translation.
//	Band origins: pageHeader at 0, content directly below it, and
//	pageFooter at (printableHeight - footerHeight), where
//	printableHeight = pageHeight - marginTop - marginBottom.
//
// max(hhea ascent) per chain, from the committed faces:
//
//	["Roboto-Regular"]                        ->  928   (BELOW the em)
//	["Noto Sans"]                             -> 1069
//	["Noto Sans","Noto Sans Thai","Noto Sans SC"] -> 1160  (Noto Sans SC)
//
// max(|hhea descent|) per chain: 244 / 293 / 450 (Noto Sans Thai).
// hhea lineGap is 0 on all four faces, so max(gap) contributes nothing
// to any number below — and nothing here may be strengthened to try to
// observe it (D-000.39 sharpened; the teeth are in
// TestVerticalModelArithmeticOverFabricatedMetrics).
var baselineAcceptanceFixtures = []baselineFixture{
	{
		// fixtures/font-text/ — THE SIGN DISCRIMINATOR. The only fixture
		// not using the shipped Noto chain, and the only one whose
		// baselines move UP. Roboto's max(A) is 928 < 1000, so its first
		// baseline sits ABOVE what the point size implies, in the
		// opposite direction from every other fixture here.
		//
		// margins 36 all round; pageHeader height 20, pageFooter 20.
		//	printable = 841890 - 36000 - 36000 = 769890
		//	content origin  = 20000
		//	pageFooter origin = 769890 - 20000 = 749890
		//
		//	e1  content     y=0  size 14  first = 928*14 = 12992
		//	    run.Y = 20000 + 0 = 20000
		//	    pdfY  = 841890 - 36000 - 20000 - 12992 = 772898
		//	e2  pageFooter  y=0  size  9  first = 928* 9 =  8352
		//	    run.Y = 749890 + 0 = 749890
		//	    pdfY  = 841890 - 36000 - 749890 - 8352 =  47648
		//
		// The superseded model put these at 841890-36000-20000-14000 =
		// 771890 and 841890-36000-749890-9000 = 46000 — i.e. it placed
		// them 1008 and 648 millipoints LOWER in PDF space, which is
		// HIGHER on the page.
		name:   "font-text",
		chain:  []string{"Roboto-Regular"},
		render: func(t *testing.T) []byte { return renderFontTextForAcceptance(t) },
		elements: []baselineElement{
			{id: "e1", band: "content", fontSizeMP: 14000, maxAscent1000: 928, wantFirstBaselineMP: 12992, wantBaselineY: []int64{772898}},
			{id: "e2", band: "pageFooter", fontSizeMP: 9000, maxAscent1000: 928, wantFirstBaselineMP: 8352, wantBaselineY: []int64{47648}},
		},
	},
	{
		// fixtures/multi-script-fallback/ — one element, three FACE
		// SEGMENTS (Latin, Thai, CJK) sharing ONE baseline. That is the
		// fixture that would expose a per-run baseline: if the offset
		// were derived from each run's own resolved face rather than
		// from the declared chain, these three runs would land on THREE
		// different baselines instead of one.
		//
		// margins 36; pageHeader height 20 -> content origin 20000.
		//	e1  content  y=0  size 14  first = 1160*14 = 16240
		//	    run.Y = 20000
		//	    pdfY  = 841890 - 36000 - 20000 - 16240 = 769650
		name:   "multi-script-fallback",
		chain:  []string{"Noto Sans", "Noto Sans Thai", "Noto Sans SC"},
		render: func(t *testing.T) []byte { return renderMultiScriptForAcceptance(t) },
		elements: []baselineElement{
			{id: "e1", band: "content", fontSizeMP: 14000, maxAscent1000: 1160, wantFirstBaselineMP: 16240, wantBaselineY: []int64{769650}},
		},
	},
	{
		// fixtures/shaped-text/ — THE SIGN-OFF-BINDING FIXTURE. Seven
		// single-line elements at 16 pt, 28 pt apart.
		//
		// margins 36; pageHeader height 20 -> content origin 20000.
		//	first = 1160*16 = 18560 for every element.
		//	pdfY  = 841890 - 36000 - (20000 + y) - 18560
		//	      = 767330 - y
		//	e1 y=0       -> 767330      e5 y=112000 -> 655330
		//	e2 y= 28000  -> 739330      e6 y=140000 -> 627330
		//	e3 y= 56000  -> 711330      e7 y=168000 -> 599330
		//	e4 y= 84000  -> 683330
		name:   "shaped-text",
		chain:  []string{"Noto Sans", "Noto Sans Thai", "Noto Sans SC"},
		render: func(t *testing.T) []byte { return renderShapedTextFixture(t) },
		elements: []baselineElement{
			{id: "e1", band: "content", fontSizeMP: 16000, maxAscent1000: 1160, wantFirstBaselineMP: 18560, wantBaselineY: []int64{767330}},
			{id: "e2", band: "content", fontSizeMP: 16000, maxAscent1000: 1160, wantFirstBaselineMP: 18560, wantBaselineY: []int64{739330}},
			{id: "e3", band: "content", fontSizeMP: 16000, maxAscent1000: 1160, wantFirstBaselineMP: 18560, wantBaselineY: []int64{711330}},
			{id: "e4", band: "content", fontSizeMP: 16000, maxAscent1000: 1160, wantFirstBaselineMP: 18560, wantBaselineY: []int64{683330}},
			{id: "e5", band: "content", fontSizeMP: 16000, maxAscent1000: 1160, wantFirstBaselineMP: 18560, wantBaselineY: []int64{655330}},
			{id: "e6", band: "content", fontSizeMP: 16000, maxAscent1000: 1160, wantFirstBaselineMP: 18560, wantBaselineY: []int64{627330}},
			{id: "e7", band: "content", fontSizeMP: 16000, maxAscent1000: 1160, wantFirstBaselineMP: 18560, wantBaselineY: []int64{599330}},
		},
	},
	{
		// fixtures/three-band-page/ — three DISTINCT band origins and
		// three different point sizes, so it is the fixture that proves
		// the offset scales with the size rather than being a constant.
		//
		// margins top 30, bottom 42; pageHeader height 18, footer 24.
		//	printable = 841890 - 30000 - 42000 = 769890
		//	pageHeader origin = 0
		//	content    origin = 18000
		//	pageFooter origin = 769890 - 24000 = 745890
		//
		//	e4 pageHeader y=4    size 9  first = 1069* 9 =  9621
		//	   run.Y = 0 + 4000 = 4000
		//	   pdfY = 841890 - 30000 -   4000 -  9621 = 798269
		//	e1 content    y=0    size 12 first = 1069*12 = 12828
		//	   run.Y = 18000
		//	   pdfY = 841890 - 30000 -  18000 - 12828 = 781062
		//	e2 content    y=120  size 12 first = 12828
		//	   run.Y = 18000 + 120000 = 138000
		//	   pdfY = 841890 - 30000 - 138000 - 12828 = 661062
		//	e3 pageFooter y=6    size 8  first = 1069* 8 =  8552
		//	   run.Y = 745890 + 6000 = 751890
		//	   pdfY = 841890 - 30000 - 751890 -  8552 =  51448
		//
		// THREE DISTINCT OFFSETS (9621 / 12828 / 8552) for the three
		// sizes, which is what keeps the "four distinct placements"
		// assertion in TestThreeBandPageSemanticAcceptance meaningful
		// after this story (AC14 / D-000.34).
		name:   "three-band-page",
		chain:  []string{"Noto Sans"},
		render: func(t *testing.T) []byte { return renderThreeBandPage(t) },
		elements: []baselineElement{
			{id: "e4", band: "pageHeader", fontSizeMP: 9000, maxAscent1000: 1069, wantFirstBaselineMP: 9621, wantBaselineY: []int64{798269}},
			{id: "e1", band: "content", fontSizeMP: 12000, maxAscent1000: 1069, wantFirstBaselineMP: 12828, wantBaselineY: []int64{781062}},
			{id: "e2", band: "content", fontSizeMP: 12000, maxAscent1000: 1069, wantFirstBaselineMP: 12828, wantBaselineY: []int64{661062}},
			{id: "e3", band: "pageFooter", fontSizeMP: 8000, maxAscent1000: 1069, wantFirstBaselineMP: 8552, wantBaselineY: []int64{51448}},
		},
	},
	{
		// fixtures/wrapped-text/ — THE ONLY FIXTURE WITH MULTI-LINE
		// ELEMENTS, and therefore the ONLY artifact in this repository
		// where the AMENDED advance is observable at all.
		//
		// That is a consequence of the amendment's arithmetic, not an
		// accident of this fixture: for a chain resolving to ONE present
		// face, max(A)+max(|D|)+max(gap) is IDENTICALLY A - D + gap, so
		// only the Noto x3 chain's advance moves (1511 -> 1610 units) —
		// and of the three fixtures declaring that chain, only this one
		// has an element occupying more than one line.
		//
		// margins 36; pageHeader height 20 -> content origin 20000.
		//	first   = 1160*11 = 12760
		//	advance = 1610*11 = 17710      (superseded: 1511*11 = 16621)
		//	pdfY of an element's FIRST baseline
		//	        = 841890 - 36000 - (20000 + y) - 12760 = 773130 - y
		//	each subsequent line is 17710 LOWER in PDF y.
		//
		//	e1 y=0      3 lines: 773130, 755420, 737710
		//	e2 y= 80000 2 lines: 693130, 675420
		//	e3 y=160000 2 lines: 613130, 595420
		//	e4 y=240000 2 lines: 533130, 515420
		name:   "wrapped-text",
		chain:  []string{"Noto Sans", "Noto Sans Thai", "Noto Sans SC"},
		render: func(t *testing.T) []byte { return renderWrappedText(t) },
		elements: []baselineElement{
			{id: "e1", band: "content", fontSizeMP: 11000, maxAscent1000: 1160, wantFirstBaselineMP: 12760, wantAdvanceMP: 17710, wantBaselineY: []int64{773130, 755420, 737710}},
			{id: "e2", band: "content", fontSizeMP: 11000, maxAscent1000: 1160, wantFirstBaselineMP: 12760, wantAdvanceMP: 17710, wantBaselineY: []int64{693130, 675420}},
			{id: "e3", band: "content", fontSizeMP: 11000, maxAscent1000: 1160, wantFirstBaselineMP: 12760, wantAdvanceMP: 17710, wantBaselineY: []int64{613130, 595420}},
			{id: "e4", band: "content", fontSizeMP: 11000, maxAscent1000: 1160, wantFirstBaselineMP: 12760, wantAdvanceMP: 17710, wantBaselineY: []int64{533130, 515420}},
		},
	},
}

// TestFirstBaselineSemanticAcceptanceAcrossEveryReRecordedGolden is
// AC6's machine-checkable half, for all FIVE fixtures at once.
//
// It is deliberately ONE test over a declared table rather than five
// hand-written ones: the property is identical in all five cases and
// only the subject changes, so a table is the form in which a missing
// fixture is a visible hole rather than a test nobody wrote (the state
// measured at 17f5f7a, where three of the five had no step at all).
func TestFirstBaselineSemanticAcceptanceAcrossEveryReRecordedGolden(t *testing.T) {
	if len(baselineAcceptanceFixtures) != 5 {
		t.Fatalf("this story re-records exactly five goldens; the acceptance table declares %d", len(baselineAcceptanceFixtures))
	}

	var multiLineFixtures, aboveEm, belowEm int
	// The NARRATION is derived from the same walk as the guard, never
	// from what the author expected (D-000.48's companion finding: this
	// message once stated both directions backwards while the guard
	// below was correct, and a reader believes the message). Which
	// fixtures supply which direction is COLLECTED here rather than
	// named in a literal, so the sentence cannot disagree with the
	// counts it is printed beside.
	aboveEmBy := map[string]int{}
	belowEmBy := map[string]int{}
	for _, fx := range baselineAcceptanceFixtures {
		t.Run(fx.name, func(t *testing.T) {
			// PRESENCE PRECONDITIONS FIRST. Every one of them, before
			// any comparison: a baseline assertion over a document with
			// no text runs passes trivially, and a hash over bytes
			// nobody read certifies nothing (Story 1.1's "two empty
			// files are byte-identical").
			if len(fx.elements) == 0 {
				t.Fatalf("presence precondition: %s declares no elements", fx.name)
			}
			b := fx.render(t)
			if len(b) == 0 {
				t.Fatalf("presence precondition: %s rendered no bytes", fx.name)
			}
			assertWellFormedPDF(t, fx.name+" first-baseline acceptance", b, 1)
			if !containsFontFile2(b) {
				t.Fatalf("presence precondition: %s embeds no /FontFile2 — a baseline assertion over a document that embedded no face would certify nothing", fx.name)
			}

			runs := readEmittedRuns(t, b)
			if len(runs) == 0 {
				t.Fatalf("presence precondition: %s emitted no text runs, so every assertion below is vacuous", fx.name)
			}

			// (1) THE TABLE STATES ITS OWN PREMISES CORRECTLY. Each
			//     declared first-baseline offset equals the ruled
			//     formula applied to the declared max(A). A typo in a
			//     literal is caught here rather than being "confirmed"
			//     by whatever the renderer happened to emit.
			for _, el := range fx.elements {
				want := geom.ScaleRound(geom.Length(el.maxAscent1000), el.fontSizeMP, 1000)
				if int64(want) != el.wantFirstBaselineMP {
					t.Fatalf("%s/%s: the TABLE is inconsistent — max(A)=%d at size %d gives %d mp by the ruled formula, but the table declares %d mp",
						fx.name, el.id, el.maxAscent1000, el.fontSizeMP, want, el.wantFirstBaselineMP)
				}
			}

			// (2) PRODUCTION AGREES WITH THE TABLE'S max(A). Asserted
			//     through the real chain against the real FontSet, so
			//     the table cannot drift from the committed faces.
			fs := fixtureFontSetFor(t, fx.name)
			for _, el := range fx.elements {
				vm, err := chainVerticalModel(fx.chain, geom.Length(el.fontSizeMP), defaultLineSpacing, fs, newFontCache())
				if err != nil {
					t.Fatalf("%s/%s: chainVerticalModel(%v, %d): %v", fx.name, el.id, fx.chain, el.fontSizeMP, err)
				}
				if int64(vm.FirstBaseline) != el.wantFirstBaselineMP {
					t.Errorf("%s/%s: the model's first-baseline span is %d mp, want the hand-derived %d mp (max(A)=%d over %v at size %d)",
						fx.name, el.id, vm.FirstBaseline, el.wantFirstBaselineMP, el.maxAscent1000, fx.chain, el.fontSizeMP)
				}
				if el.wantAdvanceMP != 0 && int64(vm.Advance) != el.wantAdvanceMP {
					t.Errorf("%s/%s: the model's inter-baseline span is %d mp, want the hand-derived %d mp",
						fx.name, el.id, vm.Advance, el.wantAdvanceMP)
				}
			}

			// (3) THE ARTIFACT CARRIES THOSE BASELINES. This is the
			//     assertion the other two exist to make meaningful: the
			//     `ty` operands actually emitted into the content
			//     stream, read back off the produced PDF.
			var wantY []int64
			for _, el := range fx.elements {
				wantY = append(wantY, el.wantBaselineY...)
			}
			gotY := linesByOrigin(runs)
			if len(gotY) != len(wantY) {
				t.Fatalf("%s: the produced PDF carries %d distinct baselines %v, want %d %v — a differing COUNT means lines appeared or vanished, which is a wrapping change and not a baseline shift",
					fx.name, len(gotY), gotY, len(wantY), wantY)
			}
			for i := range wantY {
				if gotY[i] != wantY[i] {
					t.Errorf("%s: baseline %d is at pdfY %d mp, want the hand-derived %d mp (difference %+d)",
						fx.name, i, gotY[i], wantY[i], gotY[i]-wantY[i])
				}
			}

			// (4) THE OFFSET IS A PROPERTY OF THE CHAIN, NOT OF WHAT WAS
			//     DRAWN. Within one element every run shares one
			//     baseline per line, whatever face it resolved to. On
			//     multi-script-fallback that is three face segments on
			//     ONE baseline; a per-run offset derived from
			//     faces[run.Face] would split them onto three.
			//
			//     NON-DEGENERACY PRECONDITION (D-000.33): stated and
			//     checked before the spacing claim means anything.
			for _, el := range fx.elements {
				if len(el.wantBaselineY) < 2 {
					continue
				}
				if el.wantAdvanceMP == 0 {
					t.Fatalf("%s/%s declares %d baselines but no advance — a multi-line element must declare the spacing it is asserted against", fx.name, el.id, len(el.wantBaselineY))
				}
				for i := 1; i < len(el.wantBaselineY); i++ {
					// Descending pdfY: each line is `advance` LOWER.
					if d := el.wantBaselineY[i-1] - el.wantBaselineY[i]; d != el.wantAdvanceMP {
						t.Errorf("%s/%s: declared baselines %d and %d are %d mp apart, but the declared advance is %d mp — the table contradicts itself",
							fx.name, el.id, i-1, i, d, el.wantAdvanceMP)
					}
				}
				multiLineFixtures++
			}
		})

		for _, el := range fx.elements {
			// Vacuity counters over the TABLE (never an assertion about
			// a direction — D-000.45).
			// wantFirstBaselineMP > fontSizeMP means the offset from the
			// element's top is LARGER than the point size, i.e. the
			// baseline sits LOWER on the page. The reverse is a baseline
			// sitting HIGHER. Both spellings are written out here so the
			// counters cannot be read backwards downstream.
			if el.wantFirstBaselineMP > el.fontSizeMP {
				aboveEm++ // sits LOWER on the page
				aboveEmBy[fx.name]++
			}
			if el.wantFirstBaselineMP < el.fontSizeMP {
				belowEm++ // sits HIGHER on the page
				belowEmBy[fx.name]++
			}
		}
	}

	// THE VACUITY GUARD THAT MAKES THE SIGN DISCRIMINATOR REAL. If every
	// subject moved the same way, this table could not tell a computed
	// value from a directional rule — and a directional rule is FALSE on
	// font-text, which ships. D-000.45's second instance.
	if aboveEm == 0 || belowEm == 0 {
		t.Fatalf("vacuity: %d elements place the first baseline further down the page than the point size implies and %d place it further UP. BOTH must occur across the re-recorded set, or nothing here distinguishes the ruled computed value from a guard phrased as a direction (D-000.45)", aboveEm, belowEm)
	}
	if multiLineFixtures == 0 {
		t.Fatal("vacuity: no element in the re-recorded set occupies more than one baseline, so the AMENDED advance (D-2.4.2 amended) is not observed on any artifact at all")
	}
	t.Logf("AC6/D-000.44: five re-recorded goldens carry a machine-checkable first-baseline acceptance step. %d elements sit LOWER on the page than the point size implies (contributed by %v) and %d sit HIGHER (contributed by %v — a chain whose max(hhea ascent) is below 1000 em units, which is why no assertion here is phrased as a direction); %d multi-line elements observe the amended advance.",
		aboveEm, contributorsOf(aboveEmBy), belowEm, contributorsOf(belowEmBy), multiLineFixtures)
}

// contributorsOf renders a per-fixture tally in a deterministic order,
// so the acceptance test's success message reports WHICH subjects
// supply each direction from the same walk that counted them, rather
// than naming a fixture in a literal that a later re-recording can
// silently falsify.
func contributorsOf(by map[string]int) []string {
	names := make([]string, 0, len(by))
	for name := range by {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, fmt.Sprintf("%s:%d", name, by[name]))
	}
	return out
}

// renderFontTextForAcceptance renders fixtures/font-text/ through the
// public entry point, exactly as TestRenderMatchesFontTextGoldenFixture
// does.
func renderFontTextForAcceptance(t *testing.T) []byte {
	t.Helper()
	tpl := parseFontTestTemplate(t)
	res, err := Render(tpl, Data("{}"), nil, testFontSet())
	if err != nil {
		t.Fatalf("render font-text: %v", err)
	}
	return res.Bytes
}

// renderMultiScriptForAcceptance renders
// fixtures/multi-script-fallback/ through the public entry point.
func renderMultiScriptForAcceptance(t *testing.T) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(multiScriptTestTemplateJSON))
	if err != nil {
		t.Fatalf("parse multi-script template: %v", err)
	}
	res, rerr := Render(tpl, Data("{}"), nil, testShippedFontSet())
	if rerr != nil {
		t.Fatalf("render multi-script-fallback: %v", rerr)
	}
	return res.Bytes
}

// fixtureFontSetFor returns the FontSet each fixture renders against.
// font-text is the only one not using the shipped Noto chain, and that
// is exactly why it is the sign discriminator.
func fixtureFontSetFor(t *testing.T, fixture string) FontSet {
	t.Helper()
	if fixture == "font-text" {
		return testFontSet()
	}
	return testShippedFontSet()
}
