package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// findRepoRoot walks up from the working directory until it finds a
// directory holding both folio-go/ and lint/ — the same pattern
// duplicated across internal/text/trie_test.go,
// lint/internal/rules/testutil_test.go, lint/internal/manifest and
// lint/cmd/genmanifest/main_test.go. Kept as its own copy per this
// package's `_test.go` D-1.3.1 exemption (os, path/filepath).
func findRepoRoot(t *testing.T) string {
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

// TestCorpusRegeneratedMatchesCommitted mirrors
// internal/text.TestTrieRegeneratedMatchesCommitted's shape (this
// story's second QA review, Major 5): D-000.17's "the generator may
// assemble from sourced data; it may not invent items to reach a
// number" property was, before this test existed, enforced only by
// the obsoleteConsonants panic loops inside buildItems — which fire
// only when a human actually runs `go run ./cmd/gencorpus`. Nothing
// compared the committed fixture (fixtures/thai-break-corpus/corpus.json)
// against the generator's own output, so a hand-edit inserting an
// invented item directly into corpus.json (bypassing buildItems
// entirely, and therefore bypassing the panic loops too) passed every
// test in the repository. This test closes that gap: it regenerates
// the corpus from buildItems() and compares it, structurally, against
// the committed JSON. A drift here means someone edited
// fixtures/thai-break-corpus/corpus.json by hand, or the curated word
// lists in this package changed without re-running
// `go run ./cmd/gencorpus` (and, since break positions derive from the
// corpus, `go run ./cmd/genbreaks`) and committing the result.
func TestCorpusRegeneratedMatchesCommitted(t *testing.T) {
	regenerated := buildItems()
	if len(regenerated) == 0 {
		t.Fatal("buildItems() produced zero items")
	}

	regeneratedJSON, err := json.MarshalIndent(regenerated, "", "  ")
	if err != nil {
		t.Fatalf("marshal regenerated items: %v", err)
	}
	regeneratedJSON = append(regeneratedJSON, '\n')

	root := findRepoRoot(t)
	committedPath := filepath.Join(root, "fixtures", "thai-break-corpus", "corpus.json")
	committed, err := os.ReadFile(committedPath)
	if err != nil {
		t.Fatalf("read committed corpus: %v", err)
	}

	if string(regeneratedJSON) != string(committed) {
		var committedItems []CorpusItem
		if jerr := json.Unmarshal(committed, &committedItems); jerr != nil {
			t.Fatalf("committed corpus.json is stale AND fails to parse as []CorpusItem: %v", jerr)
		}
		if len(committedItems) != len(regenerated) {
			t.Fatalf("fixtures/thai-break-corpus/corpus.json is stale: committed has %d items, buildItems() produces %d — run `cd folio-go && go run ./cmd/gencorpus && go run ./cmd/genbreaks` from the module root and commit the result", len(committedItems), len(regenerated))
		}
		t.Fatalf("fixtures/thai-break-corpus/corpus.json does not byte-match buildItems()'s output (same item count, %d, but different content) — run `cd folio-go && go run ./cmd/gencorpus && go run ./cmd/genbreaks` from the module root and commit the result", len(regenerated))
	}
}
