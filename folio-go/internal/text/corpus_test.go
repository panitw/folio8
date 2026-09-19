package text

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// corpusItem mirrors cmd/gencorpus's CorpusItem — kept independent
// (not shared via an import) since the corpus lives in repo-root
// fixtures/ (Trap 3) and this test reads it as data, the same way any
// consumer of the committed fixture would.
type corpusItem struct {
	ID              string   `json:"id"`
	Category        string   `json:"category"`
	Provenance      string   `json:"provenance"`
	Text            string   `json:"text"`
	ProperNounSpans [][2]int `json:"properNounSpans,omitempty"`
}

// sourced reports whether it counts toward a GENUINE floor (P5, P6e,
// P6f, P6g) — D-000.17: a floor that counts genuine items must exclude
// synthetic exercise tokens, which exist ONLY for P6a's value.
func (it corpusItem) sourced() bool {
	return it.Provenance == "sourced"
}

func loadCorpus(t *testing.T) []corpusItem {
	t.Helper()
	root := repoRootFromTest(t)
	path := filepath.Join(root, "fixtures", "thai-break-corpus", "corpus.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}
	var items []corpusItem
	if err := json.Unmarshal(b, &items); err != nil {
		t.Fatalf("unmarshal corpus: %v", err)
	}
	return items
}

func isToneMark(r rune) bool {
	return r >= 0x0E48 && r <= 0x0E4B
}

// isAboveSign reports whether r is a combining mark that renders in
// the "above" visual slot (the slot a tone mark also occupies, hence
// the "stacked" shape P6c measures): mai han-akat, the four above
// vowel signs, mai taikhu, thanthakhat, nikhahit, yamakkan.
func isAboveSign(r rune) bool {
	switch r {
	case 0x0E31, 0x0E34, 0x0E35, 0x0E36, 0x0E37, 0x0E47, 0x0E4C, 0x0E4D, 0x0E4E:
		return true
	}
	return false
}

// isAnyVowel reports whether r is any Thai vowel sign — leading,
// following, or a non-tone combining mark (above or below).
func isAnyVowel(r rune) bool {
	cls := classify(r)
	if cls == classThaiLeadingVowel || cls == classThaiFollowingVowel {
		return true
	}
	if cls == classThaiCombining && !isToneMark(r) {
		return true
	}
	return false
}

// clusterSpans groups runes into Thai-character-cluster spans using
// ClusterBoundaries — the same boundaries the break engine uses (P1),
// so P6b/P6c are measured against the identical cluster definition
// AC7 asserts, not a second, independently-invented one.
func clusterSpans(runes []rune) [][2]int {
	b := ClusterBoundaries(runes)
	var spans [][2]int
	start := 0
	for i := 1; i < len(b); i++ {
		if b[i] {
			spans = append(spans, [2]int{start, i})
			start = i
		}
	}
	return spans
}

// p6Stats holds every measured P6 exercise-floor count (D-2.1.4),
// computed from the corpus actually read, never narrated (D-000.14).
type p6Stats struct {
	P6a int // items with >=1 uncoverable run (constrained RunUnknownThai)
	P6b int // items with a cluster carrying both a vowel and a tone mark
	P6c int // ...of which, stacked vowel+tone on one base (above-slot)
	P6d int // items mixing Thai with Latin/digits
	P6e int // hand-identified proper nouns (personal_name + place_name items)
	P6f int // personal names: unconstrained matcher proposes >=1 interior break on the surname
	P6g int // personal names: unconstrained matcher proposes NONE
}

// p3Violation records one measured P3 failure: a break the CONSTRAINED
// engine proposes strictly inside a hand-identified proper-noun span.
type p3Violation struct {
	ItemID   string
	Text     string
	BreakPos int
	Span     [2]int
}

// TestCorpusMeetsP5Floors is P5's measurement, over SOURCED items only
// (D-000.17, the reopening): a "synthetic_probe" item is never a real
// personal name, place name, or transaction description, and counting
// it toward P5 would be exactly the sampling-floor-filling this
// story's reopening forbids. A floor that is not met is reported as
// unmet (t.Errorf), never silently passed.
func TestCorpusMeetsP5Floors(t *testing.T) {
	items := loadCorpus(t)

	var names, places, txns, synthetic int
	for _, it := range items {
		switch it.Category {
		case "personal_name":
			if !it.sourced() {
				t.Fatalf("item %s is category personal_name but Provenance=%q — every personal_name item must be sourced", it.ID, it.Provenance)
			}
			names++
		case "place_name":
			if !it.sourced() {
				t.Fatalf("item %s is category place_name but Provenance=%q", it.ID, it.Provenance)
			}
			places++
		case "transaction_description":
			if !it.sourced() {
				t.Fatalf("item %s is category transaction_description but Provenance=%q", it.ID, it.Provenance)
			}
			txns++
		case "synthetic_probe":
			if it.sourced() {
				t.Fatalf("item %s is category synthetic_probe but Provenance=%q — synthetic items must never claim to be sourced", it.ID, it.Provenance)
			}
			synthetic++
		default:
			t.Fatalf("item %s has unknown category %q", it.ID, it.Category)
		}
	}

	if names < 120 {
		t.Errorf("P5 personal-name floor not met (sourced only): got %d, need >=120", names)
	}
	if places < 40 {
		t.Errorf("P5 place-name floor not met: got %d, need >=40", places)
	}
	if txns < 40 {
		t.Errorf("P5 transaction-description floor not met: got %d, need >=40", txns)
	}
	total := names + places + txns
	if total < 200 {
		t.Errorf("P5 total-corpus floor not met (sourced only): got %d, need >=200", total)
	}
	t.Logf("P5 (sourced only): personal_name=%d place_name=%d transaction_description=%d total=%d | synthetic_probe=%d (excluded)", names, places, txns, total, synthetic)
}

// TestCorpusMeetsP6ExerciseFloors is AC4's exercise-floor half (D-2.1.4),
// and V3/V4/V5/V11's anti-vacuity checks: every count is computed from
// the corpus actually read, and P6a=0 (or any floor unmet) fails this
// test exactly as a P5 shortfall would (V3).
func TestCorpusMeetsP6ExerciseFloors(t *testing.T) {
	items := loadCorpus(t)
	dict := Dictionary()
	stats := computeP6Stats(t, items, dict)

	checks := []struct {
		name string
		got  int
		want int
	}{
		{"P6a (uncoverable-run items)", stats.P6a, 60},
		{"P6b (vowel+tone cluster items)", stats.P6b, 30},
		{"P6c (stacked vowel+tone items)", stats.P6c, 10},
		{"P6d (mixed-script items)", stats.P6d, 20},
		{"P6e (proper nouns)", stats.P6e, 160},
		{"P6f (decomposable names)", stats.P6f, 90},
		{"P6g (opaque names)", stats.P6g, 20},
	}
	// D-000.74 change (1): each floor gets its OWN verdict. All seven
	// still evaluate — they always did, one t.Errorf each with no
	// short-circuit — but P6a through P6f were green facts reported
	// under a red parent name, which is D-000.70's masking shape one
	// level below CI. Nothing about D-000.17 or D-2.1.14 moves here: the
	// floor is still reported unmet, and now reported more precisely.
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if c.got < c.want {
				t.Errorf("%s floor not met: got %d, need >=%d", c.name, c.got, c.want)
			}
		})
	}
	t.Logf("P6 stats: %+v", stats)
}

