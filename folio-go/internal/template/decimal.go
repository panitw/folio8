package template

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/panitw/folio8/folio-go/internal/geom"
)

// This file is the "points" numeric kind's exact decimal path (AC24,
// D-1.4.3): a json.Number literal (never a float64 — encoding/json's
// default decodes every number to float64, which is why UseNumber is
// mandatory, AC26) is converted to geom.Length millipoints by exact
// ×1000 decimal-string arithmetic (math/big.Int only — never
// strconv.ParseFloat, never float64). The on-disk spelling this
// produces (appendPoints) matches internal/pdf's appendLength byte for
// byte (AC25), by independent reimplementation rather than a shared
// symbol, because internal/template may not import internal/pdf
// (AD-1). folio-go/testdata/template/length-spelling-cases.txt is the
// shared value table (AC25) both packages' tests assert against, so a
// future divergence is a red test rather than a byte surprise.

// decodePoints converts a JSON number literal (as produced by
// json.Decoder.UseNumber) into millipoints. Non-canonical but valid
// spellings are accepted and normalised (AC27): "36.0000" and "1e3" are
// both legal. A value that is not an exact whole number of millipoints
// is a load error (AC28) — nothing is rounded. int64 millipoint
// overflow is a load error, never a wrap (AC27).
func decodePoints(literal string) (geom.Length, error) {
	sign, intPart, fracPart, exp, err := SplitJSONNumber(literal)
	if err != nil {
		return 0, err
	}

	// digits*10^(exp - len(fracPart)) is the exact decimal value; we
	// want that value * 1000, i.e. digits * 10^(exp - len(fracPart) + 3).
	digitsStr := intPart + fracPart
	digits := new(big.Int)
	if _, ok := digits.SetString(digitsStr, 10); !ok {
		return 0, fmt.Errorf("template: invalid numeric literal %q", literal)
	}

	shift := exp - len(fracPart) + 3

	var millipoints *big.Int
	if shift >= 0 {
		pow := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(shift)), nil)
		millipoints = new(big.Int).Mul(digits, pow)
	} else {
		pow := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-shift)), nil)
		q, r := new(big.Int), new(big.Int)
		q.QuoRem(digits, pow, r)
		if r.Sign() != 0 {
			return 0, fmt.Errorf("template: value %q has more than three decimal places (not an exact whole number of millipoints)", literal)
		}
		millipoints = q
	}

	if sign < 0 {
		millipoints.Neg(millipoints)
	}

	if !millipoints.IsInt64() {
		return 0, fmt.Errorf("template: value %q overflows int64 millipoints", literal)
	}
	return geom.Length(millipoints.Int64()), nil
}

// SplitJSONNumber splits a valid JSON number literal into its sign
// (1 or -1), integer digit string, fractional digit string (without the
// '.') and decimal exponent (0 if absent). It does not itself validate
// full JSON number grammar — literal always comes from
// encoding/json's own number scanner (via json.Number), which already
// guarantees that.
//
// This is the module's ONE splitter (AC6, D-1.6.1): internal/expr's
// Decimal (Story 1.6; moved to internal/expr at Story 3.2/D-3.2.1 — F12,
// Story 3.3, corrects this comment's stale internal/bind citation)
// reuses it rather than duplicating it, which is why it is exported —
// internal/expr importing internal/template is the correct direction
// (Document -> BoundTree). Both consumers
// (decodePoints here, and internal/expr.NewDecimal) share the exponent
// bound this function enforces (see parseDecimalExponent) and each
// layers its own tighter check on top (AC4b, D-1.6.6).
func SplitJSONNumber(literal string) (sign int, intPart, fracPart string, exp int, err error) {
	s := literal
	sign = 1
	if strings.HasPrefix(s, "-") {
		sign = -1
		s = s[1:]
	}

	mantissa := s
	expPart := ""
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		mantissa = s[:i]
		expPart = s[i+1:]
	}

	if i := strings.IndexByte(mantissa, '.'); i >= 0 {
		intPart = mantissa[:i]
		fracPart = mantissa[i+1:]
	} else {
		intPart = mantissa
	}

	if expPart != "" {
		e, perr := parseDecimalExponent(expPart)
		if perr != nil {
			return 0, "", "", 0, fmt.Errorf("template: invalid exponent in numeric literal %q: %w", literal, perr)
		}
		exp = e
	}
	return sign, intPart, fracPart, exp, nil
}

