package folio8

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boxesandglue/textshape/ot"

	"github.com/panitw/folio8/folio-go/internal/fontset"
)

// ---------------------------------------------------------------------
// Story 2.3, AC2 + AC3: the frozen shaping expectation table.
//
// This is a DECLARATIVE SPEC, one record per case, and the test below
// asserts SPEC EQUALS ARTIFACT — D-000.23's consequent obligation. The
// point of the shape is that a case added later inherits every assertion
// automatically: add a row and it is checked on all five fields, counted
// in the coverage witness, classified as observable or not, and
// cross-validated against HarfBuzz, with no new assertion written.
//
// ALL FIVE FIELDS ARE ASSERTED, FOR EVERY GLYPH OF EVERY ROW, and that
// is the whole design (D-000.23). The field about to burn this project
// is GlyphID — Thai's lowered mark forms. Guarding only that would leave
// the four position fields uncovered. Guarding only XOffset — the field
// that visibly moves in these faces — would leave YOffset uncovered, and
// YOffset is the field a fourth face would use. Count the places the
// meaning lives, then assert all of them.
//
// **YOffset is UNPROVED BY THIS TABLE, and the population that is
// measured over is THIS TABLE'S SIXTEEN ROWS — not the shipped faces.**
// Measured: YOffset is 0 for every glyph of every row here, so zeroing
// YOffset after shaping reddens nothing *in this file*.
//
// THE WIDER CLAIM THIS COMMENT USED TO MAKE WAS FALSE, and it is
// corrected rather than deleted so the next reader can see the shape of
// the error. It read: YOffset "is 0 for every glyph of every row across
// all three shipped faces ... Do not manufacture a red-proof for it."
// The shipped Noto Sans Thai gives ั+tone a GPOS y-displacement of -57,
// which is the entire subject of Story 8.0 (DW-28, HIGH): the defect
// reached the owner in production, and a comment asserting the same
// unreachability in the emitter is why nobody looked (D-8.0.1). Story
// 2.3 measured ITS OWN SAMPLES and reported on THE SHIPPED SET — two
// different populations, the same measure-one-report-wider error this
// run has now recorded four times, this being the fourth site.
//
// A RED-PROOF IS THEREFORE AVAILABLE, and it is deliberately NOT taken
// here: adding a ทั้ง row would red-prove YOffset, but this table is
// bound to fixtures/shaped-text/harfbuzz-oracle.json and re-recording
// that oracle is outside Story 8.0's scope. Filed as deferred work at
// Story 8.0's close. Until it lands, treat YOffset as a forward guard
// whose red-proof exists but has not been built, and do not weaken the
// other four fields to match it.
//
// The other four ARE red-proved, and each red-proof was run:
//   - replace the shaper call with the rune->cmap loop it supersedes:
//     every Observable row reddens and every negative control stays
//     green — which is itself the proof that the negative controls are
//     negative controls;
//   - change the "ปั" row's expected 46 to the naive 45: that row
//     reddens by name;
//   - zero XOffset after shaping: "ป้ำ" and "น้ำ" redden;
//   - take advances from ot.Face.HorizontalAdvance instead of
//     GlyphPos.XAdvance: "AV" and "Wo. To," redden. That one is the
//     AD-23 float hazard turned into a test.
// ---------------------------------------------------------------------

// faceCitation names, precisely, the artifact a table row was measured
// against — not just the FontSet key the row cites.
//
// This exists because of how this story's own F3 went wrong. It recorded
// "fixtures/font-text/: Hello — shaping INVISIBLE", measured against a
// shipped Noto face. That fixture renders "Hello, World!" through
// testdata/fonts/Roboto-Regular.ttf at upem 2048, whose GPOS kerns two
// pairs; the golden had to be re-recorded. A mis-aimed measurement
// propagates further than a stated assumption, because nobody re-checks
// a measurement — a table labelled "measured" transfers its authority to
// whatever it says. The remedy is not to measure less: CITE THE SUBJECT,
// NOT JUST THE RESULT, precisely enough that a reader can tell it is the
// right artifact without re-measuring.
//
// TestShapedExpectationSubjectsAreCited asserts every citation here
// against the face actually loaded, and against shipped_faces_test.go's
// own spec, so a citation cannot drift from its subject.
type faceCitation struct {
	// File is the committed face, relative to the repository root.
	File string
	// UnitsPerEm is the face's head.unitsPerEm — the denominator every
	// number in this table is implicitly expressed against.
	UnitsPerEm uint16
	// PostScriptName is the face's own name record 6.
	PostScriptName string
}

