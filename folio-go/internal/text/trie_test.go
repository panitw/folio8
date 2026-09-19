package text

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// repoRootFromTest mirrors the identical helper duplicated across
// lint/internal/rules, lint/internal/manifest — this package needs its
// own copy since it lives in a different module (D-1.3.1's `_test.go`
// exemption permits `os`/`path/filepath` here).
func repoRootFromTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		folio8Go, err1 := os.Stat(filepath.Join(dir, "folio-go"))
		lintDir, err2 := os.Stat(filepath.Join(dir, "lint"))
		if err1 == nil && folio8Go.IsDir() && err2 == nil && lintDir.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root walking up from %s", dir)
		}
		dir = parent
	}
}

func loadWordlist(t *testing.T) []string {
	t.Helper()
	root := repoRootFromTest(t)
	path := filepath.Join(root, "folio-go", "internal", "text", "wordlist", "words_th.txt")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open wordlist: %v", err)
	}
	defer f.Close()

	var words []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			words = append(words, line)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan wordlist: %v", err)
	}
	return words
}

// TestWordlistMeasuredCount is AC1's "measure and record the actual
// word count" clause (D-000.14): the epic's carried figure (62,106) is
// a claim to verify, not a number to restate. It asserts on DISTINCT
// words, not raw lines — the file has 62,107 LINES but one duplicate
// entry ("โรม่า", appearing twice), so the raw line count is an
// artifact, not the word count. Reported per the reopening: reporting
// a fabricated discrepancy against the epic's figure is exactly as
// corrosive as reporting a fabricated agreement (D-000.17/18's spirit).
func TestWordlistMeasuredCount(t *testing.T) {
	words := loadWordlist(t)
	const measuredLines = 62107
	if len(words) != measuredLines {
		t.Fatalf("wordlist LINE count drifted: got %d, this test's recorded measurement is %d — update this constant, TestWordlistDistinctCount below, and the spike report together", len(words), measuredLines)
	}

	distinct := map[string]bool{}
	var dupes []string
	for _, w := range words {
		if distinct[w] {
			dupes = append(dupes, w)
		}
		distinct[w] = true
	}

	const epicClaimed = 62106
	if len(distinct) != epicClaimed {
		t.Fatalf("wordlist DISTINCT word count (%d) no longer matches the epic's carried figure (%d) — this is the number AC1 actually cares about; update the spike report", len(distinct), epicClaimed)
	}
	// This measurement's own duplicate-word finding: exactly one
	// duplicate expected ("โรม่า"). Asserted explicitly so this test
	// is not vacuously checking a count that happens to match by
	// coincidence — it names the mechanism (one duplicate line)
	// that makes 62,107 lines resolve to 62,106 distinct words.
	if len(dupes) != 1 {
		t.Fatalf("expected exactly 1 duplicate word (measured, this story's dev record: \"โรม่า\"), got %d: %v", len(dupes), dupes)
	}
	if dupes[0] != "โรม่า" {
		t.Fatalf("the duplicate word changed identity: got %q, recorded measurement was \"โรม่า\" — update this test and the spike report", dupes[0])
	}
}

// TestCompileTrieDeterministic is Trap 2 / task 3's requirement: the
// compiled artifact must be byte-stable across runs, because it is
// committed and hashed. Two independent compiles of the same input
// must be byte-identical.
func TestCompileTrieDeterministic(t *testing.T) {
	words := []string{"กก", "กกกอด", "ขนม", "ขนมปัง", "ก"}
	a := CompileTrie(words)
	b := CompileTrie(words)
	if string(a) != string(b) {
		t.Fatal("CompileTrie is non-deterministic across two runs over identical input")
	}

	shuffled := append([]string(nil), words...)
	sort.Sort(sort.Reverse(sort.StringSlice(shuffled)))
	c := CompileTrie(shuffled)
	if string(a) != string(c) {
		t.Fatal("CompileTrie output depends on input order — it must not (children are always serialized in sorted byte order)")
	}
}

// TestTrieRegeneratedMatchesCommitted regenerates the trie from the
// committed wordlist and compares it, byte for byte, against the
// committed embedded artifact — mirroring TestManifestUpToDate's
// regeneration-consistency shape. A drift here means someone edited
// data/thai_words.trie by hand, or wordlist/words_th.txt changed
// without re-running `go run ./cmd/gentrie`.
func TestTrieRegeneratedMatchesCommitted(t *testing.T) {
	words := loadWordlist(t)
	regenerated := CompileTrie(words)
	if string(regenerated) != string(compiledTrie) {
		t.Fatalf("data/thai_words.trie is stale (got %d bytes regenerated, %d committed) — run `cd folio-go && go run ./cmd/gentrie` and commit the result", len(regenerated), len(compiledTrie))
	}
}

// TestBytesTrieContainsAndLongestMatch is AC1/AC3's functional
// baseline, exercised against the real embedded dictionary rather than
// a synthetic fixture, so the probe queries used again for the js/wasm
// leg (AC3) are grounded here first on the native target.
func TestBytesTrieContainsAndLongestMatch(t *testing.T) {
	d := Dictionary()

	if !d.Contains("ประเทศ") {
		t.Error("expected dictionary to contain a known-present word: ประเทศ")
	}
	if d.Contains("กขฃคฅฆงจฉ") {
		t.Error("expected dictionary NOT to contain a nonsense string with no meaning")
	}
	if got := d.LongestMatch([]byte("ประเทศไทย")); got == 0 {
		t.Error("expected a nonzero longest-prefix match for ประเทศไทย")
	}
}
