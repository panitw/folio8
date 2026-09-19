package pdf

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// This file lives under internal/pdf, so TestNumberFormattingIsConfinedToNumbersGo
// (emit_source_test.go, D-1.1.b/AD-3) forbids strconv.Itoa/Format*/Append*
// and every fmt formatting call here, with NO _test.go exemption — the
// one file that rule exempts is numbers.go itself. strconv.Atoi is not
// one of the forbidden selectors (it parses, it does not format), so
// this file compares parsed INTEGERS rather than building formatted
// strings.

// TestToUnicodeSectionsAreChunkedAtTheSpecifiedCap is DW-14's direct
// witness. It is REQUIRED because the chunking is otherwise inert: the
// largest ToUnicode section anywhere in the repository is 45 (measured
// at b5eb557), so at cap 100 the chunking branch is never taken by any
// fixture and the whole change could be reverted with the gate green.
//
// It asserts on buildToUnicodeCMap's ACTUAL EMITTED BYTES (parsed back
// out below), never on an intermediate chunking helper's return value —
// so a future refactor that stops calling the chunker (but leaves some
// other helper reporting the "right" answer) cannot leave this green.
//
// Anchor (D-000.68): the 250 entries and the expected [100,100,50]
// split are literals this test owns; nothing here reads
// bfcharSectionCap (textdoc.go) or derives its expectation from it —
// two independent statements, so a wrong cap value cannot pass both
// this test and its own definition.
func TestToUnicodeSectionsAreChunkedAtTheSpecifiedCap(t *testing.T) {
	const n = 250
	face := EmbeddedFace{Name: "probe"}
	for i := 0; i < n; i++ {
		face.ToUnicode = append(face.ToUnicode, CIDText{CID: uint16(i), Text: "a"})
	}
	got, err := buildToUnicodeCMap(face)
	if err != nil {
		t.Fatalf("buildToUnicodeCMap: %v", err)
	}
	re := regexp.MustCompile(`(?s)(\d+) beginbfchar\n(.*?)endbfchar`)
	ms := re.FindAllStringSubmatch(string(got), -1)
	if len(ms) == 0 {
		t.Fatal("no beginbfchar section at all")
	}
	var counted, declared []int
	total := 0
	for _, m := range ms {
		c := strings.Count(m[2], "\n")
		total += c
		counted = append(counted, c)
		d, aerr := strconv.Atoi(m[1])
		if aerr != nil {
			t.Fatalf("section declares a non-numeric count %q: %v", m[1], aerr)
		}
		declared = append(declared, d)
	}
	t.Logf("entries=%d sections=%d declared=%v counted=%v", n, len(ms), declared, counted)
	if len(ms) != 3 {
		t.Errorf("want 3 sections for %d entries, got %d", n, len(ms))
	}
	for i := range ms {
		if declared[i] != counted[i] {
			t.Errorf("section %d declares %d but carries %d entries", i, declared[i], counted[i])
		}
		if counted[i] > 100 {
			t.Errorf("section %d carries %d entries, over the 100 cap", i, counted[i])
		}
	}
	if total != n {
		t.Errorf("sections carry %d entries in total, want %d", total, n)
	}
	// Test-owned literal split, independent of the emitter's own
	// counting above (D-000.68/D-000.9): 250 entries at cap 100 MUST
	// split as exactly [100, 100, 50], not merely "3 sections totalling
	// 250".
	wantDeclared := []int{100, 100, 50}
	if len(declared) != len(wantDeclared) {
		t.Fatalf("declared section sizes = %v, want %v", declared, wantDeclared)
	}
	for i := range wantDeclared {
		if declared[i] != wantDeclared[i] {
			t.Errorf("declared section sizes = %v, want %v", declared, wantDeclared)
		}
	}
}

// capForTest is THIS TEST's own literal for the cap (D-000.68/D-000.9,
// same discipline as tounicode_corpus_test.go's bfcharCapForTest): it
// does NOT read internal/pdf.bfcharSectionCap, so a production value
// that quietly changed could not still agree with this test — the two
// are independent statements of the same number, unlike a check of the
// form `c > bfcharSectionCap`, which holds for ANY value the constant
// takes and so cannot catch the cap itself being wrong.
const capForTest = 100

// TestToUnicodeSectionCapBoundaryValues is DW-14's boundary-value
// check on buildToUnicodeCMap itself, over SYNTHETIC faces this file
// constructs (just under, at, and over the cap, plus D1's own measured
// real-corpus maximum of 45) — a unit-level complement to
// TestToUnicodeSectionsAreChunkedAtTheSpecifiedCap above and to
// package folio8's TestNoRealToUnicodeSectionExceedsTheCap, which runs
// this same property over the REAL shipped fixture corpus (this
// package has no fixtures/no renderer of its own to draw one from).
//
// Anti-vacuity (D-000.9): it reports the OBSERVED MAXIMUM and the
// number of faces examined, via t.Logf, every run — not merely "no
// violation".
func TestToUnicodeSectionCapBoundaryValues(t *testing.T) {
	faces := toUnicodeCorpusFacesForTest(t)
	if len(faces) == 0 {
		t.Fatal("presence precondition: the corpus produced zero faces to examine — this test would prove nothing")
	}
	observedMax := 0
	examined := 0
	re := regexp.MustCompile(`(?s)(\d+) beginbfchar\n(.*?)endbfchar`)
	for _, face := range faces {
		examined++
		got, err := buildToUnicodeCMap(face)
		if err != nil {
			t.Fatalf("buildToUnicodeCMap(%s): %v", face.Name, err)
		}
		ms := re.FindAllStringSubmatch(string(got), -1)
		for _, m := range ms {
			c := strings.Count(m[2], "\n")
			if c > observedMax {
				observedMax = c
			}
			if c > capForTest {
				t.Errorf("face %q emits a /ToUnicode section with %d entries, over the %d cap", face.Name, c, capForTest)
			}
		}
	}
	t.Logf("DW-14 corpus witness: examined %d face(s), observed maximum /ToUnicode section size = %d (cap %d)", examined, observedMax, capForTest)
}

// toUnicodeCorpusFacesForTest builds the EmbeddedFace population
// TestToUnicodeSectionCapBoundaryValues walks: one synthetic small face
// (a stand-in for the shipped fixtures' own faces, all of which
// measure at or under 45 entries — D1), one just under the cap, one at
// it, and one just over it, so the corpus itself spans the boundary
// the property must hold across.
func toUnicodeCorpusFacesForTest(t *testing.T) []EmbeddedFace {
	t.Helper()
	mk := func(name string, n int) EmbeddedFace {
		f := EmbeddedFace{Name: name}
		for i := 0; i < n; i++ {
			f.ToUnicode = append(f.ToUnicode, CIDText{CID: uint16(i), Text: "x"})
		}
		return f
	}
	return []EmbeddedFace{
		mk("corpus-small-45", 45), // D1's own measured maximum today
		mk("corpus-just-under-cap-99", 99),
		mk("corpus-at-cap-100", 100),
		mk("corpus-over-cap-101", 101),
		mk("corpus-large-250", 250),
	}
}
