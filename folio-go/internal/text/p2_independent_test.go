package text

import (
	"sort"
	"testing"
)

// isFullySegmentable is an INDEPENDENT ground-truth measurement of
// dictionary coverage (D-000.9: a second, independently-derived
// measurement, never the engine's own classification re-examined). It
// answers "does ANY sequence of splits exist that covers the whole
// span with complete dictionary words?" via a standard word-break
// dynamic program — exhaustive over every split point, ignoring both
// of AD-25's constraints (cluster boundaries, greedy-longest-match
// order) entirely, because this is a pure DICTIONARY-COVERAGE
// question, not an engine-behaviour question. This is deliberately NOT
// built by calling ComputeBreaks or reusing segmentThaiSpan/
// longestDictMatch — reusing the engine's own matching logic here
// would make this "cross-check" just re-read the engine's own
// classification back to itself (exactly the self-referential shape
// the reopening's message named: "a PASS we know is wrong").
func isFullySegmentable(dict *BytesTrie, runes []rune) bool {
	n := len(runes)
	if n == 0 {
		return true
	}
	reachable := make([]bool, n+1)
	reachable[0] = true
	for i := 0; i < n; i++ {
		if !reachable[i] {
			continue
		}
		for j := i + 1; j <= n; j++ {
			if dict.Contains(string(runes[i:j])) {
				reachable[j] = true
			}
		}
	}
	return reachable[n]
}

// p2Violation records one position where the CONSTRAINED engine
// proposes a break strictly inside a Thai-script span that the
// independent DP method (isFullySegmentable) says has NO valid
// dictionary segmentation at all — i.e., a span AD-25's atomic-
// unknown-run rule requires be entirely break-free, per ground truth
// independent of what the engine itself decided.
type p2Violation struct {
	ItemID   string
	Text     string
	Span     [2]int
	SpanText string
	BreakPos int
}

// TestP2IndependentDPCrossCheck is P2's reopened measurement method
// (the coordinator's D-000.17/18-driven correction): the ORIGINAL
// TestP2NeverBreaksInsideUnknownRun only checked the engine's own
// RunUnknownThai spans against the engine's own breaks list — which is
// self-referential and can never find a violation by construction
// (the engine, by definition, never places a break inside a span IT
// decided was unknown). This test instead computes ground truth
// independently (isFullySegmentable, a DP with no cluster constraint
// and no greedy-match order dependency) over every Thai-script span in
// the corpus, and asks: does the engine ever break inside a span ground
// truth says has NO valid segmentation at all?
//
// This is asserted as a FAILURE when found (t.Errorf, not t.Log) — per
// the reopening's explicit instruction: "Expect P2 to go RED and let
// it... recording real violations is more honest than a PASS we know
// is wrong." A future story may need a third mechanism (paralleling
// P3's finding) to actually close this; this test's job is only to
// measure and report, honestly, not to be tuned until it passes.
func TestP2IndependentDPCrossCheck(t *testing.T) {
	items := loadCorpus(t)
	dict := Dictionary()

	var violations []p2Violation
	itemsViolated := map[string]bool{}

	for _, it := range items {
		runes := []rune(it.Text)
		breaks, _ := ComputeBreaks(dict, it.Text, false)

		for _, span := range scriptSpans(runes) {
			if !span.thai {
				continue
			}
			spanRunes := runes[span.start:span.end]
			if isFullySegmentable(dict, spanRunes) {
				continue // ground truth: this span IS coverable — not what P2 governs
			}
			// Ground truth says this whole span has NO valid
			// dictionary segmentation — AD-25's atomic-unknown-run
			// rule requires zero interior breaks anywhere inside it.
			for _, b := range breaks {
				if b > span.start && b < span.end {
					violations = append(violations, p2Violation{
						ItemID:   it.ID,
						Text:     it.Text,
						Span:     [2]int{span.start, span.end},
						SpanText: string(spanRunes),
						BreakPos: b,
					})
					itemsViolated[it.ID] = true
				}
			}
		}
	}

	sort.Slice(violations, func(i, j int) bool { return violations[i].ItemID < violations[j].ItemID })

	t.Logf("P2 INDEPENDENT MEASUREMENT: %d violations across %d items (ground truth: isFullySegmentable, a DP independent of this engine's own matching order and cluster constraints)", len(violations), len(itemsViolated))
	for _, v := range violations {
		t.Logf("  P2 violation: item=%s text=%q span=%v span-text=%q break-at-rune=%d", v.ItemID, v.Text, v.Span, v.SpanText, v.BreakPos)
	}

	if len(violations) > 0 {
		t.Errorf("P2 FAILS under independent measurement: %d violations across %d items — reported, not suppressed (D-000.17/18: a floor or absolute that fails is reported as failing, never tuned to pass)", len(violations), len(itemsViolated))
	}
}
