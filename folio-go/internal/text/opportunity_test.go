package text

import "testing"

// runesOf is a readability helper: the tests below state positions in
// rune indices, and the inputs are multi-byte, so a byte-indexed
// mistake would be invisible without it.
func runesOf(s string) []rune { return []rune(s) }

// TestLatinBreaksAfterWhitespaceRunAndConsumesIt is AC1.
//
// Presence precondition: the opportunity set asserted on is required to
// be NON-EMPTY, so an implementation returning nil for every input
// cannot pass. The exact positions are asserted, not merely the count,
// because a count assertion cannot tell "after the space" from "before
// the space" — which is the whole polarity AC1 fixes.
//
// Red-proof: change Opportunities to emit {LineEnd: j, NextStart: j}
// (break AFTER the run but draw the space on the previous line) or
// {LineEnd: i, NextStart: i} (break BEFORE the run and draw the space
// on the next line); either moves "Page footer 0123456789" off the
// asserted pair and this test fails naming the input.
func TestLatinBreaksAfterWhitespaceRunAndConsumesIt(t *testing.T) {
	dict := Dictionary()

	const subject = "Page footer 0123456789"
	ops := Opportunities(dict, subject, nil)
	if len(ops) == 0 {
		t.Fatalf("precondition: %q yielded no break opportunities at all — the assertions below would be vacuous", subject)
	}

	// "Page footer 0123456789": spaces at rune 4 and rune 11.
	want := []Opportunity{{LineEnd: 4, NextStart: 5}, {LineEnd: 11, NextStart: 12}}
	if len(ops) != len(want) {
		t.Fatalf("%q: got %d opportunities %v, want %d %v", subject, len(ops), ops, len(want), want)
	}
	for i := range want {
		if ops[i] != want[i] {
			t.Errorf("%q: opportunity %d = %+v, want %+v — LineEnd is where the line ENDS (before the space) and NextStart where the next line BEGINS (after it)", subject, i, ops[i], want[i])
		}
	}

	// The consumed run is genuinely consumed: the whitespace belongs to
	// neither side.
	r := runesOf(subject)
	for _, o := range ops {
		if o.NextStart == o.LineEnd {
			t.Errorf("%q: opportunity %+v consumes nothing, but it sits at a whitespace run", subject, o)
			continue
		}
		for k := o.LineEnd; k < o.NextStart; k++ {
			if r[k] != ' ' {
				t.Errorf("%q: opportunity %+v claims to consume rune %d = %q, which is not whitespace", subject, o, k, string(r[k]))
			}
		}
	}
}

// TestLatinDoesNotBreakAnywhereElse is AC1's negative half, stated as a
// category rather than as a list of samples: for a Latin input with no
// whitespace at all, there must be ZERO opportunities — no hyphenation,
// no break at "-", no break inside a word.
func TestLatinDoesNotBreakAnywhereElse(t *testing.T) {
	dict := Dictionary()
	for _, subject := range []string{
		"co-operative",
		"antidisestablishmentarianism",
		"office",
		"AV",
		"0123456789",
		"e-mail@example.com",
	} {
		ops := Opportunities(dict, subject, nil)
		if len(ops) != 0 {
			t.Errorf("%q: got %d break opportunities %v, want 0 — Latin breaks at whitespace and nowhere else (this is NOT UAX #14)", subject, len(ops), ops)
		}
	}
}

