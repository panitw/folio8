package folio8

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/bind"
	"github.com/panitw/folio8/folio-go/internal/fontset"
	"github.com/panitw/folio8/folio-go/internal/text"
)

// ---------------------------------------------------------------------
// Story 2.3, AC4 / AC7 / AC10 / AC11: fixtures/shaped-text/.
//
// This is the fixture that makes the story observable. Measured at
// creation: EVERY pre-2.3 golden's text shapes to itself, so a correct
// implementation and one that never calls the shaper produce identical
// bytes for all of them. A fifth fixture recorded over equally blind
// text would look like coverage in every report while being unable to
// fail when this story regresses — the "all clear" and the "I could not
// look" produced by the same code path (D-000.9).
//
// AC11 is therefore not decoration: it is the coverage witness for
// AC10, and it is asserted by MEASUREMENT against the naive rune->cmap
// answer, not by inspection of the text.
// ---------------------------------------------------------------------

// pdfObjects splits a classic (uncompressed) PDF into object number ->
// object body. It re-derives structure from the document's own bytes,
// consistent with this package's no-PDF-reading-dependency rule (AC2).
func pdfObjects(t *testing.T, b []byte) map[int][]byte {
	t.Helper()
	objs := map[int][]byte{}
	rest := b
	offset := 0
	for {
		idx := bytes.Index(rest, []byte(" 0 obj\n"))
		if idx == -1 {
			break
		}
		// Walk backwards over the object number's digits.
		start := idx
		for start > 0 && rest[start-1] >= '0' && rest[start-1] <= '9' {
			start--
		}
		num, err := strconv.Atoi(string(rest[start:idx]))
		if err != nil {
			rest = rest[idx+7:]
			offset += idx + 7
			continue
		}
		bodyStart := idx + len(" 0 obj\n")
		end := bytes.Index(rest[bodyStart:], []byte("\nendobj\n"))
		if end == -1 {
			break
		}
		objs[num] = rest[bodyStart : bodyStart+end]
		rest = rest[bodyStart+end:]
		offset += bodyStart + end
	}
	if len(objs) == 0 {
		t.Fatal("could not parse any PDF objects out of the rendered document")
	}
	return objs
}

// streamBody returns the bytes between "stream\n" and "endstream" in an
// object body, and whether the object has one at all (D-000.21
// sharpened: prove the artifact carries the thing before reading it).
func streamBody(obj []byte) ([]byte, bool) {
	i := bytes.Index(obj, []byte(">>\nstream\n"))
	if i == -1 {
		return nil, false
	}
	start := i + len(">>\nstream\n")
	j := bytes.Index(obj[start:], []byte("endstream"))
	if j == -1 {
		return nil, false
	}
	return obj[start : start+j], true
}

// objRefAfter returns the object number of an indirect reference
// following key in obj (e.g. "/ToUnicode 7 0 R").
func objRefAfter(obj []byte, key string) (int, bool) {
	i := bytes.Index(obj, []byte(key))
	if i == -1 {
		return 0, false
	}
	after := obj[i+len(key):]
	sp := bytes.Index(after, []byte(" 0 R"))
	if sp == -1 {
		return 0, false
	}
	num, err := strconv.Atoi(strings.TrimSpace(string(after[:sp])))
	if err != nil {
		return 0, false
	}
	return num, true
}

// resourceType0Objects maps each PDF font RESOURCE NAME to its Type0
// font object, read off the produced document's page resource
// dictionaries — EVERY page object's, not an arbitrary one. Factored out
// so that every per-face reader below — the /ToUnicode CMap and the
// /CIDToGIDMap — walks the SAME path to the same object rather than each
// re-deriving it.
//
// Repaired by Story 2.6a (AC7): before the repair this walked pdfObjects'
// map in undefined iteration order, took the FIRST object matching
// "/Type /Page " that carried a /Font resource dictionary, and stopped —
// silently discarding every other page's font resources. On a document
// where two pages use different faces, this would read one page's fonts
// and report them as the whole document's.
//
// D-000.50's answer (corrected by the review, Finding 7): the reason no
// COMMITTED GOLDEN expresses this is not merely that fixtures/multi-page/'s
// two page objects happen to carry a byte-identical
// `/Font << /NotoSans 3 0 R >>` — it is that no document THIS ENGINE CAN
// EMIT ever could. internal/pdf/textdoc.go collects the face set
// DOCUMENT-GLOBALLY (`faceNames := slices.Sorted(maps.Keys(faces))`,
// textdoc.go:130), rejects an escaped-name collision across the WHOLE
// document up front (textdoc.go:140-143), and then writes that identical
// `/Font << … >>` dictionary, in the same sorted order, into EVERY page
// object (textdoc.go:217). Two pages with different fonts is not a
// fixture this repo lacks; it is a document shape the serializer cannot
// produce until it starts writing per-page font dictionaries.
//
// Despite that, this guard is PROVEN, not forward (D-000.50, D-000.24):
// TestResourceType0ObjectsResolvesFontsFromEveryPageObject below builds an
// in-memory literal — not a fixture, not an emitted byte — carrying two
// page objects with two DIFFERENT font resource names, which the
// pre-repair "stop at the first" code demonstrably cannot resolve both of
// (red-proofed by reverting to that shape; the reverted test fails). A
// synthetic subject that expresses a defect is an available red-proof
// under D-000.50 whether or not any committed golden also expresses it.
//
// Shape note, not a live defect (Finding 7): the repaired helper is
// page-COMPLETE (it now visits every page) but not page-AWARE — it merges
// every page's dict into one map[string][]byte, so two pages mapping the
// same resource NAME to two DIFFERENT objects would still be
// last-writer-wins, silently. That cannot happen today for the same
// serializer reason above (names are assigned and collision-checked
// globally), so it is recorded here for whoever changes the serializer's
// per-page assumption next, not repaired now.
//
// Every step fails LOUDLY if an object it needs is absent (D-000.21
// sharpened). A missing object must not read as "no discrepancy found".
func resourceType0Objects(t *testing.T, b []byte) (map[int][]byte, map[string][]byte) {
	t.Helper()
	objs := pdfObjects(t, b)

	nums := make([]int, 0, len(objs))
	for num := range objs {
		nums = append(nums, num)
	}
	sort.Ints(nums)

	// Every page object carries /Resources << /Font << /Name N 0 R ... >>.
	// Collect all of them, in deterministic ascending object-number order,
	// rather than stopping at the first.
	var fontDicts [][]byte
	for _, num := range nums {
		obj := objs[num]
		if !bytes.Contains(obj, []byte("/Type /Page ")) {
			continue
		}
		i := bytes.Index(obj, []byte("/Font <<"))
		if i == -1 {
			continue
		}
		after := obj[i+len("/Font <<"):]
		j := bytes.Index(after, []byte(">>"))
		if j == -1 {
			t.Fatal("page's /Font resource dictionary is unterminated")
		}
		fontDicts = append(fontDicts, after[:j])
	}
	if len(fontDicts) == 0 {
		t.Fatal("produced PDF carries no page /Font resource dictionary — every assertion about fonts below would be vacuous")
	}

	out := map[string][]byte{}
	for _, fontDict := range fontDicts {
		fields := strings.Fields(string(fontDict))
		for i := 0; i < len(fields); i++ {
			if !strings.HasPrefix(fields[i], "/") {
				continue
			}
			if i+3 >= len(fields) || fields[i+3] != "R" {
				continue
			}
			name := strings.TrimPrefix(fields[i], "/")
			num, err := strconv.Atoi(fields[i+1])
			if err != nil {
				continue
			}
			type0, ok := objs[num]
			if !ok {
				t.Fatalf("resource /%s names object %d, which is not present", name, num)
			}
			out[name] = type0
		}
	}
	if len(out) == 0 {
		t.Fatal("no font resources resolved to a Type0 font object")
	}
	return objs, out
}

