// This file declares AD-14's diagnostic payload and D-2.8.3's result
// shape — the seam Story 2.8 was blocked on until the owner ruled OD-1.
//
// AD-14 already fixed the VALUE (one Diagnostic carrying Severity, a
// stable string code from a closed registry, an optional element id, an
// optional data path, and a message) and the DISPOSITION (a Warning
// accompanies a successful render, never silent, never fatal). Only the
// SEAM that carries it out of Render was open, and D-2.8.3 rules it: a
// result struct, not a third return value, not a sink parameter, not a
// sibling entry point. See the decision log's D-2.8.3 through D-2.8.6
// for the grounds — they are not re-argued here.
package folio8

import "github.com/panitw/folio8/folio-go/internal/diag"

// Severity classifies a Diagnostic (AD-14, verbatim): "Error aborts the
// render, Warning accompanies a successful one." A Diagnostic with
// SeverityError is never returned from Render/RenderTo alongside bytes
// — an Error is reported as Go's ordinary error return instead, and
// today's render path (this story) produces only SeverityWarning
// values. The type exists now because AD-14 defines both severities as
// one closed set, and a caller matching on severity must have both
// cases to match against, even before the engine emits the second one.
type Severity int

const (
	// severityUnset is the zero value of Severity, and it is NOT a
	// valid severity (DW-18, R10/AC6, Story 3.6). Before this story,
	// SeverityWarning WAS the zero value, so a Diagnostic{} literal
	// that omitted its Severity field was silently indistinguishable
	// from a genuine Warning — harmless while SeverityWarning was the
	// only severity ever constructed, but the moment Story 3.6 mints
	// the first SeverityError values (AC8), that same omission would
	// silently downgrade an Error to a Warning: AD-14's disposition
	// rule violated with nothing able to catch it. This constant closes
	// the window by construction: a Diagnostic built without an
	// explicit Severity now carries a value String() reports honestly
	// as unset, never one a caller could mistake for Warning.
	//
	// Unexported: it exists to make the zero value visibly wrong, not
	// to be constructed on purpose by any caller, in or out of this
	// module.
	//
	// Renumbering DW-18's own recorded "how we'd know it was forgotten"
	// failure mode is safe ONLY because it happens before this story
	// ever constructs a SeverityError value (Task 8 precedes Task 10,
	// by construction, not diligence) and because nothing downstream
	// could have pinned the previous integer values: at the time
	// folio-go/version.go declared a development version and `git tag`
	// named no folio-go/v* tag (AD-22). Since folio-go/v1.0.0,
	// renumbering a public constant here is a breaking change
	// requiring folio-go/v2.
	severityUnset Severity = iota

	// SeverityWarning accompanies a successful render: the PDF bytes
	// are complete and correct for what they contain, but something the
	// author asked for could not be honoured exactly as declared (FR44:
	// content clipped at its box boundary; FR41's fifth mode, a missing
	// glyph, Story 3.6).
	SeverityWarning

	// SeverityError aborts the render (AD-14): a Diagnostic carrying
	// this severity is never returned from Render/RenderTo alongside
	// bytes — it travels as Go's ordinary error return instead, wrapped
	// by Story 3.6's RenderError (D-3.6.3, AC8). Every production
	// construction site sets Severity explicitly; there is no site in
	// this module that relies on the zero value meaning this.
	SeverityError
)

// String reports s as the closed-registry word AD-14 names it by
// ("Warning" or "Error"), never a bare integer — a Diagnostic's
// Severity is presented to a human, and an unlabelled 0/1 would defeat
// that purpose the first time someone printed one.
func (s Severity) String() string {
	switch s {
	case severityUnset:
		return "Severity(unset)"
	case SeverityWarning:
		return "Warning"
	case SeverityError:
		return "Error"
	default:
		return "Severity(" + itoa(int(s)) + ")"
	}
}