// TestCJKBreaksBetweenAdjacentIdeographs is AC2.
//
// Presence precondition: the subject is the CJK element of the
// COMMITTED fixtures/shaped-text/ template (e5), whose glyph coverage
// against the shipped Noto Sans SC face is already proven by that
// fixture's own tests — so this is asserted on content the engine
// demonstrably renders, not on a string invented for this test.
//
// Red-proof: restrict isCJKIdeographOrKana to kana only (drop
// unicode.Han) and the subject loses every opportunity, failing the
// non-empty precondition; restrict it to Han only and the kana subject
// below loses its.
func TestCJKBreaksBetweenAdjacentIdeographs(t *testing.T) {
	dict := Dictionary()

	// fixtures/shaped-text/input.folio element e5.
	const subject = "结算单，共３页"
	ops := Opportunities(dict, subject, nil)
	if len(ops) == 0 {
		t.Fatalf("precondition: %q yielded no break opportunities — AC2's assertion would be vacuous", subject)
	}

	// 结 算 单 ， 共 ３ 页 — adjacent Han pairs are (结,算) and (算,单)
	// only. "，" (U+FF0C) and "３" (U+FF13) are neither Han nor kana, so
	// no opportunity touches them: that is kinsoku's territory and
	// kinsoku is not implemented.
	want := []Opportunity{{LineEnd: 1, NextStart: 1}, {LineEnd: 2, NextStart: 2}}
	if len(ops) != len(want) {
		t.Fatalf("%q: got %d opportunities %v, want %d %v", subject, len(ops), ops, len(want), want)
	}
	for i := range want {
		if ops[i] != want[i] {
			t.Errorf("%q: opportunity %d = %+v, want %+v", subject, i, ops[i], want[i])
		}
	}

	// The kana half of the range, so dropping either script from the
	// classifier is caught. "こんにちは" is the same Japanese sample
	// TestJapaneseTextThroughPanCJKFaceDoesNotFireDiagnostic renders
	// through the Pan-CJK face.
	const kana = "こんにちは"
	kops := Opportunities(dict, kana, nil)
	if len(kops) != len(runesOf(kana))-1 {
		t.Errorf("%q: got %d opportunities %v, want %d (between every adjacent kana pair)", kana, len(kops), kops, len(runesOf(kana))-1)
	}
}

// TestCJKZeroWidthBreaksConsumeNothing pins the other half of the
// Opportunity contract: a CJK break draws every rune, on one side or
// the other. Only whitespace is consumed.
func TestCJKZeroWidthBreaksConsumeNothing(t *testing.T) {
	dict := Dictionary()
	const subject = "结算单"
	ops := Opportunities(dict, subject, nil)
	if len(ops) == 0 {
		t.Fatalf("precondition: %q yielded no opportunities", subject)
	}
	for _, o := range ops {
		if o.LineEnd != o.NextStart {
			t.Errorf("%q: CJK opportunity %+v consumes runes [%d,%d); no CJK break may drop text", subject, o, o.LineEnd, o.NextStart)
		}
	}
}

// TestThaiOpportunitiesComeFromTheDictionaryEngine is AC3: the Thai
// positions Opportunities reports are exactly ComputeBreaks'
// constrained answer, minus the ones adjacent to whitespace (which rule
// 1 owns, at the opposite polarity).
//
// Presence precondition: a Thai subject measured to carry at least one
// interior dictionary break.
func TestThaiOpportunitiesComeFromTheDictionaryEngine(t *testing.T) {
	dict := Dictionary()

	const subject = "ประเทศไทย"
	engine, _ := ComputeBreaks(dict, subject, false)
	if len(engine) == 0 {
		t.Fatalf("precondition: %q carries no dictionary break at all", subject)
	}
	ops := Opportunities(dict, subject, nil)
	if len(ops) != len(engine) {
		t.Fatalf("%q: Opportunities reported %v, ComputeBreaks reported %v", subject, ops, engine)
	}
	for i, b := range engine {
		if ops[i].LineEnd != b || ops[i].NextStart != b {
			t.Errorf("%q: opportunity %d = %+v, want a zero-width break at %d", subject, i, ops[i], b)
		}
	}

	// AD-25's atomic-unknown-run absolute still holds through this API:
	// a Thai run the dictionary cannot cover proposes no interior break.
	// "ชัยวัฒน์" is the [0,8) uncoverable span of corpus name-021.
	const uncoverable = "ชัยวัฒน์"
	if got := Opportunities(dict, uncoverable, nil); len(got) != 0 {
		t.Errorf("%q is not dictionary-tileable, so AD-25 requires zero interior breaks; got %v", uncoverable, got)
	}
}