// TestCorpusP6StatsMatchDeclaredBaseline executes D-000.57's third
// clause instead of narrating it (D-000.74 change (2)).
//
// D-000.57 lets an epic gate close over a red, but ONLY over one whose
// "reported numbers are byte-identical to the declared baseline", and it
// says in its own text why: "the test name stays red while what it
// measures silently drifts — P6g:7 quietly becoming P6g:3 would still
// present as 'the same known failure', and a real regression would ride
// in under the cover of a sanctioned one." That clause has been checked
// by a human reading a log line once per epic. This test checks it on
// every run.
//
// THIS TEST IS GREEN, and it lives in the CI job that must stay green.
// TestCorpusMeetsP6ExerciseFloors above is red by design (D-000.17,
// D-2.1.14) and is quarantined into its own job; the drift detector is
// deliberately NOT quarantined with it, because a green job with no
// drift detector in it would stay green through exactly the P6g:7 -> 3
// slide D-000.57 describes.
//
// ANCHOR (D-000.68): a literal this test owns. Pinned rather than stated
// relationally, deliberately — the corpus is S4, "consulted for the life
// of the project" (D-000.17), so the set is frozen by design and growth
// SHOULD require editing a test that says so. D-3.1a.3 is relational
// because its set is expected to move; this one is not.
//
// NON-VACUOUS BY CONSTRUCTION: a computeP6Stats that stopped computing
// returns the zero struct, and 0 != 64 fails. There is no state in which
// this test passes without the corpus having actually been read and
// measured — the property D-000.9 usually costs a separate guard.
//
// IT REDDENS ON IMPROVEMENT TOO, AND THAT IS CORRECT. D-2.1.14 says that
// if more genuinely-opaque names are sourced "they are added" — that
// must be a deliberate, reviewable baseline edit with a gate-document
// note, never a number that slides. An unexplained delta between two
// computations is where a calibration hides (D-000.19).
//
// The literals below were transcribed from D-000.57's recorded stats and
// then verified against a live run at HEAD ba24e52; both agree. If a
// future reader finds them disagreeing with either source, that
// disagreement is the finding.
func TestCorpusP6StatsMatchDeclaredBaseline(t *testing.T) {
	const (
		baselineP6a = 64
		baselineP6b = 63
		baselineP6c = 16
		baselineP6d = 20
		baselineP6e = 284
		baselineP6f = 115
		baselineP6g = 7
	)

	items := loadCorpus(t)
	dict := Dictionary()
	got := computeP6Stats(t, items, dict)

	for _, c := range []struct {
		name string
		got  int
		want int
	}{
		{"P6a", got.P6a, baselineP6a},
		{"P6b", got.P6b, baselineP6b},
		{"P6c", got.P6c, baselineP6c},
		{"P6d", got.P6d, baselineP6d},
		{"P6e", got.P6e, baselineP6e},
		{"P6f", got.P6f, baselineP6f},
		{"P6g", got.P6g, baselineP6g},
	} {
		if c.got != c.want {
			t.Errorf("%s drifted from D-000.57's declared baseline: got %d, baseline %d.\n"+
				"This is NOT a floor check — no floor moved and none may (D-000.17, D-2.1.14). It is D-000.57's byte-identity clause, executed.\n"+
				"If the corpus legitimately changed, the baseline is edited HERE, deliberately, in the same commit, and the change is named in the next gate document. If it did not, a regression is riding in under cover of a sanctioned red.",
				c.name, c.got, c.want)
		}
	}
}

