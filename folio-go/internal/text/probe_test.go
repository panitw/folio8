package text

import (
	"runtime"
	"testing"
)

// Probe values for AC3's four-category cross-target query set, each
// independently verified against the committed wordlist (this story's
// dev record — see fixtures/thai-break-corpus and the spike report):
const (
	probeKnownPresentWord = "ประเทศ"                                                                 // "country" — an ordinary, unambiguous dictionary entry
	probeKnownAbsentWord  = "กขฃคฅฆงจฉ"                                                              // a run of consonant letters strung together; no meaning, not an entry
	probeNonWordPrefix    = "กกช้า"                                                                  // a real prefix of the entry "กกช้าง", but not itself an entry
	probeLongestWord      = "สำนักงานคณะกรรมการป้องกันและปราบปรามการทุจริตและประพฤติมิชอบในวงราชการ" // the longest space-free entry in the wordlist (70 runes)
)

// TestProbeQueries is AC3's cross-target assertion, run identically on
// every target (native here; js/wasm via wasm_test.go's harness, which
// builds and runs this exact test binary under Node). It asserts on
// QUERY OUTCOMES (D-000.13), never on exit status alone, and logs the
// reported GOOS/GOARCH so the js/wasm leg can assert it actually ran on
// the target it claims to (V6) rather than silently falling back to
// native.
func TestProbeQueries(t *testing.T) {
	t.Logf("PROBE-TARGET: GOOS=%s GOARCH=%s", runtime.GOOS, runtime.GOARCH)

	d := Dictionary()

	if !d.Contains(probeKnownPresentWord) {
		t.Errorf("known-present probe failed: Contains(%q) = false, want true", probeKnownPresentWord)
	}
	if d.Contains(probeKnownAbsentWord) {
		t.Errorf("known-absent probe failed: Contains(%q) = true, want false", probeKnownAbsentWord)
	}
	if d.Contains(probeNonWordPrefix) {
		t.Errorf("non-word-prefix probe failed: Contains(%q) = true, want false (it is a prefix of a real entry, not an entry itself)", probeNonWordPrefix)
	}
	if !d.Contains(probeLongestWord) {
		t.Errorf("longest-word probe failed: Contains(%q) = false, want true", probeLongestWord)
	}

	// Cross-check LongestMatch against the same four probes, so the
	// prefix-matching code path (used by the break engine) is exercised
	// too, not just exact Contains.
	if got := d.LongestMatch([]byte(probeKnownPresentWord)); got != len(probeKnownPresentWord) {
		t.Errorf("LongestMatch(known-present) = %d bytes, want %d (the full word)", got, len(probeKnownPresentWord))
	}
	if got := d.LongestMatch([]byte(probeKnownAbsentWord)); got == len(probeKnownAbsentWord) {
		t.Errorf("LongestMatch(known-absent) unexpectedly matched the whole string")
	}
	if got := d.LongestMatch([]byte(probeNonWordPrefix)); got == len(probeNonWordPrefix) {
		t.Errorf("LongestMatch(non-word-prefix) unexpectedly reported the prefix itself as a complete word")
	}
	if got := d.LongestMatch([]byte(probeLongestWord)); got != len(probeLongestWord) {
		t.Errorf("LongestMatch(longest-word) = %d bytes, want %d (the full word)", got, len(probeLongestWord))
	}

	t.Log("PROBE-RESULT: all four probe categories returned the expected outcome")
}
