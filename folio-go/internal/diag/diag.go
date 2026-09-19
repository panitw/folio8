// Package diag is AD-14's closed code registry (Story 3.6, D-3.6.4,
// ruled arm A): a Code defined type, the code constants, and the
// REGISTRY VALUE itself — a constructed value the package builds, not
// a bare const block (D-1.4.2's decision-log row `:9118`: "assert
// membership in the registry as constructed, not the existence of two
// string constants — a constant nothing registers is not a code").
//
// It holds ONLY the registry. `folio8.Diagnostic`, `folio8.Severity` and
// `folio8.Result` stay at the module root (D-2.8.3, an owner decision;
// see ARCHITECTURE-SPINE.md's AD-14, "Errors and diagnostics are one
// type on one channel", as amended by Story 2.8's AC11 — cited by AD
// NUMBER rather than by line, because a line number in a living
// document goes stale on the next edit to any line above it, DW-65).
//
// The caveat-kind -> code MAPPING (diagnosticFromCaveat) also stays out
// of this package and in folio-go/render.go — it switches on
// internal/expr.CaveatKind and constructs a folio8.Diagnostic, and this
// package may import neither (see below).
//
// This package imports NO first-party package, not even
// internal/geom. That is not an accident of what a registry happens to
// need — it is the REQUIREMENT R1 names: AC4's five FR41 conditions
// arise in internal/template (stage rank 2), internal/expr (rank 3),
// internal/bind (rank 4), internal/fontset (rank 6) and internal/layout
// (rank 7) — five packages spanning the whole pipeline. lint's
// stage-rank guard permits a package to import only a STRICTLY LOWER
// rank, so only a rank-0/1 leaf is importable by every one of those
// five. This package is ranked 1 (folio-go/internal/, stagerank.go)
// precisely so every stage can attach a registry code to a Diagnostic
// it constructs — the first first-party import added here would
// foreclose whichever of those five packages sits at or above this
// one's rank. internal/arch's TestDiagPackageHasZeroFirstPartyImports
// asserts this property directly, rather than leaving it to be merely
// observed (AC1).
package diag

// Code is a stable, closed-registry string naming one failure mode a
// caller can programmatically dispatch on (AD-14: "a stable string
// code from a closed registry"). A defined type, never a bare string
// (R4, D-000.68's compiler anchor): a naked string literal cannot be
// registered by accident, and a mistyped code does not silently become
// a new one. folio8.Diagnostic.Code stays `string` (a public API
// surface this story does not widen); DiagCodeTextClippedWidth and
// DiagCodeEmptyAverage bridge as `= string(diag.CodeX)`, keeping the
// two spellings in permanent lockstep.
type Code string

