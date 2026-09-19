package text

// RunKind labels what produced an atomic span in a break computation,
// so the spike report and P6 harness can report measured counts by
// category (D-000.14) rather than narrate them.
type RunKind int

const (
	// RunWord is a span the dictionary matched as a complete entry.
	RunWord RunKind = iota
	// RunUnknownThai is a maximal span of Thai text the dictionary
	// could not cover, in constrained mode — AD-25's atomic-unknown-run
	// absolute (AC6, P2). It carries no interior break opportunities.
	RunUnknownThai
	// RunNonThai is a maximal span of non-Thai content (Latin letters,
	// digits, punctuation, whitespace-separated tokens). AD-25 is
	// Thai-specific; this package treats a non-Thai span as atomic by
	// construction rather than attempting word segmentation it has no
	// dictionary for (P6d: mixed Thai/Latin/digit content).
	RunNonThai
)

// Run is one span of the decomposition ComputeBreaks reports, in rune
// indices [Start, End).
type Run struct {
	Start, End int
	Kind       RunKind
}

// scriptSpan is a maximal run of the same script classification.
type scriptSpan struct {
	start, end int
	thai       bool
}

func scriptSpans(runes []rune) []scriptSpan {
	if len(runes) == 0 {
		return nil
	}
	var spans []scriptSpan
	start := 0
	curThai := isThaiScript(runes[0])
	for i := 1; i < len(runes); i++ {
		thai := isThaiScript(runes[i])
		if thai != curThai {
			spans = append(spans, scriptSpan{start, i, curThai})
			start = i
			curThai = thai
		}
	}
	spans = append(spans, scriptSpan{start, len(runes), curThai})
	return spans
}

// ComputeBreaks computes AD-25's break opportunities over s using dict.
//
// In constrained mode (unconstrained == false, the engine's real
// behaviour), both of AD-25's absolutes apply: no break is ever
// proposed inside a Thai character cluster (P1), and a run of Thai text
// the dictionary cannot cover — even partially, via cluster-boundary-
// aligned substrings — is atomic, contributing no interior break
// (P2/AC6).
//
// In unconstrained mode (unconstrained == true), BOTH absolutes are
// switched off: matching may end at any rune position, not just a
// cluster boundary, and an unmatched fragment is consumed one rune at a
// time rather than merged into a single atomic span. This reproduces
// AD-25's Prevents line verbatim — "a greedy dictionary matcher
// shredding a word it does not recognise into legal-but-wrong
// fragments and breaking a line inside it" — and exists ONLY to compute
// P6f/P6g (D-2.1.4): the count of hand-identified personal names for
// which this naive matcher proposes at least one interior break, which
// the constrained engine must then refuse (P3).
//
// Returned breaks are the sorted rune-index positions, strictly
// between 0 and len(runes), where an interior break opportunity is
// proposed.
//
// runs is the GREEDY DICTIONARY DECOMPOSITION, for reporting — and in
// constrained mode it is deliberately NOT the same thing as "the gaps
// between breaks". Story 2.4's both-sides-coverable filter (tileable.go)
// withdraws proposals the greedy matcher made inside spans that cannot
// be tiled at all, so a boundary between two adjacent RunWord runs may
// carry no break opportunity. Reconstructing breaks from runs would
// therefore give the PRE-FIX answer. Callers that want break positions
// must read breaks; runs answers "what did the dictionary match", which
// is what the P6 exercise floors count (corpus_test.go's computeP6Stats)
// and why filtering it here would silently move a disclosed measurement.
//
// The filter never adds a position, so every property asserted over
// breaks in the withdrawing direction — P1's cluster absolute, P2's
// atomic-unknown-run absolute — holds at least as strongly after it as
// before.
func ComputeBreaks(dict *BytesTrie, s string, unconstrained bool) (breaks []int, runs []Run) {
	runes := []rune(s)
	if len(runes) == 0 {
		return nil, nil
	}

	spans := scriptSpans(runes)
	var clusterOK []bool
	if !unconstrained {
		clusterOK = ClusterBoundaries(runes)
	}

	for si, span := range spans {
		if !span.thai {
			runs = append(runs, Run{span.start, span.end, RunNonThai})
		} else {
			segBreaks, segRuns := segmentThaiSpan(dict, runes, span.start, span.end, clusterOK, unconstrained)
			if !unconstrained {
				// Story 2.4's both-sides-coverable filter (tileable.go):
				// the greedy matcher's notion of an uncoverable run is
				// order-dependent, so it proposes breaks inside spans
				// that an order-independent measurement says cannot be
				// tiled at all. Withdraw those. Constrained mode only:
				// unconstrained mode exists to reproduce AD-25's
				// Prevents line verbatim for P6f/P6g, and filtering it
				// would make it stop reproducing the very behaviour it
				// is there to measure.
				segBreaks = filterBothSidesCoverable(dict, runes, span.start, span.end, segBreaks)
			}
			breaks = append(breaks, segBreaks...)
			runs = append(runs, segRuns...)
		}
		// A boundary between two script spans (e.g. Thai text followed
		// by a space or a Latin/digit run) is always a legitimate break
		// opportunity, and is interior to s whenever it is not the very
		// end of the string.
		if si < len(spans)-1 {
			breaks = append(breaks, span.end)
		}
	}
	return breaks, runs
}