// faceCitations is the subject of every measurement in this file.
var faceCitations = map[string]faceCitation{
	"Noto Sans":      {File: "folio-go/fonts/notosans/NotoSans-Regular.ttf", UnitsPerEm: 1000, PostScriptName: "NotoSans-Regular"},
	"Noto Sans Thai": {File: "folio-go/fonts/notosansthai/NotoSansThai-Regular.ttf", UnitsPerEm: 1000, PostScriptName: "NotoSansThai-Regular"},
	"Noto Sans SC":   {File: "folio-go/fonts/notosanssc/NotoSansSC-Regular.ttf", UnitsPerEm: 1000, PostScriptName: "NotoSansSC-Regular"},
}

// expectedGlyph is one glyph's complete expected state: the five
// independent fields that carry "this run is correctly shaped".
type expectedGlyph struct {
	GlyphID  uint16
	Cluster  int
	XAdvance int16
	XOffset  int16
	YOffset  int16
}

// shapedExpectation is one frozen case.
type shapedExpectation struct {
	// Face is the FontSet key of the shipped face this row is shaped
	// through.
	Face string
	// Text is the source string, exactly.
	Text string
	// Observable records whether this row's shaped answer DIFFERS from
	// the naive rune->cmap answer the renderer used before this story —
	// in glyph ids, in advances, or in offsets. It is frozen here and
	// RE-DERIVED at test time from the two answers, so a row whose
	// classification silently changes fails rather than drifts.
	Observable bool
	// Note says, in words, what the row is for. Negative controls say so
	// explicitly, because an unlabelled row that never changes reads
	// like a broken assertion to the next person.
	Note string
	// Glyphs is the expected output, in drawing order.
	Glyphs []expectedGlyph
}