// The registered codes.
//
// R7 (D-3.6.6), ratified as the criterion for what earns a code here,
// stated in this doc comment because a principle beside the thing it
// governs is read by whoever touches this file next, not only by
// whoever reads the story that minted it:
//
//	A code exists for a failure mode a caller can act on — FR41's
//	five, plus conditions a template author can cause and fix. An
//	internal-invariant violation ("this should never happen") stays
//	a plain error.
//
// Codes are ADDITIVE ONLY (AD-14, verbatim): once shipped, a code's
// string and its meaning are permanent. Never repurpose one; never
// change its string. AC5's registry test pins every shipped code's
// exact string against a literal the test owns, independent of this
// file, so a change here that alters a byte is caught as the breaking
// change it is.
const (
	// CodeTextClippedWidth names a text element's widest packed line
	// exceeding its declared width (FR44, D-2.8.1). Story 2.8's
	// original code, absorbed here unchanged (AC3): the public
	// constant is folio8.DiagCodeTextClippedWidth.
	CodeTextClippedWidth Code = "TEXT_CLIPPED_WIDTH"

	// CodeEmptyAverage names avg() evaluated over a present-but-empty
	// collection (Story 3.3, DECISION-5). Absorbed here unchanged
	// (AC3): the public constant is folio8.DiagCodeEmptyAverage.
	CodeEmptyAverage Code = "AGGREGATE_EMPTY_AVERAGE"

	// CodeTableFooterSourceUnresolved names a table column with
	// footer: "sum"/"avg", footerOf omitted, and a bind that is not one
	// of D-1.4.1's two derivable shapes (DW-6, R8; site:
	// folio-go/folio8_expr_validate.go:316-320).
	CodeTableFooterSourceUnresolved Code = "TABLE_FOOTER_SOURCE_UNRESOLVED"

	// CodeTableFooterSourceForbidden names footerOf paired with
	// footer: "count", or a footer field present with no footer at all
	// (DW-6, R8, D-1.4.2's parenthetical; sites:
	// internal/template/parse_bands.go:408-410, :419-421 — one code,
	// two sites, because the code names the CONDITION, not the line).
	CodeTableFooterSourceForbidden Code = "TABLE_FOOTER_SOURCE_FORBIDDEN"

	// CodeTemplateMalformed names FR41's "malformed template" mode: a
	// `.folio` document that fails to load (internal/template's
	// load-time validation, surfaced through folio8.LoadTemplate/
	// ParseTemplate). AC4/AC8.
	CodeTemplateMalformed Code = "TEMPLATE_MALFORMED"

	// CodeBindingPathAbsent names FR41's "unresolvable binding" mode:
	// a data or params path an element's binding names is absent from
	// the document supplied at render time (AD-14's own "absent" data
	// case: "an absent path is an Error carrying the path"). AC4/AC8.
	CodeBindingPathAbsent Code = "BINDING_PATH_ABSENT"

	// CodeExpressionInvalid names FR41's "invalid expression" mode: a
	// "{{ }}" expression that does not parse or does not check (syntax,
	// arity, unknown function name, wrong-kind literal argument). AC4/
	// AC8.
	CodeExpressionInvalid Code = "EXPRESSION_INVALID"

	// CodeContentUnlayoutable names FR41's "unlayoutable content" mode:
	// an element (a text line or an image) taller than the content
	// window it must fit inside (internal/layout.OverflowError, FR44/
	// D-2.6.1). AC4/AC8.
	CodeContentUnlayoutable Code = "CONTENT_UNLAYOUTABLE"

	// CodeTextMissingGlyph names FR41's fifth mode, and the one
	// Warning among the five (AD-8, divergence 6, ruled OPEN-1): a rune
	// covered by no face in its element's declared font chain. Per
	// OPEN-1's ruling, the render OMITS the rune — no glyph, no
	// advance — rather than drawing `.notdef` or substituting a
	// visible replacement; this Warning is the SOLE record that the
	// rune was dropped, so its message names the element id, the rune
	// (both as U+XXXX and its literal form), and the exact chain that
	// was searched (D-000.37: a diagnostic must be actionable, not just
	// present). AC4.
	CodeTextMissingGlyph Code = "TEXT_MISSING_GLYPH"

	// CodeTextStyleFaceUndeclared names Story 11.2's absence arm
	// (FR57): an element asked its font chain for a weight or a slope,
	// and the chain entry that COVERS the rune declares no face for it.
	// The rune is drawn in that entry's OWN base face — never in another
	// entry's, and never synthetically emboldened or obliqued (I-2) — so
	// the page is correct in typeface and wrong in weight, and this
	// Warning is the sole record of the difference. Its message names the
	// element id, the rune (as U+XXXX and its literal form), and THE FACE
	// the rune was actually drawn in.
	//
	// IT IS NOT CodeTextMissingGlyph, AND THE DISTINCTION IS THE REASON
	// IT WAS MINTED. That code means the rune was DROPPED — no glyph, no
	// advance — and its message names the whole chain that was searched.
	// This one means the rune was DRAWN, at the wrong weight, by one
	// named face. D-4.5.1's two-limb reuse test fails on both limbs (a
	// different author action, a different thing done to the document),
	// and reuse would make the shipped TEXT_MISSING_GLYPH message text
	// false — which AD-14 makes a breaking change.
	CodeTextStyleFaceUndeclared Code = "TEXT_STYLE_FACE_UNDECLARED"

	// CodeTextFaceAbsent names the SUPPLY failure spec-deferred-offline-
	// cache CAP-7 requires: a rune that no PRESENT member of its
	// element's declared font chain covers, where at least one member of
	// that chain was never supplied in the FontSet at all. The engine
	// cannot ask whether the absent face would have covered the rune, so
	// it refuses the render rather than dropping the rune and shipping a
	// PDF with the text silently gone. The same code names the condition
	// one level up, in the vertical model: a chain with no present member
	// at all, from which no line height can be derived.
	//
	// IT IS NOT CodeTextMissingGlyph, AND THAT IS THE WHOLE POINT. That
	// code means every declared face WAS supplied and none of them draws
	// the rune — a document whose chain is genuinely incomplete, which
	// stays a Warning with the rune omitted. This one means the caller's
	// FontSet is short of a face the document declares, which is a
	// different author action with a different remedy (supply the face,
	// not edit the chain).
	//
	// IT IS ALSO NOT CodeTextStyleFaceUndeclared, which is about what the
	// CHAIN DECLARES rather than what the CALLER SUPPLIED, and which
	// still draws the rune.
	CodeTextFaceAbsent Code = "TEXT_FACE_ABSENT"

	// CodeInternalUnhandledCaveat names an internal/expr.Caveat whose
	// Kind has no matching arm in diagnosticFromCaveat (render.go) —
	// unreachable given expr.CaveatKind's current single member, but a
	// live, returnable construction site whose output must never carry
	// an empty Code (AD-14; R12/D-3.6.7, AC7). This is NOT a case of
	// R7's criterion being relaxed for an internal condition: the arm
	// already produces a Diagnostic that is already returned to a
	// caller, so the only choice is between a coded one and a codeless
	// one, never between a coded one and a plain error.
	CodeInternalUnhandledCaveat Code = "INTERNAL_UNHANDLED_CAVEAT"

	// CodeDocumentDateInvalid names Story 3.7's reserved params key
	// (D-3.7.2) carrying a value that is present but not a valid RFC
	// 3339 timestamp — a template author/caller-actionable condition
	// (R7's own criterion) caught by both Render and Validate (D-3.7.1)
	// before it ever reaches internal/pdf's date assembly. AC10.
	CodeDocumentDateInvalid Code = "DOCUMENT_DATE_INVALID"

	// CodeTableHeaderRepeatSuppressed names Story 4.4's own new
	// condition (FR26, DECISION-2 as ruled): a table's repeated header
	// could not be honoured on one continuation page because the next
	// unplaced row fits the bare content window but not the window
	// under the header's own reserved height. The repeat is suppressed
	// on that ONE page only (never a hard error — CodeContentUnlayoutable
	// names a DIFFERENT condition, an element taller than the content
	// window itself, which this is not) and the render completes; this
	// Warning is the record that FR26 did not hold there. Minted here,
	// at the point the condition first ships (R7/D-000.65: the ruling
	// records that this is the lead's authorization, not a deferred
	// owner call).
	CodeTableHeaderRepeatSuppressed Code = "TABLE_HEADER_REPEAT_SUPPRESSED"

	// CodeTableFooterOrphanSuppressed names Story 4.5's own new
	// condition (FR25, DECISION-2 as ruled by the engineering lead): a
	// table's footer row and the data row immediately preceding it in
	// the bound collection, together, are taller than the content
	// window — so the orphan rule (a footer never lands alone at the
	// top of a page) cannot be honoured by moving the two together. The
	// footer is placed alone on that page instead, and this Warning is
	// the record that the rule did not hold there. Distinct from
	// CodeTableHeaderRepeatSuppressed (Story 4.4): that condition DROPS
	// a declared element (the repeated header is simply absent);this
	// one RELOCATES nothing further and leaves the footer PRESENT, just
	// in the position the rule exists to prevent — same remedy (suppress
	// on one page, record, never error), a different document, per the
	// engineering lead's discriminator ("same remedy is not sufficient;
	// same thing must have happened to the document"). Minted here, at
	// the point the condition first ships (R7/D-000.65).
	CodeTableFooterOrphanSuppressed Code = "TABLE_FOOTER_ORPHAN_SUPPRESSED"

	// CodeTableRowClippedHeight names Story 4.6's own new condition
	// (FR25, AD-14, D-4.6.3 as ruled): a table GROUP — a header row, a
	// data row, or the footer row — is by itself taller than the whole
	// content window, so no page anywhere in the document could hold
	// it. AD-14 rules this never fatal, so the group is placed alone on
	// a fresh page and CUT OFF at that page's content bottom: whole
	// lines past the bottom are absent from the document forever, and
	// this Warning is the only record that they existed.
	//
	// MINTED rather than widened, on D-4.5.1's discriminator ("two
	// conditions share a code only if the author would take the same
	// action AND the same thing happened to their document"), against
	// all three neighbours:
	//
	//   - CodeTextClippedWidth (Story 2.8) is scoped by D-2.8.1 to the
	//     HORIZONTAL axis and to a box edge; it never reads a height.
	//     This is vertical, at a PAGE edge, and it destroys whole lines
	//     rather than the tail of one.
	//   - CodeTableHeaderRepeatSuppressed (Story 4.4) DROPS a redrawn
	//     copy of something still present elsewhere in the document.
	//   - CodeTableFooterOrphanSuppressed (Story 4.5) MOVES nothing and
	//     leaves everything present.
	//
	// This one DESTROYS content — a third thing, and the only one of
	// the three a reader can suffer without noticing, because a row
	// ending at the bottom of a page is exactly what a normal page
	// break looks like.
	//
	// ONE CODE FOR ALL THREE GROUP ROLES, and that is the test cutting
	// the other way rather than a reflex: an over-tall header row, an
	// over-tall data row and an over-tall footer row are the same thing
	// happening to the document (a table group taller than the window,
	// clipped, content destroyed) with the same remedy (shorten it, or
	// enlarge the page). Both limbs of D-4.5.1 are satisfied, so the
	// ROLE and the row index travel in the message, not in the code.
	// Minted here, at the point the condition first ships (R7/D-000.65,
	// D-1.4.2: never ahead of it).
	CodeTableRowClippedHeight Code = "TABLE_ROW_CLIPPED_HEIGHT"

	// CodeTableMinHeightUnplaceable names SPEC-table-rules §3's one
	// refusal: a table whose `minHeight` is TALLER THAN THE CONTENT
	// WINDOW it would have to be placed in. The alternative to refusing
	// is a table that can never be placed on any page of the document
	// that declares it, discovered at render, on every page.
	//
	// IT EARNS A CODE ON R7's TERMS: a template author causes it and a
	// template author fixes it — shorten the floor, or give the page
	// more room — and it must reach them with the element's id attached.
	// An uncoded load rejection becomes CodeTemplateMalformed at
	// folio8.ParseTemplate's boundary, and wasm/cmd/engine's
	// reportableMessage replaces THAT message, and only that one, with
	// "The template could not be processed" — so an uncoded refusal here
	// would never reach the author at all.
	//
	// DISTINCT FROM CodeContentUnlayoutable, which names an element
	// measured too tall for its window at RENDER. This one is decidable
	// from the document alone, with no data and no measurement, so it is
	// answered at load where the author is still holding the file.
	CodeTableMinHeightUnplaceable Code = "TABLE_MIN_HEIGHT_UNPLACEABLE"

	// CodeTemplateFieldInvalid names the GENERAL LOAD-STAGE condition: a
	// well-formed template carries a field value that is not acceptable.
	// A closed-set member that is not in the set, a missing required
	// field, a value of the wrong JSON kind, an id that is misspelled or
	// duplicated — every one of them is the same thing happening to the
	// document and the same thing the reader must do about it.
	//
	// THE REGISTRY-POLICY RULE THIS SETTLES (D-7.8.1, ruled 2026-08-31,
	// answering the reservation this const block carried until Story
	// 7.8):
	//
	//	The GENERAL code is the DEFAULT. A SPECIFIC code is minted only
	//	when a NAMED CONSUMER must BRANCH on it to behave differently.
	//	Everything else discriminates on the FIELD datum, which can grow
	//	freely without touching a closed registry.
	//
	// That is D-7.3.1's own lesson applied one level up — partition by
	// what the consumer DOES, not by where the value is written. A
	// designer receiving any of these does exactly one thing: locate the
	// element, name the field, show the value and the reason. One
	// behaviour, one code. Without this rule AD-14's closed registry
	// accretes one entry per style field forever.
	//
	// SUPPLIED BY THE CONSTRUCTOR, not by a per-site decision:
	// internal/template's newLoadError attaches it, so every uncoded
	// load-error site in that package became coded by construction, with
	// no enumeration and no per-site judgement. newLoadErrorCoded stays
	// as the override for the conditions that genuinely need
	// discrimination.
	//
	// WHY IT HAD TO EXIST AT ALL. An uncoded *template.LoadError became
	// CodeTemplateMalformed at folio8.ParseTemplate's boundary, and
	// wasm/cmd/engine's reportableMessage replaces THAT code's message —
	// and only that one — with "The template could not be processed". So
	// every located load error in the format was destroyed before its
	// author saw it. That destruction rule exists because a
	// malformed-template message quotes the offending document back; a
	// LoadError's message is "field F (element E): reason (value: V)".
	//
	// THAT LAST CLAUSE USED TO END "which does not", AND THAT WAS FALSE
	// (D-7.8.5). A LoadError's message DOES quote the document back —
	// `value` is author-supplied, and at nine call sites it is an
	// arbitrary JSON sub-object. The premise is true only because
	// internal/template's LoadError.Error() MAKES it true: it bounds
	// every author-supplied fragment in runes as it renders one, and
	// bounds the assembled sentence to this host's own 512-byte window,
	// always cutting on a rune boundary and always leaving a visible
	// elision marker. Read the two together — moving a population off
	// TEMPLATE_MALFORMED is safe because, and only because, that bound
	// exists. TEMPLATE_MALFORMED keeps destroying its own
	// messages, for the reason it was written; what changed at Story 7.8
	// is that LoadErrors stopped being bucketed there. It still names
	// the genuinely malformed template — bytes that are not a JSON
	// object, an unreadable value under an unknown key, a MAJOR the
	// library cannot load.
	//
	// LOAD STAGE ONLY. It does not absorb render-stage conditions.
	//
	// D-7.8.2, DISCHARGED BEFORE THE v1.0.0 TAG (2026-09-17). The audit
	// this comment used to schedule found that no consumer branches on
	// the two per-field style codes Stories 4.1 and 7.2 minted, so both
	// were retired while removing a code was still free (AD-14). An
	// out-of-domain lineSpacing is this code at its one load site, and a
	// colour that is not #RRGGBB is now refused at load under this code
	// too, located at the colour field (owner ruling, 2026-09-17).
	CodeTemplateFieldInvalid Code = "TEMPLATE_FIELD_INVALID"

	// CodeBarcodeUnencodable names a barcode whose value, resolved from
	// DATA at render, carries a character Code 128 cannot encode (above
	// ASCII 127, including Thai script). The barcode is omitted and the
	// render completes (owner decision: one bad record never stops a print
	// run). A STATIC unencodable value is refused at load instead, under
	// the general load code.
	CodeBarcodeUnencodable Code = "BARCODE_UNENCODABLE"

	// CodeBarcodeModuleTooSmall names a barcode that fits its box only with
	// modules narrower than 0.25 mm (709 mp). The bars ARE drawn; this is
	// the record that a scanner may not read them.
	CodeBarcodeModuleTooSmall Code = "BARCODE_MODULE_TOO_SMALL"

	// CodeBarcodeDoesNotFit names a barcode whose symbol plus quiet zones
	// cannot fit its box even at 1 mp per module. The barcode is omitted
	// and the render completes. Distinct from CodeBarcodeModuleTooSmall on
	// D-4.5.1's discriminator: that one DRAWS, this one draws nothing.
	CodeBarcodeDoesNotFit Code = "BARCODE_DOES_NOT_FIT"

	// CodeQRCodeTooLong names a qrcode whose value, resolved from DATA at
	// render, is longer than a version-40 symbol holds at its
	// error-correction level. The QR code is omitted and the render
	// completes (the barcode's owner decision). A STATIC value that long is
	// refused at load instead, under the general load code.
	CodeQRCodeTooLong Code = "QRCODE_TOO_LONG"

	// CodeQRCodeModuleTooSmall names a qrcode that fits its box only with
	// modules narrower than 0.5 mm (1418 mp). The modules ARE drawn.
	CodeQRCodeModuleTooSmall Code = "QRCODE_MODULE_TOO_SMALL"

	// CodeQRCodeDoesNotFit names a qrcode whose symbol plus its 4-module
	// quiet zone cannot fit the smaller side of its box even at 1 mp per
	// module. The QR code is omitted and the render completes.
	CodeQRCodeDoesNotFit Code = "QRCODE_DOES_NOT_FIT"

	// CodeSectionBreakInvalid names a content band whose `sectionBreak`
	// cannot be honoured (spec-section-break CAP-5): an offset at or above
	// the band's top or at or below its derived content height, the key
	// declared twice, or the key on the page header or page footer. A LOAD
	// error located at the band, because a template author causes and fixes
	// it, and an uncoded refusal would become TEMPLATE_MALFORMED and never
	// reach them.
	CodeSectionBreakInvalid Code = "SECTION_BREAK_INVALID"

	// CodePagesInvalid names a document whose top-level `pages` array
	// (SPEC-multi-pages CAP-7) cannot be loaded: fewer than two entries, an
	// entry that is not an object or carries an unknown key, a non-boolean
	// `pageBreak`, content declared in both `pages` and `bands.content`, or a
	// keepTogether group spanning pages. A LOAD error located at `pages`,
	// `pages[i]` (with its key) or `bands.content`.
	CodePagesInvalid Code = "PAGES_INVALID"

	// CodeSectionBreakStraddled names an element whose declared box lies on
	// both sides of the content band's `sectionBreak` — a LOAD error located
	// at the element. Every element must be unambiguously above or below the
	// line, because only the content below it moves.
	CodeSectionBreakStraddled Code = "SECTION_BREAK_STRADDLED"

	// CodeSectionBreakSplitsKeepTogether names a keepTogether group with
	// members on both sides of the section break. The break wins: the group
	// is split at the line and each side is kept together on its own. A
	// Warning, emitted on every render of such a document.
	CodeSectionBreakSplitsKeepTogether Code = "SECTION_BREAK_SPLITS_KEEP_TOGETHER"
)