// itoa is a tiny, dependency-free int->string helper (avoids pulling
// strconv into this file for one Severity.String() fallback branch that
// only fires for a value never constructed inside this module).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// DiagCodeTextClippedWidth is the closed registry's one code Story 2.8
// minted (AD-14: "a stable string code from a closed registry"; D-2.8.1:
// FR44's only subject is the horizontal axis). It names a text
// element's widest packed line exceeding its declared width, clipped at
// the box boundary rather than reflowed or dropped (AC1/AC2/AC6).
//
// Story 3.6, AC3/D-3.6.4: bridged to internal/diag.CodeTextClippedWidth
// — the exported NAME and its exact STRING both stay byte-identical
// (asserted as a literal, not merely that this line compiles: a bridge
// that alters one byte of the string is AD-14's breaking change wearing
// a refactor). internal/diag now holds the registry; this constant is
// the public API's continued spelling of the same code.
//
// Additive only (AD-14, verbatim: "changing a code's meaning is a
// breaking change"): once shipped, this string's meaning is permanent.
const DiagCodeTextClippedWidth = string(diag.CodeTextClippedWidth)

// DiagCodeEmptyAverage is the closed registry's second code (Story
// 3.3, DECISION-5/R9): avg() evaluated over a present-but-empty
// collection. D-3.1a.2's kernel has no identity element for an average
// of zero operands and refuses honestly; Story 4.2's own AC requires
// an empty-collection table to render successfully, so the render
// proceeds — the aggregate resolves to empty — and this Warning is how
// a reader learns why a total column shows nothing, rather than the
// render simply failing (which would make Story 4.2's AC
// unsatisfiable) or the blank column looking unremarkable (which it is
// not: an ALL-NULL collection is a real average and renders a number,
// R7.3 — only a genuinely EMPTY collection reaches this code).
//
// Minted here, in the module-root folio8 package, following
// DiagCodeTextClippedWidth's own precedent (D-2.8.1): D-1.4.2 forbids
// minting a code AHEAD of the condition it names, never minting one in
// this package before internal/diag exists (Story 3.6) — the condition
// ships in this story, which is exactly when 2.8 minted.
//
// Story 3.6, AC3/D-3.6.4: bridged to internal/diag.CodeEmptyAverage —
// name and exact string both byte-identical, asserted as a literal.
//
// Additive only (AD-14, verbatim: "changing a code's meaning is a
// breaking change"): once shipped, this string's meaning is permanent.
const DiagCodeEmptyAverage = string(diag.CodeEmptyAverage)

// DiagCodeTextMissingGlyph names FR41's fifth mode (Story 3.6, AC4):
// a rune covered by no face in its element's declared font chain.
// AD-8 forbids drawing `.notdef` ("never a blank box"), and OPEN-1's
// ruling (a byte-output decision, entering a golden hash under
// AD-21/AD-22) forbids substituting a visible replacement glyph too —
// a document's declared chain is not guaranteed to cover any
// particular substitute, and a silent substitution is the exact class
// of edit AD-8 rejects elsewhere (fontset.go's own "never a silent
// substitution"). So the render OMITS the rune — no glyph, no advance
// — and this Warning is the SOLE record that it happened: it names the
// element id, the rune (as U+XXXX and its literal form), and the exact
// chain that was searched, because naming the chain tells its reader
// what to fix, not only that something is wrong (D-000.37).
//
// Unlike FR41's other four modes, this one is a WARNING, not an Error
// (divergence 6, four independent sources: AD-8, EXPERIENCE.md:216,
// UX-DR22, Story 5.12's first AC) — it travels on the EXISTING
// Result.Diagnostics channel (D-2.8.3's wire), never on D-3.6.3's
// error type.
//
// Additive only (AD-14, verbatim: "changing a code's meaning is a
// breaking change"): once shipped, this string's meaning is permanent.
const DiagCodeTextMissingGlyph = string(diag.CodeTextMissingGlyph)