// shapedExpectations is the table. Its Thai rows are the story: they are
// what a Thai reader sees as wrong today.
var shapedExpectations = []shapedExpectation{
	{
		Face: "Noto Sans Thai", Text: "ปั",
		Observable: true,
		Note:       "Thai tall consonant PO PLA + MAI HAN AKAT: GSUB selects the LOWERED mark form 46; the naive answer 45 collides with the ascender. THE defect this story exists to fix.",
		Glyphs: []expectedGlyph{
			{80, 0, 605, 0, 0},
			{46, 0, 0, -1, 0},
		},
	},
	{
		Face: "Noto Sans Thai", Text: "ฟั",
		Observable: true,
		Note:       "Thai tall consonant FO FAN + MAI HAN AKAT: same GSUB substitution, plus a GPOS x-offset of +21.",
		Glyphs: []expectedGlyph{
			{16, 0, 682, 0, 0},
			{46, 0, 0, 21, 0},
		},
	},
	{
		Face: "Noto Sans Thai", Text: "ที่",
		Observable: true,
		Note:       "Consonant + SARA II + MAI EK: GSUB lowers the tone mark to 44; both marks take a GPOS x-offset of -3.",
		Glyphs: []expectedGlyph{
			{117, 0, 609, 0, 0},
			{94, 0, 0, -3, 0},
			{44, 0, 0, -3, 0},
		},
	},
	{
		Face: "Noto Sans Thai", Text: "น้ำ",
		Observable: true,
		Note:       "SARA AM decomposes and reorders: 3 runes become 4 glyphs, mark 47 becomes 59, and the NIKHAHIT takes an x-offset of -29. Paired with the standalone SARA AA row below, this is the /ToUnicode CID-context case.",
		Glyphs: []expectedGlyph{
			{71, 0, 613, 0, 0},
			{59, 0, 0, 0, 0},
			{49, 0, 0, -29, 0},
			{86, 0, 406, 0, 0},
		},
	},
	{
		Face: "Noto Sans Thai", Text: "ป้ำ",
		Observable: true,
		Note:       "Tall consonant + MAI THO + SARA AM: 3 runes become 4 glyphs and the NIKHAHIT takes an x-offset of -204.",
		Glyphs: []expectedGlyph{
			{80, 0, 605, 0, 0},
			{60, 0, 0, 1, 0},
			{49, 0, 0, -204, 0},
			{86, 0, 406, 0, 0},
		},
	},
	{
		Face: "Noto Sans Thai", Text: "ณัฐวุฒิ",
		Observable: true,
		Note:       "A real Thai given name, four clusters, exercising per-syllable cluster values 0,0,2,3,3,5,5 and two GPOS offsets.",
		Glyphs: []expectedGlyph{
			{70, 0, 909, 0, 0},
			{45, 0, 0, 0, 0},
			{118, 2, 547, 0, 0},
			{134, 3, 492, 0, 0},
			{97, 3, 0, -42, 0},
			{116, 5, 903, 0, 0},
			{92, 5, 0, 36, 0},
		},
	},
	{
		Face: "Noto Sans Thai", Text: "เกิด",
		Observable: true,
		Note:       "Leading vowel SARA E, which reorders before its consonant in logical order.",
		Glyphs: []expectedGlyph{
			{91, 0, 295, 0, 0},
			{29, 1, 600, 0, 0},
			{92, 1, 0, -2, 0},
			{12, 3, 616, 0, 0},
		},
	},
	{
		Face: "Noto Sans Thai", Text: "า",
		Observable: false,
		Note:       "Standalone SARA AA: ONE glyph, and the SAME glyph that ends the merged cluster of น้ำ above. It needs /ToUnicode U+0E32 where น้ำ's tail needs the empty string, which is why CIDs are allocated per (glyph, text) and not per glyph.",
		Glyphs: []expectedGlyph{
			{86, 0, 406, 0, 0},
		},
	},
	{
		Face: "Noto Sans Thai", Text: "กรุงเทพ",
		Observable: false,
		Note:       "NEGATIVE CONTROL. Plain Thai: shapes to itself, no substitution, no offset, no advance delta. Proof that shaping does not gratuitously change text that carries no rules.",
		Glyphs: []expectedGlyph{
			{29, 0, 600, 0, 0},
			{81, 1, 488, 0, 0},
			{97, 1, 0, 0, 0},
			{58, 3, 537, 0, 0},
			{91, 4, 295, 0, 0},
			{117, 5, 609, 0, 0},
			{77, 6, 695, 0, 0},
		},
	},
	{
		Face: "Noto Sans", Text: "office",
		Observable: true,
		Note:       "Latin ffi ligature: 6 runes become 4 glyphs and glyph 1656 is drawn, which NO rune maps to.",
		Glyphs: []expectedGlyph{
			{82, 0, 605, 0, 0},
			{1656, 1, 946, 0, 0},
			{70, 4, 480, 0, 0},
			{72, 5, 564, 0, 0},
		},
	},
	{
		Face: "Noto Sans", Text: "fi",
		Observable: true,
		Note:       "Latin fi ligature: 2 runes become 1 glyph.",
		Glyphs: []expectedGlyph{
			{1654, 0, 602, 0, 0},
		},
	},
	{
		Face: "Noto Sans", Text: "AV",
		Observable: true,
		Note:       "Latin kern pair: glyph ids unchanged, XAdvance 599 against an hmtx 639 — GPOS kerning of -40, invisible to any assertion that reads only glyph ids.",
		Glyphs: []expectedGlyph{
			{36, 0, 599, 0, 0},
			{57, 1, 600, 0, 0},
		},
	},
	{
		Face: "Noto Sans", Text: "Wo. To,",
		Observable: true,
		Note:       "Two independent advance deltas in one run.",
		Glyphs: []expectedGlyph{
			{58, 0, 910, 0, 0},
			{82, 1, 605, 0, 0},
			{17, 2, 268, 0, 0},
			{3, 3, 260, 0, 0},
			{55, 4, 486, 0, 0},
			{82, 5, 605, 0, 0},
			{15, 6, 268, 0, 0},
		},
	},
	{
		Face: "Noto Sans", Text: "Hello",
		Observable: false,
		Note:       "NEGATIVE CONTROL. Shapes to itself. This is the string every existing golden fixture was measured on, and the reason those fixtures cannot observe this story.",
		Glyphs: []expectedGlyph{
			{43, 0, 741, 0, 0},
			{72, 1, 564, 0, 0},
			{79, 2, 258, 0, 0},
			{79, 3, 258, 0, 0},
			{82, 4, 605, 0, 0},
		},
	},
	{
		Face: "Noto Sans SC", Text: "结算单",
		Observable: false,
		Note:       "NEGATIVE CONTROL, and the whole CJK story. Identical glyphs, every advance exactly 1000, every offset 0. Han ideographs carry no substitution or positioning rules here. Its observable count is LEGITIMATELY zero — do not \"fix\" it.",
		Glyphs: []expectedGlyph{
			{21201, 0, 1000, 0, 0},
			{20375, 1, 1000, 0, 0},
			{10118, 2, 1000, 0, 0},
		},
	},
	{
		Face: "Noto Sans SC", Text: "结算单，共３页",
		Observable: false,
		Note:       "NEGATIVE CONTROL. CJK plus fullwidth punctuation and a fullwidth digit: still nothing changes.",
		Glyphs: []expectedGlyph{
			{21201, 0, 1000, 0, 0},
			{20375, 1, 1000, 0, 0},
			{10118, 2, 1000, 0, 0},
			{29763, 3, 1000, 0, 0},
			{9641, 4, 1000, 0, 0},
			{29770, 5, 1000, 0, 0},
			{27742, 6, 1000, 0, 0},
		},
	},
}

