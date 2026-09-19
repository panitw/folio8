package text

import (
	"bytes"
	"encoding/binary"
	"maps"
	"slices"
)

// BytesTrie is a compact, byte-oriented trie over UTF-8-encoded
// dictionary entries (AC1). It is built once, offline, by CompileTrie,
// serialized deterministically, committed, and embedded via go:embed
// (data.go) — nothing in this file ever touches a filesystem, and
// nothing here allocates a node graph at query time: BytesTrie.buf IS
// the trie, queried directly.
//
// Wire format (all integers unsigned LEB128 "varint", per
// encoding/binary.Uvarint; depth-first, deterministic child order):
//
//	Node    := flags(1 byte) numChildren(varint) Child*
//	Child   := edge(1 byte) size(varint) Node   // size = byte length of the nested Node
//	flags   := bit 0 set iff this node marks the end of a dictionary word
//
// Children are always serialized in ascending edge-byte order, so two
// compiles of the same word set produce byte-identical output (Trap 2 /
// task 3: the compiled artifact is committed and hashed).
type BytesTrie struct {
	buf []byte
}

// DecodeBytesTrie wraps an already-compiled trie for querying. It does
// not copy or validate buf beyond what querying naturally touches — buf
// is expected to be exactly CompileTrie's output (typically the
// go:embed'd data/thai_words.trie).
func DecodeBytesTrie(buf []byte) *BytesTrie {
	return &BytesTrie{buf: buf}
}

// readNode reads the node header at byte offset pos and returns whether
// it marks a word end, the number of children, and the offset of the
// first child entry.
func (t *BytesTrie) readNode(pos int) (isWord bool, numChildren uint64, childrenStart int) {
	flags := t.buf[pos]
	n, sz := binary.Uvarint(t.buf[pos+1:])
	return flags&1 != 0, n, pos + 1 + sz
}

// findChild scans numChildren child entries starting at childrenStart
// looking for edge byte target, returning the offset of that child's
// nested Node (its header) if found.
func (t *BytesTrie) findChild(childrenStart int, numChildren uint64, target byte) (childPos int, found bool) {
	p := childrenStart
	for c := uint64(0); c < numChildren; c++ {
		edge := t.buf[p]
		p++
		size, sz := binary.Uvarint(t.buf[p:])
		p += sz
		if edge == target {
			return p, true
		}
		p += int(size)
	}
	return 0, false
}

// Contains reports whether word is an exact entry in the trie.
func (t *BytesTrie) Contains(word string) bool {
	if len(t.buf) == 0 {
		return false
	}
	pos := 0
	s := []byte(word)
	for i := 0; i < len(s); i++ {
		_, numChildren, childrenStart := t.readNode(pos)
		childPos, found := t.findChild(childrenStart, numChildren, s[i])
		if !found {
			return false
		}
		pos = childPos
	}
	isWord, _, _ := t.readNode(pos)
	return isWord
}

// LongestMatch returns the byte length of the longest dictionary entry
// that is a prefix of s (0 if none, including the case where s itself
// is empty or shares no prefix with any entry).
func (t *BytesTrie) LongestMatch(s []byte) int {
	if len(t.buf) == 0 {
		return 0
	}
	pos := 0
	best := 0
	for i := 0; i <= len(s); i++ {
		isWord, numChildren, childrenStart := t.readNode(pos)
		if isWord {
			best = i
		}
		if i == len(s) {
			break
		}
		childPos, found := t.findChild(childrenStart, numChildren, s[i])
		if !found {
			break
		}
		pos = childPos
	}
	return best
}

// forEachWordPrefix walks s once from the trie root and calls fn with
// the byte length of every dictionary entry that is a prefix of s, in
// ascending length order. It stops as soon as no entry can extend
// further.
//
// It exists so Story 2.4's tileability computation (break.go's
// tileableForward / tileableBackward) costs one walk per start
// position rather than one full Contains per (start, end) pair. The
// naive form is O(n^2) trie descents AND allocates a string per pair;
// this form is O(n * longest-entry) descents and allocates nothing.
// The two agree by construction — both report exactly "is s[:k] an
// entry" — and TestForEachWordPrefixMatchesContains asserts that
// agreement over the shipped trie rather than assuming it.
func (t *BytesTrie) forEachWordPrefix(s []byte, fn func(byteLen int)) {
	if len(t.buf) == 0 {
		return
	}
	pos := 0
	for i := 0; i <= len(s); i++ {
		isWord, numChildren, childrenStart := t.readNode(pos)
		if isWord && i > 0 {
			fn(i)
		}
		if i == len(s) {
			return
		}
		childPos, found := t.findChild(childrenStart, numChildren, s[i])
		if !found {
			return
		}
		pos = childPos
	}
}

// trieNode is the in-memory build-time representation used only by
// CompileTrie. It never leaves this file.
type trieNode struct {
	isWord   bool
	children map[byte]*trieNode
}

func newTrieNode() *trieNode {
	return &trieNode{children: map[byte]*trieNode{}}
}

// CompileTrie builds a BytesTrie's serialized wire format from a set of
// dictionary words. Output is deterministic: duplicate words collapse,
// and children are always written in ascending byte order (Trap 2: the
// artifact is committed and hashed, so two runs over the same input
// must be byte-identical). Standard library only — no dependency is
// added to folio-go's module graph.
func CompileTrie(words []string) []byte {
	root := newTrieNode()
	for _, w := range words {
		cur := root
		s := []byte(w)
		for _, b := range s {
			child, ok := cur.children[b]
			if !ok {
				child = newTrieNode()
				cur.children[b] = child
			}
			cur = child
		}
		cur.isWord = true
	}

	var buf bytes.Buffer
	serializeTrieNode(root, &buf)
	return buf.Bytes()
}

func serializeTrieNode(n *trieNode, out *bytes.Buffer) {
	var flags byte
	if n.isWord {
		flags = 1
	}
	out.WriteByte(flags)

	// AD-1/D-1.3.5 bans ranging a map directly (nondeterministic
	// iteration order) — slices.Sorted(maps.Keys(m)) is the mandated
	// escape hatch (RuleMapRange's EscapeHatch, verbatim), not a
	// `for e := range n.children` loop followed by a sort.
	edges := slices.Sorted(maps.Keys(n.children))

	var numChildren [binary.MaxVarintLen64]byte
	nn := binary.PutUvarint(numChildren[:], uint64(len(edges)))
	out.Write(numChildren[:nn])

	for _, e := range edges {
		var childBuf bytes.Buffer
		serializeTrieNode(n.children[e], &childBuf)

		out.WriteByte(e)
		var sizeBuf [binary.MaxVarintLen64]byte
		sn := binary.PutUvarint(sizeBuf[:], uint64(childBuf.Len()))
		out.Write(sizeBuf[:sn])
		out.Write(childBuf.Bytes())
	}
}
