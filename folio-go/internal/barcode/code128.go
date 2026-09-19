// Package barcode encodes content as a Code 128 symbol (ISO/IEC 15417) and
// fits that symbol into a box in whole millipoints.
//
// It is a pure encoder: no floats, no map ranging, no first-party import
// beyond internal/geom. It returns MODULE RUNS — alternating bar and space
// widths, bar first — and never draws anything; the module root turns runs
// into page-model rectangles.
package barcode

import (
	"fmt"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// QuietZoneModules is the minimum quiet zone on each side of the symbol,
// inside the element box.
const QuietZoneModules = 10

// MinModuleWidth is 0.25 mm rounded up to a whole millipoint. A module
// narrower than this still draws, with a Warning.
const MinModuleWidth geom.Length = 709

// Code set identifiers.
const (
	setA = 0
	setB = 1
	setC = 2
)

// Special symbol values.
const (
	valueShift = 98
	valueCodeC = 99
	valueCodeB = 100 // in sets A and C
	valueCodeA = 101 // in sets B and C
	valueStart = 103 // START A; START B is 104, START C is 105
	valueStop  = 106
)

// patterns is the Code 128 symbol table: bar/space widths for values 0-105
// (six elements, eleven modules) and STOP at 106 (seven elements, thirteen
// modules). Indexed by symbol value.
var patterns = [107]string{
	"212222", "222122", "222221", "121223", "121322", "131222", "122213", "122312", "132212", "221213",
	"221312", "231212", "112232", "122132", "122231", "113222", "123122", "123221", "223211", "221132",
	"221231", "213212", "223112", "312131", "311222", "321122", "321221", "312212", "322112", "322211",
	"212123", "212321", "232121", "111323", "131123", "131321", "112313", "132113", "132311", "211313",
	"231113", "231311", "112133", "112331", "132131", "113123", "113321", "133121", "313121", "211331",
	"231131", "213113", "213311", "213131", "311123", "311321", "331121", "312113", "312311", "332111",
	"314111", "221411", "431111", "111224", "111422", "121124", "121421", "141122", "141221", "112214",
	"112412", "122114", "122411", "142112", "142211", "241211", "221114", "413111", "241112", "134111",
	"111242", "121142", "121241", "114212", "124112", "124211", "411212", "421112", "421211", "212141",
	"214121", "412121", "111143", "111341", "131141", "114113", "114311", "411113", "411311", "113141",
	"114131", "311141", "411131", "211412", "211214", "211232", "2331112",
}

// UnencodableError reports a byte Code 128 cannot carry: anything above
// ASCII 127, including every byte of a multi-byte UTF-8 character.
type UnencodableError struct {
	// Offset is the byte offset of the first unencodable byte.
	Offset int
	// Rune is the character starting at Offset, for the message.
	Rune rune
}

func (e *UnencodableError) Error() string {
	return fmt.Sprintf("character %q (U+%04X) at byte %d is not ASCII; Code 128 encodes ASCII 0-127 only", e.Rune, e.Rune, e.Offset)
}

// FirstUnencodable returns the first byte offset in s that Code 128 cannot
// encode and true, or 0 and false when every byte is ASCII.
func FirstUnencodable(s string) (int, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return i, true
		}
	}
	return 0, false
}