// naiveGlyph is the answer the renderer produced BEFORE this story: one
// cmap lookup per rune, the glyph's hmtx advance, and no offsets at all.
// It exists here as the comparison arm that makes "observable" mean
// something measurable rather than asserted.
type naiveGlyph struct {
	GlyphID  uint16
	XAdvance int16
}

// naiveAnswer reproduces the pre-2.3 rune->cmap path against the same
// face, so a row's classification as observable (or as a negative
// control) is DERIVED rather than trusted.
//
// It reads ot.Face.HorizontalAdvance deliberately — that IS the accessor
// the old path used, float32 return and all, and reproducing it exactly
// is the point. Production code must not, and does not: see
// internal/text.Shaper.Shape.
func naiveAnswer(t *testing.T, faceName, text string) []naiveGlyph {
	t.Helper()
	data, ok := testShippedFontSet()[faceName]
	if !ok {
		t.Fatalf("face %q is not in the shipped test font set", faceName)
	}
	parsed, err := ot.ParseFont(data, 0)
	if err != nil {
		t.Fatalf("face %q: parse: %v", faceName, err)
	}
	face, err := ot.NewFace(parsed)
	if err != nil {
		t.Fatalf("face %q: build face: %v", faceName, err)
	}
	cmap := face.Cmap()
	if cmap == nil {
		t.Fatalf("face %q has no cmap table", faceName)
	}
	var out []naiveGlyph
	for _, r := range text {
		gid, found := cmap.Lookup(ot.Codepoint(r))
		if !found {
			t.Fatalf("face %q has no glyph for rune %U — this row cannot be compared against the naive answer", faceName, r)
		}
		out = append(out, naiveGlyph{GlyphID: uint16(gid), XAdvance: int16(face.HorizontalAdvance(gid))})
	}
	return out
}

// shapeRow shapes one row through the real production seam.
func shapeRow(t *testing.T, faceName, text string) []expectedGlyph {
	t.Helper()
	data, ok := testShippedFontSet()[faceName]
	if !ok {
		t.Fatalf("face %q is not in the shipped test font set", faceName)
	}
	font, err := fontset.New(faceName, data)
	if err != nil {
		t.Fatalf("fontset.New(%q): %v", faceName, err)
	}
	glyphs, err := font.Shaper().Shape(text)
	if err != nil {
		t.Fatalf("Shape(%q): %v", text, err)
	}
	out := make([]expectedGlyph, len(glyphs))
	for i, g := range glyphs {
		out[i] = expectedGlyph{
			GlyphID:  g.GlyphID,
			Cluster:  g.Cluster,
			XAdvance: g.XAdvance,
			XOffset:  g.XOffset,
			YOffset:  g.YOffset,
		}
	}
	return out
}

// rowIsObservable derives whether the shaped answer differs from the
// naive one — in glyph ids, in advances, or in offsets.
func rowIsObservable(shaped []expectedGlyph, naive []naiveGlyph) bool {
	if len(shaped) != len(naive) {
		return true
	}
	for i := range shaped {
		if shaped[i].GlyphID != naive[i].GlyphID || shaped[i].XAdvance != naive[i].XAdvance {
			return true
		}
		if shaped[i].XOffset != 0 || shaped[i].YOffset != 0 {
			return true
		}
	}
	return false
}

// scriptOfFace classifies a shipped face into the three scripts this
// story reasons about. Derived from the FontSet key rather than
// enumerated per row, so a row added later is classified automatically.
func scriptOfFace(t *testing.T, faceName string) string {
	t.Helper()
	switch faceName {
	case "Noto Sans":
		return "Latin"
	case "Noto Sans Thai":
		return "Thai"
	case "Noto Sans SC":
		return "CJK"
	default:
		t.Fatalf("face %q has no script classification — add one before adding rows for it", faceName)
		return ""
	}
}

