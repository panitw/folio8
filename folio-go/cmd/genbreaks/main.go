// Command genbreaks computes AD-25's CONSTRAINED break opportunities
// over the committed corpus (fixtures/thai-break-corpus/corpus.json)
// and writes them to fixtures/thai-break-corpus/computed_breaks.json —
// Story 2.1's AC10 S4-basis fixture (D-2.1.1).
//
// Ordering is load-bearing (Trap 1): the hand review (this story's dev
// record, corpus_test.go's P1/P2/P3 measurement) happens FIRST, against
// this exact computation; THEN this file is checked in; only THEN does
// an ordinary test (s4_test.go) assert every target reproduces it
// byte-for-byte. This fixture is a cross-target REGRESSION anchor
// only — it certifies "every target computes the same breaks", never
// "the breaks are right" (that is P1-P3, measured separately and
// reported in the spike report, never derived from this file).
//
// Run from the folio-go module root: go run ./cmd/genbreaks
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/panitw/folio8/folio-go/internal/text"
)

type corpusItem struct {
	ID              string   `json:"id"`
	Category        string   `json:"category"`
	Text            string   `json:"text"`
	ProperNounSpans [][2]int `json:"properNounSpans,omitempty"`
}

func main() {
	corpusPath := filepath.Join("..", "fixtures", "thai-break-corpus", "corpus.json")
	b, err := os.ReadFile(corpusPath)
	if err != nil {
		panic(fmt.Sprintf("read corpus: %v", err))
	}
	var items []corpusItem
	if err := json.Unmarshal(b, &items); err != nil {
		panic(fmt.Sprintf("unmarshal corpus: %v", err))
	}

	dict := text.Dictionary()
	result := make(map[string][]int, len(items))
	for _, it := range items {
		breaks, _ := text.ComputeBreaks(dict, it.Text, false)
		if breaks == nil {
			breaks = []int{}
		}
		result[it.ID] = breaks
	}

	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(err)
	}
	outPath := filepath.Join("..", "fixtures", "thai-break-corpus", "computed_breaks.json")
	if err := os.WriteFile(outPath, append(out, '\n'), 0o644); err != nil {
		panic(fmt.Sprintf("write computed_breaks.json: %v", err))
	}
	fmt.Fprintf(os.Stderr, "genbreaks: wrote %s (%d items)\n", outPath, len(result))
}
