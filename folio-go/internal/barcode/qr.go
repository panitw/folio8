// This file is the QR Code encoder (ISO/IEC 18004, Model 2) behind the
// qrcode element (spec-barcode-qr-elements CAP-2, CAP-3).
//
// It encodes in BYTE MODE only, over the content's UTF-8 bytes with no ECI,
// at the smallest version (1-40) that holds the content at the requested
// error-correction level. The eight masks are scored with the standard's
// four penalty rules and the lowest score wins; a tie keeps the lower mask
// number. Everything is integer arithmetic over fixed tables: no floats, no
// maps, and the same input always gives the same modules.
package barcode

import (
	"fmt"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// QRQuietZoneModules is the quiet zone on each side of a QR symbol, inside
// the element box.
const QRQuietZoneModules = 4

// QRMinModuleWidth is 0.5 mm rounded up to a whole millipoint. A module
// narrower than this still draws, with a Warning.
const QRMinModuleWidth geom.Length = 1418

// QRMinVersion and QRMaxVersion bound the symbol versions the encoder uses.
const (
	QRMinVersion = 1
	QRMaxVersion = 40
)

// ECLevel is a QR error-correction level. The zero value is L.
type ECLevel int

// The four levels, ordered by recovery capacity (about 7, 15, 25, 30%).
const (
	ECLevelL ECLevel = iota
	ECLevelM
	ECLevelQ
	ECLevelH
)

// String is the level's one-letter name, as the format spells it.
func (l ECLevel) String() string {
	switch l {
	case ECLevelL:
		return "L"
	case ECLevelM:
		return "M"
	case ECLevelQ:
		return "Q"
	case ECLevelH:
		return "H"
	}
	return fmt.Sprintf("ECLevel(%d)", int(l))
}

// ParseECLevel maps "L", "M", "Q" or "H" to its level.
func ParseECLevel(s string) (ECLevel, bool) {
	switch s {
	case "L":
		return ECLevelL, true
	case "M":
		return ECLevelM, true
	case "Q":
		return ECLevelQ, true
	case "H":
		return ECLevelH, true
	}
	return 0, false
}

// formatBits is the level's two-bit indicator in the format information
// (ISO/IEC 18004 Table 25): L 01, M 00, Q 11, H 10.
func (l ECLevel) formatBits() int {
	return [4]int{1, 0, 3, 2}[l]
}

// qrECCPerBlock is the error-correction codewords per block, indexed by
// level then version (index 0 unused). ISO/IEC 18004 Table 9.
var qrECCPerBlock = [4][41]int{
	{0, 7, 10, 15, 20, 26, 18, 20, 24, 30, 18, 20, 24, 26, 30, 22, 24, 28, 30, 28, 28, 28, 28, 30, 30, 26, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},
	{0, 10, 16, 26, 18, 24, 16, 18, 22, 22, 26, 30, 22, 22, 24, 24, 28, 28, 26, 26, 26, 26, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28},
	{0, 13, 22, 18, 26, 18, 24, 18, 22, 20, 24, 28, 26, 24, 20, 30, 24, 28, 28, 26, 30, 28, 30, 30, 30, 30, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},
	{0, 17, 28, 22, 16, 22, 28, 26, 26, 24, 28, 24, 28, 22, 24, 24, 30, 28, 28, 26, 28, 30, 24, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},
}

// qrBlocks is the number of error-correction blocks, indexed like
// qrECCPerBlock. ISO/IEC 18004 Table 9.
var qrBlocks = [4][41]int{
	{0, 1, 1, 1, 1, 1, 2, 2, 2, 2, 4, 4, 4, 4, 4, 6, 6, 6, 6, 7, 8, 8, 9, 9, 10, 12, 12, 12, 13, 14, 15, 16, 17, 18, 19, 19, 20, 21, 22, 24, 25},
	{0, 1, 1, 1, 2, 2, 4, 4, 4, 5, 5, 5, 8, 9, 9, 10, 10, 11, 13, 14, 16, 17, 17, 18, 20, 21, 23, 25, 26, 28, 29, 31, 33, 35, 37, 38, 40, 43, 45, 47, 49},
	{0, 1, 1, 2, 2, 4, 4, 6, 6, 8, 8, 8, 10, 12, 16, 12, 17, 16, 18, 21, 20, 23, 23, 25, 27, 29, 34, 34, 35, 38, 40, 43, 45, 48, 51, 53, 56, 59, 62, 65, 68},
	{0, 1, 1, 2, 4, 4, 4, 5, 6, 8, 8, 11, 11, 16, 16, 18, 16, 19, 21, 25, 25, 25, 34, 30, 32, 35, 37, 40, 42, 45, 48, 51, 54, 57, 60, 63, 66, 70, 74, 77, 81},
}

// qrTooLongError reports content longer than a version-40 symbol holds at
// the requested level.
type QRTooLongError struct {
	Bytes    int
	MaxBytes int
	Level    ECLevel
}

func (e *QRTooLongError) Error() string {
	return fmt.Sprintf("%d bytes exceed the %d a version-40 QR Code holds at error-correction level %s", e.Bytes, e.MaxBytes, e.Level)
}

// QRSymbol is an encoded QR Code: Size x Size modules, row-major, true for
// dark. The quiet zone is not included.
type QRSymbol struct {
	Version int
	Level   ECLevel
	Mask    int
	Size    int
	Modules []bool
}

// Dark reports whether the module at column x, row y is dark.
func (s QRSymbol) Dark(x, y int) bool { return s.Modules[y*s.Size+x] }

// QRSize is the side of a version's symbol in modules.
func QRSize(version int) int { return 4*version + 17 }

// qrRawDataModules is the number of modules a version leaves for data and
// error-correction bits once every function pattern is placed, remainder
// bits included.
func qrRawDataModules(version int) int {
	result := (16*version+128)*version + 64
	if version >= 2 {
		align := version/7 + 2
		result -= (25*align-10)*align - 55
		if version >= 7 {
			result -= 36
		}
	}
	return result
}

// qrDataCodewords is how many data codewords a version holds at a level.
func qrDataCodewords(version int, level ECLevel) int {
	return qrRawDataModules(version)/8 - qrECCPerBlock[level][version]*qrBlocks[level][version]
}

// qrCountBits is byte mode's character-count field width for a version.
func qrCountBits(version int) int {
	if version <= 9 {
		return 8
	}
	return 16
}

// QRMaxBytes is how many content bytes a version holds at a level in byte
// mode.
func QRMaxBytes(version int, level ECLevel) int {
	n := (qrDataCodewords(version, level)*8 - 4 - qrCountBits(version)) / 8
	if limit := 1<<qrCountBits(version) - 1; n > limit {
		n = limit
	}
	return n
}

// QRVersionFor is the smallest version that holds n content bytes at a
// level, or false when even version 40 does not.
func QRVersionFor(n int, level ECLevel) (int, bool) {
	for v := QRMinVersion; v <= QRMaxVersion; v++ {
		if n <= QRMaxBytes(v, level) {
			return v, true
		}
	}
	return 0, false
}

// EncodeQR encodes data in byte mode at level. It fails only with a
// *QRTooLongError. Empty data encodes (a version-1 symbol with a zero count);
// the element never asks for one.
func EncodeQR(data []byte, level ECLevel) (QRSymbol, error) {
	if level < ECLevelL || level > ECLevelH {
		return QRSymbol{}, fmt.Errorf("barcode: unknown QR error-correction level %d", int(level))
	}
	version, ok := QRVersionFor(len(data), level)
	if !ok {
		return QRSymbol{}, &QRTooLongError{Bytes: len(data), MaxBytes: QRMaxBytes(QRMaxVersion, level), Level: level}
	}
	codewords := qrAddECCAndInterleave(qrDataBytes(data, version, level), version, level)
	g := newQRGrid(version)
	g.drawFunctionPatterns(level)
	g.drawCodewords(codewords)
	mask := g.chooseMask(level)
	return QRSymbol{Version: version, Level: level, Mask: mask, Size: g.size, Modules: g.modules}, nil
}

// qrDataBytes builds the data codewords: mode indicator 0100, the count,
// the bytes, a terminator of up to four zeros, zero bits to a byte boundary,
// then the alternating pad codewords 0xEC 0x11.
func qrDataBytes(data []byte, version int, level ECLevel) []byte {
	capacity := qrDataCodewords(version, level)
	var bb qrBitBuffer
	bb.append(0x4, 4)
	bb.append(len(data), qrCountBits(version))
	for _, b := range data {
		bb.append(int(b), 8)
	}
	capBits := capacity * 8
	if rest := capBits - bb.n; rest < 4 {
		bb.append(0, rest)
	} else {
		bb.append(0, 4)
	}
	if r := bb.n % 8; r != 0 {
		bb.append(0, 8-r)
	}
	for pad := 0xEC; bb.n < capBits; pad ^= 0xEC ^ 0x11 {
		bb.append(pad, 8)
	}
	return bb.bytes
}

type qrBitBuffer struct {
	bytes []byte
	n     int
}

func (b *qrBitBuffer) append(value, bits int) {
	for i := bits - 1; i >= 0; i-- {
		if b.n%8 == 0 {
			b.bytes = append(b.bytes, 0)
		}
		if (value>>i)&1 != 0 {
			b.bytes[b.n/8] |= 0x80 >> (b.n % 8)
		}
		b.n++
	}
}

// qrAddECCAndInterleave splits the data codewords into blocks (short blocks
// first, long blocks one codeword longer), appends each block's Reed-Solomon
// codewords, and interleaves data columns then error-correction columns.
func qrAddECCAndInterleave(data []byte, version int, level ECLevel) []byte {
	numBlocks := qrBlocks[level][version]
	eccLen := qrECCPerBlock[level][version]
	raw := qrRawDataModules(version) / 8
	numShort := numBlocks - raw%numBlocks
	shortDataLen := raw/numBlocks - eccLen
	divisor := ReedSolomonGenerator(eccLen)

	dataBlocks := make([][]byte, numBlocks)
	eccBlocks := make([][]byte, numBlocks)
	k := 0
	for i := 0; i < numBlocks; i++ {
		n := shortDataLen
		if i >= numShort {
			n++
		}
		dataBlocks[i] = data[k : k+n]
		eccBlocks[i] = ReedSolomonRemainder(dataBlocks[i], divisor)
		k += n
	}
	out := make([]byte, 0, raw)
	for i := 0; i <= shortDataLen; i++ {
		for _, block := range dataBlocks {
			if i < len(block) {
				out = append(out, block[i])
			}
		}
	}
	for i := 0; i < eccLen; i++ {
		for _, block := range eccBlocks {
			out = append(out, block[i])
		}
	}
	return out
}

// GF(256) with the QR field polynomial x^8+x^4+x^3+x^2+1 (0x11D), as
// integer exponent and logarithm tables.
var gfExp, gfLog = func() ([510]byte, [256]int) {
	var exp [510]byte
	var log [256]int
	x := 1
	for i := 0; i < 255; i++ {
		exp[i] = byte(x)
		log[x] = i
		x <<= 1
		if x&0x100 != 0 {
			x ^= 0x11D
		}
	}
	for i := 255; i < 510; i++ {
		exp[i] = exp[i-255]
	}
	return exp, log
}()

func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[gfLog[a]+gfLog[b]]
}

