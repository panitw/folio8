// Command gentrie regenerates folio-go/internal/text/data/thai_words.trie
// from folio-go/internal/text/wordlist/words_th.txt (Story 2.1, AC1).
//
// Run from the folio-go module root:
//
//	go run ./cmd/gentrie
//
// This is a build-time tool — it is not part of the render path, is
// not embedded into any shipped binary, and is the only place in this
// module allowed to read the wordlist from disk (D-000.5, Trap 3: the
// embedded artifact is the compiled trie, never the raw wordlist).
package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/panitw/folio8/folio-go/internal/text"
)

func main() {
	const (
		wordlistPath = "internal/text/wordlist/words_th.txt"
		outPath      = "internal/text/data/thai_words.trie"
	)

	f, err := os.Open(wordlistPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gentrie: open %s: %v\n", wordlistPath, err)
		os.Exit(1)
	}
	defer f.Close()

	var words []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		words = append(words, line)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "gentrie: scan %s: %v\n", wordlistPath, err)
		os.Exit(1)
	}

	compiled := text.CompileTrie(words)

	if err := os.WriteFile(outPath, compiled, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "gentrie: write %s: %v\n", outPath, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stdout, "gentrie: %d words -> %s (%d bytes)\n", len(words), outPath, len(compiled))
}