// TestAtomicSpansRemoveInteriorOpportunities is AC7's mechanism,
// asserted at the breaking API where the parameter arrives.
//
// PRESENCE PRECONDITION, AND IT IS THE POINT: each case first proves
// that the UNDECLARED form of the same input DOES carry an interior
// opportunity in that span. Asserting only the declared case would be
// vacuous — it could not distinguish an honoured declaration from a
// value that never had a break opportunity to begin with. Both
// polarities, every case.
//
// Coverage is by construction over the span set — the test drives the
// spans, not a list of sample strings (D-000.23).
func TestAtomicSpansRemoveInteriorOpportunities(t *testing.T) {
	dict := Dictionary()

	cases := []struct {
		name    string
		text    string
		atomic  []Span
		wantIn  []Span // spans that must end up break-free
		wantOut int    // opportunities expected to survive outside them
	}{
		{
			// D-2.1.6's worked case: "ศรีสุข" as a surname is
			// byte-identical to "ศรี" + "สุข", both in the dictionary.
			name:    "thai surname, whole string declared",
			text:    "ศรีสุข",
			atomic:  []Span{{Start: 0, End: 6}},
			wantIn:  []Span{{Start: 0, End: 6}},
			wantOut: 0,
		},
		{
			name:    "thai surname inside a literal sentence, only the value declared",
			text:    "ผู้รับ ศรีสุข",
			atomic:  []Span{{Start: 7, End: 13}},
			wantIn:  []Span{{Start: 7, End: 13}},
			wantOut: 1, // the whitespace break after "ผู้รับ" survives
		},
		{
			// Two placeholders: a filter that marked only the FIRST
			// substituted span atomic passes the one-span cases above
			// and fails here. That is this test's red-proof.
			name:    "two declared spans, both must be honoured",
			text:    "ศรีสุข ศรีสุข",
			atomic:  []Span{{Start: 0, End: 6}, {Start: 7, End: 13}},
			wantIn:  []Span{{Start: 0, End: 6}, {Start: 7, End: 13}},
			wantOut: 1,
		},
		{
			// The mechanism is script-agnostic by construction.
			name:    "CJK declared span",
			text:    "结算单",
			atomic:  []Span{{Start: 0, End: 3}},
			wantIn:  []Span{{Start: 0, End: 3}},
			wantOut: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Polarity 1 — UNDECLARED. Prove the interior break exists.
			undeclared := Opportunities(dict, tc.text, nil)
			for _, sp := range tc.wantIn {
				interior := 0
				for _, o := range undeclared {
					if o.LineEnd > sp.Start && o.LineEnd < sp.End {
						interior++
					}
				}
				if interior == 0 {
					t.Fatalf("PRECONDITION FAILED: %q carries no interior break opportunity in span %+v even when UNDECLARED (%v) — asserting its absence when declared would assert nothing", tc.text, sp, undeclared)
				}
			}

			// Polarity 2 — DECLARED. The interior breaks are gone.
			declared := Opportunities(dict, tc.text, tc.atomic)
			for _, sp := range tc.wantIn {
				for _, o := range declared {
					if o.LineEnd > sp.Start && o.LineEnd < sp.End {
						t.Errorf("%q: declared span %+v still carries an interior break %+v", tc.text, sp, o)
					}
					if o.NextStart > sp.Start && o.NextStart < sp.End {
						t.Errorf("%q: declared span %+v still has a line STARTING inside it: %+v", tc.text, sp, o)
					}
				}
			}

			// Literal text outside the declared spans is unaffected.
			if len(declared) != tc.wantOut {
				t.Errorf("%q: %d opportunities survived outside the declared spans (%v), want %d — a declaration must not suppress breaks in literal text around it", tc.text, len(declared), declared, tc.wantOut)
			}
		})
	}
}