// ReedSolomonGenerator returns the coefficients of the degree-n generator
// polynomial (x - 2^0)(x - 2^1)...(x - 2^(n-1)), highest power first with
// the leading 1 omitted.
func ReedSolomonGenerator(n int) []byte {
	result := make([]byte, n)
	result[n-1] = 1
	root := byte(1)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			result[j] = gfMul(result[j], root)
			if j+1 < n {
				result[j] ^= result[j+1]
			}
		}
		root = gfMul(root, 2)
	}
	return result
}

// ReedSolomonRemainder returns the error-correction codewords of data for a
// generator from ReedSolomonGenerator.
func ReedSolomonRemainder(data, generator []byte) []byte {
	result := make([]byte, len(generator))
	for _, b := range data {
		factor := b ^ result[0]
		copy(result, result[1:])
		result[len(result)-1] = 0
		for i, g := range generator {
			result[i] ^= gfMul(g, factor)
		}
	}
	return result
}

// QRFormatBits is the 15-bit format information for a level and mask:
// BCH(15,5) with generator 0x537, XORed with 0x5412.
func QRFormatBits(level ECLevel, mask int) int {
	data := level.formatBits()<<3 | mask
	rem := data
	for i := 0; i < 10; i++ {
		rem = (rem << 1) ^ ((rem >> 9) * 0x537)
	}
	return (data<<10 | rem) ^ 0x5412
}

