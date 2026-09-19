package expr

// Layer 1's oracle test: the exact reduction kernel (SumDecimals,
// AvgDecimals) compared against Story 3.1a's recorded golden
// (testdata/decimal-reduction/golden.json), whose provenance lives in
// testdata/decimal-reduction/PROVENANCE.md.

import (
	"encoding/json"
	"math/big"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// goldenCorpusMember is one entry of golden.json: an operand corpus and
// its independently-produced expected sum/avg, as decimal literal
// strings (AC7).
type goldenCorpusMember struct {
	Operands []string `json:"operands"`
	Sum      string   `json:"sum"`
	Avg      string   `json:"avg"`
}

func loadGolden(t *testing.T) map[string]goldenCorpusMember {
	t.Helper()
	root := repoRootFromTest(t)
	path := filepath.Join(root, "folio-go", "testdata", "decimal-reduction", "golden.json")
	data := mustReadFile(t, path)

	var golden map[string]goldenCorpusMember
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatalf("parse golden %s: %v", path, err)
	}
	if len(golden) == 0 {
		t.Fatalf("presence precondition (D-000.9): golden %s parsed to zero corpus members", path)
	}
	return golden
}

func goldenCorpusNames(golden map[string]goldenCorpusMember) []string {
	names := make([]string, 0, len(golden))
	for name := range golden {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// nonDiscriminatingCorpusMembers is AC13: corpus members B and C ship
// alongside A, but are retained AS EVIDENCE (D-000.29: settled, never
// carried) rather than as coverage — they demonstrate that the OBVIOUS
// fixture proves nothing, per D-000.50's population check
// (testdata/decimal-reduction/PROVENANCE.md carries the full measured
// comparison, including the honest float64-mutant divergence table).
var nonDiscriminatingCorpusMembers = map[string]string{
	"B": "exercises the alignment path (mixed operand scales) that corpus A never touches, but any " +
		"pre-quantisation divergence under an honest float64 mutant is erased once the mutant's " +
		"result is rounded to the operation's declared scale",
	"C": "the intuitive small two-decimal bank-statement fixture a reasonable person would write; " +
		"byte-identical under an honest float64 accumulator for both sum and avg — it cannot express " +
		"AD-23's defect at all (D-000.50's hazard, reproduced on this story's own subject)",
}

// discriminatingCorpusMember is the one golden member this oracle
// actually depends on for AC10's red-proof (corpus A) — the sole
// exemption from the "every shipped corpus is labelled" rule below.
const discriminatingCorpusMember = "A"

// TestGoldenCorporaBAndCAreLabelledNonDiscriminating is AC13.
//
// CORRECTION (QA review Finding 7, Minor). This test used to iterate
// nonDiscriminatingCorpusMembers and check each entry has a corpus —
// which proves labels have corpora, never that corpora have labels.
// Deleting "C" from the map, or adding an unlabelled corpus "D" to
// golden.json, both passed unchanged. AC13's actual purpose — "a later
// reader must not mistake [a non-discriminating member] for teeth" —
// requires the OTHER direction: every corpus golden.json ships, other
// than the one discriminating member, must carry a non-empty label.
// This now iterates golden.json itself.
func TestGoldenCorporaBAndCAreLabelledNonDiscriminating(t *testing.T) {
	golden := loadGolden(t)
	if _, ok := golden[discriminatingCorpusMember]; !ok {
		t.Fatalf("presence precondition: corpus %q (the discriminating member) is absent from golden.json", discriminatingCorpusMember)
	}
	if _, ok := nonDiscriminatingCorpusMembers[discriminatingCorpusMember]; ok {
		t.Fatalf("corpus %q is mislabelled non-discriminating — it is the member AC10's red-proof depends on", discriminatingCorpusMember)
	}

	for name := range golden {
		if name == discriminatingCorpusMember {
			continue
		}
		reason, ok := nonDiscriminatingCorpusMembers[name]
		if !ok {
			t.Errorf("corpus %q ships in golden.json but is not labelled non-discriminating — a later reader could mistake it for teeth (AC13)", name)
			continue
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("corpus %q carries an empty non-discriminating reason", name)
		}
	}
}

// scaleOfLiteral counts the digits after '.' in a decimal literal
// string (0 for an integer literal with no '.'). Used only to check the
// golden's OWN internal consistency (AC11), independently of NewDecimal.
func scaleOfLiteral(s string) int {
	idx := strings.IndexByte(s, '.')
	if idx < 0 {
		return 0
	}
	return len(s) - idx - 1
}

// exactBigIntSum sums a corpus's operand literals EXACTLY, using its
// own small, independent big.Int alignment — deliberately NOT calling
// SumDecimals, so this acceptance step does not depend on the very
// kernel it exists to help validate. It returns the result as a decimal
// literal string, minus-sign-and-all, at the corpus's maximum operand
// scale, for direct string comparison against golden.json.
func exactBigIntSum(t *testing.T, operands []string) string {
	t.Helper()
	maxScale := 0
	for _, s := range operands {
		if sc := scaleOfLiteral(s); sc > maxScale {
			maxScale = sc
		}
	}

	total := new(big.Int)
	for _, s := range operands {
		neg := strings.HasPrefix(s, "-")
		if neg {
			s = s[1:]
		}
		intPart, fracPart := s, ""
		if idx := strings.IndexByte(s, '.'); idx >= 0 {
			intPart, fracPart = s[:idx], s[idx+1:]
		}
		digits := intPart + fracPart + strings.Repeat("0", maxScale-len(fracPart))
		term := new(big.Int)
		if _, ok := term.SetString(digits, 10); !ok {
			t.Fatalf("exactBigIntSum: could not parse digit string %q from operand %q", digits, s)
		}
		if neg {
			term.Neg(term)
		}
		total.Add(total, term)
	}

	// Render total as a decimal literal at maxScale, matching
	// golden.json's own string formatting (a plain sign, integer part,
	// '.', then exactly maxScale fractional digits — no exponent
	// notation, since these results never need one).
	neg := total.Sign() < 0
	abs := new(big.Int).Abs(total)
	digits := abs.String()
	for len(digits) <= maxScale {
		digits = "0" + digits
	}
	var out string
	if maxScale == 0 {
		out = digits
	} else {
		splitAt := len(digits) - maxScale
		out = digits[:splitAt] + "." + digits[splitAt:]
	}
	if neg {
		out = "-" + out
	}
	return out
}

// TestGoldenIsOrderInvariant is D-000.22's semantic acceptance step for
// this FIRST recording.
//
// CORRECTION (QA review Finding 8, Minor). The comment used to name
// the forward/backward comparison itself as the load-bearing property.
// It is not: exactBigIntSum always accumulates in big.Int, which is
// commutative BY CONSTRUCTION, so "forward != backward" cannot fail for
// any input — confirmed empirically, this clause passed unchanged even
// under the float64 mutation that reddened every other assertion in
// this file. It stays (a cheap forward guard on the helper, and it
// would catch a transcription bug in the reversal loop itself), but it
// is not what discharges D-000.22/AC11 here.
//
// What actually discharges the acceptance step, both independent of
// SumDecimals/AvgDecimals: exact addition, re-derived from scratch (not
// via the kernel) rather than restated as a second copy of the
// producing hash, matches golden.json's own recorded sum
// ("forward != member.Sum" below — genuinely falsifiable, and the
// substantive half of this test). The recorded sum's scale is also
// asserted to equal its own corpus's maximum operand scale (AC11's
// second clause), read off golden.json alone.
func TestGoldenIsOrderInvariant(t *testing.T) {
	golden := loadGolden(t)
	for _, name := range goldenCorpusNames(golden) {
		member := golden[name]
		if len(member.Operands) == 0 {
			t.Fatalf("presence precondition: corpus %q has zero operands", name)
		}

		forward := exactBigIntSum(t, member.Operands)
		reversed := make([]string, len(member.Operands))
		for i, op := range member.Operands {
			reversed[len(member.Operands)-1-i] = op
		}
		backward := exactBigIntSum(t, reversed)

		if forward != backward {
			t.Errorf("corpus %q: exact sum is NOT order-invariant — forward %s, reversed %s", name, forward, backward)
		}
		if forward != member.Sum {
			t.Errorf("corpus %q: independently-computed exact sum %s does not match golden.json's recorded sum %s", name, forward, member.Sum)
		}

		maxScale := 0
		for _, op := range member.Operands {
			if sc := scaleOfLiteral(op); sc > maxScale {
				maxScale = sc
			}
		}
		if got := scaleOfLiteral(member.Sum); got != maxScale {
			t.Errorf("corpus %q: recorded sum %q carries scale %d, want the corpus's maximum operand scale %d", name, member.Sum, got, maxScale)
		}
	}
}

// TestDecimalReductionKernelMatchesGolden is AC8: the kernel's output
// compared against the golden for BOTH operations across EVERY corpus
// member, asserting the Decimal's coefficient AND exponent — never a
// rendered string, which can agree while the underlying value does not.
func TestDecimalReductionKernelMatchesGolden(t *testing.T) {
	golden := loadGolden(t)
	for _, name := range goldenCorpusNames(golden) {
		member := golden[name]

		operands := mustDecimals(t, member.Operands...)
		wantSum := mustDecimal(t, member.Sum)
		wantAvg := mustDecimal(t, member.Avg)

		gotSum, err := SumDecimals(operands)
		if err != nil {
			t.Fatalf("corpus %q: SumDecimals: unexpected error: %v", name, err)
		}
		if gotSum != wantSum {
			t.Errorf("corpus %q: SumDecimals = %+v, want %+v (golden %q)", name, gotSum, wantSum, member.Sum)
		}

		gotAvg, err := AvgDecimals(operands)
		if err != nil {
			t.Fatalf("corpus %q: AvgDecimals: unexpected error: %v", name, err)
		}
		if gotAvg != wantAvg {
			t.Errorf("corpus %q: AvgDecimals = %+v, want %+v (golden %q)", name, gotAvg, wantAvg, member.Avg)
		}
	}
}

// TestDecimalReductionKernelIsOrderInvariant is AC9: SumDecimals over
// each corpus, in declared order and in reverse, produces the IDENTICAL
// Decimal. This is what stops a red-proof passing by luck (F4, D-000.61)
// — the mutant this story red-proved against gives two different
// answers depending on operand order, so a single-order comparison could
// have agreed with the exact result by coincidence (as it did, on corpus
// A's reversed order, per D-000.61).
func TestDecimalReductionKernelIsOrderInvariant(t *testing.T) {
	golden := loadGolden(t)
	for _, name := range goldenCorpusNames(golden) {
		member := golden[name]
		operands := mustDecimals(t, member.Operands...)

		reversed := make([]Decimal, len(operands))
		for i, d := range operands {
			reversed[len(operands)-1-i] = d
		}

		forward, err := SumDecimals(operands)
		if err != nil {
			t.Fatalf("corpus %q: SumDecimals (forward): unexpected error: %v", name, err)
		}
		backward, err := SumDecimals(reversed)
		if err != nil {
			t.Fatalf("corpus %q: SumDecimals (reversed): unexpected error: %v", name, err)
		}
		if forward != backward {
			t.Errorf("corpus %q: SumDecimals is NOT order-invariant — forward %+v, reversed %+v", name, forward, backward)
		}
	}
}