// DiagCodeTextStyleFaceUndeclared names Story 11.2's absence arm (FR57):
// an element declared style.bold, style.italic or both, and the chain
// entry that COVERS the rune declares no face for that weight and slope.
//
// The rune is drawn in THAT ENTRY'S OWN BASE FACE. It is never taken
// from a later entry in the chain — losing the weight is a smaller lie
// than changing the typeface, and walking the chain for weight is the
// substitution AD-8 forbids — and it is never synthetically emboldened
// or obliqued (I-2: the engine synthesizes neither, anywhere). So the
// page is right in typeface and wrong in weight, and this Warning is the
// SOLE record of the difference: it names the element id, the rune (as
// U+XXXX and its literal form) and the face actually drawn, because
// naming the face is what tells the author which chain entry to give a
// variant to.
//
// A face name is never inferred: an entry naming "Roboto" with no
// declared variant renders Roboto Regular and warns even when the
// supplied FontSet does contain "Roboto Bold" (FR57: "resolved per rune
// through the DECLARED chain").
//
// It travels on the EXISTING Result.Diagnostics channel, never on
// D-3.6.3's error type — the render succeeds.
//
// Additive only (AD-14, verbatim: "changing a code's meaning is a
// breaking change"): once shipped, this string's meaning is permanent.
const DiagCodeTextStyleFaceUndeclared = string(diag.CodeTextStyleFaceUndeclared)

// DiagCodeTextFaceAbsent names spec-deferred-offline-cache CAP-7's
// refusal: a rune that no PRESENT member of its element's declared font
// chain covers, where at least one member of that chain was never
// supplied in the FontSet at all — including the case where no member
// of the chain was supplied, which the vertical model catches one level
// up as "no line height can be derived from it".
//
// It is an ERROR, and it travels on D-3.6.3's error type as a
// *RenderError carrying this code, located at the element and at
// `style.fontFamily`, the field the chain was resolved from. The message
// names THE ABSENT FACE — not the whole chain — because supplying that
// one face is the fix.
//
// WHY IT REFUSES RATHER THAN WARNS, and the distinction from
// DiagCodeTextMissingGlyph is the whole reason it was minted. When every
// declared face IS supplied and none draws the rune, the engine KNOWS
// the document's chain is incomplete and drops the rune under
// DiagCodeTextMissingGlyph. When a declared face was never supplied, the
// engine cannot know whether that face would have covered the rune, so
// dropping it would ship a PDF with text silently missing on a guess.
// Byte-identity exists to prevent exactly that outcome.
//
// IT REFUSES FOR EVERY CALLER, IN EVERY LANGUAGE. A caller passing
// fonts.Shipped() wholesale can never reach it; a caller supplying a
// partial set gets an error where it previously got a Warning and a PDF.
// There is no opt-in flag and no per-language leniency.
//
// Additive only (AD-14, verbatim: "changing a code's meaning is a
// breaking change"): once shipped, this string's meaning is permanent.
const DiagCodeTextFaceAbsent = string(diag.CodeTextFaceAbsent)

// DiagCodeTableFooterSourceUnresolved and DiagCodeTableFooterSourceForbidden
// are DW-6's two long-owed codes (D-1.4.2, R8), minted here now that
// internal/diag exists and both conditions ship (R5, D-000.65: mint
// where the Diagnostic is constructed, when the condition can occur —
// both occur in real documents today). CodeTableFooterSourceUnresolved
// names a table column requesting a sum/avg footer with footerOf
// omitted and a bind that is not one of D-1.4.1's two derivable shapes
// (folio8_expr_validate.go). CodeTableFooterSourceForbidden names
// footerOf paired with footer: "count", or a footer field present with
// no footer at all (internal/template/parse_bands.go, two sites, one
// code — the code names the condition, not the line).
//
// Additive only (AD-14): once shipped, these strings' meanings are
// permanent.
const DiagCodeTableFooterSourceUnresolved = string(diag.CodeTableFooterSourceUnresolved)
const DiagCodeTableFooterSourceForbidden = string(diag.CodeTableFooterSourceForbidden)

// DiagCodeTemplateMalformed, DiagCodeBindingPathAbsent,
// DiagCodeExpressionInvalid and DiagCodeContentUnlayoutable are FR41's
// four ERROR modes (AC4/AC8, R9) — the failure aborts the render, and
// the Diagnostic travels wrapped in a *RenderError (D-3.6.3, arm A),
// never on Result.Diagnostics.
//
// Additive only (AD-14): once shipped, these strings' meanings are
// permanent.
const DiagCodeTemplateMalformed = string(diag.CodeTemplateMalformed)
const DiagCodeBindingPathAbsent = string(diag.CodeBindingPathAbsent)
const DiagCodeExpressionInvalid = string(diag.CodeExpressionInvalid)
const DiagCodeContentUnlayoutable = string(diag.CodeContentUnlayoutable)

