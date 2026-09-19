package folio8

// D-2.7.2's residual hazard, PROVEN rather than assumed forward
// (D-000.24): "the face set is not frozen, and a proportional-figure
// face would silently break the reservation." Requiring tabular figures
// as an ACCEPTANCE CRITERION is dead on arrival — every shipped face
// already has them, so the AC would be satisfied at birth (D-000.28)
// and no shipped subject can falsify it (D-000.50). What IS available
// is a guard keyed on the reservation's actual purpose — "the
// reservation is valid" (D-000.15), never the phrase "tabular figures"
// — asserted from the declarative per-face record and RED-PROVED by
// perturbing one digit's advance in a scratch copy of a face, the same
// move Story 2.3a used for the stripped-OS/2 case.

import (
	"encoding/binary"
	"maps"
	"slices"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/fontset"
)

// digitAdvancesForFace returns AdvanceForRune('0'..'9') for the named
// face in fs, in digit order — the DECLARATIVE per-face record D-2.7.2
// asks for.
func digitAdvancesForFace(t *testing.T, faceName string, fs FontSet) [10]int64 {
	t.Helper()
	data, ok := fs[faceName]
	if !ok {
		t.Fatalf("face %q not present in FontSet", faceName)
	}
	font, err := fontset.New(faceName, data)
	if err != nil {
		t.Fatalf("fontset.New(%q): %v", faceName, err)
	}
	var advances [10]int64
	for d := 0; d < 10; d++ {
		adv, ok := font.AdvanceForRune(rune('0' + d))
		if !ok {
			t.Fatalf("face %q: AdvanceForRune(%q) reports no coverage — every shipped face covers 0-9 (finding 3, story creation)", faceName, rune('0'+d))
		}
		advances[d] = adv
	}
	return advances
}

func allEqual(a [10]int64) bool {
	for _, v := range a {
		if v != a[0] {
			return false
		}
	}
	return true
}

// TestShippedFacesHaveUniformDigitAdvances is D-2.7.2's declarative
// record: PROVEN true today for every face this project ships (measured
// at story creation: 572/572/555 across NotoSans/NotoSansThai/NotoSansSC,
// all ten digits identical within each face) — asserted here so it is
// checked on every run rather than trusted from that one measurement.
func TestShippedFacesHaveUniformDigitAdvances(t *testing.T) {
	fs := testShippedFontSet()
	// This story's review, Finding 13 (Nit): iterating a hardcoded
	// three-face literal list catches a face being REMOVED (the
	// digitAdvancesForFace fatal below, if a named face went missing)
	// but never a face being ADDED — a fourth shipped face would never
	// be checked for digit uniformity. Ranging fs's own keys (sorted,
	// D-1.3.5/ScanMapRange) closes that: this guard now covers whatever
	// testShippedFontSet() actually returns, not a snapshot of it.
	names := slices.Sorted(maps.Keys(fs))
	if len(names) == 0 {
		t.Fatal("presence precondition (D-000.50): testShippedFontSet() returned no faces — this guard is vacuous over an empty face set")
	}
	for _, name := range names {
		advances := digitAdvancesForFace(t, name, fs)
		if !allEqual(advances) {
			t.Errorf("face %q: digit advances are NOT uniform: %v — D-2.7.2's {{page}} reservation requires "+
				"tabular figures; this face can no longer host a Page X of Y construct without re-deriving "+
				"the reservation design", name, advances)
		} else {
			t.Logf("face %q: all ten digit advances = %d", name, advances[0])
		}
	}
}