// allCodes is the registry's own enumeration, in the order the codes
// above are declared. It is the ONE place that lists every shipped
// code; buildRegistry constructs the lookup value from it, and nothing
// else in this package or its callers may bypass it to test a bare
// string literal for validity.
var allCodes = []Code{
	CodeTextClippedWidth,
	CodeEmptyAverage,
	CodeTableFooterSourceUnresolved,
	CodeTableFooterSourceForbidden,
	CodeTemplateMalformed,
	CodeBindingPathAbsent,
	CodeExpressionInvalid,
	CodeContentUnlayoutable,
	CodeTextMissingGlyph,
	CodeTextStyleFaceUndeclared,
	CodeTextFaceAbsent,
	CodeInternalUnhandledCaveat,
	CodeDocumentDateInvalid,
	CodeTableHeaderRepeatSuppressed,
	CodeTableFooterOrphanSuppressed,
	CodeTableRowClippedHeight,
	CodeTableMinHeightUnplaceable,
	CodeTemplateFieldInvalid,
	CodeBarcodeUnencodable,
	CodeBarcodeModuleTooSmall,
	CodeBarcodeDoesNotFit,
	CodeQRCodeTooLong,
	CodeQRCodeModuleTooSmall,
	CodeQRCodeDoesNotFit,
	CodeSectionBreakInvalid,
	CodeSectionBreakStraddled,
	CodeSectionBreakSplitsKeepTogether,
	CodePagesInvalid,
}

