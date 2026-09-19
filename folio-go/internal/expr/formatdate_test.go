package expr

import (
	"strings"
	"testing"

	"github.com/panitw/folio8/folio-go/internal/template"
)

func fcFor(locale, offset string) FormatContext {
	return FormatContext{Locale: locale, UTCOffset: offset}
}

func mustEvalFormatDate(t *testing.T, src string, r Resolver, fc FormatContext) string {
	t.Helper()
	e, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	if err := Check(e); err != nil {
		t.Fatalf("Check(%q): %v", src, err)
	}
	v, _, err := Eval(e, r, fc, "e1")
	if err != nil {
		t.Fatalf("Eval(%q): unexpected error: %v", src, err)
	}
	if v.Kind != KindString {
		t.Fatalf("Eval(%q): got kind %s, want string", src, v.Kind)
	}
	return v.Str
}

// --- AC7: accepted/rejected operand kinds ---

func TestFormatDateAcceptsRFC3339AndRejectsEverythingElse(t *testing.T) {
	fc := fcFor(template.LocaleEN, "+00:00")

	r := mapResolver{
		"good":   {Kind: KindString, Str: "2026-08-15T00:00:00Z"},
		"badStr": {Kind: KindString, Str: "15 August 2026"},
		"nonInt": {Kind: KindNumber, Num: Decimal{Coefficient: 15, Exponent: -1}}, // 1.5
		"aBool":  {Kind: KindBool, Bool: true},
		"aNull":  {Kind: KindNull},

		// Finding 1 (Blocker): shape-valid-but-impossible RFC 3339
		// captures. rfc3339Pattern's regex validates digit SHAPE only,
		// so each of these matches it; before validateCivilRanges
		// existed, daysFromCivil silently NORMALISED each one into a
		// different, plausible-looking date instead of erroring
		// (measured: 2026-02-31 rendered as 03/03/2026, 2026-13-01 as
		// 01/01/2027, 2026-00-10 as 10/12/2025, and hour 25 rolled into
		// the next day).
		"badFeb31":        {Kind: KindString, Str: "2026-02-31T00:00:00Z"}, // no such day
		"badMonth13":      {Kind: KindString, Str: "2026-13-01T00:00:00Z"}, // no such month
		"badMonth00":      {Kind: KindString, Str: "2026-00-10T00:00:00Z"}, // month 0
		"badDay00":        {Kind: KindString, Str: "2026-01-00T00:00:00Z"}, // day 0
		"badDay99":        {Kind: KindString, Str: "2026-01-99T00:00:00Z"}, // day 99
		"badHour25":       {Kind: KindString, Str: "2026-01-10T25:00:00Z"}, // hour 25
		"badFeb29NonLeap": {Kind: KindString, Str: "1900-02-29T12:00:00Z"}, // 1900 is not a leap year
	}

	if _, err := Parse(`formatDate(good, "d MMMM yyyy")`); err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	got := mustEvalFormatDate(t, `formatDate(good, "d MMMM yyyy")`, r, fc)
	if got != "15 August 2026" {
		t.Fatalf("got %q, want %q", got, "15 August 2026")
	}

	cases := []string{
		`formatDate(badStr, "d MMMM yyyy")`,
		`formatDate(nonInt, "d MMMM yyyy")`,
		`formatDate(aBool, "d MMMM yyyy")`,
		`formatDate(aNull, "d MMMM yyyy")`,
		`formatDate(badFeb31, "d MMMM yyyy")`,
		`formatDate(badMonth13, "d MMMM yyyy")`,
		`formatDate(badMonth00, "d MMMM yyyy")`,
		`formatDate(badDay00, "d MMMM yyyy")`,
		`formatDate(badDay99, "d MMMM yyyy")`,
		`formatDate(badHour25, "d MMMM yyyy")`,
		`formatDate(badFeb29NonLeap, "d MMMM yyyy")`,
	}
	for _, src := range cases {
		e, perr := Parse(src)
		if perr != nil {
			t.Fatalf("Parse(%q): %v", src, perr)
		}
		if cerr := Check(e); cerr != nil {
			t.Fatalf("Check(%q): %v", src, cerr)
		}
		if _, _, err := Eval(e, r, fc, "e1"); err == nil {
			t.Errorf("%s: expected a located error", src)
		}
	}
}

