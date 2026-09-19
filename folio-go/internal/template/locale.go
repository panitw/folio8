package template

// This file is Story 3.4's AC4 (D-3.4.2, amended): the closed SET of
// valid `locale` tags is a `.folio` FORMAT constraint (folio-format.md
// states it, and the load error is raised in this package, parse.go),
// while the per-locale FORMATTING DATA (symbols, calendar, patterns)
// is internal/expr's behaviour (AD-12: "expr/ — … locale tables"). The
// split is semantic, not merely forced by the stage-rank wall (AC4's
// own note): `template` (rank 2) cannot import `expr` (rank 3), and
// `expr` already imports `template` (decimal.go, SplitJSONNumber), so
// declaring the tags here and consuming them from expr is the only
// direction that compiles at all.
//
// Each tag string is spelled EXACTLY ONCE in the module: as the string
// literal on the right-hand side of one of these four constants.
// closedLocales (below) and internal/expr's locale table are both
// keyed by these constants, never by a second copy of the literal — a
// closed set declared in two places is a set that can drift (AC4's own
// wording), and named constants make that drift a compile error
// instead of a runtime one.
const (
	LocaleEN     = "en"
	LocaleTH     = "th"
	LocaleZhHans = "zh-Hans"
	LocaleJA     = "ja"
)

// LocaleTags is the exported ORDERED tag sequence (AC4a). It is a
// declared literal slice, never produced by ranging a map: under
// D-1.3.5, ranging a map anywhere under internal/ is a BUILD FAILURE,
// not merely a determinism risk, and — independently of that — AC4a
// requires the order itself to be an asserted, exact literal sequence
// rather than "some deterministic order", so that a later switch to
// (say) sorted order cannot silently reorder whatever depends on this
// slice's iteration order today.
var LocaleTags = []string{LocaleEN, LocaleTH, LocaleZhHans, LocaleJA}

// closedLocales is BUILT FROM the constants above (AC4): the map's
// keys are the constant identifiers, not a second set of string
// literals, so this declaration cannot drift from LocaleTags/the
// constants by a spelling mistake — only by a missing or extra entry,
// which AC4b's set-difference test (internal/expr) exists to catch on
// the internal/expr side, and TestClosedLocalesMatchesLocaleTags
// (locale_test.go) catches here.
var closedLocales = map[string]bool{
	LocaleEN: true, LocaleTH: true, LocaleZhHans: true, LocaleJA: true,
}

// IsLocale reports whether s is a member of AD-12's closed locale set.
// Exported for the COMMAND path (component_commands.go's
// setDocumentLocale), which writes Document.Locale from a command and
// must validate it against the same single source the loader does
// rather than against a second literal — the obligation IsStyleAlign
// and IsTableStyleAlign already carry, for the same reason and in the
// same shape. Without it the designer's engine could stamp a document
// with a tag the loader would then refuse to reopen, and the refusal
// would arrive one save later with nothing to attribute it to.
//
// parse.go calls this too, so the load door and the command door are
// ONE implementation, not two callers of two copies.
func IsLocale(s string) bool { return closedLocales[s] }