// registry is the CONSTRUCTED value R2 requires (D-1.4.2 `:9118`): a
// map the package builds from allCodes, not the constants themselves.
// Registered queries this value, never the const block directly.
var registry = buildRegistry()

// Disposition records how a registered code reaches callers. It deliberately
// lives beside the closed registry: a string code alone cannot tell a dynamic
// census whether it must exercise RenderError or a successful Result warning.
type Disposition uint8

const (
	DispositionWarning Disposition = iota + 1
	DispositionError
)

var dispositions = map[Code]Disposition{
	CodeTextClippedWidth:               DispositionWarning,
	CodeEmptyAverage:                   DispositionWarning,
	CodeTableFooterSourceUnresolved:    DispositionError,
	CodeTableFooterSourceForbidden:     DispositionError,
	CodeTemplateMalformed:              DispositionError,
	CodeBindingPathAbsent:              DispositionError,
	CodeExpressionInvalid:              DispositionError,
	CodeContentUnlayoutable:            DispositionError,
	CodeTextMissingGlyph:               DispositionWarning,
	CodeTextStyleFaceUndeclared:        DispositionWarning,
	CodeTextFaceAbsent:                 DispositionError,
	CodeInternalUnhandledCaveat:        DispositionWarning,
	CodeDocumentDateInvalid:            DispositionError,
	CodeTableHeaderRepeatSuppressed:    DispositionWarning,
	CodeTableFooterOrphanSuppressed:    DispositionWarning,
	CodeTableRowClippedHeight:          DispositionWarning,
	CodeTableMinHeightUnplaceable:      DispositionError,
	CodeTemplateFieldInvalid:           DispositionError,
	CodeBarcodeUnencodable:             DispositionWarning,
	CodeBarcodeModuleTooSmall:          DispositionWarning,
	CodeBarcodeDoesNotFit:              DispositionWarning,
	CodeQRCodeTooLong:                  DispositionWarning,
	CodeQRCodeModuleTooSmall:           DispositionWarning,
	CodeQRCodeDoesNotFit:               DispositionWarning,
	CodeSectionBreakInvalid:            DispositionError,
	CodeSectionBreakStraddled:          DispositionError,
	CodeSectionBreakSplitsKeepTogether: DispositionWarning,
	CodePagesInvalid:                   DispositionError,
}

