package text

import "unicode"

// Span is a half-open range of RUNE indices [Start, End).
//
// Rune indices, not byte offsets, because every position this package
// and its callers exchange is a rune index: AD-25's break positions, the
// corpus's proper-noun spans, and GlyphInfo.Cluster (D-2.3.2, pinned by
// TestClusterValuesAreRuneIndices). Mixing the two units is the defect
// that story warned about, so there is only one unit here.
type Span struct {
	Start, End int
}

// BreakKind names WHERE a break opportunity came from, and it is a
// NAMED KIND rather than a bool on purpose (D-7.1.5): Story 7.3's
// justification rule is two independent conditions — "the last line of
// a paragraph" and "a line ended by a mandatory break" — and a bare
// !hardBreak invites the third case to be re-derived wrongly.
//
// The distinction is the ordinary one between GUESSING and BEING TOLD.
// Everything this package infers from a script, a dictionary or a run
// of whitespace is optional: the packer may decline it. A control
// character the CALLER supplied is not an inference at all, and the
// packer may not decline it.
type BreakKind uint8

const (
	// BreakOptional is a break this package INFERRED — after a run of
	// whitespace, between two Han-or-kana runes, or at a Thai
	// dictionary seam. The packer takes it only if the line needs it,
	// and AD-25's declared-unbreakable spans suppress it.
	//
	// It is the ZERO VALUE deliberately: an opportunity with no kind
	// stated is an inferred one, which is every opportunity this
	// package emitted before Story 7.1.
	BreakOptional BreakKind = iota

	// BreakMandatory is a break the CALLER SUPPLIED, as a line feed in
	// the text handed to Opportunities. It is never inferred. The
	// packer always takes it, however much width remained on the line
	// (FR46), and a declared-unbreakable span does not suppress it:
	// AD-25's third override binds the opportunity set the engine
	// PROPOSES, and a line feed sitting in the input is not the engine
	// proposing anything (D-7.1.1).
	BreakMandatory
)

// Opportunity is one position at which a line may break.
//
// A line that breaks here ends at LineEnd and the next line begins at
// NextStart. The two differ only where the break CONSUMES text — today
// exactly one case, a run of whitespace, which is drawn on neither
// line. For every other break the two are equal.
//
// Modelling the consumed range explicitly is what keeps "the trailing
// space does not count toward the line's width" a property of the break
// rather than a special case at the measuring site.
//
// Kind says whether the break was inferred or supplied. See BreakKind.
type Opportunity struct {
	LineEnd   int
	NextStart int
	Kind      BreakKind
}

// isCJKIdeographOrKana reports whether r is a Han ideograph or a kana
// character, using the standard library's own Unicode script tables.
//
// IT IS A CATEGORY TEST, NOT A CHARACTER LIST, AND THAT IS BINDING
// (D-000.23: a guard written for a defect covers the defect, not its
// class; a denylist is never coverage). unicode.Han, unicode.Hiragana
// and unicode.Katakana are Unicode's own script definitions, shipped
// with the toolchain and updated with it — no table is generated here
// and no module is added, so wantModuleGraph's "exactly two modules"
// still holds.
//
// NOTE, AND IT IS A REAL NARROWING: the engine has no CJK rune
// classification of its own to reuse. Face resolution is COVERAGE-based
// (resolveRuneFace asks each face in the chain whether it has a glyph),
// so it never classifies a rune by script at all. This function is
// therefore new, and it is the only rune-script classification for CJK
// in the codebase.
//
// Fullwidth punctuation and fullwidth digits are deliberately NOT
// included. They are neither Han nor kana, and deciding whether a line
// may end before "，" or begin with it is exactly the line-start /
// line-end prohibition (kinsoku) this engine does not implement — see
// the package doc. Adding them here would be a guess about that
// question wearing a script test's clothes.
func isCJKIdeographOrKana(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r)
}

