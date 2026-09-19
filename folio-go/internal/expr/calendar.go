package expr

import "fmt"

// This file is AC8/AC9: converting an instant to civil date fields
// (proleptic Gregorian), and back, using ONLY integer arithmetic —
// "import time" appears nowhere in this package (AC2's guard covers
// it). daysFromCivil/civilFromDays are Howard Hinnant's well-known
// integer day-count algorithms
// (howardhinnant.github.io/date_algorithms.html): pure int64
// arithmetic, correct for the proleptic Gregorian calendar including
// the 100-not-400 leap-year exception, and defined for years before
// 1970 (negative day counts) as well as after.

// minSupportedCivilYear/maxSupportedCivilYear are AC8's "bounded
// domain": a civil year outside this range is a located error, never
// a wrapped or negative year. The range is generous for report data
// (bank statements, invoices) while still being a NAMED, checked
// bound rather than "whatever int64 happens to allow".
const (
	minSupportedCivilYear = 1
	maxSupportedCivilYear = 9999
)

const msPerDay = 24 * 60 * 60 * 1000

// daysFromCivil returns the number of days since 1970-01-01 (may be
// negative) for the proleptic Gregorian civil date (y, m, d). m is
// 1..12; d is 1..the last day of that month.
func daysFromCivil(y int64, m, d int) int64 {
	if m <= 2 {
		y--
	}
	era := y
	if era < 0 {
		era -= 399
	}
	era /= 400
	yoe := y - era*400 // [0, 399]
	var mp int64
	if m > 2 {
		mp = int64(m) - 3
	} else {
		mp = int64(m) + 9
	}
	doy := (153*mp+2)/5 + int64(d) - 1     // [0, 365]
	doe := yoe*365 + yoe/4 - yoe/100 + doy // [0, 146096]
	return era*146097 + doe - 719468
}

// civilFromDays is daysFromCivil's inverse.
func civilFromDays(z int64) (y int64, m, d int) {
	z += 719468
	var era int64
	if z >= 0 {
		era = z / 146097
	} else {
		era = (z - 146096) / 146097
	}
	doe := z - era*146097                                  // [0, 146096]
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365 // [0, 399]
	y = yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100) // [0, 365]
	mp := (5*doy + 2) / 153                  // [0, 11]
	d = int(doy - (153*mp+2)/5 + 1)          // [1, 31]
	if mp < 10 {
		m = int(mp + 3)
	} else {
		m = int(mp - 9)
	}
	if m <= 2 {
		y++
	}
	return y, m, d
}

// floorDivMod is floor division/modulo (as opposed to Go's native
// truncating "/"/"%"), needed because a local millisecond-of-epoch
// value can be negative (an instant before 1970, or a large negative
// UTC offset) and civil date arithmetic requires the REMAINDER to
// stay non-negative — a truncating division would put midnight on the
// wrong day.
func floorDivMod(a, b int64) (q, r int64) {
	q = a / b
	r = a % b
	if r != 0 && (r < 0) != (b < 0) {
		q--
		r += b
	}
	return q, r
}

// civilInstant is one fully-resolved civil date/time — the output of
// AC8's step (2), before AC9's era conversion (which touches only
// Year) and before the pattern renders it (step 3).
//
// Hour, Minute and Second are computed on every call (below) but no
// published date-pattern token renders them today (renderDatePattern,
// formatdate.go, handles only yyyy/MMMM/MM/M/dd/d) — RESERVED for a
// future hour/minute/second token, not dead weight: dropping them
// would mean re-deriving the same floorDivMod split a second time the
// day a time-of-day token is added (Finding 20.4, this story's QA
// review).
type civilInstant struct {
	Year                 int64
	Month, Day           int
	Hour, Minute, Second int
}

// civilFromInstantMs converts instantMs (a UTC instant, milliseconds
// since the epoch) into civil fields under offsetMinutes (AC8's step
// (2)): instantMs + offsetMinutes*60000 is the LOCAL millisecond
// count, which floorDivMod splits into a day count and a
// millisecond-of-day, converted to Y/M/D and H/Mi/S by civilFromDays.
//
// The bound (AC8's "bounded domain") is checked on the resulting civil
// YEAR — the CE year, before any era offset (AC9) — never on the raw
// instant or on the wrapped/negative representation a silent overflow
// would produce.
func civilFromInstantMs(instantMs int64, offsetMinutes int) (civilInstant, error) {
	localMs := instantMs + int64(offsetMinutes)*60000
	days, msOfDay := floorDivMod(localMs, msPerDay)
	y, m, d := civilFromDays(days)
	if y < minSupportedCivilYear || y > maxSupportedCivilYear {
		return civilInstant{}, fmt.Errorf(
			"expr: formatDate: civil year %d is outside the supported range [%d, %d]",
			y, minSupportedCivilYear, maxSupportedCivilYear,
		)
	}
	h := int(msOfDay / 3600000)
	mi := int((msOfDay / 60000) % 60)
	s := int((msOfDay / 1000) % 60)
	return civilInstant{Year: y, Month: m, Day: d, Hour: h, Minute: mi, Second: s}, nil
}