// TestMandatoryBreaksComeFromLineFeedsTheCallerSupplied is Story 7.1's
// core: a line feed in the input is a break the packer may not decline,
// and k of them are SEPARATORS yielding k+1 lines.
//
// Every case states the full expected opportunity set, kinds included,
// because a count cannot tell "one break per line feed" from "one break
// per whitespace run" on "a\n\nb", and it cannot tell a mandatory break
// from the optional one it replaced.
func TestMandatoryBreaksComeFromLineFeedsTheCallerSupplied(t *testing.T) {
	dict := Dictionary()

	cases := []struct {
		name string
		text string
		want []Opportunity
	}{
		{
			// The whole point: the run [1,2) sits between two runes, so
			// rule 1 would have proposed an OPTIONAL break here. The
			// line feed replaces it rather than joining it.
			name: "interior line feed",
			text: "a\nb",
			want: []Opportunity{{LineEnd: 1, NextStart: 2, Kind: BreakMandatory}},
		},
		{
			// A paragraph gap. ONE whitespace run, TWO breaks — this is
			// the case a "one opportunity per run" implementation gets
			// wrong while passing every single-break case.
			name: "paragraph gap",
			text: "a\n\nb",
			want: []Opportunity{
				{LineEnd: 1, NextStart: 2, Kind: BreakMandatory},
				{LineEnd: 2, NextStart: 3, Kind: BreakMandatory},
			},
		},
		{
			// D-7.1.2: the trailing run reaches the end of the string,
			// so rule 1's `j < n` guard would suppress it. The guard is
			// unchanged and simply does not reach the mandatory path.
			name: "trailing line feed",
			text: "a\n",
			want: []Opportunity{{LineEnd: 1, NextStart: 2, Kind: BreakMandatory}},
		},
		{
			// D-7.1.7 finding 1: LineEnd == 0, which the collection
			// loop could not even reach before this story.
			name: "leading line feed",
			text: "\na",
			want: []Opportunity{{LineEnd: 0, NextStart: 1, Kind: BreakMandatory}},
		},
		{
			// A value that is nothing but a break: one separator, two
			// empty lines. Never zero opportunities.
			name: "line feed alone",
			text: "\n",
			want: []Opportunity{{LineEnd: 0, NextStart: 1, Kind: BreakMandatory}},
		},
		{
			// \r\n IS ONE BREAK. Two would give a spurious empty line
			// on every document authored on Windows.
			name: "CRLF is one break",
			text: "a\r\nb",
			want: []Opportunity{{LineEnd: 1, NextStart: 3, Kind: BreakMandatory}},
		},
		{
			// ...and two CRLFs are two breaks, so a Windows paragraph
			// gap is still a paragraph gap. Without this the "one
			// break" case above is satisfied by an implementation that
			// folds a whole run into one break.
			// a(0) \r(1) \n(2) \r(3) \n(4) b(5): one run [1,5) with
			// line feeds at 2 and 4, so the partition is {1,4} and
			// {4,5} — lines "a", empty, "b".
			name: "two CRLFs are two breaks",
			text: "a\r\n\r\nb",
			want: []Opportunity{
				{LineEnd: 1, NextStart: 4, Kind: BreakMandatory},
				{LineEnd: 4, NextStart: 5, Kind: BreakMandatory},
			},
		},
		{
			// A lone carriage return is UNCHANGED: it carries no line
			// feed, so its run stays an ordinary optional whitespace
			// break. This is the negative control for the CRLF cases —
			// it proves they are about \n and not about \r.
			name: "lone CR is unchanged",
			text: "a\rb",
			want: []Opportunity{{LineEnd: 1, NextStart: 2, Kind: BreakOptional}},
		},
		{
			// D-7.1.6 / AC5: the whole run is consumed at its OUTER
			// edges, exactly as an optional whitespace break consumes
			// its run. Neither space is drawn on either line.
			name: "spaces adjacent to the break are consumed with it",
			text: "a \n b",
			want: []Opportunity{{LineEnd: 1, NextStart: 4, Kind: BreakMandatory}},
		},
		{
			// The Design Notes' worked partition, verbatim:
			// a(0) sp(1) \n(2) \n(3) sp(4) b(5); run [1,5).
			name: "worked partition of a run holding two line feeds",
			text: "a \n\n b",
			want: []Opportunity{
				{LineEnd: 1, NextStart: 3, Kind: BreakMandatory},
				{LineEnd: 3, NextStart: 5, Kind: BreakMandatory},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Opportunities(dict, tc.text, nil)
			if len(got) == 0 {
				t.Fatalf("precondition: %q yielded no opportunities at all — every assertion below would be vacuous", tc.text)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("%q: got %d opportunities %+v, want %d %+v", tc.text, len(got), got, len(tc.want), tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("%q: opportunity %d = %+v, want %+v", tc.text, i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestMandatoryBreakSurvivesADeclaredSpan is D-7.1.1 at the filter site,
// and it is the assertion the whole ruling turns on.
//
// ONE span covering a value that holds BOTH a line feed and a space, so
// the case discriminates in both directions at once: the mandatory break
// must survive and the space's optional break must not. Either assertion
// alone cannot tell "the exemption works" from "span suppression broke".
//
// Red-proof: flip the kind test at opportunity.go's filter site (drop
// `o.Kind != BreakMandatory`) and the line-feed half fails; delete the
// span and the space half fails. The two failures are distinguishable,
// which is why the span is left in place rather than removed.
func TestMandatoryBreakSurvivesADeclaredSpan(t *testing.T) {
	dict := Dictionary()

	// "to: first\nsecond word" — the declared value is the substituted
	// tail, runes [4,21).
	const subject = "to: first\nsecond word"
	runes := runesOf(subject)
	const spanStart, spanEnd = 4, 21
	if spanEnd != len(runes) {
		t.Fatalf("precondition: the declared span [%d,%d) does not reach the end of %q (%d runes)", spanStart, spanEnd, subject, len(runes))
	}
	span := Span{Start: spanStart, End: spanEnd}

	// PRECONDITION, AND IT IS THE POINT (the "fixture trap", D-7.1.1):
	// both positions must be STRICTLY INTERIOR to the span, or
	// spansCover would never have looked at them and the exemption
	// would be proved by an input that never needed it.
	lf, sp := -1, -1
	for i, r := range runes {
		if r == '\n' {
			lf = i
		}
		if r == ' ' && i > spanStart {
			sp = i
		}
	}
	if lf < 0 || sp < 0 {
		t.Fatalf("precondition: %q must contain both a line feed and an interior space; found lf=%d sp=%d", subject, lf, sp)
	}
	if !(lf > span.Start && lf < span.End) {
		t.Fatalf("precondition: the line feed at rune %d is not STRICTLY inside the declared span %+v, so spansCover would never have suppressed it and the exemption would be vacuous", lf, span)
	}
	if !(sp > span.Start && sp < span.End) {
		t.Fatalf("precondition: the space at rune %d is not STRICTLY inside the declared span %+v, so its suppression would prove nothing", sp, span)
	}

	// Polarity 1 — UNDECLARED. Both breaks exist.
	undeclared := Opportunities(dict, subject, nil)
	foundLF, foundSpace := false, false
	for _, o := range undeclared {
		if o.LineEnd == lf && o.Kind == BreakMandatory {
			foundLF = true
		}
		if o.LineEnd == sp && o.Kind == BreakOptional {
			foundSpace = true
		}
	}
	if !foundLF {
		t.Fatalf("precondition: %q carries no mandatory break at rune %d even UNDECLARED (%+v)", subject, lf, undeclared)
	}
	if !foundSpace {
		t.Fatalf("precondition: %q carries no optional break at rune %d even UNDECLARED (%+v)", subject, sp, undeclared)
	}

	// Polarity 2 — DECLARED. The mandatory break survives; the optional
	// one does not.
	declared := Opportunities(dict, subject, []Span{span})
	survivedLF, survivedSpace := false, false
	for _, o := range declared {
		if o.LineEnd == lf {
			survivedLF = true
			if o.Kind != BreakMandatory {
				t.Errorf("the opportunity at rune %d survived the declared span but is %v, not BreakMandatory — the exemption must be keyed on the KIND", lf, o.Kind)
			}
		}
		if o.LineEnd == sp {
			survivedSpace = true
		}
	}
	if !survivedLF {
		t.Errorf("the mandatory break at rune %d was suppressed by declared span %+v (%+v) — declaring a value unbreakable stops the engine GUESSING at a break, it does not throw away a break somebody SUPPLIED (D-7.1.1)", lf, span, declared)
	}
	if survivedSpace {
		t.Errorf("the OPTIONAL break at rune %d survived declared span %+v (%+v) — the exemption must be keyed on the kind, not applied to the span as a whole", sp, span, declared)
	}
	t.Logf("D-7.1.1: within declared span %+v of %q, the mandatory break at %d survives and the optional break at %d does not; opportunities = %+v", span, subject, lf, sp, declared)
}

// TestMandatoryBreakIsNeverDisplacedByAnotherRule is the assertion the
// story's Design Notes rely on when they decline to add a precedence
// guard to `add`.
//
// The argument is that a mandatory break can only be displaced by
// another opportunity proposed at the SAME LineEnd, and that none can
// be: rule 2 requires both neighbours to be Han-or-kana, rule 3 skips
// any position adjacent to whitespace, and a line-feed-bearing run emits
// mandatory breaks INSTEAD OF rule 1's optional one. That reasoning was
// stated and nothing checked it — and the collision, if it existed,
// would show up precisely where the other two rules are live, which is
// CJK and Thai text.
//
// So the subjects put a line feed hard against Han, kana and Thai on
// both sides, and assert the outcome (every line feed still yields a
// surviving MANDATORY break at its own position) rather than adding the
// guard.
func TestMandatoryBreakIsNeverDisplacedByAnotherRule(t *testing.T) {
	dict := Dictionary()

	subjects := []string{
		"结算单\n共三页",   // Han on both sides — rule 2 is live either side of the break
		"结算单\n\n共三页", // ...and across a paragraph gap
		"ประเทศไทย\nประเทศไทย",   // Thai on both sides - rule 3 is live either side
		"ประเทศไทย\n\nประเทศไทย", // Thai, paragraph gap
		"结算单\nประเทศไทย",         // the two rules meet across the break
		"ศรีสุข \n 结算单",          // whitespace adjacent to the break, both scripts
	}

	for _, subject := range subjects {
		t.Run(subject, func(t *testing.T) {
			runes := runesOf(subject)
			ops := Opportunities(dict, subject, nil)

			// PRECONDITION, AND IT IS WHAT MAKES THE CASE A TEST: the
			// OTHER rules must actually be proposing here, or "no
			// collision" is satisfied by an input where nothing could
			// have collided. Measured on the same string with its line
			// feeds turned into ordinary spaces, so the script context
			// either side of the break is identical.
			neighbourly := Opportunities(dict, replaceRune(subject, '\n', ' '), nil)
			interior := 0
			for _, o := range neighbourly {
				if o.LineEnd == o.NextStart { // a zero-width break: rule 2 or rule 3 proposed it
					interior++
				}
			}
			if interior == 0 {
				t.Fatalf("precondition: %q proposes no CJK/Thai break at all even with its line feeds replaced by spaces — this case cannot express a collision", subject)
			}

			// Every line feed must still have a MANDATORY break whose
			// LineEnd is the run's start or the feed's own index, and
			// no optional opportunity may occupy a mandatory's LineEnd.
			feeds := 0
			for _, r := range runes {
				if r == '\n' {
					feeds++
				}
			}
			mandatory := 0
			for _, o := range ops {
				if o.Kind == BreakMandatory {
					mandatory++
				}
			}
			if mandatory != feeds {
				t.Errorf("%q carries %d line feed(s) but yielded %d mandatory opportunit(ies) %+v — another rule displaced one at a shared LineEnd, which the Design Notes argue cannot happen", subject, feeds, mandatory, ops)
			}

			// And the collapse helper's own outcome: no two entries
			// share a LineEnd, so "which one won" never arises.
			seen := map[int]bool{}
			for _, o := range ops {
				if seen[o.LineEnd] {
					t.Errorf("%q yielded two opportunities at LineEnd %d (%+v) — the collapse in `add` is the only arbiter and it should never have been consulted", subject, o.LineEnd, ops)
				}
				seen[o.LineEnd] = true
			}
			t.Logf("%q: %d interior CJK/Thai break(s) available, %d line feed(s), %d mandatory survived, %d opportunities total", subject, interior, feeds, mandatory, len(ops))
		})
	}
}

// replaceRune is a test-local helper: the neighbour-context precondition
// above needs the SAME string with only its line feeds changed, so that
// what the other two rules see either side of the position is identical.
func replaceRune(s string, from, to rune) string {
	out := []rune(s)
	for i, r := range out {
		if r == from {
			out[i] = to
		}
	}
	return string(out)
}
