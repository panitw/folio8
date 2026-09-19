package pdf

// This file is D-3.7.2's /Info-dictionary date assembly (AC8-AC11, R4,
// R5, R6). internal/pdf never parses or validates a date string, and
// never imports "time" or internal/expr: the caller (package folio8,
// AD-1's shell-adjacent stage that already imports internal/expr for
// Story 3.2's expression checking) resolves and validates a
// documentDate parameter through internal/expr's shipped RFC 3339
// parser and integer calendar BEFORE this value ever exists — R4's
// "stages communicate by what they pass, not by what they import"
// applied literally: the date rides into this package as a plain
// struct of exact integer components, never as a string this package
// would have to reparse, and never as a reason to import internal/expr
// here at all.

// DocumentDate is a validated civil instant, ready to be assembled into
// the PDF date-string syntax ("D:YYYYMMDDHHmmSSOHH'mm'"). Every field
// is an exact integer;
// OffsetMinutes is signed (minutes east of UTC — 0 for both "Z" and
// "+00:00", which are indistinguishable once parsed and PDF's syntax
// has no separate "Z" spelling worth preserving here).
type DocumentDate struct {
	Year                 int64
	Month, Day           int
	Hour, Minute, Second int
	OffsetMinutes        int
}

// appendPDFDate appends d as a complete PDF string literal —
// "(D:YYYYMMDDHHmmSSOHH'mm')", parentheses included — to dst and
// returns the extended slice. Every field is zero-padded through
// appendIntPadded (AD-3, R6): this file assembles no number through
// fmt or strconv, so it does not need to be numbers.go itself to stay
// outside TestNumberFormattingIsConfinedToNumbersGo's forbidden set.
//
// QA Nit 4 (this story's review): the trailing apostrophe after "mm"
// is the PDF Reference 1.6-and-earlier form; ISO 32000-1 §7.9.4 itself
// drops it ("D:YYYYMMDDHHmmSSOHH'mm", no closing quote on the minutes
// field). This function deliberately keeps the trailing apostrophe —
// it is universally accepted by PDF readers, and D-3.7.2's own R6
// idiom and every fixture already assume it — this is a citation
// correction only, no behaviour change.
func appendPDFDate(dst []byte, d DocumentDate) []byte {
	dst = append(dst, "(D:"...)
	dst = appendIntPadded(dst, d.Year, 4)
	dst = appendIntPadded(dst, int64(d.Month), 2)
	dst = appendIntPadded(dst, int64(d.Day), 2)
	dst = appendIntPadded(dst, int64(d.Hour), 2)
	dst = appendIntPadded(dst, int64(d.Minute), 2)
	dst = appendIntPadded(dst, int64(d.Second), 2)

	off := d.OffsetMinutes
	sign := byte('+')
	if off < 0 {
		sign = '-'
		off = -off
	}
	dst = append(dst, sign)
	dst = appendIntPadded(dst, int64(off/60), 2)
	dst = append(dst, '\'')
	dst = appendIntPadded(dst, int64(off%60), 2)
	dst = append(dst, '\'', ')')
	return dst
}