// hasLatinOrDigit reports whether runes contains at least one ASCII
// Latin letter or ASCII digit — P6d's actual property ("mixing Thai
// with Latin/digits"), which a plain space or punctuation rune must
// NOT satisfy (the reopening's finding: the previous version counted
// any non-Thai rune, including the separating space in every
// two-token personal name, which is not "mixing" anything).
func hasLatinOrDigit(runes []rune) bool {
	for _, r := range runes {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return true
		}
	}
	return false
}

func computeP6Stats(t *testing.T, items []corpusItem, dict *BytesTrie) p6Stats {
	t.Helper()
	var stats p6Stats

	for _, it := range items {
		runes := []rune(it.Text)

		// P6a: this item's constrained decomposition contains at
		// least one uncoverable (RunUnknownThai) span.
		_, runs := ComputeBreaks(dict, it.Text, false)
		hasUnknown := false
		for _, r := range runs {
			if r.Kind == RunUnknownThai {
				hasUnknown = true
			}
		}
		if hasUnknown {
			stats.P6a++
		}

		// P6d: the reopening's finding — isThaiScript excludes U+0020,
		// so RunNonThai fired on the plain SPACE between a given name
		// and a surname in every two-token personal-name item, which
		// is not "mixing Thai with Latin/digits" at all. P6d now
		// requires an ACTUAL Latin letter or ASCII digit rune to be
		// present, checked directly against the item's runes rather
		// than derived from script-span classification.
		if hasLatinOrDigit(runes) {
			stats.P6d++
		}

		// P6b/P6c: scan this item's clusters for vowel+tone co-occurrence.
		hasVowelTone := false
		hasStackedVowelTone := false
		for _, span := range clusterSpans(runes) {
			var sawVowel, sawTone, sawAbove bool
			for i := span[0]; i < span[1]; i++ {
				r := runes[i]
				if isToneMark(r) {
					sawTone = true
				}
				if isAnyVowel(r) {
					sawVowel = true
				}
				if isAboveSign(r) {
					sawAbove = true
				}
			}
			if sawVowel && sawTone {
				hasVowelTone = true
			}
			if sawAbove && sawTone {
				hasStackedVowelTone = true
			}
		}
		if hasVowelTone {
			stats.P6b++
		}
		if hasStackedVowelTone {
			stats.P6c++
		}

		if it.Category == "personal_name" || it.Category == "place_name" {
			stats.P6e += len(it.ProperNounSpans)
		}

		if it.Category == "personal_name" && len(it.ProperNounSpans) > 0 {
			// P6f/P6g classify the SURNAME specifically — the LAST
			// proper-noun span (index 0 is the given name, per
			// requirement 5's widened P3 labelling).
			span := it.ProperNounSpans[len(it.ProperNounSpans)-1]
			surname := string(runes[span[0]:span[1]])
			unconBreaks, _ := ComputeBreaks(dict, surname, true)
			if len(unconBreaks) > 0 {
				stats.P6f++
			} else {
				stats.P6g++
			}
		}
	}
	return stats
}