// TestFormatDateImpossibleDateNamesTheOffendingComponentAndValue is
// Finding 1's own requirement (AC7/AD-14): the error must name the
// offending COMPONENT and VALUE, not merely fire. Each case below is
// one of the exact rows the QA review measured silently rendering a
// different, plausible-looking date before validateCivilRanges existed.
func TestFormatDateImpossibleDateNamesTheOffendingComponentAndValue(t *testing.T) {
	fc := fcFor(template.LocaleEN, "+00:00")
	cases := []struct {
		name          string
		when          string
		wantSubstring string
	}{
		{"Feb31", "2026-02-31T00:00:00Z", "day 31 is out of range for 2026-02"},
		{"month13", "2026-13-01T00:00:00Z", "month 13 is out of range 1-12"},
		{"month00", "2026-00-10T00:00:00Z", "month 0 is out of range 1-12"},
		{"hour25", "2026-01-10T25:00:00Z", "hour 25 is out of range 0-23"},
	}
	for _, c := range cases {
		r := mapResolver{"x": {Kind: KindString, Str: c.when}}
		e, perr := Parse(`formatDate(x, "d MMMM yyyy")`)
		if perr != nil {
			t.Fatalf("%s: unexpected parse error: %v", c.name, perr)
		}
		if cerr := Check(e); cerr != nil {
			t.Fatalf("%s: unexpected check error: %v", c.name, cerr)
		}
		_, _, err := Eval(e, r, fc, "e1")
		if err == nil {
			t.Fatalf("%s: expected a located error for %q, got none", c.name, c.when)
		}
		if !strings.Contains(err.Error(), c.wantSubstring) {
			t.Errorf("%s: error %q does not name the offending component/value (want substring %q)", c.name, err.Error(), c.wantSubstring)
		}
	}
}

// TestFormatDateEpochMillisecondEntry is AC7: an integral
// epoch-millisecond number is accepted.
func TestFormatDateEpochMillisecondEntry(t *testing.T) {
	fc := fcFor(template.LocaleEN, "+00:00")
	// 2026-08-15T00:00:00Z in epoch ms.
	r := mapResolver{"ms": {Kind: KindNumber, Num: Decimal{Coefficient: daysFromCivil(2026, 8, 15) * msPerDay, Exponent: 0}}}
	got := mustEvalFormatDate(t, `formatDate(ms, "d MMMM yyyy")`, r, fc)
	if got != "15 August 2026" {
		t.Fatalf("got %q, want %q", got, "15 August 2026")
	}
}