// TestResourceType0ObjectsResolvesFontsFromEveryPageObject is the Story
// 2.6a review's Finding 6 red-proof, committed rather than discarded
// (D-000.24, D-000.50): the story's own Delivery Log states discriminating
// power was "proven with a throwaway synthetic two-page subject carrying
// two different font resource names ... The probe was deleted after the
// run." D-000.50 asks whether ANY subject can express the defect, not
// whether a COMMITTED GOLDEN can — an in-memory literal is not an emitted
// byte and needs no fixture, so there was never a reason to discard it.
//
// The buffer below is not a valid PDF — pdfObjects and this function only
// ever scan for "N 0 obj\n" ... "\nendobj\n" markers and a
// "/Type /Page " substring; neither needs a header, xref, or trailer — so
// this costs no fixture and moves no emitted bytes.
func TestResourceType0ObjectsResolvesFontsFromEveryPageObject(t *testing.T) {
	buf := []byte(
		"1 0 obj\n<< /Type /Page /Resources << /Font << /FaceA 10 0 R >> >> >>\nendobj\n" +
			"2 0 obj\n<< /Type /Page /Resources << /Font << /FaceB 20 0 R >> >> >>\nendobj\n" +
			"10 0 obj\n<< /Type /Font /Subtype /Type0 /BaseFont /FaceA >>\nendobj\n" +
			"20 0 obj\n<< /Type /Font /Subtype /Type0 /BaseFont /FaceB >>\nendobj\n")

	_, byName := resourceType0Objects(t, buf)

	if len(byName) != 2 {
		t.Fatalf("resourceType0Objects resolved %d font resource(s) across two page objects with DIFFERENT resource names, want 2 (one per page)", len(byName))
	}
	faceA, ok := byName["FaceA"]
	if !ok {
		t.Fatal("resourceType0Objects did not resolve page 1's /FaceA resource")
	}
	if !bytes.Contains(faceA, []byte("/BaseFont /FaceA")) {
		t.Fatalf("resourceType0Objects's /FaceA resolved to the wrong object: %q", faceA)
	}
	faceB, ok := byName["FaceB"]
	if !ok {
		t.Fatal("resourceType0Objects did not resolve page 2's /FaceB resource — this is exactly the pre-repair defect: stopping at the FIRST /Type /Page object carrying a /Font dict silently discards every other page's fonts")
	}
	if !bytes.Contains(faceB, []byte("/BaseFont /FaceB")) {
		t.Fatalf("resourceType0Objects's /FaceB resolved to the wrong object: %q", faceB)
	}
}

// cidToGlyphForResources maps each PDF font RESOURCE NAME to the
// CID -> SUBSET GLYPH ID function that face declares, read off the
// produced document's own /CIDToGIDMap.
//
// This is the indirection Story 2.3 introduced and it exists because
// CID and subset glyph id are NO LONGER the same number: the base block
// CIDs 0..NumGlyphs-1 are the identity, and each extra CID above that
// points back at a glyph already in the base block. A test that compares
// an emitted CID against a subset glyph id is therefore only accidentally
// right — right for as long as the glyph it names has no second
// (glyph, cluster text) pair (Story 2.3 finisher, Finding 11).
//
// Reading the mapping off the ARTIFACT rather than recomputing it is
// D-000.21 sharpened applied properly: the /CIDToGIDMap is the object
// that carries the property, and it is what a viewer will actually obey.
func cidToGlyphForResources(t *testing.T, b []byte) map[string]func(uint16) uint16 {
	t.Helper()
	objs, type0s := resourceType0Objects(t, b)

	out := map[string]func(uint16) uint16{}
	for name, type0 := range type0s {
		descNum, ok := objRefAfter(type0, "/DescendantFonts [")
		if !ok {
			t.Fatalf("resource /%s's Type0 font names no descendant CIDFont", name)
		}
		cidFont, ok := objs[descNum]
		if !ok {
			t.Fatalf("resource /%s's /DescendantFonts names object %d, which is not present", name, descNum)
		}
		if !bytes.Contains(cidFont, []byte("/CIDToGIDMap ")) {
			t.Fatalf("resource /%s's CIDFont declares no /CIDToGIDMap, so no CID can be resolved to a glyph", name)
		}
		if bytes.Contains(cidFont, []byte("/CIDToGIDMap /Identity")) {
			// No glyph in this face needed a second CID, so the base
			// block IS the whole CID space and the map is the identity.
			out[name] = func(cid uint16) uint16 { return cid }
			continue
		}
		mapNum, ok := objRefAfter(cidFont, "/CIDToGIDMap ")
		if !ok {
			t.Fatalf("resource /%s's /CIDToGIDMap is neither /Identity nor an object reference", name)
		}
		mapObj, ok := objs[mapNum]
		if !ok {
			t.Fatalf("resource /%s's /CIDToGIDMap names object %d, which is not present", name, mapNum)
		}
		stream, ok := streamBody(mapObj)
		if !ok {
			t.Fatalf("resource /%s's /CIDToGIDMap object %d carries no stream", name, mapNum)
		}
		if len(stream) == 0 || len(stream)%2 != 0 {
			t.Fatalf("resource /%s's /CIDToGIDMap stream is %d bytes, which is not a non-empty array of 2-byte glyph ids", name, len(stream))
		}
		table := make([]uint16, len(stream)/2)
		for i := range table {
			table[i] = uint16(stream[2*i])<<8 | uint16(stream[2*i+1])
		}
		out[name] = func(cid uint16) uint16 {
			if int(cid) >= len(table) {
				t.Fatalf("resource /%s: CID %d is beyond its %d-entry /CIDToGIDMap", name, cid, len(table))
			}
			return table[cid]
		}
	}
	return out
}

// toUnicodeForResources maps each PDF font RESOURCE NAME to its
// /ToUnicode CMap, read off the produced document: the page's resource
// dictionary names the Type0 font object, which names the CMap stream.
func toUnicodeForResources(t *testing.T, b []byte) map[string]map[uint16]string {
	t.Helper()
	objs, type0s := resourceType0Objects(t, b)

	out := map[string]map[uint16]string{}
	for name, type0 := range type0s {
		cmapNum, ok := objRefAfter(type0, "/ToUnicode ")
		if !ok {
			t.Fatalf("resource /%s's Type0 font declares no /ToUnicode — the text in this document is not extractable and no round-trip assertion below can mean anything", name)
		}
		cmapObj, ok := objs[cmapNum]
		if !ok {
			t.Fatalf("resource /%s's /ToUnicode names object %d, which is not present", name, cmapNum)
		}
		stream, ok := streamBody(cmapObj)
		if !ok {
			t.Fatalf("resource /%s's /ToUnicode object %d carries no stream", name, cmapNum)
		}
		out[name] = parseBfChar(t, name, stream)
	}
	if len(out) == 0 {
		t.Fatal("no font resources resolved to a /ToUnicode CMap")
	}
	return out
}

// parseBfChar reads a ToUnicode CMap's bfchar entries into
// CID -> extracted text. An empty destination string (<>) is a REAL
// entry meaning "this glyph contributes no text", and is kept as such —
// collapsing it with "absent" would make AC7's round-trip untestable.
func parseBfChar(t *testing.T, label string, stream []byte) map[uint16]string {
	t.Helper()
	i := bytes.Index(stream, []byte("beginbfchar\n"))
	if i == -1 {
		t.Fatalf("%s: /ToUnicode stream has no beginbfchar section", label)
	}
	j := bytes.Index(stream, []byte("endbfchar"))
	if j == -1 {
		t.Fatalf("%s: /ToUnicode stream has no endbfchar", label)
	}
	out := map[uint16]string{}
	for _, line := range strings.Split(string(stream[i+len("beginbfchar\n"):j]), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			t.Fatalf("%s: unparseable bfchar line %q", label, line)
		}
		cid64, err := strconv.ParseUint(strings.Trim(parts[0], "<>"), 16, 16)
		if err != nil {
			t.Fatalf("%s: unparseable bfchar CID %q", label, parts[0])
		}
		hexText := strings.Trim(parts[1], "<>")
		var sb strings.Builder
		for k := 0; k+4 <= len(hexText); k += 4 {
			v, err := strconv.ParseUint(hexText[k:k+4], 16, 16)
			if err != nil {
				t.Fatalf("%s: unparseable bfchar destination %q", label, parts[1])
			}
			sb.WriteRune(rune(v))
		}
		out[uint16(cid64)] = sb.String()
	}
	return out
}