// QRVersionBits is the 18-bit version information for versions 7 and up:
// BCH(18,6) with generator 0x1F25.
func QRVersionBits(version int) int {
	rem := version
	for i := 0; i < 12; i++ {
		rem = (rem << 1) ^ ((rem >> 11) * 0x1F25)
	}
	return version<<12 | rem
}

// QRAlignmentPositions is the row/column centres of a version's alignment
// patterns (ISO/IEC 18004 Annex E), empty for version 1.
func QRAlignmentPositions(version int) []int {
	if version == 1 {
		return nil
	}
	num := version/7 + 2
	step := 26
	if version != 32 {
		step = (version*4 + num*2 + 1) / (num*2 - 2) * 2
	}
	result := make([]int, num)
	result[0] = 6
	for i, pos := num-1, QRSize(version)-7; i >= 1; i, pos = i-1, pos-step {
		result[i] = pos
	}
	return result
}

type qrGrid struct {
	version    int
	size       int
	modules    []bool
	isFunction []bool
}

func newQRGrid(version int) *qrGrid {
	size := QRSize(version)
	return &qrGrid{version: version, size: size, modules: make([]bool, size*size), isFunction: make([]bool, size*size)}
}

func (g *qrGrid) setFunction(x, y int, dark bool) {
	g.modules[y*g.size+x] = dark
	g.isFunction[y*g.size+x] = true
}

