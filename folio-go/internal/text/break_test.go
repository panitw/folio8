package text

import "testing"

// TestComputeBreaksKnownWords is a sanity baseline: a run of two known,
// unrelated dictionary words yields a break at the boundary between
// them (not vacuously "no breaks anywhere").
func TestComputeBreaksKnownWords(t *testing.T) {
	d := Dictionary()
	// "ประเทศ" (country) + "ไทย" (Thailand) - two well-known dictionary words.
	breaks, runs := ComputeBreaks(d, "ประเทศไทย", false)
	if len(breaks) == 0 {
		t.Fatal("expected at least one break opportunity between two concatenated known words")
	}
	if len(runs) == 0 {
		t.Fatal("expected a non-empty run decomposition")
	}
}

// TestComputeBreaksNoBreakInsideCluster is AC7/P1 exercised through the
// full pipeline, not just ClusterBoundaries in isolation.
//
// THIS TEST WAS VACUOUS AND IS NOW NOT. Its subject was "เก็บ", which is
// itself a single dictionary entry: the greedy matcher swallows it whole
// and proposes NO interior break, so the loop asserting "no break at 1
// or 2" never executed a single iteration and the test passed by doing
// nothing. Measured at 0266a86, BEFORE Story 2.4's filter existed —
// so this is a pre-existing weakness found by auditing the guards the
// filter touches, not one the filter introduced (the same audit is why
// V11's sample moved; see below).
//
// The subject is now "เก็บเงิน" ("save money"), two dictionary entries,
// measured to carry a real break at rune 4 and to have FOUR interior
// positions that are not cluster boundaries — 1 and 2 inside "เก็",
// 5 and 6 inside "เงิ". So there is something to propose and something
// forbidden to propose it at.
//
// The assertion is against the FORBIDDEN SET computed from
// ClusterBoundaries, not against hand-written indices: it covers every
// non-boundary position by construction rather than the two a reader
// happened to notice (D-000.23).
func TestComputeBreaksNoBreakInsideCluster(t *testing.T) {
	d := Dictionary()
	const subject = "เก็บเงิน" // เ-ก-็-บ-เ-ง-ิ-น: two clusters carrying above/below marks
	runes := []rune(subject)
	breaks, _ := ComputeBreaks(d, subject, false)

	boundary := ClusterBoundaries(runes)
	var forbidden []int
	for i := 1; i < len(runes); i++ {
		if !boundary[i] {
			forbidden = append(forbidden, i)
		}
	}

	// Vacuity guards, both polarities. Without the first, an engine
	// returning no breaks at all passes; without the second, a subject
	// with no interior clusters passes.
	if len(breaks) == 0 {
		t.Fatalf("vacuity: %q proposes no break at all, so the cluster assertion below would iterate zero times and assert nothing", subject)
	}
	if len(forbidden) == 0 {
		t.Fatalf("vacuity: %q has no interior position that is inside a cluster, so there is nothing this test could catch", subject)
	}

	forbiddenSet := map[int]bool{}
	for _, f := range forbidden {
		forbiddenSet[f] = true
	}
	for _, b := range breaks {
		if forbiddenSet[b] {
			t.Errorf("break reported at rune %d, strictly inside a Thai character cluster in %q (forbidden positions: %v, runes=%q)", b, subject, forbidden, runes)
		}
	}
	t.Logf("AC7/P1: %q proposes %v; all %d non-cluster-boundary positions %v are refused", subject, breaks, len(forbidden), forbidden)
}

// TestComputeBreaksAtomicUnknownRun is AC6/P2: a nonsense Thai string
// with no dictionary coverage at all must yield NO interior breaks and
// must be reported as a single RunUnknownThai span.
func TestComputeBreaksAtomicUnknownRun(t *testing.T) {
	d := Dictionary()
	// "ฅงฌฎ": rare/obsolete Thai consonants strung together. Verified
	// (probe, this story's dev record) to contain no dictionary match
	// at any substring length, including single characters — unlike ฆ
	// and ฏ, which the wordlist itself lists as standalone entries.
	nonsense := "ฅงฌฎ"
	breaks, runs := ComputeBreaks(d, nonsense, false)
	if len(breaks) != 0 {
		t.Errorf("expected zero interior breaks in an uncoverable run, got %v", breaks)
	}
	if len(runs) != 1 || runs[0].Kind != RunUnknownThai {
		t.Errorf("expected a single RunUnknownThai span, got %+v", runs)
	}
}