// Opportunities returns every position at which s may break, in
// ascending LineEnd order, with the positions strictly interior to any
// span in atomic removed.
//
// Three sources, one per script family:
//
//   - LATIN AND EVERYTHING ELSE — a break exists AFTER each maximal run
//     of Unicode White_Space, and the run is consumed. Nothing else in
//     Latin breaks: no hyphenation, no break at "-", no contextual pair
//     rules. THIS IS NOT UAX #14 and must never be described as it —
//     full UAX #14 needs the LineBreak property for every code point,
//     which is either a new module (forbidden: go list -m all stays at
//     exactly two) or a generated table nobody has budgeted. See the
//     package doc.
//
//   - CJK — a break exists between any two adjacent Han-or-kana runes.
//     Kinsoku (line-start / line-end punctuation prohibition) is NOT
//     implemented, as a category.
//
//   - THAI — the dictionary engine, ComputeBreaks in constrained mode,
//     under both of AD-25's absolutes plus Story 2.4's
//     both-sides-coverable filter.
//
// All three INFER a break, so all three yield BreakOptional. There is a
// fourth source, and it infers nothing:
//
//   - A LINE FEED THE CALLER SUPPLIED — one BreakMandatory per U+000A,
//     emitted by the whitespace rule INSTEAD OF its optional break for
//     any run carrying at least one. Mandatory breaks are separators:
//     k of them yield k+1 lines (FR46, D-7.1.2), and they survive the
//     atomic-span filter below (D-7.1.1).
//
// ATOMIC IS A PARAMETER, NOT AN IMPORT (D-000.16, verbatim: "the signal
// rides on the value, never through an import ... the breaking API
// accepts it as a parameter. Stages communicate by what they pass, not
// by what they import"). internal/text does not know that a span came
// from a bound value, from a template declaration, or from a caller's
// own reasoning, and it must not learn.
//
// Removal is by CONSTRUCTION over the span set — every span, whatever
// its script and whatever produced it — never by matching sample
// strings.
func Opportunities(dict *BytesTrie, s string, atomic []Span) []Opportunity {
	runes := []rune(s)
	n := len(runes)
	if n == 0 {
		return nil
	}

	// byLineEnd keys on the position the CURRENT line ends at, so a
	// whitespace-consuming break and a zero-width break proposed at the
	// same place collapse to one, with the consuming form winning
	// (it is the one that does not draw the space).
	byLineEnd := map[int]Opportunity{}
	add := func(o Opportunity) {
		if prev, ok := byLineEnd[o.LineEnd]; ok && prev.NextStart >= o.NextStart {
			return
		}
		byLineEnd[o.LineEnd] = o
	}

	// 1. Whitespace. A run at the very start or very end of s yields no
	// interior opportunity — there would be no text on one side of it.
	//
	// UNLESS THE RUN CARRIES A LINE FEED (Story 7.1). Then it emits one
	// MANDATORY break per line feed INSTEAD OF the single optional one,
	// and the "no text on one side" guard below does not reach them:
	// for a mandatory break the empty side is the entire point, so a
	// leading or trailing line feed produces its empty line and a
	// paragraph gap becomes expressible (D-7.1.2). The guard's own text
	// is unchanged and still governs every inferred whitespace break.
	//
	// The run is consumed at its OUTER EDGES either way (D-7.1.6 /
	// AC5): only the NUMBER of breaks changes. "a \n b" therefore
	// renders as "a" / "b", with neither space drawn — exactly as
	// "a b" would if it broke there.
	for i := 0; i < n; {
		if !unicode.IsSpace(runes[i]) {
			i++
			continue
		}
		j := i
		for j < n && unicode.IsSpace(runes[j]) {
			j++
		}
		// \r\n IS ONE UNIT, BY COUNTING ONLY \n. A carriage return
		// carries no line feed of its own, so a lone \r leaves its run
		// on the optional path unchanged (as settled), and the \r of a
		// \r\n pair is simply more consumed whitespace inside the run
		// rather than a second break.
		feeds := feedsIn(runes, i, j)
		switch {
		case len(feeds) > 0:
			// THE PARTITION (worked in the story's Design Notes for
			// "a \n\n b"): break m ends the previous line at the run's
			// START when m is the first, and at line feed m otherwise;
			// it begins the next line at line feed m+1 when one
			// follows, and at the run's END when m is the last. k
			// breaks therefore yield k+1 lines, with k-1 empty ones
			// between them — mandatory breaks are SEPARATORS.
			for m, f := range feeds {
				lineEnd := f
				if m == 0 {
					lineEnd = i
				}
				nextStart := j
				if m+1 < len(feeds) {
					nextStart = feeds[m+1]
				}
				add(Opportunity{LineEnd: lineEnd, NextStart: nextStart, Kind: BreakMandatory})
			}
		case i > 0 && j < n:
			add(Opportunity{LineEnd: i, NextStart: j})
		}
		i = j
	}

	// 2. CJK, between adjacent Han-or-kana runes.
	for i := 1; i < n; i++ {
		if isCJKIdeographOrKana(runes[i-1]) && isCJKIdeographOrKana(runes[i]) {
			add(Opportunity{LineEnd: i, NextStart: i})
		}
	}

	// 3. Thai, from the dictionary engine. A position adjacent to
	// whitespace is skipped here and left to rule 1: ComputeBreaks
	// reports a script-span boundary at the START of a following space
	// run (isThaiScript excludes U+0020), which would break BEFORE the
	// whitespace and draw it at the head of the next line — the exact
	// polarity AC1 forbids.
	if dict != nil {
		breaks, _ := ComputeBreaks(dict, s, false)
		for _, b := range breaks {
			if b <= 0 || b >= n {
				continue
			}
			if unicode.IsSpace(runes[b-1]) || unicode.IsSpace(runes[b]) {
				continue
			}
			add(Opportunity{LineEnd: b, NextStart: b})
		}
	}

	// Collected by ascending LineEnd directly rather than by ranging
	// byLineEnd: a map range is forbidden under internal/ wherever its
	// order can reach an output byte (D-1.3.5, NFR1.d, enforced by
	// lint's map-range rule), and these opportunities decide where lines
	// break, which is about as far into the output bytes as an ordering
	// can reach. Sorting afterwards would also have been deterministic,
	// but the rule is deliberately syntactic — it does not ask whether
	// this particular range happened to be safe.
	//
	// FROM ZERO, NOT FROM ONE (D-7.1.7, finding 1). A leading line feed
	// proposes {LineEnd: 0, NextStart: ...}, and a loop starting at 1
	// would drop it silently — "\na" would lose its leading empty line
	// while every test about the trailing one passed. No INFERRED rule
	// can propose LineEnd == 0 (rule 1 guards i > 0, rule 2 starts at
	// 1, rule 3 skips b <= 0), so widening the range is corpus-neutral
	// by construction.
	ops := make([]Opportunity, 0, len(byLineEnd))
	for i := 0; i < n; i++ {
		o, ok := byLineEnd[i]
		if !ok {
			continue
		}
		// D-7.1.1, AND THIS IS THE SITE. The exemption is keyed on the
		// opportunity's KIND, here at the filter, never on "is this
		// rune a line feed" and never by shrinking or excluding the
		// span — atomicSpansFor and spansCover are both untouched. A
		// template that declares a value unbreakable stops the engine
		// GUESSING at a break inside it; it does not throw away a break
		// somebody SUPPLIED.
		if o.Kind != BreakMandatory && spansCover(atomic, o) {
			continue
		}
		ops = append(ops, o)
	}
	return ops
}

// feedsIn returns the rune indices of every line feed in runes[i:j], in
// ascending order.
//
// Only U+000A counts. That is what folds \r\n into a single unit
// without a special case: the carriage return is ordinary consumed
// whitespace inside the run, and a lone \r contributes no line feed at
// all, so its run stays on the inferred path.
func feedsIn(runes []rune, i, j int) []int {
	var feeds []int
	for k := i; k < j; k++ {
		if runes[k] == '\n' {
			feeds = append(feeds, k)
		}
	}
	return feeds
}

// spansCover reports whether o falls strictly inside any span in
// atomic. Both ends are tested: a break that ends a line inside a
// declared span would split it just as surely as one that starts the
// next line inside it.
func spansCover(atomic []Span, o Opportunity) bool {
	for _, sp := range atomic {
		if o.LineEnd > sp.Start && o.LineEnd < sp.End {
			return true
		}
		if o.NextStart > sp.Start && o.NextStart < sp.End {
			return true
		}
	}
	return false
}