// emittedRun is one BT..ET block read back off the produced content
// stream: the font resource it selected, the x origin its text matrix
// placed it at, the CIDs it showed in order, every TJ adjustment it
// carried, and whether it used TJ with any non-zero adjustment.
type emittedRun struct {
	Resource string
	// OriginXMilli is the run's Tm x translation, in MILLIPOINTS —
	// the same unit geom.Length carries, recovered from the decimal
	// point-valued operand so a test can compare segment origins
	// against hand-derived numbers (Story 2.3 finisher, Blocker 1).
	OriginXMilli int64
	// OriginYMilli is the run's Tm y translation, in MILLIPOINTS.
	// Story 2.4 reads it to count the LINES an element occupies: every
	// run on one line shares a y, and a wrapped element emits one
	// distinct y per line.
	OriginYMilli  int64
	CIDs          []uint16
	Adjustments   []int64
	UsedTJ        bool
	NonZeroAdjust bool
}

// tmOriginXMilli recovers the x translation, in millipoints, from the
// operands preceding a ` Tm` operator (`1 0 0 1 <x> <y>`). The operands
// are point-valued decimals written by internal/pdf's appendLength, so
// the inverse is "integer part * 1000 + zero-padded fraction" — parsed
// exactly, never through a float, since the whole module's premise is
// that no number on an output path is ever floating point (AD-2/AD-23).
func tmOriginXMilli(t *testing.T, operands string) int64 {
	t.Helper()
	return tmOperandMilli(t, operands, 2)
}

// tmOperandMilli is tmOriginXMilli generalised over which operand to
// read: fromEnd == 2 is x, fromEnd == 1 is y, in `1 0 0 1 <x> <y> Tm`.
func tmOperandMilli(t *testing.T, operands string, fromEnd int) int64 {
	t.Helper()
	fields := strings.Fields(operands)
	if len(fields) < 6 {
		t.Fatalf("text matrix has %d operands, want 6: %q", len(fields), operands)
	}
	raw := fields[len(fields)-fromEnd]
	neg := strings.HasPrefix(raw, "-")
	raw = strings.TrimPrefix(raw, "-")
	intPart, fracPart, _ := strings.Cut(raw, ".")
	whole, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		t.Fatalf("unparseable Tm operand %q in %q: %v", raw, operands, err)
	}
	var frac int64
	if fracPart != "" {
		for len(fracPart) < 3 {
			fracPart += "0"
		}
		if len(fracPart) > 3 {
			t.Fatalf("Tm operand %q carries more than millipoint precision", raw)
		}
		frac, err = strconv.ParseInt(fracPart, 10, 64)
		if err != nil {
			t.Fatalf("unparseable Tm operand fraction %q: %v", fracPart, err)
		}
	}
	v := whole*1000 + frac
	if neg {
		v = -v
	}
	return v
}

// readEmittedRuns parses EVERY text-bearing content stream in the document
// back into runs, in deterministic ASCENDING OBJECT-NUMBER order, and
// concatenates them via parseContentStreamRuns — the exact per-stream
// parser this function itself uses, unchanged (D-000.34: the seam stays
// per-stream; this function is what walks it across every stream).
//
// Repaired by Story 2.6a (D-2.6.9). Before the repair this function read
// only the FIRST text-bearing stream pdfObjects' map iteration happened to
// produce — which, because pdfObjects returns a map and Go randomises map
// iteration order, was not even deterministic: measured over 20 calls
// against fixtures/multi-page/expected.pdf, it returned 9 runs on some
// calls and 24 on others, against 33 actually present, with no fatal and
// no warning either way (Story 2.6a, Measured findings 1 — the red-proof).
// On every fixture that carries exactly one text-bearing stream — 11 of
// this function's call sites, at the finishing commit (Story 2.6a review
// Finding 5's correction: the pre-fix count of "12 call sites, 11 of them
// single-stream" undercounted by exactly the sites this REPAIR itself
// created, see below) — "first stream found" and "the only stream" are
// the same value, which is exactly why the defect was invisible until a
// genuinely multi-page subject existed to run this helper against.
//
// The other 3 call sites render the multi-page fixture and genuinely
// depend on every-stream reading: matrix_test.go's
// requireMultiPageIsGenuinelyMultiPage (added by the Story 2.6 finisher
// against the now-retired sibling readEmittedRunsAllPages, re-pointed
// here by this story's AC5), and this file's own
// TestReadEmittedRunsMatchesThePerPageStreamSplitOnAMultiPageDocument and
// TestReadEmittedRunsIsDeterministicAcrossRepeatedCallsOnAMultiPageDocument
// (committed by this story's finisher to close Finding 1 / Blocker 1 —
// the repair shipped with no test that could see a reversion).
//
// ascending object number is page order for this fixture's emitter:
// content objects are reserved one per page, in page order, after every
// page object (render.go's object-reservation order; see also the
// now-retired sibling readEmittedRunsAllPages this function absorbed,
// matrix_test.go, Story 2.6 finisher).
func readEmittedRuns(t *testing.T, b []byte) []emittedRun {
	t.Helper()
	objs := pdfObjects(t, b)

	nums := make([]int, 0, len(objs))
	for num := range objs {
		nums = append(nums, num)
	}
	sort.Ints(nums)

	var runs []emittedRun
	streams := 0
	for _, num := range nums {
		s, ok := streamBody(objs[num])
		if !ok || !bytes.Contains(s, []byte(" Tf\n")) {
			continue
		}
		streams++
		runs = append(runs, parseContentStreamRuns(t, s)...)
	}
	if streams == 0 {
		t.Fatal("produced PDF carries no text content stream")
	}
	if len(runs) == 0 {
		t.Fatal("content stream carries no text runs")
	}
	return runs
}

// TestReadEmittedRunsMatchesThePerPageStreamSplitOnAMultiPageDocument is
// the Story 2.6a review's Blocker 1 red-proof, committed: AC4 requires
// readEmittedRuns' repair be "verified by a test that RUNS the repaired
// helper against the multi-page subject, not by reading it", and the
// story as delivered shipped only a deleted throwaway probe (a
// measurement — 200 calls, histogram map[33:200] — never committed as an
// assertion). This test renders the multi-page fixture directly
// (renderMultiPage, multi_page_fixture_test.go — untagged, package
// folio8, costs no new fixture and no new emitted bytes) and cross-checks
// readEmittedRuns' total run count against an INDEPENDENTLY derived
// total: splitPageContentStreams walks the page tree's /Kids array and
// each page object's OWN /Contents reference (it never touches
// pdfObjects' " Tf\n" stream scan readEmittedRuns itself uses), and
// parseContentStreamRuns re-parses each of those streams the same
// per-stream way readEmittedRuns does internally. Two independently
// reached traversals agreeing is a materially stronger claim than a
// single hard-coded expected count (D-2.6.8: construction and the guard
// that verifies it are different properties, and both belong).
//
// Red-proof: reverting readEmittedRuns to `break` after the first
// Tf-bearing stream (this story's own baseline defect) makes this test
// fail — the multi-page document carries two text-bearing content
// streams, so the helper's total falls to one page's worth while the
// independent per-page total stays at both.
func TestReadEmittedRunsMatchesThePerPageStreamSplitOnAMultiPageDocument(t *testing.T) {
	raw := renderMultiPage(t)

	got := readEmittedRuns(t, raw)

	streams := splitPageContentStreams(t, raw)
	if len(streams) != mpTotalPages {
		t.Fatalf("splitPageContentStreams returned %d page content stream(s), want %d", len(streams), mpTotalPages)
	}
	var want int
	for _, s := range streams {
		want += len(parseContentStreamRuns(t, []byte(s)))
	}
	if want == 0 {
		t.Fatal("the independent per-page split+parse found ZERO runs on the multi-page document — this test would pass vacuously against a helper that also returns zero; that is not what is being measured here")
	}

	if len(got) != want {
		t.Fatalf("readEmittedRuns returned %d run(s) on the multi-page document; the independent per-page split+parse totals %d across its %d pages — readEmittedRuns must scan EVERY text-bearing stream, not an arbitrary one",
			len(got), want, mpTotalPages)
	}
}