// TestComputeBreaksMixedScript is P6d's sanity check: mixed Thai/Latin
// content produces a break at the script boundary.
func TestComputeBreaksMixedScript(t *testing.T) {
	d := Dictionary()
	breaks, runs := ComputeBreaks(d, "ประเทศABC123", false)
	if len(breaks) == 0 {
		t.Fatal("expected a break at the Thai/Latin script boundary")
	}
	var sawNonThai bool
	for _, r := range runs {
		if r.Kind == RunNonThai {
			sawNonThai = true
		}
	}
	if !sawNonThai {
		t.Error("expected at least one RunNonThai span for the Latin/digit tail")
	}
}

// TestUnconstrainedVsConstrainedSwitchActuallyToggles is V11's guard:
// the unconstrained/constrained switch must provably change behaviour
// on at least one real input, or P6f/P6g measure nothing.
func TestUnconstrainedVsConstrainedSwitchActuallyToggles(t *testing.T) {
	d := Dictionary()
	// "ชัยวัฒน์": a Thai given name, the uncoverable [0,8) span of
	// corpus items name-021 and name-081. Measured (Story 2.4's dev
	// record) at unconstrained=[3 7] against constrained=[], on a
	// SINGLE Thai script span with no space in it — so the difference
	// is attributable to the mode switch itself and not to a
	// script-boundary break both modes would emit anyway.
	//
	// It is also the clearest available illustration of what the switch
	// toggles: unconstrained is AD-25's Prevents line running live —
	// "a greedy dictionary matcher shredding a word it does not
	// recognise into legal-but-wrong fragments" — and constrained is
	// the engine refusing.
	//
	// WHY THE SAMPLE MOVED, AND WHY THAT IS NOT A REGRESSION. The
	// previous sample "ดอเลาะ" discriminated for the opposite reason:
	// unconstrained proposed nothing, while the CONSTRAINED engine's
	// atomic-run resumption scan found a short spurious legal match
	// partway through and proposed an interior break there. That
	// spurious break was the P2 defect, and Story 2.4's
	// both-sides-coverable filter withdrew it — so both modes now
	// return [] for "ดอเลาะ" and it discriminates nothing. The fix
	// consumed this guard's discriminating input, exactly as D-000.30
	// describes: closing a defect destroys the evidence that depended
	// on it. The response is a NEW measured discriminating input, not a
	// weakened assertion.
	//
	// (The sample before that, "บ้านรถ", measured IDENTICALLY under
	// both modes and never red-proved anything at all — a compound of
	// two cleanly-recognised morphemes at a clean cluster boundary
	// gives greedy matching no reason to behave differently.)
	compound := "ชัยวัฒน์"

	unconBreaks, _ := ComputeBreaks(d, compound, true)
	conBreaks, _ := ComputeBreaks(d, compound, false)

	// V11's actual requirement (the reopening's finding: a version of
	// this test asserting only "unconstrained found >=1 break" passes
	// even when `unconstrained` is hard-coded to false at the top of
	// ComputeBreaks — it never compared the two modes at all). This
	// MUST assert the two RESULTS actually differ for this input, not
	// merely that one of them is non-empty.
	sameLen := len(unconBreaks) == len(conBreaks)
	same := sameLen
	if sameLen {
		for i := range unconBreaks {
			if unconBreaks[i] != conBreaks[i] {
				same = false
				break
			}
		}
	}
	if same {
		t.Fatalf("V11 VIOLATION: unconstrained and constrained produced IDENTICAL break sets (%v) for %q — the switch is not proven to toggle anything; injecting `unconstrained = false` at the top of ComputeBreaks must make this test fail, and it would not if this assertion were absent", unconBreaks, compound)
	}

	t.Logf("compound=%q unconstrained breaks=%v constrained breaks=%v (differ, as required)", compound, unconBreaks, conBreaks)
}
