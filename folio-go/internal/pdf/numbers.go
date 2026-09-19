package pdf

import "github.com/panitw/folio8/folio-go/internal/geom"

// This file is the module's single point of numeric formatting (AD-3).
// AD-3's hazard is float formatting reaching an output byte — a millipoint
// value serialized through a platform-visible float representation (e.g.
// strconv.FormatFloat, fmt's %g/%f/%v) would break byte-identity across
// machines. Plain integers (counts, byte offsets, object and generation
// numbers) have no such hazard: their decimal text form has no platform
// variance, and they were never in AD-3's scope. Routing a 12345-byte xref
// offset through appendLength would wrongly emit "12.345" — offsets and
// counts are not geom.Length values scaled by 1000, they are already the
// number they represent.
//
// Exactly three unexported functions live here, and this is the only file
// under internal/pdf allowed to reference strconv.Format*, strconv.Itoa,
// strconv.Append*, or a fmt formatting call. TestNumberFormattingIsConfinedToNumbersGo
// in emit_source_test.go enforces this file-scoped rule with no
// exemptions — including for _test.go files — brought forward from
// D-1.1.b, which originally slated the rule for Story 1.3's lint; it is
// enforced starting in this story because this is the story where the
// hole it closes actually exists (this story's QA review, Blocker 2).
// The functions in this file themselves use only exact integer
// arithmetic — none of them call strconv or fmt — so the rule costs this
// file nothing.

// appendLength appends the decimal text form of a geom.Length to dst and
// returns the extended slice. The representation is sign, integer part,
// and up to three fractional digits with trailing zeros (and a bare
// trailing point) trimmed — computed entirely by integer arithmetic, never
// rounded here (rounding happens only when a value is converted into
// thousandths in the first place, per AD-2).
//
// For v, let n = |v|, i = n / 1000, f = n % 1000 (both exact integer
// operations). Emit "-" if v < 0 and the result is not zero; emit i; if
// f != 0, emit "." then f zero-padded to three digits with trailing zeros
// stripped. Never emits "-0" and never a bare trailing ".".
func appendLength(dst []byte, v geom.Length) []byte {
	n := int64(v)
	negative := n < 0

	// Divide first, negate the (much smaller-magnitude) quotient and
	// remainder afterward. Negating n itself would overflow when
	// v == math.MinInt64, since int64's negative range is one wider than
	// its positive range; i and f are always well inside int64's
	// positive range, so negating them is safe. Go's integer division
	// truncates toward zero, so i and f share n's sign (or are zero) —
	// exactly the "take the absolute value first" case the naive
	// approach gets wrong.
	i := n / 1000
	f := n % 1000
	if i < 0 {
		i = -i
	}
	if f < 0 {
		f = -f
	}

	if negative && (i != 0 || f != 0) {
		dst = append(dst, '-')
	}

	dst = appendInt(dst, i)

	if f != 0 {
		dst = append(dst, '.')
		// f is in [1, 999]; zero-pad to three digits then trim trailing
		// zeros.
		digits := [3]byte{
			byte('0' + f/100),
			byte('0' + (f/10)%10),
			byte('0' + f%10),
		}
		end := 3
		for end > 0 && digits[end-1] == '0' {
			end--
		}
		dst = append(dst, digits[:end]...)
	}

	return dst
}

// appendInt appends the plain decimal digits of v (no scaling, no
// separators) to dst and returns the extended slice.
func appendInt(dst []byte, v int64) []byte {
	if v == 0 {
		return append(dst, '0')
	}

	negative := v < 0
	// Handle math.MinInt64 without overflowing on negation: work in the
	// negative domain (Go integer division/remainder truncate toward
	// zero, so this is exact for the min value too).
	var buf [20]byte
	pos := len(buf)
	if negative {
		for v != 0 {
			pos--
			buf[pos] = byte('0' - v%10)
			v /= 10
		}
		dst = append(dst, '-')
	} else {
		for v != 0 {
			pos--
			buf[pos] = byte('0' + v%10)
			v /= 10
		}
	}
	return append(dst, buf[pos:]...)
}

// appendIntPadded appends the plain decimal digits of v, left zero-filled
// so that at least width digits are emitted (excluding any sign — a
// negative value's "-" is not counted against width, and is never
// zero-filled itself: appendIntPadded(dst, -1, 5) appends "-00001", six
// characters), to dst and returns the extended slice. It delegates digit
// production to appendInt — there is exactly one place integer digits are
// produced — and left-pads the digit run.
func appendIntPadded(dst []byte, v int64, width int) []byte {
	start := len(dst)
	dst = appendInt(dst, v)
	written := len(dst) - start

	negSign := 0
	if v < 0 {
		negSign = 1
	}
	digits := written - negSign

	if digits >= width {
		return dst
	}
	padCount := width - digits

	// Make room and shift the digits (after any sign) right, then fill
	// the gap with zeros.
	dst = append(dst, make([]byte, padCount)...)
	copy(dst[start+negSign+padCount:], dst[start+negSign:start+written])
	for i := 0; i < padCount; i++ {
		dst[start+negSign+i] = '0'
	}
	return dst
}