// TestReadEmittedRunsIsDeterministicAcrossRepeatedCallsOnAMultiPageDocument
// is the determinism half of the Story 2.6a review's Blocker 1 (D-2.6.8:
// construction and guard are different properties and both belong).
// Before this story's repair, readEmittedRuns walked pdfObjects' map in
// UNDEFINED Go map iteration order and returned as soon as it hit the
// first Tf-bearing stream — so on fixtures/multi-page/expected.pdf,
// measured over 20 calls in one process, it silently returned 9 runs on
// some calls and 24 on others (histogram map[9:4 24:16], against 33
// actually present), with no fatal and no warning either way (Measured
// findings 1). The repair sorts object numbers before iterating and reads
// EVERY text-bearing stream rather than stopping at one, so repeated
// calls must now always agree.
//
// Repetition count: 50, chosen to comfortably exceed the 20 calls that
// empirically produced BOTH branches under the pre-repair code (Measured
// findings 1). Go randomizes map iteration order independently on each
// `range`, so each of those 20 calls behaved close to an independent
// coin flip between the two branches; 50 independent calls all landing on
// the same branch by chance alone, were this test run against the
// unrepaired helper, has probability on the order of 2^-49 — negligible,
// not merely small — so this reliably detects a reversion rather than
// merely making one plausible to catch.
func TestReadEmittedRunsIsDeterministicAcrossRepeatedCallsOnAMultiPageDocument(t *testing.T) {
	raw := renderMultiPage(t)

	const calls = 50
	histogram := map[int]int{}
	for i := 0; i < calls; i++ {
		n := len(readEmittedRuns(t, raw))
		histogram[n]++
	}
	if n, ok := histogram[0]; ok && n > 0 {
		t.Fatal("readEmittedRuns returned zero runs on at least one call — this test would pass vacuously if every call returned zero")
	}
	if len(histogram) != 1 {
		t.Fatalf("readEmittedRuns returned inconsistent run counts across %d calls in one process: %v — it must return the SAME count on every call", calls, histogram)
	}
}

// parseContentStreamRuns parses ONE content stream's text-showing operators
// into runs. Factored out of readEmittedRuns, byte-for-byte the same logic
// that lived inline there, so a multi-page-aware caller can parse several
// streams the identical way and concatenate them rather than duplicating
// this ~70-line parser (Story 2.6 finisher, Blocker 1). This extraction
// changes NOTHING about what readEmittedRuns itself does or returns.
func parseContentStreamRuns(t *testing.T, content []byte) []emittedRun {
	t.Helper()
	var runs []emittedRun
	for _, block := range strings.Split(string(content), "BT\n")[1:] {
		end := strings.Index(block, "ET\n")
		if end == -1 {
			t.Fatalf("unterminated BT block: %q", block)
		}
		block = block[:end]

		tf := strings.Index(block, " Tf\n")
		if tf == -1 {
			t.Fatalf("BT block selects no font: %q", block)
		}
		resource := strings.TrimPrefix(strings.Fields(block[:tf])[0], "/")

		tm := strings.Index(block, " Tm\n")
		run := emittedRun{
			Resource:     resource,
			OriginXMilli: tmOperandMilli(t, block[:tm], 2),
			OriginYMilli: tmOperandMilli(t, block[:tm], 1),
		}
		body := block[tm+len(" Tm\n"):]
		run.UsedTJ = strings.Contains(body, "] TJ")

		// scanAdjustments reads every bare integer out of one span of
		// the TJ array. Split out of the loop below because it must ALSO
		// run once after it: the number emitted after the FINAL `>` and
		// before `] TJ` is real output — appendShapedRun writes
		// adjustments[len(run.Glyphs)] there, the term that makes a
		// run's total advance equal the shaped advance rather than the
		// sum of its /W widths — and the `<`-driven loop can never reach
		// it (Story 2.3 finisher, Finding 5).
		//
		// IT KNOWS ABOUT `Ts` (Story 8.0), and it has to. A text rise
		// lives in exactly the spans this function scans, and left to
		// itself the scan was wrong in BOTH directions: a rise that is a
		// whole number of points ("-12 Ts") parses as an integer and is
		// silently recorded as a TJ adjustment, and a fractional one
		// ("-0.684 Ts") fails ParseInt and is silently dropped. ~18 call
		// sites read Adjustments, so a phantom term is a wrong answer
		// handed to all of them. Ts takes ONE operand and it precedes
		// the operator, so the token before a "Ts" is that operand and
		// is never an adjustment — read explicitly here rather than left
		// to the operand happening to be fractional.
		scanAdjustments := func(span string) {
			fields := strings.Fields(strings.NewReplacer("[", " ", "]", " ", "TJ", " ", "Tj", " ").Replace(span))
			for i, num := range fields {
				if num == "Ts" || (i+1 < len(fields) && fields[i+1] == "Ts") {
					continue
				}
				v, err := strconv.ParseInt(num, 10, 64)
				if err != nil {
					continue
				}
				run.Adjustments = append(run.Adjustments, v)
				if v != 0 {
					run.NonZeroAdjust = true
				}
			}
		}

		// Hex strings hold the CIDs; bare integers between them are the
		// adjustments.
		for {
			lt := strings.Index(body, "<")
			if lt == -1 {
				break
			}
			gt := strings.Index(body[lt:], ">")
			if gt == -1 {
				t.Fatalf("unterminated hex string in %q", body)
			}
			scanAdjustments(body[:lt])
			hexStr := body[lt+1 : lt+gt]
			for k := 0; k+4 <= len(hexStr); k += 4 {
				v, err := strconv.ParseUint(hexStr[k:k+4], 16, 16)
				if err != nil {
					t.Fatalf("unparseable CID %q", hexStr[k:k+4])
				}
				run.CIDs = append(run.CIDs, uint16(v))
			}
			body = body[lt+gt+1:]
		}
		// The tail after the last `>`: `] TJ` plus, if it is non-zero,
		// the trailing adjustment.
		scanAdjustments(body)
		runs = append(runs, run)
	}
	if len(runs) == 0 {
		t.Fatal("content stream carries no text runs")
	}
	return runs
}

// TestParseContentStreamRunsNeverMistakesATextRiseForAnAdjustment is
// Story 8.0's red-proof for the same parser, in the other direction.
//
// A text rise lives in exactly the spans scanAdjustments scans, and
// before it was taught the operator the helper was wrong BOTH ways: a
// whole-point rise ("-12 Ts") parsed as an integer and was recorded as a
// TJ adjustment that no emitter ever wrote, and a fractional one
// ("-0.684 Ts") failed ParseInt and vanished — as did the "0" of every
// restoring "0 Ts". ~18 call sites read Adjustments, so a phantom term
// is a wrong answer handed to all of them.
//
// The synthetic stream below carries one of each kind, deliberately: run
// it against the untaught helper and Adjustments comes back
// [-12 -30 0 0 7 0] instead of [-30 7].
func TestParseContentStreamRunsNeverMistakesATextRiseForAnAdjustment(t *testing.T) {
	const stream = "BT\n/F1 12 Tf\n1 0 0 1 0 0 Tm\n" +
		"-12 Ts\n[-30<0001>] TJ\n0 Ts\n" + // a rise that IS a valid integer
		"-0.684 Ts\n<0002> Tj\n0 Ts\n" + // a rise that is not
		"[7<0003>] TJ\nET\n"

	runs := parseContentStreamRuns(t, []byte(stream))
	if len(runs) != 1 {
		t.Fatalf("one BT..ET block is one run however many show-text operators it holds; got %d", len(runs))
	}
	got := runs[0]
	wantCIDs := []uint16{1, 2, 3}
	if len(got.CIDs) != len(wantCIDs) {
		t.Fatalf("CIDs = %v, want %v — every segment's glyphs belong to the run", got.CIDs, wantCIDs)
	}
	for i, c := range wantCIDs {
		if got.CIDs[i] != c {
			t.Fatalf("CIDs = %v, want %v", got.CIDs, wantCIDs)
		}
	}
	wantAdj := []int64{-30, 7}
	if len(got.Adjustments) != len(wantAdj) {
		t.Fatalf("Adjustments = %v, want %v — a Ts operand is not a TJ adjustment in either direction", got.Adjustments, wantAdj)
	}
	for i, a := range wantAdj {
		if got.Adjustments[i] != a {
			t.Fatalf("Adjustments = %v, want %v", got.Adjustments, wantAdj)
		}
	}
	if !got.NonZeroAdjust {
		t.Fatal("NonZeroAdjust is false for a run carrying -30 and 7")
	}
}

