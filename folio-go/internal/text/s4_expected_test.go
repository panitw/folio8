package text

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// expectedBreakItem is one hand-authored label in S4's frozen
// expected-break fixture.
//
// Words is the LABEL — the segmentation a reader would make — and
// ExpectedBreaks is its arithmetic. A reviewer checks Words; the numbers
// follow from it. That is the shape that makes "hand-checked once" a
// thing a person can actually do (acceptance.md:65).
// DeclaredAtomic is D-2.1.6's declaration channel — AD-25's third
// mechanism ("a declared value is never split") — reaching this
// fixture for the first time (D-2.4.9). An item carrying it is passed
// to Opportunities as a single atomic span covering its whole text,
// rather than as a bare string, so the engine reproduces "no interior
// break" BECAUSE it was told, not because anything was inferred.
type expectedBreakItem struct {
	ID             string   `json:"id"`
	Text           string   `json:"text"`
	Words          []string `json:"words"`
	ExpectedBreaks []int    `json:"expectedBreaks"`
	DeclaredAtomic bool     `json:"declaredAtomic"`
	Gloss          string   `json:"gloss"`
}

type expectedBreakFixture struct {
	Items []expectedBreakItem `json:"items"`
}

func loadExpectedBreaks(t *testing.T) expectedBreakFixture {
	t.Helper()
	path := filepath.Join(repoRootFromTest(t), "fixtures", "expected-breaks", "expected_breaks.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read expected_breaks.json: %v", err)
	}
	var f expectedBreakFixture
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("unmarshal expected_breaks.json: %v", err)
	}
	return f
}

// TestS4ExpectedBreaksAreLabelsNotEngineOutput is the fixture's
// SELF-CONSISTENCY check, and it runs before the conformance assertion
// because it is what makes the labels auditable: every item's words must
// concatenate to its text, and its expected positions must be exactly
// the cumulative rune lengths of all but the last word.
//
// This is the property that keeps the file hand-checkable. If the
// numbers could drift from the segmentation, a reviewer reading the
// words would be certifying something other than what the test asserts.
// It also makes "regenerated to make a test pass" visible: you cannot
// quietly move a number without moving a word boundary a human can see.
func TestS4ExpectedBreaksAreLabelsNotEngineOutput(t *testing.T) {
	f := loadExpectedBreaks(t)
	if len(f.Items) == 0 {
		t.Fatal("V-S4: the expected-break fixture is empty")
	}
	multiWord := 0
	for _, it := range f.Items {
		if len(it.Words) >= 2 {
			multiWord++
		}
		joined := ""
		for _, w := range it.Words {
			joined += w
		}
		if joined != it.Text {
			t.Errorf("%s: words %v concatenate to %q, but text is %q", it.ID, it.Words, joined, it.Text)
			continue
		}
		var want []int
		acc := 0
		for _, w := range it.Words[:max(0, len(it.Words)-1)] {
			acc += len([]rune(w))
			want = append(want, acc)
		}
		if len(want) == 0 {
			want = nil
		}
		got := it.ExpectedBreaks
		if len(got) == 0 {
			got = nil
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s (%q): expectedBreaks %v does not match its own segmentation %v, which implies %v — the numbers must follow the words", it.ID, it.Text, it.ExpectedBreaks, it.Words, want)
		}
	}

	// NON-DEGENERACY GUARD for BOTH assertions above, which are the same
	// conservation shape D-000.33 names: "the parts reconstruct the
	// whole". A single-word item satisfies `concat(words) == text`
	// trivially — it is `whole == whole` — and derives an EMPTY expected
	// list, so a fixture of nothing but single-word items would certify
	// neither the concatenation nor the cumulative-length arithmetic.
	// At least one item must genuinely be partitioned.
	if multiWord == 0 {
		t.Fatalf("vacuity: all %d fixture items carry a single word, so `words concatenate to text` reduces to whole == whole and the cumulative-length derivation has nothing to derive", len(f.Items))
	}
	t.Logf("V-S4 self-consistency: %d of %d items are genuinely partitioned (>= 2 words), so the concatenation and the cumulative-length derivation are non-degenerate", multiWord, len(f.Items))
}