// TestFormatDateEpochMillisecondFloat64RoundTripLossyRedProof is AC7's
// own red-proof: a millisecond count whose float64 round-trip is
// lossy (17+ significant digits) must be accepted or rejected
// IDENTICALLY before and after a hypothetical float64 conversion,
// because decimalExactMilliseconds never performs one. This test
// picks a coefficient that a naive float64(coefficient) conversion
// would already have rounded, and shows the exact big.Int path still
// classifies it correctly as too large to fit an int64's plausible
// range OR (were it in range) exactly right — never silently altered.
func TestFormatDateEpochMillisecondFloat64RoundTripLossyRedProof(t *testing.T) {
	// 19 nines: exceeds maxDecimalCoefficientDigits at the literal
	// stage already, but decimalExactMilliseconds must be exercised
	// directly here since NewDecimal would reject the literal before
	// this function ever saw it — this is the unit itself, not the
	// literal parser.
	lossy := Decimal{Coefficient: 9223372036854775807, Exponent: 0} // math.MaxInt64: exact in int64, NOT exact in float64
	ms, err := decimalExactMilliseconds(lossy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ms != 9223372036854775807 {
		t.Fatalf("got %d, want the exact int64 value unchanged by any float64 pass", ms)
	}
}

// --- AC8: civil calendar correctness ---

func TestFormatDateLeapDay(t *testing.T) {
	fc := fcFor(template.LocaleEN, "+00:00")
	r := mapResolver{"x": {Kind: KindString, Str: "2024-02-29T00:00:00Z"}}
	got := mustEvalFormatDate(t, `formatDate(x, "yyyy-MM-dd")`, r, fc)
	if got != "2024-02-29" {
		t.Fatalf("got %q, want %q", got, "2024-02-29")
	}
}

func TestFormatDateCentennialNonLeapYear(t *testing.T) {
	fc := fcFor(template.LocaleEN, "+00:00")
	// 1900 is divisible by 100 but not 400: NOT a leap year, so
	// 1900-02-28 + 1 day is 1900-03-01.
	r := mapResolver{"x": {Kind: KindString, Str: "1900-02-28T12:00:00Z"}}
	got := mustEvalFormatDate(t, `formatDate(x, "yyyy-MM-dd")`, r, fc)
	if got != "1900-02-28" {
		t.Fatalf("got %q, want %q", got, "1900-02-28")
	}

	r2 := mapResolver{"x": {Kind: KindString, Str: "2100-03-01T00:00:00Z"}}
	got2 := mustEvalFormatDate(t, `formatDate(x, "yyyy-MM-dd")`, r2, fc)
	if got2 != "2100-03-01" {
		t.Fatalf("got %q, want %q", got2, "2100-03-01")
	}
}

// TestFormatDateDateLineCase is AC8's date-line assertion: an instant
// at 2026-12-31T23:30:00Z rendered under utcOffset +07:00 crosses the
// date line into 1 January 2027.
func TestFormatDateDateLineCase(t *testing.T) {
	fc := fcFor(template.LocaleEN, "+07:00")
	r := mapResolver{"x": {Kind: KindString, Str: "2026-12-31T23:30:00Z"}}
	got := mustEvalFormatDate(t, `formatDate(x, "yyyy-MM-dd")`, r, fc)
	if got != "2027-01-01" {
		t.Fatalf("got %q, want %q (the date-line case)", got, "2027-01-01")
	}
}

// TestFormatDateDateLineCaseBackwardCrossing is Finding 17's fix (this
// story's QA review): TestFormatDateDateLineCase above only exercises
// a POSITIVE offset crossing the date line forward; the backward
// crossing — a NEGATIVE offset pushing the civil date backward across
// midnight — depends on floorDivMod's non-negative-remainder
// correction (calendar.go), the piece of the calendar most likely to
// be got wrong, and it had no test. `en` and `th` are both asserted:
// the era conversion must also work correctly on a backward year
// boundary, not merely a forward one.
func TestFormatDateDateLineCaseBackwardCrossing(t *testing.T) {
	r := mapResolver{"x": {Kind: KindString, Str: "2027-01-01T02:00:00Z"}}

	fcEN := fcFor(template.LocaleEN, "-07:00")
	got := mustEvalFormatDate(t, `formatDate(x, "yyyy-MM-dd")`, r, fcEN)
	if got != "2026-12-31" {
		t.Fatalf("got %q, want %q (the backward date-line case)", got, "2026-12-31")
	}

	fcTH := fcFor(template.LocaleTH, "-07:00")
	gotTH := mustEvalFormatDate(t, `formatDate(x, "d MMMM yyyy")`, r, fcTH)
	wantTH := "31 ธันวาคม 2569"
	if gotTH != wantTH {
		t.Fatalf("got %q, want %q (backward date-line crossing, Buddhist era on the far side of the year boundary)", gotTH, wantTH)
	}
}

func TestFormatDateBoundedCivilYear(t *testing.T) {
	fc := fcFor(template.LocaleEN, "+00:00")
	r := mapResolver{"x": {Kind: KindString, Str: "9999-12-31T23:59:59Z"}}
	if _, err := Parse(`formatDate(x, "yyyy-MM-dd")`); err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	// Within bound: succeeds.
	if got := mustEvalFormatDate(t, `formatDate(x, "yyyy-MM-dd")`, r, fc); got != "9999-12-31" {
		t.Fatalf("got %q", got)
	}
	// One day further (10000-01-01) exceeds maxSupportedCivilYear.
	rOver := mapResolver{"x": {Kind: KindNumber, Num: Decimal{Coefficient: (daysFromCivil(9999, 12, 31) + 1) * msPerDay, Exponent: 0}}}
	e, _ := Parse(`formatDate(x, "yyyy-MM-dd")`)
	Check(e)
	if _, _, err := Eval(e, rOver, fc, "e1"); err == nil {
		t.Fatal("expected a located error: civil year out of the supported bound")
	}
}

// --- AC9: Buddhist era ---

func TestFormatDateThaiEraAugustExample(t *testing.T) {
	fc := fcFor(template.LocaleTH, "+07:00")
	r := mapResolver{"x": {Kind: KindString, Str: "2026-08-15T00:00:00+07:00"}}
	got := mustEvalFormatDate(t, `formatDate(x, "d MMMM yyyy")`, r, fc)
	want := "15 สิงหาคม 2569"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestFormatDateThaiEraYearBoundaryCase is AC9's required assertion:
// the era conversion must see the CIVIL year AFTER the offset shift,
// not the raw UTC year.
func TestFormatDateThaiEraYearBoundaryCase(t *testing.T) {
	fc := fcFor(template.LocaleTH, "+07:00")
	r := mapResolver{"x": {Kind: KindString, Str: "2026-12-31T23:30:00Z"}}
	got := mustEvalFormatDate(t, `formatDate(x, "d MMMM yyyy")`, r, fc)
	want := "1 มกราคม 2570"
	if got != want {
		t.Fatalf("got %q, want %q (offset shift crosses the year boundary before era conversion)", got, want)
	}
}

// TestFormatDateThaiEraStringReplaceRedProof is AC9's own red-proof: a
// naive "add 543 to whatever year the UTC instant had" implementation
// produces 2569 for the boundary case, and 2569 (August case) for
// both — the vacuity this test guards against.
func TestFormatDateThaiEraStringReplaceRedProof(t *testing.T) {
	// Simulate the naive implementation directly: apply the era to the
	// UTC year BEFORE the offset shift.
	instantMs, err := instantMsFromValue(Value{Kind: KindString, Str: "2026-12-31T23:30:00Z"}, "e1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// UTC year alone (offset 0):
	utcCivil, err := civilFromInstantMs(instantMs, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	naiveYear := utcCivil.Year + 543 // the named mutant
	if naiveYear != 2569 {
		t.Fatalf("presence precondition: naive mutant should compute 2569, got %d", naiveYear)
	}

	// The REAL implementation, exercised through Eval, must NOT agree
	// with the naive mutant on the boundary case.
	fc := fcFor(template.LocaleTH, "+07:00")
	r := mapResolver{"x": {Kind: KindString, Str: "2026-12-31T23:30:00Z"}}
	got := mustEvalFormatDate(t, `formatDate(x, "yyyy")`, r, fc)
	if got == "2569" {
		t.Fatal("RED-PROOF FAILED: the real implementation agreed with the naive string-replace mutant (2569) instead of the correct 2570")
	}
	if got != "2570" {
		t.Fatalf("got %q, want %q", got, "2570")
	}
}

// --- AC8/AC9: ja golden shape ("d/M/yyyy年月日" family) ---

func TestFormatDateJapaneseGoldenFormattingOnly(t *testing.T) {
	fc := fcFor(template.LocaleJA, "+00:00")
	r := mapResolver{"x": {Kind: KindString, Str: "2026-08-26T00:00:00Z"}}
	got := mustEvalFormatDate(t, `formatDate(x, "yyyy年M月d日")`, r, fc)
	want := "2026年8月26日"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// --- AC10a's own load-time enforcement, exercised through Check ---

func TestFormatDateInvalidPatternIsLoadError(t *testing.T) {
	e, err := Parse(`formatDate(x, "yyyy-QQ-dd")`)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if cerr := Check(e); cerr == nil {
		t.Fatal("expected a Check (load-time) error for the stray run \"QQ\"")
	} else if !strings.Contains(cerr.Error(), "QQ") {
		t.Fatalf("error must name the offending run, got: %v", cerr)
	}
}