// segmentThaiSpan greedily longest-match segments runes[start:end]
// against dict. When clusterOK is non-nil (constrained mode), matches
// must end on a cluster boundary, and an unmatched stretch is merged
// forward into one atomic RunUnknownThai up to the next position where
// matching resumes (or to end, if it never does). When clusterOK is
// nil (unconstrained mode), matches may end anywhere, and an unmatched
// rune is consumed singly (RunUnknownThai of length 1), reproducing a
// naive greedy matcher's shredding behaviour.
func segmentThaiSpan(dict *BytesTrie, runes []rune, start, end int, clusterOK []bool, unconstrained bool) (breaks []int, runs []Run) {
	p := start
	for p < end {
		matchEnd, ok := longestDictMatch(dict, runes, p, end, clusterOK, unconstrained)
		if ok {
			runs = append(runs, Run{p, matchEnd, RunWord})
			if matchEnd < end {
				breaks = append(breaks, matchEnd)
			}
			p = matchEnd
			continue
		}

		if unconstrained {
			// A naive greedy matcher that finds no legal dictionary
			// fragment at all here proposes nothing — it does not
			// invent a break around a character it cannot even
			// partially recognise. This is deliberate, and it is what
			// makes P6f/P6g measure the right thing (D-2.1.4): a
			// compound of two real, legal dictionary words gets
			// shredded AT THEIR BOUNDARY (AD-25's Prevents line,
			// verbatim: "into legal-but-wrong fragments") — that is
			// P6f. A name with NO legal fragment anywhere gets no
			// break proposed anywhere ("nothing proposed, nothing to
			// override") — that is P6g's opposite polarity.
			next := p + 1
			runs = append(runs, Run{p, next, RunUnknownThai})
			p = next
			continue
		}

		// Constrained: find the next cluster-boundary position, after
		// p, at which SOME dictionary word begins — that is where
		// matching resumes, and the entire span up to there is one
		// atomic unknown run (P2, AC6): no interior break within it.
		resume := end
		for q := p + 1; q < end; q++ {
			if !clusterOK[q] {
				continue
			}
			if _, ok := longestDictMatch(dict, runes, q, end, clusterOK, unconstrained); ok {
				resume = q
				break
			}
		}
		runs = append(runs, Run{p, resume, RunUnknownThai})
		if resume < end {
			breaks = append(breaks, resume)
		}
		p = resume
	}
	return breaks, runs
}

// longestDictMatch finds the greedy-longest dictionary word starting at
// rune index p, ending no later than end. In constrained mode the
// candidate end position must be a cluster boundary; in unconstrained
// mode any end position is a candidate.
func longestDictMatch(dict *BytesTrie, runes []rune, p, end int, clusterOK []bool, unconstrained bool) (matchEnd int, ok bool) {
	for e := end; e > p; e-- {
		if !unconstrained && !clusterOK[e] {
			continue
		}
		if dict.Contains(string(runes[p:e])) {
			return e, true
		}
	}
	return 0, false
}