// TestReadEmittedRunsScansEveryTJAdjustment guards the PARSER two guards
// depend on (Story 2.3 finisher, Finding 5).
//
// readEmittedRuns used to walk a TJ array by repeatedly finding the next
// `<`, so the number emitted after the FINAL `>` and before `] TJ` was
// never examined. That number is real output — appendShapedRun writes
// adjustments[len(run.Glyphs)] there, and it is the term that makes a
// run's total advance equal the SHAPED advance rather than the sum of
// its /W widths. Measured on this fixture: 5 of 23 emitted runs carry
// one.
//
// It was latent rather than live, and fail-closed in direction, so
// nothing was wrong on the page. But NonZeroAdjust under-reported, and
// it is consumed by the semantic acceptance step and by the matrix leg
// guard requireShapedTextIsShaped — so both were blind to a class of
// adjustment the emitter can produce, and that is the kind of parser gap
// that becomes a vacuity the moment someone answers a spurious failure
// by loosening the guard instead of the parser.
//
// The cross-check is INDEPENDENT of the parser: it re-derives every bare
// integer inside every TJ array straight off the content-stream text and
// requires the parser to have seen the same multiset. Reverting the
// parser's trailing scan makes the two disagree by exactly the trailing
// terms.
func TestReadEmittedRunsScansEveryTJAdjustment(t *testing.T) {
	b := renderShapedTextFixture(t)
	runs := readEmittedRuns(t, b)

	byParser := map[int64]int{}
	parsed := 0
	for _, r := range runs {
		for _, a := range r.Adjustments {
			byParser[a]++
			parsed++
		}
	}

	// Independent recount, straight off the bytes.
	byText := map[int64]int{}
	independent := 0
	trailingRuns := 0
	blocks := strings.Split(string(b), "BT\n")[1:]
	for _, blk := range blocks {
		end := strings.Index(blk, "ET\n")
		if end == -1 {
			continue
		}
		blk = blk[:end]
		open := strings.Index(blk, "[")
		close := strings.Index(blk, "] TJ")
		if open == -1 || close == -1 || close <= open {
			continue // a bare Tj run carries no adjustments at all
		}
		body := blk[open+1 : close]
		// Strip every hex string, leaving only the bare integers.
		var bare strings.Builder
		for {
			lt := strings.Index(body, "<")
			if lt == -1 {
				bare.WriteString(body)
				break
			}
			gt := strings.Index(body[lt:], ">")
			if gt == -1 {
				t.Fatalf("unterminated hex string in TJ array: %q", body)
			}
			bare.WriteString(body[:lt])
			bare.WriteString(" ")
			body = body[lt+gt+1:]
		}
		if last := strings.LastIndex(blk[:close], ">"); last != -1 && strings.TrimSpace(blk[last+1:close]) != "" {
			trailingRuns++
		}
		for _, f := range strings.Fields(bare.String()) {
			v, err := strconv.ParseInt(f, 10, 64)
			if err != nil {
				continue
			}
			byText[v]++
			independent++
		}
	}

	// VACUITY: both operands must exist and be non-empty before they are
	// compared, and the fixture must actually contain the trailing case
	// this test exists for — otherwise the comparison proves nothing.
	if parsed == 0 {
		t.Fatal("the parser reported no TJ adjustments at all — nothing to cross-check")
	}
	if independent == 0 {
		t.Fatal("the independent recount found no TJ adjustments at all — the recount is broken, not the parser")
	}
	if trailingRuns == 0 {
		t.Fatal(
			"no emitted run carries a TRAILING adjustment (one after the final `>`), so this test cannot " +
				"distinguish a parser that scans it from one that does not. The fixture has stopped " +
				"exercising the case Finding 5 was about",
		)
	}

	if !maps.Equal(byParser, byText) {
		t.Errorf(
			"readEmittedRuns saw %d TJ adjustments but the content stream carries %d.\n"+
				"  parser: %v\n  stream: %v\n"+
				"%d run(s) carry a trailing adjustment after the final `>`; a parser that stops at the last "+
				"hex string can never reach those, and NonZeroAdjust then under-reports for every guard "+
				"that consumes it",
			parsed, independent, byParser, byText, trailingRuns,
		)
		return
	}
	t.Logf(
		"TJ adjustment cross-check — %d of %d emitted runs carry a trailing adjustment; parser and independent recount agree on all %d values",
		trailingRuns, len(runs), parsed,
	)
}

// expectedSegments recomputes the document's face-segmentation the same
// way Story 2.2's coverage resolution does, so this file can talk about
// "the run whose source text is X" without reimplementing anything the
// story owns. Shaping is NOT reimplemented here: the segment's text is
// handed to the same production seam the renderer uses.
func expectedSegments(t *testing.T) []textRunSource {
	t.Helper()
	tpl, err := ParseTemplate([]byte(shapedTextTemplateJSON))
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	cache := newFontCache()
	runs, err := collectTextRuns(tpl, emptyBindValue(t), emptyBindValue(t), testShippedFontSet(), cache)
	if err != nil {
		t.Fatalf("collectTextRuns: %v", err)
	}
	if len(runs) == 0 {
		t.Fatal("the fixture template produced no text runs")
	}
	return runs
}

func renderShapedTextFixture(t *testing.T) []byte {
	t.Helper()
	tpl, err := ParseTemplate([]byte(shapedTextTemplateJSON))
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	res, err := Render(tpl, Data("{}"), nil, testShippedFontSet())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return res.Bytes
}

// TestShapedTextFixtureIsShapeObservable is AC11, and it is the most
// important assertion in this story.
//
// For each shipped face, the fixture's own text must contain at least
// one segment whose shaped answer DIFFERS from the naive rune->cmap
// answer — except CJK, whose expected observable count is exactly zero
// and is asserted as such, with the reason stated inline.
//
// Red-proof, run and available today against a committed file: point
// this at fixtures/multi-script-fallback/input.folio ("Ada ก 汉") and it
// reports 0 shape-observable for Latin and Thai and fails.
func TestShapedTextFixtureIsShapeObservable(t *testing.T) {
	segments := expectedSegments(t)

	evaluated := map[string]int{}
	observable := map[string]int{}
	total, totalObservable := 0, 0

	for _, seg := range segments {
		script := scriptOfFace(t, seg.face)
		evaluated[script]++
		total++

		shaped := shapeRow(t, seg.face, seg.text)
		naive := naiveAnswer(t, seg.face, seg.text)
		if rowIsObservable(shaped, naive) {
			observable[script]++
			totalObservable++
		}
	}

	t.Logf(
		"fixture observability witness — %d of %d fixture text segments evaluated, %d shape-observable "+
			"(Thai %d/%d, Latin %d/%d, CJK %d/%d)",
		total, total, totalObservable,
		observable["Thai"], evaluated["Thai"],
		observable["Latin"], evaluated["Latin"],
		observable["CJK"], evaluated["CJK"],
	)

	for _, script := range []string{"Thai", "Latin"} {
		if evaluated[script] == 0 {
			t.Fatalf("the fixture contains no %s segments at all", script)
		}
		if observable[script] == 0 {
			t.Fatalf(
				"the fixture contains %d %s segments and ZERO of them are shape-observable. This fixture "+
					"cannot fail when this story regresses, while looking like coverage in every report "+
					"(D-000.9). Change the fixture's text, never this assertion.",
				evaluated[script], script,
			)
		}
	}

	if evaluated["CJK"] == 0 {
		t.Fatal("the fixture contains no CJK segments, so its negative control asserts nothing")
	}
	if observable["CJK"] != 0 {
		t.Fatalf(
			"CJK reports %d shape-observable fixture segments, want exactly 0. CJK legitimately does not "+
				"change: Han ideographs in Noto Sans SC carry no substitution or positioning rules and every "+
				"advance is exactly 1000. This is a negative control, not a gap — do not \"fix\" it.",
			observable["CJK"],
		)
	}

	// AC11's floor on WHAT the fixture must contain, asserted rather
	// than assumed, so the text cannot be edited into blindness.
	var haveTallConsonantMark, haveSaraAm, haveLatinLigatureOrKern bool
	for _, seg := range segments {
		shaped := shapeRow(t, seg.face, seg.text)
		naive := naiveAnswer(t, seg.face, seg.text)
		switch scriptOfFace(t, seg.face) {
		case "Thai":
			for i := range shaped {
				if i < len(naive) && shaped[i].GlyphID != naive[i].GlyphID && len(shaped) == len(naive) {
					haveTallConsonantMark = true // a GSUB substitution with no length change: the lowered mark form
				}
			}
			if len(shaped) > len(naive) {
				haveSaraAm = true // SARA AM decomposes: 3 runes become 4 glyphs
			}
		case "Latin":
			if len(shaped) < len(naive) {
				haveLatinLigatureOrKern = true // a ligature
			}
			for i := range shaped {
				if i < len(naive) && shaped[i].XAdvance != naive[i].XAdvance {
					haveLatinLigatureOrKern = true // a kern pair
				}
			}
		}
	}
	if !haveTallConsonantMark {
		t.Error("the fixture contains no Thai tall-consonant-plus-mark case (a GSUB substitution to a lowered mark form)")
	}
	if !haveSaraAm {
		t.Error("the fixture contains no Thai SARA AM case (a cluster whose glyph count exceeds its rune count)")
	}
	if !haveLatinLigatureOrKern {
		t.Error("the fixture contains no Latin ligature or kern pair")
	}
}

