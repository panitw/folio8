package expr

// FormatContext is Story 3.4's AC1/R1: the small, immutable value that
// carries the document's declared locale and fixed UTC offset to Eval.
// It is a value, never a pointer to something mutable — there is
// nothing to mutate — and it reaches Eval as an explicit parameter,
// never through expr.Resolver (whose method set is closed, R1) and
// never through package-level state (AD-1).
//
// The ZERO VALUE is deliberately unusable (AC5b): Locale == "" names
// no row of the locale table (locale.go), so any attempt to format
// through a zero-value FormatContext fails with a located error rather
// than silently behaving as "en". There is no default-to-English
// fallback anywhere in this package.
type FormatContext struct {
	// Locale is the document's declared `locale` tag (AD-12), one of
	// the closed set internal/template exports (LocaleTags) — but this
	// type does not itself enforce membership; lookupLocale (locale.go)
	// is what turns an unlisted tag into a located error at the point
	// formatDate/formatNumber actually need the table (AC5b).
	Locale string

	// UTCOffset is the document's required `utcOffset` field, ±HH:MM
	// (parse.go:92-103, already enforced at load — F6/AC5a). It is
	// carried here, rather than re-read from a *Template, because
	// FormatContext is the one value threaded all the way to Eval
	// (R1); nothing under internal/expr ever holds a *Template.
	UTCOffset string
}

// NewFormatContext constructs a FormatContext from a document's own
// declared fields (render.go's render entry point, R1). It performs no
// validation itself — an unlisted locale tag or a malformed offset is
// reported, located, at the point formatDate/formatNumber actually
// look either up (AC5b) — so this constructor can never itself become
// a second place those checks could drift from the formatter's own.
func NewFormatContext(locale, utcOffset string) FormatContext {
	return FormatContext{Locale: locale, UTCOffset: utcOffset}
}