// DiagCodeInternalUnhandledCaveat names diagnosticFromCaveat's
// `default:` arm (R12/D-3.6.7, AC7): an internal/expr.Caveat whose Kind
// has no matching arm here — unreachable given expr.CaveatKind's
// current single member, but a live construction site whose output
// must never carry an empty Code (AD-14). This is NOT R7's criterion
// relaxed for an internal condition: the arm already produces a
// Diagnostic that is already returned to a caller (SeverityWarning, on
// Result.Diagnostics), so the choice is between a coded one and a
// codeless one, never between a coded one and a plain error.
//
// Additive only (AD-14): once shipped, this string's meaning is
// permanent.
const DiagCodeInternalUnhandledCaveat = string(diag.CodeInternalUnhandledCaveat)

// DiagCodeDocumentDateInvalid names Story 3.7's reserved params key
// (D-3.7.2) — "documentDate" — carrying a value that is present but not
// a valid RFC 3339 timestamp. FR41's own class (a template-author/
// caller-actionable condition, R7): both Render and Validate reject it
// with a *RenderError carrying this code (AC10), before it can ever
// reach internal/pdf's date assembly.
//
// Additive only (AD-14): once shipped, this string's meaning is
// permanent.
const DiagCodeDocumentDateInvalid = string(diag.CodeDocumentDateInvalid)

// DiagCodeTemplateFieldInvalid names the GENERAL load-stage condition
// (Story 7.8, D-7.8.1): a well-formed `.folio` document carries a field
// value that is not acceptable — a closed-set member that is not in the
// set, a missing required field, a value of the wrong JSON kind, a
// misspelled or duplicated element id. Loading fails with a
// *RenderError carrying this code, and the wrapped *template.LoadError
// names the field, the element (where one applies) and the value.
//
// It is deliberately NOT DiagCodeTemplateMalformed. The WASM engine's
// reportable-message rule replaces that one code's message with a
// generic string, because a malformed-template message quotes the
// offending document back; a located field error does not, and being
// bucketed under TEMPLATE_MALFORMED destroyed every one of them before
// its author could read it. TEMPLATE_MALFORMED still names the
// genuinely malformed template — bytes that are not a JSON object, an
// unreadable value under an unknown key, a MAJOR the library cannot
// load.
//
// Callers DISCRIMINATE ON THE FIELD, not on a per-field code: the rule
// D-7.8.1 settled is that the general code is the default and a
// specific code is minted only when a named consumer must BRANCH on it
// (internal/diag/diag.go states it in full).
//
// Additive only (AD-14): once shipped, this string's meaning is
// permanent.
const DiagCodeTemplateFieldInvalid = string(diag.CodeTemplateFieldInvalid)

// DiagCodeBarcodeUnencodable, DiagCodeBarcodeModuleTooSmall and
// DiagCodeBarcodeDoesNotFit name the barcode element's three render-time
// Warnings (spec-barcode-qr-elements CAP-4). Each carries the element id; the
// first and third omit that barcode, the second still draws it.
const DiagCodeBarcodeUnencodable = string(diag.CodeBarcodeUnencodable)
const DiagCodeBarcodeModuleTooSmall = string(diag.CodeBarcodeModuleTooSmall)
const DiagCodeBarcodeDoesNotFit = string(diag.CodeBarcodeDoesNotFit)

// DiagCodeQRCodeTooLong, DiagCodeQRCodeModuleTooSmall and
// DiagCodeQRCodeDoesNotFit name the qrcode element's three render-time
// Warnings (spec-barcode-qr-elements CAP-4). Each carries the element id; the
// first and third omit that QR code, the second still draws it.
const DiagCodeQRCodeTooLong = string(diag.CodeQRCodeTooLong)
const DiagCodeQRCodeModuleTooSmall = string(diag.CodeQRCodeModuleTooSmall)
const DiagCodeQRCodeDoesNotFit = string(diag.CodeQRCodeDoesNotFit)