// TestShapedTextFixtureCIDsOriginateInShapedRuns is AC4, asserted
// POSITIVELY off the produced content stream — never by a denylist, a
// grep, or an assertion that some function is not called.
//
// For every run the page emits, the CIDs are read back off the PDF and
// matched against the shaped run for that segment on two independent
// counts: the CID COUNT must be the shaper's glyph count (which differs
// from the rune count exactly where shaping does something), and each
// CID's /ToUnicode text must equal that glyph's own cluster text. A CID
// that originated in a rune lookup fails both.
func TestShapedTextFixtureCIDsOriginateInShapedRuns(t *testing.T) {
	b := renderShapedTextFixture(t)
	emitted := readEmittedRuns(t, b)
	segments := expectedSegments(t)
	cmaps := toUnicodeForResources(t, b)

	if len(emitted) != len(segments) {
		t.Fatalf("the page emits %d runs but the document has %d face-segments", len(emitted), len(segments))
	}

	countDiffered := 0
	for i, seg := range segments {
		run := emitted[i]
		glyphs := shapeRow(t, seg.face, seg.text)
		if len(run.CIDs) != len(glyphs) {
			t.Fatalf(
				"run %d (%q, face %q): the page emits %d CIDs but the shaper produces %d glyphs. "+
					"A CID count matching the RUNE count (%d) instead means a rune->CID route reached the "+
					"content stream (AC4).",
				i, seg.text, seg.face, len(run.CIDs), len(glyphs), len([]rune(seg.text)),
			)
		}
		if len(run.CIDs) != len([]rune(seg.text)) {
			countDiffered++
		}

		font, err := fontset.New(seg.face, testShippedFontSet()[seg.face])
		if err != nil {
			t.Fatalf("fontset.New(%q): %v", seg.face, err)
		}
		shaped, err := font.Shaper().Shape(seg.text)
		if err != nil {
			t.Fatalf("Shape(%q): %v", seg.text, err)
		}
		texts, err := text.ClusterTexts(seg.text, shaped)
		if err != nil {
			t.Fatalf("ClusterTexts(%q): %v", seg.text, err)
		}

		cmap, ok := cmaps[run.Resource]
		if !ok {
			t.Fatalf("run %d selects resource /%s, which has no /ToUnicode CMap", i, run.Resource)
		}
		for k, cid := range run.CIDs {
			got, present := cmap[cid]
			if !present {
				t.Fatalf("run %d (%q): CID %d appears in the content stream with NO /ToUnicode entry", i, seg.text, cid)
			}
			if got != texts[k] {
				t.Errorf(
					"run %d (%q): CID %d at position %d extracts as %q, but that glyph's shaped cluster text is %q",
					i, seg.text, cid, k, got, texts[k],
				)
			}
		}
	}

	// Vacuity guard: at least one run's glyph count must actually differ
	// from its rune count, or the count assertion above never
	// discriminated anything.
	if countDiffered == 0 {
		t.Fatal(
			"no run's glyph count differs from its rune count, so the CID-count assertion above could not " +
				"have distinguished a shaped run from a rune->CID run (D-000.9)",
		)
	}
	t.Logf("AC4 witness — %d of %d emitted runs have a glyph count differing from their rune count", countDiffered, len(segments))
}

// TestShapedTextFixtureToUnicodeRoundTrips is AC7: concatenating each
// run's CIDs through the CMap must reproduce the ORIGINAL source text
// exactly — for the ffi ligature, for the merged SARA AM cluster, and
// for the standalone SARA AA that shares its last glyph.
//
// "Every CID has an entry" is not the assertion, and would not be worth
// making: it is satisfied by mapping all four glyphs of "น้ำ" to "น้ำ",
// which extracts the word four times over. The round-trip is what
// distinguishes correct from plausible.
func TestShapedTextFixtureToUnicodeRoundTrips(t *testing.T) {
	b := renderShapedTextFixture(t)
	emitted := readEmittedRuns(t, b)
	segments := expectedSegments(t)
	cmaps := toUnicodeForResources(t, b)

	var whole strings.Builder
	var expectedWhole strings.Builder
	for i, seg := range segments {
		run := emitted[i]
		cmap := cmaps[run.Resource]
		var got strings.Builder
		for _, cid := range run.CIDs {
			entry, ok := cmap[cid]
			if !ok {
				t.Fatalf("run %d (%q): CID %d has no /ToUnicode entry", i, seg.text, cid)
			}
			got.WriteString(entry)
		}
		if got.String() != seg.text {
			t.Errorf("run %d: /ToUnicode round-trips as %q, want the source text %q", i, got.String(), seg.text)
		}
		whole.WriteString(got.String())
		expectedWhole.WriteString(seg.text)
	}
	if whole.String() != expectedWhole.String() {
		t.Fatalf("the document's full text does not round-trip:\n got %q\nwant %q", whole.String(), expectedWhole.String())
	}

	// Named cases, so a failure says which mechanism broke rather than
	// only that something did.
	for _, want := range []string{"office", "น้ำ", "ป้ำ", "า"} {
		if !strings.Contains(expectedWhole.String(), want) {
			t.Fatalf("the fixture no longer contains %q, so AC7's named round-trip case is not exercised", want)
		}
	}
}

