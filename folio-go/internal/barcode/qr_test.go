package barcode

import (
	"bytes"
	"errors"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// TestReedSolomonAnnexIExample checks the Reed-Solomon codewords against the
// worked example in ISO/IEC 18004 Annex I: "01234567" as a 1-M symbol, whose
// 16 data codewords produce these 10 error-correction codewords.
func TestReedSolomonAnnexIExample(t *testing.T) {
	data := []byte{0x10, 0x20, 0x0C, 0x56, 0x61, 0x80, 0xEC, 0x11, 0xEC, 0x11, 0xEC, 0x11, 0xEC, 0x11, 0xEC, 0x11}
	want := []byte{0xA5, 0x24, 0xD4, 0xC1, 0xED, 0x36, 0xC7, 0x87, 0x2C, 0x55}
	if got := ReedSolomonRemainder(data, ReedSolomonGenerator(10)); !bytes.Equal(got, want) {
		t.Fatalf("RS codewords = % X, want % X", got, want)
	}
	if qrECCPerBlock[ECLevelM][1] != 10 || qrDataCodewords(1, ECLevelM) != 16 {
		t.Fatalf("1-M must hold 16 data and 10 error-correction codewords")
	}
}

// TestQRWholeSymbolMatchesIndependentEncoder pins one complete symbol against
// a grid produced by python-qrcode (version 1, level L, mask 3), so the
// module layout is not checked only against EncodeQR itself.
func TestQRWholeSymbolMatchesIndependentEncoder(t *testing.T) {
	want := []string{
		"#######.#.#.#.#######", "#.....#...##..#.....#", "#.###.#.##.#..#.###.#", "#.###.#.##..#.#.###.#", "#.###.#.#...#.#.###.#", "#.....#..##...#.....#", "#######.#.#.#.#######", "...........#.........", "####..#.###..#..###.#", "#.#.##.#####.#..##...", ".######.##.#.....####", "...###.##.#..#...#...", "....###...#.#.##.##.#", "........#.#.##.##...#", "#######...#.....#.#..", "#.....#..#..#..#..###", "#.###.#....#.#...###.", "#.###.#.######.#.###.", "#.###.#.##..#..#.##..", "#.....#.#..#..#.##..#", "#######.###...#.###..",
	}
	sym, err := EncodeQR([]byte("INV-2026-000001"), ECLevelL)
	if err != nil {
		t.Fatal(err)
	}
	if sym.Version != 1 || sym.Mask != 3 || sym.Size != len(want) {
		t.Fatalf("version %d mask %d size %d, want version 1 mask 3 size %d", sym.Version, sym.Mask, sym.Size, len(want))
	}
	for y, row := range want {
		for x := 0; x < len(row); x++ {
			if sym.Dark(x, y) != (row[x] == '#') {
				t.Fatalf("module (%d,%d): dark %v, python-qrcode %q", x, y, sym.Dark(x, y), row[x])
			}
		}
	}
}

// TestQRFormatAndVersionBits pins the BCH values against ISO/IEC 18004
// Annex C and D.
func TestQRFormatAndVersionBits(t *testing.T) {
	for _, c := range []struct {
		level ECLevel
		mask  int
		want  int
	}{
		{ECLevelM, 0, 0x5412}, // all-zero data: the mask pattern itself
		{ECLevelL, 0, 0x77C4},
		{ECLevelL, 7, 0x6976},
		{ECLevelM, 5, 0x40CE},
		{ECLevelQ, 0, 0x355F},
		{ECLevelH, 0, 0x1689},
		{ECLevelH, 7, 0x083B},
	} {
		if got := QRFormatBits(c.level, c.mask); got != c.want {
			t.Errorf("format bits %s mask %d = %#04x, want %#04x", c.level, c.mask, got, c.want)
		}
	}
	for _, c := range []struct{ version, want int }{
		{7, 0x07C94}, {8, 0x085BC}, {21, 0x15683}, {40, 0x28C69},
	} {
		if got := QRVersionBits(c.version); got != c.want {
			t.Errorf("version bits %d = %#05x, want %#05x", c.version, got, c.want)
		}
	}
}

// TestQRCapacityEdges checks byte-mode capacity against ISO/IEC 18004
// Table 7 at the smallest, a middle and the largest version, and that one
// byte past a version's capacity moves to the next version.
func TestQRCapacityEdges(t *testing.T) {
	for _, c := range []struct {
		version    int
		l, m, q, h int
	}{
		{1, 17, 14, 11, 7},
		{2, 32, 26, 20, 14},
		{7, 154, 122, 86, 64},
		{10, 271, 213, 151, 119},
		{27, 1465, 1125, 805, 625},
		{40, 2953, 2331, 1663, 1273},
	} {
		for level, want := range []int{c.l, c.m, c.q, c.h} {
			if got := QRMaxBytes(c.version, ECLevel(level)); got != want {
				t.Errorf("version %d level %s holds %d bytes, want %d", c.version, ECLevel(level), got, want)
			}
		}
	}
	for level := ECLevelL; level <= ECLevelH; level++ {
		for v := QRMinVersion; v < QRMaxVersion; v++ {
			max := QRMaxBytes(v, level)
			if got, _ := QRVersionFor(max, level); got != v {
				t.Fatalf("%d bytes at %s chose version %d, want %d", max, level, got, v)
			}
			if got, _ := QRVersionFor(max+1, level); got != v+1 {
				t.Fatalf("%d bytes at %s chose version %d, want %d", max+1, level, got, v+1)
			}
		}
		over := QRMaxBytes(QRMaxVersion, level) + 1
		_, err := EncodeQR(make([]byte, over), level)
		var tl *QRTooLongError
		if !errors.As(err, &tl) || tl.Bytes != over {
			t.Fatalf("%d bytes at %s: err = %v, want a QRTooLongError", over, level, err)
		}
	}
}

// TestQRLevelsNeverShrink: a higher level never picks a smaller version.
func TestQRLevelsNeverShrink(t *testing.T) {
	data := []byte("00020101021230810016A00000067701011201150994000123456780214INV2026000001030900000000153037645406150.005802TH62100706INV01263049A3F")
	prev := 0
	for level := ECLevelL; level <= ECLevelH; level++ {
		sym, err := EncodeQR(data, level)
		if err != nil {
			t.Fatal(err)
		}
		if sym.Version < prev || sym.Level != level || sym.Size != QRSize(sym.Version) || len(sym.Modules) != sym.Size*sym.Size {
			t.Fatalf("level %s: %+v after version %d", level, sym, prev)
		}
		prev = sym.Version
	}
}

// TestQRMaskPenaltyIsDeterministic: the mask choice is the lowest penalty,
// ties to the lower number, and the same input always gives the same modules.
func TestQRMaskPenaltyIsDeterministic(t *testing.T) {
	for _, s := range []string{"folio8", "ชำระเงิน 0123456789", "https://example.com/pay?ref=0123456789"} {
		for level := ECLevelL; level <= ECLevelH; level++ {
			a, err := EncodeQR([]byte(s), level)
			if err != nil {
				t.Fatal(err)
			}
			b, _ := EncodeQR([]byte(s), level)
			if a.Mask != b.Mask || !equalModules(a.Modules, b.Modules) {
				t.Fatalf("%q at %s is not deterministic", s, level)
			}
			// Recompute every mask's penalty on a fresh grid.
			version, _ := QRVersionFor(len(s), level)
			scores := make([]int, 8)
			for mask := 0; mask < 8; mask++ {
				g := newQRGrid(version)
				g.drawFunctionPatterns(level)
				g.drawCodewords(qrAddECCAndInterleave(qrDataBytes([]byte(s), version, level), version, level))
				g.applyMask(mask)
				g.drawFormatBits(level, mask)
				scores[mask] = g.penalty()
				if mask == a.Mask && !equalModules(g.modules, a.Modules) {
					t.Fatalf("%q at %s: the chosen mask's grid differs from the symbol", s, level)
				}
			}
			for mask, score := range scores {
				if score < scores[a.Mask] || (score == scores[a.Mask] && mask < a.Mask) {
					t.Fatalf("%q at %s chose mask %d (penalty %d) over mask %d (penalty %d)", s, level, a.Mask, scores[a.Mask], mask, score)
				}
			}
		}
	}
}

// TestQRPenaltyRules checks each rule on hand-built grids.
func TestQRPenaltyRules(t *testing.T) {
	// A 21x21 all-light grid: every row and column is one run of 21 (N1:
	// 3 + 16 each, 42 lines), 400 2x2 blocks (N2), no finder-like pattern,
	// and 100% light (N4: k = 9).
	g := newQRGrid(1)
	want := 42*(3+16) + 400*3 + 9*10
	if got := g.penalty(); got != want {
		t.Fatalf("all-light penalty = %d, want %d", got, want)
	}
	// A checkerboard has no runs and no blocks, is balanced to within 5%
	// (221 dark of 441), and has no finder-like pattern.
	for y := 0; y < 21; y++ {
		for x := 0; x < 21; x++ {
			g.modules[y*21+x] = (x+y)%2 == 0
		}
	}
	if got := g.penalty(); got != 0 {
		t.Fatalf("checkerboard penalty = %d, want 0", got)
	}
}

func equalModules(a, b []bool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFitQR(t *testing.T) {
	// Version 1 is 21 modules, 29 with the quiet zone.
	fit, ok := FitQR(21, 200000, 80000)
	if !ok || fit.ModuleWidth != 80000/29 {
		t.Fatalf("fit = %+v %v", fit, ok)
	}
	symbol := fit.ModuleWidth * 21
	if fit.OffsetX != geom.ScaleRound(200000-symbol, 1, 2) || fit.OffsetY != geom.ScaleRound(80000-symbol, 1, 2) {
		t.Fatalf("not centred: %+v", fit)
	}
	if fit.OffsetY < 4*fit.ModuleWidth {
		t.Fatalf("quiet zone below 4 modules: %+v", fit)
	}
	if _, ok := FitQR(21, 28, 1000); ok {
		t.Fatal("a 28 mp side cannot hold 29 modules")
	}
	if fit, ok := FitQR(21, 29, 29); !ok || fit.ModuleWidth != 1 {
		t.Fatalf("29 mp holds 1 mp modules: %+v %v", fit, ok)
	}
}

func TestQRRectsMergeRuns(t *testing.T) {
	sym, err := EncodeQR([]byte("folio8"), ECLevelM)
	if err != nil {
		t.Fatal(err)
	}
	fit := QRFit{ModuleWidth: 10, OffsetX: 5, OffsetY: 7}
	rects := QRRects(sym, fit)
	// Every dark module is covered exactly once, and no two rects in a row
	// touch.
	covered := make([]bool, len(sym.Modules))
	for i, r := range rects {
		if r.H != 10 || r.W%10 != 0 || (r.X-5)%10 != 0 || (r.Y-7)%10 != 0 {
			t.Fatalf("rect %d off the module grid: %+v", i, r)
		}
		y := int((r.Y - 7) / 10)
		for x := int((r.X - 5) / 10); x < int((r.X-5+r.W)/10); x++ {
			if covered[y*sym.Size+x] || !sym.Dark(x, y) {
				t.Fatalf("rect %d covers module (%d,%d) wrongly", i, x, y)
			}
			covered[y*sym.Size+x] = true
		}
		if i > 0 && rects[i-1].Y == r.Y && rects[i-1].X+rects[i-1].W >= r.X {
			t.Fatalf("rects %d and %d are not merged or out of order", i-1, i)
		}
	}
	for i, dark := range sym.Modules {
		if dark != covered[i] {
			t.Fatalf("module %d dark=%v covered=%v", i, dark, covered[i])
		}
	}
}
