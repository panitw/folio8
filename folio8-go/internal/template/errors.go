package template

import (
	"fmt"

	"github.com/panitw/folio8/folio8-go/internal/diag"
)

// LoadError is every load-time failure this package produces (AC41,
// D-1.4.2): Field, ElementID (where applicable) and Value are always
// populated so the message can name what a person needs to fix.
//
// CODE, and how it stopped being an exception (Story 7.8, D-7.8.1).
// Until Story 7.8 the overwhelming majority of these carried NO code:
// internal/diag did not exist before Story 3.6 (D-1.4.2's sequencing
// ruling), and R7's criterion (D-3.6.6) did not license minting a code
// for every one of them once it did. Four sites were coded by hand —
// three footer-source conditions (D-1.4.2's TABLE_FOOTER_SOURCE_FORBIDDEN
// parenthetical, plus D-1.4.1's TABLE_FOOTER_SOURCE_UNRESOLVED
// out-of-collection arm swept in at Story 4.5 under D-000.67 part 2) and
// Story 7.2's line-spacing code, which D-7.8.2 retired before the v1.0.0
// tag (that site now takes the general code).
//
// That left every OTHER load error uncoded, which folio8.ParseTemplate's
// boundary (this package may never import the module root, AD-1) read
// as "no stable code — mint TEMPLATE_MALFORMED instead". The WASM host
// replaces a TEMPLATE_MALFORMED message wholesale, so a located load
// error reached the author as "The template could not be processed" and
// named neither the element nor the field. Story 7.2 minted a code to
// escape that for ONE field; Story 7.8 ruled that the escape belongs to
// the whole category, and put diag.CodeTemplateFieldInvalid in
// newLoadError itself. Code is therefore NEVER "" on an error this
// package constructs: newLoadError supplies the general code, and
// newLoadErrorCoded overrides it for the four conditions a consumer can
// genuinely branch on.
type LoadError struct {
	Field     string
	ElementID string // empty when the error is not scoped to one element/column
	Value     string
	Reason    string
	Code      diag.Code // never "": the general code, or an overriding specific one
}

