package barcode

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// decodeRuns is a TEST-ONLY Code 128 reader: it groups bar/space runs into
// symbols, checks START, the modulo-103 check character and STOP, and
// replays the code-set state machine. It shares nothing with Encode except
// the pattern table, whose own structure TestPatternTableInvariants pins.
func decodeRuns(runs []int) (string, error) {
	lookup := map[string]int{}
	for v, p := range patterns {
		lookup[p] = v
	}
	if len(runs) < 6+6+7 || (len(runs)-7)%6 != 0 {
		return "", fmt.Errorf("run count %d is not whole symbols", len(runs))
	}
	var values []int
	for i := 0; i < len(runs)-7; i += 6 {
		var b strings.Builder
		for _, w := range runs[i : i+6] {
			b.WriteByte(byte('0' + w))
		}
		v, ok := lookup[b.String()]
		if !ok || v == valueStop {
			return "", fmt.Errorf("unknown symbol %s", b.String())
		}
		values = append(values, v)
	}
	var stop strings.Builder
	for _, w := range runs[len(runs)-7:] {
		stop.WriteByte(byte('0' + w))
	}
	if stop.String() != patterns[valueStop] {
		return "", fmt.Errorf("missing STOP")
	}
	if len(values) < 2 || values[0] < 103 || values[0] > 105 {
		return "", fmt.Errorf("missing START")
	}
	check := values[len(values)-1]
	body := values[:len(values)-1]
	if checkValue(body) != check {
		return "", fmt.Errorf("check character %d, want %d", check, checkValue(body))
	}
	set := values[0] - 103
	var out []byte
	for i := 1; i < len(body); i++ {
		v := body[i]
		switch {
		case set == setC && v < 100:
			out = append(out, byte('0'+v/10), byte('0'+v%10))
		case v == valueCodeC:
			set = setC
		case v == valueCodeB && set != setB:
			set = setB
		case v == valueCodeA && set != setA:
			set = setA
		case v == valueShift && set != setC:
			i++
			other := setA
			if set == setA {
				other = setB
			}
			out = append(out, charIn(other, body[i]))
		default:
			out = append(out, charIn(set, v))
		}
	}
	return string(out), nil
}

func charIn(set, v int) byte {
	if set == setA && v >= 64 {
		return byte(v - 64)
	}
	return byte(v + 32)
}

func TestPatternTableInvariants(t *testing.T) {
	seen := map[string]bool{}
	for v, p := range patterns {
		want, elements := 11, 6
		if v == valueStop {
			want, elements = 13, 7
		}
		if len(p) != elements {
			t.Fatalf("value %d pattern %q has %d elements, want %d", v, p, len(p), elements)
		}
		sum, bars := 0, 0
		for i, c := range p {
			w := int(c - '0')
			if w < 1 || w > 4 {
				t.Fatalf("value %d pattern %q has width %d", v, p, w)
			}
			sum += w
			if i%2 == 0 {
				bars += w
			}
		}
		if sum != want {
			t.Errorf("value %d pattern %q spans %d modules, want %d", v, p, sum, want)
		}
		if v != valueStop && bars%2 != 0 {
			t.Errorf("value %d pattern %q has an odd bar module count %d (Code 128 parity)", v, p, bars)
		}
		if seen[p] {
			t.Errorf("value %d pattern %q is a duplicate", v, p)
		}
		seen[p] = true
	}
}