// Encode returns the symbol values for s, START first and STOP last, with
// the modulo-103 check character before STOP. s must be non-empty.
//
// Code-set selection is a deterministic minimal-length dynamic programme over
// (position, current code set): it never produces a longer symbol than any
// other valid encoding, and ties keep the current set, then prefer B, C, A.
func Encode(s string) ([]int, error) {
	if s == "" {
		return nil, fmt.Errorf("barcode: empty content has no symbol")
	}
	if off, bad := FirstUnencodable(s); bad {
		r := rune(s[off])
		for _, decoded := range s[off:] {
			r = decoded
			break
		}
		return nil, &UnencodableError{Offset: off, Rune: r}
	}
	n := len(s)
	const inf = 1 << 30
	// stay[i][set]: fewest symbols encoding s[i:] when positioned in set and
	// emitting the next character(s) without a CODE switch first.
	// best[i][set]: the same, allowing one CODE switch before the next
	// character.
	stay := make([][3]int, n+1)
	best := make([][3]int, n+1)
	for set := 0; set < 3; set++ {
		stay[n][set], best[n][set] = 0, 0
	}
	for i := n - 1; i >= 0; i-- {
		c := s[i]
		// Set A: ASCII 0-95 directly, 96-127 through SHIFT.
		if c <= 95 {
			stay[i][setA] = 1 + best[i+1][setA]
		} else {
			stay[i][setA] = 2 + best[i+1][setA]
		}
		// Set B: ASCII 32-127 directly, 0-31 through SHIFT.
		if c >= 32 {
			stay[i][setB] = 1 + best[i+1][setB]
		} else {
			stay[i][setB] = 2 + best[i+1][setB]
		}
		// Set C: a digit pair.
		if i+1 < n && isDigit(c) && isDigit(s[i+1]) {
			stay[i][setC] = 1 + best[i+2][setC]
		} else {
			stay[i][setC] = inf
		}
		for set := 0; set < 3; set++ {
			v := stay[i][set]
			for _, other := range switchOrder(set) {
				if w := 1 + stay[i][other]; w < v {
					v = w
				}
			}
			best[i][set] = v
		}
	}

	start := setB
	for _, set := range [3]int{setB, setC, setA} {
		if stay[0][set] < stay[0][start] {
			start = set
		}
	}
	values := []int{valueStart + start}
	set := start
	for i := 0; i < n; {
		// Switch only when strictly shorter than staying.
		if stay[i][set] != best[i][set] {
			for _, other := range switchOrder(set) {
				if 1+stay[i][other] == best[i][set] {
					values = append(values, codeValue(other))
					set = other
					break
				}
			}
		}
		c := s[i]
		switch set {
		case setC:
			values = append(values, int(c-'0')*10+int(s[i+1]-'0'))
			i += 2
		case setA:
			if c <= 95 {
				values = append(values, valueInA(c))
			} else {
				values = append(values, valueShift, int(c)-32)
			}
			i++
		default:
			if c >= 32 {
				values = append(values, int(c)-32)
			} else {
				values = append(values, valueShift, valueInA(c))
			}
			i++
		}
	}
	values = append(values, checkValue(values))
	values = append(values, valueStop)
	return values, nil
}

// Runs returns the bar/space module widths for a symbol's values, bar first.
func Runs(values []int) []int {
	runs := make([]int, 0, len(values)*6+1)
	for _, v := range values {
		for _, w := range patterns[v] {
			runs = append(runs, int(w-'0'))
		}
	}
	return runs
}

// Modules is the total module count of runs, quiet zones excluded.
func Modules(runs []int) int64 {
	var total int64
	for _, w := range runs {
		total += int64(w)
	}
	return total
}

// Fit is a symbol's placement inside a box of width boxW.
type Fit struct {
	// ModuleWidth is the one width every module shares, in millipoints.
	ModuleWidth geom.Length
	// Offset is the distance from the box's left edge to the first bar.
	Offset geom.Length
}

// FitWidth chooses the largest whole-millipoint module width for which the
// symbol plus a quiet zone of QuietZoneModules on each side fits boxW, and
// centres the symbol. ok is false when even a 1 mp module cannot fit.
func FitWidth(runs []int, boxW geom.Length) (Fit, bool) {
	symbol := Modules(runs)
	total := symbol + 2*QuietZoneModules
	if boxW <= 0 || total <= 0 {
		return Fit{}, false
	}
	module := boxW / geom.Length(total)
	if module < 1 {
		return Fit{}, false
	}
	offset := geom.ScaleRound(boxW-module*geom.Length(symbol), 1, 2)
	return Fit{ModuleWidth: module, Offset: offset}, true
}

// Bar is one drawn bar: its left edge relative to the box and its width.
type Bar struct {
	X, W geom.Length
}

// Bars lays runs out at fit, returning only the bars (spaces are the page).
func Bars(runs []int, fit Fit) []Bar {
	bars := make([]Bar, 0, len(runs)/2+1)
	x := fit.Offset
	for i, w := range runs {
		width := fit.ModuleWidth * geom.Length(w)
		if i%2 == 0 {
			bars = append(bars, Bar{X: x, W: width})
		}
		x += width
	}
	return bars
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// valueInA is a set-A value for ASCII 0-95.
func valueInA(c byte) int {
	if c < 32 {
		return int(c) + 64
	}
	return int(c) - 32
}

// switchOrder lists the sets reachable from set by a CODE symbol, in the
// tie-break order B, C, A.
func switchOrder(set int) []int {
	switch set {
	case setA:
		return []int{setB, setC}
	case setB:
		return []int{setC, setA}
	default:
		return []int{setB, setA}
	}
}

// codeValue is the CODE symbol that switches into set to. CODE C is 99 from A
// or B, CODE B is 100 from A or C, CODE A is 101 from B or C.
func codeValue(to int) int {
	switch to {
	case setC:
		return valueCodeC
	case setB:
		return valueCodeB
	default:
		return valueCodeA
	}
}

// checkValue is the modulo-103 check: the start value plus each following
// value weighted by its 1-based position.
func checkValue(values []int) int {
	sum := values[0]
	for i := 1; i < len(values); i++ {
		sum += i * values[i]
	}
	return sum % 103
}