func (g *qrGrid) drawFunctionPatterns(level ECLevel) {
	for i := 0; i < g.size; i++ {
		g.setFunction(6, i, i%2 == 0)
		g.setFunction(i, 6, i%2 == 0)
	}
	g.drawFinder(3, 3)
	g.drawFinder(g.size-4, 3)
	g.drawFinder(3, g.size-4)
	positions := QRAlignmentPositions(g.version)
	last := len(positions) - 1
	for i, px := range positions {
		for j, py := range positions {
			if (i == 0 && j == 0) || (i == 0 && j == last) || (i == last && j == 0) {
				continue
			}
			g.drawAlignment(px, py)
		}
	}
	// Reserve the format areas (and draw the dark module) before data is
	// placed; the real bits are written once the mask is chosen.
	g.drawFormatBits(level, 0)
	g.drawVersionBits()
}

// drawFinder draws a finder pattern centred at (cx, cy) with its light
// separator, clipped to the symbol.
func (g *qrGrid) drawFinder(cx, cy int) {
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			x, y := cx+dx, cy+dy
			if x < 0 || x >= g.size || y < 0 || y >= g.size {
				continue
			}
			dist := max(abs(dx), abs(dy))
			g.setFunction(x, y, dist != 2 && dist != 4)
		}
	}
}

func (g *qrGrid) drawAlignment(cx, cy int) {
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			g.setFunction(cx+dx, cy+dy, max(abs(dx), abs(dy)) != 1)
		}
	}
}

func (g *qrGrid) drawFormatBits(level ECLevel, mask int) {
	bits := QRFormatBits(level, mask)
	bit := func(i int) bool { return (bits>>i)&1 != 0 }
	for i := 0; i <= 5; i++ {
		g.setFunction(8, i, bit(i))
	}
	g.setFunction(8, 7, bit(6))
	g.setFunction(8, 8, bit(7))
	g.setFunction(7, 8, bit(8))
	for i := 9; i < 15; i++ {
		g.setFunction(14-i, 8, bit(i))
	}
	for i := 0; i < 8; i++ {
		g.setFunction(g.size-1-i, 8, bit(i))
	}
	for i := 8; i < 15; i++ {
		g.setFunction(8, g.size-15+i, bit(i))
	}
	g.setFunction(8, g.size-8, true)
}