// s4Divergence is an ENUMERATED, JUSTIFIED exception to AC14's exact
// equality, asserted as SET EQUALITY against this declared list — never
// a count (D-2.5.1's shape) — never as a relaxed comparison.
//
// WHY THIS EXISTS (D-2.4.9). The owner's 2026-08-24 hand-check proved
// "where the engine may break" and "where a reader accepts a break" are
// two different predicates that diverge on lexicalised Thai compounds:
// หนังสือพิมพ์/วันเกิด/ที่อยู่ are headwords the engine therefore never
// splits, but a reader accepts a break inside them anyway. Both
// forbidden levers stay forbidden: the shipped wordlist is not edited
// (the words ARE real Thai words; D-000.32), and no heuristic is
// invented (AD-25: "the engine never infers membership. It cannot.").
// So the gap cannot be closed — it can only be named and bounded.
//
// THE DIRECTION RULE IS WHAT KEEPS THIS LIST'S TEETH, AND IT IS
// BINDING (D-2.1.2, D-2.4.9): only FAIL-CLOSED divergences may be
// enumerated — the engine proposing FEWER breaks than the human
// accepts (conservative: the compound moves to the next line whole,
// never renders wrongly). A FAIL-OPEN divergence — the engine
// proposing a break the human REJECTS — is never admissible and is
// always a defect, never an entry here. The loop below enforces this
// by asserting every engine-proposed position is a SUBSET of the
// human label's positions for every declared entry; violating that
// fails the test rather than being silently accepted, which is this
// rule's own red-proof.
type s4Divergence struct {
	ID string
	// EngineBreaks is a second literal (D-2.3.4's mechanism) pinning
	// what Opportunities actually proposes today for this item's bare
	// text. It is re-measured below, not trusted — a change here must
	// be a deliberate, attributable edit, exactly like a golden digest.
	EngineBreaks []int
	// HumanLabel restates the fixture's expectedBreaks for this item so
	// both sides of the divergence are visible in one place without
	// cross-referencing the JSON. It is checked against the live
	// fixture value below, so it cannot drift from it silently.
	HumanLabel []int
	Reason     string
}

var s4ExpectedDivergences = []s4Divergence{
	{
		ID:           "thai-007",
		EngineBreaks: nil,
		HumanLabel:   []int{7},
		Reason:       "หนังสือพิมพ์ (newspaper) is a dictionary headword, so the engine proposes no interior break; the owner's hand-check accepts a break at the หนังสือ|พิมพ์ seam anyway. No wordlist property predicts this (D-2.4.10) — it is a per-item native-speaker judgment.",
	},
	{
		ID:           "thai-008",
		EngineBreaks: nil,
		HumanLabel:   []int{3},
		Reason:       "วันเกิด (birthday) is a dictionary headword, so the engine proposes no interior break; the owner's hand-check accepts a break at the วัน|เกิด seam anyway. No wordlist property predicts this (D-2.4.10) — it is a per-item native-speaker judgment.",
	},
	{
		ID:           "thai-009",
		EngineBreaks: nil,
		HumanLabel:   []int{3},
		Reason:       "ที่อยู่ (address) is a dictionary headword, so the engine proposes no interior break; the owner's hand-check accepts a break at the ที่|อยู่ seam anyway. No wordlist property predicts this (D-2.4.10) — it is a per-item native-speaker judgment.",
	},
}