// MaxSplitExponentMagnitude is SplitJSONNumber's own bound on the
// magnitude of a parsed decimal exponent (D-1.6.6, AC4/AC4a/AC4b). It
// is deliberately the WIDER of this module's two known consumer
// bounds — decodePoints' implicit millipoint range (a handful of
// significant digits, always far inside this bound) and
// internal/expr.Decimal's own maxDecimalExponentMagnitude (100,000) —
// so this shared splitter never refuses a literal either consumer
// could legally represent (AC4b): each consumer then applies its own
// tighter check on top of what this bound already let through.
//
// QA Finding 7 (this story's review, Minor): this was previously
// 1,000,000 — 10x the actual wider consumer bound — which let ~900,000
// exponent values no consumer can represent still reach decodePoints'
// big.Int.Exp (measured: 33.0ms of repeated-squaring work at the old
// bound, versus 29µs one past the corrected bound). Not a hang or a
// regression (the loader aborts on the first bad element, so this was
// bounded, input-proportional work, never a DoS) — but AC4a-ii's
// "rejected BEFORE any big.Int scaling is attempted" was only true
// above the old bound, and the decade of headroom was pure attack
// surface no consumer could use. Set to exactly
// internal/expr.maxDecimalExponentMagnitude (moved there from
// internal/bind at Story 3.2/D-3.2.1 — F12, Story 3.3), the wider of
// the two known consumers today.
//
// The specific number is illustrative (AC4, mechanism note); that a
// bound exists, is enforced before any big.Int scaling is attempted,
// and is documented, is binding.
const MaxSplitExponentMagnitude = 100_000

// parseDecimalExponent parses s (the digits after 'e'/'E', with an
// optional leading sign already known to belong here) into a decimal
// exponent.
//
// Two fixes, both required (D-1.6.6, AC4a) — this function used to
// accumulate with `n = n*10 + int(c-'0')` and no overflow check at
// all, so an absurd exponent like "99999999999999999999" silently
// wrapped into an arbitrary int64 (frequently math.MinInt64), which
// then reached big.Int.Exp's repeated squaring in decodePoints and
// HUNG the process — reachable from a syntactically valid `.folio`
// through the public folio8.LoadTemplate (measured, story 1.6 Dev
// Notes M-1; the defect is recorded against Story 1.4's "malformed …
// templates rejected with a located error" promise).
//
//  1. An overflow check DURING accumulation, not after: checking only
//     after the loop would read a value that has already wrapped.
//  2. A documented magnitude bound (MaxSplitExponentMagnitude),
//     rejected here — before any consumer ever attempts a big.Int
//     scaling operation. Rejecting only once the scaling has begun
//     does not prevent the hang; the hang IS the scaling.
func parseDecimalExponent(s string) (int, error) {
	neg := false
	if strings.HasPrefix(s, "+") {
		s = s[1:]
	} else if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	if s == "" {
		return 0, fmt.Errorf("empty exponent")
	}
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("non-digit %q in exponent", c)
		}
		d := int(c - '0')
		// Fix 1 (AC4a-i): overflow check DURING accumulation. n is
		// always >= 0 here, so n*10+d overflows exactly when
		// n > (MaxInt-d)/10 — this is checked BEFORE the multiply,
		// never after, so n itself never wraps.
		if n > (maxInt-d)/10 {
			return 0, fmt.Errorf("exponent %q overflows during parsing", s)
		}
		n = n*10 + d
	}
	// Fix 2 (AC4a-ii): the documented magnitude bound, rejected here —
	// before SplitJSONNumber ever returns to a caller that might feed
	// this into big.Int.Exp. n cannot have wrapped (Fix 1), so this
	// comparison is meaningful.
	if n > MaxSplitExponentMagnitude {
		return 0, fmt.Errorf("exponent magnitude %d exceeds the maximum of %d", n, MaxSplitExponentMagnitude)
	}
	if neg {
		n = -n
	}
	return n, nil
}

// maxInt is the platform int's maximum value, spelled without an
// import (this file already imports math/big, but keeping this literal
// avoids pulling in "math" just for one constant used solely by Fix 1's
// overflow check above).
const maxInt = int(^uint(0) >> 1)

// appendPoints appends the canonical on-disk spelling of a geom.Length
// to dst: sign, integer part, and up to three fractional digits with
// trailing zeros (and a bare trailing point) trimmed. This is a
// deliberate byte-for-byte reimplementation of internal/pdf's
// appendLength (AC25) — see this file's header comment for why it is
// not shared, and length-spelling_test.go for the cross-check that
// keeps the two in step.
func appendPoints(dst []byte, v geom.Length) []byte {
	n := int64(v)
	negative := n < 0

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

	dst = appendPlainInt(dst, i)

	if f != 0 {
		dst = append(dst, '.')
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

// FormatPoints spells a geom.Length the way the FORMAT spells it on disk
// (appendPoints — the canonical exact-decimal spelling), so a diagnostic can
// quote a length in the AUTHOR'S units: an author who typed `80` is told about
// `80`, never about `80000mp`.
//
// Exported for the same reason FormatLineSpacing is: the command doors live in
// the module root (folio-go/component_commands.go) and must word a length the
// way this package words it, and a second spelling of appendPoints in the root
// package is exactly the drift appendPoints exists to prevent.
func FormatPoints(v geom.Length) string {
	return string(appendPoints(nil, v))
}

// appendPlainInt appends the plain decimal digits of a non-negative
// int64 to dst (used for both appendPoints' integer part and NextID).
func appendPlainInt(dst []byte, v int64) []byte {
	if v == 0 {
		return append(dst, '0')
	}
	var buf [20]byte
	pos := len(buf)
	for v != 0 {
		pos--
		buf[pos] = byte('0' + v%10)
		v /= 10
	}
	return append(dst, buf[pos:]...)
}