// TestShapedTextFixtureThaiDrawsTheLoweredMarkForm is the PDF-level
// statement of the defect this story exists to fix: the run showing "ปั"
// must draw the LOWERED mark form (source glyph 46) and not the
// unlowered one (source glyph 45), read off the produced document.
//
// It is asserted here as well as in the expectation table because the
// table proves the SHAPER is right, and this proves the right answer
// reached the page. Those are two different artifacts and D-000.21
// sharpened says so explicitly: assert on the artifact that carries the
// property.
func TestShapedTextFixtureThaiDrawsTheLoweredMarkForm(t *testing.T) {
	const loweredMarkGlyph = 46   // MAI HAN AKAT, lowered form: what GSUB selects
	const unloweredMarkGlyph = 45 // what a rune->cmap lookup returns, and what collides with ป's ascender

	b := renderShapedTextFixture(t)
	emitted := readEmittedRuns(t, b)
	segments := expectedSegments(t)

	// Build the Thai face's subset exactly as the renderer does — the
	// union of shaped glyph ids across the document, in document order.
	font, err := fontset.New("Noto Sans Thai", testShippedFontSet()["Noto Sans Thai"])
	if err != nil {
		t.Fatalf("fontset.New: %v", err)
	}
	var union []uint16
	for _, seg := range segments {
		if seg.face != "Noto Sans Thai" {
			continue
		}
		glyphs, err := font.Shaper().Shape(seg.text)
		if err != nil {
			t.Fatalf("Shape(%q): %v", seg.text, err)
		}
		for _, g := range glyphs {
			union = append(union, g.GlyphID)
		}
	}
	sub, err := font.Subset(union)
	if err != nil {
		t.Fatalf("Subset: %v", err)
	}

	// These are SUBSET GLYPH IDS, and they are named as such. An emitted
	// CID is a different number in a different space, and the two are
	// only equal inside the base block (Story 2.3 finisher, Finding 11).
	loweredSubsetGlyph, ok := sub.GlyphForSource[loweredMarkGlyph]
	if !ok {
		t.Fatalf("source glyph %d (the LOWERED mark form) is not in the Thai subset at all — the fixture no longer exercises this story's defect", loweredMarkGlyph)
	}
	unloweredSubsetGlyph, unloweredPresent := sub.GlyphForSource[unloweredMarkGlyph]

	// Resolve the emitted CID back through the document's OWN
	// /CIDToGIDMap before comparing, rather than comparing a CID against
	// a subset glyph id directly.
	//
	// The direct comparison was valid only while glyph 46's base CID was
	// unclaimed — i.e. while nothing gave it a second (glyph, cluster
	// text) pair. It holds today, which is why it passed. It fails in
	// BOTH directions the moment it stops holding: the test would report
	// "the ปั run draws the UNLOWERED mark form" — a FALSE RED on this
	// story's headline defect, pointing a reader at a typography
	// regression that did not happen — or, the other way, it would
	// silently start checking CID identity rather than glyph identity
	// while its name still said otherwise. This is the one assertion in
	// the story whose message a future reader is most likely to trust
	// without re-deriving it.
	// Keyed by the run's OWN resource name, as read off the content
	// stream — the resource name in the PDF is the PDF-escaped face name,
	// so hard-coding "Noto Sans Thai" here would silently miss.
	cidToGlyph := cidToGlyphForResources(t, b)

	found := false
	for i, seg := range segments {
		if seg.text != "ปั" {
			continue
		}
		found = true
		run := emitted[i]
		resolve, ok := cidToGlyph[run.Resource]
		if !ok {
			t.Fatalf(
				"the %q run selects font resource %q, which has no entry in the document's font resource "+
					"dictionary, so no CID can be resolved to a glyph",
				seg.text, run.Resource,
			)
		}
		if len(run.CIDs) != 2 {
			t.Fatalf("the %q run emits %d CIDs, want 2", seg.text, len(run.CIDs))
		}
		markGlyph := resolve(run.CIDs[1])
		if markGlyph != loweredSubsetGlyph {
			t.Errorf(
				"the %q run's mark CID %d resolves through /CIDToGIDMap to subset glyph %d, want %d "+
					"(the LOWERED form, source glyph %d)",
				seg.text, run.CIDs[1], markGlyph, loweredSubsetGlyph, loweredMarkGlyph,
			)
		}
		if unloweredPresent && markGlyph == unloweredSubsetGlyph {
			t.Errorf(
				"the %q run draws the UNLOWERED mark form (source glyph %d). That is the naive rune->cmap "+
					"answer, and it collides with ป's ascender — the exact defect this story exists to fix.",
				seg.text, unloweredMarkGlyph,
			)
		}
	}
	if !found {
		t.Fatal("the fixture no longer contains a bare \"ปั\" run, so this assertion exercises nothing")
	}
}