// THE MESSAGE IS BOUNDED WHERE IT IS RENDERED, NOT WHERE IT IS BUILT
// (Story 7.8, D-7.8.5). D-7.8.1 moved this whole population off
// TEMPLATE_MALFORMED so a located refusal could reach its author in
// words — and TEMPLATE_MALFORMED's message is destroyed at the WASM
// host precisely because "that message quotes the offending document
// back, so a large or hostile one would be reflected instead of
// described". The ruling's stated ground for the move was that a
// LoadError's message is not one of those. MEASURED at 7c892f1 and
// again here, THAT WAS FALSE: nine of this package's newLoadError call
// sites pass string(raw) — an arbitrary JSON sub-object — as Value, and
// a well-formed document whose `style` key holds 2048 characters went
// from a 35-character refusal to 512 bytes of the author's own file.
//
// The fix makes the premise TRUE rather than working around its
// falsity: the premise was a claim about THE MESSAGE, so the bound
// belongs to the message. The struct fields stay COMPLETE — a Go
// integrator's CI log legitimately wants the whole offending JSON, and
// truncating the datum to fix a presentation problem would be
// over-broad in exactly the way the boundary rule was. One method, all
// call sites, no per-site judgement.
//
// RUNES, NEVER BYTES, AND NEVER A SPLIT RUNE. A byte bound hands a Thai
// or CJK author a third of the budget an English one gets — the
// script-dependence defect ruled on at Story 7.4 — and can cut a rune
// in half. (The host's own bounded() is a byte cut and does exactly
// that today; it is the last resort, not this bound.)
//
// THE ELISION IS VISIBLE. A truncated fragment that looks whole is a
// new lie in a message this story exists to make honest, so an elided
// fragment ends in "…".
//
// THE CRITERION, AND THE TWO LAYERS THAT MEET IT: the message must stay
// dominated by the engine's own words — the sentence frame, the field,
// the element and the reason — inside the wasm host's
// bounded(message, 512) window (wasm/cmd/engine/main.go).
//
// That criterion is a property of the ASSEMBLED MESSAGE, so it cannot be
// discharged by per-fragment bounds alone, and for a while it was not.
// Four bounds with no shared budget admit 84 + 24 + 96 + 256 = 460 runes
// of author content plus the frame, and one runaway datum can spend
// several of those budgets at once. MEASURED: a document whose element
// id is 2048 Thai runes — claimID passes the raw id as ElementID, again
// as Value, and again inside Reason via validateElementID's %q —
// assembled a message of 1142 bytes (436 runes). The host's bounded() is
// a raw BYTE slice value[:512], so that message reached the author cut
// mid-rune: invalid UTF-8, ending "\x81กกกก\xe0", with no elision
// marker. Both of those are things this comment's own rules forbid.
//
// So there are two layers, and they do different jobs. The FOUR RUNE
// BOUNDS below allocate the budget fairly across scripts: a Thai author
// gets the same 84 runes of value as an English one, which is the
// script-independence ruling from Story 7.4. The MESSAGE-LEVEL BYTE
// BOUND (loadErrorMessageBytes, applied last in Error()) is what
// actually GUARANTEES the criterion: the returned sentence is at most
// the host's own window in bytes, cut only on a rune boundary and always
// ending in the visible marker. The same 2048-rune id now yields 511
// bytes of valid UTF-8 ending in "…" — 511 and not 512 because the cut
// lands on the last WHOLE rune that fits, never spending a byte it
// cannot spend on a whole rune — and the host's byte cut, the last
// resort and never this package's bound, can no longer fire.
//
// FOUR, NOT ONE. D-7.8.5 named Value as "the author-supplied component
// identified so far" and required any other found to get the same
// treatment. Three more were found and measured (see the story record):
// the ELEMENT ID is the raw string from the document at claimID's two
// sites, so an invalid 4096-character id was reflected three times over
// in one 12,424-byte message; the FIELD splices an author-supplied
// object key at `assets.<k>` and `fonts.<k>`, measured at 4,263 bytes;
// and the REASON interpolates the author's id (ids.go's %q arms) and a
// table's own collection path (parse_bands.go's footerOf prefix check).
const (
	// WHAT A RUNE COSTS, AND WHY THESE BOUNDS ARE STILL THE RIGHT
	// SHAPE. An earlier note here claimed "a rune costs at most 3 bytes
	// for every script the format admits — AD-12's closed locale set
	// (en, th, zh-Hans, ja) is entirely within the BMP — so a bound of
	// N runes costs at most 3N bytes". That is FALSE for the data that
	// actually flows through these fragments. The locale set constrains
	// the document's LANGUAGE, not its bytes: Value, Field, ElementID
	// and Reason all carry arbitrary author-supplied JSON text, and one
	// emoji or CJK Extension B character is utf8.UTFMax = 4 bytes. No
	// count of runes bounds bytes here.
	//
	// The four bounds are therefore NOT a byte guarantee and are not
	// asked to be one. Each is derived from what its fragment can
	// legitimately hold, so that no engine-authored text is ever
	// truncated and each script gets the same allowance — that is the
	// fair-allocation job. The byte guarantee is loadErrorMessageBytes,
	// applied to the assembled sentence at the end of Error().

	// loadErrorValueRunes: the value is wholly the author's and is the
	// fragment that regressed, so it takes the largest single share —
	// nominally half the host's 512-byte window, 84 runes being what
	// 256 bytes buys at the 3 bytes a BMP rune costs, minus the elision
	// marker: 3N + 3 <= 256 gives N <= 84.33, so 84. Read that as the
	// SHARE it is, not a ceiling: 84 runes of 4-byte JSON is 336 bytes,
	// and the message bound is what keeps the total honest.
	loadErrorValueRunes = 84

	// loadErrorElementIDRunes: an id is short BY CONTRACT (AD-10) — the
	// longest LEGAL one is 14 runes, "e" plus the 13 base-36 digits the
	// int64 counter ceiling allows — so a long one is by definition not
	// an id and holds nothing to read. 24 is comfortably above every
	// legal id and reflects nothing beyond it.
	loadErrorElementIDRunes = 24

	// loadErrorFieldRunes: a field path is engine-shaped with an author
	// key spliced in, so the bound is the longest path the format can
	// legitimately produce — `assets.` + a 64-character digest +
	// `.mediaType` = 81 runes — rounded up to 96. No engine-authored
	// path is ever truncated; an author's runaway key is.
	loadErrorFieldRunes = 96

	// loadErrorReasonRunes: a reason is engine prose that at a few sites
	// quotes the author back, so the bound is the longest reason this
	// loader can author — the asset-digest mismatch, exactly 200 runes
	// because it names two 64-character digests — rounded up to 256 for
	// headroom over the stdlib text the `must be a …: ` arms append. No
	// engine-authored reason is ever truncated.
	loadErrorReasonRunes = 256

	// loadErrorMessageBytes is the bound that actually discharges the
	// criterion, and it is NOT a number of this package's choosing: it
	// is the host's own window, bounded(message, 512) in
	// wasm/cmd/engine/main.go. Matching it exactly is the point — one
	// byte more and the host's raw slice fires and splits a rune; fewer
	// would throw away words the author is entitled to read.
	loadErrorMessageBytes = 512

	// loadErrorElision is the visible marker every truncation leaves,
	// at both layers. It is three bytes, which the message bound
	// subtracts from its budget so the result INCLUDING the marker
	// still fits the window.
	loadErrorElision = "…"
)