func TestKnownVectors(t *testing.T) {
	for _, c := range []struct {
		in   string
		want []int
	}{
		// Start B, the check character (104 + 55*1 + ... + 65*9) mod 103 = 88.
		{"Wikipedia", []int{104, 55, 73, 75, 73, 80, 69, 68, 73, 65, 88, 106}},
		// Start C: (105 + 12*1 + 34*2) mod 103 = 82.
		{"1234", []int{105, 12, 34, 82, 106}},
		// A carriage return starts in A: CR is value 77; A is 33; check
		// (103 + 33*1 + 77*2) mod 103 = 84.
		{"A\r", []int{103, 33, 77, 84, 106}},
	} {
		got, err := Encode(c.in)
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("Encode(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestSetSelectionIsMinimalAndDeterministic(t *testing.T) {
	for _, c := range []struct {
		in      string
		symbols int // data symbols, START/check/STOP excluded
	}{
		{"12", 1},
		{"123", 3},      // three set-B symbols, or C 12 then CODE B and 3: three either way
		{"a1234b", 6},   // four interior digits: switching saves nothing
		{"a123456b", 7}, // six interior digits: switching saves one
		{"\r\n", 2},     // set A
		{"a\rb", 4},     // B, SHIFT CR, b
		{"\r\ra", 4},    // A, A, SHIFT a
		{"0994000123456", 8},
	} {
		values, err := Encode(c.in)
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if got := len(values) - 3; got != c.symbols {
			t.Errorf("%q: %d data symbols %v, want %d", c.in, got, values, c.symbols)
		}
		again, _ := Encode(c.in)
		if !slices.Equal(values, again) {
			t.Errorf("%q: encoding is not deterministic", c.in)
		}
	}
}

// TestNeverLongerThanBruteForce compares the programme's length with an
// exhaustive search over every set sequence for short strings.
func TestNeverLongerThanBruteForce(t *testing.T) {
	alphabet := []byte{'0', '1', 'a', '\r', '~'}
	var inputs []string
	var grow func(prefix []byte)
	grow = func(prefix []byte) {
		if len(prefix) > 0 {
			inputs = append(inputs, string(prefix))
		}
		if len(prefix) == 5 {
			return
		}
		for _, c := range alphabet {
			grow(append(slices.Clone(prefix), c))
		}
	}
	grow(nil)
	for _, in := range inputs {
		values, err := Encode(in)
		if err != nil {
			t.Fatal(err)
		}
		want := bruteForce(in)
		if got := len(values) - 3; got != want {
			t.Errorf("%q: %d data symbols, brute force finds %d", in, got, want)
		}
	}
}

func bruteForce(s string) int {
	best := 1 << 30
	var walk func(i, set, cost int)
	walk = func(i, set, cost int) {
		if cost >= best {
			return
		}
		if i == len(s) {
			best = cost
			return
		}
		for other := 0; other < 3; other++ {
			if other != set {
				walkChar(s, i, other, cost+1, walk)
			}
		}
		walkChar(s, i, set, cost, walk)
	}
	for set := 0; set < 3; set++ {
		walkChar(s, 0, set, 0, walk)
	}
	return best
}

func walkChar(s string, i, set, cost int, next func(i, set, cost int)) {
	c := s[i]
	switch set {
	case setC:
		if i+1 < len(s) && isDigit(c) && isDigit(s[i+1]) {
			next(i+2, set, cost+1)
		}
	case setA:
		if c <= 95 {
			next(i+1, set, cost+1)
		} else {
			next(i+1, set, cost+2)
		}
	default:
		if c >= 32 {
			next(i+1, set, cost+1)
		} else {
			next(i+1, set, cost+2)
		}
	}
}

func TestRoundTripThroughTestDecoder(t *testing.T) {
	for _, in := range []string{
		"|099400012345600\r1234567890\r0000000001\r150000",
		"Wikipedia", "1234", "12345", "a\rb", "\x00\x7f", "~~~", "0", "a0123456789b", "\r\n\\",
	} {
		values, err := Encode(in)
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		got, err := decodeRuns(Runs(values))
		if err != nil {
			t.Fatalf("%q: decode: %v", in, err)
		}
		if got != in {
			t.Errorf("round trip %q -> %q", in, got)
		}
	}
}

func TestUnencodable(t *testing.T) {
	_, err := Encode("ab ก")
	var ue *UnencodableError
	if !errors.As(err, &ue) {
		t.Fatalf("want *UnencodableError, got %v", err)
	}
	if ue.Offset != 3 || ue.Rune != 'ก' {
		t.Errorf("offset %d rune %q, want 3 'ก'", ue.Offset, ue.Rune)
	}
	if _, err := Encode(""); err == nil {
		t.Error("empty content must not encode")
	}
}

func TestFitWidth(t *testing.T) {
	values, _ := Encode("1234") // 3*11 + 11 + 13 = 57 modules, 77 with quiet zones
	runs := Runs(values)
	if m := Modules(runs); m != 57 {
		t.Fatalf("modules = %d, want 57", m)
	}
	fit, ok := FitWidth(runs, 77*1000+76)
	if !ok || fit.ModuleWidth != 1000 {
		t.Fatalf("fit = %+v %v, want module 1000", fit, ok)
	}
	// Leftover 20*1000+76 is centred: 10038.
	if fit.Offset != 10038 {
		t.Errorf("offset = %d, want 10038", fit.Offset)
	}
	bars := Bars(runs, fit)
	last := bars[len(bars)-1]
	if right := last.X + last.W; right > 77*1000+76-10*1000 {
		t.Errorf("symbol ends at %d, inside the right quiet zone", right)
	}
	if _, ok := FitWidth(runs, 76); ok {
		t.Error("a 76 mp box cannot hold 77 modules")
	}
	if fit, ok := FitWidth(runs, 77); !ok || fit.ModuleWidth != 1 {
		t.Errorf("a 77 mp box holds 1 mp modules, got %+v %v", fit, ok)
	}
	if MinModuleWidth != geom.Length(709) {
		t.Errorf("MinModuleWidth = %d", MinModuleWidth)
	}
}