// TestShapedTextGoldenFixture is AC10: the recorded golden, plus the
// semantic acceptance step D-000.22 requires at FIRST RECORDING.
//
// D-000.22, verbatim on why the semantic half is not optional: "a hash
// guards against change; it has nothing whatever to say about whether
// what we recorded was right, and it cannot acquire that ability later."
// Story 2.2 shipped Thin Chinese past three correct guards because none
// was asked whether the value MEANT what its name implied — and the face
// that broke was the one whose script nobody on this project reads. Thai
// is that face this time.
//
// THE HUMAN HALF OF THAT STEP IS PENDING, not done. Stated here in the
// negative on purpose: an earlier version of this comment claimed a
// human had read the rendered Thai before this hash was frozen, and that
// claim was false from birth — the story's completion notes it cited as
// evidence say the sign-off is outstanding. D-000.24, verbatim: "a guard
// credited with a red-proof it does not have is worse than one openly
// labelled unproven, because the label tells the next reader WHERE TO
// LOOK. A false credit tells them the opposite."
//
// What IS done is a developer-side visual check (rasterised shaped
// output compared line by line against a deliberately naive render);
// what is NOT done is a Thai reader confirming the marks read correctly.
// That obligation is tracked by a FAILING TEST, not by this comment:
// TestShapedTextThaiSemanticSignOffIsRecorded in
// shaped_signoff_matrix_test.go is gated behind the `matrix` build tag,
// so it is red at the Epic 2 boundary gate until
// fixtures/shaped-text/thai-signoff.json exists and names this fixture's
// digest (D-2.3.5).
//
// Everything ASSERTED below is the machine-checkable half of D-000.22
// and none of it is deferred — that boundary is what keeps the rule's
// teeth, and it is stated in full in the fixture's README.
func TestShapedTextGoldenFixture(t *testing.T) {
	root := repoRootFromTest(t)
	fixturePath := filepath.Join(root, "fixtures", "shaped-text", "expected.json")

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture expectedFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse fixture JSON: %v", err)
	}
	if fixture.Folio8GoVersion == "" {
		t.Fatal("fixture is missing folio8GoVersion")
	}
	if fixture.GoToolchain == "" {
		t.Fatal("fixture is missing goToolchain (this must fail before the hash comparison runs)")
	}
	if fixture.SHA256 == "" {
		t.Fatal("fixture is missing sha256")
	}
	if !isSHA256HexString(fixture.SHA256) {
		t.Fatalf("fixture sha256 %q is not a JSON string of exactly 64 lower-case hex characters (AC16)", fixture.SHA256)
	}

	inputPath := filepath.Join(root, "fixtures", "shaped-text", "input.folio")
	inputBytes, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read %s: %v", inputPath, err)
	}
	if string(inputBytes) != shapedTextTemplateJSON {
		t.Fatalf("%s has drifted from folio-go/shapedTextTemplateJSON (render_test.go) — the two are supposed to be byte-identical", inputPath)
	}

	b := renderShapedTextFixture(t)
	assertWellFormedPDF(t, "shaped-text golden fixture render", b, 1)

	// --- D-000.22's semantic acceptance step, on the produced PDF ---

	// (1) AC6's mechanism is LIVE, not merely written.
	emitted := readEmittedRuns(t, b)
	tjRuns, adjustedRuns := 0, 0
	for _, run := range emitted {
		if run.UsedTJ {
			tjRuns++
		}
		if run.NonZeroAdjust {
			adjustedRuns++
		}
	}
	if tjRuns == 0 {
		t.Fatal("the content stream contains no TJ array — AC6's mechanism is written but not reached")
	}
	if adjustedRuns == 0 {
		t.Fatal("the content stream contains no TJ array with a NON-ZERO adjustment — every offset and kern computed to zero, so this fixture certifies nothing about positioning")
	}

	// (2) The Latin face draws the ligature, and its components are not
	//     drawn separately in that position: asserted through the CMap,
	//     which is the artifact that carries the meaning "this CID is
	//     three characters".
	cmaps := toUnicodeForResources(t, b)
	latinCMap, ok := cmaps["NotoSans"]
	if !ok {
		t.Fatal("no /NotoSans font resource in the produced PDF")
	}
	var ligatureCID uint16
	haveLigature := false
	for cid, txt := range latinCMap {
		if txt == "ffi" {
			ligatureCID, haveLigature = cid, true
		}
	}
	if !haveLigature {
		t.Fatal("the Latin face's /ToUnicode carries no CID extracting as \"ffi\" — GSUB did not reach the page")
	}
	segments := expectedSegments(t)
	for i, seg := range segments {
		if !strings.HasPrefix(seg.text, "office") {
			continue
		}
		run := emitted[i]
		if !containsCID(run.CIDs, ligatureCID) {
			t.Errorf("the %q run does not emit the ffi ligature CID %d", seg.text, ligatureCID)
		}
		if len(run.CIDs) >= len([]rune(seg.text)) {
			t.Errorf("the %q run emits %d CIDs for %d runes — the ligature did not collapse its components", seg.text, len(run.CIDs), len([]rune(seg.text)))
		}
	}

	// (3) The Thai lowered mark form reaches the page: asserted in full
	//     by TestShapedTextFixtureThaiDrawsTheLoweredMarkForm, which is
	//     the same document and the same mechanism.
	// (4) /ToUnicode round-trips: TestShapedTextFixtureToUnicodeRoundTrips.

	// (5) /BaseFont is TAG+<PostScriptName> per face (D-2.2.6 amended).
	//     The TAG changes with this story, expectedly — the subset input
	//     is now the shaped glyph set — so the SHAPE is asserted, never a
	//     literal tag.
	//
	// SCOPED TO THIS FIXTURE'S OWN CHAIN, NOT EVERY SHIPPED FACE (Story
	// 16.8) — the same correction as TestMultiScriptFallbackGoldenFixture's,
	// for the same reason: `shapedTextTemplateJSON`'s "body" chain names
	// three faces, and Story 16.8 adds a fourth shipped face (Roboto)
	// that is engine-only and never embedded in any document, so it is
	// legitimately absent from this fixture's /BaseFont set and its
	// per-instance program count. Re-parsing the template here (rather
	// than threading `tpl` out of renderShapedTextFixture) keeps the
	// helper's signature untouched.
	shapedTpl, err := ParseTemplate([]byte(shapedTextTemplateJSON))
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	wantEmbeddedFaces := map[string]bool{}
	for _, entry := range shapedTpl.doc.Fonts["body"] {
		wantEmbeddedFaces[entry.Face] = true
	}
	wantPS := map[string]bool{}
	for _, spec := range shippedFaceSpecs {
		if wantEmbeddedFaces[spec.Key] {
			wantPS[spec.PostScriptName] = true
		}
	}
	assertBaseFontNames(t, "shaped-text golden", b, wantPS)
	assertSubsetTagsAppearOnlyInNames(t, "shaped-text golden", b)

	// (6) The embedded programs carry no fvar and no gvar, and are
	//     Regular — carried forward from Story 2.2's semantic set, so a
	//     face regressing to variable reddens here too.
	if !containsFontFile2(b) {
		t.Fatal("shaped-text golden fixture render contains no FontFile2")
	}
	programs := extractAllFontFile2Programs(t, b)
	wantEmbeddedCount := len(shapedTpl.doc.Fonts["body"])
	if wantEmbeddedCount == 0 {
		t.Fatal("shapedTextTemplateJSON's \"body\" chain parsed to zero entries — this assertion would be vacuous")
	}
	if len(programs) != wantEmbeddedCount {
		t.Fatalf("expected exactly %d embedded FontFile2 programs (one per face this fixture's own \"body\" chain names), got %d", wantEmbeddedCount, len(programs))
	}
	assertEmbeddedProgramsAreStaticRegular(t, "shaped-text golden", programs, 400)

	// (7) Page count and declared page size are what the template says.
	if got := bytes.Count(b, []byte("/Type /Page ")); got != 1 {
		t.Fatalf("expected exactly 1 page object, got %d", got)
	}
	if !bytes.Contains(b, []byte("/MediaBox [0 0 595.276 841.89]")) {
		t.Fatal("the produced page does not declare the A4 MediaBox the template asks for")
	}

	// --- expected.pdf must agree with the normative hash ---
	expectedPDFPath := filepath.Join(root, "fixtures", "shaped-text", "expected.pdf")
	expectedPDFBytes, err := os.ReadFile(expectedPDFPath)
	if err != nil {
		t.Fatalf("read expected.pdf: %v", err)
	}
	expectedPDFSum := sha256.Sum256(expectedPDFBytes)
	expectedPDFHex := hex.EncodeToString(expectedPDFSum[:])
	if expectedPDFHex != fixture.SHA256 {
		t.Fatalf(
			"fixtures/shaped-text/expected.pdf's own sha256 (%s) does not match expected.json's recorded sha256 (%s) — the fixture's two halves have drifted apart",
			expectedPDFHex, fixture.SHA256,
		)
	}

	if runtime.Version() != fixture.GoToolchain {
		t.Fatalf(
			"toolchain mismatch: running under %s, fixture was recorded under %s. A Go toolchain bump is a versioned breaking change under AD-22 (C6).",
			runtime.Version(), fixture.GoToolchain,
		)
	}

	sum := sha256.Sum256(b)
	gotHex := hex.EncodeToString(sum[:])
	if gotHex != fixture.SHA256 {
		t.Fatalf(
			"golden fixture mismatch: got sha256 %s, want %s (fixtures/shaped-text). Under AD-21/AD-22 this is a defect until proven to be an intended, versioned change — see fixtures/shaped-text/README.md. Do not regenerate the fixture to make this pass.",
			gotHex, fixture.SHA256,
		)
	}

	t.Logf("shaped-text semantic acceptance — %d of %d emitted runs use TJ, %d carry a non-zero adjustment", tjRuns, len(emitted), adjustedRuns)
}

func containsCID(cids []uint16, want uint16) bool {
	for _, c := range cids {
		if c == want {
			return true
		}
	}
	return false
}

// TestShapedTextFixtureObservabilityRedProof is AC11's red-proof, run
// against a COMMITTED file rather than a scratch one: the same
// observability measurement, pointed at fixtures/multi-script-fallback/
// ("Ada ก 汉"), must report ZERO shape-observable segments for Latin and
// for Thai.
//
// This is what makes AC11's green result mean something. Without it,
// "every face has an observable segment" is a claim about the
// measurement's sensitivity that the measurement itself cannot make.
func TestShapedTextFixtureObservabilityRedProof(t *testing.T) {
	tpl, err := ParseTemplate([]byte(multiScriptTestTemplateJSON))
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	cache := newFontCache()
	segments, err := collectTextRuns(tpl, emptyBindValue(t), emptyBindValue(t), testShippedFontSet(), cache)
	if err != nil {
		t.Fatalf("collectTextRuns: %v", err)
	}
	if len(segments) == 0 {
		t.Fatal("the multi-script fixture produced no segments — this red-proof would be vacuous")
	}

	observable := map[string]int{}
	for _, seg := range segments {
		shaped := shapeRow(t, seg.face, seg.text)
		naive := naiveAnswer(t, seg.face, seg.text)
		if rowIsObservable(shaped, naive) {
			observable[scriptOfFace(t, seg.face)]++
		}
	}
	for _, script := range []string{"Latin", "Thai", "CJK"} {
		if observable[script] != 0 {
			t.Fatalf(
				"AC11's red-proof no longer reproduces: fixtures/multi-script-fallback/ reports %d "+
					"shape-observable %s segments, want 0. That fixture's text was MEASURED blind to shaping; "+
					"if it is not any more, this story's premise has changed and needs re-measuring.",
				observable[script], script,
			)
		}
	}
	t.Logf("AC11 red-proof witness — fixtures/multi-script-fallback/: %d of %d segments evaluated, 0 shape-observable", len(segments), len(segments))
}

// emptyBindValue is the decoded form of an empty JSON object — what
// Render passes down for `{}` data and for absent params.
func emptyBindValue(t *testing.T) bind.Value {
	t.Helper()
	v, err := bind.DecodeData([]byte("{}"))
	if err != nil {
		t.Fatalf("decode empty data: %v", err)
	}
	return v
}

var _ = fmt.Sprint
