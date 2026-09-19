package pdf

import (
	"bytes"
	"testing"
)

// TestComputeIDIsContentDerived is AC10's derivation half: /ID must be
// the SHA-256 of the serialized body, not a constant. Two probes in this
// story's QA review (Major 13) showed the derivation was correct but
// unasserted: computeID returning a constant 16 zero bytes, and computeID
// digesting a different byte range, both passed every existing test —
// only the golden hash (a whole-document fingerprint that Story 1.4 will
// deliberately re-record) noticed. This test calls computeID directly on
// two different inputs and asserts the digests differ, independent of
// the golden fixture.
func TestComputeIDIsContentDerived(t *testing.T) {
	a := computeID([]byte("hello"))
	b := computeID([]byte("hello world"))
	if a == b {
		t.Fatalf("computeID returned the same digest for different inputs: %q", a)
	}
	if len(a) != 32 {
		t.Errorf("computeID digest is %d hex characters, want 32 (16 bytes)", len(a))
	}
}

// TestSerializeIDMatchesReDerivedDigest re-derives the SHA-256 over
// Serialize()'s own pre-"/ID" byte prefix and compares it against the
// /ID entry Serialize() actually wrote — the same check this story's QA
// review ran by hand in Python, expressed here as a Go test that runs on
// every future change (this story's QA review, Major 13's suggested
// resolution).
func TestSerializeIDMatchesReDerivedDigest(t *testing.T) {
	out := Serialize()

	idIdx := bytes.Index(out, []byte("/ID ["))
	if idIdx == -1 {
		t.Fatal("no /ID array found in Serialize() output")
	}
	prefix := out[:idIdx]
	want := computeID(prefix)

	rest := out[idIdx+len("/ID ["):]
	got, _, ok := extractAngleBracketedForTest(rest)
	if !ok {
		t.Fatal("could not parse the first /ID entry")
	}
	if got != want {
		t.Errorf("/ID entry %q does not match the SHA-256 re-derived from Serialize()'s own pre-/ID prefix (%q)", got, want)
	}
}

// extractAngleBracketedForTest parses one "<...>"-delimited token from
// the start of b. Duplicated (rather than shared) with render_test.go's
// copy in package folio8 — Go test helpers do not cross package
// boundaries, and internal/pdf is not importable in reverse from there.
func extractAngleBracketedForTest(b []byte) (content string, rest []byte, ok bool) {
	if len(b) == 0 || b[0] != '<' {
		return "", b, false
	}
	end := bytes.IndexByte(b, '>')
	if end == -1 {
		return "", b, false
	}
	return string(b[1:end]), b[end+1:], true
}
