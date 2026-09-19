package expr

// This file is Story 3.4's compiled, embedded locale table (AD-12, R2:
// "the table lives in internal/expr — no new package under
// internal/"). It is keyed by internal/template's exported locale
// constants (AC4, D-3.4.2 amended), never by a second copy of the tag
// literals: internal/template declares the closed SET of legal
// `locale` values (a .folio FORMAT constraint, enforced at load); this
// table declares the per-locale FORMATTING DATA a legal tag names (a
// behavioural constraint, applied at evaluation). AD-12 is not
// amended by this split — its own source-tree map already says
// "expr/ — … locale tables (AD-12)".
//
// `ja`'s row is built like the other three, and is IDENTICAL to
// `zh-Hans` for everything this story implements (D-3.4.1, the owner's
// ruling after the shipped cmap tables were measured directly): the
// two locales share their month-name data BY CONSTRUCTION — one
// declaration, referenced twice — never by two copies that could
// drift (AC4b).

import (
	"fmt"

	"github.com/panitw/folio8/folio-go/internal/template"
)

// LocaleTableVersion is AC6/AD-22: the locale table carries a version,
// and that version is part of the library version. This is a LABEL,
// no mechanism is claimed beyond its declaration (D-000.24) — bumping
// it when a table row's formatting data changes is a human obligation
// this constant exists to make visible, not one this constant enforces
// by itself.
//
// EXPORTED (Finding 9, this story's QA review): as an unexported
// constant this could never actually reach folio8.Version, which is
// what AD-22's "part of the library version" and D-3.4.3's "a breaking
// change for every downstream golden hash" reasoning both require.
// folio-go/version.go's own LocaleTableVersion constant is defined as
// exactly this value, so the two cannot drift.
const LocaleTableVersion = 1

// digitsWestern is DECISION-2 (OWNER ruling, AC11a): every locale in
// this table renders Western/ASCII digits, in formatDate and
// formatNumber alike — "15 สิงหาคม 2569", never Thai numerals. It is
// an EXPLICIT NAMED FIELD on every row below, never an implicit
// default, so a future change (Thai numerals for `th`, say) is a
// visible, versioned table edit rather than a silent code change —
// the operative half of the ruling, per AD-22.
const digitsWestern = "0123456789"

// zhHansMonthNames is the full ("MMMM") month name for the Han
// calendar month-number reading shared by `zh-Hans` and `ja` — one
// declaration, referenced by both rows below (AC4b: "if the two rows
// share data, they share it by construction, never by two copies that
// can drift").
var zhHansMonthNames = [12]string{
	"一月", "二月", "三月", "四月", "五月", "六月",
	"七月", "八月", "九月", "十月", "十一月", "十二月",
}

// localeEntry is one row of the closed locale table: per-tag
// FORMATTING DATA, never per-tag code. Nothing here reads the host's
// locale, time zone or clock (AD-1's "no host locale…" restated by
// this story, R3) — every field is a literal declared here.
type localeEntry struct {
	// groupSep, decimalSep are formatNumber's grouping and decimal-
	// point symbols (AC11).
	groupSep, decimalSep string

	// groupSize is the digit count between group separators (AC11) —
	// 3 for every locale this table carries.
	groupSize int

	// digits is DECISION-2's explicit digit-set field (AC11a).
	digits string

	// eraOffset, when nonzero, is ADDED to the civil (proleptic
	// Gregorian) YEAR before that year is rendered (AC9) — a calendar
	// conversion applied to the year field, never a string edit. `th`
	// is the only locale with a nonzero offset (Buddhist era, +543).
	eraOffset int

	// monthNames is the full ("MMMM" token) month name, January first
	// (AC8's civil calendar is 1-indexed at render time; index 0 here
	// is January).
	monthNames [12]string
}

// localeTable is the closed table itself (AC3-AC6), keyed by
// internal/template's exported constants — each tag string is spelled
// exactly once in the module, on the constant's own declaration
// (internal/template/locale.go).
var localeTable = map[string]localeEntry{
	template.LocaleEN: {
		groupSep: ",", decimalSep: ".", groupSize: 3, digits: digitsWestern,
		monthNames: [12]string{
			"January", "February", "March", "April", "May", "June",
			"July", "August", "September", "October", "November", "December",
		},
	},
	template.LocaleTH: {
		groupSep: ",", decimalSep: ".", groupSize: 3, digits: digitsWestern,
		eraOffset: 543,
		monthNames: [12]string{
			"มกราคม", "กุมภาพันธ์", "มีนาคม", "เมษายน", "พฤษภาคม", "มิถุนายน",
			"กรกฎาคม", "สิงหาคม", "กันยายน", "ตุลาคม", "พฤศจิกายน", "ธันวาคม",
		},
	},
	template.LocaleZhHans: {
		groupSep: ",", decimalSep: ".", groupSize: 3, digits: digitsWestern,
		monthNames: zhHansMonthNames,
	},
	template.LocaleJA: {
		// D-3.4.1: identical to zh-Hans for everything this story
		// implements. The scope fence (AC18/AC21) is about GLYPH
		// SHAPE at render time, which this table has no opinion about
		// at all — it only supplies digits, separators and names.
		groupSep: ",", decimalSep: ".", groupSize: 3, digits: digitsWestern,
		monthNames: zhHansMonthNames,
	},
}

// lookupLocale is the table's own lookup (AC5b): an unknown or empty
// tag — including FormatContext's zero value — is a LOCATED error,
// never a default entry. There is no fallback to "en" anywhere in this
// function or its callers.
func lookupLocale(tag string) (localeEntry, error) {
	e, ok := localeTable[tag]
	if !ok {
		return localeEntry{}, fmt.Errorf(
			"expr: locale %q is not one of the closed set (%v) — no default locale is ever substituted",
			tag, template.LocaleTags,
		)
	}
	return e, nil
}
