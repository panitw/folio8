package expr

import "fmt"

// This file is AC10/AC10a (FINDING-4, ruled): the date-pattern grammar
// is CLOSED and validated at Check (load time), not at Eval — a bad
// pattern is a load error, exactly like checkNumberLit's precedent
// (check.go). No quoting mechanism ships: a literal is any character
// that is NOT an ASCII letter, so `年`, `月`, `日`, `/`, `-`, space and
// punctuation all pass through verbatim with no machinery at all, and
// a stray ASCII letter that is not part of a recognised field token is
// a located LOAD error naming the pattern and the offending run.

// dateFieldTokenList is the closed set of recognised date-pattern field
// tokens, declared ONCE as a FIXED-SIZE array (Finding 4's engineering-
// lead ruling, this story's QA review): a 7th element added to this
// literal without also widening the array size is a COMPILE ERROR, the
// same external-bound shape functionTable's [8]funcEntry gives
// AC16 — never a bare `len() == 6`, which is a lossy count, not a set.
// datepattern_test.go pins its OWN independent literal copy of this
// exact set and asserts set equality against dateFieldTokens (built
// from this array below) in both directions.
var dateFieldTokenList = [6]string{"yyyy", "MMMM", "MM", "M", "dd", "d"}

// dateFieldTokens is dateFieldTokenList as a set, built once at package
// init — the form parseDatePattern actually consults.
var dateFieldTokens = func() map[string]bool {
	m := make(map[string]bool, len(dateFieldTokenList))
	for _, tok := range dateFieldTokenList {
		m[tok] = true
	}
	return m
}()

// dateToken is one element of a parsed date pattern: either a
// recognised field (Field == true, Text one of dateFieldTokens' keys)
// or a literal rune carried through verbatim.
type dateToken struct {
	Field bool
	Text  string
}

// isASCIILetter reports whether r is an ASCII letter — deliberately
// NOT unicode.IsLetter: 年/月/日/สิงหาคม are all Unicode letters, and
// AC10a requires them to pass through as literals, not to make a
// pattern containing them an error. Only an ASCII letter can ever be a
// field token or a load error; nothing else is scanned as one.
func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// parseDatePattern validates and tokenizes a date pattern (AC10a). It
// is the ONE implementation both Check (via checkDatePatternLiteral)
// and formatDate's own renderer (calendar.go) call — never two
// separate grammars that could drift.
func parseDatePattern(pattern string) ([]dateToken, error) {
	runes := []rune(pattern)
	var toks []dateToken
	i := 0
	for i < len(runes) {
		r := runes[i]
		if isASCIILetter(r) {
			j := i + 1
			for j < len(runes) && runes[j] == r {
				j++
			}
			run := string(runes[i:j])
			if !dateFieldTokens[run] {
				return nil, fmt.Errorf(
					"expr: date pattern %q: %q is not a recognised field token (closed set, no quoting mechanism — FINDING-4)",
					pattern, run,
				)
			}
			toks = append(toks, dateToken{Field: true, Text: run})
			i = j
			continue
		}
		toks = append(toks, dateToken{Field: false, Text: string(r)})
		i++
	}
	return toks, nil
}