// boundRunes returns s unchanged when it is at most max runes, and
// otherwise its first max runes followed by the elision marker. Ranging
// a string yields the byte index of each rune's FIRST byte, so the cut
// can never land inside one; invalid UTF-8 is yielded a byte at a time
// and is equally safe to cut at.
func boundRunes(s string, max int) string {
	runes := 0
	for i := range s {
		if runes == max {
			return s[:i] + loadErrorElision
		}
		runes++
	}
	return s
}

// boundBytes returns s unchanged when it is at most max BYTES, and
// otherwise its longest whole-rune prefix that leaves room for the
// elision marker, followed by that marker — so the returned string is
// never longer than max bytes, marker included.
//
// This is the layer that discharges the criterion, and it is a BYTE
// bound on purpose: the thing being satisfied is the host's byte window,
// and only a byte bound can promise the host's own slice will not fire.
// Cutting on a rune boundary is what keeps it from repeating the host's
// defect. Ranging a string yields the byte index of each rune's FIRST
// byte, so the largest such index at or below the budget is a legal cut
// for any rune width up to utf8.UTFMax; invalid UTF-8 is yielded a byte
// at a time and is equally safe to cut at. The per-fragment rune bounds
// still run first and still do the fair-allocation work — this one only
// ever fires when several fragments run long at once.
func boundBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	budget := max - len(loadErrorElision)
	if budget < 0 {
		budget = 0
	}
	cut := 0
	for i := range s {
		if i > budget {
			break
		}
		cut = i
	}
	return s[:cut] + loadErrorElision
}

func (e *LoadError) Error() string {
	field := boundRunes(e.Field, loadErrorFieldRunes)
	reason := boundRunes(e.Reason, loadErrorReasonRunes)
	value := boundRunes(e.Value, loadErrorValueRunes)
	if e.ElementID != "" {
		return boundBytes(fmt.Sprintf("template: field %s (element %s): %s (value: %s)", field, boundRunes(e.ElementID, loadErrorElementIDRunes), reason, value), loadErrorMessageBytes)
	}
	return boundBytes(fmt.Sprintf("template: field %s: %s (value: %s)", field, reason, value), loadErrorMessageBytes)
}

// newLoadError is the single constructor every GENERAL load-error site
// in this package calls (AC41's enumeration test walks call sites of
// this function).
//
// Story 7.8, D-7.8.1: it SUPPLIES diag.CodeTemplateFieldInvalid itself.
// Until this story it left Code at "", which folio8.ParseTemplate's
// boundary read as TEMPLATE_MALFORMED — whose message the WASM host
// replaces wholesale with "The template could not be processed". Every
// located load error in the format was therefore destroyed before its
// author could read it: the field, the element and the value all
// travelled inside a message nothing was allowed to show. Putting the
// code in the CONSTRUCTOR rather than at the call sites is what makes
// that structural rather than enumerative — there is no per-site
// judgement to get wrong, and no new site can be added uncoded.
//
// The condition it names is one condition, not one per field: "a
// well-formed template carries a field value that is not acceptable".
// A consumer discriminates on Field, which grows freely; the registry
// does not. See internal/diag/diag.go's CodeTemplateFieldInvalid for
// the rule in full.
func newLoadError(field, elementID, value, reason string) error {
	return &LoadError{Field: field, ElementID: elementID, Value: value, Reason: reason, Code: diag.CodeTemplateFieldInvalid}
}

// newLoadErrorCoded is Story 3.6's second constructor (R8/DW-6): the
// OVERRIDE for a condition that carries a code of its own rather than
// the general one newLoadError supplies. Called at parse_bands.go's
// three footer-source sites — the two TABLE_FOOTER_SOURCE_FORBIDDEN
// ones (one code, two sites, because the code names the CONDITION, not
// the line) and the single TABLE_FOOTER_SOURCE_UNRESOLVED one (the
// out-of-collection footerOf prefix check, swept in at Story 4.5 per
// D-000.67 part 2). Story 7.2's line-spacing site was a fourth until
// D-7.8.2 retired its code before the v1.0.0 tag.
//
// D-7.8.1 fixes when a new one is warranted: only when a NAMED CONSUMER
// must BRANCH on the code to behave differently. Everything else takes
// the general code and discriminates on Field.
func newLoadErrorCoded(field, elementID, value, reason string, code diag.Code) error {
	return &LoadError{Field: field, ElementID: elementID, Value: value, Reason: reason, Code: code}
}