// DiagCodeSectionBreakInvalid and DiagCodeSectionBreakStraddled name the
// section break's two load errors (spec-section-break CAP-5): a break the
// content band cannot honour, located at the band, and an element on both
// sides of the break, located at the element. DiagCodeSectionBreakSplitsKeepTogether
// is the Warning for a keepTogether group the break splits.
const DiagCodeSectionBreakInvalid = string(diag.CodeSectionBreakInvalid)
const DiagCodeSectionBreakStraddled = string(diag.CodeSectionBreakStraddled)
const DiagCodeSectionBreakSplitsKeepTogether = string(diag.CodeSectionBreakSplitsKeepTogether)

// DiagCodePagesInvalid names a `pages` array (SPEC-multi-pages CAP-7) that
// cannot be loaded. The load error's data path names `pages`, `pages[i]` with
// its key, or `bands.content`.
const DiagCodePagesInvalid = string(diag.CodePagesInvalid)

// DiagCodeTableHeaderRepeatSuppressed names Story 4.4's own new condition
// (FR26, DECISION-2 as ruled by the engineering lead): a table's repeated
// header could not be honoured on one continuation page because the next
// unplaced row fits the bare content window but not the window under the
// header's own reserved height. The repeat is suppressed on that ONE page
// only — the row is still placed and the render still completes — and this
// Warning travels on Result.Diagnostics (never wrapped in a *RenderError:
// the document still renders) as the sole record that FR26 did not hold on
// that page. Distinct from DiagCodeContentUnlayoutable, which names an
// element taller than the content window itself — this row is not; it is
// taller than the window less a reservation FR26 itself introduces.
//
// Additive only (AD-14): once shipped, this string's meaning is permanent.
const DiagCodeTableHeaderRepeatSuppressed = string(diag.CodeTableHeaderRepeatSuppressed)

// DiagCodeTableFooterOrphanSuppressed names Story 4.5's own new condition
// (FR25, DECISION-2 as ruled by the engineering lead): a table's footer row
// and the data row immediately preceding it in the bound collection, together,
// exceed the content window, so the orphan rule cannot be honoured by moving
// the two together. The footer is placed alone on that page instead — the
// render still completes — and this Warning travels on Result.Diagnostics
// (never wrapped in a *RenderError) as the sole record that the rule did not
// hold there. Distinct from DiagCodeTableHeaderRepeatSuppressed: that
// condition drops a declared element; this one relocates nothing further and
// leaves the footer present, in the position the rule exists to prevent.
//
// Additive only (AD-14): once shipped, this string's meaning is permanent.
const DiagCodeTableFooterOrphanSuppressed = string(diag.CodeTableFooterOrphanSuppressed)

// DiagCodeTableRowClippedHeight names Story 4.6's own new condition (FR25,
// AD-14, D-4.6.3 as ruled by the engineering lead): a table group — a header
// row, a data row, or the footer row — is by itself taller than the whole
// content window, so no page in the document could hold it. It is placed
// alone on a fresh page and CUT OFF at that page's content bottom; the lines
// that fell past the bottom are absent from the document permanently. The
// render still completes and returns bytes, and this Warning travels on
// Result.Diagnostics (never wrapped in a *RenderError) as the sole record
// that content was destroyed.
//
// Distinct from DiagCodeContentUnlayoutable, which is still what an
// UNGROUPED over-tall item produces — a line whose font size exceeds the
// content band, or an image whose declared box does. That asymmetry is
// D-4.6.2's ruling and AD-13's line: a row's height is derived from DATA the
// author may never have seen, while a font size and an image box are things
// the author typed. Distinct from DiagCodeTextClippedWidth, which D-2.8.1
// scopes to the horizontal axis at a box edge. Distinct from
// DiagCodeTableHeaderRepeatSuppressed (which drops a redrawn copy) and
// DiagCodeTableFooterOrphanSuppressed (which leaves everything present):
// this one is the only diagnostic in the family that reports LOST CONTENT.
//
// Additive only (AD-14): once shipped, this string's meaning is permanent.
const DiagCodeTableRowClippedHeight = string(diag.CodeTableRowClippedHeight)

