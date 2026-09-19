package text

// This file implements Story 2.4's both-sides-coverable filter — the
// engine fix D-2.1.9 routed to this story by name, closing the P2
// defect the Story 2.1 spike left deliberately visible.
//
// THE DEFECT. AD-25's second absolute says a run of Thai the dictionary
// cannot cover yields no interior break. break.go's greedy matcher
// implements that against its OWN notion of an uncoverable run: a
// stretch where greedy-longest matching happens to stall. But greedy
// order is not coverage. A span the dictionary cannot tile at all can
// still contain positions where greedy matched something on the way in,
// and the matcher then proposes a break there — inside a span that,
// by an order-independent measurement, has no valid segmentation
// anywhere. Measured at 0266a86 over the committed 243-item corpus:
// 26 such breaks across 17 items. The atomicity promise dissolved
// under context.
//
// THE FIX. A break opportunity strictly interior to a Thai-script span
// survives only if the span text BEFORE it and the span text AFTER it
// are each fully tileable by dictionary words. It only ever REMOVES
// proposals the greedy matcher already made; it never adds one.
//
// WHY THIS CLOSES P2 BY CONSTRUCTION, NOT BY TUNING. Suppose some
// interior position p in span S survived the filter. Then S[:p] has a
// tiling and S[p:] has a tiling; concatenating them tiles S. So S is
// tileable — contradicting the premise that P2 governs S at all. The
// filter therefore admits no interior break inside an untileable span,
// over ANY input and ANY dictionary, not merely over this corpus. The
// corpus measurement (0 violations, from 26) confirms the
// implementation matches that argument; it is not the argument.
//
// WHY IT IS A FILTER AND NOT A RE-PROPOSER. The variant that proposes a
// break at EVERY doubly-tileable position was measured too: 749 breaks
// against 558, 141 of 243 items changed, and it makes P3 (breaks inside
// a hand-labelled proper noun) worse, because it invents positions the
// dictionary matcher never proposed. Rejected on those numbers.
//
// WHAT THIS IS NOT. It is not a segmentation heuristic, and none may be
// added here. D-2.1.6 (the owner's ruling) settles that: Thai surnames
// are coined by law from ordinary dictionary words — 83% of the
// corpus's personal names decompose into entries the dictionary already
// holds — so no dictionary-coverage rule can distinguish a proper noun
// from its parts. Unbreakability is DECLARED by the template, never
// inferred. This filter makes no guess about meaning; it only refuses
// to break where the dictionary provably cannot account for both sides.

// tileableForward reports, for each rune offset i in [0, len(span)],
// whether span[:i] is exactly covered by a concatenation of dictionary
// entries. Index 0 is vacuously true (the empty prefix).
//
// It is the same question the independent DP ground truth in
// p2_independent_test.go asks (isFullySegmentable), computed here for
// every prefix rather than only for the whole span. That test remains
// independently derived — it never calls this function — which is what
// keeps it a cross-check rather than the engine reading its own answer
// back (D-000.9).
func tileableForward(dict *BytesTrie, span []rune) []bool {
	n := len(span)
	reach := make([]bool, n+1)
	reach[0] = true
	if n == 0 {
		return reach
	}
	enc, byteAt, runeAt := encodeSpan(span)
	for i := 0; i < n; i++ {
		if !reach[i] {
			continue
		}
		start := byteAt[i]
		dict.forEachWordPrefix(enc[start:], func(byteLen int) {
			reach[runeAt[start+byteLen]] = true
		})
	}
	return reach
}

// tileableBackward reports, for each rune offset i in [0, len(span)],
// whether span[i:] is exactly covered by a concatenation of dictionary
// entries. Index len(span) is vacuously true (the empty suffix).
func tileableBackward(dict *BytesTrie, span []rune) []bool {
	n := len(span)
	reach := make([]bool, n+1)
	reach[n] = true
	if n == 0 {
		return reach
	}
	enc, byteAt, runeAt := encodeSpan(span)
	// Descending, so every j > i this loop consults is already final.
	for i := n - 1; i >= 0; i-- {
		start := byteAt[i]
		dict.forEachWordPrefix(enc[start:], func(byteLen int) {
			if reach[runeAt[start+byteLen]] {
				reach[i] = true
			}
		})
	}
	return reach
}

// encodeSpan returns span's UTF-8 encoding together with the two index
// maps the tileability walks need: byteAt[i] is the byte offset at which
// rune i begins (byteAt[len(span)] is len(enc)), and runeAt[b] is the
// rune index at byte offset b for every b that begins a rune (and
// len(span) at b == len(enc)).
//
// Both directions are needed because the dictionary is byte-oriented
// (BytesTrie walks UTF-8) while AD-25's break positions, the corpus's
// proper-noun spans and GlyphInfo.Cluster are all RUNE indices
// (D-2.3.2). Converting once here keeps that conversion in one place
// instead of at every call site.
func encodeSpan(span []rune) (enc []byte, byteAt []int, runeAt []int) {
	enc = []byte(string(span))
	byteAt = make([]int, len(span)+1)
	runeAt = make([]int, len(enc)+1)
	b := 0
	for i, r := range span {
		byteAt[i] = b
		runeAt[b] = i
		b += len(string(r))
	}
	byteAt[len(span)] = len(enc)
	runeAt[len(enc)] = len(span)
	return enc, byteAt, runeAt
}

// filterBothSidesCoverable returns the subset of proposals that survive
// the both-sides-coverable rule within the Thai span runes[start:end].
// proposals must be rune indices strictly interior to that span, which
// is what segmentThaiSpan produces.
//
// The returned slice preserves the input order and never contains a
// position the input did not.
func filterBothSidesCoverable(dict *BytesTrie, runes []rune, start, end int, proposals []int) []int {
	if len(proposals) == 0 {
		return proposals
	}
	span := runes[start:end]
	fwd := tileableForward(dict, span)
	bwd := tileableBackward(dict, span)
	kept := proposals[:0:0]
	for _, p := range proposals {
		off := p - start
		if fwd[off] && bwd[off] {
			kept = append(kept, p)
		}
	}
	return kept
}