func (g *qrGrid) drawVersionBits() {
	if g.version < 7 {
		return
	}
	bits := QRVersionBits(g.version)
	for i := 0; i < 18; i++ {
		dark := (bits>>i)&1 != 0
		a, b := g.size-11+i%3, i/3
		g.setFunction(a, b, dark)
		g.setFunction(b, a, dark)
	}
}

// drawCodewords places the codeword bits in the standard two-column zigzag,
// right to left, skipping function modules and the vertical timing column.
// Remainder modules (past the last codeword bit) are left light here; they
// are not function modules, so the mask applies to them and can darken them.
func (g *qrGrid) drawCodewords(codewords []byte) {
	i := 0
	total := len(codewords) * 8
	for right := g.size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		upward := (right+1)&2 == 0
		for vert := 0; vert < g.size; vert++ {
			y := vert
			if upward {
				y = g.size - 1 - vert
			}
			for j := 0; j < 2; j++ {
				x := right - j
				if g.isFunction[y*g.size+x] || i >= total {
					continue
				}
				g.modules[y*g.size+x] = (codewords[i/8]>>(7-i%8))&1 != 0
				i++
			}
		}
	}
}

// qrMaskApplies is the standard's mask condition for column x, row y.
func qrMaskApplies(mask, x, y int) bool {
	switch mask {
	case 0:
		return (x+y)%2 == 0
	case 1:
		return y%2 == 0
	case 2:
		return x%3 == 0
	case 3:
		return (x+y)%3 == 0
	case 4:
		return (x/3+y/2)%2 == 0
	case 5:
		return x*y%2+x*y%3 == 0
	case 6:
		return (x*y%2+x*y%3)%2 == 0
	default:
		return ((x+y)%2+x*y%3)%2 == 0
	}
}

// applyMask XORs a mask over every non-function module; applying it twice
// restores the grid.
func (g *qrGrid) applyMask(mask int) {
	for y := 0; y < g.size; y++ {
		for x := 0; x < g.size; x++ {
			if !g.isFunction[y*g.size+x] && qrMaskApplies(mask, x, y) {
				g.modules[y*g.size+x] = !g.modules[y*g.size+x]
			}
		}
	}
}

// chooseMask scores all eight masks (format bits included) and leaves the
// grid masked with the lowest-penalty one; ties keep the lower number.
func (g *qrGrid) chooseMask(level ECLevel) int {
	best, bestScore := 0, -1
	for mask := 0; mask < 8; mask++ {
		g.applyMask(mask)
		g.drawFormatBits(level, mask)
		score := g.penalty()
		if bestScore < 0 || score < bestScore {
			best, bestScore = mask, score
		}
		g.applyMask(mask)
	}
	g.applyMask(best)
	g.drawFormatBits(level, best)
	return best
}

// Penalty weights N1-N4 (ISO/IEC 18004 7.8.3.1).
const (
	qrPenaltyN1 = 3
	qrPenaltyN2 = 3
	qrPenaltyN3 = 40
	qrPenaltyN4 = 10
)