// TestS4ExpectedBreaksMatchTheEngine is AC14: the engine's break
// opportunities must equal the frozen, hand-authored labels exactly
// (epics.md:836, "results match the fixture exactly") — with the
// small, enumerated, fail-closed-only exception of s4ExpectedDivergences
// above.
//
// THE FIXTURE IS THE ORACLE, NOT THE OUTPUT. If this test fails on an
// UNDECLARED item, the engine is wrong or the label is wrong — and
// which one is a question for a person. It is NEVER resolved by
// regenerating the fixture. There is no generator for it, deliberately:
// nothing in this repository can write
// fixtures/expected-breaks/expected_breaks.json. A mismatch on a
// declared item is resolved by correcting or removing the declaration,
// under the same rule.
//
// This is distinct from TestAC10ComputedBreaksMatchS4Basis, which
// compares the engine against ITS OWN recorded output across targets and
// whose own README says in terms that it is "a cross-target REGRESSION
// ANCHOR ONLY, never a correctness oracle". That one answers "does this
// target agree with the others"; this one answers "is the answer right".
//
// Every item is free of whitespace, so each opportunity is zero-width
// (LineEnd == NextStart) and comparing positions compares everything.
// That is asserted rather than assumed.
//
// VACUITY GUARDS, all of which must hold for the comparison to mean
// anything:
//   - the fixture is non-empty;
//   - at least one item expects a NON-ZERO number of breaks (otherwise an
//     engine returning nothing would pass everything);
//   - at least one item expects ZERO (otherwise an engine returning a
//     break at every position would not be caught by the corpus of
//     compounds alone);
//   - the total number of expected positions is non-zero.
func TestS4ExpectedBreaksMatchTheEngine(t *testing.T) {
	f := loadExpectedBreaks(t)
	dict := Dictionary()

	if len(f.Items) == 0 {
		t.Fatal("V-S4: the expected-break fixture is empty")
	}
	var withBreaks, withoutBreaks, totalExpected int
	for _, it := range f.Items {
		if len(it.ExpectedBreaks) > 0 {
			withBreaks++
			totalExpected += len(it.ExpectedBreaks)
		} else {
			withoutBreaks++
		}
	}
	if withBreaks == 0 {
		t.Fatal("V-S4: no item expects a break — an engine that never proposes one would pass this fixture")
	}
	if withoutBreaks == 0 {
		t.Fatal("V-S4: no item expects ZERO breaks — the atomic/uncoverable polarity is untested")
	}
	if totalExpected == 0 {
		t.Fatal("V-S4: the fixture expects no break positions at all")
	}

	divergenceByID := make(map[string]s4Divergence, len(s4ExpectedDivergences))
	for _, d := range s4ExpectedDivergences {
		divergenceByID[d.ID] = d
	}
	actualDivergences := map[string]bool{}

	var mismatches int
	for _, it := range f.Items {
		var atomic []Span
		if it.DeclaredAtomic {
			atomic = []Span{{Start: 0, End: len([]rune(it.Text))}}
		}
		ops := Opportunities(dict, it.Text, atomic)

		got := make([]int, 0, len(ops))
		for _, o := range ops {
			if o.LineEnd != o.NextStart {
				t.Errorf("%s (%q): opportunity %+v consumes runes, but every fixture item is whitespace-free and must break zero-width", it.ID, it.Text, o)
			}
			got = append(got, o.LineEnd)
		}
		if len(got) == 0 {
			got = nil
		}
		want := it.ExpectedBreaks
		if len(want) == 0 {
			want = nil
		}
		if reflect.DeepEqual(got, want) {
			continue
		}

		d, declared := divergenceByID[it.ID]
		if !declared {
			mismatches++
			t.Errorf("S4 MISMATCH: %s (%q, %s): engine proposes %v, the frozen label expects %v (segmentation: %v).\n"+
				"    DO NOT regenerate the fixture to close this. Either the engine is wrong or the label is wrong, and that is a question for a person.\n"+
				"    If this is a genuine engine/reader divergence (fail-closed only), enumerate it in s4ExpectedDivergences instead of relaxing this assertion.",
				it.ID, it.Text, it.Gloss, got, want, it.Words)
			continue
		}
		actualDivergences[it.ID] = true

		// D-2.4.9's direction rule, enforced rather than trusted: every
		// position the engine proposes for a declared divergence must
		// also be a position the human accepts. A position in `got` but
		// not in `want` is FAIL-OPEN — the engine breaking somewhere the
		// reader rejects — and that is NEVER admissible as a divergence,
		// declared or not.
		wantSet := make(map[int]bool, len(want))
		for _, w := range want {
			wantSet[w] = true
		}
		for _, g := range got {
			if !wantSet[g] {
				mismatches++
				t.Errorf("S4 DIVERGENCE %s is FAIL-OPEN and INADMISSIBLE: engine proposes a break at %d which the human label %v does not contain. Only fail-closed divergences (engine conservative, human accepts MORE breaks) may be enumerated in s4ExpectedDivergences (D-2.4.9) — this is a real engine defect, not a documentable gap, and it must be fixed rather than listed.",
					it.ID, g, want)
			}
		}

		// The pinned literals must match what was actually measured —
		// this is the second-literal check (D-2.3.4), so a silent drift
		// in either the engine's behaviour or the fixture's label moves
		// a value here that a diff will show.
		if !reflect.DeepEqual(got, d.EngineBreaks) {
			t.Errorf("s4ExpectedDivergences[%s].EngineBreaks is %v, but the engine now proposes %v — update the pinned literal deliberately, or if the engine changed behaviour investigate why", it.ID, d.EngineBreaks, got)
		}
		if !reflect.DeepEqual(want, d.HumanLabel) {
			t.Errorf("s4ExpectedDivergences[%s].HumanLabel is %v, but the fixture's expectedBreaks is now %v — the two have drifted apart", it.ID, d.HumanLabel, want)
		}

		t.Logf("S4 divergence (declared, fail-closed): %s (%q): engine proposes %v, human label is %v — %s", it.ID, it.Text, got, want, d.Reason)
	}

	// Set equality (D-2.5.1's shape), never a count: every declared
	// divergence must actually diverge today. A declared entry that no
	// longer diverges (got == want) is a stale claim about the engine
	// that AC14 would otherwise re-assert every run without anyone
	// checking it — remove it instead of leaving it inert.
	for id := range divergenceByID {
		if !actualDivergences[id] {
			mismatches++
			t.Errorf("s4ExpectedDivergences declares %q as a divergence, but the engine now matches the fixture label exactly for it — remove the stale entry; AC14's plain equality covers it now", id)
		}
	}

	if mismatches == 0 {
		t.Logf("AC14: the engine reproduces all %d hand-authored S4 labels exactly, modulo %d declared fail-closed divergence(s) (%d items expecting breaks, %d expecting none, %d positions in total)",
			len(f.Items), len(s4ExpectedDivergences), withBreaks, withoutBreaks, totalExpected)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