// TestUniformDigitAdvanceGuardRedProof is D-000.52's executed
// demonstration: perturb ONE digit's hmtx advance in a SCRATCH COPY of
// the shipped Noto Sans bytes (never the committed file) and confirm
// digitAdvancesForFace's own check — the SAME check
// TestShippedFacesHaveUniformDigitAdvances runs — actually reddens
// against it. Without this, "the guard would catch a non-tabular face"
// is a claim about the guard's sensitivity that the guard itself never
// demonstrates (D-000.24: an unprovable label over a provable guard is
// a false credit).
func TestUniformDigitAdvanceGuardRedProof(t *testing.T) {
	cache := newFontCache()
	segs, _, serr := shapeSegments("", []string{"Noto Sans"}, nil, "0123456789", testShippedFontSet(), cache, breaksAreDrawn)
	if serr != nil {
		t.Fatalf("shapeSegments(\"0123456789\"): %v", serr)
	}
	if len(segs) != 1 || len(segs[0].glyphs) != 10 {
		t.Fatalf("presence precondition: shaping \"0123456789\" against Noto Sans produced %d segment(s) with an unexpected glyph count — cannot identify digit 3's glyph id", len(segs))
	}
	digit3GID := segs[0].glyphs[3].GlyphID

	patched := patchHmtxAdvance(t, testShippedNotoSans, digit3GID, 900) // 900 != 572, an arbitrary distinct advance

	font, err := fontset.New("Noto Sans (patched)", patched)
	if err != nil {
		t.Fatalf("fontset.New on the patched scratch copy: %v", err)
	}
	adv3, ok := font.AdvanceForRune('3')
	if !ok {
		t.Fatal("presence precondition: patched face reports no advance for '3'")
	}
	adv0, ok := font.AdvanceForRune('0')
	if !ok {
		t.Fatal("presence precondition: patched face reports no advance for '0'")
	}
	if adv3 == adv0 {
		t.Fatalf("presence precondition: the patch did not take effect — digit '3' still advances %d, same as digit '0' (%d)", adv3, adv0)
	}

	fs := FontSet{"Noto Sans": patched}
	advances := digitAdvancesForFace(t, "Noto Sans", fs)
	if allEqual(advances) {
		t.Fatal("RED-PROOF FAILED: a scratch face with digit '3' perturbed to a distinct advance still reads " +
			"as uniform — TestShippedFacesHaveUniformDigitAdvances would pass on a face whose reservation " +
			"is actually broken")
	}
	t.Logf("red-proof: patched digit advances = %v, correctly detected as non-uniform", advances)
}

// patchHmtxAdvance returns a SCRATCH COPY of src with glyph id gid's
// hmtx advanceWidth overwritten to newAdvance (font design units,
// unscaled). It requires gid < hhea.numberOfHMetrics (the LONG hmtx
// record shape, four bytes per glyph: advanceWidth uint16, lsb int16) —
// true for a low glyph id such as an ASCII digit in every shipped face,
// and fataled otherwise rather than silently patching the wrong bytes.
func patchHmtxAdvance(t *testing.T, src []byte, gid uint16, newAdvance uint16) []byte {
	t.Helper()
	out := make([]byte, len(src))
	copy(out, src)

	numTables := int(binary.BigEndian.Uint16(out[4:6]))
	var hheaOff, hheaLen, hmtxOff, hmtxLen uint32
	var haveHhea, haveHmtx bool
	for i := 0; i < numTables; i++ {
		rec := 12 + i*16
		tag := string(out[rec : rec+4])
		off := binary.BigEndian.Uint32(out[rec+8 : rec+12])
		length := binary.BigEndian.Uint32(out[rec+12 : rec+16])
		switch tag {
		case "hhea":
			hheaOff, hheaLen, haveHhea = off, length, true
		case "hmtx":
			hmtxOff, hmtxLen, haveHmtx = off, length, true
		}
	}
	if !haveHhea || !haveHmtx {
		t.Fatalf("presence precondition: source face carries no hhea/hmtx table (hhea=%v, hmtx=%v)", haveHhea, haveHmtx)
	}
	if hheaLen < 36 {
		t.Fatalf("presence precondition: hhea table is %d bytes, want >= 36", hheaLen)
	}
	numberOfHMetrics := binary.BigEndian.Uint16(out[hheaOff+34 : hheaOff+36])
	if uint16(gid) >= numberOfHMetrics {
		t.Fatalf("glyph id %d is not in the LONG hmtx block (numberOfHMetrics=%d) — this helper only patches a glyph with its own long entry", gid, numberOfHMetrics)
	}
	entryOff := hmtxOff + uint32(gid)*4
	if entryOff+2 > hmtxOff+hmtxLen {
		t.Fatalf("computed hmtx entry offset %d exceeds the table's own bounds (offset %d, length %d)", entryOff, hmtxOff, hmtxLen)
	}
	binary.BigEndian.PutUint16(out[entryOff:entryOff+2], newAdvance)
	return out
}
