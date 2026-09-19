package expr

import (
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

// This file is formatDate (AC7-AC9): the second of the two functions
// this story flips to implemented (AC16).

// rfc3339Pattern accepts an RFC 3339 timestamp carrying its own offset
// or "Z" (AC7). Fractional seconds are optional and of any length;
// only the first 3 digits are used (millisecond precision — see
// fracSecondsToMillis).
var rfc3339Pattern = regexp.MustCompile(
	`^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?(Z|[+-]\d{2}:\d{2})$`,
)

// parseUTCOffsetMinutes parses "Z" or "±HH:MM" into a signed minute
// count.
//
// ONE FUNCTION, TWO POPULATIONS, AND IT ADMITS THE UNION OF WHAT ITS
// TWO CALLERS NEED (D-12.C):
//
//   - the DOCUMENT's own required `utcOffset` field — the call in
//     evalFormatDate, which passes fc.UTCOffset — which IS
//     syntax-checked at load, by internal/template's IsUTCOffset — and
//     which, since Story 12.2, is range-checked there too. The claim
//     used to be made here and was FALSE: template's pattern was
//     `^[+-][0-9]{2}:[0-9]{2}$`, so `+99:99` loaded and then failed
//     HERE, at render, for every formatDate in the document. This
//     comment is what made that gap invisible; it became true with
//     Story 12.2's repair of the loader's predicate, and D-000.10
//     applies to accurate-SOUNDING comments as much as to stale
//     rulings. The check below stays anyway: Canvas(t) reads no
//     UTCOffset (D-12.1), so there is no backstop, and a redundant
//     check INSIDE one package is cheap — what this project refuses is
//     a redundant check across an authority boundary.
//   - an RFC 3339 string's own embedded offset — the two calls that pass
//     m[8], in ParseRFC3339 and instantMsFromValue — which is
//     report DATA and is checked only here, since Parse/Check never
//     see it.
//
// "Z" IS FOR THE SECOND CALLER AND IS NOT A DIVERGENCE. It is RFC
// 3339's canonical UTC spelling and the data path requires it; the
// document field refuses it on one ground — it is not `±HH:MM`, the
// syntax folio-format.md's `utcOffset` field-table row states
// (D-12.C). Nothing
// about byte identity is involved: template's own pattern admits
// `-00:00` as readily as `+00:00`, and offsets travel verbatim. The tie
// in offset_divergence_test.go asserts exactly that asymmetry, by name.
func parseUTCOffsetMinutes(s string) (int, error) {
	if s == "Z" {
		return 0, nil
	}
	if len(s) != 6 || (s[0] != '+' && s[0] != '-') || s[3] != ':' {
		return 0, fmt.Errorf("expr: invalid UTC offset %q: must be \"Z\" or match ±HH:MM", s)
	}
	hh, err1 := strconv.Atoi(s[1:3])
	mm, err2 := strconv.Atoi(s[4:6])
	if err1 != nil || err2 != nil || hh > 23 || mm > 59 {
		return 0, fmt.Errorf("expr: invalid UTC offset %q: must be \"Z\" or match ±HH:MM", s)
	}
	total := hh*60 + mm
	if s[0] == '-' {
		total = -total
	}
	return total, nil
}

// fracSecondsToMillis converts an RFC 3339 fractional-seconds capture
// (digits after the '.', of any length, possibly empty) to whole
// milliseconds: the first 3 digits, right-padded with zeros if fewer,
// truncated (never rounded) if more — this story's output granularity
// never needs sub-millisecond precision.
func fracSecondsToMillis(frac string) int {
	if frac == "" {
		return 0
	}
	if len(frac) > 3 {
		frac = frac[:3]
	}
	for len(frac) < 3 {
		frac += "0"
	}
	ms, _ := strconv.Atoi(frac)
	return ms
}

// decimalExactMilliseconds reports the exact integer millisecond count
// d denotes, using ONLY d.Coefficient/d.Exponent (big.Int arithmetic,
// tenPowLookup) — never a float64 conversion at any point (AC7's own
// red-proof: a millisecond count whose float64 round-trip is lossy is
// accepted or rejected identically before and after, because there is
// no float64 conversion on this path to BE lossy). A non-integral
// value (Exponent < 0 with nonzero trailing digits) is a located
// error.
func decimalExactMilliseconds(d Decimal) (int64, error) {
	if d.Exponent >= 0 {
		v := big.NewInt(d.Coefficient)
		if d.Exponent > 0 {
			p, err := tenPowLookup(d.Exponent)
			if err != nil {
				return 0, err
			}
			v.Mul(v, p)
		}
		if !v.IsInt64() {
			return 0, fmt.Errorf("expr: formatDate: epoch-millisecond value %s does not fit an int64", v.String())
		}
		return v.Int64(), nil
	}

	shift := -d.Exponent
	p, err := tenPowLookup(shift)
	if err != nil {
		return 0, err
	}
	rem := new(big.Int)
	q, _ := new(big.Int).QuoRem(big.NewInt(d.Coefficient), p, rem)
	if rem.Sign() != 0 {
		return 0, fmt.Errorf(
			"expr: formatDate: epoch-millisecond value is not an exact integer (carries %d fractional digit(s))", shift,
		)
	}
	if !q.IsInt64() {
		return 0, fmt.Errorf("expr: formatDate: epoch-millisecond value does not fit an int64")
	}
	return q.Int64(), nil
}

// validateCivilRanges rejects any capture that is not a real civil
// date/time BEFORE it reaches the calendar arithmetic (Finding 1 /
// AC7/AD-14): rfc3339Pattern above validates digit SHAPE only, so an
// impossible value — month 13, day 31 of February, hour 25 — must be
// caught HERE, named precisely, before daysFromCivil silently
// normalises it into a different, plausible-looking date. Order
// matters: month is checked before day, because the day-range check
// below assumes a valid month.
//
// The day check reuses the calendar itself — civilFromDays(daysFromCivil(...))
// must round-trip to the SAME (y, m, d) — rather than adding a second,
// independently-maintained month-length table: the calendar was
// measured exhaustively correct over all 3,652,059 proleptic Gregorian
// dates from 0001-01-01 to 9999-12-31 (this story's QA review), so a
// day that does not round-trip is a day that does not exist in that
// (year, month), including every leap-year case, for free.
func validateCivilRanges(year int64, month, day, hour, minute, second int) error {
	// QA Finding 14 (Story 3.7's review, Minor): R5 records the
	// calendar as "bounded to years 1-9999" (the exhaustive round-trip
	// measurement above is over exactly that range), but nothing
	// enforced the lower bound — year 0000 round-tripped through
	// civilFromDays(daysFromCivil(...)) cleanly, since the proleptic
	// Gregorian calendar is happy to represent it, and reached
	// ParseRFC3339's caller as a valid civil date outside ISO 32000-1's
	// representable calendar (PDF's D:YYYYMMDD... date string has no
	// defined meaning for a non-positive year). Checked first, before
	// month, so an out-of-range year is never masked by a month/day
	// message that assumes a representable year.
	if year < 1 || year > 9999 {
		return fmt.Errorf("year %04d is out of range 1-9999", year)
	}
	if month < 1 || month > 12 {
		return fmt.Errorf("month %d is out of range 1-12", month)
	}
	if y2, m2, d2 := civilFromDays(daysFromCivil(year, month, day)); y2 != year || m2 != month || d2 != day {
		return fmt.Errorf("day %d is out of range for %04d-%02d", day, year, month)
	}
	if hour < 0 || hour > 23 {
		return fmt.Errorf("hour %d is out of range 0-23", hour)
	}
	if minute < 0 || minute > 59 {
		return fmt.Errorf("minute %d is out of range 0-59", minute)
	}
	if second < 0 || second > 59 {
		return fmt.Errorf("second %d is out of range 0-59", second)
	}
	return nil
}

// Civil is a parsed RFC 3339 timestamp's civil (calendar) components,
// as exact integers — never seconds-since-epoch, never a time.Time.
// Story 3.7 (D-3.7.2, R5): this is the shipped RFC 3339 parser and
// integer calendar's public face, reused by package folio8 to validate
// and resolve the reserved `documentDate` params key without a second
// parser existing anywhere in the module (D-000.42), and without
// "time" ever entering any internal/ package (AD-1).
type Civil struct {
	Year                 int64
	Month, Day           int
	Hour, Minute, Second int
	// OffsetMinutes is signed, minutes east of UTC (0 for both "Z" and
	// "+00:00" — the two are indistinguishable once parsed).
	OffsetMinutes int
}

// ParseRFC3339 parses s as an RFC 3339 timestamp (its own offset or
// "Z"), validating both its shape and its civil range (a real
// calendar date/time, not merely digit-shaped) exactly as formatDate's
// own operand parsing does (rfc3339Pattern, validateCivilRanges,
// parseUTCOffsetMinutes) — this function does not duplicate that
// logic, it exposes it. Fractional seconds, if present, are accepted
// but discarded: Civil's granularity (and the PDF date syntax
// D-3.7.2 assembles it into) is whole seconds.
func ParseRFC3339(s string) (Civil, error) {
	m := rfc3339Pattern.FindStringSubmatch(s)
	if m == nil {
		return Civil{}, fmt.Errorf("expr: %q is not a valid RFC 3339 timestamp", s)
	}
	year, _ := strconv.Atoi(m[1])
	month, _ := strconv.Atoi(m[2])
	day, _ := strconv.Atoi(m[3])
	hour, _ := strconv.Atoi(m[4])
	minute, _ := strconv.Atoi(m[5])
	second, _ := strconv.Atoi(m[6])

	if verr := validateCivilRanges(int64(year), month, day, hour, minute, second); verr != nil {
		return Civil{}, fmt.Errorf("expr: %q is not a valid RFC 3339 timestamp: %s", s, verr)
	}

	offsetMin, err := parseUTCOffsetMinutes(m[8])
	if err != nil {
		return Civil{}, fmt.Errorf("expr: %q is not a valid RFC 3339 timestamp: %w", s, err)
	}

	return Civil{
		Year: int64(year), Month: month, Day: day,
		Hour: hour, Minute: minute, Second: second,
		OffsetMinutes: offsetMin,
	}, nil
}

// instantMsFromValue is AC7: accepts an RFC 3339 string (its own
// offset or "Z") or an integral epoch-millisecond number; rejects
// everything else with a located error naming the element and the
// offending value.
func instantMsFromValue(v Value, elementID, callRaw string) (int64, error) {
	switch v.Kind {
	case KindString:
		m := rfc3339Pattern.FindStringSubmatch(v.Str)
		if m == nil {
			return 0, fmt.Errorf(
				"expr: element %s: formatDate(): %q is not a valid RFC 3339 timestamp: %s",
				elementID, v.Str, callRaw,
			)
		}
		year, _ := strconv.Atoi(m[1])
		month, _ := strconv.Atoi(m[2])
		day, _ := strconv.Atoi(m[3])
		hour, _ := strconv.Atoi(m[4])
		minute, _ := strconv.Atoi(m[5])
		second, _ := strconv.Atoi(m[6])
		ms := fracSecondsToMillis(m[7])

		if verr := validateCivilRanges(int64(year), month, day, hour, minute, second); verr != nil {
			return 0, fmt.Errorf(
				"expr: element %s: formatDate(): %q is not a valid RFC 3339 timestamp: %s: %s",
				elementID, v.Str, verr, callRaw,
			)
		}

		offsetMin, err := parseUTCOffsetMinutes(m[8])
		if err != nil {
			return 0, fmt.Errorf("expr: element %s: formatDate(): %w: %s", elementID, err, callRaw)
		}

		localMs := daysFromCivil(int64(year), month, day)*msPerDay +
			int64(hour)*3600000 + int64(minute)*60000 + int64(second)*1000 + int64(ms)
		return localMs - int64(offsetMin)*60000, nil

	case KindNumber:
		ms, err := decimalExactMilliseconds(v.Num)
		if err != nil {
			return 0, fmt.Errorf("expr: element %s: formatDate(): %w: %s", elementID, err, callRaw)
		}
		return ms, nil

	default:
		return 0, fmt.Errorf(
			"expr: element %s: formatDate(): operand must be an RFC 3339 string or an integral epoch-millisecond number, got %s (never coerced): %s",
			elementID, v.Kind, callRaw,
		)
	}
}

// renderDatePattern renders civil (already era-converted at .Year, if
// applicable) according to tokens (AC8 step 3): field tokens read from
// civil/entry, everything else copied through verbatim (AC10a's
// literal rule). The digits emitted for yyyy/MM/M/dd/d are translated
// through entry.digits (AC11a / Finding 5), exactly as
// renderNumberPattern already does — a locale's digit set is read from
// the table for BOTH functions alike, never hardcoded ASCII here.
func renderDatePattern(tokens []dateToken, civil civilInstant, entry localeEntry) string {
	var b strings.Builder
	for _, t := range tokens {
		if !t.Field {
			b.WriteString(t.Text)
			continue
		}
		switch t.Text {
		case "yyyy":
			fmt.Fprintf(&b, "%04d", civil.Year)
		case "MMMM":
			b.WriteString(entry.monthNames[civil.Month-1])
		case "MM":
			fmt.Fprintf(&b, "%02d", civil.Month)
		case "M":
			fmt.Fprintf(&b, "%d", civil.Month)
		case "dd":
			fmt.Fprintf(&b, "%02d", civil.Day)
		case "d":
			fmt.Fprintf(&b, "%d", civil.Day)
		}
	}
	return translateDigits(b.String(), entry.digits)
}

// evalFormatDate is AC7-AC9's evaluator. call.Args[1] has already been
// validated as a recognised date pattern at Check (checkDatePattern),
// so parseDatePattern's error return is never expected to fire here —
// it is still checked, never ignored, per AD-14's "never a panic".
func evalFormatDate(call *CallExpr, resolver Resolver, fc FormatContext, elementID string) (Value, []Caveat, error) {
	entry, lerr := lookupLocale(fc.Locale)
	if lerr != nil {
		return Value{}, nil, fmt.Errorf("expr: element %s: formatDate(): %w", elementID, lerr)
	}

	operandVal, caveats, err := Eval(call.Args[0], resolver, fc, elementID)
	if err != nil {
		return Value{}, nil, err
	}

	work := len(operandVal.Str)
	if operandVal.Kind == KindNumber {
		work = operandVal.Num.Exponent
		if work < 0 {
			work = -work
		}
	}
	if err := expressionBudget(resolver).Charge(work); err != nil {
		return Value{}, nil, err
	}
	instantMs, err := instantMsFromValue(operandVal, elementID, call.Raw)
	if err != nil {
		return Value{}, nil, err
	}

	docOffsetMin, err := parseUTCOffsetMinutes(fc.UTCOffset)
	if err != nil {
		return Value{}, nil, fmt.Errorf("expr: element %s: formatDate(): document utcOffset: %w", elementID, err)
	}

	civil, err := civilFromInstantMs(instantMs, docOffsetMin)
	if err != nil {
		return Value{}, nil, fmt.Errorf("expr: element %s: formatDate(): %w: %s", elementID, err, call.Raw)
	}

	// AC9: the Buddhist era (or any other locale's eraOffset) is a
	// calendar conversion applied to the YEAR FIELD, after the offset
	// shift above has already resolved the correct civil date — never
	// a post-process on a rendered string, and never applied to the
	// UTC year before the offset shift (that would be the named
	// mutant: the 2026-12-31T23:30:00Z / +07:00 case must render 2570,
	// not 2569).
	civil.Year += int64(entry.eraOffset)

	patternLit, ok := Ungroup(call.Args[1]).(*StringLit)
	if !ok {
		return Value{}, nil, fmt.Errorf("expr: element %s: formatDate(): pattern argument must be a string literal: %s", elementID, call.Raw)
	}
	if err := expressionBudget(resolver).Charge(len(patternLit.Value)); err != nil {
		return Value{}, nil, err
	}
	tokens, perr := parseDatePattern(patternLit.Value)
	if perr != nil {
		return Value{}, nil, fmt.Errorf("expr: element %s: formatDate(): internal: %w", elementID, perr)
	}

	return Value{Kind: KindString, Str: renderDatePattern(tokens, civil, entry)}, caveats, nil
}