// TestP1NeverBreaksInsideCluster is AC7 exercised over the WHOLE
// corpus (not just the synthetic cases in cluster_test.go/break_test.go):
// every break the constrained engine proposes, over every item, must
// land on a cluster boundary.
func TestP1NeverBreaksInsideCluster(t *testing.T) {
	items := loadCorpus(t)
	dict := Dictionary()

	var violations, exercised, itemsWithBreaks int
	for _, it := range items {
		runes := []rune(it.Text)
		boundary := ClusterBoundaries(runes)
		breaks, _ := ComputeBreaks(dict, it.Text, false)
		if len(breaks) > 0 {
			itemsWithBreaks++
		}
		for _, b := range breaks {
			exercised++
			if !boundary[b] {
				t.Errorf("P1 VIOLATION: item %s (%q) proposes a break at rune %d, which is not a cluster boundary", it.ID, it.Text, b)
				violations++
			}
		}
	}

	// VACUITY GUARD. P1 is a NEGATIVE assertion made once per proposed
	// break, so its strength is exactly the size of the population it
	// sweeps — and that population shrank when Story 2.4's
	// both-sides-coverable filter withdrew 32 of the corpus's 558
	// proposals (558 -> 526, measured). A future change that withdrew
	// them ALL would leave this test iterating zero times and reporting
	// "zero violations", which is how a guard dies silently while still
	// passing. The floor is deliberately well below the measured 526 —
	// it is a vacuity guard, not a second copy of the break count, which
	// AC10's fixture already pins exactly.
	if exercised == 0 {
		t.Fatal("vacuity: the corpus proposed no break at all, so P1 asserted nothing")
	}
	if exercised < 100 {
		t.Fatalf("vacuity: P1 swept only %d break positions across %d items; at the fix commit it swept 526. Something has withdrawn most of the corpus's break opportunities, and P1 is no longer measuring what it claims to", exercised, len(items))
	}
	if violations == 0 {
		t.Logf("P1: zero violations across the full corpus (absolute, per AD-25) — %d break positions swept across %d of %d items", exercised, itemsWithBreaks, len(items))
	}
}