// Classified reports the registry-owned disposition for c. A registered code
// without one is a broken registry and the Story 6.7 census fails loudly.
func Classified(c Code) (Disposition, bool) {
	d, ok := dispositions[c]
	return d, ok && Registered(c)
}

// ErrorCodes returns the registry's render-stopping Error members in declared
// order. It is intentionally derived from All rather than a test-owned list.
func ErrorCodes() []Code {
	out := make([]Code, 0, len(allCodes))
	for _, c := range All() {
		if d, ok := Classified(c); ok && d == DispositionError {
			out = append(out, c)
		}
	}
	return out
}

func buildRegistry() map[Code]struct{} {
	m := make(map[Code]struct{}, len(allCodes))
	for _, c := range allCodes {
		m[c] = struct{}{}
	}
	return m
}

// Registered reports whether c is a member of the registry AS
// CONSTRUCTED — the lookup R2 requires, never a check against the
// const block or against allCodes directly.
func Registered(c Code) bool {
	_, ok := registry[c]
	return ok
}

// All returns every registered code, in the registry's own declared
// order — a copy, so a caller cannot mutate the package's set. Tests
// that must enumerate the registry as constructed (AC5, AC7) call this
// rather than re-deriving a list of their own.
func All() []Code {
	out := make([]Code, len(allCodes))
	copy(out, allCodes)
	return out
}