// penalty is the standard's mask score: runs of five or more same-colour
// modules in a row or column (N1), 2x2 same-colour blocks (N2), 1:1:3:1:1
// finder-like patterns with four light modules on either side, the light
// area beyond the symbol counting as light (N3), and dark/light imbalance in
// 5% steps (N4).
func (g *qrGrid) penalty() int {
	result := 0
	n := g.size
	for pass := 0; pass < 2; pass++ {
		for a := 0; a < n; a++ {
			var history [7]int
			runDark := false
			runLen := 0
			for b := 0; b < n; b++ {
				var dark bool
				if pass == 0 {
					dark = g.modules[a*n+b]
				} else {
					dark = g.modules[b*n+a]
				}
				if dark == runDark {
					runLen++
					if runLen == 5 {
						result += qrPenaltyN1
					} else if runLen > 5 {
						result++
					}
					continue
				}
				qrAddHistory(&history, runLen, n)
				if !runDark {
					result += qrFinderLike(&history) * qrPenaltyN3
				}
				runDark = dark
				runLen = 1
			}
			// Terminate the line: the light border beyond it closes the run.
			if runDark {
				qrAddHistory(&history, runLen, n)
				runLen = 0
			}
			qrAddHistory(&history, runLen+n, n)
			result += qrFinderLike(&history) * qrPenaltyN3
		}
	}
	for y := 0; y < n-1; y++ {
		for x := 0; x < n-1; x++ {
			c := g.modules[y*n+x]
			if c == g.modules[y*n+x+1] && c == g.modules[(y+1)*n+x] && c == g.modules[(y+1)*n+x+1] {
				result += qrPenaltyN2
			}
		}
	}
	dark := 0
	for _, m := range g.modules {
		if m {
			dark++
		}
	}
	total := n * n
	k := (abs(dark*20-total*10)+total-1)/total - 1
	result += k * qrPenaltyN4
	return result
}

// qrAddHistory pushes a run length onto a line's run history, newest first.
// The first run of a line absorbs the light border before the symbol.
func qrAddHistory(history *[7]int, runLen, size int) {
	if history[0] == 0 {
		runLen += size
	}
	copy(history[1:], history[:6])
	history[0] = runLen
}

// qrFinderLike counts 1:1:3:1:1 dark-light-dark-light-dark patterns in the
// history (which ends on a light run) with at least four light modules on
// one side and one on the other: 0, 1 or 2.
func qrFinderLike(h *[7]int) int {
	m := h[1]
	core := m > 0 && h[2] == m && h[3] == m*3 && h[4] == m && h[5] == m
	count := 0
	if core && h[0] >= m*4 && h[6] >= m {
		count++
	}
	if core && h[6] >= m*4 && h[0] >= m {
		count++
	}
	return count
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// QRFit is a QR symbol's placement inside a box.
type QRFit struct {
	// ModuleWidth is the one side every module shares, in millipoints.
	ModuleWidth geom.Length
	// OffsetX and OffsetY place the symbol's top-left module relative to the
	// box's top-left corner.
	OffsetX, OffsetY geom.Length
}

// FitQR chooses the largest whole-millipoint module for which the symbol
// plus QRQuietZoneModules on each side fits the smaller of the box's width
// and height, and centres the symbol on both axes. ok is false when even a
// 1 mp module cannot fit.
func FitQR(size int, boxW, boxH geom.Length) (QRFit, bool) {
	total := geom.Length(size + 2*QRQuietZoneModules)
	side := min(boxW, boxH)
	if side <= 0 || size <= 0 {
		return QRFit{}, false
	}
	module := side / total
	if module < 1 {
		return QRFit{}, false
	}
	symbol := module * geom.Length(size)
	return QRFit{
		ModuleWidth: module,
		OffsetX:     geom.ScaleRound(boxW-symbol, 1, 2),
		OffsetY:     geom.ScaleRound(boxH-symbol, 1, 2),
	}, true
}

// QRRect is one drawn run of dark modules in a row, box-relative.
type QRRect struct {
	X, Y, W, H geom.Length
}

// QRRects lays a symbol out at fit: each row's dark modules merged into
// horizontal runs, top row first and left to right within a row.
func QRRects(sym QRSymbol, fit QRFit) []QRRect {
	var rects []QRRect
	m := fit.ModuleWidth
	for y := 0; y < sym.Size; y++ {
		for x := 0; x < sym.Size; {
			if !sym.Dark(x, y) {
				x++
				continue
			}
			start := x
			for x < sym.Size && sym.Dark(x, y) {
				x++
			}
			rects = append(rects, QRRect{
				X: fit.OffsetX + m*geom.Length(start),
				Y: fit.OffsetY + m*geom.Length(y),
				W: m * geom.Length(x-start),
				H: m,
			})
		}
	}
	return rects
}