// TestP2NeverBreaksInsideUnknownRun is a SELF-CONSISTENCY check only —
// it confirms the engine never contradicts its own RunUnknownThai
// classification, which is true by construction (segmentThaiSpan never
// emits a RunUnknownThai span it then also breaks inside). It is NOT
// P2's real measurement and must never be read as one: this test
// compares the engine's output against itself, so it can never find
// the failure mode that matters (the engine calling something
// "coverable" when it is not, or vice versa, per an INDEPENDENT
// ground truth). See TestP2IndependentDPCrossCheck below for the real
// measurement (D-000.17/18, the reopening) — that one is expected to,
// and does, fail.
func TestP2NeverBreaksInsideUnknownRun(t *testing.T) {
	items := loadCorpus(t)
	dict := Dictionary()

	var violations, unknownRuns, pairs int
	for _, it := range items {
		breaks, runs := ComputeBreaks(dict, it.Text, false)
		for _, r := range runs {
			if r.Kind != RunUnknownThai {
				continue
			}
			unknownRuns++
			for _, b := range breaks {
				pairs++
				if b > r.Start && b < r.End {
					t.Errorf("P2 VIOLATION: item %s (%q) proposes a break at rune %d, strictly inside an uncoverable run [%d,%d)", it.ID, it.Text, b, r.Start, r.End)
					violations++
				}
			}
		}
	}

	// VACUITY FLOOR, on the same pattern P1 carries and for the same
	// reason. This is a NEGATIVE assertion made once per
	// (uncoverable run, proposed break) pair, so its strength is exactly
	// the size of the population it sweeps — and Story 2.4's
	// both-sides-coverable filter shrank the break population it draws
	// from (558 -> 526). A change that stopped classifying anything as
	// RunUnknownThai, or withdrew the remaining breaks, would leave this
	// loop iterating zero times and still logging "zero violations",
	// which is how a self-referential guard dies silently while passing.
	//
	// The floors are deliberately well below the values measured at this
	// commit; they are vacuity guards, not second copies of a count that
	// AC10's fixture already pins exactly.
	if unknownRuns == 0 {
		t.Fatal("vacuity: the corpus produced no RunUnknownThai span at all, so P2's premise never arose and this test asserted nothing")
	}
	if pairs == 0 {
		t.Fatal("vacuity: no (uncoverable run, proposed break) pair was examined, so P2 swept an empty population")
	}
	if unknownRuns < 15 {
		t.Fatalf("vacuity: P2 found only %d RunUnknownThai spans across %d items; MEASURED AT THE STORY 2.4 FINISH COMMIT: 66 spans across 243 items. Something has stopped classifying Thai as uncoverable, and this guard is no longer measuring what it claims to", unknownRuns, len(items))
	}
	if violations == 0 {
		t.Logf("P2: zero violations across the full corpus (absolute, per AD-25) — %d RunUnknownThai spans and %d (run, break) pairs swept across %d items. SELF-CONSISTENCY ONLY: see TestP2IndependentDPCrossCheck for the measurement against an independent ground truth", unknownRuns, pairs, len(items))
	}
}

// TestP3ProperNounsNeverSplit is AC11's central measurement: does the
// CONSTRAINED engine — implementing exactly AD-25's two named
// absolutes and nothing else — ever propose a break strictly inside a
// hand-identified proper-noun span?
//
// This test does NOT fail the build on a P3 violation (t.Log, not
// t.Error): P3 is the spike's OWN pass condition, whose failure is a
// deviation routed to the OWNER (AC11's Who-decides table), not a
// defect for this developer to silently patch by adding an
// undocumented third constraint AD-25 does not name. Violations are
// recorded in full for the spike report.
func TestP3ProperNounsNeverSplit(t *testing.T) {
	items := loadCorpus(t)
	dict := Dictionary()

	var violations []p3Violation
	var itemsWithSpans, itemsViolated int
	for _, it := range items {
		if len(it.ProperNounSpans) == 0 {
			continue
		}
		itemsWithSpans++
		breaks, _ := ComputeBreaks(dict, it.Text, false)
		violatedThisItem := false
		for _, span := range it.ProperNounSpans {
			for _, b := range breaks {
				if b > span[0] && b < span[1] {
					violations = append(violations, p3Violation{
						ItemID:   it.ID,
						Text:     it.Text,
						BreakPos: b,
						Span:     span,
					})
					violatedThisItem = true
				}
			}
		}
		if violatedThisItem {
			itemsViolated++
		}
	}

	sort.Slice(violations, func(i, j int) bool { return violations[i].ItemID < violations[j].ItemID })

	t.Logf("P3 MEASUREMENT: %d/%d proper-noun-bearing items violated (%d individual break violations)", itemsViolated, itemsWithSpans, len(violations))
	for _, v := range violations {
		t.Logf("  P3 violation: item=%s text=%q break-at-rune=%d proper-noun-span=%v", v.ItemID, v.Text, v.BreakPos, v.Span)
	}
	if len(violations) == 0 {
		t.Log("P3: zero violations across the full corpus (absolute, per AD-25's Prevents line)")
	} else {
		t.Logf("P3: %d violations — see spike report for the routed DECISION NEEDED", len(violations))
	}
}