// TestShapedExpectationsMatchArtifact is AC2 and AC3 together: for every
// row, the shaped artifact must equal the frozen spec on ALL FIVE fields
// of EVERY glyph.
//
// It is deliberately one test over one table rather than a test per
// property. Splitting it would let a later row be added to the glyph-id
// test and not to the offset test, which is precisely the "guard written
// for a defect covers the defect, not its class" failure D-000.23
// names.
func TestShapedExpectationsMatchArtifact(t *testing.T) {
	if len(shapedExpectations) == 0 {
		t.Fatal("vacuity guard: the expectation table is empty, so this test asserts nothing")
	}

	for _, row := range shapedExpectations {
		t.Run(row.Face+"/"+row.Text, func(t *testing.T) {
			got := shapeRow(t, row.Face, row.Text)

			// D-000.21 sharpened: prove the artifact carries the fields
			// before asserting about their values. A shaper returning
			// nothing would otherwise make every per-glyph assertion
			// below pass vacuously, by never running.
			if len(got) == 0 {
				t.Fatalf("shaping %q through %q produced NO glyphs — every per-glyph assertion below would pass vacuously", row.Text, row.Face)
			}
			if len(got) != len(row.Glyphs) {
				t.Fatalf("glyph count: got %d, want %d\ngot:  %+v\nwant: %+v", len(got), len(row.Glyphs), got, row.Glyphs)
			}

			for i := range row.Glyphs {
				want, have := row.Glyphs[i], got[i]
				if have.GlyphID != want.GlyphID {
					t.Errorf("glyph %d: GlyphID = %d, want %d", i, have.GlyphID, want.GlyphID)
				}
				if have.Cluster != want.Cluster {
					t.Errorf("glyph %d: Cluster = %d, want %d", i, have.Cluster, want.Cluster)
				}
				if have.XAdvance != want.XAdvance {
					t.Errorf("glyph %d: XAdvance = %d, want %d", i, have.XAdvance, want.XAdvance)
				}
				if have.XOffset != want.XOffset {
					t.Errorf("glyph %d: XOffset = %d, want %d", i, have.XOffset, want.XOffset)
				}
				// FORWARD GUARD, NO AVAILABLE RED-PROOF. Every expected
				// YOffset in this table is 0, so zeroing YOffset after
				// shaping reddens nothing. It is asserted anyway, for
				// every glyph of every row, so a face that positions
				// vertically changes the table rather than slipping past
				// it. See this file's header comment.
				if have.YOffset != want.YOffset {
					t.Errorf("glyph %d: YOffset = %d, want %d", i, have.YOffset, want.YOffset)
				}
			}
		})
	}
}

// TestShapedExpectationsObservability is AC2's vacuity guard (D-000.9):
// the table must contain, and be MEASURED to contain, at least one
// shape-observable row for Thai and for Latin — and exactly zero for
// CJK, which is a legitimate negative result and is asserted as one
// rather than omitted.
//
// The "all clear" and the "I could not look" are the same value here if
// this is left implicit: a shaping implementation that was never called
// would leave every row equal to the naive answer, and a test that only
// compared spec against artifact would still be green if the spec had
// been frozen from that same broken implementation. Deriving
// observability from the two INDEPENDENT answers is what closes that.
func TestShapedExpectationsObservability(t *testing.T) {
	evaluated := map[string]int{}
	observed := map[string]int{}
	var scripts []string

	for _, row := range shapedExpectations {
		script := scriptOfFace(t, row.Face)
		if _, seen := evaluated[script]; !seen {
			scripts = append(scripts, script)
		}
		evaluated[script]++

		shaped := shapeRow(t, row.Face, row.Text)
		naive := naiveAnswer(t, row.Face, row.Text)
		got := rowIsObservable(shaped, naive)
		if got != row.Observable {
			t.Errorf(
				"%s %q: row is frozen as Observable=%v but MEASURES %v against the naive rune->cmap answer.\n"+
					"  shaped: %+v\n  naive:  %+v",
				row.Face, row.Text, row.Observable, got, shaped, naive,
			)
		}
		if got {
			observed[script]++
		}
	}

	// The coverage witness, printed rather than narrated (D-000.14).
	var witness []string
	for _, script := range scripts {
		witness = append(witness, fmt.Sprintf("%s: %d of %d rows evaluated, %d observable", script, evaluated[script], evaluated[script], observed[script]))
	}
	t.Logf("shaping coverage witness — %s", strings.Join(witness, "; "))

	for _, script := range []string{"Thai", "Latin"} {
		if evaluated[script] == 0 {
			t.Fatalf("vacuity guard: the table evaluates ZERO %s rows", script)
		}
		if observed[script] == 0 {
			t.Fatalf(
				"vacuity guard (D-000.9): ZERO shape-observable %s rows. Every %s row shapes to itself, so this "+
					"whole table would pass identically against an implementation that does no shaping at all.",
				script, script,
			)
		}
	}

	// CJK's observable count is LEGITIMATELY zero: Han ideographs in
	// Noto Sans SC carry no substitution or positioning rules, every
	// advance is exactly 1000, and shaping changes nothing. That is an
	// assertion, not an omission — and it is stated here in so many
	// words so a later reader does not "fix" it by hunting for a CJK
	// case that moves.
	if evaluated["CJK"] == 0 {
		t.Fatal("vacuity guard: the table evaluates ZERO CJK rows, so its negative control asserts nothing")
	}
	if observed["CJK"] != 0 {
		t.Fatalf(
			"CJK reports %d shape-observable rows, want exactly 0. CJK not changing is a MEASURED PROPERTY of "+
				"these faces, not an oversight: if this ever becomes non-zero, the shipped CJK face has gained "+
				"layout rules and that is a finding to investigate, not a number to update.",
			observed["CJK"],
		)
	}
}

