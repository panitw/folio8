// Decimal (D-1.6.1, moved here by Story 3.2/D-3.2.1, DW-8) is AD-23's
// exact scaled-integer decimal: an int64 coefficient and an integral
// decimal exponent, carrying a JSON number literal's own precision.
// See doc.go for the package's own overview and D-3.2.1 for why this
// type lives here rather than in internal/bind (the rank guard forces
// it: internal/expr (3) may import internal/template (2) for the
// module's one SplitJSONNumber; internal/bind (4) imports expr back).
package expr

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/panitw/folio8/folio-go/internal/template"
)

// Decimal is AD-23's exact scaled-integer decimal (D-1.6.1, AC1): an
// int64 coefficient and an integral decimal exponent, carrying a JSON
// number literal's own precision. This is NOT geom.Length: millipoints
// are a fixed three-decimal scale for geometry, and a report-data
// literal (a bank-statement figure, for instance) carries whatever
// precision its author wrote — "1.50" and "1.5" are DISTINCT values
// (AC2), which a fixed-scale type cannot represent.
//
// The value Decimal represents is Coefficient * 10^Exponent.
type Decimal struct {
	Coefficient int64
	Exponent    int
}

// maxDecimalCoefficientDigits bounds Decimal's coefficient by
// SIGNIFICANT digits only (AC3): leading zeros in the literal's
// concatenated digit string are stripped before counting; trailing
// zeros are never stripped (stripping them would collapse "1.50" into
// {15,-1} and violate AC2's distinctness). 19 is the largest digit
// count guaranteed to fit inside int64's positive range in every case;
// SetString+IsInt64 below still rejects an all-9s 19-digit string that
// exceeds MaxInt64.
const maxDecimalCoefficientDigits = 19

// maxDecimalExponentMagnitude is Decimal's OWN tighter exponent bound
// (AC4b, D-1.6.6), applied on top of the shared splitter's WIDER bound
// (template.MaxSplitExponentMagnitude). A future story that expands a
// Decimal's digits at render time (the way decodePoints already does
// for millipoints) would otherwise be able to construct a value whose
// expansion is enormous even though its exponent fit inside the
// splitter's shared, more permissive bound — so this consumer narrows
// further, exactly as AC4b's layering requires. The specific number is
// illustrative; that it exists, is checked against the CONSTRUCTED
// Decimal.Exponent (exp - len(fracPart), not the literal's raw "e"
// exponent — QA Finding 3, this story's review: a literal with no "e"
// notation at all, or one that stacks a long fractional part on top of
// its own exponent, never touched the old raw-exp check, and could
// still construct an Exponent far past this bound), and is strictly
// narrower than the splitter's raw-exponent bound, is binding.
const maxDecimalExponentMagnitude = 100_000

// NewDecimal converts a JSON number literal (as produced by
// json.Decoder.UseNumber) into a Decimal (AC1, AC6). It reuses
// internal/template.SplitJSONNumber — the module's one splitter — for
// sign/digit/exponent decomposition, and never re-derives any of that
// parsing itself.
//
// A coefficient that does not fit int64 (by significant digit count,
// AC3) or an exponent whose magnitude exceeds this package's own bound
// (AC4b) is a located error, never a truncation and never a wrap.
func NewDecimal(literal string) (Decimal, error) {
	sign, intPart, fracPart, exp, err := template.SplitJSONNumber(literal)
	if err != nil {
		return Decimal{}, fmt.Errorf("expr: %w", err)
	}

	// AC3a: intPart+fracPart may strip to the EMPTY string (every
	// digit is a leading zero, e.g. "0.00") — that is legal, and the
	// scale must still survive as Exponent even though Coefficient is
	// 0 (never an error, never a lost scale).
	digits := intPart + fracPart
	sig := strings.TrimLeft(digits, "0")

	// QA Finding 3 (this story's review, Major): the bound must apply
	// to the CONSTRUCTED exponent, not the literal's raw "e" exponent.
	// A literal with no "e" notation at all (e.g. "0." + 199999 zeros
	// + "1") never touches exp, and a literal that stacks a long
	// fracPart on top of its own exponent (e.g. "0." + 999999 zeros +
	// "1e-100000") combines the two — both previously constructed an
	// Exponent whose magnitude vastly exceeded this bound with
	// err == nil. Checking here, after exponent is computed, closes
	// both gaps; AC4b's layering (this bound strictly narrower than
	// the splitter's raw-exp bound) still holds because len(fracPart)
	// only ever makes the constructed value MORE negative, never less
	// bounded than the raw exp alone.
	exponent := exp - len(fracPart)
	if exponent > maxDecimalExponentMagnitude || exponent < -maxDecimalExponentMagnitude {
		return Decimal{}, fmt.Errorf(
			"expr: exponent magnitude exceeds %d (value %q)", maxDecimalExponentMagnitude, literal,
		)
	}

	if sig == "" {
		return Decimal{Coefficient: 0, Exponent: exponent}, nil
	}

	if len(sig) > maxDecimalCoefficientDigits {
		return Decimal{}, fmt.Errorf(
			"expr: coefficient has %d significant digits, exceeds %d (does not fit int64); value %q",
			len(sig), maxDecimalCoefficientDigits, literal,
		)
	}

	coeff := new(big.Int)
	if _, ok := coeff.SetString(sig, 10); !ok {
		return Decimal{}, fmt.Errorf("expr: invalid numeric literal %q", literal)
	}
	if sign < 0 {
		coeff.Neg(coeff)
	}
	if !coeff.IsInt64() {
		return Decimal{}, fmt.Errorf("expr: value %q: coefficient does not fit int64", literal)
	}

	return Decimal{Coefficient: coeff.Int64(), Exponent: exponent}, nil
}

// Text is the exact decimal a number prints as when it is bound into text
// (owner decision 2026-09-13, revising AD-14 for numbers in text only): the
// coefficient's digits with a '.' placed by Exponent, keeping the scale.
// There is no exponent notation, grouping, locale or rounding, and no float
// on the path: {123450,-2} is "1234.50", {1,3} is "1000", {-35,-1} is
// "-3.5", {0,-3} is "0.000". A zero coefficient never prints a sign, so -0
// is "0". formatNumber stays the way to get styled output.
//
// The absolute value is taken in uint64, so math.MinInt64's coefficient
// prints its digits rather than overflowing back to a negative value.
//
// A positive Exponent appends that many zeros. It is bounded by
// maxDecimalExponentMagnitude on every decoded or computed Decimal.
func (d Decimal) Text() string {
	neg := d.Coefficient < 0
	mag := uint64(d.Coefficient)
	if neg {
		mag = -mag
	}
	digits := strconv.FormatUint(mag, 10)
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	switch {
	case d.Exponent >= 0:
		b.WriteString(digits)
		if mag != 0 {
			b.WriteString(strings.Repeat("0", d.Exponent))
		}
	default:
		scale := -d.Exponent
		if len(digits) <= scale {
			b.WriteString("0.")
			b.WriteString(strings.Repeat("0", scale-len(digits)))
			b.WriteString(digits)
		} else {
			b.WriteString(digits[:len(digits)-scale])
			b.WriteByte('.')
			b.WriteString(digits[len(digits)-scale:])
		}
	}
	return b.String()
}
