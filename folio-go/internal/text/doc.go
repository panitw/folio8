// Package text implements Story 2.1's spike deliverable: a Thai
// break-opportunity engine built against AD-25 ("Thai break
// opportunities fail toward not breaking") and NFR3 / CAP-14 (multi-
// script text support).
//
// Story 2.4 extended it to all three scripts (Opportunities, in
// opportunity.go). It still does not shape text (Story 2.3, GPOS) and
// does not compose or paginate (Stories 2.5, 2.6). It answers one
// question mechanically: "at which rune positions may a line legally
// break?" — for Thai, that requires a dictionary, because Thai is
// written without interword spaces.
//
// # What this package deliberately does NOT do
//
// Three narrowings, stated by name so that nobody has to infer them
// from the code, and stated as CATEGORIES rather than as lists of
// unhandled characters (D-000.23).
//
// NOT UAX #14. Latin (and every script with no rule of its own here)
// breaks after a run of Unicode White_Space, and nowhere else. Absent,
// by name: hyphenation; a break at U+002D HYPHEN-MINUS or any other
// punctuation; the contextual pair rules that make up the bulk of the
// standard. Implementing UAX #14 needs the LineBreak property for every
// code point — either a new module, which folio-go's two-module graph
// forbids (gomod_test.go's wantModuleGraph), or a generated table
// nobody has budgeted. Nothing here may claim conformance to UAX #14.
//
// NO KINSOKU. CJK breaks between adjacent Han or kana characters. The
// line-start / line-end punctuation prohibition — a line may not begin
// with "，" or "。", may not end with an opening bracket — is NOT
// implemented. Fullwidth punctuation and fullwidth digits are therefore
// not break candidates at all, which is a narrowing in the safe
// direction (fewer breaks) but is still a narrowing.
//
// NO WORD SEGMENTATION HEURISTIC FOR THAI PROPER NOUNS, EVER. This one
// is a ruling, not a limitation to be fixed later (D-2.1.6, the
// owner's). Thailand's Surname Act requires every family to coin a
// unique surname, and they are coined out of ordinary dictionary words:
// 83% of the personal names in the committed corpus decompose entirely
// into entries the shipped 62,107-word list already holds, and the
// surname "ศรีสุข" is byte-identical to the two common words "ศรี" and
// "สุข". No dictionary-coverage rule can separate a proper noun from
// its parts, so the engine does not guess. A template DECLARES which
// data paths must never be split, and Opportunities receives those as
// rune spans in its atomic parameter.
//
// THE DISCLOSED COST OF THAT MECHANISM (D-2.1.6, accepted on the
// record, and stated here because this is where a reader meets it): it
// protects DECLARED VALUES only. A Thai name embedded in FREE-FORM
// text — "โอนให้ศรีสุข", "transfer to Srisuk" — is not a bound value,
// carries no declaration, and remains breakable. That is the part this
// mechanism does not reach. It is stated, not fixed.
//
// AD-25's two absolutes, both implemented here and nowhere overridden:
//
//   - No break inside a Thai character cluster — determined mechanically
//     from Unicode combining classes and the Thai block's own structure
//     (see cluster.go). This needs no dictionary and does not depend on
//     shaping or a font (Trap 4).
//   - A run of Thai text the dictionary cannot cover yields no interior
//     break opportunities — it is treated as atomic (see break.go).
//
// The dictionary itself is PyThaiNLP's words_th wordlist (CC0-1.0,
// wordlist/), compiled into a compact BytesTrie (trie.go) and embedded
// into the binary via go:embed (data.go) — never read from disk at
// runtime, so folio-go behaves identically under js/wasm and on a
// server (AC1, AC2, D-000.5, Trap 3).
package text
