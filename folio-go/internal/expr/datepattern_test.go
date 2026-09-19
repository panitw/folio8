package expr

import "testing"

// dateFieldTokensExpected is AC10a's closed, PERMANENT token set,
// pinned as a literal this TEST owns (engineering-lead ruling, Finding
// 4, this story's QA review) — never derived from datepattern.go's
// source via an AST scan.
//
// WHY THE PREVIOUS AST-SCAN VERSION WAS TAUTOLOGICAL, NOT MERELY WEAK:
// parseDatePattern (datepattern.go) validates by looking up
// dateFieldTokens itself — there is no second, independent
// implementation to disagree with. The old TestDateFieldTokenSetMatchesParser
// therefore compared that map to itself, twice over: "every declared
// token is accepted" cannot fail (the parser consults the same map),
// and the "closed" half's candidate alphabet (lettersInUse) was built
// FROM the declared set, so a token built from any OTHER letter was
// never even tried. Measured: adding a hand-written special case to
// parseDatePattern accepting "HH", with NO entry in dateFieldTokens,
// left both old tests and the entire internal/expr package green.
//
// WHY A LITERAL IS CORRECT HERE DESPITE D-3.1a.3's warning against
// hard-coded name lists that get edited in the same diff as the thing
// they guard (which is why Story 3.3 removed the `implemented`/
// `owningStory` name lists): that rule is about a population the
// roadmap is SCHEDULED to change — the two functions this story and
// 3.3 flip were always going to move, in lockstep with their guard.
// The date-token grammar is CLOSED and PERMANENT by AC10a's own text
// ("no quoting mechanism ships"); nothing in the roadmap grows it. For
// a permanently-closed set, a literal expectation is exactly right:
// growing the set should require deliberately editing a test that says
// "this set is closed" — pin to a literal when the set is permanent,
// state it relationally when it is scheduled to move.
var dateFieldTokensExpected = []string{"yyyy", "MMMM", "MM", "M", "dd", "d"}

// TestDateFieldTokenSetMatchesParser is AC10a's repaired set-equality
// check, in three parts per the engineering-lead ruling:
//
//  1. dateFieldTokensExpected (this test's own literal) is compared
//     against dateFieldTokens (the real source map, built from
//     datepattern.go's dateFieldTokenList) by set equality in BOTH
//     directions.
//  2. Every token in dateFieldTokensExpected must parse as exactly one
//     field token.
//  3. The CLOSED half is widened to ALL 52 ASCII letters (a-z, A-Z),
//     not merely letters already in use by a declared token, at every
//     run length 1-5 — and the expected acceptance is read from
//     dateFieldTokensExpected, never from dateFieldTokens itself, so a
//     parser special case for a brand-new letter (the measured "HH"
//     defect) is caught: parseDatePattern("HH") would return no error,
//     dateFieldTokensExpected has no "HH" entry, and the mismatch
//     reddens this test.
func TestDateFieldTokenSetMatchesParser(t *testing.T) {
	if len(dateFieldTokensExpected) == 0 {
		t.Fatal("presence precondition (D-000.9): dateFieldTokensExpected is empty")
	}

	if len(dateFieldTokens) != len(dateFieldTokensExpected) {
		t.Errorf("dateFieldTokens has %d entries, dateFieldTokensExpected has %d (%v vs %v)",
			len(dateFieldTokens), len(dateFieldTokensExpected), dateFieldTokens, dateFieldTokensExpected)
	}
	expectedSet := map[string]bool{}
	for _, tok := range dateFieldTokensExpected {
		expectedSet[tok] = true
		if !dateFieldTokens[tok] {
			t.Errorf("dateFieldTokensExpected declares %q, but dateFieldTokens (the source) does not — a token this test expects to stay closed was removed", tok)
		}
	}
	for tok := range dateFieldTokens {
		if !expectedSet[tok] {
			t.Errorf("dateFieldTokens declares %q, but dateFieldTokensExpected does not — the closed set grew without this test being updated", tok)
		}
	}

	for _, tok := range dateFieldTokensExpected {
		toks, err := parseDatePattern(tok)
		if err != nil {
			t.Errorf("declared token %q was rejected by parseDatePattern: %v", tok, err)
			continue
		}
		if len(toks) != 1 || !toks[0].Field || toks[0].Text != tok {
			t.Errorf("declared token %q did not parse as a single field token, got %+v", tok, toks)
		}
	}

	// The widened "closed" half: every ASCII letter, every run length
	// 1-5, expected acceptance read from THIS TEST'S OWN literal, never
	// from dateFieldTokens.
	for _, r := range asciiLetters() {
		for n := 1; n <= 5; n++ {
			candidate := stringsRepeatRune(r, n)
			_, err := parseDatePattern(candidate)
			accepted := err == nil
			wantAccepted := expectedSet[candidate]
			if accepted != wantAccepted {
				t.Errorf("parseDatePattern(%q): accepted=%v, want %v (expected set %v)", candidate, accepted, wantAccepted, dateFieldTokensExpected)
			}
		}
	}
}

// asciiLetters is the widened closed-half's full candidate alphabet
// (Finding 4): all 52 ASCII letters, independent of which letters any
// declared token happens to use.
func asciiLetters() []rune {
	var out []rune
	for r := rune('a'); r <= 'z'; r++ {
		out = append(out, r)
	}
	for r := rune('A'); r <= 'Z'; r++ {
		out = append(out, r)
	}
	return out
}

func stringsRepeatRune(r rune, n int) string {
	out := make([]rune, n)
	for i := range out {
		out[i] = r
	}
	return string(out)
}
