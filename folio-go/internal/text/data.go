package text

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"sync"
)

// compiledTrie is the BytesTrie's serialized wire format, produced
// offline and deterministically by cmd/gentrie from
// wordlist/words_th.txt, and embedded directly into the binary (AC1).
// This is the ONLY place folio-go/internal/text touches the wordlist:
// nothing here, or anywhere else in this package, reads
// wordlist/words_th.txt at runtime, opens a file, or calls
// runtime.Caller (AC2). go:embed cannot reach outside its own module
// (D-000.5), which is exactly why the wordlist and its compiled trie
// live inside folio-go rather than in repo-root fixtures/ (Trap 3).
//
//go:embed data/thai_words.trie
var compiledTrie []byte

var (
	dictOnce sync.Once
	dict     *BytesTrie
)

// Dictionary returns the process-wide Thai dictionary trie, decoded
// once from the embedded compiled artifact. It never reads from disk
// and never varies by target — js/wasm and native builds embed the
// identical bytes (AC3).
func Dictionary() *BytesTrie {
	dictOnce.Do(func() {
		dict = DecodeBytesTrie(compiledTrie)
	})
	return dict
}

// DictionarySHA256 is a bounded audit witness for the exact embedded trie.
// It exposes no dictionary entries and lets the offline release verifier ask
// the emitted js/wasm engine which compiled representation it actually uses.
func DictionarySHA256() string {
	sum := sha256.Sum256(compiledTrie)
	return hex.EncodeToString(sum[:])
}