// TestClusterValuesAreRuneIndices pins the cluster semantics
// internal/text.ClusterTexts depends on, because it indexes a []rune
// with a cluster value.
//
// This corrects the story's F7, which states clusters are BYTE offsets.
// Measured at textshape v0.0.15 and cross-checked against hb-shape
// 14.2.0: they are RUNE indices. "office" -> 0,1,4,5 is ambiguous
// between the two readings because it is ASCII; "ณัฐวุฒิ" ->
// 0,0,2,3,3,5,5 is not, because byte offsets over seven three-byte Thai
// runes would be multiples of 3.
func TestClusterValuesAreRuneIndices(t *testing.T) {
	got := shapeRow(t, "Noto Sans Thai", "ณัฐวุฒิ")
	want := []int{0, 0, 2, 3, 3, 5, 5}
	if len(got) != len(want) {
		t.Fatalf("got %d glyphs, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Cluster != want[i] {
			t.Fatalf(
				"glyph %d: cluster = %d, want %d. If these are multiples of 3 the vendor has switched to BYTE "+
					"offsets and internal/text.ClusterTexts, which indexes a []rune, is now wrong.",
				i, got[i].Cluster, want[i],
			)
		}
	}
}

// TestShapedExpectationSubjectsAreCited holds the citations honest: every
// face named by a table row must have a citation, that citation's
// unitsPerEm and PostScript name must match the face actually loaded,
// and its committed path must exist. A row measured against the wrong
// artifact is then visible on the page rather than only by re-measuring
// — which is the failure this story's own F3 demonstrated.
func TestShapedExpectationSubjectsAreCited(t *testing.T) {
	root := repoRootFromTest(t)
	cited := map[string]bool{}
	for _, row := range shapedExpectations {
		cited[row.Face] = true
	}
	if len(cited) == 0 {
		t.Fatal("vacuity guard: no faces are cited by any table row")
	}

	for _, row := range shapedExpectations {
		citation, ok := faceCitations[row.Face]
		if !ok {
			t.Fatalf("table row %q/%q names a face with no citation — name the artifact it was measured against", row.Face, row.Text)
		}
		data, present := testShippedFontSet()[row.Face]
		if !present {
			t.Fatalf("face %q is cited but is not in the shipped test font set", row.Face)
		}
		font, err := fontset.New(row.Face, data)
		if err != nil {
			t.Fatalf("fontset.New(%q): %v", row.Face, err)
		}
		if got := font.UnitsPerEm(); got != citation.UnitsPerEm {
			t.Errorf("face %q: cited unitsPerEm %d, actual %d — every number in this table is expressed against that denominator", row.Face, citation.UnitsPerEm, got)
		}
		if got := font.PostScriptName(); got != citation.PostScriptName {
			t.Errorf("face %q: cited PostScript name %q, actual %q", row.Face, citation.PostScriptName, got)
		}
		if _, serr := os.Stat(filepath.Join(root, citation.File)); serr != nil {
			t.Errorf("face %q: cited file %s does not exist: %v", row.Face, citation.File, serr)
		}
	}
}
