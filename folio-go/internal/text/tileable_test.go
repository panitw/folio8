package text

import "testing"

// TestForEachWordPrefixMatchesContains is the agreement assertion
// tileable.go's doc comment claims rather than assumes: the one-walk
// prefix enumeration reports exactly the prefix lengths Contains would
// call entries, over the SHIPPED trie and the COMMITTED corpus.
//
// The two are separate derivations — forEachWordPrefix descends once and
// reports word-end flags as it passes them; Contains re-descends from
// the root per candidate and re-encodes a string each time — so this is
// a cross-check, not a function compared against itself.
//
// Presence precondition and N-of-N witness: the test counts both the
// comparisons it made and the agreements that were POSITIVE (a prefix
// both methods called an entry). A walk that reported nothing and a
// Contains that answered false everywhere would agree vacuously on
// every comparison, so the positive count is asserted non-zero too.
func TestForEachWordPrefixMatchesContains(t *testing.T) {
	items := loadCorpus(t)
	if len(items) == 0 {
		t.Fatal("precondition: corpus is empty — nothing to compare over")
	}
	dict := Dictionary()

	var comparisons, positives, disagreements int
	for _, it := range items {
		runes := []rune(it.Text)
		for start := 0; start < len(runes); start++ {
			span := runes[start:]
			enc, byteAt, runeAt := encodeSpan(span)

			walked := map[int]bool{}
			dict.forEachWordPrefix(enc, func(byteLen int) {
				walked[runeAt[byteLen]] = true
			})

			for end := 1; end <= len(span); end++ {
				want := dict.Contains(string(span[:end]))
				got := walked[end]
				comparisons++
				if want {
					positives++
				}
				if got != want {
					disagreements++
					t.Errorf("forEachWordPrefix disagrees with Contains: item=%s start=%d end=%d text=%q walk=%v Contains=%v",
						it.ID, start, end, string(span[:end]), got, want)
				}
			}
			_ = byteAt
		}
	}

	if comparisons == 0 {
		t.Fatal("vacuity: zero comparisons were made")
	}
	if positives == 0 {
		t.Fatal("vacuity: every comparison was negative-agrees-with-negative — the walk was never actually exercised against a real dictionary entry")
	}
	if disagreements == 0 {
		t.Logf("forEachWordPrefix agrees with Contains on %d of %d prefix comparisons across %d corpus items, %d of them positive (a real dictionary entry)",
			comparisons, comparisons, len(items), positives)
	}
}

// TestTileabilityAgreesWithIndependentDP cross-checks the WHOLE-span
// answer of tileableForward and tileableBackward against
// isFullySegmentable — the independent DP that p2_independent_test.go
// uses as ground truth and that never calls into this package's
// matching code (D-000.9).
//
// Both directions are checked, because the filter needs both and a bug
// in only one would still let the forward half look right.
//
// Presence precondition: the corpus is asserted to contain BOTH
// tileable and untileable Thai spans, so neither polarity is asserted
// vacuously. Measured at the fix commit: 383 Thai spans, 58 of them
// untileable.
func TestTileabilityAgreesWithIndependentDP(t *testing.T) {
	items := loadCorpus(t)
	dict := Dictionary()

	var spans, tileable, untileable, disagreements int
	for _, it := range items {
		runes := []rune(it.Text)
		for _, sp := range scriptSpans(runes) {
			if !sp.thai {
				continue
			}
			span := runes[sp.start:sp.end]
			spans++
			want := isFullySegmentable(dict, span)
			if want {
				tileable++
			} else {
				untileable++
			}
			fwd := tileableForward(dict, span)
			bwd := tileableBackward(dict, span)
			if fwd[len(span)] != want {
				disagreements++
				t.Errorf("tileableForward disagrees with the independent DP: item=%s span=%q forward=%v DP=%v", it.ID, string(span), fwd[len(span)], want)
			}
			if bwd[0] != want {
				disagreements++
				t.Errorf("tileableBackward disagrees with the independent DP: item=%s span=%q backward=%v DP=%v", it.ID, string(span), bwd[0], want)
			}
		}
	}

	if spans == 0 {
		t.Fatal("vacuity: the corpus contains no Thai script spans")
	}
	if tileable == 0 {
		t.Fatal("vacuity: no span in the corpus is tileable — the positive polarity is untested")
	}
	if untileable == 0 {
		t.Fatal("vacuity: no span in the corpus is untileable — the negative polarity, which is the one P2 governs, is untested")
	}
	if disagreements == 0 {
		t.Logf("tileableForward/tileableBackward agree with the independent DP on all %d Thai spans (%d tileable, %d untileable)", spans, tileable, untileable)
	}
}

// TestFilterOnlyRemovesBreakOpportunities is the filter's defining
// property, and it is the one that makes the P2 argument sound: if the
// filter could ADD a position, "every surviving break has both sides
// tileable" would no longer imply "no break inside an untileable span".
//
// This is also the guard against the measured-and-rejected permissive
// variant (749 breaks against 558, 141 of 243 items changed), which
// proposes a break at every doubly-tileable position rather than
// filtering the matcher's own proposals. That variant would fail this
// test on its first corpus item.
//
// Presence precondition: the test asserts that the filter actually
// removed something somewhere in the corpus, so an implementation that
// returned its input unchanged cannot pass by doing nothing.
func TestFilterOnlyRemovesBreakOpportunities(t *testing.T) {
	items := loadCorpus(t)
	dict := Dictionary()

	var checked, removed int
	for _, it := range items {
		runes := []rune(it.Text)
		for _, sp := range scriptSpans(runes) {
			if !sp.thai {
				continue
			}
			proposals, _ := segmentThaiSpan(dict, runes, sp.start, sp.end, ClusterBoundaries(runes), false)
			kept := filterBothSidesCoverable(dict, runes, sp.start, sp.end, append([]int(nil), proposals...))
			checked += len(proposals)
			removed += len(proposals) - len(kept)

			if len(kept) > len(proposals) {
				t.Errorf("filter ADDED positions: item=%s span=%q proposals=%v kept=%v", it.ID, string(runes[sp.start:sp.end]), proposals, kept)
			}
			in := map[int]bool{}
			for _, p := range proposals {
				in[p] = true
			}
			for _, k := range kept {
				if !in[k] {
					t.Errorf("filter INVENTED position %d: item=%s span=%q proposals=%v kept=%v", k, it.ID, string(runes[sp.start:sp.end]), proposals, kept)
				}
			}
		}
	}

	if checked == 0 {
		t.Fatal("vacuity: the matcher proposed no interior Thai breaks anywhere in the corpus")
	}
	if removed == 0 {
		t.Fatal("vacuity: the filter removed nothing across the whole corpus — an identity function would pass this test, so it is not asserting the filter runs")
	}
	t.Logf("filter is subtractive over the whole corpus: %d interior Thai proposals examined, %d withdrawn, 0 added", checked, removed)
}