// DiagCodeTableMinHeightUnplaceable names SPEC-table-rules §3's refusal:
// a table whose `minHeight` exceeds the content window it must be placed
// in. A LOAD error, decidable from the document alone — see
// diag.CodeTableMinHeightUnplaceable for why it is coded rather than
// left to become TEMPLATE_MALFORMED.
const DiagCodeTableMinHeightUnplaceable = string(diag.CodeTableMinHeightUnplaceable)

// Diagnostic is AD-14's one diagnostic/error value. Every failure mode
// AD-14 names — an over-tall row and an over-tall keep-together group
// (FR25, FR51; both built, Stories 4.6 and 7.7) and clipped content
// (FR44) — is expressed as a value of this type, never a bespoke
// per-area type: the whole point of AD-14 is that a caller (a CLI, a
// designer, this library's own tests) checks one shape for every kind of
// thing that can go wrong with one element.
//
// AD-14's leniency for a keep-together group is scoped to the AGGREGATE
// case since Story 7.10: a group holding a template element that is by
// itself taller than a content window is an Error, not a Warning
// (D-7.10.1, D-7.10.2). That is a fact about which severity a condition
// gets, not about this type, which carries both.
type Diagnostic struct {
	// Severity says which of AD-14's two dispositions this Diagnostic
	// carries, and BOTH are constructed today — which is what the
	// paragraph above means by "not about this type, which carries
	// both". Where a Diagnostic is READ decides which it will be: one
	// reached through Result.Diagnostics accompanies a successful
	// render and is always a Warning, while an Error aborts the render
	// and reaches the caller as *RenderError's own Diagnostic instead
	// (SeverityError, minted since Story 3.6; an over-tall
	// author-declared group is one such, Story 7.10). See Severity's
	// own doc comment.
	Severity Severity

	// Code is a stable string from the closed registry above. Never
	// prose, never localized, never sentence-shaped — a caller
	// programmatically dispatching on the kind of problem (a CLI
	// choosing an icon, a designer highlighting a category) matches
	// this field, not Message.
	Code string

	// ElementID is the element the Diagnostic concerns, when it
	// concerns one (AD-10: "every diagnostic and every error that
	// concerns a template element carries its id"). Empty only for a
	// Diagnostic that names no single element — no such Diagnostic
	// exists in this story.
	ElementID string

	// DataPath is the bound data path the Diagnostic concerns, when it
	// concerns one. Empty for FR44's clip: a width overflow is a
	// property of the LAID-OUT content against its declared box, not of
	// one JSON value at one path.
	DataPath string

	// Message is a human-readable sentence, safe to print directly to a
	// designer or a CLI. It is never parsed by this library or by any
	// test asserting on a Diagnostic's Code/ElementID instead (D-000.21).
	Message string
}

// Result is Render's return value (D-2.8.3, the owner's decision,
// verbatim):
//
//	type Result struct {
//	    Bytes       []byte
//	    Diagnostics []Diagnostic
//	}
//	func Render(t *Template, d Data, p Params, f FontSet) (Result, error)
//
// Bytes is the complete, valid PDF document — present whenever error is
// nil, exactly as Render's pre-2.8 []byte return was. Diagnostics is
// every Warning the render produced, alongside those bytes; it is never
// consulted to decide whether Bytes is valid — a non-nil error is the
// only signal a caller needs for that, unchanged from before this
// story.
//
// Diagnostics ordering and emptiness are BOTH determinism guarantees,
// stated here because nothing else in this module checks a non-byte
// output surface (D-2.8.6):
//
//   - ORDER IS DOCUMENT ORDER: band order (page header, then content,
//     then page footer), and within one band, element DECLARATION order
//     — the order elements appear in the authored `.folio` document.
//     NEVER map order, and never an order that depends on which
//     goroutine or which pass happened to finish detecting an overflow
//     first.
//   - EMPTY IS nil, ONE REPRESENTATION: a render that produced no
//     Diagnostic ever sets Diagnostics to a non-nil empty slice. Two
//     renders of the same document therefore compare equal under
//     reflect.DeepEqual exactly when they produce identical bytes —
//     never differing only in whether Diagnostics is nil or []Diagnostic{}.
type Result struct {
	Bytes       []byte
	Diagnostics []Diagnostic
}
