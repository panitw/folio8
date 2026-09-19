package expr

import (
	"fmt"
	"strings"
)

// reservedPlaceholders is AD-4's page-number slots (D-1.6.5): "page"
// and "pages" are late-bound tokens the template layer recognises,
// never resolved from data and never an error. Every caller that walks
// "{{ }}" occurrences — internal/bind's render-time resolver and
// folio8's load-time syntax/arity scan alike — checks a placeholder's
// trimmed content against this set, via IsReserved, BEFORE attempting
// to parse it as an expression (AC4: "short-circuited before any parse
// attempt"), so there is exactly ONE definition of what "reserved"
// means, never two that could drift (AD-13).
//
// Unexported (QA Finding 10, Minor): this was previously an exported,
// mutable map — Go maps are reference types, so any importer executing
// expr.ReservedPlaceholders["balance"] = true or
// delete(expr.ReservedPlaceholders, "page") would have changed AD-4's
// fence globally, for both the load-time walk and the render-time
// resolver, at run time, with nothing to catch it. IsReserved is the
// only way in or out now.
var reservedPlaceholders = map[string]bool{
	"page":  true,
	"pages": true,
}

// IsReserved reports whether name — already trimmed of surrounding
// whitespace, exactly as ScanPlaceholders does before comparing — is
// one of AD-4's reserved placeholder tokens.
func IsReserved(name string) bool {
	return reservedPlaceholders[name]
}

// Placeholder is one "{{ ... }}" occurrence located inside a text
// string by ScanPlaceholders.
type Placeholder struct {
	// Inner is the raw content between the delimiters, exactly as
	// authored — NOT yet trimmed (mirroring the pre-3.2 tokenizer this
	// replaces, internal/bind/text.go's own loop).
	Inner string
	// Reserved reports whether Inner (trimmed) is one of
	// ReservedPlaceholders — checked here, once, so every caller gets
	// the same answer.
	Reserved bool
}

// ScanPlaceholders is the module's ONE tokenizer for "{{ ... }}"
// occurrences inside a text string (AD-13): both internal/bind's
// render-time resolver (which evaluates each non-reserved placeholder
// against real data) and folio8's load-time syntax/arity scan (which
// only parses and checks each one, never evaluates) call this, rather
// than each re-implementing the "{{"/"}}" search.
//
// literal is the literal text preceding each returned Placeholder, in
// the same order (literal[i] precedes placeholders[i]); trailing is
// whatever text follows the last placeholder (the whole string, if
// there were none). Concatenating literal[0], placeholders[0]'s
// resolved contribution, literal[1], placeholders[1]'s contribution,
// …, trailing reconstructs the caller's own output — this function
// itself never resolves or evaluates anything.
//
// An unterminated "{{" (no matching "}}" before the string ends) is an
// error (AC19's "unterminated string literal" is a DIFFERENT case,
// inside an expression; this is the same "missing closing }}" defect
// 1.6 already reported).
//
// KNOWN LIMITATION (QA Finding 11, Minor, not fixed by this story): the
// "}}" search below is quote-blind — a string literal argument that
// legitimately contains "}}", e.g. {{ f("a}}b") }}, is cut inside the
// literal and surfaces as an unterminated-placeholder error instead of
// the intended call. Fixing it would mean this scanner tracking quote
// state itself (effectively duplicating part of the expression lexer's
// own job), which the module's real string-literal grammar disallows
// anyway in the common case (lexString, parser.go, has no escapes, so
// a legitimate literal containing "}}" without also containing '"' is
// the only case this could ever matter for, and even that is rare in
// formatDate/formatNumber patterns). Left as a documented edge case
// rather than fixed here.
func ScanPlaceholders(text string) (literal []string, placeholders []Placeholder, trailing string, err error) {
	rest := text
	consumed := 0 // QA Finding 13, Minor: byte offset into the ORIGINAL text, so the error below can carry a position, not just a bare message.
	for {
		start := strings.Index(rest, "{{")
		if start < 0 {
			trailing = rest
			return literal, placeholders, trailing, nil
		}
		literal = append(literal, rest[:start])
		afterOpen := rest[start+2:]
		end := strings.Index(afterOpen, "}}")
		if end < 0 {
			// QA Finding 13, Minor: this used to carry neither an
			// offset nor the offending text, unlike every other
			// diagnostic in the module — AC19's "offending text
			// verbatim" was lost for exactly this case.
			return nil, nil, "", fmt.Errorf("unterminated \"{{\" (missing closing \"}}\") at position %d: %q", consumed+start, afterOpen)
		}
		inner := afterOpen[:end]
		rest = afterOpen[end+2:]
		consumed += start + 2 + end + 2
		trimmed := strings.TrimSpace(inner)
		placeholders = append(placeholders, Placeholder{Inner: inner, Reserved: IsReserved(trimmed)})
	}
}
